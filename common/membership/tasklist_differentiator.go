package membership

import (
	"regexp"
	"strconv"

	"github.com/dgryski/go-farm"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
)

const uuidRegex = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`

var uuidRegexp = regexp.MustCompile(uuidRegex)

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
func (d *TaskListDifferentiator) TaskListExcludedFromShardDistributor(taskListName string) bool {
	return taskListExcludedFromShardDistributor(
		taskListName,
		uint64(d.percentageOnboarded.Value()),
		d.excludeShortLivedTaskLists(),
		d.shardTaskListsByHash(),
		d.shardCount,
	)
}

// ShardID returns the shard that holds the task list. When task lists are sharded by hash it is one
// of shardCount shards, formatted the way the shard manager formats the shard IDs of a fixed
// namespace. Otherwise each task list has its own shard, named after it.
func (d *TaskListDifferentiator) ShardID(taskListName string) string {
	return taskListShardID(taskListName, d.shardTaskListsByHash(), d.shardCount)
}

// ShardsByHash reports whether task lists are currently grouped into hashed shards.
func (d *TaskListDifferentiator) ShardsByHash() bool {
	return d.shardTaskListsByHash()
}

func taskListExcludedFromShardDistributor(
	taskListName string,
	percentageOnboarded uint64,
	excludeShortLivedTaskLists bool,
	shardTaskListsByHash bool,
	shardCount int,
) bool {
	// This regex checks if the task list name has a UUID, if it does we
	// consider it a short lived tasklist that will not be managed by the shard distributor.
	if excludeShortLivedTaskLists && uuidRegexp.MatchString(taskListName) {
		return true
	}

	if shardTaskListsByHash {
		// Onboard the first percentageOnboarded percent of shards, so every task list in a shard
		// gets the same decision.
		shardIndex := taskListShardIndex(taskListName, shardCount)
		return shardIndex*100 >= percentageOnboarded*uint64(shardCount)
	}

	hash := farm.Fingerprint64([]byte(taskListName))
	return !isbelowPercentage(hash, percentageOnboarded)
}

func taskListShardID(taskListName string, shardTaskListsByHash bool, shardCount int) string {
	if shardTaskListsByHash {
		return TaskListNameToShard(taskListName, shardCount)
	}
	return taskListName
}

// TaskListNameToShard returns which of shardCount shards a task list name hashes to, formatted the
// way the shard manager formats the shard IDs of a fixed namespace.
func TaskListNameToShard(taskListName string, shardCount int) string {
	shardIndex := taskListShardIndex(taskListName, shardCount)
	return strconv.FormatUint(shardIndex, 10)
}

func taskListShardIndex(taskListName string, shardCount int) uint64 {
	hash := farm.Fingerprint64([]byte(taskListName))
	return hash % uint64(shardCount)
}

func isbelowPercentage(hash uint64, percentageOnboarded uint64) bool {
	return hash%100 < percentageOnboarded
}
