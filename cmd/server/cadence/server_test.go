// Copyright (c) 2021 Uber Technologies, Inc.
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

//go:build !race
// +build !race

package cadence

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/loggerimpl"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/testflags"
	"github.com/uber/cadence/tools/cassandra"

	_ "github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra"              // needed to load cassandra plugin
	_ "github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql/public" // needed to load the default gocql client
)

type ServerSuite struct {
	*require.Assertions
	suite.Suite
}

func TestServerSuite(t *testing.T) {
	testflags.RequireCassandra(t)
	suite.Run(t, new(ServerSuite))
}

func (s *ServerSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

/*
TestServerStartup tests the startup logic for the binary. When this fails, you should be able to reproduce by running "cadence-server start"
If you need to run locally, make sure Cassandra is up and schema is installed(run `make install-schema`)
*/
func (s *ServerSuite) TestServerStartup() {
	env := "development"
	zone := ""
	rootDir := "../../../"
	configDir := constructPathIfNeed(rootDir, "config")

	s.T().Logf("Loading config; env=%v,zone=%v,configDir=%v\n", env, zone, configDir)

	var cfg config.Config
	err := config.Load(env, configDir, zone, &cfg)
	if err != nil {
		log.Fatal("Config file corrupted.", err)
	}

	if os.Getenv("CASSANDRA_SEEDS") == "cassandra" {
		// replace local host to docker network
		// this env variable value is set by buildkite's docker-compose
		ds := cfg.Persistence.DataStores[cfg.Persistence.DefaultStore]
		ds.NoSQL.Hosts = "cassandra"
		cfg.Persistence.DataStores[cfg.Persistence.DefaultStore] = ds

		ds = cfg.Persistence.DataStores[cfg.Persistence.VisibilityStore]
		ds.NoSQL.Hosts = "cassandra"
		cfg.Persistence.DataStores[cfg.Persistence.VisibilityStore] = ds
	}

	s.T().Logf("config=\n%v\n", cfg.String())

	cfg.DynamicConfig.FileBased.Filepath = constructPathIfNeed(rootDir, cfg.DynamicConfig.FileBased.Filepath)

	if err := cfg.ValidateAndFillDefaults(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}
	// cassandra schema version validation
	if err := cassandra.VerifyCompatibleVersion(cfg.Persistence, gocql.All); err != nil {
		log.Fatal("cassandra schema version compatibility check failed: ", err)
	}

	var daemons []common.Daemon
	services := service.ShortNames(service.List)
	for _, svc := range services {
		server := newServer(svc, &cfg)
		daemons = append(daemons, server)
		server.Start()
	}

	timer := time.NewTimer(time.Second * 10)

	<-timer.C
	for _, daemon := range daemons {
		daemon.Stop()
	}
}

func TestSettingGettingZonalIsolationGroupsFromIG(t *testing.T) {

	ctrl := gomock.NewController(t)
	client := dynamicconfig.NewMockClient(ctrl)
	client.EXPECT().GetListValue(dynamicconfig.AllIsolationGroups, gomock.Any()).Return([]interface{}{
		"zone-1", "zone-2",
	}, nil)

	dc := dynamicconfig.NewCollection(client, loggerimpl.NewNopLogger())

	assert.NotPanics(t, func() {
		fn := getFromDynamicConfig(resource.Params{
			Logger: loggerimpl.NewNopLogger(),
		}, dc)
		out := fn()
		assert.Equal(t, []string{"zone-1", "zone-2"}, out)
	})
}

func TestSettingGettingZonalIsolationGroupsFromIGError(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := dynamicconfig.NewMockClient(ctrl)
	client.EXPECT().GetListValue(dynamicconfig.AllIsolationGroups, gomock.Any()).Return(nil, assert.AnError)
	dc := dynamicconfig.NewCollection(client, loggerimpl.NewNopLogger())

	assert.NotPanics(t, func() {
		getFromDynamicConfig(resource.Params{
			Logger: loggerimpl.NewNopLogger(),
		}, dc)()
	})
}
