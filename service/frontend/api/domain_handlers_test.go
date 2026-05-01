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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestDeprecateDomain(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.DeprecateDomainRequest
		setupMocks    func(*mockDeps)
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.DeprecateDomainRequest{
				Name: "domain-name",
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDeprecateDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DeprecateDomain(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectError: false,
		},
		{
			name: "validation error",
			req: &types.DeprecateDomainRequest{
				Name: "domain-name",
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDeprecateDomainRequest(gomock.Any(), gomock.Any()).Return(errors.New("validation error"))
			},
			expectError:   true,
			expectedError: "validation error",
		},
		{
			name: "deprecate domain handler error",
			req: &types.DeprecateDomainRequest{
				Name: "domain-name",
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDeprecateDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DeprecateDomain(gomock.Any(), gomock.Any()).Return(errors.New("handler error"))
			},
			expectError:   true,
			expectedError: "handler error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			tc.setupMocks(deps)

			err := wh.DeprecateDomain(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegisterDomain(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.RegisterDomainRequest
		setupMocks    func(*mockDeps)
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.RegisterDomainRequest{
				Name: "domain-name",
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateRegisterDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().RegisterDomain(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectError: false,
		},
		{
			name: "validation error",
			req: &types.RegisterDomainRequest{
				Name: "domain-name",
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateRegisterDomainRequest(gomock.Any(), gomock.Any()).Return(errors.New("validation error"))
			},
			expectError:   true,
			expectedError: "validation error",
		},
		{
			name: "register domain handler error",
			req: &types.RegisterDomainRequest{
				Name: "domain-name",
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateRegisterDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().RegisterDomain(gomock.Any(), gomock.Any()).Return(errors.New("handler error"))
			},
			expectError:   true,
			expectedError: "handler error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			tc.setupMocks(deps)
			err := wh.RegisterDomain(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDescribeDomain(t *testing.T) {
	domainName := "domain-name" // Define the domain name pointer to pass in requests
	testCases := []struct {
		name          string
		req           *types.DescribeDomainRequest
		setupMocks    func(*mockDeps)
		expectError   bool
		expectedError string
		verifyResp    func(t *testing.T, resp *types.DescribeDomainResponse)
	}{
		{
			name: "success without failover info",
			req: &types.DescribeDomainRequest{
				Name: &domainName,
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDescribeDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{
						Name: "domain-name",
					},
					FailoverInfo: nil,
				}, nil)
			},
			expectError: false,
			verifyResp: func(t *testing.T, resp *types.DescribeDomainResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, "domain-name", resp.DomainInfo.Name)
			},
		},
		{
			name: "success with failover info and no error from history client",
			req: &types.DescribeDomainRequest{
				Name: &domainName,
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDescribeDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{
						Name: "domain-name",
						UUID: "domain-id",
					},
					FailoverInfo: &types.FailoverInfo{
						FailoverExpireTimestamp: 1000,
					},
				}, nil)
				deps.mockHistoryClient.EXPECT().GetFailoverInfo(gomock.Any(), &types.GetFailoverInfoRequest{
					DomainID: "domain-id",
				}).Return(&types.GetFailoverInfoResponse{
					CompletedShardCount: 5,
					PendingShards:       []int32{10},
				}, nil)
			},
			expectError: false,
			verifyResp: func(t *testing.T, resp *types.DescribeDomainResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, int32(5), resp.FailoverInfo.CompletedShardCount)
				assert.Equal(t, []int32{10}, resp.FailoverInfo.PendingShards)
			},
		},
		{
			name: "error from validation",
			req: &types.DescribeDomainRequest{
				Name: &domainName,
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDescribeDomainRequest(gomock.Any(), gomock.Any()).Return(errors.New("validation error"))
			},
			expectError:   true,
			expectedError: "validation error",
		},
		{
			name: "error from domain handler",
			req: &types.DescribeDomainRequest{
				Name: &domainName,
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDescribeDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(nil, errors.New("domain handler error"))
			},
			expectError:   true,
			expectedError: "domain handler error",
		},
		{
			name: "error from history client",
			req: &types.DescribeDomainRequest{
				Name: &domainName,
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDescribeDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{
						Name: "domain-name",
						UUID: "domain-id",
					},
					FailoverInfo: &types.FailoverInfo{
						FailoverExpireTimestamp: 1000,
					},
				}, nil)
				deps.mockHistoryClient.EXPECT().GetFailoverInfo(gomock.Any(), &types.GetFailoverInfoRequest{
					DomainID: "domain-id",
				}).Return(nil, errors.New("history client error"))
			},
			expectError: false,
			verifyResp: func(t *testing.T, resp *types.DescribeDomainResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, "domain-name", resp.DomainInfo.Name)
				assert.Zero(t, resp.FailoverInfo.CompletedShardCount)
				assert.Nil(t, resp.FailoverInfo.PendingShards)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			tc.setupMocks(deps)
			resp, err := wh.DescribeDomain(context.Background(), tc.req)

			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				tc.verifyResp(t, resp)
			}
		})
	}
}

func TestDeleteDomain(t *testing.T) {
	domainName := "domain-name"
	testCases := []struct {
		name          string
		req           *types.DeleteDomainRequest
		setupMocks    func(*mockDeps)
		expectError   bool
		expectedError string
	}{
		{
			name: "success_delete_deprecated_domain",
			req: &types.DeleteDomainRequest{
				Name: domainName,
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockDomainCache.EXPECT().GetDomainID(gomock.Any()).Return("testDomainID", nil).AnyTimes()
				deps.mockRequestValidator.EXPECT().ValidateDeleteDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockRequestValidator.EXPECT().ValidateListWorkflowExecutionsRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: &domainName,
				}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{
						Name:   "deprecated-domain",
						Status: types.DomainStatusDeprecated.Ptr(),
					},
				}, nil)
				deps.mockVisibilityMgr.On("ListWorkflowExecutions", mock.Anything, mock.Anything).Return(&persistence.ListWorkflowExecutionsResponse{Executions: []*types.WorkflowExecutionInfo{}}, nil)
				deps.mockDomainHandler.EXPECT().DeleteDomain(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectError: false,
		},
		{
			name: "failure_domain_not_found",
			req: &types.DeleteDomainRequest{
				Name: domainName,
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDeleteDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: &domainName,
				}).Return(nil, errors.New("domain not found"))
			},
			expectError:   true,
			expectedError: "domain not found",
		},
		{
			name: "failure_domain_not_deprecated",
			req: &types.DeleteDomainRequest{
				Name: domainName,
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateDeleteDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: &domainName,
				}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{
						Name:   "active-domain",
						Status: types.DomainStatusRegistered.Ptr(),
					},
				}, nil)
			},
			expectError:   true,
			expectedError: "Domain is not in a deprecated state.",
		},
		{
			name: "failure_domain_has_workflow_history",
			req: &types.DeleteDomainRequest{
				Name: domainName,
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockDomainCache.EXPECT().GetDomainID(gomock.Any()).Return("testDomainID", nil).AnyTimes()
				deps.mockRequestValidator.EXPECT().ValidateDeleteDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockRequestValidator.EXPECT().ValidateListWorkflowExecutionsRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: &domainName,
				}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{
						Name:   "domain-with-history",
						Status: types.DomainStatusDeprecated.Ptr(),
					},
				}, nil)
				deps.mockVisibilityMgr.On("ListWorkflowExecutions", mock.Anything, mock.Anything).Return(&persistence.ListWorkflowExecutionsResponse{
					Executions: []*types.WorkflowExecutionInfo{
						{
							Execution: &types.WorkflowExecution{
								WorkflowID: "test-workflow",
								RunID:      "test-run",
							},
						},
					},
				}, nil)
			},
			expectError:   true,
			expectedError: "Domain still have workflow execution history.",
		},
		{
			name: "failure_delete_domain_error",
			req: &types.DeleteDomainRequest{
				Name: domainName,
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockDomainCache.EXPECT().GetDomainID(gomock.Any()).Return("testDomainID", nil).AnyTimes()
				deps.mockRequestValidator.EXPECT().ValidateDeleteDomainRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockRequestValidator.EXPECT().ValidateListWorkflowExecutionsRequest(gomock.Any(), gomock.Any()).Return(nil)
				deps.mockDomainHandler.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: &domainName,
				}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{
						Name:   "deprecated-domain",
						Status: types.DomainStatusDeprecated.Ptr(),
					},
				}, nil)
				deps.mockVisibilityMgr.On("ListWorkflowExecutions", mock.Anything, mock.Anything).Return(&persistence.ListWorkflowExecutionsResponse{
					Executions: []*types.WorkflowExecutionInfo{},
				}, nil)
				deps.mockDomainHandler.EXPECT().DeleteDomain(gomock.Any(), gomock.Any()).Return(errors.New("delete domain error"))
			},
			expectError:   true,
			expectedError: "delete domain error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			tc.setupMocks(deps)

			err := wh.DeleteDomain(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListFailoverHistory(t *testing.T) {
	domainID := "test-domain-id"
	eventID1 := "event-id-1"
	customPageSize := int32(100)
	nextPageToken := []byte("next-page-token")

	testCases := []struct {
		name          string
		req           *types.ListFailoverHistoryRequest
		setupMocks    func(*mockDeps)
		expectError   bool
		expectedError string
		verifyResp    func(t *testing.T, resp *types.ListFailoverHistoryResponse)
	}{
		{
			name: "success_with_default_page_size",
			req: &types.ListFailoverHistoryRequest{
				Filters: &types.ListFailoverHistoryRequestFilters{
					DomainID: domainID,
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockResource.DomainAuditMgr.EXPECT().GetDomainAuditLogs(
					gomock.Any(),
					&persistence.GetDomainAuditLogsRequest{
						DomainID:      domainID,
						OperationType: persistence.DomainAuditOperationTypeFailover,
						PageSize:      50,
						NextPageToken: nil,
					},
				).Return(&persistence.GetDomainAuditLogsResponse{
					AuditLogs: []*persistence.DomainAuditLog{
						{
							EventID:  eventID1,
							DomainID: domainID,
							StateBefore: &persistence.GetDomainResponse{
								ReplicationConfig: &persistence.DomainReplicationConfig{
									ActiveClusterName: "cluster-us-west",
								},
								FailoverVersion: 1,
							},
							StateAfter: &persistence.GetDomainResponse{
								ReplicationConfig: &persistence.DomainReplicationConfig{
									ActiveClusterName: "cluster-us-east",
								},
								FailoverVersion: 2,
							},
						},
					},
					NextPageToken: nextPageToken,
				}, nil)
			},
			expectError: false,
			verifyResp: func(t *testing.T, resp *types.ListFailoverHistoryResponse) {
				assert.NotNil(t, resp)
				assert.Len(t, resp.FailoverEvents, 1)
				assert.Equal(t, nextPageToken, resp.NextPageToken)
				assert.NotNil(t, resp.FailoverEvents[0].ClusterFailovers)
				assert.Len(t, resp.FailoverEvents[0].ClusterFailovers, 1)
				assert.Equal(t, "cluster-us-west", resp.FailoverEvents[0].ClusterFailovers[0].FromCluster.ActiveClusterName)
				assert.Equal(t, "cluster-us-east", resp.FailoverEvents[0].ClusterFailovers[0].ToCluster.ActiveClusterName)
			},
		},
		{
			name: "success_with_custom_page_size",
			req: &types.ListFailoverHistoryRequest{
				Filters: &types.ListFailoverHistoryRequestFilters{
					DomainID: domainID,
				},
				Pagination: &types.PaginationOptions{
					PageSize: &customPageSize,
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockResource.DomainAuditMgr.EXPECT().GetDomainAuditLogs(
					gomock.Any(),
					&persistence.GetDomainAuditLogsRequest{
						DomainID:      domainID,
						OperationType: persistence.DomainAuditOperationTypeFailover,
						PageSize:      100,
						NextPageToken: nil,
					},
				).Return(&persistence.GetDomainAuditLogsResponse{
					AuditLogs:     []*persistence.DomainAuditLog{},
					NextPageToken: nil,
				}, nil)
			},
			expectError: false,
			verifyResp: func(t *testing.T, resp *types.ListFailoverHistoryResponse) {
				assert.NotNil(t, resp)
				assert.Len(t, resp.FailoverEvents, 0)
				assert.Nil(t, resp.NextPageToken)
			},
		},
		{
			name: "error_nil_filters",
			req: &types.ListFailoverHistoryRequest{
				Filters: nil,
			},
			setupMocks: func(deps *mockDeps) {
				// No mocks needed as validation fails early
			},
			expectError:   true,
			expectedError: "DomainID is required in filters",
		},
		{
			name: "error_get_domain_audit_logs_fails",
			req: &types.ListFailoverHistoryRequest{
				Filters: &types.ListFailoverHistoryRequestFilters{
					DomainID: domainID,
				},
			},
			setupMocks: func(deps *mockDeps) {
				deps.mockResource.DomainAuditMgr.EXPECT().GetDomainAuditLogs(
					gomock.Any(),
					gomock.Any(),
				).Return(nil, errors.New("persistence error"))
			},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			tc.setupMocks(deps)

			resp, err := wh.ListFailoverHistory(context.Background(), tc.req)

			if tc.expectError {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.expectedError)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				tc.verifyResp(t, resp)
			}
		})
	}
}
