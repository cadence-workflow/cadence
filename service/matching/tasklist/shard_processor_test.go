package tasklist

import (
	"testing"
	"time"

	smtypes "github.com/cadence-workflow/shard-manager/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
)

func mustNewIdentifier(domainID, name string, taskType int) *Identifier {
	id, err := NewIdentifier(domainID, name, taskType)
	if err != nil {
		panic(err)
	}
	return id
}

var testIdentifier = mustNewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)

type shardProcessorTestData struct {
	mockRegistry   *MockTaskListRegistry
	shardProcessor ShardProcessor
}

func newShardProcessorTestData(t *testing.T, taskListID *Identifier) shardProcessorTestData {
	ctrl := gomock.NewController(t)

	mockRegistry := NewMockTaskListRegistry(ctrl)
	mockRegistry.EXPECT().ManagersByTaskListName(taskListID.GetName()).Return([]Manager{}).AnyTimes()

	shardCount := 100
	taskListDifferentiator := matching.NewTaskListDifferentiator(
		metrics.NewNoopMetricsClient(),
		dynamicproperties.GetBoolPropertyFn(false),
		dynamicproperties.GetBoolPropertyFn(false),
		dynamicproperties.GetBoolPropertyFn(false),
		dynamicproperties.GetIntPropertyFn(100),
		shardCount,
	)

	params := ShardProcessorParams{
		ShardID:                taskListID.GetName(),
		TaskListsRegistry:      mockRegistry,
		ReportTTL:              1 * time.Millisecond,
		TimeSource:             clock.NewRealTimeSource(),
		TaskListDifferentiator: taskListDifferentiator,
	}

	shardProcessor, err := NewShardProcessor(params)
	require.NoError(t, err)
	return shardProcessorTestData{
		mockRegistry:   mockRegistry,
		shardProcessor: shardProcessor,
	}
}

func TestNewShardProcessorFailsWithEmptyParams(t *testing.T) {
	params := ShardProcessorParams{}
	sp, err := NewShardProcessor(params)
	require.Nil(t, sp)
	require.Error(t, err)
}

func TestGetShardReport(t *testing.T) {
	td := newShardProcessorTestData(t, testIdentifier)

	shardReport := td.shardProcessor.GetShardReport()
	require.NotNil(t, shardReport)
	require.Equal(t, float64(0), shardReport.ShardLoad)
	require.Equal(t, smtypes.ShardStatusREADY, shardReport.Status)
}

func TestSetShardStatus(t *testing.T) {
	defer goleak.VerifyNone(t)
	td := newShardProcessorTestData(t, testIdentifier)

	td.shardProcessor.SetShardStatus(smtypes.ShardStatusREADY)
	shardReport := td.shardProcessor.GetShardReport()
	require.NotNil(t, shardReport)
	require.Equal(t, float64(0), shardReport.ShardLoad)
	require.Equal(t, smtypes.ShardStatusREADY, shardReport.Status)
}

func TestGetShardReportCachesLoadForReportTTL(t *testing.T) {
	ctrl := gomock.NewController(t)
	timeSource := clock.NewMockedTimeSource()
	reportTTL := time.Minute

	initialQPS := 10.0
	updatedQPS := 20.0

	tlManager := NewMockManager(ctrl)
	mockRegistry := NewMockTaskListRegistry(ctrl)
	mockRegistry.EXPECT().ManagersByTaskListName(testIdentifier.GetName()).Return([]Manager{tlManager}).Times(2)

	shardCount := 100
	taskListDifferentiator := membership.NewTaskListDifferentiator(
		dynamicproperties.GetBoolPropertyFn(false),
		dynamicproperties.GetBoolPropertyFn(false),
		membership.StaticPercentageOnboarded(100),
		shardCount,
	)

	params := ShardProcessorParams{
		ShardID:                testIdentifier.GetName(),
		TaskListsRegistry:      mockRegistry,
		ReportTTL:              reportTTL,
		TimeSource:             timeSource,
		TaskListDifferentiator: taskListDifferentiator,
	}
	shardProcessor, err := NewShardProcessor(params)
	require.NoError(t, err)

	// The first report after the TTL computes the load.
	timeSource.Advance(reportTTL + time.Second)
	tlManager.EXPECT().QueriesPerSecond().Return(initialQPS)
	shardReport := shardProcessor.GetShardReport()
	require.Equal(t, initialQPS, shardReport.ShardLoad)

	// Within the TTL, the cached load is reused, but a status change is reported right away.
	timeSource.Advance(reportTTL / 2)
	shardProcessor.SetShardStatus(smtypes.ShardStatusDONE)
	shardReport = shardProcessor.GetShardReport()
	require.Equal(t, initialQPS, shardReport.ShardLoad)
	require.Equal(t, smtypes.ShardStatusDONE, shardReport.Status)

	// Once the TTL has passed since the last computation, the load is recomputed.
	timeSource.Advance(reportTTL)
	tlManager.EXPECT().QueriesPerSecond().Return(updatedQPS)
	shardReport = shardProcessor.GetShardReport()
	require.Equal(t, updatedQPS, shardReport.ShardLoad)
}

func TestShardProcessor_ShardByHashSkipsTaskListsOwnedByHashRing(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	shardID := "77"
	shardCount := 100
	reportTTL := time.Minute
	timeSource := clock.NewMockedTimeSource()

	// The decision and activity task lists share a name, so they are in the same shard.
	decisionTaskListID := mustNewIdentifier("domain-id", "orders", persistence.TaskListTypeDecision)
	require.Equal(t, shardID, matching.TaskListToShard(decisionTaskListID.GetDomainID(), decisionTaskListID.GetName(), shardCount))
	decisionTaskListQPS := 2.0
	decisionTaskList := NewMockManager(ctrl)
	decisionTaskList.EXPECT().TaskListID().Return(decisionTaskListID).AnyTimes()
	decisionTaskList.EXPECT().QueriesPerSecond().Return(decisionTaskListQPS)
	decisionTaskList.EXPECT().Stop()

	activityTaskListID := mustNewIdentifier("domain-id", "orders", persistence.TaskListTypeActivity)
	activityTaskListQPS := 3.0
	activityTaskList := NewMockManager(ctrl)
	activityTaskList.EXPECT().TaskListID().Return(activityTaskListID).AnyTimes()
	activityTaskList.EXPECT().QueriesPerSecond().Return(activityTaskListQPS)
	activityTaskList.EXPECT().Stop()

	// The shard must not stop a task list the hash ring owns, or count its load. Short-lived task
	// lists have a UUID in their name, and the differentiator leaves them on the hash ring even
	// though this one hashes to the same shard.
	hashRingTaskListID := mustNewIdentifier("domain-id", "f47ac10b-58cc-4372-a567-0e02b2c3d209", persistence.TaskListTypeDecision)
	require.Equal(t, shardID, matching.TaskListToShard(hashRingTaskListID.GetDomainID(), hashRingTaskListID.GetName(), shardCount))
	hashRingTaskList := NewMockManager(ctrl)
	hashRingTaskList.EXPECT().TaskListID().Return(hashRingTaskListID).AnyTimes()

	mockRegistry := NewMockTaskListRegistry(ctrl)
	mockRegistry.EXPECT().ManagersByShard(shardID).Return([]Manager{decisionTaskList, activityTaskList, hashRingTaskList}).Times(2)

	taskListDifferentiator := matching.NewTaskListDifferentiator(
		metrics.NewNoopMetricsClient(),
		dynamicproperties.GetBoolPropertyFn(true),
		dynamicproperties.GetBoolPropertyFn(true),
		dynamicproperties.GetBoolPropertyFn(true),
		dynamicproperties.GetIntPropertyFn(100),
		shardCount,
	)

	shardProcessor, err := NewShardProcessor(ShardProcessorParams{
		ShardID:                shardID,
		TaskListsRegistry:      mockRegistry,
		ReportTTL:              reportTTL,
		TimeSource:             timeSource,
		TaskListDifferentiator: taskListDifferentiator,
	})
	require.NoError(t, err)

	timeSource.Advance(reportTTL + time.Second)
	shardReport := shardProcessor.GetShardReport()
	require.Equal(t, decisionTaskListQPS+activityTaskListQPS, shardReport.ShardLoad)

	shardProcessor.Stop()
}

// The registry files a task list under its shard by name and its shard by domain ID and name, so
// flipping matching.hashDomainIDForShardManager while task lists are loaded moves them
// between shard processors without re-registering them.
func TestShardProcessor_FollowsTaskListsWhenTheDomainFlagFlips(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	shardCount := 100
	reportTTL := time.Second
	timeSource := clock.NewMockedTimeSource()

	nameShardID := "14" // farm.Fingerprint64([]byte("billing"))%100

	domainBTaskListID := mustNewIdentifier("domain-b", "billing", persistence.TaskListTypeDecision)
	domainBShardID := "8" // farm.Fingerprint64([]byte("domain-b/billing"))%100
	require.Equal(t, nameShardID, matching.TaskListNameToShard(domainBTaskListID.GetName(), shardCount))
	require.Equal(t, domainBShardID, matching.TaskListToShard(domainBTaskListID.GetDomainID(), domainBTaskListID.GetName(), shardCount))
	domainBTaskListQPS := 2.0
	domainBTaskList := NewMockManager(ctrl)
	domainBTaskList.EXPECT().TaskListID().Return(domainBTaskListID).AnyTimes()
	domainBTaskList.EXPECT().QueriesPerSecond().Return(domainBTaskListQPS).AnyTimes()

	domainCTaskListID := mustNewIdentifier("domain-c", "billing", persistence.TaskListTypeDecision)
	require.NotEqual(t, domainBShardID, matching.TaskListToShard(domainCTaskListID.GetDomainID(), domainCTaskListID.GetName(), shardCount))
	domainCTaskListQPS := 3.0
	domainCTaskList := NewMockManager(ctrl)
	domainCTaskList.EXPECT().TaskListID().Return(domainCTaskListID).AnyTimes()
	domainCTaskList.EXPECT().QueriesPerSecond().Return(domainCTaskListQPS).AnyTimes()

	registry := NewTaskListRegistry(metrics.NewNoopMetricsClient(), shardCount)
	registry.Register(*domainBTaskListID, domainBTaskList)
	registry.Register(*domainCTaskListID, domainCTaskList)

	hashDomainID := false
	taskListDifferentiator := matching.NewTaskListDifferentiator(
		metrics.NewNoopMetricsClient(),
		dynamicproperties.GetBoolPropertyFn(true),
		dynamicproperties.GetBoolPropertyFn(true),
		func(...dynamicproperties.FilterOption) bool { return hashDomainID },
		dynamicproperties.GetIntPropertyFn(100),
		shardCount,
	)
	nameShardProcessor := mustNewShardProcessorForTest(t, nameShardID, registry, reportTTL, timeSource, taskListDifferentiator)
	domainBShardProcessor := mustNewShardProcessorForTest(t, domainBShardID, registry, reportTTL, timeSource, taskListDifferentiator)

	timeSource.Advance(reportTTL + time.Second)
	assert.Equal(t, domainBTaskListQPS+domainCTaskListQPS, nameShardProcessor.GetShardReport().ShardLoad, "by name alone both task lists should be in the name's shard")
	assert.Equal(t, 0.0, domainBShardProcessor.GetShardReport().ShardLoad, "no task list should be in domain-b's shard before the flip")

	hashDomainID = true

	timeSource.Advance(reportTTL + time.Second)
	assert.Equal(t, 0.0, nameShardProcessor.GetShardReport().ShardLoad, "no task list should be left in the name's shard after the flip")
	assert.Equal(t, domainBTaskListQPS, domainBShardProcessor.GetShardReport().ShardLoad, "only domain-b's task list should be in its shard after the flip")

	domainBTaskList.EXPECT().Stop()
	domainBShardProcessor.Stop()
	nameShardProcessor.Stop()
}

func mustNewShardProcessorForTest(
	t *testing.T,
	shardID string,
	registry TaskListRegistry,
	reportTTL time.Duration,
	timeSource clock.TimeSource,
	taskListDifferentiator *matching.TaskListDifferentiator,
) ShardProcessor {
	t.Helper()
	shardProcessor, err := NewShardProcessor(ShardProcessorParams{
		ShardID:                shardID,
		TaskListsRegistry:      registry,
		ReportTTL:              reportTTL,
		TimeSource:             timeSource,
		TaskListDifferentiator: taskListDifferentiator,
	})
	require.NoError(t, err)
	return shardProcessor
}
