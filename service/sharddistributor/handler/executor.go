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

	_maxMetadataKeys      = 32
	_maxMetadataKeyLength = 128
	_maxMetadataValueSize = 512 * 1024 // 512KB
)

type executor struct {
	logger                 log.Logger
	timeSource             clock.TimeSource
	storage                store.Store
	shardDistributionCfg   config.ShardDistribution
	migrationConfiguration *config.MigrationConfig
}

func NewExecutorHandler(
	logger log.Logger,
	storage store.Store,
	timeSource clock.TimeSource,
	shardDistributionCfg config.ShardDistribution,
	migrationConfig *config.MigrationConfig,
) Executor {
	return &executor{
		logger:                 logger,
		timeSource:             timeSource,
		storage:                storage,
		shardDistributionCfg:   shardDistributionCfg,
		migrationConfiguration: migrationConfig,
	}
}

func (h *executor) Heartbeat(ctx context.Context, request *types.ExecutorHeartbeatRequest) (*types.ExecutorHeartbeatResponse, error) {
	previousHeartbeat, assignedShards, err := h.storage.GetHeartbeat(ctx, request.Namespace, request.ExecutorID)
	// We ignore Executor not found errors, since it just means that this executor heartbeat the first time.
	if err != nil && !errors.Is(err, store.ErrExecutorNotFound) {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to get heartbeat: %v", err)}
	}

	now := h.timeSource.Now().UTC()
	mode := h.migrationConfiguration.GetMigrationMode(request.Namespace)

	switch mode {
	case types.MigrationModeINVALID:
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("namespace's migration mode is invalid: %v", err)}
	case types.MigrationModeLOCALPASSTHROUGH:
		h.logger.Warn("Migration mode is local passthrough, no calls to heartbeat allowed", tag.ShardNamespace(request.Namespace), tag.ShardExecutor(request.ExecutorID))
		return nil, &types.BadRequestError{Message: "migration mode is local passthrough, no calls to heartbeat allowed"}
	// From SD perspective the behaviour is the same
	case types.MigrationModeLOCALPASSTHROUGHSHADOW, types.MigrationModeDISTRIBUTEDPASSTHROUGH:
		assignedShards, err = h.assignShardsInCurrentHeartbeat(ctx, request)
		if err != nil {
			return nil, err
		}
	}

	// If the state has changed we need to update heartbeat data.
	// Otherwise, we want to do it with controlled frequency - at most every _heartbeatRefreshRate.
	if previousHeartbeat != nil && request.Status == previousHeartbeat.Status && mode == types.MigrationModeONBOARDED {
		lastHeartbeatTime := previousHeartbeat.LastHeartbeat
		if now.Sub(lastHeartbeatTime) < _heartbeatRefreshRate {
			return _convertResponse(assignedShards, mode), nil
		}
	}

	newHeartbeat := store.HeartbeatState{
		LastHeartbeat:  now,
		Status:         request.Status,
		ReportedShards: request.ShardStatusReports,
		Metadata:       request.GetMetadata(),
	}

	if err := validateMetadata(newHeartbeat.Metadata); err != nil {
		return nil, types.BadRequestError{Message: fmt.Sprintf("invalid metadata: %s", err)}
	}

	err = h.storage.RecordHeartbeat(ctx, request.Namespace, request.ExecutorID, newHeartbeat)
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to record heartbeat: %v", err)}
	}

	return _convertResponse(assignedShards, mode), nil
}

// assignShardsInCurrentHeartbeat is used during the migration phase to assign the shards to the executors according to what is reported during the heartbeat
func (h *executor) assignShardsInCurrentHeartbeat(ctx context.Context, request *types.ExecutorHeartbeatRequest) (*store.AssignedState, error) {
	assignedShards := store.AssignedState{
		AssignedShards: make(map[string]*types.ShardAssignment),
		LastUpdated:    h.timeSource.Now().UTC(),
		ModRevision:    int64(0),
	}
	err := h.storage.DeleteExecutors(ctx, request.GetNamespace(), []string{request.GetExecutorID()}, store.NopGuard())
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to delete assigned shards: %v", err)}
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
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to assign shards in the current heartbeat: %v", err)}
	}
	return &assignedShards, nil
}

func _convertResponse(shards *store.AssignedState, mode types.MigrationMode) *types.ExecutorHeartbeatResponse {
	res := &types.ExecutorHeartbeatResponse{}
	res.MigrationMode = mode
	if shards == nil {
		return res
	}
	res.ShardAssignments = shards.AssignedShards
	return res
}

func validateMetadata(metadata map[string]string) error {
	if len(metadata) > _maxMetadataKeys {
		return fmt.Errorf("metadata has %d keys, which exceeds the maximum of %d", len(metadata), _maxMetadataKeys)
	}

	for key, value := range metadata {
		if len(key) > _maxMetadataKeyLength {
			return fmt.Errorf("metadata key %q has length %d, which exceeds the maximum of %d", key, len(key), _maxMetadataKeyLength)
		}

		if len(value) > _maxMetadataValueSize {
			return fmt.Errorf("metadata value for key %q has size %d bytes, which exceeds the maximum of %d bytes", key, len(value), _maxMetadataValueSize)
		}
	}

	return nil
}
