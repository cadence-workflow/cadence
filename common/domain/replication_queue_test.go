// Copyright (c) 2023 Uber Technologies, Inc.
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

package domain

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	preambleVersion0 byte = 0x59
)

func TestReplicationQueueImpl_Start(t *testing.T) {
	tests := []struct {
		name           string
		initialStatus  int32
		expectedStatus int32
		shouldStart    bool
	}{
		{
			name:           "Should start when initialized",
			initialStatus:  common.DaemonStatusInitialized,
			expectedStatus: common.DaemonStatusStarted,
			shouldStart:    true,
		},
		{
			name:           "Should not start when already started",
			initialStatus:  common.DaemonStatusStarted,
			expectedStatus: common.DaemonStatusStarted,
			shouldStart:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)
			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t)).(*replicationQueueImpl)
			atomic.StoreInt32(&rq.status, tt.initialStatus)

			rq.Start()
			defer rq.Stop()
			assert.Equal(t, tt.expectedStatus, atomic.LoadInt32(&rq.status))

			if tt.shouldStart {
				select {
				case <-rq.done:
					t.Error("purgeProcessor should not have stopped")
				case <-time.After(time.Millisecond):
					// expected, as the purgeProcessor should still be running
				}
			}
		})
	}
}

func TestReplicationQueueImpl_Stop(t *testing.T) {
	tests := []struct {
		name           string
		initialStatus  int32
		expectedStatus int32
		shouldStop     bool
	}{
		{
			name:           "Should stop when started",
			initialStatus:  common.DaemonStatusStarted,
			expectedStatus: common.DaemonStatusStopped,
			shouldStop:     true,
		},
		{
			name:           "Should not stop when not started",
			initialStatus:  common.DaemonStatusInitialized,
			expectedStatus: common.DaemonStatusInitialized,
			shouldStop:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)
			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t)).(*replicationQueueImpl)
			atomic.StoreInt32(&rq.status, tt.initialStatus)

			rq.Stop()
			assert.Equal(t, tt.expectedStatus, atomic.LoadInt32(&rq.status))

			if tt.shouldStop {
				select {
				case <-rq.done:
					// expected channel closed
				default:
					t.Error("done channel should be closed")
				}
			}
		})
	}
}

func TestReplicationQueueImpl_Publish(t *testing.T) {
	tests := []struct {
		name      string
		task      *types.ReplicationTask
		wantErr   bool
		setupMock func(q *persistence.MockQueueManager)
	}{
		{
			name:    "successful publish",
			task:    &types.ReplicationTask{},
			wantErr: false,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().EnqueueMessage(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:    "publish fails",
			task:    &types.ReplicationTask{},
			wantErr: true,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().EnqueueMessage(gomock.Any(), gomock.Any()).Return(errors.New("enqueue error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)
			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t))
			tt.setupMock(mockQueue)
			err := rq.Publish(context.Background(), tt.task)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReplicationQueueImpl_PublishToDLQ(t *testing.T) {
	tests := []struct {
		name      string
		task      *types.ReplicationTask
		wantErr   bool
		setupMock func(q *persistence.MockQueueManager)
	}{
		{
			name:    "successful publish to DLQ",
			task:    &types.ReplicationTask{},
			wantErr: false,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().EnqueueMessageToDLQ(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:    "publish to DLQ fails",
			task:    &types.ReplicationTask{},
			wantErr: true,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().EnqueueMessageToDLQ(gomock.Any(), gomock.Any()).Return(errors.New("enqueue to DLQ error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)
			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t))
			tt.setupMock(mockQueue)
			err := rq.PublishToDLQ(context.Background(), tt.task)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetReplicationMessages(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(mockQueueManager *persistence.MockQueueManager)
		expectedTasks  int
		expectedLastID int64
		expectError    bool
	}{
		{
			name: "handles empty message list",
			setupMocks: func(mockQueueManager *persistence.MockQueueManager) {
				mockQueueManager.EXPECT().
					ReadMessages(gomock.Any(), gomock.Any()).
					Return(&persistence.ReadMessagesResponse{Messages: nil}, nil)
			},
			expectedTasks:  0,
			expectedLastID: 0,
			expectError:    false,
		},
		{
			name: "decodes single message correctly",
			setupMocks: func(mockQueueManager *persistence.MockQueueManager) {
				// Setup mock to return one encoded message
				encodedMessage, _ := mockEncodeReplicationTask(123)
				mockQueueManager.EXPECT().
					ReadMessages(gomock.Any(), gomock.Any()).
					Return(&persistence.ReadMessagesResponse{Messages: []*persistence.QueueMessage{{ID: 1, Payload: encodedMessage}}}, nil)
			},
			expectedTasks:  1,
			expectedLastID: 1,
			expectError:    false,
		},
		{
			name: "decodes multiple messages correctly",
			setupMocks: func(mockQueueManager *persistence.MockQueueManager) {
				// Setup mock to return multiple encoded messages
				encodedMessage1, _ := mockEncodeReplicationTask(123)
				encodedMessage2, _ := mockEncodeReplicationTask(456)
				mockQueueManager.EXPECT().
					ReadMessages(gomock.Any(), gomock.Any()).
					Return(&persistence.ReadMessagesResponse{Messages: []*persistence.QueueMessage{
						{ID: 1, Payload: encodedMessage1},
						{ID: 2, Payload: encodedMessage2},
					}}, nil)
			},
			expectedTasks:  2,
			expectedLastID: 2,
			expectError:    false,
		},
		{
			name:           "read messages fails",
			expectedLastID: 100,
			expectError:    true,
			setupMocks: func(mockQueueManager *persistence.MockQueueManager) {
				mockQueueManager.EXPECT().ReadMessages(gomock.Any(), gomock.Any()).Return(nil, errors.New("read error"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueueManager := persistence.NewMockQueueManager(ctrl)
			tc.setupMocks(mockQueueManager)
			replicationQueue := NewReplicationQueue(mockQueueManager, "testCluster", nil, testlogger.New(t))
			tasks, lastID, err := replicationQueue.GetReplicationMessages(context.Background(), 0, 10)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedTasks, len(tasks))
				assert.Equal(t, tc.expectedLastID, lastID)
			}
		})
	}
}

func TestUpdateAckLevel(t *testing.T) {
	tests := []struct {
		name      string
		lastID    int64
		cluster   string
		wantErr   bool
		setupMock func(q *persistence.MockQueueManager)
	}{
		{
			name:    "successful ack level update",
			lastID:  100,
			cluster: "testCluster",
			wantErr: false,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Eq(&persistence.UpdateAckLevelRequest{
					MessageID:   int64(100),
					ClusterName: "testCluster",
				})).Return(nil)
			},
		},
		{
			name:    "ack level update fails",
			lastID:  100,
			cluster: "testCluster",
			wantErr: true,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Eq(&persistence.UpdateAckLevelRequest{
					MessageID:   int64(100),
					ClusterName: "testCluster",
				})).Return(errors.New("update error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)

			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t))
			tt.setupMock(mockQueue)
			err := rq.UpdateAckLevel(context.Background(), tt.lastID, tt.cluster)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReplicationQueueImpl_GetAckLevels(t *testing.T) {
	tests := []struct {
		name      string
		want      map[string]int64
		wantErr   bool
		setupMock func(q *persistence.MockQueueManager)
	}{
		{
			name:    "successful ack levels retrieval",
			want:    map[string]int64{"testCluster": 100},
			wantErr: false,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().GetAckLevels(gomock.Any(), gomock.Any()).
					Return(&persistence.GetAckLevelsResponse{AckLevels: map[string]int64{"testCluster": 100}}, nil)
			},
		},
		{
			name:    "ack levels retrieval fails",
			wantErr: true,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().GetAckLevels(gomock.Any(), gomock.Any()).Return(nil, errors.New("retrieval error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)
			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t))
			tt.setupMock(mockQueue)
			got, err := rq.GetAckLevels(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func mockEncodeReplicationTask(sourceTaskID int64) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(preambleVersion0)
	binary.Write(&buf, binary.BigEndian, sourceTaskID)
	return buf.Bytes(), nil
}

func TestGetMessagesFromDLQ(t *testing.T) {
	tests := []struct {
		name      string
		firstID   int64
		lastID    int64
		pageSize  int
		pageToken []byte
		taskID    int64
		wantErr   bool
	}{
		{
			name:      "successful message retrieval",
			firstID:   100,
			lastID:    200,
			pageSize:  10,
			pageToken: []byte("token"),
			taskID:    12345,
			wantErr:   false,
		},
		{
			name:      "read messages fails",
			firstID:   100,
			lastID:    200,
			pageSize:  10,
			pageToken: []byte("token"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)
			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t))

			if !tt.wantErr {
				encodedData, _ := mockEncodeReplicationTask(tt.taskID)
				messages := []*persistence.QueueMessage{
					{ID: 1, Payload: encodedData},
				}
				mockQueue.EXPECT().ReadMessagesFromDLQ(gomock.Any(), gomock.Any()).
					Return(&persistence.ReadMessagesFromDLQResponse{Messages: messages, NextPageToken: []byte("nextToken")}, nil)
			} else {
				mockQueue.EXPECT().ReadMessagesFromDLQ(gomock.Any(), gomock.Any()).Return(nil, errors.New("read error"))
			}

			replicationTasks, token, err := rq.GetMessagesFromDLQ(context.Background(), tt.firstID, tt.lastID, tt.pageSize, tt.pageToken)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, replicationTasks, 1, "Expected one replication task to be returned")
				assert.Equal(t, []byte("nextToken"), token, "Expected token to match 'nextToken'")
			}
		})
	}
}

func TestUpdateDLQAckLevel(t *testing.T) {
	tests := []struct {
		name      string
		lastID    int64
		wantErr   bool
		setupMock func(q *persistence.MockQueueManager)
	}{
		{
			name:    "successful DLQ ack level update",
			lastID:  100,
			wantErr: false,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().UpdateDLQAckLevel(gomock.Any(), gomock.Eq(&persistence.UpdateDLQAckLevelRequest{
					MessageID:   int64(100),
					ClusterName: "domainReplication",
				})).Return(nil)
			},
		},
		{
			name:    "DLQ ack level update fails",
			lastID:  100,
			wantErr: true,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().UpdateDLQAckLevel(gomock.Any(), gomock.Eq(&persistence.UpdateDLQAckLevelRequest{
					MessageID:   int64(100),
					ClusterName: "domainReplication",
				})).Return(errors.New("update DLQ ack level error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)
			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t))
			tt.setupMock(mockQueue)
			err := rq.UpdateDLQAckLevel(context.Background(), tt.lastID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetDLQAckLevel(t *testing.T) {
	tests := []struct {
		name      string
		want      int64
		wantErr   bool
		setupMock func(q *persistence.MockQueueManager)
	}{
		{
			name:    "successful DLQ ack level retrieval",
			want:    100,
			wantErr: false,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().GetDLQAckLevels(gomock.Any(), gomock.Any()).
					Return(&persistence.GetDLQAckLevelsResponse{AckLevels: map[string]int64{"domainReplication": 100}}, nil)
			},
		},
		{
			name:    "DLQ ack level retrieval fails",
			wantErr: true,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().GetDLQAckLevels(gomock.Any(), gomock.Any()).Return(nil, errors.New("get DLQ ack level error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)
			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t))
			tt.setupMock(mockQueue)
			got, err := rq.GetDLQAckLevel(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRangeDeleteMessagesFromDLQ(t *testing.T) {
	tests := []struct {
		name      string
		firstID   int64
		lastID    int64
		wantErr   bool
		setupMock func(q *persistence.MockQueueManager)
	}{
		{
			name:    "successful range delete from DLQ",
			firstID: 10,
			lastID:  20,
			wantErr: false,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().RangeDeleteMessagesFromDLQ(gomock.Any(), gomock.Eq(&persistence.RangeDeleteMessagesFromDLQRequest{
					FirstMessageID: int64(10),
					LastMessageID:  int64(20),
				})).Return(nil)
			},
		},
		{
			name:    "range delete from DLQ fails",
			firstID: 10,
			lastID:  20,
			wantErr: true,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().RangeDeleteMessagesFromDLQ(gomock.Any(), gomock.Eq(&persistence.RangeDeleteMessagesFromDLQRequest{
					FirstMessageID: int64(10),
					LastMessageID:  int64(20),
				})).Return(errors.New("range delete error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)
			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t))
			tt.setupMock(mockQueue)
			err := rq.RangeDeleteMessagesFromDLQ(context.Background(), tt.firstID, tt.lastID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeleteMessageFromDLQ(t *testing.T) {
	tests := []struct {
		name      string
		messageID int64
		wantErr   bool
		setupMock func(q *persistence.MockQueueManager)
	}{
		{
			name:      "successful delete from DLQ",
			messageID: 15,
			wantErr:   false,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().DeleteMessageFromDLQ(gomock.Any(), gomock.Eq(&persistence.DeleteMessageFromDLQRequest{
					MessageID: int64(15),
				})).Return(nil)
			},
		},
		{
			name:      "delete from DLQ fails",
			messageID: 15,
			wantErr:   true,
			setupMock: func(q *persistence.MockQueueManager) {
				q.EXPECT().DeleteMessageFromDLQ(gomock.Any(), gomock.Eq(&persistence.DeleteMessageFromDLQRequest{
					MessageID: int64(15),
				})).Return(errors.New("delete error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueue := persistence.NewMockQueueManager(ctrl)
			rq := NewReplicationQueue(mockQueue, "testCluster", nil, testlogger.New(t))
			tt.setupMock(mockQueue)
			err := rq.DeleteMessageFromDLQ(context.Background(), tt.messageID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetDLQSize(t *testing.T) {
	tests := []struct {
		name      string
		wantSize  int64
		wantErr   bool
		setupMock func(m *persistence.MockQueueManager)
	}{
		{
			name:     "returns correct size for non-empty DLQ",
			wantSize: 10,
			wantErr:  false,
			setupMock: func(m *persistence.MockQueueManager) {
				m.EXPECT().GetDLQSize(gomock.Any(), gomock.Any()).Return(&persistence.GetDLQSizeResponse{Size: int64(10)}, nil)
			},
		},
		{
			name:     "returns zero for empty DLQ",
			wantSize: 0,
			wantErr:  false,
			setupMock: func(m *persistence.MockQueueManager) {
				m.EXPECT().GetDLQSize(gomock.Any(), gomock.Any()).Return(&persistence.GetDLQSizeResponse{Size: int64(0)}, nil)
			},
		},
		{
			name:    "propagates error from underlying queue",
			wantErr: true,
			setupMock: func(m *persistence.MockQueueManager) {
				m.EXPECT().GetDLQSize(gomock.Any(), gomock.Any()).Return(nil, errors.New("database error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueueManager := persistence.NewMockQueueManager(ctrl)
			tt.setupMock(mockQueueManager)
			q := &replicationQueueImpl{queue: mockQueueManager, logger: testlogger.New(t)}
			size, err := q.GetDLQSize(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantSize, size)
			}
		})
	}
}

func TestPurgeAckedMessages(t *testing.T) {
	tests := []struct {
		name      string
		wantErr   bool
		setupMock func(m *persistence.MockQueueManager)
	}{
		{
			name:    "successfully purges messages",
			wantErr: false,
			setupMock: func(m *persistence.MockQueueManager) {
				m.EXPECT().GetAckLevels(gomock.Any(), gomock.Any()).
					Return(&persistence.GetAckLevelsResponse{AckLevels: map[string]int64{"clusterA": 5}}, nil)
				m.EXPECT().DeleteMessagesBefore(gomock.Any(), gomock.Eq(&persistence.DeleteMessagesBeforeRequest{
					MessageID: int64(5),
				})).Return(nil)
			},
		},
		{
			name:    "does nothing when no ack levels",
			wantErr: false,
			setupMock: func(m *persistence.MockQueueManager) {
				m.EXPECT().GetAckLevels(gomock.Any(), gomock.Any()).
					Return(&persistence.GetAckLevelsResponse{AckLevels: map[string]int64{}}, nil)
			},
		},
		{
			name:    "error on GetAckLevels",
			wantErr: true,
			setupMock: func(m *persistence.MockQueueManager) {
				m.EXPECT().GetAckLevels(gomock.Any(), gomock.Any()).Return(nil, errors.New("database error"))
			},
		},
		{
			name:    "error on DeleteMessagesBefore",
			wantErr: true,
			setupMock: func(m *persistence.MockQueueManager) {
				m.EXPECT().GetAckLevels(gomock.Any(), gomock.Any()).
					Return(&persistence.GetAckLevelsResponse{AckLevels: map[string]int64{"clusterA": 5}}, nil)
				m.EXPECT().DeleteMessagesBefore(gomock.Any(), gomock.Eq(&persistence.DeleteMessagesBeforeRequest{
					MessageID: int64(5),
				})).Return(errors.New("delete error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQueueManager := persistence.NewMockQueueManager(ctrl)
			tt.setupMock(mockQueueManager)
			q := &replicationQueueImpl{queue: mockQueueManager, logger: testlogger.New(t)}
			err := q.purgeAckedMessages()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
