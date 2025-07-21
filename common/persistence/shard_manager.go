// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

package persistence

import (
	"context"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
)

type (
	shardManager struct {
		persistence ShardStore
		serializer  PayloadSerializer
		timeSrc     clock.TimeSource
	}
)

var _ ShardManager = (*shardManager)(nil)

type ShardManagerOption func(manager *shardManager)

func WithSerializer(serializer PayloadSerializer) ShardManagerOption {
	return func(manager *shardManager) {
		if serializer != nil {
			manager.serializer = serializer
		}
	}
}

// NewShardManager returns a new ShardManager
func NewShardManager(
	persistence ShardStore,
	options ...ShardManagerOption,
) ShardManager {
	manager := &shardManager{
		persistence: persistence,
		serializer:  NewPayloadSerializer(),
		timeSrc:     clock.NewRealTimeSource(),
	}
	for _, option := range options {
		option(manager)
	}
	return manager
}

func (m *shardManager) GetName() string {
	return m.persistence.GetName()
}

func (m *shardManager) Close() {
	m.persistence.Close()
}

func (m *shardManager) CreateShard(ctx context.Context, request *CreateShardRequest) error {
	shardInfo, err := m.toInternalShardInfo(request.ShardInfo)
	if err != nil {
		return err
	}
	internalRequest := &InternalCreateShardRequest{
		ShardInfo:        shardInfo,
		CurrentTimeStamp: m.timeSrc.Now(),
	}
	return m.persistence.CreateShard(ctx, internalRequest)
}

func (m *shardManager) GetShard(ctx context.Context, request *GetShardRequest) (*GetShardResponse, error) {
	internalRequest := &InternalGetShardRequest{
		ShardID: request.ShardID,
	}
	internalResult, err := m.persistence.GetShard(ctx, internalRequest)
	if err != nil {
		return nil, err
	}
	shardInfo, err := m.fromInternalShardInfo(internalResult.ShardInfo)
	if err != nil {
		return nil, err
	}
	result := &GetShardResponse{
		ShardInfo: shardInfo,
	}
	return result, nil
}

func (m *shardManager) UpdateShard(ctx context.Context, request *UpdateShardRequest) error {
	shardInfo, err := m.toInternalShardInfo(request.ShardInfo)
	if err != nil {
		return err
	}
	internalRequest := &InternalUpdateShardRequest{
		ShardInfo:        shardInfo,
		PreviousRangeID:  request.PreviousRangeID,
		CurrentTimeStamp: m.timeSrc.Now(),
	}
	return m.persistence.UpdateShard(ctx, internalRequest)
}

func (m *shardManager) toInternalShardInfo(shardInfo *ShardInfo) (*InternalShardInfo, error) {
	if shardInfo == nil {
		return nil, nil
	}
	serializedTransferProcessingQueueStates, err := m.serializer.SerializeProcessingQueueStates(shardInfo.TransferProcessingQueueStates, constants.EncodingTypeThriftRW)
	if err != nil {
		return nil, err
	}
	serializedTimerProcessingQueueStates, err := m.serializer.SerializeProcessingQueueStates(shardInfo.TimerProcessingQueueStates, constants.EncodingTypeThriftRW)
	if err != nil {
		return nil, err
	}
	pendingFailoverMarker, err := m.serializer.SerializePendingFailoverMarkers(shardInfo.PendingFailoverMarkers, constants.EncodingTypeThriftRW)
	if err != nil {
		return nil, err
	}

	return &InternalShardInfo{
		ShardID:                       shardInfo.ShardID,
		Owner:                         shardInfo.Owner,
		RangeID:                       shardInfo.RangeID,
		StolenSinceRenew:              shardInfo.StolenSinceRenew,
		UpdatedAt:                     shardInfo.UpdatedAt,
		ReplicationAckLevel:           shardInfo.ReplicationAckLevel,
		ReplicationDLQAckLevel:        shardInfo.ReplicationDLQAckLevel,
		TransferAckLevel:              shardInfo.TransferAckLevel,
		TimerAckLevel:                 shardInfo.TimerAckLevel,
		ClusterTransferAckLevel:       shardInfo.ClusterTransferAckLevel,
		ClusterTimerAckLevel:          shardInfo.ClusterTimerAckLevel,
		TransferProcessingQueueStates: serializedTransferProcessingQueueStates,
		TimerProcessingQueueStates:    serializedTimerProcessingQueueStates,
		ClusterReplicationLevel:       shardInfo.ClusterReplicationLevel,
		DomainNotificationVersion:     shardInfo.DomainNotificationVersion,
		PendingFailoverMarkers:        pendingFailoverMarker,
		QueueStates:                   shardInfo.QueueStates,
	}, nil
}

func (m *shardManager) fromInternalShardInfo(internalShardInfo *InternalShardInfo) (*ShardInfo, error) {
	if internalShardInfo == nil {
		return nil, nil
	}
	transferProcessingQueueStates, err := m.serializer.DeserializeProcessingQueueStates(internalShardInfo.TransferProcessingQueueStates)
	if err != nil {
		return nil, err
	}
	timerProcessingQueueStates, err := m.serializer.DeserializeProcessingQueueStates(internalShardInfo.TimerProcessingQueueStates)
	if err != nil {
		return nil, err
	}
	pendingFailoverMarker, err := m.serializer.DeserializePendingFailoverMarkers(internalShardInfo.PendingFailoverMarkers)
	if err != nil {
		return nil, err
	}

	return &ShardInfo{
		ShardID:                       internalShardInfo.ShardID,
		Owner:                         internalShardInfo.Owner,
		RangeID:                       internalShardInfo.RangeID,
		StolenSinceRenew:              internalShardInfo.StolenSinceRenew,
		UpdatedAt:                     internalShardInfo.UpdatedAt,
		ReplicationAckLevel:           internalShardInfo.ReplicationAckLevel,
		ReplicationDLQAckLevel:        internalShardInfo.ReplicationDLQAckLevel,
		TransferAckLevel:              internalShardInfo.TransferAckLevel,
		TimerAckLevel:                 internalShardInfo.TimerAckLevel,
		ClusterTransferAckLevel:       internalShardInfo.ClusterTransferAckLevel,
		ClusterTimerAckLevel:          internalShardInfo.ClusterTimerAckLevel,
		TransferProcessingQueueStates: transferProcessingQueueStates,
		TimerProcessingQueueStates:    timerProcessingQueueStates,
		ClusterReplicationLevel:       internalShardInfo.ClusterReplicationLevel,
		DomainNotificationVersion:     internalShardInfo.DomainNotificationVersion,
		PendingFailoverMarkers:        pendingFailoverMarker,
		QueueStates:                   internalShardInfo.QueueStates,
	}, nil
}
