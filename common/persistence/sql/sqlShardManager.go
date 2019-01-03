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
	"database/sql"
	"fmt"
	"time"

	"github.com/uber-common/bark"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/storage"
	"github.com/uber/cadence/common/persistence/sql/storage/sqldb"
	"github.com/uber/cadence/common/service/config"
)

type sqlShardManager struct {
	sqlStore
	currentClusterName string
}

// newShardPersistence creates an instance of ShardManager
func newShardPersistence(cfg config.SQL, currentClusterName string, log bark.Logger) (persistence.ShardManager, error) {
	var db, err = storage.NewSQLDB(&cfg)
	if err != nil {
		return nil, err
	}
	return &sqlShardManager{
		sqlStore: sqlStore{
			db:     db,
			logger: log,
		},
		currentClusterName: currentClusterName,
	}, nil
}

func (m *sqlShardManager) CreateShard(request *persistence.CreateShardRequest) error {
	if _, err := m.GetShard(&persistence.GetShardRequest{
		ShardID: request.ShardInfo.ShardID,
	}); err == nil {
		return &persistence.ShardAlreadyExistError{
			Msg: fmt.Sprintf("CreateShard operaiton failed. Shard with ID %v already exists.", request.ShardInfo.ShardID),
		}
	}

	row, err := shardInfoToShardsRow(*request.ShardInfo)
	if err != nil {
		return &workflow.InternalServiceError{
			Message: fmt.Sprintf("CreateShard operation failed. Error: %v", err),
		}
	}

	if _, err := m.db.InsertIntoShards(row); err != nil {
		return &workflow.InternalServiceError{
			Message: fmt.Sprintf("CreateShard operation failed. Failed to insert into shards table. Error: %v", err),
		}
	}

	return nil
}

func (m *sqlShardManager) GetShard(request *persistence.GetShardRequest) (*persistence.GetShardResponse, error) {
	row, err := m.db.SelectFromShards(&sqldb.QueryFilter{ShardID: request.ShardID})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &workflow.EntityNotExistsError{
				Message: fmt.Sprintf("GetShard operation failed. Shard with ID %v not found. Error: %v", request.ShardID, err),
			}
		}
		return nil, &workflow.InternalServiceError{
			Message: fmt.Sprintf("GetShard operation failed. Failed to get record. ShardId: %v. Error: %v", request.ShardID, err),
		}
	}

	clusterTransferAckLevel := make(map[string]int64)
	if err := gobDeserialize(row.ClusterTransferAckLevel, &clusterTransferAckLevel); err != nil {
		return nil, &workflow.InternalServiceError{
			Message: fmt.Sprintf("GetShard operation failed. Failed to deserialize ShardInfo.ClusterTransferAckLevel. ShardId: %v. Error: %v", request.ShardID, err),
		}
	}
	if len(clusterTransferAckLevel) == 0 {
		clusterTransferAckLevel = map[string]int64{
			m.currentClusterName: row.TransferAckLevel,
		}
	}

	clusterTimerAckLevel := make(map[string]time.Time)
	if err := gobDeserialize(row.ClusterTimerAckLevel, &clusterTimerAckLevel); err != nil {
		return nil, &workflow.InternalServiceError{
			Message: fmt.Sprintf("GetShard operation failed. Failed to deserialize ShardInfo.ClusterTimerAckLevel. ShardId: %v. Error: %v", request.ShardID, err),
		}
	}
	if len(clusterTimerAckLevel) == 0 {
		clusterTimerAckLevel = map[string]time.Time{
			m.currentClusterName: row.TimerAckLevel,
		}
	}

	resp := &persistence.GetShardResponse{ShardInfo: &persistence.ShardInfo{
		ShardID:                   int(row.ShardID),
		Owner:                     row.Owner,
		RangeID:                   row.RangeID,
		StolenSinceRenew:          int(row.StolenSinceRenew),
		UpdatedAt:                 row.UpdatedAt,
		ReplicationAckLevel:       row.ReplicationAckLevel,
		TransferAckLevel:          row.TransferAckLevel,
		TimerAckLevel:             row.TimerAckLevel,
		ClusterTransferAckLevel:   clusterTransferAckLevel,
		ClusterTimerAckLevel:      clusterTimerAckLevel,
		DomainNotificationVersion: row.DomainNotificationVersion,
	}}

	return resp, nil
}

func (m *sqlShardManager) UpdateShard(request *persistence.UpdateShardRequest) error {
	row, err := shardInfoToShardsRow(*request.ShardInfo)
	if err != nil {
		return &workflow.InternalServiceError{
			Message: fmt.Sprintf("UpdateShard operation failed. Error: %v", err),
		}
	}
	return m.txExecute("UpdateShard", func(tx sqldb.Tx) error {
		if err := lockShard(tx, request.ShardInfo.ShardID, request.PreviousRangeID); err != nil {
			return err
		}
		result, err := tx.UpdateShards(row)
		if err != nil {
			return err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("rowsAffected returned error for shardID %v: %v", request.ShardInfo.ShardID, err)
		}
		if rowsAffected != 1 {
			return fmt.Errorf("rowsAffected returned %v shards instead of one", rowsAffected)
		}
		return nil
	})
}

// initiated by the owning shard
func lockShard(tx sqldb.Tx, shardID int, oldRangeID int64) error {
	rangeID, err := tx.WriteLockShards(&sqldb.QueryFilter{ShardID: shardID})
	if err != nil {
		if err == sql.ErrNoRows {
			return &workflow.InternalServiceError{
				Message: fmt.Sprintf("Failed to lock shard with ID %v that does not exist.", shardID),
			}
		}
		return &workflow.InternalServiceError{
			Message: fmt.Sprintf("Failed to lock shard with ID: %v. Error: %v", shardID, err),
		}
	}

	if int64(rangeID) != oldRangeID {
		return &persistence.ShardOwnershipLostError{
			ShardID: shardID,
			Msg:     fmt.Sprintf("Failed to update shard. Previous range ID: %v; new range ID: %v", oldRangeID, rangeID),
		}
	}

	return nil
}

// initiated by the owning shard
func readLockShard(tx sqldb.Tx, shardID int, oldRangeID int64) error {
	rangeID, err := tx.ReadLockShards(&sqldb.QueryFilter{ShardID: shardID})
	if err != nil {
		if err == sql.ErrNoRows {
			return &workflow.InternalServiceError{
				Message: fmt.Sprintf("Failed to lock shard with ID %v that does not exist.", shardID),
			}
		}
		return &workflow.InternalServiceError{
			Message: fmt.Sprintf("Failed to lock shard with ID: %v. Error: %v", shardID, err),
		}
	}

	if int64(rangeID) != oldRangeID {
		return &persistence.ShardOwnershipLostError{
			ShardID: shardID,
			Msg:     fmt.Sprintf("Failed to lock shard. Previous range ID: %v; new range ID: %v", oldRangeID, rangeID),
		}
	}
	return nil
}

func shardInfoToShardsRow(s persistence.ShardInfo) (*sqldb.ShardsRow, error) {
	clusterTransferAckLevel, err := gobSerialize(s.ClusterTransferAckLevel)
	if err != nil {
		return nil, &workflow.InternalServiceError{
			Message: fmt.Sprintf("CreateShard operation failed. Failed to serialize ShardInfo.ClusterTransferAckLevel. Error: %v", err),
		}
	}

	clusterTimerAckLevel, err := gobSerialize(s.ClusterTimerAckLevel)
	if err != nil {
		return nil, &workflow.InternalServiceError{
			Message: fmt.Sprintf("CreateShard operation failed. Failed to serialize ShardInfo.ClusterTimerAckLevel. Error: %v", err),
		}
	}

	return &sqldb.ShardsRow{
		ShardID:                   int64(s.ShardID),
		Owner:                     s.Owner,
		RangeID:                   s.RangeID,
		StolenSinceRenew:          int64(s.StolenSinceRenew),
		UpdatedAt:                 s.UpdatedAt,
		ReplicationAckLevel:       s.ReplicationAckLevel,
		TransferAckLevel:          s.TransferAckLevel,
		TimerAckLevel:             s.TimerAckLevel,
		ClusterTransferAckLevel:   clusterTransferAckLevel,
		ClusterTimerAckLevel:      clusterTimerAckLevel,
		DomainNotificationVersion: s.DomainNotificationVersion,
	}, nil
}
