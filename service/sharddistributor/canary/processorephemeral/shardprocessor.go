package processorephemeral

import (
	"context"
	"math/rand"
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

	// We create a new shard every second. For each of them we have a chance of them to be done of 1/60 every second.
	// This means the average time to complete a shard is 60 seconds.
	// It also means we in average have 60 shards per instance running at any given time.
	stopInterval             = 1 * time.Second
	shardProcessorDoneChance = 60
)

// NewShardProcessor creates a new ShardProcessor.
func NewShardProcessor(shardID string, timeSource clock.TimeSource, logger *zap.Logger) *ShardProcessor {
	p := &ShardProcessor{
		shardID:    shardID,
		timeSource: timeSource,
		logger:     logger,
		stopChan:   make(chan struct{}),
	}
	p.SetShardStatus(types.ShardStatusREADY)
	return p
}

// ShardProcessor is a processor for a shard.
type ShardProcessor struct {
	shardID      string
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
		ShardLoad: 1.0,                                // We return 1.0 for all shards for now.
		Status:    types.ShardStatus(p.status.Load()), // Report the status of the shard
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

	stopTicker := p.timeSource.NewTicker(stopInterval)
	defer stopTicker.Stop()

	for {
		select {
		case <-p.stopChan:
			p.logger.Info("Stopping shard processor", zap.String("shardID", p.shardID), zap.Int("steps", p.processSteps), zap.String("status", types.ShardStatus(p.status.Load()).String()))
			return
		case <-ticker.Chan():
			p.logger.Info("Processing shard", zap.String("shardID", p.shardID), zap.Int("steps", p.processSteps), zap.String("status", types.ShardStatus(p.status.Load()).String()))
		case <-stopTicker.Chan():
			p.processSteps++
			if rand.Intn(shardProcessorDoneChance) == 0 {
				p.logger.Info("Setting shard processor to done", zap.String("shardID", p.shardID), zap.Int("steps", p.processSteps), zap.String("status", types.ShardStatus(p.status.Load()).String()))
				p.SetShardStatus(types.ShardStatusDONE)
			}
		}
	}
}
