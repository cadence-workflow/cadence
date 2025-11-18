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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -self_package github.com/uber/cadence/service/frontend/api
//go:generate gowrap gen -g -p . -i Handler -t ../templates/accesscontrolled.tmpl -o ../wrappers/accesscontrolled/api_generated.go -v handler=API
//go:generate gowrap gen -g -p . -i Handler -t ../templates/clusterredirection.tmpl -o ../wrappers/clusterredirection/api_generated.go
//go:generate gowrap gen -g -p . -i Handler -t ../templates/versioncheck.tmpl -o ../wrappers/versioncheck/api_generated.go
//go:generate gowrap gen -g -p . -i Handler -t ../templates/metered.tmpl -o ../wrappers/metered/api_generated.go -v handler=API
//go:generate gowrap gen -g -p . -i Handler -t ../templates/ratelimited.tmpl -o ../wrappers/ratelimited/api_generated.go -v handler=API
//go:generate gowrap gen -g -p . -i Handler -t ../../templates/grpc.tmpl -o ../wrappers/grpc/api_generated.go -v handler=API -v package=apiv1 -v path=github.com/uber/cadence-idl/go/proto/api/v1 -v prefix=
//go:generate gowrap gen -g -p ../../../.gen/go/cadence/workflowserviceserver -i Interface -t ../../templates/thrift.tmpl -o ../wrappers/thrift/api_generated.go -v handler=API -v prefix=

package api

import (
	"context"

	"github.com/uber/cadence/common/types"
)

type (
	// Handler is interface wrapping frontend handler
	Handler interface {
		Health(context.Context) (*types.HealthStatus, error)
		CountWorkflowExecutions(context.Context, *types.CountWorkflowExecutionsRequest) (*types.CountWorkflowExecutionsResponse, error)
		DeleteDomain(context.Context, *types.DeleteDomainRequest) error
		DeprecateDomain(context.Context, *types.DeprecateDomainRequest) error
		DescribeDomain(context.Context, *types.DescribeDomainRequest) (*types.DescribeDomainResponse, error)
		DescribeTaskList(context.Context, *types.DescribeTaskListRequest) (*types.DescribeTaskListResponse, error)
		DescribeWorkflowExecution(context.Context, *types.DescribeWorkflowExecutionRequest) (*types.DescribeWorkflowExecutionResponse, error)
		DiagnoseWorkflowExecution(context.Context, *types.DiagnoseWorkflowExecutionRequest) (*types.DiagnoseWorkflowExecutionResponse, error)
		GetClusterInfo(context.Context) (*types.ClusterInfo, error)
		GetSearchAttributes(context.Context) (*types.GetSearchAttributesResponse, error)
		GetWorkflowExecutionHistory(context.Context, *types.GetWorkflowExecutionHistoryRequest) (*types.GetWorkflowExecutionHistoryResponse, error)
		ListArchivedWorkflowExecutions(context.Context, *types.ListArchivedWorkflowExecutionsRequest) (*types.ListArchivedWorkflowExecutionsResponse, error)
		ListClosedWorkflowExecutions(context.Context, *types.ListClosedWorkflowExecutionsRequest) (*types.ListClosedWorkflowExecutionsResponse, error)
		ListDomains(context.Context, *types.ListDomainsRequest) (*types.ListDomainsResponse, error)
		ListOpenWorkflowExecutions(context.Context, *types.ListOpenWorkflowExecutionsRequest) (*types.ListOpenWorkflowExecutionsResponse, error)
		ListTaskListPartitions(context.Context, *types.ListTaskListPartitionsRequest) (*types.ListTaskListPartitionsResponse, error)
		GetTaskListsByDomain(context.Context, *types.GetTaskListsByDomainRequest) (*types.GetTaskListsByDomainResponse, error)
		RefreshWorkflowTasks(context.Context, *types.RefreshWorkflowTasksRequest) error
		ListWorkflowExecutions(context.Context, *types.ListWorkflowExecutionsRequest) (*types.ListWorkflowExecutionsResponse, error)
		PollForActivityTask(context.Context, *types.PollForActivityTaskRequest) (*types.PollForActivityTaskResponse, error)
		PollForDecisionTask(context.Context, *types.PollForDecisionTaskRequest) (*types.PollForDecisionTaskResponse, error)
		QueryWorkflow(context.Context, *types.QueryWorkflowRequest) (*types.QueryWorkflowResponse, error)
		RecordActivityTaskHeartbeat(context.Context, *types.RecordActivityTaskHeartbeatRequest) (*types.RecordActivityTaskHeartbeatResponse, error)
		RecordActivityTaskHeartbeatByID(context.Context, *types.RecordActivityTaskHeartbeatByIDRequest) (*types.RecordActivityTaskHeartbeatResponse, error)
		RegisterDomain(context.Context, *types.RegisterDomainRequest) error
		RequestCancelWorkflowExecution(context.Context, *types.RequestCancelWorkflowExecutionRequest) error
		ResetStickyTaskList(context.Context, *types.ResetStickyTaskListRequest) (*types.ResetStickyTaskListResponse, error)
		ResetWorkflowExecution(context.Context, *types.ResetWorkflowExecutionRequest) (*types.ResetWorkflowExecutionResponse, error)
		RespondActivityTaskCanceled(context.Context, *types.RespondActivityTaskCanceledRequest) error
		RespondActivityTaskCanceledByID(context.Context, *types.RespondActivityTaskCanceledByIDRequest) error
		RespondActivityTaskCompleted(context.Context, *types.RespondActivityTaskCompletedRequest) error
		RespondActivityTaskCompletedByID(context.Context, *types.RespondActivityTaskCompletedByIDRequest) error
		RespondActivityTaskFailed(context.Context, *types.RespondActivityTaskFailedRequest) error
		RespondActivityTaskFailedByID(context.Context, *types.RespondActivityTaskFailedByIDRequest) error
		RespondDecisionTaskCompleted(context.Context, *types.RespondDecisionTaskCompletedRequest) (*types.RespondDecisionTaskCompletedResponse, error)
		RespondDecisionTaskFailed(context.Context, *types.RespondDecisionTaskFailedRequest) error
		RespondQueryTaskCompleted(context.Context, *types.RespondQueryTaskCompletedRequest) error
		RestartWorkflowExecution(context.Context, *types.RestartWorkflowExecutionRequest) (*types.RestartWorkflowExecutionResponse, error)
		ScanWorkflowExecutions(context.Context, *types.ListWorkflowExecutionsRequest) (*types.ListWorkflowExecutionsResponse, error)
		SignalWithStartWorkflowExecution(context.Context, *types.SignalWithStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error)
		SignalWithStartWorkflowExecutionAsync(context.Context, *types.SignalWithStartWorkflowExecutionAsyncRequest) (*types.SignalWithStartWorkflowExecutionAsyncResponse, error)
		SignalWorkflowExecution(context.Context, *types.SignalWorkflowExecutionRequest) error
		StartWorkflowExecution(context.Context, *types.StartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error)
		StartWorkflowExecutionAsync(context.Context, *types.StartWorkflowExecutionAsyncRequest) (*types.StartWorkflowExecutionAsyncResponse, error)
		TerminateWorkflowExecution(context.Context, *types.TerminateWorkflowExecutionRequest) error
		UpdateDomain(context.Context, *types.UpdateDomainRequest) (*types.UpdateDomainResponse, error)
		FailoverDomain(context.Context, *types.FailoverDomainRequest) (*types.FailoverDomainResponse, error)
		ListFailoverHistory(context.Context, *types.ListFailoverHistoryRequest) (*types.ListFailoverHistoryResponse, error)
	}
)
