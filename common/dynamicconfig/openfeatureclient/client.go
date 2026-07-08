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
// OpenFeature. It lives in its own subpackage (rather than in
// common/dynamicconfig itself) so that its Config type can be referenced
// directly from common/config: common/config already imports
// common/dynamicconfig, so common/dynamicconfig cannot import common/config
// back without a cycle, which would block defining a lazily-decoded YAML
// config field there. This package has no such constraint.
package openfeatureclient

import (
	"context"
	"errors"
	"fmt"
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

// NewOpenFeatureClient creates a dynamicconfig.Client backed by OpenFeature.
// providerName selects a provider plugin previously registered via
// openfeatureprovider.Register (see that package's doc for the plugin
// convention); providerConfig is that plugin's own config, decoded lazily so
// this package never depends on any provider-specific config type.
func NewOpenFeatureClient(providerName string, providerConfig openfeatureprovider.Decoder, logger log.Logger) (dynamicconfig.Client, error) {
	constructor, ok := openfeatureprovider.Get(providerName)
	if !ok {
		return nil, fmt.Errorf("unknown openfeature provider %q: is its package blank-imported for plugin registration?", providerName)
	}
	provider, err := constructor(providerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to construct openfeature provider %q: %w", providerName, err)
	}
	if err := openfeature.SetProviderAndWait(provider); err != nil {
		return nil, fmt.Errorf("failed to set openfeature provider %q: %w", providerName, err)
	}
	return &openFeatureClient{
		client: openfeature.NewDefaultClient(),
		logger: logger,
	}, nil
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

func (c *openFeatureClient) GetValue(name dynamicproperties.Key) (interface{}, error) {
	return c.GetValueWithFilters(name, nil)
}

// GetValueWithFilters has no single OpenFeature type to evaluate against, since
// Cadence's untyped path can back int/bool/string/duration/map/list keys.
// It uses ObjectValue; callers should prefer the typed getters below, which
// is how dynamicconfig.Collection reaches this client in practice.
func (c *openFeatureClient) GetValueWithFilters(name dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}) (interface{}, error) {
	val, err := c.client.ObjectValue(context.Background(), name.String(), name.DefaultValue(), toEvalContext(filters))
	if err != nil {
		return name.DefaultValue(), err
	}
	return val, nil
}

func (c *openFeatureClient) GetIntValue(name dynamicproperties.IntKey, filters map[dynamicproperties.Filter]interface{}) (int, error) {
	c.logger.Info("GetIntValue")
	defaultValue := name.DefaultInt()
	val, err := c.client.IntValue(context.Background(), name.String(), int64(defaultValue), toEvalContext(filters))
	if err != nil {
		return defaultValue, err
	}
	return int(val), nil
}

func (c *openFeatureClient) GetFloatValue(name dynamicproperties.FloatKey, filters map[dynamicproperties.Filter]interface{}) (float64, error) {
	defaultValue := name.DefaultFloat()
	return c.client.FloatValue(context.Background(), name.String(), defaultValue, toEvalContext(filters))
}

func (c *openFeatureClient) GetBoolValue(name dynamicproperties.BoolKey, filters map[dynamicproperties.Filter]interface{}) (bool, error) {
	defaultValue := name.DefaultBool()
	return c.client.BooleanValue(context.Background(), name.String(), defaultValue, toEvalContext(filters))
}

func (c *openFeatureClient) GetStringValue(name dynamicproperties.StringKey, filters map[dynamicproperties.Filter]interface{}) (string, error) {
	defaultValue := name.DefaultString()
	return c.client.StringValue(context.Background(), name.String(), defaultValue, toEvalContext(filters))
}

func (c *openFeatureClient) GetMapValue(name dynamicproperties.MapKey, filters map[dynamicproperties.Filter]interface{}) (map[string]interface{}, error) {
	defaultValue := name.DefaultMap()
	val, err := c.client.ObjectValue(context.Background(), name.String(), defaultValue, toEvalContext(filters))
	if err != nil {
		return defaultValue, err
	}
	mapVal, ok := val.(map[string]interface{})
	if !ok {
		return defaultValue, fmt.Errorf("value type is not map but is: %T", val)
	}
	return mapVal, nil
}

// GetDurationValue stores durations as ParseDuration-compatible strings,
// since OpenFeature has no native duration type - the same convention the
// file-based client already uses on disk.
func (c *openFeatureClient) GetDurationValue(name dynamicproperties.DurationKey, filters map[dynamicproperties.Filter]interface{}) (time.Duration, error) {
	defaultValue := name.DefaultDuration()
	s, err := c.client.StringValue(context.Background(), name.String(), defaultValue.String(), toEvalContext(filters))
	if err != nil {
		return defaultValue, err
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultValue, fmt.Errorf("failed to parse duration: %v", err)
	}
	return d, nil
}

func (c *openFeatureClient) GetListValue(name dynamicproperties.ListKey, filters map[dynamicproperties.Filter]interface{}) ([]interface{}, error) {
	defaultValue := name.DefaultList()
	val, err := c.client.ObjectValue(context.Background(), name.String(), defaultValue, toEvalContext(filters))
	if err != nil {
		return defaultValue, err
	}
	listVal, ok := val.([]interface{})
	if !ok {
		return defaultValue, fmt.Errorf("value type is not list but is: %T", val)
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
