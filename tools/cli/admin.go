// Copyright (c) 2017-2020 Uber Technologies Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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
	"time"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/service/worker/scanner/executions"
)

func newAdminWorkflowCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:  "show",
			Usage: "show workflow history from database",
			Flags: append(getDBFlags(),
				// v2 history events
				&cli.StringFlag{
					Name:  FlagTreeID,
					Usage: "TreeID",
				},
				&cli.StringFlag{
					Name:  FlagBranchID,
					Usage: "BranchID",
				},
				&cli.Int64Flag{
					Name:  FlagMinEventID,
					Value: 1,
					Usage: "MinEventID",
				},
				&cli.Int64Flag{
					Name:  FlagMaxEventID,
					Value: 10000,
					Usage: "MaxEventID",
				},
				&cli.StringFlag{
					Name:    FlagOutputFilename,
					Aliases: []string{"of"},
					Usage:   "output file",
				},
				// support mysql query
				&cli.IntFlag{
					Name:    FlagShardID,
					Aliases: []string{"sid"},
					Usage:   "ShardID",
				}),
			Action: AdminShowWorkflow,
		},
		{
			Name:    "describe",
			Aliases: []string{"desc"},
			Usage:   "Describe internal information of workflow execution",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"w", "wid"},
					Usage:   "WorkflowID",
				},
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"r", "rid"},
					Usage:   "RunID",
				},
			},
			Action: AdminDescribeWorkflow,
		},
		{
			Name:    "refresh-tasks",
			Aliases: []string{"rt"},
			Usage:   "Refreshes all the tasks of a workflow",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"w", "wid"},
					Usage:   "WorkflowID",
				},
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"r", "rid"},
					Usage:   "RunID",
				},
			},
			Action: AdminRefreshWorkflowTasks,
		},
		{
			Name:    "delete",
			Aliases: []string{"del"},
			Usage:   "Delete current workflow execution and the mutableState record",
			Flags: append(getDBFlags(),
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"w", "wid"},
					Usage:   "WorkflowID",
				},
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"r", "rid"},
					Usage:   "RunID",
				},
				&cli.BoolFlag{
					Name:    FlagSkipErrorMode,
					Aliases: []string{"serr"},
					Usage:   "skip errors when deleting history",
				},
				&cli.BoolFlag{
					Name:  FlagRemote,
					Usage: "Executes deletion on server side",
				}),
			Action: AdminDeleteWorkflow,
		},
		{
			Name:    "fix_corruption",
			Aliases: []string{"fc"},
			Usage:   "Checks if workflow record is corrupted in database and cleans up",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"w", "wid"},
					Usage:   "WorkflowID",
				},
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"r", "rid"},
					Usage:   "RunID",
				},
				&cli.BoolFlag{
					Name:    FlagSkipErrorMode,
					Aliases: []string{"serr"},
					Usage:   "Skip errors and tries to delete as much as possible from the DB",
				},
			},
			Action: AdminMaintainCorruptWorkflow,
		},
	}
}

func newAdminShardManagementCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "describe",
			Aliases: []string{"d"},
			Usage:   "Describe shard by Id",
			Flags: append(
				getDBFlags(),
				&cli.IntFlag{
					Name:  FlagShardID,
					Usage: "The Id of the shard to describe",
				},
			),
			Action: AdminDescribeShard,
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "List shard distribution",
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:  FlagPageSize,
					Value: 100,
					Usage: "Max number of results to return",
				},
				&cli.IntFlag{
					Name:  FlagPageID,
					Value: 0,
					Usage: "Option to show results offset from pagesize * page_id",
				},
				getFormatFlag(),
			},
			Action: AdminDescribeShardDistribution,
		},
		{
			Name:    "setRangeID",
			Aliases: []string{"srid"},
			Usage:   "Force update shard rangeID",
			Flags: append(
				getDBFlags(),
				&cli.IntFlag{
					Name:    FlagShardID,
					Aliases: []string{"sid"},
					Usage:   "ID of the shard to reset",
				},
				&cli.Int64Flag{
					Name:    FlagRangeID,
					Aliases: []string{"rid"},
					Usage:   "new shard rangeID",
				},
			),
			Action: AdminSetShardRangeID,
		},
		{
			Name:    "closeShard",
			Aliases: []string{"clsh"},
			Usage:   "close a shard given a shard id",
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:  FlagShardID,
					Usage: "ShardID for the cadence cluster to manage",
				},
			},
			Action: AdminCloseShard,
		},
		{
			Name:    "removeTask",
			Aliases: []string{"rmtk"},
			Usage:   "remove a task based on shardID, task type, taskID, and task visibility timestamp",
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:  FlagShardID,
					Usage: "shardID",
				},
				&cli.Int64Flag{
					Name:  FlagTaskID,
					Usage: "taskID",
				},
				&cli.IntFlag{
					Name:  FlagTaskType,
					Usage: "task type: 2 (transfer task), 3 (timer task), 4 (replication task) or 6 (cross-cluster task)",
				},
				&cli.Int64Flag{
					Name:  FlagTaskVisibilityTimestamp,
					Usage: "task visibility timestamp in nano (required for removing timer task)",
				},
				&cli.StringFlag{
					Name:  FlagCluster,
					Usage: "target cluster of the task (required for removing cross-cluster task)",
				},
			},
			Action: AdminRemoveTask,
		},
		{
			Name:  "timers",
			Usage: "get scheduled timers for a given time range",
			Flags: append(getDBFlags(),
				&cli.IntFlag{
					Name:  FlagShardID,
					Usage: "shardID",
				},
				&cli.IntFlag{
					Name:  FlagPageSize,
					Usage: "page size used to query db executions table",
					Value: 500,
				},
				&cli.StringFlag{
					Name:  FlagStartDate,
					Usage: "start date",
					Value: time.Now().UTC().Format(time.RFC3339),
				},
				&cli.StringFlag{
					Name:  FlagEndDate,
					Usage: "end date",
					Value: time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
				},
				&cli.StringFlag{
					Name:  FlagDomainID,
					Usage: "filter tasks by DomainID",
				},
				&cli.IntSliceFlag{
					Name: FlagTimerType,
					Usage: "timer types: 0 - DecisionTimeoutTask, 1 - TaskTypeActivityTimeout, " +
						"2 - TaskTypeUserTimer, 3 - TaskTypeWorkflowTimeout, 4 - TaskTypeDeleteHistoryEvent, " +
						"5 - TaskTypeActivityRetryTimer, 6 - TaskTypeWorkflowBackoffTimer",
					Value: cli.NewIntSlice(-1),
				},
				&cli.BoolFlag{
					Name:  FlagPrintJSON,
					Usage: "print raw json data instead of histogram",
				},

				&cli.BoolFlag{
					Name:  FlagSkipErrorMode,
					Usage: "skip errors",
				},
				&cli.StringFlag{
					Name:  FlagInputFile,
					Usage: "file to use, will not connect to persistence",
				},
				&cli.StringFlag{
					Name:  FlagDateFormat,
					Usage: "create buckets using time format. Use Go reference time: Mon Jan 2 15:04:05 MST 2006. If set, --" + FlagBucketSize + " is ignored",
				},
				&cli.StringFlag{
					Name:  FlagBucketSize,
					Value: "hour",
					Usage: "group timers by time bucket. Available: day, hour, minute, second",
				},
				&cli.IntFlag{
					Name:  FlagShardMultiplier,
					Usage: "multiply timer counters for histogram",
					Value: 16384,
				},
			),
			Action: AdminTimers,
		},
	}
}

func newAdminHistoryHostCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "describe",
			Aliases: []string{"desc"},
			Usage:   "Describe internal information of history host",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"w", "wid"},
					Usage:   "WorkflowID",
				},
				&cli.StringFlag{
					Name:    FlagHistoryAddress,
					Aliases: []string{"had"},
					Usage:   "History Host address(IP:PORT)",
				},
				&cli.IntFlag{
					Name:    FlagShardID,
					Aliases: []string{"sid"},
					Usage:   "ShardID",
				},
				&cli.BoolFlag{
					Name:    FlagPrintFullyDetail,
					Aliases: []string{"pf"},
					Usage:   "Print fully detail",
				},
			},
			Action: AdminDescribeHistoryHost,
		},
		{
			Name:    "getshard",
			Aliases: []string{"gsh"},
			Usage:   "Get shardID for a workflowID",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"wid", "w"},
					Usage:   "WorkflowID",
				},
				&cli.IntFlag{
					Name:  FlagNumberOfShards,
					Usage: "NumberOfShards for the cadence cluster(see config for numHistoryShards)",
				},
			},
			Action: AdminGetShardID,
		},
	}
}

// may be better to do this inside an "ensure setup" in each method,
// but this reduces branches for testing.
func withDomainClient(c *cli.Context, admin bool, cb func(dc *domainCLIImpl) error) error {
	dc, err := newDomainCLI(c, admin)
	if err != nil {
		return err
	}
	return cb(dc)
}

func newAdminDomainCommands() []*cli.Command {

	return []*cli.Command{
		{
			Name:    "register",
			Aliases: []string{"re"},
			Usage:   "Register workflow domain",
			Flags:   adminRegisterDomainFlags,
			Action: func(c *cli.Context) error {
				return withDomainClient(c, true, func(dc *domainCLIImpl) error {
					return dc.RegisterDomain(c)
				})
			},
		},
		{
			Name:    "update",
			Aliases: []string{"up", "u"},
			Usage:   "Update existing workflow domain",
			Flags:   adminUpdateDomainFlags,
			Action: func(c *cli.Context) error {
				return withDomainClient(c, true, func(dc *domainCLIImpl) error {
					return dc.UpdateDomain(c)
				})
			},
		},
		{
			Name:    "deprecate",
			Aliases: []string{"dep"},
			Usage:   "Deprecate existing workflow domain",
			Flags:   adminDeprecateDomainFlags,
			Action: func(c *cli.Context) error {
				return withDomainClient(c, true, func(dc *domainCLIImpl) error {
					return dc.DeprecateDomain(c)
				})
			},
		},
		{
			Name:    "describe",
			Aliases: []string{"desc"},
			Usage:   "Describe existing workflow domain",
			Flags:   adminDescribeDomainFlags,
			Action: func(c *cli.Context) error {
				return withDomainClient(c, true, func(dc *domainCLIImpl) error {
					return dc.DescribeDomain(c)
				})
			},
		},
		{
			Name:    "getdomainidorname",
			Aliases: []string{"getdn"},
			Usage:   "Get domainID or domainName",
			Flags: append(getDBFlags(),
				&cli.StringFlag{
					Name:  FlagDomainID,
					Usage: "Domain ID(uuid)",
				}),
			Action: AdminGetDomainIDOrName,
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "List all domains in the cluster",
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:    FlagPageSize,
					Aliases: []string{"ps"},
					Value:   10,
					Usage:   "Result page size",
				},
				&cli.BoolFlag{
					Name:    FlagAll,
					Aliases: []string{"a"},
					Usage:   "List all domains, by default only domains in REGISTERED status are listed",
				},
				&cli.BoolFlag{
					Name:    FlagDeprecated,
					Aliases: []string{"dep"},
					Usage:   "List deprecated domains only, by default only domains in REGISTERED status are listed",
				},
				&cli.StringFlag{
					Name:  FlagPrefix,
					Usage: "List domains that are matching to the given prefix",
					Value: "",
				},
				&cli.BoolFlag{
					Name:    FlagPrintFullyDetail,
					Aliases: []string{"pf"},
					Usage:   "Print full domain detail",
				},
				&cli.BoolFlag{
					Name:    FlagPrintJSON,
					Aliases: []string{"pjson"},
					Usage:   "Print in raw json format (DEPRECATED: instead use --format json)",
				},
				getFormatFlag(),
			},
			Action: func(c *cli.Context) error {
				return withDomainClient(c, false, func(dc *domainCLIImpl) error {
					return dc.ListDomains(c)
				})
			},
		},
	}
}

func newAdminKafkaCommands() []*cli.Command {
	return []*cli.Command{
		{
			// TODO: do we still need this command given that kafka replication has been deprecated?
			Name:    "parse",
			Aliases: []string{"par"},
			Usage:   "Parse replication tasks from kafka messages",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagInputFile,
					Aliases: []string{"if"},
					Usage:   "Input file to use, if not present assumes piping",
				},
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"w", "wid"},
					Usage:   "WorkflowID",
				},
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"r", "rid"},
					Usage:   "RunID",
				},
				&cli.StringFlag{
					Name:    FlagOutputFilename,
					Aliases: []string{"of"},
					Usage:   "Output file to write to, if not provided output is written to stdout",
				},
				&cli.BoolFlag{
					Name:    FlagSkipErrorMode,
					Aliases: []string{"serr"},
					Usage:   "Skip errors in parsing messages",
				},
				&cli.BoolFlag{
					Name:    FlagHeadersMode,
					Aliases: []string{"he"},
					Usage:   "Output headers of messages in format: DomainID, WorkflowID, RunID, FirstEventID, NextEventID",
				},
				&cli.IntFlag{
					Name:    FlagMessageType,
					Aliases: []string{"mt"},
					Usage:   "Kafka message type (0: replicationTasks; 1: visibility)",
					Value:   0,
				},
			},
			Action: AdminKafkaParse,
		},
		{
			// TODO: move this command be a subcommand of admin workflow
			Name:    "rereplicate",
			Aliases: []string{"rrp"},
			Usage:   "Rereplicate replication tasks from history tables",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  FlagSourceCluster,
					Usage: "Name of source cluster to resend the replication task",
				},
				&cli.StringFlag{
					Name:  FlagDomainID,
					Usage: "DomainID",
				},
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"w", "wid"},
					Usage:   "WorkflowID",
				},
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"r", "rid"},
					Usage:   "RunID",
				},
				&cli.Int64Flag{
					Name:  FlagMaxEventID,
					Usage: "MaxEventID Optional, default to all events",
				},
				&cli.StringFlag{
					Name:  FlagEndEventVersion,
					Usage: "Workflow end event version, required if MaxEventID is specified",
				}},
			Action: AdminRereplicate,
		},
	}
}

func newAdminElasticSearchCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "catIndex",
			Aliases: []string{"cind"},
			Usage:   "Cat Indices on ElasticSearch",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  FlagURL,
					Usage: "URL of ElasticSearch cluster",
				},
				getFormatFlag(),
			},
			Action: AdminCatIndices,
		},
		{
			Name:    "index",
			Aliases: []string{"ind"},
			Usage:   "Index docs on ElasticSearch",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  FlagURL,
					Usage: "URL of ElasticSearch cluster",
				},
				&cli.StringFlag{
					Name:  FlagIndex,
					Usage: "ElasticSearch target index",
				},
				&cli.StringFlag{
					Name:    FlagInputFile,
					Aliases: []string{"if"},
					Usage:   "Input file of indexer.Message in json format, separated by newline",
				},
				&cli.IntFlag{
					Name:    FlagBatchSize,
					Aliases: []string{"bs"},
					Usage:   "Optional batch size of actions for bulk operations",
					Value:   1000,
				},
			},
			Action: AdminIndex,
		},
		{
			Name:    "delete",
			Aliases: []string{"del"},
			Usage:   "Delete docs on ElasticSearch",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  FlagURL,
					Usage: "URL of ElasticSearch cluster",
				},
				&cli.StringFlag{
					Name:  FlagIndex,
					Usage: "ElasticSearch target index",
				},
				&cli.StringFlag{
					Name:    FlagInputFile,
					Aliases: []string{"if"},
					Usage: "Input file name. Redirect cadence wf list result (with tale format) to a file and use as delete input. " +
						"First line should be table header like WORKFLOW TYPE | WORKFLOW ID | RUN ID | ...",
				},
				&cli.IntFlag{
					Name:    FlagBatchSize,
					Aliases: []string{"bs"},
					Usage:   "Optional batch size of actions for bulk operations",
					Value:   1000,
				},
				&cli.IntFlag{
					Name:  FlagRPS,
					Usage: "Optional batch request rate per second",
					Value: 30,
				},
			},
			Action: AdminDelete,
		},
		{
			Name:    "report",
			Aliases: []string{"rep"},
			Usage:   "Generate Report by Aggregation functions on ElasticSearch",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  FlagURL,
					Usage: "URL of ElasticSearch cluster",
				},
				&cli.StringFlag{
					Name:  FlagIndex,
					Usage: "ElasticSearch target index",
				},
				&cli.StringFlag{
					Name:  FlagListQuery,
					Usage: "SQL query of the report",
				},
				&cli.StringFlag{
					Name:  FlagOutputFormat,
					Usage: "Additional output format (html or csv)",
				},
				&cli.StringFlag{
					Name:  FlagOutputFilename,
					Usage: "Additional output filename with path",
				},
			},
			Action: GenerateReport,
		},
	}
}

func newAdminTaskListCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "describe",
			Aliases: []string{"desc"},
			Usage:   "Describe pollers and status information of tasklist",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagTaskList,
					Aliases: []string{"tl"},
					Usage:   "TaskList description",
				},
				&cli.StringFlag{
					Name:    FlagTaskListType,
					Aliases: []string{"tlt"},
					Value:   "decision",
					Usage:   "Optional TaskList type [decision|activity]",
				},
			},
			Action: AdminDescribeTaskList,
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "List active tasklist under a domain",
			Action:  AdminListTaskList,
		},
		{
			Name:    "update-partition",
			Aliases: []string{"up"},
			Usage:   "Update tasklist's partition config",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagTaskList,
					Aliases: []string{"tl"},
					Usage:   "TaskList Name",
				},
				&cli.StringFlag{
					Name:    FlagTaskListType,
					Aliases: []string{"tlt"},
					Usage:   "TaskList type [decision|activity]",
				},
				&cli.IntFlag{
					Name:    FlagNumReadPartitions,
					Aliases: []string{"nrp"},
					Usage:   "Number of read partitions",
				},
				&cli.IntFlag{
					Name:    FlagNumWritePartitions,
					Aliases: []string{"nwp"},
					Usage:   "Number of write partitions",
				},
				&cli.BoolFlag{
					Name:    FlagForce,
					Aliases: []string{"f"},
					Usage:   "Force an update operation that may be unsafe",
				},
			},
			Action: AdminUpdateTaskListPartitionConfig,
		},
	}
}

func newAdminClusterCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "add-search-attr",
			Aliases: []string{"asa"},
			Usage:   "whitelist search attribute",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  FlagSearchAttributesKey,
					Usage: "Search Attribute key to be whitelisted",
				},
				&cli.IntFlag{
					Name:  FlagSearchAttributesType,
					Value: -1,
					Usage: "Search Attribute value type. [0:String, 1:Keyword, 2:Int, 3:Double, 4:Bool, 5:Datetime]",
				},
				&cli.StringFlag{
					Name:    FlagSecurityToken,
					Aliases: []string{"st"},
					Usage:   "Optional token for security check",
				},
			},
			Action: AdminAddSearchAttribute,
		},
		{
			Name:    "describe",
			Aliases: []string{"d"},
			Usage:   "Describe cluster information",
			Action:  AdminDescribeCluster,
		},
		{
			Name:        "failover",
			Aliases:     []string{"fo"},
			Usage:       "Failover domains with domain data IsManagedByCadence=true to target cluster",
			Subcommands: newAdminFailoverCommands(),
		},
		{
			Name:    "failover_fast",
			Aliases: []string{"fof"},
			Usage:   "Failover domains with domain data IsManagedByCadence=true to target cluster",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagTargetCluster,
					Aliases: []string{"tc"},
					Usage:   "Target active cluster name",
				},
			},
			Action: func(c *cli.Context) error {
				return withDomainClient(c, false, func(dc *domainCLIImpl) error {
					return dc.FailoverDomains(c)
				})
			},
		},
		{
			Name:        "rebalance",
			Aliases:     []string{"rb"},
			Usage:       "Rebalance the domains active cluster",
			Subcommands: newAdminRebalanceCommands(),
		},
	}
}

func getDLQFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  FlagShards,
			Usage: "Comma separated shard IDs or inclusive ranges. Example: \"2,5-6,10\".  Alternatively, feed one shard ID per line via STDIN.",
		},
		&cli.StringFlag{
			Name:    FlagDLQType,
			Aliases: []string{"dt"},
			Usage:   "Type of DLQ to manage. (Options: domain, history)",
			Value:   "history",
		},
		&cli.StringFlag{
			Name:  FlagSourceCluster,
			Usage: "The cluster where the task is generated",
		},
		&cli.IntFlag{
			Name:    FlagLastMessageID,
			Aliases: []string{"lm"},
			Usage:   "The upper boundary of the read message",
		},
	}
}

func newAdminDLQCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "count",
			Aliases: []string{"c"},
			Usage:   "Count DLQ Messages",
			Flags: []cli.Flag{
				getFormatFlag(),
				&cli.StringFlag{
					Name:    FlagDLQType,
					Aliases: []string{"dt"},
					Usage:   "Type of DLQ to manage. (Options: domain, history)",
					Value:   "history",
				},
				&cli.BoolFlag{
					Name:  FlagForce,
					Usage: "Force fetch latest counts (will put additional stress on DB)",
				},
			},
			Action: AdminCountDLQMessages,
		},
		{
			Name:    "read",
			Aliases: []string{"r"},
			Usage:   "Read DLQ Messages",
			Flags: append(getDLQFlags(),
				&cli.IntFlag{
					Name:    FlagMaxMessageCount,
					Aliases: []string{"mmc"},
					Usage:   "Max message size to fetch",
				},
				getFormatFlag(),
			),
			Action: AdminGetDLQMessages,
		},
		{
			Name:    "purge",
			Aliases: []string{"p"},
			Usage:   "Delete DLQ messages with equal or smaller ids than the provided task id",
			Flags:   getDLQFlags(),
			Action:  AdminPurgeDLQMessages,
		},
		{
			Name:    "merge",
			Aliases: []string{"m"},
			Usage:   "Merge DLQ messages with equal or smaller ids than the provided task id",
			Flags:   getDLQFlags(),
			Action:  AdminMergeDLQMessages,
		},
	}
}

func newAdminQueueCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:   "reset",
			Usage:  "reset processing queue states for transfer or timer queue processor",
			Flags:  getQueueCommandFlags(),
			Action: AdminResetQueue,
		},
		{
			Name:    "describe",
			Aliases: []string{"desc"},
			Usage:   "describe processing queue states for transfer or timer queue processor",
			Flags:   getQueueCommandFlags(),
			Action:  AdminDescribeQueue,
		},
	}
}

func newAdminAsyncQueueCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:   "get",
			Usage:  "get async workflow queue configuration of a domain",
			Action: AdminGetAsyncWFConfig,
		},
		{
			Name:  "update",
			Usage: "upsert async workflow queue configuration of a domain",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     FlagJSON,
					Usage:    `AsyncWorkflowConfiguration in json format. Schema can be found in https://github.com/uber/cadence/blob/master/common/types/admin.go`,
					Required: true,
				},
			},
			Action: AdminUpdateAsyncWFConfig,
		},
	}
}

func newDBCommands() []*cli.Command {
	var collections cli.StringSlice = *cli.NewStringSlice(invariant.CollectionStrings()...)

	scanFlag := &cli.StringFlag{
		Name:     FlagScanType,
		Usage:    "Scan type to use: " + strings.Join(executions.ScanTypeStrings(), ", "),
		Required: true,
	}

	collectionsFlag := &cli.StringSliceFlag{
		Name:  FlagInvariantCollection,
		Usage: "Scan collection type to use: " + strings.Join(collections.Value(), ", "),
		Value: &collections,
	}

	verboseFlag := &cli.BoolFlag{
		Name:     FlagVerbose,
		Usage:    "verbose output",
		Required: false,
	}

	return []*cli.Command{
		{
			Name:  "scan",
			Usage: "scan executions in database and detect corruptions",
			Flags: append(getDBFlags(),
				&cli.IntFlag{
					Name:     FlagNumberOfShards,
					Usage:    "NumberOfShards for the cadence cluster (see config for numHistoryShards)",
					Required: true,
				},
				scanFlag,
				collectionsFlag,
				&cli.StringFlag{
					Name:    FlagInputFile,
					Aliases: []string{"if"},
					Usage:   "Input file of executions to scan in JSON format {\"DomainID\":\"x\",\"WorkflowID\":\"x\",\"RunID\":\"x\"} separated by a newline",
				},
				verboseFlag,
			),

			Action: AdminDBScan,
		},
		{
			Name:  "unsupported-workflow",
			Usage: "use this command when upgrade the Cadence server from version less than 0.16.0. This scan database and detect unsupported workflow type.",
			Flags: append(getDBFlags(),
				&cli.IntFlag{
					Name:  FlagRPS,
					Usage: "NumberOfShards for the cadence cluster (see config for numHistoryShards)",
					Value: 1000,
				},
				&cli.StringFlag{
					Name:    FlagOutputFilename,
					Aliases: []string{"of"},
					Usage:   "Output file to write to, if not provided output is written to stdout",
				},
				&cli.IntFlag{
					Name:     FlagLowerShardBound,
					Usage:    "FlagLowerShardBound for the start shard to scan. (Default: 0)",
					Value:    0,
					Required: true,
				},
				&cli.IntFlag{
					Name:     FlagUpperShardBound,
					Usage:    "FlagLowerShardBound for the end shard to scan. (Default: 16383)",
					Value:    16383,
					Required: true,
				},
			),

			Action: AdminDBScanUnsupportedWorkflow,
		},
		{
			Name:  "clean",
			Usage: "clean up corrupted workflows",
			Flags: append(getDBFlags(),
				scanFlag,
				collectionsFlag,
				&cli.StringFlag{
					Name:    FlagInputFile,
					Aliases: []string{"if"},
					Usage:   "Input file of execution to clean in JSON format. Use `scan` command to generate list of executions.",
				},
				verboseFlag,
			),
			Action: AdminDBClean,
		},
		{
			Name:  "decode_thrift",
			Usage: "decode thrift object, print into JSON if the data is matching with any supported struct",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagInput,
					Aliases: []string{"i"},
					EnvVars: []string{"Input"},
					Usage:   "Input of Thrift encoded data structure.",
				},
				&cli.StringFlag{
					Name:    FlagInputEncoding,
					Aliases: []string{"enc"},
					Usage:   "Encoding of the input: [hex|base64] (Default: hex)",
				},
			},
			Action: AdminDBDataDecodeThrift,
		},
	}
}

func getQueueCommandFlags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{
			Name:    FlagShardID,
			Aliases: []string{"sid"},
			Usage:   "shardID",
		},
		&cli.StringFlag{
			Name:  FlagCluster,
			Usage: "cluster the task processor is responsible for",
		},
		&cli.IntFlag{
			Name:  FlagQueueType,
			Usage: "queue type: 2 (transfer queue), 3 (timer queue) or 6 (cross-cluster queue)",
		},
	}
}

func newAdminFailoverCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "start",
			Aliases: []string{"s"},
			Usage:   "start failover workflow",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagTargetCluster,
					Aliases: []string{"tc"},
					Usage:   "Target cluster name",
				},
				&cli.StringFlag{
					Name:    FlagSourceCluster,
					Aliases: []string{"sc"},
					Usage:   "Source cluster name",
				},
				&cli.IntFlag{
					Name:    FlagFailoverTimeout,
					Aliases: []string{"fts"},
					Usage:   "Optional graceful failover timeout in seconds. If this field is define, the failover will use graceful failover.",
				},
				&cli.IntFlag{
					Name:    FlagExecutionTimeout,
					Aliases: []string{"et"},
					Usage:   "Optional Failover workflow timeout in seconds",
					Value:   defaultFailoverWorkflowTimeoutInSeconds,
				},
				&cli.IntFlag{
					Name:    FlagFailoverWaitTime,
					Aliases: []string{"fwts"},
					Usage:   "Optional Failover wait time after each batch in seconds",
					Value:   defaultBatchFailoverWaitTimeInSeconds,
				},
				&cli.IntFlag{
					Name:    FlagFailoverBatchSize,
					Aliases: []string{"fbs"},
					Usage:   "Optional number of domains to failover in one batch",
					Value:   defaultBatchFailoverSize,
				},
				&cli.StringSliceFlag{
					Name: FlagFailoverDomains,
					Usage: "Optional domains to failover, eg d1,d2..,dn. " +
						"Only provided domains in source cluster will be failover.",
				},
				&cli.IntFlag{
					Name:    FlagFailoverDrillWaitTime,
					Aliases: []string{"fdws"},
					Usage: "Optional failover drill wait time. " +
						"After the wait time, the domains will be reset to original regions." +
						"This field is required if the cron schedule is specified.",
				},
				&cli.StringFlag{
					Name: FlagCronSchedule,
					Usage: "Optional cron schedule on failover drill. Please specify failover drill wait time " +
						"if this field is specific",
				},
			},
			Action: AdminFailoverStart,
		},
		{
			Name:    "pause",
			Aliases: []string{"p"},
			Usage:   "pause failover workflow",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"rid", "r"},
					Usage:   "Optional Failover workflow runID, default is latest runID",
				},
				&cli.BoolFlag{
					Name:    FlagFailoverDrill,
					Aliases: []string{"fd"},
					Usage: "Optional to pause failover workflow or failover drill workflow." +
						" The default is normal failover workflow",
				},
			},

			Action: AdminFailoverPause,
		},
		{
			Name:    "resume",
			Aliases: []string{"re"},
			Usage:   "resume paused failover workflow",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"rid", "r"},
					Usage:   "Optional Failover workflow runID, default is latest runID",
				},
				&cli.BoolFlag{
					Name:    FlagFailoverDrill,
					Aliases: []string{"fd"},
					Usage: "Optional to resume failover workflow or failover drill workflow." +
						" The default is normal failover workflow",
				},
			},
			Action: AdminFailoverResume,
		},
		{
			Name:    "query",
			Aliases: []string{"q"},
			Usage:   "query failover workflow state",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    FlagFailoverDrill,
					Aliases: []string{"fd"},
					Usage: "Optional to query failover workflow or failover drill workflow." +
						" The default is normal failover workflow",
				},
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"rid", "r"},
					Usage:   "Optional Failover workflow runID, default is latest runID",
				},
			},
			Action: AdminFailoverQuery,
		},
		{
			Name:    "abort",
			Aliases: []string{"a"},
			Usage:   "abort failover workflow",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"rid", "r"},
					Usage:   "Optional Failover workflow runID, default is latest runID",
				},
				&cli.StringFlag{
					Name:    FlagReason,
					Aliases: []string{"re"},
					Usage:   "Optional reason why abort",
				},
				&cli.BoolFlag{
					Name:    FlagFailoverDrill,
					Aliases: []string{"fd"},
					Usage: "Optional to abort failover workflow or failover drill workflow." +
						" The default is normal failover workflow",
				},
			},
			Action: AdminFailoverAbort,
		},
		{
			Name:    "rollback",
			Aliases: []string{"ro"},
			Usage:   "rollback failover workflow",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    FlagRunID,
					Aliases: []string{"rid"},
					Usage:   "Optional Failover workflow runID, default is latest runID",
				},
				&cli.IntFlag{
					Name:    FlagFailoverTimeout,
					Aliases: []string{"fts"},
					Usage:   "Optional graceful failover timeout in seconds. If this field is define, the failover will use graceful failover.",
				},
				&cli.IntFlag{
					Name:    FlagExecutionTimeout,
					Aliases: []string{"et"},
					Usage:   "Optional Failover workflow timeout in seconds",
					Value:   defaultFailoverWorkflowTimeoutInSeconds,
				},
				&cli.IntFlag{
					Name:    FlagFailoverWaitTime,
					Aliases: []string{"fwts"},
					Usage:   "Optional Failover wait time after each batch in seconds",
					Value:   defaultBatchFailoverWaitTimeInSeconds,
				},
				&cli.IntFlag{
					Name:    FlagFailoverBatchSize,
					Aliases: []string{"fbs"},
					Usage:   "Optional number of domains to failover in one batch",
					Value:   defaultBatchFailoverSize,
				},
			},
			Action: AdminFailoverRollback,
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "list failover workflow runs closed/open. This is just a simplified list cmd",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    FlagOpen,
					Aliases: []string{"op"},
					Usage:   "List for open workflow executions, default is to list for closed ones",
				},
				&cli.IntFlag{
					Name:    FlagPageSize,
					Aliases: []string{"ps"},
					Value:   10,
					Usage:   "Result page size",
				},
				&cli.StringFlag{
					Name:    FlagWorkflowID,
					Aliases: []string{"wid", "w"},
					Usage:   "Ignore this. It is a dummy flag which will be forced overwrite",
				},
				&cli.BoolFlag{
					Name:    FlagFailoverDrill,
					Aliases: []string{"fd"},
					Usage: "Optional to query failover workflow or failover drill workflow." +
						" The default is normal failover workflow",
				},
			},
			Action: AdminFailoverList,
		},
	}
}

func newAdminRebalanceCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "start",
			Aliases: []string{"s"},
			Usage:   "start rebalance workflow",
			Flags:   []cli.Flag{},
			Action:  AdminRebalanceStart,
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "list rebalance workflow runs closed/open.",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    FlagOpen,
					Aliases: []string{"op"},
					Usage:   "List for open workflow executions, default is to list for closed ones",
				},
				&cli.IntFlag{
					Name:    FlagPageSize,
					Aliases: []string{"ps"},
					Value:   10,
					Usage:   "Result page size",
				},
			},
			Action: AdminRebalanceList,
		},
	}
}

func newAdminConfigStoreCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:    "get",
			Aliases: []string{"g"},
			Usage:   "Get Dynamic Config Value",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     FlagDynamicConfigName,
					Usage:    "Name of Dynamic Config parameter to get value of",
					Required: true,
				},
				&cli.StringFlag{
					Name:  FlagDynamicConfigFilter,
					Usage: fmt.Sprintf(`Optional. ex: --%s '{"domainName":"global-samples-domain", "shardID":1, "isEnabled": true}'`, FlagDynamicConfigFilter),
				},
			},
			Action: AdminGetDynamicConfig,
		},
		{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "Update Dynamic Config Value",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     FlagDynamicConfigName,
					Usage:    "Name of Dynamic Config parameter to update value of",
					Required: true,
				},
				&cli.StringSliceFlag{
					Name:     FlagDynamicConfigValue,
					Usage:    fmt.Sprintf(`Can be specified multiple times for multiple values. ex: --%s '{"Value":true,"Filters":[]}'`, FlagDynamicConfigValue),
					Required: true,
				},
			},
			Action: AdminUpdateDynamicConfig,
		},
		{
			Name:    "restore",
			Aliases: []string{"r"},
			Usage:   "Restore Dynamic Config Value",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     FlagDynamicConfigName,
					Usage:    "Name of Dynamic Config parameter to restore",
					Required: true,
				},
				&cli.StringFlag{
					Name:  FlagDynamicConfigFilter,
					Usage: fmt.Sprintf(`Optional. ex: --%s '{"domainName":"global-samples-domain", "shardID":1, "isEnabled": true}'`, FlagDynamicConfigFilter),
				},
			},
			Action: AdminRestoreDynamicConfig,
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "List Dynamic Config Value",
			Flags:   []cli.Flag{},
			Action:  AdminListDynamicConfig,
		},
		{
			Name:    "listall",
			Aliases: []string{"la"},
			Usage:   "List all available configuration keys",
			Flags:   []cli.Flag{getFormatFlag()},
			Action:  AdminListConfigKeys,
		},
	}
}

func newAdminIsolationGroupCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:  "get-global",
			Usage: "gets the global isolation groups",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  FlagFormat,
					Usage: `output format`,
				},
			},
			Action: AdminGetGlobalIsolationGroups,
		},
		{
			Name:  "update-global",
			Usage: "sets the global isolation groups",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     FlagJSON,
					Usage:    `the configurations to upsert: eg: [{"Name": "zone-1": "State": 2}]. To remove groups, specify an empty configuration`,
					Required: false,
				},
				&cli.StringSliceFlag{
					Name:     FlagIsolationGroupSetDrains,
					Usage:    "Use to upsert the configuration for all drains. Note that this is an upsert operation and will overwrite all existing configuration",
					Required: false,
				},
				&cli.BoolFlag{
					Name:     FlagIsolationGroupsRemoveAllDrains,
					Usage:    "Removes all drains",
					Required: false,
				},
			},
			Action: AdminUpdateGlobalIsolationGroups,
		},
		{
			Name:  "get-domain",
			Usage: "gets the domain isolation groups",
			Flags: []cli.Flag{

				&cli.StringFlag{
					Name:  FlagFormat,
					Usage: `output format`,
				},
			},
			Action: AdminGetDomainIsolationGroups,
		},
		{
			Name:  "update-domain",
			Usage: "sets the domain isolation groups",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     FlagJSON,
					Usage:    `the configurations to upsert: eg: [{"Name": "zone-1": "State": 2}]. To remove groups, specify an empty configuration`,
					Required: false,
				},
				&cli.StringSliceFlag{
					Name:     FlagIsolationGroupSetDrains,
					Usage:    "Use to upsert the configuration for all drains. Note that this is an upsert operation and will overwrite all existing configuration",
					Required: false,
				},
				&cli.BoolFlag{
					Name:     FlagIsolationGroupsRemoveAllDrains,
					Usage:    "Removes all drains",
					Required: false,
				},
			},
			Action: AdminUpdateDomainIsolationGroups,
		},
	}
}
