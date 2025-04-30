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
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/sqlite"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
)

type ServerSuite struct {
	*require.Assertions
	suite.Suite

	logger log.Logger
}

func TestServerSuite(t *testing.T) {
	suite.Run(t, new(ServerSuite))
}

func (s *ServerSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.logger = testlogger.New(s.T())
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
		s.logger.Fatal("Config file corrupted.", tag.Error(err))
	}

	// set up sqlite persistence layer and apply schema to sqlite db
	testBase := pt.NewTestBaseWithSQL(s.T(), sqlite.GetTestClusterOption())
	cfg.Persistence = testBase.Config()
	testBase.Setup()

	s.T().Logf("config=\n%v\n", cfg.String())

	cfg.DynamicConfig.FileBased.Filepath = constructPathIfNeed(rootDir, cfg.DynamicConfig.FileBased.Filepath)

	if err := cfg.ValidateAndFillDefaults(); err != nil {
		s.logger.Fatal("config validation failed", tag.Error(err))
	}

	logger := testlogger.New(s.T())

	lifecycle := fxtest.NewLifecycle(s.T())

	var daemons []common.Daemon
	// Shard distributor should be tested separately
	distributorShortName := service.ShortName(service.ShardDistributor)
	services := slices.DeleteFunc(service.ShortNames(service.List),
		func(s string) bool {
			return s == distributorShortName
		})

	for _, svc := range services {
		server := newServer(svc, cfg, logger, dynamicconfig.NewNopClient())
		daemons = append(daemons, server)
		server.Start()
	}

	timer := time.NewTimer(time.Second * 10)

	<-timer.C
	s.NoError(lifecycle.Stop(context.Background()))
	for _, daemon := range daemons {
		daemon.Stop()
	}
}

func TestSettingGettingZonalIsolationGroupsFromIG(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := dynamicconfig.NewMockClient(ctrl)
	client.EXPECT().GetListValue(dynamicproperties.AllIsolationGroups, gomock.Any()).Return([]interface{}{
		"zone-1", "zone-2",
	}, nil)

	dc := dynamicconfig.NewCollection(client, log.NewNoop())

	assert.NotPanics(t, func() {
		fn := getFromDynamicConfig(resource.Params{
			Logger: log.NewNoop(),
		}, dc)
		out := fn()
		assert.Equal(t, []string{"zone-1", "zone-2"}, out)
	})
}

func TestSettingGettingZonalIsolationGroupsFromIGError(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := dynamicconfig.NewMockClient(ctrl)
	client.EXPECT().GetListValue(dynamicproperties.AllIsolationGroups, gomock.Any()).Return(nil, assert.AnError)
	dc := dynamicconfig.NewCollection(client, log.NewNoop())

	assert.NotPanics(t, func() {
		getFromDynamicConfig(resource.Params{
			Logger: log.NewNoop(),
		}, dc)()
	})
}
