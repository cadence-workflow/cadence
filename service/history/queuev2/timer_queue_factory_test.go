package queuev2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/worker/archiver"
)

func TestTimerQueueFactory_CreateQueuev2(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		wantCached bool
	}{
		{name: "enabled", mode: "enabled", wantCached: true},
		{name: "shadow", mode: "shadow", wantCached: true},
		{name: "disabled", mode: "disabled", wantCached: false},
		{name: "unknown value", mode: "some-unknown-value", wantCached: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer goleak.VerifyNone(t)
			ctrl := gomock.NewController(t)

			cfg := config.NewForTest()
			if tc.mode != "" {
				cfg.TimerProcessorCachedQueueReaderMode = dynamicproperties.GetStringPropertyFn(tc.mode)
			}

			mockShard := shard.NewTestContext(t, ctrl, &persistence.ShardInfo{
				ShardID:          10,
				RangeID:          1,
				TransferAckLevel: 0,
			}, cfg)

			factory := &timerQueueFactory{
				taskProcessor:  task.NewMockProcessor(ctrl),
				archivalClient: archiver.NewMockClient(ctrl),
			}

			processor := factory.createQueuev2(mockShard, execution.NewMockCache(ctrl), invariant.NewMockInvariant(ctrl))

			assert.NotNil(t, processor)
			_, isCached := processor.(*cachedScheduledQueue)
			assert.Equal(t, tc.wantCached, isCached, "mode=%q", tc.mode)
		})
	}
}

func TestTimerQueueFactory_Category(t *testing.T) {
	factory := &timerQueueFactory{}

	category := factory.Category()

	assert.Equal(t, persistence.HistoryTaskCategoryTimer, category)
}

func TestTimerQueueFactory_IsQueueV2Enabled(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	mockShard := shard.NewTestContext(
		t, ctrl, &persistence.ShardInfo{
			ShardID:          10,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest())

	factory := &timerQueueFactory{}

	enabled := factory.isQueueV2Enabled(mockShard)
	assert.False(t, enabled)
}
