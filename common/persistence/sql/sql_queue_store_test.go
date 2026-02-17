// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package sql

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

var fixedTime = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

func TestGetNextID(t *testing.T) {
	tests := map[string]struct {
		acks     map[string]int64
		lastID   int64
		expected int64
	}{
		"expected case - last ID is equal to ack-levels": {
			acks:     map[string]int64{"a": 3},
			lastID:   3,
			expected: 4,
		},
		"expected case - last ID is equal to ack-levels haven't caught up": {
			acks:     map[string]int64{"a": 2},
			lastID:   3,
			expected: 4,
		},
		"error case - ack-levels are ahead for some reason": {
			acks:     map[string]int64{"a": 3},
			lastID:   2,
			expected: 4,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, getNextID(td.acks, td.lastID))
		})
	}
}

func TestEnqueueMessage(t *testing.T) {
	testCases := []struct {
		name      string
		queueType persistence.QueueType
		mockSetup func(*sqlplugin.MockDB, *sqlplugin.MockTx)
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetLastEnqueuedMessageIDForUpdate(gomock.Any(), persistence.DomainReplicationQueueType).Return(int64(0), sql.ErrNoRows)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, true).Return(nil, nil)
				mockTx.EXPECT().InsertIntoQueue(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name:      "Error case - failed to get last enqueued message ID for update",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().GetLastEnqueuedMessageIDForUpdate(gomock.Any(), persistence.DomainReplicationQueueType).Return(int64(0), err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:      "Error case - failed to get ack levels",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetLastEnqueuedMessageIDForUpdate(gomock.Any(), persistence.DomainReplicationQueueType).Return(int64(0), sql.ErrNoRows)
				err := errors.New("some error")
				mockTx.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, true).Return(nil, err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:      "Error case - failed to insert into queue",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetLastEnqueuedMessageIDForUpdate(gomock.Any(), persistence.DomainReplicationQueueType).Return(int64(0), sql.ErrNoRows)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, true).Return(nil, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoQueue(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockTx := sqlplugin.NewMockTx(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB, mockTx)
			err = store.EnqueueMessage(context.Background(), &persistence.InternalEnqueueMessageRequest{
				MessagePayload:   nil,
				CurrentTimeStamp: fixedTime,
			})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestReadMessages(t *testing.T) {
	testCases := []struct {
		name      string
		queueType persistence.QueueType
		mockSetup func(*sqlplugin.MockDB)
		want      []*persistence.InternalQueueMessage
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetMessagesFromQueue(gomock.Any(), persistence.DomainReplicationQueueType, gomock.Any(), gomock.Any()).Return([]sqlplugin.QueueRow{
					{
						QueueType:      persistence.DomainReplicationQueueType,
						MessageID:      123,
						MessagePayload: []byte(`aaaa`),
					},
				}, nil)
			},
			want: []*persistence.InternalQueueMessage{
				{
					ID:      123,
					Payload: []byte(`aaaa`),
				},
			},
			wantErr: false,
		},
		{
			name:      "Error case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().GetMessagesFromQueue(gomock.Any(), persistence.DomainReplicationQueueType, gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB)
			resp, err := store.ReadMessages(context.Background(), &persistence.InternalReadMessagesRequest{
				LastMessageID: 0,
				MaxCount:      10,
			})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, resp.Messages, "Unexpected result for test case")
			}
		})
	}
}

func TestDeleteMessagesBefore(t *testing.T) {
	testCases := []struct {
		name      string
		queueType persistence.QueueType
		mockSetup func(*sqlplugin.MockDB)
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().DeleteMessagesBefore(gomock.Any(), persistence.DomainReplicationQueueType, gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name:      "Error case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().DeleteMessagesBefore(gomock.Any(), persistence.DomainReplicationQueueType, gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB)
			err = store.DeleteMessagesBefore(context.Background(), &persistence.InternalDeleteMessagesBeforeRequest{
				MessageID: 10,
			})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestUpdateAckLevel(t *testing.T) {
	testCases := []struct {
		name        string
		queueType   persistence.QueueType
		clusterName string
		mockSetup   func(*sqlplugin.MockDB, *sqlplugin.MockTx)
		wantErr     bool
	}{
		{
			name:        "Success case - insert",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, true).Return(nil, nil)
				mockTx.EXPECT().InsertAckLevel(gomock.Any(), persistence.DomainReplicationQueueType, gomock.Any(), gomock.Any()).Return(nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "Success case - update",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, true).Return(map[string]int64{}, nil)
				mockTx.EXPECT().UpdateAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, gomock.Any()).Return(nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "Success case - no op",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, true).Return(map[string]int64{"abc": 100}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "Error case - failed to get",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, true).Return(nil, err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:        "Error case - failed to insert",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, true).Return(nil, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertAckLevel(gomock.Any(), persistence.DomainReplicationQueueType, gomock.Any(), gomock.Any()).Return(err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:        "Error case - failed to update",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, true).Return(map[string]int64{}, nil)
				err := errors.New("some error")
				mockTx.EXPECT().UpdateAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, gomock.Any()).Return(err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockTx := sqlplugin.NewMockTx(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB, mockTx)
			err = store.UpdateAckLevel(context.Background(), &persistence.InternalUpdateAckLevelRequest{
				MessageID:        0,
				ClusterName:      tc.clusterName,
				CurrentTimeStamp: fixedTime,
			})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestGetAckLevels(t *testing.T) {
	testCases := []struct {
		name      string
		queueType persistence.QueueType
		mockSetup func(*sqlplugin.MockDB)
		want      map[string]int64
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, false).Return(map[string]int64{"x": 9}, nil)
			},
			want:    map[string]int64{"x": 9},
			wantErr: false,
		},
		{
			name:      "Error case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().GetAckLevels(gomock.Any(), persistence.DomainReplicationQueueType, false).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB)
			resp, err := store.GetAckLevels(context.Background(), &persistence.InternalGetAckLevelsRequest{})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, resp.AckLevels, "Unexpected result for test case")
			}
		})
	}
}

func TestEnqueueMessageToDLQ(t *testing.T) {
	testCases := []struct {
		name      string
		queueType persistence.QueueType
		mockSetup func(*sqlplugin.MockDB, *sqlplugin.MockTx)
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetLastEnqueuedMessageIDForUpdate(gomock.Any(), -1*persistence.DomainReplicationQueueType).Return(int64(0), sql.ErrNoRows)
				mockTx.EXPECT().InsertIntoQueue(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name:      "Error case - failed to get last enqueued message ID for update",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().GetLastEnqueuedMessageIDForUpdate(gomock.Any(), -1*persistence.DomainReplicationQueueType).Return(int64(0), err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:      "Error case - failed to insert into queue",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetLastEnqueuedMessageIDForUpdate(gomock.Any(), -1*persistence.DomainReplicationQueueType).Return(int64(0), sql.ErrNoRows)
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoQueue(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockTx := sqlplugin.NewMockTx(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB, mockTx)
			err = store.EnqueueMessageToDLQ(context.Background(), &persistence.InternalEnqueueMessageToDLQRequest{
				MessagePayload:   nil,
				CurrentTimeStamp: fixedTime,
			})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestDeleteMessageFromDLQ(t *testing.T) {
	testCases := []struct {
		name      string
		queueType persistence.QueueType
		mockSetup func(*sqlplugin.MockDB)
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().DeleteMessage(gomock.Any(), -1*persistence.DomainReplicationQueueType, gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name:      "Error case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().DeleteMessage(gomock.Any(), -1*persistence.DomainReplicationQueueType, gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB)
			err = store.DeleteMessageFromDLQ(context.Background(), &persistence.InternalDeleteMessageFromDLQRequest{
				MessageID: 10,
			})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestRangeDeleteMessagesFromDLQ(t *testing.T) {
	testCases := []struct {
		name      string
		queueType persistence.QueueType
		mockSetup func(*sqlplugin.MockDB)
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteMessages(gomock.Any(), -1*persistence.DomainReplicationQueueType, gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name:      "Error case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().RangeDeleteMessages(gomock.Any(), -1*persistence.DomainReplicationQueueType, gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB)
			err = store.RangeDeleteMessagesFromDLQ(context.Background(), &persistence.InternalRangeDeleteMessagesFromDLQRequest{
				FirstMessageID: 10,
				LastMessageID:  100,
			})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestGetDLQAckLevels(t *testing.T) {
	testCases := []struct {
		name      string
		queueType persistence.QueueType
		mockSetup func(*sqlplugin.MockDB)
		want      map[string]int64
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetAckLevels(gomock.Any(), -1*persistence.DomainReplicationQueueType, false).Return(map[string]int64{"x": 9}, nil)
			},
			want:    map[string]int64{"x": 9},
			wantErr: false,
		},
		{
			name:      "Error case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().GetAckLevels(gomock.Any(), -1*persistence.DomainReplicationQueueType, false).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB)
			resp, err := store.GetDLQAckLevels(context.Background(), &persistence.InternalGetDLQAckLevelsRequest{})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, resp.AckLevels, "Unexpected result for test case")
			}
		})
	}
}

func TestUpdateDLQAckLevel(t *testing.T) {
	testCases := []struct {
		name        string
		queueType   persistence.QueueType
		clusterName string
		mockSetup   func(*sqlplugin.MockDB, *sqlplugin.MockTx)
		wantErr     bool
	}{
		{
			name:        "Success case - insert",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), -1*persistence.DomainReplicationQueueType, true).Return(nil, nil)
				mockTx.EXPECT().InsertAckLevel(gomock.Any(), -1*persistence.DomainReplicationQueueType, gomock.Any(), gomock.Any()).Return(nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "Success case - update",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), -1*persistence.DomainReplicationQueueType, true).Return(map[string]int64{}, nil)
				mockTx.EXPECT().UpdateAckLevels(gomock.Any(), -1*persistence.DomainReplicationQueueType, gomock.Any()).Return(nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "Success case - no op",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), -1*persistence.DomainReplicationQueueType, true).Return(map[string]int64{"abc": 100}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "Error case - failed to get",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().GetAckLevels(gomock.Any(), -1*persistence.DomainReplicationQueueType, true).Return(nil, err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:        "Error case - failed to insert",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), -1*persistence.DomainReplicationQueueType, true).Return(nil, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertAckLevel(gomock.Any(), -1*persistence.DomainReplicationQueueType, gomock.Any(), gomock.Any()).Return(err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:        "Error case - failed to update",
			queueType:   persistence.DomainReplicationQueueType,
			clusterName: "abc",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), sqlplugin.DbDefaultShard).Return(mockTx, nil)
				mockTx.EXPECT().GetAckLevels(gomock.Any(), -1*persistence.DomainReplicationQueueType, true).Return(map[string]int64{}, nil)
				err := errors.New("some error")
				mockTx.EXPECT().UpdateAckLevels(gomock.Any(), -1*persistence.DomainReplicationQueueType, gomock.Any()).Return(err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockTx := sqlplugin.NewMockTx(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB, mockTx)
			err = store.UpdateDLQAckLevel(context.Background(), &persistence.InternalUpdateDLQAckLevelRequest{
				MessageID:        0,
				ClusterName:      tc.clusterName,
				CurrentTimeStamp: fixedTime,
			})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestGetDLQSize(t *testing.T) {
	testCases := []struct {
		name      string
		queueType persistence.QueueType
		mockSetup func(*sqlplugin.MockDB)
		want      int64
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetQueueSize(gomock.Any(), -1*persistence.DomainReplicationQueueType).Return(int64(1000), nil)
			},
			want:    1000,
			wantErr: false,
		},
		{
			name:      "Error case",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().GetQueueSize(gomock.Any(), -1*persistence.DomainReplicationQueueType).Return(int64(0), err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB)
			resp, err := store.GetDLQSize(context.Background(), &persistence.InternalGetDLQSizeRequest{})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, resp.Size, "Unexpected result for test case")
			}
		})
	}
}

func TestReadMessagesFromDLQ(t *testing.T) {
	firstMessageID := int64(0)
	lastMessageID := int64(9999)
	pageSize := 1
	testCases := []struct {
		name      string
		queueType persistence.QueueType
		pageToken []byte
		mockSetup func(*sqlplugin.MockDB)
		want      []*persistence.InternalQueueMessage
		wantToken []byte
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: persistence.DomainReplicationQueueType,
			pageToken: serializePageToken(int64(100)),
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetMessagesBetween(gomock.Any(), -1*persistence.DomainReplicationQueueType, gomock.Any(), gomock.Any(), gomock.Any()).Return([]sqlplugin.QueueRow{
					{
						QueueType:      -1 * persistence.DomainReplicationQueueType,
						MessageID:      123,
						MessagePayload: []byte(`aaaa`),
					},
				}, nil)
			},
			want: []*persistence.InternalQueueMessage{
				{
					ID:      123,
					Payload: []byte(`aaaa`),
				},
			},
			wantToken: serializePageToken(int64(123)),
			wantErr:   false,
		},
		{
			name:      "Error case - failed to deserialize token",
			queueType: persistence.DomainReplicationQueueType,
			pageToken: []byte(`aaa`),
			mockSetup: func(mockDB *sqlplugin.MockDB) {},
			wantErr:   true,
		},
		{
			name:      "Error case - failed to get messages",
			queueType: persistence.DomainReplicationQueueType,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().GetMessagesBetween(gomock.Any(), -1*persistence.DomainReplicationQueueType, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := newQueueStore(mockDB, nil, tc.queueType)
			require.NoError(t, err, "Failed to create sql queue store")

			tc.mockSetup(mockDB)
			resp, err := store.ReadMessagesFromDLQ(context.Background(), &persistence.InternalReadMessagesFromDLQRequest{
				FirstMessageID: firstMessageID,
				LastMessageID:  lastMessageID,
				PageSize:       pageSize,
				PageToken:      tc.pageToken,
			})
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, resp.Messages, "Unexpected result for test case")
				assert.Equal(t, tc.wantToken, resp.NextPageToken, "Unexpected result for test case")
			}
		})
	}
}
