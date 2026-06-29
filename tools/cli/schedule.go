// Copyright (c) 2026 Uber Technologies, Inc.
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

import cli "github.com/urfave/cli/v2"

var (
	scheduleIDFlag = &cli.StringFlag{
		Name:     FlagScheduleID,
		Aliases:  []string{"sid"},
		Usage:    "Schedule ID",
		Required: true,
	}

	createScheduleFlags = []cli.Flag{
		scheduleIDFlag,
		&cli.StringFlag{
			Name:     FlagCronExpression,
			Aliases:  []string{"ce"},
			Usage:    "Cron expression for the schedule (e.g. '*/5 * * * *')",
			Required: true,
		},
		&cli.StringFlag{
			Name:     FlagWorkflowType,
			Aliases:  []string{"wt"},
			Usage:    "Target workflow type name",
			Required: true,
		},
		&cli.StringFlag{
			Name:    FlagTaskList,
			Aliases: []string{"tl"},
			Usage:   "Target workflow task list",
		},
		&cli.IntFlag{
			Name:    FlagExecutionTimeout,
			Aliases: []string{"et"},
			Usage:   "Target workflow execution timeout in seconds",
			Value:   3600,
		},
		&cli.IntFlag{
			Name:    FlagDecisionTimeout,
			Aliases: []string{"dt"},
			Usage:   "Target workflow decision timeout in seconds",
			Value:   10,
		},
		&cli.StringFlag{
			Name:    FlagInput,
			Aliases: []string{"i"},
			Usage:   "Target workflow input (JSON string)",
		},
		// spec extras
		&cli.StringFlag{
			Name:    FlagStartTime,
			Aliases: []string{"st"},
			Usage:   "Earliest time the schedule should trigger (RFC3339, e.g. '2024-01-01T00:00:00Z')",
		},
		&cli.StringFlag{
			Name:    FlagEndTime,
			Aliases: []string{"endt"},
			Usage:   "Latest time the schedule should trigger (RFC3339)",
		},
		&cli.StringFlag{
			Name:  FlagJitter,
			Usage: "Random jitter applied to each trigger time (e.g. '30s', '5m')",
		},
		// action extras
		&cli.StringFlag{
			Name:  FlagWorkflowIDPrefix,
			Usage: "Prefix for generated target workflow IDs",
		},
		&cli.StringFlag{
			Name:  FlagMemoKey,
			Usage: "Memo key(s) to attach to the target workflow, space-separated",
		},
		&cli.StringFlag{
			Name:  FlagMemo,
			Usage: "Memo value(s) as JSON, space-separated (paired with --memo_key)",
		},
		&cli.StringFlag{
			Name:  FlagMemoFile,
			Usage: "Path to file containing memo values as JSON",
		},
		&cli.StringFlag{
			Name:  FlagSearchAttributesKey,
			Usage: "Search attribute key(s), pipe-separated",
		},
		&cli.StringFlag{
			Name:  FlagSearchAttributesVal,
			Usage: "Search attribute value(s), pipe-separated (paired with --search_attr_key)",
		},
		&cli.IntFlag{
			Name:  FlagRetryAttempts,
			Usage: "Max retry attempts for the target workflow (0 = unlimited)",
		},
		&cli.IntFlag{
			Name:  FlagRetryInterval,
			Usage: "Initial retry interval in seconds",
			Value: 1,
		},
		&cli.IntFlag{
			Name:  FlagRetryExpiration,
			Usage: "Max total retry time in seconds",
		},
		&cli.Float64Flag{
			Name:  FlagRetryBackoff,
			Usage: "Retry backoff coefficient",
			Value: 2.0,
		},
		&cli.IntFlag{
			Name:  FlagRetryMaxInterval,
			Usage: "Max retry interval in seconds",
		},
		// policy flags
		&cli.StringFlag{
			Name:  FlagOverlapPolicy,
			Usage: "Overlap policy: SkipNew, Buffer, Concurrent, CancelPrevious, TerminatePrevious",
		},
		&cli.IntFlag{
			Name:  FlagConcurrencyLimit,
			Usage: "Max concurrent target workflows (only effective with --overlap_policy concurrent; 0 = unlimited)",
		},
		&cli.StringFlag{
			Name:  FlagCatchUpPolicy,
			Usage: "Catch-up policy: Skip, One, All",
		},
		&cli.StringFlag{
			Name:  FlagCatchUpWindow,
			Usage: "Max catch-up window for missed runs (e.g. '1h', '30m')",
		},
		&cli.BoolFlag{
			Name:  FlagPauseOnFailure,
			Usage: "Pause the schedule when a triggered workflow fails",
		},
		&cli.IntFlag{
			Name:  FlagBufferLimit,
			Usage: "Max buffered runs (only with --overlap_policy buffer; 0 = unlimited)",
		},
	}

	describeScheduleFlags = []cli.Flag{
		scheduleIDFlag,
		&cli.BoolFlag{
			Name:    FlagPrintJSON,
			Aliases: []string{"pjson"},
			Usage:   "Print output in JSON format",
		},
	}

	updateScheduleFlags = []cli.Flag{
		scheduleIDFlag,
		&cli.StringFlag{
			Name:    FlagCronExpression,
			Aliases: []string{"ce"},
			Usage:   "New cron expression",
		},
		// spec extras
		&cli.StringFlag{
			Name:    FlagStartTime,
			Aliases: []string{"st"},
			Usage:   "New schedule start time (RFC3339)",
		},
		&cli.StringFlag{
			Name:    FlagEndTime,
			Aliases: []string{"endt"},
			Usage:   "New schedule end time (RFC3339)",
		},
		&cli.StringFlag{
			Name:  FlagJitter,
			Usage: "New jitter (e.g. '30s', '5m')",
		},
		// action extras
		&cli.StringFlag{
			Name:  FlagWorkflowIDPrefix,
			Usage: "New prefix for generated target workflow IDs",
		},
		&cli.StringFlag{
			Name:  FlagMemoKey,
			Usage: "Memo key(s), space-separated",
		},
		&cli.StringFlag{
			Name:  FlagMemo,
			Usage: "Memo value(s) as JSON, space-separated",
		},
		&cli.StringFlag{
			Name:  FlagMemoFile,
			Usage: "Path to file containing memo values as JSON",
		},
		&cli.StringFlag{
			Name:  FlagSearchAttributesKey,
			Usage: "Search attribute key(s), pipe-separated",
		},
		&cli.StringFlag{
			Name:  FlagSearchAttributesVal,
			Usage: "Search attribute value(s), pipe-separated",
		},
		&cli.IntFlag{
			Name:  FlagRetryAttempts,
			Usage: "New max retry attempts (0 = unlimited)",
		},
		&cli.IntFlag{
			Name:  FlagRetryInterval,
			Usage: "New initial retry interval in seconds",
			Value: 1,
		},
		&cli.IntFlag{
			Name:  FlagRetryExpiration,
			Usage: "New max total retry time in seconds",
		},
		&cli.Float64Flag{
			Name:  FlagRetryBackoff,
			Usage: "New retry backoff coefficient",
			Value: 2.0,
		},
		&cli.IntFlag{
			Name:  FlagRetryMaxInterval,
			Usage: "New max retry interval in seconds",
		},
		// policy flags
		&cli.StringFlag{
			Name:  FlagOverlapPolicy,
			Usage: "New overlap policy: SkipNew, Buffer, Concurrent, CancelPrevious, TerminatePrevious",
		},
		&cli.IntFlag{
			Name:  FlagConcurrencyLimit,
			Usage: "Max concurrent target workflows (only effective with --overlap_policy concurrent; 0 = unlimited)",
		},
		&cli.StringFlag{
			Name:  FlagCatchUpPolicy,
			Usage: "New catch-up policy: Skip, One, All",
		},
		&cli.StringFlag{
			Name:  FlagCatchUpWindow,
			Usage: "New catch-up window (e.g. '1h', '30m')",
		},
		&cli.BoolFlag{
			Name:  FlagPauseOnFailure,
			Usage: "Pause the schedule when a triggered workflow fails",
		},
		&cli.IntFlag{
			Name:  FlagBufferLimit,
			Usage: "New max buffered runs (only with --overlap_policy buffer; 0 = unlimited)",
		},
	}

	pauseScheduleFlags = []cli.Flag{
		scheduleIDFlag,
		&cli.StringFlag{
			Name:    FlagReason,
			Aliases: []string{"re"},
			Usage:   "Reason for pausing",
		},
	}

	unpauseScheduleFlags = []cli.Flag{
		scheduleIDFlag,
		&cli.StringFlag{
			Name:    FlagReason,
			Aliases: []string{"re"},
			Usage:   "Reason for unpausing",
		},
		&cli.StringFlag{
			Name:  FlagCatchUpPolicy,
			Usage: "Override catch-up policy for this unpause: Skip, One, All",
		},
	}

	backfillScheduleFlags = []cli.Flag{
		scheduleIDFlag,
		&cli.StringFlag{
			Name:     FlagStartTime,
			Aliases:  []string{"st"},
			Usage:    "Backfill start time (RFC3339, e.g. '2024-01-01T00:00:00Z')",
			Required: true,
		},
		&cli.StringFlag{
			Name:     FlagEndTime,
			Aliases:  []string{"endt"},
			Usage:    "Backfill end time (RFC3339, e.g. '2024-01-02T00:00:00Z')",
			Required: true,
		},
		&cli.StringFlag{
			Name:  FlagOverlapPolicy,
			Usage: "Overlap policy for backfill: SkipNew, Buffer, Concurrent, CancelPrevious, TerminatePrevious",
		},
		&cli.StringFlag{
			Name:  FlagBackfillID,
			Usage: "Backfill identifier for idempotency and tracking",
		},
	}

	deleteScheduleFlags = []cli.Flag{
		scheduleIDFlag,
	}

	listScheduleFlags = []cli.Flag{
		&cli.IntFlag{
			Name:    FlagPageSize,
			Aliases: []string{"ps"},
			Usage:   "Page size for listing",
			Value:   10,
		},
	}
)

func newScheduleCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "create",
			Aliases: []string{"c"},
			Usage:   "Create a new schedule",
			Flags:   createScheduleFlags,
			Action: func(c *cli.Context) error {
				if err := checkNoAdditionalArgsPassed(c); err != nil {
					return err
				}
				return withScheduleClient(c, func(sc *scheduleCLIImpl) error {
					return sc.CreateSchedule(c)
				})
			},
		},
		{
			Name:    "describe",
			Aliases: []string{"desc"},
			Usage:   "Describe an existing schedule",
			Flags:   describeScheduleFlags,
			Action: func(c *cli.Context) error {
				if err := checkNoAdditionalArgsPassed(c); err != nil {
					return err
				}
				return withScheduleClient(c, func(sc *scheduleCLIImpl) error {
					return sc.DescribeSchedule(c)
				})
			},
		},
		{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "Update an existing schedule",
			Flags:   updateScheduleFlags,
			Action: func(c *cli.Context) error {
				if err := checkNoAdditionalArgsPassed(c); err != nil {
					return err
				}
				return withScheduleClient(c, func(sc *scheduleCLIImpl) error {
					return sc.UpdateSchedule(c)
				})
			},
		},
		{
			Name:    "delete",
			Aliases: []string{"del"},
			Usage:   "Delete a schedule",
			Flags:   deleteScheduleFlags,
			Action: func(c *cli.Context) error {
				if err := checkNoAdditionalArgsPassed(c); err != nil {
					return err
				}
				return withScheduleClient(c, func(sc *scheduleCLIImpl) error {
					return sc.DeleteSchedule(c)
				})
			},
		},
		{
			Name:    "pause",
			Aliases: []string{"p"},
			Usage:   "Pause a schedule",
			Flags:   pauseScheduleFlags,
			Action: func(c *cli.Context) error {
				if err := checkNoAdditionalArgsPassed(c); err != nil {
					return err
				}
				return withScheduleClient(c, func(sc *scheduleCLIImpl) error {
					return sc.PauseSchedule(c)
				})
			},
		},
		{
			Name:    "unpause",
			Aliases: []string{"up"},
			Usage:   "Unpause a schedule",
			Flags:   unpauseScheduleFlags,
			Action: func(c *cli.Context) error {
				if err := checkNoAdditionalArgsPassed(c); err != nil {
					return err
				}
				return withScheduleClient(c, func(sc *scheduleCLIImpl) error {
					return sc.UnpauseSchedule(c)
				})
			},
		},
		{
			Name:    "backfill",
			Aliases: []string{"bf"},
			Usage:   "Trigger a backfill for a time range",
			Flags:   backfillScheduleFlags,
			Action: func(c *cli.Context) error {
				if err := checkNoAdditionalArgsPassed(c); err != nil {
					return err
				}
				return withScheduleClient(c, func(sc *scheduleCLIImpl) error {
					return sc.BackfillSchedule(c)
				})
			},
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "List schedules in a domain",
			Flags:   listScheduleFlags,
			Action: func(c *cli.Context) error {
				if err := checkNoAdditionalArgsPassed(c); err != nil {
					return err
				}
				return withScheduleClient(c, func(sc *scheduleCLIImpl) error {
					return sc.ListSchedules(c)
				})
			},
		},
	}
}
