package metered

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	_testNamespace  = "test_namespace"
	_testExecutorID = "test_executorID"
)

func TestMeteredStore_GetHeartbeat(t *testing.T) {
	heartbeatRes := &store.HeartbeatState{
		LastHeartbeat: time.Now().UTC(),
	}
	assignedState := &store.AssignedState{
		LastUpdated: time.Now().UTC(),
	}

	tests := []struct {
		name  string
		error error
	}{
		{
			name:  "Success",
			error: nil,
		},
		{
			name:  "NotFound",
			error: store.ErrExecutorNotFound,
		},
		{
			name:  "Failure",
			error: &types.InternalServiceError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			testScope := tally.NewTestScope("test", nil)
			metricsClient := metrics.NewClient(testScope, metrics.ShardDistributor, metrics.HistogramMigration{})
			timeSource := clock.NewMockedTimeSource()
			mockHandler := store.NewMockStore(ctrl)

			mockHandler.EXPECT().GetHeartbeat(gomock.Any(), _testNamespace, _testExecutorID).Do(func(ctx context.Context, namespace string, executorID string) {
				timeSource.Advance(time.Second)
			}).Return(heartbeatRes, assignedState, tt.error)

			mockLogger := log.NewMockLogger(ctrl)
			mockLogger.EXPECT().Helper().Return(mockLogger).AnyTimes()

			wrapped := NewStore(mockHandler, metricsClient, mockLogger, timeSource).(*meteredStore)

			gotHeartbeat, gotAssignedState, err := wrapped.GetHeartbeat(context.Background(), _testNamespace, _testExecutorID)

			assert.Equal(t, heartbeatRes, gotHeartbeat)
			assert.Equal(t, assignedState, gotAssignedState)
			assert.Equal(t, tt.error, err)

			// check that the metrics were emitted for this method
			requestCounterName := "test.shard_distributor_store_requests_per_namespace+namespace=test_namespace,operation=StoreGetHeartbeat"
			assert.Contains(t, testScope.Snapshot().Counters(), requestCounterName)
			requestCounter := testScope.Snapshot().Counters()[requestCounterName]
			assert.Equal(t, int64(1), requestCounter.Value())

			latencyHistogramName := "test.shard_distributor_store_latency_histogram_per_namespace+namespace=test_namespace,operation=StoreGetHeartbeat"
			allHistograms := testScope.Snapshot().Histograms()
			assert.Contains(t, allHistograms, latencyHistogramName)
		})
	}
}
