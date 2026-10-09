package execution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func testSemaphoreInfo(initiatedID int64) *persistence.SemaphoreInfo {
	return &persistence.SemaphoreInfo{
		Version:         1,
		InitiatedID:     initiatedID,
		SemaphoreName:   "my-semaphore",
		OwnerID:         "wid:rid:2",
		TokenID:         7,
		AcquireDeadline: time.Unix(1700000000, 0).UTC(),
	}
}

func Test__GetSemaphoreInfo(t *testing.T) {
	initiatedEventID := int64(2)
	info := testSemaphoreInfo(initiatedEventID)
	t.Run("hold not found", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		_, ok := mb.GetSemaphoreInfo(initiatedEventID)
		assert.False(t, ok)
	})
	t.Run("hold found", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		mb.pendingSemaphoreInfoIDs[initiatedEventID] = info
		result, ok := mb.GetSemaphoreInfo(initiatedEventID)
		assert.True(t, ok)
		assert.Equal(t, info, result)
	})
}

func Test__GetPendingSemaphoreInfos(t *testing.T) {
	t.Run("no holds", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		assert.Empty(t, mb.GetPendingSemaphoreInfos())
	})
	t.Run("holds present", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		info := testSemaphoreInfo(2)
		mb.pendingSemaphoreInfoIDs[2] = info
		assert.Equal(t, map[int64]*persistence.SemaphoreInfo{2: info}, mb.GetPendingSemaphoreInfos())
	})
}

func Test__UpsertSemaphoreInfo(t *testing.T) {
	t.Run("records the hold for the next flush", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		info := testSemaphoreInfo(2)
		mb.UpsertSemaphoreInfo(info)
		assert.Equal(t, info, mb.pendingSemaphoreInfoIDs[2])
		assert.Equal(t, info, mb.updateSemaphoreInfos[2])
	})
	t.Run("replaces an entry under the same id", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		mb.UpsertSemaphoreInfo(testSemaphoreInfo(2))
		granted := testSemaphoreInfo(2)
		granted.TokenID = 9
		mb.UpsertSemaphoreInfo(granted)
		assert.Len(t, mb.pendingSemaphoreInfoIDs, 1)
		assert.Equal(t, 9, mb.pendingSemaphoreInfoIDs[2].TokenID)
		assert.Equal(t, 9, mb.updateSemaphoreInfos[2].TokenID)
	})
	// Load installs whatever persistence returned, and a backend that does not store holds
	// returns nothing at all. Writing to that nil map panics without the allocation.
	t.Run("allocates when Load left the map nil", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		mb.pendingSemaphoreInfoIDs = nil
		info := testSemaphoreInfo(2)
		mb.UpsertSemaphoreInfo(info)
		assert.Equal(t, info, mb.pendingSemaphoreInfoIDs[2])
	})
}

func Test__DeleteSemaphoreInfo(t *testing.T) {
	t.Run("hold found", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		mb.UpsertSemaphoreInfo(testSemaphoreInfo(2))
		err := mb.DeleteSemaphoreInfo(2)
		assert.NoError(t, err)
		assert.NotContains(t, mb.pendingSemaphoreInfoIDs, int64(2))
		assert.NotContains(t, mb.updateSemaphoreInfos, int64(2))
		assert.Contains(t, mb.deleteSemaphoreInfos, int64(2))
	})
	// A missing id is logged as an inconsistency rather than returned as an error, and the id
	// is still queued for deletion so a row left behind by an earlier failure is cleaned up.
	t.Run("hold not found", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		err := mb.DeleteSemaphoreInfo(2)
		assert.NoError(t, err)
		assert.Contains(t, mb.deleteSemaphoreInfos, int64(2))
	})
}

func Test__CheckResettable_Semaphore(t *testing.T) {
	t.Run("no holds", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		assert.NoError(t, mb.CheckResettable())
	})
	t.Run("hold outstanding", func(t *testing.T) {
		mb := testMutableStateBuilder(t)
		mb.UpsertSemaphoreInfo(testSemaphoreInfo(2))
		err := mb.CheckResettable()
		assert.ErrorContains(t, err, "pending semaphore holds")
	})
}

// testSemaphoreMutableStateBuilder returns a builder for a running workflow whose next event id
// is 5, so an event appended immediately gets id 5.
func testSemaphoreMutableStateBuilder(t *testing.T) *mutableStateBuilder {
	mb := testMutableStateBuilder(t)
	mb.executionInfo = &persistence.WorkflowExecutionInfo{
		DomainID:    "domain-id",
		WorkflowID:  "wid",
		RunID:       "rid",
		NextEventID: 5,
	}
	mb.hBuilder = NewHistoryBuilder(mb)
	return mb
}

func Test__AddSemaphoreAcquireInitiatedEvent(t *testing.T) {
	t.Run("a closed run cannot start an acquire", func(t *testing.T) {
		mb := testSemaphoreMutableStateBuilder(t)
		mb.executionInfo.State = persistence.WorkflowStateCompleted
		_, _, err := mb.AddSemaphoreAcquireInitiatedEvent(4, "my-semaphore", 30)
		assert.Equal(t, ErrWorkflowFinished, err)
		assert.Empty(t, mb.pendingSemaphoreInfoIDs)
	})
	t.Run("records the event and starts the hold", func(t *testing.T) {
		mb := testSemaphoreMutableStateBuilder(t)
		event, semaphoreInfo, err := mb.AddSemaphoreAcquireInitiatedEvent(4, "my-semaphore", 30)
		require.NoError(t, err)

		assert.Equal(t, int64(5), event.ID, "the event takes NextEventID, since acquire events are never buffered")
		assert.Equal(t, types.EventTypeSemaphoreAcquireInitiated, event.GetEventType())
		assert.Equal(t, &types.SemaphoreAcquireInitiatedEventAttributes{
			SemaphoreName:                "my-semaphore",
			WaitTimeoutSeconds:           common.Int32Ptr(30),
			DecisionTaskCompletedEventID: 4,
		}, event.SemaphoreAcquireInitiatedEventAttributes)

		assert.Equal(t, int64(5), semaphoreInfo.InitiatedID)
		assert.Equal(t, semaphoreInfo, mb.pendingSemaphoreInfoIDs[5])
		assert.Equal(t, semaphoreInfo, mb.updateSemaphoreInfos[5])
	})
}

func Test__ReplicateSemaphoreAcquireInitiatedEvent(t *testing.T) {
	timestamp := time.Unix(1700000000, 0)
	mb := testSemaphoreMutableStateBuilder(t)
	event := &types.HistoryEvent{
		ID:        7,
		Version:   3,
		Timestamp: common.Int64Ptr(timestamp.UnixNano()),
		EventType: types.EventTypeSemaphoreAcquireInitiated.Ptr(),
		SemaphoreAcquireInitiatedEventAttributes: &types.SemaphoreAcquireInitiatedEventAttributes{
			SemaphoreName:      "my-semaphore",
			WaitTimeoutSeconds: common.Int32Ptr(30),
		},
	}

	semaphoreInfo, err := mb.ReplicateSemaphoreAcquireInitiatedEvent(event)
	require.NoError(t, err)

	want := &persistence.SemaphoreInfo{
		Version:       3,
		InitiatedID:   7,
		SemaphoreName: "my-semaphore",
		OwnerID:       "3:wid:rid:7",
		// From the event timestamp, so replay gets the same deadline.
		AcquireDeadline: timestamp.Add(30 * time.Second),
	}
	assert.Equal(t, want, semaphoreInfo)
	assert.Equal(t, want, mb.pendingSemaphoreInfoIDs[7])
	assert.Equal(t, want, mb.updateSemaphoreInfos[7])
}

// Tests which grants AddSemaphoreAcquiredEvent records and which it refuses.
func Test__AddSemaphoreAcquiredEvent(t *testing.T) {
	errRefused := &types.InternalServiceError{Message: "add-semaphore-acquired-event operation failed"}

	tests := []struct {
		name          string
		holdTokenID   *int // nil means no hold is recorded
		finished      bool
		grantTokenID  int32
		wantErr       error
		wantHoldToken int
	}{
		{
			name:          "a hold still waiting gets the token",
			holdTokenID:   common.IntPtr(0),
			grantTokenID:  7,
			wantHoldToken: 7,
		},
		{
			name:          "a hold with a different token gets the new token",
			holdTokenID:   common.IntPtr(3),
			grantTokenID:  7,
			wantHoldToken: 7,
		},
		{
			name:          "the same token granted again is refused",
			holdTokenID:   common.IntPtr(7),
			grantTokenID:  7,
			wantErr:       errRefused,
			wantHoldToken: 7,
		},
		{
			name:         "a grant with no matching hold is refused",
			grantTokenID: 7,
			wantErr:      errRefused,
		},
		{
			name:          "a closed run cannot record a grant",
			holdTokenID:   common.IntPtr(0),
			finished:      true,
			grantTokenID:  7,
			wantErr:       ErrWorkflowFinished,
			wantHoldToken: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mb := testSemaphoreMutableStateBuilder(t)
			if tc.holdTokenID != nil {
				hold := testSemaphoreInfo(2)
				hold.TokenID = *tc.holdTokenID
				mb.pendingSemaphoreInfoIDs[2] = hold
			}
			if tc.finished {
				mb.executionInfo.State = persistence.WorkflowStateCompleted
			}

			event, err := mb.AddSemaphoreAcquiredEvent(2, tc.grantTokenID)

			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
				assert.Nil(t, event)
				assert.Empty(t, mb.hBuilder.history, "a refused grant writes no event")
			} else {
				require.NoError(t, err)
				assert.Equal(t, types.EventTypeSemaphoreAcquired, event.GetEventType())
				assert.Equal(t, &types.SemaphoreAcquiredEventAttributes{
					TokenID:          tc.grantTokenID,
					InitiatedEventID: 2,
				}, event.SemaphoreAcquiredEventAttributes)
			}
			if tc.holdTokenID != nil {
				assert.Equal(t, tc.wantHoldToken, mb.pendingSemaphoreInfoIDs[2].TokenID)
			}
		})
	}
}

func Test__ReplicateSemaphoreAcquiredEvent(t *testing.T) {
	event := &types.HistoryEvent{
		ID:        9,
		Version:   4,
		EventType: types.EventTypeSemaphoreAcquired.Ptr(),
		SemaphoreAcquiredEventAttributes: &types.SemaphoreAcquiredEventAttributes{
			TokenID:          7,
			InitiatedEventID: 2,
		},
	}

	t.Run("the hold gets the granted token and is marked to be saved", func(t *testing.T) {
		mb := testSemaphoreMutableStateBuilder(t)
		hold := testSemaphoreInfo(2)
		hold.TokenID = 0
		mb.pendingSemaphoreInfoIDs[2] = hold

		require.NoError(t, mb.ReplicateSemaphoreAcquiredEvent(event))
		assert.Equal(t, 7, mb.pendingSemaphoreInfoIDs[2].TokenID)
		assert.Equal(t, int64(4), mb.pendingSemaphoreInfoIDs[2].Version)
		assert.Equal(t, mb.pendingSemaphoreInfoIDs[2], mb.updateSemaphoreInfos[2])
	})
	t.Run("an event with no matching hold returns an error", func(t *testing.T) {
		mb := testSemaphoreMutableStateBuilder(t)
		assert.Equal(t, ErrMissingSemaphoreInfo, mb.ReplicateSemaphoreAcquiredEvent(event))
	})
}

// Tests that the holds rebuilt from history match the holds the Add methods recorded. A standby
// cluster or a rebuild only has the events, so each hold must come from its events alone.
func Test__SemaphoreEventsReplayToTheSameHolds(t *testing.T) {
	original := testSemaphoreMutableStateBuilder(t)
	acquireAEvent, _, err := original.AddSemaphoreAcquireInitiatedEvent(4, "sem-a", 30)
	require.NoError(t, err)
	acquireBEvent, _, err := original.AddSemaphoreAcquireInitiatedEvent(4, "sem-b", 60)
	require.NoError(t, err)
	_, err = original.AddSemaphoreAcquiredEvent(acquireAEvent.ID, 3)
	require.NoError(t, err)
	// acquireA is granted again with a new token, as after a failover.
	_, err = original.AddSemaphoreAcquiredEvent(acquireAEvent.ID, 8)
	require.NoError(t, err)
	// acquireB is never granted, so it is still waiting.
	// Grant events are buffered until the transaction closes, which flushes them and gives them IDs.
	require.NoError(t, original.FlushBufferedEvents())

	rebuilt := testSemaphoreMutableStateBuilder(t)
	// Replay runs later than the original write, so a hold that reads the clock would differ.
	rebuilt.timeSource.(clock.MockedTimeSource).Advance(time.Hour)
	// ApplyEvents adds each event to the version history, so the builder needs an empty one.
	rebuilt.versionHistories = persistence.NewVersionHistories(&persistence.VersionHistory{})
	info := original.executionInfo
	_, err = NewStateBuilder(rebuilt.shard, rebuilt.logger, rebuilt).ApplyEvents(
		info.DomainID, "request-id", types.WorkflowExecution{WorkflowID: info.WorkflowID, RunID: info.RunID}, original.hBuilder.history, nil)
	require.NoError(t, err)

	assert.Len(t, original.pendingSemaphoreInfoIDs, 2)
	assert.Equal(t, 8, original.pendingSemaphoreInfoIDs[acquireAEvent.ID].TokenID, "acquireA holds its latest token")
	assert.Equal(t, 0, original.pendingSemaphoreInfoIDs[acquireBEvent.ID].TokenID, "acquireB has no token yet")
	assert.Equal(t, original.pendingSemaphoreInfoIDs, rebuilt.pendingSemaphoreInfoIDs)
}

func Test__ReplicateSemaphoreReleasedEvent(t *testing.T) {
	mb := testSemaphoreMutableStateBuilder(t)
	mb.UpsertSemaphoreInfo(testSemaphoreInfo(2))

	err := mb.ReplicateSemaphoreReleasedEvent(&types.HistoryEvent{
		ID:        10,
		EventType: types.EventTypeSemaphoreReleased.Ptr(),
		SemaphoreReleasedEventAttributes: &types.SemaphoreReleasedEventAttributes{
			TokenID:          7,
			InitiatedEventID: 2,
		},
	})
	require.NoError(t, err)
	assert.NotContains(t, mb.pendingSemaphoreInfoIDs, int64(2))
	assert.Contains(t, mb.deleteSemaphoreInfos, int64(2))
}
