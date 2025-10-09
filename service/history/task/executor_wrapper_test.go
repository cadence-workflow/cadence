package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/activecluster"
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

			mockActiveClusterMgr := activecluster.NewMockManager(ctrl)
			if tt.lookupError == nil {
				mockActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), tt.domainID, tt.workflowID, tt.runID).
					Return(&types.ActiveClusterInfo{ActiveClusterName: tt.activeCluster}, nil)
			} else {
				mockActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), tt.domainID, tt.workflowID, tt.runID).
					Return(nil, tt.lookupError)
			}

			mockLogger := testlogger.New(t)

			// Create executor wrapper
			wrapper := NewExecutorWrapper(
				tt.currentCluster,
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
