package metrics

import (
	"time"

	"github.com/uber-go/tally"
)

const (
	// Counter metrics
	CanaryPingSuccess           = "canary_ping_success"
	CanaryPingFailure           = "canary_ping_failure"
	CanaryPingOwnershipMismatch = "canary_ping_ownership_mismatch"
	CanaryShardCreated          = "canary_shard_created"
	CanaryShardStarted          = "canary_shard_started"
	CanaryShardStopped          = "canary_shard_stopped"
	CanaryShardDone             = "canary_shard_done"
	CanaryShardProcessStep      = "canary_shard_process_step"

	// Histogram metrics
	CanaryPingLatency = "canary_ping_latency"
)

var (
	CanaryPingLatencyBuckets = tally.DurationBuckets([]time.Duration{
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
	})
)
