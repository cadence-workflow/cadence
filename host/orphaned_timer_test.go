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

func TestOrphanedTimerIntegrationSuite(t *testing.T) {
	flag.Parse()

	clusterConfig, err := GetTestClusterConfig("testdata/integration_orphaned_timer_cluster.yaml")
	if err != nil {
		t.Fatalf("failed to get cluster config: %v", err)
	}

	// Enable the feature and set threshold to 1 hour so our 48h timeout workflows
	// are above the threshold and get cleaned up, while any short-lived timers are skipped.
	clusterConfig.HistoryDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.EnableOrphanedWorkflowTimerCleanup: true,
		dynamicproperties.OrphanedTimerDeletionMinTTL:        time.Hour,
	}

	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := &OrphanedTimerSuite{}
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *OrphanedTimerSuite) SetupSuite() {
	s.setupSuite()
}

func (s *OrphanedTimerSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *OrphanedTimerSuite) TearDownSuite() {
	s.TearDownBaseSuite()
}

// TestOrphanedTimerCleanupAtWorkflowClose verifies that when a workflow with a long
// execution timeout completes early, its pending timer task is deleted at close time
// by deleteWorkflowTimerTasksBestEffortAsync.
func (s *OrphanedTimerSuite) TestOrphanedTimerCleanupAtWorkflowClose() {
	if TestFlags.PersistenceType != config.StoreTypeCassandra {
		s.T().Skip("orphaned timer cleanup only implemented for Cassandra")
	}

	id := "integration-orphaned-timer-close-test-" + uuid.New()
	wt := "integration-orphaned-timer-close-test-type"
	tl := "integration-orphaned-timer-close-test-tasklist"
	identity := "worker1"

	// 48h execution timeout — well above the 1h OrphanedTimerDeletionMinTTL,
	// so close-time cleanup will delete it rather than skip it.
	ctx, cancel := createContext()
	defer cancel()

	we, err := s.Engine.StartWorkflowExecution(ctx, &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        &types.WorkflowType{Name: wt},
		TaskList:                            &types.TaskList{Name: tl},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(int32(48 * 60 * 60)),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	})
	s.NoError(err)
	runID := we.RunID

	// Complete the workflow immediately.
	poller := s.newCompleteImmediatelyPoller(tl, identity, s.DomainName)
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.NoError(err)

	// The close-time goroutine has a 5s timeout — give it time to complete.
	time.Sleep(10 * time.Second)

	s.True(s.isTimerTaskDeletedForRun(runID),
		"expected 48h workflow timeout timer task to be deleted after workflow close")
}

// TestOrphanedTimerCleanupAtRetention verifies the end state after the full lifecycle:
// the workflow closes (triggering async cleanup) and retention fires (triggering the
// safety-net cleanup). By the time the execution record is gone, the timer must also be gone.
func (s *OrphanedTimerSuite) TestOrphanedTimerCleanupAtRetention() {
	if TestFlags.PersistenceType != config.StoreTypeCassandra {
		s.T().Skip("orphaned timer cleanup only implemented for Cassandra")
	}

	id := "integration-orphaned-timer-retention-test-" + uuid.New()
	wt := "integration-orphaned-timer-retention-test-type"
	tl := "integration-orphaned-timer-retention-test-tasklist"
	identity := "worker1"

	// Domain with 1-day retention so DeleteHistoryEventTask fires quickly in tests.
	domainName := s.RandomizeStr("orphaned-timer-retention-domain")
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

	// By this point either close-time or retention-time cleanup deleted the timer.
	s.True(s.isTimerTaskDeletedForRun(runID),
		"expected 48h workflow timeout timer task to be deleted after full lifecycle")
}

// newCompleteImmediatelyPoller returns a TaskPoller whose decision handler immediately
// completes the workflow.
func (s *OrphanedTimerSuite) newCompleteImmediatelyPoller(taskList, identity, domain string) *TaskPoller {
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
func (s *OrphanedTimerSuite) isTimerTaskDeletedForRun(runID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
	defer cancel()

	tasks, err := s.TestCluster.testBase.GetTimerIndexTasks(ctx, 1000, true)
	if err != nil {
		return false
	}

	for _, task := range tasks {
		if task.GetRunID() == runID {
			return false
		}
	}
	return true
}

// isWorkflowDeleted polls until GetWorkflowExecution returns EntityNotExistsError,
// indicating the retention-based deletion has completed.
func (s *OrphanedTimerSuite) isWorkflowDeleted(domainID string, execution *types.WorkflowExecution) bool {
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
