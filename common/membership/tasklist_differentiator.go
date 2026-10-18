package membership

import (
	"regexp"
	"strconv"

	"github.com/dgryski/go-farm"
)

const uuidRegex = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`

var uuidRegexp = regexp.MustCompile(uuidRegex)

func TaskListExcludedFromShardDistributor(
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
		// Onboarding is decided per shard, so every task list in a shard gets the same decision.
		shardIndex := taskListNameToShardIndex(taskListName, shardCount)
		return !shardBelowPercentage(shardIndex, shardCount, percentageOnboarded)
	}
	return !belowPercentage(taskListName, percentageOnboarded)
}

func belowPercentage(taskListName string, percentageOnboarded uint64) bool {
	hash := farm.Fingerprint64([]byte(taskListName))
	return isbelowPercentage(hash, percentageOnboarded)
}

func isbelowPercentage(hash uint64, percentageOnboarded uint64) bool {
	return hash%100 < percentageOnboarded
}

// shardBelowPercentage reports whether a shard is among the first percentageOnboarded percent of
// shardCount shards.
func shardBelowPercentage(shardIndex uint64, shardCount int, percentageOnboarded uint64) bool {
	return shardIndex*100 < percentageOnboarded*uint64(shardCount)
}

// TaskListShardID returns the shard that holds a task list in the shard distributor: the shard its
// name hashes to when task lists are sharded by hash, and the task list name otherwise.
func TaskListShardID(taskListName string, shardTaskListsByHash bool, shardCount int) string {
	if shardTaskListsByHash {
		return TaskListNameToShard(taskListName, shardCount)
	}
	return taskListName
}

// TaskListNameToShard returns which of shardCount shards a task list name hashes to, formatted the
// way the shard manager formats the shard IDs of a fixed namespace.
func TaskListNameToShard(taskListName string, shardCount int) string {
	shardIndex := taskListNameToShardIndex(taskListName, shardCount)
	return strconv.FormatUint(shardIndex, 10)
}

// taskListNameToShardIndex returns which of shardCount shards a task list name hashes to.
func taskListNameToShardIndex(taskListName string, shardCount int) uint64 {
	hash := farm.Fingerprint64([]byte(taskListName))
	return hash % uint64(shardCount)
}
