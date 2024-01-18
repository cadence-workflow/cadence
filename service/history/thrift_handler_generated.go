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

package history

// Code generated by gowrap. DO NOT EDIT.
// template: ../templates/thrift.tmpl
// gowrap: http://github.com/hexdigest/gowrap

import (
	"context"

	"github.com/uber/cadence/.gen/go/history"
	"github.com/uber/cadence/.gen/go/replicator"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

func (g ThriftHandler) CloseShard(ctx context.Context, Request *shared.CloseShardRequest) (err error) {
	err = g.h.CloseShard(ctx, thrift.ToHistoryCloseShardRequest(Request))
	return thrift.FromError(err)
}

func (g ThriftHandler) DescribeHistoryHost(ctx context.Context, Request *shared.DescribeHistoryHostRequest) (dp1 *shared.DescribeHistoryHostResponse, err error) {
	response, err := g.h.DescribeHistoryHost(ctx, thrift.ToHistoryDescribeHistoryHostRequest(Request))
	return thrift.FromHistoryDescribeHistoryHostResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) DescribeMutableState(ctx context.Context, Request *history.DescribeMutableStateRequest) (dp1 *history.DescribeMutableStateResponse, err error) {
	response, err := g.h.DescribeMutableState(ctx, thrift.ToHistoryDescribeMutableStateRequest(Request))
	return thrift.FromHistoryDescribeMutableStateResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) DescribeQueue(ctx context.Context, Request *shared.DescribeQueueRequest) (dp1 *shared.DescribeQueueResponse, err error) {
	response, err := g.h.DescribeQueue(ctx, thrift.ToHistoryDescribeQueueRequest(Request))
	return thrift.FromHistoryDescribeQueueResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) DescribeWorkflowExecution(ctx context.Context, DescribeRequest *history.DescribeWorkflowExecutionRequest) (dp1 *shared.DescribeWorkflowExecutionResponse, err error) {
	response, err := g.h.DescribeWorkflowExecution(ctx, thrift.ToHistoryDescribeWorkflowExecutionRequest(DescribeRequest))
	return thrift.FromHistoryDescribeWorkflowExecutionResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) GetCrossClusterTasks(ctx context.Context, Request *shared.GetCrossClusterTasksRequest) (gp1 *shared.GetCrossClusterTasksResponse, err error) {
	response, err := g.h.GetCrossClusterTasks(ctx, thrift.ToHistoryGetCrossClusterTasksRequest(Request))
	return thrift.FromHistoryGetCrossClusterTasksResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) GetDLQReplicationMessages(ctx context.Context, Request *replicator.GetDLQReplicationMessagesRequest) (gp1 *replicator.GetDLQReplicationMessagesResponse, err error) {
	response, err := g.h.GetDLQReplicationMessages(ctx, thrift.ToHistoryGetDLQReplicationMessagesRequest(Request))
	return thrift.FromHistoryGetDLQReplicationMessagesResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) GetFailoverInfo(ctx context.Context, Request *history.GetFailoverInfoRequest) (gp1 *history.GetFailoverInfoResponse, err error) {
	response, err := g.h.GetFailoverInfo(ctx, thrift.ToHistoryGetFailoverInfoRequest(Request))
	return thrift.FromHistoryGetFailoverInfoResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) GetMutableState(ctx context.Context, GetRequest *history.GetMutableStateRequest) (gp1 *history.GetMutableStateResponse, err error) {
	response, err := g.h.GetMutableState(ctx, thrift.ToHistoryGetMutableStateRequest(GetRequest))
	return thrift.FromHistoryGetMutableStateResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) GetReplicationMessages(ctx context.Context, Request *replicator.GetReplicationMessagesRequest) (gp1 *replicator.GetReplicationMessagesResponse, err error) {
	response, err := g.h.GetReplicationMessages(ctx, thrift.ToHistoryGetReplicationMessagesRequest(Request))
	return thrift.FromHistoryGetReplicationMessagesResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) MergeDLQMessages(ctx context.Context, Request *replicator.MergeDLQMessagesRequest) (mp1 *replicator.MergeDLQMessagesResponse, err error) {
	response, err := g.h.MergeDLQMessages(ctx, thrift.ToHistoryMergeDLQMessagesRequest(Request))
	return thrift.FromHistoryMergeDLQMessagesResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) NotifyFailoverMarkers(ctx context.Context, Request *history.NotifyFailoverMarkersRequest) (err error) {
	err = g.h.NotifyFailoverMarkers(ctx, thrift.ToHistoryNotifyFailoverMarkersRequest(Request))
	return thrift.FromError(err)
}

func (g ThriftHandler) PollMutableState(ctx context.Context, PollRequest *history.PollMutableStateRequest) (pp1 *history.PollMutableStateResponse, err error) {
	response, err := g.h.PollMutableState(ctx, thrift.ToHistoryPollMutableStateRequest(PollRequest))
	return thrift.FromHistoryPollMutableStateResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) PurgeDLQMessages(ctx context.Context, Request *replicator.PurgeDLQMessagesRequest) (err error) {
	err = g.h.PurgeDLQMessages(ctx, thrift.ToHistoryPurgeDLQMessagesRequest(Request))
	return thrift.FromError(err)
}

func (g ThriftHandler) QueryWorkflow(ctx context.Context, QueryRequest *history.QueryWorkflowRequest) (qp1 *history.QueryWorkflowResponse, err error) {
	response, err := g.h.QueryWorkflow(ctx, thrift.ToHistoryQueryWorkflowRequest(QueryRequest))
	return thrift.FromHistoryQueryWorkflowResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) ReadDLQMessages(ctx context.Context, Request *replicator.ReadDLQMessagesRequest) (rp1 *replicator.ReadDLQMessagesResponse, err error) {
	response, err := g.h.ReadDLQMessages(ctx, thrift.ToHistoryReadDLQMessagesRequest(Request))
	return thrift.FromHistoryReadDLQMessagesResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) ReapplyEvents(ctx context.Context, ReapplyEventsRequest *history.ReapplyEventsRequest) (err error) {
	err = g.h.ReapplyEvents(ctx, thrift.ToHistoryReapplyEventsRequest(ReapplyEventsRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) RecordActivityTaskHeartbeat(ctx context.Context, HeartbeatRequest *history.RecordActivityTaskHeartbeatRequest) (rp1 *shared.RecordActivityTaskHeartbeatResponse, err error) {
	response, err := g.h.RecordActivityTaskHeartbeat(ctx, thrift.ToHistoryRecordActivityTaskHeartbeatRequest(HeartbeatRequest))
	return thrift.FromHistoryRecordActivityTaskHeartbeatResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) RecordActivityTaskStarted(ctx context.Context, AddRequest *history.RecordActivityTaskStartedRequest) (rp1 *history.RecordActivityTaskStartedResponse, err error) {
	response, err := g.h.RecordActivityTaskStarted(ctx, thrift.ToHistoryRecordActivityTaskStartedRequest(AddRequest))
	return thrift.FromHistoryRecordActivityTaskStartedResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) RecordChildExecutionCompleted(ctx context.Context, CompletionRequest *history.RecordChildExecutionCompletedRequest) (err error) {
	err = g.h.RecordChildExecutionCompleted(ctx, thrift.ToHistoryRecordChildExecutionCompletedRequest(CompletionRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) RecordDecisionTaskStarted(ctx context.Context, AddRequest *history.RecordDecisionTaskStartedRequest) (rp1 *history.RecordDecisionTaskStartedResponse, err error) {
	response, err := g.h.RecordDecisionTaskStarted(ctx, thrift.ToHistoryRecordDecisionTaskStartedRequest(AddRequest))
	return thrift.FromHistoryRecordDecisionTaskStartedResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) RefreshWorkflowTasks(ctx context.Context, Request *history.RefreshWorkflowTasksRequest) (err error) {
	err = g.h.RefreshWorkflowTasks(ctx, thrift.ToHistoryRefreshWorkflowTasksRequest(Request))
	return thrift.FromError(err)
}

func (g ThriftHandler) RemoveSignalMutableState(ctx context.Context, RemoveRequest *history.RemoveSignalMutableStateRequest) (err error) {
	err = g.h.RemoveSignalMutableState(ctx, thrift.ToHistoryRemoveSignalMutableStateRequest(RemoveRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) RemoveTask(ctx context.Context, Request *shared.RemoveTaskRequest) (err error) {
	err = g.h.RemoveTask(ctx, thrift.ToHistoryRemoveTaskRequest(Request))
	return thrift.FromError(err)
}

func (g ThriftHandler) ReplicateEventsV2(ctx context.Context, ReplicateV2Request *history.ReplicateEventsV2Request) (err error) {
	err = g.h.ReplicateEventsV2(ctx, thrift.ToHistoryReplicateEventsV2Request(ReplicateV2Request))
	return thrift.FromError(err)
}

func (g ThriftHandler) RequestCancelWorkflowExecution(ctx context.Context, CancelRequest *history.RequestCancelWorkflowExecutionRequest) (err error) {
	err = g.h.RequestCancelWorkflowExecution(ctx, thrift.ToHistoryRequestCancelWorkflowExecutionRequest(CancelRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) ResetQueue(ctx context.Context, Request *shared.ResetQueueRequest) (err error) {
	err = g.h.ResetQueue(ctx, thrift.ToHistoryResetQueueRequest(Request))
	return thrift.FromError(err)
}

func (g ThriftHandler) ResetStickyTaskList(ctx context.Context, ResetRequest *history.ResetStickyTaskListRequest) (rp1 *history.ResetStickyTaskListResponse, err error) {
	response, err := g.h.ResetStickyTaskList(ctx, thrift.ToHistoryResetStickyTaskListRequest(ResetRequest))
	return thrift.FromHistoryResetStickyTaskListResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) ResetWorkflowExecution(ctx context.Context, ResetRequest *history.ResetWorkflowExecutionRequest) (rp1 *shared.ResetWorkflowExecutionResponse, err error) {
	response, err := g.h.ResetWorkflowExecution(ctx, thrift.ToHistoryResetWorkflowExecutionRequest(ResetRequest))
	return thrift.FromHistoryResetWorkflowExecutionResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) RespondActivityTaskCanceled(ctx context.Context, CanceledRequest *history.RespondActivityTaskCanceledRequest) (err error) {
	err = g.h.RespondActivityTaskCanceled(ctx, thrift.ToHistoryRespondActivityTaskCanceledRequest(CanceledRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) RespondActivityTaskCompleted(ctx context.Context, CompleteRequest *history.RespondActivityTaskCompletedRequest) (err error) {
	err = g.h.RespondActivityTaskCompleted(ctx, thrift.ToHistoryRespondActivityTaskCompletedRequest(CompleteRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) RespondActivityTaskFailed(ctx context.Context, FailRequest *history.RespondActivityTaskFailedRequest) (err error) {
	err = g.h.RespondActivityTaskFailed(ctx, thrift.ToHistoryRespondActivityTaskFailedRequest(FailRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) RespondCrossClusterTasksCompleted(ctx context.Context, Request *shared.RespondCrossClusterTasksCompletedRequest) (rp1 *shared.RespondCrossClusterTasksCompletedResponse, err error) {
	response, err := g.h.RespondCrossClusterTasksCompleted(ctx, thrift.ToHistoryRespondCrossClusterTasksCompletedRequest(Request))
	return thrift.FromHistoryRespondCrossClusterTasksCompletedResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) RespondDecisionTaskCompleted(ctx context.Context, CompleteRequest *history.RespondDecisionTaskCompletedRequest) (rp1 *history.RespondDecisionTaskCompletedResponse, err error) {
	response, err := g.h.RespondDecisionTaskCompleted(ctx, thrift.ToHistoryRespondDecisionTaskCompletedRequest(CompleteRequest))
	return thrift.FromHistoryRespondDecisionTaskCompletedResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) RespondDecisionTaskFailed(ctx context.Context, FailedRequest *history.RespondDecisionTaskFailedRequest) (err error) {
	err = g.h.RespondDecisionTaskFailed(ctx, thrift.ToHistoryRespondDecisionTaskFailedRequest(FailedRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) ScheduleDecisionTask(ctx context.Context, ScheduleRequest *history.ScheduleDecisionTaskRequest) (err error) {
	err = g.h.ScheduleDecisionTask(ctx, thrift.ToHistoryScheduleDecisionTaskRequest(ScheduleRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) SignalWithStartWorkflowExecution(ctx context.Context, SignalWithStartRequest *history.SignalWithStartWorkflowExecutionRequest) (sp1 *shared.StartWorkflowExecutionResponse, err error) {
	response, err := g.h.SignalWithStartWorkflowExecution(ctx, thrift.ToHistorySignalWithStartWorkflowExecutionRequest(SignalWithStartRequest))
	return thrift.FromHistorySignalWithStartWorkflowExecutionResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) SignalWorkflowExecution(ctx context.Context, SignalRequest *history.SignalWorkflowExecutionRequest) (err error) {
	err = g.h.SignalWorkflowExecution(ctx, thrift.ToHistorySignalWorkflowExecutionRequest(SignalRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) StartWorkflowExecution(ctx context.Context, StartRequest *history.StartWorkflowExecutionRequest) (sp1 *shared.StartWorkflowExecutionResponse, err error) {
	response, err := g.h.StartWorkflowExecution(ctx, thrift.ToHistoryStartWorkflowExecutionRequest(StartRequest))
	return thrift.FromHistoryStartWorkflowExecutionResponse(response), thrift.FromError(err)
}

func (g ThriftHandler) SyncActivity(ctx context.Context, SyncActivityRequest *history.SyncActivityRequest) (err error) {
	err = g.h.SyncActivity(ctx, thrift.ToHistorySyncActivityRequest(SyncActivityRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) SyncShardStatus(ctx context.Context, SyncShardStatusRequest *history.SyncShardStatusRequest) (err error) {
	err = g.h.SyncShardStatus(ctx, thrift.ToHistorySyncShardStatusRequest(SyncShardStatusRequest))
	return thrift.FromError(err)
}

func (g ThriftHandler) TerminateWorkflowExecution(ctx context.Context, TerminateRequest *history.TerminateWorkflowExecutionRequest) (err error) {
	err = g.h.TerminateWorkflowExecution(ctx, thrift.ToHistoryTerminateWorkflowExecutionRequest(TerminateRequest))
	return thrift.FromError(err)
}
