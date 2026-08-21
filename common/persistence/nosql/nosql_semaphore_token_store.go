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
	"time"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

type nosqlSemaphoreTokenStore struct {
	nosqlStore
}

// newNoSQLSemaphoreTokenStore is used to create an instance of SemaphoreTokenStore implementation
func newNoSQLSemaphoreTokenStore(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	dc *persistence.DynamicConfiguration,
) (persistence.SemaphoreTokenStore, error) {
	shardedStore, err := newShardedNosqlStore(cfg, logger, metricsClient, dc, false)
	if err != nil {
		return nil, err
	}
	return &nosqlSemaphoreTokenStore{
		nosqlStore: shardedStore.GetDefaultShard(),
	}, nil
}

// SeedSemaphoreTokens seeds a bucket with free token rows for the requested slot ids.
func (m *nosqlSemaphoreTokenStore) SeedSemaphoreTokens(
	ctx context.Context,
	request *persistence.SeedSemaphoreTokensRequest,
	updatedTime time.Time,
) error {
	rows := make([]*nosqlplugin.SemaphoreTokenRow, 0, len(request.TokenIDs))
	for _, tokenID := range request.TokenIDs {
		rows = append(rows, &nosqlplugin.SemaphoreTokenRow{
			DomainID:      request.DomainID,
			SemaphoreName: request.SemaphoreName,
			Bucket:        request.Bucket,
			TokenID:       tokenID,
			UpdatedTime:   updatedTime,
		})
	}
	if err := m.db.InsertSemaphoreTokens(ctx, rows); err != nil {
		return convertCommonErrors(m.db, "SeedSemaphoreTokens", err)
	}
	return nil
}

// GrantSemaphoreToken claims a free slot for an owner. A false applied is
// control flow, not an error, and alreadyHeldToken says why the grant was
// refused: > 0 is the token this owner already holds (reuse it), 0 means the
// slot is taken by someone else (try another).
func (m *nosqlSemaphoreTokenStore) GrantSemaphoreToken(
	ctx context.Context,
	request *persistence.GrantSemaphoreTokenRequest,
	updatedTime time.Time,
) (bool, int, error) {
	row := &nosqlplugin.SemaphoreTokenRow{
		DomainID:      request.DomainID,
		SemaphoreName: request.SemaphoreName,
		Bucket:        request.Bucket,
		TokenID:       request.TokenID,
		OwnerID:       request.OwnerID,
		UpdatedTime:   updatedTime,
	}
	result, err := m.db.GrantSemaphoreToken(ctx, row)
	if err != nil {
		return false, 0, convertCommonErrors(m.db, "GrantSemaphoreToken", err)
	}
	return result.Applied, result.AlreadyHeldToken, nil
}

// ReleaseSemaphoreToken frees a slot if it is still held by the owner. The
// applied bool is control flow, not an error: applied == false is a no-op.
func (m *nosqlSemaphoreTokenStore) ReleaseSemaphoreToken(
	ctx context.Context,
	request *persistence.ReleaseSemaphoreTokenRequest,
	updatedTime time.Time,
) (bool, error) {
	row := &nosqlplugin.SemaphoreTokenRow{
		DomainID:      request.DomainID,
		SemaphoreName: request.SemaphoreName,
		Bucket:        request.Bucket,
		TokenID:       request.TokenID,
		OwnerID:       request.OwnerID,
		UpdatedTime:   updatedTime,
	}
	applied, err := m.db.ReleaseSemaphoreToken(ctx, row)
	if err != nil {
		return false, convertCommonErrors(m.db, "ReleaseSemaphoreToken", err)
	}
	return applied, nil
}

// GetSemaphoreTokenByID reads a slot's forward row (holder) by token id.
func (m *nosqlSemaphoreTokenStore) GetSemaphoreTokenByID(
	ctx context.Context,
	request *persistence.GetSemaphoreTokenByIDRequest,
) (*persistence.SemaphoreToken, error) {
	row, err := m.db.SelectSemaphoreTokenByID(ctx, request.DomainID, request.SemaphoreName, request.Bucket, request.TokenID)
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetSemaphoreTokenByID", err)
	}
	return semaphoreTokenRowToToken(row), nil
}

// GetSemaphoreTokenByOwner reads a hold's reverse row (held token) by owner id.
func (m *nosqlSemaphoreTokenStore) GetSemaphoreTokenByOwner(
	ctx context.Context,
	request *persistence.GetSemaphoreTokenByOwnerRequest,
) (*persistence.SemaphoreToken, error) {
	row, err := m.db.SelectSemaphoreTokenByOwner(ctx, request.DomainID, request.SemaphoreName, request.Bucket, request.OwnerID)
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetSemaphoreTokenByOwner", err)
	}
	return semaphoreTokenRowToToken(row), nil
}

// ScanSemaphoreBucket scans a bucket partition (both row kinds), paginated.
func (m *nosqlSemaphoreTokenStore) ScanSemaphoreBucket(
	ctx context.Context,
	request *persistence.ScanSemaphoreBucketRequest,
) (*persistence.ScanSemaphoreBucketResponse, error) {
	filter := &nosqlplugin.SemaphoreTokenFilter{
		DomainID:      request.DomainID,
		SemaphoreName: request.SemaphoreName,
		Bucket:        request.Bucket,
		PageSize:      request.PageSize,
		NextPageToken: request.NextPageToken,
	}

	rows, nextPageToken, err := m.db.SelectSemaphoreBucketRows(ctx, filter)
	if err != nil {
		return nil, convertCommonErrors(m.db, "ScanSemaphoreBucket", err)
	}

	tokens := make([]*persistence.SemaphoreToken, 0, len(rows))
	for _, row := range rows {
		tokens = append(tokens, semaphoreTokenRowToToken(row))
	}

	return &persistence.ScanSemaphoreBucketResponse{
		Rows:          tokens,
		NextPageToken: nextPageToken,
	}, nil
}

func semaphoreTokenRowToToken(row *nosqlplugin.SemaphoreTokenRow) *persistence.SemaphoreToken {
	return &persistence.SemaphoreToken{
		DomainID:      row.DomainID,
		SemaphoreName: row.SemaphoreName,
		Bucket:        row.Bucket,
		TokenID:       row.TokenID,
		OwnerID:       row.OwnerID,
		Holder:        row.Holder,
		HeldToken:     row.HeldToken,
		UpdatedTime:   row.UpdatedTime,
	}
}
