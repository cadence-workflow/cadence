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

package cli

import (
	"fmt"
	"strings"

	"github.com/stretchr/testify/mock"
	"github.com/uber-go/tally"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/tools/common/flag"
)

const (
	dependencyMaxQPS = 100
)

var (
	registerDomainFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    FlagDescription,
			Aliases: []string{"desc"},
			Usage:   "Domain description",
		},
		&cli.StringFlag{
			Name:    FlagOwnerEmail,
			Aliases: []string{"oe"},
			Usage:   "Owner email",
		},
		&cli.StringFlag{
			Name:    FlagRetentionDays,
			Aliases: []string{"rd"},
			Usage:   "Workflow execution retention in days",
		},
		&cli.StringFlag{
			Name:    FlagActiveClusterName,
			Aliases: []string{"ac"},
			Usage:   "Active cluster name",
		},
		&cli.StringSliceFlag{
			Name:    FlagActiveClusters,
			Aliases: []string{"acs"},
			Usage:   "Active clusters by cluster attribute in the format '<cluster-attr>.<scope>:<name> ie: region.manilla:cluster0,region.newyork:cluster1'",
		},
		&cli.StringSliceFlag{
			Name:    FlagClusters,
			Aliases: []string{"cl"},
			Usage:   FlagClustersUsage,
		},
		&cli.StringFlag{
			Name:    FlagIsGlobalDomain,
			Aliases: []string{"gd"},
			Usage:   "Flag to indicate whether domain is a global domain (active-passive domain). Default to true. Local domain is now legacy.",
			Value:   "true",
		},
		&cli.GenericFlag{
			Name:    FlagDomainData,
			Aliases: []string{"dmd"},
			Usage:   "Domain data of key value pairs (must be in key1=value1,key2=value2,...,keyN=valueN format, e.g. cluster=dca or cluster=dca,instance=cadence)",
			Value:   &flag.StringMap{},
		},
		&cli.StringFlag{
			Name:    FlagSecurityToken,
			Aliases: []string{"st"},
			Usage:   "Optional token for security check",
		},
		&cli.StringFlag{
			Name:    FlagHistoryArchivalStatus,
			Aliases: []string{"has"},
			Usage:   "Flag to set history archival status, valid values are \"disabled\" and \"enabled\"",
		},
		&cli.StringFlag{
			Name:    FlagHistoryArchivalURI,
			Aliases: []string{"huri"},
			Usage:   "Optionally specify history archival URI (cannot be changed after first time archival is enabled)",
		},
		&cli.StringFlag{
			Name:    FlagVisibilityArchivalStatus,
			Aliases: []string{"vas"},
			Usage:   "Flag to set visibility archival status, valid values are \"disabled\" and \"enabled\"",
		},
		&cli.StringFlag{
			Name:    FlagVisibilityArchivalURI,
			Aliases: []string{"vuri"},
			Usage:   "Optionally specify visibility archival URI (cannot be changed after first time archival is enabled)",
		},
	}

	updateDomainFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    FlagDescription,
			Aliases: []string{"desc"},
			Usage:   "Domain description",
		},
		&cli.StringFlag{
			Name:    FlagOwnerEmail,
			Aliases: []string{"oe"},
			Usage:   "Owner email",
		},
		&cli.StringFlag{
			Name:    FlagRetentionDays,
			Aliases: []string{"rd"},
			Usage:   "Workflow execution retention in days",
		},
		&cli.StringFlag{
			Name:    FlagActiveClusterName,
			Aliases: []string{"ac"},
			Usage:   "Active cluster name",
		},
		&cli.StringSliceFlag{
			Name:    FlagActiveClusters,
			Aliases: []string{"acs"},
			Usage:   "Active clusters by cluster attribute in the format '<cluster-attr>.<scope>:<name> ie: region.manilla:cluster0,region.newyork:cluster1'",
		},
		&cli.StringSliceFlag{
			Name:    FlagClusters,
			Aliases: []string{"cl"},
			Usage:   FlagClustersUsage,
		},
		&cli.GenericFlag{
			Name:  FlagDomainData,
			Usage: "Domain data of key value pairs (must be in key1=value1,key2=value2,...,keyN=valueN format, e.g. cluster=dca or cluster=dca,instance=cadence)",
			Value: &flag.StringMap{},
		},
		&cli.StringFlag{
			Name:    FlagSecurityToken,
			Aliases: []string{"st"},
			Usage:   "Optional token for security check",
		},
		&cli.StringFlag{
			Name:    FlagHistoryArchivalStatus,
			Aliases: []string{"has"},
			Usage:   "Flag to set history archival status, valid values are \"disabled\" and \"enabled\"",
		},
		&cli.StringFlag{
			Name:    FlagHistoryArchivalURI,
			Aliases: []string{"huri"},
			Usage:   "Optionally specify history archival URI (cannot be changed after first time archival is enabled)",
		},
		&cli.StringFlag{
			Name:    FlagVisibilityArchivalStatus,
			Aliases: []string{"vas"},
			Usage:   "Flag to set visibility archival status, valid values are \"disabled\" and \"enabled\"",
		},
		&cli.StringFlag{
			Name:    FlagVisibilityArchivalURI,
			Aliases: []string{"vuri"},
			Usage:   "Optionally specify visibility archival URI (cannot be changed after first time archival is enabled)",
		},
		&cli.StringFlag{
			Name:  FlagAddBadBinary,
			Usage: "Binary checksum to add for resetting workflow",
		},
		&cli.StringFlag{
			Name:  FlagRemoveBadBinary,
			Usage: "Binary checksum to remove for resetting workflow",
		},
		&cli.StringFlag{
			Name:  FlagReason,
			Usage: "Reason for the operation",
		},
		&cli.StringFlag{
			Name:    FlagFailoverType,
			Aliases: []string{"ft"},
			Usage:   "Domain failover type. Default value: force. Options: [force,grace]",
		},
		&cli.IntFlag{
			Name:    FlagFailoverTimeout,
			Aliases: []string{"fts"},
			Value:   defaultGracefulFailoverTimeoutInSeconds,
			Usage:   "[Optional] Domain failover timeout in seconds.",
		},
	}

	deleteDomainFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    FlagSecurityToken,
			Aliases: []string{"st"},
			Usage:   "Optional token for security check",
		},
	}

	deprecateDomainFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    FlagSecurityToken,
			Aliases: []string{"st"},
			Usage:   "Optional token for security check",
		},
		&cli.BoolFlag{
			Name:  FlagForce,
			Usage: "Deprecate domain regardless of domain history.",
		},
	}

	describeDomainFlags = []cli.Flag{
		&cli.StringFlag{
			Name:  FlagDomainID,
			Usage: "Domain UUID (required if not specify domainName)",
		},
		&cli.BoolFlag{
			Name:    FlagPrintJSON,
			Aliases: []string{"pjson"},
			Usage:   "Print in raw JSON format",
		},
		getFormatFlag(),
	}

	migrateDomainFlags = []cli.Flag{

		&cli.StringFlag{
			Name:  FlagDestinationAddress,
			Usage: "Destination cadence-frontend address in <host>:<port> format",
		},
		&cli.StringFlag{
			Name:  FlagDestinationDomain,
			Usage: "Destination domain name",
		},

		&cli.StringSliceFlag{
			Name:  FlagTaskList,
			Usage: "All tasklists in the current domain",
		},

		&cli.StringSliceFlag{
			Name:  FlagSearchAttribute,
			Usage: "Specify search attributes in the format key:type, available types are STRING, KEYWORD, INT, DOUBLE, BOOL, DATETIME",
		},

		getFormatFlag(),
	}

	failoverDomainFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    FlagActiveClusterName,
			Aliases: []string{"ac"},
			Usage:   "Active cluster name",
		},
		&cli.StringSliceFlag{
			Name:    FlagActiveClusters,
			Aliases: []string{"acs"},
			Usage:   "Active clusters by cluster attribute in the format '<cluster-attr>.<scope>:<name> ie: region.manilla:cluster0,region.newyork:cluster1'",
		},
		&cli.StringFlag{
			Name:    FlagActiveClustersJSON,
			Aliases: []string{"acs-json"},
			Usage:   `Active clusters by cluster attribute in JSON format. Eg {"attributeScopes":{"region-us-east1":{"clusterAttributes":{"new-york":{"activeClusterName":"cluster1"}}}}}`,
		},
		&cli.StringFlag{
			Name:    FlagFailoverReason,
			Aliases: []string{"r"},
			Usage:   "Reason for failover (for tracking and transparency)",
		},
	}

	listFailoverHistoryFlags = []cli.Flag{
		&cli.BoolFlag{
			Name:    FlagAll,
			Aliases: []string{"a"},
			Usage:   "List all failover history events",
		},
		&cli.BoolFlag{
			Name:    FlagPrintJSON,
			Aliases: []string{"pjson"},
			Usage:   "Print in raw JSON format",
		},
		getFormatFlag(),
	}

	adminDomainCommonFlags = getDBFlags()

	adminRegisterDomainFlags = append(
		registerDomainFlags,
		adminDomainCommonFlags...,
	)

	adminUpdateDomainFlags = append(
		updateDomainFlags,
		adminDomainCommonFlags...,
	)

	adminDeleteDomainFlags = append(
		deleteDomainFlags,
		adminDomainCommonFlags...,
	)

	adminDeprecateDomainFlags = append(
		deprecateDomainFlags,
		adminDomainCommonFlags...,
	)

	adminDescribeDomainFlags = append(
		updateDomainFlags,
		adminDomainCommonFlags...,
	)
)

func initializeFrontendClient(c *cli.Context) (frontend.Client, error) {
	return getDeps(c).ServerFrontendClient(c)
}

func initializeFrontendAdminClient(c *cli.Context) (admin.Client, error) {
	return getDeps(c).ServerAdminClient(c)
}

func initializeAdminDomainHandler(c *cli.Context) (domain.Handler, error) {

	configuration, err := getDeps(c).ServerConfig(c)
	if err != nil {
		return nil, err
	}
	metricsClient := initializeMetricsClient()
	logger, err := initializeLogger(configuration)
	if err != nil {
		return nil, fmt.Errorf("Error in init admin domain handler: %w", err)
	}
	clusterMetadata := initializeClusterMetadata(configuration, metricsClient, logger)
	metadataMgr, err := getDeps(c).initializeDomainManager(c)
	if err != nil {
		return nil, fmt.Errorf("Error in init admin domain handler: %w", err)
	}
	dynamicConfig, err := initializeDynamicConfig(configuration, logger)
	if err != nil {
		return nil, fmt.Errorf("Error in init admin domain handler: %w", err)
	}
	archivalprovider, err := initializeArchivalProvider(configuration, clusterMetadata, metricsClient, logger)
	if err != nil {
		return nil, fmt.Errorf("Error in init admin domain handler: %w", err)
	}
	domainhandler := initializeDomainHandler(
		logger,
		metadataMgr,
		clusterMetadata,
		initializeArchivalMetadata(configuration, dynamicConfig),
		archivalprovider,
	)
	return domainhandler, nil
}

func loadConfig(
	context *cli.Context,
) (*config.Config, error) {
	env := getEnvironment(context)
	zone := getZone(context)
	configDir, err := getConfigDir(context)
	if err != nil {
		return nil, fmt.Errorf("Unable to load config. %w", err)
	}
	var cfg config.Config
	err = config.Load(env, configDir, zone, &cfg)
	if err != nil {
		return nil, fmt.Errorf("Unable to load config. %w", err)
	}
	return &cfg, nil
}

func initializeDomainHandler(
	logger log.Logger,
	domainManager persistence.DomainManager,
	clusterMetadata cluster.Metadata,
	archivalMetadata archiver.ArchivalMetadata,
	archiverProvider provider.ArchiverProvider,
) domain.Handler {

	domainConfig := domain.Config{
		MinRetentionDays:  dynamicproperties.GetIntPropertyFn(dynamicproperties.MinRetentionDays.DefaultInt()),
		MaxBadBinaryCount: dynamicproperties.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendMaxBadBinaries.DefaultInt()),
		FailoverCoolDown:  dynamicproperties.GetDurationPropertyFnFilteredByDomain(dynamicproperties.FrontendFailoverCoolDown.DefaultDuration()),
	}
	return domain.NewHandler(
		domainConfig,
		logger,
		domainManager,
		nil, // domainAuditManager not needed for CLI tools
		clusterMetadata,
		initializeDomainReplicator(logger),
		archivalMetadata,
		archiverProvider,
		clock.NewRealTimeSource(),
	)
}

func initializeLogger(
	serviceConfig *config.Config,
) (log.Logger, error) {
	zapLogger, err := serviceConfig.Log.NewZapLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to create zap logger, err: %w", err)
	}
	return log.NewLogger(zapLogger), nil
}

func initializeClusterMetadata(serviceConfig *config.Config, metrics metrics.Client, logger log.Logger) cluster.Metadata {
	clusterGroupMetadata := serviceConfig.ClusterGroupMetadata
	return cluster.NewMetadata(
		*clusterGroupMetadata,
		func(d string) bool { return false },
		metrics,
		logger,
	)
}

func initializeArchivalMetadata(
	serviceConfig *config.Config,
	dynamicConfig *dynamicconfig.Collection,
) archiver.ArchivalMetadata {

	return archiver.NewArchivalMetadata(
		dynamicConfig,
		serviceConfig.Archival.History.Status,
		serviceConfig.Archival.History.EnableRead,
		serviceConfig.Archival.Visibility.Status,
		serviceConfig.Archival.Visibility.EnableRead,
		&serviceConfig.DomainDefaults.Archival,
	)
}

func initializeArchivalProvider(
	serviceConfig *config.Config,
	clusterMetadata cluster.Metadata,
	metricsClient metrics.Client,
	logger log.Logger,
) (provider.ArchiverProvider, error) {

	archiverProvider := provider.NewArchiverProvider(
		serviceConfig.Archival.History.Provider,
		serviceConfig.Archival.Visibility.Provider,
	)

	historyArchiverBootstrapContainer := &archiver.HistoryBootstrapContainer{
		HistoryV2Manager: nil, // not used
		Logger:           logger,
		MetricsClient:    metricsClient,
		ClusterMetadata:  clusterMetadata,
		DomainCache:      nil, // not used
	}
	visibilityArchiverBootstrapContainer := &archiver.VisibilityBootstrapContainer{
		Logger:          logger,
		MetricsClient:   metricsClient,
		ClusterMetadata: clusterMetadata,
		DomainCache:     nil, // not used
	}

	err := archiverProvider.RegisterBootstrapContainer(
		service.Frontend,
		historyArchiverBootstrapContainer,
		visibilityArchiverBootstrapContainer,
	)
	if err != nil {
		return nil, fmt.Errorf("Error initializing archival provider. %w", err)
	}
	return archiverProvider, nil
}

func initializeDomainReplicator(
	logger log.Logger,
) domain.Replicator {

	replicationMessageSink := &mocks.KafkaProducer{}
	replicationMessageSink.On("Publish", mock.Anything, mock.Anything).Return(nil)
	return domain.NewDomainReplicator(replicationMessageSink, logger)
}

func initializeDynamicConfig(
	serviceConfig *config.Config,
	logger log.Logger,
) (*dynamicconfig.Collection, error) {

	// the done channel is used by dynamic config to stop refreshing
	// and CLI does not need that, so just close the done channel
	doneChan := make(chan struct{})
	close(doneChan)
	dynamicConfigClient, err := dynamicconfig.NewFileBasedClient(
		&serviceConfig.DynamicConfig.FileBased,
		logger,
		doneChan,
	)
	if err != nil {
		return nil, fmt.Errorf("Error initializing dynamic config. %w", err)
	}
	return dynamicconfig.NewCollection(dynamicConfigClient, logger), nil
}

func initializeMetricsClient() metrics.Client {
	return metrics.NewClient(tally.NoopScope, metrics.Common, metrics.HistogramMigration{})
}

func getEnvironment(c *cli.Context) string {
	return strings.TrimSpace(c.String(FlagServiceEnv))
}

func getZone(c *cli.Context) string {
	return strings.TrimSpace(c.String(FlagServiceZone))
}

func getConfigDir(c *cli.Context) (string, error) {
	dirPath := c.String(FlagServiceConfigDir)
	if len(dirPath) == 0 {
		return "", fmt.Errorf("Must provide service configuration dir path. %v", nil)
	}
	return dirPath, nil
}
