package plan

import "errors"

var ErrNoActiveExecutors = errors.New("no active executors available")

type Placement struct {
	ShardID    string
	ExecutorID string
}

type Move struct {
	ShardID string
	From    string
	To      string
}
