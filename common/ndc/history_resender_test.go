// Copyright (c) 2019 Uber Technologies, Inc.
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

package ndc

import (
	"context"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/types"
)

type (
	historyResenderSuite struct {
		suite.Suite
		*require.Assertions

		controller        *gomock.Controller
		mockDomainCache   *cache.MockDomainCache
		mockAdminClient   *admin.MockClient
		mockHistoryClient *history.MockClient

		domainID   string
		domainName string

		serializer persistence.PayloadSerializer
		logger     log.Logger

		rereplicator *HistoryResenderImpl
	}
)

func TestHistoryResenderSuite(t *testing.T) {
	s := new(historyResenderSuite)
	suite.Run(t, s)
}

func (s *historyResenderSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockAdminClient = admin.NewMockClient(s.controller)
	s.mockHistoryClient = history.NewMockClient(s.controller)
	s.mockDomainCache = cache.NewMockDomainCache(s.controller)

	s.logger = testlogger.New(s.Suite.T())

	s.domainID = uuid.New()
	s.domainName = "some random domain name"
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: s.domainID, Name: s.domainName},
		&persistence.DomainConfig{Retention: 1},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234,
	)
	s.mockDomainCache.EXPECT().GetDomainName(s.domainID).Return(s.domainName, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomain(s.domainName).Return(domainEntry, nil).AnyTimes()
	s.serializer = persistence.NewPayloadSerializer()

	s.rereplicator = NewHistoryResender(
		s.mockDomainCache,
		s.mockAdminClient,
		func(ctx context.Context, request *types.ReplicateEventsV2Request) error {
			return s.mockHistoryClient.ReplicateEventsV2(ctx, request)
		},
		nil,
		nil,
		s.logger,
	)
}

func (s *historyResenderSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *historyResenderSuite) TestSendSingleWorkflowHistory() {
	workflowID := "some random workflow ID"
	runID := uuid.New()
	startEventID := int64(123)
	startEventVersion := int64(100)
	token := []byte{1}
	pageSize := defaultPageSize
	eventBatch := []*types.HistoryEvent{
		{
			ID:        2,
			Version:   123,
			Timestamp: common.Int64Ptr(time.Now().UnixNano()),
			EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
		},
		{
			ID:        3,
			Version:   123,
			Timestamp: common.Int64Ptr(time.Now().UnixNano()),
			EventType: types.EventTypeDecisionTaskStarted.Ptr(),
		},
	}
	blob := s.serializeEvents(eventBatch)
	versionHistoryItems := []*types.VersionHistoryItem{
		{
			EventID: 1,
			Version: 1,
		},
	}

	s.mockAdminClient.EXPECT().GetWorkflowExecutionRawHistoryV2(
		gomock.Any(),
		&types.GetWorkflowExecutionRawHistoryV2Request{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      runID,
			},
			StartEventID:      common.Int64Ptr(startEventID),
			StartEventVersion: common.Int64Ptr(startEventVersion),
			MaximumPageSize:   pageSize,
			NextPageToken:     nil,
		}).Return(&types.GetWorkflowExecutionRawHistoryV2Response{
		HistoryBatches: []*types.DataBlob{blob},
		NextPageToken:  token,
		VersionHistory: &types.VersionHistory{
			Items: versionHistoryItems,
		},
	}, nil).Times(1)

	s.mockAdminClient.EXPECT().GetWorkflowExecutionRawHistoryV2(
		gomock.Any(),
		&types.GetWorkflowExecutionRawHistoryV2Request{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      runID,
			},
			StartEventID:      common.Int64Ptr(startEventID),
			StartEventVersion: common.Int64Ptr(startEventVersion),
			MaximumPageSize:   pageSize,
			NextPageToken:     token,
		}).Return(&types.GetWorkflowExecutionRawHistoryV2Response{
		HistoryBatches: []*types.DataBlob{blob},
		NextPageToken:  nil,
		VersionHistory: &types.VersionHistory{
			Items: versionHistoryItems,
		},
	}, nil).Times(1)

	s.mockHistoryClient.EXPECT().ReplicateEventsV2(
		gomock.Any(),
		&types.ReplicateEventsV2Request{
			DomainUUID: s.domainID,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      runID,
			},
			VersionHistoryItems: versionHistoryItems,
			Events:              blob,
		}).Return(nil).Times(2)

	err := s.rereplicator.SendSingleWorkflowHistory(
		s.domainID,
		workflowID,
		runID,
		common.Int64Ptr(startEventID),
		common.Int64Ptr(startEventVersion),
		nil,
		nil,
	)

	s.Nil(err)
}

func (s *historyResenderSuite) TestCreateReplicateRawEventsRequest() {
	workflowID := "some random workflow ID"
	runID := uuid.New()
	blob := &types.DataBlob{
		EncodingType: types.EncodingTypeThriftRW.Ptr(),
		Data:         []byte("some random history blob"),
	}
	versionHistoryItems := []*types.VersionHistoryItem{
		{
			EventID: 1,
			Version: 1,
		},
	}

	s.Equal(&types.ReplicateEventsV2Request{
		DomainUUID: s.domainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		VersionHistoryItems: versionHistoryItems,
		Events:              blob,
	}, s.rereplicator.createReplicationRawRequest(
		s.domainID,
		workflowID,
		runID,
		blob,
		versionHistoryItems))
}

func (s *historyResenderSuite) TestSendReplicationRawRequest() {
	workflowID := "some random workflow ID"
	runID := uuid.New()
	item := &types.VersionHistoryItem{
		EventID: 1,
		Version: 1,
	}
	request := &types.ReplicateEventsV2Request{
		DomainUUID: s.domainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		Events: &types.DataBlob{
			EncodingType: types.EncodingTypeThriftRW.Ptr(),
			Data:         []byte("some random history blob"),
		},
		VersionHistoryItems: []*types.VersionHistoryItem{item},
	}

	s.mockHistoryClient.EXPECT().ReplicateEventsV2(gomock.Any(), request).Return(nil).Times(1)
	err := s.rereplicator.sendReplicationRawRequest(context.Background(), request)
	s.Nil(err)
}

func (s *historyResenderSuite) TestSendReplicationRawRequest_Err() {
	workflowID := "some random workflow ID"
	runID := uuid.New()
	item := &types.VersionHistoryItem{
		EventID: 1,
		Version: 1,
	}
	request := &types.ReplicateEventsV2Request{
		DomainUUID: s.domainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		Events: &types.DataBlob{
			EncodingType: types.EncodingTypeThriftRW.Ptr(),
			Data:         []byte("some random history blob"),
		},
		VersionHistoryItems: []*types.VersionHistoryItem{item},
	}
	retryErr := &types.RetryTaskV2Error{
		DomainID:   s.domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}

	s.mockHistoryClient.EXPECT().ReplicateEventsV2(gomock.Any(), request).Return(retryErr).Times(1)
	err := s.rereplicator.sendReplicationRawRequest(context.Background(), request)
	s.Equal(retryErr, err)
}

func (s *historyResenderSuite) TestGetHistory() {
	workflowID := "some random workflow ID"
	runID := uuid.New()
	startEventID := int64(123)
	endEventID := int64(345)
	version := int64(20)
	nextTokenIn := []byte("some random next token in")
	nextTokenOut := []byte("some random next token out")
	pageSize := int32(59)
	blob := []byte("some random events blob")
	encodingTypeThriftRW := types.EncodingTypeThriftRW

	response := &types.GetWorkflowExecutionRawHistoryV2Response{
		HistoryBatches: []*types.DataBlob{&types.DataBlob{
			EncodingType: &encodingTypeThriftRW,
			Data:         blob,
		}},
		NextPageToken: nextTokenOut,
	}
	s.mockAdminClient.EXPECT().GetWorkflowExecutionRawHistoryV2(gomock.Any(), &types.GetWorkflowExecutionRawHistoryV2Request{
		Domain: s.domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		StartEventID:      common.Int64Ptr(startEventID),
		StartEventVersion: common.Int64Ptr(version),
		EndEventID:        common.Int64Ptr(endEventID),
		EndEventVersion:   common.Int64Ptr(version),
		MaximumPageSize:   pageSize,
		NextPageToken:     nextTokenIn,
	}).Return(response, nil).Times(1)

	out, err := s.rereplicator.getHistory(
		context.Background(),
		s.domainName,
		workflowID,
		runID,
		&startEventID,
		&version,
		&endEventID,
		&version,
		nextTokenIn,
		pageSize)
	s.Nil(err)
	s.Equal(response, out)
}

func (s *historyResenderSuite) TestCurrentExecutionCheck() {
	domainID := uuid.New()
	workflowID1 := uuid.New()
	workflowID2 := uuid.New()
	runID := uuid.New()
	invariantMock := invariant.NewMockInvariant(s.controller)
	s.rereplicator = NewHistoryResender(
		s.mockDomainCache,
		s.mockAdminClient,
		func(ctx context.Context, request *types.ReplicateEventsV2Request) error {
			return s.mockHistoryClient.ReplicateEventsV2(ctx, request)
		},
		nil,
		invariantMock,
		s.logger,
	)
	execution1 := &entity.CurrentExecution{
		Execution: entity.Execution{
			DomainID:   domainID,
			WorkflowID: workflowID1,
			State:      persistence.WorkflowStateRunning,
		},
	}
	execution2 := &entity.CurrentExecution{
		Execution: entity.Execution{
			DomainID:   domainID,
			WorkflowID: workflowID2,
			State:      persistence.WorkflowStateRunning,
		},
	}
	invariantMock.EXPECT().Check(gomock.Any(), execution1).Return(invariant.CheckResult{
		CheckResultType: invariant.CheckResultTypeCorrupted,
	}).Times(1)
	invariantMock.EXPECT().Check(gomock.Any(), execution2).Return(invariant.CheckResult{
		CheckResultType: invariant.CheckResultTypeHealthy,
	}).Times(1)
	invariantMock.EXPECT().Fix(gomock.Any(), gomock.Any()).Return(invariant.FixResult{}).Times(1)

	skipTask := s.rereplicator.fixCurrentExecution(context.Background(), domainID, workflowID1, runID)
	s.False(skipTask)
	skipTask = s.rereplicator.fixCurrentExecution(context.Background(), domainID, workflowID2, runID)
	s.True(skipTask)
}

func (s *historyResenderSuite) serializeEvents(events []*types.HistoryEvent) *types.DataBlob {
	blob, err := s.serializer.SerializeBatchEvents(events, constants.EncodingTypeThriftRW)
	s.Nil(err)
	return &types.DataBlob{
		EncodingType: types.EncodingTypeThriftRW.Ptr(),
		Data:         blob.Data,
	}
}
