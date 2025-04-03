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

package timers

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/workflow"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/collection"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/reconciliation/store"
	"github.com/uber/cadence/service/worker/scanner/shardscanner"
)

type timersWorkflowsSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	controller *gomock.Controller
}

func TestScannerWorkflowSuite(t *testing.T) {
	suite.Run(t, new(timersWorkflowsSuite))
}

func (s *timersWorkflowsSuite) SetupSuite() {
	workflow.Register(ScannerWorkflow)
	s.controller = gomock.NewController(s.T())

}
func (s *timersWorkflowsSuite) TestScannerWorkflow_SetsHooks() {
	dcClient := dynamicconfig.NewMockClient(s.controller)
	logger := log.NewNoop()

	dc := collection.NewCollection(dcClient, logger)
	cfg := ScannerConfig(dc)
	s.Equal(ScannerWFTypeName, cfg.ScannerWFTypeName, "scanner wf type name is set")
	s.NotNil(cfg.FixerHooks)
	s.NotNil(cfg.ScannerHooks)
}
func (s *timersWorkflowsSuite) TestScannerWorkflow_Success() {
	env := s.NewTestWorkflowEnvironment()
	cconfig := shardscanner.CustomScannerConfig{
		periodStartKey: "1",
		periodEndKey:   "2",
	}
	env.OnActivity(shardscanner.ActivityScannerConfig, mock.Anything, mock.Anything).Return(shardscanner.ResolvedScannerWorkflowConfig{
		GenericScannerConfig: shardscanner.GenericScannerConfig{
			Enabled:           true,
			Concurrency:       3,
			ActivityBatchSize: 5,
		},
		CustomScannerConfig: cconfig,
	}, nil)
	env.OnActivity(shardscanner.ActivityScannerEmitMetrics, mock.Anything, mock.Anything).Return(nil)
	shards := shardscanner.Shards{
		Range: &shardscanner.ShardRange{
			Min: 0,
			Max: 30,
		},
	}

	batches := [][]int{
		{0, 3, 6, 9, 12},
		{15, 18, 21, 24, 27},
		{1, 4, 7, 10, 13},
		{16, 19, 22, 25, 28},
		{2, 5, 8, 11, 14},
		{17, 20, 23, 26, 29},
	}

	for _, batch := range batches {
		var reports []shardscanner.ScanReport
		for i := range batch {
			if i == 0 {
				reports = append(reports, shardscanner.ScanReport{
					ShardID: batch[i],
					Stats: shardscanner.ScanStats{
						EntitiesCount: 10,
					},
					Result: shardscanner.ScanResult{
						ControlFlowFailure: &shardscanner.ControlFlowFailure{
							Info: "got control flow failure",
						},
					},
				})
			} else {
				reports = append(reports, shardscanner.ScanReport{
					ShardID: batch[i],
					Stats: shardscanner.ScanStats{
						EntitiesCount:    10,
						CorruptedCount:   2,
						CheckFailedCount: 1,
						CorruptionByType: map[invariant.Name]int64{},
					},
					Result: shardscanner.ScanResult{
						ShardScanKeys: &shardscanner.ScanKeys{
							Corrupt: &store.Keys{
								UUID:    "test_uuid",
								MinPage: 0,
								MaxPage: 10,
							},
						},
					},
				})
			}
		}
		// var customc shardscanner.CustomScannerConfig
		env.OnActivity(shardscanner.ActivityScanShard, mock.Anything, shardscanner.ScanShardActivityParams{
			Shards:        batch,
			ScannerConfig: cconfig,
		}).Return(reports, nil)
	}

	env.ExecuteWorkflow(ScannerWorkflow, shardscanner.ScannerWorkflowParams{
		Shards: shards,
	})
	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	aggValue, err := env.QueryWorkflow(shardscanner.AggregateReportQuery)
	s.NoError(err)
	var agg shardscanner.AggregateScanReportResult
	s.NoError(aggValue.Get(&agg))
	s.Equal(shardscanner.AggregateScanReportResult{
		EntitiesCount:    240,
		CorruptedCount:   48,
		CheckFailedCount: 24,
		CorruptionByType: map[invariant.Name]int64{},
	}, agg)

	for i := 0; i < 30; i++ {
		shardReportValue, err := env.QueryWorkflow(shardscanner.ShardReportQuery, i)
		s.NoError(err)
		var shardReport *shardscanner.ScanReport
		s.NoError(shardReportValue.Get(&shardReport))
		if i == 0 || i == 1 || i == 2 || i == 15 || i == 16 || i == 17 {
			s.Equal(&shardscanner.ScanReport{
				ShardID: i,
				Stats: shardscanner.ScanStats{
					EntitiesCount: 10,
				},
				Result: shardscanner.ScanResult{
					ControlFlowFailure: &shardscanner.ControlFlowFailure{
						Info: "got control flow failure",
					},
				},
			}, shardReport)
		} else {
			s.Equal(&shardscanner.ScanReport{
				ShardID: i,
				Stats: shardscanner.ScanStats{
					EntitiesCount:    10,
					CorruptedCount:   2,
					CheckFailedCount: 1,
					CorruptionByType: map[invariant.Name]int64{},
				},
				Result: shardscanner.ScanResult{
					ShardScanKeys: &shardscanner.ScanKeys{
						Corrupt: &store.Keys{
							UUID:    "test_uuid",
							MinPage: 0,
							MaxPage: 10,
						},
					},
				},
			}, shardReport)
		}
	}

	statusValue, err := env.QueryWorkflow(shardscanner.ShardStatusQuery, shardscanner.PaginatedShardQueryRequest{})
	s.NoError(err)
	var status *shardscanner.ShardStatusQueryResult
	s.NoError(statusValue.Get(&status))
	expected := make(map[int]shardscanner.ShardStatus)
	for i := 0; i < 30; i++ {
		if i == 0 || i == 1 || i == 2 || i == 15 || i == 16 || i == 17 {
			expected[i] = shardscanner.ShardStatusControlFlowFailure
		} else {
			expected[i] = shardscanner.ShardStatusSuccess
		}
	}
	s.Equal(shardscanner.ShardStatusResult(expected), status.Result)
	s.True(status.ShardQueryPaginationToken.IsDone)
	s.Nil(status.ShardQueryPaginationToken.NextShardID)

	// check for paginated query result
	statusValue, err = env.QueryWorkflow(shardscanner.ShardStatusQuery, shardscanner.PaginatedShardQueryRequest{
		StartingShardID: common.IntPtr(5),
		LimitShards:     common.IntPtr(10),
	})
	s.NoError(err)
	status = &shardscanner.ShardStatusQueryResult{}
	s.NoError(statusValue.Get(&status))
	expected = make(map[int]shardscanner.ShardStatus)
	for i := 5; i < 15; i++ {
		if i == 0 || i == 1 || i == 2 || i == 15 || i == 16 || i == 17 {
			expected[i] = shardscanner.ShardStatusControlFlowFailure
		} else {
			expected[i] = shardscanner.ShardStatusSuccess
		}
	}
	s.Equal(shardscanner.ShardStatusResult(expected), status.Result)
	s.False(status.ShardQueryPaginationToken.IsDone)
	s.Equal(15, *status.ShardQueryPaginationToken.NextShardID)

	corruptionKeysValue, err := env.QueryWorkflow(shardscanner.ShardCorruptKeysQuery, shardscanner.PaginatedShardQueryRequest{})
	s.NoError(err)
	var shardCorruptKeysResult *shardscanner.ShardCorruptKeysQueryResult
	s.NoError(corruptionKeysValue.Get(&shardCorruptKeysResult))
	expectedCorrupted := make(map[int]store.Keys)
	for i := 0; i < 30; i++ {
		if i != 0 && i != 1 && i != 2 && i != 15 && i != 16 && i != 17 {
			expectedCorrupted[i] = store.Keys{
				UUID:    "test_uuid",
				MinPage: 0,
				MaxPage: 10,
			}
		}
	}
	s.Equal(shardscanner.ShardCorruptKeysResult(expectedCorrupted), shardCorruptKeysResult.Result)
}
