// Copyright (c) 2017-2020 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

	"github.com/uber/cadence/common/constants"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

func updateSignalsRequested(
	ctx context.Context,
	tx sqlplugin.Tx,
	signalRequestedIDs []string,
	deleteSignalRequestIDs []string,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) error {

	if len(signalRequestedIDs) > 0 {
		rows := make([]sqlplugin.SignalsRequestedSetsRow, len(signalRequestedIDs))
		for i, v := range signalRequestedIDs {
			rows[i] = sqlplugin.SignalsRequestedSetsRow{
				ShardID:    int64(shardID),
				DomainID:   domainID,
				WorkflowID: workflowID,
				RunID:      runID,
				SignalID:   v,
			}
		}
		if _, err := tx.InsertIntoSignalsRequestedSets(ctx, rows); err != nil {
			return convertCommonErrors(tx, "updateSignalsRequested", "Failed to execute update query.", err)
		}
	}

	if len(deleteSignalRequestIDs) > 0 {
		if _, err := tx.DeleteFromSignalsRequestedSets(ctx, &sqlplugin.SignalsRequestedSetsFilter{
			ShardID:    int64(shardID),
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
			SignalIDs:  deleteSignalRequestIDs,
		}); err != nil {
			return convertCommonErrors(tx, "updateSignalsRequested", "Failed to execute delete query.", err)
		}
	}

	return nil
}

func getSignalsRequested(
	ctx context.Context,
	db sqlplugin.DB,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) (map[string]struct{}, error) {

	rows, err := db.SelectFromSignalsRequestedSets(ctx, &sqlplugin.SignalsRequestedSetsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	if err != nil && err != sql.ErrNoRows {
		return nil, convertCommonErrors(db, "getSignalsRequested", "", err)
	}
	var ret = make(map[string]struct{})
	for _, s := range rows {
		ret[s.SignalID] = struct{}{}
	}
	return ret, nil
}

func deleteSignalsRequestedSet(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) error {

	if _, err := tx.DeleteFromSignalsRequestedSets(ctx, &sqlplugin.SignalsRequestedSetsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}); err != nil {
		return convertCommonErrors(tx, "deleteSignalsRequestedSet", "", err)
	}
	return nil
}

func updateBufferedEvents(
	ctx context.Context,
	tx sqlplugin.Tx,
	batch *p.DataBlob,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) error {

	if batch == nil {
		return nil
	}
	row := sqlplugin.BufferedEventsRow{
		ShardID:      shardID,
		DomainID:     domainID,
		WorkflowID:   workflowID,
		RunID:        runID,
		Data:         batch.Data,
		DataEncoding: string(batch.Encoding),
	}

	if _, err := tx.InsertIntoBufferedEvents(ctx, []sqlplugin.BufferedEventsRow{row}); err != nil {
		return convertCommonErrors(tx, "updateBufferedEvents", "", err)
	}
	return nil
}

func getBufferedEvents(
	ctx context.Context,
	db sqlplugin.DB,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) ([]*p.DataBlob, error) {

	rows, err := db.SelectFromBufferedEvents(ctx, &sqlplugin.BufferedEventsFilter{
		ShardID:    shardID,
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	if err != nil && err != sql.ErrNoRows {
		return nil, convertCommonErrors(db, "getBufferedEvents", "", err)
	}
	var result []*p.DataBlob
	for _, row := range rows {
		result = append(result, p.NewDataBlob(row.Data, constants.EncodingType(row.DataEncoding)))
	}
	return result, nil
}

func deleteBufferedEvents(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) error {

	if _, err := tx.DeleteFromBufferedEvents(ctx, &sqlplugin.BufferedEventsFilter{
		ShardID:    shardID,
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}); err != nil {
		return convertCommonErrors(tx, "deleteBufferedEvents", "", err)
	}
	return nil
}
