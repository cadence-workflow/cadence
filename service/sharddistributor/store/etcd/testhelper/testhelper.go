package testhelper

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common/config"
	shardDistributorCfg "github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/testflags"
)

type StoreTestCluster struct {
	EtcdPrefix  string
	Namespace   string
	LeaderCfg   shardDistributorCfg.ShardDistribution
	Client      *clientv3.Client
	Compression string
}

func SetupStoreTestCluster(t *testing.T) *StoreTestCluster {
	t.Helper()
	flag.Parse()
	testflags.RequireEtcd(t)

	namespace := fmt.Sprintf("ns-%s", strings.ToLower(t.Name()))

	endpoints := strings.Split(os.Getenv("ETCD_ENDPOINTS"), ",")
	if len(endpoints) == 0 || endpoints[0] == "" {
		endpoints = []string{"localhost:2379"}
	}
	t.Logf("ETCD endpoints: %v", endpoints)

	etcdPrefix := fmt.Sprintf("/test-shard-store/%s", t.Name())
	etcdConfigRaw := map[string]interface{}{
		"endpoints":   endpoints,
		"dialTimeout": "5s",
		"prefix":      etcdPrefix,
		"electionTTL": "5s", // Needed for leader config part
		"compression": "snappy",
	}

	yamlCfg, err := yaml.Marshal(etcdConfigRaw)
	require.NoError(t, err)
	var yamlNode *config.YamlNode
	err = yaml.Unmarshal(yamlCfg, &yamlNode)
	require.NoError(t, err)

	leaderCfg := shardDistributorCfg.ShardDistribution{
		Store:       shardDistributorCfg.Store{StorageParams: yamlNode},
		LeaderStore: shardDistributorCfg.Store{StorageParams: yamlNode},
	}

	client, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: 5 * time.Second})
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	_, err = client.Delete(context.Background(), etcdPrefix, clientv3.WithPrefix())
	require.NoError(t, err)

	return &StoreTestCluster{
		Namespace:  namespace,
		EtcdPrefix: etcdPrefix,
		LeaderCfg:  leaderCfg,
		Client:     client,
	}
}
