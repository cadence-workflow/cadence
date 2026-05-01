package queuev2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/persistence"
)

func TestRange_IsEmpty(t *testing.T) {
	r := Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(2),
	}
	assert.False(t, r.IsEmpty())

	r = Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(1),
	}
	assert.True(t, r.IsEmpty())

	r = Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
	}
	assert.True(t, r.IsEmpty())

	r = Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(1, 0), 1),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(2, 0), 1),
	}
	assert.False(t, r.IsEmpty())

}

func TestRange_Contains(t *testing.T) {
	r := Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(2),
	}
	assert.True(t, r.Contains(persistence.NewImmediateTaskKey(1)))
	assert.False(t, r.Contains(persistence.NewImmediateTaskKey(2)))
	assert.False(t, r.Contains(persistence.NewImmediateTaskKey(0)))
	assert.False(t, r.Contains(persistence.NewImmediateTaskKey(3)))

	r = Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 100), 0),
	}
	assert.False(t, r.Contains(persistence.NewHistoryTaskKey(time.Unix(0, 0), 0)))
	assert.True(t, r.Contains(persistence.NewHistoryTaskKey(time.Unix(0, 1), 0)))
	assert.True(t, r.Contains(persistence.NewHistoryTaskKey(time.Unix(0, 1), 1000000)))
	assert.True(t, r.Contains(persistence.NewHistoryTaskKey(time.Unix(0, 99), 1000000)))
	assert.False(t, r.Contains(persistence.NewHistoryTaskKey(time.Unix(0, 100), 0)))
	assert.False(t, r.Contains(persistence.NewHistoryTaskKey(time.Unix(0, 101), 0)))
}

func TestRange_ContainsRange(t *testing.T) {
	r := Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
	}
	assert.True(t, r.ContainsRange(r))
	assert.True(t, r.ContainsRange(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(5),
	}))
	assert.True(t, r.ContainsRange(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(6),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
	}))
	assert.False(t, r.ContainsRange(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(15),
	}))
	assert.False(t, r.ContainsRange(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(0),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(1),
	}))

	r = Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
	}
	assert.True(t, r.ContainsRange(r))
	assert.True(t, r.ContainsRange(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 5), 0),
	}))
	assert.True(t, r.ContainsRange(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 5), 101),
	}))
	assert.True(t, r.ContainsRange(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 5), 1110),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
	}))
	assert.False(t, r.ContainsRange(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 15), 100),
	}))
	assert.False(t, r.ContainsRange(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 100),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 101),
	}))
}

func TestRange_CanMerge(t *testing.T) {
	r := Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
	}
	assert.True(t, r.CanMerge(r))
	assert.True(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(2),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(8),
	}))
	assert.True(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(0),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(7),
	}))
	assert.True(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(5),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(12),
	}))
	assert.True(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(10),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(20),
	}))
	assert.True(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(0),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(1),
	}))
	assert.False(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(0),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(0),
	}))
	assert.False(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(11),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(15),
	}))

	r = Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
	}
	assert.True(t, r.CanMerge(r))
	assert.True(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 1),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 99),
	}))
	assert.True(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 100), 100),
	}))
	assert.True(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 100),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
	}))
	assert.True(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 100), 100),
	}))
	assert.False(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 101),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 100), 100),
	}))
	assert.False(t, r.CanMerge(Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), -1),
	}))
}

func TestRange_CanSplitByTaskKey(t *testing.T) {
	r := Range{
		InclusiveMinTaskKey: persistence.NewImmediateTaskKey(1),
		ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(10),
	}
	assert.True(t, r.CanSplitByTaskKey(persistence.NewImmediateTaskKey(5)))
	assert.False(t, r.CanSplitByTaskKey(persistence.NewImmediateTaskKey(1)))
	assert.False(t, r.CanSplitByTaskKey(persistence.NewImmediateTaskKey(10)))
	assert.False(t, r.CanSplitByTaskKey(persistence.NewImmediateTaskKey(0)))
	assert.False(t, r.CanSplitByTaskKey(persistence.NewImmediateTaskKey(11)))

	r = Range{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 1), 0),
		ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 10), 100),
	}
	assert.True(t, r.CanSplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 5), 100)))
	assert.False(t, r.CanSplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 1), 0)))
	assert.False(t, r.CanSplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 10), 100)))
	assert.False(t, r.CanSplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 0), 0)))
	assert.False(t, r.CanSplitByTaskKey(persistence.NewHistoryTaskKey(time.Unix(0, 10), 101)))
}
