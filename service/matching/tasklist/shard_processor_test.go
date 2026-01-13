package tasklist

import (
	"go.uber.org/mock/gomock"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func paramsForTaskListManager(taskListID *Identifier) ShardProcessorParams {
	var mutex sync.RWMutex
	taskList := make(map[Identifier]Manager)
	params := ShardProcessorParams{
		ShardID:       taskListID.GetName(),
		TaskListsLock: &mutex,
		TaskLists:     taskList,
		ReportTTL:     1 * time.Millisecond,
		TimeSource:    clock.NewRealTimeSource(),
	}
	return params
}

func paramsForTaskListManagerWithStopCallback(t *testing.T, taskListID *Identifier) ShardProcessorParams {
	params := paramsForTaskListManager(taskListID)
	mockCtrl := gomock.NewController(t)
	mockManager := NewMockManager(mockCtrl)
	params.TaskLists[*taskListID] = mockManager
	mockManager.EXPECT().TaskListID().Return(
		taskListID).Times(2)
	mockManager.EXPECT().Stop().Do(
		func() {
			delete(params.TaskLists, *taskListID)
		},
	)
	return params
}

func TestNewShardProcessor(t *testing.T) {
	t.Run("NewShardProcessor fails with empty params", func(t *testing.T) {
		params := ShardProcessorParams{}
		sp, err := NewShardProcessor(params)
		require.Nil(t, sp)
		require.Error(t, err)
	})

	t.Run("NewShardProcessor success", func(t *testing.T) {
		tlID, err := NewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)
		require.NoError(t, err)
		params := paramsForTaskListManager(tlID)
		sp, err := NewShardProcessor(params)
		require.NoError(t, err)
		require.NotNil(t, sp)
	})
}

func TestStop(t *testing.T) {
	t.Run("Stop ShardProcessor", func(t *testing.T) {
		tlID, err := NewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)
		require.NoError(t, err)
		params := paramsForTaskListManagerWithStopCallback(t, tlID)

		sp, err := NewShardProcessor(params)
		require.NoError(t, err)
		params.TaskListsLock.RLock()
		require.Equal(t, 1, len(params.TaskLists))
		params.TaskListsLock.RUnlock()

		sp.Stop()
		params.TaskListsLock.RLock()
		require.Equal(t, 0, len(params.TaskLists))
		params.TaskListsLock.RUnlock()
	})
}

func TestGetShardReport(t *testing.T) {
	t.Run("GetShardReport success", func(t *testing.T) {
		tlID, err := NewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)
		require.NoError(t, err)
		params := paramsForTaskListManager(tlID)
		sp, err := NewShardProcessor(params)
		require.NoError(t, err)
		shardReport := sp.GetShardReport()
		require.NotNil(t, shardReport)
		require.Equal(t, float64(0), shardReport.ShardLoad)
		require.Equal(t, types.ShardStatusREADY, shardReport.Status)
	})
}

func TestSetShardStatus(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("SetShardStatus success", func(t *testing.T) {
		tlID, err := NewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)
		require.NoError(t, err)
		params := paramsForTaskListManager(tlID)
		sp, err := NewShardProcessor(params)
		require.NoError(t, err)
		sp.SetShardStatus(types.ShardStatusREADY)
		shardReport := sp.GetShardReport()
		require.NotNil(t, shardReport)
		require.Equal(t, float64(0), shardReport.ShardLoad)
		require.Equal(t, types.ShardStatusREADY, shardReport.Status)
	})
}
