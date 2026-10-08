package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDynamicConfigValuesFlag(t *testing.T) {
	tests := []struct {
		name        string
		values      []string
		expectedLen int
		wantErr     bool
	}{
		{
			name:        "single valid JSON object",
			values:      []string{`{"Value":1000,"Filters":[]}`},
			expectedLen: 1,
			wantErr:     false,
		},
		{
			name: "multiple JSON objects (multiple --value flags)",
			values: []string{
				`{"Value":1000,"Filters":[]}`,
				`{"Value":100,"Filters":[{"Name":"domainName","Value":"test"}]}`,
			},
			expectedLen: 2,
			wantErr:     false,
		},
		{
			name: "complex nested JSON with commas (no splitting!)",
			values: []string{
				`{"Value":100,"Filters":[{"Name":"domainName","Value":"cadence-canary-xdc-gcp-production"}]}`,
			},
			expectedLen: 1,
			wantErr:     false,
		},
		{
			name:    "invalid JSON",
			values:  []string{`{invalid json}`},
			wantErr: true,
		},
		{
			name:    "empty string",
			values:  []string{``},
			wantErr: true,
		},
		{
			name:    "array not allowed",
			values:  []string{`[{"Value":1000,"Filters":[]}]`},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewDynamicConfigValuesFlag()

			var err error
			for _, val := range tt.values {
				err = flag.Set(val)
				if tt.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
			}

			result := flag.Get()
			assert.Len(t, result, tt.expectedLen)

			// Validate that results are proper DynamicConfigValue types
			for _, val := range result {
				assert.NotNil(t, val)
				assert.NotNil(t, val.Value)
			}
		})
	}
}

func TestDynamicConfigValuesFlagString(t *testing.T) {
	flag := NewDynamicConfigValuesFlag()
	assert.Equal(t, "[]", flag.String())

	// Add first value
	err := flag.Set(`{"Value":1000,"Filters":[]}`)
	require.NoError(t, err)

	str := flag.String()
	assert.JSONEq(t, `[{"Value":1000,"Filters":[]}]`, str)

	// Add second value
	err = flag.Set(`{"Value":100,"Filters":[{"Name":"domainName","Value":"test"}]}`)
	require.NoError(t, err)

	str = flag.String()
	expected := `[
		{"Value":1000,"Filters":[]},
		{"Value":100,"Filters":[{"Name":"domainName","Value":"test"}]}
	]`
	assert.JSONEq(t, expected, str)
}
