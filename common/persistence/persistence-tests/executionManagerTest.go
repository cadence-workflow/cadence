// Copyright (c) 2017-2020 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// ExecutionManagerSuite contains matching persistence tests
	ExecutionManagerSuite struct {
		*TestBase
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
	}
)

var (
	testContextTimeout      = 5 * time.Second
	largeTestContextTimeout = 30 * time.Second

	testWorkflowChecksum = checksum.Checksum{
		Version: 22,
		Flavor:  checksum.FlavorIEEECRC32OverThriftBinary,
		Value:   []byte("test-checksum"),
	}
)

// SetupSuite implementation
func (s *ExecutionManagerSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

// TearDownSuite implementation
func (s *ExecutionManagerSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

// SetupTest implementation
func (s *ExecutionManagerSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
	s.ClearTasks()
}

func (s *ExecutionManagerSuite) newRandomChecksum() checksum.Checksum {
	return checksum.Checksum{
		Flavor:  checksum.FlavorIEEECRC32OverThriftBinary,
		Version: 22,
		Value:   uuid.NewRandom(),
	}
}

func (s *ExecutionManagerSuite) assertChecksumsEqual(expected checksum.Checksum, actual checksum.Checksum) {
	s.EqualValues(expected, actual)
}

// TestCreateWorkflowExecutionDeDup test
func (s *ExecutionManagerSuite) TestCreateWorkflowExecutionDeDup() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	domainName := uuid.New()
	workflowID := "create-workflow-test-dedup"
	runID := "3969fae6-6b75-4c2a-b74b-4054edd296a6"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      runID,
	}
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	csum := s.newRandomChecksum()
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	req := &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				WorkflowID:                  workflowID,
				RunID:                       runID,
				FirstExecutionRunID:         runID,
				TaskList:                    tasklist,
				WorkflowTypeName:            workflowType,
				WorkflowTimeout:             workflowTimeout,
				DecisionStartToCloseTimeout: decisionTimeout,
				LastFirstEventID:            constants.FirstEventID,
				NextEventID:                 nextEventID,
				LastProcessedEvent:          lastProcessedEventID,
				State:                       p.WorkflowStateCreated,
				CloseStatus:                 p.WorkflowCloseStatusNone,
			},
			ExecutionStats:   &p.ExecutionStats{},
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID:    s.ShardInfo.RangeID,
		Mode:       p.CreateWorkflowModeBrandNew,
		DomainName: domainName,
	}

	_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	info, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Nil(err)
	s.assertChecksumsEqual(csum, info.Checksum)
	updatedInfo := copyWorkflowExecutionInfo(info.ExecutionInfo)
	updatedStats := copyExecutionStats(info.ExecutionStats)
	updatedInfo.State = p.WorkflowStateCompleted
	updatedInfo.CloseStatus = p.WorkflowCloseStatusCompleted
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:    updatedInfo,
			ExecutionStats:   updatedStats,
			Condition:        nextEventID,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.UpdateWorkflowModeUpdateCurrent,
	})
	s.NoError(err)

	req.Mode = p.CreateWorkflowModeWorkflowIDReuse
	req.PreviousRunID = runID
	req.PreviousLastWriteVersion = constants.EmptyVersion
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Error(err)
	s.IsType(&p.WorkflowExecutionAlreadyStartedError{}, err)
}

func (s *ExecutionManagerSuite) TestCreateWorkflowExecutionWithWorkflowRequestsDedup() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	domainName := uuid.New()
	workflowID := "create-workflow-test-dedup"
	runID := uuid.New()
	requestID := uuid.New()
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	csum := s.newRandomChecksum()
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	req := &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             requestID,
				DomainID:                    domainID,
				WorkflowID:                  workflowID,
				RunID:                       runID,
				FirstExecutionRunID:         runID,
				TaskList:                    tasklist,
				WorkflowTypeName:            workflowType,
				WorkflowTimeout:             workflowTimeout,
				DecisionStartToCloseTimeout: decisionTimeout,
				LastFirstEventID:            constants.FirstEventID,
				NextEventID:                 nextEventID,
				LastProcessedEvent:          lastProcessedEventID,
				State:                       p.WorkflowStateCreated,
				CloseStatus:                 p.WorkflowCloseStatusNone,
			},
			ExecutionStats:   &p.ExecutionStats{},
			Checksum:         csum,
			VersionHistories: versionHistories,
			WorkflowRequests: []*p.WorkflowRequest{
				{
					RequestID:   requestID,
					Version:     1,
					RequestType: p.WorkflowRequestTypeStart,
				},
			},
		},
		RangeID:             s.ShardInfo.RangeID,
		Mode:                p.CreateWorkflowModeBrandNew,
		WorkflowRequestMode: p.CreateWorkflowRequestModeReplicated,
		DomainName:          domainName,
	}
	_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	req.WorkflowRequestMode = p.CreateWorkflowRequestModeNew
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Error(err)
	s.IsType(&p.DuplicateRequestError{}, err)
	s.Equal(persistence.WorkflowRequestTypeStart, err.(*persistence.DuplicateRequestError).RequestType)
	s.Equal(runID, err.(*persistence.DuplicateRequestError).RunID)
	req.WorkflowRequestMode = p.CreateWorkflowRequestModeReplicated
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Error(err)
	s.IsType(&p.WorkflowExecutionAlreadyStartedError{}, err)
}

// TestCreateWorkflowExecutionStateCloseStatus test
func (s *ExecutionManagerSuite) TestCreateWorkflowExecutionStateCloseStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	invalidCloseStatuses := []int{
		p.WorkflowCloseStatusCompleted,
		p.WorkflowCloseStatusFailed,
		p.WorkflowCloseStatusCanceled,
		p.WorkflowCloseStatusTerminated,
		p.WorkflowCloseStatusContinuedAsNew,
		p.WorkflowCloseStatusTimedOut,
	}
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	csum := s.newRandomChecksum()
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	req := &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				TaskList:                    tasklist,
				WorkflowTypeName:            workflowType,
				WorkflowTimeout:             workflowTimeout,
				DecisionStartToCloseTimeout: decisionTimeout,
				LastFirstEventID:            constants.FirstEventID,
				NextEventID:                 nextEventID,
				LastProcessedEvent:          lastProcessedEventID,
			},
			ExecutionStats:   &p.ExecutionStats{},
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.CreateWorkflowModeBrandNew,
	}

	workflowExecutionStatusCreated := types.WorkflowExecution{
		WorkflowID: "create-workflow-test-state-created",
		RunID:      uuid.New(),
	}
	req.NewWorkflowSnapshot.ExecutionInfo.WorkflowID = workflowExecutionStatusCreated.GetWorkflowID()
	req.NewWorkflowSnapshot.ExecutionInfo.RunID = workflowExecutionStatusCreated.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.FirstExecutionRunID = workflowExecutionStatusCreated.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.State = p.WorkflowStateCreated
	for _, invalidCloseStatus := range invalidCloseStatuses {
		req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = invalidCloseStatus
		_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
		s.IsType(&types.InternalServiceError{}, err)
	}
	req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	info, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionStatusCreated)
	s.Nil(err)
	s.Equal(p.WorkflowStateCreated, info.ExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusNone, info.ExecutionInfo.CloseStatus)
	s.assertChecksumsEqual(csum, info.Checksum)

	workflowExecutionStatusRunning := types.WorkflowExecution{
		WorkflowID: "create-workflow-test-state-running",
		RunID:      uuid.New(),
	}
	req.NewWorkflowSnapshot.ExecutionInfo.WorkflowID = workflowExecutionStatusRunning.GetWorkflowID()
	req.NewWorkflowSnapshot.ExecutionInfo.RunID = workflowExecutionStatusRunning.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.FirstExecutionRunID = workflowExecutionStatusRunning.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.State = p.WorkflowStateRunning
	for _, invalidCloseStatus := range invalidCloseStatuses {
		req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = invalidCloseStatus
		_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
		s.IsType(&types.InternalServiceError{}, err)
	}
	req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	info, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionStatusRunning)
	s.Nil(err)
	s.Equal(p.WorkflowStateRunning, info.ExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusNone, info.ExecutionInfo.CloseStatus)
	s.assertChecksumsEqual(csum, info.Checksum)

	workflowExecutionStatusCompleted := types.WorkflowExecution{
		WorkflowID: "create-workflow-test-state-completed",
		RunID:      uuid.New(),
	}
	req.NewWorkflowSnapshot.ExecutionInfo.WorkflowID = workflowExecutionStatusCompleted.GetWorkflowID()
	req.NewWorkflowSnapshot.ExecutionInfo.RunID = workflowExecutionStatusCompleted.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.FirstExecutionRunID = workflowExecutionStatusCompleted.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.State = p.WorkflowStateCompleted
	for _, invalidCloseStatus := range invalidCloseStatuses {
		req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = invalidCloseStatus
		_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
		s.IsType(&types.InternalServiceError{}, err)
	}
	req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.IsType(&types.InternalServiceError{}, err)

	// for zombie workflow creation, we must use existing workflow ID which got created
	// since we do not allow creation of zombie workflow without current record
	workflowExecutionStatusZombie := types.WorkflowExecution{
		WorkflowID: workflowExecutionStatusRunning.WorkflowID,
		RunID:      uuid.New(),
	}
	req.Mode = p.CreateWorkflowModeZombie
	req.NewWorkflowSnapshot.ExecutionInfo.WorkflowID = workflowExecutionStatusZombie.GetWorkflowID()
	req.NewWorkflowSnapshot.ExecutionInfo.RunID = workflowExecutionStatusZombie.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.FirstExecutionRunID = workflowExecutionStatusZombie.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.State = p.WorkflowStateZombie
	for _, invalidCloseStatus := range invalidCloseStatuses {
		req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = invalidCloseStatus
		_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
		s.IsType(&types.InternalServiceError{}, err)
	}
	req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	info, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionStatusZombie)
	s.Nil(err)
	s.Equal(p.WorkflowStateZombie, info.ExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusNone, info.ExecutionInfo.CloseStatus)
	s.assertChecksumsEqual(csum, info.Checksum)
}

// TestCreateWorkflowExecutionWithZombieState test
func (s *ExecutionManagerSuite) TestCreateWorkflowExecutionWithZombieState() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	workflowID := "create-workflow-test-with-zombie-state"
	workflowExecutionZombie1 := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      uuid.New(),
	}
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	csum := s.newRandomChecksum()
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	req := &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				WorkflowID:                  workflowID,
				RunID:                       workflowExecutionZombie1.GetRunID(),
				FirstExecutionRunID:         workflowExecutionZombie1.GetRunID(),
				TaskList:                    tasklist,
				WorkflowTypeName:            workflowType,
				WorkflowTimeout:             workflowTimeout,
				DecisionStartToCloseTimeout: decisionTimeout,
				NextEventID:                 nextEventID,
				LastProcessedEvent:          lastProcessedEventID,
				State:                       p.WorkflowStateZombie,
				CloseStatus:                 p.WorkflowCloseStatusNone,
			},
			ExecutionStats:   &p.ExecutionStats{},
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.CreateWorkflowModeZombie,
	}
	_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err) // allow creating a zombie workflow if no current running workflow
	_, err = s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.IsType(&types.EntityNotExistsError{}, err) // no current workflow

	workflowExecutionRunning := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      uuid.New(),
	}
	req.NewWorkflowSnapshot.ExecutionInfo.RunID = workflowExecutionRunning.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.FirstExecutionRunID = workflowExecutionRunning.GetRunID()
	req.Mode = p.CreateWorkflowModeBrandNew
	req.NewWorkflowSnapshot.ExecutionInfo.State = p.WorkflowStateRunning
	req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	currentRunID, err := s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.Nil(err)
	s.Equal(workflowExecutionRunning.GetRunID(), currentRunID)

	workflowExecutionZombie := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      uuid.New(),
	}
	req.NewWorkflowSnapshot.ExecutionInfo.RunID = workflowExecutionZombie.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.FirstExecutionRunID = workflowExecutionZombie.GetRunID()
	req.Mode = p.CreateWorkflowModeZombie
	req.NewWorkflowSnapshot.ExecutionInfo.State = p.WorkflowStateZombie
	req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	// current run ID is still the prev running run ID
	currentRunID, err = s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecutionRunning.GetWorkflowID())
	s.Nil(err)
	s.Equal(workflowExecutionRunning.GetRunID(), currentRunID)
	info, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionZombie)
	s.Nil(err)
	s.Equal(p.WorkflowStateZombie, info.ExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusNone, info.ExecutionInfo.CloseStatus)
	s.assertChecksumsEqual(csum, info.Checksum)
}

func (s *ExecutionManagerSuite) TestUpdateWorkflowExecutionWithWorkflowRequestsDedup() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	domainName := uuid.New()
	workflowID := "create-workflow-test-dedup"
	runID := uuid.New()
	requestID := uuid.New()
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	csum := s.newRandomChecksum()
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	req := &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             requestID,
				DomainID:                    domainID,
				WorkflowID:                  workflowID,
				RunID:                       runID,
				FirstExecutionRunID:         runID,
				TaskList:                    tasklist,
				WorkflowTypeName:            workflowType,
				WorkflowTimeout:             workflowTimeout,
				DecisionStartToCloseTimeout: decisionTimeout,
				LastFirstEventID:            constants.FirstEventID,
				NextEventID:                 nextEventID,
				LastProcessedEvent:          lastProcessedEventID,
				State:                       p.WorkflowStateCreated,
				CloseStatus:                 p.WorkflowCloseStatusNone,
			},
			ExecutionStats:   &p.ExecutionStats{},
			Checksum:         csum,
			VersionHistories: versionHistories,
			WorkflowRequests: []*p.WorkflowRequest{
				{
					RequestID:   requestID,
					Version:     1,
					RequestType: p.WorkflowRequestTypeStart,
				},
				{
					RequestID:   requestID,
					Version:     1,
					RequestType: p.WorkflowRequestTypeSignal,
				},
			},
		},
		RangeID:             s.ShardInfo.RangeID,
		Mode:                p.CreateWorkflowModeBrandNew,
		WorkflowRequestMode: p.CreateWorkflowRequestModeNew,
		DomainName:          domainName,
	}
	_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	info, err := s.GetWorkflowExecutionInfo(ctx, domainID, types.WorkflowExecution{WorkflowID: workflowID, RunID: runID})
	s.Nil(err)
	csum = s.newRandomChecksum() // update the checksum to new value
	updatedInfo := copyWorkflowExecutionInfo(info.ExecutionInfo)
	updatedStats := copyExecutionStats(info.ExecutionStats)
	updatedInfo.State = p.WorkflowStateRunning
	updatedInfo.CloseStatus = p.WorkflowCloseStatusNone
	updateReq := &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:    updatedInfo,
			ExecutionStats:   updatedStats,
			Condition:        nextEventID,
			Checksum:         csum,
			VersionHistories: versionHistories,
			WorkflowRequests: []*p.WorkflowRequest{
				{
					RequestID:   requestID,
					Version:     1,
					RequestType: p.WorkflowRequestTypeSignal,
				},
			},
		},
		RangeID:             s.ShardInfo.RangeID,
		Mode:                p.UpdateWorkflowModeUpdateCurrent,
		WorkflowRequestMode: p.CreateWorkflowRequestModeNew,
	}
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, updateReq)
	s.Error(err)
	s.IsType(&p.DuplicateRequestError{}, err)
	s.Equal(persistence.WorkflowRequestTypeSignal, err.(*persistence.DuplicateRequestError).RequestType)
	s.Equal(runID, err.(*persistence.DuplicateRequestError).RunID)
	updateReq.WorkflowRequestMode = p.CreateWorkflowRequestModeReplicated
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, updateReq)
	s.Nil(err)
	updateReq.UpdateWorkflowMutation.WorkflowRequests[0].RequestID = uuid.New()
	updateReq.WorkflowRequestMode = p.CreateWorkflowRequestModeNew
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, updateReq)
	s.Nil(err)
}

// TestUpdateWorkflowExecutionStateCloseStatus test
func (s *ExecutionManagerSuite) TestUpdateWorkflowExecutionStateCloseStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "update-workflow-test-state",
		RunID:      uuid.New(),
	}
	closeStatuses := []int{
		p.WorkflowCloseStatusCompleted,
		p.WorkflowCloseStatusFailed,
		p.WorkflowCloseStatusCanceled,
		p.WorkflowCloseStatusTerminated,
		p.WorkflowCloseStatusContinuedAsNew,
		p.WorkflowCloseStatusTimedOut,
	}
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	csum := s.newRandomChecksum()
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	req := &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
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
			},
			ExecutionStats:   &p.ExecutionStats{},
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.CreateWorkflowModeBrandNew,
	}

	req.NewWorkflowSnapshot.ExecutionInfo.State = p.WorkflowStateCreated
	req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	info, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Nil(err)
	s.Equal(p.WorkflowStateCreated, info.ExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusNone, info.ExecutionInfo.CloseStatus)
	s.assertChecksumsEqual(csum, info.Checksum)

	csum = s.newRandomChecksum() // update the checksum to new value
	updatedInfo := copyWorkflowExecutionInfo(info.ExecutionInfo)
	updatedStats := copyExecutionStats(info.ExecutionStats)
	updatedInfo.State = p.WorkflowStateRunning
	updatedInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:    updatedInfo,
			ExecutionStats:   updatedStats,
			Condition:        nextEventID,
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.UpdateWorkflowModeUpdateCurrent,
	})
	s.NoError(err)
	info, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Nil(err)
	s.Equal(p.WorkflowStateRunning, info.ExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusNone, info.ExecutionInfo.CloseStatus)
	s.assertChecksumsEqual(csum, info.Checksum)

	updatedInfo = copyWorkflowExecutionInfo(info.ExecutionInfo)
	updatedStats = copyExecutionStats(info.ExecutionStats)
	updatedInfo.State = p.WorkflowStateRunning
	for _, closeStatus := range closeStatuses {
		updatedInfo.CloseStatus = closeStatus
		_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
			UpdateWorkflowMutation: p.WorkflowMutation{
				ExecutionInfo:  updatedInfo,
				ExecutionStats: updatedStats,
				Condition:      nextEventID,
			},
			RangeID: s.ShardInfo.RangeID,
			Mode:    p.UpdateWorkflowModeUpdateCurrent,
		})
		s.IsType(&types.InternalServiceError{}, err)
	}

	updatedInfo = copyWorkflowExecutionInfo(info.ExecutionInfo)
	updatedStats = copyExecutionStats(info.ExecutionStats)
	updatedInfo.State = p.WorkflowStateCompleted
	updatedInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:  updatedInfo,
			ExecutionStats: updatedStats,
			Condition:      nextEventID,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.UpdateWorkflowModeUpdateCurrent,
	})
	s.IsType(&types.InternalServiceError{}, err)

	for _, closeStatus := range closeStatuses {
		updatedInfo.CloseStatus = closeStatus
		_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
			UpdateWorkflowMutation: p.WorkflowMutation{
				ExecutionInfo:    updatedInfo,
				ExecutionStats:   updatedStats,
				Condition:        nextEventID,
				VersionHistories: versionHistories,
			},
			RangeID: s.ShardInfo.RangeID,
			Mode:    p.UpdateWorkflowModeUpdateCurrent,
		})
		s.Nil(err)
		info, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
		s.Nil(err)
		s.Equal(p.WorkflowStateCompleted, info.ExecutionInfo.State)
		s.Equal(closeStatus, info.ExecutionInfo.CloseStatus)
	}

	// create a new workflow with same domain ID & workflow ID
	// to enable update workflow with zombie status
	workflowExecutionRunning := types.WorkflowExecution{
		WorkflowID: workflowExecution.WorkflowID,
		RunID:      uuid.New(),
	}
	req.NewWorkflowSnapshot.ExecutionInfo.WorkflowID = workflowExecutionRunning.GetWorkflowID()
	req.NewWorkflowSnapshot.ExecutionInfo.RunID = workflowExecutionRunning.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.FirstExecutionRunID = workflowExecutionRunning.GetRunID()
	req.Mode = p.CreateWorkflowModeWorkflowIDReuse
	req.PreviousRunID = workflowExecution.GetRunID()
	req.PreviousLastWriteVersion = constants.EmptyVersion
	req.NewWorkflowSnapshot.ExecutionInfo.State = p.WorkflowStateRunning
	req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)

	updatedInfo = copyWorkflowExecutionInfo(info.ExecutionInfo)
	updatedStats = copyExecutionStats(info.ExecutionStats)
	updatedInfo.State = p.WorkflowStateZombie
	updatedInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:    updatedInfo,
			ExecutionStats:   updatedStats,
			Condition:        nextEventID,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.UpdateWorkflowModeBypassCurrent,
	})
	s.NoError(err)
	info, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Nil(err)
	s.Equal(p.WorkflowStateZombie, info.ExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusNone, info.ExecutionInfo.CloseStatus)

	updatedInfo = copyWorkflowExecutionInfo(info.ExecutionInfo)
	updatedStats = copyExecutionStats(info.ExecutionStats)
	updatedInfo.State = p.WorkflowStateZombie
	for _, closeStatus := range closeStatuses {
		updatedInfo.CloseStatus = closeStatus
		_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
			UpdateWorkflowMutation: p.WorkflowMutation{
				ExecutionInfo:  updatedInfo,
				ExecutionStats: updatedStats,
				Condition:      nextEventID,
			},
			RangeID: s.ShardInfo.RangeID,
			Mode:    p.UpdateWorkflowModeBypassCurrent,
		})
		s.IsType(&types.InternalServiceError{}, err)
	}
}

// TestUpdateWorkflowExecutionWithZombieState test
func (s *ExecutionManagerSuite) TestUpdateWorkflowExecutionWithZombieState() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	workflowID := "create-workflow-test-with-zombie-state"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      uuid.New(),
	}
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	csum := s.newRandomChecksum()
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	// create and update a workflow to make it completed
	req := &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				WorkflowID:                  workflowExecution.GetWorkflowID(),
				RunID:                       workflowExecution.GetRunID(),
				FirstExecutionRunID:         workflowExecution.GetRunID(),
				TaskList:                    tasklist,
				WorkflowTypeName:            workflowType,
				WorkflowTimeout:             workflowTimeout,
				DecisionStartToCloseTimeout: decisionTimeout,
				NextEventID:                 nextEventID,
				LastProcessedEvent:          lastProcessedEventID,
				State:                       p.WorkflowStateRunning,
				CloseStatus:                 p.WorkflowCloseStatusNone,
			},
			ExecutionStats:   &p.ExecutionStats{},
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.CreateWorkflowModeBrandNew,
	}
	_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	currentRunID, err := s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.Nil(err)
	s.Equal(workflowExecution.GetRunID(), currentRunID)

	info, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Nil(err)
	s.assertChecksumsEqual(csum, info.Checksum)

	// try to turn current workflow into zombie state, this should end with an error
	updatedInfo := copyWorkflowExecutionInfo(info.ExecutionInfo)
	updateStats := copyExecutionStats(info.ExecutionStats)
	updatedInfo.State = p.WorkflowStateZombie
	updatedInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:  updatedInfo,
			ExecutionStats: updateStats,
			Condition:      nextEventID,
			Checksum:       csum,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.UpdateWorkflowModeBypassCurrent,
	})
	s.NotNil(err)

	updatedInfo = copyWorkflowExecutionInfo(info.ExecutionInfo)
	updateStats = copyExecutionStats(info.ExecutionStats)
	updatedInfo.State = p.WorkflowStateCompleted
	updatedInfo.CloseStatus = p.WorkflowCloseStatusCompleted
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:    updatedInfo,
			ExecutionStats:   updateStats,
			Condition:        nextEventID,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.UpdateWorkflowModeUpdateCurrent,
	})
	s.NoError(err)

	// create a new workflow with same domain ID & workflow ID
	workflowExecutionRunning := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      uuid.New(),
	}
	csum = checksum.Checksum{} // set checksum to nil
	req.NewWorkflowSnapshot.ExecutionInfo.WorkflowID = workflowExecutionRunning.GetWorkflowID()
	req.NewWorkflowSnapshot.ExecutionInfo.RunID = workflowExecutionRunning.GetRunID()
	req.NewWorkflowSnapshot.ExecutionInfo.FirstExecutionRunID = workflowExecutionRunning.GetRunID()
	req.Mode = p.CreateWorkflowModeWorkflowIDReuse
	req.PreviousRunID = workflowExecution.GetRunID()
	req.PreviousLastWriteVersion = constants.EmptyVersion
	req.NewWorkflowSnapshot.ExecutionInfo.State = p.WorkflowStateRunning
	req.NewWorkflowSnapshot.ExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	req.NewWorkflowSnapshot.Checksum = csum
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	currentRunID, err = s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.Nil(err)
	s.Equal(workflowExecutionRunning.GetRunID(), currentRunID)

	// get the workflow to be turned into a zombie
	info, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Nil(err)
	s.assertChecksumsEqual(csum, info.Checksum)
	updatedInfo = copyWorkflowExecutionInfo(info.ExecutionInfo)
	updateStats = copyExecutionStats(info.ExecutionStats)
	updatedInfo.State = p.WorkflowStateZombie
	updatedInfo.CloseStatus = p.WorkflowCloseStatusNone
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:    updatedInfo,
			ExecutionStats:   updateStats,
			Condition:        nextEventID,
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.UpdateWorkflowModeBypassCurrent,
	})
	s.NoError(err)
	info, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Nil(err)
	s.Equal(p.WorkflowStateZombie, info.ExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusNone, info.ExecutionInfo.CloseStatus)
	s.assertChecksumsEqual(csum, info.Checksum)
	// check current run ID is un touched
	currentRunID, err = s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.Nil(err)
	s.Equal(workflowExecutionRunning.GetRunID(), currentRunID)
}

// TestUpdateWorkflowExecutionTasks test
func (s *ExecutionManagerSuite) TestUpdateWorkflowExecutionTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "b0a8571c-0257-40ea-afcd-3a14eae181c0"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "update-workflow-tasks-test",
		RunID:      "5ba5e531-e46b-48d9-b4b3-859919839553",
	}
	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	taskD, err := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err)
	s.Equal(1, len(taskD), "Expected 1 decision task.")
	err = s.CompleteTransferTask(ctx, taskD[0].GetTaskID())
	s.NoError(err)

	state1, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err)
	info1 := state1.ExecutionInfo
	s.NotNil(info1, "Valid Workflow info expected.")
	updatedInfo1 := copyWorkflowExecutionInfo(info1)
	updatedStats1 := copyExecutionStats(state1.ExecutionStats)

	now := time.Now()
	transferTasks := []p.Task{
		&p.ActivityTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              s.GetNextSequenceNumber(),
				Version:             constants.EmptyVersion,
			},
			TargetDomainID: domainID,
			TaskList:       "some randome tasklist name",
			ScheduleID:     123,
		},
	}
	timerTasks := []p.Task{
		&p.UserTimerTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now.Add(time.Minute),
				TaskID:              s.GetNextSequenceNumber(),
				Version:             constants.EmptyVersion,
			},
			EventID: 124,
		},
	}
	err = s.UpdateWorkflowExecutionTasks(
		ctx,
		updatedInfo1,
		updatedStats1,
		int64(3),
		transferTasks,
		timerTasks,
	)
	s.NoError(err)

	loadedTransferTasks, err := s.GetTransferTasks(ctx, 10, true)
	s.NoError(err)
	s.Len(loadedTransferTasks, len(transferTasks))

	loadedTimerTasks, err := s.GetTimerIndexTasks(ctx, 10, true)
	s.NoError(err)
	s.Len(loadedTimerTasks, len(timerTasks))
}

// TestCreateWorkflowExecutionBrandNew test
func (s *ExecutionManagerSuite) TestCreateWorkflowExecutionBrandNew() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "create-workflow-test-brand-new",
		RunID:      uuid.New(),
	}
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	req := &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				WorkflowID:                  workflowExecution.GetWorkflowID(),
				RunID:                       workflowExecution.GetRunID(),
				FirstExecutionRunID:         workflowExecution.GetRunID(),
				TaskList:                    tasklist,
				WorkflowTypeName:            workflowType,
				WorkflowTimeout:             workflowTimeout,
				DecisionStartToCloseTimeout: decisionTimeout,
				State:                       p.WorkflowStateRunning,
				CloseStatus:                 p.WorkflowCloseStatusNone,
				LastFirstEventID:            constants.FirstEventID,
				NextEventID:                 nextEventID,
				LastProcessedEvent:          lastProcessedEventID,
			},
			ExecutionStats:   &p.ExecutionStats{},
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.CreateWorkflowModeBrandNew,
	}

	_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	_, err = s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.NotNil(err)
	alreadyStartedErr, ok := err.(*p.WorkflowExecutionAlreadyStartedError)
	s.True(ok, "err is not WorkflowExecutionAlreadyStartedError")
	s.Equal(req.NewWorkflowSnapshot.ExecutionInfo.CreateRequestID, alreadyStartedErr.StartRequestID)
	s.Equal(workflowExecution.GetRunID(), alreadyStartedErr.RunID)
	s.Equal(0, alreadyStartedErr.CloseStatus)
	s.Equal(p.WorkflowStateRunning, alreadyStartedErr.State)
}

// TestUpsertWorkflowActivity test
func (s *ExecutionManagerSuite) TestUpsertWorkflowActivity() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	workflowID := "create-workflow-test-with-upsert-activity"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      uuid.New(),
	}
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	csum := s.newRandomChecksum()
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: nextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	// create and update a workflow to make it completed
	req := &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				WorkflowID:                  workflowExecution.GetWorkflowID(),
				RunID:                       workflowExecution.GetRunID(),
				FirstExecutionRunID:         workflowExecution.GetRunID(),
				TaskList:                    tasklist,
				WorkflowTypeName:            workflowType,
				WorkflowTimeout:             workflowTimeout,
				DecisionStartToCloseTimeout: decisionTimeout,
				NextEventID:                 nextEventID,
				LastProcessedEvent:          lastProcessedEventID,
				State:                       p.WorkflowStateRunning,
				CloseStatus:                 p.WorkflowCloseStatusNone,
			},
			ExecutionStats:   &p.ExecutionStats{},
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.CreateWorkflowModeBrandNew,
	}
	_, err := s.ExecutionManager.CreateWorkflowExecution(ctx, req)
	s.Nil(err)
	currentRunID, err := s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.Nil(err)
	s.Equal(workflowExecution.GetRunID(), currentRunID)

	info, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Nil(err)
	s.assertChecksumsEqual(csum, info.Checksum)
	s.Equal(0, len(info.ActivityInfos))

	// insert a new activity
	updatedInfo := copyWorkflowExecutionInfo(info.ExecutionInfo)
	updateStats := copyExecutionStats(info.ExecutionStats)
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:  updatedInfo,
			ExecutionStats: updateStats,
			Condition:      nextEventID,
			Checksum:       csum,
			UpsertActivityInfos: []*p.ActivityInfo{
				{
					Version:    0,
					ScheduleID: 100,
					TaskList:   "test-activity-tasklist-1",
				},
			},
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.UpdateWorkflowModeUpdateCurrent,
	})
	s.Nil(err)

	info2, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Nil(err)
	s.Equal(1, len(info2.ActivityInfos))
	s.Equal("test-activity-tasklist-1", info2.ActivityInfos[100].TaskList)

	// upsert the previous activity
	_, err = s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:  updatedInfo,
			ExecutionStats: updateStats,
			Condition:      nextEventID,
			Checksum:       csum,
			UpsertActivityInfos: []*p.ActivityInfo{
				{
					Version:    0,
					ScheduleID: 100,
					TaskList:   "test-activity-tasklist-2",
				},
			},
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.UpdateWorkflowModeUpdateCurrent,
	})
	s.NoError(err)
	info3, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Nil(err)
	s.Equal(1, len(info3.ActivityInfos))
	s.Equal("test-activity-tasklist-2", info3.ActivityInfos[100].TaskList)
}

// TestCreateWorkflowExecutionRunIDReuseWithoutReplication test
func (s *ExecutionManagerSuite) TestCreateWorkflowExecutionRunIDReuseWithoutReplication() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "create-workflow-test-run-id-reuse-without-replication",
		RunID:      uuid.New(),
	}
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	decisionScheduleID := int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: decisionScheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, tasklist,
		workflowType, workflowTimeout, decisionTimeout, nil, nextEventID,
		lastProcessedEventID, decisionScheduleID, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.assertChecksumsEqual(testWorkflowChecksum, state0.Checksum)
	info0 := state0.ExecutionInfo
	closeInfo := copyWorkflowExecutionInfo(info0)
	closeInfo.State = p.WorkflowStateCompleted
	closeInfo.CloseStatus = p.WorkflowCloseStatusCompleted
	closeInfo.NextEventID = int64(5)
	closeInfo.LastProcessedEvent = int64(2)

	err2 := s.UpdateWorkflowExecution(ctx, closeInfo, state0.ExecutionStats, versionHistories, nil, nil, nextEventID,
		nil, nil, nil, nil, nil)
	s.NoError(err2)

	newExecution := types.WorkflowExecution{
		WorkflowID: workflowExecution.GetWorkflowID(),
		RunID:      uuid.New(),
	}
	// this create should work since we are relying the business logic in history engine
	// to check whether the existing running workflow has finished
	_, err3 := s.ExecutionManager.CreateWorkflowExecution(ctx, &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				WorkflowID:                  newExecution.GetWorkflowID(),
				RunID:                       newExecution.GetRunID(),
				FirstExecutionRunID:         newExecution.GetRunID(),
				TaskList:                    tasklist,
				WorkflowTypeName:            workflowType,
				WorkflowTimeout:             workflowTimeout,
				DecisionStartToCloseTimeout: decisionTimeout,
				State:                       p.WorkflowStateRunning,
				CloseStatus:                 p.WorkflowCloseStatusNone,
				LastFirstEventID:            constants.FirstEventID,
				NextEventID:                 nextEventID,
				LastProcessedEvent:          lastProcessedEventID,
			},
			ExecutionStats:   &p.ExecutionStats{},
			VersionHistories: versionHistories,
		},
		RangeID:                  s.ShardInfo.RangeID,
		Mode:                     p.CreateWorkflowModeWorkflowIDReuse,
		PreviousRunID:            workflowExecution.GetRunID(),
		PreviousLastWriteVersion: constants.EmptyVersion,
	})
	s.NoError(err3)
}

// TestCreateWorkflowExecutionConcurrentCreate test
func (s *ExecutionManagerSuite) TestCreateWorkflowExecutionConcurrentCreate() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "create-workflow-test-concurrent-create",
		RunID:      uuid.New(),
	}
	tasklist := "some random tasklist"
	workflowType := "some random workflow type"
	workflowTimeout := int32(10)
	decisionTimeout := int32(14)
	lastProcessedEventID := int64(0)
	nextEventID := int64(3)
	decisionScheduleID := int64(2)

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, tasklist,
		workflowType, workflowTimeout, decisionTimeout, nil, nextEventID,
		lastProcessedEventID, decisionScheduleID, nil, nil)
	s.Nil(err0, "No error expected.")
	s.NotNil(task0, "Expected non empty task identifier.")

	times := 2
	var wg sync.WaitGroup
	wg.Add(times)
	var numOfErr int32
	var lastError error
	for i := 0; i < times; i++ {
		go func() {
			newExecution := types.WorkflowExecution{
				WorkflowID: workflowExecution.GetWorkflowID(),
				RunID:      uuid.New(),
			}

			state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
			s.NoError(err1)
			info0 := state0.ExecutionInfo
			continueAsNewInfo := copyWorkflowExecutionInfo(info0)
			continueAsNewInfo.State = p.WorkflowStateRunning
			continueAsNewInfo.NextEventID = int64(5)
			continueAsNewInfo.LastProcessedEvent = int64(2)

			err2 := s.ContinueAsNewExecution(ctx, continueAsNewInfo, state0.ExecutionStats, info0.NextEventID, newExecution, int64(3), int64(2), nil)
			if err2 != nil {
				errCount := atomic.AddInt32(&numOfErr, 1)
				if errCount > 1 {
					lastError = err2
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if lastError != nil {
		s.Fail("More than one error: %v", lastError.Error())
	}
	s.Equal(int32(1), atomic.LoadInt32(&numOfErr))
}

// TestPersistenceStartWorkflow test
func (s *ExecutionManagerSuite) TestPersistenceStartWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "2d7994bf-9de8-459d-9c81-e723daedb246"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "start-workflow-test",
		RunID:      "7f9fe8a0-9237-11e6-ae22-56b6b6499611",
	}
	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	task1, err1 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType1", 20, 14, nil, 3, 0, 2, nil, nil)
	s.Error(err1, "Expected workflow creation to fail.")
	s.T().Logf("Unable to start workflow execution: %v\n", err1)
	startedErr, ok := err1.(*p.WorkflowExecutionAlreadyStartedError)
	s.True(ok, fmt.Sprintf("Expected WorkflowExecutionAlreadyStartedError, but actual is %v", err1))
	s.Equal(workflowExecution.GetRunID(), startedErr.RunID, startedErr.Msg)

	s.Equal(p.WorkflowStateRunning, startedErr.State, startedErr.Msg)
	s.Equal(p.WorkflowCloseStatusNone, startedErr.CloseStatus, startedErr.Msg)
	s.Equal(constants.EmptyVersion, startedErr.LastWriteVersion, startedErr.Msg)
	s.Empty(task1, "Expected empty task identifier.")
	decisionScheduleID := int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: decisionScheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	response, err2 := s.ExecutionManager.CreateWorkflowExecution(ctx, &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    domainID,
				WorkflowID:                  workflowExecution.GetWorkflowID(),
				RunID:                       workflowExecution.GetRunID(),
				FirstExecutionRunID:         workflowExecution.GetRunID(),
				TaskList:                    "queue1",
				WorkflowTypeName:            "workflow_type_test",
				WorkflowTimeout:             20,
				DecisionStartToCloseTimeout: 13,
				ExecutionContext:            nil,
				State:                       p.WorkflowStateRunning,
				CloseStatus:                 p.WorkflowCloseStatusNone,
				LastFirstEventID:            constants.FirstEventID,
				NextEventID:                 int64(3),
				LastProcessedEvent:          0,
				DecisionScheduleID:          decisionScheduleID,
				DecisionStartedID:           constants.EmptyEventID,
				DecisionTimeout:             1,
			},
			ExecutionStats: &p.ExecutionStats{},
			TasksByCategory: map[p.HistoryTaskCategory][]p.Task{
				p.HistoryTaskCategoryTransfer: []p.Task{
					&p.DecisionTask{
						WorkflowIdentifier: p.WorkflowIdentifier{
							DomainID:   domainID,
							WorkflowID: workflowExecution.WorkflowID,
							RunID:      workflowExecution.RunID,
						},
						TaskData: p.TaskData{
							TaskID: s.GetNextSequenceNumber(),
						},
						TargetDomainID: domainID,
						TaskList:       "queue1",
						ScheduleID:     int64(2),
					},
				},
			},
			VersionHistories: versionHistories,
		},
		RangeID: s.ShardInfo.RangeID - 1,
	})

	s.Error(err2, "Expected workflow creation to fail.")
	s.Nil(response)
	s.T().Logf("Unable to start workflow execution: %v\n", err2)
	s.IsType(&p.ShardOwnershipLostError{}, err2)
}

// TestGetWorkflow test
func (s *ExecutionManagerSuite) TestGetWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	now := time.Now()
	testResetPoints := types.ResetPoints{
		Points: []*types.ResetPointInfo{
			{
				BinaryChecksum:           "test-binary-checksum",
				RunID:                    "test-runID",
				FirstDecisionCompletedID: 123,
				CreatedTimeNano:          common.Int64Ptr(456),
				Resettable:               true,
				ExpiringTimeNano:         common.Int64Ptr(789),
			},
		},
	}
	testSearchAttrKey := "env"
	testSearchAttrVal, _ := json.Marshal("test")
	testSearchAttr := map[string][]byte{
		testSearchAttrKey: testSearchAttrVal,
	}

	testMemoKey := "memoKey"
	testMemoVal, _ := json.Marshal("memoVal")
	testMemo := map[string][]byte{
		testMemoKey: testMemoVal,
	}

	testPartitionConfig := map[string]string{
		"zone": "dca1",
	}

	csum := s.newRandomChecksum()
	decisionScheduleID := int64(rand.Int31())
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: decisionScheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	createReq := &p.CreateWorkflowExecutionRequest{
		NewWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo: &p.WorkflowExecutionInfo{
				CreateRequestID:             uuid.New(),
				DomainID:                    uuid.New(),
				WorkflowID:                  "get-workflow-test",
				RunID:                       uuid.New(),
				ParentDomainID:              uuid.New(),
				ParentWorkflowID:            "get-workflow-test-parent",
				ParentRunID:                 uuid.New(),
				InitiatedID:                 rand.Int63(),
				TaskList:                    "get-wf-test-tasklist",
				WorkflowTypeName:            "code.uber.internal/test/workflow",
				WorkflowTimeout:             rand.Int31(),
				DecisionStartToCloseTimeout: rand.Int31(),
				ExecutionContext:            []byte("test-execution-context"),
				State:                       p.WorkflowStateRunning,
				CloseStatus:                 p.WorkflowCloseStatusNone,
				LastFirstEventID:            constants.FirstEventID,
				NextEventID:                 rand.Int63(),
				LastProcessedEvent:          int64(rand.Int31()),
				LastUpdatedTimestamp:        now,
				StartTimestamp:              now,
				SignalCount:                 rand.Int31(),
				DecisionVersion:             int64(rand.Int31()),
				DecisionScheduleID:          decisionScheduleID,
				DecisionStartedID:           int64(rand.Int31()),
				DecisionTimeout:             rand.Int31(),
				Attempt:                     rand.Int31(),
				HasRetryPolicy:              true,
				InitialInterval:             rand.Int31(),
				BackoffCoefficient:          7.78,
				MaximumInterval:             rand.Int31(),
				ExpirationTime:              time.Now(),
				MaximumAttempts:             rand.Int31(),
				NonRetriableErrors:          []string{"badRequestError", "accessDeniedError"},
				CronSchedule:                "* * * * *",
				CronOverlapPolicy:           types.CronOverlapPolicySkipped,
				ExpirationSeconds:           rand.Int31(),
				AutoResetPoints:             &testResetPoints,
				SearchAttributes:            testSearchAttr,
				Memo:                        testMemo,
				PartitionConfig:             testPartitionConfig,
				ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{
					ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
					StickyRegion:                   "region1",
				},
			},
			ExecutionStats: &p.ExecutionStats{
				HistorySize: int64(rand.Int31()),
			},
			Checksum:         csum,
			VersionHistories: versionHistories,
		},
		Mode: p.CreateWorkflowModeBrandNew,
	}
	createReq.NewWorkflowSnapshot.ExecutionInfo.FirstExecutionRunID = createReq.NewWorkflowSnapshot.ExecutionInfo.RunID

	createResp, err := s.ExecutionManager.CreateWorkflowExecution(ctx, createReq)
	s.NoError(err)
	s.NotNil(createResp, "Expected non empty task identifier.")

	state, err := s.GetWorkflowExecutionInfo(ctx, createReq.NewWorkflowSnapshot.ExecutionInfo.DomainID,
		types.WorkflowExecution{
			WorkflowID: createReq.NewWorkflowSnapshot.ExecutionInfo.WorkflowID,
			RunID:      createReq.NewWorkflowSnapshot.ExecutionInfo.RunID,
		})
	s.NoError(err)
	info := state.ExecutionInfo
	s.NotNil(info, "Valid Workflow response expected.")
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.CreateRequestID, info.CreateRequestID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.DomainID, info.DomainID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.WorkflowID, info.WorkflowID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.RunID, info.RunID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.FirstExecutionRunID, info.FirstExecutionRunID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.ParentDomainID, info.ParentDomainID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.ParentWorkflowID, info.ParentWorkflowID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.ParentRunID, info.ParentRunID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.InitiatedID, info.InitiatedID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.TaskList, info.TaskList)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.WorkflowTypeName, info.WorkflowTypeName)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.WorkflowTimeout, info.WorkflowTimeout)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.DecisionStartToCloseTimeout, info.DecisionStartToCloseTimeout)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.ExecutionContext, info.ExecutionContext)
	s.Equal(p.WorkflowStateRunning, info.State)
	s.Equal(p.WorkflowCloseStatusNone, info.CloseStatus)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.NextEventID, info.NextEventID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.LastProcessedEvent, info.LastProcessedEvent)
	s.Equal(true, s.validateTimeRange(info.LastUpdatedTimestamp, time.Hour))
	s.Equal(true, s.validateTimeRange(info.StartTimestamp, time.Hour))
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.DecisionVersion, info.DecisionVersion)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.DecisionScheduleID, info.DecisionScheduleID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.DecisionStartedID, info.DecisionStartedID)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.DecisionTimeout, info.DecisionTimeout)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.SignalCount, info.SignalCount)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.Attempt, info.Attempt)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.HasRetryPolicy, info.HasRetryPolicy)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.InitialInterval, info.InitialInterval)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.BackoffCoefficient, info.BackoffCoefficient)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.MaximumAttempts, info.MaximumAttempts)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.MaximumInterval, info.MaximumInterval)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.ExpirationSeconds, info.ExpirationSeconds)
	s.EqualTimes(createReq.NewWorkflowSnapshot.ExecutionInfo.ExpirationTime, info.ExpirationTime)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.CronSchedule, info.CronSchedule)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.CronOverlapPolicy, info.CronOverlapPolicy)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.ActiveClusterSelectionPolicy, info.ActiveClusterSelectionPolicy)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionInfo.NonRetriableErrors, info.NonRetriableErrors)
	s.Equal(testResetPoints, *info.AutoResetPoints)
	s.Equal(createReq.NewWorkflowSnapshot.ExecutionStats.HistorySize, state.ExecutionStats.HistorySize)
	val, ok := info.SearchAttributes[testSearchAttrKey]
	s.True(ok)
	s.Equal(testSearchAttrVal, val)
	val, ok = info.Memo[testMemoKey]
	s.True(ok)
	s.Equal(testMemoVal, val)
	s.Equal(testPartitionConfig, info.PartitionConfig)

	s.assertChecksumsEqual(csum, state.Checksum)
}

// TestUpdateWorkflow test
func (s *ExecutionManagerSuite) TestUpdateWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "b0a8571c-0257-40ea-afcd-3a14eae181c0"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "update-workflow-test",
		RunID:      "5ba5e531-e46b-48d9-b4b3-859919839553",
	}
	partitionConfig0 := map[string]string{
		"userID": uuid.New(),
	}
	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, partitionConfig0)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")
	s.Equal(domainID, info0.DomainID)
	s.Equal("update-workflow-test", info0.WorkflowID)
	s.Equal("5ba5e531-e46b-48d9-b4b3-859919839553", info0.RunID)
	s.Equal("5ba5e531-e46b-48d9-b4b3-859919839553", info0.FirstExecutionRunID)
	s.Equal("queue1", info0.TaskList)
	s.Equal("wType", info0.WorkflowTypeName)
	s.Equal(int32(20), info0.WorkflowTimeout)
	s.Equal(int32(13), info0.DecisionStartToCloseTimeout)
	s.Equal([]byte(nil), info0.ExecutionContext)
	s.Equal(p.WorkflowStateRunning, info0.State)
	s.Equal(p.WorkflowCloseStatusNone, info0.CloseStatus)
	s.Equal(int64(1), info0.LastFirstEventID)
	s.Equal(int64(3), info0.NextEventID)
	s.Equal(int64(0), info0.LastProcessedEvent)
	s.Equal(true, s.validateTimeRange(info0.LastUpdatedTimestamp, time.Minute))
	s.Equal(true, s.validateTimeRange(info0.StartTimestamp, time.Minute))
	s.Equal(int64(0), info0.DecisionVersion)
	s.Equal(int64(2), info0.DecisionScheduleID)
	s.Equal(constants.EmptyEventID, info0.DecisionStartedID)
	s.Equal(int32(1), info0.DecisionTimeout)
	s.Equal(int64(0), info0.DecisionAttempt)
	s.Equal(int64(0), info0.DecisionStartedTimestamp)
	s.Equal(int64(0), info0.DecisionScheduledTimestamp)
	s.Equal(int64(0), info0.DecisionOriginalScheduledTimestamp)
	s.Empty(info0.StickyTaskList)
	s.Equal(int32(0), info0.StickyScheduleToStartTimeout)
	s.Empty(info0.ClientLibraryVersion)
	s.Empty(info0.ClientFeatureVersion)
	s.Empty(info0.ClientImpl)
	s.Equal(int32(0), info0.SignalCount)
	s.Equal(info0.AutoResetPoints, &types.ResetPoints{})
	s.True(len(info0.SearchAttributes) == 0)
	s.True(len(info0.Memo) == 0)
	s.Equal(partitionConfig0, info0.PartitionConfig)
	s.assertChecksumsEqual(testWorkflowChecksum, state0.Checksum)

	s.T().Logf("Workflow execution last updated: %v\n", info0.LastUpdatedTimestamp)

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := state0.ExecutionStats
	updatedInfo.LastFirstEventID = int64(3)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	updatedInfo.DecisionVersion = int64(666)
	updatedInfo.DecisionAttempt = int64(123)
	updatedInfo.DecisionStartedTimestamp = int64(321)
	updatedInfo.DecisionScheduledTimestamp = int64(654)
	updatedInfo.DecisionOriginalScheduledTimestamp = int64(655)
	updatedInfo.StickyTaskList = "random sticky tasklist"
	updatedInfo.StickyScheduleToStartTimeout = 876
	updatedInfo.ClientLibraryVersion = "random client library version"
	updatedInfo.ClientFeatureVersion = "random client feature version"
	updatedInfo.ClientImpl = "random client impl"
	updatedInfo.SignalCount = 9
	updatedInfo.InitialInterval = math.MaxInt32
	updatedInfo.BackoffCoefficient = 4.45
	updatedInfo.MaximumInterval = math.MaxInt32
	updatedInfo.MaximumAttempts = math.MaxInt32
	updatedInfo.ExpirationSeconds = math.MaxInt32
	updatedInfo.ExpirationTime = time.Now()
	updatedInfo.NonRetriableErrors = []string{"accessDenied", "badRequest"}
	searchAttrKey := "env"
	searchAttrVal := []byte("test")
	updatedInfo.SearchAttributes = map[string][]byte{searchAttrKey: searchAttrVal}
	memoKey := "memoKey"
	memoVal := []byte("memoVal")
	updatedInfo.Memo = map[string][]byte{memoKey: memoVal}
	partitionConfig := map[string]string{"zone": "dca2"}
	updatedInfo.PartitionConfig = partitionConfig
	updatedStats.HistorySize = math.MaxInt64
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.NextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	err2 := s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, []int64{int64(4)}, nil, int64(3), nil, nil, nil, nil, nil)
	s.NoError(err2)

	state1, err3 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err3)
	info1 := state1.ExecutionInfo
	s.NotNil(info1, "Valid Workflow info expected.")
	s.Equal(domainID, info1.DomainID)
	s.Equal("update-workflow-test", info1.WorkflowID)
	s.Equal("5ba5e531-e46b-48d9-b4b3-859919839553", info1.RunID)
	s.Equal("5ba5e531-e46b-48d9-b4b3-859919839553", info1.FirstExecutionRunID)
	s.Equal("queue1", info1.TaskList)
	s.Equal("wType", info1.WorkflowTypeName)
	s.Equal(int32(20), info1.WorkflowTimeout)
	s.Equal(int32(13), info1.DecisionStartToCloseTimeout)
	s.Equal([]byte(nil), info1.ExecutionContext)
	s.Equal(p.WorkflowStateRunning, info1.State)
	s.Equal(p.WorkflowCloseStatusNone, info1.CloseStatus)
	s.Equal(int64(3), info1.LastFirstEventID)
	s.Equal(int64(5), info1.NextEventID)
	s.Equal(int64(2), info1.LastProcessedEvent)
	s.Equal(true, s.validateTimeRange(info1.LastUpdatedTimestamp, time.Hour))
	s.Equal(info0.StartTimestamp.UnixNano(), info1.StartTimestamp.UnixNano())
	s.Equal(int64(666), info1.DecisionVersion)
	s.Equal(int64(2), info1.DecisionScheduleID)
	s.Equal(constants.EmptyEventID, info1.DecisionStartedID)
	s.Equal(int32(1), info1.DecisionTimeout)
	s.Equal(int64(123), info1.DecisionAttempt)
	s.Equal(int64(321), info1.DecisionStartedTimestamp)
	s.Equal(int64(654), info1.DecisionScheduledTimestamp)
	s.Equal(int64(655), info1.DecisionOriginalScheduledTimestamp)
	s.Equal(updatedInfo.StickyTaskList, info1.StickyTaskList)
	s.Equal(updatedInfo.StickyScheduleToStartTimeout, info1.StickyScheduleToStartTimeout)
	s.Equal(updatedInfo.ClientLibraryVersion, info1.ClientLibraryVersion)
	s.Equal(updatedInfo.ClientFeatureVersion, info1.ClientFeatureVersion)
	s.Equal(updatedInfo.ClientImpl, info1.ClientImpl)
	s.Equal(updatedInfo.SignalCount, info1.SignalCount)
	s.EqualValues(updatedStats.HistorySize, state1.ExecutionStats.HistorySize)
	s.Equal(updatedInfo.InitialInterval, info1.InitialInterval)
	s.Equal(updatedInfo.BackoffCoefficient, info1.BackoffCoefficient)
	s.Equal(updatedInfo.MaximumInterval, info1.MaximumInterval)
	s.Equal(updatedInfo.MaximumAttempts, info1.MaximumAttempts)
	s.Equal(updatedInfo.ExpirationSeconds, info1.ExpirationSeconds)
	s.EqualTimes(updatedInfo.ExpirationTime, info1.ExpirationTime)
	s.Equal(updatedInfo.NonRetriableErrors, info1.NonRetriableErrors)
	searchAttrVal1, ok := info1.SearchAttributes[searchAttrKey]
	s.True(ok)
	s.Equal(searchAttrVal, searchAttrVal1)
	memoVal1, ok := info1.Memo[memoKey]
	s.True(ok)
	s.Equal(memoVal, memoVal1)
	s.Equal(partitionConfig, info1.PartitionConfig)
	s.assertChecksumsEqual(testWorkflowChecksum, state1.Checksum)

	s.T().Logf("Workflow execution last updated: %v\n", info1.LastUpdatedTimestamp)

	failedUpdateInfo := copyWorkflowExecutionInfo(updatedInfo)
	failedUpdateStats := copyExecutionStats(updatedStats)
	err4 := s.UpdateWorkflowExecution(ctx, failedUpdateInfo, failedUpdateStats, versionHistories, []int64{int64(5)}, nil, int64(3), nil, nil, nil, nil, nil)
	s.Error(err4, "expected non nil error.")
	s.IsType(&p.ConditionFailedError{}, err4)

	state2, err4 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err4)
	info2 := state2.ExecutionInfo
	s.NotNil(info2, "Valid Workflow info expected.")
	s.Equal(domainID, info2.DomainID)
	s.Equal("update-workflow-test", info2.WorkflowID)
	s.Equal("5ba5e531-e46b-48d9-b4b3-859919839553", info2.RunID)
	s.Equal("5ba5e531-e46b-48d9-b4b3-859919839553", info2.FirstExecutionRunID)
	s.Equal("queue1", info2.TaskList)
	s.Equal("wType", info2.WorkflowTypeName)
	s.Equal(int32(20), info2.WorkflowTimeout)
	s.Equal(int32(13), info2.DecisionStartToCloseTimeout)
	s.Equal([]byte(nil), info2.ExecutionContext)
	s.Equal(p.WorkflowStateRunning, info2.State)
	s.Equal(p.WorkflowCloseStatusNone, info2.CloseStatus)
	s.Equal(int64(5), info2.NextEventID)
	s.Equal(int64(2), info2.LastProcessedEvent)
	s.Equal(true, s.validateTimeRange(info2.LastUpdatedTimestamp, time.Hour))
	s.Equal(int64(666), info2.DecisionVersion)
	s.Equal(int64(2), info2.DecisionScheduleID)
	s.Equal(constants.EmptyEventID, info2.DecisionStartedID)
	s.Equal(int32(1), info2.DecisionTimeout)
	s.Equal(int64(123), info2.DecisionAttempt)
	s.Equal(int64(321), info2.DecisionStartedTimestamp)
	s.Equal(int64(654), info2.DecisionScheduledTimestamp)
	s.Equal(int64(655), info2.DecisionOriginalScheduledTimestamp)
	s.Equal(updatedInfo.SignalCount, info2.SignalCount)
	s.EqualValues(updatedStats.HistorySize, state2.ExecutionStats.HistorySize)
	s.Equal(updatedInfo.InitialInterval, info2.InitialInterval)
	s.Equal(updatedInfo.BackoffCoefficient, info2.BackoffCoefficient)
	s.Equal(updatedInfo.MaximumInterval, info2.MaximumInterval)
	s.Equal(updatedInfo.MaximumAttempts, info2.MaximumAttempts)
	s.Equal(updatedInfo.ExpirationSeconds, info2.ExpirationSeconds)
	s.EqualTimes(updatedInfo.ExpirationTime, info2.ExpirationTime)
	s.Equal(updatedInfo.NonRetriableErrors, info2.NonRetriableErrors)
	searchAttrVal2, ok := info2.SearchAttributes[searchAttrKey]
	s.True(ok)
	s.Equal(searchAttrVal, searchAttrVal2)
	memoVal2, ok := info1.Memo[memoKey]
	s.True(ok)
	s.Equal(memoVal, memoVal2)
	s.Equal(partitionConfig, info2.PartitionConfig)
	s.assertChecksumsEqual(testWorkflowChecksum, state2.Checksum)
	s.T().Logf("Workflow execution last updated: %v", info2.LastUpdatedTimestamp)

	err5 := s.UpdateWorkflowExecutionWithRangeID(ctx, failedUpdateInfo, failedUpdateStats, versionHistories, []int64{int64(5)}, nil, int64(12345), int64(5), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	s.Error(err5, "expected non nil error.")
	s.IsType(&p.ShardOwnershipLostError{}, err5)

	state3, err6 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err6)
	info3 := state3.ExecutionInfo
	s.NotNil(info3, "Valid Workflow info expected.")
	s.Equal(domainID, info3.DomainID)
	s.Equal("update-workflow-test", info3.WorkflowID)
	s.Equal("5ba5e531-e46b-48d9-b4b3-859919839553", info3.RunID)
	s.Equal("5ba5e531-e46b-48d9-b4b3-859919839553", info3.FirstExecutionRunID)
	s.Equal("queue1", info3.TaskList)
	s.Equal("wType", info3.WorkflowTypeName)
	s.Equal(int32(20), info3.WorkflowTimeout)
	s.Equal(int32(13), info3.DecisionStartToCloseTimeout)
	s.Equal([]byte(nil), info3.ExecutionContext)
	s.Equal(p.WorkflowStateRunning, info3.State)
	s.Equal(p.WorkflowCloseStatusNone, info3.CloseStatus)
	s.Equal(int64(5), info3.NextEventID)
	s.Equal(int64(2), info3.LastProcessedEvent)
	s.Equal(true, s.validateTimeRange(info3.LastUpdatedTimestamp, time.Hour))
	s.Equal(int64(666), info3.DecisionVersion)
	s.Equal(int64(2), info3.DecisionScheduleID)
	s.Equal(constants.EmptyEventID, info3.DecisionStartedID)
	s.Equal(int32(1), info3.DecisionTimeout)
	s.Equal(int64(123), info3.DecisionAttempt)
	s.Equal(int64(321), info3.DecisionStartedTimestamp)
	s.Equal(int64(654), info3.DecisionScheduledTimestamp)
	s.Equal(int64(655), info3.DecisionOriginalScheduledTimestamp)
	s.Equal(updatedInfo.SignalCount, info3.SignalCount)
	s.EqualValues(updatedStats.HistorySize, state3.ExecutionStats.HistorySize)
	s.Equal(updatedInfo.InitialInterval, info3.InitialInterval)
	s.Equal(updatedInfo.BackoffCoefficient, info3.BackoffCoefficient)
	s.Equal(updatedInfo.MaximumInterval, info3.MaximumInterval)
	s.Equal(updatedInfo.MaximumAttempts, info3.MaximumAttempts)
	s.Equal(updatedInfo.ExpirationSeconds, info3.ExpirationSeconds)
	s.EqualTimes(updatedInfo.ExpirationTime, info3.ExpirationTime)
	s.Equal(updatedInfo.NonRetriableErrors, info3.NonRetriableErrors)
	searchAttrVal3, ok := info3.SearchAttributes[searchAttrKey]
	s.True(ok)
	s.Equal(searchAttrVal, searchAttrVal3)
	memoVal3, ok := info1.Memo[memoKey]
	s.True(ok)
	s.Equal(memoVal, memoVal3)
	s.Equal(partitionConfig, info3.PartitionConfig)
	s.assertChecksumsEqual(testWorkflowChecksum, state3.Checksum)

	s.T().Logf("Workflow execution last updated: %v\n", info3.LastUpdatedTimestamp)

	// update with incorrect rangeID and condition(next_event_id)
	err7 := s.UpdateWorkflowExecutionWithRangeID(ctx, failedUpdateInfo, failedUpdateStats, versionHistories, []int64{int64(5)}, nil, int64(12345), int64(3), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	s.Error(err7, "expected non nil error.")
	s.IsType(&p.ShardOwnershipLostError{}, err7)

	state4, err8 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err8)
	info4 := state4.ExecutionInfo
	s.NotNil(info4, "Valid Workflow info expected.")
	s.Equal(domainID, info4.DomainID)
	s.Equal("update-workflow-test", info4.WorkflowID)
	s.Equal("5ba5e531-e46b-48d9-b4b3-859919839553", info4.RunID)
	s.Equal("5ba5e531-e46b-48d9-b4b3-859919839553", info4.FirstExecutionRunID)
	s.Equal("queue1", info4.TaskList)
	s.Equal("wType", info4.WorkflowTypeName)
	s.Equal(int32(20), info4.WorkflowTimeout)
	s.Equal(int32(13), info4.DecisionStartToCloseTimeout)
	s.Equal([]byte(nil), info4.ExecutionContext)
	s.Equal(p.WorkflowStateRunning, info4.State)
	s.Equal(p.WorkflowCloseStatusNone, info4.CloseStatus)
	s.Equal(int64(5), info4.NextEventID)
	s.Equal(int64(2), info4.LastProcessedEvent)
	s.Equal(true, s.validateTimeRange(info4.LastUpdatedTimestamp, time.Hour))
	s.Equal(int64(666), info4.DecisionVersion)
	s.Equal(int64(2), info4.DecisionScheduleID)
	s.Equal(constants.EmptyEventID, info4.DecisionStartedID)
	s.Equal(int32(1), info4.DecisionTimeout)
	s.Equal(int64(123), info4.DecisionAttempt)
	s.Equal(int64(321), info4.DecisionStartedTimestamp)
	s.Equal(updatedInfo.SignalCount, info4.SignalCount)
	s.EqualValues(updatedStats.HistorySize, state4.ExecutionStats.HistorySize)
	s.Equal(updatedInfo.InitialInterval, info4.InitialInterval)
	s.Equal(updatedInfo.BackoffCoefficient, info4.BackoffCoefficient)
	s.Equal(updatedInfo.MaximumInterval, info4.MaximumInterval)
	s.Equal(updatedInfo.MaximumAttempts, info4.MaximumAttempts)
	s.Equal(updatedInfo.ExpirationSeconds, info4.ExpirationSeconds)
	s.EqualTimes(updatedInfo.ExpirationTime, info4.ExpirationTime)
	s.Equal(updatedInfo.NonRetriableErrors, info4.NonRetriableErrors)
	searchAttrVal4, ok := info4.SearchAttributes[searchAttrKey]
	s.True(ok)
	s.Equal(searchAttrVal, searchAttrVal4)
	memoVal4, ok := info1.Memo[memoKey]
	s.True(ok)
	s.Equal(memoVal, memoVal4)
	s.Equal(partitionConfig, info4.PartitionConfig)
	s.assertChecksumsEqual(testWorkflowChecksum, state4.Checksum)

	s.T().Logf("Workflow execution last updated: %v\n", info4.LastUpdatedTimestamp)
}

// TestDeleteWorkflow test
func (s *ExecutionManagerSuite) TestDeleteWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "1d4abb23-b87b-457b-96ef-43aba0b9c44f"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "delete-workflow-test",
		RunID:      "4e0917f2-9361-4a14-b16f-1fafe09b287a",
	}
	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")
	s.Equal(domainID, info0.DomainID)
	s.Equal("delete-workflow-test", info0.WorkflowID)
	s.Equal("4e0917f2-9361-4a14-b16f-1fafe09b287a", info0.RunID)
	s.Equal("4e0917f2-9361-4a14-b16f-1fafe09b287a", info0.FirstExecutionRunID)
	s.Equal("queue1", info0.TaskList)
	s.Equal("wType", info0.WorkflowTypeName)
	s.Equal(int32(20), info0.WorkflowTimeout)
	s.Equal(int32(13), info0.DecisionStartToCloseTimeout)
	s.Equal([]byte(nil), info0.ExecutionContext)
	s.Equal(p.WorkflowStateRunning, info0.State)
	s.Equal(p.WorkflowCloseStatusNone, info0.CloseStatus)
	s.Equal(int64(3), info0.NextEventID)
	s.Equal(int64(0), info0.LastProcessedEvent)
	s.Equal(true, s.validateTimeRange(info0.LastUpdatedTimestamp, time.Hour))
	s.Equal(int64(2), info0.DecisionScheduleID)
	s.Equal(constants.EmptyEventID, info0.DecisionStartedID)
	s.Equal(int32(1), info0.DecisionTimeout)

	s.T().Logf("Workflow execution last updated: %v\n", info0.LastUpdatedTimestamp)

	err4 := s.DeleteWorkflowExecution(ctx, info0)
	s.NoError(err4)

	_, err3 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Error(err3, "expected non nil error.")
	s.IsType(&types.EntityNotExistsError{}, err3)

	err5 := s.DeleteWorkflowExecution(ctx, info0)
	s.NoError(err5)
}

// TestDeleteCurrentWorkflow test
func (s *ExecutionManagerSuite) TestDeleteCurrentWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	if s.ExecutionManager.GetName() != "cassandra" {
		// "this test is only applicable for cassandra (uses TTL based deletes)"
		return
	}
	domainID := "54d15308-e20e-4b91-a00f-a518a3892790"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "delete-current-workflow-test",
		RunID:      "6cae4054-6ba7-46d3-8755-e3c2db6f74ea",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	runID0, err1 := s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.NoError(err1)
	s.Equal(workflowExecution.GetRunID(), runID0)

	info0, err2 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)

	updatedInfo1 := copyWorkflowExecutionInfo(info0.ExecutionInfo)
	updatedStats1 := copyExecutionStats(info0.ExecutionStats)
	updatedInfo1.NextEventID = int64(6)
	updatedInfo1.LastProcessedEvent = int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo1.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err3 := s.UpdateWorkflowExecutionAndFinish(ctx, updatedInfo1, updatedStats1, int64(3), versionHistories)
	s.NoError(err3)

	runID4, err4 := s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.NoError(err4)
	s.Equal(workflowExecution.GetRunID(), runID4)

	fakeInfo := &p.WorkflowExecutionInfo{
		DomainID:   info0.ExecutionInfo.DomainID,
		WorkflowID: info0.ExecutionInfo.WorkflowID,
		RunID:      uuid.New(),
	}

	// test wrong run id with conditional delete
	s.NoError(s.DeleteCurrentWorkflowExecution(ctx, fakeInfo))

	runID5, err5 := s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.NoError(err5)
	s.Equal(workflowExecution.GetRunID(), runID5)

	// simulate a timer_task deleting execution after retention
	s.NoError(s.DeleteCurrentWorkflowExecution(ctx, info0.ExecutionInfo))

	runID0, err1 = s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.Error(err1)
	s.Empty(runID0)
	_, ok := err1.(*types.EntityNotExistsError)
	s.True(ok)

	// execution record should still be there
	_, err2 = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)
}

// TestUpdateDeleteWorkflow mocks the timer behavior to clean up workflow.
func (s *ExecutionManagerSuite) TestUpdateDeleteWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	finishedCurrentExecutionRetentionTTL := int32(2)
	domainID := "54d15308-e20e-4b91-a00f-a518a3892790"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "update-delete-workflow-test",
		RunID:      "6cae4054-6ba7-46d3-8755-e3c2db6f74ea",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	runID0, err1 := s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.NoError(err1)
	s.Equal(workflowExecution.GetRunID(), runID0)

	info0, err2 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)

	updatedInfo1 := copyWorkflowExecutionInfo(info0.ExecutionInfo)
	updatedStats1 := copyExecutionStats(info0.ExecutionStats)
	updatedInfo1.NextEventID = int64(6)
	updatedInfo1.LastProcessedEvent = int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo1.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err3 := s.UpdateWorkflowExecutionAndFinish(ctx, updatedInfo1, updatedStats1, int64(3), versionHistories)
	s.NoError(err3)

	runID4, err4 := s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.NoError(err4)
	s.Equal(workflowExecution.GetRunID(), runID4)

	// simulate a timer_task deleting execution after retention
	err5 := s.DeleteCurrentWorkflowExecution(ctx, info0.ExecutionInfo)
	s.NoError(err5)
	err6 := s.DeleteWorkflowExecution(ctx, info0.ExecutionInfo)
	s.NoError(err6)

	time.Sleep(time.Duration(finishedCurrentExecutionRetentionTTL*2) * time.Second)

	runID0, err1 = s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.Error(err1)
	s.Empty(runID0)
	_, ok := err1.(*types.EntityNotExistsError)
	s.True(ok)

	// execution record should still be there
	_, err2 = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Error(err2)
	_, ok = err2.(*types.EntityNotExistsError)
	s.True(ok)
}

// TestCleanupCorruptedWorkflow test
func (s *ExecutionManagerSuite) TestCleanupCorruptedWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "54d15308-e20e-4b91-a00f-a518a3892790"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "cleanup-corrupted-workflow-test",
		RunID:      "6cae4054-6ba7-46d3-8755-e3c2db6f74ea",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	runID0, err1 := s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.NoError(err1)
	s.Equal(workflowExecution.GetRunID(), runID0)

	info0, err2 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)

	// deleting current record and verify
	err3 := s.DeleteCurrentWorkflowExecution(ctx, info0.ExecutionInfo)
	s.NoError(err3)
	runID0, err4 := s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.Error(err4)
	s.Empty(runID0)
	_, ok := err4.(*types.EntityNotExistsError)
	s.True(ok)

	// we should still be able to load with runID
	info1, err5 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err5)
	s.Equal(info0, info1)

	// mark it as corrupted
	info0.ExecutionInfo.State = p.WorkflowStateCorrupted
	_, err6 := s.ExecutionManager.UpdateWorkflowExecution(ctx, &p.UpdateWorkflowExecutionRequest{
		UpdateWorkflowMutation: p.WorkflowMutation{
			ExecutionInfo:    info0.ExecutionInfo,
			ExecutionStats:   info0.ExecutionStats,
			Condition:        info0.ExecutionInfo.NextEventID,
			Checksum:         testWorkflowChecksum,
			VersionHistories: info0.VersionHistories,
		},
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.UpdateWorkflowModeBypassCurrent,
	})
	s.NoError(err6)

	// we should still be able to load with runID
	info2, err7 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err7)
	s.Equal(p.WorkflowStateCorrupted, info2.ExecutionInfo.State)
	info2.ExecutionInfo.State = info1.ExecutionInfo.State
	info2.ExecutionInfo.LastUpdatedTimestamp = info1.ExecutionInfo.LastUpdatedTimestamp
	s.Equal(info2, info1)

	// delete the run
	err8 := s.DeleteWorkflowExecution(ctx, info0.ExecutionInfo)
	s.NoError(err8)

	// execution record should be gone
	_, err9 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.Error(err9)
	_, ok = err9.(*types.EntityNotExistsError)
	s.True(ok)
}

// TestGetCurrentWorkflow test
func (s *ExecutionManagerSuite) TestGetCurrentWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "54d15308-e20e-4b91-a00f-a518a3892790"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "get-current-workflow-test",
		RunID:      "6cae4054-6ba7-46d3-8755-e3c2db6f74ea",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	response, err := s.ExecutionManager.GetCurrentExecution(ctx, &p.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowExecution.GetWorkflowID(),
	})
	s.NoError(err)
	s.Equal(workflowExecution.GetRunID(), response.RunID)
	s.Equal(constants.EmptyVersion, response.LastWriteVersion)

	info0, err2 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)

	updatedInfo1 := copyWorkflowExecutionInfo(info0.ExecutionInfo)
	updatedStats1 := copyExecutionStats(info0.ExecutionStats)
	updatedInfo1.NextEventID = int64(6)
	updatedInfo1.LastProcessedEvent = int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo1.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err3 := s.UpdateWorkflowExecutionAndFinish(ctx, updatedInfo1, updatedStats1, int64(3), versionHistories)
	s.NoError(err3)

	runID4, err4 := s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.NoError(err4)
	s.Equal(workflowExecution.GetRunID(), runID4)

	workflowExecution2 := types.WorkflowExecution{
		WorkflowID: "get-current-workflow-test",
		RunID:      "c3ff4bc6-de18-4643-83b2-037a33f45322",
	}

	task1, err5 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution2, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.Error(err5, "Error expected.")
	s.Empty(task1, "Expected empty task identifier.")
}

// TestTransferTasksThroughUpdate test
func (s *ExecutionManagerSuite) TestTransferTasksThroughUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "b785a8ba-bd7d-4760-bb05-41b115f3e10a"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "get-transfer-tasks-through-update-test",
		RunID:      "30a9fa1f-0db1-4d7a-8c34-aa82c5dad3aa",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	tasks1, err1 := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err1)
	s.NotNil(tasks1, "expected valid list of tasks.")
	s.Equal(1, len(tasks1), "Expected 1 transfer task.")
	task1, ok := tasks1[0].(*p.DecisionTask)
	s.True(ok, "Expected a decision task.")
	s.Equal(domainID, task1.GetDomainID())
	s.Equal(workflowExecution.GetWorkflowID(), task1.GetWorkflowID())
	s.Equal(workflowExecution.GetRunID(), task1.GetRunID())
	s.Equal("queue1", task1.TaskList)
	s.Equal(p.TransferTaskTypeDecisionTask, task1.GetTaskType())
	s.Equal(int64(2), task1.ScheduleID)

	err3 := s.CompleteTransferTask(ctx, task1.TaskID)
	s.NoError(err3)

	state0, err11 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err11)
	info0 := state0.ExecutionInfo
	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats0 := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats0, versionHistories, nil, []int64{int64(4)}, int64(3), nil, nil, nil, nil, nil)
	s.NoError(err2)

	tasks2, err1 := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err1)
	s.NotNil(tasks2, "expected valid list of tasks.")
	s.Equal(1, len(tasks2), "Expected 1 transfer task.")
	task2, ok := tasks2[0].(*p.ActivityTask)
	s.True(ok, "Expected an activity task.")
	s.Equal(domainID, task2.GetDomainID())
	s.Equal(workflowExecution.GetWorkflowID(), task2.GetWorkflowID())
	s.Equal(workflowExecution.GetRunID(), task2.GetRunID())
	s.Equal("queue1", task2.TaskList)
	s.Equal(p.TransferTaskTypeActivityTask, task2.GetTaskType())
	s.Equal(int64(4), task2.ScheduleID)

	err4 := s.CompleteTransferTask(ctx, task2.TaskID)
	s.NoError(err4)

	state1, _ := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	info1 := state1.ExecutionInfo
	updatedInfo1 := copyWorkflowExecutionInfo(info1)
	updatedStats1 := copyExecutionStats(state1.ExecutionStats)
	updatedInfo1.NextEventID = int64(6)
	updatedInfo1.LastProcessedEvent = int64(2)
	err5 := s.UpdateWorkflowExecutionAndFinish(ctx, updatedInfo1, updatedStats1, int64(5), versionHistories)
	s.NoError(err5)

	newExecution := types.WorkflowExecution{
		WorkflowID: workflowExecution.GetWorkflowID(),
		RunID:      "2a038c8f-b575-4151-8d2c-d443e999ab5a",
	}
	runID6, err6 := s.GetCurrentWorkflowRunID(ctx, domainID, newExecution.GetWorkflowID())
	s.NoError(err6)
	s.Equal(workflowExecution.GetRunID(), runID6)

	tasks3, err7 := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err7)
	s.NotNil(tasks3, "expected valid list of tasks.")
	s.Equal(1, len(tasks3), "Expected 1 transfer task.")
	task3, ok := tasks3[0].(*p.CloseExecutionTask)
	s.True(ok, "Expected a close execution task.")
	s.Equal(domainID, task3.GetDomainID())
	s.Equal(workflowExecution.GetWorkflowID(), task3.GetWorkflowID())
	s.Equal(workflowExecution.GetRunID(), task3.GetRunID())
	s.Equal(p.TransferTaskTypeCloseExecution, task3.GetTaskType())

	err8 := s.CompleteTransferTask(ctx, task3.TaskID)
	s.NoError(err8)

	_, err9 := s.CreateWorkflowExecution(ctx, domainID, newExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.Error(err9, "createWFExecution (brand_new) must fail when there is a previous instance of workflow state already in DB")

	err10 := s.DeleteWorkflowExecution(ctx, info1)
	s.NoError(err10)
}

// TestCancelTransferTaskTasks test
func (s *ExecutionManagerSuite) TestCancelTransferTaskTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "aeac8287-527b-4b35-80a9-667cb47e7c6d"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "cancel-workflow-test",
		RunID:      "db20f7e2-1a1e-40d9-9278-d8b886738e05",
	}

	task0, err := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err)
	s.NotNil(task0, "Expected non empty task identifier.")

	taskD, err := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err)
	s.Equal(1, len(taskD), "Expected 1 decision task.")
	err = s.CompleteTransferTask(ctx, taskD[0].GetTaskID())
	s.NoError(err)

	state1, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err)
	info1 := state1.ExecutionInfo
	s.NotNil(info1, "Valid Workflow info expected.")
	updatedInfo1 := copyWorkflowExecutionInfo(info1)
	updatedStats1 := copyExecutionStats(state1.ExecutionStats)

	targetDomainID := "f2bfaab6-7e8b-4fac-9a62-17da8d37becb"
	targetWorkflowID := "target-workflow-cancellation-id-1"
	targetRunID := "0d00698f-08e1-4d36-a3e2-3bf109f5d2d6"
	targetChildWorkflowOnly := false
	transferTasks := []p.Task{&p.CancelExecutionTask{
		WorkflowIdentifier: p.WorkflowIdentifier{
			DomainID:   domainID,
			WorkflowID: workflowExecution.WorkflowID,
			RunID:      workflowExecution.RunID,
		},
		TaskData: p.TaskData{
			TaskID: s.GetNextSequenceNumber(),
		},
		TargetDomainID:          targetDomainID,
		TargetWorkflowID:        targetWorkflowID,
		TargetRunID:             targetRunID,
		TargetChildWorkflowOnly: targetChildWorkflowOnly,
		InitiatedID:             1,
	}}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: 1,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err = s.UpdateWorkflowExecutionWithTransferTasks(ctx, updatedInfo1, updatedStats1, int64(3), transferTasks, nil, versionHistories)
	s.NoError(err)

	tasks1, err := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err)
	s.NotNil(tasks1, "expected valid list of tasks.")
	s.Equal(1, len(tasks1), "Expected 1 cancel task.")
	task1, ok := tasks1[0].(*p.CancelExecutionTask)
	s.True(ok, "Expected a cancel execution task.")
	s.Equal(p.TransferTaskTypeCancelExecution, task1.GetTaskType())
	s.Equal(domainID, task1.GetDomainID())
	s.Equal(workflowExecution.GetWorkflowID(), task1.GetWorkflowID())
	s.Equal(workflowExecution.GetRunID(), task1.GetRunID())
	s.Equal(targetDomainID, task1.TargetDomainID)
	s.Equal(targetWorkflowID, task1.TargetWorkflowID)
	s.Equal(targetRunID, task1.TargetRunID)
	s.Equal(false, task1.TargetChildWorkflowOnly)

	err = s.CompleteTransferTask(ctx, task1.TaskID)
	s.NoError(err)

	targetDomainID = "f2bfaab6-7e8b-4fac-9a62-17da8d37becb"
	targetWorkflowID = "target-workflow-cancellation-id-2"
	targetRunID = ""
	targetChildWorkflowOnly = true
	transferTasks = []p.Task{&p.CancelExecutionTask{
		WorkflowIdentifier: p.WorkflowIdentifier{
			DomainID:   domainID,
			WorkflowID: workflowExecution.WorkflowID,
			RunID:      workflowExecution.RunID,
		},
		TaskData: p.TaskData{
			TaskID: s.GetNextSequenceNumber(),
		},
		TargetDomainID:          targetDomainID,
		TargetWorkflowID:        targetWorkflowID,
		TargetRunID:             targetRunID,
		TargetChildWorkflowOnly: targetChildWorkflowOnly,
		InitiatedID:             3,
	}}

	state2, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err)
	info2 := state2.ExecutionInfo
	s.NotNil(info2, "Valid Workflow info expected.")
	updatedInfo2 := copyWorkflowExecutionInfo(info2)
	updatedStats2 := copyExecutionStats(state2.ExecutionStats)

	err = s.UpdateWorkflowExecutionWithTransferTasks(ctx, updatedInfo2, updatedStats2, int64(3), transferTasks, nil, versionHistories)
	s.NoError(err)

	tasks2, err := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err)
	s.NotNil(tasks2, "expected valid list of tasks.")
	s.Equal(1, len(tasks2), "Expected 1 cancel task.")
	task2, ok := tasks2[0].(*p.CancelExecutionTask)
	s.True(ok, "Expected a cancel execution task.")
	s.Equal(p.TransferTaskTypeCancelExecution, task2.GetTaskType())
	s.Equal(domainID, task2.GetDomainID())
	s.Equal(workflowExecution.GetWorkflowID(), task2.GetWorkflowID())
	s.Equal(workflowExecution.GetRunID(), task2.GetRunID())
	s.Equal(targetDomainID, task2.TargetDomainID)
	s.Equal(targetWorkflowID, task2.TargetWorkflowID)
	s.Equal(targetRunID, task2.TargetRunID)
	s.Equal(targetChildWorkflowOnly, task2.TargetChildWorkflowOnly)

	err = s.CompleteTransferTask(ctx, task2.TaskID)
	s.NoError(err)
}

// TestSignalTransferTaskTasks test
func (s *ExecutionManagerSuite) TestSignalTransferTaskTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "aeac8287-527b-4b35-80a9-667cb47e7c6d"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "signal-workflow-test",
		RunID:      "db20f7e2-1a1e-40d9-9278-d8b886738e05",
	}

	task0, err := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err)
	s.NotNil(task0, "Expected non empty task identifier.")

	taskD, err := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err)
	s.Equal(1, len(taskD), "Expected 1 decision task.")
	err = s.CompleteTransferTask(ctx, taskD[0].GetTaskID())
	s.NoError(err)

	state1, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err)
	info1 := state1.ExecutionInfo
	s.NotNil(info1, "Valid Workflow info expected.")
	updatedInfo1 := copyWorkflowExecutionInfo(info1)
	updatedStats1 := copyExecutionStats(state1.ExecutionStats)

	targetDomainID := "f2bfaab6-7e8b-4fac-9a62-17da8d37becb"
	targetWorkflowID := "target-workflow-signal-id-1"
	targetRunID := "0d00698f-08e1-4d36-a3e2-3bf109f5d2d6"
	targetChildWorkflowOnly := false
	transferTasks := []p.Task{&p.SignalExecutionTask{
		WorkflowIdentifier: p.WorkflowIdentifier{
			DomainID:   domainID,
			WorkflowID: workflowExecution.WorkflowID,
			RunID:      workflowExecution.RunID,
		},
		TaskData: p.TaskData{
			TaskID: s.GetNextSequenceNumber(),
		},
		TargetDomainID:          targetDomainID,
		TargetWorkflowID:        targetWorkflowID,
		TargetRunID:             targetRunID,
		TargetChildWorkflowOnly: false,
		InitiatedID:             1,
	}}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: 1,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err = s.UpdateWorkflowExecutionWithTransferTasks(ctx, updatedInfo1, updatedStats1, int64(3), transferTasks, nil, versionHistories)
	s.NoError(err)

	tasks1, err := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err)
	s.NotNil(tasks1, "expected valid list of tasks.")
	s.Equal(1, len(tasks1), "Expected 1 transfer task.")
	task1, ok := tasks1[0].(*p.SignalExecutionTask)
	s.True(ok, "Expected a signal execution task.")
	s.Equal(p.TransferTaskTypeSignalExecution, task1.GetTaskType())
	s.Equal(domainID, task1.GetDomainID())
	s.Equal(workflowExecution.GetWorkflowID(), task1.GetWorkflowID())
	s.Equal(workflowExecution.GetRunID(), task1.GetRunID())
	s.Equal(targetDomainID, task1.TargetDomainID)
	s.Equal(targetWorkflowID, task1.TargetWorkflowID)
	s.Equal(targetRunID, task1.TargetRunID)
	s.Equal(false, task1.TargetChildWorkflowOnly)

	err = s.CompleteTransferTask(ctx, task1.TaskID)
	s.NoError(err)

	targetDomainID = "f2bfaab6-7e8b-4fac-9a62-17da8d37becb"
	targetWorkflowID = "target-workflow-signal-id-2"
	targetRunID = ""
	targetChildWorkflowOnly = true
	transferTasks = []p.Task{&p.SignalExecutionTask{
		WorkflowIdentifier: p.WorkflowIdentifier{
			DomainID:   domainID,
			WorkflowID: workflowExecution.WorkflowID,
			RunID:      workflowExecution.RunID,
		},
		TaskData: p.TaskData{
			TaskID: s.GetNextSequenceNumber(),
		},
		TargetDomainID:          targetDomainID,
		TargetWorkflowID:        targetWorkflowID,
		TargetRunID:             targetRunID,
		TargetChildWorkflowOnly: targetChildWorkflowOnly,
		InitiatedID:             3,
	}}

	state2, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err)
	info2 := state2.ExecutionInfo
	s.NotNil(info2, "Valid Workflow info expected.")
	updatedInfo2 := copyWorkflowExecutionInfo(info2)
	updatedStats2 := copyExecutionStats(state2.ExecutionStats)

	err = s.UpdateWorkflowExecutionWithTransferTasks(ctx, updatedInfo2, updatedStats2, int64(3), transferTasks, nil, versionHistories)
	s.NoError(err)

	tasks2, err := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err)
	s.NotNil(tasks2, "expected valid list of tasks.")
	s.Equal(1, len(tasks2), "Expected 1 transfer task.")
	task2, ok := tasks2[0].(*p.SignalExecutionTask)
	s.True(ok, "Expected a signal execution task.")
	s.Equal(p.TransferTaskTypeSignalExecution, task2.GetTaskType())
	s.Equal(domainID, task2.GetDomainID())
	s.Equal(workflowExecution.GetWorkflowID(), task2.GetWorkflowID())
	s.Equal(workflowExecution.GetRunID(), task2.GetRunID())
	s.Equal(targetDomainID, task2.TargetDomainID)
	s.Equal(targetWorkflowID, task2.TargetWorkflowID)
	s.Equal(targetRunID, task2.TargetRunID)
	s.Equal(targetChildWorkflowOnly, task2.TargetChildWorkflowOnly)

	err = s.CompleteTransferTask(ctx, task2.TaskID)
	s.NoError(err)
}

// TestReplicationTasks test
func (s *ExecutionManagerSuite) TestReplicationTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "2466d7de-6602-4ad8-b939-fb8f8c36c711"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "get-replication-tasks-test",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}

	task0, err := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err)
	s.NotNil(task0, "Expected non empty task identifier.")
	taskD, err := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err)
	s.Equal(1, len(taskD), "Expected 1 decision task.")
	err = s.CompleteTransferTask(ctx, taskD[0].GetTaskID())
	s.NoError(err)

	state1, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err)
	info1 := state1.ExecutionInfo
	s.NotNil(info1, "Valid Workflow info expected.")
	updatedInfo1 := copyWorkflowExecutionInfo(info1)
	updatedStats1 := copyExecutionStats(state1.ExecutionStats)

	replicationTasks := []p.Task{
		&p.HistoryReplicationTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				TaskID:  s.GetNextSequenceNumber(),
				Version: 123,
			},
			FirstEventID: int64(1),
			NextEventID:  int64(3),
		},
		&p.HistoryReplicationTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				TaskID:  s.GetNextSequenceNumber(),
				Version: 456,
			},
			FirstEventID: int64(1),
			NextEventID:  int64(3),
		},
		&p.SyncActivityTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				TaskID:  s.GetNextSequenceNumber(),
				Version: 789,
			},
			ScheduledID: 99,
		},
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: 3,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err = s.UpdateWorklowStateAndReplication(ctx, updatedInfo1, updatedStats1, versionHistories, int64(3), replicationTasks)
	s.NoError(err)

	respTasks, err := s.GetReplicationTasks(ctx, 1, true)
	s.NoError(err)
	s.Equal(len(replicationTasks), len(respTasks))

	for index := range replicationTasks {
		respTasks[index].SetVisibilityTimestamp(replicationTasks[index].GetVisibilityTimestamp()) // ignore visibility timestamp
		s.Equal(replicationTasks[index], respTasks[index])
		err = s.CompleteReplicationTask(ctx, respTasks[index].GetTaskID())
		s.NoError(err)
	}
}

// TestTransferTasksComplete test
func (s *ExecutionManagerSuite) TestTransferTasksComplete() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "8bfb47be-5b57-4d55-9109-5fb35e20b1d7"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "get-transfer-tasks-test-complete",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}
	tasklist := "some random tasklist"

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, tasklist, "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	tasks1, err1 := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err1)
	s.NotNil(tasks1, "expected valid list of tasks.")
	s.Equal(1, len(tasks1), "Expected 1 decision task.")
	task1, ok := tasks1[0].(*p.DecisionTask)
	s.True(ok, "Expected a decision task.")
	s.Equal(domainID, task1.DomainID)
	s.Equal(workflowExecution.GetWorkflowID(), task1.WorkflowID)
	s.Equal(workflowExecution.GetRunID(), task1.RunID)
	s.Equal(tasklist, task1.TaskList)
	s.Equal(p.TransferTaskTypeDecisionTask, task1.GetTaskType())
	s.Equal(int64(2), task1.ScheduleID)
	err3 := s.CompleteTransferTask(ctx, task1.TaskID)
	s.NoError(err3)

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(6)
	updatedInfo.LastProcessedEvent = int64(2)
	scheduleID := int64(123)
	targetDomainID := "8bfb47be-5b57-4d66-9109-5fb35e20b1d0"
	targetWorkflowID := "some random target domain ID"
	targetRunID := uuid.New()
	currentTransferID := task1.TaskID
	now := time.Now()
	tasks := []p.Task{
		&p.ActivityTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10000,
				Version:             111,
			},
			TargetDomainID: domainID,
			TaskList:       tasklist,
			ScheduleID:     scheduleID,
		},
		&p.DecisionTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10002,
				Version:             222,
			},
			TargetDomainID: domainID,
			TaskList:       tasklist,
			ScheduleID:     scheduleID,
		},
		&p.CloseExecutionTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10003,
				Version:             333,
			}},
		&p.CancelExecutionTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10004,
				Version:             444,
			},
			TargetDomainID:          targetDomainID,
			TargetWorkflowID:        targetWorkflowID,
			TargetRunID:             targetRunID,
			TargetChildWorkflowOnly: true,
			InitiatedID:             scheduleID,
		},
		&p.SignalExecutionTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10005,
				Version:             555,
			},
			TargetDomainID:          targetDomainID,
			TargetWorkflowID:        targetWorkflowID,
			TargetRunID:             targetRunID,
			TargetChildWorkflowOnly: true,
			InitiatedID:             scheduleID,
		},
		&p.StartChildExecutionTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10006,
				Version:             666,
			},
			TargetDomainID:   targetDomainID,
			TargetWorkflowID: targetWorkflowID,
			InitiatedID:      scheduleID,
		},
		&p.RecordWorkflowClosedTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10007,
				Version:             777,
			},
		},
		&p.RecordChildExecutionCompletedTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10008,
				Version:             888,
			},
			TargetDomainID:   targetDomainID,
			TargetWorkflowID: targetWorkflowID,
			TargetRunID:      targetRunID,
		},
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: scheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpdateWorklowStateAndReplication(ctx, updatedInfo, updatedStats, versionHistories, int64(3), tasks)
	s.NoError(err2)

	txTasks, err1 := s.GetTransferTasks(ctx, 1, true) // use page size one to force pagination
	s.NoError(err1)
	s.NotNil(txTasks, "expected valid list of tasks.")
	s.Equal(len(tasks), len(txTasks))
	for index := range tasks {
		s.True(timeComparator(tasks[index].GetVisibilityTimestamp(), txTasks[index].GetVisibilityTimestamp(), TimePrecision))
	}
	s.Equal(p.TransferTaskTypeActivityTask, txTasks[0].GetTaskType())
	s.Equal(p.TransferTaskTypeDecisionTask, txTasks[1].GetTaskType())
	s.Equal(p.TransferTaskTypeCloseExecution, txTasks[2].GetTaskType())
	s.Equal(p.TransferTaskTypeCancelExecution, txTasks[3].GetTaskType())
	s.Equal(p.TransferTaskTypeSignalExecution, txTasks[4].GetTaskType())
	s.Equal(p.TransferTaskTypeStartChildExecution, txTasks[5].GetTaskType())
	s.Equal(p.TransferTaskTypeRecordWorkflowClosed, txTasks[6].GetTaskType())
	s.Equal(p.TransferTaskTypeRecordChildExecutionCompleted, txTasks[7].GetTaskType())

	for idx := range txTasks {
		// TODO: add a check similar to validateCrossClusterTasks
		s.Equal(int64(111*(idx+1)), txTasks[idx].GetVersion())
		err := s.CompleteTransferTask(ctx, txTasks[idx].GetTaskID())
		s.NoError(err)
	}

	txTasks, err2 = s.GetTransferTasks(ctx, 100, false)
	s.NoError(err2)
	s.Empty(txTasks, "expected empty task list.")
}

// TestTransferTasksRangeComplete test
func (s *ExecutionManagerSuite) TestTransferTasksRangeComplete() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "8bfb47be-5b57-4d55-9109-5fb35e20b1d7"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "get-transfer-tasks-test-range-complete",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}
	tasklist := "some random tasklist"

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, tasklist, "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	tasks1, err1 := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err1)
	s.NotNil(tasks1, "expected valid list of tasks.")
	s.Equal(1, len(tasks1), "Expected 1 decision task.")
	task1, ok := tasks1[0].(*p.DecisionTask)
	s.True(ok, "Expected a decision task.")
	s.Equal(domainID, task1.DomainID)
	s.Equal(workflowExecution.GetWorkflowID(), task1.WorkflowID)
	s.Equal(workflowExecution.GetRunID(), task1.RunID)
	s.Equal(tasklist, task1.TaskList)
	s.Equal(p.TransferTaskTypeDecisionTask, task1.GetTaskType())
	s.Equal(int64(2), task1.ScheduleID)
	err3 := s.CompleteTransferTask(ctx, task1.TaskID)
	s.NoError(err3)

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(6)
	updatedInfo.LastProcessedEvent = int64(2)
	scheduleID := int64(123)
	targetDomainID := "8bfb47be-5b57-4d66-9109-5fb35e20b1d0"
	targetWorkflowID := "some random target domain ID"
	targetRunID := uuid.New()
	currentTransferID := task1.TaskID
	now := time.Now()
	tasks := []p.Task{
		&p.ActivityTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10001,
				Version:             111,
			},
			TargetDomainID: domainID,
			TaskList:       tasklist,
			ScheduleID:     scheduleID,
		},
		&p.DecisionTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10002,
				Version:             222,
			},
			TargetDomainID: domainID,
			TaskList:       tasklist,
			ScheduleID:     scheduleID,
		},
		&p.CloseExecutionTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10003,
				Version:             333,
			},
		},
		&p.CancelExecutionTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10004,
				Version:             444,
			},
			TargetDomainID:          targetDomainID,
			TargetWorkflowID:        targetWorkflowID,
			TargetRunID:             targetRunID,
			TargetChildWorkflowOnly: true,
			InitiatedID:             scheduleID,
		},
		&p.SignalExecutionTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10005,
				Version:             555,
			},
			TargetDomainID:          targetDomainID,
			TargetWorkflowID:        targetWorkflowID,
			TargetRunID:             targetRunID,
			TargetChildWorkflowOnly: true,
			InitiatedID:             scheduleID,
		},
		&p.StartChildExecutionTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              currentTransferID + 10006,
				Version:             666,
			},
			TargetDomainID:   targetDomainID,
			TargetWorkflowID: targetWorkflowID,
			InitiatedID:      scheduleID,
		},
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: scheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpdateWorklowStateAndReplication(ctx, updatedInfo, updatedStats, versionHistories, int64(3), tasks)
	s.NoError(err2)

	txTasks, err1 := s.GetTransferTasks(ctx, 2, true) // use page size one to force pagination
	s.NoError(err1)
	s.NotNil(txTasks, "expected valid list of tasks.")
	s.Equal(len(tasks), len(txTasks))
	for index := range tasks {
		s.True(timeComparator(tasks[index].GetVisibilityTimestamp(), txTasks[index].GetVisibilityTimestamp(), TimePrecision))
	}
	s.Equal(p.TransferTaskTypeActivityTask, txTasks[0].GetTaskType())
	s.Equal(p.TransferTaskTypeDecisionTask, txTasks[1].GetTaskType())
	s.Equal(p.TransferTaskTypeCloseExecution, txTasks[2].GetTaskType())
	s.Equal(p.TransferTaskTypeCancelExecution, txTasks[3].GetTaskType())
	s.Equal(p.TransferTaskTypeSignalExecution, txTasks[4].GetTaskType())
	s.Equal(p.TransferTaskTypeStartChildExecution, txTasks[5].GetTaskType())
	s.Equal(int64(111), txTasks[0].GetVersion())
	s.Equal(int64(222), txTasks[1].GetVersion())
	s.Equal(int64(333), txTasks[2].GetVersion())
	s.Equal(int64(444), txTasks[3].GetVersion())
	s.Equal(int64(555), txTasks[4].GetVersion())
	s.Equal(int64(666), txTasks[5].GetVersion())
	s.Equal(currentTransferID+10001, txTasks[0].GetTaskID())
	s.Equal(currentTransferID+10002, txTasks[1].GetTaskID())
	s.Equal(currentTransferID+10003, txTasks[2].GetTaskID())
	s.Equal(currentTransferID+10004, txTasks[3].GetTaskID())
	s.Equal(currentTransferID+10005, txTasks[4].GetTaskID())
	s.Equal(currentTransferID+10006, txTasks[5].GetTaskID())

	err2 = s.RangeCompleteTransferTask(ctx, txTasks[0].GetTaskID(), txTasks[5].GetTaskID()+1)
	s.NoError(err2)

	txTasks, err2 = s.GetTransferTasks(ctx, 100, false)
	s.NoError(err2)
	s.Empty(txTasks, "expected empty task list.")
}

// TestTimerTasksComplete test
func (s *ExecutionManagerSuite) TestTimerTasksComplete() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "8bfb47be-5b57-4d66-9109-5fb35e20b1d7"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "get-timer-tasks-test-complete",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}

	now := time.Now()
	initialTasks := []p.Task{&p.DecisionTimeoutTask{
		WorkflowIdentifier: p.WorkflowIdentifier{
			DomainID:   domainID,
			WorkflowID: workflowExecution.WorkflowID,
			RunID:      workflowExecution.RunID,
		},
		TaskData: p.TaskData{
			VisibilityTimestamp: now.Add(1 * time.Second),
			TaskID:              1,
			Version:             11,
		},
		EventID:         2,
		ScheduleAttempt: 3,
		TimeoutType:     int(types.TimeoutTypeStartToClose)},
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "taskList", "wType", 20, 13, nil, 3, 0, 2, initialTasks, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	tasks := []p.Task{
		&p.WorkflowTimeoutTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				Version:             12,
				TaskID:              2,
				VisibilityTimestamp: now.Add(2 * time.Second),
			},
		},
		&p.DeleteHistoryEventTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				Version:             13,
				TaskID:              3,
				VisibilityTimestamp: now.Add(2 * time.Second),
			},
		},
		&p.ActivityTimeoutTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				Version:             14,
				TaskID:              4,
				VisibilityTimestamp: now.Add(3 * time.Second),
			},
			TimeoutType: int(types.TimeoutTypeStartToClose),
			EventID:     7,
			Attempt:     0,
		},
		&p.UserTimerTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				Version:             15,
				TaskID:              5,
				VisibilityTimestamp: now.Add(3 * time.Second),
			},
			EventID: 7,
		},
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.NextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, []int64{int64(4)}, nil, int64(3), tasks, nil, nil, nil, nil)
	s.NoError(err2)

	timerTasks, err1 := s.GetTimerIndexTasks(ctx, 1, true) // use page size one to force pagination
	s.NoError(err1)
	s.NotNil(timerTasks, "expected valid list of tasks.")
	s.Equal(len(tasks)+len(initialTasks), len(timerTasks))
	s.Equal(p.TaskTypeDecisionTimeout, timerTasks[0].GetTaskType())
	s.Equal(p.TaskTypeWorkflowTimeout, timerTasks[1].GetTaskType())
	s.Equal(p.TaskTypeDeleteHistoryEvent, timerTasks[2].GetTaskType())
	s.Equal(p.TaskTypeActivityTimeout, timerTasks[3].GetTaskType())
	s.Equal(p.TaskTypeUserTimer, timerTasks[4].GetTaskType())
	s.Equal(int64(11), timerTasks[0].GetVersion())
	s.Equal(int64(12), timerTasks[1].GetVersion())
	s.Equal(int64(13), timerTasks[2].GetVersion())
	s.Equal(int64(14), timerTasks[3].GetVersion())
	s.Equal(int64(15), timerTasks[4].GetVersion())

	err2 = s.RangeCompleteTimerTask(ctx, timerTasks[0].GetVisibilityTimestamp(), timerTasks[4].GetVisibilityTimestamp().Add(1*time.Second))
	s.NoError(err2)

	timerTasks2, err2 := s.GetTimerIndexTasks(ctx, 100, false)
	s.NoError(err2)
	s.Empty(timerTasks2, "expected empty task list.")
}

// TestTimerTasksRangeComplete test
func (s *ExecutionManagerSuite) TestTimerTasksRangeComplete() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "8bfb47be-5b57-4d66-9109-5fb35e20b1d7"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "get-timer-tasks-test-range-complete",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "taskList", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.NextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	now := time.Now().UTC()
	tasks := []p.Task{
		&p.DecisionTimeoutTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              1,
				Version:             11,
			},
			EventID:         2,
			ScheduleAttempt: 3,
			TimeoutType:     int(types.TimeoutTypeStartToClose),
		},
		&p.WorkflowTimeoutTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              2,
				Version:             12,
			},
		},
		&p.DeleteHistoryEventTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              3,
				Version:             13,
			},
		},
		&p.ActivityTimeoutTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              4,
				Version:             14,
			},
			TimeoutType: int(types.TimeoutTypeStartToClose),
			EventID:     7,
			Attempt:     0,
		},
		&p.UserTimerTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				VisibilityTimestamp: now,
				TaskID:              5,
				Version:             15,
			},
			EventID: 7,
		},
	}
	err2 := s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, []int64{int64(4)}, nil, int64(3), tasks, nil, nil, nil, nil)
	s.NoError(err2)

	timerTasks, err1 := s.GetTimerIndexTasks(ctx, 1, true) // use page size one to force pagination
	s.NoError(err1)
	s.NotNil(timerTasks, "expected valid list of tasks.")
	s.Equal(len(tasks), len(timerTasks))
	s.Equal(p.TaskTypeDecisionTimeout, timerTasks[0].GetTaskType())
	s.Equal(p.TaskTypeWorkflowTimeout, timerTasks[1].GetTaskType())
	s.Equal(p.TaskTypeDeleteHistoryEvent, timerTasks[2].GetTaskType())
	s.Equal(p.TaskTypeActivityTimeout, timerTasks[3].GetTaskType())
	s.Equal(p.TaskTypeUserTimer, timerTasks[4].GetTaskType())
	s.Equal(int64(11), timerTasks[0].GetVersion())
	s.Equal(int64(12), timerTasks[1].GetVersion())
	s.Equal(int64(13), timerTasks[2].GetVersion())
	s.Equal(int64(14), timerTasks[3].GetVersion())
	s.Equal(int64(15), timerTasks[4].GetVersion())

	err2 = s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, nil, nil, int64(5), nil, nil, nil, nil, nil)
	s.NoError(err2)

	err2 = s.CompleteTimerTask(ctx, timerTasks[0].GetVisibilityTimestamp(), timerTasks[0].GetTaskID())
	s.NoError(err2)

	err2 = s.CompleteTimerTask(ctx, timerTasks[1].GetVisibilityTimestamp(), timerTasks[1].GetTaskID())
	s.NoError(err2)

	err2 = s.CompleteTimerTask(ctx, timerTasks[2].GetVisibilityTimestamp(), timerTasks[2].GetTaskID())
	s.NoError(err2)

	err2 = s.CompleteTimerTask(ctx, timerTasks[3].GetVisibilityTimestamp(), timerTasks[3].GetTaskID())
	s.NoError(err2)

	err2 = s.CompleteTimerTask(ctx, timerTasks[4].GetVisibilityTimestamp(), timerTasks[4].GetTaskID())
	s.NoError(err2)

	timerTasks2, err2 := s.GetTimerIndexTasks(ctx, 100, false)
	s.NoError(err2)
	s.Empty(timerTasks2, "expected empty task list.")
}

// TestWorkflowMutableStateActivities test
func (s *ExecutionManagerSuite) TestWorkflowMutableStateActivities() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "7fcf0aa9-e121-4292-bdad-0a75181b4aa3"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-workflow-mutable-test",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "taskList", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	activityInfos := []*p.ActivityInfo{{
		Version:                  7789,
		ScheduleID:               1,
		ScheduledEventBatchID:    1,
		ScheduledEvent:           &types.HistoryEvent{ID: 1},
		ScheduledTime:            time.UnixMilli(1755030646000).UTC(),
		ActivityID:               uuid.New(),
		RequestID:                uuid.New(),
		Details:                  []byte(uuid.New()),
		StartedID:                2,
		StartedEvent:             &types.HistoryEvent{ID: 2},
		StartedTime:              time.UnixMilli(1755030646001).UTC(),
		ScheduleToCloseTimeout:   1,
		ScheduleToStartTimeout:   2,
		StartToCloseTimeout:      3,
		HeartbeatTimeout:         4,
		LastHeartBeatUpdatedTime: time.UnixMilli(1755030646002).UTC(),
		TimerTaskStatus:          1,
		CancelRequested:          true,
		CancelRequestID:          math.MaxInt64,
		Attempt:                  math.MaxInt32,
		DomainID:                 domainID,
		StartedIdentity:          uuid.New(),
		TaskList:                 uuid.New(),
		TaskListKind:             types.TaskListKindEphemeral,
		HasRetryPolicy:           true,
		InitialInterval:          math.MaxInt32,
		MaximumInterval:          math.MaxInt32,
		MaximumAttempts:          math.MaxInt32,
		BackoffCoefficient:       5.55,
		ExpirationTime:           time.UnixMilli(1755030646003).UTC(),
		NonRetriableErrors:       []string{"accessDenied", "badRequest"},
		LastFailureReason:        "some random error",
		LastWorkerIdentity:       uuid.New(),
		LastFailureDetails:       []byte(uuid.New()),
	}}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.NextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, []int64{int64(4)}, nil, int64(3), nil, activityInfos, nil, nil, nil)
	s.NoError(err2)

	state, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state, "expected valid state.")
	s.Equal(1, len(state.ActivityInfos))
	s.T().Logf("%+v\n", state.ActivityInfos)
	ai, ok := state.ActivityInfos[1]
	s.True(ok)
	s.NotNil(ai)
	// Various persistence layers are inconsistent with assigning the location value, so normalize them first
	ai.ScheduledTime = ai.ScheduledTime.UTC()
	ai.StartedTime = ai.StartedTime.UTC()
	ai.LastHeartBeatUpdatedTime = ai.LastHeartBeatUpdatedTime.UTC()
	ai.ExpirationTime = ai.ExpirationTime.UTC()
	s.Equal(activityInfos[0], ai)

	err2 = s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, nil, nil, int64(5), nil, nil, []int64{1}, nil, nil)
	s.NoError(err2)

	state, err2 = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.NotNil(state, "expected valid state.")
	s.Equal(0, len(state.ActivityInfos))
}

// TestWorkflowMutableStateTimers test
func (s *ExecutionManagerSuite) TestWorkflowMutableStateTimers() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "025d178a-709b-4c07-8dd7-86dbf9bd2e06"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-workflow-mutable-timers-test",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "taskList", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	currentTime := time.Now().UTC()
	timerID := "id_1"
	timerInfos := []*p.TimerInfo{{
		Version:    3345,
		TimerID:    timerID,
		ExpiryTime: currentTime,
		TaskStatus: 2,
		StartedID:  5,
	}}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.NextEventID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, []int64{int64(4)}, nil, int64(3), nil, nil, nil, timerInfos, nil)
	s.NoError(err2)

	state, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state, "expected valid state.")
	s.Equal(1, len(state.TimerInfos))
	s.Equal(int64(3345), state.TimerInfos[timerID].Version)
	s.Equal(timerID, state.TimerInfos[timerID].TimerID)
	s.EqualTimesWithPrecision(currentTime, state.TimerInfos[timerID].ExpiryTime, time.Millisecond*500)
	s.Equal(int64(2), state.TimerInfos[timerID].TaskStatus)
	s.Equal(int64(5), state.TimerInfos[timerID].StartedID)

	err2 = s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, nil, nil, int64(5), nil, nil, nil, nil, []string{timerID})
	s.NoError(err2)

	state, err2 = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.NotNil(state, "expected valid state.")
	s.Equal(0, len(state.TimerInfos))
}

// TestWorkflowMutableStateChildExecutions test
func (s *ExecutionManagerSuite) TestWorkflowMutableStateChildExecutions() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "88236cd2-c439-4cec-9957-2748ce3be074"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-workflow-mutable-child-executions-parent-test",
		RunID:      "c63dba1e-929c-4fbf-8ec5-4533b16269a9",
	}

	parentDomainID := "6036ded3-e541-42c9-8f69-3d9354dad081"
	parentExecution := types.WorkflowExecution{
		WorkflowID: "test-workflow-mutable-child-executions-child-test",
		RunID:      "73e89362-25ec-4305-bcb8-d9448b90856c",
	}

	partitionConfig := map[string]string{
		"userID": uuid.New(),
	}
	task0, err0 := s.CreateChildWorkflowExecution(ctx, domainID, workflowExecution, parentDomainID, parentExecution, 1, "taskList", "wType", 20, 13, nil, 3, 0, 2, nil, partitionConfig)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")
	s.Equal(parentDomainID, info0.ParentDomainID)
	s.Equal(parentExecution.GetWorkflowID(), info0.ParentWorkflowID)
	s.Equal(parentExecution.GetRunID(), info0.ParentRunID)
	s.Equal(int64(1), info0.InitiatedID)
	s.Equal(partitionConfig, info0.PartitionConfig)

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	childExecutionInfos := []*p.ChildExecutionInfo{{
		Version:           1234,
		InitiatedID:       1,
		InitiatedEvent:    &types.HistoryEvent{ID: 1},
		StartedID:         2,
		StartedRunID:      uuid.New(),
		StartedEvent:      &types.HistoryEvent{ID: 2},
		CreateRequestID:   uuid.New(),
		DomainID:          uuid.New(),
		ParentClosePolicy: types.ParentClosePolicyTerminate,
	}}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpsertChildExecutionsState(ctx, updatedInfo, updatedStats, versionHistories, int64(3), childExecutionInfos)
	s.NoError(err2)

	state, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state, "expected valid state.")
	s.Equal(1, len(state.ChildExecutionInfos))
	ci, ok := state.ChildExecutionInfos[1]
	s.True(ok)
	s.NotNil(ci)
	s.Equal(childExecutionInfos[0], ci)

	err2 = s.DeleteChildExecutionsState(ctx, updatedInfo, updatedStats, versionHistories, int64(5), int64(1))
	s.NoError(err2)

	state, err2 = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.NotNil(state, "expected valid state.")
	s.Equal(0, len(state.ChildExecutionInfos))
}

// TestWorkflowMutableStateRequestCancel test
func (s *ExecutionManagerSuite) TestWorkflowMutableStateRequestCancel() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "568b8d19-cf64-4aac-be1b-f8a3edbc1fa9"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-workflow-mutable-request-cancel-test",
		RunID:      "87f96253-b925-426e-90db-aa4ee89b5aca",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "taskList", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	requestCancelInfo := &p.RequestCancelInfo{
		Version:               456,
		InitiatedID:           2,
		InitiatedEventBatchID: 1,
		CancelRequestID:       uuid.New(),
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpsertRequestCancelState(ctx, updatedInfo, updatedStats, versionHistories, int64(3), []*p.RequestCancelInfo{requestCancelInfo})
	s.NoError(err2)

	state, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state, "expected valid state.")
	s.Equal(1, len(state.RequestCancelInfos))
	ri, ok := state.RequestCancelInfos[requestCancelInfo.InitiatedID]
	s.True(ok)
	s.NotNil(ri)
	s.Equal(requestCancelInfo, ri)

	err2 = s.DeleteCancelState(ctx, updatedInfo, updatedStats, versionHistories, int64(5), requestCancelInfo.InitiatedID)
	s.NoError(err2)

	state, err2 = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.NotNil(state, "expected valid state.")
	s.Equal(0, len(state.RequestCancelInfos))
}

// TestWorkflowMutableStateSignalInfo test
func (s *ExecutionManagerSuite) TestWorkflowMutableStateSignalInfo() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	runID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-workflow-mutable-signal-info-test",
		RunID:      runID,
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "taskList", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	signalInfo := &p.SignalInfo{
		Version:               123,
		InitiatedID:           2,
		InitiatedEventBatchID: 1,
		SignalRequestID:       uuid.New(),
		SignalName:            "my signal",
		Input:                 []byte("test signal input"),
		Control:               []byte(uuid.New()),
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpsertSignalInfoState(ctx, updatedInfo, updatedStats, versionHistories, int64(3), []*p.SignalInfo{signalInfo})
	s.NoError(err2)

	state, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state, "expected valid state.")
	s.Equal(1, len(state.SignalInfos))
	si, ok := state.SignalInfos[signalInfo.InitiatedID]
	s.True(ok)
	s.NotNil(si)
	s.Equal(signalInfo, si)

	err2 = s.DeleteSignalState(ctx, updatedInfo, updatedStats, versionHistories, int64(5), signalInfo.InitiatedID)
	s.NoError(err2)

	state, err2 = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.NotNil(state, "expected valid state.")
	s.Equal(0, len(state.SignalInfos))
}

// TestWorkflowMutableStateSignalRequested test
func (s *ExecutionManagerSuite) TestWorkflowMutableStateSignalRequested() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	runID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-workflow-mutable-signal-requested-test",
		RunID:      runID,
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "taskList", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	signalRequestedID := uuid.New()
	signalsRequested := []string{signalRequestedID}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpsertSignalsRequestedState(ctx, updatedInfo, updatedStats, versionHistories, int64(3), signalsRequested)
	s.NoError(err2)

	state, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state, "expected valid state.")
	s.Equal(1, len(state.SignalRequestedIDs))
	ri, ok := state.SignalRequestedIDs[signalRequestedID]
	s.True(ok)
	s.NotNil(ri)

	err2 = s.DeleteSignalsRequestedState(ctx, updatedInfo, updatedStats, versionHistories, int64(5), []string{signalRequestedID, uuid.New()})
	s.NoError(err2)

	state, err2 = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.NotNil(state, "expected valid state.")
	s.Equal(0, len(state.SignalRequestedIDs))
}

// TestWorkflowMutableStateInfo test
func (s *ExecutionManagerSuite) TestWorkflowMutableStateInfo() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "9ed8818b-3090-4160-9f21-c6b70e64d2dd"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-workflow-mutable-state-test",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "taskList", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err2 := s.UpdateWorkflowExecution(ctx, updatedInfo, updatedStats, versionHistories, []int64{int64(4)}, nil, int64(3), nil, nil, nil, nil, nil)
	s.NoError(err2)

	state, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state, "expected valid state.")
	s.NotNil(state.ExecutionInfo, "expected valid MS Info state.")
	s.Equal(updatedInfo.NextEventID, state.ExecutionInfo.NextEventID)
	s.Equal(updatedInfo.State, state.ExecutionInfo.State)
}

// TestContinueAsNew test
func (s *ExecutionManagerSuite) TestContinueAsNew() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "c1c0bb55-04e6-4a9c-89d0-1be7b96459f8"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "continue-as-new-workflow-test",
		RunID:      "551c88d2-d9e6-404f-8131-9eec14f36643",
	}

	partitionConfig := map[string]string{
		"userID": uuid.New(),
	}
	_, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, partitionConfig)
	s.NoError(err0)

	state0, err1 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	continueAsNewInfo := copyWorkflowExecutionInfo(info0)
	continueAsNewStats := copyExecutionStats(state0.ExecutionStats)
	continueAsNewInfo.State = p.WorkflowStateCreated
	continueAsNewInfo.CloseStatus = p.WorkflowCloseStatusNone
	continueAsNewInfo.NextEventID = int64(5)
	continueAsNewInfo.LastProcessedEvent = int64(2)

	newWorkflowExecution := types.WorkflowExecution{
		WorkflowID: "continue-as-new-workflow-test",
		RunID:      "64c7e15a-3fd7-4182-9c6f-6f25a4fa2614",
	}

	testResetPoints := types.ResetPoints{
		Points: []*types.ResetPointInfo{
			{
				BinaryChecksum:           "test-binary-checksum",
				RunID:                    "test-runID",
				FirstDecisionCompletedID: 123,
				CreatedTimeNano:          common.Int64Ptr(456),
				Resettable:               true,
				ExpiringTimeNano:         common.Int64Ptr(789),
			},
		},
	}

	err2 := s.ContinueAsNewExecution(ctx, continueAsNewInfo, continueAsNewStats, info0.NextEventID, newWorkflowExecution, int64(3), int64(2), &testResetPoints)
	s.NoError(err2)

	prevExecutionState, err3 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err3)
	prevExecutionInfo := prevExecutionState.ExecutionInfo
	s.Equal("551c88d2-d9e6-404f-8131-9eec14f36643", prevExecutionInfo.FirstExecutionRunID)
	s.Equal(p.WorkflowStateCompleted, prevExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusContinuedAsNew, prevExecutionInfo.CloseStatus)
	s.Equal(int64(5), prevExecutionInfo.NextEventID)
	s.Equal(int64(2), prevExecutionInfo.LastProcessedEvent)
	s.Equal(prevExecutionInfo.AutoResetPoints, &types.ResetPoints{})
	s.Equal(partitionConfig, prevExecutionInfo.PartitionConfig)

	newExecutionState, err4 := s.GetWorkflowExecutionInfo(ctx, domainID, newWorkflowExecution)
	s.NoError(err4)
	newExecutionInfo := newExecutionState.ExecutionInfo
	s.Equal("551c88d2-d9e6-404f-8131-9eec14f36643", newExecutionInfo.FirstExecutionRunID)
	s.Equal(p.WorkflowStateCreated, newExecutionInfo.State)
	s.Equal(p.WorkflowCloseStatusNone, newExecutionInfo.CloseStatus)
	s.Equal(int64(3), newExecutionInfo.NextEventID)
	s.Equal(constants.EmptyEventID, newExecutionInfo.LastProcessedEvent)
	s.Equal(int64(2), newExecutionInfo.DecisionScheduleID)
	s.Equal(testResetPoints, *newExecutionInfo.AutoResetPoints)
	s.Equal(partitionConfig, newExecutionInfo.PartitionConfig)

	newRunID, err5 := s.GetCurrentWorkflowRunID(ctx, domainID, workflowExecution.GetWorkflowID())
	s.NoError(err5)
	s.Equal(newWorkflowExecution.GetRunID(), newRunID)
}

// TestReplicationTransferTaskTasks test
func (s *ExecutionManagerSuite) TestReplicationTransferTaskTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "2466d7de-6602-4ad8-b939-fb8f8c36c711"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "replication-transfer-task-test",
		RunID:      "dcde9d85-5d7a-43c7-8b18-cb2cae0e29e0",
	}

	task0, err := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err)
	s.NotNil(task0, "Expected non empty task identifier.")

	taskD, err := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err)
	s.Equal(1, len(taskD), "Expected 1 decision task.")
	err = s.CompleteTransferTask(ctx, taskD[0].GetTaskID())
	s.NoError(err)

	state1, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err)
	info1 := state1.ExecutionInfo
	s.NotNil(info1, "Valid Workflow info expected.")
	updatedInfo1 := copyWorkflowExecutionInfo(info1)
	updatedStats1 := copyExecutionStats(state1.ExecutionStats)

	replicationTasks := []p.Task{&p.HistoryReplicationTask{
		WorkflowIdentifier: p.WorkflowIdentifier{
			DomainID:   domainID,
			WorkflowID: workflowExecution.WorkflowID,
			RunID:      workflowExecution.RunID,
		},
		TaskData: p.TaskData{
			TaskID:  s.GetNextSequenceNumber(),
			Version: 9,
		},
		FirstEventID: 1,
		NextEventID:  3,
	}}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: 3,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err = s.UpdateWorklowStateAndReplication(ctx, updatedInfo1, updatedStats1, versionHistories, int64(3), replicationTasks)
	s.NoError(err)

	tasks1, err := s.GetReplicationTasks(ctx, 1, false)
	s.NoError(err)
	s.NotNil(tasks1, "expected valid list of tasks.")
	s.Equal(1, len(tasks1), "Expected 1 replication task.")
	task1, ok := tasks1[0].(*p.HistoryReplicationTask)
	s.True(ok, "expected a history replication task")
	s.Equal(domainID, task1.DomainID)
	s.Equal(workflowExecution.GetWorkflowID(), task1.WorkflowID)
	s.Equal(workflowExecution.GetRunID(), task1.RunID)
	s.Equal(int64(1), task1.FirstEventID)
	s.Equal(int64(3), task1.NextEventID)
	s.Equal(int64(9), task1.Version)

	err = s.CompleteReplicationTask(ctx, task1.TaskID)
	s.NoError(err)
	tasks2, err := s.GetReplicationTasks(ctx, 1, false)
	s.NoError(err)
	s.Equal(0, len(tasks2))
}

// TestReplicationTransferTaskRangeComplete test
func (s *ExecutionManagerSuite) TestReplicationTransferTaskRangeComplete() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "replication-transfer-task--range-complete-test",
		RunID:      uuid.New(),
	}

	task0, err := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "queue1", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err)
	s.NotNil(task0, "Expected non empty task identifier.")

	taskD, err := s.GetTransferTasks(ctx, 1, false)
	s.NoError(err)
	s.Equal(1, len(taskD), "Expected 1 decision task.")
	err = s.CompleteTransferTask(ctx, taskD[0].GetTaskID())
	s.NoError(err)

	state1, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err)
	info1 := state1.ExecutionInfo
	s.NotNil(info1, "Valid Workflow info expected.")
	updatedInfo1 := copyWorkflowExecutionInfo(info1)
	updatedStats1 := copyExecutionStats(state1.ExecutionStats)

	replicationTasks := []p.Task{
		&p.HistoryReplicationTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				TaskID:  s.GetNextSequenceNumber(),
				Version: 9,
			},
			FirstEventID: 1,
			NextEventID:  3,
		},
		&p.HistoryReplicationTask{
			WorkflowIdentifier: p.WorkflowIdentifier{
				DomainID:   domainID,
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			TaskData: p.TaskData{
				TaskID:  s.GetNextSequenceNumber(),
				Version: 9,
			},
			FirstEventID: 4,
			NextEventID:  5,
		},
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: 3,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err = s.UpdateWorklowStateAndReplication(ctx,
		updatedInfo1,
		updatedStats1,
		versionHistories,
		int64(3),
		replicationTasks,
	)
	s.NoError(err)

	tasks1, err := s.GetReplicationTasks(ctx, 2, false)
	s.NoError(err)
	s.NotNil(tasks1, "expected valid list of tasks.")
	s.Equal(2, len(tasks1), "Expected 2 replication tasks.")
	task1, ok := tasks1[0].(*p.HistoryReplicationTask)
	s.True(ok, "expected a history replication task")
	s.Equal(domainID, task1.DomainID)
	s.Equal(workflowExecution.GetWorkflowID(), task1.WorkflowID)
	s.Equal(workflowExecution.GetRunID(), task1.RunID)
	s.Equal(int64(1), task1.FirstEventID)
	s.Equal(int64(3), task1.NextEventID)
	s.Equal(int64(9), task1.Version)

	task2, ok := tasks1[1].(*p.HistoryReplicationTask)
	s.True(ok, "expected a history replication task")
	s.Equal(domainID, task2.DomainID)
	s.Equal(workflowExecution.GetWorkflowID(), task2.WorkflowID)
	s.Equal(workflowExecution.GetRunID(), task2.RunID)
	s.Equal(int64(4), task2.FirstEventID)
	s.Equal(int64(5), task2.NextEventID)
	s.Equal(int64(9), task2.Version)
	err = s.RangeCompleteReplicationTask(ctx, task2.TaskID+1)
	s.NoError(err)
	tasks2, err := s.GetReplicationTasks(ctx, 1, false)
	s.NoError(err)
	s.Equal(0, len(tasks2))
}

// TestUpdateAndClearBufferedEvents test
func (s *ExecutionManagerSuite) TestUpdateAndClearBufferedEvents() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "4ca1faac-1a3a-47af-8e51-fdaa2b3d45b9"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-update-and-clear-buffered-events",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}

	task0, err0 := s.CreateWorkflowExecution(ctx, domainID, workflowExecution, "taskList", "wType", 20, 13, nil, 3, 0, 2, nil, nil)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	stats0, state0, err1 := s.GetWorkflowExecutionInfoWithStats(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")
	s.Equal(0, stats0.BufferedEventsCount)
	s.Equal(0, stats0.BufferedEventsSize)

	eventsBatch1 := []*types.HistoryEvent{
		{
			ID:        5,
			EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
			Version:   11,
			DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				ScheduledEventID: 2,
				StartedEventID:   3,
				Identity:         "test_worker",
			},
		},
		{
			ID:        6,
			EventType: types.EventTypeTimerStarted.Ptr(),
			Version:   11,
			TimerStartedEventAttributes: &types.TimerStartedEventAttributes{
				TimerID:                      "ID1",
				StartToFireTimeoutSeconds:    common.Int64Ptr(101),
				DecisionTaskCompletedEventID: 5,
			},
		},
	}

	eventsBatch2 := []*types.HistoryEvent{
		{
			ID:        21,
			EventType: types.EventTypeTimerFired.Ptr(),
			Version:   12,
			TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
				TimerID:        "2",
				StartedEventID: 3,
			},
		},
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: eventsBatch2[0].ID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedState := &p.WorkflowMutableState{
		ExecutionInfo:    updatedInfo,
		ExecutionStats:   updatedStats,
		VersionHistories: versionHistories,
	}

	err2 := s.UpdateAllMutableState(ctx, updatedState, int64(3))
	s.NoError(err2)

	partialState, err2 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.NotNil(partialState, "expected valid state.")
	partialInfo := partialState.ExecutionInfo
	s.NotNil(partialInfo, "Valid Workflow info expected.")

	bufferUpdateInfo := copyWorkflowExecutionInfo(partialInfo)
	bufferedUpdatedStats := copyExecutionStats(state0.ExecutionStats)
	err2 = s.UpdateWorklowStateAndReplication(ctx, bufferUpdateInfo, bufferedUpdatedStats, versionHistories, bufferUpdateInfo.NextEventID, nil)
	s.NoError(err2)
	err2 = s.UpdateWorklowStateAndReplication(ctx, bufferUpdateInfo, bufferedUpdatedStats, versionHistories, bufferUpdateInfo.NextEventID, nil)
	s.NoError(err2)
	err2 = s.UpdateWorkflowExecutionForBufferEvents(ctx, bufferUpdateInfo, bufferedUpdatedStats, bufferUpdateInfo.NextEventID, eventsBatch1, false, versionHistories)
	s.NoError(err2)
	stats0, state0, err2 = s.GetWorkflowExecutionInfoWithStats(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.Equal(1, stats0.BufferedEventsCount)
	s.True(stats0.BufferedEventsSize > 0)
	testHistory := &types.History{Events: make([]*types.HistoryEvent, 0)}
	testHistory.Events = append(testHistory.Events, eventsBatch1...)
	history0 := &types.History{Events: state0.BufferedEvents}
	s.Equal(testHistory, history0)
	testHistory.Events = append(testHistory.Events, eventsBatch2...)

	err2 = s.UpdateWorkflowExecutionForBufferEvents(ctx, bufferUpdateInfo, bufferedUpdatedStats, bufferUpdateInfo.NextEventID, eventsBatch2, false, versionHistories)
	s.NoError(err2)

	stats1, state1, err1 := s.GetWorkflowExecutionInfoWithStats(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state1, "expected valid state.")
	info1 := state1.ExecutionInfo
	s.NotNil(info1, "Valid Workflow info expected.")
	s.Equal(2, stats1.BufferedEventsCount)
	s.True(stats1.BufferedEventsSize > 0)
	history1 := &types.History{Events: state1.BufferedEvents}
	s.Equal(testHistory, history1)

	err3 := s.UpdateWorkflowExecutionForBufferEvents(ctx, bufferUpdateInfo, bufferedUpdatedStats, bufferUpdateInfo.NextEventID, nil, true, versionHistories)
	s.NoError(err3)

	stats3, state3, err3 := s.GetWorkflowExecutionInfoWithStats(ctx, domainID, workflowExecution)
	s.NoError(err3)
	s.NotNil(state3, "expected valid state.")
	info3 := state3.ExecutionInfo
	s.NotNil(info3, "Valid Workflow info expected.")
	s.Equal(0, stats3.BufferedEventsCount)
	s.Equal(0, stats3.BufferedEventsSize)
}

// TestConflictResolveWorkflowExecutionCurrentIsSelf test
func (s *ExecutionManagerSuite) TestConflictResolveWorkflowExecutionCurrentIsSelf() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "4ca1faac-1a3a-47af-8e51-fdaa2b3d45b9"
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-reset-mutable-state-test-current-is-self",
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}
	partitionConfig := map[string]string{
		"userid": uuid.New(),
	}
	task0, err0 := s.CreateWorkflowExecution(
		ctx,
		domainID,
		workflowExecution,
		"taskList",
		"wType",
		20,
		13,
		nil,
		3,
		0,
		2,
		nil,
		partitionConfig,
	)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	stats0, state0, err1 := s.GetWorkflowExecutionInfoWithStats(ctx, domainID, workflowExecution)
	s.NoError(err1)
	info0 := state0.ExecutionInfo
	s.NotNil(info0, "Valid Workflow info expected.")
	s.Equal(0, stats0.BufferedEventsCount)
	s.Equal(0, stats0.BufferedEventsSize)

	updatedInfo := copyWorkflowExecutionInfo(info0)
	updatedStats := copyExecutionStats(state0.ExecutionStats)
	updatedInfo.NextEventID = int64(5)
	updatedInfo.LastProcessedEvent = int64(2)
	currentTime := time.Now().UTC()
	expiryTime := currentTime.Add(10 * time.Second)
	eventsBatch1 := []*types.HistoryEvent{
		{
			ID:        5,
			EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
			Version:   11,
			DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				ScheduledEventID: 2,
				StartedEventID:   3,
				Identity:         "test_worker",
			},
		},
		{
			ID:        6,
			EventType: types.EventTypeTimerStarted.Ptr(),
			Version:   11,
			TimerStartedEventAttributes: &types.TimerStartedEventAttributes{
				TimerID:                      "ID1",
				StartToFireTimeoutSeconds:    common.Int64Ptr(101),
				DecisionTaskCompletedEventID: 5,
			},
		},
	}

	eventsBatch2 := []*types.HistoryEvent{
		{
			ID:        21,
			EventType: types.EventTypeTimerFired.Ptr(),
			Version:   12,
			TimerFiredEventAttributes: &types.TimerFiredEventAttributes{
				TimerID:        "2",
				StartedEventID: 3,
			},
		},
	}

	csum := s.newRandomChecksum()

	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: updatedInfo.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	updatedState := &p.WorkflowMutableState{
		ExecutionInfo:  updatedInfo,
		ExecutionStats: updatedStats,
		ActivityInfos: map[int64]*p.ActivityInfo{
			4: {
				Version:                  7789,
				ScheduleID:               4,
				ScheduledEventBatchID:    3,
				ScheduledEvent:           &types.HistoryEvent{ID: 40},
				ScheduledTime:            currentTime,
				StartedID:                6,
				StartedEvent:             &types.HistoryEvent{ID: 60},
				StartedTime:              currentTime,
				ScheduleToCloseTimeout:   1,
				ScheduleToStartTimeout:   2,
				StartToCloseTimeout:      3,
				HeartbeatTimeout:         4,
				LastHeartBeatUpdatedTime: currentTime,
				TimerTaskStatus:          1,
			},
			5: {
				Version:                  7789,
				ScheduleID:               5,
				ScheduledEventBatchID:    3,
				ScheduledEvent:           &types.HistoryEvent{ID: 50},
				ScheduledTime:            currentTime,
				StartedID:                7,
				StartedEvent:             &types.HistoryEvent{ID: 70},
				StartedTime:              currentTime,
				ScheduleToCloseTimeout:   1,
				ScheduleToStartTimeout:   2,
				StartToCloseTimeout:      3,
				HeartbeatTimeout:         4,
				LastHeartBeatUpdatedTime: currentTime,
				TimerTaskStatus:          1,
			}},

		TimerInfos: map[string]*p.TimerInfo{
			"t1": {
				Version:    2333,
				TimerID:    "t1",
				StartedID:  1,
				ExpiryTime: expiryTime,
				TaskStatus: 500,
			},
			"t2": {
				Version:    2333,
				TimerID:    "t2",
				StartedID:  2,
				ExpiryTime: expiryTime,
				TaskStatus: 501,
			},
			"t3": {
				Version:    2333,
				TimerID:    "t3",
				StartedID:  3,
				ExpiryTime: expiryTime,
				TaskStatus: 502,
			},
		},

		ChildExecutionInfos: map[int64]*p.ChildExecutionInfo{
			9: {
				Version:         2334,
				InitiatedID:     9,
				InitiatedEvent:  &types.HistoryEvent{ID: 123},
				StartedID:       11,
				StartedRunID:    uuid.New(),
				StartedEvent:    nil,
				CreateRequestID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				DomainID:        uuid.New(),
			},
		},

		RequestCancelInfos: map[int64]*p.RequestCancelInfo{
			19: {
				Version:               2335,
				InitiatedID:           19,
				InitiatedEventBatchID: 17,
				CancelRequestID:       "cancel_requested_id",
			},
		},

		SignalInfos: map[int64]*p.SignalInfo{
			39: {
				Version:               2336,
				InitiatedID:           39,
				InitiatedEventBatchID: 38,
				SignalRequestID:       "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				SignalName:            "signalA",
				Input:                 []byte("signal_input_A"),
				Control:               []byte("signal_control_A"),
			},
		},

		SignalRequestedIDs: map[string]struct{}{
			"00000000-0000-0000-0000-000000000001": {},
			"00000000-0000-0000-0000-000000000002": {},
			"00000000-0000-0000-0000-000000000003": {},
		},
		Checksum:         csum,
		VersionHistories: versionHistories,
	}

	err2 := s.UpdateAllMutableState(ctx, updatedState, int64(3))
	s.NoError(err2)

	partialState, err2 := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.NotNil(partialState, "expected valid state.")
	partialInfo := partialState.ExecutionInfo
	s.NotNil(partialInfo, "Valid Workflow info expected.")
	s.assertChecksumsEqual(csum, partialState.Checksum)

	bufferUpdateInfo := copyWorkflowExecutionInfo(partialInfo)
	bufferedUpdatedStats := copyExecutionStats(partialState.ExecutionStats)
	err2 = s.UpdateWorklowStateAndReplication(ctx, bufferUpdateInfo, bufferedUpdatedStats, versionHistories, bufferUpdateInfo.NextEventID, nil)
	s.NoError(err2)
	err2 = s.UpdateWorklowStateAndReplication(ctx, bufferUpdateInfo, bufferedUpdatedStats, versionHistories, bufferUpdateInfo.NextEventID, nil)
	s.NoError(err2)
	err2 = s.UpdateWorkflowExecutionForBufferEvents(ctx, bufferUpdateInfo, bufferedUpdatedStats, bufferUpdateInfo.NextEventID, eventsBatch1, false, versionHistories)
	s.NoError(err2)
	stats0, state0, err2 = s.GetWorkflowExecutionInfoWithStats(ctx, domainID, workflowExecution)
	s.NoError(err2)
	s.Equal(1, stats0.BufferedEventsCount)
	s.True(stats0.BufferedEventsSize > 0)
	s.assertChecksumsEqual(testWorkflowChecksum, state0.Checksum)
	testHistory := &types.History{Events: make([]*types.HistoryEvent, 0)}
	testHistory.Events = append(testHistory.Events, eventsBatch1...)
	history0 := &types.History{Events: state0.BufferedEvents}
	s.Equal(testHistory, history0)
	testHistory.Events = append(testHistory.Events, eventsBatch2...)

	err2 = s.UpdateWorkflowExecutionForBufferEvents(ctx, bufferUpdateInfo, bufferedUpdatedStats, bufferUpdateInfo.NextEventID, eventsBatch2, false, versionHistories)
	s.NoError(err2)

	stats1, state1, err1 := s.GetWorkflowExecutionInfoWithStats(ctx, domainID, workflowExecution)
	s.NoError(err1)
	s.NotNil(state1, "expected valid state.")
	info1 := state1.ExecutionInfo
	s.NotNil(info1, "Valid Workflow info expected.")
	s.Equal(2, stats1.BufferedEventsCount)
	s.True(stats1.BufferedEventsSize > 0)
	s.assertChecksumsEqual(testWorkflowChecksum, state1.Checksum)
	history1 := &types.History{Events: state1.BufferedEvents}
	s.Equal(testHistory, history1)

	s.Equal(2, len(state1.ActivityInfos))
	ai, ok := state1.ActivityInfos[4]
	s.True(ok)
	s.NotNil(ai)
	s.Equal(int64(7789), ai.Version)
	s.Equal(int64(4), ai.ScheduleID)
	s.Equal(int64(3), ai.ScheduledEventBatchID)
	s.Equal(int64(40), ai.ScheduledEvent.ID)
	s.EqualTimes(currentTime, ai.ScheduledTime)
	s.Equal(int64(6), ai.StartedID)
	s.Equal(int64(60), ai.StartedEvent.ID)
	s.EqualTimes(currentTime, ai.StartedTime)
	s.Equal(int32(1), ai.ScheduleToCloseTimeout)
	s.Equal(int32(2), ai.ScheduleToStartTimeout)
	s.Equal(int32(3), ai.StartToCloseTimeout)
	s.Equal(int32(4), ai.HeartbeatTimeout)
	s.EqualTimes(currentTime, ai.LastHeartBeatUpdatedTime)
	s.Equal(int32(1), ai.TimerTaskStatus)

	ai, ok = state1.ActivityInfos[5]
	s.True(ok)
	s.NotNil(ai)
	s.Equal(int64(7789), ai.Version)
	s.Equal(int64(5), ai.ScheduleID)
	s.Equal(int64(3), ai.ScheduledEventBatchID)
	s.Equal(int64(50), ai.ScheduledEvent.ID)
	s.EqualTimes(currentTime, ai.ScheduledTime)
	s.Equal(int64(7), ai.StartedID)
	s.Equal(int64(70), ai.StartedEvent.ID)
	s.EqualTimes(currentTime, ai.StartedTime)
	s.Equal(int32(1), ai.ScheduleToCloseTimeout)
	s.Equal(int32(2), ai.ScheduleToStartTimeout)
	s.Equal(int32(3), ai.StartToCloseTimeout)
	s.Equal(int32(4), ai.HeartbeatTimeout)
	s.EqualTimes(currentTime, ai.LastHeartBeatUpdatedTime)
	s.Equal(int32(1), ai.TimerTaskStatus)

	s.Equal(3, len(state1.TimerInfos))
	ti, ok := state1.TimerInfos["t1"]
	s.True(ok)
	s.NotNil(ti)
	s.Equal(int64(2333), ti.Version)
	s.Equal("t1", ti.TimerID)
	s.Equal(int64(1), ti.StartedID)
	s.EqualTimes(expiryTime, ti.ExpiryTime)
	s.Equal(int64(500), ti.TaskStatus)

	ti, ok = state1.TimerInfos["t2"]
	s.True(ok)
	s.NotNil(ti)
	s.Equal(int64(2333), ti.Version)
	s.Equal("t2", ti.TimerID)
	s.Equal(int64(2), ti.StartedID)
	s.EqualTimes(expiryTime, ti.ExpiryTime)
	s.Equal(int64(501), ti.TaskStatus)

	ti, ok = state1.TimerInfos["t3"]
	s.True(ok)
	s.NotNil(ti)
	s.Equal(int64(2333), ti.Version)
	s.Equal("t3", ti.TimerID)
	s.Equal(int64(3), ti.StartedID)
	s.EqualTimes(expiryTime, ti.ExpiryTime)
	s.Equal(int64(502), ti.TaskStatus)

	s.Equal(1, len(state1.ChildExecutionInfos))
	ci, ok := state1.ChildExecutionInfos[9]
	s.True(ok)
	s.NotNil(ci)
	s.Equal(updatedState.ChildExecutionInfos[9], ci)

	s.Equal(1, len(state1.RequestCancelInfos))
	rci, ok := state1.RequestCancelInfos[19]
	s.True(ok)
	s.NotNil(rci)
	s.Equal(int64(2335), rci.Version)

	s.Equal(1, len(state1.SignalInfos))
	si, ok := state1.SignalInfos[39]
	s.True(ok)
	s.NotNil(si)
	s.Equal(int64(2336), si.Version)

	s.Equal(3, len(state1.SignalRequestedIDs))
	_, contains := state1.SignalRequestedIDs["00000000-0000-0000-0000-000000000001"]
	s.True(contains)
	_, contains = state1.SignalRequestedIDs["00000000-0000-0000-0000-000000000002"]
	s.True(contains)
	_, contains = state1.SignalRequestedIDs["00000000-0000-0000-0000-000000000003"]
	s.True(contains)

	s.Equal(3, len(state1.BufferedEvents))

	updatedInfo1 := copyWorkflowExecutionInfo(info1)
	updatedStats1 := copyExecutionStats(state1.ExecutionStats)
	updatedInfo1.NextEventID = int64(3)
	resetActivityInfos := []*p.ActivityInfo{
		{
			Version:                  8789,
			ScheduleID:               40,
			ScheduledEventBatchID:    30,
			ScheduledEvent:           &types.HistoryEvent{ID: 400},
			ScheduledTime:            currentTime,
			StartedID:                60,
			StartedEvent:             &types.HistoryEvent{ID: 600},
			StartedTime:              currentTime,
			ScheduleToCloseTimeout:   10,
			ScheduleToStartTimeout:   20,
			StartToCloseTimeout:      30,
			HeartbeatTimeout:         40,
			LastHeartBeatUpdatedTime: currentTime,
			TimerTaskStatus:          1,
		}}

	resetTimerInfos := []*p.TimerInfo{
		{
			Version:    3333,
			TimerID:    "t1_new",
			StartedID:  1,
			ExpiryTime: expiryTime,
			TaskStatus: 600,
		},
		{
			Version:    3333,
			TimerID:    "t2_new",
			StartedID:  2,
			ExpiryTime: expiryTime,
			TaskStatus: 601,
		}}

	resetChildExecutionInfos := []*p.ChildExecutionInfo{
		{
			Version:         3334,
			InitiatedID:     10,
			InitiatedEvent:  &types.HistoryEvent{ID: 10},
			StartedID:       15,
			StartedRunID:    uuid.New(),
			StartedEvent:    nil,
			CreateRequestID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			DomainID:        uuid.New(),
		}}

	resetRequestCancelInfos := []*p.RequestCancelInfo{
		{
			Version:         3335,
			InitiatedID:     29,
			CancelRequestID: "new_cancel_requested_id",
		}}

	resetSignalInfos := []*p.SignalInfo{
		{
			Version:         3336,
			InitiatedID:     39,
			SignalRequestID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			SignalName:      "signalB",
			Input:           []byte("signal_input_b"),
			Control:         []byte("signal_control_b"),
		},
		{
			Version:         3336,
			InitiatedID:     42,
			SignalRequestID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			SignalName:      "signalC",
			Input:           []byte("signal_input_c"),
			Control:         []byte("signal_control_c"),
		}}
	err3 := s.ConflictResolveWorkflowExecution(
		ctx,
		updatedInfo1,
		updatedStats1,
		int64(5),
		resetActivityInfos,
		resetTimerInfos,
		resetChildExecutionInfos,
		resetRequestCancelInfos,
		resetSignalInfos,
		nil,
		versionHistories,
	)
	s.NoError(err3)

	stats4, state4, err4 := s.GetWorkflowExecutionInfoWithStats(ctx, domainID, workflowExecution)
	s.NoError(err4)
	s.NotNil(state4, "expected valid state.")
	s.Equal(0, stats4.BufferedEventsCount)
	s.Equal(0, stats4.BufferedEventsSize)

	info4 := state4.ExecutionInfo
	s.T().Logf("%+v\n", info4)
	s.NotNil(info4, "Valid Workflow info expected.")
	s.Equal(int64(3), info4.NextEventID)
	s.Equal("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", info4.FirstExecutionRunID)
	s.Equal(partitionConfig, info4.PartitionConfig)

	s.Equal(1, len(state4.ActivityInfos))
	ai, ok = state4.ActivityInfos[40]
	s.True(ok)
	s.NotNil(ai)
	s.Equal(int64(8789), ai.Version)
	s.Equal(int64(40), ai.ScheduleID)
	s.Equal(int64(30), ai.ScheduledEventBatchID)
	s.Equal(int64(400), ai.ScheduledEvent.ID)
	s.Equal(currentTime.Unix(), ai.ScheduledTime.Unix())
	s.Equal(int64(60), ai.StartedID)
	s.Equal(int64(600), ai.StartedEvent.ID)
	s.Equal(currentTime.Unix(), ai.StartedTime.Unix())
	s.Equal(int32(10), ai.ScheduleToCloseTimeout)
	s.Equal(int32(20), ai.ScheduleToStartTimeout)
	s.Equal(int32(30), ai.StartToCloseTimeout)
	s.Equal(int32(40), ai.HeartbeatTimeout)
	s.Equal(currentTime.Unix(), ai.LastHeartBeatUpdatedTime.Unix())
	s.Equal(int32(1), ai.TimerTaskStatus)

	s.Equal(2, len(state4.TimerInfos))
	ti, ok = state4.TimerInfos["t1_new"]
	s.True(ok)
	s.NotNil(ai)
	s.Equal(int64(3333), ti.Version)
	s.Equal("t1_new", ti.TimerID)
	s.Equal(int64(1), ti.StartedID)
	s.EqualTimes(expiryTime, ti.ExpiryTime)
	s.Equal(int64(600), ti.TaskStatus)

	ti, ok = state4.TimerInfos["t2_new"]
	s.True(ok)
	s.NotNil(ai)
	s.Equal(int64(3333), ti.Version)
	s.Equal("t2_new", ti.TimerID)
	s.Equal(int64(2), ti.StartedID)
	s.EqualTimes(expiryTime, ti.ExpiryTime)
	s.Equal(int64(601), ti.TaskStatus)

	s.Equal(1, len(state4.ChildExecutionInfos))
	ci, ok = state4.ChildExecutionInfos[10]
	s.True(ok)
	s.NotNil(ci)
	s.Equal(resetChildExecutionInfos[0], ci)

	s.Equal(1, len(state4.RequestCancelInfos))
	rci, ok = state4.RequestCancelInfos[29]
	s.True(ok)
	s.NotNil(rci)
	s.Equal(int64(3335), rci.Version)
	s.Equal(int64(29), rci.InitiatedID)
	s.Equal("new_cancel_requested_id", rci.CancelRequestID)

	s.Equal(2, len(state4.SignalInfos))
	si, ok = state4.SignalInfos[39]
	s.True(ok)
	s.NotNil(si)
	s.Equal(int64(3336), si.Version)
	s.Equal(int64(39), si.InitiatedID)
	s.Equal("signalB", si.SignalName)
	s.Equal([]byte("signal_input_b"), si.Input)
	s.Equal([]byte("signal_control_b"), si.Control)

	si, ok = state4.SignalInfos[42]
	s.True(ok)
	s.NotNil(si)
	s.Equal(int64(3336), si.Version)
	s.Equal(int64(42), si.InitiatedID)
	s.Equal("signalC", si.SignalName)
	s.Equal([]byte("signal_input_c"), si.Input)
	s.Equal([]byte("signal_control_c"), si.Control)

	s.Equal(0, len(state4.SignalRequestedIDs))

	s.Equal(0, len(state4.BufferedEvents))
	s.assertChecksumsEqual(testWorkflowChecksum, state4.Checksum)

}

// TestConflictResolveWorkflowExecutionWithCASMismatch test
func (s *ExecutionManagerSuite) TestConflictResolveWorkflowExecutionWithCASMismatch() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "4ca1faac-1a3a-47af-8e51-fdaa2b3d45b9"
	workflowID := "test-reset-mutable-state-test-mismatch"

	// first create a workflow and continue as new it
	workflowExecutionReset := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa0",
	}
	partitionConfig := map[string]string{
		"userid": uuid.New(),
	}
	nextEventID := int64(3)
	resp, err := s.CreateWorkflowExecution(
		ctx,
		domainID,
		workflowExecutionReset,
		"taskList",
		"wType",
		20,
		13,
		nil,
		nextEventID,
		0,
		2,
		nil,
		partitionConfig,
	)
	s.NoError(err)
	s.NotNil(resp)

	state, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)

	info := state.ExecutionInfo
	continueAsNewInfo := copyWorkflowExecutionInfo(info)
	continueAsNewStats := copyExecutionStats(state.ExecutionStats)
	continueAsNewInfo.State = p.WorkflowStateRunning
	continueAsNewInfo.NextEventID = int64(5)
	continueAsNewInfo.LastProcessedEvent = int64(2)

	workflowExecutionCurrent := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1",
	}
	err = s.ContinueAsNewExecution(
		ctx,
		continueAsNewInfo,
		continueAsNewStats,
		info.NextEventID,
		workflowExecutionCurrent,
		int64(3),
		int64(2),
		nil,
	)
	s.NoError(err)

	runID1, err := s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.NoError(err)
	s.Equal(workflowExecutionCurrent.GetRunID(), runID1)
	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionCurrent)
	s.NoError(err)
	currentInfo := copyWorkflowExecutionInfo(state.ExecutionInfo)
	currentStats := copyExecutionStats(state.ExecutionStats)
	currentInfo.State = p.WorkflowStateCompleted
	currentInfo.CloseStatus = p.WorkflowCloseStatusCompleted
	currentInfo.NextEventID = int64(6)
	currentInfo.LastProcessedEvent = int64(2)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: currentInfo.LastProcessedEvent,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	err3 := s.UpdateWorkflowExecutionAndFinish(ctx, currentInfo, currentStats, int64(3), versionHistories)
	s.NoError(err3)
	runID1, err = s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.NoError(err)
	s.Equal(workflowExecutionCurrent.GetRunID(), runID1)

	resetExecutionInfo := &p.WorkflowExecutionInfo{
		DomainID:                    domainID,
		WorkflowID:                  workflowExecutionReset.GetWorkflowID(),
		RunID:                       workflowExecutionReset.GetRunID(),
		ParentDomainID:              uuid.New(),
		ParentWorkflowID:            "some random parent workflow ID",
		ParentRunID:                 uuid.New(),
		InitiatedID:                 12345,
		TaskList:                    "some random tasklist",
		WorkflowTypeName:            "some random workflow type name",
		WorkflowTimeout:             1112,
		DecisionStartToCloseTimeout: 14,
		State:                       p.WorkflowStateRunning,
		LastFirstEventID:            constants.FirstEventID,
		NextEventID:                 123,
		CreateRequestID:             uuid.New(),
		DecisionVersion:             constants.EmptyVersion,
		DecisionScheduleID:          111,
		DecisionStartedID:           222,
		DecisionRequestID:           uuid.New(),
		DecisionTimeout:             0,
	}
	resetStats := &p.ExecutionStats{}
	resetActivityInfos := []*p.ActivityInfo{}
	resetTimerInfos := []*p.TimerInfo{}
	resetChildExecutionInfos := []*p.ChildExecutionInfo{}
	resetRequestCancelInfos := []*p.RequestCancelInfo{}
	resetSignalInfos := []*p.SignalInfo{}

	err = s.ConflictResolveWorkflowExecution(
		ctx,
		resetExecutionInfo,
		resetStats,
		continueAsNewInfo.NextEventID,
		resetActivityInfos,
		resetTimerInfos,
		resetChildExecutionInfos,
		resetRequestCancelInfos,
		resetSignalInfos,
		nil,
		versionHistories,
	)
	s.NotNil(err)

	err = s.ConflictResolveWorkflowExecution(
		ctx,
		resetExecutionInfo,
		resetStats,
		continueAsNewInfo.NextEventID,
		resetActivityInfos,
		resetTimerInfos,
		resetChildExecutionInfos,
		resetRequestCancelInfos,
		resetSignalInfos,
		nil,
		versionHistories,
	)
	s.NotNil(err)

	err = s.ConflictResolveWorkflowExecution(
		ctx,
		resetExecutionInfo,
		resetStats,
		continueAsNewInfo.NextEventID,
		resetActivityInfos,
		resetTimerInfos,
		resetChildExecutionInfos,
		resetRequestCancelInfos,
		resetSignalInfos,
		nil,
		versionHistories,
	)
	s.NotNil(err)

	// this test only assert whether the current workflow execution record is reset
	runID, err := s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.NoError(err)
	s.Equal(workflowExecutionCurrent.GetRunID(), runID)
}

// TestConflictResolveWorkflowExecutionWithTransactionCurrentIsNotSelf test
func (s *ExecutionManagerSuite) TestConflictResolveWorkflowExecutionWithTransactionCurrentIsNotSelf() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "4ca1faac-1a3a-47af-8e51-fdaa2b3d45b9"
	workflowID := "test-reset-mutable-state-test-with-transaction-current-is-not-self"

	// first create a workflow and continue as new it
	workflowExecutionReset := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa0",
	}
	nextEventID := int64(3)
	partitionConfig := map[string]string{
		"userid": uuid.New(),
	}
	resp, err := s.CreateWorkflowExecution(
		ctx,
		domainID,
		workflowExecutionReset,
		"taskList",
		"wType",
		20,
		13,
		nil,
		nextEventID,
		0,
		2,
		nil,
		partitionConfig,
	)
	s.NoError(err)
	s.NotNil(resp)

	state, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)

	info := state.ExecutionInfo
	continueAsNewInfo := copyWorkflowExecutionInfo(info)
	continueAsNewStats := copyExecutionStats(state.ExecutionStats)
	continueAsNewInfo.State = p.WorkflowStateRunning
	continueAsNewInfo.NextEventID = int64(5)
	continueAsNewInfo.LastProcessedEvent = int64(2)

	workflowExecutionCurrent := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1",
	}
	err = s.ContinueAsNewExecution(
		ctx,
		continueAsNewInfo,
		continueAsNewStats,
		info.NextEventID,
		workflowExecutionCurrent,
		int64(3),
		int64(2),
		nil,
	)
	s.NoError(err)

	currentRunID, err := s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.NoError(err)
	s.Equal(workflowExecutionCurrent.GetRunID(), currentRunID)
	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionCurrent)
	s.NoError(err)
	currentInfo := copyWorkflowExecutionInfo(state.ExecutionInfo)
	currentInfo.State = p.WorkflowStateCompleted
	currentInfo.CloseStatus = p.WorkflowCloseStatusTerminated

	resetExecutionInfo := &p.WorkflowExecutionInfo{
		DomainID:                    domainID,
		WorkflowID:                  workflowExecutionReset.GetWorkflowID(),
		RunID:                       workflowExecutionReset.GetRunID(),
		FirstExecutionRunID:         workflowExecutionReset.GetRunID(),
		ParentDomainID:              uuid.New(),
		ParentWorkflowID:            "some random parent workflow ID",
		ParentRunID:                 uuid.New(),
		InitiatedID:                 12345,
		TaskList:                    "some random tasklist",
		WorkflowTypeName:            "some random workflow type name",
		WorkflowTimeout:             1112,
		DecisionStartToCloseTimeout: 14,
		State:                       p.WorkflowStateCompleted,
		CloseStatus:                 p.WorkflowCloseStatusCompleted,
		LastFirstEventID:            constants.FirstEventID,
		NextEventID:                 123,
		CreateRequestID:             uuid.New(),
		DecisionVersion:             constants.EmptyVersion,
		DecisionScheduleID:          111,
		DecisionStartedID:           222,
		DecisionRequestID:           uuid.New(),
		DecisionTimeout:             0,
		AutoResetPoints:             &types.ResetPoints{},
		PartitionConfig:             nil,
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: resetExecutionInfo.DecisionScheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	resetReq := &p.ConflictResolveWorkflowExecutionRequest{
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.ConflictResolveWorkflowModeBypassCurrent,
		ResetWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo:  resetExecutionInfo,
			ExecutionStats: &p.ExecutionStats{},
			Condition:      int64(5),

			ActivityInfos:       []*p.ActivityInfo{},
			TimerInfos:          []*p.TimerInfo{},
			ChildExecutionInfos: []*p.ChildExecutionInfo{},
			RequestCancelInfos:  []*p.RequestCancelInfo{},
			SignalInfos:         []*p.SignalInfo{},
			SignalRequestedIDs:  []string{},
			VersionHistories:    versionHistories,
		},
		NewWorkflowSnapshot: nil,
		CurrentWorkflowMutation: &p.WorkflowMutation{
			ExecutionInfo:  currentInfo,
			ExecutionStats: &p.ExecutionStats{},
			Condition:      int64(3),

			UpsertActivityInfos:       []*p.ActivityInfo{},
			UpsertTimerInfos:          []*p.TimerInfo{},
			UpsertChildExecutionInfos: []*p.ChildExecutionInfo{},
			UpsertRequestCancelInfos:  []*p.RequestCancelInfo{},
			UpsertSignalInfos:         []*p.SignalInfo{},
			UpsertSignalRequestedIDs:  []string{},
			VersionHistories:          versionHistories,
		},
		Encoding: pickRandomEncoding(),
	}
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.Error(err)
	resetReq.Mode = p.ConflictResolveWorkflowModeUpdateCurrent
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.NoError(err)

	currentRecord, err := s.ExecutionManager.GetCurrentExecution(ctx, &p.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
	})
	s.NoError(err)
	s.Equal(resetExecutionInfo.RunID, currentRecord.RunID)
	s.Equal(resetExecutionInfo.CreateRequestID, currentRecord.StartRequestID)
	s.Equal(resetExecutionInfo.State, currentRecord.State)
	s.Equal(resetExecutionInfo.CloseStatus, currentRecord.CloseStatus)
	s.Equal(int64(-24), currentRecord.LastWriteVersion)

	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.ResetWorkflowSnapshot.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.ResetWorkflowSnapshot.ExecutionInfo, state.ExecutionInfo)

	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionCurrent)
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.CurrentWorkflowMutation.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.CurrentWorkflowMutation.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.CurrentWorkflowMutation.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.CurrentWorkflowMutation.ExecutionInfo, state.ExecutionInfo)
}

// TestConflictResolveWorkflowExecutionWithTransactionCurrentIsNotSelfWithContinueAsNew test
func (s *ExecutionManagerSuite) TestConflictResolveWorkflowExecutionWithTransactionCurrentIsNotSelfWithContinueAsNew() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "4ca1faac-1a3a-47af-8e51-fdaa2b3d45b9"
	workflowID := "test-reset-mutable-state-test-with-transaction-current-is-not-self-with-continue-as-new"

	// first create a workflow and continue as new it
	workflowExecutionReset := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa0",
	}
	nextEventID := int64(3)
	partitionConfig := map[string]string{
		"userid": uuid.New(),
	}
	resp, err := s.CreateWorkflowExecution(
		ctx,
		domainID,
		workflowExecutionReset,
		"taskList",
		"wType",
		20,
		13,
		nil,
		nextEventID,
		0,
		2,
		nil,
		partitionConfig,
	)
	s.NoError(err)
	s.NotNil(resp)

	state, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)
	continueAsNewStats := copyExecutionStats(state.ExecutionStats)

	info := state.ExecutionInfo
	continueAsNewInfo := copyWorkflowExecutionInfo(info)
	continueAsNewInfo.State = p.WorkflowStateRunning
	continueAsNewInfo.NextEventID = int64(5)
	continueAsNewInfo.LastProcessedEvent = int64(2)

	workflowExecutionCurrent := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1",
	}
	err = s.ContinueAsNewExecution(
		ctx,
		continueAsNewInfo,
		continueAsNewStats,
		info.NextEventID,
		workflowExecutionCurrent,
		int64(3),
		int64(2),
		nil,
	)
	s.NoError(err)

	currentRunID, err := s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.NoError(err)
	s.Equal(workflowExecutionCurrent.GetRunID(), currentRunID)
	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionCurrent)
	s.NoError(err)
	currentInfo := copyWorkflowExecutionInfo(state.ExecutionInfo)
	currentStats := copyExecutionStats(state.ExecutionStats)
	currentInfo.State = p.WorkflowStateCompleted
	currentInfo.CloseStatus = p.WorkflowCloseStatusTerminated
	decisionScheduleID := int64(111)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: decisionScheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	resetExecutionInfo := &p.WorkflowExecutionInfo{
		DomainID:                    domainID,
		WorkflowID:                  workflowExecutionReset.GetWorkflowID(),
		RunID:                       workflowExecutionReset.GetRunID(),
		FirstExecutionRunID:         workflowExecutionReset.GetRunID(),
		ParentDomainID:              uuid.New(),
		ParentWorkflowID:            "some random parent workflow ID",
		ParentRunID:                 uuid.New(),
		InitiatedID:                 12345,
		TaskList:                    "some random tasklist",
		WorkflowTypeName:            "some random workflow type name",
		WorkflowTimeout:             1112,
		DecisionStartToCloseTimeout: 14,
		State:                       p.WorkflowStateCompleted,
		CloseStatus:                 p.WorkflowCloseStatusContinuedAsNew,
		LastFirstEventID:            constants.FirstEventID,
		NextEventID:                 123,
		CreateRequestID:             uuid.New(),
		DecisionVersion:             constants.EmptyVersion,
		DecisionScheduleID:          decisionScheduleID,
		DecisionStartedID:           222,
		DecisionRequestID:           uuid.New(),
		DecisionTimeout:             0,
		AutoResetPoints:             &types.ResetPoints{},
		PartitionConfig:             map[string]string{"zone": "dca1"},
	}
	newWorkflowExecutionInfo := copyWorkflowExecutionInfo(resetExecutionInfo)
	newWorkflowExecutionStats := &p.ExecutionStats{}
	newWorkflowExecutionInfo.CreateRequestID = uuid.New()
	newWorkflowExecutionInfo.RunID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2"
	newWorkflowExecutionInfo.State = p.WorkflowStateRunning
	newWorkflowExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	newWorkflowExecutionInfo.PartitionConfig = map[string]string{"user": uuid.New()}

	resetReq := &p.ConflictResolveWorkflowExecutionRequest{
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.ConflictResolveWorkflowModeBypassCurrent,
		ResetWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo:  resetExecutionInfo,
			ExecutionStats: &p.ExecutionStats{},
			Condition:      int64(5),

			ActivityInfos:       []*p.ActivityInfo{},
			TimerInfos:          []*p.TimerInfo{},
			ChildExecutionInfos: []*p.ChildExecutionInfo{},
			RequestCancelInfos:  []*p.RequestCancelInfo{},
			SignalInfos:         []*p.SignalInfo{},
			SignalRequestedIDs:  []string{},
			VersionHistories:    versionHistories,
		},
		NewWorkflowSnapshot: &p.WorkflowSnapshot{
			ExecutionInfo:  newWorkflowExecutionInfo,
			ExecutionStats: newWorkflowExecutionStats,
			Condition:      0,

			ActivityInfos:       []*p.ActivityInfo{},
			TimerInfos:          []*p.TimerInfo{},
			ChildExecutionInfos: []*p.ChildExecutionInfo{},
			RequestCancelInfos:  []*p.RequestCancelInfo{},
			SignalInfos:         []*p.SignalInfo{},
			SignalRequestedIDs:  []string{},
			VersionHistories:    versionHistories,
		},
		CurrentWorkflowMutation: &p.WorkflowMutation{
			ExecutionInfo:  currentInfo,
			ExecutionStats: currentStats,
			Condition:      int64(3),

			UpsertActivityInfos:       []*p.ActivityInfo{},
			UpsertTimerInfos:          []*p.TimerInfo{},
			UpsertChildExecutionInfos: []*p.ChildExecutionInfo{},
			UpsertRequestCancelInfos:  []*p.RequestCancelInfo{},
			UpsertSignalInfos:         []*p.SignalInfo{},
			UpsertSignalRequestedIDs:  []string{},
			VersionHistories:          versionHistories,
		},
		Encoding: pickRandomEncoding(),
	}
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.Error(err)
	resetReq.Mode = p.ConflictResolveWorkflowModeUpdateCurrent
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.NoError(err)

	currentRecord, err := s.ExecutionManager.GetCurrentExecution(ctx, &p.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
	})
	s.NoError(err)
	s.Equal(newWorkflowExecutionInfo.RunID, currentRecord.RunID)
	s.Equal(newWorkflowExecutionInfo.CreateRequestID, currentRecord.StartRequestID)
	s.Equal(newWorkflowExecutionInfo.State, currentRecord.State)
	s.Equal(newWorkflowExecutionInfo.CloseStatus, currentRecord.CloseStatus)
	s.Equal(int64(-24), currentRecord.LastWriteVersion)

	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.ResetWorkflowSnapshot.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.ResetWorkflowSnapshot.ExecutionInfo, state.ExecutionInfo)

	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionCurrent)
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.CurrentWorkflowMutation.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.CurrentWorkflowMutation.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.CurrentWorkflowMutation.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.CurrentWorkflowMutation.ExecutionInfo, state.ExecutionInfo)

	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      resetReq.NewWorkflowSnapshot.ExecutionInfo.RunID,
	})
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.NewWorkflowSnapshot.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.NewWorkflowSnapshot.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.NewWorkflowSnapshot.ExecutionInfo, state.ExecutionInfo)
}

// TestConflictResolveWorkflowExecutionWithTransactionCurrentIsSelf test
func (s *ExecutionManagerSuite) TestConflictResolveWorkflowExecutionWithTransactionCurrentIsSelf() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "4ca1faac-1a3a-47af-8e51-fdaa2b3d45b9"
	workflowID := "test-reset-mutable-state-test-with-transaction-current-is-self"

	// first create a workflow and continue as new it
	workflowExecutionReset := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa0",
	}
	partitionConfig := map[string]string{
		"userid": uuid.New(),
	}
	nextEventID := int64(3)
	resp, err := s.CreateWorkflowExecution(
		ctx,
		domainID,
		workflowExecutionReset,
		"taskList",
		"wType",
		20,
		13,
		nil,
		nextEventID,
		0,
		2,
		nil,
		partitionConfig,
	)
	s.NoError(err)
	s.NotNil(resp)

	_, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)

	resetExecutionInfo := &p.WorkflowExecutionInfo{
		DomainID:                    domainID,
		WorkflowID:                  workflowExecutionReset.GetWorkflowID(),
		RunID:                       workflowExecutionReset.GetRunID(),
		FirstExecutionRunID:         workflowExecutionReset.GetRunID(),
		ParentDomainID:              uuid.New(),
		ParentWorkflowID:            "some random parent workflow ID",
		ParentRunID:                 uuid.New(),
		InitiatedID:                 12345,
		TaskList:                    "some random tasklist",
		WorkflowTypeName:            "some random workflow type name",
		WorkflowTimeout:             1112,
		DecisionStartToCloseTimeout: 14,
		State:                       p.WorkflowStateRunning,
		CloseStatus:                 p.WorkflowCloseStatusNone,
		LastFirstEventID:            constants.FirstEventID,
		NextEventID:                 123,
		CreateRequestID:             uuid.New(),
		DecisionVersion:             constants.EmptyVersion,
		DecisionScheduleID:          111,
		DecisionStartedID:           222,
		DecisionRequestID:           uuid.New(),
		DecisionTimeout:             0,
		AutoResetPoints:             &types.ResetPoints{},
		PartitionConfig:             nil,
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: resetExecutionInfo.DecisionScheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	resetReq := &p.ConflictResolveWorkflowExecutionRequest{
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.ConflictResolveWorkflowModeBypassCurrent,
		ResetWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo:  resetExecutionInfo,
			ExecutionStats: &p.ExecutionStats{},
			Condition:      nextEventID,

			ActivityInfos:       []*p.ActivityInfo{},
			TimerInfos:          []*p.TimerInfo{},
			ChildExecutionInfos: []*p.ChildExecutionInfo{},
			RequestCancelInfos:  []*p.RequestCancelInfo{},
			SignalInfos:         []*p.SignalInfo{},
			SignalRequestedIDs:  []string{},
			VersionHistories:    versionHistories,
		},
		NewWorkflowSnapshot:     nil,
		CurrentWorkflowMutation: nil,
		Encoding:                pickRandomEncoding(),
	}
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.Error(err)
	resetReq.Mode = p.ConflictResolveWorkflowModeUpdateCurrent
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.NoError(err)

	currentRecord, err := s.ExecutionManager.GetCurrentExecution(ctx, &p.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
	})
	s.NoError(err)
	s.Equal(resetExecutionInfo.RunID, currentRecord.RunID)
	s.Equal(resetExecutionInfo.CreateRequestID, currentRecord.StartRequestID)
	s.Equal(resetExecutionInfo.State, currentRecord.State)
	s.Equal(resetExecutionInfo.CloseStatus, currentRecord.CloseStatus)
	s.Equal(int64(-24), currentRecord.LastWriteVersion)

	state, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.ResetWorkflowSnapshot.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.ResetWorkflowSnapshot.ExecutionInfo, state.ExecutionInfo)
}

// TestConflictResolveWorkflowExecutionWithTransactionCurrentIsSelfWithContinueAsNew test
func (s *ExecutionManagerSuite) TestConflictResolveWorkflowExecutionWithTransactionCurrentIsSelfWithContinueAsNew() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "4ca1faac-1a3a-47af-8e51-fdaa2b3d45b9"
	workflowID := "test-reset-mutable-state-test-with-transaction-current-is-self-with-continue-as-new"

	// first create a workflow and continue as new it
	workflowExecutionReset := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa0",
	}
	partitionConfig := map[string]string{
		"userid": uuid.New(),
	}
	nextEventID := int64(3)
	resp, err := s.CreateWorkflowExecution(
		ctx,
		domainID,
		workflowExecutionReset,
		"taskList",
		"wType",
		20,
		13,
		nil,
		nextEventID,
		0,
		2,
		nil,
		partitionConfig,
	)
	s.NoError(err)
	s.NotNil(resp)

	_, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)

	decisionScheduleID := int64(111)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: decisionScheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	resetExecutionInfo := &p.WorkflowExecutionInfo{
		DomainID:                    domainID,
		WorkflowID:                  workflowExecutionReset.GetWorkflowID(),
		RunID:                       workflowExecutionReset.GetRunID(),
		FirstExecutionRunID:         workflowExecutionReset.GetRunID(),
		ParentDomainID:              uuid.New(),
		ParentWorkflowID:            "some random parent workflow ID",
		ParentRunID:                 uuid.New(),
		InitiatedID:                 12345,
		TaskList:                    "some random tasklist",
		WorkflowTypeName:            "some random workflow type name",
		WorkflowTimeout:             1112,
		DecisionStartToCloseTimeout: 14,
		State:                       p.WorkflowStateCompleted,
		CloseStatus:                 p.WorkflowCloseStatusContinuedAsNew,
		LastFirstEventID:            constants.FirstEventID,
		NextEventID:                 123,
		CreateRequestID:             uuid.New(),
		DecisionVersion:             constants.EmptyVersion,
		DecisionScheduleID:          decisionScheduleID,
		DecisionStartedID:           222,
		DecisionRequestID:           uuid.New(),
		DecisionTimeout:             0,
		AutoResetPoints:             &types.ResetPoints{},
		PartitionConfig:             nil,
	}
	newWorkflowExecutionInfo := copyWorkflowExecutionInfo(resetExecutionInfo)
	newWorkflowExecutionStats := &p.ExecutionStats{}
	newWorkflowExecutionInfo.CreateRequestID = uuid.New()
	newWorkflowExecutionInfo.RunID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2"
	newWorkflowExecutionInfo.State = p.WorkflowStateRunning
	newWorkflowExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	newWorkflowExecutionInfo.PartitionConfig = map[string]string{"zone": "dca1"}

	resetReq := &p.ConflictResolveWorkflowExecutionRequest{
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.ConflictResolveWorkflowModeBypassCurrent,
		ResetWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo:  resetExecutionInfo,
			ExecutionStats: &p.ExecutionStats{},
			Condition:      nextEventID,

			ActivityInfos:       []*p.ActivityInfo{},
			TimerInfos:          []*p.TimerInfo{},
			ChildExecutionInfos: []*p.ChildExecutionInfo{},
			RequestCancelInfos:  []*p.RequestCancelInfo{},
			SignalInfos:         []*p.SignalInfo{},
			SignalRequestedIDs:  []string{},
			VersionHistories:    versionHistories,
		},
		NewWorkflowSnapshot: &p.WorkflowSnapshot{
			ExecutionInfo:  newWorkflowExecutionInfo,
			ExecutionStats: newWorkflowExecutionStats,
			Condition:      0,

			ActivityInfos:       []*p.ActivityInfo{},
			TimerInfos:          []*p.TimerInfo{},
			ChildExecutionInfos: []*p.ChildExecutionInfo{},
			RequestCancelInfos:  []*p.RequestCancelInfo{},
			SignalInfos:         []*p.SignalInfo{},
			SignalRequestedIDs:  []string{},
			VersionHistories:    versionHistories,
		},
		CurrentWorkflowMutation: nil,
		Encoding:                pickRandomEncoding(),
	}
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.Error(err)
	resetReq.Mode = p.ConflictResolveWorkflowModeUpdateCurrent
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.NoError(err)

	currentRecord, err := s.ExecutionManager.GetCurrentExecution(ctx, &p.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
	})
	s.NoError(err)
	s.Equal(newWorkflowExecutionInfo.RunID, currentRecord.RunID)
	s.Equal(newWorkflowExecutionInfo.CreateRequestID, currentRecord.StartRequestID)
	s.Equal(newWorkflowExecutionInfo.State, currentRecord.State)
	s.Equal(newWorkflowExecutionInfo.CloseStatus, currentRecord.CloseStatus)
	s.Equal(int64(-24), currentRecord.LastWriteVersion)

	state, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.ResetWorkflowSnapshot.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.ResetWorkflowSnapshot.ExecutionInfo, state.ExecutionInfo)
	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      resetReq.NewWorkflowSnapshot.ExecutionInfo.RunID,
	})
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.NewWorkflowSnapshot.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.NewWorkflowSnapshot.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.NewWorkflowSnapshot.ExecutionInfo, state.ExecutionInfo)
}

// TestConflictResolveWorkflowExecutionWithTransactionZombieIsSelf test
func (s *ExecutionManagerSuite) TestConflictResolveWorkflowExecutionWithTransactionZombieIsSelf() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "4ca1faac-1a3a-47af-8e51-fdaa2b3d45b9"
	workflowID := "test-reset-mutable-state-test-with-transaction-current-is-zombie"

	// first create a workflow and continue as new it
	workflowExecutionReset := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa0",
	}
	partitionConfig := map[string]string{
		"userid": uuid.New(),
	}
	nextEventID := int64(3)
	resp, err := s.CreateWorkflowExecution(
		ctx,
		domainID,
		workflowExecutionReset,
		"taskList",
		"wType",
		20,
		13,
		nil,
		nextEventID,
		0,
		2,
		nil,
		partitionConfig,
	)
	s.NoError(err)
	s.NotNil(resp)

	state, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)

	info := state.ExecutionInfo
	continueAsNewInfo := copyWorkflowExecutionInfo(info)
	continueAsNewStats := copyExecutionStats(state.ExecutionStats)
	continueAsNewInfo.State = p.WorkflowStateRunning
	continueAsNewInfo.NextEventID = int64(5)
	continueAsNewInfo.LastProcessedEvent = int64(2)

	workflowExecutionCurrent := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1",
	}
	err = s.ContinueAsNewExecution(
		ctx,
		continueAsNewInfo,
		continueAsNewStats,
		info.NextEventID,
		workflowExecutionCurrent,
		int64(3),
		int64(2),
		nil,
	)
	s.NoError(err)

	currentRunID, err := s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.NoError(err)
	s.Equal(workflowExecutionCurrent.GetRunID(), currentRunID)

	resetExecutionInfo := &p.WorkflowExecutionInfo{
		DomainID:                    domainID,
		WorkflowID:                  workflowExecutionReset.GetWorkflowID(),
		RunID:                       workflowExecutionReset.GetRunID(),
		FirstExecutionRunID:         workflowExecutionReset.GetRunID(),
		ParentDomainID:              uuid.New(),
		ParentWorkflowID:            "some random parent workflow ID",
		ParentRunID:                 uuid.New(),
		InitiatedID:                 12345,
		TaskList:                    "some random tasklist",
		WorkflowTypeName:            "some random workflow type name",
		WorkflowTimeout:             1112,
		DecisionStartToCloseTimeout: 14,
		State:                       p.WorkflowStateCompleted,
		CloseStatus:                 p.WorkflowCloseStatusCompleted,
		LastFirstEventID:            constants.FirstEventID,
		NextEventID:                 123,
		CreateRequestID:             uuid.New(),
		DecisionVersion:             constants.EmptyVersion,
		DecisionScheduleID:          111,
		DecisionStartedID:           222,
		DecisionRequestID:           uuid.New(),
		DecisionTimeout:             0,
		AutoResetPoints:             &types.ResetPoints{},
		PartitionConfig:             nil,
	}
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: resetExecutionInfo.DecisionScheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)

	resetReq := &p.ConflictResolveWorkflowExecutionRequest{
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.ConflictResolveWorkflowModeUpdateCurrent,
		ResetWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo:  resetExecutionInfo,
			ExecutionStats: &p.ExecutionStats{},
			Condition:      int64(5),

			ActivityInfos:       []*p.ActivityInfo{},
			TimerInfos:          []*p.TimerInfo{},
			ChildExecutionInfos: []*p.ChildExecutionInfo{},
			RequestCancelInfos:  []*p.RequestCancelInfo{},
			SignalInfos:         []*p.SignalInfo{},
			SignalRequestedIDs:  []string{},
			VersionHistories:    versionHistories,
		},
		NewWorkflowSnapshot:     nil,
		CurrentWorkflowMutation: nil,
		Encoding:                pickRandomEncoding(),
	}
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.Error(err)
	resetReq.Mode = p.ConflictResolveWorkflowModeBypassCurrent
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.NoError(err)

	currentRecord, err := s.ExecutionManager.GetCurrentExecution(ctx, &p.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
	})
	s.NoError(err)
	s.Equal(currentRunID, currentRecord.RunID)

	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.ResetWorkflowSnapshot.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.ResetWorkflowSnapshot.ExecutionInfo, state.ExecutionInfo)
}

// TestConflictResolveWorkflowExecutionWithTransactionZombieIsSelfWithContinueAsNew test
func (s *ExecutionManagerSuite) TestConflictResolveWorkflowExecutionWithTransactionZombieIsSelfWithContinueAsNew() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "4ca1faac-1a3a-47af-8e51-fdaa2b3d45b9"
	workflowID := "test-reset-mutable-state-test-with-transaction-current-is-zombie-with-continue-as-new"

	// first create a workflow and continue as new it
	workflowExecutionReset := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa0",
	}
	partitionConfig := map[string]string{
		"userid": uuid.New(),
	}
	nextEventID := int64(3)
	resp, err := s.CreateWorkflowExecution(
		ctx,
		domainID,
		workflowExecutionReset,
		"taskList",
		"wType",
		20,
		13,
		nil,
		nextEventID,
		0,
		2,
		nil,
		partitionConfig,
	)
	s.NoError(err)
	s.NotNil(resp)

	state, err := s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)
	continueAsNewStats := copyExecutionStats(state.ExecutionStats)

	info := state.ExecutionInfo
	continueAsNewInfo := copyWorkflowExecutionInfo(info)
	continueAsNewInfo.State = p.WorkflowStateRunning
	continueAsNewInfo.NextEventID = int64(5)
	continueAsNewInfo.LastProcessedEvent = int64(2)

	workflowExecutionCurrent := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1",
	}
	err = s.ContinueAsNewExecution(
		ctx,
		continueAsNewInfo,
		continueAsNewStats,
		info.NextEventID,
		workflowExecutionCurrent,
		int64(3),
		int64(2),
		nil,
	)
	s.NoError(err)

	currentRunID, err := s.GetCurrentWorkflowRunID(ctx, domainID, workflowID)
	s.NoError(err)
	s.Equal(workflowExecutionCurrent.GetRunID(), currentRunID)

	decisionScheduleID := int64(111)
	versionHistory := p.NewVersionHistory([]byte{}, []*p.VersionHistoryItem{
		{
			EventID: decisionScheduleID,
			Version: constants.EmptyVersion,
		},
	})
	versionHistories := p.NewVersionHistories(versionHistory)
	resetExecutionInfo := &p.WorkflowExecutionInfo{
		DomainID:                    domainID,
		WorkflowID:                  workflowExecutionReset.GetWorkflowID(),
		RunID:                       workflowExecutionReset.GetRunID(),
		FirstExecutionRunID:         workflowExecutionReset.GetRunID(),
		ParentDomainID:              uuid.New(),
		ParentWorkflowID:            "some random parent workflow ID",
		ParentRunID:                 uuid.New(),
		InitiatedID:                 12345,
		TaskList:                    "some random tasklist",
		WorkflowTypeName:            "some random workflow type name",
		WorkflowTimeout:             1112,
		DecisionStartToCloseTimeout: 14,
		State:                       p.WorkflowStateCompleted,
		CloseStatus:                 p.WorkflowCloseStatusContinuedAsNew,
		LastFirstEventID:            constants.FirstEventID,
		NextEventID:                 123,
		CreateRequestID:             uuid.New(),
		DecisionVersion:             constants.EmptyVersion,
		DecisionScheduleID:          decisionScheduleID,
		DecisionStartedID:           222,
		DecisionRequestID:           uuid.New(),
		DecisionTimeout:             0,
		AutoResetPoints:             &types.ResetPoints{},
		PartitionConfig:             nil,
	}
	newWorkflowExecutionInfo := copyWorkflowExecutionInfo(resetExecutionInfo)
	newWorkflowExecutionStats := &p.ExecutionStats{}
	newWorkflowExecutionInfo.CreateRequestID = uuid.New()
	newWorkflowExecutionInfo.RunID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2"
	newWorkflowExecutionInfo.State = p.WorkflowStateZombie
	newWorkflowExecutionInfo.CloseStatus = p.WorkflowCloseStatusNone
	newWorkflowExecutionInfo.PartitionConfig = map[string]string{"zone": "dca1"}
	resetReq := &p.ConflictResolveWorkflowExecutionRequest{
		RangeID: s.ShardInfo.RangeID,
		Mode:    p.ConflictResolveWorkflowModeUpdateCurrent,
		ResetWorkflowSnapshot: p.WorkflowSnapshot{
			ExecutionInfo:  resetExecutionInfo,
			ExecutionStats: &p.ExecutionStats{},
			Condition:      int64(5),

			ActivityInfos:       []*p.ActivityInfo{},
			TimerInfos:          []*p.TimerInfo{},
			ChildExecutionInfos: []*p.ChildExecutionInfo{},
			RequestCancelInfos:  []*p.RequestCancelInfo{},
			SignalInfos:         []*p.SignalInfo{},
			SignalRequestedIDs:  []string{},
			VersionHistories:    versionHistories,
		},
		NewWorkflowSnapshot: &p.WorkflowSnapshot{
			ExecutionInfo:  newWorkflowExecutionInfo,
			ExecutionStats: newWorkflowExecutionStats,
			Condition:      0,

			ActivityInfos:       []*p.ActivityInfo{},
			TimerInfos:          []*p.TimerInfo{},
			ChildExecutionInfos: []*p.ChildExecutionInfo{},
			RequestCancelInfos:  []*p.RequestCancelInfo{},
			SignalInfos:         []*p.SignalInfo{},
			SignalRequestedIDs:  []string{},
			VersionHistories:    versionHistories,
		},
		CurrentWorkflowMutation: nil,
		Encoding:                pickRandomEncoding(),
	}
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.Error(err)
	resetReq.Mode = p.ConflictResolveWorkflowModeBypassCurrent
	_, err = s.ExecutionManager.ConflictResolveWorkflowExecution(ctx, resetReq)
	s.NoError(err)

	currentRecord, err := s.ExecutionManager.GetCurrentExecution(ctx, &p.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
	})
	s.NoError(err)
	s.Equal(currentRunID, currentRecord.RunID)

	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, workflowExecutionReset)
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.ResetWorkflowSnapshot.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.ResetWorkflowSnapshot.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.ResetWorkflowSnapshot.ExecutionInfo, state.ExecutionInfo)

	state, err = s.GetWorkflowExecutionInfo(ctx, domainID, types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      resetReq.NewWorkflowSnapshot.ExecutionInfo.RunID,
	})
	s.NoError(err)
	state.ExecutionInfo.StartTimestamp = resetReq.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp
	state.ExecutionInfo.LastUpdatedTimestamp = resetReq.NewWorkflowSnapshot.ExecutionInfo.LastUpdatedTimestamp
	state.ExecutionInfo.ExpirationTime = resetReq.NewWorkflowSnapshot.ExecutionInfo.ExpirationTime
	s.Equal(resetReq.NewWorkflowSnapshot.ExecutionInfo, state.ExecutionInfo)
}

// TestReplicationDLQ test
func (s *ExecutionManagerSuite) TestReplicationDLQ() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	sourceCluster := "test"
	taskInfo := &p.ReplicationTaskInfo{
		DomainID:   uuid.New(),
		WorkflowID: uuid.New(),
		RunID:      uuid.New(),
		TaskID:     0,
		TaskType:   0,
	}
	err := s.PutReplicationTaskToDLQ(ctx, sourceCluster, taskInfo)
	s.NoError(err)
	resp, err := s.GetReplicationTasksFromDLQ(ctx, sourceCluster, 0, 1, 1, nil)
	s.NoError(err)
	s.Len(resp.Tasks, 1)
	err = s.DeleteReplicationTaskFromDLQ(ctx, sourceCluster, 0)
	s.NoError(err)
	resp, err = s.GetReplicationTasksFromDLQ(ctx, sourceCluster, 0, 1, 1, nil)
	s.NoError(err)
	s.Len(resp.Tasks, 0)

	taskInfo1 := &p.ReplicationTaskInfo{
		DomainID:   uuid.New(),
		WorkflowID: uuid.New(),
		RunID:      uuid.New(),
		TaskID:     1,
		TaskType:   0,
	}
	taskInfo2 := &p.ReplicationTaskInfo{
		DomainID:   uuid.New(),
		WorkflowID: uuid.New(),
		RunID:      uuid.New(),
		TaskID:     2,
		TaskType:   0,
	}
	err = s.PutReplicationTaskToDLQ(ctx, sourceCluster, taskInfo1)
	s.NoError(err)
	err = s.PutReplicationTaskToDLQ(ctx, sourceCluster, taskInfo2)
	s.NoError(err)
	resp, err = s.GetReplicationTasksFromDLQ(ctx, sourceCluster, 1, 3, 2, nil)
	s.NoError(err)
	s.Len(resp.Tasks, 2)
	sizeResp, err := s.GetReplicationDLQSize(ctx, sourceCluster)
	s.NoError(err)
	s.Equal(int64(2), sizeResp.Size)
	err = s.RangeDeleteReplicationTaskFromDLQ(ctx, sourceCluster, 1, 3)
	s.NoError(err)
	resp, err = s.GetReplicationTasksFromDLQ(ctx, sourceCluster, 1, 3, 2, nil)
	s.NoError(err)
	s.Len(resp.Tasks, 0)
}

// TestCreateFailoverMarkerTasks test
func (s *ExecutionManagerSuite) TestCreateFailoverMarkerTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	markers := []*p.FailoverMarkerTask{
		{
			TaskData: p.TaskData{
				TaskID:              1,
				VisibilityTimestamp: time.Now(),
				Version:             1,
			},
			DomainID: domainID,
		},
	}
	err := s.CreateFailoverMarkers(ctx, markers)
	s.NoError(err)

	tasks, err := s.GetReplicationTasks(ctx, 1, true)
	s.NoError(err)
	s.Equal(len(tasks), 1)
	s.Equal(tasks[0].GetVersion(), int64(1))
	s.Equal(tasks[0].GetTaskID(), int64(1))
	s.Equal(tasks[0].GetDomainID(), domainID)
	s.Equal(tasks[0].GetTaskType(), p.ReplicationTaskTypeFailoverMarker)
}

func copyWorkflowExecutionInfo(sourceInfo *p.WorkflowExecutionInfo) *p.WorkflowExecutionInfo {
	return &p.WorkflowExecutionInfo{
		DomainID:                    sourceInfo.DomainID,
		WorkflowID:                  sourceInfo.WorkflowID,
		RunID:                       sourceInfo.RunID,
		FirstExecutionRunID:         sourceInfo.FirstExecutionRunID,
		ParentDomainID:              sourceInfo.ParentDomainID,
		ParentWorkflowID:            sourceInfo.ParentWorkflowID,
		ParentRunID:                 sourceInfo.ParentRunID,
		InitiatedID:                 sourceInfo.InitiatedID,
		CompletionEvent:             sourceInfo.CompletionEvent,
		TaskList:                    sourceInfo.TaskList,
		TaskListKind:                sourceInfo.TaskListKind,
		WorkflowTypeName:            sourceInfo.WorkflowTypeName,
		WorkflowTimeout:             sourceInfo.WorkflowTimeout,
		DecisionStartToCloseTimeout: sourceInfo.DecisionStartToCloseTimeout,
		ExecutionContext:            sourceInfo.ExecutionContext,
		State:                       sourceInfo.State,
		CloseStatus:                 sourceInfo.CloseStatus,
		LastFirstEventID:            sourceInfo.LastFirstEventID,
		NextEventID:                 sourceInfo.NextEventID,
		LastProcessedEvent:          sourceInfo.LastProcessedEvent,
		LastUpdatedTimestamp:        sourceInfo.LastUpdatedTimestamp,
		StartTimestamp:              sourceInfo.StartTimestamp,
		CreateRequestID:             sourceInfo.CreateRequestID,
		DecisionVersion:             sourceInfo.DecisionVersion,
		DecisionScheduleID:          sourceInfo.DecisionScheduleID,
		DecisionStartedID:           sourceInfo.DecisionStartedID,
		DecisionRequestID:           sourceInfo.DecisionRequestID,
		DecisionTimeout:             sourceInfo.DecisionTimeout,
		BranchToken:                 sourceInfo.BranchToken,
		AutoResetPoints:             sourceInfo.AutoResetPoints,
		PartitionConfig:             sourceInfo.PartitionConfig,
	}
}

func copyExecutionStats(sourceStats *p.ExecutionStats) *p.ExecutionStats {
	return &p.ExecutionStats{
		HistorySize: sourceStats.HistorySize,
	}
}

// Note: cassandra only provide millisecond precision timestamp
// ref: https://docs.datastax.com/en/cql/3.3/cql/cql_reference/timestamp_type_r.html
// so to use equal function, we need to do conversion, getting rid of sub milliseconds
func timestampConvertor(t time.Time) time.Time {
	return time.Unix(
		0,
		p.DBTimestampToUnixNano(p.UnixNanoToDBTimestamp(t.UnixNano())),
	).UTC()
}

func timeComparator(t1, t2 time.Time, timeTolerance time.Duration) bool {
	diff := t2.Sub(t1)
	return diff.Nanoseconds() <= timeTolerance.Nanoseconds()
}
