package store

import (
	"fmt"
)

type ExecutorState string

const (
	ExecutorStateActive   ExecutorState = "ACTIVE"
	ExecutorStateDraining ExecutorState = "DRAINING"
	ExecutorStateDrained  ExecutorState = "DRAINED"
)

func ParseExecutorState(s string) (ExecutorState, error) {
	switch s {
	case "ACTIVE":
		return ExecutorStateActive, nil
	case "DRAINING":
		return ExecutorStateDraining, nil
	case "DRAINED":
		return ExecutorStateDrained, nil
	default:
		return "", fmt.Errorf("invalid state: %s", s)
	}
}

type ShardState string

const (
	ShardStateReady ShardState = "READY"
)

type HeartbeatState struct {
	LastHeartbeat  int64         `json:"last_heartbeat"`
	State          ExecutorState `json:"state"`
	ReportedShards map[string]ShardInfo
}

type ShardInfo struct {
	Status      ShardState        `json:"status"`
	ShardLoad   float64           `json:"shard_load"`
	LastUpdated int64             `json:"last_updated"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type ShardAssignment struct {
	AssignedAt int64      `json:"assigned_at"`
	Status     ShardState `json:"status"`
}

type AssignedState struct {
	AssignedShards map[string]ShardAssignment `json:"assigned_shards"` // What we assigned
	LastUpdated    int64                      `json:"last_updated"`
}
