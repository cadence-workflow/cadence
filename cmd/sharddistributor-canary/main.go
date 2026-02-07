package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/uber-go/tally"
	"github.com/uber-go/tally/prometheus"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/peer"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/zap"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/common/clock"
	cadenceconfig "github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/canary"
	"github.com/uber/cadence/service/sharddistributor/canary/executors"
	"github.com/uber/cadence/service/sharddistributor/canary/replay"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
	sdconfig "github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/tools/common/commoncli"
)

const (
	// Default configuration
	defaultShardDistributorEndpoint = "127.0.0.1:7943"
	defaultFixedNamespace           = "shard-distributor-canary"
	defaultEphemeralNamespace       = "shard-distributor-canary-ephemeral"
	defaultCanaryGRPCPort           = 7953 // Port for canary to receive ping requests
	defaultCanaryMetricsPort        = 9098
	defaultReplayNamespace          = "shard-distributor-replay"

	shardDistributorServiceName = "cadence-shard-distributor"
)

func runApp(c *cli.Context) {
	endpoint := c.String("endpoint")
	fixedNamespace := c.String("fixed-namespace")
	ephemeralNamespace := c.String("ephemeral-namespace")
	canaryGRPCPort := c.Int("canary-grpc-port")
	canaryMetricsPort := c.Int("canary-metrics-port")

	replayOpts := replay.Options{
		CSVPath:           c.String("replay-csv"),
		Speed:             c.Float64("replay-speed"),
		Namespace:         c.String("replay-namespace"),
		NumFixedExecutors: c.Int("replay-num-fixed-executors"),
	}

	fx.New(opts(fixedNamespace, ephemeralNamespace, endpoint, canaryGRPCPort, canaryMetricsPort, replayOpts)).Run()
}

func opts(fixedNamespace, ephemeralNamespace, endpoint string, canaryGRPCPort int, canaryMetricsPort int, replayOpts replay.Options) fx.Option {
	logger, _ := zap.NewDevelopment()
	cadenceLogger := log.NewLogger(logger)

	metricsConfig := cadenceconfig.Metrics{
		Prometheus: &prometheus.Configuration{
			ListenAddress: fmt.Sprintf("127.0.0.1:%d", canaryMetricsPort),
			TimerType:     "histogram",
		},
	}
	metricsScope := metricsConfig.NewScope(cadenceLogger, "shard-distributor-canary")

	if replayOpts.Namespace == "" {
		replayOpts.Namespace = defaultReplayNamespace
	}
	if replayOpts.NumFixedExecutors <= 0 {
		replayOpts.NumFixedExecutors = 3
	}

	configuration := clientcommon.Config{}
	if replayOpts.Enabled() {
		configuration.Namespaces = []clientcommon.NamespaceConfig{
			{Namespace: replayOpts.Namespace, HeartBeatInterval: 1 * time.Second, MigrationMode: sdconfig.MigrationModeONBOARDED},
		}
	} else {
		configuration.Namespaces = []clientcommon.NamespaceConfig{
			{Namespace: fixedNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: sdconfig.MigrationModeONBOARDED},
			{Namespace: ephemeralNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: sdconfig.MigrationModeONBOARDED},
			{Namespace: executors.LocalPassthroughNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: sdconfig.MigrationModeLOCALPASSTHROUGH},
			{Namespace: executors.LocalPassthroughShadowNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: sdconfig.MigrationModeLOCALPASSTHROUGHSHADOW},
			{Namespace: executors.DistributedPassthroughNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: sdconfig.MigrationModeDISTRIBUTEDPASSTHROUGH},
			{Namespace: executors.ExternalAssignmentNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: sdconfig.MigrationModeDISTRIBUTEDPASSTHROUGH},
		}
	}

	canaryGRPCAddress := fmt.Sprintf("127.0.0.1:%d", canaryGRPCPort)

	// Create listener for GRPC inbound
	listener, err := net.Listen("tcp", canaryGRPCAddress)
	if err != nil {
		panic(err)
	}

	transport := grpc.NewTransport()

	executorMetadata := executorclient.ExecutorMetadata{
		clientcommon.GrpcAddressMetadataKey: canaryGRPCAddress,
	}

	options := []fx.Option{
		fx.Supply(
			fx.Annotate(metricsScope, fx.As(new(tally.Scope))),
			fx.Annotate(clock.NewRealTimeSource(), fx.As(new(clock.TimeSource))),
			configuration,
			transport,
			executorMetadata,
			logger,
			replayOpts,
		),
	}

	if replayOpts.Enabled() {
		options = append(options, fx.Provide(func() yarpc.Config {
			return yarpc.Config{
				Name: "shard-distributor-canary",
				Inbounds: yarpc.Inbounds{
					transport.NewInbound(listener), // Listen for incoming ping requests
				},
				Outbounds: yarpc.Outbounds{
					shardDistributorServiceName: {
						Unary:  transport.NewSingleOutbound(endpoint),
						Stream: transport.NewSingleOutbound(endpoint),
					},
				},
			}
		}))
	} else {
		options = append(options, fx.Provide(func(peerChooser spectatorclient.SpectatorPeerChooserInterface) yarpc.Config {
			return yarpc.Config{
				Name: "shard-distributor-canary",
				Inbounds: yarpc.Inbounds{
					transport.NewInbound(listener), // Listen for incoming ping requests
				},
				Outbounds: yarpc.Outbounds{
					shardDistributorServiceName: {
						Unary:  transport.NewSingleOutbound(endpoint),
						Stream: transport.NewSingleOutbound(endpoint),
					},
					// canary-to-canary outbound uses peer chooser to route to other canary instances
					"shard-distributor-canary": {
						Unary:  transport.NewOutbound(peerChooser),
						Stream: transport.NewOutbound(peerChooser),
					},
				},
			}
		}))
	}

	options = append(options,
		fx.Provide(
			func(t *grpc.Transport) peer.Transport { return t },
		),
		fx.Provide(
			yarpc.NewDispatcher,
			func(d *yarpc.Dispatcher) yarpc.ClientConfig { return d }, // Reprovide the dispatcher as a client config
			func(l *zap.Logger) log.Logger { return log.NewLogger(l) },
		),

		// We do decorate instead of Invoke because we want to start and stop the dispatcher at the
		// correct time.
		// It will start before all dependencies are started and stop after all dependencies are stopped.
		// The Decorate gives fx enough information, so it can start and stop the dispatcher at the correct time.
		//
		// It is critical to start and stop the dispatcher at the correct time.
		// Since the executors need to
		// be able to send a final "drain" request to the shard distributor before the application is stopped.
		fx.Decorate(func(
			lc fx.Lifecycle,
			dispatcher *yarpc.Dispatcher,
			server sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCServer,
		) *yarpc.Dispatcher {
			// Register canary procedures and ensure dispatcher lifecycle is managed by fx.
			dispatcher.Register(sharddistributorv1.BuildShardDistributorExecutorCanaryAPIYARPCProcedures(server))
			lc.Append(fx.StartStopHook(dispatcher.Start, dispatcher.Stop))
			return dispatcher
		}),

		// Include the canary module - it will set up spectator peer choosers and canary client
		canary.ModuleWithReplay(
			canary.NamespacesNames{
				FixedNamespace:              fixedNamespace,
				EphemeralNamespace:          ephemeralNamespace,
				ExternalAssignmentNamespace: executors.ExternalAssignmentNamespace,
				SharddistributorServiceName: shardDistributorServiceName,
			},
			replayOpts,
		),
	)

	return fx.Options(options...)
}

func buildCLI() *cli.App {
	app := cli.NewApp()
	app.Name = "sharddistributor-canary"
	app.Usage = "Cadence shard distributor canary"
	app.Version = "0.0.1"

	app.Commands = []*cli.Command{
		{
			Name:  "start",
			Usage: "start shard distributor canary",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "endpoint",
					Aliases: []string{"e"},
					Value:   defaultShardDistributorEndpoint,
					Usage:   "shard distributor endpoint address",
				},
				&cli.StringFlag{
					Name:  "fixed-namespace",
					Value: defaultFixedNamespace,
					Usage: "namespace for fixed shard processing",
				},
				&cli.StringFlag{
					Name:  "ephemeral-namespace",
					Value: defaultEphemeralNamespace,
					Usage: "namespace for ephemeral shard creation testing",
				},
				&cli.IntFlag{
					Name:  "canary-grpc-port",
					Value: defaultCanaryGRPCPort,
					Usage: "port for canary to receive ping requests",
				},
				&cli.IntFlag{
					Name:  "canary-metrics-port",
					Value: defaultCanaryMetricsPort,
					Usage: "port for canary Prometheus metrics",
				},
				&cli.StringFlag{
					Name:  "replay-csv",
					Usage: "enable CSV load replay (path to CSV: timestamp,load0,...,loadN-1)",
				},
				&cli.StringFlag{
					Name:  "replay-namespace",
					Value: defaultReplayNamespace,
					Usage: "fixed namespace used for CSV replay",
				},
				&cli.IntFlag{
					Name:  "replay-num-fixed-executors",
					Value: 3,
					Usage: "number of fixed-namespace executors to run in-process during replay",
				},
				&cli.Float64Flag{
					Name:  "replay-speed",
					Value: 1.0,
					Usage: "CSV replay speed multiplier (timestamp-following mode)",
				},
			},
			Action: func(c *cli.Context) error {
				runApp(c)
				return nil
			},
		},
	}

	return app
}

func main() {
	app := buildCLI()
	commoncli.ExitHandler(app.Run(os.Args))
}
