package cli

import (
	cli "github.com/urfave/cli/v2"

	"github.com/uber/cadence/service/worker/batcher"
)

var batchStartCommonFlags = []cli.Flag{
	&cli.StringFlag{
		Name:    FlagListQuery,
		Aliases: []string{"q"},
		Usage:   "Query to get workflows for being executed this batch operation",
	},
	&cli.StringFlag{
		Name:    FlagReason,
		Aliases: []string{"re"},
		Usage:   "Reason to run this batch operation",
	},
	&cli.StringFlag{
		Name:  FlagBatchID,
		Usage: "Client-provided batch ID; retries with the same ID are deduplicated. Defaults to a random ID",
	},
	&cli.IntFlag{
		Name:  FlagRPS,
		Value: batcher.DefaultRPS,
		Usage: "RPS of processing",
	},
	&cli.IntFlag{
		Name:  FlagConcurrency,
		Value: batcher.DefaultConcurrency,
		Usage: "Concurrency of batch activity",
	},
	&cli.IntFlag{
		Name:  FlagPageSize,
		Value: batcher.DefaultPageSize,
		Usage: "PageSize of processing",
	},
	&cli.BoolFlag{
		Name:  FlagYes,
		Usage: "Optional flag to disable confirmation prompt",
	},
}

// batchStartFlags returns a fresh copy of batchStartCommonFlags plus extra,
// so appending here never mutates the shared batchStartCommonFlags backing array.
func batchStartFlags(extra ...cli.Flag) []cli.Flag {
	flags := make([]cli.Flag, 0, len(batchStartCommonFlags)+len(extra))
	flags = append(flags, batchStartCommonFlags...)
	flags = append(flags, extra...)
	return flags
}

// newBatchOperationCommands is distinct from newBatchCommands in workflow.go,
// the older "cadence workflow batch start --batch_type X" command.
func newBatchOperationCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:        "start",
			Usage:       "Start a batch operation; pick the subcommand matching the operation to run",
			Subcommands: newBatchStartCommands(),
		},
	}
}

func newBatchStartCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:  "terminate",
			Usage: "Terminate all workflows matching the query",
			Flags: batchStartCommonFlags,
			Action: func(c *cli.Context) error {
				return withBatchOperationsClient(c, func(bc *batchOperationsCLIImpl) error {
					return bc.StartTerminate(c)
				})
			},
		},
		{
			Name:  "cancel",
			Usage: "Request cancellation of all workflows matching the query",
			Flags: batchStartCommonFlags,
			Action: func(c *cli.Context) error {
				return withBatchOperationsClient(c, func(bc *batchOperationsCLIImpl) error {
					return bc.StartCancel(c)
				})
			},
		},
		{
			Name:  "signal",
			Usage: "Signal all workflows matching the query",
			Flags: batchStartFlags(
				&cli.StringFlag{
					Name:    FlagSignalName,
					Aliases: []string{"sn"},
					Usage:   "Required: name of the signal to send",
				},
				&cli.StringFlag{
					Name:    FlagInput,
					Aliases: []string{"in"},
					Usage:   "Optional input of signal",
				},
			),
			Action: func(c *cli.Context) error {
				return withBatchOperationsClient(c, func(bc *batchOperationsCLIImpl) error {
					return bc.StartSignal(c)
				})
			},
		},
		{
			Name:  "replicate",
			Usage: "Resend replication tasks for all workflows matching the query",
			Flags: batchStartFlags(
				&cli.StringFlag{
					Name:    FlagSourceCluster,
					Aliases: []string{"sc"},
					Usage:   "Required: source cluster to replicate from",
				},
				&cli.StringFlag{
					Name:    FlagTargetCluster,
					Aliases: []string{"tc"},
					Usage:   "Required: target cluster to replicate to",
				},
			),
			Action: func(c *cli.Context) error {
				return withBatchOperationsClient(c, func(bc *batchOperationsCLIImpl) error {
					return bc.StartReplicate(c)
				})
			},
		},
		{
			Name:  "refresh",
			Usage: "Refresh workflow tasks for all workflows matching the query",
			Flags: batchStartCommonFlags,
			Action: func(c *cli.Context) error {
				return withBatchOperationsClient(c, func(bc *batchOperationsCLIImpl) error {
					return bc.StartRefresh(c)
				})
			},
		},
	}
}
