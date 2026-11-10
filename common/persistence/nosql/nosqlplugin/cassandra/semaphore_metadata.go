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

package cassandra

import (
	"context"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

const (
	templateInsertSemaphoreMetadataQuery = `INSERT INTO semaphore_metadata (` +
		`domain_id, semaphore_name, size, bucket_size, created_time) ` +
		`VALUES(?, ?, ?, ?, ?) IF NOT EXISTS`

	templateSelectSemaphoreMetadataQuery = `SELECT ` +
		`domain_id, semaphore_name, size, bucket_size, created_time ` +
		`FROM semaphore_metadata ` +
		`WHERE domain_id = ? AND semaphore_name = ?`

	templateSelectSemaphoreMetadataByDomainQuery = `SELECT ` +
		`domain_id, semaphore_name, size, bucket_size, created_time ` +
		`FROM semaphore_metadata ` +
		`WHERE domain_id = ?`
)

// InsertSemaphoreMetadata creates a semaphore's metadata with a conflict-detecting
// INSERT ... IF NOT EXISTS (LWT). It does not overwrite: if a row with the same
// (domain_id, semaphore_name) already exists, it returns a ConditionFailure.
func (db *CDB) InsertSemaphoreMetadata(ctx context.Context, row *nosqlplugin.SemaphoreMetadataRow) error {
	query := db.session.Query(templateInsertSemaphoreMetadataQuery,
		row.DomainID,
		row.SemaphoreName,
		row.Size,
		row.BucketSize,
		row.CreatedTime,
	).WithContext(ctx)

	applied, err := query.MapScanCAS(make(map[string]interface{}))
	if err != nil {
		return err
	}
	if !applied {
		return nosqlplugin.NewConditionFailure("InsertSemaphoreMetadata operation failed because the semaphore already exists")
	}
	return nil
}

// SelectSemaphoreMetadata returns a single semaphore's metadata by (domainID, semaphoreName).
func (db *CDB) SelectSemaphoreMetadata(ctx context.Context, domainID, semaphoreName string) (*nosqlplugin.SemaphoreMetadataRow, error) {
	row := &nosqlplugin.SemaphoreMetadataRow{}
	query := db.session.Query(templateSelectSemaphoreMetadataQuery, domainID, semaphoreName).WithContext(ctx)
	err := query.Scan(
		&row.DomainID,
		&row.SemaphoreName,
		&row.Size,
		&row.BucketSize,
		&row.CreatedTime,
	)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// SelectSemaphoreMetadataByDomain returns the semaphores in a domain, paginated.
func (db *CDB) SelectSemaphoreMetadataByDomain(ctx context.Context, filter *nosqlplugin.SemaphoreMetadataFilter) ([]*nosqlplugin.SemaphoreMetadataRow, []byte, error) {
	query := db.session.Query(templateSelectSemaphoreMetadataByDomainQuery, filter.DomainID).WithContext(ctx)

	if filter.PageSize > 0 {
		query = query.PageSize(filter.PageSize)
	}
	if len(filter.NextPageToken) > 0 {
		query = query.PageState(filter.NextPageToken)
	}

	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectSemaphoreMetadataByDomain operation failed. Not able to create query iterator.",
		}
	}

	var rows []*nosqlplugin.SemaphoreMetadataRow
	row := &nosqlplugin.SemaphoreMetadataRow{}
	for iter.Scan(
		&row.DomainID,
		&row.SemaphoreName,
		&row.Size,
		&row.BucketSize,
		&row.CreatedTime,
	) {
		rows = append(rows, row)
		row = &nosqlplugin.SemaphoreMetadataRow{}

		if filter.PageSize > 0 && len(rows) >= filter.PageSize {
			break
		}
	}

	nextPageToken := iter.PageState()
	if err := iter.Close(); err != nil {
		return nil, nil, err
	}

	return rows, nextPageToken, nil
}
