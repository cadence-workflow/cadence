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
	"time"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

// sqlSemaphoreTokenStore is a stub. The distributed semaphore feature is only
// supported on the NoSQL (Cassandra) persistence backend. These methods exist to
// satisfy the SemaphoreTokenStore interface and always return a not-supported error.
type sqlSemaphoreTokenStore struct {
	sqlStore
}

// newSQLSemaphoreTokenStore creates an instance of sqlSemaphoreTokenStore
func newSQLSemaphoreTokenStore(
	db sqlplugin.DB,
	logger log.Logger,
	parser serialization.Parser,
) (persistence.SemaphoreTokenStore, error) {
	return &sqlSemaphoreTokenStore{
		sqlStore: sqlStore{
			db:     db,
			logger: logger,
			parser: parser,
		},
	}, nil
}

func (m *sqlSemaphoreTokenStore) SeedSemaphoreTokens(
	ctx context.Context,
	request *persistence.SeedSemaphoreTokensRequest,
	updatedTime time.Time,
) error {
	return errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTokenStore) GrantSemaphoreToken(
	ctx context.Context,
	request *persistence.GrantSemaphoreTokenRequest,
	updatedTime time.Time,
) (*persistence.GrantSemaphoreTokenResponse, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTokenStore) ReleaseSemaphoreToken(
	ctx context.Context,
	request *persistence.ReleaseSemaphoreTokenRequest,
	updatedTime time.Time,
) (bool, error) {
	return false, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTokenStore) GetSemaphoreOwnershipByToken(
	ctx context.Context,
	request *persistence.GetSemaphoreOwnershipByTokenRequest,
) (*persistence.SemaphoreOwnership, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTokenStore) GetSemaphoreOwnershipByOwner(
	ctx context.Context,
	request *persistence.GetSemaphoreOwnershipByOwnerRequest,
) (*persistence.SemaphoreOwnership, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}

func (m *sqlSemaphoreTokenStore) ScanSemaphoreBucket(
	ctx context.Context,
	request *persistence.ScanSemaphoreBucketRequest,
) (*persistence.ScanSemaphoreBucketResponse, error) {
	return nil, errSemaphoreNotSupportedOnSQL()
}
