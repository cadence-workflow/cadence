// Copyright (c) 2021 Uber Technologies, Inc.
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

package scanner

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"

	c "github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/reconciliation/fetcher"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/resource"
)

func init() {
	workflow.RegisterWithOptions(
		CheckDataCorruptionWorkflow,
		workflow.RegisterOptions{Name: reconciliation.CheckDataCorruptionWorkflowType},
	)
	activity.Register(ExecutionFixerActivity)
	activity.Register(EmitResultMetricsActivity)
}

const (
	workflowTimer        = 5 * time.Minute
	maxSignalNumber      = 1000
	fixerActivityTimeout = time.Minute
)

// CheckDataCorruptionWorkflow is invoked by remote cluster via signals
func CheckDataCorruptionWorkflow(ctx workflow.Context, fixList []entity.Execution) error {

	logger := workflow.GetLogger(ctx)
	signalCh := workflow.GetSignalChannel(ctx, reconciliation.CheckDataCorruptionWorkflowSignalName)
	signalCount := 0
	maxReceivedSignalNumber := workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		return maxSignalNumber
	})

	for {
		selector := workflow.NewSelector(ctx)
		// timer
		timerCtx, timerCancel := workflow.WithCancel(ctx)
		waitTimer := workflow.NewTimer(timerCtx, workflowTimer)
		selector.AddFuture(waitTimer, func(f workflow.Future) {
			// do nothing. Unblock the selector
		})

		// signal
		selector.AddReceive(signalCh, func(c workflow.Channel, more bool) {
			var fixExecution entity.Execution
			for c.ReceiveAsync(&fixExecution) {
				signalCount++
				fixList = append(fixList, fixExecution)
			}
		})

		selector.Select(ctx)
		if len(fixList) == 0 {
			return nil
		}
		timerCancel()

		timeout := fixerActivityTimeout * time.Duration(len(fixList))
		activityOptions = workflow.ActivityOptions{
			ScheduleToStartTimeout: time.Minute,
			StartToCloseTimeout:    timeout,
			HeartbeatTimeout:       fixerActivityTimeout,
			RetryPolicy:            &activityRetryPolicy,
		}
		activityCtx := workflow.WithActivityOptions(ctx, activityOptions)
		var fixResults []invariant.FixResult
		err := workflow.ExecuteActivity(activityCtx, ExecutionFixerActivity, fixList).Get(activityCtx, &fixResults)
		if err != nil {
			logger.Error("failed to run execution fixer activity", zap.Error(err))
			return err
		}

		activityOptions = workflow.ActivityOptions{
			ScheduleToStartTimeout: time.Minute,
			StartToCloseTimeout:    time.Minute,
		}
		activityCtx = workflow.WithActivityOptions(ctx, activityOptions)
		err = workflow.ExecuteActivity(activityCtx, EmitResultMetricsActivity, fixResults).Get(activityCtx, nil)
		if err != nil {
			logger.Error("failed to run execution fixer activity", zap.Error(err))
			return err
		}

		fixList = []entity.Execution{}
		workflow.GetMetricsScope(ctx)
		var maxSignalCount int
		if err := maxReceivedSignalNumber.Get(&maxSignalCount); err != nil {
			logger.Error("failed to get max supported signal number")
			return err
		}

		if signalCount > maxSignalCount {
			var fixExecution entity.Execution
			for signalCh.ReceiveAsync(&fixExecution) {
				fixList = append(fixList, fixExecution)
			}
			return workflow.NewContinueAsNewError(ctx, reconciliation.CheckDataCorruptionWorkflowType, fixList)
		}
	}
}

func ExecutionFixerActivity(ctx context.Context, fixList []entity.Execution) ([]invariant.FixResult, error) {
	var result []invariant.FixResult
	index := 0
	if activity.HasHeartbeatDetails(ctx) {
		activity.GetHeartbeatDetails(ctx, &index)
	}

	for index < len(fixList) {
		execution := fixList[index]
		pr, cache, err := getDefaultDAO(ctx, execution.ShardID)
		if err != nil {
			return nil, err
		}
		request := fetcher.ExecutionRequest{
			DomainID:   execution.DomainID,
			WorkflowID: execution.WorkflowID,
			RunID:      execution.RunID,
		}
		concreteExecution, err := fetcher.ConcreteExecution(ctx, pr, request)
		if err != nil {
			return nil, err
		}

		currentExecutionInvariant := invariant.NewOpenCurrentExecution(pr, cache)
		fixResult := currentExecutionInvariant.Fix(ctx, concreteExecution)
		result = append(result, fixResult)
		historyInvariant := invariant.NewHistoryExists(pr, cache)
		fixResult = historyInvariant.Fix(ctx, concreteExecution)
		result = append(result, fixResult)
		activity.RecordHeartbeat(ctx, index)
		index++
	}
	return result, nil
}

func getDefaultDAO(
	ctx context.Context,
	shardID int,
) (persistence.Retryer, cache.DomainCache, error) {
	sc, err := getScannerContext(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot find key %v in context", reconciliation.CheckDataCorruptionWorkflowType)
	}
	res := sc.resource
	cache := res.GetDomainCache()
	execManager, err := res.GetExecutionManager(shardID)
	if err != nil {
		return nil, nil, err
	}
	pr := persistence.NewPersistenceRetryer(execManager, res.GetHistoryManager(), c.CreatePersistenceRetryPolicy())
	return pr, cache, nil
}

func EmitResultMetricsActivity(ctx context.Context, fixResults []invariant.FixResult) error {
	sc, err := getScannerContext(ctx)
	if err != nil {
		return fmt.Errorf("cannot find key %v in context", reconciliation.CheckDataCorruptionWorkflowType)
	}
	res := sc.resource

	for _, result := range fixResults {
		scope := res.GetMetricsClient().Scope(
			metrics.CheckDataCorruptionWorkflowScope,
			metrics.InvariantTypeTag(string(result.InvariantName)))

		scope.IncCounter(metrics.DataCorruptionWorkflowCount)
		switch result.FixResultType {
		case invariant.FixResultTypeFailed:
			scope.IncCounter(metrics.DataCorruptionWorkflowFailure)
		case invariant.FixResultTypeFixed:
			scope.IncCounter(metrics.DataCorruptionWorkflowSuccessCount)
		case invariant.FixResultTypeSkipped:
			scope.IncCounter(metrics.DataCorruptionWorkflowSkipCount)
		}
	}
	return nil
}

func NewDataCorruptionWorkflowWorker(
	resource resource.Resource,
	params *BootstrapParams,
) *Scanner {

	zapLogger, err := zap.NewProduction()
	if err != nil {
		resource.GetLogger().Fatal("failed to initialize zap logger", tag.Error(err))
	}
	return &Scanner{
		context: scannerContext{
			resource: resource,
		},
		tallyScope: params.TallyScope,
		zapLogger:  zapLogger.Named("data-corruption-workflow"),
	}

}

func (s *Scanner) StartDataCorruptionWorkflowWorker() error {
	ctx := context.WithValue(context.Background(), contextKey(reconciliation.CheckDataCorruptionWorkflowType), s.context)
	workerOpts := worker.Options{
		Logger:                                 s.zapLogger,
		MetricsScope:                           s.tallyScope,
		MaxConcurrentActivityExecutionSize:     maxConcurrentActivityExecutionSize,
		MaxConcurrentDecisionTaskExecutionSize: maxConcurrentDecisionTaskExecutionSize,
		BackgroundActivityContext:              ctx,
	}

	err := worker.New(
		s.context.resource.GetSDKClient(),
		constants.SystemLocalDomainName,
		reconciliation.CheckDataCorruptionWorkflowTaskList,
		workerOpts,
	).Start()
	return err
}
