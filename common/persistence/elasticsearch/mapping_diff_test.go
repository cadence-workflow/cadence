package elasticsearch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsMissingMappings(t *testing.T) {
	tests := []struct {
		name        string
		current     map[string]any
		expected    map[string]any
		wantMissing bool
	}{
		{
			name: "current is empty",
			current: map[string]any{
				"properties": map[string]any{},
			},
			expected: map[string]any{
				"properties": map[string]any{
					"WorkflowID": map[string]any{"type": "keyword"},
				},
			},
			wantMissing: true,
		},
		{
			name: "current is allowed to have values not present in expected",
			current: map[string]any{
				"properties": map[string]any{
					"WorkflowID": map[string]any{"type": "keyword"},
					"ExtraField": map[string]any{"type": "text"},
				},
			},
			expected: map[string]any{
				"properties": map[string]any{
					"WorkflowID": map[string]any{"type": "keyword"},
				},
			},
			wantMissing: false,
		},
		{
			name: "expected is not allowed to have values not present in current",
			current: map[string]any{
				"properties": map[string]any{
					"WorkflowID": map[string]any{"type": "keyword"},
				},
			},
			expected: map[string]any{
				"properties": map[string]any{
					"WorkflowID": map[string]any{"type": "keyword"},
					"CloseTime":  map[string]any{"type": "long"},
				},
			},
			wantMissing: true,
		},
		{
			name: "current and expected are equal",
			current: map[string]any{
				"dynamic": "false",
				"properties": map[string]any{
					"WorkflowID": map[string]any{"type": "keyword"},
					"CloseTime":  map[string]any{"type": "long"},
				},
			},
			expected: map[string]any{
				"dynamic": "false",
				"properties": map[string]any{
					"WorkflowID": map[string]any{"type": "keyword"},
					"CloseTime":  map[string]any{"type": "long"},
				},
			},
			wantMissing: false,
		},
		{
			name: "there are differences in nested properties",
			current: map[string]any{
				"properties": map[string]any{
					"Attr": map[string]any{
						"properties": map[string]any{
							"CustomKeywordField": map[string]any{"type": "keyword"},
						},
					},
				},
			},
			expected: map[string]any{
				"properties": map[string]any{
					"Attr": map[string]any{
						"properties": map[string]any{
							"CustomKeywordField": map[string]any{"type": "keyword"},
							"CustomBoolField":    map[string]any{"type": "boolean"},
						},
					},
				},
			},
			wantMissing: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing, err := IsMissingMappings(tt.current, tt.expected)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMissing, missing)
		})
	}
}
