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

package postgres

import (
	"context"
	"database/sql"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	_insertDomainAuditLogQuery = `INSERT INTO domain_audit_log (
		domain_id, event_id, state_before, state_before_encoding, state_after, state_after_encoding,
		operation_type, created_time, last_updated_time, identity, identity_type, comment
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_selectDomainAuditLogsQuery = `SELECT
		event_id, domain_id, state_before, state_before_encoding, state_after, state_after_encoding,
		operation_type, created_time, last_updated_time, identity, identity_type, comment
	FROM domain_audit_log
	WHERE domain_id = $1 AND operation_type = $2 AND created_time >= $3
	AND (created_time < $4 OR (created_time = $4 AND event_id > $5))
	ORDER BY created_time DESC, event_id ASC
	LIMIT $6`
	_selectAllDomainAuditLogsQuery = `SELECT
		event_id, domain_id, state_before, state_before_encoding, state_after, state_after_encoding,
		operation_type, created_time, last_updated_time, identity, identity_type, comment
	FROM domain_audit_log
	WHERE domain_id = $1 AND operation_type = $2 AND created_time >= $3
	AND (created_time < $4 OR (created_time = $4 AND event_id > $5))
	ORDER BY created_time DESC, event_id ASC`
)

// InsertIntoDomainAuditLog inserts a single row into domain_audit_log table
func (pdb *db) InsertIntoDomainAuditLog(ctx context.Context, row *sqlplugin.DomainAuditLogRow) (sql.Result, error) {
	return pdb.driver.ExecContext(
		ctx,
		sqlplugin.DbDefaultShard,
		_insertDomainAuditLogQuery,
		row.DomainID,
		row.EventID,
		row.StateBefore,
		row.StateBeforeEncoding,
		row.StateAfter,
		row.StateAfterEncoding,
		row.OperationType,
		row.CreatedTime,
		row.LastUpdatedTime,
		row.Identity,
		row.IdentityType,
		row.Comment,
	)
}

// SelectFromDomainAuditLogs returns audit log entries for a domain, operation type, and time range
func (pdb *db) SelectFromDomainAuditLogs(
	ctx context.Context,
	filter *sqlplugin.DomainAuditLogFilter,
) ([]*sqlplugin.DomainAuditLogRow, error) {
	args := []interface{}{
		filter.DomainID,
		filter.OperationType,
		*filter.MinCreatedTime,
		*filter.PageMaxCreatedTime,
		*filter.PageMinEventID,
	}

	var rows []*sqlplugin.DomainAuditLogRow
	if filter.PageSize > 0 {
		args = append(args, filter.PageSize)
		err := pdb.driver.SelectContext(ctx, sqlplugin.DbDefaultShard, &rows, _selectDomainAuditLogsQuery, args...)
		if err != nil {
			return nil, err
		}
	} else {
		err := pdb.driver.SelectContext(ctx, sqlplugin.DbDefaultShard, &rows, _selectAllDomainAuditLogsQuery, args...)
		if err != nil {
			return nil, err
		}
	}
	return rows, nil
}
