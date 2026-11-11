package shard

import (
	"sync"

	"github.com/uber/cadence/common/persistence"
)

type (
	// DLQAckLevelKey identifies one DLQ partition and task category within a shard.
	DLQAckLevelKey struct {
		DomainID              string
		ClusterAttributeScope string
		ClusterAttributeName  string
		TaskCategoryID        int
	}

	// IndexedDLQAckLevel is a snapshot of one index entry.
	IndexedDLQAckLevel struct {
		Level      persistence.HistoryDLQAckLevel
		Generation uint64
	}

	dlqIndexEntry struct {
		level      persistence.HistoryDLQAckLevel
		dirty      bool   // a task may exist past the ack level
		generation uint64 // bumped by every MarkWrite
	}

	// DLQAckLevelIndex is an in-memory view of a shard's history_task_dlq_ack_level rows.
	// Seeded() is false until the first Merge. After that, a missing key means the row does not exist.
	// Each periodic sweep merges the rows from persistence again. Safe for concurrent use.
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

// Seeded reports whether Merge has run at least once.
func (i *DLQAckLevelIndex) Seeded() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.seeded
}

// Merge marks the index seeded and adds the given ack levels. New keys start dirty. Known keys keep
// the higher ack position, and no entry is removed.
func (i *DLQAckLevelIndex) Merge(levels []persistence.HistoryDLQAckLevel) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.seeded = true
	for _, level := range levels {
		i.upsertLocked(level)
	}
}

// Lookup returns snapshots of the entries for one partition, across task categories. The
// active-passive partition is ("", ""). dirtyOnly skips clean entries.
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
	return result
}

// All returns snapshots of every entry.
func (i *DLQAckLevelIndex) All() []IndexedDLQAckLevel {
	i.mu.Lock()
	defer i.mu.Unlock()
	result := make([]IndexedDLQAckLevel, 0, len(i.entries))
	for _, entry := range i.entries {
		result = append(result, entry.snapshot())
	}
	return result
}

// Has reports whether the key is present. It works before the index is seeded.
func (i *DLQAckLevelIndex) Has(key DLQAckLevelKey) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	_, ok := i.entries[key]
	return ok
}

// Add inserts level as a dirty entry if its key is absent. It does not mark the index seeded.
func (i *DLQAckLevelIndex) Add(level persistence.HistoryDLQAckLevel) {
	i.mu.Lock()
	defer i.mu.Unlock()
	key := DLQAckLevelKeyOf(level)
	if _, ok := i.entries[key]; ok {
		return
	}
	i.entries[key] = &dlqIndexEntry{level: level, dirty: true}
}

// MarkWrite marks the entry dirty and bumps its generation after a task write. It does nothing if
// the key is absent.
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

// Advance moves the entry's ack position forward, inserting a dirty entry if the key is absent.
func (i *DLQAckLevelIndex) Advance(level persistence.HistoryDLQAckLevel) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.upsertLocked(level)
}

// MarkClean clears the dirty bit if the entry's generation still equals generation.
func (i *DLQAckLevelIndex) MarkClean(key DLQAckLevelKey, generation uint64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	entry, ok := i.entries[key]
	if !ok || entry.generation != generation {
		return
	}
	entry.dirty = false
}

// upsertLocked inserts level as a dirty entry if absent, or keeps the higher ack position. The
// caller must hold i.mu.
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
