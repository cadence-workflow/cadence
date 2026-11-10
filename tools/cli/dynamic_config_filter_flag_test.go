package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDynamicConfigFilterFlag_Set(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "valid filter with single field",
			input:       `{"domainName":"test-domain"}`,
			expectError: false,
		},
		{
			name:        "valid filter with multiple fields",
			input:       `{"domainName":"test-domain","shardID":1}`,
			expectError: false,
		},
		{
			name:        "empty object is valid",
			input:       `{}`,
			expectError: false,
		},
		{
			name:        "empty string is valid",
			input:       ``,
			expectError: false,
		},
		{
			name:        "whitespace only is valid",
			input:       `  `,
			expectError: false,
		},
		{
			name:        "array is not allowed",
			input:       `[{"domainName":"test"}]`,
			expectError: true,
		},
		{
			name:        "invalid JSON",
			input:       `{invalid}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewDynamicConfigFilterFlag()
			err := flag.Set(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDynamicConfigFilterFlag_MultipleSetRejected(t *testing.T) {
	flag := NewDynamicConfigFilterFlag()

	// First set should succeed
	err := flag.Set(`{"domainName":"test"}`)
	assert.NoError(t, err)

	// Second set should fail
	err = flag.Set(`{"shardID":1}`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "filter can only be specified once")
}

func TestDynamicConfigFilterFlag_String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single field filter",
			input:    `{"domainName":"test-domain"}`,
			expected: `{"domainName":"test-domain"}`,
		},
		{
			name:     "multiple fields filter",
			input:    `{"domainName":"test-domain","shardID":1}`,
			expected: `{"domainName":"test-domain","shardID":1}`,
		},
		{
			name:     "empty filter",
			input:    `{}`,
			expected: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewDynamicConfigFilterFlag()
			err := flag.Set(tt.input)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expected, flag.String())
		})
	}
}

func TestDynamicConfigFilterFlag_Get(t *testing.T) {
	flag := NewDynamicConfigFilterFlag()
	err := flag.Set(`{"domainName":"test-domain","shardID":1}`)
	assert.NoError(t, err)

	filters := flag.Get()
	assert.NotNil(t, filters)
	assert.Len(t, filters, 2)

	// Check that both filters are present (order may vary)
	filterMap := make(map[string]interface{})
	for _, f := range filters {
		var val interface{}
		err := json.Unmarshal(f.Value.Data, &val)
		assert.NoError(t, err)
		filterMap[f.Name] = val
	}

	assert.Equal(t, "test-domain", filterMap["domainName"])
	assert.Equal(t, float64(1), filterMap["shardID"]) // JSON numbers unmarshal as float64
}
