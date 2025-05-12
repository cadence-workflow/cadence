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
	"errors"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/replication"
)

func (e *historyEngineImpl) NotifyNewHistoryEvent(event *events.Notification) {
	e.historyEventNotifier.NotifyNewHistoryEvent(event)
}

func (e *historyEngineImpl) NotifyNewTransferTasks(info *hcommon.NotifyTaskInfo) {
	if len(info.Tasks) == 0 {
		return
	}

	task := info.Tasks[0]
	clusterName, err := e.shard.GetActiveClusterManager().ClusterNameForFailoverVersion(task.GetVersion(), info.ExecutionInfo.DomainID)
	if err == nil {
		transferProcessor, ok := e.queueProcessors[persistence.HistoryTaskCategoryTransfer]
		if !ok {
			e.logger.Error("transfer processor not found", tag.Error(err))
			return
		}
		transferProcessor.NotifyNewTask(clusterName, info)
	}
}

func (e *historyEngineImpl) NotifyNewTimerTasks(info *hcommon.NotifyTaskInfo) {
	if len(info.Tasks) == 0 {
		return
	}

	task := info.Tasks[0]
	clusterName, err := e.shard.GetActiveClusterManager().ClusterNameForFailoverVersion(task.GetVersion(), info.ExecutionInfo.DomainID)
	if err == nil {
		timerProcessor, ok := e.queueProcessors[persistence.HistoryTaskCategoryTimer]
		if !ok {
			e.logger.Error("timer processor not found", tag.Error(err))
			return
		}
		timerProcessor.NotifyNewTask(clusterName, info)
	}
}

func (e *historyEngineImpl) NotifyNewReplicationTasks(info *hcommon.NotifyTaskInfo) {
	for _, task := range info.Tasks {
		hTask, err := hydrateReplicationTask(task, info.ExecutionInfo, info.VersionHistories, info.Activities, info.History)
		if err != nil {
			e.logger.Error("failed to preemptively hydrate replication task", tag.Error(err))
			continue
		}
		e.replicationTaskStore.Put(hTask)
	}
}

func hydrateReplicationTask(
	task persistence.Task,
	exec *persistence.WorkflowExecutionInfo,
	versionHistories *persistence.VersionHistories,
	activities map[int64]*persistence.ActivityInfo,
	history events.PersistedBlobs,
) (*types.ReplicationTask, error) {
	info := persistence.ReplicationTaskInfo{
		DomainID:     exec.DomainID,
		WorkflowID:   exec.WorkflowID,
		RunID:        exec.RunID,
		TaskType:     task.GetTaskType(),
		CreationTime: task.GetVisibilityTimestamp().UnixNano(),
		TaskID:       task.GetTaskID(),
		Version:      task.GetVersion(),
	}

	switch t := task.(type) {
	case *persistence.HistoryReplicationTask:
		info.BranchToken = t.BranchToken
		info.NewRunBranchToken = t.NewRunBranchToken
		info.FirstEventID = t.FirstEventID
		info.NextEventID = t.NextEventID
	case *persistence.SyncActivityTask:
		info.ScheduledID = t.ScheduledID
	case *persistence.FailoverMarkerTask:
		// No specific fields, but supported
	default:
		return nil, errors.New("unknown replication task")
	}

	hydrator := replication.NewImmediateTaskHydrator(
		exec.IsRunning(),
		versionHistories,
		activities,
		history.Find(info.BranchToken, info.FirstEventID),
		history.Find(info.NewRunBranchToken, constants.FirstEventID),
	)

	return hydrator.Hydrate(context.Background(), task)
}
