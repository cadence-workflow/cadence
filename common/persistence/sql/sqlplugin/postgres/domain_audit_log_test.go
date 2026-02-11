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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqldriver"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

func TestInsertIntoDomainAuditLog(t *testing.T) {
	now := time.Now().UTC()
	domainID := serialization.MustParseUUID("d1111111-1111-1111-1111-111111111111")
	eventID := serialization.MustParseUUID("e1111111-1111-1111-1111-111111111111")

	tests := []struct {
		name      string
		row       *sqlplugin.DomainAuditLogRow
		mockSetup func(*sqldriver.MockDriver)
		wantErr   bool
	}{
		{
			name: "successfully inserted",
			row: &sqlplugin.DomainAuditLogRow{
				DomainID:            domainID,
				EventID:             eventID,
				StateBefore:         []byte("state-before"),
				StateBeforeEncoding: "json",
				StateAfter:          []byte("state-after"),
				StateAfterEncoding:  "json",
				OperationType:       persistence.DomainAuditOperationTypeFailover,
				CreatedTime:         now,
				LastUpdatedTime:     now,
				Identity:            "test-identity",
				IdentityType:        "user",
				Comment:             "test comment",
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().ExecContext(
					gomock.Any(),
					sqlplugin.DbDefaultShard,
					_insertDomainAuditLogQuery,
					domainID,
					eventID,
					[]byte("state-before"),
					"json",
					[]byte("state-after"),
					"json",
					persistence.DomainAuditOperationTypeFailover,
					now,
					now,
					"test-identity",
					"user",
					"test comment",
				).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name: "exec failed",
			row: &sqlplugin.DomainAuditLogRow{
				DomainID:      domainID,
				EventID:       eventID,
				OperationType: persistence.DomainAuditOperationTypeFailover,
				CreatedTime:   now,
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().ExecContext(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil, errors.New("exec failed"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDriver := sqldriver.NewMockDriver(ctrl)
			tc.mockSetup(mockDriver)

			pdb := &db{
				driver:    mockDriver,
				converter: &converter{},
			}

			_, err := pdb.InsertIntoDomainAuditLog(context.Background(), tc.row)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSelectFromDomainAuditLogs(t *testing.T) {
	domainID := serialization.MustParseUUID("d1111111-1111-1111-1111-111111111111")
	operationType := persistence.DomainAuditOperationTypeFailover
	minTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	maxTime := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)

	// Create times in descending order
	createdTime1 := time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)
	createdTime2 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	createdTime3 := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	createdTime4 := time.Date(2024, 4, 1, 12, 0, 0, 0, time.UTC)

	eventID1 := serialization.MustParseUUID("e1111111-1111-1111-1111-111111111111")
	eventID2 := serialization.MustParseUUID("e2222222-2222-2222-2222-222222222222")
	eventID3 := serialization.MustParseUUID("e3333333-3333-3333-3333-333333333333")
	eventID4 := serialization.MustParseUUID("e4444444-4444-4444-4444-444444444444")

	// Default page cursor: maxCreatedTime with max UUID (no page token)
	defaultPageMaxCreatedTime := maxTime
	defaultPageMinEventID := serialization.UUID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	tests := []struct {
		name      string
		filter    *sqlplugin.DomainAuditLogFilter
		mockSetup func(*sqldriver.MockDriver)
		wantRows  []*sqlplugin.DomainAuditLogRow
		wantErr   bool
	}{
		{
			name: "pageSize limits number of results; no pageToken",
			filter: &sqlplugin.DomainAuditLogFilter{
				DomainID:           domainID,
				OperationType:      operationType,
				MinCreatedTime:     &minTime,
				MaxCreatedTime:     &maxTime,
				PageSize:           2,
				PageMaxCreatedTime: &defaultPageMaxCreatedTime,
				PageMinEventID:     &defaultPageMinEventID,
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().SelectContext(
					gomock.Any(),
					sqlplugin.DbDefaultShard,
					gomock.Any(),
					_selectDomainAuditLogsQuery,
					domainID, operationType, minTime, maxTime, defaultPageMaxCreatedTime, defaultPageMinEventID, 2,
				).DoAndReturn(func(ctx context.Context, shardID int, dest interface{}, query string, args ...interface{}) error {
					rows := dest.(*[]*sqlplugin.DomainAuditLogRow)
					*rows = []*sqlplugin.DomainAuditLogRow{
						{
							EventID:       eventID1,
							DomainID:      domainID,
							CreatedTime:   createdTime1,
							OperationType: operationType,
						},
						{
							EventID:       eventID2,
							DomainID:      domainID,
							CreatedTime:   createdTime2,
							OperationType: operationType,
						},
					}
					return nil
				})
			},
			wantRows: []*sqlplugin.DomainAuditLogRow{
				{
					EventID:       eventID1,
					DomainID:      domainID,
					CreatedTime:   createdTime1,
					OperationType: operationType,
				},
				{
					EventID:       eventID2,
					DomainID:      domainID,
					CreatedTime:   createdTime2,
					OperationType: operationType,
				},
			},
			wantErr: false,
		},
		{
			name: "pageToken filters results by createdTime and eventID; pageSize is 0",
			filter: &sqlplugin.DomainAuditLogFilter{
				DomainID:           domainID,
				OperationType:      operationType,
				MinCreatedTime:     &minTime,
				MaxCreatedTime:     &maxTime,
				PageSize:           0,
				PageMaxCreatedTime: &createdTime2,
				PageMinEventID:     &eventID2,
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().SelectContext(
					gomock.Any(),
					sqlplugin.DbDefaultShard,
					gomock.Any(),
					_selectAllDomainAuditLogsQuery,
					domainID, operationType, minTime, maxTime, createdTime2, eventID2,
				).DoAndReturn(func(ctx context.Context, shardID int, dest interface{}, query string, args ...interface{}) error {
					rows := dest.(*[]*sqlplugin.DomainAuditLogRow)
					*rows = []*sqlplugin.DomainAuditLogRow{
						{
							EventID:       eventID3,
							DomainID:      domainID,
							CreatedTime:   createdTime3,
							OperationType: operationType,
						},
						{
							EventID:       eventID4,
							DomainID:      domainID,
							CreatedTime:   createdTime4,
							OperationType: operationType,
						},
					}
					return nil
				})
			},
			wantRows: []*sqlplugin.DomainAuditLogRow{
				{
					EventID:       eventID3,
					DomainID:      domainID,
					CreatedTime:   createdTime3,
					OperationType: operationType,
				},
				{
					EventID:       eventID4,
					DomainID:      domainID,
					CreatedTime:   createdTime4,
					OperationType: operationType,
				},
			},
			wantErr: false,
		},
		{
			name: "success with no results",
			filter: &sqlplugin.DomainAuditLogFilter{
				DomainID:           domainID,
				OperationType:      operationType,
				MinCreatedTime:     &minTime,
				MaxCreatedTime:     &maxTime,
				PageMaxCreatedTime: &defaultPageMaxCreatedTime,
				PageMinEventID:     &defaultPageMinEventID,
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().SelectContext(
					gomock.Any(),
					sqlplugin.DbDefaultShard,
					gomock.Any(),
					_selectAllDomainAuditLogsQuery,
					domainID, operationType, minTime, maxTime, defaultPageMaxCreatedTime, defaultPageMinEventID,
				).DoAndReturn(func(ctx context.Context, shardID int, dest interface{}, query string, args ...interface{}) error {
					return nil
				})
			},
			wantRows: []*sqlplugin.DomainAuditLogRow{},
			wantErr:  false,
		},
		{
			name: "error when select fails",
			filter: &sqlplugin.DomainAuditLogFilter{
				DomainID:           domainID,
				OperationType:      operationType,
				MinCreatedTime:     &minTime,
				MaxCreatedTime:     &maxTime,
				PageMaxCreatedTime: &defaultPageMaxCreatedTime,
				PageMinEventID:     &defaultPageMinEventID,
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().SelectContext(
					gomock.Any(),
					sqlplugin.DbDefaultShard,
					gomock.Any(),
					_selectAllDomainAuditLogsQuery,
					domainID, operationType, minTime, maxTime, defaultPageMaxCreatedTime, defaultPageMinEventID,
				).Return(errors.New("select failed"))
			},
			wantRows: nil,
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDriver := sqldriver.NewMockDriver(ctrl)
			tc.mockSetup(mockDriver)

			pdb := &db{
				driver:    mockDriver,
				converter: &converter{},
			}

			rows, err := pdb.SelectFromDomainAuditLogs(context.Background(), tc.filter)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, len(tc.wantRows), len(rows), "number of rows should match")

			for i, wantRow := range tc.wantRows {
				if i < len(rows) {
					assert.Equal(t, wantRow.EventID, rows[i].EventID, "row %d eventID", i)
					assert.Equal(t, wantRow.DomainID, rows[i].DomainID, "row %d domainID", i)
					assert.Equal(t, wantRow.OperationType, rows[i].OperationType, "row %d operationType", i)
					assert.Equal(t, wantRow.CreatedTime.Unix(), rows[i].CreatedTime.Unix(), "row %d createdTime", i)
				}
			}
		})
	}
}
