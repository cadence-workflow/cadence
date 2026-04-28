package common

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/mvccpb"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdkeys"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdtypes"
)

func TestParseExecutorKVs(t *testing.T) {
	prefix := "/test-prefix"
	namespace := "test-ns"
	executorID := "exec-1"

	writer, err := NewRecordWriter(CompressionSnappy)
	require.NoError(t, err)

	heartbeatTime := time.Date(2025, 11, 18, 12, 0, 0, 0, time.UTC)
	status := types.ExecutorStatusACTIVE
	reportedShards := map[string]*types.ShardStatusReport{
		"shard-1": {Status: types.ShardStatusREADY},
	}
	assignedState := &etcdtypes.AssignedState{
		AssignedShards: map[string]*types.ShardAssignment{
			"shard-1": {Status: types.AssignmentStatusREADY},
		},
		LastUpdated: etcdtypes.Time(heartbeatTime),
	}
	stats := map[string]etcdtypes.ShardStatistics{
		"shard-1": {SmoothedLoad: 1.23, LastUpdateTime: etcdtypes.Time(heartbeatTime)},
	}

	marshal := func(v interface{}) []byte {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		compressed, err := writer.Write(b)
		require.NoError(t, err)
		return compressed
	}

	kvs := []*mvccpb.KeyValue{
		{
			Key:   []byte(etcdkeys.BuildExecutorKey(prefix, namespace, executorID, etcdkeys.ExecutorHeartbeatKey)),
			Value: []byte(etcdtypes.FormatTime(heartbeatTime)),
		},
		{
			Key:   []byte(etcdkeys.BuildExecutorKey(prefix, namespace, executorID, etcdkeys.ExecutorStatusKey)),
			Value: marshal(status),
		},
		{
			Key:   []byte(etcdkeys.BuildExecutorKey(prefix, namespace, executorID, etcdkeys.ExecutorReportedShardsKey)),
			Value: marshal(reportedShards),
		},
		{
			Key:         []byte(etcdkeys.BuildExecutorKey(prefix, namespace, executorID, etcdkeys.ExecutorAssignedStateKey)),
			Value:       marshal(assignedState),
			ModRevision: 123,
		},
		{
			Key:   []byte(etcdkeys.BuildMetadataKey(prefix, namespace, executorID, "k1")),
			Value: []byte("v1"),
		},
		{
			Key:   []byte(etcdkeys.BuildExecutorKey(prefix, namespace, executorID, etcdkeys.ExecutorShardStatisticsKey)),
			Value: marshal(stats),
		},
	}

	result, err := ParseExecutorKVs(prefix, namespace, kvs)
	require.NoError(t, err)
	require.Len(t, result, 1)

	data, ok := result[executorID]
	require.True(t, ok)
	require.NotNil(t, data)

	assert.Equal(t, etcdtypes.Time(heartbeatTime), data.LastHeartbeat)
	assert.Equal(t, status, data.Status)
	assert.Equal(t, reportedShards, data.ReportedShards)
	require.NotNil(t, data.AssignedState)
	assert.Equal(t, assignedState.AssignedShards, data.AssignedState.AssignedShards)
	assert.Equal(t, int64(123), data.AssignedState.ModRevision)
	assert.Equal(t, map[string]string{"k1": "v1"}, data.Metadata)
	assert.Equal(t, stats, data.Statistics)
}
