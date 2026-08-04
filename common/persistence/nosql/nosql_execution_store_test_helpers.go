// Copyright (c) 2026 Uber Technologies, Inc.
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
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/serialization"
)

const testExecutionShardID = 1

// fakeShardedNosqlStore is a test double that always returns the same nosqlStore.
type fakeShardedNosqlStore struct {
	store  nosqlStore
	logger log.Logger
}

func (f *fakeShardedNosqlStore) GetStoreShardByHistoryShard(shardID int) (*nosqlStore, error) {
	return &f.store, nil
}

func (f *fakeShardedNosqlStore) GetStoreShardByTaskList(domainID string, taskListName string, taskType int) (*nosqlStore, error) {
	return &f.store, nil
}

func (f *fakeShardedNosqlStore) GetDefaultShard() nosqlStore {
	return f.store
}

func (f *fakeShardedNosqlStore) Close() {}

func (f *fakeShardedNosqlStore) GetName() string {
	if f.store.db != nil {
		return f.store.db.PluginName()
	}
	return "fake"
}

func (f *fakeShardedNosqlStore) GetShardingPolicy() shardingPolicy {
	return shardingPolicy{}
}

func (f *fakeShardedNosqlStore) GetLogger() log.Logger {
	if f.logger != nil {
		return f.logger
	}
	return f.store.logger
}

func (f *fakeShardedNosqlStore) GetMetricsClient() metrics.Client {
	return metrics.NewNoopMetricsClient()
}

func newTestNosqlExecutionStoreWithOptions(
	db nosqlplugin.DB,
	logger log.Logger,
	taskSerializer serialization.TaskSerializer,
	dc *persistence.DynamicConfiguration,
) *nosqlExecutionStore {
	if dc == nil {
		dc = &persistence.DynamicConfiguration{}
	}
	store := nosqlStore{logger: logger, db: db, dc: dc}
	return &nosqlExecutionStore{
		shardedNosqlStore: &fakeShardedNosqlStore{store: store, logger: logger},
		taskSerializer:    taskSerializer,
	}
}
