package elasticsearch

import (
	"encoding/json"

	"golang.org/x/exp/maps"
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

func IsMissingMappings(current, expected map[string]any) (bool, error) {
	c, err := convertMappings(current)
	if err != nil {
		return false, err
	}
	e, err := convertMappings(expected)
	if err != nil {
		return false, err
	}
	diff := mappingsDifferences(c.Properties, e.Properties)
	return len(diff) > 0, nil
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

func mappingsDifferences(current map[string]property, expected map[string]property) map[string]property {
	missing := make(map[string]property)
	keys := append(maps.Keys(current), maps.Keys(expected)...)
	visited := make(map[string]bool)

	// go through and find which are missing, which are
	// additional and superfluous
	for _, key := range keys {

		// the list of keys will contain duplicates because it's summing up the maps keys directly above, so don't
		// bother visiting more than once.
		if visited[key] {
			continue
		}

		existingProperty, presentInCurrent := current[key]
		expectedProperty, presentInExpected := expected[key]

		if presentInCurrent && presentInExpected {
			missingSubtree := mappingsDifferences(existingProperty.Properties, expectedProperty.Properties)
			if len(missingSubtree) > 0 {
				missing[key] = property{
					Properties: missingSubtree,
				}
			}
		} else if presentInCurrent && !presentInExpected {
			// ignore, superfluous mapping
		} else if presentInExpected && !presentInCurrent {
			// missing key identified
			missing[key] = expectedProperty
		}

		visited[key] = true
	}

	return missing
}
