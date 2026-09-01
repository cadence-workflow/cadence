package antipatterns

import (
	"context"
	"sort"
	"time"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

// Antipatterns is an invariant that identifies workflow implementation patterns that are known to
// cause operational problems even though they are not failures on their own.
type Antipatterns invariant.Invariant

type antipatterns struct {
}

func NewInvariant() Antipatterns {
	return &antipatterns{}
}

func (a *antipatterns) Check(ctx context.Context, params invariant.InvariantCheckInput) ([]invariant.InvariantCheckResult, error) {
	result := make([]invariant.InvariantCheckResult, 0)
	events := params.WorkflowExecutionHistory.GetHistory().GetEvents()

	for _, burst := range detectActivityScheduleBursts(events) {
		result = append(result, invariant.InvariantCheckResult{
			IssueID:       len(result),
			InvariantType: ActivityScheduleBurst.String(),
			Reason:        ActivityScheduleBurstDetected.String(),
			Metadata:      invariant.MarshalData(burst),
		})
	}

	if issue := detectContinueAsNewInCronWorkflow(events); issue != nil {
		result = append(result, invariant.InvariantCheckResult{
			IssueID:       len(result),
			InvariantType: ContinueAsNewInCronWorkflow.String(),
			Reason:        ContinueAsNewInitiatedByDeciderInCronWorkflow.String(),
			Metadata:      invariant.MarshalData(issue),
		})
	}

	return result, nil
}

// detectActivityScheduleBursts finds every maximal cluster of timestamped ActivityTaskScheduled
// events in which a sliding window of activityBurstWindowInSeconds always contains at least
// activityBurstCountThreshold events, and reports each cluster once as a single span covering all
// of its events. Clusters are only split where the scheduling density genuinely drops below the
// threshold between them; a single sustained burst that runs longer than the window itself is
// still reported as one span rather than being cut at an arbitrary window boundary. Events without
// a timestamp are excluded: a nil timestamp is unknown, not time zero, and treating it as zero
// would collapse unrelated events into a phantom burst.
func detectActivityScheduleBursts(events []*types.HistoryEvent) []*ActivityScheduleBurstMetadata {
	type scheduledEvent struct {
		eventID   int64
		timestamp int64
	}

	scheduled := make([]scheduledEvent, 0, len(events))
	for _, event := range events {
		if event.GetActivityTaskScheduledEventAttributes() == nil || event.Timestamp == nil {
			continue
		}
		scheduled = append(scheduled, scheduledEvent{eventID: event.ID, timestamp: *event.Timestamp})
	}
	if len(scheduled) < activityBurstCountThreshold {
		return nil
	}

	sort.SliceStable(scheduled, func(i, j int) bool {
		return scheduled[i].timestamp < scheduled[j].timestamp
	})

	windowNanos := (time.Duration(activityBurstWindowInSeconds) * time.Second).Nanoseconds()

	newBurst := func(clusterStart, clusterEnd int) *ActivityScheduleBurstMetadata {
		return &ActivityScheduleBurstMetadata{
			FirstEventID:    scheduled[clusterStart].eventID,
			LastEventID:     scheduled[clusterEnd].eventID,
			EventCount:      clusterEnd - clusterStart + 1,
			WindowStart:     time.Unix(0, scheduled[clusterStart].timestamp).UTC(),
			WindowEnd:       time.Unix(0, scheduled[clusterEnd].timestamp).UTC(),
			WindowInSeconds: activityBurstWindowInSeconds,
			Threshold:       activityBurstCountThreshold,
		}
	}

	var bursts []*ActivityScheduleBurstMetadata
	clusterStart := -1
	clusterEnd := -1
	left := 0
	for right := 0; right < len(scheduled); right++ {
		for scheduled[right].timestamp-scheduled[left].timestamp > windowNanos {
			left++
		}
		if right-left+1 >= activityBurstCountThreshold {
			if clusterStart == -1 {
				clusterStart = left
			}
			clusterEnd = right
			continue
		}
		if clusterStart != -1 {
			bursts = append(bursts, newBurst(clusterStart, clusterEnd))
			clusterStart, clusterEnd = -1, -1
		}
	}
	if clusterStart != -1 {
		bursts = append(bursts, newBurst(clusterStart, clusterEnd))
	}
	return bursts
}

// detectContinueAsNewInCronWorkflow flags a workflow that was started with a cron schedule and
// continued-as-new from workflow code. A nil Initiator reads as ContinueAsNewInitiatorDecider,
// which is correct: the server only sets the initiator explicitly for its own cron- and
// retry-driven continuations, and passes decisions from workflow code through unmodified.
func detectContinueAsNewInCronWorkflow(events []*types.HistoryEvent) *ContinueAsNewInCronWorkflowMetadata {
	startedEvent := fetchWfStartedEvent(events)
	if startedEvent == nil {
		return nil
	}
	cronSchedule := startedEvent.GetWorkflowExecutionStartedEventAttributes().GetCronSchedule()
	if cronSchedule == "" {
		return nil
	}

	for _, event := range events {
		attr := event.GetWorkflowExecutionContinuedAsNewEventAttributes()
		if attr == nil {
			continue
		}
		if attr.GetInitiator() == types.ContinueAsNewInitiatorDecider {
			return &ContinueAsNewInCronWorkflowMetadata{
				StartedEventID:        startedEvent.ID,
				CronSchedule:          cronSchedule,
				ContinuedAsNewEventID: event.ID,
			}
		}
	}
	return nil
}

func fetchWfStartedEvent(events []*types.HistoryEvent) *types.HistoryEvent {
	for _, event := range events {
		if event.GetWorkflowExecutionStartedEventAttributes() != nil {
			return event
		}
	}
	return nil
}

func (a *antipatterns) RootCause(ctx context.Context, params invariant.InvariantRootCauseInput) ([]invariant.InvariantRootCauseResult, error) {
	// Not implemented since this invariant does not have any root cause.
	// Issues identified in Check() are self-explanatory.
	result := make([]invariant.InvariantRootCauseResult, 0)
	return result, nil
}
