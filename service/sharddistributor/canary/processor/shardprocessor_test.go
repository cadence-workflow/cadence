package processor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
)

func TestNewShardProcessor(t *testing.T) {
	shardID := "test-shard-123"
	timeSource := clock.NewRealTimeSource()
	logger := zaptest.NewLogger(t)

	processor := NewShardProcessor(shardID, timeSource, logger)

	require.NotNil(t, processor)
	assert.Equal(t, shardID, processor.shardID)
	assert.Equal(t, timeSource, processor.timeSource)
	assert.Equal(t, logger, processor.logger)
	assert.NotNil(t, processor.stopChan)
}

func TestShardProcessor_GetShardReport(t *testing.T) {
	processor := NewShardProcessor("test-shard", clock.NewRealTimeSource(), zaptest.NewLogger(t))

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
	processor := NewShardProcessor("test-shard", clock, logger)

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

func Test_shardLoadFromID(t *testing.T) {
	tests := []struct {
		shardID string
		want    float64
	}{
		{"0", 1.0},
		{"shard-1", 1.0},
		{"42", 42.0},
		{"999", 999.0},
		{"", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.shardID, func(t *testing.T) {
			got := shardLoadFromID(tt.shardID)
			assert.Equal(t, tt.want, got)
		})
	}
}
