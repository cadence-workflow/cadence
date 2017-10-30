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

// Code generated by thriftrw v1.6.0. DO NOT EDIT.
// @generated

package shared

import "go.uber.org/thriftrw/thriftreflect"

var ThriftModule = &thriftreflect.ThriftModule{Name: "shared", Package: "github.com/uber/cadence/.gen/go/shared", FilePath: "shared.thrift", SHA1: "d2ddf10df2c3cff520cd93292db93290acc53621", Raw: rawIDL}

const rawIDL = "// Copyright (c) 2017 Uber Technologies, Inc.\n//\n// Permission is hereby granted, free of charge, to any person obtaining a copy\n// of this software and associated documentation files (the \"Software\"), to deal\n// in the Software without restriction, including without limitation the rights\n// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell\n// copies of the Software, and to permit persons to whom the Software is\n// furnished to do so, subject to the following conditions:\n//\n// The above copyright notice and this permission notice shall be included in\n// all copies or substantial portions of the Software.\n//\n// THE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR\n// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,\n// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE\n// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER\n// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,\n// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN\n// THE SOFTWARE.\n\nnamespace java com.uber.cadence\n\nexception BadRequestError {\n  1: required string message\n}\n\nexception InternalServiceError {\n  1: required string message\n}\n\nexception DomainAlreadyExistsError {\n  1: required string message\n}\n\nexception WorkflowExecutionAlreadyStartedError {\n  10: optional string message\n  20: optional string startRequestId\n  30: optional string runId\n}\n\nexception EntityNotExistsError {\n  1: required string message\n}\n\nexception ServiceBusyError {\n  1: required string message\n}\n\nexception CancellationAlreadyRequestedError {\n  1: required string message\n}\n\nexception QueryFailedError {\n  1: required string message\n}\n\nenum DomainStatus {\n  REGISTERED,\n  DEPRECATED,\n  DELETED,\n}\n\nenum TimeoutType {\n  START_TO_CLOSE,\n  SCHEDULE_TO_START,\n  SCHEDULE_TO_CLOSE,\n  HEARTBEAT,\n}\n\nenum DecisionType {\n  ScheduleActivityTask,\n  RequestCancelActivityTask,\n  StartTimer,\n  CompleteWorkflowExecution,\n  FailWorkflowExecution,\n  CancelTimer,\n  CancelWorkflowExecution,\n  RequestCancelExternalWorkflowExecution,\n  RecordMarker,\n  ContinueAsNewWorkflowExecution,\n  StartChildWorkflowExecution,\n}\n\nenum EventType {\n  WorkflowExecutionStarted,\n  WorkflowExecutionCompleted,\n  WorkflowExecutionFailed,\n  WorkflowExecutionTimedOut,\n  DecisionTaskScheduled,\n  DecisionTaskStarted,\n  DecisionTaskCompleted,\n  DecisionTaskTimedOut\n  DecisionTaskFailed,\n  ActivityTaskScheduled,\n  ActivityTaskStarted,\n  ActivityTaskCompleted,\n  ActivityTaskFailed,\n  ActivityTaskTimedOut,\n  ActivityTaskCancelRequested,\n  RequestCancelActivityTaskFailed,\n  ActivityTaskCanceled,\n  TimerStarted,\n  TimerFired,\n  CancelTimerFailed,\n  TimerCanceled,\n  WorkflowExecutionCancelRequested,\n  WorkflowExecutionCanceled,\n  RequestCancelExternalWorkflowExecutionInitiated,\n  RequestCancelExternalWorkflowExecutionFailed,\n  ExternalWorkflowExecutionCancelRequested,\n  MarkerRecorded,\n  WorkflowExecutionSignaled,\n  WorkflowExecutionTerminated,\n  WorkflowExecutionContinuedAsNew,\n  StartChildWorkflowExecutionInitiated,\n  StartChildWorkflowExecutionFailed,\n  ChildWorkflowExecutionStarted,\n  ChildWorkflowExecutionCompleted,\n  ChildWorkflowExecutionFailed,\n  ChildWorkflowExecutionCanceled,\n  ChildWorkflowExecutionTimedOut,\n  ChildWorkflowExecutionTerminated,\n}\n\nenum DecisionTaskFailedCause {\n  UNHANDLED_DECISION,\n  BAD_SCHEDULE_ACTIVITY_ATTRIBUTES,\n  BAD_REQUEST_CANCEL_ACTIVITY_ATTRIBUTES,\n  BAD_START_TIMER_ATTRIBUTES,\n  BAD_CANCEL_TIMER_ATTRIBUTES,\n  BAD_RECORD_MARKER_ATTRIBUTES,\n  BAD_COMPLETE_WORKFLOW_EXECUTION_ATTRIBUTES,\n  BAD_FAIL_WORKFLOW_EXECUTION_ATTRIBUTES,\n  BAD_CANCEL_WORKFLOW_EXECUTION_ATTRIBUTES,\n  BAD_REQUEST_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_ATTRIBUTES,\n  BAD_CONTINUE_AS_NEW_ATTRIBUTES,\n  START_TIMER_DUPLICATE_ID,\n}\n\nenum CancelExternalWorkflowExecutionFailedCause {\n  UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION,\n}\n\nenum ChildWorkflowExecutionFailedCause {\n  WORKFLOW_ALREADY_RUNNING,\n}\n\nenum WorkflowExecutionCloseStatus {\n  COMPLETED,\n  FAILED,\n  CANCELED,\n  TERMINATED,\n  CONTINUED_AS_NEW,\n  TIMED_OUT,\n}\n\nenum ChildPolicy {\n  TERMINATE,\n  REQUEST_CANCEL,\n  ABANDON,\n}\n\nenum QueryTaskCompletedType {\n  COMPLETED,\n  FAILED,\n}\n\nstruct WorkflowType {\n  10: optional string name\n}\n\nstruct ActivityType {\n  10: optional string name\n}\n\nstruct TaskList {\n  10: optional string name\n}\n\nstruct WorkflowExecution {\n  10: optional string workflowId\n  20: optional string runId\n}\n\nstruct WorkflowExecutionInfo {\n  10: optional WorkflowExecution execution\n  20: optional WorkflowType type\n  30: optional i64 (js.type = \"Long\") startTime\n  40: optional i64 (js.type = \"Long\") closeTime\n  50: optional WorkflowExecutionCloseStatus closeStatus\n  60: optional i64 (js.type = \"Long\") historyLength\n}\n\nstruct ScheduleActivityTaskDecisionAttributes {\n  10: optional string activityId\n  20: optional ActivityType activityType\n  25: optional string domain\n  30: optional TaskList taskList\n  40: optional binary input\n  45: optional i32 scheduleToCloseTimeoutSeconds\n  50: optional i32 scheduleToStartTimeoutSeconds\n  55: optional i32 startToCloseTimeoutSeconds\n  60: optional i32 heartbeatTimeoutSeconds\n}\n\nstruct RequestCancelActivityTaskDecisionAttributes {\n  10: optional string activityId\n}\n\nstruct StartTimerDecisionAttributes {\n  10: optional string timerId\n  20: optional i64 (js.type = \"Long\") startToFireTimeoutSeconds\n}\n\nstruct CompleteWorkflowExecutionDecisionAttributes {\n  10: optional binary result\n}\n\nstruct FailWorkflowExecutionDecisionAttributes {\n  10: optional string reason\n  20: optional binary details\n}\n\nstruct CancelTimerDecisionAttributes {\n  10: optional string timerId\n}\n\nstruct CancelWorkflowExecutionDecisionAttributes {\n  10: optional binary details\n}\n\nstruct RequestCancelExternalWorkflowExecutionDecisionAttributes {\n  10: optional string domain\n  20: optional string workflowId\n  30: optional string runId\n  40: optional binary control\n}\n\nstruct RecordMarkerDecisionAttributes {\n  10: optional string markerName\n  20: optional binary details\n}\n\nstruct ContinueAsNewWorkflowExecutionDecisionAttributes {\n  10: optional WorkflowType workflowType\n  20: optional TaskList taskList\n  30: optional binary input\n  40: optional i32 executionStartToCloseTimeoutSeconds\n  50: optional i32 taskStartToCloseTimeoutSeconds\n}\n\nstruct StartChildWorkflowExecutionDecisionAttributes {\n  10: optional string domain\n  20: optional string workflowId\n  30: optional WorkflowType workflowType\n  40: optional TaskList taskList\n  50: optional binary input\n  60: optional i32 executionStartToCloseTimeoutSeconds\n  70: optional i32 taskStartToCloseTimeoutSeconds\n  80: optional ChildPolicy childPolicy\n  90: optional binary control\n}\n\nstruct Decision {\n  10:  optional DecisionType decisionType\n  20:  optional ScheduleActivityTaskDecisionAttributes scheduleActivityTaskDecisionAttributes\n  25:  optional StartTimerDecisionAttributes startTimerDecisionAttributes\n  30:  optional CompleteWorkflowExecutionDecisionAttributes completeWorkflowExecutionDecisionAttributes\n  35:  optional FailWorkflowExecutionDecisionAttributes failWorkflowExecutionDecisionAttributes\n  40:  optional RequestCancelActivityTaskDecisionAttributes requestCancelActivityTaskDecisionAttributes\n  50:  optional CancelTimerDecisionAttributes cancelTimerDecisionAttributes\n  60:  optional CancelWorkflowExecutionDecisionAttributes cancelWorkflowExecutionDecisionAttributes\n  70:  optional RequestCancelExternalWorkflowExecutionDecisionAttributes requestCancelExternalWorkflowExecutionDecisionAttributes\n  80:  optional RecordMarkerDecisionAttributes recordMarkerDecisionAttributes\n  90:  optional ContinueAsNewWorkflowExecutionDecisionAttributes continueAsNewWorkflowExecutionDecisionAttributes\n  100: optional StartChildWorkflowExecutionDecisionAttributes startChildWorkflowExecutionDecisionAttributes\n}\n\nstruct WorkflowExecutionStartedEventAttributes {\n  10: optional WorkflowType workflowType\n  20: optional TaskList taskList\n  30: optional binary input\n  40: optional i32 executionStartToCloseTimeoutSeconds\n  50: optional i32 taskStartToCloseTimeoutSeconds\n  60: optional string identity\n}\n\nstruct WorkflowExecutionCompletedEventAttributes {\n  10: optional binary result\n  20: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n}\n\nstruct WorkflowExecutionFailedEventAttributes {\n  10: optional string reason\n  20: optional binary details\n  30: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n}\n\nstruct WorkflowExecutionTimedOutEventAttributes {\n  10: optional TimeoutType timeoutType\n}\n\nstruct WorkflowExecutionContinuedAsNewEventAttributes {\n  10: optional string newExecutionRunId\n  20: optional WorkflowType workflowType\n  30: optional TaskList taskList\n  40: optional binary input\n  50: optional i32 executionStartToCloseTimeoutSeconds\n  60: optional i32 taskStartToCloseTimeoutSeconds\n  70: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n}\n\nstruct DecisionTaskScheduledEventAttributes {\n  10: optional TaskList taskList\n  20: optional i32 startToCloseTimeoutSeconds\n}\n\nstruct DecisionTaskStartedEventAttributes {\n  10: optional i64 (js.type = \"Long\") scheduledEventId\n  20: optional string identity\n  30: optional string requestId\n}\n\nstruct DecisionTaskCompletedEventAttributes {\n  10: optional binary executionContext\n  20: optional i64 (js.type = \"Long\") scheduledEventId\n  30: optional i64 (js.type = \"Long\") startedEventId\n  40: optional string identity\n}\n\nstruct DecisionTaskTimedOutEventAttributes {\n  10: optional i64 (js.type = \"Long\") scheduledEventId\n  20: optional i64 (js.type = \"Long\") startedEventId\n  30: optional TimeoutType timeoutType\n}\n\nstruct DecisionTaskFailedEventAttributes {\n  10: optional i64 (js.type = \"Long\") scheduledEventId\n  20: optional i64 (js.type = \"Long\") startedEventId\n  30: optional DecisionTaskFailedCause cause\n  40: optional string identity\n}\n\nstruct ActivityTaskScheduledEventAttributes {\n  10: optional string activityId\n  20: optional ActivityType activityType\n  25: optional string domain\n  30: optional TaskList taskList\n  40: optional binary input\n  45: optional i32 scheduleToCloseTimeoutSeconds\n  50: optional i32 scheduleToStartTimeoutSeconds\n  55: optional i32 startToCloseTimeoutSeconds\n  60: optional i32 heartbeatTimeoutSeconds\n  90: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n}\n\nstruct ActivityTaskStartedEventAttributes {\n  10: optional i64 (js.type = \"Long\") scheduledEventId\n  20: optional string identity\n  30: optional string requestId\n}\n\nstruct ActivityTaskCompletedEventAttributes {\n  10: optional binary result\n  20: optional i64 (js.type = \"Long\") scheduledEventId\n  30: optional i64 (js.type = \"Long\") startedEventId\n  40: optional string identity\n}\n\nstruct ActivityTaskFailedEventAttributes {\n  10: optional string reason\n  20: optional binary details\n  30: optional i64 (js.type = \"Long\") scheduledEventId\n  40: optional i64 (js.type = \"Long\") startedEventId\n  50: optional string identity\n}\n\nstruct ActivityTaskTimedOutEventAttributes {\n  05: optional binary details\n  10: optional i64 (js.type = \"Long\") scheduledEventId\n  20: optional i64 (js.type = \"Long\") startedEventId\n  30: optional TimeoutType timeoutType\n}\n\nstruct ActivityTaskCancelRequestedEventAttributes {\n  10: optional string activityId\n  20: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n}\n\nstruct RequestCancelActivityTaskFailedEventAttributes{\n  10: optional string activityId\n  20: optional string cause\n  30: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n}\n\nstruct ActivityTaskCanceledEventAttributes {\n  10: optional binary details\n  20: optional i64 (js.type = \"Long\") latestCancelRequestedEventId\n  30: optional i64 (js.type = \"Long\") scheduledEventId\n  40: optional i64 (js.type = \"Long\") startedEventId\n  50: optional string identity\n}\n\nstruct TimerStartedEventAttributes {\n  10: optional string timerId\n  20: optional i64 (js.type = \"Long\") startToFireTimeoutSeconds\n  30: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n}\n\nstruct TimerFiredEventAttributes {\n  10: optional string timerId\n  20: optional i64 (js.type = \"Long\") startedEventId\n}\n\nstruct TimerCanceledEventAttributes {\n  10: optional string timerId\n  20: optional i64 (js.type = \"Long\") startedEventId\n  30: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n  40: optional string identity\n}\n\nstruct CancelTimerFailedEventAttributes {\n  10: optional string timerId\n  20: optional string cause\n  30: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n  40: optional string identity\n}\n\nstruct WorkflowExecutionCancelRequestedEventAttributes {\n  10: optional string cause\n  20: optional i64 (js.type = \"Long\") externalInitiatedEventId\n  30: optional WorkflowExecution externalWorkflowExecution\n  40: optional string identity\n}\n\nstruct WorkflowExecutionCanceledEventAttributes {\n  10: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n  20: optional binary details\n}\n\nstruct MarkerRecordedEventAttributes {\n  10: optional string markerName\n  20: optional binary details\n  30: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n}\n\nstruct WorkflowExecutionSignaledEventAttributes {\n  10: optional string signalName\n  20: optional binary input\n  30: optional string identity\n}\n\nstruct WorkflowExecutionTerminatedEventAttributes {\n  10: optional string reason\n  20: optional binary details\n  30: optional string identity\n}\n\nstruct RequestCancelExternalWorkflowExecutionInitiatedEventAttributes {\n  10: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n  20: optional string domain\n  30: optional WorkflowExecution workflowExecution\n  40: optional binary control\n}\n\nstruct RequestCancelExternalWorkflowExecutionFailedEventAttributes {\n  10: optional CancelExternalWorkflowExecutionFailedCause cause\n  20: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n  30: optional string domain\n  40: optional WorkflowExecution workflowExecution\n  50: optional i64 (js.type = \"Long\") initiatedEventId\n  60: optional binary control\n}\n\nstruct ExternalWorkflowExecutionCancelRequestedEventAttributes {\n  10: optional i64 (js.type = \"Long\") initiatedEventId\n  20: optional string domain\n  30: optional WorkflowExecution workflowExecution\n}\n\nstruct StartChildWorkflowExecutionInitiatedEventAttributes {\n  10:  optional string domain\n  20:  optional string workflowId\n  30:  optional WorkflowType workflowType\n  40:  optional TaskList taskList\n  50:  optional binary input\n  60:  optional i32 executionStartToCloseTimeoutSeconds\n  70:  optional i32 taskStartToCloseTimeoutSeconds\n  80:  optional ChildPolicy childPolicy\n  90:  optional binary control\n  100: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n}\n\nstruct StartChildWorkflowExecutionFailedEventAttributes {\n  10: optional string domain\n  20: optional string workflowId\n  30: optional WorkflowType workflowType\n  40: optional ChildWorkflowExecutionFailedCause cause\n  50: optional binary control\n  60: optional i64 (js.type = \"Long\") initiatedEventId\n  70: optional i64 (js.type = \"Long\") decisionTaskCompletedEventId\n}\n\nstruct ChildWorkflowExecutionStartedEventAttributes {\n  10: optional string domain\n  20: optional i64 (js.type = \"Long\") initiatedEventId\n  30: optional WorkflowExecution workflowExecution\n  40: optional WorkflowType workflowType\n}\n\nstruct ChildWorkflowExecutionCompletedEventAttributes {\n  10: optional binary result\n  20: optional string domain\n  30: optional WorkflowExecution workflowExecution\n  40: optional WorkflowType workflowType\n  50: optional i64 (js.type = \"Long\") initiatedEventId\n  60: optional i64 (js.type = \"Long\") startedEventId\n}\n\nstruct ChildWorkflowExecutionFailedEventAttributes {\n  10: optional string reason\n  20: optional binary details\n  30: optional string domain\n  40: optional WorkflowExecution workflowExecution\n  50: optional WorkflowType workflowType\n  60: optional i64 (js.type = \"Long\") initiatedEventId\n  70: optional i64 (js.type = \"Long\") startedEventId\n}\n\nstruct ChildWorkflowExecutionCanceledEventAttributes {\n  10: optional binary details\n  20: optional string domain\n  30: optional WorkflowExecution workflowExecution\n  40: optional WorkflowType workflowType\n  50: optional i64 (js.type = \"Long\") initiatedEventId\n  60: optional i64 (js.type = \"Long\") startedEventId\n}\n\nstruct ChildWorkflowExecutionTimedOutEventAttributes {\n  10: optional TimeoutType timeoutType\n  20: optional string domain\n  30: optional WorkflowExecution workflowExecution\n  40: optional WorkflowType workflowType\n  50: optional i64 (js.type = \"Long\") initiatedEventId\n  60: optional i64 (js.type = \"Long\") startedEventId\n}\n\nstruct ChildWorkflowExecutionTerminatedEventAttributes {\n  10: optional string domain\n  20: optional WorkflowExecution workflowExecution\n  30: optional WorkflowType workflowType\n  40: optional i64 (js.type = \"Long\") initiatedEventId\n  50: optional i64 (js.type = \"Long\") startedEventId\n}\n\nstruct HistoryEvent {\n  10:  optional i64 (js.type = \"Long\") eventId\n  20:  optional i64 (js.type = \"Long\") timestamp\n  30:  optional EventType eventType\n  40:  optional WorkflowExecutionStartedEventAttributes workflowExecutionStartedEventAttributes\n  50:  optional WorkflowExecutionCompletedEventAttributes workflowExecutionCompletedEventAttributes\n  60:  optional WorkflowExecutionFailedEventAttributes workflowExecutionFailedEventAttributes\n  70:  optional WorkflowExecutionTimedOutEventAttributes workflowExecutionTimedOutEventAttributes\n  80:  optional DecisionTaskScheduledEventAttributes decisionTaskScheduledEventAttributes\n  90:  optional DecisionTaskStartedEventAttributes decisionTaskStartedEventAttributes\n  100: optional DecisionTaskCompletedEventAttributes decisionTaskCompletedEventAttributes\n  110: optional DecisionTaskTimedOutEventAttributes decisionTaskTimedOutEventAttributes\n  120: optional DecisionTaskFailedEventAttributes decisionTaskFailedEventAttributes\n  130: optional ActivityTaskScheduledEventAttributes activityTaskScheduledEventAttributes\n  140: optional ActivityTaskStartedEventAttributes activityTaskStartedEventAttributes\n  150: optional ActivityTaskCompletedEventAttributes activityTaskCompletedEventAttributes\n  160: optional ActivityTaskFailedEventAttributes activityTaskFailedEventAttributes\n  170: optional ActivityTaskTimedOutEventAttributes activityTaskTimedOutEventAttributes\n  180: optional TimerStartedEventAttributes timerStartedEventAttributes\n  190: optional TimerFiredEventAttributes timerFiredEventAttributes\n  200: optional ActivityTaskCancelRequestedEventAttributes activityTaskCancelRequestedEventAttributes\n  210: optional RequestCancelActivityTaskFailedEventAttributes requestCancelActivityTaskFailedEventAttributes\n  220: optional ActivityTaskCanceledEventAttributes activityTaskCanceledEventAttributes\n  230: optional TimerCanceledEventAttributes timerCanceledEventAttributes\n  240: optional CancelTimerFailedEventAttributes cancelTimerFailedEventAttributes\n  250: optional MarkerRecordedEventAttributes markerRecordedEventAttributes\n  260: optional WorkflowExecutionSignaledEventAttributes workflowExecutionSignaledEventAttributes\n  270: optional WorkflowExecutionTerminatedEventAttributes workflowExecutionTerminatedEventAttributes\n  280: optional WorkflowExecutionCancelRequestedEventAttributes workflowExecutionCancelRequestedEventAttributes\n  290: optional WorkflowExecutionCanceledEventAttributes workflowExecutionCanceledEventAttributes\n  300: optional RequestCancelExternalWorkflowExecutionInitiatedEventAttributes requestCancelExternalWorkflowExecutionInitiatedEventAttributes\n  310: optional RequestCancelExternalWorkflowExecutionFailedEventAttributes requestCancelExternalWorkflowExecutionFailedEventAttributes\n  320: optional ExternalWorkflowExecutionCancelRequestedEventAttributes externalWorkflowExecutionCancelRequestedEventAttributes\n  330: optional WorkflowExecutionContinuedAsNewEventAttributes workflowExecutionContinuedAsNewEventAttributes\n  340: optional StartChildWorkflowExecutionInitiatedEventAttributes startChildWorkflowExecutionInitiatedEventAttributes\n  350: optional StartChildWorkflowExecutionFailedEventAttributes startChildWorkflowExecutionFailedEventAttributes\n  360: optional ChildWorkflowExecutionStartedEventAttributes childWorkflowExecutionStartedEventAttributes\n  370: optional ChildWorkflowExecutionCompletedEventAttributes childWorkflowExecutionCompletedEventAttributes\n  380: optional ChildWorkflowExecutionFailedEventAttributes childWorkflowExecutionFailedEventAttributes\n  390: optional ChildWorkflowExecutionCanceledEventAttributes childWorkflowExecutionCanceledEventAttributes\n  400: optional ChildWorkflowExecutionTimedOutEventAttributes childWorkflowExecutionTimedOutEventAttributes\n  410: optional ChildWorkflowExecutionTerminatedEventAttributes childWorkflowExecutionTerminatedEventAttributes\n}\n\nstruct History {\n  10: optional list<HistoryEvent> events\n}\n\nstruct WorkflowExecutionFilter {\n  10: optional string workflowId\n}\n\nstruct WorkflowTypeFilter {\n  10: optional string name\n}\n\nstruct StartTimeFilter {\n  10: optional i64 (js.type = \"Long\") earliestTime\n  20: optional i64 (js.type = \"Long\") latestTime\n}\n\nstruct DomainInfo {\n  10: optional string name\n  20: optional DomainStatus status\n  30: optional string description\n  40: optional string ownerEmail\n}\n\nstruct DomainConfiguration {\n  10: optional i32 workflowExecutionRetentionPeriodInDays\n  20: optional bool emitMetric\n}\n\nstruct UpdateDomainInfo {\n  10: optional string description\n  20: optional string ownerEmail\n}\n\nstruct RegisterDomainRequest {\n  10: optional string name\n  20: optional string description\n  30: optional string ownerEmail\n  40: optional i32 workflowExecutionRetentionPeriodInDays\n  50: optional bool emitMetric\n}\n\nstruct DescribeDomainRequest {\n 10: optional string name\n}\n\nstruct DescribeDomainResponse {\n  10: optional DomainInfo domainInfo\n  20: optional DomainConfiguration configuration\n}\n\nstruct UpdateDomainRequest {\n 10: optional string name\n 20: optional UpdateDomainInfo updatedInfo\n 30: optional DomainConfiguration configuration\n}\n\nstruct UpdateDomainResponse {\n  10: optional DomainInfo domainInfo\n  20: optional DomainConfiguration configuration\n}\n\nstruct DeprecateDomainRequest {\n 10: optional string name\n}\n\nstruct StartWorkflowExecutionRequest {\n  10: optional string domain\n  20: optional string workflowId\n  30: optional WorkflowType workflowType\n  40: optional TaskList taskList\n  50: optional binary input\n  60: optional i32 executionStartToCloseTimeoutSeconds\n  70: optional i32 taskStartToCloseTimeoutSeconds\n  80: optional string identity\n  90: optional string requestId\n}\n\nstruct StartWorkflowExecutionResponse {\n  10: optional string runId\n}\n\nstruct PollForDecisionTaskRequest {\n  10: optional string domain\n  20: optional TaskList taskList\n  30: optional string identity\n}\n\nstruct PollForDecisionTaskResponse {\n  10: optional binary taskToken\n  20: optional WorkflowExecution workflowExecution\n  30: optional WorkflowType workflowType\n  40: optional i64 (js.type = \"Long\") previousStartedEventId\n  50: optional i64 (js.type = \"Long\") startedEventId\n  60: optional History history\n  70: optional binary nextPageToken\n  80: optional WorkflowQuery query\n}\n\nstruct StickyExecutionAttributes {\n  10: optional string WorkerTaskList\n  20: optional i32 scheduleToStartTimeoutSeconds\n}\n\nstruct RespondDecisionTaskCompletedRequest {\n  10: optional binary taskToken\n  20: optional list<Decision> decisions\n  30: optional binary executionContext\n  40: optional string identity\n  50: optional StickyExecutionAttributes stickyAttributes\n}\n\nstruct PollForActivityTaskRequest {\n  10: optional string domain\n  20: optional TaskList taskList\n  30: optional string identity\n}\n\nstruct PollForActivityTaskResponse {\n  10:  optional binary taskToken\n  20:  optional WorkflowExecution workflowExecution\n  30:  optional string activityId\n  40:  optional ActivityType activityType\n  50:  optional binary input\n  70:  optional i64 (js.type = \"Long\") scheduledTimestamp\n  80:  optional i32 scheduleToCloseTimeoutSeconds\n  90:  optional i64 (js.type = \"Long\") startedTimestamp\n  100: optional i32 startToCloseTimeoutSeconds\n  110: optional i32 heartbeatTimeoutSeconds\n}\n\nstruct RecordActivityTaskHeartbeatRequest {\n  10: optional binary taskToken\n  20: optional binary details\n  30: optional string identity\n}\n\nstruct RecordActivityTaskHeartbeatResponse {\n  10: optional bool cancelRequested\n}\n\nstruct RespondActivityTaskCompletedRequest {\n  10: optional binary taskToken\n  20: optional binary result\n  30: optional string identity\n}\n\nstruct RespondActivityTaskFailedRequest {\n  10: optional binary taskToken\n  20: optional string reason\n  30: optional binary details\n  40: optional string identity\n}\n\nstruct RespondActivityTaskCanceledRequest {\n  10: optional binary taskToken\n  20: optional binary details\n  30: optional string identity\n}\n\nstruct RequestCancelWorkflowExecutionRequest {\n  10: optional string domain\n  20: optional WorkflowExecution workflowExecution\n  30: optional string identity\n  40: optional string requestId\n}\n\nstruct GetWorkflowExecutionHistoryRequest {\n  10: optional string domain\n  20: optional WorkflowExecution execution\n  30: optional i32 maximumPageSize\n  40: optional binary nextPageToken\n}\n\nstruct GetWorkflowExecutionHistoryResponse {\n  10: optional History history\n  20: optional binary nextPageToken\n}\n\nstruct SignalWorkflowExecutionRequest {\n  10: optional string domain\n  20: optional WorkflowExecution workflowExecution\n  30: optional string signalName\n  40: optional binary input\n  50: optional string identity\n}\n\nstruct TerminateWorkflowExecutionRequest {\n  10: optional string domain\n  20: optional WorkflowExecution workflowExecution\n  30: optional string reason\n  40: optional binary details\n  50: optional string identity\n}\n\nstruct ListOpenWorkflowExecutionsRequest {\n  10: optional string domain\n  20: optional i32 maximumPageSize\n  30: optional binary nextPageToken\n  40: optional StartTimeFilter StartTimeFilter\n  50: optional WorkflowExecutionFilter executionFilter\n  60: optional WorkflowTypeFilter typeFilter\n}\n\nstruct ListOpenWorkflowExecutionsResponse {\n  10: optional list<WorkflowExecutionInfo> executions\n  20: optional binary nextPageToken\n}\n\nstruct ListClosedWorkflowExecutionsRequest {\n  10: optional string domain\n  20: optional i32 maximumPageSize\n  30: optional binary nextPageToken\n  40: optional StartTimeFilter StartTimeFilter\n  50: optional WorkflowExecutionFilter executionFilter\n  60: optional WorkflowTypeFilter typeFilter\n  70: optional WorkflowExecutionCloseStatus statusFilter\n}\n\nstruct ListClosedWorkflowExecutionsResponse {\n  10: optional list<WorkflowExecutionInfo> executions\n  20: optional binary nextPageToken\n}\n\nstruct QueryWorkflowRequest {\n  10: optional string domain\n  20: optional WorkflowExecution execution\n  30: optional WorkflowQuery query\n}\n\nstruct QueryWorkflowResponse {\n  10: optional binary queryResult\n}\n\nstruct WorkflowQuery {\n  10: optional string queryType\n  20: optional binary queryArgs\n}\n\nstruct RespondQueryTaskCompletedRequest {\n  10: optional binary taskToken\n  20: optional QueryTaskCompletedType completedType\n  30: optional binary queryResult\n  40: optional string errorMessage\n}\n"
