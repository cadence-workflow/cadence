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
	"fmt"
	"time"

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
	WHERE domain_id = $1 AND operation_type = $2 AND created_time >= $3 AND created_time < $4`

	_selectDomainAuditLogsOrderByQuery = ` ORDER BY created_time DESC, event_id ASC`
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

// SelectFromDomainAuditLogs returns audit log entries for a domain, operation type, and time range (optional)
func (pdb *db) SelectFromDomainAuditLogs(
	ctx context.Context,
	filter *sqlplugin.DomainAuditLogFilter,
) ([]*sqlplugin.DomainAuditLogRow, error) {

	start := time.Unix(0, 0)
	end := time.Unix(0, time.Now().UnixNano())

	if filter.MinCreatedTime != nil {
		start = *filter.MinCreatedTime
	}
	if filter.MaxCreatedTime != nil {
		end = *filter.MaxCreatedTime
	}

	// Build base query with all required filters
	query := _selectDomainAuditLogsQuery
	args := []interface{}{filter.DomainID, filter.OperationType, start, end}
	argIndex := 5

	// Handle pagination token
	if filter.PageTokenMinCreatedTime != nil && filter.PageTokenMinEventID != nil {
		query += fmt.Sprintf(` AND (created_time < $%d OR (created_time = $%d AND event_id > $%d))`,
			argIndex, argIndex, argIndex+1)
		args = append(args, *filter.PageTokenMinCreatedTime, *filter.PageTokenMinEventID)
		argIndex += 2
	}

	query += _selectDomainAuditLogsOrderByQuery

	if filter.PageSize > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, argIndex)
		args = append(args, filter.PageSize)
	}

	var rows []*sqlplugin.DomainAuditLogRow
	err := pdb.driver.SelectContext(ctx, sqlplugin.DbDefaultShard, &rows, query, args...)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
