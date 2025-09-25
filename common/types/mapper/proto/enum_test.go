// Copyright (c) 2021 Uber Technologies Inc.
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

package proto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"

	sharedv1 "github.com/uber/cadence/.gen/proto/shared/v1"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const UnknownValue = 9999

func TestTaskSource(t *testing.T) {
	for _, item := range []*types.TaskSource{
		nil,
		types.TaskSourceHistory.Ptr(),
		types.TaskSourceDbBacklog.Ptr(),
	} {
		assert.Equal(t, item, ToTaskSource(FromTaskSource(item)))
	}
}

func TestToTaskSource(t *testing.T) {
	cases := []struct {
		name     string
		input    sharedv1.TaskSource
		expected *types.TaskSource
	}{
		{
			name:  "when input is invalid it should return nil",
			input: sharedv1.TaskSource_TASK_SOURCE_INVALID,
		},
		{
			name:  "when input is out of range it should return nil",
			input: UnknownValue,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    sharedv1.TaskSource_TASK_SOURCE_HISTORY,
			expected: types.TaskSourceHistory.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToTaskSource(tc.input))
		})
	}
}

func TestFromTaskSource(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.TaskSource
		expected sharedv1.TaskSource
	}{
		{
			name:  "when input is nil it should return INVALID",
			input: nil,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    types.TaskSourceHistory.Ptr(),
			expected: sharedv1.TaskSource_TASK_SOURCE_HISTORY,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.TaskSource(UnknownValue).Ptr(),
			expected: sharedv1.TaskSource_TASK_SOURCE_INVALID,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromTaskSource(tc.input))
		})
	}
}

func TestDLQType(t *testing.T) {
	for _, item := range []*types.DLQType{
		nil,
		types.DLQTypeReplication.Ptr(),
		types.DLQTypeDomain.Ptr(),
	} {
		assert.Equal(t, item, ToDLQType(FromDLQType(item)))
	}
}

func TestToDLQType(t *testing.T) {
	cases := []struct {
		name     string
		input    adminv1.DLQType
		expected *types.DLQType
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    adminv1.DLQType_DLQ_TYPE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    UnknownValue,
			expected: nil,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    adminv1.DLQType_DLQ_TYPE_REPLICATION,
			expected: types.DLQTypeReplication.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToDLQType(tc.input))
		})
	}
}

func TestFromDLQType(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.DLQType
		expected adminv1.DLQType
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: adminv1.DLQType_DLQ_TYPE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.DLQType(UnknownValue).Ptr(),
			expected: adminv1.DLQType_DLQ_TYPE_INVALID,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    types.DLQTypeReplication.Ptr(),
			expected: adminv1.DLQType_DLQ_TYPE_REPLICATION,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromDLQType(tc.input))
		})
	}
}

func TestDomainOperation(t *testing.T) {
	for _, item := range []*types.DomainOperation{
		nil,
		types.DomainOperationCreate.Ptr(),
		types.DomainOperationUpdate.Ptr(),
		types.DomainOperationDelete.Ptr(),
	} {
		assert.Equal(t, item, ToDomainOperation(FromDomainOperation(item)))
	}
}

func TestToDomainOperation(t *testing.T) {
	cases := []struct {
		name     string
		input    adminv1.DomainOperation
		expected *types.DomainOperation
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    adminv1.DomainOperation_DOMAIN_OPERATION_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    UnknownValue,
			expected: nil,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    adminv1.DomainOperation_DOMAIN_OPERATION_CREATE,
			expected: types.DomainOperationCreate.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToDomainOperation(tc.input))
		})
	}
}

func TestFromDomainOperation(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.DomainOperation
		expected adminv1.DomainOperation
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: adminv1.DomainOperation_DOMAIN_OPERATION_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.DomainOperation(UnknownValue).Ptr(),
			expected: adminv1.DomainOperation_DOMAIN_OPERATION_INVALID,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    types.DomainOperationCreate.Ptr(),
			expected: adminv1.DomainOperation_DOMAIN_OPERATION_CREATE,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromDomainOperation(tc.input))
		})
	}
}

func TestReplicationTaskType(t *testing.T) {
	for _, item := range []*types.ReplicationTaskType{
		nil,
		types.ReplicationTaskTypeDomain.Ptr(),
		types.ReplicationTaskTypeHistory.Ptr(),
		types.ReplicationTaskTypeSyncShardStatus.Ptr(),
		types.ReplicationTaskTypeSyncActivity.Ptr(),
		types.ReplicationTaskTypeHistoryMetadata.Ptr(),
		types.ReplicationTaskTypeHistoryV2.Ptr(),
		types.ReplicationTaskTypeFailoverMarker.Ptr(),
	} {
		assert.Equal(t, item, ToReplicationTaskType(FromReplicationTaskType(item)))
	}
}

func TestToReplicationTaskType(t *testing.T) {
	cases := []struct {
		name     string
		input    adminv1.ReplicationTaskType
		expected *types.ReplicationTaskType
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    adminv1.ReplicationTaskType(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_DOMAIN,
			expected: types.ReplicationTaskTypeDomain.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToReplicationTaskType(tc.input))
		})
	}
}

func TestFromReplicationTaskType(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.ReplicationTaskType
		expected adminv1.ReplicationTaskType
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.ReplicationTaskType(UnknownValue).Ptr(),
			expected: adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_INVALID,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    types.ReplicationTaskTypeDomain.Ptr(),
			expected: adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_DOMAIN,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromReplicationTaskType(tc.input))
		})
	}
}
func TestArchivalStatus(t *testing.T) {
	for _, item := range []*types.ArchivalStatus{
		nil,
		types.ArchivalStatusDisabled.Ptr(),
		types.ArchivalStatusEnabled.Ptr(),
	} {
		assert.Equal(t, item, ToArchivalStatus(FromArchivalStatus(item)))
	}
}

func TestToArchivalStatus(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.ArchivalStatus
		expected *types.ArchivalStatus
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.ArchivalStatus_ARCHIVAL_STATUS_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.ArchivalStatus(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is disabled it should return the correctly mapped value",
			input:    apiv1.ArchivalStatus_ARCHIVAL_STATUS_DISABLED,
			expected: types.ArchivalStatusDisabled.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToArchivalStatus(tc.input))
		})
	}
}

func TestFromArchivalStatus(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.ArchivalStatus
		expected apiv1.ArchivalStatus
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.ArchivalStatus_ARCHIVAL_STATUS_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.ArchivalStatus(UnknownValue).Ptr(),
			expected: apiv1.ArchivalStatus_ARCHIVAL_STATUS_INVALID,
		},
		{
			name:     "when input is disabled it should return the correctly mapped value",
			input:    types.ArchivalStatusDisabled.Ptr(),
			expected: apiv1.ArchivalStatus_ARCHIVAL_STATUS_DISABLED,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromArchivalStatus(tc.input))
		})
	}
}
func TestCancelExternalWorkflowExecutionFailedCause(t *testing.T) {
	for _, item := range []*types.CancelExternalWorkflowExecutionFailedCause{
		nil,
		types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
		types.CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted.Ptr(),
	} {
		assert.Equal(t, item, ToCancelExternalWorkflowExecutionFailedCause(FromCancelExternalWorkflowExecutionFailedCause(item)))
	}
}

func TestToCancelExternalWorkflowExecutionFailedCause(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.CancelExternalWorkflowExecutionFailedCause
		expected *types.CancelExternalWorkflowExecutionFailedCause
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.CancelExternalWorkflowExecutionFailedCause(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is unknown external workflow execution it should return the correctly mapped value",
			input:    apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION,
			expected: types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToCancelExternalWorkflowExecutionFailedCause(tc.input))
		})
	}
}

func TestFromCancelExternalWorkflowExecutionFailedCause(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.CancelExternalWorkflowExecutionFailedCause
		expected apiv1.CancelExternalWorkflowExecutionFailedCause
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.CancelExternalWorkflowExecutionFailedCause(UnknownValue).Ptr(),
			expected: apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID,
		},
		{
			name:     "when input is unknown external workflow execution it should return the correctly mapped value",
			input:    types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
			expected: apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromCancelExternalWorkflowExecutionFailedCause(tc.input))
		})
	}
}
func TestChildWorkflowExecutionFailedCause(t *testing.T) {
	for _, item := range []*types.ChildWorkflowExecutionFailedCause{
		nil,
		types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning.Ptr(),
	} {
		assert.Equal(t, item, ToChildWorkflowExecutionFailedCause(FromChildWorkflowExecutionFailedCause(item)))
	}
}

func TestToChildWorkflowExecutionFailedCause(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.ChildWorkflowExecutionFailedCause
		expected *types.ChildWorkflowExecutionFailedCause
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.ChildWorkflowExecutionFailedCause_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.ChildWorkflowExecutionFailedCause(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is workflow already running it should return the correctly mapped value",
			input:    apiv1.ChildWorkflowExecutionFailedCause_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_WORKFLOW_ALREADY_RUNNING,
			expected: types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToChildWorkflowExecutionFailedCause(tc.input))
		})
	}
}

func TestFromChildWorkflowExecutionFailedCause(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.ChildWorkflowExecutionFailedCause
		expected apiv1.ChildWorkflowExecutionFailedCause
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.ChildWorkflowExecutionFailedCause_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.ChildWorkflowExecutionFailedCause(UnknownValue).Ptr(),
			expected: apiv1.ChildWorkflowExecutionFailedCause_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID,
		},
		{
			name:     "when input is workflow already running it should return the correctly mapped value",
			input:    types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning.Ptr(),
			expected: apiv1.ChildWorkflowExecutionFailedCause_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_WORKFLOW_ALREADY_RUNNING,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromChildWorkflowExecutionFailedCause(tc.input))
		})
	}
}
func TestContinueAsNewInitiator(t *testing.T) {
	for _, item := range []*types.ContinueAsNewInitiator{
		nil,
		types.ContinueAsNewInitiatorDecider.Ptr(),
		types.ContinueAsNewInitiatorRetryPolicy.Ptr(),
		types.ContinueAsNewInitiatorCronSchedule.Ptr(),
	} {
		assert.Equal(t, item, ToContinueAsNewInitiator(FromContinueAsNewInitiator(item)))
	}
}

func TestToContinueAsNewInitiator(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.ContinueAsNewInitiator
		expected *types.ContinueAsNewInitiator
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.ContinueAsNewInitiator(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is decider it should return the correctly mapped value",
			input:    apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_DECIDER,
			expected: types.ContinueAsNewInitiatorDecider.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToContinueAsNewInitiator(tc.input))
		})
	}
}

func TestFromContinueAsNewInitiator(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.ContinueAsNewInitiator
		expected apiv1.ContinueAsNewInitiator
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.ContinueAsNewInitiator(UnknownValue).Ptr(),
			expected: apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_INVALID,
		},
		{
			name:     "when input is decider it should return the correctly mapped value",
			input:    types.ContinueAsNewInitiatorDecider.Ptr(),
			expected: apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_DECIDER,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromContinueAsNewInitiator(tc.input))
		})
	}
}
func TestCrossClusterTaskFailedCause(t *testing.T) {
	for _, item := range []*types.CrossClusterTaskFailedCause{
		nil,
		types.CrossClusterTaskFailedCauseDomainNotActive.Ptr(),
		types.CrossClusterTaskFailedCauseDomainNotExists.Ptr(),
		types.CrossClusterTaskFailedCauseWorkflowAlreadyRunning.Ptr(),
		types.CrossClusterTaskFailedCauseWorkflowNotExists.Ptr(),
		types.CrossClusterTaskFailedCauseWorkflowAlreadyCompleted.Ptr(),
		types.CrossClusterTaskFailedCauseUncategorized.Ptr(),
	} {
		assert.Equal(t, item, ToCrossClusterTaskFailedCause(FromCrossClusterTaskFailedCause(item)))
	}
}

func TestToCrossClusterTaskFailedCause(t *testing.T) {
	cases := []struct {
		name     string
		input    adminv1.CrossClusterTaskFailedCause
		expected *types.CrossClusterTaskFailedCause
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    adminv1.CrossClusterTaskFailedCause(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is domain not active it should return the correctly mapped value",
			input:    adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_DOMAIN_NOT_ACTIVE,
			expected: types.CrossClusterTaskFailedCauseDomainNotActive.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToCrossClusterTaskFailedCause(tc.input))
		})
	}
}

func TestFromCrossClusterTaskFailedCause(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.CrossClusterTaskFailedCause
		expected adminv1.CrossClusterTaskFailedCause
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.CrossClusterTaskFailedCause(UnknownValue).Ptr(),
			expected: adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_INVALID,
		},
		{
			name:     "when input is domain not active it should return the correctly mapped value",
			input:    types.CrossClusterTaskFailedCauseDomainNotActive.Ptr(),
			expected: adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_DOMAIN_NOT_ACTIVE,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromCrossClusterTaskFailedCause(tc.input))
		})
	}
}
func TestDecisionTaskFailedCause(t *testing.T) {
	for _, item := range []*types.DecisionTaskFailedCause{
		nil,
		types.DecisionTaskFailedCauseUnhandledDecision.Ptr(),
		types.DecisionTaskFailedCauseBadScheduleActivityAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadRequestCancelActivityAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadStartTimerAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadCancelTimerAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadRecordMarkerAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadContinueAsNewAttributes.Ptr(),
		types.DecisionTaskFailedCauseStartTimerDuplicateID.Ptr(),
		types.DecisionTaskFailedCauseResetStickyTasklist.Ptr(),
		types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure.Ptr(),
		types.DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadStartChildExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseForceCloseDecision.Ptr(),
		types.DecisionTaskFailedCauseFailoverCloseDecision.Ptr(),
		types.DecisionTaskFailedCauseBadSignalInputSize.Ptr(),
		types.DecisionTaskFailedCauseResetWorkflow.Ptr(),
		types.DecisionTaskFailedCauseBadBinary.Ptr(),
		types.DecisionTaskFailedCauseScheduleActivityDuplicateID.Ptr(),
		types.DecisionTaskFailedCauseBadSearchAttributes.Ptr(),
	} {
		assert.Equal(t, item, ToDecisionTaskFailedCause(FromDecisionTaskFailedCause(item)))
	}
}

func TestToDecisionTaskFailedCause(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.DecisionTaskFailedCause
		expected *types.DecisionTaskFailedCause
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.DecisionTaskFailedCause(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is unhandled decision it should return the correctly mapped value",
			input:    apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_UNHANDLED_DECISION,
			expected: types.DecisionTaskFailedCauseUnhandledDecision.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToDecisionTaskFailedCause(tc.input))
		})
	}
}

func TestFromDecisionTaskFailedCause(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.DecisionTaskFailedCause
		expected apiv1.DecisionTaskFailedCause
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.DecisionTaskFailedCause(UnknownValue).Ptr(),
			expected: apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_INVALID,
		},
		{
			name:     "when input is unhandled decision it should return the correctly mapped value",
			input:    types.DecisionTaskFailedCauseUnhandledDecision.Ptr(),
			expected: apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_UNHANDLED_DECISION,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromDecisionTaskFailedCause(tc.input))
		})
	}
}
func TestDomainStatus(t *testing.T) {
	for _, item := range []*types.DomainStatus{
		nil,
		types.DomainStatusRegistered.Ptr(),
		types.DomainStatusDeprecated.Ptr(),
		types.DomainStatusDeleted.Ptr(),
	} {
		assert.Equal(t, item, ToDomainStatus(FromDomainStatus(item)))
	}
}

func TestToDomainStatus(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.DomainStatus
		expected *types.DomainStatus
	}{
		{
			name:  "when input is invalid it should return nil",
			input: apiv1.DomainStatus_DOMAIN_STATUS_INVALID,
		},
		{
			name:  "when input is out of range it should return nil",
			input: apiv1.DomainStatus(UnknownValue),
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    apiv1.DomainStatus_DOMAIN_STATUS_REGISTERED,
			expected: types.DomainStatusRegistered.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToDomainStatus(tc.input))
		})
	}
}

func TestFromDomainStatus(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.DomainStatus
		expected apiv1.DomainStatus
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.DomainStatus_DOMAIN_STATUS_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.DomainStatus(UnknownValue).Ptr(),
			expected: apiv1.DomainStatus_DOMAIN_STATUS_INVALID,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    types.DomainStatusRegistered.Ptr(),
			expected: apiv1.DomainStatus_DOMAIN_STATUS_REGISTERED,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromDomainStatus(tc.input))
		})
	}
}
func TestEncodingType(t *testing.T) {
	for _, item := range []*types.EncodingType{
		nil,
		types.EncodingTypeThriftRW.Ptr(),
		types.EncodingTypeJSON.Ptr(),
	} {
		assert.Equal(t, item, ToEncodingType(FromEncodingType(item)))
	}
}

func TestToEncodingType(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.EncodingType
		expected *types.EncodingType
	}{
		{
			name:  "when input is invalid it should return nil",
			input: apiv1.EncodingType_ENCODING_TYPE_INVALID,
		},
		{
			name:  "when input is out of range it should return nil",
			input: apiv1.EncodingType(UnknownValue),
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    apiv1.EncodingType_ENCODING_TYPE_THRIFTRW,
			expected: types.EncodingTypeThriftRW.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToEncodingType(tc.input))
		})
	}
}

func TestFromEncodingType(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.EncodingType
		expected apiv1.EncodingType
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.EncodingType_ENCODING_TYPE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.EncodingType(UnknownValue).Ptr(),
			expected: apiv1.EncodingType_ENCODING_TYPE_INVALID,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    types.EncodingTypeThriftRW.Ptr(),
			expected: apiv1.EncodingType_ENCODING_TYPE_THRIFTRW,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromEncodingType(tc.input))
		})
	}
}
func TestEventFilterType(t *testing.T) {
	for _, item := range []*types.HistoryEventFilterType{
		nil,
		types.HistoryEventFilterTypeAllEvent.Ptr(),
		types.HistoryEventFilterTypeCloseEvent.Ptr(),
	} {
		assert.Equal(t, item, ToEventFilterType(FromEventFilterType(item)))
	}
}

func TestToEventFilterType(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.EventFilterType
		expected *types.HistoryEventFilterType
	}{
		{
			name:  "when input is invalid it should return nil",
			input: apiv1.EventFilterType_EVENT_FILTER_TYPE_INVALID,
		},
		{
			name:  "when input is out of range it should return nil",
			input: apiv1.EventFilterType(UnknownValue),
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    apiv1.EventFilterType_EVENT_FILTER_TYPE_ALL_EVENT,
			expected: types.HistoryEventFilterTypeAllEvent.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToEventFilterType(tc.input))
		})
	}
}

func TestFromEventFilterType(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.HistoryEventFilterType
		expected apiv1.EventFilterType
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.EventFilterType_EVENT_FILTER_TYPE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.HistoryEventFilterType(UnknownValue).Ptr(),
			expected: apiv1.EventFilterType_EVENT_FILTER_TYPE_INVALID,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    types.HistoryEventFilterTypeAllEvent.Ptr(),
			expected: apiv1.EventFilterType_EVENT_FILTER_TYPE_ALL_EVENT,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromEventFilterType(tc.input))
		})
	}
}
func TestIndexedValueType(t *testing.T) {
	for _, item := range []types.IndexedValueType{
		types.IndexedValueTypeString,
		types.IndexedValueTypeKeyword,
		types.IndexedValueTypeInt,
		types.IndexedValueTypeDouble,
		types.IndexedValueTypeBool,
		types.IndexedValueTypeDatetime,
	} {
		assert.Equal(t, item, ToIndexedValueType(FromIndexedValueType(item)))
	}
}

func TestToIndexedValueType(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.IndexedValueType
		expected types.IndexedValueType
	}{
		{
			name:     "when input is invalid it should return string",
			input:    apiv1.IndexedValueType_INDEXED_VALUE_TYPE_INVALID,
			expected: types.IndexedValueTypeString,
		},
		{
			name:     "when input is out of range it should return string",
			input:    apiv1.IndexedValueType(UnknownValue),
			expected: types.IndexedValueTypeString,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    apiv1.IndexedValueType_INDEXED_VALUE_TYPE_STRING,
			expected: types.IndexedValueTypeString,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToIndexedValueType(tc.input))
		})
	}
}

func TestFromIndexedValueType(t *testing.T) {
	cases := []struct {
		name     string
		input    types.IndexedValueType
		expected apiv1.IndexedValueType
	}{
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.IndexedValueType(UnknownValue),
			expected: apiv1.IndexedValueType_INDEXED_VALUE_TYPE_INVALID,
		},
		{
			name:     "when input is valid it should return the correctly mapped value",
			input:    types.IndexedValueTypeString,
			expected: apiv1.IndexedValueType_INDEXED_VALUE_TYPE_STRING,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromIndexedValueType(tc.input))
		})
	}
}
func TestParentClosePolicy(t *testing.T) {
	for _, item := range []*types.ParentClosePolicy{
		nil,
		types.ParentClosePolicyAbandon.Ptr(),
		types.ParentClosePolicyRequestCancel.Ptr(),
		types.ParentClosePolicyTerminate.Ptr(),
	} {
		assert.Equal(t, item, ToParentClosePolicy(FromParentClosePolicy(item)))
	}
}

func TestToParentClosePolicy(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.ParentClosePolicy
		expected *types.ParentClosePolicy
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.ParentClosePolicy(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is abandon it should return the correctly mapped value",
			input:    apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_ABANDON,
			expected: types.ParentClosePolicyAbandon.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToParentClosePolicy(tc.input))
		})
	}
}

func TestFromParentClosePolicy(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.ParentClosePolicy
		expected apiv1.ParentClosePolicy
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.ParentClosePolicy(UnknownValue).Ptr(),
			expected: apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_INVALID,
		},
		{
			name:     "when input is abandon it should return the correctly mapped value",
			input:    types.ParentClosePolicyAbandon.Ptr(),
			expected: apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_ABANDON,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromParentClosePolicy(tc.input))
		})
	}
}
func TestPendingActivityState(t *testing.T) {
	for _, item := range []*types.PendingActivityState{
		nil,
		types.PendingActivityStateScheduled.Ptr(),
		types.PendingActivityStateStarted.Ptr(),
		types.PendingActivityStateCancelRequested.Ptr(),
	} {
		assert.Equal(t, item, ToPendingActivityState(FromPendingActivityState(item)))
	}
}

func TestToPendingActivityState(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.PendingActivityState
		expected *types.PendingActivityState
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.PendingActivityState(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is scheduled it should return the correctly mapped value",
			input:    apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_SCHEDULED,
			expected: types.PendingActivityStateScheduled.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToPendingActivityState(tc.input))
		})
	}
}

func TestFromPendingActivityState(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.PendingActivityState
		expected apiv1.PendingActivityState
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.PendingActivityState(UnknownValue).Ptr(),
			expected: apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_INVALID,
		},
		{
			name:     "when input is scheduled it should return the correctly mapped value",
			input:    types.PendingActivityStateScheduled.Ptr(),
			expected: apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_SCHEDULED,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromPendingActivityState(tc.input))
		})
	}
}
func TestPendingDecisionState(t *testing.T) {
	for _, item := range []*types.PendingDecisionState{
		nil,
		types.PendingDecisionStateScheduled.Ptr(),
		types.PendingDecisionStateStarted.Ptr(),
	} {
		assert.Equal(t, item, ToPendingDecisionState(FromPendingDecisionState(item)))
	}
}

func TestToPendingDecisionState(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.PendingDecisionState
		expected *types.PendingDecisionState
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.PendingDecisionState_PENDING_DECISION_STATE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.PendingDecisionState(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is scheduled it should return the correctly mapped value",
			input:    apiv1.PendingDecisionState_PENDING_DECISION_STATE_SCHEDULED,
			expected: types.PendingDecisionStateScheduled.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToPendingDecisionState(tc.input))
		})
	}
}

func TestFromPendingDecisionState(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.PendingDecisionState
		expected apiv1.PendingDecisionState
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.PendingDecisionState_PENDING_DECISION_STATE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.PendingDecisionState(UnknownValue).Ptr(),
			expected: apiv1.PendingDecisionState_PENDING_DECISION_STATE_INVALID,
		},
		{
			name:     "when input is scheduled it should return the correctly mapped value",
			input:    types.PendingDecisionStateScheduled.Ptr(),
			expected: apiv1.PendingDecisionState_PENDING_DECISION_STATE_SCHEDULED,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromPendingDecisionState(tc.input))
		})
	}
}
func TestQueryConsistencyLevel(t *testing.T) {
	for _, item := range []*types.QueryConsistencyLevel{
		nil,
		types.QueryConsistencyLevelEventual.Ptr(),
		types.QueryConsistencyLevelStrong.Ptr(),
	} {
		assert.Equal(t, item, ToQueryConsistencyLevel(FromQueryConsistencyLevel(item)))
	}
}

func TestToQueryConsistencyLevel(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.QueryConsistencyLevel
		expected *types.QueryConsistencyLevel
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.QueryConsistencyLevel(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is eventual it should return the correctly mapped value",
			input:    apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_EVENTUAL,
			expected: types.QueryConsistencyLevelEventual.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToQueryConsistencyLevel(tc.input))
		})
	}
}

func TestFromQueryConsistencyLevel(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.QueryConsistencyLevel
		expected apiv1.QueryConsistencyLevel
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.QueryConsistencyLevel(UnknownValue).Ptr(),
			expected: apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_INVALID,
		},
		{
			name:     "when input is eventual it should return the correctly mapped value",
			input:    types.QueryConsistencyLevelEventual.Ptr(),
			expected: apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_EVENTUAL,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromQueryConsistencyLevel(tc.input))
		})
	}
}
func TestQueryRejectCondition(t *testing.T) {
	for _, item := range []*types.QueryRejectCondition{
		nil,
		types.QueryRejectConditionNotOpen.Ptr(),
		types.QueryRejectConditionNotCompletedCleanly.Ptr(),
	} {
		assert.Equal(t, item, ToQueryRejectCondition(FromQueryRejectCondition(item)))
	}
}

func TestToQueryRejectCondition(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.QueryRejectCondition
		expected *types.QueryRejectCondition
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.QueryRejectCondition(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is not open it should return the correctly mapped value",
			input:    apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_NOT_OPEN,
			expected: types.QueryRejectConditionNotOpen.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToQueryRejectCondition(tc.input))
		})
	}
}

func TestFromQueryRejectCondition(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.QueryRejectCondition
		expected apiv1.QueryRejectCondition
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.QueryRejectCondition(UnknownValue).Ptr(),
			expected: apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_INVALID,
		},
		{
			name:     "when input is not open it should return the correctly mapped value",
			input:    types.QueryRejectConditionNotOpen.Ptr(),
			expected: apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_NOT_OPEN,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromQueryRejectCondition(tc.input))
		})
	}
}
func TestQueryResultType(t *testing.T) {
	for _, item := range []*types.QueryResultType{
		nil,
		types.QueryResultTypeAnswered.Ptr(),
		types.QueryResultTypeFailed.Ptr(),
	} {
		assert.Equal(t, item, ToQueryResultType(FromQueryResultType(item)))
	}
}

func TestToQueryResultType(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.QueryResultType
		expected *types.QueryResultType
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.QueryResultType(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is answered it should return the correctly mapped value",
			input:    apiv1.QueryResultType_QUERY_RESULT_TYPE_ANSWERED,
			expected: types.QueryResultTypeAnswered.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToQueryResultType(tc.input))
		})
	}
}

func TestFromQueryResultType(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.QueryResultType
		expected apiv1.QueryResultType
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.QueryResultType(UnknownValue).Ptr(),
			expected: apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID,
		},
		{
			name:     "when input is answered it should return the correctly mapped value",
			input:    types.QueryResultTypeAnswered.Ptr(),
			expected: apiv1.QueryResultType_QUERY_RESULT_TYPE_ANSWERED,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromQueryResultType(tc.input))
		})
	}
}
func TestQueryTaskCompletedType(t *testing.T) {
	for _, item := range []*types.QueryTaskCompletedType{
		nil,
		types.QueryTaskCompletedTypeCompleted.Ptr(),
		types.QueryTaskCompletedTypeFailed.Ptr(),
	} {
		assert.Equal(t, item, ToQueryTaskCompletedType(FromQueryTaskCompletedType(item)))
	}
}

func TestToQueryTaskCompletedType(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.QueryResultType
		expected *types.QueryTaskCompletedType
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.QueryResultType(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is completed it should return the correctly mapped value",
			input:    apiv1.QueryResultType_QUERY_RESULT_TYPE_ANSWERED,
			expected: types.QueryTaskCompletedTypeCompleted.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToQueryTaskCompletedType(tc.input))
		})
	}
}

func TestFromQueryTaskCompletedType(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.QueryTaskCompletedType
		expected apiv1.QueryResultType
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.QueryTaskCompletedType(UnknownValue).Ptr(),
			expected: apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID,
		},
		{
			name:     "when input is completed it should return the correctly mapped value",
			input:    types.QueryTaskCompletedTypeCompleted.Ptr(),
			expected: apiv1.QueryResultType_QUERY_RESULT_TYPE_ANSWERED,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromQueryTaskCompletedType(tc.input))
		})
	}
}
func TestSignalExternalWorkflowExecutionFailedCause(t *testing.T) {
	for _, item := range []*types.SignalExternalWorkflowExecutionFailedCause{
		nil,
		types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
		types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted.Ptr(),
	} {
		assert.Equal(t, item, ToSignalExternalWorkflowExecutionFailedCause(FromSignalExternalWorkflowExecutionFailedCause(item)))
	}
}

func TestToSignalExternalWorkflowExecutionFailedCause(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.SignalExternalWorkflowExecutionFailedCause
		expected *types.SignalExternalWorkflowExecutionFailedCause
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.SignalExternalWorkflowExecutionFailedCause(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is unknown external workflow execution it should return the correctly mapped value",
			input:    apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION,
			expected: types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToSignalExternalWorkflowExecutionFailedCause(tc.input))
		})
	}
}

func TestFromSignalExternalWorkflowExecutionFailedCause(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.SignalExternalWorkflowExecutionFailedCause
		expected apiv1.SignalExternalWorkflowExecutionFailedCause
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.SignalExternalWorkflowExecutionFailedCause(UnknownValue).Ptr(),
			expected: apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID,
		},
		{
			name:     "when input is unknown external workflow execution it should return the correctly mapped value",
			input:    types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
			expected: apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromSignalExternalWorkflowExecutionFailedCause(tc.input))
		})
	}
}

func TestTaskListKind(t *testing.T) {
	for _, item := range []*types.TaskListKind{
		nil,
		types.TaskListKindNormal.Ptr(),
		types.TaskListKindSticky.Ptr(),
		types.TaskListKindEphemeral.Ptr(),
	} {
		assert.Equal(t, item, ToTaskListKind(FromTaskListKind(item)))
	}
}

func TestToTaskListKind(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.TaskListKind
		expected *types.TaskListKind
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.TaskListKind_TASK_LIST_KIND_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.TaskListKind(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is normal it should return the correctly mapped value",
			input:    apiv1.TaskListKind_TASK_LIST_KIND_NORMAL,
			expected: types.TaskListKindNormal.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToTaskListKind(tc.input))
		})
	}
}

func TestFromTaskListKind(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.TaskListKind
		expected apiv1.TaskListKind
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.TaskListKind_TASK_LIST_KIND_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.TaskListKind(UnknownValue).Ptr(),
			expected: apiv1.TaskListKind_TASK_LIST_KIND_INVALID,
		},
		{
			name:     "when input is normal it should return the correctly mapped value",
			input:    types.TaskListKindNormal.Ptr(),
			expected: apiv1.TaskListKind_TASK_LIST_KIND_NORMAL,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromTaskListKind(tc.input))
		})
	}
}

func TestTaskListType(t *testing.T) {
	for _, item := range []*types.TaskListType{
		nil,
		types.TaskListTypeDecision.Ptr(),
		types.TaskListTypeActivity.Ptr(),
	} {
		assert.Equal(t, item, ToTaskListType(FromTaskListType(item)))
	}
}

func TestToTaskListType(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.TaskListType
		expected *types.TaskListType
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.TaskListType_TASK_LIST_TYPE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.TaskListType(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is decision it should return the correctly mapped value",
			input:    apiv1.TaskListType_TASK_LIST_TYPE_DECISION,
			expected: types.TaskListTypeDecision.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToTaskListType(tc.input))
		})
	}
}

func TestFromTaskListType(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.TaskListType
		expected apiv1.TaskListType
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.TaskListType_TASK_LIST_TYPE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.TaskListType(UnknownValue).Ptr(),
			expected: apiv1.TaskListType_TASK_LIST_TYPE_INVALID,
		},
		{
			name:     "when input is decision it should return the correctly mapped value",
			input:    types.TaskListTypeDecision.Ptr(),
			expected: apiv1.TaskListType_TASK_LIST_TYPE_DECISION,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromTaskListType(tc.input))
		})
	}
}
func TestTimeoutType(t *testing.T) {
	for _, item := range []*types.TimeoutType{
		nil,
		types.TimeoutTypeStartToClose.Ptr(),
		types.TimeoutTypeScheduleToStart.Ptr(),
		types.TimeoutTypeScheduleToClose.Ptr(),
		types.TimeoutTypeHeartbeat.Ptr(),
	} {
		assert.Equal(t, item, ToTimeoutType(FromTimeoutType(item)))
	}
}

func TestToTimeoutType(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.TimeoutType
		expected *types.TimeoutType
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.TimeoutType_TIMEOUT_TYPE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.TimeoutType(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is start to close it should return the correctly mapped value",
			input:    apiv1.TimeoutType_TIMEOUT_TYPE_START_TO_CLOSE,
			expected: types.TimeoutTypeStartToClose.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToTimeoutType(tc.input))
		})
	}
}

func TestFromTimeoutType(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.TimeoutType
		expected apiv1.TimeoutType
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.TimeoutType_TIMEOUT_TYPE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.TimeoutType(UnknownValue).Ptr(),
			expected: apiv1.TimeoutType_TIMEOUT_TYPE_INVALID,
		},
		{
			name:     "when input is start to close it should return the correctly mapped value",
			input:    types.TimeoutTypeStartToClose.Ptr(),
			expected: apiv1.TimeoutType_TIMEOUT_TYPE_START_TO_CLOSE,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromTimeoutType(tc.input))
		})
	}
}
func TestDecisionTaskTimedOutCause(t *testing.T) {
	for _, item := range []*types.DecisionTaskTimedOutCause{
		nil,
		types.DecisionTaskTimedOutCauseTimeout.Ptr(),
		types.DecisionTaskTimedOutCauseReset.Ptr(),
	} {
		assert.Equal(t, item, ToDecisionTaskTimedOutCause(FromDecisionTaskTimedOutCause(item)))
	}
}

func TestToDecisionTaskTimedOutCause(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.DecisionTaskTimedOutCause
		expected *types.DecisionTaskTimedOutCause
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.DecisionTaskTimedOutCause(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is timeout it should return the correctly mapped value",
			input:    apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_TIMEOUT,
			expected: types.DecisionTaskTimedOutCauseTimeout.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToDecisionTaskTimedOutCause(tc.input))
		})
	}
}

func TestFromDecisionTaskTimedOutCause(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.DecisionTaskTimedOutCause
		expected apiv1.DecisionTaskTimedOutCause
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.DecisionTaskTimedOutCause(UnknownValue).Ptr(),
			expected: apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_INVALID,
		},
		{
			name:     "when input is timeout it should return the correctly mapped value",
			input:    types.DecisionTaskTimedOutCauseTimeout.Ptr(),
			expected: apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_TIMEOUT,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromDecisionTaskTimedOutCause(tc.input))
		})
	}
}
func TestWorkflowExecutionCloseStatus(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionCloseStatus{
		nil,
		types.WorkflowExecutionCloseStatusCompleted.Ptr(),
		types.WorkflowExecutionCloseStatusFailed.Ptr(),
		types.WorkflowExecutionCloseStatusCanceled.Ptr(),
		types.WorkflowExecutionCloseStatusTerminated.Ptr(),
		types.WorkflowExecutionCloseStatusContinuedAsNew.Ptr(),
		types.WorkflowExecutionCloseStatusTimedOut.Ptr(),
	} {
		assert.Equal(t, item, ToWorkflowExecutionCloseStatus(FromWorkflowExecutionCloseStatus(item)))
	}
}

func TestToWorkflowExecutionCloseStatus(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.WorkflowExecutionCloseStatus
		expected *types.WorkflowExecutionCloseStatus
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.WorkflowExecutionCloseStatus(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is completed it should return the correctly mapped value",
			input:    apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_COMPLETED,
			expected: types.WorkflowExecutionCloseStatusCompleted.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToWorkflowExecutionCloseStatus(tc.input))
		})
	}
}

func TestFromWorkflowExecutionCloseStatus(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.WorkflowExecutionCloseStatus
		expected apiv1.WorkflowExecutionCloseStatus
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.WorkflowExecutionCloseStatus(UnknownValue).Ptr(),
			expected: apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_INVALID,
		},
		{
			name:     "when input is completed it should return the correctly mapped value",
			input:    types.WorkflowExecutionCloseStatusCompleted.Ptr(),
			expected: apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_COMPLETED,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromWorkflowExecutionCloseStatus(tc.input))
		})
	}
}
func TestWorkflowIDReusePolicy(t *testing.T) {
	for _, item := range []*types.WorkflowIDReusePolicy{
		nil,
		types.WorkflowIDReusePolicyAllowDuplicateFailedOnly.Ptr(),
		types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
		types.WorkflowIDReusePolicyRejectDuplicate.Ptr(),
		types.WorkflowIDReusePolicyTerminateIfRunning.Ptr(),
	} {
		assert.Equal(t, item, ToWorkflowIDReusePolicy(FromWorkflowIDReusePolicy(item)))
	}
}

func TestToWorkflowIDReusePolicy(t *testing.T) {
	cases := []struct {
		name     string
		input    apiv1.WorkflowIdReusePolicy
		expected *types.WorkflowIDReusePolicy
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    apiv1.WorkflowIdReusePolicy(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is allow duplicate failed only it should return the correctly mapped value",
			input:    apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
			expected: types.WorkflowIDReusePolicyAllowDuplicateFailedOnly.Ptr(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToWorkflowIDReusePolicy(tc.input))
		})
	}
}

func TestFromWorkflowIDReusePolicy(t *testing.T) {
	cases := []struct {
		name     string
		input    *types.WorkflowIDReusePolicy
		expected apiv1.WorkflowIdReusePolicy
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    types.WorkflowIDReusePolicy(UnknownValue).Ptr(),
			expected: apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_INVALID,
		},
		{
			name:     "when input is allow duplicate failed only it should return the correctly mapped value",
			input:    types.WorkflowIDReusePolicyAllowDuplicateFailedOnly.Ptr(),
			expected: apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromWorkflowIDReusePolicy(tc.input))
		})
	}
}
func TestWorkflowState(t *testing.T) {
	for _, item := range []*int32{
		nil,
		common.Int32Ptr(persistence.WorkflowStateCreated),
		common.Int32Ptr(persistence.WorkflowStateRunning),
		common.Int32Ptr(persistence.WorkflowStateCompleted),
		common.Int32Ptr(persistence.WorkflowStateZombie),
		common.Int32Ptr(persistence.WorkflowStateVoid),
		common.Int32Ptr(persistence.WorkflowStateCorrupted),
	} {
		assert.Equal(t, item, ToWorkflowState(FromWorkflowState(item)))
	}
}

func TestToWorkflowState(t *testing.T) {
	cases := []struct {
		name     string
		input    sharedv1.WorkflowState
		expected *int32
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    sharedv1.WorkflowState_WORKFLOW_STATE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    sharedv1.WorkflowState(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is created it should return the correctly mapped value",
			input:    sharedv1.WorkflowState_WORKFLOW_STATE_CREATED,
			expected: common.Int32Ptr(persistence.WorkflowStateCreated),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToWorkflowState(tc.input))
		})
	}
}

func TestFromWorkflowState(t *testing.T) {
	cases := []struct {
		name     string
		input    *int32
		expected sharedv1.WorkflowState
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: sharedv1.WorkflowState_WORKFLOW_STATE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    common.Int32Ptr(UnknownValue),
			expected: sharedv1.WorkflowState_WORKFLOW_STATE_INVALID,
		},
		{
			name:     "when input is created it should return the correctly mapped value",
			input:    common.Int32Ptr(persistence.WorkflowStateCreated),
			expected: sharedv1.WorkflowState_WORKFLOW_STATE_CREATED,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromWorkflowState(tc.input))
		})
	}
}
func TestTaskType(t *testing.T) {
	for _, item := range []*int32{
		nil,
		common.Int32Ptr(int32(constants.TaskTypeTransfer)),
		common.Int32Ptr(int32(constants.TaskTypeTimer)),
		common.Int32Ptr(int32(constants.TaskTypeReplication)),
		common.Int32Ptr(int32(constants.TaskTypeCrossCluster)),
	} {
		assert.Equal(t, item, ToTaskType(FromTaskType(item)))
	}
}

func TestToTaskType(t *testing.T) {
	cases := []struct {
		name     string
		input    adminv1.TaskType
		expected *int32
	}{
		{
			name:     "when input is invalid it should return nil",
			input:    adminv1.TaskType_TASK_TYPE_INVALID,
			expected: nil,
		},
		{
			name:     "when input is out of range it should return nil",
			input:    adminv1.TaskType(UnknownValue),
			expected: nil,
		},
		{
			name:     "when input is transfer it should return the correctly mapped value",
			input:    adminv1.TaskType_TASK_TYPE_TRANSFER,
			expected: common.Int32Ptr(int32(constants.TaskTypeTransfer)),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToTaskType(tc.input))
		})
	}
}

func TestFromTaskType(t *testing.T) {
	cases := []struct {
		name     string
		input    *int32
		expected adminv1.TaskType
	}{
		{
			name:     "when input is nil it should return INVALID",
			input:    nil,
			expected: adminv1.TaskType_TASK_TYPE_INVALID,
		},
		{
			name:     "when input is out of range it should return INVALID",
			input:    common.Int32Ptr(UnknownValue),
			expected: adminv1.TaskType_TASK_TYPE_INVALID,
		},
		{
			name:     "when input is transfer it should return the correctly mapped value",
			input:    common.Int32Ptr(int32(constants.TaskTypeTransfer)),
			expected: adminv1.TaskType_TASK_TYPE_TRANSFER,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromTaskType(tc.input))
		})
	}
}
