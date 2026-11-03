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

package testdata

import (
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

const (
	TaskType        = int16(1)
	MaxiumuPageSize = int32(5)
)

var (
	ReplicationMessages = types.ReplicationMessages{
		ReplicationTasks:       ReplicationTaskArray,
		LastRetrievedMessageID: MessageID1,
		HasMore:                true,
		SyncShardStatus:        &SyncShardStatus,
	}
	ReplicationMessagesMap = map[int32]*types.ReplicationMessages{
		ShardID: &ReplicationMessages,
	}
	SyncShardStatus = types.SyncShardStatus{
		Timestamp: &Timestamp1,
	}
	ReplicationToken = types.ReplicationToken{
		ShardID:                ShardID,
		LastRetrievedMessageID: MessageID1,
		LastProcessedMessageID: MessageID2,
	}
	ReplicationTokenArray = []*types.ReplicationToken{
		&ReplicationToken,
	}
	ReplicationTaskInfo = types.ReplicationTaskInfo{
		DomainID:     DomainID,
		WorkflowID:   WorkflowID,
		RunID:        RunID,
		TaskType:     TaskType,
		TaskID:       TaskID,
		Version:      Version1,
		FirstEventID: EventID1,
		NextEventID:  EventID2,
		ScheduledID:  EventID3,
	}
	ReplicationTaskInfoArray = []*types.ReplicationTaskInfo{
		&ReplicationTaskInfo,
	}
	ReplicationTask_Domain = types.ReplicationTask{
		TaskType:             types.ReplicationTaskTypeDomain.Ptr(),
		SourceTaskID:         TaskID,
		DomainTaskAttributes: &DomainTaskAttributes,
		CreationTime:         &Timestamp1,
	}
	ReplicationTask_SyncShard = types.ReplicationTask{
		TaskType:                      types.ReplicationTaskTypeSyncShardStatus.Ptr(),
		SourceTaskID:                  TaskID,
		SyncShardStatusTaskAttributes: &SyncShardStatusTaskAttributes,
		CreationTime:                  &Timestamp1,
	}
	ReplicationTask_SyncActivity = types.ReplicationTask{
		TaskType:                   types.ReplicationTaskTypeSyncActivity.Ptr(),
		SourceTaskID:               TaskID,
		SyncActivityTaskAttributes: &SyncActivityTaskAttributes,
		CreationTime:               &Timestamp1,
	}
	ReplicationTask_History = types.ReplicationTask{
		TaskType:                types.ReplicationTaskTypeHistoryV2.Ptr(),
		SourceTaskID:            TaskID,
		HistoryTaskV2Attributes: &HistoryTaskV2Attributes,
		CreationTime:            &Timestamp1,
	}
	ReplicationTask_Failover = types.ReplicationTask{
		TaskType:                 types.ReplicationTaskTypeFailoverMarker.Ptr(),
		SourceTaskID:             TaskID,
		FailoverMarkerAttributes: &FailoverMarkerAttributes,
		CreationTime:             &Timestamp1,
	}
	ReplicationTask_SemaphoreMetadata = types.ReplicationTask{
		TaskType:                        types.ReplicationTaskTypeSemaphoreMetadata.Ptr(),
		SourceTaskID:                    TaskID,
		SemaphoreMetadataTaskAttributes: &SemaphoreMetadataTaskAttributes,
		CreationTime:                    &Timestamp1,
	}
	ReplicationTaskArray = []*types.ReplicationTask{
		&ReplicationTask_Domain,
		&ReplicationTask_SyncShard,
		&ReplicationTask_SyncActivity,
		&ReplicationTask_History,
		&ReplicationTask_Failover,
		&ReplicationTask_SemaphoreMetadata,
	}
	DomainTaskAttributes = types.DomainTaskAttributes{
		DomainOperation:         types.DomainOperationUpdate.Ptr(),
		ID:                      DomainID,
		Info:                    &DomainInfo,
		Config:                  &DomainConfiguration,
		ReplicationConfig:       &DomainReplicationConfiguration,
		ConfigVersion:           Version1,
		FailoverVersion:         FailoverVersion1,
		PreviousFailoverVersion: FailoverVersion2,
	}
	SyncShardStatusTaskAttributes = types.SyncShardStatusTaskAttributes{
		SourceCluster: ClusterName1,
		ShardID:       ShardID,
		Timestamp:     &Timestamp1,
	}
	SyncActivityTaskAttributes = types.SyncActivityTaskAttributes{
		DomainID:           DomainID,
		WorkflowID:         WorkflowID,
		RunID:              RunID,
		Version:            Version1,
		ScheduledID:        EventID1,
		ScheduledTime:      &Timestamp1,
		StartedID:          EventID2,
		StartedTime:        &Timestamp2,
		LastHeartbeatTime:  &Timestamp3,
		Details:            Payload1,
		Attempt:            Attempt,
		LastFailureReason:  &FailureReason,
		LastWorkerIdentity: Identity,
		LastFailureDetails: FailureDetails,
		VersionHistory:     &VersionHistory,
	}
	HistoryTaskV2Attributes = types.HistoryTaskV2Attributes{
		DomainID:            DomainID,
		WorkflowID:          WorkflowID,
		RunID:               RunID,
		VersionHistoryItems: VersionHistoryItemArray,
		Events:              &DataBlob,
		NewRunEvents:        &DataBlob,
	}
	SemaphoreMetadataTaskAttributes = types.SemaphoreMetadataTaskAttributes{
		DomainID:       DomainID,
		SemaphoreName:  SemaphoreName,
		Capacity:       10,
		BucketCapacity: 5,
	}
	FailoverMarkerAttributes = types.FailoverMarkerAttributes{
		DomainID:        DomainID,
		FailoverVersion: FailoverVersion1,
		CreationTime:    &Timestamp1,
	}
	FailoverMarkerAttributesArray = []*types.FailoverMarkerAttributes{
		&FailoverMarkerAttributes,
	}
	PersistenceFeatures = []*types.PersistenceFeature{
		{
			Key:     "PersistenceFeature",
			Enabled: true,
		},
		nil,
	}
	PersistenceSettings = []*types.PersistenceSetting{
		{
			Key:   "PersistenceKey",
			Value: "PersistenceValue",
		},
		nil,
	}
	PersistenceInfoMap = map[string]*types.PersistenceInfo{
		"Backend": {
			Backend:  "Backend",
			Settings: PersistenceSettings,
			Features: PersistenceFeatures,
		},
		"Invalid": nil,
	}
	GetDomainReplicationMessagesRequest = types.GetDomainReplicationMessagesRequest{
		LastRetrievedMessageID: common.Int64Ptr(MessageID1),
		LastProcessedMessageID: common.Int64Ptr(MessageID2),
		ClusterName:            ClusterName1,
	}
	MergeDLQMessagesRequest = types.MergeDLQMessagesRequest{
		Type:                  types.DLQTypeDomain.Ptr(),
		ShardID:               ShardID,
		SourceCluster:         ClusterName1,
		InclusiveEndMessageID: common.Int64Ptr(MessageID1),
		MaximumPageSize:       MaxiumuPageSize,
		NextPageToken:         Token1,
	}
	PurgeDLQMessagesRequest = types.PurgeDLQMessagesRequest{
		Type:                  types.DLQTypeDomain.Ptr(),
		ShardID:               ShardID,
		SourceCluster:         ClusterName1,
		InclusiveEndMessageID: common.Int64Ptr(MessageID1),
	}
	ReadDLQMessagesRequest = types.ReadDLQMessagesRequest{
		Type:                  types.DLQTypeDomain.Ptr(),
		ShardID:               ShardID,
		SourceCluster:         ClusterName1,
		InclusiveEndMessageID: common.Int64Ptr(MessageID1),
		MaximumPageSize:       MaxiumuPageSize,
		NextPageToken:         Token1,
	}
	ReadDLQMessagesResponse = types.ReadDLQMessagesResponse{
		Type:                 types.DLQTypeDomain.Ptr(),
		ReplicationTasks:     ReplicationTaskArray,
		ReplicationTasksInfo: ReplicationTaskInfoArray,
		NextPageToken:        Token1,
	}
)
