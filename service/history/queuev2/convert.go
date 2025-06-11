package queuev2

import (
	"time"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func FromPersistenceQueueState(state *types.QueueState) *QueueState {
	virtualQueueStates := make(map[int64][]VirtualSliceState)
	for k, v := range state.VirtualQueueStates {
		virtualQueueStates[k] = FromPersistenceVirtualQueueState(v)
	}
	return &QueueState{
		VirtualQueueStates:    virtualQueueStates,
		ExclusiveMaxReadLevel: FromPersistenceTaskKey(state.ExclusiveMaxReadLevel),
	}
}

func ToPersistenceQueueState(state *QueueState) *types.QueueState {
	virtualQueueStates := make(map[int64]*types.VirtualQueueState)
	for k, v := range state.VirtualQueueStates {
		virtualQueueStates[k] = ToPersistenceVirtualQueueState(v)
	}
	return &types.QueueState{
		VirtualQueueStates:    virtualQueueStates,
		ExclusiveMaxReadLevel: ToPersistenceTaskKey(state.ExclusiveMaxReadLevel),
	}
}

func FromPersistenceVirtualQueueState(state *types.VirtualQueueState) []VirtualSliceState {
	states := make([]VirtualSliceState, 0, len(state.VirtualSliceStates))
	for _, v := range state.VirtualSliceStates {
		states = append(states, FromPersistenceVirtualSliceState(v))
	}
	return states
}

func ToPersistenceVirtualQueueState(state []VirtualSliceState) *types.VirtualQueueState {
	states := make([]*types.VirtualSliceState, 0, len(state))
	for _, v := range state {
		states = append(states, ToPersistenceVirtualSliceState(v))
	}
	return &types.VirtualQueueState{
		VirtualSliceStates: states,
	}
}

func FromPersistenceVirtualSliceState(state *types.VirtualSliceState) VirtualSliceState {
	return VirtualSliceState{
		Range:     FromPersistenceTaskRange(state.TaskRange),
		Predicate: NewUniversalPredicate(),
		// Predicate: FromPersistencePredicate(state.Predicate),
	}
}

func ToPersistenceVirtualSliceState(state VirtualSliceState) *types.VirtualSliceState {
	return &types.VirtualSliceState{
		TaskRange: ToPersistenceTaskRange(state.Range),
		// Predicate: ToPersistencePredicate(state.Predicate),
	}
}

func FromPersistenceTaskRange(state *types.TaskRange) Range {
	return Range{
		InclusiveMinTaskKey: FromPersistenceTaskKey(state.InclusiveMin),
		ExclusiveMaxTaskKey: FromPersistenceTaskKey(state.ExclusiveMax),
	}
}

func ToPersistenceTaskRange(r Range) *types.TaskRange {
	return &types.TaskRange{
		InclusiveMin: ToPersistenceTaskKey(r.InclusiveMinTaskKey),
		ExclusiveMax: ToPersistenceTaskKey(r.ExclusiveMaxTaskKey),
	}
}

func FromPersistenceTaskKey(key *types.TaskKey) persistence.HistoryTaskKey {
	return persistence.NewHistoryTaskKey(time.Unix(0, key.ScheduledTimeNano).UTC(), key.TaskID)
}

func ToPersistenceTaskKey(key persistence.HistoryTaskKey) *types.TaskKey {
	return &types.TaskKey{
		TaskID:            key.GetTaskID(),
		ScheduledTimeNano: key.GetScheduledTime().UnixNano(),
	}
}
