package processorephemeral

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber-go/tally"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/canary/latencykind"
	canarymetrics "github.com/uber/cadence/service/sharddistributor/canary/metrics"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

// This is a small shard processor, the only thing it currently does it
// count the number of steps it has processed and log that information.
const (
	// We create a new shard every second. For each of them we have a chance of them to be done of 1/60 every second.
	// This means the average time to complete a shard is 60 seconds.
	// It also means we in average have 60 shards per instance running at any given time.
	processInterval          = 1 * time.Second
	shardProcessorDoneChance = 60
)

// NewShardProcessor creates a new ShardProcessor.
func NewShardProcessor(shardID string, timeSource clock.TimeSource, logger *zap.Logger, metricsScope tally.Scope) *ShardProcessor {
	kind := latencykind.ShardIDToKind(shardID)
	scope := metricsScope.Tagged(map[string]string{"latency_kind": kind.String()})
	p := &ShardProcessor{
		shardID:      shardID,
		kind:         kind,
		timeSource:   timeSource,
		logger:       logger,
		metricsScope: scope,
		stopChan:     make(chan struct{}),
	}
	p.SetShardStatus(types.ShardStatusREADY)
	return p
}

// ShardProcessor is a processor for a shard.
type ShardProcessor struct {
	shardID      string
	kind         latencykind.Kind
	timeSource   clock.TimeSource
	logger       *zap.Logger
	metricsScope tally.Scope
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
	start := p.timeSource.Now()
	defer func() {
		p.metricsScope.Histogram(canarymetrics.CanaryShardStartLatency, canarymetrics.CanaryPingLatencyBuckets).
			RecordDuration(p.timeSource.Since(start))
	}()

	if delay := p.kind.StartDelay(); delay > 0 {
		p.metricsScope.Tagged(map[string]string{"lifecycle": "start"}).
			Counter(canarymetrics.CanaryShardLifecycleInjected).Inc(1)
		p.timeSource.Sleep(delay)
	}

	p.metricsScope.Counter(canarymetrics.CanaryShardStarted).Inc(1)
	p.logger.Debug("Starting shard processor", zap.String("shardID", p.shardID))
	p.goRoutineWg.Add(1)
	go p.process()
	return nil
}

// Stop implements executorclient.ShardProcessor.
func (p *ShardProcessor) Stop() {
	start := p.timeSource.Now()
	defer func() {
		p.metricsScope.Histogram(canarymetrics.CanaryShardStopLatency, canarymetrics.CanaryPingLatencyBuckets).
			RecordDuration(p.timeSource.Since(start))
	}()

	if delay := p.kind.StopDelay(); delay > 0 {
		p.metricsScope.Tagged(map[string]string{"lifecycle": "stop"}).
			Counter(canarymetrics.CanaryShardLifecycleInjected).Inc(1)
		p.timeSource.Sleep(delay)
	}

	p.metricsScope.Counter(canarymetrics.CanaryShardStopped).Inc(1)
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
			p.logger.Debug("Stopping shard processor", zap.String("shardID", p.shardID), zap.Int("steps", p.processSteps), zap.String("status", types.ShardStatus(p.status.Load()).String()))
			return
		case <-ticker.Chan():
			p.processSteps++
			p.metricsScope.Counter(canarymetrics.CanaryShardProcessStep).Inc(1)
			if rand.Intn(shardProcessorDoneChance) == 0 {
				p.logger.Debug("Setting shard processor to done", zap.String("shardID", p.shardID), zap.Int("steps", p.processSteps), zap.String("status", types.ShardStatus(p.status.Load()).String()))
				p.metricsScope.Counter(canarymetrics.CanaryShardDone).Inc(1)
				p.SetShardStatus(types.ShardStatusDONE)
			}
		}
	}
}
