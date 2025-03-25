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
	"github.com/uber/cadence/common/constants"
	es "github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
)

// NewMigrationDualIndexer create a new Indexer that will be used during visibility migration
// When migrate from ES to OS, we will have this indexer to index to both ES and OS
func NewMigrationDualIndexer(config *Config,
	client messaging.Client,
	primaryClient es.GenericClient,
	secondaryClient es.GenericClient,
	primaryVisibilityName string,
	secondaryVisibilityName string,
	primaryConsumerName string,
	secondaryConsumerName string,
	logger log.Logger,
	metricsClient metrics.Client) *DualIndexer {

	logger = logger.WithTags(tag.ComponentIndexer)

	visibilityProcessor, err := newESProcessor(processorName, config, primaryClient, logger, metricsClient)
	if err != nil {
		logger.Fatal("Index ES processor state changed", tag.LifeCycleStartFailed, tag.Error(err))
	}

	if primaryConsumerName == "" {
		primaryConsumerName = getConsumerName(primaryVisibilityName)
	}
	consumer, err := client.NewConsumer(constants.VisibilityAppName, primaryConsumerName)
	if err != nil {
		logger.Fatal("Index consumer state changed", tag.LifeCycleStartFailed, tag.Error(err))
	}

	sourceIndexer := &Indexer{
		config:              config,
		esIndexName:         primaryVisibilityName,
		consumer:            consumer,
		logger:              logger.WithTags(tag.ComponentIndexerProcessor),
		scope:               metricsClient.Scope(metrics.IndexProcessorScope),
		shutdownCh:          make(chan struct{}),
		visibilityProcessor: visibilityProcessor,
		msgEncoder:          defaultEncoder,
	}

	secondaryVisibilityProcessor, err := newESProcessor(migrationProcessorName, config, secondaryClient, logger, metricsClient)
	if err != nil {
		logger.Fatal("Migration Index ES processor state changed", tag.LifeCycleStartFailed, tag.Error(err))
	}

	if secondaryConsumerName == "" {
		secondaryConsumerName = getConsumerName(primaryVisibilityName)
	}
	secondaryConsumer, err := client.NewConsumer(constants.VisibilityAppName, secondaryConsumerName)
	if err != nil {
		logger.Fatal("Migration Index consumer state changed", tag.LifeCycleStartFailed, tag.Error(err))
	}

	destIndexer := &Indexer{
		config:              config,
		esIndexName:         secondaryVisibilityName,
		consumer:            secondaryConsumer,
		logger:              logger.WithTags(tag.ComponentIndexerProcessor),
		scope:               metricsClient.Scope(metrics.IndexProcessorScope),
		shutdownCh:          make(chan struct{}),
		visibilityProcessor: secondaryVisibilityProcessor,
		msgEncoder:          defaultEncoder,
	}

	return &DualIndexer{
		SourceIndexer: sourceIndexer,
		DestIndexer:   destIndexer,
	}
}

func (i *DualIndexer) Start() error {
	if err := i.SourceIndexer.Start(); err != nil {
		i.SourceIndexer.Stop()
		return err
	}

	if err := i.DestIndexer.Start(); err != nil {
		i.DestIndexer.Stop()
		return err
	}

	return nil
}

func (i *DualIndexer) Stop() {
	i.SourceIndexer.Stop()
	i.DestIndexer.Stop()
}
