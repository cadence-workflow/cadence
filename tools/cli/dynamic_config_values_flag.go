package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/uber/cadence/common/types"
)

// DynamicConfigValuesFlag is a custom flag type that accepts raw JSON objects without comma-splitting
// and parses them directly into DynamicConfigValue types.
// Use multiple --value flags to set multiple values:
//
//	--value '{"Value":1,"Filters":[]}' --value '{"Value":2,"Filters":[]}'
type DynamicConfigValuesFlag struct {
	values []*types.DynamicConfigValue
}

// Set is called by urfave/cli for each --value flag
func (f *DynamicConfigValuesFlag) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("empty value not allowed")
	}

	// Parse into intermediate cliValue format
	var parsedInputValue *cliValue
	if err := json.Unmarshal([]byte(trimmed), &parsedInputValue); err != nil {
		return fmt.Errorf("invalid JSON object: %w", err)
	}

	// Convert to DynamicConfigValue
	parsedValue, err := convertFromInputValue(parsedInputValue)
	if err != nil {
		return fmt.Errorf("unable to convert to DynamicConfigValue: %w", err)
	}

	f.values = append(f.values, parsedValue)
	return nil
}

// String returns a JSON representation of the values
func (f *DynamicConfigValuesFlag) String() string {
	if len(f.values) == 0 {
		return "[]"
	}

	// Convert back to cliValue format for consistent representation
	var cliValues []*cliValue
	for _, dcValue := range f.values {
		cliVal, err := convertToInputValue(dcValue)
		if err != nil {
			// Fallback to count if conversion fails
			return fmt.Sprintf("%d value(s)", len(f.values))
		}
		cliValues = append(cliValues, cliVal)
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(cliValues)
	if err != nil {
		return fmt.Sprintf("%d value(s)", len(f.values))
	}

	return string(jsonBytes)
}

// Get returns the parsed DynamicConfigValue objects
func (f *DynamicConfigValuesFlag) Get() []*types.DynamicConfigValue {
	return f.values
}

// NewDynamicConfigValuesFlag creates a new DynamicConfigValuesFlag
func NewDynamicConfigValuesFlag() *DynamicConfigValuesFlag {
	return &DynamicConfigValuesFlag{
		values: []*types.DynamicConfigValue{},
	}
}
