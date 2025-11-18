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

package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/dgryski/go-farm"
	"github.com/pborman/uuid"
	"go.uber.org/yarpc/yarpcerrors"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/constants"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

const (
	golandMapReserverNumberOfBytes = 48

	retryPersistenceOperationInitialInterval    = 50 * time.Millisecond
	retryPersistenceOperationMaxInterval        = 10 * time.Second
	retryPersistenceOperationExpirationInterval = 30 * time.Second

	historyServiceOperationInitialInterval    = 50 * time.Millisecond
	historyServiceOperationMaxInterval        = 10 * time.Second
	historyServiceOperationExpirationInterval = 30 * time.Second

	matchingServiceOperationInitialInterval    = 1000 * time.Millisecond
	matchingServiceOperationMaxInterval        = 10 * time.Second
	matchingServiceOperationExpirationInterval = 30 * time.Second

	frontendServiceOperationInitialInterval    = 200 * time.Millisecond
	frontendServiceOperationMaxInterval        = 5 * time.Second
	frontendServiceOperationExpirationInterval = 15 * time.Second

	shardDistributorServiceOperationInitialInterval    = 200 * time.Millisecond
	shardDistributorServiceOperationMaxInterval        = 10 * time.Second
	shardDistributorServiceOperationExpirationInterval = 15 * time.Second

	adminServiceOperationInitialInterval    = 200 * time.Millisecond
	adminServiceOperationMaxInterval        = 5 * time.Second
	adminServiceOperationExpirationInterval = 15 * time.Second

	retryKafkaOperationInitialInterval = 50 * time.Millisecond
	retryKafkaOperationMaxInterval     = 10 * time.Second
	retryKafkaOperationMaxAttempts     = 10

	retryTaskProcessingInitialInterval = 50 * time.Millisecond
	retryTaskProcessingMaxInterval     = 100 * time.Millisecond
	retryTaskProcessingMaxAttempts     = 3

	replicationServiceBusyInitialInterval    = 2 * time.Second
	replicationServiceBusyMaxInterval        = 10 * time.Second
	replicationServiceBusyExpirationInterval = 5 * time.Minute

	taskCompleterInitialInterval    = 1 * time.Second
	taskCompleterMaxInterval        = 10 * time.Second
	taskCompleterExpirationInterval = 5 * time.Minute

	domainCacheInitialInterval    = 1 * time.Second
	domainCacheMaxInterval        = 5 * time.Second
	domainCacheExpirationInterval = 2 * time.Minute

	contextExpireThreshold = 10 * time.Millisecond

	// FailureReasonCompleteResultExceedsLimit is failureReason for complete result exceeds limit
	FailureReasonCompleteResultExceedsLimit = "COMPLETE_RESULT_EXCEEDS_LIMIT"
	// FailureReasonFailureDetailsExceedsLimit is failureReason for failure details exceeds limit
	FailureReasonFailureDetailsExceedsLimit = "FAILURE_DETAILS_EXCEEDS_LIMIT"
	// FailureReasonCancelDetailsExceedsLimit is failureReason for cancel details exceeds limit
	FailureReasonCancelDetailsExceedsLimit = "CANCEL_DETAILS_EXCEEDS_LIMIT"
	// FailureReasonHeartbeatExceedsLimit is failureReason for heartbeat exceeds limit
	FailureReasonHeartbeatExceedsLimit = "HEARTBEAT_EXCEEDS_LIMIT"
	// FailureReasonDecisionBlobSizeExceedsLimit is the failureReason for decision blob exceeds size limit
	FailureReasonDecisionBlobSizeExceedsLimit = "DECISION_BLOB_SIZE_EXCEEDS_LIMIT"
	// FailureReasonSizeExceedsLimit is reason to fail workflow when history size or count exceed limit
	FailureReasonSizeExceedsLimit = "HISTORY_EXCEEDS_LIMIT"
	// FailureReasonTransactionSizeExceedsLimit is the failureReason for when transaction cannot be committed because it exceeds size limit
	FailureReasonTransactionSizeExceedsLimit = "TRANSACTION_SIZE_EXCEEDS_LIMIT"
	// FailureReasonDecisionAttemptsExceedsLimit is reason to fail workflow when decision attempts fail too many times
	FailureReasonDecisionAttemptsExceedsLimit = "DECISION_ATTEMPTS_EXCEEDS_LIMIT"
	// FailureReasonPendingActivityExceedsLimit is reason to fail overflow when pending activity exceeds limit
	FailureReasonPendingActivityExceedsLimit = "PENDING_ACTIVITY_EXCEEDS_LIMIT"
)

var (
	// ErrBlobSizeExceedsLimit is error for event blob size exceeds limit
	ErrBlobSizeExceedsLimit = &types.BadRequestError{Message: "Blob data size exceeds limit."}
	// ErrContextTimeoutTooShort is error for setting a very short context timeout when calling a long poll API
	ErrContextTimeoutTooShort = &types.BadRequestError{Message: "Context timeout is too short."}
	// ErrContextTimeoutNotSet is error for not setting a context timeout when calling a long poll API
	ErrContextTimeoutNotSet = &types.BadRequestError{Message: "Context timeout is not set."}
	// ErrDecisionResultCountTooLarge error for decision result count exceeds limit
	ErrDecisionResultCountTooLarge = &types.BadRequestError{Message: "Decision result count exceeds limit."}
	stickyTaskListMetricTag        = metrics.TaskListTag("__sticky__")
	ephemeralTaskListMetricTag     = metrics.TaskListTag("__ephemeral__")
)

// AwaitWaitGroup calls Wait on the given wait
// Returns true if the Wait() call succeeded before the timeout
// Returns false if the Wait() did not return before the timeout
func AwaitWaitGroup(wg *sync.WaitGroup, timeout time.Duration) bool {
	doneC := make(chan struct{})

	go func() {
		wg.Wait()
		close(doneC)
	}()

	select {
	case <-doneC:
		return true
	case <-time.After(timeout):
		return false
	}
}

// CreatePersistenceRetryPolicy creates a retry policy for persistence layer operations
func CreatePersistenceRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(retryPersistenceOperationInitialInterval)
	policy.SetMaximumInterval(retryPersistenceOperationMaxInterval)
	policy.SetExpirationInterval(retryPersistenceOperationExpirationInterval)

	return policy
}

// CreateHistoryServiceRetryPolicy creates a retry policy for calls to history service
func CreateHistoryServiceRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(historyServiceOperationInitialInterval)
	policy.SetMaximumInterval(historyServiceOperationMaxInterval)
	policy.SetExpirationInterval(historyServiceOperationExpirationInterval)

	return policy
}

// CreateMatchingServiceRetryPolicy creates a retry policy for calls to matching service
func CreateMatchingServiceRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(matchingServiceOperationInitialInterval)
	policy.SetMaximumInterval(matchingServiceOperationMaxInterval)
	policy.SetExpirationInterval(matchingServiceOperationExpirationInterval)

	return policy
}

// CreateFrontendServiceRetryPolicy creates a retry policy for calls to frontend service
func CreateFrontendServiceRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(frontendServiceOperationInitialInterval)
	policy.SetMaximumInterval(frontendServiceOperationMaxInterval)
	policy.SetExpirationInterval(frontendServiceOperationExpirationInterval)

	return policy
}

func CreateShardDistributorServiceRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(shardDistributorServiceOperationInitialInterval)
	policy.SetMaximumInterval(shardDistributorServiceOperationMaxInterval)
	policy.SetExpirationInterval(shardDistributorServiceOperationExpirationInterval)

	return policy
}

// CreateAdminServiceRetryPolicy creates a retry policy for calls to matching service
func CreateAdminServiceRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(adminServiceOperationInitialInterval)
	policy.SetMaximumInterval(adminServiceOperationMaxInterval)
	policy.SetExpirationInterval(adminServiceOperationExpirationInterval)

	return policy
}

// CreateDlqPublishRetryPolicy creates a retry policy for kafka operation
func CreateDlqPublishRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(retryKafkaOperationInitialInterval)
	policy.SetMaximumInterval(retryKafkaOperationMaxInterval)
	policy.SetMaximumAttempts(retryKafkaOperationMaxAttempts)

	return policy
}

// CreateTaskProcessingRetryPolicy creates a retry policy for task processing
func CreateTaskProcessingRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(retryTaskProcessingInitialInterval)
	policy.SetMaximumInterval(retryTaskProcessingMaxInterval)
	policy.SetMaximumAttempts(retryTaskProcessingMaxAttempts)

	return policy
}

// CreateReplicationServiceBusyRetryPolicy creates a retry policy to handle replication service busy
func CreateReplicationServiceBusyRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(replicationServiceBusyInitialInterval)
	policy.SetMaximumInterval(replicationServiceBusyMaxInterval)
	policy.SetExpirationInterval(replicationServiceBusyExpirationInterval)

	return policy
}

// CreateTaskCompleterRetryPolicy creates a retry policy to handle tasks not being started
func CreateTaskCompleterRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(taskCompleterInitialInterval)
	policy.SetMaximumInterval(taskCompleterMaxInterval)
	policy.SetExpirationInterval(taskCompleterExpirationInterval)

	return policy
}

// CreateDomainCacheRetryPolicy creates a retry policy to handle domain cache refresh failures
func CreateDomainCacheRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(domainCacheInitialInterval)
	policy.SetMaximumInterval(domainCacheMaxInterval)
	policy.SetExpirationInterval(domainCacheExpirationInterval)

	return policy
}

// IsValidIDLength checks if id is valid according to its length
func IsValidIDLength(
	id string,
	scope metrics.Scope,
	warnLimit int,
	errorLimit int,
	metricsCounter metrics.MetricIdx,
	domainName string,
	logger log.Logger,
	idTypeViolationTag tag.Tag,
) bool {
	if len(id) > warnLimit {
		scope.IncCounter(metricsCounter)
		logger.Warn("ID length exceeds limit.",
			tag.WorkflowDomainName(domainName),
			tag.Name(id),
			idTypeViolationTag)
	}
	return len(id) <= errorLimit
}

// CheckDecisionResultLimit checks if decision result count exceeds limits.
func CheckDecisionResultLimit(
	actualSize int,
	limit int,
	scope metrics.Scope,
) error {
	scope.RecordTimer(metrics.DecisionResultCount, time.Duration(actualSize))
	if limit > 0 && actualSize > limit {
		return ErrDecisionResultCountTooLarge
	}
	return nil
}

// ToServiceTransientError converts an error to ServiceTransientError
func ToServiceTransientError(err error) error {
	if err == nil || IsServiceTransientError(err) {
		return err
	}
	return yarpcerrors.Newf(yarpcerrors.CodeUnavailable, err.Error())
}

// HistoryRetryFuncFrontendExceptions checks if an error should be retried
// in a call from frontend
func FrontendRetry(err error) bool {
	var sbErr *types.ServiceBusyError
	if errors.As(err, &sbErr) {
		// If the service busy error is due to workflow id rate limiting, proxy it to the caller
		return sbErr.Reason != constants.WorkflowIDRateLimitReason
	}
	return IsServiceTransientError(err)
}

// IsExpectedError checks if an error is expected to happen in normal operation of the system
func IsExpectedError(err error) bool {
	return IsServiceTransientError(err) ||
		IsEntityNotExistsError(err) ||
		errors.As(err, new(*types.WorkflowExecutionAlreadyCompletedError))
}

// IsServiceTransientError checks if the error is a transient error.
func IsServiceTransientError(err error) bool {

	var (
		typesInternalServiceError        *types.InternalServiceError
		typesServiceBusyError            *types.ServiceBusyError
		typesShardOwnershipLostError     *types.ShardOwnershipLostError
		typesTaskListNotOwnedByHostError *cadence_errors.TaskListNotOwnedByHostError
		yarpcErrorsStatus                *yarpcerrors.Status
	)

	switch {
	case errors.As(err, &typesInternalServiceError):
		return true
	case errors.As(err, &typesServiceBusyError):
		return true
	case errors.As(err, &typesShardOwnershipLostError):
		return true
	case errors.As(err, &typesTaskListNotOwnedByHostError):
		return true
	case errors.As(err, &yarpcErrorsStatus):
		// We only selectively retry the following yarpc errors client can safe retry with a backoff
		if yarpcerrors.IsUnavailable(err) ||
			yarpcerrors.IsUnknown(err) ||
			yarpcerrors.IsCancelled(err) ||
			yarpcerrors.IsInternal(err) {
			return true
		}
		return false
	}

	return false
}

// IsEntityNotExistsError checks if the error is an entity not exists error.
func IsEntityNotExistsError(err error) bool {
	_, ok := err.(*types.EntityNotExistsError)
	return ok
}

// IsServiceBusyError checks if the error is a service busy error.
func IsServiceBusyError(err error) bool {
	switch err.(type) {
	case *types.ServiceBusyError:
		return true
	}
	return false
}

// IsContextTimeoutError checks if the error is context timeout error
func IsContextTimeoutError(err error) bool {
	switch err := err.(type) {
	case *types.InternalServiceError:
		return err.Message == context.DeadlineExceeded.Error()
	}
	return err == context.DeadlineExceeded || yarpcerrors.IsDeadlineExceeded(err)
}

// WorkflowIDToHistoryShard is used to map a workflowID to a shardID
func WorkflowIDToHistoryShard(workflowID string, numberOfShards int) int {
	hash := farm.Fingerprint32([]byte(workflowID))
	return int(hash % uint32(numberOfShards))
}

// DomainIDToHistoryShard is used to map a domainID to a shardID
func DomainIDToHistoryShard(domainID string, numberOfShards int) int {
	hash := farm.Fingerprint32([]byte(domainID))
	return int(hash % uint32(numberOfShards))
}

// IsValidContext checks that the thrift context is not expired on cancelled.
// Returns nil if the context is still valid. Otherwise, returns the result of
// ctx.Err()
func IsValidContext(ctx context.Context) error {
	ch := ctx.Done()
	if ch != nil {
		select {
		case <-ch:
			return ctx.Err()
		default:
			// go to the next line
		}
	}

	deadline, ok := ctx.Deadline()
	if ok && time.Until(deadline) < contextExpireThreshold {
		return context.DeadlineExceeded
	}
	return nil
}

// emptyCancelFunc wraps an empty func by context.CancelFunc interface
var emptyCancelFunc = context.CancelFunc(func() {})

// CreateChildContext creates a child context which shorted context timeout
// from the given parent context
// tailroom must be in range [0, 1] and
// (1-tailroom) * parent timeout will be the new child context timeout
// if tailroom is less 0, tailroom will be considered as 0
// if tailroom is greater than 1, tailroom wil be considered as 1
func CreateChildContext(
	parent context.Context,
	tailroom float64,
) (context.Context, context.CancelFunc) {
	if parent == nil {
		return nil, emptyCancelFunc
	}
	if parent.Err() != nil {
		return parent, emptyCancelFunc
	}

	now := time.Now()
	deadline, ok := parent.Deadline()
	if !ok || deadline.Before(now) {
		return parent, emptyCancelFunc
	}

	// if tailroom is about or less 0, then return a context with the same deadline as parent
	if tailroom <= 0 {
		return context.WithDeadline(parent, deadline)
	}
	// if tailroom is about or greater 1, then return a context with deadline of now
	if tailroom >= 1 {
		return context.WithDeadline(parent, now)
	}

	newDeadline := now.Add(time.Duration(math.Ceil(float64(deadline.Sub(now)) * (1.0 - tailroom))))
	return context.WithDeadline(parent, newDeadline)
}

// GenerateRandomString is used for generate test string
func GenerateRandomString(n int) string {
	if n <= 0 {
		return ""
	}

	letterRunes := []rune("random")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// CreateMatchingPollForDecisionTaskResponse create response for matching's PollForDecisionTask
func CreateMatchingPollForDecisionTaskResponse(historyResponse *types.RecordDecisionTaskStartedResponse, workflowExecution *types.WorkflowExecution, token []byte) *types.MatchingPollForDecisionTaskResponse {
	matchingResp := &types.MatchingPollForDecisionTaskResponse{
		WorkflowExecution:         workflowExecution,
		TaskToken:                 token,
		Attempt:                   historyResponse.GetAttempt(),
		WorkflowType:              historyResponse.WorkflowType,
		StartedEventID:            historyResponse.StartedEventID,
		StickyExecutionEnabled:    historyResponse.StickyExecutionEnabled,
		NextEventID:               historyResponse.NextEventID,
		DecisionInfo:              historyResponse.DecisionInfo,
		WorkflowExecutionTaskList: historyResponse.WorkflowExecutionTaskList,
		BranchToken:               historyResponse.BranchToken,
		ScheduledTimestamp:        historyResponse.ScheduledTimestamp,
		StartedTimestamp:          historyResponse.StartedTimestamp,
		Queries:                   historyResponse.Queries,
		TotalHistoryBytes:         historyResponse.HistorySize,
	}
	if historyResponse.GetPreviousStartedEventID() != constants.EmptyEventID {
		matchingResp.PreviousStartedEventID = historyResponse.PreviousStartedEventID
	}
	return matchingResp
}

// ValidateRetryPolicy validates a retry policy
func ValidateRetryPolicy(policy *types.RetryPolicy) error {
	if policy == nil {
		// nil policy is valid which means no retry
		return nil
	}
	if policy.GetInitialIntervalInSeconds() <= 0 {
		return &types.BadRequestError{Message: "InitialIntervalInSeconds must be greater than 0 on retry policy."}
	}
	if policy.GetBackoffCoefficient() < 1 {
		return &types.BadRequestError{Message: "BackoffCoefficient cannot be less than 1 on retry policy."}
	}
	if policy.GetMaximumIntervalInSeconds() < 0 {
		return &types.BadRequestError{Message: "MaximumIntervalInSeconds cannot be less than 0 on retry policy."}
	}
	if policy.GetMaximumIntervalInSeconds() > 0 && policy.GetMaximumIntervalInSeconds() < policy.GetInitialIntervalInSeconds() {
		return &types.BadRequestError{Message: "MaximumIntervalInSeconds cannot be less than InitialIntervalInSeconds on retry policy."}
	}
	if policy.GetMaximumAttempts() < 0 {
		return &types.BadRequestError{Message: "MaximumAttempts cannot be less than 0 on retry policy."}
	}
	if policy.GetExpirationIntervalInSeconds() < 0 {
		return &types.BadRequestError{Message: "ExpirationIntervalInSeconds cannot be less than 0 on retry policy."}
	}
	if policy.GetMaximumAttempts() == 0 && policy.GetExpirationIntervalInSeconds() == 0 {
		return &types.BadRequestError{Message: "MaximumAttempts and ExpirationIntervalInSeconds are both 0. At least one of them must be specified."}
	}
	return nil
}

// CreateHistoryStartWorkflowRequest create a start workflow request for history
func CreateHistoryStartWorkflowRequest(
	domainID string,
	startRequest *types.StartWorkflowExecutionRequest,
	now time.Time,
	partitionConfig map[string]string,
) (*types.HistoryStartWorkflowExecutionRequest, error) {
	histRequest := &types.HistoryStartWorkflowExecutionRequest{
		DomainUUID:      domainID,
		StartRequest:    startRequest,
		PartitionConfig: partitionConfig,
	}

	delayStartSeconds := startRequest.GetDelayStartSeconds()
	jitterStartSeconds := startRequest.GetJitterStartSeconds()
	firstRunAtTimestamp := startRequest.GetFirstRunAtTimeStamp()

	firstDecisionTaskBackoffSeconds := delayStartSeconds

	// if the user specified a timestamp for the first run, we will use that as the start time,
	// ignoring the delayStartSeconds, jitterStartSeconds, and cronSchedule
	// The following condition guarantees two things:
	// - The logic is only triggered when the user specifies a first run timestamp
	// - AND that timestamp is only triggered ONCE hence not interfering with other scheduling logic
	if firstRunAtTimestamp > now.UnixNano() {
		firstDecisionTaskBackoffSeconds = int32((firstRunAtTimestamp - now.UnixNano()) / int64(time.Second))
	} else {
		if len(startRequest.GetCronSchedule()) > 0 {
			delayedStartTime := now.Add(time.Second * time.Duration(delayStartSeconds))
			var err error
			firstDecisionTaskBackoffSeconds, err = backoff.GetBackoffForNextScheduleInSeconds(
				startRequest.GetCronSchedule(), delayedStartTime, delayedStartTime, jitterStartSeconds, startRequest.GetCronOverlapPolicy())
			if err != nil {
				return nil, err
			}

			// backoff seconds was calculated based on delayed start time, so we need to
			// add the delayStartSeconds to that backoff.
			firstDecisionTaskBackoffSeconds += delayStartSeconds
		} else if jitterStartSeconds > 0 {
			// Add a random jitter to start time, if requested.
			firstDecisionTaskBackoffSeconds += rand.Int31n(jitterStartSeconds + 1)
		}
	}

	histRequest.FirstDecisionTaskBackoffSeconds = Int32Ptr(firstDecisionTaskBackoffSeconds)

	if startRequest.RetryPolicy != nil && startRequest.RetryPolicy.GetExpirationIntervalInSeconds() > 0 {
		expirationInSeconds := startRequest.RetryPolicy.GetExpirationIntervalInSeconds() + firstDecisionTaskBackoffSeconds
		// expirationTime calculates from first decision task schedule to the end of the workflow
		deadline := now.Add(time.Duration(expirationInSeconds) * time.Second)
		histRequest.ExpirationTimestamp = Int64Ptr(deadline.Round(time.Millisecond).UnixNano())
	}

	return histRequest, nil
}

// CheckEventBlobSizeLimit checks if a blob data exceeds limits. It logs a warning if it exceeds warnLimit,
// and return ErrBlobSizeExceedsLimit if it exceeds errorLimit.
func CheckEventBlobSizeLimit(
	actualSize int,
	warnLimit int,
	errorLimit int,
	domainID string,
	domainName string,
	workflowID string,
	runID string,
	scope metrics.Scope,
	logger log.Logger,
	blobSizeViolationOperationTag tag.Tag,
) error {

	scope.RecordTimer(metrics.EventBlobSize, time.Duration(actualSize))

	if errorLimit < warnLimit {
		logger.Warn("Error limit is less than warn limit.", tag.WorkflowDomainName(domainName), tag.WorkflowDomainID(domainID))

		warnLimit = errorLimit
	}

	if actualSize <= warnLimit {
		return nil
	}

	tags := []tag.Tag{
		tag.WorkflowDomainName(domainName),
		tag.WorkflowDomainID(domainID),
		tag.WorkflowID(workflowID),
		tag.WorkflowRunID(runID),
		tag.WorkflowSize(int64(actualSize)),
		blobSizeViolationOperationTag,
	}

	if actualSize <= errorLimit {
		logger.Warn("Blob size close to the limit.", tags...)

		return nil
	}

	scope.Tagged(metrics.DomainTag(domainName)).IncCounter(metrics.EventBlobSizeExceedLimit)

	logger.Error("Blob size exceeds limit.", tags...)

	return ErrBlobSizeExceedsLimit
}

// ValidateLongPollContextTimeout check if the context timeout for a long poll handler is too short or below a normal value.
// If the timeout is not set or too short, it logs an error, and return ErrContextTimeoutNotSet or ErrContextTimeoutTooShort
// accordingly. If the timeout is only below a normal value, it just logs an info and return nil.
func ValidateLongPollContextTimeout(
	ctx context.Context,
	handlerName string,
	logger log.Logger,
) error {

	deadline, err := ValidateLongPollContextTimeoutIsSet(ctx, handlerName, logger)
	if err != nil {
		return err
	}
	timeout := time.Until(deadline)
	if timeout < constants.MinLongPollTimeout {
		err := ErrContextTimeoutTooShort
		logger.Error("Context timeout is too short for long poll API.",
			tag.WorkflowHandlerName(handlerName), tag.Error(err), tag.WorkflowPollContextTimeout(timeout))
		return err
	}
	if timeout < constants.CriticalLongPollTimeout {
		logger.Debug("Context timeout is lower than critical value for long poll API.",
			tag.WorkflowHandlerName(handlerName), tag.WorkflowPollContextTimeout(timeout))
	}
	return nil
}

// ValidateLongPollContextTimeoutIsSet checks if the context timeout is set for long poll requests.
func ValidateLongPollContextTimeoutIsSet(
	ctx context.Context,
	handlerName string,
	logger log.Logger,
) (time.Time, error) {

	deadline, ok := ctx.Deadline()
	if !ok {
		err := ErrContextTimeoutNotSet
		logger.Error("Context timeout not set for long poll API.",
			tag.WorkflowHandlerName(handlerName), tag.Error(err))
		return deadline, err
	}
	return deadline, nil
}

// ValidateDomainUUID checks if the given domainID string is a valid UUID
func ValidateDomainUUID(
	domainUUID string,
) error {

	if domainUUID == "" {
		return &types.BadRequestError{Message: "Missing domain UUID."}
	} else if uuid.Parse(domainUUID) == nil {
		return &types.BadRequestError{Message: "Invalid domain UUID."}
	}
	return nil
}

// GetSizeOfMapStringToByteArray get size of map[string][]byte
func GetSizeOfMapStringToByteArray(input map[string][]byte) int {
	if input == nil {
		return 0
	}

	res := 0
	for k, v := range input {
		res += len(k) + len(v)
	}
	return res + golandMapReserverNumberOfBytes
}

// GetSizeOfHistoryEvent returns approximate size in bytes of the history event taking into account byte arrays only now
func GetSizeOfHistoryEvent(event *types.HistoryEvent) uint64 {
	if event == nil || event.EventType == nil {
		return 0
	}

	res := 0
	switch *event.EventType {
	case types.EventTypeWorkflowExecutionStarted:
		res += len(event.WorkflowExecutionStartedEventAttributes.Input)
		res += len(event.WorkflowExecutionStartedEventAttributes.ContinuedFailureDetails)
		res += len(event.WorkflowExecutionStartedEventAttributes.LastCompletionResult)
		if event.WorkflowExecutionStartedEventAttributes.Memo != nil {
			res += GetSizeOfMapStringToByteArray(event.WorkflowExecutionStartedEventAttributes.Memo.Fields)
		}
		if event.WorkflowExecutionStartedEventAttributes.Header != nil {
			res += GetSizeOfMapStringToByteArray(event.WorkflowExecutionStartedEventAttributes.Header.Fields)
		}
		if event.WorkflowExecutionStartedEventAttributes.SearchAttributes != nil {
			res += GetSizeOfMapStringToByteArray(event.WorkflowExecutionStartedEventAttributes.SearchAttributes.IndexedFields)
		}
	case types.EventTypeWorkflowExecutionCompleted:
		res += len(event.WorkflowExecutionCompletedEventAttributes.Result)
	case types.EventTypeWorkflowExecutionFailed:
		res += len(event.WorkflowExecutionFailedEventAttributes.Details)
	case types.EventTypeWorkflowExecutionTimedOut:
	case types.EventTypeDecisionTaskScheduled:
	case types.EventTypeDecisionTaskStarted:
	case types.EventTypeDecisionTaskCompleted:
		res += len(event.DecisionTaskCompletedEventAttributes.ExecutionContext)
	case types.EventTypeDecisionTaskTimedOut:
	case types.EventTypeDecisionTaskFailed:
		res += len(event.DecisionTaskFailedEventAttributes.Details)
	case types.EventTypeActivityTaskScheduled:
		res += len(event.ActivityTaskScheduledEventAttributes.Input)
		if event.ActivityTaskScheduledEventAttributes.Header != nil {
			res += GetSizeOfMapStringToByteArray(event.ActivityTaskScheduledEventAttributes.Header.Fields)
		}
	case types.EventTypeActivityTaskStarted:
		res += len(event.ActivityTaskStartedEventAttributes.LastFailureDetails)
	case types.EventTypeActivityTaskCompleted:
		res += len(event.ActivityTaskCompletedEventAttributes.Result)
	case types.EventTypeActivityTaskFailed:
		res += len(event.ActivityTaskFailedEventAttributes.Details)
	case types.EventTypeActivityTaskTimedOut:
		res += len(event.ActivityTaskTimedOutEventAttributes.Details)
		res += len(event.ActivityTaskTimedOutEventAttributes.LastFailureDetails)
	case types.EventTypeActivityTaskCancelRequested:
	case types.EventTypeRequestCancelActivityTaskFailed:
	case types.EventTypeActivityTaskCanceled:
		res += len(event.ActivityTaskCanceledEventAttributes.Details)
	case types.EventTypeTimerStarted:
	case types.EventTypeTimerFired:
	case types.EventTypeCancelTimerFailed:
	case types.EventTypeTimerCanceled:
	case types.EventTypeWorkflowExecutionCancelRequested:
	case types.EventTypeWorkflowExecutionCanceled:
		res += len(event.WorkflowExecutionCanceledEventAttributes.Details)
	case types.EventTypeRequestCancelExternalWorkflowExecutionInitiated:
		res += len(event.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes.Control)
	case types.EventTypeRequestCancelExternalWorkflowExecutionFailed:
		res += len(event.RequestCancelExternalWorkflowExecutionFailedEventAttributes.Control)
	case types.EventTypeExternalWorkflowExecutionCancelRequested:
	case types.EventTypeMarkerRecorded:
		res += len(event.MarkerRecordedEventAttributes.Details)
	case types.EventTypeWorkflowExecutionSignaled:
		res += len(event.WorkflowExecutionSignaledEventAttributes.Input)
	case types.EventTypeWorkflowExecutionTerminated:
		res += len(event.WorkflowExecutionTerminatedEventAttributes.Details)
	case types.EventTypeWorkflowExecutionContinuedAsNew:
		res += len(event.WorkflowExecutionContinuedAsNewEventAttributes.Input)
		if event.WorkflowExecutionContinuedAsNewEventAttributes.Memo != nil {
			res += GetSizeOfMapStringToByteArray(event.WorkflowExecutionContinuedAsNewEventAttributes.Memo.Fields)
		}
		if event.WorkflowExecutionContinuedAsNewEventAttributes.Header != nil {
			res += GetSizeOfMapStringToByteArray(event.WorkflowExecutionContinuedAsNewEventAttributes.Header.Fields)
		}
		if event.WorkflowExecutionContinuedAsNewEventAttributes.SearchAttributes != nil {
			res += GetSizeOfMapStringToByteArray(event.WorkflowExecutionContinuedAsNewEventAttributes.SearchAttributes.IndexedFields)
		}
	case types.EventTypeStartChildWorkflowExecutionInitiated:
		res += len(event.StartChildWorkflowExecutionInitiatedEventAttributes.Input)
		res += len(event.StartChildWorkflowExecutionInitiatedEventAttributes.Control)
		if event.StartChildWorkflowExecutionInitiatedEventAttributes.Memo != nil {
			res += GetSizeOfMapStringToByteArray(event.StartChildWorkflowExecutionInitiatedEventAttributes.Memo.Fields)
		}
		if event.StartChildWorkflowExecutionInitiatedEventAttributes.Header != nil {
			res += GetSizeOfMapStringToByteArray(event.StartChildWorkflowExecutionInitiatedEventAttributes.Header.Fields)
		}
		if event.StartChildWorkflowExecutionInitiatedEventAttributes.SearchAttributes != nil {
			res += GetSizeOfMapStringToByteArray(event.StartChildWorkflowExecutionInitiatedEventAttributes.SearchAttributes.IndexedFields)
		}
	case types.EventTypeStartChildWorkflowExecutionFailed:
		res += len(event.StartChildWorkflowExecutionFailedEventAttributes.Control)
	case types.EventTypeChildWorkflowExecutionStarted:
		if event.ChildWorkflowExecutionStartedEventAttributes != nil && event.ChildWorkflowExecutionStartedEventAttributes.Header != nil {
			res += GetSizeOfMapStringToByteArray(event.ChildWorkflowExecutionStartedEventAttributes.Header.Fields)
		}
	case types.EventTypeChildWorkflowExecutionCompleted:
		res += len(event.ChildWorkflowExecutionCompletedEventAttributes.Result)
	case types.EventTypeChildWorkflowExecutionFailed:
		res += len(event.ChildWorkflowExecutionFailedEventAttributes.Details)
	case types.EventTypeChildWorkflowExecutionCanceled:
		res += len(event.ChildWorkflowExecutionCanceledEventAttributes.Details)
	case types.EventTypeChildWorkflowExecutionTimedOut:
	case types.EventTypeChildWorkflowExecutionTerminated:
	case types.EventTypeSignalExternalWorkflowExecutionInitiated:
		res += len(event.SignalExternalWorkflowExecutionInitiatedEventAttributes.Input)
		res += len(event.SignalExternalWorkflowExecutionInitiatedEventAttributes.Control)
	case types.EventTypeSignalExternalWorkflowExecutionFailed:
		res += len(event.SignalExternalWorkflowExecutionFailedEventAttributes.Control)
	case types.EventTypeExternalWorkflowExecutionSignaled:
		res += len(event.ExternalWorkflowExecutionSignaledEventAttributes.Control)
	case types.EventTypeUpsertWorkflowSearchAttributes:
		if event.UpsertWorkflowSearchAttributesEventAttributes.SearchAttributes != nil {
			res += GetSizeOfMapStringToByteArray(event.UpsertWorkflowSearchAttributesEventAttributes.SearchAttributes.IndexedFields)
		}
	}
	return uint64(res)
}

// IsJustOrderByClause return true is query start with order by
func IsJustOrderByClause(clause string) bool {
	whereClause := strings.TrimSpace(clause)
	whereClause = strings.ToLower(whereClause)
	return strings.HasPrefix(whereClause, "order by")
}

// ConvertIndexedValueTypeToInternalType takes fieldType as interface{} and convert to IndexedValueType.
// Because different implementation of dynamic config client may lead to different types
func ConvertIndexedValueTypeToInternalType(fieldType interface{}, logger log.Logger) types.IndexedValueType {
	switch t := fieldType.(type) {
	case float64:
		return types.IndexedValueType(t)
	case int:
		return types.IndexedValueType(t)
	case types.IndexedValueType:
		return t
	case []byte:
		var result types.IndexedValueType
		if err := result.UnmarshalText(t); err != nil {
			logger.Error("unknown index value type", tag.Value(fieldType), tag.ValueType(t), tag.Error(err))
			return fieldType.(types.IndexedValueType) // it will panic and been captured by logger
		}
		return result
	case string:
		var result types.IndexedValueType
		if err := result.UnmarshalText([]byte(t)); err != nil {
			logger.Error("unknown index value type", tag.Value(fieldType), tag.ValueType(t), tag.Error(err))
			return fieldType.(types.IndexedValueType) // it will panic and been captured by logger
		}
		return result
	default:
		// Unknown fieldType, please make sure dynamic config return correct value type
		logger.Error("unknown index value type", tag.Value(fieldType), tag.ValueType(t))
		return fieldType.(types.IndexedValueType) // it will panic and been captured by logger
	}
}

// DeserializeSearchAttributeValue takes json encoded search attribute value and it's type as input, then
// unmarshal the value into a concrete type and return the value
func DeserializeSearchAttributeValue(value []byte, valueType types.IndexedValueType) (interface{}, error) {
	switch valueType {
	case types.IndexedValueTypeString, types.IndexedValueTypeKeyword:
		var val string
		if err := json.Unmarshal(value, &val); err != nil {
			var listVal []string
			err = json.Unmarshal(value, &listVal)
			return listVal, err
		}
		return val, nil
	case types.IndexedValueTypeInt:
		var val int64
		if err := json.Unmarshal(value, &val); err != nil {
			var listVal []int64
			err = json.Unmarshal(value, &listVal)
			return listVal, err
		}
		return val, nil
	case types.IndexedValueTypeDouble:
		var val float64
		if err := json.Unmarshal(value, &val); err != nil {
			var listVal []float64
			err = json.Unmarshal(value, &listVal)
			return listVal, err
		}
		return val, nil
	case types.IndexedValueTypeBool:
		var val bool
		if err := json.Unmarshal(value, &val); err != nil {
			var listVal []bool
			err = json.Unmarshal(value, &listVal)
			return listVal, err
		}
		return val, nil
	case types.IndexedValueTypeDatetime:
		var val time.Time
		if err := json.Unmarshal(value, &val); err != nil {
			var listVal []time.Time
			err = json.Unmarshal(value, &listVal)
			return listVal, err
		}
		return val, nil
	default:
		return nil, fmt.Errorf("error: unknown index value type [%v]", valueType)
	}
}

// IsAdvancedVisibilityWritingEnabled returns true if we should write to advanced visibility
func IsAdvancedVisibilityWritingEnabled(advancedVisibilityWritingMode string, isAdvancedVisConfigExist bool) bool {
	return advancedVisibilityWritingMode != constants.AdvancedVisibilityModeOff && isAdvancedVisConfigExist
}

// IsAdvancedVisibilityReadingEnabled returns true if we should read from advanced visibility
func IsAdvancedVisibilityReadingEnabled(isAdvancedVisReadEnabled, isAdvancedVisConfigExist bool) bool {
	return isAdvancedVisReadEnabled && isAdvancedVisConfigExist
}

// IsStickyTaskConditionError is error from matching engine
func IsStickyTaskConditionError(err error) bool {
	if e, ok := err.(*types.InternalServiceError); ok {
		return e.GetMessage() == constants.StickyTaskConditionFailedErrorMsg
	}
	return false
}

// DurationToDays converts time.Duration to number of 24 hour days
func DurationToDays(d time.Duration) int32 {
	return int32(d / (24 * time.Hour))
}

// DurationToSeconds converts time.Duration to number of seconds
func DurationToSeconds(d time.Duration) int64 {
	return int64(d / time.Second)
}

// DaysToDuration converts number of 24 hour days to time.Duration
func DaysToDuration(d int32) time.Duration {
	return time.Duration(d) * (24 * time.Hour)
}

// SecondsToDuration converts number of seconds to time.Duration
func SecondsToDuration(d int64) time.Duration {
	return time.Duration(d) * time.Second
}

// SleepWithMinDuration sleeps for the minimum of desired and available duration
// returns the remaining available time duration
func SleepWithMinDuration(desired time.Duration, available time.Duration) time.Duration {
	d := min(desired, available)
	if d > 0 {
		time.Sleep(d)
	}
	return available - d
}

// ConvertErrToGetTaskFailedCause converts error to GetTaskFailedCause
func ConvertErrToGetTaskFailedCause(err error) types.GetTaskFailedCause {
	if IsContextTimeoutError(err) {
		return types.GetTaskFailedCauseTimeout
	}
	if IsServiceBusyError(err) {
		return types.GetTaskFailedCauseServiceBusy
	}
	if _, ok := err.(*types.ShardOwnershipLostError); ok {
		return types.GetTaskFailedCauseShardOwnershipLost
	}
	return types.GetTaskFailedCauseUncategorized
}

// ConvertGetTaskFailedCauseToErr converts GetTaskFailedCause to error
func ConvertGetTaskFailedCauseToErr(failedCause types.GetTaskFailedCause) error {
	switch failedCause {
	case types.GetTaskFailedCauseServiceBusy:
		return &types.ServiceBusyError{}
	case types.GetTaskFailedCauseTimeout:
		return context.DeadlineExceeded
	case types.GetTaskFailedCauseShardOwnershipLost:
		return &types.ShardOwnershipLostError{}
	default:
		return &types.InternalServiceError{Message: "uncategorized error"}
	}
}

// IntersectionStringSlice get the intersection of 2 string slices
func IntersectionStringSlice(a, b []string) []string {
	var result []string
	m := make(map[string]struct{})
	for _, item := range a {
		m[item] = struct{}{}
	}
	for _, item := range b {
		if _, ok := m[item]; ok {
			result = append(result, item)
		}
	}
	return result
}

// NewPerTaskListScope creates a tasklist metrics scope
func NewPerTaskListScope(
	domainName string,
	taskListName string,
	taskListKind types.TaskListKind,
	client metrics.Client,
	scopeIdx metrics.ScopeIdx,
) metrics.Scope {
	domainTag := metrics.DomainUnknownTag()
	taskListTag := metrics.TaskListUnknownTag()
	if domainName != "" {
		domainTag = metrics.DomainTag(domainName)
	}
	if taskListName != "" && taskListKind == types.TaskListKindNormal {
		taskListTag = metrics.TaskListTag(taskListName)
	}
	if taskListKind == types.TaskListKindSticky {
		taskListTag = stickyTaskListMetricTag
	}
	if taskListKind == types.TaskListKindEphemeral {
		taskListTag = ephemeralTaskListMetricTag
	}
	return client.Scope(scopeIdx, domainTag, taskListTag)
}
