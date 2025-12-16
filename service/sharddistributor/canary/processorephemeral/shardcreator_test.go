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
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

func TestShardCreator_PingsShards(t *testing.T) {
	goleak.VerifyNone(t)

	logger := zaptest.NewLogger(t)
	timeSource := clock.NewMockedTimeSource()
	ctrl := gomock.NewController(t)

	namespace := "test-namespace"
	mockCanaryClient := NewMockShardDistributorExecutorCanaryAPIYARPCClient(ctrl)

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

	params := ShardCreatorParams{
		Logger:       logger,
		TimeSource:   timeSource,
		CanaryClient: mockCanaryClient,
	}

	creator := NewShardCreator(params, []string{namespace})
	creator.Start()

	// Wait for the goroutine to start and do it's ping
	timeSource.BlockUntil(1)
	timeSource.Advance(shardCreationInterval + 100*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	creator.Stop()
}

func TestShardCreator_isAllowed_noExecutors(t *testing.T) {
	testNamespace := "test-namespace"
	params := ShardCreatorParams{Logger: zaptest.NewLogger(t)}

	creator := NewShardCreator(params, []string{testNamespace})

	assert.True(t, creator.isAllowed(testNamespace))
}

func TestShardCreator_isAllowed_AnotherNamespace(t *testing.T) {
	goleak.VerifyNone(t)

	logger := zaptest.NewLogger(t)
	ctrl := gomock.NewController(t)

	testNamespace := "test-namespace"

	mockExecutor := executorclient.NewMockExecutor[*ShardProcessor](ctrl)
	mockExecutor.EXPECT().GetNamespace().Return(testNamespace)

	params := ShardCreatorParams{
		Logger: logger,
		ExecutorsEphemeral: []executorclient.Executor[*ShardProcessor]{
			mockExecutor,
		},
	}

	creator := NewShardCreator(params, []string{testNamespace})

	assert.True(t, creator.isAllowed("another-namespace"))
}

func TestShardCreator_isAllowed_LessThanMax(t *testing.T) {
	goleak.VerifyNone(t)

	logger := zaptest.NewLogger(t)
	ctrl := gomock.NewController(t)

	testNamespace := "test-namespace"

	mockExecutor := executorclient.NewMockExecutor[*ShardProcessor](ctrl)
	mockExecutor.EXPECT().GetNamespace().Return(testNamespace)
	mockExecutor.EXPECT().GetAssignedShardsCount().Return(int64(10))

	params := ShardCreatorParams{
		Logger: logger,
		ExecutorsEphemeral: []executorclient.Executor[*ShardProcessor]{
			mockExecutor,
		},
	}

	creator := NewShardCreator(params, []string{testNamespace})

	assert.True(t, creator.isAllowed(testNamespace))
}

func TestShardCreator_isAllowed_AssignedCountGreaterMax(t *testing.T) {
	goleak.VerifyNone(t)

	logger := zaptest.NewLogger(t)
	ctrl := gomock.NewController(t)

	testNamespace := "test-namespace"

	mockExecutor := executorclient.NewMockExecutor[*ShardProcessor](ctrl)
	mockExecutor.EXPECT().GetNamespace().Return(testNamespace)
	mockExecutor.EXPECT().GetAssignedShardsCount().Return(int64(maxAssignedShardsLimit))

	params := ShardCreatorParams{
		Logger: logger,
		ExecutorsEphemeral: []executorclient.Executor[*ShardProcessor]{
			mockExecutor,
		},
	}

	creator := NewShardCreator(params, []string{testNamespace})

	assert.False(t, creator.isAllowed(testNamespace))
}
