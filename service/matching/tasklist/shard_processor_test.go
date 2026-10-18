package tasklist

import (
	"testing"
	"time"

	smtypes "github.com/cadence-workflow/shard-manager/common/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/persistence"
)

func mustNewIdentifier(domainID, name string, taskType int) *Identifier {
	id, err := NewIdentifier(domainID, name, taskType)
	if err != nil {
		panic(err)
	}
	return id
}

var testIdentifier = mustNewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)

type shardProcessorTestData struct {
	mockRegistry   *MockTaskListRegistry
	shardProcessor ShardProcessor
}

func newShardProcessorTestData(t *testing.T, taskListID *Identifier) shardProcessorTestData {
	ctrl := gomock.NewController(t)

	mockRegistry := NewMockTaskListRegistry(ctrl)
	mockRegistry.EXPECT().ManagersByTaskListName(taskListID.GetName()).Return([]Manager{}).AnyTimes()

	params := ShardProcessorParams{
		ShardID:             taskListID.GetName(),
		TaskListsRegistry:   mockRegistry,
		ReportTTL:           1 * time.Millisecond,
		TimeSource:          clock.NewRealTimeSource(),
		ShardIsTaskListName: true,
	}

	shardProcessor, err := NewShardProcessor(params)
	require.NoError(t, err)
	return shardProcessorTestData{
		mockRegistry:   mockRegistry,
		shardProcessor: shardProcessor,
	}
}

func TestNewShardProcessorFailsWithEmptyParams(t *testing.T) {
	params := ShardProcessorParams{}
	sp, err := NewShardProcessor(params)
	require.Nil(t, sp)
	require.Error(t, err)
}

func TestGetShardReport(t *testing.T) {
	td := newShardProcessorTestData(t, testIdentifier)

	shardReport := td.shardProcessor.GetShardReport()
	require.NotNil(t, shardReport)
	require.Equal(t, float64(0), shardReport.ShardLoad)
	require.Equal(t, smtypes.ShardStatusREADY, shardReport.Status)
}

func TestSetShardStatus(t *testing.T) {
	tests := []struct {
		name                string
		shardIsTaskListName bool
		status              smtypes.ShardStatus
		wantStatus          smtypes.ShardStatus
	}{
		{
			name:                "task list name shard becomes ready",
			shardIsTaskListName: true,
			status:              smtypes.ShardStatusREADY,
			wantStatus:          smtypes.ShardStatusREADY,
		},
		{
			name:                "task list name shard becomes done",
			shardIsTaskListName: true,
			status:              smtypes.ShardStatusDONE,
			wantStatus:          smtypes.ShardStatusDONE,
		},
		{
			name:                "shard by hash is never done",
			shardIsTaskListName: false,
			status:              smtypes.ShardStatusDONE,
			wantStatus:          smtypes.ShardStatusREADY,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer goleak.VerifyNone(t)
			ctrl := gomock.NewController(t)
			mockRegistry := NewMockTaskListRegistry(ctrl)
			mockRegistry.EXPECT().ManagersByTaskListName(gomock.Any()).Return([]Manager{}).AnyTimes()
			mockRegistry.EXPECT().ManagersByShard(gomock.Any()).Return([]Manager{}).AnyTimes()
			shardProcessor, err := NewShardProcessor(ShardProcessorParams{
				ShardID:             "shard-id",
				ShardIsTaskListName: tt.shardIsTaskListName,
				TaskListsRegistry:   mockRegistry,
				ReportTTL:           time.Minute,
				TimeSource:          clock.NewMockedTimeSource(),
			})
			require.NoError(t, err)

			shardProcessor.SetShardStatus(tt.status)

			shardReport := shardProcessor.GetShardReport()
			require.Equal(t, tt.wantStatus, shardReport.Status)
		})
	}
}

func TestShardProcessor_ShardByHashSkipsTaskListsOwnedByHashRing(t *testing.T) {
	ctrl := gomock.NewController(t)
	shardID := "58"
	reportTTL := time.Minute
	timeSource := clock.NewMockedTimeSource()

	firstTaskListID := mustNewIdentifier("domain-id", "first-task-list", persistence.TaskListTypeDecision)
	firstTaskListQPS := 2.0
	firstTaskList := NewMockManager(ctrl)
	firstTaskList.EXPECT().TaskListID().Return(firstTaskListID).AnyTimes()
	firstTaskList.EXPECT().QueriesPerSecond().Return(firstTaskListQPS)
	firstTaskList.EXPECT().Stop()

	secondTaskListID := mustNewIdentifier("domain-id", "second-task-list", persistence.TaskListTypeActivity)
	secondTaskListQPS := 3.0
	secondTaskList := NewMockManager(ctrl)
	secondTaskList.EXPECT().TaskListID().Return(secondTaskListID).AnyTimes()
	secondTaskList.EXPECT().QueriesPerSecond().Return(secondTaskListQPS)
	secondTaskList.EXPECT().Stop()

	// The shard must not stop a task list the hash ring owns, or count its load.
	hashRingTaskListID := mustNewIdentifier("domain-id", "hash-ring-task-list", persistence.TaskListTypeDecision)
	hashRingTaskList := NewMockManager(ctrl)
	hashRingTaskList.EXPECT().TaskListID().Return(hashRingTaskListID).AnyTimes()

	mockRegistry := NewMockTaskListRegistry(ctrl)
	mockRegistry.EXPECT().ManagersByShard(shardID).Return([]Manager{firstTaskList, secondTaskList, hashRingTaskList}).Times(2)

	shardProcessor, err := NewShardProcessor(ShardProcessorParams{
		ShardID:           shardID,
		TaskListsRegistry: mockRegistry,
		ReportTTL:         reportTTL,
		TimeSource:        timeSource,
		OwnedByHashRing: func(taskListName string) bool {
			return taskListName == hashRingTaskListID.GetName()
		},
	})
	require.NoError(t, err)

	timeSource.Advance(reportTTL + time.Second)
	shardReport := shardProcessor.GetShardReport()
	require.Equal(t, firstTaskListQPS+secondTaskListQPS, shardReport.ShardLoad)

	shardProcessor.Stop()
}
