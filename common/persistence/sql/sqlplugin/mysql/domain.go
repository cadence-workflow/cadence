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

package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	createDomainQuery = `INSERT INTO 
 domains (id, name, is_global, data, data_encoding)
 VALUES(?, ?, ?, ?, ?)`

	updateDomainQuery = `UPDATE domains 
 SET name = ?, data = ?, data_encoding = ?
 WHERE shard_id=54321 AND id = ?`

	getDomainPart = `SELECT id, name, is_global, data, data_encoding FROM domains`

	getDomainByIDQuery   = getDomainPart + ` WHERE shard_id=? AND id = ?`
	getDomainByNameQuery = getDomainPart + ` WHERE shard_id=? AND name = ?`

	listDomainsQuery      = getDomainPart + ` WHERE shard_id=? ORDER BY id LIMIT ?`
	listDomainsRangeQuery = getDomainPart + ` WHERE shard_id=? AND id > ? ORDER BY id LIMIT ?`

	deleteDomainByIDQuery   = `DELETE FROM domains WHERE shard_id=? AND id = ?`
	deleteDomainByNameQuery = `DELETE FROM domains WHERE shard_id=? AND name = ?`

	getDomainMetadataQuery    = `SELECT notification_version FROM domain_metadata`
	lockDomainMetadataQuery   = `SELECT notification_version FROM domain_metadata FOR UPDATE`
	updateDomainMetadataQuery = `UPDATE domain_metadata SET notification_version = ? WHERE notification_version = ?`
)

const (
	shardID = 54321
)

var errMissingArgs = errors.New("missing one or more args for API")

// InsertIntoDomain inserts a single row into domains table
func (mdb *DB) InsertIntoDomain(ctx context.Context, row *sqlplugin.DomainRow) (sql.Result, error) {
	return mdb.driver.ExecContext(ctx, sqlplugin.DbDefaultShard, createDomainQuery, row.ID, row.Name, row.IsGlobal, row.Data, row.DataEncoding)
}

// UpdateDomain updates a single row in domains table
func (mdb *DB) UpdateDomain(ctx context.Context, row *sqlplugin.DomainRow) (sql.Result, error) {
	return mdb.driver.ExecContext(ctx, sqlplugin.DbDefaultShard, updateDomainQuery, row.Name, row.Data, row.DataEncoding, row.ID)
}

// SelectFromDomain reads one or more rows from domains table
func (mdb *DB) SelectFromDomain(ctx context.Context, filter *sqlplugin.DomainFilter) ([]sqlplugin.DomainRow, error) {
	switch {
	case filter.ID != nil || filter.Name != nil:
		return mdb.selectFromDomain(ctx, filter)
	case filter.PageSize != nil && *filter.PageSize > 0:
		return mdb.selectAllFromDomain(ctx, filter)
	default:
		return nil, errMissingArgs
	}
}

func (mdb *DB) selectFromDomain(ctx context.Context, filter *sqlplugin.DomainFilter) ([]sqlplugin.DomainRow, error) {
	var err error
	var row sqlplugin.DomainRow
	switch {
	case filter.ID != nil:
		err = mdb.driver.GetContext(ctx, sqlplugin.DbDefaultShard, &row, getDomainByIDQuery, shardID, *filter.ID)
	case filter.Name != nil:
		err = mdb.driver.GetContext(ctx, sqlplugin.DbDefaultShard, &row, getDomainByNameQuery, shardID, *filter.Name)
	}
	if err != nil {
		return nil, err
	}
	return []sqlplugin.DomainRow{row}, err
}

func (mdb *DB) selectAllFromDomain(ctx context.Context, filter *sqlplugin.DomainFilter) ([]sqlplugin.DomainRow, error) {
	var err error
	var rows []sqlplugin.DomainRow
	switch {
	case filter.GreaterThanID != nil:
		err = mdb.driver.SelectContext(ctx, sqlplugin.DbDefaultShard, &rows, listDomainsRangeQuery, shardID, *filter.GreaterThanID, *filter.PageSize)
	default:
		err = mdb.driver.SelectContext(ctx, sqlplugin.DbDefaultShard, &rows, listDomainsQuery, shardID, filter.PageSize)
	}
	return rows, err
}

// DeleteFromDomain deletes a single row in domains table
func (mdb *DB) DeleteFromDomain(ctx context.Context, filter *sqlplugin.DomainFilter) (sql.Result, error) {
	var err error
	var result sql.Result
	switch {
	case filter.ID != nil:
		result, err = mdb.driver.ExecContext(ctx, sqlplugin.DbDefaultShard, deleteDomainByIDQuery, shardID, filter.ID)
	default:
		result, err = mdb.driver.ExecContext(ctx, sqlplugin.DbDefaultShard, deleteDomainByNameQuery, shardID, filter.Name)
	}
	return result, err
}

// LockDomainMetadata acquires a write lock on a single row in domain_metadata table
func (mdb *DB) LockDomainMetadata(ctx context.Context) error {
	var row sqlplugin.DomainMetadataRow
	err := mdb.driver.GetContext(ctx, sqlplugin.DbDefaultShard, &row.NotificationVersion, lockDomainMetadataQuery)
	return err
}

// SelectFromDomainMetadata reads a single row in domain_metadata table
func (mdb *DB) SelectFromDomainMetadata(ctx context.Context) (*sqlplugin.DomainMetadataRow, error) {
	var row sqlplugin.DomainMetadataRow
	err := mdb.driver.GetContext(ctx, sqlplugin.DbDefaultShard, &row.NotificationVersion, getDomainMetadataQuery)
	return &row, err
}

// UpdateDomainMetadata updates a single row in domain_metadata table
func (mdb *DB) UpdateDomainMetadata(ctx context.Context, row *sqlplugin.DomainMetadataRow) (sql.Result, error) {
	return mdb.driver.ExecContext(ctx, sqlplugin.DbDefaultShard, updateDomainMetadataQuery, row.NotificationVersion+1, row.NotificationVersion)
}
