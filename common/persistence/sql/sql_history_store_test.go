// Copyright (c) 2018 Uber Technologies, Inc.
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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

func TestGetHistoryTree(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.InternalGetHistoryTreeRequest
		mockSetup func(*sqlplugin.MockDB, *serialization.MockParser)
		want      *persistence.InternalGetHistoryTreeResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.InternalGetHistoryTreeRequest{
				TreeID:  "530ec3d3-f74b-423f-a138-3b35494fe691",
				ShardID: common.IntPtr(1),
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
					TreeID:  serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					ShardID: 1,
				}).Return([]sqlplugin.HistoryTreeRow{
					{
						ShardID:      1,
						TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
						BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
						Data:         []byte(`aaaa`),
						DataEncoding: "json",
					},
				}, nil)
				mockParser.EXPECT().HistoryTreeInfoFromBlob([]byte(`aaaa`), "json").Return(&serialization.HistoryTreeInfo{
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   3,
						},
					},
				}, nil)
			},
			want: &persistence.InternalGetHistoryTreeResponse{
				Branches: []*types.HistoryBranch{
					{
						TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
						BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
						Ancestors: []*types.HistoryBranchRange{
							{
								BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
								BeginNodeID: 1,
								EndNodeID:   3,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Success case - no record",
			req: &persistence.InternalGetHistoryTreeRequest{
				TreeID:  "530ec3d3-f74b-423f-a138-3b35494fe691",
				ShardID: common.IntPtr(1),
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
					TreeID:  serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					ShardID: 1,
				}).Return(nil, sql.ErrNoRows)
			},
			want:    &persistence.InternalGetHistoryTreeResponse{},
			wantErr: false,
		},
		{
			name: "Error case - database failure",
			req: &persistence.InternalGetHistoryTreeRequest{
				TreeID:  "530ec3d3-f74b-423f-a138-3b35494fe691",
				ShardID: common.IntPtr(1),
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
					TreeID:  serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					ShardID: 1,
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - decode error",
			req: &persistence.InternalGetHistoryTreeRequest{
				TreeID:  "530ec3d3-f74b-423f-a138-3b35494fe691",
				ShardID: common.IntPtr(1),
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
					TreeID:  serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					ShardID: 1,
				}).Return([]sqlplugin.HistoryTreeRow{
					{
						ShardID:      1,
						TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
						BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
						Data:         []byte(`aaaa`),
						DataEncoding: "json",
					},
				}, nil)
				mockParser.EXPECT().HistoryTreeInfoFromBlob([]byte(`aaaa`), "json").Return(nil, errors.New("some error"))
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
			store, err := NewHistoryV2Persistence(mockDB, nil, mockParser, nil)
			require.NoError(t, err, "Failed to create sql history store")

			tc.mockSetup(mockDB, mockParser)
			got, err := store.GetHistoryTree(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestDeleteHistoryBranch(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.InternalDeleteHistoryBranchRequest
		mockSetup func(*sqlplugin.MockDB, *sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.InternalDeleteHistoryBranchRequest{
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				ShardID: 1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
					TreeID:  serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					ShardID: 1,
				}).Return([]sqlplugin.HistoryTreeRow{
					{
						ShardID:      1,
						TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
						BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
						Data:         []byte(`aaaa`),
						DataEncoding: "json",
					},
				}, nil)
				mockParser.EXPECT().HistoryTreeInfoFromBlob([]byte(`aaaa`), "json").Return(&serialization.HistoryTreeInfo{
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   3,
						},
					},
				}, nil)

				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().DeleteFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
					TreeID:   serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					BranchID: serialization.UUIDPtr(serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691")),
					ShardID:  1,
				}).Return(nil, nil)
				mockTx.EXPECT().DeleteFromHistoryNode(gomock.Any(), &sqlplugin.HistoryNodeFilter{
					TreeID:    serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					BranchID:  serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
					ShardID:   1,
					PageSize:  1000,
					MinNodeID: common.Int64Ptr(10),
				}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().DeleteFromHistoryNode(gomock.Any(), &sqlplugin.HistoryNodeFilter{
					TreeID:    serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					BranchID:  serialization.MustParseUUID("730ec3d3-f74b-423f-a138-3b35494fe691"),
					ShardID:   1,
					PageSize:  1000,
					MinNodeID: common.Int64Ptr(3),
				}).Return(&sqlResult{rowsAffected: 7}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name: "Error case - failed to get history tree",
			req: &persistence.InternalDeleteHistoryBranchRequest{
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				ShardID: 1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - failed to delete history tree",
			req: &persistence.InternalDeleteHistoryBranchRequest{
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				ShardID: 1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
					TreeID:  serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					ShardID: 1,
				}).Return([]sqlplugin.HistoryTreeRow{
					{
						ShardID:      1,
						TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
						BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
						Data:         []byte(`aaaa`),
						DataEncoding: "json",
					},
				}, nil)
				mockParser.EXPECT().HistoryTreeInfoFromBlob([]byte(`aaaa`), "json").Return(&serialization.HistoryTreeInfo{
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   3,
						},
					},
				}, nil)

				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().DeleteFromHistoryTree(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - failed to delete history node",
			req: &persistence.InternalDeleteHistoryBranchRequest{
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				ShardID: 1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), gomock.Any()).Return([]sqlplugin.HistoryTreeRow{
					{
						ShardID:      1,
						TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
						BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
						Data:         []byte(`aaaa`),
						DataEncoding: "json",
					},
				}, nil)
				mockParser.EXPECT().HistoryTreeInfoFromBlob(gomock.Any(), gomock.Any()).Return(&serialization.HistoryTreeInfo{
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   3,
						},
					},
				}, nil)

				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().DeleteFromHistoryTree(gomock.Any(), gomock.Any()).Return(nil, nil)
				err := errors.New("some error")
				mockTx.EXPECT().DeleteFromHistoryNode(gomock.Any(), gomock.Any()).Return(nil, err)
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
			mockParser := serialization.NewMockParser(ctrl)
			store, err := NewHistoryV2Persistence(mockDB, nil, mockParser, nil)
			require.NoError(t, err, "Failed to create sql history store")

			tc.mockSetup(mockDB, mockTx, mockParser)
			err = store.DeleteHistoryBranch(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestForkHistoryBranch(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		req       *persistence.InternalForkHistoryBranchRequest
		mockSetup func(*sqlplugin.MockDB, *serialization.MockParser)
		want      *persistence.InternalForkHistoryBranchResponse
		wantErr   bool
	}{
		{
			name: "Success case - case 1",
			req: &persistence.InternalForkHistoryBranchRequest{
				ForkBranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   2,
						},
						{
							BranchID:    "830ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 3,
							EndNodeID:   5,
						},
					},
				},
				ForkNodeID:       4,
				NewBranchID:      "630ec3d3-f74b-423f-a138-3b35494fe699",
				Info:             "test",
				ShardID:          1,
				CurrentTimeStamp: now,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockParser.EXPECT().HistoryTreeInfoToBlob(gomock.Any()).DoAndReturn(func(info *serialization.HistoryTreeInfo) (persistence.DataBlob, error) {
					assert.WithinDuration(t, now, info.CreatedTimestamp, time.Second)
					return persistence.DataBlob{}, nil
				})
				mockDB.EXPECT().InsertIntoHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeRow{
					ShardID:  1,
					TreeID:   serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					BranchID: serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe699"),
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			want: &persistence.InternalForkHistoryBranchResponse{
				NewBranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe699",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   2,
						},
						{
							BranchID:    "830ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 3,
							EndNodeID:   4,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Success case - case 2",
			req: &persistence.InternalForkHistoryBranchRequest{
				ForkBranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   2,
						},
					},
				},
				ForkNodeID:  4,
				NewBranchID: "630ec3d3-f74b-423f-a138-3b35494fe699",
				Info:        "test",
				ShardID:     1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockParser.EXPECT().HistoryTreeInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil)
				mockDB.EXPECT().InsertIntoHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeRow{
					ShardID:  1,
					TreeID:   serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					BranchID: serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe699"),
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			want: &persistence.InternalForkHistoryBranchResponse{
				NewBranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe699",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   2,
						},
						{
							BranchID:    "630ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 2,
							EndNodeID:   4,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Error case - failed to encode blob",
			req: &persistence.InternalForkHistoryBranchRequest{
				ForkBranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   2,
						},
					},
				},
				ForkNodeID:  4,
				NewBranchID: "630ec3d3-f74b-423f-a138-3b35494fe699",
				Info:        "test",
				ShardID:     1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockParser.EXPECT().HistoryTreeInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "Error case - failed to insert data",
			req: &persistence.InternalForkHistoryBranchRequest{
				ForkBranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   2,
						},
					},
				},
				ForkNodeID:  4,
				NewBranchID: "630ec3d3-f74b-423f-a138-3b35494fe699",
				Info:        "test",
				ShardID:     1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockParser.EXPECT().HistoryTreeInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil)
				err := errors.New("some error")
				mockDB.EXPECT().InsertIntoHistoryTree(gomock.Any(), gomock.Any()).Return(nil, err)
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
			store, err := NewHistoryV2Persistence(mockDB, nil, mockParser, nil)
			require.NoError(t, err, "Failed to create sql history store")

			tc.mockSetup(mockDB, mockParser)
			got, err := store.ForkHistoryBranch(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestAppendHistoryNodes(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		req       *persistence.InternalAppendHistoryNodesRequest
		mockSetup func(*sqlplugin.MockDB, *sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case - new branch",
			req: &persistence.InternalAppendHistoryNodesRequest{
				IsNewBranch: true,
				Info:        "test",
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				NodeID:           11,
				Events:           &persistence.DataBlob{},
				TransactionID:    100,
				ShardID:          1,
				CurrentTimeStamp: now,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().HistoryTreeInfoToBlob(gomock.Any()).DoAndReturn(func(info *serialization.HistoryTreeInfo) (persistence.DataBlob, error) {
					assert.WithinDuration(t, now, info.CreatedTimestamp, time.Second)
					return persistence.DataBlob{}, nil
				})
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().InsertIntoHistoryNode(gomock.Any(), &sqlplugin.HistoryNodeRow{
					TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
					NodeID:       11,
					TxnID:        common.Int64Ptr(100),
					Data:         nil,
					DataEncoding: "",
					ShardID:      1,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().InsertIntoHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeRow{
					ShardID:      1,
					TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
					Data:         nil,
					DataEncoding: "",
				}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name: "Success case - old branch",
			req: &persistence.InternalAppendHistoryNodesRequest{
				IsNewBranch: false,
				Info:        "test",
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				NodeID:        11,
				Events:        &persistence.DataBlob{},
				TransactionID: 100,
				ShardID:       1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockDB.EXPECT().InsertIntoHistoryNode(gomock.Any(), &sqlplugin.HistoryNodeRow{
					TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
					NodeID:       11,
					TxnID:        common.Int64Ptr(100),
					Data:         nil,
					DataEncoding: "",
					ShardID:      1,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case - new branch, failed to encode data",
			req: &persistence.InternalAppendHistoryNodesRequest{
				IsNewBranch: true,
				Info:        "test",
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				NodeID:        11,
				Events:        &persistence.DataBlob{},
				TransactionID: 100,
				ShardID:       1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().HistoryTreeInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "Error case - new branch, failed to insert history node",
			req: &persistence.InternalAppendHistoryNodesRequest{
				IsNewBranch: true,
				Info:        "test",
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				NodeID:        11,
				Events:        &persistence.DataBlob{},
				TransactionID: 100,
				ShardID:       1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().HistoryTreeInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil)
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoHistoryNode(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - new branch, failed to insert history tree",
			req: &persistence.InternalAppendHistoryNodesRequest{
				IsNewBranch: true,
				Info:        "test",
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				NodeID:        11,
				Events:        &persistence.DataBlob{},
				TransactionID: 100,
				ShardID:       1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().HistoryTreeInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, nil)
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().InsertIntoHistoryNode(gomock.Any(), gomock.Any()).Return(&sqlResult{rowsAffected: 1}, nil)
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoHistoryTree(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - old branch, duplicate entry",
			req: &persistence.InternalAppendHistoryNodesRequest{
				IsNewBranch: false,
				Info:        "test",
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				NodeID:        11,
				Events:        &persistence.DataBlob{},
				TransactionID: 100,
				ShardID:       1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				err := errors.New("some error")
				mockDB.EXPECT().InsertIntoHistoryNode(gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsDupEntryError(err).Return(true)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *persistence.ConditionFailedError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be ConditionFailedError")
			},
		},
		{
			name: "Error case - invalid history",
			req: &persistence.InternalAppendHistoryNodesRequest{
				IsNewBranch: false,
				Info:        "test",
				BranchInfo: types.HistoryBranch{
					TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
					BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
					Ancestors: []*types.HistoryBranchRange{
						{
							BranchID:    "730ec3d3-f74b-423f-a138-3b35494fe691",
							BeginNodeID: 1,
							EndNodeID:   10,
						},
					},
				},
				NodeID:        5,
				Events:        &persistence.DataBlob{},
				TransactionID: 100,
				ShardID:       1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {},
			wantErr:   true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *persistence.InvalidPersistenceRequestError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be InvalidPersistenceRequestError")
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			mockTx := sqlplugin.NewMockTx(ctrl)
			mockParser := serialization.NewMockParser(ctrl)
			store, err := NewHistoryV2Persistence(mockDB, nil, mockParser, nil)
			require.NoError(t, err, "Failed to create sql history store")

			tc.mockSetup(mockDB, mockTx, mockParser)
			err = store.AppendHistoryNodes(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestReadHistoryBranch(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.InternalReadHistoryBranchRequest
		mockSetup func(*sqlplugin.MockDB)
		want      *persistence.InternalReadHistoryBranchResponse
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			req: &persistence.InternalReadHistoryBranchRequest{
				TreeID:            "530ec3d3-f74b-423f-a138-3b35494fe691",
				BranchID:          "630ec3d3-f74b-423f-a138-3b35494fe691",
				MinNodeID:         100,
				MaxNodeID:         1000,
				PageSize:          2,
				NextPageToken:     serializePageToken(200),
				LastNodeID:        120,
				LastTransactionID: 100,
				ShardID:           1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().SelectFromHistoryNode(gomock.Any(), &sqlplugin.HistoryNodeFilter{
					TreeID:    serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
					BranchID:  serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
					MinNodeID: common.Int64Ptr(201),
					MaxNodeID: common.Int64Ptr(1000),
					PageSize:  2,
					ShardID:   1,
				}).Return([]sqlplugin.HistoryNodeRow{
					{
						NodeID:       201,
						TxnID:        common.Int64Ptr(99),
						Data:         []byte(`a`),
						DataEncoding: "a",
					},
					{
						NodeID:       202,
						TxnID:        common.Int64Ptr(101),
						Data:         []byte(`b`),
						DataEncoding: "b",
					},
				}, nil)
			},
			want: &persistence.InternalReadHistoryBranchResponse{
				History:           []*persistence.DataBlob{{Data: []byte(`b`), Encoding: constants.EncodingType("b")}},
				NextPageToken:     serializePageToken(202),
				LastNodeID:        202,
				LastTransactionID: 101,
			},
			wantErr: false,
		},
		{
			name: "Success case - no row",
			req: &persistence.InternalReadHistoryBranchRequest{
				TreeID:            "530ec3d3-f74b-423f-a138-3b35494fe691",
				BranchID:          "630ec3d3-f74b-423f-a138-3b35494fe691",
				MinNodeID:         100,
				MaxNodeID:         1000,
				PageSize:          2,
				NextPageToken:     serializePageToken(200),
				LastNodeID:        120,
				LastTransactionID: 100,
				ShardID:           1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().SelectFromHistoryNode(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
			},
			want:    &persistence.InternalReadHistoryBranchResponse{},
			wantErr: false,
		},
		{
			name: "Error case - corrupted data 1",
			req: &persistence.InternalReadHistoryBranchRequest{
				TreeID:            "530ec3d3-f74b-423f-a138-3b35494fe691",
				BranchID:          "630ec3d3-f74b-423f-a138-3b35494fe691",
				MinNodeID:         100,
				MaxNodeID:         1000,
				PageSize:          2,
				NextPageToken:     serializePageToken(200),
				LastNodeID:        120,
				LastTransactionID: 100,
				ShardID:           1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().SelectFromHistoryNode(gomock.Any(), gomock.Any()).Return([]sqlplugin.HistoryNodeRow{
					{
						NodeID:       119,
						TxnID:        common.Int64Ptr(99),
						Data:         []byte(`a`),
						DataEncoding: "a",
					},
					{
						NodeID:       202,
						TxnID:        common.Int64Ptr(101),
						Data:         []byte(`b`),
						DataEncoding: "b",
					},
				}, nil)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *types.InternalDataInconsistencyError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be InternalDataInconsistencyError")
				assert.Contains(t, err.Error(), "corrupted data, nodeID cannot decrease")
			},
		},
		{
			name: "Error case - corrupted data 2",
			req: &persistence.InternalReadHistoryBranchRequest{
				TreeID:            "530ec3d3-f74b-423f-a138-3b35494fe691",
				BranchID:          "630ec3d3-f74b-423f-a138-3b35494fe691",
				MinNodeID:         100,
				MaxNodeID:         1000,
				PageSize:          2,
				NextPageToken:     serializePageToken(200),
				LastNodeID:        120,
				LastTransactionID: 100,
				ShardID:           1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().SelectFromHistoryNode(gomock.Any(), gomock.Any()).Return([]sqlplugin.HistoryNodeRow{
					{
						NodeID:       119,
						TxnID:        common.Int64Ptr(101),
						Data:         []byte(`a`),
						DataEncoding: "a",
					},
					{
						NodeID:       202,
						TxnID:        common.Int64Ptr(101),
						Data:         []byte(`b`),
						DataEncoding: "b",
					},
				}, nil)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *types.InternalDataInconsistencyError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be InternalDataInconsistencyError")
				assert.Contains(t, err.Error(), "corrupted data, nodeID cannot decrease")
			},
		},
		{
			name: "Error case - corrupted data 3",
			req: &persistence.InternalReadHistoryBranchRequest{
				TreeID:            "530ec3d3-f74b-423f-a138-3b35494fe691",
				BranchID:          "630ec3d3-f74b-423f-a138-3b35494fe691",
				MinNodeID:         100,
				MaxNodeID:         1000,
				PageSize:          2,
				NextPageToken:     serializePageToken(200),
				LastNodeID:        120,
				LastTransactionID: 100,
				ShardID:           1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().SelectFromHistoryNode(gomock.Any(), gomock.Any()).Return([]sqlplugin.HistoryNodeRow{
					{
						NodeID:       120,
						TxnID:        common.Int64Ptr(101),
						Data:         []byte(`a`),
						DataEncoding: "a",
					},
				}, nil)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *types.InternalDataInconsistencyError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be InternalDataInconsistencyError")
				assert.Contains(t, err.Error(), "corrupted data, same nodeID must have smaller txnID")
			},
		},
		{
			name: "Error case - database error",
			req: &persistence.InternalReadHistoryBranchRequest{
				TreeID:            "530ec3d3-f74b-423f-a138-3b35494fe691",
				BranchID:          "630ec3d3-f74b-423f-a138-3b35494fe691",
				MinNodeID:         100,
				MaxNodeID:         1000,
				PageSize:          2,
				NextPageToken:     serializePageToken(200),
				LastNodeID:        120,
				LastTransactionID: 100,
				ShardID:           1,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromHistoryNode(gomock.Any(), gomock.Any()).Return(nil, err)
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
			store, err := NewHistoryV2Persistence(mockDB, nil, nil, nil)
			require.NoError(t, err, "Failed to create sql history store")

			tc.mockSetup(mockDB)
			got, err := store.ReadHistoryBranch(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestDeleteHistoryBranch_CustomBatchSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := sqlplugin.NewMockDB(ctrl)
	mockTx := sqlplugin.NewMockTx(ctrl)
	mockParser := serialization.NewMockParser(ctrl)

	customBatchSize := 500
	dc := &persistence.DynamicConfiguration{
		HistoryNodeDeleteBatchSize: func(...dynamicproperties.FilterOption) int { return customBatchSize },
	}

	store, err := NewHistoryV2Persistence(mockDB, nil, mockParser, dc)
	require.NoError(t, err)

	req := &persistence.InternalDeleteHistoryBranchRequest{
		BranchInfo: types.HistoryBranch{
			TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
			BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
		},
		ShardID: 1,
	}

	mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
		TreeID:  serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
		ShardID: 1,
	}).Return([]sqlplugin.HistoryTreeRow{
		{
			ShardID:      1,
			TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
			BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
			Data:         []byte(`aaaa`),
			DataEncoding: "json",
		},
	}, nil)
	mockParser.EXPECT().HistoryTreeInfoFromBlob([]byte(`aaaa`), "json").Return(&serialization.HistoryTreeInfo{}, nil)

	mockDB.EXPECT().GetTotalNumDBShards().Return(1)
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().DeleteFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
		TreeID:   serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
		BranchID: serialization.UUIDPtr(serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691")),
		ShardID:  1,
	}).Return(nil, nil)
	mockTx.EXPECT().DeleteFromHistoryNode(gomock.Any(), &sqlplugin.HistoryNodeFilter{
		TreeID:    serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
		BranchID:  serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
		ShardID:   1,
		PageSize:  customBatchSize,
		MinNodeID: common.Int64Ptr(1),
	}).Return(&sqlResult{rowsAffected: 1}, nil)
	mockTx.EXPECT().Commit().Return(nil)

	err = store.DeleteHistoryBranch(context.Background(), req)
	assert.NoError(t, err)
}

func TestDeleteHistoryBranch_UnboundedBatchSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := sqlplugin.NewMockDB(ctrl)
	mockTx := sqlplugin.NewMockTx(ctrl)
	mockParser := serialization.NewMockParser(ctrl)

	dc := &persistence.DynamicConfiguration{
		HistoryNodeDeleteBatchSize: func(...dynamicproperties.FilterOption) int { return 0 },
	}

	store, err := NewHistoryV2Persistence(mockDB, nil, mockParser, dc)
	require.NoError(t, err)

	req := &persistence.InternalDeleteHistoryBranchRequest{
		BranchInfo: types.HistoryBranch{
			TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
			BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
		},
		ShardID: 1,
	}

	mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
		TreeID:  serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
		ShardID: 1,
	}).Return([]sqlplugin.HistoryTreeRow{
		{
			ShardID:      1,
			TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
			BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
			Data:         []byte(`aaaa`),
			DataEncoding: "json",
		},
	}, nil)
	mockParser.EXPECT().HistoryTreeInfoFromBlob([]byte(`aaaa`), "json").Return(&serialization.HistoryTreeInfo{}, nil)

	mockDB.EXPECT().GetTotalNumDBShards().Return(1)
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().DeleteFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
		TreeID:   serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
		BranchID: serialization.UUIDPtr(serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691")),
		ShardID:  1,
	}).Return(nil, nil)
	mockTx.EXPECT().DeleteFromHistoryNode(gomock.Any(), &sqlplugin.HistoryNodeFilter{
		TreeID:    serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
		BranchID:  serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
		ShardID:   1,
		PageSize:  0,
		MinNodeID: common.Int64Ptr(1),
	}).Return(&sqlResult{rowsAffected: 100}, nil)
	mockTx.EXPECT().Commit().Return(nil)

	err = store.DeleteHistoryBranch(context.Background(), req)
	assert.NoError(t, err)
}

func TestDeleteHistoryBranch_NegativeBatchSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := sqlplugin.NewMockDB(ctrl)
	mockTx := sqlplugin.NewMockTx(ctrl)
	mockParser := serialization.NewMockParser(ctrl)

	dc := &persistence.DynamicConfiguration{
		HistoryNodeDeleteBatchSize: func(...dynamicproperties.FilterOption) int { return -100 },
	}

	store, err := NewHistoryV2Persistence(mockDB, nil, mockParser, dc)
	require.NoError(t, err)

	req := &persistence.InternalDeleteHistoryBranchRequest{
		BranchInfo: types.HistoryBranch{
			TreeID:   "530ec3d3-f74b-423f-a138-3b35494fe691",
			BranchID: "630ec3d3-f74b-423f-a138-3b35494fe691",
		},
		ShardID: 1,
	}

	mockDB.EXPECT().SelectFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
		TreeID:  serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
		ShardID: 1,
	}).Return([]sqlplugin.HistoryTreeRow{
		{
			ShardID:      1,
			TreeID:       serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
			BranchID:     serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
			Data:         []byte(`aaaa`),
			DataEncoding: "json",
		},
	}, nil)
	mockParser.EXPECT().HistoryTreeInfoFromBlob([]byte(`aaaa`), "json").Return(&serialization.HistoryTreeInfo{}, nil)

	mockDB.EXPECT().GetTotalNumDBShards().Return(1)
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().DeleteFromHistoryTree(gomock.Any(), &sqlplugin.HistoryTreeFilter{
		TreeID:   serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
		BranchID: serialization.UUIDPtr(serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691")),
		ShardID:  1,
	}).Return(nil, nil)
	mockTx.EXPECT().DeleteFromHistoryNode(gomock.Any(), &sqlplugin.HistoryNodeFilter{
		TreeID:    serialization.MustParseUUID("530ec3d3-f74b-423f-a138-3b35494fe691"),
		BranchID:  serialization.MustParseUUID("630ec3d3-f74b-423f-a138-3b35494fe691"),
		ShardID:   1,
		PageSize:  -100,
		MinNodeID: common.Int64Ptr(1),
	}).Return(&sqlResult{rowsAffected: 50}, nil)
	mockTx.EXPECT().Commit().Return(nil)

	err = store.DeleteHistoryBranch(context.Background(), req)
	assert.NoError(t, err)
}
