// Copyright (c) 2025 Uber Technologies, Inc.
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

// Package semaphore holds the Matching side of the distributed semaphore: the bucket
// owner, which is the counting authority for one bucket of slots.
package semaphore

import "fmt"

// Identifier names one semaphore bucket. A bucket is the unit of three things at once:
// ownership (exactly one Matching host owns it), counting (its free-set is the authority
// on which of its slots are open), and storage (it is one Cassandra partition).
//
// A semaphore of `size` slots is split into ceil(size/bucket_size) buckets, and an
// acquire reaches its bucket through f(owner_id). That routing is not built yet; a
// Bucket is constructed for an already-chosen Identifier.
//
// The struct is comparable, so it works directly as a map key — use it that way rather
// than String(), which joins user-supplied names with a separator and is meant for logs
// and metrics, where an ambiguous rendering costs nothing.
type Identifier struct {
	DomainID      string
	SemaphoreName string
	Bucket        int
}

// NewIdentifier builds a bucket identifier, rejecting the values the persistence layer
// would reject anyway. Catching them here means a misconfigured bucket fails at
// construction rather than on its first grant.
func NewIdentifier(domainID, semaphoreName string, bucket int) (Identifier, error) {
	if domainID == "" {
		return Identifier{}, fmt.Errorf("domainID is required")
	}
	if semaphoreName == "" {
		return Identifier{}, fmt.Errorf("semaphoreName is required")
	}
	if bucket < 0 {
		return Identifier{}, fmt.Errorf("bucket must not be negative, got %d", bucket)
	}
	return Identifier{DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket}, nil
}

func (id Identifier) String() string {
	return fmt.Sprintf("%s/%s/%d", id.DomainID, id.SemaphoreName, id.Bucket)
}
