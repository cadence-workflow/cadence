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
	ctx "context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

const (
	testTransactionID = 123
	testShardID       = 456
	testNodeID        = 8
)

func validInternalAppendHistoryNodesRequest() *persistence.InternalAppendHistoryNodesRequest {
	return &persistence.InternalAppendHistoryNodesRequest{
		IsNewBranch: false,
		Info:        "TestInfo",
		BranchInfo: types.HistoryBranch{
			TreeID:   "TestTreeID",
			BranchID: "TestBranchID",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "TestAncestorBranchID",
					BeginNodeID: 0,
					EndNodeID:   5,
				},
			},
		},
		NodeID: testNodeID,
		Events: &persistence.DataBlob{
			Encoding: constants.EncodingTypeThriftRW,
			Data:     []byte("TestEvents"),
		},
		TransactionID:    testTransactionID,
		ShardID:          testShardID,
		CurrentTimeStamp: FixedTime,
	}
}

func validHistoryNodeRow() *nosqlplugin.HistoryNodeRow {
	expectedNodeRow := &nosqlplugin.HistoryNodeRow{
		TreeID:          "TestTreeID",
		BranchID:        "TestBranchID",
		NodeID:          testNodeID,
		TxnID:           common.Ptr[int64](123),
		Data:            []byte("TestEvents"),
		DataEncoding:    string(constants.EncodingTypeThriftRW),
		ShardID:         testShardID,
		CreateTimestamp: FixedTime,
	}
	return expectedNodeRow
}

func registerCassandraMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := nosqlplugin.NewMockDB(ctrl)

	mockPlugin := nosqlplugin.NewMockPlugin(ctrl)
	mockPlugin.EXPECT().CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockDB, nil).AnyTimes()
	RegisterPlugin("cassandra", mockPlugin)
}

func TestNewNoSQLHistoryStore(t *testing.T) {
	registerCassandraMock(t)
	cfg := getValidShardedNoSQLConfig()

	store, err := newNoSQLHistoryStore(cfg, log.NewNoop(), metrics.NewNoopMetricsClient(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func setUpMocks(t *testing.T) (*nosqlHistoryStore, *nosqlplugin.MockDB, *MockshardedNosqlStore) {
	ctrl := gomock.NewController(t)
	dbMock := nosqlplugin.NewMockDB(ctrl)

	nosqlSt := nosqlStore{
		logger: log.NewNoop(),
		db:     dbMock,
	}

	shardedNosqlStoreMock := NewMockshardedNosqlStore(ctrl)
	shardedNosqlStoreMock.EXPECT().GetStoreShardByHistoryShard(testShardID).Return(&nosqlSt, nil).AnyTimes()
	shardedNosqlStoreMock.EXPECT().GetLogger().Return(log.NewNoop()).AnyTimes()

	store := &nosqlHistoryStore{
		shardedNosqlStore: shardedNosqlStoreMock,
	}

	return store, dbMock, shardedNosqlStoreMock
}

func TestAppendHistoryNodes_ErrorIfAppendAbove(t *testing.T) {
	store, _, _ := setUpMocks(t)

	request := validInternalAppendHistoryNodesRequest()

	// If the nodeID to append is smaller than the last ancestor's end node ID, return an error
	request.NodeID = 3
	ans := request.BranchInfo.Ancestors
	ans[len(ans)-1].EndNodeID = 5

	err := store.AppendHistoryNodes(ctx.Background(), request)

	var invalidErr *persistence.InvalidPersistenceRequestError
	assert.ErrorAs(t, err, &invalidErr)
	assert.ErrorContains(t, err, "cannot append to ancestors' nodes")
}

func TestAppendHistoryNodes_NotNewBranch(t *testing.T) {
	store, dbMock, _ := setUpMocks(t)

	// Expect to insert the node into the history tree and node, as this is not a new branch, expect treeRow to be nil
	dbMock.EXPECT().InsertIntoHistoryTreeAndNode(gomock.Any(), nil, validHistoryNodeRow()).Return(nil).Times(1)

	request := validInternalAppendHistoryNodesRequest()
	err := store.AppendHistoryNodes(ctx.Background(), request)

	assert.NoError(t, err)
}

func TestAppendHistoryNodes_NewBranch(t *testing.T) {
	request := validInternalAppendHistoryNodesRequest()
	request.IsNewBranch = true

	store, dbMock, _ := setUpMocks(t)

	// Expect to insert the node into the history tree and node, as this is a new branch expect treeRow to be set
	dbMock.EXPECT().InsertIntoHistoryTreeAndNode(gomock.Any(), gomock.Any(), validHistoryNodeRow()).
		DoAndReturn(func(ctx ctx.Context, treeRow *nosqlplugin.HistoryTreeRow, nodeRow *nosqlplugin.HistoryNodeRow) error {
			// Assert that the treeRow is as expected
			assert.Equal(t, testShardID, treeRow.ShardID)
			assert.Equal(t, "TestTreeID", treeRow.TreeID)
			assert.Equal(t, "TestBranchID", treeRow.BranchID)
			assert.Equal(t, request.BranchInfo.Ancestors, treeRow.Ancestors)
			assert.Equal(t, request.Info, treeRow.Info)
			assert.Equal(t, FixedTime, treeRow.CreateTimestamp)

			return nil
		})

	err := store.AppendHistoryNodes(ctx.Background(), request)

	assert.NoError(t, err)
}

const (
	testMinNodeID         = 111
	testMaxNodeID         = 222
	testRequestLastNodeID = 333
	testLastTransactionID = 444

	// These needs to be greater than testRequestLastNodeID
	testRowNodeID1 = int64(334)
	testRowNodeID2 = int64(335)

	// These needs to be greater than testLastTransactionID
	testRowTxnID1 = int64(445)
	testRowTxnID2 = int64(446)
)

func validInternalReadHistoryBranchRequest() *persistence.InternalReadHistoryBranchRequest {
	return &persistence.InternalReadHistoryBranchRequest{
		TreeID:            "TestTreeID",
		BranchID:          "TestBranchID",
		MinNodeID:         testMinNodeID,
		MaxNodeID:         testMaxNodeID,
		PageSize:          0,
		NextPageToken:     nil,
		LastNodeID:        testRequestLastNodeID,
		LastTransactionID: testLastTransactionID,
		ShardID:           testShardID,
	}
}

func expectedHistoryNodeFilter() *nosqlplugin.HistoryNodeFilter {
	return &nosqlplugin.HistoryNodeFilter{
		ShardID:       testShardID,
		TreeID:        "TestTreeID",
		BranchID:      "TestBranchID",
		MinNodeID:     testMinNodeID,
		MaxNodeID:     testMaxNodeID,
		NextPageToken: nil,
		PageSize:      0,
	}
}

func validHistoryNodeRows() []*nosqlplugin.HistoryNodeRow {
	return []*nosqlplugin.HistoryNodeRow{
		{
			TreeID:       "TestTreeID",
			BranchID:     "TestBranchID",
			NodeID:       testRowNodeID1,
			TxnID:        common.Ptr(testRowTxnID1),
			Data:         []byte("TestEvents"),
			DataEncoding: string(constants.EncodingTypeThriftRW),
			ShardID:      testShardID,
		},
		{
			TreeID:       "TestTreeID",
			BranchID:     "TestBranchID",
			NodeID:       testRowNodeID2,
			TxnID:        common.Ptr(testRowTxnID2),
			Data:         []byte("TestEvents2"),
			DataEncoding: string(constants.EncodingTypeThriftRW),
			ShardID:      testShardID,
		},
	}
}

func TestReadHistoryBranch(t *testing.T) {
	store, dbMock, _ := setUpMocks(t)

	request := validInternalReadHistoryBranchRequest()
	rows := validHistoryNodeRows()
	// Append a rowID with a lower transaction ID to test that it is discarded
	badRow := *rows[0]
	badRow.TxnID = common.Ptr[int64](testLastTransactionID - 1)
	rows = append(rows, &badRow)

	// Expect to read the history branch
	dbMock.EXPECT().SelectFromHistoryNode(gomock.Any(), expectedHistoryNodeFilter()).
		Return(rows, nil, nil).Times(1)

	resp, err := store.ReadHistoryBranch(ctx.Background(), request)
	require.NoError(t, err)

	// Asset that we got the history for all the nodes
	assert.Equal(t, 2, len(resp.History))
	assert.Equal(t, rows[0].Data, resp.History[0].Data)
	assert.Equal(t, rows[1].Data, resp.History[1].Data)
	assert.Equal(t, constants.EncodingTypeThriftRW, resp.History[0].Encoding)
	assert.Equal(t, constants.EncodingTypeThriftRW, resp.History[1].Encoding)

	assert.Nil(t, resp.NextPageToken)

	// Assert that these ids corresponds to the last node and transaction id
	assert.Equal(t, testRowNodeID2, resp.LastNodeID)
	assert.Equal(t, testRowTxnID2, resp.LastTransactionID)
}

func TestReadHistoryBranch_ErrorIfSelectFromHistoryNodeErrors(t *testing.T) {
	store, dbMock, _ := setUpMocks(t)

	request := validInternalReadHistoryBranchRequest()

	testError := fmt.Errorf("test error")

	dbMock.EXPECT().SelectFromHistoryNode(gomock.Any(), expectedHistoryNodeFilter()).
		Return(nil, nil, testError).Times(1)
	dbMock.EXPECT().IsNotFoundError(testError).Return(true).Times(1)

	_, err := store.ReadHistoryBranch(ctx.Background(), request)

	var notExistsErr *types.EntityNotExistsError
	assert.ErrorAs(t, err, &notExistsErr)
	assert.ErrorContains(t, err, "SelectFromHistoryNode")
	assert.ErrorContains(t, err, "test error")
}

func TestReadHistoryBranch_ErrorIfDecreasingNodeID(t *testing.T) {
	store, dbMock, _ := setUpMocks(t)

	request := validInternalReadHistoryBranchRequest()
	rows := validHistoryNodeRows()
	// Set the first row to have a node id that is less than the last node id in the request
	rows[0].NodeID = 1

	// Expect to read the history branch
	dbMock.EXPECT().SelectFromHistoryNode(gomock.Any(), expectedHistoryNodeFilter()).
		Return(rows, nil, nil).Times(1)

	_, err := store.ReadHistoryBranch(ctx.Background(), request)

	var dataError *types.InternalDataInconsistencyError
	assert.ErrorAs(t, err, &dataError)
	assert.ErrorContains(t, err, "corrupted data, nodeID cannot decrease")
}

func TestReadHistoryBranch_ErrorIfSameNodeID(t *testing.T) {
	store, dbMock, _ := setUpMocks(t)

	request := validInternalReadHistoryBranchRequest()
	rows := validHistoryNodeRows()
	// Set the second row to have the same node id as the first row
	rows[1].NodeID = rows[0].NodeID

	// Expect to read the history branch
	dbMock.EXPECT().SelectFromHistoryNode(gomock.Any(), expectedHistoryNodeFilter()).
		Return(rows, nil, nil).Times(1)

	_, err := store.ReadHistoryBranch(ctx.Background(), request)

	var dataError *types.InternalDataInconsistencyError
	assert.ErrorAs(t, err, &dataError)
	assert.ErrorContains(t, err, "corrupted data, same nodeID must have smaller txnID")
}

func validInternalForkHistoryBranchRequest(forkNodeID int64) *persistence.InternalForkHistoryBranchRequest {
	return &persistence.InternalForkHistoryBranchRequest{
		ForkBranchInfo: types.HistoryBranch{
			TreeID:   "TestTreeID",
			BranchID: "TestBranchID",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "TestAncestorBranchID",
					BeginNodeID: 0,
					EndNodeID:   5,
				},
				{
					BranchID:    "TestAncestorBranchID",
					BeginNodeID: 6,
					EndNodeID:   10,
				},
			},
		},
		ForkNodeID:       forkNodeID,
		NewBranchID:      "TestNewBranchID",
		Info:             "TestInfo",
		ShardID:          testShardID,
		CurrentTimeStamp: FixedTime,
	}
}

func expectedInternalForkHistoryBranchResponse() *persistence.InternalForkHistoryBranchResponse {
	return &persistence.InternalForkHistoryBranchResponse{
		NewBranchInfo: types.HistoryBranch{
			TreeID:   "TestTreeID",
			BranchID: "TestNewBranchID",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "TestAncestorBranchID",
					BeginNodeID: 0,
					EndNodeID:   5,
				},
				{
					BranchID:    "TestAncestorBranchID",
					BeginNodeID: 6,
					EndNodeID:   10,
				},
			},
		},
	}
}

func expectedTreeRow() *nosqlplugin.HistoryTreeRow {
	return &nosqlplugin.HistoryTreeRow{
		ShardID:  testShardID,
		TreeID:   "TestTreeID",
		BranchID: "TestNewBranchID",
		Ancestors: []*types.HistoryBranchRange{
			{
				BranchID:  "TestAncestorBranchID",
				EndNodeID: 5,
			},
			{
				BranchID:  "TestAncestorBranchID",
				EndNodeID: 10,
			},
		},
		CreateTimestamp: FixedTime,
		Info:            "TestInfo",
	}
}

func treeRowEqual(t *testing.T, expected, actual *nosqlplugin.HistoryTreeRow) {
	t.Helper()

	assert.Equal(t, expected.ShardID, actual.ShardID)
	assert.Equal(t, expected.TreeID, actual.TreeID)
	assert.Equal(t, expected.BranchID, actual.BranchID)
	assert.Equal(t, expected.Ancestors, actual.Ancestors)
	assert.Equal(t, expected.Info, actual.Info)
	assert.Equal(t, FixedTime, actual.CreateTimestamp)
}

func TestForkHistoryBranch_NotAllAncestors(t *testing.T) {
	request := validInternalForkHistoryBranchRequest(8)
	expecedResp := expectedInternalForkHistoryBranchResponse()
	expTreeRow := expectedTreeRow()

	// The new branch ends at the fork node
	expecedResp.NewBranchInfo.Ancestors[1].EndNodeID = 8
	expTreeRow.Ancestors[1].EndNodeID = 8

	store, dbMock, _ := setUpMocks(t)

	// Expect to insert the new branch into the history tree
	dbMock.EXPECT().InsertIntoHistoryTreeAndNode(gomock.Any(), gomock.Any(), nil).
		DoAndReturn(func(ctx ctx.Context, treeRow *nosqlplugin.HistoryTreeRow, nodeRow *nosqlplugin.HistoryNodeRow) error {
			// Assert that the treeRow is as expected
			treeRowEqual(t, expTreeRow, treeRow)
			return nil
		}).Times(1)

	resp, err := store.ForkHistoryBranch(ctx.Background(), request)
	assert.NoError(t, err)
	assert.Equal(t, expecedResp, resp)
}

func TestForkHistoryBranch_AllAncestors(t *testing.T) {
	request := validInternalForkHistoryBranchRequest(14)
	expecedResp := expectedInternalForkHistoryBranchResponse()
	expTreeRow := expectedTreeRow()

	// The new branch inherits the ancestors from the fork node, and adds a new ancestor
	expecedResp.NewBranchInfo.Ancestors = append(expecedResp.NewBranchInfo.Ancestors, &types.HistoryBranchRange{
		BranchID:    "TestBranchID",
		BeginNodeID: 10, // The last in the fork node's ancestors
		EndNodeID:   14, // The fork node
	})
	expTreeRow.Ancestors = append(expTreeRow.Ancestors, &types.HistoryBranchRange{
		BranchID:  "TestBranchID",
		EndNodeID: 14,
	})

	store, dbMock, _ := setUpMocks(t)

	// Expect to insert the new branch into the history tree
	dbMock.EXPECT().InsertIntoHistoryTreeAndNode(gomock.Any(), gomock.Any(), nil).
		DoAndReturn(func(ctx ctx.Context, treeRow *nosqlplugin.HistoryTreeRow, nodeRow *nosqlplugin.HistoryNodeRow) error {
			// Assert that the treeRow is as expected
			treeRowEqual(t, expTreeRow, treeRow)
			return nil
		}).Times(1)

	resp, err := store.ForkHistoryBranch(ctx.Background(), request)
	assert.NoError(t, err)
	assert.Equal(t, expecedResp, resp)
}

func getValidInternalDeleteHistoryBranchRequest() *persistence.InternalDeleteHistoryBranchRequest {
	return &persistence.InternalDeleteHistoryBranchRequest{
		BranchInfo: types.HistoryBranch{
			TreeID:   "TestTreeID",
			BranchID: "TestBranchID",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "TestAncestorBranchID",
					BeginNodeID: 0,
					EndNodeID:   5,
				},
				{
					BranchID:    "TestAncestorBranchID",
					BeginNodeID: 6,
					EndNodeID:   10,
				},
			},
		},
		ShardID: testShardID,
	}
}

func TestDeleteHistoryBranch_unusedBranch(t *testing.T) {
	store, dbMock, _ := setUpMocks(t)

	request := getValidInternalDeleteHistoryBranchRequest()

	expecedTreeFilter := &nosqlplugin.HistoryTreeFilter{
		ShardID:  testShardID,
		TreeID:   "TestTreeID",
		BranchID: common.Ptr("TestBranchID"),
	}

	// Delete in reverse order, add 0 in the end
	expectedNodeFilters := []*nosqlplugin.HistoryNodeFilter{
		{
			ShardID:   testShardID,
			TreeID:    "TestTreeID",
			BranchID:  "TestBranchID",
			MinNodeID: 10,
		},
		{
			ShardID:   testShardID,
			TreeID:    "TestTreeID",
			BranchID:  "TestAncestorBranchID",
			MinNodeID: 6,
		},
		{
			ShardID:   testShardID,
			TreeID:    "TestTreeID",
			BranchID:  "TestAncestorBranchID",
			MinNodeID: 0,
		},
	}

	// Expect to delete the history branch
	dbMock.EXPECT().DeleteFromHistoryTreeAndNode(gomock.Any(), expecedTreeFilter, expectedNodeFilters).
		Return(nil).Times(1)
	dbMock.EXPECT().SelectFromHistoryTree(gomock.Any(), gomock.Any()).
		Return(nil, nil).Times(1)

	err := store.DeleteHistoryBranch(ctx.Background(), request)
	assert.NoError(t, err)
}

//	 In this base-case scenario, a workflow's been forked a few times, and the
//	 parent / ancestor workflow (branch A) is being removed.
//
//	  Trees
//	  ┌───────────────────┐    ┌───────────────────┐   ┌────────────────────┐
//	  │  Tree: 123        │    │  Tree: 123        │   │ Tree: 123          │
//	  │  Branch: C        │    │  Branch: A        │   │ Branch: B          │
//	  │  Ancestors:       │    │                   │   │ Ancestors:         │
//	  │    Branch: A      │    │                   │   │  Branch A          │
//	  │    EndNode: 3     │    │                   │   │  EndNode: 2        │
//	  └───────────────────┘    └───────────────────┘   └────────────────────┘
//	  Nodes
//	                           ┌───────────────────┐
//	                           │     Tree: 123     │
//	                           │     Branch: A     │
//	                           │     Node: 1       │
//	                           └─────────┬─────────┘
//	                                     │
//	                                     ┼───────────────────────┐
//	                           ┌─────────▼─────────┐   ┌─────────▼─────────┐
//	                           │     Tree: 123     │   │     Tree: 123     │
//	                           │     Branch: A     │   │     Branch: B     │
//	                           │     Node: 2       │   │     Node: 2       │
//	                           └─────────┬─────────┘   └─────────┬─────────┘
//	            ┌────────────────────────┤                       │
//	            │                        │                       │
//	  ┌─────────▼─────────┐    ┌─────────▼─────────┐   ┌─────────▼─────────┐
//	  │     Tree: 123     │    │     Tree: 123     │   │     Tree: 123     │
//	  │     Branch: C     │    │     Branch: A     │   │     Branch: B     │
//	  │     Node: 3       │    │     Node: 3       │   │     Node: 3       │
//	  └─────────┬─────────┘    └─────────┬─────────┘   └─────────┬─────────┘
//	            │                        │                       │
//	            │                        │                       │
//	  ┌─────────▼─────────┐    ┌─────────▼─────────┐   ┌─────────▼─────────┐
//	  │     Tree: 123     │    │     Tree: 123     │   │     Tree: 123     │
//	  │     Branch: C     │    │     Branch: A     │   │     Branch: B     │
//	  │     Node: 4       │    │     Node: 4       │   │     Node: 4       │
//	  └───────────────────┘    └───────────────────┘   └───────────────────┘
//
//	 The Expected behaviour is that the tree and unused nodes are trimmed off
//	 but the referenced nodes from other branches are kept so those workflows
//	 aren't broken.
//
//	           Trees
//
//	           ┌───────────────────┐    ┌─ ─ ─ ─ ─ ─ ─ ─ ─ ─┐   ┌────────────────────┐
//	           │  Tree: 123        │                            │ Tree: 123          │
//	           │  Branch: C        │    │  <deleted>        │   │ Branch: B          │
//	           │  Ancestors:       │                            │ Ancestors:         │
//	           │    Branch: A      │    │                   │   │  Branch A          │
//	           │    EndNode: 3     │                            │  EndNode: 2        │
//	           └───────────────────┘    └─ ─ ─ ─ ─ ─ ─ ─ ─ ─┘   └────────────────────┘
//
//	           Nodes
//	                                    ┌───────────────────┐
//	                                    │     Tree: 123     │
//	                                    │     Branch: A     │
//	                                    │     Node: 1       │
//	                                    └─────────┬─────────┘
//	                                              │
//	                                              ┼───────────────────────┐
//	                                    ┌─────────▼─────────┐   ┌─────────▼─────────┐
//	                                    │     Tree: 123     │   │     Tree: 123     │
//	                                    │     Branch: A     │   │     Branch: B     │
//	                                    │     Node: 2       │   │     Node: 2       │
//	                                    └─────────┬─────────┘   └─────────┬─────────┘
//	                     ┌────────────────────────┤                       │
//	                     │                        │                       │
//	           ┌─────────▼─────────┐    ┌─ ─ ─ ─ ─▼ ─ ─ ─ ─ ┐   ┌─────────▼─────────┐
//	           │     Tree: 123     │                            │     Tree: 123     │
//	           │     Branch: C     │    │ <deleted>         │   │     Branch: B     │
//	           │     Node: 3       │                            │     Node: 3       │
//	           └─────────┬─────────┘    └─ ─ ─ ─ ─┐ ─ ─ ─ ─ ┘   └─────────┬─────────┘
//	                     │                        │                       │
//	                     │                        │                       │
//	           ┌─────────▼─────────┐    ┌─ ─ ─ ─ ─▼ ─ ─ ─ ─ ┐   ┌─────────▼─────────┐
//	           │     Tree: 123     │                            │     Tree: 123     │
//	           │     Branch: C     │    │ <deleted>         │   │     Branch: B     │
//	           │     Node: 4       │                            │     Node: 4       │
//	           └───────────────────┘    └─ ─ ─ ─ ── ─ ─ ─ ─ ┘   └───────────────────┘

func TestDeleteHistoryBranchWithAFewBranches_baseCase(t *testing.T) {

	store, dbMock, _ := setUpMocks(t)

	request := &persistence.InternalDeleteHistoryBranchRequest{
		BranchInfo: types.HistoryBranch{
			TreeID:   "TestTreeID",
			BranchID: "A",
		},
		ShardID: testShardID,
	}

	expectedTreeFilter := &nosqlplugin.HistoryTreeFilter{
		ShardID:  testShardID,
		TreeID:   "TestTreeID",
		BranchID: common.Ptr("A"),
	}

	// Delete in reverse order, add 0 in the end
	expectedNodeFilters := []*nosqlplugin.HistoryNodeFilter{
		{
			ShardID:   testShardID,
			TreeID:    "TestTreeID",
			BranchID:  "A",
			MinNodeID: 3,
		},
	}

	dbMock.EXPECT().DeleteFromHistoryTreeAndNode(gomock.Any(), expectedTreeFilter, expectedNodeFilters).
		Return(nil).Times(1)

	historyTree := []*nosqlplugin.HistoryTreeRow{
		{
			ShardID:  testShardID,
			TreeID:   "TestTreeID",
			BranchID: "A",
		},
		{
			ShardID:  testShardID,
			TreeID:   "TestTreeID",
			BranchID: "B",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "A",
					BeginNodeID: 0,
					EndNodeID:   2,
				},
			},
		},
		{
			ShardID:  testShardID,
			TreeID:   "TestTreeID",
			BranchID: "C",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "A",
					BeginNodeID: 0,
					EndNodeID:   3,
				},
			},
		},
	}

	dbMock.EXPECT().SelectFromHistoryTree(gomock.Any(), gomock.Any()).
		Return(historyTree, nil).Times(1)

	err := store.DeleteHistoryBranch(ctx.Background(), request)
	assert.NoError(t, err)

}

// In this scenario, Branch A has already been deleted, but is referenced by
// a couple of other branches.
//
// In this scenario Branch B is being deleted.
//
// Given the following starting state:
//
//	Trees
//
//	 ┌───────────────────┐    ┌─ ─ ─ ─ ─ ─ ─ ─ ─ ─┐   ┌────────────────────┐
//	 │  Tree: 123        │                            │ Tree: 123          │
//	 │  Branch: C        │    │  <branch A is     │   │ Branch: B          │
//	 │  Ancestors:       │        deleted>            │ Ancestors:         │
//	 │    Branch: A      │    │                   │   │  Branch A          │
//	 │    EndNode: 3     │                            │  EndNode: 2        │
//	 └───────────────────┘    └─ ─ ─ ─ ─ ─ ─ ─ ─ ─┘   └────────────────────┘
//
//	  Nodes
//
//	                        ┌───────────────────┐
//	                        │     Tree: 123     │
//	                        │     Branch: A     │
//	                        │     Node: 1       │
//	                        └─────────┬─────────┘
//	                                  │
//	                                  ┼─────────────────────────┐
//	                        ┌─────────▼─────────┐     ┌─────────▼─────────┐
//	                        │     Tree: 123     │     │     Tree: 123     │
//	                        │     Branch: A     │     │     Branch: B     │
//	                        │     Node: 2       │     │     Node: 2       │
//	                        └─────────┬─────────┘     └─────────┬─────────┘
//	         ┌────────────────────────┘                         │
//	         │                                                  │
//	 ┌─────────▼─────────┐                            ┌─────────▼─────────┐
//	 │     Tree: 123     │                            │     Tree: 123     │
//	 │     Branch: C     │                            │     Branch: B     │
//	 │     Node: 3       │                            │     Node: 3       │
//	 └─────────┬─────────┘                            └─────────┬─────────┘
//	           │                                                │
//	           │                                                │
//	 ┌─────────▼─────────┐                            ┌─────────▼─────────┐
//	 │     Tree: 123     │                            │     Tree: 123     │
//	 │     Branch: C     │                            │     Branch: B     │
//	 │     Node: 4       │                            │     Node: 4       │
//	 └───────────────────┘                            └───────────────────┘
//
// The following is expected: It preserves the remaining nodes for any dependent
// branches.
//
//	Trees
//
//	┌───────────────────┐    ┌─ ─ ─ ─ ─ ─ ─ ─ ─ ─┐     ┌─ ─ ─ ─ ─ ─ ─ ─ ─ ─┐
//	│  Tree: 123        │
//	│  Branch: C        │    │  <branch A is     │     │  <branch B is     │
//	│  Ancestors:       │        deleted>                  deleted>
//	│    Branch: A      │    │                   │     │                   │
//	│    EndNode: 3     │
//	└───────────────────┘    └─ ─ ─ ─ ─ ─ ─ ─ ─ ─┘     └─ ─ ─ ─ ─ ─ ─ ─ ─ ─┘
//
//	Nodes
//	                         ┌───────────────────┐
//	                         │     Tree: 123     │
//	                         │     Branch: A     │
//	                         │     Node: 1       │
//	                         └─────────┬─────────┘
//	                                   │
//	                                   ┼
//	                         ┌─────────▼─────────┐
//	                         │     Tree: 123     │
//	                         │     Branch: A     │
//	                         │     Node: 2       │
//	                         └─────────┬─────────┘
//	          ┌────────────────────────┘
//	          │
//	┌─────────▼─────────┐
//	│     Tree: 123     │
//	│     Branch: C     │
//	│     Node: 3       │
//	└─────────┬─────────┘
//	          │
//	          │
//	┌─────────▼─────────┐
//	│     Tree: 123     │
//	│     Branch: C     │
//	│     Node: 4       │
//	└───────────────────┘
func TestDeleteHistoryBranch_DeletedAncestor(t *testing.T) {
	store, dbMock, _ := setUpMocks(t)

	request := &persistence.InternalDeleteHistoryBranchRequest{
		BranchInfo: types.HistoryBranch{
			TreeID:   "TestTreeID",
			BranchID: "B",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "A",
					BeginNodeID: 0,
					EndNodeID:   2,
				},
			},
		},
		ShardID: testShardID,
	}

	expectedTreeFilter := &nosqlplugin.HistoryTreeFilter{
		ShardID:  testShardID,
		TreeID:   "TestTreeID",
		BranchID: common.Ptr("B"),
	}

	expectedNodeFilters := []*nosqlplugin.HistoryNodeFilter{
		{
			ShardID:   testShardID,
			TreeID:    "TestTreeID",
			BranchID:  "B",
			MinNodeID: 2,
		},
		{
			ShardID:   testShardID,
			TreeID:    "TestTreeID",
			BranchID:  "A",
			MinNodeID: 3,
		},
	}

	dbMock.EXPECT().DeleteFromHistoryTreeAndNode(gomock.Any(), expectedTreeFilter, expectedNodeFilters).
		Return(nil).Times(1)

	historyTree := []*nosqlplugin.HistoryTreeRow{
		{
			ShardID:  testShardID,
			TreeID:   "TestTreeID",
			BranchID: "B",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "A",
					BeginNodeID: 0,
					EndNodeID:   2,
				},
			},
		},
		{
			ShardID:  testShardID,
			TreeID:   "TestTreeID",
			BranchID: "C",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "A",
					BeginNodeID: 0,
					EndNodeID:   3,
				},
			},
		},
		// notably A does not exist
	}

	dbMock.EXPECT().SelectFromHistoryTree(gomock.Any(), gomock.Any()).
		Return(historyTree, nil).Times(1)

	err := store.DeleteHistoryBranch(ctx.Background(), request)
	assert.NoError(t, err)
}

// In this scenario, something like the following has happened:
// There was a normal, original workflow, which was then branched (perhaps by a reset).
// By the time the workflow is being cleaned up, the ancestor branch has been deleted already,
// and does not exist in the history_tree table as a valid branch.
//
// In this scenario, branch B is being deleted. Notably, Branch B is the *last*
// valid branch for this tree, whereas the ancestor branches were removed earlier.
//
//	 Trees
//
//	 ┌─ ─ ─ ─ ─ ─ ─ ─ ─ ─┐    ┌───────────────────┐
//	                          │  Tree: 123        │
//	 │  <branch A is     │    │  Branch: B        │
//	     deleted>             │  Ancestors:       │
//	 │                   │    │    Branch: A      │
//	                          │    EndNode: 3     │
//	 └─ ─ ─ ─ ─ ─ ─ ─ ─ ─┘    └───────────────────┘
//
//
//	 Nodes
//
//	 ┌───────────────────┐
//	 │     Tree: 123     │
//	 │     Branch: A     │
//	 │     Node: 1       │
//	 └─────────┬─────────┘
//	           │
//	 ┌─────────┼─────────┐
//	 │     Tree: 123     │
//	 │     Branch: A     │
//	 │     Node: 2       │
//	 └─────────┬─────────┘
//	           ┼────────────────────────┐
//	                                    │
//	                                    │
//	                          ┌─────────▼─────────┐
//	                          │     Tree: 123     │
//	                          │     Branch: B     │
//	                          │     Node: 3       │
//	                          └─────────┬─────────┘
//	                                    │
//	                                    │
//	                          ┌─────────▼─────────┐
//	                          │     Tree: 123     │
//	                          │     Branch: B     │
//	                          │     Node: 4       │
//	                          └───────────────────┘
//
//	The expected behaviour, is that the child/branched workflow needs to clean up both its own history nodes
//	but *all* of the parent's remaining and now unreferenced history nodes. They're otherwise unreachable
//	and will be just history_node table garbage which forever grows.
//
//	 Trees
//
//	 ┌─ ─ ─ ─ ─ ─ ─ ─ ─ ─┐    ┌─ ─ ─ ─ ─ ─ ─ ─ ─ ─┐
//	                          │                   │
//	 │  <branch A is     │
//	     deleted>             │   deleted>        │
//	 │                   │
//	                          │                   │
//	 └─ ─ ─ ─ ─ ─ ─ ─ ─ ─┘    └─ ─ ─ ─ ─ ─ ─ ─ ─ ─┘
//
//
//	 Nodes
//
//	 ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
//	 │                   │
//	 │  <deleted>        │
//	 │                   │
//	 └ ─ ─ ─ ─ ┌ ─ ─ ─ ─ ┘
//	           │
//	 ┌ ─ ─ ─ ─ ▼ ─ ─ ─ ─ ┐
//	 │                   │
//	 │  <deleted>        │
//	 │                   │
//	 └ ─ ─ ─ ─ ┌ ─ ─ ─ ─ ┘
//	           │
//	           ┼ ─ ─ ─ ─ ── ─ ── ─ ─ ── ┐
//	           │                        │
//	 ┌ ─ ─ ─ ─ ▼ ─ ─ ─ ─ ┐    ┌─ ─ ─ ── ▼─ ─ ─ ─ ─┐
//	 │                   │    │                   │
//	 │  <deleted>        │    │ <deleted>         │
//	 │                   │    │                   │
//	 └ ─ ─ ─ ─ ┌ ─ ─ ─ ─ ┘    └─ ─ ─ ── ┌─ ─ ─ ─ ─┘
//	           │                        │
//	           │                        │
//	 ┌ ─ ─ ─ ─ ▼ ─ ─ ─ ─ ┐    ┌─ ─ ─ ── ▼─ ─ ─ ─ ─┐
//	 │                   │    │                   │
//	 │  <deleted>        │    │ <deleted>         │
//	 │                   │    │                   │
//	 └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘    └─ ─ ─ ── ── ─ ─ ─ ─┘
func TestDeleteHistoryBranch_usedBranchWithGarbageFullyCleanedUp(t *testing.T) {
	store, dbMock, _ := setUpMocks(t)

	request := &persistence.InternalDeleteHistoryBranchRequest{
		BranchInfo: types.HistoryBranch{
			TreeID:   "TestTreeID",
			BranchID: "B",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:  "A",
					EndNodeID: 3,
				},
			},
		},
		ShardID: testShardID,
	}

	expectedTreeFilter := &nosqlplugin.HistoryTreeFilter{
		ShardID:  testShardID,
		TreeID:   "TestTreeID",
		BranchID: common.Ptr("B"),
	}

	expectedNodeFilters := []*nosqlplugin.HistoryNodeFilter{
		{
			ShardID:   testShardID,
			TreeID:    "TestTreeID",
			BranchID:  "B",
			MinNodeID: 3,
		},
		{
			ShardID:  testShardID,
			TreeID:   "TestTreeID",
			BranchID: "A",
		},
	}

	dbMock.EXPECT().DeleteFromHistoryTreeAndNode(gomock.Any(), expectedTreeFilter, expectedNodeFilters).
		Return(nil).Times(1)

	historyTree := []*nosqlplugin.HistoryTreeRow{
		{
			ShardID:  testShardID,
			TreeID:   "TestTreeID",
			BranchID: "B",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "A",
					BeginNodeID: 0,
					EndNodeID:   3,
				},
			},
		},
		// notably A does not exist
	}

	dbMock.EXPECT().SelectFromHistoryTree(gomock.Any(), gomock.Any()).
		Return(historyTree, nil).Times(1)

	err := store.DeleteHistoryBranch(ctx.Background(), request)
	assert.NoError(t, err)
}

// In this scenario, there's a few branches of a normal workflow. In this example, both history_trees exist
// and the original branch has not yet been deleted, and (for whatever reason, the second branch is being removed
// before the original/parent (it's not super obvious this might happen, but it's forseeable with deletion jitter
// or maybe failover scenarios where this might happen).
//
// In this scenario, branch B is being deleted.
//
//	Trees
//
//	┌───────────────────┐    ┌───────────────────┐   ┌────────────────────┐
//	│  Tree: 123        │    │  Tree: 123        │   │ Tree: 123          │
//	│  Branch: C        │    │  Branch: A        │   │ Branch: B          │
//	│  Ancestors:       │    │                   │   │ Ancestors:         │
//	│    Branch: A      │    │                   │   │  Branch A          │
//	│    Endnode: 3     │    │                   │   │  EndNode: 2        │
//	└───────────────────┘    └───────────────────┘   └────────────────────┘
//
//	Nodes
//
//	                         ┌───────────────────┐
//	                         │     Tree: 123     │
//	                         │     Branch: A     │
//	                         │     Node: 1       │
//	                         └─────────┬─────────┘
//	                                   │
//	                                   ┼───────────────────────┐
//	                         ┌─────────▼─────────┐   ┌─────────▼─────────┐
//	                         │     Tree: 123     │   │     Tree: 123     │
//	                         │     Branch: A     │   │     Branch: B     │
//	                         │     Node: 2       │   │     Node: 2       │
//	                         └─────────┬─────────┘   └─────────┬─────────┘
//	          ┌────────────────────────┤                       │
//	          │                        │                       │
//	┌─────────▼─────────┐    ┌─────────▼─────────┐   ┌─────────▼─────────┐
//	│     Tree: 123     │    │     Tree: 123     │   │     Tree: 123     │
//	│     Branch: C     │    │     Branch: A     │   │     Branch: B     │
//	│     Node: 3       │    │     Node: 3       │   │     Node: 3       │
//	└─────────┬─────────┘    └─────────┬─────────┘   └─────────┬─────────┘
//	          │                        │                       │
//	          │                        │                       │
//	┌─────────▼─────────┐    ┌─────────▼─────────┐   ┌─────────▼─────────┐
//	│     Tree: 123     │    │     Tree: 123     │   │     Tree: 123     │
//	│     Branch: C     │    │     Branch: A     │   │     Branch: B     │
//	│     Node: 4       │    │     Node: 4       │   │     Node: 4       │
//	└───────────────────┘    └───────────────────┘   └───────────────────┘
//
// The expected behaviour is that the child/second branch should only clean up it's history nodes, but
// leave the parents alone, so as to not break the parent.
//
//	Trees
//
//	┌───────────────────┐     ┌───────────────────┐     ┌─ ─ ─ ─ ─ ─ ─ ─ ─ ─┐
//	│  Tree: 123        │     │  Tree: 123        │
//	│  Branch: C        │     │  Branch: A        │     │  <branch B is     │
//	│  Ancestors:       │     │                   │         deleted>
//	│    Branch: A      │     │                   │     │                   │
//	│    EndNode: 2     │     │                   │
//	└───────────────────┘     └───────────────────┘     └─ ─ ─ ─ ─ ─ ─ ─ ─ ─┘
//
//
//	Nodes
//
//	                          ┌───────────────────┐
//	                          │     Tree: 123     │
//	                          │     Branch: A     │
//	                          │     Node: 1       │
//	                          └─────────┬─────────┘
//	                                    │
//	                                    ┼
//	                          ┌─────────▼─────────┐
//	                          │     Tree: 123     │
//	                          │     Branch: A     │
//	                          │     Node: 2       │
//	                          └─────────┬─────────┘
//	          ┌─────────────────────────┤
//	          │                         │
//	┌─────────▼─────────┐     ┌─────────▼─────────┐
//	│     Tree: 123     │     │     Tree: 123     │
//	│     Branch: C     │     │     Branch: A     │
//	│     Node: 3       │     │     Node: 3       │
//	└─────────┬─────────┘     └─────────┬─────────┘
//	          │                         │
//	          │                         │
//	┌─────────▼─────────┐     ┌─────────▼─────────┐
//	│     Tree: 123     │     │     Tree: 123     │
//	│     Branch: C     │     │     Branch: A     │
//	│     Node: 4       │     │     Node: 4       │
//	└───────────────────┘     └───────────────────┘
func TestDeleteHistoryBranch_withAnAncestorBranchWhichIsStillInUse(t *testing.T) {
	store, dbMock, _ := setUpMocks(t)

	request := &persistence.InternalDeleteHistoryBranchRequest{
		BranchInfo: types.HistoryBranch{
			TreeID:   "TestTreeID",
			BranchID: "B",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:  "A",
					EndNodeID: 2,
				},
			},
		},
		ShardID: testShardID,
	}

	expectedTreeFilter := &nosqlplugin.HistoryTreeFilter{
		ShardID:  testShardID,
		TreeID:   "TestTreeID",
		BranchID: common.Ptr("B"),
	}

	expectedNodeFilters := []*nosqlplugin.HistoryNodeFilter{
		{
			ShardID:   testShardID,
			TreeID:    "TestTreeID",
			BranchID:  "B",
			MinNodeID: 2,
		},
		// we do not delete any of the ancestor, it's still valid
	}

	// Expect to delete the history branch
	dbMock.EXPECT().DeleteFromHistoryTreeAndNode(gomock.Any(), expectedTreeFilter, expectedNodeFilters).
		Return(nil).Times(1)

	historyTree := []*nosqlplugin.HistoryTreeRow{
		{
			ShardID:   testShardID,
			TreeID:    "TestTreeID",
			BranchID:  "A",
			Ancestors: []*types.HistoryBranchRange{},
		},
		{
			ShardID:  testShardID,
			TreeID:   "TestTreeID",
			BranchID: "B",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:  "A",
					EndNodeID: 2,
				},
			},
		},
		{
			ShardID:  testShardID,
			TreeID:   "TestTreeID",
			BranchID: "C",
			Ancestors: []*types.HistoryBranchRange{
				{
					BranchID:    "A",
					BeginNodeID: 0,
					EndNodeID:   3,
				},
			},
		},
	}

	dbMock.EXPECT().SelectFromHistoryTree(gomock.Any(), gomock.Any()).
		Return(historyTree, nil).Times(1)

	err := store.DeleteHistoryBranch(ctx.Background(), request)
	assert.NoError(t, err)
}

func TestGetAllHistoryTreeBranches(t *testing.T) {
	request := &persistence.GetAllHistoryTreeBranchesRequest{
		NextPageToken: []byte("nextPageToken"),
		PageSize:      1000,
	}

	store, dbMock, shardedNoSQLStoreMock := setUpMocks(t)
	shardedNoSQLStoreMock.EXPECT().GetShardingPolicy().Return(shardingPolicy{hasShardedHistory: false})
	shardedNoSQLStoreMock.EXPECT().GetDefaultShard().Return(nosqlStore{db: dbMock}).Times(1)

	expTreeRow := expectedTreeRow()

	// Create another tree row with some different data
	expTreeRow2 := expectedTreeRow()
	expTreeRow2.TreeID = "TestTreeID2"
	expTreeRow2.BranchID = "TestNewBranchID2"
	expTreeRow2.CreateTimestamp = time.Unix(123, 456)
	expTreeRow2.Info = "TestInfo2"

	expTreeRows := []*nosqlplugin.HistoryTreeRow{expTreeRow, expTreeRow2}
	dbMock.EXPECT().SelectAllHistoryTrees(gomock.Any(), request.NextPageToken, request.PageSize).
		Return(expTreeRows, []byte("anotherPageToken"), nil).Times(1)

	expectedBranches := []persistence.HistoryBranchDetail{
		{
			TreeID:   "TestTreeID",
			BranchID: "TestNewBranchID",
			ForkTime: expTreeRow.CreateTimestamp,
			Info:     "TestInfo",
		},
		{
			TreeID:   "TestTreeID2",
			BranchID: "TestNewBranchID2",
			ForkTime: expTreeRow2.CreateTimestamp,
			Info:     "TestInfo2",
		},
	}

	expectedResponse := &persistence.GetAllHistoryTreeBranchesResponse{
		Branches:      expectedBranches,
		NextPageToken: []byte("anotherPageToken"),
	}

	resp, err := store.GetAllHistoryTreeBranches(ctx.Background(), request)
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, resp)
}

func TestGetAllHistoryTreeBranches_dbError(t *testing.T) {
	request := &persistence.GetAllHistoryTreeBranchesRequest{
		NextPageToken: []byte("nextPageToken"),
		PageSize:      1000,
	}

	store, dbMock, shardedNoSQLStoreMock := setUpMocks(t)
	shardedNoSQLStoreMock.EXPECT().GetShardingPolicy().Return(shardingPolicy{hasShardedHistory: false})
	shardedNoSQLStoreMock.EXPECT().GetDefaultShard().Return(nosqlStore{db: dbMock}).Times(1)

	testError := errors.New("TEST ERROR")
	dbMock.EXPECT().SelectAllHistoryTrees(gomock.Any(), request.NextPageToken, request.PageSize).
		Return(nil, nil, testError).Times(1)
	dbMock.EXPECT().IsNotFoundError(testError).Return(true).Times(1)

	_, err := store.GetAllHistoryTreeBranches(ctx.Background(), request)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "TEST ERROR")
	assert.ErrorContains(t, err, "SelectAllHistoryTrees")
}

func TestGetAllHistoryTreeBranches_hasShardedPolicy(t *testing.T) {
	request := &persistence.GetAllHistoryTreeBranchesRequest{
		NextPageToken: []byte("nextPageToken"),
		PageSize:      1000,
	}

	store, _, shardedNoSQLStoreMock := setUpMocks(t)
	shardedNoSQLStoreMock.EXPECT().GetShardingPolicy().Return(shardingPolicy{hasShardedHistory: true})

	_, err := store.GetAllHistoryTreeBranches(ctx.Background(), request)
	assert.Error(t, err)
	var internalServiceErr *types.InternalServiceError
	assert.ErrorAs(t, err, &internalServiceErr)
	assert.Equal(t, "SelectAllHistoryTrees is not supported on sharded nosql db", internalServiceErr.Message)
}
