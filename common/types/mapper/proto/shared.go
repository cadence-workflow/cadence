// Copyright (c) 2021 Uber Technologies Inc.
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

package proto

import (
	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"

	sharedv1 "github.com/uber/cadence/.gen/proto/shared/v1"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func FromHostInfo(t *types.HostInfo) *adminv1.HostInfo {
	if t == nil {
		return nil
	}
	return &adminv1.HostInfo{
		Identity: t.Identity,
	}
}

func ToHostInfo(t *adminv1.HostInfo) *types.HostInfo {
	if t == nil {
		return nil
	}
	return &types.HostInfo{
		Identity: t.Identity,
	}
}

func FromMembershipInfo(t *types.MembershipInfo) *adminv1.MembershipInfo {
	if t == nil {
		return nil
	}
	return &adminv1.MembershipInfo{
		CurrentHost:      FromHostInfo(t.CurrentHost),
		ReachableMembers: t.ReachableMembers,
		Rings:            FromRingInfoArray(t.Rings),
	}
}

func FromPersistenceSettings(t []*types.PersistenceSetting) []*adminv1.PersistenceSetting {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.PersistenceSetting, len(t))
	for i := range t {
		v[i] = FromPersistenceSetting(t[i])
	}
	return v
}

func FromPersistenceSetting(t *types.PersistenceSetting) *adminv1.PersistenceSetting {
	if t == nil {
		return nil
	}
	return &adminv1.PersistenceSetting{
		Key:   t.Key,
		Value: t.Value,
	}
}

func FromPersistenceFeatures(t []*types.PersistenceFeature) []*adminv1.PersistenceFeature {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.PersistenceFeature, len(t))
	for i := range t {
		v[i] = FromPersistenceFeature(t[i])
	}
	return v
}

func FromPersistenceFeature(t *types.PersistenceFeature) *adminv1.PersistenceFeature {
	if t == nil {
		return nil
	}
	return &adminv1.PersistenceFeature{
		Key:     t.Key,
		Enabled: t.Enabled,
	}
}

func FromPersistenceInfoMap(t map[string]*types.PersistenceInfo) map[string]*adminv1.PersistenceInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*adminv1.PersistenceInfo, len(t))
	for key := range t {
		v[key] = FromPersistenceInfo(t[key])
	}

	return v
}

func FromPersistenceInfo(t *types.PersistenceInfo) *adminv1.PersistenceInfo {
	if t == nil {
		return nil
	}

	return &adminv1.PersistenceInfo{
		Backend:  t.Backend,
		Settings: FromPersistenceSettings(t.Settings),
		Features: FromPersistenceFeatures(t.Features),
	}
}

func ToPersistenceSettings(t []*adminv1.PersistenceSetting) []*types.PersistenceSetting {
	if t == nil {
		return nil
	}
	v := make([]*types.PersistenceSetting, len(t))
	for i := range t {
		v[i] = ToPersistenceSetting(t[i])
	}
	return v
}

func ToPersistenceSetting(t *adminv1.PersistenceSetting) *types.PersistenceSetting {
	if t == nil {
		return nil
	}
	return &types.PersistenceSetting{
		Key:   t.Key,
		Value: t.Value,
	}
}

func ToPersistenceFeatures(t []*adminv1.PersistenceFeature) []*types.PersistenceFeature {
	if t == nil {
		return nil
	}
	v := make([]*types.PersistenceFeature, len(t))
	for i := range t {
		v[i] = ToPersistenceFeature(t[i])
	}
	return v
}

func ToPersistenceFeature(t *adminv1.PersistenceFeature) *types.PersistenceFeature {
	if t == nil {
		return nil
	}
	return &types.PersistenceFeature{
		Key:     t.Key,
		Enabled: t.Enabled,
	}
}

func ToPersistenceInfoMap(t map[string]*adminv1.PersistenceInfo) map[string]*types.PersistenceInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.PersistenceInfo, len(t))
	for key := range t {
		v[key] = ToPersistenceInfo(t[key])
	}

	return v
}

func ToPersistenceInfo(t *adminv1.PersistenceInfo) *types.PersistenceInfo {
	if t == nil {
		return nil
	}

	return &types.PersistenceInfo{
		Backend:  t.Backend,
		Settings: ToPersistenceSettings(t.Settings),
		Features: ToPersistenceFeatures(t.Features),
	}
}

func ToMembershipInfo(t *adminv1.MembershipInfo) *types.MembershipInfo {
	if t == nil {
		return nil
	}
	return &types.MembershipInfo{
		CurrentHost:      ToHostInfo(t.CurrentHost),
		ReachableMembers: t.ReachableMembers,
		Rings:            ToRingInfoArray(t.Rings),
	}
}

func FromDomainCacheInfo(t *types.DomainCacheInfo) *adminv1.DomainCacheInfo {
	if t == nil {
		return nil
	}
	return &adminv1.DomainCacheInfo{
		NumOfItemsInCacheById:   t.NumOfItemsInCacheByID,
		NumOfItemsInCacheByName: t.NumOfItemsInCacheByName,
	}
}

func ToDomainCacheInfo(t *adminv1.DomainCacheInfo) *types.DomainCacheInfo {
	if t == nil {
		return nil
	}
	return &types.DomainCacheInfo{
		NumOfItemsInCacheByID:   t.NumOfItemsInCacheById,
		NumOfItemsInCacheByName: t.NumOfItemsInCacheByName,
	}
}

func FromRingInfo(t *types.RingInfo) *adminv1.RingInfo {
	if t == nil {
		return nil
	}
	return &adminv1.RingInfo{
		Role:        t.Role,
		MemberCount: t.MemberCount,
		Members:     FromHostInfoArray(t.Members),
	}
}

func ToRingInfo(t *adminv1.RingInfo) *types.RingInfo {
	if t == nil {
		return nil
	}
	return &types.RingInfo{
		Role:        t.Role,
		MemberCount: t.MemberCount,
		Members:     ToHostInfoArray(t.Members),
	}
}

func FromTaskSource(t *types.TaskSource) sharedv1.TaskSource {
	if t == nil {
		return sharedv1.TaskSource_TASK_SOURCE_INVALID
	}
	switch *t {
	case types.TaskSourceHistory:
		return sharedv1.TaskSource_TASK_SOURCE_HISTORY
	case types.TaskSourceDbBacklog:
		return sharedv1.TaskSource_TASK_SOURCE_DB_BACKLOG
	}
	return sharedv1.TaskSource_TASK_SOURCE_INVALID
}

func ToTaskSource(t sharedv1.TaskSource) *types.TaskSource {
	switch t {
	case sharedv1.TaskSource_TASK_SOURCE_INVALID:
		return nil
	case sharedv1.TaskSource_TASK_SOURCE_HISTORY:
		return types.TaskSourceHistory.Ptr()
	case sharedv1.TaskSource_TASK_SOURCE_DB_BACKLOG:
		return types.TaskSourceDbBacklog.Ptr()
	}
	return nil
}

func FromTransientDecisionInfo(t *types.TransientDecisionInfo) *sharedv1.TransientDecisionInfo {
	if t == nil {
		return nil
	}
	return &sharedv1.TransientDecisionInfo{
		ScheduledEvent: FromHistoryEvent(t.ScheduledEvent),
		StartedEvent:   FromHistoryEvent(t.StartedEvent),
	}
}

func ToTransientDecisionInfo(t *sharedv1.TransientDecisionInfo) *types.TransientDecisionInfo {
	if t == nil {
		return nil
	}
	return &types.TransientDecisionInfo{
		ScheduledEvent: ToHistoryEvent(t.ScheduledEvent),
		StartedEvent:   ToHistoryEvent(t.StartedEvent),
	}
}

func FromVersionHistories(t *types.VersionHistories) *sharedv1.VersionHistories {
	if t == nil {
		return nil
	}
	return &sharedv1.VersionHistories{
		CurrentVersionHistoryIndex: t.CurrentVersionHistoryIndex,
		Histories:                  FromVersionHistoryArray(t.Histories),
	}
}

func ToVersionHistories(t *sharedv1.VersionHistories) *types.VersionHistories {
	if t == nil {
		return nil
	}
	return &types.VersionHistories{
		CurrentVersionHistoryIndex: t.CurrentVersionHistoryIndex,
		Histories:                  ToVersionHistoryArray(t.Histories),
	}
}

func FromVersionHistory(t *types.VersionHistory) *adminv1.VersionHistory {
	if t == nil {
		return nil
	}
	return &adminv1.VersionHistory{
		BranchToken: t.BranchToken,
		Items:       FromVersionHistoryItemArray(t.Items),
	}
}

func ToVersionHistory(t *adminv1.VersionHistory) *types.VersionHistory {
	if t == nil {
		return nil
	}
	return &types.VersionHistory{
		BranchToken: t.BranchToken,
		Items:       ToVersionHistoryItemArray(t.Items),
	}
}

func FromVersionHistoryItem(t *types.VersionHistoryItem) *adminv1.VersionHistoryItem {
	if t == nil {
		return nil
	}
	return &adminv1.VersionHistoryItem{
		EventId: t.EventID,
		Version: t.Version,
	}
}

func ToVersionHistoryItem(t *adminv1.VersionHistoryItem) *types.VersionHistoryItem {
	if t == nil {
		return nil
	}
	return &types.VersionHistoryItem{
		EventID: t.EventId,
		Version: t.Version,
	}
}

func FromWorkflowState(t *int32) sharedv1.WorkflowState {
	if t == nil {
		return sharedv1.WorkflowState_WORKFLOW_STATE_INVALID
	}
	switch *t {
	case persistence.WorkflowStateCreated:
		return sharedv1.WorkflowState_WORKFLOW_STATE_CREATED
	case persistence.WorkflowStateRunning:
		return sharedv1.WorkflowState_WORKFLOW_STATE_RUNNING
	case persistence.WorkflowStateCompleted:
		return sharedv1.WorkflowState_WORKFLOW_STATE_COMPLETED
	case persistence.WorkflowStateZombie:
		return sharedv1.WorkflowState_WORKFLOW_STATE_ZOMBIE
	case persistence.WorkflowStateVoid:
		return sharedv1.WorkflowState_WORKFLOW_STATE_VOID
	case persistence.WorkflowStateCorrupted:
		return sharedv1.WorkflowState_WORKFLOW_STATE_CORRUPTED
	}
	return sharedv1.WorkflowState_WORKFLOW_STATE_INVALID
}

func ToWorkflowState(t sharedv1.WorkflowState) *int32 {
	switch t {
	case sharedv1.WorkflowState_WORKFLOW_STATE_INVALID:
		return nil
	case sharedv1.WorkflowState_WORKFLOW_STATE_CREATED:
		v := int32(persistence.WorkflowStateCreated)
		return &v
	case sharedv1.WorkflowState_WORKFLOW_STATE_RUNNING:
		v := int32(persistence.WorkflowStateRunning)
		return &v
	case sharedv1.WorkflowState_WORKFLOW_STATE_COMPLETED:
		v := int32(persistence.WorkflowStateCompleted)
		return &v
	case sharedv1.WorkflowState_WORKFLOW_STATE_ZOMBIE:
		v := int32(persistence.WorkflowStateZombie)
		return &v
	case sharedv1.WorkflowState_WORKFLOW_STATE_VOID:
		v := int32(persistence.WorkflowStateVoid)
		return &v
	case sharedv1.WorkflowState_WORKFLOW_STATE_CORRUPTED:
		v := int32(persistence.WorkflowStateCorrupted)
		return &v
	}
	return nil
}

func FromHostInfoArray(t []*types.HostInfo) []*adminv1.HostInfo {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.HostInfo, len(t))
	for i := range t {
		v[i] = FromHostInfo(t[i])
	}
	return v
}

func ToHostInfoArray(t []*adminv1.HostInfo) []*types.HostInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.HostInfo, len(t))
	for i := range t {
		v[i] = ToHostInfo(t[i])
	}
	return v
}

func FromVersionHistoryArray(t []*types.VersionHistory) []*adminv1.VersionHistory {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.VersionHistory, len(t))
	for i := range t {
		v[i] = FromVersionHistory(t[i])
	}
	return v
}

func ToVersionHistoryArray(t []*adminv1.VersionHistory) []*types.VersionHistory {
	if t == nil {
		return nil
	}
	v := make([]*types.VersionHistory, len(t))
	for i := range t {
		v[i] = ToVersionHistory(t[i])
	}
	return v
}

func FromRingInfoArray(t []*types.RingInfo) []*adminv1.RingInfo {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.RingInfo, len(t))
	for i := range t {
		v[i] = FromRingInfo(t[i])
	}
	return v
}

func ToRingInfoArray(t []*adminv1.RingInfo) []*types.RingInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.RingInfo, len(t))
	for i := range t {
		v[i] = ToRingInfo(t[i])
	}
	return v
}

func FromDLQType(t *types.DLQType) adminv1.DLQType {
	if t == nil {
		return adminv1.DLQType_DLQ_TYPE_INVALID
	}
	switch *t {
	case types.DLQTypeReplication:
		return adminv1.DLQType_DLQ_TYPE_REPLICATION
	case types.DLQTypeDomain:
		return adminv1.DLQType_DLQ_TYPE_DOMAIN
	}
	return adminv1.DLQType_DLQ_TYPE_INVALID
}

func ToDLQType(t adminv1.DLQType) *types.DLQType {
	switch t {
	case adminv1.DLQType_DLQ_TYPE_INVALID:
		return nil
	case adminv1.DLQType_DLQ_TYPE_REPLICATION:
		return types.DLQTypeReplication.Ptr()
	case adminv1.DLQType_DLQ_TYPE_DOMAIN:
		return types.DLQTypeDomain.Ptr()
	}
	return nil
}

func FromDomainOperation(t *types.DomainOperation) adminv1.DomainOperation {
	if t == nil {
		return adminv1.DomainOperation_DOMAIN_OPERATION_INVALID
	}
	switch *t {
	case types.DomainOperationCreate:
		return adminv1.DomainOperation_DOMAIN_OPERATION_CREATE
	case types.DomainOperationUpdate:
		return adminv1.DomainOperation_DOMAIN_OPERATION_UPDATE
	case types.DomainOperationDelete:
		return adminv1.DomainOperation_DOMAIN_OPERATION_DELETE
	}
	return adminv1.DomainOperation_DOMAIN_OPERATION_INVALID
}

func ToDomainOperation(t adminv1.DomainOperation) *types.DomainOperation {
	switch t {
	case adminv1.DomainOperation_DOMAIN_OPERATION_INVALID:
		return nil
	case adminv1.DomainOperation_DOMAIN_OPERATION_CREATE:
		return types.DomainOperationCreate.Ptr()
	case adminv1.DomainOperation_DOMAIN_OPERATION_UPDATE:
		return types.DomainOperationUpdate.Ptr()
	case adminv1.DomainOperation_DOMAIN_OPERATION_DELETE:
		return types.DomainOperationDelete.Ptr()
	}
	return nil
}

func FromDomainTaskAttributes(t *types.DomainTaskAttributes) *adminv1.DomainTaskAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.DomainTaskAttributes{
		DomainOperation: FromDomainOperation(t.DomainOperation),
		Id:              t.ID,
		Domain: FromDescribeDomainResponseDomain(&types.DescribeDomainResponse{
			DomainInfo:               t.Info,
			Configuration:            t.Config,
			ReplicationConfiguration: t.ReplicationConfig,
		}),
		ConfigVersion:           t.ConfigVersion,
		FailoverVersion:         t.FailoverVersion,
		PreviousFailoverVersion: t.PreviousFailoverVersion,
	}
}

func ToDomainTaskAttributes(t *adminv1.DomainTaskAttributes) *types.DomainTaskAttributes {
	if t == nil {
		return nil
	}
	domain := ToDescribeDomainResponseDomain(t.Domain)
	return &types.DomainTaskAttributes{
		DomainOperation:         ToDomainOperation(t.DomainOperation),
		ID:                      t.Id,
		Info:                    domain.DomainInfo,
		Config:                  domain.Configuration,
		ReplicationConfig:       domain.ReplicationConfiguration,
		ConfigVersion:           t.ConfigVersion,
		FailoverVersion:         t.FailoverVersion,
		PreviousFailoverVersion: t.PreviousFailoverVersion,
	}
}

func FromFailoverMarkerAttributes(t *types.FailoverMarkerAttributes) *adminv1.FailoverMarkerAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.FailoverMarkerAttributes{
		DomainId:        t.DomainID,
		FailoverVersion: t.FailoverVersion,
		CreationTime:    unixNanoToTime(t.CreationTime),
	}
}

func ToFailoverMarkerAttributes(t *adminv1.FailoverMarkerAttributes) *types.FailoverMarkerAttributes {
	if t == nil {
		return nil
	}
	return &types.FailoverMarkerAttributes{
		DomainID:        t.DomainId,
		FailoverVersion: t.FailoverVersion,
		CreationTime:    timeToUnixNano(t.CreationTime),
	}
}

func FromFailoverMarkerToken(t *types.FailoverMarkerToken) *adminv1.FailoverMarkerToken {
	if t == nil {
		return nil
	}
	return &adminv1.FailoverMarkerToken{
		ShardIds:       t.ShardIDs,
		FailoverMarker: FromFailoverMarkerAttributes(t.FailoverMarker),
	}
}

func ToFailoverMarkerToken(t *adminv1.FailoverMarkerToken) *types.FailoverMarkerToken {
	if t == nil {
		return nil
	}
	return &types.FailoverMarkerToken{
		ShardIDs:       t.ShardIds,
		FailoverMarker: ToFailoverMarkerAttributes(t.FailoverMarker),
	}
}

func FromHistoryTaskV2Attributes(t *types.HistoryTaskV2Attributes) *adminv1.HistoryTaskV2Attributes {
	if t == nil {
		return nil
	}
	return &adminv1.HistoryTaskV2Attributes{
		DomainId:            t.DomainID,
		WorkflowExecution:   FromWorkflowRunPair(t.WorkflowID, t.RunID),
		VersionHistoryItems: FromVersionHistoryItemArray(t.VersionHistoryItems),
		Events:              FromDataBlob(t.Events),
		NewRunEvents:        FromDataBlob(t.NewRunEvents),
	}
}

func ToHistoryTaskV2Attributes(t *adminv1.HistoryTaskV2Attributes) *types.HistoryTaskV2Attributes {
	if t == nil {
		return nil
	}
	return &types.HistoryTaskV2Attributes{
		DomainID:            t.DomainId,
		WorkflowID:          ToWorkflowID(t.WorkflowExecution),
		RunID:               ToRunID(t.WorkflowExecution),
		VersionHistoryItems: ToVersionHistoryItemArray(t.VersionHistoryItems),
		Events:              ToDataBlob(t.Events),
		NewRunEvents:        ToDataBlob(t.NewRunEvents),
	}
}

func FromReplicationMessages(t *types.ReplicationMessages) *adminv1.ReplicationMessages {
	if t == nil {
		return nil
	}
	return &adminv1.ReplicationMessages{
		ReplicationTasks:       FromReplicationTaskArray(t.ReplicationTasks),
		LastRetrievedMessageId: t.LastRetrievedMessageID,
		HasMore:                t.HasMore,
		SyncShardStatus:        FromSyncShardStatus(t.SyncShardStatus),
	}
}

func ToReplicationMessages(t *adminv1.ReplicationMessages) *types.ReplicationMessages {
	if t == nil {
		return nil
	}
	return &types.ReplicationMessages{
		ReplicationTasks:       ToReplicationTaskArray(t.ReplicationTasks),
		LastRetrievedMessageID: t.LastRetrievedMessageId,
		HasMore:                t.HasMore,
		SyncShardStatus:        ToSyncShardStatus(t.SyncShardStatus),
	}
}

// ReplicationMessagesSize returns the size (in bytes) of the types.ReplicationMessages
func ReplicationMessagesSize(t *types.ReplicationMessages) int {
	if t == nil {
		return 0
	}
	return FromReplicationMessages(t).Size()
}

func FromReplicationTaskInfo(t *types.ReplicationTaskInfo) *adminv1.ReplicationTaskInfo {
	if t == nil {
		return nil
	}
	return &adminv1.ReplicationTaskInfo{
		DomainId:          t.DomainID,
		WorkflowExecution: FromWorkflowRunPair(t.WorkflowID, t.RunID),
		TaskType:          int32(t.TaskType),
		TaskId:            t.TaskID,
		Version:           t.Version,
		FirstEventId:      t.FirstEventID,
		NextEventId:       t.NextEventID,
		ScheduledId:       t.ScheduledID,
	}
}

func ToReplicationTaskInfo(t *adminv1.ReplicationTaskInfo) *types.ReplicationTaskInfo {
	if t == nil {
		return nil
	}
	return &types.ReplicationTaskInfo{
		DomainID:     t.DomainId,
		WorkflowID:   ToWorkflowID(t.WorkflowExecution),
		RunID:        ToRunID(t.WorkflowExecution),
		TaskType:     int16(t.TaskType),
		TaskID:       t.TaskId,
		Version:      t.Version,
		FirstEventID: t.FirstEventId,
		NextEventID:  t.NextEventId,
		ScheduledID:  t.ScheduledId,
	}
}

func FromReplicationTaskType(t *types.ReplicationTaskType) adminv1.ReplicationTaskType {
	if t == nil {
		return adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_INVALID
	}
	switch *t {
	case types.ReplicationTaskTypeDomain:
		return adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_DOMAIN
	case types.ReplicationTaskTypeHistory:
		return adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_HISTORY
	case types.ReplicationTaskTypeSyncShardStatus:
		return adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_SYNC_SHARD_STATUS
	case types.ReplicationTaskTypeSyncActivity:
		return adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_SYNC_ACTIVITY
	case types.ReplicationTaskTypeHistoryMetadata:
		return adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_HISTORY_METADATA
	case types.ReplicationTaskTypeHistoryV2:
		return adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_HISTORY_V2
	case types.ReplicationTaskTypeFailoverMarker:
		return adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_FAILOVER_MARKER
	}
	return adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_INVALID
}

func ToReplicationTaskType(t adminv1.ReplicationTaskType) *types.ReplicationTaskType {
	switch t {
	case adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_INVALID:
		return nil
	case adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_DOMAIN:
		return types.ReplicationTaskTypeDomain.Ptr()
	case adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_HISTORY:
		return types.ReplicationTaskTypeHistory.Ptr()
	case adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_SYNC_SHARD_STATUS:
		return types.ReplicationTaskTypeSyncShardStatus.Ptr()
	case adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_SYNC_ACTIVITY:
		return types.ReplicationTaskTypeSyncActivity.Ptr()
	case adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_HISTORY_METADATA:
		return types.ReplicationTaskTypeHistoryMetadata.Ptr()
	case adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_HISTORY_V2:
		return types.ReplicationTaskTypeHistoryV2.Ptr()
	case adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_FAILOVER_MARKER:
		return types.ReplicationTaskTypeFailoverMarker.Ptr()
	}
	return nil
}

func FromReplicationToken(t *types.ReplicationToken) *adminv1.ReplicationToken {
	if t == nil {
		return nil
	}
	return &adminv1.ReplicationToken{
		ShardId:                t.ShardID,
		LastRetrievedMessageId: t.LastRetrievedMessageID,
		LastProcessedMessageId: t.LastProcessedMessageID,
	}
}

func ToReplicationToken(t *adminv1.ReplicationToken) *types.ReplicationToken {
	if t == nil {
		return nil
	}
	return &types.ReplicationToken{
		ShardID:                t.ShardId,
		LastRetrievedMessageID: t.LastRetrievedMessageId,
		LastProcessedMessageID: t.LastProcessedMessageId,
	}
}

func FromSyncActivityTaskAttributes(t *types.SyncActivityTaskAttributes) *adminv1.SyncActivityTaskAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.SyncActivityTaskAttributes{
		DomainId:           t.DomainID,
		WorkflowExecution:  FromWorkflowRunPair(t.WorkflowID, t.RunID),
		Version:            t.Version,
		ScheduledId:        t.ScheduledID,
		ScheduledTime:      unixNanoToTime(t.ScheduledTime),
		StartedId:          t.StartedID,
		StartedTime:        unixNanoToTime(t.StartedTime),
		LastHeartbeatTime:  unixNanoToTime(t.LastHeartbeatTime),
		Details:            FromPayload(t.Details),
		Attempt:            t.Attempt,
		LastFailure:        FromFailure(t.LastFailureReason, t.LastFailureDetails),
		LastWorkerIdentity: t.LastWorkerIdentity,
		VersionHistory:     FromVersionHistory(t.VersionHistory),
	}
}

func ToSyncActivityTaskAttributes(t *adminv1.SyncActivityTaskAttributes) *types.SyncActivityTaskAttributes {
	if t == nil {
		return nil
	}
	return &types.SyncActivityTaskAttributes{
		DomainID:           t.DomainId,
		WorkflowID:         ToWorkflowID(t.WorkflowExecution),
		RunID:              ToRunID(t.WorkflowExecution),
		Version:            t.Version,
		ScheduledID:        t.ScheduledId,
		ScheduledTime:      timeToUnixNano(t.ScheduledTime),
		StartedID:          t.StartedId,
		StartedTime:        timeToUnixNano(t.StartedTime),
		LastHeartbeatTime:  timeToUnixNano(t.LastHeartbeatTime),
		Details:            ToPayload(t.Details),
		Attempt:            t.Attempt,
		LastFailureReason:  ToFailureReason(t.LastFailure),
		LastFailureDetails: ToFailureDetails(t.LastFailure),
		LastWorkerIdentity: t.LastWorkerIdentity,
		VersionHistory:     ToVersionHistory(t.VersionHistory),
	}
}

func FromSyncShardStatus(t *types.SyncShardStatus) *adminv1.SyncShardStatus {
	if t == nil {
		return nil
	}
	return &adminv1.SyncShardStatus{
		Timestamp: unixNanoToTime(t.Timestamp),
	}
}

func ToSyncShardStatus(t *adminv1.SyncShardStatus) *types.SyncShardStatus {
	if t == nil {
		return nil
	}
	return &types.SyncShardStatus{
		Timestamp: timeToUnixNano(t.Timestamp),
	}
}

func FromSyncShardStatusTaskAttributes(t *types.SyncShardStatusTaskAttributes) *adminv1.SyncShardStatusTaskAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.SyncShardStatusTaskAttributes{
		SourceCluster: t.SourceCluster,
		ShardId:       int32(t.ShardID),
		Timestamp:     unixNanoToTime(t.Timestamp),
	}
}

func ToSyncShardStatusTaskAttributes(t *adminv1.SyncShardStatusTaskAttributes) *types.SyncShardStatusTaskAttributes {
	if t == nil {
		return nil
	}
	return &types.SyncShardStatusTaskAttributes{
		SourceCluster: t.SourceCluster,
		ShardID:       int64(t.ShardId),
		Timestamp:     timeToUnixNano(t.Timestamp),
	}
}

func FromReplicationTaskInfoArray(t []*types.ReplicationTaskInfo) []*adminv1.ReplicationTaskInfo {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.ReplicationTaskInfo, len(t))
	for i := range t {
		v[i] = FromReplicationTaskInfo(t[i])
	}
	return v
}

func ToReplicationTaskInfoArray(t []*adminv1.ReplicationTaskInfo) []*types.ReplicationTaskInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.ReplicationTaskInfo, len(t))
	for i := range t {
		v[i] = ToReplicationTaskInfo(t[i])
	}
	return v
}

func FromReplicationTaskArray(t []*types.ReplicationTask) []*adminv1.ReplicationTask {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.ReplicationTask, len(t))
	for i := range t {
		v[i] = FromReplicationTask(t[i])
	}
	return v
}

func ToReplicationTaskArray(t []*adminv1.ReplicationTask) []*types.ReplicationTask {
	if t == nil {
		return nil
	}
	v := make([]*types.ReplicationTask, len(t))
	for i := range t {
		v[i] = ToReplicationTask(t[i])
	}
	return v
}

func FromReplicationTokenArray(t []*types.ReplicationToken) []*adminv1.ReplicationToken {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.ReplicationToken, len(t))
	for i := range t {
		v[i] = FromReplicationToken(t[i])
	}
	return v
}

func ToReplicationTokenArray(t []*adminv1.ReplicationToken) []*types.ReplicationToken {
	if t == nil {
		return nil
	}
	v := make([]*types.ReplicationToken, len(t))
	for i := range t {
		v[i] = ToReplicationToken(t[i])
	}
	return v
}

func FromReplicationMessagesMap(t map[int32]*types.ReplicationMessages) map[int32]*adminv1.ReplicationMessages {
	if t == nil {
		return nil
	}
	v := make(map[int32]*adminv1.ReplicationMessages, len(t))
	for key := range t {
		v[key] = FromReplicationMessages(t[key])
	}
	return v
}

func ToReplicationMessagesMap(t map[int32]*adminv1.ReplicationMessages) map[int32]*types.ReplicationMessages {
	if t == nil {
		return nil
	}
	v := make(map[int32]*types.ReplicationMessages, len(t))
	for key := range t {
		v[key] = ToReplicationMessages(t[key])
	}
	return v
}

func FromReplicationTask(t *types.ReplicationTask) *adminv1.ReplicationTask {
	if t == nil {
		return nil
	}
	task := adminv1.ReplicationTask{
		TaskType:     FromReplicationTaskType(t.TaskType),
		SourceTaskId: t.SourceTaskID,
		CreationTime: unixNanoToTime(t.CreationTime),
	}
	if t.DomainTaskAttributes != nil {
		task.Attributes = &adminv1.ReplicationTask_DomainTaskAttributes{
			DomainTaskAttributes: FromDomainTaskAttributes(t.DomainTaskAttributes),
		}
	}
	if t.SyncShardStatusTaskAttributes != nil {
		task.Attributes = &adminv1.ReplicationTask_SyncShardStatusTaskAttributes{
			SyncShardStatusTaskAttributes: FromSyncShardStatusTaskAttributes(t.SyncShardStatusTaskAttributes),
		}
	}
	if t.SyncActivityTaskAttributes != nil {
		task.Attributes = &adminv1.ReplicationTask_SyncActivityTaskAttributes{
			SyncActivityTaskAttributes: FromSyncActivityTaskAttributes(t.SyncActivityTaskAttributes),
		}
	}
	if t.HistoryTaskV2Attributes != nil {
		task.Attributes = &adminv1.ReplicationTask_HistoryTaskV2Attributes{
			HistoryTaskV2Attributes: FromHistoryTaskV2Attributes(t.HistoryTaskV2Attributes),
		}
	}
	if t.FailoverMarkerAttributes != nil {
		task.Attributes = &adminv1.ReplicationTask_FailoverMarkerAttributes{
			FailoverMarkerAttributes: FromFailoverMarkerAttributes(t.FailoverMarkerAttributes),
		}
	}

	return &task
}

func ToReplicationTask(t *adminv1.ReplicationTask) *types.ReplicationTask {
	if t == nil {
		return nil
	}
	task := types.ReplicationTask{
		TaskType:     ToReplicationTaskType(t.TaskType),
		SourceTaskID: t.SourceTaskId,
		CreationTime: timeToUnixNano(t.CreationTime),
	}

	switch attr := t.Attributes.(type) {
	case *adminv1.ReplicationTask_DomainTaskAttributes:
		task.DomainTaskAttributes = ToDomainTaskAttributes(attr.DomainTaskAttributes)
	case *adminv1.ReplicationTask_SyncShardStatusTaskAttributes:
		task.SyncShardStatusTaskAttributes = ToSyncShardStatusTaskAttributes(attr.SyncShardStatusTaskAttributes)
	case *adminv1.ReplicationTask_SyncActivityTaskAttributes:
		task.SyncActivityTaskAttributes = ToSyncActivityTaskAttributes(attr.SyncActivityTaskAttributes)
	case *adminv1.ReplicationTask_HistoryTaskV2Attributes:
		task.HistoryTaskV2Attributes = ToHistoryTaskV2Attributes(attr.HistoryTaskV2Attributes)
	case *adminv1.ReplicationTask_FailoverMarkerAttributes:
		task.FailoverMarkerAttributes = ToFailoverMarkerAttributes(attr.FailoverMarkerAttributes)
	}
	return &task
}

func FromTaskType(t *int32) adminv1.TaskType {
	if t == nil {
		return adminv1.TaskType_TASK_TYPE_INVALID
	}
	switch constants.TaskType(*t) {
	case constants.TaskTypeTransfer:
		return adminv1.TaskType_TASK_TYPE_TRANSFER
	case constants.TaskTypeTimer:
		return adminv1.TaskType_TASK_TYPE_TIMER
	case constants.TaskTypeReplication:
		return adminv1.TaskType_TASK_TYPE_REPLICATION
	case constants.TaskTypeCrossCluster:
		return adminv1.TaskType_TASK_TYPE_CROSS_CLUSTER
	}
	return adminv1.TaskType_TASK_TYPE_INVALID
}

func ToTaskType(t adminv1.TaskType) *int32 {
	switch t {
	case adminv1.TaskType_TASK_TYPE_INVALID:
		return nil
	case adminv1.TaskType_TASK_TYPE_TRANSFER:
		return common.Int32Ptr(int32(constants.TaskTypeTransfer))
	case adminv1.TaskType_TASK_TYPE_TIMER:
		return common.Int32Ptr(int32(constants.TaskTypeTimer))
	case adminv1.TaskType_TASK_TYPE_REPLICATION:
		return common.Int32Ptr(int32(constants.TaskTypeReplication))
	case adminv1.TaskType_TASK_TYPE_CROSS_CLUSTER:
		return common.Int32Ptr(int32(constants.TaskTypeCrossCluster))
	}
	return nil
}

func FromFailoverMarkerTokenArray(t []*types.FailoverMarkerToken) []*adminv1.FailoverMarkerToken {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.FailoverMarkerToken, len(t))
	for i := range t {
		v[i] = FromFailoverMarkerToken(t[i])
	}
	return v
}

func ToFailoverMarkerTokenArray(t []*adminv1.FailoverMarkerToken) []*types.FailoverMarkerToken {
	if t == nil {
		return nil
	}
	v := make([]*types.FailoverMarkerToken, len(t))
	for i := range t {
		v[i] = ToFailoverMarkerToken(t[i])
	}
	return v
}

func FromVersionHistoryItemArray(t []*types.VersionHistoryItem) []*adminv1.VersionHistoryItem {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.VersionHistoryItem, len(t))
	for i := range t {
		v[i] = FromVersionHistoryItem(t[i])
	}
	return v
}

func ToVersionHistoryItemArray(t []*adminv1.VersionHistoryItem) []*types.VersionHistoryItem {
	if t == nil {
		return nil
	}
	v := make([]*types.VersionHistoryItem, len(t))
	for i := range t {
		v[i] = ToVersionHistoryItem(t[i])
	}
	return v
}

func FromEventIDVersionPair(id, version *int64) *adminv1.VersionHistoryItem {
	if id == nil || version == nil {
		return nil
	}
	return &adminv1.VersionHistoryItem{
		EventId: *id,
		Version: *version,
	}
}

func ToEventID(item *adminv1.VersionHistoryItem) *int64 {
	if item == nil {
		return nil
	}
	return common.Int64Ptr(item.EventId)
}

func ToEventVersion(item *adminv1.VersionHistoryItem) *int64 {
	if item == nil {
		return nil
	}
	return common.Int64Ptr(item.Version)
}

// FromCrossClusterTaskType converts internal CrossClusterTaskType type to proto
func FromCrossClusterTaskType(t *types.CrossClusterTaskType) adminv1.CrossClusterTaskType {
	if t == nil {
		return adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_INVALID
	}
	switch *t {
	case types.CrossClusterTaskTypeStartChildExecution:
		return adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_START_CHILD_EXECUTION
	case types.CrossClusterTaskTypeCancelExecution:
		return adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_CANCEL_EXECUTION
	case types.CrossClusterTaskTypeSignalExecution:
		return adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_SIGNAL_EXECUTION
	case types.CrossClusterTaskTypeRecordChildWorkflowExeuctionComplete:
		return adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_RECORD_CHILD_WORKKLOW_EXECUTION_COMPLETE
	case types.CrossClusterTaskTypeApplyParentPolicy:
		return adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_APPLY_PARENT_CLOSE_POLICY
	}
	return adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_INVALID
}

// ToCrossClusterTaskType converts proto CrossClusterTaskType type to internal
func ToCrossClusterTaskType(t adminv1.CrossClusterTaskType) *types.CrossClusterTaskType {
	switch t {
	case adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_INVALID:
		return nil
	case adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_START_CHILD_EXECUTION:
		return types.CrossClusterTaskTypeStartChildExecution.Ptr()
	case adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_CANCEL_EXECUTION:
		return types.CrossClusterTaskTypeCancelExecution.Ptr()
	case adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_SIGNAL_EXECUTION:
		return types.CrossClusterTaskTypeSignalExecution.Ptr()
	case adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_RECORD_CHILD_WORKKLOW_EXECUTION_COMPLETE:
		return types.CrossClusterTaskTypeRecordChildWorkflowExeuctionComplete.Ptr()
	case adminv1.CrossClusterTaskType_CROSS_CLUSTER_TASK_TYPE_APPLY_PARENT_CLOSE_POLICY:
		return types.CrossClusterTaskTypeApplyParentPolicy.Ptr()
	}
	return nil
}

// FromCrossClusterTaskFailedCause converts internal CrossClusterTaskFailedCause type to proto
func FromCrossClusterTaskFailedCause(t *types.CrossClusterTaskFailedCause) adminv1.CrossClusterTaskFailedCause {
	if t == nil {
		return adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_INVALID
	}
	switch *t {
	case types.CrossClusterTaskFailedCauseDomainNotActive:
		return adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_DOMAIN_NOT_ACTIVE
	case types.CrossClusterTaskFailedCauseDomainNotExists:
		return adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_DOMAIN_NOT_EXISTS
	case types.CrossClusterTaskFailedCauseWorkflowAlreadyRunning:
		return adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_WORKFLOW_ALREADY_RUNNING
	case types.CrossClusterTaskFailedCauseWorkflowNotExists:
		return adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_WORKFLOW_NOT_EXISTS
	case types.CrossClusterTaskFailedCauseWorkflowAlreadyCompleted:
		return adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_WORKFLOW_ALREADY_COMPLETED
	case types.CrossClusterTaskFailedCauseUncategorized:
		return adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_UNCATEGORIZED
	}
	return adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_INVALID
}

// ToCrossClusterTaskFailedCause converts proto CrossClusterTaskFailedCause type to internal
func ToCrossClusterTaskFailedCause(t adminv1.CrossClusterTaskFailedCause) *types.CrossClusterTaskFailedCause {
	switch t {
	case adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_INVALID:
		return nil
	case adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_DOMAIN_NOT_ACTIVE:
		return types.CrossClusterTaskFailedCauseDomainNotActive.Ptr()
	case adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_DOMAIN_NOT_EXISTS:
		return types.CrossClusterTaskFailedCauseDomainNotExists.Ptr()
	case adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_WORKFLOW_ALREADY_RUNNING:
		return types.CrossClusterTaskFailedCauseWorkflowAlreadyRunning.Ptr()
	case adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_WORKFLOW_NOT_EXISTS:
		return types.CrossClusterTaskFailedCauseWorkflowNotExists.Ptr()
	case adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_WORKFLOW_ALREADY_COMPLETED:
		return types.CrossClusterTaskFailedCauseWorkflowAlreadyCompleted.Ptr()
	case adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_UNCATEGORIZED:
		return types.CrossClusterTaskFailedCauseUncategorized.Ptr()

	}
	return nil
}

// FromGetTaskFailedCause converts internal GetTaskFailedCause type to proto
func FromGetTaskFailedCause(t *types.GetTaskFailedCause) adminv1.GetTaskFailedCause {
	if t == nil {
		return adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_INVALID
	}
	switch *t {
	case types.GetTaskFailedCauseServiceBusy:
		return adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_SERVICE_BUSY
	case types.GetTaskFailedCauseTimeout:
		return adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_TIMEOUT
	case types.GetTaskFailedCauseShardOwnershipLost:
		return adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_SHARD_OWNERSHIP_LOST
	case types.GetTaskFailedCauseUncategorized:
		return adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_UNCATEGORIZED
	}
	return adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_INVALID
}

// ToGetTaskFailedCause converts proto GetTaskFailedCause type to internal
func ToGetTaskFailedCause(t adminv1.GetTaskFailedCause) *types.GetTaskFailedCause {
	switch t {
	case adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_INVALID:
		return nil
	case adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_SERVICE_BUSY:
		return types.GetTaskFailedCauseServiceBusy.Ptr()
	case adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_TIMEOUT:
		return types.GetTaskFailedCauseTimeout.Ptr()
	case adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_SHARD_OWNERSHIP_LOST:
		return types.GetTaskFailedCauseShardOwnershipLost.Ptr()
	case adminv1.GetTaskFailedCause_GET_TASK_FAILED_CAUSE_UNCATEGORIZED:
		return types.GetTaskFailedCauseUncategorized.Ptr()
	}
	return nil
}

// FromCrossClusterTaskInfo converts internal CrossClusterTaskInfo type to proto
func FromCrossClusterTaskInfo(t *types.CrossClusterTaskInfo) *adminv1.CrossClusterTaskInfo {
	if t == nil {
		return nil
	}
	return &adminv1.CrossClusterTaskInfo{
		DomainId:            t.DomainID,
		WorkflowExecution:   FromWorkflowRunPair(t.WorkflowID, t.RunID),
		TaskType:            FromCrossClusterTaskType(t.TaskType),
		TaskState:           int32(t.TaskState),
		TaskId:              t.TaskID,
		VisibilityTimestamp: unixNanoToTime(t.VisibilityTimestamp),
	}
}

// ToCrossClusterTaskInfo converts proto CrossClusterTaskInfo type to internal
func ToCrossClusterTaskInfo(t *adminv1.CrossClusterTaskInfo) *types.CrossClusterTaskInfo {
	if t == nil {
		return nil
	}
	return &types.CrossClusterTaskInfo{
		DomainID:            t.DomainId,
		WorkflowID:          ToWorkflowID(t.WorkflowExecution),
		RunID:               ToRunID(t.WorkflowExecution),
		TaskType:            ToCrossClusterTaskType(t.TaskType),
		TaskState:           int16(t.TaskState),
		TaskID:              t.TaskId,
		VisibilityTimestamp: timeToUnixNano(t.VisibilityTimestamp),
	}
}

// FromCrossClusterStartChildExecutionRequestAttributes converts internal CrossClusterStartChildExecutionRequestAttributes type to proto
func FromCrossClusterStartChildExecutionRequestAttributes(t *types.CrossClusterStartChildExecutionRequestAttributes) *adminv1.CrossClusterStartChildExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.CrossClusterStartChildExecutionRequestAttributes{
		TargetDomainId:           t.TargetDomainID,
		RequestId:                t.RequestID,
		InitiatedEventId:         t.InitiatedEventID,
		InitiatedEventAttributes: FromStartChildWorkflowExecutionInitiatedEventAttributes(t.InitiatedEventAttributes),
		TargetRunId:              t.GetTargetRunID(),
		PartitionConfig:          t.PartitionConfig,
	}
}

// ToCrossClusterStartChildExecutionRequestAttributes converts proto CrossClusterStartChildExecutionRequestAttributes type to internal
func ToCrossClusterStartChildExecutionRequestAttributes(t *adminv1.CrossClusterStartChildExecutionRequestAttributes) *types.CrossClusterStartChildExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterStartChildExecutionRequestAttributes{
		TargetDomainID:           t.TargetDomainId,
		RequestID:                t.RequestId,
		InitiatedEventID:         t.InitiatedEventId,
		InitiatedEventAttributes: ToStartChildWorkflowExecutionInitiatedEventAttributes(t.InitiatedEventAttributes),
		TargetRunID:              &t.TargetRunId,
		PartitionConfig:          t.PartitionConfig,
	}
}

// FromCrossClusterStartChildExecutionResponseAttributes converts internal CrossClusterStartChildExecutionResponseAttributes type to proto
func FromCrossClusterStartChildExecutionResponseAttributes(t *types.CrossClusterStartChildExecutionResponseAttributes) *adminv1.CrossClusterStartChildExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.CrossClusterStartChildExecutionResponseAttributes{
		RunId: t.RunID,
	}
}

// ToCrossClusterStartChildExecutionResponseAttributes converts proto CrossClusterStartChildExecutionResponseAttributes type to internal
func ToCrossClusterStartChildExecutionResponseAttributes(t *adminv1.CrossClusterStartChildExecutionResponseAttributes) *types.CrossClusterStartChildExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterStartChildExecutionResponseAttributes{
		RunID: t.RunId,
	}
}

// FromCrossClusterCancelExecutionRequestAttributes converts internal CrossClusterCancelExecutionRequestAttributes type to proto
func FromCrossClusterCancelExecutionRequestAttributes(t *types.CrossClusterCancelExecutionRequestAttributes) *adminv1.CrossClusterCancelExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.CrossClusterCancelExecutionRequestAttributes{
		TargetDomainId:          t.TargetDomainID,
		TargetWorkflowExecution: FromWorkflowRunPair(t.TargetWorkflowID, t.TargetRunID),
		RequestId:               t.RequestID,
		InitiatedEventId:        t.InitiatedEventID,
		ChildWorkflowOnly:       t.ChildWorkflowOnly,
	}
}

// ToCrossClusterCancelExecutionRequestAttributes converts proto CrossClusterCancelExecutionRequestAttributes type to internal
func ToCrossClusterCancelExecutionRequestAttributes(t *adminv1.CrossClusterCancelExecutionRequestAttributes) *types.CrossClusterCancelExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterCancelExecutionRequestAttributes{
		TargetDomainID:    t.TargetDomainId,
		TargetWorkflowID:  ToWorkflowID(t.TargetWorkflowExecution),
		TargetRunID:       ToRunID(t.TargetWorkflowExecution),
		RequestID:         t.RequestId,
		InitiatedEventID:  t.InitiatedEventId,
		ChildWorkflowOnly: t.ChildWorkflowOnly,
	}
}

// FromCrossClusterCancelExecutionResponseAttributes converts internal CrossClusterCancelExecutionResponseAttributes type to proto
func FromCrossClusterCancelExecutionResponseAttributes(t *types.CrossClusterCancelExecutionResponseAttributes) *adminv1.CrossClusterCancelExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.CrossClusterCancelExecutionResponseAttributes{}
}

// ToCrossClusterCancelExecutionResponseAttributes converts proto CrossClusterCancelExecutionResponseAttributes type to internal
func ToCrossClusterCancelExecutionResponseAttributes(t *adminv1.CrossClusterCancelExecutionResponseAttributes) *types.CrossClusterCancelExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterCancelExecutionResponseAttributes{}
}

// FromCrossClusterSignalExecutionRequestAttributes converts internal CrossClusterSignalExecutionRequestAttributes type to proto
func FromCrossClusterSignalExecutionRequestAttributes(t *types.CrossClusterSignalExecutionRequestAttributes) *adminv1.CrossClusterSignalExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.CrossClusterSignalExecutionRequestAttributes{
		TargetDomainId:          t.TargetDomainID,
		TargetWorkflowExecution: FromWorkflowRunPair(t.TargetWorkflowID, t.TargetRunID),
		RequestId:               t.RequestID,
		InitiatedEventId:        t.InitiatedEventID,
		ChildWorkflowOnly:       t.ChildWorkflowOnly,
		SignalName:              t.SignalName,
		SignalInput:             FromPayload(t.SignalInput),
		Control:                 t.Control,
	}
}

// ToCrossClusterSignalExecutionRequestAttributes converts proto CrossClusterSignalExecutionRequestAttributes type to internal
func ToCrossClusterSignalExecutionRequestAttributes(t *adminv1.CrossClusterSignalExecutionRequestAttributes) *types.CrossClusterSignalExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterSignalExecutionRequestAttributes{
		TargetDomainID:    t.TargetDomainId,
		TargetWorkflowID:  ToWorkflowID(t.TargetWorkflowExecution),
		TargetRunID:       ToRunID(t.TargetWorkflowExecution),
		RequestID:         t.RequestId,
		InitiatedEventID:  t.InitiatedEventId,
		ChildWorkflowOnly: t.ChildWorkflowOnly,
		SignalName:        t.SignalName,
		SignalInput:       ToPayload(t.SignalInput),
		Control:           t.Control,
	}
}

// FromCrossClusterSignalExecutionResponseAttributes converts internal CrossClusterSignalExecutionResponseAttributes type to proto
func FromCrossClusterSignalExecutionResponseAttributes(t *types.CrossClusterSignalExecutionResponseAttributes) *adminv1.CrossClusterSignalExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.CrossClusterSignalExecutionResponseAttributes{}
}

// ToCrossClusterSignalExecutionResponseAttributes converts proto CrossClusterSignalExecutionResponseAttributes type to internal
func ToCrossClusterSignalExecutionResponseAttributes(t *adminv1.CrossClusterSignalExecutionResponseAttributes) *types.CrossClusterSignalExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterSignalExecutionResponseAttributes{}
}

// FromCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes converts internal CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes type to proto
func FromCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes(t *types.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes) *adminv1.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes{
		TargetDomainId:          t.TargetDomainID,
		TargetWorkflowExecution: FromWorkflowRunPair(t.TargetWorkflowID, t.TargetRunID),
		InitiatedEventId:        t.InitiatedEventID,
		CompletionEvent:         FromHistoryEvent(t.CompletionEvent),
	}
}

// ToCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes converts proto CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes type to internal
func ToCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes(t *adminv1.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes) *types.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes{
		TargetDomainID:   t.TargetDomainId,
		TargetWorkflowID: ToWorkflowID(t.TargetWorkflowExecution),
		TargetRunID:      ToRunID(t.TargetWorkflowExecution),
		InitiatedEventID: t.InitiatedEventId,
		CompletionEvent:  ToHistoryEvent(t.CompletionEvent),
	}
}

// FromCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes converts internal CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes type to proto
func FromCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes(t *types.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes) *adminv1.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes{}
}

// ToCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes converts proto CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes type to internal
func ToCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes(t *adminv1.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes) *types.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes{}
}

// FromApplyParentClosePolicyStatus converts internal ApplyParentClosePolicyStatus type to proto
func FromApplyParentClosePolicyStatus(t *types.ApplyParentClosePolicyStatus) *adminv1.ApplyParentClosePolicyStatus {
	if t == nil {
		return nil
	}
	return &adminv1.ApplyParentClosePolicyStatus{
		Completed:   t.Completed,
		FailedCause: FromCrossClusterTaskFailedCause(t.FailedCause),
	}
}

// ToApplyParentClosePolicyStatus converts proto ApplyParentClosePolicyStatus type to internal
func ToApplyParentClosePolicyStatus(t *adminv1.ApplyParentClosePolicyStatus) *types.ApplyParentClosePolicyStatus {
	if t == nil {
		return nil
	}
	return &types.ApplyParentClosePolicyStatus{
		Completed:   t.Completed,
		FailedCause: ToCrossClusterTaskFailedCause(t.FailedCause),
	}
}

// FromApplyParentClosePolicyAttributes converts internal ApplyParentClosePolicyAttributes type to proto
func FromApplyParentClosePolicyAttributes(t *types.ApplyParentClosePolicyAttributes) *adminv1.ApplyParentClosePolicyAttributes {
	if t == nil {
		return nil
	}
	return &adminv1.ApplyParentClosePolicyAttributes{
		ChildDomainId:     t.ChildDomainID,
		ChildWorkflowId:   t.ChildWorkflowID,
		ChildRunId:        t.ChildRunID,
		ParentClosePolicy: FromParentClosePolicy(t.ParentClosePolicy),
	}
}

// ToApplyParentClosePolicyAttributes converts proto ApplyParentClosePolicyAttributes type to internal
func ToApplyParentClosePolicyAttributes(t *adminv1.ApplyParentClosePolicyAttributes) *types.ApplyParentClosePolicyAttributes {
	if t == nil {
		return nil
	}
	return &types.ApplyParentClosePolicyAttributes{
		ChildDomainID:     t.ChildDomainId,
		ChildWorkflowID:   t.ChildWorkflowId,
		ChildRunID:        t.ChildRunId,
		ParentClosePolicy: ToParentClosePolicy(t.ParentClosePolicy),
	}
}

// FromApplyParentClosePolicyResult converts proto ApplyParentClosePolicyResult type to internal
func FromApplyParentClosePolicyResult(t *types.ApplyParentClosePolicyResult) *adminv1.ApplyParentClosePolicyResult {
	if t == nil {
		return nil
	}
	return &adminv1.ApplyParentClosePolicyResult{
		Child:       FromApplyParentClosePolicyAttributes(t.Child),
		FailedCause: FromCrossClusterTaskFailedCause(t.FailedCause),
	}
}

// ToApplyParentClosePolicyResult converts proto ApplyParentClosePolicyResult type to internal
func ToApplyParentClosePolicyResult(t *adminv1.ApplyParentClosePolicyResult) *types.ApplyParentClosePolicyResult {
	if t == nil {
		return nil
	}
	return &types.ApplyParentClosePolicyResult{
		Child:       ToApplyParentClosePolicyAttributes(t.Child),
		FailedCause: ToCrossClusterTaskFailedCause(t.FailedCause),
	}
}

// FromCrossClusterApplyParentClosePolicyRequestAttributes converts internal CrossClusterApplyParentClosePolicyRequestAttributes type to proto
func FromCrossClusterApplyParentClosePolicyRequestAttributes(t *types.CrossClusterApplyParentClosePolicyRequestAttributes) *adminv1.CrossClusterApplyParentClosePolicyRequestAttributes {
	if t == nil {
		return nil
	}
	requestAttributes := &adminv1.CrossClusterApplyParentClosePolicyRequestAttributes{}
	for _, execution := range t.Children {
		requestAttributes.Children = append(
			requestAttributes.Children,
			&adminv1.ApplyParentClosePolicyRequest{
				Child:  FromApplyParentClosePolicyAttributes(execution.Child),
				Status: FromApplyParentClosePolicyStatus(execution.Status),
			},
		)
	}
	return requestAttributes
}

// ToCrossClusterApplyParentClosePolicyRequestAttributes converts proto CrossClusterApplyParentClosePolicyRequestAttributes type to internal
func ToCrossClusterApplyParentClosePolicyRequestAttributes(t *adminv1.CrossClusterApplyParentClosePolicyRequestAttributes) *types.CrossClusterApplyParentClosePolicyRequestAttributes {
	if t == nil {
		return nil
	}
	requestAttributes := &types.CrossClusterApplyParentClosePolicyRequestAttributes{}
	for _, execution := range t.Children {
		requestAttributes.Children = append(
			requestAttributes.Children,
			&types.ApplyParentClosePolicyRequest{
				Child:  ToApplyParentClosePolicyAttributes(execution.Child),
				Status: ToApplyParentClosePolicyStatus(execution.Status),
			},
		)
	}
	return requestAttributes
}

// FromCrossClusterApplyParentClosePolicyResponseAttributes converts internal CrossClusterApplyParentClosePolicyResponseAttributes type to proto
func FromCrossClusterApplyParentClosePolicyResponseAttributes(t *types.CrossClusterApplyParentClosePolicyResponseAttributes) *adminv1.CrossClusterApplyParentClosePolicyResponseAttributes {
	if t == nil {
		return nil
	}
	response := &adminv1.CrossClusterApplyParentClosePolicyResponseAttributes{}
	for _, childStatus := range t.ChildrenStatus {
		response.ChildrenStatus = append(
			response.ChildrenStatus,
			FromApplyParentClosePolicyResult(childStatus),
		)
	}
	return response
}

// ToCrossClusterApplyParentClosePolicyResponseAttributes converts proto CrossClusterApplyParentClosePolicyResponseAttributes type to internal
func ToCrossClusterApplyParentClosePolicyResponseAttributes(t *adminv1.CrossClusterApplyParentClosePolicyResponseAttributes) *types.CrossClusterApplyParentClosePolicyResponseAttributes {
	if t == nil {
		return nil
	}
	response := &types.CrossClusterApplyParentClosePolicyResponseAttributes{}
	for _, childStatus := range t.ChildrenStatus {
		response.ChildrenStatus = append(
			response.ChildrenStatus,
			ToApplyParentClosePolicyResult(childStatus),
		)
	}
	return response
}

// FromCrossClusterTaskRequest converts internal CrossClusterTaskRequest type to proto
func FromCrossClusterTaskRequest(t *types.CrossClusterTaskRequest) *adminv1.CrossClusterTaskRequest {
	if t == nil {
		return nil
	}
	request := adminv1.CrossClusterTaskRequest{
		TaskInfo: FromCrossClusterTaskInfo(t.TaskInfo),
	}
	if t.StartChildExecutionAttributes != nil {
		request.Attributes = &adminv1.CrossClusterTaskRequest_StartChildExecutionAttributes{
			StartChildExecutionAttributes: FromCrossClusterStartChildExecutionRequestAttributes(t.StartChildExecutionAttributes),
		}
	}
	if t.CancelExecutionAttributes != nil {
		request.Attributes = &adminv1.CrossClusterTaskRequest_CancelExecutionAttributes{
			CancelExecutionAttributes: FromCrossClusterCancelExecutionRequestAttributes(t.CancelExecutionAttributes),
		}
	}
	if t.SignalExecutionAttributes != nil {
		request.Attributes = &adminv1.CrossClusterTaskRequest_SignalExecutionAttributes{
			SignalExecutionAttributes: FromCrossClusterSignalExecutionRequestAttributes(t.SignalExecutionAttributes),
		}
	}
	if t.RecordChildWorkflowExecutionCompleteAttributes != nil {
		request.Attributes = &adminv1.CrossClusterTaskRequest_RecordChildWorkflowExecutionCompleteRequestAttributes{
			RecordChildWorkflowExecutionCompleteRequestAttributes: FromCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes(t.RecordChildWorkflowExecutionCompleteAttributes),
		}
	}
	if t.ApplyParentClosePolicyAttributes != nil {
		request.Attributes = &adminv1.CrossClusterTaskRequest_ApplyParentClosePolicyRequestAttributes{
			ApplyParentClosePolicyRequestAttributes: FromCrossClusterApplyParentClosePolicyRequestAttributes(t.ApplyParentClosePolicyAttributes),
		}
	}
	return &request
}

// ToCrossClusterTaskRequest converts proto CrossClusterTaskRequest type to internal
func ToCrossClusterTaskRequest(t *adminv1.CrossClusterTaskRequest) *types.CrossClusterTaskRequest {
	if t == nil {
		return nil
	}
	request := types.CrossClusterTaskRequest{
		TaskInfo: ToCrossClusterTaskInfo(t.TaskInfo),
	}
	switch attr := t.Attributes.(type) {
	case *adminv1.CrossClusterTaskRequest_StartChildExecutionAttributes:
		request.StartChildExecutionAttributes = ToCrossClusterStartChildExecutionRequestAttributes(attr.StartChildExecutionAttributes)
	case *adminv1.CrossClusterTaskRequest_CancelExecutionAttributes:
		request.CancelExecutionAttributes = ToCrossClusterCancelExecutionRequestAttributes(attr.CancelExecutionAttributes)
	case *adminv1.CrossClusterTaskRequest_SignalExecutionAttributes:
		request.SignalExecutionAttributes = ToCrossClusterSignalExecutionRequestAttributes(attr.SignalExecutionAttributes)
	case *adminv1.CrossClusterTaskRequest_RecordChildWorkflowExecutionCompleteRequestAttributes:
		request.RecordChildWorkflowExecutionCompleteAttributes = ToCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes(attr.RecordChildWorkflowExecutionCompleteRequestAttributes)
	case *adminv1.CrossClusterTaskRequest_ApplyParentClosePolicyRequestAttributes:
		request.ApplyParentClosePolicyAttributes = ToCrossClusterApplyParentClosePolicyRequestAttributes(attr.ApplyParentClosePolicyRequestAttributes)
	}
	return &request
}

// FromCrossClusterTaskResponse converts internal CrossClusterTaskResponse type to proto
func FromCrossClusterTaskResponse(t *types.CrossClusterTaskResponse) *adminv1.CrossClusterTaskResponse {
	if t == nil {
		return nil
	}
	response := adminv1.CrossClusterTaskResponse{
		TaskId:      t.TaskID,
		TaskType:    FromCrossClusterTaskType(t.TaskType),
		TaskState:   int32(t.TaskState),
		FailedCause: FromCrossClusterTaskFailedCause(t.FailedCause),
	}
	if t.StartChildExecutionAttributes != nil {
		response.Attributes = &adminv1.CrossClusterTaskResponse_StartChildExecutionAttributes{
			StartChildExecutionAttributes: FromCrossClusterStartChildExecutionResponseAttributes(t.StartChildExecutionAttributes),
		}
	}
	if t.CancelExecutionAttributes != nil {
		response.Attributes = &adminv1.CrossClusterTaskResponse_CancelExecutionAttributes{
			CancelExecutionAttributes: FromCrossClusterCancelExecutionResponseAttributes(t.CancelExecutionAttributes),
		}
	}
	if t.SignalExecutionAttributes != nil {
		response.Attributes = &adminv1.CrossClusterTaskResponse_SignalExecutionAttributes{
			SignalExecutionAttributes: FromCrossClusterSignalExecutionResponseAttributes(t.SignalExecutionAttributes),
		}
	}
	if t.RecordChildWorkflowExecutionCompleteAttributes != nil {
		response.Attributes = &adminv1.CrossClusterTaskResponse_RecordChildWorkflowExecutionCompleteRequestAttributes{
			RecordChildWorkflowExecutionCompleteRequestAttributes: FromCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes(t.RecordChildWorkflowExecutionCompleteAttributes),
		}
	}
	if t.ApplyParentClosePolicyAttributes != nil {
		response.Attributes = &adminv1.CrossClusterTaskResponse_ApplyParentClosePolicyResponseAttributes{
			ApplyParentClosePolicyResponseAttributes: FromCrossClusterApplyParentClosePolicyResponseAttributes(t.ApplyParentClosePolicyAttributes),
		}
	}
	return &response
}

// ToCrossClusterTaskResponse converts proto CrossClusterTaskResponse type to internal
func ToCrossClusterTaskResponse(t *adminv1.CrossClusterTaskResponse) *types.CrossClusterTaskResponse {
	if t == nil {
		return nil
	}
	response := types.CrossClusterTaskResponse{
		TaskID:      t.TaskId,
		TaskType:    ToCrossClusterTaskType(t.TaskType),
		TaskState:   int16(t.TaskState),
		FailedCause: ToCrossClusterTaskFailedCause(t.FailedCause),
	}
	switch attr := t.Attributes.(type) {
	case *adminv1.CrossClusterTaskResponse_StartChildExecutionAttributes:
		response.StartChildExecutionAttributes = ToCrossClusterStartChildExecutionResponseAttributes(attr.StartChildExecutionAttributes)
	case *adminv1.CrossClusterTaskResponse_CancelExecutionAttributes:
		response.CancelExecutionAttributes = ToCrossClusterCancelExecutionResponseAttributes(attr.CancelExecutionAttributes)
	case *adminv1.CrossClusterTaskResponse_SignalExecutionAttributes:
		response.SignalExecutionAttributes = ToCrossClusterSignalExecutionResponseAttributes(attr.SignalExecutionAttributes)
	case *adminv1.CrossClusterTaskResponse_RecordChildWorkflowExecutionCompleteRequestAttributes:
		response.RecordChildWorkflowExecutionCompleteAttributes = ToCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes(attr.RecordChildWorkflowExecutionCompleteRequestAttributes)
	case *adminv1.CrossClusterTaskResponse_ApplyParentClosePolicyResponseAttributes:
		response.ApplyParentClosePolicyAttributes = ToCrossClusterApplyParentClosePolicyResponseAttributes(attr.ApplyParentClosePolicyResponseAttributes)
	}
	return &response
}

// FromCrossClusterTaskRequestArray converts internal CrossClusterTaskRequest type array to proto
func FromCrossClusterTaskRequestArray(t []*types.CrossClusterTaskRequest) *adminv1.CrossClusterTaskRequests {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.CrossClusterTaskRequest, len(t))
	for i := range t {
		v[i] = FromCrossClusterTaskRequest(t[i])
	}
	return &adminv1.CrossClusterTaskRequests{
		TaskRequests: v,
	}
}

// ToCrossClusterTaskRequestArray converts proto CrossClusterTaskRequest type array to internal
func ToCrossClusterTaskRequestArray(t *adminv1.CrossClusterTaskRequests) []*types.CrossClusterTaskRequest {
	if t == nil || t.TaskRequests == nil {
		return nil
	}
	v := make([]*types.CrossClusterTaskRequest, len(t.TaskRequests))
	for i := range t.TaskRequests {
		v[i] = ToCrossClusterTaskRequest(t.TaskRequests[i])
	}
	return v
}

// FromCrossClusterTaskRequestMap converts internal CrossClusterTaskRequest type map to proto
func FromCrossClusterTaskRequestMap(t map[int32][]*types.CrossClusterTaskRequest) map[int32]*adminv1.CrossClusterTaskRequests {
	if t == nil {
		return nil
	}
	v := make(map[int32]*adminv1.CrossClusterTaskRequests, len(t))
	for key := range t {
		v[key] = FromCrossClusterTaskRequestArray(t[key])
	}
	return v
}

// ToCrossClusterTaskRequestMap converts proto CrossClusterTaskRequest type map to internal
func ToCrossClusterTaskRequestMap(t map[int32]*adminv1.CrossClusterTaskRequests) map[int32][]*types.CrossClusterTaskRequest {
	if t == nil {
		return nil
	}
	v := make(map[int32][]*types.CrossClusterTaskRequest, len(t))
	for key := range t {
		value := ToCrossClusterTaskRequestArray(t[key])
		if value == nil {
			// grpc can't differentiate between empty array or nil array
			// our application logic ensure no nil array will be returned
			// for CrossClusterTaskRequest, so always convert to empty array
			// we only need the special handling here as this array is used
			// as a map value in GetCrossClusterTasksResponse,
			// and if the map value is nil, THRIFT won't be able to encode the value
			// this may happen when we are using grpc within a cluster but thrift across cluster
			value = []*types.CrossClusterTaskRequest{}
		}
		v[key] = value
	}
	return v
}

// FromGetTaskFailedCauseMap converts internal GetTaskFailedCause type map to proto
func FromGetTaskFailedCauseMap(t map[int32]types.GetTaskFailedCause) map[int32]adminv1.GetTaskFailedCause {
	if t == nil {
		return nil
	}
	v := make(map[int32]adminv1.GetTaskFailedCause, len(t))
	for key, value := range t {
		v[key] = FromGetTaskFailedCause(&value)
	}
	return v
}

// ToGetTaskFailedCauseMap converts proto GetTaskFailedCause type map to internal
func ToGetTaskFailedCauseMap(t map[int32]adminv1.GetTaskFailedCause) map[int32]types.GetTaskFailedCause {
	if t == nil {
		return nil
	}
	v := make(map[int32]types.GetTaskFailedCause, len(t))
	for key := range t {
		if internalValue := ToGetTaskFailedCause(t[key]); internalValue != nil {
			v[key] = *internalValue
		}
	}
	return v
}

// FromCrossClusterTaskResponseArray converts internal CrossClusterTaskResponse type array to proto
func FromCrossClusterTaskResponseArray(t []*types.CrossClusterTaskResponse) []*adminv1.CrossClusterTaskResponse {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.CrossClusterTaskResponse, len(t))
	for i := range t {
		v[i] = FromCrossClusterTaskResponse(t[i])
	}
	return v
}

// ToCrossClusterTaskResponseArray converts proto CrossClusterTaskResponse type array to internal
func ToCrossClusterTaskResponseArray(t []*adminv1.CrossClusterTaskResponse) []*types.CrossClusterTaskResponse {
	if t == nil {
		return nil
	}
	v := make([]*types.CrossClusterTaskResponse, len(t))
	for i := range t {
		v[i] = ToCrossClusterTaskResponse(t[i])
	}
	return v
}

func FromHistoryDLQCountEntryMap(t map[types.HistoryDLQCountKey]int64) []*adminv1.HistoryDLQCountEntry {
	if t == nil {
		return nil
	}
	entries := make([]*adminv1.HistoryDLQCountEntry, 0, len(t))
	for key, count := range t {
		entries = append(entries, &adminv1.HistoryDLQCountEntry{
			ShardId:       key.ShardID,
			SourceCluster: key.SourceCluster,
			Count:         count,
		})
	}
	return entries
}

func ToHistoryDLQCountEntryMap(t []*adminv1.HistoryDLQCountEntry) map[types.HistoryDLQCountKey]int64 {
	if t == nil {
		return nil
	}
	entries := make(map[types.HistoryDLQCountKey]int64, len(t))
	for _, entry := range t {
		key := types.HistoryDLQCountKey{ShardID: entry.ShardId, SourceCluster: entry.SourceCluster}
		entries[key] = entry.Count
	}
	return entries
}

// ToAny converts thrift Any type to internal
func ToAny(t *sharedv1.Any) *types.Any {
	if t == nil {
		return nil
	}
	return &types.Any{
		ValueType: t.GetValueType(),
		Value:     t.Value,
	}
}

// FromAny converts internal Any type to thrift
func FromAny(t *types.Any) *sharedv1.Any {
	if t == nil {
		return nil
	}
	return &sharedv1.Any{
		ValueType: t.ValueType,
		Value:     t.Value,
	}
}
