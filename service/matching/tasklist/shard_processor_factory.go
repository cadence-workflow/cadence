package tasklist

import (
	"time"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
)

// ShardProcessorFactory is a generic factory for creating ShardProcessor instances.
type ShardProcessorFactory struct {
	DomainCache     cache.DomainCache
	Logger          log.Logger
	MetricsClient   metrics.Client
	TaskManager     persistence.TaskManager
	ClusterMetadata cluster.Metadata
	IsolationState  isolationgroup.State
	MatchingClient  matching.Client
	CloseCallback   func(processor ShardProcessor)
	Cfg             *config.Config
	TimeSource      clock.TimeSource
	CreateTime      time.Time
	HistoryService  history.Client
}

func (spf ShardProcessorFactory) NewShardProcessor(shardID string) (ShardProcessor, error) {
	name, err := newTaskListName(shardID)
	if err != nil {
		return nil, err
	}
	identifier := &Identifier{
		qualifiedTaskListName: name,
	}
	params := ManagerParams{
		DomainCache:     spf.DomainCache,
		Logger:          spf.Logger,
		MetricsClient:   spf.MetricsClient,
		TaskManager:     spf.TaskManager,
		ClusterMetadata: spf.ClusterMetadata,
		IsolationState:  spf.IsolationState,
		MatchingClient:  spf.MatchingClient,
		CloseCallback:   spf.CloseCallback,
		TaskList:        identifier,
		TaskListKind:    0,
		Cfg:             spf.Cfg,
		TimeSource:      spf.TimeSource,
		CreateTime:      spf.TimeSource.Now(),
		HistoryService:  spf.HistoryService,
	}
	return NewShardProcessor(params)
}

func (spf ShardProcessorFactory) NewShardProcessorWithTaskListIdentifier(taskListID *Identifier, taskListKind types.TaskListKind) (ShardProcessor, error) {
	params := ManagerParams{
		DomainCache:     spf.DomainCache,
		Logger:          spf.Logger,
		MetricsClient:   spf.MetricsClient,
		TaskManager:     spf.TaskManager,
		ClusterMetadata: spf.ClusterMetadata,
		IsolationState:  spf.IsolationState,
		MatchingClient:  spf.MatchingClient,
		CloseCallback:   spf.CloseCallback,
		TaskList:        taskListID,
		TaskListKind:    taskListKind,
		Cfg:             spf.Cfg,
		TimeSource:      spf.TimeSource,
		CreateTime:      spf.TimeSource.Now(),
		HistoryService:  spf.HistoryService,
	}
	return NewShardProcessor(params)
}
