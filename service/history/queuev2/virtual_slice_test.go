package queuev2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/task"
)

func TestMergeProgressWithSamePredicate(t *testing.T) {
	tests := []struct {
		name     string
		left     *GetTaskProgress
		right    *GetTaskProgress
		expected []*GetTaskProgress
	}{
		{
			name: "Case 1: [a,b,c,x,y,z] - Non-overlapping ranges",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(2),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(5),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Case 2: [a,b,x,c,y,z] - Partially overlapping ranges",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(2),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(5),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Case 3: [a,b,x,y,c,z] - Overlapping ranges with interleaved keys",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(2),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(4),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(4),
				},
			},
		},
		{
			name: "Case 4: [a,b,x,y,z,c] - Overlapping ranges with right extending beyond",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(2),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(4),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(4),
				},
			},
		},
		{
			name: "Case 5: [a,x,b,c,y,z] - Left range contains right range",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(3),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(3),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(3),
				},
			},
		},
		{
			name: "Case 6: [a,x,b,y,c,z] - Overlapping ranges with interleaved keys",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(3),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(4),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(4),
				},
			},
		},
		{
			name: "Case 7: [a,x,b,y,z,c] - Overlapping ranges with left extending beyond",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(3),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(4),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(4),
				},
			},
		},
		{
			name: "Case 8: [a,x,y,b,c,z] - Right range contains left range",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(4),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(3),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(4),
				},
			},
		},
		{
			name: "Case 9: [a,x,y,b,z,c] - Overlapping ranges with right extending beyond",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(4),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(3),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(4),
				},
			},
		},
		{
			name: "Case 10: [a,x,y,z,b,c] - Right range completely contains left range",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(5),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(3),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeProgressWithSamePredicate(tt.left, tt.right)
			assert.Equal(t, tt.expected, result)

			result = mergeProgressWithSamePredicate(tt.right, tt.left)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppendOrMergeProgressWithSamePredicate(t *testing.T) {
	tests := []struct {
		name           string
		mergedProgress []*GetTaskProgress
		progress       *GetTaskProgress
		expected       []*GetTaskProgress
	}{
		{
			name:           "Empty slice - should append",
			mergedProgress: []*GetTaskProgress{},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(2),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
		},
		{
			name: "Non-overlapping ranges - should append",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(5),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Overlapping ranges - should merge",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(5),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Contained ranges - should merge",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(3),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(3),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(3),
				},
			},
		},
		{
			name: "Multiple existing progress - should merge with last",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
				},
				NextTaskKey: persistence.NewImmediateTaskKey(6),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(6),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendOrMergeProgressWithSamePredicate(tt.mergedProgress, tt.progress)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeGetTaskProgress(t *testing.T) {
	tests := []struct {
		name     string
		left     []*GetTaskProgress
		right    []*GetTaskProgress
		expected []*GetTaskProgress
	}{
		{
			name:     "Empty slices",
			left:     []*GetTaskProgress{},
			right:    []*GetTaskProgress{},
			expected: []*GetTaskProgress{},
		},
		{
			name: "Left empty, right non-empty",
			left: []*GetTaskProgress{},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
		},
		{
			name: "Left non-empty, right empty",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
			right: []*GetTaskProgress{},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
		},
		{
			name: "Non-overlapping ranges",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Overlapping ranges",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Multiple progress items with overlaps",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(3),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(6),
				},
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(3),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(6),
				},
			},
		},
		{
			name: "Contained ranges",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(3),
				},
			},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(3),
				},
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(3),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeGetTaskProgressWithSamePredicate(tt.left, tt.right)
			assert.Equal(t, tt.expected, result)

			result = mergeGetTaskProgressWithSamePredicate(tt.right, tt.left)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeVirtualSlicesByRange(t *testing.T) {
	tests := []struct {
		name     string
		left     *virtualSliceImpl
		right    *virtualSliceImpl
		expected VirtualSliceState
	}{
		{
			name: "Non-overlapping ranges",
			left: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			right: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expected: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				Predicate: NewUniversalPredicate(),
			},
		},
		{
			name: "Overlapping ranges",
			left: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			right: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expected: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				Predicate: NewUniversalPredicate(),
			},
		},
		{
			name: "Contained ranges",
			left: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			right: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expected: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				Predicate: NewUniversalPredicate(),
			},
		},
		{
			name: "Identical ranges",
			left: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			right: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expected: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
				},
				Predicate: NewUniversalPredicate(),
			},
		},
		{
			name: "Adjacent ranges",
			left: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			right: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expected: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				Predicate: NewUniversalPredicate(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockPendingTaskTracker0 := NewMockPendingTaskTracker(ctrl)
			mockPendingTaskTracker1 := NewMockPendingTaskTracker(ctrl)
			mockPendingTaskTracker1.EXPECT().GetTasks().Return(map[persistence.HistoryTaskKey]task.Task{
				persistence.NewImmediateTaskKey(1): task.NewMockTask(ctrl),
				persistence.NewImmediateTaskKey(2): task.NewMockTask(ctrl),
			})
			mockPendingTaskTracker0.EXPECT().AddTask(gomock.Any()).Times(2)
			tt.left.pendingTaskTracker = mockPendingTaskTracker0
			tt.right.pendingTaskTracker = mockPendingTaskTracker1
			result := mergeVirtualSlicesByRange(tt.left, tt.right)
			assert.Equal(t, tt.expected, result.GetState())
		})
	}
}

func TestTrySplitByTaskKey(t *testing.T) {
	tests := []struct {
		name                  string
		slice                 *virtualSliceImpl
		splitKey              persistence.HistoryTaskKey
		expectedLeft          VirtualSliceState
		expectedRight         VirtualSliceState
		expectedOk            bool
		expectedLeftProgress  []*GetTaskProgress
		expectedRightProgress []*GetTaskProgress
	}{
		{
			name: "Split at middle of range",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
			},
			splitKey: persistence.NewImmediateTaskKey(3),
			expectedLeft: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedRight: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedOk: true,
			expectedLeftProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
			expectedRightProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(3),
				},
			},
		},
		{
			name: "Split at middle of range with multiple progress items",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(30),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(20),
						NextPageToken: []byte{1, 2, 3},
					},
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(30),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(60),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(40),
						NextPageToken: []byte{4, 5, 6},
					},
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(60),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(70),
						NextPageToken: []byte{7, 8, 9},
					},
				},
			},
			splitKey: persistence.NewImmediateTaskKey(50),
			expectedLeft: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(50),
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedRight: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(50),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedOk: true,
			expectedLeftProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(30),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(20),
					NextPageToken: []byte{1, 2, 3},
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(40),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(50),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(40),
				},
			},
			expectedRightProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(50),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(60),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(50),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(60),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(70),
					NextPageToken: []byte{7, 8, 9},
				},
			},
		},
		{
			name: "Split at middle of range with multiple progress items - 2",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(30),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(20),
						NextPageToken: []byte{1, 2, 3},
					},
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(30),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(60),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(55),
						NextPageToken: []byte{4, 5, 6},
					},
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(60),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(70),
						NextPageToken: []byte{7, 8, 9},
					},
				},
			},
			splitKey: persistence.NewImmediateTaskKey(50),
			expectedLeft: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(50),
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedRight: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(50),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
				},
				Predicate: NewUniversalPredicate(),
			},
			expectedOk: true,
			expectedLeftProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(30),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(20),
					NextPageToken: []byte{1, 2, 3},
				},
			},
			expectedRightProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(55),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(60),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(55),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(60),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(70),
					NextPageToken: []byte{7, 8, 9},
				},
			},
		},
		{
			name: "Split key outside range - should fail",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
			},
			splitKey:   persistence.NewImmediateTaskKey(6),
			expectedOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueueReader := NewMockQueueReader(ctrl)

			tt.slice.taskInitializer = func(t persistence.Task) task.Task {
				mockTask := task.NewMockTask(ctrl)
				return mockTask
			}
			tt.slice.queueReader = mockQueueReader
			tt.slice.pendingTaskTracker = NewPendingTaskTracker()

			left, right, ok := tt.slice.TrySplitByTaskKey(tt.splitKey)
			assert.Equal(t, tt.expectedOk, ok)

			if ok {
				assert.Equal(t, tt.expectedLeft, left.GetState())
				assert.Equal(t, tt.expectedRight, right.GetState())

				// Verify progress
				leftImpl, ok := left.(*virtualSliceImpl)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedLeftProgress, leftImpl.progress)

				rightImpl, ok := right.(*virtualSliceImpl)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedRightProgress, rightImpl.progress)
			} else {
				assert.Nil(t, left)
				assert.Nil(t, right)
			}
		})
	}
}

func TestUpdateAndGetState(t *testing.T) {
	tests := []struct {
		name          string
		slice         *virtualSliceImpl
		expectedState VirtualSliceState
		setupMock     func(*MockPendingTaskTracker)
	}{
		{
			name: "No pending tasks, no more tasks to read",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expectedState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				Predicate: NewUniversalPredicate(),
			},
			setupMock: func(mock *MockPendingTaskTracker) {
				mock.EXPECT().PruneAckedTasks()
				mock.EXPECT().GetMinimumTaskKey().Return(persistence.MaximumHistoryTaskKey, false)
			},
		},
		{
			name: "No pending tasks, has more tasks to read",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(3),
					},
				},
			},
			expectedState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				Predicate: NewUniversalPredicate(),
			},
			setupMock: func(mock *MockPendingTaskTracker) {
				mock.EXPECT().PruneAckedTasks()
				mock.EXPECT().GetMinimumTaskKey().Return(persistence.MaximumHistoryTaskKey, false)
			},
		},
		{
			name: "Has pending tasks, no more tasks to read",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
			},
			expectedState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				Predicate: NewUniversalPredicate(),
			},
			setupMock: func(mock *MockPendingTaskTracker) {
				mock.EXPECT().PruneAckedTasks()
				mock.EXPECT().GetMinimumTaskKey().Return(persistence.NewImmediateTaskKey(2), true)
			},
		},
		{
			name: "Has pending tasks, has more tasks to read",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(3),
					},
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
				},
			},
			expectedState: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
				},
				Predicate: NewUniversalPredicate(),
			},
			setupMock: func(mock *MockPendingTaskTracker) {
				mock.EXPECT().PruneAckedTasks()
				mock.EXPECT().GetMinimumTaskKey().Return(persistence.NewImmediateTaskKey(5), true)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueueReader := NewMockQueueReader(ctrl)
			mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)

			tt.slice.queueReader = mockQueueReader
			tt.slice.pendingTaskTracker = mockPendingTaskTracker
			tt.slice.logger = testlogger.New(t)

			// Setup mock expectations using the setupMock function
			tt.setupMock(mockPendingTaskTracker)

			result := tt.slice.UpdateAndGetState()
			assert.Equal(t, tt.expectedState, result)
		})
	}
}

func TestGetTasks(t *testing.T) {
	historyTasks := []persistence.Task{
		&persistence.DecisionTask{},
		&persistence.ActivityTask{},
		&persistence.ActivityTask{},
	}
	tests := []struct {
		name               string
		slice              *virtualSliceImpl
		pageSize           int
		setupMock          func(*MockQueueReader, *MockPendingTaskTracker)
		expectedTasksCount int
		expectedError      error
		expectedProgress   []*GetTaskProgress
	}{
		{
			name: "Empty progress - should return empty tasks",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{},
			},
			pageSize: 10,
			setupMock: func(mockQueueReader *MockQueueReader, mockPendingTaskTracker *MockPendingTaskTracker) {
				// No expectations needed as progress is empty
			},
			expectedTasksCount: 0,
			expectedError:      nil,
			expectedProgress:   []*GetTaskProgress{},
		},
		{
			name: "Single page of tasks",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(1),
					},
				},
			},
			pageSize: 2,
			setupMock: func(mockQueueReader *MockQueueReader, mockPendingTaskTracker *MockPendingTaskTracker) {
				mockQueueReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(1),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  2,
				}).Return(&GetTaskResponse{
					Tasks: []persistence.Task{historyTasks[0], historyTasks[1]},
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(3),
						NextPageToken: []byte("token"),
					},
				}, nil)

				mockPendingTaskTracker.EXPECT().AddTask(gomock.Any()).Times(2)
			},
			expectedTasksCount: 2,
			expectedError:      nil,
			expectedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
					NextPageToken: []byte("token"),
				},
			},
		},
		{
			name: "Multiple pages of tasks",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(3),
						NextPageToken: []byte("token"),
					},
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(5),
					},
				},
			},
			pageSize: 3,
			setupMock: func(mockQueueReader *MockQueueReader, mockPendingTaskTracker *MockPendingTaskTracker) {
				// First page
				mockQueueReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(3),
						NextPageToken: []byte("token"),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  3,
				}).Return(&GetTaskResponse{
					Tasks: []persistence.Task{historyTasks[0], historyTasks[1]},
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(5),
					},
				}, nil)

				// Second page
				mockQueueReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  1,
				}).Return(&GetTaskResponse{
					Tasks: []persistence.Task{historyTasks[2]},
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(6),
						NextPageToken: []byte("token"),
					},
				}, nil)

				mockPendingTaskTracker.EXPECT().AddTask(gomock.Any()).Times(3)
			},
			expectedTasksCount: 3,
			expectedError:      nil,
			expectedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(6),
					NextPageToken: []byte("token"),
				},
			},
		},
		{
			name: "Error from queue reader",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(1),
					},
				},
			},
			pageSize: 2,
			setupMock: func(mockQueueReader *MockQueueReader, mockPendingTaskTracker *MockPendingTaskTracker) {
				mockQueueReader.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			expectedTasksCount: 0,
			expectedError:      assert.AnError,
		},
		{
			name: "Single page of tasks with error from queue reader",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(1),
					},
				},
			},
			pageSize: 2,
			setupMock: func(mockQueueReader *MockQueueReader, mockPendingTaskTracker *MockPendingTaskTracker) {
				mockQueueReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(1),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  2,
				}).Return(&GetTaskResponse{
					Tasks: []persistence.Task{historyTasks[0]},
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(3),
						NextPageToken: []byte("token"),
					},
				}, nil)
				mockPendingTaskTracker.EXPECT().AddTask(gomock.Any()).Times(1)
				mockQueueReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(3),
						NextPageToken: []byte("token"),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  1,
				}).Return(nil, assert.AnError)
			},
			expectedTasksCount: 1,
			expectedError:      nil,
			expectedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
					NextPageToken: []byte("token"),
				},
			},
		},
		{
			name: "Multiple pages of tasks with error from queue reader",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(3),
						NextPageToken: []byte("token"),
					},
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(5),
					},
				},
			},
			pageSize: 3,
			setupMock: func(mockQueueReader *MockQueueReader, mockPendingTaskTracker *MockPendingTaskTracker) {
				// First page
				mockQueueReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(3),
						NextPageToken: []byte("token"),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  3,
				}).Return(&GetTaskResponse{
					Tasks: []persistence.Task{historyTasks[0], historyTasks[1]},
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(5),
					},
				}, nil)

				// Second page
				mockQueueReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  1,
				}).Return(nil, assert.AnError)

				mockPendingTaskTracker.EXPECT().AddTask(gomock.Any()).Times(2)
			},
			expectedTasksCount: 2,
			expectedError:      nil,
			expectedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Single page of tasks with next task key equal to exclusive max task key but non-empty next page token",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(1),
					},
				},
			},
			pageSize: 2,
			setupMock: func(mockQueueReader *MockQueueReader, mockPendingTaskTracker *MockPendingTaskTracker) {
				mockQueueReader.EXPECT().GetTask(gomock.Any(), &GetTaskRequest{
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(1),
					},
					Predicate: NewUniversalPredicate(),
					PageSize:  2,
				}).Return(&GetTaskResponse{
					Tasks: []persistence.Task{historyTasks[0]},
					Progress: &GetTaskProgress{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(5),
						NextPageToken: []byte("token"),
					},
				}, nil)
				mockPendingTaskTracker.EXPECT().AddTask(gomock.Any()).Times(1)
			},
			expectedTasksCount: 1,
			expectedError:      nil,
			expectedProgress:   []*GetTaskProgress{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueueReader := NewMockQueueReader(ctrl)
			mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)

			tt.slice.queueReader = mockQueueReader
			tt.slice.pendingTaskTracker = mockPendingTaskTracker
			tt.slice.taskInitializer = func(t persistence.Task) task.Task {
				return task.NewMockTask(ctrl)
			}

			// Setup mock expectations
			tt.setupMock(mockQueueReader, mockPendingTaskTracker)

			tasks, err := tt.slice.GetTasks(context.Background(), tt.pageSize)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, tasks, tt.expectedTasksCount)
			}
		})
	}
}

func TestMergeProgressWithDifferentPredicate(t *testing.T) {
	tests := []struct {
		name     string
		left     *GetTaskProgress
		right    *GetTaskProgress
		expected []*GetTaskProgress
	}{
		{
			name: "Case 1: Non-overlapping ranges",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(2),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(5),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Case 1,2: Adjacent ranges",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(2),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(5),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Case 2: Overlapping ranges",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(2),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(5),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Case 3: Adjacent ranges",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(2),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: nil,
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
				},
			},
		},
		{
			name: "Case 3: Overlapping ranges",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(2),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(4),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
				},
			},
		},
		{
			name: "Case 4: Completely overlapping ranges - left contains right",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(5),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
				},
			},
		},
		{
			name: "Case 3,4: Equal exclusive max task keys",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(2),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(4),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
				},
			},
		},
		{
			name: "Case 5: Overlapping ranges",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(5),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
				},
			},
		},
		{
			name: "Case 6: Overlapping ranges with same next task key",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
				},
			},
		},
		{
			name: "Case 7: Completely overlapping ranges - left contains right",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
				},
			},
		},
		{
			name: "Identical ranges and next task keys",
			left: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: []byte("token1"),
			},
			right: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// make sure that these test inputs are valid
			if tt.left.NextTaskKey.Compare(tt.left.Range.InclusiveMinTaskKey) == 0 {
				require.Nil(t, tt.left.NextPageToken)
			} else {
				require.NotNil(t, tt.left.NextPageToken)
			}
			if tt.right.NextTaskKey.Compare(tt.right.Range.InclusiveMinTaskKey) == 0 {
				require.Nil(t, tt.right.NextPageToken)
			} else {
				require.NotNil(t, tt.right.NextPageToken)
			}

			// test the merge function
			result := mergeProgressWithDifferentPredicate(tt.left, tt.right)
			assert.Equal(t, tt.expected, result)

			// Test commutativity - the function should handle argument order internally
			result2 := mergeProgressWithDifferentPredicate(tt.right, tt.left)
			assert.Equal(t, tt.expected, result2)
		})
	}
}

func TestAppendOrMergeProgressWithDifferentPredicate(t *testing.T) {
	tests := []struct {
		name           string
		mergedProgress []*GetTaskProgress
		progress       *GetTaskProgress
		expected       []*GetTaskProgress
	}{
		{
			name:           "Empty slice - should append",
			mergedProgress: []*GetTaskProgress{},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(2),
				NextPageToken: []byte("token"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token"),
				},
			},
		},
		{
			name: "Non-overlapping ranges - should append both with reset tokens",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token1"),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(5),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: nil,
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
					NextPageToken: nil,
				},
			},
		},
		{
			name: "Adjacent ranges - should merge into single range",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token1"),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: nil,
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: nil,
				},
			},
		},
		{
			name: "Overlapping ranges - should merge with earliest next task key",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token1"),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(5),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: nil,
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
					NextPageToken: nil,
				},
			},
		},
		{
			name: "Completely contained ranges - should merge to outer range",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
					NextPageToken: []byte("token1"),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(5),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
					NextPageToken: nil,
				},
			},
		},
		{
			name: "Same next task key - different ranges",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
					NextPageToken: []byte("token1"),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(3),
				NextPageToken: []byte("token2"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
					NextPageToken: nil,
				},
			},
		},
		{
			name: "Multiple existing progress - should merge with last only",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token1"),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
					NextPageToken: []byte("token2"),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(6),
				NextPageToken: []byte("token3"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token1"),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
					NextPageToken: nil,
				},
			},
		},
		{
			name: "Multiple existing progress - last range non-overlapping",
			mergedProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token1"),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
					NextPageToken: []byte("token2"),
				},
			},
			progress: &GetTaskProgress{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(8),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
				},
				NextTaskKey:   persistence.NewImmediateTaskKey(9),
				NextPageToken: []byte("token3"),
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token1"),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
					NextPageToken: nil,
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(9),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(9),
					NextPageToken: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendOrMergeProgressWithDifferentPredicate(tt.mergedProgress, tt.progress)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeGetTaskProgressWithDifferentPredicate(t *testing.T) {
	tests := []struct {
		name     string
		left     []*GetTaskProgress
		right    []*GetTaskProgress
		expected []*GetTaskProgress
	}{
		{
			name:     "Empty slices",
			left:     []*GetTaskProgress{},
			right:    []*GetTaskProgress{},
			expected: []*GetTaskProgress{},
		},
		{
			name: "Left empty, right non-empty",
			left: []*GetTaskProgress{},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token"),
				},
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token"),
				},
			},
		},
		{
			name: "Left non-empty, right empty",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token"),
				},
			},
			right: []*GetTaskProgress{},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token"),
				},
			},
		},
		{
			name: "Non-overlapping ranges - ordered by NextTaskKey",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token1"),
				},
			},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
					NextPageToken: []byte("token2"),
				},
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: nil,
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
					NextPageToken: nil,
				},
			},
		},
		{
			name: "Adjacent ranges by NextTaskKey",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token1"),
				},
			},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(4), // comes after left's range
					NextPageToken: []byte("token2"),
				},
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: nil,
				},
			},
		},
		{
			name: "Multiple progress items - complex merge by NextTaskKey",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: []byte("token1"),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(7),
					NextPageToken: []byte("token2"),
				},
			},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(4),
					NextPageToken: []byte("token3"),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(8),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(9),
					NextPageToken: []byte("token4"),
				},
			},
			expected: []*GetTaskProgress{
				// Merged based on NextTaskKey order: 2, 4, 7, 9
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(2),
					NextPageToken: nil,
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(4),
					NextPageToken: nil,
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(7),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(7),
					NextPageToken: nil,
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(9),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(9),
					NextPageToken: nil,
				},
			},
		},
		{
			name: "Overlapping ranges with interleaved NextTaskKeys",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
					NextPageToken: []byte("token1"),
				},
			},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(5),
					NextPageToken: []byte("token2"),
				},
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
					NextPageToken: nil,
				},
			},
		},
		{
			name: "Same NextTaskKey - should merge ranges",
			left: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(4),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
					NextPageToken: []byte("token1"),
				},
			},
			right: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3), // Same NextTaskKey
					NextPageToken: []byte("token2"),
				},
			},
			expected: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(3),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(3),
					NextPageToken: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeGetTaskProgressWithDifferentPredicate(tt.left, tt.right)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVirtualSliceImpl_TrySplitByPredicate(t *testing.T) {
	tests := []struct {
		name                  string
		slice                 *virtualSliceImpl
		splitPredicate        Predicate
		expectedLeft          VirtualSliceState
		expectedRight         VirtualSliceState
		expectedOk            bool
		expectedLeftProgress  []*GetTaskProgress
		expectedRightProgress []*GetTaskProgress
	}{
		{
			name: "Universal predicate should not split",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
			},
			splitPredicate: NewUniversalPredicate(),
			expectedOk:     false,
		},
		{
			name: "Empty predicate should not split",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
			},
			splitPredicate: NewEmptyPredicate(),
			expectedOk:     false,
		},
		{
			name: "Identical predicate should not split",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
			},
			splitPredicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			expectedOk:     false,
		},
		{
			name: "Different predicate should split successfully",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
			},
			splitPredicate: NewDomainIDPredicate([]string{"domain3"}, false),
			expectedOk:     true,
			expectedLeft: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				Predicate: And(NewDomainIDPredicate([]string{"domain1", "domain2"}, false), NewDomainIDPredicate([]string{"domain3"}, false)),
			},
			expectedRight: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
				},
				Predicate: And(NewDomainIDPredicate([]string{"domain1", "domain2"}, false), Not(NewDomainIDPredicate([]string{"domain3"}, false))),
			},
			expectedLeftProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
			expectedRightProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					NextTaskKey: persistence.NewImmediateTaskKey(2),
				},
			},
		},
		{
			name: "Split with multiple progress items",
			slice: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
					},
					Predicate: NewUniversalPredicate(),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(30),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(20),
						NextPageToken: []byte{1, 2, 3},
					},
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(30),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(60),
						},
						NextTaskKey:   persistence.NewImmediateTaskKey(40),
						NextPageToken: []byte{4, 5, 6},
					},
				},
			},
			splitPredicate: NewDomainIDPredicate([]string{"domain1"}, false),
			expectedOk:     true,
			expectedLeft: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
				},
				Predicate: And(NewUniversalPredicate(), NewDomainIDPredicate([]string{"domain1"}, false)),
			},
			expectedRight: VirtualSliceState{
				Range: Range{
					InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
					ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(100),
				},
				Predicate: And(NewUniversalPredicate(), Not(NewDomainIDPredicate([]string{"domain1"}, false))),
			},
			expectedLeftProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(30),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(20),
					NextPageToken: []byte{1, 2, 3},
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(30),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(60),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(40),
					NextPageToken: []byte{4, 5, 6},
				},
			},
			expectedRightProgress: []*GetTaskProgress{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(30),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(20),
					NextPageToken: []byte{1, 2, 3},
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(30),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(60),
					},
					NextTaskKey:   persistence.NewImmediateTaskKey(40),
					NextPageToken: []byte{4, 5, 6},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueueReader := NewMockQueueReader(ctrl)

			tt.slice.taskInitializer = func(t persistence.Task) task.Task {
				mockTask := task.NewMockTask(ctrl)
				return mockTask
			}
			tt.slice.queueReader = mockQueueReader
			tt.slice.pendingTaskTracker = NewPendingTaskTracker()

			left, right, ok := tt.slice.TrySplitByPredicate(tt.splitPredicate)
			assert.Equal(t, tt.expectedOk, ok)

			if ok {
				assert.Equal(t, tt.expectedLeft, left.GetState())
				assert.Equal(t, tt.expectedRight, right.GetState())

				// Verify progress
				leftImpl, ok := left.(*virtualSliceImpl)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedLeftProgress, leftImpl.progress)

				rightImpl, ok := right.(*virtualSliceImpl)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedRightProgress, rightImpl.progress)
			} else {
				assert.Nil(t, left)
				assert.Nil(t, right)
			}
		})
	}
}

func TestPendingTaskStats(t *testing.T) {
	tests := []struct {
		name          string
		mockSetup     func(*MockPendingTaskTracker)
		expectedStats PendingTaskStats
	}{
		{
			name: "Empty pending task tracker - should return empty stats",
			mockSetup: func(mock *MockPendingTaskTracker) {
				mock.EXPECT().GetPerDomainPendingTaskCount().Return(map[string]int{})
			},
			expectedStats: PendingTaskStats{
				PendingTaskCountPerDomain: map[string]int{},
			},
		},
		{
			name: "Single domain with tasks - should return correct stats",
			mockSetup: func(mock *MockPendingTaskTracker) {
				mock.EXPECT().GetPerDomainPendingTaskCount().Return(map[string]int{
					"domain1": 5,
				})
			},
			expectedStats: PendingTaskStats{
				PendingTaskCountPerDomain: map[string]int{
					"domain1": 5,
				},
			},
		},
		{
			name: "Multiple domains with tasks - should return correct stats",
			mockSetup: func(mock *MockPendingTaskTracker) {
				mock.EXPECT().GetPerDomainPendingTaskCount().Return(map[string]int{
					"domain1": 3,
					"domain2": 7,
					"domain3": 2,
				})
			},
			expectedStats: PendingTaskStats{
				PendingTaskCountPerDomain: map[string]int{
					"domain1": 3,
					"domain2": 7,
					"domain3": 2,
				},
			},
		},
		{
			name: "Domain with zero tasks - should include zero counts",
			mockSetup: func(mock *MockPendingTaskTracker) {
				mock.EXPECT().GetPerDomainPendingTaskCount().Return(map[string]int{
					"domain1": 5,
					"domain2": 0,
					"domain3": 3,
				})
			},
			expectedStats: PendingTaskStats{
				PendingTaskCountPerDomain: map[string]int{
					"domain1": 5,
					"domain2": 0,
					"domain3": 3,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)

			// Setup the virtual slice with mock dependencies
			slice := &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewUniversalPredicate(),
				},
				pendingTaskTracker: mockPendingTaskTracker,
			}

			// Setup mock expectations
			tt.mockSetup(mockPendingTaskTracker)

			// Call the method under test
			result := slice.PendingTaskStats()

			// Verify the result
			assert.Equal(t, tt.expectedStats, result)
		})
	}
}

func TestMergeVirtualSlicesWithDifferentPredicate(t *testing.T) {
	tests := []struct {
		name           string
		this           *virtualSliceImpl
		that           *virtualSliceImpl
		expectedStates []VirtualSliceState
		expectedOk     bool
	}{
		{
			name: "Case 1: Non-overlapping ranges",
			this: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(3),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			that: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					Predicate: NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(4),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(5),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			expectedOk: false,
		},
		{
			name: "Case 2: Overlapping ranges",
			this: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			that: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					Predicate: NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(3),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			expectedOk: true,
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(2),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2", "domain3", "domain4"}, false),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
					},
					Predicate: NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
				},
			},
		},
		{
			name: "Case 3: Completely overlapping ranges - left contains right and has the same exclusive max task key",
			this: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			that: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(3),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			expectedOk: true,
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(2),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2", "domain3", "domain4"}, false),
				},
			},
		},
		{
			name: "Case 4: Completely overlapping ranges - left contains right and has different exclusive max task key",
			this: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			that: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					Predicate: NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(3),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			expectedOk: true,
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(2),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2", "domain3", "domain4"}, false),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
			},
		},
		{
			name: "Case 5: Completely overlapping ranges - left contains right and has different exclusive max task key",
			this: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			that: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					Predicate: NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			expectedOk: true,
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(6),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2", "domain3", "domain4"}, false),
				},
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
			},
		},
		{
			name: "Case 6: Same ranges",
			this: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			that: &virtualSliceImpl{
				state: VirtualSliceState{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewDomainIDPredicate([]string{"domain3", "domain4"}, false),
				},
				progress: []*GetTaskProgress{
					{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						NextTaskKey: persistence.NewImmediateTaskKey(2),
					},
				},
				pendingTaskTracker: NewPendingTaskTracker(),
			},
			expectedOk: true,
			expectedStates: []VirtualSliceState{
				{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
						ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
					},
					Predicate: NewDomainIDPredicate([]string{"domain1", "domain2", "domain3", "domain4"}, false),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergedSlices, ok := mergeVirtualSlicesWithDifferentPredicate(tt.this, tt.that)
			assert.Equal(t, tt.expectedOk, ok)
			if ok {
				actualStates := make([]VirtualSliceState, len(mergedSlices))
				for i, slice := range mergedSlices {
					actualStates[i] = slice.GetState()
				}
				assert.Equal(t, tt.expectedStates, actualStates)
			}
			// Test commutativity - the function should handle argument order internally

			mergedSlices, ok = mergeVirtualSlicesWithDifferentPredicate(tt.that, tt.this)
			assert.Equal(t, tt.expectedOk, ok)
			if ok {
				actualStates := make([]VirtualSliceState, len(mergedSlices))
				for i, slice := range mergedSlices {
					actualStates[i] = slice.GetState()
				}
				assert.Equal(t, tt.expectedStates, actualStates)
			}
		})
	}
}

func TestVirtualSliceImpl_InsertTask(t *testing.T) {
	tests := []struct {
		name           string
		setupSlice     func(ctrl *gomock.Controller) *virtualSliceImpl
		setupTask      func(ctrl *gomock.Controller) task.Task
		expectedResult bool
		expectAddTask  bool
	}{
		{
			name: "Task within range and predicate matches - should insert successfully",
			setupSlice: func(ctrl *gomock.Controller) *virtualSliceImpl {
				mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)
				mockPredicate := NewMockPredicate(ctrl)
				mockPredicate.EXPECT().Check(gomock.Any()).Return(true)

				return &virtualSliceImpl{
					state: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						Predicate: mockPredicate,
					},
					pendingTaskTracker: mockPendingTaskTracker,
				}
			},
			setupTask: func(ctrl *gomock.Controller) task.Task {
				mockTask := task.NewMockTask(ctrl)
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewImmediateTaskKey(5)).AnyTimes()
				return mockTask
			},
			expectedResult: true,
			expectAddTask:  true,
		},
		{
			name: "Task outside range - should not insert",
			setupSlice: func(ctrl *gomock.Controller) *virtualSliceImpl {
				mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)
				mockPredicate := NewMockPredicate(ctrl)

				return &virtualSliceImpl{
					state: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						Predicate: mockPredicate,
					},
					pendingTaskTracker: mockPendingTaskTracker,
				}
			},
			setupTask: func(ctrl *gomock.Controller) task.Task {
				mockTask := task.NewMockTask(ctrl)
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewImmediateTaskKey(15)).AnyTimes()
				return mockTask
			},
			expectedResult: false,
			expectAddTask:  false,
		},
		{
			name: "Task within range but predicate doesn't match - should not insert",
			setupSlice: func(ctrl *gomock.Controller) *virtualSliceImpl {
				mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)
				mockPredicate := NewMockPredicate(ctrl)
				mockPredicate.EXPECT().Check(gomock.Any()).Return(false)

				return &virtualSliceImpl{
					state: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						Predicate: mockPredicate,
					},
					pendingTaskTracker: mockPendingTaskTracker,
				}
			},
			setupTask: func(ctrl *gomock.Controller) task.Task {
				mockTask := task.NewMockTask(ctrl)
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewImmediateTaskKey(5)).AnyTimes()
				return mockTask
			},
			expectedResult: false,
			expectAddTask:  false,
		},
		{
			name: "Task at inclusive min boundary - should insert successfully",
			setupSlice: func(ctrl *gomock.Controller) *virtualSliceImpl {
				mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)
				mockPredicate := NewMockPredicate(ctrl)
				mockPredicate.EXPECT().Check(gomock.Any()).Return(true)

				return &virtualSliceImpl{
					state: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						Predicate: mockPredicate,
					},
					pendingTaskTracker: mockPendingTaskTracker,
				}
			},
			setupTask: func(ctrl *gomock.Controller) task.Task {
				mockTask := task.NewMockTask(ctrl)
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewImmediateTaskKey(1)).AnyTimes()
				return mockTask
			},
			expectedResult: true,
			expectAddTask:  true,
		},
		{
			name: "Task at exclusive max boundary - should not insert",
			setupSlice: func(ctrl *gomock.Controller) *virtualSliceImpl {
				mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)
				mockPredicate := NewMockPredicate(ctrl)

				return &virtualSliceImpl{
					state: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
						Predicate: mockPredicate,
					},
					pendingTaskTracker: mockPendingTaskTracker,
				}
			},
			setupTask: func(ctrl *gomock.Controller) task.Task {
				mockTask := task.NewMockTask(ctrl)
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewImmediateTaskKey(10)).AnyTimes()
				return mockTask
			},
			expectedResult: false,
			expectAddTask:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			slice := tt.setupSlice(ctrl)
			task := tt.setupTask(ctrl)

			if tt.expectAddTask {
				slice.pendingTaskTracker.(*MockPendingTaskTracker).EXPECT().AddTask(task).Times(1)
			}

			result := slice.InsertTask(task)

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestVirtualSliceImpl_ResetProgress(t *testing.T) {
	tests := []struct {
		name                string
		setupSlice          func(ctrl *gomock.Controller) *virtualSliceImpl
		resetKey            persistence.HistoryTaskKey
		expectedProgressLen int
		expectedCancelCalls int
		validateProgress    func(t *testing.T, progress []*GetTaskProgress)
	}{
		{
			name: "Reset key after slice range - should not reset",
			setupSlice: func(ctrl *gomock.Controller) *virtualSliceImpl {
				mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)
				mockPendingTaskTracker.EXPECT().GetTasks().Return(map[persistence.HistoryTaskKey]task.Task{}).AnyTimes()

				return &virtualSliceImpl{
					state: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
					pendingTaskTracker: mockPendingTaskTracker,
					progress: []*GetTaskProgress{
						{
							Range: Range{
								InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
								ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
							},
							NextTaskKey: persistence.NewImmediateTaskKey(5),
						},
					},
				}
			},
			resetKey:            persistence.NewImmediateTaskKey(15),
			expectedProgressLen: 1,
			expectedCancelCalls: 0,
			validateProgress: func(t *testing.T, progress []*GetTaskProgress) {
				assert.Equal(t, persistence.NewImmediateTaskKey(5), progress[0].NextTaskKey)
			},
		},
		{
			name: "Reset key within range with no progress - should create new progress",
			setupSlice: func(ctrl *gomock.Controller) *virtualSliceImpl {
				mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)
				mockPendingTaskTracker.EXPECT().GetTasks().Return(map[persistence.HistoryTaskKey]task.Task{}).AnyTimes()

				return &virtualSliceImpl{
					state: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
					pendingTaskTracker: mockPendingTaskTracker,
					progress:           []*GetTaskProgress{},
				}
			},
			resetKey:            persistence.NewImmediateTaskKey(5),
			expectedProgressLen: 1,
			expectedCancelCalls: 0,
			validateProgress: func(t *testing.T, progress []*GetTaskProgress) {
				assert.Equal(t, persistence.NewImmediateTaskKey(5), progress[0].NextTaskKey)
				assert.Equal(t, persistence.NewImmediateTaskKey(5), progress[0].InclusiveMinTaskKey)
				assert.Equal(t, persistence.NewImmediateTaskKey(10), progress[0].ExclusiveMaxTaskKey)
			},
		},
		{
			name: "Reset key within range with existing progress - should reset and cancel tasks",
			setupSlice: func(ctrl *gomock.Controller) *virtualSliceImpl {
				mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)
				mockTask1 := task.NewMockTask(ctrl)
				mockTask2 := task.NewMockTask(ctrl)
				mockTask3 := task.NewMockTask(ctrl)

				mockTask1.EXPECT().GetTaskKey().Return(persistence.NewImmediateTaskKey(3)).AnyTimes()

				mockTask2.EXPECT().GetTaskKey().Return(persistence.NewImmediateTaskKey(5)).AnyTimes()
				mockTask2.EXPECT().Cancel().Times(1)

				mockTask3.EXPECT().GetTaskKey().Return(persistence.NewImmediateTaskKey(7)).AnyTimes()
				mockTask3.EXPECT().Cancel().Times(1)

				mockPendingTaskTracker.EXPECT().GetTasks().Return(map[persistence.HistoryTaskKey]task.Task{
					persistence.NewImmediateTaskKey(3): mockTask1,
					persistence.NewImmediateTaskKey(5): mockTask2,
					persistence.NewImmediateTaskKey(7): mockTask3,
				})

				return &virtualSliceImpl{
					state: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
					pendingTaskTracker: mockPendingTaskTracker,
					progress: []*GetTaskProgress{
						{
							Range: Range{
								InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
								ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
							},
							NextTaskKey: persistence.NewImmediateTaskKey(8),
						},
					},
				}
			},
			resetKey:            persistence.NewImmediateTaskKey(5),
			expectedProgressLen: 1,
			expectedCancelCalls: 2,
			validateProgress: func(t *testing.T, progress []*GetTaskProgress) {
				assert.Equal(t, persistence.NewImmediateTaskKey(5), progress[0].NextTaskKey)
				assert.Equal(t, persistence.NewImmediateTaskKey(1), progress[0].InclusiveMinTaskKey)
				assert.Equal(t, persistence.NewImmediateTaskKey(10), progress[0].ExclusiveMaxTaskKey)
			},
		},
		{
			name: "Reset key with multiple progress ranges - should merge remaining ranges",
			setupSlice: func(ctrl *gomock.Controller) *virtualSliceImpl {
				mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)
				mockPendingTaskTracker.EXPECT().GetTasks().Return(map[persistence.HistoryTaskKey]task.Task{}).AnyTimes()

				return &virtualSliceImpl{
					state: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
						},
					},
					pendingTaskTracker: mockPendingTaskTracker,
					progress: []*GetTaskProgress{
						{
							Range: Range{
								InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
								ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
							},
							NextTaskKey: persistence.NewImmediateTaskKey(5),
						},
						{
							Range: Range{
								InclusiveMinTaskKey: persistence.NewImmediateTaskKey(10),
								ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
							},
							NextTaskKey: persistence.NewImmediateTaskKey(15),
						},
					},
				}
			},
			resetKey:            persistence.NewImmediateTaskKey(7),
			expectedProgressLen: 1,
			expectedCancelCalls: 0,
			validateProgress: func(t *testing.T, progress []*GetTaskProgress) {
				assert.Equal(t, persistence.NewImmediateTaskKey(5), progress[0].NextTaskKey)
				assert.Equal(t, persistence.NewImmediateTaskKey(1), progress[0].InclusiveMinTaskKey)
				assert.Equal(t, persistence.NewImmediateTaskKey(20), progress[0].ExclusiveMaxTaskKey)
			},
		},
		{
			name: "Reset key before current progress next task key - should use reset key as next task key",
			setupSlice: func(ctrl *gomock.Controller) *virtualSliceImpl {
				mockPendingTaskTracker := NewMockPendingTaskTracker(ctrl)
				mockPendingTaskTracker.EXPECT().GetTasks().Return(map[persistence.HistoryTaskKey]task.Task{}).AnyTimes()

				return &virtualSliceImpl{
					state: VirtualSliceState{
						Range: Range{
							InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
							ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
						},
					},
					pendingTaskTracker: mockPendingTaskTracker,
					progress: []*GetTaskProgress{
						{
							Range: Range{
								InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
								ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
							},
							NextTaskKey: persistence.NewImmediateTaskKey(8),
						},
					},
				}
			},
			resetKey:            persistence.NewImmediateTaskKey(3),
			expectedProgressLen: 1,
			expectedCancelCalls: 0,
			validateProgress: func(t *testing.T, progress []*GetTaskProgress) {
				assert.Equal(t, persistence.NewImmediateTaskKey(3), progress[0].NextTaskKey)
				assert.Equal(t, persistence.NewImmediateTaskKey(1), progress[0].InclusiveMinTaskKey)
				assert.Equal(t, persistence.NewImmediateTaskKey(10), progress[0].ExclusiveMaxTaskKey)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			slice := tt.setupSlice(ctrl)

			slice.ResetProgress(tt.resetKey)

			assert.Equal(t, tt.expectedProgressLen, len(slice.progress))
			if tt.validateProgress != nil {
				tt.validateProgress(t, slice.progress)
			}
		})
	}
}
