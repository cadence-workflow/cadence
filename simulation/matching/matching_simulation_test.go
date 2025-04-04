// Copyright (c) 2018 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

/*
To run locally:

1. Pick a scenario from the existing config files simulation/matching/testdata/matching_simulation_.*.yaml or add a new one

2. Run the scenario
`./simulation/matching/run.sh default`

Full test logs can be found at test.log file. Event json logs can be found at matching-simulator-output folder.
See the run.sh script for more details about how to parse events.

If you want to run multiple scenarios and compare them refer to simulation/matching/comparison/README.md
*/
package matching

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"
	"golang.org/x/exp/slices"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/host"

	_ "github.com/uber/cadence/common/asyncworkflow/queue/kafka" // needed to load kafka asyncworkflow queue
)

type operation string

const (
	operationPollForDecisionTask operation = "PollForDecisionTask"
	operationPollReceivedTask    operation = "PollReceivedTask"
	operationAddDecisionTask     operation = "AddDecisionTask"
	defaultTestCase                        = "testdata/matching_simulation_default.yaml"
)

type operationStats struct {
	op        operation
	dur       time.Duration
	err       error
	timestamp time.Time
}

type operationAggStats struct {
	successCnt    int
	failCnt       int
	totalDuration time.Duration
	maxDuration   time.Duration
	lastUpdated   time.Time
}

type MatchingSimulationSuite struct {
	*require.Assertions
	*host.IntegrationBase
}

func TestMatchingSimulation(t *testing.T) {
	flag.Parse()

	confPath := os.Getenv("MATCHING_SIMULATION_CONFIG")
	if confPath == "" {
		confPath = defaultTestCase
	}
	clusterConfig, err := host.GetTestClusterConfig(confPath)
	if err != nil {
		t.Fatalf("failed creating cluster config from %s, err: %v", confPath, err)
	}

	isolationGroups := getIsolationGroups(clusterConfig.MatchingConfig.SimulationConfig)

	clusterConfig.MatchingDynamicConfigOverrides = map[dynamicconfig.Key]interface{}{
		dynamicconfig.MatchingNumTasklistWritePartitions:           getPartitions(clusterConfig.MatchingConfig.SimulationConfig.TaskListWritePartitions),
		dynamicconfig.MatchingNumTasklistReadPartitions:            getPartitions(clusterConfig.MatchingConfig.SimulationConfig.TaskListReadPartitions),
		dynamicconfig.MatchingForwarderMaxOutstandingPolls:         getForwarderMaxOutstandingPolls(clusterConfig.MatchingConfig.SimulationConfig.ForwarderMaxOutstandingPolls),
		dynamicconfig.MatchingForwarderMaxOutstandingTasks:         getForwarderMaxOutstandingTasks(clusterConfig.MatchingConfig.SimulationConfig.ForwarderMaxOutstandingTasks),
		dynamicconfig.MatchingForwarderMaxRatePerSecond:            getForwarderMaxRPS(clusterConfig.MatchingConfig.SimulationConfig.ForwarderMaxRatePerSecond),
		dynamicconfig.MatchingForwarderMaxChildrenPerNode:          getForwarderMaxChildPerNode(clusterConfig.MatchingConfig.SimulationConfig.ForwarderMaxChildrenPerNode),
		dynamicconfig.LocalPollWaitTime:                            clusterConfig.MatchingConfig.SimulationConfig.LocalPollWaitTime,
		dynamicconfig.LocalTaskWaitTime:                            clusterConfig.MatchingConfig.SimulationConfig.LocalTaskWaitTime,
		dynamicconfig.EnableTasklistIsolation:                      len(isolationGroups) > 0,
		dynamicconfig.AllIsolationGroups:                           isolationGroups,
		dynamicconfig.TasklistLoadBalancerStrategy:                 getTasklistLoadBalancerStrategy(clusterConfig.MatchingConfig.SimulationConfig.TasklistLoadBalancerStrategy),
		dynamicconfig.MatchingEnableGetNumberOfPartitionsFromCache: clusterConfig.MatchingConfig.SimulationConfig.GetPartitionConfigFromDB,
		dynamicconfig.MatchingEnableAdaptiveScaler:                 clusterConfig.MatchingConfig.SimulationConfig.EnableAdaptiveScaler,
		dynamicconfig.MatchingPartitionDownscaleFactor:             clusterConfig.MatchingConfig.SimulationConfig.PartitionDownscaleFactor,
		dynamicconfig.MatchingPartitionUpscaleRPS:                  clusterConfig.MatchingConfig.SimulationConfig.PartitionUpscaleRPS,
		dynamicconfig.MatchingPartitionUpscaleSustainedDuration:    clusterConfig.MatchingConfig.SimulationConfig.PartitionUpscaleSustainedDuration,
		dynamicconfig.MatchingPartitionDownscaleSustainedDuration:  clusterConfig.MatchingConfig.SimulationConfig.PartitionDownscaleSustainedDuration,
		dynamicconfig.MatchingAdaptiveScalerUpdateInterval:         clusterConfig.MatchingConfig.SimulationConfig.AdaptiveScalerUpdateInterval,
		dynamicconfig.MatchingQPSTrackerInterval:                   getQPSTrackerInterval(clusterConfig.MatchingConfig.SimulationConfig.QPSTrackerInterval),
		dynamicconfig.TaskIsolationDuration:                        clusterConfig.MatchingConfig.SimulationConfig.TaskIsolationDuration,
	}

	ctrl := gomock.NewController(t)
	mockHistoryCl := history.NewMockClient(ctrl)
	mockHistoryCl.EXPECT().RecordDecisionTaskStarted(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *types.RecordDecisionTaskStartedRequest, opts ...yarpc.CallOption) (*types.RecordDecisionTaskStartedResponse, error) {
			time.Sleep(getRecordDecisionTaskStartedTime(clusterConfig.MatchingConfig.SimulationConfig.RecordDecisionTaskStartedTime))
			return &types.RecordDecisionTaskStartedResponse{
				ScheduledEventID: req.ScheduleID,
			}, nil
		}).AnyTimes()
	clusterConfig.HistoryConfig.MockClient = mockHistoryCl

	testCluster := host.NewPersistenceTestCluster(t, clusterConfig)

	s := new(MatchingSimulationSuite)
	params := host.IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = host.NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *MatchingSimulationSuite) SetupSuite() {
	s.SetupLogger()

	s.Logger.Info("Running integration test against test cluster")
	clusterMetadata := host.NewClusterMetadata(s.T(), s.TestClusterConfig)
	dc := persistence.DynamicConfiguration{
		EnableCassandraAllConsistencyLevelDelete: dynamicconfig.GetBoolPropertyFn(true),
		PersistenceSampleLoggingRate:             dynamicconfig.GetIntPropertyFn(100),
		EnableShardIDMetrics:                     dynamicconfig.GetBoolPropertyFn(true),
		EnableHistoryTaskDualWriteMode:           dynamicconfig.GetBoolPropertyFn(true),
	}
	params := pt.TestBaseParams{
		DefaultTestCluster:    s.DefaultTestCluster,
		VisibilityTestCluster: s.VisibilityTestCluster,
		ClusterMetadata:       clusterMetadata,
		DynamicConfiguration:  dc,
	}
	cluster, err := host.NewCluster(s.T(), s.TestClusterConfig, s.Logger, params)
	s.Require().NoError(err)
	s.TestCluster = cluster
	s.Engine = s.TestCluster.GetFrontendClient()
	s.AdminClient = s.TestCluster.GetAdminClient()

	s.DomainName = s.RandomizeStr("integration-test-domain")
	s.Require().NoError(s.RegisterDomain(s.DomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, ""))
	s.SecondaryDomainName = s.RandomizeStr("unused-test-domain")
	s.Require().NoError(s.RegisterDomain(s.SecondaryDomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, ""))

	time.Sleep(2 * time.Second)
}

func (s *MatchingSimulationSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *MatchingSimulationSuite) TearDownSuite() {
	// Sleep for a while to ensure all metrics are emitted/scraped by prometheus
	time.Sleep(5 * time.Second)
	s.TearDownBaseSuite()
}

func (s *MatchingSimulationSuite) TestMatchingSimulation() {
	matchingClients := s.TestCluster.GetMatchingClients()

	ctx, cancel := context.WithCancel(context.Background())

	domainID := s.domainID(ctx)
	tasklist := "my-tasklist"

	if s.TestClusterConfig.MatchingConfig.SimulationConfig.GetPartitionConfigFromDB &&
		!s.TestClusterConfig.MatchingConfig.SimulationConfig.EnableAdaptiveScaler {
		_, err := s.TestCluster.GetMatchingClient().UpdateTaskListPartitionConfig(ctx, &types.MatchingUpdateTaskListPartitionConfigRequest{
			DomainUUID:   domainID,
			TaskList:     &types.TaskList{Name: tasklist, Kind: types.TaskListKindNormal.Ptr()},
			TaskListType: types.TaskListTypeDecision.Ptr(),
			PartitionConfig: &types.TaskListPartitionConfig{
				ReadPartitions:  getPartitions(s.TestClusterConfig.MatchingConfig.SimulationConfig.TaskListReadPartitions),
				WritePartitions: getPartitions(s.TestClusterConfig.MatchingConfig.SimulationConfig.TaskListWritePartitions),
			},
		})
		s.NoError(err)
	}

	// Start stat collector
	statsCh := make(chan *operationStats, 200000)
	aggStats := make(map[operation]*operationAggStats)
	var collectorWG sync.WaitGroup
	collectorWG.Add(1)
	go s.collectStats(statsCh, aggStats, &collectorWG)

	totalTaskCount := getTotalTasks(s.TestClusterConfig.MatchingConfig.SimulationConfig.Tasks)
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	totalBacklogCount := 0
	for idx, backlogConfig := range s.TestClusterConfig.MatchingConfig.SimulationConfig.Backlogs {
		totalBacklogCount += backlogConfig.BacklogCount
		partition := getPartitionTaskListName(tasklist, backlogConfig.Partition)
		for i := 0; i < backlogConfig.BacklogCount; i++ {
			isolationGroup := ""
			if len(backlogConfig.IsolationGroups) > 0 {
				isolationGroup = randomlyPickKey(backlogConfig.IsolationGroups)
			}
			decisionTask := newDecisionTask(domainID, partition, isolationGroup, idx)
			reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			_, err := matchingClients[0].AddDecisionTask(reqCtx, decisionTask)
			cancel()
			if err != nil {
				s.log("Error when adding decision task, err: %v", err)
			}
		}
	}

	// Start pollers
	numPollers := 0
	var tasksToReceive sync.WaitGroup
	tasksToReceive.Add(totalTaskCount + totalBacklogCount)
	var pollerWG sync.WaitGroup
	for idx, pollerConfig := range s.TestClusterConfig.MatchingConfig.SimulationConfig.Pollers {
		for i := 0; i < getNumPollers(pollerConfig); i++ {
			numPollers++
			pollerWG.Add(1)
			pollerID := fmt.Sprintf("[%d]-%s-%d", idx, getIsolationGroup(pollerConfig), i)
			config := pollerConfig
			go s.poll(ctx, matchingClients[i%len(matchingClients)], domainID, tasklist, pollerID, &pollerWG, statsCh, &tasksToReceive, config)
		}
	}

	// wait a bit for pollers to start.
	time.Sleep(300 * time.Millisecond)

	startTime := time.Now()
	// Start task generators
	numGenerators := 0
	var generatorWG sync.WaitGroup
	lastTaskScheduleID := int32(0)
	for _, taskConfig := range s.TestClusterConfig.MatchingConfig.SimulationConfig.Tasks {
		tasksGenerated := int32(0)
		rateLimiter := newSimulationRateLimiter(taskConfig, startTime, clock.NewRealTimeSource(), s.log)
		for i := 0; i < getNumTaskGenerators(taskConfig); i++ {
			numGenerators++
			generatorWG.Add(1)
			config := taskConfig
			go s.generate(
				ctx,
				matchingClients[i%len(matchingClients)],
				domainID,
				tasklist,
				&tasksGenerated,
				&lastTaskScheduleID,
				&generatorWG,
				statsCh,
				config,
				rateLimiter,
			)
		}
	}

	// Let it run until all tasks have been polled.
	// There's a test timeout configured in docker/buildkite/docker-compose-local-matching-simulation.yml that you
	// can change if your test case needs more time
	s.log("Waiting until all tasks are received")
	tasksToReceive.Wait()
	executionTime := time.Since(startTime)
	s.log("Completed benchmark in %v", executionTime)
	s.log("Canceling context to stop pollers and task generators")
	cancel()
	pollerWG.Wait()
	s.log("Pollers stopped")
	generatorWG.Wait()
	s.log("Generators stopped")
	s.log("Stopping stats collector")
	close(statsCh)
	collectorWG.Wait()
	s.log("Stats collector stopped")

	// Print the test summary.
	// Don't change the start/end line format as it is used by scripts to parse the summary info
	testSummary := []string{}
	testSummary = append(testSummary, "Simulation Summary:")
	testSummary = append(testSummary, fmt.Sprintf("Random seed: %v", seed))
	testSummary = append(testSummary, fmt.Sprintf("Task generate Duration: %v", aggStats[operationAddDecisionTask].lastUpdated.Sub(startTime)))
	testSummary = append(testSummary, fmt.Sprintf("Simulation Duration: %v", executionTime))
	testSummary = append(testSummary, fmt.Sprintf("Num of Pollers: %d", numPollers))
	testSummary = append(testSummary, fmt.Sprintf("Num of Task Generators: %d", numGenerators))
	testSummary = append(testSummary, fmt.Sprintf("Record Decision Task Started Time: %v", s.TestClusterConfig.MatchingConfig.SimulationConfig.RecordDecisionTaskStartedTime))
	testSummary = append(testSummary, fmt.Sprintf("Num of Write Partitions: %d", s.TestClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingNumTasklistWritePartitions]))
	testSummary = append(testSummary, fmt.Sprintf("Num of Read Partitions: %d", s.TestClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingNumTasklistReadPartitions]))
	testSummary = append(testSummary, fmt.Sprintf("Get Num of Partitions from DB: %v", s.TestClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingEnableGetNumberOfPartitionsFromCache]))
	testSummary = append(testSummary, fmt.Sprintf("Tasklist load balancer strategy: %v", s.TestClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.TasklistLoadBalancerStrategy]))
	testSummary = append(testSummary, fmt.Sprintf("Forwarder Max Outstanding Polls: %d", s.TestClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingForwarderMaxOutstandingPolls]))
	testSummary = append(testSummary, fmt.Sprintf("Forwarder Max Outstanding Tasks: %d", s.TestClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingForwarderMaxOutstandingTasks]))
	testSummary = append(testSummary, fmt.Sprintf("Forwarder Max RPS: %d", s.TestClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingForwarderMaxRatePerSecond]))
	testSummary = append(testSummary, fmt.Sprintf("Forwarder Max Children per Node: %d", s.TestClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingForwarderMaxChildrenPerNode]))
	testSummary = append(testSummary, fmt.Sprintf("Local Poll Wait Time: %v", s.TestClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.LocalPollWaitTime]))
	testSummary = append(testSummary, fmt.Sprintf("Local Task Wait Time: %v", s.TestClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.LocalTaskWaitTime]))
	testSummary = append(testSummary, fmt.Sprintf("Tasks generated: %d", aggStats[operationAddDecisionTask].successCnt))
	testSummary = append(testSummary, fmt.Sprintf("Tasks polled: %d", aggStats[operationPollReceivedTask].successCnt))

	testSummary = appendMetric(testSummary, operationPollForDecisionTask, aggStats)
	testSummary = appendMetric(testSummary, operationAddDecisionTask, aggStats)

	testSummary = append(testSummary, "End of Simulation Summary")
	fmt.Println(strings.Join(testSummary, "\n"))
}

func (s *MatchingSimulationSuite) log(msg string, args ...interface{}) {
	msg = time.Now().Format(time.RFC3339Nano) + "\t" + msg
	s.T().Logf(msg, args...)
}

func (s *MatchingSimulationSuite) generate(
	ctx context.Context,
	matchingClient host.MatchingClient,
	domainID, tasklist string,
	tasksGenerated *int32,
	lastTaskScheduleID *int32,
	wg *sync.WaitGroup,
	statsCh chan *operationStats,
	taskConfig host.SimulationTaskConfiguration,
	rateLimiter *simulationRateLimiter,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			s.log("Generator done")
			return
		default:
			if err := rateLimiter.Wait(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					s.T().Error("Rate limiter failed: ", err)
				}
				return
			}
			newTasksGenerated := int(atomic.AddInt32(tasksGenerated, 1))
			if newTasksGenerated > getMaxTasksToGenerate(taskConfig) {
				s.log("Generated %d tasks so generator will stop", newTasksGenerated)
				return
			}
			isolationGroup := ""
			if igs := getTaskIsolationGroups(taskConfig); len(igs) > 0 {
				isolationGroup = igs[newTasksGenerated%len(igs)]
			}
			scheduleID := int(atomic.AddInt32(lastTaskScheduleID, 1))
			start := time.Now()
			decisionTask := newDecisionTask(domainID, tasklist, isolationGroup, scheduleID)
			reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			_, err := matchingClient.AddDecisionTask(reqCtx, decisionTask)
			statsCh <- &operationStats{
				op:        operationAddDecisionTask,
				dur:       time.Since(start),
				err:       err,
				timestamp: time.Now(),
			}
			cancel()
			if err != nil {
				s.log("Error when adding decision task, err: %v", err)
				continue
			}
		}
	}
}

func (s *MatchingSimulationSuite) poll(
	ctx context.Context,
	matchingClient host.MatchingClient,
	domainID, tasklist, pollerID string,
	wg *sync.WaitGroup,
	statsCh chan *operationStats,
	tasksToReceive *sync.WaitGroup,
	pollerConfig host.SimulationPollerConfiguration,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			s.log("Poller done")
			return
		default:
			s.log("Poller will initiate a poll")
			reqCtx, cancel := context.WithTimeout(ctx, getPollTimeout(pollerConfig))
			start := time.Now()
			resp, err := matchingClient.PollForDecisionTask(reqCtx, &types.MatchingPollForDecisionTaskRequest{
				DomainUUID: domainID,
				PollerID:   pollerID,
				PollRequest: &types.PollForDecisionTaskRequest{
					TaskList: &types.TaskList{
						Name: tasklist,
						Kind: types.TaskListKindNormal.Ptr(),
					},
					Identity: pollerID,
				},
				IsolationGroup: getIsolationGroup(pollerConfig),
			})
			cancel()

			statsCh <- &operationStats{
				op:        operationPollForDecisionTask,
				dur:       time.Since(start),
				err:       err,
				timestamp: time.Now(),
			}

			if err != nil {
				s.log("PollForDecisionTask failed: %v", err)
				continue
			}

			empty := &types.MatchingPollForDecisionTaskResponse{}

			if reflect.DeepEqual(empty, resp) {
				s.log("PollForDecisionTask response is empty")
				continue
			}

			statsCh <- &operationStats{
				op:        operationPollReceivedTask,
				timestamp: time.Now(),
			}

			s.log("PollForDecisionTask got a task with startedid: %d. resp: %+v", resp.StartedEventID, resp)
			tasksToReceive.Done()
			time.Sleep(getTaskProcessTime(pollerConfig))
		}
	}
}

func (s *MatchingSimulationSuite) collectStats(statsCh chan *operationStats, aggStats map[operation]*operationAggStats, wg *sync.WaitGroup) {
	defer wg.Done()
	for stat := range statsCh {
		opAggStats, ok := aggStats[stat.op]
		if !ok {
			opAggStats = &operationAggStats{}
			aggStats[stat.op] = opAggStats
		}
		if stat.timestamp.After(opAggStats.lastUpdated) {
			opAggStats.lastUpdated = stat.timestamp
		}
		if stat.err != nil {
			opAggStats.failCnt++
		} else {
			opAggStats.successCnt++
		}

		opAggStats.totalDuration += stat.dur
		if stat.dur > opAggStats.maxDuration {
			opAggStats.maxDuration = stat.dur
		}
	}

	s.log("Stats collector done")
}

func (s *MatchingSimulationSuite) domainID(ctx context.Context) string {
	reqCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	domainDesc, err := s.TestCluster.GetFrontendClient().DescribeDomain(reqCtx, &types.DescribeDomainRequest{
		Name: &s.DomainName,
	})
	s.Require().NoError(err, "Error when describing domain")

	domainID := domainDesc.GetDomainInfo().UUID
	s.T().Logf("DomainID: %s", domainID)
	return domainID
}

func newDecisionTask(domainID, tasklist, isolationGroup string, i int) *types.AddDecisionTaskRequest {
	return &types.AddDecisionTaskRequest{
		DomainUUID: domainID,
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      uuid.New(),
		},
		TaskList: &types.TaskList{
			Name: tasklist,
			Kind: types.TaskListKindNormal.Ptr(),
		},
		ScheduleID: int64(i),
		PartitionConfig: map[string]string{
			isolationgroup.GroupKey: isolationGroup,
		},
	}
}

func appendMetric(testSummary []string, op operation, aggStats map[operation]*operationAggStats) []string {
	total := 0
	if pollStats, ok := aggStats[op]; ok {
		total = pollStats.successCnt + pollStats.failCnt
	}
	testSummary = append(testSummary, fmt.Sprintf("Operation Summary (%v): ", op))

	if total == 0 {
		testSummary = append(testSummary, "  N/A")
	} else {
		testSummary = append(testSummary, fmt.Sprintf("  Total: %d", total))
		testSummary = append(testSummary, fmt.Sprintf("  Failure rate %%: %d", 100*aggStats[op].failCnt/total))
		testSummary = append(testSummary, fmt.Sprintf("  Avg duration (ms): %d", (aggStats[op].totalDuration/time.Duration(total)).Milliseconds()))
		testSummary = append(testSummary, fmt.Sprintf("  Max duration (ms): %d", aggStats[op].maxDuration.Milliseconds()))
	}
	return testSummary
}

func getTotalTasks(tasks []host.SimulationTaskConfiguration) int {
	total := 0
	for _, taskConfiguration := range tasks {
		total += getMaxTasksToGenerate(taskConfiguration)
	}
	return total
}

func getIsolationGroups(c host.MatchingSimulationConfig) []any {
	groups := make(map[string]struct{})
	for _, poller := range c.Pollers {
		if getIsolationGroup(poller) != "" {
			groups[getIsolationGroup(poller)] = struct{}{}
		}
	}
	for _, task := range c.Tasks {
		for _, group := range getTaskIsolationGroups(task) {
			groups[group] = struct{}{}
		}
	}
	var uniqueGroups []any
	for group := range groups {
		uniqueGroups = append(uniqueGroups, group)
	}
	return uniqueGroups
}

func getNumTaskGenerators(c host.SimulationTaskConfiguration) int {
	if c.NumTaskGenerators == 0 {
		return 1
	}
	return c.NumTaskGenerators
}

func getMaxTasksToGenerate(c host.SimulationTaskConfiguration) int {
	if c.MaxTaskToGenerate == 0 {
		return 2000
	}
	return c.MaxTaskToGenerate
}

func getTasksPerSecond(c host.SimulationTaskConfiguration) int {
	if c.TasksPerSecond == 0 {
		return 40
	}
	return c.TasksPerSecond
}

func getTasksBurst(c host.SimulationTaskConfiguration) int {
	if c.TasksBurst == 0 {
		return 1
	}
	return c.TasksBurst
}

func getTaskIsolationGroups(c host.SimulationTaskConfiguration) []string {
	return c.IsolationGroups
}

func getPartitions(i int) map[int]*types.TaskListPartition {
	if i == 0 {
		return map[int]*types.TaskListPartition{
			0: {},
		}
	}
	result := make(map[int]*types.TaskListPartition)
	for j := 0; j < i; j++ {
		result[j] = &types.TaskListPartition{}
	}
	return result
}

func getForwarderMaxOutstandingPolls(i int) int {
	return i
}

func getForwarderMaxOutstandingTasks(i int) int {
	return i
}

func getForwarderMaxRPS(i int) int {
	if i == 0 {
		return 10
	}
	return i
}

func getForwarderMaxChildPerNode(i int) int {
	if i == 0 {
		return 20
	}
	return i
}

func getNumPollers(c host.SimulationPollerConfiguration) int {
	if c.NumPollers == 0 {
		return 1
	}
	return c.NumPollers
}

func getTaskProcessTime(c host.SimulationPollerConfiguration) time.Duration {
	if c.TaskProcessTime == 0 {
		return time.Millisecond
	}
	return c.TaskProcessTime
}

func getPollTimeout(c host.SimulationPollerConfiguration) time.Duration {
	if c.PollTimeout == 0 {
		return 15 * time.Second
	}
	return c.PollTimeout
}

func getIsolationGroup(c host.SimulationPollerConfiguration) string {
	return c.IsolationGroup
}

func getRecordDecisionTaskStartedTime(duration time.Duration) time.Duration {
	if duration == 0 {
		return time.Millisecond
	}

	return duration
}

func getTasklistLoadBalancerStrategy(strategy string) string {
	if strategy == "" {
		return "random"
	}
	return strategy
}

func getPartitionTaskListName(root string, partition int) string {
	if partition <= 0 {
		return root
	}
	return fmt.Sprintf("%v%v/%v", constants.ReservedTaskListPrefix, root, partition)
}

func getQPSTrackerInterval(duration time.Duration) time.Duration {
	if duration == 0 {
		return 10 * time.Second
	}
	return duration
}

func randomlyPickKey(weights map[string]int) string {
	// Calculate the total weight
	totalWeight := 0
	for _, weight := range weights {
		totalWeight += weight
	}

	// Generate a random number between 0 and totalWeight - 1
	randomWeight := rand.Intn(totalWeight)

	// Iterate through the map to find the key corresponding to the random weight
	for key, weight := range weights {
		if randomWeight < weight {
			return key
		}
		randomWeight -= weight
	}

	// Return an empty string as a fallback (should not happen if weights are positive)
	return ""
}

type rateLimiterForTimeRange struct {
	limiter    *rate.Limiter
	start, end int
}

func (r *rateLimiterForTimeRange) String() string {
	return fmt.Sprintf("{start: %d, end: %d}", r.start, r.end)
}

type simulationRateLimiter struct {
	startTime    time.Time
	timeSrc      clock.TimeSource
	rateLimiters []*rateLimiterForTimeRange
	logFn        func(msg string, args ...interface{})
}

func newSimulationRateLimiter(
	taskConfig host.SimulationTaskConfiguration,
	startTime time.Time,
	timeSrc clock.TimeSource,
	logFn func(msg string, args ...interface{}),
) *simulationRateLimiter {
	var rateLimiters []*rateLimiterForTimeRange
	if len(taskConfig.OverTime) == 0 {
		l := rate.NewLimiter(rate.Limit(getTasksPerSecond(taskConfig)), getTasksBurst(taskConfig))
		rateLimiters = append(rateLimiters, &rateLimiterForTimeRange{limiter: l, start: 0, end: -1})
	} else {
		start := 0
		for _, spec := range taskConfig.OverTime {
			l := rate.NewLimiter(rate.Limit(spec.TasksPerSecond), spec.TasksBurst)
			end := -1
			if spec.Duration != nil {
				end = start + int(spec.Duration.Seconds())
			}
			rateLimiters = append(rateLimiters, &rateLimiterForTimeRange{limiter: l, start: start, end: end})
			start = end
		}
	}

	sort.Slice(rateLimiters, func(i, j int) bool {
		return rateLimiters[i].start < rateLimiters[j].start
	})

	logFn("Rate limiters: %v", rateLimiters)

	return &simulationRateLimiter{
		startTime:    startTime,
		timeSrc:      timeSrc,
		rateLimiters: rateLimiters,
		logFn:        logFn,
	}
}

func (r *simulationRateLimiter) Wait(ctx context.Context) error {
	limiter, err := r.getLimiter()
	if err != nil {
		return err
	}

	return limiter.limiter.Wait(ctx)
}

func (r *simulationRateLimiter) getLimiter() (*rateLimiterForTimeRange, error) {
	elapsed := int(r.timeSrc.Since(r.startTime).Seconds())
	idx, ok := slices.BinarySearchFunc(r.rateLimiters, elapsed, func(r *rateLimiterForTimeRange, t int) int {
		if t >= r.start && (r.end == -1 || t < r.end) {
			return 0
		}

		if r.start > t {
			return 1
		}

		return -1
	})

	if !ok {
		return nil, fmt.Errorf("rate limiter not found, elapsed: %ds", elapsed)
	}

	r.logFn("Elapsed %vs so using rate limiter at index %d", elapsed, idx)
	return r.rateLimiters[idx], nil
}

func TestMatchingSimulation_RateLimiterBST(t *testing.T) {
	mockTimeSrc := clock.NewMockedTimeSource()
	srl := &simulationRateLimiter{
		startTime: mockTimeSrc.Now(),
		timeSrc:   mockTimeSrc,
		rateLimiters: []*rateLimiterForTimeRange{
			{limiter: rate.NewLimiter(rate.Limit(10), 1), start: 0, end: 5},
			{limiter: rate.NewLimiter(rate.Limit(10), 1), start: 5, end: -1},
		},
		logFn: t.Logf,
	}

	// t = 0
	l, err := srl.getLimiter()
	require.NoError(t, err)
	require.Equal(t, 0, l.start) // limiter at index 0 should be used

	// t = 3
	mockTimeSrc.Advance(time.Second * 3)
	l, err = srl.getLimiter()
	require.NoError(t, err)
	require.Equal(t, 0, l.start) // limiter at index 0 should be used

	// t = 5
	mockTimeSrc.Advance(time.Second * 2)
	l, err = srl.getLimiter()
	require.NoError(t, err)
	require.Equal(t, 5, l.start) // limiter at index 1 should be used

	// t = 10
	mockTimeSrc.Advance(time.Second * 5)
	l, err = srl.getLimiter()
	require.NoError(t, err)
	require.Equal(t, 5, l.start) // limiter at index 1 should be used
}
