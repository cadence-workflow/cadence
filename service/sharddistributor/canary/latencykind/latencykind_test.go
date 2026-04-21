package latencykind

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestKind_String(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{Normal, "normal"},
		{SlowStart, "slow_start"},
		{SlowStop, "slow_stop"},
		{StuckStart, "stuck_start"},
		{StuckStop, "stuck_stop"},
		{Kind(99), "normal"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}

func TestKind_Delays(t *testing.T) {
	tests := []struct {
		kind      Kind
		wantStart time.Duration
		wantStop  time.Duration
	}{
		{Normal, 0, 0},
		{SlowStart, slowDelay, 0},
		{SlowStop, 0, slowDelay},
		{StuckStart, stuckDelay, 0},
		{StuckStop, 0, stuckDelay},
	}
	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			assert.Equal(t, tt.wantStart, tt.kind.StartDelay())
			assert.Equal(t, tt.wantStop, tt.kind.StopDelay())
		})
	}
}

func TestShardIDToKind_Deterministic(t *testing.T) {
	for _, id := range []string{"0", "1", "shard-foo", uuid.New().String()} {
		assert.Equal(t, ShardIDToKind(id), ShardIDToKind(id), "shardID %q must map to the same kind on every call", id)
	}
}

func TestShardIDToKind_Distribution(t *testing.T) {
	const n = 50_000
	counts := make(map[Kind]int, 5)
	for i := 0; i < n; i++ {
		counts[ShardIDToKind(uuid.New().String())]++
	}

	// Allow 30% relative tolerance — we want to catch big skews but not be
	// flaky on hash variance for low-frequency kinds.
	checks := []struct {
		kind     Kind
		fraction float64
	}{
		{Normal, 96.0 / distributionMod},
		{SlowStart, 1.0 / distributionMod},
		{SlowStop, 1.0 / distributionMod},
		{StuckStart, 1.0 / distributionMod},
		{StuckStop, 1.0 / distributionMod},
	}
	for _, c := range checks {
		t.Run(c.kind.String(), func(t *testing.T) {
			expected := float64(n) * c.fraction
			assert.InDelta(t, expected, float64(counts[c.kind]), expected*0.3,
				"kind %s saw %d / %d, expected ~%.0f", c.kind, counts[c.kind], n, expected)
		})
	}
}

func TestShardIDToKind_FixedNamespaceCoverage(t *testing.T) {
	// The fixed namespace currently uses 32 numbered shards. Make sure the
	// hash maps at least one of them to a non-normal kind so the canary
	// always exercises slow paths even without UUID churn.
	saw := make(map[Kind]bool)
	for i := 0; i < 32; i++ {
		saw[ShardIDToKind(strconv.Itoa(i))] = true
	}
	assert.Truef(t, len(saw) > 1, "expected at least one non-normal kind across 32 fixed shards, got only %v", saw)
}
