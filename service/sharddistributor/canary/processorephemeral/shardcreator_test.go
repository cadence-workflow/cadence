package processorephemeral

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
)

func TestShardCreator_Lifecycle(t *testing.T) {
	goleak.VerifyNone(t)

	logger := zaptest.NewLogger(t)
	timeSource := clock.NewMockedTimeSource()
	ctrl := gomock.NewController(t)

	namespace := "test-namespace"

	mockSpectator := spectatorclient.NewMockSpectator(ctrl)
	mockCanaryClient := NewMockShardDistributorExecutorCanaryAPIYARPCClient(ctrl)

	// First call fails - no ping should happen
	firstCall := mockSpectator.EXPECT().
		GetShardOwner(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx interface{}, shardKey string) (*spectatorclient.ShardOwner, error) {
			assert.NotEmpty(t, shardKey)
			return nil, assert.AnError
		})

	// Second call succeeds - ping should happen
	mockSpectator.EXPECT().
		GetShardOwner(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx interface{}, shardKey string) (*spectatorclient.ShardOwner, error) {
			assert.NotEmpty(t, shardKey)
			return &spectatorclient.ShardOwner{
				ExecutorID: "executor-1",
			}, nil
		}).
		After(firstCall)

	// Ping happens after successful GetShardOwner
	mockCanaryClient.EXPECT().
		Ping(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx interface{}, req *sharddistributorv1.PingRequest, opts ...interface{}) (*sharddistributorv1.PingResponse, error) {
			assert.NotEmpty(t, req.ShardKey)
			assert.Equal(t, namespace, req.Namespace)
			return &sharddistributorv1.PingResponse{
				OwnsShard:  true,
				ExecutorId: "executor-1",
			}, nil
		})

	spectators := map[string]spectatorclient.Spectator{
		namespace: mockSpectator,
	}

	params := ShardCreatorParams{
		Logger:       logger,
		TimeSource:   timeSource,
		Spectators:   spectators,
		CanaryClient: mockCanaryClient,
	}

	creator := NewShardCreator(params, []string{namespace})
	creator.Start()

	timeSource.BlockUntil(1)

	// First cycle - GetShardOwner fails, no ping
	timeSource.Advance(shardCreationInterval + 100*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	// Second cycle - GetShardOwner succeeds, ping happens
	timeSource.Advance(shardCreationInterval + 100*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	creator.Stop()
}
