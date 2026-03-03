package membership

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskListExcludedFromShardDistributor(t *testing.T) {
	tests := []struct {
		name         string
		taskListName string
		want         bool
	}{
		{
			name:         "task list with UUID",
			taskListName: "tasklist-550e8400-e29b-41d4-a716-446655440000",
			want:         true,
		},
		{
			name:         "task list with uppercase UUID",
			taskListName: "tasklist-550E8400-E29B-41D4-A716-446655440000",
			want:         true,
		},
		{
			name:         "task list name is UUID only",
			taskListName: "550e8400-e29b-41d4-a716-446655440000",
			want:         true,
		},
		{
			name:         "task list without UUID",
			taskListName: "my-task-list",
			want:         false,
		},
		{
			name:         "empty task list name",
			taskListName: "",
			want:         false,
		},
		{
			name:         "task list with partial UUID-like string",
			taskListName: "tasklist-550e8400-e29b",
			want:         false,
		},
		{
			name:         "task list with UUID prefix",
			taskListName: "550e8400-e29b-41d4-a716-446655440000-suffix",
			want:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TaskListExcludedFromShardDistributor(tt.taskListName)
			assert.Equal(t, tt.want, got)
		})
	}
}
