package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	lockTaskListQry = `SELECT range_id FROM task_lists ` +
		`WHERE shard_id = ? AND domain_id = ? AND name = ? AND task_type = ?`

	deleteTaskQry = `DELETE FROM tasks ` +
		`WHERE domain_id = ? AND task_list_name = ? AND task_type = ? AND task_id = ?`

	rangeDeleteTaskQry = `WITH tasks_to_delete AS (
    SELECT domain_id,task_list_name,task_type,task_id 
    FROM tasks
    WHERE domain_id = ? AND task_list_name = ? AND task_type = ? AND task_id <= ?
    ORDER BY domain_id,task_list_name,task_type,task_id
    LIMIT ?
)

DELETE FROM transfer_tasks
WHERE (domain_id,task_list_name,task_type,task_id ) IN (SELECT domain_id,task_list_name,task_type,task_id  tasks_to_delete);`
)

// LockTaskLists locks a row in task_lists table
func (mdb *DB) LockTaskLists(ctx context.Context, filter *sqlplugin.TaskListsFilter) (int64, error) {
	var rangeID int64
	err := mdb.driver.GetContext(ctx, filter.ShardID, &rangeID, lockTaskListQry, filter.ShardID, *filter.DomainID, *filter.Name, *filter.TaskType)
	return rangeID, err
}

// DeleteFromTasks deletes one or more rows from tasks table
func (mdb *DB) DeleteFromTasks(ctx context.Context, filter *sqlplugin.TasksFilter) (sql.Result, error) {
	if filter.TaskIDLessThanEquals != nil {
		if filter.Limit == nil || *filter.Limit == 0 {
			return nil, fmt.Errorf("missing limit parameter")
		}
		return mdb.driver.ExecContext(ctx, filter.ShardID, rangeDeleteTaskQry,
			filter.DomainID, filter.TaskListName, filter.TaskType, *filter.TaskIDLessThanEquals, *filter.Limit)
	}
	return mdb.driver.ExecContext(ctx, filter.ShardID, deleteTaskQry, filter.DomainID, filter.TaskListName, filter.TaskType, *filter.TaskID)
}
