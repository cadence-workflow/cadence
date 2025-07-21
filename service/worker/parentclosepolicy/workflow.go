// Copyright (c) 2017 Uber Technologies, Inc.
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

package parentclosepolicy

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/pborman/uuid"
	"go.uber.org/cadence"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/encoded"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

type (
	contextKey string
)

const (
	processorContextKey contextKey = "processorContext"
	// processorTaskListName is the tasklist name
	processorTaskListName = "cadence-sys-processor-parent-close-policy"
	// processorWFTypeName is the workflow type
	processorWFTypeName   = "cadence-sys-parent-close-policy-workflow"
	processorActivityName = "cadence-sys-parent-close-policy-activity"
	infiniteDuration      = 20 * 365 * 24 * time.Hour
	processorChannelName  = "ParentClosePolicyProcessorChannelName"
)

type (
	// RequestDetail defines detail of each workflow to process
	RequestDetail struct {
		DomainID   string
		DomainName string
		WorkflowID string
		RunID      string
		Policy     types.ParentClosePolicy
	}

	// Request defines the request for parent close policy
	Request struct {
		ParentExecution types.WorkflowExecution
		Executions      []RequestDetail

		// DEPRECATED: the following field is deprecated since childworkflow
		// might in a different domain, use the DomainName field in RequestDetail
		DomainName string
	}
)

var (
	retryPolicy = cadence.RetryPolicy{
		InitialInterval:    10 * time.Second,
		BackoffCoefficient: 1.7,
		MaximumInterval:    5 * time.Minute,
		ExpirationInterval: infiniteDuration,
	}

	activityOptions = workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    5 * time.Minute,
		RetryPolicy:            &retryPolicy,
	}
)

func init() {
	workflow.RegisterWithOptions(ProcessorWorkflow, workflow.RegisterOptions{Name: processorWFTypeName})
	activity.RegisterWithOptions(ProcessorActivity, activity.RegisterOptions{Name: processorActivityName})
}

// ProcessorWorkflow is the workflow that performs actions for ParentClosePolicy
func ProcessorWorkflow(ctx workflow.Context) error {
	requestCh := workflow.GetSignalChannel(ctx, processorChannelName)
	for {
		var request Request
		if !requestCh.ReceiveAsync(&request) {
			// no more request
			break
		}

		opt := workflow.WithActivityOptions(ctx, activityOptions)
		_ = workflow.ExecuteActivity(opt, processorActivityName, request).Get(ctx, nil)
	}
	return nil
}

// ProcessorActivity is activity for processing batch operation
func ProcessorActivity(ctx context.Context, request Request) error {
	processor := ctx.Value(processorContextKey).(*Processor)
	domainCache := processor.domainCache
	historyClient := processor.clientBean.GetHistoryClient()
	logger := getActivityLogger(ctx)
	scope := processor.metricsClient.Scope(metrics.ParentClosePolicyProcessorScope)

	childWorkflowOnly := false
	if request.ParentExecution.WorkflowID != "" && request.ParentExecution.RunID != "" {
		// this is for backward compatibility
		// ideally we should always set childWorkflowOnly = true
		// however if ParentExecution is not specified, setting it to true
		// will cause terminate or cancel request to return mismatch error
		childWorkflowOnly = true
	}

	remoteExecutions := make(map[string][]RequestDetail)
	for _, execution := range request.Executions {
		domainName := execution.DomainName
		if domainName == "" {
			// for backward compatibility
			domainName = request.DomainName
		}

		var err error
		domainID := execution.DomainID
		if domainID == "" {
			// for backward compatibility
			domainID, err = domainCache.GetDomainID(domainName)
			if err != nil {
				if common.IsEntityNotExistsError(err) {
					scope.IncCounter(metrics.ParentClosePolicyProcessorSuccess)
					continue
				}
				scope.IncCounter(metrics.ParentClosePolicyProcessorFailures)
				logger.Error("Failed to process parent close policy", tag.Error(err))
				return err
			}
		}

		switch execution.Policy {
		case types.ParentClosePolicyAbandon:
			// no-op
			continue
		case types.ParentClosePolicyTerminate:
			terminateReq := &types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID: domainID,
				TerminateRequest: &types.TerminateWorkflowExecutionRequest{
					Domain: domainName,
					WorkflowExecution: &types.WorkflowExecution{
						WorkflowID: execution.WorkflowID,
					},
					Reason:              "by parent close policy",
					Identity:            processorWFTypeName,
					FirstExecutionRunID: execution.RunID,
				},
			}
			if childWorkflowOnly {
				terminateReq.ChildWorkflowOnly = true
				terminateReq.ExternalWorkflowExecution = &request.ParentExecution
			}
			err = historyClient.TerminateWorkflowExecution(ctx, terminateReq)
		case types.ParentClosePolicyRequestCancel:
			cancelReq := &types.HistoryRequestCancelWorkflowExecutionRequest{
				DomainUUID: domainID,
				CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
					Domain: domainName,
					WorkflowExecution: &types.WorkflowExecution{
						WorkflowID: execution.WorkflowID,
					},
					Identity:            processorWFTypeName,
					FirstExecutionRunID: execution.RunID,
				},
			}
			if childWorkflowOnly {
				cancelReq.ChildWorkflowOnly = true
				cancelReq.ExternalWorkflowExecution = &request.ParentExecution
			}
			err = historyClient.RequestCancelWorkflowExecution(ctx, cancelReq)
		default:
			err = fmt.Errorf("unknown parent close policy: %v", execution.Policy)
		}
		if err != nil {
			switch err.(type) {
			case *types.EntityNotExistsError,
				*types.WorkflowExecutionAlreadyCompletedError,
				*types.CancellationAlreadyRequestedError:
				err = nil
			case *types.DomainNotActiveError:
				var domainEntry *cache.DomainCacheEntry
				if domainEntry, err = domainCache.GetDomainByID(domainID); err == nil {
					cluster := domainEntry.GetReplicationConfig().ActiveClusterName
					remoteExecutions[cluster] = append(remoteExecutions[cluster], execution)
				}
			}
		}

		if err != nil {
			scope.IncCounter(metrics.ParentClosePolicyProcessorFailures)
			logger.Error("Failed to process parent close policy", tag.Error(err))
			return err
		}
		scope.IncCounter(metrics.ParentClosePolicyProcessorSuccess)
	}

	if err := signalRemoteCluster(
		ctx,
		processor.clientBean,
		request.ParentExecution,
		remoteExecutions,
		processor.numWorkflows,
	); err != nil {
		scope.IncCounter(metrics.ParentClosePolicyProcessorFailures)
		logger.Error("Failed to signal remote parent close policy workflow", tag.Error(err))
		return err
	}

	return nil
}

func signalRemoteCluster(
	ctx context.Context,
	clientBean client.Bean,
	parentExecution types.WorkflowExecution,
	remoteExecutions map[string][]RequestDetail,
	numWorkflows int,
) error {
	for cluster, executions := range remoteExecutions {
		remoteClient, err := clientBean.GetRemoteFrontendClient(cluster)
		if err != nil {
			return err
		}
		signalCtx, cancel := context.WithTimeout(ctx, signalTimeout)
		signalValue := Request{
			ParentExecution: parentExecution,
			Executions:      executions,
		}

		dc := encoded.GetDefaultDataConverter()
		signalInput, err := dc.ToData(signalValue)
		if err != nil {
			cancel()
			return err
		}

		_, err = remoteClient.SignalWithStartWorkflowExecution(signalCtx, &types.SignalWithStartWorkflowExecutionRequest{
			Domain:                              constants.SystemLocalDomainName,
			RequestID:                           uuid.New(),
			WorkflowID:                          fmt.Sprintf("%v-%v", workflowIDPrefix, rand.Intn(numWorkflows)),
			WorkflowType:                        &types.WorkflowType{Name: processorWFTypeName},
			TaskList:                            &types.TaskList{Name: processorTaskListName},
			Input:                               nil,
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(int32(infiniteDuration.Seconds())),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(60),
			Identity:                            "cadence-worker",
			WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
			SignalName:                          processorChannelName,
			SignalInput:                         signalInput,
		})
		cancel()

		if err != nil {
			return err
		}
	}

	return nil
}

func getActivityLogger(ctx context.Context) log.Logger {
	processor := ctx.Value(processorContextKey).(*Processor)
	wfInfo := activity.GetInfo(ctx)
	return processor.logger.WithTags(
		tag.WorkflowID(wfInfo.WorkflowExecution.ID),
		tag.WorkflowRunID(wfInfo.WorkflowExecution.RunID),
		tag.WorkflowDomainName(wfInfo.WorkflowDomain),
	)
}
