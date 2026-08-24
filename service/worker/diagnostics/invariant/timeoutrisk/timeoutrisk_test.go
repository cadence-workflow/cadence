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

	explicitExpirationMetadata := ActivityRetryWindowExceedsStandbyDiscardDelayMetadata{
		EventID:              2,
		ActivityID:           "201",
		ActivityType:         "test-activity",
		EstimatedRetryWindow: 1800 * time.Second,
		Threshold:            1500 * time.Second,
		RetryPolicy: &types.RetryPolicy{
			ExpirationIntervalInSeconds: 1800,
			MaximumAttempts:             0,
		},
	}
	explicitExpirationMetadataInBytes, err := json.Marshal(explicitExpirationMetadata)
	require.NoError(t, err)

	unlimitedRidesToWfTimeoutMetadata := ActivityRetryWindowExceedsStandbyDiscardDelayMetadata{
		EventID:              2,
		ActivityID:           "202",
		ActivityType:         "test-activity",
		EstimatedRetryWindow: 3600 * time.Second,
		Threshold:            1500 * time.Second,
		RetryPolicy: &types.RetryPolicy{
			MaximumAttempts: 0,
		},
	}
	unlimitedRidesToWfTimeoutMetadataInBytes, err := json.Marshal(unlimitedRidesToWfTimeoutMetadata)
	require.NoError(t, err)

	boundedCumulativeMetadata := ActivityRetryWindowExceedsStandbyDiscardDelayMetadata{
		EventID:              2,
		ActivityID:           "203",
		ActivityType:         "test-activity",
		EstimatedRetryWindow: 6900 * time.Second,
		Threshold:            1500 * time.Second,
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds: 60,
			BackoffCoefficient:       2,
			MaximumIntervalInSeconds: 600,
			MaximumAttempts:          10,
		},
	}
	boundedCumulativeMetadataInBytes, err := json.Marshal(boundedCumulativeMetadata)
	require.NoError(t, err)

	allThreeAtCapMetadata := ActivityStartToCloseAtWorkflowTimeoutCapMetadata{
		EventID:             2,
		ActivityID:          "107",
		ActivityType:        "test-activity",
		StartToCloseTimeout: 1800 * time.Second,
		WorkflowTimeout:     1800 * time.Second,
	}
	allThreeAtCapMetadataInBytes, err := json.Marshal(allThreeAtCapMetadata)
	require.NoError(t, err)

	allThreeMissingHeartbeatMetadata := ActivityMissingHeartbeatTimeoutMetadata{
		EventID:             2,
		ActivityID:          "107",
		ActivityType:        "test-activity",
		StartToCloseTimeout: 1800 * time.Second,
		Threshold:           600 * time.Second,
	}
	allThreeMissingHeartbeatMetadataInBytes, err := json.Marshal(allThreeMissingHeartbeatMetadata)
	require.NoError(t, err)

	allThreeRetryWindowMetadata := ActivityRetryWindowExceedsStandbyDiscardDelayMetadata{
		EventID:              2,
		ActivityID:           "107",
		ActivityType:         "test-activity",
		EstimatedRetryWindow: 1800 * time.Second,
		Threshold:            1500 * time.Second,
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds: 1,
			MaximumAttempts:          0,
		},
	}
	allThreeRetryWindowMetadataInBytes, err := json.Marshal(allThreeRetryWindowMetadata)
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
			name: "retry window: explicit expiration exceeds threshold",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 7200),
						scheduledEvent(2, "201", "test-activity", 30, 5, 10, &types.RetryPolicy{
							ExpirationIntervalInSeconds: 1800,
							MaximumAttempts:             0,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityRetryWindowExceedsStandbyDiscardDelay.String(),
					Reason:        RetryWindowExceedsStandbyDiscardDelay.String(),
					Metadata:      explicitExpirationMetadataInBytes,
				},
			},
		},
		{
			name: "retry window: unlimited attempts ride out to the workflow timeout",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 3600),
						scheduledEvent(2, "202", "test-activity", 30, 5, 10, &types.RetryPolicy{
							MaximumAttempts: 0,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityRetryWindowExceedsStandbyDiscardDelay.String(),
					Reason:        RetryWindowExceedsStandbyDiscardDelay.String(),
					Metadata:      unlimitedRidesToWfTimeoutMetadataInBytes,
				},
			},
		},
		{
			name: "retry window: bounded attempts with large cumulative backoff",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 7200),
						scheduledEvent(2, "203", "test-activity", 300, 5, 10, &types.RetryPolicy{
							InitialIntervalInSeconds: 60,
							BackoffCoefficient:       2,
							MaximumIntervalInSeconds: 600,
							MaximumAttempts:          10,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityRetryWindowExceedsStandbyDiscardDelay.String(),
					Reason:        RetryWindowExceedsStandbyDiscardDelay.String(),
					Metadata:      boundedCumulativeMetadataInBytes,
				},
			},
		},
		{
			name: "retry window: explicit expiration below threshold - no issue",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 3600),
						scheduledEvent(2, "204", "test-activity", 30, 5, 10, &types.RetryPolicy{
							ExpirationIntervalInSeconds: 600,
							MaximumAttempts:             0,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "retry window: explicit expiration capped at workflow timeout - no issue",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 1200),
						scheduledEvent(2, "205", "test-activity", 30, 5, 10, &types.RetryPolicy{
							ExpirationIntervalInSeconds: 7200,
							MaximumAttempts:             0,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "retry window: single attempt cannot retry - no issue",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 3600),
						scheduledEvent(2, "206", "test-activity", 30, 5, 10, &types.RetryPolicy{
							ExpirationIntervalInSeconds: 7200,
							MaximumAttempts:             1,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "retry window: bounded attempts with small cumulative backoff - no issue",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 3600),
						scheduledEvent(2, "207", "test-activity", 60, 5, 10, &types.RetryPolicy{
							InitialIntervalInSeconds: 1,
							BackoffCoefficient:       2,
							MaximumAttempts:          2,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "retry window: unlimited attempts but no workflow started event - no issue",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						scheduledEvent(1, "208", "test-activity", 10, 5, 10, &types.RetryPolicy{
							MaximumAttempts: 0,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{},
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
			name: "all three risks fire on one scheduled event",
			testData: &types.GetWorkflowExecutionHistoryResponse{
				History: &types.History{
					Events: []*types.HistoryEvent{
						startedEvent(1, 1800),
						scheduledEvent(2, "107", "test-activity", 1800, 5, 0, &types.RetryPolicy{
							InitialIntervalInSeconds: 1,
							MaximumAttempts:          0,
						}),
					},
				},
			},
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityStartToCloseAtWorkflowTimeoutCap.String(),
					Reason:        StartToCloseAtWorkflowTimeoutCap.String(),
					Metadata:      allThreeAtCapMetadataInBytes,
				},
				{
					IssueID:       1,
					InvariantType: ActivityMissingHeartbeatTimeout.String(),
					Reason:        MissingHeartbeatTimeoutForLongActivity.String(),
					Metadata:      allThreeMissingHeartbeatMetadataInBytes,
				},
				{
					IssueID:       2,
					InvariantType: ActivityRetryWindowExceedsStandbyDiscardDelay.String(),
					Reason:        RetryWindowExceedsStandbyDiscardDelay.String(),
					Metadata:      allThreeRetryWindowMetadataInBytes,
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
