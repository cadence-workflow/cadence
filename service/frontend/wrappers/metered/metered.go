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

package metered

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/yarpc/yarpcerrors"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func (h *apiHandler) handleErr(err error, scope metrics.Scope, logger log.Logger) error {
	switch err := err.(type) {
	case *types.InternalServiceError:
		logger.Error("Internal service error", tag.Error(err))
		scope.IncCounter(metrics.CadenceFailures)
		return frontendInternalServiceError("cadence internal error, msg: %v", err.Message)
	case *types.BadRequestError:
		scope.IncCounter(metrics.CadenceErrBadRequestCounter)
		return err
	case *types.DomainNotActiveError:
		scope.IncCounter(metrics.CadenceErrBadRequestCounter)
		return err
	case *types.ServiceBusyError:
		scope.IncCounter(metrics.CadenceErrServiceBusyCounter)
		return err
	case *types.EntityNotExistsError:
		scope.IncCounter(metrics.CadenceErrEntityNotExistsCounter)
		return err
	case *types.WorkflowExecutionAlreadyCompletedError:
		scope.IncCounter(metrics.CadenceErrWorkflowExecutionAlreadyCompletedCounter)
		return err
	case *types.WorkflowExecutionAlreadyStartedError:
		scope.IncCounter(metrics.CadenceErrExecutionAlreadyStartedCounter)
		return err
	case *types.DomainAlreadyExistsError:
		scope.IncCounter(metrics.CadenceErrDomainAlreadyExistsCounter)
		return err
	case *types.CancellationAlreadyRequestedError:
		scope.IncCounter(metrics.CadenceErrCancellationAlreadyRequestedCounter)
		return err
	case *types.QueryFailedError:
		scope.IncCounter(metrics.CadenceErrQueryFailedCounter)
		return err
	case *types.LimitExceededError:
		scope.IncCounter(metrics.CadenceErrLimitExceededCounter)
		return err
	case *types.ClientVersionNotSupportedError:
		scope.IncCounter(metrics.CadenceErrClientVersionNotSupportedCounter)
		return err
	case *yarpcerrors.Status:
		if err.Code() == yarpcerrors.CodeDeadlineExceeded {
			logger.Error("Frontend request timedout", tag.Error(err))
			scope.IncCounter(metrics.CadenceErrContextTimeoutCounter)
			return err
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		logger.Error("Frontend request timedout", tag.Error(err))
		scope.IncCounter(metrics.CadenceErrContextTimeoutCounter)
		return err
	}
	logger.Error("Uncategorized error", tag.Error(err))
	scope.IncCounter(metrics.CadenceFailures)
	return frontendInternalServiceError("cadence internal uncategorized error, msg: %v", err.Error())
}

func (h *apiHandler) withSignalName(
	ctx context.Context,
	domainName string,
	signalName string,
) context.Context {
	if h.cfg.EmitSignalNameMetricsTag(domainName) {
		return metrics.TagContext(ctx, metrics.SignalNameTag(signalName))
	}
	return ctx
}

func frontendInternalServiceError(fmtStr string, args ...interface{}) error {
	// NOTE: For internal error, we can't return thrift error from cadence-frontend.
	// Because in uber internal metrics, thrift errors are counted as user errors.
	return fmt.Errorf(fmtStr, args...)
}

func toCountWorkflowExecutionsRequestTags(req *types.CountWorkflowExecutionsRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
	}
}

func toDescribeTaskListRequestTags(req *types.DescribeTaskListRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowTaskListName(req.GetTaskList().GetName()),
		tag.WorkflowTaskListType(int(req.GetTaskListType())),
		tag.WorkflowTaskListKind(int32(req.GetTaskList().GetKind())),
	}
}

func toDescribeWorkflowExecutionRequestTags(req *types.DescribeWorkflowExecutionRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetExecution().GetRunID()),
	}
}

func toDiagnoseWorkflowExecutionRequestTags(req *types.DiagnoseWorkflowExecutionRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetWorkflowExecution().GetRunID()),
	}
}

func toGetTaskListsByDomainRequestTags(req *types.GetTaskListsByDomainRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
	}
}

func toGetWorkflowExecutionHistoryRequestTags(req *types.GetWorkflowExecutionHistoryRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetExecution().GetRunID()),
	}
}

func toListArchivedWorkflowExecutionsRequestTags(req *types.ListArchivedWorkflowExecutionsRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
	}
}

func toListClosedWorkflowExecutionsRequestTags(req *types.ListClosedWorkflowExecutionsRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
	}
}

func toListOpenWorkflowExecutionsRequestTags(req *types.ListOpenWorkflowExecutionsRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
	}
}

func toListTaskListPartitionsRequestTags(req *types.ListTaskListPartitionsRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowTaskListName(req.GetTaskList().GetName()),
		tag.WorkflowTaskListKind(int32(req.GetTaskList().GetKind())),
	}
}

func toListWorkflowExecutionsRequestTags(req *types.ListWorkflowExecutionsRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
	}
}

func toPollForActivityTaskRequestTags(req *types.PollForActivityTaskRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowTaskListName(req.GetTaskList().GetName()),
		tag.WorkflowTaskListKind(int32(req.GetTaskList().GetKind())),
	}
}

func toPollForDecisionTaskRequestTags(req *types.PollForDecisionTaskRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowTaskListName(req.GetTaskList().GetName()),
		tag.WorkflowTaskListKind(int32(req.GetTaskList().GetKind())),
	}
}

func toQueryWorkflowRequestTags(req *types.QueryWorkflowRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetExecution().GetRunID()),
	}
}

func toRecordActivityTaskHeartbeatByIDRequestTags(req *types.RecordActivityTaskHeartbeatByIDRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowID()),
		tag.WorkflowRunID(req.GetRunID()),
	}
}

func toRefreshWorkflowTasksRequestTags(req *types.RefreshWorkflowTasksRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetExecution().GetRunID()),
	}
}

func toRequestCancelWorkflowExecutionRequestTags(req *types.RequestCancelWorkflowExecutionRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetWorkflowExecution().GetRunID()),
	}
}

func toResetStickyTaskListRequestTags(req *types.ResetStickyTaskListRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetExecution().GetRunID()),
	}
}

func toResetWorkflowExecutionRequestTags(req *types.ResetWorkflowExecutionRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetWorkflowExecution().GetRunID()),
	}
}

func toRespondActivityTaskCanceledByIDRequestTags(req *types.RespondActivityTaskCanceledByIDRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowID()),
		tag.WorkflowRunID(req.GetRunID()),
	}
}

func toRespondActivityTaskCompletedByIDRequestTags(req *types.RespondActivityTaskCompletedByIDRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowID()),
		tag.WorkflowRunID(req.GetRunID()),
	}
}

func toRespondActivityTaskFailedByIDRequestTags(req *types.RespondActivityTaskFailedByIDRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowID()),
		tag.WorkflowRunID(req.GetRunID()),
	}
}

func toRestartWorkflowExecutionRequestTags(req *types.RestartWorkflowExecutionRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetWorkflowExecution().GetRunID()),
	}
}

func toSignalWithStartWorkflowExecutionRequestTags(req *types.SignalWithStartWorkflowExecutionRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowID()),
		tag.WorkflowType(req.WorkflowType.GetName()),
		tag.WorkflowSignalName(req.GetSignalName()),
	}
}

func toSignalWithStartWorkflowExecutionAsyncRequestTags(req *types.SignalWithStartWorkflowExecutionAsyncRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowID()),
		tag.WorkflowType(req.WorkflowType.GetName()),
		tag.WorkflowSignalName(req.GetSignalName()),
	}
}

func toSignalWorkflowExecutionRequestTags(req *types.SignalWorkflowExecutionRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetWorkflowExecution().GetRunID()),
		tag.WorkflowSignalName(req.GetSignalName()),
	}
}

func toStartWorkflowExecutionRequestTags(req *types.StartWorkflowExecutionRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowID()),
		tag.WorkflowType(req.WorkflowType.GetName()),
		tag.WorkflowCronSchedule(req.GetCronSchedule()),
	}
}

func toStartWorkflowExecutionAsyncRequestTags(req *types.StartWorkflowExecutionAsyncRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowID()),
		tag.WorkflowType(req.WorkflowType.GetName()),
		tag.WorkflowCronSchedule(req.GetCronSchedule()),
	}
}

func toTerminateWorkflowExecutionRequestTags(req *types.TerminateWorkflowExecutionRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
		tag.WorkflowID(req.GetWorkflowExecution().GetWorkflowID()),
		tag.WorkflowRunID(req.GetWorkflowExecution().GetRunID()),
	}
}

func toScanWorkflowExecutionsRequestTags(req *types.ListWorkflowExecutionsRequest) []tag.Tag {
	return []tag.Tag{
		tag.WorkflowDomainName(req.GetDomain()),
	}
}
