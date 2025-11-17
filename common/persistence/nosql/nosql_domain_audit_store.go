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

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

type nosqlDomainAuditStore struct {
	nosqlStore
}

// newNoSQLDomainAuditStore is used to create an instance of DomainAuditStore implementation
func newNoSQLDomainAuditStore(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	dc *persistence.DynamicConfiguration,
) (persistence.DomainAuditStore, error) {
	shardedStore, err := newShardedNosqlStore(cfg, logger, metricsClient, dc)
	if err != nil {
		return nil, err
	}
	return &nosqlDomainAuditStore{
		nosqlStore: shardedStore.GetDefaultShard(),
	}, nil
}

// CreateDomainAuditLog creates a new domain audit log entry
func (m *nosqlDomainAuditStore) CreateDomainAuditLog(
	ctx context.Context,
	request *persistence.InternalCreateDomainAuditLogRequest,
) (*persistence.CreateDomainAuditLogResponse, error) {
	row := &nosqlplugin.DomainAuditLogRow{
		DomainID:            request.DomainID,
		EventID:             request.EventID,
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
		TTLSeconds:          request.TTLSeconds,
	}

	err := m.db.InsertDomainAuditLog(ctx, row)
	if err != nil {
		return nil, convertCommonErrors(m.db, "CreateDomainAuditLog", err)
	}

	return &persistence.CreateDomainAuditLogResponse{
		EventID: request.EventID,
	}, nil
}

// GetDomainAuditLogs retrieves domain audit logs
func (m *nosqlDomainAuditStore) GetDomainAuditLogs(
	ctx context.Context,
	request *persistence.GetDomainAuditLogsRequest,
) (*persistence.InternalGetDomainAuditLogsResponse, error) {
	filter := &nosqlplugin.DomainAuditLogFilter{
		DomainID:       request.DomainID,
		OperationType:  request.OperationType,
		MinCreatedTime: request.MinCreatedTime,
		MaxCreatedTime: request.MaxCreatedTime,
		PageSize:       request.PageSize,
		NextPageToken:  request.NextPageToken,
	}

	rows, nextPageToken, err := m.db.SelectDomainAuditLogs(ctx, filter)
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetDomainAuditLogs", err)
	}

	var auditLogs []*persistence.InternalDomainAuditLog
	for _, row := range rows {
		auditLog := &persistence.InternalDomainAuditLog{
			EventID:         row.EventID,
			DomainID:        row.DomainID,
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
