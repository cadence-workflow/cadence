// Copyright (c) 2017-2020 Uber Technologies, Inc.
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
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/yarpc"
	"golang.org/x/sync/errgroup"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/elasticsearch/validator"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	persistenceutils "github.com/uber/cadence/common/persistence/persistence-utils"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
	"github.com/uber/cadence/service/frontend/config"
	"github.com/uber/cadence/service/frontend/validate"
	"github.com/uber/cadence/service/worker/diagnostics"
)

const (
	// HealthStatusOK is used when this node is healthy and rpc requests are allowed
	HealthStatusOK HealthStatus = iota + 1
	// HealthStatusWarmingUp is used when the rpc handler is warming up
	HealthStatusWarmingUp
	// HealthStatusShuttingDown is used when the rpc handler is shutting down
	HealthStatusShuttingDown
)

var _ Handler = (*WorkflowHandler)(nil)

type (
	// WorkflowHandler - Thrift handler interface for workflow service
	WorkflowHandler struct {
		resource.Resource

		shuttingDown              int32
		healthStatus              int32
		tokenSerializer           common.TaskTokenSerializer
		config                    *config.Config
		versionChecker            client.VersionChecker
		domainHandler             domain.Handler
		visibilityQueryValidator  *validator.VisibilityQueryValidator
		searchAttributesValidator *validator.SearchAttributesValidator
		throttleRetry             *backoff.ThrottleRetry
		producerManager           ProducerManager
		thriftrwEncoder           codec.BinaryEncoder
		requestValidator          RequestValidator
	}

	getHistoryContinuationToken struct {
		RunID              string
		FirstEventID       int64
		NextEventID        int64
		IsWorkflowRunning  bool
		PersistenceToken   []byte
		TransientDecision  *types.TransientDecisionInfo
		BranchToken        []byte
		VersionHistoryItem *types.VersionHistoryItem
	}

	domainGetter interface {
		GetDomain() string
	}

	// HealthStatus is an enum that refers to the rpc handler health status
	HealthStatus int32
)

var (
	frontendServiceRetryPolicy = common.CreateFrontendServiceRetryPolicy()
)

// NewWorkflowHandler creates a thrift handler for the cadence service
func NewWorkflowHandler(
	resource resource.Resource,
	config *config.Config,
	versionChecker client.VersionChecker,
	domainHandler domain.Handler,
) *WorkflowHandler {
	return &WorkflowHandler{
		Resource:        resource,
		config:          config,
		healthStatus:    int32(HealthStatusWarmingUp),
		tokenSerializer: common.NewJSONTaskTokenSerializer(),
		versionChecker:  versionChecker,
		domainHandler:   domainHandler,
		visibilityQueryValidator: validator.NewQueryValidator(
			config.ValidSearchAttributes,
			config.EnableQueryAttributeValidation,
		),
		searchAttributesValidator: validator.NewSearchAttributesValidator(
			resource.GetLogger(),
			config.EnableQueryAttributeValidation,
			config.ValidSearchAttributes,
			config.SearchAttributesNumberOfKeysLimit,
			config.SearchAttributesSizeOfValueLimit,
			config.SearchAttributesTotalSizeLimit,
		),
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(frontendServiceRetryPolicy),
			backoff.WithRetryableError(common.IsServiceTransientError),
		),
		producerManager: NewProducerManager(
			resource.GetDomainCache(),
			resource.GetAsyncWorkflowQueueProvider(),
			resource.GetLogger(),
			resource.GetMetricsClient(),
		),
		thriftrwEncoder:  codec.NewThriftRWEncoder(),
		requestValidator: NewRequestValidator(resource.GetLogger(), resource.GetMetricsClient(), config),
	}
}

// Start starts the handler
func (wh *WorkflowHandler) Start() {
	// TODO: Get warmup duration from config. Even better, run proactive checks such as probing downstream connections.
	const warmUpDuration = 30 * time.Second

	warmupTimer := time.NewTimer(warmUpDuration)
	go func() {
		<-warmupTimer.C
		wh.GetLogger().Warn("Service warmup duration has elapsed.")
		if atomic.CompareAndSwapInt32(&wh.healthStatus, int32(HealthStatusWarmingUp), int32(HealthStatusOK)) {
			wh.GetLogger().Warn("Warmup time has elapsed. Service is healthy.")
		} else {
			status := HealthStatus(atomic.LoadInt32(&wh.healthStatus))
			wh.GetLogger().Warn(fmt.Sprintf("Warmup time has elapsed. Service status is: %v", status.String()))
		}
	}()
}

// Stop stops the handler
func (wh *WorkflowHandler) Stop() {
	atomic.StoreInt32(&wh.shuttingDown, 1)
}

// UpdateHealthStatus sets the health status for this rpc handler.
// This health status will be used within the rpc health check handler
func (wh *WorkflowHandler) UpdateHealthStatus(status HealthStatus) {
	atomic.StoreInt32(&wh.healthStatus, int32(status))
}

func (wh *WorkflowHandler) isShuttingDown() bool {
	return atomic.LoadInt32(&wh.shuttingDown) != 0
}

// Health is for health check
func (wh *WorkflowHandler) Health(ctx context.Context) (*types.HealthStatus, error) {
	status := HealthStatus(atomic.LoadInt32(&wh.healthStatus))
	msg := status.String()

	if status != HealthStatusOK {
		wh.GetLogger().Warn(fmt.Sprintf("Service status is: %v", msg))
	}

	return &types.HealthStatus{
		Ok:  status == HealthStatusOK,
		Msg: msg,
	}, nil
}

// DiagnoseWorkflowExecution is to diagnose a workflow execution
func (wh *WorkflowHandler) DiagnoseWorkflowExecution(ctx context.Context, request *types.DiagnoseWorkflowExecutionRequest) (*types.DiagnoseWorkflowExecutionResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	if request.GetDomain() == "" {
		return nil, validate.ErrDomainNotSet
	}

	if request.GetWorkflowExecution().GetWorkflowID() == "" || request.GetWorkflowExecution().GetRunID() == "" {
		return nil, validate.ErrExecutionNotSet
	}

	wfExecution := request.GetWorkflowExecution()
	diagnosticWorkflowDomain := "cadence-system"
	diagnosticWorkflowID := fmt.Sprintf("%s-%s", request.GetDomain(), wfExecution.GetRunID())

	diagnosticWorkflowInput := diagnostics.DiagnosticsStarterWorkflowInput{
		Domain:     request.GetDomain(),
		WorkflowID: request.GetWorkflowExecution().GetWorkflowID(),
		RunID:      request.GetWorkflowExecution().GetRunID(),
		Identity:   request.GetIdentity(),
	}
	inputInBytes, err := json.Marshal(diagnosticWorkflowInput)
	if err != nil {
		return nil, err
	}

	resp, err := wh.StartWorkflowExecution(ctx, &types.StartWorkflowExecutionRequest{
		Domain:     diagnosticWorkflowDomain,
		WorkflowID: diagnosticWorkflowID,
		WorkflowType: &types.WorkflowType{
			Name: "diagnostics-starter-workflow",
		},
		TaskList: &types.TaskList{
			Name: "diagnostics-wf-tasklist",
		},
		Input:                               inputInBytes,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(86400), // 24 hours
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(300),   // 5 minutes
		Identity:                            request.GetIdentity(),
		RequestID:                           uuid.New().String(),
		WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
	})
	if err != nil {
		return nil, err
	}

	return &types.DiagnoseWorkflowExecutionResponse{
		Domain: diagnosticWorkflowDomain,
		DiagnosticWorkflowExecution: &types.WorkflowExecution{
			WorkflowID: diagnosticWorkflowID,
			RunID:      resp.GetRunID(),
		},
	}, nil
}

// PollForActivityTask - Poll for an activity task.
func (wh *WorkflowHandler) PollForActivityTask(
	ctx context.Context,
	pollRequest *types.PollForActivityTaskRequest,
) (resp *types.PollForActivityTaskResponse, retError error) {
	callTime := time.Now()

	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if pollRequest == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := pollRequest.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}

	scope := getMetricsScopeWithDomain(metrics.FrontendPollForActivityTaskScope, pollRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	wh.GetLogger().Debug("Received PollForActivityTask")
	if err := common.ValidateLongPollContextTimeout(
		ctx,
		"PollForActivityTask",
		wh.GetThrottledLogger().WithTags(tag.WorkflowDomainName(domainName), tag.WorkflowTaskListName(pollRequest.GetTaskList().GetName())),
	); err != nil {
		return nil, err
	}

	idLengthWarnLimit := wh.config.MaxIDLengthWarnLimit()
	if !common.IsValidIDLength(
		domainName,
		scope,
		idLengthWarnLimit,
		wh.config.DomainNameMaxLength(domainName),
		metrics.CadenceErrDomainNameExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeDomainName) {
		return nil, validate.ErrDomainTooLong
	}

	if err := wh.validateTaskList(pollRequest.TaskList, scope, domainName); err != nil {
		return nil, err
	}

	if !common.IsValidIDLength(
		pollRequest.GetIdentity(),
		scope,
		idLengthWarnLimit,
		wh.config.IdentityMaxLength(domainName),
		metrics.CadenceErrIdentityExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeIdentity) {
		return nil, validate.ErrIdentityTooLong
	}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}

	isolationGroup := wh.getIsolationGroup(ctx, domainName)
	if !wh.waitUntilIsolationGroupHealthy(ctx, domainName, isolationGroup) {
		return &types.PollForActivityTaskResponse{}, nil
	}
	// it is possible that we wait for a very long time and the remaining time is not long enough for a long poll
	// in this case, return an empty response
	if err := common.ValidateLongPollContextTimeout(
		ctx,
		"PollForActivityTask",
		wh.GetThrottledLogger().WithTags(tag.WorkflowDomainName(domainName), tag.WorkflowTaskListName(pollRequest.GetTaskList().GetName())),
	); err != nil {
		return &types.PollForActivityTaskResponse{}, nil
	}
	pollerID := uuid.New().String()
	var matchingResp *types.MatchingPollForActivityTaskResponse
	op := func() error {
		matchingResp, err = wh.GetMatchingClient().PollForActivityTask(ctx, &types.MatchingPollForActivityTaskRequest{
			DomainUUID:     domainID,
			PollerID:       pollerID,
			PollRequest:    pollRequest,
			IsolationGroup: isolationGroup,
		})
		return err
	}

	err = wh.throttleRetry.Do(ctx, op)
	if err != nil {
		err = wh.cancelOutstandingPoll(ctx, err, domainID, persistence.TaskListTypeActivity, pollRequest.TaskList, pollerID)
		if err != nil {
			// For all other errors log an error and return it back to client.
			ctxTimeout := "not-set"
			ctxDeadline, ok := ctx.Deadline()
			if ok {
				ctxTimeout = ctxDeadline.Sub(callTime).String()
			}
			wh.GetLogger().Error("PollForActivityTask failed.",
				tag.WorkflowTaskListName(pollRequest.GetTaskList().GetName()),
				tag.Value(ctxTimeout),
				tag.Error(err))
			return nil, err
		}
		// Must be cancellation error.  Doesn't matter what we return here.  Client already went away.
		return nil, nil
	}

	return &types.PollForActivityTaskResponse{
		TaskToken:                       matchingResp.TaskToken,
		WorkflowExecution:               matchingResp.WorkflowExecution,
		ActivityID:                      matchingResp.ActivityID,
		ActivityType:                    matchingResp.ActivityType,
		Input:                           matchingResp.Input,
		ScheduledTimestamp:              matchingResp.ScheduledTimestamp,
		ScheduleToCloseTimeoutSeconds:   matchingResp.ScheduleToCloseTimeoutSeconds,
		StartedTimestamp:                matchingResp.StartedTimestamp,
		StartToCloseTimeoutSeconds:      matchingResp.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:         matchingResp.HeartbeatTimeoutSeconds,
		Attempt:                         matchingResp.Attempt,
		ScheduledTimestampOfThisAttempt: matchingResp.ScheduledTimestampOfThisAttempt,
		HeartbeatDetails:                matchingResp.HeartbeatDetails,
		WorkflowType:                    matchingResp.WorkflowType,
		WorkflowDomain:                  matchingResp.WorkflowDomain,
		Header:                          matchingResp.Header,
		AutoConfigHint:                  matchingResp.AutoConfigHint,
	}, nil
}

// PollForDecisionTask - Poll for a decision task.
func (wh *WorkflowHandler) PollForDecisionTask(
	ctx context.Context,
	pollRequest *types.PollForDecisionTaskRequest,
) (resp *types.PollForDecisionTaskResponse, retError error) {
	callTime := time.Now()

	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if pollRequest == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := pollRequest.GetDomain()
	tags := []tag.Tag{tag.WorkflowDomainName(domainName)}

	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}

	scope := getMetricsScopeWithDomain(metrics.FrontendPollForDecisionTaskScope, pollRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	wh.GetLogger().Debug("Received PollForDecisionTask")
	if err := common.ValidateLongPollContextTimeout(
		ctx,
		"PollForDecisionTask",
		wh.GetThrottledLogger().WithTags(tag.WorkflowDomainName(domainName), tag.WorkflowTaskListName(pollRequest.GetTaskList().GetName())),
	); err != nil {
		return nil, err
	}

	idLengthWarnLimit := wh.config.MaxIDLengthWarnLimit()
	if !common.IsValidIDLength(
		domainName,
		scope,
		idLengthWarnLimit,
		wh.config.DomainNameMaxLength(domainName),
		metrics.CadenceErrDomainNameExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeDomainName) {
		return nil, validate.ErrDomainTooLong
	}

	if !common.IsValidIDLength(
		pollRequest.GetIdentity(),
		scope,
		idLengthWarnLimit,
		wh.config.IdentityMaxLength(domainName),
		metrics.CadenceErrIdentityExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeIdentity) {
		return nil, validate.ErrIdentityTooLong
	}

	if err := wh.validateTaskList(pollRequest.TaskList, scope, domainName); err != nil {
		return nil, err
	}

	domainEntry, err := wh.GetDomainCache().GetDomain(domainName)
	if err != nil {
		return nil, err
	}
	domainID := domainEntry.GetInfo().ID

	wh.GetLogger().Debug("Poll for decision.", tag.WorkflowDomainName(domainName), tag.WorkflowDomainID(domainID))
	if err := wh.checkBadBinary(domainEntry, pollRequest.GetBinaryChecksum()); err != nil {
		return nil, err
	}

	isolationGroup := wh.getIsolationGroup(ctx, domainName)
	if !wh.waitUntilIsolationGroupHealthy(ctx, domainName, isolationGroup) {
		return &types.PollForDecisionTaskResponse{}, nil
	}
	// it is possible that we wait for a very long time and the remaining time is not long enough for a long poll
	// in this case, return an empty response
	if err := common.ValidateLongPollContextTimeout(
		ctx,
		"PollForDecisionTask",
		wh.GetThrottledLogger().WithTags(tag.WorkflowDomainName(domainName), tag.WorkflowTaskListName(pollRequest.GetTaskList().GetName())),
	); err != nil {
		return &types.PollForDecisionTaskResponse{}, nil
	}

	pollerID := uuid.New().String()
	var matchingResp *types.MatchingPollForDecisionTaskResponse
	op := func() error {
		matchingResp, err = wh.GetMatchingClient().PollForDecisionTask(ctx, &types.MatchingPollForDecisionTaskRequest{
			DomainUUID:     domainID,
			PollerID:       pollerID,
			PollRequest:    pollRequest,
			IsolationGroup: isolationGroup,
		})
		return err
	}

	err = wh.throttleRetry.Do(ctx, op)
	if err != nil {
		err = wh.cancelOutstandingPoll(ctx, err, domainID, persistence.TaskListTypeDecision, pollRequest.TaskList, pollerID)
		if err != nil {
			// For all other errors log an error and return it back to client.
			ctxTimeout := "not-set"
			ctxDeadline, ok := ctx.Deadline()
			if ok {
				ctxTimeout = ctxDeadline.Sub(callTime).String()
			}
			wh.GetLogger().Error("PollForDecisionTask failed.",
				tag.WorkflowTaskListName(pollRequest.GetTaskList().GetName()),
				tag.Value(ctxTimeout),
				tag.Error(err))
			return nil, err
		}

		// Must be cancellation error.  Doesn't matter what we return here.  Client already went away.
		return nil, nil
	}

	tags = append(tags, []tag.Tag{tag.WorkflowID(
		matchingResp.GetWorkflowExecution().GetWorkflowID()),
		tag.WorkflowRunID(matchingResp.GetWorkflowExecution().GetRunID())}...)
	resp, err = wh.createPollForDecisionTaskResponse(ctx, scope, domainID, matchingResp, matchingResp.GetBranchToken())
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (wh *WorkflowHandler) getIsolationGroup(ctx context.Context, domainName string) string {
	return isolationgroup.IsolationGroupFromContext(ctx)
}

func (wh *WorkflowHandler) getPartitionConfig(ctx context.Context, domainName string) map[string]string {
	return isolationgroup.ConfigFromContext(ctx)
}

func (wh *WorkflowHandler) isIsolationGroupHealthy(ctx context.Context, domainName, isolationGroup string) bool {
	if wh.GetIsolationGroupState() != nil && wh.config.EnableTasklistIsolation(domainName) {
		isDrained, err := wh.GetIsolationGroupState().IsDrained(ctx, domainName, isolationGroup)
		if err != nil {
			wh.GetLogger().Error("Failed to check if an isolation group is drained, assume it's healthy", tag.Error(err))
			return true
		}
		return !isDrained
	}
	return true
}

func (wh *WorkflowHandler) waitUntilIsolationGroupHealthy(ctx context.Context, domainName, isolationGroup string) bool {
	if wh.GetIsolationGroupState() != nil && wh.config.EnableTasklistIsolation(domainName) {
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		childCtx, cancel := common.CreateChildContext(ctx, 0.05)
		defer cancel()
		for {
			isDrained, err := wh.GetIsolationGroupState().IsDrained(childCtx, domainName, isolationGroup)
			if err != nil {
				wh.GetLogger().Error("Failed to check if an isolation group is drained, assume it's healthy", tag.Error(err))
				return true
			}
			if !isDrained {
				break
			}
			select {
			case <-childCtx.Done():
				return false
			case <-ticker.C:
			}
		}
	}
	return true
}

func (wh *WorkflowHandler) checkBadBinary(domainEntry *cache.DomainCacheEntry, binaryChecksum string) error {
	if domainEntry.GetConfig().BadBinaries.Binaries != nil {
		badBinaries := domainEntry.GetConfig().BadBinaries.Binaries
		_, ok := badBinaries[binaryChecksum]
		if ok {
			wh.GetMetricsClient().IncCounter(metrics.FrontendPollForDecisionTaskScope, metrics.CadenceErrBadBinaryCounter)
			return &types.BadRequestError{
				Message: fmt.Sprintf("binary %v already marked as bad deployment", binaryChecksum),
			}
		}
	}
	return nil
}

func (wh *WorkflowHandler) cancelOutstandingPoll(ctx context.Context, err error, domainID string, taskListType int32,
	taskList *types.TaskList, pollerID string) error {
	// First check if this err is due to context cancellation.  This means client connection to frontend is closed.
	if ctx.Err() == context.Canceled {
		// Our rpc stack does not propagates context cancellation to the other service.  Lets make an explicit
		// call to matching to notify this poller is gone to prevent any tasks being dispatched to zombie pollers.
		err = wh.GetMatchingClient().CancelOutstandingPoll(context.Background(), &types.CancelOutstandingPollRequest{
			DomainUUID:   domainID,
			TaskListType: common.Int32Ptr(taskListType),
			TaskList:     taskList,
			PollerID:     pollerID,
		})
		// We can not do much if this call fails.  Just log the error and move on
		if err != nil {
			wh.GetLogger().Warn("Failed to cancel outstanding poller.",
				tag.WorkflowTaskListName(taskList.GetName()), tag.Error(err))
		}

		// Clear error as we don't want to report context cancellation error to count against our SLA
		return nil
	}

	return err
}

// RecordActivityTaskHeartbeat - Record Activity Task Heart beat.
func (wh *WorkflowHandler) RecordActivityTaskHeartbeat(
	ctx context.Context,
	heartbeatRequest *types.RecordActivityTaskHeartbeatRequest,
) (resp *types.RecordActivityTaskHeartbeatResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if heartbeatRequest == nil {
		return nil, validate.ErrRequestNotSet
	}

	wh.GetLogger().Debug("Received RecordActivityTaskHeartbeat")
	if heartbeatRequest.TaskToken == nil {
		return nil, validate.ErrTaskTokenNotSet
	}
	taskToken, err := wh.tokenSerializer.Deserialize(heartbeatRequest.TaskToken)
	if err != nil {
		return nil, err
	}
	if taskToken.DomainID == "" {
		return nil, validate.ErrDomainNotSet
	}

	domainName, err := wh.GetDomainCache().GetDomainName(taskToken.DomainID)
	if err != nil {
		return nil, err
	}

	dw := domainWrapper{
		domain: domainName,
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendRecordActivityTaskHeartbeatScope, dw, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	if err := common.CheckEventBlobSizeLimit(
		len(heartbeatRequest.Details),
		sizeLimitWarn,
		sizeLimitError,
		taskToken.DomainID,
		domainName,
		taskToken.WorkflowID,
		taskToken.RunID,
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("RecordActivityTaskHeartbeat"),
	); err != nil {
		// heartbeat details exceed size limit, we would fail the activity immediately with explicit error reason
		failRequest := &types.RespondActivityTaskFailedRequest{
			TaskToken: heartbeatRequest.TaskToken,
			Reason:    common.StringPtr(common.FailureReasonHeartbeatExceedsLimit),
			Details:   heartbeatRequest.Details[0:sizeLimitError],
			Identity:  heartbeatRequest.Identity,
		}
		err = wh.GetHistoryClient().RespondActivityTaskFailed(ctx, &types.HistoryRespondActivityTaskFailedRequest{
			DomainUUID:    taskToken.DomainID,
			FailedRequest: failRequest,
		})
		if err != nil {
			return nil, wh.normalizeVersionedErrors(ctx, err)
		}
		resp = &types.RecordActivityTaskHeartbeatResponse{CancelRequested: true}
	} else {
		resp, err = wh.GetHistoryClient().RecordActivityTaskHeartbeat(ctx, &types.HistoryRecordActivityTaskHeartbeatRequest{
			DomainUUID:       taskToken.DomainID,
			HeartbeatRequest: heartbeatRequest,
		})
		if err != nil {
			return nil, wh.normalizeVersionedErrors(ctx, err)
		}
	}

	return resp, nil
}

// RecordActivityTaskHeartbeatByID - Record Activity Task Heart beat.
func (wh *WorkflowHandler) RecordActivityTaskHeartbeatByID(
	ctx context.Context,
	heartbeatRequest *types.RecordActivityTaskHeartbeatByIDRequest,
) (resp *types.RecordActivityTaskHeartbeatResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if heartbeatRequest == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := heartbeatRequest.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}

	wh.GetLogger().Debug("Received RecordActivityTaskHeartbeatByID")
	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}
	workflowID := heartbeatRequest.GetWorkflowID()
	runID := heartbeatRequest.GetRunID() // runID is optional so can be empty
	activityID := heartbeatRequest.GetActivityID()

	if domainID == "" {
		return nil, validate.ErrDomainNotSet
	}
	if workflowID == "" {
		return nil, validate.ErrWorkflowIDNotSet
	}
	if activityID == "" {
		return nil, validate.ErrActivityIDNotSet
	}

	taskToken := &common.TaskToken{
		DomainID:   domainID,
		RunID:      runID,
		WorkflowID: workflowID,
		ScheduleID: constants.EmptyEventID,
		ActivityID: activityID,
	}
	token, err := wh.tokenSerializer.Serialize(taskToken)
	if err != nil {
		return nil, err
	}

	scope := getMetricsScopeWithDomain(metrics.FrontendRecordActivityTaskHeartbeatByIDScope, heartbeatRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	if err := common.CheckEventBlobSizeLimit(
		len(heartbeatRequest.Details),
		sizeLimitWarn,
		sizeLimitError,
		taskToken.DomainID,
		domainName,
		taskToken.WorkflowID,
		taskToken.RunID,
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("RecordActivityTaskHeartbeatByID"),
	); err != nil {
		// heartbeat details exceed size limit, we would fail the activity immediately with explicit error reason
		failRequest := &types.RespondActivityTaskFailedRequest{
			TaskToken: token,
			Reason:    common.StringPtr(common.FailureReasonHeartbeatExceedsLimit),
			Details:   heartbeatRequest.Details[0:sizeLimitError],
			Identity:  heartbeatRequest.Identity,
		}
		err = wh.GetHistoryClient().RespondActivityTaskFailed(ctx, &types.HistoryRespondActivityTaskFailedRequest{
			DomainUUID:    taskToken.DomainID,
			FailedRequest: failRequest,
		})
		if err != nil {
			return nil, wh.normalizeVersionedErrors(ctx, err)
		}
		resp = &types.RecordActivityTaskHeartbeatResponse{CancelRequested: true}
	} else {
		req := &types.RecordActivityTaskHeartbeatRequest{
			TaskToken: token,
			Details:   heartbeatRequest.Details,
			Identity:  heartbeatRequest.Identity,
		}

		resp, err = wh.GetHistoryClient().RecordActivityTaskHeartbeat(ctx, &types.HistoryRecordActivityTaskHeartbeatRequest{
			DomainUUID:       taskToken.DomainID,
			HeartbeatRequest: req,
		})
		if err != nil {
			return nil, wh.normalizeVersionedErrors(ctx, err)
		}
	}

	return resp, nil
}

// RespondActivityTaskCompleted - response to an activity task
func (wh *WorkflowHandler) RespondActivityTaskCompleted(
	ctx context.Context,
	completeRequest *types.RespondActivityTaskCompletedRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if completeRequest == nil {
		return validate.ErrRequestNotSet
	}

	if completeRequest.TaskToken == nil {
		return validate.ErrTaskTokenNotSet
	}
	taskToken, err := wh.tokenSerializer.Deserialize(completeRequest.TaskToken)
	if err != nil {
		return err
	}
	if taskToken.DomainID == "" {
		return validate.ErrDomainNotSet
	}

	domainName, err := wh.GetDomainCache().GetDomainName(taskToken.DomainID)
	if err != nil {
		return err
	}

	dw := domainWrapper{
		domain: domainName,
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendRespondActivityTaskCompletedScope, dw, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if !common.IsValidIDLength(
		completeRequest.GetIdentity(),
		scope,
		wh.config.MaxIDLengthWarnLimit(),
		wh.config.IdentityMaxLength(domainName),
		metrics.CadenceErrIdentityExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeIdentity) {
		return validate.ErrIdentityTooLong
	}

	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	if err := common.CheckEventBlobSizeLimit(
		len(completeRequest.Result),
		sizeLimitWarn,
		sizeLimitError,
		taskToken.DomainID,
		domainName,
		taskToken.WorkflowID,
		taskToken.RunID,
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("RespondActivityTaskCompleted"),
	); err != nil {
		// result exceeds blob size limit, we would record it as failure
		failRequest := &types.RespondActivityTaskFailedRequest{
			TaskToken: completeRequest.TaskToken,
			Reason:    common.StringPtr(common.FailureReasonCompleteResultExceedsLimit),
			Details:   completeRequest.Result[0:sizeLimitError],
			Identity:  completeRequest.Identity,
		}
		err = wh.GetHistoryClient().RespondActivityTaskFailed(ctx, &types.HistoryRespondActivityTaskFailedRequest{
			DomainUUID:    taskToken.DomainID,
			FailedRequest: failRequest,
		})
		if err != nil {
			return wh.normalizeVersionedErrors(ctx, err)
		}
	} else {
		err = wh.GetHistoryClient().RespondActivityTaskCompleted(ctx, &types.HistoryRespondActivityTaskCompletedRequest{
			DomainUUID:      taskToken.DomainID,
			CompleteRequest: completeRequest,
		})
		if err != nil {
			return wh.normalizeVersionedErrors(ctx, err)
		}
	}

	return nil
}

// RespondActivityTaskCompletedByID - response to an activity task
func (wh *WorkflowHandler) RespondActivityTaskCompletedByID(
	ctx context.Context,
	completeRequest *types.RespondActivityTaskCompletedByIDRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if completeRequest == nil {
		return validate.ErrRequestNotSet
	}

	domainName := completeRequest.GetDomain()
	if domainName == "" {
		return validate.ErrDomainNotSet
	}
	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return err
	}
	workflowID := completeRequest.GetWorkflowID()
	runID := completeRequest.GetRunID() // runID is optional so can be empty
	activityID := completeRequest.GetActivityID()

	if domainID == "" {
		return validate.ErrDomainNotSet
	}
	if workflowID == "" {
		return validate.ErrWorkflowIDNotSet
	}
	if activityID == "" {
		return validate.ErrActivityIDNotSet
	}

	scope := getMetricsScopeWithDomain(metrics.FrontendRespondActivityTaskCompletedByIDScope, completeRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if !common.IsValidIDLength(
		completeRequest.GetIdentity(),
		scope,
		wh.config.MaxIDLengthWarnLimit(),
		wh.config.IdentityMaxLength(domainName),
		metrics.CadenceErrIdentityExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeIdentity) {
		return validate.ErrIdentityTooLong
	}

	taskToken := &common.TaskToken{
		DomainID:   domainID,
		RunID:      runID,
		WorkflowID: workflowID,
		ScheduleID: constants.EmptyEventID,
		ActivityID: activityID,
	}
	token, err := wh.tokenSerializer.Serialize(taskToken)
	if err != nil {
		return err
	}

	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	if err := common.CheckEventBlobSizeLimit(
		len(completeRequest.Result),
		sizeLimitWarn,
		sizeLimitError,
		taskToken.DomainID,
		domainName,
		taskToken.WorkflowID,
		taskToken.RunID,
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("RespondActivityTaskCompletedByID"),
	); err != nil {
		// result exceeds blob size limit, we would record it as failure
		failRequest := &types.RespondActivityTaskFailedRequest{
			TaskToken: token,
			Reason:    common.StringPtr(common.FailureReasonCompleteResultExceedsLimit),
			Details:   completeRequest.Result[0:sizeLimitError],
			Identity:  completeRequest.Identity,
		}
		err = wh.GetHistoryClient().RespondActivityTaskFailed(ctx, &types.HistoryRespondActivityTaskFailedRequest{
			DomainUUID:    taskToken.DomainID,
			FailedRequest: failRequest,
		})
		if err != nil {
			return wh.normalizeVersionedErrors(ctx, err)
		}
	} else {
		req := &types.RespondActivityTaskCompletedRequest{
			TaskToken: token,
			Result:    completeRequest.Result,
			Identity:  completeRequest.Identity,
		}

		err = wh.GetHistoryClient().RespondActivityTaskCompleted(ctx, &types.HistoryRespondActivityTaskCompletedRequest{
			DomainUUID:      taskToken.DomainID,
			CompleteRequest: req,
		})
		if err != nil {
			return wh.normalizeVersionedErrors(ctx, err)
		}
	}

	return nil
}

// RespondActivityTaskFailed - response to an activity task failure
func (wh *WorkflowHandler) RespondActivityTaskFailed(
	ctx context.Context,
	failedRequest *types.RespondActivityTaskFailedRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if failedRequest == nil {
		return validate.ErrRequestNotSet
	}

	if failedRequest.TaskToken == nil {
		return validate.ErrTaskTokenNotSet
	}
	taskToken, err := wh.tokenSerializer.Deserialize(failedRequest.TaskToken)
	if err != nil {
		return err
	}
	if taskToken.DomainID == "" {
		return validate.ErrDomainNotSet
	}

	domainName, err := wh.GetDomainCache().GetDomainName(taskToken.DomainID)
	if err != nil {
		return err
	}

	dw := domainWrapper{
		domain: domainName,
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendRespondActivityTaskFailedScope, dw, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if !common.IsValidIDLength(
		failedRequest.GetIdentity(),
		scope,
		wh.config.MaxIDLengthWarnLimit(),
		wh.config.IdentityMaxLength(domainName),
		metrics.CadenceErrIdentityExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeIdentity) {
		return validate.ErrIdentityTooLong
	}

	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	if err := common.CheckEventBlobSizeLimit(
		len(failedRequest.Details),
		sizeLimitWarn,
		sizeLimitError,
		taskToken.DomainID,
		domainName,
		taskToken.WorkflowID,
		taskToken.RunID,
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("RespondActivityTaskFailed"),
	); err != nil {
		// details exceeds blob size limit, we would truncate the details and put a specific error reason
		failedRequest.Reason = common.StringPtr(common.FailureReasonFailureDetailsExceedsLimit)
		failedRequest.Details = failedRequest.Details[0:sizeLimitError]
	}

	err = wh.GetHistoryClient().RespondActivityTaskFailed(ctx, &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID:    taskToken.DomainID,
		FailedRequest: failedRequest,
	})
	if err != nil {
		return wh.normalizeVersionedErrors(ctx, err)
	}
	return nil
}

// RespondActivityTaskFailedByID - response to an activity task failure
func (wh *WorkflowHandler) RespondActivityTaskFailedByID(
	ctx context.Context,
	failedRequest *types.RespondActivityTaskFailedByIDRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if failedRequest == nil {
		return validate.ErrRequestNotSet
	}

	domainName := failedRequest.GetDomain()

	if domainName == "" {
		return validate.ErrDomainNotSet
	}
	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return err
	}
	workflowID := failedRequest.GetWorkflowID()
	runID := failedRequest.GetRunID() // runID is optional so can be empty
	activityID := failedRequest.GetActivityID()

	if domainID == "" {
		return validate.ErrDomainNotSet
	}
	if workflowID == "" {
		return validate.ErrWorkflowIDNotSet
	}
	if activityID == "" {
		return validate.ErrActivityIDNotSet
	}

	scope := getMetricsScopeWithDomain(metrics.FrontendRespondActivityTaskFailedByIDScope, failedRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if !common.IsValidIDLength(
		failedRequest.GetIdentity(),
		scope,
		wh.config.MaxIDLengthWarnLimit(),
		wh.config.IdentityMaxLength(failedRequest.GetDomain()),
		metrics.CadenceErrIdentityExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeIdentity) {
		return validate.ErrIdentityTooLong
	}

	taskToken := &common.TaskToken{
		DomainID:   domainID,
		RunID:      runID,
		WorkflowID: workflowID,
		ScheduleID: constants.EmptyEventID,
		ActivityID: activityID,
	}
	token, err := wh.tokenSerializer.Serialize(taskToken)
	if err != nil {
		return err
	}

	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	if err := common.CheckEventBlobSizeLimit(
		len(failedRequest.Details),
		sizeLimitWarn,
		sizeLimitError,
		taskToken.DomainID,
		domainName,
		taskToken.WorkflowID,
		taskToken.RunID,
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("RespondActivityTaskFailedByID"),
	); err != nil {
		// details exceeds blob size limit, we would truncate the details and put a specific error reason
		failedRequest.Reason = common.StringPtr(common.FailureReasonFailureDetailsExceedsLimit)
		failedRequest.Details = failedRequest.Details[0:sizeLimitError]
	}

	req := &types.RespondActivityTaskFailedRequest{
		TaskToken: token,
		Reason:    failedRequest.Reason,
		Details:   failedRequest.Details,
		Identity:  failedRequest.Identity,
	}

	err = wh.GetHistoryClient().RespondActivityTaskFailed(ctx, &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID:    taskToken.DomainID,
		FailedRequest: req,
	})
	if err != nil {
		return wh.normalizeVersionedErrors(ctx, err)
	}
	return nil
}

// RespondActivityTaskCanceled - called to cancel an activity task
func (wh *WorkflowHandler) RespondActivityTaskCanceled(
	ctx context.Context,
	cancelRequest *types.RespondActivityTaskCanceledRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if cancelRequest == nil {
		return validate.ErrRequestNotSet
	}

	if cancelRequest.TaskToken == nil {
		return validate.ErrTaskTokenNotSet
	}

	taskToken, err := wh.tokenSerializer.Deserialize(cancelRequest.TaskToken)
	if err != nil {
		return err
	}

	if taskToken.DomainID == "" {
		return validate.ErrDomainNotSet
	}

	domainName, err := wh.GetDomainCache().GetDomainName(taskToken.DomainID)
	if err != nil {
		return err
	}

	dw := domainWrapper{
		domain: domainName,
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendRespondActivityTaskCanceledScope, dw, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if !common.IsValidIDLength(
		cancelRequest.GetIdentity(),
		scope,
		wh.config.MaxIDLengthWarnLimit(),
		wh.config.IdentityMaxLength(domainName),
		metrics.CadenceErrIdentityExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeIdentity) {
		return validate.ErrIdentityTooLong
	}

	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	if err := common.CheckEventBlobSizeLimit(
		len(cancelRequest.Details),
		sizeLimitWarn,
		sizeLimitError,
		taskToken.DomainID,
		domainName,
		taskToken.WorkflowID,
		taskToken.RunID,
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("RespondActivityTaskCanceled"),
	); err != nil {
		// details exceeds blob size limit, we would record it as failure
		failRequest := &types.RespondActivityTaskFailedRequest{
			TaskToken: cancelRequest.TaskToken,
			Reason:    common.StringPtr(common.FailureReasonCancelDetailsExceedsLimit),
			Details:   cancelRequest.Details[0:sizeLimitError],
			Identity:  cancelRequest.Identity,
		}
		err = wh.GetHistoryClient().RespondActivityTaskFailed(ctx, &types.HistoryRespondActivityTaskFailedRequest{
			DomainUUID:    taskToken.DomainID,
			FailedRequest: failRequest,
		})
		if err != nil {
			return wh.normalizeVersionedErrors(ctx, err)
		}
	} else {
		err = wh.GetHistoryClient().RespondActivityTaskCanceled(ctx, &types.HistoryRespondActivityTaskCanceledRequest{
			DomainUUID:    taskToken.DomainID,
			CancelRequest: cancelRequest,
		})
		if err != nil {
			return wh.normalizeVersionedErrors(ctx, err)
		}
	}

	return nil
}

// RespondActivityTaskCanceledByID - called to cancel an activity task
func (wh *WorkflowHandler) RespondActivityTaskCanceledByID(
	ctx context.Context,
	cancelRequest *types.RespondActivityTaskCanceledByIDRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if cancelRequest == nil {
		return validate.ErrRequestNotSet
	}

	domainName := cancelRequest.GetDomain()
	if domainName == "" {
		return validate.ErrDomainNotSet
	}
	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return err
	}
	workflowID := cancelRequest.GetWorkflowID()
	runID := cancelRequest.GetRunID() // runID is optional so can be empty
	activityID := cancelRequest.GetActivityID()

	if domainID == "" {
		return validate.ErrDomainNotSet
	}
	if workflowID == "" {
		return validate.ErrWorkflowIDNotSet
	}
	if activityID == "" {
		return validate.ErrActivityIDNotSet
	}

	scope := getMetricsScopeWithDomain(metrics.FrontendRespondActivityTaskCanceledByIDScope, cancelRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if !common.IsValidIDLength(
		cancelRequest.GetIdentity(),
		scope,
		wh.config.MaxIDLengthWarnLimit(),
		wh.config.IdentityMaxLength(cancelRequest.GetDomain()),
		metrics.CadenceErrIdentityExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeIdentity) {
		return validate.ErrIdentityTooLong
	}

	taskToken := &common.TaskToken{
		DomainID:   domainID,
		RunID:      runID,
		WorkflowID: workflowID,
		ScheduleID: constants.EmptyEventID,
		ActivityID: activityID,
	}
	token, err := wh.tokenSerializer.Serialize(taskToken)
	if err != nil {
		return err
	}

	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	if err := common.CheckEventBlobSizeLimit(
		len(cancelRequest.Details),
		sizeLimitWarn,
		sizeLimitError,
		taskToken.DomainID,
		domainName,
		taskToken.WorkflowID,
		taskToken.RunID,
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("RespondActivityTaskCanceledByID"),
	); err != nil {
		// details exceeds blob size limit, we would record it as failure
		failRequest := &types.RespondActivityTaskFailedRequest{
			TaskToken: token,
			Reason:    common.StringPtr(common.FailureReasonCancelDetailsExceedsLimit),
			Details:   cancelRequest.Details[0:sizeLimitError],
			Identity:  cancelRequest.Identity,
		}
		err = wh.GetHistoryClient().RespondActivityTaskFailed(ctx, &types.HistoryRespondActivityTaskFailedRequest{
			DomainUUID:    taskToken.DomainID,
			FailedRequest: failRequest,
		})
		if err != nil {
			return wh.normalizeVersionedErrors(ctx, err)
		}
	} else {
		req := &types.RespondActivityTaskCanceledRequest{
			TaskToken: token,
			Details:   cancelRequest.Details,
			Identity:  cancelRequest.Identity,
		}

		err = wh.GetHistoryClient().RespondActivityTaskCanceled(ctx, &types.HistoryRespondActivityTaskCanceledRequest{
			DomainUUID:    taskToken.DomainID,
			CancelRequest: req,
		})
		if err != nil {
			return wh.normalizeVersionedErrors(ctx, err)
		}
	}

	return nil
}

// RespondDecisionTaskCompleted - response to a decision task
func (wh *WorkflowHandler) RespondDecisionTaskCompleted(
	ctx context.Context,
	completeRequest *types.RespondDecisionTaskCompletedRequest,
) (resp *types.RespondDecisionTaskCompletedResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if completeRequest == nil {
		return nil, validate.ErrRequestNotSet
	}

	if completeRequest.TaskToken == nil {
		return nil, validate.ErrTaskTokenNotSet
	}
	taskToken, err := wh.tokenSerializer.Deserialize(completeRequest.TaskToken)
	if err != nil {
		return nil, err
	}
	if taskToken.DomainID == "" {
		return nil, validate.ErrDomainNotSet
	}

	domainName, err := wh.GetDomainCache().GetDomainName(taskToken.DomainID)
	if err != nil {
		return nil, err
	}

	dw := domainWrapper{
		domain: domainName,
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendRespondDecisionTaskCompletedScope, dw, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if !common.IsValidIDLength(
		completeRequest.GetIdentity(),
		scope,
		wh.config.MaxIDLengthWarnLimit(),
		wh.config.IdentityMaxLength(domainName),
		metrics.CadenceErrIdentityExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeIdentity) {
		return nil, validate.ErrIdentityTooLong
	}

	if err := common.CheckDecisionResultLimit(
		len(completeRequest.Decisions),
		wh.config.DecisionResultCountLimit(domainName),
		scope); err != nil {
		return nil, err
	}

	histResp, err := wh.GetHistoryClient().RespondDecisionTaskCompleted(ctx, &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID:      taskToken.DomainID,
		CompleteRequest: completeRequest},
	)
	if err != nil {
		return nil, wh.normalizeVersionedErrors(ctx, err)
	}

	completedResp := &types.RespondDecisionTaskCompletedResponse{}
	completedResp.ActivitiesToDispatchLocally = histResp.ActivitiesToDispatchLocally
	if completeRequest.GetReturnNewDecisionTask() && histResp != nil && histResp.StartedResponse != nil {
		taskToken := &common.TaskToken{
			DomainID:        taskToken.DomainID,
			WorkflowID:      taskToken.WorkflowID,
			RunID:           taskToken.RunID,
			ScheduleID:      histResp.StartedResponse.GetScheduledEventID(),
			ScheduleAttempt: histResp.StartedResponse.GetAttempt(),
		}
		token, _ := wh.tokenSerializer.Serialize(taskToken)
		workflowExecution := &types.WorkflowExecution{
			WorkflowID: taskToken.WorkflowID,
			RunID:      taskToken.RunID,
		}
		matchingResp := common.CreateMatchingPollForDecisionTaskResponse(histResp.StartedResponse, workflowExecution, token)

		newDecisionTask, err := wh.createPollForDecisionTaskResponse(ctx, scope, taskToken.DomainID, matchingResp, matchingResp.GetBranchToken())
		if err != nil {
			return nil, err
		}
		completedResp.DecisionTask = newDecisionTask
	}

	return completedResp, nil
}

// RespondDecisionTaskFailed - failed response to a decision task
func (wh *WorkflowHandler) RespondDecisionTaskFailed(
	ctx context.Context,
	failedRequest *types.RespondDecisionTaskFailedRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if failedRequest == nil {
		return validate.ErrRequestNotSet
	}

	if failedRequest.TaskToken == nil {
		return validate.ErrTaskTokenNotSet
	}
	taskToken, err := wh.tokenSerializer.Deserialize(failedRequest.TaskToken)
	if err != nil {
		return err
	}
	if taskToken.DomainID == "" {
		return validate.ErrDomainNotSet
	}

	domainName, err := wh.GetDomainCache().GetDomainName(taskToken.DomainID)
	if err != nil {
		return err
	}

	dw := domainWrapper{
		domain: domainName,
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendRespondDecisionTaskFailedScope, dw, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if !common.IsValidIDLength(
		failedRequest.GetIdentity(),
		scope,
		wh.config.MaxIDLengthWarnLimit(),
		wh.config.IdentityMaxLength(domainName),
		metrics.CadenceErrIdentityExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeIdentity) {
		return validate.ErrIdentityTooLong
	}

	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	if err := common.CheckEventBlobSizeLimit(
		len(failedRequest.Details),
		sizeLimitWarn,
		sizeLimitError,
		taskToken.DomainID,
		domainName,
		taskToken.WorkflowID,
		taskToken.RunID,
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("RespondDecisionTaskFailed"),
	); err != nil {
		// details exceed, we would just truncate the size for decision task failed as the details is not used anywhere by client code
		failedRequest.Details = failedRequest.Details[0:sizeLimitError]
	}

	err = wh.GetHistoryClient().RespondDecisionTaskFailed(ctx, &types.HistoryRespondDecisionTaskFailedRequest{
		DomainUUID:    taskToken.DomainID,
		FailedRequest: failedRequest,
	})
	if err != nil {
		return wh.normalizeVersionedErrors(ctx, err)
	}
	return nil
}

// RespondQueryTaskCompleted - response to a query task
func (wh *WorkflowHandler) RespondQueryTaskCompleted(
	ctx context.Context,
	completeRequest *types.RespondQueryTaskCompletedRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if completeRequest == nil {
		return validate.ErrRequestNotSet
	}

	if completeRequest.TaskToken == nil {
		return validate.ErrTaskTokenNotSet
	}
	queryTaskToken, err := wh.tokenSerializer.DeserializeQueryTaskToken(completeRequest.TaskToken)
	if err != nil {
		return err
	}
	if queryTaskToken.DomainID == "" || queryTaskToken.TaskList == "" || queryTaskToken.TaskID == "" {
		return validate.ErrInvalidTaskToken
	}

	domainName, err := wh.GetDomainCache().GetDomainName(queryTaskToken.DomainID)
	if err != nil {
		return err
	}

	dw := domainWrapper{
		domain: domainName,
	}
	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	scope := getMetricsScopeWithDomain(metrics.FrontendRespondQueryTaskCompletedScope, dw, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if err := common.CheckEventBlobSizeLimit(
		len(completeRequest.GetQueryResult()),
		sizeLimitWarn,
		sizeLimitError,
		queryTaskToken.DomainID,
		domainName,
		queryTaskToken.WorkflowID,
		queryTaskToken.RunID,
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("RespondQueryTaskCompleted"),
	); err != nil {
		completeRequest = &types.RespondQueryTaskCompletedRequest{
			TaskToken:     completeRequest.TaskToken,
			CompletedType: types.QueryTaskCompletedTypeFailed.Ptr(),
			QueryResult:   nil,
			ErrorMessage:  err.Error(),
		}
	}

	call := yarpc.CallFromContext(ctx)

	completeRequest.WorkerVersionInfo = &types.WorkerVersionInfo{
		Impl:           call.Header(common.ClientImplHeaderName),
		FeatureVersion: call.Header(common.FeatureVersionHeaderName),
	}
	matchingRequest := &types.MatchingRespondQueryTaskCompletedRequest{
		DomainUUID:       queryTaskToken.DomainID,
		TaskList:         &types.TaskList{Name: queryTaskToken.TaskList},
		TaskID:           queryTaskToken.TaskID,
		CompletedRequest: completeRequest,
	}

	err = wh.GetMatchingClient().RespondQueryTaskCompleted(ctx, matchingRequest)
	if err != nil {
		return err
	}
	return nil
}

func (wh *WorkflowHandler) StartWorkflowExecutionAsync(
	ctx context.Context,
	startRequest *types.StartWorkflowExecutionAsyncRequest,
) (resp *types.StartWorkflowExecutionAsyncResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendStartWorkflowExecutionAsyncScope, startRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	// validate request before pushing to queue
	err := wh.validateStartWorkflowExecutionRequest(ctx, startRequest.StartWorkflowExecutionRequest, scope)
	if err != nil {
		return nil, err
	}

	producer, err := wh.producerManager.GetProducerByDomain(startRequest.GetDomain())
	if err != nil {
		return nil, err
	}

	// Serialize the message to be sent to the queue.
	// Avoid JSON because json encoding of requests excludes PII fields such as input. JSON encoded request are logged by acccess controlled api layer for audit purposes.
	payload, err := wh.thriftrwEncoder.Encode(thrift.FromStartWorkflowExecutionAsyncRequest(startRequest))
	if err != nil {
		return nil, fmt.Errorf("failed to encode StartWorkflowExecutionAsyncRequest: %v", err)
	}
	scope.RecordTimer(metrics.AsyncRequestPayloadSize, time.Duration(len(payload)))

	// propagate the headers from the context to the message
	header := &shared.Header{
		Fields: make(map[string][]byte),
	}
	for k, v := range yarpc.CallFromContext(ctx).OriginalHeaders() {
		header.Fields[k] = []byte(v)
	}
	messageType := sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest
	message := &sqlblobs.AsyncRequestMessage{
		PartitionKey: common.StringPtr(startRequest.GetWorkflowID()),
		Type:         &messageType,
		Header:       header,
		Encoding:     common.StringPtr(string(constants.EncodingTypeThriftRW)),
		Payload:      payload,
	}
	err = producer.Publish(ctx, message)
	if err != nil {
		return nil, err
	}
	return &types.StartWorkflowExecutionAsyncResponse{}, nil
}

// StartWorkflowExecution - Creates a new workflow execution
func (wh *WorkflowHandler) StartWorkflowExecution(
	ctx context.Context,
	startRequest *types.StartWorkflowExecutionRequest,
) (resp *types.StartWorkflowExecutionResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendStartWorkflowExecutionScope, startRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	err := wh.validateStartWorkflowExecutionRequest(ctx, startRequest, scope)
	if err != nil {
		return nil, err
	}
	domainName := startRequest.GetDomain()
	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}
	historyRequest, err := common.CreateHistoryStartWorkflowRequest(
		domainID, startRequest, time.Now(), wh.getPartitionConfig(ctx, domainName))
	if err != nil {
		return nil, err
	}

	// for debugging jitter workflow
	// will be removed later
	jitterStartSeconds := startRequest.GetJitterStartSeconds()
	if startRequest.GetDomain() == "cadence-canary" && jitterStartSeconds > 0 {
		wh.GetLogger().Debug("Start workflow execution request domainID",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(startRequest.WorkflowID),
			tag.Dynamic("JitterStartSeconds", jitterStartSeconds),
			tag.Dynamic("firstDecisionTaskBackoffSeconds", historyRequest.GetFirstDecisionTaskBackoffSeconds()),
		)
	} else {
		wh.GetLogger().Debug("Start workflow execution request domainID", tag.WorkflowDomainID(domainID))
	}

	resp, err = wh.GetHistoryClient().StartWorkflowExecution(ctx, historyRequest)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (wh *WorkflowHandler) validateStartWorkflowExecutionRequest(ctx context.Context, startRequest *types.StartWorkflowExecutionRequest, scope metrics.Scope) error {
	if startRequest == nil {
		return validate.ErrRequestNotSet
	}
	domainName := startRequest.GetDomain()
	if domainName == "" {
		return validate.ErrDomainNotSet
	}
	if startRequest.GetWorkflowID() == "" {
		return validate.ErrWorkflowIDNotSet
	}
	if _, err := uuid.Parse(startRequest.RequestID); err != nil {
		return &types.BadRequestError{Message: fmt.Sprintf("requestId %q is not a valid UUID", startRequest.RequestID)}
	}
	if startRequest.WorkflowType == nil || startRequest.WorkflowType.GetName() == "" {
		return validate.ErrWorkflowTypeNotSet
	}
	idLengthWarnLimit := wh.config.MaxIDLengthWarnLimit()
	if !common.IsValidIDLength(
		domainName,
		scope,
		idLengthWarnLimit,
		wh.config.DomainNameMaxLength(domainName),
		metrics.CadenceErrDomainNameExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeDomainName) {
		return validate.ErrDomainTooLong
	}
	if !common.IsValidIDLength(
		startRequest.GetWorkflowID(),
		scope,
		idLengthWarnLimit,
		wh.config.WorkflowIDMaxLength(domainName),
		metrics.CadenceErrWorkflowIDExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeWorkflowID) {
		return validate.ErrWorkflowIDTooLong
	}
	if err := common.ValidateRetryPolicy(startRequest.RetryPolicy); err != nil {
		return err
	}
	wh.GetLogger().Debug(
		"Received StartWorkflowExecution. WorkflowID",
		tag.WorkflowID(startRequest.GetWorkflowID()))
	if !common.IsValidIDLength(
		startRequest.WorkflowType.GetName(),
		scope,
		idLengthWarnLimit,
		wh.config.WorkflowTypeMaxLength(domainName),
		metrics.CadenceErrWorkflowTypeExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeWorkflowType) {
		return validate.ErrWorkflowTypeTooLong
	}
	if err := wh.validateTaskList(startRequest.TaskList, scope, domainName); err != nil {
		return err
	}
	if startRequest.GetExecutionStartToCloseTimeoutSeconds() <= 0 {
		return validate.ErrInvalidExecutionStartToCloseTimeoutSeconds
	}
	if startRequest.GetTaskStartToCloseTimeoutSeconds() <= 0 {
		return validate.ErrInvalidTaskStartToCloseTimeoutSeconds
	}
	if startRequest.GetDelayStartSeconds() < 0 {
		return validate.ErrInvalidDelayStartSeconds
	}
	if startRequest.GetJitterStartSeconds() < 0 {
		return validate.ErrInvalidJitterStartSeconds
	}
	jitter := startRequest.GetJitterStartSeconds()
	cron := startRequest.GetCronSchedule()
	if cron != "" {
		if _, err := backoff.ValidateSchedule(startRequest.GetCronSchedule()); err != nil {
			return err
		}
	}
	if jitter > 0 && cron != "" {
		// Calculate the cron duration and ensure that jitter is not greater than the cron duration,
		// because that would be confusing to users.

		// Request using start/end time zero value, which will get us an exact answer (i.e. its not in the
		// middle of a minute)
		backoffSeconds, err := backoff.GetBackoffForNextScheduleInSeconds(cron, time.Time{}, time.Time{}, jitter)
		if err != nil {
			return err
		}
		if jitter > backoffSeconds {
			return validate.ErrInvalidJitterStartSeconds2
		}
	}
	if !common.IsValidIDLength(
		startRequest.GetRequestID(),
		scope,
		idLengthWarnLimit,
		wh.config.RequestIDMaxLength(domainName),
		metrics.CadenceErrRequestIDExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeRequestID) {
		return validate.ErrRequestIDTooLong
	}
	if err := wh.searchAttributesValidator.ValidateSearchAttributes(startRequest.SearchAttributes, domainName); err != nil {
		return err
	}
	wh.GetLogger().Debug("Start workflow execution request domain", tag.WorkflowDomainName(domainName))
	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return err
	}
	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)
	actualSize := len(startRequest.Input)
	if startRequest.Memo != nil {
		actualSize += common.GetSizeOfMapStringToByteArray(startRequest.Memo.GetFields())
	}
	if err := common.CheckEventBlobSizeLimit(
		actualSize,
		sizeLimitWarn,
		sizeLimitError,
		domainID,
		domainName,
		startRequest.GetWorkflowID(),
		"",
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("StartWorkflowExecution"),
	); err != nil {
		return err
	}
	isolationGroup := wh.getIsolationGroup(ctx, domainName)
	if !wh.isIsolationGroupHealthy(ctx, domainName, isolationGroup) {
		return &types.BadRequestError{fmt.Sprintf("Domain %s is drained from isolation group %s.", domainName, isolationGroup)}
	}
	return nil
}

// GetWorkflowExecutionHistory - retrieves the history of workflow execution
func (wh *WorkflowHandler) GetWorkflowExecutionHistory(
	ctx context.Context,
	getRequest *types.GetWorkflowExecutionHistoryRequest,
) (resp *types.GetWorkflowExecutionHistoryResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if getRequest == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := getRequest.GetDomain()
	wfExecution := getRequest.GetExecution()

	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}

	if err := validate.CheckExecution(wfExecution); err != nil {
		return nil, err
	}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}

	if getRequest.GetMaximumPageSize() <= 0 {
		getRequest.MaximumPageSize = int32(wh.config.HistoryMaxPageSize(getRequest.GetDomain()))
	}
	// force limit page size if exceed
	if getRequest.GetMaximumPageSize() > constants.GetHistoryMaxPageSize {
		wh.GetThrottledLogger().Warn("GetHistory page size is larger than threshold",
			tag.WorkflowID(getRequest.Execution.GetWorkflowID()),
			tag.WorkflowRunID(getRequest.Execution.GetRunID()),
			tag.WorkflowDomainID(domainID),
			tag.WorkflowSize(int64(getRequest.GetMaximumPageSize())))
		getRequest.MaximumPageSize = constants.GetHistoryMaxPageSize
	}

	scope := getMetricsScopeWithDomain(metrics.FrontendGetWorkflowExecutionHistoryScope, getRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if !getRequest.GetSkipArchival() {
		enableArchivalRead := wh.GetArchivalMetadata().GetHistoryConfig().ReadEnabled()
		historyArchived := wh.historyArchived(ctx, getRequest, domainID)
		if enableArchivalRead && historyArchived {
			return wh.getArchivedHistory(ctx, getRequest, domainID)
		}
	}

	// this function return the following 6 things,
	// 1. branch token
	// 2. the workflow run ID
	// 3. the last first event ID (the event ID of the last batch of events in the history)
	// 4. the next event ID
	// 5. whether the workflow is closed
	// 6 The version history
	// 7. error if any
	queryHistory := func(
		domainUUID string,
		execution *types.WorkflowExecution,
		expectedNextEventID int64,
		currentBranchToken []byte,
		versionHistoryItem *persistence.VersionHistoryItem,
	) ([]byte, string, int64, int64, bool, *types.VersionHistoryItem, error) {

		response, err := wh.GetHistoryClient().PollMutableState(ctx, &types.PollMutableStateRequest{
			DomainUUID:          domainUUID,
			Execution:           execution,
			ExpectedNextEventID: expectedNextEventID,
			CurrentBranchToken:  currentBranchToken,
			VersionHistoryItem:  versionHistoryItem.ToInternalType(),
		})

		if err != nil {
			return nil, "", 0, 0, false, nil, err
		}

		isWorkflowRunning := response.GetWorkflowCloseState() == persistence.WorkflowCloseStatusNone
		currentVersionHistory, err := persistence.NewVersionHistoriesFromInternalType(response.VersionHistories).GetCurrentVersionHistory()
		if err != nil {
			wh.GetLogger().Error("Failed to get current version history", tag.Dynamic("version-histories", response.VersionHistories))
			return nil, "", 0, 0, false, nil, fmt.Errorf("failed to get the current version from the response from history: %w", err)
		}

		lastVersionHistoryItem, err := currentVersionHistory.GetLastItem()
		if err != nil {
			return nil, "", 0, 0, false, nil, err
		}

		return response.CurrentBranchToken,
			response.Execution.GetRunID(),
			response.GetLastFirstEventID(),
			response.GetNextEventID(),
			isWorkflowRunning,
			lastVersionHistoryItem.ToInternalType(),
			nil
	}

	isLongPoll := getRequest.GetWaitForNewEvent()
	isCloseEventOnly := getRequest.GetHistoryEventFilterType() == types.HistoryEventFilterTypeCloseEvent
	execution := getRequest.Execution
	token := &getHistoryContinuationToken{}

	var runID string
	lastFirstEventID := constants.FirstEventID
	var nextEventID int64
	var isWorkflowRunning bool

	// process the token for paging
	queryNextEventID := constants.EndEventID
	if getRequest.NextPageToken != nil {
		token, err = deserializeHistoryToken(getRequest.NextPageToken)
		if err != nil {
			return nil, validate.ErrInvalidNextPageToken
		}
		if execution.RunID != "" && execution.GetRunID() != token.RunID {
			return nil, validate.ErrNextPageTokenRunIDMismatch
		}

		execution.RunID = token.RunID

		// we need to update the current next event ID and whether workflow is running
		if len(token.PersistenceToken) == 0 && isLongPoll && token.IsWorkflowRunning {
			logger := wh.GetLogger().WithTags(
				tag.WorkflowDomainName(getRequest.GetDomain()),
				tag.WorkflowID(getRequest.Execution.GetWorkflowID()),
				tag.WorkflowRunID(getRequest.Execution.GetRunID()),
			)
			// TODO: for now we only log the invalid timeout (this is done inside the helper function) in case
			// this change breaks existing customers. Once we are sure no one is calling this API with very short timeout
			// we can return the error.
			_ = common.ValidateLongPollContextTimeout(ctx, "GetWorkflowExecutionHistory", logger)

			if !isCloseEventOnly {
				queryNextEventID = token.NextEventID
			}

			vh := persistence.NewVersionHistoryItemFromInternalType(token.VersionHistoryItem)
			token.BranchToken, _, lastFirstEventID, nextEventID, isWorkflowRunning, token.VersionHistoryItem, err =
				queryHistory(domainID, execution, queryNextEventID, token.BranchToken, vh)
			if err != nil {
				return nil, err
			}
			token.FirstEventID = token.NextEventID
			token.NextEventID = nextEventID
			token.IsWorkflowRunning = isWorkflowRunning
		}
	} else {
		if !isCloseEventOnly {
			queryNextEventID = constants.FirstEventID
		}
		token.BranchToken, runID, lastFirstEventID, nextEventID, isWorkflowRunning, token.VersionHistoryItem, err =
			queryHistory(domainID, execution, queryNextEventID, nil, nil)
		if err != nil {
			return nil, err
		}

		execution.RunID = runID

		token.RunID = runID
		token.FirstEventID = constants.FirstEventID
		token.NextEventID = nextEventID
		token.IsWorkflowRunning = isWorkflowRunning
		token.PersistenceToken = nil
	}

	call := yarpc.CallFromContext(ctx)
	clientFeatureVersion := call.Header(common.FeatureVersionHeaderName)
	clientImpl := call.Header(common.ClientImplHeaderName)
	supportsRawHistoryQuery := wh.versionChecker.SupportsRawHistoryQuery(clientImpl, clientFeatureVersion) == nil
	isRawHistoryEnabled := wh.config.SendRawWorkflowHistory(domainName) && supportsRawHistoryQuery

	history := &types.History{}
	history.Events = []*types.HistoryEvent{}
	var historyBlob []*types.DataBlob

	// helper function to just getHistory
	getHistory := func(firstEventID, nextEventID int64, nextPageToken []byte) error {
		if isRawHistoryEnabled {
			historyBlob, token.PersistenceToken, err = wh.getRawHistory(
				ctx,
				scope,
				domainID,
				domainName,
				*execution,
				firstEventID,
				nextEventID,
				getRequest.GetMaximumPageSize(),
				nextPageToken,
				token.TransientDecision,
				token.BranchToken,
			)
		} else {
			history, token.PersistenceToken, err = wh.getHistory(
				ctx,
				scope,
				domainID,
				domainName,
				*execution,
				firstEventID,
				nextEventID,
				getRequest.GetMaximumPageSize(),
				nextPageToken,
				token.TransientDecision,
				token.BranchToken,
			)
		}
		if err != nil {
			return err
		}
		return nil
	}

	if isCloseEventOnly {
		if !isWorkflowRunning {
			if err := getHistory(lastFirstEventID, nextEventID, nil); err != nil {
				return nil, err
			}
			if isRawHistoryEnabled {
				// since getHistory func will not return empty history, so the below is safe
				historyBlob = historyBlob[len(historyBlob)-1:]
			} else {
				// since getHistory func will not return empty history, so the below is safe
				history.Events = history.Events[len(history.Events)-1:]
			}
			token = nil
		} else if isLongPoll {
			// set the persistence token to be nil so next time we will query history for updates
			token.PersistenceToken = nil
		} else {
			token = nil
		}
	} else {
		// return all events
		if token.FirstEventID >= token.NextEventID {
			// currently there is no new event
			history.Events = []*types.HistoryEvent{}
			if !isWorkflowRunning {
				token = nil
			}
		} else {
			if err := getHistory(token.FirstEventID, token.NextEventID, token.PersistenceToken); err != nil {
				return nil, err
			}
			// here, for long pull on history events, we need to intercept the paging token from cassandra
			// and do something clever
			if len(token.PersistenceToken) == 0 && (!token.IsWorkflowRunning || !isLongPoll) {
				// meaning, there is no more history to be returned
				token = nil
			}
		}
	}

	nextToken, err := serializeHistoryToken(token)
	if err != nil {
		return nil, err
	}
	return &types.GetWorkflowExecutionHistoryResponse{
		History:       history,
		RawHistory:    historyBlob,
		NextPageToken: nextToken,
		Archived:      false,
	}, nil
}

// SignalWorkflowExecution is used to send a signal event to running workflow execution.  This results in
// WorkflowExecutionSignaled event recorded in the history and a decision task being created for the execution.
func (wh *WorkflowHandler) SignalWorkflowExecution(
	ctx context.Context,
	signalRequest *types.SignalWorkflowExecutionRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if signalRequest == nil {
		return validate.ErrRequestNotSet
	}

	domainName := signalRequest.GetDomain()
	wfExecution := signalRequest.GetWorkflowExecution()

	if domainName == "" {
		return validate.ErrDomainNotSet
	}
	if err := validate.CheckExecution(wfExecution); err != nil {
		return err
	}

	scope := getMetricsScopeWithDomain(metrics.FrontendSignalWorkflowExecutionScope, signalRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	idLengthWarnLimit := wh.config.MaxIDLengthWarnLimit()
	if !common.IsValidIDLength(
		domainName,
		scope,
		idLengthWarnLimit,
		wh.config.DomainNameMaxLength(domainName),
		metrics.CadenceErrDomainNameExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeDomainName) {
		return validate.ErrDomainTooLong
	}

	if signalRequest.GetSignalName() == "" {
		return validate.ErrSignalNameNotSet
	}

	if !common.IsValidIDLength(
		signalRequest.GetSignalName(),
		scope,
		idLengthWarnLimit,
		wh.config.SignalNameMaxLength(domainName),
		metrics.CadenceErrSignalNameExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeSignalName) {
		return validate.ErrSignalNameTooLong
	}

	if !common.IsValidIDLength(
		signalRequest.GetRequestID(),
		scope,
		idLengthWarnLimit,
		wh.config.RequestIDMaxLength(domainName),
		metrics.CadenceErrRequestIDExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeRequestID) {
		return validate.ErrRequestIDTooLong
	}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return err
	}

	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)
	if err := common.CheckEventBlobSizeLimit(
		len(signalRequest.Input),
		sizeLimitWarn,
		sizeLimitError,
		domainID,
		domainName,
		signalRequest.GetWorkflowExecution().GetWorkflowID(),
		signalRequest.GetWorkflowExecution().GetRunID(),
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("SignalWorkflowExecution"),
	); err != nil {
		return err
	}

	isolationGroup := wh.getIsolationGroup(ctx, domainName)
	if !wh.isIsolationGroupHealthy(ctx, domainName, isolationGroup) {
		return &types.BadRequestError{fmt.Sprintf("Domain %s is drained from isolation group %s.", domainName, isolationGroup)}
	}

	err = wh.GetHistoryClient().SignalWorkflowExecution(ctx, &types.HistorySignalWorkflowExecutionRequest{
		DomainUUID:    domainID,
		SignalRequest: signalRequest,
	})
	if err != nil {
		return wh.normalizeVersionedErrors(ctx, err)
	}

	return nil
}

func (wh *WorkflowHandler) SignalWithStartWorkflowExecutionAsync(
	ctx context.Context,
	signalWithStartRequest *types.SignalWithStartWorkflowExecutionAsyncRequest,
) (resp *types.SignalWithStartWorkflowExecutionAsyncResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendSignalWithStartWorkflowExecutionAsyncScope, signalWithStartRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	// validate request before pushing to queue
	err := wh.validateSignalWithStartWorkflowExecutionRequest(ctx, signalWithStartRequest.SignalWithStartWorkflowExecutionRequest, scope)
	if err != nil {
		return nil, err
	}
	producer, err := wh.producerManager.GetProducerByDomain(signalWithStartRequest.GetDomain())
	if err != nil {
		return nil, err
	}

	// Serialize the message to be sent to the queue.
	// Avoid JSON because json encoding of requests excludes PII fields such as input. JSON encoded request are logged by acccess controlled api layer for audit purposes.
	payload, err := wh.thriftrwEncoder.Encode(thrift.FromSignalWithStartWorkflowExecutionAsyncRequest(signalWithStartRequest))
	if err != nil {
		return nil, fmt.Errorf("failed to encode SignalWithStartWorkflowExecutionAsyncRequest: %v", err)
	}
	scope.RecordTimer(metrics.AsyncRequestPayloadSize, time.Duration(len(payload)))

	// propagate the headers from the context to the message
	header := &shared.Header{
		Fields: map[string][]byte{},
	}
	for k, v := range yarpc.CallFromContext(ctx).OriginalHeaders() {
		header.Fields[k] = []byte(v)
	}
	messageType := sqlblobs.AsyncRequestTypeSignalWithStartWorkflowExecutionAsyncRequest
	message := &sqlblobs.AsyncRequestMessage{
		PartitionKey: common.StringPtr(signalWithStartRequest.GetWorkflowID()),
		Type:         &messageType,
		Header:       header,
		Encoding:     common.StringPtr(string(constants.EncodingTypeThriftRW)),
		Payload:      payload,
	}
	err = producer.Publish(ctx, message)
	if err != nil {
		return nil, err
	}
	return &types.SignalWithStartWorkflowExecutionAsyncResponse{}, nil
}

// SignalWithStartWorkflowExecution is used to ensure sending a signal event to a workflow execution.
// If workflow is running, this results in WorkflowExecutionSignaled event recorded in the history
// and a decision task being created for the execution.
// If workflow is not running or not found, this results in WorkflowExecutionStarted and WorkflowExecutionSignaled
// event recorded in history, and a decision task being created for the execution
func (wh *WorkflowHandler) SignalWithStartWorkflowExecution(
	ctx context.Context,
	signalWithStartRequest *types.SignalWithStartWorkflowExecutionRequest,
) (resp *types.StartWorkflowExecutionResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	scope := getMetricsScopeWithDomain(metrics.FrontendSignalWithStartWorkflowExecutionScope, signalWithStartRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	err := wh.validateSignalWithStartWorkflowExecutionRequest(ctx, signalWithStartRequest, scope)
	if err != nil {
		return nil, err
	}

	domainName := signalWithStartRequest.GetDomain()
	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}
	resp, err = wh.GetHistoryClient().SignalWithStartWorkflowExecution(ctx, &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID:             domainID,
		SignalWithStartRequest: signalWithStartRequest,
		PartitionConfig:        wh.getPartitionConfig(ctx, domainName),
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (wh *WorkflowHandler) validateSignalWithStartWorkflowExecutionRequest(ctx context.Context, signalWithStartRequest *types.SignalWithStartWorkflowExecutionRequest, scope metrics.Scope) error {
	if signalWithStartRequest == nil {
		return validate.ErrRequestNotSet
	}

	domainName := signalWithStartRequest.GetDomain()
	if domainName == "" {
		return validate.ErrDomainNotSet
	}
	if signalWithStartRequest.GetWorkflowID() == "" {
		return validate.ErrWorkflowIDNotSet
	}

	idLengthWarnLimit := wh.config.MaxIDLengthWarnLimit()
	if !common.IsValidIDLength(
		domainName,
		scope,
		idLengthWarnLimit,
		wh.config.DomainNameMaxLength(domainName),
		metrics.CadenceErrDomainNameExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeDomainName) {
		return validate.ErrDomainTooLong
	}

	if !common.IsValidIDLength(
		signalWithStartRequest.GetWorkflowID(),
		scope,
		idLengthWarnLimit,
		wh.config.WorkflowIDMaxLength(domainName),
		metrics.CadenceErrWorkflowIDExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeWorkflowID) {
		return validate.ErrWorkflowIDTooLong
	}

	if signalWithStartRequest.GetSignalName() == "" {
		return validate.ErrSignalNameNotSet
	}

	if !common.IsValidIDLength(
		signalWithStartRequest.GetSignalName(),
		scope,
		idLengthWarnLimit,
		wh.config.SignalNameMaxLength(domainName),
		metrics.CadenceErrSignalNameExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeSignalName) {
		return validate.ErrSignalNameTooLong
	}

	if signalWithStartRequest.WorkflowType == nil || signalWithStartRequest.WorkflowType.GetName() == "" {
		return validate.ErrWorkflowTypeNotSet
	}

	if !common.IsValidIDLength(
		signalWithStartRequest.WorkflowType.GetName(),
		scope,
		idLengthWarnLimit,
		wh.config.WorkflowTypeMaxLength(domainName),
		metrics.CadenceErrWorkflowTypeExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeWorkflowType) {
		return validate.ErrWorkflowTypeTooLong
	}

	if err := wh.validateTaskList(signalWithStartRequest.TaskList, scope, domainName); err != nil {
		return err
	}

	if !common.IsValidIDLength(
		signalWithStartRequest.GetRequestID(),
		scope,
		idLengthWarnLimit,
		wh.config.RequestIDMaxLength(domainName),
		metrics.CadenceErrRequestIDExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeRequestID) {
		return validate.ErrRequestIDTooLong
	}

	if signalWithStartRequest.GetExecutionStartToCloseTimeoutSeconds() <= 0 {
		return validate.ErrInvalidExecutionStartToCloseTimeoutSeconds
	}

	if signalWithStartRequest.GetTaskStartToCloseTimeoutSeconds() <= 0 {
		return validate.ErrInvalidTaskStartToCloseTimeoutSeconds
	}

	if err := common.ValidateRetryPolicy(signalWithStartRequest.RetryPolicy); err != nil {
		return err
	}

	if signalWithStartRequest.GetCronSchedule() != "" {
		if _, err := backoff.ValidateSchedule(signalWithStartRequest.GetCronSchedule()); err != nil {
			return err
		}
	}

	if err := wh.searchAttributesValidator.ValidateSearchAttributes(signalWithStartRequest.SearchAttributes, domainName); err != nil {
		return err
	}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return err
	}

	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)
	if err := common.CheckEventBlobSizeLimit(
		len(signalWithStartRequest.SignalInput),
		sizeLimitWarn,
		sizeLimitError,
		domainID,
		domainName,
		signalWithStartRequest.GetWorkflowID(),
		"",
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("SignalWithStartWorkflowExecution"),
	); err != nil {
		return err
	}
	actualSize := len(signalWithStartRequest.Input) + common.GetSizeOfMapStringToByteArray(signalWithStartRequest.Memo.GetFields())
	if err := common.CheckEventBlobSizeLimit(
		actualSize,
		sizeLimitWarn,
		sizeLimitError,
		domainID,
		domainName,
		signalWithStartRequest.GetWorkflowID(),
		"",
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("SignalWithStartWorkflowExecution"),
	); err != nil {
		return err
	}

	isolationGroup := wh.getIsolationGroup(ctx, domainName)
	if !wh.isIsolationGroupHealthy(ctx, domainName, isolationGroup) {
		return &types.BadRequestError{fmt.Sprintf("Domain %s is drained from isolation group %s.", domainName, isolationGroup)}
	}
	return nil
}

// TerminateWorkflowExecution terminates an existing workflow execution by recording WorkflowExecutionTerminated event
// in the history and immediately terminating the execution instance.
func (wh *WorkflowHandler) TerminateWorkflowExecution(
	ctx context.Context,
	terminateRequest *types.TerminateWorkflowExecutionRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if terminateRequest == nil {
		return validate.ErrRequestNotSet
	}

	domainName := terminateRequest.GetDomain()
	wfExecution := terminateRequest.GetWorkflowExecution()
	if terminateRequest.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	if err := validate.CheckExecution(wfExecution); err != nil {
		return err
	}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return err
	}

	err = wh.GetHistoryClient().TerminateWorkflowExecution(ctx, &types.HistoryTerminateWorkflowExecutionRequest{
		DomainUUID:       domainID,
		TerminateRequest: terminateRequest,
	})
	if err != nil {
		return wh.normalizeVersionedErrors(ctx, err)
	}

	return nil
}

// ResetWorkflowExecution reset an existing workflow execution to the nextFirstEventID
// in the history and immediately terminating the current execution instance.
func (wh *WorkflowHandler) ResetWorkflowExecution(
	ctx context.Context,
	resetRequest *types.ResetWorkflowExecutionRequest,
) (resp *types.ResetWorkflowExecutionResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if resetRequest == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := resetRequest.GetDomain()
	wfExecution := resetRequest.GetWorkflowExecution()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	if err := validate.CheckExecution(wfExecution); err != nil {
		return nil, err
	}

	domainID, err := wh.GetDomainCache().GetDomainID(resetRequest.GetDomain())
	if err != nil {
		return nil, err
	}

	resp, err = wh.GetHistoryClient().ResetWorkflowExecution(ctx, &types.HistoryResetWorkflowExecutionRequest{
		DomainUUID:   domainID,
		ResetRequest: resetRequest,
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// RequestCancelWorkflowExecution - requests to cancel a workflow execution
func (wh *WorkflowHandler) RequestCancelWorkflowExecution(
	ctx context.Context,
	cancelRequest *types.RequestCancelWorkflowExecutionRequest,
) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}

	if cancelRequest == nil {
		return validate.ErrRequestNotSet
	}

	domainName := cancelRequest.GetDomain()
	wfExecution := cancelRequest.GetWorkflowExecution()
	if domainName == "" {
		return validate.ErrDomainNotSet
	}
	if err := validate.CheckExecution(wfExecution); err != nil {
		return err
	}

	domainID, err := wh.GetDomainCache().GetDomainID(cancelRequest.GetDomain())
	if err != nil {
		return err
	}

	err = wh.GetHistoryClient().RequestCancelWorkflowExecution(ctx, &types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID:    domainID,
		CancelRequest: cancelRequest,
	})
	if err != nil {
		return wh.normalizeVersionedErrors(ctx, err)
	}

	return nil
}

// RestartWorkflowExecution - retrieves info for an existing workflow then restarts it
func (wh *WorkflowHandler) RestartWorkflowExecution(ctx context.Context, request *types.RestartWorkflowExecutionRequest) (resp *types.RestartWorkflowExecutionResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	wfExecution := request.GetWorkflowExecution()

	if request.GetDomain() == "" {
		return nil, validate.ErrDomainNotSet
	}

	if err := validate.CheckExecution(wfExecution); err != nil {
		return nil, err
	}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}

	isolationGroup := wh.getIsolationGroup(ctx, domainName)
	if !wh.isIsolationGroupHealthy(ctx, domainName, isolationGroup) {
		return nil, &types.BadRequestError{fmt.Sprintf("Domain %s is drained from isolation group %s.", domainName, isolationGroup)}
	}

	history, err := wh.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain: domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: wfExecution.WorkflowID,
			RunID:      wfExecution.RunID,
		},
		SkipArchival: true,
	})
	if err != nil {
		return nil, validate.ErrHistoryNotFound
	}
	startRequest := constructRestartWorkflowRequest(history.History.Events[0].WorkflowExecutionStartedEventAttributes,
		domainName, request.Identity, wfExecution.WorkflowID)
	req, err := common.CreateHistoryStartWorkflowRequest(domainID, startRequest, time.Now(), wh.getPartitionConfig(ctx, domainName))
	if err != nil {
		return nil, err
	}
	startResp, err := wh.GetHistoryClient().StartWorkflowExecution(ctx, req)
	if err != nil {
		return nil, wh.normalizeVersionedErrors(ctx, err)
	}
	resp = &types.RestartWorkflowExecutionResponse{
		RunID: startResp.RunID,
	}

	return resp, nil
}

// GetSearchAttributes return valid indexed keys
func (wh *WorkflowHandler) GetSearchAttributes(ctx context.Context) (resp *types.GetSearchAttributesResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	keys := wh.config.ValidSearchAttributes()
	resp = &types.GetSearchAttributesResponse{
		Keys: wh.convertIndexedKeyToThrift(keys),
	}
	return resp, nil
}

// QueryWorkflow returns query result for a specified workflow execution
func (wh *WorkflowHandler) QueryWorkflow(
	ctx context.Context,
	queryRequest *types.QueryWorkflowRequest,
) (resp *types.QueryWorkflowResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if queryRequest == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := queryRequest.GetDomain()
	wfExecution := queryRequest.GetExecution()

	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}

	if err := validate.CheckExecution(wfExecution); err != nil {
		return nil, err
	}

	if wh.config.DisallowQuery(domainName) {
		return nil, validate.ErrQueryDisallowedForDomain
	}

	if queryRequest.Query == nil {
		return nil, validate.ErrQueryNotSet
	}

	if queryRequest.Query.GetQueryType() == "" {
		return nil, validate.ErrQueryTypeNotSet
	}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}

	sizeLimitError := wh.config.BlobSizeLimitError(domainName)
	sizeLimitWarn := wh.config.BlobSizeLimitWarn(domainName)

	scope := getMetricsScopeWithDomain(metrics.FrontendQueryWorkflowScope, queryRequest, wh.GetMetricsClient()).Tagged(metrics.GetContextTags(ctx)...)
	if err := common.CheckEventBlobSizeLimit(
		len(queryRequest.GetQuery().GetQueryArgs()),
		sizeLimitWarn,
		sizeLimitError,
		domainID,
		domainName,
		queryRequest.GetExecution().GetWorkflowID(),
		queryRequest.GetExecution().GetRunID(),
		scope,
		wh.GetLogger(),
		tag.BlobSizeViolationOperation("QueryWorkflow")); err != nil {
		return nil, err
	}

	req := &types.HistoryQueryWorkflowRequest{
		DomainUUID: domainID,
		Request:    queryRequest,
	}
	hResponse, err := wh.GetHistoryClient().QueryWorkflow(ctx, req)
	if err != nil {
		return nil, err
	}
	return hResponse.GetResponse(), nil
}

// DescribeWorkflowExecution returns information about the specified workflow execution.
func (wh *WorkflowHandler) DescribeWorkflowExecution(
	ctx context.Context,
	request *types.DescribeWorkflowExecutionRequest,
) (resp *types.DescribeWorkflowExecutionResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	wfExecution := request.GetExecution()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}

	if err := validate.CheckExecution(wfExecution); err != nil {
		return nil, err
	}

	domainID, err := wh.GetDomainCache().GetDomainID(request.GetDomain())
	if err != nil {
		return nil, err
	}

	response, err := wh.GetHistoryClient().DescribeWorkflowExecution(ctx, &types.HistoryDescribeWorkflowExecutionRequest{
		DomainUUID: domainID,
		Request:    request,
	})

	wh.emitDescribeWorkflowExecutionMetrics(domainName, response, err)

	if err != nil {
		return nil, err
	}

	return response, nil
}

func (wh *WorkflowHandler) getRawHistory(
	ctx context.Context,
	scope metrics.Scope,
	domainID string,
	domainName string,
	execution types.WorkflowExecution,
	firstEventID int64,
	nextEventID int64,
	pageSize int32,
	nextPageToken []byte,
	transientDecision *types.TransientDecisionInfo,
	branchToken []byte,
) ([]*types.DataBlob, []byte, error) {
	rawHistory := []*types.DataBlob{}
	shardID := common.WorkflowIDToHistoryShard(execution.WorkflowID, wh.config.NumHistoryShards)

	resp, err := wh.GetHistoryManager().ReadRawHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{
		BranchToken:   branchToken,
		MinEventID:    firstEventID,
		MaxEventID:    nextEventID,
		PageSize:      int(pageSize),
		NextPageToken: nextPageToken,
		ShardID:       common.IntPtr(shardID),
		DomainName:    domainName,
	})
	if err != nil {
		return nil, nil, err
	}

	var encoding *types.EncodingType
	for _, data := range resp.HistoryEventBlobs {
		switch data.Encoding {
		case constants.EncodingTypeJSON:
			encoding = types.EncodingTypeJSON.Ptr()
		case constants.EncodingTypeThriftRW:
			encoding = types.EncodingTypeThriftRW.Ptr()
		default:
			panic(fmt.Sprintf("Invalid encoding type for raw history, encoding type: %s", data.Encoding))
		}
		rawHistory = append(rawHistory, &types.DataBlob{
			EncodingType: encoding,
			Data:         data.Data,
		})
	}

	if len(resp.NextPageToken) == 0 && transientDecision != nil {
		if err := wh.validateTransientDecisionEvents(nextEventID, transientDecision); err != nil {
			scope.IncCounter(metrics.CadenceErrIncompleteHistoryCounter)
			wh.GetLogger().Error("getHistory error",
				tag.WorkflowDomainID(domainID),
				tag.WorkflowID(execution.GetWorkflowID()),
				tag.WorkflowRunID(execution.GetRunID()),
				tag.Error(err))
		}
		blob, err := wh.GetPayloadSerializer().SerializeBatchEvents(
			[]*types.HistoryEvent{transientDecision.ScheduledEvent, transientDecision.StartedEvent}, constants.EncodingTypeThriftRW)
		if err != nil {
			return nil, nil, err
		}
		rawHistory = append(rawHistory, &types.DataBlob{
			EncodingType: types.EncodingTypeThriftRW.Ptr(),
			Data:         blob.Data,
		})
	}

	return rawHistory, resp.NextPageToken, nil
}

func (wh *WorkflowHandler) getHistory(
	ctx context.Context,
	scope metrics.Scope,
	domainID string,
	domainName string,
	execution types.WorkflowExecution,
	firstEventID, nextEventID int64,
	pageSize int32,
	nextPageToken []byte,
	transientDecision *types.TransientDecisionInfo,
	branchToken []byte,
) (*types.History, []byte, error) {

	var size int

	isFirstPage := len(nextPageToken) == 0
	shardID := common.WorkflowIDToHistoryShard(execution.WorkflowID, wh.config.NumHistoryShards)
	var err error
	historyEvents, size, nextPageToken, err := persistenceutils.ReadFullPageV2Events(ctx, wh.GetHistoryManager(), &persistence.ReadHistoryBranchRequest{
		BranchToken:   branchToken,
		MinEventID:    firstEventID,
		MaxEventID:    nextEventID,
		PageSize:      int(pageSize),
		NextPageToken: nextPageToken,
		ShardID:       common.IntPtr(shardID),
		DomainName:    domainName,
	})

	if err != nil {
		return nil, nil, err
	}

	scope.RecordTimer(metrics.HistorySize, time.Duration(size))

	isLastPage := len(nextPageToken) == 0
	if err := verifyHistoryIsComplete(
		historyEvents,
		firstEventID,
		nextEventID-1,
		isFirstPage,
		isLastPage,
		int(pageSize)); err != nil {
		scope.IncCounter(metrics.CadenceErrIncompleteHistoryCounter)
		wh.GetLogger().Error("getHistory: incomplete history",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(execution.GetWorkflowID()),
			tag.WorkflowRunID(execution.GetRunID()),
			tag.Error(err))
		return nil, nil, err
	}

	if len(nextPageToken) == 0 && transientDecision != nil {
		if err := wh.validateTransientDecisionEvents(nextEventID, transientDecision); err != nil {
			scope.IncCounter(metrics.CadenceErrIncompleteHistoryCounter)
			wh.GetLogger().Error("getHistory error",
				tag.WorkflowDomainID(domainID),
				tag.WorkflowID(execution.GetWorkflowID()),
				tag.WorkflowRunID(execution.GetRunID()),
				tag.Error(err))
		}
		// Append the transient decision events once we are done enumerating everything from the events table
		historyEvents = append(historyEvents, transientDecision.ScheduledEvent, transientDecision.StartedEvent)
	}

	executionHistory := &types.History{}
	executionHistory.Events = historyEvents
	return executionHistory, nextPageToken, nil
}

func (wh *WorkflowHandler) validateTransientDecisionEvents(
	expectedNextEventID int64,
	decision *types.TransientDecisionInfo,
) error {

	if decision.ScheduledEvent.ID == expectedNextEventID &&
		decision.StartedEvent.ID == expectedNextEventID+1 {
		return nil
	}

	return fmt.Errorf(
		"invalid transient decision: "+
			"expectedScheduledEventID=%v expectedStartedEventID=%v but have scheduledEventID=%v startedEventID=%v",
		expectedNextEventID,
		expectedNextEventID+1,
		decision.ScheduledEvent.ID,
		decision.StartedEvent.ID)
}

func (wh *WorkflowHandler) validateTaskList(t *types.TaskList, scope metrics.Scope, domain string) error {
	if t == nil || t.GetName() == "" {
		return validate.ErrTaskListNotSet
	}

	if !common.IsValidIDLength(
		t.GetName(),
		scope,
		wh.config.MaxIDLengthWarnLimit(),
		wh.config.TaskListNameMaxLength(domain),
		metrics.CadenceErrTaskListNameExceededWarnLimit,
		domain,
		wh.GetLogger(),
		tag.IDTypeTaskListName) {
		return validate.ErrTaskListTooLong
	}
	return nil
}

func (wh *WorkflowHandler) createPollForDecisionTaskResponse(
	ctx context.Context,
	scope metrics.Scope,
	domainID string,
	matchingResp *types.MatchingPollForDecisionTaskResponse,
	branchToken []byte,
) (*types.PollForDecisionTaskResponse, error) {

	if matchingResp.WorkflowExecution == nil {
		// this will happen if there is no decision task to be send to worker / caller
		return &types.PollForDecisionTaskResponse{
			AutoConfigHint: matchingResp.AutoConfigHint,
		}, nil
	}

	var history *types.History
	var continuation []byte
	var err error

	if matchingResp.GetStickyExecutionEnabled() && matchingResp.Query != nil {
		// meaning sticky query, we should not return any events to worker
		// since query task only check the current status
		history = &types.History{
			Events: []*types.HistoryEvent{},
		}
	} else {
		// here we have 3 cases:
		// 1. sticky && non query task
		// 2. non sticky &&  non query task
		// 3. non sticky && query task
		// for 1, partial history have to be send back
		// for 2 and 3, full history have to be send back

		var persistenceToken []byte

		firstEventID := constants.FirstEventID
		nextEventID := matchingResp.GetNextEventID()
		if matchingResp.GetStickyExecutionEnabled() {
			firstEventID = matchingResp.GetPreviousStartedEventID() + 1
		}
		domainName, dErr := wh.GetDomainCache().GetDomainName(domainID)
		if dErr != nil {
			return nil, dErr
		}
		scope = scope.Tagged(metrics.DomainTag(domainName))
		history, persistenceToken, err = wh.getHistory(
			ctx,
			scope,
			domainID,
			domainName,
			*matchingResp.WorkflowExecution,
			firstEventID,
			nextEventID,
			int32(wh.config.HistoryMaxPageSize(domainName)),
			nil,
			matchingResp.DecisionInfo,
			branchToken,
		)
		if err != nil {
			return nil, err
		}

		if len(persistenceToken) != 0 {
			continuation, err = serializeHistoryToken(&getHistoryContinuationToken{
				RunID:             matchingResp.WorkflowExecution.GetRunID(),
				FirstEventID:      firstEventID,
				NextEventID:       nextEventID,
				PersistenceToken:  persistenceToken,
				TransientDecision: matchingResp.DecisionInfo,
				BranchToken:       branchToken,
			})
			if err != nil {
				return nil, err
			}
		}
	}

	resp := &types.PollForDecisionTaskResponse{
		TaskToken:                 matchingResp.TaskToken,
		WorkflowExecution:         matchingResp.WorkflowExecution,
		WorkflowType:              matchingResp.WorkflowType,
		PreviousStartedEventID:    matchingResp.PreviousStartedEventID,
		StartedEventID:            matchingResp.StartedEventID, // this field is not set for query tasks as there's no decision task started event
		Query:                     matchingResp.Query,
		BacklogCountHint:          matchingResp.BacklogCountHint,
		Attempt:                   matchingResp.Attempt,
		History:                   history,
		NextPageToken:             continuation,
		WorkflowExecutionTaskList: matchingResp.WorkflowExecutionTaskList,
		ScheduledTimestamp:        matchingResp.ScheduledTimestamp,
		StartedTimestamp:          matchingResp.StartedTimestamp,
		Queries:                   matchingResp.Queries,
		NextEventID:               matchingResp.NextEventID,
		TotalHistoryBytes:         matchingResp.TotalHistoryBytes,
		AutoConfigHint:            matchingResp.AutoConfigHint,
	}

	return resp, nil
}

func verifyHistoryIsComplete(
	events []*types.HistoryEvent,
	expectedFirstEventID int64,
	expectedLastEventID int64,
	isFirstPage bool,
	isLastPage bool,
	pageSize int,
) error {

	nEvents := len(events)
	if nEvents == 0 {
		if isLastPage {
			// we seem to be returning a non-nil pageToken on the lastPage which
			// in turn cases the client to call getHistory again - only to find
			// there are no more events to consume - bail out if this is the case here
			return nil
		}
		return fmt.Errorf("invalid history: contains zero events")
	}

	firstEventID := events[0].ID
	lastEventID := events[nEvents-1].ID

	if !isFirstPage { // atleast one page of history has been read previously
		if firstEventID <= expectedFirstEventID {
			// not first page and no events have been read in the previous pages - not possible
			return &types.InternalServiceError{
				Message: fmt.Sprintf(
					"invalid history: expected first eventID to be > %v but got %v", expectedFirstEventID, firstEventID),
			}
		}
		expectedFirstEventID = firstEventID
	}

	if !isLastPage {
		// estimate lastEventID based on pageSize. This is a lower bound
		// since the persistence layer counts "batch of events" as a single page
		expectedLastEventID = expectedFirstEventID + int64(pageSize) - 1
	}

	nExpectedEvents := expectedLastEventID - expectedFirstEventID + 1

	if firstEventID == expectedFirstEventID &&
		((isLastPage && lastEventID == expectedLastEventID && int64(nEvents) == nExpectedEvents) ||
			(!isLastPage && lastEventID >= expectedLastEventID && int64(nEvents) >= nExpectedEvents)) {
		return nil
	}

	return &types.InternalServiceError{
		Message: fmt.Sprintf(
			"incomplete history: "+
				"expected events [%v-%v] but got events [%v-%v] of length %v:"+
				"isFirstPage=%v,isLastPage=%v,pageSize=%v",
			expectedFirstEventID,
			expectedLastEventID,
			firstEventID,
			lastEventID,
			nEvents,
			isFirstPage,
			isLastPage,
			pageSize),
	}
}

func deserializeHistoryToken(bytes []byte) (*getHistoryContinuationToken, error) {
	token := &getHistoryContinuationToken{}
	err := json.Unmarshal(bytes, token)
	return token, err
}

func serializeHistoryToken(token *getHistoryContinuationToken) ([]byte, error) {
	if token == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(token)
	return bytes, err
}

func isFailoverRequest(updateRequest *types.UpdateDomainRequest) bool {
	return updateRequest.ActiveClusterName != nil
}

func isGraceFailoverRequest(updateRequest *types.UpdateDomainRequest) bool {
	return updateRequest.FailoverTimeoutInSeconds != nil
}

func (wh *WorkflowHandler) checkOngoingFailover(
	ctx context.Context,
	domainName *string,
) error {

	enabledClusters := wh.GetClusterMetadata().GetEnabledClusterInfo()
	respChan := make(chan *types.DescribeDomainResponse, len(enabledClusters))

	g := &errgroup.Group{}
	for clusterName := range enabledClusters {
		frontendClient := wh.GetRemoteFrontendClient(clusterName)
		g.Go(func() (e error) {
			defer func() { log.CapturePanic(recover(), wh.GetLogger(), &e) }()

			resp, _ := frontendClient.DescribeDomain(ctx, &types.DescribeDomainRequest{Name: domainName})
			respChan <- resp
			return nil
		})
	}
	g.Wait()
	close(respChan)

	var failoverVersion *int64
	for resp := range respChan {
		if resp == nil {
			return &types.InternalServiceError{
				Message: "Failed to verify failover version from all clusters",
			}
		}
		if failoverVersion == nil {
			failoverVersion = &resp.FailoverVersion
		}
		if *failoverVersion != resp.GetFailoverVersion() {
			return &types.BadRequestError{
				Message: "Concurrent failover is not allow.",
			}
		}
	}
	return nil
}

func (wh *WorkflowHandler) historyArchived(ctx context.Context, request *types.GetWorkflowExecutionHistoryRequest, domainID string) bool {
	if request.GetExecution() == nil || request.GetExecution().GetRunID() == "" {
		return false
	}
	getMutableStateRequest := &types.GetMutableStateRequest{
		DomainUUID: domainID,
		Execution:  request.Execution,
	}
	_, err := wh.GetHistoryClient().GetMutableState(ctx, getMutableStateRequest)
	if err == nil {
		return false
	}
	switch err.(type) {
	case *types.EntityNotExistsError:
		// the only case in which history is assumed to be archived is if getting mutable state returns entity not found error
		return true
	}
	return false
}

func (wh *WorkflowHandler) getArchivedHistory(
	ctx context.Context,
	request *types.GetWorkflowExecutionHistoryRequest,
	domainID string,
) (*types.GetWorkflowExecutionHistoryResponse, error) {
	entry, err := wh.GetDomainCache().GetDomainByID(domainID)
	if err != nil {
		return nil, err
	}

	URIString := entry.GetConfig().HistoryArchivalURI
	if URIString == "" {
		// if URI is empty, it means the domain has never enabled for archival.
		// the error is not "workflow has passed retention period", because
		// we have no way to tell if the requested workflow exists or not.
		return nil, validate.ErrHistoryNotFound
	}

	URI, err := archiver.NewURI(URIString)
	if err != nil {
		return nil, err
	}

	historyArchiver, err := wh.GetArchiverProvider().GetHistoryArchiver(URI.Scheme(), service.Frontend)
	if err != nil {
		return nil, err
	}

	resp, err := historyArchiver.Get(ctx, URI, &archiver.GetHistoryRequest{
		DomainID:      domainID,
		WorkflowID:    request.GetExecution().GetWorkflowID(),
		RunID:         request.GetExecution().GetRunID(),
		NextPageToken: request.GetNextPageToken(),
		PageSize:      int(request.GetMaximumPageSize()),
	})
	if err != nil {
		return nil, err
	}

	history := &types.History{}
	for _, batch := range resp.HistoryBatches {
		history.Events = append(history.Events, batch.Events...)
	}
	return &types.GetWorkflowExecutionHistoryResponse{
		History:       history,
		NextPageToken: resp.NextPageToken,
		Archived:      true,
	}, nil
}

func (wh *WorkflowHandler) convertIndexedKeyToThrift(keys map[string]interface{}) map[string]types.IndexedValueType {
	converted := make(map[string]types.IndexedValueType)
	for k, v := range keys {
		converted[k] = common.ConvertIndexedValueTypeToInternalType(v, wh.GetLogger())
	}
	return converted
}

func (wh *WorkflowHandler) isListRequestPageSizeTooLarge(pageSize int32, domain string) bool {
	return common.IsAdvancedVisibilityReadingEnabled(wh.config.ReadVisibilityStoreName(domain) != "db",
		wh.config.IsAdvancedVisConfigExist) && pageSize > int32(wh.config.ESIndexMaxResultWindow())
}

// GetClusterInfo return information about cadence deployment
func (wh *WorkflowHandler) GetClusterInfo(
	ctx context.Context,
) (resp *types.ClusterInfo, err error) {
	return &types.ClusterInfo{
		SupportedClientVersions: &types.SupportedClientVersions{
			GoSdk:   client.SupportedGoSDKVersion,
			JavaSdk: client.SupportedJavaSDKVersion,
		},
	}, nil
}

type domainWrapper struct {
	domain string
}

func (d domainWrapper) GetDomain() string {
	return d.domain
}

func (hs HealthStatus) String() string {
	switch hs {
	case HealthStatusOK:
		return "OK"
	case HealthStatusWarmingUp:
		return "WarmingUp"
	case HealthStatusShuttingDown:
		return "ShuttingDown"
	default:
		return "unknown"
	}
}

func (wh *WorkflowHandler) emitDescribeWorkflowExecutionMetrics(domain string, response *types.DescribeWorkflowExecutionResponse, err error) {
	scope := wh.GetMetricsClient().Scope(metrics.FrontendDescribeWorkflowExecutionStatusScope, metrics.DomainTag(domain))

	if err != nil || response == nil {
		scope.IncCounter(metrics.DescribeWorkflowStatusError)
		return
	}

	status := "unknown"
	if response.WorkflowExecutionInfo != nil && response.WorkflowExecutionInfo.CloseStatus != nil {
		status = response.WorkflowExecutionInfo.CloseStatus.String()
	}

	scope = scope.Tagged(metrics.WorkflowCloseStatusTag(status))
	scope.IncCounter(metrics.DescribeWorkflowStatusCount)
}

// Some error types are introduced later that some clients might not support
// To make them backward compatible, we continue returning the legacy error types
// for older clients
func (wh *WorkflowHandler) normalizeVersionedErrors(ctx context.Context, err error) error {
	switch err.(type) {
	case *types.WorkflowExecutionAlreadyCompletedError:
		call := yarpc.CallFromContext(ctx)
		clientFeatureVersion := call.Header(common.FeatureVersionHeaderName)
		clientImpl := call.Header(common.ClientImplHeaderName)
		featureFlags := client.GetFeatureFlagsFromHeader(call)

		vErr := wh.versionChecker.SupportsWorkflowAlreadyCompletedError(clientImpl, clientFeatureVersion, featureFlags)
		if vErr == nil {
			return err
		}
		return &types.EntityNotExistsError{Message: "Workflow execution already completed."}
	default:
		return err
	}
}
func constructRestartWorkflowRequest(w *types.WorkflowExecutionStartedEventAttributes, domain string, identity string, workflowID string) *types.StartWorkflowExecutionRequest {

	startRequest := &types.StartWorkflowExecutionRequest{
		RequestID:  uuid.New().String(),
		Domain:     domain,
		WorkflowID: workflowID,
		WorkflowType: &types.WorkflowType{
			Name: w.WorkflowType.Name,
		},
		TaskList: &types.TaskList{
			Name: w.TaskList.Name,
		},
		Input:                               w.Input,
		ExecutionStartToCloseTimeoutSeconds: w.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      w.TaskStartToCloseTimeoutSeconds,
		Identity:                            identity,
		WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyTerminateIfRunning.Ptr(),
	}
	startRequest.CronSchedule = w.CronSchedule
	startRequest.RetryPolicy = w.RetryPolicy
	startRequest.DelayStartSeconds = w.FirstDecisionTaskBackoffSeconds
	startRequest.Header = w.Header
	startRequest.Memo = w.Memo
	startRequest.SearchAttributes = w.SearchAttributes

	return startRequest
}

func getMetricsScopeWithDomain(
	scope int,
	d domainGetter,
	metricsClient metrics.Client,
) metrics.Scope {
	var metricsScope metrics.Scope
	if d != nil {
		metricsScope = metricsClient.Scope(scope).Tagged(metrics.DomainTag(d.GetDomain()))
	} else {
		metricsScope = metricsClient.Scope(scope).Tagged(metrics.DomainUnknownTag())
	}
	return metricsScope
}
