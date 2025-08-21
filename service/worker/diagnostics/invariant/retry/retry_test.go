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

package retry

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

func Test__Check(t *testing.T) {
	invalidAttemptsMetadata := RetryMetadata{
		EventID: 5,
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds: 1,
			MaximumAttempts:          1,
		},
	}
	invalidAttemptsMetadataInBytes, err := json.Marshal(invalidAttemptsMetadata)
	require.NoError(t, err)
	invalidExpIntervalMetadata := RetryMetadata{
		EventID: 1,
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    10,
			ExpirationIntervalInSeconds: 5,
		},
	}
	invalidExpIntervalMetadataInBytes, err := json.Marshal(invalidExpIntervalMetadata)
	require.NoError(t, err)
	invalidHBMetadata := RetryMetadata{
		EventID: 6,
	}
	invalidHBMetadataInBytes, err := json.Marshal(invalidHBMetadata)
	require.NoError(t, err)
	testCases := []struct {
		name           string
		testData       *types.GetWorkflowExecutionHistoryResponse
		expectedResult []invariant.InvariantCheckResult
		err            error
	}{
		{
			name:     "invalid retry policy",
			testData: invalidRetryPolicyWfHistory(),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: ActivityRetryIssue.String(),
					Reason:        RetryPolicyValidationMaxAttempts.String(),
					Metadata:      invalidAttemptsMetadataInBytes,
				},
				{
					IssueID:       0,
					InvariantType: WorkflowRetryIssue.String(),
					Reason:        RetryPolicyValidationExpInterval.String(),
					Metadata:      invalidExpIntervalMetadataInBytes,
				},
				{
					IssueID:       2,
					InvariantType: ActivityHeartbeatIssue.String(),
					Reason:        HeartBeatTimeoutEqualToStartToCloseTimeout.String(),
					Metadata:      invalidHBMetadataInBytes,
				},
			},
			err: nil,
		},
	}
	for _, tc := range testCases {
		inv := NewInvariant()
		result, err := inv.Check(context.Background(), invariant.InvariantCheckInput{
			WorkflowExecutionHistory: tc.testData,
		})
		require.Equal(t, tc.err, err)
		require.Equal(t, len(tc.expectedResult), len(result))
		require.ElementsMatch(t, tc.expectedResult, result)
	}
}

func retriedWfHistory() *types.GetWorkflowExecutionHistoryResponse {
	return &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID: 1,
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						RetryPolicy: &types.RetryPolicy{
							InitialIntervalInSeconds: 1,
							MaximumAttempts:          2,
						},
						Attempt: 0,
					},
				},
				{
					WorkflowExecutionContinuedAsNewEventAttributes: &types.WorkflowExecutionContinuedAsNewEventAttributes{
						FailureReason:                common.StringPtr("cadenceInternal:Timeout START_TO_CLOSE"),
						DecisionTaskCompletedEventID: 10,
					},
				},
			},
		},
	}
}

func invalidRetryPolicyWfHistory() *types.GetWorkflowExecutionHistoryResponse {
	timeout := int32(5)
	return &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID: 1,
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						RetryPolicy: &types.RetryPolicy{
							InitialIntervalInSeconds:    10,
							ExpirationIntervalInSeconds: 5,
						},
					},
				},
				{
					ID: 5,
					ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
						HeartbeatTimeoutSeconds:    common.Int32Ptr(timeout),
						StartToCloseTimeoutSeconds: common.Int32Ptr(timeout * 2),
						RetryPolicy: &types.RetryPolicy{
							InitialIntervalInSeconds: 1,
							MaximumAttempts:          1,
						},
					},
				},
				{
					ID: 6,
					ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
						HeartbeatTimeoutSeconds:    common.Int32Ptr(timeout),
						StartToCloseTimeoutSeconds: common.Int32Ptr(timeout),
						RetryPolicy:                nil,
					},
				},
			},
		},
	}
}
