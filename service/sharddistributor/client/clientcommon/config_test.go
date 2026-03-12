package clientcommon

import (
	"testing"
	"time"

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

func TestConfig_GetPeerTTL(t *testing.T) {
	tests := []struct {
		name     string
		peerTTL  time.Duration
		expected time.Duration
	}{
		{
			name:     "zero uses default",
			peerTTL:  0,
			expected: 2 * time.Minute,
		},
		{
			name:     "non-zero returned as-is",
			peerTTL:  5 * time.Minute,
			expected: 5 * time.Minute,
		},
		{
			name:     "negative uses default",
			peerTTL:  -1 * time.Second,
			expected: 2 * time.Minute,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{PeerTTL: tc.peerTTL}
			assert.Equal(t, tc.expected, cfg.GetPeerTTL())
		})
	}
}
