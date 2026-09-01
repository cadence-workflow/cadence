// Copyright (c) 2026 Uber Technologies, Inc.
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

package semaphore

import "fmt"

// Identifier names one semaphore bucket. A semaphore of `size` slots is split into
// ceil(size/bucket_size) buckets.
//
// A bucket is the unit of three things at once, and the design depends on all three lining
// up: exactly one Matching host owns it, it is one partition of semaphore_tokens, and every
// grant against it is one conditional write to that partition.
//
// The struct is used directly as a map key, so keep every field comparable. String() is
// for logs and metrics.
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
