// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package engineimpl

import (
	ctx "context"
	"fmt"
	"testing"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/reset"
)

var (
	testRequestID                = "this is a test request"
	testRequestReason            = "Test reason"
	testRequestSkipSignalReapply = true
	latestRunID                  = constants.TestRunID
	latestExecution              = &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: latestRunID}
	previousRunID                = "bbbbbeef-0123-4567-890a-bcdef0123456"
	previousExecution            = &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: previousRunID}

	version         = int64(12)
	branchToken     = []byte("other random branch token")
	partitionConfig = map[string]string{
		"userid": uuid.New(),
	}
	firstEventID   = commonconstants.FirstEventID
	workflowEvents = []*types.HistoryEvent{
		{
			ID:        1,
			Version:   version,
			EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
			WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
				WorkflowType:                        &types.WorkflowType{Name: "some random workflow type"},
				TaskList:                            &types.TaskList{Name: "some random workflow type"},
				Input:                               []byte("some random input"),
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(123),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(233),
				Identity:                            "some random identity",
				PartitionConfig:                     partitionConfig,
			},
		},
		{
			ID:                                   2,
			Version:                              version,
			EventType:                            types.EventTypeDecisionTaskScheduled.Ptr(),
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
		},
		{
			ID:        3,
			Version:   version,
			EventType: types.EventTypeDecisionTaskStarted.Ptr(),
			DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
				ScheduledEventID: 2,
			},
		},
		{
			ID:        4,
			Version:   version,
			EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
			DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				ScheduledEventID: 2,
				StartedEventID:   3,
			},
		},
		{
			ID:        5,
			Version:   version,
			EventType: types.EventTypeActivityTaskScheduled.Ptr(),
			ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
				ActivityID: "1",
			},
		},
		{
			ID:        6,
			Version:   version,
			EventType: types.EventTypeActivityTaskStarted.Ptr(),
			ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
				ScheduledEventID: 5,
			},
		},
		{
			ID:        7,
			Version:   version,
			EventType: types.EventTypeActivityTaskCompleted.Ptr(),
			ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{
				ScheduledEventID: 5,
				StartedEventID:   6,
			},
		},
		{
			ID:                                   8,
			Version:                              version,
			EventType:                            types.EventTypeDecisionTaskScheduled.Ptr(),
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
		},
		{
			ID:        9,
			Version:   version,
			EventType: types.EventTypeDecisionTaskTimedOut.Ptr(),
			DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
				ScheduledEventID: 8,
			},
		},
		{
			ID:        10,
			Version:   version,
			EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
				TaskList: &types.TaskList{Name: "some random workflow type"},
			},
		},
		{
			ID:        11,
			Version:   version,
			EventType: types.EventTypeDecisionTaskStarted.Ptr(),
			DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
				ScheduledEventID: 10,
			},
		},
		{
			ID:        12,
			Version:   version,
			EventType: types.EventTypeDecisionTaskFailed.Ptr(),
			DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
				ScheduledEventID: 10,
				StartedEventID:   11,
			},
		},
		{
			ID:                                   13,
			Version:                              version,
			EventType:                            types.EventTypeDecisionTaskScheduled.Ptr(),
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
		},
		{
			ID:        14,
			Version:   version,
			EventType: types.EventTypeDecisionTaskStarted.Ptr(),
			DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
				ScheduledEventID: 13,
			},
		},
		{
			ID:        15,
			Version:   version,
			EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
			DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				ScheduledEventID: 13,
				StartedEventID:   14,
			},
		},
		{
			ID:                          16,
			Version:                     version,
			EventType:                   types.EventTypeTimerStarted.Ptr(),
			TimerStartedEventAttributes: &types.TimerStartedEventAttributes{},
		},
		{
			ID:                        17,
			Version:                   version,
			EventType:                 types.EventTypeTimerFired.Ptr(),
			TimerFiredEventAttributes: &types.TimerFiredEventAttributes{},
		},
		{
			ID:                                       18,
			Version:                                  version,
			EventType:                                types.EventTypeWorkflowExecutionSignaled.Ptr(),
			WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{},
		},
		{
			ID:                                   19,
			Version:                              version,
			EventType:                            types.EventTypeActivityTaskScheduled.Ptr(),
			ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{},
		},
		{
			ID:        20,
			Version:   version,
			EventType: types.EventTypeActivityTaskStarted.Ptr(),
			ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
				ScheduledEventID: 19,
			},
		},
		{
			ID:        21,
			Version:   version,
			EventType: types.EventTypeActivityTaskFailed.Ptr(),
			ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{
				ScheduledEventID: 19,
				StartedEventID:   20,
			},
		},
		{
			ID:                                   22,
			Version:                              version,
			EventType:                            types.EventTypeDecisionTaskScheduled.Ptr(),
			DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
		},
		{
			ID:        23,
			Version:   version,
			EventType: types.EventTypeDecisionTaskStarted.Ptr(),
			DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{
				ScheduledEventID: 22,
			},
		},
		{
			ID:        24,
			Version:   version,
			EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
			DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
				ScheduledEventID: 22,
				StartedEventID:   23,
			},
		},
		{
			ID:        25,
			Version:   version,
			EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
			WorkflowExecutionCompletedEventAttributes: &types.WorkflowExecutionCompletedEventAttributes{},
		},
	}

	history = []testHistoryEvents{
		{
			Events: workflowEvents[:2],
			Size:   1,
		},
		{
			Events: workflowEvents[2:3],
			Size:   2,
		},
		{
			Events: workflowEvents[3:6],
			Size:   3,
		},
		{
			Events: workflowEvents[6:8],
			Size:   4,
		},
		{
			Events: workflowEvents[8:10],
			Size:   5,
		},
		{
			Events: workflowEvents[10:11],
			Size:   6,
		},
		{
			Events: workflowEvents[11:13],
			Size:   7,
		},
		{
			Events: workflowEvents[13:14],
			Size:   8,
		},
		{
			Events: workflowEvents[14:16],
			Size:   9,
		},
		{
			Events: workflowEvents[16:19],
			Size:   10,
		},
		{
			Events: workflowEvents[19:22],
			Size:   11,
		},
		{
			Events: workflowEvents[22:23],
			Size:   12,
		},
		{
			Events: workflowEvents[23:24],
			Size:   13,
		},
		{
			Events: workflowEvents[24:25],
			Size:   14,
		},
	}
)

type (
	InitFn func(t *testing.T, engine *testdata.EngineForTest)

	testHistoryEvents struct {
		Events []*types.HistoryEvent
		Size   int
	}
)

func TestResetWorkflowExecution(t *testing.T) {
	cases := []struct {
		name        string
		request     *types.HistoryResetWorkflowExecutionRequest
		init        []InitFn
		expectedErr error
		expected    *types.ResetWorkflowExecutionResponse
	}{
		{
			name:    "No processed or pending decision",
			request: resetExecutionRequest(latestExecution, 100),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:           constants.TestDomainID,
						WorkflowID:         constants.TestWorkflowID,
						RunID:              latestRunID,
						DecisionScheduleID: commonconstants.EmptyEventID,
						LastProcessedEvent: commonconstants.EmptyEventID,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
			},
			expectedErr: &types.BadRequestError{Message: "Cannot reset workflow without a decision task schedule."},
		},
		{
			name:    "Invalid DecisionFinishEventId",
			request: resetExecutionRequest(latestExecution, 100),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 26,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
			},
			expectedErr: &types.BadRequestError{Message: "Decision finish ID must be > 1 && <= workflow next event ID."},
		},
		{
			name:    "Duplicate Request",
			request: resetExecutionRequest(latestExecution, 24),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:        constants.TestDomainID,
						WorkflowID:      constants.TestWorkflowID,
						RunID:           latestRunID,
						NextEventID:     26,
						CreateRequestID: testRequestID,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
			},
			expected: &types.ResetWorkflowExecutionResponse{
				RunID: latestRunID,
			},
		},
		{
			name:    "Success",
			request: resetExecutionRequest(latestExecution, 24),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 26,
						BranchToken: branchToken,
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: version,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				withHistoryPagination(branchToken, 24),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(latestExecution.RunID),
						gomock.Eq(branchToken),
						gomock.Eq(int64(24)-1), // Request.DecisionFinishEventID - 1
						gomock.Eq(version),     // CurrentVersion
						gomock.Eq(int64(26)),   // NextEventID
						gomock.Any(),           // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(nil).Times(1)
				},
			},
			// Can't assert on the result because the runID is random
		},
		{
			name: "Success using version histories started in current cluster",
			// This corresponds to VersionHistories.Histories.Items.EventID
			request: resetExecutionRequest(latestExecution, 24),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 26,
						BranchToken: branchToken,
					},
					VersionHistories: &persistence.VersionHistories{
						CurrentVersionHistoryIndex: 1,
						Histories: []*persistence.VersionHistory{
							{
								BranchToken: []byte("some other branch token"),
							},
							{
								BranchToken: branchToken,
								Items: []*persistence.VersionHistoryItem{
									{
										EventID: 1,
										Version: 1000, // current cluster
									},
									{
										EventID: 23,
										Version: 1001,
									},
									{
										EventID: 24,
										Version: 1002,
									},
								},
							},
						},
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				withHistoryPagination(branchToken, 24),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(latestExecution.RunID),
						gomock.Eq(branchToken), //VersionHistories.Histories.BranchToken
						gomock.Eq(int64(23)),   // Request.DecisionFinishEventID - 1
						gomock.Eq(int64(1001)), // VersionHistories.Histories.Items.Version
						gomock.Eq(int64(26)),   // NextEventID
						gomock.Any(),           // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(nil).Times(1)
				},
			},
			// Can't assert on the result because the runID is random
		},
		{
			name:    "Success using previous version",
			request: resetExecutionRequest(previousExecution, 24),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      latestRunID,
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: 1,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				withState(previousExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 26,
						BranchToken: branchToken,
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: version,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				withHistoryPagination(branchToken, 24),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(previousExecution.RunID),
						gomock.Eq(branchToken),
						gomock.Eq(int64(23)), // Request.DecisionFinishEventID - 1
						gomock.Eq(version),   // CurrentVersion
						gomock.Eq(int64(26)), // NextEventID
						gomock.Any(),         // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(nil).Times(1)
				},
			},
			// Can't assert on the result because the runID is random
		},
		{
			name:    "Persistence Duplicate Request",
			request: resetExecutionRequest(latestExecution, 24),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 26,
						BranchToken: branchToken,
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: version,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				withHistoryPagination(branchToken, 24),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(latestExecution.RunID),
						gomock.Eq(branchToken),
						gomock.Eq(int64(23)), // Request.DecisionFinishEventID - 1
						gomock.Eq(version),   // CurrentVersion
						gomock.Eq(int64(26)), // NextEventID
						gomock.Any(),         // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(&persistence.DuplicateRequestError{
						RequestType: persistence.WorkflowRequestTypeReset,
						RunID:       "errorID",
					}).Times(1)
				},
			},
			expected: &types.ResetWorkflowExecutionResponse{
				RunID: "errorID",
			},
		},
		{
			name:    "Persistence Duplicate Request Bug",
			request: resetExecutionRequest(latestExecution, 24),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 26,
						BranchToken: branchToken,
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: version,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				withHistoryPagination(branchToken, 24),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(latestExecution.RunID),
						gomock.Eq(branchToken),
						gomock.Eq(int64(23)), // Request.DecisionFinishEventID - 1
						gomock.Eq(version),   // CurrentVersion
						gomock.Eq(int64(26)), // NextEventID
						gomock.Any(),         // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(&persistence.DuplicateRequestError{
						RequestType: persistence.WorkflowRequestTypeStart,
						RunID:       "errorID",
					}).Times(1)
				},
			},
			expectedErr: &persistence.DuplicateRequestError{
				RequestType: persistence.WorkflowRequestTypeStart,
				RunID:       "errorID",
			},
		},
		{
			name:    "Reset returns Err",
			request: resetExecutionRequest(latestExecution, 24),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 26,
						BranchToken: branchToken,
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: version,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				withHistoryPagination(branchToken, 24),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(latestExecution.RunID),
						gomock.Eq(branchToken),
						gomock.Eq(int64(23)), // Request.DecisionFinishEventID - 1
						gomock.Eq(version),   // CurrentVersion
						gomock.Eq(int64(26)), // NextEventID
						gomock.Any(),         // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(&types.BadRequestError{
						Message: "didn't work",
					}).Times(1)
				},
			},
			expectedErr: &types.BadRequestError{
				Message: "didn't work",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			for _, setup := range testCase.init {
				setup(t, eft)
			}
			eft.Engine.Start()
			result, err := eft.Engine.ResetWorkflowExecution(ctx.Background(), testCase.request)
			eft.Engine.Stop()

			if testCase.expectedErr == nil {
				if assert.NotNil(t, result) {
					assert.NotEmpty(t, result.RunID)
				}
				assert.NoError(t, err)
			} else {
				assert.Nil(t, result)
				assert.Equal(t, testCase.expectedErr, err)
			}

		})
	}
}

func TestResetWorkflowExecution_ResetPointsValidation(t *testing.T) {

	testCases := []struct {
		name         string
		resetEventID int64
		setupMock    func(mockResetter *reset.MockWorkflowResetter, resetEventID int64)
		err          error
	}{
		{
			name:         "error - reset on decision task scheduled",
			resetEventID: 10,
			setupMock:    func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[9].EventType.String()),
			},
		},
		{
			name:         "error - reset on activity task scheduled",
			setupMock:    func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {},
			resetEventID: 5,
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[4].EventType.String()),
			},
		},
		{
			name:         "error - reset on activity task started",
			setupMock:    func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {},
			resetEventID: 6,
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[5].EventType.String()),
			},
		},
		{
			name:         "error - reset on activity task completed",
			setupMock:    func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {},
			resetEventID: 7,
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[6].EventType.String()),
			},
		},
		{
			name:         "error - reset on timer started",
			setupMock:    func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {},
			resetEventID: 16,
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[15].EventType.String()),
			},
		},
		{
			name:         "error - reset on timer fired",
			setupMock:    func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {},
			resetEventID: 17,
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[16].EventType.String()),
			},
		},
		{
			name:         "error - reset on workflow execution signaled",
			setupMock:    func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {},
			resetEventID: 18,
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[17].EventType.String()),
			},
		},
		{
			name:         "error - reset on activity task failed",
			setupMock:    func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {},
			resetEventID: 21,
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[20].EventType.String()),
			},
		},
		{
			name:         "error - reset on workflow execution completed",
			setupMock:    func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {},
			resetEventID: 25,
			err: &types.BadRequestError{
				Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", workflowEvents[24].EventType.String()),
			},
		},
		{
			name: "success - reset on decision task started",
			setupMock: func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {
				mockResetter.EXPECT().ResetWorkflow(
					gomock.Any(), // Context
					gomock.Eq(constants.TestDomainID),
					gomock.Eq(constants.TestWorkflowID),
					gomock.Eq(latestExecution.RunID),
					gomock.Eq(branchToken),
					gomock.Eq(resetEventID-1), // Request.DecisionFinishEventID - 1
					gomock.Eq(version),        // CurrentVersion
					gomock.Eq(int64(26)),      // NextEventID
					gomock.Any(),              // random uuid
					gomock.Eq(testRequestID),
					&workflowMatcher{latestExecution},
					gomock.Eq(testRequestReason),
					gomock.Nil(),
					gomock.Eq(testRequestSkipSignalReapply),
				).Return(nil).Times(1)
			},
			resetEventID: 23,
			err:          nil,
		},
		{
			name: "success - reset on decision task timed out",
			setupMock: func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {
				mockResetter.EXPECT().ResetWorkflow(
					gomock.Any(), // Context
					gomock.Eq(constants.TestDomainID),
					gomock.Eq(constants.TestWorkflowID),
					gomock.Eq(latestExecution.RunID),
					gomock.Eq(branchToken),
					gomock.Eq(resetEventID-1), // Request.DecisionFinishEventID - 1
					gomock.Eq(version),        // CurrentVersion
					gomock.Eq(int64(26)),      // NextEventID
					gomock.Any(),              // random uuid
					gomock.Eq(testRequestID),
					&workflowMatcher{latestExecution},
					gomock.Eq(testRequestReason),
					gomock.Nil(),
					gomock.Eq(testRequestSkipSignalReapply),
				).Return(nil).Times(1)
			},
			resetEventID: 9,
			err:          nil,
		},
		{
			name: "success - reset on decision task failed",
			setupMock: func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {
				mockResetter.EXPECT().ResetWorkflow(
					gomock.Any(), // Context
					gomock.Eq(constants.TestDomainID),
					gomock.Eq(constants.TestWorkflowID),
					gomock.Eq(latestExecution.RunID),
					gomock.Eq(branchToken),
					gomock.Eq(resetEventID-1), // Request.DecisionFinishEventID - 1
					gomock.Eq(version),        // CurrentVersion
					gomock.Eq(int64(26)),      // NextEventID
					gomock.Any(),              // random uuid
					gomock.Eq(testRequestID),
					&workflowMatcher{latestExecution},
					gomock.Eq(testRequestReason),
					gomock.Nil(),
					gomock.Eq(testRequestSkipSignalReapply),
				).Return(nil).Times(1)
			},
			resetEventID: 12,
			err:          nil,
		},
		{
			name: "success - reset on decision task completed",
			setupMock: func(mockResetter *reset.MockWorkflowResetter, resetEventID int64) {
				mockResetter.EXPECT().ResetWorkflow(
					gomock.Any(), // Context
					gomock.Eq(constants.TestDomainID),
					gomock.Eq(constants.TestWorkflowID),
					gomock.Eq(latestExecution.RunID),
					gomock.Eq(branchToken),
					gomock.Eq(resetEventID-1), // Request.DecisionFinishEventID - 1
					gomock.Eq(version),        // CurrentVersion
					gomock.Eq(int64(26)),      // NextEventID
					gomock.Any(),              // random uuid
					gomock.Eq(testRequestID),
					&workflowMatcher{latestExecution},
					gomock.Eq(testRequestReason),
					gomock.Nil(),
					gomock.Eq(testRequestSkipSignalReapply),
				).Return(nil).Times(1)
			},
			resetEventID: 24,
			err:          nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			pageToken := []byte("some random pagination token")

			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)

			request := resetExecutionRequest(latestExecution, int(tc.resetEventID))

			mockResetter := reset.NewMockWorkflowResetter(ctrl)
			eft.Engine.(*historyEngineImpl).workflowResetter = mockResetter

			tc.setupMock(mockResetter, tc.resetEventID)

			withHistoryPagination(pageToken, tc.resetEventID)(t, eft)

			withState(latestExecution, &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:           constants.TestDomainID,
					WorkflowID:         constants.TestWorkflowID,
					RunID:              latestRunID,
					DecisionScheduleID: commonconstants.EmptyEventID,
					LastProcessedEvent: 25,
					NextEventID:        26,
				},
				ExecutionStats: &persistence.ExecutionStats{HistorySize: 100},
				VersionHistories: &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories: []*persistence.VersionHistory{
						{
							BranchToken: branchToken,
							Items: []*persistence.VersionHistoryItem{
								{
									EventID: 25,
									Version: version,
								},
							},
						},
					},
				},
			})(t, eft)

			withCurrentExecution(latestExecution)(t, eft)

			eft.Engine.Start()
			result, err := eft.Engine.ResetWorkflowExecution(ctx.Background(), request)
			eft.Engine.Stop()

			if tc.err != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tc.err.Error())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func withHistoryPagination(pageToken []byte, resetEventID int64) InitFn {
	return func(_ *testing.T, engine *testdata.EngineForTest) {
		counter := int64(0)
		historySize := 0
		for index, historyBatch := range history {
			token := pageToken

			if index == 0 {
				token = nil
			}

			counter += int64(len(historyBatch.Events))

			if counter >= resetEventID {
				pageToken = nil
			}

			engine.ShardCtx.GetHistoryManager().(*mocks.HistoryV2Manager).On("ReadHistoryBranchByBatch", mock.Anything, &persistence.ReadHistoryBranchRequest{
				BranchToken:   branchToken,
				MinEventID:    firstEventID,
				MaxEventID:    resetEventID + 1, // Rebuild adds 1 to nextEventID
				PageSize:      DefaultPageSize,
				NextPageToken: token,
				ShardID:       common.IntPtr(engine.ShardCtx.GetShardID()),
				DomainName:    constants.TestDomainName,
			}).Return(&persistence.ReadHistoryBranchByBatchResponse{
				History:       []*types.History{{Events: historyBatch.Events}},
				NextPageToken: pageToken,
				Size:          historyBatch.Size,
			}, nil).Once()

			historySize += historyBatch.Size

			if counter >= resetEventID {
				break
			}
		}
	}
}

func withCurrentExecution(execution *types.WorkflowExecution) InitFn {
	return func(_ *testing.T, engine *testdata.EngineForTest) {
		engine.ShardCtx.Resource.ExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(&persistence.GetCurrentExecutionResponse{
			StartRequestID: "CurrentExecutionStartRequestID",
			RunID:          execution.RunID,
			// Other fields don't matter
		}, nil)
	}
}

func withState(execution *types.WorkflowExecution, state *persistence.WorkflowMutableState) InitFn {
	return func(_ *testing.T, engine *testdata.EngineForTest) {
		engine.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.GetWorkflowExecutionRequest) bool {
			return req.Execution == *execution
		})).Return(&persistence.GetWorkflowExecutionResponse{
			State: state,
		}, nil)
	}
}

func resetExecutionRequest(execution *types.WorkflowExecution, decisionFinishEventID int) *types.HistoryResetWorkflowExecutionRequest {
	return &types.HistoryResetWorkflowExecutionRequest{
		DomainUUID: constants.TestDomainID,
		ResetRequest: &types.ResetWorkflowExecutionRequest{
			Domain:                constants.TestDomainName,
			WorkflowExecution:     execution,
			Reason:                testRequestReason,
			DecisionFinishEventID: int64(decisionFinishEventID),
			RequestID:             testRequestID,
			SkipSignalReapply:     testRequestSkipSignalReapply,
		},
	}
}

type workflowMatcher struct {
	execution *types.WorkflowExecution
}

func (m *workflowMatcher) Matches(obj interface{}) bool {
	if ex, ok := obj.(execution.Workflow); ok {
		executionInfo := ex.GetMutableState().GetExecutionInfo()
		return executionInfo.WorkflowID == m.execution.WorkflowID && executionInfo.RunID == m.execution.RunID
	}
	return false
}

func (m *workflowMatcher) String() string {
	return fmt.Sprintf("Workflow with WorkflowID %s and RunID %s", m.execution.WorkflowID, m.execution.RunID)
}
