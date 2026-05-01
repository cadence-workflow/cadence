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

package testdata

import "github.com/uber/cadence/common/types"

var (
	ShardDistributorGetShardOwnerRequest = types.GetShardOwnerRequest{
		ShardKey:  "shard-key",
		Namespace: "namespace",
	}
	ShardDistributorGetShardOwnerResponse = types.GetShardOwnerResponse{
		Owner:     "owner",
		Namespace: "namespace",
		Metadata:  map[string]string{"key-1": "value-1", "key-2": "value-2"},
	}
	ShardDistributorExecutorHeartbeatRequest = types.ExecutorHeartbeatRequest{
		Namespace:  "namespace",
		ExecutorID: "executor-id",
		Status:     types.ExecutorStatusACTIVE,
		ShardStatusReports: map[string]*types.ShardStatusReport{
			"shard-key-1": {
				Status:    types.ShardStatusREADY,
				ShardLoad: 0.5,
			},
			"shard-key-2": {
				Status:    types.ShardStatusINVALID,
				ShardLoad: 0.75,
			},
		},
		Metadata: map[string]string{
			"key-1": "value-1",
			"key-2": "value-2",
		},
	}
	ShardDistributorExecutorHeartbeatResponse = types.ExecutorHeartbeatResponse{
		ShardAssignments: map[string]*types.ShardAssignment{
			"shard-key-1": {
				Status: types.AssignmentStatusREADY,
			},
			"shard-key-2": {
				Status: types.AssignmentStatusINVALID,
			},
		},
	}
	ShardDistributorWatchNamespaceStateRequest = types.WatchNamespaceStateRequest{
		Namespace: "namespace",
	}
	ShardDistributorWatchNamespaceStateResponse = types.WatchNamespaceStateResponse{
		Executors: []*types.ExecutorShardAssignment{
			{
				ExecutorID:     "executor-1",
				AssignedShards: []*types.Shard{&types.Shard{ShardKey: "shard-1"}, &types.Shard{ShardKey: "shard-2"}},
				Metadata:       map[string]string{"key-1": "value-1"},
			},
			{
				ExecutorID:     "executor-2",
				AssignedShards: []*types.Shard{&types.Shard{ShardKey: "shard-3"}},
				Metadata:       map[string]string{"key-2": "value-2"},
			},
		},
	}
)
