package membership

import (
	"regexp"
	"strconv"

	"github.com/dgryski/go-farm"
)

const uuidRegex = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`

var uuidRegexp = regexp.MustCompile(uuidRegex)

// TaskListSharding is how task list names map to shard distributor shards, and which of those
// shards are onboarded at a given percentage.
type TaskListSharding interface {
	// ShardID returns the shard that holds the task list.
	ShardID(taskListName string) string
	onboarded(taskListName string, percentageOnboarded uint64) bool
}

// shardPerTaskList gives each task list its own shard, named after it.
type shardPerTaskList struct{}

// fixedShards hashes task list names into a fixed number of shards.
type fixedShards struct {
	count int
}

// NewTaskListSharding returns fixed shards when task lists are sharded by hash, and one shard per
// task list otherwise.
func NewTaskListSharding(shardTaskListsByHash bool, shardCount int) TaskListSharding {
	if shardTaskListsByHash {
		return fixedShards{count: shardCount}
	}
	return shardPerTaskList{}
}

func TaskListExcludedFromShardDistributor(
	taskListName string,
	percentageOnboarded uint64,
	excludeShortLivedTaskLists bool,
	sharding TaskListSharding,
) bool {
	// This regex checks if the task list name has a UUID, if it does we
	// consider it a short lived tasklist that will not be managed by the shard distributor.
	if excludeShortLivedTaskLists && uuidRegexp.MatchString(taskListName) {
		return true
	}

	return !sharding.onboarded(taskListName, percentageOnboarded)
}

// TaskListNameToShard returns which of shardCount shards a task list name hashes to, formatted the
// way the shard manager formats the shard IDs of a fixed namespace.
func TaskListNameToShard(taskListName string, shardCount int) string {
	sharding := fixedShards{count: shardCount}
	return sharding.ShardID(taskListName)
}

func (shardPerTaskList) ShardID(taskListName string) string {
	return taskListName
}

func (shardPerTaskList) onboarded(taskListName string, percentageOnboarded uint64) bool {
	hash := farm.Fingerprint64([]byte(taskListName))
	return isbelowPercentage(hash, percentageOnboarded)
}

func (s fixedShards) ShardID(taskListName string) string {
	shardIndex := s.shardIndex(taskListName)
	return strconv.FormatUint(shardIndex, 10)
}

// onboarded reports whether the task list's shard is among the first percentageOnboarded percent
// of shards, so every task list in a shard gets the same decision.
func (s fixedShards) onboarded(taskListName string, percentageOnboarded uint64) bool {
	shardIndex := s.shardIndex(taskListName)
	return shardIndex*100 < percentageOnboarded*uint64(s.count)
}

func (s fixedShards) shardIndex(taskListName string) uint64 {
	hash := farm.Fingerprint64([]byte(taskListName))
	return hash % uint64(s.count)
}

func isbelowPercentage(hash uint64, percentageOnboarded uint64) bool {
	return hash%100 < percentageOnboarded
}
