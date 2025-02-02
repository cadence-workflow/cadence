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

package host

import (
	"sync"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/types"
)

var (
	// Override values for dynamic configs
	staticOverrides = map[dynamicconfig.Key]interface{}{
		dynamicconfig.FrontendUserRPS:                               3000,
		dynamicconfig.FrontendVisibilityListMaxQPS:                  200,
		dynamicconfig.FrontendESIndexMaxResultWindow:                defaultTestValueOfESIndexMaxResultWindow,
		dynamicconfig.MatchingNumTasklistWritePartitions:            3,
		dynamicconfig.MatchingNumTasklistReadPartitions:             3,
		dynamicconfig.TimerProcessorHistoryArchivalSizeLimit:        5 * 1024,
		dynamicconfig.ReplicationTaskProcessorErrorRetryMaxAttempts: 1,
		dynamicconfig.WriteVisibilityStoreName:                      common.AdvancedVisibilityModeOff,
		dynamicconfig.DecisionHeartbeatTimeout:                      5 * time.Second,
		dynamicconfig.ReplicationTaskFetcherAggregationInterval:     200 * time.Millisecond,
		dynamicconfig.ReplicationTaskFetcherErrorRetryWait:          50 * time.Millisecond,
		dynamicconfig.ReplicationTaskProcessorErrorRetryWait:        time.Millisecond,
		dynamicconfig.EnableConsistentQueryByDomain:                 true,
		dynamicconfig.MinRetentionDays:                              0,
		dynamicconfig.WorkflowDeletionJitterRange:                   1,
	}
)

type dynamicClient struct {
	sync.RWMutex

	overrides map[dynamicconfig.Key]interface{}
	client    dynamicconfig.Client
}

func (d *dynamicClient) GetValue(name dynamicconfig.Key) (interface{}, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		d.RUnlock()
		return val, nil
	}
	d.RUnlock()
	return d.client.GetValue(name)
}

func (d *dynamicClient) GetValueWithFilters(name dynamicconfig.Key, filters map[dynamicconfig.Filter]interface{}) (interface{}, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		d.RUnlock()
		return val, nil
	}
	d.RUnlock()
	return d.client.GetValueWithFilters(name, filters)
}

func (d *dynamicClient) GetIntValue(name dynamicconfig.IntKey, filters map[dynamicconfig.Filter]interface{}) (int, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		if intVal, ok := val.(int); ok {
			d.RUnlock()
			return intVal, nil
		}
	}
	d.RUnlock()
	return d.client.GetIntValue(name, filters)
}

func (d *dynamicClient) GetFloatValue(name dynamicconfig.FloatKey, filters map[dynamicconfig.Filter]interface{}) (float64, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		if floatVal, ok := val.(float64); ok {
			d.RUnlock()
			return floatVal, nil
		}
	}
	d.RUnlock()
	return d.client.GetFloatValue(name, filters)
}

func (d *dynamicClient) GetBoolValue(name dynamicconfig.BoolKey, filters map[dynamicconfig.Filter]interface{}) (bool, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		if boolVal, ok := val.(bool); ok {
			d.RUnlock()
			return boolVal, nil
		}
	}
	d.RUnlock()
	return d.client.GetBoolValue(name, filters)
}

func (d *dynamicClient) GetStringValue(name dynamicconfig.StringKey, filters map[dynamicconfig.Filter]interface{}) (string, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		if stringVal, ok := val.(string); ok {
			d.RUnlock()
			return stringVal, nil
		}
	}
	d.RUnlock()
	return d.client.GetStringValue(name, filters)
}

func (d *dynamicClient) GetMapValue(name dynamicconfig.MapKey, filters map[dynamicconfig.Filter]interface{}) (map[string]interface{}, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		if mapVal, ok := val.(map[string]interface{}); ok {
			d.RUnlock()
			return mapVal, nil
		}
	}
	d.RUnlock()
	return d.client.GetMapValue(name, filters)
}

func (d *dynamicClient) GetDurationValue(name dynamicconfig.DurationKey, filters map[dynamicconfig.Filter]interface{}) (time.Duration, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		if durationVal, ok := val.(time.Duration); ok {
			d.RUnlock()
			return durationVal, nil
		}
	}
	d.RUnlock()
	return d.client.GetDurationValue(name, filters)
}
func (d *dynamicClient) GetListValue(name dynamicconfig.ListKey, filters map[dynamicconfig.Filter]interface{}) ([]interface{}, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		if listVal, ok := val.([]interface{}); ok {
			d.RUnlock()
			return listVal, nil
		}
	}
	d.RUnlock()
	return d.client.GetListValue(name, filters)
}

func (d *dynamicClient) UpdateValue(name dynamicconfig.Key, value interface{}) error {
	if name == dynamicconfig.WriteVisibilityStoreName { // override for es integration tests
		d.Lock()
		defer d.Unlock()
		d.overrides[dynamicconfig.WriteVisibilityStoreName] = value.(string)
		return nil
	} else if name == dynamicconfig.ReadVisibilityStoreName { // override for pinot integration tests
		d.Lock()
		defer d.Unlock()
		d.overrides[dynamicconfig.ReadVisibilityStoreName] = value.(string)
		return nil
	}
	return d.client.UpdateValue(name, value)
}

func (d *dynamicClient) OverrideValue(name dynamicconfig.Key, value interface{}) {
	d.Lock()
	defer d.Unlock()
	d.overrides[name] = value
}

func (d *dynamicClient) ListValue(name dynamicconfig.Key) ([]*types.DynamicConfigEntry, error) {
	return d.client.ListValue(name)
}

func (d *dynamicClient) RestoreValue(name dynamicconfig.Key, filters map[dynamicconfig.Filter]interface{}) error {
	return d.client.RestoreValue(name, filters)
}

var _ dynamicconfig.Client = (*dynamicClient)(nil)

// newIntegrationConfigClient - returns a dynamic config client for integration testing
func newIntegrationConfigClient(client dynamicconfig.Client, overrides map[dynamicconfig.Key]interface{}) *dynamicClient {
	integrationClient := &dynamicClient{
		overrides: make(map[dynamicconfig.Key]interface{}),
		client:    client,
	}

	for key, value := range staticOverrides {
		integrationClient.OverrideValue(key, value)
	}

	for key, value := range overrides {
		integrationClient.OverrideValue(key, value)
	}
	return integrationClient
}
