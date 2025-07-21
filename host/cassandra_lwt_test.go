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

//go:build !race && cassandralwt
// +build !race,cassandralwt

/*
To run locally:

1. Stop the previous run if any

	docker compose -f docker/buildkite/docker-compose-cassandra-lwt.yml down

2. Build the integration-test-async-wf image

	docker compose -f docker/buildkite/docker-compose-cassandra-lwt.yml build test-cass-lwt

3. Run the test in the docker container

	docker compose -f docker/buildkite/docker-compose-cassandra-lwt.yml run --rm test-cass-lwt

4. Full test run logs can be found at test.log file
*/

/*
-- Benchmark Results --

Single node cassandra cluster, maxConns=2, replicas=1:
	Total updates: 1000, concurrency: 1, success: 1000, failed: 0, avg duration: 7.41ms, max duration: 26.9ms, min duration: 4.8ms, elapsed: 7.42s
	Total updates: 1000, concurrency: 10, success: 1000, failed: 0, avg duration: 54.54ms, max duration: 808.2ms, min duration: 2.8ms, elapsed: 5.16s
	Total updates: 1000, concurrency: 20, success: 999, failed: 1, avg duration: 90.61ms, max duration: 1.075s, min duration: 2.8ms, elapsed: 4.63s
	Total updates: 1000, concurrency: 40, success: 993, failed: 7, avg duration: 145.95ms, max duration: 1.071s, min duration: 2.2ms, elapsed: 3.78s
	Total updates: 1000, concurrency: 80, success: 976, failed: 24, avg duration: 254.02ms, max duration: 1.093s, min duration: 1.4ms, elapsed: 3.36s

Two nodes cassandra cluster, maxConns=2, replicas=1:
	Total updates: 1000, concurrency: 10, success: 1000, failed: 0, avg duration: 52.37s, max duration: 681.20ms, min duration: 2.91ms, elapsed: 5.29s
	Total updates: 1000, concurrency: 100, success: 949, failed: 51, avg duration: 283.95ms, max duration: 1.11s, min duration: 2.21ms, elapsed: 3.09s

Two nodes cassandra cluster, maxConns=2, replicas=2:
	Total updates: 1000, concurrency: 10, success: 898, failed: 102, avg duration: 367.31ms, max duration: 1.12s, min duration: 7.98ms, elapsed: 36.88s
    Total updates: 1000, concurrency: 60, success: 160, failed: 840, avg duration: 951.3ms, max duration: 1.11s, min duration: 8.99ms, elapsed: 16.35s
    Total updates: 1000, concurrency: 80, success: 71, failed: 929, avg duration: 1.00s, max duration: 1.12s, min duration: 8.10ms, elapsed: 13.02s
	Total updates: 1000, concurrency: 100, success: 38, failed: 962, avg duration: 1.02s, max duration: 1.11s, min duration: 7.89ms, elapsed: 10.52s

Two nodes cassandra cluster, maxConns=0, replicas=2:
    Total updates: 1000, concurrency: 1, success: 1000, failed: 0, avg duration: 11.75ms, max duration: 50.09ms, min duration: 6.69ms, elapsed: 11.76s
    Total updates: 1000, concurrency: 10, success: 900, failed: 100, avg duration: 370.96ms, max duration: 1.12s, min duration: 7.92ms, elapsed: 37.31s
    Total updates: 1000, concurrency: 20, success: 642, failed: 358, avg duration: 602.63ms, max duration: 1.12s, min duration: 5.95ms, elapsed: 30.40s
    Total updates: 1000, concurrency: 40, success: 365, failed: 635, avg duration: 826.45ms, max duration: 1.13s, min duration: 7.83ms, elapsed: 21.12s
    Total updates: 1000, concurrency: 80, success: 64, failed: 936, avg duration: 1.00s, max duration: 1.12s, min duration: 6.87ms, elapsed: 12.93s
	Total updates: 1000, concurrency: 100, success: 29, failed: 971, avg duration: 1.02s, max duration: 1.11s, min duration: 36.97ms, elapsed: 10.51s
*/

package host

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"go.uber.org/multierr"

	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql/public"
	persistencetests "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/testflags"
)

const (
	updateRequestsChannelSize = 1000
	concurrency               = 10
	totalUpdates              = 1000
	replicas                  = 1
	maxConns                  = 2
)

type updateStats struct {
	updateNum int
	duration  time.Duration
	ts        time.Time
	err       error
}

type aggStats struct {
	successCnt    int
	failCnt       int
	totalDuration time.Duration
	maxDuration   time.Duration
	minDuration   time.Duration
	lastUpdate    time.Time
	errs          error
}

func TestCassandraLWT(t *testing.T) {
	flag.Parse()
	testflags.RequireCassandra(t)
	t.Logf("Running Cassandra LWT tests, concurrency: %d", concurrency)

	testBase := public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{
		Replicas: replicas,
		MaxConns: maxConns,
	})
	testBase.Setup()
	defer testBase.TearDownWorkflowStore()

	// Create workflows
	wfID := fmt.Sprintf("workflow-%v", 0)
	wfInfo, err := createWorkflow(testBase, wfID)
	if err != nil {
		t.Fatalf("createWorkflow failed: %v", err)
	}
	updateReq := infoToUpdateReq(testBase, wfInfo)

	t.Logf("Created %d workflows", concurrency)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updateRequestsCh := make(chan int, updateRequestsChannelSize)

	// Start a goroutine to consume the update stats
	var aggStats aggStats
	statsCh := make(chan updateStats, totalUpdates)
	var statCollectorWG sync.WaitGroup
	statCollectorWG.Add(1)
	go func() {
		defer statCollectorWG.Done()
		for i := 0; i < totalUpdates; i++ {
			stats := <-statsCh
			if stats.err != nil {
				aggStats.errs = multierr.Append(aggStats.errs, fmt.Errorf("updateNum: %v, duration: %v, err: %v", stats.updateNum, stats.duration, stats.err))
				aggStats.failCnt++
			} else {
				aggStats.successCnt++
			}

			aggStats.totalDuration += stats.duration
			if aggStats.maxDuration < stats.duration {
				aggStats.maxDuration = stats.duration
			}
			if aggStats.minDuration == 0 || aggStats.minDuration > stats.duration {
				aggStats.minDuration = stats.duration
			}

			if aggStats.lastUpdate.Before(stats.ts) {
				aggStats.lastUpdate = stats.ts
			}
		}
	}()

	// Start concurrent handlers to update the workflows. Each one will update its own workflow
	var updateHandlersWG sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		updateHandlersWG.Add(1)
		go updateHandler(ctx, testBase, updateRequestsCh, statsCh, &updateHandlersWG, i, updateReq)
	}

	// wait a bit before starting the updates
	time.Sleep(1 * time.Second)

	now := time.Now()
	for i := 0; i < totalUpdates; i++ {
		updateRequestsCh <- i
	}

	// wait for the updates to complete
	statCollectorWG.Wait()

	t.Logf("Total updates: %v, concurrency: %d, success: %v, failed: %v, avg duration: %v, max duration: %v, min duration: %v, elapsed: %v",
		totalUpdates,
		concurrency,
		aggStats.successCnt,
		aggStats.failCnt,
		aggStats.totalDuration/time.Duration(totalUpdates),
		aggStats.maxDuration,
		aggStats.minDuration,
		aggStats.lastUpdate.Sub(now),
	)
	if aggStats.errs != nil {
		msg := aggStats.errs.Error()
		len := len(msg)
		if len > 1000 {
			len = 1000
		}
		t.Errorf("Errors: %s", msg[:len-1])
	}

	// close the channel to signal the handlers to exit
	close(updateRequestsCh)
	// wait for the handlers to exit
	updateHandlersWG.Wait()
}

func updateHandler(
	ctx context.Context,
	s *persistencetests.TestBase,
	incomingUpdatesCh chan int,
	statsCh chan updateStats,
	wg *sync.WaitGroup,
	handlerNo int,
	updateReq *persistence.UpdateWorkflowExecutionRequest,
) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			s.T().Logf("updateHandler %v exiting because context done", handlerNo)
			return
		case updateNum, ok := <-incomingUpdatesCh:
			if !ok {
				s.T().Logf("updateHandler %v exiting because channel closed", handlerNo)
				return
			}
			start := time.Now()
			err := updateWorkflow(s, updateReq)
			end := time.Now()

			statsCh <- updateStats{
				updateNum: updateNum,
				duration:  end.Sub(start),
				ts:        end,
				err:       err,
			}
		}
	}
}

// createWorkflow creates a new workflow and returns the workflow info to be used for updates
func createWorkflow(s *persistencetests.TestBase, wfID string) (*persistence.WorkflowMutableState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	domainID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: wfID,
		RunID:      uuid.New(),
	}

	tasklist := "tasklist1"
	workflowType := "workflowtype1"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	csum := checksum.Checksum{
		Flavor:  checksum.FlavorIEEECRC32OverThriftBinary,
		Version: 22,
		Value:   uuid.NewRandom(),
	}
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := persistence.NewVersionHistories(versionHistory)

	req := &persistence.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: persistence.WorkflowSnapshot{
			ExecutionInfo: &persistence.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				WorkflowID:                  workflowExecution.GetWorkflowID(),
				RunID:                       workflowExecution.GetRunID(),
				FirstExecutionRunID:         workflowExecution.GetRunID(),
				TaskList:                    tasklist,
				WorkflowTypeName:            workflowType,
				WorkflowTimeout:             workflowTimeout,
				DecisionStartToCloseTimeout: decisionTimeout,
				LastFirstEventID:            constants.FirstEventID,
				NextEventID:                 nextEventID,
				LastProcessedEvent:          lastProcessedEventID,
				State:                       persistence.WorkflowStateCreated,
				CloseStatus:                 persistence.WorkflowCloseStatusNone,
			},
			ExecutionStats:   &persistence.ExecutionStats{},
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    persistence.CreateWorkflowModeBrandNew,
	}

	_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ExecutionManager.CreateWorkflowExecution failed: %v", err)
	}

	info, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	if err != nil {
		return nil, fmt.Errorf("GetWorkflowExecutionInfo failed: %v", err)
	}
	if info.ExecutionInfo.State != persistence.WorkflowStateCreated {
		return nil, fmt.Errorf("Unexpected state: %v", info.ExecutionInfo.State)
	}
	if info.ExecutionInfo.CloseStatus != persistence.WorkflowCloseStatusNone {
		return nil, fmt.Errorf("Unexpected close status: %v", info.ExecutionInfo.CloseStatus)
	}

	return info, nil
}

func infoToUpdateReq(s *persistencetests.TestBase, info *persistence.WorkflowMutableState) *persistence.UpdateWorkflowExecutionRequest {
	updatedInfo := info.ExecutionInfo
	updatedStats := info.ExecutionStats
	updatedInfo.State = persistence.WorkflowStateRunning
	nextEventID := int64(3)
	csum := checksum.Checksum{
		Flavor:  checksum.FlavorIEEECRC32OverThriftBinary,
		Version: 22,
		Value:   uuid.NewRandom(),
	}
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := persistence.NewVersionHistories(versionHistory)
	return &persistence.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: persistence.WorkflowMutation{
			ExecutionInfo:    updatedInfo,
			ExecutionStats:   updatedStats,
			Condition:        nextEventID,
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
	}
}

func updateWorkflow(s *persistencetests.TestBase, req *persistence.UpdateWorkflowExecutionRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.ExecutionManager.UpdateWorkflowExecution(ctx, req)
	if err == nil {
		return nil
	}

	return fmt.Errorf("ExecutionManager.UpdateWorkflowExecution failed: %v", err)
}
