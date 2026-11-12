package matching

import (
	"regexp"
	"strconv"

	"github.com/dgryski/go-farm"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

const uuidRegex = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`

var uuidRegexp = regexp.MustCompile(uuidRegex)

// TaskListDifferentiator decides which shard holds a task list and whether the shard distributor
// manages it. It reads the dynamic config on every call, so changes take effect without
// rebuilding it.
type TaskListDifferentiator struct {
	metricsClient              metrics.Client
	excludeShortLivedTaskLists dynamicproperties.BoolPropertyFn
	shardTaskListsByHash       dynamicproperties.BoolPropertyFn
	percentageOnboarded        dynamicproperties.IntPropertyFn
	shardCount                 int
}

func NewTaskListDifferentiator(
	metricsClient metrics.Client,
	excludeShortLivedTaskLists dynamicproperties.BoolPropertyFn,
	shardTaskListsByHash dynamicproperties.BoolPropertyFn,
	percentageOnboarded dynamicproperties.IntPropertyFn,
	shardCount int,
) *TaskListDifferentiator {
	return &TaskListDifferentiator{
		metricsClient:              metricsClient,
		excludeShortLivedTaskLists: excludeShortLivedTaskLists,
		shardTaskListsByHash:       shardTaskListsByHash,
		percentageOnboarded:        percentageOnboarded,
		shardCount:                 shardCount,
	}
}

// TaskListShard is the shard that holds a task list, and whether the shard distributor manages it.
type TaskListShard struct {
	// ID is one of shardCount shards when task lists are sharded by hash, formatted the way the shard
	// manager formats the shard IDs of a fixed namespace. Otherwise each task list has its own shard,
	// named after it.
	ID string
	// Onboarded is true when the shard distributor manages the task list, and false when the hash
	// ring does.
	Onboarded bool
}

// TaskListShard returns the shard that holds the task list and whether the shard distributor
// manages it. It reads the dynamic config once, so the two always agree. Callers that make several
// decisions about one task list should compute it once and pass it on.
func (d *TaskListDifferentiator) TaskListShard(taskListName string) TaskListShard {
	// This regex checks if the task list name has a UUID, if it does we
	// consider it a short lived tasklist that will not be managed by the shard distributor.
	excludedAsShortLived := d.excludeShortLivedTaskLists() && uuidRegexp.MatchString(taskListName)
	percentageOnboarded := uint64(d.currentPercentageOnboarded())

	if d.shardTaskListsByHash() {
		shardIndex := taskListShardIndex(taskListName, d.shardCount)
		// Onboard the first percentageOnboarded percent of shards, so every task list in a shard
		// gets the same decision.
		shardOnboarded := shardIndex*100 < percentageOnboarded*uint64(d.shardCount)
		return TaskListShard{
			ID:        formatShardID(shardIndex),
			Onboarded: !excludedAsShortLived && shardOnboarded,
		}
	}

	hash := farm.Fingerprint64([]byte(taskListName))
	return TaskListShard{
		ID:        taskListName,
		Onboarded: !excludedAsShortLived && isbelowPercentage(hash, percentageOnboarded),
	}
}

// ShardDistributorEnabled reports whether the shard distributor manages any task lists, that is
// whether the onboarding percentage is above 0. Matching hosts heartbeat to the shard distributor
// only while it is, so driving the percentage to 0 offboards them.
func (d *TaskListDifferentiator) ShardDistributorEnabled() bool {
	return d.currentPercentageOnboarded() > 0
}

// ShardsByHash reports whether task lists are currently grouped into hashed shards.
func (d *TaskListDifferentiator) ShardsByHash() bool {
	return d.shardTaskListsByHash()
}

// ShardCount returns how many shards task list names hash into.
func (d *TaskListDifferentiator) ShardCount() int {
	return d.shardCount
}

// TaskListNameToShard returns which of shardCount shards a task list name hashes to, formatted the
// way the shard manager formats the shard IDs of a fixed namespace.
func TaskListNameToShard(taskListName string, shardCount int) string {
	shardIndex := taskListShardIndex(taskListName, shardCount)
	return formatShardID(shardIndex)
}

// currentPercentageOnboarded reads the onboarding percentage and reports it as a gauge, so
// dashboards show the value each host acts on.
func (d *TaskListDifferentiator) currentPercentageOnboarded() int {
	percentageOnboarded := d.percentageOnboarded()
	d.metricsClient.Scope(metrics.ShardManagerOnboardingScope).
		UpdateGauge(metrics.PercentageOnboardedToShardManagerGauge, float64(percentageOnboarded))
	return percentageOnboarded
}

func taskListShardIndex(taskListName string, shardCount int) uint64 {
	hash := farm.Fingerprint64([]byte(taskListName))
	return hash % uint64(shardCount)
}

// formatShardID formats a shard index the way the shard manager formats the shard IDs of a fixed
// namespace. The registry and the routing decision must agree on it.
func formatShardID(shardIndex uint64) string {
	return strconv.FormatUint(shardIndex, 10)
}

func isbelowPercentage(hash uint64, percentageOnboarded uint64) bool {
	return hash%100 < percentageOnboarded
}
