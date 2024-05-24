package execution

import (
	"github.com/stretchr/testify/assert"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"testing"
)

func Test__IsSignalRequested(t *testing.T) {
	mb := testMutableStateBuilder(t)
	requestID := "101"
	t.Run("signal not found", func(t *testing.T) {
		result := mb.IsSignalRequested(requestID)
		assert.False(t, result)
	})
	t.Run("signal found", func(t *testing.T) {
		mb.pendingSignalRequestedIDs[requestID] = struct{}{}
		result := mb.IsSignalRequested(requestID)
		assert.True(t, result)
	})
}

func Test__GetSignalInfo(t *testing.T) {
	mb := testMutableStateBuilder(t)
	initiatedEventID := int64(1)
	info := &persistence.SignalInfo{
		InitiatedID:     1,
		SignalRequestID: "101",
	}
	t.Run("signal not found", func(t *testing.T) {
		_, ok := mb.GetSignalInfo(initiatedEventID)
		assert.False(t, ok)
	})
	t.Run("signal found", func(t *testing.T) {
		mb.pendingSignalInfoIDs[initiatedEventID] = info
		result, ok := mb.GetSignalInfo(initiatedEventID)
		assert.True(t, ok)
		assert.Equal(t, info, result)
	})
}

func Test__ReplicateExternalWorkflowExecutionSignaled(t *testing.T) {
	mb := testMutableStateBuilder(t)
	event := &types.HistoryEvent{
		ExternalWorkflowExecutionSignaledEventAttributes: &types.ExternalWorkflowExecutionSignaledEventAttributes{
			InitiatedEventID: 1,
		},
	}
	info := &persistence.SignalInfo{
		InitiatedID:     1,
		SignalRequestID: "101",
	}
	mb.pendingSignalInfoIDs[int64(1)] = info
	mb.updateSignalInfos[int64(1)] = info
	err := mb.ReplicateExternalWorkflowExecutionSignaled(event)
	assert.NoError(t, err)
	assert.NotNil(t, mb.deleteSignalInfos[int64(1)])
	_, ok := mb.pendingSignalInfoIDs[int64(1)]
	assert.False(t, ok)
	_, ok = mb.updateSignalInfos[int64(1)]
	assert.False(t, ok)
}

func Test__ReplicateSignalExternalWorkflowExecutionFailedEvent(t *testing.T) {
	mb := testMutableStateBuilder(t)
	event := &types.HistoryEvent{
		SignalExternalWorkflowExecutionFailedEventAttributes: &types.SignalExternalWorkflowExecutionFailedEventAttributes{
			InitiatedEventID: 1,
		},
	}
	info := &persistence.SignalInfo{
		InitiatedID:     1,
		SignalRequestID: "101",
	}
	mb.pendingSignalInfoIDs[int64(1)] = info
	mb.updateSignalInfos[int64(1)] = info
	err := mb.ReplicateSignalExternalWorkflowExecutionFailedEvent(event)
	assert.NoError(t, err)
	assert.NotNil(t, mb.deleteSignalInfos[int64(1)])
	_, ok := mb.pendingSignalInfoIDs[int64(1)]
	assert.False(t, ok)
	_, ok = mb.updateSignalInfos[int64(1)]
	assert.False(t, ok)
}

func Test__AddSignalRequested(t *testing.T) {
	mb := testMutableStateBuilder(t)
	requestID := "101"
	mb.pendingSignalRequestedIDs = nil
	mb.updateSignalRequestedIDs = nil
	mb.AddSignalRequested(requestID)
	assert.NotNil(t, mb.pendingSignalRequestedIDs[requestID])
	assert.NotNil(t, mb.updateSignalRequestedIDs[requestID])
}

func Test__DeleteSignalRequested(t *testing.T) {
	mb := testMutableStateBuilder(t)
	requestID := "101"
	mb.pendingSignalRequestedIDs[requestID] = struct{}{}
	mb.updateSignalRequestedIDs[requestID] = struct{}{}
	mb.DeleteSignalRequested(requestID)
	assert.NotNil(t, mb.deleteSignalRequestedIDs[requestID])
}

func Test__AddExternalWorkflowExecutionSignaled(t *testing.T) {
	mb := testMutableStateBuilder(t)
	t.Run("error workflow finished", func(t *testing.T) {
		mbCompleted := testMutableStateBuilder(t)
		mbCompleted.executionInfo.State = persistence.WorkflowStateCompleted
		_, err := mbCompleted.AddExternalWorkflowExecutionSignaled(1, "test-domain", "wid", "rid", []byte{10})
		assert.Error(t, err)
		assert.Equal(t, ErrWorkflowFinished, err)
	})
	t.Run("error getting signal info", func(t *testing.T) {
		_, err := mb.AddExternalWorkflowExecutionSignaled(1, "test-domain", "wid", "rid", []byte{10})
		assert.Error(t, err)
		assert.Equal(t, "add-externalworkflow-signal-requested-event operation failed", err.Error())
	})
	t.Run("success", func(t *testing.T) {
		si := &persistence.SignalInfo{
			InitiatedID: 1,
		}
		mb.pendingSignalInfoIDs[1] = si
		mb.hBuilder = NewHistoryBuilder(mb)
		event, err := mb.AddExternalWorkflowExecutionSignaled(1, "test-domain", "wid", "rid", []byte{10})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), event.ExternalWorkflowExecutionSignaledEventAttributes.GetInitiatedEventID())
	})
}

func Test__AddSignalExternalWorkflowExecutionFailedEvent(t *testing.T) {
	mb := testMutableStateBuilder(t)
	t.Run("error workflow finished", func(t *testing.T) {
		mbCompleted := testMutableStateBuilder(t)
		mbCompleted.executionInfo.State = persistence.WorkflowStateCompleted
		_, err := mbCompleted.AddSignalExternalWorkflowExecutionFailedEvent(1, 1, "test-domain", "wid", "rid", []byte{10}, types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted)
		assert.Error(t, err)
		assert.Equal(t, ErrWorkflowFinished, err)
	})
	t.Run("error getting signal info", func(t *testing.T) {
		_, err := mb.AddSignalExternalWorkflowExecutionFailedEvent(1, 1, "test-domain", "wid", "rid", []byte{10}, types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted)
		assert.Error(t, err)
		assert.Equal(t, "add-externalworkflow-signal-failed-event operation failed", err.Error())
	})
	t.Run("success", func(t *testing.T) {
		si := &persistence.SignalInfo{
			InitiatedID: 1,
		}
		mb.pendingSignalInfoIDs[1] = si
		mb.hBuilder = NewHistoryBuilder(mb)
		event, err := mb.AddSignalExternalWorkflowExecutionFailedEvent(1, 1, "test-domain", "wid", "rid", []byte{10}, types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), event.SignalExternalWorkflowExecutionFailedEventAttributes.GetInitiatedEventID())
	})
}

func Test__AddSignalExternalWorkflowExecutionInitiatedEvent(t *testing.T) {
	mb := testMutableStateBuilder(t)
	request := &types.SignalExternalWorkflowExecutionDecisionAttributes{
		Domain: constants.TestDomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: "wid",
			RunID:      "rid",
		},
		SignalName: "test-signal",
		Input:      make([]byte, 0),
	}
	mb.executionInfo = &persistence.WorkflowExecutionInfo{
		DomainID:   constants.TestDomainID,
		WorkflowID: "wid",
		RunID:      "rid",
	}
	t.Run("error workflow finished", func(t *testing.T) {
		mbCompleted := testMutableStateBuilder(t)
		mbCompleted.executionInfo.State = persistence.WorkflowStateCompleted
		_, _, err := mbCompleted.AddSignalExternalWorkflowExecutionInitiatedEvent(1, "101", request)
		assert.Error(t, err)
		assert.Equal(t, ErrWorkflowFinished, err)
	})
	t.Run("success", func(t *testing.T) {
		mb.hBuilder = NewHistoryBuilder(mb)
		event, si, err := mb.AddSignalExternalWorkflowExecutionInitiatedEvent(1, "101", request)
		assert.NoError(t, err)
		assert.Equal(t, request.Execution, event.SignalExternalWorkflowExecutionInitiatedEventAttributes.GetWorkflowExecution())
		assert.Equal(t, "101", si.SignalRequestID)
	})
}
