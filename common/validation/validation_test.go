// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package validation

import (
	"context"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"golang.org/x/exp/maps"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func TestIsValidIDLength(t *testing.T) {
	var (
		// test setup
		scope = metrics.NoopScope(0)

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

func TestValidateLongPollContextTimeout(t *testing.T) {
	const handlerName = "testHandler"

	t.Run("context timeout is not set", func(t *testing.T) {
		logger := new(log.MockLogger)
		logger.On(
			"Error",
			"Context timeout not set for long poll API.",
			[]tag.Tag{
				tag.WorkflowHandlerName(handlerName),
				tag.Error(common.ErrContextTimeoutNotSet),
			},
		)

		ctx := context.Background()
		got := ValidateLongPollContextTimeout(ctx, handlerName, logger)
		require.Error(t, got)
		require.ErrorIs(t, got, common.ErrContextTimeoutNotSet)
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
		require.ErrorIs(t, got, common.ErrContextTimeoutTooShort, "should return ErrContextTimeoutTooShort, because context timeout is less than MinLongPollTimeout")
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
			wantErr:  common.ErrBlobSizeExceedsLimit,
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
			wantErr:  common.ErrBlobSizeExceedsLimit,
			prepareLogger: func(logger *log.MockLogger) {
				logger.On("Warn", "Error limit is less than warn limit.", mock.Anything).Once()
				logger.On("Error", "Blob size exceeds limit.", mock.Anything).Once()
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			testScope := tally.NewTestScope("test", nil)
			metricsClient := metrics.NewClient(testScope, metrics.History)
			logger := &log.MockLogger{}
			defer logger.AssertExpectations(t)

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
