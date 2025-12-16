package processor

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

// This is a small shard processor, the only thing it currently does it
// count the number of steps it has processed and log that information.
const (
	processInterval = 10 * time.Second
)

// NewShardProcessor creates a new ShardProcessor.
func NewShardProcessor(shardID string, timeSource clock.TimeSource, logger *zap.Logger) *ShardProcessor {
	return &ShardProcessor{
		shardID:    shardID,
		timeSource: timeSource,
		logger:     logger,
		stopChan:   make(chan struct{}),
	}
}

// ShardProcessor is a processor for a shard.
type ShardProcessor struct {
	shardID      string
	timeSource   clock.TimeSource
	logger       *zap.Logger
	stopChan     chan struct{}
	goRoutineWg  sync.WaitGroup
	processSteps int
}

var _ executorclient.ShardProcessor = (*ShardProcessor)(nil)

// GetShardReport implements executorclient.ShardProcessor.
func (p *ShardProcessor) GetShardReport() executorclient.ShardReport {
	return executorclient.ShardReport{
		ShardLoad: 1.0,                    // We return 1.0 for all shards for now.
		Status:    types.ShardStatusREADY, // Report the shard as ready since it's actively processing
	}
}

// Start implements executorclient.ShardProcessor.
func (p *ShardProcessor) Start(_ context.Context) error {
	p.logger.Info("Starting shard processor", zap.String("shardID", p.shardID))
	p.goRoutineWg.Add(1)
	go p.process()
	return nil
}

// Stop implements executorclient.ShardProcessor.
func (p *ShardProcessor) Stop() {
	p.logger.Info("Stopping shard processor", zap.String("shardID", p.shardID))
	close(p.stopChan)
	p.goRoutineWg.Wait()
}

func (p *ShardProcessor) process() {
	defer p.goRoutineWg.Done()

	ticker := p.timeSource.NewTicker(processInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			p.logger.Info("Stopping shard processor", zap.String("shardID", p.shardID), zap.Int("steps", p.processSteps))
			return
		case <-ticker.Chan():
			p.processSteps++
			p.logger.Info("Processing shard", zap.String("shardID", p.shardID), zap.Int("steps", p.processSteps))
		}
	}
}
