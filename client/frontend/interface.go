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

package frontend

import (
	"context"

	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -self_package github.com/uber/cadence/client/frontend
//go:generate gowrap gen -g -p . -i Client -t ../templates/retry.tmpl -o ../wrappers/retryable/frontend_generated.go -v client=Frontend
//go:generate gowrap gen -g -p . -i Client -t ../templates/metered.tmpl -o ../wrappers/metered/frontend_generated.go -v client=Frontend
//go:generate gowrap gen -g -p . -i Client -t ../templates/errorinjectors.tmpl -o ../wrappers/errorinjectors/frontend_generated.go -v client=Frontend
//go:generate gowrap gen -g -p . -i Client -t ../templates/grpc.tmpl -o ../wrappers/grpc/frontend_generated.go -v client=Frontend -v package=apiv1 -v path=github.com/uber/cadence-idl/go/proto/api/v1 -v prefix=
//go:generate gowrap gen -g -p . -i Client -t ../templates/thrift.tmpl -o ../wrappers/thrift/frontend_generated.go -v client=Frontend -v prefix=
//go:generate gowrap gen -g -p . -i Client -t ../templates/timeout.tmpl -o ../wrappers/timeout/frontend_generated.go -v client=Frontend

// Client is the interface exposed by frontend service client
type Client interface {
	CountWorkflowExecutions(context.Context, *types.CountWorkflowExecutionsRequest, ...yarpc.CallOption) (*types.CountWorkflowExecutionsResponse, error)
	DeleteDomain(context.Context, *types.DeleteDomainRequest, ...yarpc.CallOption) error
	DeprecateDomain(context.Context, *types.DeprecateDomainRequest, ...yarpc.CallOption) error
	DescribeDomain(context.Context, *types.DescribeDomainRequest, ...yarpc.CallOption) (*types.DescribeDomainResponse, error)
	DescribeTaskList(context.Context, *types.DescribeTaskListRequest, ...yarpc.CallOption) (*types.DescribeTaskListResponse, error)
	DescribeWorkflowExecution(context.Context, *types.DescribeWorkflowExecutionRequest, ...yarpc.CallOption) (*types.DescribeWorkflowExecutionResponse, error)
	DiagnoseWorkflowExecution(context.Context, *types.DiagnoseWorkflowExecutionRequest, ...yarpc.CallOption) (*types.DiagnoseWorkflowExecutionResponse, error)
	GetClusterInfo(context.Context, ...yarpc.CallOption) (*types.ClusterInfo, error)
	GetSearchAttributes(context.Context, ...yarpc.CallOption) (*types.GetSearchAttributesResponse, error)
	GetWorkflowExecutionHistory(context.Context, *types.GetWorkflowExecutionHistoryRequest, ...yarpc.CallOption) (*types.GetWorkflowExecutionHistoryResponse, error)
	ListArchivedWorkflowExecutions(context.Context, *types.ListArchivedWorkflowExecutionsRequest, ...yarpc.CallOption) (*types.ListArchivedWorkflowExecutionsResponse, error)
	ListClosedWorkflowExecutions(context.Context, *types.ListClosedWorkflowExecutionsRequest, ...yarpc.CallOption) (*types.ListClosedWorkflowExecutionsResponse, error)
	ListDomains(context.Context, *types.ListDomainsRequest, ...yarpc.CallOption) (*types.ListDomainsResponse, error)
	ListOpenWorkflowExecutions(context.Context, *types.ListOpenWorkflowExecutionsRequest, ...yarpc.CallOption) (*types.ListOpenWorkflowExecutionsResponse, error)
	ListTaskListPartitions(context.Context, *types.ListTaskListPartitionsRequest, ...yarpc.CallOption) (*types.ListTaskListPartitionsResponse, error)
	GetTaskListsByDomain(context.Context, *types.GetTaskListsByDomainRequest, ...yarpc.CallOption) (*types.GetTaskListsByDomainResponse, error)
	RefreshWorkflowTasks(context.Context, *types.RefreshWorkflowTasksRequest, ...yarpc.CallOption) error
	ListWorkflowExecutions(context.Context, *types.ListWorkflowExecutionsRequest, ...yarpc.CallOption) (*types.ListWorkflowExecutionsResponse, error)
	PollForActivityTask(context.Context, *types.PollForActivityTaskRequest, ...yarpc.CallOption) (*types.PollForActivityTaskResponse, error)
	PollForDecisionTask(context.Context, *types.PollForDecisionTaskRequest, ...yarpc.CallOption) (*types.PollForDecisionTaskResponse, error)
	QueryWorkflow(context.Context, *types.QueryWorkflowRequest, ...yarpc.CallOption) (*types.QueryWorkflowResponse, error)
	RecordActivityTaskHeartbeat(context.Context, *types.RecordActivityTaskHeartbeatRequest, ...yarpc.CallOption) (*types.RecordActivityTaskHeartbeatResponse, error)
	RecordActivityTaskHeartbeatByID(context.Context, *types.RecordActivityTaskHeartbeatByIDRequest, ...yarpc.CallOption) (*types.RecordActivityTaskHeartbeatResponse, error)
	RegisterDomain(context.Context, *types.RegisterDomainRequest, ...yarpc.CallOption) error
	RequestCancelWorkflowExecution(context.Context, *types.RequestCancelWorkflowExecutionRequest, ...yarpc.CallOption) error
	ResetStickyTaskList(context.Context, *types.ResetStickyTaskListRequest, ...yarpc.CallOption) (*types.ResetStickyTaskListResponse, error)
	ResetWorkflowExecution(context.Context, *types.ResetWorkflowExecutionRequest, ...yarpc.CallOption) (*types.ResetWorkflowExecutionResponse, error)
	RespondActivityTaskCanceled(context.Context, *types.RespondActivityTaskCanceledRequest, ...yarpc.CallOption) error
	RespondActivityTaskCanceledByID(context.Context, *types.RespondActivityTaskCanceledByIDRequest, ...yarpc.CallOption) error
	RespondActivityTaskCompleted(context.Context, *types.RespondActivityTaskCompletedRequest, ...yarpc.CallOption) error
	RespondActivityTaskCompletedByID(context.Context, *types.RespondActivityTaskCompletedByIDRequest, ...yarpc.CallOption) error
	RespondActivityTaskFailed(context.Context, *types.RespondActivityTaskFailedRequest, ...yarpc.CallOption) error
	RespondActivityTaskFailedByID(context.Context, *types.RespondActivityTaskFailedByIDRequest, ...yarpc.CallOption) error
	RespondDecisionTaskCompleted(context.Context, *types.RespondDecisionTaskCompletedRequest, ...yarpc.CallOption) (*types.RespondDecisionTaskCompletedResponse, error)
	RespondDecisionTaskFailed(context.Context, *types.RespondDecisionTaskFailedRequest, ...yarpc.CallOption) error
	RespondQueryTaskCompleted(context.Context, *types.RespondQueryTaskCompletedRequest, ...yarpc.CallOption) error
	RestartWorkflowExecution(context.Context, *types.RestartWorkflowExecutionRequest, ...yarpc.CallOption) (*types.RestartWorkflowExecutionResponse, error)
	ScanWorkflowExecutions(context.Context, *types.ListWorkflowExecutionsRequest, ...yarpc.CallOption) (*types.ListWorkflowExecutionsResponse, error)
	SignalWithStartWorkflowExecution(context.Context, *types.SignalWithStartWorkflowExecutionRequest, ...yarpc.CallOption) (*types.StartWorkflowExecutionResponse, error)
	SignalWithStartWorkflowExecutionAsync(context.Context, *types.SignalWithStartWorkflowExecutionAsyncRequest, ...yarpc.CallOption) (*types.SignalWithStartWorkflowExecutionAsyncResponse, error)
	SignalWorkflowExecution(context.Context, *types.SignalWorkflowExecutionRequest, ...yarpc.CallOption) error
	StartWorkflowExecution(context.Context, *types.StartWorkflowExecutionRequest, ...yarpc.CallOption) (*types.StartWorkflowExecutionResponse, error)
	StartWorkflowExecutionAsync(context.Context, *types.StartWorkflowExecutionAsyncRequest, ...yarpc.CallOption) (*types.StartWorkflowExecutionAsyncResponse, error)
	TerminateWorkflowExecution(context.Context, *types.TerminateWorkflowExecutionRequest, ...yarpc.CallOption) error
	UpdateDomain(context.Context, *types.UpdateDomainRequest, ...yarpc.CallOption) (*types.UpdateDomainResponse, error)
	FailoverDomain(context.Context, *types.FailoverDomainRequest, ...yarpc.CallOption) (*types.FailoverDomainResponse, error)
	ListFailoverHistory(context.Context, *types.ListFailoverHistoryRequest, ...yarpc.CallOption) (*types.ListFailoverHistoryResponse, error)
}
