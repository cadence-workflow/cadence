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
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

// Implements ShardStore
type nosqlShardStore struct {
	shardedNosqlStore
	currentClusterName string
}

// newNoSQLShardStore is used to create an instance of ShardStore implementation
func newNoSQLShardStore(
	cfg config.ShardedNoSQL,
	clusterName string,
	logger log.Logger,
	dc *persistence.DynamicConfiguration,
) (persistence.ShardStore, error) {
	s, err := newShardedNosqlStore(cfg, logger, dc)
	if err != nil {
		return nil, err
	}
	return &nosqlShardStore{
		shardedNosqlStore:  s,
		currentClusterName: clusterName,
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
	err = storeShard.db.InsertShard(ctx, request.ShardInfo, time.Now())
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
	rangeID, shardInfo, err := storeShard.db.SelectShard(ctx, shardID, sh.currentClusterName)

	if err != nil {
		if storeShard.db.IsNotFoundError(err) {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf("Shard not found.  ShardId: %v", shardID),
			}
		}

		return nil, convertCommonErrors(storeShard.db, "GetShard", err)
	}

	shardInfoRangeID := shardInfo.RangeID

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
		shardInfo.RangeID = rangeID
		//
		// If shardInfoRangeID = rangeID, no corruption, so no action needed.
		//
		// If shardInfoRangeID < rangeID, we also don't need to do anything here as createShardInfo will ignore
		// shardInfoRangeID and return rangeID instead. Later when updating the shard, CAS can still succeed
		// as the value from rangeID columns is returned, shardInfoRangeID will also be updated to the correct value.
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
	err = storeShard.db.UpdateShard(ctx, request.ShardInfo, request.PreviousRangeID, time.Now())
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
