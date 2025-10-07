// Copyright (c) 2019 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package config

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	uconfig "go.uber.org/config"
	"gopkg.in/validator.v2"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

func TestToString(t *testing.T) {
	var cfg Config
	err := Load("", "../../config", "", &cfg)
	assert.NoError(t, err)
	assert.NotEmpty(t, cfg.String())
}

func TestFillingDefaultSQLEncodingDecodingTypes(t *testing.T) {
	cfg := &Config{
		Persistence: Persistence{
			DataStores: map[string]DataStore{
				"sql": {
					SQL: &SQL{},
				},
			},
		},
		ClusterGroupMetadata: &ClusterGroupMetadata{},
	}
	cfg.fillDefaults()
	assert.Equal(t, string(constants.EncodingTypeThriftRW), cfg.Persistence.DataStores["sql"].SQL.EncodingType)
	assert.Equal(t, []string{string(constants.EncodingTypeThriftRW)}, cfg.Persistence.DataStores["sql"].SQL.DecodingTypes)
}

func getValidMultipleDatabasseConfig() *Config {
	metadata := validClusterGroupMetadata()
	cfg := &Config{
		ClusterGroupMetadata: metadata,
		Persistence: Persistence{
			DefaultStore:            "default",
			AdvancedVisibilityStore: "esv7",
			DataStores: map[string]DataStore{
				"default": {
					SQL: &SQL{
						PluginName:           "fake",
						ConnectProtocol:      "tcp",
						NumShards:            2,
						UseMultipleDatabases: true,
						MultipleDatabasesConfig: []MultipleDatabasesConfigEntry{
							{
								DatabaseName: "db1",
								ConnectAddr:  "192.168.0.1:3306",
							},
							{
								DatabaseName: "db2",
								ConnectAddr:  "192.168.0.2:3306",
							},
						},
					},
				},
				"esv7": {
					ElasticSearch: &ElasticSearchConfig{
						Version: "v7",
						URL: url.URL{Scheme: "http",
							Host: "127.0.0.1:9200",
						},
						Indices: map[string]string{
							"visibility": "cadence-visibility-dev",
						},
					},
					// no sql or nosql, should be populated from cassandra
				},
			},
		},
	}
	return cfg
}

func TestValidMultipleDatabaseConfig(t *testing.T) {
	cfg := getValidMultipleDatabasseConfig()
	err := cfg.ValidateAndFillDefaults()
	require.NoError(t, err)
}

func TestInvalidMultipleDatabaseConfig_useBasicVisibility(t *testing.T) {
	cfg := getValidMultipleDatabasseConfig()
	cfg.Persistence.VisibilityStore = "basic"
	cfg.Persistence.DataStores["basic"] = DataStore{
		SQL: &SQL{},
	}
	err := cfg.ValidateAndFillDefaults()
	require.EqualError(t, err, "sql persistence config: multipleSQLDatabases can only be used with advanced visibility only")
}

func TestInvalidMultipleDatabaseConfig_wrongNumDBShards(t *testing.T) {
	cfg := getValidMultipleDatabasseConfig()
	sqlds := cfg.Persistence.DataStores["default"]
	sqlds.SQL.NumShards = 3
	cfg.Persistence.DataStores["default"] = sqlds
	err := cfg.ValidateAndFillDefaults()
	require.EqualError(t, err, "sql persistence config: nShards must be greater than one and equal to the length of multipleDatabasesConfig")
}

func TestInvalidMultipleDatabaseConfig_nonEmptySQLUser(t *testing.T) {
	cfg := getValidMultipleDatabasseConfig()
	sqlds := cfg.Persistence.DataStores["default"]
	sqlds.SQL.User = "user"
	cfg.Persistence.DataStores["default"] = sqlds
	err := cfg.ValidateAndFillDefaults()
	require.EqualError(t, err, "sql persistence config: user can only be configured in multipleDatabasesConfig when UseMultipleDatabases is true")
}

func TestInvalidMultipleDatabaseConfig_nonEmptySQLPassword(t *testing.T) {
	cfg := getValidMultipleDatabasseConfig()
	sqlds := cfg.Persistence.DataStores["default"]
	sqlds.SQL.Password = "pw"
	cfg.Persistence.DataStores["default"] = sqlds
	err := cfg.ValidateAndFillDefaults()
	require.EqualError(t, err, "sql persistence config: password can only be configured in multipleDatabasesConfig when UseMultipleDatabases is true")
}

func TestInvalidMultipleDatabaseConfig_nonEmptySQLDatabaseName(t *testing.T) {
	cfg := getValidMultipleDatabasseConfig()
	sqlds := cfg.Persistence.DataStores["default"]
	sqlds.SQL.DatabaseName = "db"
	cfg.Persistence.DataStores["default"] = sqlds
	err := cfg.ValidateAndFillDefaults()
	require.EqualError(t, err, "sql persistence config: databaseName can only be configured in multipleDatabasesConfig when UseMultipleDatabases is true")
}

func TestInvalidMultipleDatabaseConfig_nonEmptySQLConnAddr(t *testing.T) {
	cfg := getValidMultipleDatabasseConfig()
	sqlds := cfg.Persistence.DataStores["default"]
	sqlds.SQL.ConnectAddr = "127.0.0.1:3306"
	cfg.Persistence.DataStores["default"] = sqlds
	err := cfg.ValidateAndFillDefaults()
	require.EqualError(t, err, "sql persistence config: connectAddr can only be configured in multipleDatabasesConfig when UseMultipleDatabases is true")
}

func TestConfigFallbacks(t *testing.T) {
	metadata := validClusterGroupMetadata()
	cfg := &Config{
		Services: map[string]Service{
			"frontend": {
				RPC: RPC{
					Port: 7900,
				},
			},
		},
		ClusterGroupMetadata: metadata,
		Persistence: Persistence{
			DefaultStore:    "default",
			VisibilityStore: "cass",
			DataStores: map[string]DataStore{
				"default": {
					SQL: &SQL{
						PluginName:      "fake",
						ConnectProtocol: "tcp",
						ConnectAddr:     "192.168.0.1:3306",
						DatabaseName:    "db1",
						NumShards:       0, // default value, should be changed
					},
				},
				"cass": {
					Cassandra: &NoSQL{
						Hosts: "127.0.0.1",
					},
					// no sql or nosql, should be populated from cassandra
				},
			},
		},
	}
	err := cfg.ValidateAndFillDefaults()
	require.NoError(t, err) // sanity check, must be valid or the later tests are potentially useless

	assert.NotEmpty(t, cfg.Persistence.DataStores["cass"].Cassandra, "cassandra config should remain after update")
	assert.NotEmpty(t, cfg.Persistence.DataStores["cass"].NoSQL, "nosql config should contain cassandra config / not be empty")
	assert.NotZero(t, cfg.Persistence.DataStores["default"].SQL.NumShards, "num shards should be nonzero")
	assert.Equal(t, "localhost:7833", cfg.PublicClient.HostPort)
}

func TestConfigErrorInAuthorizationConfig(t *testing.T) {
	cfg := &Config{
		Authorization: Authorization{
			OAuthAuthorizer: OAuthAuthorizer{
				Enable: true,
			},
			NoopAuthorizer: NoopAuthorizer{
				Enable: true,
			},
		},
		ClusterGroupMetadata: &ClusterGroupMetadata{},
	}

	err := cfg.ValidateAndFillDefaults()
	require.Error(t, err)
}

func TestGetServiceConfig(t *testing.T) {
	cfg := Config{}
	_, err := cfg.GetServiceConfig(service.Frontend)
	assert.EqualError(t, err, "no config section for service: frontend")

	cfg = Config{Services: map[string]Service{"frontend": {RPC: RPC{GRPCPort: 123}}}}
	svc, err := cfg.GetServiceConfig(service.Frontend)
	assert.NoError(t, err)
	assert.NotEmpty(t, svc)
}

func getValidShardedNoSQLConfig() *Config {
	metadata := validClusterGroupMetadata()
	cfg := &Config{
		ClusterGroupMetadata: metadata,
		Persistence: Persistence{
			NumHistoryShards:        2,
			DefaultStore:            "default",
			AdvancedVisibilityStore: "visibility",
			DataStores: map[string]DataStore{
				"default": {
					ShardedNoSQL: &ShardedNoSQL{
						DefaultShard: "shard-1",
						ShardingPolicy: ShardingPolicy{
							HistoryShardMapping: []HistoryShardRange{
								HistoryShardRange{
									Start: 0,
									End:   1,
									Shard: "shard-1",
								},
								HistoryShardRange{
									Start: 1,
									End:   2,
									Shard: "shard-2",
								},
							},
							TaskListHashing: TasklistHashing{
								ShardOrder: []string{
									"shard-1",
									"shard-2",
								},
							},
						},
						Connections: map[string]DBShardConnection{
							"shard-1": {
								NoSQLPlugin: &NoSQL{
									PluginName: "cassandra",
									Hosts:      "127.0.0.1",
									Keyspace:   "unit-test",
									Port:       1234,
								},
							},
							"shard-2": {
								NoSQLPlugin: &NoSQL{
									PluginName: "cassandra",
									Hosts:      "127.0.0.1",
									Keyspace:   "unit-test",
									Port:       5678,
								},
							},
						},
					},
				},
				"visibility": {
					ElasticSearch: &ElasticSearchConfig{
						Version: "v7",
						URL: url.URL{Scheme: "http",
							Host: "127.0.0.1:9200",
						},
						Indices: map[string]string{
							"visibility": "cadence-visibility-dev",
						},
					},
				},
			},
		},
	}
	return cfg
}

func TestValidShardedNoSQLConfig(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()
	err := cfg.ValidateAndFillDefaults()
	require.NoError(t, err)
}

func TestInvalidShardedNoSQLConfig_MultipleConfigTypes(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()
	store := cfg.Persistence.DataStores["default"]
	store.NoSQL = &NoSQL{}
	cfg.Persistence.DataStores["default"] = store

	err := cfg.ValidateAndFillDefaults()
	require.ErrorContains(t, err, "must provide exactly one type of config, but provided 2")
}

func TestInvalidShardedNoSQLConfig_MissingDefaultShard(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()
	store := cfg.Persistence.DataStores["default"]
	store.ShardedNoSQL.DefaultShard = ""

	err := cfg.ValidateAndFillDefaults()
	require.ErrorContains(t, err, "defaultShard can not be empty")
}

func TestInvalidShardedNoSQLConfig_UnknownDefaultShard(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()
	store := cfg.Persistence.DataStores["default"]
	delete(store.ShardedNoSQL.Connections, store.ShardedNoSQL.DefaultShard)

	err := cfg.ValidateAndFillDefaults()
	require.ErrorContains(t, err, "defaultShard (shard-1) is not defined in connections list")
}

func TestInvalidShardedNoSQLConfig_HistoryShardingUnordered(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()
	store := cfg.Persistence.DataStores["default"]
	store.ShardedNoSQL.ShardingPolicy.HistoryShardMapping[0].Start = 1
	store.ShardedNoSQL.ShardingPolicy.HistoryShardMapping[0].End = 2
	store.ShardedNoSQL.ShardingPolicy.HistoryShardMapping[1].Start = 0
	store.ShardedNoSQL.ShardingPolicy.HistoryShardMapping[1].End = 1

	err := cfg.ValidateAndFillDefaults()
	require.ErrorContains(t, err, "Non-continuous history shard range")
}

func TestInvalidShardedNoSQLConfig_HistoryShardingOverlapping(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()
	store := cfg.Persistence.DataStores["default"]
	store.ShardedNoSQL.ShardingPolicy.HistoryShardMapping[0].End = 2 // 0-2 overlaps with 1-2

	err := cfg.ValidateAndFillDefaults()
	require.ErrorContains(t, err, "Non-continuous history shard range")
}

func TestInvalidShardedNoSQLConfig_HistoryShardingMissingFirstShard(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()
	store := cfg.Persistence.DataStores["default"]
	store.ShardedNoSQL.ShardingPolicy.HistoryShardMapping = store.ShardedNoSQL.ShardingPolicy.HistoryShardMapping[1:]

	err := cfg.ValidateAndFillDefaults()
	require.ErrorContains(t, err, "Non-continuous history shard range")
}

func TestInvalidShardedNoSQLConfig_HistoryShardingMissingLastShard(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()
	cfg.Persistence.NumHistoryShards = 3 // config only specifies shards 0 and 1, so this is invalid

	err := cfg.ValidateAndFillDefaults()
	require.ErrorContains(t, err, "Last history shard found in the config is 1 while the max is 2")
}

func TestInvalidShardedNoSQLConfig_HistoryShardingRefersToUnknownConnection(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()
	store := cfg.Persistence.DataStores["default"]
	store.ShardedNoSQL.ShardingPolicy.HistoryShardMapping[0].Shard = "unknown-shard-name"

	err := cfg.ValidateAndFillDefaults()
	require.ErrorContains(t, err, "Unknown history shard name")
}

func TestInvalidShardedNoSQLConfig_TasklistShardingRefersToUnknownConnection(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()
	store := cfg.Persistence.DataStores["default"]
	store.ShardedNoSQL.ShardingPolicy.TaskListHashing.ShardOrder[1] = "unknown-shard-name"

	err := cfg.ValidateAndFillDefaults()
	require.ErrorContains(t, err, "Unknown tasklist shard name")
}

func TestHistogramMigrationConfig(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		orig := metrics.HistogramMigrationMetrics
		t.Cleanup(func() {
			metrics.HistogramMigrationMetrics = orig
		})
		metrics.HistogramMigrationMetrics = map[string]struct{}{
			"key1": {},
			"key2": {},
			"key3": {},
			"key4": {},
		}

		// intentionally avoiding the full config struct, as it has other required config
		yaml, err := uconfig.NewYAML(uconfig.RawSource(strings.NewReader(`
default: histogram
names:
  key1: true
  key2: false
  key3:
`)))
		require.NoError(t, err)

		var cfg metrics.HistogramMigration
		err = yaml.Get(uconfig.Root).Populate(&cfg)
		require.NoError(t, err)

		err = validator.Validate(cfg)
		require.NoError(t, err)

		check := func(key string, timer, histogram bool) {
			assert.Equalf(t, timer, cfg.EmitTimer(key), "wrong value for EmitTimer(%q)", key)
			assert.Equalf(t, histogram, cfg.EmitHistogram(key), "wrong value for EmitHistogram(%q)", key)
		}
		check("key1", true, true)
		check("key2", false, false)
		check("key3", false, false) // the type's default mode == false.  not truly intended behavior, but it's weird config so it's fine.
		check("key4", false, true)  // configured default == histogram
		check("key5", true, true)   // not migrating = always emitted
		if t.Failed() {
			t.Logf("config: %#v", cfg)
		}
	})
	t.Run("invalid default", func(t *testing.T) {
		yaml, err := uconfig.NewYAML(uconfig.RawSource(strings.NewReader(`
default: xyz
`)))
		require.NoError(t, err)

		var cfg metrics.HistogramMigration
		err = yaml.Get(uconfig.Root).Populate(&cfg)
		assert.ErrorContains(t, err, `unsupported histogram migration mode "xyz", must be "timer", "histogram", or "both"`)
	})
	t.Run("invalid key", func(t *testing.T) {
		orig := metrics.HistogramMigrationMetrics
		t.Cleanup(func() {
			metrics.HistogramMigrationMetrics = orig
		})
		metrics.HistogramMigrationMetrics = map[string]struct{}{
			"key1": {},
		}
		yaml, err := uconfig.NewYAML(uconfig.RawSource(strings.NewReader(`
names:
  key1: xyz
`)))
		require.NoError(t, err)

		var cfg metrics.HistogramMigration
		err = yaml.Get(uconfig.Root).Populate(&cfg)
		assert.ErrorContains(t, err, "cannot unmarshal !!str `xyz` into bool")
	})
	t.Run("nonexistent key", func(t *testing.T) {
		yaml, err := uconfig.NewYAML(uconfig.RawSource(strings.NewReader(`
names:
  definitely_does_not_exist: true
`)))
		require.NoError(t, err)

		var cfg metrics.HistogramMigration
		err = yaml.Get(uconfig.Root).Populate(&cfg)
		assert.ErrorContains(t, err, `unknown histogram-migration metric name "definitely_does_not_exist"`)
	})
}
