// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package queuev2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
)

func TestNewUniversalPredicate(t *testing.T) {
	predicate := NewUniversalPredicate()
	assert.NotNil(t, predicate)
	assert.IsType(t, &universalPredicate{}, predicate)
}

func TestUniversalPredicate_IsEmpty(t *testing.T) {
	predicate := NewUniversalPredicate()
	assert.False(t, predicate.IsEmpty())
}

func TestUniversalPredicate_Check(t *testing.T) {
	predicate := NewUniversalPredicate()

	// Test with nil task
	assert.True(t, predicate.Check(nil))

	// Test with mock task
	ctrl := gomock.NewController(t)

	task := persistence.NewMockTask(ctrl)
	task.EXPECT().GetDomainID().Return("test-domain").AnyTimes()
	assert.True(t, predicate.Check(task))

	// Test with different domain ID
	task2 := persistence.NewMockTask(ctrl)
	task2.EXPECT().GetDomainID().Return("different-domain").AnyTimes()
	assert.True(t, predicate.Check(task2))
}

func TestUniversalPredicate_Equals(t *testing.T) {
	predicate1 := NewUniversalPredicate()
	predicate2 := NewUniversalPredicate()
	emptyPredicate := NewEmptyPredicate()
	domainPredicate := NewDomainIDPredicate([]string{"test"}, false)

	// Same type should be equal
	assert.True(t, predicate1.Equals(predicate2))
	assert.True(t, predicate2.Equals(predicate1))

	// Different types should not be equal
	assert.False(t, predicate1.Equals(emptyPredicate))
	assert.False(t, predicate1.Equals(domainPredicate))
	assert.False(t, predicate1.Equals(nil))
}

func TestNewEmptyPredicate(t *testing.T) {
	predicate := NewEmptyPredicate()
	assert.NotNil(t, predicate)
	assert.IsType(t, &emptyPredicate{}, predicate)
}

func TestEmptyPredicate_IsEmpty(t *testing.T) {
	predicate := NewEmptyPredicate()
	assert.True(t, predicate.IsEmpty())
}

func TestEmptyPredicate_Check(t *testing.T) {
	predicate := NewEmptyPredicate()

	// Test with nil task
	assert.False(t, predicate.Check(nil))

	// Test with mock task
	ctrl := gomock.NewController(t)

	task := persistence.NewMockTask(ctrl)
	task.EXPECT().GetDomainID().Return("test-domain").AnyTimes()
	assert.False(t, predicate.Check(task))

	// Test with different domain ID
	task2 := persistence.NewMockTask(ctrl)
	task2.EXPECT().GetDomainID().Return("different-domain").AnyTimes()
	assert.False(t, predicate.Check(task2))
}

func TestEmptyPredicate_Equals(t *testing.T) {
	predicate1 := NewEmptyPredicate()
	predicate2 := NewEmptyPredicate()
	universalPredicate := NewUniversalPredicate()
	domainPredicate := NewDomainIDPredicate([]string{"test"}, false)

	// Same type should be equal
	assert.True(t, predicate1.Equals(predicate2))
	assert.True(t, predicate2.Equals(predicate1))

	// Different types should not be equal
	assert.False(t, predicate1.Equals(universalPredicate))
	assert.False(t, predicate1.Equals(domainPredicate))
	assert.False(t, predicate1.Equals(nil))
}

func TestNewDomainIDPredicate(t *testing.T) {
	// Test with empty domain IDs
	predicate := NewDomainIDPredicate([]string{}, false)
	assert.NotNil(t, predicate)
	assert.IsType(t, &domainIDPredicate{}, predicate)

	// Test with single domain ID
	predicate = NewDomainIDPredicate([]string{"domain1"}, false)
	assert.NotNil(t, predicate)

	// Test with multiple domain IDs
	predicate = NewDomainIDPredicate([]string{"domain1", "domain2", "domain3"}, true)
	assert.NotNil(t, predicate)

	// Test with duplicate domain IDs (should be deduplicated)
	predicate = NewDomainIDPredicate([]string{"domain1", "domain1", "domain2"}, false)
	assert.NotNil(t, predicate)
}

func TestDomainIDPredicate_IsEmpty(t *testing.T) {
	// Empty domain IDs with inclusive mode
	predicate := NewDomainIDPredicate([]string{}, false)
	assert.True(t, predicate.IsEmpty())

	// Empty domain IDs with exclusive mode
	predicate = NewDomainIDPredicate([]string{}, true)
	assert.False(t, predicate.IsEmpty())

	// Non-empty domain IDs with inclusive mode
	predicate = NewDomainIDPredicate([]string{"domain1"}, false)
	assert.False(t, predicate.IsEmpty())

	// Non-empty domain IDs with exclusive mode
	predicate = NewDomainIDPredicate([]string{"domain1"}, true)
	assert.False(t, predicate.IsEmpty())
}

func TestDomainIDPredicate_Check_Inclusive(t *testing.T) {
	// Test inclusive mode (isExclusive = false)
	predicate := NewDomainIDPredicate([]string{"domain1", "domain2"}, false)

	ctrl := gomock.NewController(t)

	// Task with domain in the list should pass
	task1 := persistence.NewMockTask(ctrl)
	task1.EXPECT().GetDomainID().Return("domain1").AnyTimes()
	assert.True(t, predicate.Check(task1))

	task2 := persistence.NewMockTask(ctrl)
	task2.EXPECT().GetDomainID().Return("domain2").AnyTimes()
	assert.True(t, predicate.Check(task2))

	// Task with domain not in the list should fail
	task3 := persistence.NewMockTask(ctrl)
	task3.EXPECT().GetDomainID().Return("domain3").AnyTimes()
	assert.False(t, predicate.Check(task3))

	// Test with empty domain ID
	task4 := persistence.NewMockTask(ctrl)
	task4.EXPECT().GetDomainID().Return("").AnyTimes()
	assert.False(t, predicate.Check(task4))
}

func TestDomainIDPredicate_Check_Exclusive(t *testing.T) {
	// Test exclusive mode (isExclusive = true)
	predicate := NewDomainIDPredicate([]string{"domain1", "domain2"}, true)

	ctrl := gomock.NewController(t)

	// Task with domain in the list should fail
	task1 := persistence.NewMockTask(ctrl)
	task1.EXPECT().GetDomainID().Return("domain1").AnyTimes()
	assert.False(t, predicate.Check(task1))

	task2 := persistence.NewMockTask(ctrl)
	task2.EXPECT().GetDomainID().Return("domain2").AnyTimes()
	assert.False(t, predicate.Check(task2))

	// Task with domain not in the list should pass
	task3 := persistence.NewMockTask(ctrl)
	task3.EXPECT().GetDomainID().Return("domain3").AnyTimes()
	assert.True(t, predicate.Check(task3))

	// Test with empty domain ID
	task4 := persistence.NewMockTask(ctrl)
	task4.EXPECT().GetDomainID().Return("").AnyTimes()
	assert.True(t, predicate.Check(task4))
}

func TestDomainIDPredicate_Check_EmptyList(t *testing.T) {
	// Test with empty domain list and inclusive mode
	predicate := NewDomainIDPredicate([]string{}, false)

	ctrl := gomock.NewController(t)

	task := persistence.NewMockTask(ctrl)
	task.EXPECT().GetDomainID().Return("any-domain").AnyTimes()
	assert.False(t, predicate.Check(task)) // No domains in list, so inclusive returns false

	// Test with empty domain list and exclusive mode
	predicate = NewDomainIDPredicate([]string{}, true)
	assert.True(t, predicate.Check(task)) // No domains in list, so exclusive returns true
}

func TestDomainIDPredicate_Equals(t *testing.T) {
	// Test equal predicates
	predicate1 := NewDomainIDPredicate([]string{"domain1", "domain2"}, false)
	predicate2 := NewDomainIDPredicate([]string{"domain1", "domain2"}, false)
	assert.True(t, predicate1.Equals(predicate2))

	// Test equal predicates with different order
	predicate3 := NewDomainIDPredicate([]string{"domain2", "domain1"}, false)
	assert.True(t, predicate1.Equals(predicate3))

	// Test different domain sets
	predicate4 := NewDomainIDPredicate([]string{"domain1", "domain3"}, false)
	assert.False(t, predicate1.Equals(predicate4))

	// Test different domain count
	predicate5 := NewDomainIDPredicate([]string{"domain1"}, false)
	assert.False(t, predicate1.Equals(predicate5))

	// Test different exclusion mode
	predicate6 := NewDomainIDPredicate([]string{"domain1", "domain2"}, true)
	assert.False(t, predicate1.Equals(predicate6))

	// Test against different predicate types
	universalPredicate := NewUniversalPredicate()
	emptyPredicate := NewEmptyPredicate()
	assert.False(t, predicate1.Equals(universalPredicate))
	assert.False(t, predicate1.Equals(emptyPredicate))
	assert.False(t, predicate1.Equals(nil))
}

func TestDomainIDPredicate_Equals_EmptyLists(t *testing.T) {
	// Test with both empty lists
	predicate1 := NewDomainIDPredicate([]string{}, false)
	predicate2 := NewDomainIDPredicate([]string{}, false)
	assert.True(t, predicate1.Equals(predicate2))

	// Test with one empty, one non-empty
	predicate3 := NewDomainIDPredicate([]string{"domain1"}, false)
	assert.False(t, predicate1.Equals(predicate3))

	// Test with both empty but different exclusion mode
	predicate4 := NewDomainIDPredicate([]string{}, true)
	assert.False(t, predicate1.Equals(predicate4))
}

func TestDomainIDPredicate_DuplicateDomains(t *testing.T) {
	// Test that duplicate domains are handled correctly
	predicate := NewDomainIDPredicate([]string{"domain1", "domain1", "domain2", "domain2"}, false)

	ctrl := gomock.NewController(t)

	task1 := persistence.NewMockTask(ctrl)
	task1.EXPECT().GetDomainID().Return("domain1").AnyTimes()
	task2 := persistence.NewMockTask(ctrl)
	task2.EXPECT().GetDomainID().Return("domain2").AnyTimes()
	task3 := persistence.NewMockTask(ctrl)
	task3.EXPECT().GetDomainID().Return("domain3").AnyTimes()

	assert.True(t, predicate.Check(task1))
	assert.True(t, predicate.Check(task2))
	assert.False(t, predicate.Check(task3))

	// Test equals with duplicate domains
	predicate2 := NewDomainIDPredicate([]string{"domain1", "domain2"}, false)
	assert.True(t, predicate.Equals(predicate2))
}

func TestPredicateIntegration(t *testing.T) {
	// Test integration between different predicate types
	universal := NewUniversalPredicate()
	empty := NewEmptyPredicate()
	domainInclusive := NewDomainIDPredicate([]string{"domain1"}, false)
	domainExclusive := NewDomainIDPredicate([]string{"domain1"}, true)

	ctrl := gomock.NewController(t)

	task := persistence.NewMockTask(ctrl)
	task.EXPECT().GetDomainID().Return("domain1").AnyTimes()

	// Universal should always pass
	assert.True(t, universal.Check(task))
	assert.False(t, universal.IsEmpty())

	// Empty should always fail
	assert.False(t, empty.Check(task))
	assert.True(t, empty.IsEmpty())

	// Domain inclusive should pass for matching domain
	assert.True(t, domainInclusive.Check(task))
	assert.False(t, domainInclusive.IsEmpty())

	// Domain exclusive should fail for matching domain
	assert.False(t, domainExclusive.Check(task))
	assert.False(t, domainExclusive.IsEmpty())
}
