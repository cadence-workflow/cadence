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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func setUpMocksForDomainAuditStore(t *testing.T) (*nosqlDomainAuditStore, *nosqlplugin.MockDB) {
	ctrl := gomock.NewController(t)
	dbMock := nosqlplugin.NewMockDB(ctrl)

	shardedStore := nosqlStore{
		db: dbMock,
	}

	domainAuditStore := &nosqlDomainAuditStore{
		nosqlStore: shardedStore,
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
		setupMock   func(*nosqlplugin.MockDB)
		request     *persistence.InternalCreateDomainAuditLogRequest
		expectError bool
		expectedID  string
	}{
		"success with full data": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				expectedRow := &nosqlplugin.DomainAuditLogRow{
					DomainID:            "domain-123",
					EventID:             "event-456",
					StateBefore:         stateBeforeBlob.Data,
					StateBeforeEncoding: string(stateBeforeBlob.Encoding),
					StateAfter:          stateAfterBlob.Data,
					StateAfterEncoding:  string(stateAfterBlob.Encoding),
					OperationType:       persistence.DomainAuditOperationTypeUpdate,
					CreatedTime:         now,
					LastUpdatedTime:     now,
					Identity:            "test-user",
					IdentityType:        "user",
					Comment:             "test comment",
				}
				dbMock.EXPECT().InsertDomainAuditLog(ctx, expectedRow).Return(nil).Times(1)
			},
			request: &persistence.InternalCreateDomainAuditLogRequest{
				DomainID:        "domain-123",
				EventID:         "event-456",
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
			expectedID:  "event-456",
		},
		"success with nil state blobs": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				expectedRow := &nosqlplugin.DomainAuditLogRow{
					DomainID:            "domain-123",
					EventID:             "event-789",
					StateBefore:         nil,
					StateBeforeEncoding: "",
					StateAfter:          nil,
					StateAfterEncoding:  "",
					OperationType:       persistence.DomainAuditOperationTypeCreate,
					CreatedTime:         now,
					LastUpdatedTime:     now,
					Identity:            "system",
					IdentityType:        "system",
					Comment:             "",
				}
				dbMock.EXPECT().InsertDomainAuditLog(ctx, expectedRow).Return(nil).Times(1)
			},
			request: &persistence.InternalCreateDomainAuditLogRequest{
				DomainID:        "domain-123",
				EventID:         "event-789",
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
			expectedID:  "event-789",
		},
		"database error": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertDomainAuditLog(ctx, gomock.Any()).Return(errors.New("database error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(false).AnyTimes()
				dbMock.EXPECT().IsTimeoutError(gomock.Any()).Return(false).AnyTimes()
				dbMock.EXPECT().IsThrottlingError(gomock.Any()).Return(false).AnyTimes()
				dbMock.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false).AnyTimes()
			},
			request: &persistence.InternalCreateDomainAuditLogRequest{
				DomainID:        "domain-123",
				EventID:         "event-999",
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

	tests := map[string]struct {
		setupMock      func(*nosqlplugin.MockDB)
		request        *persistence.GetDomainAuditLogsRequest
		expectError    bool
		expectedCount  int
		validateResult func(*testing.T, *persistence.InternalGetDomainAuditLogsResponse)
	}{
		"success with results": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				rows := []*nosqlplugin.DomainAuditLogRow{
					{
						EventID:             "event-1",
						DomainID:            "domain-123",
						OperationType:       persistence.DomainAuditOperationTypeUpdate,
						CreatedTime:         now,
						LastUpdatedTime:     now,
						Identity:            "user-1",
						IdentityType:        "user",
						Comment:             "comment 1",
						StateBefore:         []byte("state-before-1"),
						StateBeforeEncoding: string(constants.EncodingTypeThriftRWSnappy),
						StateAfter:          []byte("state-after-1"),
						StateAfterEncoding:  string(constants.EncodingTypeThriftRW),
					},
					{
						EventID:             "event-2",
						DomainID:            "domain-123",
						OperationType:       persistence.DomainAuditOperationTypeFailover,
						CreatedTime:         now.Add(-1 * time.Hour),
						LastUpdatedTime:     now.Add(-1 * time.Hour),
						Identity:            "user-2",
						IdentityType:        "user",
						Comment:             "comment 2",
						StateBefore:         []byte("state-before-2"),
						StateBeforeEncoding: string(constants.EncodingTypeThriftRWSnappy),
						StateAfter:          []byte("state-after-2"),
						StateAfterEncoding:  string(constants.EncodingTypeThriftRWSnappy),
					},
				}
				nextPageToken := []byte("next-token")

				expectedFilter := &nosqlplugin.DomainAuditLogFilter{
					DomainID:       "domain-123",
					OperationType:  persistence.DomainAuditOperationTypeUpdate,
					MinCreatedTime: &minTime,
					MaxCreatedTime: &maxTime,
					PageSize:       10,
					NextPageToken:  []byte("current-token"),
				}

				dbMock.EXPECT().SelectDomainAuditLogs(ctx, expectedFilter).Return(rows, nextPageToken, nil).Times(1)
			},
			request: &persistence.GetDomainAuditLogsRequest{
				DomainID:       "domain-123",
				OperationType:  persistence.DomainAuditOperationTypeUpdate,
				MinCreatedTime: &minTime,
				MaxCreatedTime: &maxTime,
				PageSize:       10,
				NextPageToken:  []byte("current-token"),
			},
			expectError:   false,
			expectedCount: 2,
			validateResult: func(t *testing.T, resp *persistence.InternalGetDomainAuditLogsResponse) {
				require.Len(t, resp.AuditLogs, 2)
				assert.Equal(t, "event-1", resp.AuditLogs[0].EventID)
				assert.Equal(t, "domain-123", resp.AuditLogs[0].DomainID)
				assert.Equal(t, persistence.DomainAuditOperationTypeUpdate, resp.AuditLogs[0].OperationType)
				assert.NotNil(t, resp.AuditLogs[0].StateBefore)
				assert.Equal(t, constants.EncodingTypeThriftRWSnappy, resp.AuditLogs[0].StateBefore.Encoding)
				assert.Equal(t, []byte("state-before-1"), resp.AuditLogs[0].StateBefore.Data)
				assert.NotNil(t, resp.AuditLogs[0].StateAfter)
				assert.Equal(t, constants.EncodingTypeThriftRW, resp.AuditLogs[0].StateAfter.Encoding)

				assert.Equal(t, "event-2", resp.AuditLogs[1].EventID)
				assert.NotNil(t, resp.AuditLogs[1].StateBefore)
				assert.NotNil(t, resp.AuditLogs[1].StateAfter)

				assert.Equal(t, []byte("next-token"), resp.NextPageToken)
			},
		},
		"success with empty state blobs": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				rows := []*nosqlplugin.DomainAuditLogRow{
					{
						EventID:             "event-3",
						DomainID:            "domain-456",
						OperationType:       persistence.DomainAuditOperationTypeCreate,
						CreatedTime:         now,
						LastUpdatedTime:     now,
						Identity:            "system",
						IdentityType:        "system",
						Comment:             "created",
						StateBefore:         []byte{}, // Empty slice
						StateBeforeEncoding: "",
						StateAfter:          nil, // Nil
						StateAfterEncoding:  "",
					},
				}

				dbMock.EXPECT().SelectDomainAuditLogs(ctx, gomock.Any()).Return(rows, nil, nil).Times(1)
			},
			request: &persistence.GetDomainAuditLogsRequest{
				DomainID: "domain-456",
				PageSize: 10,
			},
			expectError:   false,
			expectedCount: 1,
			validateResult: func(t *testing.T, resp *persistence.InternalGetDomainAuditLogsResponse) {
				require.Len(t, resp.AuditLogs, 1)
				assert.Equal(t, "event-3", resp.AuditLogs[0].EventID)
				// Empty/nil state blobs should not be converted
				assert.Nil(t, resp.AuditLogs[0].StateBefore)
				assert.Nil(t, resp.AuditLogs[0].StateAfter)
			},
		},
		"success with no results": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectDomainAuditLogs(ctx, gomock.Any()).Return([]*nosqlplugin.DomainAuditLogRow{}, nil, nil).Times(1)
			},
			request: &persistence.GetDomainAuditLogsRequest{
				DomainID: "domain-789",
				PageSize: 10,
			},
			expectError:   false,
			expectedCount: 0,
			validateResult: func(t *testing.T, resp *persistence.InternalGetDomainAuditLogsResponse) {
				assert.Empty(t, resp.AuditLogs)
				assert.Nil(t, resp.NextPageToken)
			},
		},
		"database error": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectDomainAuditLogs(ctx, gomock.Any()).Return(nil, nil, errors.New("database error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(false).AnyTimes()
				dbMock.EXPECT().IsTimeoutError(gomock.Any()).Return(false).AnyTimes()
				dbMock.EXPECT().IsThrottlingError(gomock.Any()).Return(false).AnyTimes()
				dbMock.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false).AnyTimes()
			},
			request: &persistence.GetDomainAuditLogsRequest{
				DomainID: "domain-999",
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
			expected: nil,
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
		expected string
	}{
		"nil blob": {
			input:    nil,
			expected: "",
		},
		"blob with ThriftRWSnappy encoding": {
			input: &persistence.DataBlob{
				Encoding: constants.EncodingTypeThriftRWSnappy,
				Data:     []byte("test-data"),
			},
			expected: string(constants.EncodingTypeThriftRWSnappy),
		},
		"blob with ThriftRW encoding": {
			input: &persistence.DataBlob{
				Encoding: constants.EncodingTypeThriftRW,
				Data:     []byte("test-data"),
			},
			expected: string(constants.EncodingTypeThriftRW),
		},
		"blob with empty encoding": {
			input: &persistence.DataBlob{
				Encoding: "",
				Data:     []byte("test-data"),
			},
			expected: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := getDataBlobEncoding(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
