package factory

import (
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

// ShardProcessorFactory is a generic factory for creating ShardProcessor instances.
type ShardProcessorFactory[T executorclient.ShardProcessor] struct {
	logger      *zap.Logger
	timeSource  clock.TimeSource
	constructor func(shardID string, timeSource clock.TimeSource, logger *zap.Logger) T
}

// NewShardProcessor creates a new ShardProcessor.
func (s *ShardProcessorFactory[T]) NewShardProcessor(shardID string) (T, error) {
	return s.constructor(shardID, s.timeSource, s.logger), nil
}

// Params are the parameters for creating a ShardProcessorFactory.
type Params struct {
	fx.In

	Logger     *zap.Logger
	TimeSource clock.TimeSource
}

// NewShardProcessorFactory creates a new ShardProcessorFactory with a constructor function.
func NewShardProcessorFactory[T executorclient.ShardProcessor](
	params Params,
	constructor func(shardID string, timeSource clock.TimeSource, logger *zap.Logger) T,
) executorclient.ShardProcessorFactory[T] {
	return &ShardProcessorFactory[T]{
		logger:      params.Logger,
		timeSource:  params.TimeSource,
		constructor: constructor,
	}
}
