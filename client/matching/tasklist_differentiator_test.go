package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
)

func TestTaskListDifferentiator_TaskListShard_Onboarded(t *testing.T) {
	shardCount := 1000

	tests := []struct {
		name                       string
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
			taskListName:         "orders", // farm.Fingerprint64([]byte("orders"))%1000 = 639
			wantOnboarded:        true,
			percentageOnboarded:  64,
			shardTaskListsByHash: true,
		},
		{
			// The task list's own hash%100 is 39, so only the shard puts it above the percentage.
			name:                 "sharded by hash - shard above percentage",
			taskListName:         "orders", // farm.Fingerprint64([]byte("orders"))%1000 = 639
			wantOnboarded:        false,
			percentageOnboarded:  63,
			shardTaskListsByHash: true,
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
				dynamicproperties.GetBoolPropertyFn(tt.excludeShortLivedTaskLists),
				dynamicproperties.GetBoolPropertyFn(tt.shardTaskListsByHash),
				StaticPercentageOnboarded(tt.percentageOnboarded),
				shardCount,
			)

			shard := differentiator.TaskListShard(tt.taskListName)
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
		taskListName         string
		shardTaskListsByHash bool
		want                 string
	}{
		{
			name:                 "shard by task list name",
			taskListName:         "orders",
			shardTaskListsByHash: false,
			want:                 "orders",
		},
		{
			name:                 "shard by hash",
			taskListName:         "orders", // farm.Fingerprint64([]byte("orders"))%1000 = 639
			shardTaskListsByHash: true,
			want:                 "639",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			differentiator := NewTaskListDifferentiator(
				dynamicproperties.GetBoolPropertyFn(false),
				dynamicproperties.GetBoolPropertyFn(tt.shardTaskListsByHash),
				StaticPercentageOnboarded(100),
				shardCount,
			)

			shard := differentiator.TaskListShard(tt.taskListName)
			assert.Equal(t, tt.want, shard.ID)
		})
	}
}

func TestTaskListDifferentiator_ReadsDynamicConfigOnEveryCall(t *testing.T) {
	shardCount := 1000
	taskListName := "orders" // farm.Fingerprint64([]byte("orders"))%1000 = 639
	hashedShardID := "639"

	shardTaskListsByHash := false
	percentageOnboarded := 0
	pct := NewMockPercentageOnboarded(gomock.NewController(t))
	pct.EXPECT().Value().DoAndReturn(func() int { return percentageOnboarded }).AnyTimes()

	differentiator := NewTaskListDifferentiator(
		dynamicproperties.GetBoolPropertyFn(false),
		func(...dynamicproperties.FilterOption) bool { return shardTaskListsByHash },
		pct,
		shardCount,
	)

	assert.False(t, differentiator.ShardsByHash())
	assert.Equal(t, TaskListShard{ID: taskListName, Onboarded: false}, differentiator.TaskListShard(taskListName))

	shardTaskListsByHash = true
	percentageOnboarded = 100

	assert.True(t, differentiator.ShardsByHash())
	assert.Equal(t, TaskListShard{ID: hashedShardID, Onboarded: true}, differentiator.TaskListShard(taskListName))
}
