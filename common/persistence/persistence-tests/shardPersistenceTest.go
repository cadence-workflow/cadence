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
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cluster"
	"os"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	gen "github.com/uber/cadence/.gen/go/shared"
	p "github.com/uber/cadence/common/persistence"
)

type (
	// ShardPersistenceSuite contains shard persistence tests
	ShardPersistenceSuite struct {
		TestBase
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

	err0 := s.CreateShard(ctx, 19, "test_create_shard1", 123, nil, nil)
	s.Nil(err0, "No error expected.")

	err1 := s.CreateShard(ctx, 19, "test_create_shard2", 124, nil, nil)
	s.NotNil(err1, "expected non nil error.")
	s.IsType(&p.ShardAlreadyExistError{}, err1)
	log.Infof("CreateShard failed with error: %v", err1)
}

// TestGetShard test
func (s *ShardPersistenceSuite) TestGetShard() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	shardID := 20
	owner := "test_get_shard"
	rangeID := int64(131)

	currentClusterTransferAck := int64(21)
	alternativeClusterTransferAck := int64(32)
	currentClusterTimerAck := timestampConvertor(time.Now().Add(-10 * time.Second))
	alternativeClusterTimerAck := timestampConvertor(time.Now().Add(-20 * time.Second))
	transferPQS := createTransferPQS(cluster.TestCurrentClusterName, 0, currentClusterTransferAck,
		cluster.TestAlternativeClusterName, 1, alternativeClusterTransferAck)
	transferPQSBlob, _ := s.PayloadSerializer.SerializeProcessingQueueStates(
		&transferPQS,
		common.EncodingTypeThriftRW,
	)
	timerPQS := createTimerPQS(cluster.TestCurrentClusterName, 0, currentClusterTimerAck,
		cluster.TestAlternativeClusterName, 1, alternativeClusterTimerAck)
	timerPQSBlob, _ := s.PayloadSerializer.SerializeProcessingQueueStates(
		&timerPQS,
		common.EncodingTypeThriftRW,
	)
	err0 := s.CreateShard(ctx, shardID, owner, rangeID, transferPQSBlob, timerPQSBlob)
	s.Nil(err0, "No error expected.")

	shardInfo, err1 := s.GetShard(ctx, shardID)
	s.Nil(err1)
	s.NotNil(shardInfo)
	s.Equal(shardID, shardInfo.ShardID)
	s.Equal(owner, shardInfo.Owner)
	s.Equal(rangeID, shardInfo.RangeID)
	s.Equal(0, shardInfo.StolenSinceRenew)

	s.Equal(shardInfo.TransferProcessingQueueStates, shardInfo.TransferProcessingQueueStates)
	s.Equal(shardInfo.TimerProcessingQueueStates, shardInfo.TimerProcessingQueueStates)
	deserializedTransferPQS, err := s.PayloadSerializer.DeserializeProcessingQueueStates(shardInfo.TransferProcessingQueueStates)
	s.Nil(err)
	s.Equal(&transferPQS, deserializedTransferPQS)
	deserializedTimerPQS, err := s.PayloadSerializer.DeserializeProcessingQueueStates(shardInfo.TimerProcessingQueueStates)
	s.Nil(err)
	s.Equal(&timerPQS, deserializedTimerPQS)

	_, err2 := s.GetShard(ctx, 4766)
	s.NotNil(err2)
	s.IsType(&gen.EntityNotExistsError{}, err2)
	log.Infof("GetShard failed with error: %v", err2)
}

// TestUpdateShard test
func (s *ShardPersistenceSuite) TestUpdateShard() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	shardID := 30
	owner := "test_update_shard"
	rangeID := int64(141)
	err0 := s.CreateShard(ctx, shardID, owner, rangeID, nil, nil)
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
	updatedTransferAckLevel := int64(1000)
	updatedReplicationAckLevel := int64(2000)
	updatedStolenSinceRenew := 10
	updatedInfo := copyShardInfo(shardInfo)
	updatedInfo.Owner = updatedOwner
	updatedInfo.RangeID = updatedRangeID
	updatedInfo.TransferAckLevel = updatedTransferAckLevel
	updatedInfo.ReplicationAckLevel = updatedReplicationAckLevel
	updatedInfo.StolenSinceRenew = updatedStolenSinceRenew
	updatedTimerAckLevel := time.Now()
	updatedInfo.TimerAckLevel = updatedTimerAckLevel
	err2 := s.UpdateShard(ctx, updatedInfo, shardInfo.RangeID)
	s.Nil(err2)

	info1, err3 := s.GetShard(ctx, shardID)
	s.Nil(err3)
	s.NotNil(info1)
	s.Equal(updatedOwner, info1.Owner)
	s.Equal(updatedRangeID, info1.RangeID)
	s.Equal(updatedTransferAckLevel, info1.TransferAckLevel)
	s.Equal(updatedReplicationAckLevel, info1.ReplicationAckLevel)
	s.Equal(updatedStolenSinceRenew, info1.StolenSinceRenew)
	s.EqualTimes(updatedTimerAckLevel, info1.TimerAckLevel)

	failedUpdateInfo := copyShardInfo(shardInfo)
	failedUpdateInfo.Owner = "failed_owner"
	failedUpdateInfo.TransferAckLevel = int64(4000)
	failedUpdateInfo.ReplicationAckLevel = int64(5000)
	err4 := s.UpdateShard(ctx, failedUpdateInfo, shardInfo.RangeID)
	s.NotNil(err4)
	s.IsType(&p.ShardOwnershipLostError{}, err4)
	log.Infof("Update shard failed with error: %v", err4)

	info2, err5 := s.GetShard(ctx, shardID)
	s.Nil(err5)
	s.NotNil(info2)
	s.Equal(updatedOwner, info2.Owner)
	s.Equal(updatedRangeID, info2.RangeID)
	s.Equal(updatedTransferAckLevel, info2.TransferAckLevel)
	s.Equal(updatedReplicationAckLevel, info2.ReplicationAckLevel)
	s.Equal(updatedStolenSinceRenew, info2.StolenSinceRenew)
	s.EqualTimes(updatedTimerAckLevel, info1.TimerAckLevel)
}

func copyShardInfo(sourceInfo *p.ShardInfo) *p.ShardInfo {
	return &p.ShardInfo{
		ShardID:             sourceInfo.ShardID,
		Owner:               sourceInfo.Owner,
		RangeID:             sourceInfo.RangeID,
		TransferAckLevel:    sourceInfo.TransferAckLevel,
		ReplicationAckLevel: sourceInfo.ReplicationAckLevel,
		StolenSinceRenew:    sourceInfo.StolenSinceRenew,
		TimerAckLevel:       sourceInfo.TimerAckLevel,
	}
}
