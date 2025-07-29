package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	_heartbeatRefreshRate = 2 * time.Second
)

type executor struct {
	timeSource clock.TimeSource
	storage    store.Store
}

func NewExecutorHandler(storage store.Store,
	timeSource clock.TimeSource,
) Executor {
	return &executor{
		timeSource: timeSource,
		storage:    storage,
	}
}

func (h *executor) Heartbeat(ctx context.Context, request *types.ExecutorHeartbeatRequest) (*types.ExecutorHeartbeatResponse, error) {
	previousHeartbeat, assignedShards, err := h.storage.GetHeartbeat(ctx, request.Namespace, request.ExecutorID)
	// We ignore Executor not found errors, since it just means that this executor heartbeat the first time.
	if err != nil && !errors.Is(err, store.ErrExecutorNotFound) {
		return nil, fmt.Errorf("get heartbeat: %w", err)
	}

	now := h.timeSource.Now().UTC()

	// If the state has changed we need to update heartbeat data.
	// Otherwise, we want to do it with controlled frequency - at most every _heartbeatRefreshRate.
	if previousHeartbeat != nil && request.Status == previousHeartbeat.Status {
		lastHeartbeatTime := time.Unix(previousHeartbeat.LastHeartbeat, 0)
		if now.Sub(lastHeartbeatTime) < _heartbeatRefreshRate {
			return _convertResponse(assignedShards), nil
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

	return _convertResponse(assignedShards), nil
}

func _convertResponse(shards *store.AssignedState) *types.ExecutorHeartbeatResponse {
	res := &types.ExecutorHeartbeatResponse{}
	if shards == nil {
		return res
	}
	res.ShardAssignments = shards.AssignedShards
	return res
}
