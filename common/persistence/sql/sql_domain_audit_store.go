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
	"fmt"
	"time"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

type sqlDomainAuditStore struct {
	sqlStore
}

// domainAuditLogPageToken is used for pagination
type domainAuditLogPageToken struct {
	CreatedTime time.Time          `json:"created_time"`
	EventID     serialization.UUID `json:"event_id"`
}

// newSQLDomainAuditStore creates an instance of sqlDomainAuditStore
func newSQLDomainAuditStore(
	db sqlplugin.DB,
	logger log.Logger,
	parser serialization.Parser,
) (persistence.DomainAuditStore, error) {
	return &sqlDomainAuditStore{
		sqlStore: sqlStore{
			db:     db,
			logger: logger,
			parser: parser,
		},
	}, nil
}

// CreateDomainAuditLog creates a new domain audit log entry
func (m *sqlDomainAuditStore) CreateDomainAuditLog(
	ctx context.Context,
	request *persistence.InternalCreateDomainAuditLogRequest,
) (*persistence.CreateDomainAuditLogResponse, error) {
	row := &sqlplugin.DomainAuditLogRow{
		DomainID:            serialization.MustParseUUID(request.DomainID),
		EventID:             serialization.MustParseUUID(request.EventID),
		StateBefore:         getDataBlobBytes(request.StateBefore),
		StateBeforeEncoding: getDataBlobEncoding(request.StateBefore),
		StateAfter:          getDataBlobBytes(request.StateAfter),
		StateAfterEncoding:  getDataBlobEncoding(request.StateAfter),
		OperationType:       request.OperationType,
		CreatedTime:         request.CreatedTime,
		LastUpdatedTime:     request.LastUpdatedTime,
		Identity:            request.Identity,
		IdentityType:        request.IdentityType,
		Comment:             request.Comment,
	}

	_, err := m.db.InsertIntoDomainAuditLog(ctx, row)
	if err != nil {
		return nil, convertCommonErrors(m.db, "CreateDomainAuditLog", "", err)
	}

	return &persistence.CreateDomainAuditLogResponse{
		EventID: request.EventID,
	}, nil
}

// GetDomainAuditLogs retrieves domain audit logs
func (m *sqlDomainAuditStore) GetDomainAuditLogs(
	ctx context.Context,
	request *persistence.GetDomainAuditLogsRequest,
) (*persistence.InternalGetDomainAuditLogsResponse, error) {
	minCreatedTime := time.Unix(0, 0)
	maxCreatedTime := time.Now().UTC()
	if request.MinCreatedTime != nil {
		minCreatedTime = *request.MinCreatedTime
	}
	if request.MaxCreatedTime != nil {
		maxCreatedTime = *request.MaxCreatedTime
	}

	pageMaxCreatedTime := maxCreatedTime
	// if next page token is not present, set pageMinEventID to largest possible uuid
	// to prevent the query from returning rows where created_time is equal to pageMaxCreatedTime
	pageMinEventID := serialization.UUID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	if request.NextPageToken != nil {
		page := domainAuditLogPageToken{}
		if err := gobDeserialize(request.NextPageToken, &page); err != nil {
			return nil, fmt.Errorf("unable to decode next page token")
		}
		pageMaxCreatedTime = page.CreatedTime
		pageMinEventID = page.EventID
	}

	filter := &sqlplugin.DomainAuditLogFilter{
		DomainID:           serialization.MustParseUUID(request.DomainID),
		OperationType:      request.OperationType,
		MinCreatedTime:     &minCreatedTime,
		MaxCreatedTime:     &maxCreatedTime,
		PageSize:           request.PageSize,
		PageMaxCreatedTime: &pageMaxCreatedTime,
		PageMinEventID:     &pageMinEventID,
	}

	rows, err := m.db.SelectFromDomainAuditLogs(ctx, filter)
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetDomainAuditLogs", "", err)
	}

	var nextPageToken []byte
	if request.PageSize > 0 && len(rows) >= request.PageSize {
		// there could be more results
		lastRow := rows[request.PageSize-1]
		token := domainAuditLogPageToken{
			CreatedTime: lastRow.CreatedTime,
			EventID:     lastRow.EventID,
		}
		nextPageToken, err = gobSerialize(token)
		if err != nil {
			return nil, &types.InternalServiceError{Message: fmt.Sprintf("error serializing nextPageToken:%v", err)}
		}
	}

	var auditLogs []*persistence.InternalDomainAuditLog
	for _, row := range rows {
		auditLog := &persistence.InternalDomainAuditLog{
			EventID:         row.EventID.String(),
			DomainID:        row.DomainID.String(),
			OperationType:   row.OperationType,
			CreatedTime:     row.CreatedTime,
			LastUpdatedTime: row.LastUpdatedTime,
			Identity:        row.Identity,
			IdentityType:    row.IdentityType,
			Comment:         row.Comment,
		}

		if len(row.StateBefore) > 0 {
			auditLog.StateBefore = &persistence.DataBlob{
				Encoding: constants.EncodingType(row.StateBeforeEncoding),
				Data:     row.StateBefore,
			}
		}

		if len(row.StateAfter) > 0 {
			auditLog.StateAfter = &persistence.DataBlob{
				Encoding: constants.EncodingType(row.StateAfterEncoding),
				Data:     row.StateAfter,
			}
		}

		auditLogs = append(auditLogs, auditLog)
	}

	return &persistence.InternalGetDomainAuditLogsResponse{
		AuditLogs:     auditLogs,
		NextPageToken: nextPageToken,
	}, nil
}

func getDataBlobBytes(blob *persistence.DataBlob) []byte {
	if blob == nil {
		return nil
	}
	return blob.Data
}

func getDataBlobEncoding(blob *persistence.DataBlob) string {
	if blob == nil {
		return ""
	}
	return string(blob.Encoding)
}
