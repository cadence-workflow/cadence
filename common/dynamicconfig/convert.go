// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package dynamicconfig

import (
	"fmt"
	"strconv"
	"strings"
)

// ConvertIntMapToDynamicConfigMapProperty converts a map whose key value type are both int to
// a map value that is compatible with dynamic config's map property
func ConvertIntMapToDynamicConfigMapProperty(
	intMap map[int]int,
) map[string]interface{} {
	dcValue := make(map[string]interface{})
	for key, value := range intMap {
		dcValue[strconv.Itoa(key)] = value
	}
	return dcValue
}

// ConvertDynamicConfigMapPropertyToIntMap convert a map property from dynamic config to a map
// whose type for both key and value are int
func ConvertDynamicConfigMapPropertyToIntMap(dcValue map[string]interface{}) (map[int]int, error) {
	intMap := make(map[int]int)
	for key, value := range dcValue {
		intKey, err := strconv.Atoi(strings.TrimSpace(key))
		if err != nil {
			return nil, fmt.Errorf("failed to convert key %v, error: %v", key, err)
		}

		var intValue int
		switch value := value.(type) {
		case float64:
			intValue = int(value)
		case int:
			intValue = value
		case int32:
			intValue = int(value)
		case int64:
			intValue = int(value)
		default:
			return nil, fmt.Errorf("unknown value %v with type %T", value, value)
		}
		intMap[intKey] = intValue
	}
	return intMap, nil
}
