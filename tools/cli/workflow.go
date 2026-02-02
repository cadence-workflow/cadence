// Copyright (c) 2017 Uber Technologies, Inc.
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
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/service/worker/batcher"
)

func newWorkflowCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "restart",
			Aliases: []string{"res"},
			Usage:   "restarts a previous workflow execution",
			Flags:   flagsForExecution,
			Action:  RestartWorkflow,
		},
		{
			Name:    "diagnose",
			Aliases: []string{"diag"},
			Usage:   "diagnoses a previous workflow execution",
			Flags:   flagsForExecution,
			Action:  DiagnoseWorkflow,
		},
		{
			Name:    "refresh-tasks",
			Aliases: []string{"rt"},
			Usage:   "refreshes all the workflow tasks to resume progress",
			Flags:   flagsForExecution,
			Action:  RefreshWorkflowTasks,
		},
		{
			Name:        "activity",
			Aliases:     []string{"act"},
			Usage:       "operate activities of workflow",
			Subcommands: newActivityCommands(),
		},
		{
			Name:   "show",
			Usage:  "show workflow history",
			Flags:  getFlagsForShow(),
			Action: ShowHistory,
		},
		{
			Name:        "showid",
			Usage:       "show workflow history with given workflow_id and run_id (a shortcut of `show -w <wid> -r <rid>`). run_id is only required for archived history",
			Description: "cadence workflow showid <workflow_id> <run_id>. workflow_id is required; run_id is only required for archived history",
			Flags:       getFlagsForShowID(),
			Action:      ShowHistoryWithWID,
		},
		{
			Name:   "start",
			Usage:  "start a new workflow execution",
			Flags:  getFlagsForStart(),
			Action: StartWorkflow,
		},
		{
			Name:   "run",
			Usage:  "start a new workflow execution and get workflow progress",
			Flags:  getFlagsForRun(),
			Action: RunWorkflow,
		},
		{
			Name:    "terminate",
			Aliases: []string{"term"},
			Usage:   "force-stop a workflow execution (no cleanup)",
			Flags:   getFlagsForTerminate(),
			Action:  TerminateWorkflow,
		},
		{
			Name:    "cancel",
			Aliases: []string{"c"},
			Usage:   "gracefully stop a workflow execution (similar to terminate, but allows to cleanup)",
			Flags:   getFlagsForCancel(),
			Action:  CancelWorkflow,
		},
		{
			Name:    "signal",
			Aliases: []string{"s"},
			Usage:   "signal a workflow execution",
			Flags:   getFlagsForSignal(),
			Action:  SignalWorkflow,
		},
		{
			Name:   "signalwithstart",
			Usage:  "signal the current open workflow if exists, or attempt to start a new run based on IDResuePolicy and signals it",
			Flags:  getFlagsForSignalWithStart(),
			Action: SignalWithStartWorkflowExecution,
		},
		{
			Name:        "list",
			Aliases:     []string{"l"},
			Usage:       "list open or closed workflow executions",
			Description: "list one page (default size 10 items) by default, use flag --pagesize to change page size",
			Flags:       getFlagsForList(),
			Action:      ListWorkflow,
		},
		{
			Name:    "listall",
			Aliases: []string{"la"},
			Usage:   "list all open or closed workflow executions",
			Flags:   getFlagsForListAll(),
			Action:  ListAllWorkflow,
		},
		{
			Name:   "listarchived",
			Usage:  "list archived workflow executions",
			Flags:  getFlagsForListArchived(),
			Action: ListArchivedWorkflow,
		},
		{
			Name:    "scan",
			Aliases: []string{"sc", "scanall"},
			Usage: "scan workflow executions (need to enable Cadence server on ElasticSearch). " +
				"It will be faster than listall, but result are not sorted.",
			Flags:  getFlagsForScan(),
			Action: ScanAllWorkflow,
		},
		{
			Name:    "count",
			Aliases: []string{"cnt"},
			Usage:   "count number of workflow executions (need to enable Cadence server on ElasticSearch)",
			Flags:   getFlagsForCount(),
			Action:  CountWorkflow,
		},
		{
			Name:        "query",
			Usage:       "query workflow execution",
			Description: "query result will be printed as JSON",
			Flags:       getFlagsForQuery(),
			Action:      QueryWorkflow,
		},
		{
			Name:   "query-types",
			Usage:  "list all available query types",
			Flags:  getFlagsForStack(),
			Action: QueryWorkflowUsingQueryTypes,
		},
		{
			Name:   "stack",
			Usage:  "query workflow execution with __stack_trace as query type",
			Flags:  getFlagsForStack(),
			Action: QueryWorkflowUsingStackTrace,
		},
		{
			Name:    "describe",
			Aliases: []string{"desc"},
			Usage:   "show information of workflow execution",
			Flags:   getFlagsForDescribe(),
			Action:  DescribeWorkflow,
		},
		{
			Name:        "describeid",
			Aliases:     []string{"descid"},
			Usage:       "show information of workflow execution with given workflow_id and optional run_id (a shortcut of `describe -w <wid> -r <rid>`)",
			Description: "cadence workflow describeid <workflow_id> <run_id>. workflow_id is required; run_id is optional",
			Flags:       getFlagsForDescribeID(),
			Action:      DescribeWorkflowWithID,
		},
		{
			Name:    "observe",
			Aliases: []string{"ob"},
			Usage:   "show the progress of workflow history",
			Flags:   getFlagsForObserve(),
			Action:  ObserveHistory,
		},
		{
			Name:    "observeid",
			Aliases: []string{"obid"},
			Usage:   "show the progress of workflow history with given workflow_id and optional run_id (a shortcut of `observe -w <wid> -r <rid>`)",
			Flags:   getFlagsForObserveID(),
			Action:  ObserveHistoryWithID,
		},
		{
			Name:    "reset",
			Aliases: []string{"rs"},
			Usage:   "reset the workflow, by either eventID or resetType.",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"wid", "w"},
					Usage:   "WorkflowID, required",
				},
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"rid", "r"},
					Usage:   "RunID, optional, default to the current/latest RunID",
				},
				&cli.StringFlag{
					Name: FlagEventID,
					Usage: "The eventID of any event after DecisionTaskStarted you want to reset to (this event is exclusive in a new run. The new run " +
						"history will fork and continue from the previous eventID of this). It can be DecisionTaskCompleted, DecisionTaskFailed or others",
				},
				&cli.StringFlag{
					Name:  FlagReason,
					Usage: "reason to do the reset, required for tracking purpose",
				},
				&cli.StringFlag{
					Name:  FlagResetType,
					Usage: "where to reset. Support one of these: " + strings.Join(mapKeysToArray(resetTypesMap), ","),
				},
				&cli.StringFlag{
					Name:  FlagDecisionOffset,
					Usage: "based on the reset point calculated by resetType, this offset will move/offset the point by decision. Currently only negative number is supported, and only works with LastDecisionCompleted.",
				},
				&cli.StringFlag{
					Name:  FlagResetBadBinaryChecksum,
					Usage: "Binary checksum for resetType of BadBinary",
				},
				&cli.StringFlag{
					Name:    FlagEarliestTime,
					Aliases: []string{"et"},
					Usage: "EarliestTime of decision start time, required for resetType of DecisionCompletedTime." +
						"Supported formats are '2006-01-02T15:04:05+07:00', raw UnixNano and " +
						"time range (N<duration>), where 0 < N < 1000000 and duration (full-notation/short-notation) can be second/s, " +
						"minute/m, hour/h, day/d, week/w, month/M or year/y. For example, '15minute' or '15m' implies last 15 minutes, " +
						"meaning that workflow will be reset to the first decision that completed in last 15 minutes.",
				},
				&cli.BoolFlag{
					Name:  FlagSkipSignalReapply,
					Usage: "whether or not skipping signals reapply after the reset point",
				},
			},
			Action: ResetWorkflow,
		},
		{
			Name: "reset-batch",
			Usage: "reset workflow in batch by resetType: " + strings.Join(mapKeysToArray(resetTypesMap), ",") +
				"To get base workflowIDs/runIDs to reset, source is from input file or visibility query.",
			ArgsUsage: "\n\t To reset workflows specify --input_file <csv_file> of workflow_id and run_id and run: cadence wf reset-batch --input_file <csv_file>",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagInputFile,
					Aliases: []string{"if"},
					Usage:   "Input file to use for resetting, one workflow per line of WorkflowID and RunID. RunID is optional, default to current runID if not specified. ",
				},
				&cli.StringFlag{
					Name:    FlagListQuery,
					Aliases: []string{"lq"},
					Usage:   "visibility query to get workflows to reset",
				},
				&cli.StringFlag{
					Name:  FlagExcludeFile,
					Value: "",
					Usage: "Another input file to use for excluding from resetting, only workflowID is needed.",
				},
				&cli.StringFlag{
					Name: FlagExcludeWorkflowIDByQuery,
					Usage: "Another visibility SQL like query, but for excluding the results by workflowIDs. This is useful because a single query cannot do join operation. One use case is to " +
						"find failed workflows excluding any workflow that has another run that is open or completed.",
				},
				&cli.StringFlag{
					Name:  FlagInputSeparator,
					Value: "\t",
					Usage: "Separator for input file(default to tab)",
				},
				&cli.StringFlag{
					Name:  FlagReason,
					Usage: "Reason for reset, required for tracking purpose",
				},
				&cli.IntFlag{
					Name:   FlagParallismDeprecated,
					Value:  1,
					Usage:  "Number of goroutines to run in parallel. Each goroutine would process one line for every second.",
					Hidden: true,
				},
				&cli.IntFlag{
					Name:  FlagParallelism,
					Value: 1,
					Usage: "Number of goroutines to run in parallel. Each goroutine would process one line for every second.",
				},
				&cli.BoolFlag{
					Name:  FlagSkipCurrentOpen,
					Usage: "Skip the workflow if the current run is open for the same workflowID as base.",
				},
				&cli.BoolFlag{
					Name:  FlagSkipCurrentCompleted,
					Usage: "Skip the workflow if the current run is completed for the same workflowID as base.",
				},
				&cli.BoolFlag{
					Name: FlagSkipBaseIsNotCurrent,
					// TODO https://github.com/uber/cadence/issues/2930
					// The right way to prevent needs server side implementation .
					// This client side is only best effort
					Usage: "Skip if base run is not current run.",
				},
				&cli.BoolFlag{
					Name:  FlagNonDeterministicOnly,
					Usage: "Only apply onto workflows whose last event is decisionTaskFailed with non deterministic error.",
				},
				&cli.BoolFlag{
					Name:  FlagDryRun,
					Usage: "Not do real action of reset(just logging in STDOUT)",
				},
				&cli.StringFlag{
					Name:  FlagResetType,
					Usage: "where to reset. Support one of these: " + strings.Join(mapKeysToArray(resetTypesMap), ","),
				},
				&cli.StringFlag{
					Name: FlagDecisionOffset,
					Usage: "based on the reset point calculated by resetType, this offset will move/offset the point by decision. " +
						"Limitation: currently only negative number is supported, and only works with LastDecisionCompleted.",
				},
				&cli.StringFlag{
					Name:  FlagResetBadBinaryChecksum,
					Usage: "Binary checksum for resetType of BadBinary",
				},
				&cli.BoolFlag{
					Name:  FlagSkipSignalReapply,
					Usage: "whether or not skipping signals reapply after the reset point",
				},
				&cli.StringFlag{
					Name:    FlagEarliestTime,
					Aliases: []string{"et"},
					Usage: "EarliestTime of decision start time, required for resetType of DecisionCompletedTime." +
						"Supported formats are '2006-01-02T15:04:05+07:00', raw UnixNano and " +
						"time range (N<duration>), where 0 < N < 1000000 and duration (full-notation/short-notation) can be second/s, " +
						"minute/m, hour/h, day/d, week/w, month/M or year/y. For example, '15minute' or '15m' implies last 15 minutes, " +
						"meaning that workflow will be reset to the first decision that completed in last 15 minutes.",
				},
			},
			Action: ResetInBatch,
		},
		{
			Name:        "batch",
			Usage:       "batch operation on a list of workflows from query.",
			Subcommands: newBatchCommands(),
			ArgsUsage: "\n\t To make a batch operation use wf batch start command and specify --batch_type to terminate/signal/cancel workflows.\n" +
				"\t ex: to batch terminate workflows run: cadence batch start --batch_type terminate --query <targeted_workflows_query>\n" +
				"\t cadence wf batch terminate - is used to terminate a batch operation not workflows.\n" +
				"\t To inspect the progress run: cadence wf batch desc --job_id <your_job_id>",
		},
	}
}

func newActivityCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "complete",
			Aliases: []string{"comp"},
			Usage:   "complete an activity",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"wid", "w"},
					Usage:   "WorkflowID",
				},
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"rid", "r"},
					Usage:   "RunID",
				},
				&cli.StringFlag{
					Name:    FlagActivityID,
					Aliases: []string{"aid"},
					Usage:   "The activityID to operate on",
				},
				&cli.StringFlag{
					Name:  FlagResult,
					Usage: "Result of the activity",
				},
				&cli.StringFlag{
					Name:  FlagIdentity,
					Usage: "Identity of the operator",
				},
			},
			Action: CompleteActivity,
		},
		{
			Name:  "fail",
			Usage: "fail an activity",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"wid", "w"},
					Usage:   "WorkflowID",
				},
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"rid", "r"},
					Usage:   "RunID",
				},
				&cli.StringFlag{
					Name:    FlagActivityID,
					Aliases: []string{"aid"},
					Usage:   "The activityID to operate on",
				},
				&cli.StringFlag{
					Name:  FlagReason,
					Usage: "Reason to fail the activity",
				},
				&cli.StringFlag{
					Name:  FlagDetail,
					Usage: "Detail to fail the activity",
				},
				&cli.StringFlag{
					Name:  FlagIdentity,
					Usage: "Identity of the operator",
				},
			},
			Action: FailActivity,
		},
	}
}

func newBatchCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "describe",
			Aliases: []string{"desc"},
			Usage:   "Describe a batch operation job",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagJobID,
					Aliases: []string{"jid"},
					Usage:   "Batch Job ID",
				},
			},
			Action: DescribeBatchJob,
		},
		{
			Name:  "terminate",
			Usage: "terminate a batch operation job",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagJobID,
					Aliases: []string{"jid"},
					Usage:   "Batch Job ID",
				},
				&cli.StringFlag{
					Name:    FlagReason,
					Aliases: []string{"re"},
					Usage:   "Reason to stop this batch job",
				},
			},
			Action: TerminateBatchJob,
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "Describe a batch operation job",
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:    FlagPageSize,
					Aliases: []string{"ps"},
					Value:   30,
					Usage:   "Result page size",
				},
			},
			Action: ListBatchJobs,
		},
		{
			Name:  "start",
			Usage: "Start a batch operation job",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagListQuery,
					Aliases: []string{"q"},
					Usage:   "Query to get workflows for being executed this batch operation",
				},
				&cli.StringFlag{
					Name:    FlagReason,
					Aliases: []string{"re"},
					Usage:   "Reason to run this batch job",
				},
				&cli.StringFlag{
					Name:    FlagBatchType,
					Aliases: []string{"bt"},
					Usage:   "Types supported: " + strings.Join(batcher.AllBatchTypes, ","),
				},
				// below are optional
				&cli.StringFlag{
					Name:    FlagSignalName,
					Aliases: []string{"sn"},
					Usage:   "Required for batch signal",
				},
				&cli.StringFlag{
					Name:    FlagInput,
					Aliases: []string{"in"},
					Usage:   "Optional input of signal",
				},
				&cli.StringFlag{
					Name:    FlagSourceCluster,
					Aliases: []string{"sc"},
					Usage:   "Required for batch replicate",
				},
				&cli.StringFlag{
					Name:    FlagTargetCluster,
					Aliases: []string{"tc"},
					Usage:   "Required for batch replicate",
				},
				&cli.IntFlag{
					Name:  FlagRPS,
					Value: batcher.DefaultRPS,
					Usage: "RPS of processing",
				},
				&cli.BoolFlag{
					Name:  FlagYes,
					Usage: "Optional flag to disable confirmation prompt",
				},
				&cli.IntFlag{
					Name:  FlagPageSize,
					Value: batcher.DefaultPageSize,
					Usage: "PageSize of processiing",
				},
				&cli.IntFlag{
					Name:  FlagRetryAttempts,
					Value: batcher.DefaultAttemptsOnRetryableError,
					Usage: "Retry attempts for retriable errors",
				},
				// TODO duration should use DurationFlag instead of IntFlag
				&cli.IntFlag{
					Name:    FlagActivityHeartBeatTimeout,
					Aliases: []string{"hbt"},
					Value:   int(batcher.DefaultActivityHeartBeatTimeout / time.Second),
					Usage:   "Heartbeat timeout for batcher activity in seconds",
				},
				&cli.IntFlag{
					Name:  FlagConcurrency,
					Value: batcher.DefaultConcurrency,
					Usage: "Concurrency of batch activity",
				},
				&cli.IntFlag{
					Name:  FlagMaxActivityRetries,
					Value: batcher.DefaultMaxActivityRetries,
					Usage: "Max retries of batch activity, before retrying the whole workflow (0 means unlimited)",
				},
			},
			Action: StartBatchJob,
		},
	}
}
