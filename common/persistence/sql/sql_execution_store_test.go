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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

func TestDeleteCurrentWorkflowExecution(t *testing.T) {
	shardID := int64(100)
	testCases := []struct {
		name      string
		req       *persistence.DeleteCurrentWorkflowExecutionRequest
		mockSetup func(*sqlplugin.MockDB)
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.DeleteCurrentWorkflowExecutionRequest{
				DomainID:   "abdcea69-61d5-44c3-9d55-afe23505a542",
				WorkflowID: "aaaa",
				RunID:      "fd65967f-777d-45de-8dee-be49dfda6716",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().DeleteFromCurrentExecutions(gomock.Any(), &sqlplugin.CurrentExecutionsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "aaaa",
					RunID:      serialization.MustParseUUID("fd65967f-777d-45de-8dee-be49dfda6716"),
				}).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.DeleteCurrentWorkflowExecutionRequest{
				DomainID:   "abdcea69-61d5-44c3-9d55-afe23505a542",
				WorkflowID: "aaaa",
				RunID:      "fd65967f-777d-45de-8dee-be49dfda6716",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().DeleteFromCurrentExecutions(gomock.Any(), &sqlplugin.CurrentExecutionsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "aaaa",
					RunID:      serialization.MustParseUUID("fd65967f-777d-45de-8dee-be49dfda6716"),
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := NewSQLExecutionStore(mockDB, nil, int(shardID), nil, nil, nil)
			require.NoError(t, err, "failed to create execution store")

			tc.mockSetup(mockDB)

			err = store.DeleteCurrentWorkflowExecution(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestGetCurrentExecution(t *testing.T) {
	shardID := int64(100)
	testCases := []struct {
		name      string
		req       *persistence.GetCurrentExecutionRequest
		mockSetup func(*sqlplugin.MockDB)
		want      *persistence.GetCurrentExecutionResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.GetCurrentExecutionRequest{
				DomainID:   "abdcea69-61d5-44c3-9d55-afe23505a542",
				WorkflowID: "aaaa",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().SelectFromCurrentExecutions(gomock.Any(), &sqlplugin.CurrentExecutionsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "aaaa",
				}).Return(&sqlplugin.CurrentExecutionsRow{
					ShardID:          shardID,
					DomainID:         serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID:       "aaaa",
					RunID:            serialization.MustParseUUID("fd65967f-777d-45de-8dee-be49dfda6716"),
					CreateRequestID:  "create",
					State:            2,
					CloseStatus:      3,
					LastWriteVersion: 9,
				}, nil)
			},
			want: &persistence.GetCurrentExecutionResponse{
				StartRequestID:   "create",
				RunID:            "fd65967f-777d-45de-8dee-be49dfda6716",
				State:            2,
				CloseStatus:      3,
				LastWriteVersion: 9,
			},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.GetCurrentExecutionRequest{
				DomainID:   "abdcea69-61d5-44c3-9d55-afe23505a542",
				WorkflowID: "aaaa",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromCurrentExecutions(gomock.Any(), &sqlplugin.CurrentExecutionsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "aaaa",
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := NewSQLExecutionStore(mockDB, nil, int(shardID), nil, nil, nil)
			require.NoError(t, err, "failed to create execution store")

			tc.mockSetup(mockDB)

			got, err := store.GetCurrentExecution(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestGetReplicationTasksFromDLQ(t *testing.T) {
	shardID := 0
	testCases := []struct {
		name      string
		req       *persistence.GetReplicationTasksFromDLQRequest
		mockSetup func(*sqlplugin.MockDB, *serialization.MockTaskSerializer)
		want      *persistence.GetHistoryTasksResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.GetReplicationTasksFromDLQRequest{
				SourceClusterName: "source",
				NextPageToken:     serializePageToken(100),
				MaxReadLevel:      199,
				BatchSize:         1000,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectFromReplicationTasksDLQ(gomock.Any(), &sqlplugin.ReplicationTasksDLQFilter{
					ReplicationTasksFilter: sqlplugin.ReplicationTasksFilter{
						ShardID:            shardID,
						InclusiveMinTaskID: 100,
						ExclusiveMaxTaskID: 1100,
						PageSize:           1000,
					},
					SourceClusterName: "source",
				}).Return([]sqlplugin.ReplicationTasksRow{
					{
						ShardID:      shardID,
						TaskID:       100,
						Data:         []byte(`replication`),
						DataEncoding: "replication",
					},
				}, nil)
				mockParser.EXPECT().DeserializeTask(persistence.HistoryTaskCategoryReplication, persistence.NewDataBlob([]byte(`replication`), "replication")).Return(&persistence.HistoryReplicationTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID:   "abdcea69-61d5-44c3-9d55-afe23505a542",
						WorkflowID: "test",
						RunID:      "abdcea69-61d5-44c3-9d55-afe23505a54a",
					},
					TaskData: persistence.TaskData{
						Version:             202,
						VisibilityTimestamp: time.Unix(1, 1),
					},
					FirstEventID:      10,
					NextEventID:       101,
					BranchToken:       []byte(`bt`),
					NewRunBranchToken: []byte(`nbt`),
				}, nil)
			},
			want: &persistence.GetHistoryTasksResponse{
				Tasks: []persistence.Task{
					&persistence.HistoryReplicationTask{
						WorkflowIdentifier: persistence.WorkflowIdentifier{
							DomainID:   "abdcea69-61d5-44c3-9d55-afe23505a542",
							WorkflowID: "test",
							RunID:      "abdcea69-61d5-44c3-9d55-afe23505a54a",
						},
						TaskData: persistence.TaskData{
							TaskID:              100,
							Version:             202,
							VisibilityTimestamp: time.Unix(1, 1),
						},
						FirstEventID:      10,
						NextEventID:       101,
						BranchToken:       []byte(`bt`),
						NewRunBranchToken: []byte(`nbt`),
					},
				},
				NextPageToken: serializePageToken(101),
			},
			wantErr: false,
		},
		{
			name: "Error case - failed to load from database",
			req: &persistence.GetReplicationTasksFromDLQRequest{
				SourceClusterName: "source",
				NextPageToken:     serializePageToken(100),
				MaxReadLevel:      199,
				BatchSize:         1000,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockTaskSerializer) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromReplicationTasksDLQ(gomock.Any(), &sqlplugin.ReplicationTasksDLQFilter{
					ReplicationTasksFilter: sqlplugin.ReplicationTasksFilter{
						ShardID:            shardID,
						InclusiveMinTaskID: 100,
						ExclusiveMaxTaskID: 1100,
						PageSize:           1000,
					},
					SourceClusterName: "source",
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error case - failed to decode data",
			req: &persistence.GetReplicationTasksFromDLQRequest{
				SourceClusterName: "source",
				NextPageToken:     serializePageToken(100),
				MaxReadLevel:      199,
				BatchSize:         1000,
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectFromReplicationTasksDLQ(gomock.Any(), &sqlplugin.ReplicationTasksDLQFilter{
					ReplicationTasksFilter: sqlplugin.ReplicationTasksFilter{
						ShardID:            shardID,
						InclusiveMinTaskID: 100,
						ExclusiveMaxTaskID: 1100,
						PageSize:           1000,
					},
					SourceClusterName: "source",
				}).Return([]sqlplugin.ReplicationTasksRow{
					{
						ShardID:      shardID,
						TaskID:       101,
						Data:         []byte(`replication`),
						DataEncoding: "replication",
					},
				}, nil)
				mockParser.EXPECT().DeserializeTask(persistence.HistoryTaskCategoryReplication, persistence.NewDataBlob([]byte(`replication`), "replication")).Return(nil, errors.New("some error"))
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
			mockParser := serialization.NewMockTaskSerializer(ctrl)
			store, err := NewSQLExecutionStore(mockDB, nil, int(shardID), nil, mockParser, nil)
			require.NoError(t, err, "failed to create execution store")

			tc.mockSetup(mockDB, mockParser)

			got, err := store.GetReplicationTasksFromDLQ(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestGetReplicationDLQSize(t *testing.T) {
	shardID := 9
	testCases := []struct {
		name      string
		req       *persistence.GetReplicationDLQSizeRequest
		mockSetup func(*sqlplugin.MockDB)
		want      *persistence.GetReplicationDLQSizeResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.GetReplicationDLQSizeRequest{
				SourceClusterName: "source",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().SelectFromReplicationDLQ(gomock.Any(), &sqlplugin.ReplicationTaskDLQFilter{
					SourceClusterName: "source",
					ShardID:           shardID,
				}).Return(int64(1), nil)
			},
			want: &persistence.GetReplicationDLQSizeResponse{
				Size: 1,
			},
			wantErr: false,
		},
		{
			name: "Success case - no row",
			req: &persistence.GetReplicationDLQSizeRequest{
				SourceClusterName: "source",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().SelectFromReplicationDLQ(gomock.Any(), &sqlplugin.ReplicationTaskDLQFilter{
					SourceClusterName: "source",
					ShardID:           shardID,
				}).Return(int64(0), sql.ErrNoRows)
			},
			want: &persistence.GetReplicationDLQSizeResponse{
				Size: 0,
			},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.GetReplicationDLQSizeRequest{
				SourceClusterName: "source",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().SelectFromReplicationDLQ(gomock.Any(), &sqlplugin.ReplicationTaskDLQFilter{
					SourceClusterName: "source",
					ShardID:           shardID,
				}).Return(int64(0), err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := NewSQLExecutionStore(mockDB, nil, int(shardID), nil, nil, nil)
			require.NoError(t, err, "failed to create execution store")

			tc.mockSetup(mockDB)

			got, err := store.GetReplicationDLQSize(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestDeleteReplicationTaskFromDLQ(t *testing.T) {
	shardID := 100
	testCases := []struct {
		name      string
		req       *persistence.DeleteReplicationTaskFromDLQRequest
		mockSetup func(*sqlplugin.MockDB)
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.DeleteReplicationTaskFromDLQRequest{
				TaskID:            123,
				SourceClusterName: "source",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().DeleteMessageFromReplicationTasksDLQ(gomock.Any(), &sqlplugin.ReplicationTasksDLQFilter{
					ReplicationTasksFilter: sqlplugin.ReplicationTasksFilter{
						ShardID: shardID,
						TaskID:  123,
					},
					SourceClusterName: "source",
				}).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.DeleteReplicationTaskFromDLQRequest{
				TaskID:            123,
				SourceClusterName: "source",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().DeleteMessageFromReplicationTasksDLQ(gomock.Any(), &sqlplugin.ReplicationTasksDLQFilter{
					ReplicationTasksFilter: sqlplugin.ReplicationTasksFilter{
						ShardID: shardID,
						TaskID:  123,
					},
					SourceClusterName: "source",
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := NewSQLExecutionStore(mockDB, nil, int(shardID), nil, nil, nil)
			require.NoError(t, err, "failed to create execution store")

			tc.mockSetup(mockDB)

			err = store.DeleteReplicationTaskFromDLQ(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestRangeDeleteReplicationTaskFromDLQ(t *testing.T) {
	shardID := 100
	testCases := []struct {
		name      string
		req       *persistence.RangeDeleteReplicationTaskFromDLQRequest
		mockSetup func(*sqlplugin.MockDB)
		want      *persistence.RangeDeleteReplicationTaskFromDLQResponse
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.RangeDeleteReplicationTaskFromDLQRequest{
				InclusiveBeginTaskID: 123,
				ExclusiveEndTaskID:   345,
				PageSize:             10,
				SourceClusterName:    "source",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteMessageFromReplicationTasksDLQ(gomock.Any(), &sqlplugin.ReplicationTasksDLQFilter{
					ReplicationTasksFilter: sqlplugin.ReplicationTasksFilter{
						ShardID:            shardID,
						InclusiveMinTaskID: 123,
						ExclusiveMaxTaskID: 345,
						PageSize:           10,
					},
					SourceClusterName: "source",
				}).Return(&sqlResult{rowsAffected: 10}, nil)
			},
			want: &persistence.RangeDeleteReplicationTaskFromDLQResponse{
				TasksCompleted: 10,
			},
			wantErr: false,
		},
		{
			name: "Error case",
			req: &persistence.RangeDeleteReplicationTaskFromDLQRequest{
				InclusiveBeginTaskID: 123,
				ExclusiveEndTaskID:   345,
				PageSize:             10,
				SourceClusterName:    "source",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("some error")
				mockDB.EXPECT().RangeDeleteMessageFromReplicationTasksDLQ(gomock.Any(), &sqlplugin.ReplicationTasksDLQFilter{
					ReplicationTasksFilter: sqlplugin.ReplicationTasksFilter{
						ShardID:            shardID,
						InclusiveMinTaskID: 123,
						ExclusiveMaxTaskID: 345,
						PageSize:           10,
					},
					SourceClusterName: "source",
				}).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := NewSQLExecutionStore(mockDB, nil, int(shardID), nil, nil, nil)
			require.NoError(t, err, "failed to create execution store")

			tc.mockSetup(mockDB)

			got, err := store.RangeDeleteReplicationTaskFromDLQ(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestPutReplicationTaskToDLQ(t *testing.T) {
	shardID := 100
	testCases := []struct {
		name      string
		req       *persistence.InternalPutReplicationTaskToDLQRequest
		mockSetup func(*sqlplugin.MockDB, *serialization.MockParser)
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.InternalPutReplicationTaskToDLQRequest{
				SourceClusterName: "source",
				TaskInfo: &persistence.InternalReplicationTaskInfo{
					DomainID:          "abdcea69-61d5-44c3-9d55-afe23505a542",
					WorkflowID:        "test",
					RunID:             "abdcea69-61d5-44c3-9d55-afe23505a54a",
					TaskType:          1,
					TaskID:            101,
					Version:           202,
					FirstEventID:      10,
					NextEventID:       101,
					ScheduledID:       19,
					BranchToken:       []byte(`bt`),
					NewRunBranchToken: []byte(`nbt`),
					CreationTime:      time.Unix(1, 1),
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockParser.EXPECT().ReplicationTaskInfoToBlob(&serialization.ReplicationTaskInfo{
					DomainID:                serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID:              "test",
					RunID:                   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a54a"),
					TaskType:                1,
					Version:                 202,
					FirstEventID:            10,
					NextEventID:             101,
					ScheduledID:             19,
					EventStoreVersion:       persistence.EventStoreVersion,
					NewRunEventStoreVersion: persistence.EventStoreVersion,
					BranchToken:             []byte(`bt`),
					NewRunBranchToken:       []byte(`nbt`),
					CreationTimestamp:       time.Unix(1, 1),
				}).Return(persistence.DataBlob{Data: []byte(`replication`), Encoding: "replication"}, nil)
				mockDB.EXPECT().InsertIntoReplicationTasksDLQ(gomock.Any(), &sqlplugin.ReplicationTaskDLQRow{
					SourceClusterName: "source",
					ShardID:           shardID,
					TaskID:            101,
					Data:              []byte(`replication`),
					DataEncoding:      "replication",
				}).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name: "Error case - failed to encode data",
			req: &persistence.InternalPutReplicationTaskToDLQRequest{
				SourceClusterName: "source",
				TaskInfo: &persistence.InternalReplicationTaskInfo{
					DomainID:          "abdcea69-61d5-44c3-9d55-afe23505a542",
					WorkflowID:        "test",
					RunID:             "abdcea69-61d5-44c3-9d55-afe23505a54a",
					TaskType:          1,
					TaskID:            101,
					Version:           202,
					FirstEventID:      10,
					NextEventID:       101,
					ScheduledID:       19,
					BranchToken:       []byte(`bt`),
					NewRunBranchToken: []byte(`nbt`),
					CreationTime:      time.Unix(1, 1),
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockParser.EXPECT().ReplicationTaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "Error case - failed to insert into database",
			req: &persistence.InternalPutReplicationTaskToDLQRequest{
				SourceClusterName: "source",
				TaskInfo: &persistence.InternalReplicationTaskInfo{
					DomainID:          "abdcea69-61d5-44c3-9d55-afe23505a542",
					WorkflowID:        "test",
					RunID:             "abdcea69-61d5-44c3-9d55-afe23505a54a",
					TaskType:          1,
					TaskID:            101,
					Version:           202,
					FirstEventID:      10,
					NextEventID:       101,
					ScheduledID:       19,
					BranchToken:       []byte(`bt`),
					NewRunBranchToken: []byte(`nbt`),
					CreationTime:      time.Unix(1, 1),
				},
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockParser *serialization.MockParser) {
				mockParser.EXPECT().ReplicationTaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{Data: []byte(`replication`), Encoding: "replication"}, nil)
				err := errors.New("some error")
				mockDB.EXPECT().InsertIntoReplicationTasksDLQ(gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsDupEntryError(err).Return(false)
				mockDB.EXPECT().IsNotFoundError(err).Return(true)
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
			store, err := NewSQLExecutionStore(mockDB, nil, int(shardID), mockParser, nil, nil)
			require.NoError(t, err, "failed to create execution store")

			tc.mockSetup(mockDB, mockParser)

			err = store.PutReplicationTaskToDLQ(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestDeleteWorkflowExecution(t *testing.T) {
	shardID := int64(100)
	testCases := []struct {
		name      string
		req       *persistence.DeleteWorkflowExecutionRequest
		mockSetup func(*sqlplugin.MockDB, *sqlplugin.MockTx)
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.DeleteWorkflowExecutionRequest{
				DomainID:   "abdcea69-61d5-44c3-9d55-afe23505a542",
				WorkflowID: "wid",
				RunID:      "bbdcea69-61d5-44c3-9d55-afe23505a542",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().DeleteFromExecutions(gomock.Any(), &sqlplugin.ExecutionsFilter{
					ShardID:    int(shardID),
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "wid",
					RunID:      serialization.MustParseUUID("bbdcea69-61d5-44c3-9d55-afe23505a542"),
				}).Return(nil, nil)
				mockTx.EXPECT().DeleteFromActivityInfoMaps(gomock.Any(), &sqlplugin.ActivityInfoMapsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "wid",
					RunID:      serialization.MustParseUUID("bbdcea69-61d5-44c3-9d55-afe23505a542"),
				}).Return(nil, nil)
				mockTx.EXPECT().DeleteFromTimerInfoMaps(gomock.Any(), &sqlplugin.TimerInfoMapsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "wid",
					RunID:      serialization.MustParseUUID("bbdcea69-61d5-44c3-9d55-afe23505a542"),
				}).Return(nil, nil)
				mockTx.EXPECT().DeleteFromChildExecutionInfoMaps(gomock.Any(), &sqlplugin.ChildExecutionInfoMapsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "wid",
					RunID:      serialization.MustParseUUID("bbdcea69-61d5-44c3-9d55-afe23505a542"),
				}).Return(nil, nil)
				mockTx.EXPECT().DeleteFromRequestCancelInfoMaps(gomock.Any(), &sqlplugin.RequestCancelInfoMapsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "wid",
					RunID:      serialization.MustParseUUID("bbdcea69-61d5-44c3-9d55-afe23505a542"),
				}).Return(nil, nil)
				mockTx.EXPECT().DeleteFromSignalInfoMaps(gomock.Any(), &sqlplugin.SignalInfoMapsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "wid",
					RunID:      serialization.MustParseUUID("bbdcea69-61d5-44c3-9d55-afe23505a542"),
				}).Return(nil, nil)
				mockTx.EXPECT().DeleteFromBufferedEvents(gomock.Any(), &sqlplugin.BufferedEventsFilter{
					ShardID:    int(shardID),
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "wid",
					RunID:      serialization.MustParseUUID("bbdcea69-61d5-44c3-9d55-afe23505a542"),
				}).Return(nil, nil)
				mockTx.EXPECT().DeleteFromSignalsRequestedSets(gomock.Any(), &sqlplugin.SignalsRequestedSetsFilter{
					ShardID:    shardID,
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "wid",
					RunID:      serialization.MustParseUUID("bbdcea69-61d5-44c3-9d55-afe23505a542"),
				}).Return(nil, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			wantErr: false,
		},
		{
			name: "Error case - failed to delete from executions",
			req: &persistence.DeleteWorkflowExecutionRequest{
				DomainID:   "abdcea69-61d5-44c3-9d55-afe23505a542",
				WorkflowID: "wid",
				RunID:      "bbdcea69-61d5-44c3-9d55-afe23505a542",
			},
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().GetTotalNumDBShards().Return(1)
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().DeleteFromExecutions(gomock.Any(), &sqlplugin.ExecutionsFilter{
					ShardID:    int(shardID),
					DomainID:   serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a542"),
					WorkflowID: "wid",
					RunID:      serialization.MustParseUUID("bbdcea69-61d5-44c3-9d55-afe23505a542"),
				}).Return(nil, errors.New("some error"))
				mockTx.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
				mockTx.EXPECT().Rollback().Return(nil)
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
			store, err := NewSQLExecutionStore(mockDB, nil, int(shardID), nil, nil, nil)
			require.NoError(t, err, "failed to create execution store")

			tc.mockSetup(mockDB, mockTx)

			err = store.DeleteWorkflowExecution(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestTxExecuteShardLocked(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*sqlplugin.MockDB, *sqlplugin.MockTx)
		operation string
		rangeID   int64
		fn        func(sqlplugin.Tx) error
		wantError error
	}{
		{
			name: "Success",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().ReadLockShards(gomock.Any(), gomock.Any()).Return(11, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			operation: "Insert",
			rangeID:   11,
			fn:        func(sqlplugin.Tx) error { return nil },
			wantError: nil,
		},
		{
			name: "Error",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().ReadLockShards(gomock.Any(), gomock.Any()).Return(11, nil)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false)
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false)
			},
			operation: "Insert",
			rangeID:   11,
			fn:        func(sqlplugin.Tx) error { return errors.New("error") },
			wantError: &types.InternalServiceError{Message: "Insert operation failed.  Error: error"},
		},
		{
			name: "Error - shard ownership lost",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().ReadLockShards(gomock.Any(), gomock.Any()).Return(12, nil)
				mockTx.EXPECT().Rollback().Return(nil)
			},
			operation: "Insert",
			rangeID:   11,
			fn:        func(sqlplugin.Tx) error { return errors.New("error") },
			wantError: &persistence.ShardOwnershipLostError{ShardID: 0, Msg: "Failed to lock shard. Previous range ID: 11; new range ID: 12"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := sqlplugin.NewMockDB(ctrl)
			mockTx := sqlplugin.NewMockTx(ctrl)
			tt.mockSetup(mockDB, mockTx)

			s := &sqlExecutionStore{
				shardID: 0,
				sqlStore: sqlStore{
					db:     mockDB,
					logger: testlogger.New(t),
				},
			}

			gotError := s.txExecuteShardLocked(context.Background(), 0, tt.operation, tt.rangeID, tt.fn)
			assert.Equal(t, tt.wantError, gotError)
		})
	}
}

func TestCreateWorkflowExecution(t *testing.T) {
	testCases := []struct {
		name                             string
		req                              *persistence.InternalCreateWorkflowExecutionRequest
		lockCurrentExecutionIfExistsFn   func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error)
		createOrUpdateCurrentExecutionFn func(context.Context, sqlplugin.Tx, persistence.CreateWorkflowMode, int, serialization.UUID, string, serialization.UUID, int, int, string, int64, int64) error
		applyWorkflowSnapshotTxAsNewFn   func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error
		wantErr                          bool
		want                             *persistence.CreateWorkflowExecutionResponse
		assertErr                        func(t *testing.T, err error)
	}{
		{
			name: "Success - mode brand new",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeBrandNew,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return nil, nil
			},
			createOrUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, persistence.CreateWorkflowMode, int, serialization.UUID, string, serialization.UUID, int, int, string, int64, int64) error {
				return nil
			},
			applyWorkflowSnapshotTxAsNewFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			want: &persistence.CreateWorkflowExecutionResponse{},
		},
		{
			name: "Success - mode workflow ID reuse",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeWorkflowIDReuse,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return &sqlplugin.CurrentExecutionsRow{
					State: persistence.WorkflowStateCompleted,
				}, nil
			},
			createOrUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, persistence.CreateWorkflowMode, int, serialization.UUID, string, serialization.UUID, int, int, string, int64, int64) error {
				return nil
			},
			applyWorkflowSnapshotTxAsNewFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			want: &persistence.CreateWorkflowExecutionResponse{},
		},
		{
			name: "Success - mode zombie",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeZombie,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateZombie,
					},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return &sqlplugin.CurrentExecutionsRow{
					RunID: serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a54a"),
				}, nil
			},
			createOrUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, persistence.CreateWorkflowMode, int, serialization.UUID, string, serialization.UUID, int, int, string, int64, int64) error {
				return nil
			},
			applyWorkflowSnapshotTxAsNewFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			want: &persistence.CreateWorkflowExecutionResponse{},
		},
		{
			name: "Error - mode state validation failed",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeZombie,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCreated,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Error - lockCurrentExecutionIfExists failed",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeBrandNew,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return nil, errors.New("some random error")
			},
			wantErr: true,
		},
		{
			name: "Error - mode brand new",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeBrandNew,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return &sqlplugin.CurrentExecutionsRow{
					CreateRequestID:  "test",
					WorkflowID:       "test",
					RunID:            serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a54a"),
					State:            persistence.WorkflowStateCreated,
					CloseStatus:      persistence.WorkflowCloseStatusNone,
					LastWriteVersion: 10,
				}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &persistence.WorkflowExecutionAlreadyStartedError{
					Msg:              "Workflow execution already running. WorkflowId: test",
					StartRequestID:   "test",
					RunID:            "abdcea69-61d5-44c3-9d55-afe23505a54a",
					State:            persistence.WorkflowStateCreated,
					CloseStatus:      persistence.WorkflowCloseStatusNone,
					LastWriteVersion: 10,
				}, err)
			},
		},
		{
			name: "Error - mode workflow ID reuse, version mismatch",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeWorkflowIDReuse,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return &sqlplugin.CurrentExecutionsRow{
					State:            persistence.WorkflowStateCompleted,
					LastWriteVersion: 10,
				}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &persistence.CurrentWorkflowConditionFailedError{
					Msg: "Workflow execution creation condition failed. WorkflowId: , LastWriteVersion: 10, PreviousLastWriteVersion: 0",
				}, err)
			},
		},
		{
			name: "Error - mode workflow ID reuse, state mismatch",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeWorkflowIDReuse,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return &sqlplugin.CurrentExecutionsRow{
					State: persistence.WorkflowStateCreated,
				}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &persistence.CurrentWorkflowConditionFailedError{
					Msg: "Workflow execution creation condition failed. WorkflowId: , State: 0, Expected: 2",
				}, err)
			},
		},
		{
			name: "Error - mode workflow ID reuse, run ID mismatch",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeWorkflowIDReuse,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return &sqlplugin.CurrentExecutionsRow{
					State: persistence.WorkflowStateCompleted,
					RunID: serialization.MustParseUUID("abdcea69-61d5-44c3-9d55-afe23505a54a"),
				}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &persistence.CurrentWorkflowConditionFailedError{
					Msg: "Workflow execution creation condition failed. WorkflowId: , RunID: abdcea69-61d5-44c3-9d55-afe23505a54a, PreviousRunID: ",
				}, err)
			},
		},
		{
			name: "Error - mode zombie, run ID match",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeZombie,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateZombie,
					},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return &sqlplugin.CurrentExecutionsRow{}, nil
			},
			wantErr: true,
		},
		{
			name: "Error - unknown mode",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowMode(100),
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			wantErr: true,
		},
		{
			name: "Error - createOrUpdateCurrentExecution failed",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeBrandNew,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return nil, nil
			},
			createOrUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, persistence.CreateWorkflowMode, int, serialization.UUID, string, serialization.UUID, int, int, string, int64, int64) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
		{
			name: "Error - applyWorkflowSnapshotTxAsNew failed",
			req: &persistence.InternalCreateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.CreateWorkflowModeBrandNew,
				NewWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			lockCurrentExecutionIfExistsFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error) {
				return nil, nil
			},
			createOrUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, persistence.CreateWorkflowMode, int, serialization.UUID, string, serialization.UUID, int, int, string, int64, int64) error {
				return nil
			},
			applyWorkflowSnapshotTxAsNewFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := sqlplugin.NewMockDB(ctrl)
			mockDB.EXPECT().GetTotalNumDBShards().Return(1)
			s := &sqlExecutionStore{
				shardID: 0,
				sqlStore: sqlStore{
					db:     mockDB,
					logger: testlogger.New(t),
				},
				txExecuteShardLockedFn: func(_ context.Context, _ int, _ string, _ int64, fn func(sqlplugin.Tx) error) error {
					return fn(nil)
				},
				lockCurrentExecutionIfExistsFn:   tc.lockCurrentExecutionIfExistsFn,
				createOrUpdateCurrentExecutionFn: tc.createOrUpdateCurrentExecutionFn,
				applyWorkflowSnapshotTxAsNewFn:   tc.applyWorkflowSnapshotTxAsNewFn,
			}

			got, err := s.CreateWorkflowExecution(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, got, "Unexpected result for test case")
			}
		})
	}
}

func TestUpdateWorkflowExecution(t *testing.T) {
	testCases := []struct {
		name                                   string
		req                                    *persistence.InternalUpdateWorkflowExecutionRequest
		assertNotCurrentExecutionFn            func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID) error
		assertRunIDAndUpdateCurrentExecutionFn func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error
		applyWorkflowSnapshotTxAsNewFn         func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error
		applyWorkflowMutationTxFn              func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error
		wantErr                                bool
		assertErr                              func(t *testing.T, err error)
	}{
		{
			name: "Success - mode ignore current",
			req: &persistence.InternalUpdateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.UpdateWorkflowModeIgnoreCurrent,
				UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				},
			},
			applyWorkflowMutationTxFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "Success - mode bypass current",
			req: &persistence.InternalUpdateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.UpdateWorkflowModeBypassCurrent,
				UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertNotCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID) error {
				return nil
			},
			applyWorkflowMutationTxFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "Success - mode update current, new workflow",
			req: &persistence.InternalUpdateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
				UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
				NewWorkflowSnapshot: &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCreated,
					},
				},
			},
			assertRunIDAndUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error {
				return nil
			},
			applyWorkflowMutationTxFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			applyWorkflowSnapshotTxAsNewFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "Success - mode update current, no new workflow",
			req: &persistence.InternalUpdateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
				UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateRunning,
					},
				},
			},
			assertRunIDAndUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error {
				return nil
			},
			applyWorkflowMutationTxFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "Error - mode state validation failed",
			req: &persistence.InternalUpdateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
				UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateZombie,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Error - assertNotCurrentExecution failed",
			req: &persistence.InternalUpdateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.UpdateWorkflowModeBypassCurrent,
				UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertNotCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
		{
			name: "Error - domain ID mismatch",
			req: &persistence.InternalUpdateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
				UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						DomainID: "a8ead65c-9d0d-43a2-a6ad-dd17c99509af",
						State:    persistence.WorkflowStateCompleted,
					},
				},
				NewWorkflowSnapshot: &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						DomainID: "c3fab112-5175-4044-a096-a32e7badd4a8",
					},
				},
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &types.InternalServiceError{
					Message: "UpdateWorkflowExecution: cannot continue as new to another domain",
				}, err)
			},
		},
		{
			name: "Error - assertRunIDAndUpdateCurrentExecution failed",
			req: &persistence.InternalUpdateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
				UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertRunIDAndUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
		{
			name: "Error - applyWorkflowMutationTxFn failed",
			req: &persistence.InternalUpdateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
				UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertRunIDAndUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error {
				return nil
			},
			applyWorkflowMutationTxFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
		{
			name: "Error - applyWorkflowSnapshotTxAsNew failed",
			req: &persistence.InternalUpdateWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.UpdateWorkflowModeUpdateCurrent,
				UpdateWorkflowMutation: persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
				NewWorkflowSnapshot: &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCreated,
					},
				},
			},
			assertRunIDAndUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error {
				return nil
			},
			applyWorkflowMutationTxFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			applyWorkflowSnapshotTxAsNewFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := sqlplugin.NewMockDB(ctrl)
			mockDB.EXPECT().GetTotalNumDBShards().Return(1)
			s := &sqlExecutionStore{
				shardID: 0,
				sqlStore: sqlStore{
					db:     mockDB,
					logger: testlogger.New(t),
				},
				txExecuteShardLockedFn: func(_ context.Context, _ int, _ string, _ int64, fn func(sqlplugin.Tx) error) error {
					return fn(nil)
				},
				assertNotCurrentExecutionFn:            tc.assertNotCurrentExecutionFn,
				assertRunIDAndUpdateCurrentExecutionFn: tc.assertRunIDAndUpdateCurrentExecutionFn,
				applyWorkflowMutationTxFn:              tc.applyWorkflowMutationTxFn,
				applyWorkflowSnapshotTxAsNewFn:         tc.applyWorkflowSnapshotTxAsNewFn,
			}

			err := s.UpdateWorkflowExecution(context.Background(), tc.req)
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

func TestConflictResolveWorkflowExecution(t *testing.T) {
	testCases := []struct {
		name                                   string
		req                                    *persistence.InternalConflictResolveWorkflowExecutionRequest
		assertRunIDAndUpdateCurrentExecutionFn func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error
		assertNotCurrentExecutionFn            func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID) error
		applyWorkflowMutationTxFn              func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error
		applyWorkflowSnapshotTxAsResetFn       func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error
		applyWorkflowSnapshotTxAsNewFn         func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error
		wantErr                                bool
		assertErr                              func(t *testing.T, err error)
	}{
		{
			name: "Success - mode bypass current",
			req: &persistence.InternalConflictResolveWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.ConflictResolveWorkflowModeBypassCurrent,
				ResetWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertNotCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID) error {
				return nil
			},
			applyWorkflowSnapshotTxAsResetFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "Success - mode update current, current workflow exists",
			req: &persistence.InternalConflictResolveWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.ConflictResolveWorkflowModeUpdateCurrent,
				ResetWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
				NewWorkflowSnapshot: &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCreated,
					},
				},
				CurrentWorkflowMutation: &persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertRunIDAndUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error {
				return nil
			},
			applyWorkflowSnapshotTxAsResetFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			applyWorkflowMutationTxFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			applyWorkflowSnapshotTxAsNewFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "Success - mode update current, no current workflow",
			req: &persistence.InternalConflictResolveWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.ConflictResolveWorkflowModeUpdateCurrent,
				ResetWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
				NewWorkflowSnapshot: &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCreated,
					},
				},
			},
			assertRunIDAndUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error {
				return nil
			},
			applyWorkflowSnapshotTxAsResetFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			applyWorkflowSnapshotTxAsNewFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "Error - mode state validation failed",
			req: &persistence.InternalConflictResolveWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.ConflictResolveWorkflowModeUpdateCurrent,
				ResetWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateZombie,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Error - assertNotCurrentExecution failed",
			req: &persistence.InternalConflictResolveWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.ConflictResolveWorkflowModeBypassCurrent,
				ResetWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertNotCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
		{
			name: "Error - assertRunIDAndUpdateCurrentExecution failed",
			req: &persistence.InternalConflictResolveWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.ConflictResolveWorkflowModeUpdateCurrent,
				ResetWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
				NewWorkflowSnapshot: &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCreated,
					},
				},
				CurrentWorkflowMutation: &persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertRunIDAndUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
		{
			name: "Error - applyWorkflowResetSnapshotTx failed",
			req: &persistence.InternalConflictResolveWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.ConflictResolveWorkflowModeBypassCurrent,
				ResetWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertNotCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID) error {
				return nil
			},
			applyWorkflowSnapshotTxAsResetFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
		{
			name: "Error - applyWorkflowMutationTxFn failed",
			req: &persistence.InternalConflictResolveWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.ConflictResolveWorkflowModeUpdateCurrent,
				ResetWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
				NewWorkflowSnapshot: &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCreated,
					},
				},
				CurrentWorkflowMutation: &persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertRunIDAndUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error {
				return nil
			},
			applyWorkflowSnapshotTxAsResetFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			applyWorkflowMutationTxFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
		{
			name: "Error - applyWorkflowSnapshotTxAsNew failed",
			req: &persistence.InternalConflictResolveWorkflowExecutionRequest{
				RangeID: 1,
				Mode:    persistence.ConflictResolveWorkflowModeUpdateCurrent,
				ResetWorkflowSnapshot: persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
				NewWorkflowSnapshot: &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCreated,
					},
				},
				CurrentWorkflowMutation: &persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						State: persistence.WorkflowStateCompleted,
					},
				},
			},
			assertRunIDAndUpdateCurrentExecutionFn: func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error {
				return nil
			},
			applyWorkflowSnapshotTxAsResetFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			applyWorkflowMutationTxFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowMutation, serialization.Parser, serialization.TaskSerializer) error {
				return nil
			},
			applyWorkflowSnapshotTxAsNewFn: func(context.Context, sqlplugin.Tx, int, *persistence.InternalWorkflowSnapshot, serialization.Parser, serialization.TaskSerializer) error {
				return errors.New("some random error")
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := sqlplugin.NewMockDB(ctrl)
			mockDB.EXPECT().GetTotalNumDBShards().Return(1)
			s := &sqlExecutionStore{
				shardID: 0,
				sqlStore: sqlStore{
					db:     mockDB,
					logger: testlogger.New(t),
				},
				txExecuteShardLockedFn: func(_ context.Context, _ int, _ string, _ int64, fn func(sqlplugin.Tx) error) error {
					return fn(nil)
				},
				assertNotCurrentExecutionFn:            tc.assertNotCurrentExecutionFn,
				assertRunIDAndUpdateCurrentExecutionFn: tc.assertRunIDAndUpdateCurrentExecutionFn,
				applyWorkflowMutationTxFn:              tc.applyWorkflowMutationTxFn,
				applyWorkflowSnapshotTxAsResetFn:       tc.applyWorkflowSnapshotTxAsResetFn,
				applyWorkflowSnapshotTxAsNewFn:         tc.applyWorkflowSnapshotTxAsNewFn,
			}

			err := s.ConflictResolveWorkflowExecution(context.Background(), tc.req)
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

func TestCreateFailoverMarkerTasks(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.CreateFailoverMarkersRequest
		mockSetup func(*sqlplugin.MockTx, *serialization.MockParser)
		wantErr   bool
	}{
		{
			name: "Success case",
			req: &persistence.CreateFailoverMarkersRequest{
				RangeID: 1,
				Markers: []*persistence.FailoverMarkerTask{
					{
						TaskData: persistence.TaskData{
							TaskID:              1,
							VisibilityTimestamp: time.Unix(11, 12),
							Version:             101,
						},
						DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
					},
				},
			},
			mockSetup: func(tx *sqlplugin.MockTx, parser *serialization.MockParser) {
				parser.EXPECT().ReplicationTaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Encoding: constants.EncodingTypeThriftRW,
					Data:     []byte("test data"),
				}, nil)
				tx.EXPECT().InsertIntoReplicationTasks(gomock.Any(), []sqlplugin.ReplicationTasksRow{
					{
						ShardID:      0,
						TaskID:       1,
						Data:         []byte("test data"),
						DataEncoding: "thriftrw",
					},
				}).Return(&sqlResult{
					rowsAffected: 1,
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "Error - ReplicationTaskInfoToBlob failed",
			req: &persistence.CreateFailoverMarkersRequest{
				RangeID: 1,
				Markers: []*persistence.FailoverMarkerTask{
					{
						TaskData: persistence.TaskData{
							TaskID:              1,
							VisibilityTimestamp: time.Unix(11, 12),
							Version:             101,
						},
						DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
					},
				},
			},
			mockSetup: func(tx *sqlplugin.MockTx, parser *serialization.MockParser) {
				parser.EXPECT().ReplicationTaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{}, errors.New("some random error"))
			},
			wantErr: true,
		},
		{
			name: "Error - InsertIntoReplicationTasks failed",
			req: &persistence.CreateFailoverMarkersRequest{
				RangeID: 1,
				Markers: []*persistence.FailoverMarkerTask{
					{
						TaskData: persistence.TaskData{
							TaskID:              1,
							VisibilityTimestamp: time.Unix(11, 12),
							Version:             101,
						},
						DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
					},
				},
			},
			mockSetup: func(tx *sqlplugin.MockTx, parser *serialization.MockParser) {
				parser.EXPECT().ReplicationTaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Encoding: constants.EncodingTypeThriftRW,
					Data:     []byte("test data"),
				}, nil)
				tx.EXPECT().InsertIntoReplicationTasks(gomock.Any(), []sqlplugin.ReplicationTasksRow{
					{
						ShardID:      0,
						TaskID:       1,
						Data:         []byte("test data"),
						DataEncoding: "thriftrw",
					},
				}).Return(nil, errors.New("some random error"))
				tx.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			wantErr: true,
		},
		{
			name: "Error - row affected error",
			req: &persistence.CreateFailoverMarkersRequest{
				RangeID: 1,
				Markers: []*persistence.FailoverMarkerTask{
					{
						TaskData: persistence.TaskData{
							TaskID:              1,
							VisibilityTimestamp: time.Unix(11, 12),
							Version:             101,
						},
						DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
					},
				},
			},
			mockSetup: func(tx *sqlplugin.MockTx, parser *serialization.MockParser) {
				parser.EXPECT().ReplicationTaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Encoding: constants.EncodingTypeThriftRW,
					Data:     []byte("test data"),
				}, nil)
				tx.EXPECT().InsertIntoReplicationTasks(gomock.Any(), []sqlplugin.ReplicationTasksRow{
					{
						ShardID:      0,
						TaskID:       1,
						Data:         []byte("test data"),
						DataEncoding: "thriftrw",
					},
				}).Return(&sqlResult{
					err: errors.New("some error"),
				}, nil)
			},
			wantErr: true,
		},
		{
			name: "Error - row affected number mismatch",
			req: &persistence.CreateFailoverMarkersRequest{
				RangeID: 1,
				Markers: []*persistence.FailoverMarkerTask{
					{
						TaskData: persistence.TaskData{
							TaskID:              1,
							VisibilityTimestamp: time.Unix(11, 12),
							Version:             101,
						},
						DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
					},
				},
			},
			mockSetup: func(tx *sqlplugin.MockTx, parser *serialization.MockParser) {
				parser.EXPECT().ReplicationTaskInfoToBlob(gomock.Any()).Return(persistence.DataBlob{
					Encoding: constants.EncodingTypeThriftRW,
					Data:     []byte("test data"),
				}, nil)
				tx.EXPECT().InsertIntoReplicationTasks(gomock.Any(), []sqlplugin.ReplicationTasksRow{
					{
						ShardID:      0,
						TaskID:       1,
						Data:         []byte("test data"),
						DataEncoding: "thriftrw",
					},
				}).Return(&sqlResult{
					rowsAffected: 0,
				}, nil)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := sqlplugin.NewMockDB(ctrl)
			db.EXPECT().GetTotalNumDBShards().Return(1)
			tx := sqlplugin.NewMockTx(ctrl)
			parser := serialization.NewMockParser(ctrl)
			tc.mockSetup(tx, parser)
			s := &sqlExecutionStore{
				shardID: 0,
				sqlStore: sqlStore{
					db:     db,
					logger: testlogger.New(t),
					parser: parser,
				},
				txExecuteShardLockedFn: func(_ context.Context, _ int, _ string, _ int64, fn func(sqlplugin.Tx) error) error {
					return fn(tx)
				},
			}

			err := s.CreateFailoverMarkerTasks(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestGetWorkflowExecution(t *testing.T) {
	testCases := []struct {
		name      string
		req       *persistence.InternalGetWorkflowExecutionRequest
		mockSetup func(*sqlplugin.MockDB, *serialization.MockParser)
		want      *persistence.InternalGetWorkflowExecutionResponse
		wantErr   bool
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "Success case",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
				RangeID: 1,
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromExecutions(gomock.Any(), gomock.Any()).Return([]sqlplugin.ExecutionsRow{
					{
						ShardID:          0,
						DomainID:         serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
						WorkflowID:       "test-workflow-id",
						RunID:            serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
						NextEventID:      101,
						LastWriteVersion: 11,
						Data:             []byte("test data"),
						DataEncoding:     "thriftrw",
					},
				}, nil)
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return([]sqlplugin.ActivityInfoMapsRow{
					{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
						WorkflowID:   "test-workflow-id",
						RunID:        serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
						ScheduleID:   101,
						Data:         []byte("test data"),
						DataEncoding: "thriftrw",
					},
				}, nil)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return([]sqlplugin.TimerInfoMapsRow{
					{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
						WorkflowID:   "test-workflow-id",
						RunID:        serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
						TimerID:      "101",
						Data:         []byte("test data"),
						DataEncoding: "thriftrw",
					},
				}, nil)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return([]sqlplugin.ChildExecutionInfoMapsRow{
					{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
						WorkflowID:   "test-workflow-id",
						RunID:        serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
						InitiatedID:  101,
						Data:         []byte("test data"),
						DataEncoding: "thriftrw",
					},
				}, nil)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return([]sqlplugin.RequestCancelInfoMapsRow{
					{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
						WorkflowID:   "test-workflow-id",
						RunID:        serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
						InitiatedID:  101,
						Data:         []byte("test data"),
						DataEncoding: "thriftrw",
					},
				}, nil)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return([]sqlplugin.SignalInfoMapsRow{
					{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
						WorkflowID:   "test-workflow-id",
						RunID:        serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
						InitiatedID:  101,
						Data:         []byte("test data"),
						DataEncoding: "thriftrw",
					},
				}, nil)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return([]sqlplugin.SignalsRequestedSetsRow{
					{
						ShardID:    0,
						DomainID:   serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
						WorkflowID: "test-workflow-id",
						RunID:      serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
						SignalID:   "test-signal-id",
					},
				}, nil)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return([]sqlplugin.BufferedEventsRow{
					{
						ShardID:      0,
						DomainID:     serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
						WorkflowID:   "test-workflow-id",
						RunID:        serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
						Data:         []byte("test data"),
						DataEncoding: "thriftrw",
					},
				}, nil)
				parser.EXPECT().WorkflowExecutionInfoFromBlob(gomock.Any(), gomock.Any()).Return(&serialization.WorkflowExecutionInfo{
					ParentDomainID:                       serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
					ParentWorkflowID:                     "test-parent-workflow-id",
					ParentRunID:                          serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
					InitiatedID:                          101,
					CompletionEventBatchID:               common.Int64Ptr(11),
					CompletionEvent:                      []byte("test completion event"),
					CompletionEventEncoding:              "json",
					TaskList:                             "test-task-list",
					IsCron:                               true,
					WorkflowTypeName:                     "test-workflow-type",
					WorkflowTimeout:                      time.Duration(101),
					DecisionTaskTimeout:                  time.Duration(102),
					ExecutionContext:                     []byte("test execution context"),
					State:                                persistence.WorkflowStateCompleted,
					CloseStatus:                          persistence.WorkflowCloseStatusCompleted,
					StartVersion:                         111,
					LastWriteEventID:                     common.Int64Ptr(11),
					LastEventTaskID:                      12,
					LastFirstEventID:                     13,
					LastProcessedEvent:                   14,
					StartTimestamp:                       time.Unix(11, 12),
					LastUpdatedTimestamp:                 time.Unix(13, 14),
					DecisionVersion:                      101,
					DecisionScheduleID:                   102,
					DecisionStartedID:                    103,
					DecisionTimeout:                      time.Duration(104),
					DecisionAttempt:                      105,
					DecisionStartedTimestamp:             time.Unix(15, 16),
					DecisionScheduledTimestamp:           time.Unix(17, 18),
					CancelRequested:                      true,
					DecisionOriginalScheduledTimestamp:   time.Unix(19, 20),
					CreateRequestID:                      "test-create-request-id",
					DecisionRequestID:                    "test-decision-request-id",
					CancelRequestID:                      "test-cancel-request-id",
					StickyTaskList:                       "test-sticky-task-list",
					StickyScheduleToStartTimeout:         time.Duration(106),
					RetryAttempt:                         107,
					RetryInitialInterval:                 time.Duration(108),
					RetryMaximumInterval:                 time.Duration(109),
					RetryMaximumAttempts:                 110,
					RetryExpiration:                      time.Duration(111),
					RetryBackoffCoefficient:              111,
					RetryExpirationTimestamp:             time.Unix(23, 24),
					RetryNonRetryableErrors:              []string{"error1", "error2"},
					HasRetryPolicy:                       true,
					CronSchedule:                         "test-cron-schedule",
					EventStoreVersion:                    112,
					EventBranchToken:                     []byte("test-event-branch-token"),
					SignalCount:                          113,
					HistorySize:                          114,
					ClientLibraryVersion:                 "test-client-library-version",
					ClientFeatureVersion:                 "test-client-feature-version",
					ClientImpl:                           "test-client-impl",
					AutoResetPoints:                      []byte("test-auto-reset-points"),
					AutoResetPointsEncoding:              "json",
					SearchAttributes:                     map[string][]byte{"test-key": []byte("test-value")},
					Memo:                                 map[string][]byte{"test-key": []byte("test-value")},
					VersionHistories:                     []byte("test-version-histories"),
					VersionHistoriesEncoding:             "json",
					FirstExecutionRunID:                  serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
					PartitionConfig:                      map[string]string{"test-key": "test-value"},
					Checksum:                             []byte("test-checksum"),
					ChecksumEncoding:                     "test-checksum-encoding",
					ActiveClusterSelectionPolicy:         []byte("ActiveClusterSelectionPolicy"),
					ActiveClusterSelectionPolicyEncoding: "json",
				}, nil)
				parser.EXPECT().ActivityInfoFromBlob(gomock.Any(), gomock.Any()).Return(&serialization.ActivityInfo{
					Version:                  101,
					ScheduledEventBatchID:    102,
					ScheduledEvent:           []byte("test scheduled event"),
					ScheduledEventEncoding:   "json",
					ScheduledTimestamp:       time.Unix(11, 12),
					StartedID:                103,
					StartedEvent:             []byte("test started event"),
					StartedEventEncoding:     "json",
					StartedTimestamp:         time.Unix(13, 14),
					ActivityID:               "test-activity-id",
					RequestID:                "test-request-id",
					ScheduleToStartTimeout:   time.Duration(101),
					ScheduleToCloseTimeout:   time.Duration(102),
					StartToCloseTimeout:      time.Duration(103),
					HeartbeatTimeout:         time.Duration(104),
					CancelRequested:          true,
					CancelRequestID:          105,
					TimerTaskStatus:          105,
					Attempt:                  106,
					TaskList:                 "test-task-list",
					TaskListKind:             types.TaskListKindEphemeral,
					StartedIdentity:          "test-started-identity",
					HasRetryPolicy:           true,
					RetryInitialInterval:     time.Duration(107),
					RetryMaximumInterval:     time.Duration(108),
					RetryMaximumAttempts:     109,
					RetryExpirationTimestamp: time.Unix(15, 16),
					RetryBackoffCoefficient:  110,
					RetryNonRetryableErrors:  []string{"error1", "error2"},
					RetryLastFailureReason:   "test-retry-last-failure-reason",
					RetryLastWorkerIdentity:  "test-retry-last-worker-identity",
					RetryLastFailureDetails:  []byte("test-retry-last-failure-details"),
				}, nil)
				parser.EXPECT().TimerInfoFromBlob(gomock.Any(), gomock.Any()).Return(&serialization.TimerInfo{
					Version:         101,
					StartedID:       102,
					ExpiryTimestamp: time.Unix(11, 12),
					TaskID:          103,
				}, nil)
				parser.EXPECT().ChildExecutionInfoFromBlob(gomock.Any(), gomock.Any()).Return(&serialization.ChildExecutionInfo{
					Version:                101,
					InitiatedEventBatchID:  102,
					InitiatedEvent:         []byte("test initiated event"),
					InitiatedEventEncoding: "json",
					StartedID:              103,
					StartedWorkflowID:      "test-started-workflow-id",
					StartedRunID:           serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
					CreateRequestID:        "test-create-request-id",
					StartedEvent:           []byte("test started event"),
					StartedEventEncoding:   "json",
					DomainID:               "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
					WorkflowTypeName:       "test-workflow-type",
					ParentClosePolicy:      101,
				}, nil)
				parser.EXPECT().RequestCancelInfoFromBlob(gomock.Any(), gomock.Any()).Return(&serialization.RequestCancelInfo{
					Version:               101,
					InitiatedEventBatchID: 102,
					CancelRequestID:       "test-cancel-request-id",
				}, nil)
				parser.EXPECT().SignalInfoFromBlob(gomock.Any(), gomock.Any()).Return(&serialization.SignalInfo{
					Version:               101,
					InitiatedEventBatchID: 102,
					Name:                  "test-signal-name",
					Input:                 []byte("test input"),
					Control:               []byte("test control"),
					RequestID:             "test-signal-request-id",
				}, nil)
				db.EXPECT().SelectFromShards(gomock.Any(), gomock.Any()).Return(&sqlplugin.ShardsRow{
					RangeID: 1,
				}, nil)
			},
			want: &persistence.InternalGetWorkflowExecutionResponse{
				State: &persistence.InternalWorkflowMutableState{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						DomainID:                           "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
						WorkflowID:                         "test-workflow-id",
						RunID:                              "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
						ParentDomainID:                     "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
						ParentWorkflowID:                   "test-parent-workflow-id",
						ParentRunID:                        "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
						InitiatedID:                        101,
						CompletionEventBatchID:             11,
						CompletionEvent:                    persistence.NewDataBlob([]byte("test completion event"), constants.EncodingTypeJSON),
						TaskList:                           "test-task-list",
						IsCron:                             true,
						WorkflowTypeName:                   "test-workflow-type",
						WorkflowTimeout:                    time.Duration(101),
						DecisionStartToCloseTimeout:        time.Duration(102),
						DecisionTimeout:                    time.Duration(104),
						ExecutionContext:                   []byte("test execution context"),
						State:                              persistence.WorkflowStateCompleted,
						CloseStatus:                        persistence.WorkflowCloseStatusCompleted,
						NextEventID:                        101,
						LastEventTaskID:                    12,
						LastFirstEventID:                   13,
						LastProcessedEvent:                 14,
						StartTimestamp:                     time.Unix(11, 12),
						LastUpdatedTimestamp:               time.Unix(13, 14),
						DecisionVersion:                    101,
						DecisionScheduleID:                 102,
						DecisionStartedID:                  103,
						DecisionAttempt:                    105,
						DecisionStartedTimestamp:           time.Unix(15, 16),
						DecisionScheduledTimestamp:         time.Unix(17, 18),
						CancelRequested:                    true,
						DecisionOriginalScheduledTimestamp: time.Unix(19, 20),
						CreateRequestID:                    "test-create-request-id",
						DecisionRequestID:                  "test-decision-request-id",
						CancelRequestID:                    "test-cancel-request-id",
						StickyTaskList:                     "test-sticky-task-list",
						StickyScheduleToStartTimeout:       time.Duration(106),
						HasRetryPolicy:                     true,
						CronSchedule:                       "test-cron-schedule",
						SignalCount:                        113,
						HistorySize:                        114,
						ClientLibraryVersion:               "test-client-library-version",
						ClientFeatureVersion:               "test-client-feature-version",
						ClientImpl:                         "test-client-impl",
						FirstExecutionRunID:                "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
						PartitionConfig:                    map[string]string{"test-key": "test-value"},
						AutoResetPoints:                    persistence.NewDataBlob([]byte("test-auto-reset-points"), constants.EncodingTypeJSON),
						Attempt:                            107,
						InitialInterval:                    time.Duration(108),
						BackoffCoefficient:                 111,
						MaximumInterval:                    time.Duration(109),
						ExpirationTime:                     time.Unix(23, 24),
						MaximumAttempts:                    110,
						NonRetriableErrors:                 []string{"error1", "error2"},
						BranchToken:                        []byte("test-event-branch-token"),
						SearchAttributes:                   map[string][]byte{"test-key": []byte("test-value")},
						Memo:                               map[string][]byte{"test-key": []byte("test-value")},
						ExpirationInterval:                 time.Duration(111),
						ActiveClusterSelectionPolicy:       persistence.NewDataBlob([]byte("ActiveClusterSelectionPolicy"), constants.EncodingTypeJSON),
					},
					VersionHistories: persistence.NewDataBlob([]byte("test-version-histories"), constants.EncodingTypeJSON),
					ReplicationState: &persistence.ReplicationState{
						StartVersion:     111,
						LastWriteVersion: 11,
						LastWriteEventID: 11,
					},
					ActivityInfos: map[int64]*persistence.InternalActivityInfo{
						101: {
							Version:                101,
							ScheduleID:             101,
							ScheduledEventBatchID:  102,
							ScheduledEvent:         persistence.NewDataBlob([]byte("test scheduled event"), constants.EncodingTypeJSON),
							ScheduledTime:          time.Unix(11, 12),
							StartedID:              103,
							StartedTime:            time.Unix(13, 14),
							StartedEvent:           persistence.NewDataBlob([]byte("test started event"), constants.EncodingTypeJSON),
							ActivityID:             "test-activity-id",
							RequestID:              "test-request-id",
							ScheduleToStartTimeout: time.Duration(101),
							ScheduleToCloseTimeout: time.Duration(102),
							StartToCloseTimeout:    time.Duration(103),
							HeartbeatTimeout:       time.Duration(104),
							CancelRequested:        true,
							CancelRequestID:        105,
							TimerTaskStatus:        105,
							Attempt:                106,
							TaskList:               "test-task-list",
							TaskListKind:           types.TaskListKindEphemeral,
							StartedIdentity:        "test-started-identity",
							HasRetryPolicy:         true,
							DomainID:               "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
							InitialInterval:        time.Duration(107),
							MaximumInterval:        time.Duration(108),
							MaximumAttempts:        109,
							ExpirationTime:         time.Unix(15, 16),
							BackoffCoefficient:     110,
							NonRetriableErrors:     []string{"error1", "error2"},
							LastFailureReason:      "test-retry-last-failure-reason",
							LastWorkerIdentity:     "test-retry-last-worker-identity",
							LastFailureDetails:     []byte("test-retry-last-failure-details"),
						},
					},
					TimerInfos: map[string]*persistence.TimerInfo{
						"101": {
							Version:    101,
							StartedID:  102,
							ExpiryTime: time.Unix(11, 12),
							TaskStatus: 103,
							TimerID:    "101",
						},
					},
					ChildExecutionInfos: map[int64]*persistence.InternalChildExecutionInfo{
						101: {
							Version:               101,
							InitiatedID:           101,
							InitiatedEvent:        persistence.NewDataBlob([]byte("test initiated event"), constants.EncodingTypeJSON),
							InitiatedEventBatchID: 102,
							StartedID:             103,
							StartedEvent:          persistence.NewDataBlob([]byte("test started event"), constants.EncodingTypeJSON),
							StartedWorkflowID:     "test-started-workflow-id",
							StartedRunID:          "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
							CreateRequestID:       "test-create-request-id",
							DomainID:              "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
							WorkflowTypeName:      "test-workflow-type",
							ParentClosePolicy:     101,
						},
					},
					RequestCancelInfos: map[int64]*persistence.RequestCancelInfo{
						101: {
							Version:               101,
							InitiatedID:           101,
							InitiatedEventBatchID: 102,
							CancelRequestID:       "test-cancel-request-id",
						},
					},
					SignalInfos: map[int64]*persistence.SignalInfo{
						101: {
							Version:               101,
							InitiatedID:           101,
							InitiatedEventBatchID: 102,
							SignalName:            "test-signal-name",
							Input:                 []byte("test input"),
							Control:               []byte("test control"),
							SignalRequestID:       "test-signal-request-id",
						},
					},
					SignalRequestedIDs: map[string]struct{}{
						"test-signal-id": {},
					},
					BufferedEvents: []*persistence.DataBlob{
						{
							Encoding: constants.EncodingTypeThriftRW,
							Data:     []byte("test data"),
						},
					},
					ChecksumData: persistence.NewDataBlob([]byte("test-checksum"), "test-checksum-encoding"),
				},
			},
			wantErr: false,
		},
		{
			name: "Error - Shard owner changed",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromExecutions(gomock.Any(), gomock.Any()).Return([]sqlplugin.ExecutionsRow{
					{
						ShardID:          0,
						DomainID:         serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
						WorkflowID:       "test-workflow-id",
						RunID:            serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
						NextEventID:      101,
						LastWriteVersion: 11,
						Data:             []byte("test data"),
						DataEncoding:     "thriftrw",
					},
				}, nil)
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				parser.EXPECT().WorkflowExecutionInfoFromBlob(gomock.Any(), gomock.Any()).Return(&serialization.WorkflowExecutionInfo{
					Checksum:         []byte("test-checksum"),
					ChecksumEncoding: "test-checksum-encoding",
				}, nil)
				db.EXPECT().SelectFromShards(gomock.Any(), gomock.Any()).Return(&sqlplugin.ShardsRow{
					RangeID: 1,
				}, nil)
			},
			want: &persistence.InternalGetWorkflowExecutionResponse{
				State: &persistence.InternalWorkflowMutableState{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						DomainID:               "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
						WorkflowID:             "test-workflow-id",
						RunID:                  "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
						NextEventID:            101,
						CompletionEventBatchID: -23,
					},
					ActivityInfos:       map[int64]*persistence.InternalActivityInfo{},
					TimerInfos:          map[string]*persistence.TimerInfo{},
					ChildExecutionInfos: map[int64]*persistence.InternalChildExecutionInfo{},
					RequestCancelInfos:  map[int64]*persistence.RequestCancelInfo{},
					SignalInfos:         map[int64]*persistence.SignalInfo{},
					SignalRequestedIDs:  map[string]struct{}{},
					ChecksumData:        nil,
				},
			},
			wantErr: false,
		},
		{
			name: "Error - failed to get shard",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromExecutions(gomock.Any(), gomock.Any()).Return([]sqlplugin.ExecutionsRow{
					{
						ShardID:          0,
						DomainID:         serialization.MustParseUUID("ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d"),
						WorkflowID:       "test-workflow-id",
						RunID:            serialization.MustParseUUID("ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f"),
						NextEventID:      101,
						LastWriteVersion: 11,
						Data:             []byte("test data"),
						DataEncoding:     "thriftrw",
					},
				}, nil)
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				parser.EXPECT().WorkflowExecutionInfoFromBlob(gomock.Any(), gomock.Any()).Return(&serialization.WorkflowExecutionInfo{
					Checksum:         []byte("test-checksum"),
					ChecksumEncoding: "test-checksum-encoding",
				}, nil)
				db.EXPECT().SelectFromShards(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
			},
			wantErr: true,
		},
		{
			name: "Error - SelectFromExecutions no row",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromExecutions(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.EntityNotExistsError{}, err)
			},
		},
		{
			name: "Error - SelectFromExecutions failed",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromExecutions(gomock.Any(), gomock.Any()).Return(nil, errors.New("some random error"))
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
			},
			wantErr: true,
		},
		{
			name: "Error - SelectFromActivityInfoMaps failed",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, errors.New("some random error"))
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
			},
			wantErr: true,
		},
		{
			name: "Error - SelectFromTimerInfoMaps failed",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, errors.New("some random error"))
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
			},
			wantErr: true,
		},
		{
			name: "Error - SelectFromChildExecutionInfoMaps failed",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, errors.New("some random error"))
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
			},
			wantErr: true,
		},
		{
			name: "Error - SelectFromRequestCancelInfoMaps failed",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, errors.New("some random error"))
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
			},
			wantErr: true,
		},
		{
			name: "Error - SelectFromSignalInfoMaps failed",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, errors.New("some random error"))
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
			},
			wantErr: true,
		},
		{
			name: "Error - SelectFromSignalsRequestedSets failed",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, errors.New("some random error"))
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
			},
			wantErr: true,
		},
		{
			name: "Error - SelectFromBufferedEvents failed",
			req: &persistence.InternalGetWorkflowExecutionRequest{
				DomainID: "ff9c8a3f-0e4f-4d3e-a4d2-6f5f8f3f7d9d",
				Execution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "ee8d7b6e-876c-4b1e-9b6e-5e3e3c6b6b3f",
				},
			},
			mockSetup: func(db *sqlplugin.MockDB, parser *serialization.MockParser) {
				db.EXPECT().SelectFromActivityInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromTimerInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromChildExecutionInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromRequestCancelInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalInfoMaps(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, sql.ErrNoRows)
				db.EXPECT().SelectFromBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, errors.New("some random error"))
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := sqlplugin.NewMockDB(ctrl)
			parser := serialization.NewMockParser(ctrl)
			tc.mockSetup(db, parser)
			s := &sqlExecutionStore{
				shardID: 0,
				sqlStore: sqlStore{
					db:     db,
					logger: testlogger.New(t),
					parser: parser,
				},
			}

			resp, err := s.GetWorkflowExecution(context.Background(), tc.req)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.want, resp, "Response mismatch")
			}
		})
	}
}

func TestRangeCompleteHistoryTask(t *testing.T) {
	ctx := context.Background()
	shardID := 1

	tests := []struct {
		name          string
		request       *persistence.RangeCompleteHistoryTaskRequest
		setupMock     func(*sqlplugin.MockDB)
		expectedError error
	}{
		{
			name: "success - scheduled timer task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTimer,
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0).Add(time.Minute), 0),
				PageSize:            1000,
			},
			setupMock: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteFromTimerTasks(ctx, &sqlplugin.TimerTasksFilter{
					ShardID:                shardID,
					MinVisibilityTimestamp: time.Unix(0, 0),
					MaxVisibilityTimestamp: time.Unix(0, 0).Add(time.Minute),
					PageSize:               1000,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			expectedError: nil,
		},
		{
			name: "success - immediate transfer task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTransfer,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            1000,
			},
			setupMock: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteFromTransferTasks(ctx, &sqlplugin.TransferTasksFilter{
					ShardID:            shardID,
					InclusiveMinTaskID: 100,
					ExclusiveMaxTaskID: 200,
					PageSize:           1000,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			expectedError: nil,
		},
		{
			name: "success - immediate replication task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryReplication,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100), // this is ignored by replication task
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            1000,
			},
			setupMock: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteFromReplicationTasks(ctx, &sqlplugin.ReplicationTasksFilter{
					ShardID:            shardID,
					ExclusiveMaxTaskID: 200,
					PageSize:           1000,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			expectedError: nil,
		},
		{
			name: "unknown task category error",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategory{},
			},
			setupMock:     func(mockDB *sqlplugin.MockDB) {},
			expectedError: &types.BadRequestError{},
		},
		{
			name: "database error on timer task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTimer,
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0).Add(time.Minute), 0),
				PageSize:            1000,
			},
			setupMock: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteFromTimerTasks(ctx, &sqlplugin.TimerTasksFilter{
					ShardID:                shardID,
					MinVisibilityTimestamp: time.Unix(0, 0),
					MaxVisibilityTimestamp: time.Unix(0, 0).Add(time.Minute),
					PageSize:               1000,
				}).Return(nil, errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "database error on transfer task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTransfer,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            1000,
			},
			setupMock: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteFromTransferTasks(ctx, &sqlplugin.TransferTasksFilter{
					ShardID:            shardID,
					InclusiveMinTaskID: 100,
					ExclusiveMaxTaskID: 200,
					PageSize:           1000,
				}).Return(nil, errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "database error on replication task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryReplication,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            1000,
			},
			setupMock: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteFromReplicationTasks(ctx, &sqlplugin.ReplicationTasksFilter{
					ShardID:            shardID,
					ExclusiveMaxTaskID: 200,
					PageSize:           1000,
				}).Return(nil, errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "sql result error on timer task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTimer,
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0).Add(time.Minute), 0),
				PageSize:            1000,
			},
			setupMock: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteFromTimerTasks(ctx, &sqlplugin.TimerTasksFilter{
					ShardID:                shardID,
					MinVisibilityTimestamp: time.Unix(0, 0),
					MaxVisibilityTimestamp: time.Unix(0, 0).Add(time.Minute),
					PageSize:               1000,
				}).Return(&sqlResult{err: errors.New("sql result error")}, nil)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("sql result error"),
		},
		{
			name: "sql result error on transfer task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTransfer,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            1000,
			},
			setupMock: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteFromTransferTasks(ctx, &sqlplugin.TransferTasksFilter{
					ShardID:            shardID,
					InclusiveMinTaskID: 100,
					ExclusiveMaxTaskID: 200,
					PageSize:           1000,
				}).Return(&sqlResult{err: errors.New("sql result error")}, nil)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("sql result error"),
		},
		{
			name: "sql result error on replication task",
			request: &persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryReplication,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            1000,
			},
			setupMock: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().RangeDeleteFromReplicationTasks(ctx, &sqlplugin.ReplicationTasksFilter{
					ShardID:            shardID,
					ExclusiveMaxTaskID: 200,
					PageSize:           1000,
				}).Return(&sqlResult{err: errors.New("sql result error")}, nil)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("sql result error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			defer controller.Finish()

			mockDB := sqlplugin.NewMockDB(controller)
			store := &sqlExecutionStore{sqlStore: sqlStore{db: mockDB}, shardID: shardID}

			tc.setupMock(mockDB)

			resp, err := store.RangeCompleteHistoryTask(ctx, tc.request)
			if tc.expectedError != nil {
				require.ErrorAs(t, err, &tc.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, resp.TasksCompleted)
			}
		})
	}
}

func TestGetHistoryTasks_SQL(t *testing.T) {
	ctx := context.Background()
	shardID := 1

	tests := []struct {
		name                  string
		request               *persistence.GetHistoryTasksRequest
		setupMock             func(*sqlplugin.MockDB, *serialization.MockTaskSerializer)
		expectedError         error
		expectedTasks         []persistence.Task
		expectedNextPageToken []byte
	}{
		{
			name: "success - get immediate transfer tasks",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTransfer,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            10,
				NextPageToken:       serializePageToken(101),
			},
			setupMock: func(mockDB *sqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectFromTransferTasks(ctx, &sqlplugin.TransferTasksFilter{
					ShardID:            shardID,
					InclusiveMinTaskID: 101,
					ExclusiveMaxTaskID: 200,
					PageSize:           10,
				}).Return([]sqlplugin.TransferTasksRow{
					{
						ShardID:      shardID,
						TaskID:       101,
						Data:         []byte(`{"task": "transfer"}`),
						DataEncoding: "json",
					},
				}, nil)
				mockTaskSerializer.EXPECT().DeserializeTask(persistence.HistoryTaskCategoryTransfer, persistence.NewDataBlob([]byte(`{"task": "transfer"}`), constants.EncodingTypeJSON)).Return(&persistence.DecisionTask{
					TaskList: "test-task-list",
				}, nil)
			},
			expectedError: nil,
			expectedTasks: []persistence.Task{
				&persistence.DecisionTask{
					TaskList: "test-task-list",
					TaskData: persistence.TaskData{
						TaskID: 101,
					},
				},
			},
			expectedNextPageToken: serializePageToken(102),
		},
		{
			name: "success - get scheduled timer tasks",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory:        persistence.HistoryTaskCategoryTimer,
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0).UTC(), 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0).Add(time.Minute).UTC(), 0),
				PageSize:            1,
				NextPageToken: func() []byte {
					ti := &timerTaskPageToken{TaskID: 10, Timestamp: time.Unix(0, 1).UTC()}
					token, err := ti.serialize()
					require.NoError(t, err, "failed to serialize timer page token")
					return token
				}(),
			},
			setupMock: func(mockDB *sqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectFromTimerTasks(ctx, &sqlplugin.TimerTasksFilter{
					ShardID:                shardID,
					MinVisibilityTimestamp: time.Unix(0, 1).UTC(),
					TaskID:                 10,
					MaxVisibilityTimestamp: time.Unix(0, 0).Add(time.Minute).UTC(),
					PageSize:               2,
				}).Return([]sqlplugin.TimerTasksRow{
					{
						ShardID:             shardID,
						TaskID:              10,
						VisibilityTimestamp: time.Unix(1, 1),
						Data:                []byte(`{"task": "timer"}`),
						DataEncoding:        "json",
					},
					{
						ShardID:             shardID,
						TaskID:              101,
						VisibilityTimestamp: time.Unix(1, 1),
						Data:                []byte(`{"task": "timer"}`),
						DataEncoding:        "json",
					},
				}, nil)
				mockTaskSerializer.EXPECT().DeserializeTask(persistence.HistoryTaskCategoryTimer, persistence.NewDataBlob([]byte(`{"task": "timer"}`), constants.EncodingTypeJSON)).Return(&persistence.UserTimerTask{
					EventID: 100,
				}, nil)
				mockTaskSerializer.EXPECT().DeserializeTask(persistence.HistoryTaskCategoryTimer, persistence.NewDataBlob([]byte(`{"task": "timer"}`), constants.EncodingTypeJSON)).Return(&persistence.UserTimerTask{
					EventID: 101,
				}, nil)
			},
			expectedError: nil,
			expectedTasks: []persistence.Task{
				&persistence.UserTimerTask{
					EventID: 100,
					TaskData: persistence.TaskData{
						TaskID:              10,
						VisibilityTimestamp: time.Unix(1, 1),
					},
				},
			},
			expectedNextPageToken: func() []byte {
				ti := &timerTaskPageToken{TaskID: 101, Timestamp: time.Unix(1, 1)}
				token, err := ti.serialize()
				require.NoError(t, err, "failed to serialize timer page token")
				return token
			}(),
		},
		{
			name: "success - get immediate replication tasks",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory:        persistence.HistoryTaskCategoryReplication,
				InclusiveMinTaskKey: persistence.NewImmediateTaskKey(100),
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(200),
				PageSize:            10,
				NextPageToken:       serializePageToken(101),
			},
			setupMock: func(mockDB *sqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectFromReplicationTasks(ctx, &sqlplugin.ReplicationTasksFilter{
					ShardID:            shardID,
					InclusiveMinTaskID: 101,
					ExclusiveMaxTaskID: 200,
					PageSize:           10,
				}).Return([]sqlplugin.ReplicationTasksRow{
					{
						ShardID:      shardID,
						TaskID:       101,
						Data:         []byte(`{"task": "replication"}`),
						DataEncoding: "json",
					},
				}, nil)
				mockTaskSerializer.EXPECT().DeserializeTask(persistence.HistoryTaskCategoryReplication, persistence.NewDataBlob([]byte(`{"task": "replication"}`), constants.EncodingTypeJSON)).Return(&persistence.HistoryReplicationTask{
					FirstEventID: 100,
					NextEventID:  200,
				}, nil)
			},
			expectedError: nil,
			expectedTasks: []persistence.Task{
				&persistence.HistoryReplicationTask{
					FirstEventID: 100,
					NextEventID:  200,
					TaskData: persistence.TaskData{
						TaskID: 101,
					},
				},
			},
			expectedNextPageToken: serializePageToken(102),
		},
		{
			name: "database error on transfer task retrieval",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory: persistence.HistoryTaskCategoryTransfer,
				PageSize:     10,
			},
			setupMock: func(mockDB *sqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectFromTransferTasks(ctx, gomock.Any()).Return(nil, errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "database error on replication task retrieval",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory: persistence.HistoryTaskCategoryReplication,
				PageSize:     10,
			},
			setupMock: func(mockDB *sqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectFromReplicationTasks(ctx, gomock.Any()).Return(nil, errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "database error on timer task retrieval",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory: persistence.HistoryTaskCategoryTimer,
				PageSize:     10,
			},
			setupMock: func(mockDB *sqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {
				mockDB.EXPECT().SelectFromTimerTasks(ctx, gomock.Any()).Return(nil, errors.New("db error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "unknown task category error",
			request: &persistence.GetHistoryTasksRequest{
				TaskCategory: persistence.HistoryTaskCategory{},
			},
			setupMock:     func(mockDB *sqlplugin.MockDB, mockTaskSerializer *serialization.MockTaskSerializer) {},
			expectedError: &types.BadRequestError{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			defer controller.Finish()

			mockDB := sqlplugin.NewMockDB(controller)
			mockTaskSerializer := serialization.NewMockTaskSerializer(controller)
			store := &sqlExecutionStore{sqlStore: sqlStore{db: mockDB}, shardID: shardID, taskSerializer: mockTaskSerializer}

			tc.setupMock(mockDB, mockTaskSerializer)

			resp, err := store.GetHistoryTasks(ctx, tc.request)
			if tc.expectedError != nil {
				require.ErrorAs(t, err, &tc.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedTasks, resp.Tasks)
				assert.Equal(t, tc.expectedNextPageToken, resp.NextPageToken)
			}
		})
	}
}

func TestCompleteHistoryTask(t *testing.T) {
	ctx := context.Background()
	shardID := 1

	tests := []struct {
		name          string
		request       *persistence.CompleteHistoryTaskRequest
		setupMock     func(any)
		expectedError error
	}{
		{
			name: "success - complete scheduled timer task",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryTimer,
				TaskKey:      persistence.NewHistoryTaskKey(time.Unix(10, 10), 1),
			},
			setupMock: func(mockDB any) {
				mock := mockDB.(*sqlplugin.MockDB)
				mock.EXPECT().DeleteFromTimerTasks(ctx, &sqlplugin.TimerTasksFilter{
					ShardID:             shardID,
					VisibilityTimestamp: time.Unix(10, 10),
					TaskID:              1,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			expectedError: nil,
		},
		{
			name: "success - complete immediate transfer task",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryTransfer,
				TaskKey:      persistence.NewImmediateTaskKey(2),
			},
			setupMock: func(mockDB any) {
				mock := mockDB.(*sqlplugin.MockDB)
				mock.EXPECT().DeleteFromTransferTasks(ctx, &sqlplugin.TransferTasksFilter{
					ShardID: shardID,
					TaskID:  2,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			expectedError: nil,
		},
		{
			name: "success - complete immediate replication task",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryReplication,
				TaskKey:      persistence.NewImmediateTaskKey(3),
			},
			setupMock: func(mockDB any) {
				mock := mockDB.(*sqlplugin.MockDB)
				mock.EXPECT().DeleteFromReplicationTasks(ctx, &sqlplugin.ReplicationTasksFilter{
					ShardID: shardID,
					TaskID:  3,
				}).Return(&sqlResult{rowsAffected: 1}, nil)
			},
			expectedError: nil,
		},
		{
			name: "unknown task category type",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategory{},
			},
			setupMock:     func(mockDB any) {},
			expectedError: &types.BadRequestError{},
		},
		{
			name: "delete timer task error",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryTimer,
				TaskKey:      persistence.NewHistoryTaskKey(time.Unix(10, 10), 1),
			},
			setupMock: func(mockDB any) {
				mock := mockDB.(*sqlplugin.MockDB)
				mock.EXPECT().DeleteFromTimerTasks(ctx, &sqlplugin.TimerTasksFilter{
					ShardID:             shardID,
					VisibilityTimestamp: time.Unix(10, 10),
					TaskID:              1,
				}).Return(&sqlResult{rowsAffected: 0}, errors.New("db error"))
				mock.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "delete transfer task error",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryTransfer,
				TaskKey:      persistence.NewImmediateTaskKey(2),
			},
			setupMock: func(mockDB any) {
				mock := mockDB.(*sqlplugin.MockDB)
				mock.EXPECT().DeleteFromTransferTasks(ctx, &sqlplugin.TransferTasksFilter{
					ShardID: shardID,
					TaskID:  2,
				}).Return(&sqlResult{rowsAffected: 0}, errors.New("db error"))
				mock.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
		{
			name: "delete replication task error",
			request: &persistence.CompleteHistoryTaskRequest{
				TaskCategory: persistence.HistoryTaskCategoryReplication,
				TaskKey:      persistence.NewImmediateTaskKey(3),
			},
			setupMock: func(mockDB any) {
				mock := mockDB.(*sqlplugin.MockDB)
				mock.EXPECT().DeleteFromReplicationTasks(ctx, &sqlplugin.ReplicationTasksFilter{
					ShardID: shardID,
					TaskID:  3,
				}).Return(&sqlResult{rowsAffected: 0}, errors.New("db error"))
				mock.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
			},
			expectedError: errors.New("db error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			defer controller.Finish()

			mockDB := sqlplugin.NewMockDB(controller)
			store := &sqlExecutionStore{sqlStore: sqlStore{db: mockDB}, shardID: shardID}

			tc.setupMock(mockDB)

			err := store.CompleteHistoryTask(ctx, tc.request)
			if tc.expectedError != nil {
				require.ErrorAs(t, err, &tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
