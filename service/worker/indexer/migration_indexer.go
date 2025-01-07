// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package indexer

import (
	"github.com/uber/cadence/common"
	es "github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
)

// NewMigrationIndexer create a new Indexer that can index to both ES and OS
func NewMigrationIndexer(
	config *Config,
	client messaging.Client,
	primaryClient es.GenericClient,
	secondaryClient es.GenericClient,
	visibilityName string,
	logger log.Logger,
	metricsClient metrics.Client,
) *Indexer {
	logger = logger.WithTags(tag.ComponentIndexer)

	visibilityProcessor, err := newESDualProcessor(processorName, config, primaryClient, secondaryClient, logger, metricsClient)
	if err != nil {
		logger.Fatal("Index ES processor state changed", tag.LifeCycleStartFailed, tag.Error(err))
	}

	consumer, err := client.NewConsumer(common.VisibilityAppName, getConsumerName(visibilityName))
	if err != nil {
		logger.Fatal("Index consumer state changed", tag.LifeCycleStartFailed, tag.Error(err))
	}

	return &Indexer{
		config:              config,
		esIndexName:         visibilityName,
		consumer:            consumer,
		logger:              logger.WithTags(tag.ComponentIndexerProcessor),
		scope:               metricsClient.Scope(metrics.IndexProcessorScope),
		shutdownCh:          make(chan struct{}),
		visibilityProcessor: visibilityProcessor,
		msgEncoder:          defaultEncoder,
	}
}
