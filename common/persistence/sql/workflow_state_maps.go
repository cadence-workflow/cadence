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
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

func updateActivityInfos(
	ctx context.Context,
	tx sqlplugin.Tx,
	activityInfos []*persistence.InternalActivityInfo,
	deleteInfos []int64,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	parser serialization.Parser,
) error {

	if len(activityInfos) > 0 {
		rows := make([]sqlplugin.ActivityInfoMapsRow, len(activityInfos))
		for i, activityInfo := range activityInfos {
			scheduledEvent, scheduledEncoding := persistence.FromDataBlob(activityInfo.ScheduledEvent)
			startEvent, startEncoding := persistence.FromDataBlob(activityInfo.StartedEvent)

			info := &serialization.ActivityInfo{
				Version:                  activityInfo.Version,
				ScheduledEventBatchID:    activityInfo.ScheduledEventBatchID,
				ScheduledEvent:           scheduledEvent,
				ScheduledEventEncoding:   scheduledEncoding,
				ScheduledTimestamp:       activityInfo.ScheduledTime,
				StartedID:                activityInfo.StartedID,
				StartedEvent:             startEvent,
				StartedEventEncoding:     startEncoding,
				StartedTimestamp:         activityInfo.StartedTime,
				ActivityID:               activityInfo.ActivityID,
				RequestID:                activityInfo.RequestID,
				ScheduleToStartTimeout:   activityInfo.ScheduleToStartTimeout,
				ScheduleToCloseTimeout:   activityInfo.ScheduleToCloseTimeout,
				StartToCloseTimeout:      activityInfo.StartToCloseTimeout,
				HeartbeatTimeout:         activityInfo.HeartbeatTimeout,
				CancelRequested:          activityInfo.CancelRequested,
				CancelRequestID:          activityInfo.CancelRequestID,
				TimerTaskStatus:          activityInfo.TimerTaskStatus,
				Attempt:                  activityInfo.Attempt,
				TaskList:                 activityInfo.TaskList,
				TaskListKind:             activityInfo.TaskListKind,
				StartedIdentity:          activityInfo.StartedIdentity,
				HasRetryPolicy:           activityInfo.HasRetryPolicy,
				RetryInitialInterval:     activityInfo.InitialInterval,
				RetryBackoffCoefficient:  activityInfo.BackoffCoefficient,
				RetryMaximumInterval:     activityInfo.MaximumInterval,
				RetryExpirationTimestamp: activityInfo.ExpirationTime,
				RetryMaximumAttempts:     activityInfo.MaximumAttempts,
				RetryNonRetryableErrors:  activityInfo.NonRetriableErrors,
				RetryLastFailureReason:   activityInfo.LastFailureReason,
				RetryLastWorkerIdentity:  activityInfo.LastWorkerIdentity,
				RetryLastFailureDetails:  activityInfo.LastFailureDetails,
			}
			blob, err := parser.ActivityInfoToBlob(info)
			if err != nil {
				return err
			}
			rows[i] = sqlplugin.ActivityInfoMapsRow{
				ShardID:                  int64(shardID),
				DomainID:                 domainID,
				WorkflowID:               workflowID,
				RunID:                    runID,
				ScheduleID:               activityInfo.ScheduleID,
				LastHeartbeatUpdatedTime: activityInfo.LastHeartBeatUpdatedTime,
				LastHeartbeatDetails:     activityInfo.Details,
				Data:                     blob.Data,
				DataEncoding:             string(blob.Encoding),
			}
		}

		if _, err := tx.ReplaceIntoActivityInfoMaps(ctx, rows); err != nil {
			return convertCommonErrors(tx, "updateActivityInfos", "Failed to execute update query.", err)
		}
	}

	if len(deleteInfos) > 0 {
		if _, err := tx.DeleteFromActivityInfoMaps(ctx, &sqlplugin.ActivityInfoMapsFilter{
			ShardID:     int64(shardID),
			DomainID:    domainID,
			WorkflowID:  workflowID,
			RunID:       runID,
			ScheduleIDs: deleteInfos,
		}); err != nil {
			return convertCommonErrors(tx, "updateActivityInfos", "Failed to execute delete query.", err)
		}
	}

	return nil
}

func getActivityInfoMap(
	ctx context.Context,
	db sqlplugin.DB,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	parser serialization.Parser,
) (map[int64]*persistence.InternalActivityInfo, error) {

	rows, err := db.SelectFromActivityInfoMaps(ctx, &sqlplugin.ActivityInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	if err != nil && err != sql.ErrNoRows {
		return nil, convertCommonErrors(db, "getActivityInfoMap", "", err)
	}

	ret := make(map[int64]*persistence.InternalActivityInfo)
	for _, row := range rows {
		decoded, err := parser.ActivityInfoFromBlob(row.Data, row.DataEncoding)
		if err != nil {
			return nil, err
		}
		info := &persistence.InternalActivityInfo{
			DomainID:                 row.DomainID.String(),
			ScheduleID:               row.ScheduleID,
			Details:                  row.LastHeartbeatDetails,
			LastHeartBeatUpdatedTime: row.LastHeartbeatUpdatedTime,
			Version:                  decoded.GetVersion(),
			ScheduledEventBatchID:    decoded.GetScheduledEventBatchID(),
			ScheduledEvent:           persistence.NewDataBlob(decoded.ScheduledEvent, constants.EncodingType(decoded.GetScheduledEventEncoding())),
			ScheduledTime:            decoded.GetScheduledTimestamp(),
			StartedID:                decoded.GetStartedID(),
			StartedTime:              decoded.GetStartedTimestamp(),
			ActivityID:               decoded.GetActivityID(),
			RequestID:                decoded.GetRequestID(),
			ScheduleToStartTimeout:   decoded.GetScheduleToStartTimeout(),
			ScheduleToCloseTimeout:   decoded.GetScheduleToCloseTimeout(),
			StartToCloseTimeout:      decoded.GetStartToCloseTimeout(),
			HeartbeatTimeout:         decoded.GetHeartbeatTimeout(),
			CancelRequested:          decoded.GetCancelRequested(),
			CancelRequestID:          decoded.GetCancelRequestID(),
			TimerTaskStatus:          decoded.GetTimerTaskStatus(),
			Attempt:                  decoded.GetAttempt(),
			StartedIdentity:          decoded.GetStartedIdentity(),
			TaskList:                 decoded.GetTaskList(),
			TaskListKind:             decoded.GetTaskListKind(),
			HasRetryPolicy:           decoded.GetHasRetryPolicy(),
			InitialInterval:          decoded.GetRetryInitialInterval(),
			BackoffCoefficient:       decoded.GetRetryBackoffCoefficient(),
			MaximumInterval:          decoded.GetRetryMaximumInterval(),
			ExpirationTime:           decoded.GetRetryExpirationTimestamp(),
			MaximumAttempts:          decoded.GetRetryMaximumAttempts(),
			NonRetriableErrors:       decoded.GetRetryNonRetryableErrors(),
			LastFailureReason:        decoded.GetRetryLastFailureReason(),
			LastWorkerIdentity:       decoded.GetRetryLastWorkerIdentity(),
			LastFailureDetails:       decoded.GetRetryLastFailureDetails(),
		}
		if decoded.StartedEvent != nil {
			info.StartedEvent = persistence.NewDataBlob(decoded.StartedEvent, constants.EncodingType(decoded.GetStartedEventEncoding()))
		}
		ret[row.ScheduleID] = info
	}

	return ret, nil
}

func deleteActivityInfoMap(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) error {

	if _, err := tx.DeleteFromActivityInfoMaps(ctx, &sqlplugin.ActivityInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}); err != nil {
		return convertCommonErrors(tx, "deleteActivityInfoMap", "", err)
	}
	return nil
}

func updateTimerInfos(
	ctx context.Context,
	tx sqlplugin.Tx,
	timerInfos []*persistence.TimerInfo,
	deleteInfos []string,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	parser serialization.Parser,
) error {

	if len(timerInfos) > 0 {
		rows := make([]sqlplugin.TimerInfoMapsRow, len(timerInfos))
		for i, timerInfo := range timerInfos {
			blob, err := parser.TimerInfoToBlob(&serialization.TimerInfo{
				Version:         timerInfo.Version,
				StartedID:       timerInfo.StartedID,
				ExpiryTimestamp: timerInfo.ExpiryTime,
				// TaskID is a misleading variable, it actually serves
				// the purpose of indicating whether a timer task is
				// generated for this timer info
				TaskID: timerInfo.TaskStatus,
			})
			if err != nil {
				return err
			}
			rows[i] = sqlplugin.TimerInfoMapsRow{
				ShardID:      int64(shardID),
				DomainID:     domainID,
				WorkflowID:   workflowID,
				RunID:        runID,
				TimerID:      timerInfo.TimerID,
				Data:         blob.Data,
				DataEncoding: string(blob.Encoding),
			}
		}
		if _, err := tx.ReplaceIntoTimerInfoMaps(ctx, rows); err != nil {
			return convertCommonErrors(tx, "updateTimerInfos", "Failed to execute update query.", err)
		}
	}

	if len(deleteInfos) > 0 {
		if _, err := tx.DeleteFromTimerInfoMaps(ctx, &sqlplugin.TimerInfoMapsFilter{
			ShardID:    int64(shardID),
			DomainID:   domainID,
			WorkflowID: workflowID,
			RunID:      runID,
			TimerIDs:   deleteInfos,
		}); err != nil {
			return convertCommonErrors(tx, "updateTimerInfos", "Failed to execute delete query.", err)
		}
	}

	return nil
}

func getTimerInfoMap(
	ctx context.Context,
	db sqlplugin.DB,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	parser serialization.Parser,
) (map[string]*persistence.TimerInfo, error) {

	rows, err := db.SelectFromTimerInfoMaps(ctx, &sqlplugin.TimerInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	if err != nil && err != sql.ErrNoRows {
		return nil, convertCommonErrors(db, "getTimerInfoMap", "", err)
	}
	ret := make(map[string]*persistence.TimerInfo)
	for _, row := range rows {
		info, err := parser.TimerInfoFromBlob(row.Data, row.DataEncoding)
		if err != nil {
			return nil, err
		}
		ret[row.TimerID] = &persistence.TimerInfo{
			TimerID:    row.TimerID,
			Version:    info.GetVersion(),
			StartedID:  info.GetStartedID(),
			ExpiryTime: info.GetExpiryTimestamp(),
			// TaskID is a misleading variable, it actually serves
			// the purpose of indicating whether a timer task is
			// generated for this timer info
			TaskStatus: info.GetTaskID(),
		}
	}

	return ret, nil
}

func deleteTimerInfoMap(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) error {

	if _, err := tx.DeleteFromTimerInfoMaps(ctx, &sqlplugin.TimerInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}); err != nil {
		return convertCommonErrors(tx, "deleteTimerInfoMap", "", err)
	}
	return nil
}

func updateChildExecutionInfos(
	ctx context.Context,
	tx sqlplugin.Tx,
	childExecutionInfos []*persistence.InternalChildExecutionInfo,
	deleteInfos []int64,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	parser serialization.Parser,
) error {

	if len(childExecutionInfos) > 0 {
		rows := make([]sqlplugin.ChildExecutionInfoMapsRow, len(childExecutionInfos))
		for i, childExecutionInfo := range childExecutionInfos {
			initiateEvent, initiateEncoding := persistence.FromDataBlob(childExecutionInfo.InitiatedEvent)
			startEvent, startEncoding := persistence.FromDataBlob(childExecutionInfo.StartedEvent)

			info := &serialization.ChildExecutionInfo{
				Version:                childExecutionInfo.Version,
				InitiatedEventBatchID:  childExecutionInfo.InitiatedEventBatchID,
				InitiatedEvent:         initiateEvent,
				InitiatedEventEncoding: initiateEncoding,
				StartedEvent:           startEvent,
				StartedEventEncoding:   startEncoding,
				StartedID:              childExecutionInfo.StartedID,
				StartedWorkflowID:      childExecutionInfo.StartedWorkflowID,
				StartedRunID:           serialization.MustParseUUID(childExecutionInfo.StartedRunID),
				CreateRequestID:        childExecutionInfo.CreateRequestID,
				DomainID:               childExecutionInfo.DomainID,
				DomainNameDEPRECATED:   childExecutionInfo.DomainNameDEPRECATED,
				WorkflowTypeName:       childExecutionInfo.WorkflowTypeName,
				ParentClosePolicy:      int32(childExecutionInfo.ParentClosePolicy),
			}
			blob, err := parser.ChildExecutionInfoToBlob(info)
			if err != nil {
				return err
			}
			rows[i] = sqlplugin.ChildExecutionInfoMapsRow{
				ShardID:      int64(shardID),
				DomainID:     domainID,
				WorkflowID:   workflowID,
				RunID:        runID,
				InitiatedID:  childExecutionInfo.InitiatedID,
				Data:         blob.Data,
				DataEncoding: string(blob.Encoding),
			}
		}
		if _, err := tx.ReplaceIntoChildExecutionInfoMaps(ctx, rows); err != nil {
			return convertCommonErrors(tx, "updateChildExecutionInfos", "Failed to execute update query.", err)
		}
	}

	if len(deleteInfos) > 0 {
		if _, err := tx.DeleteFromChildExecutionInfoMaps(ctx, &sqlplugin.ChildExecutionInfoMapsFilter{
			ShardID:      int64(shardID),
			DomainID:     domainID,
			WorkflowID:   workflowID,
			RunID:        runID,
			InitiatedIDs: deleteInfos,
		}); err != nil {
			return convertCommonErrors(tx, "updateChildExecutionInfos", "Failed to execute delete query.", err)
		}
	}

	return nil
}

func getChildExecutionInfoMap(
	ctx context.Context,
	db sqlplugin.DB,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	parser serialization.Parser,
) (map[int64]*persistence.InternalChildExecutionInfo, error) {

	rows, err := db.SelectFromChildExecutionInfoMaps(ctx, &sqlplugin.ChildExecutionInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	if err != nil && err != sql.ErrNoRows {
		return nil, convertCommonErrors(db, "getChildExecutionInfoMap", "", err)
	}

	ret := make(map[int64]*persistence.InternalChildExecutionInfo)
	for _, row := range rows {
		rowInfo, err := parser.ChildExecutionInfoFromBlob(row.Data, row.DataEncoding)
		if err != nil {
			return nil, err
		}
		info := &persistence.InternalChildExecutionInfo{
			InitiatedID:           row.InitiatedID,
			InitiatedEventBatchID: rowInfo.GetInitiatedEventBatchID(),
			Version:               rowInfo.GetVersion(),
			StartedID:             rowInfo.GetStartedID(),
			StartedWorkflowID:     rowInfo.GetStartedWorkflowID(),
			StartedRunID:          serialization.UUID(rowInfo.GetStartedRunID()).String(),
			CreateRequestID:       rowInfo.GetCreateRequestID(),
			DomainID:              rowInfo.GetDomainID(),
			DomainNameDEPRECATED:  rowInfo.GetDomainNameDEPRECATED(),
			WorkflowTypeName:      rowInfo.GetWorkflowTypeName(),
			ParentClosePolicy:     types.ParentClosePolicy(rowInfo.GetParentClosePolicy()),
		}
		if rowInfo.InitiatedEvent != nil {
			info.InitiatedEvent = persistence.NewDataBlob(rowInfo.InitiatedEvent, constants.EncodingType(rowInfo.GetInitiatedEventEncoding()))
		}
		if rowInfo.StartedEvent != nil {
			info.StartedEvent = persistence.NewDataBlob(rowInfo.StartedEvent, constants.EncodingType(rowInfo.GetStartedEventEncoding()))
		}
		ret[row.InitiatedID] = info
	}

	return ret, nil
}

func deleteChildExecutionInfoMap(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) error {

	if _, err := tx.DeleteFromChildExecutionInfoMaps(ctx, &sqlplugin.ChildExecutionInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}); err != nil {
		return convertCommonErrors(tx, "deleteChildExecutionInfoMap", "", err)
	}
	return nil
}

func updateRequestCancelInfos(
	ctx context.Context,
	tx sqlplugin.Tx,
	requestCancelInfos []*persistence.RequestCancelInfo,
	deleteInfos []int64,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	parser serialization.Parser,
) error {

	if len(requestCancelInfos) > 0 {
		rows := make([]sqlplugin.RequestCancelInfoMapsRow, len(requestCancelInfos))
		for i, requestCancelInfo := range requestCancelInfos {
			blob, err := parser.RequestCancelInfoToBlob(&serialization.RequestCancelInfo{
				Version:               requestCancelInfo.Version,
				InitiatedEventBatchID: requestCancelInfo.InitiatedEventBatchID,
				CancelRequestID:       requestCancelInfo.CancelRequestID,
			})
			if err != nil {
				return err
			}
			rows[i] = sqlplugin.RequestCancelInfoMapsRow{
				ShardID:      int64(shardID),
				DomainID:     domainID,
				WorkflowID:   workflowID,
				RunID:        runID,
				InitiatedID:  requestCancelInfo.InitiatedID,
				Data:         blob.Data,
				DataEncoding: string(blob.Encoding),
			}
		}

		if _, err := tx.ReplaceIntoRequestCancelInfoMaps(ctx, rows); err != nil {
			return convertCommonErrors(tx, "updateRequestCancelInfos", "Failed to execute update query.", err)
		}
	}

	if len(deleteInfos) > 0 {
		if _, err := tx.DeleteFromRequestCancelInfoMaps(ctx, &sqlplugin.RequestCancelInfoMapsFilter{
			ShardID:      int64(shardID),
			DomainID:     domainID,
			WorkflowID:   workflowID,
			RunID:        runID,
			InitiatedIDs: deleteInfos,
		}); err != nil {
			return convertCommonErrors(tx, "updateRequestCancelInfos", "Failed to execute delete query.", err)
		}
	}

	return nil
}

func getRequestCancelInfoMap(
	ctx context.Context,
	db sqlplugin.DB,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	parser serialization.Parser,
) (map[int64]*persistence.RequestCancelInfo, error) {

	rows, err := db.SelectFromRequestCancelInfoMaps(ctx, &sqlplugin.RequestCancelInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	if err != nil && err != sql.ErrNoRows {
		return nil, convertCommonErrors(db, "getRequestCancelInfoMap", "", err)
	}

	ret := make(map[int64]*persistence.RequestCancelInfo)
	for _, row := range rows {
		rowInfo, err := parser.RequestCancelInfoFromBlob(row.Data, row.DataEncoding)
		if err != nil {
			return nil, err
		}
		ret[row.InitiatedID] = &persistence.RequestCancelInfo{
			Version:               rowInfo.GetVersion(),
			InitiatedID:           row.InitiatedID,
			InitiatedEventBatchID: rowInfo.GetInitiatedEventBatchID(),
			CancelRequestID:       rowInfo.GetCancelRequestID(),
		}
	}

	return ret, nil
}

func deleteRequestCancelInfoMap(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) error {

	if _, err := tx.DeleteFromRequestCancelInfoMaps(ctx, &sqlplugin.RequestCancelInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}); err != nil {
		return convertCommonErrors(tx, "deleteRequestCancelInfoMap", "", err)
	}
	return nil
}

func updateSignalInfos(
	ctx context.Context,
	tx sqlplugin.Tx,
	signalInfos []*persistence.SignalInfo,
	deleteInfos []int64,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	parser serialization.Parser,
) error {

	if len(signalInfos) > 0 {
		rows := make([]sqlplugin.SignalInfoMapsRow, len(signalInfos))
		for i, signalInfo := range signalInfos {
			blob, err := parser.SignalInfoToBlob(&serialization.SignalInfo{
				Version:               signalInfo.Version,
				InitiatedEventBatchID: signalInfo.InitiatedEventBatchID,
				RequestID:             signalInfo.SignalRequestID,
				Name:                  signalInfo.SignalName,
				Input:                 signalInfo.Input,
				Control:               signalInfo.Control,
			})
			if err != nil {
				return err
			}
			rows[i] = sqlplugin.SignalInfoMapsRow{
				ShardID:      int64(shardID),
				DomainID:     domainID,
				WorkflowID:   workflowID,
				RunID:        runID,
				InitiatedID:  signalInfo.InitiatedID,
				Data:         blob.Data,
				DataEncoding: string(blob.Encoding),
			}
		}

		if _, err := tx.ReplaceIntoSignalInfoMaps(ctx, rows); err != nil {
			return convertCommonErrors(tx, "updateSignalInfos", "Failed to execute update query.", err)
		}
	}

	if len(deleteInfos) > 0 {
		if _, err := tx.DeleteFromSignalInfoMaps(ctx, &sqlplugin.SignalInfoMapsFilter{
			ShardID:      int64(shardID),
			DomainID:     domainID,
			WorkflowID:   workflowID,
			RunID:        runID,
			InitiatedIDs: deleteInfos,
		}); err != nil {
			return convertCommonErrors(tx, "updateSignalInfos", "Failed to execute delete query.", err)
		}
	}

	return nil
}

func getSignalInfoMap(
	ctx context.Context,
	db sqlplugin.DB,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	parser serialization.Parser,
) (map[int64]*persistence.SignalInfo, error) {

	rows, err := db.SelectFromSignalInfoMaps(ctx, &sqlplugin.SignalInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	if err != nil && err != sql.ErrNoRows {
		return nil, convertCommonErrors(db, "getSignalInfoMap", "", err)
	}

	ret := make(map[int64]*persistence.SignalInfo)
	for _, row := range rows {
		rowInfo, err := parser.SignalInfoFromBlob(row.Data, row.DataEncoding)
		if err != nil {
			return nil, err
		}
		ret[row.InitiatedID] = &persistence.SignalInfo{
			Version:               rowInfo.GetVersion(),
			InitiatedID:           row.InitiatedID,
			InitiatedEventBatchID: rowInfo.GetInitiatedEventBatchID(),
			SignalRequestID:       rowInfo.GetRequestID(),
			SignalName:            rowInfo.GetName(),
			Input:                 rowInfo.GetInput(),
			Control:               rowInfo.GetControl(),
		}
	}

	return ret, nil
}

func deleteSignalInfoMap(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) error {

	if _, err := tx.DeleteFromSignalInfoMaps(ctx, &sqlplugin.SignalInfoMapsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}); err != nil {
		return convertCommonErrors(tx, "deleteSignalInfoMap", "", err)
	}
	return nil
}
