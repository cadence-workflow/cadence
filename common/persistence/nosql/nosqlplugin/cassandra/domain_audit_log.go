// Copyright (c) 2020 Uber Technologies, Inc.
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
	"time"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

const (
	templateInsertDomainAuditLogQuery = `INSERT INTO domain_audit_log (` +
		`domain_id, event_id, state_before, state_before_encoding, state_after, state_after_encoding, ` +
		`operation_type, created_time, last_updated_time, identity, identity_type, comment) ` +
		`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) USING TTL ?`

	templateSelectDomainAuditLogsQuery = `SELECT ` +
		`event_id, domain_id, state_before, state_before_encoding, state_after, state_after_encoding, ` +
		`operation_type, created_time, last_updated_time, identity, identity_type, comment ` +
		`FROM domain_audit_log ` +
		`WHERE domain_id = ? AND operation_type = ? ` +
		`AND created_time >= ? AND created_time < ?`
)

// InsertDomainAuditLog inserts a new audit log entry for a domain operation
func (db *CDB) InsertDomainAuditLog(ctx context.Context, row *nosqlplugin.DomainAuditLogRow) error {
	query := db.session.Query(templateInsertDomainAuditLogQuery,
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
		row.TTLSeconds,
	).WithContext(ctx)

	err := query.Exec()
	return err
}

// SelectDomainAuditLogs returns audit log entries for a domain and operation type
func (db *CDB) SelectDomainAuditLogs(ctx context.Context, filter *nosqlplugin.DomainAuditLogFilter) ([]*nosqlplugin.DomainAuditLogRow, []byte, error) {

	start := time.Unix(0, 0)
	end := time.Unix(0, time.Now().UnixNano())

	if filter.MinCreatedTime != nil {
		start = *filter.MinCreatedTime
	}
	if filter.MaxCreatedTime != nil {
		end = *filter.MaxCreatedTime
	}

	query := db.session.Query(templateSelectDomainAuditLogsQuery,
		filter.DomainID,
		filter.OperationType,
		start,
		end,
	).WithContext(ctx)

	// Set page size
	if filter.PageSize > 0 {
		query = query.PageSize(filter.PageSize)
	}

	// Set page state for pagination
	if len(filter.NextPageToken) > 0 {
		query = query.PageState(filter.NextPageToken)
	}

	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectDomainAuditLogs operation failed. Not able to create query iterator.",
		}
	}

	var rows []*nosqlplugin.DomainAuditLogRow
	row := &nosqlplugin.DomainAuditLogRow{}

	for iter.Scan(
		&row.EventID,
		&row.DomainID,
		&row.StateBefore,
		&row.StateBeforeEncoding,
		&row.StateAfter,
		&row.StateAfterEncoding,
		&row.OperationType,
		&row.CreatedTime,
		&row.LastUpdatedTime,
		&row.Identity,
		&row.IdentityType,
		&row.Comment,
	) {
		rows = append(rows, row)
		row = &nosqlplugin.DomainAuditLogRow{}

		// Break after collecting PageSize number of rows
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
