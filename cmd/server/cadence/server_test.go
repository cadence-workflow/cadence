package cadence

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/configstore"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/sqlite"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"

	_ "github.com/ncruces/go-sqlite3/driver" // register sqlite3 driver for tests
	_ "github.com/ncruces/go-sqlite3/embed"  // embed sqlite db for tests
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
	metadata := cluster.GetTestClusterMetadata(true)
	testBase := pt.NewTestBase(s.T(), pt.TestBaseParams{
		PersistenceConfig: pt.SimplePersistenceConfig(s.T(), sqlite.GetTestConfig),
		ClusterMetadata:   &metadata,
	})
	cfg.Persistence = testBase.PersistenceConfig
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
		client := dynamicconfig.NewNopClient()
		dc := dynamicconfig.NewNopCollection()
		operationalConfigStore := configstore.NewNopClient()
		operationalDC := dynamicconfig.NewNopCollection()
		rpcParams, err := rpc.NewParams(service.FullName(svc), &cfg, dc, logger, metrics.NewNoopMetricsClient())
		s.NoError(err)
		rpcFactory := rpc.NewFactory(logger, rpcParams)
		// Use noop archival for tests - archival is tested separately
		archivalMetadata := archiver.NewArchivalMetadata(
			dc,
			"",
			false,
			"",
			false,
			&archiver.ArchivalDomainDefaults{},
		)
		archiverProvider := provider.NewNoOpArchiverProvider()
		ctrl := gomock.NewController(s.T())
		mockPeerProvider := membership.NewMockPeerProvider(ctrl)
		mockPeerProvider.EXPECT().Start(gomock.Any()).Return(nil).AnyTimes()
		mockPeerProvider.EXPECT().Stop(gomock.Any()).Return(nil).AnyTimes()
		mockPeerProvider.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		mockPeerProvider.EXPECT().WhoAmI().Return(membership.HostInfo{}, nil).AnyTimes()
		mockPeerProvider.EXPECT().GetMembers(gomock.Any()).Return([]membership.HostInfo{}, nil).AnyTimes()
		mockPeerProvider.EXPECT().SelfEvict().Return(nil).AnyTimes()
		server := newServer(svc, cfg, logger, testlogger.NewZap(s.T()), client, dc, operationalConfigStore, operationalDC, tally.NoopScope, metrics.NewNoopMetricsClient(), rpcFactory, mockPeerProvider, archivalMetadata, archiverProvider)
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
