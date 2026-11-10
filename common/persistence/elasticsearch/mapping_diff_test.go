package elasticsearch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMissingMappings(t *testing.T) {
	tests := []struct {
		name        string
		current     map[string]any
		expected    map[string]any
		wantMissing []string
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
			wantMissing: []string{"WorkflowID"},
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
			wantMissing: nil,
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
			wantMissing: []string{"CloseTime"},
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
			wantMissing: nil,
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
			wantMissing: []string{"Attr.CustomBoolField"},
		},
		{
			name: "missing parent is reported without descending into it",
			current: map[string]any{
				"properties": map[string]any{},
			},
			expected: map[string]any{
				"properties": map[string]any{
					"Attr": map[string]any{
						"properties": map[string]any{
							"CustomKeywordField": map[string]any{"type": "keyword"},
						},
					},
				},
			},
			wantMissing: []string{"Attr"},
		},
		{
			name: "results are sorted and deeply nested",
			current: map[string]any{
				"properties": map[string]any{
					"Attr": map[string]any{
						"properties": map[string]any{
							"Nested": map[string]any{
								"properties": map[string]any{},
							},
						},
					},
				},
			},
			expected: map[string]any{
				"properties": map[string]any{
					"WorkflowID": map[string]any{"type": "keyword"},
					"Attr": map[string]any{
						"properties": map[string]any{
							"CustomBoolField": map[string]any{"type": "boolean"},
							"Nested": map[string]any{
								"properties": map[string]any{
									"Deep": map[string]any{"type": "long"},
								},
							},
						},
					},
				},
			},
			wantMissing: []string{"Attr.CustomBoolField", "Attr.Nested.Deep", "WorkflowID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing, err := GetMissingMappings(tt.current, tt.expected)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMissing, missing)
		})
	}
}
