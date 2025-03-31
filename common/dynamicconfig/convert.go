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
