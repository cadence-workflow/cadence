package tasklist

import (
	"sync"
	"time"

	"github.com/uber/cadence/common/clock"
)

// ShardProcessorFactory is a generic factory for creating ShardProcessor instances.
type ShardProcessorFactory struct {
	TaskListsLock *sync.RWMutex          // locks mutation of taskLists
	TaskLists     map[Identifier]Manager // Convert to LRU cache
	ReportTTL     time.Duration
	TimeSource    clock.TimeSource
}

func (spf ShardProcessorFactory) NewShardProcessor(shardID string) (ShardProcessor, error) {

	params := ShardProcessorParams{
		ShardID:       shardID,
		TaskListsLock: spf.TaskListsLock,
		TaskLists:     spf.TaskLists,
		ReportTTL:     spf.ReportTTL,
		TimeSource:    spf.TimeSource,
	}
	return NewShardProcessor(params)
}
