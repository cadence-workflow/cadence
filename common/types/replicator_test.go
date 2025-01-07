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

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDLQType_Ptr(t *testing.T) {
	dlqType := DLQTypeReplication
	ptr := dlqType.Ptr()

	assert.Equal(t, &dlqType, ptr)
}

func TestDLQType_String(t *testing.T) {
	dlqType := DLQTypeReplication
	assert.Equal(t, "Replication", dlqType.String())

	dlqType = DLQTypeDomain
	assert.Equal(t, "Domain", dlqType.String())

	dlqType = 2
	assert.Equal(t, "DLQType(2)", dlqType.String())
}

func TestDLQType_UnmarshalText(t *testing.T) {
	var dlqType DLQType
	err := dlqType.UnmarshalText([]byte("Replication"))
	assert.NoError(t, err)
	assert.Equal(t, DLQTypeReplication, dlqType)

	err = dlqType.UnmarshalText([]byte("Domain"))
	assert.NoError(t, err)
	assert.Equal(t, DLQTypeDomain, dlqType)

	err = dlqType.UnmarshalText([]byte("2"))
	assert.NoError(t, err)
	assert.Equal(t, DLQType(2), dlqType)

	err = dlqType.UnmarshalText([]byte("Invalid"))
	assert.Error(t, err)
}

func TestDLQType_MarshalText(t *testing.T) {
	dlqType := DLQTypeReplication
	text, err := dlqType.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("Replication"), text)

	dlqType = DLQTypeDomain
	text, err = dlqType.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("Domain"), text)
}

func TestDomainOperation_Ptr(t *testing.T) {
	domainOp := DomainOperationCreate
	ptr := domainOp.Ptr()

	assert.Equal(t, &domainOp, ptr)
}

func TestDomainOperation_String(t *testing.T) {
	domainOp := DomainOperationCreate
	assert.Equal(t, "Create", domainOp.String())

	domainOp = DomainOperationUpdate
	assert.Equal(t, "Update", domainOp.String())

	domainOp = 2
	assert.Equal(t, "DomainOperation(2)", domainOp.String())
}

func TestDomainOperation_UnmarshalText(t *testing.T) {
	var domainOp DomainOperation
	err := domainOp.UnmarshalText([]byte("Create"))
	assert.NoError(t, err)
	assert.Equal(t, DomainOperationCreate, domainOp)

	err = domainOp.UnmarshalText([]byte("Update"))
	assert.NoError(t, err)
	assert.Equal(t, DomainOperationUpdate, domainOp)

	err = domainOp.UnmarshalText([]byte("2"))
	assert.NoError(t, err)
	assert.Equal(t, DomainOperation(2), domainOp)

	err = domainOp.UnmarshalText([]byte("Invalid"))
	assert.Error(t, err)
}

func TestDomainOperation_MarshalText(t *testing.T) {
	domainOp := DomainOperationCreate
	text, err := domainOp.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("Create"), text)

	domainOp = DomainOperationUpdate
	text, err = domainOp.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("Update"), text)
}

func TestDomainTaskAttributes_GetDomainOperation(t *testing.T) {
	domainOp := DomainOperationCreate
	testStruct := DomainTaskAttributes{
		DomainOperation: &domainOp,
	}

	res := testStruct.GetDomainOperation()
	assert.Equal(t, domainOp, res)

	var nilStruct *DomainTaskAttributes
	res = nilStruct.GetDomainOperation()
	assert.Equal(t, DomainOperation(0), res)
}

func TestDomainTaskAttributes_GetID(t *testing.T) {
	testStruct := DomainTaskAttributes{
		ID: "test-id",
	}

	res := testStruct.GetID()
	assert.Equal(t, "test-id", res)

	var nilStruct *DomainTaskAttributes
	res = nilStruct.GetID()
	assert.Equal(t, "", res)
}

func TestDomainTaskAttributes_GetInfo(t *testing.T) {
	domainInfo := &DomainInfo{}
	testStruct := DomainTaskAttributes{
		Info: domainInfo,
	}

	res := testStruct.GetInfo()
	assert.Equal(t, domainInfo, res)

	var nilStruct *DomainTaskAttributes
	res = nilStruct.GetInfo()
	assert.Nil(t, res)
}

func TestDomainTaskAttributes_GetConfigVersion(t *testing.T) {
	testStruct := DomainTaskAttributes{
		ConfigVersion: 123,
	}

	res := testStruct.GetConfigVersion()
	assert.Equal(t, int64(123), res)

	var nilStruct *DomainTaskAttributes
	res = nilStruct.GetConfigVersion()
	assert.Equal(t, int64(0), res)
}

func TestDomainTaskAttributes_GetFailoverVersion(t *testing.T) {
	testStruct := DomainTaskAttributes{
		FailoverVersion: 456,
	}

	res := testStruct.GetFailoverVersion()
	assert.Equal(t, int64(456), res)

	var nilStruct *DomainTaskAttributes
	res = nilStruct.GetFailoverVersion()
	assert.Equal(t, int64(0), res)
}

func TestDomainTaskAttributes_GetPreviousFailoverVersion(t *testing.T) {
	testStruct := DomainTaskAttributes{
		PreviousFailoverVersion: 789,
	}

	res := testStruct.GetPreviousFailoverVersion()
	assert.Equal(t, int64(789), res)

	var nilStruct *DomainTaskAttributes
	res = nilStruct.GetPreviousFailoverVersion()
	assert.Equal(t, int64(0), res)
}

func TestFailoverMarkerAttributes_GetDomainID(t *testing.T) {
	testStruct := FailoverMarkerAttributes{
		DomainID: "domain-id",
	}

	res := testStruct.GetDomainID()
	assert.Equal(t, "domain-id", res)

	var nilStruct *FailoverMarkerAttributes
	res = nilStruct.GetDomainID()
	assert.Equal(t, "", res)
}

func TestFailoverMarkerAttributes_GetFailoverVersion(t *testing.T) {
	testStruct := FailoverMarkerAttributes{
		FailoverVersion: 1234,
	}

	res := testStruct.GetFailoverVersion()
	assert.Equal(t, int64(1234), res)

	var nilStruct *FailoverMarkerAttributes
	res = nilStruct.GetFailoverVersion()
	assert.Equal(t, int64(0), res)
}

func TestFailoverMarkerAttributes_GetCreationTime(t *testing.T) {
	creationTime := int64(5678)
	testStruct := FailoverMarkerAttributes{
		CreationTime: &creationTime,
	}

	res := testStruct.GetCreationTime()
	assert.Equal(t, creationTime, res)

	var nilStruct *FailoverMarkerAttributes
	res = nilStruct.GetCreationTime()
	assert.Equal(t, int64(0), res)
}

func TestMergeDLQMessagesRequest_GetType(t *testing.T) {
	dlqType := DLQTypeReplication
	testStruct := MergeDLQMessagesRequest{
		Type: &dlqType,
	}

	res := testStruct.GetType()
	assert.Equal(t, dlqType, res)

	var nilStruct *MergeDLQMessagesRequest
	res = nilStruct.GetType()
	assert.Equal(t, DLQType(0), res)
}

func TestMergeDLQMessagesRequest_GetShardID(t *testing.T) {
	testStruct := MergeDLQMessagesRequest{
		ShardID: 101,
	}

	res := testStruct.GetShardID()
	assert.Equal(t, int32(101), res)

	var nilStruct *MergeDLQMessagesRequest
	res = nilStruct.GetShardID()
	assert.Equal(t, int32(0), res)
}

func TestMergeDLQMessagesRequest_GetSourceCluster(t *testing.T) {
	testStruct := MergeDLQMessagesRequest{
		SourceCluster: "cluster-1",
	}

	res := testStruct.GetSourceCluster()
	assert.Equal(t, "cluster-1", res)

	var nilStruct *MergeDLQMessagesRequest
	res = nilStruct.GetSourceCluster()
	assert.Equal(t, "", res)
}

func TestMergeDLQMessagesRequest_GetInclusiveEndMessageID(t *testing.T) {
	endMessageID := int64(102)
	testStruct := MergeDLQMessagesRequest{
		InclusiveEndMessageID: &endMessageID,
	}

	res := testStruct.GetInclusiveEndMessageID()
	assert.Equal(t, endMessageID, res)

	var nilStruct *MergeDLQMessagesRequest
	res = nilStruct.GetInclusiveEndMessageID()
	assert.Equal(t, int64(0), res)
}

func TestGetDLQReplicationMessagesRequest_GetTaskInfos(t *testing.T) {
	taskInfos := []*ReplicationTaskInfo{{}, {}}
	testStruct := GetDLQReplicationMessagesRequest{
		TaskInfos: taskInfos,
	}

	res := testStruct.GetTaskInfos()
	assert.Equal(t, taskInfos, res)

	var nilStruct *GetDLQReplicationMessagesRequest
	res = nilStruct.GetTaskInfos()
	assert.Nil(t, res)
}

func TestGetDomainReplicationMessagesRequest_GetLastRetrievedMessageID(t *testing.T) {
	lastRetrievedMessageID := int64(12345)
	testStruct := GetDomainReplicationMessagesRequest{
		LastRetrievedMessageID: &lastRetrievedMessageID,
	}

	res := testStruct.GetLastRetrievedMessageID()
	assert.Equal(t, lastRetrievedMessageID, res)

	var nilStruct *GetDomainReplicationMessagesRequest
	res = nilStruct.GetLastRetrievedMessageID()
	assert.Equal(t, int64(0), res)
}

func TestGetDomainReplicationMessagesRequest_GetLastProcessedMessageID(t *testing.T) {
	lastProcessedMessageID := int64(67890)
	testStruct := GetDomainReplicationMessagesRequest{
		LastProcessedMessageID: &lastProcessedMessageID,
	}

	res := testStruct.GetLastProcessedMessageID()
	assert.Equal(t, lastProcessedMessageID, res)

	var nilStruct *GetDomainReplicationMessagesRequest
	res = nilStruct.GetLastProcessedMessageID()
	assert.Equal(t, int64(0), res)
}

func TestGetDomainReplicationMessagesRequest_GetClusterName(t *testing.T) {
	testStruct := GetDomainReplicationMessagesRequest{
		ClusterName: "test-cluster",
	}

	res := testStruct.GetClusterName()
	assert.Equal(t, "test-cluster", res)

	var nilStruct *GetDomainReplicationMessagesRequest
	res = nilStruct.GetClusterName()
	assert.Equal(t, "", res)
}

func TestGetReplicationMessagesRequest_GetClusterName(t *testing.T) {
	testStruct := GetReplicationMessagesRequest{
		ClusterName: "test-cluster",
	}

	res := testStruct.GetClusterName()
	assert.Equal(t, "test-cluster", res)

	var nilStruct *GetReplicationMessagesRequest
	res = nilStruct.GetClusterName()
	assert.Equal(t, "", res)
}

func TestGetReplicationMessagesResponse_GetMessagesByShard(t *testing.T) {
	messagesByShard := map[int32]*ReplicationMessages{
		1: {},
		2: {},
	}
	testStruct := GetReplicationMessagesResponse{
		MessagesByShard: messagesByShard,
	}

	res := testStruct.GetMessagesByShard()
	assert.Equal(t, messagesByShard, res)

	var nilStruct *GetReplicationMessagesResponse
	res = nilStruct.GetMessagesByShard()
	assert.Nil(t, res)
}

func TestCountDLQMessagesResponse(t *testing.T) {
	history := map[HistoryDLQCountKey]int64{
		{ShardID: 1, SourceCluster: "cluster-1"}: 100,
	}
	testStruct := CountDLQMessagesResponse{
		History: history,
		Domain:  200,
	}

	assert.Equal(t, history, testStruct.History)
	assert.Equal(t, int64(200), testStruct.Domain)

	// Test for empty history
	emptyStruct := CountDLQMessagesResponse{}
	assert.Nil(t, emptyStruct.History)
	assert.Equal(t, int64(0), emptyStruct.Domain)
}

func TestHistoryCountDLQMessagesResponse(t *testing.T) {
	entries := map[HistoryDLQCountKey]int64{
		{ShardID: 1, SourceCluster: "cluster-1"}: 100,
	}
	testStruct := HistoryCountDLQMessagesResponse{
		Entries: entries,
	}

	assert.Equal(t, entries, testStruct.Entries)

	// Test for empty entries
	emptyStruct := HistoryCountDLQMessagesResponse{}
	assert.Nil(t, emptyStruct.Entries)
}

func TestMergeDLQMessagesRequest_Getters(t *testing.T) {
	dlqType := DLQTypeReplication
	endMessageID := int64(102)
	nextPageToken := []byte("token")
	testStruct := MergeDLQMessagesRequest{
		Type:                  &dlqType,
		ShardID:               101,
		SourceCluster:         "cluster-1",
		InclusiveEndMessageID: &endMessageID,
		MaximumPageSize:       50,
		NextPageToken:         nextPageToken,
	}

	assert.Equal(t, dlqType, testStruct.GetType())
	assert.Equal(t, int32(101), testStruct.GetShardID())
	assert.Equal(t, "cluster-1", testStruct.GetSourceCluster())
	assert.Equal(t, endMessageID, testStruct.GetInclusiveEndMessageID())
	assert.Equal(t, int32(50), testStruct.GetMaximumPageSize())
	assert.Equal(t, nextPageToken, testStruct.GetNextPageToken())

	// Test for nil values
	var nilStruct *MergeDLQMessagesRequest
	assert.Equal(t, DLQType(0), nilStruct.GetType())
	assert.Equal(t, int32(0), nilStruct.GetShardID())
	assert.Equal(t, "", nilStruct.GetSourceCluster())
	assert.Equal(t, int64(0), nilStruct.GetInclusiveEndMessageID())
	assert.Equal(t, int32(0), nilStruct.GetMaximumPageSize())
	assert.Nil(t, nilStruct.GetNextPageToken())
}

func TestHistoryTaskV2Attributes_GetDomainID(t *testing.T) {
	testStruct := HistoryTaskV2Attributes{
		DomainID: "domain-id",
	}

	res := testStruct.GetDomainID()
	assert.Equal(t, "domain-id", res)

	var nilStruct *HistoryTaskV2Attributes
	res = nilStruct.GetDomainID()
	assert.Equal(t, "", res)
}

func TestHistoryTaskV2Attributes_GetWorkflowID(t *testing.T) {
	testStruct := HistoryTaskV2Attributes{
		WorkflowID: "workflow-id",
	}

	res := testStruct.GetWorkflowID()
	assert.Equal(t, "workflow-id", res)

	var nilStruct *HistoryTaskV2Attributes
	res = nilStruct.GetWorkflowID()
	assert.Equal(t, "", res)
}

func TestHistoryTaskV2Attributes_GetRunID(t *testing.T) {
	testStruct := HistoryTaskV2Attributes{
		RunID: "run-id",
	}

	res := testStruct.GetRunID()
	assert.Equal(t, "run-id", res)

	var nilStruct *HistoryTaskV2Attributes
	res = nilStruct.GetRunID()
	assert.Equal(t, "", res)
}

func TestHistoryTaskV2Attributes_GetVersionHistoryItems(t *testing.T) {
	versionHistoryItems := []*VersionHistoryItem{{}, {}}
	testStruct := HistoryTaskV2Attributes{
		VersionHistoryItems: versionHistoryItems,
	}

	res := testStruct.GetVersionHistoryItems()
	assert.Equal(t, versionHistoryItems, res)

	var nilStruct *HistoryTaskV2Attributes
	res = nilStruct.GetVersionHistoryItems()
	assert.Nil(t, res)
}

func TestHistoryTaskV2Attributes_GetEvents(t *testing.T) {
	events := &DataBlob{}
	testStruct := HistoryTaskV2Attributes{
		Events: events,
	}

	res := testStruct.GetEvents()
	assert.Equal(t, events, res)

	var nilStruct *HistoryTaskV2Attributes
	res = nilStruct.GetEvents()
	assert.Nil(t, res)
}

func TestHistoryTaskV2Attributes_GetNewRunEvents(t *testing.T) {
	newRunEvents := &DataBlob{}
	testStruct := HistoryTaskV2Attributes{
		NewRunEvents: newRunEvents,
	}

	res := testStruct.GetNewRunEvents()
	assert.Equal(t, newRunEvents, res)

	var nilStruct *HistoryTaskV2Attributes
	res = nilStruct.GetNewRunEvents()
	assert.Nil(t, res)
}

func TestPurgeDLQMessagesRequest_Getters(t *testing.T) {
	dlqType := DLQTypeReplication
	endMessageID := int64(12345)
	testStruct := PurgeDLQMessagesRequest{
		Type:                  &dlqType,
		ShardID:               101,
		SourceCluster:         "test-cluster",
		InclusiveEndMessageID: &endMessageID,
	}

	assert.Equal(t, dlqType, testStruct.GetType())
	assert.Equal(t, int32(101), testStruct.GetShardID())
	assert.Equal(t, "test-cluster", testStruct.GetSourceCluster())
	assert.Equal(t, endMessageID, testStruct.GetInclusiveEndMessageID())

	// Test nil case
	var nilStruct *PurgeDLQMessagesRequest
	assert.Equal(t, DLQType(0), nilStruct.GetType())
	assert.Equal(t, int32(0), nilStruct.GetShardID())
	assert.Equal(t, "", nilStruct.GetSourceCluster())
	assert.Equal(t, int64(0), nilStruct.GetInclusiveEndMessageID())
}

func TestReadDLQMessagesRequest_Getters(t *testing.T) {
	dlqType := DLQTypeReplication
	endMessageID := int64(12345)
	nextPageToken := []byte("token")
	testStruct := ReadDLQMessagesRequest{
		Type:                  &dlqType,
		ShardID:               101,
		SourceCluster:         "test-cluster",
		InclusiveEndMessageID: &endMessageID,
		MaximumPageSize:       50,
		NextPageToken:         nextPageToken,
	}

	assert.Equal(t, dlqType, testStruct.GetType())
	assert.Equal(t, int32(101), testStruct.GetShardID())
	assert.Equal(t, "test-cluster", testStruct.GetSourceCluster())
	assert.Equal(t, endMessageID, testStruct.GetInclusiveEndMessageID())
	assert.Equal(t, int32(50), testStruct.GetMaximumPageSize())
	assert.Equal(t, nextPageToken, testStruct.GetNextPageToken())

	// Test nil case
	var nilStruct *ReadDLQMessagesRequest
	assert.Equal(t, DLQType(0), nilStruct.GetType())
	assert.Equal(t, int32(0), nilStruct.GetShardID())
	assert.Equal(t, "", nilStruct.GetSourceCluster())
	assert.Equal(t, int64(0), nilStruct.GetInclusiveEndMessageID())
	assert.Equal(t, int32(0), nilStruct.GetMaximumPageSize())
	assert.Nil(t, nilStruct.GetNextPageToken())
}

func TestReplicationMessages_GetReplicationTasks(t *testing.T) {
	tasks := []*ReplicationTask{{}, {}}
	testStruct := ReplicationMessages{
		ReplicationTasks: tasks,
	}

	res := testStruct.GetReplicationTasks()
	assert.Equal(t, tasks, res)

	var nilStruct *ReplicationMessages
	res = nilStruct.GetReplicationTasks()
	assert.Nil(t, res)
}

func TestReplicationMessages_GetLastRetrievedMessageID(t *testing.T) {
	testStruct := ReplicationMessages{
		LastRetrievedMessageID: 12345,
	}

	res := testStruct.GetLastRetrievedMessageID()
	assert.Equal(t, int64(12345), res)

	var nilStruct *ReplicationMessages
	res = nilStruct.GetLastRetrievedMessageID()
	assert.Equal(t, int64(0), res)
}

func TestReplicationMessages_GetHasMore(t *testing.T) {
	testStruct := ReplicationMessages{
		HasMore: true,
	}

	res := testStruct.GetHasMore()
	assert.True(t, res)

	var nilStruct *ReplicationMessages
	res = nilStruct.GetHasMore()
	assert.False(t, res)
}

func TestReplicationMessages_GetSyncShardStatus(t *testing.T) {
	status := &SyncShardStatus{}
	testStruct := ReplicationMessages{
		SyncShardStatus: status,
	}

	res := testStruct.GetSyncShardStatus()
	assert.Equal(t, status, res)

	var nilStruct *ReplicationMessages
	res = nilStruct.GetSyncShardStatus()
	assert.Nil(t, res)
}

func TestReplicationTask_GetTaskType(t *testing.T) {
	taskType := ReplicationTaskTypeDomain
	testStruct := ReplicationTask{
		TaskType: &taskType,
	}

	res := testStruct.GetTaskType()
	assert.Equal(t, taskType, res)

	var nilStruct *ReplicationTask
	res = nilStruct.GetTaskType()
	assert.Equal(t, ReplicationTaskType(0), res)
}

func TestReplicationTask_GetSourceTaskID(t *testing.T) {
	testStruct := ReplicationTask{
		SourceTaskID: 12345,
	}

	res := testStruct.GetSourceTaskID()
	assert.Equal(t, int64(12345), res)

	var nilStruct *ReplicationTask
	res = nilStruct.GetSourceTaskID()
	assert.Equal(t, int64(0), res)
}

func TestReplicationTask_GetDomainTaskAttributes(t *testing.T) {
	domainTask := &DomainTaskAttributes{}
	testStruct := ReplicationTask{
		DomainTaskAttributes: domainTask,
	}

	res := testStruct.GetDomainTaskAttributes()
	assert.Equal(t, domainTask, res)

	var nilStruct *ReplicationTask
	res = nilStruct.GetDomainTaskAttributes()
	assert.Nil(t, res)
}

func TestReplicationTask_GetSyncActivityTaskAttributes(t *testing.T) {
	syncActivityTask := &SyncActivityTaskAttributes{}
	testStruct := ReplicationTask{
		SyncActivityTaskAttributes: syncActivityTask,
	}

	res := testStruct.GetSyncActivityTaskAttributes()
	assert.Equal(t, syncActivityTask, res)

	var nilStruct *ReplicationTask
	res = nilStruct.GetSyncActivityTaskAttributes()
	assert.Nil(t, res)
}

func TestReplicationTask_GetHistoryTaskV2Attributes(t *testing.T) {
	historyTaskV2 := &HistoryTaskV2Attributes{}
	testStruct := ReplicationTask{
		HistoryTaskV2Attributes: historyTaskV2,
	}

	res := testStruct.GetHistoryTaskV2Attributes()
	assert.Equal(t, historyTaskV2, res)

	var nilStruct *ReplicationTask
	res = nilStruct.GetHistoryTaskV2Attributes()
	assert.Nil(t, res)
}

func TestReplicationTask_GetFailoverMarkerAttributes(t *testing.T) {
	failoverMarker := &FailoverMarkerAttributes{}
	testStruct := ReplicationTask{
		FailoverMarkerAttributes: failoverMarker,
	}

	res := testStruct.GetFailoverMarkerAttributes()
	assert.Equal(t, failoverMarker, res)

	var nilStruct *ReplicationTask
	res = nilStruct.GetFailoverMarkerAttributes()
	assert.Nil(t, res)
}

func TestReplicationTask_GetCreationTime(t *testing.T) {
	creationTime := int64(123456)
	testStruct := ReplicationTask{
		CreationTime: &creationTime,
	}

	res := testStruct.GetCreationTime()
	assert.Equal(t, creationTime, res)

	var nilStruct *ReplicationTask
	res = nilStruct.GetCreationTime()
	assert.Equal(t, int64(0), res)
}

func TestReplicationTaskInfo_GetDomainID(t *testing.T) {
	testStruct := ReplicationTaskInfo{
		DomainID: "domain-id",
	}

	res := testStruct.GetDomainID()
	assert.Equal(t, "domain-id", res)

	var nilStruct *ReplicationTaskInfo
	res = nilStruct.GetDomainID()
	assert.Equal(t, "", res)
}

func TestReplicationTaskInfo_GetWorkflowID(t *testing.T) {
	testStruct := ReplicationTaskInfo{
		WorkflowID: "workflow-id",
	}

	res := testStruct.GetWorkflowID()
	assert.Equal(t, "workflow-id", res)

	var nilStruct *ReplicationTaskInfo
	res = nilStruct.GetWorkflowID()
	assert.Equal(t, "", res)
}

func TestReplicationTaskInfo_GetRunID(t *testing.T) {
	testStruct := ReplicationTaskInfo{
		RunID: "run-id",
	}

	res := testStruct.GetRunID()
	assert.Equal(t, "run-id", res)

	var nilStruct *ReplicationTaskInfo
	res = nilStruct.GetRunID()
	assert.Equal(t, "", res)
}

func TestReplicationTaskInfo_GetTaskType(t *testing.T) {
	testStruct := ReplicationTaskInfo{
		TaskType: 1,
	}

	res := testStruct.GetTaskType()
	assert.Equal(t, int16(1), res)

	var nilStruct *ReplicationTaskInfo
	res = nilStruct.GetTaskType()
	assert.Equal(t, int16(0), res)
}

func TestReplicationTaskInfo_GetTaskID(t *testing.T) {
	testStruct := ReplicationTaskInfo{
		TaskID: 12345,
	}

	res := testStruct.GetTaskID()
	assert.Equal(t, int64(12345), res)

	var nilStruct *ReplicationTaskInfo
	res = nilStruct.GetTaskID()
	assert.Equal(t, int64(0), res)
}

func TestReplicationTaskInfo_GetVersion(t *testing.T) {
	testStruct := ReplicationTaskInfo{
		Version: 100,
	}

	res := testStruct.GetVersion()
	assert.Equal(t, int64(100), res)

	var nilStruct *ReplicationTaskInfo
	res = nilStruct.GetVersion()
	assert.Equal(t, int64(0), res)
}

func TestReplicationTaskInfo_GetFirstEventID(t *testing.T) {
	testStruct := ReplicationTaskInfo{
		FirstEventID: 11111,
	}

	res := testStruct.GetFirstEventID()
	assert.Equal(t, int64(11111), res)

	var nilStruct *ReplicationTaskInfo
	res = nilStruct.GetFirstEventID()
	assert.Equal(t, int64(0), res)
}

func TestReplicationTaskInfo_GetNextEventID(t *testing.T) {
	testStruct := ReplicationTaskInfo{
		NextEventID: 22222,
	}

	res := testStruct.GetNextEventID()
	assert.Equal(t, int64(22222), res)

	var nilStruct *ReplicationTaskInfo
	res = nilStruct.GetNextEventID()
	assert.Equal(t, int64(0), res)
}

func TestReplicationTaskInfo_GetScheduledID(t *testing.T) {
	testStruct := ReplicationTaskInfo{
		ScheduledID: 33333,
	}

	res := testStruct.GetScheduledID()
	assert.Equal(t, int64(33333), res)

	var nilStruct *ReplicationTaskInfo
	res = nilStruct.GetScheduledID()
	assert.Equal(t, int64(0), res)
}

func TestReplicationToken_GetShardID(t *testing.T) {
	testStruct := ReplicationToken{
		ShardID: 1,
	}

	res := testStruct.GetShardID()
	assert.Equal(t, int32(1), res)

	var nilStruct *ReplicationToken
	res = nilStruct.GetShardID()
	assert.Equal(t, int32(0), res)
}

func TestReplicationToken_GetLastRetrievedMessageID(t *testing.T) {
	testStruct := ReplicationToken{
		LastRetrievedMessageID: 123456,
	}

	res := testStruct.GetLastRetrievedMessageID()
	assert.Equal(t, int64(123456), res)

	var nilStruct *ReplicationToken
	res = nilStruct.GetLastRetrievedMessageID()
	assert.Equal(t, int64(0), res)
}

func TestReplicationToken_GetLastProcessedMessageID(t *testing.T) {
	testStruct := ReplicationToken{
		LastProcessedMessageID: 654321,
	}

	res := testStruct.GetLastProcessedMessageID()
	assert.Equal(t, int64(654321), res)

	var nilStruct *ReplicationToken
	res = nilStruct.GetLastProcessedMessageID()
	assert.Equal(t, int64(0), res)
}

func TestReplicationTaskType_Ptr(t *testing.T) {
	taskType := ReplicationTaskTypeDomain
	ptr := taskType.Ptr()

	assert.Equal(t, &taskType, ptr)
}

func TestReplicationTaskType_String(t *testing.T) {
	var taskType ReplicationTaskType

	taskType = ReplicationTaskTypeDomain
	assert.Equal(t, "Domain", taskType.String())

	taskType = ReplicationTaskTypeHistory
	assert.Equal(t, "History", taskType.String())

	taskType = ReplicationTaskTypeSyncShardStatus
	assert.Equal(t, "SyncShardStatus", taskType.String())

	taskType = ReplicationTaskTypeSyncActivity
	assert.Equal(t, "SyncActivity", taskType.String())

	taskType = ReplicationTaskTypeHistoryMetadata
	assert.Equal(t, "HistoryMetadata", taskType.String())

	taskType = ReplicationTaskTypeHistoryV2
	assert.Equal(t, "HistoryV2", taskType.String())

	taskType = ReplicationTaskTypeFailoverMarker
	assert.Equal(t, "FailoverMarker", taskType.String())

	// Test unknown task type
	taskType = ReplicationTaskType(10)
	assert.Equal(t, "ReplicationTaskType(10)", taskType.String())
}

func TestReplicationTaskType_UnmarshalText(t *testing.T) {
	var taskType ReplicationTaskType

	err := taskType.UnmarshalText([]byte("DOMAIN"))
	assert.NoError(t, err)
	assert.Equal(t, ReplicationTaskTypeDomain, taskType)

	err = taskType.UnmarshalText([]byte("HISTORY"))
	assert.NoError(t, err)
	assert.Equal(t, ReplicationTaskTypeHistory, taskType)

	err = taskType.UnmarshalText([]byte("SYNCSHARDSTATUS"))
	assert.NoError(t, err)
	assert.Equal(t, ReplicationTaskTypeSyncShardStatus, taskType)

	err = taskType.UnmarshalText([]byte("SYNCACTIVITY"))
	assert.NoError(t, err)
	assert.Equal(t, ReplicationTaskTypeSyncActivity, taskType)

	err = taskType.UnmarshalText([]byte("HISTORYMETADATA"))
	assert.NoError(t, err)
	assert.Equal(t, ReplicationTaskTypeHistoryMetadata, taskType)

	err = taskType.UnmarshalText([]byte("HISTORYV2"))
	assert.NoError(t, err)
	assert.Equal(t, ReplicationTaskTypeHistoryV2, taskType)

	err = taskType.UnmarshalText([]byte("FAILOVERMARKER"))
	assert.NoError(t, err)
	assert.Equal(t, ReplicationTaskTypeFailoverMarker, taskType)

	// Test unknown string value
	err = taskType.UnmarshalText([]byte("UNKNOWN"))
	assert.Error(t, err)

	// Test numeric value parsing
	err = taskType.UnmarshalText([]byte("10"))
	assert.NoError(t, err)
	assert.Equal(t, ReplicationTaskType(10), taskType)
}

func TestReplicationTaskType_MarshalText(t *testing.T) {
	var taskType ReplicationTaskType

	taskType = ReplicationTaskTypeDomain
	text, err := taskType.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("Domain"), text)

	taskType = ReplicationTaskTypeHistory
	text, err = taskType.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("History"), text)

	taskType = ReplicationTaskTypeSyncShardStatus
	text, err = taskType.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("SyncShardStatus"), text)

	taskType = ReplicationTaskTypeSyncActivity
	text, err = taskType.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("SyncActivity"), text)

	taskType = ReplicationTaskTypeHistoryMetadata
	text, err = taskType.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("HistoryMetadata"), text)

	taskType = ReplicationTaskTypeHistoryV2
	text, err = taskType.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("HistoryV2"), text)

	taskType = ReplicationTaskTypeFailoverMarker
	text, err = taskType.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("FailoverMarker"), text)

	// Test unknown task type
	taskType = ReplicationTaskType(10)
	text, err = taskType.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("ReplicationTaskType(10)"), text)
}

func TestSyncActivityTaskAttributes_GetDomainID(t *testing.T) {
	testStruct := SyncActivityTaskAttributes{
		DomainID: "domain-id",
	}

	res := testStruct.GetDomainID()
	assert.Equal(t, "domain-id", res)

	var nilStruct *SyncActivityTaskAttributes
	res = nilStruct.GetDomainID()
	assert.Equal(t, "", res)
}

func TestSyncActivityTaskAttributes_GetWorkflowID(t *testing.T) {
	testStruct := SyncActivityTaskAttributes{
		WorkflowID: "workflow-id",
	}

	res := testStruct.GetWorkflowID()
	assert.Equal(t, "workflow-id", res)

	var nilStruct *SyncActivityTaskAttributes
	res = nilStruct.GetWorkflowID()
	assert.Equal(t, "", res)
}

func TestSyncActivityTaskAttributes_GetRunID(t *testing.T) {
	testStruct := SyncActivityTaskAttributes{
		RunID: "run-id",
	}

	res := testStruct.GetRunID()
	assert.Equal(t, "run-id", res)

	var nilStruct *SyncActivityTaskAttributes
	res = nilStruct.GetRunID()
	assert.Equal(t, "", res)
}

func TestSyncActivityTaskAttributes_GetVersion(t *testing.T) {
	testStruct := SyncActivityTaskAttributes{
		Version: 12345,
	}

	res := testStruct.GetVersion()
	assert.Equal(t, int64(12345), res)

	var nilStruct *SyncActivityTaskAttributes
	res = nilStruct.GetVersion()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityTaskAttributes_GetScheduledID(t *testing.T) {
	testStruct := SyncActivityTaskAttributes{
		ScheduledID: 67890,
	}

	res := testStruct.GetScheduledID()
	assert.Equal(t, int64(67890), res)

	var nilStruct *SyncActivityTaskAttributes
	res = nilStruct.GetScheduledID()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityTaskAttributes_GetScheduledTime(t *testing.T) {
	scheduledTime := int64(1234567890)
	testStruct := SyncActivityTaskAttributes{
		ScheduledTime: &scheduledTime,
	}

	res := testStruct.GetScheduledTime()
	assert.Equal(t, scheduledTime, res)

	var nilStruct *SyncActivityTaskAttributes
	res = nilStruct.GetScheduledTime()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityTaskAttributes_GetStartedID(t *testing.T) {
	testStruct := SyncActivityTaskAttributes{
		StartedID: 11111,
	}

	res := testStruct.GetStartedID()
	assert.Equal(t, int64(11111), res)

	var nilStruct *SyncActivityTaskAttributes
	res = nilStruct.GetStartedID()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityTaskAttributes_GetStartedTime(t *testing.T) {
	startedTime := int64(1234567890)
	testStruct := SyncActivityTaskAttributes{
		StartedTime: &startedTime,
	}

	res := testStruct.GetStartedTime()
	assert.Equal(t, startedTime, res)

	var nilStruct *SyncActivityTaskAttributes
	res = nilStruct.GetStartedTime()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityTaskAttributes_GetLastHeartbeatTime(t *testing.T) {
	lastHeartbeatTime := int64(9876543210)
	testStruct := SyncActivityTaskAttributes{
		LastHeartbeatTime: &lastHeartbeatTime,
	}

	res := testStruct.GetLastHeartbeatTime()
	assert.Equal(t, lastHeartbeatTime, res)

	var nilStruct *SyncActivityTaskAttributes
	res = nilStruct.GetLastHeartbeatTime()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityTaskAttributes_GetVersionHistory(t *testing.T) {
	versionHistory := &VersionHistory{}
	testStruct := SyncActivityTaskAttributes{
		VersionHistory: versionHistory,
	}

	res := testStruct.GetVersionHistory()
	assert.Equal(t, versionHistory, res)

	var nilStruct *SyncActivityTaskAttributes
	res = nilStruct.GetVersionHistory()
	assert.Nil(t, res)
}

func TestSyncShardStatus_GetTimestamp(t *testing.T) {
	timestamp := int64(9876543210)
	testStruct := SyncShardStatus{
		Timestamp: &timestamp,
	}

	res := testStruct.GetTimestamp()
	assert.Equal(t, timestamp, res)

	var nilStruct *SyncShardStatus
	res = nilStruct.GetTimestamp()
	assert.Equal(t, int64(0), res)
}
