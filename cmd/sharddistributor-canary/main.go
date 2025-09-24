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
	"github.com/uber/cadence/service/sharddistributor/executorclient"
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
	config := executorclient.Config{
		Namespaces: []executorclient.NamespaceConfig{
			{Namespace: fixedNamespace, HeartBeatInterval: 1 * time.Second},
			{Namespace: ephemeralNamespace, HeartBeatInterval: 1 * time.Second},
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
			config,
		),
		fx.Provide(
			yarpc.NewDispatcher,
			func(d *yarpc.Dispatcher) yarpc.ClientConfig { return d }, // Reprovide the dispatcher as a client config
		),
		fx.Provide(zap.NewDevelopment),
		fx.Provide(log.NewLogger),

		// Start the YARPC dispatcher
		fx.Invoke(func(lc fx.Lifecycle, dispatcher *yarpc.Dispatcher) {
			lc.Append(fx.StartStopHook(dispatcher.Start, dispatcher.Stop))
		}),

		// Include the canary module
		canary.Module(fixedNamespace, ephemeralNamespace, shardDistributorServiceName),
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
