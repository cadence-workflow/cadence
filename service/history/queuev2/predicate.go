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

//go:generate mockgen -package $GOPACKAGE -destination predicate_mock.go github.com/uber/cadence/service/history/queuev2 Predicate

package queuev2

import (
	"maps"

	"github.com/uber/cadence/common/persistence"
)

type (
	// Predicate defines a predicate that can be used to filter tasks
	Predicate interface {
		// IsEmpty returns true if no task satisfies the predicate
		IsEmpty() bool
		// Check returns true if the task satisfies the predicate
		Check(task persistence.Task) bool
		// Equals returns true if the predicate is the same as the other predicate
		Equals(other Predicate) bool
	}

	domainIDPredicate struct {
		domainIDs   map[string]struct{}
		isExclusive bool
	}

	universalPredicate struct{}

	emptyPredicate struct{}
)

func NewUniversalPredicate() Predicate {
	return &universalPredicate{}
}

func (p *universalPredicate) IsEmpty() bool {
	return false
}

func (p *universalPredicate) Check(task persistence.Task) bool {
	return true
}

func (p *universalPredicate) Equals(other Predicate) bool {
	_, ok := other.(*universalPredicate)
	return ok
}

func NewEmptyPredicate() Predicate {
	return &emptyPredicate{}
}

func (p *emptyPredicate) IsEmpty() bool {
	return true
}

func (p *emptyPredicate) Check(task persistence.Task) bool {
	return false
}

func (p *emptyPredicate) Equals(other Predicate) bool {
	_, ok := other.(*emptyPredicate)
	return ok
}

func NewDomainIDPredicate(domainIDs []string, isExclusive bool) Predicate {
	domainIDSet := make(map[string]struct{})
	for _, domainID := range domainIDs {
		domainIDSet[domainID] = struct{}{}
	}
	return &domainIDPredicate{
		domainIDs:   domainIDSet,
		isExclusive: isExclusive,
	}
}

func (p *domainIDPredicate) IsEmpty() bool {
	return len(p.domainIDs) == 0 && !p.isExclusive
}

func (p *domainIDPredicate) Check(task persistence.Task) bool {
	if _, ok := p.domainIDs[task.GetDomainID()]; ok {
		return !p.isExclusive
	}
	return p.isExclusive
}

func (p *domainIDPredicate) Equals(other Predicate) bool {
	o, ok := other.(*domainIDPredicate)
	if !ok {
		return false
	}
	return p.isExclusive == o.isExclusive && maps.Equal(p.domainIDs, o.domainIDs)
}
