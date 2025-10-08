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

package task

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
)

type (
	taskPriorityAssignerSuite struct {
		*require.Assertions
		suite.Suite

		controller      *gomock.Controller
		mockDomainCache *cache.MockDomainCache

		config             *config.Config
		priorityAssigner   *priorityAssignerImpl
		testTaskProcessRPS int
	}
)

func TestTaskPriorityAssignerSuite(t *testing.T) {
	s := new(taskPriorityAssignerSuite)
	suite.Run(t, s)
}

func (s *taskPriorityAssignerSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockDomainCache = cache.NewMockDomainCache(s.controller)

	s.testTaskProcessRPS = 10
	client := dynamicconfig.NewInMemoryClient()
	err := client.UpdateValue(dynamicproperties.TaskProcessRPS, s.testTaskProcessRPS)
	s.NoError(err)
	dc := dynamicconfig.NewCollection(client, log.NewNoop())
	s.config = config.NewForTest()
	s.config.TaskProcessRPS = dc.GetIntPropertyFilteredByDomain(dynamicproperties.TaskProcessRPS)

	s.priorityAssigner = NewPriorityAssigner(
		cluster.TestCurrentClusterName,
		s.mockDomainCache,
		log.NewNoop(),
		metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{}),
		s.config,
	).(*priorityAssignerImpl)
}

func (s *taskPriorityAssignerSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *taskPriorityAssignerSuite) TestGetDomainInfo_Success_Active() {
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil)

	domainName, isActive, err := s.priorityAssigner.getDomainInfo(constants.TestDomainID)
	s.NoError(err)
	s.Equal(constants.TestDomainName, domainName)
	s.True(isActive)
}

func (s *taskPriorityAssignerSuite) TestGetDomainInfo_Success_Passive() {
	constants.TestGlobalDomainEntry.GetReplicationConfig().ActiveClusterName = cluster.TestAlternativeClusterName
	defer func() {
		constants.TestGlobalDomainEntry.GetReplicationConfig().ActiveClusterName = cluster.TestCurrentClusterName
	}()
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil)

	domainName, isActive, err := s.priorityAssigner.getDomainInfo(constants.TestDomainID)
	s.NoError(err)
	s.Equal(constants.TestDomainName, domainName)
	s.False(isActive)
}

func (s *taskPriorityAssignerSuite) TestGetDomainInfo_Success_Local() {
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestLocalDomainEntry, nil)

	domainName, isActive, err := s.priorityAssigner.getDomainInfo(constants.TestDomainID)
	s.NoError(err)
	s.Equal(constants.TestDomainName, domainName)
	s.True(isActive)
}

func (s *taskPriorityAssignerSuite) TestGetDomainInfo_Fail_DomainNotExist() {
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(
		nil,
		&types.EntityNotExistsError{Message: "domain not exist"},
	)

	domainName, isActive, err := s.priorityAssigner.getDomainInfo(constants.TestDomainID)
	s.NoError(err)
	s.Empty(domainName)
	s.True(isActive)
}

func (s *taskPriorityAssignerSuite) TestGetDomainInfo_Fail_UnknownError() {
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(
		nil,
		errors.New("some random error"),
	)

	domainName, isActive, err := s.priorityAssigner.getDomainInfo(constants.TestDomainID)
	s.Error(err)
	s.Empty(domainName)
	s.False(isActive)
}

func (s *taskPriorityAssignerSuite) TestAssign_ReplicationTask() {
	mockTask := NewMockTask(s.controller)
	mockTask.EXPECT().GetQueueType().Return(QueueTypeReplication).Times(1)
	mockTask.EXPECT().Priority().Return(noPriority).Times(1)
	mockTask.EXPECT().SetPriority(commonconstants.GetTaskPriority(commonconstants.LowPriorityClass, commonconstants.DefaultPrioritySubclass)).Times(1)

	err := s.priorityAssigner.Assign(mockTask)
	s.NoError(err)
}

func (s *taskPriorityAssignerSuite) TestAssign_StandbyTask_StandbyDomain() {
	constants.TestGlobalDomainEntry.GetReplicationConfig().ActiveClusterName = cluster.TestAlternativeClusterName
	defer func() {
		constants.TestGlobalDomainEntry.GetReplicationConfig().ActiveClusterName = cluster.TestCurrentClusterName
	}()
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil)

	mockTask := NewMockTask(s.controller)
	mockTask.EXPECT().GetQueueType().Return(QueueTypeStandbyTransfer).AnyTimes()
	mockTask.EXPECT().GetDomainID().Return(constants.TestDomainID).Times(1)
	mockTask.EXPECT().Priority().Return(noPriority).Times(1)
	mockTask.EXPECT().SetPriority(commonconstants.GetTaskPriority(commonconstants.LowPriorityClass, commonconstants.DefaultPrioritySubclass)).Times(1)

	err := s.priorityAssigner.Assign(mockTask)
	s.NoError(err)
}

func (s *taskPriorityAssignerSuite) TestAssign_StandbyTask_ActiveDomain() {
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil)

	mockTask := NewMockTask(s.controller)
	mockTask.EXPECT().GetQueueType().Return(QueueTypeStandbyTransfer).AnyTimes()
	mockTask.EXPECT().GetDomainID().Return(constants.TestDomainID).Times(1)
	mockTask.EXPECT().Priority().Return(noPriority).Times(1)
	mockTask.EXPECT().SetPriority(commonconstants.GetTaskPriority(commonconstants.HighPriorityClass, commonconstants.DefaultPrioritySubclass)).Times(1)

	err := s.priorityAssigner.Assign(mockTask)
	s.NoError(err)
}

func (s *taskPriorityAssignerSuite) TestAssign_ActiveTask_StandbyDomain() {
	constants.TestGlobalDomainEntry.GetReplicationConfig().ActiveClusterName = cluster.TestAlternativeClusterName
	defer func() {
		constants.TestGlobalDomainEntry.GetReplicationConfig().ActiveClusterName = cluster.TestCurrentClusterName
	}()
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil)

	mockTask := NewMockTask(s.controller)
	mockTask.EXPECT().GetQueueType().Return(QueueTypeActiveTimer).AnyTimes()
	mockTask.EXPECT().GetDomainID().Return(constants.TestDomainID).Times(1)
	mockTask.EXPECT().Priority().Return(noPriority).Times(1)
	mockTask.EXPECT().SetPriority(commonconstants.GetTaskPriority(commonconstants.HighPriorityClass, commonconstants.DefaultPrioritySubclass)).Times(1)

	err := s.priorityAssigner.Assign(mockTask)
	s.NoError(err)
}

func (s *taskPriorityAssignerSuite) TestAssign_ActiveTransferTask_ActiveDomain() {
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil)

	mockTask := NewMockTask(s.controller)
	mockTask.EXPECT().GetQueueType().Return(QueueTypeActiveTransfer).AnyTimes()
	mockTask.EXPECT().GetDomainID().Return(constants.TestDomainID).Times(1)
	mockTask.EXPECT().Priority().Return(noPriority).Times(1)
	mockTask.EXPECT().SetPriority(commonconstants.GetTaskPriority(commonconstants.HighPriorityClass, commonconstants.DefaultPrioritySubclass)).Times(1)

	err := s.priorityAssigner.Assign(mockTask)
	s.NoError(err)
}

func (s *taskPriorityAssignerSuite) TestAssign_ActiveTimerTask_ActiveDomain() {
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil)

	mockTask := NewMockTask(s.controller)
	mockTask.EXPECT().GetQueueType().Return(QueueTypeActiveTimer).AnyTimes()
	mockTask.EXPECT().GetDomainID().Return(constants.TestDomainID).Times(1)
	mockTask.EXPECT().Priority().Return(noPriority).Times(1)
	mockTask.EXPECT().SetPriority(commonconstants.GetTaskPriority(commonconstants.HighPriorityClass, commonconstants.DefaultPrioritySubclass)).Times(1)

	err := s.priorityAssigner.Assign(mockTask)
	s.NoError(err)
}

func (s *taskPriorityAssignerSuite) TestAssign_ThrottledTask() {
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestGlobalDomainEntry, nil).AnyTimes()

	for i := 0; i != s.testTaskProcessRPS*2; i++ {
		mockTask := NewMockTask(s.controller)
		mockTask.EXPECT().GetQueueType().Return(QueueTypeActiveTimer).AnyTimes()
		mockTask.EXPECT().GetDomainID().Return(constants.TestDomainID).Times(1)
		mockTask.EXPECT().Priority().Return(noPriority).Times(1)
		if i < s.testTaskProcessRPS {
			mockTask.EXPECT().SetPriority(commonconstants.GetTaskPriority(commonconstants.HighPriorityClass, commonconstants.DefaultPrioritySubclass)).Times(1)
		} else {
			mockTask.EXPECT().SetPriority(commonconstants.GetTaskPriority(commonconstants.DefaultPriorityClass, commonconstants.DefaultPrioritySubclass)).Times(1)
		}

		err := s.priorityAssigner.Assign(mockTask)
		s.NoError(err)
	}
}

func (s *taskPriorityAssignerSuite) TestAssign_AlreadyAssigned() {
	priority := 5

	// case 1: task attempt less than critical retry count
	mockTask := NewMockTask(s.controller)
	mockTask.EXPECT().Priority().Return(priority).Times(1)
	mockTask.EXPECT().GetAttempt().Return(s.config.TaskCriticalRetryCount() - 1).Times(1)
	err := s.priorityAssigner.Assign(mockTask)
	s.NoError(err)

	// case 2: task attempt higher than critical retry count
	mockTask.EXPECT().Priority().Return(priority).Times(1)
	mockTask.EXPECT().GetAttempt().Return(s.config.TaskCriticalRetryCount() + 1).Times(1)
	mockTask.EXPECT().SetPriority(lowTaskPriority).Times(1)
	err = s.priorityAssigner.Assign(mockTask)
	s.NoError(err)
}

func (s *taskPriorityAssignerSuite) TestGetTaskPriority() {
	testCases := []struct {
		class            int
		subClass         int
		expectedPriority int
	}{
		{
			class:            commonconstants.HighPriorityClass,
			subClass:         commonconstants.DefaultPrioritySubclass,
			expectedPriority: 1,
		},
		{
			class:            commonconstants.DefaultPriorityClass,
			subClass:         commonconstants.LowPrioritySubclass,
			expectedPriority: 10,
		},
		{
			class:            commonconstants.LowPriorityClass,
			subClass:         commonconstants.HighPrioritySubclass,
			expectedPriority: 16,
		},
	}

	for _, tc := range testCases {
		s.Equal(tc.expectedPriority, commonconstants.GetTaskPriority(tc.class, tc.subClass))
	}
}
