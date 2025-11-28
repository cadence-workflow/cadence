package pinger

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

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

func TestPingShard_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := NewMockShardDistributorExecutorCanaryAPIYARPCClient(ctrl)

	pinger := NewPinger(Params{
		Logger:       zap.NewNop(),
		TimeSource:   clock.NewRealTimeSource(),
		CanaryClient: mockClient,
	}, "test-ns", 10)
	pinger.ctx, pinger.cancel = context.WithCancel(context.Background())
	defer pinger.cancel()

	mockClient.EXPECT().
		Ping(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&sharddistributorv1.PingResponse{
			OwnsShard:  true,
			ExecutorId: "127.0.0.1:7953",
		}, nil)

	err := pinger.pingShard("5")
	assert.NoError(t, err)
}

func TestPingShard_DoesNotOwnShard(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := NewMockShardDistributorExecutorCanaryAPIYARPCClient(ctrl)

	pinger := NewPinger(Params{
		Logger:       zap.NewNop(),
		TimeSource:   clock.NewRealTimeSource(),
		CanaryClient: mockClient,
	}, "test-ns", 10)
	pinger.ctx, pinger.cancel = context.WithCancel(context.Background())
	defer pinger.cancel()

	mockClient.EXPECT().
		Ping(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&sharddistributorv1.PingResponse{
			OwnsShard:  false,
			ExecutorId: "127.0.0.1:7953",
		}, nil)

	err := pinger.pingShard("5")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not own shard")
}

func TestPingShard_RPCError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := NewMockShardDistributorExecutorCanaryAPIYARPCClient(ctrl)

	pinger := NewPinger(Params{
		Logger:       zap.NewNop(),
		TimeSource:   clock.NewRealTimeSource(),
		CanaryClient: mockClient,
	}, "test-ns", 10)
	pinger.ctx, pinger.cancel = context.WithCancel(context.Background())
	defer pinger.cancel()

	mockClient.EXPECT().
		Ping(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("network error"))

	err := pinger.pingShard("5")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ping rpc failed")
}

func TestNewPinger(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := NewMockShardDistributorExecutorCanaryAPIYARPCClient(ctrl)

	pinger := NewPinger(Params{
		Logger:       zap.NewNop(),
		TimeSource:   clock.NewRealTimeSource(),
		CanaryClient: mockClient,
	}, "test-ns", 100)

	require.NotNil(t, pinger)
	assert.Equal(t, "test-ns", pinger.namespace)
	assert.Equal(t, 100, pinger.numShards)
}
