package semaphore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func mustNewIdentifierForTest(t *testing.T, bucket int) Identifier {
	t.Helper()
	id, err := NewIdentifier("domain-1", "sem-1", bucket)
	require.NoError(t, err)
	return id
}

func newMockManagerWithID(t *testing.T, ctrl *gomock.Controller, id Identifier) *MockManager {
	t.Helper()
	mgr := NewMockManager(ctrl)
	mgr.EXPECT().Identifier().Return(id).AnyTimes()
	return mgr
}

// Tests the plain path: an empty registry holds nothing, a registered manager can be looked up,
// and unregistering it takes it back out.
func TestSemaphoreRegistry_RegisterLookupAndUnregister(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := NewSemaphoreRegistry()
	id := mustNewIdentifierForTest(t, 0)

	_, ok := r.ManagerByIdentifier(id)
	assert.False(t, ok, "an empty registry holds nothing")

	mgr := newMockManagerWithID(t, ctrl, id)
	r.Register(mgr)

	got, ok := r.ManagerByIdentifier(id)
	require.True(t, ok)
	assert.Same(t, mgr, got)

	assert.True(t, r.Unregister(mgr), "unregistering the manager actually held reports true")
	_, ok = r.ManagerByIdentifier(id)
	assert.False(t, ok)

	assert.False(t, r.Unregister(mgr), "unregistering twice reports false")
}

func TestSemaphoreRegistry_UnregisterWillNotEvictAReplacement(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := NewSemaphoreRegistry()
	id := mustNewIdentifierForTest(t, 0)

	old := newMockManagerWithID(t, ctrl, id)
	replacement := newMockManagerWithID(t, ctrl, id)

	r.Register(old)
	r.Register(replacement)

	assert.False(t, r.Unregister(old), "the old manager is no longer the one held")

	got, ok := r.ManagerByIdentifier(id)
	require.True(t, ok, "the replacement must survive the late teardown")
	assert.Same(t, replacement, got)
}

// Tests that AllManagers hands back the caller's own slice. Shutdown iterates it while calling
// Stop, so it has to stay valid while the registry underneath it changes.
func TestSemaphoreRegistry_AllManagersIsASnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := NewSemaphoreRegistry()
	r.Register(newMockManagerWithID(t, ctrl, mustNewIdentifierForTest(t, 0)))
	r.Register(newMockManagerWithID(t, ctrl, mustNewIdentifierForTest(t, 1)))

	snapshot := r.AllManagers()
	require.Len(t, snapshot, 2)

	for _, mgr := range snapshot {
		assert.True(t, r.Unregister(mgr))
	}
	assert.Len(t, snapshot, 2, "emptying the registry must not change a snapshot already taken")
	assert.Empty(t, r.AllManagers())
}
