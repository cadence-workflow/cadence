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

package retryable

// Code generated by gowrap. DO NOT EDIT.
// template: ../../templates/retry.tmpl
// gowrap: http://github.com/hexdigest/gowrap

import (
	"context"

	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/types"
)

// adminClient implements admin.Client interface instrumented with retries
type adminClient struct {
	client        admin.Client
	throttleRetry *backoff.ThrottleRetry
}

// NewAdminClient creates a new instance of adminClient with retry policy
func NewAdminClient(client admin.Client, policy backoff.RetryPolicy, isRetryable backoff.IsRetryable) admin.Client {
	return &adminClient{
		client: client,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(policy),
			backoff.WithRetryableError(isRetryable),
		),
	}
}

func (c *adminClient) AddSearchAttribute(ctx context.Context, ap1 *types.AddSearchAttributeRequest, p1 ...yarpc.CallOption) (err error) {
	op := func() error {
		return c.client.AddSearchAttribute(ctx, ap1, p1...)
	}
	return c.throttleRetry.Do(ctx, op)
}

func (c *adminClient) CloseShard(ctx context.Context, cp1 *types.CloseShardRequest, p1 ...yarpc.CallOption) (err error) {
	op := func() error {
		return c.client.CloseShard(ctx, cp1, p1...)
	}
	return c.throttleRetry.Do(ctx, op)
}

func (c *adminClient) CountDLQMessages(ctx context.Context, cp1 *types.CountDLQMessagesRequest, p1 ...yarpc.CallOption) (cp2 *types.CountDLQMessagesResponse, err error) {
	var resp *types.CountDLQMessagesResponse
	op := func() error {
		var err error
		resp, err = c.client.CountDLQMessages(ctx, cp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) DeleteWorkflow(ctx context.Context, ap1 *types.AdminDeleteWorkflowRequest, p1 ...yarpc.CallOption) (ap2 *types.AdminDeleteWorkflowResponse, err error) {
	var resp *types.AdminDeleteWorkflowResponse
	op := func() error {
		var err error
		resp, err = c.client.DeleteWorkflow(ctx, ap1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) DescribeCluster(ctx context.Context, p1 ...yarpc.CallOption) (dp1 *types.DescribeClusterResponse, err error) {
	var resp *types.DescribeClusterResponse
	op := func() error {
		var err error
		resp, err = c.client.DescribeCluster(ctx, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) DescribeHistoryHost(ctx context.Context, dp1 *types.DescribeHistoryHostRequest, p1 ...yarpc.CallOption) (dp2 *types.DescribeHistoryHostResponse, err error) {
	var resp *types.DescribeHistoryHostResponse
	op := func() error {
		var err error
		resp, err = c.client.DescribeHistoryHost(ctx, dp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) DescribeQueue(ctx context.Context, dp1 *types.DescribeQueueRequest, p1 ...yarpc.CallOption) (dp2 *types.DescribeQueueResponse, err error) {
	var resp *types.DescribeQueueResponse
	op := func() error {
		var err error
		resp, err = c.client.DescribeQueue(ctx, dp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) DescribeShardDistribution(ctx context.Context, dp1 *types.DescribeShardDistributionRequest, p1 ...yarpc.CallOption) (dp2 *types.DescribeShardDistributionResponse, err error) {
	var resp *types.DescribeShardDistributionResponse
	op := func() error {
		var err error
		resp, err = c.client.DescribeShardDistribution(ctx, dp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) DescribeWorkflowExecution(ctx context.Context, ap1 *types.AdminDescribeWorkflowExecutionRequest, p1 ...yarpc.CallOption) (ap2 *types.AdminDescribeWorkflowExecutionResponse, err error) {
	var resp *types.AdminDescribeWorkflowExecutionResponse
	op := func() error {
		var err error
		resp, err = c.client.DescribeWorkflowExecution(ctx, ap1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) GetDLQReplicationMessages(ctx context.Context, gp1 *types.GetDLQReplicationMessagesRequest, p1 ...yarpc.CallOption) (gp2 *types.GetDLQReplicationMessagesResponse, err error) {
	var resp *types.GetDLQReplicationMessagesResponse
	op := func() error {
		var err error
		resp, err = c.client.GetDLQReplicationMessages(ctx, gp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) GetDomainAsyncWorkflowConfiguraton(ctx context.Context, request *types.GetDomainAsyncWorkflowConfiguratonRequest, opts ...yarpc.CallOption) (gp1 *types.GetDomainAsyncWorkflowConfiguratonResponse, err error) {
	var resp *types.GetDomainAsyncWorkflowConfiguratonResponse
	op := func() error {
		var err error
		resp, err = c.client.GetDomainAsyncWorkflowConfiguraton(ctx, request, opts...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) GetDomainIsolationGroups(ctx context.Context, request *types.GetDomainIsolationGroupsRequest, opts ...yarpc.CallOption) (gp1 *types.GetDomainIsolationGroupsResponse, err error) {
	var resp *types.GetDomainIsolationGroupsResponse
	op := func() error {
		var err error
		resp, err = c.client.GetDomainIsolationGroups(ctx, request, opts...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) GetDomainReplicationMessages(ctx context.Context, gp1 *types.GetDomainReplicationMessagesRequest, p1 ...yarpc.CallOption) (gp2 *types.GetDomainReplicationMessagesResponse, err error) {
	var resp *types.GetDomainReplicationMessagesResponse
	op := func() error {
		var err error
		resp, err = c.client.GetDomainReplicationMessages(ctx, gp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) GetDynamicConfig(ctx context.Context, gp1 *types.GetDynamicConfigRequest, p1 ...yarpc.CallOption) (gp2 *types.GetDynamicConfigResponse, err error) {
	var resp *types.GetDynamicConfigResponse
	op := func() error {
		var err error
		resp, err = c.client.GetDynamicConfig(ctx, gp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) GetGlobalIsolationGroups(ctx context.Context, request *types.GetGlobalIsolationGroupsRequest, opts ...yarpc.CallOption) (gp1 *types.GetGlobalIsolationGroupsResponse, err error) {
	var resp *types.GetGlobalIsolationGroupsResponse
	op := func() error {
		var err error
		resp, err = c.client.GetGlobalIsolationGroups(ctx, request, opts...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) GetReplicationMessages(ctx context.Context, gp1 *types.GetReplicationMessagesRequest, p1 ...yarpc.CallOption) (gp2 *types.GetReplicationMessagesResponse, err error) {
	var resp *types.GetReplicationMessagesResponse
	op := func() error {
		var err error
		resp, err = c.client.GetReplicationMessages(ctx, gp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) GetWorkflowExecutionRawHistoryV2(ctx context.Context, gp1 *types.GetWorkflowExecutionRawHistoryV2Request, p1 ...yarpc.CallOption) (gp2 *types.GetWorkflowExecutionRawHistoryV2Response, err error) {
	var resp *types.GetWorkflowExecutionRawHistoryV2Response
	op := func() error {
		var err error
		resp, err = c.client.GetWorkflowExecutionRawHistoryV2(ctx, gp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) ListDynamicConfig(ctx context.Context, lp1 *types.ListDynamicConfigRequest, p1 ...yarpc.CallOption) (lp2 *types.ListDynamicConfigResponse, err error) {
	var resp *types.ListDynamicConfigResponse
	op := func() error {
		var err error
		resp, err = c.client.ListDynamicConfig(ctx, lp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) MaintainCorruptWorkflow(ctx context.Context, ap1 *types.AdminMaintainWorkflowRequest, p1 ...yarpc.CallOption) (ap2 *types.AdminMaintainWorkflowResponse, err error) {
	var resp *types.AdminMaintainWorkflowResponse
	op := func() error {
		var err error
		resp, err = c.client.MaintainCorruptWorkflow(ctx, ap1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) MergeDLQMessages(ctx context.Context, mp1 *types.MergeDLQMessagesRequest, p1 ...yarpc.CallOption) (mp2 *types.MergeDLQMessagesResponse, err error) {
	var resp *types.MergeDLQMessagesResponse
	op := func() error {
		var err error
		resp, err = c.client.MergeDLQMessages(ctx, mp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) PurgeDLQMessages(ctx context.Context, pp1 *types.PurgeDLQMessagesRequest, p1 ...yarpc.CallOption) (err error) {
	op := func() error {
		return c.client.PurgeDLQMessages(ctx, pp1, p1...)
	}
	return c.throttleRetry.Do(ctx, op)
}

func (c *adminClient) ReadDLQMessages(ctx context.Context, rp1 *types.ReadDLQMessagesRequest, p1 ...yarpc.CallOption) (rp2 *types.ReadDLQMessagesResponse, err error) {
	var resp *types.ReadDLQMessagesResponse
	op := func() error {
		var err error
		resp, err = c.client.ReadDLQMessages(ctx, rp1, p1...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) ReapplyEvents(ctx context.Context, rp1 *types.ReapplyEventsRequest, p1 ...yarpc.CallOption) (err error) {
	op := func() error {
		return c.client.ReapplyEvents(ctx, rp1, p1...)
	}
	return c.throttleRetry.Do(ctx, op)
}

func (c *adminClient) RefreshWorkflowTasks(ctx context.Context, rp1 *types.RefreshWorkflowTasksRequest, p1 ...yarpc.CallOption) (err error) {
	op := func() error {
		return c.client.RefreshWorkflowTasks(ctx, rp1, p1...)
	}
	return c.throttleRetry.Do(ctx, op)
}

func (c *adminClient) RemoveTask(ctx context.Context, rp1 *types.RemoveTaskRequest, p1 ...yarpc.CallOption) (err error) {
	op := func() error {
		return c.client.RemoveTask(ctx, rp1, p1...)
	}
	return c.throttleRetry.Do(ctx, op)
}

func (c *adminClient) ResendReplicationTasks(ctx context.Context, rp1 *types.ResendReplicationTasksRequest, p1 ...yarpc.CallOption) (err error) {
	op := func() error {
		return c.client.ResendReplicationTasks(ctx, rp1, p1...)
	}
	return c.throttleRetry.Do(ctx, op)
}

func (c *adminClient) ResetQueue(ctx context.Context, rp1 *types.ResetQueueRequest, p1 ...yarpc.CallOption) (err error) {
	op := func() error {
		return c.client.ResetQueue(ctx, rp1, p1...)
	}
	return c.throttleRetry.Do(ctx, op)
}

func (c *adminClient) RestoreDynamicConfig(ctx context.Context, rp1 *types.RestoreDynamicConfigRequest, p1 ...yarpc.CallOption) (err error) {
	op := func() error {
		return c.client.RestoreDynamicConfig(ctx, rp1, p1...)
	}
	return c.throttleRetry.Do(ctx, op)
}

func (c *adminClient) UpdateDomainAsyncWorkflowConfiguraton(ctx context.Context, request *types.UpdateDomainAsyncWorkflowConfiguratonRequest, opts ...yarpc.CallOption) (up1 *types.UpdateDomainAsyncWorkflowConfiguratonResponse, err error) {
	var resp *types.UpdateDomainAsyncWorkflowConfiguratonResponse
	op := func() error {
		var err error
		resp, err = c.client.UpdateDomainAsyncWorkflowConfiguraton(ctx, request, opts...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) UpdateDomainIsolationGroups(ctx context.Context, request *types.UpdateDomainIsolationGroupsRequest, opts ...yarpc.CallOption) (up1 *types.UpdateDomainIsolationGroupsResponse, err error) {
	var resp *types.UpdateDomainIsolationGroupsResponse
	op := func() error {
		var err error
		resp, err = c.client.UpdateDomainIsolationGroups(ctx, request, opts...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) UpdateDynamicConfig(ctx context.Context, up1 *types.UpdateDynamicConfigRequest, p1 ...yarpc.CallOption) (err error) {
	op := func() error {
		return c.client.UpdateDynamicConfig(ctx, up1, p1...)
	}
	return c.throttleRetry.Do(ctx, op)
}

func (c *adminClient) UpdateGlobalIsolationGroups(ctx context.Context, request *types.UpdateGlobalIsolationGroupsRequest, opts ...yarpc.CallOption) (up1 *types.UpdateGlobalIsolationGroupsResponse, err error) {
	var resp *types.UpdateGlobalIsolationGroupsResponse
	op := func() error {
		var err error
		resp, err = c.client.UpdateGlobalIsolationGroups(ctx, request, opts...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}

func (c *adminClient) UpdateTaskListPartitionConfig(ctx context.Context, request *types.UpdateTaskListPartitionConfigRequest, opts ...yarpc.CallOption) (up1 *types.UpdateTaskListPartitionConfigResponse, err error) {
	var resp *types.UpdateTaskListPartitionConfigResponse
	op := func() error {
		var err error
		resp, err = c.client.UpdateTaskListPartitionConfig(ctx, request, opts...)
		return err
	}
	err = c.throttleRetry.Do(ctx, op)
	return resp, err
}
