// Package latencykind assigns each canary shard a Start/Stop latency kind.
//
// The canary intentionally injects slow and stuck Start/Stop calls on a small
// fraction of shards. This exercises the executor's per-shard goroutine and
// timeout machinery and lets us alert when normal shards stop being processed
// because of misbehaving ones.
//
// The kind is selected by a deterministic hash of the shard ID, so the fixed
// namespace's stable numeric shard IDs keep the same kind across restarts and
// ephemeral UUID shards distribute across kinds at the configured rate.
package latencykind

import (
	"time"

	farm "github.com/dgryski/go-farm"
)

// Kind categorises a shard's intended Start/Stop latency behaviour. Each
// shard is assigned exactly one kind for its lifetime.
type Kind int

const (
	// Normal shards Start and Stop with no injected delay.
	Normal Kind = iota
	// SlowStart shards delay Start below the executor's per-shard timeout.
	SlowStart
	// SlowStop shards delay Stop below the executor's per-shard timeout.
	SlowStop
	// StuckStart shards delay Start past the executor's per-shard timeout so
	// the framework's start-timeout path is exercised. The Start call still
	// returns afterwards so goroutines do not leak forever.
	StuckStart
	// StuckStop shards delay Stop past the executor's per-shard timeout so
	// the framework's stop-timeout path is exercised. The Stop call still
	// returns afterwards.
	StuckStop
)

// The executor's processorAsyncOperationTimeout is 10s. Slow delays sit safely
// below it so the framework never times out, stuck delays sit just above it so
// the framework's timeout metric fires once per stuck transition then the call
// completes. Keep stuck delays modest to bound goroutine accumulation.
const (
	slowDelay  = 5 * time.Second
	stuckDelay = 15 * time.Second
)

// distributionMod is the modulus applied to the shard ID hash for kind
// selection. With the assignments below: SlowStart 1%, SlowStop 1%,
// StuckStart 1%, StuckStop 1%, Normal 96%.
const distributionMod = 100

// String returns a stable lowercase tag for metric tagging.
func (k Kind) String() string {
	switch k {
	case SlowStart:
		return "slow_start"
	case SlowStop:
		return "slow_stop"
	case StuckStart:
		return "stuck_start"
	case StuckStop:
		return "stuck_stop"
	default:
		return "normal"
	}
}

// StartDelay is the delay injected before Start() returns.
func (k Kind) StartDelay() time.Duration {
	switch k {
	case SlowStart:
		return slowDelay
	case StuckStart:
		return stuckDelay
	default:
		return 0
	}
}

// StopDelay is the delay injected before Stop() returns.
func (k Kind) StopDelay() time.Duration {
	switch k {
	case SlowStop:
		return slowDelay
	case StuckStop:
		return stuckDelay
	default:
		return 0
	}
}

// ShardIDToKind deterministically maps a shard ID to a Kind.
func ShardIDToKind(shardID string) Kind {
	n := farm.Fingerprint32([]byte(shardID)) % distributionMod

	switch {
	case n < 1:
		return SlowStart
	case n < 2:
		return SlowStop
	case n < 3:
		return StuckStart
	case n < 4:
		return StuckStop
	default:
		return Normal
	}
}
