package processorephemeral

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/canary/testhelper"
)

func TestNewShardProcessor(t *testing.T) {
	shardID := "test-shard-123"
	timeSource := clock.NewRealTimeSource()
	logger := zaptest.NewLogger(t)

	processor := NewShardProcessor(shardID, timeSource, logger, tally.NoopScope)

	require.NotNil(t, processor)
	assert.Equal(t, shardID, processor.shardID)
	assert.Equal(t, timeSource, processor.timeSource)
	assert.Equal(t, logger, processor.logger)
	assert.NotNil(t, processor.stopChan)
}

func TestShardProcessor_GetShardReport(t *testing.T) {
	processor := NewShardProcessor("test-shard", clock.NewRealTimeSource(), zaptest.NewLogger(t), tally.NoopScope)

	report := processor.GetShardReport()
	// the simple implementation just returns 1.0 for load and READY status
	assert.Equal(t, 1.0, report.ShardLoad)
	assert.Equal(t, types.ShardStatusREADY, report.Status)
}

func TestShardProcessor_Start_Process_Stop(t *testing.T) {
	// Verify that after stopping the processor, there are no goroutines left
	goleak.VerifyNone(t)

	logger := zaptest.NewLogger(t)
	clock := clock.NewMockedTimeSource()
	processor := NewShardProcessor("test-shard", clock, logger, tally.NoopScope)

	ctx := context.Background()
	processor.Start(ctx)

	// The processor will block on time
	clock.BlockUntil(1)

	// Let time pass until the ticker is triggered
	clock.Advance(processInterval + 1*time.Second)

	// Sleep for a bit to schedule the go-routine
	time.Sleep(10 * time.Millisecond)

	// Stop the processor
	processor.Stop()

	// Assert that the processor has processed at least once
	assert.Greater(t, processor.processSteps, 0)
}

// TestShardProcessor_InjectsLifecycleDelay verifies the ephemeral processor
// also sleeps and emits the lifecycle_injected counter for non-normal kinds.
func TestShardProcessor_InjectsLifecycleDelay(t *testing.T) {
	testhelper.AssertInjectsLifecycleDelay(t, func(id string, ts clock.TimeSource, logger *zap.Logger, scope tally.Scope) testhelper.Lifecycler {
		return NewShardProcessor(id, ts, logger, scope)
	})
}
