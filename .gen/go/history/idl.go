// Code generated by thriftrw v1.6.0. DO NOT EDIT.
// @generated

package history

import (
	"github.com/uber/cadence/.gen/go/shared"
	"go.uber.org/thriftrw/thriftreflect"
)

var ThriftModule = &thriftreflect.ThriftModule{Name: "history", Package: "github.com/uber/cadence/.gen/go/history", FilePath: "history.thrift", SHA1: "fdcb3ebb5e91070b4809f22338e2ae3917b80042", Includes: []*thriftreflect.ThriftModule{shared.ThriftModule}, Raw: rawIDL}

const rawIDL = "// Copyright (c) 2017 Uber Technologies, Inc.\n//\n// Permission is hereby granted, free of charge, to any person obtaining a copy\n// of this software and associated documentation files (the \"Software\"), to deal\n// in the Software without restriction, including without limitation the rights\n// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell\n// copies of the Software, and to permit persons to whom the Software is\n// furnished to do so, subject to the following conditions:\n//\n// The above copyright notice and this permission notice shall be included in\n// all copies or substantial portions of the Software.\n//\n// THE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR\n// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,\n// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE\n// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER\n// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,\n// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN\n// THE SOFTWARE.\n\ninclude \"shared.thrift\"\n\nnamespace java com.uber.cadence.history\n\nexception EventAlreadyStartedError {\n  1: required string message\n}\n\nexception ShardOwnershipLostError {\n  10: optional string message\n  20: optional string owner\n}\n\nstruct ParentExecutionInfo {\n  10: optional string domainUUID\n  20: optional shared.WorkflowExecution execution\n  30: optional i64 (js.type = \"Long\") initiatedId\n}\n\nstruct StartWorkflowExecutionRequest {\n  10: optional string domainUUID\n  20: optional shared.StartWorkflowExecutionRequest startRequest\n  30: optional ParentExecutionInfo parentExecutionInfo\n}\n\nstruct GetWorkflowExecutionNextEventIDRequest {\n  10: optional string domainUUID\n  20: optional shared.WorkflowExecution execution\n}\n\nstruct GetWorkflowExecutionNextEventIDResponse {\n  10: optional i64 (js.type = \"Long\") eventId\n  20: optional string runId\n  30: optional shared.TaskList tasklist\n}\n\nstruct RespondDecisionTaskCompletedRequest {\n  10: optional string domainUUID\n  20: optional shared.RespondDecisionTaskCompletedRequest completeRequest\n}\n\nstruct RecordActivityTaskHeartbeatRequest {\n  10: optional string domainUUID\n  20: optional shared.RecordActivityTaskHeartbeatRequest heartbeatRequest\n}\n\nstruct RespondActivityTaskCompletedRequest {\n  10: optional string domainUUID\n  20: optional shared.RespondActivityTaskCompletedRequest completeRequest\n}\n\nstruct RespondActivityTaskFailedRequest {\n  10: optional string domainUUID\n  20: optional shared.RespondActivityTaskFailedRequest failedRequest\n}\n\nstruct RespondActivityTaskCanceledRequest {\n  10: optional string domainUUID\n  20: optional shared.RespondActivityTaskCanceledRequest cancelRequest\n}\n\nstruct RecordActivityTaskStartedRequest {\n  10: optional string domainUUID\n  20: optional shared.WorkflowExecution workflowExecution\n  30: optional i64 (js.type = \"Long\") scheduleId\n  40: optional i64 (js.type = \"Long\") taskId\n  45: optional string requestId // Unique id of each poll request. Used to ensure at most once delivery of tasks.\n  50: optional shared.PollForActivityTaskRequest pollRequest\n}\n\nstruct RecordActivityTaskStartedResponse {\n  10: optional shared.HistoryEvent startedEvent\n  20: optional shared.HistoryEvent scheduledEvent\n}\n\nstruct RecordDecisionTaskStartedRequest {\n  10: optional string domainUUID\n  20: optional shared.WorkflowExecution workflowExecution\n  30: optional i64 (js.type = \"Long\") scheduleId\n  40: optional i64 (js.type = \"Long\") taskId\n  45: optional string requestId // Unique id of each poll request. Used to ensure at most once delivery of tasks.\n  50: optional shared.PollForDecisionTaskRequest pollRequest\n}\n\nstruct RecordDecisionTaskStartedResponse {\n  10: optional shared.WorkflowType workflowType\n  20: optional i64 (js.type = \"Long\") previousStartedEventId\n  30: optional i64 (js.type = \"Long\") startedEventId\n  40: optional bool stickyExecutionEnabled\n}\n\nstruct SignalWorkflowExecutionRequest {\n  10: optional string domainUUID\n  20: optional shared.SignalWorkflowExecutionRequest signalRequest\n}\n\nstruct TerminateWorkflowExecutionRequest {\n  10: optional string domainUUID\n  20: optional shared.TerminateWorkflowExecutionRequest terminateRequest\n}\n\nstruct RequestCancelWorkflowExecutionRequest {\n  10: optional string domainUUID\n  20: optional shared.RequestCancelWorkflowExecutionRequest cancelRequest\n  30: optional i64 (js.type = \"Long\") externalInitiatedEventId\n  40: optional shared.WorkflowExecution externalWorkflowExecution\n}\n\nstruct ScheduleDecisionTaskRequest {\n  10: optional string domainUUID\n  20: optional shared.WorkflowExecution workflowExecution\n}\n\nstruct DescribeWorkflowExecutionRequest {\n  10: optional string domainUUID\n  20: optional shared.DescribeWorkflowExecutionRequest request\n}\n\n/**\n* RecordChildExecutionCompletedRequest is used for reporting the completion of child execution to parent workflow\n* execution which started it.  When a child execution is completed it creates this request and calls the\n* RecordChildExecutionCompleted API with the workflowExecution of parent.  It also sets the completedExecution of the\n* child as it could potentially be different than the ChildExecutionStartedEvent of parent in the situation when\n* child creates multiple runs through ContinueAsNew before finally completing.\n**/\nstruct RecordChildExecutionCompletedRequest {\n  10: optional string domainUUID\n  20: optional shared.WorkflowExecution workflowExecution\n  30: optional i64 (js.type = \"Long\") initiatedId\n  40: optional shared.WorkflowExecution completedExecution\n  50: optional shared.HistoryEvent completionEvent\n}\n\n/**\n* HistoryService provides API to start a new long running workflow instance, as well as query and update the history\n* of workflow instances already created.\n**/\nservice HistoryService {\n  /**\n  * StartWorkflowExecution starts a new long running workflow instance.  It will create the instance with\n  * 'WorkflowExecutionStarted' event in history and also schedule the first DecisionTask for the worker to make the\n  * first decision for this instance.  It will return 'WorkflowExecutionAlreadyStartedError', if an instance already\n  * exists with same workflowId.\n  **/\n  shared.StartWorkflowExecutionResponse StartWorkflowExecution(1: StartWorkflowExecutionRequest startRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.WorkflowExecutionAlreadyStartedError sessionAlreadyExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * Returns the nextEventID of the history of workflow execution. Only events in the history with Ids below the returned Id are\n  * guaranteed to be valid, so the first step of reading an execution's history is to retrieve this event Id.\n  * It fails with 'EntityNotExistError' if specified workflow execution in unknown to the service.\n  **/\n  GetWorkflowExecutionNextEventIDResponse GetWorkflowExecutionNextEventID(1: GetWorkflowExecutionNextEventIDRequest getRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * RecordDecisionTaskStarted is called by the Matchingservice before it hands a decision task to the application worker in response to\n  * a PollForDecisionTask call. It records in the history the event that the decision task has started. It will return 'EventAlreadyStartedError',\n  * if the workflow's execution history already includes a record of the event starting.\n  **/\n  RecordDecisionTaskStartedResponse RecordDecisionTaskStarted(1: RecordDecisionTaskStartedRequest addRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: EventAlreadyStartedError eventAlreadyStartedError,\n      4: shared.EntityNotExistsError entityNotExistError,\n      5: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * RecordActivityTaskStarted is called by the Matchingservice before it hands a decision task to the application worker in response to\n  * a PollForActivityTask call. It records in the history the event that the decision task has started. It will return 'EventAlreadyStartedError',\n  * if the workflow's execution history already includes a record of the event starting.\n  **/\n  RecordActivityTaskStartedResponse RecordActivityTaskStarted(1: RecordActivityTaskStartedRequest addRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: EventAlreadyStartedError eventAlreadyStartedError,\n      4: shared.EntityNotExistsError entityNotExistError,\n      5: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * RespondDecisionTaskCompleted is called by application worker to complete a DecisionTask handed as a result of\n  * 'PollForDecisionTask' API call.  Completing a DecisionTask will result in new events for the workflow execution and\n  * potentially new ActivityTask being created for corresponding decisions.  It will also create a DecisionTaskCompleted\n  * event in the history for that session.  Use the 'taskToken' provided as response of PollForDecisionTask API call\n  * for completing the DecisionTask.\n  **/\n  void RespondDecisionTaskCompleted(1: RespondDecisionTaskCompletedRequest completeRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * RecordActivityTaskHeartbeat is called by application worker while it is processing an ActivityTask.  If worker fails\n  * to heartbeat within 'heartbeatTimeoutSeconds' interval for the ActivityTask, then it will be marked as timedout and\n  * 'ActivityTaskTimedOut' event will be written to the workflow history.  Calling 'RecordActivityTaskHeartbeat' will\n  * fail with 'EntityNotExistsError' in such situations.  Use the 'taskToken' provided as response of\n  * PollForActivityTask API call for heartbeating.\n  **/\n  shared.RecordActivityTaskHeartbeatResponse RecordActivityTaskHeartbeat(1: RecordActivityTaskHeartbeatRequest heartbeatRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * RespondActivityTaskCompleted is called by application worker when it is done processing an ActivityTask.  It will\n  * result in a new 'ActivityTaskCompleted' event being written to the workflow history and a new DecisionTask\n  * created for the workflow so new decisions could be made.  Use the 'taskToken' provided as response of\n  * PollForActivityTask API call for completion. It fails with 'EntityNotExistsError' if the taskToken is not valid\n  * anymore due to activity timeout.\n  **/\n  void  RespondActivityTaskCompleted(1: RespondActivityTaskCompletedRequest completeRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * RespondActivityTaskFailed is called by application worker when it is done processing an ActivityTask.  It will\n  * result in a new 'ActivityTaskFailed' event being written to the workflow history and a new DecisionTask\n  * created for the workflow instance so new decisions could be made.  Use the 'taskToken' provided as response of\n  * PollForActivityTask API call for completion. It fails with 'EntityNotExistsError' if the taskToken is not valid\n  * anymore due to activity timeout.\n  **/\n  void RespondActivityTaskFailed(1: RespondActivityTaskFailedRequest failRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * RespondActivityTaskCanceled is called by application worker when it is successfully canceled an ActivityTask.  It will\n  * result in a new 'ActivityTaskCanceled' event being written to the workflow history and a new DecisionTask\n  * created for the workflow instance so new decisions could be made.  Use the 'taskToken' provided as response of\n  * PollForActivityTask API call for completion. It fails with 'EntityNotExistsError' if the taskToken is not valid\n  * anymore due to activity timeout.\n  **/\n  void RespondActivityTaskCanceled(1: RespondActivityTaskCanceledRequest canceledRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * SignalWorkflowExecution is used to send a signal event to running workflow execution.  This results in\n  * WorkflowExecutionSignaled event recorded in the history and a decision task being created for the execution.\n  **/\n  void SignalWorkflowExecution(1: SignalWorkflowExecutionRequest signalRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * TerminateWorkflowExecution terminates an existing workflow execution by recording WorkflowExecutionTerminated event\n  * in the history and immediately terminating the execution instance.\n  **/\n  void TerminateWorkflowExecution(1: TerminateWorkflowExecutionRequest terminateRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * RequestCancelWorkflowExecution is called by application worker when it wants to request cancellation of a workflow instance.\n  * It will result in a new 'WorkflowExecutionCancelRequested' event being written to the workflow history and a new DecisionTask\n  * created for the workflow instance so new decisions could be made. It fails with 'EntityNotExistsError' if the workflow is not valid\n  * anymore due to completion or doesn't exist.\n  **/\n  void RequestCancelWorkflowExecution(1: RequestCancelWorkflowExecutionRequest cancelRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n      5: shared.CancellationAlreadyRequestedError cancellationAlreadyRequestedError,\n    )\n\n  /**\n  * ScheduleDecisionTask is used for creating a decision task for already started workflow execution.  This is mainly\n  * used by transfer queue processor during the processing of StartChildWorkflowExecution task, where it first starts\n  * child execution without creating the decision task and then calls this API after updating the mutable state of\n  * parent execution.\n  **/\n  void ScheduleDecisionTask(1: ScheduleDecisionTaskRequest scheduleRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * RecordChildExecutionCompleted is used for reporting the completion of child workflow execution to parent.\n  * This is mainly called by transfer queue processor during the processing of DeleteExecution task.\n  **/\n  void RecordChildExecutionCompleted(1: RecordChildExecutionCompletedRequest completionRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n  /**\n  * DescribeWorkflowExecution returns information about the specified workflow execution.\n  **/\n  shared.DescribeWorkflowExecutionResponse DescribeWorkflowExecution(1: DescribeWorkflowExecutionRequest describeRequest)\n    throws (\n      1: shared.BadRequestError badRequestError,\n      2: shared.InternalServiceError internalServiceError,\n      3: shared.EntityNotExistsError entityNotExistError,\n      4: ShardOwnershipLostError shardOwnershipLostError,\n    )\n\n}\n"