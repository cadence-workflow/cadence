package tasklist

import (
	"fmt"
	"strconv"
	"strings"
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
	StartCallback   func(Manager)
	CloseCallback   func(Manager)
	StopCallback    func(Manager)
	Cfg             *config.Config
	TimeSource      clock.TimeSource
	CreateTime      time.Time
	HistoryService  history.Client
}

func (spf ShardProcessorFactory) NewShardProcessor(shardID string) (Manager, error) {
	identifier, listKind, err := FromShardRepresentationToIdentifierAndType(shardID)
	if err != nil {
		return nil, err
	}
	params := ManagerParams{
		DomainCache:     spf.DomainCache,
		Logger:          spf.Logger,
		MetricsClient:   spf.MetricsClient,
		TaskManager:     spf.TaskManager,
		ClusterMetadata: spf.ClusterMetadata,
		IsolationState:  spf.IsolationState,
		MatchingClient:  spf.MatchingClient,
		StartCallback:   spf.StartCallback,
		CloseCallback:   spf.CloseCallback,
		StopCallback:    spf.StopCallback,
		TaskList:        identifier,
		TaskListKind:    listKind,
		Cfg:             spf.Cfg,
		TimeSource:      spf.TimeSource,
		CreateTime:      spf.TimeSource.Now(),
		HistoryService:  spf.HistoryService,
	}
	return NewManager(params)
}

func (spf ShardProcessorFactory) NewShardProcessorWithTaskListIdentifier(taskListID *Identifier, taskListKind types.TaskListKind) (Manager, error) {
	params := ManagerParams{
		DomainCache:     spf.DomainCache,
		Logger:          spf.Logger,
		MetricsClient:   spf.MetricsClient,
		TaskManager:     spf.TaskManager,
		ClusterMetadata: spf.ClusterMetadata,
		IsolationState:  spf.IsolationState,
		MatchingClient:  spf.MatchingClient,
		StartCallback:   spf.StartCallback,
		CloseCallback:   spf.CloseCallback,
		StopCallback:    spf.StopCallback,
		TaskList:        taskListID,
		TaskListKind:    taskListKind,
		Cfg:             spf.Cfg,
		TimeSource:      spf.TimeSource,
		CreateTime:      spf.TimeSource.Now(),
		HistoryService:  spf.HistoryService,
	}
	return NewManager(params)
}

func FromIdentifierToShardNameRepresentation(tid *Identifier, taskListKind types.TaskListKind) string {
	return fmt.Sprintf("%v$%v$%v$%v", tid.domainID, tid.taskType, tid.name, taskListKind.String())
}

func FromShardRepresentationToIdentifierAndType(shardID string) (*Identifier, types.TaskListKind, error) {
	splitted := strings.Split(shardID, "$")
	if len(splitted) != 4 {
		return nil, types.TaskListKindNormal, fmt.Errorf("invalid ShardID format for the tasklist %s", shardID)
	}
	taskType, err := strconv.Atoi(splitted[1])
	if err != nil {
		return nil, types.TaskListKindNormal, err
	}
	identifier, err := NewIdentifier(splitted[0], splitted[2], taskType)
	if err != nil {
		return nil, types.TaskListKindSticky, err
	}
	taskListKind := strings.ToUpper(splitted[3])
	var tlk types.TaskListKind
	switch taskListKind {
	case "NORMAL":
		tlk = types.TaskListKindNormal
	case "STICKY":
		tlk = types.TaskListKindSticky
	case "EPHEMERAL":
		tlk = types.TaskListKindEphemeral
	}

	return identifier, tlk, nil
}
