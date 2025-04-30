// Copyright (c) 2017 Uber Technologies, Inc.
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

package persistencetests

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cluster"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// ShardPersistenceSuite contains shard persistence tests
	ShardPersistenceSuite struct {
		*TestBase
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
	}
)

// SetupSuite implementation
func (s *ShardPersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

// SetupTest implementation
func (s *ShardPersistenceSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
}

// TearDownSuite implementation
func (s *ShardPersistenceSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

// TestCreateShard test
func (s *ShardPersistenceSuite) TestCreateShard() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	err0 := s.CreateShard(ctx, 19, "test_create_shard1", 123)
	s.Nil(err0, "No error expected.")

	err1 := s.CreateShard(ctx, 19, "test_create_shard2", 124)
	s.NotNil(err1, "expected non nil error.")
	s.IsType(&p.ShardAlreadyExistError{}, err1)
	s.T().Logf("CreateShard failed with error: %v\n", err1)
}

// TestGetShard test
func (s *ShardPersistenceSuite) TestGetShard() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	shardID := 20
	owner := "test_get_shard"
	rangeID := int64(131)
	err0 := s.CreateShard(ctx, shardID, owner, rangeID)
	s.Nil(err0, "No error expected.")

	shardInfo, err1 := s.GetShard(ctx, shardID)
	s.Nil(err1)
	s.NotNil(shardInfo)
	s.Equal(shardID, shardInfo.ShardID)
	s.Equal(owner, shardInfo.Owner)
	s.Equal(rangeID, shardInfo.RangeID)
	s.Equal(0, shardInfo.StolenSinceRenew)

	_, err2 := s.GetShard(ctx, 4766)
	s.NotNil(err2)
	s.IsType(&types.EntityNotExistsError{}, err2)
	s.T().Logf("GetShard failed with error: %v\n", err2)
}

// TestUpdateShard test
func (s *ShardPersistenceSuite) TestUpdateShard() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	shardID := 30
	owner := "test_update_shard"
	rangeID := int64(141)
	err0 := s.CreateShard(ctx, shardID, owner, rangeID)
	s.Nil(err0, "No error expected.")

	shardInfo, err1 := s.GetShard(ctx, shardID)
	s.Nil(err1)
	s.NotNil(shardInfo)
	s.Equal(shardID, shardInfo.ShardID)
	s.Equal(owner, shardInfo.Owner)
	s.Equal(rangeID, shardInfo.RangeID)
	s.Equal(0, shardInfo.StolenSinceRenew)

	updatedOwner := "updatedOwner"
	updatedRangeID := int64(142)
	updatedCurrentClusterTransferAckLevel := int64(1000)
	updatedAlternativeClusterTransferAckLevel := int64(2000)
	updatedCurrentClusterTimerAckLevel := time.Now()
	updatedAlternativeClusterTimerAckLevel := updatedCurrentClusterTimerAckLevel.Add(time.Minute)
	updatedReplicationAckLevel := int64(2000)
	updatedAlternativeClusterDLQAckLevel := int64(100)
	updatedStolenSinceRenew := 10

	updatedInfo := shardInfo.ToNilSafeCopy()
	updatedInfo.Owner = updatedOwner
	updatedInfo.RangeID = updatedRangeID
	updatedInfo.TransferAckLevel = updatedCurrentClusterTransferAckLevel
	updatedInfo.ClusterTransferAckLevel = map[string]int64{
		cluster.TestCurrentClusterName:     updatedCurrentClusterTransferAckLevel,
		cluster.TestAlternativeClusterName: updatedAlternativeClusterTransferAckLevel,
	}
	updatedInfo.TransferProcessingQueueStates = createProcessingQueueStates(
		cluster.TestCurrentClusterName, 0, updatedCurrentClusterTransferAckLevel,
		cluster.TestAlternativeClusterName, 1, updatedAlternativeClusterTransferAckLevel,
	)
	updatedInfo.TimerAckLevel = updatedCurrentClusterTimerAckLevel
	updatedInfo.ClusterTimerAckLevel = map[string]time.Time{
		cluster.TestCurrentClusterName:     updatedCurrentClusterTimerAckLevel,
		cluster.TestAlternativeClusterName: updatedAlternativeClusterTimerAckLevel,
	}
	updatedInfo.TimerProcessingQueueStates = createProcessingQueueStates(
		cluster.TestCurrentClusterName, 0, updatedCurrentClusterTimerAckLevel.UnixNano(),
		cluster.TestAlternativeClusterName, 1, updatedAlternativeClusterTimerAckLevel.UnixNano(),
	)
	updatedInfo.ReplicationDLQAckLevel = map[string]int64{
		cluster.TestAlternativeClusterName: updatedAlternativeClusterDLQAckLevel,
	}
	updatedInfo.QueueStates = map[int32]*types.QueueState{
		p.HistoryTaskCategoryIDTransfer: &types.QueueState{
			VirtualQueueStates: map[int64]*types.VirtualQueueState{
				0: &types.VirtualQueueState{
					VirtualSliceStates: []*types.VirtualSliceState{
						{
							TaskRange: &types.TaskRange{
								InclusiveMin: &types.TaskKey{
									TaskID: updatedCurrentClusterTransferAckLevel + 1,
								},
								ExclusiveMax: &types.TaskKey{
									TaskID: updatedCurrentClusterTransferAckLevel + 10,
								},
							},
						},
					},
				},
			},
			ExclusiveMaxReadLevel: &types.TaskKey{
				TaskID: updatedCurrentClusterTransferAckLevel + 10,
			},
		},
	}
	updatedInfo.ReplicationAckLevel = updatedReplicationAckLevel
	updatedInfo.StolenSinceRenew = updatedStolenSinceRenew

	err2 := s.UpdateShard(ctx, updatedInfo, shardInfo.RangeID)
	s.Nil(err2)

	info1, err3 := s.GetShard(ctx, shardID)
	s.Nil(err3)
	s.NotNil(info1)
	s.Equal(updatedOwner, info1.Owner)
	s.Equal(updatedRangeID, info1.RangeID)
	s.Equal(updatedCurrentClusterTransferAckLevel, info1.TransferAckLevel)
	s.Equal(updatedInfo.ClusterTransferAckLevel, info1.ClusterTransferAckLevel)
	s.Equal(updatedInfo.TransferProcessingQueueStates, info1.TransferProcessingQueueStates)
	s.EqualTimes(updatedCurrentClusterTimerAckLevel, info1.TimerAckLevel)
	s.EqualTimes(updatedCurrentClusterTimerAckLevel, info1.ClusterTimerAckLevel[cluster.TestCurrentClusterName])
	s.EqualTimes(updatedAlternativeClusterTimerAckLevel, info1.ClusterTimerAckLevel[cluster.TestAlternativeClusterName])
	s.Equal(updatedInfo.TimerProcessingQueueStates, info1.TimerProcessingQueueStates)
	s.Equal(updatedReplicationAckLevel, info1.ReplicationAckLevel)
	s.Equal(updatedInfo.ReplicationDLQAckLevel, info1.ReplicationDLQAckLevel)
	s.Equal(updatedStolenSinceRenew, info1.StolenSinceRenew)

	if s.DynamicConfiguration.ReadNoSQLShardFromDataBlob() || s.ShardMgr.GetName() != "shardedNosql" {
		s.Equal(updatedInfo.QueueStates, info1.QueueStates)
	}

	failedUpdateInfo := shardInfo.ToNilSafeCopy()
	failedUpdateInfo.Owner = "failed_owner"
	failedUpdateInfo.TransferAckLevel = int64(4000)
	failedUpdateInfo.ReplicationAckLevel = int64(5000)
	err4 := s.UpdateShard(ctx, failedUpdateInfo, shardInfo.RangeID)
	s.NotNil(err4)
	s.IsType(&p.ShardOwnershipLostError{}, err4)
	s.T().Logf("Update shard failed with error: %v\n", err4)

	info2, err5 := s.GetShard(ctx, shardID)
	s.Nil(err5)
	s.NotNil(info2)
	s.Equal(updatedOwner, info2.Owner)
	s.Equal(updatedRangeID, info2.RangeID)
	s.Equal(updatedCurrentClusterTransferAckLevel, info2.TransferAckLevel)
	s.Equal(updatedInfo.ClusterTransferAckLevel, info2.ClusterTransferAckLevel)
	s.Equal(updatedInfo.TransferProcessingQueueStates, info2.TransferProcessingQueueStates)
	s.EqualTimes(updatedCurrentClusterTimerAckLevel, info2.TimerAckLevel)
	s.EqualTimes(updatedCurrentClusterTimerAckLevel, info2.ClusterTimerAckLevel[cluster.TestCurrentClusterName])
	s.EqualTimes(updatedAlternativeClusterTimerAckLevel, info2.ClusterTimerAckLevel[cluster.TestAlternativeClusterName])
	s.Equal(updatedInfo.TimerProcessingQueueStates, info2.TimerProcessingQueueStates)
	s.Equal(updatedReplicationAckLevel, info2.ReplicationAckLevel)
	s.Equal(updatedInfo.ReplicationDLQAckLevel, info2.ReplicationDLQAckLevel)
	s.Equal(updatedStolenSinceRenew, info2.StolenSinceRenew)

	if s.DynamicConfiguration.ReadNoSQLShardFromDataBlob() || s.ShardMgr.GetName() != "shardedNosql" {
		s.Equal(updatedInfo.QueueStates, info2.QueueStates)
	}
}

func (s *ShardPersistenceSuite) TestCreateGetShardBackfill() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	shardID := 4
	rangeID := int64(59)

	// test create && get
	currentReplicationAck := int64(27)
	currentClusterTransferAck := int64(21)
	currentClusterTimerAck := timestampConvertor(time.Now().Add(-10 * time.Second))
	shardInfo := &p.ShardInfo{
		ShardID:                 shardID,
		Owner:                   "some random owner",
		RangeID:                 rangeID,
		StolenSinceRenew:        12,
		UpdatedAt:               timestampConvertor(time.Now()),
		ReplicationAckLevel:     currentReplicationAck,
		TransferAckLevel:        currentClusterTransferAck,
		TimerAckLevel:           currentClusterTimerAck,
		ClusterReplicationLevel: map[string]int64{},
		ReplicationDLQAckLevel:  map[string]int64{},
	}
	createRequest := &p.CreateShardRequest{
		ShardInfo: shardInfo,
	}

	s.Nil(s.ShardMgr.CreateShard(ctx, createRequest))

	// ClusterTransfer/TimerAckLevel will be backfilled if not exists when getting shard
	currentClusterName := s.ClusterMetadata.GetCurrentClusterName()
	shardInfo.ClusterTransferAckLevel = map[string]int64{
		currentClusterName: currentClusterTransferAck,
	}
	shardInfo.ClusterTimerAckLevel = map[string]time.Time{
		currentClusterName: currentClusterTimerAck,
	}
	resp, err := s.GetShard(ctx, shardID)
	s.NoError(err)
	s.EqualTimes(shardInfo.UpdatedAt, resp.UpdatedAt)
	s.EqualTimes(shardInfo.TimerAckLevel, resp.TimerAckLevel)
	s.EqualTimes(shardInfo.ClusterTimerAckLevel[currentClusterName], resp.ClusterTimerAckLevel[currentClusterName])

	resp.TimerAckLevel = shardInfo.TimerAckLevel
	resp.UpdatedAt = shardInfo.UpdatedAt
	resp.ClusterTimerAckLevel = shardInfo.ClusterTimerAckLevel
	s.Equal(shardInfo, resp)
}

func (s *ShardPersistenceSuite) TestCreateGetUpdateGetShard() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	shardID := 8
	rangeID := int64(59)

	// test create && get
	currentReplicationAck := int64(27)
	currentClusterTransferAck := int64(21)
	alternativeClusterTransferAck := int64(32)
	currentClusterTimerAck := timestampConvertor(time.Now().Add(-10 * time.Second))
	alternativeClusterTimerAck := timestampConvertor(time.Now().Add(-20 * time.Second))
	domainNotificationVersion := int64(8192)
	transferPQS := createProcessingQueueStates(
		cluster.TestCurrentClusterName, 0, currentClusterTransferAck,
		cluster.TestAlternativeClusterName, 1, alternativeClusterTransferAck,
	)
	timerPQS := createProcessingQueueStates(
		cluster.TestCurrentClusterName, 0, currentClusterTimerAck.UnixNano(),
		cluster.TestAlternativeClusterName, 1, alternativeClusterTimerAck.UnixNano(),
	)
	shardInfo := &p.ShardInfo{
		ShardID:             shardID,
		Owner:               "some random owner",
		RangeID:             rangeID,
		StolenSinceRenew:    12,
		UpdatedAt:           timestampConvertor(time.Now()),
		ReplicationAckLevel: currentReplicationAck,
		TransferAckLevel:    currentClusterTransferAck,
		TimerAckLevel:       currentClusterTimerAck,
		ClusterTransferAckLevel: map[string]int64{
			cluster.TestCurrentClusterName:     currentClusterTransferAck,
			cluster.TestAlternativeClusterName: alternativeClusterTransferAck,
		},
		ClusterTimerAckLevel: map[string]time.Time{
			cluster.TestCurrentClusterName:     currentClusterTimerAck,
			cluster.TestAlternativeClusterName: alternativeClusterTimerAck,
		},
		TransferProcessingQueueStates: transferPQS,
		TimerProcessingQueueStates:    timerPQS,
		DomainNotificationVersion:     domainNotificationVersion,
		ClusterReplicationLevel:       map[string]int64{},
		ReplicationDLQAckLevel:        map[string]int64{},
		QueueStates: map[int32]*types.QueueState{
			p.HistoryTaskCategoryIDTransfer: &types.QueueState{
				VirtualQueueStates: map[int64]*types.VirtualQueueState{
					0: &types.VirtualQueueState{
						VirtualSliceStates: []*types.VirtualSliceState{
							{
								TaskRange: &types.TaskRange{
									InclusiveMin: &types.TaskKey{
										TaskID: currentClusterTransferAck + 1,
									},
									ExclusiveMax: &types.TaskKey{
										TaskID: currentClusterTransferAck + 10,
									},
								},
							},
						},
					},
				},
				ExclusiveMaxReadLevel: &types.TaskKey{
					TaskID: currentClusterTransferAck + 10,
				},
			},
		},
	}
	createRequest := &p.CreateShardRequest{
		ShardInfo: shardInfo,
	}
	s.Nil(s.ShardMgr.CreateShard(ctx, createRequest))
	resp, err := s.GetShard(ctx, shardID)
	s.NoError(err)
	s.EqualTimes(shardInfo.UpdatedAt, resp.UpdatedAt)
	s.EqualTimes(shardInfo.TimerAckLevel, resp.TimerAckLevel)
	s.EqualTimes(shardInfo.ClusterTimerAckLevel[cluster.TestCurrentClusterName], resp.ClusterTimerAckLevel[cluster.TestCurrentClusterName])
	s.EqualTimes(shardInfo.ClusterTimerAckLevel[cluster.TestAlternativeClusterName], resp.ClusterTimerAckLevel[cluster.TestAlternativeClusterName])

	resp.TimerAckLevel = shardInfo.TimerAckLevel
	resp.UpdatedAt = shardInfo.UpdatedAt
	resp.ClusterTimerAckLevel = shardInfo.ClusterTimerAckLevel
	if !s.DynamicConfiguration.ReadNoSQLHistoryTaskFromDataBlob() && s.ShardMgr.GetName() == "shardedNosql" {
		resp.QueueStates = shardInfo.QueueStates
	}
	s.Equal(shardInfo, resp)

	// test update && get
	currentReplicationAck = int64(270)
	currentClusterTransferAck = int64(210)
	alternativeClusterTransferAck = int64(320)
	currentClusterTimerAck = timestampConvertor(time.Now().Add(-100 * time.Second))
	alternativeClusterTimerAck = timestampConvertor(time.Now().Add(-200 * time.Second))
	domainNotificationVersion = int64(16384)
	transferPQS = createProcessingQueueStates(
		cluster.TestCurrentClusterName, 0, currentClusterTransferAck,
		cluster.TestAlternativeClusterName, 1, alternativeClusterTransferAck,
	)
	timerPQS = createProcessingQueueStates(
		cluster.TestCurrentClusterName, 0, currentClusterTimerAck.UnixNano(),
		cluster.TestAlternativeClusterName, 1, alternativeClusterTimerAck.UnixNano(),
	)
	shardInfo = &p.ShardInfo{
		ShardID:             shardID,
		Owner:               "some random owner",
		RangeID:             int64(28),
		StolenSinceRenew:    4,
		UpdatedAt:           timestampConvertor(time.Now()),
		ReplicationAckLevel: currentReplicationAck,
		TransferAckLevel:    currentClusterTransferAck,
		TimerAckLevel:       currentClusterTimerAck,
		ClusterTransferAckLevel: map[string]int64{
			cluster.TestCurrentClusterName:     currentClusterTransferAck,
			cluster.TestAlternativeClusterName: alternativeClusterTransferAck,
		},
		ClusterTimerAckLevel: map[string]time.Time{
			cluster.TestCurrentClusterName:     currentClusterTimerAck,
			cluster.TestAlternativeClusterName: alternativeClusterTimerAck,
		},
		TransferProcessingQueueStates: transferPQS,
		TimerProcessingQueueStates:    timerPQS,
		DomainNotificationVersion:     domainNotificationVersion,
		ClusterReplicationLevel:       map[string]int64{cluster.TestAlternativeClusterName: 12345},
		ReplicationDLQAckLevel:        map[string]int64{},
	}
	updateRequest := &p.UpdateShardRequest{
		ShardInfo:       shardInfo,
		PreviousRangeID: rangeID,
	}
	s.Nil(s.ShardMgr.UpdateShard(ctx, updateRequest))

	resp, err = s.GetShard(ctx, shardID)
	s.NoError(err)
	s.EqualTimes(shardInfo.UpdatedAt, resp.UpdatedAt)
	s.EqualTimes(shardInfo.TimerAckLevel, resp.TimerAckLevel)
	s.EqualTimes(shardInfo.ClusterTimerAckLevel[cluster.TestCurrentClusterName], resp.ClusterTimerAckLevel[cluster.TestCurrentClusterName])
	s.EqualTimes(shardInfo.ClusterTimerAckLevel[cluster.TestAlternativeClusterName], resp.ClusterTimerAckLevel[cluster.TestAlternativeClusterName])

	resp.UpdatedAt = shardInfo.UpdatedAt
	resp.TimerAckLevel = shardInfo.TimerAckLevel
	resp.ClusterTimerAckLevel = shardInfo.ClusterTimerAckLevel
	s.Equal(shardInfo, resp)
}

func createProcessingQueueStates(
	cluster1 string, level1 int32, ackLevel1 int64,
	cluster2 string, level2 int32, ackLevel2 int64,
) *types.ProcessingQueueStates {
	domainFilter := &types.DomainFilter{
		DomainIDs:    nil,
		ReverseMatch: true,
	}
	processingQueueStateMap := map[string][]*types.ProcessingQueueState{}
	if len(cluster1) != 0 {
		processingQueueStateMap[cluster1] = []*types.ProcessingQueueState{
			{
				Level:        common.Int32Ptr(level1),
				AckLevel:     common.Int64Ptr(ackLevel1),
				MaxLevel:     common.Int64Ptr(ackLevel1),
				DomainFilter: domainFilter,
			},
		}
	}
	if len(cluster2) != 0 {
		processingQueueStateMap[cluster2] = []*types.ProcessingQueueState{
			{
				Level:        common.Int32Ptr(level2),
				AckLevel:     common.Int64Ptr(ackLevel2),
				MaxLevel:     common.Int64Ptr(ackLevel2),
				DomainFilter: domainFilter,
			},
		}
	}

	return &types.ProcessingQueueStates{StatesByCluster: processingQueueStateMap}
}
