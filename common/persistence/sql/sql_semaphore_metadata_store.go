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
	"github.com/uber/cadence/common/types"
)

// sqlSemaphoreMetadataStore is a stub. The distributed semaphore feature is only
// supported on the NoSQL (Cassandra) persistence backend. These methods exist to
// satisfy the SemaphoreMetadataStore interface and always return a not-supported error.
type sqlSemaphoreMetadataStore struct {
	sqlStore
}

// newSQLSemaphoreMetadataStore creates an instance of sqlSemaphoreMetadataStore
func newSQLSemaphoreMetadataStore(
	db sqlplugin.DB,
	logger log.Logger,
	parser serialization.Parser,
) (persistence.SemaphoreMetadataStore, error) {
	return &sqlSemaphoreMetadataStore{
		sqlStore: sqlStore{
			db:     db,
			logger: logger,
			parser: parser,
		},
	}, nil
}

func errSemaphoreNotSupportedOnSQL() error {
	return &types.BadRequestError{
		Message: "distributed semaphore is not supported on the SQL persistence backend",
	}
}

func (m *sqlSemaphoreMetadataStore) CreateSemaphore(
	ctx context.Context,
	semaphore *persistence.SemaphoreMetadata,
) error {
	return errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreMetadataStore) GetSemaphore(
	ctx context.Context,
	request *persistence.GetSemaphoreRequest,
) (*persistence.SemaphoreMetadata, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreMetadataStore) ListSemaphores(
	ctx context.Context,
	request *persistence.ListSemaphoresRequest,
) (*persistence.ListSemaphoresResponse, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}
