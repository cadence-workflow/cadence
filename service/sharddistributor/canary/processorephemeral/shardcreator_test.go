package processorephemeral

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
)

func TestShardCreator_Lifecycle(t *testing.T) {
	goleak.VerifyNone(t)

	logger := zaptest.NewLogger(t)
	timeSource := clock.NewMockedTimeSource()
	ctrl := gomock.NewController(t)

	mockShardDistributor := sharddistributor.NewMockClient(ctrl)
	namespace := "test-namespace"

	// Set up expectation for GetShardOwner calls that return errors
	mockShardDistributor.EXPECT().
		GetShardOwner(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx interface{}, req *types.GetShardOwnerRequest, opts ...interface{}) (*types.GetShardOwnerResponse, error) {
			// Verify the request contains the correct namespace even on error
			assert.Equal(t, namespace, req.Namespace)
			assert.NotEmpty(t, req.ShardKey)

			return nil, assert.AnError // Using testify's AnError for consistency
		}).
		Times(2)

	params := ShardCreatorParams{
		Logger:           logger,
		TimeSource:       timeSource,
		ShardDistributor: mockShardDistributor,
	}

	creator := NewShardCreator(params, namespace)
	creator.Start()

	// Wait for the goroutine to start
	timeSource.BlockUntil(1)

	// Trigger shard creation that will fail
	timeSource.Advance(shardCreationInterval + 100*time.Millisecond)
	time.Sleep(10 * time.Millisecond) // Allow processing

	// Trigger another shard creation to ensure processing continues after error
	timeSource.Advance(shardCreationInterval + 100*time.Millisecond)
	time.Sleep(10 * time.Millisecond) // Allow processing

	creator.Stop()
}
