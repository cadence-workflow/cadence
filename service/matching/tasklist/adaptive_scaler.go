// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package tasklist

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/stats"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/event"
)

type (
	AdaptiveScaler interface {
		common.Daemon
	}

	adaptiveScalerImpl struct {
		taskListID     *Identifier
		tlMgr          Manager
		qpsTracker     stats.QPSTracker
		config         *config.TaskListConfig
		timeSource     clock.TimeSource
		logger         log.Logger
		scope          metrics.Scope
		matchingClient matching.Client

		taskListType       *types.TaskListType
		status             int32
		wg                 sync.WaitGroup
		ctx                context.Context
		cancel             func()
		overLoad           bool
		overLoadStartTime  time.Time
		underLoad          bool
		underLoadStartTime time.Time
		baseEvent          event.E
	}
)

func NewAdaptiveScaler(
	taskListID *Identifier,
	tlMgr Manager,
	qpsTracker stats.QPSTracker,
	config *config.TaskListConfig,
	timeSource clock.TimeSource,
	logger log.Logger,
	scope metrics.Scope,
	matchingClient matching.Client,
	baseEvent event.E,
) AdaptiveScaler {
	ctx, cancel := context.WithCancel(context.Background())
	return &adaptiveScalerImpl{
		taskListID:     taskListID,
		tlMgr:          tlMgr,
		qpsTracker:     qpsTracker,
		config:         config,
		timeSource:     timeSource,
		logger:         logger.WithTags(tag.ComponentTaskListAdaptiveScaler),
		scope:          scope,
		matchingClient: matchingClient,
		taskListType:   getTaskListType(taskListID.GetType()),
		ctx:            ctx,
		cancel:         cancel,
		overLoad:       false,
		underLoad:      false,
		baseEvent:      baseEvent,
	}
}

func (a *adaptiveScalerImpl) Start() {
	if !atomic.CompareAndSwapInt32(&a.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	a.logger.Info("adaptive task list scaler state changed", tag.LifeCycleStarted)
	a.wg.Add(1)
	go a.runPeriodicLoop()
}

func (a *adaptiveScalerImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&a.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	a.cancel()
	a.wg.Wait()
	a.logger.Info("adaptive task list scaler state changed", tag.LifeCycleStopped)
}

func (a *adaptiveScalerImpl) runPeriodicLoop() {
	defer a.wg.Done()
	timer := a.timeSource.NewTimer(a.config.AdaptiveScalerUpdateInterval())
	defer timer.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-timer.Chan():
			a.run()
			timer.Reset(a.config.AdaptiveScalerUpdateInterval())
		}
	}
}

func (a *adaptiveScalerImpl) run() {
	if !a.config.EnableAdaptiveScaler() || !a.config.EnableGetNumberOfPartitionsFromCache() {
		return
	}
	qps := a.qpsTracker.QPS()
	partitionConfig := a.getPartitionConfig()
	// adjust the number of write partitions based on qps
	numWritePartitions := a.adjustWritePartitions(qps, partitionConfig.NumWritePartitions)
	// adjust the number of read partitions
	numReadPartitions := a.adjustReadPartitions(partitionConfig.NumReadPartitions, numWritePartitions)

	e := a.baseEvent
	e.EventName = "AdaptiveScalerCalculationResult"
	e.Payload = map[string]any{
		"NumReadPartitions":  numReadPartitions,
		"NumWritePartitions": numWritePartitions,
		"QPS":                qps,
	}
	event.Log(e)

	if numReadPartitions == partitionConfig.NumReadPartitions && numWritePartitions == partitionConfig.NumWritePartitions {
		return
	}
	a.logger.Info("adaptive scaler is updating number of partitions",
		tag.CurrentQPS(qps),
		tag.NumReadPartitions(numReadPartitions),
		tag.NumWritePartitions(numWritePartitions),
		tag.Dynamic("task-list-partition-config", partitionConfig),
	)
	a.scope.IncCounter(metrics.CadenceRequests)
	err := a.tlMgr.UpdateTaskListPartitionConfig(a.ctx, &types.TaskListPartitionConfig{
		NumReadPartitions:  numReadPartitions,
		NumWritePartitions: numWritePartitions,
	})
	if err != nil {
		a.logger.Error("failed to update task list partition config", tag.Error(err))
		a.scope.IncCounter(metrics.CadenceFailures)
	}
}

func (a *adaptiveScalerImpl) getPartitionConfig() *types.TaskListPartitionConfig {
	partitionConfig := a.tlMgr.TaskListPartitionConfig()
	if partitionConfig == nil {
		partitionConfig = &types.TaskListPartitionConfig{
			NumReadPartitions:  1,
			NumWritePartitions: 1,
		}
	}
	return partitionConfig
}

func (a *adaptiveScalerImpl) adjustWritePartitions(qps float64, numWritePartitions int32) int32 {
	upscaleThreshold := float64(a.config.PartitionUpscaleRPS())
	downscaleFactor := a.config.PartitionDownscaleFactor()
	downscaleThreshold := float64(numWritePartitions-1) * upscaleThreshold * downscaleFactor / float64(numWritePartitions)
	a.scope.UpdateGauge(metrics.EstimatedAddTaskQPSGauge, qps)
	a.scope.UpdateGauge(metrics.TaskListPartitionUpscaleThresholdGauge, upscaleThreshold)
	a.scope.UpdateGauge(metrics.TaskListPartitionDownscaleThresholdGauge, downscaleThreshold)

	result := numWritePartitions
	if qps > upscaleThreshold {
		if !a.overLoad {
			a.overLoad = true
			a.overLoadStartTime = a.timeSource.Now()
		} else if a.timeSource.Now().Sub(a.overLoadStartTime) > a.config.PartitionUpscaleSustainedDuration() {
			result = getNumberOfPartitions(numWritePartitions, qps, upscaleThreshold)
			a.logger.Info("adjust write partitions", tag.CurrentQPS(qps), tag.PartitionUpscaleThreshold(upscaleThreshold), tag.PartitionDownscaleThreshold(downscaleThreshold), tag.PartitionDownscaleFactor(downscaleFactor), tag.CurrentNumWritePartitions(numWritePartitions), tag.NumWritePartitions(result))
			a.overLoad = false
		}
	} else {
		a.overLoad = false
	}
	if qps < downscaleThreshold {
		if !a.underLoad {
			a.underLoad = true
			a.underLoadStartTime = a.timeSource.Now()
		} else if a.timeSource.Now().Sub(a.underLoadStartTime) > a.config.PartitionDownscaleSustainedDuration() {
			result = getNumberOfPartitions(numWritePartitions, qps, upscaleThreshold)
			a.logger.Info("adjust write partitions", tag.CurrentQPS(qps), tag.PartitionUpscaleThreshold(upscaleThreshold), tag.PartitionDownscaleThreshold(downscaleThreshold), tag.PartitionDownscaleFactor(downscaleFactor), tag.CurrentNumWritePartitions(numWritePartitions), tag.NumWritePartitions(result))
			a.underLoad = false
		}
	} else {
		a.underLoad = false
	}
	return result
}

func (a *adaptiveScalerImpl) adjustReadPartitions(numReadPartitions, numWritePartitions int32) int32 {
	if numReadPartitions < numWritePartitions {
		a.logger.Info("adjust read partitions", tag.NumReadPartitions(numWritePartitions), tag.NumWritePartitions(numWritePartitions))
		return numWritePartitions
	}
	changed := false
	// check the backlog of the drained partitions
	for i := numReadPartitions - 1; i >= numWritePartitions; i-- {
		resp, err := a.matchingClient.DescribeTaskList(a.ctx, &types.MatchingDescribeTaskListRequest{
			DomainUUID: a.taskListID.GetDomainID(),
			DescRequest: &types.DescribeTaskListRequest{
				TaskListType: a.taskListType,
				TaskList: &types.TaskList{
					Name: a.taskListID.GetPartition(int(i)),
					Kind: types.TaskListKindNormal.Ptr(),
				},
				IncludeTaskListStatus: true,
			},
		})
		if err != nil {
			a.logger.Error("failed to get task list backlog", tag.Error(err))
			break
		}
		nw := int32(1)
		if resp.PartitionConfig != nil {
			nw = resp.PartitionConfig.NumWritePartitions
		}
		// in order to drain a partition, 2 conditions need to be met:
		// 1. the backlog size is 0
		// 2. no task is being added to the partition, which is guaranteed to be true if the partition knows that the number of write partition is less or equal to its partition ID
		if resp.TaskListStatus.GetBacklogCountHint() == 0 && nw <= i {
			// if the partition is drained, we can downscale the number of read partitions
			numReadPartitions = i
			changed = true
		} else {
			break
		}
	}
	if changed {
		a.logger.Info("adjust read partitions", tag.NumReadPartitions(numReadPartitions), tag.NumWritePartitions(numWritePartitions))
	}
	return numReadPartitions
}

func getTaskListType(taskListType int) *types.TaskListType {
	if taskListType == persistence.TaskListTypeDecision {
		return types.TaskListTypeDecision.Ptr()
	} else if taskListType == persistence.TaskListTypeActivity {
		return types.TaskListTypeActivity.Ptr()
	}
	return nil
}

func getNumberOfPartitions(numPartitions int32, qps float64, threshold float64) int32 {
	p := int32(math.Ceil(qps * float64(numPartitions) / threshold))
	if p <= 0 {
		p = 1
	}
	return p
}
