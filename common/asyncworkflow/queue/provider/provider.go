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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -self_package github.com/uber/cadence/common/asyncworkflow/queue/provider

package provider

import (
	"fmt"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/syncmap"
	"github.com/uber/cadence/common/types"
)

type (
	Params struct {
		Logger         log.Logger
		MetricsClient  metrics.Client
		FrontendClient frontend.Client
		// HistoryClient and NumHistoryShards are required by history-backed queues, whose producer
		// routes each request to the history shard that owns its workflow id.
		HistoryClient    history.Client
		NumHistoryShards int
	}

	Decoder interface {
		Decode(any) error
	}

	Consumer interface {
		Start() error
		Stop()
	}

	// Queue is an interface for async queue
	Queue interface {
		ID() string
		CreateConsumer(*Params) (Consumer, error)
		CreateProducer(*Params) (messaging.Producer, error)
	}

	QueueConstructor func(Decoder) (Queue, error)

	DecoderConstructor func(*types.DataBlob) Decoder
)

var (
	queueConstructors   = syncmap.New[string, QueueConstructor]()
	decoderConstructors = syncmap.New[string, DecoderConstructor]()
)

// RegisterQueueProvider registers a queue constructor for a given queue type
func RegisterQueueProvider(queueType string, queueConstructor QueueConstructor) error {
	inserted := queueConstructors.Put(queueType, queueConstructor)
	if !inserted {
		return fmt.Errorf("queue type %v already registered", queueType)
	}
	return nil
}

// GetQueueProvider returns a queue constructor for a given queue type
func GetQueueProvider(queueType string) (QueueConstructor, bool) {
	return queueConstructors.Get(queueType)
}

// RegisterDecoder registers a decoder constructor for a given queue type
func RegisterDecoder(queueType string, decoderConstructor DecoderConstructor) error {
	inserted := decoderConstructors.Put(queueType, decoderConstructor)
	if !inserted {
		return fmt.Errorf("decoder type %v already registered", queueType)
	}
	return nil
}

// GetDecoder returns a decoder constructor for a given queue type
func GetDecoder(queueType string) (DecoderConstructor, bool) {
	return decoderConstructors.Get(queueType)
}
