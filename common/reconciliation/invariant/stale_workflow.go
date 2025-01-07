// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package invariant

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/types"
)

const (
	// how long to allow things to be beyond when they should have been fully cleaned up, for safety purposes
	retentionSafetyMargin = time.Hour * 24 * 10

	// time.DateOnly, introduced in go1.20
	//
	// deprecated: use time.DateOnly when able
	dateOnly = "2006-01-02"
)

// sane time ranges for workflows.
// this is largely to protect against accidental conversions from zero values, or incorrect second/nanosecond/etc scales.
var (
	// too old to be real.  e.g. prior to Cadence existing.
	//
	// anything "reasonable" is likely fine, but this should be substantially newer than 1970 to catch zero times.
	impossiblyOld = time.Date(2015, 0, 0, 0, 0, 0, 0, time.UTC)
	// too far in the future to be real.
	//
	// crazy values are technically possible if someone sets a 100-year timeout on a workflow or something,
	// but we should probably block those anyway.  hopefully nonexistent or rare enough to handle manually (and correct).
	impossiblyFuture = time.Date(2100, 0, 0, 0, 0, 0, 0, time.UTC)
	// our internal limits are:
	// - practically: 30 days
	// - for monthly-job people: 40 days
	// - rare exceptions we are trying to eliminate: 90+ days
	//
	// this should be far beyond that and should imply bad data, not merely an exceptional domain.
	//
	// note that this is not used to *force* deletion.  workflows in domains beyond this value will fail and be skipped.
	maxRetentionDays = 200
)

type staleWorkflowCheck struct {
	pr  persistence.Retryer
	dc  cache.DomainCache
	log *zap.Logger
}

// NewStaleWorkflow checks to see if a workflow has out-lived its retention window.
// This primarily asserts that that now < (min(start + execution timeout, min) + (domain retention*2)).
// If a workflow fails this check, its data is still lingering well beyond when it should have been cleaned up.
func NewStaleWorkflow(
	pr persistence.Retryer,
	dc cache.DomainCache,
	log *zap.Logger,
) Invariant {
	return &staleWorkflowCheck{
		pr:  pr,
		dc:  dc,
		log: log,
	}
}

func (c *staleWorkflowCheck) Check(
	ctx context.Context,
	execution interface{},
) CheckResult {
	_, result := c.check(ctx, execution)
	return result
}

func (c *staleWorkflowCheck) check(
	ctx context.Context,
	execution interface{},
) (deleteConcrete bool, result CheckResult) {

	concreteExecution, ok := execution.(*entity.ConcreteExecution)
	if !ok {
		return false, c.failed("expected concrete execution", "")
	}

	if concreteExecution.WorkflowID == "" ||
		concreteExecution.RunID == "" ||
		concreteExecution.DomainID == "" {
		return false, c.failed("missing critical data", "")
	}
	domainID := concreteExecution.GetDomainID()

	concreteWorkflow, err := c.pr.GetWorkflowExecution(ctx, &persistence.GetWorkflowExecutionRequest{
		DomainID: domainID,
		Execution: types.WorkflowExecution{
			WorkflowID: concreteExecution.WorkflowID,
			RunID:      concreteExecution.RunID,
		},
	})
	if err != nil {
		return false, c.failed("failed to get concrete execution record", err.Error())
	}

	pastExpiration, checkresult := c.CheckAge(concreteWorkflow)
	if pastExpiration {
		if checkresult.CheckResultType == CheckResultTypeCorrupted {
			return true, checkresult // delete the concrete execution, it's out of retention
		}
		// else check the current record, as it may be causing issues for the concrete
	}
	return false, checkresult

	// TODO: ^ similar check for current record?  Bad-current can sometimes block a different concrete.
}

func (c *staleWorkflowCheck) Fix(ctx context.Context, execution interface{}) FixResult {
	// essentially checkBeforeFix
	deleteConcrete, checkResult := c.check(ctx, execution)
	if checkResult.CheckResultType == CheckResultTypeHealthy {
		return FixResult{
			FixResultType: FixResultTypeSkipped,
			InvariantName: c.Name(),
			Info:          fmt.Sprintf("no need to fix: %s", checkResult.Info),
			InfoDetails:   checkResult.InfoDetails,
		}
	}

	if checkResult.CheckResultType != CheckResultTypeCorrupted {
		return FixResult{
			FixResultType: FixResultTypeFailed,
			InvariantName: c.Name(),
			Info:          fmt.Sprintf("unable to determine if beyond expiration: %s", checkResult.Info),
			InfoDetails:   checkResult.InfoDetails,
		}
	}

	if deleteConcrete {
		fixResult := DeleteExecution(ctx, execution, c.pr, c.dc)
		fixResult.CheckResult = checkResult
		fixResult.InvariantName = c.Name()
		return *fixResult
	}

	// TODO: handle current deletion?
	// probably not possible to reach currently because current records are not handled in check.
	return FixResult{
		FixResultType: FixResultTypeFailed,
		InvariantName: c.Name(),
		Info:          fmt.Sprintf("current record deletion not yet built: %v", checkResult.Info),
		InfoDetails:   checkResult.InfoDetails,
	}
}

func (c *staleWorkflowCheck) Name() Name {
	return StaleWorkflow
}

func (c *staleWorkflowCheck) CheckAge(workflow *persistence.GetWorkflowExecutionResponse) (pastExpiration bool, result CheckResult) {
	info := workflow.State.ExecutionInfo
	retentionNum, domainName, err := c.getDomainInfo(info)
	if err != nil {
		return false, c.failed(
			"unable to get domain retention",
			"domain: %v, name: %v, err: %v", info.DomainID, domainName, err)
	}
	if retentionNum <= 0 {
		return false, c.failed(
			"non-positive retention days in domain",
			"domain: %v, name: %v", info.DomainID, domainName)
	}
	if retentionNum > int32(maxRetentionDays) {
		return false, c.failed(
			"irrationally-large retention days in domain",
			"domain: %v, name: %v, days: %v, max: %v", info.DomainID, domainName, retentionNum, maxRetentionDays)
	}

	retention := time.Hour * 24 * time.Duration(retentionNum)
	// add a safety buffer of 10 days
	safety := time.Hour * 24 * 10
	maxLifespan := retention + safety

	logStaleWorkflow := func(state string, expected time.Time) {
		c.log.Info("scanner found stale "+state+" workflow",
			zap.String("wid", info.WorkflowID),
			zap.String("rid", info.RunID),
			zap.String("domain_name", domainName),
			zap.String("domain_id", info.DomainID),
			zap.String("started_at", info.StartTimestamp.Format(dateOnly)),
			zap.String("expected_cleanup_at", expected.Format(dateOnly)),
			zap.String("state", state), // may simplify log processing
		)
	}

	// treat "running" (has had decision tasks) and "created" (no decisions, e.g. cron / delay / lost initial tasks)
	// as essentially the same thing.  either way there's no close event to check.
	if info.State == persistence.WorkflowStateRunning || info.State == persistence.WorkflowStateCreated {
		stale, expected, result := c.checkRunningAge(workflow, maxLifespan, domainName)
		if stale {
			// we know these exist, but we don't expect them to be numerous "recently".
			// log for verification.
			logStaleWorkflow("running", expected)
		}
		return stale, result
	} else if info.State == persistence.WorkflowStateZombie {
		// TODO: what exactly is zombie?  seems like probably "current but no concrete" but I'm surprised this is in the state.
		//
		// https://github.com/uber/cadence/issues/1800
		// "The ability to create workflow with zombie status allows replication stack to backfill finished workflow execution sent by remote, while not affecting local running workflow"
		// so these are just replication of completed workflows?  why is this a special state?
		stale, expected, result := c.checkZombieAge(workflow, maxLifespan, domainName)
		if stale {
			// we know these exist, but we don't have a good feel for the distribution
			logStaleWorkflow("zombie", expected)
		}
		return stale, result
	} else if info.State == persistence.WorkflowStateCompleted {
		stale, expected, result := c.checkClosedAge(workflow, maxLifespan, domainName)
		if stale {
			// we know these exist, but we don't have a good feel for the distribution
			logStaleWorkflow("closed", expected)
		}
		return stale, result
	}

	// currently one of: corrupted, created, void
	// but if more are added in the future, this is still the safe option.
	return false, c.failed(
		"unhandled workflow state",
		"info.State value: %d", info.State)
}

func (c *staleWorkflowCheck) checkRunningAge(workflow *persistence.GetWorkflowExecutionResponse, maxLifespan time.Duration, domainName string) (pastExpiration bool, expected time.Time, result CheckResult) {
	// workflow-expiration timer is calculated in GenerateWorkflowStartTasks, mimic it here for safety: https://github.com/uber/cadence/blob/master/service/history/execution/mutable_state_task_generator.go#L140-L163
	/*
		// running workflows might contain critical information in their first history record, so it must be retrieved.
		info := workflow.State.ExecutionInfo

		duration := time.Duration(info.WorkflowTimeout) * time.Second
		// First-decision-backoff is only in first event.  It must be included because the expiration time does not include it.
		// I really wish it wasn't built this way, this makes it much much harder to scan in bulk.
		timeout := info.StartTimestamp.Add(duration + firstdecisionbackoff)

		// ensure that the first attempt does not time out early based on retry policy timeout
		if attr.Attempt > 0 && !executionInfo.ExpirationTime.IsZero() && workflowTimeoutTimestamp.After(executionInfo.ExpirationTime) {
			workflowTimeoutTimestamp = executionInfo.ExpirationTime
		}
	*/
	first, ok, result := c.firstEvent(workflow, domainName)
	if !ok {
		// completely missing history should be caught by the history-exists check, no need to handle here
		return false, expected, result
	}
	startedAt := workflow.State.ExecutionInfo.StartTimestamp                             // nonzero
	timeout := time.Second * time.Duration(workflow.State.ExecutionInfo.WorkflowTimeout) // nonzero
	backoff := time.Second * time.Duration(first.GetFirstDecisionTaskBackoffSeconds())   // may be zero

	// comment in GenerateWorkflowStartTasks seems backwards, it clamps to the minimum of timeout and retry-expiration,
	// which means we should be able to ignore it and be *safe* if perhaps not *optimal*.
	//
	// completed workflows seem to have this set, but perhaps not running.
	// which can happen here since this method is reused for closed-but-corrupt workflows.
	// since I'm not confident on what this field's value is / if it's trustworthy, ignore for now.
	//
	// expiration := workflow.State.ExecutionInfo.ExpirationTime
	// if !expiration.IsZero() {
	// 	fmt.Println("non-zero expiration time, not sure what this means")
	// }

	timesOutAt := startedAt.Add(timeout).Add(backoff)
	cleansUpAt := timesOutAt.Add(maxLifespan)
	// sanity checks on calculated times
	if ok, result := c.checkTimeInSaneRange(timesOutAt, "workflow timeout"); !ok {
		return false, expected, result
	}
	if ok, result := c.checkTimeInSaneRange(cleansUpAt, "workflow cleanup time"); !ok {
		return false, expected, result
	}

	if cleansUpAt.Before(time.Now()) {
		return true, cleansUpAt, CheckResult{
			CheckResultType: CheckResultTypeCorrupted,
			InvariantName:   c.Name(),
			Info:            "stale running workflow",
			InfoDetails:     fmt.Sprintf("running workflow should have timed out by %v and been cleaned up by %v, but it still exists", timesOutAt.Format(dateOnly), cleansUpAt.Format(dateOnly)),
		}
	}
	return false, cleansUpAt, CheckResult{
		CheckResultType: CheckResultTypeHealthy,
		InvariantName:   c.Name(),
		Info:            "running workflow is within expiration window",
	}
}

// intended as defensive programming, in case values are in some wrong scale or some other surprise occurs.
//
// these should not trigger in practice, and are intended to catch bugs or logical flaws while developing or changing
// this fixer, or flawed/missing knowledge - if they occur, that's likely a scenario we should be handling explicitly.
func (c *staleWorkflowCheck) checkTimeInSaneRange(t time.Time, kind string) (ok bool, result CheckResult) {
	if t.IsZero() {
		// will also be before-impossibly-old, but separated for clarity purposes
		return false, c.failed(fmt.Sprintf("calculated %v is zero, failing", kind), "")
	}
	if t.Before(impossiblyOld) || t.After(impossiblyFuture) {
		// something screwed up, time is outside sane bounds
		return false, c.failed(
			fmt.Sprintf("calculated %v seems insane, failing", kind),
			fmt.Sprintf("expected %v to be within range: %q < actual %q < %q", kind, impossiblyOld, t, impossiblyFuture))
	}
	return true, result
}

func (c *staleWorkflowCheck) checkClosedAge(workflow *persistence.GetWorkflowExecutionResponse, maxLifespan time.Duration, domainName string) (pastExpiration bool, expected time.Time, result CheckResult) {
	closed, ok, result := c.closeEventTime(workflow, domainName)
	if !ok {
		// error of some kind, see if we can consider it stale from just the start info.
		// this is somewhat common if we are missing the final event in a workflow, e.g. due to broken replication or partial (un)deletion.
		// running retention will never be shorter than closed retention, so this is always safe.
		runningStale, expected, runningResult := c.checkRunningAge(workflow, maxLifespan, domainName)
		// rewrite to mention closed, but interpreted as running.
		// the reason for failure is still relevant, but this way at least we know it was from the fallback.
		return runningStale, expected, CheckResult{
			CheckResultType: runningResult.CheckResultType,
			InvariantName:   runningResult.InvariantName,
			Info:            strings.Replace(runningResult.Info, "running workflow", "completed workflow (err, treating as running)", 1) + ". closed info: " + result.Info,
			InfoDetails:     strings.Replace(runningResult.InfoDetails, "running workflow", "completed workflow (err, treating as running)", 1) + ". closed info details: " + result.InfoDetails,
		}
	}
	if ok, result := c.checkTimeInSaneRange(closed, "workflow close"); !ok {
		// bad data, zero value, or perhaps a number-scale issue.  either way untrustworthy.
		return false, expected, result
	}
	expected = closed.Add(maxLifespan)
	if expected.Before(time.Now()) {
		// >10 days beyond retention, should be deleted
		return true, expected, CheckResult{
			CheckResultType: CheckResultTypeCorrupted,
			InvariantName:   c.Name(),
			Info:            "completed workflow exists beyond retention window",
			InfoDetails:     fmt.Sprintf("completed workflow exists beyond retention window, should have disappeared by %v", closed.Add(maxLifespan).Format(dateOnly)),
		}
	}

	// still in retention, it's fine
	return false, expected, CheckResult{
		CheckResultType: CheckResultTypeHealthy,
		InvariantName:   c.Name(),
		Info:            "completed workflow still within retention + 10-day buffer",
		InfoDetails:     fmt.Sprintf("completed workflow still within retention + 10-day buffer, closed %v and allowed to exist until %v", closed, closed.Add(maxLifespan).Format(dateOnly)),
	}

}

func (c *staleWorkflowCheck) checkZombieAge(workflow *persistence.GetWorkflowExecutionResponse, maxLifespan time.Duration, domainName string) (pastExpiration bool, expected time.Time, result CheckResult) {
	// zombies may or may not have history, and may or may not be "running" depending on if enough history has replicated.
	// they are not ever "current" though apparently.  despite there being a current execution record pointing to them.  wut.
	/*
		func (e *mutableStateBuilder) IsCurrentWorkflowGuaranteed() bool {
			// stateInDB is used like a bloom filter:
			//
			// 1. stateInDB being created / running meaning that this workflow must be the current
			//  workflow (assuming there is no rebuild of mutable state).
			// 2. stateInDB being completed does not guarantee this workflow being the current workflow
			// 3. stateInDB being zombie guarantees this workflow not being the current workflow
			// 4. stateInDB cannot be void, void is only possible when mutable state is just initialized
	*/

	// AFAICT zombie workflows are both not running and not completely replicated, i.e. there is no completion info or final completion event.
	// so just check them as running, at least until we learn otherwise.
	//
	// if there are completed ones, just check completed-result if running-age fails.
	// the cleanup time is just the minimum of the two (should always be completed if it exists)
	cleanup, expected, result := c.checkRunningAge(workflow, maxLifespan, domainName)
	// just reword it
	result.Info = strings.Replace(result.Info, "running workflow", "zombie workflow", 1)
	result.InfoDetails = strings.Replace(result.InfoDetails, "running workflow", "zombie workflow", 1)
	return cleanup, expected, result
}

// it's just quite verbose
func (c *staleWorkflowCheck) failed(info string, details string, args ...interface{}) CheckResult {
	if details != "" {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   c.Name(),
			Info:            info,
			InfoDetails:     fmt.Sprintf(details, args...),
		}
	}
	return CheckResult{
		CheckResultType: CheckResultTypeFailed,
		InvariantName:   c.Name(),
		Info:            info,
	}
}

func (c *staleWorkflowCheck) firstEvent(workflow *persistence.GetWorkflowExecutionResponse, domainName string) (attrs types.WorkflowExecutionStartedEventAttributes, ok bool, result CheckResult) {
	// see: func (e *mutableStateBuilder) GetStartEvent(
	/*
		currentBranchToken, err := e.GetCurrentBranchToken()
		if err != nil {
			return nil, err
		}

		startEvent, err := e.eventsCache.GetEvent(
			ctx,
			e.shard.GetShardID(),
			e.executionInfo.DomainID,
			e.executionInfo.WorkflowID,
			e.executionInfo.RunID,
			common.FirstEventID,
			common.FirstEventID,
			currentBranchToken,
		)
	*/
	branchToken, ok, result := c.getBranchToken(workflow)
	if !ok {
		return attrs, false, result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shard := c.pr.GetShardID()
	history, err := c.pr.ReadHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{
		BranchToken: branchToken,
		MinEventID:  common.FirstEventID,
		MaxEventID:  common.FirstEventID + 1, // exclusive bound
		ShardID:     &shard,
		PageSize:    1, // just 1 necessary
		DomainName:  domainName,
	})
	if err != nil {
		return attrs, false, c.failed(
			"could not read first event history",
			"read branch error for shard: %v, token: %s, err: %v", c.pr.GetShardID(), branchToken, err)
	}
	if len(history.HistoryEvents) < 1 {
		return attrs, false, c.failed(
			"incomplete history, missing first event?",
			"got: %v", history.HistoryEvents)
	}
	first := history.HistoryEvents[0]
	if first.WorkflowExecutionStartedEventAttributes == nil {
		return attrs, false, c.failed("missing start event attributes", "got: %v", first)
	}
	return *first.WorkflowExecutionStartedEventAttributes, true, result
}

func (c *staleWorkflowCheck) getBranchToken(workflow *persistence.GetWorkflowExecutionResponse) (token []byte, ok bool, result CheckResult) {
	/*
		func (e *mutableStateBuilder) GetCurrentBranchToken() ([]byte, error) {
			if e.versionHistories != nil {
				currentVersionHistory, err := e.versionHistories.GetCurrentVersionHistory()
				if err != nil {
					return nil, err
				}
				return currentVersionHistory.GetBranchToken(), nil
			}
			return e.executionInfo.BranchToken, nil
		}
	*/
	if workflow.State.VersionHistories == nil {
		// should imply no history data, should be impossible?
		return nil, false, c.failed("no version histories", "")
	}
	currentVersionHistory, err := workflow.State.VersionHistories.GetCurrentVersionHistory()
	if err != nil {
		// should be impossible?
		return nil, false, c.failed("unable to get current version history", "err: %v", err)
	}
	currentBranchToken := currentVersionHistory.GetBranchToken()
	if len(currentBranchToken) == 0 {
		// should be impossible?
		return nil, false, c.failed("no current branch token", "")
	}
	return currentBranchToken, true, result
}

func (c *staleWorkflowCheck) closeEventTime(workflow *persistence.GetWorkflowExecutionResponse, domainName string) (when time.Time, ok bool, result CheckResult) {
	// completion event is (sometimes?) nil, pull it from history instead.

	// strangely, close-event-time is not part of the workflow execution response.
	// pull the final event in history to figure it out.
	branchToken, ok, result := c.getBranchToken(workflow)
	if !ok {
		return when, false, result
	}
	last, err := c.getLastEvent(branchToken, domainName)
	if err != nil {
		return when, false, c.failed("failed to get last event", err.Error())
	}
	if last == nil {
		return when, false, c.failed("empty last event, should be impossible", "") // successful totally-empty history seems impossible
	}

	// make extra sure it's a completion event.
	// all history events have a timestamp, but it needs to be a final-state event, not a random one.
	if !anyPresent(
		last.WorkflowExecutionCanceledEventAttributes,
		last.WorkflowExecutionCompletedEventAttributes,
		last.WorkflowExecutionContinuedAsNewEventAttributes,
		last.WorkflowExecutionFailedEventAttributes,
		last.WorkflowExecutionTerminatedEventAttributes,
		last.WorkflowExecutionTimedOutEventAttributes,
	) {
		// completion event may simply not exist due to incomplete replication.
		// this may be handled by history_exists or broken-history invariants, and we don't need to handle it here.
		return when, false, c.failed("last event is not a completion-type", "missing data? got: %v", last)

	}

	if last.Timestamp == nil {
		return when, false, c.failed("last event has a nil timestamp", "bad data? got: %v", last)
	}
	// sanity check, so far I have not seen any completion events in the top-level state.
	// I suspect it's only an old / cache field, but bail if it disagrees, just in case.
	if workflow.State.ExecutionInfo.CompletionEvent != nil &&
		workflow.State.ExecutionInfo.CompletionEvent.Timestamp != nil &&
		*last.Timestamp != *workflow.State.ExecutionInfo.CompletionEvent.Timestamp {
		return when, false, c.failed(
			"workflow execution info and final event disagree",
			"bad data? got execution-completion-event timestamp: %v and final history event timestamp: %v",
			time.Unix(0, *last.Timestamp), time.Unix(0, *workflow.State.ExecutionInfo.CompletionEvent.Timestamp))
	}
	ts := time.Unix(0, *last.Timestamp)
	if ok, result := c.checkTimeInSaneRange(ts, "last event timestamp"); !ok {
		return when, false, result
	}
	return ts, true, result
}

func anyPresent(items ...interface{}) bool {
	for _, i := range items {
		// cannot use `i == nil` because typed nil pointers are boxed into non-nil interface{} due to having a type
		if !reflect.ValueOf(i).IsNil() {
			return true
		}
	}
	return false
}

func (c *staleWorkflowCheck) getLastEvent(branchToken []byte, domainName string) (*types.HistoryEvent, error) {
	const (
		maxHistoryLen = 250000 // our internal limits are much smaller, but it would be fine to raise this
		pageSize      = 1000   // multiple 1000/page limits elsewhere, also fine to change
		batchTimeout  = 5 * time.Second
	)
	shard := c.pr.GetShardID()
	iter := 0
	var nextPageToken []byte
	var lastEvent *types.HistoryEvent
	for ; iter < (maxHistoryLen / pageSize); iter++ { // loose sanity check
		ctx, cancel := context.WithTimeout(context.Background(), batchTimeout)
		defer cancel() // revive:disable-line:defer clean up on panics, dups are just noops

		history, err := c.pr.ReadHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{
			BranchToken: branchToken,
			// only interested in the last event, but we have to read in order to get there.
			MinEventID:    common.FirstEventID,
			MaxEventID:    common.EndEventID,
			ShardID:       &shard,
			PageSize:      pageSize,
			DomainName:    domainName,
			NextPageToken: nextPageToken,
		})
		cancel() // handle non-panics
		if err != nil {
			return nil, err
		}

		if len(history.HistoryEvents) > 0 {
			// should only be false if it didn't notice end-of-history in the previous batch (not sure if this happens or not)
			lastEvent = history.HistoryEvents[len(history.HistoryEvents)-1]
		}
		if len(history.NextPageToken) == 0 {
			// received last batch
			return lastEvent, nil
		}
		nextPageToken = history.NextPageToken
	}

	// should be unreachable, assuming our limits work
	return nil, fmt.Errorf("exceeded max history requests (%v), failing for branch token: %s", iter, branchToken)
}

func (c *staleWorkflowCheck) getDomainInfo(info *persistence.WorkflowExecutionInfo) (retention int32, name string, err error) {
	// domain cache entries have private fields, hence this testable method is necessary to avoid using them

	domain, err := c.dc.GetDomainByID(info.DomainID)
	if err != nil {
		return -1, "", err
	}
	retentionNum := domain.GetRetentionDays(info.WorkflowID) // takes retention-sampling into account
	return retentionNum, domain.GetInfo().Name, nil
}
