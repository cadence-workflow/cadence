package processor

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
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
	p := &ShardProcessor{
		shardID:    shardID,
		shardLoad:  shardLoadFromID(shardID),
		timeSource: timeSource,
		logger:     logger,
		stopChan:   make(chan struct{}),
	}
	p.status.Store(int32(types.ShardStatusREADY))
	return p
}

// ShardProcessor is a processor for a shard.
type ShardProcessor struct {
	shardID      string
	shardLoad    float64
	timeSource   clock.TimeSource
	logger       *zap.Logger
	stopChan     chan struct{}
	goRoutineWg  sync.WaitGroup
	processSteps int

	status atomic.Int32
}

var _ executorclient.ShardProcessor = (*ShardProcessor)(nil)

// GetShardReport implements executorclient.ShardProcessor.
func (p *ShardProcessor) GetShardReport() executorclient.ShardReport {
	return executorclient.ShardReport{
		ShardLoad: p.shardLoad,                        // We return a load from shardID
		Status:    types.ShardStatus(p.status.Load()), // Report the shard as ready since it's actively processing
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

func (p *ShardProcessor) SetShardStatus(status types.ShardStatus) {
	p.status.Store(int32(status))
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

// shardLoadFromID returns a shard load based on the shard ID.
// If the shard ID is not a valid integer, it returns 1.0.
func shardLoadFromID(shardID string) float64 {
	if parsed, err := strconv.Atoi(shardID); err == nil && parsed > 0 {
		return float64(parsed)
	}
	return 1.0
}
