// Copyright (c) 2017-2020 Uber Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// AccessDeniedError is an internal type (TBD...)
type AccessDeniedError struct {
	Message string `json:"message,required"`
}

// ActivityLocalDispatchInfo is an internal type (TBD...)
type ActivityLocalDispatchInfo struct {
	ActivityID                      string `json:"activityId,omitempty"`
	ScheduledTimestamp              *int64 `json:"scheduledTimestamp,omitempty"`
	StartedTimestamp                *int64 `json:"startedTimestamp,omitempty"`
	ScheduledTimestampOfThisAttempt *int64 `json:"scheduledTimestampOfThisAttempt,omitempty"`
	TaskToken                       []byte `json:"taskToken,omitempty"`
}

// GetScheduledTimestamp is an internal getter (TBD...)
func (v *ActivityLocalDispatchInfo) GetScheduledTimestamp() (o int64) {
	if v != nil && v.ScheduledTimestamp != nil {
		return *v.ScheduledTimestamp
	}
	return
}

// ActivityTaskCancelRequestedEventAttributes is an internal type (TBD...)
type ActivityTaskCancelRequestedEventAttributes struct {
	ActivityID                   string `json:"activityId,omitempty"`
	DecisionTaskCompletedEventID int64  `json:"decisionTaskCompletedEventId,omitempty"`
}

// GetActivityID is an internal getter (TBD...)
func (v *ActivityTaskCancelRequestedEventAttributes) GetActivityID() (o string) {
	if v != nil {
		return v.ActivityID
	}
	return
}

// ActivityTaskCanceledEventAttributes is an internal type (TBD...)
type ActivityTaskCanceledEventAttributes struct {
	Details                      []byte `json:"details,omitempty"`
	LatestCancelRequestedEventID int64  `json:"latestCancelRequestedEventId,omitempty"`
	ScheduledEventID             int64  `json:"scheduledEventId,omitempty"`
	StartedEventID               int64  `json:"startedEventId,omitempty"`
	Identity                     string `json:"identity,omitempty"`
}

// GetScheduledEventID is an internal getter (TBD...)
func (v *ActivityTaskCanceledEventAttributes) GetScheduledEventID() (o int64) {
	if v != nil {
		return v.ScheduledEventID
	}
	return
}

// ActivityTaskCompletedEventAttributes is an internal type (TBD...)
type ActivityTaskCompletedEventAttributes struct {
	Result           []byte `json:"result,omitempty"`
	ScheduledEventID int64  `json:"scheduledEventId,omitempty"`
	StartedEventID   int64  `json:"startedEventId,omitempty"`
	Identity         string `json:"identity,omitempty"`
}

// GetScheduledEventID is an internal getter (TBD...)
func (v *ActivityTaskCompletedEventAttributes) GetScheduledEventID() (o int64) {
	if v != nil {
		return v.ScheduledEventID
	}
	return
}

// GetStartedEventID is an internal getter (TBD...)
func (v *ActivityTaskCompletedEventAttributes) GetStartedEventID() (o int64) {
	if v != nil {
		return v.StartedEventID
	}
	return
}

// ActivityTaskFailedEventAttributes is an internal type (TBD...)
type ActivityTaskFailedEventAttributes struct {
	Reason           *string `json:"reason,omitempty"`
	Details          []byte  `json:"details,omitempty"`
	ScheduledEventID int64   `json:"scheduledEventId,omitempty"`
	StartedEventID   int64   `json:"startedEventId,omitempty"`
	Identity         string  `json:"identity,omitempty"`
}

// GetScheduledEventID is an internal getter (TBD...)
func (v *ActivityTaskFailedEventAttributes) GetScheduledEventID() (o int64) {
	if v != nil {
		return v.ScheduledEventID
	}
	return
}

// GetStartedEventID is an internal getter (TBD...)
func (v *ActivityTaskFailedEventAttributes) GetStartedEventID() (o int64) {
	if v != nil {
		return v.StartedEventID
	}
	return
}

// ActivityTaskScheduledEventAttributes is an internal type (TBD...)
type ActivityTaskScheduledEventAttributes struct {
	ActivityID                    string        `json:"activityId,omitempty"`
	ActivityType                  *ActivityType `json:"activityType,omitempty"`
	Domain                        *string       `json:"domain,omitempty"`
	TaskList                      *TaskList     `json:"taskList,omitempty"`
	Input                         []byte        `json:"input,omitempty"`
	ScheduleToCloseTimeoutSeconds *int32        `json:"scheduleToCloseTimeoutSeconds,omitempty"`
	ScheduleToStartTimeoutSeconds *int32        `json:"scheduleToStartTimeoutSeconds,omitempty"`
	StartToCloseTimeoutSeconds    *int32        `json:"startToCloseTimeoutSeconds,omitempty"`
	HeartbeatTimeoutSeconds       *int32        `json:"heartbeatTimeoutSeconds,omitempty"`
	DecisionTaskCompletedEventID  int64         `json:"decisionTaskCompletedEventId,omitempty"`
	RetryPolicy                   *RetryPolicy  `json:"retryPolicy,omitempty"`
	Header                        *Header       `json:"header,omitempty"`
}

// GetActivityID is an internal getter (TBD...)
func (v *ActivityTaskScheduledEventAttributes) GetActivityID() (o string) {
	if v != nil {
		return v.ActivityID
	}
	return
}

// GetActivityType is an internal getter (TBD...)
func (v *ActivityTaskScheduledEventAttributes) GetActivityType() (o *ActivityType) {
	if v != nil && v.ActivityType != nil {
		return v.ActivityType
	}
	return
}

// GetDomain is an internal getter (TBD...)
func (v *ActivityTaskScheduledEventAttributes) GetDomain() (o string) {
	if v != nil && v.Domain != nil {
		return *v.Domain
	}
	return
}

// GetTaskList is an internal getter (TBD...)
func (v *ActivityTaskScheduledEventAttributes) GetTaskList() (o *TaskList) {
	if v != nil && v.TaskList != nil {
		return v.TaskList
	}
	return
}

// GetScheduleToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *ActivityTaskScheduledEventAttributes) GetScheduleToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.ScheduleToCloseTimeoutSeconds != nil {
		return *v.ScheduleToCloseTimeoutSeconds
	}
	return
}

// GetScheduleToStartTimeoutSeconds is an internal getter (TBD...)
func (v *ActivityTaskScheduledEventAttributes) GetScheduleToStartTimeoutSeconds() (o int32) {
	if v != nil && v.ScheduleToStartTimeoutSeconds != nil {
		return *v.ScheduleToStartTimeoutSeconds
	}
	return
}

// GetStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *ActivityTaskScheduledEventAttributes) GetStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.StartToCloseTimeoutSeconds != nil {
		return *v.StartToCloseTimeoutSeconds
	}
	return
}

// GetHeartbeatTimeoutSeconds is an internal getter (TBD...)
func (v *ActivityTaskScheduledEventAttributes) GetHeartbeatTimeoutSeconds() (o int32) {
	if v != nil && v.HeartbeatTimeoutSeconds != nil {
		return *v.HeartbeatTimeoutSeconds
	}
	return
}

// ActivityTaskStartedEventAttributes is an internal type (TBD...)
type ActivityTaskStartedEventAttributes struct {
	ScheduledEventID   int64   `json:"scheduledEventId,omitempty"`
	Identity           string  `json:"identity,omitempty"`
	RequestID          string  `json:"requestId,omitempty"`
	Attempt            int32   `json:"attempt,omitempty"`
	LastFailureReason  *string `json:"lastFailureReason,omitempty"`
	LastFailureDetails []byte  `json:"lastFailureDetails,omitempty"`
}

// GetScheduledEventID is an internal getter (TBD...)
func (v *ActivityTaskStartedEventAttributes) GetScheduledEventID() (o int64) {
	if v != nil {
		return v.ScheduledEventID
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *ActivityTaskStartedEventAttributes) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// ActivityTaskTimedOutEventAttributes is an internal type (TBD...)
type ActivityTaskTimedOutEventAttributes struct {
	Details            []byte       `json:"details,omitempty"`
	ScheduledEventID   int64        `json:"scheduledEventId,omitempty"`
	StartedEventID     int64        `json:"startedEventId,omitempty"`
	TimeoutType        *TimeoutType `json:"timeoutType,omitempty"`
	LastFailureReason  *string      `json:"lastFailureReason,omitempty"`
	LastFailureDetails []byte       `json:"lastFailureDetails,omitempty"`
}

// GetScheduledEventID is an internal getter (TBD...)
func (v *ActivityTaskTimedOutEventAttributes) GetScheduledEventID() (o int64) {
	if v != nil {
		return v.ScheduledEventID
	}
	return
}

// GetTimeoutType is an internal getter (TBD...)
func (v *ActivityTaskTimedOutEventAttributes) GetTimeoutType() (o TimeoutType) {
	if v != nil && v.TimeoutType != nil {
		return *v.TimeoutType
	}
	return
}

// ActivityType is an internal type (TBD...)
type ActivityType struct {
	Name string `json:"name,omitempty"`
}

// GetName is an internal getter (TBD...)
func (v *ActivityType) GetName() (o string) {
	if v != nil {
		return v.Name
	}
	return
}

// ArchivalStatus is an internal type (TBD...)
type ArchivalStatus int32

// Ptr is a helper function for getting pointer value
func (e ArchivalStatus) Ptr() *ArchivalStatus {
	return &e
}

// String returns a readable string representation of ArchivalStatus.
func (e ArchivalStatus) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "DISABLED"
	case 1:
		return "ENABLED"
	}
	return fmt.Sprintf("ArchivalStatus(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *ArchivalStatus) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "DISABLED":
		*e = ArchivalStatusDisabled
		return nil
	case "ENABLED":
		*e = ArchivalStatusEnabled
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "ArchivalStatus", err)
		}
		*e = ArchivalStatus(val)
		return nil
	}
}

// MarshalText encodes ArchivalStatus to text.
func (e ArchivalStatus) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// ArchivalStatusDisabled is an option for ArchivalStatus
	ArchivalStatusDisabled ArchivalStatus = iota
	// ArchivalStatusEnabled is an option for ArchivalStatus
	ArchivalStatusEnabled
)

// BadBinaries is an internal type (TBD...)
type BadBinaries struct {
	Binaries map[string]*BadBinaryInfo `json:"binaries,omitempty"`
}

// BadBinaryInfo is an internal type (TBD...)
type BadBinaryInfo struct {
	Reason          string `json:"reason,omitempty"`
	Operator        string `json:"operator,omitempty"`
	CreatedTimeNano *int64 `json:"createdTimeNano,omitempty"`
}

// GetReason is an internal getter (TBD...)
func (v *BadBinaryInfo) GetReason() (o string) {
	if v != nil {
		return v.Reason
	}
	return
}

// GetOperator is an internal getter (TBD...)
func (v *BadBinaryInfo) GetOperator() (o string) {
	if v != nil {
		return v.Operator
	}
	return
}

// GetCreatedTimeNano is an internal getter (TBD...)
func (v *BadBinaryInfo) GetCreatedTimeNano() (o int64) {
	if v != nil && v.CreatedTimeNano != nil {
		return *v.CreatedTimeNano
	}
	return
}

// BadRequestError is an internal type (TBD...)
type BadRequestError struct {
	Message string `json:"message,required"`
}

// CancelExternalWorkflowExecutionFailedCause is an internal type (TBD...)
type CancelExternalWorkflowExecutionFailedCause int32

// Ptr is a helper function for getting pointer value
func (e CancelExternalWorkflowExecutionFailedCause) Ptr() *CancelExternalWorkflowExecutionFailedCause {
	return &e
}

// String returns a readable string representation of CancelExternalWorkflowExecutionFailedCause.
func (e CancelExternalWorkflowExecutionFailedCause) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION"
	case 1:
		return "WORKFLOW_ALREADY_COMPLETED"
	}
	return fmt.Sprintf("CancelExternalWorkflowExecutionFailedCause(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *CancelExternalWorkflowExecutionFailedCause) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION":
		*e = CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution
		return nil
	case "WORKFLOW_ALREADY_COMPLETED":
		*e = CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "CancelExternalWorkflowExecutionFailedCause", err)
		}
		*e = CancelExternalWorkflowExecutionFailedCause(val)
		return nil
	}
}

// MarshalText encodes CancelExternalWorkflowExecutionFailedCause to text.
func (e CancelExternalWorkflowExecutionFailedCause) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution is an option for CancelExternalWorkflowExecutionFailedCause
	CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution CancelExternalWorkflowExecutionFailedCause = iota
	CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted
)

// CancelTimerDecisionAttributes is an internal type (TBD...)
type CancelTimerDecisionAttributes struct {
	TimerID string `json:"timerId,omitempty"`
}

// GetTimerID is an internal getter (TBD...)
func (v *CancelTimerDecisionAttributes) GetTimerID() (o string) {
	if v != nil {
		return v.TimerID
	}
	return
}

// CancelTimerFailedEventAttributes is an internal type (TBD...)
type CancelTimerFailedEventAttributes struct {
	TimerID                      string `json:"timerId,omitempty"`
	Cause                        string `json:"cause,omitempty"`
	DecisionTaskCompletedEventID int64  `json:"decisionTaskCompletedEventId,omitempty"`
	Identity                     string `json:"identity,omitempty"`
}

// CancelWorkflowExecutionDecisionAttributes is an internal type (TBD...)
type CancelWorkflowExecutionDecisionAttributes struct {
	Details []byte `json:"details,omitempty"`
}

// CancellationAlreadyRequestedError is an internal type (TBD...)
type CancellationAlreadyRequestedError struct {
	Message string `json:"message,required"`
}

// ChildWorkflowExecutionCanceledEventAttributes is an internal type (TBD...)
type ChildWorkflowExecutionCanceledEventAttributes struct {
	Details           []byte             `json:"details,omitempty"`
	Domain            string             `json:"domain,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	WorkflowType      *WorkflowType      `json:"workflowType,omitempty"`
	InitiatedEventID  int64              `json:"initiatedEventId,omitempty"`
	StartedEventID    int64              `json:"startedEventId,omitempty"`
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *ChildWorkflowExecutionCanceledEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// ChildWorkflowExecutionCompletedEventAttributes is an internal type (TBD...)
type ChildWorkflowExecutionCompletedEventAttributes struct {
	Result            []byte             `json:"result,omitempty"`
	Domain            string             `json:"domain,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	WorkflowType      *WorkflowType      `json:"workflowType,omitempty"`
	InitiatedEventID  int64              `json:"initiatedEventId,omitempty"`
	StartedEventID    int64              `json:"startedEventId,omitempty"`
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *ChildWorkflowExecutionCompletedEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// ChildWorkflowExecutionFailedCause is an internal type (TBD...)
type ChildWorkflowExecutionFailedCause int32

// Ptr is a helper function for getting pointer value
func (e ChildWorkflowExecutionFailedCause) Ptr() *ChildWorkflowExecutionFailedCause {
	return &e
}

// String returns a readable string representation of ChildWorkflowExecutionFailedCause.
func (e ChildWorkflowExecutionFailedCause) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "WORKFLOW_ALREADY_RUNNING"
	}
	return fmt.Sprintf("ChildWorkflowExecutionFailedCause(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *ChildWorkflowExecutionFailedCause) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "WORKFLOW_ALREADY_RUNNING":
		*e = ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "ChildWorkflowExecutionFailedCause", err)
		}
		*e = ChildWorkflowExecutionFailedCause(val)
		return nil
	}
}

// MarshalText encodes ChildWorkflowExecutionFailedCause to text.
func (e ChildWorkflowExecutionFailedCause) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning is an option for ChildWorkflowExecutionFailedCause
	ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning ChildWorkflowExecutionFailedCause = iota
)

// ChildWorkflowExecutionFailedEventAttributes is an internal type (TBD...)
type ChildWorkflowExecutionFailedEventAttributes struct {
	Reason            *string            `json:"reason,omitempty"`
	Details           []byte             `json:"details,omitempty"`
	Domain            string             `json:"domain,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	WorkflowType      *WorkflowType      `json:"workflowType,omitempty"`
	InitiatedEventID  int64              `json:"initiatedEventId,omitempty"`
	StartedEventID    int64              `json:"startedEventId,omitempty"`
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *ChildWorkflowExecutionFailedEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// ChildWorkflowExecutionStartedEventAttributes is an internal type (TBD...)
type ChildWorkflowExecutionStartedEventAttributes struct {
	Domain            string             `json:"domain,omitempty"`
	InitiatedEventID  int64              `json:"initiatedEventId,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	WorkflowType      *WorkflowType      `json:"workflowType,omitempty"`
	Header            *Header            `json:"header,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *ChildWorkflowExecutionStartedEventAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *ChildWorkflowExecutionStartedEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// GetWorkflowExecution is an internal getter (TBD...)
func (v *ChildWorkflowExecutionStartedEventAttributes) GetWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.WorkflowExecution != nil {
		return v.WorkflowExecution
	}
	return
}

// ChildWorkflowExecutionTerminatedEventAttributes is an internal type (TBD...)
type ChildWorkflowExecutionTerminatedEventAttributes struct {
	Domain            string             `json:"domain,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	WorkflowType      *WorkflowType      `json:"workflowType,omitempty"`
	InitiatedEventID  int64              `json:"initiatedEventId,omitempty"`
	StartedEventID    int64              `json:"startedEventId,omitempty"`
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *ChildWorkflowExecutionTerminatedEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// ChildWorkflowExecutionTimedOutEventAttributes is an internal type (TBD...)
type ChildWorkflowExecutionTimedOutEventAttributes struct {
	TimeoutType       *TimeoutType       `json:"timeoutType,omitempty"`
	Domain            string             `json:"domain,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	WorkflowType      *WorkflowType      `json:"workflowType,omitempty"`
	InitiatedEventID  int64              `json:"initiatedEventId,omitempty"`
	StartedEventID    int64              `json:"startedEventId,omitempty"`
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *ChildWorkflowExecutionTimedOutEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// ClientVersionNotSupportedError is an internal type (TBD...)
type ClientVersionNotSupportedError struct {
	FeatureVersion    string `json:"featureVersion,required"`
	ClientImpl        string `json:"clientImpl,required"`
	SupportedVersions string `json:"supportedVersions,required"`
}

// FeatureNotEnabledError is an internal type (TBD...)
type FeatureNotEnabledError struct {
	FeatureFlag string `json:"featureFlag,required"`
}

// CloseShardRequest is an internal type (TBD...)
type CloseShardRequest struct {
	ShardID int32 `json:"shardID,omitempty"`
}

// GetShardID is an internal getter (TBD...)
func (v *CloseShardRequest) GetShardID() (o int32) {
	if v != nil {
		return v.ShardID
	}
	return
}

// ClusterInfo is an internal type (TBD...)
type ClusterInfo struct {
	SupportedClientVersions *SupportedClientVersions `json:"supportedClientVersions,omitempty"`
}

// ClusterReplicationConfiguration is an internal type (TBD...)
type ClusterReplicationConfiguration struct {
	ClusterName string `json:"clusterName,omitempty"`
}

// GetClusterName is an internal getter (TBD...)
func (v *ClusterReplicationConfiguration) GetClusterName() (o string) {
	if v != nil {
		return v.ClusterName
	}
	return
}

// CompleteWorkflowExecutionDecisionAttributes is an internal type (TBD...)
type CompleteWorkflowExecutionDecisionAttributes struct {
	Result []byte `json:"result,omitempty"`
}

// ContinueAsNewInitiator is an internal type (TBD...)
type ContinueAsNewInitiator int32

// Ptr is a helper function for getting pointer value
func (e ContinueAsNewInitiator) Ptr() *ContinueAsNewInitiator {
	return &e
}

// String returns a readable string representation of ContinueAsNewInitiator.
func (e ContinueAsNewInitiator) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "Decider"
	case 1:
		return "RetryPolicy"
	case 2:
		return "CronSchedule"
	}
	return fmt.Sprintf("ContinueAsNewInitiator(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *ContinueAsNewInitiator) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "DECIDER":
		*e = ContinueAsNewInitiatorDecider
		return nil
	case "RETRYPOLICY":
		*e = ContinueAsNewInitiatorRetryPolicy
		return nil
	case "CRONSCHEDULE":
		*e = ContinueAsNewInitiatorCronSchedule
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "ContinueAsNewInitiator", err)
		}
		*e = ContinueAsNewInitiator(val)
		return nil
	}
}

// MarshalText encodes ContinueAsNewInitiator to text.
func (e ContinueAsNewInitiator) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// ContinueAsNewInitiatorDecider is an option for ContinueAsNewInitiator
	ContinueAsNewInitiatorDecider ContinueAsNewInitiator = iota
	// ContinueAsNewInitiatorRetryPolicy is an option for ContinueAsNewInitiator
	ContinueAsNewInitiatorRetryPolicy
	// ContinueAsNewInitiatorCronSchedule is an option for ContinueAsNewInitiator
	ContinueAsNewInitiatorCronSchedule
)

// ContinueAsNewWorkflowExecutionDecisionAttributes is an internal type (TBD...)
type ContinueAsNewWorkflowExecutionDecisionAttributes struct {
	WorkflowType                        *WorkflowType           `json:"workflowType,omitempty"`
	TaskList                            *TaskList               `json:"taskList,omitempty"`
	Input                               []byte                  `json:"input,omitempty"`
	ExecutionStartToCloseTimeoutSeconds *int32                  `json:"executionStartToCloseTimeoutSeconds,omitempty"`
	TaskStartToCloseTimeoutSeconds      *int32                  `json:"taskStartToCloseTimeoutSeconds,omitempty"`
	BackoffStartIntervalInSeconds       *int32                  `json:"backoffStartIntervalInSeconds,omitempty"`
	RetryPolicy                         *RetryPolicy            `json:"retryPolicy,omitempty"`
	Initiator                           *ContinueAsNewInitiator `json:"initiator,omitempty"`
	FailureReason                       *string                 `json:"failureReason,omitempty"`
	FailureDetails                      []byte                  `json:"failureDetails,omitempty"`
	LastCompletionResult                []byte                  `json:"lastCompletionResult,omitempty"`
	CronSchedule                        string                  `json:"cronSchedule,omitempty"`
	Header                              *Header                 `json:"header,omitempty"`
	Memo                                *Memo                   `json:"memo,omitempty"`
	SearchAttributes                    *SearchAttributes       `json:"searchAttributes,omitempty"`
	JitterStartSeconds                  *int32                  `json:"jitterStartSeconds,omitempty"`
}

// GetExecutionStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *ContinueAsNewWorkflowExecutionDecisionAttributes) GetExecutionStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.ExecutionStartToCloseTimeoutSeconds != nil {
		return *v.ExecutionStartToCloseTimeoutSeconds
	}
	return
}

// GetTaskStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *ContinueAsNewWorkflowExecutionDecisionAttributes) GetTaskStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.TaskStartToCloseTimeoutSeconds != nil {
		return *v.TaskStartToCloseTimeoutSeconds
	}
	return
}

// GetBackoffStartIntervalInSeconds is an internal getter (TBD...)
func (v *ContinueAsNewWorkflowExecutionDecisionAttributes) GetBackoffStartIntervalInSeconds() (o int32) {
	if v != nil && v.BackoffStartIntervalInSeconds != nil {
		return *v.BackoffStartIntervalInSeconds
	}
	return
}

// GetJitterStartSeconds is an internal getter (TBD...)
func (v *ContinueAsNewWorkflowExecutionDecisionAttributes) GetJitterStartSeconds() (o int32) {
	if v != nil && v.JitterStartSeconds != nil {
		return *v.JitterStartSeconds
	}
	return
}

// GetInitiator is an internal getter (TBD...)
func (v *ContinueAsNewWorkflowExecutionDecisionAttributes) GetInitiator() (o ContinueAsNewInitiator) {
	if v != nil && v.Initiator != nil {
		return *v.Initiator
	}
	return
}

// GetSearchAttributes is an internal getter (TBD...)
func (v *ContinueAsNewWorkflowExecutionDecisionAttributes) GetSearchAttributes() (o *SearchAttributes) {
	if v != nil && v.SearchAttributes != nil {
		return v.SearchAttributes
	}
	return
}

// CountWorkflowExecutionsRequest is an internal type (TBD...)
type CountWorkflowExecutionsRequest struct {
	Domain string `json:"domain,omitempty"`
	Query  string `json:"query,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *CountWorkflowExecutionsRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetQuery is an internal getter (TBD...)
func (v *CountWorkflowExecutionsRequest) GetQuery() (o string) {
	if v != nil {
		return v.Query
	}
	return
}

// CountWorkflowExecutionsResponse is an internal type (TBD...)
type CountWorkflowExecutionsResponse struct {
	Count int64 `json:"count,omitempty"`
}

// GetCount is an internal getter (TBD...)
func (v *CountWorkflowExecutionsResponse) GetCount() (o int64) {
	if v != nil {
		return v.Count
	}
	return
}

// CurrentBranchChangedError is an internal type (TBD...)
type CurrentBranchChangedError struct {
	Message            string `json:"message,required"`
	CurrentBranchToken []byte `json:"currentBranchToken,required"`
}

// GetCurrentBranchToken is an internal getter (TBD...)
func (v *CurrentBranchChangedError) GetCurrentBranchToken() (o []byte) {
	if v != nil && v.CurrentBranchToken != nil {
		return v.CurrentBranchToken
	}
	return
}

// DataBlob is an internal type (TBD...)
type DataBlob struct {
	EncodingType *EncodingType `json:"EncodingType,omitempty"`
	Data         []byte        `json:"Data,omitempty"`
}

// GetEncodingType is an internal getter (TBD...)
func (v *DataBlob) GetEncodingType() (o EncodingType) {
	if v != nil && v.EncodingType != nil {
		return *v.EncodingType
	}
	return
}

// GetData is an internal getter (TBD...)
func (v *DataBlob) GetData() (o []byte) {
	if v != nil && v.Data != nil {
		return v.Data
	}
	return
}

func (v *DataBlob) DeepCopy() *DataBlob {
	if v == nil {
		return nil
	}

	res := &DataBlob{
		EncodingType: v.EncodingType,
	}

	if v.Data != nil {
		res.Data = make([]byte, len(v.Data))
		copy(res.Data, v.Data)
	}

	return res
}

// Decision is an internal type (TBD...)
type Decision struct {
	DecisionType                                             *DecisionType                                             `json:"decisionType,omitempty"`
	ScheduleActivityTaskDecisionAttributes                   *ScheduleActivityTaskDecisionAttributes                   `json:"scheduleActivityTaskDecisionAttributes,omitempty"`
	StartTimerDecisionAttributes                             *StartTimerDecisionAttributes                             `json:"startTimerDecisionAttributes,omitempty"`
	CompleteWorkflowExecutionDecisionAttributes              *CompleteWorkflowExecutionDecisionAttributes              `json:"completeWorkflowExecutionDecisionAttributes,omitempty"`
	FailWorkflowExecutionDecisionAttributes                  *FailWorkflowExecutionDecisionAttributes                  `json:"failWorkflowExecutionDecisionAttributes,omitempty"`
	RequestCancelActivityTaskDecisionAttributes              *RequestCancelActivityTaskDecisionAttributes              `json:"requestCancelActivityTaskDecisionAttributes,omitempty"`
	CancelTimerDecisionAttributes                            *CancelTimerDecisionAttributes                            `json:"cancelTimerDecisionAttributes,omitempty"`
	CancelWorkflowExecutionDecisionAttributes                *CancelWorkflowExecutionDecisionAttributes                `json:"cancelWorkflowExecutionDecisionAttributes,omitempty"`
	RequestCancelExternalWorkflowExecutionDecisionAttributes *RequestCancelExternalWorkflowExecutionDecisionAttributes `json:"requestCancelExternalWorkflowExecutionDecisionAttributes,omitempty"`
	RecordMarkerDecisionAttributes                           *RecordMarkerDecisionAttributes                           `json:"recordMarkerDecisionAttributes,omitempty"`
	ContinueAsNewWorkflowExecutionDecisionAttributes         *ContinueAsNewWorkflowExecutionDecisionAttributes         `json:"continueAsNewWorkflowExecutionDecisionAttributes,omitempty"`
	StartChildWorkflowExecutionDecisionAttributes            *StartChildWorkflowExecutionDecisionAttributes            `json:"startChildWorkflowExecutionDecisionAttributes,omitempty"`
	SignalExternalWorkflowExecutionDecisionAttributes        *SignalExternalWorkflowExecutionDecisionAttributes        `json:"signalExternalWorkflowExecutionDecisionAttributes,omitempty"`
	UpsertWorkflowSearchAttributesDecisionAttributes         *UpsertWorkflowSearchAttributesDecisionAttributes         `json:"upsertWorkflowSearchAttributesDecisionAttributes,omitempty"`
}

// GetDecisionType is an internal getter (TBD...)
func (v *Decision) GetDecisionType() (o DecisionType) {
	if v != nil && v.DecisionType != nil {
		return *v.DecisionType
	}
	return
}

// DecisionTaskCompletedEventAttributes is an internal type (TBD...)
type DecisionTaskCompletedEventAttributes struct {
	ExecutionContext []byte `json:"executionContext,omitempty"`
	ScheduledEventID int64  `json:"scheduledEventId,omitempty"`
	StartedEventID   int64  `json:"startedEventId,omitempty"`
	Identity         string `json:"identity,omitempty"`
	BinaryChecksum   string `json:"binaryChecksum,omitempty"`
}

// GetStartedEventID is an internal getter (TBD...)
func (v *DecisionTaskCompletedEventAttributes) GetStartedEventID() (o int64) {
	if v != nil {
		return v.StartedEventID
	}
	return
}

// GetBinaryChecksum is an internal getter (TBD...)
func (v *DecisionTaskCompletedEventAttributes) GetBinaryChecksum() (o string) {
	if v != nil {
		return v.BinaryChecksum
	}
	return
}

// DecisionTaskFailedCause is an internal type (TBD...)
type DecisionTaskFailedCause int32

// Ptr is a helper function for getting pointer value
func (e DecisionTaskFailedCause) Ptr() *DecisionTaskFailedCause {
	return &e
}

// String returns a readable string representation of DecisionTaskFailedCause.
func (e DecisionTaskFailedCause) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "UNHANDLED_DECISION"
	case 1:
		return "BAD_SCHEDULE_ACTIVITY_ATTRIBUTES"
	case 2:
		return "BAD_REQUEST_CANCEL_ACTIVITY_ATTRIBUTES"
	case 3:
		return "BAD_START_TIMER_ATTRIBUTES"
	case 4:
		return "BAD_CANCEL_TIMER_ATTRIBUTES"
	case 5:
		return "BAD_RECORD_MARKER_ATTRIBUTES"
	case 6:
		return "BAD_COMPLETE_WORKFLOW_EXECUTION_ATTRIBUTES"
	case 7:
		return "BAD_FAIL_WORKFLOW_EXECUTION_ATTRIBUTES"
	case 8:
		return "BAD_CANCEL_WORKFLOW_EXECUTION_ATTRIBUTES"
	case 9:
		return "BAD_REQUEST_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_ATTRIBUTES"
	case 10:
		return "BAD_CONTINUE_AS_NEW_ATTRIBUTES"
	case 11:
		return "START_TIMER_DUPLICATE_I_D"
	case 12:
		return "RESET_STICKY_TASKLIST"
	case 13:
		return "WORKFLOW_WORKER_UNHANDLED_FAILURE"
	case 14:
		return "BAD_SIGNAL_WORKFLOW_EXECUTION_ATTRIBUTES"
	case 15:
		return "BAD_START_CHILD_EXECUTION_ATTRIBUTES"
	case 16:
		return "FORCE_CLOSE_DECISION"
	case 17:
		return "FAILOVER_CLOSE_DECISION"
	case 18:
		return "BAD_SIGNAL_INPUT_SIZE"
	case 19:
		return "RESET_WORKFLOW"
	case 20:
		return "BAD_BINARY"
	case 21:
		return "SCHEDULE_ACTIVITY_DUPLICATE_I_D"
	case 22:
		return "BAD_SEARCH_ATTRIBUTES"
	}
	return fmt.Sprintf("DecisionTaskFailedCause(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *DecisionTaskFailedCause) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "UNHANDLED_DECISION":
		*e = DecisionTaskFailedCauseUnhandledDecision
		return nil
	case "BAD_SCHEDULE_ACTIVITY_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadScheduleActivityAttributes
		return nil
	case "BAD_REQUEST_CANCEL_ACTIVITY_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadRequestCancelActivityAttributes
		return nil
	case "BAD_START_TIMER_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadStartTimerAttributes
		return nil
	case "BAD_CANCEL_TIMER_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadCancelTimerAttributes
		return nil
	case "BAD_RECORD_MARKER_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadRecordMarkerAttributes
		return nil
	case "BAD_COMPLETE_WORKFLOW_EXECUTION_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes
		return nil
	case "BAD_FAIL_WORKFLOW_EXECUTION_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes
		return nil
	case "BAD_CANCEL_WORKFLOW_EXECUTION_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes
		return nil
	case "BAD_REQUEST_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes
		return nil
	case "BAD_CONTINUE_AS_NEW_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadContinueAsNewAttributes
		return nil
	case "START_TIMER_DUPLICATE_I_D":
		*e = DecisionTaskFailedCauseStartTimerDuplicateID
		return nil
	case "RESET_STICKY_TASKLIST":
		*e = DecisionTaskFailedCauseResetStickyTasklist
		return nil
	case "WORKFLOW_WORKER_UNHANDLED_FAILURE":
		*e = DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure
		return nil
	case "BAD_SIGNAL_WORKFLOW_EXECUTION_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes
		return nil
	case "BAD_START_CHILD_EXECUTION_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadStartChildExecutionAttributes
		return nil
	case "FORCE_CLOSE_DECISION":
		*e = DecisionTaskFailedCauseForceCloseDecision
		return nil
	case "FAILOVER_CLOSE_DECISION":
		*e = DecisionTaskFailedCauseFailoverCloseDecision
		return nil
	case "BAD_SIGNAL_INPUT_SIZE":
		*e = DecisionTaskFailedCauseBadSignalInputSize
		return nil
	case "RESET_WORKFLOW":
		*e = DecisionTaskFailedCauseResetWorkflow
		return nil
	case "BAD_BINARY":
		*e = DecisionTaskFailedCauseBadBinary
		return nil
	case "SCHEDULE_ACTIVITY_DUPLICATE_I_D":
		*e = DecisionTaskFailedCauseScheduleActivityDuplicateID
		return nil
	case "BAD_SEARCH_ATTRIBUTES":
		*e = DecisionTaskFailedCauseBadSearchAttributes
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "DecisionTaskFailedCause", err)
		}
		*e = DecisionTaskFailedCause(val)
		return nil
	}
}

// MarshalText encodes DecisionTaskFailedCause to text.
func (e DecisionTaskFailedCause) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// DecisionTaskFailedCauseUnhandledDecision is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseUnhandledDecision DecisionTaskFailedCause = iota
	// DecisionTaskFailedCauseBadScheduleActivityAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadScheduleActivityAttributes
	// DecisionTaskFailedCauseBadRequestCancelActivityAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadRequestCancelActivityAttributes
	// DecisionTaskFailedCauseBadStartTimerAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadStartTimerAttributes
	// DecisionTaskFailedCauseBadCancelTimerAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadCancelTimerAttributes
	// DecisionTaskFailedCauseBadRecordMarkerAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadRecordMarkerAttributes
	// DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes
	// DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes
	// DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes
	// DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes
	// DecisionTaskFailedCauseBadContinueAsNewAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadContinueAsNewAttributes
	// DecisionTaskFailedCauseStartTimerDuplicateID is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseStartTimerDuplicateID
	// DecisionTaskFailedCauseResetStickyTasklist is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseResetStickyTasklist
	// DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure
	// DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes
	// DecisionTaskFailedCauseBadStartChildExecutionAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadStartChildExecutionAttributes
	// DecisionTaskFailedCauseForceCloseDecision is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseForceCloseDecision
	// DecisionTaskFailedCauseFailoverCloseDecision is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseFailoverCloseDecision
	// DecisionTaskFailedCauseBadSignalInputSize is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadSignalInputSize
	// DecisionTaskFailedCauseResetWorkflow is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseResetWorkflow
	// DecisionTaskFailedCauseBadBinary is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadBinary
	// DecisionTaskFailedCauseScheduleActivityDuplicateID is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseScheduleActivityDuplicateID
	// DecisionTaskFailedCauseBadSearchAttributes is an option for DecisionTaskFailedCause
	DecisionTaskFailedCauseBadSearchAttributes
)

// DecisionTaskFailedEventAttributes is an internal type (TBD...)
type DecisionTaskFailedEventAttributes struct {
	ScheduledEventID int64                    `json:"scheduledEventId,omitempty"`
	StartedEventID   int64                    `json:"startedEventId,omitempty"`
	Cause            *DecisionTaskFailedCause `json:"cause,omitempty"`
	Details          []byte                   `json:"details,omitempty"`
	Identity         string                   `json:"identity,omitempty"`
	Reason           *string                  `json:"reason,omitempty"`
	BaseRunID        string                   `json:"baseRunId,omitempty"`
	NewRunID         string                   `json:"newRunId,omitempty"`
	ForkEventVersion int64                    `json:"forkEventVersion,omitempty"`
	BinaryChecksum   string                   `json:"binaryChecksum,omitempty"`
	RequestID        string                   `json:"requestId,omitempty"`
}

// GetCause is an internal getter (TBD...)
func (v *DecisionTaskFailedEventAttributes) GetCause() (o DecisionTaskFailedCause) {
	if v != nil && v.Cause != nil {
		return *v.Cause
	}
	return
}

// GetDetails is an internal getter (TBD...)
func (v *DecisionTaskFailedEventAttributes) GetDetails() (o []byte) {
	if v != nil && v.Details != nil {
		return v.Details
	}
	return
}

// GetBaseRunID is an internal getter (TBD...)
func (v *DecisionTaskFailedEventAttributes) GetBaseRunID() (o string) {
	if v != nil {
		return v.BaseRunID
	}
	return
}

// GetNewRunID is an internal getter (TBD...)
func (v *DecisionTaskFailedEventAttributes) GetNewRunID() (o string) {
	if v != nil {
		return v.NewRunID
	}
	return
}

// GetForkEventVersion is an internal getter (TBD...)
func (v *DecisionTaskFailedEventAttributes) GetForkEventVersion() (o int64) {
	if v != nil {
		return v.ForkEventVersion
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *DecisionTaskFailedEventAttributes) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// DecisionTaskScheduledEventAttributes is an internal type (TBD...)
type DecisionTaskScheduledEventAttributes struct {
	TaskList                   *TaskList `json:"taskList,omitempty"`
	StartToCloseTimeoutSeconds *int32    `json:"startToCloseTimeoutSeconds,omitempty"`
	Attempt                    int64     `json:"attempt,omitempty"`
}

// GetTaskList is an internal getter (TBD...)
func (v *DecisionTaskScheduledEventAttributes) GetTaskList() (o *TaskList) {
	if v != nil && v.TaskList != nil {
		return v.TaskList
	}
	return
}

// GetStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *DecisionTaskScheduledEventAttributes) GetStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.StartToCloseTimeoutSeconds != nil {
		return *v.StartToCloseTimeoutSeconds
	}
	return
}

// GetAttempt is an internal getter (TBD...)
func (v *DecisionTaskScheduledEventAttributes) GetAttempt() (o int64) {
	if v != nil {
		return v.Attempt
	}
	return
}

// DecisionTaskStartedEventAttributes is an internal type (TBD...)
type DecisionTaskStartedEventAttributes struct {
	ScheduledEventID int64  `json:"scheduledEventId,omitempty"`
	Identity         string `json:"identity,omitempty"`
	RequestID        string `json:"requestId,omitempty"`
}

// GetScheduledEventID is an internal getter (TBD...)
func (v *DecisionTaskStartedEventAttributes) GetScheduledEventID() (o int64) {
	if v != nil {
		return v.ScheduledEventID
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *DecisionTaskStartedEventAttributes) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// DecisionTaskTimedOutCause is an internal type (TBD...)
type DecisionTaskTimedOutCause int32

// Ptr is a helper function for getting pointer value
func (e DecisionTaskTimedOutCause) Ptr() *DecisionTaskTimedOutCause {
	return &e
}

// String returns a readable string representation of DecisionTaskTimedOutCause.
func (e DecisionTaskTimedOutCause) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "Timeout"
	case 1:
		return "Reset"
	}
	return fmt.Sprintf("DecisionTaskTimedOutCause(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *DecisionTaskTimedOutCause) UnmarshalText(value []byte) error {
	switch s := strings.ToLower(string(value)); s {
	case "timeout":
		*e = DecisionTaskTimedOutCauseTimeout
		return nil
	case "reset":
		*e = DecisionTaskTimedOutCauseReset
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "DecisionTaskTimedOutCause", err)
		}
		*e = DecisionTaskTimedOutCause(val)
		return nil
	}
}

// MarshalText encodes DecisionTaskFailedCause to text.
func (e DecisionTaskTimedOutCause) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// DecisionTaskTimedOutCauseTimeout is an option for DecisionTaskTimedOutCause
	DecisionTaskTimedOutCauseTimeout DecisionTaskTimedOutCause = iota
	// DecisionTaskTimedOutCauseReset is an option for DecisionTaskTimedOutCause
	DecisionTaskTimedOutCauseReset
)

// DecisionTaskTimedOutEventAttributes is an internal type (TBD...)
type DecisionTaskTimedOutEventAttributes struct {
	ScheduledEventID int64                      `json:"scheduledEventId,omitempty"`
	StartedEventID   int64                      `json:"startedEventId,omitempty"`
	TimeoutType      *TimeoutType               `json:"timeoutType,omitempty"`
	BaseRunID        string                     `json:"baseRunId,omitempty"`
	NewRunID         string                     `json:"newRunId,omitempty"`
	ForkEventVersion int64                      `json:"forkEventVersion,omitempty"`
	Reason           string                     `json:"reason,omitempty"`
	Cause            *DecisionTaskTimedOutCause `json:"cause,omitempty"`
	RequestID        string                     `json:"requestId,omitempty"`
}

// GetScheduledEventID is an internal getter (TBD...)
func (v *DecisionTaskTimedOutEventAttributes) GetScheduledEventID() (o int64) {
	if v != nil {
		return v.ScheduledEventID
	}
	return
}

// GetTimeoutType is an internal getter (TBD...)
func (v *DecisionTaskTimedOutEventAttributes) GetTimeoutType() (o TimeoutType) {
	if v != nil && v.TimeoutType != nil {
		return *v.TimeoutType
	}
	return
}

// GetBaseRunID is an internal getter (TBD...)
func (v *DecisionTaskTimedOutEventAttributes) GetBaseRunID() (o string) {
	if v != nil {
		return v.BaseRunID
	}
	return
}

// GetNewRunID is an internal getter (TBD...)
func (v *DecisionTaskTimedOutEventAttributes) GetNewRunID() (o string) {
	if v != nil {
		return v.NewRunID
	}
	return
}

// GetForkEventVersion is an internal getter (TBD...)
func (v *DecisionTaskTimedOutEventAttributes) GetForkEventVersion() (o int64) {
	if v != nil {
		return v.ForkEventVersion
	}
	return
}

// GetCause is an internal getter (TBD...)
func (v *DecisionTaskTimedOutEventAttributes) GetCause() (o DecisionTaskTimedOutCause) {
	if v != nil && v.Cause != nil {
		return *v.Cause
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *DecisionTaskTimedOutEventAttributes) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// DecisionType is an internal type (TBD...)
type DecisionType int32

// Ptr is a helper function for getting pointer value
func (e DecisionType) Ptr() *DecisionType {
	return &e
}

// String returns a readable string representation of DecisionType.
func (e DecisionType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "ScheduleActivityTask"
	case 1:
		return "RequestCancelActivityTask"
	case 2:
		return "StartTimer"
	case 3:
		return "CompleteWorkflowExecution"
	case 4:
		return "FailWorkflowExecution"
	case 5:
		return "CancelTimer"
	case 6:
		return "CancelWorkflowExecution"
	case 7:
		return "RequestCancelExternalWorkflowExecution"
	case 8:
		return "RecordMarker"
	case 9:
		return "ContinueAsNewWorkflowExecution"
	case 10:
		return "StartChildWorkflowExecution"
	case 11:
		return "SignalExternalWorkflowExecution"
	case 12:
		return "UpsertWorkflowSearchAttributes"
	}
	return fmt.Sprintf("DecisionType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *DecisionType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "SCHEDULEACTIVITYTASK":
		*e = DecisionTypeScheduleActivityTask
		return nil
	case "REQUESTCANCELACTIVITYTASK":
		*e = DecisionTypeRequestCancelActivityTask
		return nil
	case "STARTTIMER":
		*e = DecisionTypeStartTimer
		return nil
	case "COMPLETEWORKFLOWEXECUTION":
		*e = DecisionTypeCompleteWorkflowExecution
		return nil
	case "FAILWORKFLOWEXECUTION":
		*e = DecisionTypeFailWorkflowExecution
		return nil
	case "CANCELTIMER":
		*e = DecisionTypeCancelTimer
		return nil
	case "CANCELWORKFLOWEXECUTION":
		*e = DecisionTypeCancelWorkflowExecution
		return nil
	case "REQUESTCANCELEXTERNALWORKFLOWEXECUTION":
		*e = DecisionTypeRequestCancelExternalWorkflowExecution
		return nil
	case "RECORDMARKER":
		*e = DecisionTypeRecordMarker
		return nil
	case "CONTINUEASNEWWORKFLOWEXECUTION":
		*e = DecisionTypeContinueAsNewWorkflowExecution
		return nil
	case "STARTCHILDWORKFLOWEXECUTION":
		*e = DecisionTypeStartChildWorkflowExecution
		return nil
	case "SIGNALEXTERNALWORKFLOWEXECUTION":
		*e = DecisionTypeSignalExternalWorkflowExecution
		return nil
	case "UPSERTWORKFLOWSEARCHATTRIBUTES":
		*e = DecisionTypeUpsertWorkflowSearchAttributes
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "DecisionType", err)
		}
		*e = DecisionType(val)
		return nil
	}
}

// MarshalText encodes DecisionType to text.
func (e DecisionType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// DecisionTypeScheduleActivityTask is an option for DecisionType
	DecisionTypeScheduleActivityTask DecisionType = iota
	// DecisionTypeRequestCancelActivityTask is an option for DecisionType
	DecisionTypeRequestCancelActivityTask
	// DecisionTypeStartTimer is an option for DecisionType
	DecisionTypeStartTimer
	// DecisionTypeCompleteWorkflowExecution is an option for DecisionType
	DecisionTypeCompleteWorkflowExecution
	// DecisionTypeFailWorkflowExecution is an option for DecisionType
	DecisionTypeFailWorkflowExecution
	// DecisionTypeCancelTimer is an option for DecisionType
	DecisionTypeCancelTimer
	// DecisionTypeCancelWorkflowExecution is an option for DecisionType
	DecisionTypeCancelWorkflowExecution
	// DecisionTypeRequestCancelExternalWorkflowExecution is an option for DecisionType
	DecisionTypeRequestCancelExternalWorkflowExecution
	// DecisionTypeRecordMarker is an option for DecisionType
	DecisionTypeRecordMarker
	// DecisionTypeContinueAsNewWorkflowExecution is an option for DecisionType
	DecisionTypeContinueAsNewWorkflowExecution
	// DecisionTypeStartChildWorkflowExecution is an option for DecisionType
	DecisionTypeStartChildWorkflowExecution
	// DecisionTypeSignalExternalWorkflowExecution is an option for DecisionType
	DecisionTypeSignalExternalWorkflowExecution
	// DecisionTypeUpsertWorkflowSearchAttributes is an option for DecisionType
	DecisionTypeUpsertWorkflowSearchAttributes
)

// DeprecateDomainRequest is an internal type (TBD...)
type DeprecateDomainRequest struct {
	Name          string `json:"name,omitempty"`
	SecurityToken string `json:"securityToken,omitempty"`
}

// GetName is an internal getter (TBD...)
func (v *DeprecateDomainRequest) GetName() (o string) {
	if v != nil {
		return v.Name
	}
	return
}

// DescribeDomainRequest is an internal type (TBD...)
type DescribeDomainRequest struct {
	Name *string `json:"name,omitempty"`
	UUID *string `json:"uuid,omitempty"`
}

// GetName is an internal getter (TBD...)
func (v *DescribeDomainRequest) GetName() (o string) {
	if v != nil && v.Name != nil {
		return *v.Name
	}
	return
}

// GetUUID is an internal getter (TBD...)
func (v *DescribeDomainRequest) GetUUID() (o string) {
	if v != nil && v.UUID != nil {
		return *v.UUID
	}
	return
}

// DescribeDomainResponse is an internal type (TBD...)
type DescribeDomainResponse struct {
	DomainInfo               *DomainInfo                     `json:"domainInfo,omitempty"`
	Configuration            *DomainConfiguration            `json:"configuration,omitempty"`
	ReplicationConfiguration *DomainReplicationConfiguration `json:"replicationConfiguration,omitempty"`
	FailoverVersion          int64                           `json:"failoverVersion,omitempty"`
	IsGlobalDomain           bool                            `json:"isGlobalDomain,omitempty"`
	FailoverInfo             *FailoverInfo                   `json:"failoverInfo,omitempty"`
}

// GetDomainInfo is an internal getter (TBD...)
func (v *DescribeDomainResponse) GetDomainInfo() (o *DomainInfo) {
	if v != nil && v.DomainInfo != nil {
		return v.DomainInfo
	}
	return
}

// GetFailoverVersion is an internal getter (TBD...)
func (v *DescribeDomainResponse) GetFailoverVersion() (o int64) {
	if v != nil {
		return v.FailoverVersion
	}
	return
}

// GetIsGlobalDomain is an internal getter (TBD...)
func (v *DescribeDomainResponse) GetIsGlobalDomain() (o bool) {
	if v != nil {
		return v.IsGlobalDomain
	}
	return
}

// GetFailoverInfo is an internal getter (TBD...)
func (v *DescribeDomainResponse) GetFailoverInfo() (o *FailoverInfo) {
	if v != nil {
		return v.FailoverInfo
	}
	return
}

// DescribeHistoryHostRequest is an internal type (TBD...)
type DescribeHistoryHostRequest struct {
	HostAddress      *string            `json:"hostAddress,omitempty"`
	ShardIDForHost   *int32             `json:"shardIdForHost,omitempty"`
	ExecutionForHost *WorkflowExecution `json:"executionForHost,omitempty"`
}

// DescribeShardDistributionRequest is an internal type (TBD...)
type DescribeShardDistributionRequest struct {
	PageSize int32 `json:"pageSize,omitempty"`
	PageID   int32 `json:"pageID,omitempty"`
}

// GetHostAddress is an internal getter (TBD...)
func (v *DescribeHistoryHostRequest) GetHostAddress() (o string) {
	if v != nil && v.HostAddress != nil {
		return *v.HostAddress
	}
	return
}

// GetShardIDForHost is an internal getter (TBD...)
func (v *DescribeHistoryHostRequest) GetShardIDForHost() (o int32) {
	if v != nil && v.ShardIDForHost != nil {
		return *v.ShardIDForHost
	}
	return
}

// DescribeShardDistributionResponse is an internal type (TBD...)
type DescribeShardDistributionResponse struct {
	NumberOfShards int32            `json:"numberOfShards,omitempty"`
	Shards         map[int32]string `json:"shardIDs,omitempty"`
}

// DescribeHistoryHostResponse is an internal type (TBD...)
type DescribeHistoryHostResponse struct {
	NumberOfShards        int32            `json:"numberOfShards,omitempty"`
	ShardIDs              []int32          `json:"shardIDs,omitempty"`
	DomainCache           *DomainCacheInfo `json:"domainCache,omitempty"`
	ShardControllerStatus string           `json:"shardControllerStatus,omitempty"`
	Address               string           `json:"address,omitempty"`
}

// DescribeQueueRequest is an internal type (TBD...)
type DescribeQueueRequest struct {
	ShardID     int32  `json:"shardID,omitempty"`
	ClusterName string `json:"clusterName,omitempty"`
	Type        *int32 `json:"type,omitempty"`
}

// GetShardID is an internal getter (TBD...)
func (v *DescribeQueueRequest) GetShardID() (o int32) {
	if v != nil {
		return v.ShardID
	}
	return
}

// GetClusterName is an internal getter (TBD...)
func (v *DescribeQueueRequest) GetClusterName() (o string) {
	if v != nil {
		return v.ClusterName
	}
	return
}

// GetType is an internal getter (TBD...)
func (v *DescribeQueueRequest) GetType() (o int32) {
	if v != nil && v.Type != nil {
		return *v.Type
	}
	return
}

// DescribeQueueResponse is an internal type (TBD...)
type DescribeQueueResponse struct {
	ProcessingQueueStates []string `json:"processingQueueStates,omitempty"`
}

// DescribeTaskListRequest is an internal type (TBD...)
type DescribeTaskListRequest struct {
	Domain                string        `json:"domain,omitempty"`
	TaskList              *TaskList     `json:"taskList,omitempty"`
	TaskListType          *TaskListType `json:"taskListType,omitempty"`
	IncludeTaskListStatus bool          `json:"includeTaskListStatus,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *DescribeTaskListRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetTaskList is an internal getter (TBD...)
func (v *DescribeTaskListRequest) GetTaskList() (o *TaskList) {
	if v != nil && v.TaskList != nil {
		return v.TaskList
	}
	return
}

// GetTaskListType is an internal getter (TBD...)
func (v *DescribeTaskListRequest) GetTaskListType() (o TaskListType) {
	if v != nil && v.TaskListType != nil {
		return *v.TaskListType
	}
	return
}

// GetIncludeTaskListStatus is an internal getter (TBD...)
func (v *DescribeTaskListRequest) GetIncludeTaskListStatus() (o bool) {
	if v != nil {
		return v.IncludeTaskListStatus
	}
	return
}

// DescribeTaskListResponse is an internal type (TBD...)
type DescribeTaskListResponse struct {
	Pollers         []*PollerInfo            `json:"pollers,omitempty"`
	TaskListStatus  *TaskListStatus          `json:"taskListStatus,omitempty"`
	PartitionConfig *TaskListPartitionConfig `json:"partitionConfig,omitempty"`
}

// GetPollers is an internal getter (TBD...)
func (v *DescribeTaskListResponse) GetPollers() (o []*PollerInfo) {
	if v != nil && v.Pollers != nil {
		return v.Pollers
	}
	return
}

// GetTaskListStatus is an internal getter (TBD...)
func (v *DescribeTaskListResponse) GetTaskListStatus() (o *TaskListStatus) {
	if v != nil && v.TaskListStatus != nil {
		return v.TaskListStatus
	}
	return
}

// DescribeWorkflowExecutionRequest is an internal type (TBD...)
type DescribeWorkflowExecutionRequest struct {
	Domain    string             `json:"domain,omitempty"`
	Execution *WorkflowExecution `json:"execution,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *DescribeWorkflowExecutionRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetExecution is an internal getter (TBD...)
func (v *DescribeWorkflowExecutionRequest) GetExecution() (o *WorkflowExecution) {
	if v != nil && v.Execution != nil {
		return v.Execution
	}
	return
}

// DescribeWorkflowExecutionResponse is an internal type (TBD...)
type DescribeWorkflowExecutionResponse struct {
	ExecutionConfiguration *WorkflowExecutionConfiguration `json:"executionConfiguration,omitempty"`
	WorkflowExecutionInfo  *WorkflowExecutionInfo          `json:"workflowExecutionInfo,omitempty"`
	PendingActivities      []*PendingActivityInfo          `json:"pendingActivities,omitempty"`
	PendingChildren        []*PendingChildExecutionInfo    `json:"pendingChildren,omitempty"`
	PendingDecision        *PendingDecisionInfo            `json:"pendingDecision,omitempty"`
}

// GetWorkflowExecutionInfo is an internal getter (TBD...)
func (v *DescribeWorkflowExecutionResponse) GetWorkflowExecutionInfo() (o *WorkflowExecutionInfo) {
	if v != nil && v.WorkflowExecutionInfo != nil {
		return v.WorkflowExecutionInfo
	}
	return
}

// GetPendingActivities is an internal getter (TBD...)
func (v *DescribeWorkflowExecutionResponse) GetPendingActivities() (o []*PendingActivityInfo) {
	if v != nil && v.PendingActivities != nil {
		return v.PendingActivities
	}
	return
}

// DomainAlreadyExistsError is an internal type (TBD...)
type DomainAlreadyExistsError struct {
	Message string `json:"message,required"`
}

// DomainCacheInfo is an internal type (TBD...)
type DomainCacheInfo struct {
	NumOfItemsInCacheByID   int64 `json:"numOfItemsInCacheByID,omitempty"`
	NumOfItemsInCacheByName int64 `json:"numOfItemsInCacheByName,omitempty"`
}

// DomainConfiguration is an internal type (TBD...)
type DomainConfiguration struct {
	WorkflowExecutionRetentionPeriodInDays int32                        `json:"workflowExecutionRetentionPeriodInDays,omitempty"`
	EmitMetric                             bool                         `json:"emitMetric,omitempty"`
	BadBinaries                            *BadBinaries                 `json:"badBinaries,omitempty"`
	HistoryArchivalStatus                  *ArchivalStatus              `json:"historyArchivalStatus,omitempty"`
	HistoryArchivalURI                     string                       `json:"historyArchivalURI,omitempty"`
	VisibilityArchivalStatus               *ArchivalStatus              `json:"visibilityArchivalStatus,omitempty"`
	VisibilityArchivalURI                  string                       `json:"visibilityArchivalURI,omitempty"`
	IsolationGroups                        *IsolationGroupConfiguration `json:"isolationGroupConfiguration,omitempty"`
	AsyncWorkflowConfig                    *AsyncWorkflowConfiguration  `json:"asyncWorkflowConfiguration,omitempty"`
}

// GetWorkflowExecutionRetentionPeriodInDays is an internal getter (TBD...)
func (v *DomainConfiguration) GetWorkflowExecutionRetentionPeriodInDays() (o int32) {
	if v != nil {
		return v.WorkflowExecutionRetentionPeriodInDays
	}
	return
}

// GetEmitMetric is an internal getter (TBD...)
func (v *DomainConfiguration) GetEmitMetric() (o bool) {
	if v != nil {
		return v.EmitMetric
	}
	return
}

// GetBadBinaries is an internal getter (TBD...)
func (v *DomainConfiguration) GetBadBinaries() (o *BadBinaries) {
	if v != nil && v.BadBinaries != nil {
		return v.BadBinaries
	}
	return
}

// GetHistoryArchivalStatus is an internal getter (TBD...)
func (v *DomainConfiguration) GetHistoryArchivalStatus() (o ArchivalStatus) {
	if v != nil && v.HistoryArchivalStatus != nil {
		return *v.HistoryArchivalStatus
	}
	return
}

// GetHistoryArchivalURI is an internal getter (TBD...)
func (v *DomainConfiguration) GetHistoryArchivalURI() (o string) {
	if v != nil {
		return v.HistoryArchivalURI
	}
	return
}

// GetVisibilityArchivalStatus is an internal getter (TBD...)
func (v *DomainConfiguration) GetVisibilityArchivalStatus() (o ArchivalStatus) {
	if v != nil && v.VisibilityArchivalStatus != nil {
		return *v.VisibilityArchivalStatus
	}
	return
}

// GetVisibilityArchivalURI is an internal getter (TBD...)
func (v *DomainConfiguration) GetVisibilityArchivalURI() (o string) {
	if v != nil {
		return v.VisibilityArchivalURI
	}
	return
}

// GetIsolationGroupsConfiguration is an internal getter (TBD...)
func (v *DomainConfiguration) GetIsolationGroupsConfiguration() IsolationGroupConfiguration {
	if v.IsolationGroups != nil {
		return *v.IsolationGroups
	}
	return nil
}

func (v *DomainConfiguration) GetAsyncWorkflowConfiguration() AsyncWorkflowConfiguration {
	if v.AsyncWorkflowConfig != nil {
		return *v.AsyncWorkflowConfig
	}
	return AsyncWorkflowConfiguration{}
}

// DomainInfo is an internal type (TBD...)
type DomainInfo struct {
	Name        string            `json:"name,omitempty"`
	Status      *DomainStatus     `json:"status,omitempty"`
	Description string            `json:"description,omitempty"`
	OwnerEmail  string            `json:"ownerEmail,omitempty"`
	Data        map[string]string `json:"data,omitempty"`
	UUID        string            `json:"uuid,omitempty"`
}

// GetName is an internal getter (TBD...)
func (v *DomainInfo) GetName() (o string) {
	if v != nil {
		return v.Name
	}
	return
}

// GetStatus is an internal getter (TBD...)
func (v *DomainInfo) GetStatus() (o DomainStatus) {
	if v != nil && v.Status != nil {
		return *v.Status
	}
	return
}

// GetDescription is an internal getter (TBD...)
func (v *DomainInfo) GetDescription() (o string) {
	if v != nil {
		return v.Description
	}
	return
}

// GetOwnerEmail is an internal getter (TBD...)
func (v *DomainInfo) GetOwnerEmail() (o string) {
	if v != nil {
		return v.OwnerEmail
	}
	return
}

// GetData is an internal getter (TBD...)
func (v *DomainInfo) GetData() (o map[string]string) {
	if v != nil && v.Data != nil {
		return v.Data
	}
	return
}

// GetUUID is an internal getter (TBD...)
func (v *DomainInfo) GetUUID() (o string) {
	if v != nil {
		return v.UUID
	}
	return
}

// DomainNotActiveError is an internal type.
// this is a retriable error and *must* be retried under at least
// some circumstances due to domain failover races.
type DomainNotActiveError struct {
	Message        string `json:"message,required"`
	DomainName     string `json:"domainName,required"`
	CurrentCluster string `json:"currentCluster,required"`
	ActiveCluster  string `json:"activeCluster,required"`
}

// GetCurrentCluster is an internal getter (TBD...)
func (v *DomainNotActiveError) GetCurrentCluster() (o string) {
	if v != nil {
		return v.CurrentCluster
	}
	return
}

// GetActiveCluster is an internal getter (TBD...)
func (v *DomainNotActiveError) GetActiveCluster() (o string) {
	if v != nil {
		return v.ActiveCluster
	}
	return
}

// DomainReplicationConfiguration is an internal type (TBD...)
type DomainReplicationConfiguration struct {
	ActiveClusterName string                             `json:"activeClusterName,omitempty"`
	Clusters          []*ClusterReplicationConfiguration `json:"clusters,omitempty"`
}

// GetActiveClusterName is an internal getter (TBD...)
func (v *DomainReplicationConfiguration) GetActiveClusterName() (o string) {
	if v != nil {
		return v.ActiveClusterName
	}
	return
}

// GetClusters is an internal getter (TBD...)
func (v *DomainReplicationConfiguration) GetClusters() (o []*ClusterReplicationConfiguration) {
	if v != nil && v.Clusters != nil {
		return v.Clusters
	}
	return
}

// DomainStatus is an internal type (TBD...)
type DomainStatus int32

// Ptr is a helper function for getting pointer value
func (e DomainStatus) Ptr() *DomainStatus {
	return &e
}

// String returns a readable string representation of DomainStatus.
func (e DomainStatus) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "REGISTERED"
	case 1:
		return "DEPRECATED"
	case 2:
		return "DELETED"
	}
	return fmt.Sprintf("DomainStatus(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *DomainStatus) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "REGISTERED":
		*e = DomainStatusRegistered
		return nil
	case "DEPRECATED":
		*e = DomainStatusDeprecated
		return nil
	case "DELETED":
		*e = DomainStatusDeleted
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "DomainStatus", err)
		}
		*e = DomainStatus(val)
		return nil
	}
}

// MarshalText encodes DomainStatus to text.
func (e DomainStatus) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// DomainStatusRegistered is an option for DomainStatus
	DomainStatusRegistered DomainStatus = iota
	// DomainStatusDeprecated is an option for DomainStatus
	DomainStatusDeprecated
	// DomainStatusDeleted is an option for DomainStatus
	DomainStatusDeleted
)

// EncodingType is an internal type (TBD...)
type EncodingType int32

// Ptr is a helper function for getting pointer value
func (e EncodingType) Ptr() *EncodingType {
	return &e
}

// String returns a readable string representation of EncodingType.
func (e EncodingType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "ThriftRW"
	case 1:
		return "JSON"
	}
	return fmt.Sprintf("EncodingType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *EncodingType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "THRIFTRW":
		*e = EncodingTypeThriftRW
		return nil
	case "JSON":
		*e = EncodingTypeJSON
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "EncodingType", err)
		}
		*e = EncodingType(val)
		return nil
	}
}

// MarshalText encodes EncodingType to text.
func (e EncodingType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// EncodingTypeThriftRW is an option for EncodingType
	EncodingTypeThriftRW EncodingType = iota
	// EncodingTypeJSON is an option for EncodingType
	EncodingTypeJSON
)

// EntityNotExistsError is an internal type (TBD...)
type EntityNotExistsError struct {
	Message        string `json:"message,required"`
	CurrentCluster string `json:"currentCluster,omitempty"`
	ActiveCluster  string `json:"activeCluster,omitempty"`
}

// WorkflowExecutionAlreadyCompletedError is an internal type (TBD...)
type WorkflowExecutionAlreadyCompletedError struct {
	Message string `json:"message,required"`
}

// EventType is an internal type (TBD...)
type EventType int32

// Ptr is a helper function for getting pointer value
func (e EventType) Ptr() *EventType {
	return &e
}

// String returns a readable string representation of EventType.
func (e EventType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "WorkflowExecutionStarted"
	case 1:
		return "WorkflowExecutionCompleted"
	case 2:
		return "WorkflowExecutionFailed"
	case 3:
		return "WorkflowExecutionTimedOut"
	case 4:
		return "DecisionTaskScheduled"
	case 5:
		return "DecisionTaskStarted"
	case 6:
		return "DecisionTaskCompleted"
	case 7:
		return "DecisionTaskTimedOut"
	case 8:
		return "DecisionTaskFailed"
	case 9:
		return "ActivityTaskScheduled"
	case 10:
		return "ActivityTaskStarted"
	case 11:
		return "ActivityTaskCompleted"
	case 12:
		return "ActivityTaskFailed"
	case 13:
		return "ActivityTaskTimedOut"
	case 14:
		return "ActivityTaskCancelRequested"
	case 15:
		return "RequestCancelActivityTaskFailed"
	case 16:
		return "ActivityTaskCanceled"
	case 17:
		return "TimerStarted"
	case 18:
		return "TimerFired"
	case 19:
		return "CancelTimerFailed"
	case 20:
		return "TimerCanceled"
	case 21:
		return "WorkflowExecutionCancelRequested"
	case 22:
		return "WorkflowExecutionCanceled"
	case 23:
		return "RequestCancelExternalWorkflowExecutionInitiated"
	case 24:
		return "RequestCancelExternalWorkflowExecutionFailed"
	case 25:
		return "ExternalWorkflowExecutionCancelRequested"
	case 26:
		return "MarkerRecorded"
	case 27:
		return "WorkflowExecutionSignaled"
	case 28:
		return "WorkflowExecutionTerminated"
	case 29:
		return "WorkflowExecutionContinuedAsNew"
	case 30:
		return "StartChildWorkflowExecutionInitiated"
	case 31:
		return "StartChildWorkflowExecutionFailed"
	case 32:
		return "ChildWorkflowExecutionStarted"
	case 33:
		return "ChildWorkflowExecutionCompleted"
	case 34:
		return "ChildWorkflowExecutionFailed"
	case 35:
		return "ChildWorkflowExecutionCanceled"
	case 36:
		return "ChildWorkflowExecutionTimedOut"
	case 37:
		return "ChildWorkflowExecutionTerminated"
	case 38:
		return "SignalExternalWorkflowExecutionInitiated"
	case 39:
		return "SignalExternalWorkflowExecutionFailed"
	case 40:
		return "ExternalWorkflowExecutionSignaled"
	case 41:
		return "UpsertWorkflowSearchAttributes"
	}
	return fmt.Sprintf("EventType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *EventType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "WORKFLOWEXECUTIONSTARTED":
		*e = EventTypeWorkflowExecutionStarted
		return nil
	case "WORKFLOWEXECUTIONCOMPLETED":
		*e = EventTypeWorkflowExecutionCompleted
		return nil
	case "WORKFLOWEXECUTIONFAILED":
		*e = EventTypeWorkflowExecutionFailed
		return nil
	case "WORKFLOWEXECUTIONTIMEDOUT":
		*e = EventTypeWorkflowExecutionTimedOut
		return nil
	case "DECISIONTASKSCHEDULED":
		*e = EventTypeDecisionTaskScheduled
		return nil
	case "DECISIONTASKSTARTED":
		*e = EventTypeDecisionTaskStarted
		return nil
	case "DECISIONTASKCOMPLETED":
		*e = EventTypeDecisionTaskCompleted
		return nil
	case "DECISIONTASKTIMEDOUT":
		*e = EventTypeDecisionTaskTimedOut
		return nil
	case "DECISIONTASKFAILED":
		*e = EventTypeDecisionTaskFailed
		return nil
	case "ACTIVITYTASKSCHEDULED":
		*e = EventTypeActivityTaskScheduled
		return nil
	case "ACTIVITYTASKSTARTED":
		*e = EventTypeActivityTaskStarted
		return nil
	case "ACTIVITYTASKCOMPLETED":
		*e = EventTypeActivityTaskCompleted
		return nil
	case "ACTIVITYTASKFAILED":
		*e = EventTypeActivityTaskFailed
		return nil
	case "ACTIVITYTASKTIMEDOUT":
		*e = EventTypeActivityTaskTimedOut
		return nil
	case "ACTIVITYTASKCANCELREQUESTED":
		*e = EventTypeActivityTaskCancelRequested
		return nil
	case "REQUESTCANCELACTIVITYTASKFAILED":
		*e = EventTypeRequestCancelActivityTaskFailed
		return nil
	case "ACTIVITYTASKCANCELED":
		*e = EventTypeActivityTaskCanceled
		return nil
	case "TIMERSTARTED":
		*e = EventTypeTimerStarted
		return nil
	case "TIMERFIRED":
		*e = EventTypeTimerFired
		return nil
	case "CANCELTIMERFAILED":
		*e = EventTypeCancelTimerFailed
		return nil
	case "TIMERCANCELED":
		*e = EventTypeTimerCanceled
		return nil
	case "WORKFLOWEXECUTIONCANCELREQUESTED":
		*e = EventTypeWorkflowExecutionCancelRequested
		return nil
	case "WORKFLOWEXECUTIONCANCELED":
		*e = EventTypeWorkflowExecutionCanceled
		return nil
	case "REQUESTCANCELEXTERNALWORKFLOWEXECUTIONINITIATED":
		*e = EventTypeRequestCancelExternalWorkflowExecutionInitiated
		return nil
	case "REQUESTCANCELEXTERNALWORKFLOWEXECUTIONFAILED":
		*e = EventTypeRequestCancelExternalWorkflowExecutionFailed
		return nil
	case "EXTERNALWORKFLOWEXECUTIONCANCELREQUESTED":
		*e = EventTypeExternalWorkflowExecutionCancelRequested
		return nil
	case "MARKERRECORDED":
		*e = EventTypeMarkerRecorded
		return nil
	case "WORKFLOWEXECUTIONSIGNALED":
		*e = EventTypeWorkflowExecutionSignaled
		return nil
	case "WORKFLOWEXECUTIONTERMINATED":
		*e = EventTypeWorkflowExecutionTerminated
		return nil
	case "WORKFLOWEXECUTIONCONTINUEDASNEW":
		*e = EventTypeWorkflowExecutionContinuedAsNew
		return nil
	case "STARTCHILDWORKFLOWEXECUTIONINITIATED":
		*e = EventTypeStartChildWorkflowExecutionInitiated
		return nil
	case "STARTCHILDWORKFLOWEXECUTIONFAILED":
		*e = EventTypeStartChildWorkflowExecutionFailed
		return nil
	case "CHILDWORKFLOWEXECUTIONSTARTED":
		*e = EventTypeChildWorkflowExecutionStarted
		return nil
	case "CHILDWORKFLOWEXECUTIONCOMPLETED":
		*e = EventTypeChildWorkflowExecutionCompleted
		return nil
	case "CHILDWORKFLOWEXECUTIONFAILED":
		*e = EventTypeChildWorkflowExecutionFailed
		return nil
	case "CHILDWORKFLOWEXECUTIONCANCELED":
		*e = EventTypeChildWorkflowExecutionCanceled
		return nil
	case "CHILDWORKFLOWEXECUTIONTIMEDOUT":
		*e = EventTypeChildWorkflowExecutionTimedOut
		return nil
	case "CHILDWORKFLOWEXECUTIONTERMINATED":
		*e = EventTypeChildWorkflowExecutionTerminated
		return nil
	case "SIGNALEXTERNALWORKFLOWEXECUTIONINITIATED":
		*e = EventTypeSignalExternalWorkflowExecutionInitiated
		return nil
	case "SIGNALEXTERNALWORKFLOWEXECUTIONFAILED":
		*e = EventTypeSignalExternalWorkflowExecutionFailed
		return nil
	case "EXTERNALWORKFLOWEXECUTIONSIGNALED":
		*e = EventTypeExternalWorkflowExecutionSignaled
		return nil
	case "UPSERTWORKFLOWSEARCHATTRIBUTES":
		*e = EventTypeUpsertWorkflowSearchAttributes
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "EventType", err)
		}
		*e = EventType(val)
		return nil
	}
}

// MarshalText encodes EventType to text.
func (e EventType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// EventTypeWorkflowExecutionStarted is an option for EventType
	EventTypeWorkflowExecutionStarted EventType = iota
	// EventTypeWorkflowExecutionCompleted is an option for EventType
	EventTypeWorkflowExecutionCompleted
	// EventTypeWorkflowExecutionFailed is an option for EventType
	EventTypeWorkflowExecutionFailed
	// EventTypeWorkflowExecutionTimedOut is an option for EventType
	EventTypeWorkflowExecutionTimedOut
	// EventTypeDecisionTaskScheduled is an option for EventType
	EventTypeDecisionTaskScheduled
	// EventTypeDecisionTaskStarted is an option for EventType
	EventTypeDecisionTaskStarted
	// EventTypeDecisionTaskCompleted is an option for EventType
	EventTypeDecisionTaskCompleted
	// EventTypeDecisionTaskTimedOut is an option for EventType
	EventTypeDecisionTaskTimedOut
	// EventTypeDecisionTaskFailed is an option for EventType
	EventTypeDecisionTaskFailed
	// EventTypeActivityTaskScheduled is an option for EventType
	EventTypeActivityTaskScheduled
	// EventTypeActivityTaskStarted is an option for EventType
	EventTypeActivityTaskStarted
	// EventTypeActivityTaskCompleted is an option for EventType
	EventTypeActivityTaskCompleted
	// EventTypeActivityTaskFailed is an option for EventType
	EventTypeActivityTaskFailed
	// EventTypeActivityTaskTimedOut is an option for EventType
	EventTypeActivityTaskTimedOut
	// EventTypeActivityTaskCancelRequested is an option for EventType
	EventTypeActivityTaskCancelRequested
	// EventTypeRequestCancelActivityTaskFailed is an option for EventType
	EventTypeRequestCancelActivityTaskFailed
	// EventTypeActivityTaskCanceled is an option for EventType
	EventTypeActivityTaskCanceled
	// EventTypeTimerStarted is an option for EventType
	EventTypeTimerStarted
	// EventTypeTimerFired is an option for EventType
	EventTypeTimerFired
	// EventTypeCancelTimerFailed is an option for EventType
	EventTypeCancelTimerFailed
	// EventTypeTimerCanceled is an option for EventType
	EventTypeTimerCanceled
	// EventTypeWorkflowExecutionCancelRequested is an option for EventType
	EventTypeWorkflowExecutionCancelRequested
	// EventTypeWorkflowExecutionCanceled is an option for EventType
	EventTypeWorkflowExecutionCanceled
	// EventTypeRequestCancelExternalWorkflowExecutionInitiated is an option for EventType
	EventTypeRequestCancelExternalWorkflowExecutionInitiated
	// EventTypeRequestCancelExternalWorkflowExecutionFailed is an option for EventType
	EventTypeRequestCancelExternalWorkflowExecutionFailed
	// EventTypeExternalWorkflowExecutionCancelRequested is an option for EventType
	EventTypeExternalWorkflowExecutionCancelRequested
	// EventTypeMarkerRecorded is an option for EventType
	EventTypeMarkerRecorded
	// EventTypeWorkflowExecutionSignaled is an option for EventType
	EventTypeWorkflowExecutionSignaled
	// EventTypeWorkflowExecutionTerminated is an option for EventType
	EventTypeWorkflowExecutionTerminated
	// EventTypeWorkflowExecutionContinuedAsNew is an option for EventType
	EventTypeWorkflowExecutionContinuedAsNew
	// EventTypeStartChildWorkflowExecutionInitiated is an option for EventType
	EventTypeStartChildWorkflowExecutionInitiated
	// EventTypeStartChildWorkflowExecutionFailed is an option for EventType
	EventTypeStartChildWorkflowExecutionFailed
	// EventTypeChildWorkflowExecutionStarted is an option for EventType
	EventTypeChildWorkflowExecutionStarted
	// EventTypeChildWorkflowExecutionCompleted is an option for EventType
	EventTypeChildWorkflowExecutionCompleted
	// EventTypeChildWorkflowExecutionFailed is an option for EventType
	EventTypeChildWorkflowExecutionFailed
	// EventTypeChildWorkflowExecutionCanceled is an option for EventType
	EventTypeChildWorkflowExecutionCanceled
	// EventTypeChildWorkflowExecutionTimedOut is an option for EventType
	EventTypeChildWorkflowExecutionTimedOut
	// EventTypeChildWorkflowExecutionTerminated is an option for EventType
	EventTypeChildWorkflowExecutionTerminated
	// EventTypeSignalExternalWorkflowExecutionInitiated is an option for EventType
	EventTypeSignalExternalWorkflowExecutionInitiated
	// EventTypeSignalExternalWorkflowExecutionFailed is an option for EventType
	EventTypeSignalExternalWorkflowExecutionFailed
	// EventTypeExternalWorkflowExecutionSignaled is an option for EventType
	EventTypeExternalWorkflowExecutionSignaled
	// EventTypeUpsertWorkflowSearchAttributes is an option for EventType
	EventTypeUpsertWorkflowSearchAttributes
)

// ExternalWorkflowExecutionCancelRequestedEventAttributes is an internal type (TBD...)
type ExternalWorkflowExecutionCancelRequestedEventAttributes struct {
	InitiatedEventID  int64              `json:"initiatedEventId,omitempty"`
	Domain            string             `json:"domain,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *ExternalWorkflowExecutionCancelRequestedEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// GetDomain is an internal getter (TBD...)
func (v *ExternalWorkflowExecutionCancelRequestedEventAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// ExternalWorkflowExecutionSignaledEventAttributes is an internal type (TBD...)
type ExternalWorkflowExecutionSignaledEventAttributes struct {
	InitiatedEventID  int64              `json:"initiatedEventId,omitempty"`
	Domain            string             `json:"domain,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	Control           []byte             `json:"control,omitempty"`
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *ExternalWorkflowExecutionSignaledEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// GetDomain is an internal getter (TBD...)
func (v *ExternalWorkflowExecutionSignaledEventAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// FailWorkflowExecutionDecisionAttributes is an internal type (TBD...)
type FailWorkflowExecutionDecisionAttributes struct {
	Reason  *string `json:"reason,omitempty"`
	Details []byte  `json:"details,omitempty"`
}

// GetReason is an internal getter (TBD...)
func (v *FailWorkflowExecutionDecisionAttributes) GetReason() (o string) {
	if v != nil && v.Reason != nil {
		return *v.Reason
	}
	return
}

// GetSearchAttributesResponse is an internal type (TBD...)
type GetSearchAttributesResponse struct {
	Keys map[string]IndexedValueType `json:"keys,omitempty"`
}

// GetKeys is an internal getter (TBD...)
func (v *GetSearchAttributesResponse) GetKeys() (o map[string]IndexedValueType) {
	if v != nil && v.Keys != nil {
		return v.Keys
	}
	return
}

// GetWorkflowExecutionHistoryRequest is an internal type (TBD...)
type GetWorkflowExecutionHistoryRequest struct {
	Domain                 string                  `json:"domain,omitempty"`
	Execution              *WorkflowExecution      `json:"execution,omitempty"`
	MaximumPageSize        int32                   `json:"maximumPageSize,omitempty"`
	NextPageToken          []byte                  `json:"nextPageToken,omitempty"`
	WaitForNewEvent        bool                    `json:"waitForNewEvent,omitempty"`
	HistoryEventFilterType *HistoryEventFilterType `json:"HistoryEventFilterType,omitempty"`
	SkipArchival           bool                    `json:"skipArchival,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *GetWorkflowExecutionHistoryRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetExecution is an internal getter (TBD...)
func (v *GetWorkflowExecutionHistoryRequest) GetExecution() (o *WorkflowExecution) {
	if v != nil && v.Execution != nil {
		return v.Execution
	}
	return
}

// GetMaximumPageSize is an internal getter (TBD...)
func (v *GetWorkflowExecutionHistoryRequest) GetMaximumPageSize() (o int32) {
	if v != nil {
		return v.MaximumPageSize
	}
	return
}

// GetNextPageToken is an internal getter (TBD...)
func (v *GetWorkflowExecutionHistoryRequest) GetNextPageToken() (o []byte) {
	if v != nil && v.NextPageToken != nil {
		return v.NextPageToken
	}
	return
}

// GetWaitForNewEvent is an internal getter (TBD...)
func (v *GetWorkflowExecutionHistoryRequest) GetWaitForNewEvent() (o bool) {
	if v != nil {
		return v.WaitForNewEvent
	}
	return
}

// GetHistoryEventFilterType is an internal getter (TBD...)
func (v *GetWorkflowExecutionHistoryRequest) GetHistoryEventFilterType() (o HistoryEventFilterType) {
	if v != nil && v.HistoryEventFilterType != nil {
		return *v.HistoryEventFilterType
	}
	return
}

// GetSkipArchival is an internal getter (TBD...)
func (v *GetWorkflowExecutionHistoryRequest) GetSkipArchival() (o bool) {
	if v != nil {
		return v.SkipArchival
	}
	return
}

// GetWorkflowExecutionHistoryResponse is an internal type (TBD...)
type GetWorkflowExecutionHistoryResponse struct {
	History       *History    `json:"history,omitempty"`
	RawHistory    []*DataBlob `json:"rawHistory,omitempty"`
	NextPageToken []byte      `json:"nextPageToken,omitempty"`
	Archived      bool        `json:"archived,omitempty"`
}

// GetHistory is an internal getter (TBD...)
func (v *GetWorkflowExecutionHistoryResponse) GetHistory() (o *History) {
	if v != nil && v.History != nil {
		return v.History
	}
	return
}

// GetArchived is an internal getter (TBD...)
func (v *GetWorkflowExecutionHistoryResponse) GetArchived() (o bool) {
	if v != nil {
		return v.Archived
	}
	return
}

// FailoverInfo is an internal type (TBD...)
type FailoverInfo struct {
	FailoverVersion         int64   `json:"failoverVersion,omitempty"`
	FailoverStartTimestamp  int64   `json:"failoverStartTimestamp,omitempty"`
	FailoverExpireTimestamp int64   `json:"failoverExpireTimestamp,omitempty"`
	CompletedShardCount     int32   `json:"completedShardCount,omitempty"`
	PendingShards           []int32 `json:"pendingShards,omitempty"`
}

// GetFailoverVersion is an internal getter (TBD...)
func (v *FailoverInfo) GetFailoverVersion() (o int64) {
	if v != nil {
		return v.FailoverVersion
	}
	return
}

// GetFailoverStartTimestamp is an internal getter (TBD...)
func (v *FailoverInfo) GetFailoverStartTimestamp() (o int64) {
	if v != nil {
		return v.FailoverStartTimestamp
	}
	return
}

// GetFailoverExpireTimestamp is an internal getter (TBD...)
func (v *FailoverInfo) GetFailoverExpireTimestamp() (o int64) {
	if v != nil {
		return v.FailoverExpireTimestamp
	}
	return
}

// GetCompletedShardCount is an internal getter (TBD...)
func (v *FailoverInfo) GetCompletedShardCount() (o int32) {
	if v != nil {
		return v.CompletedShardCount
	}
	return
}

// GetPendingShards is an internal getter (TBD...)
func (v *FailoverInfo) GetPendingShards() (o []int32) {
	if v != nil {
		return v.PendingShards
	}
	return
}

// Header is an internal type (TBD...)
type Header struct {
	Fields map[string][]byte `json:"fields,omitempty"`
}

// History is an internal type (TBD...)
type History struct {
	Events []*HistoryEvent `json:"events,omitempty"`
}

// GetEvents is an internal getter (TBD...)
func (v *History) GetEvents() (o []*HistoryEvent) {
	if v != nil && v.Events != nil {
		return v.Events
	}
	return
}

// HistoryBranch is an internal type (TBD...)
type HistoryBranch struct {
	TreeID    string
	BranchID  string
	Ancestors []*HistoryBranchRange
}

// HistoryBranchRange is an internal type (TBD...)
type HistoryBranchRange struct {
	BranchID    string
	BeginNodeID int64
	EndNodeID   int64
}

// HistoryEvent is an internal type (TBD...)
type HistoryEvent struct {
	ID                                                             int64                                                           `json:"eventId,omitempty"`
	Timestamp                                                      *int64                                                          `json:"timestamp,omitempty"`
	EventType                                                      *EventType                                                      `json:"eventType,omitempty"`
	Version                                                        int64                                                           `json:"version,omitempty"`
	TaskID                                                         int64                                                           `json:"taskId,omitempty"`
	WorkflowExecutionStartedEventAttributes                        *WorkflowExecutionStartedEventAttributes                        `json:"workflowExecutionStartedEventAttributes,omitempty"`
	WorkflowExecutionCompletedEventAttributes                      *WorkflowExecutionCompletedEventAttributes                      `json:"workflowExecutionCompletedEventAttributes,omitempty"`
	WorkflowExecutionFailedEventAttributes                         *WorkflowExecutionFailedEventAttributes                         `json:"workflowExecutionFailedEventAttributes,omitempty"`
	WorkflowExecutionTimedOutEventAttributes                       *WorkflowExecutionTimedOutEventAttributes                       `json:"workflowExecutionTimedOutEventAttributes,omitempty"`
	DecisionTaskScheduledEventAttributes                           *DecisionTaskScheduledEventAttributes                           `json:"decisionTaskScheduledEventAttributes,omitempty"`
	DecisionTaskStartedEventAttributes                             *DecisionTaskStartedEventAttributes                             `json:"decisionTaskStartedEventAttributes,omitempty"`
	DecisionTaskCompletedEventAttributes                           *DecisionTaskCompletedEventAttributes                           `json:"decisionTaskCompletedEventAttributes,omitempty"`
	DecisionTaskTimedOutEventAttributes                            *DecisionTaskTimedOutEventAttributes                            `json:"decisionTaskTimedOutEventAttributes,omitempty"`
	DecisionTaskFailedEventAttributes                              *DecisionTaskFailedEventAttributes                              `json:"decisionTaskFailedEventAttributes,omitempty"`
	ActivityTaskScheduledEventAttributes                           *ActivityTaskScheduledEventAttributes                           `json:"activityTaskScheduledEventAttributes,omitempty"`
	ActivityTaskStartedEventAttributes                             *ActivityTaskStartedEventAttributes                             `json:"activityTaskStartedEventAttributes,omitempty"`
	ActivityTaskCompletedEventAttributes                           *ActivityTaskCompletedEventAttributes                           `json:"activityTaskCompletedEventAttributes,omitempty"`
	ActivityTaskFailedEventAttributes                              *ActivityTaskFailedEventAttributes                              `json:"activityTaskFailedEventAttributes,omitempty"`
	ActivityTaskTimedOutEventAttributes                            *ActivityTaskTimedOutEventAttributes                            `json:"activityTaskTimedOutEventAttributes,omitempty"`
	TimerStartedEventAttributes                                    *TimerStartedEventAttributes                                    `json:"timerStartedEventAttributes,omitempty"`
	TimerFiredEventAttributes                                      *TimerFiredEventAttributes                                      `json:"timerFiredEventAttributes,omitempty"`
	ActivityTaskCancelRequestedEventAttributes                     *ActivityTaskCancelRequestedEventAttributes                     `json:"activityTaskCancelRequestedEventAttributes,omitempty"`
	RequestCancelActivityTaskFailedEventAttributes                 *RequestCancelActivityTaskFailedEventAttributes                 `json:"requestCancelActivityTaskFailedEventAttributes,omitempty"`
	ActivityTaskCanceledEventAttributes                            *ActivityTaskCanceledEventAttributes                            `json:"activityTaskCanceledEventAttributes,omitempty"`
	TimerCanceledEventAttributes                                   *TimerCanceledEventAttributes                                   `json:"timerCanceledEventAttributes,omitempty"`
	CancelTimerFailedEventAttributes                               *CancelTimerFailedEventAttributes                               `json:"cancelTimerFailedEventAttributes,omitempty"`
	MarkerRecordedEventAttributes                                  *MarkerRecordedEventAttributes                                  `json:"markerRecordedEventAttributes,omitempty"`
	WorkflowExecutionSignaledEventAttributes                       *WorkflowExecutionSignaledEventAttributes                       `json:"workflowExecutionSignaledEventAttributes,omitempty"`
	WorkflowExecutionTerminatedEventAttributes                     *WorkflowExecutionTerminatedEventAttributes                     `json:"workflowExecutionTerminatedEventAttributes,omitempty"`
	WorkflowExecutionCancelRequestedEventAttributes                *WorkflowExecutionCancelRequestedEventAttributes                `json:"workflowExecutionCancelRequestedEventAttributes,omitempty"`
	WorkflowExecutionCanceledEventAttributes                       *WorkflowExecutionCanceledEventAttributes                       `json:"workflowExecutionCanceledEventAttributes,omitempty"`
	RequestCancelExternalWorkflowExecutionInitiatedEventAttributes *RequestCancelExternalWorkflowExecutionInitiatedEventAttributes `json:"requestCancelExternalWorkflowExecutionInitiatedEventAttributes,omitempty"`
	RequestCancelExternalWorkflowExecutionFailedEventAttributes    *RequestCancelExternalWorkflowExecutionFailedEventAttributes    `json:"requestCancelExternalWorkflowExecutionFailedEventAttributes,omitempty"`
	ExternalWorkflowExecutionCancelRequestedEventAttributes        *ExternalWorkflowExecutionCancelRequestedEventAttributes        `json:"externalWorkflowExecutionCancelRequestedEventAttributes,omitempty"`
	WorkflowExecutionContinuedAsNewEventAttributes                 *WorkflowExecutionContinuedAsNewEventAttributes                 `json:"workflowExecutionContinuedAsNewEventAttributes,omitempty"`
	StartChildWorkflowExecutionInitiatedEventAttributes            *StartChildWorkflowExecutionInitiatedEventAttributes            `json:"startChildWorkflowExecutionInitiatedEventAttributes,omitempty"`
	StartChildWorkflowExecutionFailedEventAttributes               *StartChildWorkflowExecutionFailedEventAttributes               `json:"startChildWorkflowExecutionFailedEventAttributes,omitempty"`
	ChildWorkflowExecutionStartedEventAttributes                   *ChildWorkflowExecutionStartedEventAttributes                   `json:"childWorkflowExecutionStartedEventAttributes,omitempty"`
	ChildWorkflowExecutionCompletedEventAttributes                 *ChildWorkflowExecutionCompletedEventAttributes                 `json:"childWorkflowExecutionCompletedEventAttributes,omitempty"`
	ChildWorkflowExecutionFailedEventAttributes                    *ChildWorkflowExecutionFailedEventAttributes                    `json:"childWorkflowExecutionFailedEventAttributes,omitempty"`
	ChildWorkflowExecutionCanceledEventAttributes                  *ChildWorkflowExecutionCanceledEventAttributes                  `json:"childWorkflowExecutionCanceledEventAttributes,omitempty"`
	ChildWorkflowExecutionTimedOutEventAttributes                  *ChildWorkflowExecutionTimedOutEventAttributes                  `json:"childWorkflowExecutionTimedOutEventAttributes,omitempty"`
	ChildWorkflowExecutionTerminatedEventAttributes                *ChildWorkflowExecutionTerminatedEventAttributes                `json:"childWorkflowExecutionTerminatedEventAttributes,omitempty"`
	SignalExternalWorkflowExecutionInitiatedEventAttributes        *SignalExternalWorkflowExecutionInitiatedEventAttributes        `json:"signalExternalWorkflowExecutionInitiatedEventAttributes,omitempty"`
	SignalExternalWorkflowExecutionFailedEventAttributes           *SignalExternalWorkflowExecutionFailedEventAttributes           `json:"signalExternalWorkflowExecutionFailedEventAttributes,omitempty"`
	ExternalWorkflowExecutionSignaledEventAttributes               *ExternalWorkflowExecutionSignaledEventAttributes               `json:"externalWorkflowExecutionSignaledEventAttributes,omitempty"`
	UpsertWorkflowSearchAttributesEventAttributes                  *UpsertWorkflowSearchAttributesEventAttributes                  `json:"upsertWorkflowSearchAttributesEventAttributes,omitempty"`
}

// GetTimestamp is an internal getter (TBD...)
func (v *HistoryEvent) GetTimestamp() (o int64) {
	if v != nil && v.Timestamp != nil {
		return *v.Timestamp
	}
	return
}

// GetEventType is an internal getter (TBD...)
func (v *HistoryEvent) GetEventType() (o EventType) {
	if v != nil && v.EventType != nil {
		return *v.EventType
	}
	return
}

// GetWorkflowExecutionStartedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetWorkflowExecutionStartedEventAttributes() (o *WorkflowExecutionStartedEventAttributes) {
	if v != nil && v.WorkflowExecutionStartedEventAttributes != nil {
		return v.WorkflowExecutionStartedEventAttributes
	}
	return
}

// GetWorkflowExecutionCompletedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetWorkflowExecutionCompletedEventAttributes() (o *WorkflowExecutionCompletedEventAttributes) {
	if v != nil && v.WorkflowExecutionCompletedEventAttributes != nil {
		return v.WorkflowExecutionCompletedEventAttributes
	}
	return
}

// GetWorkflowExecutionFailedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetWorkflowExecutionFailedEventAttributes() (o *WorkflowExecutionFailedEventAttributes) {
	if v != nil && v.WorkflowExecutionFailedEventAttributes != nil {
		return v.WorkflowExecutionFailedEventAttributes
	}
	return
}

// GetWorkflowExecutionTimedOutEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetWorkflowExecutionTimedOutEventAttributes() (o *WorkflowExecutionTimedOutEventAttributes) {
	if v != nil && v.WorkflowExecutionTimedOutEventAttributes != nil {
		return v.WorkflowExecutionTimedOutEventAttributes
	}
	return
}

// GetDecisionTaskScheduledEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetDecisionTaskScheduledEventAttributes() (o *DecisionTaskScheduledEventAttributes) {
	if v != nil && v.DecisionTaskScheduledEventAttributes != nil {
		return v.DecisionTaskScheduledEventAttributes
	}
	return
}

// GetDecisionTaskStartedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetDecisionTaskStartedEventAttributes() (o *DecisionTaskStartedEventAttributes) {
	if v != nil && v.DecisionTaskStartedEventAttributes != nil {
		return v.DecisionTaskStartedEventAttributes
	}
	return
}

// GetDecisionTaskCompletedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetDecisionTaskCompletedEventAttributes() (o *DecisionTaskCompletedEventAttributes) {
	if v != nil && v.DecisionTaskCompletedEventAttributes != nil {
		return v.DecisionTaskCompletedEventAttributes
	}
	return
}

// GetDecisionTaskTimedOutEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetDecisionTaskTimedOutEventAttributes() (o *DecisionTaskTimedOutEventAttributes) {
	if v != nil && v.DecisionTaskTimedOutEventAttributes != nil {
		return v.DecisionTaskTimedOutEventAttributes
	}
	return
}

// GetDecisionTaskFailedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetDecisionTaskFailedEventAttributes() (o *DecisionTaskFailedEventAttributes) {
	if v != nil && v.DecisionTaskFailedEventAttributes != nil {
		return v.DecisionTaskFailedEventAttributes
	}
	return
}

// GetActivityTaskScheduledEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetActivityTaskScheduledEventAttributes() (o *ActivityTaskScheduledEventAttributes) {
	if v != nil && v.ActivityTaskScheduledEventAttributes != nil {
		return v.ActivityTaskScheduledEventAttributes
	}
	return
}

// GetActivityTaskStartedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetActivityTaskStartedEventAttributes() (o *ActivityTaskStartedEventAttributes) {
	if v != nil && v.ActivityTaskStartedEventAttributes != nil {
		return v.ActivityTaskStartedEventAttributes
	}
	return
}

// GetActivityTaskCompletedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetActivityTaskCompletedEventAttributes() (o *ActivityTaskCompletedEventAttributes) {
	if v != nil && v.ActivityTaskCompletedEventAttributes != nil {
		return v.ActivityTaskCompletedEventAttributes
	}
	return
}

// GetActivityTaskFailedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetActivityTaskFailedEventAttributes() (o *ActivityTaskFailedEventAttributes) {
	if v != nil && v.ActivityTaskFailedEventAttributes != nil {
		return v.ActivityTaskFailedEventAttributes
	}
	return
}

// GetActivityTaskTimedOutEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetActivityTaskTimedOutEventAttributes() (o *ActivityTaskTimedOutEventAttributes) {
	if v != nil && v.ActivityTaskTimedOutEventAttributes != nil {
		return v.ActivityTaskTimedOutEventAttributes
	}
	return
}

// GetTimerStartedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetTimerStartedEventAttributes() (o *TimerStartedEventAttributes) {
	if v != nil && v.TimerStartedEventAttributes != nil {
		return v.TimerStartedEventAttributes
	}
	return
}

// GetTimerFiredEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetTimerFiredEventAttributes() (o *TimerFiredEventAttributes) {
	if v != nil && v.TimerFiredEventAttributes != nil {
		return v.TimerFiredEventAttributes
	}
	return
}

// GetActivityTaskCancelRequestedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetActivityTaskCancelRequestedEventAttributes() (o *ActivityTaskCancelRequestedEventAttributes) {
	if v != nil && v.ActivityTaskCancelRequestedEventAttributes != nil {
		return v.ActivityTaskCancelRequestedEventAttributes
	}
	return
}

// GetRequestCancelActivityTaskFailedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetRequestCancelActivityTaskFailedEventAttributes() (o *RequestCancelActivityTaskFailedEventAttributes) {
	if v != nil && v.RequestCancelActivityTaskFailedEventAttributes != nil {
		return v.RequestCancelActivityTaskFailedEventAttributes
	}
	return
}

// GetActivityTaskCanceledEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetActivityTaskCanceledEventAttributes() (o *ActivityTaskCanceledEventAttributes) {
	if v != nil && v.ActivityTaskCanceledEventAttributes != nil {
		return v.ActivityTaskCanceledEventAttributes
	}
	return
}

// GetTimerCanceledEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetTimerCanceledEventAttributes() (o *TimerCanceledEventAttributes) {
	if v != nil && v.TimerCanceledEventAttributes != nil {
		return v.TimerCanceledEventAttributes
	}
	return
}

// GetCancelTimerFailedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetCancelTimerFailedEventAttributes() (o *CancelTimerFailedEventAttributes) {
	if v != nil && v.CancelTimerFailedEventAttributes != nil {
		return v.CancelTimerFailedEventAttributes
	}
	return
}

// GetMarkerRecordedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetMarkerRecordedEventAttributes() (o *MarkerRecordedEventAttributes) {
	if v != nil && v.MarkerRecordedEventAttributes != nil {
		return v.MarkerRecordedEventAttributes
	}
	return
}

// GetWorkflowExecutionSignaledEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetWorkflowExecutionSignaledEventAttributes() (o *WorkflowExecutionSignaledEventAttributes) {
	if v != nil && v.WorkflowExecutionSignaledEventAttributes != nil {
		return v.WorkflowExecutionSignaledEventAttributes
	}
	return
}

// GetWorkflowExecutionTerminatedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetWorkflowExecutionTerminatedEventAttributes() (o *WorkflowExecutionTerminatedEventAttributes) {
	if v != nil && v.WorkflowExecutionTerminatedEventAttributes != nil {
		return v.WorkflowExecutionTerminatedEventAttributes
	}
	return
}

// GetWorkflowExecutionCancelRequestedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetWorkflowExecutionCancelRequestedEventAttributes() (o *WorkflowExecutionCancelRequestedEventAttributes) {
	if v != nil && v.WorkflowExecutionCancelRequestedEventAttributes != nil {
		return v.WorkflowExecutionCancelRequestedEventAttributes
	}
	return
}

// GetWorkflowExecutionCanceledEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetWorkflowExecutionCanceledEventAttributes() (o *WorkflowExecutionCanceledEventAttributes) {
	if v != nil && v.WorkflowExecutionCanceledEventAttributes != nil {
		return v.WorkflowExecutionCanceledEventAttributes
	}
	return
}

// GetRequestCancelExternalWorkflowExecutionInitiatedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetRequestCancelExternalWorkflowExecutionInitiatedEventAttributes() (o *RequestCancelExternalWorkflowExecutionInitiatedEventAttributes) {
	if v != nil && v.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes != nil {
		return v.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes
	}
	return
}

// GetRequestCancelExternalWorkflowExecutionFailedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetRequestCancelExternalWorkflowExecutionFailedEventAttributes() (o *RequestCancelExternalWorkflowExecutionFailedEventAttributes) {
	if v != nil && v.RequestCancelExternalWorkflowExecutionFailedEventAttributes != nil {
		return v.RequestCancelExternalWorkflowExecutionFailedEventAttributes
	}
	return
}

// GetExternalWorkflowExecutionCancelRequestedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetExternalWorkflowExecutionCancelRequestedEventAttributes() (o *ExternalWorkflowExecutionCancelRequestedEventAttributes) {
	if v != nil && v.ExternalWorkflowExecutionCancelRequestedEventAttributes != nil {
		return v.ExternalWorkflowExecutionCancelRequestedEventAttributes
	}
	return
}

// GetWorkflowExecutionContinuedAsNewEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetWorkflowExecutionContinuedAsNewEventAttributes() (o *WorkflowExecutionContinuedAsNewEventAttributes) {
	if v != nil && v.WorkflowExecutionContinuedAsNewEventAttributes != nil {
		return v.WorkflowExecutionContinuedAsNewEventAttributes
	}
	return
}

// GetStartChildWorkflowExecutionInitiatedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetStartChildWorkflowExecutionInitiatedEventAttributes() (o *StartChildWorkflowExecutionInitiatedEventAttributes) {
	if v != nil && v.StartChildWorkflowExecutionInitiatedEventAttributes != nil {
		return v.StartChildWorkflowExecutionInitiatedEventAttributes
	}
	return
}

// GetStartChildWorkflowExecutionFailedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetStartChildWorkflowExecutionFailedEventAttributes() (o *StartChildWorkflowExecutionFailedEventAttributes) {
	if v != nil && v.StartChildWorkflowExecutionFailedEventAttributes != nil {
		return v.StartChildWorkflowExecutionFailedEventAttributes
	}
	return
}

// GetChildWorkflowExecutionStartedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetChildWorkflowExecutionStartedEventAttributes() (o *ChildWorkflowExecutionStartedEventAttributes) {
	if v != nil && v.ChildWorkflowExecutionStartedEventAttributes != nil {
		return v.ChildWorkflowExecutionStartedEventAttributes
	}
	return
}

// GetChildWorkflowExecutionCompletedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetChildWorkflowExecutionCompletedEventAttributes() (o *ChildWorkflowExecutionCompletedEventAttributes) {
	if v != nil && v.ChildWorkflowExecutionCompletedEventAttributes != nil {
		return v.ChildWorkflowExecutionCompletedEventAttributes
	}
	return
}

// GetChildWorkflowExecutionFailedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetChildWorkflowExecutionFailedEventAttributes() (o *ChildWorkflowExecutionFailedEventAttributes) {
	if v != nil && v.ChildWorkflowExecutionFailedEventAttributes != nil {
		return v.ChildWorkflowExecutionFailedEventAttributes
	}
	return
}

// GetChildWorkflowExecutionCanceledEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetChildWorkflowExecutionCanceledEventAttributes() (o *ChildWorkflowExecutionCanceledEventAttributes) {
	if v != nil && v.ChildWorkflowExecutionCanceledEventAttributes != nil {
		return v.ChildWorkflowExecutionCanceledEventAttributes
	}
	return
}

// GetChildWorkflowExecutionTimedOutEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetChildWorkflowExecutionTimedOutEventAttributes() (o *ChildWorkflowExecutionTimedOutEventAttributes) {
	if v != nil && v.ChildWorkflowExecutionTimedOutEventAttributes != nil {
		return v.ChildWorkflowExecutionTimedOutEventAttributes
	}
	return
}

// GetChildWorkflowExecutionTerminatedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetChildWorkflowExecutionTerminatedEventAttributes() (o *ChildWorkflowExecutionTerminatedEventAttributes) {
	if v != nil && v.ChildWorkflowExecutionTerminatedEventAttributes != nil {
		return v.ChildWorkflowExecutionTerminatedEventAttributes
	}
	return
}

// GetSignalExternalWorkflowExecutionInitiatedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetSignalExternalWorkflowExecutionInitiatedEventAttributes() (o *SignalExternalWorkflowExecutionInitiatedEventAttributes) {
	if v != nil && v.SignalExternalWorkflowExecutionInitiatedEventAttributes != nil {
		return v.SignalExternalWorkflowExecutionInitiatedEventAttributes
	}
	return
}

// GetSignalExternalWorkflowExecutionFailedEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetSignalExternalWorkflowExecutionFailedEventAttributes() (o *SignalExternalWorkflowExecutionFailedEventAttributes) {
	if v != nil && v.SignalExternalWorkflowExecutionFailedEventAttributes != nil {
		return v.SignalExternalWorkflowExecutionFailedEventAttributes
	}
	return
}

// GetExternalWorkflowExecutionSignaledEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetExternalWorkflowExecutionSignaledEventAttributes() (o *ExternalWorkflowExecutionSignaledEventAttributes) {
	if v != nil && v.ExternalWorkflowExecutionSignaledEventAttributes != nil {
		return v.ExternalWorkflowExecutionSignaledEventAttributes
	}
	return
}

// GetUpsertWorkflowSearchAttributesEventAttributes is an internal getter (TBD...)
func (v *HistoryEvent) GetUpsertWorkflowSearchAttributesEventAttributes() (o *UpsertWorkflowSearchAttributesEventAttributes) {
	if v != nil && v.UpsertWorkflowSearchAttributesEventAttributes != nil {
		return v.UpsertWorkflowSearchAttributesEventAttributes
	}
	return
}

// HistoryEventFilterType is an internal type (TBD...)
type HistoryEventFilterType int32

// Ptr is a helper function for getting pointer value
func (e HistoryEventFilterType) Ptr() *HistoryEventFilterType {
	return &e
}

// String returns a readable string representation of HistoryEventFilterType.
func (e HistoryEventFilterType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "ALL_EVENT"
	case 1:
		return "CLOSE_EVENT"
	}
	return fmt.Sprintf("HistoryEventFilterType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *HistoryEventFilterType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "ALL_EVENT":
		*e = HistoryEventFilterTypeAllEvent
		return nil
	case "CLOSE_EVENT":
		*e = HistoryEventFilterTypeCloseEvent
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "HistoryEventFilterType", err)
		}
		*e = HistoryEventFilterType(val)
		return nil
	}
}

// MarshalText encodes HistoryEventFilterType to text.
func (e HistoryEventFilterType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// HistoryEventFilterTypeAllEvent is an option for HistoryEventFilterType
	HistoryEventFilterTypeAllEvent HistoryEventFilterType = iota
	// HistoryEventFilterTypeCloseEvent is an option for HistoryEventFilterType
	HistoryEventFilterTypeCloseEvent
)

// IndexedValueType is an internal type (TBD...)
type IndexedValueType int32

// Ptr is a helper function for getting pointer value
func (e IndexedValueType) Ptr() *IndexedValueType {
	return &e
}

// String returns a readable string representation of IndexedValueType.
func (e IndexedValueType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "STRING"
	case 1:
		return "KEYWORD"
	case 2:
		return "INT"
	case 3:
		return "DOUBLE"
	case 4:
		return "BOOL"
	case 5:
		return "DATETIME"
	}
	return fmt.Sprintf("IndexedValueType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *IndexedValueType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "STRING":
		*e = IndexedValueTypeString
		return nil
	case "KEYWORD":
		*e = IndexedValueTypeKeyword
		return nil
	case "INT":
		*e = IndexedValueTypeInt
		return nil
	case "DOUBLE":
		*e = IndexedValueTypeDouble
		return nil
	case "BOOL":
		*e = IndexedValueTypeBool
		return nil
	case "DATETIME":
		*e = IndexedValueTypeDatetime
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "IndexedValueType", err)
		}
		*e = IndexedValueType(val)
		return nil
	}
}

// MarshalText encodes IndexedValueType to text.
func (e IndexedValueType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// IndexedValueTypeString is an option for IndexedValueType
	IndexedValueTypeString IndexedValueType = iota
	// IndexedValueTypeKeyword is an option for IndexedValueType
	IndexedValueTypeKeyword
	// IndexedValueTypeInt is an option for IndexedValueType
	IndexedValueTypeInt
	// IndexedValueTypeDouble is an option for IndexedValueType
	IndexedValueTypeDouble
	// IndexedValueTypeBool is an option for IndexedValueType
	IndexedValueTypeBool
	// IndexedValueTypeDatetime is an option for IndexedValueType
	IndexedValueTypeDatetime
)

// InternalDataInconsistencyError is an internal type (TBD...)
type InternalDataInconsistencyError struct {
	Message string `json:"message,required"`
}

// InternalServiceError is an internal type (TBD...)
type InternalServiceError struct {
	Message string `json:"message,required"`
}

// GetMessage is an internal getter (TBD...)
func (v *InternalServiceError) GetMessage() (o string) {
	if v != nil {
		return v.Message
	}
	return
}

// LimitExceededError is an internal type (TBD...)
type LimitExceededError struct {
	Message string `json:"message,required"`
}

// ListArchivedWorkflowExecutionsRequest is an internal type (TBD...)
type ListArchivedWorkflowExecutionsRequest struct {
	Domain        string `json:"domain,omitempty"`
	PageSize      int32  `json:"pageSize,omitempty"`
	NextPageToken []byte `json:"nextPageToken,omitempty"`
	Query         string `json:"query,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *ListArchivedWorkflowExecutionsRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetPageSize is an internal getter (TBD...)
func (v *ListArchivedWorkflowExecutionsRequest) GetPageSize() (o int32) {
	if v != nil {
		return v.PageSize
	}
	return
}

// GetQuery is an internal getter (TBD...)
func (v *ListArchivedWorkflowExecutionsRequest) GetQuery() (o string) {
	if v != nil {
		return v.Query
	}
	return
}

// ListArchivedWorkflowExecutionsResponse is an internal type (TBD...)
type ListArchivedWorkflowExecutionsResponse struct {
	Executions    []*WorkflowExecutionInfo `json:"executions,omitempty"`
	NextPageToken []byte                   `json:"nextPageToken,omitempty"`
}

// GetExecutions is an internal getter (TBD...)
func (v *ListArchivedWorkflowExecutionsResponse) GetExecutions() (o []*WorkflowExecutionInfo) {
	if v != nil && v.Executions != nil {
		return v.Executions
	}
	return
}

// ListClosedWorkflowExecutionsRequest is an internal type (TBD...)
type ListClosedWorkflowExecutionsRequest struct {
	Domain          string                        `json:"domain,omitempty"`
	MaximumPageSize int32                         `json:"maximumPageSize,omitempty"`
	NextPageToken   []byte                        `json:"nextPageToken,omitempty"`
	StartTimeFilter *StartTimeFilter              `json:"StartTimeFilter,omitempty"`
	ExecutionFilter *WorkflowExecutionFilter      `json:"executionFilter,omitempty"`
	TypeFilter      *WorkflowTypeFilter           `json:"typeFilter,omitempty"`
	StatusFilter    *WorkflowExecutionCloseStatus `json:"statusFilter,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *ListClosedWorkflowExecutionsRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetMaximumPageSize is an internal getter (TBD...)
func (v *ListClosedWorkflowExecutionsRequest) GetMaximumPageSize() (o int32) {
	if v != nil {
		return v.MaximumPageSize
	}
	return
}

// GetStatusFilter is an internal getter (TBD...)
func (v *ListClosedWorkflowExecutionsRequest) GetStatusFilter() (o WorkflowExecutionCloseStatus) {
	if v != nil && v.StatusFilter != nil {
		return *v.StatusFilter
	}
	return
}

// ListClosedWorkflowExecutionsResponse is an internal type (TBD...)
type ListClosedWorkflowExecutionsResponse struct {
	Executions    []*WorkflowExecutionInfo `json:"executions,omitempty"`
	NextPageToken []byte                   `json:"nextPageToken,omitempty"`
}

// GetExecutions is an internal getter (TBD...)
func (v *ListClosedWorkflowExecutionsResponse) GetExecutions() (o []*WorkflowExecutionInfo) {
	if v != nil && v.Executions != nil {
		return v.Executions
	}
	return
}

// ListDomainsRequest is an internal type (TBD...)
type ListDomainsRequest struct {
	PageSize      int32  `json:"pageSize,omitempty"`
	NextPageToken []byte `json:"nextPageToken,omitempty"`
}

// GetPageSize is an internal getter (TBD...)
func (v *ListDomainsRequest) GetPageSize() (o int32) {
	if v != nil {
		return v.PageSize
	}
	return
}

// ListDomainsResponse is an internal type (TBD...)
type ListDomainsResponse struct {
	Domains       []*DescribeDomainResponse `json:"domains,omitempty"`
	NextPageToken []byte                    `json:"nextPageToken,omitempty"`
}

// GetDomains is an internal getter (TBD...)
func (v *ListDomainsResponse) GetDomains() (o []*DescribeDomainResponse) {
	if v != nil && v.Domains != nil {
		return v.Domains
	}
	return
}

// GetNextPageToken is an internal getter (TBD...)
func (v *ListDomainsResponse) GetNextPageToken() (o []byte) {
	if v != nil && v.NextPageToken != nil {
		return v.NextPageToken
	}
	return
}

// ListOpenWorkflowExecutionsRequest is an internal type (TBD...)
type ListOpenWorkflowExecutionsRequest struct {
	Domain          string                   `json:"domain,omitempty"`
	MaximumPageSize int32                    `json:"maximumPageSize,omitempty"`
	NextPageToken   []byte                   `json:"nextPageToken,omitempty"`
	StartTimeFilter *StartTimeFilter         `json:"StartTimeFilter,omitempty"`
	ExecutionFilter *WorkflowExecutionFilter `json:"executionFilter,omitempty"`
	TypeFilter      *WorkflowTypeFilter      `json:"typeFilter,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *ListOpenWorkflowExecutionsRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetMaximumPageSize is an internal getter (TBD...)
func (v *ListOpenWorkflowExecutionsRequest) GetMaximumPageSize() (o int32) {
	if v != nil {
		return v.MaximumPageSize
	}
	return
}

// ListOpenWorkflowExecutionsResponse is an internal type (TBD...)
type ListOpenWorkflowExecutionsResponse struct {
	Executions    []*WorkflowExecutionInfo `json:"executions,omitempty"`
	NextPageToken []byte                   `json:"nextPageToken,omitempty"`
}

// GetExecutions is an internal getter (TBD...)
func (v *ListOpenWorkflowExecutionsResponse) GetExecutions() (o []*WorkflowExecutionInfo) {
	if v != nil && v.Executions != nil {
		return v.Executions
	}
	return
}

// ListTaskListPartitionsRequest is an internal type (TBD...)
type ListTaskListPartitionsRequest struct {
	Domain   string    `json:"domain,omitempty"`
	TaskList *TaskList `json:"taskList,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *ListTaskListPartitionsRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetTaskList is an internal getter (TBD...)
func (v *ListTaskListPartitionsRequest) GetTaskList() (o *TaskList) {
	if v != nil && v.TaskList != nil {
		return v.TaskList
	}
	return
}

// ListTaskListPartitionsResponse is an internal type (TBD...)
type ListTaskListPartitionsResponse struct {
	ActivityTaskListPartitions []*TaskListPartitionMetadata `json:"activityTaskListPartitions,omitempty"`
	DecisionTaskListPartitions []*TaskListPartitionMetadata `json:"decisionTaskListPartitions,omitempty"`
}

// GetTaskListsByDomainRequest is an internal type (TBD...)
type GetTaskListsByDomainRequest struct {
	Domain string `json:"domain,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *GetTaskListsByDomainRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetTaskListsByDomainResponse is an internal type (TBD...)
type GetTaskListsByDomainResponse struct {
	DecisionTaskListMap map[string]*DescribeTaskListResponse `json:"decisionTaskListMap,omitempty"`
	ActivityTaskListMap map[string]*DescribeTaskListResponse `json:"activityTaskListMap,omitempty"`
}

// GetDecisionTaskListMap is an internal getter (TBD...)
func (v *GetTaskListsByDomainResponse) GetDecisionTaskListMap() (o map[string]*DescribeTaskListResponse) {
	if v != nil && v.DecisionTaskListMap != nil {
		return v.DecisionTaskListMap
	}
	return
}

// GetActivityTaskListMap is an internal getter (TBD...)
func (v *GetTaskListsByDomainResponse) GetActivityTaskListMap() (o map[string]*DescribeTaskListResponse) {
	if v != nil && v.ActivityTaskListMap != nil {
		return v.ActivityTaskListMap
	}
	return
}

// ListWorkflowExecutionsRequest is an internal type (TBD...)
type ListWorkflowExecutionsRequest struct {
	Domain        string `json:"domain,omitempty"`
	PageSize      int32  `json:"pageSize,omitempty"`
	NextPageToken []byte `json:"nextPageToken,omitempty"`
	Query         string `json:"query,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *ListWorkflowExecutionsRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetPageSize is an internal getter (TBD...)
func (v *ListWorkflowExecutionsRequest) GetPageSize() (o int32) {
	if v != nil {
		return v.PageSize
	}
	return
}

// GetQuery is an internal getter (TBD...)
func (v *ListWorkflowExecutionsRequest) GetQuery() (o string) {
	if v != nil {
		return v.Query
	}
	return
}

// ListWorkflowExecutionsResponse is an internal type (TBD...)
type ListWorkflowExecutionsResponse struct {
	Executions    []*WorkflowExecutionInfo `json:"executions,omitempty"`
	NextPageToken []byte                   `json:"nextPageToken,omitempty"`
}

// GetExecutions is an internal getter (TBD...)
func (v *ListWorkflowExecutionsResponse) GetExecutions() (o []*WorkflowExecutionInfo) {
	if v != nil && v.Executions != nil {
		return v.Executions
	}
	return
}

// GetNextPageToken is an internal getter (TBD...)
func (v *ListWorkflowExecutionsResponse) GetNextPageToken() (o []byte) {
	if v != nil && v.NextPageToken != nil {
		return v.NextPageToken
	}
	return
}

// MarkerRecordedEventAttributes is an internal type (TBD...)
type MarkerRecordedEventAttributes struct {
	MarkerName                   string  `json:"markerName,omitempty"`
	Details                      []byte  `json:"details,omitempty"`
	DecisionTaskCompletedEventID int64   `json:"decisionTaskCompletedEventId,omitempty"`
	Header                       *Header `json:"header,omitempty"`
}

// GetMarkerName is an internal getter (TBD...)
func (v *MarkerRecordedEventAttributes) GetMarkerName() (o string) {
	if v != nil {
		return v.MarkerName
	}
	return
}

// Memo is an internal type (TBD...)
type Memo struct {
	Fields map[string][]byte `json:"fields,omitempty"`
}

// GetFields is an internal getter (TBD...)
func (v *Memo) GetFields() (o map[string][]byte) {
	if v != nil && v.Fields != nil {
		return v.Fields
	}
	return
}

// ParentClosePolicy is an internal type (TBD...)
type ParentClosePolicy int32

// Ptr is a helper function for getting pointer value
func (e ParentClosePolicy) Ptr() *ParentClosePolicy {
	return &e
}

// String returns a readable string representation of ParentClosePolicy.
func (e ParentClosePolicy) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "ABANDON"
	case 1:
		return "REQUEST_CANCEL"
	case 2:
		return "TERMINATE"
	}
	return fmt.Sprintf("ParentClosePolicy(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *ParentClosePolicy) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "ABANDON":
		*e = ParentClosePolicyAbandon
		return nil
	case "REQUEST_CANCEL":
		*e = ParentClosePolicyRequestCancel
		return nil
	case "TERMINATE":
		*e = ParentClosePolicyTerminate
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "ParentClosePolicy", err)
		}
		*e = ParentClosePolicy(val)
		return nil
	}
}

// MarshalText encodes ParentClosePolicy to text.
func (e ParentClosePolicy) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// ParentClosePolicyAbandon is an option for ParentClosePolicy
	ParentClosePolicyAbandon ParentClosePolicy = iota
	// ParentClosePolicyRequestCancel is an option for ParentClosePolicy
	ParentClosePolicyRequestCancel
	// ParentClosePolicyTerminate is an option for ParentClosePolicy
	ParentClosePolicyTerminate
)

// PendingActivityInfo is an internal type (TBD...)
type PendingActivityInfo struct {
	ActivityID             string                `json:"activityID,omitempty"`
	ActivityType           *ActivityType         `json:"activityType,omitempty"`
	State                  *PendingActivityState `json:"state,omitempty"`
	HeartbeatDetails       []byte                `json:"heartbeatDetails,omitempty"`
	LastHeartbeatTimestamp *int64                `json:"lastHeartbeatTimestamp,omitempty"`
	LastStartedTimestamp   *int64                `json:"lastStartedTimestamp,omitempty"`
	Attempt                int32                 `json:"attempt,omitempty"`
	MaximumAttempts        int32                 `json:"maximumAttempts,omitempty"`
	ScheduledTimestamp     *int64                `json:"scheduledTimestamp,omitempty"`
	ExpirationTimestamp    *int64                `json:"expirationTimestamp,omitempty"`
	LastFailureReason      *string               `json:"lastFailureReason,omitempty"`
	StartedWorkerIdentity  string                `json:"startedWorkerIdentity,omitempty"`
	LastWorkerIdentity     string                `json:"lastWorkerIdentity,omitempty"`
	LastFailureDetails     []byte                `json:"lastFailureDetails,omitempty"`
	ScheduleID             int64                 `json:"scheduleID,omitempty"`
}

// GetActivityID is an internal getter (TBD...)
func (v *PendingActivityInfo) GetActivityID() (o string) {
	if v != nil {
		return v.ActivityID
	}
	return
}

// GetState is an internal getter (TBD...)
func (v *PendingActivityInfo) GetState() (o PendingActivityState) {
	if v != nil && v.State != nil {
		return *v.State
	}
	return
}

// GetHeartbeatDetails is an internal getter (TBD...)
func (v *PendingActivityInfo) GetHeartbeatDetails() (o []byte) {
	if v != nil && v.HeartbeatDetails != nil {
		return v.HeartbeatDetails
	}
	return
}

// GetLastHeartbeatTimestamp is an internal getter (TBD...)
func (v *PendingActivityInfo) GetLastHeartbeatTimestamp() (o int64) {
	if v != nil && v.LastHeartbeatTimestamp != nil {
		return *v.LastHeartbeatTimestamp
	}
	return
}

// GetAttempt is an internal getter (TBD...)
func (v *PendingActivityInfo) GetAttempt() (o int32) {
	if v != nil {
		return v.Attempt
	}
	return
}

// GetMaximumAttempts is an internal getter (TBD...)
func (v *PendingActivityInfo) GetMaximumAttempts() (o int32) {
	if v != nil {
		return v.MaximumAttempts
	}
	return
}

// GetLastFailureReason is an internal getter (TBD...)
func (v *PendingActivityInfo) GetLastFailureReason() (o string) {
	if v != nil && v.LastFailureReason != nil {
		return *v.LastFailureReason
	}
	return
}

// GetStartedWorkerIdentity is an internal getter (TBD...)
func (v *PendingActivityInfo) GetStartedWorkerIdentity() (o string) {
	if v != nil {
		return v.StartedWorkerIdentity
	}
	return
}

// GetLastWorkerIdentity is an internal getter (TBD...)
func (v *PendingActivityInfo) GetLastWorkerIdentity() (o string) {
	if v != nil {
		return v.LastWorkerIdentity
	}
	return
}

// GetLastFailureDetails is an internal getter (TBD...)
func (v *PendingActivityInfo) GetLastFailureDetails() (o []byte) {
	if v != nil && v.LastFailureDetails != nil {
		return v.LastFailureDetails
	}
	return
}

// GetScheduleID is an internal getter (TBD...)
func (v *PendingActivityInfo) GetScheduleID() (o int64) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

// PendingActivityState is an internal type (TBD...)
type PendingActivityState int32

// Ptr is a helper function for getting pointer value
func (e PendingActivityState) Ptr() *PendingActivityState {
	return &e
}

// String returns a readable string representation of PendingActivityState.
func (e PendingActivityState) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "SCHEDULED"
	case 1:
		return "STARTED"
	case 2:
		return "CANCEL_REQUESTED"
	}
	return fmt.Sprintf("PendingActivityState(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *PendingActivityState) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "SCHEDULED":
		*e = PendingActivityStateScheduled
		return nil
	case "STARTED":
		*e = PendingActivityStateStarted
		return nil
	case "CANCEL_REQUESTED":
		*e = PendingActivityStateCancelRequested
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "PendingActivityState", err)
		}
		*e = PendingActivityState(val)
		return nil
	}
}

// MarshalText encodes PendingActivityState to text.
func (e PendingActivityState) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// PendingActivityStateScheduled is an option for PendingActivityState
	PendingActivityStateScheduled PendingActivityState = iota
	// PendingActivityStateStarted is an option for PendingActivityState
	PendingActivityStateStarted
	// PendingActivityStateCancelRequested is an option for PendingActivityState
	PendingActivityStateCancelRequested
)

// PendingChildExecutionInfo is an internal type (TBD...)
type PendingChildExecutionInfo struct {
	Domain            string             `json:"domain,omitempty"`
	WorkflowID        string             `json:"workflowID,omitempty"`
	RunID             string             `json:"runID,omitempty"`
	WorkflowTypeName  string             `json:"workflowTypeName,omitempty"`
	InitiatedID       int64              `json:"initiatedID,omitempty"`
	ParentClosePolicy *ParentClosePolicy `json:"parentClosePolicy,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *PendingChildExecutionInfo) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *PendingChildExecutionInfo) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *PendingChildExecutionInfo) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// GetWorkflowTypeName is an internal getter (TBD...)
func (v *PendingChildExecutionInfo) GetWorkflowTypeName() (o string) {
	if v != nil {
		return v.WorkflowTypeName
	}
	return
}

// PendingDecisionInfo is an internal type (TBD...)
type PendingDecisionInfo struct {
	State                      *PendingDecisionState `json:"state,omitempty"`
	ScheduledTimestamp         *int64                `json:"scheduledTimestamp,omitempty"`
	StartedTimestamp           *int64                `json:"startedTimestamp,omitempty"`
	Attempt                    int64                 `json:"attempt,omitempty"`
	OriginalScheduledTimestamp *int64                `json:"originalScheduledTimestamp,omitempty"`
	ScheduleID                 int64                 `json:"scheduleID,omitempty"`
}

// PendingDecisionState is an internal type (TBD...)
type PendingDecisionState int32

// Ptr is a helper function for getting pointer value
func (e PendingDecisionState) Ptr() *PendingDecisionState {
	return &e
}

// String returns a readable string representation of PendingDecisionState.
func (e PendingDecisionState) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "SCHEDULED"
	case 1:
		return "STARTED"
	}
	return fmt.Sprintf("PendingDecisionState(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *PendingDecisionState) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "SCHEDULED":
		*e = PendingDecisionStateScheduled
		return nil
	case "STARTED":
		*e = PendingDecisionStateStarted
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "PendingDecisionState", err)
		}
		*e = PendingDecisionState(val)
		return nil
	}
}

// MarshalText encodes PendingDecisionState to text.
func (e PendingDecisionState) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// PendingDecisionStateScheduled is an option for PendingDecisionState
	PendingDecisionStateScheduled PendingDecisionState = iota
	// PendingDecisionStateStarted is an option for PendingDecisionState
	PendingDecisionStateStarted
)

// PollForActivityTaskRequest is an internal type (TBD...)
type PollForActivityTaskRequest struct {
	Domain           string            `json:"domain,omitempty"`
	TaskList         *TaskList         `json:"taskList,omitempty"`
	Identity         string            `json:"identity,omitempty"`
	TaskListMetadata *TaskListMetadata `json:"taskListMetadata,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *PollForActivityTaskRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetTaskList is an internal getter (TBD...)
func (v *PollForActivityTaskRequest) GetTaskList() (o *TaskList) {
	if v != nil && v.TaskList != nil {
		return v.TaskList
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *PollForActivityTaskRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// PollForActivityTaskResponse is an internal type (TBD...)
type PollForActivityTaskResponse struct {
	TaskToken                       []byte             `json:"taskToken,omitempty"`
	WorkflowExecution               *WorkflowExecution `json:"workflowExecution,omitempty"`
	ActivityID                      string             `json:"activityId,omitempty"`
	ActivityType                    *ActivityType      `json:"activityType,omitempty"`
	Input                           []byte             `json:"input,omitempty"`
	ScheduledTimestamp              *int64             `json:"scheduledTimestamp,omitempty"`
	ScheduleToCloseTimeoutSeconds   *int32             `json:"scheduleToCloseTimeoutSeconds,omitempty"`
	StartedTimestamp                *int64             `json:"startedTimestamp,omitempty"`
	StartToCloseTimeoutSeconds      *int32             `json:"startToCloseTimeoutSeconds,omitempty"`
	HeartbeatTimeoutSeconds         *int32             `json:"heartbeatTimeoutSeconds,omitempty"`
	Attempt                         int32              `json:"attempt,omitempty"`
	ScheduledTimestampOfThisAttempt *int64             `json:"scheduledTimestampOfThisAttempt,omitempty"`
	HeartbeatDetails                []byte             `json:"heartbeatDetails,omitempty"`
	WorkflowType                    *WorkflowType      `json:"workflowType,omitempty"`
	WorkflowDomain                  string             `json:"workflowDomain,omitempty"`
	Header                          *Header            `json:"header,omitempty"`
	AutoConfigHint                  *AutoConfigHint    `json:"autoConfigHint,omitempty"`
}

// GetActivityID is an internal getter (TBD...)
func (v *PollForActivityTaskResponse) GetActivityID() (o string) {
	if v != nil {
		return v.ActivityID
	}
	return
}

// PollForDecisionTaskRequest is an internal type (TBD...)
type PollForDecisionTaskRequest struct {
	Domain         string    `json:"domain,omitempty"`
	TaskList       *TaskList `json:"taskList,omitempty"`
	Identity       string    `json:"identity,omitempty"`
	BinaryChecksum string    `json:"binaryChecksum,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *PollForDecisionTaskRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetTaskList is an internal getter (TBD...)
func (v *PollForDecisionTaskRequest) GetTaskList() (o *TaskList) {
	if v != nil && v.TaskList != nil {
		return v.TaskList
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *PollForDecisionTaskRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// GetBinaryChecksum is an internal getter (TBD...)
func (v *PollForDecisionTaskRequest) GetBinaryChecksum() (o string) {
	if v != nil {
		return v.BinaryChecksum
	}
	return
}

// PollForDecisionTaskResponse is an internal type (TBD...)
type PollForDecisionTaskResponse struct {
	TaskToken                 []byte                    `json:"taskToken,omitempty"`
	WorkflowExecution         *WorkflowExecution        `json:"workflowExecution,omitempty"`
	WorkflowType              *WorkflowType             `json:"workflowType,omitempty"`
	PreviousStartedEventID    *int64                    `json:"previousStartedEventId,omitempty"`
	StartedEventID            int64                     `json:"startedEventId,omitempty"`
	Attempt                   int64                     `json:"attempt,omitempty"`
	BacklogCountHint          int64                     `json:"backlogCountHint,omitempty"`
	History                   *History                  `json:"history,omitempty"`
	NextPageToken             []byte                    `json:"nextPageToken,omitempty"`
	Query                     *WorkflowQuery            `json:"query,omitempty"`
	WorkflowExecutionTaskList *TaskList                 `json:"WorkflowExecutionTaskList,omitempty"`
	ScheduledTimestamp        *int64                    `json:"scheduledTimestamp,omitempty"`
	StartedTimestamp          *int64                    `json:"startedTimestamp,omitempty"`
	Queries                   map[string]*WorkflowQuery `json:"queries,omitempty"`
	NextEventID               int64                     `json:"nextEventId,omitempty"`
	TotalHistoryBytes         int64                     `json:"historySize,omitempty"`
	AutoConfigHint            *AutoConfigHint           `json:"autoConfigHint,omitempty"`
}

// GetTaskToken is an internal getter (TBD...)
func (v *PollForDecisionTaskResponse) GetTaskToken() (o []byte) {
	if v != nil && v.TaskToken != nil {
		return v.TaskToken
	}
	return
}

// GetPreviousStartedEventID is an internal getter (TBD...)
func (v *PollForDecisionTaskResponse) GetPreviousStartedEventID() (o int64) {
	if v != nil && v.PreviousStartedEventID != nil {
		return *v.PreviousStartedEventID
	}
	return
}

// GetStartedEventID is an internal getter (TBD...)
func (v *PollForDecisionTaskResponse) GetStartedEventID() (o int64) {
	if v != nil {
		return v.StartedEventID
	}
	return
}

// GetAttempt is an internal getter (TBD...)
func (v *PollForDecisionTaskResponse) GetAttempt() (o int64) {
	if v != nil {
		return v.Attempt
	}
	return
}

// GetQueries is an internal getter (TBD...)
func (v *PollForDecisionTaskResponse) GetQueries() (o map[string]*WorkflowQuery) {
	if v != nil && v.Queries != nil {
		return v.Queries
	}
	return
}

// GetNextEventID is an internal getter (TBD...)
func (v *PollForDecisionTaskResponse) GetNextEventID() (o int64) {
	if v != nil {
		return v.NextEventID
	}
	return
}

func (v *PollForDecisionTaskResponse) GetHistorySize() (o int64) {
	if v != nil {
		return v.TotalHistoryBytes
	}
	return
}

// PollerInfo is an internal type (TBD...)
type PollerInfo struct {
	LastAccessTime *int64  `json:"lastAccessTime,omitempty"`
	Identity       string  `json:"identity,omitempty"`
	RatePerSecond  float64 `json:"ratePerSecond,omitempty"`
}

// GetLastAccessTime is an internal getter (TBD...)
func (v *PollerInfo) GetLastAccessTime() (o int64) {
	if v != nil && v.LastAccessTime != nil {
		return *v.LastAccessTime
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *PollerInfo) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// GetRatePerSecond is an internal getter (TBD...)
func (v *PollerInfo) GetRatePerSecond() (o float64) {
	if v != nil {
		return v.RatePerSecond
	}
	return
}

// QueryConsistencyLevel is an internal type (TBD...)
type QueryConsistencyLevel int32

// Ptr is a helper function for getting pointer value
func (e QueryConsistencyLevel) Ptr() *QueryConsistencyLevel {
	return &e
}

// String returns a readable string representation of QueryConsistencyLevel.
func (e QueryConsistencyLevel) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "EVENTUAL"
	case 1:
		return "STRONG"
	}
	return fmt.Sprintf("QueryConsistencyLevel(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *QueryConsistencyLevel) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "EVENTUAL":
		*e = QueryConsistencyLevelEventual
		return nil
	case "STRONG":
		*e = QueryConsistencyLevelStrong
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "QueryConsistencyLevel", err)
		}
		*e = QueryConsistencyLevel(val)
		return nil
	}
}

// MarshalText encodes QueryConsistencyLevel to text.
func (e QueryConsistencyLevel) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// QueryConsistencyLevelEventual is an option for QueryConsistencyLevel
	QueryConsistencyLevelEventual QueryConsistencyLevel = iota
	// QueryConsistencyLevelStrong is an option for QueryConsistencyLevel
	QueryConsistencyLevelStrong
)

// QueryFailedError is an internal type (TBD...)
type QueryFailedError struct {
	Message string `json:"message,required"`
}

// QueryRejectCondition is an internal type (TBD...)
type QueryRejectCondition int32

// Ptr is a helper function for getting pointer value
func (e QueryRejectCondition) Ptr() *QueryRejectCondition {
	return &e
}

// String returns a readable string representation of QueryRejectCondition.
func (e QueryRejectCondition) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "NOT_OPEN"
	case 1:
		return "NOT_COMPLETED_CLEANLY"
	}
	return fmt.Sprintf("QueryRejectCondition(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *QueryRejectCondition) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "NOT_OPEN":
		*e = QueryRejectConditionNotOpen
		return nil
	case "NOT_COMPLETED_CLEANLY":
		*e = QueryRejectConditionNotCompletedCleanly
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "QueryRejectCondition", err)
		}
		*e = QueryRejectCondition(val)
		return nil
	}
}

// MarshalText encodes QueryRejectCondition to text.
func (e QueryRejectCondition) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// QueryRejectConditionNotOpen is an option for QueryRejectCondition
	QueryRejectConditionNotOpen QueryRejectCondition = iota
	// QueryRejectConditionNotCompletedCleanly is an option for QueryRejectCondition
	QueryRejectConditionNotCompletedCleanly
)

// QueryRejected is an internal type (TBD...)
type QueryRejected struct {
	CloseStatus *WorkflowExecutionCloseStatus `json:"closeStatus,omitempty"`
}

// QueryResultType is an internal type (TBD...)
type QueryResultType int32

// Ptr is a helper function for getting pointer value
func (e QueryResultType) Ptr() *QueryResultType {
	return &e
}

// String returns a readable string representation of QueryResultType.
func (e QueryResultType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "ANSWERED"
	case 1:
		return "FAILED"
	}
	return fmt.Sprintf("QueryResultType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *QueryResultType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "ANSWERED":
		*e = QueryResultTypeAnswered
		return nil
	case "FAILED":
		*e = QueryResultTypeFailed
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "QueryResultType", err)
		}
		*e = QueryResultType(val)
		return nil
	}
}

// MarshalText encodes QueryResultType to text.
func (e QueryResultType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// QueryResultTypeAnswered is an option for QueryResultType
	QueryResultTypeAnswered QueryResultType = iota
	// QueryResultTypeFailed is an option for QueryResultType
	QueryResultTypeFailed
)

// QueryTaskCompletedType is an internal type (TBD...)
type QueryTaskCompletedType int32

// Ptr is a helper function for getting pointer value
func (e QueryTaskCompletedType) Ptr() *QueryTaskCompletedType {
	return &e
}

// String returns a readable string representation of QueryTaskCompletedType.
func (e QueryTaskCompletedType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "COMPLETED"
	case 1:
		return "FAILED"
	}
	return fmt.Sprintf("QueryTaskCompletedType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *QueryTaskCompletedType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "COMPLETED":
		*e = QueryTaskCompletedTypeCompleted
		return nil
	case "FAILED":
		*e = QueryTaskCompletedTypeFailed
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "QueryTaskCompletedType", err)
		}
		*e = QueryTaskCompletedType(val)
		return nil
	}
}

// MarshalText encodes QueryTaskCompletedType to text.
func (e QueryTaskCompletedType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// QueryTaskCompletedTypeCompleted is an option for QueryTaskCompletedType
	QueryTaskCompletedTypeCompleted QueryTaskCompletedType = iota
	// QueryTaskCompletedTypeFailed is an option for QueryTaskCompletedType
	QueryTaskCompletedTypeFailed
)

// QueryWorkflowRequest is an internal type (TBD...)
type QueryWorkflowRequest struct {
	Domain                string                 `json:"domain,omitempty"`
	Execution             *WorkflowExecution     `json:"execution,omitempty"`
	Query                 *WorkflowQuery         `json:"query,omitempty"`
	QueryRejectCondition  *QueryRejectCondition  `json:"queryRejectCondition,omitempty"`
	QueryConsistencyLevel *QueryConsistencyLevel `json:"queryConsistencyLevel,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *QueryWorkflowRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetExecution is an internal getter (TBD...)
func (v *QueryWorkflowRequest) GetExecution() (o *WorkflowExecution) {
	if v != nil && v.Execution != nil {
		return v.Execution
	}
	return
}

// GetQuery is an internal getter (TBD...)
func (v *QueryWorkflowRequest) GetQuery() (o *WorkflowQuery) {
	if v != nil && v.Query != nil {
		return v.Query
	}
	return
}

// GetQueryRejectCondition is an internal getter (TBD...)
func (v *QueryWorkflowRequest) GetQueryRejectCondition() (o QueryRejectCondition) {
	if v != nil && v.QueryRejectCondition != nil {
		return *v.QueryRejectCondition
	}
	return
}

// GetQueryConsistencyLevel is an internal getter (TBD...)
func (v *QueryWorkflowRequest) GetQueryConsistencyLevel() (o QueryConsistencyLevel) {
	if v != nil && v.QueryConsistencyLevel != nil {
		return *v.QueryConsistencyLevel
	}
	return
}

// QueryWorkflowResponse is an internal type (TBD...)
type QueryWorkflowResponse struct {
	QueryResult   []byte         `json:"queryResult,omitempty"`
	QueryRejected *QueryRejected `json:"queryRejected,omitempty"`
}

// GetQueryResult is an internal getter (TBD...)
func (v *QueryWorkflowResponse) GetQueryResult() (o []byte) {
	if v != nil && v.QueryResult != nil {
		return v.QueryResult
	}
	return
}

// GetQueryRejected is an internal getter (TBD...)
func (v *QueryWorkflowResponse) GetQueryRejected() (o *QueryRejected) {
	if v != nil && v.QueryRejected != nil {
		return v.QueryRejected
	}
	return
}

// ReapplyEventsRequest is an internal type (TBD...)
type ReapplyEventsRequest struct {
	DomainName        string             `json:"domainName,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	Events            *DataBlob          `json:"events,omitempty"`
}

// GetDomainName is an internal getter (TBD...)
func (v *ReapplyEventsRequest) GetDomainName() (o string) {
	if v != nil {
		return v.DomainName
	}
	return
}

// GetWorkflowExecution is an internal getter (TBD...)
func (v *ReapplyEventsRequest) GetWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.WorkflowExecution != nil {
		return v.WorkflowExecution
	}
	return
}

// GetEvents is an internal getter (TBD...)
func (v *ReapplyEventsRequest) GetEvents() (o *DataBlob) {
	if v != nil && v.Events != nil {
		return v.Events
	}
	return
}

// RecordActivityTaskHeartbeatByIDRequest is an internal type (TBD...)
type RecordActivityTaskHeartbeatByIDRequest struct {
	Domain     string `json:"domain,omitempty"`
	WorkflowID string `json:"workflowID,omitempty"`
	RunID      string `json:"runID,omitempty"`
	ActivityID string `json:"activityID,omitempty"`
	Details    []byte `json:"details,omitempty"`
	Identity   string `json:"identity,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *RecordActivityTaskHeartbeatByIDRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *RecordActivityTaskHeartbeatByIDRequest) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *RecordActivityTaskHeartbeatByIDRequest) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// GetActivityID is an internal getter (TBD...)
func (v *RecordActivityTaskHeartbeatByIDRequest) GetActivityID() (o string) {
	if v != nil {
		return v.ActivityID
	}
	return
}

// RecordActivityTaskHeartbeatRequest is an internal type (TBD...)
type RecordActivityTaskHeartbeatRequest struct {
	TaskToken []byte `json:"taskToken,omitempty"`
	Details   []byte `json:"details,omitempty"`
	Identity  string `json:"identity,omitempty"`
}

// RecordActivityTaskHeartbeatResponse is an internal type (TBD...)
type RecordActivityTaskHeartbeatResponse struct {
	CancelRequested bool `json:"cancelRequested,omitempty"`
}

// RecordMarkerDecisionAttributes is an internal type (TBD...)
type RecordMarkerDecisionAttributes struct {
	MarkerName string  `json:"markerName,omitempty"`
	Details    []byte  `json:"details,omitempty"`
	Header     *Header `json:"header,omitempty"`
}

// GetMarkerName is an internal getter (TBD...)
func (v *RecordMarkerDecisionAttributes) GetMarkerName() (o string) {
	if v != nil {
		return v.MarkerName
	}
	return
}

// RefreshWorkflowTasksRequest is an internal type (TBD...)
type RefreshWorkflowTasksRequest struct {
	Domain    string             `json:"domain,omitempty"`
	Execution *WorkflowExecution `json:"execution,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *RefreshWorkflowTasksRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetExecution is an internal getter (TBD...)
func (v *RefreshWorkflowTasksRequest) GetExecution() (o *WorkflowExecution) {
	if v != nil && v.Execution != nil {
		return v.Execution
	}
	return
}

// RegisterDomainRequest is an internal type (TBD...)
type RegisterDomainRequest struct {
	Name                                   string                             `json:"name,omitempty"`
	Description                            string                             `json:"description,omitempty"`
	OwnerEmail                             string                             `json:"ownerEmail,omitempty"`
	WorkflowExecutionRetentionPeriodInDays int32                              `json:"workflowExecutionRetentionPeriodInDays,omitempty"`
	EmitMetric                             *bool                              `json:"emitMetric,omitempty"`
	Clusters                               []*ClusterReplicationConfiguration `json:"clusters,omitempty"`
	ActiveClusterName                      string                             `json:"activeClusterName,omitempty"`
	Data                                   map[string]string                  `json:"data,omitempty"`
	SecurityToken                          string                             `json:"securityToken,omitempty"`
	IsGlobalDomain                         bool                               `json:"isGlobalDomain,omitempty"`
	HistoryArchivalStatus                  *ArchivalStatus                    `json:"historyArchivalStatus,omitempty"`
	HistoryArchivalURI                     string                             `json:"historyArchivalURI,omitempty"`
	VisibilityArchivalStatus               *ArchivalStatus                    `json:"visibilityArchivalStatus,omitempty"`
	VisibilityArchivalURI                  string                             `json:"visibilityArchivalURI,omitempty"`
}

// GetName is an internal getter (TBD...)
func (v *RegisterDomainRequest) GetName() (o string) {
	if v != nil {
		return v.Name
	}
	return
}

// GetDescription is an internal getter (TBD...)
func (v *RegisterDomainRequest) GetDescription() (o string) {
	if v != nil {
		return v.Description
	}
	return
}

// GetOwnerEmail is an internal getter (TBD...)
func (v *RegisterDomainRequest) GetOwnerEmail() (o string) {
	if v != nil {
		return v.OwnerEmail
	}
	return
}

// GetWorkflowExecutionRetentionPeriodInDays is an internal getter (TBD...)
func (v *RegisterDomainRequest) GetWorkflowExecutionRetentionPeriodInDays() (o int32) {
	if v != nil {
		return v.WorkflowExecutionRetentionPeriodInDays
	}
	return
}

// GetEmitMetric is an internal getter (TBD...)
func (v *RegisterDomainRequest) GetEmitMetric() (o bool) {
	if v != nil && v.EmitMetric != nil {
		return *v.EmitMetric
	}
	o = true
	return
}

// GetActiveClusterName is an internal getter (TBD...)
func (v *RegisterDomainRequest) GetActiveClusterName() (o string) {
	if v != nil {
		return v.ActiveClusterName
	}
	return
}

// GetData is an internal getter (TBD...)
func (v *RegisterDomainRequest) GetData() (o map[string]string) {
	if v != nil && v.Data != nil {
		return v.Data
	}
	return
}

// GetIsGlobalDomain is an internal getter (TBD...)
func (v *RegisterDomainRequest) GetIsGlobalDomain() (o bool) {
	if v != nil {
		return v.IsGlobalDomain
	}
	return
}

// GetHistoryArchivalURI is an internal getter (TBD...)
func (v *RegisterDomainRequest) GetHistoryArchivalURI() (o string) {
	if v != nil {
		return v.HistoryArchivalURI
	}
	return
}

// GetVisibilityArchivalURI is an internal getter (TBD...)
func (v *RegisterDomainRequest) GetVisibilityArchivalURI() (o string) {
	if v != nil {
		return v.VisibilityArchivalURI
	}
	return
}

// RemoteSyncMatchedError is an internal type (TBD...)
type RemoteSyncMatchedError struct {
	Message string `json:"message,required"`
}

// RemoveTaskRequest is an internal type (TBD...)
type RemoveTaskRequest struct {
	ShardID             int32  `json:"shardID,omitempty"`
	Type                *int32 `json:"type,omitempty"`
	TaskID              int64  `json:"taskID,omitempty"`
	VisibilityTimestamp *int64 `json:"visibilityTimestamp,omitempty"`
	ClusterName         string `json:"clusterName,omitempty"`
}

// GetShardID is an internal getter (TBD...)
func (v *RemoveTaskRequest) GetShardID() (o int32) {
	if v != nil {
		return v.ShardID
	}
	return
}

// GetType is an internal getter (TBD...)
func (v *RemoveTaskRequest) GetType() (o int32) {
	if v != nil && v.Type != nil {
		return *v.Type
	}
	return
}

// GetTaskID is an internal getter (TBD...)
func (v *RemoveTaskRequest) GetTaskID() (o int64) {
	if v != nil {
		return v.TaskID
	}
	return
}

// GetVisibilityTimestamp is an internal getter (TBD...)
func (v *RemoveTaskRequest) GetVisibilityTimestamp() (o int64) {
	if v != nil && v.VisibilityTimestamp != nil {
		return *v.VisibilityTimestamp
	}
	return
}

// GetClusterName is an internal getter (TBD...)
func (v *RemoveTaskRequest) GetClusterName() (o string) {
	if v != nil {
		return v.ClusterName
	}
	return
}

// RequestCancelActivityTaskDecisionAttributes is an internal type (TBD...)
type RequestCancelActivityTaskDecisionAttributes struct {
	ActivityID string `json:"activityId,omitempty"`
}

// GetActivityID is an internal getter (TBD...)
func (v *RequestCancelActivityTaskDecisionAttributes) GetActivityID() (o string) {
	if v != nil {
		return v.ActivityID
	}
	return
}

// RequestCancelActivityTaskFailedEventAttributes is an internal type (TBD...)
type RequestCancelActivityTaskFailedEventAttributes struct {
	ActivityID                   string `json:"activityId,omitempty"`
	Cause                        string `json:"cause,omitempty"`
	DecisionTaskCompletedEventID int64  `json:"decisionTaskCompletedEventId,omitempty"`
}

// RequestCancelExternalWorkflowExecutionDecisionAttributes is an internal type (TBD...)
type RequestCancelExternalWorkflowExecutionDecisionAttributes struct {
	Domain            string `json:"domain,omitempty"`
	WorkflowID        string `json:"workflowId,omitempty"`
	RunID             string `json:"runId,omitempty"`
	Control           []byte `json:"control,omitempty"`
	ChildWorkflowOnly bool   `json:"childWorkflowOnly,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *RequestCancelExternalWorkflowExecutionDecisionAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *RequestCancelExternalWorkflowExecutionDecisionAttributes) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *RequestCancelExternalWorkflowExecutionDecisionAttributes) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// RequestCancelExternalWorkflowExecutionFailedEventAttributes is an internal type (TBD...)
type RequestCancelExternalWorkflowExecutionFailedEventAttributes struct {
	Cause                        *CancelExternalWorkflowExecutionFailedCause `json:"cause,omitempty"`
	DecisionTaskCompletedEventID int64                                       `json:"decisionTaskCompletedEventId,omitempty"`
	Domain                       string                                      `json:"domain,omitempty"`
	WorkflowExecution            *WorkflowExecution                          `json:"workflowExecution,omitempty"`
	InitiatedEventID             int64                                       `json:"initiatedEventId,omitempty"`
	Control                      []byte                                      `json:"control,omitempty"`
}

// GetDecisionTaskCompletedEventID is an internal getter (TBD...)
func (v *RequestCancelExternalWorkflowExecutionFailedEventAttributes) GetDecisionTaskCompletedEventID() (o int64) {
	if v != nil {
		return v.DecisionTaskCompletedEventID
	}
	return
}

// GetDomain is an internal getter (TBD...)
func (v *RequestCancelExternalWorkflowExecutionFailedEventAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *RequestCancelExternalWorkflowExecutionFailedEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// RequestCancelExternalWorkflowExecutionInitiatedEventAttributes is an internal type (TBD...)
type RequestCancelExternalWorkflowExecutionInitiatedEventAttributes struct {
	DecisionTaskCompletedEventID int64              `json:"decisionTaskCompletedEventId,omitempty"`
	Domain                       string             `json:"domain,omitempty"`
	WorkflowExecution            *WorkflowExecution `json:"workflowExecution,omitempty"`
	Control                      []byte             `json:"control,omitempty"`
	ChildWorkflowOnly            bool               `json:"childWorkflowOnly,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *RequestCancelExternalWorkflowExecutionInitiatedEventAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowExecution is an internal getter (TBD...)
func (v *RequestCancelExternalWorkflowExecutionInitiatedEventAttributes) GetWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.WorkflowExecution != nil {
		return v.WorkflowExecution
	}
	return
}

// GetChildWorkflowOnly is an internal getter (TBD...)
func (v *RequestCancelExternalWorkflowExecutionInitiatedEventAttributes) GetChildWorkflowOnly() (o bool) {
	if v != nil {
		return v.ChildWorkflowOnly
	}
	return
}

// RequestCancelWorkflowExecutionRequest is an internal type (TBD...)
type RequestCancelWorkflowExecutionRequest struct {
	Domain              string             `json:"domain,omitempty"`
	WorkflowExecution   *WorkflowExecution `json:"workflowExecution,omitempty"`
	Identity            string             `json:"identity,omitempty"`
	RequestID           string             `json:"requestId,omitempty"`
	Cause               string             `json:"cause,omitempty"`
	FirstExecutionRunID string             `json:"first_execution_run_id,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *RequestCancelWorkflowExecutionRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowExecution is an internal getter (TBD...)
func (v *RequestCancelWorkflowExecutionRequest) GetWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.WorkflowExecution != nil {
		return v.WorkflowExecution
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *RequestCancelWorkflowExecutionRequest) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// GetFirstExecutionRunID is an internal getter (TBD...)
func (v *RequestCancelWorkflowExecutionRequest) GetFirstExecutionRunID() (o string) {
	if v != nil {
		return v.FirstExecutionRunID
	}
	return
}

// ResetPointInfo is an internal type (TBD...)
type ResetPointInfo struct {
	BinaryChecksum           string `json:"binaryChecksum,omitempty"`
	RunID                    string `json:"runId,omitempty"`
	FirstDecisionCompletedID int64  `json:"firstDecisionCompletedId,omitempty"`
	CreatedTimeNano          *int64 `json:"createdTimeNano,omitempty"`
	ExpiringTimeNano         *int64 `json:"expiringTimeNano,omitempty"`
	Resettable               bool   `json:"resettable,omitempty"`
}

// GetBinaryChecksum is an internal getter (TBD...)
func (v *ResetPointInfo) GetBinaryChecksum() (o string) {
	if v != nil {
		return v.BinaryChecksum
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *ResetPointInfo) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// GetFirstDecisionCompletedID is an internal getter (TBD...)
func (v *ResetPointInfo) GetFirstDecisionCompletedID() (o int64) {
	if v != nil {
		return v.FirstDecisionCompletedID
	}
	return
}

// GetCreatedTimeNano is an internal getter (TBD...)
func (v *ResetPointInfo) GetCreatedTimeNano() (o int64) {
	if v != nil && v.CreatedTimeNano != nil {
		return *v.CreatedTimeNano
	}
	return
}

// GetExpiringTimeNano is an internal getter (TBD...)
func (v *ResetPointInfo) GetExpiringTimeNano() (o int64) {
	if v != nil && v.ExpiringTimeNano != nil {
		return *v.ExpiringTimeNano
	}
	return
}

// GetResettable is an internal getter (TBD...)
func (v *ResetPointInfo) GetResettable() (o bool) {
	if v != nil {
		return v.Resettable
	}
	return
}

// ResetPoints is an internal type (TBD...)
type ResetPoints struct {
	Points []*ResetPointInfo `json:"points,omitempty"`
}

// ResetQueueRequest is an internal type (TBD...)
type ResetQueueRequest struct {
	ShardID     int32  `json:"shardID,omitempty"`
	ClusterName string `json:"clusterName,omitempty"`
	Type        *int32 `json:"type,omitempty"`
}

// GetShardID is an internal getter (TBD...)
func (v *ResetQueueRequest) GetShardID() (o int32) {
	if v != nil {
		return v.ShardID
	}
	return
}

// GetClusterName is an internal getter (TBD...)
func (v *ResetQueueRequest) GetClusterName() (o string) {
	if v != nil {
		return v.ClusterName
	}
	return
}

// GetType is an internal getter (TBD...)
func (v *ResetQueueRequest) GetType() (o int32) {
	if v != nil && v.Type != nil {
		return *v.Type
	}
	return
}

// ResetStickyTaskListRequest is an internal type (TBD...)
type ResetStickyTaskListRequest struct {
	Domain    string             `json:"domain,omitempty"`
	Execution *WorkflowExecution `json:"execution,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *ResetStickyTaskListRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetExecution is an internal getter (TBD...)
func (v *ResetStickyTaskListRequest) GetExecution() (o *WorkflowExecution) {
	if v != nil && v.Execution != nil {
		return v.Execution
	}
	return
}

// ResetStickyTaskListResponse is an internal type (TBD...)
type ResetStickyTaskListResponse struct {
}

// ResetWorkflowExecutionRequest is an internal type (TBD...)
type ResetWorkflowExecutionRequest struct {
	Domain                string             `json:"domain,omitempty"`
	WorkflowExecution     *WorkflowExecution `json:"workflowExecution,omitempty"`
	Reason                string             `json:"reason,omitempty"`
	DecisionFinishEventID int64              `json:"decisionFinishEventId,omitempty"`
	RequestID             string             `json:"requestId,omitempty"`
	SkipSignalReapply     bool               `json:"skipSignalReapply,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *ResetWorkflowExecutionRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowExecution is an internal getter (TBD...)
func (v *ResetWorkflowExecutionRequest) GetWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.WorkflowExecution != nil {
		return v.WorkflowExecution
	}
	return
}

// GetReason is an internal getter (TBD...)
func (v *ResetWorkflowExecutionRequest) GetReason() (o string) {
	if v != nil {
		return v.Reason
	}
	return
}

// GetDecisionFinishEventID is an internal getter (TBD...)
func (v *ResetWorkflowExecutionRequest) GetDecisionFinishEventID() (o int64) {
	if v != nil {
		return v.DecisionFinishEventID
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *ResetWorkflowExecutionRequest) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// GetSkipSignalReapply is an internal getter (TBD...)
func (v *ResetWorkflowExecutionRequest) GetSkipSignalReapply() (o bool) {
	if v != nil {
		return v.SkipSignalReapply
	}
	return
}

// ResetWorkflowExecutionResponse is an internal type (TBD...)
type ResetWorkflowExecutionResponse struct {
	RunID string `json:"runId,omitempty"`
}

// GetRunID is an internal getter (TBD...)
func (v *ResetWorkflowExecutionResponse) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// RespondActivityTaskCanceledByIDRequest is an internal type (TBD...)
type RespondActivityTaskCanceledByIDRequest struct {
	Domain     string `json:"domain,omitempty"`
	WorkflowID string `json:"workflowID,omitempty"`
	RunID      string `json:"runID,omitempty"`
	ActivityID string `json:"activityID,omitempty"`
	Details    []byte `json:"details,omitempty"`
	Identity   string `json:"identity,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *RespondActivityTaskCanceledByIDRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *RespondActivityTaskCanceledByIDRequest) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *RespondActivityTaskCanceledByIDRequest) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// GetActivityID is an internal getter (TBD...)
func (v *RespondActivityTaskCanceledByIDRequest) GetActivityID() (o string) {
	if v != nil {
		return v.ActivityID
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *RespondActivityTaskCanceledByIDRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// RespondActivityTaskCanceledRequest is an internal type (TBD...)
type RespondActivityTaskCanceledRequest struct {
	TaskToken []byte `json:"taskToken,omitempty"`
	Details   []byte `json:"details,omitempty"`
	Identity  string `json:"identity,omitempty"`
}

// GetIdentity is an internal getter (TBD...)
func (v *RespondActivityTaskCanceledRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// RespondActivityTaskCompletedByIDRequest is an internal type (TBD...)
type RespondActivityTaskCompletedByIDRequest struct {
	Domain     string `json:"domain,omitempty"`
	WorkflowID string `json:"workflowID,omitempty"`
	RunID      string `json:"runID,omitempty"`
	ActivityID string `json:"activityID,omitempty"`
	Result     []byte `json:"result,omitempty"`
	Identity   string `json:"identity,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *RespondActivityTaskCompletedByIDRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *RespondActivityTaskCompletedByIDRequest) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *RespondActivityTaskCompletedByIDRequest) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// GetActivityID is an internal getter (TBD...)
func (v *RespondActivityTaskCompletedByIDRequest) GetActivityID() (o string) {
	if v != nil {
		return v.ActivityID
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *RespondActivityTaskCompletedByIDRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// RespondActivityTaskCompletedRequest is an internal type (TBD...)
type RespondActivityTaskCompletedRequest struct {
	TaskToken []byte `json:"taskToken,omitempty"`
	Result    []byte `json:"result,omitempty"`
	Identity  string `json:"identity,omitempty"`
}

// GetIdentity is an internal getter (TBD...)
func (v *RespondActivityTaskCompletedRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// RespondActivityTaskFailedByIDRequest is an internal type (TBD...)
type RespondActivityTaskFailedByIDRequest struct {
	Domain     string  `json:"domain,omitempty"`
	WorkflowID string  `json:"workflowID,omitempty"`
	RunID      string  `json:"runID,omitempty"`
	ActivityID string  `json:"activityID,omitempty"`
	Reason     *string `json:"reason,omitempty"`
	Details    []byte  `json:"details,omitempty"`
	Identity   string  `json:"identity,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *RespondActivityTaskFailedByIDRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *RespondActivityTaskFailedByIDRequest) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *RespondActivityTaskFailedByIDRequest) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// GetActivityID is an internal getter (TBD...)
func (v *RespondActivityTaskFailedByIDRequest) GetActivityID() (o string) {
	if v != nil {
		return v.ActivityID
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *RespondActivityTaskFailedByIDRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// RespondActivityTaskFailedRequest is an internal type (TBD...)
type RespondActivityTaskFailedRequest struct {
	TaskToken []byte  `json:"taskToken,omitempty"`
	Reason    *string `json:"reason,omitempty"`
	Details   []byte  `json:"details,omitempty"`
	Identity  string  `json:"identity,omitempty"`
}

// GetReason is an internal getter (TBD...)
func (v *RespondActivityTaskFailedRequest) GetReason() (o string) {
	if v != nil && v.Reason != nil {
		return *v.Reason
	}
	return
}

// GetDetails is an internal getter (TBD...)
func (v *RespondActivityTaskFailedRequest) GetDetails() (o []byte) {
	if v != nil && v.Details != nil {
		return v.Details
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *RespondActivityTaskFailedRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// RespondDecisionTaskCompletedRequest is an internal type (TBD...)
type RespondDecisionTaskCompletedRequest struct {
	TaskToken                  []byte                          `json:"taskToken,omitempty"`
	Decisions                  []*Decision                     `json:"decisions,omitempty"`
	ExecutionContext           []byte                          `json:"executionContext,omitempty"`
	Identity                   string                          `json:"identity,omitempty"`
	StickyAttributes           *StickyExecutionAttributes      `json:"stickyAttributes,omitempty"`
	ReturnNewDecisionTask      bool                            `json:"returnNewDecisionTask,omitempty"`
	ForceCreateNewDecisionTask bool                            `json:"forceCreateNewDecisionTask,omitempty"`
	BinaryChecksum             string                          `json:"binaryChecksum,omitempty"`
	QueryResults               map[string]*WorkflowQueryResult `json:"queryResults,omitempty"`
}

// GetIdentity is an internal getter (TBD...)
func (v *RespondDecisionTaskCompletedRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// GetReturnNewDecisionTask is an internal getter (TBD...)
func (v *RespondDecisionTaskCompletedRequest) GetReturnNewDecisionTask() (o bool) {
	if v != nil {
		return v.ReturnNewDecisionTask
	}
	return
}

// GetForceCreateNewDecisionTask is an internal getter (TBD...)
func (v *RespondDecisionTaskCompletedRequest) GetForceCreateNewDecisionTask() (o bool) {
	if v != nil {
		return v.ForceCreateNewDecisionTask
	}
	return
}

// GetBinaryChecksum is an internal getter (TBD...)
func (v *RespondDecisionTaskCompletedRequest) GetBinaryChecksum() (o string) {
	if v != nil {
		return v.BinaryChecksum
	}
	return
}

// GetQueryResults is an internal getter (TBD...)
func (v *RespondDecisionTaskCompletedRequest) GetQueryResults() (o map[string]*WorkflowQueryResult) {
	if v != nil && v.QueryResults != nil {
		return v.QueryResults
	}
	return
}

// RespondDecisionTaskCompletedResponse is an internal type (TBD...)
type RespondDecisionTaskCompletedResponse struct {
	DecisionTask                *PollForDecisionTaskResponse          `json:"decisionTask,omitempty"`
	ActivitiesToDispatchLocally map[string]*ActivityLocalDispatchInfo `json:"activitiesToDispatchLocally,omitempty"`
}

// GetDecisionTask is an internal getter (TBD...)
func (v *RespondDecisionTaskCompletedResponse) GetDecisionTask() (o *PollForDecisionTaskResponse) {
	if v != nil && v.DecisionTask != nil {
		return v.DecisionTask
	}
	return
}

// RespondDecisionTaskFailedRequest is an internal type (TBD...)
type RespondDecisionTaskFailedRequest struct {
	TaskToken      []byte                   `json:"taskToken,omitempty"`
	Cause          *DecisionTaskFailedCause `json:"cause,omitempty"`
	Details        []byte                   `json:"details,omitempty"`
	Identity       string                   `json:"identity,omitempty"`
	BinaryChecksum string                   `json:"binaryChecksum,omitempty"`
}

// GetCause is an internal getter (TBD...)
func (v *RespondDecisionTaskFailedRequest) GetCause() (o DecisionTaskFailedCause) {
	if v != nil && v.Cause != nil {
		return *v.Cause
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *RespondDecisionTaskFailedRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// GetBinaryChecksum is an internal getter (TBD...)
func (v *RespondDecisionTaskFailedRequest) GetBinaryChecksum() (o string) {
	if v != nil {
		return v.BinaryChecksum
	}
	return
}

// RespondQueryTaskCompletedRequest is an internal type (TBD...)
type RespondQueryTaskCompletedRequest struct {
	TaskToken         []byte                  `json:"taskToken,omitempty"`
	CompletedType     *QueryTaskCompletedType `json:"completedType,omitempty"`
	QueryResult       []byte                  `json:"queryResult,omitempty"`
	ErrorMessage      string                  `json:"errorMessage,omitempty"`
	WorkerVersionInfo *WorkerVersionInfo      `json:"workerVersionInfo,omitempty"`
}

// GetCompletedType is an internal getter (TBD...)
func (v *RespondQueryTaskCompletedRequest) GetCompletedType() (o QueryTaskCompletedType) {
	if v != nil && v.CompletedType != nil {
		return *v.CompletedType
	}
	return
}

// GetQueryResult is an internal getter (TBD...)
func (v *RespondQueryTaskCompletedRequest) GetQueryResult() (o []byte) {
	if v != nil && v.QueryResult != nil {
		return v.QueryResult
	}
	return
}

// GetErrorMessage is an internal getter (TBD...)
func (v *RespondQueryTaskCompletedRequest) GetErrorMessage() (o string) {
	if v != nil {
		return v.ErrorMessage
	}
	return
}

// GetWorkerVersionInfo is an internal getter (TBD...)
func (v *RespondQueryTaskCompletedRequest) GetWorkerVersionInfo() (o *WorkerVersionInfo) {
	if v != nil && v.WorkerVersionInfo != nil {
		return v.WorkerVersionInfo
	}
	return
}

// RetryPolicy is an internal type (TBD...)
type RetryPolicy struct {
	InitialIntervalInSeconds    int32    `json:"initialIntervalInSeconds,omitempty"`
	BackoffCoefficient          float64  `json:"backoffCoefficient,omitempty"`
	MaximumIntervalInSeconds    int32    `json:"maximumIntervalInSeconds,omitempty"`
	MaximumAttempts             int32    `json:"maximumAttempts,omitempty"`
	NonRetriableErrorReasons    []string `json:"nonRetriableErrorReasons,omitempty"`
	ExpirationIntervalInSeconds int32    `json:"expirationIntervalInSeconds,omitempty"`
}

// GetInitialIntervalInSeconds is an internal getter (TBD...)
func (v *RetryPolicy) GetInitialIntervalInSeconds() (o int32) {
	if v != nil {
		return v.InitialIntervalInSeconds
	}
	return
}

// GetBackoffCoefficient is an internal getter (TBD...)
func (v *RetryPolicy) GetBackoffCoefficient() (o float64) {
	if v != nil {
		return v.BackoffCoefficient
	}
	return
}

// GetMaximumIntervalInSeconds is an internal getter (TBD...)
func (v *RetryPolicy) GetMaximumIntervalInSeconds() (o int32) {
	if v != nil {
		return v.MaximumIntervalInSeconds
	}
	return
}

// GetMaximumAttempts is an internal getter (TBD...)
func (v *RetryPolicy) GetMaximumAttempts() (o int32) {
	if v != nil {
		return v.MaximumAttempts
	}
	return
}

// GetNonRetriableErrorReasons is an internal getter (TBD...)
func (v *RetryPolicy) GetNonRetriableErrorReasons() (o []string) {
	if v != nil && v.NonRetriableErrorReasons != nil {
		return v.NonRetriableErrorReasons
	}
	return
}

// GetExpirationIntervalInSeconds is an internal getter (TBD...)
func (v *RetryPolicy) GetExpirationIntervalInSeconds() (o int32) {
	if v != nil {
		return v.ExpirationIntervalInSeconds
	}
	return
}

// RetryTaskV2Error is an internal type (TBD...)
type RetryTaskV2Error struct {
	Message           string `json:"message,required"`
	DomainID          string `json:"domainId,omitempty"`
	WorkflowID        string `json:"workflowId,omitempty"`
	RunID             string `json:"runId,omitempty"`
	StartEventID      *int64 `json:"startEventId,omitempty"`
	StartEventVersion *int64 `json:"startEventVersion,omitempty"`
	EndEventID        *int64 `json:"endEventId,omitempty"`
	EndEventVersion   *int64 `json:"endEventVersion,omitempty"`
}

// GetDomainID is an internal getter (TBD...)
func (v *RetryTaskV2Error) GetDomainID() (o string) {
	if v != nil {
		return v.DomainID
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *RetryTaskV2Error) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *RetryTaskV2Error) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// ScheduleActivityTaskDecisionAttributes is an internal type (TBD...)
type ScheduleActivityTaskDecisionAttributes struct {
	ActivityID                    string        `json:"activityId,omitempty"`
	ActivityType                  *ActivityType `json:"activityType,omitempty"`
	Domain                        string        `json:"domain,omitempty"`
	TaskList                      *TaskList     `json:"taskList,omitempty"`
	Input                         []byte        `json:"input,omitempty"`
	ScheduleToCloseTimeoutSeconds *int32        `json:"scheduleToCloseTimeoutSeconds,omitempty"`
	ScheduleToStartTimeoutSeconds *int32        `json:"scheduleToStartTimeoutSeconds,omitempty"`
	StartToCloseTimeoutSeconds    *int32        `json:"startToCloseTimeoutSeconds,omitempty"`
	HeartbeatTimeoutSeconds       *int32        `json:"heartbeatTimeoutSeconds,omitempty"`
	RetryPolicy                   *RetryPolicy  `json:"retryPolicy,omitempty"`
	Header                        *Header       `json:"header,omitempty"`
	RequestLocalDispatch          bool          `json:"requestLocalDispatch,omitempty"`
}

// GetActivityID is an internal getter (TBD...)
func (v *ScheduleActivityTaskDecisionAttributes) GetActivityID() (o string) {
	if v != nil {
		return v.ActivityID
	}
	return
}

// GetActivityType is an internal getter (TBD...)
func (v *ScheduleActivityTaskDecisionAttributes) GetActivityType() (o *ActivityType) {
	if v != nil && v.ActivityType != nil {
		return v.ActivityType
	}
	return
}

// GetDomain is an internal getter (TBD...)
func (v *ScheduleActivityTaskDecisionAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetScheduleToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *ScheduleActivityTaskDecisionAttributes) GetScheduleToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.ScheduleToCloseTimeoutSeconds != nil {
		return *v.ScheduleToCloseTimeoutSeconds
	}
	return
}

// GetScheduleToStartTimeoutSeconds is an internal getter (TBD...)
func (v *ScheduleActivityTaskDecisionAttributes) GetScheduleToStartTimeoutSeconds() (o int32) {
	if v != nil && v.ScheduleToStartTimeoutSeconds != nil {
		return *v.ScheduleToStartTimeoutSeconds
	}
	return
}

// GetStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *ScheduleActivityTaskDecisionAttributes) GetStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.StartToCloseTimeoutSeconds != nil {
		return *v.StartToCloseTimeoutSeconds
	}
	return
}

// GetHeartbeatTimeoutSeconds is an internal getter (TBD...)
func (v *ScheduleActivityTaskDecisionAttributes) GetHeartbeatTimeoutSeconds() (o int32) {
	if v != nil && v.HeartbeatTimeoutSeconds != nil {
		return *v.HeartbeatTimeoutSeconds
	}
	return
}

// SearchAttributes is an internal type (TBD...)
type SearchAttributes struct {
	IndexedFields map[string][]byte `json:"indexedFields,omitempty"`
}

// GetIndexedFields is an internal getter (TBD...)
func (v *SearchAttributes) GetIndexedFields() (o map[string][]byte) {
	if v != nil && v.IndexedFields != nil {
		return v.IndexedFields
	}
	return
}

// ServiceBusyError is an internal type (TBD...)
type ServiceBusyError struct {
	Message string `json:"message,required"`
	Reason  string `json:"reason,omitempty"`
}

// SignalExternalWorkflowExecutionDecisionAttributes is an internal type (TBD...)
type SignalExternalWorkflowExecutionDecisionAttributes struct {
	Domain            string             `json:"domain,omitempty"`
	Execution         *WorkflowExecution `json:"execution,omitempty"`
	SignalName        string             `json:"signalName,omitempty"`
	Input             []byte             `json:"input,omitempty"`
	Control           []byte             `json:"control,omitempty"`
	ChildWorkflowOnly bool               `json:"childWorkflowOnly,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *SignalExternalWorkflowExecutionDecisionAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetSignalName is an internal getter (TBD...)
func (v *SignalExternalWorkflowExecutionDecisionAttributes) GetSignalName() (o string) {
	if v != nil {
		return v.SignalName
	}
	return
}

// SignalExternalWorkflowExecutionFailedCause is an internal type (TBD...)
type SignalExternalWorkflowExecutionFailedCause int32

// Ptr is a helper function for getting pointer value
func (e SignalExternalWorkflowExecutionFailedCause) Ptr() *SignalExternalWorkflowExecutionFailedCause {
	return &e
}

// String returns a readable string representation of SignalExternalWorkflowExecutionFailedCause.
func (e SignalExternalWorkflowExecutionFailedCause) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION"
	case 1:
		return "WORKFLOW_ALREADY_COMPLETED"
	}
	return fmt.Sprintf("SignalExternalWorkflowExecutionFailedCause(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *SignalExternalWorkflowExecutionFailedCause) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION":
		*e = SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution
		return nil
	case "WORKFLOW_ALREADY_COMPLETED":
		*e = SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "SignalExternalWorkflowExecutionFailedCause", err)
		}
		*e = SignalExternalWorkflowExecutionFailedCause(val)
		return nil
	}
}

// MarshalText encodes SignalExternalWorkflowExecutionFailedCause to text.
func (e SignalExternalWorkflowExecutionFailedCause) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution is an option for SignalExternalWorkflowExecutionFailedCause
	SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution SignalExternalWorkflowExecutionFailedCause = iota
	SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted
)

// SignalExternalWorkflowExecutionFailedEventAttributes is an internal type (TBD...)
type SignalExternalWorkflowExecutionFailedEventAttributes struct {
	Cause                        *SignalExternalWorkflowExecutionFailedCause `json:"cause,omitempty"`
	DecisionTaskCompletedEventID int64                                       `json:"decisionTaskCompletedEventId,omitempty"`
	Domain                       string                                      `json:"domain,omitempty"`
	WorkflowExecution            *WorkflowExecution                          `json:"workflowExecution,omitempty"`
	InitiatedEventID             int64                                       `json:"initiatedEventId,omitempty"`
	Control                      []byte                                      `json:"control,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *SignalExternalWorkflowExecutionFailedEventAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *SignalExternalWorkflowExecutionFailedEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// SignalExternalWorkflowExecutionInitiatedEventAttributes is an internal type (TBD...)
type SignalExternalWorkflowExecutionInitiatedEventAttributes struct {
	DecisionTaskCompletedEventID int64              `json:"decisionTaskCompletedEventId,omitempty"`
	Domain                       string             `json:"domain,omitempty"`
	WorkflowExecution            *WorkflowExecution `json:"workflowExecution,omitempty"`
	SignalName                   string             `json:"signalName,omitempty"`
	Input                        []byte             `json:"input,omitempty"`
	Control                      []byte             `json:"control,omitempty"`
	ChildWorkflowOnly            bool               `json:"childWorkflowOnly,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *SignalExternalWorkflowExecutionInitiatedEventAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowExecution is an internal getter (TBD...)
func (v *SignalExternalWorkflowExecutionInitiatedEventAttributes) GetWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.WorkflowExecution != nil {
		return v.WorkflowExecution
	}
	return
}

// GetSignalName is an internal getter (TBD...)
func (v *SignalExternalWorkflowExecutionInitiatedEventAttributes) GetSignalName() (o string) {
	if v != nil {
		return v.SignalName
	}
	return
}

// GetChildWorkflowOnly is an internal getter (TBD...)
func (v *SignalExternalWorkflowExecutionInitiatedEventAttributes) GetChildWorkflowOnly() (o bool) {
	if v != nil {
		return v.ChildWorkflowOnly
	}
	return
}

// SignalWithStartWorkflowExecutionRequest is an internal type (TBD...)
type SignalWithStartWorkflowExecutionRequest struct {
	Domain                              string                 `json:"domain,omitempty"`
	WorkflowID                          string                 `json:"workflowId,omitempty"`
	WorkflowType                        *WorkflowType          `json:"workflowType,omitempty"`
	TaskList                            *TaskList              `json:"taskList,omitempty"`
	Input                               []byte                 `json:"-"` // Filtering PII
	ExecutionStartToCloseTimeoutSeconds *int32                 `json:"executionStartToCloseTimeoutSeconds,omitempty"`
	TaskStartToCloseTimeoutSeconds      *int32                 `json:"taskStartToCloseTimeoutSeconds,omitempty"`
	Identity                            string                 `json:"identity,omitempty"`
	RequestID                           string                 `json:"requestId,omitempty"`
	WorkflowIDReusePolicy               *WorkflowIDReusePolicy `json:"workflowIdReusePolicy,omitempty"`
	SignalName                          string                 `json:"signalName,omitempty"`
	SignalInput                         []byte                 `json:"-"` // Filtering PII
	Control                             []byte                 `json:"control,omitempty"`
	RetryPolicy                         *RetryPolicy           `json:"retryPolicy,omitempty"`
	CronSchedule                        string                 `json:"cronSchedule,omitempty"`
	Memo                                *Memo                  `json:"-"` // Filtering PII
	SearchAttributes                    *SearchAttributes      `json:"-"` // Filtering PII
	Header                              *Header                `json:"header,omitempty"`
	DelayStartSeconds                   *int32                 `json:"delayStartSeconds,omitempty"`
	JitterStartSeconds                  *int32                 `json:"jitterStartSeconds,omitempty"`
	FirstRunAtTimestamp                 *int64                 `json:"firstRunAtTimestamp,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetWorkflowType is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetWorkflowType() (o *WorkflowType) {
	if v != nil && v.WorkflowType != nil {
		return v.WorkflowType
	}
	return
}

// GetTaskList is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetTaskList() (o *TaskList) {
	if v != nil && v.TaskList != nil {
		return v.TaskList
	}
	return
}

// GetExecutionStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetExecutionStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.ExecutionStartToCloseTimeoutSeconds != nil {
		return *v.ExecutionStartToCloseTimeoutSeconds
	}
	return
}

// GetTaskStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetTaskStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.TaskStartToCloseTimeoutSeconds != nil {
		return *v.TaskStartToCloseTimeoutSeconds
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// GetWorkflowIDReusePolicy is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetWorkflowIDReusePolicy() (o WorkflowIDReusePolicy) {
	if v != nil && v.WorkflowIDReusePolicy != nil {
		return *v.WorkflowIDReusePolicy
	}
	return
}

// GetSignalName is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetSignalName() (o string) {
	if v != nil {
		return v.SignalName
	}
	return
}

// GetSignalInput is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetSignalInput() (o []byte) {
	if v != nil && v.SignalInput != nil {
		return v.SignalInput
	}
	return
}

// GetCronSchedule is an internal getter (TBD...)
func (v *SignalWithStartWorkflowExecutionRequest) GetCronSchedule() (o string) {
	if v != nil {
		return v.CronSchedule
	}
	return
}

// SignalWithStartWorkflowExecutionAsyncRequest is an internal type (TBD...)
type SignalWithStartWorkflowExecutionAsyncRequest struct {
	*SignalWithStartWorkflowExecutionRequest
}

// SignalWithStartWorkflowExecutionAsyncResponse is an internal type (TBD...)
type SignalWithStartWorkflowExecutionAsyncResponse struct {
}

// SignalWorkflowExecutionRequest is an internal type (TBD...)
type SignalWorkflowExecutionRequest struct {
	Domain            string             `json:"domain,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	SignalName        string             `json:"signalName,omitempty"`
	Input             []byte             `json:"-"` // Filtering PII
	Identity          string             `json:"identity,omitempty"`
	RequestID         string             `json:"requestId,omitempty"`
	Control           []byte             `json:"control,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *SignalWorkflowExecutionRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowExecution is an internal getter (TBD...)
func (v *SignalWorkflowExecutionRequest) GetWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.WorkflowExecution != nil {
		return v.WorkflowExecution
	}
	return
}

// GetSignalName is an internal getter (TBD...)
func (v *SignalWorkflowExecutionRequest) GetSignalName() (o string) {
	if v != nil {
		return v.SignalName
	}
	return
}

// GetInput is an internal getter (TBD...)
func (v *SignalWorkflowExecutionRequest) GetInput() (o []byte) {
	if v != nil && v.Input != nil {
		return v.Input
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *SignalWorkflowExecutionRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *SignalWorkflowExecutionRequest) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// StartChildWorkflowExecutionDecisionAttributes is an internal type (TBD...)
type StartChildWorkflowExecutionDecisionAttributes struct {
	Domain                              string                 `json:"domain,omitempty"`
	WorkflowID                          string                 `json:"workflowId,omitempty"`
	WorkflowType                        *WorkflowType          `json:"workflowType,omitempty"`
	TaskList                            *TaskList              `json:"taskList,omitempty"`
	Input                               []byte                 `json:"input,omitempty"`
	ExecutionStartToCloseTimeoutSeconds *int32                 `json:"executionStartToCloseTimeoutSeconds,omitempty"`
	TaskStartToCloseTimeoutSeconds      *int32                 `json:"taskStartToCloseTimeoutSeconds,omitempty"`
	ParentClosePolicy                   *ParentClosePolicy     `json:"parentClosePolicy,omitempty"`
	Control                             []byte                 `json:"control,omitempty"`
	WorkflowIDReusePolicy               *WorkflowIDReusePolicy `json:"workflowIdReusePolicy,omitempty"`
	RetryPolicy                         *RetryPolicy           `json:"retryPolicy,omitempty"`
	CronSchedule                        string                 `json:"cronSchedule,omitempty"`
	Header                              *Header                `json:"header,omitempty"`
	Memo                                *Memo                  `json:"memo,omitempty"`
	SearchAttributes                    *SearchAttributes      `json:"searchAttributes,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionDecisionAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionDecisionAttributes) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetExecutionStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionDecisionAttributes) GetExecutionStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.ExecutionStartToCloseTimeoutSeconds != nil {
		return *v.ExecutionStartToCloseTimeoutSeconds
	}
	return
}

// GetTaskStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionDecisionAttributes) GetTaskStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.TaskStartToCloseTimeoutSeconds != nil {
		return *v.TaskStartToCloseTimeoutSeconds
	}
	return
}

// GetParentClosePolicy is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionDecisionAttributes) GetParentClosePolicy() (o ParentClosePolicy) {
	if v != nil && v.ParentClosePolicy != nil {
		return *v.ParentClosePolicy
	}
	return
}

// GetCronSchedule is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionDecisionAttributes) GetCronSchedule() (o string) {
	if v != nil {
		return v.CronSchedule
	}
	return
}

// StartChildWorkflowExecutionFailedEventAttributes is an internal type (TBD...)
type StartChildWorkflowExecutionFailedEventAttributes struct {
	Domain                       string                             `json:"domain,omitempty"`
	WorkflowID                   string                             `json:"workflowId,omitempty"`
	WorkflowType                 *WorkflowType                      `json:"workflowType,omitempty"`
	Cause                        *ChildWorkflowExecutionFailedCause `json:"cause,omitempty"`
	Control                      []byte                             `json:"control,omitempty"`
	InitiatedEventID             int64                              `json:"initiatedEventId,omitempty"`
	DecisionTaskCompletedEventID int64                              `json:"decisionTaskCompletedEventId,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionFailedEventAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetInitiatedEventID is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionFailedEventAttributes) GetInitiatedEventID() (o int64) {
	if v != nil {
		return v.InitiatedEventID
	}
	return
}

// StartChildWorkflowExecutionInitiatedEventAttributes is an internal type (TBD...)
type StartChildWorkflowExecutionInitiatedEventAttributes struct {
	Domain                              string                 `json:"domain,omitempty"`
	WorkflowID                          string                 `json:"workflowId,omitempty"`
	WorkflowType                        *WorkflowType          `json:"workflowType,omitempty"`
	TaskList                            *TaskList              `json:"taskList,omitempty"`
	Input                               []byte                 `json:"input,omitempty"`
	ExecutionStartToCloseTimeoutSeconds *int32                 `json:"executionStartToCloseTimeoutSeconds,omitempty"`
	TaskStartToCloseTimeoutSeconds      *int32                 `json:"taskStartToCloseTimeoutSeconds,omitempty"`
	ParentClosePolicy                   *ParentClosePolicy     `json:"parentClosePolicy,omitempty"`
	Control                             []byte                 `json:"control,omitempty"`
	DecisionTaskCompletedEventID        int64                  `json:"decisionTaskCompletedEventId,omitempty"`
	WorkflowIDReusePolicy               *WorkflowIDReusePolicy `json:"workflowIdReusePolicy,omitempty"`
	RetryPolicy                         *RetryPolicy           `json:"retryPolicy,omitempty"`
	CronSchedule                        string                 `json:"cronSchedule,omitempty"`
	Header                              *Header                `json:"header,omitempty"`
	Memo                                *Memo                  `json:"memo,omitempty"`
	SearchAttributes                    *SearchAttributes      `json:"searchAttributes,omitempty"`
	DelayStartSeconds                   *int32                 `json:"delayStartSeconds,omitempty"`
	JitterStartSeconds                  *int32                 `json:"jitterStartSeconds,omitempty"`
	FirstRunAtTimestamp                 *int64                 `json:"firstRunAtTimestamp,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionInitiatedEventAttributes) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionInitiatedEventAttributes) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetWorkflowType is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionInitiatedEventAttributes) GetWorkflowType() (o *WorkflowType) {
	if v != nil && v.WorkflowType != nil {
		return v.WorkflowType
	}
	return
}

// GetParentClosePolicy is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionInitiatedEventAttributes) GetParentClosePolicy() (o ParentClosePolicy) {
	if v != nil && v.ParentClosePolicy != nil {
		return *v.ParentClosePolicy
	}
	return
}

// GetExecutionStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *StartChildWorkflowExecutionInitiatedEventAttributes) GetExecutionStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.ExecutionStartToCloseTimeoutSeconds != nil {
		return *v.ExecutionStartToCloseTimeoutSeconds
	}
	return
}

// StartTimeFilter is an internal type (TBD...)
type StartTimeFilter struct {
	EarliestTime *int64 `json:"earliestTime,omitempty"`
	LatestTime   *int64 `json:"latestTime,omitempty"`
}

// GetEarliestTime is an internal getter (TBD...)
func (v *StartTimeFilter) GetEarliestTime() (o int64) {
	if v != nil && v.EarliestTime != nil {
		return *v.EarliestTime
	}
	return
}

// GetLatestTime is an internal getter (TBD...)
func (v *StartTimeFilter) GetLatestTime() (o int64) {
	if v != nil && v.LatestTime != nil {
		return *v.LatestTime
	}
	return
}

// StartTimerDecisionAttributes is an internal type (TBD...)
type StartTimerDecisionAttributes struct {
	TimerID                   string `json:"timerId,omitempty"`
	StartToFireTimeoutSeconds *int64 `json:"startToFireTimeoutSeconds,omitempty"`
}

// GetTimerID is an internal getter (TBD...)
func (v *StartTimerDecisionAttributes) GetTimerID() (o string) {
	if v != nil {
		return v.TimerID
	}
	return
}

// GetStartToFireTimeoutSeconds is an internal getter (TBD...)
func (v *StartTimerDecisionAttributes) GetStartToFireTimeoutSeconds() (o int64) {
	if v != nil && v.StartToFireTimeoutSeconds != nil {
		return *v.StartToFireTimeoutSeconds
	}
	return
}

type RestartWorkflowExecutionRequest struct {
	Domain            string             `json:"domain,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	Identity          string             `json:"identity,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *RestartWorkflowExecutionRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowExecution is an internal getter (TBD...)
func (v *RestartWorkflowExecutionRequest) GetWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.WorkflowExecution != nil {
		return v.WorkflowExecution
	}
	return
}

type DiagnoseWorkflowExecutionRequest struct {
	Domain            string             `json:"domain,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	Identity          string             `json:"identity,omitempty"`
}

// GetDomain returns the domain
func (v *DiagnoseWorkflowExecutionRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowExecution returns the workflow execution
func (v *DiagnoseWorkflowExecutionRequest) GetWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.WorkflowExecution != nil {
		return v.WorkflowExecution
	}
	return
}

type DiagnoseWorkflowExecutionResponse struct {
	Domain                      string             `json:"domain,omitempty"`
	DiagnosticWorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
}

// GetDomain returns the fomain
func (v *DiagnoseWorkflowExecutionResponse) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetDiagnosticWorkflowExecution returns the workflow execution
func (v *DiagnoseWorkflowExecutionResponse) GetDiagnosticWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.DiagnosticWorkflowExecution != nil {
		return v.DiagnosticWorkflowExecution
	}
	return
}

// StartWorkflowExecutionRequest is an internal type (TBD...)
type StartWorkflowExecutionRequest struct {
	Domain                              string                 `json:"domain,omitempty"`
	WorkflowID                          string                 `json:"workflowId,omitempty"`
	WorkflowType                        *WorkflowType          `json:"workflowType,omitempty"`
	TaskList                            *TaskList              `json:"taskList,omitempty"`
	Input                               []byte                 `json:"-"`
	ExecutionStartToCloseTimeoutSeconds *int32                 `json:"executionStartToCloseTimeoutSeconds,omitempty"`
	TaskStartToCloseTimeoutSeconds      *int32                 `json:"taskStartToCloseTimeoutSeconds,omitempty"`
	Identity                            string                 `json:"identity,omitempty"`
	RequestID                           string                 `json:"requestId,omitempty"`
	WorkflowIDReusePolicy               *WorkflowIDReusePolicy `json:"workflowIdReusePolicy,omitempty"`
	RetryPolicy                         *RetryPolicy           `json:"retryPolicy,omitempty"`
	CronSchedule                        string                 `json:"cronSchedule,omitempty"`
	Memo                                *Memo                  `json:"-"`
	SearchAttributes                    *SearchAttributes      `json:"-"`
	Header                              *Header                `json:"header,omitempty"`
	DelayStartSeconds                   *int32                 `json:"delayStartSeconds,omitempty"`
	JitterStartSeconds                  *int32                 `json:"jitterStartSeconds,omitempty"`
	FirstRunAtTimeStamp                 *int64                 `json:"firstRunAtTimeStamp,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *StartWorkflowExecutionRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *StartWorkflowExecutionRequest) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetExecutionStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *StartWorkflowExecutionRequest) GetExecutionStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.ExecutionStartToCloseTimeoutSeconds != nil {
		return *v.ExecutionStartToCloseTimeoutSeconds
	}
	return
}

// GetTaskStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *StartWorkflowExecutionRequest) GetTaskStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.TaskStartToCloseTimeoutSeconds != nil {
		return *v.TaskStartToCloseTimeoutSeconds
	}
	return
}

// GetDelayStartSeconds is an internal getter (TBD...)
func (v *StartWorkflowExecutionRequest) GetDelayStartSeconds() (o int32) {
	if v != nil && v.DelayStartSeconds != nil {
		return *v.DelayStartSeconds
	}
	return
}

// GetJitterStartSeconds is an internal getter (TBD...)
func (v *StartWorkflowExecutionRequest) GetJitterStartSeconds() (o int32) {
	if v != nil && v.JitterStartSeconds != nil {
		return *v.JitterStartSeconds
	}
	return
}

// GetFirstRunAtTimeStamp is an internal getter (TBD...)
func (v *StartWorkflowExecutionRequest) GetFirstRunAtTimeStamp() (o int64) {
	if v != nil && v.FirstRunAtTimeStamp != nil {
		return *v.FirstRunAtTimeStamp
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *StartWorkflowExecutionRequest) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// GetWorkflowIDReusePolicy is an internal getter (TBD...)
func (v *StartWorkflowExecutionRequest) GetWorkflowIDReusePolicy() (o WorkflowIDReusePolicy) {
	if v != nil && v.WorkflowIDReusePolicy != nil {
		return *v.WorkflowIDReusePolicy
	}
	return
}

// GetCronSchedule is an internal getter (TBD...)
func (v *StartWorkflowExecutionRequest) GetCronSchedule() (o string) {
	if v != nil {
		return v.CronSchedule
	}
	return
}

// StartWorkflowExecutionResponse is an internal type (TBD...)
type StartWorkflowExecutionResponse struct {
	RunID string `json:"runId,omitempty"`
}

// GetRunID is an internal getter (TBD...)
func (v *StartWorkflowExecutionResponse) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

type StartWorkflowExecutionAsyncRequest struct {
	*StartWorkflowExecutionRequest
}

type StartWorkflowExecutionAsyncResponse struct {
}

// RestartWorkflowExecutionResponse is an internal type (TBD...)
type RestartWorkflowExecutionResponse struct {
	RunID string `json:"runId,omitempty"`
}

// GetRunID is an internal getter (TBD...)
func (v *RestartWorkflowExecutionResponse) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// StickyExecutionAttributes is an internal type (TBD...)
type StickyExecutionAttributes struct {
	WorkerTaskList                *TaskList `json:"workerTaskList,omitempty"`
	ScheduleToStartTimeoutSeconds *int32    `json:"scheduleToStartTimeoutSeconds,omitempty"`
}

// GetScheduleToStartTimeoutSeconds is an internal getter (TBD...)
func (v *StickyExecutionAttributes) GetScheduleToStartTimeoutSeconds() (o int32) {
	if v != nil && v.ScheduleToStartTimeoutSeconds != nil {
		return *v.ScheduleToStartTimeoutSeconds
	}
	return
}

// SupportedClientVersions is an internal type (TBD...)
type SupportedClientVersions struct {
	GoSdk   string `json:"goSdk,omitempty"`
	JavaSdk string `json:"javaSdk,omitempty"`
}

// TaskIDBlock is an internal type (TBD...)
type TaskIDBlock struct {
	StartID int64 `json:"startID,omitempty"`
	EndID   int64 `json:"endID,omitempty"`
}

// GetStartID is an internal getter (TBD...)
func (v *TaskIDBlock) GetStartID() (o int64) {
	if v != nil {
		return v.StartID
	}
	return
}

// GetEndID is an internal getter (TBD...)
func (v *TaskIDBlock) GetEndID() (o int64) {
	if v != nil {
		return v.EndID
	}
	return
}

// TaskList is an internal type (TBD...)
type TaskList struct {
	Name string        `json:"name,omitempty"`
	Kind *TaskListKind `json:"kind,omitempty"`
}

// GetName is an internal getter (TBD...)
func (v *TaskList) GetName() (o string) {
	if v != nil {
		return v.Name
	}
	return
}

// GetKind is an internal getter (TBD...)
func (v *TaskList) GetKind() (o TaskListKind) {
	if v != nil && v.Kind != nil {
		return *v.Kind
	}
	return
}

// TaskListKind is an internal type (TBD...)
type TaskListKind int32

// Ptr is a helper function for getting pointer value
func (e TaskListKind) Ptr() *TaskListKind {
	return &e
}

// String returns a readable string representation of TaskListKind.
func (e TaskListKind) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "NORMAL"
	case 1:
		return "STICKY"
	}
	return fmt.Sprintf("TaskListKind(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *TaskListKind) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "NORMAL":
		*e = TaskListKindNormal
		return nil
	case "STICKY":
		*e = TaskListKindSticky
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "TaskListKind", err)
		}
		*e = TaskListKind(val)
		return nil
	}
}

// MarshalText encodes TaskListKind to text.
func (e TaskListKind) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// TaskListKindNormal is an option for TaskListKind
	TaskListKindNormal TaskListKind = iota
	// TaskListKindSticky is an option for TaskListKind
	TaskListKindSticky
)

// TaskListMetadata is an internal type (TBD...)
type TaskListMetadata struct {
	MaxTasksPerSecond *float64 `json:"maxTasksPerSecond,omitempty"`
}

// TaskListPartitionMetadata is an internal type (TBD...)
type TaskListPartitionMetadata struct {
	Key           string `json:"key,omitempty"`
	OwnerHostName string `json:"ownerHostName,omitempty"`
}

// GetKey is an internal getter (TBD...)
func (v *TaskListPartitionMetadata) GetKey() (o string) {
	if v != nil {
		return v.Key
	}
	return
}

// GetOwnerHostName is an internal getter (TBD...)
func (v *TaskListPartitionMetadata) GetOwnerHostName() (o string) {
	if v != nil {
		return v.OwnerHostName
	}
	return
}

type IsolationGroupMetrics struct {
	NewTasksPerSecond float64 `json:"newTasksPerSecond,omitempty"`
	PollerCount       int64   `json:"pollerCount,omitempty"`
}

// TaskListStatus is an internal type (TBD...)
type TaskListStatus struct {
	BacklogCountHint      int64                             `json:"backlogCountHint,omitempty"`
	ReadLevel             int64                             `json:"readLevel,omitempty"`
	AckLevel              int64                             `json:"ackLevel,omitempty"`
	RatePerSecond         float64                           `json:"ratePerSecond,omitempty"`
	TaskIDBlock           *TaskIDBlock                      `json:"taskIDBlock,omitempty"`
	IsolationGroupMetrics map[string]*IsolationGroupMetrics `json:"isolationGroupMetrics,omitempty"`
	NewTasksPerSecond     float64                           `json:"newTasksPerSecond,omitempty"`
}

// GetBacklogCountHint is an internal getter (TBD...)
func (v *TaskListStatus) GetBacklogCountHint() (o int64) {
	if v != nil {
		return v.BacklogCountHint
	}
	return
}

// GetReadLevel is an internal getter (TBD...)
func (v *TaskListStatus) GetReadLevel() (o int64) {
	if v != nil {
		return v.ReadLevel
	}
	return
}

// GetAckLevel is an internal getter (TBD...)
func (v *TaskListStatus) GetAckLevel() (o int64) {
	if v != nil {
		return v.AckLevel
	}
	return
}

// GetRatePerSecond is an internal getter (TBD...)
func (v *TaskListStatus) GetRatePerSecond() (o float64) {
	if v != nil {
		return v.RatePerSecond
	}
	return
}

// GetTaskIDBlock is an internal getter (TBD...)
func (v *TaskListStatus) GetTaskIDBlock() (o *TaskIDBlock) {
	if v != nil && v.TaskIDBlock != nil {
		return v.TaskIDBlock
	}
	return
}

// TaskListType is an internal type (TBD...)
type TaskListType int32

// Ptr is a helper function for getting pointer value
func (e TaskListType) Ptr() *TaskListType {
	return &e
}

// String returns a readable string representation of TaskListType.
func (e TaskListType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "Decision"
	case 1:
		return "Activity"
	}
	return fmt.Sprintf("TaskListType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *TaskListType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "DECISION":
		*e = TaskListTypeDecision
		return nil
	case "ACTIVITY":
		*e = TaskListTypeActivity
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "TaskListType", err)
		}
		*e = TaskListType(val)
		return nil
	}
}

// MarshalText encodes TaskListType to text.
func (e TaskListType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// TaskListTypeDecision is an option for TaskListType
	TaskListTypeDecision TaskListType = iota
	// TaskListTypeActivity is an option for TaskListType
	TaskListTypeActivity
)

// TerminateWorkflowExecutionRequest is an internal type (TBD...)
type TerminateWorkflowExecutionRequest struct {
	Domain              string             `json:"domain,omitempty"`
	WorkflowExecution   *WorkflowExecution `json:"workflowExecution,omitempty"`
	Reason              string             `json:"reason,omitempty"`
	Details             []byte             `json:"details,omitempty"`
	Identity            string             `json:"identity,omitempty"`
	FirstExecutionRunID string             `json:"first_execution_run_id,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *TerminateWorkflowExecutionRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetWorkflowExecution is an internal getter (TBD...)
func (v *TerminateWorkflowExecutionRequest) GetWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.WorkflowExecution != nil {
		return v.WorkflowExecution
	}
	return
}

// GetReason is an internal getter (TBD...)
func (v *TerminateWorkflowExecutionRequest) GetReason() (o string) {
	if v != nil {
		return v.Reason
	}
	return
}

// GetDetails is an internal getter (TBD...)
func (v *TerminateWorkflowExecutionRequest) GetDetails() (o []byte) {
	if v != nil && v.Details != nil {
		return v.Details
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *TerminateWorkflowExecutionRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// GetFirstExecutionRunID is an internal getter (TBD...)
func (v *TerminateWorkflowExecutionRequest) GetFirstExecutionRunID() (o string) {
	if v != nil {
		return v.FirstExecutionRunID
	}
	return
}

// TimeoutType is an internal type (TBD...)
type TimeoutType int32

// Ptr is a helper function for getting pointer value
func (e TimeoutType) Ptr() *TimeoutType {
	return &e
}

// String returns a readable string representation of TimeoutType.
func (e TimeoutType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "START_TO_CLOSE"
	case 1:
		return "SCHEDULE_TO_START"
	case 2:
		return "SCHEDULE_TO_CLOSE"
	case 3:
		return "HEARTBEAT"
	}
	return fmt.Sprintf("TimeoutType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *TimeoutType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "START_TO_CLOSE":
		*e = TimeoutTypeStartToClose
		return nil
	case "SCHEDULE_TO_START":
		*e = TimeoutTypeScheduleToStart
		return nil
	case "SCHEDULE_TO_CLOSE":
		*e = TimeoutTypeScheduleToClose
		return nil
	case "HEARTBEAT":
		*e = TimeoutTypeHeartbeat
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "TimeoutType", err)
		}
		*e = TimeoutType(val)
		return nil
	}
}

// MarshalText encodes TimeoutType to text.
func (e TimeoutType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// TimeoutTypeStartToClose is an option for TimeoutType
	TimeoutTypeStartToClose TimeoutType = iota
	// TimeoutTypeScheduleToStart is an option for TimeoutType
	TimeoutTypeScheduleToStart
	// TimeoutTypeScheduleToClose is an option for TimeoutType
	TimeoutTypeScheduleToClose
	// TimeoutTypeHeartbeat is an option for TimeoutType
	TimeoutTypeHeartbeat
)

// TimerCanceledEventAttributes is an internal type (TBD...)
type TimerCanceledEventAttributes struct {
	TimerID                      string `json:"timerId,omitempty"`
	StartedEventID               int64  `json:"startedEventId,omitempty"`
	DecisionTaskCompletedEventID int64  `json:"decisionTaskCompletedEventId,omitempty"`
	Identity                     string `json:"identity,omitempty"`
}

// GetTimerID is an internal getter (TBD...)
func (v *TimerCanceledEventAttributes) GetTimerID() (o string) {
	if v != nil {
		return v.TimerID
	}
	return
}

// TimerFiredEventAttributes is an internal type (TBD...)
type TimerFiredEventAttributes struct {
	TimerID        string `json:"timerId,omitempty"`
	StartedEventID int64  `json:"startedEventId,omitempty"`
}

// GetTimerID is an internal getter (TBD...)
func (v *TimerFiredEventAttributes) GetTimerID() (o string) {
	if v != nil {
		return v.TimerID
	}
	return
}

// GetStartedEventID is an internal getter (TBD...)
func (v *TimerFiredEventAttributes) GetStartedEventID() (o int64) {
	if v != nil {
		return v.StartedEventID
	}
	return
}

// TimerStartedEventAttributes is an internal type (TBD...)
type TimerStartedEventAttributes struct {
	TimerID                      string `json:"timerId,omitempty"`
	StartToFireTimeoutSeconds    *int64 `json:"startToFireTimeoutSeconds,omitempty"`
	DecisionTaskCompletedEventID int64  `json:"decisionTaskCompletedEventId,omitempty"`
}

// GetTimerID is an internal getter (TBD...)
func (v *TimerStartedEventAttributes) GetTimerID() (o string) {
	if v != nil {
		return v.TimerID
	}
	return
}

// GetStartToFireTimeoutSeconds is an internal getter (TBD...)
func (v *TimerStartedEventAttributes) GetStartToFireTimeoutSeconds() (o int64) {
	if v != nil && v.StartToFireTimeoutSeconds != nil {
		return *v.StartToFireTimeoutSeconds
	}
	return
}

// TransientDecisionInfo is an internal type (TBD...)
type TransientDecisionInfo struct {
	ScheduledEvent *HistoryEvent `json:"scheduledEvent,omitempty"`
	StartedEvent   *HistoryEvent `json:"startedEvent,omitempty"`
}

// UpdateDomainRequest is an internal type (TBD...)
type UpdateDomainRequest struct {
	Name                                   string                             `json:"name,omitempty"`
	Description                            *string                            `json:"description,omitempty"`
	OwnerEmail                             *string                            `json:"ownerEmail,omitempty"`
	Data                                   map[string]string                  `json:"data,omitempty"`
	WorkflowExecutionRetentionPeriodInDays *int32                             `json:"workflowExecutionRetentionPeriodInDays,omitempty"`
	EmitMetric                             *bool                              `json:"emitMetric,omitempty"`
	BadBinaries                            *BadBinaries                       `json:"badBinaries,omitempty"`
	HistoryArchivalStatus                  *ArchivalStatus                    `json:"historyArchivalStatus,omitempty"`
	HistoryArchivalURI                     *string                            `json:"historyArchivalURI,omitempty"`
	VisibilityArchivalStatus               *ArchivalStatus                    `json:"visibilityArchivalStatus,omitempty"`
	VisibilityArchivalURI                  *string                            `json:"visibilityArchivalURI,omitempty"`
	ActiveClusterName                      *string                            `json:"activeClusterName,omitempty"`
	Clusters                               []*ClusterReplicationConfiguration `json:"clusters,omitempty"`
	SecurityToken                          string                             `json:"securityToken,omitempty"`
	DeleteBadBinary                        *string                            `json:"deleteBadBinary,omitempty"`
	FailoverTimeoutInSeconds               *int32                             `json:"failoverTimeoutInSeconds,omitempty"`
}

// GetName is an internal getter (TBD...)
func (v *UpdateDomainRequest) GetName() (o string) {
	if v != nil {
		return v.Name
	}
	return
}

// GetFailoverTimeoutInSeconds is an internal getter (TBD...)
func (v *UpdateDomainRequest) GetFailoverTimeoutInSeconds() (o int32) {
	if v != nil && v.FailoverTimeoutInSeconds != nil {
		return *v.FailoverTimeoutInSeconds
	}
	return
}

// GetHistoryArchivalURI is an internal getter (TBD...)
func (v *UpdateDomainRequest) GetHistoryArchivalURI() (o string) {
	if v != nil && v.HistoryArchivalURI != nil {
		return *v.HistoryArchivalURI
	}
	return
}

// GetVisibilityArchivalURI is an internal getter (TBD...)
func (v *UpdateDomainRequest) GetVisibilityArchivalURI() (o string) {
	if v != nil && v.VisibilityArchivalURI != nil {
		return *v.VisibilityArchivalURI
	}
	return
}

// UpdateDomainResponse is an internal type (TBD...)
type UpdateDomainResponse struct {
	DomainInfo               *DomainInfo                     `json:"domainInfo,omitempty"`
	Configuration            *DomainConfiguration            `json:"configuration,omitempty"`
	ReplicationConfiguration *DomainReplicationConfiguration `json:"replicationConfiguration,omitempty"`
	FailoverVersion          int64                           `json:"failoverVersion,omitempty"`
	IsGlobalDomain           bool                            `json:"isGlobalDomain,omitempty"`
}

// GetDomainInfo is an internal getter (TBD...)
func (v *UpdateDomainResponse) GetDomainInfo() (o *DomainInfo) {
	if v != nil && v.DomainInfo != nil {
		return v.DomainInfo
	}
	return
}

// GetFailoverVersion is an internal getter (TBD...)
func (v *UpdateDomainResponse) GetFailoverVersion() (o int64) {
	if v != nil {
		return v.FailoverVersion
	}
	return
}

// GetIsGlobalDomain is an internal getter (TBD...)
func (v *UpdateDomainResponse) GetIsGlobalDomain() (o bool) {
	if v != nil {
		return v.IsGlobalDomain
	}
	return
}

// UpsertWorkflowSearchAttributesDecisionAttributes is an internal type (TBD...)
type UpsertWorkflowSearchAttributesDecisionAttributes struct {
	SearchAttributes *SearchAttributes `json:"searchAttributes,omitempty"`
}

// GetSearchAttributes is an internal getter (TBD...)
func (v *UpsertWorkflowSearchAttributesDecisionAttributes) GetSearchAttributes() (o *SearchAttributes) {
	if v != nil && v.SearchAttributes != nil {
		return v.SearchAttributes
	}
	return
}

// UpsertWorkflowSearchAttributesEventAttributes is an internal type (TBD...)
type UpsertWorkflowSearchAttributesEventAttributes struct {
	DecisionTaskCompletedEventID int64             `json:"decisionTaskCompletedEventId,omitempty"`
	SearchAttributes             *SearchAttributes `json:"searchAttributes,omitempty"`
}

// GetSearchAttributes is an internal getter (TBD...)
func (v *UpsertWorkflowSearchAttributesEventAttributes) GetSearchAttributes() (o *SearchAttributes) {
	if v != nil && v.SearchAttributes != nil {
		return v.SearchAttributes
	}
	return
}

// VersionHistories is an internal type (TBD...)
type VersionHistories struct {
	CurrentVersionHistoryIndex int32             `json:"currentVersionHistoryIndex,omitempty"`
	Histories                  []*VersionHistory `json:"histories,omitempty"`
}

// GetCurrentVersionHistoryIndex is an internal getter (TBD...)
func (v *VersionHistories) GetCurrentVersionHistoryIndex() (o int32) {
	if v != nil {
		return v.CurrentVersionHistoryIndex
	}
	return
}

// VersionHistory is an internal type (TBD...)
type VersionHistory struct {
	BranchToken []byte                `json:"branchToken,omitempty"`
	Items       []*VersionHistoryItem `json:"items,omitempty"`
}

// GetItems is an internal getter (TBD...)
func (v *VersionHistory) GetItems() (o []*VersionHistoryItem) {
	if v != nil && v.Items != nil {
		return v.Items
	}
	return
}

// VersionHistoryItem is an internal type (TBD...)
type VersionHistoryItem struct {
	EventID int64 `json:"eventID,omitempty"`
	Version int64 `json:"version,omitempty"`
}

// GetVersion is an internal getter (TBD...)
func (v *VersionHistoryItem) GetVersion() (o int64) {
	if v != nil {
		return v.Version
	}
	return
}

// WorkerVersionInfo is an internal type (TBD...)
type WorkerVersionInfo struct {
	Impl           string `json:"impl,omitempty"`
	FeatureVersion string `json:"featureVersion,omitempty"`
}

// GetImpl is an internal getter (TBD...)
func (v *WorkerVersionInfo) GetImpl() (o string) {
	if v != nil {
		return v.Impl
	}
	return
}

// GetFeatureVersion is an internal getter (TBD...)
func (v *WorkerVersionInfo) GetFeatureVersion() (o string) {
	if v != nil {
		return v.FeatureVersion
	}
	return
}

// WorkflowExecution is an internal type (TBD...)
type WorkflowExecution struct {
	WorkflowID string `json:"workflowId,omitempty"`
	RunID      string `json:"runId,omitempty"`
}

// GetWorkflowID is an internal getter (TBD...)
func (v *WorkflowExecution) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *WorkflowExecution) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// WorkflowExecutionAlreadyStartedError is an internal type (TBD...)
type WorkflowExecutionAlreadyStartedError struct {
	Message        string `json:"message,omitempty"`
	StartRequestID string `json:"startRequestId,omitempty"`
	RunID          string `json:"runId,omitempty"`
}

// GetMessage is an internal getter (TBD...)
func (v *WorkflowExecutionAlreadyStartedError) GetMessage() (o string) {
	if v != nil {
		return v.Message
	}
	return
}

// WorkflowExecutionCancelRequestedEventAttributes is an internal type (TBD...)
type WorkflowExecutionCancelRequestedEventAttributes struct {
	Cause                     string             `json:"cause,omitempty"`
	ExternalInitiatedEventID  *int64             `json:"externalInitiatedEventId,omitempty"`
	ExternalWorkflowExecution *WorkflowExecution `json:"externalWorkflowExecution,omitempty"`
	Identity                  string             `json:"identity,omitempty"`
	RequestID                 string             `json:"requestId,omitempty"`
}

// WorkflowExecutionCanceledEventAttributes is an internal type (TBD...)
type WorkflowExecutionCanceledEventAttributes struct {
	DecisionTaskCompletedEventID int64  `json:"decisionTaskCompletedEventId,omitempty"`
	Details                      []byte `json:"details,omitempty"`
}

// WorkflowExecutionCloseStatus is an internal type (TBD...)
type WorkflowExecutionCloseStatus int32

// Ptr is a helper function for getting pointer value
func (e WorkflowExecutionCloseStatus) Ptr() *WorkflowExecutionCloseStatus {
	return &e
}

// String returns a readable string representation of WorkflowExecutionCloseStatus.
func (e WorkflowExecutionCloseStatus) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "COMPLETED"
	case 1:
		return "FAILED"
	case 2:
		return "CANCELED"
	case 3:
		return "TERMINATED"
	case 4:
		return "CONTINUED_AS_NEW"
	case 5:
		return "TIMED_OUT"
	}
	return fmt.Sprintf("WorkflowExecutionCloseStatus(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *WorkflowExecutionCloseStatus) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "COMPLETED":
		*e = WorkflowExecutionCloseStatusCompleted
		return nil
	case "FAILED":
		*e = WorkflowExecutionCloseStatusFailed
		return nil
	case "CANCELED":
		*e = WorkflowExecutionCloseStatusCanceled
		return nil
	case "TERMINATED":
		*e = WorkflowExecutionCloseStatusTerminated
		return nil
	case "CONTINUED_AS_NEW":
		*e = WorkflowExecutionCloseStatusContinuedAsNew
		return nil
	case "TIMED_OUT":
		*e = WorkflowExecutionCloseStatusTimedOut
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "WorkflowExecutionCloseStatus", err)
		}
		*e = WorkflowExecutionCloseStatus(val)
		return nil
	}
}

// MarshalText encodes WorkflowExecutionCloseStatus to text.
func (e WorkflowExecutionCloseStatus) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// WorkflowExecutionCloseStatusCompleted is an option for WorkflowExecutionCloseStatus
	WorkflowExecutionCloseStatusCompleted WorkflowExecutionCloseStatus = iota
	// WorkflowExecutionCloseStatusFailed is an option for WorkflowExecutionCloseStatus
	WorkflowExecutionCloseStatusFailed
	// WorkflowExecutionCloseStatusCanceled is an option for WorkflowExecutionCloseStatus
	WorkflowExecutionCloseStatusCanceled
	// WorkflowExecutionCloseStatusTerminated is an option for WorkflowExecutionCloseStatus
	WorkflowExecutionCloseStatusTerminated
	// WorkflowExecutionCloseStatusContinuedAsNew is an option for WorkflowExecutionCloseStatus
	WorkflowExecutionCloseStatusContinuedAsNew
	// WorkflowExecutionCloseStatusTimedOut is an option for WorkflowExecutionCloseStatus
	WorkflowExecutionCloseStatusTimedOut
)

// WorkflowExecutionCompletedEventAttributes is an internal type (TBD...)
type WorkflowExecutionCompletedEventAttributes struct {
	Result                       []byte `json:"result,omitempty"`
	DecisionTaskCompletedEventID int64  `json:"decisionTaskCompletedEventId,omitempty"`
}

// WorkflowExecutionConfiguration is an internal type (TBD...)
type WorkflowExecutionConfiguration struct {
	TaskList                            *TaskList `json:"taskList,omitempty"`
	ExecutionStartToCloseTimeoutSeconds *int32    `json:"executionStartToCloseTimeoutSeconds,omitempty"`
	TaskStartToCloseTimeoutSeconds      *int32    `json:"taskStartToCloseTimeoutSeconds,omitempty"`
}

// WorkflowExecutionContinuedAsNewEventAttributes is an internal type (TBD...)
type WorkflowExecutionContinuedAsNewEventAttributes struct {
	NewExecutionRunID                   string                  `json:"newExecutionRunId,omitempty"`
	WorkflowType                        *WorkflowType           `json:"workflowType,omitempty"`
	TaskList                            *TaskList               `json:"taskList,omitempty"`
	Input                               []byte                  `json:"input,omitempty"`
	ExecutionStartToCloseTimeoutSeconds *int32                  `json:"executionStartToCloseTimeoutSeconds,omitempty"`
	TaskStartToCloseTimeoutSeconds      *int32                  `json:"taskStartToCloseTimeoutSeconds,omitempty"`
	DecisionTaskCompletedEventID        int64                   `json:"decisionTaskCompletedEventId,omitempty"`
	BackoffStartIntervalInSeconds       *int32                  `json:"backoffStartIntervalInSeconds,omitempty"`
	Initiator                           *ContinueAsNewInitiator `json:"initiator,omitempty"`
	FailureReason                       *string                 `json:"failureReason,omitempty"`
	FailureDetails                      []byte                  `json:"failureDetails,omitempty"`
	LastCompletionResult                []byte                  `json:"lastCompletionResult,omitempty"`
	Header                              *Header                 `json:"header,omitempty"`
	Memo                                *Memo                   `json:"memo,omitempty"`
	SearchAttributes                    *SearchAttributes       `json:"searchAttributes,omitempty"`
	JitterStartSeconds                  *int32                  `json:"jitterStartSeconds,omitempty"`
}

// GetNewExecutionRunID is an internal getter (TBD...)
func (v *WorkflowExecutionContinuedAsNewEventAttributes) GetNewExecutionRunID() (o string) {
	if v != nil {
		return v.NewExecutionRunID
	}
	return
}

// GetInitiator is an internal getter (TBD...)
func (v *WorkflowExecutionContinuedAsNewEventAttributes) GetInitiator() (o ContinueAsNewInitiator) {
	if v != nil && v.Initiator != nil {
		return *v.Initiator
	}
	return
}

// GetFailureReason is an internal getter (TBD...)
func (v *WorkflowExecutionContinuedAsNewEventAttributes) GetFailureReason() (o string) {
	if v != nil && v.FailureReason != nil {
		return *v.FailureReason
	}
	return
}

// GetLastCompletionResult is an internal getter (TBD...)
func (v *WorkflowExecutionContinuedAsNewEventAttributes) GetLastCompletionResult() (o []byte) {
	if v != nil && v.LastCompletionResult != nil {
		return v.LastCompletionResult
	}
	return
}

// WorkflowExecutionFailedEventAttributes is an internal type (TBD...)
type WorkflowExecutionFailedEventAttributes struct {
	Reason                       *string `json:"reason,omitempty"`
	Details                      []byte  `json:"details,omitempty"`
	DecisionTaskCompletedEventID int64   `json:"decisionTaskCompletedEventId,omitempty"`
}

// GetReason is an internal getter (TBD...)
func (v *WorkflowExecutionFailedEventAttributes) GetReason() (o string) {
	if v != nil && v.Reason != nil {
		return *v.Reason
	}
	return
}

// WorkflowExecutionFilter is an internal type (TBD...)
type WorkflowExecutionFilter struct {
	WorkflowID string `json:"workflowId,omitempty"`
	RunID      string `json:"runId,omitempty"`
}

// GetWorkflowID is an internal getter (TBD...)
func (v *WorkflowExecutionFilter) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// WorkflowExecutionInfo is an internal type (TBD...)
type WorkflowExecutionInfo struct {
	Execution         *WorkflowExecution            `json:"execution,omitempty"`
	Type              *WorkflowType                 `json:"type,omitempty"`
	StartTime         *int64                        `json:"startTime,omitempty"`
	CloseTime         *int64                        `json:"closeTime,omitempty"`
	CloseStatus       *WorkflowExecutionCloseStatus `json:"closeStatus,omitempty"`
	HistoryLength     int64                         `json:"historyLength,omitempty"` // should be history count
	ParentDomainID    *string                       `json:"parentDomainId,omitempty"`
	ParentDomain      *string                       `json:"parentDomain,omitempty"`
	ParentExecution   *WorkflowExecution            `json:"parentExecution,omitempty"`
	ParentInitiatedID *int64                        `json:"parentInitiatedId,omitempty"`
	ExecutionTime     *int64                        `json:"executionTime,omitempty"`
	Memo              *Memo                         `json:"memo,omitempty"`
	SearchAttributes  *SearchAttributes             `json:"searchAttributes,omitempty"`
	AutoResetPoints   *ResetPoints                  `json:"autoResetPoints,omitempty"`
	TaskList          string                        `json:"taskList,omitempty"`
	IsCron            bool                          `json:"isCron,omitempty"`
	UpdateTime        *int64                        `json:"updateTime,omitempty"`
	PartitionConfig   map[string]string
}

// GetExecution is an internal getter (TBD...)
func (v *WorkflowExecutionInfo) GetExecution() (o *WorkflowExecution) {
	if v != nil && v.Execution != nil {
		return v.Execution
	}
	return
}

// GetType is an internal getter (TBD...)
func (v *WorkflowExecutionInfo) GetType() (o *WorkflowType) {
	if v != nil && v.Type != nil {
		return v.Type
	}
	return
}

// GetStartTime is an internal getter (TBD...)
func (v *WorkflowExecutionInfo) GetStartTime() (o int64) {
	if v != nil && v.StartTime != nil {
		return *v.StartTime
	}
	return
}

// GetCloseTime is an internal getter (TBD...)
func (v *WorkflowExecutionInfo) GetCloseTime() (o int64) {
	if v != nil && v.CloseTime != nil {
		return *v.CloseTime
	}
	return
}

// GetCloseStatus is an internal getter (TBD...)
func (v *WorkflowExecutionInfo) GetCloseStatus() (o WorkflowExecutionCloseStatus) {
	if v != nil && v.CloseStatus != nil {
		return *v.CloseStatus
	}
	return
}

// GetExecutionTime is an internal getter (TBD...)
func (v *WorkflowExecutionInfo) GetExecutionTime() (o int64) {
	if v != nil && v.ExecutionTime != nil {
		return *v.ExecutionTime
	}
	return
}

// GetUpdateTime is an internal getter (TBD...)
func (v *WorkflowExecutionInfo) GetUpdateTime() (o int64) {
	if v != nil && v.UpdateTime != nil {
		return *v.UpdateTime
	}
	return
}

// GetSearchAttributes is an internal getter (TBD...)
func (v *WorkflowExecutionInfo) GetSearchAttributes() (o *SearchAttributes) {
	if v != nil && v.SearchAttributes != nil {
		return v.SearchAttributes
	}
	return
}

// GetPartitionConfig is an internal getter (TBD...)
func (v *WorkflowExecutionInfo) GetPartitionConfig() (o map[string]string) {
	if v != nil && v.PartitionConfig != nil {
		return v.PartitionConfig
	}
	return
}

// WorkflowExecutionSignaledEventAttributes is an internal type (TBD...)
type WorkflowExecutionSignaledEventAttributes struct {
	SignalName string `json:"signalName,omitempty"`
	Input      []byte `json:"input,omitempty"`
	Identity   string `json:"identity,omitempty"`
	RequestID  string `json:"requestId,omitempty"`
}

// GetSignalName is an internal getter (TBD...)
func (v *WorkflowExecutionSignaledEventAttributes) GetSignalName() (o string) {
	if v != nil {
		return v.SignalName
	}
	return
}

// GetInput is an internal getter (TBD...)
func (v *WorkflowExecutionSignaledEventAttributes) GetInput() (o []byte) {
	if v != nil && v.Input != nil {
		return v.Input
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *WorkflowExecutionSignaledEventAttributes) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *WorkflowExecutionSignaledEventAttributes) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// WorkflowExecutionStartedEventAttributes is an internal type (TBD...)
type WorkflowExecutionStartedEventAttributes struct {
	WorkflowType                        *WorkflowType           `json:"workflowType,omitempty"`
	ParentWorkflowDomainID              *string                 `json:"parentWorkflowDomainID,omitempty"`
	ParentWorkflowDomain                *string                 `json:"parentWorkflowDomain,omitempty"`
	ParentWorkflowExecution             *WorkflowExecution      `json:"parentWorkflowExecution,omitempty"`
	ParentInitiatedEventID              *int64                  `json:"parentInitiatedEventId,omitempty"`
	TaskList                            *TaskList               `json:"taskList,omitempty"`
	Input                               []byte                  `json:"input,omitempty"`
	ExecutionStartToCloseTimeoutSeconds *int32                  `json:"executionStartToCloseTimeoutSeconds,omitempty"`
	TaskStartToCloseTimeoutSeconds      *int32                  `json:"taskStartToCloseTimeoutSeconds,omitempty"`
	ContinuedExecutionRunID             string                  `json:"continuedExecutionRunId,omitempty"`
	Initiator                           *ContinueAsNewInitiator `json:"initiator,omitempty"`
	ContinuedFailureReason              *string                 `json:"continuedFailureReason,omitempty"`
	ContinuedFailureDetails             []byte                  `json:"continuedFailureDetails,omitempty"`
	LastCompletionResult                []byte                  `json:"lastCompletionResult,omitempty"`
	OriginalExecutionRunID              string                  `json:"originalExecutionRunId,omitempty"`
	Identity                            string                  `json:"identity,omitempty"`
	FirstExecutionRunID                 string                  `json:"firstExecutionRunId,omitempty"`
	FirstScheduleTime                   *time.Time              `json:"firstScheduleTimeNano,omitempty"`
	RetryPolicy                         *RetryPolicy            `json:"retryPolicy,omitempty"`
	Attempt                             int32                   `json:"attempt,omitempty"`
	ExpirationTimestamp                 *int64                  `json:"expirationTimestamp,omitempty"`
	CronSchedule                        string                  `json:"cronSchedule,omitempty"`
	FirstDecisionTaskBackoffSeconds     *int32                  `json:"firstDecisionTaskBackoffSeconds,omitempty"`
	Memo                                *Memo                   `json:"memo,omitempty"`
	SearchAttributes                    *SearchAttributes       `json:"searchAttributes,omitempty"`
	PrevAutoResetPoints                 *ResetPoints            `json:"prevAutoResetPoints,omitempty"`
	Header                              *Header                 `json:"header,omitempty"`
	JitterStartSeconds                  *int32                  `json:"jitterStartSeconds,omitempty"`
	PartitionConfig                     map[string]string
	RequestID                           string `json:"requestId,omitempty"`
}

// GetParentWorkflowDomain is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetParentWorkflowDomain() (o string) {
	if v != nil && v.ParentWorkflowDomain != nil {
		return *v.ParentWorkflowDomain
	}
	return
}

// GetParentInitiatedEventID is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetParentInitiatedEventID() (o int64) {
	if v != nil && v.ParentInitiatedEventID != nil {
		return *v.ParentInitiatedEventID
	}
	return
}

// GetExecutionStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetExecutionStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.ExecutionStartToCloseTimeoutSeconds != nil {
		return *v.ExecutionStartToCloseTimeoutSeconds
	}
	return
}

// GetTaskStartToCloseTimeoutSeconds is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetTaskStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.TaskStartToCloseTimeoutSeconds != nil {
		return *v.TaskStartToCloseTimeoutSeconds
	}
	return
}

// GetContinuedExecutionRunID is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetContinuedExecutionRunID() (o string) {
	if v != nil {
		return v.ContinuedExecutionRunID
	}
	return
}

// GetInitiator is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetInitiator() (o ContinueAsNewInitiator) {
	if v != nil && v.Initiator != nil {
		return *v.Initiator
	}
	return
}

// GetFirstExecutionRunID is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetFirstExecutionRunID() (o string) {
	if v != nil {
		return v.FirstExecutionRunID
	}
	return
}

// Get
func (v *WorkflowExecutionStartedEventAttributes) GetFirstScheduledTime() (o time.Time) {
	if v != nil && v.FirstScheduleTime != nil {
		return *v.FirstScheduleTime
	}
	return
}

// GetAttempt is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetAttempt() (o int32) {
	if v != nil {
		return v.Attempt
	}
	return
}

// GetExpirationTimestamp is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetExpirationTimestamp() (o int64) {
	if v != nil && v.ExpirationTimestamp != nil {
		return *v.ExpirationTimestamp
	}
	return
}

// GetCronSchedule is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetCronSchedule() (o string) {
	if v != nil {
		return v.CronSchedule
	}
	return
}

// GetFirstDecisionTaskBackoffSeconds is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetFirstDecisionTaskBackoffSeconds() (o int32) {
	if v != nil && v.FirstDecisionTaskBackoffSeconds != nil {
		return *v.FirstDecisionTaskBackoffSeconds
	}
	return
}

func (v *WorkflowExecutionStartedEventAttributes) GetJitterStartSeconds() (o int32) {
	if v != nil && v.JitterStartSeconds != nil {
		return *v.JitterStartSeconds
	}
	return
}

// GetMemo is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetMemo() (o *Memo) {
	if v != nil && v.Memo != nil {
		return v.Memo
	}
	return
}

// GetSearchAttributes is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetSearchAttributes() (o *SearchAttributes) {
	if v != nil && v.SearchAttributes != nil {
		return v.SearchAttributes
	}
	return
}

// GetPrevAutoResetPoints is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetPrevAutoResetPoints() (o *ResetPoints) {
	if v != nil && v.PrevAutoResetPoints != nil {
		return v.PrevAutoResetPoints
	}
	return
}

// GetPartitionConfig is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetPartitionConfig() (o map[string]string) {
	if v != nil && v.PartitionConfig != nil {
		return v.PartitionConfig
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *WorkflowExecutionStartedEventAttributes) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// WorkflowExecutionTerminatedEventAttributes is an internal type (TBD...)
type WorkflowExecutionTerminatedEventAttributes struct {
	Reason   string `json:"reason,omitempty"`
	Details  []byte `json:"details,omitempty"`
	Identity string `json:"identity,omitempty"`
}

// GetReason is an internal getter (TBD...)
func (v *WorkflowExecutionTerminatedEventAttributes) GetReason() (o string) {
	if v != nil {
		return v.Reason
	}
	return
}

// GetIdentity is an internal getter (TBD...)
func (v *WorkflowExecutionTerminatedEventAttributes) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// WorkflowExecutionTimedOutEventAttributes is an internal type (TBD...)
type WorkflowExecutionTimedOutEventAttributes struct {
	TimeoutType *TimeoutType `json:"timeoutType,omitempty"`
}

// GetTimeoutType is an internal getter (TBD...)
func (v *WorkflowExecutionTimedOutEventAttributes) GetTimeoutType() (o TimeoutType) {
	if v != nil && v.TimeoutType != nil {
		return *v.TimeoutType
	}
	return
}

// WorkflowIDReusePolicy is an internal type (TBD...)
type WorkflowIDReusePolicy int32

// Ptr is a helper function for getting pointer value
func (e WorkflowIDReusePolicy) Ptr() *WorkflowIDReusePolicy {
	return &e
}

// String returns a readable string representation of WorkflowIDReusePolicy.
func (e WorkflowIDReusePolicy) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "AllowDuplicateFailedOnly"
	case 1:
		return "AllowDuplicate"
	case 2:
		return "RejectDuplicate"
	case 3:
		return "TerminateIfRunning"
	}
	return fmt.Sprintf("WorkflowIDReusePolicy(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *WorkflowIDReusePolicy) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "ALLOWDUPLICATEFAILEDONLY":
		*e = WorkflowIDReusePolicyAllowDuplicateFailedOnly
		return nil
	case "ALLOWDUPLICATE":
		*e = WorkflowIDReusePolicyAllowDuplicate
		return nil
	case "REJECTDUPLICATE":
		*e = WorkflowIDReusePolicyRejectDuplicate
		return nil
	case "TERMINATEIFRUNNING":
		*e = WorkflowIDReusePolicyTerminateIfRunning
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "WorkflowIDReusePolicy", err)
		}
		*e = WorkflowIDReusePolicy(val)
		return nil
	}
}

// MarshalText encodes WorkflowIDReusePolicy to text.
func (e WorkflowIDReusePolicy) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// WorkflowIDReusePolicyAllowDuplicateFailedOnly is an option for WorkflowIDReusePolicy
	WorkflowIDReusePolicyAllowDuplicateFailedOnly WorkflowIDReusePolicy = iota
	// WorkflowIDReusePolicyAllowDuplicate is an option for WorkflowIDReusePolicy
	WorkflowIDReusePolicyAllowDuplicate
	// WorkflowIDReusePolicyRejectDuplicate is an option for WorkflowIDReusePolicy
	WorkflowIDReusePolicyRejectDuplicate
	// WorkflowIDReusePolicyTerminateIfRunning is an option for WorkflowIDReusePolicy
	WorkflowIDReusePolicyTerminateIfRunning
)

// WorkflowQuery is an internal type (TBD...)
type WorkflowQuery struct {
	QueryType string `json:"queryType,omitempty"`
	QueryArgs []byte `json:"queryArgs,omitempty"`
}

// GetQueryType is an internal getter (TBD...)
func (v *WorkflowQuery) GetQueryType() (o string) {
	if v != nil {
		return v.QueryType
	}
	return
}

// GetQueryArgs is an internal getter (TBD...)
func (v *WorkflowQuery) GetQueryArgs() (o []byte) {
	if v != nil && v.QueryArgs != nil {
		return v.QueryArgs
	}
	return
}

// WorkflowQueryResult is an internal type (TBD...)
type WorkflowQueryResult struct {
	ResultType   *QueryResultType `json:"resultType,omitempty"`
	Answer       []byte           `json:"answer,omitempty"`
	ErrorMessage string           `json:"errorMessage,omitempty"`
}

// GetResultType is an internal getter (TBD...)
func (v *WorkflowQueryResult) GetResultType() (o QueryResultType) {
	if v != nil && v.ResultType != nil {
		return *v.ResultType
	}
	return
}

// GetAnswer is an internal getter (TBD...)
func (v *WorkflowQueryResult) GetAnswer() (o []byte) {
	if v != nil && v.Answer != nil {
		return v.Answer
	}
	return
}

// GetErrorMessage is an internal getter (TBD...)
func (v *WorkflowQueryResult) GetErrorMessage() (o string) {
	if v != nil {
		return v.ErrorMessage
	}
	return
}

// WorkflowType is an internal type (TBD...)
type WorkflowType struct {
	Name string `json:"name,omitempty"`
}

// GetName is an internal getter (TBD...)
func (v *WorkflowType) GetName() (o string) {
	if v != nil {
		return v.Name
	}
	return
}

// WorkflowTypeFilter is an internal type (TBD...)
type WorkflowTypeFilter struct {
	Name string `json:"name,omitempty"`
}

// GetName is an internal getter (TBD...)
func (v *WorkflowTypeFilter) GetName() (o string) {
	if v != nil {
		return v.Name
	}
	return
}

// CrossClusterTaskType is an internal type (TBD...)
type CrossClusterTaskType int32

// Ptr is a helper function for getting pointer value
func (e CrossClusterTaskType) Ptr() *CrossClusterTaskType {
	return &e
}

// String returns a readable string representation of CrossClusterTaskType.
func (e CrossClusterTaskType) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "StartChildExecution"
	case 1:
		return "CancelExecution"
	case 2:
		return "SignalExecution"
	case 3:
		return "RecordChildWorkflowExecutionComplete"
	case 4:
		return "ApplyParentClosePolicy"
	}
	return fmt.Sprintf("CrossClusterTaskType(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *CrossClusterTaskType) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "STARTCHILDEXECUTION":
		*e = CrossClusterTaskTypeStartChildExecution
		return nil
	case "CANCELEXECUTION":
		*e = CrossClusterTaskTypeCancelExecution
		return nil
	case "SIGNALEXECUTION":
		*e = CrossClusterTaskTypeSignalExecution
		return nil
	case "RECORDCHILDWORKLOWEXECUTIONCOMPLETE":
		*e = CrossClusterTaskTypeRecordChildWorkflowExeuctionComplete
		return nil
	case "APPLYPARENTCLOSEPOLICY":
		*e = CrossClusterTaskTypeApplyParentPolicy
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "CrossClusterTaskType", err)
		}
		*e = CrossClusterTaskType(val)
		return nil
	}
}

// MarshalText encodes CrossClusterTaskType to text.
func (e CrossClusterTaskType) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// CrossClusterTaskTypeStartChildExecution is an option for CrossClusterTaskType
	CrossClusterTaskTypeStartChildExecution CrossClusterTaskType = iota
	// CrossClusterTaskTypeCancelExecution is an option for CrossClusterTaskType
	CrossClusterTaskTypeCancelExecution
	// CrossClusterTaskTypeSignalExecution is an option for CrossClusterTaskType
	CrossClusterTaskTypeSignalExecution
	// CrossClusterTaskTypeRecordChildWorkflowExeuctionComplete is an option for CrossClusterTaskType
	CrossClusterTaskTypeRecordChildWorkflowExeuctionComplete
	// CrossClusterTaskTypeApplyParentPolicy is an option for CrossClusterTaskType
	CrossClusterTaskTypeApplyParentPolicy
)

// CrossClusterTaskFailedCause is an internal type (TBD...)
type CrossClusterTaskFailedCause int32

// Ptr is a helper function for getting pointer value
func (e CrossClusterTaskFailedCause) Ptr() *CrossClusterTaskFailedCause {
	return &e
}

// String returns a readable string representation of CrossClusterTaskFailedCause.
func (e CrossClusterTaskFailedCause) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "DOMAIN_NOT_ACTIVE"
	case 1:
		return "DOMAIN_NOT_EXISTS"
	case 2:
		return "WORKFLOW_ALREADY_RUNNING"
	case 3:
		return "WORKFLOW_NOT_EXISTS"
	case 4:
		return "WORKFLOW_ALREADY_COMPLETED"
	case 5:
		return "UNCATEGORIZED"
	}
	return fmt.Sprintf("CrossClusterTaskFailedCause(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *CrossClusterTaskFailedCause) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "DOMAIN_NOT_ACTIVE":
		*e = CrossClusterTaskFailedCauseDomainNotActive
		return nil
	case "DOMAIN_NOT_EXISTS":
		*e = CrossClusterTaskFailedCauseDomainNotExists
		return nil
	case "WORKFLOW_ALREADY_RUNNING":
		*e = CrossClusterTaskFailedCauseWorkflowAlreadyRunning
		return nil
	case "WORKFLOW_NOT_EXISTS":
		*e = CrossClusterTaskFailedCauseWorkflowNotExists
		return nil
	case "WORKFLOW_ALREADY_COMPLETED":
		*e = CrossClusterTaskFailedCauseWorkflowAlreadyCompleted
		return nil
	case "UNCATEGORIZED":
		*e = CrossClusterTaskFailedCauseUncategorized
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "CrossClusterTaskFailedCause", err)
		}
		*e = CrossClusterTaskFailedCause(val)
		return nil
	}
}

// MarshalText encodes CrossClusterTaskFailedCause to text.
func (e CrossClusterTaskFailedCause) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// CrossClusterTaskFailedCauseDomainNotActive is an option for CrossClusterTaskFailedCause
	CrossClusterTaskFailedCauseDomainNotActive CrossClusterTaskFailedCause = iota
	// CrossClusterTaskFailedCauseDomainNotExists is an option for CrossClusterTaskFailedCause
	CrossClusterTaskFailedCauseDomainNotExists
	// CrossClusterTaskFailedCauseWorkflowAlreadyRunning is an option for CrossClusterTaskFailedCause
	CrossClusterTaskFailedCauseWorkflowAlreadyRunning
	// CrossClusterTaskFailedCauseWorkflowNotExists is an option for CrossClusterTaskFailedCause
	CrossClusterTaskFailedCauseWorkflowNotExists
	// CrossClusterTaskFailedCauseWorkflowAlreadyCompleted is an option for CrossClusterTaskFailedCause
	CrossClusterTaskFailedCauseWorkflowAlreadyCompleted
	// CrossClusterTaskFailedCauseUncategorized is an option for CrossClusterTaskFailedCause
	CrossClusterTaskFailedCauseUncategorized
)

// GetTaskFailedCause is an internal type (TBD...)
type GetTaskFailedCause int32

// Ptr is a helper function for getting pointer value
func (e GetTaskFailedCause) Ptr() *GetTaskFailedCause {
	return &e
}

// String returns a readable string representation of GetCrossClusterTaskFailedCause.
func (e GetTaskFailedCause) String() string {
	w := int32(e)
	switch w {
	case 0:
		return "SERVICE_BUSY"
	case 1:
		return "TIMEOUT"
	case 2:
		return "SHARD_OWNERSHIP_LOST"
	case 3:
		return "UNCATEGORIZED"
	}
	return fmt.Sprintf("GetCrossClusterTaskFailedCause(%d)", w)
}

// UnmarshalText parses enum value from string representation
func (e *GetTaskFailedCause) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "SERVICE_BUSY":
		*e = GetTaskFailedCauseServiceBusy
		return nil
	case "TIMEOUT":
		*e = GetTaskFailedCauseTimeout
		return nil
	case "SHARD_OWNERSHIP_LOST":
		*e = GetTaskFailedCauseShardOwnershipLost
		return nil
	case "UNCATEGORIZED":
		*e = GetTaskFailedCauseUncategorized
		return nil
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "GetCrossClusterTaskFailedCause", err)
		}
		*e = GetTaskFailedCause(val)
		return nil
	}
}

// MarshalText encodes GetCrossClusterTaskFailedCause to text.
func (e GetTaskFailedCause) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

const (
	// GetTaskFailedCauseServiceBusy is an option for GetCrossClusterTaskFailedCause
	GetTaskFailedCauseServiceBusy GetTaskFailedCause = iota
	// GetTaskFailedCauseTimeout is an option for GetCrossClusterTaskFailedCause
	GetTaskFailedCauseTimeout
	// GetTaskFailedCauseShardOwnershipLost is an option for GetCrossClusterTaskFailedCause
	GetTaskFailedCauseShardOwnershipLost
	// GetTaskFailedCauseUncategorized is an option for GetCrossClusterTaskFailedCause
	GetTaskFailedCauseUncategorized
)

// CrossClusterTaskInfo is an internal type (TBD...)
type CrossClusterTaskInfo struct {
	DomainID            string                `json:"domainID,omitempty"`
	WorkflowID          string                `json:"workflowID,omitempty"`
	RunID               string                `json:"runID,omitempty"`
	TaskType            *CrossClusterTaskType `json:"taskType,omitempty"`
	TaskState           int16                 `json:"taskState,omitempty"`
	TaskID              int64                 `json:"taskID,omitempty"`
	VisibilityTimestamp *int64                `json:"visibilityTimestamp,omitempty"`
}

// GetTaskType is an internal getter (TBD...)
func (v *CrossClusterTaskInfo) GetTaskType() (o CrossClusterTaskType) {
	if v != nil && v.TaskType != nil {
		return *v.TaskType
	}
	return
}

// GetTaskID is an internal getter (TBD...)
func (v *CrossClusterTaskInfo) GetTaskID() (o int64) {
	if v != nil {
		return v.TaskID
	}
	return
}

// GetVisibilityTimestamp is an internal getter (TBD...)
func (v *CrossClusterTaskInfo) GetVisibilityTimestamp() (o int64) {
	if v != nil && v.VisibilityTimestamp != nil {
		return *v.VisibilityTimestamp
	}
	return
}

// CrossClusterStartChildExecutionRequestAttributes is an internal type (TBD...)
type CrossClusterStartChildExecutionRequestAttributes struct {
	TargetDomainID           string                                               `json:"targetDomainID,omitempty"`
	RequestID                string                                               `json:"requestID,omitempty"`
	InitiatedEventID         int64                                                `json:"initiatedEventID,omitempty"`
	InitiatedEventAttributes *StartChildWorkflowExecutionInitiatedEventAttributes `json:"initiatedEventAttributes,omitempty"`
	TargetRunID              *string                                              `json:"targetRunID,omitempty"`
	PartitionConfig          map[string]string
}

// GetRequestID is an internal getter (TBD...)
func (v *CrossClusterStartChildExecutionRequestAttributes) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// GetInitiatedEventAttributes is an internal getter (TBD...)
func (v *CrossClusterStartChildExecutionRequestAttributes) GetInitiatedEventAttributes() (o *StartChildWorkflowExecutionInitiatedEventAttributes) {
	if v != nil && v.InitiatedEventAttributes != nil {
		return v.InitiatedEventAttributes
	}
	return
}

// GetTargetRunID is an internal getter (TBD...)
func (v *CrossClusterStartChildExecutionRequestAttributes) GetTargetRunID() (o string) {
	if v != nil && v.TargetRunID != nil {
		return *v.TargetRunID
	}
	return
}

// GetTargetRunID is an internal getter (TBD...)
func (v *CrossClusterStartChildExecutionRequestAttributes) GetPartitionConfig() (o map[string]string) {
	if v != nil && v.PartitionConfig != nil {
		return v.PartitionConfig
	}
	return
}

// CrossClusterStartChildExecutionResponseAttributes is an internal type (TBD...)
type CrossClusterStartChildExecutionResponseAttributes struct {
	RunID string `json:"runID,omitempty"`
}

// GetRunID is an internal getter (TBD...)
func (v *CrossClusterStartChildExecutionResponseAttributes) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// CrossClusterCancelExecutionRequestAttributes is an internal type (TBD...)
type CrossClusterCancelExecutionRequestAttributes struct {
	TargetDomainID    string `json:"targetDomainID,omitempty"`
	TargetWorkflowID  string `json:"targetWorkflowID,omitempty"`
	TargetRunID       string `json:"targetRunID,omitempty"`
	RequestID         string `json:"requestID,omitempty"`
	InitiatedEventID  int64  `json:"initiatedEventID,omitempty"`
	ChildWorkflowOnly bool   `json:"childWorkflowOnly,omitempty"`
}

// CrossClusterCancelExecutionResponseAttributes is an internal type (TBD...)
type CrossClusterCancelExecutionResponseAttributes struct {
}

// CrossClusterSignalExecutionRequestAttributes is an internal type (TBD...)
type CrossClusterSignalExecutionRequestAttributes struct {
	TargetDomainID    string `json:"targetDomainID,omitempty"`
	TargetWorkflowID  string `json:"targetWorkflowID,omitempty"`
	TargetRunID       string `json:"targetRunID,omitempty"`
	RequestID         string `json:"requestID,omitempty"`
	InitiatedEventID  int64  `json:"initiatedEventID,omitempty"`
	ChildWorkflowOnly bool   `json:"childWorkflowOnly,omitempty"`
	SignalName        string `json:"signalName,omitempty"`
	SignalInput       []byte `json:"signalInput,omitempty"`
	Control           []byte `json:"control,omitempty"`
}

// CrossClusterSignalExecutionResponseAttributes is an internal type (TBD...)
type CrossClusterSignalExecutionResponseAttributes struct {
}

type CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes struct {
	TargetDomainID   string        `json:"targetDomainID,omitempty"`
	TargetWorkflowID string        `json:"targetWorkflowID,omitempty"`
	TargetRunID      string        `json:"targetRunID,omitempty"`
	InitiatedEventID int64         `json:"initiatedEventID,omitempty"`
	CompletionEvent  *HistoryEvent `json:"completionEvent,omitempty"`
}

// CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes is an internal type (TBD...)
type CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes struct {
}

type ApplyParentClosePolicyStatus struct {
	Completed   bool                         `json:"completed,omitempty"`
	FailedCause *CrossClusterTaskFailedCause `json:"failedCause,omitempty"`
}

type ApplyParentClosePolicyAttributes struct {
	ChildDomainID     string             `json:"ChildDomainID,omitempty"`
	ChildWorkflowID   string             `json:"ChildWorkflowID,omitempty"`
	ChildRunID        string             `json:"ChildRunID,omitempty"`
	ParentClosePolicy *ParentClosePolicy `json:"parentClosePolicy,omitempty"`
}

// GetParentClosePolicy is an internal getter (TBD...)
func (v *ApplyParentClosePolicyAttributes) GetParentClosePolicy() (o *ParentClosePolicy) {
	if v != nil {
		return v.ParentClosePolicy
	}
	return
}

type ApplyParentClosePolicyRequest struct {
	Child  *ApplyParentClosePolicyAttributes `json:"child,omitempty"`
	Status *ApplyParentClosePolicyStatus     `json:"status,omitempty"`
}

type CrossClusterApplyParentClosePolicyRequestAttributes struct {
	Children []*ApplyParentClosePolicyRequest `json:"children,omitempty"`
}

type ApplyParentClosePolicyResult struct {
	Child       *ApplyParentClosePolicyAttributes `json:"child,omitempty"`
	FailedCause *CrossClusterTaskFailedCause      `json:"failedCause,omitempty"`
}

// CrossClusterApplyParentClosePolicyResponseAttributes is an internal type (TBD...)
type CrossClusterApplyParentClosePolicyResponseAttributes struct {
	ChildrenStatus []*ApplyParentClosePolicyResult `json:"childrenStatus,omitempty"`
}

// CrossClusterTaskRequest is an internal type (TBD...)
type CrossClusterTaskRequest struct {
	TaskInfo                                       *CrossClusterTaskInfo                                              `json:"taskInfo,omitempty"`
	StartChildExecutionAttributes                  *CrossClusterStartChildExecutionRequestAttributes                  `json:"startChildExecutionAttributes,omitempty"`
	CancelExecutionAttributes                      *CrossClusterCancelExecutionRequestAttributes                      `json:"cancelExecutionAttributes,omitempty"`
	SignalExecutionAttributes                      *CrossClusterSignalExecutionRequestAttributes                      `json:"signalExecutionAttributes,omitempty"`
	RecordChildWorkflowExecutionCompleteAttributes *CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes `json:"RecordChildWorkflowExecutionCompleteAttributes,omitempty"`
	ApplyParentClosePolicyAttributes               *CrossClusterApplyParentClosePolicyRequestAttributes               `json:"ApplyParentClosePolicyAttributes,omitempty"`
}

// CrossClusterTaskResponse is an internal type (TBD...)
type CrossClusterTaskResponse struct {
	TaskID                                         int64                                                               `json:"taskID,omitempty"`
	TaskType                                       *CrossClusterTaskType                                               `json:"taskType,omitempty"`
	TaskState                                      int16                                                               `json:"taskState,omitempty"`
	FailedCause                                    *CrossClusterTaskFailedCause                                        `json:"failedCause,omitempty"`
	StartChildExecutionAttributes                  *CrossClusterStartChildExecutionResponseAttributes                  `json:"startChildExecutionAttributes,omitempty"`
	CancelExecutionAttributes                      *CrossClusterCancelExecutionResponseAttributes                      `json:"cancelExecutionAttributes,omitempty"`
	SignalExecutionAttributes                      *CrossClusterSignalExecutionResponseAttributes                      `json:"signalExecutionAttributes,omitempty"`
	RecordChildWorkflowExecutionCompleteAttributes *CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes `json:"RecordChildWorkflowExecutionCompleteAttributes,omitempty"`
	ApplyParentClosePolicyAttributes               *CrossClusterApplyParentClosePolicyResponseAttributes               `json:"ApplyParentClosePolicyAttributes,omitempty"`
}

// GetTaskID is an internal getter (TBD...)
func (v *CrossClusterTaskResponse) GetTaskID() (o int64) {
	if v != nil {
		return v.TaskID
	}
	return
}

// GetTaskType is an internal getter (TBD...)
func (v *CrossClusterTaskResponse) GetTaskType() (o CrossClusterTaskType) {
	if v != nil && v.TaskType != nil {
		return *v.TaskType
	}
	return
}

// GetFailedCause is an internal getter (TBD...)
func (v *CrossClusterTaskResponse) GetFailedCause() (o CrossClusterTaskFailedCause) {
	if v != nil && v.FailedCause != nil {
		return *v.FailedCause
	}
	return
}

// GetCrossClusterTasksRequest is an internal type (TBD...)
type GetCrossClusterTasksRequest struct {
	ShardIDs      []int32 `json:"shardIDs,omitempty"`
	TargetCluster string  `json:"targetCluster,omitempty"`
}

// GetShardIDs is an internal getter (TBD...)
func (v *GetCrossClusterTasksRequest) GetShardIDs() (o []int32) {
	if v != nil && v.ShardIDs != nil {
		return v.ShardIDs
	}
	return
}

// GetTargetCluster is an internal getter (TBD...)
func (v *GetCrossClusterTasksRequest) GetTargetCluster() (o string) {
	if v != nil {
		return v.TargetCluster
	}
	return
}

// GetCrossClusterTasksResponse is an internal type (TBD...)
type GetCrossClusterTasksResponse struct {
	TasksByShard       map[int32][]*CrossClusterTaskRequest `json:"tasksByShard,omitempty"`
	FailedCauseByShard map[int32]GetTaskFailedCause         `json:"failedCauseByShard,omitempty"`
}

// GetTasksByShard is an internal getter (TBD...)
func (v *GetCrossClusterTasksResponse) GetTasksByShard() (o map[int32][]*CrossClusterTaskRequest) {
	if v != nil && v.TasksByShard != nil {
		return v.TasksByShard
	}
	return
}

// RespondCrossClusterTasksCompletedRequest is an internal type (TBD...)
type RespondCrossClusterTasksCompletedRequest struct {
	ShardID       int32                       `json:"shardID,omitempty"`
	TargetCluster string                      `json:"targetCluster,omitempty"`
	TaskResponses []*CrossClusterTaskResponse `json:"taskResponses,omitempty"`
	FetchNewTasks bool                        `json:"fetchNewTasks,omitempty"`
}

// GetShardID is an internal getter (TBD...)
func (v *RespondCrossClusterTasksCompletedRequest) GetShardID() (o int32) {
	if v != nil {
		return v.ShardID
	}
	return
}

// GetFetchNewTasks is an internal getter (TBD...)
func (v *RespondCrossClusterTasksCompletedRequest) GetFetchNewTasks() (o bool) {
	if v != nil {
		return v.FetchNewTasks
	}
	return
}

// RespondCrossClusterTasksCompletedResponse is an internal type (TBD...)
type RespondCrossClusterTasksCompletedResponse struct {
	Tasks []*CrossClusterTaskRequest `json:"tasks,omitempty"`
}

// StickyWorkerUnavailableError is an internal type (TBD...)
type StickyWorkerUnavailableError struct {
	Message string `json:"message,required"`
}

// Any is an internal mirror of google.protobuf.Any, serving the same purposes, but
// intentionally breaking direct compatibility because it may hold data that is not
// actually protobuf encoded.
//
// All uses of Any must either:
//   - check that ValueType is a recognized type, and deserialize based on its contents
//   - or just pass it along as an opaque type for something else to use
//
// Contents are intentionally undefined to allow external definitions, e.g. from
// third-party plugins that are not part of this source repository.
type Any struct {
	// ValueType describes the type of encoded data in Value.
	//
	// No structure or allowed values are defined, but you are strongly encouraged
	// to use hard-coded strings (or unambiguous prefixes) or URLs when possible.
	//
	// For more concise encoding of exclusively known types, use e.g. DataBlob instead.
	ValueType string `json:"value_type"`
	// Value holds arbitrary bytes, and is described by ValueType.
	//
	// To interpret, you MUST check ValueType.
	Value []byte `json:"value"`
}

// AutoConfigHint is an internal type (TBD...)
type AutoConfigHint struct {
	EnableAutoConfig   bool  `json:"enableAutoConfig"`
	PollerWaitTimeInMs int64 `json:"pollerWaitTimeInMs"`
}
