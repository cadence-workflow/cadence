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

package openfeatureclient

import (
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/open-feature/go-sdk/openfeature/memprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/dynamicconfig/openfeatureprovider"
	"github.com/uber/cadence/common/log"
)

func TestNewOpenFeatureClient_UnknownProvider(t *testing.T) {
	_, err := NewOpenFeatureClient("does-not-exist", nil, log.NewNoop())
	assert.ErrorContains(t, err, "unknown openfeature provider")
}

func TestNewOpenFeatureClient_ConstructorError(t *testing.T) {
	providerName := t.Name()
	require.NoError(t, openfeatureprovider.Register(providerName, func(openfeatureprovider.Decoder) (openfeature.FeatureProvider, error) {
		return nil, assert.AnError
	}))

	_, err := NewOpenFeatureClient(providerName, nil, log.NewNoop())
	assert.ErrorContains(t, err, "failed to construct openfeature provider")
}

func TestNewOpenFeatureClient_EndToEnd(t *testing.T) {
	providerName := t.Name()
	require.NoError(t, openfeatureprovider.Register(providerName, func(openfeatureprovider.Decoder) (openfeature.FeatureProvider, error) {
		return memprovider.NewInMemoryProvider(map[string]memprovider.InMemoryFlag{
			"testGetBoolPropertyKey": {
				Key:            "testGetBoolPropertyKey",
				State:          memprovider.Enabled,
				DefaultVariant: "on",
				Variants:       map[string]any{"on": true},
			},
		}), nil
	}))

	client, err := NewOpenFeatureClient(providerName, nil, log.NewNoop())
	require.NoError(t, err)

	val, err := client.GetBoolValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	require.NoError(t, err)
	assert.True(t, val)
}
