// Copyright (c) 2017 Uber Technologies, Inc.
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

package persistencetests

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/client"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
)

type (
	// DBVisibilityPersistenceSuite tests visibility persistence
	// It only tests against DB based visibility, AdvancedVisibility test is in ESVisibilitySuite
	DBVisibilityPersistenceSuite struct {
		*TestBase
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
		VisibilityMgr p.VisibilityManager
	}
)

// SetupSuite implementation
func (s *DBVisibilityPersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
	// setup visibility manager
	if s.VisibilityTestCluster != s.DefaultTestCluster {
		s.VisibilityTestCluster.SetupTestDatabase()
	}
	clusterName := s.ClusterMetadata.GetCurrentClusterName()
	vCfg := s.VisibilityTestCluster.Config()
	visibilityFactory := client.NewFactory(&vCfg, nil, clusterName, nil, s.Logger, &s.DynamicConfiguration, "test-host", nil)
	// SQL currently doesn't have support for visibility manager
	var err error
	s.VisibilityMgr, err = visibilityFactory.NewVisibilityManager(
		&client.Params{
			PersistenceConfig: config.Persistence{
				VisibilityStore: "something not empty",
			},
		},
		&service.Config{
			ReadVisibilityStoreName:                     dynamicproperties.GetStringPropertyFnFilteredByDomain("db"),
			WriteVisibilityStoreName:                    dynamicproperties.GetStringPropertyFn("db"),
			EnableReadDBVisibilityFromClosedExecutionV2: dynamicproperties.GetBoolPropertyFn(false),
			EnableDBVisibilitySampling:                  dynamicproperties.GetBoolPropertyFn(false),
		},
	)
	if err != nil {
		s.fatalOnError("NewVisibilityManager", err)
	}
}

// SetupTest implementation
func (s *DBVisibilityPersistenceSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
}

// TearDownSuite implementation
func (s *DBVisibilityPersistenceSuite) TearDownSuite() {
	// TODO VisibilityMgr/Store is created with a separated code path, this is incorrect and may cause leaking connection
	// And Postgres requires all connection to be closed before dropping a database
	// https://github.com/uber/cadence/issues/2854
	// Remove the below line after the issue is fix
	s.VisibilityMgr.Close()

	s.TearDownWorkflowStore()
}

// TestBasicVisibility test
func (s *DBVisibilityPersistenceSuite) TestBasicVisibility() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	testDomainUUID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "visibility-workflow-test",
		RunID:      "fb15e4b5-356f-466d-8c6d-a29223e5c536",
	}

	startTime := time.Now().Add(time.Second * -5).UnixNano()
	startReq := &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		UpdateTimestamp:  0,
		ShardID:          0,
	}
	err0 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, startReq)
	s.Nil(err0)

	resp, err1 := s.VisibilityMgr.ListOpenWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:   testDomainUUID,
		PageSize:     1,
		EarliestTime: startTime,
		LatestTime:   startTime,
	})
	s.Nil(err1)
	s.Equal(1, len(resp.Executions))
	s.assertOpenExecutionEquals(startReq, resp.Executions[0])

	closeReq := &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		CloseTimestamp:   time.Now().UnixNano(),
		UpdateTimestamp:  time.Now().UnixNano(),
		HistoryLength:    5,
		ShardID:          1234,
	}
	err2 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, closeReq)
	s.Nil(err2)

	resp, err3 := s.VisibilityMgr.ListOpenWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:   testDomainUUID,
		PageSize:     1,
		EarliestTime: startTime,
		LatestTime:   startTime,
	})
	s.Nil(err3)
	s.Equal(0, len(resp.Executions))

	resp, err4 := s.VisibilityMgr.ListClosedWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:   testDomainUUID,
		PageSize:     1,
		EarliestTime: startTime,
		LatestTime:   startTime,
	})
	s.Nil(err4)
	s.Equal(1, len(resp.Executions))
	s.assertClosedExecutionEquals(closeReq, resp.Executions[0])

	uninitializedReq := &p.RecordWorkflowExecutionUninitializedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-workflow",
		UpdateTimestamp:  time.Now().UnixNano(),
		ShardID:          1234,
	}
	err5 := s.VisibilityMgr.RecordWorkflowExecutionUninitialized(ctx, uninitializedReq)
	s.Nil(err5)
}

// TestCronVisibility test
func (s *DBVisibilityPersistenceSuite) TestCronVisibility() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	testDomainUUID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "visibility-cron-workflow-test",
		RunID:      "fb15e4b5-356f-466d-8c6d-a29223e5c537",
	}

	startTime := time.Now().Add(time.Second * -5).UnixNano()
	startReq := &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-cron-workflow",
		StartTimestamp:   startTime,
		IsCron:           true,
		ShardID:          1234,
	}
	err0 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, startReq)
	s.Nil(err0)

	resp, err1 := s.VisibilityMgr.ListOpenWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:   testDomainUUID,
		PageSize:     1,
		EarliestTime: startTime,
		LatestTime:   startTime,
	})
	s.Nil(err1)
	s.Equal(1, len(resp.Executions))
	s.True(resp.Executions[0].IsCron)

	closeReq := &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		CloseTimestamp:   time.Now().UnixNano(),
		HistoryLength:    5,
		IsCron:           true,
		ShardID:          1234,
	}
	err2 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, closeReq)
	s.Nil(err2)

	resp, err4 := s.VisibilityMgr.ListClosedWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:   testDomainUUID,
		PageSize:     1,
		EarliestTime: startTime,
		LatestTime:   startTime,
	})
	s.Nil(err4)
	s.Equal(1, len(resp.Executions))
	s.True(resp.Executions[0].IsCron)
}

// TestBasicVisibilityTimeSkew test
func (s *DBVisibilityPersistenceSuite) TestBasicVisibilityTimeSkew() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	testDomainUUID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "visibility-workflow-test-time-skew",
		RunID:      "fb15e4b5-356f-466d-8c6d-a29223e5c536",
	}

	startTime := time.Now().Add(time.Second * -5).UnixNano()
	err0 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		ShardID:          1234,
	})
	s.Nil(err0)

	resp, err1 := s.VisibilityMgr.ListOpenWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:   testDomainUUID,
		PageSize:     1,
		EarliestTime: startTime,
		LatestTime:   startTime,
	})
	s.Nil(err1)
	s.Equal(1, len(resp.Executions))
	s.Equal(workflowExecution.WorkflowID, resp.Executions[0].Execution.WorkflowID)

	err2 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		CloseTimestamp:   startTime - (10 * time.Second).Nanoseconds(),
		ShardID:          1234,
	})
	s.Nil(err2)

	resp, err3 := s.VisibilityMgr.ListOpenWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:   testDomainUUID,
		PageSize:     1,
		EarliestTime: startTime,
		LatestTime:   startTime,
	})
	s.Nil(err3)
	s.Equal(0, len(resp.Executions))

	resp, err4 := s.VisibilityMgr.ListClosedWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:   testDomainUUID,
		PageSize:     1,
		EarliestTime: startTime,
		LatestTime:   startTime,
	})
	s.Nil(err4)
	s.Equal(1, len(resp.Executions))
}

// TestVisibilityPagination test
func (s *DBVisibilityPersistenceSuite) TestVisibilityPagination() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	testDomainUUID := uuid.New()

	// Create 2 executions
	startTime1 := time.Now()
	workflowExecution1 := types.WorkflowExecution{
		WorkflowID: "visibility-pagination-test1",
		RunID:      "fb15e4b5-356f-466d-8c6d-a29223e5c536",
	}

	startReq1 := &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution1,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime1.UnixNano(),
		ShardID:          1234,
	}

	err0 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, startReq1)
	s.Nil(err0)

	startTime2 := startTime1.Add(time.Second)
	workflowExecution2 := types.WorkflowExecution{
		WorkflowID: "visibility-pagination-test2",
		RunID:      "843f6fc7-102a-4c63-a2d4-7c653b01bf52",
	}

	startReq2 := &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution2,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime2.UnixNano(),
		ShardID:          1234,
	}
	err1 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, startReq2)
	s.Nil(err1)

	// Get the first one
	resp, err2 := s.VisibilityMgr.ListOpenWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:   testDomainUUID,
		PageSize:     1,
		EarliestTime: startTime1.UnixNano(),
		LatestTime:   startTime2.UnixNano(),
	})
	s.Nil(err2)
	s.Equal(1, len(resp.Executions))
	s.assertOpenExecutionEquals(startReq2, resp.Executions[0])

	// Use token to get the second one
	resp, err3 := s.VisibilityMgr.ListOpenWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:    testDomainUUID,
		PageSize:      1,
		EarliestTime:  startTime1.UnixNano(),
		LatestTime:    startTime2.UnixNano(),
		NextPageToken: resp.NextPageToken,
	})
	s.Nil(err3)
	s.Equal(1, len(resp.Executions))
	s.assertOpenExecutionEquals(startReq1, resp.Executions[0])

	// It is possible to not return non empty token which is going to return empty result
	if len(resp.NextPageToken) != 0 {
		// Now should get empty result by using token
		resp, err4 := s.VisibilityMgr.ListOpenWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
			DomainUUID:    testDomainUUID,
			PageSize:      1,
			EarliestTime:  startTime1.UnixNano(),
			LatestTime:    startTime2.UnixNano(),
			NextPageToken: resp.NextPageToken,
		})
		s.Nil(err4)
		s.Equal(0, len(resp.Executions))
	}
}

// TestFilteringByType test
func (s *DBVisibilityPersistenceSuite) TestFilteringByType() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	testDomainUUID := uuid.New()
	startTime := time.Now().UnixNano()

	// Create 2 executions
	workflowExecution1 := types.WorkflowExecution{
		WorkflowID: "visibility-filtering-test1",
		RunID:      "fb15e4b5-356f-466d-8c6d-a29223e5c536",
	}
	err0 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution1,
		WorkflowTypeName: "visibility-workflow-1",
		StartTimestamp:   startTime,
		ShardID:          1234,
	})
	s.Nil(err0)

	workflowExecution2 := types.WorkflowExecution{
		WorkflowID: "visibility-filtering-test2",
		RunID:      "843f6fc7-102a-4c63-a2d4-7c653b01bf52",
	}
	err1 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution2,
		WorkflowTypeName: "visibility-workflow-2",
		StartTimestamp:   startTime,
		ShardID:          1234,
	})
	s.Nil(err1)

	// List open with filtering
	resp, err2 := s.VisibilityMgr.ListOpenWorkflowExecutionsByType(ctx, &p.ListWorkflowExecutionsByTypeRequest{
		ListWorkflowExecutionsRequest: p.ListWorkflowExecutionsRequest{
			DomainUUID:   testDomainUUID,
			PageSize:     2,
			EarliestTime: startTime,
			LatestTime:   startTime,
		},
		WorkflowTypeName: "visibility-workflow-1",
	})
	s.Nil(err2)
	s.Equal(1, len(resp.Executions))
	s.Equal(workflowExecution1.WorkflowID, resp.Executions[0].Execution.WorkflowID)

	stopTime := time.Now().UnixNano()

	// Close both executions
	err3 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution1,
		WorkflowTypeName: "visibility-workflow-1",
		StartTimestamp:   startTime,
		CloseTimestamp:   stopTime,
		ShardID:          1234,
	})
	s.Nil(err3)

	closeReq := &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution2,
		WorkflowTypeName: "visibility-workflow-2",
		StartTimestamp:   startTime,
		CloseTimestamp:   stopTime,
		HistoryLength:    3,
		ShardID:          1234,
	}
	err4 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, closeReq)
	s.Nil(err4)

	// List closed with filtering
	resp, err5 := s.VisibilityMgr.ListClosedWorkflowExecutionsByType(ctx, &p.ListWorkflowExecutionsByTypeRequest{
		ListWorkflowExecutionsRequest: p.ListWorkflowExecutionsRequest{
			DomainUUID:   testDomainUUID,
			PageSize:     2,
			EarliestTime: startTime,
			LatestTime:   startTime,
		},
		WorkflowTypeName: "visibility-workflow-2",
	})
	s.Nil(err5)
	s.Equal(1, len(resp.Executions))
	s.assertClosedExecutionEquals(closeReq, resp.Executions[0])
}

// TestFilteringByWorkflowID test
func (s *DBVisibilityPersistenceSuite) TestFilteringByWorkflowID() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	testDomainUUID := uuid.New()
	startTime := time.Now().UnixNano()

	// Create 2 executions
	workflowExecution1 := types.WorkflowExecution{
		WorkflowID: "visibility-filtering-test1",
		RunID:      "fb15e4b5-356f-466d-8c6d-a29223e5c536",
	}
	err0 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution1,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		ShardID:          1234,
	})
	s.Nil(err0)

	workflowExecution2 := types.WorkflowExecution{
		WorkflowID: "visibility-filtering-test2",
		RunID:      "843f6fc7-102a-4c63-a2d4-7c653b01bf52",
	}
	err1 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution2,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		ShardID:          1234,
	})
	s.Nil(err1)

	// List open with filtering
	resp, err2 := s.VisibilityMgr.ListOpenWorkflowExecutionsByWorkflowID(ctx, &p.ListWorkflowExecutionsByWorkflowIDRequest{
		ListWorkflowExecutionsRequest: p.ListWorkflowExecutionsRequest{
			DomainUUID:   testDomainUUID,
			PageSize:     2,
			EarliestTime: startTime,
			LatestTime:   startTime,
		},
		WorkflowID: "visibility-filtering-test1",
	})
	s.Nil(err2)
	s.Equal(1, len(resp.Executions))
	s.Equal(workflowExecution1.WorkflowID, resp.Executions[0].Execution.WorkflowID)

	stopTime := time.Now().UnixNano()

	// Close both executions
	err3 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution1,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		CloseTimestamp:   stopTime,
	})
	s.Nil(err3)

	closeReq := &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution2,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		CloseTimestamp:   stopTime,
		HistoryLength:    3,
		ShardID:          1234,
	}
	err4 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, closeReq)
	s.Nil(err4)

	// List closed with filtering
	resp, err5 := s.VisibilityMgr.ListClosedWorkflowExecutionsByWorkflowID(ctx, &p.ListWorkflowExecutionsByWorkflowIDRequest{
		ListWorkflowExecutionsRequest: p.ListWorkflowExecutionsRequest{
			DomainUUID:   testDomainUUID,
			PageSize:     2,
			EarliestTime: startTime,
			LatestTime:   startTime,
		},
		WorkflowID: "visibility-filtering-test2",
	})
	s.Nil(err5)
	s.Equal(1, len(resp.Executions))
	s.assertClosedExecutionEquals(closeReq, resp.Executions[0])
}

// TestFilteringByCloseStatus test
func (s *DBVisibilityPersistenceSuite) TestFilteringByCloseStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	testDomainUUID := uuid.New()
	startTime := time.Now().UnixNano()

	// Create 2 executions
	workflowExecution1 := types.WorkflowExecution{
		WorkflowID: "visibility-filtering-test1",
		RunID:      "fb15e4b5-356f-466d-8c6d-a29223e5c536",
	}
	err0 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution1,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		ShardID:          1234,
	})
	s.Nil(err0)

	workflowExecution2 := types.WorkflowExecution{
		WorkflowID: "visibility-filtering-test2",
		RunID:      "843f6fc7-102a-4c63-a2d4-7c653b01bf52",
	}
	err1 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution2,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		ShardID:          1234,
	})
	s.Nil(err1)

	stopTime := time.Now().UnixNano()

	// Close both executions with different status
	err2 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution1,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		CloseTimestamp:   stopTime,
		Status:           types.WorkflowExecutionCloseStatusCompleted,
		ShardID:          1234,
	})
	s.Nil(err2)

	closeReq := &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution2,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		Status:           types.WorkflowExecutionCloseStatusFailed,
		CloseTimestamp:   stopTime,
		HistoryLength:    3,
		ShardID:          1234,
	}
	err3 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, closeReq)
	s.Nil(err3)

	// List closed with filtering
	resp, err4 := s.VisibilityMgr.ListClosedWorkflowExecutionsByStatus(ctx, &p.ListClosedWorkflowExecutionsByStatusRequest{
		ListWorkflowExecutionsRequest: p.ListWorkflowExecutionsRequest{
			DomainUUID:   testDomainUUID,
			PageSize:     2,
			EarliestTime: startTime,
			LatestTime:   startTime,
		},
		Status: types.WorkflowExecutionCloseStatusFailed,
	})
	s.Nil(err4)
	s.Equal(1, len(resp.Executions))
	s.assertClosedExecutionEquals(closeReq, resp.Executions[0])
}

// TestGetClosedExecution test
func (s *DBVisibilityPersistenceSuite) TestGetClosedExecution() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	testDomainUUID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "visibility-workflow-test",
		RunID:      "a3dbc7bf-deb1-4946-b57c-cf0615ea553f",
	}

	startTime := time.Now().Add(time.Second * -5).UnixNano()
	err0 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, &p.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		ShardID:          1234,
	})
	s.Nil(err0)

	closedResp, err1 := s.VisibilityMgr.GetClosedWorkflowExecution(ctx, &p.GetClosedWorkflowExecutionRequest{
		DomainUUID: testDomainUUID,
		Execution:  workflowExecution,
	})
	s.Error(err1)
	_, ok := err1.(*types.EntityNotExistsError)
	s.True(ok, "EntityNotExistsError")
	s.Nil(closedResp)

	stopTime := time.Now().UnixNano()

	closeReq := &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		Status:           types.WorkflowExecutionCloseStatusFailed,
		CloseTimestamp:   stopTime,
		HistoryLength:    3,
		ShardID:          1234,
	}
	err2 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, closeReq)
	s.Nil(err2)

	resp, err3 := s.VisibilityMgr.GetClosedWorkflowExecution(ctx, &p.GetClosedWorkflowExecutionRequest{
		DomainUUID: testDomainUUID,
		Execution:  workflowExecution,
	})
	s.Nil(err3)
	s.assertClosedExecutionEquals(closeReq, resp.Execution)
}

// TestClosedWithoutStarted test
func (s *DBVisibilityPersistenceSuite) TestClosedWithoutStarted() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	testDomainUUID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "visibility-workflow-test",
		RunID:      "1bdb0122-e8c9-4b35-b6f8-d692ab259b09",
	}

	closedResp, err0 := s.VisibilityMgr.GetClosedWorkflowExecution(ctx, &p.GetClosedWorkflowExecutionRequest{
		DomainUUID: testDomainUUID,
		Execution:  workflowExecution,
	})
	s.Error(err0)
	_, ok := err0.(*types.EntityNotExistsError)
	s.True(ok, "EntityNotExistsError")
	s.Nil(closedResp)

	closeReq := &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   time.Now().Add(time.Second * -5).UnixNano(),
		Status:           types.WorkflowExecutionCloseStatusFailed,
		CloseTimestamp:   time.Now().UnixNano(),
		HistoryLength:    3,
		ShardID:          1234,
	}
	err1 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, closeReq)
	s.Nil(err1)

	resp, err2 := s.VisibilityMgr.GetClosedWorkflowExecution(ctx, &p.GetClosedWorkflowExecutionRequest{
		DomainUUID: testDomainUUID,
		Execution:  workflowExecution,
	})
	s.Nil(err2)
	s.assertClosedExecutionEquals(closeReq, resp.Execution)
}

// TestMultipleUpserts test
func (s *DBVisibilityPersistenceSuite) TestMultipleUpserts() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	testDomainUUID := uuid.New()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "visibility-workflow-test",
		RunID:      "a3dbc7bf-deb1-4946-b57c-cf0615ea553f",
	}

	startTime := time.Now().Add(time.Second * -5).UnixNano()
	closeReq := &p.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       testDomainUUID,
		Execution:        workflowExecution,
		WorkflowTypeName: "visibility-workflow",
		StartTimestamp:   startTime,
		Status:           types.WorkflowExecutionCloseStatusFailed,
		CloseTimestamp:   time.Now().UnixNano(),
		HistoryLength:    3,
		ShardID:          1234,
	}

	count := 3
	for i := 0; i < count; i++ {
		err0 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, &p.RecordWorkflowExecutionStartedRequest{
			DomainUUID:       testDomainUUID,
			Execution:        workflowExecution,
			WorkflowTypeName: "visibility-workflow",
			StartTimestamp:   startTime,
			ShardID:          1234,
		})
		s.Nil(err0)
		if i < count-1 {
			err1 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, closeReq)
			s.Nil(err1)
		}
	}

	resp, err3 := s.VisibilityMgr.GetClosedWorkflowExecution(ctx, &p.GetClosedWorkflowExecutionRequest{
		DomainUUID: testDomainUUID,
		Execution:  workflowExecution,
	})
	s.Nil(err3)
	s.assertClosedExecutionEquals(closeReq, resp.Execution)

}

// TestDelete test
func (s *DBVisibilityPersistenceSuite) TestDelete() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	if s.VisibilityMgr.GetName() == "cassandra" {
		// this test is not applicable for cassandra
		return
	}
	nRows := 5
	testDomainUUID := uuid.New()
	startTime := time.Now().Add(time.Second * -5).UnixNano()
	for i := 0; i < nRows; i++ {
		workflowExecution := types.WorkflowExecution{
			WorkflowID: uuid.New(),
			RunID:      uuid.New(),
		}
		err0 := s.VisibilityMgr.RecordWorkflowExecutionStarted(ctx, &p.RecordWorkflowExecutionStartedRequest{
			DomainUUID:       testDomainUUID,
			Execution:        workflowExecution,
			WorkflowTypeName: "visibility-workflow",
			StartTimestamp:   startTime,
		})
		s.Nil(err0)
		closeReq := &p.RecordWorkflowExecutionClosedRequest{
			DomainUUID:       testDomainUUID,
			Execution:        workflowExecution,
			WorkflowTypeName: "visibility-workflow",
			StartTimestamp:   startTime,
			Status:           types.WorkflowExecutionCloseStatusFailed,
			CloseTimestamp:   time.Now().UnixNano(),
			HistoryLength:    3,
			ShardID:          1234,
		}
		err1 := s.VisibilityMgr.RecordWorkflowExecutionClosed(ctx, closeReq)
		s.Nil(err1)
	}

	resp, err3 := s.VisibilityMgr.ListClosedWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
		DomainUUID:   testDomainUUID,
		EarliestTime: startTime,
		LatestTime:   time.Now().UnixNano(),
		PageSize:     10,
	})
	s.Nil(err3)
	s.Equal(nRows, len(resp.Executions))

	remaining := nRows
	for _, row := range resp.Executions {
		err4 := s.VisibilityMgr.DeleteWorkflowExecution(ctx, &p.VisibilityDeleteWorkflowExecutionRequest{
			DomainID: testDomainUUID,
			RunID:    row.GetExecution().GetRunID(),
		})
		s.Nil(err4)
		remaining--
		resp, err5 := s.VisibilityMgr.ListClosedWorkflowExecutions(ctx, &p.ListWorkflowExecutionsRequest{
			DomainUUID:   testDomainUUID,
			EarliestTime: startTime,
			LatestTime:   time.Now().UnixNano(),
			PageSize:     10,
		})
		s.Nil(err5)
		s.Equal(remaining, len(resp.Executions))
	}
}

// TestUpsertWorkflowExecution test
func (s *DBVisibilityPersistenceSuite) TestUpsertWorkflowExecution() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	tests := []struct {
		request  *p.UpsertWorkflowExecutionRequest
		expected error
	}{
		{
			request: &p.UpsertWorkflowExecutionRequest{
				DomainUUID:         "",
				Domain:             "",
				Execution:          types.WorkflowExecution{},
				WorkflowTypeName:   "",
				StartTimestamp:     0,
				ExecutionTimestamp: 0,
				WorkflowTimeout:    0,
				TaskID:             0,
				Memo:               nil,
				SearchAttributes: map[string][]byte{
					definition.CadenceChangeVersion: []byte("dummy"),
				},
				ShardID: 1234,
			},
			expected: nil,
		},
		{
			request: &p.UpsertWorkflowExecutionRequest{
				DomainUUID:         "",
				Domain:             "",
				Execution:          types.WorkflowExecution{},
				WorkflowTypeName:   "",
				StartTimestamp:     0,
				ExecutionTimestamp: 0,
				WorkflowTimeout:    0,
				TaskID:             0,
				Memo:               nil,
				SearchAttributes:   nil,
				ShardID:            1234,
			},
			expected: &types.InternalServiceError{
				Message: "Error writing to visibility: Operation is not supported",
			},
		},
	}

	for _, test := range tests {
		err := s.VisibilityMgr.UpsertWorkflowExecution(ctx, test.request)
		if test.expected == nil {
			s.Equal(test.expected, err)
		} else {
			s.Equal(test.expected.Error(), err.Error())
		}
	}
}

func (s *DBVisibilityPersistenceSuite) assertClosedExecutionEquals(
	req *p.RecordWorkflowExecutionClosedRequest, resp *types.WorkflowExecutionInfo) {
	s.Equal(req.Execution.RunID, resp.Execution.RunID)
	s.Equal(req.Execution.WorkflowID, resp.Execution.WorkflowID)
	s.Equal(req.WorkflowTypeName, resp.GetType().GetName())
	s.Equal(s.nanosToMillis(req.StartTimestamp), s.nanosToMillis(resp.GetStartTime()))
	s.Equal(s.nanosToMillis(req.CloseTimestamp), s.nanosToMillis(resp.GetCloseTime()))
	s.Equal(req.Status, resp.GetCloseStatus())
	s.Equal(req.HistoryLength, resp.HistoryLength)
}

func (s *DBVisibilityPersistenceSuite) assertOpenExecutionEquals(
	req *p.RecordWorkflowExecutionStartedRequest, resp *types.WorkflowExecutionInfo) {
	s.Equal(req.Execution.GetRunID(), resp.Execution.GetRunID())
	s.Equal(req.Execution.WorkflowID, resp.Execution.WorkflowID)
	s.Equal(req.WorkflowTypeName, resp.GetType().GetName())
	s.Equal(s.nanosToMillis(req.StartTimestamp), s.nanosToMillis(resp.GetStartTime()))
	s.Nil(resp.CloseTime)
	s.Nil(resp.CloseStatus)
	s.Zero(resp.HistoryLength)
}

func (s *DBVisibilityPersistenceSuite) nanosToMillis(nanos int64) int64 {
	return nanos / int64(time.Millisecond)
}
