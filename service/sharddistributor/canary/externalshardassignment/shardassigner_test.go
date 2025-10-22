package exetrnalshardassignment

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/executorclient"
)

func TestSShardAssigner_Lifecycle(t *testing.T) {
	goleak.VerifyNone(t)

	logger := zaptest.NewLogger(t)
	timeSource := clock.NewMockedTimeSource()
	ctrl := gomock.NewController(t)
	executorclientmock := executorclient.NewMockExecutor[*processor.ShardProcessor](ctrl)

	namespace := "test-namespace"

	executorclientmock.EXPECT().AssignShardsFromLocalLogic(gomock.Any(), gomock.Any())
	executorclientmock.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
	executorclientmock.EXPECT().AssignShardsFromLocalLogic(gomock.Any(), gomock.Any())
	executorclientmock.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	params := ShardAssignerParams{
		Logger:         logger,
		TimeSource:     timeSource,
		Executorclient: executorclientmock,
	}

	assigner := NewShardAssigner(params, namespace)
	assigner.Start()

	// Wait for the goroutine to start
	timeSource.BlockUntil(1)

	// Trigger shard creation that will fail
	timeSource.Advance(shardAssignmentInterval + 100*time.Millisecond)
	time.Sleep(10 * time.Millisecond) // Allow processing

	// Trigger another shard creation to ensure processing continues after error
	timeSource.Advance(shardAssignmentInterval + 100*time.Millisecond)
	time.Sleep(10 * time.Millisecond) // Allow processing

	assigner.Stop()
}
