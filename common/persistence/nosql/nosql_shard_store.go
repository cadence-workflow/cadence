// Copyright (c) 2020 Uber Technologies, Inc.
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
	"fmt"
	"time"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/types"
)

// Implements ShardStore
type nosqlShardStore struct {
	shardedNosqlStore
	currentClusterName string
	parser             serialization.Parser
}

// newNoSQLShardStore is used to create an instance of ShardStore implementation
func newNoSQLShardStore(
	cfg config.ShardedNoSQL,
	clusterName string,
	logger log.Logger,
	metricsClient metrics.Client,
	dc *persistence.DynamicConfiguration,
	parser serialization.Parser,
) (persistence.ShardStore, error) {
	s, err := newShardedNosqlStore(cfg, logger, metricsClient, dc)
	if err != nil {
		return nil, err
	}
	return &nosqlShardStore{
		shardedNosqlStore:  s,
		currentClusterName: clusterName,
		parser:             parser,
	}, nil
}

func (sh *nosqlShardStore) CreateShard(
	ctx context.Context,
	request *persistence.InternalCreateShardRequest,
) error {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardInfo.ShardID)
	if err != nil {
		return err
	}
	shardRow, err := sh.shardInfoToShardsRow(request.ShardInfo)
	if err != nil {
		return err
	}
	err = storeShard.db.InsertShard(ctx, shardRow)
	if err != nil {
		conditionFailure, ok := err.(*nosqlplugin.ShardOperationConditionFailure)
		if ok {
			return &persistence.ShardAlreadyExistError{
				Msg: fmt.Sprintf("Shard already exists in executions table.  ShardId: %v, request_range_id: %v, actual_range_id : %v, columns: (%v)",
					request.ShardInfo.ShardID, request.ShardInfo.RangeID, conditionFailure.RangeID, conditionFailure.Details),
			}
		}
		return convertCommonErrors(storeShard.db, "CreateShard", err)
	}

	return nil
}

func (sh *nosqlShardStore) GetShard(
	ctx context.Context,
	request *persistence.InternalGetShardRequest,
) (*persistence.InternalGetShardResponse, error) {
	shardID := request.ShardID
	storeShard, err := sh.GetStoreShardByHistoryShard(shardID)
	if err != nil {
		return nil, err
	}
	rangeID, shardRow, err := storeShard.db.SelectShard(ctx, shardID, sh.currentClusterName)

	if err != nil {
		if storeShard.db.IsNotFoundError(err) {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf("Shard not found.  ShardId: %v", shardID),
			}
		}

		return nil, convertCommonErrors(storeShard.db, "GetShard", err)
	}

	shardInfoRangeID := shardRow.RangeID

	// check if rangeID column and rangeID field in shard column matches, if not we need to pick the larger
	// rangeID.
	if shardInfoRangeID > rangeID {
		// In this case we need to fix the rangeID column before returning the result as:
		// 1. if we return shardInfoRangeID, then later shard CAS operation will fail
		// 2. if we still return rangeID, CAS will work but rangeID will move backward which
		// result in lost tasks, corrupted workflow history, etc.

		sh.GetLogger().Warn("Corrupted shard rangeID", tag.ShardID(shardID), tag.ShardRangeID(shardInfoRangeID), tag.PreviousShardRangeID(rangeID))
		if err := sh.updateRangeID(ctx, shardID, shardInfoRangeID, rangeID); err != nil {
			return nil, err
		}
	} else {
		// return the actual rangeID
		shardRow.RangeID = rangeID
		//
		// If shardInfoRangeID = rangeID, no corruption, so no action needed.
		//
		// If shardInfoRangeID < rangeID, we also don't need to do anything here as createShardInfo will ignore
		// shardInfoRangeID and return rangeID instead. Later when updating the shard, CAS can still succeed
		// as the value from rangeID columns is returned, shardInfoRangeID will also be updated to the correct value.
	}
	shardInfo, err := sh.shardsRowToShardInfo(storeShard, shardRow, rangeID)
	if err != nil {
		return nil, err
	}

	return &persistence.InternalGetShardResponse{ShardInfo: shardInfo}, nil
}

func (sh *nosqlShardStore) updateRangeID(
	ctx context.Context,
	shardID int,
	rangeID int64,
	previousRangeID int64,
) error {
	storeShard, err := sh.GetStoreShardByHistoryShard(shardID)
	if err != nil {
		return err
	}
	err = storeShard.db.UpdateRangeID(ctx, shardID, rangeID, previousRangeID)
	if err != nil {
		conditionFailure, ok := err.(*nosqlplugin.ShardOperationConditionFailure)
		if ok {
			return &persistence.ShardOwnershipLostError{
				ShardID: shardID,
				Msg: fmt.Sprintf("Failed to update shard rangeID.  request_range_id: %v, actual_range_id : %v, columns: (%v)",
					previousRangeID, conditionFailure.RangeID, conditionFailure.Details),
			}
		}
		return convertCommonErrors(storeShard.db, "UpdateRangeID", err)
	}

	return nil
}

func (sh *nosqlShardStore) UpdateShard(
	ctx context.Context,
	request *persistence.InternalUpdateShardRequest,
) error {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardInfo.ShardID)
	if err != nil {
		return err
	}
	shardRow, err := sh.shardInfoToShardsRow(request.ShardInfo)
	if err != nil {
		return err
	}
	err = storeShard.db.UpdateShard(ctx, shardRow, request.PreviousRangeID)
	if err != nil {
		conditionFailure, ok := err.(*nosqlplugin.ShardOperationConditionFailure)
		if ok {
			return &persistence.ShardOwnershipLostError{
				ShardID: request.ShardInfo.ShardID,
				Msg: fmt.Sprintf("Failed to update shard rangeID.  request_range_id: %v, actual_range_id : %v, columns: (%v)",
					request.PreviousRangeID, conditionFailure.RangeID, conditionFailure.Details),
			}
		}
		return convertCommonErrors(storeShard.db, "UpdateShard", err)
	}

	return nil
}

func (sh *nosqlShardStore) shardInfoToShardsRow(s *persistence.InternalShardInfo) (*nosqlplugin.ShardRow, error) {
	var markerData []byte
	markerEncoding := string(constants.EncodingTypeEmpty)
	if s.PendingFailoverMarkers != nil {
		markerData = s.PendingFailoverMarkers.Data
		markerEncoding = string(s.PendingFailoverMarkers.Encoding)
	}

	var transferPQSData []byte
	transferPQSEncoding := string(constants.EncodingTypeEmpty)
	if s.TransferProcessingQueueStates != nil {
		transferPQSData = s.TransferProcessingQueueStates.Data
		transferPQSEncoding = string(s.TransferProcessingQueueStates.Encoding)
	}

	var timerPQSData []byte
	timerPQSEncoding := string(constants.EncodingTypeEmpty)
	if s.TimerProcessingQueueStates != nil {
		timerPQSData = s.TimerProcessingQueueStates.Data
		timerPQSEncoding = string(s.TimerProcessingQueueStates.Encoding)
	}

	shardInfo := &serialization.ShardInfo{
		StolenSinceRenew:                      int32(s.StolenSinceRenew),
		UpdatedAt:                             s.UpdatedAt,
		ReplicationAckLevel:                   s.ReplicationAckLevel,
		TransferAckLevel:                      s.TransferAckLevel,
		TimerAckLevel:                         s.TimerAckLevel,
		ClusterTransferAckLevel:               s.ClusterTransferAckLevel,
		ClusterTimerAckLevel:                  s.ClusterTimerAckLevel,
		TransferProcessingQueueStates:         transferPQSData,
		TransferProcessingQueueStatesEncoding: transferPQSEncoding,
		TimerProcessingQueueStates:            timerPQSData,
		TimerProcessingQueueStatesEncoding:    timerPQSEncoding,
		DomainNotificationVersion:             s.DomainNotificationVersion,
		Owner:                                 s.Owner,
		ClusterReplicationLevel:               s.ClusterReplicationLevel,
		ReplicationDlqAckLevel:                s.ReplicationDLQAckLevel,
		PendingFailoverMarkers:                markerData,
		PendingFailoverMarkersEncoding:        markerEncoding,
		QueueStates:                           s.QueueStates,
	}

	blob, err := sh.parser.ShardInfoToBlob(shardInfo)
	if err != nil {
		return nil, err
	}
	return &nosqlplugin.ShardRow{
		InternalShardInfo: s,
		Data:              blob.Data,
		DataEncoding:      string(blob.Encoding),
	}, nil
}

func (sh *nosqlShardStore) shardsRowToShardInfo(storeShard *nosqlStore, shardRow *nosqlplugin.ShardRow, rangeID int64) (*persistence.InternalShardInfo, error) {
	if !storeShard.dc.ReadNoSQLShardFromDataBlob() {
		sh.GetMetricsClient().IncCounter(metrics.PersistenceGetShardScope, metrics.NoSQLShardStoreReadFromOriginalColumnCounter)
		return shardRow.InternalShardInfo, nil
	}
	if len(shardRow.Data) == 0 {
		sh.GetMetricsClient().IncCounter(metrics.PersistenceGetShardScope, metrics.NoSQLShardStoreReadFromOriginalColumnCounter)
		sh.GetLogger().Warn("Shard info data blob is empty, falling back to typed fields")
		return shardRow.InternalShardInfo, nil
	}
	shardInfo, err := sh.parser.ShardInfoFromBlob(shardRow.Data, shardRow.DataEncoding)
	if err != nil {
		sh.GetMetricsClient().IncCounter(metrics.PersistenceGetShardScope, metrics.NoSQLShardStoreReadFromOriginalColumnCounter)
		sh.GetLogger().Error("Failed to decode shard info from data blob, falling back to typed fields", tag.Error(err))
		return shardRow.InternalShardInfo, nil
	}

	sh.GetMetricsClient().IncCounter(metrics.PersistenceGetShardScope, metrics.NoSQLShardStoreReadFromDataBlobCounter)

	if len(shardInfo.ClusterTransferAckLevel) == 0 {
		shardInfo.ClusterTransferAckLevel = map[string]int64{
			sh.currentClusterName: shardInfo.GetTransferAckLevel(),
		}
	}

	timerAckLevel := make(map[string]time.Time, len(shardInfo.ClusterTimerAckLevel))
	for k, v := range shardInfo.ClusterTimerAckLevel {
		timerAckLevel[k] = v
	}

	if len(timerAckLevel) == 0 {
		timerAckLevel = map[string]time.Time{
			sh.currentClusterName: shardInfo.GetTimerAckLevel(),
		}
	}

	if shardInfo.ClusterReplicationLevel == nil {
		shardInfo.ClusterReplicationLevel = make(map[string]int64)
	}
	if shardInfo.ReplicationDlqAckLevel == nil {
		shardInfo.ReplicationDlqAckLevel = make(map[string]int64)
	}

	var transferPQS *persistence.DataBlob
	if shardInfo.GetTransferProcessingQueueStates() != nil {
		transferPQS = &persistence.DataBlob{
			Encoding: constants.EncodingType(shardInfo.GetTransferProcessingQueueStatesEncoding()),
			Data:     shardInfo.GetTransferProcessingQueueStates(),
		}
	}

	var timerPQS *persistence.DataBlob
	if shardInfo.GetTimerProcessingQueueStates() != nil {
		timerPQS = &persistence.DataBlob{
			Encoding: constants.EncodingType(shardInfo.GetTimerProcessingQueueStatesEncoding()),
			Data:     shardInfo.GetTimerProcessingQueueStates(),
		}
	}

	var pendingFailoverMarkers *persistence.DataBlob
	if shardInfo.GetPendingFailoverMarkers() != nil {
		pendingFailoverMarkers = &persistence.DataBlob{
			Encoding: constants.EncodingType(shardInfo.GetPendingFailoverMarkersEncoding()),
			Data:     shardInfo.GetPendingFailoverMarkers(),
		}
	}
	return &persistence.InternalShardInfo{
		ShardID:                       int(shardRow.ShardID),
		RangeID:                       rangeID,
		Owner:                         shardInfo.GetOwner(),
		StolenSinceRenew:              int(shardInfo.GetStolenSinceRenew()),
		UpdatedAt:                     shardInfo.GetUpdatedAt(),
		ReplicationAckLevel:           shardInfo.GetReplicationAckLevel(),
		TransferAckLevel:              shardInfo.GetTransferAckLevel(),
		TimerAckLevel:                 shardInfo.GetTimerAckLevel(),
		ClusterTransferAckLevel:       shardInfo.ClusterTransferAckLevel,
		ClusterTimerAckLevel:          timerAckLevel,
		TransferProcessingQueueStates: transferPQS,
		TimerProcessingQueueStates:    timerPQS,
		DomainNotificationVersion:     shardInfo.GetDomainNotificationVersion(),
		ClusterReplicationLevel:       shardInfo.ClusterReplicationLevel,
		ReplicationDLQAckLevel:        shardInfo.ReplicationDlqAckLevel,
		PendingFailoverMarkers:        pendingFailoverMarkers,
		QueueStates:                   shardInfo.GetQueueStates(),
	}, nil
}
