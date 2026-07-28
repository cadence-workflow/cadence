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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	historyclient "github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func TestRegistration(t *testing.T) {
	queueCtor, ok := provider.GetQueueProvider("history")
	require.True(t, ok, "history queue provider should be registered")
	require.NotNil(t, queueCtor)

	decoderCtor, ok := provider.GetDecoder("history")
	require.True(t, ok, "history decoder should be registered")
	require.NotNil(t, decoderCtor)
}

func TestNewQueueAndID(t *testing.T) {
	blob := &types.DataBlob{
		EncodingType: types.EncodingTypeJSON.Ptr(),
		Data:         []byte(`{"queueName":"q1"}`),
	}
	q, err := newQueue(newDecoder(blob))
	require.NoError(t, err)
	assert.Equal(t, "history::q1", q.ID())
}

func TestNewQueueBadConfig(t *testing.T) {
	// non-JSON encoding causes decode to fail
	blob := &types.DataBlob{
		EncodingType: types.EncodingTypeThriftRW.Ptr(),
		Data:         []byte(`{"queueName":"q1"}`),
	}
	_, err := newQueue(newDecoder(blob))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad config")
}

func TestDecoder(t *testing.T) {
	t.Run("json ok", func(t *testing.T) {
		blob := &types.DataBlob{
			EncodingType: types.EncodingTypeJSON.Ptr(),
			Data:         []byte(`{"queueName":"q1"}`),
		}
		var cfg queueConfig
		require.NoError(t, newDecoder(blob).Decode(&cfg))
		assert.Equal(t, "q1", cfg.QueueName)
	})

	t.Run("non-json errors", func(t *testing.T) {
		blob := &types.DataBlob{
			EncodingType: types.EncodingTypeThriftRW.Ptr(),
			Data:         []byte(`{"queueName":"q1"}`),
		}
		var cfg queueConfig
		err := newDecoder(blob).Decode(&cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported encoding type")
	})
}

// CreateConsumer always returns a no-op consumer: consumption happens inside the
// history service (per-shard queue processor), not through the worker's
// ConsumerManager. The no-op keeps the ConsumerManager's refresh loop quiet.
func TestCreateConsumerNoop(t *testing.T) {
	q := &queueImpl{config: &queueConfig{QueueName: "q1"}}

	c, err := q.CreateConsumer(&provider.Params{
		Logger:        log.NewNoop(),
		MetricsClient: metrics.NewNoopMetricsClient(),
	})
	require.NoError(t, err)
	require.NotNil(t, c)

	// Start/Stop are no-ops and must be safe to call.
	require.NoError(t, c.Start())
	c.Stop()
}

func TestCreateProducerValidation(t *testing.T) {
	q := &queueImpl{config: &queueConfig{QueueName: "q1"}}

	t.Run("nil history client", func(t *testing.T) {
		_, err := q.CreateProducer(&provider.Params{
			Logger:           log.NewNoop(),
			MetricsClient:    metrics.NewNoopMetricsClient(),
			NumHistoryShards: 4,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "history client is required")
	})

	t.Run("non-positive shards", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		_, err := q.CreateProducer(&provider.Params{
			Logger:           log.NewNoop(),
			MetricsClient:    metrics.NewNoopMetricsClient(),
			HistoryClient:    historyclient.NewMockClient(ctrl),
			NumHistoryShards: 0,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid number of history shards")
	})
}
