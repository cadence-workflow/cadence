package shard

import (
	"sort"
	"sync"

	"github.com/uber/cadence/common/persistence"
)

type (
	// DLQAckLevelKey identifies a single DLQ partition + task category within a shard.
	DLQAckLevelKey struct {
		DomainID              string
		ClusterAttributeScope string
		ClusterAttributeName  string
		TaskCategoryID        int
	}

	// IndexedDLQAckLevel is a snapshot of one index entry handed to the processor.
	IndexedDLQAckLevel struct {
		Level      persistence.HistoryDLQAckLevel
		Generation uint64
	}

	dlqIndexEntry struct {
		level      persistence.HistoryDLQAckLevel
		dirty      bool   // a task may exist past the ack level; cleared only by MarkClean
		generation uint64 // bumped on every MarkWrite; MarkClean is ignored if it moved
	}

	// DLQAckLevelIndex is the shard-owned in-memory view of the shard's history_task_dlq_ack_level rows.
	// Before the first Merge, Seeded() is false and absence means "unknown"; after it, absence means
	// "no ack-level row exists for this shard" (only the owning shard writes rows; the periodic sweep
	// re-merges every interval as a backstop). All methods are safe for concurrent use.
	DLQAckLevelIndex struct {
		mu      sync.Mutex
		seeded  bool
		entries map[DLQAckLevelKey]*dlqIndexEntry
	}
)

// DLQAckLevelKeyOf returns the index key for an ack level.
func DLQAckLevelKeyOf(level persistence.HistoryDLQAckLevel) DLQAckLevelKey {
	return DLQAckLevelKey{
		DomainID:              level.DomainID,
		ClusterAttributeScope: level.ClusterAttributeScope,
		ClusterAttributeName:  level.ClusterAttributeName,
		TaskCategoryID:        level.TaskCategory.ID(),
	}
}

// NewDLQAckLevelIndex returns an empty, unseeded index.
func NewDLQAckLevelIndex() *DLQAckLevelIndex {
	return &DLQAckLevelIndex{entries: make(map[DLQAckLevelKey]*dlqIndexEntry)}
}

// Seeded reports whether the index has been populated by at least one Merge.
func (i *DLQAckLevelIndex) Seeded() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.seeded
}

// Merge is monotone: it marks the index seeded, inserts unknown keys as dirty, and for known keys
// takes the max ack position, leaving dirty/generation untouched. It never removes entries.
// It is used by the seed read and the periodic sweep read.
func (i *DLQAckLevelIndex) Merge(levels []persistence.HistoryDLQAckLevel) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.seeded = true
	for _, level := range levels {
		i.upsertLocked(level)
	}
}

// Lookup returns snapshots of the entries whose key matches (domainID, scope, name) exactly. The
// default (active-passive) partition is exactly ("", ""). dirtyOnly filters to dirty entries.
// Results are sorted by (DomainID, ClusterAttributeScope, ClusterAttributeName, TaskCategoryID).
func (i *DLQAckLevelIndex) Lookup(domainID, clusterAttributeScope, clusterAttributeName string, dirtyOnly bool) []IndexedDLQAckLevel {
	i.mu.Lock()
	defer i.mu.Unlock()
	var result []IndexedDLQAckLevel
	for key, entry := range i.entries {
		if key.DomainID != domainID ||
			key.ClusterAttributeScope != clusterAttributeScope ||
			key.ClusterAttributeName != clusterAttributeName {
			continue
		}
		if dirtyOnly && !entry.dirty {
			continue
		}
		result = append(result, entry.snapshot())
	}
	sortSnapshots(result)
	return result
}

// All returns snapshots of every entry, sorted the same way as Lookup.
func (i *DLQAckLevelIndex) All() []IndexedDLQAckLevel {
	i.mu.Lock()
	defer i.mu.Unlock()
	result := make([]IndexedDLQAckLevel, 0, len(i.entries))
	for _, entry := range i.entries {
		result = append(result, entry.snapshot())
	}
	sortSnapshots(result)
	return result
}

// Has reports whether the key is present. It does not require the index to be seeded.
func (i *DLQAckLevelIndex) Has(key DLQAckLevelKey) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	_, ok := i.entries[key]
	return ok
}

// Add inserts level as a dirty entry if its key is absent; it is a no-op when the key is present.
// It does not mark the index seeded.
func (i *DLQAckLevelIndex) Add(level persistence.HistoryDLQAckLevel) {
	i.mu.Lock()
	defer i.mu.Unlock()
	key := DLQAckLevelKeyOf(level)
	if _, ok := i.entries[key]; ok {
		return
	}
	i.entries[key] = &dlqIndexEntry{level: level, dirty: true}
}

// MarkWrite records that a task was (about to be) written past the ack level: the entry becomes
// dirty and its generation is bumped. It is a no-op when the key is absent.
func (i *DLQAckLevelIndex) MarkWrite(key DLQAckLevelKey) {
	i.mu.Lock()
	defer i.mu.Unlock()
	entry, ok := i.entries[key]
	if !ok {
		return
	}
	entry.dirty = true
	entry.generation++
}

// Advance max-merges the ack position for level's key, inserting a dirty entry if absent.
func (i *DLQAckLevelIndex) Advance(level persistence.HistoryDLQAckLevel) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.upsertLocked(level)
}

// MarkClean clears the dirty bit only if the entry exists and its generation still equals generation.
func (i *DLQAckLevelIndex) MarkClean(key DLQAckLevelKey, generation uint64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	entry, ok := i.entries[key]
	if !ok || entry.generation != generation {
		return
	}
	entry.dirty = false
}

// upsertLocked inserts level as a dirty entry if absent, otherwise max-merges its ack position.
// The caller must hold i.mu.
func (i *DLQAckLevelIndex) upsertLocked(level persistence.HistoryDLQAckLevel) {
	key := DLQAckLevelKeyOf(level)
	entry, ok := i.entries[key]
	if !ok {
		i.entries[key] = &dlqIndexEntry{level: level, dirty: true}
		return
	}
	if ackPosition(level).Compare(ackPosition(entry.level)) > 0 {
		entry.level.AckLevelVisibilityTS = level.AckLevelVisibilityTS
		entry.level.AckLevelTaskID = level.AckLevelTaskID
	}
}

func (e *dlqIndexEntry) snapshot() IndexedDLQAckLevel {
	return IndexedDLQAckLevel{Level: e.level, Generation: e.generation}
}

func ackPosition(level persistence.HistoryDLQAckLevel) persistence.HistoryTaskKey {
	return persistence.NewHistoryTaskKey(level.AckLevelVisibilityTS, level.AckLevelTaskID)
}

func sortSnapshots(snapshots []IndexedDLQAckLevel) {
	sort.Slice(snapshots, func(a, b int) bool {
		x, y := DLQAckLevelKeyOf(snapshots[a].Level), DLQAckLevelKeyOf(snapshots[b].Level)
		if x.DomainID != y.DomainID {
			return x.DomainID < y.DomainID
		}
		if x.ClusterAttributeScope != y.ClusterAttributeScope {
			return x.ClusterAttributeScope < y.ClusterAttributeScope
		}
		if x.ClusterAttributeName != y.ClusterAttributeName {
			return x.ClusterAttributeName < y.ClusterAttributeName
		}
		return x.TaskCategoryID < y.TaskCategoryID
	})
}
