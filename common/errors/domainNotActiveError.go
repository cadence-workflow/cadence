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

package errors

import (
	"fmt"
	"slices"

	"github.com/uber/cadence/common/types"
)

// NewDomainNotActiveError return a domain not exist error
func NewDomainNotActiveError(domainName string, currentCluster string, activeClusters ...string) *types.DomainNotActiveError {
	activeCluster := ""
	if len(activeClusters) == 1 {
		activeCluster = activeClusters[0]
	}
	// ensure predictable order in the error.
	slices.Sort(activeClusters)
	return &types.DomainNotActiveError{
		Message: fmt.Sprintf(
			"Domain: %s is active in cluster(s): %v, while current cluster %s is a standby cluster.",
			domainName,
			activeClusters,
			currentCluster,
		),
		DomainName:     domainName,
		CurrentCluster: currentCluster,
		ActiveCluster:  activeCluster,
		// TODO(active-active): After ActiveClusters field is introduced, uncomment this line and update following lines
		// - common/types/testdata/error.go
		// - common/testing/allisset_test.go
		// - mappers
		// ActiveClusters: activeClusters,
	}
}

// NewDomainPendingActiveError return a domain not active error
func NewDomainPendingActiveError(domainName string, currentCluster string) *types.DomainNotActiveError {
	return &types.DomainNotActiveError{
		Message: fmt.Sprintf(
			"Domain: %s is pending active in cluster: %s.",
			domainName,
			currentCluster,
		),
		DomainName:     domainName,
		CurrentCluster: currentCluster,
		ActiveCluster:  currentCluster,
	}
}
