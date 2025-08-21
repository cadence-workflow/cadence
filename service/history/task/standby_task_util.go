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

package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
)

type (
	standbyActionFn     func(context.Context, execution.Context, execution.MutableState) (interface{}, error)
	standbyPostActionFn func(context.Context, persistence.Task, interface{}, log.Logger) error

	standbyCurrentTimeFn func(persistence.Task) (time.Time, error)
)

func standbyTaskPostActionNoOp(
	ctx context.Context,
	taskInfo persistence.Task,
	postActionInfo interface{},
	logger log.Logger,
) error {

	if postActionInfo == nil {
		return nil
	}

	// return error so task processing logic will retry
	logger.Debug("standbyTaskPostActionNoOp return redispatch error so task processing logic will retry",
		tag.WorkflowID(taskInfo.GetWorkflowID()),
		tag.WorkflowRunID(taskInfo.GetRunID()),
		tag.WorkflowDomainID(taskInfo.GetDomainID()),
		tag.TaskID(taskInfo.GetTaskID()),
		tag.TaskType(taskInfo.GetTaskType()),
		tag.FailoverVersion(taskInfo.GetVersion()),
		tag.Timestamp(taskInfo.GetVisibilityTimestamp()))
	return &redispatchError{Reason: fmt.Sprintf("post action is %T", postActionInfo)}
}

func standbyTaskPostActionTaskDiscarded(
	ctx context.Context,
	task persistence.Task,
	postActionInfo interface{},
	logger log.Logger,
) error {

	if postActionInfo == nil {
		return nil
	}

	logger.Error("Discarding standby task due to task being pending for too long.",
		tag.WorkflowID(task.GetWorkflowID()),
		tag.WorkflowRunID(task.GetRunID()),
		tag.WorkflowDomainID(task.GetDomainID()),
		tag.TaskID(task.GetTaskID()),
		tag.TaskType(task.GetTaskType()),
		tag.FailoverVersion(task.GetVersion()),
		tag.Timestamp(task.GetVisibilityTimestamp()))
	return ErrTaskDiscarded
}

type (
	historyResendInfo struct {
		// used by NDC
		lastEventID      *int64
		lastEventVersion *int64
	}

	pushActivityToMatchingInfo struct {
		activityScheduleToStartTimeout int32
		tasklist                       types.TaskList
		partitionConfig                map[string]string
	}

	pushDecisionToMatchingInfo struct {
		decisionScheduleToStartTimeout int32
		tasklist                       types.TaskList
		partitionConfig                map[string]string
	}
)

func newPushActivityToMatchingInfo(
	activityScheduleToStartTimeout int32,
	tasklist types.TaskList,
	partitionConfig map[string]string,
) *pushActivityToMatchingInfo {

	return &pushActivityToMatchingInfo{
		activityScheduleToStartTimeout: activityScheduleToStartTimeout,
		tasklist:                       tasklist,
		partitionConfig:                partitionConfig,
	}
}

func newPushDecisionToMatchingInfo(
	decisionScheduleToStartTimeout int32,
	tasklist types.TaskList,
	partitionConfig map[string]string,
) *pushDecisionToMatchingInfo {

	return &pushDecisionToMatchingInfo{
		decisionScheduleToStartTimeout: decisionScheduleToStartTimeout,
		tasklist:                       tasklist,
		partitionConfig:                partitionConfig,
	}
}

func getHistoryResendInfo(
	mutableState execution.MutableState,
) (*historyResendInfo, error) {

	versionHistories := mutableState.GetVersionHistories()
	if versionHistories == nil {
		return nil, execution.ErrMissingVersionHistories
	}
	currentBranch, err := versionHistories.GetCurrentVersionHistory()
	if err != nil {
		return nil, err
	}
	lastItem, err := currentBranch.GetLastItem()
	if err != nil {
		return nil, err
	}
	return &historyResendInfo{
		lastEventID:      common.Int64Ptr(lastItem.EventID),
		lastEventVersion: common.Int64Ptr(lastItem.Version),
	}, nil
}

func getStandbyPostActionFn(
	logger log.Logger,
	taskInfo persistence.Task,
	standbyNow standbyCurrentTimeFn,
	standbyTaskMissingEventsResendDelay time.Duration,
	standbyTaskMissingEventsDiscardDelay time.Duration,
	fetchHistoryStandbyPostActionFn standbyPostActionFn,
	discardTaskStandbyPostActionFn standbyPostActionFn,
) standbyPostActionFn {

	taskTime := taskInfo.GetVisibilityTimestamp()
	resendTime := taskTime.Add(standbyTaskMissingEventsResendDelay)
	discardTime := taskTime.Add(standbyTaskMissingEventsDiscardDelay)

	tags := []tag.Tag{
		tag.WorkflowID(taskInfo.GetWorkflowID()),
		tag.WorkflowRunID(taskInfo.GetRunID()),
		tag.WorkflowDomainID(taskInfo.GetDomainID()),
		tag.TaskID(taskInfo.GetTaskID()),
		tag.TaskType(int(taskInfo.GetTaskType())),
		tag.Timestamp(taskInfo.GetVisibilityTimestamp()),
	}

	now, err := standbyNow(taskInfo)
	if err != nil {
		tags = append(tags, tag.Error(err))
		logger.Error("getStandbyPostActionFn error getting current time, fallback to standbyTaskPostActionNoOp", tags...)
		return standbyTaskPostActionNoOp
	}

	// now < task start time + StandbyTaskMissingEventsResendDelay
	if now.Before(resendTime) {
		logger.Debug("getStandbyPostActionFn returning standbyTaskPostActionNoOp because now < task start time + StandbyTaskMissingEventsResendDelay", tags...)
		return standbyTaskPostActionNoOp
	}

	// task start time + StandbyTaskMissingEventsResendDelay <= now < task start time + StandbyTaskMissingEventsResendDelay
	if now.Before(discardTime) {
		logger.Debug("getStandbyPostActionFn returning fetchHistoryStandbyPostActionFn because task start time + StandbyTaskMissingEventsResendDelay <= now < task start time + StandbyTaskMissingEventsResendDelay", tags...)
		return fetchHistoryStandbyPostActionFn
	}

	// task start time + StandbyTaskMissingEventsResendDelay <= now
	logger.Debug("getStandbyPostActionFn returning discardTaskStandbyPostActionFn because task start time + StandbyTaskMissingEventsResendDelay <= now", tags...)
	return discardTaskStandbyPostActionFn
}

func getRemoteClusterName(
	ctx context.Context,
	currentCluster string,
	domainCache cache.DomainCache,
	activeClusterMgr activecluster.Manager,
	taskInfo persistence.Task,
) (string, error) {
	domainEntry, err := domainCache.GetDomainByID(taskInfo.GetDomainID())
	if err != nil {
		return "", err
	}

	if domainEntry.GetReplicationConfig().IsActiveActive() {
		resp, err := activeClusterMgr.LookupWorkflow(ctx, taskInfo.GetDomainID(), taskInfo.GetWorkflowID(), taskInfo.GetRunID())
		if err != nil {
			return "", err
		}
		if resp.ClusterName == currentCluster {
			// domain has turned active, retry the task
			return "", errors.New("domain becomes active when processing task as standby")
		}
		return resp.ClusterName, nil
	}

	remoteClusterName := domainEntry.GetReplicationConfig().ActiveClusterName
	if remoteClusterName == currentCluster {
		// domain has turned active, retry the task
		return "", errors.New("domain becomes active when processing task as standby")
	}
	return remoteClusterName, nil
}
