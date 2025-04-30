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

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

func TestGetShard(t *testing.T) {
	testCases := []struct {
		name        string
		clusterName string
		req         *persistence.InternalGetShardRequest
		mockSetup   func(*sqlplugin.MockDB, *serialization.MockParser)
		want        *persistence.InternalGetShardResponse
		wantErr     bool
	}{
		{
			name:        "Success case",
			clusterName: "active",
			req: &persistence.InternalGetShardRequest{
				ShardID: 2,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 2}).Return(&sqlplugin.ShardsRow{
					ShardID:      2,
					RangeID:      4,
					Data:         []byte(`aaaa`),
					DataEncoding: "json",
				}, nil)
				mockParser.EXPECT().ShardInfoFromBlob([]byte(`aaaa`), "json").Return(&serialization.ShardInfo{
					Owner:                                 "owner",
					StolenSinceRenew:                      1,
					UpdatedAt:                             time.Unix(100, 10),
					ReplicationAckLevel:                   1001,
					TransferAckLevel:                      1002,
					TimerAckLevel:                         time.Unix(2, 1),
					TransferProcessingQueueStates:         []byte(`transfer`),
					TransferProcessingQueueStatesEncoding: "transfer",
					TimerProcessingQueueStates:            []byte(`timer`),
					TimerProcessingQueueStatesEncoding:    "timer",
					DomainNotificationVersion:             99,
					PendingFailoverMarkers:                []byte(`markers`),
					PendingFailoverMarkersEncoding:        "markers",
					ReplicationDlqAckLevel:                map[string]int64{"active": 10},
					ClusterReplicationLevel:               map[string]int64{"active": 1002},
					ClusterTimerAckLevel:                  map[string]time.Time{"active": time.Unix(2, 1)},
					ClusterTransferAckLevel:               map[string]int64{"active": 1002},
					QueueStates: map[int32]*types.QueueState{
						0: &types.QueueState{
							VirtualQueueStates: map[int64]*types.VirtualQueueState{
								0: {},
							},
						},
					},
				}, nil)
			},
			want: &persistence.InternalGetShardResponse{
				ShardInfo: &persistence.InternalShardInfo{
					ShardID:                 2,
					RangeID:                 4,
					Owner:                   "owner",
					StolenSinceRenew:        1,
					UpdatedAt:               time.Unix(100, 10),
					ReplicationAckLevel:     1001,
					TransferAckLevel:        1002,
					TimerAckLevel:           time.Unix(2, 1),
					ClusterTransferAckLevel: map[string]int64{"active": 1002},
					ClusterTimerAckLevel:    map[string]time.Time{"active": time.Unix(2, 1)},
					TransferProcessingQueueStates: &persistence.DataBlob{
						Encoding: constants.EncodingType("transfer"),
						Data:     []byte(`transfer`),
					},
					TimerProcessingQueueStates: &persistence.DataBlob{
						Encoding: constants.EncodingType("timer"),
						Data:     []byte(`timer`),
					},
					DomainNotificationVersion: 99,
					ClusterReplicationLevel:   map[string]int64{"active": 1002},
					ReplicationDLQAckLevel:    map[string]int64{"active": 10},
					PendingFailoverMarkers: &persistence.DataBlob{
						Encoding: constants.EncodingType("markers"),
						Data:     []byte(`markers`),
					},
					QueueStates: map[int32]*types.QueueState{
						0: &types.QueueState{
							VirtualQueueStates: map[int64]*types.VirtualQueueState{
								0: {},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:        "Error case - failed to get shard",
			clusterName: "active",
			req: &persistence.InternalGetShardRequest{
				ShardID: 2,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromShards(gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:        "Error case - failed to decode data",
			clusterName: "active",
			req: &persistence.InternalGetShardRequest{
				ShardID: 2,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 2}).Return(&sqlplugin.ShardsRow{
					ShardID:      2,
					RangeID:      4,
					Data:         []byte(`aaaa`),
					DataEncoding: "json",
				}, nil)
				err := errors.New("some error")
				mockParser.EXPECT().ShardInfoFromBlob(gomock.Any(), gomock.Any()).Return(nil, err)
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
			store, err := NewShardPersistence(mockDB, tc.clusterName, nil, mockParser)
			require.NoError(t, err, "Failed to create sql shard store")

			tc.mockSetup(mockDB, mockParser)
			got, err := store.GetShard(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestCreateShard(t *testing.T) {
	testCases := []struct {
		name        string
		clusterName string
		req         *persistence.InternalCreateShardRequest
		mockSetup   func(*sqlplugin.MockDB, *serialization.MockParser)
		wantErr     bool
		assertErr   func(*testing.T, error)
	}{
		{
			name:        "Success case",
			clusterName: "active",
			req: &persistence.InternalCreateShardRequest{
				ShardInfo: &persistence.InternalShardInfo{
					ShardID:                 2,
					Owner:                   "owner",
					RangeID:                 4,
					StolenSinceRenew:        1,
					UpdatedAt:               time.Unix(1, 2),
					ReplicationAckLevel:     9,
					TransferAckLevel:        99,
					TimerAckLevel:           time.Unix(9, 9),
					ClusterTransferAckLevel: map[string]int64{"a": 8},
					ClusterTimerAckLevel:    map[string]time.Time{"b": time.Unix(8, 8)},
					TransferProcessingQueueStates: &persistence.DataBlob{
						Data:     []byte(`transfer`),
						Encoding: constants.EncodingType("transfer"),
					},
					TimerProcessingQueueStates: &persistence.DataBlob{
						Data:     []byte(`timer`),
						Encoding: constants.EncodingType("timer"),
					},
					DomainNotificationVersion: 101,
					ClusterReplicationLevel:   map[string]int64{"z": 199},
					ReplicationDLQAckLevel:    map[string]int64{"y": 1111},
					PendingFailoverMarkers: &persistence.DataBlob{
						Data:     []byte(`markers`),
						Encoding: constants.EncodingType("markers"),
					},
					QueueStates: map[int32]*types.QueueState{
						0: &types.QueueState{
							VirtualQueueStates: map[int64]*types.VirtualQueueState{
								0: {},
							},
						},
					},
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromShards(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
				mockParser.EXPECT().ShardInfoToBlob(&serialization.ShardInfo{
					StolenSinceRenew:                      1,
					UpdatedAt:                             time.Unix(1, 2),
					ReplicationAckLevel:                   9,
					TransferAckLevel:                      99,
					TimerAckLevel:                         time.Unix(9, 9),
					ClusterTransferAckLevel:               map[string]int64{"a": 8},
					ClusterTimerAckLevel:                  map[string]time.Time{"b": time.Unix(8, 8)},
					TransferProcessingQueueStates:         []byte(`transfer`),
					TransferProcessingQueueStatesEncoding: "transfer",
					TimerProcessingQueueStates:            []byte(`timer`),
					TimerProcessingQueueStatesEncoding:    "timer",
					DomainNotificationVersion:             101,
					Owner:                                 "owner",
					ClusterReplicationLevel:               map[string]int64{"z": 199},
					ReplicationDlqAckLevel:                map[string]int64{"y": 1111},
					PendingFailoverMarkers:                []byte(`markers`),
					PendingFailoverMarkersEncoding:        "markers",
					QueueStates: map[int32]*types.QueueState{
						0: &types.QueueState{
							VirtualQueueStates: map[int64]*types.VirtualQueueState{
								0: {},
							},
						},
					},
				}).Return(persistence.DataBlob{
					Encoding: constants.EncodingType("shard"),
					Data:     []byte(`shard`),
				}, nil)
				mockDB.EXPECT().InsertIntoShards(gomock.Any(), &sqlplugin.ShardsRow{
					ShardID:      2,
					RangeID:      4,
					Data:         []byte(`shard`),
					DataEncoding: "shard",
				}).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name:        "Error case - already exists",
			clusterName: "active",
			req: &persistence.InternalCreateShardRequest{
				ShardInfo: &persistence.InternalShardInfo{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromShards(gomock.Any(), gomock.Any()).Return(&sqlplugin.ShardsRow{}, nil)
				mockParser.EXPECT().ShardInfoFromBlob(gomock.Any(), gomock.Any()).Return(&serialization.ShardInfo{}, nil)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *persistence.ShardAlreadyExistError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be ShardAlreadyExistError")
			},
		},
		{
			name:        "Error case - failed to encode",
			clusterName: "active",
			req: &persistence.InternalCreateShardRequest{
				ShardInfo: &persistence.InternalShardInfo{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromShards(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
				mockParser.EXPECT().ShardInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name:        "Error case - failed to insert",
			clusterName: "active",
			req: &persistence.InternalCreateShardRequest{
				ShardInfo: &persistence.InternalShardInfo{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockDB.EXPECT().SelectFromShards(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
				mockParser.EXPECT().ShardInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Encoding: constants.EncodingType("shard"),
					Data:     []byte(`shard`),
				}, nil)
				err := errors.New("some error")
				mockDB.EXPECT().InsertIntoShards(gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
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
			store, err := NewShardPersistence(mockDB, tc.clusterName, nil, mockParser)
			require.NoError(t, err, "Failed to create sql shard store")

			tc.mockSetup(mockDB, mockParser)
			err = store.CreateShard(context.Background(), tc.req)
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

func TestUpdateShard(t *testing.T) {
	testCases := []struct {
		name        string
		clusterName string
		req         *persistence.InternalUpdateShardRequest
		mockSetup   func(*sqlplugin.MockDB, *sqlplugin.MockTx, *serialization.MockParser)
		wantErr     bool
	}{
		{
			name:        "Success case",
			clusterName: "active",
			req: &persistence.InternalUpdateShardRequest{
				PreviousRangeID: 1,
				ShardInfo: &persistence.InternalShardInfo{
					ShardID:                 2,
					Owner:                   "owner",
					RangeID:                 4,
					StolenSinceRenew:        1,
					UpdatedAt:               time.Unix(1, 2),
					ReplicationAckLevel:     9,
					TransferAckLevel:        99,
					TimerAckLevel:           time.Unix(9, 9),
					ClusterTransferAckLevel: map[string]int64{"a": 8},
					ClusterTimerAckLevel:    map[string]time.Time{"b": time.Unix(8, 8)},
					TransferProcessingQueueStates: &persistence.DataBlob{
						Data:     []byte(`transfer`),
						Encoding: constants.EncodingType("transfer"),
					},
					TimerProcessingQueueStates: &persistence.DataBlob{
						Data:     []byte(`timer`),
						Encoding: constants.EncodingType("timer"),
					},
					DomainNotificationVersion: 101,
					ClusterReplicationLevel:   map[string]int64{"z": 199},
					ReplicationDLQAckLevel:    map[string]int64{"y": 1111},
					PendingFailoverMarkers: &persistence.DataBlob{
						Data:     []byte(`markers`),
						Encoding: constants.EncodingType("markers"),
					},
					QueueStates: map[int32]*types.QueueState{
						0: &types.QueueState{
							VirtualQueueStates: map[int64]*types.VirtualQueueState{
								0: {},
							},
						},
					},
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().ShardInfoToBlob(&serialization.ShardInfo{
					StolenSinceRenew:                      1,
					UpdatedAt:                             time.Unix(1, 2),
					ReplicationAckLevel:                   9,
					TransferAckLevel:                      99,
					TimerAckLevel:                         time.Unix(9, 9),
					ClusterTransferAckLevel:               map[string]int64{"a": 8},
					ClusterTimerAckLevel:                  map[string]time.Time{"b": time.Unix(8, 8)},
					TransferProcessingQueueStates:         []byte(`transfer`),
					TransferProcessingQueueStatesEncoding: "transfer",
					TimerProcessingQueueStates:            []byte(`timer`),
					TimerProcessingQueueStatesEncoding:    "timer",
					DomainNotificationVersion:             101,
					Owner:                                 "owner",
					ClusterReplicationLevel:               map[string]int64{"z": 199},
					ReplicationDlqAckLevel:                map[string]int64{"y": 1111},
					PendingFailoverMarkers:                []byte(`markers`),
					PendingFailoverMarkersEncoding:        "markers",
					QueueStates: map[int32]*types.QueueState{
						0: &types.QueueState{
							VirtualQueueStates: map[int64]*types.VirtualQueueState{
								0: {},
							},
						},
					},
				}).Return(persistence.DataBlob{
					Encoding: constants.EncodingType("shard"),
					Data:     []byte(`shard`),
				}, nil)
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().WriteLockShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 2}).Return(1, nil)
				mockTx.EXPECT().UpdateShards(gomock.Any(), &sqlplugin.ShardsRow{
					ShardID:      2,
					RangeID:      4,
					Data:         []byte(`shard`),
					DataEncoding: "shard",
				}).Return(&sqlResult{rowsAffected: 1}, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "Error case - failed to encode",
			clusterName: "active",
			req: &persistence.InternalUpdateShardRequest{
				ShardInfo: &persistence.InternalShardInfo{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().ShardInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name:        "Error case - failed to lock",
			clusterName: "active",
			req: &persistence.InternalUpdateShardRequest{
				ShardInfo: &persistence.InternalShardInfo{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().ShardInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Encoding: constants.EncodingType("shard"),
					Data:     []byte(`shard`),
				}, nil)
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				err := errors.New("some error")
				mockTx.EXPECT().WriteLockShards(gomock.Any(), gomock.Any()).Return(0, err)
				mockTx.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			wantErr: true,
		},
		{
			name:        "Error case - failed to update",
			clusterName: "active",
			req: &persistence.InternalUpdateShardRequest{
				ShardInfo: &persistence.InternalShardInfo{},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx, mockParser *serialization.MockParser) {
				mockParser.EXPECT().ShardInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Encoding: constants.EncodingType("shard"),
					Data:     []byte(`shard`),
				}, nil)
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().WriteLockShards(gomock.Any(), gomock.Any()).Return(0, nil)
				err := errors.New("some error")
				mockTx.EXPECT().UpdateShards(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
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
			store, err := NewShardPersistence(mockDB, tc.clusterName, nil, mockParser)
			require.NoError(t, err, "Failed to create sql shard store")

			tc.mockSetup(mockDB, mockTx, mockParser)
			err = store.UpdateShard(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestLockShard(t *testing.T) {
	shardID := 1
	oldRangeID := int64(99)

	testCases := []struct {
		name      string
		mockSetup func(*sqlplugin.MockTx)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().WriteLockShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 1}).Return(99, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case - not found",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().WriteLockShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 1}).Return(0, sql.ErrNoRows)
			},
			wantErr: true,
		},
		{
			name: "Error case - database failure",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				err := errors.New("some error")
				mockTx.EXPECT().WriteLockShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 1}).Return(0, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - rangeID mismatch",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().WriteLockShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 1}).Return(98, nil)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *persistence.ShardOwnershipLostError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be ShardOwnershipLostError")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)

			tc.mockSetup(mockTx)
			err := lockShard(context.Background(), mockTx, shardID, oldRangeID)
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

func TestReadLockShard(t *testing.T) {
	shardID := 1
	oldRangeID := int64(99)

	testCases := []struct {
		name      string
		mockSetup func(*sqlplugin.MockTx)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().ReadLockShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 1}).Return(99, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case - not found",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().ReadLockShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 1}).Return(0, sql.ErrNoRows)
			},
			wantErr: true,
		},
		{
			name: "Error case - database failure",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				err := errors.New("some error")
				mockTx.EXPECT().ReadLockShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 1}).Return(0, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - rangeID mismatch",
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().ReadLockShards(gomock.Any(), &sqlplugin.ShardsFilter{ShardID: 1}).Return(98, nil)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *persistence.ShardOwnershipLostError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be ShardOwnershipLostError")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTx := sqlplugin.NewMockTx(ctrl)

			tc.mockSetup(mockTx)
			err := readLockShard(context.Background(), mockTx, shardID, oldRangeID)
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
