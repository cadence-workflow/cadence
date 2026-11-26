package engineimpl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	cc "github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	test "github.com/uber/cadence/service/history/testing"
)

func TestQueryWorkflowEmitsAgeDays(t *testing.T) {
	const metricName = "workflow_query_age_days"

	tests := map[string]struct {
		complete       func(execution.MutableState, int64)
		closedAgo      time.Duration
		matchingErr    error
		expectMetric   bool
		expectedStatus string
		expectedBucket float64
	}{
		"closed completed workflow": {
			complete:       func(ms execution.MutableState, id int64) { test.AddCompleteWorkflowEvent(ms, id, nil) },
			closedAgo:      71 * time.Hour,
			expectMetric:   true,
			expectedStatus: "success",
			expectedBucket: 3,
		},
		"closed failed workflow": {
			complete:       func(ms execution.MutableState, id int64) { test.AddFailWorkflowEvent(ms, id, "reason", nil) },
			closedAgo:      23 * time.Hour,
			expectMetric:   true,
			expectedStatus: "failure",
			expectedBucket: 1,
		},
		"open workflow": {},
		"closed workflow but query fails": {
			complete:    func(ms execution.MutableState, id int64) { test.AddCompleteWorkflowEvent(ms, id, nil) },
			closedAgo:   71 * time.Hour,
			matchingErr: errors.New("matching failed"),
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockShard := shard.NewTestContext(t, ctrl, &persistence.ShardInfo{RangeID: 1}, config.NewForTest())
			defer mockShard.Finish(t)

			domainCache := mockShard.Resource.DomainCache
			mockShard.SetEventsCache(events.NewCache(mockShard.GetShardID(), mockShard.GetHistoryManager(), mockShard.GetConfig(), mockShard.GetLogger(), mockShard.GetMetricsClient(), domainCache))
			domainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestLocalDomainEntry, nil).AnyTimes()
			domainCache.EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).AnyTimes()
			mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&types.ActiveClusterInfo{
				ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
				FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
			}, nil).AnyTimes()

			engine := &historyEngineImpl{
				currentClusterName:   mockShard.GetClusterMetadata().GetCurrentClusterName(),
				shard:                mockShard,
				timeSource:           mockShard.GetTimeSource(),
				clusterMetadata:      mockShard.Resource.ClusterMetadata,
				executionManager:     mockShard.Resource.ExecutionMgr,
				executionCache:       execution.NewCache(mockShard),
				logger:               mockShard.GetLogger(),
				metricsClient:        mockShard.GetMetricsClient(),
				config:               config.NewForTest(),
				activeClusterManager: mockShard.Resource.ActiveClusterMgr,
				matchingClient:       mockShard.Resource.MatchingClient,
				clientChecker:        cc.NewVersionChecker(),
			}

			workflowExecution := types.WorkflowExecution{WorkflowID: "query-age-wf", RunID: constants.TestRunID}
			tasklist, identity := "testTaskList", "testIdentity"
			msBuilder := execution.NewMutableStateBuilderWithEventV2(mockShard, testlogger.New(t), workflowExecution.GetRunID(), constants.TestLocalDomainEntry)
			test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
			di := test.AddDecisionTaskScheduledEvent(msBuilder)
			started := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
			completed := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID, started.ID, nil, identity)
			if td.complete != nil {
				td.complete(msBuilder, completed.ID)
			}

			ms := execution.CreatePersistenceMutableState(t, msBuilder)
			if td.complete != nil {
				// The completion event is read from here, which lets the test control the close time.
				ms.ExecutionInfo.CompletionEvent = &types.HistoryEvent{Timestamp: new(time.Now().Add(-td.closedAgo).UnixNano())}
			}
			mockShard.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: ms}, nil)
			if td.matchingErr != nil {
				mockShard.Resource.MatchingClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(nil, td.matchingErr)
			} else {
				mockShard.Resource.MatchingClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(&types.MatchingQueryWorkflowResponse{QueryResult: []byte{1}}, nil)
			}

			_, err := engine.QueryWorkflow(context.Background(), &types.HistoryQueryWorkflowRequest{
				DomainUUID: constants.TestDomainID,
				Request: &types.QueryWorkflowRequest{
					Domain:                constants.TestDomainName,
					Execution:             &workflowExecution,
					Query:                 &types.WorkflowQuery{},
					QueryConsistencyLevel: types.QueryConsistencyLevelEventual.Ptr(),
				},
			})
			if td.matchingErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			found := false
			for _, h := range mockShard.Resource.MetricsScope.Snapshot().Histograms() {
				if h.Name() != "test."+metricName { // the shared test scope prefixes names with "test."
					continue
				}
				found = true
				assert.Equal(t, "QueryWorkflow", h.Tags()["operation"])
				assert.Equal(t, td.expectedStatus, h.Tags()["workflow_close_status"])
				assert.Equal(t, constants.TestDomainName, h.Tags()["domain"])
				assert.Equal(t, int64(1), h.Values()[td.expectedBucket], "sample should land in bucket %v", td.expectedBucket)
			}
			assert.Equal(t, td.expectMetric, found)
		})
	}
}
