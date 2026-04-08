// Copyright (c) 2017-2021 Uber Technologies Inc.

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

package signal

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"go.uber.org/cadence"
	"go.uber.org/cadence/.gen/go/shared"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/client"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/bench/lib"
	"github.com/uber/cadence/bench/load/common"
	c "github.com/uber/cadence/common"
)

const (
	// TestName is the test name
	TestName = "signal"

	// LauncherWorkflowName is the workflow name for launching load test
	LauncherWorkflowName = "signal-load-test-workflow"

	loadTestActivityName      = "loadTestActivity"
	verifyResultActivityName  = "verifyResultActivity"
	processSignalWorkflowName = "processSignalWorkflow"
	stressTestSignalName      = "stressSignalName"
	processSignalActivityName = "processSignalActivity"
	exitSignalNumber          = -1
	failedSignalWorkflowQuery = "WorkflowType='%v' and CloseStatus=1 and StartTime > %v and CloseTime < %v"
)

var (
	maxPageSize = int32(10)
)

// RegisterLauncher registers workflows and activities for load launching
func RegisterLauncher(w worker.Worker) {
	w.RegisterWorkflowWithOptions(loadTestWorkflow, workflow.RegisterOptions{Name: LauncherWorkflowName})
	w.RegisterActivityWithOptions(loadTestActivity, activity.RegisterOptions{Name: loadTestActivityName})
	w.RegisterActivityWithOptions(verifyResultActivity, activity.RegisterOptions{Name: verifyResultActivityName})
}

// RegisterWorker registers workflows and activities for sync api load
func RegisterWorker(w worker.Worker) {
	w.RegisterWorkflowWithOptions(processSignalWorkflow, workflow.RegisterOptions{Name: processSignalWorkflowName})
	w.RegisterActivityWithOptions(processSignalActivity, activity.RegisterOptions{Name: processSignalActivityName})
}

type (
	loadTestActivityParams struct {
		WorkflowExecutionTimeoutInSeconds int
		DecisionTaskTimeoutInSeconds      int
		CampaignCount                     int
		ActionRate                        float64
		FailureRate                       float64
		StartingWorkflowID                int
		BatchWorkflowCount                int
		SignalCount                       int
		SignalDataSize                    int
		RateLimit                         int
		SignalCountBeforeContinueAsNew    int
		EnableRollingWindow               bool
		MaxSignalDelayInSeconds           int
		MaxSignalDelayCount               int
	}

	signalActivityResult struct {
		SucceedCount int
		FailureCount int
	}

	signalEvent struct {
		SignalNumber int
		Data         []byte
		Timestamp    int64
	}

	verifyActivityParams struct {
		FailedWorkflowCount int64
		WorkflowStartTime   int64
	}
)

// loadTestWorkflow sends signals to the stress workflow
func loadTestWorkflow(ctx workflow.Context, params lib.SignalTestConfig) error {
	info := workflow.GetInfo(ctx)
	startTime := workflow.Now(ctx)
	expiration := time.Duration(info.ExecutionStartToCloseTimeoutSeconds) * time.Second
	profile, err := lib.BeginWorkflow(ctx, LauncherWorkflowName, startTime.UnixNano())
	if err != nil {
		return err
	}

	retryPolicy := &cadence.RetryPolicy{
		InitialInterval:          time.Second * 5,
		BackoffCoefficient:       1, // always backoff 5s
		MaximumInterval:          time.Second * 5,
		ExpirationInterval:       expiration,
		MaximumAttempts:          2000,
		NonRetriableErrorReasons: []string{common.ErrReasonValidationFailed},
	}
	ao := workflow.ActivityOptions{
		ScheduleToStartTimeout: expiration,
		StartToCloseTimeout:    expiration,
		HeartbeatTimeout:       time.Second * 30,
		RetryPolicy:            retryPolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	batchSize := params.LoadTestWorkflowCount / params.LoaderCount
	doneCh := workflow.NewChannel(ctx)
	for i := 0; i < params.LoaderCount; i++ {
		loaderParams := loadTestActivityParams{
			StartingWorkflowID:                i * batchSize,
			BatchWorkflowCount:                batchSize,
			WorkflowExecutionTimeoutInSeconds: params.WorkflowExecutionTimeoutInSeconds,
			DecisionTaskTimeoutInSeconds:      params.DecisionTaskTimeoutInSeconds,
			CampaignCount:                     params.CampaignCount,
			ActionRate:                        params.ActionRate,
			FailureRate:                       params.FailureRate,
			RateLimit:                         params.RateLimit,
			SignalCount:                       params.SignalCount,
			SignalDataSize:                    params.SignalDataSize,
			SignalCountBeforeContinueAsNew:    params.SignalBeforeContinueAsNew,
			EnableRollingWindow:               params.EnableRollingWindow,
			MaxSignalDelayInSeconds:           params.MaxSignalDelayInSeconds,
			MaxSignalDelayCount:               params.MaxSignalDelayCount,
		}

		workflow.Go(ctx, func(ctx workflow.Context) {
			var activityResult signalActivityResult
			err := workflow.ExecuteActivity(ctx, loadTestActivityName, loaderParams).Get(ctx, &activityResult)
			if err != nil {
				workflow.GetLogger(ctx).Info("signal LoadTestActivity failed", zap.Error(err))
			} else {
				workflow.GetLogger(ctx).Info("signal LoadTestActivity completed")
			}

			doneCh.Send(ctx, "done")
		})
	}

	for i := 0; i < params.LoaderCount; i++ {
		doneCh.Receive(ctx, nil)
	}

	if err := workflow.Sleep(ctx, time.Minute*5); err != nil {
		profile.End(err)
		return err
	}

	actInput := verifyActivityParams{
		FailedWorkflowCount: int64(float64(params.LoadTestWorkflowCount) * params.FailureThreshold),
		WorkflowStartTime:   startTime.UnixNano(),
	}
	err = workflow.ExecuteActivity(ctx, verifyResultActivityName, actInput).Get(ctx, nil)
	return profile.End(err)
}

func loadTestActivity(ctx context.Context, params loadTestActivityParams) (signalActivityResult, error) {
	info := activity.GetInfo(ctx)
	logger := activity.GetLogger(ctx)
	cc := ctx.Value(lib.CtxKeyCadenceClient).(lib.CadenceClient)
	rc := ctx.Value(lib.CtxKeyRuntimeContext).(*lib.RuntimeContext)
	numTaskList := rc.Bench.NumTaskLists
	loaderID := info.WorkflowExecution.ID
	limiter := rate.NewLimiter(rate.Limit(params.RateLimit), 1)

	workflowOptions := client.StartWorkflowOptions{
		ExecutionStartToCloseTimeout:    time.Second * time.Duration(params.WorkflowExecutionTimeoutInSeconds),
		DecisionTaskStartToCloseTimeout: time.Second * time.Duration(params.DecisionTaskTimeoutInSeconds),
		WorkflowIDReusePolicy:           client.WorkflowIDReusePolicyAllowDuplicate,
	}

	wfParams := lib.ProcessSignalWorkflowConfig{
		CampaignCount:             params.CampaignCount,
		ActionRate:                params.ActionRate,
		FailureRate:               params.FailureRate,
		SignalBeforeContinueAsNew: params.SignalCountBeforeContinueAsNew,
		MaxSignalDelayInSeconds:   params.MaxSignalDelayInSeconds,
		MaxSignalDelayCount:       params.MaxSignalDelayCount,
	}

	data := make([]byte, params.SignalDataSize)
	for i := 0; i < len(data); i++ {
		data[i] = 'a'
	}
	totalSignalCount := params.SignalCount * params.BatchWorkflowCount
	succeedCount, failedCount := 0, 0
	wfIDs := make(map[string]struct{})

	batchStartID := 0
	if activity.HasHeartbeatDetails(ctx) {
		var finishedID int
		if err := activity.GetHeartbeatDetails(ctx, &finishedID); err == nil {
			batchStartID = finishedID + 1
			logger.Info("recover from failed attempt", zap.Int("FinishedID", finishedID))
		}
	}

	logger.Info("start sending signals",
		zap.Int32("Attempt", info.Attempt),
		zap.Int("StartSignalCount", batchStartID),
		zap.Int("TotalSignalCount", totalSignalCount),
		zap.Int("BatchWorkflowCount", params.BatchWorkflowCount))

	wid := -1
	for i := batchStartID; i < totalSignalCount; i++ {
		randTaskListID := rand.Intn(numTaskList)
		workflowOptions.TaskList = common.GetTaskListName(randTaskListID)

		if wid == -1 || rand.Float64() > 0.1 {
			// 10% chance we are going to send signal to the same workflow as last time, this is to simulate multiple
			// signals to same workflow within short time.
			wid = params.StartingWorkflowID + int(rand.Int31n(int32(params.BatchWorkflowCount)))

			/*
				If we have large number of target workflow set:
					We want to simulate the real scenario where signals coming to a small set of users instead of randomly to any
					user in the entire user set. This set of active user is moving around the glob as day time moves. So we want to
					simulate here is that, we define a 1 hour window around current time, and only select random user from this
					window.
			*/
			if params.EnableRollingWindow {
				wid = params.StartingWorkflowID + getRandomID(params.BatchWorkflowCount, time.Now())
			}
		}

		workflowID := getStressWorkflowID(loaderID, wid)
		wfIDs[workflowID] = struct{}{}
		signal := signalEvent{SignalNumber: i, Data: data, Timestamp: time.Now().UnixNano()}
		if err := limiter.Wait(ctx); err != nil {
			if ctx.Err() != nil {
				return signalActivityResult{}, ctx.Err()
			}
			continue
		}
		wfParams.ScheduleTimeNano = time.Now().UnixNano()
		_, err := cc.SignalWithStartWorkflow(ctx, workflowID, stressTestSignalName, signal, workflowOptions, processSignalWorkflowName, wfParams)
		if err == nil {
			logger.Debug("SignalWithStartWorkflow succeed", zap.String("workflowID", workflowID))
			succeedCount++
		} else {
			logger.Error("SignalWithStartWorkflow failed", zap.Error(err))
			failedCount++
		}
		activity.RecordHeartbeat(ctx, i)
		if ctx.Err() != nil {
			return signalActivityResult{}, ctx.Err()
		}
	}

	// send last signal to notify the target workflow to exit
	exitStartID := 0
	if batchStartID > totalSignalCount {
		// this means previous attempt failed after sending all normal signals, and failed while sending exit signal.
		exitStartID = batchStartID - totalSignalCount
	}
	for i := exitStartID; i < params.BatchWorkflowCount; i++ {
		wid := params.StartingWorkflowID + i
		workflowID := getStressWorkflowID(loaderID, wid)
		signal := signalEvent{SignalNumber: -1, Data: data}
		if err := limiter.Wait(ctx); err != nil {
			if ctx.Err() != nil {
				return signalActivityResult{}, ctx.Err()
			}
			continue
		}
		err := cc.SignalWorkflow(context.Background(), workflowID, "", stressTestSignalName, signal)
		if err == nil {
			logger.Debug("SignalWorkflow succeed", zap.String("workflowID", workflowID))
			succeedCount++
		} else {
			logger.Error("SignalWorkflow failed", zap.Error(err))
			failedCount++
		}
		activity.RecordHeartbeat(ctx, totalSignalCount+i)
		if ctx.Err() != nil {
			return signalActivityResult{}, ctx.Err()
		}
	}

	return signalActivityResult{SucceedCount: succeedCount, FailureCount: failedCount}, nil
}

func verifyResultActivity(ctx context.Context, params verifyActivityParams) error {
	cc := ctx.Value(lib.CtxKeyCadenceClient).(lib.CadenceClient)
	info := activity.GetInfo(ctx)
	// step 1. verify if any open workflow
	listWorkflowRequest := &shared.ListOpenWorkflowExecutionsRequest{
		Domain:          c.StringPtr(info.WorkflowDomain),
		MaximumPageSize: &maxPageSize,
		StartTimeFilter: &shared.StartTimeFilter{
			EarliestTime: c.Int64Ptr(params.WorkflowStartTime),
			LatestTime:   c.Int64Ptr(time.Now().UnixNano()),
		},
		TypeFilter: &shared.WorkflowTypeFilter{
			Name: c.StringPtr(processSignalWorkflowName),
		},
	}
	openWorkflow, err := cc.ListOpenWorkflow(ctx, listWorkflowRequest)
	if err != nil {
		return err
	}
	if len(openWorkflow.Executions) > 0 {
		return cadence.NewCustomError(
			common.ErrReasonValidationFailed,
			"found open workflows after signal load test completed",
		)
	}

	// step 2: check failed workflow reason
	query := fmt.Sprintf(
		failedSignalWorkflowQuery,
		processSignalWorkflowName,
		params.WorkflowStartTime,
		time.Now().UnixNano())
	request := &shared.CountWorkflowExecutionsRequest{
		Domain: c.StringPtr(info.WorkflowDomain),
		Query:  &query,
	}
	resp, err := cc.CountWorkflow(ctx, request)
	if err != nil {
		return err
	}
	if resp.GetCount() > params.FailedWorkflowCount {
		return cadence.NewCustomError(
			common.ErrReasonValidationFailed,
			"found failed workflows after signal load test completed",
		)
	}
	return nil
}

func getStressWorkflowID(loaderID string, wid int) string {
	return fmt.Sprintf("%v-sync-stress-%v", loaderID, wid)
}

func processSignalWorkflow(ctx workflow.Context, params lib.ProcessSignalWorkflowConfig) error {
	logger := workflow.GetLogger(ctx)
	info := workflow.GetInfo(ctx)
	profile, err := lib.BeginWorkflow(ctx, processSignalWorkflowName, params.ScheduleTimeNano)
	if err != nil {
		return err
	}
	ch := workflow.GetSignalChannel(ctx, stressTestSignalName)
	totalSigCount := 0
	signalDelayCount := 0
main_loop:
	for {
		var signal signalEvent
		ch.Receive(ctx, &signal)
		totalSigCount++
		if signal.SignalNumber == exitSignalNumber {
			break main_loop
		}

		err := processSignal(ctx, signal, params)
		if err != nil {
			// log the error and continue
			logger.Error("sync api bench test process signal failed.")
		}

		batchSigCount := 1
		for ch.ReceiveAsync(&signal) {
			totalSigCount++
			batchSigCount++
			if signal.SignalNumber == exitSignalNumber {
				break main_loop
			}
			if workflow.Now(ctx).Sub(time.Unix(0, signal.Timestamp)) > time.Duration(params.MaxSignalDelayInSeconds)*time.Second {
				signalDelayCount++
				if signalDelayCount > params.MaxSignalDelayCount {
					return fmt.Errorf("received %v signals are longer than %v seconds", params.MaxSignalDelayCount, params.MaxSignalDelayInSeconds)
				}
			}

			err := processSignal(ctx, signal, params)
			if err != nil {
				// log the error and continue
				logger.Error("sync stress workflow process signal failed.")
			}

			if batchSigCount >= 5 {
				logger.Info("force sleep 1s", zap.String("WorkflowID", info.WorkflowExecution.ID), zap.String("RunID", info.WorkflowExecution.RunID))
				err := workflow.Sleep(ctx, time.Second)
				if err != nil {
					logger.Error("Failed sleep", zap.Error(err))
				}
				break // continue main_loop
			}
		}

		if totalSigCount >= params.SignalBeforeContinueAsNew {
			logger.Info("ContinueAsNew", zap.Int("ProcessedSignalCount", totalSigCount))
			profile.End(nil)
			params.ScheduleTimeNano = workflow.Now(ctx).UnixNano()
			return workflow.NewContinueAsNewError(ctx, processSignalWorkflowName, params)
		}
	}

	logger.Info("sync stress workflow completed")
	profile.End(nil)
	return nil
}

func processSignal(ctx workflow.Context, signal signalEvent, params lib.ProcessSignalWorkflowConfig) error {
	retryPolicy := &cadence.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 3,
		MaximumInterval:    time.Minute,
		ExpirationInterval: time.Minute * 5,
		MaximumAttempts:    5,
	}

	lao := workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: time.Second * 3,
		RetryPolicy:            retryPolicy,
	}
	ctx = workflow.WithLocalActivityOptions(ctx, lao)

	ao := workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Hour,
		StartToCloseTimeout:    time.Minute,
		RetryPolicy:            retryPolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var actionFutures []workflow.Future
	var trueConditions []string
	for i := 0; i < params.CampaignCount; i++ {
		var conditionMeet bool
		err := workflow.ExecuteLocalActivity(ctx, checkCondition, params.ActionRate, params.FailureRate).Get(ctx, &conditionMeet)
		if err != nil {
			return err
		}

		if conditionMeet {
			f := workflow.ExecuteActivity(ctx, processSignalActivityName, signal, params.FailureRate, workflow.Now(ctx).UnixNano())
			actionFutures = append(actionFutures, f)
			trueConditions = append(trueConditions, strconv.Itoa(i))
		}
	}

	for _, f := range actionFutures {
		var actionResult string
		err := f.Get(ctx, &actionResult)
		if err != nil {
			return err
		}
	}

	workflow.GetLogger(ctx).Info("Processed signal", zap.Int("SignalNumber", signal.SignalNumber), zap.Int("ActionCount", len(trueConditions)))
	return nil
}

// checkCondition is a local activity to check condition, it returns true if action needs to be taken
func checkCondition(
	ctx context.Context,
	actionRate float64,
	failureRate float64,
) (bool, error) {

	info := activity.GetInfo(ctx)
	logger := activity.GetLogger(ctx)
	if info.Attempt > 0 {
		failureRate += 0.1 * float64(info.Attempt) // increase failure rate for retry attempt
		logger.Info("Retry attempt", zap.Int32("Attempt", info.Attempt), zap.Float64("FailureRate", failureRate))
	}
	if rand.Float64() < failureRate {

		return false, errors.New("failed by rand")
	}
	return rand.Float64() < actionRate, nil
}

// processSignalActivity is a regular activity simulate actual action
func processSignalActivity(
	ctx context.Context,
	evt signalEvent,
	failureRate float64,
	scheduleTimeNano int64,
) (err error) {

	info := activity.GetInfo(ctx)
	logger := activity.GetLogger(ctx)
	svcConfig := common.GetActivityServiceConfig(ctx)
	scope := svcConfig.Metrics
	if scope == nil {
		panic("metrics client is not set")
	}
	scope, stopWatch := lib.RecordActivityStart(scope, processSignalActivityName, scheduleTimeNano)
	defer lib.RecordActivityEnd(scope, stopWatch, err)

	if info.Attempt > 0 {
		failureRate += 0.1 * float64(info.Attempt) // incrase failure rate for retry attempt
		logger.Info("Retry attempt", zap.Int32("Attempt", info.Attempt), zap.Float64("FailureRate", failureRate))
	}
	if rand.Float64() < failureRate {
		return errors.New("failed by rand")
	}

	return nil
}

var secondsInADay = 24 * 60 * 60

func getRandomID(totalCount int, now time.Time) int {
	r := rand.Float64()
	windowSize := totalCount / 24 // window size
	seconds := now.Second() + now.Minute()*60 + now.Hour()*3600

	beginOfWindowID := int64(seconds-1800) * int64(totalCount) / int64(secondsInADay)
	wid := int(beginOfWindowID + int64(r*float64(windowSize)))
	wid = (wid + totalCount) % totalCount

	return wid
}
