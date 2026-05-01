// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package thrift

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/.gen/go/health"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/api"
)

func TestThriftHandler(t *testing.T) {
	ctrl := gomock.NewController(t)

	h := api.NewMockHandler(ctrl)
	th := NewAPIHandler(h)
	ctx := context.Background()
	internalErr := &types.InternalServiceError{Message: "test"}
	expectedErr := &shared.InternalServiceError{Message: "test"}

	t.Run("Health", func(t *testing.T) {
		h.EXPECT().Health(ctx).Return(&types.HealthStatus{}, internalErr).Times(1)
		resp, err := th.Health(ctx)
		assert.Equal(t, health.HealthStatus{Msg: common.StringPtr("")}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("CountWorkflowExecutions", func(t *testing.T) {
		h.EXPECT().CountWorkflowExecutions(ctx, &types.CountWorkflowExecutionsRequest{}).Return(&types.CountWorkflowExecutionsResponse{}, internalErr).Times(1)
		resp, err := th.CountWorkflowExecutions(ctx, &shared.CountWorkflowExecutionsRequest{})
		assert.Equal(t, shared.CountWorkflowExecutionsResponse{Count: common.Int64Ptr(0)}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("DeprecateDomain", func(t *testing.T) {
		h.EXPECT().DeprecateDomain(ctx, &types.DeprecateDomainRequest{}).Return(internalErr).Times(1)
		err := th.DeprecateDomain(ctx, &shared.DeprecateDomainRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("DescribeDomain", func(t *testing.T) {
		h.EXPECT().DescribeDomain(ctx, &types.DescribeDomainRequest{}).Return(&types.DescribeDomainResponse{}, internalErr).Times(1)
		resp, err := th.DescribeDomain(ctx, &shared.DescribeDomainRequest{})
		assert.Equal(t, shared.DescribeDomainResponse{IsGlobalDomain: common.BoolPtr(false), FailoverVersion: common.Int64Ptr(0)}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("DescribeTaskList", func(t *testing.T) {
		h.EXPECT().DescribeTaskList(ctx, &types.DescribeTaskListRequest{}).Return(&types.DescribeTaskListResponse{}, internalErr).Times(1)
		resp, err := th.DescribeTaskList(ctx, &shared.DescribeTaskListRequest{})
		assert.Equal(t, shared.DescribeTaskListResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("DescribeWorkflowExecution", func(t *testing.T) {
		h.EXPECT().DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{}).Return(&types.DescribeWorkflowExecutionResponse{}, internalErr).Times(1)
		resp, err := th.DescribeWorkflowExecution(ctx, &shared.DescribeWorkflowExecutionRequest{})
		assert.Equal(t, shared.DescribeWorkflowExecutionResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("GetClusterInfo", func(t *testing.T) {
		h.EXPECT().GetClusterInfo(ctx).Return(&types.ClusterInfo{}, internalErr).Times(1)
		resp, err := th.GetClusterInfo(ctx)
		assert.Equal(t, shared.ClusterInfo{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("GetSearchAttributes", func(t *testing.T) {
		h.EXPECT().GetSearchAttributes(ctx).Return(&types.GetSearchAttributesResponse{}, internalErr).Times(1)
		resp, err := th.GetSearchAttributes(ctx)
		assert.Equal(t, shared.GetSearchAttributesResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("GetWorkflowExecutionHistory", func(t *testing.T) {
		h.EXPECT().GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{}).Return(&types.GetWorkflowExecutionHistoryResponse{}, internalErr).Times(1)
		resp, err := th.GetWorkflowExecutionHistory(ctx, &shared.GetWorkflowExecutionHistoryRequest{})
		assert.Equal(t, shared.GetWorkflowExecutionHistoryResponse{Archived: common.BoolPtr(false)}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ListArchivedWorkflowExecutions", func(t *testing.T) {
		h.EXPECT().ListArchivedWorkflowExecutions(ctx, &types.ListArchivedWorkflowExecutionsRequest{}).Return(&types.ListArchivedWorkflowExecutionsResponse{}, internalErr).Times(1)
		resp, err := th.ListArchivedWorkflowExecutions(ctx, &shared.ListArchivedWorkflowExecutionsRequest{})
		assert.Equal(t, shared.ListArchivedWorkflowExecutionsResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ListClosedWorkflowExecutions", func(t *testing.T) {
		h.EXPECT().ListClosedWorkflowExecutions(ctx, &types.ListClosedWorkflowExecutionsRequest{}).Return(&types.ListClosedWorkflowExecutionsResponse{}, internalErr).Times(1)
		resp, err := th.ListClosedWorkflowExecutions(ctx, &shared.ListClosedWorkflowExecutionsRequest{})
		assert.Equal(t, shared.ListClosedWorkflowExecutionsResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ListDomains", func(t *testing.T) {
		h.EXPECT().ListDomains(ctx, &types.ListDomainsRequest{}).Return(&types.ListDomainsResponse{}, internalErr).Times(1)
		resp, err := th.ListDomains(ctx, &shared.ListDomainsRequest{})
		assert.Equal(t, shared.ListDomainsResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ListOpenWorkflowExecutions", func(t *testing.T) {
		h.EXPECT().ListOpenWorkflowExecutions(ctx, &types.ListOpenWorkflowExecutionsRequest{}).Return(&types.ListOpenWorkflowExecutionsResponse{}, internalErr).Times(1)
		resp, err := th.ListOpenWorkflowExecutions(ctx, &shared.ListOpenWorkflowExecutionsRequest{})
		assert.Equal(t, shared.ListOpenWorkflowExecutionsResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ListTaskListPartitions", func(t *testing.T) {
		h.EXPECT().ListTaskListPartitions(ctx, &types.ListTaskListPartitionsRequest{}).Return(&types.ListTaskListPartitionsResponse{}, internalErr).Times(1)
		resp, err := th.ListTaskListPartitions(ctx, &shared.ListTaskListPartitionsRequest{})
		assert.Equal(t, shared.ListTaskListPartitionsResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ListWorkflowExecutions", func(t *testing.T) {
		h.EXPECT().ListWorkflowExecutions(ctx, &types.ListWorkflowExecutionsRequest{}).Return(&types.ListWorkflowExecutionsResponse{}, internalErr).Times(1)
		resp, err := th.ListWorkflowExecutions(ctx, &shared.ListWorkflowExecutionsRequest{})
		assert.Equal(t, shared.ListWorkflowExecutionsResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("PollForActivityTask", func(t *testing.T) {
		h.EXPECT().PollForActivityTask(ctx, &types.PollForActivityTaskRequest{}).Return(&types.PollForActivityTaskResponse{}, internalErr).Times(1)
		resp, err := th.PollForActivityTask(ctx, &shared.PollForActivityTaskRequest{})
		assert.Equal(t, shared.PollForActivityTaskResponse{WorkflowDomain: common.StringPtr(""), ActivityId: common.StringPtr(""), Attempt: common.Int32Ptr(0)}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("PollForDecisionTask", func(t *testing.T) {
		h.EXPECT().PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{}).Return(&types.PollForDecisionTaskResponse{}, internalErr).Times(1)
		resp, err := th.PollForDecisionTask(ctx, &shared.PollForDecisionTaskRequest{})
		assert.Equal(t, shared.PollForDecisionTaskResponse{StartedEventId: common.Int64Ptr(0), Attempt: common.Int64Ptr(0), BacklogCountHint: common.Int64Ptr(0), NextEventId: common.Int64Ptr(0), TotalHistoryBytes: common.Int64Ptr(0)}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("QueryWorkflow", func(t *testing.T) {
		h.EXPECT().QueryWorkflow(ctx, &types.QueryWorkflowRequest{}).Return(&types.QueryWorkflowResponse{}, internalErr).Times(1)
		resp, err := th.QueryWorkflow(ctx, &shared.QueryWorkflowRequest{})
		assert.Equal(t, shared.QueryWorkflowResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RecordActivityTaskHeartbeat", func(t *testing.T) {
		h.EXPECT().RecordActivityTaskHeartbeat(ctx, &types.RecordActivityTaskHeartbeatRequest{}).Return(&types.RecordActivityTaskHeartbeatResponse{}, internalErr).Times(1)
		resp, err := th.RecordActivityTaskHeartbeat(ctx, &shared.RecordActivityTaskHeartbeatRequest{})
		assert.Equal(t, shared.RecordActivityTaskHeartbeatResponse{CancelRequested: common.BoolPtr(false)}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RecordActivityTaskHeartbeatByID", func(t *testing.T) {
		h.EXPECT().RecordActivityTaskHeartbeatByID(ctx, &types.RecordActivityTaskHeartbeatByIDRequest{}).Return(&types.RecordActivityTaskHeartbeatResponse{}, internalErr).Times(1)
		resp, err := th.RecordActivityTaskHeartbeatByID(ctx, &shared.RecordActivityTaskHeartbeatByIDRequest{})
		assert.Equal(t, shared.RecordActivityTaskHeartbeatResponse{CancelRequested: common.BoolPtr(false)}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RegisterDomain", func(t *testing.T) {
		h.EXPECT().RegisterDomain(ctx, &types.RegisterDomainRequest{}).Return(internalErr).Times(1)
		err := th.RegisterDomain(ctx, &shared.RegisterDomainRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RequestCancelWorkflowExecution", func(t *testing.T) {
		h.EXPECT().RequestCancelWorkflowExecution(ctx, &types.RequestCancelWorkflowExecutionRequest{}).Return(internalErr).Times(1)
		err := th.RequestCancelWorkflowExecution(ctx, &shared.RequestCancelWorkflowExecutionRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ResetStickyTaskList", func(t *testing.T) {
		h.EXPECT().ResetStickyTaskList(ctx, &types.ResetStickyTaskListRequest{}).Return(&types.ResetStickyTaskListResponse{}, internalErr).Times(1)
		resp, err := th.ResetStickyTaskList(ctx, &shared.ResetStickyTaskListRequest{})
		assert.Equal(t, shared.ResetStickyTaskListResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ResetWorkflowExecution", func(t *testing.T) {
		h.EXPECT().ResetWorkflowExecution(ctx, &types.ResetWorkflowExecutionRequest{}).Return(&types.ResetWorkflowExecutionResponse{}, internalErr).Times(1)
		resp, err := th.ResetWorkflowExecution(ctx, &shared.ResetWorkflowExecutionRequest{})
		assert.Equal(t, shared.ResetWorkflowExecutionResponse{RunId: common.StringPtr("")}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondActivityTaskCanceled", func(t *testing.T) {
		h.EXPECT().RespondActivityTaskCanceled(ctx, &types.RespondActivityTaskCanceledRequest{}).Return(internalErr).Times(1)
		err := th.RespondActivityTaskCanceled(ctx, &shared.RespondActivityTaskCanceledRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondActivityTaskCanceledByID", func(t *testing.T) {
		h.EXPECT().RespondActivityTaskCanceledByID(ctx, &types.RespondActivityTaskCanceledByIDRequest{}).Return(internalErr).Times(1)
		err := th.RespondActivityTaskCanceledByID(ctx, &shared.RespondActivityTaskCanceledByIDRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondActivityTaskCompleted", func(t *testing.T) {
		h.EXPECT().RespondActivityTaskCompleted(ctx, &types.RespondActivityTaskCompletedRequest{}).Return(internalErr).Times(1)
		err := th.RespondActivityTaskCompleted(ctx, &shared.RespondActivityTaskCompletedRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondActivityTaskCompletedByID", func(t *testing.T) {
		h.EXPECT().RespondActivityTaskCompletedByID(ctx, &types.RespondActivityTaskCompletedByIDRequest{}).Return(internalErr).Times(1)
		err := th.RespondActivityTaskCompletedByID(ctx, &shared.RespondActivityTaskCompletedByIDRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondActivityTaskFailed", func(t *testing.T) {
		h.EXPECT().RespondActivityTaskFailed(ctx, &types.RespondActivityTaskFailedRequest{}).Return(internalErr).Times(1)
		err := th.RespondActivityTaskFailed(ctx, &shared.RespondActivityTaskFailedRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondActivityTaskFailedByID", func(t *testing.T) {
		h.EXPECT().RespondActivityTaskFailedByID(ctx, &types.RespondActivityTaskFailedByIDRequest{}).Return(internalErr).Times(1)
		err := th.RespondActivityTaskFailedByID(ctx, &shared.RespondActivityTaskFailedByIDRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondDecisionTaskCompleted", func(t *testing.T) {
		h.EXPECT().RespondDecisionTaskCompleted(ctx, &types.RespondDecisionTaskCompletedRequest{}).Return(&types.RespondDecisionTaskCompletedResponse{}, internalErr).Times(1)
		resp, err := th.RespondDecisionTaskCompleted(ctx, &shared.RespondDecisionTaskCompletedRequest{})
		assert.Equal(t, shared.RespondDecisionTaskCompletedResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondDecisionTaskFailed", func(t *testing.T) {
		h.EXPECT().RespondDecisionTaskFailed(ctx, &types.RespondDecisionTaskFailedRequest{}).Return(internalErr).Times(1)
		err := th.RespondDecisionTaskFailed(ctx, &shared.RespondDecisionTaskFailedRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("RespondQueryTaskCompleted", func(t *testing.T) {
		h.EXPECT().RespondQueryTaskCompleted(ctx, &types.RespondQueryTaskCompletedRequest{}).Return(internalErr).Times(1)
		err := th.RespondQueryTaskCompleted(ctx, &shared.RespondQueryTaskCompletedRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("ScanWorkflowExecutions", func(t *testing.T) {
		h.EXPECT().ScanWorkflowExecutions(ctx, &types.ListWorkflowExecutionsRequest{}).Return(&types.ListWorkflowExecutionsResponse{}, internalErr).Times(1)
		resp, err := th.ScanWorkflowExecutions(ctx, &shared.ListWorkflowExecutionsRequest{})
		assert.Equal(t, shared.ListWorkflowExecutionsResponse{}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("SignalWithStartWorkflowExecution", func(t *testing.T) {
		h.EXPECT().SignalWithStartWorkflowExecution(ctx, &types.SignalWithStartWorkflowExecutionRequest{}).Return(&types.StartWorkflowExecutionResponse{}, internalErr).Times(1)
		resp, err := th.SignalWithStartWorkflowExecution(ctx, &shared.SignalWithStartWorkflowExecutionRequest{})
		assert.Equal(t, shared.StartWorkflowExecutionResponse{RunId: common.StringPtr("")}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("SignalWorkflowExecution", func(t *testing.T) {
		h.EXPECT().SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{}).Return(internalErr).Times(1)
		err := th.SignalWorkflowExecution(ctx, &shared.SignalWorkflowExecutionRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("StartWorkflowExecution", func(t *testing.T) {
		h.EXPECT().StartWorkflowExecution(ctx, &types.StartWorkflowExecutionRequest{}).Return(&types.StartWorkflowExecutionResponse{}, internalErr).Times(1)
		resp, err := th.StartWorkflowExecution(ctx, &shared.StartWorkflowExecutionRequest{})
		assert.Equal(t, shared.StartWorkflowExecutionResponse{RunId: common.StringPtr("")}, *resp)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("TerminateWorkflowExecution", func(t *testing.T) {
		h.EXPECT().TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{}).Return(internalErr).Times(1)
		err := th.TerminateWorkflowExecution(ctx, &shared.TerminateWorkflowExecutionRequest{})
		assert.Equal(t, expectedErr, err)
	})
	t.Run("UpdateDomain", func(t *testing.T) {
		h.EXPECT().UpdateDomain(ctx, &types.UpdateDomainRequest{}).Return(&types.UpdateDomainResponse{}, internalErr).Times(1)
		resp, err := th.UpdateDomain(ctx, &shared.UpdateDomainRequest{})
		assert.Equal(t, shared.UpdateDomainResponse{IsGlobalDomain: common.BoolPtr(false), FailoverVersion: common.Int64Ptr(0)}, *resp)
		assert.Equal(t, expectedErr, err)
	})
}
