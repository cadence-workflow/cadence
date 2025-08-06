package queuev2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
)

func TestQueueState_IsEmpty(t *testing.T) {
	state := &VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
			ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
		},
		Predicate: NewUniversalPredicate(),
	}
	assert.True(t, state.IsEmpty())

	ctrl := gomock.NewController(t)
	mockPredicate := NewMockPredicate(ctrl)
	mockPredicate.EXPECT().IsEmpty().Return(true)

	state = &VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
			ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
		},
		Predicate: mockPredicate,
	}
	assert.True(t, state.IsEmpty())
}

func TestVirtualSliceState_Contains(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockPredicate := NewMockPredicate(ctrl)
	mockPredicate.EXPECT().Check(gomock.Any()).Return(true).AnyTimes()

	state := &VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
			ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
		},
		Predicate: mockPredicate,
	}
	assert.True(t, state.Contains(&persistence.DecisionTimeoutTask{
		TaskData: persistence.TaskData{
			TaskID:              1,
			VisibilityTimestamp: time.Unix(0, 1),
		},
	}))
	assert.False(t, state.Contains(&persistence.DecisionTimeoutTask{
		TaskData: persistence.TaskData{
			TaskID:              100,
			VisibilityTimestamp: time.Unix(0, 10),
		},
	}))
	assert.False(t, state.Contains(&persistence.DecisionTimeoutTask{
		TaskData: persistence.TaskData{
			TaskID:              101,
			VisibilityTimestamp: time.Unix(0, 10),
		},
	}))
	assert.False(t, state.Contains(&persistence.DecisionTimeoutTask{
		TaskData: persistence.TaskData{
			TaskID:              101,
			VisibilityTimestamp: time.Unix(0, 0),
		},
	}))
}

func TestVirtualSliceState_TrySplitByTaskKey(t *testing.T) {
	state := &VirtualSliceState{
		Range: Range{
			InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
			ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
		},
		Predicate: NewUniversalPredicate(),
	}

	split1, split2, ok := state.TrySplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 5), 50))
	assert.True(t, ok)
	assert.Equal(t, Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 5), 50),
	}, split1.Range)
	assert.Equal(t, Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 5), 50),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
	}, split2.Range)

	_, _, ok = state.TrySplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 10), 100))
	assert.False(t, ok)

	_, _, ok = state.TrySplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 1), 0))
	assert.False(t, ok)

	_, _, ok = state.TrySplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 11), 100))
	assert.False(t, ok)

	_, _, ok = state.TrySplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 0), 101))
	assert.False(t, ok)
}

func TestVirtualSliceState_TrySplitByPredicate(t *testing.T) {
	baseRange := Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
	}
	basePredicate := NewDomainIDPredicate([]string{"domain1", "domain2"}, false)

	tests := []struct {
		name           string
		state          VirtualSliceState
		splitPredicate Predicate
		expectedSplit  bool
		expectedFirst  VirtualSliceState
		expectedSecond VirtualSliceState
	}{
		{
			name: "universal predicate should not split",
			state: VirtualSliceState{
				Range:     baseRange,
				Predicate: basePredicate,
			},
			splitPredicate: NewUniversalPredicate(),
			expectedSplit:  false,
			expectedFirst:  VirtualSliceState{},
			expectedSecond: VirtualSliceState{},
		},
		{
			name: "empty predicate should not split",
			state: VirtualSliceState{
				Range:     baseRange,
				Predicate: basePredicate,
			},
			splitPredicate: NewEmptyPredicate(),
			expectedSplit:  false,
			expectedFirst:  VirtualSliceState{},
			expectedSecond: VirtualSliceState{},
		},
		{
			name: "identical predicate should not split",
			state: VirtualSliceState{
				Range:     baseRange,
				Predicate: basePredicate,
			},
			splitPredicate: NewDomainIDPredicate([]string{"domain1", "domain2"}, false),
			expectedSplit:  false,
			expectedFirst:  VirtualSliceState{},
			expectedSecond: VirtualSliceState{},
		},
		{
			name: "different predicate should split successfully",
			state: VirtualSliceState{
				Range:     baseRange,
				Predicate: basePredicate,
			},
			splitPredicate: NewDomainIDPredicate([]string{"domain3"}, false),
			expectedSplit:  true,
			expectedFirst: VirtualSliceState{
				Range:     baseRange,
				Predicate: And(basePredicate, NewDomainIDPredicate([]string{"domain3"}, false)),
			},
			expectedSecond: VirtualSliceState{
				Range:     baseRange,
				Predicate: And(basePredicate, Not(NewDomainIDPredicate([]string{"domain3"}, false))),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, second, split := tt.state.TrySplitByPredicate(tt.splitPredicate)

			assert.Equal(t, tt.expectedSplit, split)

			if tt.expectedSplit {
				assert.Equal(t, tt.expectedFirst.Range, first.Range)
				assert.Equal(t, tt.expectedSecond.Range, second.Range)
				// For predicates, we check if they produce the same results rather than exact equality
				// since the And and Not operations create new predicate instances
				assert.True(t, tt.expectedFirst.Predicate.Equals(first.Predicate))
				assert.True(t, tt.expectedSecond.Predicate.Equals(second.Predicate))
			} else {
				assert.Equal(t, tt.expectedFirst, first)
				assert.Equal(t, tt.expectedSecond, second)
			}
		})
	}
}
