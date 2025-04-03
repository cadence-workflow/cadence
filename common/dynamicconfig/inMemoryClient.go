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

	"github.com/uber/cadence/common/types"
)

type InMemory struct {
	sync.RWMutex

	globalValues map[Key]interface{}
}

// NewInMemoryClient creates a new in memory dynamic config client for testing purpose
func NewInMemoryClient() Client {
	return &InMemory{
		globalValues: make(map[Key]interface{}),
	}
}

func (mc *InMemory) SetValue(key Key, value interface{}) {
	mc.Lock()
	defer mc.Unlock()

	mc.globalValues[key] = value
}

func (mc *InMemory) GetValue(key Key) (interface{}, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[key]; ok {
		return val, nil
	}
	return key.DefaultValue(), NotFoundError
}

func (mc *InMemory) GetValueWithFilters(name Key, filters map[Filter]interface{}) (interface{}, error) {
	mc.RLock()
	defer mc.RUnlock()

	return mc.GetValue(name)
}

func (mc *InMemory) GetIntValue(name IntKey, filters map[Filter]interface{}) (int, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(int), nil
	}
	return name.DefaultInt(), NotFoundError
}

func (mc *InMemory) GetFloatValue(name FloatKey, filters map[Filter]interface{}) (float64, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(float64), nil
	}
	return name.DefaultFloat(), NotFoundError
}

func (mc *InMemory) GetBoolValue(name BoolKey, filters map[Filter]interface{}) (bool, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(bool), nil
	}
	return name.DefaultBool(), NotFoundError
}

func (mc *InMemory) GetStringValue(name StringKey, filters map[Filter]interface{}) (string, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(string), nil
	}
	return name.DefaultString(), NotFoundError
}

func (mc *InMemory) GetMapValue(name MapKey, filters map[Filter]interface{}) (map[string]interface{}, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(map[string]interface{}), nil
	}
	return name.DefaultMap(), NotFoundError
}

func (mc *InMemory) GetDurationValue(name DurationKey, filters map[Filter]interface{}) (time.Duration, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.(time.Duration), nil
	}
	return name.DefaultDuration(), NotFoundError
}

func (mc *InMemory) GetListValue(name ListKey, filters map[Filter]interface{}) ([]interface{}, error) {
	mc.RLock()
	defer mc.RUnlock()

	if val, ok := mc.globalValues[name]; ok {
		return val.([]interface{}), nil
	}
	return name.DefaultList(), NotFoundError
}

func (mc *InMemory) UpdateValue(key Key, value interface{}) error {
	if err := ValidateKeyValuePair(key, value); err != nil {
		return err
	}
	mc.SetValue(key, value)
	return nil
}

func (mc *InMemory) RestoreValue(name Key, filters map[Filter]interface{}) error {
	return errors.New("not supported for in-memory client")
}

func (mc *InMemory) ListValue(name Key) ([]*types.DynamicConfigEntry, error) {
	return nil, errors.New("not supported for in-memory client")
}
