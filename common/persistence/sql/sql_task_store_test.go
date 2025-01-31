// Copyright (c) 2020 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

func TestGetTaskListSize(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.GetTaskListSizeRequest
		mockSetup func(*sqlplugin.MockDB)
		want      *persistence.GetTaskListSizeResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.GetTaskListSizeRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskListName: "tl",
				TaskListType: 0,
				AckLevel:     10,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().GetTasksCount(gomock.Any(), &sqlplugin.TasksFilter{
					ShardID:      0,
					DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
					TaskListName: "tl",
					TaskType:     0,
					MinTaskID:    common.Int64Ptr(10),
				}).Return(int64(1), nil)
			},
			want: &persistence.GetTaskListSizeResponse{
				Size: 1,
			},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.GetTaskListSizeRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskListName: "tl",
				TaskListType: 0,
				AckLevel:     10,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				err := errors.New("some error")
				mockDB.EXPECT().GetTasksCount(gomock.Any(), gomock.Any()).Return(int64(0), err)
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
			store := &sqlTaskStore{
				sqlStore: sqlStore{db: mockDB},
			}

			tc.mockSetup(mockDB)

			got, err := store.GetTaskListSize(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestLeaseTaskList(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.LeaseTaskListRequest
		mockSetup func(*sqlplugin.MockDB, *sqlplugin.MockTx, *serialization.MockParser)
		want      *persistence.LeaseTaskListResponse
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case - first lease",
			req: &persistence.LeaseTaskListRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList:     "tl",
				TaskType:     0,
				TaskListKind: persistence.TaskListKindSticky,
				RangeID:      0,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(nil, sql.ErrNoRows)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).DoAndReturn(func(info *serialization.TaskListInfo) (persistence.DataBlob, error) {
					assert.Equal(t, int16(persistence.TaskListKindSticky), info.Kind)
					assert.Equal(t, int64(0), info.AckLevel)
					return persistence.DataBlob{
						Data:     []byte(`tl`),
						Encoding: common.EncodingType("tl"),
					}, nil
				})
				mockDB.EXPECT().SupportsTTL().Return(true)
				mockDB.EXPECT().InsertIntoTaskListsWithTTL(gomock.Any(), &sqlplugin.TaskListsRowWithTTL{
					TaskListsRow: sqlplugin.TaskListsRow{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:         "tl",
						TaskType:     0,
						Data:         []byte(`tl`),
						DataEncoding: "tl",
					},
					TTL: stickyTasksListsTTL,
				}).Return(nil, nil)
				mockParser.EXPECT().TaskListInfoFromBlob([]byte(`tl`), "tl").Return(&serialization.TaskListInfo{
					AckLevel:        0,
					Kind:            int16(persistence.TaskListKindSticky),
					ExpiryTimestamp: time.Unix(0, 0),
					LastUpdated:     time.Unix(0, 1),
				}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), 0).Return(mockTx, nil)
				mockTx.EXPECT().LockTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(int64(0), nil)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().SupportsTTL().Return(true)
				mockTx.EXPECT().UpdateTaskListsWithTTL(gomock.Any(), &sqlplugin.TaskListsRowWithTTL{
					TaskListsRow: sqlplugin.TaskListsRow{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:         "tl",
						TaskType:     0,
						RangeID:      1,
						Data:         []byte(`tl`),
						DataEncoding: "tl",
					},
					TTL: stickyTasksListsTTL,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			want: &persistence.LeaseTaskListResponse{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
					Name:     "tl",
					TaskType: 0,
					RangeID:  1,
					AckLevel: 0,
					Kind:     persistence.TaskListKindSticky,
				},
			},
			wantErr: false,
		},
		{
			name: "Success case - first lease - normal tasklist",
			req: &persistence.LeaseTaskListRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList:     "tl",
				TaskType:     0,
				TaskListKind: persistence.TaskListKindNormal,
				RangeID:      0,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(nil, sql.ErrNoRows)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).DoAndReturn(func(info *serialization.TaskListInfo) (persistence.DataBlob, error) {
					assert.Equal(t, int16(persistence.TaskListKindNormal), info.Kind)
					assert.Equal(t, int64(0), info.AckLevel)
					return persistence.DataBlob{
						Data:     []byte(`tl`),
						Encoding: common.EncodingType("tl"),
					}, nil
				})
				mockDB.EXPECT().SupportsTTL().Return(true)
				mockDB.EXPECT().InsertIntoTaskLists(gomock.Any(),
					&sqlplugin.TaskListsRow{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:         "tl",
						TaskType:     0,
						Data:         []byte(`tl`),
						DataEncoding: "tl",
					},
				).Return(nil, nil)
				mockParser.EXPECT().TaskListInfoFromBlob([]byte(`tl`), "tl").Return(&serialization.TaskListInfo{
					AckLevel:        0,
					Kind:            int16(persistence.TaskListKindSticky),
					ExpiryTimestamp: time.Unix(0, 0),
					LastUpdated:     time.Unix(0, 1),
					AdaptivePartitionConfig: &serialization.TaskListPartitionConfig{
						Version:            0,
						NumReadPartitions:  1,
						ReadPartitions:     map[int32]*serialization.TaskListPartition{0: {}},
						NumWritePartitions: 1,
						WritePartitions:    map[int32]*serialization.TaskListPartition{0: {}},
					},
				}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), 0).Return(mockTx, nil)
				mockTx.EXPECT().LockTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(int64(0), nil)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().SupportsTTL().Return(true)
				mockTx.EXPECT().UpdateTaskListsWithTTL(gomock.Any(), &sqlplugin.TaskListsRowWithTTL{
					TaskListsRow: sqlplugin.TaskListsRow{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:         "tl",
						TaskType:     0,
						RangeID:      1,
						Data:         []byte(`tl`),
						DataEncoding: "tl",
					},
					TTL: stickyTasksListsTTL,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			want: &persistence.LeaseTaskListResponse{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
					Name:     "tl",
					TaskType: 0,
					RangeID:  1,
					AckLevel: 0,
					Kind:     persistence.TaskListKindNormal,
					AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
						Version: 0,
						ReadPartitions: map[int]*persistence.TaskListPartition{
							0: {},
						},
						WritePartitions: map[int]*persistence.TaskListPartition{
							0: {},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Error case - failed to get tasklist",
			req: &persistence.LeaseTaskListRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList:     "tl",
				TaskType:     0,
				TaskListKind: persistence.TaskListKindSticky,
				RangeID:      0,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - failed to insert tasklist",
			req: &persistence.LeaseTaskListRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList:     "tl",
				TaskType:     0,
				TaskListKind: persistence.TaskListKindSticky,
				RangeID:      0,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().SupportsTTL().Return(true)
				err := errors.New("some error")
				mockDB.EXPECT().InsertIntoTaskListsWithTTL(gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - condition failed",
			req: &persistence.LeaseTaskListRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList:     "tl",
				TaskType:     0,
				TaskListKind: persistence.TaskListKindSticky,
				RangeID:      1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(nil, sql.ErrNoRows)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().SupportsTTL().Return(true)
				mockDB.EXPECT().InsertIntoTaskListsWithTTL(gomock.Any(), &sqlplugin.TaskListsRowWithTTL{
					TaskListsRow: sqlplugin.TaskListsRow{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:         "tl",
						TaskType:     0,
						Data:         []byte(`tl`),
						DataEncoding: "tl",
					},
					TTL: stickyTasksListsTTL,
				}).Return(nil, nil)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *persistence.ConditionFailedError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be ConditionFailedError")
			},
		},
		{
			name: "Error case - failed to lock tasklist",
			req: &persistence.LeaseTaskListRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList:     "tl",
				TaskType:     0,
				TaskListKind: persistence.TaskListKindSticky,
				RangeID:      0,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(nil, sql.ErrNoRows)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().SupportsTTL().Return(true)
				mockDB.EXPECT().InsertIntoTaskListsWithTTL(gomock.Any(), &sqlplugin.TaskListsRowWithTTL{
					TaskListsRow: sqlplugin.TaskListsRow{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:         "tl",
						TaskType:     0,
						Data:         []byte(`tl`),
						DataEncoding: "tl",
					},
					TTL: stickyTasksListsTTL,
				}).Return(nil, nil)
				mockParser.EXPECT().TaskListInfoFromBlob([]byte(`tl`), "tl").Return(&serialization.TaskListInfo{
					AckLevel:        0,
					Kind:            int16(persistence.TaskListKindSticky),
					ExpiryTimestamp: time.Unix(0, 0),
					LastUpdated:     time.Unix(0, 1),
				}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), 0).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().LockTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(int64(0), err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			wantErr: true,
		},
		{
			name: "Error case - failed to update tasklists",
			req: &persistence.LeaseTaskListRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList:     "tl",
				TaskType:     0,
				TaskListKind: persistence.TaskListKindSticky,
				RangeID:      0,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(nil, sql.ErrNoRows)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().SupportsTTL().Return(true)
				mockDB.EXPECT().InsertIntoTaskListsWithTTL(gomock.Any(), &sqlplugin.TaskListsRowWithTTL{
					TaskListsRow: sqlplugin.TaskListsRow{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:         "tl",
						TaskType:     0,
						Data:         []byte(`tl`),
						DataEncoding: "tl",
					},
					TTL: stickyTasksListsTTL,
				}).Return(nil, nil)
				mockParser.EXPECT().TaskListInfoFromBlob([]byte(`tl`), "tl").Return(&serialization.TaskListInfo{
					AckLevel:        0,
					Kind:            int16(persistence.TaskListKindSticky),
					ExpiryTimestamp: time.Unix(0, 0),
					LastUpdated:     time.Unix(0, 1),
				}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), 0).Return(mockTx, nil)
				mockTx.EXPECT().LockTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(int64(0), nil)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().SupportsTTL().Return(true)
				err := errors.New("some error")
				mockTx.EXPECT().UpdateTaskListsWithTTL(gomock.Any(), &sqlplugin.TaskListsRowWithTTL{
					TaskListsRow: sqlplugin.TaskListsRow{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:         "tl",
						TaskType:     0,
						RangeID:      1,
						Data:         []byte(`tl`),
						DataEncoding: "tl",
					},
					TTL: stickyTasksListsTTL,
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
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
			mockParser := serialization.NewMockParser(ctrl)
			store := &sqlTaskStore{
				sqlStore: sqlStore{db: mockDB, parser: mockParser},
			}

			tc.mockSetup(mockDB, mockTx, mockParser)

			got, err := store.LeaseTaskList(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				got.TaskListInfo.LastUpdated = tc.want.TaskListInfo.LastUpdated
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestGetTaskList(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.GetTaskListRequest
		mockSetup func(*sqlplugin.MockDB, *serialization.MockParser)
		want      *persistence.GetTaskListResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.GetTaskListRequest{
				DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList: "tl",
				TaskType: 1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, filter *sqlplugin.TaskListsFilter) ([]sqlplugin.TaskListsRow, error) {
					assert.Equal(t, serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"), *filter.DomainID)
					assert.Equal(t, "tl", *filter.Name)
					assert.Equal(t, int64(1), *filter.TaskType)
					return []sqlplugin.TaskListsRow{
						{
							ShardID:      11,
							DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
							Name:         "tl",
							TaskType:     1,
							RangeID:      123,
							Data:         []byte(`tl`),
							DataEncoding: "tl",
						},
					}, nil
				})
				mockParser.EXPECT().TaskListInfoFromBlob([]byte(`tl`), "tl").Return(&serialization.TaskListInfo{
					Kind:            1,
					AckLevel:        2,
					ExpiryTimestamp: time.Unix(1, 4),
					LastUpdated:     time.Unix(10, 0),
					AdaptivePartitionConfig: &serialization.TaskListPartitionConfig{
						Version:            0,
						NumReadPartitions:  1,
						NumWritePartitions: 1,
						ReadPartitions: map[int32]*serialization.TaskListPartition{
							0: {
								IsolationGroups: []string{"foo"},
							},
						},
						WritePartitions: map[int32]*serialization.TaskListPartition{
							0: {
								IsolationGroups: []string{"bar"},
							},
						},
					},
				}, nil)
			},
			want: &persistence.GetTaskListResponse{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID:    "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
					Name:        "tl",
					TaskType:    1,
					RangeID:     123,
					Kind:        1,
					AckLevel:    2,
					Expiry:      time.Unix(1, 4),
					LastUpdated: time.Unix(10, 0),
					AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
						Version: 0,
						ReadPartitions: map[int]*persistence.TaskListPartition{
							0: {
								IsolationGroups: []string{"foo"},
							},
						},
						WritePartitions: map[int]*persistence.TaskListPartition{
							0: {
								IsolationGroups: []string{"bar"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Partition counts instead of data",
			req: &persistence.GetTaskListRequest{
				DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList: "tl",
				TaskType: 1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, filter *sqlplugin.TaskListsFilter) ([]sqlplugin.TaskListsRow, error) {
					assert.Equal(t, serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"), *filter.DomainID)
					assert.Equal(t, "tl", *filter.Name)
					assert.Equal(t, int64(1), *filter.TaskType)
					return []sqlplugin.TaskListsRow{
						{
							ShardID:      11,
							DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
							Name:         "tl",
							TaskType:     1,
							RangeID:      123,
							Data:         []byte(`tl`),
							DataEncoding: "tl",
						},
					}, nil
				})
				mockParser.EXPECT().TaskListInfoFromBlob([]byte(`tl`), "tl").Return(&serialization.TaskListInfo{
					Kind:            1,
					AckLevel:        2,
					ExpiryTimestamp: time.Unix(1, 4),
					LastUpdated:     time.Unix(10, 0),
					AdaptivePartitionConfig: &serialization.TaskListPartitionConfig{
						Version:            0,
						NumReadPartitions:  2,
						NumWritePartitions: 2,
					},
				}, nil)
			},
			want: &persistence.GetTaskListResponse{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID:    "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
					Name:        "tl",
					TaskType:    1,
					RangeID:     123,
					Kind:        1,
					AckLevel:    2,
					Expiry:      time.Unix(1, 4),
					LastUpdated: time.Unix(10, 0),
					AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
						Version: 0,
						ReadPartitions: map[int]*persistence.TaskListPartition{
							0: {},
							1: {},
						},
						WritePartitions: map[int]*persistence.TaskListPartition{
							0: {},
							1: {},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.GetTaskListRequest{
				DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList: "tl",
				TaskType: 0,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				err := errors.New("some error")
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockParser := serialization.NewMockParser(ctrl)
			store := &sqlTaskStore{
				sqlStore: sqlStore{db: mockDB, parser: mockParser},
				nShards:  1000,
			}

			tc.mockSetup(mockDB, mockParser)

			got, err := store.GetTaskList(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestUpdateTaskList(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.UpdateTaskListRequest
		mockSetup func(*sqlplugin.MockDB, *sqlplugin.MockTx, *serialization.MockParser)
		want      *persistence.UpdateTaskListResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.UpdateTaskListRequest{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
					Name:     "tl",
					TaskType: 0,
					RangeID:  1,
					AckLevel: 0,
					Kind:     persistence.TaskListKindSticky,
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().LockTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(int64(1), nil)
				mockDB.EXPECT().SupportsTTL().Return(true)
				mockTx.EXPECT().UpdateTaskListsWithTTL(gomock.Any(), &sqlplugin.TaskListsRowWithTTL{
					TaskListsRow: sqlplugin.TaskListsRow{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:         "tl",
						TaskType:     0,
						RangeID:      1,
						Data:         []byte(`tl`),
						DataEncoding: "tl",
					},
					TTL: stickyTasksListsTTL,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			want:    &persistence.UpdateTaskListResponse{},
			wantErr: false,
		},
		{
			name: "Success case - normal tasklist",
			req: &persistence.UpdateTaskListRequest{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
					Name:     "tl",
					TaskType: 0,
					RangeID:  1,
					AckLevel: 0,
					Kind:     persistence.TaskListKindNormal,
					AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
						Version: 0,
						ReadPartitions: map[int]*persistence.TaskListPartition{
							0: {
								IsolationGroups: []string{"foo"},
							},
						},
						WritePartitions: map[int]*persistence.TaskListPartition{
							0: {
								IsolationGroups: []string{"bar"},
							},
						},
					},
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).DoAndReturn(func(info *serialization.TaskListInfo) (persistence.DataBlob, error) {
					assert.Equal(t, int16(persistence.TaskListKindNormal), info.Kind)
					assert.Equal(t, int64(0), info.AckLevel)
					assert.Equal(t, int64(0), info.AdaptivePartitionConfig.Version)
					assert.Equal(t, int32(1), info.AdaptivePartitionConfig.NumReadPartitions)
					assert.Equal(t, int32(1), info.AdaptivePartitionConfig.NumWritePartitions)
					assert.Equal(t, "foo", info.AdaptivePartitionConfig.ReadPartitions[0].IsolationGroups[0])
					assert.Equal(t, "bar", info.AdaptivePartitionConfig.WritePartitions[0].IsolationGroups[0])
					return persistence.DataBlob{
						Data:     []byte(`tl`),
						Encoding: common.EncodingType("tl"),
					}, nil
				})
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().LockTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(int64(1), nil)
				mockDB.EXPECT().SupportsTTL().Return(true)
				mockTx.EXPECT().UpdateTaskLists(gomock.Any(),
					&sqlplugin.TaskListsRow{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:         "tl",
						TaskType:     0,
						RangeID:      1,
						Data:         []byte(`tl`),
						DataEncoding: "tl",
					},
				).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			want:    &persistence.UpdateTaskListResponse{},
			wantErr: false,
		},
		{
			name: "Error case - failed to lock task list",
			req: &persistence.UpdateTaskListRequest{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
					Name:     "tl",
					TaskType: 0,
					RangeID:  1,
					AckLevel: 0,
					Kind:     persistence.TaskListKindSticky,
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().LockTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(int64(1), err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			want:    &persistence.UpdateTaskListResponse{},
			wantErr: true,
		},
		{
			name: "Error case - failed to update task list",
			req: &persistence.UpdateTaskListRequest{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
					Name:     "tl",
					TaskType: 0,
					RangeID:  1,
					AckLevel: 0,
					Kind:     persistence.TaskListKindSticky,
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockParser.EXPECT().TaskListInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().LockTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
				}).Return(int64(1), nil)
				err := errors.New("some error")
				mockDB.EXPECT().SupportsTTL().Return(false)
				mockTx.EXPECT().UpdateTaskLists(gomock.Any(), &sqlplugin.TaskListsRow{
					ShardID:      0,
					DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
					Name:         "tl",
					TaskType:     0,
					RangeID:      1,
					Data:         []byte(`tl`),
					DataEncoding: "tl",
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			want:    &persistence.UpdateTaskListResponse{},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)
			store := &sqlTaskStore{
				sqlStore: sqlStore{db: mockDB, parser: mockParser},
			}

			tc.mockSetup(mockDB, mockTx, mockParser)

			got, err := store.UpdateTaskList(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestListTaskList(t *testing.T) {
	pageSize := 1
	testCases := []struct {
		name      string
		pageToken *taskListPageToken
		mockSetup func(*sqlplugin.MockDB, *serialization.MockParser)
		want      *persistence.ListTaskListResponse
		wantToken *taskListPageToken
		wantErr   bool
	}{
		{
			name: "Success case",
			pageToken: &taskListPageToken{
				ShardID:  10,
				DomainID: serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
				Name:     "tl",
				TaskType: 0,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, filter *sqlplugin.TaskListsFilter) ([]sqlplugin.TaskListsRow, error) {
					assert.Equal(t, 10, filter.ShardID)
					assert.Equal(t, serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"), *filter.DomainIDGreaterThan)
					assert.Equal(t, "tl", *filter.NameGreaterThan)
					assert.Equal(t, int64(0), *filter.TaskTypeGreaterThan)
					assert.Equal(t, 1, *filter.PageSize)
					return []sqlplugin.TaskListsRow{}, nil
				})
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, filter *sqlplugin.TaskListsFilter) ([]sqlplugin.TaskListsRow, error) {
					assert.Equal(t, 11, filter.ShardID)
					assert.Equal(t, serialization.UUID{}, *filter.DomainIDGreaterThan)
					assert.Equal(t, "", *filter.NameGreaterThan)
					assert.Equal(t, int64(math.MinInt16), *filter.TaskTypeGreaterThan)
					assert.Equal(t, 1, *filter.PageSize)
					return []sqlplugin.TaskListsRow{
						{
							ShardID:      11,
							DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
							Name:         "tl",
							TaskType:     1,
							RangeID:      123,
							Data:         []byte(`tl`),
							DataEncoding: "tl",
						},
					}, nil
				})
				mockParser.EXPECT().TaskListInfoFromBlob([]byte(`tl`), "tl").Return(&serialization.TaskListInfo{
					Kind:            1,
					AckLevel:        2,
					ExpiryTimestamp: time.Unix(1, 4),
					LastUpdated:     time.Unix(10, 0),
				}, nil)
			},
			want: &persistence.ListTaskListResponse{
				Items: []persistence.TaskListInfo{
					{
						DomainID:    "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
						Name:        "tl",
						TaskType:    1,
						RangeID:     123,
						Kind:        1,
						AckLevel:    2,
						Expiry:      time.Unix(1, 4),
						LastUpdated: time.Unix(10, 0),
					},
				},
				NextPageToken: func() []byte {
					token, err := gobSerialize(&taskListPageToken{
						ShardID:  11,
						DomainID: serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
						Name:     "tl",
						TaskType: 1,
					})
					require.NoError(t, err, "failed to serialize page token")
					return token
				}(),
			},
			wantErr: false,
		},
		{
			name: "Error case",
			pageToken: &taskListPageToken{
				ShardID:  10,
				DomainID: serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
				Name:     "tl",
				TaskType: 0,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromTaskLists(gomock.Any(), gomock.Any()).Return(nil, err)
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
			mockParser := serialization.NewMockParser(ctrl)
			store := &sqlTaskStore{
				sqlStore: sqlStore{db: mockDB, parser: mockParser},
				nShards:  1000,
			}

			tc.mockSetup(mockDB, mockParser)

			pageToken, err := gobSerialize(tc.pageToken)
			require.NoError(t, err, "invalid pageToken")
			req := &persistence.ListTaskListRequest{
				PageSize:  pageSize,
				PageToken: pageToken,
			}
			got, err := store.ListTaskList(context.Background(), req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestDeleteTaskList(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.DeleteTaskListRequest
		mockSetup func(*sqlplugin.MockDB)
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.DeleteTaskListRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskListName: "tl",
				TaskListType: 0,
				RangeID:      10,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().DeleteFromTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(0),
					RangeID:  common.Int64Ptr(10),
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.DeleteTaskListRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskListName: "tl",
				TaskListType: 0,
				RangeID:      10,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				err := errors.New("some error")
				mockDB.EXPECT().DeleteFromTaskLists(gomock.Any(), gomock.Any()).Return(nil, err)
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
			store := &sqlTaskStore{
				sqlStore: sqlStore{db: mockDB},
			}

			tc.mockSetup(mockDB)

			err := store.DeleteTaskList(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestCreateTasks(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.CreateTasksRequest
		mockSetup func(*sqlplugin.MockDB, *sqlplugin.MockTx, *serialization.MockParser)
		want      *persistence.CreateTasksResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.CreateTasksRequest{
				TaskListInfo: &persistence.TaskListInfo{
					Name:     "tl",
					TaskType: 1,
					RangeID:  9,
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				},
				Tasks: []*persistence.CreateTaskInfo{
					{
						Data: &persistence.TaskInfo{
							ScheduleToStartTimeoutSeconds: 1,
							DomainID:                      "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
							TaskID:                        999,
						},
						TaskID: 999,
					},
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SupportsTTL().Return(true).Times(4)
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().MaxAllowedTTL().Return(common.DurationPtr(time.Hour), nil)
				mockParser.EXPECT().TaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().InsertIntoTasksWithTTL(gomock.Any(), []sqlplugin.TasksRowWithTTL{
					{
						TasksRow: sqlplugin.TasksRow{
							ShardID:      0,
							DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
							TaskListName: "tl",
							TaskType:     1,
							TaskID:       999,
							Data:         []byte(`tl`),
							DataEncoding: "tl",
						},
						TTL: common.DurationPtr(time.Second),
					},
				}).Return(nil, nil)
				mockTx.EXPECT().LockTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(1),
				}).Return(int64(9), nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			want:    &persistence.CreateTasksResponse{},
			wantErr: false,
		},
		{
			name: "Error case - failed to insert tasks",
			req: &persistence.CreateTasksRequest{
				TaskListInfo: &persistence.TaskListInfo{
					Name:     "tl",
					TaskType: 1,
					RangeID:  9,
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				},
				Tasks: []*persistence.CreateTaskInfo{
					{
						Data: &persistence.TaskInfo{
							ScheduleToStartTimeoutSeconds: 1,
							DomainID:                      "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
							TaskID:                        999,
						},
						TaskID: 999,
					},
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SupportsTTL().Return(true).Times(4)
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().MaxAllowedTTL().Return(common.DurationPtr(time.Hour), nil)
				mockParser.EXPECT().TaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoTasksWithTTL(gomock.Any(), []sqlplugin.TasksRowWithTTL{
					{
						TasksRow: sqlplugin.TasksRow{
							ShardID:      0,
							DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
							TaskListName: "tl",
							TaskType:     1,
							TaskID:       999,
							Data:         []byte(`tl`),
							DataEncoding: "tl",
						},
						TTL: common.DurationPtr(time.Second),
					},
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			wantErr: true,
		},
		{
			name: "Error case - failed to lock tasklist",
			req: &persistence.CreateTasksRequest{
				TaskListInfo: &persistence.TaskListInfo{
					Name:     "tl",
					TaskType: 1,
					RangeID:  9,
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				},
				Tasks: []*persistence.CreateTaskInfo{
					{
						Data: &persistence.TaskInfo{
							ScheduleToStartTimeoutSeconds: 1,
							DomainID:                      "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
							TaskID:                        999,
						},
						TaskID: 999,
					},
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SupportsTTL().Return(true).Times(4)
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().MaxAllowedTTL().Return(common.DurationPtr(time.Hour), nil)
				mockParser.EXPECT().TaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Data:     []byte(`tl`),
					Encoding: common.EncodingType("tl"),
				}, nil)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().InsertIntoTasksWithTTL(gomock.Any(), []sqlplugin.TasksRowWithTTL{
					{
						TasksRow: sqlplugin.TasksRow{
							ShardID:      0,
							DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
							TaskListName: "tl",
							TaskType:     1,
							TaskID:       999,
							Data:         []byte(`tl`),
							DataEncoding: "tl",
						},
						TTL: common.DurationPtr(time.Second),
					},
				}).Return(nil, nil)
				err := errors.New("some error")
				mockTx.EXPECT().LockTaskLists(gomock.Any(), &sqlplugin.TaskListsFilter{
					ShardID:  0,
					DomainID: serialization.UUIDPtr(serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0")),
					Name:     common.StringPtr("tl"),
					TaskType: common.Int64Ptr(1),
				}).Return(int64(0), err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
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
			mockParser := serialization.NewMockParser(ctrl)
			store := &sqlTaskStore{
				sqlStore: sqlStore{db: mockDB, parser: mockParser},
			}

			tc.mockSetup(mockDB, mockTx, mockParser)

			got, err := store.CreateTasks(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestGetTasks(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.GetTasksRequest
		mockSetup func(*sqlplugin.MockDB, *serialization.MockParser)
		want      *persistence.GetTasksResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.GetTasksRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList:     "tl",
				TaskType:     0,
				ReadLevel:    10,
				MaxReadLevel: common.Int64Ptr(9999),
				BatchSize:    1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().SelectFromTasks(gomock.Any(), &sqlplugin.TasksFilter{
					ShardID:      0,
					DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
					TaskListName: "tl",
					TaskType:     0,
					MinTaskID:    common.Int64Ptr(10),
					MaxTaskID:    common.Int64Ptr(9999),
					PageSize:     common.IntPtr(1),
				}).Return([]sqlplugin.TasksRow{
					{
						TaskID:       888,
						Data:         []byte(`task`),
						DataEncoding: "task",
					},
				}, nil)
				mockParser.EXPECT().TaskInfoFromBlob([]byte(`task`), "task").Return(&serialization.TaskInfo{
					WorkflowID:       "test",
					RunID:            serialization.MustParseUUID("b9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
					ScheduleID:       7,
					ExpiryTimestamp:  time.Unix(9, 0),
					CreatedTimestamp: time.Unix(8, 7),
					PartitionConfig:  map[string]string{"a": "b"},
				}, nil)
			},
			want: &persistence.GetTasksResponse{
				Tasks: []*persistence.TaskInfo{
					{
						DomainID:        "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
						WorkflowID:      "test",
						RunID:           "b9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
						TaskID:          888,
						ScheduleID:      7,
						Expiry:          time.Unix(9, 0),
						CreatedTime:     time.Unix(8, 7),
						PartitionConfig: map[string]string{"a": "b"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.GetTasksRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskList:     "tl",
				TaskType:     0,
				ReadLevel:    10,
				MaxReadLevel: common.Int64Ptr(9999),
				BatchSize:    1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromTasks(gomock.Any(), &sqlplugin.TasksFilter{
					ShardID:      0,
					DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
					TaskListName: "tl",
					TaskType:     0,
					MinTaskID:    common.Int64Ptr(10),
					MaxTaskID:    common.Int64Ptr(9999),
					PageSize:     common.IntPtr(1),
				}).Return(nil, err)
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
			mockParser := serialization.NewMockParser(ctrl)
			store := &sqlTaskStore{
				sqlStore: sqlStore{db: mockDB, parser: mockParser},
			}

			tc.mockSetup(mockDB, mockParser)

			got, err := store.GetTasks(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestCompleteTask(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.CompleteTaskRequest
		mockSetup func(*sqlplugin.MockDB)
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.CompleteTaskRequest{
				TaskList: &persistence.TaskListInfo{
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
					Name:     "tl",
					TaskType: 0,
				},
				TaskID: 1001,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().DeleteFromTasks(gomock.Any(), &sqlplugin.TasksFilter{
					ShardID:      0,
					DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
					TaskListName: "tl",
					TaskType:     0,
					TaskID:       common.Int64Ptr(1001),
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.CompleteTaskRequest{
				TaskList: &persistence.TaskListInfo{
					DomainID: "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
					Name:     "tl",
					TaskType: 0,
				},
				TaskID: 1001,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				err := errors.New("some error")
				mockDB.EXPECT().DeleteFromTasks(gomock.Any(), &sqlplugin.TasksFilter{
					ShardID:      0,
					DomainID:     serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
					TaskListName: "tl",
					TaskType:     0,
					TaskID:       common.Int64Ptr(1001),
				}).Return(nil, err)
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
			store := &sqlTaskStore{
				sqlStore: sqlStore{db: mockDB},
			}

			tc.mockSetup(mockDB)

			err := store.CompleteTask(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestCompleteTaskLessThan(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.CompleteTasksLessThanRequest
		mockSetup func(*sqlplugin.MockDB)
		want      *persistence.CompleteTasksLessThanResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.CompleteTasksLessThanRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskListName: "tl",
				TaskType:     0,
				TaskID:       1001,
				Limit:        100,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().DeleteFromTasks(gomock.Any(), &sqlplugin.TasksFilter{
					ShardID:              0,
					DomainID:             serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
					TaskListName:         "tl",
					TaskType:             0,
					TaskIDLessThanEquals: common.Int64Ptr(1001),
					Limit:                common.IntPtr(100),
				}).Return(&sqlResult{rowsAffected: 100}, nil)
			},
			want:    &persistence.CompleteTasksLessThanResponse{TasksCompleted: 100},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.CompleteTasksLessThanRequest{
				DomainID:     "c9488dc7-20b2-44c3-b2e4-bfea5af62ac0",
				TaskListName: "tl",
				TaskType:     0,
				TaskID:       1001,
				Limit:        100,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				err := errors.New("some error")
				mockDB.EXPECT().DeleteFromTasks(gomock.Any(), &sqlplugin.TasksFilter{
					ShardID:              0,
					DomainID:             serialization.MustParseUUID("c9488dc7-20b2-44c3-b2e4-bfea5af62ac0"),
					TaskListName:         "tl",
					TaskType:             0,
					TaskIDLessThanEquals: common.Int64Ptr(1001),
					Limit:                common.IntPtr(100),
				}).Return(nil, err)
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
			store := &sqlTaskStore{
				sqlStore: sqlStore{db: mockDB},
			}

			tc.mockSetup(mockDB)

			got, err := store.CompleteTasksLessThan(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestHelp(t *testing.T) {
	shard := sqlplugin.GetDBShardIDFromDomainIDAndTasklist("c4b5cb22-c213-4812-bb4a-fc1ade5405ef", "pgtasklist", 16384)
	println("shard: ", shard)
	// BgAKAAAKAAwAAAAABQ+kWAoADgAAAAAAAAAACgAQGB6uFsU9cvEMABIKAAoAAAAAAAAAAQgADAAAAAAIAA4AAAAAAAA=
	parser, err := serialization.NewParser(common.EncodingTypeThriftRW, common.EncodingTypeThriftRW)
	require.NoError(t, err)
	data, err := base64.StdEncoding.DecodeString("BgAKAAAKAAwAAAAABGGFYAoADgAAAAAAAAAACgAQGB6uGPaVqOUMABIKAAoAAAAAAAAAAQgADAAAAAIIAA4AAAACAAA=")
	require.NoError(t, err)
	info, err := parser.TaskListInfoFromBlob(data, "thriftrw")
	require.NoError(t, err)
	fmt.Printf("info: %v", info)
}
