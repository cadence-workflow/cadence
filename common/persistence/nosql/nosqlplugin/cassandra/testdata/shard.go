// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package testdata

import (
	"time"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func NewShardRow(ts time.Time) *nosqlplugin.ShardRow {
	return &nosqlplugin.ShardRow{
		InternalShardInfo: &persistence.InternalShardInfo{
			ShardID:                       15,
			Owner:                         "owner",
			RangeID:                       1000,
			ReplicationAckLevel:           2000,
			TransferAckLevel:              3000,
			TimerAckLevel:                 ts.Add(-time.Hour),
			ClusterTransferAckLevel:       map[string]int64{"cluster2": 4000},
			ClusterTimerAckLevel:          map[string]time.Time{"cluster2": ts.Add(-2 * time.Hour)},
			DomainNotificationVersion:     3,
			ClusterReplicationLevel:       map[string]int64{"cluster2": 5000},
			ReplicationDLQAckLevel:        map[string]int64{"cluster2": 10},
			PendingFailoverMarkers:        &persistence.DataBlob{Encoding: "thriftrw", Data: []byte("failovermarkers")},
			TransferProcessingQueueStates: &persistence.DataBlob{Encoding: "thriftrw", Data: []byte("transferqueue")},
			TimerProcessingQueueStates:    &persistence.DataBlob{Encoding: "thriftrw", Data: []byte("timerqueue")},
			CurrentTimestamp:              ts,
		},
		Data:         []byte("sharddata"),
		DataEncoding: "thriftrw",
	}
}

func NewShardMap(ts time.Time) map[string]interface{} {
	return map[string]interface{}{
		"shard_id":              int(15),
		"range_id":              int64(1000),
		"owner":                 "owner",
		"stolen_since_renew":    0,
		"updated_at":            ts,
		"replication_ack_level": int64(2000),
		"transfer_ack_level":    int64(3000),
		"timer_ack_level":       ts.Add(-1 * time.Hour),
		"cluster_transfer_ack_level": map[string]int64{
			"cluster1": int64(3000),
		},
		"cluster_timer_ack_level": map[string]time.Time{
			"cluster1": ts.Add(-1 * time.Hour),
		},
		"transfer_processing_queue_states":          []byte("transferqueue"),
		"transfer_processing_queue_states_encoding": "thriftrw",
		"timer_processing_queue_states":             []byte("timerqueue"),
		"timer_processing_queue_states_encoding":    "thriftrw",
		"domain_notification_version":               int64(3),
		"cluster_replication_level":                 map[string]int64{"cluster2": 1500},
		"replication_dlq_ack_level":                 map[string]int64{"cluster2": 5},
		"pending_failover_markers":                  []byte("failovermarkers"),
		"pending_failover_markers_encoding":         "thriftrw",
	}
}
