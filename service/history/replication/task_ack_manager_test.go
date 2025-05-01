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
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/proto"
	"github.com/uber/cadence/service/history/config"
)

var (
	testTask11 = persistence.HistoryReplicationTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID: testDomainID,
		},
		TaskData: persistence.TaskData{
			TaskID: 11,
		},
	}
	testTask12 = persistence.SyncActivityTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID: testDomainID,
		},
		TaskData: persistence.TaskData{
			TaskID: 12,
		},
	}
	testTask13 = persistence.HistoryReplicationTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID: testDomainID,
		},
		TaskData: persistence.TaskData{
			TaskID: 13,
		},
	}
	testTask14 = persistence.FailoverMarkerTask{
		DomainID: testDomainID,
		TaskData: persistence.TaskData{
			TaskID: 14,
		},
	}
	testHydratedTask11 = types.ReplicationTask{SourceTaskID: 11, HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{DomainID: testDomainID}}
	testHydratedTask12 = types.ReplicationTask{SourceTaskID: 12, SyncActivityTaskAttributes: &types.SyncActivityTaskAttributes{DomainID: testDomainID}}
	testHydratedTask13 = types.ReplicationTask{SourceTaskID: 13, HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{DomainID: testDomainID}}
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

	testConfig = &config.Config{
		MaxResponseSize: testMaxResponseSize,
	}
)

const (
	testMaxResponseSize = 4 * 1024 * 1024 // 4MB
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
		batchSize      fakeDynamicTaskBatchSizer
		expectResult   *types.ReplicationMessages
		expectErr      string
		expectAckLevel int64
		config         *config.Config
	}{
		{
			name: "main flow - no replication tasks",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 2}},
			},
			domains:        fakeDomainCache{testDomainID: testDomain},
			reader:         fakeTaskReader{},
			hydrator:       fakeTaskHydrator{},
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			batchSize:      10,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{},
				LastRetrievedMessageID: 5,
				HasMore:                false,
			},
			expectAckLevel: 5,
			config:         testConfig,
		},
		{
			name: "main flow - continues on recoverable error",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 2}},
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
			batchSize:      10,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask11, &testHydratedTask12, &testHydratedTask14},
				LastRetrievedMessageID: 14,
				HasMore:                false,
			},
			expectAckLevel: 5,
			config:         testConfig,
		},
		{
			name: "main flow - stops at non recoverable error",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 2}},
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
			batchSize:      10,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask11, &testHydratedTask12},
				LastRetrievedMessageID: 12,
				HasMore:                true,
			},
			expectAckLevel: 5,
			config:         testConfig,
		},
		{
			name: "main flow - stops at second task, batch size is 2",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 2}},
			},
			domains: fakeDomainCache{testDomainID: testDomain},
			reader:  fakeTaskReader{&testTask11, &testTask12, &testTask13, &testTask14},
			hydrator: fakeTaskHydrator{
				testTask11.TaskID: testHydratedTask11,
				testTask12.TaskID: testHydratedTask12, // Will stop hydrating beyond this point
				testTask13.TaskID: testHydratedTask13,
				testTask14.TaskID: testHydratedTask14,
			},
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			batchSize:      2,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask11, &testHydratedTask12},
				LastRetrievedMessageID: 12,
				HasMore:                true,
			},
			expectAckLevel: 5,
			config:         testConfig,
		},
		{
			name: "main flow - stops at a message exceeded max response size",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 2}},
			},
			domains: fakeDomainCache{testDomainID: testDomain},
			reader:  fakeTaskReader{&testTask11, &testTask12, &testTask13, &testTask14},
			hydrator: fakeTaskHydrator{
				testTask11.TaskID: testHydratedTask11,
				testTask12.TaskID: testHydratedTask12,
				testTask13.TaskID: testHydratedTask13,
				testTask14.TaskID: testHydratedTask14, // Will stop adding tasks beyond this point
			},
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask11, &testHydratedTask12, &testHydratedTask13},
				LastRetrievedMessageID: 13,
				HasMore:                true,
			},
			expectAckLevel: 5,
			batchSize:      10,
			config: &config.Config{
				MaxResponseSize: proto.FromReplicationMessages(&types.ReplicationMessages{
					ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask11, &testHydratedTask12, &testHydratedTask13},
					LastRetrievedMessageID: 13,
					HasMore:                true,
				}).Size() + 1,
			},
		},
		{
			name: "main flow - fail at a message exceeded max response size",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 2}},
			},
			domains: fakeDomainCache{testDomainID: testDomain},
			reader:  fakeTaskReader{&testTask11, &testTask12, &testTask13, &testTask14},
			hydrator: fakeTaskHydrator{
				testTask11.TaskID: testHydratedTask11,
				testTask12.TaskID: testHydratedTask12,
				testTask13.TaskID: testHydratedTask13,
				testTask14.TaskID: testHydratedTask14, // Will stop adding tasks beyond this point
			},
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			batchSize:      10,
			expectResult:   nil,
			expectErr:      "replication messages size is too large and cannot be shrunk anymore, shard will be stuck until the message size is reduced or max size is increased",
			expectAckLevel: 2,
			config: &config.Config{
				MaxResponseSize: 0,
			},
		},
		{
			name: "skips tasks for domains non belonging to polling cluster",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 2}},
			},
			domains:        fakeDomainCache{testDomainID: testDomain},
			reader:         fakeTaskReader{&testTask11},
			hydrator:       fakeTaskHydrator{testTask11.TaskID: testHydratedTask11},
			pollingCluster: testClusterB,
			lastReadLevel:  5,
			batchSize:      10,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{},
				LastRetrievedMessageID: 11,
				HasMore:                false,
			},
			expectAckLevel: 5,
			config:         testConfig,
		},
		{
			name: "uses remote ack level for first fetch (empty task ID)",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 12}},
			},
			domains:        fakeDomainCache{testDomainID: testDomain},
			reader:         fakeTaskReader{&testTask11, &testTask12},
			hydrator:       fakeTaskHydrator{testTask12.TaskID: testHydratedTask12},
			pollingCluster: testClusterA,
			lastReadLevel:  -1,
			batchSize:      10,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask12},
				LastRetrievedMessageID: 12,
				HasMore:                false,
			},
			expectAckLevel: 12,
			config:         testConfig,
		},
		{
			name: "failed to read replication tasks - return error",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 2}},
			},
			reader:         (fakeTaskReader)(nil),
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			batchSize:      10,
			expectErr:      "error reading replication tasks",
			config:         testConfig,
		},
		{
			name: "failed to get domain - stops",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 2}},
			},
			domains:        fakeDomainCache{},
			reader:         fakeTaskReader{&testTask11},
			hydrator:       fakeTaskHydrator{},
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			batchSize:      10,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{},
				LastRetrievedMessageID: 5,
				HasMore:                true,
			},
			config: testConfig,
		},
		{
			name: "failed to update ack level - no error, return response anyway",
			ackLevels: &fakeAckLevelStore{
				readLevel: 200,
				remote:    map[string]persistence.HistoryTaskKey{testClusterA: {TaskID: 2}},
				updateErr: errors.New("error update ack level"),
			},
			domains:        fakeDomainCache{testDomainID: testDomain},
			reader:         fakeTaskReader{&testTask11},
			hydrator:       fakeTaskHydrator{testTask11.TaskID: testHydratedTask11},
			pollingCluster: testClusterA,
			lastReadLevel:  5,
			batchSize:      10,
			expectResult: &types.ReplicationMessages{
				ReplicationTasks:       []*types.ReplicationTask{&testHydratedTask11},
				LastRetrievedMessageID: 11,
				HasMore:                false,
			},
			config: testConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := createTestTaskStore(t, tt.domains, tt.hydrator)
			ackManager := NewTaskAckManager(
				testShardID,
				tt.ackLevels,
				metrics.NewNoopMetricsClient(),
				log.NewNoop(),
				tt.reader,
				taskStore,
				clock.NewMockedTimeSource(),
				tt.config,
				proto.ReplicationMessagesSize,
				tt.batchSize,
			)
			result, err := ackManager.GetTasks(context.Background(), tt.pollingCluster, tt.lastReadLevel)

			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectResult, result)
			}

			if tt.expectAckLevel != 0 {
				assert.Equal(t, tt.expectAckLevel, tt.ackLevels.remote[tt.pollingCluster].TaskID)
			}
		})
	}
}

type fakeAckLevelStore struct {
	remote    map[string]persistence.HistoryTaskKey
	readLevel int64
	updateErr error
}

func (s *fakeAckLevelStore) UpdateIfNeededAndGetQueueMaxReadLevel(category persistence.HistoryTaskCategory, cluster string) persistence.HistoryTaskKey {
	return persistence.HistoryTaskKey{
		TaskID: s.readLevel,
	}
}
func (s *fakeAckLevelStore) GetQueueClusterAckLevel(category persistence.HistoryTaskCategory, cluster string) persistence.HistoryTaskKey {
	return s.remote[cluster]
}
func (s *fakeAckLevelStore) UpdateQueueClusterAckLevel(category persistence.HistoryTaskCategory, cluster string, lastTaskID persistence.HistoryTaskKey) error {
	s.remote[cluster] = lastTaskID
	return s.updateErr
}

type fakeTaskReader []persistence.Task

func (r fakeTaskReader) Read(ctx context.Context, readLevel int64, maxReadLevel int64, batchSize int) ([]persistence.Task, bool, error) {
	if r == nil {
		return nil, false, errors.New("error reading replication tasks")
	}

	hasMore := false
	var result []persistence.Task
	for _, task := range r {
		if task.GetTaskID() < readLevel {
			continue
		}
		if task.GetTaskID() >= maxReadLevel {
			hasMore = true
			break
		}
		result = append(result, task)
	}

	if len(result) > batchSize {
		return result[:batchSize], true, nil
	}

	return result, hasMore, nil
}

type fakeTaskHydrator map[int64]types.ReplicationTask

func (h fakeTaskHydrator) Hydrate(ctx context.Context, task persistence.Task) (*types.ReplicationTask, error) {
	if hydratedTask, ok := h[task.GetTaskID()]; ok {
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

type fakeDynamicTaskBatchSizer int

func (s fakeDynamicTaskBatchSizer) analyse(_ error, _ *getTasksResult) {}
func (s fakeDynamicTaskBatchSizer) value() int                         { return int(s) }
