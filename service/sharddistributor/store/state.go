package store

import (
	"time"

	"github.com/uber/cadence/common/types"
)

type HeartbeatState struct {
	// LastHeartbeat is the time of the last heartbeat received from the executor
	LastHeartbeat  time.Time
	Status         types.ExecutorStatus
	ReportedShards map[string]*types.ShardStatusReport
	Metadata       map[string]string
}

type AssignedState struct {
	// AssignedShards is the map of shard ID to shard assignment
	AssignedShards map[string]*types.ShardAssignment

	// LastUpdated is the time we last updated this assignment
	LastUpdated time.Time
	ModRevision int64
}

type NamespaceState struct {
	Executors        map[string]HeartbeatState
	ShardStats       map[string]ShardStatistics
	ShardAssignments map[string]AssignedState
	GlobalRevision   int64
}

type ShardState struct {
	ExecutorID string
}

type ShardStatistics struct {
	// EWMA of shard load that persists across executor changes
	SmoothedLoad float64

	// LastUpdateTime is the heartbeat timestamp that last updated the EWMA
	LastUpdateTime time.Time

	// LastMoveTime is the timestamp when this shard was last reassigned
	LastMoveTime time.Time
}

type ShardOwner struct {
	ExecutorID string
	Metadata   map[string]string
}
