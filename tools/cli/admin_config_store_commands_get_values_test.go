package cli

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

func TestAdminUpdateDynamicConfig_WithValueFile(t *testing.T) {
	// Create a temporary JSON file with config values
	content := `[
		{"Value": 1000, "Filters": []},
		{"Value": 100, "Filters": [{"Name": "domainName", "Value": "test-domain"}]}
	]`
	tmpfile, err := os.CreateTemp("", "test_config_*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(content))
	assert.NoError(t, err)
	tmpfile.Close()

	// Test with --value-file flag
	td := newCLITestData(t)
	td.mockAdminClient.EXPECT().UpdateDynamicConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request *types.UpdateDynamicConfigRequest, _ ...yarpc.CallOption) error {
			assert.Equal(t, "test-config", request.ConfigName)
			assert.Len(t, request.ConfigValues, 2)

			// Verify first value (global default)
			var val1 interface{}
			err := json.Unmarshal(request.ConfigValues[0].Value.Data, &val1)
			assert.NoError(t, err)
			assert.Equal(t, float64(1000), val1)
			assert.Empty(t, request.ConfigValues[0].Filters)

			// Verify second value (domain-specific)
			var val2 interface{}
			err = json.Unmarshal(request.ConfigValues[1].Value.Data, &val2)
			assert.NoError(t, err)
			assert.Equal(t, float64(100), val2)
			assert.Len(t, request.ConfigValues[1].Filters, 1)
			assert.Equal(t, "domainName", request.ConfigValues[1].Filters[0].Name)

			var filterVal interface{}
			err = json.Unmarshal(request.ConfigValues[1].Filters[0].Value.Data, &filterVal)
			assert.NoError(t, err)
			assert.Equal(t, "test-domain", filterVal)

			return nil
		})

	cmdline := `cadence admin config update --name test-config --value-file ` + tmpfile.Name()
	err = clitest.RunCommandLine(t, td.app, cmdline)
	assert.NoError(t, err)
}

func TestAdminUpdateDynamicConfig_WithBothValueAndFile(t *testing.T) {
	// Create a temporary JSON file
	content := `[{"Value": 100, "Filters": [{"Name": "domainName", "Value": "file-domain"}]}]`
	tmpfile, err := os.CreateTemp("", "test_config_*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(content))
	assert.NoError(t, err)
	tmpfile.Close()

	// Test with both --value and --value-file (should fail - mutually exclusive)
	td := newCLITestData(t)

	cmdline := `cadence admin config update --name test-config --value '{"Value":1000,"Filters":[]}' --value-file ` + tmpfile.Name()
	err = clitest.RunCommandLine(t, td.app, cmdline)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestAdminUpdateDynamicConfig_NoValuesProvided(t *testing.T) {
	td := newCLITestData(t)

	cmdline := `cadence admin config update --name test-config`
	err := clitest.RunCommandLine(t, td.app, cmdline)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must provide either")
}

func TestAdminUpdateDynamicConfig_EmptyFileArray(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		errorMsg string
	}{
		{
			name:     "empty array",
			content:  `[]`,
			errorMsg: "contains no config values",
		},
		{
			name:     "null",
			content:  `null`,
			errorMsg: "contains no config values",
		},
		{
			name:     "null element in array",
			content:  `[{"Value": 1, "Filters": []}, null]`,
			errorMsg: "contains null element at index 1",
		},
		{
			name:     "null filter in Filters array",
			content:  `[{"Value": 1, "Filters": [null]}]`,
			errorMsg: "null filter at index 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "test_config_*.json")
			assert.NoError(t, err)
			defer os.Remove(tmpfile.Name())

			_, err = tmpfile.Write([]byte(tt.content))
			assert.NoError(t, err)
			tmpfile.Close()

			// Test with invalid file (should fail with validation error, not panic)
			td := newCLITestData(t)

			cmdline := `cadence admin config update --name test-config --value-file ` + tmpfile.Name()
			err = clitest.RunCommandLine(t, td.app, cmdline)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

func TestAdminUpdateOperationalDynamicConfig_WithValueFile(t *testing.T) {
	// Create a temporary JSON file with config values
	content := `[
		{"Value": 2000, "Filters": []},
		{"Value": 200, "Filters": [{"Name": "domainName", "Value": "ops-domain"}]}
	]`
	tmpfile, err := os.CreateTemp("", "test_config_*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(content))
	assert.NoError(t, err)
	tmpfile.Close()

	// Test with --value-file flag
	td := newCLITestData(t)
	td.mockAdminClient.EXPECT().UpdateOperationalDynamicConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request *types.UpdateOperationalDynamicConfigRequest, _ ...yarpc.CallOption) error {
			assert.Equal(t, "test-config", request.ConfigName)
			assert.Len(t, request.ConfigValues, 2)

			// Verify first value (global default)
			var val1 interface{}
			err := json.Unmarshal(request.ConfigValues[0].Value.Data, &val1)
			assert.NoError(t, err)
			assert.Equal(t, float64(2000), val1)
			assert.Empty(t, request.ConfigValues[0].Filters)

			// Verify second value (domain-specific)
			var val2 interface{}
			err = json.Unmarshal(request.ConfigValues[1].Value.Data, &val2)
			assert.NoError(t, err)
			assert.Equal(t, float64(200), val2)
			assert.Len(t, request.ConfigValues[1].Filters, 1)
			assert.Equal(t, "domainName", request.ConfigValues[1].Filters[0].Name)

			var filterVal interface{}
			err = json.Unmarshal(request.ConfigValues[1].Filters[0].Value.Data, &filterVal)
			assert.NoError(t, err)
			assert.Equal(t, "ops-domain", filterVal)

			return nil
		})

	cmdline := `cadence admin config operational-update --name test-config --value-file ` + tmpfile.Name()
	err = clitest.RunCommandLine(t, td.app, cmdline)
	assert.NoError(t, err)
}

func TestAdminUpdateOperationalDynamicConfig_EmptyFileArray(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		errorMsg string
	}{
		{
			name:     "empty array",
			content:  `[]`,
			errorMsg: "contains no config values",
		},
		{
			name:     "null element in array",
			content:  `[{"Value": 1, "Filters": []}, null]`,
			errorMsg: "contains null element at index 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "test_config_*.json")
			assert.NoError(t, err)
			defer os.Remove(tmpfile.Name())

			_, err = tmpfile.Write([]byte(tt.content))
			assert.NoError(t, err)
			tmpfile.Close()

			// Test with invalid file (should fail with validation error, not panic)
			td := newCLITestData(t)

			cmdline := `cadence admin config operational-update --name test-config --value-file ` + tmpfile.Name()
			err = clitest.RunCommandLine(t, td.app, cmdline)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}
