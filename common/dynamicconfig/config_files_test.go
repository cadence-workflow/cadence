package dynamicconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
)

const configFilesDir = "../../config/dynamicconfig"

func TestConfigFilesAreValid(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(configFilesDir, "*.y*ml"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			client := newConfigFileClient(t, file)
			for keyName, values := range readConfigFile(t, file) {
				key, err := dynamicproperties.GetKeyFromKeyName(keyName)
				if !assert.NoError(t, err) {
					continue
				}
				for _, value := range values {
					filters := make(map[dynamicproperties.Filter]interface{}, len(value.Constraints))
					for name, constraint := range value.Constraints {
						filter := dynamicproperties.ParseFilter(name)
						assert.NotEqual(t, dynamicproperties.UnknownFilter, filter, "key %s: unknown constraint %q", keyName, name)
						filters[filter] = constraint
					}
					assert.NoError(t, getTypedValue(client, key, filters), "key %s", keyName)
				}
			}
		})
	}
}

func TestDevelopmentConfigsCancelOutstandingPollsOnShutdown(t *testing.T) {
	for _, file := range []string{"development.yaml", "development_es.yaml", "development_pinot.yaml"} {
		t.Run(file, func(t *testing.T) {
			client := newConfigFileClient(t, filepath.Join(configFilesDir, file))

			enabled, err := client.GetBoolValue(dynamicproperties.FrontendCancelOutstandingPollsOnShutdown, nil)

			require.NoError(t, err)
			assert.True(t, enabled)
		})
	}
}

func newConfigFileClient(t *testing.T, file string) Client {
	doneCh := make(chan struct{})
	t.Cleanup(func() { close(doneCh) })
	client, err := NewFileBasedClient(&FileBasedClientConfig{Filepath: file}, log.NewNoop(), doneCh)
	require.NoError(t, err)
	return client
}

func readConfigFile(t *testing.T, file string) map[string][]*constrainedValue {
	content, err := os.ReadFile(file)
	require.NoError(t, err)
	values := make(map[string][]*constrainedValue)
	require.NoError(t, yaml.Unmarshal(content, values))
	return values
}

func getTypedValue(client Client, key dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}) error {
	var err error
	switch k := key.(type) {
	case dynamicproperties.IntKey:
		_, err = client.GetIntValue(k, filters)
	case dynamicproperties.BoolKey:
		_, err = client.GetBoolValue(k, filters)
	case dynamicproperties.FloatKey:
		_, err = client.GetFloatValue(k, filters)
	case dynamicproperties.StringKey:
		_, err = client.GetStringValue(k, filters)
	case dynamicproperties.DurationKey:
		_, err = client.GetDurationValue(k, filters)
	case dynamicproperties.MapKey:
		_, err = client.GetMapValue(k, filters)
	case dynamicproperties.ListKey:
		_, err = client.GetListValue(k, filters)
	default:
		err = fmt.Errorf("unsupported key type %T", key)
	}
	return err
}
