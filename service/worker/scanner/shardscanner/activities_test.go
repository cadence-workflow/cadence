// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

package shardscanner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/cadence/.gen/go/shared"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/worker"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/blobstore"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/pagination"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/reconciliation/store"
	"github.com/uber/cadence/common/resource"
)

const testWorkflowName = "default-test-workflow-type-name"

type activitiesSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	controller   *gomock.Controller
	mockResource *resource.Test
}

func TestActivitiesSuite(t *testing.T) {
	suite.Run(t, new(activitiesSuite))
}

func (s *activitiesSuite) SetupSuite() {
	activity.Register(scannerConfigActivity)
	activity.Register(fixerCorruptedKeysActivity)
	activity.Register(scanShardActivity)
	activity.Register(fixShardActivity)
}

func (s *activitiesSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.mockResource = resource.NewTest(s.T(), s.controller, metrics.Worker)
	domainCache := cache.NewMockDomainCache(s.controller)
	domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil).AnyTimes()
	s.mockResource.DomainCache = domainCache
}

func (s *activitiesSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *activitiesSuite) TestScanShardActivity() {

	testCases := []struct {
		params       ScanShardActivityParams
		wantErr      bool
		managerHook  func(ctx context.Context, pr persistence.Retryer, params ScanShardActivityParams, cache cache.DomainCache) invariant.Manager
		itHook       func(ctx context.Context, pr persistence.Retryer, params ScanShardActivityParams) pagination.Iterator
		workflowName string
	}{
		{
			params: ScanShardActivityParams{
				Shards: []int{0},
			},
			managerHook: func(ctx context.Context, pr persistence.Retryer, params ScanShardActivityParams, cache cache.DomainCache) invariant.Manager {
				manager := invariant.NewMockManager(s.controller)
				manager.EXPECT().RunChecks(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(
						invariant.ManagerCheckResult{CheckResultType: invariant.CheckResultTypeHealthy},
					)
				return manager
			},
			itHook: func(ctx context.Context, pr persistence.Retryer, params ScanShardActivityParams) pagination.Iterator {
				it := pagination.NewMockIterator(s.controller)
				calls := 0
				it.EXPECT().HasNext().DoAndReturn(
					func() bool {
						if calls > 1 {
							return false
						}
						calls++
						return true
					},
				).AnyTimes()
				it.EXPECT().Next().Return(&entity.ConcreteExecution{}, nil).Times(2)
				return it
			},
			workflowName: testWorkflowName,
			wantErr:      false,
		},
		{
			params: ScanShardActivityParams{
				Shards: []int{0},
			},
			workflowName: "wrong-test-name",
			wantErr:      true,
		},
	}

	for _, tc := range testCases {

		env := s.NewTestActivityEnvironment()
		hooks, _ := NewScannerHooks(tc.managerHook, tc.itHook, func(scanner ScannerContext) CustomScannerConfig {
			return nil // no config overrides
		})
		sc := NewShardScannerContext(s.mockResource, &ScannerConfig{
			ScannerHooks: func() *ScannerHooks { return hooks },
		})
		ctx := NewScannerContext(context.Background(), tc.workflowName, sc)
		env.SetWorkerOptions(worker.Options{
			BackgroundActivityContext: ctx,
		})
		report, err := env.ExecuteActivity(scanShardActivity, tc.params)
		if tc.wantErr {
			s.Error(err)
			break
		} else {
			s.NoError(err)
		}
		var reports []ScanReport
		s.NoError(report.Get(&reports))

		for _, v := range s.mockResource.MetricsScope.Snapshot().Timers() {
			tags := v.Tags()
			s.Equal("cadence-sys-shardscanner-scanshard-activity", tags["activityType"])
			s.Equal(tc.workflowName, tags["workflowType"])
		}

	}
}

func (s *activitiesSuite) TestFixShardActivity() {

	testCases := []struct {
		name        string
		params      FixShardActivityParams
		wantErr     bool
		managerHook FixerManagerCB
		itHook      FixerIteratorCB
	}{
		{
			name:    "run fixer",
			wantErr: false,
			params: FixShardActivityParams{
				CorruptedKeysEntries: []CorruptedKeysEntry{
					{ShardID: 1, CorruptedKeys: struct {
						UUID      string
						MinPage   int
						MaxPage   int
						Extension store.Extension
					}{UUID: "test-uuid", MinPage: 0, MaxPage: 1, Extension: "test-ext"}},
				},
				ResolvedFixerWorkflowConfig: ResolvedFixerWorkflowConfig{},
			},
			managerHook: func(ctx context.Context, pr persistence.Retryer, p FixShardActivityParams, cache cache.DomainCache) invariant.Manager {
				manager := invariant.NewMockManager(s.controller)
				manager.EXPECT().RunFixes(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(
						invariant.ManagerFixResult{FixResultType: invariant.FixResultTypeFixed},
					)
				return manager
			},
			itHook: func(ctx context.Context, client blobstore.Client, k store.Keys, params FixShardActivityParams) store.ScanOutputIterator {
				it := store.NewMockScanOutputIterator(s.controller)
				calls := 0
				it.EXPECT().HasNext().DoAndReturn(
					func() bool {
						if calls > 1 {
							return false
						}
						calls++
						return true
					},
				).AnyTimes()
				it.EXPECT().Next().Return(&store.ScanOutputEntity{
					Execution: &entity.ConcreteExecution{
						Execution: entity.Execution{
							DomainID: "test_domain",
						},
					},
				}, nil).AnyTimes()
				return it
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.mockResource.BlobstoreClient.
				EXPECT().
				Put(gomock.Any(), gomock.Any()).
				Return(&blobstore.PutResponse{}, nil).
				Times(2)
			domainCache := cache.NewMockDomainCache(s.controller)
			domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil).AnyTimes()
			s.mockResource.DomainCache = domainCache
			cfg := &ScannerConfig{
				DynamicParams: DynamicParams{
					AllowDomain: dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
				},
			}
			if tc.itHook != nil && tc.managerHook != nil {
				cfg.FixerHooks = func() *FixerHooks {
					return &FixerHooks{
						InvariantManager: tc.managerHook,
						Iterator:         tc.itHook,
					}
				}
			}
			fc := NewShardFixerContext(s.mockResource, cfg)

			env := s.NewTestActivityEnvironment()

			env.SetWorkerOptions(worker.Options{
				BackgroundActivityContext: NewFixerContext(context.Background(), testWorkflowName, fc),
			})
			report, execErr := env.ExecuteActivity(fixShardActivity, tc.params)
			if tc.wantErr {
				s.Error(execErr)
			} else {
				var reports []FixReport
				s.NoError(report.Get(&reports))
			}
		})
	}

}

func (s *activitiesSuite) TestScannerConfigActivity() {
	testCases := []struct {
		dynamicParams *DynamicParams
		params        ScannerConfigActivityParams
		resolved      ResolvedScannerWorkflowConfig
		addHook       bool
	}{
		{
			dynamicParams: &DynamicParams{
				ScannerEnabled:          dynamicproperties.GetBoolPropertyFn(true),
				Concurrency:             dynamicproperties.GetIntPropertyFn(10),
				PageSize:                dynamicproperties.GetIntPropertyFn(100),
				ActivityBatchSize:       dynamicproperties.GetIntPropertyFn(10),
				BlobstoreFlushThreshold: dynamicproperties.GetIntPropertyFn(1000),
			},
			params: ScannerConfigActivityParams{
				Overwrites: ScannerWorkflowConfigOverwrites{},
			},
			addHook: true,
			resolved: ResolvedScannerWorkflowConfig{
				GenericScannerConfig: GenericScannerConfig{
					Enabled:                 true,
					Concurrency:             10,
					ActivityBatchSize:       10,
					PageSize:                100,
					BlobstoreFlushThreshold: 1000,
				},
				CustomScannerConfig: CustomScannerConfig{
					"test-key": "test-value",
				},
			},
		},
		{
			dynamicParams: &DynamicParams{
				ScannerEnabled:          dynamicproperties.GetBoolPropertyFn(true),
				Concurrency:             dynamicproperties.GetIntPropertyFn(10),
				PageSize:                dynamicproperties.GetIntPropertyFn(100),
				ActivityBatchSize:       dynamicproperties.GetIntPropertyFn(10),
				BlobstoreFlushThreshold: dynamicproperties.GetIntPropertyFn(1000),
			},
			params: ScannerConfigActivityParams{
				Overwrites: ScannerWorkflowConfigOverwrites{},
			},
			resolved: ResolvedScannerWorkflowConfig{
				GenericScannerConfig: GenericScannerConfig{
					Enabled:                 true,
					Concurrency:             10,
					ActivityBatchSize:       10,
					PageSize:                100,
					BlobstoreFlushThreshold: 1000,
				},
			},
		},
		{
			dynamicParams: &DynamicParams{
				ScannerEnabled:          dynamicproperties.GetBoolPropertyFn(true),
				Concurrency:             dynamicproperties.GetIntPropertyFn(10),
				ActivityBatchSize:       dynamicproperties.GetIntPropertyFn(100),
				PageSize:                dynamicproperties.GetIntPropertyFn(100),
				BlobstoreFlushThreshold: dynamicproperties.GetIntPropertyFn(1000),
			},
			params: ScannerConfigActivityParams{
				Overwrites: ScannerWorkflowConfigOverwrites{
					GenericScannerConfig: GenericScannerConfigOverwrites{
						Enabled:                 common.BoolPtr(false),
						ActivityBatchSize:       common.IntPtr(1),
						BlobstoreFlushThreshold: common.IntPtr(100),
					},
					CustomScannerConfig: &CustomScannerConfig{
						"test": "test",
					},
				},
			},
			resolved: ResolvedScannerWorkflowConfig{
				GenericScannerConfig: GenericScannerConfig{
					Enabled:                 false,
					Concurrency:             10,
					ActivityBatchSize:       1,
					PageSize:                100,
					BlobstoreFlushThreshold: 100,
				},
				CustomScannerConfig: CustomScannerConfig{
					"test": "test",
				},
			},
		},
	}

	for _, tc := range testCases {
		env := s.NewTestActivityEnvironment()

		cfg := &ScannerConfig{
			DynamicParams: *tc.dynamicParams,
			ScannerHooks: func() *ScannerHooks {
				return &ScannerHooks{}
			},
		}
		if tc.addHook {
			cfg.ScannerHooks = func() *ScannerHooks {
				return &ScannerHooks{
					GetScannerConfig: func(scanner ScannerContext) CustomScannerConfig {
						return map[string]string{"test-key": "test-value"}
					},
				}
			}
		}

		env.SetWorkerOptions(worker.Options{
			BackgroundActivityContext: NewScannerContext(
				context.Background(),
				testWorkflowName,
				NewShardScannerContext(s.mockResource, cfg),
			),
		},
		)

		resolvedValue, err := env.ExecuteActivity(scannerConfigActivity, tc.params)
		s.NoError(err)
		var resolved ResolvedScannerWorkflowConfig
		s.NoError(resolvedValue.Get(&resolved))
		s.Equal(tc.resolved, resolved)
	}
}

func (s *activitiesSuite) TestFixerCorruptedKeysActivity() {
	s.mockResource.SDKClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&shared.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &shared.WorkflowExecutionInfo{
			CloseStatus: shared.WorkflowExecutionCloseStatusCompleted.Ptr(),
		},
	}, nil)
	queryResult := &ShardCorruptKeysQueryResult{
		Result: map[int]store.Keys{
			1: {UUID: "first"},
			2: {UUID: "second"},
			3: {UUID: "third"},
		},
		ShardQueryPaginationToken: ShardQueryPaginationToken{
			NextShardID: common.IntPtr(4),
			IsDone:      false,
		},
	}
	queryResultData, err := json.Marshal(queryResult)
	response := &shared.ListClosedWorkflowExecutionsResponse{
		Executions: []*shared.WorkflowExecutionInfo{
			{
				CloseStatus: shared.WorkflowExecutionCloseStatusCompleted.Ptr(),
			},
			{
				Execution: &shared.WorkflowExecution{
					WorkflowId: common.StringPtr("test-list-workflow-id"),
					RunId:      common.StringPtr(uuid.New()),
				},
				Type: &shared.WorkflowType{
					Name: common.StringPtr("test-list-workflow-type"),
				},
				StartTime:     common.Int64Ptr(time.Now().UnixNano()),
				CloseTime:     common.Int64Ptr(time.Now().Add(time.Hour).UnixNano()),
				CloseStatus:   shared.WorkflowExecutionCloseStatusContinuedAsNew.Ptr(),
				HistoryLength: common.Int64Ptr(12),
			},
		},
	}
	s.NoError(err)
	s.mockResource.SDKClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(response, nil)

	s.mockResource.SDKClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(&shared.QueryWorkflowResponse{
		QueryResult: queryResultData,
	}, nil)
	env := s.getFixerActivityEnvironment()
	fixerResultValue, err := env.ExecuteActivity(fixerCorruptedKeysActivity, FixerCorruptedKeysActivityParams{})
	s.NoError(err)
	fixerResult := &FixerCorruptedKeysActivityResult{}
	s.NoError(fixerResultValue.Get(&fixerResult))
	s.Equal(1, *fixerResult.MinShard)
	s.Equal(3, *fixerResult.MaxShard)
	s.Equal(ShardQueryPaginationToken{
		NextShardID: common.IntPtr(4),
		IsDone:      false,
	}, fixerResult.ShardQueryPaginationToken)
	s.Contains(fixerResult.CorruptedKeys, CorruptedKeysEntry{
		ShardID: 1,
		CorruptedKeys: store.Keys{
			UUID: "first",
		},
	})
	s.Contains(fixerResult.CorruptedKeys, CorruptedKeysEntry{
		ShardID: 2,
		CorruptedKeys: store.Keys{
			UUID: "second",
		},
	})
	s.Contains(fixerResult.CorruptedKeys, CorruptedKeysEntry{
		ShardID: 3,
		CorruptedKeys: store.Keys{
			UUID: "third",
		},
	})
}

func (s *activitiesSuite) TestFixerCorruptedKeysActivity_Fails_WhenNoSuitableExecutionsAreFound() {
	response := &shared.ListClosedWorkflowExecutionsResponse{
		Executions: []*shared.WorkflowExecutionInfo{
			{
				CloseStatus: shared.WorkflowExecutionCloseStatusCompleted.Ptr(),
			},
			{
				CloseStatus: shared.WorkflowExecutionCloseStatusCanceled.Ptr(),
			},
			{
				CloseStatus: shared.WorkflowExecutionCloseStatusTimedOut.Ptr(),
			},
			{
				CloseStatus: shared.WorkflowExecutionCloseStatusTerminated.Ptr(),
			},
			{
				CloseStatus: shared.WorkflowExecutionCloseStatusFailed.Ptr(),
			},
		},
	}
	s.mockResource.SDKClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(response, nil)

	env := s.getFixerActivityEnvironment()
	fixerResultValue, err := env.ExecuteActivity(fixerCorruptedKeysActivity, FixerCorruptedKeysActivityParams{})
	s.Nil(fixerResultValue)
	s.EqualError(err, "failed to find a recent scanner workflow execution with ContinuedAsNew status")
}

func (s *activitiesSuite) getFixerActivityEnvironment() *testsuite.TestActivityEnvironment {
	env := s.NewTestActivityEnvironment()
	cfg := &ScannerConfig{
		DynamicParams: DynamicParams{
			AllowDomain: dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
		},
		FixerHooks: func() *FixerHooks {
			return &FixerHooks{}
		},
	}
	fc := NewShardFixerContext(s.mockResource, cfg)
	env.SetWorkerOptions(worker.Options{
		BackgroundActivityContext: NewFixerContext(context.Background(), testWorkflowName, fc),
	})
	return env
}
