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
