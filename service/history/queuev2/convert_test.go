package queuev2

import (
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

// TODO: remove this once we implement converter for predicates
// We're creating small PRs for introducing predicates, in the current PR, we only create mappers between common/types and thrift types
func predicateFuzzGenerator(t **types.Predicate, c fuzz.Continue) {
	*t = nil
}

func TestConvertVirtualSliceState(t *testing.T) {
	f := fuzz.New().NilChance(0).Funcs(predicateFuzzGenerator)
	for i := 0; i < 1000; i++ {
		var s types.VirtualSliceState
		f.Fuzz(&s)
		persistenceState := FromPersistenceVirtualSliceState(&s)
		convertedState := ToPersistenceVirtualSliceState(persistenceState)
		assert.Equal(t, s, *convertedState)
	}
}

func TestConvertVirtualQueueState(t *testing.T) {
	f := fuzz.New().NilChance(0).Funcs(predicateFuzzGenerator)
	for i := 0; i < 1000; i++ {
		var s types.VirtualQueueState
		f.Fuzz(&s)
		persistenceState := FromPersistenceVirtualQueueState(&s)
		convertedState := ToPersistenceVirtualQueueState(persistenceState)
		assert.Equal(t, s, *convertedState)
	}
}

func TestConvertQueueState(t *testing.T) {
	f := fuzz.New().NilChance(0).Funcs(predicateFuzzGenerator)
	for i := 0; i < 1000; i++ {
		var s types.QueueState
		f.Fuzz(&s)
		persistenceState := FromPersistenceQueueState(&s)
		convertedState := ToPersistenceQueueState(persistenceState)
		assert.Equal(t, s, *convertedState)
	}
}
