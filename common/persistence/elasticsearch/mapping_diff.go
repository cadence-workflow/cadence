package elasticsearch

import (
	"encoding/json"
	"sort"
)

type (
	property struct {
		Type       string              `json:"type,omitempty"`
		Properties map[string]property `json:"properties,omitempty"`
	}

	mappings struct {
		Dynamic    string              `json:"dynamic,omitempty"`
		Properties map[string]property `json:"properties,omitempty"`
	}
)

// GetMissingMappings returns the sorted dot notation paths of every property that is present in
// expected but missing from current, e.g. ["Attr.CustomBoolField", "CloseTime"].
// Properties that only exist in current are considered superfluous and are ignored.
func GetMissingMappings(current, expected map[string]any) ([]string, error) {
	c, err := convertMappings(current)
	if err != nil {
		return nil, err
	}
	e, err := convertMappings(expected)
	if err != nil {
		return nil, err
	}
	missing := missingMappingPaths("", c.Properties, e.Properties)
	sort.Strings(missing)
	return missing, nil
}

func convertMappings(m map[string]interface{}) (*mappings, error) {
	mappingsJSON, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var result mappings
	if err := json.Unmarshal(mappingsJSON, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// missingMappingPaths walks the expected properties and collects the dot notation path of every
// property that isn't present in current. Properties present in both are recursed into so that
// only the missing leaves of a shared subtree are reported.
func missingMappingPaths(prefix string, current, expected map[string]property) []string {
	var missing []string
	for key, expectedProperty := range expected {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		currentProperty, presentInCurrent := current[key]
		if !presentInCurrent {
			// missing key identified, no need to descend into it
			missing = append(missing, path)
			continue
		}

		missing = append(missing, missingMappingPaths(path, currentProperty.Properties, expectedProperty.Properties)...)
	}

	return missing
}
