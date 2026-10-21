package tasklist

import (
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
)

// ShardProcessorFactory is a generic factory for creating ShardProcessor instances.
type ShardProcessorFactory struct {
	TaskListsRegistry TaskListRegistry
	ReportTTL         time.Duration
	TimeSource        clock.TimeSource
	// ShardTaskListsByHash sets ShardProcessorParams.ShardTaskListsByHash for every new shard.
	ShardTaskListsByHash dynamicproperties.BoolPropertyFn
	// OwnedByHashRing reports whether a task list is excluded from the shard distributor.
	OwnedByHashRing func(taskListName string) bool
}

func (spf ShardProcessorFactory) NewShardProcessor(shardID string) (ShardProcessor, error) {

	params := ShardProcessorParams{
		ShardID:           shardID,
		TaskListsRegistry: spf.TaskListsRegistry,
		ReportTTL:         spf.ReportTTL,
		TimeSource:        spf.TimeSource,
		// The flag only changes while matching is offboarded from the shard distributor, so a
		// shard assigned now has the kind the flag says. Shards assigned before a change keep
		// their kind until they stop.
		ShardTaskListsByHash: spf.ShardTaskListsByHash(),
		OwnedByHashRing:      spf.OwnedByHashRing,
	}
	return NewShardProcessor(params)
}
