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

package types

import (
	"fmt"
	"strconv"
	"strings"
)

// DLQType is an internal type (TBD...)
type DLQType int32

// Ptr is a helper function for getting pointer value
func (e DLQType) Ptr() *DLQType {
	return &e
}

// String returns a readable string representation of DLQType.
func (e DLQType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "Replication"
	case 1:
		return "Domain"
	}
	return fmt.Sprintf("DLQType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *DLQType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "REPLICATION":
		*e = DLQTypeReplication
		return nil
	case "DOMAIN":
		*e = DLQTypeDomain
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "DLQType", err)
		}
		*e = DLQType(val)
		return nil
	}
}

// MarshalText encodes DLQType to text.
func (e DLQType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// DLQTypeReplication is an option for DLQType
	DLQTypeReplication DLQType = iota
	// DLQTypeDomain is an option for DLQType
	DLQTypeDomain
)

// DomainOperation is an internal type (TBD...)
type DomainOperation int32

// Ptr is a helper function for getting pointer value
func (e DomainOperation) Ptr() *DomainOperation {
	return &e
}

// String returns a readable string representation of DomainOperation.
func (e DomainOperation) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "Create"
	case 1:
		return "Update"
	}
	return fmt.Sprintf("DomainOperation(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *DomainOperation) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "CREATE":
		*e = DomainOperationCreate
		return nil
	case "UPDATE":
		*e = DomainOperationUpdate
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "DomainOperation", err)
		}
		*e = DomainOperation(val)
		return nil
	}
}

// MarshalText encodes DomainOperation to text.
func (e DomainOperation) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// DomainOperationCreate is an option for DomainOperation
	DomainOperationCreate DomainOperation = iota
	// DomainOperationUpdate is an option for DomainOperation
	DomainOperationUpdate
)

// DomainTaskAttributes is an internal type (TBD...)
type DomainTaskAttributes struct {
	DomainOperation         *DomainOperation                `json:"domainOperation,omitempty"`
	ID                      string                          `json:"id,omitempty"`
	Info                    *DomainInfo                     `json:"info,omitempty"`
	Config                  *DomainConfiguration            `json:"config,omitempty"`
	ReplicationConfig       *DomainReplicationConfiguration `json:"replicationConfig,omitempty"`
	ConfigVersion           int64                           `json:"configVersion,omitempty"`
	FailoverVersion         int64                           `json:"failoverVersion,omitempty"`
	PreviousFailoverVersion int64                           `json:"previousFailoverVersion,omitempty"`
}

// GetDomainOperation is an internal getter (TBD...)
func (v *DomainTaskAttributes) GetDomainOperation() (o DomainOperation) {
	if v != nil && v.DomainOperation != nil {
		return *v.DomainOperation
	}
	return
}

// GetID is an internal getter (TBD...)
func (v *DomainTaskAttributes) GetID() (o string) {
	if v != nil {
		return v.ID
	}
	return
}

// GetInfo is an internal getter (TBD...)
func (v *DomainTaskAttributes) GetInfo() (o *DomainInfo) {
	if v != nil && v.Info != nil {
		return v.Info
	}
	return
}

// GetConfigVersion is an internal getter (TBD...)
func (v *DomainTaskAttributes) GetConfigVersion() (o int64) {
	if v != nil {
		return v.ConfigVersion
	}
	return
}

// GetFailoverVersion is an internal getter (TBD...)
func (v *DomainTaskAttributes) GetFailoverVersion() (o int64) {
	if v != nil {
		return v.FailoverVersion
	}
	return
}

// GetPreviousFailoverVersion is an internal getter (TBD...)
func (v *DomainTaskAttributes) GetPreviousFailoverVersion() (o int64) {
	if v != nil {
		return v.PreviousFailoverVersion
	}
	return
}

// FailoverMarkerAttributes is an internal type (TBD...)
type FailoverMarkerAttributes struct {
	DomainID        string `json:"domainID,omitempty"`
	FailoverVersion int64  `json:"failoverVersion,omitempty"`
	CreationTime    *int64 `json:"creationTime,omitempty"`
}

// GetDomainID is an internal getter (TBD...)
func (v *FailoverMarkerAttributes) GetDomainID() (o string) {
	if v != nil {
		return v.DomainID
	}
	return
}

// GetFailoverVersion is an internal getter (TBD...)
func (v *FailoverMarkerAttributes) GetFailoverVersion() (o int64) {
	if v != nil {
		return v.FailoverVersion
	}
	return
}

// GetCreationTime is an internal getter (TBD...)
func (v *FailoverMarkerAttributes) GetCreationTime() (o int64) {
	if v != nil && v.CreationTime != nil {
		return *v.CreationTime
	}
	return
}

// FailoverMarkers is an internal type (TBD...)
type FailoverMarkers struct {
	FailoverMarkers []*FailoverMarkerAttributes `json:"failoverMarkers,omitempty"`
}

// GetDLQReplicationMessagesRequest is an internal type (TBD...)
type GetDLQReplicationMessagesRequest struct {
	TaskInfos []*ReplicationTaskInfo `json:"taskInfos,omitempty"`
}

// GetTaskInfos is an internal getter (TBD...)
func (v *GetDLQReplicationMessagesRequest) GetTaskInfos() (o []*ReplicationTaskInfo) {
	if v != nil && v.TaskInfos != nil {
		return v.TaskInfos
	}
	return
}

// GetDLQReplicationMessagesResponse is an internal type (TBD...)
type GetDLQReplicationMessagesResponse struct {
	ReplicationTasks []*ReplicationTask `json:"replicationTasks,omitempty"`
}

// GetDomainReplicationMessagesRequest is an internal type (TBD...)
type GetDomainReplicationMessagesRequest struct {
	LastRetrievedMessageID *int64 `json:"lastRetrievedMessageId,omitempty"`
	LastProcessedMessageID *int64 `json:"lastProcessedMessageId,omitempty"`
	ClusterName            string `json:"clusterName,omitempty"`
}

// GetLastRetrievedMessageID is an internal getter (TBD...)
func (v *GetDomainReplicationMessagesRequest) GetLastRetrievedMessageID() (o int64) {
	if v != nil && v.LastRetrievedMessageID != nil {
		return *v.LastRetrievedMessageID
	}
	return
}

// GetLastProcessedMessageID is an internal getter (TBD...)
func (v *GetDomainReplicationMessagesRequest) GetLastProcessedMessageID() (o int64) {
	if v != nil && v.LastProcessedMessageID != nil {
		return *v.LastProcessedMessageID
	}
	return
}

// GetClusterName is an internal getter (TBD...)
func (v *GetDomainReplicationMessagesRequest) GetClusterName() (o string) {
	if v != nil {
		return v.ClusterName
	}
	return
}

// GetDomainReplicationMessagesResponse is an internal type (TBD...)
type GetDomainReplicationMessagesResponse struct {
	Messages *ReplicationMessages `json:"messages,omitempty"`
}

// GetReplicationMessagesRequest is an internal type (TBD...)
type GetReplicationMessagesRequest struct {
	Tokens      []*ReplicationToken `json:"tokens,omitempty"`
	ClusterName string              `json:"clusterName,omitempty"`
}

// GetClusterName is an internal getter (TBD...)
func (v *GetReplicationMessagesRequest) GetClusterName() (o string) {
	if v != nil {
		return v.ClusterName
	}
	return
}

// GetReplicationMessagesResponse is an internal type (TBD...)
type GetReplicationMessagesResponse struct {
	MessagesByShard map[int32]*ReplicationMessages `json:"messagesByShard,omitempty"`
}

// GetMessagesByShard is an internal getter (TBD...)
func (v *GetReplicationMessagesResponse) GetMessagesByShard() (o map[int32]*ReplicationMessages) {
	if v != nil && v.MessagesByShard != nil {
		return v.MessagesByShard
	}
	return
}

// HistoryTaskV2Attributes is an internal type (TBD...)
type HistoryTaskV2Attributes struct {
	DomainID            string                `json:"domainId,omitempty"`
	WorkflowID          string                `json:"workflowId,omitempty"`
	RunID               string                `json:"runId,omitempty"`
	VersionHistoryItems []*VersionHistoryItem `json:"versionHistoryItems,omitempty"`
	Events              *DataBlob             `json:"events,omitempty"`
	NewRunEvents        *DataBlob             `json:"newRunEvents,omitempty"`
}

// GetDomainID is an internal getter (TBD...)
func (v *HistoryTaskV2Attributes) GetDomainID() (o string) {
	if v != nil {
		return v.DomainID
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *HistoryTaskV2Attributes) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *HistoryTaskV2Attributes) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// GetVersionHistoryItems is an internal getter (TBD...)
func (v *HistoryTaskV2Attributes) GetVersionHistoryItems() (o []*VersionHistoryItem) {
	if v != nil && v.VersionHistoryItems != nil {
		return v.VersionHistoryItems
	}
	return
}

// GetEvents is an internal getter (TBD...)
func (v *HistoryTaskV2Attributes) GetEvents() (o *DataBlob) {
	if v != nil && v.Events != nil {
		return v.Events
	}
	return
}

// GetNewRunEvents is an internal getter (TBD...)
func (v *HistoryTaskV2Attributes) GetNewRunEvents() (o *DataBlob) {
	if v != nil && v.NewRunEvents != nil {
		return v.NewRunEvents
	}
	return
}

type CountDLQMessagesRequest struct {
	// ForceFetch will force fetching current values from DB
	// instead of using cached values used for emitting metrics.
	ForceFetch bool
}

type CountDLQMessagesResponse struct {
	History map[HistoryDLQCountKey]int64
	Domain  int64
}

type HistoryDLQCountKey struct {
	ShardID       int32
	SourceCluster string
}

type HistoryCountDLQMessagesResponse struct {
	Entries map[HistoryDLQCountKey]int64
}

// MergeDLQMessagesRequest is an internal type (TBD...)
type MergeDLQMessagesRequest struct {
	Type                  *DLQType `json:"type,omitempty"`
	ShardID               int32    `json:"shardID,omitempty"`
	SourceCluster         string   `json:"sourceCluster,omitempty"`
	InclusiveEndMessageID *int64   `json:"inclusiveEndMessageID,omitempty"`
	MaximumPageSize       int32    `json:"maximumPageSize,omitempty"`
	NextPageToken         []byte   `json:"nextPageToken,omitempty"`
}

// GetType is an internal getter (TBD...)
func (v *MergeDLQMessagesRequest) GetType() (o DLQType) {
	if v != nil && v.Type != nil {
		return *v.Type
	}
	return
}

// GetShardID is an internal getter (TBD...)
func (v *MergeDLQMessagesRequest) GetShardID() (o int32) {
	if v != nil {
		return v.ShardID
	}
	return
}

// GetSourceCluster is an internal getter (TBD...)
func (v *MergeDLQMessagesRequest) GetSourceCluster() (o string) {
	if v != nil {
		return v.SourceCluster
	}
	return
}

// GetInclusiveEndMessageID is an internal getter (TBD...)
func (v *MergeDLQMessagesRequest) GetInclusiveEndMessageID() (o int64) {
	if v != nil && v.InclusiveEndMessageID != nil {
		return *v.InclusiveEndMessageID
	}
	return
}

// GetMaximumPageSize is an internal getter (TBD...)
func (v *MergeDLQMessagesRequest) GetMaximumPageSize() (o int32) {
	if v != nil {
		return v.MaximumPageSize
	}
	return
}

// GetNextPageToken is an internal getter (TBD...)
func (v *MergeDLQMessagesRequest) GetNextPageToken() (o []byte) {
	if v != nil && v.NextPageToken != nil {
		return v.NextPageToken
	}
	return
}

// MergeDLQMessagesResponse is an internal type (TBD...)
type MergeDLQMessagesResponse struct {
	NextPageToken []byte `json:"nextPageToken,omitempty"`
}

// PurgeDLQMessagesRequest is an internal type (TBD...)
type PurgeDLQMessagesRequest struct {
	Type                  *DLQType `json:"type,omitempty"`
	ShardID               int32    `json:"shardID,omitempty"`
	SourceCluster         string   `json:"sourceCluster,omitempty"`
	InclusiveEndMessageID *int64   `json:"inclusiveEndMessageID,omitempty"`
}

// GetType is an internal getter (TBD...)
func (v *PurgeDLQMessagesRequest) GetType() (o DLQType) {
	if v != nil && v.Type != nil {
		return *v.Type
	}
	return
}

// GetShardID is an internal getter (TBD...)
func (v *PurgeDLQMessagesRequest) GetShardID() (o int32) {
	if v != nil {
		return v.ShardID
	}
	return
}

// GetSourceCluster is an internal getter (TBD...)
func (v *PurgeDLQMessagesRequest) GetSourceCluster() (o string) {
	if v != nil {
		return v.SourceCluster
	}
	return
}

// GetInclusiveEndMessageID is an internal getter (TBD...)
func (v *PurgeDLQMessagesRequest) GetInclusiveEndMessageID() (o int64) {
	if v != nil && v.InclusiveEndMessageID != nil {
		return *v.InclusiveEndMessageID
	}
	return
}

// ReadDLQMessagesRequest is an internal type (TBD...)
type ReadDLQMessagesRequest struct {
	Type                  *DLQType `json:"type,omitempty"`
	ShardID               int32    `json:"shardID,omitempty"`
	SourceCluster         string   `json:"sourceCluster,omitempty"`
	InclusiveEndMessageID *int64   `json:"inclusiveEndMessageID,omitempty"`
	MaximumPageSize       int32    `json:"maximumPageSize,omitempty"`
	NextPageToken         []byte   `json:"nextPageToken,omitempty"`
}

// GetType is an internal getter (TBD...)
func (v *ReadDLQMessagesRequest) GetType() (o DLQType) {
	if v != nil && v.Type != nil {
		return *v.Type
	}
	return
}

// GetShardID is an internal getter (TBD...)
func (v *ReadDLQMessagesRequest) GetShardID() (o int32) {
	if v != nil {
		return v.ShardID
	}
	return
}

// GetSourceCluster is an internal getter (TBD...)
func (v *ReadDLQMessagesRequest) GetSourceCluster() (o string) {
	if v != nil {
		return v.SourceCluster
	}
	return
}

// GetInclusiveEndMessageID is an internal getter (TBD...)
func (v *ReadDLQMessagesRequest) GetInclusiveEndMessageID() (o int64) {
	if v != nil && v.InclusiveEndMessageID != nil {
		return *v.InclusiveEndMessageID
	}
	return
}

// GetMaximumPageSize is an internal getter (TBD...)
func (v *ReadDLQMessagesRequest) GetMaximumPageSize() (o int32) {
	if v != nil {
		return v.MaximumPageSize
	}
	return
}

// GetNextPageToken is an internal getter (TBD...)
func (v *ReadDLQMessagesRequest) GetNextPageToken() (o []byte) {
	if v != nil && v.NextPageToken != nil {
		return v.NextPageToken
	}
	return
}

// ReadDLQMessagesResponse is an internal type (TBD...)
type ReadDLQMessagesResponse struct {
	Type                 *DLQType               `json:"type,omitempty"`
	ReplicationTasks     []*ReplicationTask     `json:"replicationTasks,omitempty"`
	ReplicationTasksInfo []*ReplicationTaskInfo `json:"replicationTasksInfo,omitempty"`
	NextPageToken        []byte                 `json:"nextPageToken,omitempty"`
}

// ReplicationMessages is an internal type (TBD...)
type ReplicationMessages struct {
	ReplicationTasks       []*ReplicationTask `json:"replicationTasks,omitempty"`
	LastRetrievedMessageID int64              `json:"lastRetrievedMessageId,omitempty"`
	HasMore                bool               `json:"hasMore,omitempty"`
	SyncShardStatus        *SyncShardStatus   `json:"syncShardStatus,omitempty"`
}

// GetReplicationTasks is an internal getter (TBD...)
func (v *ReplicationMessages) GetReplicationTasks() (o []*ReplicationTask) {
	if v != nil && v.ReplicationTasks != nil {
		return v.ReplicationTasks
	}
	return
}

// GetLastRetrievedMessageID is an internal getter (TBD...)
func (v *ReplicationMessages) GetLastRetrievedMessageID() (o int64) {
	if v != nil {
		return v.LastRetrievedMessageID
	}
	return
}

// GetHasMore is an internal getter (TBD...)
func (v *ReplicationMessages) GetHasMore() (o bool) {
	if v != nil {
		return v.HasMore
	}
	return
}

// GetSyncShardStatus is an internal getter (TBD...)
func (v *ReplicationMessages) GetSyncShardStatus() (o *SyncShardStatus) {
	if v != nil && v.SyncShardStatus != nil {
		return v.SyncShardStatus
	}
	return
}

// ReplicationTask is an internal type (TBD...)
type ReplicationTask struct {
	TaskType                      *ReplicationTaskType           `json:"taskType,omitempty"`
	SourceTaskID                  int64                          `json:"sourceTaskId,omitempty"`
	DomainTaskAttributes          *DomainTaskAttributes          `json:"domainTaskAttributes,omitempty"`
	SyncShardStatusTaskAttributes *SyncShardStatusTaskAttributes `json:"syncShardStatusTaskAttributes,omitempty"`
	SyncActivityTaskAttributes    *SyncActivityTaskAttributes    `json:"syncActivityTaskAttributes,omitempty"`
	HistoryTaskV2Attributes       *HistoryTaskV2Attributes       `json:"historyTaskV2Attributes,omitempty"`
	FailoverMarkerAttributes      *FailoverMarkerAttributes      `json:"failoverMarkerAttributes,omitempty"`
	CreationTime                  *int64                         `json:"creationTime,omitempty"`
}

// GetTaskType is an internal getter (TBD...)
func (v *ReplicationTask) GetTaskType() (o ReplicationTaskType) {
	if v != nil && v.TaskType != nil {
		return *v.TaskType
	}
	return
}

// GetSourceTaskID is an internal getter (TBD...)
func (v *ReplicationTask) GetSourceTaskID() (o int64) {
	if v != nil {
		return v.SourceTaskID
	}
	return
}

// GetDomainTaskAttributes is an internal getter (TBD...)
func (v *ReplicationTask) GetDomainTaskAttributes() (o *DomainTaskAttributes) {
	if v != nil && v.DomainTaskAttributes != nil {
		return v.DomainTaskAttributes
	}
	return
}

// GetSyncActivityTaskAttributes is an internal getter (TBD...)
func (v *ReplicationTask) GetSyncActivityTaskAttributes() (o *SyncActivityTaskAttributes) {
	if v != nil && v.SyncActivityTaskAttributes != nil {
		return v.SyncActivityTaskAttributes
	}
	return
}

// GetHistoryTaskV2Attributes is an internal getter (TBD...)
func (v *ReplicationTask) GetHistoryTaskV2Attributes() (o *HistoryTaskV2Attributes) {
	if v != nil && v.HistoryTaskV2Attributes != nil {
		return v.HistoryTaskV2Attributes
	}
	return
}

// GetFailoverMarkerAttributes is an internal getter (TBD...)
func (v *ReplicationTask) GetFailoverMarkerAttributes() (o *FailoverMarkerAttributes) {
	if v != nil && v.FailoverMarkerAttributes != nil {
		return v.FailoverMarkerAttributes
	}
	return
}

// GetCreationTime is an internal getter (TBD...)
func (v *ReplicationTask) GetCreationTime() (o int64) {
	if v != nil && v.CreationTime != nil {
		return *v.CreationTime
	}
	return
}

// ReplicationTaskInfo is an internal type (TBD...)
type ReplicationTaskInfo struct {
	DomainID     string `json:"domainID,omitempty"`
	WorkflowID   string `json:"workflowID,omitempty"`
	RunID        string `json:"runID,omitempty"`
	TaskType     int16  `json:"taskType,omitempty"`
	TaskID       int64  `json:"taskID,omitempty"`
	Version      int64  `json:"version,omitempty"`
	FirstEventID int64  `json:"firstEventID,omitempty"`
	NextEventID  int64  `json:"nextEventID,omitempty"`
	ScheduledID  int64  `json:"scheduledID,omitempty"`
}

// GetDomainID is an internal getter (TBD...)
func (v *ReplicationTaskInfo) GetDomainID() (o string) {
	if v != nil {
		return v.DomainID
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *ReplicationTaskInfo) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *ReplicationTaskInfo) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// GetTaskType is an internal getter (TBD...)
func (v *ReplicationTaskInfo) GetTaskType() (o int16) {
	if v != nil {
		return v.TaskType
	}
	return
}

// GetTaskID is an internal getter (TBD...)
func (v *ReplicationTaskInfo) GetTaskID() (o int64) {
	if v != nil {
		return v.TaskID
	}
	return
}

// GetVersion is an internal getter (TBD...)
func (v *ReplicationTaskInfo) GetVersion() (o int64) {
	if v != nil {
		return v.Version
	}
	return
}

// GetFirstEventID is an internal getter (TBD...)
func (v *ReplicationTaskInfo) GetFirstEventID() (o int64) {
	if v != nil {
		return v.FirstEventID
	}
	return
}

// GetNextEventID is an internal getter (TBD...)
func (v *ReplicationTaskInfo) GetNextEventID() (o int64) {
	if v != nil {
		return v.NextEventID
	}
	return
}

// GetScheduledID is an internal getter (TBD...)
func (v *ReplicationTaskInfo) GetScheduledID() (o int64) {
	if v != nil {
		return v.ScheduledID
	}
	return
}

// ReplicationTaskType is an internal type (TBD...)
type ReplicationTaskType int32

// Ptr is a helper function for getting pointer value
func (e ReplicationTaskType) Ptr() *ReplicationTaskType {
	return &e
}

// String returns a readable string representation of ReplicationTaskType.
func (e ReplicationTaskType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "Domain"
	case 1:
		return "History"
	case 2:
		return "SyncShardStatus"
	case 3:
		return "SyncActivity"
	case 4:
		return "HistoryMetadata"
	case 5:
		return "HistoryV2"
	case 6:
		return "FailoverMarker"
	}
	return fmt.Sprintf("ReplicationTaskType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *ReplicationTaskType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "DOMAIN":
		*e = ReplicationTaskTypeDomain
		return nil
	case "HISTORY":
		*e = ReplicationTaskTypeHistory
		return nil
	case "SYNCSHARDSTATUS":
		*e = ReplicationTaskTypeSyncShardStatus
		return nil
	case "SYNCACTIVITY":
		*e = ReplicationTaskTypeSyncActivity
		return nil
	case "HISTORYMETADATA":
		*e = ReplicationTaskTypeHistoryMetadata
		return nil
	case "HISTORYV2":
		*e = ReplicationTaskTypeHistoryV2
		return nil
	case "FAILOVERMARKER":
		*e = ReplicationTaskTypeFailoverMarker
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "ReplicationTaskType", err)
		}
		*e = ReplicationTaskType(val)
		return nil
	}
}

// MarshalText encodes ReplicationTaskType to text.
func (e ReplicationTaskType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// ReplicationTaskTypeDomain is an option for ReplicationTaskType
	ReplicationTaskTypeDomain ReplicationTaskType = iota
	// ReplicationTaskTypeHistory is an option for ReplicationTaskType
	ReplicationTaskTypeHistory
	// ReplicationTaskTypeSyncShardStatus is an option for ReplicationTaskType
	ReplicationTaskTypeSyncShardStatus
	// ReplicationTaskTypeSyncActivity is an option for ReplicationTaskType
	ReplicationTaskTypeSyncActivity
	// ReplicationTaskTypeHistoryMetadata is an option for ReplicationTaskType
	ReplicationTaskTypeHistoryMetadata
	// ReplicationTaskTypeHistoryV2 is an option for ReplicationTaskType
	ReplicationTaskTypeHistoryV2
	// ReplicationTaskTypeFailoverMarker is an option for ReplicationTaskType
	ReplicationTaskTypeFailoverMarker
)

// ReplicationToken is an internal type (TBD...)
type ReplicationToken struct {
	ShardID                int32 `json:"shardID,omitempty"`
	LastRetrievedMessageID int64 `json:"lastRetrievedMessageId,omitempty"`
	LastProcessedMessageID int64 `json:"lastProcessedMessageId,omitempty"`
}

// GetShardID is an internal getter (TBD...)
func (v *ReplicationToken) GetShardID() (o int32) {
	if v != nil {
		return v.ShardID
	}
	return
}

// GetLastRetrievedMessageID is an internal getter (TBD...)
func (v *ReplicationToken) GetLastRetrievedMessageID() (o int64) {
	if v != nil {
		return v.LastRetrievedMessageID
	}
	return
}

// GetLastProcessedMessageID is an internal getter (TBD...)
func (v *ReplicationToken) GetLastProcessedMessageID() (o int64) {
	if v != nil {
		return v.LastProcessedMessageID
	}
	return
}

// SyncActivityTaskAttributes is an internal type (TBD...)
type SyncActivityTaskAttributes struct {
	DomainID           string          `json:"domainId,omitempty"`
	WorkflowID         string          `json:"workflowId,omitempty"`
	RunID              string          `json:"runId,omitempty"`
	Version            int64           `json:"version,omitempty"`
	ScheduledID        int64           `json:"scheduledId,omitempty"`
	ScheduledTime      *int64          `json:"scheduledTime,omitempty"`
	StartedID          int64           `json:"startedId,omitempty"`
	StartedTime        *int64          `json:"startedTime,omitempty"`
	LastHeartbeatTime  *int64          `json:"lastHeartbeatTime,omitempty"`
	Details            []byte          `json:"details,omitempty"`
	Attempt            int32           `json:"attempt,omitempty"`
	LastFailureReason  *string         `json:"lastFailureReason,omitempty"`
	LastWorkerIdentity string          `json:"lastWorkerIdentity,omitempty"`
	LastFailureDetails []byte          `json:"lastFailureDetails,omitempty"`
	VersionHistory     *VersionHistory `json:"versionHistory,omitempty"`
}

// GetDomainID is an internal getter (TBD...)
func (v *SyncActivityTaskAttributes) GetDomainID() (o string) {
	if v != nil {
		return v.DomainID
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *SyncActivityTaskAttributes) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *SyncActivityTaskAttributes) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// GetVersion is an internal getter (TBD...)
func (v *SyncActivityTaskAttributes) GetVersion() (o int64) {
	if v != nil {
		return v.Version
	}
	return
}

// GetScheduledID is an internal getter (TBD...)
func (v *SyncActivityTaskAttributes) GetScheduledID() (o int64) {
	if v != nil {
		return v.ScheduledID
	}
	return
}

// GetScheduledTime is an internal getter (TBD...)
func (v *SyncActivityTaskAttributes) GetScheduledTime() (o int64) {
	if v != nil && v.ScheduledTime != nil {
		return *v.ScheduledTime
	}
	return
}

// GetStartedID is an internal getter (TBD...)
func (v *SyncActivityTaskAttributes) GetStartedID() (o int64) {
	if v != nil {
		return v.StartedID
	}
	return
}

// GetStartedTime is an internal getter (TBD...)
func (v *SyncActivityTaskAttributes) GetStartedTime() (o int64) {
	if v != nil && v.StartedTime != nil {
		return *v.StartedTime
	}
	return
}

// GetLastHeartbeatTime is an internal getter (TBD...)
func (v *SyncActivityTaskAttributes) GetLastHeartbeatTime() (o int64) {
	if v != nil && v.LastHeartbeatTime != nil {
		return *v.LastHeartbeatTime
	}
	return
}

// GetVersionHistory is an internal getter (TBD...)
func (v *SyncActivityTaskAttributes) GetVersionHistory() (o *VersionHistory) {
	if v != nil && v.VersionHistory != nil {
		return v.VersionHistory
	}
	return
}

// SyncShardStatus is an internal type (TBD...)
type SyncShardStatus struct {
	Timestamp *int64 `json:"timestamp,omitempty"`
}

// GetTimestamp is an internal getter (TBD...)
func (v *SyncShardStatus) GetTimestamp() (o int64) {
	if v != nil && v.Timestamp != nil {
		return *v.Timestamp
	}
	return
}

// SyncShardStatusTaskAttributes is an internal type (TBD...)
type SyncShardStatusTaskAttributes struct {
	SourceCluster string `json:"sourceCluster,omitempty"`
	ShardID       int64  `json:"shardId,omitempty"`
	Timestamp     *int64 `json:"timestamp,omitempty"`
}
