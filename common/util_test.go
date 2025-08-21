// The MIT License (MIT)
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package common

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/yarpc/yarpcerrors"
	"golang.org/x/exp/maps"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func TestIsServiceTransientError(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want bool
	}{
		"ContextTimeout": {
			err:  context.DeadlineExceeded,
			want: false,
		},
		"YARPCDeadlineExceeded": {
			err:  yarpcerrors.DeadlineExceededErrorf("yarpc deadline exceeded"),
			want: false,
		},
		"YARPCUnavailable": {
			err:  yarpcerrors.UnavailableErrorf("yarpc unavailable"),
			want: true,
		},
		"YARPCUnavailable wrapped": {
			err:  fmt.Errorf("wrapped err: %w", yarpcerrors.UnavailableErrorf("yarpc unavailable")),
			want: true,
		},
		"YARPCUnknown": {
			err:  yarpcerrors.UnknownErrorf("yarpc unknown"),
			want: true,
		},
		"YARPCInternal": {
			err:  yarpcerrors.InternalErrorf("yarpc internal"),
			want: true,
		},
		"ContextCancel": {
			err:  context.Canceled,
			want: false,
		},
		"ServiceBusyError": {
			err:  &types.ServiceBusyError{},
			want: true,
		},
		"ShardOwnershipLostError": {
			err:  &types.ShardOwnershipLostError{},
			want: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.want, IsServiceTransientError(c.err))
		})
	}

}

func TestIsExpectedError(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want bool
	}{
		"An error": {
			err:  assert.AnError,
			want: false,
		},
		"Transient error": {
			err:  &types.ServiceBusyError{},
			want: true,
		},
		"Entity not exists error": {
			err:  &types.EntityNotExistsError{},
			want: true,
		},
		"Already completed error": {
			err:  &types.WorkflowExecutionAlreadyCompletedError{},
			want: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.want, IsExpectedError(c.err))
		})
	}
}

func TestFrontendRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ServiceBusyError due to workflow id rate limiting",
			err:  &types.ServiceBusyError{Reason: constants.WorkflowIDRateLimitReason},
			want: false,
		},
		{
			name: "ServiceBusyError not due to workflow id rate limiting",
			err:  &types.ServiceBusyError{Reason: "some other reason"},
			want: true,
		},
		{
			name: "ServiceBusyError empty reason",
			err:  &types.ServiceBusyError{Reason: ""},
			want: true,
		},
		{
			name: "Non-ServiceBusyError",
			err:  errors.New("some random error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, FrontendRetry(tt.err))
		})
	}
}

func TestIsContextTimeoutError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	time.Sleep(50 * time.Millisecond)
	require.True(t, IsContextTimeoutError(ctx.Err()))
	require.True(t, IsContextTimeoutError(&types.InternalServiceError{Message: ctx.Err().Error()}))

	yarpcErr := yarpcerrors.DeadlineExceededErrorf("yarpc deadline exceeded")
	require.True(t, IsContextTimeoutError(yarpcErr))

	require.False(t, IsContextTimeoutError(errors.New("some random error")))

	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	require.False(t, IsContextTimeoutError(ctx.Err()))
}

func TestCreateHistoryStartWorkflowRequest_ExpirationTimeWithCron(t *testing.T) {
	domainID := uuid.New()
	request := &types.StartWorkflowExecutionRequest{
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    60,
			ExpirationIntervalInSeconds: 60,
		},
		CronSchedule: "@every 300s",
	}
	now := time.Now()
	startRequest, err := CreateHistoryStartWorkflowRequest(domainID, request, now, nil)
	require.NoError(t, err)
	require.NotNil(t, startRequest)

	expirationTime := startRequest.GetExpirationTimestamp()
	require.NotNil(t, expirationTime)
	require.True(t, time.Unix(0, expirationTime).Sub(now) > 60*time.Second)
}

// Test to ensure we get the right value for FirstDecisionTaskBackoff during StartWorkflow request,
// with & without cron, delayStart and jitterStart.
// - Also see tests in cron_test.go for more exhaustive testing.
func TestFirstDecisionTaskBackoffDuringStartWorkflow(t *testing.T) {
	firstRunTimestampPast, _ := time.Parse(time.RFC3339, "2017-12-17T08:00:00+00:00")
	firstRunTimestampFuture, _ := time.Parse(time.RFC3339, "2018-12-17T08:00:02+00:00")
	firstRunAtTimestampMap := map[string]int64{
		"null":   0,
		"past":   firstRunTimestampPast.UnixNano(),
		"future": firstRunTimestampFuture.UnixNano(),
	}

	var tests = []struct {
		cron                   bool
		jitterStartSeconds     int32
		delayStartSeconds      int32
		firstRunAtTimeCategory string
	}{
		// Null first run at cases
		{true, 0, 0, "null"},
		{true, 15, 0, "null"},
		{true, 0, 600, "null"},
		{true, 15, 600, "null"},
		{false, 0, 0, "null"},
		{false, 15, 0, "null"},
		{false, 0, 600, "null"},
		{false, 15, 600, "null"},

		// Past first run at cases
		{true, 0, 0, "past"},
		{true, 15, 0, "past"},
		{true, 0, 600, "past"},
		{true, 15, 600, "past"},
		{false, 0, 0, "past"},
		{false, 15, 0, "past"},
		{false, 0, 600, "past"},
		{false, 15, 600, "past"},

		// Future first run at cases
		{true, 0, 0, "future"},
		{true, 15, 0, "future"},
		{true, 0, 600, "future"},
		{true, 15, 600, "future"},
		{false, 0, 0, "future"},
		{false, 15, 0, "future"},
		{false, 0, 600, "future"},
		{false, 15, 600, "future"},
	}

	rand.Seed(int64(time.Now().Nanosecond()))

	for idx, tt := range tests {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			domainID := uuid.New()
			request := &types.StartWorkflowExecutionRequest{
				FirstRunAtTimeStamp: Int64Ptr(firstRunAtTimestampMap[tt.firstRunAtTimeCategory]),
				DelayStartSeconds:   Int32Ptr(tt.delayStartSeconds),
				JitterStartSeconds:  Int32Ptr(tt.jitterStartSeconds),
			}
			if tt.cron {
				request.CronSchedule = "* * * * *"
			}

			// Run X loops so that the test isn't flaky, since jitter adds randomness.
			caseCount := 10
			exactCount := 0
			for i := 0; i < caseCount; i++ {
				// Start at the minute boundary so we know what the backoff should be
				startTime, _ := time.Parse(time.RFC3339, "2018-12-17T08:00:00+00:00")
				startRequest, err := CreateHistoryStartWorkflowRequest(domainID, request, startTime, nil)
				require.NoError(t, err)
				require.NotNil(t, startRequest)

				backoff := startRequest.GetFirstDecisionTaskBackoffSeconds()

				expectedWithoutJitter := tt.delayStartSeconds
				if tt.cron {
					expectedWithoutJitter += 60
				}
				expectedMax := expectedWithoutJitter + tt.jitterStartSeconds

				if backoff == expectedWithoutJitter {
					exactCount++
				}

				if tt.firstRunAtTimeCategory == "future" {
					require.Equal(t, int32(2), backoff, "test specs = %v", tt)
				} else {
					if tt.jitterStartSeconds == 0 {
						require.Equal(t, expectedWithoutJitter, backoff, "test specs = %v", tt)
					} else {
						require.True(t, backoff >= expectedWithoutJitter && backoff <= expectedMax,
							"test specs = %v, backoff (%v) should be >= %v and <= %v",
							tt, backoff, expectedWithoutJitter, expectedMax)
					}
				}
			}

			// If firstRunTimestamp is either null or past ONLY
			// If jitter is > 0, we want to detect whether jitter is being applied - BUT we don't want the test
			// to be flaky if the code randomly chooses a jitter of 0, so we try to have enough data points by
			// checking the next X cron times AND by choosing a jitter thats not super low.
			if tt.firstRunAtTimeCategory != "future" {
				if tt.jitterStartSeconds > 0 && exactCount == caseCount {
					// Test to make sure a jitter test case sometimes doesn't get exact values
					t.Fatalf("FAILED to jitter properly? Test specs = %v\n", tt)
				} else if tt.jitterStartSeconds == 0 && exactCount != caseCount {
					// Test to make sure a non-jitter test case always gets exact values
					t.Fatalf("Jittered when we weren't supposed to? Test specs = %v\n", tt)
				}
			}
		})
	}
}

func TestCreateHistoryStartWorkflowRequest(t *testing.T) {
	var tests = []struct {
		delayStartSeconds  int
		cronSeconds        int
		jitterStartSeconds int
	}{
		{0, 0, 0},
		{100, 0, 0},
		{100, 300, 0},
		{0, 0, 2000},
		{100, 0, 2000},
		{0, 300, 2000},
		{100, 300, 2000},
		{0, 300, 2000},
		{0, 300, 2000},
	}

	for idx, tt := range tests {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			testExpirationTime(t, tt.delayStartSeconds, tt.cronSeconds, tt.jitterStartSeconds)
		})
	}
}

func testExpirationTime(t *testing.T, delayStartSeconds int, cronSeconds int, jitterSeconds int) {
	domainID := uuid.New()
	request := &types.StartWorkflowExecutionRequest{
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    60,
			ExpirationIntervalInSeconds: 60,
		},
		DelayStartSeconds:  Int32Ptr(int32(delayStartSeconds)),
		JitterStartSeconds: Int32Ptr(int32(jitterSeconds)),
	}
	if cronSeconds > 0 {
		request.CronSchedule = fmt.Sprintf("@every %ds", cronSeconds)
	}
	minDelay := delayStartSeconds + cronSeconds
	maxDelay := delayStartSeconds + 2*cronSeconds + jitterSeconds
	now := time.Now()
	startRequest, err := CreateHistoryStartWorkflowRequest(domainID, request, now, nil)
	require.NoError(t, err)
	require.NotNil(t, startRequest)

	expirationTime := startRequest.GetExpirationTimestamp()
	require.NotNil(t, expirationTime)

	// Since we assign the expiration time after we create the workflow request,
	// There's a chance that the test thread might sleep or get deprioritized and
	// expirationTime - now may not be equal to DelayStartSeconds. Adding 2 seconds
	// buffer to avoid this test being flaky
	require.True(
		t,
		time.Unix(0, expirationTime).Sub(now) >= (time.Duration(minDelay)+58)*time.Second,
		"Integration test took too short: %f seconds vs %f seconds",
		time.Duration(time.Unix(0, expirationTime).Sub(now)).Round(time.Millisecond).Seconds(),
		time.Duration((time.Duration(minDelay)+58)*time.Second).Round(time.Millisecond).Seconds(),
	)
	require.True(
		t,
		time.Unix(0, expirationTime).Sub(now) < (time.Duration(maxDelay)+68)*time.Second,
		"Integration test took too long: %f seconds vs %f seconds",
		time.Duration(time.Unix(0, expirationTime).Sub(now)).Round(time.Millisecond).Seconds(),
		time.Duration((time.Duration(minDelay)+68)*time.Second).Round(time.Millisecond).Seconds(),
	)
}

func TestCreateHistoryStartWorkflowRequest_ExpirationTimeWithoutCron(t *testing.T) {
	domainID := uuid.New()
	request := &types.StartWorkflowExecutionRequest{
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    60,
			ExpirationIntervalInSeconds: 60,
		},
	}
	now := time.Now()
	startRequest, err := CreateHistoryStartWorkflowRequest(domainID, request, now, nil)
	require.NoError(t, err)
	require.NotNil(t, startRequest)

	expirationTime := startRequest.GetExpirationTimestamp()
	require.NotNil(t, expirationTime)
	delta := time.Unix(0, expirationTime).Sub(now)
	require.True(t, delta > 58*time.Second)
	require.True(t, delta < 62*time.Second)
}

func TestConvertIndexedValueTypeToInternalType(t *testing.T) {
	values := []types.IndexedValueType{types.IndexedValueTypeString, types.IndexedValueTypeKeyword, types.IndexedValueTypeInt, types.IndexedValueTypeDouble, types.IndexedValueTypeBool, types.IndexedValueTypeDatetime}
	for _, expected := range values {
		require.Equal(t, expected, ConvertIndexedValueTypeToInternalType(int(expected), nil))
		require.Equal(t, expected, ConvertIndexedValueTypeToInternalType(float64(expected), nil))

		buffer, err := expected.MarshalText()
		require.NoError(t, err)
		require.Equal(t, expected, ConvertIndexedValueTypeToInternalType(buffer, nil))
		require.Equal(t, expected, ConvertIndexedValueTypeToInternalType(string(buffer), nil))
	}
}

func TestValidateDomainUUID(t *testing.T) {
	testCases := []struct {
		msg        string
		domainUUID string
		valid      bool
	}{
		{
			msg:        "empty",
			domainUUID: "",
			valid:      false,
		},
		{
			msg:        "invalid",
			domainUUID: "some random uuid",
			valid:      false,
		},
		{
			msg:        "valid",
			domainUUID: uuid.New(),
			valid:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.msg, func(t *testing.T) {
			err := ValidateDomainUUID(tc.domainUUID)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestConvertErrToGetTaskFailedCause(t *testing.T) {
	testCases := []struct {
		err                 error
		expectedFailedCause types.GetTaskFailedCause
	}{
		{
			err:                 errors.New("some random error"),
			expectedFailedCause: types.GetTaskFailedCauseUncategorized,
		},
		{
			err:                 context.DeadlineExceeded,
			expectedFailedCause: types.GetTaskFailedCauseTimeout,
		},
		{
			err:                 &types.ServiceBusyError{},
			expectedFailedCause: types.GetTaskFailedCauseServiceBusy,
		},
		{
			err:                 &types.ShardOwnershipLostError{},
			expectedFailedCause: types.GetTaskFailedCauseShardOwnershipLost,
		},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.expectedFailedCause, ConvertErrToGetTaskFailedCause(tc.err))
	}
}

func TestToServiceTransientError(t *testing.T) {
	t.Run("it converts nil", func(t *testing.T) {
		assert.NoError(t, ToServiceTransientError(nil))
	})

	t.Run("it keeps transient errors", func(t *testing.T) {
		err := &types.InternalServiceError{}
		assert.Equal(t, err, ToServiceTransientError(err))
		assert.True(t, IsServiceTransientError(ToServiceTransientError(err)))
	})

	t.Run("it converts errors to transient errors", func(t *testing.T) {
		err := fmt.Errorf("error")
		assert.True(t, IsServiceTransientError(ToServiceTransientError(err)))
	})
}

func TestIntersectionStringSlice(t *testing.T) {
	t.Run("it returns all items", func(t *testing.T) {
		a := []string{"a", "b", "c"}
		b := []string{"a", "b", "c"}
		c := IntersectionStringSlice(a, b)
		assert.ElementsMatch(t, []string{"a", "b", "c"}, c)
	})

	t.Run("it returns no item", func(t *testing.T) {
		a := []string{"a", "b", "c"}
		b := []string{"d", "e", "f"}
		c := IntersectionStringSlice(a, b)
		assert.ElementsMatch(t, []string{}, c)
	})

	t.Run("it returns intersection", func(t *testing.T) {
		a := []string{"a", "b", "c"}
		b := []string{"c", "b", "f"}
		c := IntersectionStringSlice(a, b)
		assert.ElementsMatch(t, []string{"c", "b"}, c)
	})
}

func TestAwaitWaitGroup(t *testing.T) {
	t.Run("wait group done before timeout", func(t *testing.T) {
		var wg sync.WaitGroup

		wg.Add(1)
		wg.Done()

		got := AwaitWaitGroup(&wg, time.Second)
		require.True(t, got)
	})

	t.Run("wait group done after timeout", func(t *testing.T) {
		var (
			wg    sync.WaitGroup
			doneC = make(chan struct{})
		)

		wg.Add(1)
		go func() {
			<-doneC
			wg.Done()
		}()

		got := AwaitWaitGroup(&wg, time.Microsecond)
		require.False(t, got)

		doneC <- struct{}{}
		close(doneC)
	})
}

func TestIsValidIDLength(t *testing.T) {
	var (
		// test setup
		scope = metrics.NoopScope

		// arguments
		metricCounter      = 0
		idTypeViolationTag = tag.ClusterName("idTypeViolationTag")
		domainName         = "domain_name"
		id                 = "12345"
	)

	mockWarnCall := func(logger *log.MockLogger) {
		logger.On(
			"Warn",
			"ID length exceeds limit.",
			[]tag.Tag{
				tag.WorkflowDomainName(domainName),
				tag.Name(id),
				idTypeViolationTag,
			},
		).Once()
	}

	t.Run("valid id length, no warnings", func(t *testing.T) {
		logger := new(log.MockLogger)
		got := IsValidIDLength(id, scope, 7, 10, metricCounter, domainName, logger, idTypeViolationTag)
		require.True(t, got, "expected true, because id length is 5 and it's less than error limit 10")
	})

	t.Run("valid id length, with warnings", func(t *testing.T) {
		logger := new(log.MockLogger)
		mockWarnCall(logger)

		got := IsValidIDLength(id, scope, 4, 10, metricCounter, domainName, logger, idTypeViolationTag)
		require.True(t, got, "expected true, because id length is 5 and it's less than error limit 10")

		// logger should be called once
		logger.AssertExpectations(t)
	})

	t.Run("non valid id length", func(t *testing.T) {
		logger := new(log.MockLogger)
		mockWarnCall(logger)

		got := IsValidIDLength(id, scope, 1, 4, metricCounter, domainName, logger, idTypeViolationTag)
		require.False(t, got, "expected false, because id length is 5 and it's more than error limit 4")

		// logger should be called once
		logger.AssertExpectations(t)
	})
}

func TestIsEntityNotExistsError(t *testing.T) {
	t.Run("is entity not exists error", func(t *testing.T) {
		err := &types.EntityNotExistsError{}
		require.True(t, IsEntityNotExistsError(err), "expected true, because err is entity not exists error")
	})

	t.Run("is not entity not exists error", func(t *testing.T) {
		err := fmt.Errorf("generic error")
		require.False(t, IsEntityNotExistsError(err), "expected false, because err is a generic error")
	})
}

func TestCreateXXXRetryPolicyWithSetExpirationInterval(t *testing.T) {
	for name, c := range map[string]struct {
		createFn func() backoff.RetryPolicy

		wantInitialInterval       time.Duration
		wantMaximumInterval       time.Duration
		wantSetExpirationInterval time.Duration
	}{
		"CreatePersistenceRetryPolicy": {
			createFn:                  CreatePersistenceRetryPolicy,
			wantInitialInterval:       retryPersistenceOperationInitialInterval,
			wantMaximumInterval:       retryPersistenceOperationMaxInterval,
			wantSetExpirationInterval: retryPersistenceOperationExpirationInterval,
		},
		"CreateHistoryServiceRetryPolicy": {
			createFn:                  CreateHistoryServiceRetryPolicy,
			wantInitialInterval:       historyServiceOperationInitialInterval,
			wantMaximumInterval:       historyServiceOperationMaxInterval,
			wantSetExpirationInterval: historyServiceOperationExpirationInterval,
		},
		"CreateMatchingServiceRetryPolicy": {
			createFn:                  CreateMatchingServiceRetryPolicy,
			wantInitialInterval:       matchingServiceOperationInitialInterval,
			wantMaximumInterval:       matchingServiceOperationMaxInterval,
			wantSetExpirationInterval: matchingServiceOperationExpirationInterval,
		},
		"CreateFrontendServiceRetryPolicy": {
			createFn:                  CreateFrontendServiceRetryPolicy,
			wantInitialInterval:       frontendServiceOperationInitialInterval,
			wantMaximumInterval:       frontendServiceOperationMaxInterval,
			wantSetExpirationInterval: frontendServiceOperationExpirationInterval,
		},
		"CreateAdminServiceRetryPolicy": {
			createFn:                  CreateAdminServiceRetryPolicy,
			wantInitialInterval:       adminServiceOperationInitialInterval,
			wantMaximumInterval:       adminServiceOperationMaxInterval,
			wantSetExpirationInterval: adminServiceOperationExpirationInterval,
		},
		"CreateReplicationServiceBusyRetryPolicy": {
			createFn:                  CreateReplicationServiceBusyRetryPolicy,
			wantInitialInterval:       replicationServiceBusyInitialInterval,
			wantMaximumInterval:       replicationServiceBusyMaxInterval,
			wantSetExpirationInterval: replicationServiceBusyExpirationInterval,
		},
		"CreateShardDistributorServiceRetryPolicy": {
			createFn:                  CreateShardDistributorServiceRetryPolicy,
			wantInitialInterval:       shardDistributorServiceOperationInitialInterval,
			wantMaximumInterval:       shardDistributorServiceOperationMaxInterval,
			wantSetExpirationInterval: shardDistributorServiceOperationExpirationInterval,
		},
	} {
		t.Run(name, func(t *testing.T) {
			want := backoff.NewExponentialRetryPolicy(c.wantInitialInterval)
			want.SetMaximumInterval(c.wantMaximumInterval)
			want.SetExpirationInterval(c.wantSetExpirationInterval)
			got := c.createFn()
			require.Equal(t, want, got)
		})
	}
}

func TestCreateXXXRetryPolicyWithMaximumAttempts(t *testing.T) {
	for name, c := range map[string]struct {
		createFn func() backoff.RetryPolicy

		wantInitialInterval time.Duration
		wantMaximumInterval time.Duration
		wantMaximumAttempts int
	}{
		"CreateDlqPublishRetryPolicy": {
			createFn:            CreateDlqPublishRetryPolicy,
			wantInitialInterval: retryKafkaOperationInitialInterval,
			wantMaximumInterval: retryKafkaOperationMaxInterval,
			wantMaximumAttempts: retryKafkaOperationMaxAttempts,
		},
		"CreateTaskProcessingRetryPolicy": {
			createFn:            CreateTaskProcessingRetryPolicy,
			wantInitialInterval: retryTaskProcessingInitialInterval,
			wantMaximumInterval: retryTaskProcessingMaxInterval,
			wantMaximumAttempts: retryTaskProcessingMaxAttempts,
		},
	} {
		t.Run(name, func(t *testing.T) {
			want := backoff.NewExponentialRetryPolicy(c.wantInitialInterval)
			want.SetMaximumInterval(c.wantMaximumInterval)
			want.SetMaximumAttempts(c.wantMaximumAttempts)
			got := c.createFn()
			require.Equal(t, want, got)
		})
	}
}

func TestValidateRetryPolicy_Success(t *testing.T) {
	for name, policy := range map[string]*types.RetryPolicy{
		"nil policy": nil,
		"MaximumAttempts is no zero": &types.RetryPolicy{
			InitialIntervalInSeconds:    2,
			BackoffCoefficient:          1,
			MaximumIntervalInSeconds:    0,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 0,
		},
		"ExpirationIntervalInSeconds is no zero": &types.RetryPolicy{
			InitialIntervalInSeconds:    2,
			BackoffCoefficient:          1,
			MaximumIntervalInSeconds:    0,
			MaximumAttempts:             0,
			ExpirationIntervalInSeconds: 1,
		},
		"MaximumIntervalInSeconds is greater than InitialIntervalInSeconds": &types.RetryPolicy{
			InitialIntervalInSeconds:    2,
			BackoffCoefficient:          1,
			MaximumIntervalInSeconds:    0,
			MaximumAttempts:             0,
			ExpirationIntervalInSeconds: 1,
		},
		"MaximumIntervalInSeconds equals InitialIntervalInSeconds": &types.RetryPolicy{
			InitialIntervalInSeconds:    2,
			BackoffCoefficient:          1,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             0,
			ExpirationIntervalInSeconds: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, ValidateRetryPolicy(policy))
		})
	}
}

func TestValidateRetryPolicy_Error(t *testing.T) {
	for name, c := range map[string]struct {
		policy  *types.RetryPolicy
		wantErr *types.BadRequestError
	}{
		"InitialIntervalInSeconds equals 0": {
			policy: &types.RetryPolicy{
				InitialIntervalInSeconds: 0,
			},
			wantErr: &types.BadRequestError{Message: "InitialIntervalInSeconds must be greater than 0 on retry policy."},
		},
		"InitialIntervalInSeconds less than 0": {
			policy: &types.RetryPolicy{
				InitialIntervalInSeconds: -1,
			},
			wantErr: &types.BadRequestError{Message: "InitialIntervalInSeconds must be greater than 0 on retry policy."},
		},
		"BackoffCoefficient equals 0": {
			policy: &types.RetryPolicy{
				InitialIntervalInSeconds: 1,
				BackoffCoefficient:       0,
			},
			wantErr: &types.BadRequestError{Message: "BackoffCoefficient cannot be less than 1 on retry policy."},
		},
		"MaximumIntervalInSeconds equals -1": {
			policy: &types.RetryPolicy{
				InitialIntervalInSeconds: 1,
				BackoffCoefficient:       1,
				MaximumIntervalInSeconds: -1,
			},
			wantErr: &types.BadRequestError{Message: "MaximumIntervalInSeconds cannot be less than 0 on retry policy."},
		},
		"MaximumIntervalInSeconds equals 1 and less than InitialIntervalInSeconds": {
			policy: &types.RetryPolicy{
				InitialIntervalInSeconds: 2,
				BackoffCoefficient:       1,
				MaximumIntervalInSeconds: 1,
			},
			wantErr: &types.BadRequestError{Message: "MaximumIntervalInSeconds cannot be less than InitialIntervalInSeconds on retry policy."},
		},
		"MaximumAttempts equals -1": {
			policy: &types.RetryPolicy{
				InitialIntervalInSeconds: 2,
				BackoffCoefficient:       1,
				MaximumIntervalInSeconds: 0,
				MaximumAttempts:          -1,
			},
			wantErr: &types.BadRequestError{Message: "MaximumAttempts cannot be less than 0 on retry policy."},
		},
		"ExpirationIntervalInSeconds equals -1": {
			policy: &types.RetryPolicy{
				InitialIntervalInSeconds:    2,
				BackoffCoefficient:          1,
				MaximumIntervalInSeconds:    0,
				MaximumAttempts:             0,
				ExpirationIntervalInSeconds: -1,
			},
			wantErr: &types.BadRequestError{Message: "ExpirationIntervalInSeconds cannot be less than 0 on retry policy."},
		},
		"MaximumAttempts and ExpirationIntervalInSeconds equal 0": {
			policy: &types.RetryPolicy{
				InitialIntervalInSeconds:    2,
				BackoffCoefficient:          1,
				MaximumIntervalInSeconds:    0,
				MaximumAttempts:             0,
				ExpirationIntervalInSeconds: 0,
			},
			wantErr: &types.BadRequestError{Message: "MaximumAttempts and ExpirationIntervalInSeconds are both 0. At least one of them must be specified."},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := ValidateRetryPolicy(c.policy)
			require.Error(t, got)
			require.ErrorContains(t, got, c.wantErr.Message)
		})
	}
}

func TestConvertGetTaskFailedCauseToErr(t *testing.T) {
	for cause, wantErr := range map[types.GetTaskFailedCause]error{
		types.GetTaskFailedCauseServiceBusy:        &types.ServiceBusyError{},
		types.GetTaskFailedCauseTimeout:            context.DeadlineExceeded,
		types.GetTaskFailedCauseShardOwnershipLost: &types.ShardOwnershipLostError{},
		types.GetTaskFailedCauseUncategorized:      &types.InternalServiceError{Message: "uncategorized error"},
	} {
		t.Run(cause.String(), func(t *testing.T) {
			gotErr := ConvertGetTaskFailedCauseToErr(cause)
			require.Equal(t, wantErr, gotErr)
		})
	}
}

func TestWorkflowIDToHistoryShard(t *testing.T) {
	for _, c := range []struct {
		workflowID     string
		numberOfShards int

		want int
	}{
		{
			workflowID:     "",
			numberOfShards: 1000,
			want:           242,
		},
		{
			workflowID:     "workflowId",
			numberOfShards: 1000,
			want:           580,
		},
	} {
		t.Run(fmt.Sprintf("%s-%v", c.workflowID, c.numberOfShards), func(t *testing.T) {
			got := WorkflowIDToHistoryShard(c.workflowID, c.numberOfShards)
			require.Equal(t, c.want, got)
		})
	}
}

func TestDomainIDToHistoryShard(t *testing.T) {
	for _, c := range []struct {
		domainID       string
		numberOfShards int

		want int
	}{
		{
			domainID:       "",
			numberOfShards: 1000,
			want:           242,
		},
		{
			domainID:       "domainId",
			numberOfShards: 1000,
			want:           600,
		},
	} {
		t.Run(fmt.Sprintf("%s-%v", c.domainID, c.numberOfShards), func(t *testing.T) {
			got := DomainIDToHistoryShard(c.domainID, c.numberOfShards)
			require.Equal(t, c.want, got)
		})
	}
}

func TestGenerateRandomString(t *testing.T) {
	for input, wantSize := range map[int]int{
		-1: 0,
		0:  0,
		10: 10,
	} {
		t.Run(fmt.Sprintf("%d", input), func(t *testing.T) {
			got := GenerateRandomString(input)
			require.Len(t, got, wantSize)
		})
	}
}

func TestIsValidContext(t *testing.T) {
	t.Run("background context", func(t *testing.T) {
		require.NoError(t, IsValidContext(context.Background()))
	})
	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got := IsValidContext(ctx)
		require.Error(t, got)
		require.ErrorIs(t, got, context.Canceled)
	})
	t.Run("deadline exceeded context", func(t *testing.T) {
		ctx, _ := context.WithTimeout(context.Background(), -time.Second)
		got := IsValidContext(ctx)
		require.Error(t, got)
		require.ErrorIs(t, got, context.DeadlineExceeded)
	})
	t.Run("context with deadline exceeded contextExpireThreshold", func(t *testing.T) {
		ctx, _ := context.WithTimeout(context.Background(), contextExpireThreshold/2)
		got := IsValidContext(ctx)
		require.Error(t, got)
		require.ErrorIs(t, got, context.DeadlineExceeded, "context.DeadlineExceeded should be returned, because context timeout is not later than now + contextExpireThreshold")
	})
	t.Run("valid context", func(t *testing.T) {
		ctx, _ := context.WithTimeout(context.Background(), contextExpireThreshold*2)
		require.NoError(t, IsValidContext(ctx), "nil should be returned, because context timeout is later than now + contextExpireThreshold")
	})
}

func TestCreateChildContext(t *testing.T) {
	t.Run("nil parent", func(t *testing.T) {
		gotCtx, gotFunc := CreateChildContext(nil, 0)
		require.Nil(t, gotCtx)
		require.Equal(t, funcName(emptyCancelFunc), funcName(gotFunc))
	})
	t.Run("canceled parent", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		gotCtx, gotFunc := CreateChildContext(ctx, 0)
		require.Equal(t, ctx, gotCtx)
		require.Equal(t, funcName(emptyCancelFunc), funcName(gotFunc))
	})
	t.Run("non-canceled parent without deadline", func(t *testing.T) {
		ctx, _ := context.WithCancel(context.Background())
		gotCtx, gotFunc := CreateChildContext(ctx, 0)
		require.Equal(t, ctx, gotCtx)
		require.Equal(t, funcName(emptyCancelFunc), funcName(gotFunc))
	})
	t.Run("context with deadline exceeded", func(t *testing.T) {
		ctx, _ := context.WithTimeout(context.Background(), -time.Second)
		gotCtx, gotFunc := CreateChildContext(ctx, 0)
		require.Equal(t, ctx, gotCtx)
		require.Equal(t, funcName(emptyCancelFunc), funcName(gotFunc))
	})

	t.Run("tailroom is less or equal to 0", func(t *testing.T) {
		testCase := func(t *testing.T, tailroom float64) {
			deadline := time.Now().Add(time.Hour)
			ctx, _ := context.WithDeadline(context.Background(), deadline)
			gotCtx, gotFunc := CreateChildContext(ctx, tailroom)

			gotDeadline, ok := gotCtx.Deadline()
			require.True(t, ok)
			require.Equal(t, deadline, gotDeadline, "deadline should be equal to parent deadline")

			require.NotEqual(t, ctx, gotCtx)
			require.NotEqual(t, funcName(emptyCancelFunc), funcName(gotFunc))
		}

		t.Run("0", func(t *testing.T) {
			testCase(t, 0)
		})
		t.Run("-1", func(t *testing.T) {
			testCase(t, -1)
		})

	})

	t.Run("tailroom is greater or equal to 1", func(t *testing.T) {
		testCase := func(t *testing.T, tailroom float64) {
			deadline := time.Now().Add(time.Hour)
			ctx, _ := context.WithDeadline(context.Background(), deadline)

			// we can't mock time.Now, but we know that the deadline should be in
			// range between the start and finish of function's execution
			beforeNow := time.Now()
			gotCtx, gotFunc := CreateChildContext(ctx, tailroom)
			afterNow := time.Now()

			gotDeadline, ok := gotCtx.Deadline()
			require.True(t, ok)
			require.NotEqual(t, deadline, gotDeadline)
			require.Less(t, gotDeadline, deadline)

			// gotDeadline should be between beforeNow and afterNow (exclusive)
			require.GreaterOrEqual(t, afterNow, gotDeadline)
			require.LessOrEqual(t, beforeNow, gotDeadline)

			require.NotEqual(t, ctx, gotCtx)
			require.NotEqual(t, funcName(emptyCancelFunc), funcName(gotFunc))
		}
		t.Run("1", func(t *testing.T) {
			testCase(t, 1)
		})
		t.Run("2", func(t *testing.T) {
			testCase(t, 2)
		})
	})
	t.Run("tailroom is 0.5", func(t *testing.T) {
		now := time.Now()
		deadline := now.Add(time.Hour)

		ctx, _ := context.WithDeadline(context.Background(), deadline)
		gotCtx, gotFunc := CreateChildContext(ctx, 0.5)

		gotDeadline, ok := gotCtx.Deadline()
		require.True(t, ok)
		require.NotEqual(t, deadline, gotDeadline)
		require.Less(t, gotDeadline, deadline)

		// we can't mock time.Now, so we assume that the deadline should be
		// in range 29:59 and 30:01 minutes after start
		minDeadline := now.Add(30*time.Minute - time.Second)
		maxDeadline := now.Add(30*time.Minute + time.Second)

		// gotDeadline should be between minDeadline and maxDeadline (exclusive)
		require.GreaterOrEqual(t, maxDeadline, gotDeadline)
		require.LessOrEqual(t, minDeadline, gotDeadline)

		require.NotEqual(t, ctx, gotCtx)
		require.NotEqual(t, funcName(emptyCancelFunc), funcName(gotFunc))
	})
}

// funcName returns the name of the function
func funcName(fn any) string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}

func TestDeserializeSearchAttributeValue_Success(t *testing.T) {
	for name, c := range map[string]struct {
		value     string
		valueType types.IndexedValueType

		wantValue any
	}{
		"string": {
			value:     `"string"`,
			valueType: types.IndexedValueTypeString,
			wantValue: "string",
		},
		"[]string": {
			value:     `["1", "2", "3"]`,
			valueType: types.IndexedValueTypeString,
			wantValue: []string{"1", "2", "3"},
		},
		"keyword": {
			value:     `"keyword"`,
			valueType: types.IndexedValueTypeKeyword,
			wantValue: "keyword",
		},
		"[]keyword": {
			value:     `["1", "2", "3"]`,
			valueType: types.IndexedValueTypeKeyword,
			wantValue: []string{"1", "2", "3"},
		},
		"int": {
			value:     `1`,
			valueType: types.IndexedValueTypeInt,
			wantValue: int64(1),
		},
		"[]int": {
			value:     `[1, 2, 3]`,
			valueType: types.IndexedValueTypeInt,
			wantValue: []int64{1, 2, 3},
		},
		"double": {
			value:     `1.1`,
			valueType: types.IndexedValueTypeDouble,
			wantValue: float64(1.1),
		},
		"[]double": {
			value:     `[1.1, 2.2, 3.3]`,
			valueType: types.IndexedValueTypeDouble,
			wantValue: []float64{1.1, 2.2, 3.3},
		},
		"bool": {
			value:     `true`,
			valueType: types.IndexedValueTypeBool,
			wantValue: true,
		},
		"[]bool": {
			value:     `[true, false, true]`,
			valueType: types.IndexedValueTypeBool,
			wantValue: []bool{true, false, true},
		},
		"datetime": {
			value:     `"2020-01-01T00:00:00Z"`,
			valueType: types.IndexedValueTypeDatetime,
			wantValue: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		"[]datetime": {
			value:     `["2020-01-01T00:00:00Z", "2020-01-02T00:00:00Z", "2020-01-03T00:00:00Z"]`,
			valueType: types.IndexedValueTypeDatetime,
			wantValue: []time.Time{
				time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
				time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			gotValue, err := DeserializeSearchAttributeValue([]byte(c.value), c.valueType)
			require.NoError(t, err)
			require.Equal(t, c.wantValue, gotValue)
		})
	}
}

func TestDeserializeSearchAttributeValue_Error(t *testing.T) {
	for name, c := range map[string]struct {
		value     string
		valueType types.IndexedValueType

		wantErrorMsg string
	}{
		"invalid string": {
			value:        `"string`,
			valueType:    types.IndexedValueTypeString,
			wantErrorMsg: "unexpected end of JSON input",
		},
		"invalid keyword": {
			value:        `"keyword`,
			valueType:    types.IndexedValueTypeKeyword,
			wantErrorMsg: "unexpected end of JSON input",
		},
		"invalid int": {
			value:        `1.1`,
			valueType:    types.IndexedValueTypeInt,
			wantErrorMsg: "json: cannot unmarshal number into Go value of type []int64",
		},
		"invalid double": {
			value:        `1as`,
			valueType:    types.IndexedValueTypeDouble,
			wantErrorMsg: "invalid character 'a' after top-level value",
		},
		"invalid bool": {
			value:        `1`,
			valueType:    types.IndexedValueTypeBool,
			wantErrorMsg: "json: cannot unmarshal number into Go value of type []bool",
		},
		"invalid datetime": {
			value:        `1`,
			valueType:    types.IndexedValueTypeDatetime,
			wantErrorMsg: "json: cannot unmarshal number into Go value of type []time.Time",
		},
		"invalid value type": {
			value:        `1`,
			valueType:    types.IndexedValueType(100),
			wantErrorMsg: fmt.Sprintf("error: unknown index value type [%v]", types.IndexedValueType(100)),
		},
	} {
		t.Run(name, func(t *testing.T) {
			gotValue, err := DeserializeSearchAttributeValue([]byte(c.value), c.valueType)
			require.Error(t, err)
			require.ErrorContains(t, err, c.wantErrorMsg)
			require.Nil(t, gotValue)
		})
	}
}

// someMapStringToByteArraySize is the size of someMapStringToByteArray calculated by GetSizeOfMapStringToByteArray
const someMapStringToByteArraySize = 66

var someMapStringToByteArray = map[string][]byte{
	"key":  []byte("value"),
	"key2": []byte("value2"),
}

// someBytesArraySize is len(someBytesArray)
const someBytesArraySize = 17

var someBytesArray = []byte("some random bytes")

func TestGetSizeOfMapStringToByteArray(t *testing.T) {
	require.Equal(t, someMapStringToByteArraySize, GetSizeOfMapStringToByteArray(someMapStringToByteArray))
}

func TestGetSizeOfHistoryEvent_NilEvent(t *testing.T) {
	require.Equal(t, uint64(0), GetSizeOfHistoryEvent(nil))
	require.Equal(t, uint64(0), GetSizeOfHistoryEvent(&types.HistoryEvent{}), "should be zero, because eventType is nil")
}

func TestGetSizeOfHistoryEvent(t *testing.T) {
	for eventType, c := range map[types.EventType]struct {
		event *types.HistoryEvent
		want  uint64
	}{
		types.EventTypeWorkflowExecutionStarted: {
			event: &types.HistoryEvent{
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					Input:                   someBytesArray, // 17 bytes
					ContinuedFailureDetails: someBytesArray, // 17 bytes
					LastCompletionResult:    someBytesArray, // 17 bytes
					Memo: &types.Memo{
						Fields: someMapStringToByteArray, // 66 bytes
					},
					Header: &types.Header{
						Fields: someMapStringToByteArray, // 66 bytes
					},
					SearchAttributes: &types.SearchAttributes{
						IndexedFields: someMapStringToByteArray, // 66 bytes
					},
				},
			},
			want: 3*someBytesArraySize + 3*someMapStringToByteArraySize,
		},
		types.EventTypeWorkflowExecutionCompleted: {
			event: &types.HistoryEvent{
				WorkflowExecutionCompletedEventAttributes: &types.WorkflowExecutionCompletedEventAttributes{
					Result: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeWorkflowExecutionFailed: {
			event: &types.HistoryEvent{
				WorkflowExecutionFailedEventAttributes: &types.WorkflowExecutionFailedEventAttributes{
					Details: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeWorkflowExecutionTimedOut: {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeDecisionTaskScheduled:     {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeDecisionTaskStarted:       {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeDecisionTaskCompleted: {
			event: &types.HistoryEvent{
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
					ExecutionContext: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeDecisionTaskTimedOut: {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeDecisionTaskFailed: {
			event: &types.HistoryEvent{
				DecisionTaskFailedEventAttributes: &types.DecisionTaskFailedEventAttributes{
					Details: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeActivityTaskScheduled: {
			event: &types.HistoryEvent{
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
					Input: someBytesArray, // 17 bytes
					Header: &types.Header{
						Fields: someMapStringToByteArray, // 66 bytes
					},
				},
			},
			want: someBytesArraySize + someMapStringToByteArraySize,
		},
		types.EventTypeActivityTaskStarted: {
			event: &types.HistoryEvent{
				ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
					LastFailureDetails: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeActivityTaskCompleted: {
			event: &types.HistoryEvent{
				ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{
					Result: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeActivityTaskFailed: {
			event: &types.HistoryEvent{
				ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{
					Details: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeActivityTaskTimedOut: {
			event: &types.HistoryEvent{
				ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{
					Details:            someBytesArray, // 17 bytes
					LastFailureDetails: someBytesArray, // 17 bytes
				},
			},
			want: 2 * someBytesArraySize,
		},
		types.EventTypeActivityTaskCancelRequested:     {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeRequestCancelActivityTaskFailed: {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeActivityTaskCanceled: {
			event: &types.HistoryEvent{
				ActivityTaskCanceledEventAttributes: &types.ActivityTaskCanceledEventAttributes{
					Details: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeTimerStarted:                     {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeTimerFired:                       {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeCancelTimerFailed:                {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeTimerCanceled:                    {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeWorkflowExecutionCancelRequested: {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeWorkflowExecutionCanceled: {
			event: &types.HistoryEvent{
				WorkflowExecutionCanceledEventAttributes: &types.WorkflowExecutionCanceledEventAttributes{
					Details: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeRequestCancelExternalWorkflowExecutionInitiated: {
			event: &types.HistoryEvent{
				RequestCancelExternalWorkflowExecutionInitiatedEventAttributes: &types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{
					Control: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeRequestCancelExternalWorkflowExecutionFailed: {
			event: &types.HistoryEvent{
				RequestCancelExternalWorkflowExecutionFailedEventAttributes: &types.RequestCancelExternalWorkflowExecutionFailedEventAttributes{
					Control: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeExternalWorkflowExecutionCancelRequested: {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeMarkerRecorded: {
			event: &types.HistoryEvent{
				MarkerRecordedEventAttributes: &types.MarkerRecordedEventAttributes{
					Details: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeWorkflowExecutionSignaled: {
			event: &types.HistoryEvent{
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{
					Input: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeWorkflowExecutionTerminated: {
			event: &types.HistoryEvent{
				WorkflowExecutionTerminatedEventAttributes: &types.WorkflowExecutionTerminatedEventAttributes{
					Details: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeWorkflowExecutionContinuedAsNew: {
			event: &types.HistoryEvent{
				WorkflowExecutionContinuedAsNewEventAttributes: &types.WorkflowExecutionContinuedAsNewEventAttributes{
					Input: someBytesArray, // 17 bytes
					Memo: &types.Memo{
						Fields: someMapStringToByteArray, // 66 bytes
					},
					Header: &types.Header{
						Fields: someMapStringToByteArray, // 66 bytes
					},
					SearchAttributes: &types.SearchAttributes{
						IndexedFields: someMapStringToByteArray, // 66 bytes
					},
				},
			},
			want: someBytesArraySize + 3*someMapStringToByteArraySize,
		},
		types.EventTypeStartChildWorkflowExecutionInitiated: {
			event: &types.HistoryEvent{
				StartChildWorkflowExecutionInitiatedEventAttributes: &types.StartChildWorkflowExecutionInitiatedEventAttributes{
					Input:   someBytesArray, // 17 bytes
					Control: someBytesArray, // 17 bytes
					Memo: &types.Memo{
						Fields: someMapStringToByteArray, // 66 bytes
					},
					Header: &types.Header{
						Fields: someMapStringToByteArray, // 66 bytes
					},
					SearchAttributes: &types.SearchAttributes{
						IndexedFields: someMapStringToByteArray, // 66 bytes
					},
				},
			},
			want: 2*someBytesArraySize + 3*someMapStringToByteArraySize,
		},
		types.EventTypeStartChildWorkflowExecutionFailed: {
			event: &types.HistoryEvent{
				StartChildWorkflowExecutionFailedEventAttributes: &types.StartChildWorkflowExecutionFailedEventAttributes{
					Control: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeChildWorkflowExecutionStarted: {
			event: &types.HistoryEvent{
				ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
					Header: &types.Header{
						Fields: someMapStringToByteArray, // 66 bytes
					},
				},
			},
			want: someMapStringToByteArraySize,
		},
		types.EventTypeChildWorkflowExecutionCompleted: {
			event: &types.HistoryEvent{
				ChildWorkflowExecutionCompletedEventAttributes: &types.ChildWorkflowExecutionCompletedEventAttributes{
					Result: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeChildWorkflowExecutionFailed: {
			event: &types.HistoryEvent{
				ChildWorkflowExecutionFailedEventAttributes: &types.ChildWorkflowExecutionFailedEventAttributes{
					Details: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeChildWorkflowExecutionCanceled: {
			event: &types.HistoryEvent{
				ChildWorkflowExecutionCanceledEventAttributes: &types.ChildWorkflowExecutionCanceledEventAttributes{
					Details: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeChildWorkflowExecutionTimedOut:   {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeChildWorkflowExecutionTerminated: {event: &types.HistoryEvent{}, want: 0},
		types.EventTypeSignalExternalWorkflowExecutionInitiated: {
			event: &types.HistoryEvent{
				SignalExternalWorkflowExecutionInitiatedEventAttributes: &types.SignalExternalWorkflowExecutionInitiatedEventAttributes{
					Input:   someBytesArray, // 17 bytes
					Control: someBytesArray, // 17 bytes
				},
			},
			want: 2 * someBytesArraySize,
		},
		types.EventTypeSignalExternalWorkflowExecutionFailed: {
			event: &types.HistoryEvent{
				SignalExternalWorkflowExecutionFailedEventAttributes: &types.SignalExternalWorkflowExecutionFailedEventAttributes{
					Control: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeExternalWorkflowExecutionSignaled: {
			event: &types.HistoryEvent{
				ExternalWorkflowExecutionSignaledEventAttributes: &types.ExternalWorkflowExecutionSignaledEventAttributes{
					Control: someBytesArray, // 17 bytes
				},
			},
			want: someBytesArraySize,
		},
		types.EventTypeUpsertWorkflowSearchAttributes: {
			event: &types.HistoryEvent{
				UpsertWorkflowSearchAttributesEventAttributes: &types.UpsertWorkflowSearchAttributesEventAttributes{
					SearchAttributes: &types.SearchAttributes{
						IndexedFields: someMapStringToByteArray, // 66 bytes
					},
				},
			},
			want: someMapStringToByteArraySize,
		},
	} {
		t.Run(eventType.String(), func(t *testing.T) {
			if c.event != nil {
				c.event.EventType = &eventType
			}

			got := GetSizeOfHistoryEvent(c.event)
			require.Equal(t, c.want, got)
		})
	}
}

func TestIsAdvancedVisibilityWritingEnabled(t *testing.T) {
	for name, c := range map[string]struct {
		advancedVisibilityWritingMode string
		isAdvancedVisConfigExist      bool
		want                          bool
	}{
		"mode is someMode, config exist": {
			advancedVisibilityWritingMode: "someMode",
			isAdvancedVisConfigExist:      true,
			want:                          true,
		},
		"mode is someMode, config not exist": {
			advancedVisibilityWritingMode: "someMode",
			isAdvancedVisConfigExist:      false,
			want:                          false,
		},
		"mode is off, config exist": {
			advancedVisibilityWritingMode: "off",
			isAdvancedVisConfigExist:      true,
			want:                          false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := IsAdvancedVisibilityWritingEnabled(c.advancedVisibilityWritingMode, c.isAdvancedVisConfigExist)
			require.Equal(t, c.want, got)
		})
	}
}

func TestValidateLongPollContextTimeout(t *testing.T) {
	const handlerName = "testHandler"

	t.Run("context timeout is not set", func(t *testing.T) {
		logger := new(log.MockLogger)
		logger.On(
			"Error",
			"Context timeout not set for long poll API.",
			[]tag.Tag{
				tag.WorkflowHandlerName(handlerName),
				tag.Error(ErrContextTimeoutNotSet),
			},
		)

		ctx := context.Background()
		got := ValidateLongPollContextTimeout(ctx, handlerName, logger)
		require.Error(t, got)
		require.ErrorIs(t, got, ErrContextTimeoutNotSet)
		logger.AssertExpectations(t)
	})

	t.Run("context timeout is set, but less than MinLongPollTimeout", func(t *testing.T) {
		logger := new(log.MockLogger)
		logger.On(
			"Error",
			"Context timeout is too short for long poll API.",
			// we can't mock time between deadline and now, so we just check it as it is
			mock.Anything,
		)
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		got := ValidateLongPollContextTimeout(ctx, handlerName, logger)
		require.Error(t, got)
		require.ErrorIs(t, got, ErrContextTimeoutTooShort, "should return ErrContextTimeoutTooShort, because context timeout is less than MinLongPollTimeout")
		logger.AssertExpectations(t)

	})

	t.Run("context timeout is set, but less than CriticalLongPollTimeout", func(t *testing.T) {
		logger := new(log.MockLogger)
		logger.On(
			"Debug",
			"Context timeout is lower than critical value for long poll API.",
			// we can't mock time between deadline and now, so we just check it as it is
			mock.Anything,
		)
		ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
		got := ValidateLongPollContextTimeout(ctx, handlerName, logger)
		require.NoError(t, got)
		logger.AssertExpectations(t)

	})

	t.Run("context timeout is set, but greater than CriticalLongPollTimeout", func(t *testing.T) {
		logger := new(log.MockLogger)
		ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
		got := ValidateLongPollContextTimeout(ctx, handlerName, logger)
		require.NoError(t, got)
		logger.AssertExpectations(t)
	})
}

func TestDurationToDays(t *testing.T) {
	for duration, want := range map[time.Duration]int32{
		0:              0,
		time.Hour:      0,
		24 * time.Hour: 1,
		25 * time.Hour: 1,
		48 * time.Hour: 2,
	} {
		t.Run(duration.String(), func(t *testing.T) {
			got := DurationToDays(duration)
			require.Equal(t, want, got)
		})
	}
}

func TestDurationToSeconds(t *testing.T) {
	for duration, want := range map[time.Duration]int64{
		0:                           0,
		time.Second:                 1,
		time.Second + time.Second/2: 1,
		2 * time.Second:             2,
	} {
		t.Run(duration.String(), func(t *testing.T) {
			got := DurationToSeconds(duration)
			require.Equal(t, want, got)
		})
	}
}

func TestDaysToDuration(t *testing.T) {
	for days, want := range map[int32]time.Duration{
		0: 0,
		1: 24 * time.Hour,
		2: 48 * time.Hour,
	} {
		t.Run(strconv.Itoa(int(days)), func(t *testing.T) {
			got := DaysToDuration(days)
			require.Equal(t, want, got)
		})
	}
}

func TestSecondsToDuration(t *testing.T) {
	for seconds, want := range map[int64]time.Duration{
		0: 0,
		1: time.Second,
		2: 2 * time.Second,
	} {
		t.Run(strconv.Itoa(int(seconds)), func(t *testing.T) {
			got := SecondsToDuration(seconds)
			require.Equal(t, want, got)
		})
	}
}

func TestNewPerTaskListScope(t *testing.T) {
	assert.NotNil(t, NewPerTaskListScope("test-domain", "test-tasklist", types.TaskListKindNormal, metrics.NewNoopMetricsClient(), 0))
	assert.NotNil(t, NewPerTaskListScope("test-domain", "test-tasklist", types.TaskListKindSticky, metrics.NewNoopMetricsClient(), 0))
	assert.NotNil(t, NewPerTaskListScope("test-domain", "test-tasklist", types.TaskListKindEphemeral, metrics.NewNoopMetricsClient(), 0))
}

func TestCheckEventBlobSizeLimit(t *testing.T) {
	for name, c := range map[string]struct {
		blobSize      int
		warnSize      int
		errSize       int
		wantErr       error
		prepareLogger func(*log.MockLogger)
		assertMetrics func(tally.Snapshot)
	}{
		"blob size is less than limit": {
			blobSize: 10,
			warnSize: 20,
			errSize:  30,
			wantErr:  nil,
		},
		"blob size is greater than warn limit": {
			blobSize: 21,
			warnSize: 20,
			errSize:  30,
			wantErr:  nil,
			prepareLogger: func(logger *log.MockLogger) {
				logger.On("Warn", "Blob size close to the limit.", mock.Anything).Once()
			},
		},
		"blob size is greater than error limit": {
			blobSize: 31,
			warnSize: 20,
			errSize:  30,
			wantErr:  ErrBlobSizeExceedsLimit,
			prepareLogger: func(logger *log.MockLogger) {
				logger.On("Error", "Blob size exceeds limit.", mock.Anything).Once()
			},
			assertMetrics: func(snapshot tally.Snapshot) {
				counters := snapshot.Counters()
				assert.Len(t, counters, 1)
				values := maps.Values(counters)
				assert.Equal(t, "test.blob_size_exceed_limit", values[0].Name())
				assert.Equal(t, int64(1), values[0].Value())
			},
		},
		"error limit is less then warn limit": {
			blobSize: 21,
			warnSize: 30,
			errSize:  20,
			wantErr:  ErrBlobSizeExceedsLimit,
			prepareLogger: func(logger *log.MockLogger) {
				logger.On("Warn", "Error limit is less than warn limit.", mock.Anything).Once()
				logger.On("Error", "Blob size exceeds limit.", mock.Anything).Once()
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			testScope := tally.NewTestScope("test", nil)
			metricsClient := metrics.NewClient(testScope, metrics.History)
			logger := log.NewMockLogger(t)

			if c.prepareLogger != nil {
				c.prepareLogger(logger)
			}

			const (
				domainID   = "testDomainID"
				domainName = "testDomainName"
				workflowID = "testWorkflowID"
				runID      = "testRunID"
			)

			got := CheckEventBlobSizeLimit(
				c.blobSize,
				c.warnSize,
				c.errSize,
				domainID,
				domainName,
				workflowID,
				runID,
				metricsClient.Scope(1),
				logger,
				tag.OperationName("testOperation"),
			)
			require.Equal(t, c.wantErr, got)
			if c.assertMetrics != nil {
				c.assertMetrics(testScope.Snapshot())
			}
		})
	}
}
