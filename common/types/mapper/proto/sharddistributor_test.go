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

package proto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/testdata"
)

func TestFromShardDistributorGetShardOwnerRequest(t *testing.T) {
	for _, item := range []*types.GetShardOwnerRequest{nil, {}, &testdata.ShardDistributorGetShardOwnerRequest} {
		assert.Equal(t, item, ToShardDistributorGetShardOwnerRequest(FromShardDistributorGetShardOwnerRequest(item)))
	}
}

func TestFromShardDistributorGetShardOwnerResponse(t *testing.T) {
	for _, item := range []*types.GetShardOwnerResponse{nil, {}, &testdata.ShardDistributorGetShardOwnerResponse} {
		assert.Equal(t, item, ToShardDistributorGetShardOwnerResponse(FromShardDistributorGetShardOwnerResponse(item)))
	}
}

func TestFromShardDistributorExecutorHeartbeatRequest(t *testing.T) {
	for _, item := range []*types.ExecutorHeartbeatRequest{nil, {}, &testdata.ShardDistributorExecutorHeartbeatRequest} {
		assert.Equal(t, item, ToShardDistributorExecutorHeartbeatRequest(FromShardDistributorExecutorHeartbeatRequest(item)))
	}
}

func TestToShardDistributorExecutorHeartbeatResponse(t *testing.T) {
	for _, item := range []*types.ExecutorHeartbeatResponse{nil, {}, &testdata.ShardDistributorExecutorHeartbeatResponse} {
		assert.Equal(t, item, ToShardDistributorExecutorHeartbeatResponse(FromShardDistributorExecutorHeartbeatResponse(item)))
	}
}

func TestFromShardDistributorWatchNamespaceStateRequest(t *testing.T) {
	for _, item := range []*types.WatchNamespaceStateRequest{nil, {}, &testdata.ShardDistributorWatchNamespaceStateRequest} {
		assert.Equal(t, item, ToShardDistributorWatchNamespaceStateRequest(FromShardDistributorWatchNamespaceStateRequest(item)))
	}
}

func TestFromShardDistributorWatchNamespaceStateResponse(t *testing.T) {
	for _, item := range []*types.WatchNamespaceStateResponse{nil, {}, &testdata.ShardDistributorWatchNamespaceStateResponse} {
		assert.Equal(t, item, ToShardDistributorWatchNamespaceStateResponse(FromShardDistributorWatchNamespaceStateResponse(item)))
	}
}
