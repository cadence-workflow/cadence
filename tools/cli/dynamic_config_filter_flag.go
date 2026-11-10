package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/uber/cadence/common/types"
)

// DynamicConfigFilterFlag is a custom flag type that accepts a JSON object for filters
// without comma-splitting and parses it directly into DynamicConfigFilter types.
//
// Example: --filter '{"domainName":"test-domain","shardID":1}'
type DynamicConfigFilterFlag struct {
	filters []*types.DynamicConfigFilter
	isSet   bool
}

// Set is called by urfave/cli when the flag is provided
func (f *DynamicConfigFilterFlag) Set(value string) error {
	// Reject if filter was already set
	if f.isSet {
		return fmt.Errorf("filter can only be specified once")
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "{}" {
		// Empty filter is valid (means no filtering)
		f.filters = nil
		f.isSet = true
		return nil
	}

	// Parse the filter map
	parsedFilters, err := parseInputFilter(trimmed)
	if err != nil {
		return fmt.Errorf("invalid filter JSON: %w", err)
	}

	f.filters = parsedFilters
	f.isSet = true
	return nil
}

// String returns a JSON representation of the filter
func (f *DynamicConfigFilterFlag) String() string {
	if len(f.filters) == 0 {
		return "{}"
	}

	// Convert to map for display
	filterMap := make(map[string]interface{})
	for _, filter := range f.filters {
		cliFilter, err := convertToInputFilter(filter)
		if err != nil {
			// Fallback to count if conversion fails
			return fmt.Sprintf("%d filter(s)", len(f.filters))
		}
		filterMap[cliFilter.Name] = cliFilter.Value
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(filterMap)
	if err != nil {
		return fmt.Sprintf("%d filter(s)", len(f.filters))
	}

	return string(jsonBytes)
}

// Get returns the parsed DynamicConfigFilter objects
func (f *DynamicConfigFilterFlag) Get() []*types.DynamicConfigFilter {
	return f.filters
}

// NewDynamicConfigFilterFlag creates a new DynamicConfigFilterFlag
func NewDynamicConfigFilterFlag() *DynamicConfigFilterFlag {
	return &DynamicConfigFilterFlag{
		filters: nil,
	}
}
