package semaphore

import (
	"sync"
	"sync/atomic"
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

// registerForTest puts mgr in the registry the way the engine does, and fails if something else
// was already held for that bucket.
func registerForTest(t *testing.T, r SemaphoreRegistry, mgr Manager) {
	t.Helper()
	got, err := r.GetOrCreate(mgr.Identifier(), func() (Manager, error) { return mgr, nil })
	require.NoError(t, err)
	require.Same(t, mgr, got, "the bucket was already held by another manager")
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
	registerForTest(t, r, mgr)

	got, ok := r.ManagerByIdentifier(id)
	require.True(t, ok)
	assert.Same(t, mgr, got)

	assert.True(t, r.Unregister(mgr), "unregistering the manager actually held reports true")
	_, ok = r.ManagerByIdentifier(id)
	assert.False(t, ok)

	assert.False(t, r.Unregister(mgr), "unregistering twice reports false")
}

// Tests that Unregister drops only the manager it is given. Two managers can share a bucket id,
// so a stale one tearing down must not evict the one now serving that bucket.
func TestSemaphoreRegistry_UnregisterIgnoresAStaleManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := NewSemaphoreRegistry()
	id := mustNewIdentifierForTest(t, 0)

	stopping := newMockManagerWithID(t, ctrl, id)
	current := newMockManagerWithID(t, ctrl, id)

	// The order a ring change produces: the bucket is unregistered, a request loads a fresh
	// manager for it, and only then does the first manager finish tearing down.
	registerForTest(t, r, stopping)
	require.True(t, r.Unregister(stopping))
	registerForTest(t, r, current)

	assert.False(t, r.Unregister(stopping), "the registry no longer holds this manager")

	got, ok := r.ManagerByIdentifier(id)
	require.True(t, ok, "the manager now serving the bucket must survive")
	assert.Same(t, current, got)
}

// Tests that AllManagers hands back the caller's own slice. Shutdown iterates it while calling
// Stop, so it has to stay valid while the registry underneath it changes.
func TestSemaphoreRegistry_AllManagersIsASnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := NewSemaphoreRegistry()
	registerForTest(t, r, newMockManagerWithID(t, ctrl, mustNewIdentifierForTest(t, 0)))
	registerForTest(t, r, newMockManagerWithID(t, ctrl, mustNewIdentifierForTest(t, 1)))

	snapshot := r.AllManagers()
	require.Len(t, snapshot, 2)

	for _, mgr := range snapshot {
		assert.True(t, r.Unregister(mgr))
	}
	assert.Len(t, snapshot, 2, "emptying the registry must not change a snapshot already taken")
	assert.Empty(t, r.AllManagers())
}

func TestSemaphoreRegistry_GetOrCreateBuildsOnlyWhenThereIsNothingHeld(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := NewSemaphoreRegistry()
	id := mustNewIdentifierForTest(t, 0)
	mgr := newMockManagerWithID(t, ctrl, id)

	calls := 0
	build := func() (Manager, error) {
		calls++
		return mgr, nil
	}

	got, err := r.GetOrCreate(id, build)
	require.NoError(t, err)
	assert.Same(t, mgr, got)

	got, err = r.GetOrCreate(id, build)
	require.NoError(t, err)
	assert.Same(t, mgr, got)
	assert.Equal(t, 1, calls, "a bucket already held is never built again")

	held, ok := r.ManagerByIdentifier(id)
	require.True(t, ok, "a built manager is registered, not just returned")
	assert.Same(t, mgr, held)
}

// Tests that a failed build leaves nothing behind, so the next request tries again rather than
// finding a manager that was never usable.
func TestSemaphoreRegistry_GetOrCreateHoldsNothingWhenTheBuildFails(t *testing.T) {
	r := NewSemaphoreRegistry()
	id := mustNewIdentifierForTest(t, 0)

	got, err := r.GetOrCreate(id, func() (Manager, error) {
		return nil, assert.AnError
	})
	assert.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, got)

	_, ok := r.ManagerByIdentifier(id)
	assert.False(t, ok)
}

// Tests the guarantee the engine relies on: however many requests arrive for a cold bucket at
// once, one manager is built and they all get it. A second manager would run a second idle clock
// and a second free-set over the same partition.
func TestSemaphoreRegistry_GetOrCreateBuildsOnceUnderConcurrentCallers(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := NewSemaphoreRegistry()
	id := mustNewIdentifierForTest(t, 0)

	const callers = 32
	var builds int32
	results := make([]Manager, callers)

	var start, done sync.WaitGroup
	start.Add(1)
	done.Add(callers)
	for i := 0; i < callers; i++ {
		go func(i int) {
			defer done.Done()
			start.Wait()
			// A distinct manager per build, so a caller handed someone else's build is visible.
			got, err := r.GetOrCreate(id, func() (Manager, error) {
				atomic.AddInt32(&builds, 1)
				return newMockManagerWithID(t, ctrl, id), nil
			})
			assert.NoError(t, err)
			results[i] = got
		}(i)
	}
	start.Done()
	done.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&builds), "the build runs under the lock, so it runs once")
	held, ok := r.ManagerByIdentifier(id)
	require.True(t, ok)
	for _, got := range results {
		assert.Same(t, held, got, "every caller must get the one manager that was built")
	}
}
