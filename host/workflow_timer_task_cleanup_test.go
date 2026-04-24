// Copyright (c) 2017-2020 Uber Technologies, Inc.
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

package host

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestWorkflowTimerTaskCleanupIntegrationSuite(t *testing.T) {
	flag.Parse()

	clusterConfig, err := GetTestClusterConfig("testdata/integration_timer_cleanup_cluster.yaml")
	if err != nil {
		t.Fatalf("failed to get cluster config: %v", err)
	}

	// Enable the feature and set threshold to 1 hour so our 48h timeout workflows
	// are above the threshold and get cleaned up, while any short-lived timers are skipped.
	clusterConfig.HistoryDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.EnableWorkflowTimerTaskCleanup: true,
		dynamicproperties.WorkflowTimerTaskCleanupMinTTL: time.Hour,
	}

	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := &WorkflowTimerTaskCleanupSuite{}
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *WorkflowTimerTaskCleanupSuite) SetupSuite() {
	s.setupSuite()
}

func (s *WorkflowTimerTaskCleanupSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *WorkflowTimerTaskCleanupSuite) TearDownSuite() {
	s.TearDownBaseSuite()
}

// TestTimerCleanupAtRetention verifies that timer tasks are deleted when the workflow
// execution record is cleaned up at the end of the retention period.
func (s *WorkflowTimerTaskCleanupSuite) TestTimerCleanupAtRetention() {
	if TestFlags.PersistenceType != config.StoreTypeCassandra {
		s.T().Skip("workflow timer task cleanup only implemented for Cassandra")
	}

	id := "integration-timer-cleanup-retention-test-" + uuid.New()
	wt := "integration-timer-cleanup-retention-test-type"
	tl := "integration-timer-cleanup-retention-test-tasklist"
	identity := "worker1"

	// Domain with 1-day retention so DeleteHistoryEventTask fires quickly in tests.
	domainName := s.RandomizeStr("timer-cleanup-retention-domain")
	s.NoError(s.RegisterDomain(domainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))

	ctx, cancel := createContext()
	defer cancel()

	we, err := s.Engine.StartWorkflowExecution(ctx, &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              domainName,
		WorkflowID:                          id,
		WorkflowType:                        &types.WorkflowType{Name: wt},
		TaskList:                            &types.TaskList{Name: tl},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(int32(48 * 60 * 60)),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	})
	s.NoError(err)
	runID := we.RunID

	poller := s.newCompleteImmediatelyPoller(tl, identity, domainName)
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.NoError(err)

	// Wait for retention-based deletion: mutable state gone means retention fired.
	domainResp, err := s.Engine.DescribeDomain(ctx, &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.NoError(err)
	domainID := domainResp.DomainInfo.GetUUID()

	execution := &types.WorkflowExecution{WorkflowID: id, RunID: runID}
	s.True(s.isWorkflowDeleted(domainID, execution),
		"expected workflow execution to be deleted after retention")

	// By this point retention-time cleanup deleted the timer.
	s.True(s.isTimerTaskDeletedForRun(runID),
		"expected 48h workflow timeout timer task to be deleted after full lifecycle")
}

// newCompleteImmediatelyPoller returns a TaskPoller whose decision handler immediately
// completes the workflow.
func (s *WorkflowTimerTaskCleanupSuite) newCompleteImmediatelyPoller(taskList, identity, domain string) *TaskPoller {
	return &TaskPoller{
		Engine:   s.Engine,
		Domain:   domain,
		TaskList: &types.TaskList{Name: taskList},
		Identity: identity,
		DecisionHandler: func(
			_ *types.WorkflowExecution,
			_ *types.WorkflowType,
			_, _ int64,
			_ *types.History,
		) ([]byte, []*types.Decision, error) {
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
				CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
					Result: []byte("done"),
				},
			}}, nil
		},
		Logger: s.Logger,
		T:      s.T(),
	}
}

// isTimerTaskDeletedForRun returns true if no timer task for the given runID exists in the queue.
func (s *WorkflowTimerTaskCleanupSuite) isTimerTaskDeletedForRun(runID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
	defer cancel()

	tasks, err := s.TestCluster.testBase.GetTimerIndexTasks(ctx, 1000, true)
	s.NoError(err)

	for _, task := range tasks {
		if task.GetRunID() == runID {
			return false
		}
	}
	return true
}

// isWorkflowDeleted polls until GetWorkflowExecution returns EntityNotExistsError,
// indicating the retention-based deletion has completed.
func (s *WorkflowTimerTaskCleanupSuite) isWorkflowDeleted(domainID string, execution *types.WorkflowExecution) bool {
	request := &persistence.GetWorkflowExecutionRequest{
		DomainID:  domainID,
		Execution: *execution,
	}
	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
		_, err := s.TestCluster.testBase.ExecutionManager.GetWorkflowExecution(ctx, request)
		cancel()
		if _, ok := err.(*types.EntityNotExistsError); ok {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func TestWorkflowTimerTaskCleanupDisabledIntegrationSuite(t *testing.T) {
	flag.Parse()

	clusterConfig, err := GetTestClusterConfig("testdata/integration_timer_cleanup_cluster.yaml")
	if err != nil {
		t.Fatalf("failed to get cluster config: %v", err)
	}

	// Explicitly disable the feature so we can verify the positive test is not
	// passing due to some incidental cleanup path.
	clusterConfig.HistoryDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.EnableWorkflowTimerTaskCleanup: false,
	}

	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := &WorkflowTimerTaskCleanupDisabledSuite{}
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *WorkflowTimerTaskCleanupDisabledSuite) SetupSuite() {
	s.setupSuite()
}

func (s *WorkflowTimerTaskCleanupDisabledSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *WorkflowTimerTaskCleanupDisabledSuite) TearDownSuite() {
	s.TearDownBaseSuite()
}

// TestTimerNotCleanedWhenDisabled verifies that with the feature flag off, timer tasks
// persist after the workflow execution record is deleted — confirming that
// TestTimerCleanupAtRetention passes because of the feature, not incidental cleanup.
func (s *WorkflowTimerTaskCleanupDisabledSuite) TestTimerNotCleanedWhenDisabled() {
	if TestFlags.PersistenceType != config.StoreTypeCassandra {
		s.T().Skip("workflow timer task cleanup only implemented for Cassandra")
	}

	id := "integration-timer-cleanup-disabled-test-" + uuid.New()
	wt := "integration-timer-cleanup-disabled-test-type"
	tl := "integration-timer-cleanup-disabled-test-tasklist"
	identity := "worker1"

	domainName := s.RandomizeStr("timer-cleanup-disabled-domain")
	s.NoError(s.RegisterDomain(domainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))

	ctx, cancel := createContext()
	defer cancel()

	we, err := s.Engine.StartWorkflowExecution(ctx, &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              domainName,
		WorkflowID:                          id,
		WorkflowType:                        &types.WorkflowType{Name: wt},
		TaskList:                            &types.TaskList{Name: tl},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(int32(48 * 60 * 60)),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	})
	s.NoError(err)
	runID := we.RunID

	poller := s.newCompleteImmediatelyPoller(tl, identity, domainName)
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.NoError(err)

	// Confirm the 48h timer task exists immediately after completion, before retention fires.
	// If this fails, the timer was never written or was deleted during workflow completion.
	{
		ctx2, cancel2 := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
		tasks, err := s.TestCluster.testBase.GetTimerIndexTasks(ctx2, 1000, true)
		cancel2()
		s.NoError(err)
		var foundRunIDs []string
		for _, task := range tasks {
			foundRunIDs = append(foundRunIDs, task.GetRunID())
		}
		s.Contains(foundRunIDs, runID,
			"expected 48h timer task for runID %s to exist right after completion; runIDs in queue: %v", runID, foundRunIDs)
	}

	// Also verify testBase can see the workflow execution record (confirms same Cassandra).
	{
		domainResp2, err := s.Engine.DescribeDomain(ctx, &types.DescribeDomainRequest{Name: common.StringPtr(domainName)})
		s.NoError(err)
		domainID2 := domainResp2.DomainInfo.GetUUID()
		ctx3, cancel3 := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
		_, execErr := s.TestCluster.testBase.ExecutionManager.GetWorkflowExecution(ctx3, &persistence.GetWorkflowExecutionRequest{
			DomainID:  domainID2,
			Execution: types.WorkflowExecution{WorkflowID: id, RunID: runID},
		})
		cancel3()
		s.NoError(execErr, "testBase.ExecutionManager should see the workflow execution record")
	}

	domainResp, err := s.Engine.DescribeDomain(ctx, &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	})
	s.NoError(err)
	domainID := domainResp.DomainInfo.GetUUID()

	execution := &types.WorkflowExecution{WorkflowID: id, RunID: runID}
	s.True(s.isWorkflowDeleted(domainID, execution),
		"expected workflow execution to be deleted after retention")

	// Flag is off: deletion never runs, so the 48h timer task should still be present.
	s.False(s.isTimerTaskDeletedForRun(runID),
		"expected 48h timer task to remain when feature flag is disabled")
}

func (s *WorkflowTimerTaskCleanupDisabledSuite) newCompleteImmediatelyPoller(taskList, identity, domain string) *TaskPoller {
	return &TaskPoller{
		Engine:   s.Engine,
		Domain:   domain,
		TaskList: &types.TaskList{Name: taskList},
		Identity: identity,
		DecisionHandler: func(
			_ *types.WorkflowExecution,
			_ *types.WorkflowType,
			_, _ int64,
			_ *types.History,
		) ([]byte, []*types.Decision, error) {
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
				CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
					Result: []byte("done"),
				},
			}}, nil
		},
		Logger: s.Logger,
		T:      s.T(),
	}
}

func (s *WorkflowTimerTaskCleanupDisabledSuite) isTimerTaskDeletedForRun(runID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
	defer cancel()

	tasks, err := s.TestCluster.testBase.GetTimerIndexTasks(ctx, 1000, true)
	s.NoError(err)

	for _, task := range tasks {
		if task.GetRunID() == runID {
			return false
		}
	}
	return true
}

func (s *WorkflowTimerTaskCleanupDisabledSuite) isWorkflowDeleted(domainID string, execution *types.WorkflowExecution) bool {
	request := &persistence.GetWorkflowExecutionRequest{
		DomainID:  domainID,
		Execution: *execution,
	}
	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
		_, err := s.TestCluster.testBase.ExecutionManager.GetWorkflowExecution(ctx, request)
		cancel()
		if _, ok := err.(*types.EntityNotExistsError); ok {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
