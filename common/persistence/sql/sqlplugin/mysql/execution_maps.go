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

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	deleteMapQryTemplate = `DELETE FROM %v
WHERE
shard_id = ? AND
domain_id = ? AND
workflow_id = ? AND
run_id = ?`

	// %[2]v is the columns of the value struct (i.e. no primary key columns), comma separated
	// %[3]v should be %[2]v with colons prepended.
	// i.e. %[3]v = ",".join(":" + s for s in %[2]v)
	// So that this query can be used with BindNamed
	// %[4]v should be the name of the key associated with the map
	// e.g. for ActivityInfo it is "schedule_id"
	setKeyInMapQryTemplate = `REPLACE INTO %[1]v
(shard_id, domain_id, workflow_id, run_id, %[4]v, %[2]v)
VALUES
(:shard_id, :domain_id, :workflow_id, :run_id, :%[4]v, %[3]v)`

	// %[2]v is the name of the key
	deleteKeyInMapQryTemplate = `DELETE FROM %[1]v
WHERE
shard_id = ? AND
domain_id = ? AND
workflow_id = ? AND
run_id = ? AND
%[2]v IN ( ? )`

	// %[1]v is the name of the table
	// %[2]v is the name of the key
	// %[3]v is the value columns, separated by commas
	getMapQryTemplate = `SELECT %[2]v, %[3]v FROM %[1]v
WHERE
shard_id = ? AND
domain_id = ? AND
workflow_id = ? AND
run_id = ?`
)

func stringMap(a []string, f func(string) string) []string {
	b := make([]string, len(a))
	for i, v := range a {
		b[i] = f(v)
	}
	return b
}

func makeDeleteMapQry(tableName string) string {
	return fmt.Sprintf(deleteMapQryTemplate, tableName)
}

func makeSetKeyInMapQry(tableName string, nonPrimaryKeyColumns []string, mapKeyName string) string {
	return fmt.Sprintf(setKeyInMapQryTemplate,
		tableName,
		strings.Join(nonPrimaryKeyColumns, ","),
		strings.Join(stringMap(nonPrimaryKeyColumns, func(x string) string {
			return ":" + x
		}), ","),
		mapKeyName)
}

func makeDeleteKeyInMapQry(tableName string, mapKeyName string) string {
	return fmt.Sprintf(deleteKeyInMapQryTemplate,
		tableName,
		mapKeyName)
}

func makeGetMapQryTemplate(tableName string, nonPrimaryKeyColumns []string, mapKeyName string) string {
	return fmt.Sprintf(getMapQryTemplate,
		tableName,
		mapKeyName,
		strings.Join(nonPrimaryKeyColumns, ","))
}

var (
	// Omit shard_id, run_id, domain_id, workflow_id, schedule_id since they're in the primary key
	activityInfoColumns = []string{
		"data",
		"data_encoding",
		"last_heartbeat_details",
		"last_heartbeat_updated_time",
	}
	activityInfoTableName = "activity_info_maps"
	activityInfoKey       = "schedule_id"

	deleteActivityInfoMapQry      = makeDeleteMapQry(activityInfoTableName)
	setKeyInActivityInfoMapQry    = makeSetKeyInMapQry(activityInfoTableName, activityInfoColumns, activityInfoKey)
	deleteKeyInActivityInfoMapQry = makeDeleteKeyInMapQry(activityInfoTableName, activityInfoKey)
	getActivityInfoMapQry         = makeGetMapQryTemplate(activityInfoTableName, activityInfoColumns, activityInfoKey)
)

// ReplaceIntoActivityInfoMaps replaces one or more rows in activity_info_maps table
func (mdb *db) ReplaceIntoActivityInfoMaps(ctx context.Context, rows []sqlplugin.ActivityInfoMapsRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(rows[0].ShardID), mdb.GetTotalNumDBShards())
	for i := range rows {
		rows[i].LastHeartbeatUpdatedTime = mdb.converter.ToMySQLDateTime(rows[i].LastHeartbeatUpdatedTime)
	}
	return mdb.driver.NamedExecContext(ctx, dbShardID, setKeyInActivityInfoMapQry, rows)
}

// SelectFromActivityInfoMaps reads one or more rows from activity_info_maps table
func (mdb *db) SelectFromActivityInfoMaps(ctx context.Context, filter *sqlplugin.ActivityInfoMapsFilter) ([]sqlplugin.ActivityInfoMapsRow, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	var rows []sqlplugin.ActivityInfoMapsRow
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getActivityInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	for i := 0; i < len(rows); i++ {
		rows[i].ShardID = int64(filter.ShardID)
		rows[i].DomainID = filter.DomainID
		rows[i].WorkflowID = filter.WorkflowID
		rows[i].RunID = filter.RunID
		rows[i].LastHeartbeatUpdatedTime = mdb.converter.FromMySQLDateTime(rows[i].LastHeartbeatUpdatedTime)
	}
	return rows, err
}

// DeleteFromActivityInfoMaps deletes one or more rows from activity_info_maps table
func (mdb *db) DeleteFromActivityInfoMaps(ctx context.Context, filter *sqlplugin.ActivityInfoMapsFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	if len(filter.ScheduleIDs) > 0 {
		query, args, err := sqlx.In(deleteKeyInActivityInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID, filter.ScheduleIDs)
		if err != nil {
			return nil, err
		}
		return mdb.driver.ExecContext(ctx, dbShardID, sqlx.Rebind(sqlx.BindType(PluginName), query), args...)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, deleteActivityInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

var (
	timerInfoColumns = []string{
		"data",
		"data_encoding",
	}
	timerInfoTableName = "timer_info_maps"
	timerInfoKey       = "timer_id"

	deleteTimerInfoMapSQLQuery      = makeDeleteMapQry(timerInfoTableName)
	setKeyInTimerInfoMapSQLQuery    = makeSetKeyInMapQry(timerInfoTableName, timerInfoColumns, timerInfoKey)
	deleteKeyInTimerInfoMapSQLQuery = makeDeleteKeyInMapQry(timerInfoTableName, timerInfoKey)
	getTimerInfoMapSQLQuery         = makeGetMapQryTemplate(timerInfoTableName, timerInfoColumns, timerInfoKey)
)

// ReplaceIntoTimerInfoMaps replaces one or more rows in timer_info_maps table
func (mdb *db) ReplaceIntoTimerInfoMaps(ctx context.Context, rows []sqlplugin.TimerInfoMapsRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(rows[0].ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, setKeyInTimerInfoMapSQLQuery, rows)
}

// SelectFromTimerInfoMaps reads one or more rows from timer_info_maps table
func (mdb *db) SelectFromTimerInfoMaps(ctx context.Context, filter *sqlplugin.TimerInfoMapsFilter) ([]sqlplugin.TimerInfoMapsRow, error) {
	var rows []sqlplugin.TimerInfoMapsRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getTimerInfoMapSQLQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	for i := 0; i < len(rows); i++ {
		rows[i].ShardID = int64(filter.ShardID)
		rows[i].DomainID = filter.DomainID
		rows[i].WorkflowID = filter.WorkflowID
		rows[i].RunID = filter.RunID
	}
	return rows, err
}

// DeleteFromTimerInfoMaps deletes one or more rows from timer_info_maps table
func (mdb *db) DeleteFromTimerInfoMaps(ctx context.Context, filter *sqlplugin.TimerInfoMapsFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	if len(filter.TimerIDs) > 0 {
		query, args, err := sqlx.In(deleteKeyInTimerInfoMapSQLQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID, filter.TimerIDs)
		if err != nil {
			return nil, err
		}
		return mdb.driver.ExecContext(ctx, dbShardID, sqlx.Rebind(sqlx.BindType(PluginName), query), args...)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, deleteTimerInfoMapSQLQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

var (
	childExecutionInfoColumns = []string{
		"data",
		"data_encoding",
	}
	childExecutionInfoTableName = "child_execution_info_maps"
	childExecutionInfoKey       = "initiated_id"

	deleteChildExecutionInfoMapQry      = makeDeleteMapQry(childExecutionInfoTableName)
	setKeyInChildExecutionInfoMapQry    = makeSetKeyInMapQry(childExecutionInfoTableName, childExecutionInfoColumns, childExecutionInfoKey)
	deleteKeyInChildExecutionInfoMapQry = makeDeleteKeyInMapQry(childExecutionInfoTableName, childExecutionInfoKey)
	getChildExecutionInfoMapQry         = makeGetMapQryTemplate(childExecutionInfoTableName, childExecutionInfoColumns, childExecutionInfoKey)
)

// ReplaceIntoChildExecutionInfoMaps replaces one or more rows in child_execution_info_maps table
func (mdb *db) ReplaceIntoChildExecutionInfoMaps(ctx context.Context, rows []sqlplugin.ChildExecutionInfoMapsRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(rows[0].ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, setKeyInChildExecutionInfoMapQry, rows)
}

// SelectFromChildExecutionInfoMaps reads one or more rows from child_execution_info_maps table
func (mdb *db) SelectFromChildExecutionInfoMaps(ctx context.Context, filter *sqlplugin.ChildExecutionInfoMapsFilter) ([]sqlplugin.ChildExecutionInfoMapsRow, error) {
	var rows []sqlplugin.ChildExecutionInfoMapsRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getChildExecutionInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	for i := 0; i < len(rows); i++ {
		rows[i].ShardID = int64(filter.ShardID)
		rows[i].DomainID = filter.DomainID
		rows[i].WorkflowID = filter.WorkflowID
		rows[i].RunID = filter.RunID
	}
	return rows, err
}

// DeleteFromChildExecutionInfoMaps deletes one or more rows from child_execution_info_maps table
func (mdb *db) DeleteFromChildExecutionInfoMaps(ctx context.Context, filter *sqlplugin.ChildExecutionInfoMapsFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	if len(filter.InitiatedIDs) > 0 {
		query, args, err := sqlx.In(deleteKeyInChildExecutionInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID, filter.InitiatedIDs)
		if err != nil {
			return nil, err
		}
		return mdb.driver.ExecContext(ctx, dbShardID, sqlx.Rebind(sqlx.BindType(PluginName), query), args...)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, deleteChildExecutionInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

var (
	requestCancelInfoColumns = []string{
		"data",
		"data_encoding",
	}
	requestCancelInfoTableName = "request_cancel_info_maps"
	requestCancelInfoKey       = "initiated_id"

	deleteRequestCancelInfoMapQry      = makeDeleteMapQry(requestCancelInfoTableName)
	setKeyInRequestCancelInfoMapQry    = makeSetKeyInMapQry(requestCancelInfoTableName, requestCancelInfoColumns, requestCancelInfoKey)
	deleteKeyInRequestCancelInfoMapQry = makeDeleteKeyInMapQry(requestCancelInfoTableName, requestCancelInfoKey)
	getRequestCancelInfoMapQry         = makeGetMapQryTemplate(requestCancelInfoTableName, requestCancelInfoColumns, requestCancelInfoKey)
)

// ReplaceIntoRequestCancelInfoMaps replaces one or more rows in request_cancel_info_maps table
func (mdb *db) ReplaceIntoRequestCancelInfoMaps(ctx context.Context, rows []sqlplugin.RequestCancelInfoMapsRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(rows[0].ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, setKeyInRequestCancelInfoMapQry, rows)
}

// SelectFromRequestCancelInfoMaps reads one or more rows from request_cancel_info_maps table
func (mdb *db) SelectFromRequestCancelInfoMaps(ctx context.Context, filter *sqlplugin.RequestCancelInfoMapsFilter) ([]sqlplugin.RequestCancelInfoMapsRow, error) {
	var rows []sqlplugin.RequestCancelInfoMapsRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getRequestCancelInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	for i := 0; i < len(rows); i++ {
		rows[i].ShardID = int64(filter.ShardID)
		rows[i].DomainID = filter.DomainID
		rows[i].WorkflowID = filter.WorkflowID
		rows[i].RunID = filter.RunID
	}
	return rows, err
}

// DeleteFromRequestCancelInfoMaps deletes one or more rows from request_cancel_info_maps table
func (mdb *db) DeleteFromRequestCancelInfoMaps(ctx context.Context, filter *sqlplugin.RequestCancelInfoMapsFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	if len(filter.InitiatedIDs) > 0 {
		query, args, err := sqlx.In(deleteKeyInRequestCancelInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID, filter.InitiatedIDs)
		if err != nil {
			return nil, err
		}
		return mdb.driver.ExecContext(ctx, dbShardID, sqlx.Rebind(sqlx.BindType(PluginName), query), args...)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, deleteRequestCancelInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

var (
	signalInfoColumns = []string{
		"data",
		"data_encoding",
	}
	signalInfoTableName = "signal_info_maps"
	signalInfoKey       = "initiated_id"

	deleteSignalInfoMapQry      = makeDeleteMapQry(signalInfoTableName)
	setKeyInSignalInfoMapQry    = makeSetKeyInMapQry(signalInfoTableName, signalInfoColumns, signalInfoKey)
	deleteKeyInSignalInfoMapQry = makeDeleteKeyInMapQry(signalInfoTableName, signalInfoKey)
	getSignalInfoMapQry         = makeGetMapQryTemplate(signalInfoTableName, signalInfoColumns, signalInfoKey)
)

// ReplaceIntoSignalInfoMaps replaces one or more rows in signal_info_maps table
func (mdb *db) ReplaceIntoSignalInfoMaps(ctx context.Context, rows []sqlplugin.SignalInfoMapsRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(rows[0].ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, setKeyInSignalInfoMapQry, rows)
}

// SelectFromSignalInfoMaps reads one or more rows from signal_info_maps table
func (mdb *db) SelectFromSignalInfoMaps(ctx context.Context, filter *sqlplugin.SignalInfoMapsFilter) ([]sqlplugin.SignalInfoMapsRow, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	var rows []sqlplugin.SignalInfoMapsRow
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getSignalInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	for i := 0; i < len(rows); i++ {
		rows[i].ShardID = int64(filter.ShardID)
		rows[i].DomainID = filter.DomainID
		rows[i].WorkflowID = filter.WorkflowID
		rows[i].RunID = filter.RunID
	}
	return rows, err
}

// DeleteFromSignalInfoMaps deletes one or more rows from signal_info_maps table
func (mdb *db) DeleteFromSignalInfoMaps(ctx context.Context, filter *sqlplugin.SignalInfoMapsFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	if len(filter.InitiatedIDs) > 0 {
		query, args, err := sqlx.In(deleteKeyInSignalInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID, filter.InitiatedIDs)
		if err != nil {
			return nil, err
		}
		return mdb.driver.ExecContext(ctx, dbShardID, sqlx.Rebind(sqlx.BindType(PluginName), query), args...)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, deleteSignalInfoMapQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

const (
	deleteAllSignalsRequestedSetQry = `DELETE FROM signals_requested_sets
WHERE
shard_id = ? AND
domain_id = ? AND
workflow_id = ? AND
run_id = ?
`

	createSignalsRequestedSetQry = `INSERT IGNORE INTO signals_requested_sets
(shard_id, domain_id, workflow_id, run_id, signal_id) VALUES
(:shard_id, :domain_id, :workflow_id, :run_id, :signal_id)`

	deleteSignalsRequestedSetQry = `DELETE FROM signals_requested_sets
WHERE
shard_id = ? AND
domain_id = ? AND
workflow_id = ? AND
run_id = ? AND
signal_id IN ( ? )`

	getSignalsRequestedSetQry = `SELECT signal_id FROM signals_requested_sets WHERE
shard_id = ? AND
domain_id = ? AND
workflow_id = ? AND
run_id = ?`
)

// InsertIntoSignalsRequestedSets inserts one or more rows into signals_requested_sets table
func (mdb *db) InsertIntoSignalsRequestedSets(ctx context.Context, rows []sqlplugin.SignalsRequestedSetsRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(rows[0].ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, createSignalsRequestedSetQry, rows)
}

// SelectFromSignalsRequestedSets reads one or more rows from signals_requested_sets table
func (mdb *db) SelectFromSignalsRequestedSets(ctx context.Context, filter *sqlplugin.SignalsRequestedSetsFilter) ([]sqlplugin.SignalsRequestedSetsRow, error) {
	var rows []sqlplugin.SignalsRequestedSetsRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getSignalsRequestedSetQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	for i := 0; i < len(rows); i++ {
		rows[i].ShardID = int64(filter.ShardID)
		rows[i].DomainID = filter.DomainID
		rows[i].WorkflowID = filter.WorkflowID
		rows[i].RunID = filter.RunID
	}
	return rows, err
}

// DeleteFromSignalsRequestedSets deletes one or more rows from signals_requested_sets table
func (mdb *db) DeleteFromSignalsRequestedSets(ctx context.Context, filter *sqlplugin.SignalsRequestedSetsFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	if len(filter.SignalIDs) > 0 {
		query, args, err := sqlx.In(deleteSignalsRequestedSetQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID, filter.SignalIDs)
		if err != nil {
			return nil, err
		}
		return mdb.driver.ExecContext(ctx, dbShardID, sqlx.Rebind(sqlx.BindType(PluginName), query), args...)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, deleteAllSignalsRequestedSetQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}
