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

package replication

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/shard"
)

const (
	defaultBeginningMessageID = -1
)

var (
	errInvalidCluster = &types.BadRequestError{Message: "Invalid target cluster name."}
)

type (
	// DLQHandler is the interface handles replication DLQ messages
	DLQHandler interface {
		common.Daemon

		GetMessageCount(
			ctx context.Context,
			forceFetch bool,
		) (map[string]int64, error)
		ReadMessages(
			ctx context.Context,
			sourceCluster string,
			lastMessageID int64,
			pageSize int,
			pageToken []byte,
		) ([]*types.ReplicationTask, []*types.ReplicationTaskInfo, []byte, error)
		PurgeMessages(
			ctx context.Context,
			sourceCluster string,
			lastMessageID int64,
		) error
		MergeMessages(
			ctx context.Context,
			sourceCluster string,
			lastMessageID int64,
			pageSize int,
			pageToken []byte,
		) ([]byte, error)
	}

	dlqHandlerImpl struct {
		taskExecutors map[string]TaskExecutor
		shard         shard.Context
		logger        log.Logger
		metricsClient metrics.Client
		done          chan struct{}
		status        int32
		timeSource    clock.TimeSource

		mu           sync.Mutex
		latestCounts map[string]int64
	}
)

var _ DLQHandler = (*dlqHandlerImpl)(nil)

// NewDLQHandler initialize the replication message DLQ handler
func NewDLQHandler(
	shard shard.Context,
	taskExecutors map[string]TaskExecutor,
) DLQHandler {

	if taskExecutors == nil {
		panic("Failed to initialize replication DLQ handler due to nil task executors")
	}

	return &dlqHandlerImpl{
		shard:         shard,
		taskExecutors: taskExecutors,
		logger:        shard.GetLogger(),
		metricsClient: shard.GetMetricsClient(),
		done:          make(chan struct{}),
		timeSource:    clock.NewRealTimeSource(),
	}
}

// Start starts the DLQ handler
func (r *dlqHandlerImpl) Start() {
	if !atomic.CompareAndSwapInt32(&r.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	go r.emitDLQSizeMetricsLoop()
	r.logger.Info("DLQ handler started.")
}

// Stop stops the DLQ handler
func (r *dlqHandlerImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&r.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	r.logger.Debug("DLQ handler shutting down.")
	close(r.done)
}

func (r *dlqHandlerImpl) GetMessageCount(ctx context.Context, forceFetch bool) (map[string]int64, error) {
	if forceFetch || r.latestCounts == nil {
		if err := r.fetchAndEmitMessageCount(ctx); err != nil {
			return nil, err
		}
	}

	return r.latestCounts, nil
}

func (r *dlqHandlerImpl) fetchAndEmitMessageCount(ctx context.Context) error {
	shardID := strconv.Itoa(r.shard.GetShardID())
	result := map[string]int64{}
	for sourceCluster := range r.taskExecutors {
		request := persistence.GetReplicationDLQSizeRequest{SourceClusterName: sourceCluster}
		response, err := r.shard.GetExecutionManager().GetReplicationDLQSize(ctx, &request)
		if err != nil {
			r.logger.Error("failed to get replication DLQ size", tag.Error(err))
			r.metricsClient.Scope(metrics.ReplicationDLQStatsScope).IncCounter(metrics.ReplicationDLQProbeFailed)
			return err
		}
		r.metricsClient.Scope(
			metrics.ReplicationDLQStatsScope,
			metrics.SourceClusterTag(sourceCluster),
			metrics.InstanceTag(shardID),
		).UpdateGauge(metrics.ReplicationDLQSize, float64(response.Size))

		if response.Size > 0 {
			result[sourceCluster] = response.Size
		}
	}

	r.mu.Lock()
	r.latestCounts = result
	r.mu.Unlock()

	return nil
}

func (r *dlqHandlerImpl) emitDLQSizeMetricsLoop() {
	getInterval := func() time.Duration {
		return backoff.JitDuration(
			dlqMetricsEmitTimerInterval,
			dlqMetricsEmitTimerCoefficient,
		)
	}

	timer := r.timeSource.NewTimer(getInterval())
	defer timer.Stop()

	for {
		select {
		case <-timer.Chan():
			r.fetchAndEmitMessageCount(context.Background())
			timer.Reset(getInterval())
		case <-r.done:
			return
		}
	}
}

func (r *dlqHandlerImpl) ReadMessages(
	ctx context.Context,
	sourceCluster string,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]*types.ReplicationTask, []*types.ReplicationTaskInfo, []byte, error) {

	return r.readMessagesWithAckLevel(
		ctx,
		sourceCluster,
		lastMessageID,
		pageSize,
		pageToken,
	)
}

func (r *dlqHandlerImpl) readMessagesWithAckLevel(
	ctx context.Context,
	sourceCluster string,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]*types.ReplicationTask, []*types.ReplicationTaskInfo, []byte, error) {

	resp, err := r.shard.GetExecutionManager().GetReplicationTasksFromDLQ(
		ctx,
		&persistence.GetReplicationTasksFromDLQRequest{
			SourceClusterName: sourceCluster,
			ReadLevel:         defaultBeginningMessageID + 1,
			MaxReadLevel:      lastMessageID + 1,
			BatchSize:         pageSize,
			NextPageToken:     pageToken,
		},
	)
	if err != nil {
		return nil, nil, nil, err
	}

	remoteAdminClient, err := r.shard.GetService().GetClientBean().GetRemoteAdminClient(sourceCluster)
	if err != nil {
		return nil, nil, nil, err
	}

	taskInfo := make([]*types.ReplicationTaskInfo, 0, len(resp.Tasks))
	for _, task := range resp.Tasks {
		ti, err := task.ToInternalReplicationTaskInfo()
		if err != nil {
			return nil, nil, nil, err
		}
		taskInfo = append(taskInfo, ti)
	}
	response := &types.GetDLQReplicationMessagesResponse{}
	if len(taskInfo) > 0 {
		response, err = remoteAdminClient.GetDLQReplicationMessages(
			ctx,
			&types.GetDLQReplicationMessagesRequest{
				TaskInfos: taskInfo,
			},
		)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return response.ReplicationTasks, taskInfo, resp.NextPageToken, nil
}

func (r *dlqHandlerImpl) PurgeMessages(
	ctx context.Context,
	sourceCluster string,
	lastMessageID int64,
) error {

	_, err := r.shard.GetExecutionManager().RangeDeleteReplicationTaskFromDLQ(
		ctx,
		&persistence.RangeDeleteReplicationTaskFromDLQRequest{
			SourceClusterName:    sourceCluster,
			InclusiveBeginTaskID: defaultBeginningMessageID + 1,
			ExclusiveEndTaskID:   lastMessageID + 1,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *dlqHandlerImpl) MergeMessages(
	ctx context.Context,
	sourceCluster string,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]byte, error) {

	if _, ok := r.taskExecutors[sourceCluster]; !ok {
		return nil, errInvalidCluster
	}

	tasks, rawTasks, token, err := r.readMessagesWithAckLevel(
		ctx,
		sourceCluster,
		lastMessageID,
		pageSize,
		pageToken,
	)
	if err != nil {
		return nil, err
	}

	replicationTasks := map[int64]*types.ReplicationTask{}
	for _, task := range tasks {
		replicationTasks[task.SourceTaskID] = task
	}

	lastMessageID = defaultBeginningMessageID
	for _, raw := range rawTasks {
		if task, ok := replicationTasks[raw.TaskID]; ok {
			if _, err := r.taskExecutors[sourceCluster].execute(task, true); err != nil {
				return nil, err
			}
		}

		// If hydrated replication task does not exist in remote cluster - continue merging
		// Record lastMessageID with raw task id, so that they can be purged after.
		if lastMessageID < raw.TaskID {
			lastMessageID = raw.TaskID
		}
	}

	_, err = r.shard.GetExecutionManager().RangeDeleteReplicationTaskFromDLQ(
		ctx,
		&persistence.RangeDeleteReplicationTaskFromDLQRequest{
			SourceClusterName:    sourceCluster,
			InclusiveBeginTaskID: defaultBeginningMessageID + 1,
			ExclusiveEndTaskID:   lastMessageID + 1,
		},
	)
	if err != nil {
		return nil, err
	}
	return token, nil
}
