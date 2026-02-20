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
	// AssignedShards holds the current assignment of shards to this executor
	// Key: ShardID
	AssignedShards map[string]*types.ShardAssignment

	// ShardHandoverStats holds handover statistics of all shards experienced handovers to this executor
	// Mostly all shards in AssignedShards will have corresponding entries here
	// But if a shard was assigned but never had a handover (e.g., first assignment), it does not have an entry here
	// Key: ShardID
	ShardHandoverStats map[string]ShardHandoverStats

	// LastUpdated is the time when this assignment state was last updated
	// Used to calculate assignment distribution latency for newly assigned shards
	LastUpdated time.Time
	ModRevision int64
}

// ShardHandoverStats holds statistics related to the latest handover of a shard
type ShardHandoverStats struct {
	// PreviousExecutorLastHeartbeatTime is the last heartbeat time received
	// from the previous executor before the shard was reassigned.
	PreviousExecutorLastHeartbeatTime time.Time

	// HandoverType indicates the type of handover that occurred during the last shard reassignment.
	HandoverType types.HandoverType
}

type NamespaceState struct {
	// Executors holds the heartbeat states of all executors in the namespace.
	// Key: ExecutorID
	Executors map[string]HeartbeatState

	// ShardStats holds the statistics of all shards in the namespace.
	// Only loaded for namespace which types.LoadBalancingMode is types.LoadBalancingModeGREEDY
	// Key: ShardID
	ShardStats map[string]ShardStatistics

	// ShardAssignments holds the assignment states of all shards in the namespace.
	// Key: ExecutorID
	ShardAssignments map[string]AssignedState
}

type ShardState struct {
	ExecutorID string
}

type ShardStatistics struct {
	// Exponential weighted moving average of shard load that persists across executor changes
	SmoothedLoad float64

	// LastUpdateTime is the heartbeat timestamp that last updated the smoothed load
	LastUpdateTime time.Time

	// LastMoveTime is the timestamp when this shard was last reassigned
	LastMoveTime time.Time
}

type ShardOwner struct {
	ExecutorID string
	Metadata   map[string]string
}

// CountExecutorsByStatus returns a map of executor status to the count of executors with that status
func (ns *NamespaceState) CountExecutorsByStatus() map[types.ExecutorStatus]int {
	counts := make(map[types.ExecutorStatus]int)
	for _, executor := range ns.Executors {
		counts[executor.Status]++
	}
	return counts
}
