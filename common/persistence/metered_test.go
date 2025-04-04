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

package persistence

import (
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/testing/testdatagen"
	"github.com/uber/cadence/common/types"
)

func TestGetTasksResponseEstimatePayloadSize(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		fuzzer := testdatagen.NewWithNilChance(t, int64(123), 0.25)
		assert.NotPanics(t, func() {
			for i := 0; i < 100; i++ {
				response := &GetTasksResponse{}
				fuzzer.Fuzz(&response)

				_ = response.ByteSize()
			}
		})
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := &GetTasksResponse{
			Tasks: []*TaskInfo{
				{
					DomainID: "domainID", WorkflowID: "workflowID", RunID: "runID",
					TaskID: 0, ScheduleID: 0, ScheduleToStartTimeoutSeconds: 0,
					Expiry:          time.Time{},
					CreatedTime:     time.Time{},
					PartitionConfig: nil,
				},
			},
		}

		assert.Equal(t, uint64(175), response.ByteSize())
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

		assert.Equal(t, uint64(193), response.ByteSize())
	})
}

func TestGetListDomainsResponseEstimatePayloadSize(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		fuzzer := testdatagen.NewWithNilChance(t, int64(123), 0.25)
		assert.NotPanics(t, func() {
			for i := 0; i < 100; i++ {
				response := &ListDomainsResponse{}
				fuzzer.Fuzz(&response)

				_ = response.ByteSize()
			}
		})
	})

	t.Run("domain info", func(t *testing.T) {
		info := &DomainInfo{ID: "ID", Name: "Name", Status: 2, Description: "Desc", OwnerEmail: "Email", Data: nil}
		assert.Equal(t, uint64(95), info.ByteSize())

		info = &DomainInfo{
			ID: "ID", Name: "Name", Status: 0, Description: "Desc", OwnerEmail: "Email",
			Data: map[string]string{"key": "value"},
		}
		assert.Equal(t, uint64(103), info.ByteSize())
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

		assert.Equal(t, uint64(535), response.ByteSize())
	})
}

func TestRawReadHistoryResponseEstimatePayloadSize(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		fuzzer := testdatagen.NewWithNilChance(t, int64(123), 0.25)
		assert.NotPanics(t, func() {
			for i := 0; i < 100; i++ {
				response := &ReadRawHistoryBranchResponse{}
				fuzzer.Fuzz(&response)

				_ = response.Size2()
			}
		})
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

		assert.Equal(t, uint64(109), response.Size2())
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

		assert.Equal(t, uint64(113), response.Size2())
	})
}

func TestListCurrentExecutionsResponseEstimatePayloadSize(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		fuzzer := testdatagen.NewWithNilChance(t, int64(123), 0.25)
		assert.NotPanics(t, func() {
			for i := 0; i < 100; i++ {
				response := &ListCurrentExecutionsResponse{}
				fuzzer.Fuzz(&response)

				_ = response.ByteSize()
			}
		})
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := &ListCurrentExecutionsResponse{
			Executions: []*CurrentWorkflowExecution{
				nil,
				{DomainID: "DomainID", WorkflowID: "WorkflowID", RunID: "ID", State: 0, CurrentRunID: "ID"},
			},
			PageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, uint64(145), response.ByteSize())
	})

	t.Run("a bigger response emits a bigger value", func(t *testing.T) {
		response := &ListCurrentExecutionsResponse{
			Executions: []*CurrentWorkflowExecution{
				{DomainID: "LongDomainID", WorkflowID: "LongWorkflowID", RunID: "ID", State: 0, CurrentRunID: "ID"},
			},
			PageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, uint64(153), response.ByteSize())
	})
}

func TestGetHistoryTasksResponseEstimatePayloadSize(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		fuzzer := testdatagen.NewWithNilChance(t, int64(123), 0.25).Funcs(
			func(t *Task, c fuzz.Continue) {
				switch c.Intn(21) {
				case 0:
					var m DecisionTask
					c.Fuzz(&m)
					*t = &m
				case 1:
					var m ActivityTask
					c.Fuzz(&m)
					*t = &m
				case 2:
					var m RecordWorkflowStartedTask
					c.Fuzz(&m)
					*t = &m
				case 3:
					var m ResetWorkflowTask
					c.Fuzz(&m)
					*t = &m
				case 4:
					var m CloseExecutionTask
					c.Fuzz(&m)
					*t = &m
				case 5:
					var m DeleteHistoryEventTask
					c.Fuzz(&m)
					*t = &m
				case 6:
					var m DecisionTimeoutTask
					c.Fuzz(&m)
					*t = &m
				case 7:
					var m WorkflowTimeoutTask
					c.Fuzz(&m)
					*t = &m
				case 8:
					var m CancelExecutionTask
					c.Fuzz(&m)
					*t = &m
				case 9:
					var m SignalExecutionTask
					c.Fuzz(&m)
					*t = &m
				case 10:
					var m RecordChildExecutionCompletedTask
					c.Fuzz(&m)
					*t = &m
				case 11:
					var m UpsertWorkflowSearchAttributesTask
					c.Fuzz(&m)
					*t = &m
				case 12:
					var m UserTimerTask
					c.Fuzz(&m)
					*t = &m
				case 13:
					var m ActivityTimeoutTask
					c.Fuzz(&m)
					*t = &m
				case 14:
					var m ActivityRetryTimerTask
					c.Fuzz(&m)
					*t = &m
				case 15:
					var m WorkflowBackoffTimerTask
					c.Fuzz(&m)
					*t = &m
				case 16:
					var m HistoryReplicationTask
					c.Fuzz(&m)
					*t = &m
				case 17:
					var m StartChildExecutionTask
					c.Fuzz(&m)
					*t = &m
				case 18:
					var m RecordWorkflowClosedTask
					c.Fuzz(&m)
					*t = &m
				case 19:
					var m SyncActivityTask
					c.Fuzz(&m)
					*t = &m
				case 20:
					var m FailoverMarkerTask
					c.Fuzz(&m)
					*t = &m
				}
			},
		)
		assert.NotPanics(t, func() {
			for i := 0; i < 100; i++ {
				response := &GetHistoryTasksResponse{}
				fuzzer.Fuzz(&response)

				_ = response.ByteSize()
			}
		})
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := &GetHistoryTasksResponse{
			Tasks: []Task{
				nil,
				&DecisionTask{
					WorkflowIdentifier: WorkflowIdentifier{
						DomainID: "DomainID", WorkflowID: "WorkflowID", RunID: "ID",
					},
					TaskData: TaskData{
						TaskID:              0,
						VisibilityTimestamp: time.Time{},
						Version:             0,
					},
					TargetDomainID: "DomainID",
					TaskList:       "", ScheduleID: 0,
				},
			},
			NextPageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, uint64(127), response.ByteSize())
	})

	t.Run("a bigger response emits a bigger value", func(t *testing.T) {
		response := &GetHistoryTasksResponse{
			Tasks: []Task{
				&SignalExecutionTask{
					WorkflowIdentifier: WorkflowIdentifier{
						DomainID: "DomainID", WorkflowID: "WorkflowID", RunID: "ID",
					},
					TaskData: TaskData{
						TaskID:              0,
						VisibilityTimestamp: time.Time{},
						Version:             0,
					},
					TargetDomainID:          "DomainID",
					TargetWorkflowID:        "WorkflowID",
					TargetRunID:             "ID",
					TargetChildWorkflowOnly: false,
				},
			},
			NextPageToken: []byte{1, 2, 3},
		}

		assert.Equal(t, uint64(140), response.ByteSize())
	})
}

func TestQueueMessageListEstimatePayloadSize(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		fuzzer := testdatagen.NewWithNilChance(t, int64(123), 0.25)
		assert.NotPanics(t, func() {
			for i := 0; i < 100; i++ {
				response := &QueueMessageList{}
				fuzzer.Fuzz(response)

				_ = response.ByteSize()
			}
		})
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := &QueueMessageList{
			{ID: 0, QueueType: 0, Payload: nil},
		}

		assert.Equal(t, uint64(40), response.ByteSize())
	})

	t.Run("a bigger response emits a bigger value", func(t *testing.T) {
		response := &QueueMessageList{
			{ID: 0, QueueType: 0, Payload: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
		}

		assert.Equal(t, uint64(50), response.ByteSize())
	})
}

func TestGetAllHistoryTreeBranchesResponseEstimatePayloadSize(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		fuzzer := testdatagen.NewWithNilChance(t, int64(123), 0.25)
		assert.NotPanics(t, func() {
			for i := 0; i < 100; i++ {
				response := &GetAllHistoryTreeBranchesResponse{}
				fuzzer.Fuzz(response)

				_ = response.ByteSize()
			}
		})
	})

	t.Run("response is not nil", func(t *testing.T) {
		response := &GetAllHistoryTreeBranchesResponse{
			NextPageToken: []byte{1, 2, 3},
			Branches: []HistoryBranchDetail{
				{TreeID: "", BranchID: "", ForkTime: time.Time{}, Info: ""},
			},
		}

		assert.Equal(t, uint64(123), response.ByteSize())
	})

	t.Run("a bigger response emits a bigger value", func(t *testing.T) {
		response := &GetAllHistoryTreeBranchesResponse{
			NextPageToken: []byte{1, 2, 3},
			Branches: []HistoryBranchDetail{
				{TreeID: "TreeID", BranchID: "BID", ForkTime: time.Time{}, Info: "Info"},
			},
		}

		assert.Equal(t, uint64(136), response.ByteSize())
	})
}
