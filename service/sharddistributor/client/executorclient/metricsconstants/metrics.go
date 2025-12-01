package metricsconstants

import (
	"time"

	"github.com/uber-go/tally"
)

const (
	// Operation tag names for ShardDistributorExecutor metrics
	ShardDistributorExecutorOperationTagName          = "ShardDistributorExecutor"
	ShardDistributorExecutorHeartbeatOperationTagName = "ShardDistributorExecutorHeartbeat"

	// Counter metrics
	ShardDistributorExecutorHeartbeatSkipped          = "shard_distributor_executor_heartbeat_skipped"
	ShardDistributorExecutorShardsStarted             = "shard_distributor_executor_shards_started"
	ShardDistributorExecutorShardsStopped             = "shard_distributor_executor_shards_stopped"
	ShardDistributorExecutorProcessorCreationFailures = "shard_distributor_executor_processor_creation_failures"
	ShardDistributorExecutorClientRequests            = "shard_distributor_executor_client_requests"
	ShardDistributorExecutorClientFailures            = "shard_distributor_executor_client_failures"
	ShardDistributorExecutorAssignmentDivergence      = "shard_distributor_executor_assignment_divergence"
	ShardDistributorExecutorAssignmentConvergence     = "shard_distributor_executor_assignment_convergence"

	// Gauge metrics
	ShardDistributorExecutorOwnedShards = "shard_distributor_executor_owned_shards"

	// Histogram/Timer metrics
	ShardDistributorExecutorAssignLoopLatency = "shard_distributor_executor_assign_loop_latency"
	ShardDistributorExecutorClientLatency     = "shard_distributor_executor_client_latency"
)

var (
	// Histogram buckets for ShardDistributorExecutor metrics
	ShardDistributorExecutorAssignLoopLatencyBuckets = tally.DurationBuckets([]time.Duration{
		0,
		1 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		25 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		20 * time.Second,
		60 * time.Second,
	})
)
