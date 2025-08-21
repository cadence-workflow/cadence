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
}

type NamespaceState struct {
	Executors        map[string]HeartbeatState
	Shards           map[string]ShardState
	ShardAssignments map[string]AssignedState
	GlobalRevision   int64
}

type ShardState struct {
	ExecutorID string
	Revision   int64
}
