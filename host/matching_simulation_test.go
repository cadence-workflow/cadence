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

//go:build !race && matchingsim
// +build !race,matchingsim

/*
To run locally:

1. Pick a scenario from the existing config files host/testdata/matching_simulation_.*.yaml or add a new one

2. Run the scenario
`./scripts/run_matching_simulator.sh default`

Full test logs can be found at test.log file. Event json logs can be found at matching-simulator-output folder.
See the run_matching_simulator.sh script for more details about how to parse events.

If you want to run multiple scenarios and compare them refer to tools/matchingsimulationcomparison/README.md
*/
package host

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/yarpc"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/partition"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/types"

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

func TestMatchingSimulationSuite(t *testing.T) {
	flag.Parse()

	confPath := os.Getenv("MATCHING_SIMULATION_CONFIG")
	if confPath == "" {
		confPath = defaultTestCase
	}
	clusterConfig, err := GetTestClusterConfig(confPath)
	if err != nil {
		t.Fatalf("failed creating cluster config from %s, err: %v", confPath, err)
	}

	isolationGroups := getIsolationGroups(&clusterConfig.MatchingConfig.SimulationConfig)

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

	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := new(MatchingSimulationSuite)
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *MatchingSimulationSuite) SetupSuite() {
	s.setupLogger()

	s.Logger.Info("Running integration test against test cluster")
	clusterMetadata := NewClusterMetadata(s.T(), s.testClusterConfig)
	dc := persistence.DynamicConfiguration{
		EnableCassandraAllConsistencyLevelDelete: dynamicconfig.GetBoolPropertyFn(true),
		PersistenceSampleLoggingRate:             dynamicconfig.GetIntPropertyFn(100),
		EnableShardIDMetrics:                     dynamicconfig.GetBoolPropertyFn(true),
	}
	params := pt.TestBaseParams{
		DefaultTestCluster:    s.defaultTestCluster,
		VisibilityTestCluster: s.visibilityTestCluster,
		ClusterMetadata:       clusterMetadata,
		DynamicConfiguration:  dc,
	}
	cluster, err := NewCluster(s.T(), s.testClusterConfig, s.Logger, params)
	s.Require().NoError(err)
	s.testCluster = cluster
	s.engine = s.testCluster.GetFrontendClient()
	s.adminClient = s.testCluster.GetAdminClient()

	s.domainName = s.randomizeStr("integration-test-domain")
	s.Require().NoError(s.registerDomain(s.domainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, ""))
	s.secondaryDomainName = s.randomizeStr("unused-test-domain")
	s.Require().NoError(s.registerDomain(s.secondaryDomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, ""))

	time.Sleep(2 * time.Second)
}

func (s *MatchingSimulationSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *MatchingSimulationSuite) TearDownSuite() {
	s.tearDownSuite()
}

func (s *MatchingSimulationSuite) TestMatchingSimulation() {
	matchingClients := s.testCluster.GetMatchingClients()

	ctx, cancel := context.WithCancel(context.Background())

	domainID := s.domainID(ctx)
	tasklist := "my-tasklist"

	if s.testClusterConfig.MatchingConfig.SimulationConfig.GetPartitionConfigFromDB &&
		!s.testClusterConfig.MatchingConfig.SimulationConfig.EnableAdaptiveScaler {
		_, err := s.testCluster.GetMatchingClient().UpdateTaskListPartitionConfig(ctx, &types.MatchingUpdateTaskListPartitionConfigRequest{
			DomainUUID:   domainID,
			TaskList:     &types.TaskList{Name: tasklist, Kind: types.TaskListKindNormal.Ptr()},
			TaskListType: types.TaskListTypeDecision.Ptr(),
			PartitionConfig: &types.TaskListPartitionConfig{
				NumReadPartitions:  int32(getPartitions(s.testClusterConfig.MatchingConfig.SimulationConfig.TaskListReadPartitions)),
				NumWritePartitions: int32(getPartitions(s.testClusterConfig.MatchingConfig.SimulationConfig.TaskListWritePartitions)),
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

	totalTaskCount := getTotalTasks(s.testClusterConfig.MatchingConfig.SimulationConfig.Tasks)
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	totalBacklogCount := 0
	for idx, backlogConfig := range s.testClusterConfig.MatchingConfig.SimulationConfig.Backlogs {
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
	for idx, pollerConfig := range s.testClusterConfig.MatchingConfig.SimulationConfig.Pollers {
		for i := 0; i < pollerConfig.getNumPollers(); i++ {
			numPollers++
			pollerWG.Add(1)
			pollerID := fmt.Sprintf("[%d]-%s-%d", idx, pollerConfig.getIsolationGroup(), i)
			config := pollerConfig
			go s.poll(ctx, matchingClients[i%len(matchingClients)], domainID, tasklist, pollerID, &pollerWG, statsCh, &tasksToReceive, &config)
		}
	}

	// wait a bit for pollers to start.
	time.Sleep(300 * time.Millisecond)

	startTime := time.Now()
	// Start task generators
	numGenerators := 0
	var generatorWG sync.WaitGroup
	lastTaskScheduleID := int32(0)
	for _, taskConfig := range s.testClusterConfig.MatchingConfig.SimulationConfig.Tasks {
		tasksGenerated := int32(0)
		rateLimiter := rate.NewLimiter(rate.Limit(taskConfig.getTasksPerSecond()), taskConfig.getTasksBurst())
		for i := 0; i < taskConfig.getNumTaskGenerators(); i++ {
			numGenerators++
			generatorWG.Add(1)
			config := taskConfig
			go s.generate(ctx, matchingClients[i%len(matchingClients)], domainID, tasklist, rateLimiter, &tasksGenerated, &lastTaskScheduleID, &generatorWG, statsCh, &config)
		}
	}

	// Let it run until all tasks have been polled.
	// There's a test timeout configured in docker/buildkite/docker-compose-local-matching-simulation.yml that you
	// can change if your test case needs more time
	s.log("Waiting until all tasks are received")
	tasksToReceive.Wait()
	executionTime := time.Now().Sub(startTime)
	s.log("Completed benchmark in %v", time.Now().Sub(startTime))
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
	testSummary = append(testSummary, fmt.Sprintf("Record Decision Task Started Time: %v", s.testClusterConfig.MatchingConfig.SimulationConfig.RecordDecisionTaskStartedTime))
	testSummary = append(testSummary, fmt.Sprintf("Num of Write Partitions: %d", s.testClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingNumTasklistWritePartitions]))
	testSummary = append(testSummary, fmt.Sprintf("Num of Read Partitions: %d", s.testClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingNumTasklistReadPartitions]))
	testSummary = append(testSummary, fmt.Sprintf("Get Num of Partitions from DB: %v", s.testClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingEnableGetNumberOfPartitionsFromCache]))
	testSummary = append(testSummary, fmt.Sprintf("Tasklist load balancer strategy: %v", s.testClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.TasklistLoadBalancerStrategy]))
	testSummary = append(testSummary, fmt.Sprintf("Forwarder Max Outstanding Polls: %d", s.testClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingForwarderMaxOutstandingPolls]))
	testSummary = append(testSummary, fmt.Sprintf("Forwarder Max Outstanding Tasks: %d", s.testClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingForwarderMaxOutstandingTasks]))
	testSummary = append(testSummary, fmt.Sprintf("Forwarder Max RPS: %d", s.testClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingForwarderMaxRatePerSecond]))
	testSummary = append(testSummary, fmt.Sprintf("Forwarder Max Children per Node: %d", s.testClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.MatchingForwarderMaxChildrenPerNode]))
	testSummary = append(testSummary, fmt.Sprintf("Local Poll Wait Time: %v", s.testClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.LocalPollWaitTime]))
	testSummary = append(testSummary, fmt.Sprintf("Local Task Wait Time: %v", s.testClusterConfig.MatchingDynamicConfigOverrides[dynamicconfig.LocalTaskWaitTime]))
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
	matchingClient MatchingClient,
	domainID, tasklist string,
	rateLimiter *rate.Limiter,
	tasksGenerated *int32,
	lastTaskScheduleID *int32,
	wg *sync.WaitGroup,
	statsCh chan *operationStats,
	taskConfig *SimulationTaskConfiguration) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			s.log("Generator done")
			return
		default:
			if err := rateLimiter.Wait(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					s.T().Fatal("Rate limiter failed: ", err)
				}
				return
			}
			newTasksGenerated := int(atomic.AddInt32(tasksGenerated, 1))
			if newTasksGenerated > taskConfig.getMaxTasksToGenerate() {
				s.log("Generated %d tasks so generator will stop", newTasksGenerated)
				return
			}
			isolationGroup := ""
			if len(taskConfig.getIsolationGroups()) > 0 {
				isolationGroup = taskConfig.getIsolationGroups()[newTasksGenerated%len(taskConfig.getIsolationGroups())]
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
	matchingClient MatchingClient,
	domainID, tasklist, pollerID string,
	wg *sync.WaitGroup,
	statsCh chan *operationStats,
	tasksToReceive *sync.WaitGroup,
	pollerConfig *SimulationPollerConfiguration,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			s.log("Poller done")
			return
		default:
			s.log("Poller will initiate a poll")
			reqCtx, cancel := context.WithTimeout(ctx, pollerConfig.getPollTimeout())
			start := time.Now()
			resp, err := matchingClient.PollForDecisionTask(reqCtx, &types.MatchingPollForDecisionTaskRequest{
				DomainUUID: domainID,
				PollerID:   pollerID,
				PollRequest: &types.PollForDecisionTaskRequest{
					TaskList: &types.TaskList{
						Name: tasklist,
						Kind: types.TaskListKindNormal.Ptr(),
					},
				},
				IsolationGroup: pollerConfig.getIsolationGroup(),
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
			time.Sleep(pollerConfig.getTaskProcessTime())
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
	domainDesc, err := s.testCluster.GetFrontendClient().DescribeDomain(reqCtx, &types.DescribeDomainRequest{
		Name: &s.domainName,
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
			partition.IsolationGroupKey: isolationGroup,
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

func getTotalTasks(tasks []SimulationTaskConfiguration) int {
	total := 0
	for _, taskConfiguration := range tasks {
		total += taskConfiguration.getMaxTasksToGenerate()
	}
	return total
}

func getIsolationGroups(c *MatchingSimulationConfig) []any {
	groups := make(map[string]struct{})
	for _, poller := range c.Pollers {
		if poller.getIsolationGroup() != "" {
			groups[poller.getIsolationGroup()] = struct{}{}
		}
	}
	for _, tasks := range c.Tasks {
		for _, group := range tasks.getIsolationGroups() {
			groups[group] = struct{}{}
		}
	}
	var uniqueGroups []any
	for group, _ := range groups {
		uniqueGroups = append(uniqueGroups, group)
	}
	return uniqueGroups
}

func (c *SimulationTaskConfiguration) getNumTaskGenerators() int {
	if c.NumTaskGenerators == 0 {
		return 1
	}
	return c.NumTaskGenerators
}

func (c *SimulationTaskConfiguration) getMaxTasksToGenerate() int {
	if c.MaxTaskToGenerate == 0 {
		return 2000
	}
	return c.MaxTaskToGenerate
}

func (c *SimulationTaskConfiguration) getTasksPerSecond() int {
	if c.TasksPerSecond == 0 {
		return 40
	}
	return c.TasksPerSecond
}

func (c *SimulationTaskConfiguration) getTasksBurst() int {
	if c.TasksBurst == 0 {
		return 1
	}
	return c.TasksBurst
}

func (c *SimulationTaskConfiguration) getIsolationGroups() []string {
	return c.IsolationGroups
}

func getPartitions(i int) int {
	if i == 0 {
		return 1
	}
	return i
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

func (c *SimulationPollerConfiguration) getNumPollers() int {
	if c.NumPollers == 0 {
		return 1
	}
	return c.NumPollers
}

func (c *SimulationPollerConfiguration) getTaskProcessTime() time.Duration {
	if c.TaskProcessTime == 0 {
		return time.Millisecond
	}
	return c.TaskProcessTime
}

func (c *SimulationPollerConfiguration) getPollTimeout() time.Duration {
	if c.PollTimeout == 0 {
		return 15 * time.Second
	}
	return c.PollTimeout
}

func (c *SimulationPollerConfiguration) getIsolationGroup() string {
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
	return fmt.Sprintf("%v%v/%v", common.ReservedTaskListPrefix, root, partition)
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
