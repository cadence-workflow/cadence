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

package partition

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination partitioning_mock.go -self_package github.com/uber/cadence/common/partition

import "context"

// PollerInfo captures relevant information from the poller side
type PollerInfo struct {
	DomainID     string
	TasklistName string
	// The isolation groups that are known to have pollers in them and are able to receive tasks
	// for this domain and tasklist.
	AvailableIsolationGroups []string
}

type Partitioner interface {
	// GetIsolationGroupByDomainID gets where the task workflow should be executing. Largely used by Matching
	// when determining which isolationGroup to place the tasks in.
	// Implementations ought to return (nil, nil) for when the feature is not enabled.
	GetIsolationGroupByDomainID(ctx context.Context, pollerinfo PollerInfo, partitionKey PartitionConfig) (string, error)
}
