package queuev2

import (
	"math/rand"
	"sort"
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
)

func TestConvertTaskKey(t *testing.T) {
	f := fuzz.New()
	for i := 0; i < 1000; i++ {
		var key types.TaskKey
		f.Fuzz(&key)
		persistenceKey := FromPersistenceTaskKey(&key)
		convertedKey := ToPersistenceTaskKey(persistenceKey)
		assert.Equal(t, key, *convertedKey)
	}
}

func TestConvertTaskRange(t *testing.T) {
	f := fuzz.New().NilChance(0)
	for i := 0; i < 1000; i++ {
		var r types.TaskRange
		f.Fuzz(&r)
		persistenceRange := FromPersistenceTaskRange(&r)
		convertedRange := ToPersistenceTaskRange(persistenceRange)
		assert.Equal(t, r, *convertedRange)
	}
}

func predicateFuzzGenerator(t *types.Predicate, c fuzz.Continue) {
	switch c.Intn(int(types.NumPredicateTypes)) {
	case 0:
		t.PredicateType = types.PredicateTypeUniversal
		c.Fuzz(&t.UniversalPredicateAttributes)
	case 1:
		t.PredicateType = types.PredicateTypeEmpty
		c.Fuzz(&t.EmptyPredicateAttributes)
	case 2:
		t.PredicateType = types.PredicateTypeDomainID
		c.Fuzz(&t.DomainIDPredicateAttributes)
	default:
		panic("invalid predicate type")
	}
}

func domainIDPredicateAttributesFuzzGenerator(t *types.DomainIDPredicateAttributes, c fuzz.Continue) {
	const maxCount = 10 // adjust as needed
	count := rand.Intn(maxCount) + 1

	seen := make(map[string]struct{})
	t.DomainIDs = make([]string, 0, count)

	for len(t.DomainIDs) < count {
		var s string
		c.Fuzz(&s)
		if s != "" && len(s) >= 3 {
			if _, exists := seen[s]; !exists {
				seen[s] = struct{}{}
				t.DomainIDs = append(t.DomainIDs, s)
			}
		}
	}

	sort.Strings(t.DomainIDs)

	var b bool
	c.Fuzz(&b)
	t.IsExclusive = &b
}

func TestConvertVirtualSliceState(t *testing.T) {
	f := fuzz.New().NilChance(0).Funcs(predicateFuzzGenerator, domainIDPredicateAttributesFuzzGenerator)
	for i := 0; i < 1000; i++ {
		var s types.VirtualSliceState
		f.Fuzz(&s)
		persistenceState := FromPersistenceVirtualSliceState(&s)
		convertedState := ToPersistenceVirtualSliceState(persistenceState)
		assert.Equal(t, s, *convertedState)
	}
}

func TestConvertVirtualQueueState(t *testing.T) {
	f := fuzz.New().NilChance(0).Funcs(predicateFuzzGenerator, domainIDPredicateAttributesFuzzGenerator)
	for i := 0; i < 1000; i++ {
		var s types.VirtualQueueState
		f.Fuzz(&s)
		persistenceState := FromPersistenceVirtualQueueState(&s)
		convertedState := ToPersistenceVirtualQueueState(persistenceState)
		assert.Equal(t, s, *convertedState)
	}
}

func TestConvertQueueState(t *testing.T) {
	f := fuzz.New().NilChance(0).Funcs(predicateFuzzGenerator, domainIDPredicateAttributesFuzzGenerator)
	for i := 0; i < 1000; i++ {
		var s types.QueueState
		f.Fuzz(&s)
		persistenceState := FromPersistenceQueueState(&s)
		convertedState := ToPersistenceQueueState(persistenceState)
		assert.Equal(t, s, *convertedState)
	}
}
