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
	"fmt"

	"github.com/uber/cadence/.gen/go/sqlblobs"
	historyclient "github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

type (
	producerImpl struct {
		historyClient    historyclient.Client
		numHistoryShards int
		encoder          codec.BinaryEncoder
		logger           log.Logger
		metricsClient    metrics.Client
	}
)

var _ messaging.Producer = (*producerImpl)(nil)

func newProducer(
	queueName string,
	historyClient historyclient.Client,
	numHistoryShards int,
	logger log.Logger,
	metricsClient metrics.Client,
) *producerImpl {
	return &producerImpl{
		historyClient:    historyClient,
		numHistoryShards: numHistoryShards,
		encoder:          codec.NewThriftRWEncoder(),
		logger:           logger.WithTags(tag.AsyncWFQueueID((&queueConfig{QueueName: queueName}).ID())),
		metricsClient:    metricsClient,
	}
}

// Publish enqueues an async workflow request onto the owning history shard via RPC,
// which persists it to Cassandra.
func (p *producerImpl) Publish(ctx context.Context, message interface{}) error {
	msg, ok := message.(*sqlblobs.AsyncRequestMessage)
	if !ok {
		return fmt.Errorf("unexpected message type %T, expected *sqlblobs.AsyncRequestMessage", message)
	}

	partitionKey := msg.GetPartitionKey()
	shardID := common.WorkflowIDToHistoryShard(partitionKey, p.numHistoryShards)

	payload, err := p.encoder.Encode(msg)
	if err != nil {
		p.logger.Error("Failed to serialize async workflow message", tag.Error(err))
		return err
	}

	_, err = p.historyClient.EnqueueAsyncWorkflowMessage(ctx, &types.EnqueueAsyncWorkflowMessageRequest{
		ShardID:      int32(shardID),
		Payload:      payload,
		Encoding:     string(constants.EncodingTypeThriftRW),
		PartitionKey: partitionKey,
	})
	if err != nil {
		p.logger.Warn("Failed to enqueue async workflow message to history",
			tag.ShardID(shardID),
			tag.WorkflowID(partitionKey),
			tag.Error(err))
		return err
	}
	return nil
}
