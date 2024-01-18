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

package accesscontrolled

// Code generated by gowrap. DO NOT EDIT.
// template: ../../templates/accesscontrolled.tmpl
// gowrap: http://github.com/hexdigest/gowrap

import (
	"context"

	"github.com/uber/cadence/common/authorization"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/admin"
)

// adminHandler frontend handler wrapper for authentication and authorization
type adminHandler struct {
	handler    admin.Handler
	authorizer authorization.Authorizer
	resource.Resource
}

// NewAdminHandler creates frontend handler with authentication support
func NewAdminHandler(handler admin.Handler, resource resource.Resource, authorizer authorization.Authorizer, cfg config.Authorization) admin.Handler {
	if authorizer == nil {
		var err error
		authorizer, err = authorization.NewAuthorizer(cfg, resource.GetLogger(), resource.GetDomainCache())
		if err != nil {
			resource.GetLogger().Fatal("Error when initiating the Authorizer", tag.Error(err))
		}
	}
	return &adminHandler{
		handler:    handler,
		authorizer: authorizer,
		Resource:   resource,
	}
}

func (a *adminHandler) AddSearchAttribute(ctx context.Context, ap1 *types.AddSearchAttributeRequest) (err error) {
	attr := &authorization.Attributes{
		APIName:     "AddSearchAttribute",
		Permission:  authorization.PermissionAdmin,
		RequestBody: ap1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return err
	}
	if !isAuthorized {
		return errUnauthorized
	}
	return a.handler.AddSearchAttribute(ctx, ap1)
}

func (a *adminHandler) CloseShard(ctx context.Context, cp1 *types.CloseShardRequest) (err error) {
	attr := &authorization.Attributes{
		APIName:     "CloseShard",
		Permission:  authorization.PermissionAdmin,
		RequestBody: cp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return err
	}
	if !isAuthorized {
		return errUnauthorized
	}
	return a.handler.CloseShard(ctx, cp1)
}

func (a *adminHandler) CountDLQMessages(ctx context.Context, cp1 *types.CountDLQMessagesRequest) (cp2 *types.CountDLQMessagesResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "CountDLQMessages",
		Permission:  authorization.PermissionAdmin,
		RequestBody: cp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.CountDLQMessages(ctx, cp1)
}

func (a *adminHandler) DeleteWorkflow(ctx context.Context, ap1 *types.AdminDeleteWorkflowRequest) (ap2 *types.AdminDeleteWorkflowResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "DeleteWorkflow",
		Permission:  authorization.PermissionAdmin,
		RequestBody: ap1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.DeleteWorkflow(ctx, ap1)
}

func (a *adminHandler) DescribeCluster(ctx context.Context) (dp1 *types.DescribeClusterResponse, err error) {
	attr := &authorization.Attributes{
		APIName:    "DescribeCluster",
		Permission: authorization.PermissionAdmin,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.DescribeCluster(ctx)
}

func (a *adminHandler) DescribeHistoryHost(ctx context.Context, dp1 *types.DescribeHistoryHostRequest) (dp2 *types.DescribeHistoryHostResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "DescribeHistoryHost",
		Permission:  authorization.PermissionAdmin,
		RequestBody: dp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.DescribeHistoryHost(ctx, dp1)
}

func (a *adminHandler) DescribeQueue(ctx context.Context, dp1 *types.DescribeQueueRequest) (dp2 *types.DescribeQueueResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "DescribeQueue",
		Permission:  authorization.PermissionAdmin,
		RequestBody: dp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.DescribeQueue(ctx, dp1)
}

func (a *adminHandler) DescribeShardDistribution(ctx context.Context, dp1 *types.DescribeShardDistributionRequest) (dp2 *types.DescribeShardDistributionResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "DescribeShardDistribution",
		Permission:  authorization.PermissionAdmin,
		RequestBody: dp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.DescribeShardDistribution(ctx, dp1)
}

func (a *adminHandler) DescribeWorkflowExecution(ctx context.Context, ap1 *types.AdminDescribeWorkflowExecutionRequest) (ap2 *types.AdminDescribeWorkflowExecutionResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "DescribeWorkflowExecution",
		Permission:  authorization.PermissionAdmin,
		RequestBody: ap1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.DescribeWorkflowExecution(ctx, ap1)
}

func (a *adminHandler) GetCrossClusterTasks(ctx context.Context, gp1 *types.GetCrossClusterTasksRequest) (gp2 *types.GetCrossClusterTasksResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "GetCrossClusterTasks",
		Permission:  authorization.PermissionAdmin,
		RequestBody: gp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.GetCrossClusterTasks(ctx, gp1)
}

func (a *adminHandler) GetDLQReplicationMessages(ctx context.Context, gp1 *types.GetDLQReplicationMessagesRequest) (gp2 *types.GetDLQReplicationMessagesResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "GetDLQReplicationMessages",
		Permission:  authorization.PermissionAdmin,
		RequestBody: gp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.GetDLQReplicationMessages(ctx, gp1)
}

func (a *adminHandler) GetDomainIsolationGroups(ctx context.Context, request *types.GetDomainIsolationGroupsRequest) (gp1 *types.GetDomainIsolationGroupsResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "GetDomainIsolationGroups",
		Permission:  authorization.PermissionAdmin,
		RequestBody: request,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.GetDomainIsolationGroups(ctx, request)
}

func (a *adminHandler) GetDomainReplicationMessages(ctx context.Context, gp1 *types.GetDomainReplicationMessagesRequest) (gp2 *types.GetDomainReplicationMessagesResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "GetDomainReplicationMessages",
		Permission:  authorization.PermissionAdmin,
		RequestBody: gp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.GetDomainReplicationMessages(ctx, gp1)
}

func (a *adminHandler) GetDynamicConfig(ctx context.Context, gp1 *types.GetDynamicConfigRequest) (gp2 *types.GetDynamicConfigResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "GetDynamicConfig",
		Permission:  authorization.PermissionAdmin,
		RequestBody: gp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.GetDynamicConfig(ctx, gp1)
}

func (a *adminHandler) GetGlobalIsolationGroups(ctx context.Context, request *types.GetGlobalIsolationGroupsRequest) (gp1 *types.GetGlobalIsolationGroupsResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "GetGlobalIsolationGroups",
		Permission:  authorization.PermissionAdmin,
		RequestBody: request,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.GetGlobalIsolationGroups(ctx, request)
}

func (a *adminHandler) GetReplicationMessages(ctx context.Context, gp1 *types.GetReplicationMessagesRequest) (gp2 *types.GetReplicationMessagesResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "GetReplicationMessages",
		Permission:  authorization.PermissionAdmin,
		RequestBody: gp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.GetReplicationMessages(ctx, gp1)
}

func (a *adminHandler) GetWorkflowExecutionRawHistoryV2(ctx context.Context, gp1 *types.GetWorkflowExecutionRawHistoryV2Request) (gp2 *types.GetWorkflowExecutionRawHistoryV2Response, err error) {
	attr := &authorization.Attributes{
		APIName:     "GetWorkflowExecutionRawHistoryV2",
		Permission:  authorization.PermissionAdmin,
		RequestBody: gp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.GetWorkflowExecutionRawHistoryV2(ctx, gp1)
}

func (a *adminHandler) ListDynamicConfig(ctx context.Context, lp1 *types.ListDynamicConfigRequest) (lp2 *types.ListDynamicConfigResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "ListDynamicConfig",
		Permission:  authorization.PermissionAdmin,
		RequestBody: lp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.ListDynamicConfig(ctx, lp1)
}

func (a *adminHandler) MaintainCorruptWorkflow(ctx context.Context, ap1 *types.AdminMaintainWorkflowRequest) (ap2 *types.AdminMaintainWorkflowResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "MaintainCorruptWorkflow",
		Permission:  authorization.PermissionAdmin,
		RequestBody: ap1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.MaintainCorruptWorkflow(ctx, ap1)
}

func (a *adminHandler) MergeDLQMessages(ctx context.Context, mp1 *types.MergeDLQMessagesRequest) (mp2 *types.MergeDLQMessagesResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "MergeDLQMessages",
		Permission:  authorization.PermissionAdmin,
		RequestBody: mp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.MergeDLQMessages(ctx, mp1)
}

func (a *adminHandler) PurgeDLQMessages(ctx context.Context, pp1 *types.PurgeDLQMessagesRequest) (err error) {
	attr := &authorization.Attributes{
		APIName:     "PurgeDLQMessages",
		Permission:  authorization.PermissionAdmin,
		RequestBody: pp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return err
	}
	if !isAuthorized {
		return errUnauthorized
	}
	return a.handler.PurgeDLQMessages(ctx, pp1)
}

func (a *adminHandler) ReadDLQMessages(ctx context.Context, rp1 *types.ReadDLQMessagesRequest) (rp2 *types.ReadDLQMessagesResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "ReadDLQMessages",
		Permission:  authorization.PermissionAdmin,
		RequestBody: rp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.ReadDLQMessages(ctx, rp1)
}

func (a *adminHandler) ReapplyEvents(ctx context.Context, rp1 *types.ReapplyEventsRequest) (err error) {
	attr := &authorization.Attributes{
		APIName:     "ReapplyEvents",
		Permission:  authorization.PermissionAdmin,
		RequestBody: rp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return err
	}
	if !isAuthorized {
		return errUnauthorized
	}
	return a.handler.ReapplyEvents(ctx, rp1)
}

func (a *adminHandler) RefreshWorkflowTasks(ctx context.Context, rp1 *types.RefreshWorkflowTasksRequest) (err error) {
	attr := &authorization.Attributes{
		APIName:     "RefreshWorkflowTasks",
		Permission:  authorization.PermissionAdmin,
		RequestBody: rp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return err
	}
	if !isAuthorized {
		return errUnauthorized
	}
	return a.handler.RefreshWorkflowTasks(ctx, rp1)
}

func (a *adminHandler) RemoveTask(ctx context.Context, rp1 *types.RemoveTaskRequest) (err error) {
	attr := &authorization.Attributes{
		APIName:     "RemoveTask",
		Permission:  authorization.PermissionAdmin,
		RequestBody: rp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return err
	}
	if !isAuthorized {
		return errUnauthorized
	}
	return a.handler.RemoveTask(ctx, rp1)
}

func (a *adminHandler) ResendReplicationTasks(ctx context.Context, rp1 *types.ResendReplicationTasksRequest) (err error) {
	attr := &authorization.Attributes{
		APIName:     "ResendReplicationTasks",
		Permission:  authorization.PermissionAdmin,
		RequestBody: rp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return err
	}
	if !isAuthorized {
		return errUnauthorized
	}
	return a.handler.ResendReplicationTasks(ctx, rp1)
}

func (a *adminHandler) ResetQueue(ctx context.Context, rp1 *types.ResetQueueRequest) (err error) {
	attr := &authorization.Attributes{
		APIName:     "ResetQueue",
		Permission:  authorization.PermissionAdmin,
		RequestBody: rp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return err
	}
	if !isAuthorized {
		return errUnauthorized
	}
	return a.handler.ResetQueue(ctx, rp1)
}

func (a *adminHandler) RespondCrossClusterTasksCompleted(ctx context.Context, rp1 *types.RespondCrossClusterTasksCompletedRequest) (rp2 *types.RespondCrossClusterTasksCompletedResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "RespondCrossClusterTasksCompleted",
		Permission:  authorization.PermissionAdmin,
		RequestBody: rp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.RespondCrossClusterTasksCompleted(ctx, rp1)
}

func (a *adminHandler) RestoreDynamicConfig(ctx context.Context, rp1 *types.RestoreDynamicConfigRequest) (err error) {
	attr := &authorization.Attributes{
		APIName:     "RestoreDynamicConfig",
		Permission:  authorization.PermissionAdmin,
		RequestBody: rp1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return err
	}
	if !isAuthorized {
		return errUnauthorized
	}
	return a.handler.RestoreDynamicConfig(ctx, rp1)
}

func (a *adminHandler) Start() {
	a.handler.Start()
}

func (a *adminHandler) Stop() {
	a.handler.Stop()
}

func (a *adminHandler) UpdateDomainIsolationGroups(ctx context.Context, request *types.UpdateDomainIsolationGroupsRequest) (up1 *types.UpdateDomainIsolationGroupsResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "UpdateDomainIsolationGroups",
		Permission:  authorization.PermissionAdmin,
		RequestBody: request,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.UpdateDomainIsolationGroups(ctx, request)
}

func (a *adminHandler) UpdateDynamicConfig(ctx context.Context, up1 *types.UpdateDynamicConfigRequest) (err error) {
	attr := &authorization.Attributes{
		APIName:     "UpdateDynamicConfig",
		Permission:  authorization.PermissionAdmin,
		RequestBody: up1,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return err
	}
	if !isAuthorized {
		return errUnauthorized
	}
	return a.handler.UpdateDynamicConfig(ctx, up1)
}

func (a *adminHandler) UpdateGlobalIsolationGroups(ctx context.Context, request *types.UpdateGlobalIsolationGroupsRequest) (up1 *types.UpdateGlobalIsolationGroupsResponse, err error) {
	attr := &authorization.Attributes{
		APIName:     "UpdateGlobalIsolationGroups",
		Permission:  authorization.PermissionAdmin,
		RequestBody: request,
	}
	isAuthorized, err := a.isAuthorized(ctx, attr)
	if err != nil {
		return nil, err
	}
	if !isAuthorized {
		return nil, errUnauthorized
	}
	return a.handler.UpdateGlobalIsolationGroups(ctx, request)
}
