package clientcommon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
)

func TestNamespaceConfig_GetMigrationMode(t *testing.T) {
	tests := []struct {
		name          string
		migrationMode string
		expected      types.MigrationMode
	}{
		{
			name:          "local_passthrough",
			migrationMode: "local_pass",
			expected:      types.MigrationModeLOCALPASSTHROUGH,
		},
		{
			name:          "local_passthrough_shadow",
			migrationMode: "local_pass_shadow",
			expected:      types.MigrationModeLOCALPASSTHROUGHSHADOW,
		},
		{
			name:          "distributed_passthrough",
			migrationMode: "distributed_pass",
			expected:      types.MigrationModeDISTRIBUTEDPASSTHROUGH,
		},
		{
			name:          "onboarded",
			migrationMode: "onboarded",
			expected:      types.MigrationModeONBOARDED,
		},
		{
			name:          "invalid",
			migrationMode: "invalid",
			expected:      types.MigrationModeINVALID,
		},
		{
			name:          "empty string",
			migrationMode: "",
			expected:      types.MigrationModeINVALID,
		},
		{
			name:          "unknown mode",
			migrationMode: "unknown_mode",
			expected:      types.MigrationModeINVALID,
		},
		{
			name:          "case insensitive - uppercase",
			migrationMode: "ONBOARDED",
			expected:      types.MigrationModeONBOARDED,
		},
		{
			name:          "case insensitive - mixed case",
			migrationMode: "Local_Pass",
			expected:      types.MigrationModeLOCALPASSTHROUGH,
		},
		{
			name:          "whitespace trimming",
			migrationMode: "  onboarded  ",
			expected:      types.MigrationModeONBOARDED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &NamespaceConfig{
				MigrationMode: tt.migrationMode,
			}
			assert.Equal(t, tt.expected, config.GetMigrationMode())
		})
	}
}
