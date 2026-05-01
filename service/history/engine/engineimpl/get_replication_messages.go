// Copyright (c) 2017-2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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

package engineimpl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func (e *historyEngineImpl) GetReplicationMessages(
	ctx context.Context,
	pollingCluster string,
	lastReadMessageID int64,
) (*types.ReplicationMessages, error) {

	scope := metrics.HistoryGetReplicationMessagesScope
	sw := e.metricsClient.StartTimer(scope, metrics.GetReplicationMessagesForShardLatency)
	defer sw.Stop()

	replicationMessages, err := e.replicationAckManager.GetTasks(
		ctx,
		pollingCluster,
		lastReadMessageID,
	)
	if err != nil {
		e.logger.Error("Failed to retrieve replication messages.", tag.Error(err))
		return nil, err
	}

	// Set cluster status for sync shard info
	replicationMessages.SyncShardStatus = &types.SyncShardStatus{
		Timestamp: common.Int64Ptr(e.timeSource.Now().UnixNano()),
	}
	e.logger.Debug("Successfully fetched replication messages.", tag.Counter(len(replicationMessages.ReplicationTasks)), tag.ClusterName(pollingCluster))

	if e.logger.DebugOn() {
		for _, task := range replicationMessages.ReplicationTasks {
			data, err := json.Marshal(task)
			if err != nil {
				e.logger.Error("Failed to marshal replication task.", tag.Error(err))
				continue
			}
			e.logger.Debugf("Replication task: %s", string(data))
		}
	}
	return replicationMessages, nil
}

func (e *historyEngineImpl) GetDLQReplicationMessages(
	ctx context.Context,
	taskInfos []*types.ReplicationTaskInfo,
) ([]*types.ReplicationTask, error) {

	scope := metrics.HistoryGetDLQReplicationMessagesScope
	sw := e.metricsClient.StartTimer(scope, metrics.GetDLQReplicationMessagesLatency)
	defer sw.Stop()

	tasks := make([]*types.ReplicationTask, 0, len(taskInfos))
	for _, taskInfo := range taskInfos {
		t, err := convertToReplicationTask(taskInfo)
		if err != nil {
			e.logger.Error("Failed to convert replication task.", tag.Error(err))
			return nil, err
		}
		task, err := e.replicationHydrator.Hydrate(ctx, t)
		if err != nil {
			e.logger.Error("Failed to fetch DLQ replication messages.", tag.Error(err))
			return nil, err
		}
		if task != nil {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

func convertToReplicationTask(taskInfo *types.ReplicationTaskInfo) (persistence.Task, error) {
	switch taskInfo.TaskType {
	case persistence.ReplicationTaskTypeHistory:
		return &persistence.HistoryReplicationTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID:   taskInfo.DomainID,
				WorkflowID: taskInfo.WorkflowID,
				RunID:      taskInfo.RunID,
			},
			TaskData: persistence.TaskData{
				TaskID:  taskInfo.TaskID,
				Version: taskInfo.Version,
			},
			FirstEventID: taskInfo.FirstEventID,
			NextEventID:  taskInfo.NextEventID,
		}, nil
	case persistence.ReplicationTaskTypeSyncActivity:
		return &persistence.SyncActivityTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID:   taskInfo.DomainID,
				WorkflowID: taskInfo.WorkflowID,
				RunID:      taskInfo.RunID,
			},
			TaskData: persistence.TaskData{
				TaskID:  taskInfo.TaskID,
				Version: taskInfo.Version,
			},
			ScheduledID: taskInfo.ScheduledID,
		}, nil
	case persistence.ReplicationTaskTypeFailoverMarker:
		return &persistence.FailoverMarkerTask{
			DomainID: taskInfo.DomainID,
			TaskData: persistence.TaskData{
				TaskID:  taskInfo.TaskID,
				Version: taskInfo.Version,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported task type: %v", taskInfo.TaskType)
	}
}
