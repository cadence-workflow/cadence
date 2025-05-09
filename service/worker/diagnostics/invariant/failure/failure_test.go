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

package failure

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

const (
	testDomain = "test-domain"
)

func Test__Check(t *testing.T) {
	metadata := FailureIssuesMetadata{
		Identity: "localhost",
	}
	metadataInBytes, err := json.Marshal(metadata)
	require.NoError(t, err)
	actMetadata := FailureIssuesMetadata{
		Identity:            "localhost",
		ActivityType:        "test-activity",
		ActivityScheduledID: 1,
		ActivityStartedID:   2,
	}
	actMetadataInBytes, err := json.Marshal(actMetadata)
	require.NoError(t, err)
	testCases := []struct {
		name           string
		testData       *types.GetWorkflowExecutionHistoryResponse
		expectedResult []invariant.InvariantCheckResult
		err            error
	}{
		{
			name:     "workflow execution timeout",
			testData: failedWfHistory(),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: ActivityFailed.String(),
					Reason:        GenericError.String(),
					Metadata:      actMetadataInBytes,
				},
				{
					IssueID:       2,
					InvariantType: ActivityFailed.String(),
					Reason:        PanicError.String(),
					Metadata:      actMetadataInBytes,
				},
				{
					IssueID:       3,
					InvariantType: ActivityFailed.String(),
					Reason:        CustomError.String(),
					Metadata:      actMetadataInBytes,
				},
				{
					IssueID:       4,
					InvariantType: WorkflowFailed.String(),
					Reason:        TimeoutError.String(),
					Metadata:      metadataInBytes,
				},
			},
			err: nil,
		},
		{
			name:     "blob size limit exceeded",
			testData: blobSizeLimitExceededHistory(),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: ActivityFailed.String(),
					Reason:        ActivityOutputBlobSizeLimit.String(),
					Metadata:      actMetadataInBytes,
				},
				{
					IssueID:       2,
					InvariantType: DecisionCausedFailure.String(),
					Reason:        DecisionBlobSizeLimit.String(),
					Metadata:      metadataInBytes,
				},
			},
			err: nil,
		},
	}
	for _, tc := range testCases {
		inv := NewInvariant()
		result, err := inv.Check(context.Background(), invariant.InvariantCheckInput{
			WorkflowExecutionHistory: tc.testData,
			Domain:                   testDomain,
		})
		require.Equal(t, tc.err, err)
		require.Equal(t, len(tc.expectedResult), len(result))
		require.ElementsMatch(t, tc.expectedResult, result)
	}
}

func failedWfHistory() *types.GetWorkflowExecutionHistoryResponse {
	return &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID: 1,
					ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
						ActivityID:   "101",
						ActivityType: &types.ActivityType{Name: "test-activity"},
					},
				},
				{
					ID: 2,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						Identity: "localhost",
						Attempt:  0,
					},
				},
				{
					ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{
						Reason:           common.StringPtr("cadenceInternal:Generic"),
						Details:          []byte("test-activity-failure"),
						Identity:         "localhost",
						ScheduledEventID: 1,
						StartedEventID:   2,
					},
				},
				{
					ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{
						Reason:           common.StringPtr("cadenceInternal:Panic"),
						Details:          []byte("test-activity-failure"),
						Identity:         "localhost",
						ScheduledEventID: 1,
						StartedEventID:   2,
					},
				},
				{
					ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{
						Reason:           common.StringPtr("custom error"),
						Details:          []byte("test-activity-failure"),
						Identity:         "localhost",
						ScheduledEventID: 1,
						StartedEventID:   2,
					},
				},
				{
					ID: 10,
					DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
						Identity: "localhost",
					},
				},
				{
					WorkflowExecutionFailedEventAttributes: &types.WorkflowExecutionFailedEventAttributes{
						Reason:                       common.StringPtr("cadenceInternal:Timeout START_TO_CLOSE"),
						Details:                      []byte("test-activity-failure"),
						DecisionTaskCompletedEventID: 10,
					},
				},
			},
		},
	}
}

func blobSizeLimitExceededHistory() *types.GetWorkflowExecutionHistoryResponse {
	return &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID: 1,
					ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
						ActivityID:   "101",
						ActivityType: &types.ActivityType{Name: "test-activity"},
					},
				},
				{
					ID: 2,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						Identity: "localhost",
						Attempt:  0,
					},
				},
				{
					ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{
						Reason:           common.StringPtr("COMPLETE_RESULT_EXCEEDS_LIMIT"),
						Details:          []byte("test-activity-failure"),
						Identity:         "localhost",
						ScheduledEventID: 1,
						StartedEventID:   2,
					},
				},
				{
					ID: 10,
					DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
						Identity: "localhost",
					},
				},
				{
					WorkflowExecutionFailedEventAttributes: &types.WorkflowExecutionFailedEventAttributes{
						Reason:                       common.StringPtr("DECISION_BLOB_SIZE_EXCEEDS_LIMIT"),
						Details:                      []byte("test-wf-failure"),
						DecisionTaskCompletedEventID: 10,
					},
				},
			},
		},
	}
}

func Test__RootCause(t *testing.T) {
	metadata := FailureIssuesMetadata{
		Identity: "localhost",
	}
	metadataInBytes, err := json.Marshal(metadata)
	require.NoError(t, err)
	testCases := []struct {
		name           string
		input          []invariant.InvariantCheckResult
		expectedResult []invariant.InvariantRootCauseResult
		err            error
	}{
		{
			name: "customer side known failure",
			input: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: ActivityFailed.String(),
					Reason:        CustomError.String(),
					Metadata:      metadataInBytes,
				}},
			expectedResult: []invariant.InvariantRootCauseResult{{
				IssueID:   1,
				RootCause: invariant.RootCauseTypeServiceSideCustomError,
				Metadata:  metadataInBytes,
			}},
			err: nil,
		},
		{
			name: "customer side error",
			input: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: ActivityFailed.String(),
					Reason:        GenericError.String(),
					Metadata:      metadataInBytes,
				}},
			expectedResult: []invariant.InvariantRootCauseResult{{
				IssueID:   1,
				RootCause: invariant.RootCauseTypeServiceSideIssue,
				Metadata:  metadataInBytes,
			}},
			err: nil,
		},
		{
			name: "customer side panic",
			input: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: ActivityFailed.String(),
					Reason:        PanicError.String(),
					Metadata:      metadataInBytes,
				}},
			expectedResult: []invariant.InvariantRootCauseResult{{
				IssueID:   1,
				RootCause: invariant.RootCauseTypeServiceSidePanic,
				Metadata:  metadataInBytes,
			}},
			err: nil,
		},
	}
	inv := NewInvariant()
	for _, tc := range testCases {
		result, err := inv.RootCause(context.Background(), invariant.InvariantRootCauseInput{
			Issues: tc.input,
		})
		require.Equal(t, tc.err, err)
		require.Equal(t, len(tc.expectedResult), len(result))
		require.ElementsMatch(t, tc.expectedResult, result)
	}
}
