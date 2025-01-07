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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -self_package github.com/uber/cadence/service/frontend/admin
//go:generate gowrap gen -g -p . -i Handler -t ../templates/accesscontrolled.tmpl -o ../wrappers/accesscontrolled/admin_generated.go -v handler=Admin
//go:generate gowrap gen -g -p . -i Handler -t ../../templates/grpc.tmpl -o ../wrappers/grpc/admin_generated.go -v handler=Admin -v package=adminv1 -v path=github.com/uber/cadence-idl/go/proto/admin/v1 -v prefix=Admin
//go:generate gowrap gen -g -p ../../../.gen/go/admin/adminserviceserver -i Interface -t ../../templates/thrift.tmpl -o ../wrappers/thrift/admin_generated.go -v handler=Admin -v prefix=Admin

package admin

import (
	"context"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

// Handler interface for admin service
type Handler interface {
	common.Daemon

	AddSearchAttribute(context.Context, *types.AddSearchAttributeRequest) error
	CloseShard(context.Context, *types.CloseShardRequest) error
	DescribeCluster(context.Context) (*types.DescribeClusterResponse, error)
	DescribeShardDistribution(context.Context, *types.DescribeShardDistributionRequest) (*types.DescribeShardDistributionResponse, error)
	DescribeHistoryHost(context.Context, *types.DescribeHistoryHostRequest) (*types.DescribeHistoryHostResponse, error)
	DescribeQueue(context.Context, *types.DescribeQueueRequest) (*types.DescribeQueueResponse, error)
	DescribeWorkflowExecution(context.Context, *types.AdminDescribeWorkflowExecutionRequest) (*types.AdminDescribeWorkflowExecutionResponse, error)
	GetDLQReplicationMessages(context.Context, *types.GetDLQReplicationMessagesRequest) (*types.GetDLQReplicationMessagesResponse, error)
	GetDomainReplicationMessages(context.Context, *types.GetDomainReplicationMessagesRequest) (*types.GetDomainReplicationMessagesResponse, error)
	GetReplicationMessages(context.Context, *types.GetReplicationMessagesRequest) (*types.GetReplicationMessagesResponse, error)
	GetWorkflowExecutionRawHistoryV2(context.Context, *types.GetWorkflowExecutionRawHistoryV2Request) (*types.GetWorkflowExecutionRawHistoryV2Response, error)
	CountDLQMessages(context.Context, *types.CountDLQMessagesRequest) (*types.CountDLQMessagesResponse, error)
	MergeDLQMessages(context.Context, *types.MergeDLQMessagesRequest) (*types.MergeDLQMessagesResponse, error)
	PurgeDLQMessages(context.Context, *types.PurgeDLQMessagesRequest) error
	ReadDLQMessages(context.Context, *types.ReadDLQMessagesRequest) (*types.ReadDLQMessagesResponse, error)
	ReapplyEvents(context.Context, *types.ReapplyEventsRequest) error
	RefreshWorkflowTasks(context.Context, *types.RefreshWorkflowTasksRequest) error
	RemoveTask(context.Context, *types.RemoveTaskRequest) error
	ResendReplicationTasks(context.Context, *types.ResendReplicationTasksRequest) error
	ResetQueue(context.Context, *types.ResetQueueRequest) error
	GetCrossClusterTasks(context.Context, *types.GetCrossClusterTasksRequest) (*types.GetCrossClusterTasksResponse, error)
	RespondCrossClusterTasksCompleted(context.Context, *types.RespondCrossClusterTasksCompletedRequest) (*types.RespondCrossClusterTasksCompletedResponse, error)
	GetDynamicConfig(context.Context, *types.GetDynamicConfigRequest) (*types.GetDynamicConfigResponse, error)
	UpdateDynamicConfig(context.Context, *types.UpdateDynamicConfigRequest) error
	RestoreDynamicConfig(context.Context, *types.RestoreDynamicConfigRequest) error
	ListDynamicConfig(context.Context, *types.ListDynamicConfigRequest) (*types.ListDynamicConfigResponse, error)
	DeleteWorkflow(context.Context, *types.AdminDeleteWorkflowRequest) (*types.AdminDeleteWorkflowResponse, error)
	MaintainCorruptWorkflow(context.Context, *types.AdminMaintainWorkflowRequest) (*types.AdminMaintainWorkflowResponse, error)
	GetGlobalIsolationGroups(ctx context.Context, request *types.GetGlobalIsolationGroupsRequest) (*types.GetGlobalIsolationGroupsResponse, error)
	UpdateGlobalIsolationGroups(ctx context.Context, request *types.UpdateGlobalIsolationGroupsRequest) (*types.UpdateGlobalIsolationGroupsResponse, error)
	GetDomainIsolationGroups(ctx context.Context, request *types.GetDomainIsolationGroupsRequest) (*types.GetDomainIsolationGroupsResponse, error)
	UpdateDomainIsolationGroups(ctx context.Context, request *types.UpdateDomainIsolationGroupsRequest) (*types.UpdateDomainIsolationGroupsResponse, error)
	GetDomainAsyncWorkflowConfiguraton(context.Context, *types.GetDomainAsyncWorkflowConfiguratonRequest) (*types.GetDomainAsyncWorkflowConfiguratonResponse, error)
	UpdateDomainAsyncWorkflowConfiguraton(context.Context, *types.UpdateDomainAsyncWorkflowConfiguratonRequest) (*types.UpdateDomainAsyncWorkflowConfiguratonResponse, error)
	UpdateTaskListPartitionConfig(context.Context, *types.UpdateTaskListPartitionConfigRequest) (*types.UpdateTaskListPartitionConfigResponse, error)
}
