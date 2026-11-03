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

package matching

import (
	"context"

	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -package matching github.com/uber/cadence/client/matching Client,Gateway
//go:generate gowrap gen -g -p . -i Gateway -t ../templates/retry.tmpl -o ../wrappers/retryable/matching_generated.go -v client=Matching
//go:generate gowrap gen -g -p . -i Gateway -t ../templates/metered.tmpl -o ../wrappers/metered/matching_generated.go -v client=Matching
//go:generate gowrap gen -g -p . -i Gateway -t ../templates/errorinjectors.tmpl -o ../wrappers/errorinjectors/matching_generated.go -v client=Matching
//go:generate gowrap gen -g -p . -i Client -t ../templates/grpc.tmpl -o ../wrappers/grpc/matching_generated.go -v client=Matching -v package=matchingv1 -v path=github.com/uber/cadence/.gen/proto/matching/v1 -v prefix=Matching
//go:generate gowrap gen -g -p . -i Client -t ../templates/thrift.tmpl -o ../wrappers/thrift/matching_generated.go -v client=Matching -v prefix=Matching
//go:generate gowrap gen -g -p . -i Gateway -t ../templates/timeout.tmpl -o ../wrappers/timeout/matching_generated.go -v client=Matching

// Gateway is the interface services use to talk to matching.
// It hides task list partitioning and peer resolution, so it does not have to follow the RPC API one to one.
type Gateway interface {
	AddActivityTask(context.Context, *types.AddActivityTaskRequest) (*types.AddActivityTaskResponse, error)
	AddDecisionTask(context.Context, *types.AddDecisionTaskRequest) (*types.AddDecisionTaskResponse, error)
	CancelOutstandingPoll(context.Context, *types.CancelOutstandingPollRequest) error
	DescribeTaskList(context.Context, *types.MatchingDescribeTaskListRequest) (*types.DescribeTaskListResponse, error)
	ListTaskListPartitions(context.Context, *types.MatchingListTaskListPartitionsRequest) (*types.ListTaskListPartitionsResponse, error)
	GetTaskListsByDomain(context.Context, *types.GetTaskListsByDomainRequest) (*types.GetTaskListsByDomainResponse, error)
	PollForActivityTask(context.Context, *types.MatchingPollForActivityTaskRequest) (*types.MatchingPollForActivityTaskResponse, error)
	PollForDecisionTask(context.Context, *types.MatchingPollForDecisionTaskRequest) (*types.MatchingPollForDecisionTaskResponse, error)
	QueryWorkflow(context.Context, *types.MatchingQueryWorkflowRequest) (*types.MatchingQueryWorkflowResponse, error)
	RespondQueryTaskCompleted(context.Context, *types.MatchingRespondQueryTaskCompletedRequest) error
	UpdateTaskListPartitionConfig(context.Context, *types.MatchingUpdateTaskListPartitionConfigRequest) (*types.MatchingUpdateTaskListPartitionConfigResponse, error)
	RefreshTaskListPartitionConfig(context.Context, *types.MatchingRefreshTaskListPartitionConfigRequest) (*types.MatchingRefreshTaskListPartitionConfigResponse, error)
	AcquireSemaphore(context.Context, *types.AcquireSemaphoreRequest) (*types.AcquireSemaphoreResponse, error)
	ReleaseSemaphore(context.Context, *types.ReleaseSemaphoreRequest) (*types.ReleaseSemaphoreResponse, error)
}

// Client is the RPC client of the matching service API, implemented by the gRPC and Thrift transports.
// Requests are sent as is to the peer given in the call options.
type Client interface {
	AddActivityTask(context.Context, *types.AddActivityTaskRequest, ...yarpc.CallOption) (*types.AddActivityTaskResponse, error)
	AddDecisionTask(context.Context, *types.AddDecisionTaskRequest, ...yarpc.CallOption) (*types.AddDecisionTaskResponse, error)
	CancelOutstandingPoll(context.Context, *types.CancelOutstandingPollRequest, ...yarpc.CallOption) error
	DescribeTaskList(context.Context, *types.MatchingDescribeTaskListRequest, ...yarpc.CallOption) (*types.DescribeTaskListResponse, error)
	ListTaskListPartitions(context.Context, *types.MatchingListTaskListPartitionsRequest, ...yarpc.CallOption) (*types.ListTaskListPartitionsResponse, error)
	GetTaskListsByDomain(context.Context, *types.GetTaskListsByDomainRequest, ...yarpc.CallOption) (*types.GetTaskListsByDomainResponse, error)
	PollForActivityTask(context.Context, *types.MatchingPollForActivityTaskRequest, ...yarpc.CallOption) (*types.MatchingPollForActivityTaskResponse, error)
	PollForDecisionTask(context.Context, *types.MatchingPollForDecisionTaskRequest, ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error)
	QueryWorkflow(context.Context, *types.MatchingQueryWorkflowRequest, ...yarpc.CallOption) (*types.MatchingQueryWorkflowResponse, error)
	RespondQueryTaskCompleted(context.Context, *types.MatchingRespondQueryTaskCompletedRequest, ...yarpc.CallOption) error
	UpdateTaskListPartitionConfig(context.Context, *types.MatchingUpdateTaskListPartitionConfigRequest, ...yarpc.CallOption) (*types.MatchingUpdateTaskListPartitionConfigResponse, error)
	RefreshTaskListPartitionConfig(context.Context, *types.MatchingRefreshTaskListPartitionConfigRequest, ...yarpc.CallOption) (*types.MatchingRefreshTaskListPartitionConfigResponse, error)
	AcquireSemaphore(context.Context, *types.AcquireSemaphoreRequest, ...yarpc.CallOption) (*types.AcquireSemaphoreResponse, error)
	ReleaseSemaphore(context.Context, *types.ReleaseSemaphoreRequest, ...yarpc.CallOption) (*types.ReleaseSemaphoreResponse, error)
}
