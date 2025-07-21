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
	assert.Panics(t, func() { ToTaskSource(sharedv1.TaskSource(UnknownValue)) })
	assert.Panics(t, func() { FromTaskSource(types.TaskSource(UnknownValue).Ptr()) })
}
func TestDLQType(t *testing.T) {
	for _, item := range []*types.DLQType{
		nil,
		types.DLQTypeReplication.Ptr(),
		types.DLQTypeDomain.Ptr(),
	} {
		assert.Equal(t, item, ToDLQType(FromDLQType(item)))
	}
	assert.Panics(t, func() { ToDLQType(adminv1.DLQType(UnknownValue)) })
	assert.Panics(t, func() { FromDLQType(types.DLQType(UnknownValue).Ptr()) })
}
func TestDomainOperation(t *testing.T) {
	for _, item := range []*types.DomainOperation{
		nil,
		types.DomainOperationCreate.Ptr(),
		types.DomainOperationUpdate.Ptr(),
	} {
		assert.Equal(t, item, ToDomainOperation(FromDomainOperation(item)))
	}
	assert.Panics(t, func() { ToDomainOperation(adminv1.DomainOperation(UnknownValue)) })
	assert.Panics(t, func() { FromDomainOperation(types.DomainOperation(UnknownValue).Ptr()) })
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
	assert.Panics(t, func() { ToReplicationTaskType(adminv1.ReplicationTaskType(UnknownValue)) })
	assert.Panics(t, func() { FromReplicationTaskType(types.ReplicationTaskType(UnknownValue).Ptr()) })
}
func TestArchivalStatus(t *testing.T) {
	for _, item := range []*types.ArchivalStatus{
		nil,
		types.ArchivalStatusDisabled.Ptr(),
		types.ArchivalStatusEnabled.Ptr(),
	} {
		assert.Equal(t, item, ToArchivalStatus(FromArchivalStatus(item)))
	}
	assert.Panics(t, func() { ToArchivalStatus(apiv1.ArchivalStatus(UnknownValue)) })
	assert.Panics(t, func() { FromArchivalStatus(types.ArchivalStatus(UnknownValue).Ptr()) })
}
func TestCancelExternalWorkflowExecutionFailedCause(t *testing.T) {
	for _, item := range []*types.CancelExternalWorkflowExecutionFailedCause{
		nil,
		types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
		types.CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted.Ptr(),
	} {
		assert.Equal(t, item, ToCancelExternalWorkflowExecutionFailedCause(FromCancelExternalWorkflowExecutionFailedCause(item)))
	}
	assert.Panics(t, func() {
		ToCancelExternalWorkflowExecutionFailedCause(apiv1.CancelExternalWorkflowExecutionFailedCause(UnknownValue))
	})
	assert.Panics(t, func() {
		FromCancelExternalWorkflowExecutionFailedCause(types.CancelExternalWorkflowExecutionFailedCause(UnknownValue).Ptr())
	})
}
func TestChildWorkflowExecutionFailedCause(t *testing.T) {
	for _, item := range []*types.ChildWorkflowExecutionFailedCause{
		nil,
		types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning.Ptr(),
	} {
		assert.Equal(t, item, ToChildWorkflowExecutionFailedCause(FromChildWorkflowExecutionFailedCause(item)))
	}
	assert.Panics(t, func() { ToChildWorkflowExecutionFailedCause(apiv1.ChildWorkflowExecutionFailedCause(UnknownValue)) })
	assert.Panics(t, func() {
		FromChildWorkflowExecutionFailedCause(types.ChildWorkflowExecutionFailedCause(UnknownValue).Ptr())
	})
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
	assert.Panics(t, func() { ToContinueAsNewInitiator(apiv1.ContinueAsNewInitiator(UnknownValue)) })
	assert.Panics(t, func() { FromContinueAsNewInitiator(types.ContinueAsNewInitiator(UnknownValue).Ptr()) })
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
	assert.Panics(t, func() { ToCrossClusterTaskFailedCause(adminv1.CrossClusterTaskFailedCause(UnknownValue)) })
	assert.Panics(t, func() { FromCrossClusterTaskFailedCause(types.CrossClusterTaskFailedCause(UnknownValue).Ptr()) })
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
	assert.Panics(t, func() { ToDecisionTaskFailedCause(apiv1.DecisionTaskFailedCause(UnknownValue)) })
	assert.Panics(t, func() { FromDecisionTaskFailedCause(types.DecisionTaskFailedCause(UnknownValue).Ptr()) })
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
	assert.Panics(t, func() { ToDomainStatus(apiv1.DomainStatus(UnknownValue)) })
	assert.Panics(t, func() { FromDomainStatus(types.DomainStatus(UnknownValue).Ptr()) })
}
func TestEncodingType(t *testing.T) {
	for _, item := range []*types.EncodingType{
		nil,
		types.EncodingTypeThriftRW.Ptr(),
		types.EncodingTypeJSON.Ptr(),
	} {
		assert.Equal(t, item, ToEncodingType(FromEncodingType(item)))
	}
	assert.Panics(t, func() { ToEncodingType(apiv1.EncodingType(UnknownValue)) })
	assert.Panics(t, func() { FromEncodingType(types.EncodingType(UnknownValue).Ptr()) })
}
func TestEventFilterType(t *testing.T) {
	for _, item := range []*types.HistoryEventFilterType{
		nil,
		types.HistoryEventFilterTypeAllEvent.Ptr(),
		types.HistoryEventFilterTypeCloseEvent.Ptr(),
	} {
		assert.Equal(t, item, ToEventFilterType(FromEventFilterType(item)))
	}
	assert.Panics(t, func() { ToEventFilterType(apiv1.EventFilterType(UnknownValue)) })
	assert.Panics(t, func() { FromEventFilterType(types.HistoryEventFilterType(UnknownValue).Ptr()) })
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
	assert.Panics(t, func() { ToIndexedValueType(apiv1.IndexedValueType_INDEXED_VALUE_TYPE_INVALID) })
	assert.Panics(t, func() { ToIndexedValueType(apiv1.IndexedValueType(UnknownValue)) })
	assert.Panics(t, func() { FromIndexedValueType(types.IndexedValueType(UnknownValue)) })
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
	assert.Panics(t, func() { ToParentClosePolicy(apiv1.ParentClosePolicy(UnknownValue)) })
	assert.Panics(t, func() { FromParentClosePolicy(types.ParentClosePolicy(UnknownValue).Ptr()) })
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
	assert.Panics(t, func() { ToPendingActivityState(apiv1.PendingActivityState(UnknownValue)) })
	assert.Panics(t, func() { FromPendingActivityState(types.PendingActivityState(UnknownValue).Ptr()) })
}
func TestPendingDecisionState(t *testing.T) {
	for _, item := range []*types.PendingDecisionState{
		nil,
		types.PendingDecisionStateScheduled.Ptr(),
		types.PendingDecisionStateStarted.Ptr(),
	} {
		assert.Equal(t, item, ToPendingDecisionState(FromPendingDecisionState(item)))
	}
	assert.Panics(t, func() { ToPendingDecisionState(apiv1.PendingDecisionState(UnknownValue)) })
	assert.Panics(t, func() { FromPendingDecisionState(types.PendingDecisionState(UnknownValue).Ptr()) })
}
func TestQueryConsistencyLevel(t *testing.T) {
	for _, item := range []*types.QueryConsistencyLevel{
		nil,
		types.QueryConsistencyLevelEventual.Ptr(),
		types.QueryConsistencyLevelStrong.Ptr(),
	} {
		assert.Equal(t, item, ToQueryConsistencyLevel(FromQueryConsistencyLevel(item)))
	}
	assert.Panics(t, func() { ToQueryConsistencyLevel(apiv1.QueryConsistencyLevel(UnknownValue)) })
	assert.Panics(t, func() { FromQueryConsistencyLevel(types.QueryConsistencyLevel(UnknownValue).Ptr()) })
}
func TestQueryRejectCondition(t *testing.T) {
	for _, item := range []*types.QueryRejectCondition{
		nil,
		types.QueryRejectConditionNotOpen.Ptr(),
		types.QueryRejectConditionNotCompletedCleanly.Ptr(),
	} {
		assert.Equal(t, item, ToQueryRejectCondition(FromQueryRejectCondition(item)))
	}
	assert.Panics(t, func() { ToQueryRejectCondition(apiv1.QueryRejectCondition(UnknownValue)) })
	assert.Panics(t, func() { FromQueryRejectCondition(types.QueryRejectCondition(UnknownValue).Ptr()) })
}
func TestQueryResultType(t *testing.T) {
	for _, item := range []*types.QueryResultType{
		nil,
		types.QueryResultTypeAnswered.Ptr(),
		types.QueryResultTypeFailed.Ptr(),
	} {
		assert.Equal(t, item, ToQueryResultType(FromQueryResultType(item)))
	}
	assert.Panics(t, func() { ToQueryResultType(apiv1.QueryResultType(UnknownValue)) })
	assert.Panics(t, func() { FromQueryResultType(types.QueryResultType(UnknownValue).Ptr()) })
}
func TestQueryTaskCompletedType(t *testing.T) {
	for _, item := range []*types.QueryTaskCompletedType{
		nil,
		types.QueryTaskCompletedTypeCompleted.Ptr(),
		types.QueryTaskCompletedTypeFailed.Ptr(),
	} {
		assert.Equal(t, item, ToQueryTaskCompletedType(FromQueryTaskCompletedType(item)))
	}
	assert.Panics(t, func() { ToQueryTaskCompletedType(apiv1.QueryResultType(UnknownValue)) })
	assert.Panics(t, func() { FromQueryTaskCompletedType(types.QueryTaskCompletedType(UnknownValue).Ptr()) })
}
func TestSignalExternalWorkflowExecutionFailedCause(t *testing.T) {
	for _, item := range []*types.SignalExternalWorkflowExecutionFailedCause{
		nil,
		types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
		types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted.Ptr(),
	} {
		assert.Equal(t, item, ToSignalExternalWorkflowExecutionFailedCause(FromSignalExternalWorkflowExecutionFailedCause(item)))
	}
	assert.Panics(t, func() {
		ToSignalExternalWorkflowExecutionFailedCause(apiv1.SignalExternalWorkflowExecutionFailedCause(UnknownValue))
	})
	assert.Panics(t, func() {
		FromSignalExternalWorkflowExecutionFailedCause(types.SignalExternalWorkflowExecutionFailedCause(UnknownValue).Ptr())
	})
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
	assert.Panics(t, func() { ToTaskListKind(apiv1.TaskListKind(UnknownValue)) })
	assert.Panics(t, func() { FromTaskListKind(types.TaskListKind(UnknownValue).Ptr()) })
}
func TestTaskListType(t *testing.T) {
	for _, item := range []*types.TaskListType{
		nil,
		types.TaskListTypeDecision.Ptr(),
		types.TaskListTypeActivity.Ptr(),
	} {
		assert.Equal(t, item, ToTaskListType(FromTaskListType(item)))
	}
	assert.Panics(t, func() { ToTaskListType(apiv1.TaskListType(UnknownValue)) })
	assert.Panics(t, func() { FromTaskListType(types.TaskListType(UnknownValue).Ptr()) })
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
	assert.Panics(t, func() { ToTimeoutType(apiv1.TimeoutType(UnknownValue)) })
	assert.Panics(t, func() { FromTimeoutType(types.TimeoutType(UnknownValue).Ptr()) })
}
func TestDecisionTaskTimedOutCause(t *testing.T) {
	for _, item := range []*types.DecisionTaskTimedOutCause{
		nil,
		types.DecisionTaskTimedOutCauseTimeout.Ptr(),
		types.DecisionTaskTimedOutCauseReset.Ptr(),
	} {
		assert.Equal(t, item, ToDecisionTaskTimedOutCause(FromDecisionTaskTimedOutCause(item)))
	}
	assert.Panics(t, func() { ToDecisionTaskTimedOutCause(apiv1.DecisionTaskTimedOutCause(UnknownValue)) })
	assert.Panics(t, func() { FromDecisionTaskTimedOutCause(types.DecisionTaskTimedOutCause(UnknownValue).Ptr()) })
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
	assert.Panics(t, func() { ToWorkflowExecutionCloseStatus(apiv1.WorkflowExecutionCloseStatus(UnknownValue)) })
	assert.Panics(t, func() { FromWorkflowExecutionCloseStatus(types.WorkflowExecutionCloseStatus(UnknownValue).Ptr()) })
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
	assert.Panics(t, func() { ToWorkflowIDReusePolicy(apiv1.WorkflowIdReusePolicy(UnknownValue)) })
	assert.Panics(t, func() { FromWorkflowIDReusePolicy(types.WorkflowIDReusePolicy(UnknownValue).Ptr()) })
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
	assert.Panics(t, func() { ToWorkflowState(sharedv1.WorkflowState(UnknownValue)) })
	assert.Panics(t, func() { FromWorkflowState(common.Int32Ptr(UnknownValue)) })
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
	assert.Panics(t, func() { ToTaskType(adminv1.TaskType(UnknownValue)) })
	assert.Panics(t, func() { FromTaskType(common.Int32Ptr(UnknownValue)) })
}
