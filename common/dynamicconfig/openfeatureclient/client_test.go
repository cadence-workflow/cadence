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
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/open-feature/go-sdk/openfeature/memprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/dynamicconfig/openfeatureprovider"
	"github.com/uber/cadence/common/log"
)

// resetOpenFeatureRegistration clears RegisterProvider's process-wide
// registration guard. Tests in this file share one process (and so share
// registerOnce/registeredProviderName/attachedCount), so any test that wants
// to exercise "first successful registration" behavior needs a clean slate
// rather than silently reusing - or, since RegisterProvider rejects
// mismatched providers, tripping over - whatever a previous test in this
// binary already registered.
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

// newTestClient registers a fresh in-memory provider backing flags, attaches
// a dynamicconfig.Client to it, and tears both down at test cleanup.
func newTestClient(t *testing.T, flags map[string]memprovider.InMemoryFlag) dynamicconfig.Client {
	t.Helper()
	resetOpenFeatureRegistration()
	providerName := t.Name()
	require.NoError(t, openfeatureprovider.Register(providerName, newInMemoryProvider(flags)))

	client := NewOpenFeatureClient(log.NewNoop())
	require.NoError(t, RegisterProvider(context.Background(), providerName, nil))
	t.Cleanup(DeregisterProvider)
	return client
}

func staticVariant(key, variant string, value any) memprovider.InMemoryFlag {
	return memprovider.InMemoryFlag{
		Key:            key,
		State:          memprovider.Enabled,
		DefaultVariant: variant,
		Variants:       map[string]any{variant: value},
	}
}

func TestRegisterProvider_UnknownProvider(t *testing.T) {
	resetOpenFeatureRegistration()
	err := RegisterProvider(context.Background(), "does-not-exist", nil)
	assert.ErrorContains(t, err, "unknown openfeature provider")
}

func TestRegisterProvider_ConstructorError(t *testing.T) {
	resetOpenFeatureRegistration()
	providerName := t.Name()
	require.NoError(t, openfeatureprovider.Register(providerName, func(openfeatureprovider.Decoder) (openfeature.FeatureProvider, error) {
		return nil, assert.AnError
	}))

	err := RegisterProvider(context.Background(), providerName, nil)
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

	client := NewOpenFeatureClient(log.NewNoop())
	require.NoError(t, RegisterProvider(context.Background(), providerName, nil))
	defer DeregisterProvider()

	val, err := client.GetBoolValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	require.NoError(t, err)
	assert.True(t, val)
}

// TestGetValue_TranslatesFlagNotFoundToNotFoundError guards against WARN-level
// log noise: Cadence declares hundreds of dynamic config keys and only a
// handful are ever expected to be set in an external flag platform like
// Unleash, so evaluating an undefined key is the common case, not a real
// failure. dynamicconfig.Collection logs dynamicconfig.NotFoundError (a
// *types.EntityNotExistsError) at DEBUG and anything else at WARN - so the raw
// FLAG_NOT_FOUND resolution error from OpenFeature must be translated, or
// every unset key would log at WARN on every access.
func TestGetValue_TranslatesFlagNotFoundToNotFoundError(t *testing.T) {
	resetOpenFeatureRegistration()
	providerName := t.Name()
	require.NoError(t, openfeatureprovider.Register(providerName, newInMemoryProvider(nil)))

	client := NewOpenFeatureClient(log.NewNoop())
	require.NoError(t, RegisterProvider(context.Background(), providerName, nil))
	defer DeregisterProvider()

	_, err := client.GetBoolValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	assert.Equal(t, dynamicconfig.NotFoundError, err)
}

// TestRegisterProvider_SingletonAcrossCalls guards against a regression where
// Cadence's server binary runs one fx.App per service (frontend/history/matching/
// worker) in the same process, calling RegisterProvider once per service.
// OpenFeature's default provider - and the vendor SDK behind it (unleash-client-go
// keeps a process-wide global client) - is process-wide state: registering a second
// provider makes the OpenFeature SDK shut down the "old" one, which for a
// global-singleton client tears down whichever client instance is currently live and
// deadlocks any later flag evaluation. RegisterProvider must register the provider
// at most once per process and every caller must share that one registration.
func TestRegisterProvider_SingletonAcrossCalls(t *testing.T) {
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

	require.NoError(t, RegisterProvider(context.Background(), providerName, nil))
	defer DeregisterProvider()
	require.NoError(t, RegisterProvider(context.Background(), providerName, nil))
	defer DeregisterProvider()

	assert.Equal(t, 1, constructCount, "expected the provider to be constructed at most once across services sharing this process")

	client := NewOpenFeatureClient(log.NewNoop())
	val, err := client.GetBoolValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	require.NoError(t, err)
	assert.True(t, val)
}

// TestRegisterProvider_RejectsMismatchedProvider ensures a second service
// requesting a different provider than the one already registered in this
// process fails loudly, instead of silently attaching to someone else's
// provider. Cadence's config currently applies the same
// dynamicconfig.openfeature block to every service, so a mismatch here means
// a caller's config is being ignored - that must be surfaced, not hidden.
func TestRegisterProvider_RejectsMismatchedProvider(t *testing.T) {
	resetOpenFeatureRegistration()
	firstProvider := t.Name() + "_first"
	secondProvider := t.Name() + "_second"
	require.NoError(t, openfeatureprovider.Register(firstProvider, newInMemoryProvider(nil)))
	require.NoError(t, openfeatureprovider.Register(secondProvider, newInMemoryProvider(nil)))

	require.NoError(t, RegisterProvider(context.Background(), firstProvider, nil))
	defer DeregisterProvider()

	err := RegisterProvider(context.Background(), secondProvider, nil)
	assert.ErrorContains(t, err, "already registered in this process")
}

// TestDeregisterProvider_ShutdownOnlyAfterLastServiceStops guards the
// refcounted teardown: openfeature.Shutdown() tears down the entire global
// OpenFeature API (every provider, hook, and event handler, process-wide),
// so it must only actually run once every attached service in this process
// has stopped - otherwise whichever service stops first would kill flag
// evaluation for the others still running.
func TestDeregisterProvider_ShutdownOnlyAfterLastServiceStops(t *testing.T) {
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

	require.NoError(t, RegisterProvider(context.Background(), providerName, nil))
	require.NoError(t, RegisterProvider(context.Background(), providerName, nil))

	client := NewOpenFeatureClient(log.NewNoop())

	DeregisterProvider()

	// The second service is still attached: evaluation must still work,
	// proving openfeature.Shutdown() hasn't run yet.
	val, err := client.GetBoolValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	require.NoError(t, err)
	assert.True(t, val)

	// Stopping the last attached service must not hang or panic (guards
	// against unleash-client-go-style double-close panics propagating up
	// through openfeature.Shutdown()).
	DeregisterProvider()
}

func TestGetValue(t *testing.T) {
	key := dynamicproperties.TestGetIntPropertyKey
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): staticVariant(key.String(), "on", int64(42)),
	})

	val, err := client.GetValue(key)
	require.NoError(t, err)
	assert.EqualValues(t, 42, val)
}

func TestGetValueWithFilters_NotFound_ReturnsDefault(t *testing.T) {
	key := dynamicproperties.TestGetIntPropertyKey
	client := newTestClient(t, nil)

	val, err := client.GetValueWithFilters(key, map[dynamicproperties.Filter]interface{}{dynamicproperties.DomainName: "some-domain"})
	assert.Equal(t, dynamicconfig.NotFoundError, err)
	assert.Equal(t, key.DefaultValue(), val)
}

// TestGetValueWithFilters_PropagatesFiltersAsContextAttributes guards
// toEvalContext: Cadence's Filter/value pairs must reach the provider as
// EvaluationContext attributes, since that's how targeting rules (e.g. "only
// enable for domain X") match against them.
func TestGetValueWithFilters_PropagatesFiltersAsContextAttributes(t *testing.T) {
	key := dynamicproperties.TestGetStringPropertyKey
	var evaluator memprovider.ContextEvaluator
	fn := func(this memprovider.InMemoryFlag, flatCtx openfeature.FlattenedContext) (any, openfeature.ProviderResolutionDetail) {
		domain, _ := flatCtx[dynamicproperties.DomainName.String()].(string)
		return domain, openfeature.ProviderResolutionDetail{Reason: openfeature.TargetingMatchReason}
	}
	evaluator = &fn
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): {
			Key:              key.String(),
			State:            memprovider.Enabled,
			ContextEvaluator: evaluator,
		},
	})

	val, err := client.GetStringValue(key, map[dynamicproperties.Filter]interface{}{dynamicproperties.DomainName: "my-domain"})
	require.NoError(t, err)
	assert.Equal(t, "my-domain", val)
}

func TestGetIntValue(t *testing.T) {
	key := dynamicproperties.TestGetIntPropertyKey
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): staticVariant(key.String(), "on", 7),
	})

	val, err := client.GetIntValue(key, nil)
	require.NoError(t, err)
	assert.Equal(t, 7, val)
}

func TestGetIntValue_NotFound_ReturnsDefault(t *testing.T) {
	key := dynamicproperties.TestGetIntPropertyKey
	client := newTestClient(t, nil)

	val, err := client.GetIntValue(key, nil)
	assert.Equal(t, dynamicconfig.NotFoundError, err)
	assert.Equal(t, key.DefaultInt(), val)
}

func TestGetFloatValue(t *testing.T) {
	key := dynamicproperties.TestGetFloat64PropertyKey
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): staticVariant(key.String(), "on", 3.14),
	})

	val, err := client.GetFloatValue(key, nil)
	require.NoError(t, err)
	assert.Equal(t, 3.14, val)
}

func TestGetStringValue(t *testing.T) {
	key := dynamicproperties.TestGetStringPropertyKey
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): staticVariant(key.String(), "on", "hello"),
	})

	val, err := client.GetStringValue(key, nil)
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
}

func TestGetMapValue(t *testing.T) {
	key := dynamicproperties.TestGetMapPropertyKey
	want := map[string]interface{}{"a": "b"}
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): staticVariant(key.String(), "on", want),
	})

	val, err := client.GetMapValue(key, nil)
	require.NoError(t, err)
	assert.Equal(t, want, val)
}

// TestGetMapValue_TypeMismatch guards the explicit type assertion in
// GetMapValue: OpenFeature's ObjectEvaluation has no per-type contract, so a
// provider returning a non-map value for a map key must surface a clear
// error instead of a panic on the failed assertion.
func TestGetMapValue_TypeMismatch(t *testing.T) {
	key := dynamicproperties.TestGetMapPropertyKey
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): staticVariant(key.String(), "on", "not-a-map"),
	})

	val, err := client.GetMapValue(key, nil)
	assert.ErrorContains(t, err, "value type is not map but is")
	assert.Equal(t, key.DefaultMap(), val)
}

// TestGetDurationValue_StoredAsParseableString guards the convention noted
// on GetDurationValue: OpenFeature has no native duration type, so durations
// round-trip as time.ParseDuration-compatible strings, matching the
// file-based client's on-disk format.
func TestGetDurationValue_StoredAsParseableString(t *testing.T) {
	key := dynamicproperties.TestGetDurationPropertyKey
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): staticVariant(key.String(), "on", "5s"),
	})

	val, err := client.GetDurationValue(key, nil)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, val)
}

func TestGetDurationValue_UnparseableString(t *testing.T) {
	key := dynamicproperties.TestGetDurationPropertyKey
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): staticVariant(key.String(), "on", "not-a-duration"),
	})

	val, err := client.GetDurationValue(key, nil)
	assert.ErrorContains(t, err, "failed to parse duration")
	assert.Equal(t, key.DefaultDuration(), val)
}

func TestGetListValue(t *testing.T) {
	key := dynamicproperties.TestGetListPropertyKey
	want := []interface{}{"a", "b"}
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): staticVariant(key.String(), "on", want),
	})

	val, err := client.GetListValue(key, nil)
	require.NoError(t, err)
	assert.Equal(t, want, val)
}

// TestGetListValue_TypeMismatch mirrors TestGetMapValue_TypeMismatch for the
// list-typed getter's own type assertion.
func TestGetListValue_TypeMismatch(t *testing.T) {
	key := dynamicproperties.TestGetListPropertyKey
	client := newTestClient(t, map[string]memprovider.InMemoryFlag{
		key.String(): staticVariant(key.String(), "on", "not-a-list"),
	})

	val, err := client.GetListValue(key, nil)
	assert.ErrorContains(t, err, "value type is not list but is")
	assert.Equal(t, key.DefaultList(), val)
}

// TestUnsupportedWriteOperations guards the documented behavior that
// OpenFeature is a read/evaluation API with no admin write path: mutation
// happens in the provider's own control plane, not through this client.
func TestUnsupportedWriteOperations(t *testing.T) {
	client := newTestClient(t, nil)

	err := client.UpdateValue(dynamicproperties.TestGetIntPropertyKey, 1)
	assert.ErrorContains(t, err, "not supported for openfeature client")

	err = client.RestoreValue(dynamicproperties.TestGetIntPropertyKey, nil)
	assert.ErrorContains(t, err, "not supported for openfeature client")

	entries, err := client.ListValue(dynamicproperties.TestGetIntPropertyKey)
	assert.ErrorContains(t, err, "not supported for openfeature client")
	assert.Nil(t, entries)
}
