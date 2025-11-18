package etcdtypes

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func TestAssignedState_FieldNumberMatched(t *testing.T) {
	require.Equal(t,
		reflect.TypeOf(AssignedState{}).NumField(),
		reflect.TypeOf(store.AssignedState{}).NumField(),
		"AssignedState field count mismatch with store.AssignedState; ensure conversion is updated",
	)
}

func TestAssignedState_ToAssignedState(t *testing.T) {
	tests := map[string]struct {
		input  *AssignedState
		expect *store.AssignedState
	}{
		"nil": {
			input:  nil,
			expect: nil,
		},
		"success": {
			input: &AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"1": {Status: types.AssignmentStatusREADY},
				},
				LastUpdated: Time(time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC)),
				ModRevision: 42,
			},
			expect: &store.AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"1": {Status: types.AssignmentStatusREADY},
				},
				LastUpdated: time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC),
				ModRevision: 42,
			},
		},
	}

	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			got := c.input.ToAssignedState()

			if c.expect == nil {
				require.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			require.Equal(t, len(c.input.AssignedShards), len(got.AssignedShards))
			for k := range c.input.AssignedShards {
				require.Equal(t, c.input.AssignedShards[k].Status, got.AssignedShards[k].Status)
			}
			require.Equal(t, time.Time(c.input.LastUpdated).UnixNano(), got.LastUpdated.UnixNano())
			require.Equal(t, c.input.ModRevision, got.ModRevision)
		})
	}
}
func TestAssignedState_FromAssignedState(t *testing.T) {
	tests := map[string]struct {
		input  *store.AssignedState
		expect *AssignedState
	}{
		"nil": {
			input:  nil,
			expect: nil,
		},
		"success": {
			input: &store.AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"9": {Status: types.AssignmentStatusREADY},
				},
				LastUpdated: time.Date(2025, 11, 18, 13, 0, 0, 987654321, time.UTC),
				ModRevision: 77,
			},
			expect: &AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"9": {Status: types.AssignmentStatusREADY},
				},
				LastUpdated: Time(time.Date(2025, 11, 18, 13, 0, 0, 987654321, time.UTC)),
				ModRevision: 77,
			},
		},
	}

	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			got := FromAssignedState(c.input)
			if c.expect == nil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, len(c.input.AssignedShards), len(got.AssignedShards))
			for k := range c.input.AssignedShards {
				require.Equal(t, c.input.AssignedShards[k].Status, got.AssignedShards[k].Status)
			}
			require.Equal(t, c.input.LastUpdated.UnixNano(), time.Time(got.LastUpdated).UnixNano())
			require.Equal(t, c.input.ModRevision, got.ModRevision)
		})
	}
}

func TestAssignedState_JSONMarshalling(t *testing.T) {
	const jsonStr = `{"assigned_shards":{"1":{"status":"READY"}},"last_updated":"2025-11-18T12:00:00.123456789Z","mod_revision":42}`

	state := &AssignedState{
		AssignedShards: map[string]*types.ShardAssignment{
			"1": {Status: types.AssignmentStatusREADY},
		},
		LastUpdated: Time(time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC)),
		ModRevision: 42,
	}

	// Marshal to JSON
	b, err := json.Marshal(state)
	require.NoError(t, err)
	require.JSONEq(t, jsonStr, string(b))

	// Unmarshal from JSON
	var unmarshalled AssignedState
	err = json.Unmarshal([]byte(jsonStr), &unmarshalled)
	require.NoError(t, err)
	require.Equal(t, state.AssignedShards["1"].Status, unmarshalled.AssignedShards["1"].Status)
	require.Equal(t, time.Time(state.LastUpdated).UnixNano(), time.Time(unmarshalled.LastUpdated).UnixNano())
	require.Equal(t, state.ModRevision, unmarshalled.ModRevision)
}
func TestShardStatistics_FieldNumberMatched(t *testing.T) {
	require.Equal(t,
		reflect.TypeOf(ShardStatistics{}).NumField(),
		reflect.TypeOf(store.ShardStatistics{}).NumField(),
		"ShardStatistics field count mismatch with store.ShardStatistics; ensure conversion is updated",
	)
}

func TestShardStatistics_ToShardStatistics(t *testing.T) {
	tests := map[string]struct {
		input  *ShardStatistics
		expect *store.ShardStatistics
	}{
		"nil": {
			input:  nil,
			expect: nil,
		},
		"success": {
			input: &ShardStatistics{
				SmoothedLoad:   12.34,
				LastUpdateTime: Time(time.Date(2025, 11, 18, 14, 0, 0, 111111111, time.UTC)),
				LastMoveTime:   Time(time.Date(2025, 11, 18, 15, 0, 0, 222222222, time.UTC)),
			},
			expect: &store.ShardStatistics{
				SmoothedLoad:   12.34,
				LastUpdateTime: time.Date(2025, 11, 18, 14, 0, 0, 111111111, time.UTC),
				LastMoveTime:   time.Date(2025, 11, 18, 15, 0, 0, 222222222, time.UTC),
			},
		},
	}

	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			got := c.input.ToShardStatistics()
			if c.expect == nil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, c.input.SmoothedLoad, got.SmoothedLoad)
			require.Equal(t, time.Time(c.input.LastUpdateTime).UnixNano(), got.LastUpdateTime.UnixNano())
			require.Equal(t, time.Time(c.input.LastMoveTime).UnixNano(), got.LastMoveTime.UnixNano())
		})
	}
}

func TestShardStatistics_FromShardStatistics(t *testing.T) {
	tests := map[string]struct {
		input  *store.ShardStatistics
		expect *ShardStatistics
	}{
		"nil": {
			input:  nil,
			expect: nil,
		},
		"success": {
			input: &store.ShardStatistics{
				SmoothedLoad:   99.01,
				LastUpdateTime: time.Date(2025, 11, 18, 16, 0, 0, 333333333, time.UTC),
				LastMoveTime:   time.Date(2025, 11, 18, 17, 0, 0, 444444444, time.UTC),
			},
			expect: &ShardStatistics{
				SmoothedLoad:   99.01,
				LastUpdateTime: Time(time.Date(2025, 11, 18, 16, 0, 0, 333333333, time.UTC)),
				LastMoveTime:   Time(time.Date(2025, 11, 18, 17, 0, 0, 444444444, time.UTC)),
			},
		},
	}

	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			got := FromShardStatistics(c.input)
			if c.expect == nil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.InDelta(t, c.input.SmoothedLoad, got.SmoothedLoad, 0.0000001)
			require.Equal(t, c.input.LastUpdateTime.UnixNano(), time.Time(got.LastUpdateTime).UnixNano())
			require.Equal(t, c.input.LastMoveTime.UnixNano(), time.Time(got.LastMoveTime).UnixNano())
		})
	}
}

func TestShardStatistics_JSONMarshalling(t *testing.T) {
	const jsonStr = `{"smoothed_load":12.34,"last_update_time":"2025-11-18T14:00:00.111111111Z","last_move_time":"2025-11-18T15:00:00.222222222Z"}`

	state := &ShardStatistics{
		SmoothedLoad:   12.34,
		LastUpdateTime: Time(time.Date(2025, 11, 18, 14, 0, 0, 111111111, time.UTC)),
		LastMoveTime:   Time(time.Date(2025, 11, 18, 15, 0, 0, 222222222, time.UTC)),
	}

	// Marshal to JSON
	b, err := json.Marshal(state)
	require.NoError(t, err)
	require.JSONEq(t, jsonStr, string(b))

	// Unmarshal from JSON
	var unmarshalled ShardStatistics
	err = json.Unmarshal([]byte(jsonStr), &unmarshalled)
	require.NoError(t, err)
	require.InDelta(t, state.SmoothedLoad, unmarshalled.SmoothedLoad, 0.0000001)
	require.Equal(t, time.Time(state.LastUpdateTime).UnixNano(), time.Time(unmarshalled.LastUpdateTime).UnixNano())
	require.Equal(t, time.Time(state.LastMoveTime).UnixNano(), time.Time(unmarshalled.LastMoveTime).UnixNano())
}
