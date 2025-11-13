package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

func TestNewShardProcessorFactory(t *testing.T) {
	logger := zaptest.NewLogger(t)
	timeSource := clock.NewRealTimeSource()

	params := Params{
		Logger:     logger,
		TimeSource: timeSource,
	}

	factory := NewShardProcessorFactory(params, processor.NewShardProcessor)

	// Test that the factory implements the correct interface
	var _ executorclient.ShardProcessorFactory[*processor.ShardProcessor] = factory

	// Test creating a processor
	processor, err := factory.NewShardProcessor("test-shard-1")
	require.NoError(t, err)
	assert.NotNil(t, processor)
}

func TestShardProcessorFactory_NewShardProcessor(t *testing.T) {
	logger := zaptest.NewLogger(t)
	timeSource := clock.NewRealTimeSource()

	factory := &ShardProcessorFactory[*processor.ShardProcessor]{
		logger:      logger,
		timeSource:  timeSource,
		constructor: processor.NewShardProcessor,
	}

	// Test creating multiple processors
	processor1, err := factory.NewShardProcessor("shard-1")
	require.NoError(t, err)

	processor2, err := factory.NewShardProcessor("shard-2")
	require.NoError(t, err)

	// Ensure they are different instances
	assert.NotEqual(t, processor1, processor2)
}
