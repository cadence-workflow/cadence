// Copyright (c) 2017-2020 Uber Technologies Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -package handler github.com/uber/cadence/service/history/handler Handler
//go:generate gowrap gen -g -p . -i Handler -t ../../templates/grpc.tmpl -o ../wrappers/grpc/grpc_handler_generated.go -v handler=GRPC -v package=historyv1 -v path=github.com/uber/cadence/.gen/proto/history/v1 -v prefix=History
//go:generate gowrap gen -g -p ../../../.gen/go/history/historyserviceserver -i Interface -t ../../templates/thrift.tmpl -o ../wrappers/thrift/thrift_handler_generated.go -v handler=Thrift -v prefix=History

// TODO: generate a wrapper for handler which handles redirection to the correct cluster based on <domainid, workflowid, runid> and redirection policy.
// This will be needed only if we want to support forwarding of active-active domain calls in frontend instead of always relying on history.

package handler

import (
	"context"
	"time"

	"github.com/uber/cadence/common/types"
)

// Handler interface for history service
type Handler interface {
	// Do not use embeded methods, otherwise, we got the following error from gowrap
	// and we only get this error from history/interface.go, not sure why
	// failed to parse interface declaration: Daemon: target declaration not found
	// service/history/interface.go:22: running "gowrap": exit status 1
	// common.Daemon
	Start()
	Stop()

	PrepareToStop(time.Duration) time.Duration
	Health(context.Context) (*types.HealthStatus, error)
	CloseShard(context.Context, *types.CloseShardRequest) error
	DescribeHistoryHost(context.Context, *types.DescribeHistoryHostRequest) (*types.DescribeHistoryHostResponse, error)
	DescribeMutableState(context.Context, *types.DescribeMutableStateRequest) (*types.DescribeMutableStateResponse, error)
	DescribeQueue(context.Context, *types.DescribeQueueRequest) (*types.DescribeQueueResponse, error)
	DescribeWorkflowExecution(context.Context, *types.HistoryDescribeWorkflowExecutionRequest) (*types.DescribeWorkflowExecutionResponse, error)
	GetCrossClusterTasks(context.Context, *types.GetCrossClusterTasksRequest) (*types.GetCrossClusterTasksResponse, error)
	CountDLQMessages(context.Context, *types.CountDLQMessagesRequest) (*types.HistoryCountDLQMessagesResponse, error)
	GetDLQReplicationMessages(context.Context, *types.GetDLQReplicationMessagesRequest) (*types.GetDLQReplicationMessagesResponse, error)
	GetMutableState(context.Context, *types.GetMutableStateRequest) (*types.GetMutableStateResponse, error)
	GetReplicationMessages(context.Context, *types.GetReplicationMessagesRequest) (*types.GetReplicationMessagesResponse, error)
	MergeDLQMessages(context.Context, *types.MergeDLQMessagesRequest) (*types.MergeDLQMessagesResponse, error)
	NotifyFailoverMarkers(context.Context, *types.NotifyFailoverMarkersRequest) error
	PollMutableState(context.Context, *types.PollMutableStateRequest) (*types.PollMutableStateResponse, error)
	PurgeDLQMessages(context.Context, *types.PurgeDLQMessagesRequest) error
	QueryWorkflow(context.Context, *types.HistoryQueryWorkflowRequest) (*types.HistoryQueryWorkflowResponse, error)
	ReadDLQMessages(context.Context, *types.ReadDLQMessagesRequest) (*types.ReadDLQMessagesResponse, error)
	ReapplyEvents(context.Context, *types.HistoryReapplyEventsRequest) error
	RecordActivityTaskHeartbeat(context.Context, *types.HistoryRecordActivityTaskHeartbeatRequest) (*types.RecordActivityTaskHeartbeatResponse, error)
	RecordActivityTaskStarted(context.Context, *types.RecordActivityTaskStartedRequest) (*types.RecordActivityTaskStartedResponse, error)
	RecordChildExecutionCompleted(context.Context, *types.RecordChildExecutionCompletedRequest) error
	RecordDecisionTaskStarted(context.Context, *types.RecordDecisionTaskStartedRequest) (*types.RecordDecisionTaskStartedResponse, error)
	RefreshWorkflowTasks(context.Context, *types.HistoryRefreshWorkflowTasksRequest) error
	RemoveSignalMutableState(context.Context, *types.RemoveSignalMutableStateRequest) error
	RemoveTask(context.Context, *types.RemoveTaskRequest) error
	ReplicateEventsV2(context.Context, *types.ReplicateEventsV2Request) error
	RequestCancelWorkflowExecution(context.Context, *types.HistoryRequestCancelWorkflowExecutionRequest) error
	ResetQueue(context.Context, *types.ResetQueueRequest) error
	ResetStickyTaskList(context.Context, *types.HistoryResetStickyTaskListRequest) (*types.HistoryResetStickyTaskListResponse, error)
	ResetWorkflowExecution(context.Context, *types.HistoryResetWorkflowExecutionRequest) (*types.ResetWorkflowExecutionResponse, error)
	RespondActivityTaskCanceled(context.Context, *types.HistoryRespondActivityTaskCanceledRequest) error
	RespondActivityTaskCompleted(context.Context, *types.HistoryRespondActivityTaskCompletedRequest) error
	RespondActivityTaskFailed(context.Context, *types.HistoryRespondActivityTaskFailedRequest) error
	RespondCrossClusterTasksCompleted(context.Context, *types.RespondCrossClusterTasksCompletedRequest) (*types.RespondCrossClusterTasksCompletedResponse, error)
	RespondDecisionTaskCompleted(context.Context, *types.HistoryRespondDecisionTaskCompletedRequest) (*types.HistoryRespondDecisionTaskCompletedResponse, error)
	RespondDecisionTaskFailed(context.Context, *types.HistoryRespondDecisionTaskFailedRequest) error
	ScheduleDecisionTask(context.Context, *types.ScheduleDecisionTaskRequest) error
	SignalWithStartWorkflowExecution(context.Context, *types.HistorySignalWithStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error)
	SignalWorkflowExecution(context.Context, *types.HistorySignalWorkflowExecutionRequest) error
	StartWorkflowExecution(context.Context, *types.HistoryStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error)
	SyncActivity(context.Context, *types.SyncActivityRequest) error
	SyncShardStatus(context.Context, *types.SyncShardStatusRequest) error
	TerminateWorkflowExecution(context.Context, *types.HistoryTerminateWorkflowExecutionRequest) error
	GetFailoverInfo(context.Context, *types.GetFailoverInfoRequest) (*types.GetFailoverInfoResponse, error)
	RatelimitUpdate(context.Context, *types.RatelimitUpdateRequest) (*types.RatelimitUpdateResponse, error)
}
