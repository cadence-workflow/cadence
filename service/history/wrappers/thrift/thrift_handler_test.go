// Copyright (c) 2020 Uber Technologies Inc.
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

package thrift

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/.gen/go/health"
	hist "github.com/uber/cadence/.gen/go/history"
	"github.com/uber/cadence/.gen/go/replicator"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/handler"
)

func TestThriftHandler(t *testing.T) {
	ctrl := gomock.NewController(t)

	h := handler.NewMockHandler(ctrl)
	th := NewThriftHandler(h)
	ctx := context.Background()
	internalErr := &types.InternalServiceError{Message: "test"}
	expectedErr := &shared.InternalServiceError{Message: "test"}

	t.Run("Health", func(t *testing.T) {
		h.EXPECT().Health(ctx).Return(&types.HealthStatus{}, internalErr).Times(1)
		resp, err := th.Health(ctx)
		assert.Equal(t, health.HealthStatus{Msg: common.StringPtr("")}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("CloseShard", func(t *testing.T) {
		h.EXPECT().CloseShard(ctx, &types.CloseShardRequest{}).Return(internalErr).Times(1)
		err := th.CloseShard(ctx, &shared.CloseShardRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("DescribeHistoryHost", func(t *testing.T) {
		h.EXPECT().DescribeHistoryHost(ctx, &types.DescribeHistoryHostRequest{}).Return(&types.DescribeHistoryHostResponse{}, internalErr).Times(1)
		resp, err := th.DescribeHistoryHost(ctx, &shared.DescribeHistoryHostRequest{})
		assert.Equal(t, shared.DescribeHistoryHostResponse{NumberOfShards: common.Int32Ptr(0), ShardControllerStatus: common.StringPtr(""), Address: common.StringPtr("")}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("DescribeMutableState", func(t *testing.T) {
		h.EXPECT().DescribeMutableState(ctx, &types.DescribeMutableStateRequest{}).Return(&types.DescribeMutableStateResponse{}, internalErr).Times(1)
		resp, err := th.DescribeMutableState(ctx, &hist.DescribeMutableStateRequest{})
		assert.Equal(t, hist.DescribeMutableStateResponse{MutableStateInCache: common.StringPtr(""), MutableStateInDatabase: common.StringPtr("")}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("DescribeQueue", func(t *testing.T) {
		h.EXPECT().DescribeQueue(ctx, &types.DescribeQueueRequest{}).Return(&types.DescribeQueueResponse{}, internalErr).Times(1)
		resp, err := th.DescribeQueue(ctx, &shared.DescribeQueueRequest{})
		assert.Equal(t, shared.DescribeQueueResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("DescribeWorkflowExecution", func(t *testing.T) {
		h.EXPECT().DescribeWorkflowExecution(ctx, &types.HistoryDescribeWorkflowExecutionRequest{}).Return(&types.DescribeWorkflowExecutionResponse{}, internalErr).Times(1)
		resp, err := th.DescribeWorkflowExecution(ctx, &hist.DescribeWorkflowExecutionRequest{})
		assert.Equal(t, shared.DescribeWorkflowExecutionResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("GetCrossClusterTasks", func(t *testing.T) {
		h.EXPECT().GetCrossClusterTasks(ctx, &types.GetCrossClusterTasksRequest{}).Return(&types.GetCrossClusterTasksResponse{}, internalErr).Times(1)
		resp, err := th.GetCrossClusterTasks(ctx, &shared.GetCrossClusterTasksRequest{})
		assert.Equal(t, shared.GetCrossClusterTasksResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("GetDLQReplicationMessages", func(t *testing.T) {
		h.EXPECT().GetDLQReplicationMessages(ctx, &types.GetDLQReplicationMessagesRequest{}).Return(&types.GetDLQReplicationMessagesResponse{}, internalErr).Times(1)
		resp, err := th.GetDLQReplicationMessages(ctx, &replicator.GetDLQReplicationMessagesRequest{})
		assert.Equal(t, replicator.GetDLQReplicationMessagesResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("GetMutableState", func(t *testing.T) {
		h.EXPECT().GetMutableState(ctx, &types.GetMutableStateRequest{}).Return(&types.GetMutableStateResponse{}, internalErr).Times(1)
		resp, err := th.GetMutableState(ctx, &hist.GetMutableStateRequest{})
		assert.Equal(t, hist.GetMutableStateResponse{
			IsWorkflowRunning:       common.BoolPtr(false),
			NextEventId:             common.Int64Ptr(0),
			LastFirstEventId:        common.Int64Ptr(0),
			ClientLibraryVersion:    common.StringPtr(""),
			ClientFeatureVersion:    common.StringPtr(""),
			ClientImpl:              common.StringPtr(""),
			EventStoreVersion:       common.Int32Ptr(0),
			IsStickyTaskListEnabled: common.BoolPtr(false),
			HistorySize:             common.Int64Ptr(0),
		}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("GetReplicationMessages", func(t *testing.T) {
		h.EXPECT().GetReplicationMessages(ctx, &types.GetReplicationMessagesRequest{}).Return(&types.GetReplicationMessagesResponse{}, internalErr).Times(1)
		resp, err := th.GetReplicationMessages(ctx, &replicator.GetReplicationMessagesRequest{})
		assert.Equal(t, replicator.GetReplicationMessagesResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("MergeDLQMessages", func(t *testing.T) {
		h.EXPECT().MergeDLQMessages(ctx, &types.MergeDLQMessagesRequest{}).Return(&types.MergeDLQMessagesResponse{}, internalErr).Times(1)
		resp, err := th.MergeDLQMessages(ctx, &replicator.MergeDLQMessagesRequest{})
		assert.Equal(t, replicator.MergeDLQMessagesResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("NotifyFailoverMarkers", func(t *testing.T) {
		h.EXPECT().NotifyFailoverMarkers(ctx, &types.NotifyFailoverMarkersRequest{}).Return(internalErr).Times(1)
		err := th.NotifyFailoverMarkers(ctx, &hist.NotifyFailoverMarkersRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("PollMutableState", func(t *testing.T) {
		h.EXPECT().PollMutableState(ctx, &types.PollMutableStateRequest{}).Return(&types.PollMutableStateResponse{}, internalErr).Times(1)
		resp, err := th.PollMutableState(ctx, &hist.PollMutableStateRequest{})
		assert.Equal(t, hist.PollMutableStateResponse{
			NextEventId:          common.Int64Ptr(0),
			LastFirstEventId:     common.Int64Ptr(0),
			ClientLibraryVersion: common.StringPtr(""),
			ClientFeatureVersion: common.StringPtr(""),
			ClientImpl:           common.StringPtr(""),
		}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("PurgeDLQMessages", func(t *testing.T) {
		h.EXPECT().PurgeDLQMessages(ctx, &types.PurgeDLQMessagesRequest{}).Return(internalErr).Times(1)
		err := th.PurgeDLQMessages(ctx, &replicator.PurgeDLQMessagesRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("QueryWorkflow", func(t *testing.T) {
		h.EXPECT().QueryWorkflow(ctx, &types.HistoryQueryWorkflowRequest{}).Return(&types.HistoryQueryWorkflowResponse{}, internalErr).Times(1)
		resp, err := th.QueryWorkflow(ctx, &hist.QueryWorkflowRequest{})
		assert.Equal(t, hist.QueryWorkflowResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ReadDLQMessages", func(t *testing.T) {
		h.EXPECT().ReadDLQMessages(ctx, &types.ReadDLQMessagesRequest{}).Return(&types.ReadDLQMessagesResponse{}, internalErr).Times(1)
		resp, err := th.ReadDLQMessages(ctx, &replicator.ReadDLQMessagesRequest{})
		assert.Equal(t, replicator.ReadDLQMessagesResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ReapplyEvents", func(t *testing.T) {
		h.EXPECT().ReapplyEvents(ctx, &types.HistoryReapplyEventsRequest{}).Return(internalErr).Times(1)
		err := th.ReapplyEvents(ctx, &hist.ReapplyEventsRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RecordActivityTaskHeartbeat", func(t *testing.T) {
		h.EXPECT().RecordActivityTaskHeartbeat(ctx, &types.HistoryRecordActivityTaskHeartbeatRequest{}).Return(&types.RecordActivityTaskHeartbeatResponse{}, internalErr).Times(1)
		resp, err := th.RecordActivityTaskHeartbeat(ctx, &hist.RecordActivityTaskHeartbeatRequest{})
		assert.Equal(t, shared.RecordActivityTaskHeartbeatResponse{CancelRequested: common.BoolPtr(false)}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RecordActivityTaskStarted", func(t *testing.T) {
		h.EXPECT().RecordActivityTaskStarted(ctx, &types.RecordActivityTaskStartedRequest{}).Return(&types.RecordActivityTaskStartedResponse{}, internalErr).Times(1)
		resp, err := th.RecordActivityTaskStarted(ctx, &hist.RecordActivityTaskStartedRequest{})
		assert.Equal(t, hist.RecordActivityTaskStartedResponse{WorkflowDomain: common.StringPtr(""), Attempt: common.Int64Ptr(0)}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RecordChildExecutionCompleted", func(t *testing.T) {
		h.EXPECT().RecordChildExecutionCompleted(ctx, &types.RecordChildExecutionCompletedRequest{}).Return(internalErr).Times(1)
		err := th.RecordChildExecutionCompleted(ctx, &hist.RecordChildExecutionCompletedRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RecordDecisionTaskStarted", func(t *testing.T) {
		h.EXPECT().RecordDecisionTaskStarted(ctx, &types.RecordDecisionTaskStartedRequest{}).Return(&types.RecordDecisionTaskStartedResponse{}, internalErr).Times(1)
		resp, err := th.RecordDecisionTaskStarted(ctx, &hist.RecordDecisionTaskStartedRequest{})
		assert.Equal(t, hist.RecordDecisionTaskStartedResponse{
			ScheduledEventId:       common.Int64Ptr(0),
			StartedEventId:         common.Int64Ptr(0),
			NextEventId:            common.Int64Ptr(0),
			Attempt:                common.Int64Ptr(0),
			StickyExecutionEnabled: common.BoolPtr(false),
			EventStoreVersion:      common.Int32Ptr(0),
			HistorySize:            common.Int64Ptr(0),
		}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RefreshWorkflowTasks", func(t *testing.T) {
		h.EXPECT().RefreshWorkflowTasks(ctx, &types.HistoryRefreshWorkflowTasksRequest{}).Return(internalErr).Times(1)
		err := th.RefreshWorkflowTasks(ctx, &hist.RefreshWorkflowTasksRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RemoveSignalMutableState", func(t *testing.T) {
		h.EXPECT().RemoveSignalMutableState(ctx, &types.RemoveSignalMutableStateRequest{}).Return(internalErr).Times(1)
		err := th.RemoveSignalMutableState(ctx, &hist.RemoveSignalMutableStateRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RemoveTask", func(t *testing.T) {
		h.EXPECT().RemoveTask(ctx, &types.RemoveTaskRequest{}).Return(internalErr).Times(1)
		err := th.RemoveTask(ctx, &shared.RemoveTaskRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ReplicateEventsV2", func(t *testing.T) {
		h.EXPECT().ReplicateEventsV2(ctx, &types.ReplicateEventsV2Request{}).Return(internalErr).Times(1)
		err := th.ReplicateEventsV2(ctx, &hist.ReplicateEventsV2Request{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RequestCancelWorkflowExecution", func(t *testing.T) {
		h.EXPECT().RequestCancelWorkflowExecution(ctx, &types.HistoryRequestCancelWorkflowExecutionRequest{}).Return(internalErr).Times(1)
		err := th.RequestCancelWorkflowExecution(ctx, &hist.RequestCancelWorkflowExecutionRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ResetQueue", func(t *testing.T) {
		h.EXPECT().ResetQueue(ctx, &types.ResetQueueRequest{}).Return(internalErr).Times(1)
		err := th.ResetQueue(ctx, &shared.ResetQueueRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ResetStickyTaskList", func(t *testing.T) {
		h.EXPECT().ResetStickyTaskList(ctx, &types.HistoryResetStickyTaskListRequest{}).Return(&types.HistoryResetStickyTaskListResponse{}, internalErr).Times(1)
		resp, err := th.ResetStickyTaskList(ctx, &hist.ResetStickyTaskListRequest{})
		assert.Equal(t, hist.ResetStickyTaskListResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ResetWorkflowExecution", func(t *testing.T) {
		h.EXPECT().ResetWorkflowExecution(ctx, &types.HistoryResetWorkflowExecutionRequest{}).Return(&types.ResetWorkflowExecutionResponse{}, internalErr).Times(1)
		resp, err := th.ResetWorkflowExecution(ctx, &hist.ResetWorkflowExecutionRequest{})
		assert.Equal(t, shared.ResetWorkflowExecutionResponse{RunId: common.StringPtr("")}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondActivityTaskCanceled", func(t *testing.T) {
		h.EXPECT().RespondActivityTaskCanceled(ctx, &types.HistoryRespondActivityTaskCanceledRequest{}).Return(internalErr).Times(1)
		err := th.RespondActivityTaskCanceled(ctx, &hist.RespondActivityTaskCanceledRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondActivityTaskCompleted", func(t *testing.T) {
		h.EXPECT().RespondActivityTaskCompleted(ctx, &types.HistoryRespondActivityTaskCompletedRequest{}).Return(internalErr).Times(1)
		err := th.RespondActivityTaskCompleted(ctx, &hist.RespondActivityTaskCompletedRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondActivityTaskFailed", func(t *testing.T) {
		h.EXPECT().RespondActivityTaskFailed(ctx, &types.HistoryRespondActivityTaskFailedRequest{}).Return(internalErr).Times(1)
		err := th.RespondActivityTaskFailed(ctx, &hist.RespondActivityTaskFailedRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondCrossClusterTasksCompleted", func(t *testing.T) {
		h.EXPECT().RespondCrossClusterTasksCompleted(ctx, &types.RespondCrossClusterTasksCompletedRequest{}).Return(&types.RespondCrossClusterTasksCompletedResponse{}, internalErr).Times(1)
		resp, err := th.RespondCrossClusterTasksCompleted(ctx, &shared.RespondCrossClusterTasksCompletedRequest{})
		assert.Equal(t, shared.RespondCrossClusterTasksCompletedResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondDecisionTaskCompleted", func(t *testing.T) {
		h.EXPECT().RespondDecisionTaskCompleted(ctx, &types.HistoryRespondDecisionTaskCompletedRequest{}).Return(&types.HistoryRespondDecisionTaskCompletedResponse{}, internalErr).Times(1)
		resp, err := th.RespondDecisionTaskCompleted(ctx, &hist.RespondDecisionTaskCompletedRequest{})
		assert.Equal(t, hist.RespondDecisionTaskCompletedResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondDecisionTaskFailed", func(t *testing.T) {
		h.EXPECT().RespondDecisionTaskFailed(ctx, &types.HistoryRespondDecisionTaskFailedRequest{}).Return(internalErr).Times(1)
		err := th.RespondDecisionTaskFailed(ctx, &hist.RespondDecisionTaskFailedRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ScheduleDecisionTask", func(t *testing.T) {
		h.EXPECT().ScheduleDecisionTask(ctx, &types.ScheduleDecisionTaskRequest{}).Return(internalErr).Times(1)
		err := th.ScheduleDecisionTask(ctx, &hist.ScheduleDecisionTaskRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("SignalWithStartWorkflowExecution", func(t *testing.T) {
		h.EXPECT().SignalWithStartWorkflowExecution(ctx, &types.HistorySignalWithStartWorkflowExecutionRequest{}).Return(&types.StartWorkflowExecutionResponse{}, internalErr).Times(1)
		resp, err := th.SignalWithStartWorkflowExecution(ctx, &hist.SignalWithStartWorkflowExecutionRequest{})
		assert.Equal(t, shared.StartWorkflowExecutionResponse{RunId: common.StringPtr("")}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("SignalWorkflowExecution", func(t *testing.T) {
		h.EXPECT().SignalWorkflowExecution(ctx, &types.HistorySignalWorkflowExecutionRequest{}).Return(internalErr).Times(1)
		err := th.SignalWorkflowExecution(ctx, &hist.SignalWorkflowExecutionRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("StartWorkflowExecution", func(t *testing.T) {
		h.EXPECT().StartWorkflowExecution(ctx, &types.HistoryStartWorkflowExecutionRequest{}).Return(&types.StartWorkflowExecutionResponse{}, internalErr).Times(1)
		resp, err := th.StartWorkflowExecution(ctx, &hist.StartWorkflowExecutionRequest{})
		assert.Equal(t, shared.StartWorkflowExecutionResponse{RunId: common.StringPtr("")}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("SyncActivity", func(t *testing.T) {
		h.EXPECT().SyncActivity(ctx, &types.SyncActivityRequest{}).Return(internalErr).Times(1)
		err := th.SyncActivity(ctx, &hist.SyncActivityRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("SyncShardStatus", func(t *testing.T) {
		h.EXPECT().SyncShardStatus(ctx, &types.SyncShardStatusRequest{}).Return(internalErr).Times(1)
		err := th.SyncShardStatus(ctx, &hist.SyncShardStatusRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("TerminateWorkflowExecution", func(t *testing.T) {
		h.EXPECT().TerminateWorkflowExecution(ctx, &types.HistoryTerminateWorkflowExecutionRequest{}).Return(internalErr).Times(1)
		err := th.TerminateWorkflowExecution(ctx, &hist.TerminateWorkflowExecutionRequest{})
		assert.Equal(t, expectedErr, err)
	})
}
