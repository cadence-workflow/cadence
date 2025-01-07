// Copyright (c) 2023 Uber Technologies, Inc.
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

package nosql

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	. "github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

type shardedNosqlStoreTestSuite struct {
	suite.Suite
	*require.Assertions
	mockController *gomock.Controller

	mockPlugin *nosqlplugin.MockPlugin
}

func (s *shardedNosqlStoreTestSuite) SetupSuite() {
	s.mockController = gomock.NewController(s.T())
}

func (s *shardedNosqlStoreTestSuite) TearDownSuite() {
}

func (s *shardedNosqlStoreTestSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	mockDB := nosqlplugin.NewMockDB(s.mockController)

	mockPlugin := nosqlplugin.NewMockPlugin(s.mockController)
	mockPlugin.EXPECT().
		CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(mockDB, nil).AnyTimes()
	delete(supportedPlugins, "cassandra")
	RegisterPlugin("cassandra", mockPlugin)
}

func TestShardedNosqlStoreTestSuite(t *testing.T) {
	s := new(shardedNosqlStoreTestSuite)
	suite.Run(t, s)
}

func (s *shardedNosqlStoreTestSuite) TestValidConfiguration() {
	store := s.newShardedStoreForTest()
	s.Equal(1, len(store.connectedShards))
	s.Contains(store.connectedShards, "shard-1")
	s.Equal(store.GetDefaultShard(), store.defaultShard)
	s.Equal(store.connectedShards["shard-1"], store.defaultShard)
	s.Equal("shard-1", store.shardingPolicy.defaultShard)
	s.True(store.shardingPolicy.hasShardedTasklist)
	s.True(store.shardingPolicy.hasShardedHistory)
}

func (s *shardedNosqlStoreTestSuite) TestStoreSelectionForHistoryShard() {
	mockDB1 := nosqlplugin.NewMockDB(s.mockController)
	mockDB1.EXPECT().Close().Times(1)
	mockDB2 := nosqlplugin.NewMockDB(s.mockController)
	mockDB2.EXPECT().Close().Times(1)

	mockPlugin := nosqlplugin.NewMockPlugin(s.mockController)
	gomock.InOrder(
		mockPlugin.EXPECT().
			CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(mockDB1, nil),
		mockPlugin.EXPECT().
			CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("error creating db")),
		mockPlugin.EXPECT().
			CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(mockDB2, nil),
	)
	delete(supportedPlugins, "cassandra")
	RegisterPlugin("cassandra", mockPlugin)

	store := s.newShardedStoreForTest()
	defer store.Close()

	s.Equal(1, len(store.connectedShards))
	s.True(mockDB1 == store.defaultShard.db)

	// Shard 0 is same default shard in this test, so connectedShards shouldn't change
	storeShard1, err := store.GetStoreShardByHistoryShard(0)
	s.NoError(err)
	s.Equal(1, len(store.connectedShards))
	s.True(mockDB1 == storeShard1.db)

	// Getting the same shard again shouldn't create a new connection
	storeShard1, err = store.GetStoreShardByHistoryShard(0)
	s.NoError(err)
	s.Equal(1, len(store.connectedShards))
	s.True(mockDB1 == storeShard1.db)

	// Getting a new shard should create a new connection but it will fail on first attempt
	storeShard2, err := store.GetStoreShardByHistoryShard(1)
	s.Error(err)
	s.Equal(1, len(store.connectedShards))

	// Getting a new shard should create a new connection on second attempt
	storeShard2, err = store.GetStoreShardByHistoryShard(1)
	s.NoError(err)
	s.Equal(2, len(store.connectedShards))
	s.True(mockDB2 == storeShard2.db)

	// After the new connection, getting the previous shard should still work as it used to
	storeShard1, err = store.GetStoreShardByHistoryShard(0)
	s.NoError(err)
	s.Equal(2, len(store.connectedShards))
	s.True(mockDB1 == storeShard1.db)

	// Getting a non-existing shard should result in an error
	unknownShard, err := store.GetStoreShardByHistoryShard(2)
	s.ErrorContains(err, "Failed to identify store shard")
	s.Empty(unknownShard)

	// Ensure the store shard connections created for history shards are available for tasklists, too
	storeShard1, err = store.GetStoreShardByTaskList("domain1", "tl1", 0)
	s.NoError(err)
	s.Equal(2, len(store.connectedShards))
	s.True(mockDB1 == storeShard1.db)
	storeShard2, err = store.GetStoreShardByTaskList("domain1", "tl2", 0)
	s.NoError(err)
	s.Equal(2, len(store.connectedShards))
	s.True(mockDB2 == storeShard2.db)
}

func (s *shardedNosqlStoreTestSuite) newShardedStoreForTest() *shardedNosqlStoreImpl {
	cfg := getValidShardedNoSQLConfig()
	logger := log.NewNoop()
	storeInterface, err := newShardedNosqlStore(cfg, logger, nil)
	s.NoError(err)
	s.Equal("shardedNosql", storeInterface.GetName())
	s.Equal(logger, storeInterface.GetLogger())
	store := storeInterface.(*shardedNosqlStoreImpl)
	s.Equal(storeInterface.GetShardingPolicy(), store.shardingPolicy)
	return store
}

func (s *shardedNosqlStoreTestSuite) TestStoreSelectionForTasklist() {
	mockDB1 := nosqlplugin.NewMockDB(s.mockController)
	mockDB1.EXPECT().Close().Times(1)
	mockDB2 := nosqlplugin.NewMockDB(s.mockController)
	mockDB2.EXPECT().Close().Times(1)

	mockPlugin := nosqlplugin.NewMockPlugin(s.mockController)
	gomock.InOrder(
		mockPlugin.EXPECT().
			CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(mockDB1, nil),
		mockPlugin.EXPECT().
			CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(mockDB2, nil),
	)
	delete(supportedPlugins, "cassandra")
	RegisterPlugin("cassandra", mockPlugin)

	store := s.newShardedStoreForTest()
	defer store.Close()

	s.Equal(1, len(store.connectedShards))
	s.True(mockDB1 == store.defaultShard.db)

	// Shard 0 is same default shard in this test, so connectedShards shouldn't change
	storeShard1, err := store.GetStoreShardByTaskList("domain1", "tl1", 0)
	s.NoError(err)
	s.Equal(1, len(store.connectedShards))
	s.True(mockDB1 == storeShard1.db)

	// Getting the same shard again shouldn't create a new connection
	storeShard1, err = store.GetStoreShardByTaskList("domain1", "tl1", 0)
	s.NoError(err)
	s.Equal(1, len(store.connectedShards))
	s.True(mockDB1 == storeShard1.db)

	// Getting a new shard should create a new connection
	storeShard2, err := store.GetStoreShardByTaskList("domain1", "tl2", 0)
	s.NoError(err)
	s.Equal(2, len(store.connectedShards))
	s.True(mockDB2 == storeShard2.db)

	// After the new connection, getting the previous shard should still work as it used to
	storeShard1, err = store.GetStoreShardByTaskList("domain1", "tl1", 0)
	s.NoError(err)
	s.Equal(2, len(store.connectedShards))
	s.True(mockDB1 == storeShard1.db)

	// Ensure the store shard connections created for tasklists are available for tasklists, too
	storeShard1, err = store.GetStoreShardByHistoryShard(0)
	s.NoError(err)
	s.Equal(2, len(store.connectedShards))
	s.True(mockDB1 == storeShard1.db)
	storeShard2, err = store.GetStoreShardByHistoryShard(1)
	s.NoError(err)
	s.Equal(2, len(store.connectedShards))
	s.True(mockDB2 == storeShard2.db)
}

func getValidShardedNoSQLConfig() ShardedNoSQL {
	return ShardedNoSQL{
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
	}
}
