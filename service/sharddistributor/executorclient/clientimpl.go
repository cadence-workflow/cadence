package executorclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/uber/cadence/client/sharddistributorexecutor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/executorclient/syncgeneric"
)

type processorState int

const (
	processorStateStarting processorState = iota
	processorStateStarted
	processorStateStopping
)

type managedProcessor[SP ShardProcessor] struct {
	processor SP
	state     processorState
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
}

func (e *executorImpl[SP]) Start(ctx context.Context) {
	go e.heartbeatloop(ctx)
}

func (e *executorImpl[SP]) Stop() {
	close(e.stopC)
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
			e.updateShardAssignment(ctx, shardAssignment)
		}
	}
}

func (e *executorImpl[SP]) heartbeat(ctx context.Context) (shardAssignments map[string]*types.ShardAssignment, err error) {
	// Fill in the shard status reports
	shardStatusReports := make(map[string]*types.ShardStatusReport)
	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		if managedProcessor.state == processorStateStarted {
			shardStatusReports[shardID] = &types.ShardStatusReport{
				ShardLoad: managedProcessor.processor.GetShardLoad(),
				Status:    types.ShardStatusREADY,
			}
		}
		return true
	})

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
	// Stop shard processing for shards not assigned to this executor
	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		if assignment, ok := shardAssignments[shardID]; !ok || assignment.Status != types.AssignmentStatusREADY {
			go func() {
				managedProcessor.state = processorStateStopping
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
				go func() {
					processor, err := e.shardProcessorFactory.NewShardProcessor(shardID)
					if err != nil {
						e.logger.Error("failed to create shard processor", tag.Error(err))
						return
					}
					processor.Start(ctx)
					e.managedProcessors.Store(shardID, &managedProcessor[SP]{
						processor: processor,
						state:     processorStateStarted,
					})
				}()
			}
		}
	}
}

func (e *executorImpl[SP]) stopShardProcessors() {
	wg := sync.WaitGroup{}

	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		// If the processor is already stopping, skip it
		if managedProcessor.state == processorStateStopping {
			return true
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			managedProcessor.state = processorStateStopping
			managedProcessor.processor.Stop()
			e.managedProcessors.Delete(shardID)
		}()
		return true
	})

	wg.Wait()
}
