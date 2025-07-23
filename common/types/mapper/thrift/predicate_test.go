// Copyright (c) 2017-2022 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package thrift

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common"
)

func predicateTypeFuzzGenerator(t *types.PredicateType, c fuzz.Continue) {
	switch c.Intn(int(types.NumPredicateTypes)) {
	case 0:
		*t = types.PredicateTypeUniversal
	case 1:
		*t = types.PredicateTypeEmpty
	case 2:
		*t = types.PredicateTypeAnd
	case 3:
		*t = types.PredicateTypeOr
	case 4:
		*t = types.PredicateTypeNot
	case 5:
		*t = types.PredicateTypeDomainID
	default:
		panic("invalid predicate type")
	}
}

func predicateTypeSharedFuzzGenerator(t **shared.PredicateType, c fuzz.Continue) {
	switch c.Intn(int(types.NumPredicateTypes)) {
	case 0:
		*t = common.Ptr(shared.PredicateTypeUniversal)
	case 1:
		*t = common.Ptr(shared.PredicateTypeEmpty)
	case 2:
		*t = common.Ptr(shared.PredicateTypeAnd)
	case 3:
		*t = common.Ptr(shared.PredicateTypeOr)
	case 4:
		*t = common.Ptr(shared.PredicateTypeNot)
	case 5:
		*t = common.Ptr(shared.PredicateTypeDomainID)
	default:
		panic("invalid predicate type")
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
		t.PredicateType = types.PredicateTypeAnd
		c.Fuzz(&t.AndPredicateAttributes)
	case 3:
		t.PredicateType = types.PredicateTypeOr
		c.Fuzz(&t.OrPredicateAttributes)
	case 4:
		t.PredicateType = types.PredicateTypeNot
		c.Fuzz(&t.NotPredicateAttributes)
	case 5:
		t.PredicateType = types.PredicateTypeDomainID
		c.Fuzz(&t.DomainIDPredicateAttributes)
	default:
		panic("invalid predicate type")
	}
}

func predicateSharedFuzzGenerator(t *shared.Predicate, c fuzz.Continue) {
	switch c.Intn(int(types.NumPredicateTypes)) {
	case 0:
		t.PredicateType = common.Ptr(shared.PredicateTypeUniversal)
		c.Fuzz(&t.UniversalPredicateAttributes)
	case 1:
		t.PredicateType = common.Ptr(shared.PredicateTypeEmpty)
		c.Fuzz(&t.EmptyPredicateAttributes)
	case 2:
		t.PredicateType = common.Ptr(shared.PredicateTypeAnd)
		c.Fuzz(&t.AndPredicateAttributes)
	case 3:
		t.PredicateType = common.Ptr(shared.PredicateTypeOr)
		c.Fuzz(&t.OrPredicateAttributes)
	case 4:
		t.PredicateType = common.Ptr(shared.PredicateTypeNot)
		c.Fuzz(&t.NotPredicateAttributes)
	case 5:
		t.PredicateType = common.Ptr(shared.PredicateTypeDomainID)
		c.Fuzz(&t.DomainIDPredicateAttributes)
	default:
		panic("invalid predicate type")
	}
}

func TestFuzzPredicateType_Roundtrip_FromTypes(t *testing.T) {
	f := fuzz.New().Funcs(
		predicateTypeFuzzGenerator,
	)

	for i := 0; i < 1000; i++ {
		// Generate only valid enum values
		var original types.PredicateType
		f.Fuzz(&original)

		// types → shared → types
		shared := FromPredicateType(original)
		converted := ToPredicateType(shared)

		assert.Equal(t, original, converted, "PredicateType roundtrip failed: types → shared → types")
	}
}

func TestFuzzPredicateType_Roundtrip_FromShared(t *testing.T) {
	f := fuzz.New().Funcs(
		predicateTypeSharedFuzzGenerator,
	)
	for i := 0; i < 1000; i++ {
		// Generate only valid enum values
		var original *shared.PredicateType
		f.Fuzz(&original)

		// shared → types → shared
		types := ToPredicateType(original)
		converted := FromPredicateType(types)

		assert.Equal(t, original, converted, "PredicateType roundtrip failed: shared → types → shared")
	}
}

func TestFuzzUniversalPredicateAttributes_Roundtrip_FromTypes(t *testing.T) {
	f := fuzz.New()
	for i := 0; i < 1000; i++ {
		var original types.UniversalPredicateAttributes
		f.Fuzz(&original)

		// types → shared → types
		shared := FromUniversalPredicateAttributes(&original)
		converted := ToUniversalPredicateAttributes(shared)

		assert.Equal(t, original, *converted, "UniversalPredicateAttributes roundtrip failed: types → shared → types")
	}
}

func TestFuzzUniversalPredicateAttributes_Roundtrip_FromShared(t *testing.T) {
	f := fuzz.New()
	for i := 0; i < 1000; i++ {
		var original shared.UniversalPredicateAttributes
		f.Fuzz(&original)

		// shared → types → shared
		types := ToUniversalPredicateAttributes(&original)
		converted := FromUniversalPredicateAttributes(types)

		assert.Equal(t, original, *converted, "UniversalPredicateAttributes roundtrip failed: shared → types → shared")
	}
}

func TestFuzzEmptyPredicateAttributes_Roundtrip_FromTypes(t *testing.T) {
	f := fuzz.New()
	for i := 0; i < 1000; i++ {
		var original types.EmptyPredicateAttributes
		f.Fuzz(&original)

		// types → shared → types
		shared := FromEmptyPredicateAttributes(&original)
		converted := ToEmptyPredicateAttributes(shared)

		assert.Equal(t, original, *converted, "EmptyPredicateAttributes roundtrip failed: types → shared → types")
	}
}

func TestFuzzEmptyPredicateAttributes_Roundtrip_FromShared(t *testing.T) {
	f := fuzz.New()
	for i := 0; i < 1000; i++ {
		var original shared.EmptyPredicateAttributes
		f.Fuzz(&original)

		// shared → types → shared
		types := ToEmptyPredicateAttributes(&original)
		converted := FromEmptyPredicateAttributes(types)

		assert.Equal(t, original, *converted, "EmptyPredicateAttributes roundtrip failed: shared → types → shared")
	}
}

func TestFuzzDomainIDPredicateAttributes_Roundtrip_FromTypes(t *testing.T) {
	f := fuzz.New().NilChance(0.1)
	for i := 0; i < 1000; i++ {
		var original types.DomainIDPredicateAttributes
		f.Fuzz(&original)

		// types → shared → types
		shared := FromDomainIDPredicateAttributes(&original)
		converted := ToDomainIDPredicateAttributes(shared)

		assert.Equal(t, original, *converted, "DomainIDPredicateAttributes roundtrip failed: types → shared → types")
	}
}

func TestFuzzDomainIDPredicateAttributes_Roundtrip_FromShared(t *testing.T) {
	f := fuzz.New().NilChance(0.1)
	for i := 0; i < 1000; i++ {
		var original shared.DomainIDPredicateAttributes
		f.Fuzz(&original)

		// shared → types → shared
		types := ToDomainIDPredicateAttributes(&original)
		converted := FromDomainIDPredicateAttributes(types)

		// Note: IsExclusive field handling - shared uses *bool, types uses bool
		// We need to handle the conversion properly
		expected := original
		if original.IsExclusive == nil {
			// When shared.IsExclusive is nil, types will default to false
			// and FromDomainIDPredicateAttributes will convert that back to &false
			falseVal := false
			expected.IsExclusive = &falseVal
		}

		assert.Equal(t, expected, *converted, "DomainIDPredicateAttributes roundtrip failed: shared → types → shared")
	}
}

func TestFuzzAndPredicateAttributes_Roundtrip_FromTypes(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(
		predicateFuzzGenerator,
	).MaxDepth(15) // Limit depth to avoid infinite recursion
	for i := 0; i < 1000; i++ {
		var original types.AndPredicateAttributes
		f.Fuzz(&original)

		// types → shared → types
		shared := FromAndPredicateAttributes(&original)
		converted := ToAndPredicateAttributes(shared)

		assert.Equal(t, original, *converted, "AndPredicateAttributes roundtrip failed: types → shared → types")
	}
}

func TestFuzzAndPredicateAttributes_Roundtrip_FromShared(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(
		predicateSharedFuzzGenerator,
	).MaxDepth(15) // Limit depth to avoid infinite recursion
	for i := 0; i < 1000; i++ {
		var original shared.AndPredicateAttributes
		f.Fuzz(&original)

		// shared → types → shared
		types := ToAndPredicateAttributes(&original)
		converted := FromAndPredicateAttributes(types)

		assert.Equal(t, original, *converted, "AndPredicateAttributes roundtrip failed: shared → types → shared")
	}
}

func TestFuzzOrPredicateAttributes_Roundtrip_FromTypes(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(
		predicateFuzzGenerator,
	).MaxDepth(15) // Limit depth to avoid infinite recursion
	for i := 0; i < 1000; i++ {
		var original types.OrPredicateAttributes
		f.Fuzz(&original)

		// types → shared → types
		shared := FromOrPredicateAttributes(&original)
		converted := ToOrPredicateAttributes(shared)

		assert.Equal(t, original, *converted, "OrPredicateAttributes roundtrip failed: types → shared → types")
	}
}

func TestFuzzOrPredicateAttributes_Roundtrip_FromShared(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(
		predicateSharedFuzzGenerator,
	).MaxDepth(15) // Limit depth to avoid infinite recursion
	for i := 0; i < 1000; i++ {
		var original shared.OrPredicateAttributes
		f.Fuzz(&original)

		// shared → types → shared
		types := ToOrPredicateAttributes(&original)
		converted := FromOrPredicateAttributes(types)

		assert.Equal(t, original, *converted, "OrPredicateAttributes roundtrip failed: shared → types → shared")
	}
}

func TestFuzzNotPredicateAttributes_Roundtrip_FromTypes(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(
		predicateFuzzGenerator,
	).MaxDepth(15) // Limit depth to avoid infinite recursion
	for i := 0; i < 1000; i++ {
		var original types.NotPredicateAttributes
		f.Fuzz(&original)

		// types → shared → types
		shared := FromNotPredicateAttributes(&original)
		converted := ToNotPredicateAttributes(shared)

		assert.Equal(t, original, *converted, "NotPredicateAttributes roundtrip failed: types → shared → types")
	}
}

func TestFuzzNotPredicateAttributes_Roundtrip_FromShared(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(
		predicateSharedFuzzGenerator,
	).MaxDepth(15) // Limit depth to avoid infinite recursion
	for i := 0; i < 1000; i++ {
		var original shared.NotPredicateAttributes
		f.Fuzz(&original)

		// shared → types → shared
		types := ToNotPredicateAttributes(&original)
		converted := FromNotPredicateAttributes(types)

		assert.Equal(t, original, *converted, "NotPredicateAttributes roundtrip failed: shared → types → shared")
	}
}

func TestFuzzPredicate_Roundtrip_FromTypes(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(
		predicateFuzzGenerator,
	).MaxDepth(15) // Limit depth to avoid infinite recursion
	for i := 0; i < 1000; i++ {
		var original types.Predicate
		f.Fuzz(&original)

		// types → shared → types
		shared := FromPredicate(&original)
		converted := ToPredicate(shared)

		assert.Equal(t, original, *converted, "Predicate roundtrip failed: types → shared → types")
	}
}

func TestFuzzPredicate_Roundtrip_FromShared(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(
		predicateSharedFuzzGenerator,
	).MaxDepth(15) // Limit depth to avoid infinite recursion
	for i := 0; i < 1000; i++ {
		var original shared.Predicate
		f.Fuzz(&original)

		// shared → types → shared
		types := ToPredicate(&original)
		converted := FromPredicate(types)

		assert.Equal(t, original, *converted, "Predicate roundtrip failed: shared → types → shared")
	}
}

func TestFuzzPredicateList_Roundtrip_FromTypes(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(
		predicateFuzzGenerator,
	).MaxDepth(15) // Limit depth to avoid infinite recursion
	for i := 0; i < 1000; i++ {
		var original []*types.Predicate
		f.Fuzz(&original)

		// types → shared → types
		shared := FromPredicateList(original)
		converted := ToPredicateList(shared)

		assert.Equal(t, original, converted, "PredicateList roundtrip failed: types → shared → types")
	}
}

func TestFuzzPredicateList_Roundtrip_FromShared(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(
		predicateSharedFuzzGenerator,
	).MaxDepth(15) // Limit depth to avoid infinite recursion
	for i := 0; i < 1000; i++ {
		var original []*shared.Predicate
		f.Fuzz(&original)

		// shared → types → shared
		types := ToPredicateList(original)
		converted := FromPredicateList(types)

		assert.Equal(t, original, converted, "PredicateList roundtrip failed: shared → types → shared")
	}
}

func TestFuzzPredicate_NilHandling(t *testing.T) {
	// Test nil handling
	assert.Nil(t, FromPredicate(nil))
	assert.Nil(t, ToPredicate(nil))
	assert.Nil(t, FromPredicateList(nil))
	assert.Nil(t, ToPredicateList(nil))
	assert.Nil(t, FromAndPredicateAttributes(nil))
	assert.Nil(t, ToAndPredicateAttributes(nil))
	assert.Nil(t, FromOrPredicateAttributes(nil))
	assert.Nil(t, ToOrPredicateAttributes(nil))
	assert.Nil(t, FromNotPredicateAttributes(nil))
	assert.Nil(t, ToNotPredicateAttributes(nil))
	assert.Nil(t, FromDomainIDPredicateAttributes(nil))
	assert.Nil(t, ToDomainIDPredicateAttributes(nil))
}

func TestPredicateType_InvalidValues(t *testing.T) {
	assert.Equal(t, shared.PredicateTypeUniversal, *FromPredicateType(types.PredicateType(100)))
	assert.Equal(t, types.PredicateTypeUniversal, ToPredicateType(nil))
	assert.Equal(t, types.PredicateTypeUniversal, ToPredicateType(common.Ptr(shared.PredicateType(100))))
}
