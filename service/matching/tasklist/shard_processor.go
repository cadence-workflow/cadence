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
	ShardID       string
	TaskListsLock *sync.RWMutex
	TaskLists     map[Identifier]Manager
	ReportTTL     time.Duration
	TimeSource    clock.TimeSource
}

type shardProcessorImpl struct {
	shardID       string
	taskListsLock sync.RWMutex           // locks mutation of taskLists
	taskLists     map[Identifier]Manager // Convert to LRU cache
	Status        atomic.Int32
	reportLock    sync.RWMutex
	shardReport   executorclient.ShardReport
	reportTime    time.Time
	reportTTL     time.Duration
	timeSource    clock.TimeSource
}

func NewShardProcessor(params ShardProcessorParams) (ShardProcessor, error) {
	err := validateSPParams(params)
	if err != nil {
		return nil, err
	}
	shardprocessor := &shardProcessorImpl{
		shardID:     params.ShardID,
		taskLists:   params.TaskLists,
		shardReport: executorclient.ShardReport{},
		reportTime:  params.TimeSource.Now(),
		reportTTL:   params.ReportTTL,
		timeSource:  params.TimeSource,
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

// Stop is noop since the shard lifecycle management is still handled by the logic in matching.
// In the future the executor client will stop the shard processor modelling the tasklist when a shard is not assigned to this executor anymore.
func (sp *shardProcessorImpl) Stop() {

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
	sp.taskListsLock.RLock()
	defer sp.taskListsLock.RUnlock()
	var load float64

	// We assign a shard only based on the task list name
	// so task lists of differt task type (decisions/activities), of different kind (normal, sticky, ephemeral) or partitions
	// will be assigned all to the same matching instance (executor)
	// we need to sum the rps for each of the tasklist to calculate the load.
	for _, tlMgr := range sp.taskLists {
		if tlMgr.TaskListID().name == sp.shardID {
			lbh := tlMgr.LoadBalancerHints()
			load = load + lbh.RatePerSecond
		}
	}
	return load
}

func validateSPParams(params ShardProcessorParams) error {
	if params.ShardID == "" {
		return errors.New("ShardID must be specified")
	}
	if params.TaskListsLock == nil {
		return errors.New("TaskListsLock must be specified")
	}
	if params.TaskLists == nil {
		return errors.New("TaskLists must be specified")
	}
	if params.TimeSource == nil {
		return errors.New("TimeSource must be specified")
	}
	return nil
}
