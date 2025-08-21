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

package metered

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/handler"
)

func TestMetricsHandler_GetShardOwner(t *testing.T) {
	request := &types.GetShardOwnerRequest{
		ShardKey:  "test-shard-key",
		Namespace: "test-namespace",
	}
	response := &types.GetShardOwnerResponse{
		Owner:     "test-owner",
		Namespace: "test-namespace",
	}

	tests := []struct {
		name       string
		setupMocks func(logger *log.MockLogger)
		error      error
	}{
		{
			name:       "Success",
			setupMocks: func(logger *log.MockLogger) {},
			error:      nil,
		},
		{
			name: "Failure",
			setupMocks: func(logger *log.MockLogger) {
				logger.On("Helper").Return(logger)
				logger.On(
					"Error",
					"Internal service error",
					[]tag.Tag{tag.Error(&types.InternalServiceError{})},
				).Once()
			},
			error: &types.InternalServiceError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			testScope := tally.NewTestScope("test", nil)
			metricsClient := metrics.NewClient(testScope, metrics.ShardDistributor)
			mockHandler := handler.NewMockHandler(ctrl)

			mockHandler.EXPECT().GetShardOwner(gomock.Any(), request).Return(response, tt.error)

			mockLogger := log.NewMockLogger(t)
			mockLogger.On("WithTags", []tag.Tag{tag.ShardNamespace("test-namespace")}).Return(mockLogger)

			handler := NewMetricsHandler(mockHandler, mockLogger, metricsClient).(*metricsHandler)
			tt.setupMocks(mockLogger)

			gotResponse, err := handler.GetShardOwner(context.Background(), request)

			assert.Equal(t, response, gotResponse)
			assert.Equal(t, tt.error, err)

			// check that the metrics were emitted for this method
			requestCounterName := "test.shard_distributor_requests+namespace=test-namespace,operation=GetShardOwner"
			requestCounter := testScope.Snapshot().Counters()[requestCounterName]
			assert.NotNil(t, requestCounter)
			assert.Equal(t, int64(1), requestCounter.Value())

			latencyTimerName := "test.shard_distributor_latency+namespace=test-namespace,operation=GetShardOwner"
			lantencyTimer := testScope.Snapshot().Timers()[latencyTimerName]
			assert.NotNil(t, lantencyTimer)

			// check that the latency timer was run, and is not 0
			assert.Greater(t, lantencyTimer.Values()[0], int64(0))
		})
	}
}

// For these methods we expect no metrics nor logs to be emitted
func TestPassThroughMethods(t *testing.T) {
	tests := []struct {
		name string
		call func(*testing.T, *metricsHandler, *handler.MockHandler)
	}{
		{
			name: "Health",
			call: func(t *testing.T, h *metricsHandler, handlerMock *handler.MockHandler) {
				ctx := context.Background()
				healthStatus := &types.HealthStatus{
					Ok:  true,
					Msg: "test",
				}
				handlerMock.EXPECT().Health(ctx).Return(healthStatus, nil)

				healthStatusRet, err := h.Health(ctx)
				assert.NoError(t, err)
				assert.Equal(t, healthStatus, healthStatusRet)
			},
		},
		{
			name: "Start",
			call: func(t *testing.T, h *metricsHandler, handlerMock *handler.MockHandler) {
				handlerMock.EXPECT().Start()
				h.Start()
			},
		},
		{
			name: "Stop",
			call: func(t *testing.T, h *metricsHandler, handlerMock *handler.MockHandler) {
				handlerMock.EXPECT().Stop()
				h.Stop()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			testScope := tally.NewTestScope("test", nil)
			metricsClient := metrics.NewClient(testScope, metrics.ShardDistributor)
			mockHandler := handler.NewMockHandler(ctrl)

			// We expect _no_ log calls
			mockLogger := log.NewMockLogger(t)

			handler := NewMetricsHandler(mockHandler, mockLogger, metricsClient).(*metricsHandler)

			tt.call(t, handler, mockHandler)

			// check that no metrics were emitted for this method
			assert.Empty(t, testScope.Snapshot().Counters())
			assert.Empty(t, testScope.Snapshot().Timers())
			assert.Empty(t, testScope.Snapshot().Gauges())
			assert.Empty(t, testScope.Snapshot().Histograms())
		})
	}
}

func TestHandleErr(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		expectedError error
		setupMocks    func(mockLogger *log.MockLogger)
		metricName    string
	}{
		{
			name:          "InternalServiceError",
			err:           &types.InternalServiceError{},
			expectedError: &types.InternalServiceError{},
			setupMocks: func(mockLogger *log.MockLogger) {
				mockLogger.On(
					"Error",
					"Internal service error",
					[]tag.Tag{tag.Error(&types.InternalServiceError{})},
				).Once()
			},
			metricName: "shard_distributor_failures",
		},
		{
			name:          "NamespaceNotFoundError",
			err:           &types.NamespaceNotFoundError{},
			expectedError: &types.NamespaceNotFoundError{},
			setupMocks:    func(mockLogger *log.MockLogger) {},
			metricName:    "shard_distributor_err_namespace_not_found",
		},
		{
			name:          "ShardNotFoundError",
			err:           &types.ShardNotFoundError{},
			expectedError: &types.ShardNotFoundError{},
			setupMocks:    func(mockLogger *log.MockLogger) {},
			metricName:    "shard_distributor_err_shard_not_found",
		},
		{
			name:          "ContextDeadlineExceeded",
			err:           context.DeadlineExceeded,
			expectedError: context.DeadlineExceeded,
			setupMocks: func(mockLogger *log.MockLogger) {
				mockLogger.On(
					"Error",
					"request timeout",
					[]tag.Tag{tag.Error(context.DeadlineExceeded)},
				).Once()
			},
			metricName: "shard_distributor_err_context_timeout",
		},
		{
			name:          "UncategorizedError",
			err:           errors.New("uncategorized error"),
			expectedError: errors.New("uncategorized error"),
			setupMocks: func(mockLogger *log.MockLogger) {
				mockLogger.On(
					"Error",
					"internal uncategorized error",
					[]tag.Tag{tag.Error(errors.New("uncategorized error"))},
				).Once()
			},
			metricName: "shard_distributor_failures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testScope := tally.NewTestScope("test", nil)
			metricsClient := metrics.NewClient(testScope, metrics.ShardDistributor)
			mockLogger := log.NewMockLogger(t)
			mockLogger.On("Helper").Return(mockLogger)

			tt.setupMocks(mockLogger)
			err := handleErr(tt.err, metricsClient.Scope(metrics.ShardDistributorGetShardOwnerScope), mockLogger)
			require.Equal(t, tt.expectedError, err)

			// check metrics
			fullName := fmt.Sprintf("test.%s+operation=GetShardOwner", tt.metricName)
			counter := testScope.Snapshot().Counters()[fullName]
			assert.NotNil(t, counter)
			assert.Equal(t, int64(1), counter.Value())
		})
	}
}
