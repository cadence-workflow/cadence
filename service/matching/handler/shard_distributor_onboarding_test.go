package handler

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
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

	// noHost means SD hasn't given the shard to any host.
	noHost = ""

	// routedBySD and routedByHashRing say which one the resolver asked for a task list's owner.
	routedBySD       = "SD"
	routedByHashRing = "ring"
)

// onboardingTestData holds two matching hosts running the real matching engine, with a fake SD and
// a fake hash ring.
type onboardingTestData struct {
	ctrl       *gomock.Controller
	timeSource clock.MockedTimeSource

	// percentageOnboarded is the onboarding percentage every host reads.
	percentageOnboarded int
	taskLists           []onboardingTaskList
	// shardOwners holds the host SD gives each shard to when the host joins SD.
	shardOwners map[string]string
	// routedBy is whichever of SD and the hash ring the resolver asked last.
	routedBy string

	// taskListDifferentiator and shardDistributorResolver are shared by both hosts.
	taskListDifferentiator   *membership.TaskListDifferentiator
	shardDistributorResolver membership.SingleProvider
	hosts                    []*onboardingHost
}

// onboardingTaskList is one task list in the test.
type onboardingTaskList struct {
	name string
	// shard is the shard the name hashes into.
	shard string
	// ringOwner is the host the hash ring routes the task list to.
	ringOwner string
}

// onboardingHost is one matching host running the real matching engine.
type onboardingHost struct {
	name   string
	engine *matchingEngineImpl
	// joinedSD is set once matching lets the host's executor heartbeat to SD.
	joinedSD bool
	// shardProcessors holds the processors of the shards SD has given the host.
	shardProcessors map[string]tasklist.ShardProcessor
	// managers holds the host's manager for every task list in the test.
	managers map[string]*tasklist.MockManager
	// stopped holds the names of the task lists that stopped on the host since the last
	// requireShards. Task lists stop in parallel, so it's a sync.Map.
	stopped sync.Map
}

// route is where the resolver sends a task list.
type route struct {
	routedBy string
	owner    string
}

// routes holds the route of each task list, by task list name.
type routes map[string]route

// shard is the host that holds a shard, the load the shard reports, and where the resolver sends
// each task list in the shard. A shard reports the qps of the task lists SD routes to it. Every task
// list reports 1 qps, so the load is the number of those task lists.
type shard struct {
	heldBy    string
	load      float64
	taskLists routes
}

// shards holds every shard SD gives out, by shard ID.
type shards map[string]shard

// TestShardDistributorOnboardingWithFixedShards follows two matching hosts while the onboarding
// percentage goes up and down. Every task list reports 1 qps, so a shard's load is the number of
// sd entries in its row.
func TestShardDistributorOnboardingWithFixedShards(t *testing.T) {
	taskLists := []onboardingTaskList{
		{name: "checkout", shard: "0", ringOwner: host1},
		{name: "returns", shard: "0", ringOwner: host2},
		{name: "pricing", shard: "1", ringOwner: host2},
	}
	// A host joins SD once the percentage is above 0. SD then gives it these shards, onboarded or not.
	shardOwners := map[string]string{"0": host1, "1": host2}
	td := newOnboardingTestData(t, taskLists, shardOwners)

	// No host has joined SD, so no host holds a shard. The hash ring routes every task list.
	td.setPercentage(t, 0)
	td.requireShards(t, shards{
		"0": {heldBy: noHost, load: 0, taskLists: routes{"checkout": ring(host1), "returns": ring(host2)}},
		"1": {heldBy: noHost, load: 0, taskLists: routes{"pricing": ring(host2)}},
	})

	// The hosts join SD, so host1 holds shard 0 and host2 holds shard 1. At 50% only shard 0 is
	// onboarded. SD takes over "checkout" and "returns", so "returns" moves from host2 to host1. The
	// hash ring keeps "pricing".
	td.setPercentage(t, 50)
	td.requireShards(t, shards{
		"0": {heldBy: host1, load: 2, taskLists: routes{"checkout": sd(host1), "returns": sd(host1)}},
		"1": {heldBy: host2, load: 0, taskLists: routes{"pricing": ring(host2)}},
	})

	// At 100% shard 1 is onboarded too. SD takes over "pricing", which stays on host2.
	td.setPercentage(t, 100)
	td.requireShards(t, shards{
		"0": {heldBy: host1, load: 2, taskLists: routes{"checkout": sd(host1), "returns": sd(host1)}},
		"1": {heldBy: host2, load: 1, taskLists: routes{"pricing": sd(host2)}},
	})

	// At 50% shard 1 is no longer onboarded. The hash ring takes back "pricing", which stays on
	// host2, and shard 1 stops counting its load.
	td.setPercentage(t, 50)
	td.requireShards(t, shards{
		"0": {heldBy: host1, load: 2, taskLists: routes{"checkout": sd(host1), "returns": sd(host1)}},
		"1": {heldBy: host2, load: 0, taskLists: routes{"pricing": ring(host2)}},
	})

	// SD moves shard 0 from host1 to host2. "checkout" and "returns" move from host1 to host2 with it.
	td.moveShard(t, "0", host2)
	td.requireShards(t, shards{
		"0": {heldBy: host2, load: 2, taskLists: routes{"checkout": sd(host2), "returns": sd(host2)}},
		"1": {heldBy: host2, load: 0, taskLists: routes{"pricing": ring(host2)}},
	})

	// At 0% the hash ring takes back every task list. "checkout" moves from host2 to host1, and
	// "returns" and "pricing" stay on host2. host2 keeps both shards, but they count no load.
	td.setPercentage(t, 0)
	td.requireShards(t, shards{
		"0": {heldBy: host2, load: 0, taskLists: routes{"checkout": ring(host1), "returns": ring(host2)}},
		"1": {heldBy: host2, load: 0, taskLists: routes{"pricing": ring(host2)}},
	})
}

// sd is the route of a task list that SD sends to the host.
func sd(host string) route {
	return route{routedBy: routedBySD, owner: host}
}

// ring is the route of a task list that the hash ring sends to the host.
func ring(host string) route {
	return route{routedBy: routedByHashRing, owner: host}
}

func newOnboardingTestData(
	t *testing.T,
	taskLists []onboardingTaskList,
	shardOwners map[string]string,
) *onboardingTestData {
	// Earlier tests in this package leave engine goroutines running, so only goroutines started
	// from here on count as leaks.
	ignoreEarlierGoroutines := goleak.IgnoreCurrent()
	t.Cleanup(func() { goleak.VerifyNone(t, ignoreEarlierGoroutines) })

	td := &onboardingTestData{
		ctrl:        gomock.NewController(t),
		timeSource:  clock.NewMockedTimeSource(),
		taskLists:   taskLists,
		shardOwners: shardOwners,
	}

	// SD gives out every shard, so shardOwners names them all.
	shardCount := len(shardOwners)
	percentageOnboarded := membership.NewPercentageOnboarded(
		metrics.NewNoopMetricsClient(),
		func(...dynamicproperties.FilterOption) int { return td.percentageOnboarded },
	)
	shardTaskListsByHash := true
	td.taskListDifferentiator = membership.NewTaskListDifferentiator(
		dynamicproperties.GetBoolPropertyFn(true), // short-lived task lists stay on the hash ring
		dynamicproperties.GetBoolPropertyFn(shardTaskListsByHash),
		percentageOnboarded,
		shardCount,
	)
	for _, taskList := range taskLists {
		require.Contains(t, shardOwners, taskList.shard, "SD gives out the shard of task list %q", taskList.name)
		gotShardID := td.taskListDifferentiator.ShardID(taskList.name)
		require.Equal(t, taskList.shard, gotShardID, "shard of task list %q", taskList.name)
	}

	hostNames := []string{host1, host2}
	td.shardDistributorResolver = td.newShardDistributorResolver(len(hostNames))
	for _, hostName := range hostNames {
		td.hosts = append(td.hosts, td.newHost(t, hostName, percentageOnboarded))
	}
	return td
}

// newShardDistributorResolver builds the resolver matching routes task lists with. It asks SD
// about onboarded task lists and the hash ring about the rest. SD answers with the host holding
// the shard, and the hash ring with the task list's ringOwner. Both note that the resolver asked
// them.
func (td *onboardingTestData) newShardDistributorResolver(hostCount int) membership.SingleProvider {
	spectator := spectatorclient.NewMockSpectator(td.ctrl)
	spectator.EXPECT().GetShardOwner(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, shardID string) (*spectatorclient.ShardOwner, error) {
			td.routedBy = routedBySD
			host := td.shardHolder(shardID)
			if host == nil {
				return nil, fmt.Errorf("SD has given shard %q to no host", shardID)
			}
			shardOwner := &spectatorclient.ShardOwner{
				ExecutorID: host.name,
				Metadata:   map[string]string{"hostIP": "127.0.0.1", "tchannel": "7933", "grpc": "7833"},
			}
			return shardOwner, nil
		},
	).AnyTimes()

	hashRing := membership.NewMockSingleProvider(td.ctrl)
	for _, taskList := range td.taskLists {
		owner := membership.NewDetailedHostInfo("", taskList.ringOwner, nil)
		hashRing.EXPECT().Lookup(taskList.name).DoAndReturn(
			func(string) (membership.HostInfo, error) {
				td.routedBy = routedByHashRing
				return owner, nil
			},
		).AnyTimes()
	}
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
		persistence.NewMockSemaphoreTaskManager(td.ctrl),
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
		name:            hostName,
		engine:          engine,
		shardProcessors: make(map[string]tasklist.ShardProcessor),
		managers:        make(map[string]*tasklist.MockManager, len(td.taskLists)),
	}
	engine.executor = td.newExecutor(host)
	for _, taskList := range td.taskLists {
		host.managers[taskList.name] = td.newManager(t, host, taskList)
	}
	return host
}

// newExecutor returns an executor that holds the processors of the shards SD has given the host.
func (td *onboardingTestData) newExecutor(host *onboardingHost) executorclient.Executor[tasklist.ShardProcessor] {
	executor := executorclient.NewMockExecutor[tasklist.ShardProcessor](td.ctrl)
	executor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, shardID string) (tasklist.ShardProcessor, error) {
			shardProcessor, ok := host.shardProcessors[shardID]
			if !ok {
				return nil, executorclient.ErrShardProcessNotFound
			}
			return shardProcessor, nil
		},
	).AnyTimes()
	return executor
}

// newManager returns the host's manager for the task list, which reports 1 qps. requireShards
// checks every stop, so Stop can happen any number of times.
func (td *onboardingTestData) newManager(t *testing.T, host *onboardingHost, taskList onboardingTaskList) *tasklist.MockManager {
	taskListID := tasklist.NewTestTaskListID(t, "domain-id", taskList.name, persistence.TaskListTypeDecision)
	manager := tasklist.NewMockManager(td.ctrl)
	manager.EXPECT().TaskListID().Return(taskListID).AnyTimes()
	qps := 1.0
	manager.EXPECT().QueriesPerSecond().Return(qps).AnyTimes()
	manager.EXPECT().String().Return(taskList.name).AnyTimes()
	// A real manager unregisters itself when it stops, so the mock does the same.
	manager.EXPECT().Stop().Do(func() {
		host.stopped.Store(taskList.name, true)
		host.engine.taskListRegistry.Unregister(manager)
	}).AnyTimes()
	return manager
}

// setPercentage changes the onboarding percentage every host reads. A host joins SD and gets its
// shards once matching lets its executor heartbeat. Then each host stops the task lists it no
// longer owns and starts the ones it now owns. In production a host only runs that stop check when
// the hash ring's membership changes.
func (td *onboardingTestData) setPercentage(t *testing.T, percentage int) {
	td.percentageOnboarded = percentage
	td.joinSDOnceOnboarded(t)
	td.stopTaskListsHostsNoLongerOwn(t)
	td.startTaskListsHostsNowOwn(t)
}

// moveShard moves the shard to the host, as SD does when it rebalances. The host that loses the
// shard stops its processor. Then each host starts the task lists it now owns.
func (td *onboardingTestData) moveShard(t *testing.T, shardID string, toHostName string) {
	td.shardHolder(shardID).removeShard(shardID)
	td.host(toHostName).addShard(t, shardID)
	td.startTaskListsHostsNowOwn(t)
}

// requireShards requires the host that holds each shard, the load it reports, and where the
// resolver sends each task list in the shard. It also requires that each task list runs only on the
// host it's routed to, and didn't stop there since the last requireShards.
func (td *onboardingTestData) requireShards(t *testing.T, want shards) {
	t.Helper()
	// Move time past the report TTL, so every shard computes a fresh load.
	reportTTL := td.hosts[0].engine.taskListsFactory.ReportTTL
	td.timeSource.Advance(reportTTL + time.Second)

	got := td.currentShards(t)
	require.Equal(t, want, got, "every shard after this step")

	for _, taskList := range td.taskLists {
		taskListRoute := got[taskList.shard].taskLists[taskList.name]
		td.requireRunsOnlyOn(t, taskList.name, taskListRoute.owner)
	}
	for _, host := range td.hosts {
		host.stopped.Clear()
	}
}

// currentShards returns each shard as the hosts and the resolver report it now: the host holding
// it, the load it reports, and where the resolver sends each of its task lists.
func (td *onboardingTestData) currentShards(t *testing.T) shards {
	t.Helper()
	shardsByID := make(shards)
	for _, shardID := range td.shardIDs() {
		shardState := shard{heldBy: noHost, taskLists: make(routes)}
		holder := td.shardHolder(shardID)
		if holder != nil {
			shardState.heldBy = holder.name
			shardState.load = holder.shardProcessors[shardID].GetShardReport().ShardLoad
		}
		shardsByID[shardID] = shardState
	}
	for _, taskList := range td.taskLists {
		shardsByID[taskList.shard].taskLists[taskList.name] = td.routeOf(t, taskList.name)
	}
	return shardsByID
}

// joinSDOnceOnboarded gives each host its shards once matching lets the host's executor heartbeat
// to SD. A host that stops heartbeating keeps its shards, as its executor no longer cleans them up.
func (td *onboardingTestData) joinSDOnceOnboarded(t *testing.T) {
	for _, host := range td.hosts {
		if host.joinedSD || !host.engine.shardDistributorOnboarded() {
			continue
		}
		host.joinedSD = true
		for shardID, owner := range td.shardOwners {
			if owner == host.name {
				host.addShard(t, shardID)
			}
		}
	}
}

// stopTaskListsHostsNoLongerOwn runs, on every host, the check matching runs when the hash ring's
// membership changes. It stops every task list the resolver no longer routes to the host.
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
			if host.runs(taskListID.GetName()) || !host.owns(t, taskListID) {
				continue
			}
			host.engine.taskListRegistry.Register(*taskListID, manager)
		}
	}
}

// routeOf returns where the resolver sends the task list.
func (td *onboardingTestData) routeOf(t *testing.T, taskListName string) route {
	t.Helper()
	td.routedBy = ""
	owner, err := td.shardDistributorResolver.Lookup(taskListName)
	require.NoError(t, err, "routing task list %q", taskListName)
	return route{routedBy: td.routedBy, owner: owner.Identity()}
}

// requireRunsOnlyOn requires that the task list runs on owner and on no other host, and that it
// didn't stop on owner since the last requireShards.
func (td *onboardingTestData) requireRunsOnlyOn(t *testing.T, taskListName string, owner string) {
	t.Helper()
	var runsOn []string
	for _, host := range td.hosts {
		if host.runs(taskListName) {
			runsOn = append(runsOn, host.name)
		}
	}
	require.Equal(t, []string{owner}, runsOn, "%s is routed to %s, so it should run there and nowhere else", taskListName, owner)

	_, restarted := td.host(owner).stopped.Load(taskListName)
	require.False(t, restarted, "%s should keep running on %s, but it restarted there", taskListName, owner)
}

// shardIDs returns the IDs of the shards SD gives out, in order.
func (td *onboardingTestData) shardIDs() []string {
	return slices.Sorted(maps.Keys(td.shardOwners))
}

// shardHolder returns the host SD has given the shard to, or nil.
func (td *onboardingTestData) shardHolder(shardID string) *onboardingHost {
	for _, host := range td.hosts {
		if _, ok := host.shardProcessors[shardID]; ok {
			return host
		}
	}
	return nil
}

// host returns the host with the name.
func (td *onboardingTestData) host(hostName string) *onboardingHost {
	for _, host := range td.hosts {
		if host.name == hostName {
			return host
		}
	}
	panic(fmt.Sprintf("no host %q in the test", hostName))
}

// addShard creates the shard's processor, as the executor does when SD gives the host a shard.
func (h *onboardingHost) addShard(t *testing.T, shardID string) {
	shardProcessor, err := h.engine.taskListsFactory.NewShardProcessor(shardID)
	require.NoError(t, err)
	h.shardProcessors[shardID] = shardProcessor
}

// removeShard stops the shard's processor, as the executor does when SD takes the shard away.
func (h *onboardingHost) removeShard(shardID string) {
	shardProcessor := h.shardProcessors[shardID]
	delete(h.shardProcessors, shardID)
	shardProcessor.Stop()
}

// owns runs the ownership check matching runs before it starts a task list.
func (h *onboardingHost) owns(t *testing.T, taskListID *tasklist.Identifier) bool {
	err := h.engine.errIfShardOwnershipLost(context.Background(), taskListID)
	if err == nil {
		return true
	}
	var notOwnedErr *cadence_errors.TaskListNotOwnedByHostError
	require.ErrorAs(t, err, &notOwnedErr, "ownership check of task list %q on %s", taskListID.GetName(), h.name)
	return false
}

// runs reports whether the host runs the task list.
func (h *onboardingHost) runs(taskListName string) bool {
	taskListID := h.managers[taskListName].TaskListID()
	_, running := h.engine.taskListRegistry.ManagerByTaskListIdentifier(*taskListID)
	return running
}
