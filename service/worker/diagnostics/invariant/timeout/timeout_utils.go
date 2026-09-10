package timeout

import (
	"fmt"
	"sort"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func reasonForDecisionTaskTimeouts(event *types.HistoryEvent, allEvents []*types.HistoryEvent) (string, TimeoutIssuesMetadata) {
	eventScheduledID := event.GetDecisionTaskTimedOutEventAttributes().GetScheduledEventID()
	attr := event.GetDecisionTaskTimedOutEventAttributes()
	cause := attr.GetCause()
	var reason string
	switch cause {
	case types.DecisionTaskTimedOutCauseTimeout:
		reason = attr.TimeoutType.String()
	case types.DecisionTaskTimedOutCauseReset:
		newRunID := attr.GetNewRunID()
		reason = fmt.Sprintf("%s - New run ID: %s", attr.Reason, newRunID)
	}
	return reason, TimeoutIssuesMetadata{
		EventID:           event.ID,
		ConfiguredTimeout: time.Duration(getDecisionTaskConfiguredTimeout(eventScheduledID, allEvents)) * time.Second,
	}
}

func getWorkflowExecutionConfiguredTimeout(events []*types.HistoryEvent) int32 {
	for _, event := range events {
		if event.ID == 1 { // event 1 is workflow execution started event
			return event.GetWorkflowExecutionStartedEventAttributes().GetExecutionStartToCloseTimeoutSeconds()
		}
	}
	return 0
}

func getWorkflowExecutionTasklist(events []*types.HistoryEvent) *types.TaskList {
	for _, event := range events {
		if event.ID == 1 { // event 1 is workflow execution started event
			return event.GetWorkflowExecutionStartedEventAttributes().TaskList
		}
	}
	return nil
}

func getActivityTaskMetadata(e *types.HistoryEvent, events []*types.HistoryEvent) (TimeoutIssuesMetadata, error) {
	eventScheduledID := e.GetActivityTaskTimedOutEventAttributes().GetScheduledEventID()
	eventStartedID := e.GetActivityTaskTimedOutEventAttributes().StartedEventID
	timeoutType := e.GetActivityTaskTimedOutEventAttributes().GetTimeoutType()
	var configuredTimeout int32
	var timeElapsed time.Duration
	for _, event := range events {
		if event.ID == eventScheduledID {
			attr := event.GetActivityTaskScheduledEventAttributes()
			switch timeoutType {
			case types.TimeoutTypeHeartbeat:
				configuredTimeout = attr.GetHeartbeatTimeoutSeconds()
				timeElapsed = getExecutionTime(eventStartedID, e.ID, events)
			case types.TimeoutTypeScheduleToClose:
				configuredTimeout = attr.GetScheduleToCloseTimeoutSeconds()
				timeElapsed = getExecutionTime(eventScheduledID, e.ID, events)
			case types.TimeoutTypeScheduleToStart:
				configuredTimeout = attr.GetScheduleToStartTimeoutSeconds()
				timeElapsed = getExecutionTime(eventScheduledID, e.ID, events)
			case types.TimeoutTypeStartToClose:
				configuredTimeout = attr.GetStartToCloseTimeoutSeconds()
				timeElapsed = getExecutionTime(eventStartedID, e.ID, events)
			default:
				return TimeoutIssuesMetadata{}, fmt.Errorf("unknown timeout type")
			}
			return TimeoutIssuesMetadata{
				EventID:           e.ID,
				ConfiguredTimeout: time.Duration(configuredTimeout) * time.Second,
				ActivityTimeout: &ActivityTimeoutMetadata{
					TimeoutType:      timeoutType.Ptr(),
					TimeElapsed:      timeElapsed,
					RetryPolicy:      attr.RetryPolicy,
					HeartBeatTimeout: time.Duration(attr.GetHeartbeatTimeoutSeconds()) * time.Second,
					Tasklist:         attr.TaskList,
				},
			}, nil
		}

	}
	return TimeoutIssuesMetadata{}, fmt.Errorf("activity scheduled event not found")
}

func getDecisionTaskConfiguredTimeout(eventScheduledID int64, events []*types.HistoryEvent) int32 {
	for _, event := range events {
		if event.ID == eventScheduledID {
			return event.GetDecisionTaskScheduledEventAttributes().GetStartToCloseTimeoutSeconds()
		}
	}
	return 0
}

func getChildWorkflowExecutionConfiguredTimeout(e *types.HistoryEvent, events []*types.HistoryEvent) int32 {
	wfInitiatedID := e.GetChildWorkflowExecutionTimedOutEventAttributes().GetInitiatedEventID()
	for _, event := range events {
		if event.ID == wfInitiatedID {
			return event.GetStartChildWorkflowExecutionInitiatedEventAttributes().GetExecutionStartToCloseTimeoutSeconds()
		}
	}
	return 0
}

func getExecutionTime(startID, timeoutID int64, events []*types.HistoryEvent) time.Duration {
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].ID < events[j].ID
	})

	firstEvent := events[startID-1]
	lastEvent := events[timeoutID-1]
	return time.Unix(0, common.Int64Default(lastEvent.Timestamp)).Sub(time.Unix(0, common.Int64Default(firstEvent.Timestamp)))
}
