// Copyright (c) 2024 Uber Technologies, Inc.
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

package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
)

// newTestHandler creates a new instance of the handler with mocked dependencies for testing.
func newTestHandler(domainManager persistence.DomainManager, primaryCluster bool, domainReplicator Replicator) Handler {
	mockDC := dynamicconfig.NewCollection(dynamicconfig.NewNopClient(), log.NewNoop())
	domainDefaults := &config.ArchivalDomainDefaults{
		History: config.HistoryArchivalDomainDefaults{
			Status: "Disabled",
			URI:    "https://history.example.com",
		},
		Visibility: config.VisibilityArchivalDomainDefaults{
			Status: "Disabled",
			URI:    "https://visibility.example.com",
		},
	}
	archivalMetadata := archiver.NewArchivalMetadata(mockDC, "Enabled", true, "Enabled", true, domainDefaults)
	testConfig := Config{
		MinRetentionDays:       dynamicproperties.GetIntPropertyFn(1),
		MaxRetentionDays:       dynamicproperties.GetIntPropertyFn(5),
		RequiredDomainDataKeys: nil,
		MaxBadBinaryCount:      func(string) int { return 3 },
		FailoverCoolDown:       func(string) time.Duration { return time.Second },
	}

	return NewHandler(
		testConfig,
		log.NewNoop(),
		domainManager,
		cluster.GetTestClusterMetadata(primaryCluster),
		domainReplicator,
		archivalMetadata,
		provider.NewArchiverProvider(nil, nil),
		clock.NewMockedTimeSource(),
	)
}

func TestRegisterDomain(t *testing.T) {
	tests := []struct {
		name             string
		request          *types.RegisterDomainRequest
		isPrimaryCluster bool
		mockSetup        func(*persistence.MockDomainManager, *MockReplicator, *types.RegisterDomainRequest)
		wantErr          bool
		expectedErr      error
	}{
		{
			name:             "success",
			isPrimaryCluster: true,
			request: &types.RegisterDomainRequest{
				Name:                                   "test-domain",
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, replicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
				mockDomainMgr.EXPECT().CreateDomain(gomock.Any(), gomock.Any()).Return(&persistence.CreateDomainResponse{ID: "test-domain-id"}, nil)
			},
			wantErr: false,
		},
		{
			name:             "domain already exists",
			isPrimaryCluster: true,
			request: &types.RegisterDomainRequest{
				Name:                                   "existing-domain",
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, replicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(&persistence.GetDomainResponse{}, nil)
			},
			wantErr: true,
		},
		{
			name:             "global domain registration on non-primary cluster",
			isPrimaryCluster: false,
			request: &types.RegisterDomainRequest{
				Name:           "global-test-domain",
				IsGlobalDomain: true,
			},
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, replicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, req *persistence.GetDomainRequest) (*persistence.GetDomainResponse, error) {
						if req.Name == "global-test-domain" {
							return nil, &types.EntityNotExistsError{}
						}
						return nil, errors.New("unexpected domain name")
					}).AnyTimes()

			},
			wantErr:     true,
			expectedErr: errNotPrimaryCluster,
		},
		{
			name: "unexpected error on domain lookup",
			request: &types.RegisterDomainRequest{
				Name:                                   "test-domain-with-lookup-error",
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, replicator *MockReplicator, request *types.RegisterDomainRequest) {
				unexpectedErr := &types.InternalServiceError{Message: "Internal server error."}
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, unexpectedErr)
			},
			wantErr:     true,
			expectedErr: &types.InternalServiceError{},
		},
		{
			name: "domain name does not match regex",
			request: &types.RegisterDomainRequest{
				Name:                                   "invalid_domain!",
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, replicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
			},
			wantErr:     true,
			expectedErr: errInvalidDomainName,
		},
		{
			name: "specify active cluster name",
			request: &types.RegisterDomainRequest{
				Name:                                   "test-domain-with-active-cluster",
				WorkflowExecutionRetentionPeriodInDays: 3,
				ActiveClusterName:                      "active",
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, replicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})

				mockDomainMgr.EXPECT().CreateDomain(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *persistence.CreateDomainRequest) (*persistence.CreateDomainResponse, error) {
						return &persistence.CreateDomainResponse{ID: "test-domain-id"}, nil
					})
			},
			wantErr: false,
		},
		{
			name: "specify clusters including an invalid one",
			request: &types.RegisterDomainRequest{
				Name: "test-domain-with-clusters",
				Clusters: []*types.ClusterReplicationConfiguration{
					{ClusterName: "valid-cluster-1"},
					{ClusterName: "invalid-cluster"},
				},
				IsGlobalDomain: true,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, replicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
			},
			wantErr:     true,
			expectedErr: &types.BadRequestError{},
		},
		{
			name: "invalid history archival configuration",
			request: &types.RegisterDomainRequest{
				Name:                  "test-domain-invalid-archival-config",
				HistoryArchivalStatus: types.ArchivalStatusEnabled.Ptr(),
				HistoryArchivalURI:    "invalid-uri",
				IsGlobalDomain:        true,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, replicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
			},
			wantErr:     true,
			expectedErr: &url.Error{},
		},
		{
			name: "error during domain creation",
			request: &types.RegisterDomainRequest{
				Name:                                   "domain-creation-error",
				WorkflowExecutionRetentionPeriodInDays: 2,
				IsGlobalDomain:                         false,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, replicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})

				mockDomainMgr.EXPECT().CreateDomain(gomock.Any(), gomock.Any()).Return(nil, errors.New("creation failed"))
			},
			wantErr: true,
		},
		{
			name: "global domain with replication task",
			request: &types.RegisterDomainRequest{
				Name:                                   "global-domain-with-replication",
				IsGlobalDomain:                         true,
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, replicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
				mockDomainMgr.EXPECT().CreateDomain(gomock.Any(), gomock.Any()).Return(&persistence.CreateDomainResponse{ID: "domain-id"}, nil)
				replicator.EXPECT().HandleTransmissionTask(gomock.Any(), types.DomainOperationCreate, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), commonconstants.InitialPreviousFailoverVersion, true).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "global domain replication task failure",
			request: &types.RegisterDomainRequest{
				Name:                                   "global-domain-replication-failure",
				IsGlobalDomain:                         true,
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, mockReplicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
				mockDomainMgr.EXPECT().CreateDomain(gomock.Any(), gomock.Any()).Return(&persistence.CreateDomainResponse{ID: "domain-id"}, nil)
				mockReplicator.EXPECT().HandleTransmissionTask(
					gomock.Any(),
					types.DomainOperationCreate,
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					commonconstants.InitialPreviousFailoverVersion,
					true,
				).Return(errors.New("replication task failed"))
			},
			wantErr:     true,
			expectedErr: errors.New("replication task failed"),
		},
		{
			name: "visibility archival URI triggers parsing/validation error and not able to reach next state",
			request: &types.RegisterDomainRequest{
				Name:                     "domain-with-invalid-visibility-uri",
				VisibilityArchivalStatus: types.ArchivalStatusEnabled.Ptr(),
				VisibilityArchivalURI:    "invalid-visibility-uri",
				IsGlobalDomain:           true,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, mockReplicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
			},
			wantErr:     true,
			expectedErr: &url.Error{},
		},
		{
			name: "local domain with invalid replication configuration: non-global domain",
			request: &types.RegisterDomainRequest{
				Name:              "local-invalid-replication",
				IsGlobalDomain:    false,
				ActiveClusterName: "current-cluster",
				Clusters: []*types.ClusterReplicationConfiguration{
					{ClusterName: "non-current-cluster"},
					{ClusterName: "non-current-cluster2"},
				},
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, mockReplicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
			},
			wantErr:     true,
			expectedErr: &types.BadRequestError{Message: "Invalid local domain active cluster"},
		},
		{
			name: "local domain with invalid replication configuration: global domain",
			request: &types.RegisterDomainRequest{
				Name:              "global-invalid-replication",
				IsGlobalDomain:    true,
				ActiveClusterName: "current-cluster",
				Clusters: []*types.ClusterReplicationConfiguration{
					{ClusterName: "non-current-cluster2"},
					{ClusterName: "non-current-cluster3"},
				},
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, mockReplicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
			},
			wantErr:     true,
			expectedErr: &types.BadRequestError{Message: "Invalid local domain active cluster"},
		},
		{
			name: "active-active domain with an invalid cluster in request",
			request: &types.RegisterDomainRequest{
				Name:           "active-active-domain",
				IsGlobalDomain: true,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								cluster.TestRegion1: {ActiveClusterName: cluster.TestCurrentClusterName},
								cluster.TestRegion2: {ActiveClusterName: "invalid-cluster"},
							},
						},
					},
				},
				Clusters: []*types.ClusterReplicationConfiguration{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, mockReplicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
			},
			wantErr:     true,
			expectedErr: &types.BadRequestError{},
		},
		{
			name: "active-active domain with an active cluster in request that doesn't exist in domain's clusters list",
			request: &types.RegisterDomainRequest{
				Name:           "active-active-domain",
				IsGlobalDomain: true,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								cluster.TestRegion1: {ActiveClusterName: cluster.TestCurrentClusterName},
								cluster.TestRegion2: {ActiveClusterName: cluster.TestAlternativeClusterName},
							},
						},
					},
				},
				Clusters: []*types.ClusterReplicationConfiguration{
					{ClusterName: cluster.TestCurrentClusterName},
				},
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, mockReplicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
			},
			wantErr:     true,
			expectedErr: &types.BadRequestError{},
		},
		{
			name: "active-active domain successfully registered with explicit ActiveClusterName",
			request: &types.RegisterDomainRequest{
				Name:              "active-active-domain",
				IsGlobalDomain:    true,
				ActiveClusterName: cluster.TestCurrentClusterName,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								cluster.TestRegion1: {ActiveClusterName: cluster.TestCurrentClusterName},
								cluster.TestRegion2: {ActiveClusterName: cluster.TestAlternativeClusterName},
							},
						},
					},
				},
				Clusters: []*types.ClusterReplicationConfiguration{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, mockReplicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
				mockDomainMgr.EXPECT().CreateDomain(gomock.Any(), gomock.Any()).Return(&persistence.CreateDomainResponse{ID: "test-domain-id"}, nil)
				mockReplicator.EXPECT().HandleTransmissionTask(gomock.Any(), types.DomainOperationCreate, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), commonconstants.InitialPreviousFailoverVersion, true).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "active-active domain successfully registered without explicit ActiveClusterName (uses current cluster)",
			request: &types.RegisterDomainRequest{
				Name:           "active-active-domain",
				IsGlobalDomain: true,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								cluster.TestRegion1: {ActiveClusterName: cluster.TestCurrentClusterName},
								cluster.TestRegion2: {ActiveClusterName: cluster.TestAlternativeClusterName},
							},
						},
					},
				},
				Clusters: []*types.ClusterReplicationConfiguration{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
				WorkflowExecutionRetentionPeriodInDays: 3,
			},
			isPrimaryCluster: true,
			mockSetup: func(mockDomainMgr *persistence.MockDomainManager, mockReplicator *MockReplicator, request *types.RegisterDomainRequest) {
				mockDomainMgr.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: request.Name}).Return(nil, &types.EntityNotExistsError{})
				mockDomainMgr.EXPECT().CreateDomain(gomock.Any(), gomock.Any()).Return(&persistence.CreateDomainResponse{ID: "test-domain-id"}, nil)
				mockReplicator.EXPECT().HandleTransmissionTask(gomock.Any(), types.DomainOperationCreate, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), commonconstants.InitialPreviousFailoverVersion, true).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDomainMgr := persistence.NewMockDomainManager(controller)
			mockReplicator := NewMockReplicator(controller)

			handler := newTestHandler(mockDomainMgr, tc.isPrimaryCluster, mockReplicator)

			tc.mockSetup(mockDomainMgr, mockReplicator, tc.request)

			err := handler.RegisterDomain(context.Background(), tc.request)

			if tc.wantErr {
				assert.Error(t, err)

				if tc.expectedErr != nil {
					assert.IsType(t, tc.expectedErr, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListDomains(t *testing.T) {

	var configIsolationGroup types.IsolationGroupConfiguration
	tests := []struct {
		name          string
		request       *types.ListDomainsRequest
		setupMocks    func(mockDomainManager *persistence.MockDomainManager)
		expectedResp  *types.ListDomainsResponse
		expectedError error
	}{
		{
			name: "Success case - List domains",
			request: &types.ListDomainsRequest{
				PageSize: 2,
			},
			setupMocks: func(mockDomainManager *persistence.MockDomainManager) {
				resp := &persistence.ListDomainsResponse{
					Domains: []*persistence.GetDomainResponse{
						{
							Info: &persistence.DomainInfo{
								ID:   "domainID1",
								Name: "domainName1",
							},
							Config:            &persistence.DomainConfig{},
							ReplicationConfig: &persistence.DomainReplicationConfig{},
						},
						{
							Info: &persistence.DomainInfo{
								ID:   "domainID2",
								Name: "domainName2",
							},
							Config:            &persistence.DomainConfig{},
							ReplicationConfig: &persistence.DomainReplicationConfig{},
						},
					},
					NextPageToken: nil,
				}
				mockDomainManager.EXPECT().ListDomains(gomock.Any(), &persistence.ListDomainsRequest{
					PageSize:      2,
					NextPageToken: nil,
				}).Return(resp, nil)
			},
			expectedResp: &types.ListDomainsResponse{
				Domains: []*types.DescribeDomainResponse{
					{
						DomainInfo: &types.DomainInfo{
							Name:        "domainName1",
							UUID:        "domainID1",
							Status:      types.DomainStatusRegistered.Ptr(),
							Description: "",
							OwnerEmail:  "",
							Data:        nil,
						},
						Configuration: &types.DomainConfiguration{
							EmitMetric:               false,
							BadBinaries:              &types.BadBinaries{Binaries: nil},
							HistoryArchivalStatus:    types.ArchivalStatusDisabled.Ptr(),
							HistoryArchivalURI:       "",
							VisibilityArchivalStatus: types.ArchivalStatusDisabled.Ptr(),
							VisibilityArchivalURI:    "",
							IsolationGroups:          &configIsolationGroup,
							AsyncWorkflowConfig:      &types.AsyncWorkflowConfiguration{},
						},
						ReplicationConfiguration: &types.DomainReplicationConfiguration{
							ActiveClusterName: "",
							Clusters:          []*types.ClusterReplicationConfiguration{},
						},
					},
					{
						DomainInfo: &types.DomainInfo{
							Name:        "domainName2",
							UUID:        "domainID2",
							Status:      types.DomainStatusRegistered.Ptr(),
							Description: "",
							OwnerEmail:  "",
							Data:        nil,
						},
						Configuration: &types.DomainConfiguration{
							EmitMetric:               false,
							BadBinaries:              &types.BadBinaries{Binaries: nil},
							HistoryArchivalStatus:    types.ArchivalStatusDisabled.Ptr(),
							HistoryArchivalURI:       "",
							VisibilityArchivalStatus: types.ArchivalStatusDisabled.Ptr(),
							VisibilityArchivalURI:    "",
							IsolationGroups:          &configIsolationGroup,
							AsyncWorkflowConfig:      &types.AsyncWorkflowConfiguration{},
						},
						ReplicationConfiguration: &types.DomainReplicationConfiguration{
							ActiveClusterName: "",
							Clusters:          []*types.ClusterReplicationConfiguration{},
						},
					},
				},
				NextPageToken: nil,
			},
			expectedError: nil,
		},
		{
			name: "Error case - Persistence error",
			request: &types.ListDomainsRequest{
				PageSize: 10,
			},
			setupMocks: func(mockDomainManager *persistence.MockDomainManager) {
				mockDomainManager.EXPECT().ListDomains(gomock.Any(), &persistence.ListDomainsRequest{
					PageSize:      10,
					NextPageToken: nil,
				}).Return(nil, errors.New("persistence error"))
			},
			expectedResp:  nil,
			expectedError: errors.New("persistence error"),
		},
	}

	for _, test := range tests {
		controller := gomock.NewController(t)
		mockDomainMgr := persistence.NewMockDomainManager(controller)
		mockReplicator := NewMockReplicator(controller)
		handler := newTestHandler(mockDomainMgr, true, mockReplicator)

		t.Run(test.name, func(t *testing.T) {
			test.setupMocks(mockDomainMgr)

			resp, err := handler.ListDomains(context.Background(), test.request)

			if test.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, test.expectedResp, resp)
			}
		})

	}
}

func TestHandler_DescribeDomain(t *testing.T) {
	controller := gomock.NewController(t)
	mockDomainManager := persistence.NewMockDomainManager(controller)
	mockReplicator := NewMockReplicator(controller)

	handler := newTestHandler(mockDomainManager, true, mockReplicator)

	domainName := "test-domain"
	domainID := "test-domain-id"
	isGlobalDomain := true
	failoverVersion := int64(123)
	lastUpdatedTime := time.Now().UnixNano()
	failoverEndTime := lastUpdatedTime + int64(time.Hour)
	var configIsolationGroup types.IsolationGroupConfiguration

	domainInfo := &persistence.DomainInfo{
		ID:          domainID,
		Name:        domainName,
		Status:      persistence.DomainStatusRegistered,
		Description: "Test Domain",
		OwnerEmail:  "test@uber.com",
		Data:        map[string]string{"k1": "v1"},
	}
	domainConfig := &persistence.DomainConfig{
		Retention: 24,
	}
	domainReplicationConfig := &persistence.DomainReplicationConfig{
		ActiveClusterName: "active",
		Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: "active"}, {ClusterName: "standby"}},
	}

	mockDomainManager.EXPECT().
		GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: domainName}).
		Return(&persistence.GetDomainResponse{
			Info:              domainInfo,
			Config:            domainConfig,
			ReplicationConfig: domainReplicationConfig,
			IsGlobalDomain:    isGlobalDomain,
			FailoverVersion:   failoverVersion,
			FailoverEndTime:   &failoverEndTime,
			LastUpdatedTime:   lastUpdatedTime,
		}, nil)

	describeRequest := &types.DescribeDomainRequest{Name: &domainName}
	expectedResponse := &types.DescribeDomainResponse{
		IsGlobalDomain:  isGlobalDomain,
		FailoverVersion: failoverVersion,
		FailoverInfo: &types.FailoverInfo{
			FailoverVersion:         failoverVersion,
			FailoverStartTimestamp:  lastUpdatedTime,
			FailoverExpireTimestamp: failoverEndTime,
		},
		DomainInfo: &types.DomainInfo{
			Name:        domainName,
			Status:      types.DomainStatusRegistered.Ptr(),
			Description: "Test Domain",
			OwnerEmail:  "test@uber.com",
			Data:        map[string]string{"k1": "v1"},
			UUID:        domainID,
		},
		Configuration: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: 24,
			EmitMetric:                             false,
			BadBinaries:                            &types.BadBinaries{Binaries: nil},
			HistoryArchivalStatus:                  types.ArchivalStatusDisabled.Ptr(),
			HistoryArchivalURI:                     "",
			VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
			VisibilityArchivalURI:                  "",
			IsolationGroups:                        &configIsolationGroup,
			AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{},
		},
		ReplicationConfiguration: &types.DomainReplicationConfiguration{
			ActiveClusterName: "active",
			Clusters: []*types.ClusterReplicationConfiguration{
				{ClusterName: "active"},
				{ClusterName: "standby"},
			},
		},
	}

	resp, err := handler.DescribeDomain(context.Background(), describeRequest)
	assert.NoError(t, err)
	assert.EqualValues(t, expectedResponse, resp)

	mockDomainManager.EXPECT().
		GetDomain(gomock.Any(), gomock.Any()).
		Return(nil, types.EntityNotExistsError{}).
		Times(1)

	_, err = handler.DescribeDomain(context.Background(), describeRequest)

	assert.Error(t, types.EntityNotExistsError{})
}

func TestHandler_DeprecateDomain(t *testing.T) {
	tests := []struct {
		name           string
		domainName     string
		setupMocks     func(m *persistence.MockDomainManager, r *MockReplicator)
		isGlobalDomain bool
		primaryCluster bool
		expectedErr    error
	}{
		{
			name:       "success - deprecate local domain",
			domainName: "local-domain",
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				// Mock calls
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: "local-domain"}).Return(&persistence.GetDomainResponse{
					Info: &persistence.DomainInfo{Name: "local-domain", Status: persistence.DomainStatusRegistered},
					Config: &persistence.DomainConfig{
						Retention: 7,
					},
					ReplicationConfig: &persistence.DomainReplicationConfig{},
					IsGlobalDomain:    false,
				}, nil)
				m.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(nil)
			},
			isGlobalDomain: false,
			primaryCluster: true,
			expectedErr:    nil,
		},
		{
			name:       "failure - domain not found",
			domainName: "non-existent-domain",
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: "non-existent-domain"}).Return(nil, errors.New("domain not found"))
			},
			isGlobalDomain: false,
			primaryCluster: false,
			expectedErr:    errors.New("domain not found"),
		},
		{
			name:       "failure - get metadata error",
			domainName: "test-domain",
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(nil, errors.New("metadata error"))
			},
			expectedErr: errors.New("metadata error"),
		},
		{
			name:           "failure - not primary cluster for global domain",
			domainName:     "global-domain",
			isGlobalDomain: true,
			primaryCluster: false,
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: "global-domain"}).Return(&persistence.GetDomainResponse{
					Info: &persistence.DomainInfo{Name: "global-domain", Status: persistence.DomainStatusRegistered},
					Config: &persistence.DomainConfig{
						Retention: 7,
					},
					ReplicationConfig: &persistence.DomainReplicationConfig{},
					IsGlobalDomain:    true,
				}, nil)
			},
			expectedErr: errNotPrimaryCluster,
		},
		{
			name:           "failure - update domain error",
			domainName:     "test-domain",
			primaryCluster: true,
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), gomock.Any()).Return(&persistence.GetDomainResponse{
					Info: &persistence.DomainInfo{Name: "test-domain", Status: persistence.DomainStatusRegistered},
					Config: &persistence.DomainConfig{
						Retention: 7,
					},
					ReplicationConfig: &persistence.DomainReplicationConfig{},
					FailoverVersion:   123,
					IsGlobalDomain:    true,
					ConfigVersion:     1,
				}, nil)

				m.EXPECT().UpdateDomain(gomock.Any(), gomock.AssignableToTypeOf(&persistence.UpdateDomainRequest{})).DoAndReturn(
					func(_ context.Context, req *persistence.UpdateDomainRequest) error {
						if req.Info.Name != "test-domain" || req.ConfigVersion != 2 || req.Info.Status != persistence.DomainStatusDeprecated {
							return errors.New("unexpected UpdateDomainRequest")
						}
						return errors.New("update domain error")
					},
				)
			},
			expectedErr: errors.New("update domain error"),
		},
		{
			name:           "failure - replicator error for global domain",
			domainName:     "global-domain",
			isGlobalDomain: true,
			primaryCluster: true,
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), gomock.Any()).Return(&persistence.GetDomainResponse{
					Info:           &persistence.DomainInfo{Status: persistence.DomainStatusDeprecated},
					IsGlobalDomain: true,
					ConfigVersion:  1}, nil)
				m.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(nil)
				r.EXPECT().HandleTransmissionTask(gomock.Any(), types.DomainOperationUpdate, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), true).Return(errors.New("replicator error"))
			},
			expectedErr: errors.New("replicator error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDomainManager := persistence.NewMockDomainManager(controller)
			mockReplicator := NewMockReplicator(controller)

			handler := newTestHandler(mockDomainManager, tc.primaryCluster, mockReplicator)
			tc.setupMocks(mockDomainManager, mockReplicator)

			err := handler.DeprecateDomain(context.Background(), &types.DeprecateDomainRequest{Name: tc.domainName})

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandler_UpdateIsolationGroups(t *testing.T) {
	tests := []struct {
		name             string
		domain           string
		isolationGroups  types.IsolationGroupConfiguration
		isPrimaryCluster bool
		setupMocks       func(m *persistence.MockDomainManager, r *MockReplicator)
		expectedErr      error
	}{
		{
			name:   "Successful Update for Local Domain",
			domain: "local-domain",
			isolationGroups: types.IsolationGroupConfiguration{
				"group1": {Name: "group1", State: types.IsolationGroupStateHealthy},
			},
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), gomock.Eq(&persistence.GetDomainRequest{Name: "local-domain"})).Return(&persistence.GetDomainResponse{
					Info: &persistence.DomainInfo{
						Name: "local-domain",
						ID:   "domainID",
					},
					Config: &persistence.DomainConfig{
						Retention:                7,
						EmitMetric:               true,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{"group1": {Name: "group1", State: types.IsolationGroupStateHealthy}},
						HistoryArchivalStatus:    types.ArchivalStatusEnabled,
						VisibilityArchivalStatus: types.ArchivalStatusEnabled,
						HistoryArchivalURI:       "https://test.history",
						VisibilityArchivalURI:    "https://test.visibility",
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: false},
					},
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: "active",
						Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: "active"}, {ClusterName: "standby"}},
					},
					ConfigVersion:   1,
					FailoverVersion: 123,
					IsGlobalDomain:  false,
					LastUpdatedTime: time.Date(2024, 1, 1, 1, 1, 1, 1, time.UTC).UnixNano(),
				}, nil)
				m.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(nil)
			},

			expectedErr: nil,
		},
		{
			name:   "Successful Update for Global Domain",
			domain: "global-domain",
			isolationGroups: types.IsolationGroupConfiguration{
				"group1": {State: types.IsolationGroupStateDrained},
			},
			isPrimaryCluster: true,
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), gomock.Any()).Return(&persistence.GetDomainResponse{
					Info: &persistence.DomainInfo{Name: "global-domain"},
					Config: &persistence.DomainConfig{
						Retention: 30,
					},
					ReplicationConfig: &persistence.DomainReplicationConfig{},
					ConfigVersion:     int64(1),
					IsGlobalDomain:    true,
					LastUpdatedTime:   time.Date(2024, 1, 1, 1, 1, 1, 1, time.UTC).UnixNano(),
				}, nil)
				m.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(nil)
				r.EXPECT().HandleTransmissionTask(gomock.Any(), types.DomainOperationUpdate, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), true).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:   "Failure Due to Invalid Request (nil Isolation Groups)",
			domain: "domain-with-nil-isolation-groups",
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
			},
			expectedErr: fmt.Errorf("invalid request, isolationGroup configuration must be set: Got: {domain-with-nil-isolation-groups map[]}"),
		},
		{
			name:   "Failure Due to Domain Not Found",
			domain: "nonexistent-domain",
			isolationGroups: types.IsolationGroupConfiguration{
				"group1": {State: types.IsolationGroupStateHealthy},
			},
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), gomock.Any()).Return(nil, errors.New("domain not found"))
			},
			expectedErr: errors.New("domain not found"),
		},
		{
			name:   "Failure Due to UpdateDomain Error",
			domain: "domain-with-update-error",
			isolationGroups: types.IsolationGroupConfiguration{
				"group1": {State: types.IsolationGroupStateHealthy},
			},
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), gomock.Any()).Return(&persistence.GetDomainResponse{
					Info:              &persistence.DomainInfo{Name: "domain-with-update-error"},
					Config:            &persistence.DomainConfig{},
					ReplicationConfig: &persistence.DomainReplicationConfig{},
					IsGlobalDomain:    false,
				}, nil)
				m.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(errors.New("update error"))
			},
			expectedErr: errors.New("update error"),
		},
		{
			name:   "Failure Due to Replication Error (for Global Domains)",
			domain: "global-domain-with-replication-error",
			isolationGroups: types.IsolationGroupConfiguration{
				"group1": {State: types.IsolationGroupStateHealthy},
			},
			isPrimaryCluster: true,
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), gomock.Any()).Return(&persistence.GetDomainResponse{
					Info:              &persistence.DomainInfo{Name: "global-domain-with-replication-error"},
					Config:            &persistence.DomainConfig{},
					ReplicationConfig: &persistence.DomainReplicationConfig{},
					IsGlobalDomain:    true,
				}, nil)
				m.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(nil)
				r.EXPECT().HandleTransmissionTask(gomock.Any(), types.DomainOperationUpdate, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), true).Return(errors.New("replication error"))
			},
			expectedErr: errors.New("replication error"),
		},
		{
			name:   "Failure due to GetMetadata error",
			domain: "test-domain",
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(nil, errors.New("metadata error"))
			},
			expectedErr: errors.New("metadata error"),
		},
		{
			name:            "Failure due to nil domain config",
			domain:          "test-domain",
			isolationGroups: types.IsolationGroupConfiguration{},
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), gomock.Any()).Return(&persistence.GetDomainResponse{
					Info:              &persistence.DomainInfo{Name: "test-domain", ID: "domainID"},
					Config:            nil,
					ReplicationConfig: &persistence.DomainReplicationConfig{},
					IsGlobalDomain:    false,
				}, nil)
			},
			expectedErr: fmt.Errorf("unable to load config for domain as expected"),
		},
		{
			name:   "Failure due to failover cool-down",
			domain: "test-domain",
			isolationGroups: types.IsolationGroupConfiguration{
				"group1": {Name: "group1", State: types.IsolationGroupStateHealthy},
			},
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				lastUpdatedWithinCoolDownPeriod := time.Now().UnixNano() - (500 * int64(time.Millisecond))
				m.EXPECT().GetDomain(gomock.Any(), gomock.Eq(&persistence.GetDomainRequest{Name: "test-domain"})).Return(&persistence.GetDomainResponse{
					Info: &persistence.DomainInfo{Name: "test-domain", ID: "domainID"},
					Config: &persistence.DomainConfig{
						Retention:                7,
						EmitMetric:               true,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						HistoryArchivalStatus:    types.ArchivalStatusEnabled,
						VisibilityArchivalStatus: types.ArchivalStatusEnabled,
						HistoryArchivalURI:       "https://test.history",
						VisibilityArchivalURI:    "https://test.visibility",
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: false},
					},
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: "active",
						Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: "active"}, {ClusterName: "standby"}},
					},
					ConfigVersion:   1,
					FailoverVersion: 123,
					IsGlobalDomain:  false,
					LastUpdatedTime: lastUpdatedWithinCoolDownPeriod,
				}, nil)
			},
			expectedErr: errDomainUpdateTooFrequent,
		},
		{
			name:   "Global domain update from non-primary cluster",
			domain: "global-domain",
			isolationGroups: types.IsolationGroupConfiguration{
				"group1": {Name: "group1", State: types.IsolationGroupStateHealthy},
			},
			setupMocks: func(m *persistence.MockDomainManager, r *MockReplicator) {
				m.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				m.EXPECT().GetDomain(gomock.Any(), gomock.Eq(&persistence.GetDomainRequest{Name: "global-domain"})).Return(&persistence.GetDomainResponse{
					Info: &persistence.DomainInfo{Name: "global-domain", ID: "domainID"},
					Config: &persistence.DomainConfig{
						Retention: 7,
					},
					ReplicationConfig: &persistence.DomainReplicationConfig{},
					IsGlobalDomain:    true,
				}, nil)
			},
			expectedErr: errNotPrimaryCluster,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDomainManager := persistence.NewMockDomainManager(controller)
			mockReplicator := NewMockReplicator(controller)

			handler := newTestHandler(mockDomainManager, tc.isPrimaryCluster, mockReplicator)
			tc.setupMocks(mockDomainManager, mockReplicator)

			err := handler.UpdateIsolationGroups(context.Background(), types.UpdateDomainIsolationGroupsRequest{
				Domain:          tc.domain,
				IsolationGroups: tc.isolationGroups,
			})

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandler_UpdateAsyncWorkflowConfiguraton(t *testing.T) {
	testDomain := "testDomain"

	createDomainConfig := func(asyncEnabled bool) *persistence.DomainConfig {
		return &persistence.DomainConfig{
			Retention:                10,
			EmitMetric:               true,
			HistoryArchivalStatus:    types.ArchivalStatusEnabled,
			HistoryArchivalURI:       "https://history.example.com",
			VisibilityArchivalStatus: types.ArchivalStatusEnabled,
			VisibilityArchivalURI:    "https://visibility.example.com",
			BadBinaries:              types.BadBinaries{},
			IsolationGroups:          types.IsolationGroupConfiguration{},
			AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: asyncEnabled},
		}
	}

	tests := []struct {
		name             string
		setupMocks       func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator)
		isPrimaryCluster bool
		request          *types.UpdateDomainAsyncWorkflowConfiguratonRequest
		expectedErr      error
	}{
		{
			name: "success",
			setupMocks: func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator) {
				mockDomainManager.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: testDomain}).Return(&persistence.GetDomainResponse{
					Info:            &persistence.DomainInfo{ID: "domainID", Name: testDomain},
					Config:          createDomainConfig(false),
					ConfigVersion:   1,
					FailoverVersion: 1,
					LastUpdatedTime: time.Now().Add(-10 * time.Minute).UnixNano(),
				}, nil)
				mockDomainManager.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req *persistence.UpdateDomainRequest) error {
						expectedReq := &persistence.UpdateDomainRequest{
							Info:                        &persistence.DomainInfo{ID: "domainID", Name: testDomain},
							Config:                      createDomainConfig(true),
							ConfigVersion:               2,
							FailoverVersion:             1,
							FailoverNotificationVersion: 0,
							LastUpdatedTime:             req.LastUpdatedTime,
							NotificationVersion:         1,
						}
						if !reflect.DeepEqual(req, expectedReq) {
							return fmt.Errorf("unexpected UpdateDomainRequest: got %+v, want %+v", req, expectedReq)
						}
						return nil
					},
				)
			},
			request: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: testDomain,
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled: true,
				},
			},
			expectedErr: nil,
		},
		{
			name: "fail to get metadata",
			setupMocks: func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator) {
				mockDomainManager.EXPECT().GetMetadata(gomock.Any()).Return(nil, fmt.Errorf("metadata error"))
			},
			request: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: testDomain,
			},
			expectedErr: fmt.Errorf("metadata error"),
		},
		{
			name: "fail to get domain",
			setupMocks: func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator) {
				mockDomainManager.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("get domain error"))
			},
			request: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: testDomain,
			},
			expectedErr: fmt.Errorf("get domain error"),
		},
		{
			name: "fail due to nil domain config",
			setupMocks: func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator) {
				mockDomainManager.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: testDomain}).Return(&persistence.GetDomainResponse{
					Info:   &persistence.DomainInfo{ID: "domainID", Name: testDomain},
					Config: nil,
				}, nil)
			},
			request: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: testDomain,
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled: true,
				},
			},
			expectedErr: fmt.Errorf("unable to load config for domain as expected"),
		},
		{
			name: "fail due to update too frequent",
			setupMocks: func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator) {
				mockDomainManager.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				lastUpdatedWithinCoolDownPeriod := time.Now().UnixNano() - (500 * int64(time.Millisecond))
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), gomock.Eq(&persistence.GetDomainRequest{Name: "test-domain"})).Return(&persistence.GetDomainResponse{
					Info:   &persistence.DomainInfo{Name: "test-domain", ID: "domainID"},
					Config: createDomainConfig(false),
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: "active",
						Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: "active"}, {ClusterName: "standby"}},
					},
					ConfigVersion:   1,
					FailoverVersion: 123,
					IsGlobalDomain:  false,
					LastUpdatedTime: lastUpdatedWithinCoolDownPeriod,
				}, nil)
			},
			request: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled: true,
				},
			},
			expectedErr: errDomainUpdateTooFrequent,
		},
		{
			name:             "fail due to non-primary cluster update for global domain",
			isPrimaryCluster: false,
			setupMocks: func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator) {
				mockDomainManager.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), gomock.Eq(&persistence.GetDomainRequest{Name: "global-test-domain"})).Return(&persistence.GetDomainResponse{
					Info:   &persistence.DomainInfo{Name: "global-test-domain", ID: "domainID"},
					Config: createDomainConfig(false),
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: "active",
						Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: "active"}, {ClusterName: "standby"}},
					},
					ConfigVersion:   1,
					FailoverVersion: 123,
					IsGlobalDomain:  true,
					LastUpdatedTime: time.Now().Add(-24 * time.Hour).UnixNano(),
				}, nil)

			},
			request: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "global-test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled: true,
				},
			},
			expectedErr: errNotPrimaryCluster,
		},
		{
			name: "configuration delete request",
			setupMocks: func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator) {
				mockDomainManager.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), gomock.Eq(&persistence.GetDomainRequest{Name: "test-domain"})).Return(&persistence.GetDomainResponse{
					Info:   &persistence.DomainInfo{Name: "test-domain", ID: "domainID"},
					Config: createDomainConfig(true),
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: "active",
						Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: "active"}, {ClusterName: "standby"}},
					},
					ConfigVersion:   1,
					FailoverVersion: 123,
					IsGlobalDomain:  false,
					LastUpdatedTime: time.Now().Add(-24 * time.Hour).UnixNano(),
				}, nil)
				mockDomainManager.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req *persistence.UpdateDomainRequest) error {
						if req.Config.AsyncWorkflowConfig.Enabled != false {
							return fmt.Errorf("unexpected UpdateDomainRequest for delete configuration")
						}
						return nil
					},
				)
			},
			request: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain:        "test-domain",
				Configuration: nil,
			},
			expectedErr: nil,
		},
		{
			name: "fail on domain manager UpdateDomain call",
			setupMocks: func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator) {
				mockDomainManager.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), gomock.Eq(&persistence.GetDomainRequest{Name: "test-domain"})).Return(&persistence.GetDomainResponse{
					Info:   &persistence.DomainInfo{Name: "test-domain", ID: "domainID"},
					Config: createDomainConfig(false),
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: "active",
						Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: "active"}, {ClusterName: "standby"}},
					},
					ConfigVersion:   1,
					FailoverVersion: 123,
					IsGlobalDomain:  false,
					LastUpdatedTime: time.Now().Add(-24 * time.Hour).UnixNano(),
				}, nil)
				mockDomainManager.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(fmt.Errorf("update domain error"))
			},
			request: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled: true,
				},
			},
			expectedErr: fmt.Errorf("update domain error"),
		},
		{
			name:             "global domain update triggers replication",
			isPrimaryCluster: true,
			setupMocks: func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator) {
				mockDomainManager.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), gomock.Eq(&persistence.GetDomainRequest{Name: "global-test-domain"})).Return(&persistence.GetDomainResponse{
					Info:   &persistence.DomainInfo{Name: "global-test-domain", ID: "domainID"},
					Config: createDomainConfig(false),
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: "active",
						Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: "active"}, {ClusterName: "standby"}},
					},
					ConfigVersion:   1,
					FailoverVersion: 123,
					IsGlobalDomain:  true,
					LastUpdatedTime: time.Now().Add(-24 * time.Hour).UnixNano(),
				}, nil)
				mockDomainManager.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req *persistence.UpdateDomainRequest) error {
						if req.Config.AsyncWorkflowConfig.Enabled != true {
							return fmt.Errorf("unexpected UpdateDomainRequest for global domain update")
						}
						return nil
					},
				)
				mockReplicator.EXPECT().HandleTransmissionTask(
					gomock.Any(),
					types.DomainOperationUpdate,
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					true,
				).Return(nil)
			},
			request: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "global-test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled: true,
				},
			},
			expectedErr: nil,
		},
		{
			name:             "global domain update replication error",
			isPrimaryCluster: true,
			setupMocks: func(mockDomainManager *persistence.MockDomainManager, mockReplicator *MockReplicator) {
				mockDomainManager.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil)
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), gomock.Eq(&persistence.GetDomainRequest{Name: "global-test-domain"})).Return(&persistence.GetDomainResponse{
					Info:   &persistence.DomainInfo{Name: "global-test-domain", ID: "domainID"},
					Config: createDomainConfig(false),
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: "active",
						Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: "active"}, {ClusterName: "standby"}},
					},
					ConfigVersion:   1,
					FailoverVersion: 123,
					IsGlobalDomain:  true,
					LastUpdatedTime: time.Now().Add(-24 * time.Hour).UnixNano(),
				}, nil)
				mockDomainManager.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req *persistence.UpdateDomainRequest) error {
						// Validate the UpdateDomainRequest for global domain update
						if req.Config.AsyncWorkflowConfig.Enabled != true {
							return fmt.Errorf("unexpected UpdateDomainRequest for global domain update")
						}
						return nil
					},
				)
				mockReplicator.EXPECT().HandleTransmissionTask(
					gomock.Any(),
					types.DomainOperationUpdate,
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					true,
				).Return(fmt.Errorf("replication error"))
			},
			request: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "global-test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled: true,
				},
			},
			expectedErr: fmt.Errorf("replication error"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockDomainManager := persistence.NewMockDomainManager(ctrl)
			mockReplicator := NewMockReplicator(ctrl)
			handler := newTestHandler(mockDomainManager, test.isPrimaryCluster, mockReplicator)
			test.setupMocks(mockDomainManager, mockReplicator)

			err := handler.UpdateAsyncWorkflowConfiguraton(context.Background(), *test.request)
			if test.expectedErr != nil {
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandler_UpdateDomain(t *testing.T) {
	ctx := context.Background()
	maxLength := 1

	testCases := []struct {
		name      string
		setupMock func(
			domainManager *persistence.MockDomainManager,
			updateRequest *types.UpdateDomainRequest,
			archivalMetadata *archiver.MockArchivalMetadata,
			timeSource clock.MockedTimeSource,
			domainReplicator *MockReplicator,
		)
		request  *types.UpdateDomainRequest
		response func(timeSource clock.MockedTimeSource) *types.UpdateDomainResponse
		err      error
	}{
		{
			name: "Success case - global domain force failover",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, domainReplicator *MockReplicator) {
				domainResponse := &persistence.GetDomainResponse{
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					Info: &persistence.DomainInfo{
						Name:   constants.TestDomainName,
						ID:     constants.TestDomainID,
						Status: persistence.DomainStatusRegistered,
					},
					IsGlobalDomain:  true,
					LastUpdatedTime: timeSource.Now().UnixNano(),
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion,
				}
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(domainResponse, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
				timeSource.Advance(time.Hour)

				failoverData, _ := json.Marshal([]FailoverEvent{{
					EventTime:          timeSource.Now(),
					FromCluster:        cluster.TestCurrentClusterName,
					ToCluster:          cluster.TestAlternativeClusterName,
					FailoverType:       commonconstants.FailoverType(commonconstants.FailoverTypeForce).String(),
					FromActiveClusters: types.ActiveClusters{},
					ToActiveClusters:   types.ActiveClusters{},
				}})

				expectedUpdateRequest := &persistence.UpdateDomainRequest{
					Info: &persistence.DomainInfo{
						Name:        constants.TestDomainName,
						ID:          constants.TestDomainID,
						Status:      persistence.DomainStatusRegistered,
						Description: domainResponse.Info.Description,
						OwnerEmail:  domainResponse.Info.OwnerEmail,
						Data: map[string]string{
							"FailoverHistory": string(failoverData),
						},
					},
					Config: domainResponse.Config,
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestAlternativeClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName},
						},
					},
					PreviousFailoverVersion: commonconstants.InitialPreviousFailoverVersion,
					ConfigVersion:           domainResponse.ConfigVersion,
					FailoverVersion:         cluster.TestAlternativeClusterInitialFailoverVersion,
					LastUpdatedTime:         timeSource.Now().UnixNano(),
				}

				domainManager.EXPECT().UpdateDomain(ctx, expectedUpdateRequest).Return(nil).Times(1)
				domainReplicator.EXPECT().
					HandleTransmissionTask(
						ctx,
						types.DomainOperationUpdate,
						expectedUpdateRequest.Info,
						domainResponse.Config,
						expectedUpdateRequest.ReplicationConfig,
						domainResponse.ConfigVersion,
						cluster.TestAlternativeClusterInitialFailoverVersion,
						commonconstants.InitialPreviousFailoverVersion,
						domainResponse.IsGlobalDomain,
					).Return(nil).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name:              constants.TestDomainName,
				ActiveClusterName: common.Ptr(cluster.TestAlternativeClusterName),
			},
			response: func(timeSource clock.MockedTimeSource) *types.UpdateDomainResponse {
				data, _ := json.Marshal([]FailoverEvent{{EventTime: timeSource.Now(), FromCluster: cluster.TestCurrentClusterName, ToCluster: cluster.TestAlternativeClusterName, FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeForce).String()}})
				return &types.UpdateDomainResponse{
					IsGlobalDomain:  true,
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion/cluster.TestFailoverVersionIncrement*cluster.TestFailoverVersionIncrement + cluster.TestAlternativeClusterInitialFailoverVersion,
					DomainInfo: &types.DomainInfo{
						Name:   constants.TestDomainName,
						UUID:   constants.TestDomainID,
						Data:   map[string]string{commonconstants.DomainDataKeyForFailoverHistory: string(data)},
						Status: common.Ptr(types.DomainStatusRegistered),
					},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 1,
						EmitMetric:                             true,
						HistoryArchivalStatus:                  common.Ptr(types.ArchivalStatusDisabled),
						VisibilityArchivalStatus:               common.Ptr(types.ArchivalStatusDisabled),
						BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:                        &types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
					},
					ReplicationConfiguration: &types.DomainReplicationConfiguration{
						ActiveClusterName: cluster.TestAlternativeClusterName,
						Clusters: []*types.ClusterReplicationConfiguration{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName},
						},
					},
				}
			},
		},
		{
			name: "Success case - global domain grace failover",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, domainReplicator *MockReplicator) {
				domainResponse := &persistence.GetDomainResponse{
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestAlternativeClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					Info: &persistence.DomainInfo{
						Name:   constants.TestDomainName,
						ID:     constants.TestDomainID,
						Status: persistence.DomainStatusRegistered,
					},
					IsGlobalDomain:  true,
					LastUpdatedTime: timeSource.Now().UnixNano(),
					FailoverVersion: 1,
				}
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(domainResponse, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
				timeSource.Advance(time.Hour)

				data, _ := json.Marshal([]FailoverEvent{{EventTime: timeSource.Now(), FromCluster: cluster.TestAlternativeClusterName, ToCluster: cluster.TestCurrentClusterName, FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeGrace).String()}})

				expectedUpdateRequest := &persistence.UpdateDomainRequest{
					Info: &persistence.DomainInfo{
						Name:        constants.TestDomainName,
						ID:          constants.TestDomainID,
						Status:      persistence.DomainStatusRegistered,
						Description: domainResponse.Info.Description,
						OwnerEmail:  domainResponse.Info.OwnerEmail,
						Data: map[string]string{
							commonconstants.DomainDataKeyForFailoverHistory: string(data),
						},
					},
					Config: domainResponse.Config,
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName},
						},
					},
					PreviousFailoverVersion: 1,
					ConfigVersion:           domainResponse.ConfigVersion,
					FailoverVersion:         10,
					FailoverEndTime:         common.Ptr(timeSource.Now().Add(time.Duration(10) * time.Second).UnixNano()),
					LastUpdatedTime:         timeSource.Now().UnixNano(),
				}

				domainManager.EXPECT().UpdateDomain(ctx, expectedUpdateRequest).Return(nil).Times(1)
				domainReplicator.EXPECT().
					HandleTransmissionTask(
						ctx,
						types.DomainOperationUpdate,
						expectedUpdateRequest.Info,
						domainResponse.Config,
						expectedUpdateRequest.ReplicationConfig,
						domainResponse.ConfigVersion,
						int64(10),
						int64(1),
						domainResponse.IsGlobalDomain,
					).Return(nil).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name:                     constants.TestDomainName,
				ActiveClusterName:        common.Ptr(cluster.TestCurrentClusterName),
				FailoverTimeoutInSeconds: common.Int32Ptr(10),
			},
			response: func(timeSource clock.MockedTimeSource) *types.UpdateDomainResponse {
				data, _ := json.Marshal([]FailoverEvent{{EventTime: timeSource.Now(), FromCluster: cluster.TestAlternativeClusterName, ToCluster: cluster.TestCurrentClusterName, FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeGrace).String()}})
				return &types.UpdateDomainResponse{
					IsGlobalDomain:  true,
					FailoverVersion: cluster.TestFailoverVersionIncrement,
					DomainInfo: &types.DomainInfo{
						Name:   constants.TestDomainName,
						UUID:   constants.TestDomainID,
						Data:   map[string]string{commonconstants.DomainDataKeyForFailoverHistory: string(data)},
						Status: common.Ptr(types.DomainStatusRegistered),
					},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 1,
						EmitMetric:                             true,
						HistoryArchivalStatus:                  common.Ptr(types.ArchivalStatusDisabled),
						VisibilityArchivalStatus:               common.Ptr(types.ArchivalStatusDisabled),
						BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:                        &types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
					},
					ReplicationConfiguration: &types.DomainReplicationConfiguration{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*types.ClusterReplicationConfiguration{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName},
						},
					},
				}
			},
		},
		{
			name: "Success case - global domain config change",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, domainReplicator *MockReplicator) {
				domainResponse := &persistence.GetDomainResponse{
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					Info: &persistence.DomainInfo{
						Name:   constants.TestDomainName,
						ID:     constants.TestDomainID,
						Status: persistence.DomainStatusRegistered,
					},
					IsGlobalDomain:  true,
					LastUpdatedTime: timeSource.Now().UnixNano(),
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion,
				}
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(domainResponse, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
				timeSource.Advance(time.Hour)

				expectedUpdateRequest := &persistence.UpdateDomainRequest{
					Info: &persistence.DomainInfo{
						Name:        constants.TestDomainName,
						ID:          constants.TestDomainID,
						Status:      persistence.DomainStatusRegistered,
						Description: domainResponse.Info.Description,
						OwnerEmail:  domainResponse.Info.OwnerEmail,
						Data:        nil,
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               false,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					ReplicationConfig:       domainResponse.ReplicationConfig,
					PreviousFailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion,
					ConfigVersion:           domainResponse.ConfigVersion + 1,
					FailoverVersion:         cluster.TestCurrentClusterInitialFailoverVersion,
					LastUpdatedTime:         timeSource.Now().UnixNano(),
				}

				domainManager.EXPECT().UpdateDomain(ctx, expectedUpdateRequest).Return(nil).Times(1)
				domainReplicator.EXPECT().
					HandleTransmissionTask(
						ctx,
						types.DomainOperationUpdate,
						expectedUpdateRequest.Info,
						expectedUpdateRequest.Config,
						domainResponse.ReplicationConfig,
						domainResponse.ConfigVersion+1,
						cluster.TestCurrentClusterInitialFailoverVersion,
						cluster.TestCurrentClusterInitialFailoverVersion,
						domainResponse.IsGlobalDomain,
					).Return(nil).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name:       constants.TestDomainName,
				EmitMetric: common.Ptr(false),
			},
			response: func(_ clock.MockedTimeSource) *types.UpdateDomainResponse {
				return &types.UpdateDomainResponse{
					IsGlobalDomain:  true,
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion,
					DomainInfo: &types.DomainInfo{
						Name:   constants.TestDomainName,
						UUID:   constants.TestDomainID,
						Status: common.Ptr(types.DomainStatusRegistered),
					},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 1,
						EmitMetric:                             false,
						HistoryArchivalStatus:                  common.Ptr(types.ArchivalStatusDisabled),
						VisibilityArchivalStatus:               common.Ptr(types.ArchivalStatusDisabled),
						BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:                        &types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
					},
					ReplicationConfiguration: &types.DomainReplicationConfiguration{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*types.ClusterReplicationConfiguration{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName},
						},
					},
				}
			},
		},
		{
			name: "Success case - active-active domain active clusters change",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, domainReplicator *MockReplicator) {
				domainResponse := &persistence.GetDomainResponse{
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName},
							{ClusterName: cluster.TestAlternativeClusterName},
						},
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										cluster.TestRegion1: {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
										},
										cluster.TestRegion2: {
											ActiveClusterName: cluster.TestAlternativeClusterName,
											FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
										},
									},
								},
							},
						},
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					Info: &persistence.DomainInfo{
						Name:   constants.TestDomainName,
						ID:     constants.TestDomainID,
						Status: persistence.DomainStatusRegistered,
					},
					IsGlobalDomain:  true,
					LastUpdatedTime: timeSource.Now().UnixNano(),
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion,
				}
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(domainResponse, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
				timeSource.Advance(time.Hour)

				data, _ := json.Marshal([]FailoverEvent{{
					EventTime:    timeSource.Now(),
					FromCluster:  cluster.TestCurrentClusterName,
					FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeForce).String(),
					FromActiveClusters: types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									cluster.TestRegion1: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
									},
									cluster.TestRegion2: {
										ActiveClusterName: cluster.TestAlternativeClusterName,
										FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
									},
								},
							},
						},
					},
					ToActiveClusters: types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									cluster.TestRegion1: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
									},
									cluster.TestRegion2: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion + cluster.TestFailoverVersionIncrement,
									},
								},
							},
						},
					},
				}})

				expectedUpdateRequest := &persistence.UpdateDomainRequest{
					Info: &persistence.DomainInfo{
						Name:        constants.TestDomainName,
						ID:          constants.TestDomainID,
						Status:      persistence.DomainStatusRegistered,
						Description: domainResponse.Info.Description,
						OwnerEmail:  domainResponse.Info.OwnerEmail,
						Data: map[string]string{
							commonconstants.DomainDataKeyForFailoverHistory: string(data),
						},
					},
					Config: domainResponse.Config,
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName},
							{ClusterName: cluster.TestAlternativeClusterName},
						},
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										cluster.TestRegion1: {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
										},
										cluster.TestRegion2: {
											ActiveClusterName: cluster.TestCurrentClusterName,
											// previously it was alternative cluster with failover version 1.
											// failover version should be the next failover version of the new cluster
											FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion + cluster.TestFailoverVersionIncrement,
										},
									},
								},
							},
						},
					},
					PreviousFailoverVersion: -1, // this is not applicable to active-active domain
					ConfigVersion:           domainResponse.ConfigVersion + 1,
					FailoverVersion:         cluster.TestCurrentClusterInitialFailoverVersion + cluster.TestFailoverVersionIncrement, // this is incremented to indicate there was a change in replication config
					LastUpdatedTime:         timeSource.Now().UnixNano(),
				}

				domainManager.EXPECT().UpdateDomain(ctx, expectedUpdateRequest).Return(nil).Times(1)
				domainReplicator.EXPECT().
					HandleTransmissionTask(
						ctx,
						types.DomainOperationUpdate,
						expectedUpdateRequest.Info,
						domainResponse.Config,
						expectedUpdateRequest.ReplicationConfig,
						domainResponse.ConfigVersion+1,
						cluster.TestCurrentClusterInitialFailoverVersion+cluster.TestFailoverVersionIncrement,
						int64(-1), // previous failover version is not applicable to active-active domain
						domainResponse.IsGlobalDomain,
					).Return(nil).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name: constants.TestDomainName,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								cluster.TestRegion1: {
									ActiveClusterName: cluster.TestCurrentClusterName,
									FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
								},
								cluster.TestRegion2: { // failover region2 to region1
									ActiveClusterName: cluster.TestCurrentClusterName,
									FailoverVersion:   123123123123, // this number will be ignored and replaced with appropriate failover version
								},
							},
						},
					},
				},
			},
			response: func(timeSource clock.MockedTimeSource) *types.UpdateDomainResponse {
				data, _ := json.Marshal([]FailoverEvent{{
					EventTime:    timeSource.Now(),
					FromCluster:  cluster.TestCurrentClusterName,
					FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeForce).String(),
					FromActiveClusters: types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									cluster.TestRegion1: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
									},
									cluster.TestRegion2: {
										ActiveClusterName: cluster.TestAlternativeClusterName,
										FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
									},
								},
							},
						},
					},
					ToActiveClusters: types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									cluster.TestRegion1: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
									},
									cluster.TestRegion2: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion + cluster.TestFailoverVersionIncrement,
									},
								},
							},
						},
					},
				}})
				return &types.UpdateDomainResponse{
					IsGlobalDomain:  true,
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion + cluster.TestFailoverVersionIncrement,
					DomainInfo: &types.DomainInfo{
						Name:   constants.TestDomainName,
						UUID:   constants.TestDomainID,
						Status: common.Ptr(types.DomainStatusRegistered),
						Data:   map[string]string{commonconstants.DomainDataKeyForFailoverHistory: string(data)},
					},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 1,
						EmitMetric:                             true,
						HistoryArchivalStatus:                  common.Ptr(types.ArchivalStatusDisabled),
						VisibilityArchivalStatus:               common.Ptr(types.ArchivalStatusDisabled),
						BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:                        &types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
					},
					ReplicationConfiguration: &types.DomainReplicationConfiguration{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*types.ClusterReplicationConfiguration{
							{ClusterName: cluster.TestCurrentClusterName},
							{ClusterName: cluster.TestAlternativeClusterName},
						},
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										cluster.TestRegion1: {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
										},
										cluster.TestRegion2: {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion + cluster.TestFailoverVersionIncrement,
										},
									},
								},
							},
						},
					},
				}
			},
		},
		{
			name: "Success case - active-passive to active-active migration",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, domainReplicator *MockReplicator) {
				domainResponse := &persistence.GetDomainResponse{
					ReplicationConfig: &persistence.DomainReplicationConfig{
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName},
							{ClusterName: cluster.TestAlternativeClusterName},
						},
						ActiveClusterName: cluster.TestCurrentClusterName,
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					Info: &persistence.DomainInfo{
						Name:   constants.TestDomainName,
						ID:     constants.TestDomainID,
						Status: persistence.DomainStatusRegistered,
					},
					IsGlobalDomain:  true,
					LastUpdatedTime: timeSource.Now().UnixNano(),
					// once migrated to active clusters, this version should be carried over to new active clusters config
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion + 2*cluster.TestFailoverVersionIncrement,
				}
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(domainResponse, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
				timeSource.Advance(time.Hour)

				data, _ := json.Marshal([]FailoverEvent{{
					EventTime:    timeSource.Now(),
					FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeForce).String(),
					FromCluster:  cluster.TestCurrentClusterName,
					ToActiveClusters: types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									cluster.TestRegion1: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
									},
									cluster.TestRegion2: {
										ActiveClusterName: cluster.TestAlternativeClusterName,
										FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
									},
								},
							},
						},
					},
				}})

				expectedUpdateRequest := &persistence.UpdateDomainRequest{
					Info: &persistence.DomainInfo{
						Name:        constants.TestDomainName,
						ID:          constants.TestDomainID,
						Status:      persistence.DomainStatusRegistered,
						Description: domainResponse.Info.Description,
						OwnerEmail:  domainResponse.Info.OwnerEmail,
						Data: map[string]string{
							commonconstants.DomainDataKeyForFailoverHistory: string(data),
						},
					},
					Config: domainResponse.Config,
					ReplicationConfig: &persistence.DomainReplicationConfig{
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName},
							{ClusterName: cluster.TestAlternativeClusterName},
						},
						ActiveClusterName: cluster.TestCurrentClusterName, // should be left as is
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										cluster.TestRegion1: {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
										},
										cluster.TestRegion2: {
											ActiveClusterName: cluster.TestAlternativeClusterName,
											FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
										},
									},
								},
							},
						},
					},
					PreviousFailoverVersion: -1, // this is not applicable to active-active domain
					ConfigVersion:           domainResponse.ConfigVersion + 1,
					FailoverVersion:         cluster.TestCurrentClusterInitialFailoverVersion + 3*cluster.TestFailoverVersionIncrement, // this is incremented to indicate there was a change in replication config
					LastUpdatedTime:         timeSource.Now().UnixNano(),
				}

				domainManager.EXPECT().UpdateDomain(ctx, expectedUpdateRequest).Return(nil).Times(1)
				domainReplicator.EXPECT().
					HandleTransmissionTask(
						ctx,
						types.DomainOperationUpdate,
						expectedUpdateRequest.Info,
						domainResponse.Config,
						expectedUpdateRequest.ReplicationConfig,
						domainResponse.ConfigVersion+1,
						cluster.TestCurrentClusterInitialFailoverVersion+3*cluster.TestFailoverVersionIncrement,
						int64(-1), // previous failover version is not applicable to active-active domain
						domainResponse.IsGlobalDomain,
					).Return(nil).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name: constants.TestDomainName,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								cluster.TestRegion1: {
									ActiveClusterName: cluster.TestCurrentClusterName,
									FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
								},
								cluster.TestRegion2: {
									ActiveClusterName: cluster.TestAlternativeClusterName,
									FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
								},
							},
						},
					},
				},
			},
			response: func(timeSource clock.MockedTimeSource) *types.UpdateDomainResponse {
				data, _ := json.Marshal([]FailoverEvent{{
					EventTime:    timeSource.Now(),
					FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeForce).String(),
					FromCluster:  cluster.TestCurrentClusterName,
					ToActiveClusters: types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									cluster.TestRegion1: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
									},
									cluster.TestRegion2: {
										ActiveClusterName: cluster.TestAlternativeClusterName,
										FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
									},
								},
							},
						},
					},
				}})

				return &types.UpdateDomainResponse{
					IsGlobalDomain:  true,
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion + 3*cluster.TestFailoverVersionIncrement,
					DomainInfo: &types.DomainInfo{
						Name:   constants.TestDomainName,
						UUID:   constants.TestDomainID,
						Status: common.Ptr(types.DomainStatusRegistered),
						Data:   map[string]string{commonconstants.DomainDataKeyForFailoverHistory: string(data)},
					},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 1,
						EmitMetric:                             true,
						HistoryArchivalStatus:                  common.Ptr(types.ArchivalStatusDisabled),
						VisibilityArchivalStatus:               common.Ptr(types.ArchivalStatusDisabled),
						BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:                        &types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
					},
					ReplicationConfiguration: &types.DomainReplicationConfiguration{
						Clusters: []*types.ClusterReplicationConfiguration{
							{ClusterName: cluster.TestCurrentClusterName},
							{ClusterName: cluster.TestAlternativeClusterName},
						},
						ActiveClusterName: cluster.TestCurrentClusterName,
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										cluster.TestRegion1: {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
										},
										cluster.TestRegion2: {
											ActiveClusterName: cluster.TestAlternativeClusterName,
											FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
										},
									},
								},
							},
						},
					},
				}
			},
		},
		{
			name: "Error case - local domain force failover - shoudl not be able to failover a local domain",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, _ *MockReplicator) {
				domainResponse := &persistence.GetDomainResponse{
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName}},
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					Info: &persistence.DomainInfo{
						Name:   constants.TestDomainName,
						ID:     constants.TestDomainID,
						Status: persistence.DomainStatusRegistered,
					},
					IsGlobalDomain:  false,
					LastUpdatedTime: timeSource.Now().UnixNano(),
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion,
				}
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(domainResponse, nil).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name:              constants.TestDomainName,
				ActiveClusterName: common.Ptr(cluster.TestCurrentClusterName),
			},
			response: func(_ clock.MockedTimeSource) *types.UpdateDomainResponse {
				return &types.UpdateDomainResponse{
					IsGlobalDomain:  false,
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion/cluster.TestFailoverVersionIncrement*cluster.TestFailoverVersionIncrement + cluster.TestCurrentClusterInitialFailoverVersion,
					DomainInfo: &types.DomainInfo{
						Name:   constants.TestDomainName,
						UUID:   constants.TestDomainID,
						Status: common.Ptr(types.DomainStatusRegistered),
					},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 1,
						EmitMetric:                             true,
						HistoryArchivalStatus:                  common.Ptr(types.ArchivalStatusDisabled),
						VisibilityArchivalStatus:               common.Ptr(types.ArchivalStatusDisabled),
						BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:                        &types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
					},
					ReplicationConfiguration: &types.DomainReplicationConfiguration{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*types.ClusterReplicationConfiguration{
							{ClusterName: cluster.TestCurrentClusterName},
						},
					},
				}
			},
			err: errLocalDomainsCannotFailover,
		},
		{
			name: "Error case - GetMetadata error",
			setupMock: func(domainManager *persistence.MockDomainManager, _ *types.UpdateDomainRequest, _ *archiver.MockArchivalMetadata, _ clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(nil, errors.New("get-metadata-error")).Times(1)
			},
			err: errors.New("get-metadata-error"),
		},
		{
			name: "Error case - GetDomain error",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, _ *archiver.MockArchivalMetadata, _ clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).Return(nil, errors.New("get-domain-error")).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name: constants.TestDomainName,
			},
			err: errors.New("get-domain-error"),
		},
		{
			name: "Error case - updateHistoryArchivalState error",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, _ clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(&persistence.GetDomainResponse{
						ReplicationConfig: &persistence.DomainReplicationConfig{},
						Config:            &persistence.DomainConfig{},
					}, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalEnabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalEnabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalEnabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name: constants.TestDomainName,
			},
			err: errInvalidEvent,
		},
		{
			name: "Error case - updateVisibilityArchivalState error",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, _ clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(&persistence.GetDomainResponse{
						ReplicationConfig: &persistence.DomainReplicationConfig{},
						Config:            &persistence.DomainConfig{},
					}, nil).Times(1)
				archivalConfigHistory := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalConfigVisibility := archiver.NewArchivalConfig(
					commonconstants.ArchivalEnabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalEnabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalEnabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfigHistory).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfigVisibility).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name: constants.TestDomainName,
			},
			err: errInvalidEvent,
		},
		{
			name: "Error case - updateDomainConfiguration error",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, _ clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(&persistence.GetDomainResponse{
						ReplicationConfig: &persistence.DomainReplicationConfig{},
						Config:            &persistence.DomainConfig{},
						LastUpdatedTime:   time.Now().UnixNano(),
					}, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name: constants.TestDomainName,
				BadBinaries: &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{
					"bad-binary": {
						Reason: "test-reason",
					},
					"bad-binary-2": {
						Reason: "test-reason-2",
					},
				}},
			},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("Total resetBinaries cannot exceed the max limit: %v", maxLength),
			},
		},
		{
			name: "Error case - updateDeleteBadBinary error",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, _ clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(&persistence.GetDomainResponse{
						ReplicationConfig: &persistence.DomainReplicationConfig{},
						Config:            &persistence.DomainConfig{},
					}, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name:            constants.TestDomainName,
				DeleteBadBinary: common.Ptr("bad-binary"),
			},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("Bad binary checksum %v doesn't exists.", "bad-binary"),
			},
		},
		{
			name: "Error case - handleGracefulFailover error in the case of a global domain - it should return an error to the user",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, _ clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(&persistence.GetDomainResponse{
						Info: &persistence.DomainInfo{
							Name: constants.TestDomainName,
						},
						ReplicationConfig: &persistence.DomainReplicationConfig{},
						Config:            &persistence.DomainConfig{},
						IsGlobalDomain:    true,
					}, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name:                     constants.TestDomainName,
				FailoverTimeoutInSeconds: common.Int32Ptr(1),
			},
			err: errInvalidGracefulFailover,
		},
		{
			name: "Error case - validateDomainConfig error",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, _ clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(&persistence.GetDomainResponse{
						ReplicationConfig: &persistence.DomainReplicationConfig{},
						Config:            &persistence.DomainConfig{},
						IsGlobalDomain:    true,
						Info: &persistence.DomainInfo{
							Name: constants.TestDomainName,
						},
					}, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name: constants.TestDomainName,
			},
			err: errInvalidRetentionPeriod,
		},
		{
			name: "Error case - validateDomainReplicationConfigForUpdateDomain error",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, _ clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(&persistence.GetDomainResponse{
						ReplicationConfig: &persistence.DomainReplicationConfig{
							ActiveClusterName: "bad-cluster",
						},
						Config: &persistence.DomainConfig{
							Retention: 1,
						},
						Info: &persistence.DomainInfo{
							Name: constants.TestDomainName,
						},
						IsGlobalDomain: true,
					}, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name: constants.TestDomainName,
			},
			err: &types.BadRequestError{Message: fmt.Sprintf(
				"Invalid cluster name: %v",
				"bad-cluster",
			)},
		},
		{
			name: "Error case - updateChangesForUpdateDomain error",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(&persistence.GetDomainResponse{
						ReplicationConfig: &persistence.DomainReplicationConfig{
							ActiveClusterName: cluster.TestCurrentClusterName,
							Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
						},
						Config: &persistence.DomainConfig{
							Retention: 1,
						},
						Info: &persistence.DomainInfo{
							Name: constants.TestDomainName,
						},
						IsGlobalDomain:  true,
						LastUpdatedTime: timeSource.Now().UnixNano(),
					}, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
				timeSource.Advance(-time.Hour)
			},
			request: &types.UpdateDomainRequest{
				Name:              constants.TestDomainName,
				ActiveClusterName: common.Ptr(cluster.TestAlternativeClusterName),
			},
			err: errDomainUpdateTooFrequent,
		},
		{
			name: "Error case - domainManager.UpdateDomain error",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, _ *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(&persistence.GetDomainResponse{
						ReplicationConfig: &persistence.DomainReplicationConfig{
							ActiveClusterName: cluster.TestCurrentClusterName,
							Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
						},
						Config: &persistence.DomainConfig{
							Retention: 1,
						},
						Info: &persistence.DomainInfo{
							Name: constants.TestDomainName,
						},
						IsGlobalDomain:  true,
						LastUpdatedTime: timeSource.Now().UnixNano(),
					}, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
				timeSource.Advance(time.Hour)
				domainManager.EXPECT().UpdateDomain(ctx, gomock.Any()).Return(errors.New("update-domain-error")).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name:              constants.TestDomainName,
				ActiveClusterName: common.Ptr(cluster.TestAlternativeClusterName),
			},
			err: errors.New("update-domain-error"),
		},
		{
			name: "Success case - active-active domain failover with explicit ActiveClusterName uses correct cluster for failover version",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, domainReplicator *MockReplicator) {
				domainResponse := &persistence.GetDomainResponse{
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName},
							{ClusterName: cluster.TestAlternativeClusterName},
						},
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										cluster.TestRegion1: {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
										},
										cluster.TestRegion2: {
											ActiveClusterName: cluster.TestAlternativeClusterName,
											FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
										},
									},
								},
							},
						},
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					Info: &persistence.DomainInfo{
						Name:   constants.TestDomainName,
						ID:     constants.TestDomainID,
						Status: persistence.DomainStatusRegistered,
					},
					IsGlobalDomain:  true,
					LastUpdatedTime: timeSource.Now().UnixNano(),
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion,
				}
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(domainResponse, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
				timeSource.Advance(time.Hour)

				data, _ := json.Marshal([]FailoverEvent{{
					EventTime:    timeSource.Now(),
					FromCluster:  cluster.TestCurrentClusterName,
					ToCluster:    cluster.TestAlternativeClusterName,
					FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeForce).String(),
					FromActiveClusters: types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									cluster.TestRegion1: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
									},
									cluster.TestRegion2: {
										ActiveClusterName: cluster.TestAlternativeClusterName,
										FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
									},
								},
							},
						},
					},
					ToActiveClusters: types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									cluster.TestRegion1: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
									},
									cluster.TestRegion2: {
										ActiveClusterName: cluster.TestAlternativeClusterName,
										FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
									},
								},
							},
						},
					},
				}})

				expectedUpdateRequest := &persistence.UpdateDomainRequest{
					Info: &persistence.DomainInfo{
						Name:        constants.TestDomainName,
						ID:          constants.TestDomainID,
						Status:      persistence.DomainStatusRegistered,
						Description: domainResponse.Info.Description,
						OwnerEmail:  domainResponse.Info.OwnerEmail,
						Data: map[string]string{
							commonconstants.DomainDataKeyForFailoverHistory: string(data),
						},
					},
					Config: domainResponse.Config,
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestAlternativeClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName},
							{ClusterName: cluster.TestAlternativeClusterName},
						},
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										cluster.TestRegion1: {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
										},
										cluster.TestRegion2: {
											ActiveClusterName: cluster.TestAlternativeClusterName,
											FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
										},
									},
								},
							},
						},
					},
					PreviousFailoverVersion: -1,
					ConfigVersion:           domainResponse.ConfigVersion,
					FailoverVersion:         cluster.TestAlternativeClusterInitialFailoverVersion,
					LastUpdatedTime:         timeSource.Now().UnixNano(),
				}

				domainManager.EXPECT().UpdateDomain(ctx, expectedUpdateRequest).Return(nil).Times(1)
				domainReplicator.EXPECT().
					HandleTransmissionTask(
						ctx,
						types.DomainOperationUpdate,
						expectedUpdateRequest.Info,
						domainResponse.Config,
						expectedUpdateRequest.ReplicationConfig,
						domainResponse.ConfigVersion,
						cluster.TestAlternativeClusterInitialFailoverVersion,
						int64(-1),
						domainResponse.IsGlobalDomain,
					).Return(nil).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name:              constants.TestDomainName,
				ActiveClusterName: common.Ptr(cluster.TestAlternativeClusterName),
			},
			response: func(timeSource clock.MockedTimeSource) *types.UpdateDomainResponse {
				data, _ := json.Marshal([]FailoverEvent{{
					EventTime:    timeSource.Now(),
					FromCluster:  cluster.TestCurrentClusterName,
					ToCluster:    cluster.TestAlternativeClusterName,
					FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeForce).String(),
					FromActiveClusters: types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									cluster.TestRegion1: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
									},
									cluster.TestRegion2: {
										ActiveClusterName: cluster.TestAlternativeClusterName,
										FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
									},
								},
							},
						},
					},
					ToActiveClusters: types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									cluster.TestRegion1: {
										ActiveClusterName: cluster.TestCurrentClusterName,
										FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
									},
									cluster.TestRegion2: {
										ActiveClusterName: cluster.TestAlternativeClusterName,
										FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
									},
								},
							},
						},
					},
				}})
				return &types.UpdateDomainResponse{
					IsGlobalDomain:  true,
					FailoverVersion: cluster.TestAlternativeClusterInitialFailoverVersion,
					DomainInfo: &types.DomainInfo{
						Name:   constants.TestDomainName,
						UUID:   constants.TestDomainID,
						Status: common.Ptr(types.DomainStatusRegistered),
						Data:   map[string]string{commonconstants.DomainDataKeyForFailoverHistory: string(data)},
					},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 1,
						EmitMetric:                             true,
						HistoryArchivalStatus:                  common.Ptr(types.ArchivalStatusDisabled),
						VisibilityArchivalStatus:               common.Ptr(types.ArchivalStatusDisabled),
						BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:                        &types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
					},
					ReplicationConfiguration: &types.DomainReplicationConfiguration{
						ActiveClusterName: cluster.TestAlternativeClusterName,
						Clusters: []*types.ClusterReplicationConfiguration{
							{ClusterName: cluster.TestCurrentClusterName},
							{ClusterName: cluster.TestAlternativeClusterName},
						},
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										cluster.TestRegion1: {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
										},
										cluster.TestRegion2: {
											ActiveClusterName: cluster.TestAlternativeClusterName,
											FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
										},
									},
								},
							},
						},
					},
				}
			},
		},
		{
			name: "Error case - HandleTransmissionTask error",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, domainReplicator *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(&persistence.GetDomainResponse{
						ReplicationConfig: &persistence.DomainReplicationConfig{
							ActiveClusterName: cluster.TestCurrentClusterName,
							Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
						},
						Config: &persistence.DomainConfig{
							Retention: 1,
						},
						Info: &persistence.DomainInfo{
							Name: constants.TestDomainName,
						},
						IsGlobalDomain:  true,
						LastUpdatedTime: timeSource.Now().UnixNano(),
					}, nil).Times(1)
				archivalConfig := archiver.NewArchivalConfig(
					commonconstants.ArchivalDisabled,
					dynamicproperties.GetStringPropertyFn(commonconstants.ArchivalDisabled),
					false,
					dynamicproperties.GetBoolPropertyFn(false),
					commonconstants.ArchivalDisabled,
					"")
				archivalMetadata.On("GetHistoryConfig").Return(archivalConfig).Times(1)
				archivalMetadata.On("GetVisibilityConfig").Return(archivalConfig).Times(1)
				timeSource.Advance(time.Hour)
				domainManager.EXPECT().UpdateDomain(ctx, gomock.Any()).Return(nil).Times(1)
				domainReplicator.EXPECT().
					HandleTransmissionTask(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("handle-transmission-task-error")).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name:              constants.TestDomainName,
				ActiveClusterName: common.Ptr(cluster.TestAlternativeClusterName),
			},
			err: errors.New("handle-transmission-task-error"),
		},
		{
			name: "Error case - update deprecated domain should return errDomainDeprecated",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.UpdateDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, _ *MockReplicator) {
				domainResponse := &persistence.GetDomainResponse{
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					Info: &persistence.DomainInfo{
						Name:   constants.TestDomainName,
						ID:     constants.TestDomainID,
						Status: persistence.DomainStatusDeprecated,
					},
					IsGlobalDomain:  true,
					LastUpdatedTime: timeSource.Now().UnixNano(),
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion,
				}
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()}).
					Return(domainResponse, nil).Times(1)
			},
			request: &types.UpdateDomainRequest{
				Name:              constants.TestDomainName,
				ActiveClusterName: common.Ptr(cluster.TestAlternativeClusterName),
			},
			err: errDomainDeprecated,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDomainManager := persistence.NewMockDomainManager(ctrl)
			mockReplicator := NewMockReplicator(ctrl)
			mockArchivalMetadata := &archiver.MockArchivalMetadata{}

			testConfig := Config{
				MinRetentionDays:       dynamicproperties.GetIntPropertyFn(1),
				MaxRetentionDays:       dynamicproperties.GetIntPropertyFn(5),
				RequiredDomainDataKeys: nil,
				MaxBadBinaryCount:      dynamicproperties.GetIntPropertyFilteredByDomain(maxLength),
				FailoverCoolDown:       func(string) time.Duration { return time.Second },
				FailoverHistoryMaxSize: dynamicproperties.GetIntPropertyFilteredByDomain(5),
			}

			clusterMetadata := cluster.GetTestClusterMetadata(true)
			mockTimeSource := clock.NewMockedTimeSourceAt(time.Unix(1761769472, 0))

			handler := handlerImpl{
				domainManager:       mockDomainManager,
				clusterMetadata:     clusterMetadata,
				domainReplicator:    mockReplicator,
				domainAttrValidator: newAttrValidator(clusterMetadata, int32(testConfig.MinRetentionDays())),
				archivalMetadata:    mockArchivalMetadata,
				archiverProvider:    provider.NewArchiverProvider(nil, nil),
				timeSource:          mockTimeSource,
				config:              testConfig,
				logger:              log.NewNoop(),
			}

			tc.setupMock(mockDomainManager, tc.request, mockArchivalMetadata, mockTimeSource, mockReplicator)

			response, err := handler.UpdateDomain(ctx, tc.request)

			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, tc.response(mockTimeSource), response)
			}
		})
	}
}

func TestUpdateDomainInfo(t *testing.T) {
	testCases := []struct {
		name              string
		request           *types.UpdateDomainRequest
		changed           bool
		updatedDomainInfo *persistence.DomainInfo
	}{
		{
			name: "Success case - new domain info",
			request: &types.UpdateDomainRequest{
				Description: common.Ptr("new-description"),
				OwnerEmail:  common.Ptr("new-email"),
				Data:        map[string]string{"new-key": "new-value"},
			},
			changed: true,
			updatedDomainInfo: &persistence.DomainInfo{
				ID:          constants.TestDomainID,
				Name:        constants.TestDomainName,
				Status:      persistence.DomainStatusRegistered,
				Description: "new-description",
				OwnerEmail:  "new-email",
				Data:        map[string]string{"key": "value", "new-key": "new-value"},
			},
		},
		{
			name:    "Success case - no new domain info in request",
			request: &types.UpdateDomainRequest{},
			updatedDomainInfo: &persistence.DomainInfo{
				ID:          constants.TestDomainID,
				Name:        constants.TestDomainName,
				Status:      persistence.DomainStatusRegistered,
				Description: "some-description",
				OwnerEmail:  "some-email",
				Data:        map[string]string{"key": "value"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			domainInfo := &persistence.DomainInfo{
				ID:          constants.TestDomainID,
				Name:        constants.TestDomainName,
				Status:      persistence.DomainStatusRegistered,
				Description: "some-description",
				OwnerEmail:  "some-email",
				Data:        map[string]string{"key": "value"},
			}

			mockDomainMgr := persistence.NewMockDomainManager(controller)
			mockReplicator := NewMockReplicator(controller)

			handler := newTestHandler(mockDomainMgr, true, mockReplicator)

			updatedDomainInfo, changed := (*handlerImpl).updateDomainInfo(handler.(*handlerImpl), tc.request, domainInfo)

			assert.Equal(t, tc.changed, changed)
			assert.Equal(t, tc.updatedDomainInfo, updatedDomainInfo)
		})
	}
}

func TestUpdateDomainConfiguration(t *testing.T) {
	testCases := []struct {
		name                string
		request             *types.UpdateDomainRequest
		changed             bool
		updatedDomainConfig func(now int64) *persistence.DomainConfig
		err                 error
	}{
		{
			name: "Success case - new domain config",
			request: &types.UpdateDomainRequest{
				EmitMetric:                             common.Ptr(false),
				WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(3),
				BadBinaries: &types.BadBinaries{
					Binaries: map[string]*types.BadBinaryInfo{
						"bad-binary": {
							Reason: "test-reason",
						},
					},
				},
			},
			changed: true,
			updatedDomainConfig: func(now int64) *persistence.DomainConfig {
				return &persistence.DomainConfig{
					Retention:                3,
					EmitMetric:               false,
					HistoryArchivalStatus:    types.ArchivalStatusDisabled,
					HistoryArchivalURI:       "",
					VisibilityArchivalStatus: types.ArchivalStatusDisabled,
					VisibilityArchivalURI:    "",
					BadBinaries: types.BadBinaries{
						Binaries: map[string]*types.BadBinaryInfo{
							"bad-binary": {
								Reason:          "test-reason",
								CreatedTimeNano: common.Ptr(now),
							},
						},
					},
					IsolationGroups:     types.IsolationGroupConfiguration{},
					AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{Enabled: true},
				}
			},
		},
		{
			name: "Success case - new domain config - only new bad binaries",
			request: &types.UpdateDomainRequest{
				BadBinaries: &types.BadBinaries{
					Binaries: map[string]*types.BadBinaryInfo{
						"bad-binary": {
							Reason: "test-reason",
						},
					},
				},
			},
			changed: false,
			updatedDomainConfig: func(now int64) *persistence.DomainConfig {
				return &persistence.DomainConfig{
					Retention:                1,
					EmitMetric:               true,
					HistoryArchivalStatus:    types.ArchivalStatusDisabled,
					HistoryArchivalURI:       "",
					VisibilityArchivalStatus: types.ArchivalStatusDisabled,
					VisibilityArchivalURI:    "",
					BadBinaries: types.BadBinaries{
						Binaries: map[string]*types.BadBinaryInfo{
							"bad-binary": {
								Reason:          "test-reason",
								CreatedTimeNano: common.Ptr(now),
							},
						},
					},
					IsolationGroups:     types.IsolationGroupConfiguration{},
					AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{Enabled: true},
				}
			},
		},
		{
			name: "Error case - new domain config has bad binaries greater than max length",
			request: &types.UpdateDomainRequest{
				EmitMetric:                             common.Ptr(false),
				WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(3),
				BadBinaries: &types.BadBinaries{
					Binaries: map[string]*types.BadBinaryInfo{
						"bad-binary": {
							Reason: "test-reason",
						},
						"bad-binary-2": {
							Reason: "test-reason-2",
						},
						"bad-binary-3": {
							Reason: "test-reason-3",
						},
						"bad-binary-4": {
							Reason: "test-reason-4",
						},
					},
				},
			},
			err: &types.BadRequestError{
				Message: fmt.Sprintf("Total resetBinaries cannot exceed the max limit: %v", 3),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDomainMgr := persistence.NewMockDomainManager(controller)
			mockReplicator := NewMockReplicator(controller)

			handler := newTestHandler(mockDomainMgr, true, mockReplicator)

			cfg := &persistence.DomainConfig{
				Retention:                1,
				EmitMetric:               true,
				HistoryArchivalStatus:    types.ArchivalStatusDisabled,
				HistoryArchivalURI:       "",
				VisibilityArchivalStatus: types.ArchivalStatusDisabled,
				VisibilityArchivalURI:    "",
				BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
				IsolationGroups:          types.IsolationGroupConfiguration{},
				AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
			}

			now := handler.(*handlerImpl).timeSource.Now().UnixNano()

			updatedDomainConfig, changed, err := (*handlerImpl).updateDomainConfiguration(handler.(*handlerImpl), constants.TestDomainName, cfg, tc.request)

			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.changed, changed)
				assert.Equal(t, tc.updatedDomainConfig(now), updatedDomainConfig)
			}
		})
	}
}

func TestUpdateDeleteBadBinary(t *testing.T) {
	now := time.Now().UnixNano()

	testCases := []struct {
		name                string
		deleteBadBinary     *string
		changed             bool
		updatedDomainConfig *persistence.DomainConfig
		err                 error
	}{
		{
			name:            "Success case - deleteBadBinary not nil",
			deleteBadBinary: common.Ptr("bad-binary"),
			changed:         true,
			updatedDomainConfig: &persistence.DomainConfig{
				Retention:                1,
				EmitMetric:               true,
				HistoryArchivalStatus:    types.ArchivalStatusDisabled,
				HistoryArchivalURI:       "",
				VisibilityArchivalStatus: types.ArchivalStatusDisabled,
				VisibilityArchivalURI:    "",
				BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
				IsolationGroups:          types.IsolationGroupConfiguration{},
				AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
			},
		},
		{
			name: "Success case - deleteBadBinary nil",
			updatedDomainConfig: &persistence.DomainConfig{
				Retention:                1,
				EmitMetric:               true,
				HistoryArchivalStatus:    types.ArchivalStatusDisabled,
				HistoryArchivalURI:       "",
				VisibilityArchivalStatus: types.ArchivalStatusDisabled,
				VisibilityArchivalURI:    "",
				BadBinaries: types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{
					"bad-binary": {
						Reason:          "test-reason",
						CreatedTimeNano: &now,
					},
				}},
				IsolationGroups:     types.IsolationGroupConfiguration{},
				AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{Enabled: true},
			},
		},
		{
			name:            "Error case - deleteBadBinary not in config.BadBinaries.Binaries",
			deleteBadBinary: common.Ptr("bad-binary-2"),
			err:             &types.BadRequestError{Message: "Bad binary checksum bad-binary-2 doesn't exists."},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDomainMgr := persistence.NewMockDomainManager(controller)
			mockReplicator := NewMockReplicator(controller)

			handler := newTestHandler(mockDomainMgr, true, mockReplicator)

			cfg := &persistence.DomainConfig{
				Retention:                1,
				EmitMetric:               true,
				HistoryArchivalStatus:    types.ArchivalStatusDisabled,
				HistoryArchivalURI:       "",
				VisibilityArchivalStatus: types.ArchivalStatusDisabled,
				VisibilityArchivalURI:    "",
				BadBinaries: types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{
					"bad-binary": {
						Reason:          "test-reason",
						CreatedTimeNano: &now,
					},
				}},
				IsolationGroups:     types.IsolationGroupConfiguration{},
				AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{Enabled: true},
			}

			updatedDomainConfig, changed, err := (*handlerImpl).updateDeleteBadBinary(handler.(*handlerImpl), cfg, tc.deleteBadBinary)

			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.changed, changed)
				assert.Equal(t, tc.updatedDomainConfig, updatedDomainConfig)
			}
		})
	}
}

func TestUpdateReplicationConfig(t *testing.T) {
	cfg := func() *persistence.DomainReplicationConfig {
		return &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
		}
	}

	activeActiveCfg := func() *persistence.DomainReplicationConfig {
		return &persistence.DomainReplicationConfig{
			Clusters: []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
			ActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							cluster.TestRegion1: {
								ActiveClusterName: cluster.TestCurrentClusterName,
								FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
							},
							cluster.TestRegion2: {
								ActiveClusterName: cluster.TestAlternativeClusterName,
								FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
							},
						},
					},
				},
			},
		}
	}

	testCases := []struct {
		name                     string
		request                  *types.UpdateDomainRequest
		currentReplicationConfig *persistence.DomainReplicationConfig
		updatedReplicationConfig *persistence.DomainReplicationConfig
		clusterUpdated           bool
		activeClusterUpdated     bool
	}{
		{
			name:                     "Success case - no change",
			request:                  &types.UpdateDomainRequest{},
			currentReplicationConfig: cfg(),
			updatedReplicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
			},
		},
		{
			name: "Success case - cluster and activeCluster updated",
			request: &types.UpdateDomainRequest{
				ActiveClusterName: common.Ptr(cluster.TestAlternativeClusterName),
				Clusters:          []*types.ClusterReplicationConfiguration{{ClusterName: cluster.TestDisabledClusterName}, {ClusterName: cluster.TestAlternativeClusterName}, {ClusterName: cluster.TestCurrentClusterName}},
			},
			currentReplicationConfig: cfg(),
			updatedReplicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestAlternativeClusterName,
				Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestDisabledClusterName}, {ClusterName: cluster.TestAlternativeClusterName}, {ClusterName: cluster.TestCurrentClusterName}},
			},
			clusterUpdated:       true,
			activeClusterUpdated: true,
		},
		{
			name: "Success case - cluster and activeCluster updated with warning",
			request: &types.UpdateDomainRequest{
				ActiveClusterName: common.Ptr(cluster.TestAlternativeClusterName),
				Clusters:          []*types.ClusterReplicationConfiguration{{ClusterName: cluster.TestAlternativeClusterName}},
			},
			currentReplicationConfig: cfg(),
			updatedReplicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestAlternativeClusterName,
				Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestAlternativeClusterName}},
			},
			clusterUpdated:       true,
			activeClusterUpdated: true,
		},
		{
			name: "active-active domain - update cluster of region2",
			request: &types.UpdateDomainRequest{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								cluster.TestRegion2: {
									// changing this from alternative cluster to current cluster
									ActiveClusterName: cluster.TestCurrentClusterName,
									FailoverVersion:   99999, // this will be ignored
								},
							},
						},
					},
				},
			},
			currentReplicationConfig: activeActiveCfg(),
			updatedReplicationConfig: &persistence.DomainReplicationConfig{
				Clusters: []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								cluster.TestRegion1: {
									ActiveClusterName: cluster.TestCurrentClusterName,
									FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
								},
								cluster.TestRegion2: {
									ActiveClusterName: cluster.TestCurrentClusterName,
									FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion + cluster.TestFailoverVersionIncrement,
								},
							},
						},
					},
				},
			},
			clusterUpdated:       true,
			activeClusterUpdated: true,
		},
		{
			name: "active-active domain - add a new cluster in a new region",
			request: &types.UpdateDomainRequest{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"region3": {
									ActiveClusterName: cluster.TestCurrentClusterName,
								},
							},
						},
					},
				},
			},
			currentReplicationConfig: activeActiveCfg(),
			updatedReplicationConfig: &persistence.DomainReplicationConfig{
				Clusters: []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								cluster.TestRegion1: {
									ActiveClusterName: cluster.TestCurrentClusterName,
									FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
								},
								cluster.TestRegion2: {
									ActiveClusterName: cluster.TestAlternativeClusterName,
									FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
								},
								"region3": {
									ActiveClusterName: cluster.TestCurrentClusterName,
									FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
								},
							},
						},
					},
				},
			},
			clusterUpdated:       true,
			activeClusterUpdated: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDomainMgr := persistence.NewMockDomainManager(controller)
			mockReplicator := NewMockReplicator(controller)

			handler := newTestHandler(mockDomainMgr, true, mockReplicator).(*handlerImpl)

			updatedReplicationConfig, clusterUpdated, activeClusterUpdated, err := handler.updateReplicationConfig(
				constants.TestDomainName,
				tc.currentReplicationConfig,
				tc.request,
			)

			assert.NoError(t, err)
			assert.Equal(t, tc.clusterUpdated, clusterUpdated, "cluster-updated field was %v when it was expected to be %v", clusterUpdated, tc.clusterUpdated)
			assert.Equal(t, tc.activeClusterUpdated, activeClusterUpdated, "active-cluster-updated field was %v when it was expected to be %v", activeClusterUpdated, tc.activeClusterUpdated)
			assert.Equal(t, tc.updatedReplicationConfig, updatedReplicationConfig, "replication-config-updated was flagged as %v when it was expected to be %v", updatedReplicationConfig, tc.updatedReplicationConfig)
		})
	}
}

func TestHandleGracefulFailover(t *testing.T) {
	failoverTimeoutInSeconds := int32(1)
	failoverVersion := int64(3)

	testCases := []struct {
		name                           string
		replicationConfig              *persistence.DomainReplicationConfig
		currentActiveCluster           string
		gracefulFailoverEndTime        *int64
		activeClusterChange            bool
		isGlobalDomain                 bool
		updatedGracefulFailoverEndTime func(now time.Time) *int64
		err                            error
	}{
		{
			name: "Success case",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
			},
			currentActiveCluster: cluster.TestAlternativeClusterName,
			activeClusterChange:  true,
			isGlobalDomain:       true,
			updatedGracefulFailoverEndTime: func(now time.Time) *int64 {
				return common.Ptr(now.Add(time.Duration(failoverTimeoutInSeconds) * time.Second).UnixNano())
			},
		},
		{
			name:                "Error case - activeClusterChange is false",
			activeClusterChange: false,
			isGlobalDomain:      true,
			err:                 errInvalidGracefulFailover,
		},
		{
			name:                "Error case - isGlobalDomain is false",
			activeClusterChange: true,
			isGlobalDomain:      false,
			err:                 errInvalidGracefulFailover,
		},
		{
			name: "Error case - replication ActiveClusterName is different from clusterMetadata currentClusterName",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestAlternativeClusterName,
				Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
			},
			activeClusterChange: true,
			isGlobalDomain:      true,
			err:                 errCannotDoGracefulFailoverFromCluster,
		},
		{
			name: "Error case - replication ActiveClusterName is the same as target currentActiveCluster",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
			},
			currentActiveCluster: cluster.TestCurrentClusterName,
			activeClusterChange:  true,
			isGlobalDomain:       true,
			err:                  errGracefulFailoverInActiveCluster,
		},
		{
			name: "Error case - ongoing failover, cannot have concurrent failover",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
			},
			currentActiveCluster:    cluster.TestAlternativeClusterName,
			activeClusterChange:     true,
			isGlobalDomain:          true,
			gracefulFailoverEndTime: common.Int64Ptr(time.Now().UnixNano()),
			err:                     errOngoingGracefulFailover,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockDomainMgr := persistence.NewMockDomainManager(controller)
			mockReplicator := NewMockReplicator(controller)

			handler := newTestHandler(mockDomainMgr, true, mockReplicator)

			now := handler.(*handlerImpl).timeSource.Now()

			request := &types.UpdateDomainRequest{
				FailoverTimeoutInSeconds: common.Int32Ptr(failoverTimeoutInSeconds),
			}

			gracefulFailoverEndTime, previousFailoverVersion, err := (*handlerImpl).handleGracefulFailover(
				handler.(*handlerImpl),
				request,
				tc.replicationConfig,
				tc.currentActiveCluster,
				tc.gracefulFailoverEndTime,
				failoverVersion,
				tc.activeClusterChange,
				tc.isGlobalDomain,
			)

			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.updatedGracefulFailoverEndTime(now), gracefulFailoverEndTime)
				assert.Equal(t, failoverVersion, previousFailoverVersion)
			}
		})
	}
}

func TestUpdateFailoverHistory(t *testing.T) {
	now := time.Now()
	fromCluster := "fromCluster"
	toCluster := "toCluster"
	failoverType := commonconstants.FailoverType(commonconstants.FailoverTypeForce)
	failoverHistoryMaxSize := 5

	testCases := []struct {
		name             string
		domainInfo       func() *persistence.DomainInfo
		newFailoverEvent FailoverEvent
		response         func() string
		err              error
	}{
		{
			name:       "Success case - DomainInfo data is nil",
			domainInfo: func() *persistence.DomainInfo { return &persistence.DomainInfo{} },
			newFailoverEvent: FailoverEvent{
				EventTime:    now,
				FromCluster:  fromCluster,
				ToCluster:    toCluster,
				FailoverType: failoverType.String(),
			},
			response: func() string {
				failoverHistory := []FailoverEvent{{EventTime: now, FromCluster: fromCluster, ToCluster: toCluster, FailoverType: failoverType.String()}}
				jsonResp, _ := json.Marshal(failoverHistory)
				return string(jsonResp)
			},
		},
		{
			name:       "Success case - FailoverHistory is nil",
			domainInfo: func() *persistence.DomainInfo { return &persistence.DomainInfo{Data: map[string]string{}} },
			newFailoverEvent: FailoverEvent{
				EventTime:    now,
				FromCluster:  fromCluster,
				ToCluster:    toCluster,
				FailoverType: failoverType.String(),
			},
			response: func() string {
				failoverHistory := []FailoverEvent{{EventTime: now, FromCluster: fromCluster, ToCluster: toCluster, FailoverType: failoverType.String()}}
				jsonResp, _ := json.Marshal(failoverHistory)
				return string(jsonResp)
			},
		},
		{
			name: "Success case - FailoverHistory is not nil",
			domainInfo: func() *persistence.DomainInfo {
				eventTime := time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC)
				failoverHistory := []FailoverEvent{{EventTime: eventTime, FromCluster: "fromCluster1", ToCluster: "toCluster1", FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeGrace).String()}}
				failoverHistoryJSON, _ := json.Marshal(failoverHistory)
				return &persistence.DomainInfo{Data: map[string]string{commonconstants.DomainDataKeyForFailoverHistory: string(failoverHistoryJSON)}}
			},
			newFailoverEvent: FailoverEvent{
				EventTime:    now,
				FromCluster:  fromCluster,
				ToCluster:    toCluster,
				FailoverType: failoverType.String(),
			},
			response: func() string {
				eventTime := time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC)
				failoverHistory := []FailoverEvent{{EventTime: eventTime, FromCluster: "fromCluster1", ToCluster: "toCluster1", FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeGrace).String()}}
				failoverHistory = append([]FailoverEvent{{EventTime: now, FromCluster: fromCluster, ToCluster: toCluster, FailoverType: failoverType.String()}}, failoverHistory...)
				jsonResp, _ := json.Marshal(failoverHistory)
				return string(jsonResp)
			},
		},
		{
			name: "Success case - active-passive to active-active",
			domainInfo: func() *persistence.DomainInfo {
				eventTime := time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC)
				failoverHistory := []FailoverEvent{{EventTime: eventTime, FromCluster: "fromCluster1", ToCluster: "toCluster1", FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeGrace).String()}}
				failoverHistoryJSON, _ := json.Marshal(failoverHistory)
				return &persistence.DomainInfo{Data: map[string]string{commonconstants.DomainDataKeyForFailoverHistory: string(failoverHistoryJSON)}}
			},
			newFailoverEvent: FailoverEvent{
				EventTime:   now,
				FromCluster: fromCluster,
				ToCluster:   "",
				ToActiveClusters: types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								cluster.TestRegion1: {
									ActiveClusterName: "fromCluster1",
								},
								cluster.TestRegion2: {
									ActiveClusterName: "fromCluster2",
								},
							},
						},
					},
				},
				FailoverType: failoverType.String(),
			},
			response: func() string {
				eventTime := time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC)
				failoverHistory := []FailoverEvent{{EventTime: eventTime, FromCluster: "fromCluster1", ToCluster: "toCluster1", FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeGrace).String()}}
				failoverHistory = append([]FailoverEvent{
					{
						EventTime:   now,
						FromCluster: fromCluster,
						ToCluster:   "",
						ToActiveClusters: types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										cluster.TestRegion1: {
											ActiveClusterName: "fromCluster1",
										},
										cluster.TestRegion2: {
											ActiveClusterName: "fromCluster2",
										},
									},
								},
							},
						},
						FailoverType: failoverType.String(),
					}}, failoverHistory...)
				jsonResp, _ := json.Marshal(failoverHistory)
				return string(jsonResp)
			},
		},
		{
			name: "Success case - FailoverHistory is at max size",
			domainInfo: func() *persistence.DomainInfo {
				var failoverHistory []FailoverEvent
				for i := 0; i < failoverHistoryMaxSize; i++ {
					eventTime := time.Date(2021, 1, i, 1, 1, 1, 1, time.UTC)
					failoverHistory = append(failoverHistory, FailoverEvent{EventTime: eventTime, FromCluster: "fromCluster" + strconv.Itoa(i), ToCluster: "toCluster" + strconv.Itoa(i), FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeGrace).String()})
				}
				failoverHistoryJSON, _ := json.Marshal(failoverHistory)
				return &persistence.DomainInfo{Data: map[string]string{commonconstants.DomainDataKeyForFailoverHistory: string(failoverHistoryJSON)}}
			},
			newFailoverEvent: FailoverEvent{
				EventTime:    now,
				FromCluster:  fromCluster,
				ToCluster:    toCluster,
				FailoverType: failoverType.String(),
			},
			response: func() string {
				var failoverHistory []FailoverEvent
				for i := 0; i < 5; i++ {
					eventTime := time.Date(2021, 1, i, 1, 1, 1, 1, time.UTC)
					failoverHistory = append(failoverHistory, FailoverEvent{EventTime: eventTime, FromCluster: "fromCluster" + strconv.Itoa(i), ToCluster: "toCluster" + strconv.Itoa(i), FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeGrace).String()})
				}
				failoverHistory = append([]FailoverEvent{{EventTime: now, FromCluster: fromCluster, ToCluster: toCluster, FailoverType: failoverType.String()}}, failoverHistory[:(5-1)]...)
				jsonResp, _ := json.Marshal(failoverHistory)
				return string(jsonResp)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				FailoverHistoryMaxSize: func(domain string) int {
					return failoverHistoryMaxSize
				},
			}

			domainInfo := tc.domainInfo()
			err := updateFailoverHistoryInDomainData(domainInfo, cfg, tc.newFailoverEvent)

			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, domainInfo.Data)
				assert.NotNil(t, domainInfo.Data[commonconstants.DomainDataKeyForFailoverHistory])
				assert.Equal(t, tc.response(), domainInfo.Data[commonconstants.DomainDataKeyForFailoverHistory])
			}
		})
	}
}

func TestHandler_FailoverDomain(t *testing.T) {
	ctx := context.Background()
	maxLength := 1

	testCases := []struct {
		name      string
		setupMock func(
			domainManager *persistence.MockDomainManager,
			updateRequest *types.FailoverDomainRequest,
			archivalMetadata *archiver.MockArchivalMetadata,
			timeSource clock.MockedTimeSource,
			domainReplicator *MockReplicator,
		)
		request  *types.FailoverDomainRequest
		response func(timeSource clock.MockedTimeSource) *types.FailoverDomainResponse
		err      error
	}{
		{
			name: "Success case - global domain force failover via replication config",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.FailoverDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, domainReplicator *MockReplicator) {
				domainResponse := &persistence.GetDomainResponse{
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					Info: &persistence.DomainInfo{
						Name:   constants.TestDomainName,
						ID:     constants.TestDomainID,
						Status: persistence.DomainStatusRegistered,
					},
					IsGlobalDomain:  true,
					LastUpdatedTime: timeSource.Now().UnixNano(),
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion,
				}
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetDomainName()}).
					Return(domainResponse, nil).Times(1)
				timeSource.Advance(time.Hour)
				domainManager.EXPECT().UpdateDomain(ctx, &persistence.UpdateDomainRequest{
					Info:                    domainResponse.Info,
					Config:                  domainResponse.Config,
					ReplicationConfig:       domainResponse.ReplicationConfig,
					PreviousFailoverVersion: commonconstants.InitialPreviousFailoverVersion,
					ConfigVersion:           domainResponse.ConfigVersion,
					FailoverVersion:         cluster.TestAlternativeClusterInitialFailoverVersion,
					LastUpdatedTime:         timeSource.Now().UnixNano(),
				}).Return(nil).Times(1)
				domainReplicator.EXPECT().
					HandleTransmissionTask(
						ctx,
						types.DomainOperationUpdate,
						domainResponse.Info,
						domainResponse.Config,
						domainResponse.ReplicationConfig,
						domainResponse.ConfigVersion,
						cluster.TestAlternativeClusterInitialFailoverVersion,
						commonconstants.InitialPreviousFailoverVersion,
						domainResponse.IsGlobalDomain,
					).Return(nil).Times(1)
			},
			request: &types.FailoverDomainRequest{
				DomainName:              constants.TestDomainName,
				DomainActiveClusterName: common.Ptr(cluster.TestAlternativeClusterName),
			},
			response: func(timeSource clock.MockedTimeSource) *types.FailoverDomainResponse {
				data, _ := json.Marshal([]FailoverEvent{{EventTime: timeSource.Now(), FromCluster: cluster.TestCurrentClusterName, ToCluster: cluster.TestAlternativeClusterName, FailoverType: commonconstants.FailoverType(commonconstants.FailoverTypeForce).String()}})
				return &types.FailoverDomainResponse{
					IsGlobalDomain:  true,
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion/cluster.TestFailoverVersionIncrement*cluster.TestFailoverVersionIncrement + cluster.TestAlternativeClusterInitialFailoverVersion,
					DomainInfo: &types.DomainInfo{
						Name:   constants.TestDomainName,
						UUID:   constants.TestDomainID,
						Data:   map[string]string{commonconstants.DomainDataKeyForFailoverHistory: string(data)},
						Status: common.Ptr(types.DomainStatusRegistered),
					},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 1,
						EmitMetric:                             true,
						HistoryArchivalStatus:                  common.Ptr(types.ArchivalStatusDisabled),
						VisibilityArchivalStatus:               common.Ptr(types.ArchivalStatusDisabled),
						BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:                        &types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
					},
					ReplicationConfiguration: &types.DomainReplicationConfiguration{
						ActiveClusterName: cluster.TestAlternativeClusterName,
						Clusters: []*types.ClusterReplicationConfiguration{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName},
						},
					},
				}
			},
		},
		{
			name: "Error case - domain not found",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.FailoverDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, domainReplicator *MockReplicator) {
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetDomainName()}).
					Return(nil, &types.EntityNotExistsError{Message: "Domain not found"}).Times(1)
			},
			request: &types.FailoverDomainRequest{
				DomainName:              constants.TestDomainName,
				DomainActiveClusterName: common.Ptr(cluster.TestAlternativeClusterName),
			},
			err: &types.EntityNotExistsError{Message: "Domain not found"},
		},
		{
			name: "Error case - update too frequent",
			setupMock: func(domainManager *persistence.MockDomainManager, updateRequest *types.FailoverDomainRequest, archivalMetadata *archiver.MockArchivalMetadata, timeSource clock.MockedTimeSource, domainReplicator *MockReplicator) {
				domainResponse := &persistence.GetDomainResponse{
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: cluster.TestCurrentClusterName}, {ClusterName: cluster.TestAlternativeClusterName}},
					},
					Config: &persistence.DomainConfig{
						Retention:                1,
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusDisabled,
						VisibilityArchivalStatus: types.ArchivalStatusDisabled,
						BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
						IsolationGroups:          types.IsolationGroupConfiguration{},
						AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
					},
					Info: &persistence.DomainInfo{
						Name:   constants.TestDomainName,
						ID:     constants.TestDomainID,
						Status: persistence.DomainStatusRegistered,
					},
					IsGlobalDomain:  true,
					LastUpdatedTime: timeSource.Now().UnixNano(), // Set to current time to trigger cool down
					FailoverVersion: cluster.TestCurrentClusterInitialFailoverVersion,
				}
				domainManager.EXPECT().GetMetadata(ctx).Return(&persistence.GetMetadataResponse{}, nil).Times(1)
				domainManager.EXPECT().GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetDomainName()}).
					Return(domainResponse, nil).Times(1)
			},
			request: &types.FailoverDomainRequest{
				DomainName:              constants.TestDomainName,
				DomainActiveClusterName: common.Ptr(cluster.TestCurrentClusterName),
			},
			err: errDomainUpdateTooFrequent,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDomainManager := persistence.NewMockDomainManager(ctrl)
			mockReplicator := NewMockReplicator(ctrl)
			mockArchivalMetadata := &archiver.MockArchivalMetadata{}

			testConfig := Config{
				MinRetentionDays:       dynamicproperties.GetIntPropertyFn(1),
				MaxRetentionDays:       dynamicproperties.GetIntPropertyFn(5),
				RequiredDomainDataKeys: nil,
				MaxBadBinaryCount:      dynamicproperties.GetIntPropertyFilteredByDomain(maxLength),
				FailoverCoolDown:       func(string) time.Duration { return time.Second },
				FailoverHistoryMaxSize: dynamicproperties.GetIntPropertyFilteredByDomain(5),
			}

			clusterMetadata := cluster.GetTestClusterMetadata(true)
			mockTimeSource := clock.NewMockedTimeSource()

			handler := handlerImpl{
				domainManager:       mockDomainManager,
				clusterMetadata:     clusterMetadata,
				domainReplicator:    mockReplicator,
				domainAttrValidator: newAttrValidator(clusterMetadata, int32(testConfig.MinRetentionDays())),
				archivalMetadata:    mockArchivalMetadata,
				archiverProvider:    provider.NewArchiverProvider(nil, nil),
				timeSource:          mockTimeSource,
				config:              testConfig,
				logger:              log.NewNoop(),
			}

			tc.setupMock(mockDomainManager, tc.request, mockArchivalMetadata, mockTimeSource, mockReplicator)

			response, err := handler.FailoverDomain(ctx, tc.request)

			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, tc.response(mockTimeSource), response)
			}
		})
	}
}

func TestBuildActiveActiveClustersFromUpdateRequest(t *testing.T) {

	testsCases := map[string]struct {
		updateRequest          *types.UpdateDomainRequest
		config                 *persistence.DomainReplicationConfig
		domainName             string
		handler                *handlerImpl
		expectedActiveClusters *types.ActiveClusters
		expectedIsChanged      bool
	}{
		"Success case - ActiveClusters - failover event - where there's existing cluster attributes and we expect them to be incremented": {
			updateRequest: &types.UpdateDomainRequest{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"location": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"nyc": {
									ActiveClusterName: "clusterC", // this is expected to be a failvoer to cluster C from the existing A
								},
							},
						},
					},
				},
			},
			config: &persistence.DomainReplicationConfig{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"location": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"nyc": {
									ActiveClusterName: "clusterA",
									FailoverVersion:   100,
								},
								"morocco": {
									ActiveClusterName: "clusterB",
									FailoverVersion:   1,
								},
								"tokyo": {
									ActiveClusterName: "clusterC",
									FailoverVersion:   2,
								},
							},
						},
					},
				},
			},
			expectedActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"nyc": {
								ActiveClusterName: "clusterC",
								FailoverVersion:   102,
							},
							"morocco": {
								ActiveClusterName: "clusterB",
								FailoverVersion:   1,
							},
							"tokyo": {
								ActiveClusterName: "clusterC",
								FailoverVersion:   2,
							},
						},
					},
				},
			},
			expectedIsChanged: true,
		},
		"Success case - ActiveClusters - where there is the introduction of cluster attributes for the first time - we should see that these results are reflected": {
			updateRequest: &types.UpdateDomainRequest{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"location": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"nyc": {
									ActiveClusterName: "clusterA",
									// failover version can be absent
								},
								"morocco": {
									ActiveClusterName: "clusterB",
									// failover version can be absent
								},
								"tokyo": {
									ActiveClusterName: "clusterC",
									// failover version can be absent
								},
							},
						},
					},
				},
			},
			expectedActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"nyc": {
								ActiveClusterName: "clusterA",
								FailoverVersion:   0,
							},
							"morocco": {
								ActiveClusterName: "clusterB",
								FailoverVersion:   1,
							},
							"tokyo": {
								ActiveClusterName: "clusterC",
								FailoverVersion:   2,
							},
						},
					},
				},
			},
			expectedIsChanged: true,
		},
		"Success case - ActiveClusters - where there existing cluster attributes. These should be merged": {
			updateRequest: &types.UpdateDomainRequest{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"location": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"nyc": {
									ActiveClusterName: "clusterA",
									// failover version can be absent
								},
							},
						},
					},
				},
			},
			config: &persistence.DomainReplicationConfig{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"location": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"tokyo": {
									ActiveClusterName: "clusterC",
									FailoverVersion:   2,
								},
								"morocco": {
									ActiveClusterName: "clusterB",
									FailoverVersion:   1,
								},
							},
						},
					},
				},
			},
			expectedActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"nyc": {
								ActiveClusterName: "clusterA",
								FailoverVersion:   0,
							},
							"tokyo": {
								ActiveClusterName: "clusterC",
								FailoverVersion:   2,
							},
							"morocco": {
								ActiveClusterName: "clusterB",
								FailoverVersion:   1,
							},
						},
					},
				},
			},
			expectedIsChanged: true,
		},
		"Success case - AttributeScopes is nil": {
			updateRequest: &types.UpdateDomainRequest{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: nil,
				},
			},
			expectedActiveClusters: nil,
			expectedIsChanged:      false,
		},
	}

	for name, tc := range testsCases {
		t.Run(name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDomainManager := persistence.NewMockDomainManager(ctrl)

			metadata := cluster.NewMetadata(
				config.ClusterGroupMetadata{
					FailoverVersionIncrement: 100,
					ClusterGroup: map[string]config.ClusterInformation{
						"clusterA": {
							InitialFailoverVersion: 0,
						},
						"clusterB": {
							InitialFailoverVersion: 1,
						},
						"clusterC": {
							InitialFailoverVersion: 2,
						},
						"clusterD": {
							InitialFailoverVersion: 3,
						},
					},
				},
				func(d string) bool { return false },
				metrics.NewNoopMetricsClient(),
				log.NewNoop(),
			)

			mockTimeSource := clock.NewMockedTimeSource()
			handler := handlerImpl{
				domainManager:    mockDomainManager,
				clusterMetadata:  metadata,
				archiverProvider: provider.NewArchiverProvider(nil, nil),
				timeSource:       mockTimeSource,
				logger:           log.NewNoop(),
			}

			activeClusters, isChanged := handler.buildActiveActiveClusterScopesFromUpdateRequest(tc.updateRequest, tc.config, tc.domainName)
			assert.Equal(t, tc.expectedActiveClusters, activeClusters)
			assert.Equal(t, tc.expectedIsChanged, isChanged)
		})
	}
}

func TestActiveClustersFromRegisterRequest(t *testing.T) {
	tests := []struct {
		name            string
		request         *types.RegisterDomainRequest
		expectedResult  *types.ActiveClusters
		expectedErr     error
		clusterMetadata func() cluster.Metadata
	}{
		{
			name: "local domain returns nil",
			request: &types.RegisterDomainRequest{
				Name:           "test-domain",
				IsGlobalDomain: false,
			},
			expectedResult: nil,
			expectedErr:    nil,
			clusterMetadata: func() cluster.Metadata {
				return cluster.GetTestClusterMetadata(true)
			},
		},
		{
			name: "global domain with no active cluster data returns nil",
			request: &types.RegisterDomainRequest{
				Name:           "test-domain",
				IsGlobalDomain: true,
			},
			expectedResult: nil,
			expectedErr:    nil,
			clusterMetadata: func() cluster.Metadata {
				return cluster.GetTestClusterMetadata(true)
			},
		},
		{
			name: "new AttributeScopes with valid clusters",
			request: &types.RegisterDomainRequest{
				Name:           "test-domain",
				IsGlobalDomain: true,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"datacenter": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"dc1": {
									ActiveClusterName: cluster.TestCurrentClusterName,
								},
								"dc2": {
									ActiveClusterName: cluster.TestAlternativeClusterName,
								},
							},
						},
					},
				},
			},
			expectedResult: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"datacenter": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"dc1": {
								ActiveClusterName: cluster.TestCurrentClusterName,
								FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
							},
							"dc2": {
								ActiveClusterName: cluster.TestAlternativeClusterName,
								FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
							},
						},
					},
				},
			},
			expectedErr: nil,
			clusterMetadata: func() cluster.Metadata {
				return cluster.GetTestClusterMetadata(true)
			},
		},
		{
			name: "new AttributeScopes with invalid cluster",
			request: &types.RegisterDomainRequest{
				Name:           "test-domain",
				IsGlobalDomain: true,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-west": {
									ActiveClusterName: "invalid-cluster",
								},
							},
						},
					},
				},
			},
			expectedResult: nil,
			expectedErr:    &types.BadRequestError{},
			clusterMetadata: func() cluster.Metadata {
				return cluster.GetTestClusterMetadata(true)
			},
		},
		{
			name: "multiple scopes with multiple attributes",
			request: &types.RegisterDomainRequest{
				Name:           "test-domain",
				IsGlobalDomain: true,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-west": {
									ActiveClusterName: cluster.TestCurrentClusterName,
								},
								"us-east": {
									ActiveClusterName: cluster.TestAlternativeClusterName,
								},
							},
						},
						"datacenter": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"dc1": {
									ActiveClusterName: cluster.TestCurrentClusterName,
								},
							},
						},
					},
				},
			},
			expectedResult: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: cluster.TestCurrentClusterName,
								FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
							},
							"us-east": {
								ActiveClusterName: cluster.TestAlternativeClusterName,
								FailoverVersion:   cluster.TestAlternativeClusterInitialFailoverVersion,
							},
						},
					},
					"datacenter": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"dc1": {
								ActiveClusterName: cluster.TestCurrentClusterName,
								FailoverVersion:   cluster.TestCurrentClusterInitialFailoverVersion,
							},
						},
					},
				},
			},
			expectedErr: nil,
			clusterMetadata: func() cluster.Metadata {
				return cluster.GetTestClusterMetadata(true)
			},
		},
		{
			name: "empty ActiveClusters with non-nil AttributeScopes",
			request: &types.RegisterDomainRequest{
				Name:           "test-domain",
				IsGlobalDomain: true,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{},
				},
			},
			expectedResult: nil,
			expectedErr:    nil,
			clusterMetadata: func() cluster.Metadata {
				return cluster.GetTestClusterMetadata(true)
			},
		},
		{
			name: "custom cluster metadata with different failover versions",
			request: &types.RegisterDomainRequest{
				Name:           "test-domain",
				IsGlobalDomain: true,
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"region1": {ActiveClusterName: "clusterA"},
								"region2": {ActiveClusterName: "clusterB"},
							},
						},
					},
				},
			},
			expectedResult: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"region1": {
								ActiveClusterName: "clusterA",
								FailoverVersion:   10,
							},
							"region2": {
								ActiveClusterName: "clusterB",
								FailoverVersion:   20,
							},
						},
					},
				},
			},
			expectedErr: nil,
			clusterMetadata: func() cluster.Metadata {
				return cluster.NewMetadata(
					config.ClusterGroupMetadata{
						FailoverVersionIncrement: 100,
						PrimaryClusterName:       "clusterA",
						CurrentClusterName:       "clusterA",
						ClusterGroup: map[string]config.ClusterInformation{
							"clusterA": {
								Enabled:                true,
								InitialFailoverVersion: 10,
							},
							"clusterB": {
								Enabled:                true,
								InitialFailoverVersion: 20,
							},
						},
					},
					func(d string) bool { return false },
					metrics.NewNoopMetricsClient(),
					log.NewNoop(),
				)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := &handlerImpl{
				clusterMetadata: tc.clusterMetadata(),
				logger:          log.NewNoop(),
			}

			result, err := handler.activeClustersFromRegisterRequest(tc.request)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.IsType(t, tc.expectedErr, err)
				if badReqErr, ok := tc.expectedErr.(*types.BadRequestError); ok {
					resultErr, ok := err.(*types.BadRequestError)
					assert.True(t, ok)
					if badReqErr.Message != "" {
						assert.Equal(t, badReqErr.Message, resultErr.Message)
					}
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func TestValidateDomainReplicationConfigForUpdateDomain(t *testing.T) {
	tests := []struct {
		name                 string
		replicationConfig    *persistence.DomainReplicationConfig
		isGlobalDomain       bool
		configurationChanged bool
		activeClusterChanged bool
		isPrimaryCluster     bool
		expectedErr          error
	}{
		{
			name: "local domain with valid config",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
				},
			},
			isGlobalDomain:       false,
			configurationChanged: false,
			activeClusterChanged: false,
			isPrimaryCluster:     true,
			expectedErr:          nil,
		},
		{
			name: "local domain with invalid active cluster",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestAlternativeClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			isGlobalDomain:       false,
			configurationChanged: false,
			activeClusterChanged: false,
			isPrimaryCluster:     true,
			expectedErr:          &types.BadRequestError{},
		},
		{
			name: "local domain with invalid cluster configuration",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			isGlobalDomain:       false,
			configurationChanged: false,
			activeClusterChanged: false,
			isPrimaryCluster:     true,
			expectedErr:          &types.BadRequestError{},
		},
		{
			name: "global domain with valid config on primary cluster - no changes",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			isGlobalDomain:       true,
			configurationChanged: false,
			activeClusterChanged: false,
			isPrimaryCluster:     true,
			expectedErr:          nil,
		},
		{
			name: "global domain config change only on primary cluster",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			isGlobalDomain:       true,
			configurationChanged: true,
			activeClusterChanged: false,
			isPrimaryCluster:     true,
			expectedErr:          nil,
		},
		{
			name: "global domain active cluster change only on primary cluster",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestAlternativeClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			isGlobalDomain:       true,
			configurationChanged: false,
			activeClusterChanged: true,
			isPrimaryCluster:     true,
			expectedErr:          nil,
		},
		{
			name: "global domain config change on non-primary cluster should fail",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			isGlobalDomain:       true,
			configurationChanged: true,
			activeClusterChanged: false,
			isPrimaryCluster:     false,
			expectedErr:          errNotPrimaryCluster,
		},
		{
			name: "global active-passive domain cannot change both config and active cluster",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestAlternativeClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
			},
			isGlobalDomain:       true,
			configurationChanged: true,
			activeClusterChanged: true,
			isPrimaryCluster:     true,
			expectedErr:          errCannotDoDomainFailoverAndUpdate,
		},
		{
			name: "global active-active domain can change both config and active cluster",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestAlternativeClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"region1": {ActiveClusterName: cluster.TestCurrentClusterName},
								"region2": {ActiveClusterName: cluster.TestAlternativeClusterName},
							},
						},
					},
				},
			},
			isGlobalDomain:       true,
			configurationChanged: true,
			activeClusterChanged: true,
			isPrimaryCluster:     true,
			expectedErr:          nil,
		},
		{
			name: "global active-active domain with AttributeScopes can change both",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestAlternativeClusterName,
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: cluster.TestCurrentClusterName},
					{ClusterName: cluster.TestAlternativeClusterName},
				},
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"datacenter": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"dc1": {ActiveClusterName: cluster.TestCurrentClusterName},
								"dc2": {ActiveClusterName: cluster.TestAlternativeClusterName},
							},
						},
					},
				},
			},
			isGlobalDomain:       true,
			configurationChanged: true,
			activeClusterChanged: true,
			isPrimaryCluster:     true,
			expectedErr:          nil,
		},
		{
			name: "global domain with invalid cluster name",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: "invalid-cluster",
				Clusters: []*persistence.ClusterReplicationConfig{
					{ClusterName: "invalid-cluster"},
				},
			},
			isGlobalDomain:       true,
			configurationChanged: false,
			activeClusterChanged: false,
			isPrimaryCluster:     true,
			expectedErr:          &types.BadRequestError{},
		},
		{
			name: "global domain with no clusters should fail",
			replicationConfig: &persistence.DomainReplicationConfig{
				ActiveClusterName: cluster.TestCurrentClusterName,
				Clusters:          []*persistence.ClusterReplicationConfig{},
			},
			isGlobalDomain:       true,
			configurationChanged: false,
			activeClusterChanged: false,
			isPrimaryCluster:     true,
			expectedErr:          &types.BadRequestError{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clusterMetadata := cluster.GetTestClusterMetadata(tc.isPrimaryCluster)
			handler := &handlerImpl{
				clusterMetadata:     clusterMetadata,
				domainAttrValidator: newAttrValidator(clusterMetadata, 1),
				logger:              log.NewNoop(),
			}

			err := handler.validateDomainReplicationConfigForUpdateDomain(
				tc.replicationConfig,
				tc.isGlobalDomain,
				tc.configurationChanged,
				tc.activeClusterChanged,
			)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.IsType(t, tc.expectedErr, err)
				if tc.expectedErr == errNotPrimaryCluster {
					assert.Equal(t, errNotPrimaryCluster, err)
				} else if tc.expectedErr == errCannotDoDomainFailoverAndUpdate {
					assert.Equal(t, errCannotDoDomainFailoverAndUpdate, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
