package handler

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/cadence-workflow/shard-manager/service/sharddistributor/client/executorclient"
	"github.com/cadence-workflow/shard-manager/service/sharddistributor/client/spectatorclient"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/matching/tasklist"
)

const (
	// host1 and host2 are the two matching hosts in the test.
	host1 = "host1"
	host2 = "host2"
)

// onboardingTestData holds two matching hosts, the onboarding percentage they both read, and the
// SD and hash ring mocks they share. SD reports each shard as owned by the host it gave the shard
// to. The hash ring routes each task list to its ringOwner.
type onboardingTestData struct {
	ctrl       *gomock.Controller
	timeSource clock.MockedTimeSource

	// percentageOnboarded is the onboarding percentage every matching host reads.
	percentageOnboarded int
	// taskLists describes every task list in the test.
	taskLists map[string]onboardingTaskList

	// taskListDifferentiator and shardDistributorResolver are shared by both hosts.
	// shardDistributorResolver routes each task list through SD or the hash ring, as matching does.
	taskListDifferentiator   *membership.TaskListDifferentiator
	shardDistributorResolver membership.SingleProvider

	// hosts holds host1 and host2 by name.
	hosts map[string]*onboardingHost
}

// onboardingHost is one matching host running the real matching engine.
type onboardingHost struct {
	engine *matchingEngineImpl
	// assignedShards holds the processors of the shards SD has given the host.
	assignedShards map[string]tasklist.ShardProcessor
	// managers holds the host's manager for every task list in the test.
	managers map[string]*tasklist.MockManager
}

// onboardingTaskList is one task list in the test.
type onboardingTaskList struct {
	// ringOwner is the host the hash ring routes the task list to.
	ringOwner string
	// qps is the load the task list's manager reports.
	qps float64
}

// hostState is what one host looks like at a step of the test.
type hostState struct {
	// throughSD and throughHashRing hold the task lists the host runs, split by whether SD or the
	// hash ring routes them to the host.
	throughSD       []string
	throughHashRing []string
	// shardLoads holds the load each of the host's shards reports to SD.
	shardLoads map[string]float64
}

// TestShardDistributorOnboardingWithFixedShards follows two matching hosts while the onboarding
// percentage goes 0, 50, 100, 50, 0. Every task list name hashes into one of 4 shards, and a
// percentage onboards that share of the shards, starting from shard 0.
func TestShardDistributorOnboardingWithFixedShards(t *testing.T) {
	taskList1 := "checkout"
	taskList1QPS := 3.0

	taskList2 := "returns"
	taskList2QPS := 4.0

	taskList3 := "pricing"
	taskList3QPS := 5.0

	taskList4 := "cart"
	taskList4QPS := 6.0

	taskList5 := "emails"
	taskList5QPS := 2.0

	// A UUID in the name marks a short-lived task list, which always stays on the hash ring.
	shortLivedTaskList := "550e8400-e29b-41d4-a716-446655440000"
	shortLivedQPS := 1.0

	taskLists := map[string]onboardingTaskList{
		taskList1:          {ringOwner: host2, qps: taskList1QPS},
		taskList2:          {ringOwner: host2, qps: taskList2QPS},
		taskList3:          {ringOwner: host2, qps: taskList3QPS},
		taskList4:          {ringOwner: host2, qps: taskList4QPS},
		taskList5:          {ringOwner: host1, qps: taskList5QPS},
		shortLivedTaskList: {ringOwner: host1, qps: shortLivedQPS},
	}

	shardTaskListsByHash := true
	td := newOnboardingTestData(t, shardTaskListsByHash, taskLists)
	td.requireTaskListsInShards(t, map[string][]string{
		"0": {taskList1, taskList2},
		"1": {taskList3},
		"2": {taskList4},
		"3": {taskList5},
	})

	// 0%: the hash ring routes every task list.
	td.setPercentage(t, 0)
	td.requireHost(t, host1, hostState{
		throughHashRing: []string{shortLivedTaskList, taskList5},
	})
	td.requireHost(t, host2, hostState{
		throughHashRing: []string{taskList1, taskList2, taskList3, taskList4},
	})

	// 50%: the hosts join SD, which gives out every shard. Shards 0 and 1 are onboarded, so
	// taskList1, taskList2 and taskList3 move to host1.
	td.giveShards(t, host1, "0", "1", "3")
	td.giveShards(t, host2, "2")
	td.expectStop(host2, taskList1, taskList2, taskList3)
	td.setPercentage(t, 50)
	td.requireHost(t, host1, hostState{
		throughSD:       []string{taskList1, taskList2, taskList3},
		throughHashRing: []string{shortLivedTaskList, taskList5},
		shardLoads: map[string]float64{
			"0": taskList1QPS + taskList2QPS,
			"1": taskList3QPS,
			"3": 0,
		},
	})
	td.requireHost(t, host2, hostState{
		throughHashRing: []string{taskList4},
		shardLoads:      map[string]float64{"2": 0},
	})

	// 100%: every shard is onboarded. SD takes over taskList4 and taskList5 without moving them.
	td.setPercentage(t, 100)
	td.requireHost(t, host1, hostState{
		throughSD:       []string{taskList1, taskList2, taskList3, taskList5},
		throughHashRing: []string{shortLivedTaskList},
		shardLoads: map[string]float64{
			"0": taskList1QPS + taskList2QPS,
			"1": taskList3QPS,
			"3": taskList5QPS,
		},
	})
	td.requireHost(t, host2, hostState{
		throughSD:  []string{taskList4},
		shardLoads: map[string]float64{"2": taskList4QPS},
	})

	// Back to 50%: the hash ring takes back taskList4 and taskList5 without moving them.
	td.setPercentage(t, 50)
	td.requireHost(t, host1, hostState{
		throughSD:       []string{taskList1, taskList2, taskList3},
		throughHashRing: []string{shortLivedTaskList, taskList5},
		shardLoads: map[string]float64{
			"0": taskList1QPS + taskList2QPS,
			"1": taskList3QPS,
			"3": 0,
		},
	})
	td.requireHost(t, host2, hostState{
		throughHashRing: []string{taskList4},
		shardLoads:      map[string]float64{"2": 0},
	})

	// SD moves shards 1 and 3 to host2. Only taskList3 moves with them, because the hash ring
	// routes taskList5.
	td.expectStop(host1, taskList3)
	td.takeShards(t, host1, "1", "3")
	td.giveShards(t, host2, "1", "3")
	td.requireHost(t, host1, hostState{
		throughSD:       []string{taskList1, taskList2},
		throughHashRing: []string{shortLivedTaskList, taskList5},
		shardLoads:      map[string]float64{"0": taskList1QPS + taskList2QPS},
	})
	td.requireHost(t, host2, hostState{
		throughSD:       []string{taskList3},
		throughHashRing: []string{taskList4},
		shardLoads:      map[string]float64{"1": taskList3QPS, "2": 0, "3": 0},
	})

	// 0%: taskList1, taskList2 and taskList3 return to the hash ring, which puts them on host2.
	// The hosts keep their shards, but matching no longer follows them.
	td.expectStop(host1, taskList1, taskList2)
	td.setPercentage(t, 0)
	td.requireHost(t, host1, hostState{
		throughHashRing: []string{shortLivedTaskList, taskList5},
		shardLoads:      map[string]float64{"0": 0},
	})
	td.requireHost(t, host2, hostState{
		throughHashRing: []string{taskList1, taskList2, taskList3, taskList4},
		shardLoads:      map[string]float64{"1": 0, "2": 0, "3": 0},
	})
}

// TestShardDistributorOnboardingWithOneShardPerTaskListName follows a task list's shard after the
// hash ring takes the task list back without moving it. With one shard per task list name, each
// shard is named after its task list, and each name is onboarded on its own.
func TestShardDistributorOnboardingWithOneShardPerTaskListName(t *testing.T) {
	taskList := "emails"
	taskListQPS := 2.0

	taskLists := map[string]onboardingTaskList{
		taskList: {ringOwner: host1, qps: taskListQPS},
	}

	shardTaskListsByHash := false
	td := newOnboardingTestData(t, shardTaskListsByHash, taskLists)

	// 0%: the hash ring routes the task list to host1.
	td.setPercentage(t, 0)
	td.requireHost(t, host1, hostState{
		throughHashRing: []string{taskList},
	})

	// 100%: the task list is onboarded. SD gives its shard to host1, so SD takes the task list
	// over without moving it.
	td.giveShards(t, host1, taskList)
	td.setPercentage(t, 100)
	td.requireHost(t, host1, hostState{
		throughSD:  []string{taskList},
		shardLoads: map[string]float64{taskList: taskListQPS},
	})

	// 50%: the task list isn't onboarded at 50%, so the hash ring takes it back without moving it.
	// Unlike with fixed shards, its shard still counts it, because the shard doesn't check the
	// hash ring.
	td.setPercentage(t, 50)
	td.requireHost(t, host1, hostState{
		throughHashRing: []string{taskList},
		shardLoads:      map[string]float64{taskList: taskListQPS},
	})

	// Matching stops asking for the shard, so it goes idle and SD takes it away. The task list
	// stops with its shard, and starts again on its next request, because the hash ring still
	// routes it to host1.
	td.expectStop(host1, taskList)
	td.takeShards(t, host1, taskList)
	td.requireHost(t, host1, hostState{
		throughHashRing: []string{taskList},
	})
}

func newOnboardingTestData(t *testing.T, shardTaskListsByHash bool, taskLists map[string]onboardingTaskList) *onboardingTestData {
	// Earlier tests in this package leave engine goroutines running, so only goroutines started
	// from here on count as leaks.
	ignoreEarlierGoroutines := goleak.IgnoreCurrent()
	t.Cleanup(func() { goleak.VerifyNone(t, ignoreEarlierGoroutines) })

	td := &onboardingTestData{
		ctrl:       gomock.NewController(t),
		timeSource: clock.NewMockedTimeSource(),
		taskLists:  taskLists,
		hosts:      make(map[string]*onboardingHost),
	}

	// Four shards keep the test feasible to follow.
	shardCount := 4
	percentageOnboarded := membership.NewPercentageOnboarded(
		metrics.NewNoopMetricsClient(),
		func(...dynamicproperties.FilterOption) int { return td.percentageOnboarded },
	)
	td.taskListDifferentiator = membership.NewTaskListDifferentiator(
		dynamicproperties.GetBoolPropertyFn(true), // short-lived task lists stay on the hash ring
		dynamicproperties.GetBoolPropertyFn(shardTaskListsByHash),
		percentageOnboarded,
		shardCount,
	)

	hostNames := []string{host1, host2}
	td.shardDistributorResolver = td.newShardDistributorResolver(len(hostNames))
	for _, hostName := range hostNames {
		td.hosts[hostName] = td.newHost(t, hostName, percentageOnboarded)
	}
	return td
}

// newShardDistributorResolver builds the resolver matching routes task lists with. It asks SD
// about onboarded task lists and the hash ring about the rest. SD reports each shard as owned by
// the host it gave the shard to, and the hash ring answers with each task list's ringOwner.
func (td *onboardingTestData) newShardDistributorResolver(hostCount int) membership.SingleProvider {
	spectator := spectatorclient.NewMockSpectator(td.ctrl)
	spectator.EXPECT().GetShardOwner(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, shardID string) (*spectatorclient.ShardOwner, error) {
			for hostName, host := range td.hosts {
				if _, ok := host.assignedShards[shardID]; ok {
					shardOwner := &spectatorclient.ShardOwner{
						ExecutorID: hostName,
						Metadata:   map[string]string{"hostIP": "127.0.0.1", "tchannel": "7933", "grpc": "7833"},
					}
					return shardOwner, nil
				}
			}
			return nil, fmt.Errorf("SD has given shard %q to no host", shardID)
		},
	).AnyTimes()

	hashRing := membership.NewMockSingleProvider(td.ctrl)
	hashRing.EXPECT().Lookup(gomock.Any()).DoAndReturn(
		func(taskListName string) (membership.HostInfo, error) {
			owner := membership.NewDetailedHostInfo("", td.taskLists[taskListName].ringOwner, nil)
			return owner, nil
		},
	).AnyTimes()
	// Each host's engine starts a membership loop that subscribes to matching's ring once.
	hashRing.EXPECT().Subscribe("matching-engine", gomock.Any()).Return(nil).Times(hostCount)

	shardDistributorResolver := membership.NewShardDistributorResolver(
		spectator,
		td.taskListDifferentiator,
		hashRing,
		log.NewNoop(),
		metrics.NoopScope,
	)
	return shardDistributorResolver
}

// newHost starts a matching engine that knows itself as hostName and routes task lists with the
// shared resolver.
func (td *onboardingTestData) newHost(t *testing.T, hostName string, percentageOnboarded membership.PercentageOnboarded) *onboardingHost {
	peerProvider := membership.NewMockPeerProvider(td.ctrl)
	self := membership.NewDetailedHostInfo("", hostName, nil)
	peerProvider.EXPECT().WhoAmI().Return(self, nil).AnyTimes()

	rings := map[string]membership.SingleProvider{service.Matching: td.shardDistributorResolver}
	resolver, err := membership.NewResolver(peerProvider, metrics.NewNoopMetricsClient(), log.NewNoop(), rings)
	require.NoError(t, err)

	// The test never reaches persistence, history or the domain cache, so their mocks have no
	// expectations.
	engine := NewEngine(
		persistence.NewMockTaskManager(td.ctrl),
		persistence.NewMockSemaphoreTokenManager(td.ctrl),
		persistence.NewMockSemaphoreMetadataManager(td.ctrl),
		cluster.GetTestClusterMetadata(true),
		history.NewMockClient(td.ctrl),
		matching.NewMockClient(td.ctrl),
		defaultTestConfig(),
		log.NewNoop(),
		zap.NewNop(),
		metrics.NewNoopMetricsClient(),
		tally.NoopScope,
		cache.NewMockDomainCache(td.ctrl),
		resolver,
		isolationgroup.NewMockState(td.ctrl),
		td.timeSource,
		executorclient.NewMockClient(td.ctrl),
		defaultSDExecutorConfig(),
		td.taskListDifferentiator,
		nil,
		percentageOnboarded,
	).(*matchingEngineImpl)
	// NewEngine starts the membership loop. Stop it directly, since engine.Stop would also stop
	// every task list still running.
	t.Cleanup(func() {
		close(engine.shutdown)
		engine.shutdownCompletion.Wait()
	})

	host := &onboardingHost{
		engine:         engine,
		assignedShards: make(map[string]tasklist.ShardProcessor),
		managers:       make(map[string]*tasklist.MockManager, len(td.taskLists)),
	}
	engine.executor = td.newExecutor(host)
	for taskListName, taskList := range td.taskLists {
		host.managers[taskListName] = td.newManager(t, taskListName, taskList.qps)
	}
	return host
}

// newExecutor returns an executor that holds the processors of the shards SD has given the host.
func (td *onboardingTestData) newExecutor(host *onboardingHost) executorclient.Executor[tasklist.ShardProcessor] {
	executor := executorclient.NewMockExecutor[tasklist.ShardProcessor](td.ctrl)
	executor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, shardID string) (tasklist.ShardProcessor, error) {
			shardProcessor, ok := host.assignedShards[shardID]
			if !ok {
				return nil, executorclient.ErrShardProcessNotFound
			}
			return shardProcessor, nil
		},
	).AnyTimes()
	return executor
}

// newManager returns a manager for the task list that reports qps as its load. Stop has no
// expectation until expectStop sets one, so a stop the test doesn't expect fails it.
func (td *onboardingTestData) newManager(t *testing.T, taskListName string, qps float64) *tasklist.MockManager {
	taskListID := tasklist.NewTestTaskListID(t, "domain-id", taskListName, persistence.TaskListTypeDecision)
	manager := tasklist.NewMockManager(td.ctrl)
	manager.EXPECT().TaskListID().Return(taskListID).AnyTimes()
	manager.EXPECT().QueriesPerSecond().Return(qps).AnyTimes()
	manager.EXPECT().String().Return(taskListName).AnyTimes()
	return manager
}

// requireTaskListsInShards requires that each task list name hashes into the given shard.
func (td *onboardingTestData) requireTaskListsInShards(t *testing.T, taskListsByShard map[string][]string) {
	t.Helper()
	for wantShardID, taskListNames := range taskListsByShard {
		for _, taskListName := range taskListNames {
			gotShardID := td.taskListDifferentiator.ShardID(taskListName)
			require.Equal(t, wantShardID, gotShardID, "shard of task list %q", taskListName)
		}
	}
}

// giveShards gives the host the shards, as SD does when it assigns them. Each host then starts the
// task lists it now owns.
func (td *onboardingTestData) giveShards(t *testing.T, hostName string, shardIDs ...string) {
	host := td.hosts[hostName]
	for _, shardID := range shardIDs {
		shardProcessor, err := host.engine.taskListsFactory.NewShardProcessor(shardID)
		require.NoError(t, err)
		host.assignedShards[shardID] = shardProcessor
	}
	td.startTaskListsHostsNowOwn(t)
}

// setPercentage changes the onboarding percentage every host reads. Each host then stops the task
// lists it no longer owns and starts the ones it now owns.
func (td *onboardingTestData) setPercentage(t *testing.T, percentage int) {
	td.percentageOnboarded = percentage
	td.stopTaskListsHostsNoLongerOwn(t)
	td.startTaskListsHostsNowOwn(t)
}

// requireHost requires that the host runs exactly the given task lists, that SD or the hash ring
// routes each one to the host as given, and that the host's shards report the given loads.
func (td *onboardingTestData) requireHost(t *testing.T, hostName string, want hostState) {
	t.Helper()
	host := td.hosts[hostName]

	var got hostState
	for taskListName := range td.taskLists {
		owner, err := td.shardDistributorResolver.Lookup(taskListName)
		require.NoError(t, err)
		if owner.Identity() != hostName {
			continue
		}
		if td.taskListDifferentiator.TaskListExcludedFromShardDistributor(taskListName) {
			got.throughHashRing = append(got.throughHashRing, taskListName)
		} else {
			got.throughSD = append(got.throughSD, taskListName)
		}
	}
	require.ElementsMatch(t, want.throughSD, got.throughSD, "task lists SD routes to %s", hostName)
	require.ElementsMatch(t, want.throughHashRing, got.throughHashRing, "task lists the hash ring routes to %s", hostName)

	routedToHost := slices.Concat(want.throughSD, want.throughHashRing)
	require.ElementsMatch(t, routedToHost, host.runningTaskLists(), "task lists running on %s", hostName)

	// Move time past the report TTL, so every shard computes a fresh load.
	td.timeSource.Advance(host.engine.taskListsFactory.ReportTTL + time.Second)
	gotShardLoads := host.shardLoads()
	require.True(t, maps.Equal(want.shardLoads, gotShardLoads), "shard loads of %s: want %v, got %v", hostName, want.shardLoads, gotShardLoads)
}

// expectStop expects each task list to stop on the host once. A real task list manager
// unregisters itself when it stops, so the mock does the same.
func (td *onboardingTestData) expectStop(hostName string, taskListNames ...string) {
	host := td.hosts[hostName]
	for _, taskListName := range taskListNames {
		manager := host.managers[taskListName]
		manager.EXPECT().Stop().Do(func() {
			host.engine.taskListRegistry.Unregister(manager)
		})
	}
}

// takeShards takes the shards from the host, as SD does when it moves or deletes them. The host's
// executor stops each shard's processor. Each host then starts the task lists it now owns.
func (td *onboardingTestData) takeShards(t *testing.T, hostName string, shardIDs ...string) {
	host := td.hosts[hostName]
	for _, shardID := range shardIDs {
		shardProcessor := host.assignedShards[shardID]
		delete(host.assignedShards, shardID)
		shardProcessor.Stop()
	}
	td.startTaskListsHostsNowOwn(t)
}

// stopTaskListsHostsNoLongerOwn stops, on every host, the task lists the host runs but no longer
// owns. Matching runs this check when a host joins or leaves the hash ring, not when the
// percentage changes.
func (td *onboardingTestData) stopTaskListsHostsNoLongerOwn(t *testing.T) {
	for _, host := range td.hosts {
		err := host.engine.shutDownNonOwnedTasklists()
		require.NoError(t, err)
	}
}

// startTaskListsHostsNowOwn starts, on every host, the task lists the host owns but isn't running
// yet. Matching starts a task list when its first request arrives, after checking that the host
// owns it.
func (td *onboardingTestData) startTaskListsHostsNowOwn(t *testing.T) {
	for _, host := range td.hosts {
		for _, manager := range host.managers {
			taskListID := manager.TaskListID()
			_, running := host.engine.taskListRegistry.ManagerByTaskListIdentifier(*taskListID)
			if running || !host.owns(t, taskListID) {
				continue
			}
			host.engine.taskListRegistry.Register(*taskListID, manager)
		}
	}
}

// owns runs the ownership check matching runs before it starts a task list.
func (h *onboardingHost) owns(t *testing.T, taskListID *tasklist.Identifier) bool {
	err := h.engine.errIfShardOwnershipLost(context.Background(), taskListID)
	if err == nil {
		return true
	}
	var notOwnedErr *cadence_errors.TaskListNotOwnedByHostError
	require.ErrorAs(t, err, &notOwnedErr, "task list %q", taskListID.GetName())
	return false
}

// runningTaskLists returns the names of the task lists the host runs.
func (h *onboardingHost) runningTaskLists() []string {
	var running []string
	for _, manager := range h.engine.taskListRegistry.AllManagers() {
		running = append(running, manager.TaskListID().GetName())
	}
	return running
}

// shardLoads returns the load each of the host's shards reports to SD.
func (h *onboardingHost) shardLoads() map[string]float64 {
	shardLoads := make(map[string]float64, len(h.assignedShards))
	for shardID, shardProcessor := range h.assignedShards {
		shardLoads[shardID] = shardProcessor.GetShardReport().ShardLoad
	}
	return shardLoads
}
