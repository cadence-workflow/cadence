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

// Code generated by gowrap. DO NOT EDIT.
// template: ../../templates/metered.tmpl
// gowrap: http://github.com/hexdigest/gowrap

import (
	"context"

	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

// frontendClient implements frontend.Client interface instrumented with retries
type frontendClient struct {
	client        frontend.Client
	metricsClient metrics.Client
}

// NewFrontendClient creates a new instance of frontendClient with retry policy
func NewFrontendClient(client frontend.Client, metricsClient metrics.Client) frontend.Client {
	return &frontendClient{
		client:        client,
		metricsClient: metricsClient,
	}
}

func (c *frontendClient) CountWorkflowExecutions(ctx context.Context, cp1 *types.CountWorkflowExecutionsRequest, p1 ...yarpc.CallOption) (cp2 *types.CountWorkflowExecutionsResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientCountWorkflowExecutionsScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientCountWorkflowExecutionsScope, metrics.CadenceClientLatency)
	cp2, err = c.client.CountWorkflowExecutions(ctx, cp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientCountWorkflowExecutionsScope, metrics.CadenceClientFailures)
	}
	return cp2, err
}

func (c *frontendClient) DeprecateDomain(ctx context.Context, dp1 *types.DeprecateDomainRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientDeprecateDomainScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientDeprecateDomainScope, metrics.CadenceClientLatency)
	err = c.client.DeprecateDomain(ctx, dp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientDeprecateDomainScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) DescribeDomain(ctx context.Context, dp1 *types.DescribeDomainRequest, p1 ...yarpc.CallOption) (dp2 *types.DescribeDomainResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientDescribeDomainScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientDescribeDomainScope, metrics.CadenceClientLatency)
	dp2, err = c.client.DescribeDomain(ctx, dp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientDescribeDomainScope, metrics.CadenceClientFailures)
	}
	return dp2, err
}

func (c *frontendClient) DescribeTaskList(ctx context.Context, dp1 *types.DescribeTaskListRequest, p1 ...yarpc.CallOption) (dp2 *types.DescribeTaskListResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientDescribeTaskListScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientDescribeTaskListScope, metrics.CadenceClientLatency)
	dp2, err = c.client.DescribeTaskList(ctx, dp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientDescribeTaskListScope, metrics.CadenceClientFailures)
	}
	return dp2, err
}

func (c *frontendClient) DescribeWorkflowExecution(ctx context.Context, dp1 *types.DescribeWorkflowExecutionRequest, p1 ...yarpc.CallOption) (dp2 *types.DescribeWorkflowExecutionResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientDescribeWorkflowExecutionScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientDescribeWorkflowExecutionScope, metrics.CadenceClientLatency)
	dp2, err = c.client.DescribeWorkflowExecution(ctx, dp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientDescribeWorkflowExecutionScope, metrics.CadenceClientFailures)
	}
	return dp2, err
}

func (c *frontendClient) GetClusterInfo(ctx context.Context, p1 ...yarpc.CallOption) (cp1 *types.ClusterInfo, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientGetClusterInfoScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientGetClusterInfoScope, metrics.CadenceClientLatency)
	cp1, err = c.client.GetClusterInfo(ctx, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientGetClusterInfoScope, metrics.CadenceClientFailures)
	}
	return cp1, err
}

func (c *frontendClient) GetSearchAttributes(ctx context.Context, p1 ...yarpc.CallOption) (gp1 *types.GetSearchAttributesResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientGetSearchAttributesScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientGetSearchAttributesScope, metrics.CadenceClientLatency)
	gp1, err = c.client.GetSearchAttributes(ctx, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientGetSearchAttributesScope, metrics.CadenceClientFailures)
	}
	return gp1, err
}

func (c *frontendClient) GetTaskListsByDomain(ctx context.Context, gp1 *types.GetTaskListsByDomainRequest, p1 ...yarpc.CallOption) (gp2 *types.GetTaskListsByDomainResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientGetTaskListsByDomainScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientGetTaskListsByDomainScope, metrics.CadenceClientLatency)
	gp2, err = c.client.GetTaskListsByDomain(ctx, gp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientGetTaskListsByDomainScope, metrics.CadenceClientFailures)
	}
	return gp2, err
}

func (c *frontendClient) GetWorkflowExecutionHistory(ctx context.Context, gp1 *types.GetWorkflowExecutionHistoryRequest, p1 ...yarpc.CallOption) (gp2 *types.GetWorkflowExecutionHistoryResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientGetWorkflowExecutionHistoryScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientGetWorkflowExecutionHistoryScope, metrics.CadenceClientLatency)
	gp2, err = c.client.GetWorkflowExecutionHistory(ctx, gp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientGetWorkflowExecutionHistoryScope, metrics.CadenceClientFailures)
	}
	return gp2, err
}

func (c *frontendClient) ListArchivedWorkflowExecutions(ctx context.Context, lp1 *types.ListArchivedWorkflowExecutionsRequest, p1 ...yarpc.CallOption) (lp2 *types.ListArchivedWorkflowExecutionsResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientListArchivedWorkflowExecutionsScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientListArchivedWorkflowExecutionsScope, metrics.CadenceClientLatency)
	lp2, err = c.client.ListArchivedWorkflowExecutions(ctx, lp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientListArchivedWorkflowExecutionsScope, metrics.CadenceClientFailures)
	}
	return lp2, err
}

func (c *frontendClient) ListClosedWorkflowExecutions(ctx context.Context, lp1 *types.ListClosedWorkflowExecutionsRequest, p1 ...yarpc.CallOption) (lp2 *types.ListClosedWorkflowExecutionsResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientListClosedWorkflowExecutionsScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientListClosedWorkflowExecutionsScope, metrics.CadenceClientLatency)
	lp2, err = c.client.ListClosedWorkflowExecutions(ctx, lp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientListClosedWorkflowExecutionsScope, metrics.CadenceClientFailures)
	}
	return lp2, err
}

func (c *frontendClient) ListDomains(ctx context.Context, lp1 *types.ListDomainsRequest, p1 ...yarpc.CallOption) (lp2 *types.ListDomainsResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientListDomainsScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientListDomainsScope, metrics.CadenceClientLatency)
	lp2, err = c.client.ListDomains(ctx, lp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientListDomainsScope, metrics.CadenceClientFailures)
	}
	return lp2, err
}

func (c *frontendClient) ListOpenWorkflowExecutions(ctx context.Context, lp1 *types.ListOpenWorkflowExecutionsRequest, p1 ...yarpc.CallOption) (lp2 *types.ListOpenWorkflowExecutionsResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientListOpenWorkflowExecutionsScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientListOpenWorkflowExecutionsScope, metrics.CadenceClientLatency)
	lp2, err = c.client.ListOpenWorkflowExecutions(ctx, lp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientListOpenWorkflowExecutionsScope, metrics.CadenceClientFailures)
	}
	return lp2, err
}

func (c *frontendClient) ListTaskListPartitions(ctx context.Context, lp1 *types.ListTaskListPartitionsRequest, p1 ...yarpc.CallOption) (lp2 *types.ListTaskListPartitionsResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientListTaskListPartitionsScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientListTaskListPartitionsScope, metrics.CadenceClientLatency)
	lp2, err = c.client.ListTaskListPartitions(ctx, lp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientListTaskListPartitionsScope, metrics.CadenceClientFailures)
	}
	return lp2, err
}

func (c *frontendClient) ListWorkflowExecutions(ctx context.Context, lp1 *types.ListWorkflowExecutionsRequest, p1 ...yarpc.CallOption) (lp2 *types.ListWorkflowExecutionsResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientListWorkflowExecutionsScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientListWorkflowExecutionsScope, metrics.CadenceClientLatency)
	lp2, err = c.client.ListWorkflowExecutions(ctx, lp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientListWorkflowExecutionsScope, metrics.CadenceClientFailures)
	}
	return lp2, err
}

func (c *frontendClient) PollForActivityTask(ctx context.Context, pp1 *types.PollForActivityTaskRequest, p1 ...yarpc.CallOption) (pp2 *types.PollForActivityTaskResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientPollForActivityTaskScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientPollForActivityTaskScope, metrics.CadenceClientLatency)
	pp2, err = c.client.PollForActivityTask(ctx, pp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientPollForActivityTaskScope, metrics.CadenceClientFailures)
	}
	return pp2, err
}

func (c *frontendClient) PollForDecisionTask(ctx context.Context, pp1 *types.PollForDecisionTaskRequest, p1 ...yarpc.CallOption) (pp2 *types.PollForDecisionTaskResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientPollForDecisionTaskScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientPollForDecisionTaskScope, metrics.CadenceClientLatency)
	pp2, err = c.client.PollForDecisionTask(ctx, pp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientPollForDecisionTaskScope, metrics.CadenceClientFailures)
	}
	return pp2, err
}

func (c *frontendClient) QueryWorkflow(ctx context.Context, qp1 *types.QueryWorkflowRequest, p1 ...yarpc.CallOption) (qp2 *types.QueryWorkflowResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientQueryWorkflowScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientQueryWorkflowScope, metrics.CadenceClientLatency)
	qp2, err = c.client.QueryWorkflow(ctx, qp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientQueryWorkflowScope, metrics.CadenceClientFailures)
	}
	return qp2, err
}

func (c *frontendClient) RecordActivityTaskHeartbeat(ctx context.Context, rp1 *types.RecordActivityTaskHeartbeatRequest, p1 ...yarpc.CallOption) (rp2 *types.RecordActivityTaskHeartbeatResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRecordActivityTaskHeartbeatScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRecordActivityTaskHeartbeatScope, metrics.CadenceClientLatency)
	rp2, err = c.client.RecordActivityTaskHeartbeat(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRecordActivityTaskHeartbeatScope, metrics.CadenceClientFailures)
	}
	return rp2, err
}

func (c *frontendClient) RecordActivityTaskHeartbeatByID(ctx context.Context, rp1 *types.RecordActivityTaskHeartbeatByIDRequest, p1 ...yarpc.CallOption) (rp2 *types.RecordActivityTaskHeartbeatResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRecordActivityTaskHeartbeatByIDScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRecordActivityTaskHeartbeatByIDScope, metrics.CadenceClientLatency)
	rp2, err = c.client.RecordActivityTaskHeartbeatByID(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRecordActivityTaskHeartbeatByIDScope, metrics.CadenceClientFailures)
	}
	return rp2, err
}

func (c *frontendClient) RefreshWorkflowTasks(ctx context.Context, rp1 *types.RefreshWorkflowTasksRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRefreshWorkflowTasksScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRefreshWorkflowTasksScope, metrics.CadenceClientLatency)
	err = c.client.RefreshWorkflowTasks(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRefreshWorkflowTasksScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) RegisterDomain(ctx context.Context, rp1 *types.RegisterDomainRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRegisterDomainScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRegisterDomainScope, metrics.CadenceClientLatency)
	err = c.client.RegisterDomain(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRegisterDomainScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) RequestCancelWorkflowExecution(ctx context.Context, rp1 *types.RequestCancelWorkflowExecutionRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRequestCancelWorkflowExecutionScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRequestCancelWorkflowExecutionScope, metrics.CadenceClientLatency)
	err = c.client.RequestCancelWorkflowExecution(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRequestCancelWorkflowExecutionScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) ResetStickyTaskList(ctx context.Context, rp1 *types.ResetStickyTaskListRequest, p1 ...yarpc.CallOption) (rp2 *types.ResetStickyTaskListResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientResetStickyTaskListScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientResetStickyTaskListScope, metrics.CadenceClientLatency)
	rp2, err = c.client.ResetStickyTaskList(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientResetStickyTaskListScope, metrics.CadenceClientFailures)
	}
	return rp2, err
}

func (c *frontendClient) ResetWorkflowExecution(ctx context.Context, rp1 *types.ResetWorkflowExecutionRequest, p1 ...yarpc.CallOption) (rp2 *types.ResetWorkflowExecutionResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientResetWorkflowExecutionScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientResetWorkflowExecutionScope, metrics.CadenceClientLatency)
	rp2, err = c.client.ResetWorkflowExecution(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientResetWorkflowExecutionScope, metrics.CadenceClientFailures)
	}
	return rp2, err
}

func (c *frontendClient) RespondActivityTaskCanceled(ctx context.Context, rp1 *types.RespondActivityTaskCanceledRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskCanceledScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRespondActivityTaskCanceledScope, metrics.CadenceClientLatency)
	err = c.client.RespondActivityTaskCanceled(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskCanceledScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) RespondActivityTaskCanceledByID(ctx context.Context, rp1 *types.RespondActivityTaskCanceledByIDRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskCanceledByIDScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRespondActivityTaskCanceledByIDScope, metrics.CadenceClientLatency)
	err = c.client.RespondActivityTaskCanceledByID(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskCanceledByIDScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) RespondActivityTaskCompleted(ctx context.Context, rp1 *types.RespondActivityTaskCompletedRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskCompletedScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRespondActivityTaskCompletedScope, metrics.CadenceClientLatency)
	err = c.client.RespondActivityTaskCompleted(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskCompletedScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) RespondActivityTaskCompletedByID(ctx context.Context, rp1 *types.RespondActivityTaskCompletedByIDRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskCompletedByIDScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRespondActivityTaskCompletedByIDScope, metrics.CadenceClientLatency)
	err = c.client.RespondActivityTaskCompletedByID(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskCompletedByIDScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) RespondActivityTaskFailed(ctx context.Context, rp1 *types.RespondActivityTaskFailedRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskFailedScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRespondActivityTaskFailedScope, metrics.CadenceClientLatency)
	err = c.client.RespondActivityTaskFailed(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskFailedScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) RespondActivityTaskFailedByID(ctx context.Context, rp1 *types.RespondActivityTaskFailedByIDRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskFailedByIDScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRespondActivityTaskFailedByIDScope, metrics.CadenceClientLatency)
	err = c.client.RespondActivityTaskFailedByID(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRespondActivityTaskFailedByIDScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) RespondDecisionTaskCompleted(ctx context.Context, rp1 *types.RespondDecisionTaskCompletedRequest, p1 ...yarpc.CallOption) (rp2 *types.RespondDecisionTaskCompletedResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRespondDecisionTaskCompletedScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRespondDecisionTaskCompletedScope, metrics.CadenceClientLatency)
	rp2, err = c.client.RespondDecisionTaskCompleted(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRespondDecisionTaskCompletedScope, metrics.CadenceClientFailures)
	}
	return rp2, err
}

func (c *frontendClient) RespondDecisionTaskFailed(ctx context.Context, rp1 *types.RespondDecisionTaskFailedRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRespondDecisionTaskFailedScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRespondDecisionTaskFailedScope, metrics.CadenceClientLatency)
	err = c.client.RespondDecisionTaskFailed(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRespondDecisionTaskFailedScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) RespondQueryTaskCompleted(ctx context.Context, rp1 *types.RespondQueryTaskCompletedRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRespondQueryTaskCompletedScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRespondQueryTaskCompletedScope, metrics.CadenceClientLatency)
	err = c.client.RespondQueryTaskCompleted(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRespondQueryTaskCompletedScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) RestartWorkflowExecution(ctx context.Context, rp1 *types.RestartWorkflowExecutionRequest, p1 ...yarpc.CallOption) (rp2 *types.RestartWorkflowExecutionResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientRestartWorkflowExecutionScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientRestartWorkflowExecutionScope, metrics.CadenceClientLatency)
	rp2, err = c.client.RestartWorkflowExecution(ctx, rp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientRestartWorkflowExecutionScope, metrics.CadenceClientFailures)
	}
	return rp2, err
}

func (c *frontendClient) ScanWorkflowExecutions(ctx context.Context, lp1 *types.ListWorkflowExecutionsRequest, p1 ...yarpc.CallOption) (lp2 *types.ListWorkflowExecutionsResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientScanWorkflowExecutionsScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientScanWorkflowExecutionsScope, metrics.CadenceClientLatency)
	lp2, err = c.client.ScanWorkflowExecutions(ctx, lp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientScanWorkflowExecutionsScope, metrics.CadenceClientFailures)
	}
	return lp2, err
}

func (c *frontendClient) SignalWithStartWorkflowExecution(ctx context.Context, sp1 *types.SignalWithStartWorkflowExecutionRequest, p1 ...yarpc.CallOption) (sp2 *types.StartWorkflowExecutionResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientSignalWithStartWorkflowExecutionScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientSignalWithStartWorkflowExecutionScope, metrics.CadenceClientLatency)
	sp2, err = c.client.SignalWithStartWorkflowExecution(ctx, sp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientSignalWithStartWorkflowExecutionScope, metrics.CadenceClientFailures)
	}
	return sp2, err
}

func (c *frontendClient) SignalWorkflowExecution(ctx context.Context, sp1 *types.SignalWorkflowExecutionRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientSignalWorkflowExecutionScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientSignalWorkflowExecutionScope, metrics.CadenceClientLatency)
	err = c.client.SignalWorkflowExecution(ctx, sp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientSignalWorkflowExecutionScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) StartWorkflowExecution(ctx context.Context, sp1 *types.StartWorkflowExecutionRequest, p1 ...yarpc.CallOption) (sp2 *types.StartWorkflowExecutionResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientStartWorkflowExecutionScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientStartWorkflowExecutionScope, metrics.CadenceClientLatency)
	sp2, err = c.client.StartWorkflowExecution(ctx, sp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientStartWorkflowExecutionScope, metrics.CadenceClientFailures)
	}
	return sp2, err
}

func (c *frontendClient) TerminateWorkflowExecution(ctx context.Context, tp1 *types.TerminateWorkflowExecutionRequest, p1 ...yarpc.CallOption) (err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientTerminateWorkflowExecutionScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientTerminateWorkflowExecutionScope, metrics.CadenceClientLatency)
	err = c.client.TerminateWorkflowExecution(ctx, tp1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientTerminateWorkflowExecutionScope, metrics.CadenceClientFailures)
	}
	return err
}

func (c *frontendClient) UpdateDomain(ctx context.Context, up1 *types.UpdateDomainRequest, p1 ...yarpc.CallOption) (up2 *types.UpdateDomainResponse, err error) {
	c.metricsClient.IncCounter(metrics.FrontendClientUpdateDomainScope, metrics.CadenceClientRequests)

	sw := c.metricsClient.StartTimer(metrics.FrontendClientUpdateDomainScope, metrics.CadenceClientLatency)
	up2, err = c.client.UpdateDomain(ctx, up1, p1...)
	sw.Stop()

	if err != nil {
		c.metricsClient.IncCounter(metrics.FrontendClientUpdateDomainScope, metrics.CadenceClientFailures)
	}
	return up2, err
}
