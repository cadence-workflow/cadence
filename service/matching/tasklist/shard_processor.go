package tasklist

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

type ShardProcessorParams struct {
	ShardID           string
	TaskListsRegistry TaskListRegistry
	ReportTTL         time.Duration
	TimeSource        clock.TimeSource
}

type shardProcessorImpl struct {
	shardID           string
	taskListsRegistry TaskListRegistry
	Status            atomic.Int32
	reportLock        sync.RWMutex
	shardReport       executorclient.ShardReport
	reportTime        time.Time
	reportTTL         time.Duration
	timeSource        clock.TimeSource
}

func NewShardProcessor(params ShardProcessorParams) (ShardProcessor, error) {
	err := validateSPParams(params)
	if err != nil {
		return nil, err
	}
	shardprocessor := &shardProcessorImpl{
		shardID:           params.ShardID,
		taskListsRegistry: params.TaskListsRegistry,
		shardReport:       executorclient.ShardReport{},
		reportTime:        params.TimeSource.Now(),
		reportTTL:         params.ReportTTL,
		timeSource:        params.TimeSource,
	}
	shardprocessor.SetShardStatus(types.ShardStatusREADY)
	shardprocessor.shardReport = executorclient.ShardReport{
		ShardLoad: 0,
		Status:    types.ShardStatusREADY,
	}
	return shardprocessor, nil
}

// Start is now not doing anything since the shard lifecycle management is still handled by the logic in matching.
// In the future the executor client will start the shard processor when a shard is assigned.
// Ideally we want to have a tasklist mapping to a shard process, but in order to do that we need a major refactoring
// of tasklists partitions and reloading processes
func (sp *shardProcessorImpl) Start(ctx context.Context) error {
	return nil
}

// Stop is stopping the tasklist when a shard is not assigned to this executor anymore.
func (sp *shardProcessorImpl) Stop() {
	toShutDown := sp.taskListsRegistry.ManagersByTaskListName(sp.shardID)
	for _, tlMgr := range toShutDown {
		tlMgr.Stop()
	}
}

func (sp *shardProcessorImpl) GetShardReport() executorclient.ShardReport {
	sp.reportLock.Lock()
	defer sp.reportLock.Unlock()
	load := sp.shardReport.ShardLoad

	if sp.reportTime.Add(sp.reportTTL).Before(sp.timeSource.Now()) {
		load = sp.getShardLoad()
	}
	sp.shardReport = executorclient.ShardReport{
		ShardLoad: load,
		Status:    types.ShardStatus(sp.Status.Load()),
	}
	return sp.shardReport
}

func (sp *shardProcessorImpl) SetShardStatus(status types.ShardStatus) {
	sp.Status.Store(int32(status))
}

func (sp *shardProcessorImpl) getShardLoad() float64 {
	var load float64

	// We assign a shard only based on the task list name
	// so task lists of different task type (decisions/activities), of different kind (normal, sticky, ephemeral) or partitions
	// will be assigned all to the same matching instance (executor)
	// we need to sum the rps for each of the tasklist to calculate the load.
	for _, tlMgr := range sp.taskListsRegistry.ManagersByTaskListName(sp.shardID) {
		qps := tlMgr.QueriesPerSecond()
		load = load + qps
	}
	return load
}

func validateSPParams(params ShardProcessorParams) error {
	if params.ShardID == "" {
		return errors.New("ShardID must be specified")
	}
	if params.TaskListsRegistry == nil {
		return errors.New("TaskListsRegistry must be specified")
	}
	if params.TimeSource == nil {
		return errors.New("TimeSource must be specified")
	}
	return nil
}
