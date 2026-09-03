package antipatterns

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

const (
	testTimestamp    = int64(1700000000000000000)
	testCronSchedule = "*/5 * * * *"
	testStepNanos    = int64(time.Millisecond)
)

func Test__Check(t *testing.T) {
	windowNanos := activityBurstWindowInSeconds * int64(time.Second)

	burstMetadata := ActivityScheduleBurstMetadata{
		FirstEventID:    2,
		LastEventID:     2 + int64(activityBurstCountThreshold) - 1,
		EventCount:      activityBurstCountThreshold,
		WindowStart:     time.Unix(0, testTimestamp).UTC(),
		WindowEnd:       time.Unix(0, testTimestamp+int64(activityBurstCountThreshold-1)*testStepNanos).UTC(),
		WindowInSeconds: activityBurstWindowInSeconds,
		Threshold:       activityBurstCountThreshold,
	}
	burstMetadataInBytes, err := json.Marshal(burstMetadata)
	require.NoError(t, err)

	burstAtWindowEdgeMetadata := ActivityScheduleBurstMetadata{
		FirstEventID:    2,
		LastEventID:     2 + int64(activityBurstCountThreshold) - 1,
		EventCount:      activityBurstCountThreshold,
		WindowStart:     time.Unix(0, testTimestamp).UTC(),
		WindowEnd:       time.Unix(0, testTimestamp+windowNanos).UTC(),
		WindowInSeconds: activityBurstWindowInSeconds,
		Threshold:       activityBurstCountThreshold,
	}
	burstAtWindowEdgeMetadataInBytes, err := json.Marshal(burstAtWindowEdgeMetadata)
	require.NoError(t, err)

	secondBurstMetadata := ActivityScheduleBurstMetadata{
		FirstEventID:    100,
		LastEventID:     100 + int64(activityBurstCountThreshold) + 4,
		EventCount:      activityBurstCountThreshold + 5,
		WindowStart:     time.Unix(0, testTimestamp+int64(time.Hour)).UTC(),
		WindowEnd:       time.Unix(0, testTimestamp+int64(time.Hour)+int64(activityBurstCountThreshold+4)*testStepNanos).UTC(),
		WindowInSeconds: activityBurstWindowInSeconds,
		Threshold:       activityBurstCountThreshold,
	}
	secondBurstMetadataInBytes, err := json.Marshal(secondBurstMetadata)
	require.NoError(t, err)

	const sustainedBurstCount = 100
	const sustainedBurstStepNanos = int64(150 * time.Millisecond)
	sustainedBurstMetadata := ActivityScheduleBurstMetadata{
		FirstEventID:    2,
		LastEventID:     2 + int64(sustainedBurstCount) - 1,
		EventCount:      sustainedBurstCount,
		WindowStart:     time.Unix(0, testTimestamp).UTC(),
		WindowEnd:       time.Unix(0, testTimestamp+int64(sustainedBurstCount-1)*sustainedBurstStepNanos).UTC(),
		WindowInSeconds: activityBurstWindowInSeconds,
		Threshold:       activityBurstCountThreshold,
	}
	sustainedBurstMetadataInBytes, err := json.Marshal(sustainedBurstMetadata)
	require.NoError(t, err)

	group1LastTimestamp := testTimestamp + int64(activityBurstCountThreshold-1)*testStepNanos
	sharedBoundaryGroup2Start := group1LastTimestamp + windowNanos
	sharedBoundaryMetadata := ActivityScheduleBurstMetadata{
		FirstEventID:    2,
		LastEventID:     2 * int64(activityBurstCountThreshold),
		EventCount:      2*activityBurstCountThreshold - 1,
		WindowStart:     time.Unix(0, testTimestamp).UTC(),
		WindowEnd:       time.Unix(0, sharedBoundaryGroup2Start).UTC(),
		WindowInSeconds: activityBurstWindowInSeconds,
		Threshold:       activityBurstCountThreshold,
	}
	sharedBoundaryMetadataInBytes, err := json.Marshal(sharedBoundaryMetadata)
	require.NoError(t, err)

	cronCaNMetadata := ContinueAsNewInCronWorkflowMetadata{
		StartedEventID:        1,
		CronSchedule:          testCronSchedule,
		ContinuedAsNewEventID: 2,
	}
	cronCaNMetadataInBytes, err := json.Marshal(cronCaNMetadata)
	require.NoError(t, err)

	cronCaNAfterBurstMetadata := ContinueAsNewInCronWorkflowMetadata{
		StartedEventID:        1,
		CronSchedule:          testCronSchedule,
		ContinuedAsNewEventID: 90,
	}
	cronCaNAfterBurstMetadataInBytes, err := json.Marshal(cronCaNAfterBurstMetadata)
	require.NoError(t, err)

	testCases := []struct {
		name           string
		testData       *types.GetWorkflowExecutionHistoryResponse
		expectedResult []invariant.InvariantCheckResult
	}{
		{
			name: "activity schedule burst",
			testData: wfHistory(
				startedEvent(1, ""),
				activityScheduleBurst(activityBurstCountThreshold, 2, testTimestamp, testStepNanos),
			),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityScheduleBurst.String(),
					Reason:        ActivityScheduleBurstDetected.String(),
					Metadata:      burstMetadataInBytes,
				},
			},
		},
		{
			name: "activity schedule burst one below threshold",
			testData: wfHistory(
				startedEvent(1, ""),
				activityScheduleBurst(activityBurstCountThreshold-1, 2, testTimestamp, testStepNanos),
			),
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "activity schedule burst spanning exactly the window",
			testData: wfHistory(
				startedEvent(1, ""),
				activityScheduleBurst(activityBurstCountThreshold-1, 2, testTimestamp, 0),
				activityScheduleBurst(1, 2+int64(activityBurstCountThreshold)-1, testTimestamp+windowNanos, 0),
			),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityScheduleBurst.String(),
					Reason:        ActivityScheduleBurstDetected.String(),
					Metadata:      burstAtWindowEdgeMetadataInBytes,
				},
			},
		},
		{
			name: "each independent burst cluster is reported as its own issue",
			testData: wfHistory(
				startedEvent(1, ""),
				activityScheduleBurst(activityBurstCountThreshold, 2, testTimestamp, testStepNanos),
				activityScheduleBurst(activityBurstCountThreshold+5, 100, testTimestamp+int64(time.Hour), testStepNanos),
			),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityScheduleBurst.String(),
					Reason:        ActivityScheduleBurstDetected.String(),
					Metadata:      burstMetadataInBytes,
				},
				{
					IssueID:       1,
					InvariantType: ActivityScheduleBurst.String(),
					Reason:        ActivityScheduleBurstDetected.String(),
					Metadata:      secondBurstMetadataInBytes,
				},
			},
		},
		{
			name: "a sustained burst longer than the window is reported as a single issue",
			testData: wfHistory(
				startedEvent(1, ""),
				activityScheduleBurst(sustainedBurstCount, 2, testTimestamp, sustainedBurstStepNanos),
			),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityScheduleBurst.String(),
					Reason:        ActivityScheduleBurstDetected.String(),
					Metadata:      sustainedBurstMetadataInBytes,
				},
			},
		},
		{
			name: "two windows sharing a boundary event merge instead of double-counting it",
			testData: wfHistory(
				startedEvent(1, ""),
				activityScheduleBurst(activityBurstCountThreshold, 2, testTimestamp, testStepNanos),
				activityScheduleBurst(activityBurstCountThreshold-1, 2+int64(activityBurstCountThreshold), sharedBoundaryGroup2Start, 0),
			),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ActivityScheduleBurst.String(),
					Reason:        ActivityScheduleBurstDetected.String(),
					Metadata:      sharedBoundaryMetadataInBytes,
				},
			},
		},
		{
			name: "scheduled events without timestamps are excluded",
			testData: wfHistory(
				startedEvent(1, ""),
				untimestampedActivityScheduledEvents(activityBurstCountThreshold+10, 2),
			),
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "cron workflow with cron-initiated continue-as-new",
			testData: wfHistory(
				startedEvent(1, testCronSchedule),
				continuedAsNewEvent(2, types.ContinueAsNewInitiatorCronSchedule),
			),
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "cron workflow with retry-initiated continue-as-new",
			testData: wfHistory(
				startedEvent(1, testCronSchedule),
				continuedAsNewEvent(2, types.ContinueAsNewInitiatorRetryPolicy),
			),
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "non-cron workflow with decider-initiated continue-as-new",
			testData: wfHistory(
				startedEvent(1, ""),
				continuedAsNewEvent(2, types.ContinueAsNewInitiatorDecider),
			),
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "cron workflow with decider-initiated continue-as-new",
			testData: wfHistory(
				startedEvent(1, testCronSchedule),
				continuedAsNewEvent(2, types.ContinueAsNewInitiatorDecider),
			),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ContinueAsNewInCronWorkflow.String(),
					Reason:        ContinueAsNewInitiatedByDeciderInCronWorkflow.String(),
					Metadata:      cronCaNMetadataInBytes,
				},
			},
		},
		{
			name: "both issues fire with continue-as-new reported first",
			testData: wfHistory(
				startedEvent(1, testCronSchedule),
				activityScheduleBurst(activityBurstCountThreshold, 2, testTimestamp, testStepNanos),
				continuedAsNewEvent(90, types.ContinueAsNewInitiatorDecider),
			),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       0,
					InvariantType: ContinueAsNewInCronWorkflow.String(),
					Reason:        ContinueAsNewInitiatedByDeciderInCronWorkflow.String(),
					Metadata:      cronCaNAfterBurstMetadataInBytes,
				},
				{
					IssueID:       1,
					InvariantType: ActivityScheduleBurst.String(),
					Reason:        ActivityScheduleBurstDetected.String(),
					Metadata:      burstMetadataInBytes,
				},
			},
		},
		{
			name: "well-behaved workflow",
			testData: wfHistory(
				startedEvent(1, ""),
				activityScheduleBurst(5, 2, testTimestamp, int64(5*time.Minute)),
			),
			expectedResult: []invariant.InvariantCheckResult{},
		},
		{
			name: "history without started event",
			testData: wfHistory(
				activityScheduleBurst(3, 1, testTimestamp, testStepNanos),
			),
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
			require.Equal(t, tc.expectedResult, result)
		})
	}
}

func Test__RootCause(t *testing.T) {
	inv := NewInvariant()
	result, err := inv.RootCause(context.Background(), invariant.InvariantRootCauseInput{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result)
}

func wfHistory(eventGroups ...[]*types.HistoryEvent) *types.GetWorkflowExecutionHistoryResponse {
	events := make([]*types.HistoryEvent, 0)
	for _, group := range eventGroups {
		events = append(events, group...)
	}
	return &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{Events: events},
	}
}

func startedEvent(id int64, cronSchedule string) []*types.HistoryEvent {
	return []*types.HistoryEvent{
		{
			ID: id,
			WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
				CronSchedule: cronSchedule,
			},
		},
	}
}

func continuedAsNewEvent(id int64, initiator types.ContinueAsNewInitiator) []*types.HistoryEvent {
	return []*types.HistoryEvent{
		{
			ID: id,
			WorkflowExecutionContinuedAsNewEventAttributes: &types.WorkflowExecutionContinuedAsNewEventAttributes{
				Initiator: initiator.Ptr(),
			},
		},
	}
}

func activityScheduledEvent(id int64, timestamp *int64) *types.HistoryEvent {
	return &types.HistoryEvent{
		ID:        id,
		Timestamp: timestamp,
		ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
			ActivityID:   "101",
			ActivityType: &types.ActivityType{Name: "test-activity"},
		},
	}
}

func activityScheduleBurst(count int, startID, startTimestamp, stepNanos int64) []*types.HistoryEvent {
	events := make([]*types.HistoryEvent, 0, count)
	for i := 0; i < count; i++ {
		events = append(events, activityScheduledEvent(startID+int64(i), common.Int64Ptr(startTimestamp+int64(i)*stepNanos)))
	}
	return events
}

func untimestampedActivityScheduledEvents(count int, startID int64) []*types.HistoryEvent {
	events := make([]*types.HistoryEvent, 0, count)
	for i := 0; i < count; i++ {
		events = append(events, activityScheduledEvent(startID+int64(i), nil))
	}
	return events
}
