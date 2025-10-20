package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	_heartbeatRefreshRate = 2 * time.Second
)

type executor struct {
	logger               log.Logger
	timeSource           clock.TimeSource
	storage              store.Store
	shardDistributionCfg config.ShardDistribution
}

func NewExecutorHandler(
	logger log.Logger,
	storage store.Store,
	timeSource clock.TimeSource,
	shardDistributionCfg config.ShardDistribution,
) Executor {
	return &executor{
		logger:               logger,
		timeSource:           timeSource,
		storage:              storage,
		shardDistributionCfg: shardDistributionCfg,
	}
}

func (h *executor) Heartbeat(ctx context.Context, request *types.ExecutorHeartbeatRequest) (*types.ExecutorHeartbeatResponse, error) {
	previousHeartbeat, assignedShards, err := h.storage.GetHeartbeat(ctx, request.Namespace, request.ExecutorID)
	// We ignore Executor not found errors, since it just means that this executor heartbeat the first time.
	if err != nil && !errors.Is(err, store.ErrExecutorNotFound) {
		return nil, fmt.Errorf("get heartbeat: %w", err)
	}

	now := h.timeSource.Now().UTC()

	mode := h.shardDistributionCfg.GetMigrationMode(request.Namespace)

	switch mode {
	case types.MigrationModeINVALID:
		h.logger.Warn("Migration mode is invalid", tag.ShardNamespace(request.Namespace), tag.ShardExecutor(request.ExecutorID))
		return nil, fmt.Errorf("migration mode is invalid")
	case types.MigrationModeLOCALPASSTHROUGH:
		h.logger.Warn("Migration mode is local passthrough, no calls to heartbeat allowed", tag.ShardNamespace(request.Namespace), tag.ShardExecutor(request.ExecutorID))
		return nil, fmt.Errorf("migration mode is local passthrough")
	// From SD perspective the behaviour is the same
	case types.MigrationModeLOCALPASSTHROUGHSHADOW, types.MigrationModeDISTRIBUTEDPASSTHROUGH:
		assignedShards, err = h.assignShardsInCurrentHeartbeat(ctx, request, previousHeartbeat, assignedShards)
		if err != nil {
			return nil, err
		}
	}

	// If the state has changed we need to update heartbeat data.
	// Otherwise, we want to do it with controlled frequency - at most every _heartbeatRefreshRate.
	if previousHeartbeat != nil && request.Status == previousHeartbeat.Status && mode == types.MigrationModeONBOARDED {
		lastHeartbeatTime := time.Unix(previousHeartbeat.LastHeartbeat, 0)
		if now.Sub(lastHeartbeatTime) < _heartbeatRefreshRate {
			return _convertResponse(assignedShards, mode), nil
		}
	}

	newHeartbeat := store.HeartbeatState{
		LastHeartbeat:  now.Unix(),
		Status:         request.Status,
		ReportedShards: request.ShardStatusReports,
	}

	err = h.storage.RecordHeartbeat(ctx, request.Namespace, request.ExecutorID, newHeartbeat)
	if err != nil {
		return nil, fmt.Errorf("record heartbeat: %w", err)
	}

	return _convertResponse(assignedShards, mode), nil
}

// assignShardsInCurrentHeartbeat is used during the migration phase to assign the shards to the executors according to what is reported during the heartbeat
func (h *executor) assignShardsInCurrentHeartbeat(ctx context.Context, request *types.ExecutorHeartbeatRequest, previousHeartbeat *store.HeartbeatState, previousAssignedShards *store.AssignedState) (*store.AssignedState, error) {
	assignedShards := *previousAssignedShards

	assignedShards = store.AssignedState{
		AssignedShards: make(map[string]*types.ShardAssignment),
		LastUpdated:    h.timeSource.Now().Unix(),
		ModRevision:    int64(0),
	}
	err := h.storage.DeleteExecutors(ctx, request.GetNamespace(), []string{request.GetExecutorID()}, store.NopGuard())
	if err != nil {
		return nil, fmt.Errorf("delete executors: %w", err)
	}
	for shard := range request.GetShardStatusReports() {
		assignedShards.AssignedShards[shard] = &types.ShardAssignment{
			Status: types.AssignmentStatusREADY,
		}
	}
	assignShardsRequest := store.AssignShardsRequest{
		NewState: &store.NamespaceState{
			ShardAssignments: map[string]store.AssignedState{
				request.GetExecutorID(): assignedShards,
			},
		},
	}
	err = h.storage.AssignShards(ctx, request.GetNamespace(), assignShardsRequest, store.NopGuard())
	if err != nil {
		return nil, fmt.Errorf("assign shards in current heartbeat: %w", err)
	}

	return &assignedShards, nil
}

func _convertResponse(shards *store.AssignedState, mode types.MigrationMode) *types.ExecutorHeartbeatResponse {
	res := &types.ExecutorHeartbeatResponse{}
	if shards == nil {
		return res
	}
	res.ShardAssignments = shards.AssignedShards
	res.MigrationMode = mode
	return res
}
