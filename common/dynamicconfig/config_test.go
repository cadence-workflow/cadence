// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package dynamicconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDynamicConfigKeyIsMapped(t *testing.T) {
	for i := UnknownIntKey + 1; i < LastIntKey; i++ {
		key, ok := IntKeys[i]
		require.True(t, ok, "missing IntKey: %d", i)
		require.NotEmpty(t, key, "empty IntKey: %d", i)
	}
	for i := UnknownBoolKey + 1; i < LastBoolKey; i++ {
		key, ok := BoolKeys[i]
		require.True(t, ok, "missing BoolKey: %d", i)
		require.NotEmpty(t, key, "empty BoolKey: %d", i)
	}
	for i := UnknownFloatKey + 1; i < LastFloatKey; i++ {
		key, ok := FloatKeys[i]
		require.True(t, ok, "missing FloatKey: %d", i)
		require.NotEmpty(t, key, "empty FloatKey: %d", i)
	}
	for i := UnknownStringKey + 1; i < LastStringKey; i++ {
		key, ok := StringKeys[i]
		require.True(t, ok, "missing StringKey: %d", i)
		require.NotEmpty(t, key, "empty StringKey: %d", i)
	}
	for i := UnknownDurationKey + 1; i < LastDurationKey; i++ {
		key, ok := DurationKeys[i]
		require.True(t, ok, "missing DurationKey: %d", i)
		require.NotEmpty(t, key, "empty DurationKey: %d", i)
	}
	for i := UnknownMapKey + 1; i < LastMapKey; i++ {
		key, ok := MapKeys[i]
		require.True(t, ok, "missing MapKey: %d", i)
		require.NotEmpty(t, key, "empty MapKey: %d", i)
	}
}

func TestDynamicConfigFilterTypeIsMapped(t *testing.T) {
	require.Equal(t, int(LastFilterTypeForTest), len(filters))
	for i := UnknownFilter; i < LastFilterTypeForTest; i++ {
		require.NotEmpty(t, filters[i])
	}
}

func TestDynamicConfigFilterTypeIsParseable(t *testing.T) {
	allFilters := map[Filter]int{}
	for idx, filterString := range filters { // TestDynamicConfigFilterTypeIsMapped ensures this is a complete list
		// all filter-strings must parse to unique filters
		parsed := ParseFilter(filterString)
		prev, ok := allFilters[parsed]
		assert.False(t, ok, "%q is already mapped to the same filter type as %q", filterString, filters[prev])
		allFilters[parsed] = idx

		// otherwise, only "unknown" should map to "unknown".
		// ParseFilter should probably be re-implemented to simply use a map that is shared with the definitions
		// so values cannot get out of sync, but for now this is just asserting what is currently built.
		if idx == 0 {
			assert.Equalf(t, UnknownFilter, ParseFilter(filterString), "first filter string should have parsed as unknown: %v", filterString)
			// unknown filter string is likely safe to change and then should be updated here, but otherwise this ensures the logic isn't entirely position-dependent.
			require.Equalf(t, "unknownFilter", filterString, "expected first filter to be 'unknownFilter', but it was %v", filterString)
		} else {
			assert.NotEqualf(t, UnknownFilter, ParseFilter(filterString), "failed to parse filter: %s, make sure it is in ParseFilter's switch statement", filterString)
		}
	}
}

func TestDynamicConfigFilterStringsCorrectly(t *testing.T) {
	for _, filterString := range filters {
		// filter-string-parsing and the resulting filter's String() must match
		parsed := ParseFilter(filterString)
		assert.Equal(t, filterString, parsed.String(), "filters need to String() correctly as some impls rely on it")
	}
	// should not be possible normally, but improper casting could trigger it
	badFilter := Filter(len(filters))
	assert.Equal(t, UnknownFilter.String(), badFilter.String(), "filters with indexes outside the list of known strings should String() to the unknown filter type")
}
