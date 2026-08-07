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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/quotas/global/algorithm"
	"github.com/uber/cadence/common/quotas/global/shared"
	"github.com/uber/cadence/common/types"
)

func TestMapping(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		host := uuid.NewString()
		elapsed := 3 * time.Second

		inLoad := map[shared.GlobalKey]Calls{
			"domain-x-user":   {1, 2},
			"domain-y-worker": {3, 4},
		}
		// same contents but different types
		outLoad := map[algorithm.Limit]algorithm.Requests{
			"domain-x-user":   {1, 2},
			"domain-y-worker": {3, 4},
		}

		intermediate, err := updateToAny(host, elapsed, inLoad)
		require.NoError(t, err)

		params, err := AnyToAggregatorUpdate(intermediate)
		require.NoError(t, err)
		assert.EqualValues(t, host, params.ID)
		assert.Equal(t, elapsed, params.Elapsed)
		assert.Equal(t, outLoad, params.Load)
	})
	t.Run("response", func(t *testing.T) {
		inResponse := map[algorithm.Limit]algorithm.HostUsage{
			"domain-x-user":   {Weight: 0.5, Used: 10},
			"domain-y-worker": {Weight: 0.3, Used: 20},
		}
		// same contents but different types
		outResponse := map[shared.GlobalKey]UpdateEntry{
			"domain-x-user":   {Weight: 0.5, UsedRPS: 10},
			"domain-y-worker": {Weight: 0.3, UsedRPS: 20},
		}

		intermediate, err := AggregatorWeightsToAny(inResponse)
		require.NoError(t, err)

		weights, err := anyToWeights(intermediate)
		require.NoError(t, err)
		assert.Equal(t, outResponse, weights)
	})
	t.Run("bad data", func(t *testing.T) {
		validUpdate, err := updateToAny(uuid.NewString(), time.Second, map[shared.GlobalKey]Calls{"domain-x-user": {1, 2}})
		require.NoError(t, err)
		validResponse, err := AggregatorWeightsToAny(map[algorithm.Limit]algorithm.HostUsage{"domain-x-user": {Weight: 0.7}})
		require.NoError(t, err)

		_, err = AnyToAggregatorUpdate(validUpdate)
		require.NoError(t, err, "sanity check failed, starting data should be valid")
		_, err = anyToWeights(validResponse)
		require.NoError(t, err, "sanity check failed, starting data should be valid")

		withType := func(valueType string, in *types.Any) *types.Any {
			dup := *in
			dup.ValueType = valueType
			return &dup
		}
		withData := func(data []byte, in *types.Any) *types.Any {
			dup := *in
			dup.Value = data
			return &dup
		}
		toUpdate := func(data *types.Any) error {
			_, err := AnyToAggregatorUpdate(data)
			return err
		}
		toWeights := func(data *types.Any) error {
			_, err := anyToWeights(data)
			return err
		}

		tests := map[string]struct {
			value       *types.Any
			errContains string
			do          func(data *types.Any) error
		}{
			"wrong value type update": {
				value:       withType("other", validUpdate),
				do:          toUpdate,
				errContains: "other", // should reference the wrong value
			},
			"wrong value type weights": {
				value:       withType("other", validResponse),
				do:          toWeights,
				errContains: "other", // should reference the wrong value
			},
			"bad data update": {
				value:       withData([]byte("bad"), validUpdate),
				do:          toUpdate,
				errContains: "could not decode",
			},
			"bad data weights": {
				value:       withData([]byte("bad"), validResponse),
				do:          toWeights,
				errContains: "could not decode",
			},
			"empty data update": {
				value:       withData(nil, validUpdate),
				do:          toUpdate,
				errContains: "could not decode",
			},
			"empty data weights": {
				value:       withData(nil, validResponse),
				do:          toWeights,
				errContains: "could not decode",
			},
			"swapped types update": {
				value:       validUpdate,
				do:          toWeights,
				errContains: "unrecognized Any type",
			},
			"swapped types weights": {
				value:       validResponse,
				do:          toUpdate,
				errContains: "unrecognized Any type",
			},
		}
		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				require.ErrorContains(t, test.do(test.value), test.errContains)
			})
		}
	})
}
