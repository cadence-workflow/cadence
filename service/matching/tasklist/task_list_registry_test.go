package tasklist

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/metrics"
	metricsmocks "github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/persistence"
)

func mustNewIdentifierForTest(t *testing.T, domainID, taskListName string) *Identifier {
	t.Helper()
	id, err := NewIdentifier(domainID, taskListName, persistence.TaskListTypeDecision)
	require.NoError(t, err)
	return id
}

func newMockManagerWithID(t *testing.T, ctrl *gomock.Controller, id *Identifier) *MockManager {
	t.Helper()
	mgr := NewMockManager(ctrl)
	mgr.EXPECT().TaskListID().Return(id).AnyTimes()
	return mgr
}

func TestTaskListRegistry_RegisterLookupAndUnregister(t *testing.T) {
	ctrl := gomock.NewController(t)
	metricsClient := metricsmocks.Client{}
	metricsScope := metricsmocks.Scope{}
	metricsClient.On("Scope", metrics.MatchingTaskListMgrScope).Return(&metricsScope)
	registry := NewTaskListRegistry(&metricsClient)

	id := mustNewIdentifierForTest(t, "domain-a", "task-list-a")

	initialMgr := newMockManagerWithID(t, ctrl, id)
	updatedMgr := newMockManagerWithID(t, ctrl, id)

	metricsScope.On("UpdateGauge", metrics.TaskListManagersGauge, float64(1)).Once()
	registry.Register(*id, initialMgr)
	got, ok := registry.ManagerByTaskListIdentifier(*id)
	require.True(t, ok)
	assert.Equal(t, initialMgr, got)

	// Re-register with the same identifier should replace the manager.
	metricsScope.On("UpdateGauge", metrics.TaskListManagersGauge, float64(1)).Once()
	registry.Register(*id, updatedMgr)
	got, ok = registry.ManagerByTaskListIdentifier(*id)
	require.True(t, ok)
	assert.Equal(t, updatedMgr, got)

	// Unregister should not remove a replaced/stale manager.
	assert.False(t, registry.Unregister(initialMgr))
	got, ok = registry.ManagerByTaskListIdentifier(*id)
	require.True(t, ok)
	assert.Equal(t, updatedMgr, got)

	// Unregistering the current manager should remove the entry.
	metricsScope.On("UpdateGauge", metrics.TaskListManagersGauge, float64(0)).Once()
	assert.True(t, registry.Unregister(updatedMgr))
	_, ok = registry.ManagerByTaskListIdentifier(*id)
	assert.False(t, ok)

	impl := registry.(*taskListRegistryImpl)
	assert.Empty(t, impl.taskListsByDomainID, "should be empty because there are no other task lists for this domain")
	assert.Empty(t, impl.taskListsByTaskListName, "should be empty because there are no other task lists for this task list name")

	metricsClient.AssertExpectations(t)
	metricsScope.AssertExpectations(t)
}

func TestTaskListRegistry_Filters(t *testing.T) {
	ctrl := gomock.NewController(t)
	registry := NewTaskListRegistry(metrics.NewNoopMetricsClient())

	domainA1 := mustNewIdentifierForTest(t, "domain-a", "shared-name")
	domainA2 := mustNewIdentifierForTest(t, "domain-a", "other-name")
	domainB1 := mustNewIdentifierForTest(t, "domain-b", "shared-name")

	mgrA1 := newMockManagerWithID(t, ctrl, domainA1)
	mgrA2 := newMockManagerWithID(t, ctrl, domainA2)
	mgrB1 := newMockManagerWithID(t, ctrl, domainB1)

	registry.Register(*domainA1, mgrA1)
	registry.Register(*domainA2, mgrA2)
	registry.Register(*domainB1, mgrB1)

	t.Run("all managers", func(t *testing.T) {
		assert.ElementsMatch(t, []Manager{mgrA1, mgrA2, mgrB1}, registry.AllManagers())
	})

	t.Run("managers by domain", func(t *testing.T) {
		assert.ElementsMatch(t, []Manager{mgrA1, mgrA2}, registry.ManagersByDomainID("domain-a"))
		assert.ElementsMatch(t, []Manager{mgrB1}, registry.ManagersByDomainID("domain-b"))
		assert.Empty(t, registry.ManagersByDomainID("missing-domain"))
	})

	t.Run("managers by task list name", func(t *testing.T) {
		assert.ElementsMatch(
			t,
			[]Manager{mgrA1, mgrB1},
			registry.ManagersByTaskListName("shared-name"),
		)
		assert.ElementsMatch(t, []Manager{mgrA2}, registry.ManagersByTaskListName("other-name"))
		assert.Empty(t, registry.ManagersByTaskListName("missing-name"))
	})
}
