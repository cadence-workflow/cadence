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

	"golang.org/x/sync/errgroup"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
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
		config         *config.TaskListConfig
		timeSource     clock.TimeSource
		logger         log.Logger
		scope          metrics.Scope
		matchingClient matching.Client

		taskListType *types.TaskListType
		status       int32
		wg           sync.WaitGroup
		ctx          context.Context
		cancel       func()
		overLoad     clock.Sustain
		underLoad    clock.Sustain
		isolation    *isolationBalancer
		baseEvent    event.E
	}

	aggregatePartitionMetrics struct {
		totalQPS                   float64
		qpsByIsolationGroup        map[string]float64
		hasPollersByIsolationGroup map[string]bool
		byPartition                map[int]*partitionMetrics
		isIsolationEnabled         bool
	}

	partitionMetrics struct {
		qps      float64
		backlog  int64
		readOnly bool
	}
)

func NewAdaptiveScaler(
	taskListID *Identifier,
	tlMgr Manager,
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
		config:         config,
		timeSource:     timeSource,
		logger:         logger.WithTags(tag.ComponentTaskListAdaptiveScaler),
		scope:          scope,
		matchingClient: matchingClient,
		taskListType:   getTaskListType(taskListID.GetType()),
		ctx:            ctx,
		cancel:         cancel,
		isolation:      newIsolationBalancer(timeSource, scope, config),
		overLoad:       clock.NewSustain(timeSource, config.PartitionUpscaleSustainedDuration),
		underLoad:      clock.NewSustain(timeSource, config.PartitionDownscaleSustainedDuration),
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

	partitionConfig := a.getPartitionConfig()
	m, err := a.collectPartitionMetrics(partitionConfig)
	// TODO: Handle this better. Maybe we should allow scaling up but not down if our data is incomplete?
	if err != nil {
		a.underLoad.Reset()
		a.overLoad.Reset()
		a.isolation.reset()
		a.logger.Error("Failed to collect partition metrics", tag.Error(err))
		return
	}
	// adjust the number of write partitions based on qps
	numWritePartitions := a.calculateWritePartitionCount(m.totalQPS, len(partitionConfig.WritePartitions))
	writePartitions, writeChanged := a.adjustWritePartitions(partitionConfig.WritePartitions, numWritePartitions)

	isolationChanged := false
	if m.isIsolationEnabled {
		writePartitions, isolationChanged = a.isolation.adjustWritePartitions(m, writePartitions)
		if isolationChanged {
			a.scope.IncCounter(metrics.IsolationRebalance)
		}
	} else {
		a.isolation.reset()
	}

	// adjustReadPartitions will copy over any changes to the writePartitions, so it needs to happen after we
	// potentially change the isolation group assignments
	readPartitions, readChanged := a.adjustReadPartitions(m, partitionConfig.ReadPartitions, writePartitions)

	e := a.baseEvent
	e.EventName = "AdaptiveScalerCalculationResult"
	e.Payload = map[string]any{
		"NumReadPartitions":  len(readPartitions),
		"NumWritePartitions": len(writePartitions),
		"ReadChanged":        readChanged,
		"WriteChanged":       writeChanged,
		"IsolationChanged":   isolationChanged,
		"QPS":                m.totalQPS,
	}
	event.Log(e)

	if !writeChanged && !readChanged && !isolationChanged {
		return
	}
	newConfig := &types.TaskListPartitionConfig{
		ReadPartitions:  readPartitions,
		WritePartitions: writePartitions,
	}
	a.logger.Info("adaptive scaler is updating number of partitions",
		tag.CurrentQPS(m.totalQPS),
		tag.NumReadPartitions(len(readPartitions)),
		tag.NumWritePartitions(len(writePartitions)),
		tag.ReadChanged(readChanged),
		tag.WriteChanged(writeChanged),
		tag.IsolationChanged(isolationChanged),
		tag.Dynamic("old-partition-config", partitionConfig),
		tag.Dynamic("new-partition-config", newConfig),
	)
	a.scope.IncCounter(metrics.CadenceRequests)
	err = a.tlMgr.UpdateTaskListPartitionConfig(a.ctx, newConfig)
	if err != nil {
		a.logger.Error("failed to update task list partition config", tag.Error(err))
		a.scope.IncCounter(metrics.CadenceFailures)
	}
}

func (a *adaptiveScalerImpl) getPartitionConfig() *types.TaskListPartitionConfig {
	partitionConfig := a.tlMgr.TaskListPartitionConfig()
	if partitionConfig == nil {
		partitionConfig = &types.TaskListPartitionConfig{
			ReadPartitions: map[int]*types.TaskListPartition{
				0: {},
			},
			WritePartitions: map[int]*types.TaskListPartition{
				0: {},
			},
		}
	}
	return partitionConfig
}

func (a *adaptiveScalerImpl) calculateWritePartitionCount(qps float64, numWritePartitions int) int {
	upscaleRps := float64(a.config.PartitionUpscaleRPS())
	partitions := float64(numWritePartitions)
	downscaleFactor := a.config.PartitionDownscaleFactor()
	upscaleThreshold := partitions * upscaleRps
	downscaleThreshold := (partitions - 1) * upscaleRps * downscaleFactor
	a.scope.UpdateGauge(metrics.EstimatedAddTaskQPSGauge, qps)
	a.scope.UpdateGauge(metrics.TaskListPartitionUpscaleThresholdGauge, upscaleThreshold)
	a.scope.UpdateGauge(metrics.TaskListPartitionDownscaleThresholdGauge, downscaleThreshold)

	result := numWritePartitions
	if a.overLoad.CheckAndReset(qps > upscaleThreshold) {
		result = getNumberOfPartitions(qps, upscaleRps)
		a.scope.IncCounter(metrics.PartitionUpscale)
		a.logger.Info("adjust write partitions", tag.CurrentQPS(qps), tag.PartitionUpscaleThreshold(upscaleThreshold), tag.PartitionDownscaleThreshold(downscaleThreshold), tag.PartitionDownscaleFactor(downscaleFactor), tag.CurrentNumWritePartitions(numWritePartitions), tag.NumWritePartitions(result))
	}
	if a.underLoad.CheckAndReset(qps < downscaleThreshold) {
		result = getNumberOfPartitions(qps, upscaleRps)
		a.scope.IncCounter(metrics.PartitionDownscale)
		a.logger.Info("adjust write partitions", tag.CurrentQPS(qps), tag.PartitionUpscaleThreshold(upscaleThreshold), tag.PartitionDownscaleThreshold(downscaleThreshold), tag.PartitionDownscaleFactor(downscaleFactor), tag.CurrentNumWritePartitions(numWritePartitions), tag.NumWritePartitions(result))

	}
	return result
}

func (a *adaptiveScalerImpl) adjustWritePartitions(writePartitions map[int]*types.TaskListPartition, targetWritePartitions int) (map[int]*types.TaskListPartition, bool) {
	if len(writePartitions) == targetWritePartitions {
		return writePartitions, false
	}
	result := make(map[int]*types.TaskListPartition, targetWritePartitions)

	for i := 0; i < targetWritePartitions; i++ {
		if p, ok := writePartitions[i]; ok {
			result[i] = p
		} else {
			result[i] = &types.TaskListPartition{}
		}
	}
	return result, true
}

func (a *adaptiveScalerImpl) adjustReadPartitions(m *aggregatePartitionMetrics, oldReadPartitions map[int]*types.TaskListPartition, newWritePartitions map[int]*types.TaskListPartition) (map[int]*types.TaskListPartition, bool) {
	result := make(map[int]*types.TaskListPartition, len(newWritePartitions))
	changed := false
	for id, p := range oldReadPartitions {
		result[id] = p
	}
	for id, p := range newWritePartitions {
		if _, ok := result[id]; !ok {
			changed = true
		}
		result[id] = p
	}

	for i := len(result) - 1; i >= len(newWritePartitions); i-- {
		p, ok := m.byPartition[i]
		if !ok {
			resp, err := a.describePartition(i)
			if err != nil {
				a.logger.Warn("failed to get partition metrics", tag.WorkflowTaskListName(a.taskListID.GetPartition(i)), tag.Error(err))
				break
			}
			p = toPartitionMetrics(i, resp)
		}
		if p.readOnly && p.backlog == 0 {
			changed = true
			delete(result, i)
			a.scope.IncCounter(metrics.PartitionDrained)
		} else {
			break
		}
	}
	if changed {
		a.logger.Info("adjust read partitions", tag.NumReadPartitions(len(result)), tag.NumWritePartitions(len(newWritePartitions)))
	}
	return result, changed
}

func (a *adaptiveScalerImpl) collectPartitionMetrics(config *types.TaskListPartitionConfig) (*aggregatePartitionMetrics, error) {
	if a.config.EnablePartitionIsolationGroupAssignment() {
		return a.fetchMetricsFromPartitions(config)
	}
	return a.assumeEvenQPS(config)
}

func (a *adaptiveScalerImpl) assumeEvenQPS(config *types.TaskListPartitionConfig) (*aggregatePartitionMetrics, error) {
	resp := a.tlMgr.DescribeTaskList(true)

	totalQPS := resp.TaskListStatus.NewTasksPerSecond * float64(len(config.WritePartitions))

	return &aggregatePartitionMetrics{
		totalQPS:           totalQPS,
		isIsolationEnabled: false,
	}, nil
}

func (a *adaptiveScalerImpl) fetchMetricsFromPartitions(config *types.TaskListPartitionConfig) (*aggregatePartitionMetrics, error) {
	var mutex sync.Mutex
	results := make(map[int]*types.DescribeTaskListResponse, len(config.ReadPartitions))
	g := &errgroup.Group{}
	for p := range config.ReadPartitions {
		partitionID := p
		g.Go(func() (e error) {
			defer func() { log.CapturePanic(recover(), a.logger, &e) }()

			result, e := a.describePartition(partitionID)
			if e != nil {
				a.logger.Warn("failed to get partition metrics", tag.WorkflowTaskListName(a.taskListID.GetPartition(partitionID)), tag.Error(e))
			}
			if result != nil {
				mutex.Lock()
				defer mutex.Unlock()
				results[partitionID] = result
			}
			return e
		})
	}
	err := g.Wait()
	if err != nil {
		return nil, err
	}

	return toAggregateMetrics(results), err
}

func (a *adaptiveScalerImpl) describePartition(partitionID int) (*types.DescribeTaskListResponse, error) {
	if partitionID == 0 {
		return a.tlMgr.DescribeTaskList(true), nil
	}
	return a.matchingClient.DescribeTaskList(a.ctx, &types.MatchingDescribeTaskListRequest{
		DomainUUID: a.taskListID.GetDomainID(),
		DescRequest: &types.DescribeTaskListRequest{
			TaskListType: a.taskListType,
			TaskList: &types.TaskList{
				Name: a.taskListID.GetPartition(partitionID),
				Kind: types.TaskListKindNormal.Ptr(),
			},
			IncludeTaskListStatus: true,
		},
	})
}

func toAggregateMetrics(partitions map[int]*types.DescribeTaskListResponse) *aggregatePartitionMetrics {
	total := 0.0
	byIsolationGroup := make(map[string]float64)
	hasPollersByIsolationGroup := make(map[string]bool)
	byPartition := make(map[int]*partitionMetrics, len(partitions))
	for id, p := range partitions {
		for ig, groupMetrics := range p.TaskListStatus.IsolationGroupMetrics {
			byIsolationGroup[ig] += groupMetrics.NewTasksPerSecond
			hasPollersByIsolationGroup[ig] = hasPollersByIsolationGroup[ig] || groupMetrics.PollerCount > 0
		}
		total += p.TaskListStatus.NewTasksPerSecond

		byPartition[id] = toPartitionMetrics(id, p)
	}
	return &aggregatePartitionMetrics{
		totalQPS:                   total,
		qpsByIsolationGroup:        byIsolationGroup,
		hasPollersByIsolationGroup: hasPollersByIsolationGroup,
		byPartition:                byPartition,
		isIsolationEnabled:         true,
	}
}

func toPartitionMetrics(id int, p *types.DescribeTaskListResponse) *partitionMetrics {
	hasWritePartition := true
	if p.PartitionConfig != nil {
		_, hasWritePartition = p.PartitionConfig.WritePartitions[id]
	}
	return &partitionMetrics{
		qps:      p.TaskListStatus.NewTasksPerSecond,
		backlog:  p.TaskListStatus.BacklogCountHint,
		readOnly: !hasWritePartition,
	}
}

func getTaskListType(taskListType int) *types.TaskListType {
	if taskListType == persistence.TaskListTypeDecision {
		return types.TaskListTypeDecision.Ptr()
	} else if taskListType == persistence.TaskListTypeActivity {
		return types.TaskListTypeActivity.Ptr()
	}
	return nil
}

func getNumberOfPartitions(qps float64, upscaleQPS float64) int {
	p := int(math.Ceil(qps / upscaleQPS))
	if p <= 0 {
		p = 1
	}
	return p
}
