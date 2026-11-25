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
	yamlFiles, err := filepath.Glob(filepath.Join(configFilesDir, "*.yaml"))
	require.NoError(t, err)
	ymlFiles, err := filepath.Glob(filepath.Join(configFilesDir, "*.yml"))
	require.NoError(t, err)
	files := append(yamlFiles, ymlFiles...)
	require.NotEmpty(t, files)

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			for keyName, values := range readConfigFile(t, file) {
				key, err := dynamicproperties.GetKeyFromKeyName(keyName)
				if !assert.NoError(t, err) {
					continue
				}
				for _, value := range values {
					assertValidValue(t, key, value)
				}
			}
		})
	}
}

// assertValidValue checks that the constraints of the value are known filters with a value, and that the value
// has the type of its key. The value is checked alone, so other values of the same key can't shadow it.
func assertValidValue(t *testing.T, key dynamicproperties.Key, value *constrainedValue) {
	for name, constraint := range value.Constraints {
		assert.NotEqual(t, dynamicproperties.UnknownFilter, dynamicproperties.ParseFilter(name), "key %s: unknown constraint %q", key, name)
		assert.NotNil(t, constraint, "key %s: constraint %q has no value", key, name)
	}
	client := &fileBasedClient{logger: log.NewNoop()}
	require.NoError(t, client.storeValues(map[string][]*constrainedValue{key.String(): {{Value: value.Value}}}))
	assert.NoError(t, getTypedValue(client, key), "key %s", key)
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

func getTypedValue(client Client, key dynamicproperties.Key) error {
	var err error
	switch k := key.(type) {
	case dynamicproperties.IntKey:
		_, err = client.GetIntValue(k, nil)
	case dynamicproperties.BoolKey:
		_, err = client.GetBoolValue(k, nil)
	case dynamicproperties.FloatKey:
		_, err = client.GetFloatValue(k, nil)
	case dynamicproperties.StringKey:
		_, err = client.GetStringValue(k, nil)
	case dynamicproperties.DurationKey:
		_, err = client.GetDurationValue(k, nil)
	case dynamicproperties.MapKey:
		_, err = client.GetMapValue(k, nil)
	case dynamicproperties.ListKey:
		_, err = client.GetListValue(k, nil)
	default:
		err = fmt.Errorf("unsupported key type %T", key)
	}
	return err
}
