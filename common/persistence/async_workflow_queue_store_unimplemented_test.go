package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/types"
)

func TestUnimplementedAsyncWorkflowQueueStore_MethodsReturnNotImplemented(t *testing.T) {
	ctx := context.Background()
	store := NewUnimplementedAsyncWorkflowQueueStore("test-backend")

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "EnqueueAsyncWorkflowMessage",
			call: func() error {
				return store.EnqueueAsyncWorkflowMessage(ctx, &InternalEnqueueAsyncWorkflowMessageRequest{})
			},
		},
		{
			name: "ReadAsyncWorkflowMessages",
			call: func() error {
				resp, err := store.ReadAsyncWorkflowMessages(ctx, &ReadAsyncWorkflowMessagesRequest{})
				assert.Nil(t, resp)
				return err
			},
		},
		{
			name: "GetAsyncWorkflowAckLevels",
			call: func() error {
				resp, err := store.GetAsyncWorkflowAckLevels(ctx, &GetAsyncWorkflowAckLevelsRequest{})
				assert.Nil(t, resp)
				return err
			},
		},
		{
			name: "UpdateAsyncWorkflowAckLevel",
			call: func() error {
				return store.UpdateAsyncWorkflowAckLevel(ctx, &InternalUpdateAsyncWorkflowAckLevelRequest{})
			},
		},
		{
			name: "RangeDeleteAsyncWorkflowMessages",
			call: func() error {
				return store.RangeDeleteAsyncWorkflowMessages(ctx, &RangeDeleteAsyncWorkflowMessagesRequest{})
			},
		},
		{
			name: "EnqueueAsyncWorkflowMessageToDLQ",
			call: func() error {
				return store.EnqueueAsyncWorkflowMessageToDLQ(ctx, &InternalEnqueueAsyncWorkflowMessageToDLQRequest{})
			},
		},
		{
			name: "ReadAsyncWorkflowMessagesFromDLQ",
			call: func() error {
				resp, err := store.ReadAsyncWorkflowMessagesFromDLQ(ctx, &ReadAsyncWorkflowMessagesFromDLQRequest{})
				assert.Nil(t, resp)
				return err
			},
		},
		{
			name: "RangeDeleteAsyncWorkflowMessagesFromDLQ",
			call: func() error {
				return store.RangeDeleteAsyncWorkflowMessagesFromDLQ(ctx, &RangeDeleteAsyncWorkflowMessagesFromDLQRequest{})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrAsyncWorkflowQueueNotImplemented), "got %v", err)

			// A plain error, never a client fault: the metered wrapper must count this as a persistence failure.
			var badRequest *types.BadRequestError
			assert.False(t, errors.As(err, &badRequest), "must not be a BadRequestError")
		})
	}
}

func TestUnimplementedAsyncWorkflowQueueStore_GetNameAndClose(t *testing.T) {
	tests := []struct {
		name      string
		storeName string
	}{
		{name: "nosql", storeName: "nosql"},
		{name: "sql", storeName: "sql"},
		{name: "arbitrary name", storeName: "custom-backend"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := NewUnimplementedAsyncWorkflowQueueStore(tc.storeName)

			assert.Equal(t, tc.storeName, store.GetName())
			assert.NotPanics(t, store.Close)
			assert.Equal(t, tc.storeName, store.GetName(), "Close must be a no-op")
		})
	}
}
