package tasklist

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	smtypes "github.com/cadence-workflow/shard-manager/common/types"
	"github.com/cadence-workflow/shard-manager/service/sharddistributor/client/executorclient"

	"github.com/uber/cadence/common/clock"
)

type ShardProcessorParams struct {
	ShardID           string
	TaskListsRegistry TaskListRegistry
	ReportTTL         time.Duration
	TimeSource        clock.TimeSource
	// ShardIsTaskListName is true when ShardID is a task list name, and false when it is the shard
	// that task list names hash to.
	ShardIsTaskListName bool
	// OwnedByHashRing reports whether a task list is excluded from the shard distributor, so the
	// hash ring owns it. When task lists are sharded by hash, a shard leaves those task lists alone.
	OwnedByHashRing func(taskListName string) bool
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

	shardIsTaskListName bool
	ownedByHashRing     func(taskListName string) bool
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

		shardIsTaskListName: params.ShardIsTaskListName,
		ownedByHashRing:     params.OwnedByHashRing,
	}
	shardprocessor.SetShardStatus(smtypes.ShardStatusREADY)
	shardprocessor.shardReport = executorclient.ShardReport{
		ShardLoad: 0,
		Status:    smtypes.ShardStatusREADY,
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
	// A shard can hold many task lists, and stopping one is IO heavy, so they stop in parallel.
	var wg sync.WaitGroup
	for _, tlMgr := range sp.taskListsInShard() {
		wg.Add(1)
		go func(tlMgr Manager) {
			defer wg.Done()
			tlMgr.Stop()
		}(tlMgr)
	}
	wg.Wait()
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
		Status:    smtypes.ShardStatus(sp.Status.Load()),
	}
	return sp.shardReport
}

func (sp *shardProcessorImpl) SetShardStatus(status smtypes.ShardStatus) {
	// When task lists are sharded by hash, a shard is one of a fixed set, so it is never done, even
	// when all its task lists are idle.
	if !sp.shardIsTaskListName && status == smtypes.ShardStatusDONE {
		return
	}
	sp.Status.Store(int32(status))
}

func (sp *shardProcessorImpl) getShardLoad() float64 {
	var load float64

	// We assign a shard only based on the task list name
	// so task lists of different task type (decisions/activities) or of different kind (normal, sticky, ephemeral)
	// will be assigned all to the same matching instance (executor)
	// we need to sum the rps for each of the tasklist to calculate the load.
	for _, tlMgr := range sp.taskListsInShard() {
		qps := tlMgr.QueriesPerSecond()
		load = load + qps
	}
	return load
}

// taskListsInShard returns the managers of the task lists this shard owns.
func (sp *shardProcessorImpl) taskListsInShard() []Manager {
	if sp.shardIsTaskListName {
		return sp.taskListsRegistry.ManagersByTaskListName(sp.shardID)
	}

	var owned []Manager
	for _, tlMgr := range sp.taskListsRegistry.ManagersByShard(sp.shardID) {
		if !sp.ownedByHashRing(tlMgr.TaskListID().GetName()) {
			owned = append(owned, tlMgr)
		}
	}
	return owned
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
