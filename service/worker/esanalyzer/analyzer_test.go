// Copyright (c) 2017-2021 Uber Technologies Inc.
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
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABxILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package esanalyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/elasticsearch"
	esMocks "github.com/uber/cadence/common/elasticsearch/mocks"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/service/history/resource"
)

type esanalyzerWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	activityEnv        *testsuite.TestActivityEnvironment
	workflowEnv        *testsuite.TestWorkflowEnvironment
	controller         *gomock.Controller
	resource           *resource.Test
	mockAdminClient    *admin.MockClient
	mockDomainCache    *cache.MockDomainCache
	clientBean         *client.MockBean
	logger             *log.MockLogger
	mockMetricClient   *mocks.Client
	scopedMetricClient *mocks.Scope
	mockESClient       *esMocks.GenericClient
	tallyScope         tally.Scope
	analyzer           *Analyzer
	workflow           *Workflow
	config             Config
	DomainID           string
	DomainName         string
	WorkflowType       string
	WorkflowID         string
	RunID              string
	WorkflowVersion    string
}

func TestESAnalyzerWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(esanalyzerWorkflowTestSuite))
}

func (s *esanalyzerWorkflowTestSuite) SetupTest() {
	s.DomainID = "deadbeef-0123-4567-890a-bcdef0123460"
	s.DomainName = "test-domain"
	s.WorkflowType = "test-workflow-type"
	s.WorkflowID = "test-workflow_id"
	s.RunID = "test-run_id"
	s.WorkflowVersion = "test-workflow-version"
	s.tallyScope = tally.NoopScope
	activeDomainCache := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: s.DomainID, Name: s.DomainName},
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

	s.config = Config{
		ESAnalyzerPause:                          dynamicconfig.GetBoolPropertyFn(false),
		ESAnalyzerTimeWindow:                     dynamicconfig.GetDurationPropertyFn(time.Hour * 24 * 30),
		ESAnalyzerMaxNumDomains:                  dynamicconfig.GetIntPropertyFn(500),
		ESAnalyzerMaxNumWorkflowTypes:            dynamicconfig.GetIntPropertyFn(100),
		ESAnalyzerLimitToTypes:                   dynamicconfig.GetStringPropertyFn(""),
		ESAnalyzerLimitToDomains:                 dynamicconfig.GetStringPropertyFn(""),
		ESAnalyzerNumWorkflowsToRefresh:          dynamicconfig.GetIntPropertyFilteredByWorkflowType(2),
		ESAnalyzerBufferWaitTime:                 dynamicconfig.GetDurationPropertyFilteredByWorkflowType(time.Minute * 30),
		ESAnalyzerMinNumWorkflowsForAvg:          dynamicconfig.GetIntPropertyFilteredByWorkflowType(100),
		ESAnalyzerWorkflowDurationWarnThresholds: dynamicconfig.GetStringPropertyFn(""),
	}

	s.activityEnv = s.NewTestActivityEnvironment()
	s.workflowEnv = s.NewTestWorkflowEnvironment()
	s.controller = gomock.NewController(s.T())
	s.mockDomainCache = cache.NewMockDomainCache(s.controller)
	s.resource = resource.NewTest(s.T(), s.controller, metrics.Worker)
	s.mockAdminClient = admin.NewMockClient(s.controller)
	s.clientBean = client.NewMockBean(s.controller)
	s.logger = &log.MockLogger{}
	s.mockMetricClient = &mocks.Client{}
	s.scopedMetricClient = &mocks.Scope{}
	s.mockESClient = &esMocks.GenericClient{}

	//
	// s.mockDomainCache.EXPECT().GetDomainByID(s.DomainID).Return(activeDomainCache, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomain(s.DomainName).Return(activeDomainCache, nil).AnyTimes()

	// SET UP ANALYZER
	s.analyzer = &Analyzer{
		svcClient:   s.resource.GetSDKClient(),
		clientBean:  s.clientBean,
		domainCache: s.mockDomainCache,
		logger:      s.logger,
		esClient:    s.mockESClient,
		config:      &s.config,
		tallyScope:  s.tallyScope,
	}
	s.activityEnv.SetTestTimeout(time.Second * 5)
	s.activityEnv.SetWorkerOptions(worker.Options{BackgroundActivityContext: context.Background()})

	// REGISTER WORKFLOWS AND ACTIVITIES
	s.workflow = &Workflow{analyzer: s.analyzer}
	s.workflowEnv.RegisterWorkflowWithOptions(
		s.workflow.workflowFunc,
		workflow.RegisterOptions{Name: esanalyzerWFTypeName})
	s.workflowEnv.RegisterActivityWithOptions(
		s.workflow.emitWorkflowVersionMetrics,
		activity.RegisterOptions{Name: emitWorkflowVersionMetricsActivity},
	)
	s.activityEnv.RegisterActivityWithOptions(
		s.workflow.emitWorkflowVersionMetrics,
		activity.RegisterOptions{Name: emitWorkflowVersionMetricsActivity},
	)

	s.workflowEnv.RegisterWorkflowWithOptions(
		s.workflow.emitWorkflowTypeCount,
		workflow.RegisterOptions{Name: domainWFTypeCountWorkflowTypeName})
	s.activityEnv.RegisterActivityWithOptions(
		s.workflow.emitWorkflowTypeCountMetrics,
		activity.RegisterOptions{Name: emitDomainWorkflowTypeCountMetricsActivity},
	)
}

func (s *esanalyzerWorkflowTestSuite) TearDownTest() {
	defer s.controller.Finish()
	defer s.resource.Finish(s.T())

	s.workflowEnv.AssertExpectations(s.T())
}

func (s *esanalyzerWorkflowTestSuite) TestExecuteWorkflow() {
	s.workflowEnv.OnActivity(emitWorkflowVersionMetricsActivity, mock.Anything).Return(nil).Times(1)

	s.workflowEnv.ExecuteWorkflow(esanalyzerWFTypeName, mock.Anything)
	err := s.workflowEnv.GetWorkflowResult(nil)
	s.NoError(err)
}

func (s *esanalyzerWorkflowTestSuite) TestEmitWorkflowVersionMetricsActivity() {
	s.config.ESAnalyzerWorkflowVersionDomains = dynamicconfig.GetStringPropertyFn(
		fmt.Sprintf(`["%s"]`, s.DomainName),
	)
	esRaw := `
{
  "took": 642,
  "timed_out": false,
  "_shards": {
    "total": 20,
    "successful": 20,
    "skipped": 0,
    "failed": 0
  },
  "hits": {
    "total": 4588,
    "max_score": 0,
    "hits": [

    ]
  },
  "aggregations": {
    "wftypes": {
      "doc_count_error_upper_bound": 0,
      "sum_other_doc_count": 0,
      "buckets": [
        {
          "key": "CourierUpdateWorkflow",
          "doc_count": 4539,
          "versions": {
            "doc_count_error_upper_bound": 0,
            "sum_other_doc_count": 0,
            "buckets": [

            ]
          }
        },
        {
          "key": "UpdateTipWorkflow",
          "doc_count": 33,
          "versions": {
            "doc_count_error_upper_bound": 0,
            "sum_other_doc_count": 0,
            "buckets": [
              {
                "key": "update tip efp flow-1",
                "doc_count": 3
              }
            ]
          }
        },
        {
          "key": "RoboCourierWorkflow",
          "doc_count": 12,
          "versions": {
            "doc_count_error_upper_bound": 0,
            "sum_other_doc_count": 0,
            "buckets": [

            ]
          }
        },
        {
          "key": "ImproveDropoffPinWorkflow",
          "doc_count": 2,
          "versions": {
            "doc_count_error_upper_bound": 0,
            "sum_other_doc_count": 0,
            "buckets": [

            ]
          }
        },
        {
          "key": "OrderStateUpdatedWorkflow",
          "doc_count": 2,
          "versions": {
            "doc_count_error_upper_bound": 0,
            "sum_other_doc_count": 0,
            "buckets": [

            ]
          }
        }
      ]
    }
  }
}
	`
	var rawEs elasticsearch.RawResponse
	err := json.Unmarshal([]byte(esRaw), &rawEs)
	s.NoError(err)
	s.mockESClient.On("SearchRaw", mock.Anything, mock.Anything, mock.Anything).Return(
		&rawEs, nil).Times(1)
	_, err = s.activityEnv.ExecuteActivity(s.workflow.emitWorkflowVersionMetrics)
	s.NoError(err)

}

func (s *esanalyzerWorkflowTestSuite) TestEmitWorkflowTypeCountMetricsActivity() {
	s.config.ESAnalyzerWorkflowTypeDomains = dynamicconfig.GetStringPropertyFn(
		fmt.Sprintf(`["%s"]`, s.DomainName),
	)
	esRaw := `
{
  "took": 642,
  "timed_out": false,
  "_shards": {
    "total": 20,
    "successful": 20,
    "skipped": 0,
    "failed": 0
  },
  "hits": {
    "total": 4588,
    "max_score": 0,
    "hits": [

    ]
  },
  "aggregations": {
    "wftypes": {
      "doc_count_error_upper_bound": 0,
      "sum_other_doc_count": 0,
      "buckets": [
        {
          "key": "CourierUpdateWorkflow",
          "doc_count": 4539
        },
        {
          "key": "UpdateTipWorkflow",
          "doc_count": 33
        },
        {
          "key": "RoboCourierWorkflow",
          "doc_count": 12
        },
        {
          "key": "ImproveDropoffPinWorkflow",
          "doc_count": 2
        },
        {
          "key": "OrderStateUpdatedWorkflow",
          "doc_count": 2
        }
      ]
    }
  }
}
	`
	var rawEs elasticsearch.RawResponse
	err := json.Unmarshal([]byte(esRaw), &rawEs)
	s.NoError(err)
	s.mockESClient.On("SearchRaw", mock.Anything, mock.Anything, mock.Anything).Return(
		&rawEs, nil).Times(1)
	_, err = s.activityEnv.ExecuteActivity(s.workflow.emitWorkflowTypeCountMetrics)
	s.NoError(err)

}

func TestNewAnalyzer(t *testing.T) {
	mockESConfig := &config.ElasticSearchConfig{
		Indices: map[string]string{
			common.VisibilityAppName: "test",
		},
	}
	mockPinotConfig := &config.PinotVisibilityConfig{
		Table: "test",
	}

	mockESClient := &esMocks.GenericClient{}
	testAnalyzer1 := New(nil, nil, nil, mockESClient, nil, mockESConfig, mockPinotConfig, nil, nil, nil, nil, nil)

	mockPinotClient := &pinot.MockGenericClient{}
	testAnalyzer2 := New(nil, nil, nil, nil, mockPinotClient, mockESConfig, mockPinotConfig, nil, nil, nil, nil, nil)

	assert.Equal(t, testAnalyzer1.readMode, ES)
	assert.Equal(t, testAnalyzer2.readMode, Pinot)
}

func TestEmitWorkflowTypeCountMetricsESErrorCases(t *testing.T) {
	mockESConfig := &config.ElasticSearchConfig{
		Indices: map[string]string{
			common.VisibilityAppName: "test",
		},
	}
	mockPinotConfig := &config.PinotVisibilityConfig{
		Table: "test",
	}

	ctrl := gomock.NewController(t)
	mockESClient := &esMocks.GenericClient{}
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	testAnalyzer := New(nil, nil, nil, mockESClient, nil, mockESConfig, mockPinotConfig, log.NewNoop(), tally.NoopScope, nil, mockDomainCache, nil)
	testWorkflow := &Workflow{analyzer: testAnalyzer}

	tests := map[string]struct {
		domainCacheAffordance func(mockDomainCache *cache.MockDomainCache)
		ESClientAffordance    func(mockESClient *esMocks.GenericClient)
		expectedErr           error
	}{
		"Case1: error getting domain": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			ESClientAffordance: func(mockESClient *esMocks.GenericClient) {},
			expectedErr:        fmt.Errorf("error"),
		},
		"Case2: error ES searchRaw": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			ESClientAffordance: func(mockESClient *esMocks.GenericClient) {
				mockESClient.On("SearchRaw", mock.Anything, mock.Anything, mock.Anything).Return(
					nil, fmt.Errorf("error")).Times(1)
			},
			expectedErr: fmt.Errorf("error"),
		},
		"Case3: foundAggregation is false": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			ESClientAffordance: func(mockESClient *esMocks.GenericClient) {
				mockESClient.On("SearchRaw", mock.Anything, mock.Anything, mock.Anything).Return(
					&elasticsearch.RawResponse{
						Aggregations: map[string]json.RawMessage{},
					}, nil).Times(1)
			},
			expectedErr: fmt.Errorf("aggregation failed for domain in ES: test-domain"),
		},
		"Case4: error unmarshalling aggregation": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			ESClientAffordance: func(mockESClient *esMocks.GenericClient) {
				mockESClient.On("SearchRaw", mock.Anything, mock.Anything, mock.Anything).Return(
					&elasticsearch.RawResponse{
						Aggregations: map[string]json.RawMessage{
							"wftypes": []byte("invalid"),
						},
					}, nil).Times(1)
			},
			expectedErr: fmt.Errorf("invalid character 'i' looking for beginning of value"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up mocks
			test.domainCacheAffordance(mockDomainCache)
			test.ESClientAffordance(mockESClient)

			err := testWorkflow.emitWorkflowTypeCountMetricsES(context.Background(), "test-domain", zap.NewNop())
			if err == nil {
				assert.Equal(t, test.expectedErr, err)
			} else {
				assert.Equal(t, test.expectedErr.Error(), err.Error())
			}
		})
	}
}

func TestEmitWorkflowVersionMetricsESErrorCases(t *testing.T) {
	mockESConfig := &config.ElasticSearchConfig{
		Indices: map[string]string{
			common.VisibilityAppName: "test",
		},
	}
	mockPinotConfig := &config.PinotVisibilityConfig{
		Table: "test",
	}

	ctrl := gomock.NewController(t)
	mockESClient := &esMocks.GenericClient{}
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	testAnalyzer := New(nil, nil, nil, mockESClient, nil, mockESConfig, mockPinotConfig, log.NewNoop(), tally.NoopScope, nil, mockDomainCache, nil)
	testWorkflow := &Workflow{analyzer: testAnalyzer}

	tests := map[string]struct {
		domainCacheAffordance func(mockDomainCache *cache.MockDomainCache)
		ESClientAffordance    func(mockESClient *esMocks.GenericClient)
		expectedErr           error
	}{
		"Case1: error getting domain": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			ESClientAffordance: func(mockESClient *esMocks.GenericClient) {},
			expectedErr:        fmt.Errorf("error"),
		},
		"Case2: error ES searchRaw": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			ESClientAffordance: func(mockESClient *esMocks.GenericClient) {
				mockESClient.On("SearchRaw", mock.Anything, mock.Anything, mock.Anything).Return(
					nil, fmt.Errorf("error")).Times(1)
			},
			expectedErr: fmt.Errorf("error"),
		},
		"Case3: foundAggregation is false": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			ESClientAffordance: func(mockESClient *esMocks.GenericClient) {
				mockESClient.On("SearchRaw", mock.Anything, mock.Anything, mock.Anything).Return(
					&elasticsearch.RawResponse{
						Aggregations: map[string]json.RawMessage{},
					}, nil).Times(1)
			},
			expectedErr: fmt.Errorf("aggregation failed for domain in ES: test-domain"),
		},
		"Case4: error unmarshalling aggregation": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			ESClientAffordance: func(mockESClient *esMocks.GenericClient) {
				mockESClient.On("SearchRaw", mock.Anything, mock.Anything, mock.Anything).Return(
					&elasticsearch.RawResponse{
						Aggregations: map[string]json.RawMessage{
							"wftypes": []byte("invalid"),
						},
					}, nil).Times(1)
			},
			expectedErr: fmt.Errorf("invalid character 'i' looking for beginning of value"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up mocks
			test.domainCacheAffordance(mockDomainCache)
			test.ESClientAffordance(mockESClient)

			err := testWorkflow.emitWorkflowVersionMetricsES(context.Background(), "test-domain", zap.NewNop())
			if err == nil {
				assert.Equal(t, test.expectedErr, err)
			} else {
				assert.Equal(t, test.expectedErr.Error(), err.Error())
			}
		})
	}
}

func TestEmitWorkflowTypeCountMetricsPinot(t *testing.T) {
	mockPinotConfig := &config.PinotVisibilityConfig{
		Table: "test",
	}
	mockESConfig := &config.ElasticSearchConfig{
		Indices: map[string]string{
			common.VisibilityAppName: "test",
		},
	}

	ctrl := gomock.NewController(t)

	mockPinotClient := pinot.NewMockGenericClient(ctrl)
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	testAnalyzer := New(nil, nil, nil, nil, mockPinotClient, mockESConfig, mockPinotConfig, log.NewNoop(), tally.NoopScope, nil, mockDomainCache, nil)
	testWorkflow := &Workflow{analyzer: testAnalyzer}

	tests := map[string]struct {
		domainCacheAffordance func(mockDomainCache *cache.MockDomainCache)
		PinotClientAffordance func(mockPinotClient *pinot.MockGenericClient)
		expectedErr           error
	}{
		"Case0: success": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{
					{"test0", float64(1)},
					{"test1", float64(2)},
				}, nil).Times(1)
			},
			expectedErr: nil,
		},
		"Case1: error getting domain": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(nil, fmt.Errorf("domain error")).Times(1)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {},
			expectedErr:           fmt.Errorf("domain error"),
		},
		"Case2: error Pinot query": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return(nil, fmt.Errorf("pinot error")).Times(1)
			},
			expectedErr: fmt.Errorf("pinot error"),
		},
		"Case3: Aggregation is empty": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{}, nil).Times(1)
			},
			expectedErr: fmt.Errorf("aggregation failed for domain in Pinot: test-domain"),
		},
		"Case4: error parsing workflow count": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{
					{"test", "invalid"},
				}, nil).Times(1)
			},
			expectedErr: fmt.Errorf("error parsing workflow count for workflow type test"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up mocks
			test.domainCacheAffordance(mockDomainCache)
			test.PinotClientAffordance(mockPinotClient)

			err := testWorkflow.emitWorkflowTypeCountMetricsPinot("test-domain", zap.NewNop())
			if err == nil {
				assert.Equal(t, test.expectedErr, err)
			} else {
				assert.Equal(t, test.expectedErr.Error(), err.Error())
			}
		})
	}
}

func TestEmitWorkflowVersionMetricsPinot(t *testing.T) {
	mockPinotConfig := &config.PinotVisibilityConfig{
		Table: "test",
	}
	mockESConfig := &config.ElasticSearchConfig{
		Indices: map[string]string{
			common.VisibilityAppName: "test",
		},
	}

	ctrl := gomock.NewController(t)

	mockPinotClient := pinot.NewMockGenericClient(ctrl)
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	testAnalyzer := New(nil, nil, nil, nil, mockPinotClient, mockESConfig, mockPinotConfig, log.NewNoop(), tally.NoopScope, nil, mockDomainCache, nil)
	testWorkflow := &Workflow{analyzer: testAnalyzer}

	tests := map[string]struct {
		domainCacheAffordance func(mockDomainCache *cache.MockDomainCache)
		PinotClientAffordance func(mockPinotClient *pinot.MockGenericClient)
		expectedErr           error
	}{
		"Case0: success": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil).Times(3)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{
					{"test-wf-type0", float64(100)},
					{"test-wf-type1", float64(200)},
				}, nil).Times(1)
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{
					{"test-wf-version0", float64(10)},
					{"test-wf-version1", float64(20)},
				}, nil).Times(1)
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{
					{"test-wf-version3", float64(10)},
					{"test-wf-version4", float64(2)},
				}, nil).Times(1)
			},
			expectedErr: nil,
		},
		"Case1: error getting domain": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(nil, fmt.Errorf("domain error")).Times(1)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {},
			expectedErr:           fmt.Errorf("domain error"),
		},
		"Case2: error Pinot query": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return(nil, fmt.Errorf("pinot error")).Times(1)
			},
			expectedErr: fmt.Errorf("failed to query Pinot to find workflow type count Info: test-domain, error: pinot error"),
		},
		"Case3: Aggregation is empty": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{}, nil).Times(1)
			},
			expectedErr: fmt.Errorf("aggregation failed for domain in Pinot: test-domain"),
		},
		"Case4: error parsing workflow count": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{
					{"test", "invalid"},
				}, nil).Times(1)
			},
			expectedErr: fmt.Errorf("error parsing workflow count for workflow type test"),
		},
		"Case5-1: failure case in queryWorkflowVersionsWithType: getWorkflowVersionPinotQuery error": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil).Times(1)
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(nil, fmt.Errorf("domain error")).Times(1)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{
					{"test-wf-type0", float64(100)},
					{"test-wf-type1", float64(200)},
				}, nil).Times(1)
			},
			expectedErr: fmt.Errorf("error querying workflow versions for workflow type: test-wf-type0: error: domain error"),
		},
		"Case5-2: failure case in queryWorkflowVersionsWithType: SearchAggr error": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil).Times(2)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{
					{"test-wf-type0", float64(100)},
					{"test-wf-type1", float64(200)},
				}, nil).Times(1)
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return(nil, fmt.Errorf("pinot error")).Times(1)
			},
			expectedErr: fmt.Errorf("error querying workflow versions for workflow type: test-wf-type0: error: pinot error"),
		},
		"Case5-3: failure case in queryWorkflowVersionsWithType: error parsing workflow count": {
			domainCacheAffordance: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-id"}, nil, false, nil, 0, nil, 0, 0, 0), nil).Times(2)
			},
			PinotClientAffordance: func(mockPinotClient *pinot.MockGenericClient) {
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{
					{"test-wf-type0", float64(100)},
					{"test-wf-type1", float64(200)},
				}, nil).Times(1)
				mockPinotClient.EXPECT().SearchAggr(gomock.Any()).Return([][]interface{}{
					{"test-wf-version0", float64(3.14)},
					{"test-wf-version1", 20},
				}, nil).Times(1)
			},
			expectedErr: fmt.Errorf("error querying workflow versions for workflow type: " +
				"test-wf-type0: error: error parsing workflow count for cadence version test-wf-version1"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up mocks
			test.domainCacheAffordance(mockDomainCache)
			test.PinotClientAffordance(mockPinotClient)

			err := testWorkflow.emitWorkflowVersionMetricsPinot("test-domain", zap.NewNop())
			if err == nil {
				assert.Equal(t, test.expectedErr, err)
			} else {
				assert.Equal(t, test.expectedErr.Error(), err.Error())
			}
		})
	}
}
