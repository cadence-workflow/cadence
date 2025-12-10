package etcdtypes

import (
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type AssignedState struct {
	AssignedShards     map[string]*types.ShardAssignment `json:"assigned_shards"`
	ShardHandoverStats map[string]ShardHandoverStats     `json:"shard_handover_stats,omitempty"`
	LastUpdated        Time                              `json:"last_updated"`
	// ModRevision is the etcd mod revision for this record. It is not serialized.
	ModRevision int64 `json:"-"`
}

// ToAssignedState converts the current AssignedState to store.AssignedState.
func (s *AssignedState) ToAssignedState() *store.AssignedState {
	if s == nil {
		return nil
	}

	return &store.AssignedState{
		AssignedShards:     s.AssignedShards,
		ShardHandoverStats: convertMap(s.ShardHandoverStats, ToShardHandoverStats),
		LastUpdated:        s.LastUpdated.ToTime(),
		ModRevision:        s.ModRevision,
	}
}

// FromAssignedState creates an AssignedState from a store.AssignedState.
func FromAssignedState(src *store.AssignedState) *AssignedState {
	if src == nil {
		return nil
	}

	return &AssignedState{
		AssignedShards:     src.AssignedShards,
		LastUpdated:        Time(src.LastUpdated),
		ShardHandoverStats: convertMap(src.ShardHandoverStats, FromShardHandoverStats),
		ModRevision:        src.ModRevision,
	}
}

type ShardHandoverStats struct {
	PreviousExecutorLastHeartbeatTime Time               `json:"previous_executor_last_heartbeat_time"`
	HandoverType                      types.HandoverType `json:"handover_type"`
}

// FromShardHandoverStats creates an ShardHandoverStats from a store.ShardHandoverStats.
func FromShardHandoverStats(src *store.ShardHandoverStats) *ShardHandoverStats {
	if src == nil {
		return nil
	}

	return &ShardHandoverStats{
		PreviousExecutorLastHeartbeatTime: Time(src.PreviousExecutorLastHeartbeatTime),
		HandoverType:                      src.HandoverType,
	}
}

// ToShardHandoverStats converts the current ShardHandoverStats to store.ShardHandoverStats.
func ToShardHandoverStats(src *ShardHandoverStats) *store.ShardHandoverStats {
	if src == nil {
		return nil
	}

	return &store.ShardHandoverStats{
		PreviousExecutorLastHeartbeatTime: src.PreviousExecutorLastHeartbeatTime.ToTime(),
		HandoverType:                      src.HandoverType,
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

// ConvertMap converts a map[K]SrcType to map[K]DstType using a provided converter function.
func convertMap[K comparable, SrcType any, DstType any](src map[K]SrcType, converter func(*SrcType) *DstType) map[K]DstType {
	if src == nil {
		return nil
	}
	dst := make(map[K]DstType, len(src))
	for k, v := range src {
		dst[k] = *converter(&v)
	}
	return dst
}
