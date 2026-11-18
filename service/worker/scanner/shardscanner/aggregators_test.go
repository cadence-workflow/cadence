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
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	c "github.com/uber/cadence/common"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/reconciliation/store"
)

type aggregatorsSuite struct {
	suite.Suite
}

func TestAggregatorSuite(t *testing.T) {
	suite.Run(t, new(aggregatorsSuite))
}

func (s *aggregatorsSuite) TestShardScanResultAggregator() {
	agg := NewShardScanResultAggregator([]int{1, 2, 3}, 1, 3)
	expected := &ShardScanResultAggregator{
		minShard: 1,
		maxShard: 3,
		reports:  map[int]ScanReport{},
		status: map[int]ShardStatus{
			1: ShardStatusRunning,
			2: ShardStatusRunning,
			3: ShardStatusRunning,
		},
		aggregation: AggregateScanReportResult{
			CorruptionByType: make(map[invariant.Name]int64),
		},
		corruptionKeys: make(map[int]store.Keys),
		statusSummary: map[ShardStatus]int{
			ShardStatusRunning:            3,
			ShardStatusControlFlowFailure: 0,
			ShardStatusSuccess:            0,
		},
		shardSizes: nil,
	}
	s.Equal(expected, agg)
	report, err := agg.GetReport(1)
	s.Nil(report)
	s.Equal("shard 1 has not finished yet, check back later for report", err.Error())
	report, err = agg.GetReport(5)
	s.Nil(report)
	s.Equal("shard 5 is not included in shards which will be processed", err.Error())
	firstReport := ScanReport{
		ShardID: 1,
		Stats: ScanStats{
			EntitiesCount:    10,
			CorruptedCount:   3,
			CheckFailedCount: 1,
			CorruptionByType: map[invariant.Name]int64{
				invariant.HistoryExists:        2,
				invariant.OpenCurrentExecution: 1,
			},
		},
		Result: ScanResult{
			ShardScanKeys: &ScanKeys{
				Corrupt: &store.Keys{
					UUID: "test_uuid",
				},
			},
		},
	}
	agg.AddReport(firstReport)
	expected.status[1] = ShardStatusSuccess
	expected.statusSummary[ShardStatusRunning] = 2
	expected.statusSummary[ShardStatusSuccess] = 1
	expected.reports[1] = firstReport
	expected.shardSizes = []ShardSizeTuple{
		{
			ShardID:       1,
			EntitiesCount: 10,
		},
	}
	expected.aggregation.EntitiesCount = 10
	expected.aggregation.CorruptedCount = 3
	expected.aggregation.CheckFailedCount = 1
	expected.aggregation.CorruptionByType = map[invariant.Name]int64{
		invariant.HistoryExists:        2,
		invariant.OpenCurrentExecution: 1,
	}
	expected.corruptionKeys = map[int]store.Keys{
		1: {
			UUID: "test_uuid",
		},
	}
	s.Equal(expected, agg)
	report, err = agg.GetReport(1)
	s.NoError(err)
	s.Equal(firstReport, *report)
	secondReport := ScanReport{
		ShardID: 2,
		Stats: ScanStats{
			EntitiesCount:    10,
			CorruptedCount:   3,
			CheckFailedCount: 1,
			CorruptionByType: map[invariant.Name]int64{
				invariant.HistoryExists:        2,
				invariant.OpenCurrentExecution: 1,
			},
		},
		Result: ScanResult{
			ControlFlowFailure: &ControlFlowFailure{},
		},
	}
	agg.AddReport(secondReport)
	expected.status[2] = ShardStatusControlFlowFailure
	expected.statusSummary[ShardStatusRunning] = 1
	expected.statusSummary[ShardStatusControlFlowFailure] = 1
	expected.reports[2] = secondReport
	expected.shardSizes = []ShardSizeTuple{
		{
			ShardID:       1,
			EntitiesCount: 10,
		},
	}
	s.Equal(expected, agg)
	shardStatus, err := agg.GetStatusResult(PaginatedShardQueryRequest{
		StartingShardID: c.IntPtr(1),
		LimitShards:     c.IntPtr(2),
	})
	s.NoError(err)
	s.Equal(&ShardStatusQueryResult{
		Result: map[int]ShardStatus{
			1: ShardStatusSuccess,
			2: ShardStatusControlFlowFailure,
		},
		ShardQueryPaginationToken: ShardQueryPaginationToken{
			NextShardID: c.IntPtr(3),
			IsDone:      false,
		},
	}, shardStatus)
	corruptedKeys, err := agg.GetCorruptionKeys(PaginatedShardQueryRequest{
		StartingShardID: c.IntPtr(1),
		LimitShards:     c.IntPtr(3),
	})
	s.NoError(err)
	s.Equal(&ShardCorruptKeysQueryResult{
		Result: map[int]store.Keys{
			1: {
				UUID: "test_uuid",
			},
		},
		ShardQueryPaginationToken: ShardQueryPaginationToken{
			NextShardID: nil,
			IsDone:      true,
		},
	}, corruptedKeys)

}

func (s *aggregatorsSuite) TestShardFixResultAggregator() {
	agg := NewShardFixResultAggregator([]CorruptedKeysEntry{{ShardID: 1}, {ShardID: 2}, {ShardID: 3}}, 1, 3)
	expected := &ShardFixResultAggregator{
		minShard: 1,
		maxShard: 3,
		reports:  map[int]FixReport{},
		status: map[int]ShardStatus{
			1: ShardStatusRunning,
			2: ShardStatusRunning,
			3: ShardStatusRunning,
		},
		statusSummary: map[ShardStatus]int{
			ShardStatusRunning:            3,
			ShardStatusControlFlowFailure: 0,
			ShardStatusSuccess:            0,
		},
		aggregation: AggregateFixReportResult{},
	}
	s.Equal(expected, agg)
	report, err := agg.GetReport(1)
	s.Nil(report)
	s.Equal("shard 1 has not finished yet, check back later for report", err.Error())
	report, err = agg.GetReport(5)
	s.Nil(report)
	s.Equal("shard 5 is not included in shards which will be processed", err.Error())
	firstReport := FixReport{
		ShardID: 1,
		Stats: FixStats{
			EntitiesCount: 10,
			FixedCount:    3,
			FailedCount:   1,
		},
		Result: FixResult{
			ShardFixKeys: &FixKeys{
				Fixed: &store.Keys{
					UUID: "test_uuid",
				},
			},
		},
	}
	agg.AddReport(firstReport)
	expected.status[1] = ShardStatusSuccess
	expected.statusSummary[ShardStatusSuccess] = 1
	expected.statusSummary[ShardStatusRunning] = 2
	expected.reports[1] = firstReport
	expected.aggregation.EntitiesCount = 10
	expected.aggregation.FixedCount = 3
	expected.aggregation.FailedCount = 1
	s.Equal(expected, agg)
	report, err = agg.GetReport(1)
	s.NoError(err)
	s.Equal(firstReport, *report)
	secondReport := FixReport{
		ShardID: 2,
		Stats: FixStats{
			EntitiesCount: 10,
			FixedCount:    3,
			FailedCount:   1,
		},
		Result: FixResult{
			ControlFlowFailure: &ControlFlowFailure{},
		},
	}
	agg.AddReport(secondReport)
	expected.status[2] = ShardStatusControlFlowFailure
	expected.statusSummary[ShardStatusControlFlowFailure] = 1
	expected.statusSummary[ShardStatusRunning] = 1
	expected.reports[2] = secondReport
	s.Equal(expected, agg)
	shardStatus, err := agg.GetStatusResult(PaginatedShardQueryRequest{
		StartingShardID: c.IntPtr(1),
		LimitShards:     c.IntPtr(2),
	})
	s.NoError(err)
	s.Equal(&ShardStatusQueryResult{
		Result: map[int]ShardStatus{
			1: ShardStatusSuccess,
			2: ShardStatusControlFlowFailure,
		},
		ShardQueryPaginationToken: ShardQueryPaginationToken{
			NextShardID: c.IntPtr(3),
			IsDone:      false,
		},
	}, shardStatus)
}

func (s *aggregatorsSuite) TestGetStatusResult() {
	testCases := []struct {
		minShardID     int
		maxShardID     int
		req            PaginatedShardQueryRequest
		status         ShardStatusResult
		expectedResult *ShardStatusQueryResult
		expectedError  bool
	}{
		{
			minShardID: 0,
			maxShardID: 5,
			req: PaginatedShardQueryRequest{
				StartingShardID: c.IntPtr(6),
			},
			expectedResult: nil,
			expectedError:  true,
		},
		{
			minShardID: 0,
			maxShardID: 5,
			req: PaginatedShardQueryRequest{
				StartingShardID: c.IntPtr(0),
				LimitShards:     c.IntPtr(10),
			},
			status: map[int]ShardStatus{
				1: ShardStatusRunning,
				2: ShardStatusRunning,
				3: ShardStatusSuccess,
				4: ShardStatusSuccess,
				5: ShardStatusControlFlowFailure,
			},
			expectedResult: &ShardStatusQueryResult{
				Result: map[int]ShardStatus{
					1: ShardStatusRunning,
					2: ShardStatusRunning,
					3: ShardStatusSuccess,
					4: ShardStatusSuccess,
					5: ShardStatusControlFlowFailure,
				},
				ShardQueryPaginationToken: ShardQueryPaginationToken{
					NextShardID: nil,
					IsDone:      true,
				},
			},
			expectedError: false,
		},
		{
			minShardID: 0,
			maxShardID: 5,
			req: PaginatedShardQueryRequest{
				StartingShardID: c.IntPtr(0),
				LimitShards:     c.IntPtr(2),
			},
			status: map[int]ShardStatus{
				1: ShardStatusRunning,
				2: ShardStatusRunning,
				3: ShardStatusSuccess,
				4: ShardStatusSuccess,
				5: ShardStatusControlFlowFailure,
			},
			expectedResult: &ShardStatusQueryResult{
				Result: map[int]ShardStatus{
					1: ShardStatusRunning,
					2: ShardStatusRunning,
				},
				ShardQueryPaginationToken: ShardQueryPaginationToken{
					NextShardID: c.IntPtr(3),
					IsDone:      false,
				},
			},
			expectedError: false,
		},
		{
			minShardID: 0,
			maxShardID: 5,
			req: PaginatedShardQueryRequest{
				StartingShardID: c.IntPtr(0),
				LimitShards:     c.IntPtr(3),
			},
			status: map[int]ShardStatus{
				1: ShardStatusRunning,
				2: ShardStatusRunning,
				4: ShardStatusSuccess,
				5: ShardStatusControlFlowFailure,
			},
			expectedResult: &ShardStatusQueryResult{
				Result: map[int]ShardStatus{
					1: ShardStatusRunning,
					2: ShardStatusRunning,
					4: ShardStatusSuccess,
				},
				ShardQueryPaginationToken: ShardQueryPaginationToken{
					NextShardID: c.IntPtr(5),
					IsDone:      false,
				},
			},
			expectedError: false,
		},
		{
			minShardID: 0,
			maxShardID: 5,
			req: PaginatedShardQueryRequest{
				StartingShardID: c.IntPtr(2),
				LimitShards:     c.IntPtr(3),
			},
			status: map[int]ShardStatus{
				1: ShardStatusRunning,
				2: ShardStatusRunning,
				4: ShardStatusSuccess,
				5: ShardStatusControlFlowFailure,
			},
			expectedResult: &ShardStatusQueryResult{
				Result: map[int]ShardStatus{
					2: ShardStatusRunning,
					4: ShardStatusSuccess,
					5: ShardStatusControlFlowFailure,
				},
				ShardQueryPaginationToken: ShardQueryPaginationToken{
					NextShardID: nil,
					IsDone:      true,
				},
			},
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		result, err := getStatusResult(tc.minShardID, tc.maxShardID, tc.req, tc.status)
		s.Equal(tc.expectedResult, result)
		if tc.expectedError {
			s.Error(err)
		} else {
			s.NoError(err)
		}
	}
}

func (s *aggregatorsSuite) TestGetShardSizeQueryResult() {
	testCases := []struct {
		shardSizes       []ShardSizeTuple
		req              ShardSizeQueryRequest
		expectedErrorStr *string
		expectedResult   ShardSizeQueryResult
	}{
		{
			shardSizes: nil,
			req: ShardSizeQueryRequest{
				StartIndex: -1,
			},
			expectedErrorStr: c.StringPtr("index out of bounds exception (required startIndex >= 0 && startIndex < endIndex && endIndex <= 0)"),
			expectedResult:   nil,
		},
		{
			shardSizes: nil,
			req: ShardSizeQueryRequest{
				StartIndex: 1,
				EndIndex:   1,
			},
			expectedErrorStr: c.StringPtr("index out of bounds exception (required startIndex >= 0 && startIndex < endIndex && endIndex <= 0)"),
			expectedResult:   nil,
		},
		{
			shardSizes: nil,
			req: ShardSizeQueryRequest{
				StartIndex: 0,
				EndIndex:   1,
			},
			expectedErrorStr: c.StringPtr("index out of bounds exception (required startIndex >= 0 && startIndex < endIndex && endIndex <= 0)"),
			expectedResult:   nil,
		},
		{
			shardSizes: make([]ShardSizeTuple, 10),
			req: ShardSizeQueryRequest{
				StartIndex: 0,
				EndIndex:   11,
			},
			expectedErrorStr: c.StringPtr("index out of bounds exception (required startIndex >= 0 && startIndex < endIndex && endIndex <= 10)"),
			expectedResult:   nil,
		},
		{
			shardSizes: make([]ShardSizeTuple, 10000),
			req: ShardSizeQueryRequest{
				StartIndex: 0,
				EndIndex:   maxShardQueryResult + 1,
			},
			expectedErrorStr: c.StringPtr("too many shards requested, the limit is 1000"),
			expectedResult:   nil,
		},
		{
			shardSizes: []ShardSizeTuple{
				{
					ShardID: 1,
				},
				{
					ShardID: 2,
				},
				{
					ShardID: 3,
				},
				{
					ShardID: 4,
				},
				{
					ShardID: 5,
				},
			},
			req: ShardSizeQueryRequest{
				StartIndex: 0,
				EndIndex:   1,
			},
			expectedErrorStr: nil,
			expectedResult: []ShardSizeTuple{
				{
					ShardID: 1,
				},
			},
		},
		{
			shardSizes: []ShardSizeTuple{
				{
					ShardID: 1,
				},
				{
					ShardID: 2,
				},
				{
					ShardID: 3,
				},
				{
					ShardID: 4,
				},
				{
					ShardID: 5,
				},
			},
			req: ShardSizeQueryRequest{
				StartIndex: 0,
				EndIndex:   5,
			},
			expectedErrorStr: nil,
			expectedResult: []ShardSizeTuple{
				{
					ShardID: 1,
				},
				{
					ShardID: 2,
				},
				{
					ShardID: 3,
				},
				{
					ShardID: 4,
				},
				{
					ShardID: 5,
				},
			},
		},
	}

	for _, tc := range testCases {
		agg := &ShardScanResultAggregator{
			shardSizes: tc.shardSizes,
		}
		result, err := agg.GetShardSizeQueryResult(tc.req)
		if tc.expectedErrorStr != nil {
			s.Equal(*tc.expectedErrorStr, err.Error())
		} else {
			s.Equal(tc.expectedResult, result)
		}
	}
}

func (s *aggregatorsSuite) TestInsertReportIntoSizes() {
	randomReport := func() ScanReport {
		return ScanReport{
			ShardID: 0,
			Stats: ScanStats{
				EntitiesCount: int64(rand.Intn(10)),
			},
		}
	}
	agg := &ShardScanResultAggregator{}
	for i := 0; i < 1000; i++ {
		agg.insertReportIntoSizes(randomReport())
	}
	for i := 0; i < 999; i++ {
		s.GreaterOrEqual(agg.shardSizes[i].EntitiesCount, agg.shardSizes[i+1].EntitiesCount)
	}
}

func TestGetShardDistributionStats(t *testing.T) {
	failed := ScanResult{ControlFlowFailure: &ControlFlowFailure{}}
	tests := map[string]struct {
		reports  []ScanReport
		expected ShardDistributionStats
	}{
		"no reports":        {expected: ShardDistributionStats{}},
		"all shards failed": {reports: []ScanReport{{ShardID: 1, Result: failed}, {ShardID: 2, Result: failed}}, expected: ShardDistributionStats{}},
		"single shard": {
			reports:  []ScanReport{{ShardID: 1, Stats: ScanStats{EntitiesCount: 10}, Result: ScanResult{ShardScanKeys: &ScanKeys{}}}},
			expected: ShardDistributionStats{Max: 10, Median: 10, Min: 10, P90: 10, P75: 10, P25: 10, P10: 10},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			agg := NewShardScanResultAggregator([]int{1, 2}, 1, 2)
			for _, r := range tc.reports {
				agg.AddReport(r)
			}
			assert.Equal(t, tc.expected, agg.GetShardDistributionStats())
		})
	}
}
