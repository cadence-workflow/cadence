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

package nosql

import (
	"sync"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
)

type (
	// Factory vends datastore implementations backed by cassandra
	Factory struct {
		sync.RWMutex
		cfg              config.ShardedNoSQL
		clusterName      string
		logger           log.Logger
		metricsClient    metrics.Client
		execStoreFactory *executionStoreFactory
		dc               *persistence.DynamicConfiguration
		parser           serialization.Parser
		taskSerializer   serialization.TaskSerializer
	}

	executionStoreFactory struct {
		logger            log.Logger
		shardedNosqlStore shardedNosqlStore
		taskSerializer    serialization.TaskSerializer
	}
)

// NewFactory returns an instance of a factory object which can be used to create
// datastores that are backed by cassandra
func NewFactory(cfg config.ShardedNoSQL, clusterName string, logger log.Logger, metricsClient metrics.Client, taskSerializer serialization.TaskSerializer, parser serialization.Parser, dc *persistence.DynamicConfiguration) *Factory {
	return &Factory{
		cfg:            cfg,
		clusterName:    clusterName,
		logger:         logger,
		metricsClient:  metricsClient,
		taskSerializer: taskSerializer,
		dc:             dc,
		parser:         parser,
	}
}

// NewTaskStore returns a new task store
func (f *Factory) NewTaskStore() (persistence.TaskStore, error) {
	return newNoSQLTaskStore(f.cfg, f.logger, f.metricsClient, f.dc)
}

// NewShardStore returns a new shard store
func (f *Factory) NewShardStore() (persistence.ShardStore, error) {
	return newNoSQLShardStore(f.cfg, f.clusterName, f.logger, f.metricsClient, f.dc, f.parser)
}

// NewHistoryStore returns a new history store
func (f *Factory) NewHistoryStore() (persistence.HistoryStore, error) {
	return newNoSQLHistoryStore(f.cfg, f.logger, f.metricsClient, f.dc)
}

// NewDomainStore returns a metadata store that understands only v2
func (f *Factory) NewDomainStore() (persistence.DomainStore, error) {
	return newNoSQLDomainStore(f.cfg, f.clusterName, f.logger, f.metricsClient, f.dc)
}

// NewDomainAuditStore returns a domain audit store
func (f *Factory) NewDomainAuditStore() (persistence.DomainAuditStore, error) {
	return newNoSQLDomainAuditStore(f.cfg, f.logger, f.metricsClient, f.dc)
}

// NewExecutionStore returns an ExecutionStore for a given shardID
func (f *Factory) NewExecutionStore(shardID int) (persistence.ExecutionStore, error) {
	factory, err := f.executionStoreFactory()
	if err != nil {
		return nil, err
	}
	return factory.new(shardID)
}

// NewVisibilityStore returns a visibility store
func (f *Factory) NewVisibilityStore(sortByCloseTime bool) (persistence.VisibilityStore, error) {
	return newNoSQLVisibilityStore(sortByCloseTime, f.cfg, f.logger, f.metricsClient, f.dc)
}

// NewQueue returns a new queue backed by cassandra
func (f *Factory) NewQueue(queueType persistence.QueueType) (persistence.QueueStore, error) {
	return newNoSQLQueueStore(f.cfg, f.logger, f.metricsClient, queueType, f.dc)
}

// NewConfigStore returns a new config store
func (f *Factory) NewConfigStore() (persistence.ConfigStore, error) {
	return NewNoSQLConfigStore(f.cfg, f.logger, f.metricsClient, f.dc)
}

// Close closes the factory
func (f *Factory) Close() {
	f.Lock()
	defer f.Unlock()
	if f.execStoreFactory != nil {
		f.execStoreFactory.close()
	}
}

func (f *Factory) executionStoreFactory() (*executionStoreFactory, error) {
	f.RLock()
	if f.execStoreFactory != nil {
		f.RUnlock()
		return f.execStoreFactory, nil
	}
	f.RUnlock()
	f.Lock()
	defer f.Unlock()
	if f.execStoreFactory != nil {
		return f.execStoreFactory, nil
	}

	factory, err := newExecutionStoreFactory(f.cfg, f.logger, f.metricsClient, f.taskSerializer, f.dc)
	if err != nil {
		return nil, err
	}
	f.execStoreFactory = factory
	return f.execStoreFactory, nil
}

// newExecutionStoreFactory is used to create an instance of ExecutionStoreFactory implementation
func newExecutionStoreFactory(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	taskSerializer serialization.TaskSerializer,
	dc *persistence.DynamicConfiguration,
) (*executionStoreFactory, error) {
	s, err := newShardedNosqlStore(cfg, logger, metricsClient, dc)
	if err != nil {
		return nil, err
	}
	return &executionStoreFactory{
		taskSerializer:    taskSerializer,
		logger:            logger,
		shardedNosqlStore: s,
	}, nil
}

func (f *executionStoreFactory) close() {
	f.shardedNosqlStore.Close()
}

// new implements ExecutionStoreFactory interface
func (f *executionStoreFactory) new(shardID int) (persistence.ExecutionStore, error) {
	storeShard, err := f.shardedNosqlStore.GetStoreShardByHistoryShard(shardID)
	if err != nil {
		return nil, err
	}
	pmgr, err := NewExecutionStore(shardID, storeShard.db, f.logger, f.taskSerializer, storeShard.dc)
	if err != nil {
		return nil, err
	}
	return pmgr, nil
}
