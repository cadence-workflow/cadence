package store

type ExecutorState string

const (
	ExecutorStateActive   ExecutorState = "ACTIVE"
	ExecutorStateDraining ExecutorState = "DRAINING"
	ExecutorStateStopped  ExecutorState = "STOPPED"
)

type HeartbeatState struct {
	ExecutorID     string        `json:"executor_id"`
	LastHeartbeat  int64         `json:"last_heartbeat"`
	State          ExecutorState `json:"state"`
	ReportedShards map[string]ShardState
}

type ShardState struct {
	Status      string            `json:"status"` // e.g., "running", "stopped", "error"
	LastUpdated int64             `json:"last_updated"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type ShardAssignment struct {
	ShardID    string            `json:"shard_id"`
	AssignedAt int64             `json:"assigned_at"`
	Priority   int               `json:"priority,omitempty"`
	Config     map[string]string `json:"config,omitempty"`
}

type AssignedState struct {
	ExecutorID     string                     `json:"executor_id"`
	ReportedShards map[string]ShardState      `json:"reported_shards"` // What executor reports
	AssignedShards map[string]ShardAssignment `json:"assigned_shards"` // What we assigned
	LastUpdated    int64                      `json:"last_updated"`
}
