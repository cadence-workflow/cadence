package spectatorclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/clock"
)

func TestStreamTimesOutAfterDeadline(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockClient := sharddistributor.NewMockClient(ctrl)
	mockStreamClient := sharddistributor.NewMockWatchNamespaceStateClient(ctrl)
	mockTimeSource := clock.NewMockedTimeSource()

	mockClient.EXPECT().
		WatchNamespaceState(gomock.Any(), gomock.Any()).
		Return(mockStreamClient, nil)

	stream, err := newSpectatorStream(context.Background(), mockClient, mockTimeSource, "test-ns")
	require.NoError(t, err)
	require.NoError(t, stream.ctx.Err())

	// Advance past the timeout — the timer should cancel the stream context
	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(defaultStreamTimeout + time.Second)

	// Verify that the stream context is canceled
	require.Eventually(t, func() bool {
		return stream.ctx.Err() != nil
	}, time.Second, time.Millisecond)
	assert.ErrorIs(t, stream.ctx.Err(), context.Canceled)
}

func TestStreamCloseStopsTimer(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockClient := sharddistributor.NewMockClient(ctrl)
	mockStreamClient := sharddistributor.NewMockWatchNamespaceStateClient(ctrl)
	mockTimeSource := clock.NewMockedTimeSource()

	mockClient.EXPECT().
		WatchNamespaceState(gomock.Any(), gomock.Any()).
		Return(mockStreamClient, nil)

	stream, err := newSpectatorStream(context.Background(), mockClient, mockTimeSource, "test-ns")
	require.NoError(t, err)

	stream.Close()

	assert.ErrorIs(t, stream.ctx.Err(), context.Canceled)
}

func TestStreamCreationFailureCleansUp(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockClient := sharddistributor.NewMockClient(ctrl)
	mockTimeSource := clock.NewMockedTimeSource()

	mockClient.EXPECT().
		WatchNamespaceState(gomock.Any(), gomock.Any()).
		Return(nil, assert.AnError)

	stream, err := newSpectatorStream(context.Background(), mockClient, mockTimeSource, "test-ns")
	assert.Error(t, err)
	assert.Nil(t, stream)
}
