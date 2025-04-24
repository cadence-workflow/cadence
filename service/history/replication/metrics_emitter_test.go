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

package replication

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
)

var (
	cluster1 = "cluster1"
	cluster2 = "cluster2"
	cluster3 = "cluster3"
)

func TestMetricsEmitterStartStop(t *testing.T) {
	goleak.VerifyNone(t)

	timeSource := clock.NewMockedTimeSource()
	metadata := newClusterMetadata(t)
	testShardData := newTestShardData(timeSource, metadata)

	metricsEmitter := NewMetricsEmitter(1, testShardData, fakeTaskReader{}, metrics.NewNoopMetricsClient())
	metricsEmitter.interval = 5 * time.Millisecond
	metricsEmitter.Start()
	time.Sleep(20 * time.Millisecond) // let the metrics emitter run a few times
	metricsEmitter.Stop()
}

func TestMetricsEmitter(t *testing.T) {
	timeSource := clock.NewMockedTimeSource()
	metadata := newClusterMetadata(t)
	testShardData := newTestShardData(timeSource, metadata)

	task1 := &persistence.HistoryReplicationTask{
		TaskData: persistence.TaskData{
			TaskID:              1,
			VisibilityTimestamp: timeSource.Now().Add(-time.Hour),
		},
	}
	task2 := &persistence.HistoryReplicationTask{
		TaskData: persistence.TaskData{
			TaskID:              2,
			VisibilityTimestamp: timeSource.Now().Add(-time.Minute),
		},
	}
	reader := fakeTaskReader{task1, task2}

	metricsEmitter := NewMetricsEmitter(1, testShardData, reader, metrics.NewNoopMetricsClient())
	latency, err := metricsEmitter.determineReplicationLatency(cluster2)
	assert.NoError(t, err)
	assert.Equal(t, time.Hour, latency)

	// Move replication level up for cluster2 and our latency shortens
	testShardData.clusterReplicationLevel[cluster2] = persistence.HistoryTaskKey{
		TaskID: 2,
	}
	latency, err = metricsEmitter.determineReplicationLatency(cluster2)
	assert.NoError(t, err)
	assert.Equal(t, time.Minute, latency)

	// Move replication level up for cluster2 and we no longer have latency
	testShardData.clusterReplicationLevel[cluster2] = persistence.HistoryTaskKey{
		TaskID: 3,
	}
	latency, err = metricsEmitter.determineReplicationLatency(cluster2)
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), latency)

	// Cluster3 will still have latency
	latency, err = metricsEmitter.determineReplicationLatency(cluster3)
	assert.NoError(t, err)
	assert.Equal(t, time.Hour, latency)
}

type testShardData struct {
	logger                  log.Logger
	clusterReplicationLevel map[string]persistence.HistoryTaskKey
	timeSource              clock.TimeSource
	metadata                cluster.Metadata
}

func newTestShardData(timeSource clock.TimeSource, metadata cluster.Metadata) testShardData {
	remotes := metadata.GetRemoteClusterInfo()
	clusterReplicationLevels := make(map[string]persistence.HistoryTaskKey, len(remotes))
	for remote := range remotes {
		clusterReplicationLevels[remote] = persistence.HistoryTaskKey{
			TaskID: 1,
		}
	}
	return testShardData{
		logger:                  log.NewNoop(),
		timeSource:              timeSource,
		metadata:                metadata,
		clusterReplicationLevel: clusterReplicationLevels,
	}
}

func (t testShardData) GetLogger() log.Logger {
	return t.logger
}

func (t testShardData) GetQueueClusterAckLevel(category persistence.HistoryTaskCategory, cluster string) persistence.HistoryTaskKey {
	return t.clusterReplicationLevel[cluster]
}

func (t testShardData) GetTimeSource() clock.TimeSource {
	return t.timeSource
}

func (t testShardData) GetClusterMetadata() cluster.Metadata {
	return t.metadata
}

func newClusterMetadata(t *testing.T) cluster.Metadata {
	return cluster.NewMetadata(
		config.ClusterGroupMetadata{
			PrimaryClusterName: cluster1,
			CurrentClusterName: cluster1,
			ClusterGroup: map[string]config.ClusterInformation{
				cluster1: {Enabled: true},
				cluster2: {Enabled: true},
				cluster3: {Enabled: true},
			},
		},
		func(d string) bool { return false },
		metrics.NewNoopMetricsClient(),
		testlogger.New(t),
	)
}
