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

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

var (
	// Override values for dynamic configs
	staticOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.FrontendUserRPS:                                   3000,
		dynamicproperties.FrontendVisibilityListMaxQPS:                      200,
		dynamicproperties.FrontendESIndexMaxResultWindow:                    defaultTestValueOfESIndexMaxResultWindow,
		dynamicproperties.MatchingNumTasklistWritePartitions:                3,
		dynamicproperties.MatchingNumTasklistReadPartitions:                 3,
		dynamicproperties.TimerProcessorHistoryArchivalSizeLimit:            5 * 1024,
		dynamicproperties.ReplicationTaskProcessorErrorRetryMaxAttempts:     1,
		dynamicproperties.WriteVisibilityStoreName:                          constants.AdvancedVisibilityModeOff,
		dynamicproperties.DecisionHeartbeatTimeout:                          5 * time.Second,
		dynamicproperties.ReplicationTaskFetcherAggregationInterval:         200 * time.Millisecond,
		dynamicproperties.ReplicationTaskFetcherErrorRetryWait:              50 * time.Millisecond,
		dynamicproperties.ReplicationTaskProcessorErrorRetryWait:            time.Millisecond,
		dynamicproperties.EnableConsistentQueryByDomain:                     true,
		dynamicproperties.MinRetentionDays:                                  0,
		dynamicproperties.WorkflowDeletionJitterRange:                       1,
		dynamicproperties.EnableActiveClusterSelectionPolicyInStartWorkflow: true,
	}
)

type dynamicClient struct {
	sync.RWMutex

	overrides map[dynamicproperties.Key]interface{}
	client    dynamicconfig.Client
}

func (d *dynamicClient) GetValue(name dynamicproperties.Key) (interface{}, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		d.RUnlock()
		return val, nil
	}
	d.RUnlock()
	return d.client.GetValue(name)
}

func (d *dynamicClient) GetValueWithFilters(name dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}) (interface{}, error) {
	d.RLock()
	if val, ok := d.overrides[name]; ok {
		d.RUnlock()
		return val, nil
	}
	d.RUnlock()
	return d.client.GetValueWithFilters(name, filters)
}

func (d *dynamicClient) GetIntValue(name dynamicproperties.IntKey, filters map[dynamicproperties.Filter]interface{}) (int, error) {
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

func (d *dynamicClient) GetFloatValue(name dynamicproperties.FloatKey, filters map[dynamicproperties.Filter]interface{}) (float64, error) {
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

func (d *dynamicClient) GetBoolValue(name dynamicproperties.BoolKey, filters map[dynamicproperties.Filter]interface{}) (bool, error) {
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

func (d *dynamicClient) GetStringValue(name dynamicproperties.StringKey, filters map[dynamicproperties.Filter]interface{}) (string, error) {
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

func (d *dynamicClient) GetMapValue(name dynamicproperties.MapKey, filters map[dynamicproperties.Filter]interface{}) (map[string]interface{}, error) {
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

func (d *dynamicClient) GetDurationValue(name dynamicproperties.DurationKey, filters map[dynamicproperties.Filter]interface{}) (time.Duration, error) {
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
func (d *dynamicClient) GetListValue(name dynamicproperties.ListKey, filters map[dynamicproperties.Filter]interface{}) ([]interface{}, error) {
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

func (d *dynamicClient) UpdateValue(name dynamicproperties.Key, value interface{}) error {
	if name == dynamicproperties.WriteVisibilityStoreName { // override for es integration tests
		d.Lock()
		defer d.Unlock()
		d.overrides[dynamicproperties.WriteVisibilityStoreName] = value.(string)
		return nil
	} else if name == dynamicproperties.ReadVisibilityStoreName { // override for pinot integration tests
		d.Lock()
		defer d.Unlock()
		d.overrides[dynamicproperties.ReadVisibilityStoreName] = value.(string)
		return nil
	}
	return d.client.UpdateValue(name, value)
}

func (d *dynamicClient) OverrideValue(name dynamicproperties.Key, value interface{}) {
	d.Lock()
	defer d.Unlock()
	d.overrides[name] = value
}

func (d *dynamicClient) ListValue(name dynamicproperties.Key) ([]*types.DynamicConfigEntry, error) {
	return d.client.ListValue(name)
}

func (d *dynamicClient) RestoreValue(name dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}) error {
	return d.client.RestoreValue(name, filters)
}

var _ dynamicconfig.Client = (*dynamicClient)(nil)

// newIntegrationConfigClient - returns a dynamic config client for integration testing
func newIntegrationConfigClient(client dynamicconfig.Client, overrides map[dynamicproperties.Key]interface{}) *dynamicClient {
	integrationClient := &dynamicClient{
		overrides: make(map[dynamicproperties.Key]interface{}),
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
