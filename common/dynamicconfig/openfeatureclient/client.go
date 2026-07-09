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

// Package openfeatureclient implements dynamicconfig.Client on top of
// OpenFeature. Its YAML-decodable Config type lives in the
// common/dynamicconfig/openfeatureclient/config subpackage instead of here,
// specifically so that common/config (which embeds that Config type) doesn't
// transitively import github.com/open-feature/go-sdk/openfeature: that SDK
// starts a background goroutine (its event executor) unconditionally from
// its own package init(), the moment anything imports it - not only when a
// provider is actually configured. common/config is imported by nearly every
// binary and test package in this repo, so if it pulled in the OpenFeature
// SDK, that goroutine would exist in every one of them, permanently, and
// break any test using goleak.VerifyNone. Only dynamicconfigfx.New (which
// backs an actual running server) should import this package.
package openfeatureclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/open-feature/go-sdk/openfeature"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/dynamicconfig/openfeatureprovider"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

var _ dynamicconfig.Client = (*openFeatureClient)(nil)

type openFeatureClient struct {
	client *openfeature.Client
	logger log.Logger
}

// registerOnce guards openfeature.SetProviderWithContextAndWait. OpenFeature's
// default provider - and vendor SDKs behind it such as unleash-client-go,
// which keeps a single package-level client - is process-wide state, not
// per-caller state. Cadence's server binary runs one fx.App per service
// (frontend/history/matching/worker) in the same OS process, so
// RegisterProvider gets called once per service: without this guard, each
// call would register a new default provider and make the OpenFeature SDK
// shut down the previous one, which for a global-singleton client tears down
// whichever client instance is currently live and deadlocks any later flag
// evaluation. Registration happens at most once per process, inside the
// first service's call to run; every other service's call just verifies
// it's requesting the same provider and attaches to it.
//
// registeredProviderName records which provider "won" the race so a later
// call requesting a *different* provider fails loudly instead of silently
// reusing the first one's - Cadence's config currently applies the same
// dynamicconfig.openfeature block to every service, so a mismatch here means
// a caller's config is being ignored, not honored.
//
// attachedCount tracks how many services are currently attached to the
// shared provider (incremented by a successful RegisterProvider, decremented
// by a matching DeregisterProvider). openfeature.Shutdown() tears down the
// *entire* global OpenFeature API (every provider, hook, and event handler,
// process-wide), so it must only run once every attached service has
// stopped - otherwise whichever service stops first would kill flag
// evaluation for the others still running in this process.
var (
	registerOnce           sync.Once
	registeredProviderName string
	registerErr            error

	attachedMu    sync.Mutex
	attachedCount int
)

// NewOpenFeatureClient creates a dynamicconfig.Client backed by OpenFeature.
// The returned client is usable immediately - it wraps
// openfeature.NewDefaultClient(), which evaluates against whatever provider
// is live at call time and falls back to OpenFeature's own no-op default
// provider until one is registered via RegisterProvider.
func NewOpenFeatureClient(logger log.Logger) dynamicconfig.Client {
	return &openFeatureClient{
		client: openfeature.NewDefaultClient(),
		logger: logger,
	}
}

// RegisterProvider registers providerName's OpenFeature provider (looked up
// via openfeatureprovider.Get - see that package's doc for the plugin
// convention) as the process-wide default provider, at most once per
// process: every call after the first successful one just verifies
// providerName matches what's already registered and attaches to it.
// providerConfig is the provider plugin's own config, decoded lazily so this
// package never depends on any provider-specific config type.
//
// This does blocking work (openfeature.SetProviderWithContextAndWait, which
// blocks on the provider's own readiness) and so is meant to be called from
// an fx.Hook's OnStart (see dynamicconfigfx.New), matching how other
// blocking/validating work in this codebase is deferred to fx.StartHook
// (common/config/fx.go's cfg.validate) rather than run eagerly inside an
// fx.Provide constructor. ctx should be the OnStart context fx provides,
// which carries fx's own start timeout - without it, a provider that never
// becomes ready (e.g. an unreachable Unleash server) would block
// fx.App.Start() forever instead of failing after that timeout. A returned
// error should fail the caller's fx.App.Start().
func RegisterProvider(ctx context.Context, providerName string, providerConfig openfeatureprovider.Decoder) error {
	registerOnce.Do(func() {
		constructor, ok := openfeatureprovider.Get(providerName)
		if !ok {
			registerErr = fmt.Errorf("unknown openfeature provider: %q", providerName)
			return
		}
		provider, err := constructor(providerConfig)
		if err != nil {
			registerErr = fmt.Errorf("failed to construct openfeature provider %q: %w", providerName, err)
			return
		}
		if err := openfeature.SetProviderWithContextAndWait(ctx, provider); err != nil {
			registerErr = fmt.Errorf("failed to set openfeature provider %q: %w", providerName, err)
			return
		}
		registeredProviderName = providerName
	})
	if registerErr != nil {
		return registerErr
	}
	if registeredProviderName != providerName {
		return fmt.Errorf("provider %q already registered in this process; cannot also use %q", registeredProviderName, providerName)
	}
	attachedMu.Lock()
	attachedCount++
	attachedMu.Unlock()
	return nil
}

// DeregisterProvider marks one previously-RegisterProvider-attached service
// as stopped, calling openfeature.Shutdown() once every attached service in
// this process has done so. Meant to be called from the matching fx.Hook's
// OnStop - fx only calls a hook's OnStop if its own OnStart succeeded, so
// this can't be called without a preceding successful RegisterProvider.
func DeregisterProvider() {
	attachedMu.Lock()
	attachedCount--
	last := attachedCount == 0
	attachedMu.Unlock()
	if last {
		openfeature.Shutdown()
	}
}

// toEvalContext maps Cadence's Filter/value pairs onto an OpenFeature
// EvaluationContext as plain attributes. No targeting key is set: Cadence's
// filters (domain, task list, shard, etc.) are matched as context attributes
// by the provider's targeting rules, not via per-user/per-entity bucketing.
func toEvalContext(filters map[dynamicproperties.Filter]interface{}) openfeature.EvaluationContext {
	attrs := make(map[string]interface{}, len(filters))
	for f, v := range filters {
		attrs[f.String()] = v
	}
	return openfeature.NewTargetlessEvaluationContext(attrs)
}

// translateErr maps an OpenFeature evaluation error onto dynamicconfig's own
// error vocabulary, using the resolution details' exported ErrorCode rather
// than string-matching err.Error() (Client.*Value doesn't preserve a typed
// openfeature.ResolutionError - see openfeature.ProviderResolutionDetail.Error
// - so ErrorCode is only available via the *ValueDetails variants).
//
// This distinction matters because Cadence declares hundreds of dynamic
// config keys and only a handful are ever expected to be set in an external
// flag platform - an unset key is the common case, not a real failure.
// dynamicconfig.Collection already special-cases dynamicconfig.NotFoundError
// (a *types.EntityNotExistsError) down to a DEBUG log instead of WARN; without
// this translation, every unset key would log at WARN on every access.
func translateErr(err error, errorCode openfeature.ErrorCode) error {
	if err == nil {
		return nil
	}
	if errorCode == openfeature.FlagNotFoundCode {
		return dynamicconfig.NotFoundError
	}
	return err
}

func (c *openFeatureClient) GetValue(name dynamicproperties.Key) (interface{}, error) {
	return c.GetValueWithFilters(name, nil)
}

// GetValueWithFilters has no single OpenFeature type to evaluate against, since
// Cadence's untyped path can back int/bool/string/duration/map/list keys.
// It uses ObjectValue; callers should prefer the typed getters below, which
// is how dynamicconfig.Collection reaches this client in practice.
func (c *openFeatureClient) GetValueWithFilters(name dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}) (interface{}, error) {
	details, err := c.client.ObjectValueDetails(context.Background(), name.String(), name.DefaultValue(), toEvalContext(filters))
	if err != nil {
		return name.DefaultValue(), translateErr(err, details.ErrorCode)
	}
	return details.Value, nil
}

func (c *openFeatureClient) GetIntValue(name dynamicproperties.IntKey, filters map[dynamicproperties.Filter]interface{}) (int, error) {
	defaultValue := name.DefaultInt()
	details, err := c.client.IntValueDetails(context.Background(), name.String(), int64(defaultValue), toEvalContext(filters))
	if err != nil {
		return defaultValue, translateErr(err, details.ErrorCode)
	}
	return int(details.Value), nil
}

func (c *openFeatureClient) GetFloatValue(name dynamicproperties.FloatKey, filters map[dynamicproperties.Filter]interface{}) (float64, error) {
	defaultValue := name.DefaultFloat()
	details, err := c.client.FloatValueDetails(context.Background(), name.String(), defaultValue, toEvalContext(filters))
	if err != nil {
		return defaultValue, translateErr(err, details.ErrorCode)
	}
	return details.Value, nil
}

func (c *openFeatureClient) GetBoolValue(name dynamicproperties.BoolKey, filters map[dynamicproperties.Filter]interface{}) (bool, error) {
	defaultValue := name.DefaultBool()
	details, err := c.client.BooleanValueDetails(context.Background(), name.String(), defaultValue, toEvalContext(filters))
	if err != nil {
		return defaultValue, translateErr(err, details.ErrorCode)
	}
	return details.Value, nil
}

func (c *openFeatureClient) GetStringValue(name dynamicproperties.StringKey, filters map[dynamicproperties.Filter]interface{}) (string, error) {
	defaultValue := name.DefaultString()
	details, err := c.client.StringValueDetails(context.Background(), name.String(), defaultValue, toEvalContext(filters))
	if err != nil {
		return defaultValue, translateErr(err, details.ErrorCode)
	}
	return details.Value, nil
}

func (c *openFeatureClient) GetMapValue(name dynamicproperties.MapKey, filters map[dynamicproperties.Filter]interface{}) (map[string]interface{}, error) {
	defaultValue := name.DefaultMap()
	details, err := c.client.ObjectValueDetails(context.Background(), name.String(), defaultValue, toEvalContext(filters))
	if err != nil {
		return defaultValue, translateErr(err, details.ErrorCode)
	}
	mapVal, ok := details.Value.(map[string]interface{})
	if !ok {
		return defaultValue, fmt.Errorf("value type is not map but is: %T", details.Value)
	}
	return mapVal, nil
}

// GetDurationValue stores durations as ParseDuration-compatible strings,
// since OpenFeature has no native duration type - the same convention the
// file-based client already uses on disk.
func (c *openFeatureClient) GetDurationValue(name dynamicproperties.DurationKey, filters map[dynamicproperties.Filter]interface{}) (time.Duration, error) {
	defaultValue := name.DefaultDuration()
	details, err := c.client.StringValueDetails(context.Background(), name.String(), defaultValue.String(), toEvalContext(filters))
	if err != nil {
		return defaultValue, translateErr(err, details.ErrorCode)
	}
	d, err := time.ParseDuration(details.Value)
	if err != nil {
		return defaultValue, fmt.Errorf("failed to parse duration: %v", err)
	}
	return d, nil
}

func (c *openFeatureClient) GetListValue(name dynamicproperties.ListKey, filters map[dynamicproperties.Filter]interface{}) ([]interface{}, error) {
	defaultValue := name.DefaultList()
	details, err := c.client.ObjectValueDetails(context.Background(), name.String(), defaultValue, toEvalContext(filters))
	if err != nil {
		return defaultValue, translateErr(err, details.ErrorCode)
	}
	listVal, ok := details.Value.([]interface{})
	if !ok {
		return defaultValue, fmt.Errorf("value type is not list but is: %T", details.Value)
	}
	return listVal, nil
}

// UpdateValue, RestoreValue, ListValue: OpenFeature is a read/evaluation API
// with no admin write path. Flag mutation happens in the provider's own
// control plane (e.g. flagd's flag source file, a vendor console), not here.
func (c *openFeatureClient) UpdateValue(name dynamicproperties.Key, value interface{}) error {
	return errors.New("not supported for openfeature client: manage flags via the configured provider")
}

func (c *openFeatureClient) RestoreValue(name dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}) error {
	return errors.New("not supported for openfeature client")
}

func (c *openFeatureClient) ListValue(name dynamicproperties.Key) ([]*types.DynamicConfigEntry, error) {
	return nil, errors.New("not supported for openfeature client")
}
