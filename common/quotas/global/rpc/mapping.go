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
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/uber/cadence/.gen/go/history"
	"github.com/uber/cadence/common/quotas/global/algorithm"
	"github.com/uber/cadence/common/quotas/global/shared"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

// funcs are ordered to match how an Update request flows:
// limiter -> Update on history -> aggregator, and return.
//
// 1. updateToAny: limiter-load is gathered and converted to an Any
// 2. AnyToAggregatorUpdate: the Any is received by a history host and decoded for an aggregator
// 3. AggregatorWeightsToAny: the aggregator responds, and is converted to another Any
// 4. anyToWeights: the response is decoded to get the weights to use
//
// to make the rpc API generic for other kinds of data / multiple algorithms, just inject
// these mappers so the request sharding can encode data per peer as needed.
// e.g. use a mapper with a `peer_callback(keys for peer) Any` so it does not need to
// know what kind of data is being converted.

func updateToAny(host string, elapsed time.Duration, load map[shared.GlobalKey]Calls) (*types.Any, error) {
	calls := make(map[string]*history.WeightedRatelimitCalls, len(load))
	for k, v := range load {
		calls[string(k)] = &history.WeightedRatelimitCalls{
			Allowed:  saturatingInt32(v.Allowed),
			Rejected: saturatingInt32(v.Rejected),
		}
	}
	req := &history.WeightedRatelimitUsage{
		Caller:    host,
		ElapsedMS: saturatingInt32(elapsed / time.Millisecond),
		Calls:     calls,
	}
	bytes, err := thrift.EncodeToBytes(req)
	if err != nil {
		// should be impossible
		return nil, &SerializationError{err}
	}
	return &types.Any{
		ValueType: history.WeightedRatelimitUsageAnyType,
		Value:     bytes,
	}, nil
}

// AnyToAggregatorUpdate converts an in-bound ratelimiter's Any-typed data to the
// structure that [algorithm.RequestWeighted.Update] expects.
func AnyToAggregatorUpdate(request *types.Any) (algorithm.UpdateParams, error) {
	if request.ValueType != history.WeightedRatelimitUsageAnyType {
		return algorithm.UpdateParams{}, fmt.Errorf("unrecognized Any type: %q", request.ValueType)
	}
	var out history.WeightedRatelimitUsage
	err := thrift.DecodeStructFromBytes(request.Value, &out)
	if err != nil {
		return algorithm.UpdateParams{}, &SerializationError{err}
	}
	load := make(map[algorithm.Limit]algorithm.Requests, len(out.Calls))
	for k, v := range out.Calls {
		load[algorithm.Limit(k)] = algorithm.Requests{
			Accepted: int(v.Allowed),
			Rejected: int(v.Rejected),
		}
	}
	par := algorithm.UpdateParams{
		ID:      algorithm.Identity(out.Caller),
		Elapsed: time.Duration(out.ElapsedMS) * time.Millisecond,
		Load:    load,
	}
	return par, par.Validate()
}

// AggregatorWeightsToAny converts the [algorithm.RequestWeighted.HostWeights] response
// (for an in-bound Update request) to an Any-type compatible with RPC.
func AggregatorWeightsToAny(response map[algorithm.Limit]algorithm.HostUsage) (*types.Any, error) {
	quotas := make(map[string]*history.WeightedRatelimitUsageQuotaEntry, len(response))
	for k, v := range response {
		quotas[string(k)] = &history.WeightedRatelimitUsageQuotaEntry{
			Weight: float64(v.Weight),
			Used:   float64(v.Used),
		}
	}
	wrapper := &history.WeightedRatelimitUsageQuotas{
		Quotas: quotas,
	}
	data, err := thrift.EncodeToBytes(wrapper)
	if err != nil {
		// should be impossible
		return nil, &SerializationError{err}
	}
	return &types.Any{
		ValueType: history.WeightedRatelimitUsageQuotasAnyType,
		Value:     data,
	}, nil
}

func anyToWeights(response *types.Any) (map[shared.GlobalKey]UpdateEntry, error) {
	if response.ValueType != history.WeightedRatelimitUsageQuotasAnyType {
		return nil, fmt.Errorf("unrecognized Any type: %q", response.ValueType)
	}
	var out history.WeightedRatelimitUsageQuotas
	err := thrift.DecodeStructFromBytes(response.Value, &out)
	if err != nil {
		return nil, &SerializationError{err}
	}
	result := make(map[shared.GlobalKey]UpdateEntry, len(out.Quotas))
	for k, v := range out.Quotas {
		result[shared.GlobalKey(k)] = UpdateEntry{
			Weight:  v.Weight,
			UsedRPS: v.Used,
		}
	}
	return result, nil
}

type numeric interface {
	int | time.Duration
}

func saturatingInt32[T numeric](i T) int32 {
	if i > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(i)
}

// exposed only for testing purposes

// TestUpdateToAny is exposed for handler tests, use updateToAny in internal code instead.
func TestUpdateToAny(t *testing.T, host string, elapsed time.Duration, load map[shared.GlobalKey]Calls) (*types.Any, error) {
	t.Helper()
	return updateToAny(host, elapsed, load)
}

// TestAnyToWeights is exposed for handler tests, use anyToWeights in internal code instead
func TestAnyToWeights(t *testing.T, response *types.Any) (map[shared.GlobalKey]UpdateEntry, error) {
	t.Helper()
	return anyToWeights(response)
}
