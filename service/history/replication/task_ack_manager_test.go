// The MIT License (MIT)
//
// Copyright (c) 2017-2022 Uber Technologies Inc.
//
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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var (
	testTask11 = persistence.ReplicationTaskInfo{TaskID: 11, DomainID: testDomainID}
	testTask12 = persistence.ReplicationTaskInfo{TaskID: 12, DomainID: testDomainID}
	testTask13 = persistence.ReplicationTaskInfo{TaskID: 13, DomainID: testDomainID}
	testTask14 = persistence.ReplicationTaskInfo{TaskID: 14, DomainID: testDomainID}

	testHydratedTask11 = types.ReplicationTask{SourceTaskID: 11, HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{DomainID: testDomainID}}
	testHydratedTask12 = types.ReplicationTask{SourceTaskID: 12, SyncActivityTaskAttributes: &types.SyncActivityTaskAttributes{DomainID: testDomainID}}
	testHydratedTask14 = types.ReplicationTask{SourceTaskID: 14, FailoverMarkerAttributes: &types.FailoverMarkerAttributes{DomainID: testDomainID}}

	testHydratedTaskErrorRecoverable    = types.ReplicationTask{SourceTaskID: -100}
	testHydratedTaskErrorNonRecoverable = types.ReplicationTask{SourceTaskID: -200}

	testClusterA = "cluster-A"
	testClusterB = "cluster-B"
	testClusterC = "cluster-C"

	testDomainName = "test-domain-name"

	testDomain = cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: testDomainID, Name: testDomainName},
		&persistence.DomainConfig{},
		true,
		&persistence.DomainReplicationConfig{Clusters: []*persistence.ClusterReplicationConfig{
			{ClusterName: testClusterA},
		}},
		0,
		nil,
		0,
		0,
		0,
	)
)

func TestTaskAckManager_GetTasks(t *testing.T) {
	tests := []struct {
		name           string
		ackLevels      *fakeAckLevelStore
		domains        domainCache
		reader         taskReader
		hydrator       taskHydrator
		pollingCluster string
		lastReadLevel  int64
		expectResult   *types.ReplicationMessages
		expectErr      string
		expectAckLevel int64
	}{
		{
			name: "main flow - continues on recoverable error",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]int64{testClusterA: 2},
			},
			domains: fakeDomainCache{testDomainID: testDomain},
			reader:  fakeTaskReader{&testTask11, &testTask12, &testTask13, &testTask14},
			hydrator: fakeTaskHydrator{
				testTask11.TaskID: testHydratedTask11,
				testTask12.TaskID: testHydratedTask12,
				testTask13.TaskID: testHydratedTaskErrorRecoverable, // Will continue hydrating beyond this point
				testTask14.TaskID: testHydratedTask14,
			},
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask11, &testHydratedTask12, &testHydratedTask14},
				LastRetrievedMessageID: 14,
				HasMore:                false,
			},
			expectAckLevel: 5,
		},
		{
			name: "main flow - stops at non recoverable error",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]int64{testClusterA: 2},
			},
			domains: fakeDomainCache{testDomainID: testDomain},
			reader:  fakeTaskReader{&testTask11, &testTask12, &testTask13, &testTask14},
			hydrator: fakeTaskHydrator{
				testTask11.TaskID: testHydratedTask11,
				testTask12.TaskID: testHydratedTask12,
				testTask13.TaskID: testHydratedTaskErrorNonRecoverable, // Will stop hydrating beyond this point
				testTask14.TaskID: testHydratedTask14,
			},
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask11, &testHydratedTask12},
				LastRetrievedMessageID: 12,
				HasMore:                true,
			},
			expectAckLevel: 5,
		},
		{
			name: "skips tasks for domains non belonging to polling cluster",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]int64{testClusterA: 2},
			},
			domains:        fakeDomainCache{testDomainID: testDomain},
			reader:         fakeTaskReader{&testTask11},
			hydrator:       fakeTaskHydrator{testTask11.TaskID: testHydratedTask11},
			pollingCluster: testClusterB,
			lastReadLevel:  5,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       nil,
				LastRetrievedMessageID: 11,
				HasMore:                false,
			},
			expectAckLevel: 5,
		},
		{
			name: "uses remote ack level for first fetch (empty task ID)",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]int64{testClusterA: 12},
			},
			domains:        fakeDomainCache{testDomainID: testDomain},
			reader:         fakeTaskReader{&testTask11, &testTask12},
			hydrator:       fakeTaskHydrator{testTask12.TaskID: testHydratedTask12},
			pollingCluster: testClusterA,
			lastReadLevel:  -1,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask12},
				LastRetrievedMessageID: 12,
				HasMore:                false,
			},
			expectAckLevel: 12,
		},
		{
			name: "failed to read replication tasks - return error",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]int64{testClusterA: 2},
			},
			reader:         (fakeTaskReader)(nil),
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			expectErr:      "error reading replication tasks",
		},
		{
			name: "failed to get domain - stops",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]int64{testClusterA: 2},
			},
			domains:        fakeDomainCache{},
			reader:         fakeTaskReader{&testTask11},
			hydrator:       fakeTaskHydrator{},
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			expectResult: &types.ReplicationMessages{
				LastRetrievedMessageID: 5,
				HasMore:                true,
			},
		},
		{
			name: "failed to update ack level - no error, return response anyway",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]int64{testClusterA: 2},
				updateErr: errors.New("error update ack level"),
			},
			domains:        fakeDomainCache{testDomainID: testDomain},
			reader:         fakeTaskReader{&testTask11},
			hydrator:       fakeTaskHydrator{testTask11.TaskID: testHydratedTask11},
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask11},
				LastRetrievedMessageID: 11,
				HasMore:                false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := createTestTaskStore(t, tt.domains, tt.hydrator)
			ackManager := NewTaskAckManager(testShardID, tt.ackLevels, metrics.NewNoopMetricsClient(), log.NewNoop(), tt.reader, taskStore)
			result, err := ackManager.GetTasks(context.Background(), tt.pollingCluster, tt.lastReadLevel)

			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectResult, result)
			}

			if tt.expectAckLevel != 0 {
				assert.Equal(t, tt.expectAckLevel, tt.ackLevels.remote[tt.pollingCluster])
			}
		})
	}
}

type fakeAckLevelStore struct {
	remote    map[string]int64
	readLevel int64
	updateErr error
}

func (s *fakeAckLevelStore) GetTransferMaxReadLevel() int64 {
	return s.readLevel
}
func (s *fakeAckLevelStore) GetClusterReplicationLevel(cluster string) int64 {
	return s.remote[cluster]
}
func (s *fakeAckLevelStore) UpdateClusterReplicationLevel(cluster string, lastTaskID int64) error {
	s.remote[cluster] = lastTaskID
	return s.updateErr
}

type fakeTaskReader []*persistence.ReplicationTaskInfo

func (r fakeTaskReader) Read(ctx context.Context, readLevel int64, maxReadLevel int64) ([]*persistence.ReplicationTaskInfo, bool, error) {
	if r == nil {
		return nil, false, errors.New("error reading replication tasks")
	}

	hasMore := false
	var result []*persistence.ReplicationTaskInfo
	for _, task := range r {
		if task.TaskID < readLevel {
			continue
		}
		if task.TaskID >= maxReadLevel {
			hasMore = true
			break
		}
		result = append(result, task)
	}
	return result, hasMore, nil
}

type fakeTaskHydrator map[int64]types.ReplicationTask

func (h fakeTaskHydrator) Hydrate(ctx context.Context, task persistence.ReplicationTaskInfo) (*types.ReplicationTask, error) {
	if hydratedTask, ok := h[task.TaskID]; ok {
		if hydratedTask == testHydratedTaskErrorNonRecoverable {
			return nil, errors.New("error hydrating task")
		}
		if hydratedTask == testHydratedTaskErrorRecoverable {
			return nil, &types.EntityNotExistsError{}
		}
		return &hydratedTask, nil
	}
	panic("fix the test, should not reach this")
}
