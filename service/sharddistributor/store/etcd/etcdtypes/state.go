package etcdtypes

import (
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type AssignedState struct {
	AssignedShards map[string]*types.ShardAssignment `json:"assigned_shards"`
	LastUpdated    Time                              `json:"last_updated"`
	ModRevision    int64                             `json:"mod_revision"`
}

// ToAssignedState converts the current AssignedState to store.AssignedState.
func (s *AssignedState) ToAssignedState() *store.AssignedState {
	if s == nil {
		return nil
	}

	return &store.AssignedState{
		AssignedShards: s.AssignedShards,
		LastUpdated:    s.LastUpdated.ToTime(),
		ModRevision:    s.ModRevision,
	}
}

// FromAssignedState creates an AssignedState from a store.AssignedState.
func FromAssignedState(src *store.AssignedState) *AssignedState {
	if src == nil {
		return nil
	}

	return &AssignedState{
		AssignedShards: src.AssignedShards,
		LastUpdated:    Time(src.LastUpdated),
		ModRevision:    src.ModRevision,
	}
}

type ShardStatistics struct {
	SmoothedLoad   float64 `json:"smoothed_load"`
	LastUpdateTime Time    `json:"last_update_time"`
	LastMoveTime   Time    `json:"last_move_time"`
}

// ToShardStatistics converts the current ShardStatistics to store.ShardStatistics.
func (s *ShardStatistics) ToShardStatistics() *store.ShardStatistics {
	if s == nil {
		return nil
	}

	return &store.ShardStatistics{
		SmoothedLoad:   s.SmoothedLoad,
		LastUpdateTime: s.LastUpdateTime.ToTime(),
		LastMoveTime:   s.LastMoveTime.ToTime(),
	}
}

// FromShardStatistics creates a ShardStatistics from a store.ShardStatistics.
func FromShardStatistics(src *store.ShardStatistics) *ShardStatistics {
	if src == nil {
		return nil
	}

	return &ShardStatistics{
		SmoothedLoad:   src.SmoothedLoad,
		LastUpdateTime: Time(src.LastUpdateTime),
		LastMoveTime:   Time(src.LastMoveTime),
	}
}
