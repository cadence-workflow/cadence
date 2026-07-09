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
	"context"
	"sync"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/open-feature/go-sdk/openfeature/memprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/dynamicconfig/openfeatureprovider"
	"github.com/uber/cadence/common/log"
)

// resetOpenFeatureRegistration clears NewOpenFeatureClient's process-wide
// registration guard. Tests in this file share one process (and so share
// registerOnce/registeredProviderName/attachedCount), so any test that wants
// to exercise "first successful registration" behavior needs a clean slate
// rather than silently reusing - or now, since NewOpenFeatureClient's OnStart
// rejects mismatched providers, tripping over - whatever a previous test in
// this binary already registered.
func resetOpenFeatureRegistration() {
	registerOnce = sync.Once{}
	registeredProviderName = ""
	registerErr = nil
	attachedCount = 0
}

func newInMemoryProvider(flags map[string]memprovider.InMemoryFlag) func(openfeatureprovider.Decoder) (openfeature.FeatureProvider, error) {
	return func(openfeatureprovider.Decoder) (openfeature.FeatureProvider, error) {
		return memprovider.NewInMemoryProvider(flags), nil
	}
}

func TestNewOpenFeatureClient_UnknownProvider(t *testing.T) {
	resetOpenFeatureRegistration()
	lc := fxtest.NewLifecycle(t)
	NewOpenFeatureClient(lc, "does-not-exist", nil, log.NewNoop())

	err := lc.Start(context.Background())
	assert.ErrorContains(t, err, "unknown openfeature provider")
}

func TestNewOpenFeatureClient_ConstructorError(t *testing.T) {
	resetOpenFeatureRegistration()
	providerName := t.Name()
	require.NoError(t, openfeatureprovider.Register(providerName, func(openfeatureprovider.Decoder) (openfeature.FeatureProvider, error) {
		return nil, assert.AnError
	}))

	lc := fxtest.NewLifecycle(t)
	NewOpenFeatureClient(lc, providerName, nil, log.NewNoop())

	err := lc.Start(context.Background())
	assert.ErrorContains(t, err, "failed to construct openfeature provider")
}

func TestNewOpenFeatureClient_EndToEnd(t *testing.T) {
	resetOpenFeatureRegistration()
	providerName := t.Name()
	require.NoError(t, openfeatureprovider.Register(providerName, newInMemoryProvider(map[string]memprovider.InMemoryFlag{
		"testGetBoolPropertyKey": {
			Key:            "testGetBoolPropertyKey",
			State:          memprovider.Enabled,
			DefaultVariant: "on",
			Variants:       map[string]any{"on": true},
		},
	})))

	lc := fxtest.NewLifecycle(t)
	client := NewOpenFeatureClient(lc, providerName, nil, log.NewNoop())
	lc.RequireStart()
	defer lc.RequireStop()

	val, err := client.GetBoolValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	require.NoError(t, err)
	assert.True(t, val)
}

// TestNewOpenFeatureClient_SingletonAcrossCalls guards against a regression where
// Cadence's server binary runs one fx.App per service (frontend/history/matching/
// worker) in the same process, calling NewOpenFeatureClient once per service.
// OpenFeature's default provider - and the vendor SDK behind it (unleash-client-go
// keeps a process-wide global client) - is process-wide state: registering a second
// provider makes the OpenFeature SDK shut down the "old" one, which for a
// global-singleton client tears down whichever client instance is currently live and
// deadlocks any later flag evaluation. NewOpenFeatureClient must register the provider
// at most once per process and every caller must share that one registration.
func TestNewOpenFeatureClient_SingletonAcrossCalls(t *testing.T) {
	resetOpenFeatureRegistration()
	providerName := t.Name()
	var constructCount int
	require.NoError(t, openfeatureprovider.Register(providerName, func(openfeatureprovider.Decoder) (openfeature.FeatureProvider, error) {
		constructCount++
		return memprovider.NewInMemoryProvider(map[string]memprovider.InMemoryFlag{
			"testGetBoolPropertyKey": {
				Key:            "testGetBoolPropertyKey",
				State:          memprovider.Enabled,
				DefaultVariant: "on",
				Variants:       map[string]any{"on": true},
			},
		}), nil
	}))

	lc1 := fxtest.NewLifecycle(t)
	client1 := NewOpenFeatureClient(lc1, providerName, nil, log.NewNoop())
	lc1.RequireStart()
	defer lc1.RequireStop()

	lc2 := fxtest.NewLifecycle(t)
	client2 := NewOpenFeatureClient(lc2, providerName, nil, log.NewNoop())
	lc2.RequireStart()
	defer lc2.RequireStop()

	assert.Equal(t, 1, constructCount, "expected the provider to be constructed at most once across services sharing this process")

	_ = client1
	val, err := client2.GetBoolValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	require.NoError(t, err)
	assert.True(t, val)
}

// TestNewOpenFeatureClient_RejectsMismatchedProvider ensures a second service
// requesting a different provider than the one already registered in this
// process fails its own fx.App Start() loudly, instead of silently attaching
// to someone else's provider. Cadence's config currently applies the same
// dynamicconfig.openfeature block to every service, so a mismatch here means
// a caller's config is being ignored - that must be surfaced, not hidden.
func TestNewOpenFeatureClient_RejectsMismatchedProvider(t *testing.T) {
	resetOpenFeatureRegistration()
	firstProvider := t.Name() + "_first"
	secondProvider := t.Name() + "_second"
	require.NoError(t, openfeatureprovider.Register(firstProvider, newInMemoryProvider(nil)))
	require.NoError(t, openfeatureprovider.Register(secondProvider, newInMemoryProvider(nil)))

	lc1 := fxtest.NewLifecycle(t)
	NewOpenFeatureClient(lc1, firstProvider, nil, log.NewNoop())
	lc1.RequireStart()
	defer lc1.RequireStop()

	lc2 := fxtest.NewLifecycle(t)
	NewOpenFeatureClient(lc2, secondProvider, nil, log.NewNoop())
	err := lc2.Start(context.Background())
	assert.ErrorContains(t, err, "already initialized with provider")
}

// TestNewOpenFeatureClient_ShutdownOnlyAfterLastServiceStops guards the
// refcounted teardown: openfeature.Shutdown() tears down the entire global
// OpenFeature API (every provider, hook, and event handler, process-wide),
// so it must only actually run once every attached service in this process
// has stopped - otherwise whichever service stops first would kill flag
// evaluation for the others still running.
func TestNewOpenFeatureClient_ShutdownOnlyAfterLastServiceStops(t *testing.T) {
	resetOpenFeatureRegistration()
	providerName := t.Name()
	require.NoError(t, openfeatureprovider.Register(providerName, newInMemoryProvider(map[string]memprovider.InMemoryFlag{
		"testGetBoolPropertyKey": {
			Key:            "testGetBoolPropertyKey",
			State:          memprovider.Enabled,
			DefaultVariant: "on",
			Variants:       map[string]any{"on": true},
		},
	})))

	lc1 := fxtest.NewLifecycle(t)
	NewOpenFeatureClient(lc1, providerName, nil, log.NewNoop())
	lc1.RequireStart()

	lc2 := fxtest.NewLifecycle(t)
	client2 := NewOpenFeatureClient(lc2, providerName, nil, log.NewNoop())
	lc2.RequireStart()

	lc1.RequireStop()

	// The second service is still attached: evaluation must still work,
	// proving openfeature.Shutdown() hasn't run yet.
	val, err := client2.GetBoolValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	require.NoError(t, err)
	assert.True(t, val)

	// Stopping the last attached service must not hang or panic (guards
	// against unleash-client-go-style double-close panics propagating up
	// through openfeature.Shutdown()).
	lc2.RequireStop()
}
