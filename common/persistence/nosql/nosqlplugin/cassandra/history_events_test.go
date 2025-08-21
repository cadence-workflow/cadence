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

package cassandra

import (
	"context"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

func TestInsertIntoHistoryTreeAndNode(t *testing.T) {
	tests := []struct {
		name        string
		treeRow     *nosqlplugin.HistoryTreeRow
		nodeRow     *nosqlplugin.HistoryNodeRow
		setupMocks  func(*gomock.Controller, *fakeSession)
		expectError bool
	}{
		{
			name: "Successfully insert both tree and node rows using batch",
			treeRow: &nosqlplugin.HistoryTreeRow{
				TreeID:          "treeID",
				BranchID:        "branchID",
				Ancestors:       []*types.HistoryBranchRange{{BranchID: "branch1", EndNodeID: 100}},
				CreateTimestamp: time.Now(),
			},
			nodeRow: &nosqlplugin.HistoryNodeRow{
				TreeID:       "treeID",
				BranchID:     "branchID",
				NodeID:       1,
				TxnID:        nil,
				Data:         []byte("node data"),
				DataEncoding: "encoding",
			},
			setupMocks: func(ctrl *gomock.Controller, session *fakeSession) {
				mockBatch := gocql.NewMockBatch(ctrl)
				mockQuery := gocql.NewMockQuery(ctrl)
				mockBatch.EXPECT().WithContext(gomock.Any()).Return(mockBatch).AnyTimes()
				mockBatch.EXPECT().Query(v2templateInsertTree, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
				mockBatch.EXPECT().Query(v2templateUpsertData, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()

				session.query = mockQuery
			},
			expectError: false,
		},
		{
			name: "Successfully insert only tree row",
			treeRow: &nosqlplugin.HistoryTreeRow{
				TreeID:          "treeID",
				BranchID:        "branchID",
				Ancestors:       []*types.HistoryBranchRange{{BranchID: "branch1", EndNodeID: 100}},
				CreateTimestamp: time.Now(),
			},
			nodeRow: nil, // No node row provided
			setupMocks: func(ctrl *gomock.Controller, session *fakeSession) {
				mockQuery := gocql.NewMockQuery(ctrl)
				mockQuery.EXPECT().WithContext(gomock.Any()).Return(mockQuery)
				mockQuery.EXPECT().Exec().Return(nil)

				session.query = mockQuery
			},
			expectError: false,
		},
		{
			name:    "Successfully insert only node row",
			treeRow: nil, // No tree row provided
			nodeRow: &nosqlplugin.HistoryNodeRow{
				TreeID:       "treeID",
				BranchID:     "branchID",
				NodeID:       1,
				TxnID:        nil, // Optional field
				Data:         []byte("node data"),
				DataEncoding: "encoding",
			},
			setupMocks: func(ctrl *gomock.Controller, session *fakeSession) {
				mockQuery := gocql.NewMockQuery(ctrl)
				mockQuery.EXPECT().WithContext(gomock.Any()).Return(mockQuery)
				mockQuery.EXPECT().Exec().Return(nil)

				session.query = mockQuery
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			session := &fakeSession{}
			if tt.setupMocks != nil {
				tt.setupMocks(ctrl, session)
			}

			db := &CDB{session: session}
			err := db.InsertIntoHistoryTreeAndNode(context.Background(), tt.treeRow, tt.nodeRow)
			if tt.expectError {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Did not expect an error but got one")
			}
		})
	}
}

func TestSelectFromHistoryNode(t *testing.T) {
	txnID1 := int64(1)
	txnID2 := int64(2)
	tests := []struct {
		name          string
		filter        *nosqlplugin.HistoryNodeFilter
		setupMocks    func(*gomock.Controller, *fakeSession)
		expectedRows  []*nosqlplugin.HistoryNodeRow
		expectedToken []byte
		expectError   bool
	}{
		{
			name: "Successfully retrieve history nodes",
			filter: &nosqlplugin.HistoryNodeFilter{
				TreeID:        "treeID",
				BranchID:      "branchID",
				MinNodeID:     1,
				MaxNodeID:     10,
				PageSize:      5,
				NextPageToken: nil,
			},
			setupMocks: func(ctrl *gomock.Controller, session *fakeSession) {
				mockQuery := gocql.NewMockQuery(ctrl)
				mockQuery.EXPECT().WithContext(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageSize(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageState(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().Iter().Return(&fakeIter{
					scanInputs: [][]interface{}{
						{int64(1), &txnID1, []byte("data1"), "encoding"},
						{int64(2), &txnID2, []byte("data2"), "encoding"},
					},
					pageState: []byte("nextPageToken"),
				}).AnyTimes()

				session.query = mockQuery
			},
			expectedRows: []*nosqlplugin.HistoryNodeRow{
				{NodeID: int64(1), TxnID: &txnID1, Data: []byte("data1"), DataEncoding: "encoding"},
				{NodeID: int64(2), TxnID: &txnID2, Data: []byte("data2"), DataEncoding: "encoding"},
			},
			expectedToken: []byte("nextPageToken"),
			expectError:   false,
		},
		{
			name: "Failure to create query iterator",
			filter: &nosqlplugin.HistoryNodeFilter{
				TreeID:        "treeID",
				BranchID:      "branchID",
				MinNodeID:     1,
				MaxNodeID:     10,
				PageSize:      5,
				NextPageToken: nil,
			},
			setupMocks: func(ctrl *gomock.Controller, session *fakeSession) {
				mockQuery := gocql.NewMockQuery(ctrl)
				mockQuery.EXPECT().WithContext(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageSize(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageState(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().Iter().Return(nil).AnyTimes() // Simulating failure

				session.query = mockQuery
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			session := &fakeSession{}
			if tt.setupMocks != nil {
				tt.setupMocks(ctrl, session)
			}

			db := &CDB{session: session}
			rows, token, err := db.SelectFromHistoryNode(context.Background(), tt.filter)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRows, rows)
				assert.Equal(t, tt.expectedToken, token)
			}
		})
	}
}

func TestDeleteFromHistoryTreeAndNode(t *testing.T) {
	tests := []struct {
		name        string
		treeFilter  *nosqlplugin.HistoryTreeFilter
		nodeFilters []*nosqlplugin.HistoryNodeFilter
		setupMocks  func(*fakeSession)
		expectError bool
	}{
		{
			name: "Successfully delete tree and nodes",
			treeFilter: &nosqlplugin.HistoryTreeFilter{
				ShardID:  1,
				TreeID:   "treeID",
				BranchID: stringPtr("branchID"),
			},
			nodeFilters: []*nosqlplugin.HistoryNodeFilter{
				{TreeID: "treeID", BranchID: "branchID", MinNodeID: 1},
				{TreeID: "treeID", BranchID: "branchID", MinNodeID: 2},
			},
			setupMocks: func(session *fakeSession) {
				// Simulate successful batch execution
				session.mapExecuteBatchCASApplied = true
			},
			expectError: false,
		},
		{
			name: "Failure in batch execution",
			treeFilter: &nosqlplugin.HistoryTreeFilter{
				ShardID:  1,
				TreeID:   "treeID",
				BranchID: stringPtr("branchID"),
			},
			nodeFilters: []*nosqlplugin.HistoryNodeFilter{
				{TreeID: "treeID", BranchID: "branchID", MinNodeID: 1},
			},
			setupMocks: func(session *fakeSession) {
				// Simulate failure in batch execution
				session.mapExecuteBatchCASErr = types.InternalServiceError{Message: "DB operation failed"}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			session := &fakeSession{}
			if tt.setupMocks != nil {
				tt.setupMocks(session)
			}

			db := &CDB{session: session}
			err := db.DeleteFromHistoryTreeAndNode(context.Background(), tt.treeFilter, tt.nodeFilters)

			if tt.expectError {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Did not expect an error but got one")
			}
		})
	}
}

func TestSelectAllHistoryTrees(t *testing.T) {
	location, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatal(err)
	}
	fixedTime, err := time.ParseInLocation(time.RFC3339, "2023-12-12T22:08:41Z", location)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		nextPageToken []byte
		pageSize      int
		setupMocks    func(*gomock.Controller, *fakeSession)
		expectedRows  []*nosqlplugin.HistoryTreeRow
		expectedToken []byte
		expectError   bool
	}{
		{
			name:          "Successfully retrieve all history trees",
			nextPageToken: []byte("token"),
			pageSize:      10,
			setupMocks: func(ctrl *gomock.Controller, session *fakeSession) {
				mockQuery := gocql.NewMockQuery(ctrl)
				mockQuery.EXPECT().WithContext(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageSize(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageState(gomock.Any()).Return(mockQuery).AnyTimes()

				session.iter = &fakeIter{
					scanInputs: [][]interface{}{
						{"treeID1", "branchID1", fixedTime, "Tree info 1"},
						{"treeID2", "branchID2", fixedTime.Add(time.Minute), "Tree info 2"},
					},
					pageState: []byte("nextPageToken"),
				}
				mockQuery.EXPECT().Iter().Return(session.iter).AnyTimes()
				session.query = mockQuery
			},
			expectedRows: []*nosqlplugin.HistoryTreeRow{
				{TreeID: "treeID1", BranchID: "branchID1", CreateTimestamp: fixedTime, Info: "Tree info 1"},
				{TreeID: "treeID2", BranchID: "branchID2", CreateTimestamp: fixedTime.Add(time.Minute), Info: "Tree info 2"},
			},
			expectedToken: []byte("nextPageToken"),
			expectError:   false,
		},
		{
			name:          "Failed to create query iterator",
			nextPageToken: []byte("token"),
			pageSize:      10,
			setupMocks: func(ctrl *gomock.Controller, session *fakeSession) {
				mockQuery := gocql.NewMockQuery(ctrl)
				mockQuery.EXPECT().WithContext(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageSize(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageState(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().Iter().Return(nil).AnyTimes() // Simulate failure to create iterator
				session.query = mockQuery
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			session := &fakeSession{}
			if tt.setupMocks != nil {
				tt.setupMocks(ctrl, session)
			}

			db := &CDB{session: session}
			rows, token, err := db.SelectAllHistoryTrees(context.Background(), tt.nextPageToken, tt.pageSize)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for i, row := range rows {
					assertHistoryTreeRowEqual(t, tt.expectedRows[i], row)
				}
				assert.Equal(t, tt.expectedToken, token)
			}
		})
	}
}

func TestSelectFromHistoryTree(t *testing.T) {
	tests := []struct {
		name         string
		filter       *nosqlplugin.HistoryTreeFilter
		setupMocks   func(*gomock.Controller, *fakeSession)
		expectedRows []*nosqlplugin.HistoryTreeRow
		expectError  bool
	}{
		{
			name: "Successfully retrieve branches",
			filter: &nosqlplugin.HistoryTreeFilter{
				TreeID: "treeID",
			},
			setupMocks: func(ctrl *gomock.Controller, session *fakeSession) {
				mockQuery := gocql.NewMockQuery(ctrl)
				mockQuery.EXPECT().WithContext(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageSize(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageState(gomock.Any()).Return(mockQuery).AnyTimes()

				iter1 := newFakeIter([][]interface{}{
					{"branchUUID1", []map[string]interface{}{{"branch_id": uuid.Parse(permanentRunID), "end_node_id": int64(10)}}, time.Now(), "Info1"},
				}, []byte("nextPageToken"))

				iter2 := newFakeIter([][]interface{}{
					{"branchUUID2", []map[string]interface{}{{"branch_id": uuid.Parse(permanentRunID), "end_node_id": int64(20)}}, time.Now(), "Info2"},
				}, nil) // No more pages

				gomock.InOrder(
					mockQuery.EXPECT().Iter().Return(iter1),
					mockQuery.EXPECT().Iter().Return(iter2),
				)

				session.query = mockQuery
			},
			expectedRows: []*nosqlplugin.HistoryTreeRow{
				{TreeID: "treeID", BranchID: "branchUUID1", Ancestors: []*types.HistoryBranchRange{{BranchID: permanentRunID, EndNodeID: 10, BeginNodeID: 1}}, Info: ""},
				{TreeID: "treeID", BranchID: "branchUUID2", Ancestors: []*types.HistoryBranchRange{{BranchID: permanentRunID, EndNodeID: 20, BeginNodeID: 1}}, Info: ""},
			},
			expectError: false,
		},
		{
			name: "Failed to create query iterator",
			filter: &nosqlplugin.HistoryTreeFilter{
				TreeID: "treeID",
			},
			setupMocks: func(ctrl *gomock.Controller, session *fakeSession) {
				mockQuery := gocql.NewMockQuery(ctrl)
				mockQuery.EXPECT().WithContext(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageSize(gomock.Any()).Return(mockQuery).AnyTimes()
				mockQuery.EXPECT().PageState(gomock.Any()).Return(mockQuery).AnyTimes()

				mockQuery.EXPECT().Iter().Return(nil) // Simulate failure to create iterator

				session.query = mockQuery
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			session := &fakeSession{}
			if tt.setupMocks != nil {
				tt.setupMocks(ctrl, session)
			}

			db := &CDB{session: session}
			rows, err := db.SelectFromHistoryTree(context.Background(), tt.filter)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRows, rows)
			}
		})
	}
}

// Helper to create a fake iterator with predefined results
func newFakeIter(data [][]interface{}, nextPageToken []byte) *fakeIter {
	return &fakeIter{
		scanInputs: data,
		pageState:  nextPageToken,
	}
}

func assertHistoryTreeRowEqual(t *testing.T, expected, actual *nosqlplugin.HistoryTreeRow) {
	assert.Equal(t, expected.TreeID, actual.TreeID)
	assert.Equal(t, expected.BranchID, actual.BranchID)
	assert.Equal(t, expected.Info, actual.Info)
	assert.True(t, expected.CreateTimestamp.Equal(actual.CreateTimestamp), "Timestamps do not match")
}

func stringPtr(s string) *string {
	return &s
}
