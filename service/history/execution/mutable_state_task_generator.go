// Copyright (c) 2020 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination mutable_state_task_generator_mock.go -self_package github.com/uber/cadence/service/history/execution

package execution

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// MutableStateTaskGenerator generates workflow transfer and timer tasks
	MutableStateTaskGenerator interface {
		// for workflow reset startTime should be the reset time instead of
		// the startEvent time, so that workflow timeout timestamp can be
		// re-calculated.
		GenerateWorkflowStartTasks(
			startTime time.Time,
			startEvent *types.HistoryEvent,
		) error
		GenerateWorkflowCloseTasks(
			closeEvent *types.HistoryEvent,
			workflowDeletionTaskJitterRange int,
		) error
		GenerateRecordWorkflowStartedTasks(
			startEvent *types.HistoryEvent,
		) error
		GenerateDelayedDecisionTasks(
			startEvent *types.HistoryEvent,
		) error
		GenerateDecisionScheduleTasks(
			decisionScheduleID int64,
		) error
		GenerateDecisionStartTasks(
			decisionScheduleID int64,
		) error
		GenerateActivityTransferTasks(
			event *types.HistoryEvent,
		) error
		GenerateActivityRetryTasks(
			activityScheduleID int64,
		) error
		GenerateChildWorkflowTasks(
			event *types.HistoryEvent,
		) error
		GenerateRequestCancelExternalTasks(
			event *types.HistoryEvent,
		) error
		GenerateSignalExternalTasks(
			event *types.HistoryEvent,
		) error
		GenerateWorkflowSearchAttrTasks() error
		GenerateWorkflowResetTasks() error
		// these 2 APIs should only be called when mutable state transaction is being closed
		GenerateActivityTimerTasks() error
		GenerateUserTimerTasks() error
	}

	mutableStateTaskGeneratorImpl struct {
		logger          log.Logger
		clusterMetadata cluster.Metadata
		domainCache     cache.DomainCache

		mutableState MutableState
	}
)

const (
	defaultWorkflowRetentionInDays      int32 = 1
	defaultInitIntervalForDecisionRetry       = 1 * time.Minute
	defaultMaxIntervalForDecisionRetry        = 5 * time.Minute
	defaultJitterCoefficient                  = 0.2
)

var _ MutableStateTaskGenerator = (*mutableStateTaskGeneratorImpl)(nil)

// NewMutableStateTaskGenerator creates a new task generator for mutable state
func NewMutableStateTaskGenerator(
	logger log.Logger,
	clusterMetadata cluster.Metadata,
	domainCache cache.DomainCache,
	mutableState MutableState,
) MutableStateTaskGenerator {

	return &mutableStateTaskGeneratorImpl{
		logger:          logger,
		clusterMetadata: clusterMetadata,
		domainCache:     domainCache,
		mutableState:    mutableState,
	}
}

func (r *mutableStateTaskGeneratorImpl) GenerateWorkflowStartTasks(
	startTime time.Time,
	startEvent *types.HistoryEvent,
) error {
	attr := startEvent.WorkflowExecutionStartedEventAttributes
	firstDecisionDelayDuration := time.Duration(attr.GetFirstDecisionTaskBackoffSeconds()) * time.Second

	executionInfo := r.mutableState.GetExecutionInfo()
	startVersion := startEvent.Version

	workflowTimeoutDuration := time.Duration(executionInfo.WorkflowTimeout) * time.Second
	workflowTimeoutTimestamp := startTime.Add(workflowTimeoutDuration + firstDecisionDelayDuration)
	// ensure that the first attempt does not time out early based on retry policy timeout
	if attr.Attempt > 0 && !executionInfo.ExpirationTime.IsZero() && workflowTimeoutTimestamp.After(executionInfo.ExpirationTime) {
		workflowTimeoutTimestamp = executionInfo.ExpirationTime
	}
	r.mutableState.AddTimerTasks(&persistence.WorkflowTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID is set by shard
			VisibilityTimestamp: workflowTimeoutTimestamp,
			Version:             startVersion,
		},
		TaskList: executionInfo.TaskList,
	})

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateWorkflowCloseTasks(
	closeEvent *types.HistoryEvent,
	workflowDeletionTaskJitterRange int,
) error {

	executionInfo := r.mutableState.GetExecutionInfo()
	taskList := executionInfo.TaskList
	r.mutableState.AddTransferTasks(&persistence.CloseExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID and VisibilityTimestamp are set by shard context
			Version: closeEvent.Version,
		},
		TaskList: taskList,
	})

	retentionInDays := defaultWorkflowRetentionInDays
	domainEntry, err := r.domainCache.GetDomainByID(executionInfo.DomainID)
	switch err.(type) {
	case nil:
		retentionInDays = domainEntry.GetRetentionDays(executionInfo.WorkflowID)
	case *types.EntityNotExistsError:
		// domain is not accessible, use default value above
	default:
		return err
	}

	closeTimestamp := time.Unix(0, closeEvent.GetTimestamp())
	retentionDuration := (time.Duration(retentionInDays) * time.Hour * 24)
	if workflowDeletionTaskJitterRange > 1 {
		retentionDuration += time.Duration(rand.Intn(workflowDeletionTaskJitterRange*60)) * time.Second
	}

	r.logger.Debug("GenerateWorkflowCloseTasks",
		tag.WorkflowID(executionInfo.WorkflowID),
		tag.WorkflowRunID(executionInfo.RunID),
		tag.WorkflowDomainID(executionInfo.DomainID),
		tag.Timestamp(closeTimestamp),
	)
	r.mutableState.AddTimerTasks(&persistence.DeleteHistoryEventTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID is set by shard
			VisibilityTimestamp: closeTimestamp.Add(retentionDuration),
			Version:             closeEvent.Version,
		},
		TaskList: taskList,
	})

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateDelayedDecisionTasks(
	startEvent *types.HistoryEvent,
) error {

	startVersion := startEvent.Version
	startTimestamp := time.Unix(0, startEvent.GetTimestamp())
	startAttr := startEvent.WorkflowExecutionStartedEventAttributes
	decisionBackoffDuration := time.Duration(startAttr.GetFirstDecisionTaskBackoffSeconds()) * time.Second
	executionTimestamp := startTimestamp.Add(decisionBackoffDuration)

	// noParentWorkflow case
	firstDecisionDelayType := persistence.WorkflowBackoffTimeoutTypeCron
	// continue as new case
	if startAttr.Initiator != nil {
		switch startAttr.GetInitiator() {
		case types.ContinueAsNewInitiatorRetryPolicy:
			firstDecisionDelayType = persistence.WorkflowBackoffTimeoutTypeRetry
		case types.ContinueAsNewInitiatorCronSchedule:
			firstDecisionDelayType = persistence.WorkflowBackoffTimeoutTypeCron
		case types.ContinueAsNewInitiatorDecider:
			return &types.InternalServiceError{
				Message: "encounter continue as new iterator & first decision delay not 0",
			}
		default:
			return &types.InternalServiceError{
				Message: fmt.Sprintf("unknown iterator retry policy: %v", startAttr.GetInitiator()),
			}
		}
	}

	executionInfo := r.mutableState.GetExecutionInfo()
	r.mutableState.AddTimerTasks(&persistence.WorkflowBackoffTimerTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID is set by shard
			VisibilityTimestamp: executionTimestamp,
			Version:             startVersion,
		},
		TimeoutType: firstDecisionDelayType,
		TaskList:    executionInfo.TaskList,
	})

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateRecordWorkflowStartedTasks(
	startEvent *types.HistoryEvent,
) error {

	startVersion := startEvent.Version

	executionInfo := r.mutableState.GetExecutionInfo()
	r.mutableState.AddTransferTasks(&persistence.RecordWorkflowStartedTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID and VisibilityTimestamp are set by shard context
			Version: startVersion,
		},
		TaskList: executionInfo.TaskList,
	})

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateDecisionScheduleTasks(
	decisionScheduleID int64,
) error {

	executionInfo := r.mutableState.GetExecutionInfo()
	decision, ok := r.mutableState.GetDecisionInfo(
		decisionScheduleID,
	)
	if !ok {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("it could be a bug, cannot get pending decision: %v", decisionScheduleID),
		}
	}

	originalTaskList := executionInfo.TaskList
	r.mutableState.AddTransferTasks(&persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID and VisibilityTimestamp are set by shard context
			Version: decision.Version,
		},
		TargetDomainID:   executionInfo.DomainID,
		TaskList:         decision.TaskList,
		ScheduleID:       decision.ScheduleID,
		OriginalTaskList: originalTaskList,
	})

	if scheduleToStartTimeout := r.mutableState.GetDecisionScheduleToStartTimeout(); scheduleToStartTimeout != 0 {
		scheduledTime := time.Unix(0, decision.ScheduledTimestamp)
		r.mutableState.AddTimerTasks(&persistence.DecisionTimeoutTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID:   executionInfo.DomainID,
				WorkflowID: executionInfo.WorkflowID,
				RunID:      executionInfo.RunID,
			},
			TaskData: persistence.TaskData{
				// TaskID is set by shard
				VisibilityTimestamp: scheduledTime.Add(scheduleToStartTimeout),
				Version:             decision.Version,
			},
			TimeoutType:     int(TimerTypeScheduleToStart),
			EventID:         decision.ScheduleID,
			ScheduleAttempt: decision.Attempt,
			TaskList:        originalTaskList,
		})
	}

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateDecisionStartTasks(
	decisionScheduleID int64,
) error {

	decision, ok := r.mutableState.GetDecisionInfo(
		decisionScheduleID,
	)
	if !ok {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("it could be a bug, cannot get pending decision: %v", decisionScheduleID),
		}
	}

	startedTime := time.Unix(0, decision.StartedTimestamp)
	startToCloseTimeout := time.Duration(
		decision.DecisionTimeout,
	) * time.Second

	executionInfo := r.mutableState.GetExecutionInfo()
	// schedule timer exponentially if decision keeps failing
	if decision.Attempt > 1 {
		defaultStartToCloseTimeout := executionInfo.DecisionStartToCloseTimeout
		startToCloseTimeout = getNextDecisionTimeout(decision.Attempt, time.Duration(defaultStartToCloseTimeout)*time.Second)
		decision.DecisionTimeout = int32(startToCloseTimeout.Seconds()) // override decision timeout
		r.mutableState.UpdateDecision(decision)
	}

	r.mutableState.AddTimerTasks(&persistence.DecisionTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID is set by shard
			VisibilityTimestamp: startedTime.Add(startToCloseTimeout),
			Version:             decision.Version,
		},
		TimeoutType:     int(TimerTypeStartToClose),
		EventID:         decision.ScheduleID,
		ScheduleAttempt: decision.Attempt,
		TaskList:        executionInfo.TaskList,
	})

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateActivityTransferTasks(
	event *types.HistoryEvent,
) error {

	attr := event.ActivityTaskScheduledEventAttributes
	activityScheduleID := event.ID

	activityInfo, ok := r.mutableState.GetActivityInfo(activityScheduleID)
	if !ok {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("it could be a bug, cannot get pending activity: %v", activityScheduleID),
		}
	}

	var targetDomainID string
	var err error
	if activityInfo.DomainID != "" {
		targetDomainID = activityInfo.DomainID
	} else {
		// TODO remove this block after Mar, 1th, 2020
		//  previously, DomainID in activity info is not used, so need to get
		//  schedule event from DB checking whether activity to be scheduled
		//  belongs to this domain
		targetDomainID, err = r.getTargetDomainID(attr.GetDomain())
		if err != nil {
			return err
		}
	}

	executionInfo := r.mutableState.GetExecutionInfo()
	r.mutableState.AddTransferTasks(&persistence.ActivityTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID and VisibilityTimestamp are set by shard context
			Version: activityInfo.Version,
		},
		TargetDomainID: targetDomainID,
		TaskList:       activityInfo.TaskList,
		ScheduleID:     activityInfo.ScheduleID,
	})

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateActivityRetryTasks(
	activityScheduleID int64,
) error {

	ai, ok := r.mutableState.GetActivityInfo(activityScheduleID)
	if !ok {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("it could be a bug, cannot get pending activity: %v", activityScheduleID),
		}
	}

	executionInfo := r.mutableState.GetExecutionInfo()
	r.mutableState.AddTimerTasks(&persistence.ActivityRetryTimerTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID is set by shard
			Version:             ai.Version,
			VisibilityTimestamp: ai.ScheduledTime,
		},
		EventID:  ai.ScheduleID,
		Attempt:  int64(ai.Attempt),
		TaskList: ai.TaskList,
	})
	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateChildWorkflowTasks(
	event *types.HistoryEvent,
) error {

	childWorkflowScheduleID := event.ID

	childWorkflowInfo, ok := r.mutableState.GetChildExecutionInfo(childWorkflowScheduleID)
	if !ok {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("it could be a bug, cannot get pending child workflow: %v", childWorkflowScheduleID),
		}
	}

	msbDomainID := r.mutableState.GetDomainEntry().GetInfo().ID

	targetDomainID := childWorkflowInfo.DomainID
	if childWorkflowInfo.DomainID == "" {
		targetDomainID = msbDomainID
	}

	err := r.validateChildWorkflowParameters(msbDomainID, targetDomainID)
	if err != nil {
		return err
	}

	executionInfo := r.mutableState.GetExecutionInfo()
	startChildExecutionTask := &persistence.StartChildExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID and VisibilityTimestamp are set by shard context
			Version: childWorkflowInfo.Version,
		},
		TargetDomainID:   targetDomainID,
		TargetWorkflowID: childWorkflowInfo.StartedWorkflowID,
		InitiatedID:      childWorkflowInfo.InitiatedID,
		TaskList:         executionInfo.TaskList,
	}

	r.mutableState.AddTransferTasks(startChildExecutionTask)
	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateRequestCancelExternalTasks(
	event *types.HistoryEvent,
) error {

	attr := event.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes
	scheduleID := event.ID
	version := event.Version
	targetDomainName := attr.GetDomain()
	targetWorkflowID := attr.GetWorkflowExecution().GetWorkflowID()
	targetRunID := attr.GetWorkflowExecution().GetRunID()
	targetChildOnly := attr.GetChildWorkflowOnly()

	_, ok := r.mutableState.GetRequestCancelInfo(scheduleID)
	if !ok {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("it could be a bug, cannot get pending request cancel external workflow: %v", scheduleID),
		}
	}

	targetDomainID, err := r.getTargetDomainID(targetDomainName)
	if err != nil {
		return err
	}

	executionInfo := r.mutableState.GetExecutionInfo()
	cancelExecutionTask := &persistence.CancelExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID and VisibilityTimestamp are set by shard context
			Version: version,
		},
		TargetDomainID:          targetDomainID,
		TargetWorkflowID:        targetWorkflowID,
		TargetRunID:             targetRunID,
		TargetChildWorkflowOnly: targetChildOnly,
		InitiatedID:             scheduleID,
		TaskList:                executionInfo.TaskList,
	}

	r.mutableState.AddTransferTasks(cancelExecutionTask)

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateSignalExternalTasks(
	event *types.HistoryEvent,
) error {

	attr := event.SignalExternalWorkflowExecutionInitiatedEventAttributes
	scheduleID := event.ID
	version := event.Version
	targetDomainName := attr.GetDomain()
	targetWorkflowID := attr.GetWorkflowExecution().GetWorkflowID()
	targetRunID := attr.GetWorkflowExecution().GetRunID()
	targetChildOnly := attr.GetChildWorkflowOnly()

	_, ok := r.mutableState.GetSignalInfo(scheduleID)
	if !ok {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("it could be a bug, cannot get pending signal external workflow: %v", scheduleID),
		}
	}

	targetDomainID, err := r.getTargetDomainID(targetDomainName)
	if err != nil {
		return err
	}

	executionInfo := r.mutableState.GetExecutionInfo()
	signalExecutionTask := &persistence.SignalExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID and VisibilityTimestamp are set by shard context
			Version: version,
		},
		TargetDomainID:          targetDomainID,
		TargetWorkflowID:        targetWorkflowID,
		TargetRunID:             targetRunID,
		TargetChildWorkflowOnly: targetChildOnly,
		InitiatedID:             scheduleID,
		TaskList:                executionInfo.TaskList,
	}

	r.mutableState.AddTransferTasks(signalExecutionTask)

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateWorkflowSearchAttrTasks() error {

	currentVersion := r.mutableState.GetCurrentVersion()

	executionInfo := r.mutableState.GetExecutionInfo()
	r.mutableState.AddTransferTasks(&persistence.UpsertWorkflowSearchAttributesTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID and VisibilityTimestamp are set by shard context
			Version: currentVersion, // task processing does not check this version
		},
		TaskList: executionInfo.TaskList,
	})

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateWorkflowResetTasks() error {

	currentVersion := r.mutableState.GetCurrentVersion()

	executionInfo := r.mutableState.GetExecutionInfo()
	r.mutableState.AddTransferTasks(&persistence.ResetWorkflowTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID and VisibilityTimestamp are set by shard context
			Version: currentVersion,
		},
		TaskList: executionInfo.TaskList,
	})

	return nil
}

func (r *mutableStateTaskGeneratorImpl) GenerateActivityTimerTasks() error {

	_, err := NewTimerSequence(r.mutableState).CreateNextActivityTimer()
	return err
}

func (r *mutableStateTaskGeneratorImpl) GenerateUserTimerTasks() error {

	_, err := NewTimerSequence(r.mutableState).CreateNextUserTimer()
	return err
}

func (r *mutableStateTaskGeneratorImpl) getTargetDomainID(
	targetDomainName string,
) (string, error) {
	if targetDomainName != "" {
		return r.domainCache.GetDomainID(targetDomainName)
	}

	return r.mutableState.GetExecutionInfo().DomainID, nil
}

func (r *mutableStateTaskGeneratorImpl) validateChildWorkflowParameters(msbDomainID string, targetDomainID string) error {
	// standard case
	if msbDomainID == targetDomainID {
		return nil
	}

	thisDomain := r.mutableState.GetDomainEntry()
	targetDomain, err := r.domainCache.GetDomainByID(targetDomainID)
	if err != nil {
		return fmt.Errorf("cannot get target domain for child workflow: %w", err)
	}

	// Generally, cross-domain calls are not allowed for launching child workflows in global domains due to
	// the fact that we have removed cross-cluster calls (due to their overhead and limited use).
	//
	// There is a limited exception for local domains, which do not suffer from this problem can be
	// handled as an exception where the transfer task may be picked up by another domain in-cluster
	// without risk of the child workflow may end up in a different cluster.
	if thisDomain.IsGlobalDomain() || targetDomain.IsGlobalDomain() {
		return &types.BadRequestError{
			Message: fmt.Sprintf("The child workflow is "+
				"trying to use domain %s but it's running in domain %s. "+
				"Cross-cluster and cross domain child workflows are not supported for global domains",
				targetDomainID, msbDomainID),
		}
	}
	return nil
}

func getNextDecisionTimeout(attempt int64, defaultStartToCloseTimeout time.Duration) time.Duration {
	if attempt <= 1 {
		return defaultStartToCloseTimeout
	}

	nextInterval := float64(defaultInitIntervalForDecisionRetry) * math.Pow(2, float64(attempt-2))
	nextInterval = math.Min(nextInterval, float64(defaultMaxIntervalForDecisionRetry))
	jitterPortion := int(defaultJitterCoefficient * nextInterval)
	if jitterPortion < 1 {
		jitterPortion = 1
	}
	nextInterval = nextInterval*(1-defaultJitterCoefficient) + float64(rand.Intn(jitterPortion))
	return time.Duration(nextInterval)
}
