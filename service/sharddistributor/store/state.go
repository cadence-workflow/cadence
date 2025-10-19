package store

import (
	"github.com/uber/cadence/common/types"
)

type HeartbeatState struct {
	LastHeartbeat  int64                               `json:"last_heartbeat"`
	Status         types.ExecutorStatus                `json:"status"`
	ReportedShards map[string]*types.ShardStatusReport `json:"reported_shards"`
}

type AssignedState struct {
	AssignedShards map[string]*types.ShardAssignment `json:"assigned_shards"` // What we assigned
	LastUpdated    int64                             `json:"last_updated"`
	ModRevision    int64                             `json:"mod_revision"`
}

type NamespaceState struct {
	Executors        map[string]HeartbeatState
	ShardMetrics     map[string]ShardMetrics
	ShardAssignments map[string]AssignedState
	GlobalRevision   int64
}

type ShardState struct {
	ExecutorID string
}

type ShardMetrics struct {
	SmoothedLoad   float64 `json:"smoothed_load"`    // EWMA of shard load that persists across executor changes
	LastUpdateTime int64   `json:"last_update_time"` // heartbeat timestamp that last updated the EWMA
	LastMoveTime   int64   `json:"last_move_time"`   // timestamp for the latest reassignment, used for cooldowns
}
