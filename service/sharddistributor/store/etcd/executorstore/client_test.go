package executorstore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx/fxtest"
	"gopkg.in/yaml.v2"

	commonconfig "github.com/uber/cadence/common/config"
	"github.com/uber/cadence/service/sharddistributor/config"
)

func TestNewClient_WithProvidedClient(t *testing.T) {
	mockClient := &clientv3.Client{}
	params := ClientParams{
		Client: mockClient,
	}

	output, err := NewClient(params)

	require.NoError(t, err)
	require.Equal(t, mockClient, output.Client)
}

func TestNewClient_WithInvalidConfig(t *testing.T) {
	params := ClientParams{
		Cfg: ETCDConfig{
			Endpoints:   []string{},
			DialTimeout: 5 * time.Second,
		},
		Lifecycle: fxtest.NewLifecycle(t),
	}

	output, err := NewClient(params)

	require.Error(t, err)
	require.Nil(t, output.Client)
}

func TestNewETCDConfig_WithValidConfig(t *testing.T) {
	etcdCfg := ETCDConfig{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
		Prefix:      "/prefix",
		Compression: "none",
	}

	encoded, err := yaml.Marshal(etcdCfg)
	require.NoError(t, err)

	decoded := &commonconfig.YamlNode{}
	err = yaml.Unmarshal(encoded, decoded)
	require.NoError(t, err)

	sdConfig := config.ShardDistribution{
		Store: config.Store{
			StorageParams: decoded,
		},
	}

	resultCfg, err := NewETCDConfig(sdConfig)
	require.NoError(t, err)
	require.Equal(t, etcdCfg, resultCfg)
}

func TestNewETCDConfig_WithInvalidConfig(t *testing.T) {
	encoded, err := yaml.Marshal("")
	require.NoError(t, err)

	decoded := &commonconfig.YamlNode{}
	err = yaml.Unmarshal(encoded, decoded)
	require.NoError(t, err)

	sdConfig := config.ShardDistribution{
		Store: config.Store{
			StorageParams: decoded,
		},
	}

	_, err = NewETCDConfig(sdConfig)
	require.Error(t, err)
}
