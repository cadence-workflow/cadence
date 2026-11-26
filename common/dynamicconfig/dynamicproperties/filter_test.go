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

package dynamicproperties

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCollectionFilterOptions(t *testing.T) {
	tests := []struct {
		name                     string
		opts                     []CollectionFilterOption
		wantFilterOptionCount    int
		wantShardIDFilterOptions int
	}{
		{
			name: "no options",
		},
		{
			name: "only FilterOptions",
			opts: []CollectionFilterOption{
				ClusterNameFilter("cluster0"),
				DomainFilter("domain0"),
			},
			wantFilterOptionCount: 2,
		},
		{
			name: "only ShardIDFilterOptions",
			opts: []CollectionFilterOption{
				ShardIDFilterOption(func(shardID int) FilterOption { return ShardIDFilter(shardID) }),
			},
			wantShardIDFilterOptions: 1,
		},
		{
			name: "mixed",
			opts: []CollectionFilterOption{
				ClusterNameFilter("cluster0"),
				ShardIDFilterOption(func(shardID int) FilterOption { return ShardIDFilter(shardID) }),
				DomainFilter("domain0"),
			},
			wantFilterOptionCount:    2,
			wantShardIDFilterOptions: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewCollectionFilterOptions(tc.opts...)
			assert.Len(t, got.FilterOptions, tc.wantFilterOptionCount)
			assert.Len(t, got.ShardIDFilterOptions, tc.wantShardIDFilterOptions)
		})
	}
}
