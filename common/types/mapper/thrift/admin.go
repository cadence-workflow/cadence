// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

package thrift

import (
	"sort"

	"github.com/uber/cadence/.gen/go/admin"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/types"
)

var (
	FromAdminRefreshWorkflowTasksRequest = FromRefreshWorkflowTasksRequest
	ToAdminRefreshWorkflowTasksRequest   = ToRefreshWorkflowTasksRequest
)

// FromAdminAddSearchAttributeRequest converts internal AddSearchAttributeRequest type to thrift
func FromAdminAddSearchAttributeRequest(t *types.AddSearchAttributeRequest) *admin.AddSearchAttributeRequest {
	if t == nil {
		return nil
	}
	return &admin.AddSearchAttributeRequest{
		SearchAttribute: FromIndexedValueTypeMap(t.SearchAttribute),
		SecurityToken:   &t.SecurityToken,
	}
}

// ToAdminAddSearchAttributeRequest converts thrift AddSearchAttributeRequest type to internal
func ToAdminAddSearchAttributeRequest(t *admin.AddSearchAttributeRequest) *types.AddSearchAttributeRequest {
	if t == nil {
		return nil
	}
	return &types.AddSearchAttributeRequest{
		SearchAttribute: ToIndexedValueTypeMap(t.SearchAttribute),
		SecurityToken:   t.GetSecurityToken(),
	}
}

// FromAdminDescribeClusterResponse converts internal DescribeClusterResponse type to thrift
func FromAdminDescribeClusterResponse(t *types.DescribeClusterResponse) *admin.DescribeClusterResponse {
	if t == nil {
		return nil
	}
	return &admin.DescribeClusterResponse{
		SupportedClientVersions: FromSupportedClientVersions(t.SupportedClientVersions),
		MembershipInfo:          FromMembershipInfo(t.MembershipInfo),
		PersistenceInfo:         FromPersistenceInfoMap(t.PersistenceInfo),
	}
}

// ToAdminDescribeClusterResponse converts thrift DescribeClusterResponse type to internal
func ToAdminDescribeClusterResponse(t *admin.DescribeClusterResponse) *types.DescribeClusterResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeClusterResponse{
		SupportedClientVersions: ToSupportedClientVersions(t.SupportedClientVersions),
		MembershipInfo:          ToMembershipInfo(t.MembershipInfo),
		PersistenceInfo:         ToPersistenceInfoMap(t.PersistenceInfo),
	}
}

// FromAdminDescribeWorkflowExecutionRequest converts internal DescribeWorkflowExecutionRequest type to thrift
func FromAdminDescribeWorkflowExecutionRequest(t *types.AdminDescribeWorkflowExecutionRequest) *admin.DescribeWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &admin.DescribeWorkflowExecutionRequest{
		Domain:    &t.Domain,
		Execution: FromWorkflowExecution(t.Execution),
	}
}

// ToAdminDescribeWorkflowExecutionRequest converts thrift DescribeWorkflowExecutionRequest type to internal
func ToAdminDescribeWorkflowExecutionRequest(t *admin.DescribeWorkflowExecutionRequest) *types.AdminDescribeWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.AdminDescribeWorkflowExecutionRequest{
		Domain:    t.GetDomain(),
		Execution: ToWorkflowExecution(t.Execution),
	}
}

// FromAdminDescribeWorkflowExecutionResponse converts internal DescribeWorkflowExecutionResponse type to thrift
func FromAdminDescribeWorkflowExecutionResponse(t *types.AdminDescribeWorkflowExecutionResponse) *admin.DescribeWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &admin.DescribeWorkflowExecutionResponse{
		ShardId:                &t.ShardID,
		HistoryAddr:            &t.HistoryAddr,
		MutableStateInCache:    &t.MutableStateInCache,
		MutableStateInDatabase: &t.MutableStateInDatabase,
	}
}

// ToAdminDescribeWorkflowExecutionResponse converts thrift DescribeWorkflowExecutionResponse type to internal
func ToAdminDescribeWorkflowExecutionResponse(t *admin.DescribeWorkflowExecutionResponse) *types.AdminDescribeWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.AdminDescribeWorkflowExecutionResponse{
		ShardID:                t.GetShardId(),
		HistoryAddr:            t.GetHistoryAddr(),
		MutableStateInCache:    t.GetMutableStateInCache(),
		MutableStateInDatabase: t.GetMutableStateInDatabase(),
	}
}

// FromAdminGetWorkflowExecutionRawHistoryV2Request converts internal GetWorkflowExecutionRawHistoryV2Request type to thrift
func FromAdminGetWorkflowExecutionRawHistoryV2Request(t *types.GetWorkflowExecutionRawHistoryV2Request) *admin.GetWorkflowExecutionRawHistoryV2Request {
	if t == nil {
		return nil
	}
	return &admin.GetWorkflowExecutionRawHistoryV2Request{
		Domain:            &t.Domain,
		Execution:         FromWorkflowExecution(t.Execution),
		StartEventId:      t.StartEventID,
		StartEventVersion: t.StartEventVersion,
		EndEventId:        t.EndEventID,
		EndEventVersion:   t.EndEventVersion,
		MaximumPageSize:   &t.MaximumPageSize,
		NextPageToken:     t.NextPageToken,
	}
}

// ToAdminGetWorkflowExecutionRawHistoryV2Request converts thrift GetWorkflowExecutionRawHistoryV2Request type to internal
func ToAdminGetWorkflowExecutionRawHistoryV2Request(t *admin.GetWorkflowExecutionRawHistoryV2Request) *types.GetWorkflowExecutionRawHistoryV2Request {
	if t == nil {
		return nil
	}
	return &types.GetWorkflowExecutionRawHistoryV2Request{
		Domain:            t.GetDomain(),
		Execution:         ToWorkflowExecution(t.Execution),
		StartEventID:      t.StartEventId,
		StartEventVersion: t.StartEventVersion,
		EndEventID:        t.EndEventId,
		EndEventVersion:   t.EndEventVersion,
		MaximumPageSize:   t.GetMaximumPageSize(),
		NextPageToken:     t.NextPageToken,
	}
}

// FromAdminGetWorkflowExecutionRawHistoryV2Response converts internal GetWorkflowExecutionRawHistoryV2Response type to thrift
func FromAdminGetWorkflowExecutionRawHistoryV2Response(t *types.GetWorkflowExecutionRawHistoryV2Response) *admin.GetWorkflowExecutionRawHistoryV2Response {
	if t == nil {
		return nil
	}
	return &admin.GetWorkflowExecutionRawHistoryV2Response{
		NextPageToken:  t.NextPageToken,
		HistoryBatches: FromDataBlobArray(t.HistoryBatches),
		VersionHistory: FromVersionHistory(t.VersionHistory),
	}
}

// ToAdminGetWorkflowExecutionRawHistoryV2Response converts thrift GetWorkflowExecutionRawHistoryV2Response type to internal
func ToAdminGetWorkflowExecutionRawHistoryV2Response(t *admin.GetWorkflowExecutionRawHistoryV2Response) *types.GetWorkflowExecutionRawHistoryV2Response {
	if t == nil {
		return nil
	}
	return &types.GetWorkflowExecutionRawHistoryV2Response{
		NextPageToken:  t.NextPageToken,
		HistoryBatches: ToDataBlobArray(t.HistoryBatches),
		VersionHistory: ToVersionHistory(t.VersionHistory),
	}
}

// FromHostInfo converts internal HostInfo type to thrift
func FromHostInfo(t *types.HostInfo) *admin.HostInfo {
	if t == nil {
		return nil
	}
	return &admin.HostInfo{
		Identity: &t.Identity,
	}
}

// ToHostInfo converts thrift HostInfo type to internal
func ToHostInfo(t *admin.HostInfo) *types.HostInfo {
	if t == nil {
		return nil
	}
	return &types.HostInfo{
		Identity: t.GetIdentity(),
	}
}

// FromMembershipInfo converts internal MembershipInfo type to thrift
func FromMembershipInfo(t *types.MembershipInfo) *admin.MembershipInfo {
	if t == nil {
		return nil
	}
	return &admin.MembershipInfo{
		CurrentHost:      FromHostInfo(t.CurrentHost),
		ReachableMembers: t.ReachableMembers,
		Rings:            FromRingInfoArray(t.Rings),
	}
}

// ToMembershipInfo converts thrift MembershipInfo type to internal
func ToMembershipInfo(t *admin.MembershipInfo) *types.MembershipInfo {
	if t == nil {
		return nil
	}
	return &types.MembershipInfo{
		CurrentHost:      ToHostInfo(t.CurrentHost),
		ReachableMembers: t.ReachableMembers,
		Rings:            ToRingInfoArray(t.Rings),
	}
}

// FromPersistenceInfoMap converts internal map[string]*types.PersistenceInfo type to thrift
func FromPersistenceInfoMap(t map[string]*types.PersistenceInfo) map[string]*admin.PersistenceInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*admin.PersistenceInfo, len(t))
	for key := range t {
		v[key] = FromPersistenceInfo(t[key])
	}
	return v
}

// FromPersistenceInfo converts internal PersistenceInfo type to thrift
func FromPersistenceInfo(t *types.PersistenceInfo) *admin.PersistenceInfo {
	if t == nil {
		return nil
	}
	return &admin.PersistenceInfo{
		Backend:  &t.Backend,
		Settings: FromPersistenceSettings(t.Settings),
		Features: FromPersistenceFeatures(t.Features),
	}
}

// FromPersistenceSettings converts internal []*types.PersistenceSetting type to thrift
func FromPersistenceSettings(t []*types.PersistenceSetting) []*admin.PersistenceSetting {
	if t == nil {
		return nil
	}
	v := make([]*admin.PersistenceSetting, len(t))
	for i := range t {
		v[i] = FromPersistenceSetting(t[i])
	}
	return v
}

// FromPersistenceSetting converts internal PersistenceSetting type to thrift
func FromPersistenceSetting(t *types.PersistenceSetting) *admin.PersistenceSetting {
	if t == nil {
		return nil
	}
	return &admin.PersistenceSetting{
		Key:   &t.Key,
		Value: &t.Value,
	}
}

// FromPersistenceFeatures converts internal []*types.PersistenceFeature type to thrift
func FromPersistenceFeatures(t []*types.PersistenceFeature) []*admin.PersistenceFeature {
	if t == nil {
		return nil
	}
	v := make([]*admin.PersistenceFeature, len(t))
	for i := range t {
		v[i] = FromPersistenceFeature(t[i])
	}
	return v
}

// FromPersistenceFeature converts internal PersistenceFeature type to thrift
func FromPersistenceFeature(t *types.PersistenceFeature) *admin.PersistenceFeature {
	if t == nil {
		return nil
	}
	return &admin.PersistenceFeature{
		Key:     &t.Key,
		Enabled: &t.Enabled,
	}
}

// ToPersistenceInfoMap converts thrift to internal map[string]*types.PersistenceInfo type
func ToPersistenceInfoMap(t map[string]*admin.PersistenceInfo) map[string]*types.PersistenceInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.PersistenceInfo, len(t))
	for key := range t {
		v[key] = ToPersistenceInfo(t[key])
	}
	return v
}

// ToPersistenceInfo converts thrift to internal PersistenceInfo type
func ToPersistenceInfo(t *admin.PersistenceInfo) *types.PersistenceInfo {
	if t == nil {
		return nil
	}
	return &types.PersistenceInfo{
		Backend:  t.GetBackend(),
		Settings: ToPersistenceSettings(t.Settings),
		Features: ToPersistenceFeatures(t.Features),
	}
}

// ToPersistenceSettings converts thrift to internal []*types.PersistenceSetting type
func ToPersistenceSettings(t []*admin.PersistenceSetting) []*types.PersistenceSetting {
	if t == nil {
		return nil
	}
	v := make([]*types.PersistenceSetting, len(t))
	for i := range t {
		v[i] = ToPersistenceSetting(t[i])
	}
	return v
}

// ToPersistenceSetting converts from thrift to internal PersistenceSetting type
func ToPersistenceSetting(t *admin.PersistenceSetting) *types.PersistenceSetting {
	if t == nil {
		return nil
	}
	return &types.PersistenceSetting{
		Key:   t.GetKey(),
		Value: t.GetValue(),
	}
}

// ToPersistenceFeatures converts from thrift to internal []*types.PersistenceFeature type
func ToPersistenceFeatures(t []*admin.PersistenceFeature) []*types.PersistenceFeature {
	if t == nil {
		return nil
	}
	v := make([]*types.PersistenceFeature, len(t))
	for i := range t {
		v[i] = ToPersistenceFeature(t[i])
	}
	return v
}

// ToPersistenceFeature converts from thrift to internal PersistenceFeature type
func ToPersistenceFeature(t *admin.PersistenceFeature) *types.PersistenceFeature {
	if t == nil {
		return nil
	}
	return &types.PersistenceFeature{
		Key:     t.GetKey(),
		Enabled: t.GetEnabled(),
	}
}

// FromAdminResendReplicationTasksRequest converts internal ResendReplicationTasksRequest type to thrift
func FromAdminResendReplicationTasksRequest(t *types.ResendReplicationTasksRequest) *admin.ResendReplicationTasksRequest {
	if t == nil {
		return nil
	}
	return &admin.ResendReplicationTasksRequest{
		DomainID:      &t.DomainID,
		WorkflowID:    &t.WorkflowID,
		RunID:         &t.RunID,
		RemoteCluster: &t.RemoteCluster,
		StartEventID:  t.StartEventID,
		StartVersion:  t.StartVersion,
		EndEventID:    t.EndEventID,
		EndVersion:    t.EndVersion,
	}
}

// ToAdminResendReplicationTasksRequest converts thrift ResendReplicationTasksRequest type to internal
func ToAdminResendReplicationTasksRequest(t *admin.ResendReplicationTasksRequest) *types.ResendReplicationTasksRequest {
	if t == nil {
		return nil
	}
	return &types.ResendReplicationTasksRequest{
		DomainID:      t.GetDomainID(),
		WorkflowID:    t.GetWorkflowID(),
		RunID:         t.GetRunID(),
		RemoteCluster: t.GetRemoteCluster(),
		StartEventID:  t.StartEventID,
		StartVersion:  t.StartVersion,
		EndEventID:    t.EndEventID,
		EndVersion:    t.EndVersion,
	}
}

// FromRingInfo converts internal RingInfo type to thrift
func FromRingInfo(t *types.RingInfo) *admin.RingInfo {
	if t == nil {
		return nil
	}
	return &admin.RingInfo{
		Role:        &t.Role,
		MemberCount: &t.MemberCount,
		Members:     FromHostInfoArray(t.Members),
	}
}

// ToRingInfo converts thrift RingInfo type to internal
func ToRingInfo(t *admin.RingInfo) *types.RingInfo {
	if t == nil {
		return nil
	}
	return &types.RingInfo{
		Role:        t.GetRole(),
		MemberCount: t.GetMemberCount(),
		Members:     ToHostInfoArray(t.Members),
	}
}

// FromRingInfoArray converts internal RingInfo type array to thrift
func FromRingInfoArray(t []*types.RingInfo) []*admin.RingInfo {
	if t == nil {
		return nil
	}
	v := make([]*admin.RingInfo, len(t))
	for i := range t {
		v[i] = FromRingInfo(t[i])
	}
	return v
}

// ToRingInfoArray converts thrift RingInfo type array to internal
func ToRingInfoArray(t []*admin.RingInfo) []*types.RingInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.RingInfo, len(t))
	for i := range t {
		v[i] = ToRingInfo(t[i])
	}
	return v
}

// FromHostInfoArray converts internal HostInfo type array to thrift
func FromHostInfoArray(t []*types.HostInfo) []*admin.HostInfo {
	if t == nil {
		return nil
	}
	v := make([]*admin.HostInfo, len(t))
	for i := range t {
		v[i] = FromHostInfo(t[i])
	}
	return v
}

// ToHostInfoArray converts thrift HostInfo type array to internal
func ToHostInfoArray(t []*admin.HostInfo) []*types.HostInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.HostInfo, len(t))
	for i := range t {
		v[i] = ToHostInfo(t[i])
	}
	return v
}

// FromAdminGetDynamicConfigRequest converts internal GetDynamicConfigRequest type to thrift
func FromAdminGetDynamicConfigRequest(t *types.GetDynamicConfigRequest) *admin.GetDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &admin.GetDynamicConfigRequest{
		ConfigName: &t.ConfigName,
		Filters:    FromDynamicConfigFilterArray(t.Filters),
	}
}

// ToAdminGetDynamicConfigRequest converts thrift GetDynamicConfigRequest type to internal
func ToAdminGetDynamicConfigRequest(t *admin.GetDynamicConfigRequest) *types.GetDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.GetDynamicConfigRequest{
		ConfigName: t.GetConfigName(),
		Filters:    ToDynamicConfigFilterArray(t.Filters),
	}
}

// FromAdminGetDynamicConfigResponse converts internal GetDynamicConfigResponse type to thrift
func FromAdminGetDynamicConfigResponse(t *types.GetDynamicConfigResponse) *admin.GetDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &admin.GetDynamicConfigResponse{
		Value: FromDataBlob(t.Value),
	}
}

// ToAdminGetDynamicConfigResponse converts thrift GetDynamicConfigResponse type to internal
func ToAdminGetDynamicConfigResponse(t *admin.GetDynamicConfigResponse) *types.GetDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &types.GetDynamicConfigResponse{
		Value: ToDataBlob(t.Value),
	}
}

// FromAdminUpdateDynamicConfigRequest converts internal UpdateDynamicConfigRequest type to thrift
func FromAdminUpdateDynamicConfigRequest(t *types.UpdateDynamicConfigRequest) *admin.UpdateDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &admin.UpdateDynamicConfigRequest{
		ConfigName:   &t.ConfigName,
		ConfigValues: FromDynamicConfigValueArray(t.ConfigValues),
	}
}

// ToAdminUpdateDynamicConfigRequest converts thrift UpdateDynamicConfigRequest type to internal
func ToAdminUpdateDynamicConfigRequest(t *admin.UpdateDynamicConfigRequest) *types.UpdateDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.UpdateDynamicConfigRequest{
		ConfigName:   t.GetConfigName(),
		ConfigValues: ToDynamicConfigValueArray(t.ConfigValues),
	}
}

// FromAdminRestoreDynamicConfigRequest converts internal RestoreDynamicConfigRequest type to thrift
func FromAdminRestoreDynamicConfigRequest(t *types.RestoreDynamicConfigRequest) *admin.RestoreDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &admin.RestoreDynamicConfigRequest{
		ConfigName: &t.ConfigName,
		Filters:    FromDynamicConfigFilterArray(t.Filters),
	}
}

// ToAdminRestoreDynamicConfigRequest converts thrift RestoreDynamicConfigRequest type to internal
func ToAdminRestoreDynamicConfigRequest(t *admin.RestoreDynamicConfigRequest) *types.RestoreDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.RestoreDynamicConfigRequest{
		ConfigName: t.GetConfigName(),
		Filters:    ToDynamicConfigFilterArray(t.Filters),
	}
}

// FromAdminDeleteWorkflowRequest converts internal AdminDeleteWorkflowRequest type to thrift
func FromAdminDeleteWorkflowRequest(t *types.AdminDeleteWorkflowRequest) *admin.AdminDeleteWorkflowRequest {
	if t == nil {
		return nil
	}
	return &admin.AdminDeleteWorkflowRequest{
		Domain:    &t.Domain,
		Execution: FromWorkflowExecution(t.Execution),
	}
}

// ToAdminDeleteWorkflowRequest converts thrift AdminDeleteWorkflowRequest type to internal
func ToAdminDeleteWorkflowRequest(t *admin.AdminDeleteWorkflowRequest) *types.AdminDeleteWorkflowRequest {
	if t == nil {
		return nil
	}
	return &types.AdminDeleteWorkflowRequest{
		Domain:    t.GetDomain(),
		Execution: ToWorkflowExecution(t.Execution),
	}
}

// FromAdminDeleteWorkflowResponse converts internal AdminDeleteWorkflowResponse type to thrift
func FromAdminDeleteWorkflowResponse(t *types.AdminDeleteWorkflowResponse) *admin.AdminDeleteWorkflowResponse {
	if t == nil {
		return nil
	}
	return &admin.AdminDeleteWorkflowResponse{
		HistoryDeleted:    &t.HistoryDeleted,
		ExecutionsDeleted: &t.ExecutionsDeleted,
		VisibilityDeleted: &t.VisibilityDeleted,
	}
}

// ToAdminDeleteWorkflowResponse converts thrift AdminDeleteWorkflowResponse type to internal
func ToAdminDeleteWorkflowResponse(t *admin.AdminDeleteWorkflowResponse) *types.AdminDeleteWorkflowResponse {
	if t == nil {
		return nil
	}
	return &types.AdminDeleteWorkflowResponse{
		HistoryDeleted:    *t.HistoryDeleted,
		ExecutionsDeleted: *t.ExecutionsDeleted,
		VisibilityDeleted: *t.VisibilityDeleted,
	}
}

// FromAdminMaintainCorruptWorkflowRequest converts internal AdminMaintainWorkflowRequest type to thrift
func FromAdminMaintainCorruptWorkflowRequest(t *types.AdminMaintainWorkflowRequest) *admin.AdminMaintainWorkflowRequest {
	if t == nil {
		return nil
	}
	return &admin.AdminMaintainWorkflowRequest{
		Domain:    &t.Domain,
		Execution: FromWorkflowExecution(t.Execution),
	}
}

// ToAdminMaintainCorruptWorkflowRequest converts thrift AdminMaintainWorkflowRequest type to internal
func ToAdminMaintainCorruptWorkflowRequest(t *admin.AdminMaintainWorkflowRequest) *types.AdminMaintainWorkflowRequest {
	if t == nil {
		return nil
	}
	return &types.AdminMaintainWorkflowRequest{
		Domain:    t.GetDomain(),
		Execution: ToWorkflowExecution(t.Execution),
	}
}

// FromAdminMaintainCorruptWorkflowResponse converts internal AdminMaintainWorkflowResponse type to thrift
func FromAdminMaintainCorruptWorkflowResponse(t *types.AdminMaintainWorkflowResponse) *admin.AdminMaintainWorkflowResponse {
	if t == nil {
		return nil
	}
	return &admin.AdminMaintainWorkflowResponse{
		HistoryDeleted:    &t.HistoryDeleted,
		ExecutionsDeleted: &t.ExecutionsDeleted,
		VisibilityDeleted: &t.VisibilityDeleted,
	}
}

// ToAdminMaintainCorruptWorkflowResponse converts thrift AdminMaintainWorkflowResponse type to internal
func ToAdminMaintainCorruptWorkflowResponse(t *admin.AdminMaintainWorkflowResponse) *types.AdminMaintainWorkflowResponse {
	if t == nil {
		return nil
	}
	return &types.AdminMaintainWorkflowResponse{
		HistoryDeleted:    *t.HistoryDeleted,
		ExecutionsDeleted: *t.ExecutionsDeleted,
		VisibilityDeleted: *t.VisibilityDeleted,
	}
}

// FromAdminListDynamicConfigResponse converts internal ListDynamicConfigResponse type to thrift
func FromAdminListDynamicConfigResponse(t *types.ListDynamicConfigResponse) *admin.ListDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &admin.ListDynamicConfigResponse{
		Entries: FromDynamicConfigEntryArray(t.Entries),
	}
}

// FromAdminListDynamicConfigRequest converts internal ListDynamicConfigRequest type to thrift
func FromAdminListDynamicConfigRequest(t *types.ListDynamicConfigRequest) *admin.ListDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &admin.ListDynamicConfigRequest{
		ConfigName: &t.ConfigName,
	}
}

// ToAdminListDynamicConfigRequest converts thrift ListDynamicConfigRequest type to internal
func ToAdminListDynamicConfigRequest(t *admin.ListDynamicConfigRequest) *types.ListDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.ListDynamicConfigRequest{
		ConfigName: t.GetConfigName(),
	}
}

// ToAdminListDynamicConfigResponse converts thrift ListDynamicConfigResponse type to internal
func ToAdminListDynamicConfigResponse(t *admin.ListDynamicConfigResponse) *types.ListDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &types.ListDynamicConfigResponse{
		Entries: ToDynamicConfigEntryArray(t.Entries),
	}
}

// FromAdminGetOperationalDynamicConfigRequest converts internal GetOperationalDynamicConfigRequest type to thrift
func FromAdminGetOperationalDynamicConfigRequest(t *types.GetOperationalDynamicConfigRequest) *admin.GetOperationalDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &admin.GetOperationalDynamicConfigRequest{
		ConfigName: &t.ConfigName,
		Filters:    FromDynamicConfigFilterArray(t.Filters),
	}
}

// ToAdminGetOperationalDynamicConfigRequest converts thrift GetOperationalDynamicConfigRequest type to internal
func ToAdminGetOperationalDynamicConfigRequest(t *admin.GetOperationalDynamicConfigRequest) *types.GetOperationalDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.GetOperationalDynamicConfigRequest{
		ConfigName: t.GetConfigName(),
		Filters:    ToDynamicConfigFilterArray(t.Filters),
	}
}

// FromAdminGetOperationalDynamicConfigResponse converts internal GetOperationalDynamicConfigResponse type to thrift
func FromAdminGetOperationalDynamicConfigResponse(t *types.GetOperationalDynamicConfigResponse) *admin.GetOperationalDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &admin.GetOperationalDynamicConfigResponse{
		Value: FromDataBlob(t.Value),
	}
}

// ToAdminGetOperationalDynamicConfigResponse converts thrift GetOperationalDynamicConfigResponse type to internal
func ToAdminGetOperationalDynamicConfigResponse(t *admin.GetOperationalDynamicConfigResponse) *types.GetOperationalDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &types.GetOperationalDynamicConfigResponse{
		Value: ToDataBlob(t.Value),
	}
}

// FromAdminUpdateOperationalDynamicConfigRequest converts internal UpdateOperationalDynamicConfigRequest type to thrift
func FromAdminUpdateOperationalDynamicConfigRequest(t *types.UpdateOperationalDynamicConfigRequest) *admin.UpdateOperationalDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &admin.UpdateOperationalDynamicConfigRequest{
		ConfigName:   &t.ConfigName,
		ConfigValues: FromDynamicConfigValueArray(t.ConfigValues),
	}
}

// ToAdminUpdateOperationalDynamicConfigRequest converts thrift UpdateOperationalDynamicConfigRequest type to internal
func ToAdminUpdateOperationalDynamicConfigRequest(t *admin.UpdateOperationalDynamicConfigRequest) *types.UpdateOperationalDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.UpdateOperationalDynamicConfigRequest{
		ConfigName:   t.GetConfigName(),
		ConfigValues: ToDynamicConfigValueArray(t.ConfigValues),
	}
}

// FromAdminRestoreOperationalDynamicConfigRequest converts internal RestoreOperationalDynamicConfigRequest type to thrift
func FromAdminRestoreOperationalDynamicConfigRequest(t *types.RestoreOperationalDynamicConfigRequest) *admin.RestoreOperationalDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &admin.RestoreOperationalDynamicConfigRequest{
		ConfigName: &t.ConfigName,
		Filters:    FromDynamicConfigFilterArray(t.Filters),
	}
}

// ToAdminRestoreOperationalDynamicConfigRequest converts thrift RestoreOperationalDynamicConfigRequest type to internal
func ToAdminRestoreOperationalDynamicConfigRequest(t *admin.RestoreOperationalDynamicConfigRequest) *types.RestoreOperationalDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.RestoreOperationalDynamicConfigRequest{
		ConfigName: t.GetConfigName(),
		Filters:    ToDynamicConfigFilterArray(t.Filters),
	}
}

// FromAdminListOperationalDynamicConfigRequest converts internal ListOperationalDynamicConfigRequest type to thrift
func FromAdminListOperationalDynamicConfigRequest(t *types.ListOperationalDynamicConfigRequest) *admin.ListOperationalDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &admin.ListOperationalDynamicConfigRequest{
		ConfigName: &t.ConfigName,
	}
}

// ToAdminListOperationalDynamicConfigRequest converts thrift ListOperationalDynamicConfigRequest type to internal
func ToAdminListOperationalDynamicConfigRequest(t *admin.ListOperationalDynamicConfigRequest) *types.ListOperationalDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.ListOperationalDynamicConfigRequest{
		ConfigName: t.GetConfigName(),
	}
}

// FromAdminListOperationalDynamicConfigResponse converts internal ListOperationalDynamicConfigResponse type to thrift
func FromAdminListOperationalDynamicConfigResponse(t *types.ListOperationalDynamicConfigResponse) *admin.ListOperationalDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &admin.ListOperationalDynamicConfigResponse{
		Entries: FromDynamicConfigEntryArray(t.Entries),
	}
}

// ToAdminListOperationalDynamicConfigResponse converts thrift ListOperationalDynamicConfigResponse type to internal
func ToAdminListOperationalDynamicConfigResponse(t *admin.ListOperationalDynamicConfigResponse) *types.ListOperationalDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &types.ListOperationalDynamicConfigResponse{
		Entries: ToDynamicConfigEntryArray(t.Entries),
	}
}

func FromAdminGetGlobalIsolationGroupsRequest(t *types.GetGlobalIsolationGroupsRequest) *admin.GetGlobalIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	return &admin.GetGlobalIsolationGroupsRequest{}
}

func FromAdminGetDomainIsolationGroupsRequest(t *types.GetDomainIsolationGroupsRequest) *admin.GetDomainIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	return &admin.GetDomainIsolationGroupsRequest{
		Domain: &t.Domain,
	}
}

func FromAdminGetGlobalIsolationGroupsResponse(t *types.GetGlobalIsolationGroupsResponse) *admin.GetGlobalIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	cfg := FromIsolationGroupConfig(&t.IsolationGroups)
	if cfg == nil || cfg.GetIsolationGroups() == nil {
		return &admin.GetGlobalIsolationGroupsResponse{}
	}
	return &admin.GetGlobalIsolationGroupsResponse{
		IsolationGroups: cfg,
	}
}

func ToAdminGetGlobalIsolationGroupsResponse(t *admin.GetGlobalIsolationGroupsResponse) *types.GetGlobalIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	if t.IsolationGroups == nil {
		return &types.GetGlobalIsolationGroupsResponse{}
	}
	ig := ToIsolationGroupConfig(t.IsolationGroups)
	if ig == nil || len(*ig) == 0 {
		return &types.GetGlobalIsolationGroupsResponse{}
	}
	return &types.GetGlobalIsolationGroupsResponse{
		IsolationGroups: *ig,
	}
}

func ToAdminGetDomainIsolationGroupsResponse(t *admin.GetDomainIsolationGroupsResponse) *types.GetDomainIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	if t.IsolationGroups == nil {
		return &types.GetDomainIsolationGroupsResponse{}
	}
	ig := ToIsolationGroupConfig(t.IsolationGroups)
	if ig == nil || len(*ig) == 0 {
		return &types.GetDomainIsolationGroupsResponse{}
	}
	return &types.GetDomainIsolationGroupsResponse{
		IsolationGroups: *ig,
	}
}

func ToAdminGetGlobalIsolationGroupsRequest(t *admin.GetGlobalIsolationGroupsRequest) *types.GetGlobalIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	return &types.GetGlobalIsolationGroupsRequest{}
}

func FromAdminGetDomainIsolationGroupsResponse(t *types.GetDomainIsolationGroupsResponse) *admin.GetDomainIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	cfg := FromIsolationGroupConfig(&t.IsolationGroups)
	return &admin.GetDomainIsolationGroupsResponse{
		IsolationGroups: cfg,
	}
}

func ToAdminGetDomainIsolationGroupsRequest(t *admin.GetDomainIsolationGroupsRequest) *types.GetDomainIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	return &types.GetDomainIsolationGroupsRequest{Domain: t.GetDomain()}
}

func FromAdminUpdateGlobalIsolationGroupsResponse(t *types.UpdateGlobalIsolationGroupsResponse) *admin.UpdateGlobalIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	return &admin.UpdateGlobalIsolationGroupsResponse{}
}

func FromAdminUpdateGlobalIsolationGroupsRequest(t *types.UpdateGlobalIsolationGroupsRequest) *admin.UpdateGlobalIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	if t.IsolationGroups == nil {
		return &admin.UpdateGlobalIsolationGroupsRequest{}
	}
	return &admin.UpdateGlobalIsolationGroupsRequest{
		IsolationGroups: FromIsolationGroupConfig(&t.IsolationGroups),
	}
}

func FromAdminUpdateDomainIsolationGroupsRequest(t *types.UpdateDomainIsolationGroupsRequest) *admin.UpdateDomainIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	if t.IsolationGroups == nil {
		return &admin.UpdateDomainIsolationGroupsRequest{}
	}
	return &admin.UpdateDomainIsolationGroupsRequest{
		Domain:          &t.Domain,
		IsolationGroups: FromIsolationGroupConfig(&t.IsolationGroups),
	}
}

func ToAdminUpdateGlobalIsolationGroupsRequest(t *admin.UpdateGlobalIsolationGroupsRequest) *types.UpdateGlobalIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	cfg := ToIsolationGroupConfig(t.IsolationGroups)
	if cfg == nil {
		return &types.UpdateGlobalIsolationGroupsRequest{}
	}
	return &types.UpdateGlobalIsolationGroupsRequest{
		IsolationGroups: *cfg,
	}
}

func ToAdminUpdateGlobalIsolationGroupsResponse(t *admin.UpdateGlobalIsolationGroupsResponse) *types.UpdateGlobalIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	return &types.UpdateGlobalIsolationGroupsResponse{}
}

func ToAdminUpdateDomainIsolationGroupsResponse(t *admin.UpdateDomainIsolationGroupsResponse) *types.UpdateDomainIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	return &types.UpdateDomainIsolationGroupsResponse{}
}

func FromAdminUpdateDomainIsolationGroupsResponse(t *types.UpdateDomainIsolationGroupsResponse) *admin.UpdateDomainIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	return &admin.UpdateDomainIsolationGroupsResponse{}
}

func ToAdminUpdateDomainIsolationGroupsRequest(t *admin.UpdateDomainIsolationGroupsRequest) *types.UpdateDomainIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	cfg := ToIsolationGroupConfig(t.IsolationGroups)
	if cfg == nil {
		return &types.UpdateDomainIsolationGroupsRequest{
			Domain: t.GetDomain(),
		}
	}
	return &types.UpdateDomainIsolationGroupsRequest{
		Domain:          t.GetDomain(),
		IsolationGroups: *cfg,
	}
}

func FromIsolationGroupConfig(in *types.IsolationGroupConfiguration) *shared.IsolationGroupConfiguration {
	if in == nil {
		return nil
	}
	var out []*shared.IsolationGroupPartition
	for _, v := range *in {
		out = append(out, &shared.IsolationGroupPartition{
			Name:  strPtr(v.Name),
			State: igStatePtr(shared.IsolationGroupState(v.State)),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i] == nil || out[j] == nil {
			return false
		}
		return *out[i].Name < *out[j].Name
	})
	return &shared.IsolationGroupConfiguration{
		IsolationGroups: out,
	}
}

func ToIsolationGroupConfig(in *shared.IsolationGroupConfiguration) *types.IsolationGroupConfiguration {
	if in == nil {
		return nil
	}
	out := make(types.IsolationGroupConfiguration)
	for v := range in.IsolationGroups {
		out[in.IsolationGroups[v].GetName()] = types.IsolationGroupPartition{
			Name:  in.IsolationGroups[v].GetName(),
			State: types.IsolationGroupState(in.IsolationGroups[v].GetState()),
		}
	}
	return &out
}

func ToAdminGetDomainAsyncWorkflowConfiguratonRequest(in *admin.GetDomainAsyncWorkflowConfiguratonRequest) *types.GetDomainAsyncWorkflowConfiguratonRequest {
	if in == nil {
		return nil
	}
	return &types.GetDomainAsyncWorkflowConfiguratonRequest{
		Domain: in.GetDomain(),
	}
}

func FromAdminGetDomainAsyncWorkflowConfiguratonResponse(in *types.GetDomainAsyncWorkflowConfiguratonResponse) *admin.GetDomainAsyncWorkflowConfiguratonResponse {
	if in == nil {
		return nil
	}
	return &admin.GetDomainAsyncWorkflowConfiguratonResponse{
		Configuration: FromDomainAsyncWorkflowConfiguraton(in.Configuration),
	}
}

func FromDomainAsyncWorkflowConfiguraton(in *types.AsyncWorkflowConfiguration) *shared.AsyncWorkflowConfiguration {
	if in == nil {
		return nil
	}

	return &shared.AsyncWorkflowConfiguration{
		Enabled:             &in.Enabled,
		PredefinedQueueName: strPtr(in.PredefinedQueueName),
		QueueType:           strPtr(in.QueueType),
		QueueConfig:         FromDataBlob(in.QueueConfig),
	}
}

func ToAdminUpdateDomainAsyncWorkflowConfiguratonRequest(in *admin.UpdateDomainAsyncWorkflowConfiguratonRequest) *types.UpdateDomainAsyncWorkflowConfiguratonRequest {
	if in == nil {
		return nil
	}
	return &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
		Domain:        in.GetDomain(),
		Configuration: ToDomainAsyncWorkflowConfiguraton(in.Configuration),
	}
}

func ToDomainAsyncWorkflowConfiguraton(in *shared.AsyncWorkflowConfiguration) *types.AsyncWorkflowConfiguration {
	if in == nil {
		return nil
	}

	return &types.AsyncWorkflowConfiguration{
		Enabled:             in.GetEnabled(),
		PredefinedQueueName: in.GetPredefinedQueueName(),
		QueueType:           in.GetQueueType(),
		QueueConfig:         ToDataBlob(in.GetQueueConfig()),
	}
}

func FromAdminUpdateDomainAsyncWorkflowConfiguratonResponse(in *types.UpdateDomainAsyncWorkflowConfiguratonResponse) *admin.UpdateDomainAsyncWorkflowConfiguratonResponse {
	if in == nil {
		return nil
	}
	return &admin.UpdateDomainAsyncWorkflowConfiguratonResponse{}
}

func FromAdminGetDomainAsyncWorkflowConfiguratonRequest(in *types.GetDomainAsyncWorkflowConfiguratonRequest) *admin.GetDomainAsyncWorkflowConfiguratonRequest {
	if in == nil {
		return nil
	}
	return &admin.GetDomainAsyncWorkflowConfiguratonRequest{
		Domain: strPtr(in.Domain),
	}
}

func ToAdminGetDomainAsyncWorkflowConfiguratonResponse(in *admin.GetDomainAsyncWorkflowConfiguratonResponse) *types.GetDomainAsyncWorkflowConfiguratonResponse {
	if in == nil {
		return nil
	}
	return &types.GetDomainAsyncWorkflowConfiguratonResponse{
		Configuration: ToDomainAsyncWorkflowConfiguraton(in.Configuration),
	}
}

func FromAdminUpdateDomainAsyncWorkflowConfiguratonRequest(in *types.UpdateDomainAsyncWorkflowConfiguratonRequest) *admin.UpdateDomainAsyncWorkflowConfiguratonRequest {
	if in == nil {
		return nil
	}
	return &admin.UpdateDomainAsyncWorkflowConfiguratonRequest{
		Domain:        strPtr(in.Domain),
		Configuration: FromDomainAsyncWorkflowConfiguraton(in.Configuration),
	}
}

func ToAdminUpdateDomainAsyncWorkflowConfiguratonResponse(in *admin.UpdateDomainAsyncWorkflowConfiguratonResponse) *types.UpdateDomainAsyncWorkflowConfiguratonResponse {
	if in == nil {
		return nil
	}
	return &types.UpdateDomainAsyncWorkflowConfiguratonResponse{}
}

func strPtr(s string) *string                                             { return &s }
func igStatePtr(s shared.IsolationGroupState) *shared.IsolationGroupState { return &s }
