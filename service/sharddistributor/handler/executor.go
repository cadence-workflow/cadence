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
	if previousHeartbeat != nil && !needUpdate(request.Status, previousHeartbeat.State) {
		lastHeartbeatTime := time.Unix(previousHeartbeat.LastHeartbeat, 0)
		if now.Sub(lastHeartbeatTime) < _heartbeatRefreshRate {
			return _convertResponse(assignedShards), nil
		}
	}

	newHeartbeat := store.HeartbeatState{
		LastHeartbeat:  now.Unix(),
		State:          _executorStatusReverseMapping[request.Status],
		ReportedShards: _convertReportedShards(request.ShardStatusReports),
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
	res.ShardAssignments = make(map[string]*types.ShardAssignment, len(shards.AssignedShards))
	for shardID, state := range shards.AssignedShards {
		res.ShardAssignments[shardID] = &types.ShardAssignment{Status: _shardStatusReverseMapping[state.Status]}
	}
	return res
}

func _convertReportedShards(reports map[string]*types.ShardStatusReport) map[string]store.ShardInfo {
	res := make(map[string]store.ShardInfo, len(reports))
	for shardID, report := range reports {
		res[shardID] = store.ShardInfo{
			Status:    _shardStatusMapping[report.Status],
			ShardLoad: report.ShardLoad,
		}
	}
	return res
}

func needUpdate(status types.ExecutorStatus, state store.ExecutorState) bool {
	return _executorStatusMapping[status] != state
}

var _executorStatusMapping = map[types.ExecutorStatus]store.ExecutorState{
	types.ExecutorStatusACTIVE:   store.ExecutorStateActive,
	types.ExecutorStatusDRAINED:  store.ExecutorStateDrained,
	types.ExecutorStatusDRAINING: store.ExecutorStateDraining,
}

var _executorStatusReverseMapping = map[types.ExecutorStatus]store.ExecutorState{}

var _shardStatusReverseMapping = map[store.ShardState]types.AssignmentStatus{
	store.ShardStateReady: types.AssignmentStatusREADY,
}

var _shardStatusMapping = map[types.ShardStatus]store.ShardState{
	types.ShardStatusREADY: store.ShardStateReady,
}

func init() {
	for status, state := range _executorStatusMapping {
		_executorStatusReverseMapping[status] = state
	}
}
