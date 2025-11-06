// Copyright (c) 2017 Uber Technologies, Inc.
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

package decision

import (
	"fmt"
	"strings"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/elasticsearch/validator"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
)

type (
	attrValidator struct {
		config                    *config.Config
		domainCache               cache.DomainCache
		metricsClient             metrics.Client
		logger                    log.Logger
		searchAttributesValidator *validator.SearchAttributesValidator
	}

	workflowSizeChecker struct {
		domainName string

		blobSizeLimitWarn  int
		blobSizeLimitError int

		historySizeLimitWarn  int
		historySizeLimitError int

		historyCountLimitWarn  int
		historyCountLimitError int

		completedID    int64
		mutableState   execution.MutableState
		executionStats *persistence.ExecutionStats
		metricsScope   metrics.Scope
		logger         log.Logger
	}
)

func newAttrValidator(
	domainCache cache.DomainCache,
	metricsClient metrics.Client,
	config *config.Config,
	logger log.Logger,
) *attrValidator {
	return &attrValidator{
		config:        config,
		domainCache:   domainCache,
		metricsClient: metricsClient,
		logger:        logger,
		searchAttributesValidator: validator.NewSearchAttributesValidator(
			logger,
			config.EnableQueryAttributeValidation,
			config.ValidSearchAttributes,
			config.SearchAttributesNumberOfKeysLimit,
			config.SearchAttributesSizeOfValueLimit,
			config.SearchAttributesTotalSizeLimit,
		),
	}
}

func newWorkflowSizeChecker(
	domainName string,
	blobSizeLimitWarn int,
	blobSizeLimitError int,
	historySizeLimitWarn int,
	historySizeLimitError int,
	historyCountLimitWarn int,
	historyCountLimitError int,
	completedID int64,
	mutableState execution.MutableState,
	executionStats *persistence.ExecutionStats,
	metricsScope metrics.Scope,
	logger log.Logger,
) *workflowSizeChecker {
	return &workflowSizeChecker{
		domainName:             domainName,
		blobSizeLimitWarn:      blobSizeLimitWarn,
		blobSizeLimitError:     blobSizeLimitError,
		historySizeLimitWarn:   historySizeLimitWarn,
		historySizeLimitError:  historySizeLimitError,
		historyCountLimitWarn:  historyCountLimitWarn,
		historyCountLimitError: historyCountLimitError,
		completedID:            completedID,
		mutableState:           mutableState,
		executionStats:         executionStats,
		metricsScope:           metricsScope,
		logger:                 logger,
	}
}

func (c *workflowSizeChecker) failWorkflowIfBlobSizeExceedsLimit(
	decisionTypeTag metrics.Tag,
	blob []byte,
	message string,
) (bool, error) {

	executionInfo := c.mutableState.GetExecutionInfo()
	err := common.CheckEventBlobSizeLimit(
		len(blob),
		c.blobSizeLimitWarn,
		c.blobSizeLimitError,
		executionInfo.DomainID,
		c.domainName,
		executionInfo.WorkflowID,
		executionInfo.RunID,
		c.metricsScope.Tagged(decisionTypeTag),
		c.logger,
		tag.BlobSizeViolationOperation(decisionTypeTag.Value()),
	)
	if err == nil {
		return false, nil
	}

	attributes := &types.FailWorkflowExecutionDecisionAttributes{
		Reason:  common.StringPtr(common.FailureReasonDecisionBlobSizeExceedsLimit),
		Details: []byte(message),
	}

	if _, err := c.mutableState.AddFailWorkflowEvent(c.completedID, attributes); err != nil {
		return false, err
	}

	return true, nil
}

func (c *workflowSizeChecker) failWorkflowSizeExceedsLimit() (bool, error) {
	historyCount := int(c.mutableState.GetNextEventID()) - 1
	historySize := int(c.executionStats.HistorySize)

	// metricsScope already has domainName and operation: "RespondDecisionTaskCompleted"
	c.metricsScope.RecordTimer(metrics.HistorySize, time.Duration(historySize))
	c.metricsScope.RecordTimer(metrics.HistoryCount, time.Duration(historyCount))

	if historySize > c.historySizeLimitError || historyCount > c.historyCountLimitError {
		executionInfo := c.mutableState.GetExecutionInfo()
		c.logger.Error("history size exceeds error limit.",
			tag.WorkflowDomainName(c.domainName),
			tag.WorkflowDomainID(executionInfo.DomainID),
			tag.WorkflowID(executionInfo.WorkflowID),
			tag.WorkflowRunID(executionInfo.RunID),
			tag.WorkflowHistorySize(historySize),
			tag.WorkflowEventCount(historyCount))

		attributes := &types.FailWorkflowExecutionDecisionAttributes{
			Reason:  common.StringPtr(common.FailureReasonSizeExceedsLimit),
			Details: []byte("Workflow history size / count exceeds limit."),
		}

		if _, err := c.mutableState.AddFailWorkflowEvent(c.completedID, attributes); err != nil {
			return false, err
		}
		return true, nil
	}

	if historySize > c.historySizeLimitWarn || historyCount > c.historyCountLimitWarn {
		executionInfo := c.mutableState.GetExecutionInfo()
		c.logger.Warn("history size exceeds warn limit.",
			tag.WorkflowDomainName(c.domainName),
			tag.WorkflowDomainID(executionInfo.DomainID),
			tag.WorkflowID(executionInfo.WorkflowID),
			tag.WorkflowRunID(executionInfo.RunID),
			tag.WorkflowHistorySize(historySize),
			tag.WorkflowEventCount(historyCount))
		return false, nil
	}

	return false, nil
}

func (v *attrValidator) validateActivityScheduleAttributes(
	domainID string,
	targetDomainID string,
	attributes *types.ScheduleActivityTaskDecisionAttributes,
	executionInfo *persistence.WorkflowExecutionInfo,
	metricsScope metrics.ScopeIdx,
) error {

	if err := v.validateCrossDomainCall(
		domainID,
		targetDomainID,
	); err != nil {
		return err
	}

	if attributes == nil {
		return &types.BadRequestError{Message: "ScheduleActivityTaskDecisionAttributes is not set on decision."}
	}

	taskList, err := v.validatedTaskList(attributes.TaskList, &types.TaskList{Name: executionInfo.TaskList, Kind: executionInfo.TaskListKind.Ptr()}, metricsScope, attributes.GetDomain())
	if err != nil {
		return err
	}
	attributes.TaskList = taskList

	if attributes.GetActivityID() == "" {
		return &types.BadRequestError{Message: "ActivityId is not set on decision."}
	}

	if attributes.ActivityType == nil || attributes.ActivityType.GetName() == "" {
		return &types.BadRequestError{Message: "ActivityType is not set on decision."}
	}

	if err := common.ValidateRetryPolicy(attributes.RetryPolicy); err != nil {
		return err
	}

	idLengthWarnLimit := v.config.MaxIDLengthWarnLimit()
	if !common.IsValidIDLength(
		attributes.GetActivityID(),
		v.metricsClient.Scope(metricsScope),
		idLengthWarnLimit,
		v.config.ActivityIDMaxLength(attributes.GetDomain()),
		metrics.CadenceErrActivityIDExceededWarnLimit,
		attributes.GetDomain(),
		v.logger,
		tag.IDTypeActivityID) {
		return &types.BadRequestError{Message: "ActivityID exceeds length limit."}
	}

	if !common.IsValidIDLength(
		attributes.GetActivityType().GetName(),
		v.metricsClient.Scope(metricsScope),
		idLengthWarnLimit,
		v.config.ActivityTypeMaxLength(attributes.GetDomain()),
		metrics.CadenceErrActivityTypeExceededWarnLimit,
		attributes.GetDomain(),
		v.logger,
		tag.IDTypeActivityType) {
		return &types.BadRequestError{Message: "ActivityType exceeds length limit."}
	}

	if !common.IsValidIDLength(
		attributes.GetDomain(),
		v.metricsClient.Scope(metricsScope),
		idLengthWarnLimit,
		v.config.DomainNameMaxLength(attributes.GetDomain()),
		metrics.CadenceErrDomainNameExceededWarnLimit,
		attributes.GetDomain(),
		v.logger,
		tag.IDTypeDomainName) {
		return &types.BadRequestError{Message: "Domain exceeds length limit."}
	}

	// Only attempt to deduce and fill in unspecified timeouts only when all timeouts are non-negative.
	if attributes.GetScheduleToCloseTimeoutSeconds() < 0 || attributes.GetScheduleToStartTimeoutSeconds() < 0 ||
		attributes.GetStartToCloseTimeoutSeconds() < 0 || attributes.GetHeartbeatTimeoutSeconds() < 0 {
		return &types.BadRequestError{Message: "A valid timeout may not be negative."}
	}
	wfTimeout := executionInfo.WorkflowTimeout

	// ensure activity timeout never larger than workflow timeout
	if attributes.GetScheduleToCloseTimeoutSeconds() > wfTimeout {
		attributes.ScheduleToCloseTimeoutSeconds = common.Int32Ptr(wfTimeout)
	}
	if attributes.GetScheduleToStartTimeoutSeconds() > wfTimeout {
		attributes.ScheduleToStartTimeoutSeconds = common.Int32Ptr(wfTimeout)
	}
	if attributes.GetStartToCloseTimeoutSeconds() > wfTimeout {
		attributes.StartToCloseTimeoutSeconds = common.Int32Ptr(wfTimeout)
	}
	if attributes.GetHeartbeatTimeoutSeconds() > wfTimeout {
		attributes.HeartbeatTimeoutSeconds = common.Int32Ptr(wfTimeout)
	}

	validScheduleToClose := attributes.GetScheduleToCloseTimeoutSeconds() > 0
	validScheduleToStart := attributes.GetScheduleToStartTimeoutSeconds() > 0
	validStartToClose := attributes.GetStartToCloseTimeoutSeconds() > 0

	if validScheduleToClose {
		if !validScheduleToStart {
			attributes.ScheduleToStartTimeoutSeconds = common.Int32Ptr(attributes.GetScheduleToCloseTimeoutSeconds())
		}
		if !validStartToClose {
			attributes.StartToCloseTimeoutSeconds = common.Int32Ptr(attributes.GetScheduleToCloseTimeoutSeconds())
		}
	} else if validScheduleToStart && validStartToClose {
		attributes.ScheduleToCloseTimeoutSeconds = common.Int32Ptr(attributes.GetScheduleToStartTimeoutSeconds() + attributes.GetStartToCloseTimeoutSeconds())
		if attributes.GetScheduleToCloseTimeoutSeconds() > wfTimeout {
			attributes.ScheduleToCloseTimeoutSeconds = common.Int32Ptr(wfTimeout)
		}
	} else {
		// Deduction failed as there's not enough information to fill in missing timeouts.
		return &types.BadRequestError{Message: "A valid ScheduleToCloseTimeout is not set on decision."}
	}

	// ensure activity's SCHEDULE_TO_START and SCHEDULE_TO_CLOSE is as long as expiration on retry policy
	// if SCHEDULE_TO_START timeout is retryable
	p := attributes.RetryPolicy
	if p != nil {
		isScheduleToStartRetryable := true
		scheduleToStartErrorReason := execution.TimerTypeToReason(execution.TimerTypeScheduleToStart)
		for _, reason := range p.GetNonRetriableErrorReasons() {
			if reason == scheduleToStartErrorReason {
				isScheduleToStartRetryable = false
				break
			}
		}

		expiration := p.GetExpirationIntervalInSeconds()
		if expiration == 0 || expiration > wfTimeout {
			expiration = wfTimeout
		}

		if isScheduleToStartRetryable {
			// If schedule to start timeout is retryable, we don't need to fail the activity and schedule
			// it again on the same tasklist, as it's a no-op). Extending schedule to start timeout to achieve
			// the same thing.
			//
			// Theoretically, we can extend schedule to start to be as long as the expiration time,
			// but if user specifies a very long expiration time and activity task got lost after the activity is
			// scheduled, workflow will be stuck for a long time. So here, we cap the schedule to start timeout
			// to a maximum value, so that when the activity task got lost, timeout can happen sooner and schedule
			// the activity again.

			domainName, _ := v.domainCache.GetDomainName(domainID) // if this call returns an error, we will just used the default value for max timeout
			maximumScheduleToStartTimeoutForRetryInSeconds := int32(v.config.ActivityMaxScheduleToStartTimeoutForRetry(domainName).Seconds())
			scheduleToStartExpiration := min(expiration, maximumScheduleToStartTimeoutForRetryInSeconds)
			if attributes.GetScheduleToStartTimeoutSeconds() < scheduleToStartExpiration {
				attributes.ScheduleToStartTimeoutSeconds = common.Int32Ptr(scheduleToStartExpiration)
			}

			// TODO: uncomment the following code when the client side bug for calculating scheduleToClose deadline is fixed and
			// fully rolled out. Before that, we still need to extend scheduleToClose timeout to be as long as the expiration interval
			//
			// scheduleToCloseExpiration := common.MinInt32(expiration, scheduleToStartExpiration+attributes.GetStartToCloseTimeoutSeconds())
			// if attributes.GetScheduleToCloseTimeoutSeconds() < scheduleToCloseExpiration {
			// 	attributes.ScheduleToCloseTimeoutSeconds = common.Int32Ptr(scheduleToCloseExpiration)
			// }
		}

		if attributes.GetScheduleToCloseTimeoutSeconds() < expiration {
			attributes.ScheduleToCloseTimeoutSeconds = common.Int32Ptr(expiration)
		}
	}
	return nil
}

func (v *attrValidator) validateTimerScheduleAttributes(
	attributes *types.StartTimerDecisionAttributes,
	metricsScope metrics.ScopeIdx,
	domain string,
) error {

	if attributes == nil {
		return &types.BadRequestError{Message: "StartTimerDecisionAttributes is not set on decision."}
	}
	if attributes.GetTimerID() == "" {
		return &types.BadRequestError{Message: "TimerId is not set on decision."}
	}
	if !common.IsValidIDLength(
		attributes.GetTimerID(),
		v.metricsClient.Scope(metricsScope),
		v.config.MaxIDLengthWarnLimit(),
		v.config.TimerIDMaxLength(domain),
		metrics.CadenceErrTimerIDExceededWarnLimit,
		domain,
		v.logger,
		tag.IDTypeTimerID) {
		return &types.BadRequestError{Message: "TimerId exceeds length limit."}
	}
	if attributes.GetStartToFireTimeoutSeconds() <= 0 {
		return &types.BadRequestError{
			Message: fmt.Sprintf("Invalid StartToFireTimeoutSeconds: %v", attributes.GetStartToFireTimeoutSeconds()),
		}
	}
	return nil
}

func (v *attrValidator) validateActivityCancelAttributes(
	attributes *types.RequestCancelActivityTaskDecisionAttributes,
	metricsScope metrics.ScopeIdx,
	domain string,
) error {

	if attributes == nil {
		return &types.BadRequestError{Message: "RequestCancelActivityTaskDecisionAttributes is not set on decision."}
	}
	if attributes.GetActivityID() == "" {
		return &types.BadRequestError{Message: "ActivityId is not set on decision."}
	}

	if !common.IsValidIDLength(
		attributes.GetActivityID(),
		v.metricsClient.Scope(metricsScope),
		v.config.MaxIDLengthWarnLimit(),
		v.config.ActivityIDMaxLength(domain),
		metrics.CadenceErrActivityIDExceededWarnLimit,
		domain,
		v.logger,
		tag.IDTypeActivityID) {
		return &types.BadRequestError{Message: "ActivityId exceeds length limit."}
	}
	return nil
}

func (v *attrValidator) validateTimerCancelAttributes(
	attributes *types.CancelTimerDecisionAttributes,
	metricsScope metrics.ScopeIdx,
	domain string,
) error {

	if attributes == nil {
		return &types.BadRequestError{Message: "CancelTimerDecisionAttributes is not set on decision."}
	}
	if attributes.GetTimerID() == "" {
		return &types.BadRequestError{Message: "TimerId is not set on decision."}
	}
	if !common.IsValidIDLength(
		attributes.GetTimerID(),
		v.metricsClient.Scope(metricsScope),
		v.config.MaxIDLengthWarnLimit(),
		v.config.TimerIDMaxLength(domain),
		metrics.CadenceErrTimerIDExceededWarnLimit,
		domain,
		v.logger,
		tag.IDTypeTimerID) {
		return &types.BadRequestError{Message: "TimerId exceeds length limit."}
	}
	return nil
}

func (v *attrValidator) validateRecordMarkerAttributes(
	attributes *types.RecordMarkerDecisionAttributes,
	metricsScope metrics.ScopeIdx,
	domain string,
) error {

	if attributes == nil {
		return &types.BadRequestError{Message: "RecordMarkerDecisionAttributes is not set on decision."}
	}
	if attributes.GetMarkerName() == "" {
		return &types.BadRequestError{Message: "MarkerName is not set on decision."}
	}
	if !common.IsValidIDLength(
		attributes.GetMarkerName(),
		v.metricsClient.Scope(metricsScope),
		v.config.MaxIDLengthWarnLimit(),
		v.config.MarkerNameMaxLength(domain),
		metrics.CadenceErrMarkerNameExceededWarnLimit,
		domain,
		v.logger,
		tag.IDTypeMarkerName) {
		return &types.BadRequestError{Message: "MarkerName exceeds length limit."}
	}

	return nil
}

func (v *attrValidator) validateCompleteWorkflowExecutionAttributes(
	attributes *types.CompleteWorkflowExecutionDecisionAttributes,
) error {

	if attributes == nil {
		return &types.BadRequestError{Message: "CompleteWorkflowExecutionDecisionAttributes is not set on decision."}
	}
	return nil
}

func (v *attrValidator) validateFailWorkflowExecutionAttributes(
	attributes *types.FailWorkflowExecutionDecisionAttributes,
) error {

	if attributes == nil {
		return &types.BadRequestError{Message: "FailWorkflowExecutionDecisionAttributes is not set on decision."}
	}
	if attributes.Reason == nil {
		return &types.BadRequestError{Message: "Reason is not set on decision."}
	}
	return nil
}

func (v *attrValidator) validateCancelWorkflowExecutionAttributes(
	attributes *types.CancelWorkflowExecutionDecisionAttributes,
) error {

	if attributes == nil {
		return &types.BadRequestError{Message: "CancelWorkflowExecutionDecisionAttributes is not set on decision."}
	}
	return nil
}

func (v *attrValidator) validateCancelExternalWorkflowExecutionAttributes(
	domainID string,
	targetDomainID string,
	attributes *types.RequestCancelExternalWorkflowExecutionDecisionAttributes,
	metricsScope metrics.ScopeIdx,
) error {

	if err := v.validateCrossDomainCall(
		domainID,
		targetDomainID,
	); err != nil {
		return err
	}

	if attributes == nil {
		return &types.BadRequestError{Message: "RequestCancelExternalWorkflowExecutionDecisionAttributes is not set on decision."}
	}
	if attributes.WorkflowID == "" {
		return &types.BadRequestError{Message: "WorkflowId is not set on decision."}
	}

	idLengthWarnLimit := v.config.MaxIDLengthWarnLimit()
	if !common.IsValidIDLength(
		attributes.GetDomain(),
		v.metricsClient.Scope(metricsScope),
		idLengthWarnLimit,
		v.config.DomainNameMaxLength(attributes.GetDomain()),
		metrics.CadenceErrDomainNameExceededWarnLimit,
		attributes.GetDomain(),
		v.logger,
		tag.IDTypeDomainName) {
		return &types.BadRequestError{Message: "Domain exceeds length limit."}
	}

	if !common.IsValidIDLength(
		attributes.GetWorkflowID(),
		v.metricsClient.Scope(metricsScope),
		idLengthWarnLimit,
		v.config.WorkflowIDMaxLength(attributes.GetDomain()),
		metrics.CadenceErrWorkflowIDExceededWarnLimit,
		attributes.GetDomain(),
		v.logger,
		tag.IDTypeWorkflowID) {
		return &types.BadRequestError{Message: "WorkflowId exceeds length limit."}
	}
	runID := attributes.GetRunID()
	if runID != "" && uuid.Parse(runID) == nil {
		return &types.BadRequestError{Message: "Invalid RunId set on decision."}
	}

	return nil
}

func (v *attrValidator) validateSignalExternalWorkflowExecutionAttributes(
	domainID string,
	targetDomainID string,
	attributes *types.SignalExternalWorkflowExecutionDecisionAttributes,
	metricsScope metrics.ScopeIdx,
) error {

	if err := v.validateCrossDomainCall(
		domainID,
		targetDomainID,
	); err != nil {
		return err
	}

	if attributes == nil {
		return &types.BadRequestError{Message: "SignalExternalWorkflowExecutionDecisionAttributes is not set on decision."}
	}
	if attributes.Execution == nil {
		return &types.BadRequestError{Message: "Execution is nil on decision."}
	}
	if attributes.Execution.WorkflowID == "" {
		return &types.BadRequestError{Message: "WorkflowId is not set on decision."}
	}

	idLengthWarnLimit := v.config.MaxIDLengthWarnLimit()
	if !common.IsValidIDLength(
		attributes.GetDomain(),
		v.metricsClient.Scope(metricsScope),
		idLengthWarnLimit,
		v.config.DomainNameMaxLength(attributes.GetDomain()),
		metrics.CadenceErrDomainNameExceededWarnLimit,
		attributes.GetDomain(),
		v.logger,
		tag.IDTypeDomainName) {
		return &types.BadRequestError{Message: "Domain exceeds length limit."}
	}

	if !common.IsValidIDLength(
		attributes.Execution.GetWorkflowID(),
		v.metricsClient.Scope(metricsScope),
		idLengthWarnLimit,
		v.config.WorkflowIDMaxLength(attributes.GetDomain()),
		metrics.CadenceErrWorkflowIDExceededWarnLimit,
		attributes.GetDomain(),
		v.logger,
		tag.IDTypeWorkflowID) {
		return &types.BadRequestError{Message: "WorkflowId exceeds length limit."}
	}

	targetRunID := attributes.Execution.GetRunID()
	if targetRunID != "" && uuid.Parse(targetRunID) == nil {
		return &types.BadRequestError{Message: "Invalid RunId set on decision."}
	}
	if attributes.SignalName == "" {
		return &types.BadRequestError{Message: "SignalName is not set on decision."}
	}

	return nil
}

func (v *attrValidator) validateUpsertWorkflowSearchAttributes(
	domainName string,
	attributes *types.UpsertWorkflowSearchAttributesDecisionAttributes,
) error {

	if attributes == nil {
		return &types.BadRequestError{Message: "UpsertWorkflowSearchAttributesDecisionAttributes is not set on decision."}
	}

	if attributes.SearchAttributes == nil {
		return &types.BadRequestError{Message: "SearchAttributes is not set on decision."}
	}

	if len(attributes.GetSearchAttributes().GetIndexedFields()) == 0 {
		return &types.BadRequestError{Message: "IndexedFields is empty on decision."}
	}

	return v.searchAttributesValidator.ValidateSearchAttributes(attributes.GetSearchAttributes(), domainName)
}

func (v *attrValidator) validateContinueAsNewWorkflowExecutionAttributes(
	attributes *types.ContinueAsNewWorkflowExecutionDecisionAttributes,
	executionInfo *persistence.WorkflowExecutionInfo,
	metricsScope metrics.ScopeIdx,
	domain string,
) error {

	if attributes == nil {
		return &types.BadRequestError{Message: "ContinueAsNewWorkflowExecutionDecisionAttributes is not set on decision."}
	}

	// Inherit workflow type from previous execution if not provided on decision
	if attributes.WorkflowType == nil || attributes.WorkflowType.GetName() == "" {
		attributes.WorkflowType = &types.WorkflowType{Name: executionInfo.WorkflowTypeName}
	}

	if !common.IsValidIDLength(
		attributes.WorkflowType.GetName(),
		v.metricsClient.Scope(metricsScope),
		v.config.MaxIDLengthWarnLimit(),
		v.config.WorkflowTypeMaxLength(domain),
		metrics.CadenceErrWorkflowTypeExceededWarnLimit,
		domain,
		v.logger,
		tag.IDTypeWorkflowType) {
		return &types.BadRequestError{Message: "WorkflowType exceeds length limit."}
	}

	// Inherit Tasklist from previous execution if not provided on decision
	taskList, err := v.validatedTaskList(attributes.TaskList, &types.TaskList{Name: executionInfo.TaskList, Kind: executionInfo.TaskListKind.Ptr()}, metricsScope, domain)
	if err != nil {
		return err
	}
	attributes.TaskList = taskList

	// Inherit workflow timeout from previous execution if not provided on decision
	if attributes.GetExecutionStartToCloseTimeoutSeconds() <= 0 {
		attributes.ExecutionStartToCloseTimeoutSeconds = common.Int32Ptr(executionInfo.WorkflowTimeout)
	}

	// Inherit decision task timeout from previous execution if not provided on decision
	if attributes.GetTaskStartToCloseTimeoutSeconds() <= 0 {
		attributes.TaskStartToCloseTimeoutSeconds = common.Int32Ptr(executionInfo.DecisionStartToCloseTimeout)
	}

	// Check next run decision task delay
	if attributes.GetBackoffStartIntervalInSeconds() < 0 {
		return &types.BadRequestError{Message: "BackoffStartInterval is less than 0."}
	}

	domainName, err := v.domainCache.GetDomainName(executionInfo.DomainID)
	if err != nil {
		return err
	}
	return v.searchAttributesValidator.ValidateSearchAttributes(attributes.GetSearchAttributes(), domainName)
}

func (v *attrValidator) validateStartChildExecutionAttributes(
	domainID string,
	targetDomainID string,
	attributes *types.StartChildWorkflowExecutionDecisionAttributes,
	parentInfo *persistence.WorkflowExecutionInfo,
	metricsScope metrics.ScopeIdx,
) error {

	if err := v.validateCrossDomainCall(
		domainID,
		targetDomainID,
	); err != nil {
		return err
	}

	if attributes == nil {
		return &types.BadRequestError{Message: "StartChildWorkflowExecutionDecisionAttributes is not set on decision."}
	}

	if attributes.GetWorkflowID() == "" {
		return &types.BadRequestError{Message: "Required field WorkflowID is not set on decision."}
	}

	if attributes.WorkflowType == nil || attributes.WorkflowType.GetName() == "" {
		return &types.BadRequestError{Message: "Required field WorkflowType is not set on decision."}
	}

	idLengthWarnLimit := v.config.MaxIDLengthWarnLimit()
	if !common.IsValidIDLength(
		attributes.GetDomain(),
		v.metricsClient.Scope(metricsScope),
		idLengthWarnLimit,
		v.config.DomainNameMaxLength(attributes.GetDomain()),
		metrics.CadenceErrDomainNameExceededWarnLimit,
		attributes.GetDomain(),
		v.logger,
		tag.IDTypeDomainName) {
		return &types.BadRequestError{Message: "Domain exceeds length limit."}
	}
	if !common.IsValidIDLength(
		attributes.GetWorkflowID(),
		v.metricsClient.Scope(metricsScope),
		idLengthWarnLimit,
		v.config.WorkflowIDMaxLength(attributes.GetDomain()),
		metrics.CadenceErrWorkflowIDExceededWarnLimit,
		attributes.GetDomain(),
		v.logger,
		tag.IDTypeWorkflowID) {
		return &types.BadRequestError{Message: "WorkflowId exceeds length limit."}
	}

	if !common.IsValidIDLength(
		attributes.WorkflowType.GetName(),
		v.metricsClient.Scope(metricsScope),
		idLengthWarnLimit,
		v.config.WorkflowTypeMaxLength(attributes.GetDomain()),
		metrics.CadenceErrWorkflowTypeExceededWarnLimit,
		attributes.GetDomain(),
		v.logger,
		tag.IDTypeWorkflowType) {
		return &types.BadRequestError{Message: "WorkflowType exceeds length limit."}
	}

	if err := common.ValidateRetryPolicy(attributes.RetryPolicy); err != nil {
		return err
	}

	if attributes.GetCronSchedule() != "" {
		if _, err := backoff.ValidateSchedule(attributes.GetCronSchedule()); err != nil {
			return err
		}
	}

	// Inherit tasklist from parent workflow execution if not provided on decision
	taskList, err := v.validatedTaskList(attributes.TaskList, &types.TaskList{Name: parentInfo.TaskList, Kind: parentInfo.TaskListKind.Ptr()}, metricsScope, attributes.GetDomain())
	if err != nil {
		return err
	}
	attributes.TaskList = taskList

	// Inherit workflow timeout from parent workflow execution if not provided on decision
	if attributes.GetExecutionStartToCloseTimeoutSeconds() <= 0 {
		attributes.ExecutionStartToCloseTimeoutSeconds = common.Int32Ptr(parentInfo.WorkflowTimeout)
	}

	// Inherit decision task timeout from parent workflow execution if not provided on decision
	if attributes.GetTaskStartToCloseTimeoutSeconds() <= 0 {
		attributes.TaskStartToCloseTimeoutSeconds = common.Int32Ptr(parentInfo.DecisionStartToCloseTimeout)
	}

	return nil
}

func (v *attrValidator) validatedTaskList(
	taskList *types.TaskList,
	defaultVal *types.TaskList,
	metricsScope metrics.ScopeIdx,
	domain string,
) (*types.TaskList, error) {

	if taskList == nil {
		taskList = &types.TaskList{}
	}

	if taskList.GetName() == "" {
		if defaultVal.GetName() == "" {
			return taskList, &types.BadRequestError{Message: "missing task list name"}
		}
		taskList.Name = defaultVal.GetName()
		taskList.Kind = defaultVal.Kind
		return taskList, nil
	}
	if taskList.GetKind() == types.TaskListKindNormal && defaultVal.GetKind() == types.TaskListKindEphemeral {
		taskList.Kind = defaultVal.Kind
	}

	name := taskList.GetName()
	if !common.IsValidIDLength(
		name,
		v.metricsClient.Scope(metricsScope),
		v.config.MaxIDLengthWarnLimit(),
		v.config.TaskListNameMaxLength(domain),
		metrics.CadenceErrTaskListNameExceededWarnLimit,
		domain,
		v.logger,
		tag.IDTypeTaskListName) {
		return taskList, &types.BadRequestError{
			Message: fmt.Sprintf("task list name exceeds length limit of %v", v.config.TaskListNameMaxLength(domain)),
		}
	}

	if strings.HasPrefix(name, constants.ReservedTaskListPrefix) {
		return taskList, &types.BadRequestError{
			Message: fmt.Sprintf("task list name cannot start with reserved prefix %v", constants.ReservedTaskListPrefix),
		}
	}

	return taskList, nil
}

func (v *attrValidator) validateCrossDomainCall(
	sourceDomainID string,
	targetDomainID string,
) error {

	// same name, no check needed
	if sourceDomainID == targetDomainID {
		return nil
	}

	sourceDomainEntry, err := v.domainCache.GetDomainByID(sourceDomainID)
	if err != nil {
		return err
	}

	targetDomainEntry, err := v.domainCache.GetDomainByID(targetDomainID)
	if err != nil {
		return err
	}

	sourceClusters := sourceDomainEntry.GetReplicationConfig().Clusters
	targetClusters := targetDomainEntry.GetReplicationConfig().Clusters

	// both "local domain"
	// here a domain is "local domain" when:
	// - IsGlobalDomain() returns false
	// - domainCluster contains only one cluster
	// case 1 can be actually be combined with this case
	if len(sourceClusters) == 1 && len(targetClusters) == 1 {
		if sourceClusters[0].ClusterName == targetClusters[0].ClusterName {
			return nil
		}
		return v.createCrossDomainCallError(sourceDomainEntry, targetDomainEntry)
	}

	// both global domain with > 1 replication cluster
	// when code reaches here, at least one domain has more than one cluster
	if len(sourceClusters) == len(targetClusters) &&
		v.config.EnableCrossClusterOperationsForDomain(sourceDomainEntry.GetInfo().Name) {
		// check if the source domain cluster matches those for the target domain
		for _, sourceCluster := range sourceClusters {
			found := false
			for _, targetCluster := range targetClusters {
				if sourceCluster.ClusterName == targetCluster.ClusterName {
					found = true
					break
				}
			}
			if !found {
				return v.createCrossDomainCallError(sourceDomainEntry, targetDomainEntry)
			}
		}
		return nil
	}

	return v.createCrossDomainCallError(sourceDomainEntry, targetDomainEntry)
}

func (v *attrValidator) createCrossDomainCallError(
	domainEntry *cache.DomainCacheEntry,
	targetDomainEntry *cache.DomainCacheEntry,
) error {
	return &types.BadRequestError{Message: fmt.Sprintf(
		"cannot make cross domain call between %v and %v",
		domainEntry.GetInfo().Name,
		targetDomainEntry.GetInfo().Name,
	)}
}
