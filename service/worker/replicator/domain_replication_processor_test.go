// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
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
package replicator

import (
	"errors"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
)

type domainReplicationSuite struct {
	suite.Suite
	*require.Assertions
	controller *gomock.Controller

	sourceCluster          string
	currentCluster         string
	taskExecutor           *domain.MockReplicationTaskExecutor
	semaphoreTaskExecutor  *domain.MockSemaphoreReplicationTaskExecutor
	remoteClient           *admin.MockClient
	domainReplicationQueue *domain.MockReplicationQueue
	replicationProcessor   *domainReplicationProcessor
	timeSource             clock.MockedTimeSource
}

func TestDomainReplicationSuite(t *testing.T) {
	s := new(domainReplicationSuite)
	suite.Run(t, s)
}

func (s *domainReplicationSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.controller = gomock.NewController(s.T())
	resource := resource.NewTest(s.T(), s.controller, metrics.Worker)

	s.sourceCluster = "active"
	s.currentCluster = "standby"
	s.taskExecutor = domain.NewMockReplicationTaskExecutor(s.controller)
	s.semaphoreTaskExecutor = domain.NewMockSemaphoreReplicationTaskExecutor(s.controller)
	s.domainReplicationQueue = domain.NewMockReplicationQueue(s.controller)
	s.remoteClient = resource.RemoteAdminClient
	serviceResolver := resource.MembershipResolver
	serviceResolver.EXPECT().Lookup(service.Worker, s.sourceCluster).Return(resource.GetHostInfo(), nil).AnyTimes()
	s.timeSource = clock.NewMockedTimeSource()
	s.replicationProcessor = newDomainReplicationProcessor(
		s.sourceCluster,
		s.currentCluster,
		resource.GetLogger(),
		s.remoteClient,
		resource.GetMetricsClient(),
		s.taskExecutor,
		s.semaphoreTaskExecutor,
		resource.GetHostInfo(),
		serviceResolver,
		s.domainReplicationQueue,
		time.Millisecond,
		s.timeSource,
	)
	retryPolicy := backoff.NewExponentialRetryPolicy(time.Nanosecond)
	retryPolicy.SetMaximumAttempts(1)
	s.replicationProcessor.throttleRetry = backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(retryPolicy),
		backoff.WithRetryableError(isTransientRetryableError),
	)
}

func (s *domainReplicationSuite) TearDownTest() {
}

func (s *domainReplicationSuite) TestStartStop() {

	s.replicationProcessor.Start()

	// second call should be no-op
	s.replicationProcessor.Start()
	// yield execution so background goroutine starts
	time.Sleep(50 * time.Millisecond)

	s.remoteClient.EXPECT().
		GetDomainReplicationMessages(gomock.Any(), gomock.Any()).
		Return(&types.GetDomainReplicationMessagesResponse{
			Messages: &types.ReplicationMessages{},
		}, nil)

	// advance time to let it run one iteration
	s.timeSource.Advance(pollIntervalSecs * 5 * time.Second)
	// yield execution
	time.Sleep(50 * time.Millisecond)

	// stop processor
	s.replicationProcessor.Stop()

	// validate no goroutines left
	goleak.VerifyNone(s.T())
}

func (s *domainReplicationSuite) TestHandleDomainReplicationTask() {
	domainID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeDomain.Ptr(),
		DomainTaskAttributes: &types.DomainTaskAttributes{
			ID: domainID,
		},
	}

	s.taskExecutor.EXPECT().Execute(task.DomainTaskAttributes).Return(nil).Times(1)
	err := s.replicationProcessor.handleDomainReplicationTask(task)
	s.NoError(err)

	s.taskExecutor.EXPECT().Execute(task.DomainTaskAttributes).Return(errors.New("test")).Times(1)
	err = s.replicationProcessor.handleDomainReplicationTask(task)
	s.Error(err)

}

func (s *domainReplicationSuite) TestPutDomainReplicationTaskToDLQ() {
	domainID := uuid.New()
	task := &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeDomain.Ptr(),
	}

	err := s.replicationProcessor.putDomainReplicationTaskToDLQ(task)
	s.Error(err)

	task.DomainTaskAttributes = &types.DomainTaskAttributes{
		ID: domainID,
	}

	s.domainReplicationQueue.EXPECT().PublishToDLQ(gomock.Any(), task).Return(nil).Times(1)
	err = s.replicationProcessor.putDomainReplicationTaskToDLQ(task)
	s.NoError(err)

	s.domainReplicationQueue.EXPECT().PublishToDLQ(gomock.Any(), task).Return(errors.New("test")).Times(1)
	err = s.replicationProcessor.putDomainReplicationTaskToDLQ(task)
	s.Error(err)
}

func (s *domainReplicationSuite) TestFetchDomainReplicationTasks() {
	domainID1 := uuid.New()
	domainID2 := uuid.New()
	lastMessageID := int64(1000)
	resp := &types.GetDomainReplicationMessagesResponse{
		Messages: &types.ReplicationMessages{
			ReplicationTasks: []*types.ReplicationTask{
				{
					TaskType: types.ReplicationTaskTypeDomain.Ptr(),
					DomainTaskAttributes: &types.DomainTaskAttributes{
						ID: domainID1,
					},
				},
				{
					TaskType: types.ReplicationTaskTypeDomain.Ptr(),
					DomainTaskAttributes: &types.DomainTaskAttributes{
						ID: domainID2,
					},
				},
			},
			LastRetrievedMessageID: lastMessageID,
		},
	}
	s.remoteClient.EXPECT().GetDomainReplicationMessages(gomock.Any(), gomock.Any()).Return(resp, nil)
	s.taskExecutor.EXPECT().Execute(resp.Messages.ReplicationTasks[0].DomainTaskAttributes).Return(nil).Times(1)
	s.taskExecutor.EXPECT().Execute(resp.Messages.ReplicationTasks[1].DomainTaskAttributes).Return(nil).Times(1)

	s.replicationProcessor.fetchDomainReplicationTasks()
	s.Equal(lastMessageID, s.replicationProcessor.lastProcessedMessageID)
	s.Equal(lastMessageID, s.replicationProcessor.lastRetrievedMessageID)
}

func (s *domainReplicationSuite) TestFetchDomainReplicationTasks_Failed() {
	lastMessageID := int64(1000)
	s.remoteClient.EXPECT().GetDomainReplicationMessages(gomock.Any(), gomock.Any()).Return(nil, errors.New("test"))
	s.replicationProcessor.fetchDomainReplicationTasks()
	s.NotEqual(lastMessageID, s.replicationProcessor.lastProcessedMessageID)
	s.NotEqual(lastMessageID, s.replicationProcessor.lastRetrievedMessageID)
}

func (s *domainReplicationSuite) TestFetchDomainReplicationTasks_FailedOnExecution() {
	domainID1 := uuid.New()
	domainID2 := uuid.New()
	lastMessageID := int64(1000)
	resp := &types.GetDomainReplicationMessagesResponse{
		Messages: &types.ReplicationMessages{
			ReplicationTasks: []*types.ReplicationTask{
				{
					TaskType: types.ReplicationTaskTypeDomain.Ptr(),
					DomainTaskAttributes: &types.DomainTaskAttributes{
						ID: domainID1,
					},
				},
				{
					TaskType: types.ReplicationTaskTypeDomain.Ptr(),
					DomainTaskAttributes: &types.DomainTaskAttributes{
						ID: domainID2,
					},
				},
			},
			LastRetrievedMessageID: lastMessageID,
		},
	}
	s.remoteClient.EXPECT().GetDomainReplicationMessages(gomock.Any(), gomock.Any()).Return(resp, nil)
	s.taskExecutor.EXPECT().Execute(gomock.Any()).Return(errors.New("test")).AnyTimes()
	s.domainReplicationQueue.EXPECT().PublishToDLQ(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	s.replicationProcessor.fetchDomainReplicationTasks()
	s.Equal(lastMessageID, s.replicationProcessor.lastProcessedMessageID)
	s.Equal(lastMessageID, s.replicationProcessor.lastRetrievedMessageID)
}

func (s *domainReplicationSuite) TestFetchDomainReplicationTasks_FailedOnDLQ() {
	domainID1 := uuid.New()
	domainID2 := uuid.New()
	lastMessageID := int64(1001)
	resp := &types.GetDomainReplicationMessagesResponse{
		Messages: &types.ReplicationMessages{
			ReplicationTasks: []*types.ReplicationTask{
				{
					TaskType: types.ReplicationTaskTypeDomain.Ptr(),
					DomainTaskAttributes: &types.DomainTaskAttributes{
						ID: domainID1,
					},
				},
				{
					TaskType: types.ReplicationTaskTypeDomain.Ptr(),
					DomainTaskAttributes: &types.DomainTaskAttributes{
						ID: domainID2,
					},
				},
			},
			LastRetrievedMessageID: lastMessageID,
		},
	}
	s.remoteClient.EXPECT().GetDomainReplicationMessages(gomock.Any(), gomock.Any()).Return(resp, nil)
	s.taskExecutor.EXPECT().Execute(gomock.Any()).Return(nil).AnyTimes()
	s.domainReplicationQueue.EXPECT().PublishToDLQ(gomock.Any(), gomock.Any()).Return(errors.New("test")).Times(0)

	s.replicationProcessor.fetchDomainReplicationTasks()
	s.Equal(lastMessageID, s.replicationProcessor.lastProcessedMessageID)
	s.Equal(lastMessageID, s.replicationProcessor.lastRetrievedMessageID)
}

func TestHandleDomainReplicationTaskByType(t *testing.T) {
	domainTask := &types.DomainTaskAttributes{ID: "domain-id"}
	semaphoreTask := &types.SemaphoreMetadataTaskAttributes{DomainID: "domain-id", SemaphoreName: "shipping-api"}
	executorErr := errors.New("executor failed")
	tests := map[string]struct {
		task      *types.ReplicationTask
		mockSetup func(*domain.MockReplicationTaskExecutor, *domain.MockSemaphoreReplicationTaskExecutor)
		wantErr   error
	}{
		"semaphore metadata task": {
			task: &types.ReplicationTask{TaskType: types.ReplicationTaskTypeSemaphoreMetadata.Ptr(), SemaphoreMetadataTaskAttributes: semaphoreTask},
			mockSetup: func(_ *domain.MockReplicationTaskExecutor, sem *domain.MockSemaphoreReplicationTaskExecutor) {
				sem.EXPECT().Execute(semaphoreTask).Return(nil)
			},
		},
		"semaphore metadata task fails": {
			task: &types.ReplicationTask{TaskType: types.ReplicationTaskTypeSemaphoreMetadata.Ptr(), SemaphoreMetadataTaskAttributes: semaphoreTask},
			mockSetup: func(_ *domain.MockReplicationTaskExecutor, sem *domain.MockSemaphoreReplicationTaskExecutor) {
				sem.EXPECT().Execute(semaphoreTask).Return(executorErr)
			},
			wantErr: executorErr,
		},
		"task without a type goes to the domain executor": {
			task: &types.ReplicationTask{DomainTaskAttributes: domainTask},
			mockSetup: func(d *domain.MockReplicationTaskExecutor, _ *domain.MockSemaphoreReplicationTaskExecutor) {
				d.EXPECT().Execute(domainTask).Return(nil)
			},
		},
		"task of another type goes to the domain executor": {
			task: &types.ReplicationTask{TaskType: types.ReplicationTaskTypeHistoryV2.Ptr()},
			mockSetup: func(d *domain.MockReplicationTaskExecutor, _ *domain.MockSemaphoreReplicationTaskExecutor) {
				d.EXPECT().Execute(gomock.Nil()).Return(executorErr)
			},
			wantErr: executorErr,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			domainExecutor := domain.NewMockReplicationTaskExecutor(ctrl)
			semaphoreExecutor := domain.NewMockSemaphoreReplicationTaskExecutor(ctrl)
			tc.mockSetup(domainExecutor, semaphoreExecutor)
			p := &domainReplicationProcessor{
				taskExecutor:          domainExecutor,
				semaphoreTaskExecutor: semaphoreExecutor,
				metricsClient:         metrics.NewNoopMetricsClient(),
			}

			err := p.handleDomainReplicationTask(tc.task)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPutSemaphoreReplicationTaskToDLQ(t *testing.T) {
	tests := map[string]struct {
		task      *types.ReplicationTask
		mockSetup func(*domain.MockReplicationQueue, *types.ReplicationTask)
		wantErr   bool
	}{
		"semaphore task is published to the DLQ": {
			task: &types.ReplicationTask{
				TaskType:                        types.ReplicationTaskTypeSemaphoreMetadata.Ptr(),
				SemaphoreMetadataTaskAttributes: &types.SemaphoreMetadataTaskAttributes{DomainID: "domain-id", SemaphoreName: "shipping-api"},
			},
			mockSetup: func(q *domain.MockReplicationQueue, task *types.ReplicationTask) {
				q.EXPECT().PublishToDLQ(gomock.Any(), task).Return(nil)
			},
		},
		"semaphore task without attributes is rejected": {
			task:      &types.ReplicationTask{TaskType: types.ReplicationTaskTypeSemaphoreMetadata.Ptr()},
			mockSetup: func(*domain.MockReplicationQueue, *types.ReplicationTask) {},
			wantErr:   true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			queue := domain.NewMockReplicationQueue(ctrl)
			tc.mockSetup(queue, tc.task)
			p := &domainReplicationProcessor{
				domainReplicationQueue: queue,
				metricsClient:          metrics.NewNoopMetricsClient(),
			}

			err := p.putDomainReplicationTaskToDLQ(tc.task)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
