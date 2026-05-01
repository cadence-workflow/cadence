// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package replication

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
)

type (
	taskFetcherSuite struct {
		suite.Suite
		*require.Assertions
		controller *gomock.Controller

		mockResource   *resource.Test
		config         *config.Config
		frontendClient *admin.MockClient
		taskFetcher    *taskFetcherImpl
	}
)

func TestReplicationTaskFetcherSuite(t *testing.T) {
	s := new(taskFetcherSuite)
	suite.Run(t, s)
}

func (s *taskFetcherSuite) SetupSuite() {

}

func (s *taskFetcherSuite) TearDownSuite() {

}

func (s *taskFetcherSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.controller = gomock.NewController(s.T())

	s.mockResource = resource.NewTest(s.T(), s.controller, metrics.History)
	s.frontendClient = s.mockResource.RemoteAdminClient
	logger := testlogger.New(s.T())
	s.config = config.NewForTest()
	s.config.ReplicationTaskFetcherTimerJitterCoefficient = dynamicproperties.GetFloatPropertyFn(0.0) // set jitter to 0 for test

	s.taskFetcher = newReplicationTaskFetcher(
		logger,
		"standby",
		"active",
		s.config,
		s.frontendClient,
		metrics.NewNoopMetricsClient(),
	).(*taskFetcherImpl)
}

func (s *taskFetcherSuite) TearDownTest() {
	s.controller.Finish()
	s.mockResource.Finish(s.T())
}

func (s *taskFetcherSuite) TestGetMessages() {
	requestByShard := make(map[int32]*request)
	token := &types.ReplicationToken{
		ShardID:                0,
		LastProcessedMessageID: 1,
		LastRetrievedMessageID: 2,
	}
	requestByShard[0] = &request{
		token: token,
	}
	replicationMessageRequest := &types.GetReplicationMessagesRequest{
		Tokens: []*types.ReplicationToken{
			token,
		},
		ClusterName: "active",
	}
	messageByShared := make(map[int32]*types.ReplicationMessages)
	messageByShared[0] = &types.ReplicationMessages{}
	expectedResponse := &types.GetReplicationMessagesResponse{
		MessagesByShard: messageByShared,
	}
	s.frontendClient.EXPECT().GetReplicationMessages(gomock.Any(), replicationMessageRequest).Return(expectedResponse, nil)
	response, err := s.taskFetcher.getMessages(requestByShard)
	s.NoError(err)
	s.Equal(messageByShared, response)
}

func (s *taskFetcherSuite) TestFetchAndDistributeTasks() {
	requestByShard := make(map[int32]*request)
	token := &types.ReplicationToken{
		ShardID:                0,
		LastProcessedMessageID: 1,
		LastRetrievedMessageID: 2,
	}
	respChan := make(chan *types.ReplicationMessages, 1)
	requestByShard[0] = &request{
		token:    token,
		respChan: respChan,
	}
	replicationMessageRequest := &types.GetReplicationMessagesRequest{
		Tokens: []*types.ReplicationToken{
			token,
		},
		ClusterName: "active",
	}
	messageByShared := make(map[int32]*types.ReplicationMessages)
	messageByShared[0] = &types.ReplicationMessages{}
	expectedResponse := &types.GetReplicationMessagesResponse{
		MessagesByShard: messageByShared,
	}
	s.frontendClient.EXPECT().GetReplicationMessages(gomock.Any(), replicationMessageRequest).Return(expectedResponse, nil)
	err := s.taskFetcher.fetchAndDistributeTasks(requestByShard)
	s.NoError(err)
	respToken := <-respChan
	s.Equal(messageByShared[0], respToken)
}

func (s *taskFetcherSuite) TestLifecycle() {
	defer goleak.VerifyNone(s.T())
	mockTimeSource := clock.NewMockedTimeSourceAt(time.Now())
	s.taskFetcher.timeSource = mockTimeSource
	respChan0 := make(chan *types.ReplicationMessages, 1)
	respChan1 := make(chan *types.ReplicationMessages, 1)
	respChan2 := make(chan *types.ReplicationMessages, 1)
	respChan3 := make(chan *types.ReplicationMessages, 1)
	req0 := &request{
		token: &types.ReplicationToken{
			ShardID:                0,
			LastRetrievedMessageID: 100,
			LastProcessedMessageID: 10,
		},
		respChan: respChan0,
	}
	req1 := &request{
		token: &types.ReplicationToken{
			ShardID:                1,
			LastRetrievedMessageID: 10,
			LastProcessedMessageID: 1,
		},
		respChan: respChan1,
	}
	req2 := &request{
		token: &types.ReplicationToken{
			ShardID:                0,
			LastRetrievedMessageID: 10,
			LastProcessedMessageID: 1,
		},
		respChan: respChan2,
	}
	req3 := &request{
		token: &types.ReplicationToken{
			ShardID:                1,
			LastRetrievedMessageID: 11,
			LastProcessedMessageID: 2,
		},
		respChan: respChan3,
	}
	fetchAndDistributeTasksFnCall := 0
	fetchAndDistributeTasksSyncChan := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})}
	s.taskFetcher.fetchAndDistributeTasksFn = func(requestByShard map[int32]*request) error {
		defer func() {
			fetchAndDistributeTasksFnCall++
			close(fetchAndDistributeTasksSyncChan[fetchAndDistributeTasksFnCall-1])
		}()
		if fetchAndDistributeTasksFnCall == 0 {
			s.Equal(map[int32]*request{1: req1, 0: req2}, requestByShard)
			return &types.ServiceBusyError{}
		} else if fetchAndDistributeTasksFnCall == 1 {
			s.Equal(map[int32]*request{1: req3, 0: req2}, requestByShard)
			return &types.InternalServiceError{}
		} else if fetchAndDistributeTasksFnCall == 2 {
			s.Equal(map[int32]*request{1: req3, 0: req2}, requestByShard)
			for shard := range requestByShard {
				delete(requestByShard, shard)
			}
			return nil
		} else if fetchAndDistributeTasksFnCall == 3 {
			s.Equal(map[int32]*request{}, requestByShard)
			return nil
		}
		return nil
	}
	s.taskFetcher.Start()
	defer s.taskFetcher.Stop()

	requestChan := s.taskFetcher.GetRequestChan(0)
	// send 3 replication requests to the fetcher
	requestChan <- req0
	requestChan <- req1
	requestChan <- req2
	_, open := <-respChan0 // block until duplicate replication task is read from fetcher's request channel
	s.False(open)

	// process the existing replication requests and return service busy error
	s.Equal(0, fetchAndDistributeTasksFnCall)
	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(s.config.ReplicationTaskFetcherAggregationInterval())
	_, open = <-fetchAndDistributeTasksSyncChan[0] // block until fetchAndDistributeTasksFn is called
	s.False(open)
	s.Equal(1, fetchAndDistributeTasksFnCall)

	// send a new duplicate replication request to fetcher
	requestChan <- req3
	_, open = <-respChan1 // block until duplicate replication task is read from fetcher's request channel
	s.False(open)

	// process the existing replication requests and return non-service busy error
	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(s.config.ReplicationTaskFetcherServiceBusyWait())
	_, open = <-fetchAndDistributeTasksSyncChan[1] // block until fetchAndDistributeTasksFn is called
	s.False(open)
	s.Equal(2, fetchAndDistributeTasksFnCall)

	// process the existing replication requests and return success
	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(s.config.ReplicationTaskFetcherErrorRetryWait())
	_, open = <-fetchAndDistributeTasksSyncChan[2] // block until fetchAndDistributeTasksFn is called
	s.False(open)
	s.Equal(3, fetchAndDistributeTasksFnCall)

	// process empty requests and return success
	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(s.config.ReplicationTaskFetcherAggregationInterval())
	_, open = <-fetchAndDistributeTasksSyncChan[3] // block until fetchAndDistributeTasksFn is called
	s.False(open)
	s.Equal(4, fetchAndDistributeTasksFnCall)
}

func TestTaskFetchers(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	mockBean := client.NewMockBean(ctrl)
	mockAdminClient := admin.NewMockClient(ctrl)
	logger := testlogger.New(t)
	cfg := config.NewForTest()

	mockBean.EXPECT().GetRemoteAdminClient(cluster.TestAlternativeClusterName).Return(mockAdminClient, nil)
	fetchers, err := NewTaskFetchers(logger, cfg, cluster.TestActiveClusterMetadata, mockBean, metrics.NewNoopMetricsClient())
	assert.NoError(t, err)
	assert.NotNil(t, fetchers)
	assert.Len(t, fetchers.GetFetchers(), len(cluster.TestActiveClusterMetadata.GetRemoteClusterInfo()))

	fetchers.Start()
	fetchers.Stop()
}

func TestTaskFetcherParallelism(t *testing.T) {
	defer goleak.VerifyNone(t)
	logger := testlogger.New(t)
	cfg := config.NewForTest()
	parallelism := 4
	cfg.ReplicationTaskFetcherParallelism = dynamicproperties.GetIntPropertyFn(parallelism)

	ctrl := gomock.NewController(t)
	mockAdminClient := admin.NewMockClient(ctrl)

	fetcher := newReplicationTaskFetcher(
		logger,
		"standby",
		"active",
		cfg,
		mockAdminClient,
		metrics.NewNoopMetricsClient(),
	).(*taskFetcherImpl)

	// Test 1: Verify correct number of channels created
	assert.Equal(t, parallelism, len(fetcher.requestChan), "Should create 4 request channels")

	// Test 2: Verify shard-to-channel mapping
	chan0 := fetcher.GetRequestChan(0)
	chan1 := fetcher.GetRequestChan(1)
	chan4 := fetcher.GetRequestChan(4) // 4 % 4 = 0, should be same as chan0
	chan5 := fetcher.GetRequestChan(5) // 5 % 4 = 1, should be same as chan1

	assert.Equal(t, chan0, chan4, "Shards 0 and 4 should map to same channel (0 % 4 == 4 % 4)")
	assert.Equal(t, chan1, chan5, "Shards 1 and 5 should map to same channel (1 % 4 == 5 % 4)")
	assert.NotEqual(t, chan0, chan1, "Different channels should be different")

	// Test 3: Start fetcher and verify WaitGroup is properly incremented
	fetcher.Start()

	// The WaitGroup counter should now be 4 (one per goroutine)
	// We can verify this by calling Stop() which waits on the WaitGroup
	// If it hangs or times out, the goroutines weren't started correctly
	done := make(chan bool)
	go func() {
		fetcher.Stop()
		done <- true
	}()

	select {
	case <-done:
		// Success - all goroutines exited cleanly
	case <-time.After(11 * time.Second):
		t.Fatal("Stop() timed out - goroutines may not have been started correctly")
	}
}
