package executorclient

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber-go/tally"

	"github.com/uber/cadence/client/sharddistributorexecutor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/executorclient/metricsconstants"
	"github.com/uber/cadence/service/sharddistributor/executorclient/syncgeneric"
)

type processorState int32

const (
	processorStateStarting processorState = iota
	processorStateStarted
	processorStateStopping
)

type managedProcessor[SP ShardProcessor] struct {
	processor SP
	state     atomic.Int32
}

func (mp *managedProcessor[SP]) setState(state processorState) {
	mp.state.Store(int32(state))
}

func (mp *managedProcessor[SP]) getState() processorState {
	return processorState(mp.state.Load())
}

func newManagedProcessor[SP ShardProcessor](processor SP, state processorState) *managedProcessor[SP] {
	managed := &managedProcessor[SP]{
		processor: processor,
		state:     atomic.Int32{},
	}

	managed.setState(state)
	return managed
}

type executorImpl[SP ShardProcessor] struct {
	logger                 log.Logger
	shardDistributorClient sharddistributorexecutor.Client
	shardProcessorFactory  ShardProcessorFactory[SP]
	namespace              string
	stopC                  chan struct{}
	heartBeatInterval      time.Duration
	managedProcessors      syncgeneric.Map[string, *managedProcessor[SP]]
	executorID             string
	timeSource             clock.TimeSource
	processLoopWG          sync.WaitGroup
	assignmentMutex        sync.Mutex
	metrics                tally.Scope
}

func (e *executorImpl[SP]) Start(ctx context.Context) {
	e.logger.Info("starting shard distributor executor", tag.ShardNamespace(e.namespace))
	e.processLoopWG.Add(1)
	go func() {
		defer e.processLoopWG.Done()
		e.heartbeatloop(context.WithoutCancel(ctx))
	}()
}

func (e *executorImpl[SP]) Stop() {
	e.logger.Info("stopping shard distributor executor", tag.ShardNamespace(e.namespace))
	close(e.stopC)
	e.processLoopWG.Wait()
}

func (e *executorImpl[SP]) GetShardProcess(shardID string) (SP, error) {
	shardProcess, ok := e.managedProcessors.Load(shardID)
	if !ok {
		var zero SP
		return zero, fmt.Errorf("shard process not found for shard ID: %s", shardID)
	}
	return shardProcess.processor, nil
}

func (e *executorImpl[SP]) heartbeatloop(ctx context.Context) {
	heartBeatTicker := e.timeSource.NewTicker(e.heartBeatInterval)
	defer heartBeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("shard distributorexecutor context done, stopping")
			e.stopShardProcessors()
			return
		case <-e.stopC:
			e.logger.Info("shard distributorexecutor stopped")
			e.stopShardProcessors()
			return
		case <-heartBeatTicker.Chan():
			shardAssignment, err := e.heartbeat(ctx)
			if err != nil {
				e.logger.Error("failed to heartbeat", tag.Error(err))
				continue // TODO: should we stop the executor, and drop all the shards?
			}
			if !e.assignmentMutex.TryLock() {
				e.logger.Warn("already doing shard assignment, will skip this assignment")
				e.metrics.Counter(metricsconstants.ShardDistributorExecutorAssignmentSkipped).Inc(1)
				continue
			}
			go func() {
				defer e.assignmentMutex.Unlock()

				startTime := e.timeSource.Now()
				defer e.metrics.
					Histogram(metricsconstants.ShardDistributorExecutorAssignLoopLatency, metricsconstants.ShardDistributorExecutorAssignLoopLatencyBuckets).
					RecordDuration(e.timeSource.Since(startTime))

				e.updateShardAssignment(ctx, shardAssignment)
			}()
		}
	}
}

func (e *executorImpl[SP]) heartbeat(ctx context.Context) (shardAssignments map[string]*types.ShardAssignment, err error) {
	// Fill in the shard status reports
	shardStatusReports := make(map[string]*types.ShardStatusReport)
	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		if managedProcessor.getState() == processorStateStarted {
			shardStatus := managedProcessor.processor.GetShardReport()

			shardStatusReports[shardID] = &types.ShardStatusReport{
				ShardLoad: shardStatus.ShardLoad,
				Status:    shardStatus.Status,
			}
		}
		return true
	})

	e.metrics.Gauge(metricsconstants.ShardDistributorExecutorOwnedShards).Update(float64(len(shardStatusReports)))

	// Create the request
	request := &types.ExecutorHeartbeatRequest{
		Namespace:          e.namespace,
		ExecutorID:         e.executorID,
		Status:             types.ExecutorStatusACTIVE,
		ShardStatusReports: shardStatusReports,
	}

	// Send the request
	response, err := e.shardDistributorClient.Heartbeat(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("send heartbeat: %w", err)
	}

	return response.ShardAssignments, nil
}

func (e *executorImpl[SP]) updateShardAssignment(ctx context.Context, shardAssignments map[string]*types.ShardAssignment) {
	wg := sync.WaitGroup{}

	// Stop shard processing for shards not assigned to this executor
	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		if assignment, ok := shardAssignments[shardID]; !ok || assignment.Status != types.AssignmentStatusREADY {
			e.metrics.Counter(metricsconstants.ShardDistributorExecutorShardsStopped).Inc(1)

			wg.Add(1)
			go func() {
				defer wg.Done()
				managedProcessor.setState(processorStateStopping)
				managedProcessor.processor.Stop()
				e.managedProcessors.Delete(shardID)
			}()
		}
		return true
	})

	// Start shard processing for shards assigned to this executor
	for shardID, assignment := range shardAssignments {
		if assignment.Status == types.AssignmentStatusREADY {
			if _, ok := e.managedProcessors.Load(shardID); !ok {
				e.metrics.Counter(metricsconstants.ShardDistributorExecutorShardsStarted).Inc(1)

				wg.Add(1)
				go func() {
					defer wg.Done()
					processor, err := e.shardProcessorFactory.NewShardProcessor(shardID)
					if err != nil {
						e.logger.Error("failed to create shard processor", tag.Error(err))
						e.metrics.Counter(metricsconstants.ShardDistributorExecutorProcessorCreationFailures).Inc(1)
						return
					}
					managedProcessor := newManagedProcessor(processor, processorStateStarting)
					e.managedProcessors.Store(shardID, managedProcessor)

					processor.Start(ctx)

					managedProcessor.setState(processorStateStarted)
				}()
			}
		}
	}

	wg.Wait()
}

func (e *executorImpl[SP]) stopShardProcessors() {
	wg := sync.WaitGroup{}

	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		// If the processor is already stopping, skip it
		if managedProcessor.getState() == processorStateStopping {
			return true
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			managedProcessor.setState(processorStateStopping)
			managedProcessor.processor.Stop()
			e.managedProcessors.Delete(shardID)
		}()
		return true
	})

	wg.Wait()
}
