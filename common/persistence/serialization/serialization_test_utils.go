package serialization

import (
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

var shardInfoTestData = &ShardInfo{
	StolenSinceRenew:                      1,
	UpdatedAt:                             time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	ReplicationAckLevel:                   1,
	TransferAckLevel:                      1,
	TimerAckLevel:                         time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	DomainNotificationVersion:             1,
	ClusterTransferAckLevel:               map[string]int64{"test": 1},
	ClusterTimerAckLevel:                  map[string]time.Time{"test": time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)},
	TransferProcessingQueueStates:         []byte{1, 2, 3},
	TimerProcessingQueueStates:            []byte{1, 2, 3},
	Owner:                                 "owner",
	ClusterReplicationLevel:               map[string]int64{"test": 1},
	PendingFailoverMarkers:                []byte{2, 3, 4},
	PendingFailoverMarkersEncoding:        "",
	TransferProcessingQueueStatesEncoding: "",
	TimerProcessingQueueStatesEncoding:    "",
}

var domainInfoTestData = &DomainInfo{
	Description:                 "test_desc",
	Owner:                       "test_owner",
	Status:                      1,
	Retention:                   48 * time.Hour,
	EmitMetric:                  true,
	ArchivalBucket:              "test_bucket",
	ArchivalStatus:              1,
	ConfigVersion:               1,
	FailoverVersion:             1,
	NotificationVersion:         1,
	FailoverNotificationVersion: 1,
	ActiveClusterName:           "test_active_cluster",
	Clusters:                    []string{"test_active_cluster", "test_standby_cluster"},
	Data:                        map[string]string{"test_key": "test_value"},
	BadBinaries:                 []byte{1, 2, 3},
	BadBinariesEncoding:         "",
	HistoryArchivalStatus:       1,
	HistoryArchivalURI:          "test_history_archival_uri",
	VisibilityArchivalStatus:    1,
	VisibilityArchivalURI:       "test_visibility_archival_uri",
}

var historyTreeInfoTestData = &HistoryTreeInfo{
	CreatedTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	Ancestors: []*types.HistoryBranchRange{
		{
			BranchID: "test_branch_id1",
		},
	},
}

var workflowExecutionInfoTestData = &WorkflowExecutionInfo{
	ParentDomainID:                     MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	ParentWorkflowID:                   "test_parent_workflow_id",
	ParentRunID:                        MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	InitiatedID:                        1,
	CompletionEventBatchID:             common.Int64Ptr(2),
	CompletionEvent:                    []byte("test_completion_event"),
	CompletionEventEncoding:            "test_completion_event_encoding",
	TaskList:                           "test_task_list",
	WorkflowTypeName:                   "test_workflow_type",
	WorkflowTimeout:                    10 * time.Second,
	DecisionTaskTimeout:                5 * time.Second,
	ExecutionContext:                   []byte("test_execution_context"),
	State:                              1,
	CloseStatus:                        1,
	StartVersion:                       1,
	LastWriteEventID:                   common.Int64Ptr(3),
	LastEventTaskID:                    4,
	LastFirstEventID:                   5,
	LastProcessedEvent:                 6,
	StartTimestamp:                     time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	LastUpdatedTimestamp:               time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	CreateRequestID:                    "test_create_request_id",
	DecisionVersion:                    7,
	DecisionScheduleID:                 8,
	DecisionStartedID:                  9,
	DecisionRequestID:                  "test_decision_request_id",
	DecisionTimeout:                    3 * time.Second,
	DecisionAttempt:                    10,
	DecisionStartedTimestamp:           time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	DecisionScheduledTimestamp:         time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	DecisionOriginalScheduledTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	CancelRequested:                    true,
	CancelRequestID:                    "test_cancel_request_id",
	StickyTaskList:                     "test_sticky_task_list",
	StickyScheduleToStartTimeout:       2 * time.Second,
	RetryAttempt:                       11,
	RetryInitialInterval:               1 * time.Second,
	RetryMaximumInterval:               30 * time.Second,
	RetryMaximumAttempts:               3,
	RetryExpiration:                    time.Hour,
	RetryBackoffCoefficient:            2.0,
	RetryExpirationTimestamp:           time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	RetryNonRetryableErrors:            []string{"test_error"},
	HasRetryPolicy:                     true,
	CronSchedule:                       "test_cron",
	IsCron:                             true,
	EventStoreVersion:                  12,
	EventBranchToken:                   []byte("test_branch_token"),
	SignalCount:                        13,
	HistorySize:                        14,
	ClientLibraryVersion:               "test_client_version",
	ClientFeatureVersion:               "test_feature_version",
	ClientImpl:                         "test_client_impl",
	AutoResetPoints:                    []byte("test_reset_points"),
	AutoResetPointsEncoding:            "test_reset_points_encoding",
	SearchAttributes:                   map[string][]byte{"test_key": []byte("test_value")},
	Memo:                               map[string][]byte{"test_memo": []byte("test_memo_value")},
	VersionHistories:                   []byte("test_version_histories"),
	VersionHistoriesEncoding:           "test_version_histories_encoding",
	FirstExecutionRunID:                MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
}

var activityInfoTestData = &ActivityInfo{
	Version:                  1,
	ScheduledEventBatchID:    2,
	ScheduledEvent:           []byte("test_scheduled_event"),
	ScheduledEventEncoding:   "test_scheduled_encoding",
	ScheduledTimestamp:       time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	StartedID:                3,
	StartedEvent:             []byte("test_started_event"),
	StartedEventEncoding:     "test_started_encoding",
	StartedTimestamp:         time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	ActivityID:               "test_activity_id",
	RequestID:                "test_request_id",
	ScheduleToStartTimeout:   5 * time.Second,
	ScheduleToCloseTimeout:   10 * time.Second,
	StartToCloseTimeout:      8 * time.Second,
	HeartbeatTimeout:         3 * time.Second,
	CancelRequested:          true,
	CancelRequestID:          4,
	TimerTaskStatus:          5,
	Attempt:                  6,
	TaskList:                 "test_task_list",
	StartedIdentity:          "test_identity",
	HasRetryPolicy:           true,
	RetryInitialInterval:     1 * time.Second,
	RetryMaximumInterval:     30 * time.Second,
	RetryMaximumAttempts:     3,
	RetryExpirationTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	RetryBackoffCoefficient:  2.0,
	RetryNonRetryableErrors:  []string{"test_error"},
	RetryLastFailureReason:   "test_failure_reason",
	RetryLastWorkerIdentity:  "test_worker_identity",
	RetryLastFailureDetails:  []byte("test_failure_details"),
}

var childExecutionInfoTestData = &ChildExecutionInfo{
	Version:                1,
	InitiatedEventBatchID:  2,
	StartedID:              3,
	InitiatedEvent:         []byte("test_initiated_event"),
	InitiatedEventEncoding: "test_initiated_encoding",
	StartedWorkflowID:      "test_started_workflow_id",
	StartedRunID:           MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	StartedEvent:           []byte("test_started_event"),
	StartedEventEncoding:   "test_started_encoding",
	CreateRequestID:        "test_create_request_id",
	DomainID:               "test_domain_id",
	DomainNameDEPRECATED:   "test_domain_name",
	WorkflowTypeName:       "test_workflow_type",
	ParentClosePolicy:      4,
}

var signalInfoTestData = &SignalInfo{
	Version:               1,
	InitiatedEventBatchID: 2,
	RequestID:             "test_request_id",
	Name:                  "test_signal_name",
	Input:                 []byte("test_input"),
	Control:               []byte("test_control"),
}

var requestCancelInfoTestData = &RequestCancelInfo{
	Version:               1,
	InitiatedEventBatchID: 2,
	CancelRequestID:       "test_cancel_request_id",
}

var timerInfoTestData = &TimerInfo{
	Version:         1,
	StartedID:       2,
	ExpiryTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	TaskID:          3,
}

var taskInfoTestData = &TaskInfo{
	WorkflowID:       "test_workflow_id",
	RunID:            MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	ScheduleID:       1,
	ExpiryTimestamp:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	CreatedTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	PartitionConfig:  map[string]string{"test_key": "test_value"},
}

var taskListInfoTestData = &TaskListInfo{
	Kind:            1,
	AckLevel:        2,
	ExpiryTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	LastUpdated:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
}

var transferTaskInfoTestData = &TransferTaskInfo{
	DomainID:                MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	WorkflowID:              "test_workflow_id",
	RunID:                   MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	TaskType:                1,
	TargetDomainID:          MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	TargetDomainIDs:         []UUID{MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8")},
	TargetWorkflowID:        "test_target_workflow_id",
	TargetRunID:             MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	TaskList:                "test_task_list",
	TargetChildWorkflowOnly: true,
	ScheduleID:              2,
	Version:                 3,
	VisibilityTimestamp:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
}

var timerTaskInfoTestData = &TimerTaskInfo{
	DomainID:        MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	WorkflowID:      "test_workflow_id",
	RunID:           MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	TaskType:        1,
	TimeoutType:     common.Int16Ptr(2),
	Version:         3,
	ScheduleAttempt: 4,
	EventID:         5,
}

var replicationTaskInfoTestData = &ReplicationTaskInfo{
	DomainID:                MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	WorkflowID:              "test_workflow_id",
	RunID:                   MustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	TaskType:                1,
	Version:                 2,
	FirstEventID:            3,
	NextEventID:             4,
	ScheduledID:             5,
	EventStoreVersion:       6,
	NewRunEventStoreVersion: 7,
	BranchToken:             []byte("test_branch_token"),
	NewRunBranchToken:       []byte("test_new_run_branch_token"),
	CreationTimestamp:       time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
}
