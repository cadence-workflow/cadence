package queuev2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/worker/archiver"
)

func TestVisibilityQueueFactory_Category(t *testing.T) {
	ctrl := gomock.NewController(t)
	factory := NewVisibilityQueueFactory(task.NewMockProcessor(ctrl), archiver.NewMockClient(ctrl))

	assert.Equal(t, persistence.HistoryTaskCategoryVisibility, factory.Category())
}

func TestVisibilityQueueFactory_CreateQueue(t *testing.T) {
	tests := map[string]struct {
		enabled   bool
		wantQueue bool
	}{
		"enabled for the shard": {
			enabled:   true,
			wantQueue: true,
		},
		"disabled for the shard": {
			enabled:   false,
			wantQueue: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			defer goleak.VerifyNone(t)
			ctrl := gomock.NewController(t)

			cfg := config.NewForTest()
			cfg.EnableVisibilityQueue = func(int) bool { return tc.enabled }
			mockShard := shard.NewTestContext(t, ctrl, &persistence.ShardInfo{
				ShardID: 10,
				RangeID: 1,
			}, cfg)

			factory := NewVisibilityQueueFactory(task.NewMockProcessor(ctrl), archiver.NewMockClient(ctrl))
			processor := factory.CreateQueue(mockShard, execution.NewMockCache(ctrl), invariant.NewMockInvariant(ctrl))

			if !tc.wantQueue {
				assert.Nil(t, processor)
				return
			}
			immediateQueue, ok := processor.(*immediateQueue)
			if assert.True(t, ok, "expected an immediate queue, got %T", processor) {
				assert.Equal(t, persistence.HistoryTaskCategoryVisibility, immediateQueue.base.category)
			}
		})
	}
}
