// Copyright (c) 2026 Uber Technologies, Inc.
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

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
	"github.com/uber/cadence/service/worker/scheduler"
)

const (
	scheduleWorkflowIDPrefix          = "cadence-scheduler:"
	schedulerWorkflowExecutionTimeout = 10 * 365 * 24 * time.Hour // ~10 years
	schedulerWorkflowDecisionTimeout  = 10 * time.Second
	defaultListSchedulesPageSize      = 10
)

func scheduleWorkflowID(scheduleID string) string {
	return scheduleWorkflowIDPrefix + scheduleID
}

func validateSchedulePolicies(policies *types.SchedulePolicies) error {
	if policies == nil {
		return nil
	}
	if policies.OverlapPolicy == types.ScheduleOverlapPolicySkipNew &&
		policies.CatchUpPolicy == types.ScheduleCatchUpPolicyAll {
		return &types.BadRequestError{
			Message: "SKIP_NEW overlap policy with CATCH_UP_ALL catch-up policy is invalid: " +
				"caught-up fires would be immediately skipped due to overlap with the previous run.",
		}
	}
	return nil
}

// warnIfBufferLimitExceedsSystemLimit logs a warning when buffer_limit exceeds
// MaxBufferedFiresSystemLimit. The value is accepted (the policy still queues
// up to the system limit), but drops at that cap will be tagged
// reason=system_limit rather than reason=user_limit.
func (wh *WorkflowHandler) warnIfBufferLimitExceedsSystemLimit(scheduleID, domainName string, policies *types.SchedulePolicies) {
	if policies == nil ||
		policies.OverlapPolicy != types.ScheduleOverlapPolicyBuffer ||
		int(policies.BufferLimit) <= scheduler.MaxBufferedFiresSystemLimit {
		return
	}
	wh.GetLogger().Warn(
		"buffer_limit exceeds scheduler system limit; drops will be attributed to system_limit",
		tag.WorkflowDomainName(domainName),
		tag.WorkflowID(scheduleWorkflowID(scheduleID)),
		tag.Dynamic("bufferLimit", int(policies.BufferLimit)),
		tag.Dynamic("systemLimit", scheduler.MaxBufferedFiresSystemLimit),
	)
}

// validateUserSearchAttributes rejects user search attribute keys that collide
// with scheduler-reserved keys (CadenceSchedule* prefix). Without this check,
// user values would be silently overwritten by the scheduler workflow's
// UpsertSearchAttributes calls on start and on state change.
func validateUserSearchAttributes(sa *types.SearchAttributes) error {
	if sa == nil {
		return nil
	}
	for k := range sa.IndexedFields {
		if strings.HasPrefix(k, "CadenceSchedule") {
			return &types.BadRequestError{
				Message: fmt.Sprintf("search attribute key %q is reserved for internal use", k),
			}
		}
	}
	return nil
}

func (wh *WorkflowHandler) CreateSchedule(
	ctx context.Context,
	request *types.CreateScheduleRequest,
) (*types.CreateScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}
	if request.GetSpec() == nil {
		return nil, &types.BadRequestError{Message: "Spec is not set on request."}
	}
	if request.GetSpec().GetCronExpression() == "" {
		return nil, &types.BadRequestError{Message: "CronExpression is not set on request."}
	}
	if _, err := backoff.ValidateSchedule(request.GetSpec().GetCronExpression()); err != nil {
		return nil, err
	}
	if request.GetAction() == nil || request.GetAction().GetStartWorkflow() == nil {
		return nil, &types.BadRequestError{Message: "Action.StartWorkflow is not set on request."}
	}
	if err := validateSchedulePolicies(request.GetPolicies()); err != nil {
		return nil, err
	}
	wh.warnIfBufferLimitExceedsSystemLimit(scheduleID, domainName, request.GetPolicies())
	if err := validateUserSearchAttributes(request.GetSearchAttributes()); err != nil {
		return nil, err
	}

	workflowInput := scheduler.SchedulerWorkflowInput{
		Domain:     domainName,
		ScheduleID: scheduleID,
		Spec:       *request.GetSpec(),
		Action:     *request.GetAction(),
	}
	if request.GetPolicies() != nil {
		workflowInput.Policies = *request.GetPolicies()
	}

	inputBytes, err := json.Marshal(workflowInput)
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to serialize scheduler workflow input: %v", err)}
	}

	wfID := scheduleWorkflowID(scheduleID)
	requestID := uuid.New().String()
	reusePolicy := types.WorkflowIDReusePolicyRejectDuplicate
	executionTimeout := int32(schedulerWorkflowExecutionTimeout.Seconds())
	decisionTimeout := int32(schedulerWorkflowDecisionTimeout.Seconds())

	_, err = wh.StartWorkflowExecution(ctx, &types.StartWorkflowExecutionRequest{
		Domain:                              domainName,
		WorkflowID:                          wfID,
		WorkflowType:                        &types.WorkflowType{Name: scheduler.WorkflowTypeName},
		TaskList:                            &types.TaskList{Name: scheduler.TaskListName},
		Input:                               inputBytes,
		ExecutionStartToCloseTimeoutSeconds: &executionTimeout,
		TaskStartToCloseTimeoutSeconds:      &decisionTimeout,
		RequestID:                           requestID,
		WorkflowIDReusePolicy:               &reusePolicy,
		Memo:                                request.GetMemo(),
		SearchAttributes:                    request.GetSearchAttributes(),
	})
	if err != nil {
		var alreadyStarted *types.WorkflowExecutionAlreadyStartedError
		if errors.As(err, &alreadyStarted) {
			return nil, &types.BadRequestError{
				Message: fmt.Sprintf("schedule %q already exists in domain %q", scheduleID, domainName),
			}
		}
		return nil, err
	}

	return &types.CreateScheduleResponse{ScheduleID: scheduleID}, nil
}

func (wh *WorkflowHandler) DescribeSchedule(
	ctx context.Context,
	request *types.DescribeScheduleRequest,
) (*types.DescribeScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}

	wfID := scheduleWorkflowID(scheduleID)
	queryResp, err := wh.QueryWorkflow(ctx, &types.QueryWorkflowRequest{
		Domain: domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: wfID,
		},
		Query: &types.WorkflowQuery{
			QueryType: scheduler.QueryTypeDescribe,
		},
	})
	if err != nil {
		return nil, normalizeScheduleError(err, scheduleID, domainName)
	}

	if queryResp == nil || queryResp.GetQueryResult() == nil {
		return nil, &types.InternalServiceError{Message: "empty query result from scheduler workflow"}
	}

	var desc scheduler.ScheduleDescription
	if err := json.Unmarshal(queryResp.GetQueryResult(), &desc); err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to deserialize scheduler describe response: %v", err)}
	}

	return &types.DescribeScheduleResponse{
		Spec:     &desc.Spec,
		Action:   &desc.Action,
		Policies: &desc.Policies,
		State: &types.ScheduleState{
			Paused: desc.Paused,
			PauseInfo: func() *types.SchedulePauseInfo {
				if !desc.Paused {
					return nil
				}
				return &types.SchedulePauseInfo{
					Reason:   desc.PauseReason,
					PausedBy: desc.PausedBy,
				}
			}(),
		},
		Info: &types.ScheduleInfo{
			LastRunTime: desc.LastRunTime,
			NextRunTime: desc.NextRunTime,
			TotalRuns:   desc.TotalRuns,
		},
	}, nil
}

func (wh *WorkflowHandler) UpdateSchedule(
	ctx context.Context,
	request *types.UpdateScheduleRequest,
) (*types.UpdateScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}
	if request.GetSpec() == nil && request.GetAction() == nil && request.GetPolicies() == nil {
		return nil, &types.BadRequestError{Message: "At least one of Spec, Action, or Policies must be set on request."}
	}
	if err := validateSchedulePolicies(request.GetPolicies()); err != nil {
		return nil, err
	}
	wh.warnIfBufferLimitExceedsSystemLimit(scheduleID, domainName, request.GetPolicies())
	if spec := request.GetSpec(); spec != nil && spec.GetCronExpression() != "" {
		if _, err := backoff.ValidateSchedule(spec.GetCronExpression()); err != nil {
			return nil, err
		}
	}

	signal := scheduler.UpdateSignal{
		Spec:     request.GetSpec(),
		Action:   request.GetAction(),
		Policies: request.GetPolicies(),
	}

	if err := wh.signalScheduleWorkflow(ctx, domainName, scheduleID, scheduler.SignalNameUpdate, signal); err != nil {
		return nil, err
	}
	return &types.UpdateScheduleResponse{}, nil
}

func (wh *WorkflowHandler) DeleteSchedule(
	ctx context.Context,
	request *types.DeleteScheduleRequest,
) (*types.DeleteScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}

	if err := wh.signalScheduleWorkflow(ctx, domainName, scheduleID, scheduler.SignalNameDelete, nil); err != nil {
		return nil, err
	}
	return &types.DeleteScheduleResponse{}, nil
}

func (wh *WorkflowHandler) PauseSchedule(
	ctx context.Context,
	request *types.PauseScheduleRequest,
) (*types.PauseScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}

	signal := scheduler.PauseSignal{
		Reason:   request.GetReason(),
		PausedBy: request.GetIdentity(),
	}

	if err := wh.signalScheduleWorkflow(ctx, domainName, scheduleID, scheduler.SignalNamePause, signal); err != nil {
		return nil, err
	}
	return &types.PauseScheduleResponse{}, nil
}

func (wh *WorkflowHandler) UnpauseSchedule(
	ctx context.Context,
	request *types.UnpauseScheduleRequest,
) (*types.UnpauseScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}

	signal := scheduler.UnpauseSignal{
		Reason:        request.GetReason(),
		CatchUpPolicy: request.GetCatchUpPolicy(),
	}

	if err := wh.signalScheduleWorkflow(ctx, domainName, scheduleID, scheduler.SignalNameUnpause, signal); err != nil {
		return nil, err
	}
	return &types.UnpauseScheduleResponse{}, nil
}

func (wh *WorkflowHandler) BackfillSchedule(
	ctx context.Context,
	request *types.BackfillScheduleRequest,
) (*types.BackfillScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	scheduleID := request.GetScheduleID()
	if scheduleID == "" {
		return nil, &types.BadRequestError{Message: "ScheduleID is not set on request."}
	}
	if request.GetStartTime().IsZero() {
		return nil, &types.BadRequestError{Message: "StartTime is not set on request."}
	}
	if request.GetEndTime().IsZero() {
		return nil, &types.BadRequestError{Message: "EndTime is not set on request."}
	}
	if !request.GetEndTime().After(request.GetStartTime()) {
		return nil, &types.BadRequestError{Message: "EndTime must be after StartTime."}
	}

	signal := scheduler.BackfillSignal{
		StartTime:     request.GetStartTime(),
		EndTime:       request.GetEndTime(),
		OverlapPolicy: request.GetOverlapPolicy(),
		BackfillID:    request.GetBackfillID(),
	}

	if err := wh.signalScheduleWorkflow(ctx, domainName, scheduleID, scheduler.SignalNameBackfill, signal); err != nil {
		return nil, err
	}
	return &types.BackfillScheduleResponse{}, nil
}

func (wh *WorkflowHandler) ListSchedules(
	ctx context.Context,
	request *types.ListSchedulesRequest,
) (*types.ListSchedulesResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}

	pageSize := request.GetPageSize()
	if pageSize <= 0 {
		pageSize = defaultListSchedulesPageSize
	}

	// NOTE: this calls wh.ListWorkflowExecutions directly (not via frontend client),
	// which skips the cluster redirection middleware. For global (XDC) domains, the
	// passive region may return stale visibility data. This applies to all schedule
	// read APIs (Describe, List) and will be addressed when adding XDC support.
	//
	// CloseTime = missing restricts results to open scheduler workflows, so deleted
	// schedules (closed workflows with state.Deleted=true returned nil) no longer
	// surface here. This is the canonical "open only" filter used across the
	// visibility layer (ES, Pinot, SQL-backed stores all recognize it).
	listResp, err := wh.ListWorkflowExecutions(ctx, &types.ListWorkflowExecutionsRequest{
		Domain:        domainName,
		PageSize:      pageSize,
		NextPageToken: request.GetNextPageToken(),
		Query:         fmt.Sprintf("WorkflowType = '%s' and CloseTime = missing", scheduler.WorkflowTypeName),
	})
	if err != nil {
		return nil, err
	}

	entries := make([]*types.ScheduleListEntry, 0, len(listResp.GetExecutions()))
	for _, exec := range listResp.GetExecutions() {
		wfID := exec.GetExecution().GetWorkflowID()
		scheduleID := strings.TrimPrefix(wfID, scheduleWorkflowIDPrefix)

		entry := &types.ScheduleListEntry{
			ScheduleID: scheduleID,
		}

		// Default to not paused. The scheduler workflow upserts these search
		// attributes on start (and ContinueAsNew) and on state change, so a missing
		// value means the workflow hasn't run its first decision task yet (brief
		// window after CreateSchedule).
		paused := false
		var cronExpr, workflowTypeName string
		if exec.SearchAttributes != nil {
			idx := exec.SearchAttributes.IndexedFields
			if stateBytes, ok := idx[scheduler.SearchAttrScheduleState]; ok {
				var stateStr string
				if err := json.Unmarshal(stateBytes, &stateStr); err != nil {
					wh.GetLogger().Warn("failed to unmarshal CadenceScheduleState search attribute, defaulting to active",
						tag.WorkflowID(scheduleWorkflowID(scheduleID)),
						tag.Error(err),
					)
				} else {
					paused = stateStr == scheduler.ScheduleStatePaused
				}
			}
			if cronBytes, ok := idx[scheduler.SearchAttrScheduleCron]; ok {
				if err := json.Unmarshal(cronBytes, &cronExpr); err != nil {
					wh.GetLogger().Warn("failed to unmarshal CadenceScheduleCron search attribute, defaulting to empty",
						tag.WorkflowID(scheduleWorkflowID(scheduleID)),
						tag.Error(err),
					)
				}
			}
			if typeBytes, ok := idx[scheduler.SearchAttrScheduleWorkflowType]; ok {
				if err := json.Unmarshal(typeBytes, &workflowTypeName); err != nil {
					wh.GetLogger().Warn("failed to unmarshal CadenceScheduleWorkflowType search attribute, defaulting to empty",
						tag.WorkflowID(scheduleWorkflowID(scheduleID)),
						tag.Error(err),
					)
				}
			}
		}
		entry.State = &types.ScheduleState{Paused: paused}
		entry.CronExpression = cronExpr
		if workflowTypeName != "" {
			entry.WorkflowType = &types.WorkflowType{Name: workflowTypeName}
		}

		entries = append(entries, entry)
	}

	return &types.ListSchedulesResponse{
		Schedules:     entries,
		NextPageToken: listResp.GetNextPageToken(),
	}, nil
}

func (wh *WorkflowHandler) signalScheduleWorkflow(
	ctx context.Context,
	domainName string,
	scheduleID string,
	signalName string,
	signalInput interface{},
) error {
	var inputBytes []byte
	var err error
	if signalInput != nil {
		inputBytes, err = json.Marshal(signalInput)
		if err != nil {
			return &types.InternalServiceError{Message: fmt.Sprintf("failed to serialize signal input: %v", err)}
		}
	}

	wfID := scheduleWorkflowID(scheduleID)
	err = wh.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain: domainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: wfID,
		},
		SignalName: signalName,
		Input:      inputBytes,
		RequestID:  uuid.New().String(),
	})
	if err != nil {
		return normalizeScheduleError(err, scheduleID, domainName)
	}
	return nil
}

func normalizeScheduleError(err error, scheduleID, domainName string) error {
	var notFound *types.EntityNotExistsError
	if errors.As(err, &notFound) {
		return &types.EntityNotExistsError{
			Message: fmt.Sprintf("schedule %q not found in domain %q", scheduleID, domainName),
		}
	}
	return err
}
