// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package dynamicconfig

import (
	"errors"
	"sync"
	"time"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

type inMemoryClient struct {
	sync.RWMutex

	globalValues map[dynamicproperties.Key]interface{}
}

// NewInMemoryClient creates a new in memory dynamic config client for testing purpose
func NewInMemoryClient() Client {
	return &inMemoryClient{
		globalValues: make(map[dynamicproperties.Key]interface{}),
	}
}

func (mc *inMemoryClient) SetValue(key dynamicproperties.Key, value interface{}) {
	mc.Lock()
	defer mc.Unlock()

	mc.globalValues[key] = value
}

func (mc *inMemoryClient) GetValue(key dynamicproperties.Key) (interface{}, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[key]; ok {
		return val, nil
	}
	return key.DefaultValue(), NotFoundError
}

func (mc *inMemoryClient) GetValueWithFilters(name dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}) (interface{}, error) {
	mc.RLock()
	defer mc.RUnlock()

	return mc.GetValue(name)
}

func (mc *inMemoryClient) GetIntValue(name dynamicproperties.IntKey, filters map[dynamicproperties.Filter]interface{}) (int, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(int), nil
	}
	return name.DefaultInt(), NotFoundError
}

func (mc *inMemoryClient) GetFloatValue(name dynamicproperties.FloatKey, filters map[dynamicproperties.Filter]interface{}) (float64, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(float64), nil
	}
	return name.DefaultFloat(), NotFoundError
}

func (mc *inMemoryClient) GetBoolValue(name dynamicproperties.BoolKey, filters map[dynamicproperties.Filter]interface{}) (bool, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(bool), nil
	}
	return name.DefaultBool(), NotFoundError
}

func (mc *inMemoryClient) GetStringValue(name dynamicproperties.StringKey, filters map[dynamicproperties.Filter]interface{}) (string, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(string), nil
	}
	return name.DefaultString(), NotFoundError
}

func (mc *inMemoryClient) GetMapValue(name dynamicproperties.MapKey, filters map[dynamicproperties.Filter]interface{}) (map[string]interface{}, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(map[string]interface{}), nil
	}
	return name.DefaultMap(), NotFoundError
}

func (mc *inMemoryClient) GetDurationValue(name dynamicproperties.DurationKey, filters map[dynamicproperties.Filter]interface{}) (time.Duration, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(time.Duration), nil
	}
	return name.DefaultDuration(), NotFoundError
}

func (mc *inMemoryClient) GetListValue(name dynamicproperties.ListKey, filters map[dynamicproperties.Filter]interface{}) ([]interface{}, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.([]interface{}), nil
	}
	return name.DefaultList(), NotFoundError
}

func (mc *inMemoryClient) UpdateValue(key dynamicproperties.Key, value interface{}) error {
	if err := dynamicproperties.ValidateKeyValuePair(key, value); err != nil {
		return err
	}
	mc.SetValue(key, value)
	return nil
}

func (mc *inMemoryClient) RestoreValue(name dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}) error {
	return errors.New("not supported for in-memory client")
}

func (mc *inMemoryClient) ListValue(name dynamicproperties.Key) ([]*types.DynamicConfigEntry, error) {
	return nil, errors.New("not supported for in-memory client")
}
