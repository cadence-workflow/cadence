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

package persistence

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

func TestShardManagerGetName(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockShardStore(ctrl)
	store.EXPECT().GetName().Return("name")

	manager := NewShardManager(store, &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})

	assert.Equal(t, "name", manager.GetName())
}

func TestShardManagerClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockShardStore(ctrl)
	store.EXPECT().Close().Times(1)

	manager := NewShardManager(store, &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})
	manager.Close()
}

func TestShardManagerCreateShard(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request          *CreateShardRequest
		internalRequest  *InternalCreateShardRequest
		serializer       PayloadSerializer
		internalResponse error
		expected         error
	}{
		"Success": {
			request: &CreateShardRequest{
				ShardInfo: sampleShardInfo(),
			},
			internalRequest: &InternalCreateShardRequest{
				ShardInfo: sampleInternalShardInfo(t),
			},
			internalResponse: nil,
			expected:         nil,
		},
		"Serialization Failure": {
			request: &CreateShardRequest{
				ShardInfo: sampleShardInfo(),
			},
			serializer: failingSerializer(ctrl),
			expected:   fmt.Errorf("serialization"),
		},
		"Error Response": {
			request: &CreateShardRequest{
				ShardInfo: sampleShardInfo(),
			},
			internalRequest: &InternalCreateShardRequest{
				ShardInfo: sampleInternalShardInfo(t),
			},
			internalResponse: fmt.Errorf("error"),
			expected:         fmt.Errorf("error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			store := NewMockShardStore(ctrl)
			if test.internalRequest != nil {
				store.EXPECT().CreateShard(gomock.Any(), gomock.Any()).
					Do(func(ctx context.Context, req *InternalCreateShardRequest) {
						assert.Equal(t, test.internalRequest.ShardInfo, req.ShardInfo)

						assert.WithinDuration(t, time.Now(), req.CurrentTimeStamp, time.Second)
					}).Return(test.internalResponse)
			}

			manager := NewShardManager(store, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			}, WithSerializer(test.serializer))

			result := manager.CreateShard(context.Background(), test.request)

			assert.Equal(t, test.expected, result)
		})
	}
}

func TestShardManagerGetShard(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		request          *GetShardRequest
		internalRequest  *InternalGetShardRequest
		serializer       PayloadSerializer
		internalResponse *InternalGetShardResponse
		internalErr      error
		expected         *GetShardResponse
		expectedErr      error
	}{
		"Success": {
			request: &GetShardRequest{
				ShardID: shardID,
			},
			internalRequest: &InternalGetShardRequest{
				ShardID: shardID,
			},
			internalResponse: &InternalGetShardResponse{
				ShardInfo: sampleInternalShardInfo(t),
			},
			expected: &GetShardResponse{
				ShardInfo: sampleShardInfo(),
			},
		},
		"Nil response": {
			request: &GetShardRequest{
				ShardID: shardID,
			},
			internalRequest: &InternalGetShardRequest{
				ShardID: shardID,
			},
			internalResponse: &InternalGetShardResponse{
				ShardInfo: nil,
			},
			expected: &GetShardResponse{
				ShardInfo: nil,
			},
		},
		"Serialization Failure": {
			request: &GetShardRequest{
				ShardID: shardID,
			},
			internalRequest: &InternalGetShardRequest{
				ShardID: shardID,
			},
			internalResponse: &InternalGetShardResponse{
				ShardInfo: sampleInternalShardInfo(t),
			},
			serializer:  failingSerializer(ctrl),
			expectedErr: fmt.Errorf("serialization"),
		},
		"Error Response": {
			request: &GetShardRequest{
				ShardID: shardID,
			},
			internalRequest: &InternalGetShardRequest{
				ShardID: shardID,
			},
			internalErr: fmt.Errorf("error"),
			expectedErr: fmt.Errorf("error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			store := NewMockShardStore(ctrl)
			if test.internalRequest != nil {
				store.EXPECT().GetShard(gomock.Any(), gomock.Eq(test.internalRequest)).Return(test.internalResponse, test.internalErr)
			}

			manager := NewShardManager(store, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			}, WithSerializer(test.serializer))

			result, err := manager.GetShard(context.Background(), test.request)
			assert.Equal(t, test.expected, result)
			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func TestShardManagerUpdateShard(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request          *UpdateShardRequest
		internalRequest  *InternalUpdateShardRequest
		serializer       PayloadSerializer
		internalResponse error
		expected         error
	}{
		"Success": {
			request: &UpdateShardRequest{
				ShardInfo:       sampleShardInfo(),
				PreviousRangeID: 1,
			},
			internalRequest: &InternalUpdateShardRequest{
				ShardInfo:       sampleInternalShardInfo(t),
				PreviousRangeID: 1,
			},
			internalResponse: nil,
			expected:         nil,
		},
		"Serialization Failure": {
			request: &UpdateShardRequest{
				ShardInfo:       sampleShardInfo(),
				PreviousRangeID: 1,
			},
			serializer: failingSerializer(ctrl),
			expected:   fmt.Errorf("serialization"),
		},
		"Error Response": {
			request: &UpdateShardRequest{
				ShardInfo:       sampleShardInfo(),
				PreviousRangeID: 1,
			},
			internalRequest: &InternalUpdateShardRequest{
				ShardInfo:       sampleInternalShardInfo(t),
				PreviousRangeID: 1,
			},
			internalResponse: fmt.Errorf("error"),
			expected:         fmt.Errorf("error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			store := NewMockShardStore(ctrl)
			if test.internalRequest != nil {
				store.EXPECT().UpdateShard(gomock.Any(), gomock.Any()).
					Do(func(ctx context.Context, req *InternalUpdateShardRequest) {
						assert.Equal(t, test.internalRequest.PreviousRangeID, req.PreviousRangeID)
						assert.Equal(t, test.internalRequest.ShardInfo, req.ShardInfo)

						assert.WithinDuration(t, time.Now(), req.CurrentTimeStamp, time.Second)
					}).Return(test.internalResponse)
			}

			manager := NewShardManager(store, &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			}, WithSerializer(test.serializer))

			result := manager.UpdateShard(context.Background(), test.request)

			assert.Equal(t, test.expected, result)

		})
	}
}

var (
	shardID                      = 1
	owner                        = "TestOwner"
	rangeID                int64 = 2
	stolenSinceRenew             = 3
	updatedAt                    = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	replicationAckLevel    int64 = 4
	timerAckLevel                = time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC)
	transferAckLevel       int64 = 5
	replicationDlqAckLevel       = map[string]int64{
		"key": 123,
	}
	clusterTransferAckLevel = map[string]int64{
		"otherKey": 124,
	}
	clusterTimerAckLevel = map[string]time.Time{
		"foo": time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	transferProcessingQueueStates = &types.ProcessingQueueStates{
		StatesByCluster: map[string][]*types.ProcessingQueueState{
			"transfer": {
				{
					Level:    common.Int32Ptr(1),
					AckLevel: common.Int64Ptr(2),
					MaxLevel: common.Int64Ptr(3),
					DomainFilter: &types.DomainFilter{
						DomainIDs:    []string{"foo", "bar"},
						ReverseMatch: false,
					},
				},
			},
		},
	}
	timerProcesssingQueueStates = &types.ProcessingQueueStates{
		StatesByCluster: map[string][]*types.ProcessingQueueState{
			"timer": {
				{
					Level:    common.Int32Ptr(7),
					AckLevel: common.Int64Ptr(8),
					MaxLevel: common.Int64Ptr(9),
					DomainFilter: &types.DomainFilter{
						DomainIDs:    []string{"bar"},
						ReverseMatch: false,
					},
				},
			},
		},
	}
	clusterReplicationLevel = map[string]int64{
		"bar": 100,
	}
	domainNotificationVersion int64 = 6
	pendingFailoverMarkers          = []*types.FailoverMarkerAttributes{
		{
			DomainID:        "TestDomain",
			FailoverVersion: 11,
			CreationTime:    common.Int64Ptr(12),
		},
	}
)

func failingSerializer(ctrl *gomock.Controller) PayloadSerializer {
	serializer := NewMockPayloadSerializer(ctrl)
	serializer.EXPECT().SerializePendingFailoverMarkers(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("serialization")).AnyTimes()
	serializer.EXPECT().DeserializePendingFailoverMarkers(gomock.Any()).Return(nil, fmt.Errorf("serialization")).AnyTimes()
	serializer.EXPECT().SerializeProcessingQueueStates(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("serialization")).AnyTimes()
	serializer.EXPECT().DeserializeProcessingQueueStates(gomock.Any()).Return(nil, fmt.Errorf("serialization")).AnyTimes()

	return serializer
}

func sampleInternalShardInfo(t *testing.T) *InternalShardInfo {
	serializer := NewPayloadSerializer()
	transferProcessingQueueStatesBlob, err := serializer.SerializeProcessingQueueStates(transferProcessingQueueStates, constants.EncodingTypeThriftRW)
	assert.NoError(t, err)
	timerProcessingQueueStatesBlob, err := serializer.SerializeProcessingQueueStates(timerProcesssingQueueStates, constants.EncodingTypeThriftRW)
	assert.NoError(t, err)
	pendingFailoverMarkerBlob, err := serializer.SerializePendingFailoverMarkers(pendingFailoverMarkers, constants.EncodingTypeThriftRW)
	assert.NoError(t, err)
	return &InternalShardInfo{
		ShardID:                       shardID,
		Owner:                         owner,
		RangeID:                       rangeID,
		StolenSinceRenew:              3,
		UpdatedAt:                     updatedAt,
		ReplicationAckLevel:           4,
		ReplicationDLQAckLevel:        replicationDlqAckLevel,
		TransferAckLevel:              5,
		TimerAckLevel:                 timerAckLevel,
		ClusterTransferAckLevel:       clusterTransferAckLevel,
		ClusterTimerAckLevel:          clusterTimerAckLevel,
		TransferProcessingQueueStates: transferProcessingQueueStatesBlob,
		TimerProcessingQueueStates:    timerProcessingQueueStatesBlob,
		ClusterReplicationLevel:       clusterReplicationLevel,
		DomainNotificationVersion:     domainNotificationVersion,
		PendingFailoverMarkers:        pendingFailoverMarkerBlob,
	}
}

func sampleShardInfo() *ShardInfo {
	return &ShardInfo{
		ShardID:                       shardID,
		Owner:                         owner,
		RangeID:                       rangeID,
		StolenSinceRenew:              stolenSinceRenew,
		UpdatedAt:                     updatedAt,
		ReplicationAckLevel:           replicationAckLevel,
		ReplicationDLQAckLevel:        replicationDlqAckLevel,
		TransferAckLevel:              transferAckLevel,
		TimerAckLevel:                 timerAckLevel,
		ClusterTransferAckLevel:       clusterTransferAckLevel,
		ClusterTimerAckLevel:          clusterTimerAckLevel,
		TransferProcessingQueueStates: transferProcessingQueueStates,
		TimerProcessingQueueStates:    timerProcesssingQueueStates,
		ClusterReplicationLevel:       clusterReplicationLevel,
		DomainNotificationVersion:     domainNotificationVersion,
		PendingFailoverMarkers:        pendingFailoverMarkers,
	}
}
