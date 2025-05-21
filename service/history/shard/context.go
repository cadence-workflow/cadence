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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination context_mock.go -package shard github.com/uber/cadence/history/shard/context Context

package shard

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/resource"
	"github.com/uber/cadence/service/history/simulation"
)

type (
	// Context represents a history engine shard
	Context interface {
		GetShardID() int
		GetService() resource.Resource
		GetExecutionManager() persistence.ExecutionManager
		GetHistoryManager() persistence.HistoryManager
		GetDomainCache() cache.DomainCache
		GetActiveClusterManager() activecluster.Manager
		GetClusterMetadata() cluster.Metadata
		GetConfig() *config.Config
		GetEventsCache() events.Cache
		GetLogger() log.Logger
		GetThrottledLogger() log.Logger
		GetMetricsClient() metrics.Client
		GetTimeSource() clock.TimeSource
		PreviousShardOwnerWasDifferent() bool

		GetEngine() engine.Engine
		SetEngine(engine.Engine)

		GenerateTaskID() (int64, error)
		GenerateTaskIDs(number int) ([]int64, error)

		UpdateIfNeededAndGetQueueMaxReadLevel(category persistence.HistoryTaskCategory, cluster string) persistence.HistoryTaskKey

		SetCurrentTime(cluster string, currentTime time.Time)
		GetCurrentTime(cluster string) time.Time
		GetLastUpdatedTime() time.Time

		GetQueueAckLevel(category persistence.HistoryTaskCategory) persistence.HistoryTaskKey
		UpdateQueueAckLevel(category persistence.HistoryTaskCategory, ackLevel persistence.HistoryTaskKey) error
		GetQueueClusterAckLevel(category persistence.HistoryTaskCategory, cluster string) persistence.HistoryTaskKey
		UpdateQueueClusterAckLevel(category persistence.HistoryTaskCategory, cluster string, ackLevel persistence.HistoryTaskKey) error
		GetQueueState(category persistence.HistoryTaskCategory) (*types.QueueState, error)
		UpdateQueueState(category persistence.HistoryTaskCategory, state *types.QueueState) error

		GetTransferProcessingQueueStates(cluster string) []*types.ProcessingQueueState
		UpdateTransferProcessingQueueStates(cluster string, states []*types.ProcessingQueueState) error

		GetTimerProcessingQueueStates(cluster string) []*types.ProcessingQueueState
		UpdateTimerProcessingQueueStates(cluster string, states []*types.ProcessingQueueState) error

		UpdateFailoverLevel(category persistence.HistoryTaskCategory, failoverID string, level persistence.FailoverLevel) error
		DeleteFailoverLevel(category persistence.HistoryTaskCategory, failoverID string) error
		GetAllFailoverLevels(category persistence.HistoryTaskCategory) map[string]persistence.FailoverLevel

		GetDomainNotificationVersion() int64
		UpdateDomainNotificationVersion(domainNotificationVersion int64) error

		GetWorkflowExecution(ctx context.Context, request *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error)
		CreateWorkflowExecution(ctx context.Context, request *persistence.CreateWorkflowExecutionRequest) (*persistence.CreateWorkflowExecutionResponse, error)
		UpdateWorkflowExecution(ctx context.Context, request *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error)
		ConflictResolveWorkflowExecution(ctx context.Context, request *persistence.ConflictResolveWorkflowExecutionRequest) (*persistence.ConflictResolveWorkflowExecutionResponse, error)
		AppendHistoryV2Events(ctx context.Context, request *persistence.AppendHistoryNodesRequest, domainID string, execution types.WorkflowExecution) (*persistence.AppendHistoryNodesResponse, error)

		ReplicateFailoverMarkers(ctx context.Context, markers []*persistence.FailoverMarkerTask) error
		AddingPendingFailoverMarker(*types.FailoverMarkerAttributes) error
		ValidateAndUpdateFailoverMarkers() ([]*types.FailoverMarkerAttributes, error)
	}

	contextImpl struct {
		resource.Resource

		shardItem            *historyShardsItem
		shardID              int
		rangeID              int64
		executionManager     persistence.ExecutionManager
		activeClusterManager activecluster.Manager
		eventsCache          events.Cache
		closeCallback        func(int, *historyShardsItem)
		closedAt             atomic.Pointer[time.Time]
		config               *config.Config
		logger               log.Logger
		throttledLogger      log.Logger
		engine               engine.Engine

		sync.RWMutex
		lastUpdated                  time.Time
		shardInfo                    *persistence.ShardInfo
		taskSequenceNumber           int64
		maxTaskSequenceNumber        int64
		immediateTaskMaxReadLevel    int64
		scheduledTaskMaxReadLevelMap map[string]time.Time                                                     // cluster -> timerMaxReadLevel
		failoverLevels               map[persistence.HistoryTaskCategory]map[string]persistence.FailoverLevel // category -> uuid -> FailoverLevel

		// exist only in memory
		remoteClusterCurrentTime map[string]time.Time

		// true if previous owner was different from the acquirer's identity.
		previousShardOwnerWasDifferent bool
	}
)

var _ Context = (*contextImpl)(nil)

type ErrShardClosed struct {
	Msg      string
	ClosedAt time.Time
}

var _ error = (*ErrShardClosed)(nil)

func (e *ErrShardClosed) Error() string {
	return e.Msg
}

const (
	TimeBeforeShardClosedIsError = 10 * time.Second
)

const (
	// transfer/cross cluster diff/lag is in terms of taskID, which is calculated based on shard rangeID
	// on shard movement, taskID will increase by around 1 million
	logWarnTransferLevelDiff    = 3000000 // 3 million
	logWarnCrossClusterLevelLag = 3000000 // 3 million
	logWarnTimerLevelDiff       = time.Duration(30 * time.Minute)
	historySizeLogThreshold     = 10 * 1024 * 1024
	minContextTimeout           = 1 * time.Second
)

func (s *contextImpl) GetShardID() int {
	return s.shardID
}

func (s *contextImpl) GetService() resource.Resource {
	return s.Resource
}

func (s *contextImpl) GetExecutionManager() persistence.ExecutionManager {
	return s.executionManager
}

func (s *contextImpl) GetEngine() engine.Engine {
	return s.engine
}

func (s *contextImpl) SetEngine(engine engine.Engine) {
	s.engine = engine
}

func (s *contextImpl) GetActiveClusterManager() activecluster.Manager {
	return s.activeClusterManager
}

func (s *contextImpl) GenerateTaskID() (int64, error) {
	s.Lock()
	defer s.Unlock()

	return s.generateTaskIDLocked()
}

func (s *contextImpl) GenerateTaskIDs(number int) ([]int64, error) {
	s.Lock()
	defer s.Unlock()

	result := []int64{}
	for i := 0; i < number; i++ {
		id, err := s.generateTaskIDLocked()
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}

func (s *contextImpl) UpdateIfNeededAndGetQueueMaxReadLevel(category persistence.HistoryTaskCategory, cluster string) persistence.HistoryTaskKey {
	switch category.Type() {
	case persistence.HistoryTaskCategoryTypeImmediate:
		return s.getImmediateTaskMaxReadLevel()
	case persistence.HistoryTaskCategoryTypeScheduled:
		return s.updateScheduledTaskMaxReadLevel(cluster)
	default:
		s.logger.Fatal("unknown history task category", tag.Dynamic("category-type", category.Type()))
	}
	return persistence.HistoryTaskKey{}
}

func (s *contextImpl) getImmediateTaskMaxReadLevel() persistence.HistoryTaskKey {
	s.RLock()
	defer s.RUnlock()
	return persistence.NewImmediateTaskKey(s.immediateTaskMaxReadLevel)
}

func (s *contextImpl) updateScheduledTaskMaxReadLevel(cluster string) persistence.HistoryTaskKey {
	s.Lock()
	defer s.Unlock()

	currentTime := s.GetTimeSource().Now()
	if cluster != "" && cluster != s.GetClusterMetadata().GetCurrentClusterName() {
		currentTime = s.remoteClusterCurrentTime[cluster]
	}

	newMaxReadLevel := currentTime.Add(s.config.TimerProcessorMaxTimeShift()).Truncate(time.Millisecond)
	if newMaxReadLevel.After(s.scheduledTaskMaxReadLevelMap[cluster]) {
		s.scheduledTaskMaxReadLevelMap[cluster] = newMaxReadLevel
	}
	return persistence.NewHistoryTaskKey(s.scheduledTaskMaxReadLevelMap[cluster], 0)
}

func (s *contextImpl) GetQueueAckLevel(category persistence.HistoryTaskCategory) persistence.HistoryTaskKey {
	s.RLock()
	defer s.RUnlock()

	return s.getQueueAckLevelLocked(category)
}

func (s *contextImpl) getQueueAckLevelLocked(category persistence.HistoryTaskCategory) persistence.HistoryTaskKey {
	switch category {
	case persistence.HistoryTaskCategoryTransfer:
		return persistence.NewImmediateTaskKey(s.shardInfo.TransferAckLevel)
	case persistence.HistoryTaskCategoryTimer:
		return persistence.NewHistoryTaskKey(s.shardInfo.TimerAckLevel, 0)
	case persistence.HistoryTaskCategoryReplication:
		return persistence.NewImmediateTaskKey(s.shardInfo.ReplicationAckLevel)
	default:
		return persistence.HistoryTaskKey{}
	}
}

func (s *contextImpl) UpdateQueueAckLevel(category persistence.HistoryTaskCategory, ackLevel persistence.HistoryTaskKey) error {
	s.Lock()
	defer s.Unlock()

	switch category {
	case persistence.HistoryTaskCategoryTransfer:
		s.shardInfo.TransferAckLevel = ackLevel.GetTaskID()
		// for forward compatibility
		s.shardInfo.QueueStates[int32(persistence.HistoryTaskCategoryIDTransfer)] = &types.QueueState{
			ExclusiveMaxReadLevel: &types.TaskKey{TaskID: ackLevel.GetTaskID() + 1},
		}
	case persistence.HistoryTaskCategoryTimer:
		s.shardInfo.TimerAckLevel = ackLevel.GetScheduledTime()
		// for forward compatibility
		s.shardInfo.QueueStates[int32(persistence.HistoryTaskCategoryIDTimer)] = &types.QueueState{
			ExclusiveMaxReadLevel: &types.TaskKey{ScheduledTimeNano: ackLevel.GetScheduledTime().UnixNano()},
		}
	case persistence.HistoryTaskCategoryReplication:
		s.shardInfo.ReplicationAckLevel = ackLevel.GetTaskID()
	default:
		return fmt.Errorf("unknown history task category: %v", category)
	}
	s.shardInfo.StolenSinceRenew = 0
	return s.updateShardInfoLocked()
}

func (s *contextImpl) GetQueueClusterAckLevel(category persistence.HistoryTaskCategory, cluster string) persistence.HistoryTaskKey {
	s.RLock()
	defer s.RUnlock()

	switch category {
	case persistence.HistoryTaskCategoryTransfer:
		// if we can find corresponding ack level
		if ackLevel, ok := s.shardInfo.ClusterTransferAckLevel[cluster]; ok {
			return persistence.NewImmediateTaskKey(ackLevel)
		}
		// otherwise, default to existing ack level, which belongs to local cluster
		// this can happen if you add more cluster
		return persistence.NewImmediateTaskKey(s.shardInfo.TransferAckLevel)
	case persistence.HistoryTaskCategoryTimer:
		// if we can find corresponding ack level
		if ackLevel, ok := s.shardInfo.ClusterTimerAckLevel[cluster]; ok {
			return persistence.NewHistoryTaskKey(ackLevel, 0)
		}
		// otherwise, default to existing ack level, which belongs to local cluster
		// this can happen if you add more cluster
		return persistence.NewHistoryTaskKey(s.shardInfo.TimerAckLevel, 0)
	case persistence.HistoryTaskCategoryReplication:
		// if we can find corresponding replication level
		if replicationLevel, ok := s.shardInfo.ClusterReplicationLevel[cluster]; ok {
			return persistence.NewImmediateTaskKey(replicationLevel)
		}
		// New cluster always starts from -1
		return persistence.NewImmediateTaskKey(-1)
	default:
		return persistence.HistoryTaskKey{}
	}
}

func (s *contextImpl) UpdateQueueClusterAckLevel(category persistence.HistoryTaskCategory, cluster string, ackLevel persistence.HistoryTaskKey) error {
	s.Lock()
	defer s.Unlock()

	switch category {
	case persistence.HistoryTaskCategoryTransfer:
		s.shardInfo.ClusterTransferAckLevel[cluster] = ackLevel.GetTaskID()
	case persistence.HistoryTaskCategoryTimer:
		s.shardInfo.ClusterTimerAckLevel[cluster] = ackLevel.GetScheduledTime()
	case persistence.HistoryTaskCategoryReplication:
		s.shardInfo.ClusterReplicationLevel[cluster] = ackLevel.GetTaskID()
	default:
		return fmt.Errorf("unknown history task category: %v", category)
	}
	s.shardInfo.StolenSinceRenew = 0
	return s.updateShardInfoLocked()
}

func (s *contextImpl) GetQueueState(category persistence.HistoryTaskCategory) (*types.QueueState, error) {
	s.RLock()
	defer s.RUnlock()
	queueState, ok := s.shardInfo.QueueStates[int32(category.ID())]
	if !ok {
		switch category {
		case persistence.HistoryTaskCategoryTransfer:
			queueState = &types.QueueState{
				ExclusiveMaxReadLevel: &types.TaskKey{TaskID: s.shardInfo.TransferAckLevel + 1},
			}
		case persistence.HistoryTaskCategoryTimer:
			queueState = &types.QueueState{
				ExclusiveMaxReadLevel: &types.TaskKey{ScheduledTimeNano: s.shardInfo.TimerAckLevel.UnixNano()},
			}
		default:
			return nil, fmt.Errorf("unknown history task category: %v", category)
		}
	}
	return queueState, nil
}

func (s *contextImpl) UpdateQueueState(category persistence.HistoryTaskCategory, state *types.QueueState) error {
	s.Lock()
	defer s.Unlock()

	s.shardInfo.QueueStates[int32(category.ID())] = state

	// for backward compatibility
	// we must make sure that there is no task being missed when converting the queue state
	// it's ok to have tasks being processed multiple times
	// for immediate tasks the exclusive level should be converted back to inclusive level
	switch category {
	case persistence.HistoryTaskCategoryTransfer:
		ackLevel := state.ExclusiveMaxReadLevel.TaskID
		for _, virtualQueueState := range state.VirtualQueueStates {
			for _, virtualSliceState := range virtualQueueState.VirtualSliceStates {
				ackLevel = min(ackLevel, virtualSliceState.TaskRange.InclusiveMin.TaskID)
			}
		}
		// exclusive to inclusive
		ackLevel = ackLevel - 1
		s.shardInfo.TransferAckLevel = ackLevel
		for clusterName := range s.GetClusterMetadata().GetEnabledClusterInfo() {
			s.shardInfo.ClusterTransferAckLevel[clusterName] = ackLevel
			if s.shardInfo.TransferProcessingQueueStates.StatesByCluster == nil {
				s.shardInfo.TransferProcessingQueueStates.StatesByCluster = make(map[string][]*types.ProcessingQueueState)
			}
			s.shardInfo.TransferProcessingQueueStates.StatesByCluster[clusterName] = []*types.ProcessingQueueState{
				{
					Level:    common.Int32Ptr(0),
					AckLevel: common.Int64Ptr(ackLevel),
					MaxLevel: common.Int64Ptr(math.MaxInt64),
					DomainFilter: &types.DomainFilter{
						ReverseMatch: true,
					},
				},
			}
		}
	case persistence.HistoryTaskCategoryTimer:
		ackLevel := state.ExclusiveMaxReadLevel.ScheduledTimeNano
		for _, virtualQueueState := range state.VirtualQueueStates {
			for _, virtualSliceState := range virtualQueueState.VirtualSliceStates {
				ackLevel = min(ackLevel, virtualSliceState.TaskRange.InclusiveMin.ScheduledTimeNano)
			}
		}
		s.shardInfo.TimerAckLevel = time.Unix(0, ackLevel)
		for clusterName := range s.GetClusterMetadata().GetEnabledClusterInfo() {
			s.shardInfo.ClusterTimerAckLevel[clusterName] = time.Unix(0, ackLevel)
			if s.shardInfo.TimerProcessingQueueStates.StatesByCluster == nil {
				s.shardInfo.TimerProcessingQueueStates.StatesByCluster = make(map[string][]*types.ProcessingQueueState)
			}
			s.shardInfo.TimerProcessingQueueStates.StatesByCluster[clusterName] = []*types.ProcessingQueueState{
				{
					Level:    common.Int32Ptr(0),
					AckLevel: common.Int64Ptr(ackLevel),
					MaxLevel: common.Int64Ptr(math.MaxInt64),
					DomainFilter: &types.DomainFilter{
						ReverseMatch: true,
					},
				},
			}
		}
	case persistence.HistoryTaskCategoryReplication:
		return fmt.Errorf("replication queue state is not supported")
	}

	s.shardInfo.StolenSinceRenew = 0
	return s.updateShardInfoLocked()
}

func (s *contextImpl) GetTransferProcessingQueueStates(cluster string) []*types.ProcessingQueueState {
	s.RLock()
	defer s.RUnlock()

	// if we can find corresponding processing queue states
	if states, ok := s.shardInfo.TransferProcessingQueueStates.StatesByCluster[cluster]; ok {
		return states
	}

	// check if we can find corresponding ack level
	var ackLevel int64
	var ok bool
	if ackLevel, ok = s.shardInfo.ClusterTransferAckLevel[cluster]; !ok {
		// otherwise, default to existing ack level, which belongs to local cluster
		// this can happen if you add more cluster
		ackLevel = s.shardInfo.TransferAckLevel
	}

	// otherwise, create default queue state based on existing ack level,
	// which belongs to local cluster. this can happen if you add more cluster
	return []*types.ProcessingQueueState{
		{
			Level:    common.Int32Ptr(0),
			AckLevel: common.Int64Ptr(ackLevel),
			MaxLevel: common.Int64Ptr(math.MaxInt64),
			DomainFilter: &types.DomainFilter{
				ReverseMatch: true,
			},
		},
	}
}

func (s *contextImpl) UpdateTransferProcessingQueueStates(cluster string, states []*types.ProcessingQueueState) error {
	s.Lock()
	defer s.Unlock()

	if len(states) == 0 {
		return errors.New("empty transfer processing queue states")
	}

	if s.shardInfo.TransferProcessingQueueStates.StatesByCluster == nil {
		s.shardInfo.TransferProcessingQueueStates.StatesByCluster = make(map[string][]*types.ProcessingQueueState)
	}
	s.shardInfo.TransferProcessingQueueStates.StatesByCluster[cluster] = states

	// for backward compatibility
	ackLevel := states[0].GetAckLevel()
	for _, state := range states {
		ackLevel = min(ackLevel, state.GetAckLevel())
	}
	s.shardInfo.ClusterTransferAckLevel[cluster] = ackLevel

	s.shardInfo.StolenSinceRenew = 0
	return s.updateShardInfoLocked()
}

func (s *contextImpl) GetTimerProcessingQueueStates(cluster string) []*types.ProcessingQueueState {
	s.RLock()
	defer s.RUnlock()

	// if we can find corresponding processing queue states
	if states, ok := s.shardInfo.TimerProcessingQueueStates.StatesByCluster[cluster]; ok {
		return states
	}

	// check if we can find corresponding ack level
	var ackLevel time.Time
	var ok bool
	if ackLevel, ok = s.shardInfo.ClusterTimerAckLevel[cluster]; !ok {
		// otherwise, default to existing ack level, which belongs to local cluster
		// this can happen if you add more cluster
		ackLevel = s.shardInfo.TimerAckLevel
	}

	// otherwise, create default queue state based on existing ack level,
	// which belongs to local cluster. this can happen if you add more cluster
	return []*types.ProcessingQueueState{
		{
			Level:    common.Int32Ptr(0),
			AckLevel: common.Int64Ptr(ackLevel.UnixNano()),
			MaxLevel: common.Int64Ptr(math.MaxInt64),
			DomainFilter: &types.DomainFilter{
				ReverseMatch: true,
			},
		},
	}
}

func (s *contextImpl) UpdateTimerProcessingQueueStates(cluster string, states []*types.ProcessingQueueState) error {
	s.Lock()
	defer s.Unlock()

	if len(states) == 0 {
		return errors.New("empty transfer processing queue states")
	}

	if s.shardInfo.TimerProcessingQueueStates.StatesByCluster == nil {
		s.shardInfo.TimerProcessingQueueStates.StatesByCluster = make(map[string][]*types.ProcessingQueueState)
	}
	s.shardInfo.TimerProcessingQueueStates.StatesByCluster[cluster] = states

	// for backward compatibility
	ackLevel := states[0].GetAckLevel()
	for _, state := range states {
		ackLevel = min(ackLevel, state.GetAckLevel())
	}
	s.shardInfo.ClusterTimerAckLevel[cluster] = time.Unix(0, ackLevel)

	s.shardInfo.StolenSinceRenew = 0
	return s.updateShardInfoLocked()
}

func (s *contextImpl) UpdateFailoverLevel(category persistence.HistoryTaskCategory, failoverID string, level persistence.FailoverLevel) error {
	s.Lock()
	defer s.Unlock()

	if _, ok := s.failoverLevels[category]; !ok {
		s.failoverLevels[category] = make(map[string]persistence.FailoverLevel)
	}
	s.failoverLevels[category][failoverID] = level
	return nil
}

func (s *contextImpl) DeleteFailoverLevel(category persistence.HistoryTaskCategory, failoverID string) error {
	s.Lock()
	defer s.Unlock()

	if levels, ok := s.failoverLevels[category]; ok {
		if level, ok := levels[failoverID]; ok {
			delete(levels, failoverID)
			switch category {
			case persistence.HistoryTaskCategoryTransfer:
				s.GetMetricsClient().RecordTimer(metrics.ShardInfoScope, metrics.ShardInfoTransferFailoverLatencyTimer, time.Since(level.StartTime))
			case persistence.HistoryTaskCategoryTimer:
				s.GetMetricsClient().RecordTimer(metrics.ShardInfoScope, metrics.ShardInfoTimerFailoverLatencyTimer, time.Since(level.StartTime))
			}
			return nil
		}
	}
	return nil
}

func (s *contextImpl) GetAllFailoverLevels(category persistence.HistoryTaskCategory) map[string]persistence.FailoverLevel {
	s.RLock()
	defer s.RUnlock()

	ret := map[string]persistence.FailoverLevel{}
	for k, v := range s.failoverLevels[category] {
		ret[k] = v
	}
	return ret
}

func (s *contextImpl) GetDomainNotificationVersion() int64 {
	s.RLock()
	defer s.RUnlock()

	return s.shardInfo.DomainNotificationVersion
}

func (s *contextImpl) UpdateDomainNotificationVersion(domainNotificationVersion int64) error {
	s.Lock()
	defer s.Unlock()

	s.shardInfo.DomainNotificationVersion = domainNotificationVersion
	return s.updateShardInfoLocked()
}

func (s *contextImpl) GetTimerMaxReadLevel(cluster string) time.Time {
	s.RLock()
	defer s.RUnlock()

	return s.scheduledTaskMaxReadLevelMap[cluster]
}

func (s *contextImpl) UpdateTimerMaxReadLevel(cluster string) time.Time {
	s.Lock()
	defer s.Unlock()

	currentTime := s.GetTimeSource().Now()
	if cluster != "" && cluster != s.GetClusterMetadata().GetCurrentClusterName() {
		currentTime = s.remoteClusterCurrentTime[cluster]
	}

	s.scheduledTaskMaxReadLevelMap[cluster] = currentTime.Add(s.config.TimerProcessorMaxTimeShift()).Truncate(time.Millisecond)
	return s.scheduledTaskMaxReadLevelMap[cluster]
}

func (s *contextImpl) GetWorkflowExecution(
	ctx context.Context,
	request *persistence.GetWorkflowExecutionRequest,
) (*persistence.GetWorkflowExecutionResponse, error) {
	request.RangeID = atomic.LoadInt64(&s.rangeID) // This is to make sure read is not blocked by write, s.rangeID is synced with s.shardInfo.RangeID
	if err := s.closedError(); err != nil {
		return nil, err
	}
	return s.executionManager.GetWorkflowExecution(ctx, request)
}

func (s *contextImpl) CreateWorkflowExecution(
	ctx context.Context,
	request *persistence.CreateWorkflowExecutionRequest,
) (*persistence.CreateWorkflowExecutionResponse, error) {
	if err := s.closedError(); err != nil {
		return nil, err
	}

	ctx, cancel, err := s.ensureMinContextTimeout(ctx)
	if err != nil {
		return nil, err
	}
	if cancel != nil {
		defer cancel()
	}

	domainID := request.NewWorkflowSnapshot.ExecutionInfo.DomainID
	workflowID := request.NewWorkflowSnapshot.ExecutionInfo.WorkflowID

	// do not try to get domain cache within shard lock
	domainEntry, err := s.GetDomainCache().GetDomainByID(domainID)
	if err != nil {
		return nil, err
	}

	s.Lock()
	defer s.Unlock()

	immediateTaskMaxReadLevel := int64(0)
	if err := s.allocateTaskIDsLocked(
		domainEntry,
		workflowID,
		request.NewWorkflowSnapshot.TasksByCategory,
		&immediateTaskMaxReadLevel,
	); err != nil {
		return nil, err
	}

	if err := s.closedError(); err != nil {
		return nil, err
	}
	currentRangeID := s.getRangeID()
	request.RangeID = currentRangeID

	response, err := s.executionManager.CreateWorkflowExecution(ctx, request)
	switch err.(type) {
	case nil:
		// Update MaxReadLevel if write to DB succeeds
		s.updateMaxReadLevelLocked(immediateTaskMaxReadLevel)
		s.logCreateWorkflowExecutionEvents(request)
		return response, nil
	case *types.WorkflowExecutionAlreadyStartedError,
		*persistence.WorkflowExecutionAlreadyStartedError,
		*persistence.CurrentWorkflowConditionFailedError,
		*persistence.DuplicateRequestError,
		*types.ServiceBusyError:
		// No special handling required for these errors
		// We know write to DB fails if these errors are returned
		return nil, err
	case *persistence.ShardOwnershipLostError:
		{
			// Shard is stolen, trigger shutdown of history engine
			s.logger.Warn(
				"Closing shard: CreateWorkflowExecution failed due to stolen shard.",
				tag.Error(err),
			)
			s.closeShard()
			return nil, err
		}
	default:
		{
			// We have no idea if the write failed or will eventually make it to
			// persistence. Increment RangeID to guarantee that subsequent reads
			// will either see that write, or know for certain that it failed.
			// This allows the callers to reliably check the outcome by performing
			// a read.
			err1 := s.renewRangeLocked(false)
			if err1 != nil {
				// At this point we have no choice but to unload the shard, so that it
				// gets a new RangeID when it's reloaded.
				s.logger.Warn(
					"Closing shard: CreateWorkflowExecution failed due to unknown error.",
					tag.Error(err),
				)
				s.closeShard()
			}
			return nil, err
		}
	}
}

func (s *contextImpl) getDefaultEncoding(domainName string) constants.EncodingType {
	return constants.EncodingType(s.config.EventEncodingType(domainName))
}

func (s *contextImpl) UpdateWorkflowExecution(
	ctx context.Context,
	request *persistence.UpdateWorkflowExecutionRequest,
) (*persistence.UpdateWorkflowExecutionResponse, error) {
	if err := s.closedError(); err != nil {
		return nil, err
	}
	ctx, cancel, err := s.ensureMinContextTimeout(ctx)
	if err != nil {
		return nil, err
	}
	if cancel != nil {
		defer cancel()
	}

	domainID := request.UpdateWorkflowMutation.ExecutionInfo.DomainID
	workflowID := request.UpdateWorkflowMutation.ExecutionInfo.WorkflowID

	// do not try to get domain cache within shard lock
	domainEntry, err := s.GetDomainCache().GetDomainByID(domainID)
	if err != nil {
		return nil, err
	}
	request.Encoding = s.getDefaultEncoding(domainEntry.GetInfo().Name)

	s.Lock()
	defer s.Unlock()

	immediateTaskMaxReadLevel := int64(0)
	if err := s.allocateTaskIDsLocked(
		domainEntry,
		workflowID,
		request.UpdateWorkflowMutation.TasksByCategory,
		&immediateTaskMaxReadLevel,
	); err != nil {
		return nil, err
	}
	if request.NewWorkflowSnapshot != nil {
		if err := s.allocateTaskIDsLocked(
			domainEntry,
			workflowID,
			request.NewWorkflowSnapshot.TasksByCategory,
			&immediateTaskMaxReadLevel,
		); err != nil {
			return nil, err
		}
	}

	if err := s.closedError(); err != nil {
		return nil, err
	}
	currentRangeID := s.getRangeID()
	request.RangeID = currentRangeID

	resp, err := s.executionManager.UpdateWorkflowExecution(ctx, request)
	switch err.(type) {
	case nil:
		// Update MaxReadLevel if write to DB succeeds
		s.updateMaxReadLevelLocked(immediateTaskMaxReadLevel)
		s.logUpdateWorkflowExecutionEvents(request)
		return resp, nil
	case *persistence.ConditionFailedError,
		*persistence.DuplicateRequestError,
		*types.ServiceBusyError:
		// No special handling required for these errors
		// We know write to DB fails if these errors are returned
		return nil, err
	case *persistence.ShardOwnershipLostError:
		{
			// Shard is stolen, trigger shutdown of history engine
			s.logger.Warn(
				"Closing shard: UpdateWorkflowExecution failed due to stolen shard.",
				tag.Error(err),
			)
			s.closeShard()
			return nil, err
		}
	default:
		{
			// We have no idea if the write failed or will eventually make it to
			// persistence. Increment RangeID to guarantee that subsequent reads
			// will either see that write, or know for certain that it failed.
			// This allows the callers to reliably check the outcome by performing
			// a read.
			err1 := s.renewRangeLocked(false)
			if err1 != nil {
				// At this point we have no choice but to unload the shard, so that it
				// gets a new RangeID when it's reloaded.
				s.logger.Warn(
					"Closing shard: UpdateWorkflowExecution failed due to unknown error.",
					tag.Error(err),
				)
				s.closeShard()
			}
			return nil, err
		}
	}
}

func (s *contextImpl) ConflictResolveWorkflowExecution(
	ctx context.Context,
	request *persistence.ConflictResolveWorkflowExecutionRequest,
) (*persistence.ConflictResolveWorkflowExecutionResponse, error) {
	if err := s.closedError(); err != nil {
		return nil, err
	}

	ctx, cancel, err := s.ensureMinContextTimeout(ctx)
	if err != nil {
		return nil, err
	}
	if cancel != nil {
		defer cancel()
	}

	domainID := request.ResetWorkflowSnapshot.ExecutionInfo.DomainID
	workflowID := request.ResetWorkflowSnapshot.ExecutionInfo.WorkflowID

	// do not try to get domain cache within shard lock
	domainEntry, err := s.GetDomainCache().GetDomainByID(domainID)
	if err != nil {
		return nil, err
	}
	request.Encoding = s.getDefaultEncoding(domainEntry.GetInfo().Name)

	s.Lock()
	defer s.Unlock()

	immediateTaskMaxReadLevel := int64(0)
	if request.CurrentWorkflowMutation != nil {
		if err := s.allocateTaskIDsLocked(
			domainEntry,
			workflowID,
			request.CurrentWorkflowMutation.TasksByCategory,
			&immediateTaskMaxReadLevel,
		); err != nil {
			return nil, err
		}
	}
	if err := s.allocateTaskIDsLocked(
		domainEntry,
		workflowID,
		request.ResetWorkflowSnapshot.TasksByCategory,
		&immediateTaskMaxReadLevel,
	); err != nil {
		return nil, err
	}
	if request.NewWorkflowSnapshot != nil {
		if err := s.allocateTaskIDsLocked(
			domainEntry,
			workflowID,
			request.NewWorkflowSnapshot.TasksByCategory,
			&immediateTaskMaxReadLevel,
		); err != nil {
			return nil, err
		}
	}

	if err := s.closedError(); err != nil {
		return nil, err
	}
	currentRangeID := s.getRangeID()
	request.RangeID = currentRangeID
	resp, err := s.executionManager.ConflictResolveWorkflowExecution(ctx, request)
	switch err.(type) {
	case nil:
		// Update MaxReadLevel if write to DB succeeds
		s.updateMaxReadLevelLocked(immediateTaskMaxReadLevel)
		s.logConflictResolveWorkflowExecutionEvents(request)
		return resp, nil
	case *persistence.ConditionFailedError,
		*types.ServiceBusyError:
		// No special handling required for these errors
		// We know write to DB fails if these errors are returned
		return nil, err
	case *persistence.ShardOwnershipLostError:
		{
			// RangeID might have been renewed by the same host while this update was in flight
			// Retry the operation if we still have the shard ownership
			// Shard is stolen, trigger shutdown of history engine
			s.logger.Warn(
				"Closing shard: ConflictResolveWorkflowExecution failed due to stolen shard.",
				tag.Error(err),
			)
			s.closeShard()
			return nil, err
		}
	default:
		{
			// We have no idea if the write failed or will eventually make it to
			// persistence. Increment RangeID to guarantee that subsequent reads
			// will either see that write, or know for certain that it failed.
			// This allows the callers to reliably check the outcome by performing
			// a read.
			err1 := s.renewRangeLocked(false)
			if err1 != nil {
				// At this point we have no choice but to unload the shard, so that it
				// gets a new RangeID when it's reloaded.
				s.logger.Warn(
					"Closing shard: ConflictResolveWorkflowExecution failed due to unknown error.",
					tag.Error(err),
				)
				s.closeShard()
			}
			return nil, err
		}
	}
}

func (s *contextImpl) ensureMinContextTimeout(
	parent context.Context,
) (context.Context, context.CancelFunc, error) {
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}

	deadline, ok := parent.Deadline()
	if !ok || deadline.Sub(s.GetTimeSource().Now()) >= minContextTimeout {
		return parent, nil, nil
	}

	childCtx, cancel := context.WithTimeout(context.Background(), minContextTimeout)
	return childCtx, cancel, nil
}

func (s *contextImpl) AppendHistoryV2Events(
	ctx context.Context,
	request *persistence.AppendHistoryNodesRequest,
	domainID string,
	execution types.WorkflowExecution,
) (*persistence.AppendHistoryNodesResponse, error) {
	if err := s.closedError(); err != nil {
		return nil, err
	}

	domainName, err := s.GetDomainCache().GetDomainName(domainID)
	if err != nil {
		return nil, err
	}

	// NOTE: do not use generateNextTransferTaskIDLocked since
	// generateNextTransferTaskIDLocked is not guarded by lock
	transactionID, err := s.GenerateTaskID()
	if err != nil {
		return nil, err
	}

	request.Encoding = s.getDefaultEncoding(domainName)
	request.ShardID = common.IntPtr(s.shardID)
	request.TransactionID = transactionID

	size := 0
	defer func() {
		s.GetMetricsClient().Scope(metrics.SessionSizeStatsScope, metrics.DomainTag(domainName)).
			RecordTimer(metrics.HistorySize, time.Duration(size))
		if size >= historySizeLogThreshold {
			s.throttledLogger.Warn("history size threshold breached",
				tag.WorkflowID(execution.GetWorkflowID()),
				tag.WorkflowRunID(execution.GetRunID()),
				tag.WorkflowDomainID(domainID),
				tag.WorkflowHistorySizeBytes(size))
		}
	}()
	resp, err0 := s.GetHistoryManager().AppendHistoryNodes(ctx, request)
	if resp != nil {
		size = len(resp.DataBlob.Data)
	}
	return resp, err0
}

func (s *contextImpl) GetConfig() *config.Config {
	return s.config
}

func (s *contextImpl) PreviousShardOwnerWasDifferent() bool {
	return s.previousShardOwnerWasDifferent
}

func (s *contextImpl) GetEventsCache() events.Cache {
	// the shard needs to be restarted to release the shard cache once global mode is on.
	if s.config.EventsCacheGlobalEnable() {
		return s.GetEventCache()
	}
	return s.eventsCache
}

func (s *contextImpl) GetLogger() log.Logger {
	return s.logger
}

func (s *contextImpl) GetThrottledLogger() log.Logger {
	return s.throttledLogger
}

func (s *contextImpl) getRangeID() int64 {
	return s.shardInfo.RangeID
}

func (s *contextImpl) closedError() error {
	closedAt := s.closedAt.Load()
	if closedAt == nil {
		return nil
	}

	return &ErrShardClosed{
		Msg:      "shard closed",
		ClosedAt: *closedAt,
	}
}

func (s *contextImpl) closeShard() {
	if !s.closedAt.CompareAndSwap(nil, common.TimePtr(time.Now())) {
		return
	}

	s.logger.Info("Shard context closeShard called")
	go func() {
		s.closeCallback(s.shardID, s.shardItem)
	}()

	// fails any writes that may start after this point.
	s.shardInfo.RangeID = -1
	atomic.StoreInt64(&s.rangeID, s.shardInfo.RangeID)
}

func (s *contextImpl) generateTaskIDLocked() (int64, error) {
	if err := s.updateRangeIfNeededLocked(); err != nil {
		return -1, err
	}

	taskID := s.taskSequenceNumber
	s.taskSequenceNumber++

	return taskID, nil
}

func (s *contextImpl) updateRangeIfNeededLocked() error {
	if s.taskSequenceNumber < s.maxTaskSequenceNumber {
		return nil
	}

	return s.renewRangeLocked(false)
}

func (s *contextImpl) renewRangeLocked(isStealing bool) error {
	updatedShardInfo := s.shardInfo.ToNilSafeCopy()
	updatedShardInfo.RangeID++
	if isStealing {
		updatedShardInfo.StolenSinceRenew++
	}

	var err error
	if err := s.closedError(); err != nil {
		return err
	}
	err = s.GetShardManager().UpdateShard(context.Background(), &persistence.UpdateShardRequest{
		ShardInfo:       updatedShardInfo,
		PreviousRangeID: s.shardInfo.RangeID})
	switch err.(type) {
	case nil:
	case *persistence.ShardOwnershipLostError:
		// Shard is stolen, trigger history engine shutdown
		s.logger.Warn(
			"Closing shard: renewRangeLocked failed due to stolen shard.",
			tag.Error(err),
		)
		s.closeShard()
	default:
		s.logger.Warn("UpdateShard failed with an unknown error.",
			tag.Error(err),
			tag.ShardRangeID(updatedShardInfo.RangeID),
			tag.PreviousShardRangeID(s.shardInfo.RangeID))
	}
	if err != nil {
		// Failure in updating shard to grab new RangeID
		s.logger.Error("renewRangeLocked failed.",
			tag.StoreOperationUpdateShard,
			tag.Error(err),
			tag.ShardRangeID(updatedShardInfo.RangeID),
			tag.PreviousShardRangeID(s.shardInfo.RangeID))
		return err
	}

	// Range is successfully updated in cassandra now update shard context to reflect new range
	s.taskSequenceNumber = updatedShardInfo.RangeID << s.config.RangeSizeBits
	s.maxTaskSequenceNumber = (updatedShardInfo.RangeID + 1) << s.config.RangeSizeBits
	s.immediateTaskMaxReadLevel = s.taskSequenceNumber - 1
	atomic.StoreInt64(&s.rangeID, updatedShardInfo.RangeID)
	s.shardInfo = updatedShardInfo

	s.logger.Info("Range updated for shardID",
		tag.ShardRangeID(s.shardInfo.RangeID),
		tag.Number(s.taskSequenceNumber),
		tag.NextNumber(s.maxTaskSequenceNumber))
	return nil
}

func (s *contextImpl) updateMaxReadLevelLocked(rl int64) {
	if rl > s.immediateTaskMaxReadLevel {
		s.logger.Debug(fmt.Sprintf("Updating MaxReadLevel: %v", rl))
		s.immediateTaskMaxReadLevel = rl
	}
}

func (s *contextImpl) updateShardInfoLocked() error {
	return s.persistShardInfoLocked(false)
}

func (s *contextImpl) forceUpdateShardInfoLocked() error {
	return s.persistShardInfoLocked(true)
}

func (s *contextImpl) persistShardInfoLocked(
	isForced bool,
) error {

	if err := s.closedError(); err != nil {
		return err
	}

	var err error
	now := clock.NewRealTimeSource().Now()
	if !isForced && s.lastUpdated.Add(s.config.ShardUpdateMinInterval()).After(now) {
		return nil
	}
	updatedShardInfo := s.shardInfo.ToNilSafeCopy()
	s.emitShardInfoMetricsLogsLocked()

	err = s.GetShardManager().UpdateShard(context.Background(), &persistence.UpdateShardRequest{
		ShardInfo:       updatedShardInfo,
		PreviousRangeID: s.shardInfo.RangeID,
	})

	if err != nil {
		// Shard is stolen, trigger history engine shutdown
		if _, ok := err.(*persistence.ShardOwnershipLostError); ok {
			s.logger.Warn(
				"Closing shard: updateShardInfoLocked failed due to stolen shard.",
				tag.Error(err),
			)
			s.closeShard()
		}
	} else {
		s.lastUpdated = now
	}

	return err
}

func (s *contextImpl) emitShardInfoMetricsLogsLocked() {
	currentCluster := s.GetClusterMetadata().GetCurrentClusterName()
	clusterInfo := s.GetClusterMetadata().GetAllClusterInfo()

	minTransferLevel := s.shardInfo.ClusterTransferAckLevel[currentCluster]
	maxTransferLevel := s.shardInfo.ClusterTransferAckLevel[currentCluster]
	for clusterName, v := range s.shardInfo.ClusterTransferAckLevel {
		if !clusterInfo[clusterName].Enabled {
			continue
		}

		if v < minTransferLevel {
			minTransferLevel = v
		}
		if v > maxTransferLevel {
			maxTransferLevel = v
		}
	}
	diffTransferLevel := maxTransferLevel - minTransferLevel

	minTimerLevel := s.shardInfo.ClusterTimerAckLevel[currentCluster]
	maxTimerLevel := s.shardInfo.ClusterTimerAckLevel[currentCluster]
	for clusterName, v := range s.shardInfo.ClusterTimerAckLevel {
		if !clusterInfo[clusterName].Enabled {
			continue
		}

		if v.Before(minTimerLevel) {
			minTimerLevel = v
		}
		if v.After(maxTimerLevel) {
			maxTimerLevel = v
		}
	}
	diffTimerLevel := maxTimerLevel.Sub(minTimerLevel)

	replicationLag := s.immediateTaskMaxReadLevel - s.shardInfo.ReplicationAckLevel
	transferLag := s.immediateTaskMaxReadLevel - s.shardInfo.TransferAckLevel
	timerLag := time.Since(s.shardInfo.TimerAckLevel)

	transferFailoverInProgress := len(s.failoverLevels[persistence.HistoryTaskCategoryTransfer])
	timerFailoverInProgress := len(s.failoverLevels[persistence.HistoryTaskCategoryTimer])

	if s.config.EmitShardDiffLog() &&
		(logWarnTransferLevelDiff < diffTransferLevel ||
			logWarnTimerLevelDiff < diffTimerLevel ||
			logWarnTransferLevelDiff < transferLag ||
			logWarnTimerLevelDiff < timerLag) {

		logger := s.logger.WithTags(
			tag.ShardTime(s.remoteClusterCurrentTime),
			tag.ShardReplicationAck(s.shardInfo.ReplicationAckLevel),
			tag.ShardTimerAcks(s.shardInfo.ClusterTimerAckLevel),
			tag.ShardTransferAcks(s.shardInfo.ClusterTransferAckLevel),
		)

		logger.Warn("Shard ack levels diff exceeds warn threshold.")
	}

	metricsScope := s.GetMetricsClient().Scope(metrics.ShardInfoScope)
	metricsScope.RecordTimer(metrics.ShardInfoTransferDiffTimer, time.Duration(diffTransferLevel))
	metricsScope.RecordTimer(metrics.ShardInfoTimerDiffTimer, diffTimerLevel)

	metricsScope.RecordTimer(metrics.ShardInfoReplicationLagTimer, time.Duration(replicationLag))
	metricsScope.RecordTimer(metrics.ShardInfoTransferLagTimer, time.Duration(transferLag))
	metricsScope.RecordTimer(metrics.ShardInfoTimerLagTimer, timerLag)

	metricsScope.RecordTimer(metrics.ShardInfoTransferFailoverInProgressTimer, time.Duration(transferFailoverInProgress))
	metricsScope.RecordTimer(metrics.ShardInfoTimerFailoverInProgressTimer, time.Duration(timerFailoverInProgress))
}

func (s *contextImpl) allocateTaskIDsLocked(
	domainEntry *cache.DomainCacheEntry,
	workflowID string,
	tasksByCategory map[persistence.HistoryTaskCategory][]persistence.Task,
	immediateTaskMaxReadLevel *int64,
) error {
	var err error
	var replicationTasks []persistence.Task
	for c, tasks := range tasksByCategory {
		switch c.Type() {
		case persistence.HistoryTaskCategoryTypeImmediate:
			if c.ID() == persistence.HistoryTaskCategoryIDReplication {
				replicationTasks = tasks
				continue
			}
			err = s.allocateTransferIDsLocked(tasks, immediateTaskMaxReadLevel)
		case persistence.HistoryTaskCategoryTypeScheduled:
			err = s.allocateTimerIDsLocked(domainEntry, workflowID, tasks)
		}
		if err != nil {
			return err
		}
	}

	// Ensure that task IDs for replication tasks are generated last.
	// This allows optimizing replication by checking whether there no potential tasks to read.
	return s.allocateTransferIDsLocked(
		replicationTasks,
		immediateTaskMaxReadLevel,
	)
}

func (s *contextImpl) allocateTransferIDsLocked(
	tasks []persistence.Task,
	immediateTaskMaxReadLevel *int64,
) error {
	now := s.GetTimeSource().Now()

	for _, task := range tasks {
		id, err := s.generateTaskIDLocked()
		if err != nil {
			return err
		}
		s.logger.Debug(fmt.Sprintf("Assigning task ID: %v", id))
		task.SetTaskID(id)
		// only set task visibility timestamp if it's not set
		if task.GetVisibilityTimestamp().IsZero() {
			task.SetVisibilityTimestamp(now)
		}
		*immediateTaskMaxReadLevel = id
	}
	return nil
}

// NOTE: allocateTimerIDsLocked should always been called after assigning taskID for transferTasks when assigning taskID together,
// because Cadence Indexer assume timer taskID of deleteWorkflowExecution is larger than transfer taskID of closeWorkflowExecution
// for a given workflow.
// TODO(active-active): Write unit tests for this. It's missing tests for both active-active and active-passive.
func (s *contextImpl) allocateTimerIDsLocked(
	domainEntry *cache.DomainCacheEntry,
	workflowID string,
	timerTasks []persistence.Task,
) error {

	// assign IDs for the timer tasks. They need to be assigned under shard lock.
	cluster := s.GetClusterMetadata().GetCurrentClusterName()
	for _, task := range timerTasks {
		ts := task.GetVisibilityTimestamp()
		if task.GetVersion() != constants.EmptyVersion {
			// cannot use version to determine the corresponding cluster for timer task
			// this is because during failover, timer task should be created as active
			// or otherwise, failover + active processing logic may not pick up the task.
			cluster = domainEntry.GetReplicationConfig().ActiveClusterName

			// if domain is active-active, lookup the workflow to determine the corresponding cluster
			if domainEntry.GetReplicationConfig().IsActiveActive() {
				lookupRes, err := s.GetActiveClusterManager().LookupWorkflow(context.Background(), task.GetDomainID(), task.GetWorkflowID(), task.GetRunID())
				if err != nil {
					return err
				}
				cluster = lookupRes.ClusterName
			}
		}

		readCursorTS := s.scheduledTaskMaxReadLevelMap[cluster]
		if ts.Before(readCursorTS) {
			// This can happen if shard move and new host have a time SKU, or there is db write delay.
			// We generate a new timer ID using timerMaxReadLevel.
			s.logger.Warn("New timer generated is less than read level",
				tag.WorkflowDomainID(domainEntry.GetInfo().ID),
				tag.WorkflowID(workflowID),
				tag.Timestamp(ts),
				tag.CursorTimestamp(readCursorTS),
				tag.ClusterName(cluster),
				tag.ValueShardAllocateTimerBeforeRead)
			task.SetVisibilityTimestamp(s.scheduledTaskMaxReadLevelMap[cluster].Add(time.Millisecond))
		}

		seqNum, err := s.generateTaskIDLocked()
		if err != nil {
			return err
		}
		task.SetTaskID(seqNum)
		visibilityTs := task.GetVisibilityTimestamp()
		s.logger.Debug(fmt.Sprintf("Assigning new timer (timestamp: %v, seq: %v)) ackLeveL: %v",
			visibilityTs, task.GetTaskID(), s.shardInfo.TimerAckLevel))
	}
	return nil
}

func (s *contextImpl) SetCurrentTime(cluster string, currentTime time.Time) {
	s.Lock()
	defer s.Unlock()
	if cluster != s.GetClusterMetadata().GetCurrentClusterName() {
		prevTime := s.remoteClusterCurrentTime[cluster]
		if prevTime.Before(currentTime) {
			s.remoteClusterCurrentTime[cluster] = currentTime
		}
	} else {
		panic("Cannot set current time for current cluster")
	}
}

func (s *contextImpl) GetCurrentTime(cluster string) time.Time {
	s.RLock()
	defer s.RUnlock()
	if cluster != s.GetClusterMetadata().GetCurrentClusterName() {
		return s.remoteClusterCurrentTime[cluster]
	}
	return s.GetTimeSource().Now()
}

func (s *contextImpl) GetLastUpdatedTime() time.Time {
	s.RLock()
	defer s.RUnlock()
	return s.lastUpdated
}

func (s *contextImpl) ReplicateFailoverMarkers(
	ctx context.Context,
	markers []*persistence.FailoverMarkerTask,
) error {
	if err := s.closedError(); err != nil {
		return err
	}

	tasks := make([]persistence.Task, 0, len(markers))
	for _, marker := range markers {
		tasks = append(tasks, marker)
	}

	s.Lock()
	defer s.Unlock()

	immediateTaskMaxReadLevel := int64(0)
	if err := s.allocateTransferIDsLocked(
		tasks,
		&immediateTaskMaxReadLevel,
	); err != nil {
		return err
	}

	var err error
	if err := s.closedError(); err != nil {
		return err
	}
	err = s.executionManager.CreateFailoverMarkerTasks(
		ctx,
		&persistence.CreateFailoverMarkersRequest{
			RangeID: s.getRangeID(),
			Markers: markers,
		},
	)
	switch err.(type) {
	case nil:
		// Update MaxReadLevel if write to DB succeeds
		s.updateMaxReadLevelLocked(immediateTaskMaxReadLevel)
	case *persistence.ShardOwnershipLostError:
		// do not retry on ShardOwnershipLostError
		s.logger.Warn(
			"Closing shard: ReplicateFailoverMarkers failed due to stolen shard.",
			tag.Error(err),
		)
		s.closeShard()
	default:
		s.logger.Error(
			"Failed to insert the failover marker into replication queue.",
			tag.Error(err),
		)
	}
	return err
}

func (s *contextImpl) AddingPendingFailoverMarker(
	marker *types.FailoverMarkerAttributes,
) error {

	domainEntry, err := s.GetDomainCache().GetDomainByID(marker.GetDomainID())
	if err != nil {
		return err
	}
	// domain is active, the marker is expired
	isActive, _ := domainEntry.IsActiveIn(s.GetClusterMetadata().GetCurrentClusterName())
	if isActive || domainEntry.GetFailoverVersion() > marker.GetFailoverVersion() {
		s.logger.Info("Skipped out-of-date failover marker", tag.WorkflowDomainName(domainEntry.GetInfo().Name))
		return nil
	}

	s.Lock()
	defer s.Unlock()

	s.shardInfo.PendingFailoverMarkers = append(s.shardInfo.PendingFailoverMarkers, marker)
	return s.forceUpdateShardInfoLocked()
}

func (s *contextImpl) ValidateAndUpdateFailoverMarkers() ([]*types.FailoverMarkerAttributes, error) {

	completedFailoverMarkers := make(map[*types.FailoverMarkerAttributes]struct{})
	s.RLock()
	for _, marker := range s.shardInfo.PendingFailoverMarkers {
		domainEntry, err := s.GetDomainCache().GetDomainByID(marker.GetDomainID())
		if err != nil {
			s.RUnlock()
			return nil, err
		}
		isActive, _ := domainEntry.IsActiveIn(s.GetClusterMetadata().GetCurrentClusterName())
		if isActive || domainEntry.GetFailoverVersion() > marker.GetFailoverVersion() {
			completedFailoverMarkers[marker] = struct{}{}
		}
	}

	if len(completedFailoverMarkers) == 0 {
		s.RUnlock()
		return s.shardInfo.PendingFailoverMarkers, nil
	}
	s.RUnlock()

	// clean up all pending failover tasks
	s.Lock()
	defer s.Unlock()

	for idx, marker := range s.shardInfo.PendingFailoverMarkers {
		if _, ok := completedFailoverMarkers[marker]; ok {
			s.shardInfo.PendingFailoverMarkers[idx] = s.shardInfo.PendingFailoverMarkers[len(s.shardInfo.PendingFailoverMarkers)-1]
			s.shardInfo.PendingFailoverMarkers[len(s.shardInfo.PendingFailoverMarkers)-1] = nil
			s.shardInfo.PendingFailoverMarkers = s.shardInfo.PendingFailoverMarkers[:len(s.shardInfo.PendingFailoverMarkers)-1]
		}
	}
	if err := s.updateShardInfoLocked(); err != nil {
		return nil, err
	}

	return s.shardInfo.PendingFailoverMarkers, nil
}

func acquireShard(
	shardItem *historyShardsItem,
	closeCallback func(int, *historyShardsItem),
) (Context, error) {

	var shardInfo *persistence.ShardInfo

	retryPolicy := backoff.NewExponentialRetryPolicy(50 * time.Millisecond)
	retryPolicy.SetMaximumInterval(time.Second)
	retryPolicy.SetExpirationInterval(5 * time.Second)

	retryPredicate := func(err error) bool {
		if persistence.IsTransientError(err) {
			return true
		}
		_, ok := err.(*persistence.ShardAlreadyExistError)
		return ok
	}

	getShard := func(ctx context.Context) error {
		resp, err := shardItem.GetShardManager().GetShard(ctx, &persistence.GetShardRequest{
			ShardID: shardItem.shardID,
		})
		if err == nil {
			shardInfo = resp.ShardInfo
			return nil
		}
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			return err
		}

		// EntityNotExistsError error
		shardInfo = &persistence.ShardInfo{
			ShardID:          shardItem.shardID,
			RangeID:          0,
			TransferAckLevel: 0,
		}
		return shardItem.GetShardManager().CreateShard(ctx, &persistence.CreateShardRequest{ShardInfo: shardInfo})
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(retryPolicy),
		backoff.WithRetryableError(retryPredicate),
	)
	err := throttleRetry.Do(context.Background(), getShard)
	if err != nil {
		shardItem.logger.Error("Fail to acquire shard.", tag.Error(err))
		return nil, err
	}

	updatedShardInfo := shardInfo.ToNilSafeCopy()
	ownershipChanged := shardInfo.Owner != shardItem.GetHostInfo().Identity()
	updatedShardInfo.Owner = shardItem.GetHostInfo().Identity()

	// initialize the cluster current time to be the same as ack level
	remoteClusterCurrentTime := make(map[string]time.Time)
	scheduledTaskMaxReadLevelMap := make(map[string]time.Time)
	for clusterName := range shardItem.GetClusterMetadata().GetEnabledClusterInfo() {
		if clusterName != shardItem.GetClusterMetadata().GetCurrentClusterName() {
			if currentTime, ok := shardInfo.ClusterTimerAckLevel[clusterName]; ok {
				remoteClusterCurrentTime[clusterName] = currentTime
				scheduledTaskMaxReadLevelMap[clusterName] = currentTime
			} else {
				remoteClusterCurrentTime[clusterName] = shardInfo.TimerAckLevel
				scheduledTaskMaxReadLevelMap[clusterName] = shardInfo.TimerAckLevel
			}
		} else { // active cluster
			scheduledTaskMaxReadLevelMap[clusterName] = shardInfo.TimerAckLevel
		}

		scheduledTaskMaxReadLevelMap[clusterName] = scheduledTaskMaxReadLevelMap[clusterName].Truncate(time.Millisecond)
	}

	executionMgr, err := shardItem.GetExecutionManager(shardItem.shardID)
	if err != nil {
		return nil, err
	}

	context := &contextImpl{
		Resource:                       shardItem.Resource,
		shardItem:                      shardItem,
		shardID:                        shardItem.shardID,
		executionManager:               executionMgr,
		activeClusterManager:           shardItem.GetActiveClusterManager(),
		shardInfo:                      updatedShardInfo,
		closeCallback:                  closeCallback,
		config:                         shardItem.config,
		remoteClusterCurrentTime:       remoteClusterCurrentTime,
		scheduledTaskMaxReadLevelMap:   scheduledTaskMaxReadLevelMap, // use ack to init read level
		failoverLevels:                 make(map[persistence.HistoryTaskCategory]map[string]persistence.FailoverLevel),
		logger:                         shardItem.logger,
		throttledLogger:                shardItem.throttledLogger,
		previousShardOwnerWasDifferent: ownershipChanged,
	}

	// TODO remove once migrated to global event cache
	context.eventsCache = events.NewCache(
		context.shardID,
		context.Resource.GetHistoryManager(),
		context.config,
		context.logger,
		context.Resource.GetMetricsClient(),
		shardItem.GetDomainCache(),
	)

	context.logger.Debug(fmt.Sprintf("Global event cache mode: %v", context.config.EventsCacheGlobalEnable()))

	err1 := context.renewRangeLocked(true)
	if err1 != nil {
		return nil, err1
	}

	return context, nil
}

func (s *contextImpl) getEventsFromWorkflowSnapshot(snapshot *persistence.WorkflowSnapshot) []simulation.E {
	if snapshot == nil {
		return nil
	}
	var events []simulation.E
	for category, tasks := range snapshot.TasksByCategory {
		for _, task := range tasks {
			events = append(events, simulation.E{
				EventName:  simulation.EventNameCreateHistoryTask,
				Host:       s.config.HostName,
				ShardID:    s.shardID,
				DomainID:   task.GetDomainID(),
				WorkflowID: task.GetWorkflowID(),
				RunID:      task.GetRunID(),
				Payload: map[string]any{
					"task_category": category.Name(),
					"task_type":     task.GetTaskType(),
					"task_key":      task.GetTaskKey(),
				},
			})
		}
	}
	return events
}

func (s *contextImpl) getEventsFromWorkflowMutation(mutation *persistence.WorkflowMutation) []simulation.E {
	if mutation == nil {
		return nil
	}
	var events []simulation.E
	for category, tasks := range mutation.TasksByCategory {
		for _, task := range tasks {
			events = append(events, simulation.E{
				EventName:  simulation.EventNameCreateHistoryTask,
				Host:       s.config.HostName,
				ShardID:    s.shardID,
				DomainID:   task.GetDomainID(),
				WorkflowID: task.GetWorkflowID(),
				RunID:      task.GetRunID(),
				Payload: map[string]any{
					"task_category": category.Name(),
					"task_type":     task.GetTaskType(),
					"task_key":      task.GetTaskKey(),
				},
			})
		}
	}
	return events
}

func (s *contextImpl) logCreateWorkflowExecutionEvents(request *persistence.CreateWorkflowExecutionRequest) {
	events := s.getEventsFromWorkflowSnapshot(&request.NewWorkflowSnapshot)
	simulation.LogEvents(events...)
}

func (s *contextImpl) logUpdateWorkflowExecutionEvents(request *persistence.UpdateWorkflowExecutionRequest) {
	events := s.getEventsFromWorkflowMutation(&request.UpdateWorkflowMutation)
	simulation.LogEvents(events...)
	events = s.getEventsFromWorkflowSnapshot(request.NewWorkflowSnapshot)
	simulation.LogEvents(events...)
}

func (s *contextImpl) logConflictResolveWorkflowExecutionEvents(request *persistence.ConflictResolveWorkflowExecutionRequest) {
	events := s.getEventsFromWorkflowMutation(request.CurrentWorkflowMutation)
	simulation.LogEvents(events...)
	events = s.getEventsFromWorkflowSnapshot(&request.ResetWorkflowSnapshot)
	simulation.LogEvents(events...)
	events = s.getEventsFromWorkflowSnapshot(request.NewWorkflowSnapshot)
	simulation.LogEvents(events...)
}
