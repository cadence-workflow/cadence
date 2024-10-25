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
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

const (
	testDomainID         = "test-domain-id"
	testDomainName       = "test-domain"
	testTaskListName     = "test-tasklist"
	testWorkflowID       = "test-workflow-id"
	testRunID            = "test-run-id"
	testWorkflowTypeName = "test-workflow-type"
)

func TestNewNoSQLVisibilityStore(t *testing.T) {
	cfg := getValidShardedNoSQLConfig()

	store, err := newNoSQLVisibilityStore(false, cfg, log.NewNoop(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func setupNoSQLVisibilityStoreMocks(t *testing.T) (*nosqlVisibilityStore, *nosqlplugin.MockDB) {
	ctrl := gomock.NewController(t)
	dbMock := nosqlplugin.NewMockDB(ctrl)

	nosqlSt := nosqlStore{
		logger: log.NewNoop(),
		db:     dbMock,
	}

	shardedNosqlStoreMock := NewMockshardedNosqlStore(ctrl)
	shardedNosqlStoreMock.EXPECT().
		GetStoreShardByTaskList(
			TestDomainID,
			TestTaskListName,
			TestTaskType).
		Return(&nosqlSt, nil).
		AnyTimes()
	shardedNosqlStoreMock.EXPECT().GetDefaultShard().Return(nosqlStore{db: dbMock}).AnyTimes()
	visibilityStore := &nosqlVisibilityStore{
		nosqlStore:      shardedNosqlStoreMock.GetDefaultShard(),
		sortByCloseTime: false,
	}
	return visibilityStore, dbMock
}

func TestRecordWorkflowExecutionStarted(t *testing.T) {
	visibilityStore, db := setupNoSQLVisibilityStoreMocks(t)

	db.EXPECT().InsertVisibility(gomock.Any(), gomock.Any(), gomock.Any()).Return(assert.AnError)
	// The error _is_ a NotFoundError
	db.EXPECT().IsNotFoundError(assert.AnError).Return(true)

	err := visibilityStore.RecordWorkflowExecutionStarted(context.Background(), &persistence.InternalRecordWorkflowExecutionStartedRequest{
		DomainUUID:         testDomainID,
		WorkflowID:         testWorkflowID,
		RunID:              testRunID,
		WorkflowTypeName:   testWorkflowTypeName,
		WorkflowTimeout:    20 * 60,
		StartTimestamp:     time.Time{},
		ExecutionTimestamp: time.Time{},
		Memo:               nil,
		TaskList:           testTaskListName,
		IsCron:             false,
		NumClusters:        2,
		UpdateTimestamp:    time.Time{},
		ShardID:            2,
	})

	assert.Error(t, err)
	assert.ErrorContains(t, err, "RecordWorkflowExecutionStarted failed. Error:")
}

func TestRecordWorkflowExecutionClosed(t *testing.T) {
	visibilityStore, db := setupNoSQLVisibilityStoreMocks(t)

	db.EXPECT().UpdateVisibility(gomock.Any(), gomock.Any(), gomock.Any()).Return(assert.AnError)
	// The error _is_ a NotFoundError
	db.EXPECT().IsNotFoundError(assert.AnError).Return(true)

	err := visibilityStore.RecordWorkflowExecutionClosed(context.Background(), &persistence.InternalRecordWorkflowExecutionClosedRequest{
		DomainUUID:         testDomainID,
		WorkflowID:         testWorkflowID,
		RunID:              testRunID,
		WorkflowTypeName:   testWorkflowTypeName,
		StartTimestamp:     time.Time{},
		ExecutionTimestamp: time.Time{},
		Memo:               nil,
		TaskList:           testTaskListName,
		IsCron:             false,
		NumClusters:        2,
		UpdateTimestamp:    time.Time{},
		ShardID:            2,
	})

	assert.Error(t, err)
	assert.ErrorContains(t, err, "RecordWorkflowExecutionClosed failed. Error:")
}

func TestRecordWorkflowExecutionUninitialized(t *testing.T) {
	visibilityStore, _ := setupNoSQLVisibilityStoreMocks(t)
	err := visibilityStore.RecordWorkflowExecutionUninitialized(context.Background(), &persistence.InternalRecordWorkflowExecutionUninitializedRequest{})
	assert.NoError(t, err)
}

func TestUpsertWorkflowExecution(t *testing.T) {
	visibilityStore, _ := setupNoSQLVisibilityStoreMocks(t)

	err := visibilityStore.UpsertWorkflowExecution(context.Background(), &persistence.InternalUpsertWorkflowExecutionRequest{
		SearchAttributes: map[string][]byte{
			definition.CadenceChangeVersion: nil,
		},
	})

	assert.NoError(t, err)

	err = visibilityStore.UpsertWorkflowExecution(context.Background(), &persistence.InternalUpsertWorkflowExecutionRequest{})

	assert.Error(t, err)
	assert.Equal(t, persistence.ErrVisibilityOperationNotSupported, err)
}

func TestListOpenWorkflowExecutions_Success(t *testing.T) {
	visibilityStore, db := setupNoSQLVisibilityStoreMocks(t)

	db.EXPECT().SelectVisibility(gomock.Any(), gomock.Any()).Return(&nosqlplugin.SelectVisibilityResponse{
		Executions: []*nosqlplugin.VisibilityRow{{
			DomainID:         testDomainID,
			WorkflowType:     testWorkflowTypeName,
			WorkflowID:       testWorkflowID,
			RunID:            testRunID,
			TypeName:         "test-name",
			StartTime:        time.Time{},
			ExecutionTime:    time.Time{},
			CloseTime:        time.Time{},
			Status:           nil,
			HistoryLength:    2,
			Memo:             nil,
			TaskList:         testTaskListName,
			IsCron:           false,
			NumClusters:      0,
			UpdateTime:       time.Time{},
			SearchAttributes: nil,
			ShardID:          2,
		}},
		NextPageToken: nil,
	}, nil)

	response, err := visibilityStore.ListOpenWorkflowExecutions(context.Background(), &persistence.InternalListWorkflowExecutionsRequest{
		DomainUUID:    testDomainID,
		Domain:        testDomainName,
		EarliestTime:  time.Time{},
		LatestTime:    time.Time{},
		PageSize:      2,
		NextPageToken: nil,
	})

	assert.NoError(t, err)
	assert.Equal(t, len(response.Executions), 1)
}

func TestListOpenWorkflowExecutions_Failed(t *testing.T) {
	visibilityStore, db := setupNoSQLVisibilityStoreMocks(t)

	db.EXPECT().SelectVisibility(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
	// The error _is_ a NotFoundError
	db.EXPECT().IsNotFoundError(assert.AnError).Return(true)
	_, err := visibilityStore.ListOpenWorkflowExecutions(context.Background(), &persistence.InternalListWorkflowExecutionsRequest{
		DomainUUID:    testDomainID,
		Domain:        testDomainName,
		EarliestTime:  time.Time{},
		LatestTime:    time.Time{},
		PageSize:      2,
		NextPageToken: nil,
	})

	assert.Error(t, err)
	assert.ErrorContains(t, err, "ListOpenWorkflowExecutions failed. Error:")
}

func TestDeleteUninitializedWorkflowExecution(t *testing.T) {
	visibilityStore, _ := setupNoSQLVisibilityStoreMocks(t)
	err := visibilityStore.DeleteUninitializedWorkflowExecution(context.Background(), &persistence.VisibilityDeleteWorkflowExecutionRequest{})
	assert.NoError(t, err)
}

func TestListWorkflowExecutions(t *testing.T) {
	visibilityStore, _ := setupNoSQLVisibilityStoreMocks(t)
	_, err := visibilityStore.ListWorkflowExecutions(context.Background(), &persistence.ListWorkflowExecutionsByQueryRequest{})
	assert.Error(t, err)
	assert.Equal(t, persistence.ErrVisibilityOperationNotSupported, err)
}

func TestScanWorkflowExecutions(t *testing.T) {
	visibilityStore, _ := setupNoSQLVisibilityStoreMocks(t)
	_, err := visibilityStore.ScanWorkflowExecutions(context.Background(), &persistence.ListWorkflowExecutionsByQueryRequest{})
	assert.Error(t, err)
	assert.Equal(t, persistence.ErrVisibilityOperationNotSupported, err)
}

func TestCountWorkflowExecutions(t *testing.T) {
	visibilityStore, _ := setupNoSQLVisibilityStoreMocks(t)
	_, err := visibilityStore.CountWorkflowExecutions(context.Background(), &persistence.CountWorkflowExecutionsRequest{})
	assert.Error(t, err)
	assert.Equal(t, persistence.ErrVisibilityOperationNotSupported, err)
}
