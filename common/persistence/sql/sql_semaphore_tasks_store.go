// Copyright (c) 2025 Uber Technologies, Inc.
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

package sql

import (
	"context"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

// sqlSemaphoreTaskStore is a stub. The distributed semaphore feature is only supported on the
// NoSQL (Cassandra) persistence backend. These methods exist to satisfy the SemaphoreTaskStore
// interface and always return a not-supported error.
type sqlSemaphoreTaskStore struct {
	sqlStore
}

// newSQLSemaphoreTaskStore creates an instance of sqlSemaphoreTaskStore
func newSQLSemaphoreTaskStore(
	db sqlplugin.DB,
	logger log.Logger,
	parser serialization.Parser,
) (persistence.SemaphoreTaskStore, error) {
	return &sqlSemaphoreTaskStore{
		sqlStore: sqlStore{
			db:     db,
			logger: logger,
			parser: parser,
		},
	}, nil
}

func (m *sqlSemaphoreTaskStore) ClaimSemaphoreTaskBucket(
	ctx context.Context,
	request *persistence.ClaimSemaphoreTaskBucketRequest,
) (*persistence.ClaimSemaphoreTaskBucketResponse, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTaskStore) GetSemaphoreTaskBucketState(
	ctx context.Context,
	request *persistence.GetSemaphoreTaskBucketStateRequest,
) (*persistence.GetSemaphoreTaskBucketStateResponse, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTaskStore) UpdateSemaphoreTaskBucketState(
	ctx context.Context,
	request *persistence.UpdateSemaphoreTaskBucketStateRequest,
) (*persistence.UpdateSemaphoreTaskBucketStateResponse, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTaskStore) CreateSemaphoreTasks(
	ctx context.Context,
	request *persistence.CreateSemaphoreTasksRequest,
) (*persistence.CreateSemaphoreTasksResponse, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTaskStore) GetSemaphoreTasks(
	ctx context.Context,
	request *persistence.GetSemaphoreTasksRequest,
) (*persistence.GetSemaphoreTasksResponse, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTaskStore) RangeCompleteSemaphoreTasks(
	ctx context.Context,
	request *persistence.RangeCompleteSemaphoreTasksRequest,
) (*persistence.RangeCompleteSemaphoreTasksResponse, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTaskStore) GetSemaphoreTasksCount(
	ctx context.Context,
	request *persistence.GetSemaphoreTasksCountRequest,
) (*persistence.GetSemaphoreTasksCountResponse, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}
