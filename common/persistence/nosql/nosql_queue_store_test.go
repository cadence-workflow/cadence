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

package nosql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

const testQueueType = persistence.DomainReplicationQueueType
const testDLQueueType = -testQueueType // type for dlq is always -MainQueueType

var testPayload = []byte("test-message")

type queueStoreTestData struct {
	mockDB *nosqlplugin.MockDB
	queue  persistence.QueueStore
}

func newQueueStoreTestData(t *testing.T) *queueStoreTestData {
	var testData queueStoreTestData
	ctrl := gomock.NewController(t)

	testData.mockDB = nosqlplugin.NewMockDB(ctrl)

	mockPlugin := nosqlplugin.NewMockPlugin(ctrl)
	mockPlugin.EXPECT().CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).Return(testData.mockDB, nil).AnyTimes()
	RegisterPluginForTest(t, "cassandra", mockPlugin)
	return &testData
}

func (td *queueStoreTestData) newQueueStore() (persistence.QueueStore, error) {
	cfg := getValidShardedNoSQLConfig()
	return newNoSQLQueueStore(cfg, log.NewNoop(), metrics.NewNoopMetricsClient(), testQueueType, nil)
}

func (td *queueStoreTestData) createValidQueueStore(t *testing.T) persistence.QueueStore {
	const initialQueueVersion = int64(0)

	row := nosqlplugin.QueueMetadataRow{
		QueueType:        testQueueType,
		Version:          initialQueueVersion,
		CurrentTimeStamp: time.Now(),
	}
	// return no queue metadata which should force to create a new one
	mainQueueCheckExists := td.mockDB.EXPECT().SelectQueueMetadata(gomock.Any(), testQueueType).Return(nil, nil)
	mainMetadataInsert := td.mockDB.EXPECT().InsertQueueMetadata(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(_ context.Context, actualRequest nosqlplugin.QueueMetadataRow) error {
				assert.Equal(t, row.QueueType, actualRequest.QueueType)
				assert.Equal(t, row.Version, actualRequest.Version)

				assert.WithinDuration(t, row.CurrentTimeStamp, actualRequest.CurrentTimeStamp, time.Second)
				return nil
			})

	// now the corresponding DLQ metadata should be created
	dlqCheckExists := td.mockDB.EXPECT().SelectQueueMetadata(gomock.Any(), testDLQueueType).Return(nil, nil)
	dlqMetadataInsert := td.mockDB.EXPECT().InsertQueueMetadata(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(_ context.Context, actualRequest nosqlplugin.QueueMetadataRow) error {
				assert.Equal(t, testDLQueueType, actualRequest.QueueType)
				assert.Equal(t, row.Version, actualRequest.Version)

				assert.WithinDuration(t, row.CurrentTimeStamp, actualRequest.CurrentTimeStamp, time.Second)
				return nil
			})

	gomock.InOrder(
		mainQueueCheckExists,
		mainMetadataInsert,
		dlqCheckExists,
		dlqMetadataInsert,
	)

	store, err := td.newQueueStore()
	require.NoError(t, err)
	require.NotNil(t, store)

	return store
}

func (td *queueStoreTestData) mockIsNotFoundErrCheck(err error, notfound bool) {
	td.mockDB.EXPECT().IsNotFoundError(err).Return(notfound)
}

func (td *queueStoreTestData) mockErrConversion(err error) {
	td.mockDB.EXPECT().IsNotFoundError(err).Return(false)
	td.mockDB.EXPECT().IsTimeoutError(err).Return(false)
	td.mockDB.EXPECT().IsDBUnavailableError(err).Return(false)
	td.mockDB.EXPECT().IsThrottlingError(err).Return(false)
}

func TestNewNoSQLQueueStore_Succeeds(t *testing.T) {
	td := newQueueStoreTestData(t)
	td.createValidQueueStore(t) // all the validation already performed inside
}

func TestNewNoSQLQueueStore_FailsIfCantReadMetadata(t *testing.T) {
	selectErr := errors.New("select main-queue metadata failed")
	td := newQueueStoreTestData(t)

	td.mockDB.EXPECT().SelectQueueMetadata(gomock.Any(), testQueueType).Return(nil, selectErr)
	td.mockIsNotFoundErrCheck(selectErr, false)

	td.mockErrConversion(selectErr)

	store, err := td.newQueueStore()
	assert.ErrorContains(t, err, selectErr.Error())
	assert.Nil(t, store)
}

func TestNewNoSQLQueueStore_FailsIfCantInsertMetadata(t *testing.T) {
	insertErr := errors.New("insert main-queue metadata failed")
	td := newQueueStoreTestData(t)

	td.mockDB.EXPECT().SelectQueueMetadata(gomock.Any(), testQueueType).Return(nil, nil)
	td.mockDB.EXPECT().InsertQueueMetadata(gomock.Any(), gomock.Any()).Return(insertErr)
	td.mockErrConversion(insertErr)

	store, err := td.newQueueStore()
	assert.ErrorContains(t, err, insertErr.Error())
	assert.Nil(t, store)
}

func TestNewNoSQLQueueStore_FailsIfCantReadDLQMetadata(t *testing.T) {
	errSelect := errors.New("select dlq metadata failed")

	td := newQueueStoreTestData(t)
	mainQueueCheckExists := td.mockDB.EXPECT().SelectQueueMetadata(gomock.Any(), testQueueType).Return(nil, nil)
	mainMetadataInsert := td.mockDB.EXPECT().InsertQueueMetadata(gomock.Any(), gomock.Any()).
		Return(nil)

	dlqCheckExists := td.mockDB.EXPECT().SelectQueueMetadata(gomock.Any(), testDLQueueType).Return(nil, errSelect)
	td.mockIsNotFoundErrCheck(errSelect, false)

	gomock.InOrder(
		mainQueueCheckExists,
		mainMetadataInsert,
		dlqCheckExists,
	)

	td.mockErrConversion(errSelect)

	store, err := td.newQueueStore()
	assert.ErrorContains(t, err, errSelect.Error())
	assert.Nil(t, store)
}

func TestNewNoSQLQueueStore_FailsIfCantInsertDLQMetadata(t *testing.T) {
	errInsert := errors.New("insert dlq metadata failed")

	td := newQueueStoreTestData(t)
	mainQueueCheckExists := td.mockDB.EXPECT().SelectQueueMetadata(gomock.Any(), testQueueType).Return(nil, nil)
	mainMetadataInsert := td.mockDB.EXPECT().InsertQueueMetadata(gomock.Any(), gomock.Any()).
		Return(nil)

	dlqCheckExists := td.mockDB.EXPECT().SelectQueueMetadata(gomock.Any(), testDLQueueType).Return(nil, nil)
	dlqMetadataInsert := td.mockDB.EXPECT().InsertQueueMetadata(gomock.Any(), gomock.Any()).Return(errInsert)

	gomock.InOrder(
		mainQueueCheckExists,
		mainMetadataInsert,
		dlqCheckExists,
		dlqMetadataInsert,
	)

	td.mockErrConversion(errInsert)

	store, err := td.newQueueStore()
	assert.ErrorContains(t, err, errInsert.Error())
	assert.Nil(t, store)
}

func TestEnqueueMessage_Succeeds(t *testing.T) {
	const lastMessageID = int64(123)
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	// clusterAckLevels are affecting lastMessageID
	clusterAckLevels := map[string]int64{"cluster1": lastMessageID + 10, "cluster2": lastMessageID + 20}

	td.mockDB.EXPECT().SelectLastEnqueuedMessageID(ctx, testQueueType).Return(lastMessageID, nil)
	td.mockDB.EXPECT().SelectQueueMetadata(ctx, testQueueType).
		Return(&nosqlplugin.QueueMetadataRow{ClusterAckLevels: clusterAckLevels}, nil)
	td.mockDB.EXPECT().InsertIntoQueue(ctx, gomock.Any()).
		Do(func(_ context.Context, row *nosqlplugin.QueueMessageRow) {
			assert.Equal(
				t,
				&nosqlplugin.QueueMessageRow{
					QueueType:        testQueueType,
					ID:               lastMessageID + 20 + 1, // should be the max of cluster AckLevels + 1
					Payload:          testPayload,
					CurrentTimeStamp: FixedTime,
				},
				row,
			)
		}).Return(nil)

	require.NoError(t, store.EnqueueMessage(ctx, &persistence.InternalEnqueueMessageRequest{
		MessagePayload:   testPayload,
		CurrentTimeStamp: FixedTime,
	}))
}

func TestEnqueueMessage_FailsIfCantSelectLastMessageID(t *testing.T) {
	errSelect := errors.New("failed to select message ID")

	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectLastEnqueuedMessageID(ctx, testQueueType).Return(int64(0), errSelect)
	td.mockIsNotFoundErrCheck(errSelect, false)
	td.mockErrConversion(errSelect)

	assert.ErrorContains(t, store.EnqueueMessage(ctx, &persistence.InternalEnqueueMessageRequest{
		MessagePayload:   testPayload,
		CurrentTimeStamp: FixedTime,
	}), errSelect.Error())
}

func TestEnqueueMessage_FailsIfCantSelectQueueMetadata(t *testing.T) {
	errSelect := errors.New("fail to select queue metadata")
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectLastEnqueuedMessageID(ctx, testQueueType).Return(int64(0), nil)
	td.mockDB.EXPECT().SelectQueueMetadata(ctx, testQueueType).Return(nil, errSelect)
	td.mockIsNotFoundErrCheck(errSelect, false)
	td.mockErrConversion(errSelect)

	assert.ErrorContains(t, store.EnqueueMessage(ctx, &persistence.InternalEnqueueMessageRequest{
		MessagePayload:   testPayload,
		CurrentTimeStamp: FixedTime,
	}), errSelect.Error())
}

func TestEnqueueMessage_FailsIfCantInsertMessageToQueue(t *testing.T) {
	errInsert := errors.New("fail to insert into queue")
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectLastEnqueuedMessageID(ctx, testQueueType).Return(int64(0), nil)
	td.mockDB.EXPECT().SelectQueueMetadata(ctx, testQueueType).
		Return(&nosqlplugin.QueueMetadataRow{}, nil)
	td.mockDB.EXPECT().InsertIntoQueue(ctx, gomock.Any()).Return(errInsert)
	td.mockErrConversion(errInsert)

	assert.ErrorContains(t, store.EnqueueMessage(ctx, &persistence.InternalEnqueueMessageRequest{
		MessagePayload:   testPayload,
		CurrentTimeStamp: FixedTime,
	}), errInsert.Error())
}

func TestEnqueueMessageToDLQ_Succeeds(t *testing.T) {
	const dlqMessageType = -testQueueType
	lastMessageID := int64(123)
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectLastEnqueuedMessageID(ctx, dlqMessageType).Return(lastMessageID, nil)
	td.mockDB.EXPECT().InsertIntoQueue(ctx, gomock.Any()).
		Do(func(_ context.Context, row *nosqlplugin.QueueMessageRow) {
			assert.Equal(
				t,
				&nosqlplugin.QueueMessageRow{
					QueueType:        dlqMessageType,
					ID:               lastMessageID + 1,
					Payload:          testPayload,
					CurrentTimeStamp: FixedTime,
				},
				row,
			)
		}).Return(nil)

	require.NoError(t, store.EnqueueMessageToDLQ(ctx, &persistence.InternalEnqueueMessageToDLQRequest{
		MessagePayload:   testPayload,
		CurrentTimeStamp: FixedTime,
	}))
}

func TestEnqueueMessageToDLQ_FailsIfCantSelectLastMessageID(t *testing.T) {
	errSelect := errors.New("failed to select message ID")

	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectLastEnqueuedMessageID(ctx, testDLQueueType).Return(int64(0), errSelect)
	td.mockIsNotFoundErrCheck(errSelect, false)
	td.mockErrConversion(errSelect)

	assert.ErrorContains(t, store.EnqueueMessageToDLQ(ctx, &persistence.InternalEnqueueMessageToDLQRequest{
		MessagePayload:   testPayload,
		CurrentTimeStamp: FixedTime,
	}), errSelect.Error())
}

func TestEnqueueMessageToDLQ_FailsIfCantInsertMessageToQueue(t *testing.T) {
	errInsert := errors.New("fail to insert into queue")
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectLastEnqueuedMessageID(ctx, testDLQueueType).Return(int64(0), nil)
	td.mockDB.EXPECT().InsertIntoQueue(ctx, gomock.Any()).Return(errInsert)
	td.mockErrConversion(errInsert)

	assert.ErrorContains(t, store.EnqueueMessageToDLQ(ctx, &persistence.InternalEnqueueMessageToDLQRequest{
		MessagePayload:   testPayload,
		CurrentTimeStamp: FixedTime,
	}), errInsert.Error())
}

func TestReadMessages_Succeeds(t *testing.T) {
	const lastMessageID = int64(123)
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	messages := []*nosqlplugin.QueueMessageRow{
		{
			QueueType: testQueueType,
			ID:        124,
			Payload:   []byte("message-124"),
		},
		{
			QueueType: testQueueType,
			ID:        125,
			Payload:   []byte("message-125"),
		},
	}
	td.mockDB.EXPECT().SelectMessagesFrom(ctx, testQueueType, lastMessageID, len(messages)).Return(messages, nil)

	res, err := store.ReadMessages(ctx, &persistence.InternalReadMessagesRequest{
		LastMessageID: lastMessageID,
		MaxCount:      len(messages),
	})
	require.NoError(t, err)
	resMessages := res.Messages

	// resMessages has different type than messages, should compare explicitly
	assert.Len(t, resMessages, len(messages), "should match the amount of messages returned by SelectMessagesFrom()")
	for i := range messages {
		assert.Equal(t, messages[i].QueueType, resMessages[i].QueueType)
		assert.Equal(t, messages[i].ID, resMessages[i].ID)
		assert.Equal(t, messages[i].Payload, resMessages[i].Payload)
	}
}

func TestReadMessages_FailsIfCantSelectMessages(t *testing.T) {
	errSelect := errors.New("failed to select messages")
	const lastMessageID = int64(123)
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectMessagesFrom(ctx, testQueueType, lastMessageID, 2).Return(nil, errSelect)
	td.mockErrConversion(errSelect)

	res, err := store.ReadMessages(ctx, &persistence.InternalReadMessagesRequest{
		LastMessageID: lastMessageID,
		MaxCount:      2,
	})
	assert.ErrorContains(t, err, errSelect.Error())
	assert.Nil(t, res)
}

func TestReadMessagesFromDLQ_Succeeds(t *testing.T) {
	const firsMessageID = int64(123)
	const lastMessageID = int64(200) // doesn't matter, we will still request max=2
	var pageToken = []byte("page-token")
	var nextPageToken = []byte("next-page-token")

	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	messages := []nosqlplugin.QueueMessageRow{
		{
			QueueType: testQueueType,
			ID:        124,
			Payload:   []byte("message-123"),
		},
		{
			QueueType: testQueueType,
			ID:        125,
			Payload:   []byte("message-124"),
		},
	}

	td.mockDB.EXPECT().SelectMessagesBetween(ctx, gomock.Any()).Do(
		func(_ context.Context, req nosqlplugin.SelectMessagesBetweenRequest) {
			expectedReq := nosqlplugin.SelectMessagesBetweenRequest{
				QueueType:               testDLQueueType,
				ExclusiveBeginMessageID: firsMessageID,
				InclusiveEndMessageID:   lastMessageID,
				PageSize:                len(messages),
				NextPageToken:           pageToken,
			}
			assert.Equal(t, expectedReq, req)
		}).Return(&nosqlplugin.SelectMessagesBetweenResponse{NextPageToken: nextPageToken, Rows: messages}, nil)

	res, err := store.ReadMessagesFromDLQ(ctx, &persistence.InternalReadMessagesFromDLQRequest{
		FirstMessageID: firsMessageID,
		LastMessageID:  lastMessageID,
		PageSize:       len(messages),
		PageToken:      pageToken,
	})
	require.NoError(t, err)
	resMessages := res.Messages
	resPageToken := res.NextPageToken

	// resMessages has different type than messages, should compare explicitly
	assert.Len(t, resMessages, 2, "should match the amount of messages returned by SelectMessagesFrom()")
	for i := range messages {
		assert.Equal(t, messages[i].QueueType, resMessages[i].QueueType)
		assert.Equal(t, messages[i].ID, resMessages[i].ID)
		assert.Equal(t, messages[i].Payload, resMessages[i].Payload)
	}

	assert.Equal(t, nextPageToken, resPageToken)
}

func TestReadMessagesFromDLQ_FailsIfSelectMessagesFails(t *testing.T) {
	errSelect := errors.New("failed to select messages")
	const firsMessageID = int64(123)
	const lastMessageID = int64(200) // doesn't matter, we will still request max=2
	const pageSize = 2
	var pageToken = []byte("page-token")

	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectMessagesBetween(ctx, gomock.Any()).Return(nil, errSelect)
	td.mockErrConversion(errSelect)

	res, err := store.ReadMessagesFromDLQ(ctx, &persistence.InternalReadMessagesFromDLQRequest{
		FirstMessageID: firsMessageID,
		LastMessageID:  lastMessageID,
		PageSize:       pageSize,
		PageToken:      pageToken,
	})
	assert.ErrorContains(t, err, errSelect.Error())
	assert.Nil(t, res)
}

func TestDeleteMessagesBefore_Succeeds(t *testing.T) {
	messageID := int64(123)
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().DeleteMessagesBefore(ctx, testQueueType, messageID).Return(nil)
	assert.NoError(t, store.DeleteMessagesBefore(ctx, &persistence.InternalDeleteMessagesBeforeRequest{
		MessageID: messageID,
	}))
}

func TestDeleteMessagesBefore_FailsIfDeleteFails(t *testing.T) {
	errDelete := errors.New("failed to delete messages")
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().DeleteMessagesBefore(ctx, testQueueType, gomock.Any()).Return(errDelete)
	td.mockErrConversion(errDelete)

	assert.ErrorContains(t, store.DeleteMessagesBefore(ctx, &persistence.InternalDeleteMessagesBeforeRequest{
		MessageID: 0,
	}), errDelete.Error())
}

func TestDeleteMessageFromDLQ_Succeeds(t *testing.T) {
	messageID := int64(123)
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().DeleteMessage(ctx, testDLQueueType, messageID).Return(nil)
	assert.NoError(t, store.DeleteMessageFromDLQ(ctx, &persistence.InternalDeleteMessageFromDLQRequest{
		MessageID: messageID,
	}))
}

func TestDeleteMessageFromDLQ_FailsIfDeleteFails(t *testing.T) {
	errDelete := errors.New("failed to delete messages")
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().DeleteMessage(ctx, testDLQueueType, gomock.Any()).Return(errDelete)
	td.mockErrConversion(errDelete)

	assert.ErrorContains(t, store.DeleteMessageFromDLQ(ctx, &persistence.InternalDeleteMessageFromDLQRequest{
		MessageID: 0,
	}), errDelete.Error())
}

func TestRangeDeleteMessagesFromDLQ_Succeeds(t *testing.T) {
	const fistMessageID = int64(123)
	const lastMessageID = int64(130)

	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().DeleteMessagesInRange(ctx, testDLQueueType, fistMessageID, lastMessageID).Return(nil)
	assert.NoError(t, store.RangeDeleteMessagesFromDLQ(ctx, &persistence.InternalRangeDeleteMessagesFromDLQRequest{
		FirstMessageID: fistMessageID,
		LastMessageID:  lastMessageID,
	}))
}

func TestRangeDeleteMessagesFromDLQ_FailsIfDeleteFails(t *testing.T) {
	const fistMessageID = int64(123)
	const lastMessageID = int64(130)
	errDelete := errors.New("failed to delete messages")

	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().DeleteMessagesInRange(ctx, testDLQueueType, gomock.Any(), gomock.Any()).Return(errDelete)
	td.mockErrConversion(errDelete)

	assert.ErrorContains(
		t,
		store.RangeDeleteMessagesFromDLQ(ctx, &persistence.InternalRangeDeleteMessagesFromDLQRequest{
			FirstMessageID: fistMessageID,
			LastMessageID:  lastMessageID,
		}),
		errDelete.Error(),
	)
}

func TestUpdateAckLevel_Succeeds(t *testing.T) {
	const messageID = 123
	const clusterName = "test-cluster"
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	metadata := nosqlplugin.QueueMetadataRow{
		QueueType: testQueueType,
		ClusterAckLevels: map[string]int64{
			clusterName:         110,
			"unrelated-cluster": 300,
		},
		Version: 3,
	}

	td.mockDB.EXPECT().SelectQueueMetadata(ctx, testQueueType).Return(&metadata, nil)
	td.mockDB.EXPECT().UpdateQueueMetadataCas(ctx, gomock.Any()).
		Do(func(_ context.Context, newMeta nosqlplugin.QueueMetadataRow) {
			assert.Equal(t, int64(4), newMeta.Version, "version should be incremented")
			assert.Equal(t, testQueueType, newMeta.QueueType, "type should remain the same")

			expectedClusterAckLevels := map[string]int64{
				clusterName:         messageID, // messageID is greater
				"unrelated-cluster": 300,
			}
			// only target cluster ack-level should be updated
			assert.Equal(t, expectedClusterAckLevels, newMeta.ClusterAckLevels)
		}).Return(nil)

	assert.NoError(t, store.UpdateAckLevel(ctx, &persistence.InternalUpdateAckLevelRequest{
		MessageID:        messageID,
		ClusterName:      clusterName,
		CurrentTimeStamp: FixedTime,
	}))
}

func TestUpdateAckLevel_FailsIfSelectMetadataFails(t *testing.T) {
	errSelect := errors.New("select metadata failed")
	const messageID = 123
	const clusterName = "test-cluster"
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectQueueMetadata(ctx, testQueueType).Return(nil, errSelect)
	td.mockIsNotFoundErrCheck(errSelect, false)
	td.mockErrConversion(errSelect)
	assert.ErrorContains(t, store.UpdateAckLevel(ctx, &persistence.InternalUpdateAckLevelRequest{
		MessageID:        messageID,
		ClusterName:      clusterName,
		CurrentTimeStamp: FixedTime,
	}), errSelect.Error())
}

func TestUpdateAckLevel_FailsIfUpdateMetadataFails(t *testing.T) {
	errUpdate := errors.New("update metadata failed")
	const messageID = 123
	const clusterName = "test-cluster"
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	emptyMetadata := &nosqlplugin.QueueMetadataRow{ClusterAckLevels: map[string]int64{}}

	td.mockDB.EXPECT().SelectQueueMetadata(ctx, testQueueType).Return(emptyMetadata, nil)
	td.mockDB.EXPECT().UpdateQueueMetadataCas(ctx, gomock.Any()).Return(errUpdate)
	td.mockErrConversion(errUpdate)

	assert.ErrorContains(t, store.UpdateAckLevel(ctx, &persistence.InternalUpdateAckLevelRequest{
		MessageID:        messageID,
		ClusterName:      clusterName,
		CurrentTimeStamp: FixedTime,
	}), errUpdate.Error())
}

func TestUpdateDLQAckLevel_Succeeds(t *testing.T) {
	const messageID = 123
	const clusterName = "test-cluster"
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	metadata := nosqlplugin.QueueMetadataRow{
		QueueType: testDLQueueType,
		ClusterAckLevels: map[string]int64{
			clusterName:         110,
			"unrelated-cluster": 300,
		},
		Version: 3,
	}

	td.mockDB.EXPECT().SelectQueueMetadata(ctx, testDLQueueType).Return(&metadata, nil)
	td.mockDB.EXPECT().UpdateQueueMetadataCas(ctx, gomock.Any()).
		Do(func(_ context.Context, newMeta nosqlplugin.QueueMetadataRow) {
			assert.Equal(t, int64(4), newMeta.Version, "version should be incremented")
			assert.Equal(t, testDLQueueType, newMeta.QueueType, "type should remain the same")

			expectedClusterAckLevels := map[string]int64{
				clusterName:         messageID, // messageID is greater
				"unrelated-cluster": 300,
			}
			// only target cluster ack-level should be updated
			assert.Equal(t, expectedClusterAckLevels, newMeta.ClusterAckLevels)
		}).Return(nil)

	assert.NoError(t, store.UpdateDLQAckLevel(ctx, &persistence.InternalUpdateDLQAckLevelRequest{
		MessageID:        messageID,
		ClusterName:      clusterName,
		CurrentTimeStamp: FixedTime,
	}))
}

func TestGetDLQAckLevels_Succeeds(t *testing.T) {
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	expectedAckLevels := map[string]int64{
		"cluster1": 123,
		"cluster2": 456,
	}

	metadata := nosqlplugin.QueueMetadataRow{
		QueueType:        testDLQueueType,
		ClusterAckLevels: expectedAckLevels,
		Version:          3,
	}

	td.mockDB.EXPECT().SelectQueueMetadata(ctx, testDLQueueType).Return(&metadata, nil)

	resp, err := store.GetDLQAckLevels(ctx, &persistence.InternalGetDLQAckLevelsRequest{})
	require.NoError(t, err)
	assert.Equal(t, expectedAckLevels, resp.AckLevels)
}

func TestGetDLQAckLevels_FailsIfSelectMetadataFails(t *testing.T) {
	errSelect := errors.New("failed to select queue metadata")
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().SelectQueueMetadata(ctx, testDLQueueType).Return(nil, errSelect)
	td.mockIsNotFoundErrCheck(errSelect, false)
	td.mockErrConversion(errSelect)

	resp, err := store.GetDLQAckLevels(ctx, &persistence.InternalGetDLQAckLevelsRequest{})
	assert.ErrorContains(t, err, errSelect.Error())
	assert.Nil(t, resp)
}

func TestGetDLQSize_Succeeds(t *testing.T) {
	const expectedSize int64 = 12345
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().GetQueueSize(ctx, testDLQueueType).Return(expectedSize, nil)

	resp, err := store.GetDLQSize(ctx, &persistence.InternalGetDLQSizeRequest{})
	require.NoError(t, err)
	assert.Equal(t, expectedSize, resp.Size)
}

func TestGetDLQSize_FailsIfGetQueueSizeFails(t *testing.T) {
	expectedErr := errors.New("failed to retrieve queue size")
	td := newQueueStoreTestData(t)
	store := td.createValidQueueStore(t)
	ctx := context.Background()

	td.mockDB.EXPECT().GetQueueSize(ctx, testDLQueueType).Return(int64(0), expectedErr)
	td.mockErrConversion(expectedErr)

	resp, err := store.GetDLQSize(ctx, &persistence.InternalGetDLQSizeRequest{})
	assert.ErrorContains(t, err, expectedErr.Error())
	assert.Nil(t, resp)
}

func TestGetNextID(t *testing.T) {
	tests := map[string]struct {
		acks     map[string]int64
		lastID   int64
		expected int64
	}{
		"expected case - last ID is equal to ack-levels": {
			acks:     map[string]int64{"a": 3},
			lastID:   3,
			expected: 4,
		},
		"expected case - last ID is equal to ack-levels haven't caught up": {
			acks:     map[string]int64{"a": 2},
			lastID:   3,
			expected: 4,
		},
		"error case - ack-levels are ahead for some reason": {
			acks:     map[string]int64{"a": 3},
			lastID:   2,
			expected: 4,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, getNextID(td.acks, td.lastID))
		})
	}
}
