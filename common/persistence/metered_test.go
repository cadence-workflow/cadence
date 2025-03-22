package persistence

import (
	"github.com/stretchr/testify/assert"
	"github.com/uber/cadence/common/types"
	"testing"
	"time"
)

func TestGetReplicationTaskResponseEstimatePayloadSize(t *testing.T) {
	t.Run("response is nil", func(t *testing.T) {
		var response *GetReplicationTasksResponse
		assert.Equal(t, 0, response.EstimatePayloadSizeInBytes())
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := &GetReplicationTasksResponse{
			Tasks: []*ReplicationTaskInfo{
				nil,
				{
					DomainID: "domainID", WorkflowID: "workflowID", RunID: "runID",
					TaskID: 0, TaskType: 1, FirstEventID: 2, NextEventID: 3, Version: 4, ScheduledID: 5, CreationTime: 6,
					BranchToken: nil, NewRunBranchToken: nil,
				},
			},
			NextPageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, 226, response.EstimatePayloadSizeInBytes())
	})

	t.Run("response with bigger payload emits a bigger value", func(t *testing.T) {
		response := &GetReplicationTasksResponse{
			Tasks: []*ReplicationTaskInfo{
				{
					DomainID: "domainID", WorkflowID: "workflowID", RunID: "runID",
					TaskID: 0, TaskType: 1, FirstEventID: 2, NextEventID: 3, Version: 4, ScheduledID: 5, CreationTime: 6,
					BranchToken: []byte{1, 2, 3, 4, 5, 6}, NewRunBranchToken: []byte{1, 1, 1},
				},
			},
			NextPageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, 235, response.EstimatePayloadSizeInBytes())
	})
}

func TestGetTimerIndexTasksResponseEstimatePayloadSize(t *testing.T) {
	t.Run("response is nil", func(t *testing.T) {
		var response *GetTimerIndexTasksResponse
		assert.Equal(t, 0, response.EstimatePayloadSizeInBytes())
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := &GetTimerIndexTasksResponse{
			Timers: []*TimerTaskInfo{
				nil,
				{
					DomainID: "domainID", WorkflowID: "workflowID", RunID: "runID",
					VisibilityTimestamp: time.Time{},
					TaskID:              0, TaskType: 0, TimeoutType: 0, EventID: 0, ScheduleAttempt: 0, Version: 0,
				},
			},
			NextPageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, 194, response.EstimatePayloadSizeInBytes())
	})

	t.Run("response with bigger payload emits a bigger value", func(t *testing.T) {
		response := &GetTimerIndexTasksResponse{
			Timers: []*TimerTaskInfo{
				nil,
				{
					DomainID: "longDomainID", WorkflowID: "longWorkflowID", RunID: "longRunID",
					VisibilityTimestamp: time.Time{},
					TaskID:              0, TaskType: 0, TimeoutType: 0, EventID: 0, ScheduleAttempt: 0, Version: 0,
				},
			},
			NextPageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, 206, response.EstimatePayloadSizeInBytes())
	})
}

func TestGetTasksResponseEstimatePayloadSize(t *testing.T) {
	t.Run("response is nil", func(t *testing.T) {
		var response *GetTasksResponse
		assert.Equal(t, 0, response.EstimatePayloadSizeInBytes())
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := &GetTasksResponse{
			Tasks: []*TaskInfo{
				nil,
				{
					DomainID: "domainID", WorkflowID: "workflowID", RunID: "runID",
					TaskID: 0, ScheduleID: 0, ScheduleToStartTimeoutSeconds: 0,
					Expiry:          time.Time{},
					CreatedTime:     time.Time{},
					PartitionConfig: nil,
				},
			},
		}

		assert.Equal(t, 175, response.EstimatePayloadSizeInBytes())
	})

	t.Run("response with bigger payload emits a bigger value", func(t *testing.T) {
		response := &GetTasksResponse{
			Tasks: []*TaskInfo{
				{
					DomainID: "domainID", WorkflowID: "workflowID", RunID: "runID",
					TaskID: 0, ScheduleID: 0, ScheduleToStartTimeoutSeconds: 0,
					Expiry:      time.Time{},
					CreatedTime: time.Time{},
					PartitionConfig: map[string]string{
						"key":  "value",
						"key2": "value2",
					},
				},
			},
		}

		assert.Equal(t, 193, response.EstimatePayloadSizeInBytes())
	})
}

func TestGetListDomainsResponseEstimatePayloadSize(t *testing.T) {
	t.Run("response is nil", func(t *testing.T) {
		var response *ListDomainsResponse
		assert.Equal(t, 0, response.EstimatePayloadSizeInBytes())
	})

	t.Run("domain info", func(t *testing.T) {
		assert.Equal(t, 0, estimateDomainInfoSize(nil))
		assert.Equal(t, 95, estimateDomainInfoSize(&DomainInfo{
			ID: "ID", Name: "Name", Status: 2, Description: "Desc", OwnerEmail: "Email", Data: nil,
		}))
		assert.Equal(t, 103, estimateDomainInfoSize(&DomainInfo{
			ID: "ID", Name: "Name", Status: 0, Description: "Desc", OwnerEmail: "Email",
			Data: map[string]string{"key": "value"},
		}))
	})

	t.Run("domain replication config", func(t *testing.T) {
		assert.Equal(t, 0, estimateDomainReplicationConfigSize(nil))
		assert.Equal(t, 7, estimateDomainReplicationConfigSize(&DomainReplicationConfig{
			ActiveClusterName: "cluster",
			Clusters:          nil,
		}))
		assert.Equal(t, 14, estimateDomainReplicationConfigSize(&DomainReplicationConfig{
			ActiveClusterName: "cluster",
			Clusters: []*ClusterReplicationConfig{
				nil,
				{"cluster"},
			},
		}))
	})

	t.Run("domain config", func(t *testing.T) {
		assert.Equal(t, 0, estimateDomainConfigSize(nil))
		assert.Equal(t, 184, estimateDomainConfigSize(&DomainConfig{
			Retention: 0, EmitMetric: false, HistoryArchivalStatus: 0, VisibilityArchivalStatus: 0,
			HistoryArchivalURI:    "URI",
			VisibilityArchivalURI: "VisibilityURI",
			BadBinaries:           types.BadBinaries{Binaries: nil},
			IsolationGroups:       nil,
			AsyncWorkflowConfig:   types.AsyncWorkflowConfiguration{},
		}))
		assert.Equal(t, 219, estimateDomainConfigSize(&DomainConfig{
			Retention: 0, EmitMetric: false, HistoryArchivalStatus: 0, VisibilityArchivalStatus: 0,
			HistoryArchivalURI:    "URI",
			VisibilityArchivalURI: "VisibilityURI",
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{
					"key":  nil,
					"key2": {Reason: "reason", Operator: "op", CreatedTimeNano: nil},
				},
			},
			IsolationGroups: map[string]types.IsolationGroupPartition{
				"key": {Name: "abc", State: 0},
			},
			AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
				Enabled:             false,
				PredefinedQueueName: "queue",
				QueueType:           "queueType",
				QueueConfig:         nil,
			},
		}))
		assert.Equal(t, 203, estimateDomainConfigSize(&DomainConfig{
			Retention: 0, EmitMetric: false, HistoryArchivalStatus: 0, VisibilityArchivalStatus: 0,
			HistoryArchivalURI:    "URI",
			VisibilityArchivalURI: "VisibilityURI",
			BadBinaries:           types.BadBinaries{},
			IsolationGroups:       nil,
			AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
				Enabled:             false,
				PredefinedQueueName: "queue",
				QueueType:           "queueType",
				QueueConfig: &types.DataBlob{
					EncodingType: nil, Data: []byte{1, 2, 3, 4, 5},
				},
			},
		}))
	})

	t.Run("full response", func(t *testing.T) {
		response := &ListDomainsResponse{
			Domains: []*GetDomainResponse{
				nil,
				{
					Info: nil, Config: nil, ReplicationConfig: nil, IsGlobalDomain: false,
					ConfigVersion: 0, FailoverVersion: 0, FailoverNotificationVersion: 0, PreviousFailoverVersion: 0,
					FailoverEndTime: nil, LastUpdatedTime: 0, NotificationVersion: 0,
				},
				{
					Info: &DomainInfo{
						ID: "ID", Name: "Name", Status: 0, Description: "Desc", OwnerEmail: "Email",
						Data: map[string]string{
							"key": "value",
						},
					},
					Config: &DomainConfig{
						Retention: 0, EmitMetric: false, HistoryArchivalStatus: 0, HistoryArchivalURI: "URI",
						VisibilityArchivalStatus: 0, VisibilityArchivalURI: "URI", BadBinaries: types.BadBinaries{},
						IsolationGroups: map[string]types.IsolationGroupPartition{
							"key": {Name: "abc", State: 0},
						},
						AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
							Enabled:             false,
							PredefinedQueueName: "name",
							QueueType:           "type",
							QueueConfig: &types.DataBlob{
								EncodingType: nil,
								Data:         []byte{1, 2, 3},
							},
						},
					},
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster",
						Clusters: []*ClusterReplicationConfig{
							{ClusterName: "cluster"},
						},
					},
					IsGlobalDomain: false, ConfigVersion: 0, FailoverVersion: 0, FailoverNotificationVersion: 0, PreviousFailoverVersion: 0,
					FailoverEndTime: nil, LastUpdatedTime: 0, NotificationVersion: 0,
				},
			},
			NextPageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, 535, response.EstimatePayloadSizeInBytes())
	})
}

func TestRawReadHistoryResponseEstimatePayloadSize(t *testing.T) {
	t.Run("response is nil", func(t *testing.T) {
		var response *ReadRawHistoryBranchResponse
		assert.Equal(t, 0, response.EstimatePayloadSizeInBytes())
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := ReadRawHistoryBranchResponse{
			HistoryEventBlobs: []*DataBlob{
				nil,
				{
					Encoding: "abc",
					Data:     []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				},
			},
			NextPageToken: []byte{1, 2, 3},
			Size:          123,
		}

		assert.Equal(t, 69, response.EstimatePayloadSizeInBytes())
	})

	t.Run("a bigger response emits a bigger value", func(t *testing.T) {
		response := ReadRawHistoryBranchResponse{
			HistoryEventBlobs: []*DataBlob{
				{
					Encoding: "abc",
					Data:     []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
				},
			},
			NextPageToken: []byte{1, 2, 3},
			Size:          123,
		}

		assert.Equal(t, 73, response.EstimatePayloadSizeInBytes())
	})
}

func TestListCurrentExecutionsResponseEstimatePayloadSize(t *testing.T) {
	t.Run("response is nil", func(t *testing.T) {
		var response *ListCurrentExecutionsResponse
		assert.Equal(t, 0, response.EstimatePayloadSizeInBytes())
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := &ListCurrentExecutionsResponse{
			Executions: []*CurrentWorkflowExecution{
				nil,
				{DomainID: "DomainID", WorkflowID: "WorkflowID", RunID: "ID", State: 0, CurrentRunID: "ID"},
			},
			PageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, 145, response.EstimatePayloadSizeInBytes())
	})

	t.Run("a bigger response emits a bigger value", func(t *testing.T) {
		response := &ListCurrentExecutionsResponse{
			Executions: []*CurrentWorkflowExecution{
				{DomainID: "LongDomainID", WorkflowID: "LongWorkflowID", RunID: "ID", State: 0, CurrentRunID: "ID"},
			},
			PageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, 153, response.EstimatePayloadSizeInBytes())
	})
}

func TestGetTransferTasksResponseEstimatePayloadSize(t *testing.T) {
	t.Run("response is nil", func(t *testing.T) {
		var response *GetTransferTasksResponse
		assert.Equal(t, 0, response.EstimatePayloadSizeInBytes())
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := &GetTransferTasksResponse{
			Tasks: []*TransferTaskInfo{
				nil,
				{
					DomainID: "DomainID", WorkflowID: "WorkflowID", RunID: "ID",
					TargetDomainID: "DomainID", TargetWorkflowID: "WfID", TargetRunID: "RunID",
					VisibilityTimestamp: time.Time{}, TaskID: 0, TargetChildWorkflowOnly: false,
					TaskList: "", TaskType: 0, ScheduleID: 0, Version: 0, RecordVisibility: false,
					TargetDomainIDs: nil,
				},
			},
			NextPageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, 280, response.EstimatePayloadSizeInBytes())
	})

	t.Run("a bigger response emits a bigger value", func(t *testing.T) {
		response := &GetTransferTasksResponse{
			Tasks: []*TransferTaskInfo{
				{
					DomainID: "LongDomainID", WorkflowID: "LongWorkflowID", RunID: "ID",
					TargetDomainID: "LongDomainID", TargetWorkflowID: "WfID", TargetRunID: "RunID",
					VisibilityTimestamp: time.Time{}, TaskID: 0, TargetChildWorkflowOnly: false,
					TaskList: "", TaskType: 0, ScheduleID: 0, Version: 0, RecordVisibility: false,
					TargetDomainIDs: map[string]struct{}{
						"key": {},
					},
				},
			},
			NextPageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, 292, response.EstimatePayloadSizeInBytes())
	})
}

func TestQueueMessageListEstimatePayloadSize(t *testing.T) {
	t.Run("response is not nil", func(t *testing.T) {
		response := &QueueMessageList{
			{ID: 0, QueueType: 0, Payload: nil},
		}

		assert.Equal(t, 40, response.EstimatePayloadSizeInBytes())
	})

	t.Run("a bigger response emits a bigger value", func(t *testing.T) {
		response := &QueueMessageList{
			{ID: 0, QueueType: 0, Payload: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
		}

		assert.Equal(t, 50, response.EstimatePayloadSizeInBytes())
	})
}

func TestGetAllHistoryTreeBranchesResponseEstimatePayloadSize(t *testing.T) {
	t.Run("response is not nil", func(t *testing.T) {
		response := &GetAllHistoryTreeBranchesResponse{
			NextPageToken: []byte{1, 2, 3},
			Branches: []HistoryBranchDetail{
				{TreeID: "", BranchID: "", ForkTime: time.Time{}, Info: ""},
			},
		}

		assert.Equal(t, 123, response.EstimatePayloadSizeInBytes())
	})

	t.Run("a bigger response emits a bigger value", func(t *testing.T) {
		response := &GetAllHistoryTreeBranchesResponse{
			NextPageToken: []byte{1, 2, 3},
			Branches: []HistoryBranchDetail{
				{TreeID: "TreeID", BranchID: "BID", ForkTime: time.Time{}, Info: "Info"},
			},
		}

		assert.Equal(t, 136, response.EstimatePayloadSizeInBytes())
	})
}
