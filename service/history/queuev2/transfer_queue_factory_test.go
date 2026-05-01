package queuev2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/queue"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/worker/archiver"
)

func TestTransferQueueFactory_CreateQueuev2(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	mockShard := shard.NewTestContext(
		t, ctrl, &persistence.ShardInfo{
			ShardID:          10,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest())

	// Create the factory
	factory := &transferQueueFactory{
		taskProcessor:  task.NewMockProcessor(ctrl),
		archivalClient: archiver.NewMockClient(ctrl),
	}

	// Test the createQueuev2 method
	processor := factory.createQueuev2(mockShard, execution.NewMockCache(ctrl), invariant.NewMockInvariant(ctrl))

	// Verify the result
	assert.NotNil(t, processor)
	assert.Implements(t, (*queue.Processor)(nil), processor)
}

func TestTransferQueueFactory_CreateQueuev1(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	mockShard := shard.NewTestContext(
		t, ctrl, &persistence.ShardInfo{
			ShardID:          10,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest())

	// Create the factory
	factory := &transferQueueFactory{
		taskProcessor:  task.NewMockProcessor(ctrl),
		archivalClient: archiver.NewMockClient(ctrl),
	}

	// Test the createQueuev1 method
	processor := factory.createQueuev1(mockShard, execution.NewMockCache(ctrl), invariant.NewMockInvariant(ctrl))

	// Verify the result
	assert.NotNil(t, processor)
	assert.Implements(t, (*queue.Processor)(nil), processor)
}

func TestTransferQueueFactory_Category(t *testing.T) {
	factory := &transferQueueFactory{}

	category := factory.Category()

	assert.Equal(t, persistence.HistoryTaskCategoryTransfer, category)
}

func TestTransferQueueFactory_IsQueueV2Enabled(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	mockShard := shard.NewTestContext(
		t, ctrl, &persistence.ShardInfo{
			ShardID:          10,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest())

	factory := &transferQueueFactory{}

	// Test the isQueueV2Enabled method
	// by default, queue v2 is disabled
	enabled := factory.isQueueV2Enabled(mockShard)
	assert.False(t, enabled)
}
