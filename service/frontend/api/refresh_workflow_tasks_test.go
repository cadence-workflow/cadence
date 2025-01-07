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

package api

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
	frontendcfg "github.com/uber/cadence/service/frontend/config"
)

var testDomainCacheEntry = cache.NewLocalDomainCacheEntryForTest(
	&persistence.DomainInfo{Name: "domain", ID: "domain-id"},
	&persistence.DomainConfig{},
	"",
)

type mockDeps struct {
	mockResource           *resource.Test
	mockDomainCache        *cache.MockDomainCache
	mockHistoryClient      *history.MockClient
	mockMatchingClient     *matching.MockClient
	mockProducer           *mocks.KafkaProducer
	mockMessagingClient    messaging.Client
	mockMetadataMgr        *mocks.MetadataManager
	mockHistoryV2Mgr       *mocks.HistoryV2Manager
	mockVisibilityMgr      *mocks.VisibilityManager
	mockArchivalMetadata   *archiver.MockArchivalMetadata
	mockArchiverProvider   *provider.MockArchiverProvider
	mockHistoryArchiver    *archiver.HistoryArchiverMock
	mockVisibilityArchiver *archiver.VisibilityArchiverMock
	mockVersionChecker     *client.MockVersionChecker
	mockTokenSerializer    *common.MockTaskTokenSerializer
	mockDomainHandler      *domain.MockHandler
	mockRequestValidator   *MockRequestValidator
	dynamicClient          dynamicconfig.Client
}

func setupMocksForWorkflowHandler(t *testing.T) (*WorkflowHandler, *mockDeps) {
	ctrl := gomock.NewController(t)
	mockResource := resource.NewTest(t, ctrl, metrics.Frontend)
	mockProducer := &mocks.KafkaProducer{}
	dynamicClient := dynamicconfig.NewInMemoryClient()
	deps := &mockDeps{
		mockResource:         mockResource,
		mockDomainCache:      mockResource.DomainCache,
		mockHistoryClient:    mockResource.HistoryClient,
		mockMatchingClient:   mockResource.MatchingClient,
		mockMetadataMgr:      mockResource.MetadataMgr,
		mockHistoryV2Mgr:     mockResource.HistoryMgr,
		mockVisibilityMgr:    mockResource.VisibilityMgr,
		mockArchivalMetadata: mockResource.ArchivalMetadata,
		mockArchiverProvider: mockResource.ArchiverProvider,
		mockTokenSerializer:  common.NewMockTaskTokenSerializer(ctrl),

		mockProducer:           mockProducer,
		mockMessagingClient:    mocks.NewMockMessagingClient(mockProducer, nil),
		mockHistoryArchiver:    &archiver.HistoryArchiverMock{},
		mockVisibilityArchiver: &archiver.VisibilityArchiverMock{},
		mockVersionChecker:     client.NewMockVersionChecker(ctrl),
		mockDomainHandler:      domain.NewMockHandler(ctrl),
		mockRequestValidator:   NewMockRequestValidator(ctrl),
		dynamicClient:          dynamicClient,
	}

	config := frontendcfg.NewConfig(
		dynamicconfig.NewCollection(
			dynamicClient,
			testlogger.New(t),
		),
		numHistoryShards,
		false,
		"hostname",
	)
	wh := NewWorkflowHandler(deps.mockResource, config, deps.mockVersionChecker, deps.mockDomainHandler)
	wh.requestValidator = deps.mockRequestValidator
	return wh, deps
}

func TestRefreshWorkflowTasks(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.RefreshWorkflowTasksRequest
		setupMocks    func(*mockDeps)
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.RefreshWorkflowTasksRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wf",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateRefreshWorkflowTasksRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainCache.EXPECT().GetDomain("domain").Return(testDomainCacheEntry, nil)
				deps.mockHistoryClient.EXPECT().RefreshWorkflowTasks(gomock.Any(), &types.HistoryRefreshWorkflowTasksRequest{
					DomainUIID: "domain-id",
					Request: &types.RefreshWorkflowTasksRequest{
						Domain: "domain",
						Execution: &types.WorkflowExecution{
							WorkflowID: "wf",
						},
					},
				}).Return(nil)
			},
			expectError: false,
		},
		{
			name: "history client error",
			req: &types.RefreshWorkflowTasksRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wf",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateRefreshWorkflowTasksRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainCache.EXPECT().GetDomain("domain").Return(testDomainCacheEntry, nil)
				deps.mockHistoryClient.EXPECT().RefreshWorkflowTasks(gomock.Any(), &types.HistoryRefreshWorkflowTasksRequest{
					DomainUIID: "domain-id",
					Request: &types.RefreshWorkflowTasksRequest{
						Domain: "domain",
						Execution: &types.WorkflowExecution{
							WorkflowID: "wf",
						},
					},
				}).Return(errors.New("history error"))
			},
			expectError:   true,
			expectedError: "history error",
		},
		{
			name: "cache error",
			req: &types.RefreshWorkflowTasksRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wf",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateRefreshWorkflowTasksRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainCache.EXPECT().GetDomain("domain").Return(nil, errors.New("cache error"))
			},
			expectError:   true,
			expectedError: "cache error",
		},
		{
			name: "validator error",
			req: &types.RefreshWorkflowTasksRequest{
				Domain: "domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "wf",
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateRefreshWorkflowTasksRequest(gomock.Any(), gomock.Any()).Return(errors.New("validator error"))
			},
			expectError:   true,
			expectedError: "validator error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			tc.setupMocks(deps)
			err := wh.RefreshWorkflowTasks(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
