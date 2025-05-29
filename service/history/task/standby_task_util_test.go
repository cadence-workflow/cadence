package task

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestGetRemoteClusterName(t *testing.T) {
	testDomainID := "test-domain-id"
	testWorkflowID := "test-workflow-id"
	testRunID := "test-run-id"
	currentCluster := "cluster-A"
	remoteCluster := "cluster-B"

	tests := []struct {
		name           string
		setupMocks     func(*gomock.Controller) (cache.DomainCache, activecluster.Manager)
		taskInfo       persistence.Task
		expectedResult string
		expectedError  error
	}{
		{
			name: "domain cache error",
			setupMocks: func(ctrl *gomock.Controller) (cache.DomainCache, activecluster.Manager) {
				mockDomainCache := cache.NewMockDomainCache(ctrl)
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				mockDomainCache.EXPECT().
					GetDomainByID(testDomainID).
					Return(nil, errors.New("domain cache error"))

				return mockDomainCache, mockActiveClusterMgr
			},
			taskInfo: &persistence.DecisionTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
			},
			expectedResult: "",
			expectedError:  errors.New("domain cache error"),
		},
		{
			name: "active-active domain with lookup error",
			setupMocks: func(ctrl *gomock.Controller) (cache.DomainCache, activecluster.Manager) {
				mockDomainCache := cache.NewMockDomainCache(ctrl)
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				domainEntry := cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: testDomainID},
					&persistence.DomainConfig{},
					true,
					&persistence.DomainReplicationConfig{
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: currentCluster},
							{ClusterName: remoteCluster},
						},
						ActiveClusters: &types.ActiveClusters{
							ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
								"region1": {ActiveClusterName: currentCluster},
								"region2": {ActiveClusterName: remoteCluster},
							},
						},
					},
					0,
					nil,
					0,
					0,
					0,
				)
				mockDomainCache.EXPECT().
					GetDomainByID(testDomainID).
					Return(domainEntry, nil)
				mockActiveClusterMgr.EXPECT().
					LookupWorkflow(gomock.Any(), testDomainID, testWorkflowID, testRunID).
					Return(nil, errors.New("lookup error"))

				return mockDomainCache, mockActiveClusterMgr
			},
			taskInfo: &persistence.DecisionTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
			},
			expectedResult: "",
			expectedError:  errors.New("lookup error"),
		},
		{
			name: "active-active domain becomes active",
			setupMocks: func(ctrl *gomock.Controller) (cache.DomainCache, activecluster.Manager) {
				mockDomainCache := cache.NewMockDomainCache(ctrl)
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				domainEntry := cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: testDomainID},
					&persistence.DomainConfig{},
					true,
					&persistence.DomainReplicationConfig{
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: currentCluster},
							{ClusterName: remoteCluster},
						},
						ActiveClusters: &types.ActiveClusters{
							ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
								"region1": {ActiveClusterName: currentCluster},
								"region2": {ActiveClusterName: remoteCluster},
							},
						},
					},
					0,
					nil,
					0,
					0,
					0,
				)
				mockDomainCache.EXPECT().
					GetDomainByID(testDomainID).
					Return(domainEntry, nil)
				mockActiveClusterMgr.EXPECT().
					LookupWorkflow(gomock.Any(), testDomainID, testWorkflowID, testRunID).
					Return(&activecluster.LookupResult{
						ClusterName: currentCluster,
					}, nil)

				return mockDomainCache, mockActiveClusterMgr
			},
			taskInfo: &persistence.DecisionTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
			},
			expectedResult: "",
			expectedError:  errors.New("domain becomes active when processing task as standby"),
		},
		{
			name: "active-active domain successful lookup",
			setupMocks: func(ctrl *gomock.Controller) (cache.DomainCache, activecluster.Manager) {
				mockDomainCache := cache.NewMockDomainCache(ctrl)
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				domainEntry := cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: testDomainID},
					&persistence.DomainConfig{},
					true,
					&persistence.DomainReplicationConfig{
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: currentCluster},
							{ClusterName: remoteCluster},
						},
						ActiveClusters: &types.ActiveClusters{
							ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
								"region1": {ActiveClusterName: currentCluster},
								"region2": {ActiveClusterName: remoteCluster},
							},
						},
					},
					0,
					nil,
					0,
					0,
					0,
				)
				mockDomainCache.EXPECT().
					GetDomainByID(testDomainID).
					Return(domainEntry, nil)
				mockActiveClusterMgr.EXPECT().
					LookupWorkflow(gomock.Any(), testDomainID, testWorkflowID, testRunID).
					Return(&activecluster.LookupResult{
						ClusterName: remoteCluster,
					}, nil)

				return mockDomainCache, mockActiveClusterMgr
			},
			taskInfo: &persistence.DecisionTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
			},
			expectedResult: remoteCluster,
			expectedError:  nil,
		},
		{
			name: "non-active-active domain becomes active",
			setupMocks: func(ctrl *gomock.Controller) (cache.DomainCache, activecluster.Manager) {
				mockDomainCache := cache.NewMockDomainCache(ctrl)
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				domainEntry := cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: testDomainID},
					&persistence.DomainConfig{},
					false,
					&persistence.DomainReplicationConfig{
						ActiveClusterName: currentCluster,
					},
					0,
					nil,
					0,
					0,
					0,
				)
				mockDomainCache.EXPECT().
					GetDomainByID(testDomainID).
					Return(domainEntry, nil)

				return mockDomainCache, mockActiveClusterMgr
			},
			taskInfo: &persistence.DecisionTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
			},
			expectedResult: "",
			expectedError:  errors.New("domain becomes active when processing task as standby"),
		},
		{
			name: "non-active-active domain successful lookup",
			setupMocks: func(ctrl *gomock.Controller) (cache.DomainCache, activecluster.Manager) {
				mockDomainCache := cache.NewMockDomainCache(ctrl)
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				domainEntry := cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: testDomainID},
					&persistence.DomainConfig{},
					false,
					&persistence.DomainReplicationConfig{
						ActiveClusterName: remoteCluster,
					},
					0,
					nil,
					0,
					0,
					0,
				)
				mockDomainCache.EXPECT().
					GetDomainByID(testDomainID).
					Return(domainEntry, nil)

				return mockDomainCache, mockActiveClusterMgr
			},
			taskInfo: &persistence.DecisionTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
			},
			expectedResult: remoteCluster,
			expectedError:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDomainCache, mockActiveClusterMgr := tt.setupMocks(ctrl)

			result, err := getRemoteClusterName(
				context.Background(),
				currentCluster,
				mockDomainCache,
				mockActiveClusterMgr,
				tt.taskInfo,
			)

			if tt.expectedError != nil {
				assert.ErrorContains(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
