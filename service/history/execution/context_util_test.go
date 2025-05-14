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

package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

func createTestConfig() *config.Config {
	return &config.Config{
		EnableShardIDMetrics:                  dynamicproperties.GetBoolPropertyFn(true),
		LargeShardHistoryBlobMetricThreshold:  dynamicproperties.GetIntPropertyFn(1024 * 1024 * 5),    // 5 MB
		BlobSizeLimitWarn:                     func(domainName string) int { return 1024 * 1024 * 5 }, // 5 MB
		LargeShardHistoryEventMetricThreshold: dynamicproperties.GetIntPropertyFn(1500),
		HistoryCountLimitWarn:                 func(domainName string) int { return 1500 },
		LargeShardHistorySizeMetricThreshold:  dynamicproperties.GetIntPropertyFn(1024 * 1024 * 2),    // 2 MB
		HistorySizeLimitWarn:                  func(domainName string) int { return 1024 * 1024 * 2 }, // 2 MB
		SampleLoggingRate:                     dynamicproperties.GetIntPropertyFn(100),
	}
}

func TestEmitLargeWorkflowShardIDStats(t *testing.T) {
	tests := []struct {
		name                 string
		blobSize             int64
		oldHistoryCount      int64
		oldHistorySize       int64
		newHistoryCount      int64
		enableShardIDMetrics bool
		expectLogging        bool
		expectMetrics        bool
	}{
		{
			name:                 "Blob size exceeds threshold",
			blobSize:             1024 * 1024 * 10, // 10 MB
			oldHistoryCount:      1000,
			oldHistorySize:       1024 * 500, // 0.5 MB
			newHistoryCount:      2000,
			enableShardIDMetrics: true,
			expectLogging:        true,
			expectMetrics:        true,
		},
		{
			name:                 "History count and size within threshold",
			blobSize:             1024 * 500, // 0.5 MB
			oldHistoryCount:      500,
			oldHistorySize:       1024 * 1024, // 1 MB
			newHistoryCount:      800,
			enableShardIDMetrics: true,
			expectLogging:        false,
			expectMetrics:        false,
		},
		{
			name:                 "Metrics disabled",
			blobSize:             1024 * 1024 * 10, // 10 MB
			oldHistoryCount:      1000,
			oldHistorySize:       1024 * 1024 * 2, // 2 MB
			newHistoryCount:      2000,
			enableShardIDMetrics: false,
			expectLogging:        false,
			expectMetrics:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockShard := shard.NewMockContext(mockCtrl)
			metricScope := tally.NewTestScope("test", make(map[string]string))
			mockMetricsClient := metrics.NewClient(metricScope, metrics.History)
			mockLogger := log.NewMockLogger(t)
			stats := &persistence.ExecutionStats{
				HistorySize: 1024 * 1024 * 3, // 3 MB, setting it above the threshold for test
			}
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			context := &contextImpl{
				shard:         mockShard,
				metricsClient: mockMetricsClient,
				logger:        mockLogger,
				stats:         stats,
			}
			mockShard.EXPECT().GetShardID().Return(1).AnyTimes()
			mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).AnyTimes()
			mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("testDomain", nil).AnyTimes()
			mockShard.EXPECT().GetConfig().Return(createTestConfig()).AnyTimes()
			mockLogger.On("SampleInfo", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Return().Maybe()

			if tc.expectLogging {
				mockLogger.On("Info", mock.Anything, mock.Anything).Return().Once()
			}
			if tc.expectMetrics {
				mockLogger.On("Error", mock.Anything, mock.Anything).Return().Once()
			}

			context.emitLargeWorkflowShardIDStats(tc.blobSize, tc.oldHistoryCount, tc.oldHistorySize, tc.newHistoryCount)

			if tc.expectMetrics {
				countersSnapshot := metricScope.Snapshot().Counters()

				require.Contains(t, countersSnapshot, "test.large_history_blob_count+domain=testDomain,operation=LargeExecutionBlobShard,shard_id=1")
				require.Contains(t, countersSnapshot, "test.large_history_event_count+domain=testDomain,operation=LargeExecutionCountShard,shard_id=1")
				require.Contains(t, countersSnapshot, "test.large_history_size_count+domain=testDomain,operation=LargeExecutionSizeShard,shard_id=1")

				assert.Equal(t, int64(1), countersSnapshot["test.large_history_blob_count+domain=testDomain,operation=LargeExecutionBlobShard,shard_id=1"].Value())
				assert.Equal(t, int64(1), countersSnapshot["test.large_history_event_count+domain=testDomain,operation=LargeExecutionCountShard,shard_id=1"].Value())
				assert.Equal(t, int64(1), countersSnapshot["test.large_history_size_count+domain=testDomain,operation=LargeExecutionSizeShard,shard_id=1"].Value())
			}
		})
	}
}

func TestEmitWorkflowHistoryStats(t *testing.T) {

	mockMetricsClient := new(mocks.Client)
	mockScope := new(mocks.Scope)

	mockMetricsClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope)

	mockScope.On("RecordTimer", mock.Anything, mock.Anything).Return()

	domainName := "testDomain"
	historySize := 2048
	historyCount := 150

	emitWorkflowHistoryStats(mockMetricsClient, domainName, historySize, historyCount)

	mockMetricsClient.AssertExpectations(t)
	mockScope.AssertExpectations(t)
}

func TestEmitWorkflowExecutionStats(t *testing.T) {
	tests := []struct {
		name        string
		domainName  string
		stats       *persistence.MutableStateStats
		historySize int64
		expectCalls bool
	}{
		{
			name:       "With valid stats",
			domainName: "testDomain",
			stats: &persistence.MutableStateStats{
				MutableStateSize: 1024,
				ActivityInfoSize: 256,
				TimerInfoSize:    128,
			},
			historySize: 2048,
			expectCalls: true,
		},
		{
			name:        "Nil stats",
			domainName:  "testDomain",
			stats:       nil,
			historySize: 2048,
			expectCalls: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			mockMetricsClient := new(mocks.Client)
			mockScope := new(mocks.Scope)

			if tc.expectCalls {
				mockMetricsClient.On("Scope", metrics.ExecutionSizeStatsScope, mock.Anything).Return(mockScope)
				mockMetricsClient.On("Scope", metrics.ExecutionCountStatsScope, mock.Anything).Return(mockScope)
				mockScope.On("RecordTimer", mock.AnythingOfType("int"), mock.AnythingOfType("time.Duration")).Return().Times(14)
			} else {
				mockScope.AssertNotCalled(t, "RecordTimer")
			}

			emitWorkflowExecutionStats(mockMetricsClient, tc.domainName, tc.stats, tc.historySize)

			mockMetricsClient.AssertExpectations(t)
			mockScope.AssertExpectations(t)
		})
	}
}
