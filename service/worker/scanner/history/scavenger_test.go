// Copyright (c) 2017 Uber Technologies, Inc.
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

package history

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	ScavengerTestSuite struct {
		suite.Suite
		logger    log.Logger
		metric    metrics.Client
		mockCache *cache.MockDomainCache
	}
)

func TestScavengerTestSuite(t *testing.T) {
	suite.Run(t, new(ScavengerTestSuite))
}

func (s *ScavengerTestSuite) SetupTest() {
	s.logger = testlogger.New(s.T())
	s.metric = metrics.NewClient(tally.NoopScope, metrics.Worker, metrics.HistogramMigration{})
	controller := gomock.NewController(s.T())
	s.mockCache = cache.NewMockDomainCache(controller)
}

func (s *ScavengerTestSuite) createTestScavenger(rps int) (*mocks.HistoryV2Manager, *history.MockClient, *Scavenger) {
	db := &mocks.HistoryV2Manager{}
	controller := gomock.NewController(s.T())
	workflowClient := history.NewMockClient(controller)
	maxWorkflowRetentionInDays := dynamicproperties.GetIntPropertyFn(dynamicproperties.MaxRetentionDays.DefaultInt())
	scvgr := NewScavenger(db, rps, workflowClient, ScavengerHeartbeatDetails{}, s.metric, s.logger, maxWorkflowRetentionInDays, s.mockCache)
	scvgr.isInTest = true
	return db, workflowClient, scvgr
}

func (s *ScavengerTestSuite) TestAllSkipTasksTwoPages() {
	db, _, scvgr := s.createTestScavenger(100)
	db.On("GetAllHistoryTreeBranches", mock.Anything, &p.GetAllHistoryTreeBranchesRequest{
		PageSize: pageSize,
	}).Return(&p.GetAllHistoryTreeBranchesResponse{
		NextPageToken: []byte("page1"),
		Branches: []p.HistoryBranchDetail{
			{
				TreeID:   "treeID1",
				BranchID: "branchID1",
				ForkTime: time.Now(),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID1", "workflowID1", "runID1"),
			},
			{
				TreeID:   "treeID2",
				BranchID: "branchID2",
				ForkTime: time.Now(),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID2", "workflowID2", "runID2"),
			},
		},
	}, nil).Once()

	db.On("GetAllHistoryTreeBranches", mock.Anything, &p.GetAllHistoryTreeBranchesRequest{
		PageSize:      pageSize,
		NextPageToken: []byte("page1"),
	}).Return(&p.GetAllHistoryTreeBranchesResponse{
		Branches: []p.HistoryBranchDetail{
			{
				TreeID:   "treeID3",
				BranchID: "branchID3",
				ForkTime: time.Now(),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID3", "workflowID3", "runID3"),
			},
			{
				TreeID:   "treeID4",
				BranchID: "branchID4",
				ForkTime: time.Now(),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID4", "workflowID4", "runID4"),
			},
		},
	}, nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hbd, err := scvgr.Run(ctx)
	s.Nil(err)
	s.Equal(4, hbd.SkipCount)
	s.Equal(0, hbd.SuccCount)
	s.Equal(0, hbd.ErrorCount)
	s.Equal(2, hbd.CurrentPage)
	s.Equal(0, len(hbd.NextPageToken))
}

func (s *ScavengerTestSuite) TestAllErrorSplittingTasksTwoPages() {
	db, _, scvgr := s.createTestScavenger(100)
	db.On("GetAllHistoryTreeBranches", mock.Anything, &p.GetAllHistoryTreeBranchesRequest{
		PageSize: pageSize,
	}).Return(&p.GetAllHistoryTreeBranchesResponse{
		NextPageToken: []byte("page1"),
		Branches: []p.HistoryBranchDetail{
			{
				TreeID:   "treeID1",
				BranchID: "branchID1",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     "error-info",
			},
			{
				TreeID:   "treeID2",
				BranchID: "branchID2",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     "error-info",
			},
		},
	}, nil).Once()

	db.On("GetAllHistoryTreeBranches", mock.Anything, &p.GetAllHistoryTreeBranchesRequest{
		PageSize:      pageSize,
		NextPageToken: []byte("page1"),
	}).Return(&p.GetAllHistoryTreeBranchesResponse{
		Branches: []p.HistoryBranchDetail{
			{
				TreeID:   "treeID3",
				BranchID: "branchID3",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     "error-info",
			},
			{
				TreeID:   "treeID4",
				BranchID: "branchID4",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     "error-info",
			},
		},
	}, nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hbd, err := scvgr.Run(ctx)
	s.Nil(err)
	s.Equal(0, hbd.SkipCount)
	s.Equal(0, hbd.SuccCount)
	s.Equal(4, hbd.ErrorCount)
	s.Equal(2, hbd.CurrentPage)
	s.Equal(0, len(hbd.NextPageToken))
}

func (s *ScavengerTestSuite) TestNoGarbageTwoPages() {
	db, client, scvgr := s.createTestScavenger(100)
	db.On("GetAllHistoryTreeBranches", mock.Anything, &p.GetAllHistoryTreeBranchesRequest{
		PageSize: pageSize,
	}).Return(&p.GetAllHistoryTreeBranchesResponse{
		NextPageToken: []byte("page1"),
		Branches: []p.HistoryBranchDetail{
			{
				TreeID:   "treeID1",
				BranchID: "branchID1",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID1", "workflowID1", "runID1"),
			},
			{
				TreeID:   "treeID2",
				BranchID: "branchID2",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID2", "workflowID2", "runID2"),
			},
		},
	}, nil).Once()

	db.On("GetAllHistoryTreeBranches", mock.Anything, &p.GetAllHistoryTreeBranchesRequest{
		PageSize:      pageSize,
		NextPageToken: []byte("page1"),
	}).Return(&p.GetAllHistoryTreeBranchesResponse{
		Branches: []p.HistoryBranchDetail{
			{
				TreeID:   "treeID3",
				BranchID: "branchID3",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID3", "workflowID3", "runID3"),
			},
			{
				TreeID:   "treeID4",
				BranchID: "branchID4",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID4", "workflowID4", "runID4"),
			},
		},
	}, nil).Once()

	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID1",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID1",
			RunID:      "runID1",
		},
	}).Return(nil, nil)
	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID2",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID2",
			RunID:      "runID2",
		},
	}).Return(nil, nil)
	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID3",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID3",
			RunID:      "runID3",
		},
	}).Return(nil, nil)
	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID4",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID4",
			RunID:      "runID4",
		},
	}).Return(nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hbd, err := scvgr.Run(ctx)
	s.Nil(err)
	s.Equal(0, hbd.SkipCount)
	s.Equal(4, hbd.SuccCount)
	s.Equal(0, hbd.ErrorCount)
	s.Equal(2, hbd.CurrentPage)
	s.Equal(0, len(hbd.NextPageToken))
}

func (s *ScavengerTestSuite) TestDeletingBranchesTwoPages() {
	db, client, scvgr := s.createTestScavenger(100)
	db.On("GetAllHistoryTreeBranches", mock.Anything, &p.GetAllHistoryTreeBranchesRequest{
		PageSize: pageSize,
	}).Return(&p.GetAllHistoryTreeBranchesResponse{
		NextPageToken: []byte("page1"),
		Branches: []p.HistoryBranchDetail{
			{
				TreeID:   "treeID1",
				BranchID: "branchID1",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID1", "workflowID1", "runID1"),
			},
			{
				TreeID:   "treeID2",
				BranchID: "branchID2",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID2", "workflowID2", "runID2"),
			},
		},
	}, nil).Once()
	db.On("GetAllHistoryTreeBranches", mock.Anything, &p.GetAllHistoryTreeBranchesRequest{
		PageSize:      pageSize,
		NextPageToken: []byte("page1"),
	}).Return(&p.GetAllHistoryTreeBranchesResponse{
		Branches: []p.HistoryBranchDetail{
			{
				TreeID:   "treeID3",
				BranchID: "branchID3",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID3", "workflowID3", "runID3"),
			},
			{
				TreeID:   "treeID4",
				BranchID: "branchID4",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID4", "workflowID4", "runID4"),
			},
		},
	}, nil).Once()

	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID1",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID1",
			RunID:      "runID1",
		},
	}).Return(nil, &types.EntityNotExistsError{})
	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID2",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID2",
			RunID:      "runID2",
		},
	}).Return(nil, &types.EntityNotExistsError{})
	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID3",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID3",
			RunID:      "runID3",
		},
	}).Return(nil, &types.EntityNotExistsError{})
	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID4",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID4",
			RunID:      "runID4",
		},
	}).Return(nil, &types.EntityNotExistsError{})
	domainName := "test-domainName"
	s.mockCache.EXPECT().GetDomainName(gomock.Any()).Return(domainName, nil).AnyTimes()
	branchToken1, err := p.NewHistoryBranchTokenByBranchID("treeID1", "branchID1")
	s.Nil(err)
	db.On("DeleteHistoryBranch", mock.Anything, &p.DeleteHistoryBranchRequest{
		BranchToken: branchToken1,
		ShardID:     common.IntPtr(1),
		DomainName:  domainName,
	}).Return(nil).Once()
	branchToken2, err := p.NewHistoryBranchTokenByBranchID("treeID2", "branchID2")
	s.Nil(err)
	db.On("DeleteHistoryBranch", mock.Anything, &p.DeleteHistoryBranchRequest{
		BranchToken: branchToken2,
		ShardID:     common.IntPtr(1),
		DomainName:  domainName,
	}).Return(nil).Once()
	branchToken3, err := p.NewHistoryBranchTokenByBranchID("treeID3", "branchID3")
	s.Nil(err)
	db.On("DeleteHistoryBranch", mock.Anything, &p.DeleteHistoryBranchRequest{
		BranchToken: branchToken3,
		ShardID:     common.IntPtr(1),
		DomainName:  domainName,
	}).Return(nil).Once()
	branchToken4, err := p.NewHistoryBranchTokenByBranchID("treeID4", "branchID4")
	s.Nil(err)
	db.On("DeleteHistoryBranch", mock.Anything, &p.DeleteHistoryBranchRequest{
		BranchToken: branchToken4,
		ShardID:     common.IntPtr(1),
		DomainName:  domainName,
	}).Return(nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hbd, err := scvgr.Run(ctx)
	s.Nil(err)
	s.Equal(0, hbd.SkipCount)
	s.Equal(4, hbd.SuccCount)
	s.Equal(0, hbd.ErrorCount)
	s.Equal(2, hbd.CurrentPage)
	s.Equal(0, len(hbd.NextPageToken))
}

func (s *ScavengerTestSuite) TestMixesTwoPages() {
	db, client, scvgr := s.createTestScavenger(100)
	db.On("GetAllHistoryTreeBranches", mock.Anything, &p.GetAllHistoryTreeBranchesRequest{
		PageSize: pageSize,
	}).Return(&p.GetAllHistoryTreeBranchesResponse{
		NextPageToken: []byte("page1"),
		Branches: []p.HistoryBranchDetail{
			{
				// skip
				TreeID:   "treeID1",
				BranchID: "branchID1",
				ForkTime: time.Now(),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID1", "workflowID1", "runID1"),
			},
			{
				// split error
				TreeID:   "treeID2",
				BranchID: "branchID2",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     "error-info",
			},
		},
	}, nil).Once()
	db.On("GetAllHistoryTreeBranches", mock.Anything, &p.GetAllHistoryTreeBranchesRequest{
		PageSize:      pageSize,
		NextPageToken: []byte("page1"),
	}).Return(&p.GetAllHistoryTreeBranchesResponse{
		Branches: []p.HistoryBranchDetail{
			{
				// delete succ
				TreeID:   "treeID3",
				BranchID: "branchID3",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID3", "workflowID3", "runID3"),
			},
			{
				// delete fail
				TreeID:   "treeID4",
				BranchID: "branchID4",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID4", "workflowID4", "runID4"),
			},
			{
				// not delete
				TreeID:   "treeID5",
				BranchID: "branchID5",
				ForkTime: time.Now().Add(-getHistoryCleanupThreshold(dynamicproperties.MaxRetentionDays.DefaultInt()) * 2),
				Info:     p.BuildHistoryGarbageCleanupInfo("domainID5", "workflowID5", "runID5"),
			},
		},
	}, nil).Once()

	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID3",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID3",
			RunID:      "runID3",
		},
	}).Return(nil, &types.EntityNotExistsError{})

	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID4",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID4",
			RunID:      "runID4",
		},
	}).Return(nil, &types.EntityNotExistsError{})
	client.EXPECT().DescribeMutableState(gomock.Any(), &types.DescribeMutableStateRequest{
		DomainUUID: "domainID5",
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID5",
			RunID:      "runID5",
		},
	}).Return(nil, nil)
	domainName := "test-domainName"
	s.mockCache.EXPECT().GetDomainName(gomock.Any()).Return(domainName, nil).AnyTimes()
	branchToken3, err := p.NewHistoryBranchTokenByBranchID("treeID3", "branchID3")
	s.Nil(err)
	db.On("DeleteHistoryBranch", mock.Anything, &p.DeleteHistoryBranchRequest{
		BranchToken: branchToken3,
		ShardID:     common.IntPtr(1),
		DomainName:  domainName,
	}).Return(nil).Once()

	branchToken4, err := p.NewHistoryBranchTokenByBranchID("treeID4", "branchID4")
	s.Nil(err)
	db.On("DeleteHistoryBranch", mock.Anything, &p.DeleteHistoryBranchRequest{
		BranchToken: branchToken4,
		ShardID:     common.IntPtr(1),
		DomainName:  domainName,
	}).Return(fmt.Errorf("failed to delete history")).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hbd, err := scvgr.Run(ctx)
	s.Nil(err)
	s.Equal(1, hbd.SkipCount)
	s.Equal(2, hbd.SuccCount)
	s.Equal(2, hbd.ErrorCount)
	s.Equal(2, hbd.CurrentPage)
	s.Equal(0, len(hbd.NextPageToken))
}
