package task

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/activecluster"
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
		setupMocks     func(*gomock.Controller) activecluster.Manager
		taskInfo       persistence.Task
		expectedResult string
		expectedError  error
	}{
		{
			name: "active-active domain with lookup error",
			setupMocks: func(ctrl *gomock.Controller) activecluster.Manager {
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				mockActiveClusterMgr.EXPECT().
					GetActiveClusterInfoByWorkflow(gomock.Any(), testDomainID, testWorkflowID, testRunID).
					Return(nil, errors.New("lookup error"))

				return mockActiveClusterMgr
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
			setupMocks: func(ctrl *gomock.Controller) activecluster.Manager {
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				mockActiveClusterMgr.EXPECT().
					GetActiveClusterInfoByWorkflow(gomock.Any(), testDomainID, testWorkflowID, testRunID).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: currentCluster,
					}, nil)

				return mockActiveClusterMgr
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
			setupMocks: func(ctrl *gomock.Controller) activecluster.Manager {
				mockActiveClusterMgr := activecluster.NewMockManager(ctrl)

				mockActiveClusterMgr.EXPECT().
					GetActiveClusterInfoByWorkflow(gomock.Any(), testDomainID, testWorkflowID, testRunID).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: remoteCluster,
					}, nil)

				return mockActiveClusterMgr
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

			mockActiveClusterMgr := tt.setupMocks(ctrl)

			result, err := getRemoteClusterName(
				context.Background(),
				currentCluster,
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
