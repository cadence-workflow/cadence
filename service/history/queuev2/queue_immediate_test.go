package queuev2

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

func TestImmediateQueue_LifeCycle(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewMockContext(ctrl)
	mockTaskProcessor := task.NewMockProcessor(ctrl)
	mockTaskExecutor := task.NewMockExecutor(ctrl)
	mockLogger := testlogger.New(t)
	mockMetricsClient := metrics.NoopClient
	mockMetricsScope := metrics.NoopScope
	mockTimeSource := clock.NewMockedTimeSource()
	mockExecutionManager := persistence.NewMockExecutionManager(ctrl)

	// Setup mock expectations
	mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata).AnyTimes()
	mockShard.EXPECT().GetTimeSource().Return(mockTimeSource).AnyTimes()
	mockShard.EXPECT().GetQueueState(persistence.HistoryTaskCategoryTransfer).Return(&types.QueueState{
		VirtualQueueStates: map[int64]*types.VirtualQueueState{
			rootQueueID: {
				VirtualSliceStates: []*types.VirtualSliceState{
					{
						TaskRange: &types.TaskRange{
							InclusiveMin: &types.TaskKey{
								TaskID: 1,
							},
							ExclusiveMax: &types.TaskKey{
								TaskID: 10,
							},
						},
					},
				},
			},
		},
		ExclusiveMaxReadLevel: &types.TaskKey{
			TaskID: 10,
		},
	}, nil).AnyTimes()
	mockShard.EXPECT().GetExecutionManager().Return(mockExecutionManager).AnyTimes()
	mockExecutionManager.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetHistoryTasksResponse{}, nil).AnyTimes()
	mockExecutionManager.EXPECT().RangeCompleteHistoryTask(gomock.Any(), gomock.Any()).Return(&persistence.RangeCompleteHistoryTaskResponse{}, nil).AnyTimes()
	mockShard.EXPECT().UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryTransfer, cluster.TestCurrentClusterName).Return(persistence.NewImmediateTaskKey(10)).AnyTimes()
	mockShard.EXPECT().UpdateQueueState(persistence.HistoryTaskCategoryTransfer, gomock.Any()).Return(nil).AnyTimes()

	options := &Options{
		DeleteBatchSize:                    dynamicproperties.GetIntPropertyFn(100),
		RedispatchInterval:                 dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		PageSize:                           dynamicproperties.GetIntPropertyFn(100),
		PollBackoffInterval:                dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		MaxPollInterval:                    dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		MaxPollIntervalJitterCoefficient:   dynamicproperties.GetFloatPropertyFn(0.1),
		UpdateAckInterval:                  dynamicproperties.GetDurationPropertyFn(time.Second * 10),
		UpdateAckIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.1),
	}

	queue := NewImmediateQueue(
		mockShard,
		persistence.HistoryTaskCategoryTransfer,
		mockTaskProcessor,
		mockTaskExecutor,
		mockLogger,
		mockMetricsClient,
		mockMetricsScope,
		options,
	).(*immediateQueue)

	// Test Start
	queue.Start()
	assert.Equal(t, common.DaemonStatusStarted, atomic.LoadInt32(&queue.status))

	// Test NotifyNewTask
	queue.NotifyNewTask("test-cluster", &hcommon.NotifyTaskInfo{
		Tasks: []persistence.Task{
			&persistence.DecisionTask{},
		},
	})

	// Advance time to trigger poll
	mockTimeSource.Advance(options.MaxPollInterval() * 2)

	// Advance time to trigger update ack
	mockTimeSource.Advance(options.UpdateAckInterval() * 2)

	// Test Stop
	queue.Stop()
	assert.Equal(t, common.DaemonStatusStopped, atomic.LoadInt32(&queue.status))
}
