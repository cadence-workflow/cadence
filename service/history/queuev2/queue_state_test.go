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

	split1, split2, ok = state.TrySplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 10), 100))
	assert.True(t, ok)
	assert.Equal(t, Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
	}, split1.Range)
	assert.Equal(t, Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
	}, split2.Range)

	split1, split2, ok = state.TrySplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 1), 0))
	assert.True(t, ok)
	assert.Equal(t, Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
	}, split1.Range)
	assert.Equal(t, Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
	}, split2.Range)

	_, _, ok = state.TrySplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 11), 100))
	assert.False(t, ok)

	_, _, ok = state.TrySplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 0), 101))
	assert.False(t, ok)
}
