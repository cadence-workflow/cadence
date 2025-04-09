package sqlite

import (
	"context"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	lockTaskListQry = `SELECT range_id FROM task_lists ` +
		`WHERE shard_id = ? AND domain_id = ? AND name = ? AND task_type = ?`
)

// LockTaskLists locks a row in task_lists table
func (mdb *DB) LockTaskLists(ctx context.Context, filter *sqlplugin.TaskListsFilter) (int64, error) {
	var rangeID int64
	err := mdb.driver.GetContext(ctx, filter.ShardID, &rangeID, lockTaskListQry, filter.ShardID, *filter.DomainID, *filter.Name, *filter.TaskType)
	return rangeID, err
}
