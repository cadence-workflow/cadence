// Copyright (c) 2020 Uber Technologies, Inc.
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

package cassandra

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

// InsertIntoHistoryTreeAndNode inserts one or two rows: tree row and node row(at least one of them)
func (db *CDB) InsertIntoHistoryTreeAndNode(ctx context.Context, treeRow *nosqlplugin.HistoryTreeRow, nodeRow *nosqlplugin.HistoryNodeRow) error {
	if treeRow == nil && nodeRow == nil {
		return fmt.Errorf("require at least a tree row or a node row to insert")
	}

	var ancs []map[string]interface{}
	if treeRow != nil {
		for _, an := range treeRow.Ancestors {
			value := make(map[string]interface{})
			value["end_node_id"] = an.EndNodeID
			value["branch_id"] = an.BranchID
			ancs = append(ancs, value)
		}
	}

	var err error
	if treeRow != nil && nodeRow != nil {
		// Note: for perf, prefer using batch for inserting more than one records
		batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
		batch.Query(v2templateInsertTree,
			treeRow.TreeID, treeRow.BranchID, ancs, persistence.UnixNanoToDBTimestamp(treeRow.CreateTimestamp.UnixNano()), treeRow.Info, treeRow.CreateTimestamp)
		batch.Query(v2templateUpsertData,
			nodeRow.TreeID, nodeRow.BranchID, nodeRow.NodeID, nodeRow.TxnID, nodeRow.Data, nodeRow.DataEncoding, nodeRow.CreateTimestamp)
		err = db.session.ExecuteBatch(batch)
	} else {
		var query gocql.Query
		if treeRow != nil {
			query = db.session.Query(v2templateInsertTree,
				treeRow.TreeID, treeRow.BranchID, ancs, persistence.UnixNanoToDBTimestamp(treeRow.CreateTimestamp.UnixNano()), treeRow.Info, treeRow.CreateTimestamp).WithContext(ctx)
		}
		if nodeRow != nil {
			query = db.session.Query(v2templateUpsertData,
				nodeRow.TreeID, nodeRow.BranchID, nodeRow.NodeID, nodeRow.TxnID, nodeRow.Data, nodeRow.DataEncoding, nodeRow.CreateTimestamp).WithContext(ctx)
		}
		err = query.Exec()
	}

	return err
}

// SelectFromHistoryNode read nodes based on a filter
func (db *CDB) SelectFromHistoryNode(ctx context.Context, filter *nosqlplugin.HistoryNodeFilter) ([]*nosqlplugin.HistoryNodeRow, []byte, error) {
	query := db.session.Query(v2templateReadData, filter.TreeID, filter.BranchID, filter.MinNodeID, filter.MaxNodeID).WithContext(ctx)

	iter := query.PageSize(filter.PageSize).PageState(filter.NextPageToken).Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectFromHistoryNode operation failed.  Not able to create query iterator.",
		}
	}
	pagingToken := iter.PageState()
	var rows []*nosqlplugin.HistoryNodeRow
	row := &nosqlplugin.HistoryNodeRow{}
	for iter.Scan(&row.NodeID, &row.TxnID, &row.Data, &row.DataEncoding) {
		rows = append(rows, row)
		row = &nosqlplugin.HistoryNodeRow{}
	}

	if err := iter.Close(); err != nil {
		return nil, nil, err
	}
	return rows, pagingToken, nil
}

// DeleteFromHistoryTreeAndNode delete a branch record, and a list of ranges of nodes.
func (db *CDB) DeleteFromHistoryTreeAndNode(ctx context.Context, treeFilter *nosqlplugin.HistoryTreeFilter, nodeFilters []*nosqlplugin.HistoryNodeFilter) error {
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.Query(v2templateDeleteBranch, treeFilter.TreeID, treeFilter.BranchID)
	for _, nodeFilter := range nodeFilters {
		batch.Query(v2templateRangeDeleteData,
			nodeFilter.TreeID,
			nodeFilter.BranchID,
			nodeFilter.MinNodeID)
	}
	return db.executeBatchWithConsistencyAll(batch)
}

// SelectAllHistoryTrees will return all tree branches with pagination
func (db *CDB) SelectAllHistoryTrees(ctx context.Context, nextPageToken []byte, pageSize int) ([]*nosqlplugin.HistoryTreeRow, []byte, error) {
	query := db.session.Query(v2templateScanAllTreeBranches).WithContext(ctx)

	iter := query.PageSize(int(pageSize)).PageState(nextPageToken).Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectAllHistoryTrees operation failed.  Not able to create query iterator.",
		}
	}
	pagingToken := iter.PageState()

	createTime := time.Time{}
	var rows []*nosqlplugin.HistoryTreeRow
	row := &nosqlplugin.HistoryTreeRow{}
	for iter.Scan(&row.TreeID, &row.BranchID, &createTime, &row.Info) {
		row.CreateTimestamp = time.Unix(0, persistence.DBTimestampToUnixNano(persistence.UnixNanoToDBTimestamp(createTime.UnixNano())))
		rows = append(rows, row)
		row = &nosqlplugin.HistoryTreeRow{}
	}

	if err := iter.Close(); err != nil {
		return nil, nil, err
	}
	return rows, pagingToken, nil
}

// SelectFromHistoryTree read branch records for a tree
func (db *CDB) SelectFromHistoryTree(ctx context.Context, filter *nosqlplugin.HistoryTreeFilter) ([]*nosqlplugin.HistoryTreeRow, error) {
	query := db.session.Query(v2templateReadAllBranches, filter.TreeID).WithContext(ctx)
	var pagingToken []byte
	var iter gocql.Iter
	var rows []*nosqlplugin.HistoryTreeRow
	for {
		iter = query.PageSize(100).PageState(pagingToken).Iter()
		if iter == nil {
			return nil, &types.InternalServiceError{
				Message: "SelectFromHistoryTree operation failed.  Not able to create query iterator.",
			}
		}
		pagingToken = iter.PageState()

		branchUUID := ""
		var ancsResult []map[string]interface{}
		// Ideally we should just use int64. But we have been using time.Time for a long time.
		// I am not sure using a int64 will behave the same.
		// Therefore, here still using a time.Time to read, and then convert to int64
		createTime := time.Time{}
		info := ""

		for iter.Scan(&branchUUID, &ancsResult, &createTime, &info) {
			ancs := parseBranchAncestors(ancsResult)
			row := &nosqlplugin.HistoryTreeRow{
				TreeID:    filter.TreeID,
				BranchID:  branchUUID,
				Ancestors: ancs,
			}
			rows = append(rows, row)

			branchUUID = ""
			ancsResult = []map[string]interface{}{}
			createTime = time.Time{}
			info = ""
		}

		if err := iter.Close(); err != nil {
			return nil, err
		}

		if len(pagingToken) == 0 {
			break
		}
	}
	return rows, nil
}

func parseBranchAncestors(
	ancestors []map[string]interface{},
) []*types.HistoryBranchRange {

	ans := make([]*types.HistoryBranchRange, 0, len(ancestors))
	for _, e := range ancestors {
		an := &types.HistoryBranchRange{}
		for k, v := range e {
			switch k {
			case "branch_id":
				an.BranchID = v.(gocql.UUID).String()
			case "end_node_id":
				an.EndNodeID = v.(int64)
			}
		}
		ans = append(ans, an)
	}

	if len(ans) > 0 {
		// sort ans based onf EndNodeID so that we can set BeginNodeID
		sort.Slice(ans, func(i, j int) bool { return ans[i].EndNodeID < ans[j].EndNodeID })
		ans[0].BeginNodeID = int64(1)
		for i := 1; i < len(ans); i++ {
			ans[i].BeginNodeID = ans[i-1].EndNodeID
		}
	}
	return ans
}
