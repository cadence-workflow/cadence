// Copyright (c) 2026 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/yarpc/yarpcerrors"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
	"github.com/uber/cadence/service/worker/scheduler"
)

const (
	scheduleWorkflowIDPrefix          = "cadence-scheduler:"
	schedulerWorkflowExecutionTimeout = 10 * 365 * 24 * time.Hour // ~10 years
	schedulerWorkflowDecisionTimeout  = 10 * time.Second
	defaultListSchedulesPageSize      = 10

	// describeScheduleRetryAttempts bounds describeSchedulerWorkflow, which retries the
	// DescribeWorkflowExecution probe and the describe query together as one pass. The
	// transient states it rides out — CloseStatus=CONTINUED_AS_NEW during the
	// executionCache invalidation window, a query rejected with CONTINUED_AS_NEW, and a
	// run whose first decision task has not completed — typically resolve within
	// milliseconds but can take a few seconds in loaded environments. A single budget
	// keeps the DescribeSchedule worst case (~4s of backoff plus per-attempt RPC time)
	// within the typical 10s service SLA regardless of which step is slow to settle.
	describeScheduleRetryAttempts = 5
	describeScheduleRetryInterval = 1 * time.Second
)

var (
	// errSchedulerContinuedAsNew marks the transient window in which the scheduler
	// workflow has closed its previous run for ContinueAsNew and the new run is not
	// yet visible. Both the DescribeWorkflowExecution probe and the describe query
	// return it. A scheduler still in that state once the retry budget is spent is
	// no longer plausibly mid-transition, so normalizeScheduleError reports it as an
	// InternalServiceError naming the stuck state rather than asking the client to
	// keep retrying a wait the server has already performed.
	errSchedulerContinuedAsNew = errors.New("scheduler mid-ContinueAsNew")
)

// newDescribeScheduleRetryPolicy returns a fixed-interval policy allowing at most
// describeScheduleRetryAttempts calls of the operation. RetryPolicy counts retries
// rather than attempts, hence the -1.
func newDescribeScheduleRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(describeScheduleRetryInterval)
	policy.SetBackoffCoefficient(1)
	policy.SetMaximumInterval(describeScheduleRetryInterval)
	policy.SetExpirationInterval(backoff.NoInterval)
	policy.SetMaximumAttempts(describeScheduleRetryAttempts - 1)
	return policy
}

func isContinuedAsNew(closeStatus *types.WorkflowExecutionCloseStatus) bool {
	return closeStatus != nil && *closeStatus == types.WorkflowExecutionCloseStatusContinuedAsNew
}

func scheduleWorkflowID(scheduleID string) string {
	return scheduleWorkflowIDPrefix + scheduleID
}

func validateSchedulePolicies(policies *types.SchedulePolicies) error {
	if policies == nil {
		return nil
	}
	if policies.OverlapPolicy == types.ScheduleOverlapPolicySkipNew &&
		policies.CatchUpPolicy == types.ScheduleCatchUpPolicyAll {
		return &types.BadRequestError{
			Message: "SKIP_NEW overlap policy with CATCH_UP_ALL catch-up policy is invalid: " +
				"caught-up fires would be immediately skipped due to overlap with the previous run.",
		}
	}
	return nil
}

// validateScheduleSpecTimeRange rejects a spec whose EndTime is not after its
// StartTime when both are set. A zero StartTime or EndTime means "unbounded" and
// is left unchecked. Mirrors the range validation BackfillSchedule performs, and
// prevents creating a schedule that can never fire.
func validateScheduleSpecTimeRange(spec *types.ScheduleSpec) error {
	if spec == nil {
		return nil
	}
	start := spec.GetStartTime()
	end := spec.GetEndTime()
	if start.IsZero() || end.IsZero() {
		return nil
	}
	if !end.After(start) {
		return &types.BadRequestError{Message: "Spec.EndTime must be after Spec.StartTime."}
	}
	return nil
}

// warnIfBufferLimitExceedsSystemLimit logs a warning when buffer_limit exceeds
// MaxBufferedFiresSystemLimit. The value is accepted (the policy still queues
// up to the system limit), but drops at that cap will be tagged
// reason=system_limit rather than reason=user_limit.
func (wh *WorkflowHandler) warnIfBufferLimitExceedsSystemLimit(scheduleID, domainName string, policies *types.SchedulePolicies) {
	if policies == nil ||
		policies.OverlapPolicy != types.ScheduleOverlapPolicyBuffer ||
		int(policies.BufferLimit) <= scheduler.MaxBufferedFiresSystemLimit {
		return
	}
	wh.GetLogger().Warn(
		"buffer_limit exceeds scheduler system limit; drops will be attributed to system_limit",
		tag.WorkflowDomainName(domainName),
		tag.WorkflowID(scheduleWorkflowID(scheduleID)),
		tag.Dynamic("bufferLimit", int(policies.BufferLimit)),
		tag.Dynamic("systemLimit", scheduler.MaxBufferedFiresSystemLimit),
	)
}

// validateUserSearchAttributes rejects user search attribute keys that collide
// with scheduler-reserved keys (CadenceSchedule* prefix). Without this check,
// user values would be silently overwritten by the scheduler workflow's
// UpsertSearchAttributes calls on start and on state change.
func validateUserSearchAttributes(sa *types.SearchAttributes) error {
	if sa == nil {
		return nil
	}
	for k := range sa.IndexedFields {
		if strings.HasPrefix(k, "CadenceSchedule") {
			return &types.BadRequestError{
				Message: fmt.Sprintf("search attribute key %q is reserved for internal use", k),
			}
		}
	}
	return nil
}

// ongoingBackfillsForResponse converts the scheduler workflow's snapshot of
// pending backfills into the BackfillInfo pointer slice carried in
// DescribeScheduleResponse. Returns nil (not an empty slice) when there are
// no backfills so the marshalled response omits the field.
func ongoingBackfillsForResponse(in []types.BackfillInfo) []*types.BackfillInfo {
	if len(in) == 0 {
		return nil
	}
	out := make([]*types.BackfillInfo, 0, len(in))
	for i := range in {
		bf := in[i]
		out = append(out, &bf)
	}
	return out
}

func (wh *WorkflowHandler) CreateSchedule(
	ctx context.Context,
	request *types.CreateScheduleRequest,
) (*types.CreateScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}
	if request.GetSpec() == nil {
		return nil, &types.BadRequestError{Message: "Spec is not set on request."}
	}
	if request.GetSpec().GetCronExpression() == "" {
		return nil, &types.BadRequestError{Message: "CronExpression is not set on request."}
	}
	if _, err := backoff.ValidateSchedule(request.GetSpec().GetCronExpression()); err != nil {
		return nil, err
	}
	if err := validateScheduleSpecTimeRange(request.GetSpec()); err != nil {
		return nil, err
	}
	if request.GetAction() == nil || request.GetAction().GetStartWorkflow() == nil {
		return nil, &types.BadRequestError{Message: "Action.StartWorkflow is not set on request."}
	}
	if err := common.ValidateRetryPolicy(request.GetAction().GetStartWorkflow().GetRetryPolicy()); err != nil {
		return nil, err
	}
	if err := validateSchedulePolicies(request.GetPolicies()); err != nil {
		return nil, err
	}
	wh.warnIfBufferLimitExceedsSystemLimit(scheduleID, domainName, request.GetPolicies())
	if err := validateUserSearchAttributes(request.GetSearchAttributes()); err != nil {
		return nil, err
	}

	workflowInput := scheduler.SchedulerWorkflowInput{
		Domain:           domainName,
		ScheduleID:       scheduleID,
		Spec:             *request.GetSpec(),
		Action:           *request.GetAction(),
		SearchAttributes: request.GetSearchAttributes(),
		Memo:             request.GetMemo(),
	}
	if request.GetPolicies() != nil {
		workflowInput.Policies = *request.GetPolicies()
	}
	if s := request.GetState(); s != nil && s.Paused {
		workflowInput.State.Paused = true
		if info := s.GetPauseInfo(); info != nil {
			workflowInput.State.PauseReason = info.Reason
			workflowInput.State.PausedBy = info.PausedBy
		}
	}

	inputBytes, err := json.Marshal(workflowInput)
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to serialize scheduler workflow input: %v", err)}
	}

	wfID := scheduleWorkflowID(scheduleID)
	requestID := uuid.New().String()
	// AllowDuplicate lets a re-created schedule start a new scheduler workflow even when the
	// previous (deleted) run is still within the retention window. A currently-running
	// scheduler still causes WorkflowExecutionAlreadyStartedError, which the block below
	// converts into the user-visible "schedule already exists" error.
	reusePolicy := types.WorkflowIDReusePolicyAllowDuplicate
	executionTimeout := int32(schedulerWorkflowExecutionTimeout.Seconds())
	decisionTimeout := int32(schedulerWorkflowDecisionTimeout.Seconds())

	_, err = wh.StartWorkflowExecution(ctx, &types.StartWorkflowExecutionRequest{
		Domain:                              domainName,
		WorkflowID:                          wfID,
		WorkflowType:                        &types.WorkflowType{Name: scheduler.WorkflowTypeName},
		TaskList:                            &types.TaskList{Name: scheduler.TaskListName},
		Input:                               inputBytes,
		ExecutionStartToCloseTimeoutSeconds: &executionTimeout,
		TaskStartToCloseTimeoutSeconds:      &decisionTimeout,
		RequestID:                           requestID,
		WorkflowIDReusePolicy:               &reusePolicy,
		Memo:                                request.GetMemo(),
		SearchAttributes:                    request.GetSearchAttributes(),
	})
	if err != nil {
		var alreadyStarted *types.WorkflowExecutionAlreadyStartedError
		if errors.As(err, &alreadyStarted) {
			return nil, &types.BadRequestError{
				Message: fmt.Sprintf("schedule %q already exists in domain %q", scheduleID, domainName),
			}
		}
		return nil, err
	}

	return &types.CreateScheduleResponse{ScheduleID: scheduleID}, nil
}

func (wh *WorkflowHandler) DescribeSchedule(
	ctx context.Context,
	request *types.DescribeScheduleRequest,
) (*types.DescribeScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}

	wfID := scheduleWorkflowID(scheduleID)
	execution := &types.WorkflowExecution{WorkflowID: wfID}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}

	var desc *scheduler.ScheduleDescription
	op := func(ctx context.Context) error {
		var err error
		desc, err = wh.describeSchedulerWorkflowOnce(ctx, domainID, domainName, scheduleID, execution)
		return err
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(newDescribeScheduleRetryPolicy()),
		backoff.WithRetryableError(isRetryableDescribeScheduleError(ctx)),
		backoff.WithClock(wh.GetTimeSource()),
	)
	if err := throttleRetry.Do(ctx, op); err != nil {
		return nil, normalizeScheduleError(err, scheduleID, domainName)
	}

	return &types.DescribeScheduleResponse{
		Spec:     &desc.Spec,
		Action:   &desc.Action,
		Policies: &desc.Policies,
		State: &types.ScheduleState{
			Paused: desc.Paused,
			PauseInfo: func() *types.SchedulePauseInfo {
				if !desc.Paused {
					return nil
				}
				return &types.SchedulePauseInfo{
					Reason:   desc.PauseReason,
					PausedBy: desc.PausedBy,
					PausedAt: desc.PausedAt,
				}
			}(),
		},
		Info: &types.ScheduleInfo{
			LastRunTime:          desc.LastRunTime,
			NextRunTime:          desc.NextRunTime,
			TotalRuns:            desc.TotalRuns,
			MissedRuns:           desc.MissedRuns,
			SkippedRuns:          desc.SkippedRuns,
			BufferedFireCount:    desc.BufferedFireCount,
			RunningWorkflowCount: desc.RunningWorkflowCount,
			CreateTime:           desc.CreateTime,
			LastUpdateTime:       desc.LastUpdateTime,
			OngoingBackfills:     ongoingBackfillsForResponse(desc.OngoingBackfills),
		},
		Memo:             desc.Memo,
		SearchAttributes: desc.SearchAttributes,
	}, nil
}

func (wh *WorkflowHandler) UpdateSchedule(
	ctx context.Context,
	request *types.UpdateScheduleRequest,
) (*types.UpdateScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}
	if request.GetSpec() == nil && request.GetAction() == nil && request.GetPolicies() == nil && request.GetSearchAttributes() == nil {
		return nil, &types.BadRequestError{Message: "At least one of Spec, Action, Policies, or SearchAttributes must be set on request."}
	}
	if err := validateSchedulePolicies(request.GetPolicies()); err != nil {
		return nil, err
	}
	wh.warnIfBufferLimitExceedsSystemLimit(scheduleID, domainName, request.GetPolicies())
	if spec := request.GetSpec(); spec != nil && spec.GetCronExpression() != "" {
		if _, err := backoff.ValidateSchedule(spec.GetCronExpression()); err != nil {
			return nil, err
		}
	}
	if err := validateScheduleSpecTimeRange(request.GetSpec()); err != nil {
		return nil, err
	}
	if action := request.GetAction(); action != nil && action.GetStartWorkflow() != nil {
		if err := common.ValidateRetryPolicy(action.GetStartWorkflow().GetRetryPolicy()); err != nil {
			return nil, err
		}
	}
	if err := validateUserSearchAttributes(request.GetSearchAttributes()); err != nil {
		return nil, err
	}

	signal := scheduler.UpdateSignal{
		Spec:             request.GetSpec(),
		Action:           request.GetAction(),
		Policies:         request.GetPolicies(),
		SearchAttributes: request.GetSearchAttributes(),
	}

	if err := wh.signalScheduleWorkflow(ctx, domainName, scheduleID, scheduler.SignalNameUpdate, signal); err != nil {
		return nil, err
	}
	return &types.UpdateScheduleResponse{}, nil
}

func (wh *WorkflowHandler) DeleteSchedule(
	ctx context.Context,
	request *types.DeleteScheduleRequest,
) (*types.DeleteScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}

	signalErr := wh.signalScheduleWorkflow(ctx, domainName, scheduleID, scheduler.SignalNameDelete, nil)
	if signalErr == nil || isScheduleAlreadyGone(signalErr) {
		return &types.DeleteScheduleResponse{}, nil
	}

	if shouldSkipTerminateOnDeleteScheduleSignalFailure(signalErr) {
		return nil, signalErr
	}

	// Non-transient signal failure: fall back to terminating the scheduler workflow.
	// If SignalWorkflowExecution returned nil, the signal was accepted by history but
	// may still never run in the worker; that case is outside this fallback.
	wh.GetLogger().Info("DeleteSchedule: signal failed, falling back to terminate",
		tag.WorkflowDomainName(domainName),
		tag.WorkflowID(scheduleWorkflowID(scheduleID)),
		tag.Error(signalErr))

	terminateErr := wh.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain: domainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: scheduleWorkflowID(scheduleID),
		},
		Reason: "terminated by DeleteSchedule",
	})
	if terminateErr == nil || isScheduleAlreadyGone(terminateErr) {
		return &types.DeleteScheduleResponse{}, nil
	}

	wh.GetLogger().Error("DeleteSchedule: both signal and terminate failed",
		tag.WorkflowDomainName(domainName),
		tag.WorkflowID(scheduleWorkflowID(scheduleID)),
		tag.Error(signalErr),
		tag.Dynamic("terminateError", terminateErr))

	// Return the original signal error since signal is the primary path and the
	// one a caller would normally retry against.
	return nil, signalErr
}

// isScheduleAlreadyGone reports whether an error from the scheduler workflow
// signal or terminate path indicates that the underlying workflow no longer
// exists or is already closed. Such errors satisfy the idempotent contract of
// DeleteSchedule and are returned to the caller as success.
func isScheduleAlreadyGone(err error) bool {
	return errors.As(err, new(*types.EntityNotExistsError)) ||
		errors.As(err, new(*types.WorkflowExecutionAlreadyCompletedError))
}

// shouldSkipTerminateOnDeleteScheduleSignalFailure is true for errors where the
// client should retry DeleteSchedule without terminating the scheduler workflow:
// transient service/transport failures (common.FrontendRetry,
// common.IsContextTimeoutError) and workflow-ID rate limiting, which is surfaced
// as ServiceBusy but is not classified as retryable by FrontendRetry.
func shouldSkipTerminateOnDeleteScheduleSignalFailure(err error) bool {
	if common.FrontendRetry(err) || common.IsContextTimeoutError(err) {
		return true
	}
	var sb *types.ServiceBusyError
	return errors.As(err, &sb) && sb.Reason == constants.WorkflowIDRateLimitReason
}

func (wh *WorkflowHandler) PauseSchedule(
	ctx context.Context,
	request *types.PauseScheduleRequest,
) (*types.PauseScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}

	signal := scheduler.PauseSignal{
		Reason:   request.GetReason(),
		PausedBy: request.GetIdentity(),
	}

	if err := wh.signalScheduleWorkflow(ctx, domainName, scheduleID, scheduler.SignalNamePause, signal); err != nil {
		return nil, err
	}
	return &types.PauseScheduleResponse{}, nil
}

func (wh *WorkflowHandler) UnpauseSchedule(
	ctx context.Context,
	request *types.UnpauseScheduleRequest,
) (*types.UnpauseScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}

	signal := scheduler.UnpauseSignal{
		Reason:        request.GetReason(),
		CatchUpPolicy: request.GetCatchUpPolicy(),
	}

	if err := wh.signalScheduleWorkflow(ctx, domainName, scheduleID, scheduler.SignalNameUnpause, signal); err != nil {
		return nil, err
	}
	return &types.UnpauseScheduleResponse{}, nil
}

func (wh *WorkflowHandler) BackfillSchedule(
	ctx context.Context,
	request *types.BackfillScheduleRequest,
) (*types.BackfillScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}
	if request.GetStartTime().IsZero() {
		return nil, &types.BadRequestError{Message: "StartTime is not set on request."}
	}
	if request.GetEndTime().IsZero() {
		return nil, &types.BadRequestError{Message: "EndTime is not set on request."}
	}
	if !request.GetEndTime().After(request.GetStartTime()) {
		return nil, &types.BadRequestError{Message: "EndTime must be after StartTime."}
	}

	signal := scheduler.BackfillSignal{
		StartTime:     request.GetStartTime(),
		EndTime:       request.GetEndTime(),
		OverlapPolicy: request.GetOverlapPolicy(),
		BackfillID:    resolveBackfillID(request.GetBackfillID()),
	}

	if err := wh.signalScheduleWorkflow(ctx, domainName, scheduleID, scheduler.SignalNameBackfill, signal); err != nil {
		return nil, err
	}
	return &types.BackfillScheduleResponse{}, nil
}

func resolveBackfillID(clientID string) string {
	if id := strings.TrimSpace(clientID); id != "" {
		return id
	}
	return uuid.New().String()
}

func (wh *WorkflowHandler) ListSchedules(
	ctx context.Context,
	request *types.ListSchedulesRequest,
) (*types.ListSchedulesResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}

	pageSize := request.GetPageSize()
	if pageSize <= 0 {
		pageSize = defaultListSchedulesPageSize
	}

	// NOTE: schedule read handlers call visibility via the embedded WorkflowHandler
	// methods below (not via the frontend client), so cluster redirection middleware
	// is skipped. For global (XDC) domains, the passive region may return stale
	// visibility data; that applies to Describe and List schedules until XDC support.
	var executions []*types.WorkflowExecutionInfo
	var nextPageToken []byte
	var err error

	if wh.config.DisableListVisibilityByFilter(domainName) {
		// When filtered list visibility APIs are disabled, only list-by-query remains
		// (advanced visibility: ES / Pinot / SQL with custom query support).
		listResp, e := wh.ListWorkflowExecutions(ctx, &types.ListWorkflowExecutionsRequest{
			Domain:        domainName,
			PageSize:      pageSize,
			NextPageToken: request.GetNextPageToken(),
			Query:         fmt.Sprintf("WorkflowType = '%s' and CloseTime = missing", scheduler.WorkflowTypeName),
		})
		err = e
		if listResp != nil {
			executions = listResp.Executions
			nextPageToken = listResp.NextPageToken
		}
	} else {
		// Default path: list open workflows by scheduler workflow type. This uses
		// ListOpenWorkflowExecutionsByType and works on basic Cassandra/SQL visibility
		// stores that do not support ListWorkflowExecutions custom queries (which
		// previously made ListSchedules look empty in those deployments).
		nowNanos := wh.GetTimeSource().Now().UnixNano()
		openResp, e := wh.ListOpenWorkflowExecutions(ctx, &types.ListOpenWorkflowExecutionsRequest{
			Domain:          domainName,
			MaximumPageSize: pageSize,
			NextPageToken:   request.GetNextPageToken(),
			StartTimeFilter: &types.StartTimeFilter{
				EarliestTime: common.Int64Ptr(0),
				LatestTime:   common.Int64Ptr(nowNanos),
			},
			TypeFilter: &types.WorkflowTypeFilter{Name: scheduler.WorkflowTypeName},
		})
		err = e
		if openResp != nil {
			executions = openResp.Executions
			nextPageToken = openResp.NextPageToken
		}
	}
	if err != nil {
		return nil, err
	}

	entries := buildScheduleListEntriesFromExecutions(wh, domainName, executions)

	return &types.ListSchedulesResponse{
		Schedules:     entries,
		NextPageToken: nextPageToken,
	}, nil
}

func buildScheduleListEntriesFromExecutions(wh *WorkflowHandler, domainName string, executions []*types.WorkflowExecutionInfo) []*types.ScheduleListEntry {
	logger := wh.GetLogger()
	entries := make([]*types.ScheduleListEntry, 0, len(executions))
	for _, exec := range executions {
		wfID := exec.GetExecution().GetWorkflowID()
		if !strings.HasPrefix(wfID, scheduleWorkflowIDPrefix) {
			logger.Warn("skipping visibility row without schedule workflow id prefix",
				tag.WorkflowDomainName(domainName),
				tag.WorkflowID(wfID),
			)
			continue
		}
		scheduleID := strings.TrimPrefix(wfID, scheduleWorkflowIDPrefix)

		entry := &types.ScheduleListEntry{
			ScheduleID: scheduleID,
		}

		// Default to not paused. The scheduler workflow upserts these search
		// attributes on start (and ContinueAsNew) and on state change, so a missing
		// value means the workflow hasn't run its first decision task yet (brief
		// window after CreateSchedule).
		paused := false
		var cronExpr, workflowTypeName string
		if exec.SearchAttributes != nil {
			idx := exec.SearchAttributes.IndexedFields
			if stateBytes, ok := idx[scheduler.SearchAttrScheduleState]; ok {
				var stateStr string
				if err := json.Unmarshal(stateBytes, &stateStr); err != nil {
					logger.Warn("failed to unmarshal CadenceScheduleState search attribute, defaulting to active",
						tag.WorkflowID(scheduleWorkflowID(scheduleID)),
						tag.Error(err),
					)
				} else {
					paused = stateStr == scheduler.ScheduleStatePaused
				}
			}
			if cronBytes, ok := idx[scheduler.SearchAttrScheduleCron]; ok {
				if err := json.Unmarshal(cronBytes, &cronExpr); err != nil {
					logger.Warn("failed to unmarshal CadenceScheduleCron search attribute, defaulting to empty",
						tag.WorkflowID(scheduleWorkflowID(scheduleID)),
						tag.Error(err),
					)
				}
			}
			if typeBytes, ok := idx[scheduler.SearchAttrScheduleWorkflowType]; ok {
				if err := json.Unmarshal(typeBytes, &workflowTypeName); err != nil {
					logger.Warn("failed to unmarshal CadenceScheduleWorkflowType search attribute, defaulting to empty",
						tag.WorkflowID(scheduleWorkflowID(scheduleID)),
						tag.Error(err),
					)
				}
			}
		}
		entry.State = &types.ScheduleState{Paused: paused}
		entry.CronExpression = cronExpr
		if workflowTypeName != "" {
			entry.WorkflowType = &types.WorkflowType{Name: workflowTypeName}
		}

		entries = append(entries, entry)
	}
	return entries
}

func (wh *WorkflowHandler) signalScheduleWorkflow(
	ctx context.Context,
	domainName string,
	scheduleID string,
	signalName string,
	signalInput interface{},
) error {
	var inputBytes []byte
	var err error
	if signalInput != nil {
		inputBytes, err = json.Marshal(signalInput)
		if err != nil {
			return &types.InternalServiceError{Message: fmt.Sprintf("failed to serialize signal input: %v", err)}
		}
	}

	wfID := scheduleWorkflowID(scheduleID)
	err = wh.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain: domainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: wfID,
		},
		SignalName: signalName,
		Input:      inputBytes,
		RequestID:  uuid.New().String(),
	})
	if err != nil {
		return normalizeScheduleError(err, scheduleID, domainName)
	}
	return nil
}

func normalizeScheduleError(err error, scheduleID, domainName string) error {
	if errors.Is(err, errSchedulerContinuedAsNew) {
		return &types.InternalServiceError{
			Message: fmt.Sprintf(
				"schedule %q in domain %q is not queryable: scheduler workflow still mid-ContinueAsNew after %d attempts",
				scheduleID, domainName, describeScheduleRetryAttempts,
			),
		}
	}
	var notFound *types.EntityNotExistsError
	if errors.As(err, &notFound) {
		return &types.EntityNotExistsError{
			Message: fmt.Sprintf("schedule %q not found in domain %q", scheduleID, domainName),
		}
	}
	return err
}

// describeSchedulerWorkflowOnce performs a single probe-then-query pass.
//
// Schedulers ContinueAsNew on every UpdateSchedule and periodically to bound
// history. During the executionCache invalidation window on the history host,
// getMutableState can briefly resolve wfID-without-runID to the just-closed old
// run and expose CloseStatus=CONTINUED_AS_NEW for an otherwise healthy schedule.
// The probe distinguishes that transient race from a genuinely closed scheduler
// (FAILED, TERMINATED, TIMED_OUT, COMPLETED, CANCELED), which must not be
// reported as ACTIVE and is returned as a non-retryable error.
//
// The query keeps QueryRejectConditionNotCompletedCleanly as a safety net for the
// window where the scheduler closes between the two RPCs.
func (wh *WorkflowHandler) describeSchedulerWorkflowOnce(
	ctx context.Context,
	domainID, domainName, scheduleID string,
	execution *types.WorkflowExecution,
) (*scheduler.ScheduleDescription, error) {
	info, err := wh.describeSchedulerExecution(ctx, domainID, domainName, execution)
	if err != nil {
		return nil, err
	}
	if info.CloseStatus != nil {
		return nil, scheduleNotOperationalError(scheduleID, domainName, info.CloseStatus.String())
	}

	queryResp, err := wh.querySchedulerWorkflow(ctx, domainName, execution)
	if err != nil {
		return nil, err
	}
	if queryResp == nil {
		return nil, &types.InternalServiceError{Message: "nil query response from scheduler workflow"}
	}
	if queryResp.QueryRejected != nil {
		closeStatus := "unknown"
		if queryResp.QueryRejected.CloseStatus != nil {
			closeStatus = queryResp.QueryRejected.CloseStatus.String()
		}
		return nil, scheduleNotOperationalError(scheduleID, domainName, closeStatus)
	}
	if queryResp.GetQueryResult() == nil {
		return nil, &types.InternalServiceError{Message: "empty query result from scheduler workflow"}
	}

	var desc scheduler.ScheduleDescription
	if err := json.Unmarshal(queryResp.GetQueryResult(), &desc); err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to deserialize scheduler describe response: %v", err)}
	}
	return &desc, nil
}

func scheduleNotOperationalError(scheduleID, domainName, closeStatus string) error {
	return &types.InternalServiceError{
		Message: fmt.Sprintf(
			"schedule %q in domain %q is not operational: scheduler workflow ended with status %s",
			scheduleID, domainName, closeStatus,
		),
	}
}

// describeSchedulerExecution returns the latest run's WorkflowExecutionInfo for
// the scheduler workflow, reporting errSchedulerContinuedAsNew while the close
// status is CONTINUED_AS_NEW so the caller can retry the whole pass.
//
// Calls the history client directly so this internal probe does not emit
// FrontendDescribeWorkflowExecution metrics.
func (wh *WorkflowHandler) describeSchedulerExecution(
	ctx context.Context,
	domainID, domainName string,
	execution *types.WorkflowExecution,
) (*types.WorkflowExecutionInfo, error) {
	resp, err := wh.GetHistoryClient().DescribeWorkflowExecution(ctx, &types.HistoryDescribeWorkflowExecutionRequest{
		DomainUUID: domainID,
		Request: &types.DescribeWorkflowExecutionRequest{
			Domain:    domainName,
			Execution: execution,
		},
	})
	if err != nil {
		return nil, err
	}
	info := resp.GetWorkflowExecutionInfo()
	if info == nil {
		return nil, &types.InternalServiceError{Message: "nil workflow execution info from scheduler workflow"}
	}
	if isContinuedAsNew(info.CloseStatus) {
		return nil, errSchedulerContinuedAsNew
	}
	return info, nil
}

// querySchedulerWorkflow issues a QueryWorkflow call for the scheduler's
// describe query.
func (wh *WorkflowHandler) querySchedulerWorkflow(
	ctx context.Context,
	domainName string,
	execution *types.WorkflowExecution,
) (*types.QueryWorkflowResponse, error) {
	rejectCondition := types.QueryRejectConditionNotCompletedCleanly
	resp, err := wh.QueryWorkflow(ctx, &types.QueryWorkflowRequest{
		Domain:               domainName,
		Execution:            execution,
		Query:                &types.WorkflowQuery{QueryType: scheduler.QueryTypeDescribe},
		QueryRejectCondition: &rejectCondition,
	})
	if err != nil {
		return nil, err
	}
	// A rejected query is a successful RPC, so the ContinueAsNew race has to be
	// turned into an error to be retried.
	if rejected := resp.GetQueryRejected(); rejected != nil && isContinuedAsNew(rejected.CloseStatus) {
		return nil, errSchedulerContinuedAsNew
	}
	return resp, nil
}

// isRetryableDescribeScheduleError reports the transient conditions a describe
// pass rides out: the scheduler being mid-ContinueAsNew, a run whose first
// decision task has not completed yet, and history's internal query wait timing
// out while the caller's own deadline is still live. Everything else — including
// a genuinely closed scheduler — fails the pass immediately.
func isRetryableDescribeScheduleError(ctx context.Context) backoff.IsRetryable {
	return func(err error) bool {
		if errors.Is(err, errSchedulerContinuedAsNew) {
			return true
		}
		var qfe *types.QueryFailedError
		if errors.As(err, &qfe) && strings.Contains(qfe.Message, "decision task") {
			return true
		}
		queryTimeout := errors.Is(err, context.DeadlineExceeded) ||
			yarpcerrors.IsStatus(err) && yarpcerrors.FromError(err).Code() == yarpcerrors.CodeDeadlineExceeded
		return queryTimeout && ctx.Err() == nil
	}
}
