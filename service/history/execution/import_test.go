package execution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/events"
)

func TestIsImportedRun(t *testing.T) {
	source := &types.WorkflowExecution{WorkflowID: "source-workflow-id", RunID: "source-run-id"}
	tests := map[string]struct {
		startAttributes *types.WorkflowExecutionStartedEventAttributes
		runID           string
		want            bool
	}{
		"imported run": {
			startAttributes: &types.WorkflowExecutionStartedEventAttributes{ImportSourceExecution: source, OriginalExecutionRunID: "run-id"},
			runID:           "run-id",
			want:            true,
		},
		"reset run of an imported run": {
			startAttributes: &types.WorkflowExecutionStartedEventAttributes{ImportSourceExecution: source, OriginalExecutionRunID: "run-id"},
			runID:           "reset-run-id",
			want:            false,
		},
		"run that is not imported": {
			startAttributes: &types.WorkflowExecutionStartedEventAttributes{OriginalExecutionRunID: "run-id"},
			runID:           "run-id",
			want:            false,
		},
		"no start attributes": {
			startAttributes: nil,
			runID:           "run-id",
			want:            false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsImportedRun(tc.startAttributes, tc.runID))
		})
	}
}

func TestSetImportInProgress(t *testing.T) {
	controller := gomock.NewController(t)
	mutableState := NewMockMutableState(controller)
	executionInfo := &persistence.WorkflowExecutionInfo{State: persistence.WorkflowStateRunning}
	mutableState.EXPECT().GetExecutionInfo().Return(executionInfo).AnyTimes()
	source := &types.WorkflowExecution{WorkflowID: "source-workflow-id", RunID: "source-run-id"}

	SetImportInProgress(mutableState, &types.WorkflowExecutionStartedEventAttributes{
		ImportSourceExecution: source,
		ImportTimestamp:       common.Int64Ptr(1000),
	})

	assert.Equal(t, &persistence.WorkflowExecutionInfo{
		State:                 persistence.WorkflowStatePreparing,
		ImportSourceExecution: source,
		ImportTimestamp:       time.Unix(0, 1000),
		ImportInProgress:      true,
	}, executionInfo)
}

func TestIsRunningOrImporting(t *testing.T) {
	tests := map[string]struct {
		running bool
		state   int
		want    bool
	}{
		"running":            {running: true, state: persistence.WorkflowStateRunning, want: true},
		"import in progress": {running: false, state: persistence.WorkflowStatePreparing, want: true},
		"completed":          {running: false, state: persistence.WorkflowStateCompleted, want: false},
		"zombie":             {running: false, state: persistence.WorkflowStateZombie, want: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := gomock.NewController(t)
			mutableState := NewMockMutableState(controller)
			mutableState.EXPECT().IsWorkflowExecutionRunning().Return(tc.running).AnyTimes()
			mutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{State: tc.state}).AnyTimes()
			assert.Equal(t, tc.want, IsRunningOrImporting(mutableState))
		})
	}
}

// TestCloseEvents_ImportInProgress checks every close event handler: an import in progress, current
// or a zombie, keeps its state with the close status and gets no close tasks. Other workflows,
// zombies included, become Completed and get the close tasks.
func TestCloseEvents_ImportInProgress(t *testing.T) {
	handlers := map[string]struct {
		closeStatus int
		replicate   func(msb *mutableStateBuilder, event *types.HistoryEvent) error
	}{
		"completed": {
			closeStatus: persistence.WorkflowCloseStatusCompleted,
			replicate: func(msb *mutableStateBuilder, event *types.HistoryEvent) error {
				return msb.ReplicateWorkflowExecutionCompletedEvent(10, event)
			},
		},
		"failed": {
			closeStatus: persistence.WorkflowCloseStatusFailed,
			replicate: func(msb *mutableStateBuilder, event *types.HistoryEvent) error {
				return msb.ReplicateWorkflowExecutionFailedEvent(10, event)
			},
		},
		"canceled": {
			closeStatus: persistence.WorkflowCloseStatusCanceled,
			replicate: func(msb *mutableStateBuilder, event *types.HistoryEvent) error {
				return msb.ReplicateWorkflowExecutionCanceledEvent(10, event)
			},
		},
		"terminated": {
			closeStatus: persistence.WorkflowCloseStatusTerminated,
			replicate: func(msb *mutableStateBuilder, event *types.HistoryEvent) error {
				return msb.ReplicateWorkflowExecutionTerminatedEvent(10, event)
			},
		},
		"continued as new": {
			closeStatus: persistence.WorkflowCloseStatusContinuedAsNew,
			replicate: func(msb *mutableStateBuilder, event *types.HistoryEvent) error {
				return msb.ReplicateWorkflowExecutionContinuedAsNewEvent(10, "domain-id", event)
			},
		},
		"timed out": {
			closeStatus: persistence.WorkflowCloseStatusTimedOut,
			replicate: func(msb *mutableStateBuilder, event *types.HistoryEvent) error {
				return msb.ReplicateWorkflowExecutionTimedoutEvent(10, event)
			},
		},
	}
	states := map[string]struct {
		state            int
		importInProgress bool
		wantState        int
		wantCloseTasks   bool
	}{
		"running workflow": {
			state:          persistence.WorkflowStateRunning,
			wantState:      persistence.WorkflowStateCompleted,
			wantCloseTasks: true,
		},
		"zombie that is not an import": {
			state:          persistence.WorkflowStateZombie,
			wantState:      persistence.WorkflowStateCompleted,
			wantCloseTasks: true,
		},
		"import in progress": {
			state:            persistence.WorkflowStatePreparing,
			importInProgress: true,
			wantState:        persistence.WorkflowStatePreparing,
			wantCloseTasks:   false,
		},
		"zombie import": {
			state:            persistence.WorkflowStateZombie,
			importInProgress: true,
			wantState:        persistence.WorkflowStateZombie,
			wantCloseTasks:   false,
		},
	}

	for handlerName, handler := range handlers {
		for stateName, state := range states {
			t.Run(handlerName+" on "+stateName, func(t *testing.T) {
				controller := gomock.NewController(t)
				taskGenerator := NewMockMutableStateTaskGenerator(controller)
				eventsCache := events.NewMockCache(controller)
				eventsCache.EXPECT().PutEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				event := &types.HistoryEvent{ID: 11}
				if state.wantCloseTasks {
					taskGenerator.EXPECT().GenerateWorkflowCloseTasks(event, gomock.Any()).Return(nil).Times(1)
				}
				msb := &mutableStateBuilder{
					executionInfo: &persistence.WorkflowExecutionInfo{
						State:            state.state,
						CloseStatus:      persistence.WorkflowCloseStatusNone,
						ImportInProgress: state.importInProgress,
					},
					taskGenerator: taskGenerator,
					eventsCache:   eventsCache,
					config:        config.NewForTest(),
					domainEntry:   cache.NewLocalDomainCacheEntryForTest(&persistence.DomainInfo{ID: "domain-id", Name: "domain"}, &persistence.DomainConfig{}, "cluster"),
				}

				require.NoError(t, handler.replicate(msb, event))
				assert.Equal(t, state.wantState, msb.executionInfo.State)
				assert.Equal(t, handler.closeStatus, msb.executionInfo.CloseStatus)
				assert.Equal(t, int64(10), msb.executionInfo.CompletionEventBatchID)
			})
		}
	}
}
