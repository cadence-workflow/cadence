package timeoutrisk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

func Test__Check(t *testing.T) {
	atCapMetadata := ActivityStartToCloseAtWorkflowTimeoutCapMetadata{
		EventID:             2,
		ActivityID:          "101",
		ActivityType:        "test-activity",
		StartToCloseTimeout: 60 * time.Second,
		WorkflowTimeout:     60 * time.Second,
	}
	atCapMetadataInBytes, err := json.Marshal(atCapMetadata)
	require.NoError(t, err)

	missingHeartbeatMetadata := ActivityMissingHeartbeatTimeoutMetadata{
		EventID:             2,
		ActivityID:          "102",
		ActivityType:        "test-activity",
		StartToCloseTimeout: 900 * time.Second,
		Threshold:           600 * time.Second,
	}
	missingHeartbeatMetadataInBytes, err := json.Marshal(missingHeartbeatMetadata)
	require.NoError(t, err)

	twoIssuesAtCapMetadata := ActivityStartToCloseAtWorkflowTimeoutCapMetadata{
		EventID:             2,
		ActivityID:          "107",
		ActivityType:        "test-activity",
		StartToCloseTimeout: 1800 * time.Second,
		WorkflowTimeout:     1800 * time.Second,
	}
	twoIssuesAtCapMetadataInBytes, err := json.Marshal(twoIssuesAtCapMetadata)
	require.NoError(t, err)

	twoIssuesMissingHeartbeatMetadata := ActivityMissingHeartbeatTimeoutMetadata{
		EventID:             2,
		ActivityID:          "107",
		ActivityType:        "test-activity",
		StartToCloseTimeout: 1800 * time.Second,
		Threshold:           600 * time.Second,
	}
	twoIssuesMissingHeartbeatMetadataInBytes, err := json.Marshal(twoIssuesMissingHeartbeatMetadata)
	require.NoError(t, err)

	secondActivityRiskyMetadata := ActivityStartToCloseAtWorkflowTimeoutCapMetadata{
		EventID:             3,
		ActivityID:          "108b",
		ActivityType:        "test-activity",
		StartToCloseTimeout: 60 * time.Second,
		WorkflowTimeout:     60 * time.Second,
	}
	secondActivityRiskyMetadataInBytes, err := json.Marshal(secondActivityRiskyMetadata)
	require.NoError(t, err)

	testCases := []struct {
		name           string
		testData       *types.GetWorkflowExecutionHistoryResponse
		expectedResult []invariant.InvariantCheckResult
	}{
		{
			name: "activity StartToClose at workflow timeout cap (capped equality)",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 60),
						scheduledEvent(2, "101", "test-activity", 60, 10, 30, nil),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityStartToCloseAtWorkflowTimeoutCap.String(),
					Reason:        StartToCloseAtWorkflowTimeoutCap.String(),
					Metadata:      atCapMetadataInBytes,
				},
			},
		},
		{
			name: "activity StartToClose just below workflow timeout - no issue",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 60),
						scheduledEvent(2, "111", "test-activity", 59, 10, 30, &types.RetryPolicy{
							InitialIntervalInSeconds: 1,
							MaximumAttempts:          1,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "long-running activity missing heartbeat timeout",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 3600),
						scheduledEvent(2, "102", "test-activity", 900, 10, 0, nil),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityMissingHeartbeatTimeout.String(),
					Reason:        MissingHeartbeatTimeoutForLongActivity.String(),
					Metadata:      missingHeartbeatMetadataInBytes,
				},
			},
		},
		{
			name: "well-configured activity",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 110),
						scheduledEvent(2, "106", "test-activity", 10, 5, 5, &types.RetryPolicy{
							InitialIntervalInSeconds: 1,
							MaximumAttempts:          1,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "both risks fire on one scheduled event",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 1800),
						scheduledEvent(2, "107", "test-activity", 1800, 5, 0, nil),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityStartToCloseAtWorkflowTimeoutCap.String(),
					Reason:        StartToCloseAtWorkflowTimeoutCap.String(),
					Metadata:      twoIssuesAtCapMetadataInBytes,
				},
				{
					IssueID:       1,
					InvariantType: ActivityMissingHeartbeatTimeout.String(),
					Reason:        MissingHeartbeatTimeoutForLongActivity.String(),
					Metadata:      twoIssuesMissingHeartbeatMetadataInBytes,
				},
			},
		},
		{
			name: "two activities, only the second is risky",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 60),
						scheduledEvent(2, "108a", "test-activity", 10, 5, 5, &types.RetryPolicy{
							InitialIntervalInSeconds: 1,
							MaximumAttempts:          1,
						}),
						scheduledEvent(3, "108b", "test-activity", 60, 10, 30, nil),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityStartToCloseAtWorkflowTimeoutCap.String(),
					Reason:        StartToCloseAtWorkflowTimeoutCap.String(),
					Metadata:      secondActivityRiskyMetadataInBytes,
				},
			},
		},
		{
			name: "no workflow started event in history - check 1 must not false-positive on a zero baseline",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						scheduledEvent(1, "109", "test-activity", 10, 5, 5, &types.RetryPolicy{
							InitialIntervalInSeconds: 1,
							MaximumAttempts:          1,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inv := NewInvariant()
			result, err := inv.Check(context.Background(), invariant.InvariantCheckInput{
				WorkflowExecutionHistory: tc.testData,
			})
			require.NoError(t, err)
			require.Equal(t, len(tc.expectedResult), len(result))
			require.ElementsMatch(t, tc.expectedResult, result)
		})
	}
}

func Test__RootCause(t *testing.T) {
	inv := NewInvariant()
	result, err := inv.RootCause(context.Background(), invariant.InvariantRootCauseInput{})
	require.NoError(t, err)
	require.Empty(t, result)
}

func startedEvent(id int64, workflowTimeoutSeconds int32) *types.HistoryEvent {
	return &types.HistoryEvent{
		ID: id,
		WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(workflowTimeoutSeconds),
		},
	}
}

func scheduledEvent(id int64, activityID, activityType string, startToCloseSeconds, scheduleToStartSeconds, heartbeatSeconds int32, retryPolicy *types.RetryPolicy) *types.HistoryEvent {
	return &types.HistoryEvent{
		ID: id,
		ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
			ActivityID:                    activityID,
			ActivityType:                  &types.ActivityType{Name: activityType},
			StartToCloseTimeoutSeconds:    common.Int32Ptr(startToCloseSeconds),
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(scheduleToStartSeconds),
			HeartbeatTimeoutSeconds:       common.Int32Ptr(heartbeatSeconds),
			RetryPolicy:                   retryPolicy,
		},
	}
}
