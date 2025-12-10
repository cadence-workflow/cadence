package pinger

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/common/clock"
)

func TestPingerStartStop(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockClient := NewMockShardDistributorExecutorCanaryAPIYARPCClient(ctrl)

	pinger := NewPinger(Params{
		Logger:       zap.NewNop(),
		TimeSource:   clock.NewRealTimeSource(),
		CanaryClient: mockClient,
	}, "test-ns", 10)

	pinger.Start(context.Background())
	pinger.Stop()
}

func TestPingerPingRandomShard(t *testing.T) {
	defer goleak.VerifyNone(t)

	cases := []struct {
		name            string
		setupClientMock func(*MockShardDistributorExecutorCanaryAPIYARPCClient)
		expectedLog     string
	}{
		{
			name: "owns shard",
			setupClientMock: func(mockClient *MockShardDistributorExecutorCanaryAPIYARPCClient) {
				mockClient.EXPECT().Ping(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&sharddistributorv1.PingResponse{
						OwnsShard:  true,
						ExecutorId: "127.0.0.1:7953",
					}, nil)
			},
			expectedLog: "Successfully pinged shard owner",
		},
		{
			name: "does not own shard",
			setupClientMock: func(mockClient *MockShardDistributorExecutorCanaryAPIYARPCClient) {
				mockClient.EXPECT().
					Ping(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&sharddistributorv1.PingResponse{
						OwnsShard:  false,
						ExecutorId: "127.0.0.1:7953",
					}, nil)
			},
			expectedLog: "Executor does not own shard",
		},
		{
			name: "RPC error",
			setupClientMock: func(mockClient *MockShardDistributorExecutorCanaryAPIYARPCClient) {
				mockClient.EXPECT().
					Ping(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("network error"))
			},
			expectedLog: "Failed to ping shard",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := NewMockShardDistributorExecutorCanaryAPIYARPCClient(ctrl)
			zapCore, logs := observer.New(zap.InfoLevel)
			logger := zap.New(zapCore)

			pinger := NewPinger(Params{
				Logger:       logger,
				TimeSource:   clock.NewRealTimeSource(),
				CanaryClient: mockClient,
			}, "test-ns", 10)
			pinger.ctx = context.Background()

			tt.setupClientMock(mockClient)

			pinger.pingRandomShard()

			assert.Equal(t, 1, logs.FilterMessage(tt.expectedLog).Len())
		})
	}
}
