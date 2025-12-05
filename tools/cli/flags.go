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

import "github.com/urfave/cli/v2"

// Flags used to specify cli command line arguments
const (
	FlagVerbose                        = "verbose"
	FlagUsername                       = "username"
	FlagPassword                       = "password"
	FlagKeyspace                       = "keyspace"
	FlagDatabaseName                   = "db_name"
	FlagEncodingType                   = "encoding_type"
	FlagDecodingTypes                  = "decoding_types"
	FlagAddress                        = "address"
	FlagDestinationAddress             = "destination_address"
	FlagHistoryAddress                 = "history_address"
	FlagDBType                         = "db_type"
	FlagDBAddress                      = "db_address"
	FlagDBPort                         = "db_port"
	FlagDBRegion                       = "db_region"
	FlagDBShard                        = "db_shard"
	FlagProtoVersion                   = "protocol_version"
	FlagDomainID                       = "domain_id"
	FlagDomain                         = "domain"
	FlagDestinationDomain              = "destination_domain"
	FlagShardID                        = "shard_id"
	FlagShards                         = "shards"
	FlagRangeID                        = "range_id"
	FlagWorkflowID                     = "workflow_id"
	FlagRunID                          = "run_id"
	FlagTreeID                         = "tree_id"
	FlagBranchID                       = "branch_id"
	FlagNumberOfShards                 = "number_of_shards"
	FlagTargetCluster                  = "target_cluster"
	FlagSourceCluster                  = "source_cluster"
	FlagMinEventID                     = "min_event_id"
	FlagMaxEventID                     = "max_event_id"
	FlagEndEventVersion                = "end_event_version"
	FlagTaskList                       = "tasklist"
	FlagTaskListType                   = "tasklisttype"
	FlagWorkflowIDReusePolicy          = "workflowidreusepolicy"
	FlagCronSchedule                   = "cron"
	FlagWorkflowType                   = "workflow_type"
	FlagWorkflowStatus                 = "status"
	FlagExecutionTimeout               = "execution_timeout"
	FlagDecisionTimeout                = "decision_timeout"
	FlagContextTimeout                 = "context_timeout"
	FlagInput                          = "input"
	FlagInputFile                      = "input_file"
	FlagInputEncoding                  = "encoding"
	FlagSignalInput                    = "signal_input"
	FlagSignalInputFile                = "signal_input_file"
	FlagExcludeFile                    = "exclude_file"
	FlagInputSeparator                 = "input_separator"
	FlagParallelism                    = "input_parallelism"
	FlagParallismDeprecated            = "input_parallism" // typo, replaced by FlagParallelism
	FlagScanType                       = "scan_type"
	FlagInvariantCollection            = "invariant_collection"
	FlagSkipCurrentOpen                = "skip_current_open"
	FlagSkipCurrentCompleted           = "skip_current_completed"
	FlagSkipBaseIsNotCurrent           = "skip_base_is_not_current"
	FlagDryRun                         = "dry_run"
	FlagNonDeterministicOnly           = "only_non_deterministic"
	FlagInputTopic                     = "input_topic"
	FlagHostFile                       = "host_file"
	FlagCluster                        = "cluster"
	FlagInputCluster                   = "input_cluster"
	FlagStartOffset                    = "start_offset"
	FlagTopic                          = "topic"
	FlagGroup                          = "group"
	FlagResult                         = "result"
	FlagIdentity                       = "identity"
	FlagDetail                         = "detail"
	FlagReason                         = "reason"
	FlagOpen                           = "open"
	FlagMore                           = "more"
	FlagAll                            = "all"
	FlagPrefix                         = "prefix"
	FlagDeprecated                     = "deprecated"
	FlagForce                          = "force"
	FlagPageID                         = "page_id"
	FlagPageSize                       = "pagesize"
	FlagLimit                          = "limit"
	FlagEarliestTime                   = "earliest_time"
	FlagLatestTime                     = "latest_time"
	FlagPrintEventVersion              = "print_event_version"
	FlagPrintFullyDetail               = "print_full"
	FlagPrintRawTime                   = "print_raw_time"
	FlagPrintRaw                       = "print_raw"
	FlagPrintDateTime                  = "print_datetime"
	FlagPrintMemo                      = "print_memo"
	FlagPrintSearchAttr                = "print_search_attr"
	FlagPrintJSON                      = "print_json" // Deprecated: use --format json
	FlagDescription                    = "description"
	FlagOwnerEmail                     = "owner_email"
	FlagRetentionDays                  = "retention"
	FlagHistoryArchivalStatus          = "history_archival_status"
	FlagHistoryArchivalURI             = "history_uri"
	FlagVisibilityArchivalStatus       = "visibility_archival_status"
	FlagVisibilityArchivalURI          = "visibility_uri"
	FlagName                           = "name"
	FlagOutputFilename                 = "output_filename"
	FlagOutputFormat                   = "output"
	FlagQueryType                      = "query_type"
	FlagQueryRejectCondition           = "query_reject_condition"
	FlagQueryConsistencyLevel          = "query_consistency_level"
	FlagShowDetail                     = "show_detail"
	FlagActiveClusterName              = "active_cluster"
	FlagActiveClusters                 = "active_clusters"
	FlagActiveClustersJSON             = "active_clusters_json"
	FlagClusters                       = "clusters"
	FlagFailoverReason                 = "reason"
	FlagIsGlobalDomain                 = "global_domain" // active-passive domain
	FlagDomainData                     = "domain_data"
	FlagEventID                        = "event_id"
	FlagActivityID                     = "activity_id"
	FlagMaxFieldLength                 = "max_field_length"
	FlagSecurityToken                  = "security_token"
	FlagSkipErrorMode                  = "skip_errors"
	FlagRemote                         = "remote"
	FlagTimerType                      = "timer_type"
	FlagHeadersMode                    = "headers"
	FlagMessageType                    = "message_type"
	FlagURL                            = "url"
	FlagIndex                          = "index"
	FlagBatchSize                      = "batch_size"
	FlagMemoKey                        = "memo_key"
	FlagMemo                           = "memo"
	FlagMemoFile                       = "memo_file"
	FlagSearchAttributesKey            = "search_attr_key"
	FlagSearchAttributesVal            = "search_attr_value"
	FlagSearchAttributesType           = "search_attr_type"
	FlagAddBadBinary                   = "add_bad_binary"
	FlagRemoveBadBinary                = "remove_bad_binary"
	FlagResetType                      = "reset_type"
	FlagDecisionOffset                 = "decision_offset"
	FlagResetPointsOnly                = "reset_points_only"
	FlagResetBadBinaryChecksum         = "reset_bad_binary_checksum"
	FlagSkipSignalReapply              = "skip_signal_reapply"
	FlagListQuery                      = "query"
	FlagExcludeWorkflowIDByQuery       = "exclude_query"
	FlagBatchType                      = "batch_type"
	FlagSignalName                     = "signal_name"
	FlagTaskID                         = "task_id"
	FlagTaskType                       = "task_type"
	FlagTaskVisibilityTimestamp        = "task_timestamp"
	FlagQueueType                      = "queue_type"
	FlagStartingRPS                    = "starting_rps"
	FlagRPS                            = "rps"
	FlagRPSScaleUpSeconds              = "rps_scale_up_seconds"
	FlagJobID                          = "job_id"
	FlagYes                            = "yes"
	FlagServiceConfigDir               = "service_config_dir"
	FlagServiceEnv                     = "service_env"
	FlagServiceZone                    = "service_zone"
	FlagEnableTLS                      = "tls"
	FlagTLSCertPath                    = "tls_cert_path"
	FlagTLSKeyPath                     = "tls_key_path"
	FlagTLSCaPath                      = "tls_ca_path"
	FlagTLSEnableHostVerification      = "tls_enable_host_verification"
	FlagDLQType                        = "dlq_type"
	FlagMaxMessageCount                = "max_message_count"
	FlagLastMessageID                  = "last_message_id"
	FlagConcurrency                    = "concurrency"
	FlagReportRate                     = "report_rate"
	FlagLowerShardBound                = "lower_shard_bound"
	FlagUpperShardBound                = "upper_shard_bound"
	FlagInputDirectory                 = "input_directory"
	FlagSkipHistoryChecks              = "skip_history_checks"
	FlagFailoverType                   = "failover_type"
	FlagFailoverTimeout                = "failover_timeout_seconds"
	FlagActivityHeartBeatTimeout       = "heart_beat_timeout_seconds"
	FlagFailoverWaitTime               = "failover_wait_time_second"
	FlagFailoverBatchSize              = "failover_batch_size"
	FlagFailoverDomains                = "domains"
	FlagFailoverDrillWaitTime          = "failover_drill_wait_second"
	FlagFailoverDrill                  = "failover_drill"
	FlagRetryInterval                  = "retry_interval"
	FlagRetryAttempts                  = "retry_attempts"
	FlagMaxActivityRetries             = "max_activity_retries"
	FlagRetryExpiration                = "retry_expiration"
	FlagRetryBackoff                   = "retry_backoff"
	FlagRetryMaxInterval               = "retry_max_interval"
	FlagHeaderKey                      = "header_key"
	FlagHeaderValue                    = "header_value"
	FlagHeaderFile                     = "header_file"
	FlagStartDate                      = "start_date"
	FlagEndDate                        = "end_date"
	FlagDateFormat                     = "date_format"
	FlagShardMultiplier                = "shard_multiplier"
	FlagBucketSize                     = "bucket_size"
	DelayStartSeconds                  = "delay_start_seconds"
	JitterStartSeconds                 = "jitter_start_seconds"
	FirstRunAtTime                     = "first_run_at_time"
	FlagConnectionAttributes           = "conn_attrs"
	FlagJWT                            = "jwt"
	FlagJWTPrivateKey                  = "jwt-private-key"
	FlagDynamicConfigName              = "name"
	FlagDynamicConfigFilter            = "filter"
	FlagDynamicConfigValue             = "value"
	FlagTransport                      = "transport"
	FlagFormat                         = "format"
	FlagJSON                           = "json"
	FlagIsolationGroupSetDrains        = "set-drains"
	FlagIsolationGroupsRemoveAllDrains = "remove-all-drains"
	FlagSearchAttribute                = "search_attr"
	FlagNumReadPartitions              = "num_read_partitions"
	FlagNumWritePartitions             = "num_write_partitions"
	FlagCronOverlapPolicy              = "cron_overlap_policy"
	FlagClusterAttributeScope          = "cluster_attribute_scope"
	FlagClusterAttributeName           = "cluster_attribute_name"

	FlagClustersUsage = "Clusters (example: --clusters clusterA,clusterB or --cl clusterA --cl clusterB)"
)

var flagsForExecution = []cli.Flag{
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
}

var flagsOfExecutionForShow = []cli.Flag{
	&cli.StringFlag{
		Name:    FlagWorkflowID,
		Aliases: []string{"w", "wid"},
		Usage:   "WorkflowID",
	},
	&cli.StringFlag{
		Name:    FlagRunID,
		Aliases: []string{"r", "rid"},
		Usage:   "RunID, required for archived history",
	},
}

func getFlagsForShow() []cli.Flag {
	return append(flagsOfExecutionForShow, getFlagsForShowID()...)
}

func getFlagsForShowID() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:    FlagPrintDateTime,
			Aliases: []string{"pdt"},
			Usage:   "Print timestamp",
		},
		&cli.BoolFlag{
			Name:    FlagPrintRawTime,
			Aliases: []string{"prt"},
			Usage:   "Print raw timestamp",
		},
		&cli.StringFlag{
			Name:    FlagOutputFilename,
			Aliases: []string{"of"},
			Usage:   "Serialize history event to a file",
		},
		&cli.BoolFlag{
			Name:    FlagPrintFullyDetail,
			Aliases: []string{"pf"},
			Usage:   "Print fully event detail",
		},
		&cli.BoolFlag{
			Name:    FlagPrintEventVersion,
			Aliases: []string{"pev"},
			Usage:   "Print event version",
		},
		&cli.IntFlag{
			Name:    FlagEventID,
			Aliases: []string{"eid"},
			Usage:   "Print specific event details",
		},
		&cli.IntFlag{
			Name:    FlagMaxFieldLength,
			Aliases: []string{"maxl"},
			Usage:   "Maximum length for each attribute field",
			Value:   defaultMaxFieldLength,
		},
		&cli.BoolFlag{
			Name:  FlagResetPointsOnly,
			Usage: "Only show events that are eligible for reset",
		},
		&cli.StringFlag{
			Name:    FlagQueryConsistencyLevel,
			Aliases: []string{"qcl"},
			Usage:   "Optional flag to set query consistency level. Valid values are \"eventual\" and \"strong\"",
		},
	}
}

func getFlagsForStart() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    FlagTaskList,
			Aliases: []string{"tl"},
			Usage:   "TaskList",
		},
		&cli.StringFlag{
			Name:    FlagWorkflowID,
			Aliases: []string{"w", "wid"},
			Usage:   "WorkflowID",
		},
		&cli.StringFlag{
			Name:    FlagWorkflowType,
			Aliases: []string{"wt"},
			Usage:   "WorkflowTypeName",
		},
		&cli.IntFlag{
			Name:    FlagExecutionTimeout,
			Aliases: []string{"et"},
			Usage:   "Execution start to close timeout in seconds",
		},
		&cli.IntFlag{
			Name:    FlagDecisionTimeout,
			Aliases: []string{"dt"},
			Value:   defaultDecisionTimeoutInSeconds,
			Usage:   "Decision task start to close timeout in seconds",
		},
		&cli.StringFlag{
			Name: FlagCronSchedule,
			Usage: "Optional cron schedule for the workflow. Cron spec is as following: \n" +
				"\t┌───────────── minute (0 - 59) \n" +
				"\t│ ┌───────────── hour (0 - 23) \n" +
				"\t│ │ ┌───────────── day of the month (1 - 31) \n" +
				"\t│ │ │ ┌───────────── month (1 - 12) \n" +
				"\t│ │ │ │ ┌───────────── day of the week (0 - 6) (Sunday to Saturday) \n" +
				"\t│ │ │ │ │ \n" +
				"\t* * * * *",
		},
		&cli.IntFlag{
			Name:    FlagCronOverlapPolicy,
			Aliases: []string{"cop"},
			Usage: "Optional cron overlap policy for the workflow when a cron run overlaps with next scheduled run. " +
				"Available options: 0: Skip running if cron run overlaps, 1: Start new run immediately if previous run overlaps and completes",
		},
		&cli.IntFlag{
			Name:    FlagWorkflowIDReusePolicy,
			Aliases: []string{"wrp"},
			Usage: "Optional input to configure if the same workflow ID is allow to use for new workflow execution. " +
				"Available options: 0: AllowDuplicateFailedOnly, 1: AllowDuplicate, 2: RejectDuplicate, 3:TerminateIfRunning",
		},
		&cli.StringFlag{
			Name:    FlagInput,
			Aliases: []string{"i"},
			Usage:   "Optional input for the workflow, in JSON format. If there are multiple parameters, concatenate them and separate by space.",
		},
		&cli.StringFlag{
			Name:    FlagInputFile,
			Aliases: []string{"if"},
			Usage: "Optional input for the workflow from JSON file. If there are multiple JSON, concatenate them and separate by space or newline. " +
				"Input from file will be overwrite by input from command line",
		},
		&cli.StringFlag{
			Name:  FlagMemoKey,
			Usage: "Optional key of memo. If there are multiple keys, concatenate them and separate by space",
		},
		&cli.StringFlag{
			Name: FlagMemo,
			Usage: "Optional info that can be showed when list workflow, in JSON format. If there are multiple JSON, concatenate them and separate by space. " +
				"The order must be same as memo_key",
		},
		&cli.StringFlag{
			Name: FlagMemoFile,
			Usage: "Optional info that can be listed in list workflow, from JSON format file. If there are multiple JSON, concatenate them and separate by space or newline. " +
				"The order must be same as memo_key",
		},
		&cli.StringFlag{
			Name:  FlagHeaderKey,
			Usage: "Optional key of header. If there are multiple keys, concatenate them and separate by space",
		},
		&cli.StringFlag{
			Name: FlagHeaderValue,
			Usage: "Optional info to propogate via workflow context, in JSON format. If there are multiple JSON, concatenate them and separate by space. " +
				"The order must be same as " + FlagHeaderKey,
		},
		&cli.StringFlag{
			Name: FlagHeaderFile,
			Usage: "Optional info to propogate via workflow context, from JSON format file. If there are multiple JSON, concatenate them and separate by space or newline. " +
				"The order must be same as " + FlagHeaderKey,
		},
		&cli.StringFlag{
			Name: FlagSearchAttributesKey,
			Usage: "Optional search attributes keys that can be be used in list query. If there are multiple keys, concatenate them and separate by |. " +
				"Use 'cluster get-search-attr' cmd to list legal keys.",
		},
		&cli.StringFlag{
			Name: FlagSearchAttributesVal,
			Usage: "Optional search attributes value that can be be used in list query. If there are multiple keys, concatenate them and separate by |. " +
				"If value is array, use json array like [\"a\",\"b\"], [1,2], [\"true\",\"false\"], [\"2019-06-07T17:16:34-08:00\",\"2019-06-07T18:16:34-08:00\"]. " +
				"Use 'cluster get-search-attr' cmd to list legal keys and value types",
		},
		&cli.IntFlag{
			Name:  FlagRetryExpiration,
			Usage: "Optional retry expiration in seconds. If set workflow will be retried for the specified period of time. retry_attempts and retry_expiration must not both be 0.",
		},
		&cli.IntFlag{
			Name:  FlagRetryAttempts,
			Usage: "Optional retry attempts. If set workflow will be retried the specified amount of times. retry_attempts and retry_expiration must not both be 0.",
		},
		&cli.IntFlag{
			Name:  FlagRetryInterval,
			Value: 10,
			Usage: "Optional retry interval in seconds.",
		},
		&cli.Float64Flag{
			Name:  FlagRetryBackoff,
			Value: 1.0,
			Usage: "Optional retry backoff coefficient. Must be or equal or greater than 1.",
		},
		&cli.IntFlag{
			Name:  FlagRetryMaxInterval,
			Usage: "Optional retry maximum interval in seconds. If set will give an upper bound for retry interval. Must be equal or greater than retry interval.",
		},
		&cli.IntFlag{
			Name:  DelayStartSeconds,
			Usage: "Optional workflow start delay in seconds. If set workflow start will be delayed this many seconds",
		},
		&cli.IntFlag{
			Name:  JitterStartSeconds,
			Usage: "Optional workflow start jitter in seconds. If set, workflow start will be jittered between 0-n seconds (after delay)",
		},
		&cli.StringFlag{
			Name:  FirstRunAtTime,
			Usage: "Optional workflow's first run start time in RFC3339 format, like \"1970-01-01T00:00:00Z\". If set, first run of the workflow will start at the specified time.",
		},
		&cli.StringFlag{
			Name:    FlagClusterAttributeScope,
			Usage:   "Optional cluster attribute to specify how to select the active cluster. Examples might be 'region' or 'location'",
			Aliases: []string{"cascope"},
		},
		&cli.StringFlag{
			Name:    FlagClusterAttributeName,
			Usage:   "Optional cluster attribute name, paired with a cluster attribute scope, to specify how to select the active cluster. This specifies which attribute to tie the workflow to, for example, if the scope is 'region' and the name is 'Lisbon' or 'San Francisco'",
			Aliases: []string{"caname"},
		},
	}
}

func getFlagsForRun() []cli.Flag {
	flagsForRun := []cli.Flag{
		&cli.BoolFlag{
			Name:    FlagShowDetail,
			Aliases: []string{"sd"},
			Usage:   "Show event details",
		},
		&cli.IntFlag{
			Name:    FlagMaxFieldLength,
			Aliases: []string{"maxl"},
			Usage:   "Maximum length for each attribute field",
		},
	}
	flagsForRun = append(getFlagsForStart(), flagsForRun...)
	return flagsForRun
}

func getFlagsForSignal() []cli.Flag {
	return []cli.Flag{
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
			Name:    FlagName,
			Aliases: []string{"n"},
			Usage:   "SignalName",
		},
		&cli.StringFlag{
			Name:    FlagInput,
			Aliases: []string{"i"},
			Usage:   "Input for the signal, in JSON format.",
		},
		&cli.StringFlag{
			Name:    FlagInputFile,
			Aliases: []string{"if"},
			Usage:   "Input for the signal from JSON file.",
		},
	}
}

func getFlagsForSignalWithStart() []cli.Flag {
	return append(getFlagsForStart(),
		&cli.StringFlag{
			Name:    FlagName,
			Aliases: []string{"n"},
			Usage:   "SignalName",
		},
		&cli.StringFlag{
			Name:    FlagSignalInput,
			Aliases: []string{"si"},
			Usage:   "Input for the signal, in JSON format.",
		},
		&cli.StringFlag{
			Name:    FlagSignalInputFile,
			Aliases: []string{"sif"},
			Usage:   "Input for the signal from JSON file.",
		})
}

func getFlagsForTerminate() []cli.Flag {
	return append(flagsForExecution, &cli.StringFlag{
		Name:    FlagReason,
		Aliases: []string{"re"},
		Usage:   "The reason you want to terminate the workflow",
	})
}

func getFlagsForCancel() []cli.Flag {
	return append(flagsForExecution, &cli.StringFlag{
		Name:    FlagReason,
		Aliases: []string{"re"},
		Usage:   "The reason you want to cancel the workflow",
	})
}

func getFormatFlag() cli.Flag {
	return &cli.StringFlag{
		Name:  FlagFormat,
		Usage: "Format [table|json|<template>]; Use GoLang \"text/template\" syntax to format the output.",
	}
}

func getCommonFlagsForVisibility() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:    FlagPrintRawTime,
			Aliases: []string{"prt"},
			Usage:   "Print raw timestamp",
		},
		&cli.BoolFlag{
			Name:    FlagPrintDateTime,
			Aliases: []string{"pdt"},
			Usage:   "Print full date time in '2006-01-02T15:04:05Z07:00' format",
		},
		&cli.BoolFlag{
			Name:    FlagPrintMemo,
			Aliases: []string{"pme"},
			Usage:   "Print memo",
		},
		&cli.BoolFlag{
			Name:    FlagPrintSearchAttr,
			Aliases: []string{"psa"},
			Usage:   "Print search attributes",
		},
		&cli.BoolFlag{
			Name:    FlagPrintFullyDetail,
			Aliases: []string{"pf"},
			Usage:   "Print full message without table format",
		},
		&cli.BoolFlag{
			Name:    FlagPrintJSON,
			Aliases: []string{"pjson"},
			Usage:   "Print in raw json format (DEPRECATED: instead use --format json)",
		},
		getFormatFlag(),
	}
}

func getFlagsForList() []cli.Flag {
	flagsForList := []cli.Flag{
		&cli.BoolFlag{
			Name:    FlagMore,
			Aliases: []string{"m"},
			Usage:   "List more pages, default is to list one page of default page size 10",
		},
		&cli.IntFlag{
			Name:    FlagPageSize,
			Aliases: []string{"ps"},
			Value:   10,
			Usage:   "Result page size",
		},
	}
	flagsForList = append(getFlagsForListAll(), flagsForList...)
	return flagsForList
}

func getFlagsForListAll() []cli.Flag {
	flagsForListAll := []cli.Flag{
		&cli.BoolFlag{
			Name:    FlagOpen,
			Aliases: []string{"op"},
			Usage:   "List for open workflow executions, default is to list for closed ones",
		},
		&cli.StringFlag{
			Name:    FlagEarliestTime,
			Aliases: []string{"et"},
			Usage: "EarliestTime of start time, supported formats are '2006-01-02T15:04:05+07:00', raw UnixNano and " +
				"time range (N<duration>), where 0 < N < 1000000 and duration (full-notation/short-notation) can be second/s, " +
				"minute/m, hour/h, day/d, week/w, month/M or year/y. For example, '15minute' or '15m' implies last 15 minutes.",
		},
		&cli.StringFlag{
			Name:    FlagLatestTime,
			Aliases: []string{"lt"},
			Usage: "LatestTime of start time, supported formats are '2006-01-02T15:04:05+07:00', raw UnixNano and " +
				"time range (N<duration>), where 0 < N < 1000000 and duration (in full-notation/short-notation) can be second/s, " +
				"minute/m, hour/h, day/d, week/w, month/M or year/y. For example, '15minute' or '15m' implies last 15 minutes",
		},
		&cli.StringFlag{
			Name:    FlagWorkflowID,
			Aliases: []string{"wid", "w"},
			Usage:   "WorkflowID",
		},
		&cli.StringFlag{
			Name:    FlagWorkflowType,
			Aliases: []string{"wt"},
			Usage:   "WorkflowTypeName",
		},
		&cli.StringFlag{
			Name:    FlagWorkflowStatus,
			Aliases: []string{"s"},
			Usage:   "Closed workflow status [completed, failed, canceled, terminated, continued_as_new, timed_out]",
		},
		&cli.StringFlag{
			Name:    FlagListQuery,
			Aliases: []string{"q"},
			Usage: "Optional SQL like query for use of search attributes. NOTE: using query will ignore all other filter flags including: " +
				"[open, earliest_time, latest_time, workflow_id, workflow_type]",
		},
		&cli.StringFlag{
			Name: FlagExcludeWorkflowIDByQuery,
			Usage: "Another optional SQL like query, but for excluding the results by workflowIDs. This is useful because a single query cannot do join operation. One use case is to " +
				"find failed workflows excluding any workflow that has another run that is open or completed.",
		},
	}
	flagsForListAll = append(getCommonFlagsForVisibility(), flagsForListAll...)
	return flagsForListAll
}

func getFlagsForScan() []cli.Flag {
	flagsForScan := []cli.Flag{
		&cli.IntFlag{
			Name:    FlagPageSize,
			Aliases: []string{"ps"},
			Value:   2000,
			Usage:   "Page size for each Scan API call",
		},
		&cli.StringFlag{
			Name:    FlagListQuery,
			Aliases: []string{"q"},
			Usage:   "Optional SQL like query",
		},
	}
	flagsForScan = append(getCommonFlagsForVisibility(), flagsForScan...)
	return flagsForScan
}

func getFlagsForListArchived() []cli.Flag {
	flagsForListArchived := []cli.Flag{
		&cli.StringFlag{
			Name:    FlagListQuery,
			Aliases: []string{"q"},
			Usage:   "SQL like query. Please check the documentation of the visibility archiver used by your domain for detailed instructions",
		},
		&cli.IntFlag{
			Name:    FlagPageSize,
			Aliases: []string{"ps"},
			Value:   100,
			Usage:   "Count of visibility records included in a single page, default to 100",
		},
		&cli.BoolFlag{
			Name:    FlagAll,
			Aliases: []string{"a"},
			Usage:   "List all pages",
		},
	}
	flagsForListArchived = append(getCommonFlagsForVisibility(), flagsForListArchived...)
	return flagsForListArchived
}

func getFlagsForCount() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    FlagListQuery,
			Aliases: []string{"q"},
			Usage:   "Optional SQL like query. e.g count all open workflows 'CloseTime = missing'; 'WorkflowType=\"wtype\" and CloseTime > 0'",
		},
	}
}

func getFlagsForQuery() []cli.Flag {
	return []cli.Flag{
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
			Name:    FlagQueryType,
			Aliases: []string{"qt"},
			Usage:   "The query type you want to run",
		},
		&cli.StringFlag{
			Name:    FlagInput,
			Aliases: []string{"i"},
			Usage:   "Optional input for the query, in JSON format. If there are multiple parameters, concatenate them and separate by space.",
		},
		&cli.StringFlag{
			Name:    FlagInputFile,
			Aliases: []string{"if"},
			Usage: "Optional input for the query from JSON file. If there are multiple JSON, concatenate them and separate by space or newline. " +
				"Input from file will be overwrite by input from command line",
		},
		&cli.StringFlag{
			Name:    FlagQueryRejectCondition,
			Aliases: []string{"qrc"},
			Usage:   "Optional flag to reject queries based on workflow state. Valid values are \"not_open\" and \"not_completed_cleanly\"",
		},
		&cli.StringFlag{
			Name:    FlagQueryConsistencyLevel,
			Aliases: []string{"qcl"},
			Usage:   "Optional flag to set query consistency level. Valid values are \"eventual\" and \"strong\"",
		},
	}
}

// all flags of query except QueryType
func getFlagsForStack() []cli.Flag {
	flags := getFlagsForQuery()
	for i := 0; i < len(flags); i++ {
		if flags[i].Names()[0] == FlagQueryType {
			return append(flags[:i], flags[i+1:]...)
		}
	}
	return flags
}

func getFlagsForDescribe() []cli.Flag {
	return append(flagsForExecution, getFlagsForDescribeID()...)
}

func getFlagsForDescribeID() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:    FlagPrintRaw,
			Aliases: []string{"praw"},
			Usage:   "Print properties as they are stored",
		},
		&cli.BoolFlag{
			Name:  FlagResetPointsOnly,
			Usage: "Only show auto-reset points",
		},
		&cli.StringFlag{
			Name:    FlagQueryConsistencyLevel,
			Aliases: []string{"qcl"},
			Usage:   "Optional flag to set query consistency level. Valid values are \"eventual\" and \"strong\"",
		},
	}
}

func getFlagsForObserve() []cli.Flag {
	return append(flagsForExecution, getFlagsForObserveID()...)
}

func getFlagsForObserveID() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:    FlagShowDetail,
			Aliases: []string{"sd"},
			Usage:   "Optional show event details",
		},
		&cli.IntFlag{
			Name:    FlagMaxFieldLength,
			Aliases: []string{"maxl"},
			Usage:   "Optional maximum length for each attribute field when show details",
		},
	}
}
