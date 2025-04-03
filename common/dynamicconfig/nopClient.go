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

package dynamicconfig

import (
	"errors"
	"time"

	"github.com/uber/cadence/common/types"
)

// nopClient is a dummy implements of dynamicconfig Client interface, all operations will always return default values.
type nopClient struct{}

func (mc *nopClient) GetValue(name Key) (interface{}, error) {
	return nil, NotFoundError
}

func (mc *nopClient) GetValueWithFilters(name Key, filters map[Filter]interface{}) (interface{}, error) {
	return nil, NotFoundError
}

func (mc *nopClient) GetIntValue(name IntKey, filters map[Filter]interface{}) (int, error) {
	return name.DefaultInt(), NotFoundError
}

func (mc *nopClient) GetFloatValue(name FloatKey, filters map[Filter]interface{}) (float64, error) {
	return name.DefaultFloat(), NotFoundError
}

func (mc *nopClient) GetBoolValue(name BoolKey, filters map[Filter]interface{}) (bool, error) {
	if filters[DomainName] == "TestRawHistoryDomain" {
		return true, NotFoundError
	}
	return name.DefaultBool(), NotFoundError
}

func (mc *nopClient) GetStringValue(name StringKey, filters map[Filter]interface{}) (string, error) {
	return name.DefaultString(), NotFoundError
}

func (mc *nopClient) GetMapValue(name MapKey, filters map[Filter]interface{}) (map[string]interface{}, error) {
	return name.DefaultMap(), NotFoundError
}

func (mc *nopClient) GetDurationValue(name DurationKey, filters map[Filter]interface{}) (time.Duration, error) {
	return name.DefaultDuration(), NotFoundError
}

func (mc *nopClient) GetListValue(name ListKey, filters map[Filter]interface{}) ([]interface{}, error) {
	return name.DefaultList(), NotFoundError
}

func (mc *nopClient) UpdateValue(name Key, value interface{}) error {
	return errors.New("not supported for nop client")
}

func (mc *nopClient) RestoreValue(name Key, filters map[Filter]interface{}) error {
	return errors.New("not supported for nop client")
}

func (mc *nopClient) ListValue(name Key) ([]*types.DynamicConfigEntry, error) {
	return nil, errors.New("not supported for nop client")
}

// NewNopClient creates a nop client
func NewNopClient() Client {
	return &nopClient{}
}
