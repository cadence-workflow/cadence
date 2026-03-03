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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

func setUpMocksForDomainAuditStore(t *testing.T) (*sqlDomainAuditStore, *sqlplugin.MockDB) {
	ctrl := gomock.NewController(t)
	dbMock := sqlplugin.NewMockDB(ctrl)

	domainAuditStore := &sqlDomainAuditStore{
		sqlStore: sqlStore{db: dbMock},
	}

	return domainAuditStore, dbMock
}

func TestCreateDomainAuditLog(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1234567890, 0)

	stateBeforeBlob := &persistence.DataBlob{
		Encoding: constants.EncodingTypeThriftRWSnappy,
		Data:     []byte("state-before-data"),
	}

	stateAfterBlob := &persistence.DataBlob{
		Encoding: constants.EncodingTypeThriftRW,
		Data:     []byte("state-after-data"),
	}

	tests := map[string]struct {
		setupMock   func(*sqlplugin.MockDB)
		request     *persistence.InternalCreateDomainAuditLogRequest
		expectError bool
		expectedID  string
	}{
		"success with full data": {
			setupMock: func(dbMock *sqlplugin.MockDB) {
				expectedRow := &sqlplugin.DomainAuditLogRow{
					DomainID:            "d1111111-1111-1111-1111-111111111111",
					EventID:             "e1111111-1111-1111-1111-111111111111",
					StateBefore:         stateBeforeBlob.Data,
					StateBeforeEncoding: stateBeforeBlob.Encoding,
					StateAfter:          stateAfterBlob.Data,
					StateAfterEncoding:  stateAfterBlob.Encoding,
					OperationType:       persistence.DomainAuditOperationTypeUpdate,
					CreatedTime:         now,
					LastUpdatedTime:     now,
					Identity:            "test-user",
					IdentityType:        "user",
					Comment:             "test comment",
				}
				dbMock.EXPECT().InsertIntoDomainAuditLog(ctx, expectedRow).Return(&sqlResult{rowsAffected: 1}, nil).Times(1)
			},
			request: &persistence.InternalCreateDomainAuditLogRequest{
				DomainID:        "d1111111-1111-1111-1111-111111111111",
				EventID:         "e1111111-1111-1111-1111-111111111111",
				StateBefore:     stateBeforeBlob,
				StateAfter:      stateAfterBlob,
				OperationType:   persistence.DomainAuditOperationTypeUpdate,
				CreatedTime:     now,
				LastUpdatedTime: now,
				Identity:        "test-user",
				IdentityType:    "user",
				Comment:         "test comment",
			},
			expectError: false,
			expectedID:  "e1111111-1111-1111-1111-111111111111",
		},
		"success with nil state blobs": {
			setupMock: func(dbMock *sqlplugin.MockDB) {
				expectedRow := &sqlplugin.DomainAuditLogRow{
					DomainID:            "d1111111-1111-1111-1111-111111111111",
					EventID:             "e1111111-1111-1111-1111-111111111111",
					StateBefore:         []byte{},
					StateBeforeEncoding: constants.EncodingTypeEmpty,
					StateAfter:          []byte{},
					StateAfterEncoding:  constants.EncodingTypeEmpty,
					OperationType:       persistence.DomainAuditOperationTypeCreate,
					CreatedTime:         now,
					LastUpdatedTime:     now,
					Identity:            "system",
					IdentityType:        "system",
					Comment:             "",
				}
				dbMock.EXPECT().InsertIntoDomainAuditLog(ctx, expectedRow).Return(&sqlResult{rowsAffected: 1}, nil).Times(1)
			},
			request: &persistence.InternalCreateDomainAuditLogRequest{
				DomainID:        "d1111111-1111-1111-1111-111111111111",
				EventID:         "e1111111-1111-1111-1111-111111111111",
				StateBefore:     nil,
				StateAfter:      nil,
				OperationType:   persistence.DomainAuditOperationTypeCreate,
				CreatedTime:     now,
				LastUpdatedTime: now,
				Identity:        "system",
				IdentityType:    "system",
				Comment:         "",
			},
			expectError: false,
			expectedID:  "e1111111-1111-1111-1111-111111111111",
		},
		"database error": {
			setupMock: func(dbMock *sqlplugin.MockDB) {
				err := errors.New("database error")
				dbMock.EXPECT().InsertIntoDomainAuditLog(ctx, gomock.Any()).Return(nil, err).Times(1)
				dbMock.EXPECT().IsNotFoundError(err).Return(false).AnyTimes()
				dbMock.EXPECT().IsTimeoutError(err).Return(false).AnyTimes()
				dbMock.EXPECT().IsThrottlingError(err).Return(false).AnyTimes()
				dbMock.EXPECT().IsDupEntryError(err).Return(false).AnyTimes()
			},
			request: &persistence.InternalCreateDomainAuditLogRequest{
				DomainID:        "d1111111-1111-1111-1111-111111111111",
				EventID:         "e1111111-1111-1111-1111-111111111111",
				OperationType:   persistence.DomainAuditOperationTypeFailover,
				CreatedTime:     now,
				LastUpdatedTime: now,
				Identity:        "test-user",
				IdentityType:    "user",
			},
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForDomainAuditStore(t)
			tc.setupMock(dbMock)

			resp, err := store.CreateDomainAuditLog(ctx, tc.request)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tc.expectedID, resp.EventID)
			}
		})
	}
}

func TestGetDomainAuditLogs(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1234567890, 0)
	minTime := now.Add(-24 * time.Hour)
	maxTime := now
	maxUUID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	pageSize := 2
	pageSize2 := 3

	tests := map[string]struct {
		setupMock      func(*sqlplugin.MockDB)
		request        *persistence.GetDomainAuditLogsRequest
		expectError    bool
		expectedCount  int
		validateResult func(*testing.T, *persistence.InternalGetDomainAuditLogsResponse)
	}{
		"success - first page with results": {
			setupMock: func(dbMock *sqlplugin.MockDB) {
				rows := []*sqlplugin.DomainAuditLogRow{
					{
						EventID:             "e1111111-1111-1111-1111-111111111111",
						DomainID:            "d1111111-1111-1111-1111-111111111111",
						OperationType:       persistence.DomainAuditOperationTypeUpdate,
						CreatedTime:         now,
						LastUpdatedTime:     now,
						Identity:            "user-1",
						IdentityType:        "user",
						Comment:             "comment 1",
						StateBefore:         []byte("state-before-1"),
						StateBeforeEncoding: constants.EncodingTypeThriftRWSnappy,
						StateAfter:          []byte("state-after-1"),
						StateAfterEncoding:  constants.EncodingTypeThriftRW,
					},
					{
						EventID:             "e2222222-2222-2222-2222-222222222222",
						DomainID:            "d1111111-1111-1111-1111-111111111111",
						OperationType:       persistence.DomainAuditOperationTypeUpdate,
						CreatedTime:         now,
						LastUpdatedTime:     now,
						Identity:            "user-2",
						IdentityType:        "user",
						Comment:             "comment 2",
						StateBefore:         []byte("state-before-2"),
						StateBeforeEncoding: constants.EncodingTypeThriftRWSnappy,
						StateAfter:          []byte("state-after-2"),
						StateAfterEncoding:  constants.EncodingTypeThriftRWSnappy,
					},
				}

				expectedFilter := &sqlplugin.DomainAuditLogFilter{
					DomainID:           "d1111111-1111-1111-1111-111111111111",
					OperationType:      persistence.DomainAuditOperationTypeUpdate,
					MinCreatedTime:     &minTime,
					PageSize:           pageSize,
					PageMaxCreatedTime: &maxTime,
					PageMinEventID:     &maxUUID,
				}

				dbMock.EXPECT().SelectFromDomainAuditLogs(ctx, expectedFilter).Return(rows, nil).Times(1)
			},
			request: &persistence.GetDomainAuditLogsRequest{
				DomainID:       "d1111111-1111-1111-1111-111111111111",
				OperationType:  persistence.DomainAuditOperationTypeUpdate,
				MinCreatedTime: &minTime,
				MaxCreatedTime: &maxTime,
				PageSize:       2,
				NextPageToken:  nil,
			},
			expectError:   false,
			expectedCount: 2,
			validateResult: func(t *testing.T, resp *persistence.InternalGetDomainAuditLogsResponse) {
				require.Len(t, resp.AuditLogs, 2)
				assert.Equal(t, "e1111111-1111-1111-1111-111111111111", resp.AuditLogs[0].EventID)
				assert.Equal(t, "d1111111-1111-1111-1111-111111111111", resp.AuditLogs[0].DomainID)
				assert.Equal(t, persistence.DomainAuditOperationTypeUpdate, resp.AuditLogs[0].OperationType)
				assert.NotNil(t, resp.AuditLogs[0].StateBefore)
				assert.Equal(t, constants.EncodingTypeThriftRWSnappy, resp.AuditLogs[0].StateBefore.Encoding)
				assert.Equal(t, []byte("state-before-1"), resp.AuditLogs[0].StateBefore.Data)
				assert.NotNil(t, resp.AuditLogs[0].StateAfter)
				assert.Equal(t, constants.EncodingTypeThriftRW, resp.AuditLogs[0].StateAfter.Encoding)

				assert.Equal(t, "e2222222-2222-2222-2222-222222222222", resp.AuditLogs[1].EventID)
				assert.NotNil(t, resp.AuditLogs[1].StateBefore)
				assert.NotNil(t, resp.AuditLogs[1].StateAfter)

				assert.NotNil(t, resp.NextPageToken)
			},
		},
		"success - subsequent page with nextPageToken": {
			setupMock: func(dbMock *sqlplugin.MockDB) {
				expectedCreatedTime := now
				expectedEventID := "e1111111-1111-1111-1111-111111111111"

				rows := []*sqlplugin.DomainAuditLogRow{
					{
						EventID:             "e3333333-3333-3333-3333-333333333333",
						DomainID:            "d1111111-1111-1111-1111-111111111111",
						OperationType:       persistence.DomainAuditOperationTypeUpdate,
						CreatedTime:         now.Add(-2 * time.Hour),
						LastUpdatedTime:     now.Add(-2 * time.Hour),
						Identity:            "user-3",
						IdentityType:        "user",
						Comment:             "comment 3",
						StateBefore:         []byte("state-before-3"),
						StateBeforeEncoding: constants.EncodingTypeThriftRWSnappy,
						StateAfter:          []byte("state-after-3"),
						StateAfterEncoding:  constants.EncodingTypeThriftRW,
					},
					{
						EventID:             "e2222222-2222-2222-2222-222222222222",
						DomainID:            "d1111111-1111-1111-1111-111111111111",
						OperationType:       persistence.DomainAuditOperationTypeUpdate,
						CreatedTime:         expectedCreatedTime,
						LastUpdatedTime:     expectedCreatedTime,
						Identity:            "user-2",
						IdentityType:        "user",
						Comment:             "comment 2",
						StateBefore:         []byte("state-before-2"),
						StateBeforeEncoding: constants.EncodingTypeThriftRWSnappy,
						StateAfter:          []byte("state-after-2"),
						StateAfterEncoding:  constants.EncodingTypeThriftRW,
					},
				}

				expectedFilter := &sqlplugin.DomainAuditLogFilter{
					DomainID:           "d1111111-1111-1111-1111-111111111111",
					OperationType:      persistence.DomainAuditOperationTypeUpdate,
					MinCreatedTime:     &minTime,
					PageSize:           pageSize2,
					PageMaxCreatedTime: &expectedCreatedTime,
					PageMinEventID:     &expectedEventID,
				}

				dbMock.EXPECT().SelectFromDomainAuditLogs(ctx, expectedFilter).Return(rows, nil).Times(1)
			},
			request: func() *persistence.GetDomainAuditLogsRequest {
				pageToken := domainAuditLogPageToken{
					CreatedTime: now,
					EventID:     "e1111111-1111-1111-1111-111111111111",
				}
				encodedToken, _ := gobSerialize(pageToken)
				return &persistence.GetDomainAuditLogsRequest{
					DomainID:       "d1111111-1111-1111-1111-111111111111",
					OperationType:  persistence.DomainAuditOperationTypeUpdate,
					MinCreatedTime: &minTime,
					MaxCreatedTime: &maxTime,
					PageSize:       3,
					NextPageToken:  encodedToken,
				}
			}(),
			expectError:   false,
			expectedCount: 2,
			validateResult: func(t *testing.T, resp *persistence.InternalGetDomainAuditLogsResponse) {
				require.Len(t, resp.AuditLogs, 2)
				assert.Equal(t, "e3333333-3333-3333-3333-333333333333", resp.AuditLogs[0].EventID)
				assert.Equal(t, "e2222222-2222-2222-2222-222222222222", resp.AuditLogs[1].EventID)
				assert.Nil(t, resp.NextPageToken)
			},
		},
		"success with empty state blobs": {
			setupMock: func(dbMock *sqlplugin.MockDB) {
				rows := []*sqlplugin.DomainAuditLogRow{
					{
						EventID:             "e3333333-3333-3333-3333-333333333333",
						DomainID:            "d2222222-2222-2222-2222-222222222222",
						OperationType:       persistence.DomainAuditOperationTypeCreate,
						CreatedTime:         now,
						LastUpdatedTime:     now,
						Identity:            "system",
						IdentityType:        "system",
						Comment:             "created",
						StateBefore:         []byte{}, // Empty slice
						StateBeforeEncoding: constants.EncodingTypeEmpty,
						StateAfter:          nil, // Nil
						StateAfterEncoding:  constants.EncodingTypeEmpty,
					},
				}

				dbMock.EXPECT().SelectFromDomainAuditLogs(ctx, gomock.Any()).Return(rows, nil).Times(1)
			},
			request: &persistence.GetDomainAuditLogsRequest{
				DomainID: "d2222222-2222-2222-2222-222222222222",
				PageSize: 10,
			},
			expectError:   false,
			expectedCount: 1,
			validateResult: func(t *testing.T, resp *persistence.InternalGetDomainAuditLogsResponse) {
				require.Len(t, resp.AuditLogs, 1)
				assert.Equal(t, "e3333333-3333-3333-3333-333333333333", resp.AuditLogs[0].EventID)
				assert.Nil(t, resp.AuditLogs[0].StateBefore)
				assert.Nil(t, resp.AuditLogs[0].StateAfter)
			},
		},
		"success with no results": {
			setupMock: func(dbMock *sqlplugin.MockDB) {
				dbMock.EXPECT().SelectFromDomainAuditLogs(ctx, gomock.Any()).Return([]*sqlplugin.DomainAuditLogRow{}, nil).Times(1)
			},
			request: &persistence.GetDomainAuditLogsRequest{
				DomainID: "d3333333-3333-3333-3333-333333333333",
				PageSize: 10,
			},
			expectError:   false,
			expectedCount: 0,
			validateResult: func(t *testing.T, resp *persistence.InternalGetDomainAuditLogsResponse) {
				assert.Empty(t, resp.AuditLogs)
				assert.Nil(t, resp.NextPageToken)
			},
		},
		"invalid page token": {
			setupMock: func(dbMock *sqlplugin.MockDB) {
				// No DB call expected since deserialization should fail first
			},
			request: &persistence.GetDomainAuditLogsRequest{
				DomainID:      "d3333333-3333-3333-3333-333333333333",
				PageSize:      10,
				NextPageToken: []byte("invalid-token-data"),
			},
			expectError: true,
		},
		"database error": {
			setupMock: func(dbMock *sqlplugin.MockDB) {
				err := errors.New("database error")
				dbMock.EXPECT().SelectFromDomainAuditLogs(ctx, gomock.Any()).Return(nil, err).Times(1)
				dbMock.EXPECT().IsNotFoundError(err).Return(false).AnyTimes()
				dbMock.EXPECT().IsTimeoutError(err).Return(false).AnyTimes()
				dbMock.EXPECT().IsThrottlingError(err).Return(false).AnyTimes()
				dbMock.EXPECT().IsDupEntryError(err).Return(false).AnyTimes()
			},
			request: &persistence.GetDomainAuditLogsRequest{
				DomainID: "d3333333-3333-3333-3333-333333333333",
				PageSize: 10,
			},
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForDomainAuditStore(t)
			tc.setupMock(dbMock)

			resp, err := store.GetDomainAuditLogs(ctx, tc.request)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, resp)
				assert.Len(t, resp.AuditLogs, tc.expectedCount)
				if tc.validateResult != nil {
					tc.validateResult(t, resp)
				}
			}
		})
	}
}

func TestGetDataBlobBytes(t *testing.T) {
	tests := map[string]struct {
		input    *persistence.DataBlob
		expected []byte
	}{
		"nil blob": {
			input:    nil,
			expected: []byte{},
		},
		"blob with data": {
			input: &persistence.DataBlob{
				Encoding: constants.EncodingTypeThriftRWSnappy,
				Data:     []byte("test-data"),
			},
			expected: []byte("test-data"),
		},
		"blob with empty data": {
			input: &persistence.DataBlob{
				Encoding: constants.EncodingTypeThriftRW,
				Data:     []byte{},
			},
			expected: []byte{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := getDataBlobBytes(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetDataBlobEncoding(t *testing.T) {
	tests := map[string]struct {
		input    *persistence.DataBlob
		expected constants.EncodingType
	}{
		"nil blob": {
			input:    nil,
			expected: constants.EncodingTypeEmpty,
		},
		"blob with ThriftRWSnappy encoding": {
			input: &persistence.DataBlob{
				Encoding: constants.EncodingTypeThriftRWSnappy,
				Data:     []byte("test-data"),
			},
			expected: constants.EncodingTypeThriftRWSnappy,
		},
		"blob with ThriftRW encoding": {
			input: &persistence.DataBlob{
				Encoding: constants.EncodingTypeThriftRW,
				Data:     []byte("test-data"),
			},
			expected: constants.EncodingTypeThriftRW,
		},
		"blob with empty encoding": {
			input: &persistence.DataBlob{
				Encoding: "",
				Data:     []byte("test-data"),
			},
			expected: constants.EncodingTypeEmpty,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := getDataBlobEncoding(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
