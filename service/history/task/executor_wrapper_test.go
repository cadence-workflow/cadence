package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestExecutorWrapper_IsActiveTask(t *testing.T) {
	tests := []struct {
		name           string
		currentCluster string
		domainID       string
		workflowID     string
		runID          string
		domainError    error
		isActiveActive bool
		activeCluster  string
		lookupError    error
		expectedResult bool
	}{
		{
			name:           "Domain not found - process as active",
			currentCluster: "cluster1",
			domainID:       "domain1",
			workflowID:     "workflow1",
			runID:          "run1",
			domainError:    assert.AnError,
			expectedResult: true,
		},
		{
			name:           "Active-Active domain - current cluster is active",
			currentCluster: "cluster1",
			domainID:       "domain1",
			workflowID:     "workflow1",
			runID:          "run1",
			isActiveActive: true,
			activeCluster:  "cluster1",
			expectedResult: true,
		},
		{
			name:           "Active-Active domain - current cluster is not active",
			currentCluster: "cluster1",
			domainID:       "domain1",
			workflowID:     "workflow1",
			runID:          "run1",
			isActiveActive: true,
			activeCluster:  "cluster2",
			expectedResult: false,
		},
		{
			name:           "Active-Active domain - lookup error",
			currentCluster: "cluster1",
			domainID:       "domain1",
			workflowID:     "workflow1",
			runID:          "run1",
			isActiveActive: true,
			lookupError:    assert.AnError,
			expectedResult: true,
		},
		{
			name:           "Non-Active-Active domain - current cluster is active",
			currentCluster: "cluster1",
			activeCluster:  "cluster1",
			domainID:       "domain1",
			workflowID:     "workflow1",
			runID:          "run1",
			isActiveActive: false,
			expectedResult: true,
		},
		{
			name:           "Non-Active-Active domain - current cluster is not active",
			currentCluster: "cluster1",
			activeCluster:  "cluster2",
			domainID:       "domain1",
			workflowID:     "workflow1",
			runID:          "run1",
			isActiveActive: false,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// Setup mocks
			mockTask := NewMockTask(ctrl)
			mockTask.EXPECT().GetDomainID().Return(tt.domainID).AnyTimes()
			mockTask.EXPECT().GetWorkflowID().Return(tt.workflowID).AnyTimes()
			mockTask.EXPECT().GetRunID().Return(tt.runID).AnyTimes()
			mockTask.EXPECT().GetInfo().Return(&persistence.DecisionTask{}).AnyTimes()
			mockTask.EXPECT().GetTaskType().Return(0).AnyTimes() // called by debug log

			mockDomainCache := cache.NewMockDomainCache(ctrl)
			mockActiveClusterMgr := activecluster.NewMockManager(ctrl)
			if tt.isActiveActive {
				domainEntry := cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: tt.domainID},
					&persistence.DomainConfig{},
					true,
					&persistence.DomainReplicationConfig{
						ActiveClusters: &types.ActiveClusters{
							ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
								"region1": {ActiveClusterName: tt.activeCluster},
							},
						},
					},
					0,
					nil,
					0,
					0,
					0,
				)
				mockDomainCache.EXPECT().GetDomainByID(tt.domainID).Return(domainEntry, tt.domainError)
				if tt.lookupError == nil {
					mockActiveClusterMgr.EXPECT().LookupWorkflow(gomock.Any(), tt.domainID, tt.workflowID, tt.runID).
						Return(&activecluster.LookupResult{ClusterName: tt.activeCluster}, nil)
				} else {
					mockActiveClusterMgr.EXPECT().LookupWorkflow(gomock.Any(), tt.domainID, tt.workflowID, tt.runID).
						Return(nil, tt.lookupError)
				}
			} else {
				domainEntry := cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: tt.domainID},
					&persistence.DomainConfig{},
					true,
					&persistence.DomainReplicationConfig{
						ActiveClusterName: tt.activeCluster,
						Clusters: []*persistence.ClusterReplicationConfig{
							{ClusterName: tt.currentCluster},
							{ClusterName: tt.activeCluster},
						},
					},
					0,
					nil,
					0,
					0,
					0,
				)
				mockDomainCache.EXPECT().GetDomainByID(tt.domainID).Return(domainEntry, tt.domainError)
			}

			mockLogger := testlogger.New(t)

			// Create executor wrapper
			wrapper := NewExecutorWrapper(
				tt.currentCluster,
				mockDomainCache,
				mockActiveClusterMgr,
				NewMockExecutor(ctrl),
				NewMockExecutor(ctrl),
				mockLogger,
			)

			// Execute test
			result := wrapper.(*executorWrapper).isActiveTask(mockTask)

			// Verify result
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
