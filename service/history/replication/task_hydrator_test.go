// The MIT License (MIT)
//
// Copyright (c) 2017-2022 Uber Technologies Inc.
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

package replication

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

const (
	testShardID           = 0
	testDomainID          = "11111111-1111-1111-1111-111111111111"
	testWorkflowID        = "workflow-id"
	testRunID             = "22222222-2222-2222-2222-222222222222"
	testTaskID            = 111
	testCreationTime      = int64(333)
	testFirstEventID      = 6
	testNextEventID       = 8
	testVersion           = 456
	testScheduleID        = int64(10)
	testStartedID         = int64(11)
	testLastFailureReason = "failure-reason"
	testWorkerIdentity    = "worker-identity"
	testAttempt           = 42
)

var (
	testBranchToken               = []byte{91, 92, 93}
	testBranchTokenNewRun         = []byte{94, 95, 96}
	testBranchTokenVersionHistory = []byte{97, 98, 99}
	testDataBlob                  = &types.DataBlob{Data: []byte{1, 2, 3}, EncodingType: types.EncodingTypeJSON.Ptr()}
	testDataBlobNewRun            = &types.DataBlob{Data: []byte{4, 5, 6}, EncodingType: types.EncodingTypeJSON.Ptr()}
	testDataBlobVersionHistory    = &types.DataBlob{Data: []byte{7, 8, 9}, EncodingType: types.EncodingTypeJSON.Ptr()}
	testDetails                   = []byte{100, 101, 102}
	testLastFailureDetails        = []byte{103, 104, 105}
	testScheduleTime              = time.Now()
	testStartedTime               = time.Now()
	testHeartbeatTime             = time.Now()
	testWorkflowIdentifier        = definition.NewWorkflowIdentifier(testDomainID, testWorkflowID, testRunID)
)

func TestNewDeferredTaskHydrator(t *testing.T) {
	h := NewDeferredTaskHydrator(0, nil, nil, nil)
	require.NotNil(t, h)
	assert.IsType(t, historyLoader{}, h.history)
	assert.IsType(t, mutableStateLoader{}, h.msProvider)
}

func TestTaskHydrator_UnknownTask(t *testing.T) {
	task := &persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   testWorkflowIdentifier.DomainID,
			WorkflowID: testWorkflowIdentifier.WorkflowID,
			RunID:      testWorkflowIdentifier.RunID,
		},
	}
	th := TaskHydrator{msProvider: &fakeMutableStateProvider{
		workflows: map[definition.WorkflowIdentifier]mutableState{
			testWorkflowIdentifier: &fakeMutableState{},
		},
	}}
	result, err := th.Hydrate(context.Background(), task)
	assert.Equal(t, errUnknownReplicationTask, err)
	assert.Nil(t, result)
	assert.True(t, th.msProvider.(*fakeMutableStateProvider).released)
}

func TestTaskHydrator_HydrateFailoverMarkerTask(t *testing.T) {
	task := &persistence.FailoverMarkerTask{
		DomainID: testDomainID,
		TaskData: persistence.TaskData{
			TaskID:              testTaskID,
			Version:             testVersion,
			VisibilityTimestamp: time.Unix(0, testCreationTime),
		},
	}

	expected := types.ReplicationTask{
		TaskType:     types.ReplicationTaskTypeFailoverMarker.Ptr(),
		SourceTaskID: testTaskID,
		FailoverMarkerAttributes: &types.FailoverMarkerAttributes{
			DomainID:        testDomainID,
			FailoverVersion: testVersion,
		},
		CreationTime: common.Int64Ptr(testCreationTime),
	}

	th := TaskHydrator{}
	actual, err := th.Hydrate(context.Background(), task)
	assert.NoError(t, err)
	assert.Equal(t, &expected, actual)
}

func TestTaskHydrator_HydrateSyncActivityTask(t *testing.T) {
	task := &persistence.SyncActivityTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   testDomainID,
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
		TaskData: persistence.TaskData{
			TaskID:              testTaskID,
			Version:             testVersion,
			VisibilityTimestamp: time.Unix(0, testCreationTime),
		},
		ScheduledID: testScheduleID,
	}

	versionHistories := &persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: testBranchTokenVersionHistory,
				Items: []*persistence.VersionHistoryItem{
					{EventID: testFirstEventID, Version: testVersion},
				},
			},
		},
	}
	activityInfo := persistence.ActivityInfo{
		Version:                  testVersion,
		ScheduleID:               testScheduleID,
		ScheduledTime:            testScheduleTime,
		StartedID:                testStartedID,
		StartedTime:              testStartedTime,
		DomainID:                 testDomainID,
		LastHeartBeatUpdatedTime: testHeartbeatTime,
		Details:                  testDetails,
		Attempt:                  testAttempt,
		LastFailureReason:        testLastFailureReason,
		LastFailureDetails:       testLastFailureDetails,
		LastWorkerIdentity:       testWorkerIdentity,
	}

	tests := []struct {
		name       string
		task       persistence.Task
		msProvider mutableStateProvider
		expectTask *types.ReplicationTask
		expectErr  string
	}{
		{
			name: "hydrates sync activity task",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{
					testWorkflowIdentifier: &fakeMutableState{
						isWorkflowExecutionRunning: true,
						versionHistories:           versionHistories,
						activityInfos:              map[int64]persistence.ActivityInfo{testScheduleID: activityInfo},
					},
				},
			},
			expectTask: &types.ReplicationTask{
				TaskType:     types.ReplicationTaskTypeSyncActivity.Ptr(),
				SourceTaskID: testTaskID,
				CreationTime: common.Int64Ptr(testCreationTime),
				SyncActivityTaskAttributes: &types.SyncActivityTaskAttributes{
					DomainID:           testDomainID,
					WorkflowID:         testWorkflowID,
					RunID:              testRunID,
					Version:            testVersion,
					ScheduledID:        testScheduleID,
					ScheduledTime:      common.Int64Ptr(testScheduleTime.UnixNano()),
					StartedID:          testStartedID,
					StartedTime:        common.Int64Ptr(testStartedTime.UnixNano()),
					LastHeartbeatTime:  common.Int64Ptr(testHeartbeatTime.UnixNano()),
					Details:            testDetails,
					Attempt:            testAttempt,
					LastFailureReason:  common.StringPtr(testLastFailureReason),
					LastWorkerIdentity: testWorkerIdentity,
					LastFailureDetails: testLastFailureDetails,
					VersionHistory: &types.VersionHistory{
						Items:       []*types.VersionHistoryItem{{EventID: testFirstEventID, Version: testVersion}},
						BranchToken: testBranchTokenVersionHistory,
					},
				},
			},
		},
		{
			name: "workflow is not running - return nil, no error",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{
					testWorkflowIdentifier: &fakeMutableState{
						isWorkflowExecutionRunning: false,
					},
				},
			},
			expectTask: nil,
		},
		{
			name: "no activity info - return nil, no error",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{
					testWorkflowIdentifier: &fakeMutableState{
						isWorkflowExecutionRunning: true,
						activityInfos:              map[int64]persistence.ActivityInfo{},
					},
				},
			},
			expectTask: nil,
		},
		{
			name: "bad version histories - return error",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{
					testWorkflowIdentifier: &fakeMutableState{
						isWorkflowExecutionRunning: true,
						versionHistories:           &persistence.VersionHistories{},
						activityInfos:              map[int64]persistence.ActivityInfo{testScheduleID: activityInfo},
					},
				},
			},
			expectErr: "getting branch index: 0, available branch count: 0",
		},
		{
			name: "workflow does not exist - return nil, no error",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{},
			},
			expectTask: nil,
		},
		{
			name:       "error loading mutable state",
			task:       task,
			msProvider: &fakeMutableStateProvider{},
			expectErr:  "error loading mutable state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := TaskHydrator{msProvider: tt.msProvider}
			actualTask, err := th.Hydrate(context.Background(), tt.task)
			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectTask, actualTask)
			}
			assert.True(t, th.msProvider.(*fakeMutableStateProvider).released)
		})
	}
}

func TestTaskHydrator_HydrateHistoryReplicationTask(t *testing.T) {
	task := &persistence.HistoryReplicationTask{
		TaskData: persistence.TaskData{
			TaskID:              testTaskID,
			Version:             testVersion,
			VisibilityTimestamp: time.Unix(0, testCreationTime),
		},
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   testDomainID,
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
		FirstEventID:      testFirstEventID,
		NextEventID:       testNextEventID,
		BranchToken:       testBranchToken,
		NewRunBranchToken: testBranchTokenNewRun,
	}
	taskWithoutBranchToken := &persistence.HistoryReplicationTask{
		TaskData: persistence.TaskData{
			TaskID:              testTaskID,
			Version:             testVersion,
			VisibilityTimestamp: time.Unix(0, testCreationTime),
		},
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   testDomainID,
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
		FirstEventID:      testFirstEventID,
		NextEventID:       testNextEventID,
		NewRunBranchToken: testBranchTokenNewRun,
	}

	versionHistories := persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: testBranchTokenVersionHistory,
				Items: []*persistence.VersionHistoryItem{
					{EventID: testFirstEventID, Version: testVersion},
				},
			},
		},
	}

	tests := []struct {
		name       string
		task       *persistence.HistoryReplicationTask
		msProvider mutableStateProvider
		history    historyProvider
		expectTask *types.ReplicationTask
		expectErr  string
	}{
		{
			name: "hydrates history with given branch token",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{
					testWorkflowIdentifier: &fakeMutableState{versionHistories: &versionHistories},
				},
			},
			history: &fakeHistoryProvider{
				blobs: []historyBlob{
					{branch: testBranchToken, blob: testDataBlob},
					{branch: testBranchTokenNewRun, blob: testDataBlobNewRun},
				},
			},
			expectTask: &types.ReplicationTask{
				TaskType:     types.ReplicationTaskTypeHistoryV2.Ptr(),
				SourceTaskID: testTaskID,
				CreationTime: common.Int64Ptr(testCreationTime),
				HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
					DomainID:            testDomainID,
					WorkflowID:          testWorkflowID,
					RunID:               testRunID,
					VersionHistoryItems: []*types.VersionHistoryItem{{EventID: testFirstEventID, Version: testVersion}},
					Events:              testDataBlob,
					NewRunEvents:        testDataBlobNewRun,
				},
			},
		},
		{
			name: "hydrates history with branch token from version histories",
			task: taskWithoutBranchToken,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{
					testWorkflowIdentifier: &fakeMutableState{versionHistories: &versionHistories},
				},
			},
			history: &fakeHistoryProvider{
				blobs: []historyBlob{
					{branch: testBranchTokenVersionHistory, blob: testDataBlobVersionHistory},
					{branch: testBranchTokenNewRun, blob: testDataBlobNewRun},
				},
			},
			expectTask: &types.ReplicationTask{
				TaskType:     types.ReplicationTaskTypeHistoryV2.Ptr(),
				SourceTaskID: testTaskID,
				CreationTime: common.Int64Ptr(testCreationTime),
				HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
					DomainID:            testDomainID,
					WorkflowID:          testWorkflowID,
					RunID:               testRunID,
					VersionHistoryItems: []*types.VersionHistoryItem{{EventID: testFirstEventID, Version: testVersion}},
					Events:              testDataBlobVersionHistory,
					NewRunEvents:        testDataBlobNewRun,
				},
			},
		},
		{
			name: "no version histories - return nil, no error",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{
					testWorkflowIdentifier: &fakeMutableState{versionHistories: nil},
				},
			},
			expectTask: nil,
		},
		{
			name: "bad version histories - return error",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{
					testWorkflowIdentifier: &fakeMutableState{versionHistories: &persistence.VersionHistories{}},
				},
			},
			expectErr: "version histories does not contains given item.",
		},
		{
			name: "workflow does not exist - return nil, no error",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{},
			},
			expectTask: nil,
		},
		{
			name:       "error loading mutable state",
			task:       task,
			msProvider: &fakeMutableStateProvider{},
			expectErr:  "error loading mutable state",
		},
		{
			name: "failed reading event blob - return error",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{
					testWorkflowIdentifier: &fakeMutableState{versionHistories: &versionHistories},
				},
			},
			history: &fakeHistoryProvider{
				blobs: []historyBlob{
					{branch: testBranchTokenNewRun, blob: testDataBlobNewRun},
				},
			},
			expectErr: "failed reading history",
		},
		{
			name: "failed reading event blob for new run - return error",
			task: task,
			msProvider: &fakeMutableStateProvider{
				workflows: map[definition.WorkflowIdentifier]mutableState{
					testWorkflowIdentifier: &fakeMutableState{versionHistories: &versionHistories},
				},
			},
			history: &fakeHistoryProvider{
				blobs: []historyBlob{
					{branch: testBranchToken, blob: testDataBlob},
				},
			},
			expectErr: "failed reading history",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := TaskHydrator{msProvider: tt.msProvider, history: tt.history}
			actualTask, err := th.Hydrate(context.Background(), tt.task)
			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectTask, actualTask)
			}
			assert.True(t, th.msProvider.(*fakeMutableStateProvider).released)
		})
	}
}

func TestHistoryLoader_GetEventBlob(t *testing.T) {
	tests := []struct {
		name           string
		task           *persistence.HistoryReplicationTask
		domains        fakeDomainCache
		mockHistory    func(hm *mocks.HistoryV2Manager)
		expectDataBlob *types.DataBlob
		expectErr      string
	}{
		{
			name: "loads data blob",
			task: &persistence.HistoryReplicationTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID: testDomainID,
				},
				BranchToken:  testBranchToken,
				FirstEventID: 10,
				NextEventID:  11,
			},
			domains: fakeDomainCache{testDomainID: testDomain},
			mockHistory: func(hm *mocks.HistoryV2Manager) {
				hm.On("ReadRawHistoryBranch", mock.Anything, &persistence.ReadHistoryBranchRequest{
					BranchToken: testBranchToken,
					MinEventID:  10,
					MaxEventID:  11,
					PageSize:    2,
					ShardID:     common.IntPtr(testShardID),
					DomainName:  testDomainName,
				}).Return(&persistence.ReadRawHistoryBranchResponse{
					HistoryEventBlobs: []*persistence.DataBlob{{Encoding: constants.EncodingTypeJSON, Data: testDataBlob.Data}},
				}, nil)
			},
			expectDataBlob: testDataBlob,
		},
		{
			name:        "failed to get domain name",
			task:        &persistence.HistoryReplicationTask{WorkflowIdentifier: persistence.WorkflowIdentifier{DomainID: testDomainID}},
			domains:     fakeDomainCache{},
			mockHistory: func(hm *mocks.HistoryV2Manager) {},
			expectErr:   "domain does not exist",
		},
		{
			name:    "load failure",
			task:    &persistence.HistoryReplicationTask{WorkflowIdentifier: persistence.WorkflowIdentifier{DomainID: testDomainID}},
			domains: fakeDomainCache{testDomainID: testDomain},
			mockHistory: func(hm *mocks.HistoryV2Manager) {
				hm.On("ReadRawHistoryBranch", mock.Anything, mock.Anything).Return(nil, errors.New("load failure"))
			},
			expectErr: "load failure",
		},
		{
			name:    "response must contain exactly one blob",
			task:    &persistence.HistoryReplicationTask{WorkflowIdentifier: persistence.WorkflowIdentifier{DomainID: testDomainID}},
			domains: fakeDomainCache{testDomainID: testDomain},
			mockHistory: func(hm *mocks.HistoryV2Manager) {
				hm.On("ReadRawHistoryBranch", mock.Anything, mock.Anything).Return(&persistence.ReadRawHistoryBranchResponse{
					HistoryEventBlobs: []*persistence.DataBlob{{}, {}}, // two blobs
				}, nil)
			},
			expectErr: "replication hydrator encountered more than 1 NDC raw event batch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := &mocks.HistoryV2Manager{}
			tt.mockHistory(hm)
			loader := historyLoader{shardID: testShardID, history: hm, domains: tt.domains}
			dataBlob, err := loader.GetEventBlob(context.Background(), tt.task)
			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectDataBlob, dataBlob)
			}
		})
	}
}

func TestHistoryLoader_GetNextRunEventBlob(t *testing.T) {
	hm := &mocks.HistoryV2Manager{}
	loader := historyLoader{shardID: testShardID, history: hm, domains: fakeDomainCache{testDomainID: testDomain}}

	dataBlob, err := loader.GetNextRunEventBlob(context.Background(), &persistence.HistoryReplicationTask{NewRunBranchToken: nil})
	assert.NoError(t, err)
	assert.Nil(t, dataBlob)

	hm.On("ReadRawHistoryBranch", mock.Anything, &persistence.ReadHistoryBranchRequest{
		BranchToken: testBranchTokenNewRun,
		MinEventID:  1,
		MaxEventID:  2,
		PageSize:    2,
		ShardID:     common.IntPtr(testShardID),
		DomainName:  testDomainName,
	}).Return(&persistence.ReadRawHistoryBranchResponse{
		HistoryEventBlobs: []*persistence.DataBlob{{Encoding: constants.EncodingTypeJSON, Data: testDataBlob.Data}},
	}, nil)
	dataBlob, err = loader.GetNextRunEventBlob(context.Background(), &persistence.HistoryReplicationTask{WorkflowIdentifier: persistence.WorkflowIdentifier{DomainID: testDomainID}, NewRunBranchToken: testBranchTokenNewRun})
	assert.NoError(t, err)
	assert.Equal(t, testDataBlob, dataBlob)
}

func TestMutableStateLoader_GetMutableState(t *testing.T) {
	ctx := context.Background()
	controller := gomock.NewController(t)
	testShardContext := shard.NewTestContext(
		t,
		controller,
		&persistence.ShardInfo{
			ShardID:                 testShardID,
			RangeID:                 1,
			TransferAckLevel:        0,
			ClusterReplicationLevel: make(map[string]int64),
		},
		config.NewForTest(),
	)
	domainCache := testShardContext.Resource.DomainCache
	executionCache := execution.NewCache(testShardContext)
	expectedMS := execution.NewMockMutableState(controller)
	msLoader := mutableStateLoader{executionCache}

	domainCache.EXPECT().GetDomainName(gomock.Any()).Return(testDomainName, nil).AnyTimes()
	exec, release, err := executionCache.GetOrCreateWorkflowExecution(ctx, testDomainID, types.WorkflowExecution{WorkflowID: testWorkflowID, RunID: testRunID})
	require.NoError(t, err)
	// Try getting mutable state while it is still locked, will result in an error
	contextWithTimeout, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()
	_, _, err = msLoader.GetMutableState(contextWithTimeout, testDomainID, testWorkflowID, testRunID)
	assert.EqualError(t, err, "context deadline exceeded")
	release(nil)

	// Error while trying to load mutable state will be returned
	domainCache.EXPECT().GetDomainByID("non-existing-domain").Return(nil, errors.New("does not exist"))
	_, _, err = msLoader.GetMutableState(ctx, "non-existing-domain", testWorkflowID, testRunID)
	assert.EqualError(t, err, "does not exist")

	// Happy path
	domainCache.EXPECT().GetDomainByID(testDomainID).Return(&cache.DomainCacheEntry{}, nil)
	expectedMS.EXPECT().StartTransaction(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
	exec.SetWorkflowExecution(expectedMS)
	ms, release, err := msLoader.GetMutableState(ctx, testDomainID, testWorkflowID, testRunID)
	assert.NoError(t, err)
	assert.Equal(t, expectedMS, ms)
	assert.NotNil(t, release)
	release(nil)
}

func TestImmediateTaskHydrator(t *testing.T) {
	activityInfo := persistence.ActivityInfo{
		Version:                  testVersion,
		ScheduleID:               testScheduleID,
		ScheduledTime:            testScheduleTime,
		StartedID:                testStartedID,
		StartedTime:              testStartedTime,
		DomainID:                 testDomainID,
		LastHeartBeatUpdatedTime: testHeartbeatTime,
		Details:                  testDetails,
		Attempt:                  testAttempt,
		LastFailureReason:        testLastFailureReason,
		LastFailureDetails:       testLastFailureDetails,
		LastWorkerIdentity:       testWorkerIdentity,
	}
	versionHistories := &persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: testBranchTokenVersionHistory,
				Items: []*persistence.VersionHistoryItem{
					{EventID: testFirstEventID, Version: testVersion},
				},
			},
		},
	}

	tests := []struct {
		name             string
		versionHistories *persistence.VersionHistories
		activities       map[int64]*persistence.ActivityInfo
		blob             *persistence.DataBlob
		nextRunBlob      *persistence.DataBlob
		task             persistence.Task
		expectResult     *types.ReplicationTask
		expectErr        string
	}{
		{
			name:             "sync activity task - happy path",
			versionHistories: versionHistories,
			activities:       map[int64]*persistence.ActivityInfo{testScheduleID: &activityInfo},
			task: &persistence.SyncActivityTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
				TaskData: persistence.TaskData{
					TaskID:              testTaskID,
					Version:             testVersion,
					VisibilityTimestamp: time.Unix(0, testCreationTime),
				},
				ScheduledID: testScheduleID,
			},
			expectResult: &types.ReplicationTask{
				TaskType:     types.ReplicationTaskTypeSyncActivity.Ptr(),
				SourceTaskID: testTaskID,
				CreationTime: common.Int64Ptr(testCreationTime),
				SyncActivityTaskAttributes: &types.SyncActivityTaskAttributes{
					DomainID:           testDomainID,
					WorkflowID:         testWorkflowID,
					RunID:              testRunID,
					Version:            testVersion,
					ScheduledID:        testScheduleID,
					ScheduledTime:      common.Int64Ptr(testScheduleTime.UnixNano()),
					StartedID:          testStartedID,
					StartedTime:        common.Int64Ptr(testStartedTime.UnixNano()),
					LastHeartbeatTime:  common.Int64Ptr(testHeartbeatTime.UnixNano()),
					Details:            testDetails,
					Attempt:            testAttempt,
					LastFailureReason:  common.StringPtr(testLastFailureReason),
					LastWorkerIdentity: testWorkerIdentity,
					LastFailureDetails: testLastFailureDetails,
					VersionHistory: &types.VersionHistory{
						Items:       []*types.VersionHistoryItem{{EventID: testFirstEventID, Version: testVersion}},
						BranchToken: testBranchTokenVersionHistory,
					},
				},
			},
		},
		{
			name:             "sync activity task - missing activity info",
			versionHistories: versionHistories,
			activities:       map[int64]*persistence.ActivityInfo{},
			task: &persistence.SyncActivityTask{
				ScheduledID: testScheduleID,
			},
			expectResult: nil,
		},
		{
			name:             "history task - happy path",
			versionHistories: versionHistories,
			blob:             persistence.NewDataBlobFromInternal(testDataBlob),
			nextRunBlob:      persistence.NewDataBlobFromInternal(testDataBlobNewRun),
			task: &persistence.HistoryReplicationTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
				TaskData: persistence.TaskData{
					TaskID:              testTaskID,
					Version:             testVersion,
					VisibilityTimestamp: time.Unix(0, testCreationTime),
				},
				FirstEventID:      testFirstEventID,
				NextEventID:       testNextEventID,
				BranchToken:       testBranchToken,
				NewRunBranchToken: testBranchTokenNewRun,
			},
			expectResult: &types.ReplicationTask{
				TaskType:     types.ReplicationTaskTypeHistoryV2.Ptr(),
				SourceTaskID: testTaskID,
				CreationTime: common.Int64Ptr(testCreationTime),
				HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
					DomainID:            testDomainID,
					WorkflowID:          testWorkflowID,
					RunID:               testRunID,
					VersionHistoryItems: []*types.VersionHistoryItem{{EventID: testFirstEventID, Version: testVersion}},
					Events:              testDataBlob,
					NewRunEvents:        testDataBlobNewRun,
				},
			},
		},
		{
			name:             "history task - no next run",
			versionHistories: versionHistories,
			blob:             persistence.NewDataBlobFromInternal(testDataBlob),
			task: &persistence.HistoryReplicationTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
				TaskData: persistence.TaskData{
					TaskID:              testTaskID,
					Version:             testVersion,
					VisibilityTimestamp: time.Unix(0, testCreationTime),
				},
				FirstEventID: testFirstEventID,
				NextEventID:  testNextEventID,
				BranchToken:  testBranchToken,
			},
			expectResult: &types.ReplicationTask{
				TaskType:     types.ReplicationTaskTypeHistoryV2.Ptr(),
				SourceTaskID: testTaskID,
				CreationTime: common.Int64Ptr(testCreationTime),
				HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
					DomainID:            testDomainID,
					WorkflowID:          testWorkflowID,
					RunID:               testRunID,
					VersionHistoryItems: []*types.VersionHistoryItem{{EventID: testFirstEventID, Version: testVersion}},
					Events:              testDataBlob,
				},
			},
		},
		{
			name:             "history task - missing data blob",
			versionHistories: versionHistories,
			task: &persistence.HistoryReplicationTask{
				TaskData: persistence.TaskData{
					Version: testVersion,
				},
				FirstEventID: testFirstEventID,
				BranchToken:  testBranchToken,
			},
			expectErr: "history blob not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewImmediateTaskHydrator(true, tt.versionHistories, tt.activities, tt.blob, tt.nextRunBlob)
			result, err := h.Hydrate(context.Background(), tt.task)

			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectResult, result)
			}
		})
	}
}

type fakeMutableStateProvider struct {
	workflows map[definition.WorkflowIdentifier]mutableState
	released  bool
}

func (msp *fakeMutableStateProvider) GetMutableState(ctx context.Context, domainID, workflowID, runID string) (mutableState, execution.ReleaseFunc, error) {
	releaseFn := func(error) {
		msp.released = true
	}

	if msp.workflows == nil {
		return nil, releaseFn, errors.New("error loading mutable state")
	}

	ms, ok := msp.workflows[definition.NewWorkflowIdentifier(domainID, workflowID, runID)]
	if !ok {
		return nil, releaseFn, &types.EntityNotExistsError{}
	}
	return ms, releaseFn, nil
}

type fakeMutableState struct {
	isWorkflowExecutionRunning bool
	versionHistories           *persistence.VersionHistories
	activityInfos              map[int64]persistence.ActivityInfo
}

func (ms fakeMutableState) IsWorkflowExecutionRunning() bool {
	return ms.isWorkflowExecutionRunning
}
func (ms fakeMutableState) GetActivityInfo(scheduleID int64) (*persistence.ActivityInfo, bool) {
	ai, ok := ms.activityInfos[scheduleID]
	return &ai, ok
}
func (ms fakeMutableState) GetVersionHistories() *persistence.VersionHistories {
	return ms.versionHistories
}

type historyBlob struct {
	branch []byte
	blob   *types.DataBlob
}

type fakeHistoryProvider struct {
	blobs []historyBlob
}

func (h fakeHistoryProvider) GetEventBlob(ctx context.Context, task *persistence.HistoryReplicationTask) (*types.DataBlob, error) {
	return h.getBlob(task.BranchToken)
}
func (h fakeHistoryProvider) GetNextRunEventBlob(ctx context.Context, task *persistence.HistoryReplicationTask) (*types.DataBlob, error) {
	return h.getBlob(task.NewRunBranchToken)
}
func (h fakeHistoryProvider) getBlob(branch []byte) (*types.DataBlob, error) {
	for _, b := range h.blobs {
		if bytes.Equal(b.branch, branch) {
			return b.blob, nil
		}
	}
	return nil, errors.New("failed reading history")
}
