package execution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/persistence"
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
