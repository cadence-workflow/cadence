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

package rpc

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas/global/algorithm"
	"github.com/uber/cadence/common/quotas/global/shared"
	"github.com/uber/cadence/common/types"
)

func TestClient(t *testing.T) {
	setup := func(t *testing.T) (c *client, hc *history.MockClient, pr *history.MockPeerResolver) {
		ctrl := gomock.NewController(t)
		hc = history.NewMockClient(ctrl)
		pr = history.NewMockPeerResolver(ctrl)
		impl := New(hc, pr, testlogger.New(t), metrics.NewNoopMetricsClient())
		return impl.(*client), hc, pr
	}

	encode := func(t *testing.T, w map[algorithm.Limit]algorithm.HostUsage) (*types.RatelimitUpdateResponse, error) {
		a, err := AggregatorWeightsToAny(w)
		assert.NoError(t, err, "should be impossible: could not encode aggregator response to any")
		return &types.RatelimitUpdateResponse{
			Any: a,
		}, nil
	}

	t.Run("valid request", func(t *testing.T) {
		c, hc, pr := setup(t)
		data := map[shared.GlobalKey]Calls{
			"a": {10, 20},
			"b": {3, 5},
			"c": {78, 9},
		}
		// make a realistic used-RPS value (assuming 1 second elapsed)
		used := float64(0)
		for _, calls := range data {
			used += float64(calls.Allowed + calls.Rejected)
		}
		response := map[shared.GlobalKey]UpdateEntry{
			"a": {Weight: 0.1, UsedRPS: used},
			"b": {Weight: 0.2, UsedRPS: used},
			"c": {Weight: 0.3, UsedRPS: used},
		}
		stringKeys := make([]string, 0, len(data))
		for k := range data {
			stringKeys = append(stringKeys, string(k))
		}
		pr.EXPECT().GlobalRatelimitPeers(gomock.InAnyOrder(stringKeys)).Return(map[history.Peer][]string{
			"agg-1": {"a", "c"},
			"agg-2": {"b"},
		}, nil)
		hc.EXPECT().
			RatelimitUpdate(gomock.Any(), matchrequest{t, []string{"a", "c"}}, matchyarpc{t, yarpc.WithShardKey("agg-1")}).
			Return(encode(t, map[algorithm.Limit]algorithm.HostUsage{"a": {Weight: 0.1, Used: algorithm.PerSecond(used)}, "c": {Weight: 0.3, Used: algorithm.PerSecond(used)}}))
		hc.EXPECT().
			RatelimitUpdate(gomock.Any(), matchrequest{t, []string{"b"}}, matchyarpc{t, yarpc.WithShardKey("agg-2")}).
			Return(encode(t, map[algorithm.Limit]algorithm.HostUsage{"b": {Weight: 0.2, Used: algorithm.PerSecond(used)}}))

		result := c.Update(context.Background(), time.Second, data)
		assert.NoError(t, result.Err)
		assert.Equal(t, response, result.Weights)
	})
	t.Run("bad peers", func(t *testing.T) {
		c, _, pr := setup(t)
		err := errors.New("peer issue")
		pr.EXPECT().GlobalRatelimitPeers(gomock.Any()).Return(nil, err)
		res := c.Update(context.Background(), time.Second, nil)
		assert.ErrorContains(t, res.Err, "unable to shard")
		assert.ErrorIs(t, res.Err, err) // should contain the cause
	})
	t.Run("bad math", func(t *testing.T) {
		// the client should validate responses to make sure they do not contain
		// known-to-be-invalid data.
		c, hc, pr := setup(t)
		pr.EXPECT().GlobalRatelimitPeers([]string{"a"}).
			Return(map[history.Peer][]string{"agg-1": {"a"}}, nil)
		hc.EXPECT().RatelimitUpdate(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(encode(t, map[algorithm.Limit]algorithm.HostUsage{"a": {
				Weight: algorithm.HostWeight(math.NaN()), // invalid value from agg, detected after deserializing
				Used:   1,
			}}))

		result := c.Update(context.Background(), time.Second, map[shared.GlobalKey]Calls{"a": {10, 20}})
		assert.Empty(t, result.Weights, "bad data should lead to no usable response")
		assert.ErrorContains(t, result.Err, `bad value for key "a"`, "failure should mention the key")
		assert.ErrorContains(t, result.Err, "is NaN", "failure should mention the bad value type")
		assert.ErrorContains(t, result.Err, "potentially fatal error during ratelimit update requests", "failure should look alarming")
	})
}

func stringy[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i := range in {
		out[i] = string(in[i])
	}
	return out
}

// matches a single yarpc option
type matchyarpc struct {
	t   *testing.T
	opt yarpc.CallOption
}

var _ gomock.Matcher = matchyarpc{}

func (m matchyarpc) Matches(x interface{}) bool {
	opts, ok := x.([]yarpc.CallOption)
	if !ok {
		m.t.Logf("not a []yarpc.CallOption: %T", x)
		return false
	}
	m.t.Logf("opts: %#v, should be: %#v", opts, m.opt)
	return len(opts) == 1 && reflect.DeepEqual(opts[0], m.opt)
}

func (m matchyarpc) String() string {
	return fmt.Sprintf("%#v", m.opt)
}

// matches load-keys in an update request
type matchrequest struct {
	t        *testing.T
	loadKeys []string
}

var _ gomock.Matcher = matchrequest{}

func (a matchrequest) Matches(x interface{}) bool {
	v, ok := x.(*types.RatelimitUpdateRequest)
	if !ok {
		a.t.Logf("not a *types.RatelimitUpdateRequest: %T", x)
		return false
	}
	up, err := AnyToAggregatorUpdate(v.Any)
	if err != nil {
		a.t.Logf("failed to decode to agg update: %v", err)
		return false
	}
	keys := map[string]struct{}{}
	for _, k := range a.loadKeys {
		keys[k] = struct{}{}
	}
	gotKeys := map[string]struct{}{}
	for _, k := range stringy(slices.Collect(maps.Keys(up.Load))) {
		gotKeys[k] = struct{}{}
	}
	a.t.Logf("want keys: %v, got keys: %v", maps.Keys(keys), maps.Keys(gotKeys))
	return reflect.DeepEqual(keys, gotKeys)
}

func (a matchrequest) String() string {
	return fmt.Sprintf("request with load keys: %v", a.loadKeys)
}
