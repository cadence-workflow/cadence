package shard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/persistence"
)

func testIndexLevel(name string, taskID int64) persistence.HistoryDLQAckLevel {
	return persistence.HistoryDLQAckLevel{
		ShardID:               1,
		DomainID:              "d",
		ClusterAttributeScope: "s",
		ClusterAttributeName:  name,
		TaskCategory:          persistence.HistoryTaskCategoryTransfer,
		AckLevelVisibilityTS:  persistence.NewImmediateTaskKey(taskID).GetScheduledTime(),
		AckLevelTaskID:        taskID,
	}
}

func TestDLQAckLevelIndex_MergeIsMonotoneAndNeverRemoves(t *testing.T) {
	idx := NewDLQAckLevelIndex()
	require.False(t, idx.Seeded())

	low := testIndexLevel("n", 10)
	high := testIndexLevel("n", 20)

	idx.Merge([]persistence.HistoryDLQAckLevel{high})
	require.True(t, idx.Seeded())

	// A lower position from a later read never regresses the entry.
	idx.Merge([]persistence.HistoryDLQAckLevel{low})
	got := idx.All()
	require.Len(t, got, 1)
	assert.Equal(t, int64(20), got[0].Level.AckLevelTaskID)

	// An empty read never removes entries.
	idx.Merge(nil)
	require.Len(t, idx.All(), 1)

	// Entries start dirty so the first failover after a seed re-verifies them.
	require.Len(t, idx.Lookup("d", "s", "n", true), 1)
}

func TestDLQAckLevelIndex_MarkCleanRespectsGeneration(t *testing.T) {
	idx := NewDLQAckLevelIndex()
	level := testIndexLevel("n", 1)
	key := DLQAckLevelKeyOf(level)
	idx.Merge([]persistence.HistoryDLQAckLevel{level})

	gen := idx.Lookup("d", "s", "n", false)[0].Generation
	// A write lands after the snapshot was taken.
	idx.MarkWrite(key)
	// A stale generation must not clear the dirty bit.
	idx.MarkClean(key, gen)
	require.Len(t, idx.Lookup("d", "s", "n", true), 1)

	gen = idx.Lookup("d", "s", "n", false)[0].Generation
	idx.MarkClean(key, gen)
	require.Empty(t, idx.Lookup("d", "s", "n", true))
	require.Len(t, idx.Lookup("d", "s", "n", false), 1, "clean entries stay in the index")

	// MarkWrite makes it dirty again.
	idx.MarkWrite(key)
	require.Len(t, idx.Lookup("d", "s", "n", true), 1)
}

func TestDLQAckLevelIndex_LookupExactKeyForDefaultPartition(t *testing.T) {
	idx := NewDLQAckLevelIndex()
	def := testIndexLevel("", 1)
	def.ClusterAttributeScope = ""
	attr := testIndexLevel("r1", 1)
	attr.ClusterAttributeScope = "region"
	idx.Merge([]persistence.HistoryDLQAckLevel{def, attr})

	// The default (active-passive) partition is exactly ("", ""): it must not match attribute partitions.
	got := idx.Lookup("d", "", "", false)
	require.Len(t, got, 1)
	assert.Equal(t, "", got[0].Level.ClusterAttributeScope)
	assert.Equal(t, "", got[0].Level.ClusterAttributeName)

	got = idx.Lookup("d", "region", "r1", false)
	require.Len(t, got, 1)
	assert.Equal(t, "r1", got[0].Level.ClusterAttributeName)

	assert.Empty(t, idx.Lookup("other-domain", "", "", false))
}

func TestDLQAckLevelIndex_MarkWriteOnAbsentKeyIsNoOp_AddKeepsExisting(t *testing.T) {
	idx := NewDLQAckLevelIndex()
	level := testIndexLevel("n", 5)
	key := DLQAckLevelKeyOf(level)

	idx.MarkWrite(key)
	require.False(t, idx.Has(key))
	require.Empty(t, idx.All())

	idx.Add(level)
	require.True(t, idx.Has(key))
	require.False(t, idx.Seeded(), "Add is not a seed")
	require.Len(t, idx.Lookup("d", "s", "n", true), 1, "added entries start dirty")

	advanced := testIndexLevel("n", 9)
	idx.Add(advanced) // Add never overwrites an existing entry
	assert.Equal(t, int64(5), idx.All()[0].Level.AckLevelTaskID)

	idx.Advance(advanced) // Advance does
	assert.Equal(t, int64(9), idx.All()[0].Level.AckLevelTaskID)
	idx.Advance(level) // but never backwards
	assert.Equal(t, int64(9), idx.All()[0].Level.AckLevelTaskID)
}

func TestDLQAckLevelIndex_DirtyReturnsOnlyDirtyEntries(t *testing.T) {
	idx := NewDLQAckLevelIndex()
	clean := testIndexLevel("clean", 1)
	dirty := testIndexLevel("dirty", 1)
	idx.Merge([]persistence.HistoryDLQAckLevel{clean, dirty})
	idx.MarkClean(DLQAckLevelKeyOf(clean), idx.Lookup("d", "s", "clean", false)[0].Generation)

	got := idx.Dirty()
	require.Len(t, got, 1)
	assert.Equal(t, "dirty", got[0].Level.ClusterAttributeName)
}
