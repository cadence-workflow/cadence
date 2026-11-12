package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

const (
	testDomainID      = "22222222-2222-4222-8222-222222222222"
	testOtherDomainID = "55555555-5555-4555-8555-555555555555"
)

func TestTaskListDifferentiator_TaskListShard_Onboarded(t *testing.T) {
	shardCount := 1000

	tests := []struct {
		name                       string
		domainID                   string
		taskListName               string
		wantOnboarded              bool
		percentageOnboarded        int
		excludeShortLivedTaskLists bool
		shardTaskListsByHash       bool
	}{
		{
			name:                       "task list with UUID - exclude enabled",
			taskListName:               "tasklist-550e8400-e29b-41d4-a716-446655440000",
			wantOnboarded:              false,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "task list with UUID - exclude disabled",
			taskListName:               "tasklist-550e8400-e29b-41d4-a716-446655440000",
			wantOnboarded:              true,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: false,
		},
		{
			name:                       "task list with uppercase UUID - exclude enabled",
			taskListName:               "tasklist-550E8400-E29B-41D4-A716-446655440000",
			wantOnboarded:              false,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "task list name is UUID only - exclude enabled",
			taskListName:               "550e8400-e29b-41d4-a716-446655440000",
			wantOnboarded:              false,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "task list without UUID",
			taskListName:               "my-task-list",
			wantOnboarded:              true,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "empty task list name",
			taskListName:               "",
			wantOnboarded:              true,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "task list with partial UUID-like string",
			taskListName:               "tasklist-550e8400-e29b",
			wantOnboarded:              true,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "task list with UUID prefix",
			taskListName:               "550e8400-e29b-41d4-a716-446655440000-suffix",
			wantOnboarded:              false,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "task list below percentage",
			taskListName:               "my-task-list", // farm.Fingerprint64([]byte("my-task-list"))%100 = 58
			wantOnboarded:              true,
			percentageOnboarded:        60,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "task list above percentage",
			taskListName:               "my-task-list", // farm.Fingerprint64([]byte("my-task-list"))%100 = 58
			wantOnboarded:              false,
			percentageOnboarded:        50,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "task list below percentage - but UUID - exclude enabled",
			taskListName:               "my-task-list-550e8400-e29b-41d4-a716-446655440003", // farm.Fingerprint64([]byte("my-task-list-550e8400-e29b-41d4-a716-446655440003"))%100 = 69
			wantOnboarded:              false,
			percentageOnboarded:        70,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "task list below percentage - UUID - exclude disabled",
			taskListName:               "my-task-list-550e8400-e29b-41d4-a716-446655440003", // farm.Fingerprint64([]byte("my-task-list-550e8400-e29b-41d4-a716-446655440003"))%100 = 69
			wantOnboarded:              true,
			percentageOnboarded:        70,
			excludeShortLivedTaskLists: false,
		},
		{
			name:                       "task list above percentage - but UUID",
			taskListName:               "my-task-list-550e8400-e29b-41d4-a716-446655440003", // farm.Fingerprint64([]byte("my-task-list-550e8400-e29b-41d4-a716-446655440003"))%100 = 69
			wantOnboarded:              false,
			percentageOnboarded:        60,
			excludeShortLivedTaskLists: true,
		},
		{
			// By name alone the shard would be 639, so only the domain ID puts it below the percentage.
			name:                 "sharded by hash - shard below percentage",
			domainID:             testDomainID,
			taskListName:         "orders", // farm.Fingerprint64([]byte(testDomainID+"/orders"))%1000 = 134
			wantOnboarded:        true,
			percentageOnboarded:  14,
			shardTaskListsByHash: true,
		},
		{
			// By name alone the shard would be 6, so only the domain ID puts it above the percentage.
			name:                 "sharded by hash - shard above percentage",
			domainID:             testOtherDomainID,
			taskListName:         "test-key", // farm.Fingerprint64([]byte(testOtherDomainID+"/test-key"))%1000 = 868
			wantOnboarded:        false,
			percentageOnboarded:  70,
			shardTaskListsByHash: true,
		},
		{
			name:                       "sharded by hash - UUID in the domain ID only - exclude enabled",
			domainID:                   testDomainID,
			taskListName:               "orders",
			wantOnboarded:              true,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: true,
			shardTaskListsByHash:       true,
		},
		{
			name:                       "UUID in the domain ID only - exclude enabled",
			domainID:                   testDomainID,
			taskListName:               "my-task-list", // farm.Fingerprint64([]byte("my-task-list"))%100 = 58
			wantOnboarded:              true,
			percentageOnboarded:        60,
			excludeShortLivedTaskLists: true,
		},
		{
			name:                       "sharded by hash - UUID - exclude enabled",
			taskListName:               "tasklist-550e8400-e29b-41d4-a716-446655440000",
			wantOnboarded:              false,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: true,
			shardTaskListsByHash:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			differentiator := NewTaskListDifferentiator(
				metrics.NewNoopMetricsClient(),
				dynamicproperties.GetBoolPropertyFn(tt.excludeShortLivedTaskLists),
				dynamicproperties.GetBoolPropertyFn(tt.shardTaskListsByHash),
				dynamicproperties.GetIntPropertyFn(tt.percentageOnboarded),
				shardCount,
			)

			shard := differentiator.TaskListShard(tt.domainID, tt.taskListName)
			assert.Equal(t, tt.wantOnboarded, shard.Onboarded)
		})
	}
}

func TestIsbelowPercentage(t *testing.T) {
	tests := []struct {
		name       string
		hash       uint64
		percentage uint64
		want       bool
	}{
		{
			name:       "hash and percentage are 0",
			hash:       0,
			percentage: 0,
			want:       false,
		},

		{
			name:       "hash is 0 and percentage is 1",
			hash:       0,
			percentage: 1,
			want:       true,
		},
		{
			name:       "hash is 100 and percentage is 1 (we wrap)",
			hash:       100,
			percentage: 1,
			want:       true,
		},
		{
			name:       "hash is same as percentage",
			hash:       33,
			percentage: 33,
			want:       false,
		},
		{
			name:       "hash is big",
			hash:       10000000000033,
			percentage: 34,
			want:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isbelowPercentage(tt.hash, tt.percentage)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTaskListDifferentiator_TaskListShard_ID(t *testing.T) {
	shardCount := 1000

	tests := []struct {
		name                 string
		domainID             string
		taskListName         string
		shardTaskListsByHash bool
		want                 string
	}{
		{
			name:                 "shard by task list name",
			domainID:             testDomainID,
			taskListName:         "orders",
			shardTaskListsByHash: false,
			want:                 "orders",
		},
		{
			name:                 "shard by task list name is shared by every domain",
			domainID:             testOtherDomainID,
			taskListName:         "orders",
			shardTaskListsByHash: false,
			want:                 "orders",
		},
		{
			name:                 "shard by hash",
			domainID:             testDomainID,
			taskListName:         "orders", // farm.Fingerprint64([]byte(testDomainID+"/orders"))%1000 = 134
			shardTaskListsByHash: true,
			want:                 "134",
		},
		{
			name:                 "shard by hash puts the same name in another domain in another shard",
			domainID:             testOtherDomainID,
			taskListName:         "orders", // farm.Fingerprint64([]byte(testOtherDomainID+"/orders"))%1000 = 178
			shardTaskListsByHash: true,
			want:                 "178",
		},
		{
			name:                 "shard by hash without a domain ID",
			domainID:             "",
			taskListName:         "orders", // farm.Fingerprint64([]byte("/orders"))%1000 = 469
			shardTaskListsByHash: true,
			want:                 "469",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			differentiator := NewTaskListDifferentiator(
				metrics.NewNoopMetricsClient(),
				dynamicproperties.GetBoolPropertyFn(false),
				dynamicproperties.GetBoolPropertyFn(tt.shardTaskListsByHash),
				dynamicproperties.GetIntPropertyFn(100),
				shardCount,
			)

			shard := differentiator.TaskListShard(tt.domainID, tt.taskListName)
			assert.Equal(t, tt.want, shard.ID)
		})
	}
}

func TestTaskListDifferentiator_ReadsDynamicConfigOnEveryCall(t *testing.T) {
	shardCount := 1000
	taskListName := "orders" // farm.Fingerprint64([]byte(testDomainID+"/orders"))%1000 = 134
	hashedShardID := "134"

	shardTaskListsByHash := false
	percentageOnboarded := 0

	differentiator := NewTaskListDifferentiator(
		metrics.NewNoopMetricsClient(),
		dynamicproperties.GetBoolPropertyFn(false),
		func(...dynamicproperties.FilterOption) bool { return shardTaskListsByHash },
		func(...dynamicproperties.FilterOption) int { return percentageOnboarded },
		shardCount,
	)

	assert.False(t, differentiator.ShardsByHash())
	assert.Equal(t, TaskListShard{ID: taskListName, Onboarded: false}, differentiator.TaskListShard(testDomainID, taskListName))

	shardTaskListsByHash = true
	percentageOnboarded = 100

	assert.True(t, differentiator.ShardsByHash())
	assert.Equal(t, TaskListShard{ID: hashedShardID, Onboarded: true}, differentiator.TaskListShard(testDomainID, taskListName))
}

func TestTaskListDifferentiator_ShardDistributorEnabled(t *testing.T) {
	tests := []struct {
		name                string
		percentageOnboarded int
		want                bool
	}{
		{name: "offboarded at zero", percentageOnboarded: 0, want: false},
		{name: "onboarded above zero", percentageOnboarded: 1, want: true},
		{name: "onboarded at full", percentageOnboarded: 100, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testScope := tally.NewTestScope("", nil)
			metricsClient := metrics.NewClient(testScope, metrics.Matching, metrics.MigrationConfig{})
			differentiator := NewTaskListDifferentiator(
				metricsClient,
				dynamicproperties.GetBoolPropertyFn(false),
				dynamicproperties.GetBoolPropertyFn(true),
				dynamicproperties.GetIntPropertyFn(tt.percentageOnboarded),
				1000,
			)

			assert.Equal(t, tt.want, differentiator.ShardDistributorEnabled())

			gauges := testScope.Snapshot().Gauges()
			require.Len(t, gauges, 1, "reading the percentage should report it as a single gauge")
			for _, gauge := range gauges {
				assert.Equal(t, "percentage_onboarded_to_shard_manager", gauge.Name())
				assert.Equal(t, float64(tt.percentageOnboarded), gauge.Value())
			}
		})
	}
}
