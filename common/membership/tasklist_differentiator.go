package membership

import (
	"regexp"
	"strconv"

	"github.com/dgryski/go-farm"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
)

const uuidRegex = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`

var uuidRegexp = regexp.MustCompile(uuidRegex)

// TaskListKey identifies a matching task list for routing. Name is the full name, including the
// partition. DomainID can be empty for keys that carry no domain, such as semaphore buckets.
type TaskListKey struct {
	DomainID string
	Name     string
}

// TaskListDifferentiator decides which shard holds a task list and whether the shard distributor
// manages it. It reads the dynamic config on every call, so changes take effect without
// rebuilding it.
type TaskListDifferentiator struct {
	excludeShortLivedTaskLists dynamicproperties.BoolPropertyFn
	shardTaskListsByHash       dynamicproperties.BoolPropertyFn
	percentageOnboarded        PercentageOnboarded
	shardCount                 int
}

func NewTaskListDifferentiator(
	excludeShortLivedTaskLists dynamicproperties.BoolPropertyFn,
	shardTaskListsByHash dynamicproperties.BoolPropertyFn,
	percentageOnboarded PercentageOnboarded,
	shardCount int,
) *TaskListDifferentiator {
	return &TaskListDifferentiator{
		excludeShortLivedTaskLists: excludeShortLivedTaskLists,
		shardTaskListsByHash:       shardTaskListsByHash,
		percentageOnboarded:        percentageOnboarded,
		shardCount:                 shardCount,
	}
}

// TaskListExcludedFromShardDistributor reports whether the hash ring owns the task list instead of
// the shard distributor.
func (d *TaskListDifferentiator) TaskListExcludedFromShardDistributor(key TaskListKey) bool {
	// This regex checks if the task list name has a UUID, if it does we
	// consider it a short lived tasklist that will not be managed by the shard distributor.
	if d.excludeShortLivedTaskLists() && uuidRegexp.MatchString(key.Name) {
		return true
	}

	percentageOnboarded := uint64(d.percentageOnboarded.Value())
	if d.shardTaskListsByHash() {
		// Onboard the first percentageOnboarded percent of shards, so every task list in a shard
		// gets the same decision.
		shardIndex := taskListShardIndex(key, d.shardCount)
		return shardIndex*100 >= percentageOnboarded*uint64(d.shardCount)
	}

	hash := farm.Fingerprint64([]byte(key.Name))
	return !isbelowPercentage(hash, percentageOnboarded)
}

// ShardID returns the shard that holds the task list. When task lists are sharded by hash it is one
// of shardCount shards, formatted the way the shard manager formats the shard IDs of a fixed
// namespace. Otherwise each task list name has its own shard, named after it, shared by every
// domain that uses the name.
func (d *TaskListDifferentiator) ShardID(key TaskListKey) string {
	if d.shardTaskListsByHash() {
		return TaskListToShard(key, d.shardCount)
	}
	return key.Name
}

// ShardsByHash reports whether task lists are currently grouped into hashed shards.
func (d *TaskListDifferentiator) ShardsByHash() bool {
	return d.shardTaskListsByHash()
}

// ShardCount returns how many shards task lists hash into.
func (d *TaskListDifferentiator) ShardCount() int {
	return d.shardCount
}

// TaskListToShard returns which of shardCount shards a task list hashes to, formatted the way the
// shard manager formats the shard IDs of a fixed namespace.
func TaskListToShard(key TaskListKey, shardCount int) string {
	shardIndex := taskListShardIndex(key, shardCount)
	return strconv.FormatUint(shardIndex, 10)
}

// taskListShardIndex hashes the domain ID together with the name, so task lists that share a name
// across many domains, such as the scheduler's, spread over the shards instead of piling into one.
// The domain ID is a UUID and never contains the separator, so the hashed key is unambiguous.
func taskListShardIndex(key TaskListKey, shardCount int) uint64 {
	hash := farm.Fingerprint64([]byte(key.DomainID + "/" + key.Name))
	return hash % uint64(shardCount)
}

func isbelowPercentage(hash uint64, percentageOnboarded uint64) bool {
	return hash%100 < percentageOnboarded
}
