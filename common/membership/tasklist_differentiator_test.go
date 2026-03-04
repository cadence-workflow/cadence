package membership

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskListExcludedFromShardDistributor(t *testing.T) {
	tests := []struct {
		name                string
		taskListName        string
		want                bool
		percentageOnboarded uint64
	}{
		{
			name:                "task list with UUID",
			taskListName:        "tasklist-550e8400-e29b-41d4-a716-446655440000",
			want:                true,
			percentageOnboarded: 100,
		},
		{
			name:                "task list with uppercase UUID",
			taskListName:        "tasklist-550E8400-E29B-41D4-A716-446655440000",
			want:                true,
			percentageOnboarded: 100,
		},
		{
			name:                "task list name is UUID only",
			taskListName:        "550e8400-e29b-41d4-a716-446655440000",
			want:                true,
			percentageOnboarded: 100,
		},
		{
			name:                "task list without UUID",
			taskListName:        "my-task-list",
			want:                false,
			percentageOnboarded: 100,
		},
		{
			name:                "empty task list name",
			taskListName:        "",
			want:                false,
			percentageOnboarded: 100,
		},
		{
			name:                "task list with partial UUID-like string",
			taskListName:        "tasklist-550e8400-e29b",
			want:                false,
			percentageOnboarded: 100,
		},
		{
			name:                "task list with UUID prefix",
			taskListName:        "550e8400-e29b-41d4-a716-446655440000-suffix",
			want:                true,
			percentageOnboarded: 100,
		},
		{
			name:                "task list below percentage",
			taskListName:        "my-task-list", // farm.Fingerprint64([]byte("my-task-list"))%100 = 58
			want:                false,
			percentageOnboarded: 60,
		},
		{
			name:                "task list above percentage",
			taskListName:        "my-task-list", // farm.Fingerprint64([]byte("my-task-list"))%100 = 58
			want:                true,
			percentageOnboarded: 50,
		},
		{
			name:                "task list below percentage - but UUID",
			taskListName:        "my-task-list-550e8400-e29b-41d4-a716-446655440003", // farm.Fingerprint64([]byte("my-task-list-550e8400-e29b-41d4-a716-446655440003"))%100 = 69
			want:                true,
			percentageOnboarded: 70,
		},
		{
			name:                "task list above percentage - but UUID",
			taskListName:        "my-task-list-550e8400-e29b-41d4-a716-446655440003", // farm.Fingerprint64([]byte("my-task-list-550e8400-e29b-41d4-a716-446655440003"))%100 = 69
			want:                true,
			percentageOnboarded: 60,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TaskListExcludedFromShardDistributor(tt.taskListName, tt.percentageOnboarded)
			assert.Equal(t, tt.want, got)
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
