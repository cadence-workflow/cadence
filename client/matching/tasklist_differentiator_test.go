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
	testDomainID      = "e58e058d-5156-40ab-9a48-eca5ba55d0c5"
	testOtherDomainID = "291f4b66-2a7f-4289-8583-0355cae40a06"
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
			name:                 "sharded by hash - shard below percentage",
			domainID:             testDomainID,
			taskListName:         "orders", // farm.Fingerprint64([]byte(testDomainID+"/orders"))%1000 = 127
			wantOnboarded:        true,
			percentageOnboarded:  14,
			shardTaskListsByHash: true,
		},
		{
			name:                 "sharded by hash - shard above percentage",
			domainID:             testOtherDomainID,
			taskListName:         "test-key", // farm.Fingerprint64([]byte(testOtherDomainID+"/test-key"))%1000 = 942
			wantOnboarded:        false,
			percentageOnboarded:  70,
			shardTaskListsByHash: true,
		},
		{
			name:                       "sharded by hash - domain ID is a UUID - not excluded as short lived",
			domainID:                   testDomainID,
			taskListName:               "orders",
			wantOnboarded:              true,
			percentageOnboarded:        100,
			excludeShortLivedTaskLists: true,
			shardTaskListsByHash:       true,
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

			_, onboardedToShardDistributor := differentiator.TaskListShard(tt.domainID, tt.taskListName).ShardID()
			assert.Equal(t, tt.wantOnboarded, onboardedToShardDistributor)
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
		percentageOnboarded  int
		wantShardID          string
		wantOnboarded        bool
	}{
		{
			name:                 "shard by task list name",
			domainID:             testDomainID,
			taskListName:         "orders",
			shardTaskListsByHash: false,
			percentageOnboarded:  100,
			wantShardID:          "orders",
			wantOnboarded:        true,
		},
		{
			name:                 "shard by task list name is shared by every domain",
			domainID:             testOtherDomainID,
			taskListName:         "orders",
			shardTaskListsByHash: false,
			percentageOnboarded:  100,
			wantShardID:          "orders",
			wantOnboarded:        true,
		},
		{
			name:                 "shard by hash",
			domainID:             testDomainID,
			taskListName:         "orders", // farm.Fingerprint64([]byte(testDomainID+"/orders"))%1000 = 127
			shardTaskListsByHash: true,
			percentageOnboarded:  100,
			wantShardID:          "127",
			wantOnboarded:        true,
		},
		{
			name:                 "shard by hash puts the same name in another domain in another shard",
			domainID:             testOtherDomainID,
			taskListName:         "orders", // farm.Fingerprint64([]byte(testOtherDomainID+"/orders"))%1000 = 208
			shardTaskListsByHash: true,
			percentageOnboarded:  100,
			wantShardID:          "208",
			wantOnboarded:        true,
		},
		{
			name:                 "shard by hash without a domain ID",
			domainID:             "",
			taskListName:         "orders", // farm.Fingerprint64([]byte("/orders"))%1000 = 469
			shardTaskListsByHash: true,
			percentageOnboarded:  100,
			wantShardID:          "469",
			wantOnboarded:        true,
		},
		{
			name:                 "shard by task list name - not onboarded has no shard ID",
			domainID:             testDomainID,
			taskListName:         "orders",
			shardTaskListsByHash: false,
			percentageOnboarded:  0,
			wantShardID:          "",
			wantOnboarded:        false,
		},
		{
			name:                 "shard by hash - not onboarded has no shard ID",
			domainID:             testDomainID,
			taskListName:         "orders",
			shardTaskListsByHash: true,
			percentageOnboarded:  0,
			wantShardID:          "",
			wantOnboarded:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			differentiator := NewTaskListDifferentiator(
				metrics.NewNoopMetricsClient(),
				dynamicproperties.GetBoolPropertyFn(false),
				dynamicproperties.GetBoolPropertyFn(tt.shardTaskListsByHash),
				dynamicproperties.GetIntPropertyFn(tt.percentageOnboarded),
				shardCount,
			)

			shardID, onboardedToShardDistributor := differentiator.TaskListShard(tt.domainID, tt.taskListName).ShardID()
			assert.Equal(t, tt.wantShardID, shardID)
			assert.Equal(t, tt.wantOnboarded, onboardedToShardDistributor)
		})
	}
}

func TestTaskListDifferentiator_ReadsDynamicConfigOnEveryCall(t *testing.T) {
	shardCount := 1000
	taskListName := "orders" // farm.Fingerprint64([]byte(testDomainID+"/orders"))%1000 = 127
	hashedShardID := "127"

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
	assert.Equal(t, TaskListShard{id: "", onboardedToShardDistributor: false}, differentiator.TaskListShard(testDomainID, taskListName))

	shardTaskListsByHash = true
	percentageOnboarded = 100

	assert.True(t, differentiator.ShardsByHash())
	assert.Equal(t, TaskListShard{id: hashedShardID, onboardedToShardDistributor: true}, differentiator.TaskListShard(testDomainID, taskListName))
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
