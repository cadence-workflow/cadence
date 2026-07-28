// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package history

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/.gen/go/sqlblobs"
	historyclient "github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

const testNumShards = 1024

func newTestMessage(partitionKey string) *sqlblobs.AsyncRequestMessage {
	reqType := sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest
	enc := string(constants.EncodingTypeThriftRW)
	return &sqlblobs.AsyncRequestMessage{
		PartitionKey: common.StringPtr(partitionKey),
		Type:         &reqType,
		Encoding:     &enc,
		Payload:      []byte("hello-payload"),
	}
}

func TestShardDerivationDeterministic(t *testing.T) {
	const wfID = "my-workflow-id"
	first := common.WorkflowIDToHistoryShard(wfID, testNumShards)
	for i := 0; i < 100; i++ {
		assert.Equal(t, first, common.WorkflowIDToHistoryShard(wfID, testNumShards))
	}
}

func TestPublishHappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := historyclient.NewMockClient(ctrl)

	const (
		wfID      = "my-workflow-id"
		queueName = "q1"
	)
	expectedShard := common.WorkflowIDToHistoryShard(wfID, testNumShards)
	msg := newTestMessage(wfID)

	var captured *types.EnqueueAsyncWorkflowMessageRequest
	mockClient.EXPECT().
		EnqueueAsyncWorkflowMessage(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *types.EnqueueAsyncWorkflowMessageRequest, _ ...interface{}) (*types.EnqueueAsyncWorkflowMessageResponse, error) {
			captured = req
			return &types.EnqueueAsyncWorkflowMessageResponse{MessageID: 7}, nil
		}).
		Times(1)

	p := newProducer(queueName, mockClient, testNumShards, log.NewNoop(), metrics.NewNoopMetricsClient())
	require.NoError(t, p.Publish(context.Background(), msg))

	require.NotNil(t, captured)
	assert.Equal(t, int32(expectedShard), captured.ShardID)
	assert.Equal(t, wfID, captured.PartitionKey)
	assert.Equal(t, string(constants.EncodingTypeThriftRW), captured.Encoding)

	// Payload should ThriftRW-decode back to the original message.
	var decoded sqlblobs.AsyncRequestMessage
	require.NoError(t, codec.NewThriftRWEncoder().Decode(captured.Payload, &decoded))
	assert.Equal(t, msg.GetPartitionKey(), decoded.GetPartitionKey())
	assert.Equal(t, msg.GetType(), decoded.GetType())
	assert.Equal(t, msg.GetEncoding(), decoded.GetEncoding())
	assert.Equal(t, msg.GetPayload(), decoded.GetPayload())
}

func TestPublishWrongMessageType(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := historyclient.NewMockClient(ctrl)

	p := newProducer("q1", mockClient, testNumShards, log.NewNoop(), metrics.NewNoopMetricsClient())
	err := p.Publish(context.Background(), "not-a-message")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected message type")
}

func TestPublishRPCErrorPropagated(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := historyclient.NewMockClient(ctrl)

	wantErr := errors.New("rpc boom")
	mockClient.EXPECT().
		EnqueueAsyncWorkflowMessage(gomock.Any(), gomock.Any()).
		Return(nil, wantErr).
		Times(1)

	p := newProducer("q1", mockClient, testNumShards, log.NewNoop(), metrics.NewNoopMetricsClient())
	err := p.Publish(context.Background(), newTestMessage("wf"))
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}
