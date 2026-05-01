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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/pagination"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/reconciliation/store"
)

type ScannerSuite struct {
	*require.Assertions
	suite.Suite
	controller *gomock.Controller
}

func TestScannerSuite(t *testing.T) {
	suite.Run(t, new(ScannerSuite))
}

func (s *ScannerSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.controller = gomock.NewController(s.T())
}

func (s *ScannerSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *ScannerSuite) TestScan_Failure_FirstIteratorError() {
	mockItr := pagination.NewMockIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(true).Times(1)
	mockItr.EXPECT().Next().Return(nil, errors.New("iterator error")).Times(1)
	scanner := &ShardScanner{
		shardID:          0,
		itr:              mockItr,
		progressReportFn: func() {},
	}
	result := scanner.Scan(context.Background())
	s.Equal(ScanReport{
		ShardID: 0,
		Stats: ScanStats{
			CorruptionByType: make(map[invariant.Name]int64),
		},
		Result: ScanResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "persistence iterator returned error",
				InfoDetails: "iterator error",
			},
		},
		DomainStats: make(map[string]*ScanStats),
	}, result)
}

func (s *ScannerSuite) TestScan_Failure_NonFirstError() {
	mockItr := pagination.NewMockIterator(s.controller)
	iteratorCallNumber := 0
	mockItr.EXPECT().HasNext().DoAndReturn(func() bool {
		return iteratorCallNumber < 5
	}).Times(5)
	mockItr.EXPECT().Next().DoAndReturn(func() (*entity.ConcreteExecution, error) {
		defer func() {
			iteratorCallNumber++
		}()
		if iteratorCallNumber < 4 {
			return &entity.ConcreteExecution{Execution: entity.Execution{DomainID: "ABC"}}, nil
		}
		return nil, fmt.Errorf("iterator got error on: %v", iteratorCallNumber)
	}).Times(5)
	mockInvariantManager := invariant.NewMockManager(s.controller)
	mockInvariantManager.EXPECT().RunChecks(gomock.Any(), gomock.Any()).Return(invariant.ManagerCheckResult{
		CheckResultType: invariant.CheckResultTypeHealthy,
	}).Times(4)
	domainCache := cache.NewMockDomainCache(s.controller)
	domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil).Times(4)

	scanner := &ShardScanner{
		shardID:          0,
		itr:              mockItr,
		invariantManager: mockInvariantManager,
		progressReportFn: func() {},
		domainCache:      domainCache,
		scope:            metrics.NoopScope,
	}
	result := scanner.Scan(context.Background())
	s.Equal(ScanReport{
		ShardID: 0,
		Stats: ScanStats{
			EntitiesCount:    4,
			CorruptionByType: make(map[invariant.Name]int64),
		},
		Result: ScanResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "persistence iterator returned error",
				InfoDetails: "iterator got error on: 4",
			},
		},
		DomainStats: map[string]*ScanStats{
			"ABC": &ScanStats{
				EntitiesCount:    4,
				CorruptionByType: make(map[invariant.Name]int64),
			},
		},
	}, result)
}

func (s *ScannerSuite) TestScan_Failure_CorruptedWriterError() {
	mockItr := pagination.NewMockIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(true).Times(1)
	mockItr.EXPECT().Next().Return(&entity.ConcreteExecution{Execution: entity.Execution{DomainID: "ABC"}}, nil).Times(1)
	mockInvariantManager := invariant.NewMockManager(s.controller)
	mockInvariantManager.EXPECT().RunChecks(gomock.Any(), gomock.Any()).Return(invariant.ManagerCheckResult{
		CheckResultType: invariant.CheckResultTypeCorrupted,
	}).Times(1)
	corruptedWriter := store.NewMockExecutionWriter(s.controller)
	corruptedWriter.EXPECT().Add(gomock.Any()).Return(errors.New("corrupted writer add failed")).Times(1)
	domainCache := cache.NewMockDomainCache(s.controller)
	domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil).AnyTimes()

	scanner := &ShardScanner{
		shardID:          0,
		itr:              mockItr,
		invariantManager: mockInvariantManager,
		corruptedWriter:  corruptedWriter,
		progressReportFn: func() {},
		domainCache:      domainCache,
		scope:            metrics.NoopScope,
	}
	result := scanner.Scan(context.Background())
	s.Equal(ScanReport{
		ShardID: 0,
		Stats: ScanStats{
			EntitiesCount:    1,
			CorruptionByType: make(map[invariant.Name]int64),
		},
		Result: ScanResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "blobstore add failed for corrupted execution check",
				InfoDetails: "corrupted writer add failed",
			},
		},
		DomainStats: map[string]*ScanStats{
			"ABC": &ScanStats{
				EntitiesCount:    1,
				CorruptionByType: make(map[invariant.Name]int64),
			},
		},
	}, result)
}

func (s *ScannerSuite) TestScan_Failure_FailedWriterError() {
	mockItr := pagination.NewMockIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(true).Times(1)
	mockItr.EXPECT().Next().Return(&entity.ConcreteExecution{Execution: entity.Execution{DomainID: "ABC"}}, nil).Times(1)
	mockInvariantManager := invariant.NewMockManager(s.controller)
	mockInvariantManager.EXPECT().RunChecks(gomock.Any(), gomock.Any()).Return(invariant.ManagerCheckResult{
		CheckResultType: invariant.CheckResultTypeFailed,
	}).Times(1)
	failedWriter := store.NewMockExecutionWriter(s.controller)
	failedWriter.EXPECT().Add(gomock.Any()).Return(errors.New("failed writer add failed")).Times(1)
	domainCache := cache.NewMockDomainCache(s.controller)
	domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil).AnyTimes()

	scanner := &ShardScanner{
		shardID:          0,
		itr:              mockItr,
		invariantManager: mockInvariantManager,
		failedWriter:     failedWriter,
		progressReportFn: func() {},
		domainCache:      domainCache,
		scope:            metrics.NoopScope,
	}
	result := scanner.Scan(context.Background())
	s.Equal(ScanReport{
		ShardID: 0,
		Stats: ScanStats{
			EntitiesCount:    1,
			CorruptionByType: make(map[invariant.Name]int64),
		},
		Result: ScanResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "blobstore add failed for failed execution check",
				InfoDetails: "failed writer add failed",
			},
		},
		DomainStats: map[string]*ScanStats{
			"ABC": &ScanStats{
				EntitiesCount:    1,
				CorruptionByType: make(map[invariant.Name]int64),
			},
		},
	}, result)
}

func (s *ScannerSuite) TestScan_Failure_FailedWriterFlushError() {
	mockItr := pagination.NewMockIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(false).Times(1)
	failedWriter := store.NewMockExecutionWriter(s.controller)
	failedWriter.EXPECT().Flush().Return(errors.New("failed writer flush failed")).Times(1)
	scanner := &ShardScanner{
		shardID:          0,
		itr:              mockItr,
		failedWriter:     failedWriter,
		progressReportFn: func() {},
	}
	result := scanner.Scan(context.Background())
	s.Equal(ScanReport{
		ShardID: 0,
		Stats: ScanStats{
			EntitiesCount:    0,
			CorruptionByType: make(map[invariant.Name]int64),
		},
		Result: ScanResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "failed to flush for failed execution checks",
				InfoDetails: "failed writer flush failed",
			},
		},
		DomainStats: make(map[string]*ScanStats),
	}, result)
}

func (s *ScannerSuite) TestScan_Failure_CorruptedWriterFlushError() {
	mockItr := pagination.NewMockIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(false).Times(1)
	corruptedWriter := store.NewMockExecutionWriter(s.controller)
	corruptedWriter.EXPECT().Flush().Return(errors.New("corrupted writer flush failed")).Times(1)
	failedWriter := store.NewMockExecutionWriter(s.controller)
	failedWriter.EXPECT().Flush().Return(nil).Times(1)
	scanner := &ShardScanner{
		shardID:          0,
		itr:              mockItr,
		corruptedWriter:  corruptedWriter,
		failedWriter:     failedWriter,
		progressReportFn: func() {},
	}
	result := scanner.Scan(context.Background())
	s.Equal(ScanReport{
		ShardID: 0,
		Stats: ScanStats{
			EntitiesCount:    0,
			CorruptionByType: make(map[invariant.Name]int64),
		},
		Result: ScanResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "failed to flush for corrupted execution checks",
				InfoDetails: "corrupted writer flush failed",
			},
		},
		DomainStats: make(map[string]*ScanStats),
	}, result)
}

func (s *ScannerSuite) TestScan_Success() {
	mockItr := pagination.NewMockIterator(s.controller)
	iteratorCallNumber := 0
	mockItr.EXPECT().HasNext().DoAndReturn(func() bool {
		return iteratorCallNumber < 10
	}).Times(11)
	mockItr.EXPECT().Next().DoAndReturn(func() (*entity.ConcreteExecution, error) {
		defer func() {
			iteratorCallNumber++
		}()
		switch iteratorCallNumber {
		case 0, 1, 2, 3:
			return &entity.ConcreteExecution{
				Execution: entity.Execution{
					DomainID: "healthy",
				},
			}, nil
		case 4, 5, 6:
			return &entity.ConcreteExecution{
				Execution: entity.Execution{
					DomainID: "history_missing",
					State:    persistence.WorkflowStateCompleted,
				},
			}, nil
		case 7:
			return &entity.ConcreteExecution{
				Execution: entity.Execution{
					DomainID: "orphan_execution",
					State:    persistence.WorkflowStateCreated,
				},
			}, nil
		case 8, 9:
			return &entity.ConcreteExecution{
				Execution: entity.Execution{
					DomainID: "failed",
				},
			}, nil
		default:
			panic("should not get here")
		}
	}).Times(10)
	mockInvariantManager := invariant.NewMockManager(s.controller)
	mockInvariantManager.EXPECT().RunChecks(context.Background(), &entity.ConcreteExecution{
		Execution: entity.Execution{
			DomainID: "healthy",
		},
	}).Return(invariant.ManagerCheckResult{
		CheckResultType: invariant.CheckResultTypeHealthy,
	}).Times(4)
	mockInvariantManager.EXPECT().RunChecks(context.Background(), &entity.ConcreteExecution{
		Execution: entity.Execution{
			DomainID: "history_missing",
			State:    persistence.WorkflowStateCompleted,
		},
	}).Return(invariant.ManagerCheckResult{
		CheckResultType:          invariant.CheckResultTypeCorrupted,
		DeterminingInvariantType: invariant.NamePtr(invariant.HistoryExists),
		CheckResults: []invariant.CheckResult{
			{
				CheckResultType: invariant.CheckResultTypeCorrupted,
				InvariantName:   invariant.HistoryExists,
				Info:            "history did not exist",
			},
		},
	}).Times(3)
	mockInvariantManager.EXPECT().RunChecks(context.Background(), &entity.ConcreteExecution{
		Execution: entity.Execution{
			DomainID: "orphan_execution",
			State:    persistence.WorkflowStateCreated,
		}}).Return(invariant.ManagerCheckResult{
		CheckResultType:          invariant.CheckResultTypeCorrupted,
		DeterminingInvariantType: invariant.NamePtr(invariant.OpenCurrentExecution),
		CheckResults: []invariant.CheckResult{
			{
				CheckResultType: invariant.CheckResultTypeHealthy,
				InvariantName:   invariant.HistoryExists,
			},
			{
				CheckResultType: invariant.CheckResultTypeCorrupted,
				InvariantName:   invariant.OpenCurrentExecution,
				Info:            "execution was orphan",
			},
		},
	}).Times(1)
	mockInvariantManager.EXPECT().RunChecks(context.Background(), &entity.ConcreteExecution{
		Execution: entity.Execution{
			DomainID: "failed",
		}}).Return(invariant.ManagerCheckResult{
		CheckResultType:          invariant.CheckResultTypeFailed,
		DeterminingInvariantType: invariant.NamePtr(invariant.HistoryExists),
		CheckResults: []invariant.CheckResult{
			{
				CheckResultType: invariant.CheckResultTypeFailed,
				InvariantName:   invariant.HistoryExists,
				Info:            "failed to check if history exists",
			},
		},
	}).Times(2)

	mockCorruptedWriter := store.NewMockExecutionWriter(s.controller)
	mockCorruptedWriter.EXPECT().Add(store.ScanOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "history_missing",
				State:    persistence.WorkflowStateCompleted,
			}},
		Result: invariant.ManagerCheckResult{
			CheckResultType:          invariant.CheckResultTypeCorrupted,
			DeterminingInvariantType: invariant.NamePtr(invariant.HistoryExists),
			CheckResults: []invariant.CheckResult{
				{
					CheckResultType: invariant.CheckResultTypeCorrupted,
					InvariantName:   invariant.HistoryExists,
					Info:            "history did not exist",
				},
			},
		},
	}).Times(3)
	mockCorruptedWriter.EXPECT().Add(store.ScanOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "orphan_execution",
				State:    persistence.WorkflowStateCreated,
			}},
		Result: invariant.ManagerCheckResult{
			CheckResultType:          invariant.CheckResultTypeCorrupted,
			DeterminingInvariantType: invariant.NamePtr(invariant.OpenCurrentExecution),
			CheckResults: []invariant.CheckResult{
				{
					CheckResultType: invariant.CheckResultTypeHealthy,
					InvariantName:   invariant.HistoryExists,
				},
				{
					CheckResultType: invariant.CheckResultTypeCorrupted,
					InvariantName:   invariant.OpenCurrentExecution,
					Info:            "execution was orphan",
				},
			},
		},
	}).Times(1)
	mockFailedWriter := store.NewMockExecutionWriter(s.controller)
	mockFailedWriter.EXPECT().Add(store.ScanOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "failed",
			}},
		Result: invariant.ManagerCheckResult{
			CheckResultType:          invariant.CheckResultTypeFailed,
			DeterminingInvariantType: invariant.NamePtr(invariant.HistoryExists),
			CheckResults: []invariant.CheckResult{
				{
					CheckResultType: invariant.CheckResultTypeFailed,
					InvariantName:   invariant.HistoryExists,
					Info:            "failed to check if history exists",
				},
			},
		},
	}).Times(2)
	mockCorruptedWriter.EXPECT().Flush().Return(nil)
	mockFailedWriter.EXPECT().Flush().Return(nil)
	mockCorruptedWriter.EXPECT().FlushedKeys().Return(&store.Keys{UUID: "corrupt_keys_uuid"})
	mockFailedWriter.EXPECT().FlushedKeys().Return(&store.Keys{UUID: "failed_keys_uuid"})
	domainCache := cache.NewMockDomainCache(s.controller)
	domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil).AnyTimes()

	scanner := &ShardScanner{
		shardID:          0,
		invariantManager: mockInvariantManager,
		corruptedWriter:  mockCorruptedWriter,
		failedWriter:     mockFailedWriter,
		itr:              mockItr,
		progressReportFn: func() {},
		domainCache:      domainCache,
		scope:            metrics.NoopScope,
	}
	result := scanner.Scan(context.Background())
	s.Equal(ScanReport{
		ShardID: 0,
		Stats: ScanStats{
			EntitiesCount:    10,
			CorruptedCount:   4,
			CheckFailedCount: 2,
			CorruptionByType: map[invariant.Name]int64{
				invariant.HistoryExists:        3,
				invariant.OpenCurrentExecution: 1,
			},
		},
		Result: ScanResult{
			ShardScanKeys: &ScanKeys{
				Corrupt: &store.Keys{UUID: "corrupt_keys_uuid"},
				Failed:  &store.Keys{UUID: "failed_keys_uuid"},
			},
		},
		DomainStats: map[string]*ScanStats{
			"failed": &ScanStats{
				EntitiesCount:    2,
				CheckFailedCount: 2,
				CorruptionByType: make(map[invariant.Name]int64),
			},
			"healthy": &ScanStats{
				EntitiesCount:    4,
				CorruptionByType: make(map[invariant.Name]int64),
			},
			"history_missing": &ScanStats{
				EntitiesCount:  3,
				CorruptedCount: 3,
				CorruptionByType: map[invariant.Name]int64{
					invariant.HistoryExists: 3,
				},
			},
			"orphan_execution": &ScanStats{
				EntitiesCount:  1,
				CorruptedCount: 1,
				CorruptionByType: map[invariant.Name]int64{
					invariant.OpenCurrentExecution: 1,
				},
			},
		},
	}, result)
}

func (s *ScannerSuite) TestGetDomainIDFromEntity() {
	scanner := &ShardScanner{}

	concreteExecution := entity.ConcreteExecution{Execution: entity.Execution{DomainID: "A"}}
	concreteExecutionDomain, err := scanner.getDomainIDFromEntity(&concreteExecution)
	s.NoError(err)
	s.Equal("A", *concreteExecutionDomain)

	currentExecution := entity.CurrentExecution{Execution: entity.Execution{DomainID: "B"}}
	currentExecutionDomain, err := scanner.getDomainIDFromEntity(&currentExecution)
	s.NoError(err)
	s.Equal("B", *currentExecutionDomain)

	timer := entity.CurrentExecution{Execution: entity.Execution{DomainID: "C"}}
	timerDomain, err := scanner.getDomainIDFromEntity(&timer)
	s.NoError(err)
	s.Equal("C", *timerDomain)

	baseExecution := entity.Execution{DomainID: "D"}
	result, err := scanner.getDomainIDFromEntity(&baseExecution)
	s.Nil(result)
	s.Error(err)
}
