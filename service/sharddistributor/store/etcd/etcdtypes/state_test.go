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
		"success with map": {
			input: &AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"1": {Status: types.AssignmentStatusREADY},
				},
				ShardHandoverStats: map[string]ShardHandoverStats{
					"1": {
						PreviousExecutorLastHeartbeatTime: Time(time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC)),
						HandoverType:                      types.HandoverTypeGRACEFUL,
					},
				},
				LastUpdated: Time(time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC)),
				ModRevision: 42,
			},
			expect: &store.AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"1": {Status: types.AssignmentStatusREADY},
				},
				ShardHandoverStats: map[string]store.ShardHandoverStats{
					"1": {
						PreviousExecutorLastHeartbeatTime: time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC),
						HandoverType:                      types.HandoverTypeGRACEFUL,
					},
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
		"with map": {
			input: &store.AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"9": {Status: types.AssignmentStatusREADY},
				},
				ShardHandoverStats: map[string]store.ShardHandoverStats{
					"9": {
						PreviousExecutorLastHeartbeatTime: time.Date(2025, 11, 18, 13, 0, 0, 987654321, time.UTC),
						HandoverType:                      types.HandoverTypeGRACEFUL,
					},
				},
				LastUpdated: time.Date(2025, 11, 18, 13, 0, 0, 987654321, time.UTC),
				ModRevision: 77,
			},
			expect: &AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"9": {Status: types.AssignmentStatusREADY},
				},
				ShardHandoverStats: map[string]ShardHandoverStats{
					"9": {
						PreviousExecutorLastHeartbeatTime: Time(time.Date(2025, 11, 18, 13, 0, 0, 987654321, time.UTC)),
						HandoverType:                      types.HandoverTypeGRACEFUL,
					},
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
	tests := map[string]struct {
		input   *AssignedState
		jsonStr string
	}{
		"simple": {
			input: &AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"1": {Status: types.AssignmentStatusREADY},
				},
				LastUpdated: Time(time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC)),
				ModRevision: 42,
			},
			jsonStr: `{"assigned_shards":{"1":{"status":"AssignmentStatusREADY"}},"last_updated":"2025-11-18T12:00:00.123456789Z"}`,
		},
		"with map": {
			input: &AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"1": {Status: types.AssignmentStatusREADY},
				},
				ShardHandoverStats: map[string]ShardHandoverStats{
					"1": {
						PreviousExecutorLastHeartbeatTime: Time(time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC)),
						HandoverType:                      types.HandoverTypeGRACEFUL,
					},
				},
				LastUpdated: Time(time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC)),
				ModRevision: 42,
			},
			jsonStr: `{"assigned_shards":{"1":{"status":"AssignmentStatusREADY"}},"shard_handover_stats":{"1":{"previous_executor_last_heartbeat_time":"2025-11-18T12:00:00.123456789Z","handover_type":"HandoverTypeGRACEFUL"}},"last_updated":"2025-11-18T12:00:00.123456789Z"}`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Marshal to JSON
			b, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.JSONEq(t, tc.jsonStr, string(b))

			// Unmarshal from JSON
			var unmarshalled AssignedState
			err = json.Unmarshal([]byte(tc.jsonStr), &unmarshalled)
			require.NoError(t, err)
			require.Equal(t, tc.input.AssignedShards["1"].Status, unmarshalled.AssignedShards["1"].Status)
			require.Equal(t, time.Time(tc.input.LastUpdated).UnixNano(), time.Time(unmarshalled.LastUpdated).UnixNano())
			if tc.input.ShardHandoverStats != nil {
				require.NotNil(t, unmarshalled.ShardHandoverStats)
				require.Equal(t, tc.input.ShardHandoverStats["1"].HandoverType, unmarshalled.ShardHandoverStats["1"].HandoverType)
				require.Equal(t, time.Time(tc.input.ShardHandoverStats["1"].PreviousExecutorLastHeartbeatTime).UnixNano(),
					time.Time(unmarshalled.ShardHandoverStats["1"].PreviousExecutorLastHeartbeatTime).UnixNano())
			}
		})
	}
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

func TestShardHandoverStats_FieldNumberMatched(t *testing.T) {
	require.Equal(t,
		reflect.TypeOf(ShardHandoverStats{}).NumField(),
		reflect.TypeOf(store.ShardHandoverStats{}).NumField(),
		"ShardHandoverStats field count mismatch with store.ShardHandoverStats; ensure conversion is updated",
	)
}

func TestShardHandoverStats_ToShardHandoverStats(t *testing.T) {
	tests := map[string]struct {
		input  *ShardHandoverStats
		expect *store.ShardHandoverStats
	}{
		"nil": {
			input:  nil,
			expect: nil,
		},
		"success": {
			input: &ShardHandoverStats{
				PreviousExecutorLastHeartbeatTime: Time(time.Date(2025, 11, 18, 18, 0, 0, 555555555, time.UTC)),
				HandoverType:                      types.HandoverTypeGRACEFUL,
			},
			expect: &store.ShardHandoverStats{
				PreviousExecutorLastHeartbeatTime: time.Date(2025, 11, 18, 18, 0, 0, 555555555, time.UTC),
				HandoverType:                      types.HandoverTypeGRACEFUL,
			},
		},
	}

	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			got := ToShardHandoverStats(c.input)
			if c.expect == nil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, c.input.HandoverType, got.HandoverType)
			require.Equal(t, time.Time(c.input.PreviousExecutorLastHeartbeatTime).UnixNano(), got.PreviousExecutorLastHeartbeatTime.UnixNano())
		})
	}
}

func TestShardHandoverStats_FromShardHandoverStats(t *testing.T) {
	tests := map[string]struct {
		input  *store.ShardHandoverStats
		expect *ShardHandoverStats
	}{
		"nil": {
			input:  nil,
			expect: nil,
		},
		"success": {
			input: &store.ShardHandoverStats{
				PreviousExecutorLastHeartbeatTime: time.Date(2025, 11, 18, 19, 0, 0, 666666666, time.UTC),
				HandoverType:                      types.HandoverTypeGRACEFUL,
			},
			expect: &ShardHandoverStats{
				PreviousExecutorLastHeartbeatTime: Time(time.Date(2025, 11, 18, 19, 0, 0, 666666666, time.UTC)),
				HandoverType:                      types.HandoverTypeGRACEFUL,
			},
		},
	}

	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			got := FromShardHandoverStats(c.input)
			if c.expect == nil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, c.input.HandoverType, got.HandoverType)
			require.Equal(t, c.input.PreviousExecutorLastHeartbeatTime.UnixNano(), time.Time(got.PreviousExecutorLastHeartbeatTime).UnixNano())
		})
	}
}

func TestShardHandoverStats_JSONMarshalling(t *testing.T) {
	const jsonStr = `{"previous_executor_last_heartbeat_time":"2025-11-18T20:00:00.777777777Z","handover_type":"HandoverTypeGRACEFUL"}`

	stats := &ShardHandoverStats{
		PreviousExecutorLastHeartbeatTime: Time(time.Date(2025, 11, 18, 20, 0, 0, 777777777, time.UTC)),
		HandoverType:                      types.HandoverTypeGRACEFUL,
	}

	// Marshal to JSON
	b, err := json.Marshal(stats)
	require.NoError(t, err)
	require.JSONEq(t, jsonStr, string(b))

	// Unmarshal from JSON
	var unmarshalled ShardHandoverStats
	err = json.Unmarshal([]byte(jsonStr), &unmarshalled)
	require.NoError(t, err)
	require.Equal(t, stats.HandoverType, unmarshalled.HandoverType)
	require.Equal(t, time.Time(stats.PreviousExecutorLastHeartbeatTime).UnixNano(), time.Time(unmarshalled.PreviousExecutorLastHeartbeatTime).UnixNano())
}
