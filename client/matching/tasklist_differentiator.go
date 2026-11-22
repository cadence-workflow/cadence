package matching

import (
	"regexp"
	"strconv"

	"github.com/dgryski/go-farm"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/shardmanager"
)

const uuidRegex = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`

var uuidRegexp = regexp.MustCompile(uuidRegex)

// TaskListDifferentiator decides which shard holds a task list and whether the shard distributor
// manages it.
type TaskListDifferentiator struct {
	excludeShortLivedTaskLists dynamicproperties.BoolPropertyFn
	shardTaskListsByHash       dynamicproperties.BoolPropertyFn
	hashDomainID               dynamicproperties.BoolPropertyFn
	percentageOnboarded        shardmanager.PercentageOnboarded
	shardCount                 int
}

func NewTaskListDifferentiator(
	metricsClient metrics.Client,
	excludeShortLivedTaskLists dynamicproperties.BoolPropertyFn,
	shardTaskListsByHash dynamicproperties.BoolPropertyFn,
	hashDomainID dynamicproperties.BoolPropertyFn,
	percentageOnboarded dynamicproperties.IntPropertyFn,
	shardCount int,
) *TaskListDifferentiator {
	return &TaskListDifferentiator{
		excludeShortLivedTaskLists: excludeShortLivedTaskLists,
		shardTaskListsByHash:       shardTaskListsByHash,
		hashDomainID:               hashDomainID,
		percentageOnboarded:        shardmanager.NewPercentageOnboarded(metricsClient, metrics.ShardManagerOnboardingScope, percentageOnboarded),
		shardCount:                 shardCount,
	}
}

// TaskListShard is the shard that holds a task list, and whether the shard distributor manages it.
type TaskListShard struct {
	id                          string
	onboardedToShardDistributor bool
}

// ShardID returns the task list's shard ID, or "" and false if the hash ring owns the task list.
// The ID is one of shardCount shards when task lists are sharded by hash. Otherwise each task list
// name has its own shard, named after it, shared by every domain that uses the name.
func (s TaskListShard) ShardID() (id string, onboardedToShardDistributor bool) {
	return s.id, s.onboardedToShardDistributor
}

// TaskListShard returns the shard that holds the task list and whether the shard distributor
// manages it. It reads the dynamic config once, so the two always agree. Callers that make several
// decisions about one task list should compute it once and pass it on.
func (d *TaskListDifferentiator) TaskListShard(domainID, taskListName string) TaskListShard {
	// This regex checks if the task list name has a UUID, if it does we
	// consider it a short lived tasklist that will not be managed by the shard distributor.
	excludedAsShortLived := d.excludeShortLivedTaskLists() && uuidRegexp.MatchString(taskListName)
	percentageOnboarded := d.percentageOnboarded.Current()

	if d.shardTaskListsByHash() {
		var shardIndex uint64
		if d.hashDomainID() {
			shardIndex = taskListShardIndex(domainID, taskListName, d.shardCount)
		} else {
			shardIndex = taskListNameShardIndex(taskListName, d.shardCount)
		}
		// Onboard the first percentageOnboarded percent of shards, so every task list in a shard
		// gets the same decision.
		shardOnboarded := shardmanager.ShardOnboarded(int(shardIndex), d.shardCount, percentageOnboarded)
		if excludedAsShortLived || !shardOnboarded {
			return TaskListShard{id: "", onboardedToShardDistributor: false}
		}
		return TaskListShard{id: formatShardID(shardIndex), onboardedToShardDistributor: true}
	}

	hash := farm.Fingerprint64([]byte(taskListName))
	if excludedAsShortLived || !isbelowPercentage(hash, percentageOnboarded) {
		return TaskListShard{id: "", onboardedToShardDistributor: false}
	}
	return TaskListShard{id: taskListName, onboardedToShardDistributor: true}
}

// ShardDistributorEnabled reports whether the onboarding percentage is above 0. Matching hosts
// heartbeat to the shard distributor only while it is, so setting it to 0 offboards them.
func (d *TaskListDifferentiator) ShardDistributorEnabled() bool {
	return d.percentageOnboarded.Enabled()
}

// ShardsByHash reports whether task lists are currently grouped into hashed shards.
func (d *TaskListDifferentiator) ShardsByHash() bool {
	return d.shardTaskListsByHash()
}

// ShardCount returns how many shards task lists hash into.
func (d *TaskListDifferentiator) ShardCount() int {
	return d.shardCount
}

// TaskListToShard returns which of shardCount shards a task list hashes to by domain ID and name.
func TaskListToShard(domainID, taskListName string, shardCount int) string {
	shardIndex := taskListShardIndex(domainID, taskListName, shardCount)
	return formatShardID(shardIndex)
}

// TaskListNameToShard returns which of shardCount shards a task list hashes to by name alone, which
// is its shard while matching.hashDomainIDForShardManager is disabled.
func TaskListNameToShard(taskListName string, shardCount int) string {
	shardIndex := taskListNameShardIndex(taskListName, shardCount)
	return formatShardID(shardIndex)
}

// The domain ID is hashed too, so task lists that share a name across domains spread over the shards.
func taskListShardIndex(domainID, taskListName string, shardCount int) uint64 {
	hash := farm.Fingerprint64([]byte(domainID + "/" + taskListName))
	return hash % uint64(shardCount)
}

// taskListNameShardIndex puts every task list with the same name in the same shard, whatever its domain.
func taskListNameShardIndex(taskListName string, shardCount int) uint64 {
	hash := farm.Fingerprint64([]byte(taskListName))
	return hash % uint64(shardCount)
}

// formatShardID formats a shard index the way the shard manager formats the shard IDs of a fixed
// namespace.
func formatShardID(shardIndex uint64) string {
	return strconv.FormatUint(shardIndex, 10)
}

func isbelowPercentage(hash uint64, percentageOnboarded int) bool {
	return int(hash%100) < percentageOnboarded
}
