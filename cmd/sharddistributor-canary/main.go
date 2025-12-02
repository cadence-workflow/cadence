package main

import (
	"os"
	"time"

	"github.com/uber-go/tally"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/canary"
	"github.com/uber/cadence/service/sharddistributor/canary/executors"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/tools/common/commoncli"
)

const (
	// Default configuration
	defaultShardDistributorEndpoint = "127.0.0.1:7943"
	defaultFixedNamespace           = "shard-distributor-canary"
	defaultEphemeralNamespace       = "shard-distributor-canary-ephemeral"

	shardDistributorServiceName = "cadence-shard-distributor"
)

func runApp(c *cli.Context) {
	endpoint := c.String("endpoint")
	fixedNamespace := c.String("fixed-namespace")
	ephemeralNamespace := c.String("ephemeral-namespace")

	fx.New(opts(fixedNamespace, ephemeralNamespace, endpoint)).Run()
}

func opts(fixedNamespace, ephemeralNamespace, endpoint string) fx.Option {
	configuration := clientcommon.Config{
		Namespaces: []clientcommon.NamespaceConfig{
			{Namespace: fixedNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: config.MigrationModeONBOARDED},
			{Namespace: ephemeralNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: config.MigrationModeONBOARDED},
			{Namespace: executors.LocalPassthroughNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: config.MigrationModeLOCALPASSTHROUGH},
			{Namespace: executors.LocalPassthroughShadowNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: config.MigrationModeLOCALPASSTHROUGHSHADOW},
			{Namespace: executors.DistributedPassthroughNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: config.MigrationModeDISTRIBUTEDPASSTHROUGH},
			{Namespace: executors.ExternalAssignmentNamespace, HeartBeatInterval: 1 * time.Second, MigrationMode: config.MigrationModeDISTRIBUTEDPASSTHROUGH},
		},
	}

	transport := grpc.NewTransport()
	yarpcConfig := yarpc.Config{
		Name: "shard-distributor-canary",
		Outbounds: yarpc.Outbounds{
			shardDistributorServiceName: {
				Unary: transport.NewSingleOutbound(endpoint),
			},
		},
	}

	return fx.Options(
		fx.Supply(
			fx.Annotate(tally.NoopScope, fx.As(new(tally.Scope))),
			fx.Annotate(clock.NewRealTimeSource(), fx.As(new(clock.TimeSource))),
			yarpcConfig,
			configuration,
		),
		fx.Provide(
			yarpc.NewDispatcher,
			func(d *yarpc.Dispatcher) yarpc.ClientConfig { return d }, // Reprovide the dispatcher as a client config
		),
		fx.Provide(zap.NewDevelopment),
		fx.Provide(log.NewLogger),

		// We do decorate instead of Invoke because we want to start and stop the dispatcher at the
		// correct time.
		// It will start before all dependencies are started and stop after all dependencies are stopped.
		// The Decorate gives fx enough information, so it can start and stop the dispatcher at the correct time.
		//
		// It is critical to start and stop the dispatcher at the correct time.
		// Since the executors need to
		// be able to send a final "drain" request to the shard distributor before the application is stopped.
		fx.Decorate(func(lc fx.Lifecycle, dispatcher *yarpc.Dispatcher) *yarpc.Dispatcher {
			lc.Append(fx.StartStopHook(dispatcher.Start, dispatcher.Stop))
			return dispatcher
		}),

		// Include the canary module
		canary.Module(canary.NamespacesNames{FixedNamespace: fixedNamespace, EphemeralNamespace: ephemeralNamespace, ExternalAssignmentNamespace: executors.ExternalAssignmentNamespace, SharddistributorServiceName: shardDistributorServiceName}),
	)
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
