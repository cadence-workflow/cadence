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

package nosql

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

type nosqlSemaphoreMetadataStore struct {
	nosqlStore
}

// newNoSQLSemaphoreMetadataStore is used to create an instance of SemaphoreMetadataStore implementation
func newNoSQLSemaphoreMetadataStore(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	dc *persistence.DynamicConfiguration,
) (persistence.SemaphoreMetadataStore, error) {
	shardedStore, err := newShardedNosqlStore(cfg, logger, metricsClient, dc, false)
	if err != nil {
		return nil, err
	}
	return &nosqlSemaphoreMetadataStore{
		nosqlStore: shardedStore.GetDefaultShard(),
	}, nil
}

// CreateSemaphore creates a new semaphore metadata entry. It is conflict-detecting:
// if a semaphore with the same (DomainID, SemaphoreName) already exists, it returns
// a *persistence.ConditionFailedError rather than succeeding or overwriting.
func (m *nosqlSemaphoreMetadataStore) CreateSemaphore(
	ctx context.Context,
	semaphore *persistence.SemaphoreMetadata,
) error {
	row := &nosqlplugin.SemaphoreMetadataRow{
		DomainID:      semaphore.DomainID,
		SemaphoreName: semaphore.SemaphoreName,
		Size:          semaphore.Size,
		BucketSize:    semaphore.BucketSize,
		CreatedTime:   semaphore.CreatedTime,
	}

	if err := m.db.InsertSemaphoreMetadata(ctx, row); err != nil {
		if _, ok := err.(*nosqlplugin.ConditionFailure); ok {
			return &persistence.ConditionFailedError{
				Msg: fmt.Sprintf("Semaphore %q already exists in domain %q. Condition Failed", semaphore.SemaphoreName, semaphore.DomainID),
			}
		}
		return convertCommonErrors(m.db, "CreateSemaphore", err)
	}
	return nil
}

// GetSemaphore retrieves a single semaphore's metadata
func (m *nosqlSemaphoreMetadataStore) GetSemaphore(
	ctx context.Context,
	request *persistence.GetSemaphoreRequest,
) (*persistence.SemaphoreMetadata, error) {
	row, err := m.db.SelectSemaphoreMetadata(ctx, request.DomainID, request.SemaphoreName)
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetSemaphore", err)
	}
	return semaphoreRowToMetadata(row), nil
}

// ListSemaphores lists the semaphores in a domain, paginated
func (m *nosqlSemaphoreMetadataStore) ListSemaphores(
	ctx context.Context,
	request *persistence.ListSemaphoresRequest,
) (*persistence.ListSemaphoresResponse, error) {
	filter := &nosqlplugin.SemaphoreMetadataFilter{
		DomainID:      request.DomainID,
		PageSize:      request.PageSize,
		NextPageToken: request.NextPageToken,
	}

	rows, nextPageToken, err := m.db.SelectSemaphoreMetadataByDomain(ctx, filter)
	if err != nil {
		return nil, convertCommonErrors(m.db, "ListSemaphores", err)
	}

	semaphores := make([]*persistence.SemaphoreMetadata, 0, len(rows))
	for _, row := range rows {
		semaphores = append(semaphores, semaphoreRowToMetadata(row))
	}

	return &persistence.ListSemaphoresResponse{
		Semaphores:    semaphores,
		NextPageToken: nextPageToken,
	}, nil
}

func semaphoreRowToMetadata(row *nosqlplugin.SemaphoreMetadataRow) *persistence.SemaphoreMetadata {
	return &persistence.SemaphoreMetadata{
		DomainID:      row.DomainID,
		SemaphoreName: row.SemaphoreName,
		Size:          row.Size,
		BucketSize:    row.BucketSize,
		CreatedTime:   row.CreatedTime,
	}
}
