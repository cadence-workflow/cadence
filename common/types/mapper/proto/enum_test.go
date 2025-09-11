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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToTaskSource(sharedv1.TaskSource(UnknownValue)) })
	assert.NotPanics(t, func() { FromTaskSource(types.TaskSource(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToTaskSource(sharedv1.TaskSource(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, sharedv1.TaskSource_TASK_SOURCE_INVALID, FromTaskSource(types.TaskSource(UnknownValue).Ptr()))
}
func TestDLQType(t *testing.T) {
	for _, item := range []*types.DLQType{
		nil,
		types.DLQTypeReplication.Ptr(),
		types.DLQTypeDomain.Ptr(),
	} {
		assert.Equal(t, item, ToDLQType(FromDLQType(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToDLQType(adminv1.DLQType(UnknownValue)) })
	assert.NotPanics(t, func() { FromDLQType(types.DLQType(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToDLQType(adminv1.DLQType(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, adminv1.DLQType_DLQ_TYPE_INVALID, FromDLQType(types.DLQType(UnknownValue).Ptr()))
}
func TestDomainOperation(t *testing.T) {
	for _, item := range []*types.DomainOperation{
		nil,
		types.DomainOperationCreate.Ptr(),
		types.DomainOperationUpdate.Ptr(),
	} {
		assert.Equal(t, item, ToDomainOperation(FromDomainOperation(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToDomainOperation(adminv1.DomainOperation(UnknownValue)) })
	assert.NotPanics(t, func() { FromDomainOperation(types.DomainOperation(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToDomainOperation(adminv1.DomainOperation(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, adminv1.DomainOperation_DOMAIN_OPERATION_INVALID, FromDomainOperation(types.DomainOperation(UnknownValue).Ptr()))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToReplicationTaskType(adminv1.ReplicationTaskType(UnknownValue)) })
	assert.NotPanics(t, func() { FromReplicationTaskType(types.ReplicationTaskType(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToReplicationTaskType(adminv1.ReplicationTaskType(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, adminv1.ReplicationTaskType_REPLICATION_TASK_TYPE_INVALID, FromReplicationTaskType(types.ReplicationTaskType(UnknownValue).Ptr()))
}
func TestArchivalStatus(t *testing.T) {
	for _, item := range []*types.ArchivalStatus{
		nil,
		types.ArchivalStatusDisabled.Ptr(),
		types.ArchivalStatusEnabled.Ptr(),
	} {
		assert.Equal(t, item, ToArchivalStatus(FromArchivalStatus(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToArchivalStatus(apiv1.ArchivalStatus(UnknownValue)) })
	assert.NotPanics(t, func() { FromArchivalStatus(types.ArchivalStatus(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToArchivalStatus(apiv1.ArchivalStatus(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.ArchivalStatus_ARCHIVAL_STATUS_INVALID, FromArchivalStatus(types.ArchivalStatus(UnknownValue).Ptr()))
}
func TestCancelExternalWorkflowExecutionFailedCause(t *testing.T) {
	for _, item := range []*types.CancelExternalWorkflowExecutionFailedCause{
		nil,
		types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
		types.CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted.Ptr(),
	} {
		assert.Equal(t, item, ToCancelExternalWorkflowExecutionFailedCause(FromCancelExternalWorkflowExecutionFailedCause(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToCancelExternalWorkflowExecutionFailedCause(apiv1.CancelExternalWorkflowExecutionFailedCause(UnknownValue)) })
	assert.NotPanics(t, func() { FromCancelExternalWorkflowExecutionFailedCause(types.CancelExternalWorkflowExecutionFailedCause(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToCancelExternalWorkflowExecutionFailedCause(apiv1.CancelExternalWorkflowExecutionFailedCause(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID, FromCancelExternalWorkflowExecutionFailedCause(types.CancelExternalWorkflowExecutionFailedCause(UnknownValue).Ptr()))
}
func TestChildWorkflowExecutionFailedCause(t *testing.T) {
	for _, item := range []*types.ChildWorkflowExecutionFailedCause{
		nil,
		types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning.Ptr(),
	} {
		assert.Equal(t, item, ToChildWorkflowExecutionFailedCause(FromChildWorkflowExecutionFailedCause(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToChildWorkflowExecutionFailedCause(apiv1.ChildWorkflowExecutionFailedCause(UnknownValue)) })
	assert.NotPanics(t, func() {
		FromChildWorkflowExecutionFailedCause(types.ChildWorkflowExecutionFailedCause(UnknownValue).Ptr())
	})
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToChildWorkflowExecutionFailedCause(apiv1.ChildWorkflowExecutionFailedCause(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.ChildWorkflowExecutionFailedCause_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID, FromChildWorkflowExecutionFailedCause(types.ChildWorkflowExecutionFailedCause(UnknownValue).Ptr()))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToContinueAsNewInitiator(apiv1.ContinueAsNewInitiator(UnknownValue)) })
	assert.NotPanics(t, func() { FromContinueAsNewInitiator(types.ContinueAsNewInitiator(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToContinueAsNewInitiator(apiv1.ContinueAsNewInitiator(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_INVALID, FromContinueAsNewInitiator(types.ContinueAsNewInitiator(UnknownValue).Ptr()))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToCrossClusterTaskFailedCause(adminv1.CrossClusterTaskFailedCause(UnknownValue)) })
	assert.NotPanics(t, func() { FromCrossClusterTaskFailedCause(types.CrossClusterTaskFailedCause(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToCrossClusterTaskFailedCause(adminv1.CrossClusterTaskFailedCause(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, adminv1.CrossClusterTaskFailedCause_CROSS_CLUSTER_TASK_FAILED_CAUSE_INVALID, FromCrossClusterTaskFailedCause(types.CrossClusterTaskFailedCause(UnknownValue).Ptr()))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToDecisionTaskFailedCause(apiv1.DecisionTaskFailedCause(UnknownValue)) })
	assert.NotPanics(t, func() { FromDecisionTaskFailedCause(types.DecisionTaskFailedCause(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToDecisionTaskFailedCause(apiv1.DecisionTaskFailedCause(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_INVALID, FromDecisionTaskFailedCause(types.DecisionTaskFailedCause(UnknownValue).Ptr()))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToDomainStatus(apiv1.DomainStatus(UnknownValue)) })
	assert.NotPanics(t, func() { FromDomainStatus(types.DomainStatus(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToDomainStatus(apiv1.DomainStatus(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.DomainStatus_DOMAIN_STATUS_INVALID, FromDomainStatus(types.DomainStatus(UnknownValue).Ptr()))
}
func TestEncodingType(t *testing.T) {
	for _, item := range []*types.EncodingType{
		nil,
		types.EncodingTypeThriftRW.Ptr(),
		types.EncodingTypeJSON.Ptr(),
	} {
		assert.Equal(t, item, ToEncodingType(FromEncodingType(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToEncodingType(apiv1.EncodingType(UnknownValue)) })
	assert.NotPanics(t, func() { FromEncodingType(types.EncodingType(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToEncodingType(apiv1.EncodingType(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.EncodingType_ENCODING_TYPE_INVALID, FromEncodingType(types.EncodingType(UnknownValue).Ptr()))
}
func TestEventFilterType(t *testing.T) {
	for _, item := range []*types.HistoryEventFilterType{
		nil,
		types.HistoryEventFilterTypeAllEvent.Ptr(),
		types.HistoryEventFilterTypeCloseEvent.Ptr(),
	} {
		assert.Equal(t, item, ToEventFilterType(FromEventFilterType(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToEventFilterType(apiv1.EventFilterType(UnknownValue)) })
	assert.NotPanics(t, func() { FromEventFilterType(types.HistoryEventFilterType(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToEventFilterType(apiv1.EventFilterType(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.EventFilterType_EVENT_FILTER_TYPE_INVALID, FromEventFilterType(types.HistoryEventFilterType(UnknownValue).Ptr()))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToIndexedValueType(apiv1.IndexedValueType_INDEXED_VALUE_TYPE_INVALID) })
	assert.NotPanics(t, func() { ToIndexedValueType(apiv1.IndexedValueType(UnknownValue)) })
	assert.NotPanics(t, func() { FromIndexedValueType(types.IndexedValueType(UnknownValue)) })
	
	// Unknown proto enum values should map to zero value (STRING)
	assert.Equal(t, types.IndexedValueTypeString, ToIndexedValueType(apiv1.IndexedValueType_INDEXED_VALUE_TYPE_INVALID))
	assert.Equal(t, types.IndexedValueTypeString, ToIndexedValueType(apiv1.IndexedValueType(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.IndexedValueType_INDEXED_VALUE_TYPE_INVALID, FromIndexedValueType(types.IndexedValueType(UnknownValue)))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToParentClosePolicy(apiv1.ParentClosePolicy(UnknownValue)) })
	assert.NotPanics(t, func() { FromParentClosePolicy(types.ParentClosePolicy(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToParentClosePolicy(apiv1.ParentClosePolicy(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_INVALID, FromParentClosePolicy(types.ParentClosePolicy(UnknownValue).Ptr()))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToPendingActivityState(apiv1.PendingActivityState(UnknownValue)) })
	assert.NotPanics(t, func() { FromPendingActivityState(types.PendingActivityState(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToPendingActivityState(apiv1.PendingActivityState(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_INVALID, FromPendingActivityState(types.PendingActivityState(UnknownValue).Ptr()))
}
func TestPendingDecisionState(t *testing.T) {
	for _, item := range []*types.PendingDecisionState{
		nil,
		types.PendingDecisionStateScheduled.Ptr(),
		types.PendingDecisionStateStarted.Ptr(),
	} {
		assert.Equal(t, item, ToPendingDecisionState(FromPendingDecisionState(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToPendingDecisionState(apiv1.PendingDecisionState(UnknownValue)) })
	assert.NotPanics(t, func() { FromPendingDecisionState(types.PendingDecisionState(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToPendingDecisionState(apiv1.PendingDecisionState(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.PendingDecisionState_PENDING_DECISION_STATE_INVALID, FromPendingDecisionState(types.PendingDecisionState(UnknownValue).Ptr()))
}
func TestQueryConsistencyLevel(t *testing.T) {
	for _, item := range []*types.QueryConsistencyLevel{
		nil,
		types.QueryConsistencyLevelEventual.Ptr(),
		types.QueryConsistencyLevelStrong.Ptr(),
	} {
		assert.Equal(t, item, ToQueryConsistencyLevel(FromQueryConsistencyLevel(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToQueryConsistencyLevel(apiv1.QueryConsistencyLevel(UnknownValue)) })
	assert.NotPanics(t, func() { FromQueryConsistencyLevel(types.QueryConsistencyLevel(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToQueryConsistencyLevel(apiv1.QueryConsistencyLevel(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_INVALID, FromQueryConsistencyLevel(types.QueryConsistencyLevel(UnknownValue).Ptr()))
}
func TestQueryRejectCondition(t *testing.T) {
	for _, item := range []*types.QueryRejectCondition{
		nil,
		types.QueryRejectConditionNotOpen.Ptr(),
		types.QueryRejectConditionNotCompletedCleanly.Ptr(),
	} {
		assert.Equal(t, item, ToQueryRejectCondition(FromQueryRejectCondition(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToQueryRejectCondition(apiv1.QueryRejectCondition(UnknownValue)) })
	assert.NotPanics(t, func() { FromQueryRejectCondition(types.QueryRejectCondition(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToQueryRejectCondition(apiv1.QueryRejectCondition(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_INVALID, FromQueryRejectCondition(types.QueryRejectCondition(UnknownValue).Ptr()))
}
func TestQueryResultType(t *testing.T) {
	for _, item := range []*types.QueryResultType{
		nil,
		types.QueryResultTypeAnswered.Ptr(),
		types.QueryResultTypeFailed.Ptr(),
	} {
		assert.Equal(t, item, ToQueryResultType(FromQueryResultType(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToQueryResultType(apiv1.QueryResultType(UnknownValue)) })
	assert.NotPanics(t, func() { FromQueryResultType(types.QueryResultType(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToQueryResultType(apiv1.QueryResultType(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID, FromQueryResultType(types.QueryResultType(UnknownValue).Ptr()))
}
func TestQueryTaskCompletedType(t *testing.T) {
	for _, item := range []*types.QueryTaskCompletedType{
		nil,
		types.QueryTaskCompletedTypeCompleted.Ptr(),
		types.QueryTaskCompletedTypeFailed.Ptr(),
	} {
		assert.Equal(t, item, ToQueryTaskCompletedType(FromQueryTaskCompletedType(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToQueryTaskCompletedType(apiv1.QueryResultType(UnknownValue)) })
	assert.NotPanics(t, func() { FromQueryTaskCompletedType(types.QueryTaskCompletedType(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToQueryTaskCompletedType(apiv1.QueryResultType(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID, FromQueryTaskCompletedType(types.QueryTaskCompletedType(UnknownValue).Ptr()))
}
func TestSignalExternalWorkflowExecutionFailedCause(t *testing.T) {
	for _, item := range []*types.SignalExternalWorkflowExecutionFailedCause{
		nil,
		types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
		types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted.Ptr(),
	} {
		assert.Equal(t, item, ToSignalExternalWorkflowExecutionFailedCause(FromSignalExternalWorkflowExecutionFailedCause(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() {
		ToSignalExternalWorkflowExecutionFailedCause(apiv1.SignalExternalWorkflowExecutionFailedCause(UnknownValue))
	})
	assert.NotPanics(t, func() {
		FromSignalExternalWorkflowExecutionFailedCause(types.SignalExternalWorkflowExecutionFailedCause(UnknownValue).Ptr())
	})
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToSignalExternalWorkflowExecutionFailedCause(apiv1.SignalExternalWorkflowExecutionFailedCause(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID, FromSignalExternalWorkflowExecutionFailedCause(types.SignalExternalWorkflowExecutionFailedCause(UnknownValue).Ptr()))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToTaskListKind(apiv1.TaskListKind(UnknownValue)) })
	assert.NotPanics(t, func() { FromTaskListKind(types.TaskListKind(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToTaskListKind(apiv1.TaskListKind(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.TaskListKind_TASK_LIST_KIND_INVALID, FromTaskListKind(types.TaskListKind(UnknownValue).Ptr()))
}
func TestTaskListType(t *testing.T) {
	for _, item := range []*types.TaskListType{
		nil,
		types.TaskListTypeDecision.Ptr(),
		types.TaskListTypeActivity.Ptr(),
	} {
		assert.Equal(t, item, ToTaskListType(FromTaskListType(item)))
	}
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToTaskListType(apiv1.TaskListType(UnknownValue)) })
	assert.NotPanics(t, func() { FromTaskListType(types.TaskListType(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToTaskListType(apiv1.TaskListType(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.TaskListType_TASK_LIST_TYPE_INVALID, FromTaskListType(types.TaskListType(UnknownValue).Ptr()))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToTimeoutType(apiv1.TimeoutType(UnknownValue)) })
	assert.NotPanics(t, func() { FromTimeoutType(types.TimeoutType(UnknownValue).Ptr()) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToTimeoutType(apiv1.TimeoutType(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, apiv1.TimeoutType_TIMEOUT_TYPE_INVALID, FromTimeoutType(types.TimeoutType(UnknownValue).Ptr()))
}
func TestDecisionTaskTimedOutCause(t *testing.T) {
	for _, item := range []*types.DecisionTaskTimedOutCause{
		nil,
		types.DecisionTaskTimedOutCauseTimeout.Ptr(),
		types.DecisionTaskTimedOutCauseReset.Ptr(),
	} {
		assert.Equal(t, item, ToDecisionTaskTimedOutCause(FromDecisionTaskTimedOutCause(item)))
	}
	// Test safe behavior for unknown enum values
	assert.NotPanics(t, func() { ToDecisionTaskTimedOutCause(apiv1.DecisionTaskTimedOutCause(UnknownValue)) })
	assert.NotPanics(t, func() { FromDecisionTaskTimedOutCause(types.DecisionTaskTimedOutCause(UnknownValue).Ptr()) })
	// Verify safe return values
	assert.Nil(t, ToDecisionTaskTimedOutCause(apiv1.DecisionTaskTimedOutCause(UnknownValue)))
	assert.Equal(t, apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_INVALID, FromDecisionTaskTimedOutCause(types.DecisionTaskTimedOutCause(UnknownValue).Ptr()))
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
	// Test safe behavior for unknown enum values
	assert.NotPanics(t, func() { ToWorkflowExecutionCloseStatus(apiv1.WorkflowExecutionCloseStatus(UnknownValue)) })
	assert.NotPanics(t, func() { FromWorkflowExecutionCloseStatus(types.WorkflowExecutionCloseStatus(UnknownValue).Ptr()) })
	// Verify safe return values
	assert.Nil(t, ToWorkflowExecutionCloseStatus(apiv1.WorkflowExecutionCloseStatus(UnknownValue)))
	assert.Equal(t, apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_INVALID, FromWorkflowExecutionCloseStatus(types.WorkflowExecutionCloseStatus(UnknownValue).Ptr()))
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
	// Test safe behavior for unknown enum values
	assert.NotPanics(t, func() { ToWorkflowIDReusePolicy(apiv1.WorkflowIdReusePolicy(UnknownValue)) })
	assert.NotPanics(t, func() { FromWorkflowIDReusePolicy(types.WorkflowIDReusePolicy(UnknownValue).Ptr()) })
	// Verify safe return values
	assert.Nil(t, ToWorkflowIDReusePolicy(apiv1.WorkflowIdReusePolicy(UnknownValue)))
	assert.Equal(t, apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_INVALID, FromWorkflowIDReusePolicy(types.WorkflowIDReusePolicy(UnknownValue).Ptr()))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToWorkflowState(sharedv1.WorkflowState(UnknownValue)) })
	assert.NotPanics(t, func() { FromWorkflowState(common.Int32Ptr(UnknownValue)) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToWorkflowState(sharedv1.WorkflowState(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, sharedv1.WorkflowState_WORKFLOW_STATE_INVALID, FromWorkflowState(common.Int32Ptr(UnknownValue)))
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
	// Test safe handling of unknown values - should not panic
	assert.NotPanics(t, func() { ToTaskType(adminv1.TaskType(UnknownValue)) })
	assert.NotPanics(t, func() { FromTaskType(common.Int32Ptr(UnknownValue)) })
	
	// Unknown proto enum values should map to nil
	assert.Nil(t, ToTaskType(adminv1.TaskType(UnknownValue)))
	
	// Unknown internal enum values should map to INVALID
	assert.Equal(t, adminv1.TaskType_TASK_TYPE_INVALID, FromTaskType(common.Int32Ptr(UnknownValue)))
}
