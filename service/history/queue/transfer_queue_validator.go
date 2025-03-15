// Copyright (c) 2017-2021 Uber Technologies Inc.

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

package queue

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/task"
)

const (
	defaultMaxPendingTasksSize = 5000
)

type (
	pendingTaskInfo struct {
		executionInfo          *persistence.WorkflowExecutionInfo
		task                   persistence.Task
		potentialFalsePositive bool
	}

	transferQueueValidator struct {
		sync.Mutex

		processor    *transferQueueProcessorBase
		timeSource   clock.TimeSource
		logger       log.Logger
		metricsScope metrics.Scope

		pendingTaskInfos   map[int64]pendingTaskInfo
		maxReadLevels      map[int]task.Key
		minReadTaskID      int64
		lastValidateTime   time.Time
		validationInterval dynamicconfig.DurationPropertyFn
	}
)

func newTransferQueueValidator(
	processor *transferQueueProcessorBase,
	validationInterval dynamicconfig.DurationPropertyFn,
	logger log.Logger,
	metricsScope metrics.Scope,
) *transferQueueValidator {
	timeSource := processor.shard.GetTimeSource()
	return &transferQueueValidator{
		processor:          processor,
		timeSource:         timeSource,
		logger:             logger,
		metricsScope:       metricsScope,
		pendingTaskInfos:   make(map[int64]pendingTaskInfo),
		maxReadLevels:      make(map[int]task.Key),
		minReadTaskID:      0,
		lastValidateTime:   timeSource.Now(),
		validationInterval: validationInterval,
	}
}

func (v *transferQueueValidator) addTasks(info *hcommon.NotifyTaskInfo) {
	v.Lock()
	defer v.Unlock()

	numTaskToAdd := len(info.Tasks)
	if numTaskToAdd+len(v.pendingTaskInfos) > defaultMaxPendingTasksSize {
		numTaskToAdd = defaultMaxPendingTasksSize - len(v.pendingTaskInfos)
		var taskDump strings.Builder
		droppedTasks := info.Tasks[numTaskToAdd:]
		for _, task := range droppedTasks {
			taskDump.WriteString(fmt.Sprintf("%+v\n", task))
		}
		v.logger.Warn(
			"Too many pending transfer tasks, dropping new tasks",
			tag.WorkflowDomainID(info.ExecutionInfo.DomainID),
			tag.WorkflowID(info.ExecutionInfo.WorkflowID),
			tag.WorkflowRunID(info.ExecutionInfo.RunID),
			tag.Key("dropped-transfer-tasks"),
			tag.Value(taskDump.String()),
		)
		v.metricsScope.AddCounter(metrics.QueueValidatorDropTaskCounter, int64(len(droppedTasks)))
	}

	for _, task := range info.Tasks[:numTaskToAdd] {
		// It is possible that a task is acked before it is added to the validator
		// In that case, the lost task could be a potential false positive case
		potentialFalsePositive := info.PersistenceError || task.GetTaskID() <= v.minReadTaskID
		v.pendingTaskInfos[task.GetTaskID()] = pendingTaskInfo{
			executionInfo:          info.ExecutionInfo,
			task:                   task,
			potentialFalsePositive: potentialFalsePositive,
		}
	}
}

func (v *transferQueueValidator) ackTasks(
	queueLevel int,
	readLevel task.Key,
	maxReadLevel task.Key,
	loadedTasks map[task.Key]task.Task,
) {
	v.Lock()
	defer v.Unlock()

	for _, task := range loadedTasks {
		// note that loadedTasks will contain tasks not in pendingTaskInfos
		// either due to the retries when updating mutable state or the fact that we
		// have two processors for the same queue in DB
		delete(v.pendingTaskInfos, task.GetTaskID())
	}

	if queueLevel == defaultProcessingQueueLevel {
		if expectedReadLevel, ok := v.maxReadLevels[queueLevel]; ok && expectedReadLevel.Less(readLevel) {
			// TODO: implement an event logger for queue processor and dump all events when this validation fails.
			v.logger.Error("Transfer queue processor load request is not continuous")
			v.metricsScope.IncCounter(metrics.QueueValidatorInvalidLoadCounter)
		}
	}
	v.maxReadLevels[queueLevel] = maxReadLevel

	if v.timeSource.Now().After(v.lastValidateTime.Add(v.validationInterval())) {
		v.validatePendingTasks()
		v.lastValidateTime = v.timeSource.Now()
	}
}

func (v *transferQueueValidator) validatePendingTasks() {
	v.metricsScope.IncCounter(metrics.QueueValidatorValidationCounter)

	// first find the minimal read level across all processing queue levels
	minReadLevel := maximumTransferTaskKey
	for _, queueCollection := range v.processor.processingQueueCollections {
		if activeQueue := queueCollection.ActiveQueue(); activeQueue != nil {
			minReadLevel = minTaskKey(minReadLevel, activeQueue.State().ReadLevel())
		}
	}

	// all pending tasks with taskID <= minReadLevel will never be loaded,
	// log those tasks, emit metrics, and delete them from pending tasks
	//
	// NOTE: this may contain false positives as when the persistence operation for
	// updating workflow execution times out, task notification will still be sent,
	// but those tasks may not be persisted.
	//
	// As a result, when lost task metric is emitted, first check if there's corresponding
	// persistence operation errors.
	minReadTaskID := minReadLevel.(transferTaskKey).taskID
	for taskID, taskInfo := range v.pendingTaskInfos {
		if taskID <= minReadTaskID {
			v.logger.Error("Failed to load transfer task",
				tag.TaskID(taskID),
				tag.TaskVisibilityTimestamp(taskInfo.task.GetVisibilityTimestamp().UnixNano()),
				tag.FailoverVersion(taskInfo.task.GetVersion()),
				tag.TaskType(taskInfo.task.GetTaskType()),
				tag.WorkflowDomainID(taskInfo.executionInfo.DomainID),
				tag.WorkflowID(taskInfo.executionInfo.WorkflowID),
				tag.WorkflowRunID(taskInfo.executionInfo.RunID),
				tag.Bool(taskInfo.potentialFalsePositive),
			)
			v.metricsScope.IncCounter(metrics.QueueValidatorLostTaskCounter)
			delete(v.pendingTaskInfos, taskID)
		}
	}
	v.minReadTaskID = minReadTaskID
}
