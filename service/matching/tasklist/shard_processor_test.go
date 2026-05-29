package tasklist

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func mustNewIdentifier(domainID, name string, taskType int) *Identifier {
	id, err := NewIdentifier(domainID, name, taskType)
	if err != nil {
		panic(err)
	}
	return id
}

var testIdentifier = mustNewIdentifier("domain-id", "tl", persistence.TaskListTypeDecision)

type shardProcessorTestData struct {
	mockRegistry   *MockTaskListRegistry
	shardProcessor ShardProcessor
}

func newShardProcessorTestData(t *testing.T, taskListID *Identifier) shardProcessorTestData {
	ctrl := gomock.NewController(t)

	mockRegistry := NewMockTaskListRegistry(ctrl)
	mockRegistry.EXPECT().ManagersByTaskListName(taskListID.GetName()).Return([]Manager{}).AnyTimes()

	params := ShardProcessorParams{
		ShardID:           taskListID.GetName(),
		TaskListsRegistry: mockRegistry,
		ReportTTL:         1 * time.Millisecond,
		TimeSource:        clock.NewRealTimeSource(),
	}

	shardProcessor, err := NewShardProcessor(params)
	require.NoError(t, err)
	return shardProcessorTestData{
		mockRegistry:   mockRegistry,
		shardProcessor: shardProcessor,
	}
}

func TestNewShardProcessorFailsWithEmptyParams(t *testing.T) {
	params := ShardProcessorParams{}
	sp, err := NewShardProcessor(params)
	require.Nil(t, sp)
	require.Error(t, err)
}

func TestGetShardReport(t *testing.T) {
	td := newShardProcessorTestData(t, testIdentifier)

	shardReport := td.shardProcessor.GetShardReport()
	require.NotNil(t, shardReport)
	require.Equal(t, float64(0), shardReport.ShardLoad)
	require.Equal(t, types.ShardStatusREADY, shardReport.Status)
}

func TestSetShardStatus(t *testing.T) {
	defer goleak.VerifyNone(t)
	td := newShardProcessorTestData(t, testIdentifier)

	td.shardProcessor.SetShardStatus(types.ShardStatusREADY)
	shardReport := td.shardProcessor.GetShardReport()
	require.NotNil(t, shardReport)
	require.Equal(t, float64(0), shardReport.ShardLoad)
	require.Equal(t, types.ShardStatusREADY, shardReport.Status)
}
