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
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/reconciliation/store"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
)

type FixerSuite struct {
	*require.Assertions
	suite.Suite

	mockResource *resource.Test
	controller   *gomock.Controller
}

func TestFixerSuite(t *testing.T) {
	suite.Run(t, new(FixerSuite))
}

func (s *FixerSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.controller = gomock.NewController(s.T())
	s.mockResource = resource.NewTest(s.T(), s.controller, metrics.Worker)
}

func (s *FixerSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *FixerSuite) TestFix_Failure_FirstIteratorError() {
	mockItr := store.NewMockScanOutputIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(true).Times(1)
	mockItr.EXPECT().Next().Return(nil, errors.New("iterator error")).Times(1)
	fixer := &ShardFixer{
		shardID:          0,
		itr:              mockItr,
		progressReportFn: func() {},
	}
	result := fixer.Fix()
	s.Equal(FixReport{
		ShardID: 0,
		Result: FixResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "blobstore iterator returned error",
				InfoDetails: "iterator error",
			},
		},
	}, result)
}

func (s *FixerSuite) TestFix_Failure_NonFirstError() {
	mockItr := store.NewMockScanOutputIterator(s.controller)
	iteratorCallNumber := 0
	mockItr.EXPECT().HasNext().DoAndReturn(func() bool {
		return iteratorCallNumber < 5
	}).Times(5)
	mockItr.EXPECT().Next().DoAndReturn(func() (*store.ScanOutputEntity, error) {
		defer func() {
			iteratorCallNumber++
		}()
		if iteratorCallNumber < 4 {
			return &store.ScanOutputEntity{
				Execution: &entity.ConcreteExecution{
					Execution: entity.Execution{
						DomainID: "test_domain",
					},
				},
			}, nil
		}
		return nil, fmt.Errorf("iterator got error on: %v", iteratorCallNumber)
	}).Times(5)
	mockInvariantManager := invariant.NewMockManager(s.controller)
	mockInvariantManager.EXPECT().RunFixes(gomock.Any(), gomock.Any()).Return(invariant.ManagerFixResult{
		FixResultType: invariant.FixResultTypeFixed,
	}).Times(4)
	fixedWriter := store.NewMockExecutionWriter(s.controller)
	fixedWriter.EXPECT().Add(gomock.Any()).Return(nil).Times(4)
	domainCache := cache.NewMockDomainCache(s.controller)
	domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil).Times(4)
	fixer := &ShardFixer{
		shardID:          0,
		itr:              mockItr,
		invariantManager: mockInvariantManager,
		fixedWriter:      fixedWriter,
		progressReportFn: func() {},
		domainCache:      domainCache,
		allowDomain:      dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
		scope:            metrics.NoopScope,
	}
	result := fixer.Fix()
	s.Equal(FixReport{
		ShardID: 0,
		Stats: FixStats{
			EntitiesCount: 4,
			FixedCount:    4,
		},
		Result: FixResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "blobstore iterator returned error",
				InfoDetails: "iterator got error on: 4",
			},
		},
	}, result)
}

func (s *FixerSuite) TestFix_Failure_SkippedWriterError() {
	mockItr := store.NewMockScanOutputIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(true).Times(1)
	mockItr.EXPECT().Next().Return(&store.ScanOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "test_domain",
			},
		},
	}, nil).Times(1)
	mockInvariantManager := invariant.NewMockManager(s.controller)
	mockInvariantManager.EXPECT().RunFixes(gomock.Any(), gomock.Any()).Return(invariant.ManagerFixResult{
		FixResultType: invariant.FixResultTypeSkipped,
	}).Times(1)
	skippedWriter := store.NewMockExecutionWriter(s.controller)
	skippedWriter.EXPECT().Add(gomock.Any()).Return(errors.New("skipped writer error")).Times(1)
	domainCache := cache.NewMockDomainCache(s.controller)
	domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil).Times(1)
	fixer := &ShardFixer{
		shardID:          0,
		itr:              mockItr,
		skippedWriter:    skippedWriter,
		invariantManager: mockInvariantManager,
		progressReportFn: func() {},
		domainCache:      domainCache,
		allowDomain:      dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
		scope:            metrics.NoopScope,
	}
	result := fixer.Fix()
	s.Equal(FixReport{
		ShardID: 0,
		Stats: FixStats{
			EntitiesCount: 1,
		},
		Result: FixResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "blobstore add failed for skipped execution fix",
				InfoDetails: "skipped writer error",
			},
		},
	}, result)
}

func (s *FixerSuite) TestFix_Failure_FailedWriterError() {
	mockItr := store.NewMockScanOutputIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(true).Times(1)
	mockItr.EXPECT().Next().Return(&store.ScanOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "test_domain",
			},
		},
	}, nil).Times(1)
	mockInvariantManager := invariant.NewMockManager(s.controller)
	mockInvariantManager.EXPECT().RunFixes(gomock.Any(), gomock.Any()).Return(invariant.ManagerFixResult{
		FixResultType: invariant.FixResultTypeFailed,
	}).Times(1)
	failedWriter := store.NewMockExecutionWriter(s.controller)
	failedWriter.EXPECT().Add(gomock.Any()).Return(errors.New("failed writer error")).Times(1)
	domainCache := cache.NewMockDomainCache(s.controller)
	domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil).Times(1)
	fixer := &ShardFixer{
		shardID:          0,
		itr:              mockItr,
		failedWriter:     failedWriter,
		invariantManager: mockInvariantManager,
		progressReportFn: func() {},
		domainCache:      domainCache,
		allowDomain:      dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
		scope:            metrics.NoopScope,
	}
	result := fixer.Fix()
	s.Equal(FixReport{
		ShardID: 0,
		Stats: FixStats{
			EntitiesCount: 1,
		},
		Result: FixResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "blobstore add failed for failed execution fix",
				InfoDetails: "failed writer error",
			},
		},
	}, result)
}

func (s *FixerSuite) TestFix_Failure_FixedWriterError() {
	mockItr := store.NewMockScanOutputIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(true).Times(1)
	mockItr.EXPECT().Next().Return(&store.ScanOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "test_domain",
			},
		},
	}, nil).Times(1)
	mockInvariantManager := invariant.NewMockManager(s.controller)
	mockInvariantManager.EXPECT().RunFixes(gomock.Any(), gomock.Any()).Return(invariant.ManagerFixResult{
		FixResultType: invariant.FixResultTypeFixed,
	}).Times(1)
	fixedWriter := store.NewMockExecutionWriter(s.controller)
	fixedWriter.EXPECT().Add(gomock.Any()).Return(errors.New("fixed writer error")).Times(1)
	domainCache := cache.NewMockDomainCache(s.controller)
	domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil).Times(1)
	fixer := &ShardFixer{
		shardID:          0,
		itr:              mockItr,
		fixedWriter:      fixedWriter,
		invariantManager: mockInvariantManager,
		progressReportFn: func() {},
		domainCache:      domainCache,
		allowDomain:      dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
		scope:            metrics.NoopScope,
	}
	result := fixer.Fix()
	s.Equal(FixReport{
		ShardID: 0,
		Stats: FixStats{
			EntitiesCount: 1,
		},
		Result: FixResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "blobstore add failed for fixed execution fix",
				InfoDetails: "fixed writer error",
			},
		},
	}, result)
}

func (s *FixerSuite) TestFix_Failure_FixedWriterFlushError() {
	mockItr := store.NewMockScanOutputIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(false).Times(1)
	fixedWriter := store.NewMockExecutionWriter(s.controller)
	fixedWriter.EXPECT().Flush().Return(errors.New("fix writer flush failed")).Times(1)
	fixer := &ShardFixer{
		shardID:          0,
		itr:              mockItr,
		fixedWriter:      fixedWriter,
		progressReportFn: func() {},
	}
	result := fixer.Fix()
	s.Equal(FixReport{
		ShardID: 0,
		Result: FixResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "failed to flush for fixed execution fixes",
				InfoDetails: "fix writer flush failed",
			},
		},
	}, result)
}

func (s *FixerSuite) TestFix_Failure_SkippedWriterFlushError() {
	mockItr := store.NewMockScanOutputIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(false).Times(1)
	fixedWriter := store.NewMockExecutionWriter(s.controller)
	fixedWriter.EXPECT().Flush().Return(nil)
	skippedWriter := store.NewMockExecutionWriter(s.controller)
	skippedWriter.EXPECT().Flush().Return(errors.New("skip writer flush failed")).Times(1)
	fixer := &ShardFixer{
		shardID:          0,
		itr:              mockItr,
		fixedWriter:      fixedWriter,
		skippedWriter:    skippedWriter,
		progressReportFn: func() {},
	}
	result := fixer.Fix()
	s.Equal(FixReport{
		ShardID: 0,
		Result: FixResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "failed to flush for skipped execution fixes",
				InfoDetails: "skip writer flush failed",
			},
		},
	}, result)
}

func (s *FixerSuite) TestFix_Failure_FailedWriterFlushError() {
	mockItr := store.NewMockScanOutputIterator(s.controller)
	mockItr.EXPECT().HasNext().Return(false).Times(1)
	fixedWriter := store.NewMockExecutionWriter(s.controller)
	fixedWriter.EXPECT().Flush().Return(nil)
	skippedWriter := store.NewMockExecutionWriter(s.controller)
	skippedWriter.EXPECT().Flush().Return(nil).Times(1)
	failedWriter := store.NewMockExecutionWriter(s.controller)
	failedWriter.EXPECT().Flush().Return(errors.New("fail writer flush failed")).Times(1)
	fixer := &ShardFixer{
		shardID:          0,
		itr:              mockItr,
		fixedWriter:      fixedWriter,
		skippedWriter:    skippedWriter,
		failedWriter:     failedWriter,
		progressReportFn: func() {},
	}
	result := fixer.Fix()
	s.Equal(FixReport{
		ShardID: 0,
		Result: FixResult{
			ControlFlowFailure: &ControlFlowFailure{
				Info:        "failed to flush for failed execution fixes",
				InfoDetails: "fail writer flush failed",
			},
		},
	}, result)
}

func (s *FixerSuite) TestFix_Success() {
	mockItr := store.NewMockScanOutputIterator(s.controller)
	iteratorCallNumber := 0
	mockItr.EXPECT().HasNext().DoAndReturn(func() bool {
		return iteratorCallNumber < 12
	}).Times(13)
	mockItr.EXPECT().Next().DoAndReturn(func() (*store.ScanOutputEntity, error) {
		defer func() {
			iteratorCallNumber++
		}()
		switch iteratorCallNumber {
		case 0, 1, 2, 3:
			return &store.ScanOutputEntity{
				Execution: &entity.ConcreteExecution{
					Execution: entity.Execution{
						DomainID: "skipped",
					},
				},
			}, nil
		case 4, 5:
			return &store.ScanOutputEntity{
				Execution: &entity.ConcreteExecution{
					Execution: entity.Execution{
						DomainID: "history_missing",
					},
				},
			}, nil
		case 6:
			return &store.ScanOutputEntity{
				Execution: &entity.ConcreteExecution{
					Execution: entity.Execution{
						DomainID: "first_history_event",
					},
				},
			}, nil
		case 7:
			return &store.ScanOutputEntity{
				Execution: &entity.ConcreteExecution{
					Execution: entity.Execution{
						DomainID: "orphan_execution",
					},
				},
			}, nil
		case 8, 9:
			return &store.ScanOutputEntity{
				Execution: &entity.ConcreteExecution{
					Execution: entity.Execution{
						DomainID: "failed",
					},
				},
			}, nil
		case 10, 11:
			return &store.ScanOutputEntity{
				Execution: &entity.ConcreteExecution{
					Execution: entity.Execution{
						DomainID: "disallow_domain",
					},
				},
			}, nil
		default:
			panic("should not get here")
		}
	}).Times(12)
	mockInvariantManager := invariant.NewMockManager(s.controller)
	mockInvariantManager.EXPECT().RunFixes(gomock.Any(), &entity.ConcreteExecution{
		Execution: entity.Execution{
			DomainID: "skipped",
		},
	}).Return(invariant.ManagerFixResult{
		FixResultType: invariant.FixResultTypeSkipped,
		FixResults: []invariant.FixResult{
			{
				FixResultType: invariant.FixResultTypeSkipped,
				InvariantName: invariant.HistoryExists,
			},
			{
				FixResultType: invariant.FixResultTypeSkipped,
			},
			{
				FixResultType: invariant.FixResultTypeSkipped,
				InvariantName: invariant.OpenCurrentExecution,
			},
		},
	}).Times(4)
	mockInvariantManager.EXPECT().RunFixes(gomock.Any(), &entity.ConcreteExecution{
		Execution: entity.Execution{
			DomainID: "history_missing",
		},
	}).Return(invariant.ManagerFixResult{
		FixResultType: invariant.FixResultTypeFixed,
		FixResults: []invariant.FixResult{
			{
				FixResultType: invariant.FixResultTypeFixed,
				InvariantName: invariant.HistoryExists,
				Info:          "history did not exist",
			},
		},
	}).Times(2)
	mockInvariantManager.EXPECT().RunFixes(gomock.Any(), &entity.ConcreteExecution{
		Execution: entity.Execution{
			DomainID: "first_history_event",
		},
	}).Return(invariant.ManagerFixResult{
		FixResultType: invariant.FixResultTypeFixed,
		FixResults: []invariant.FixResult{
			{
				FixResultType: invariant.FixResultTypeSkipped,
				InvariantName: invariant.HistoryExists,
			},
			{
				FixResultType: invariant.FixResultTypeFixed,
				Info:          "first event is not valid",
			},
		},
	}).Times(1)
	mockInvariantManager.EXPECT().RunFixes(gomock.Any(), &entity.ConcreteExecution{
		Execution: entity.Execution{
			DomainID: "orphan_execution",
			State:    persistence.WorkflowStateCreated,
		},
	}).Return(invariant.ManagerFixResult{
		FixResultType: invariant.FixResultTypeFixed,
		FixResults: []invariant.FixResult{
			{
				FixResultType: invariant.FixResultTypeSkipped,
				InvariantName: invariant.HistoryExists,
			},
			{
				FixResultType: invariant.FixResultTypeSkipped,
			},
			{
				FixResultType: invariant.FixResultTypeFixed,
				InvariantName: invariant.OpenCurrentExecution,
				Info:          "execution was orphan",
			},
		},
	}).Times(1)
	mockInvariantManager.EXPECT().RunFixes(gomock.Any(), &entity.ConcreteExecution{
		Execution: entity.Execution{
			DomainID: "failed",
		},
	}).Return(invariant.ManagerFixResult{
		FixResultType: invariant.FixResultTypeFailed,
		FixResults: []invariant.FixResult{
			{
				FixResultType: invariant.FixResultTypeFailed,
				InvariantName: invariant.HistoryExists,
				Info:          "failed to check if history exists",
			},
		},
	}).Times(2)

	mockFixedWriter := store.NewMockExecutionWriter(s.controller)
	mockFixedWriter.EXPECT().Add(store.FixOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "history_missing",
			},
		},
		Input: store.ScanOutputEntity{
			Execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					DomainID: "history_missing",
				},
			},
		},
		Result: invariant.ManagerFixResult{
			FixResultType: invariant.FixResultTypeFixed,
			FixResults: []invariant.FixResult{
				{
					FixResultType: invariant.FixResultTypeFixed,
					InvariantName: invariant.HistoryExists,
					Info:          "history did not exist",
				},
			},
		},
	}).Times(2)
	mockFixedWriter.EXPECT().Add(store.FixOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "first_history_event",
			},
		},
		Input: store.ScanOutputEntity{
			Execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					DomainID: "first_history_event",
				},
			},
		},
		Result: invariant.ManagerFixResult{
			FixResultType: invariant.FixResultTypeFixed,
			FixResults: []invariant.FixResult{
				{
					FixResultType: invariant.FixResultTypeSkipped,
					InvariantName: invariant.HistoryExists,
				},
				{
					FixResultType: invariant.FixResultTypeFixed,
					Info:          "first event is not valid",
				},
			},
		},
	}).Times(1)
	mockFixedWriter.EXPECT().Add(store.FixOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "orphan_execution",
			},
		},
		Input: store.ScanOutputEntity{
			Execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					DomainID: "orphan_execution",
				},
			},
		},
		Result: invariant.ManagerFixResult{
			FixResultType: invariant.FixResultTypeFixed,
			FixResults: []invariant.FixResult{
				{
					FixResultType: invariant.FixResultTypeSkipped,
					InvariantName: invariant.HistoryExists,
				},
				{
					FixResultType: invariant.FixResultTypeSkipped,
				},
				{
					FixResultType: invariant.FixResultTypeFixed,
					InvariantName: invariant.OpenCurrentExecution,
					Info:          "execution was orphan",
				},
			},
		},
	}).Times(1)
	mockFailedWriter := store.NewMockExecutionWriter(s.controller)
	mockFailedWriter.EXPECT().Add(store.FixOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "failed",
			},
		},
		Input: store.ScanOutputEntity{
			Execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					DomainID: "failed",
				},
			},
		},
		Result: invariant.ManagerFixResult{
			FixResultType: invariant.FixResultTypeFailed,
			FixResults: []invariant.FixResult{
				{
					FixResultType: invariant.FixResultTypeFailed,
					InvariantName: invariant.HistoryExists,
					Info:          "failed to check if history exists",
				},
			},
		},
	}).Times(2)
	mockSkippedWriter := store.NewMockExecutionWriter(s.controller)
	mockSkippedWriter.EXPECT().Add(store.FixOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "skipped",
			},
		},
		Input: store.ScanOutputEntity{
			Execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					DomainID: "skipped",
				},
			},
		},
		Result: invariant.ManagerFixResult{
			FixResultType: invariant.FixResultTypeSkipped,
			FixResults: []invariant.FixResult{
				{
					FixResultType: invariant.FixResultTypeSkipped,
					InvariantName: invariant.HistoryExists,
				},
				{
					FixResultType: invariant.FixResultTypeSkipped,
				},
				{
					FixResultType: invariant.FixResultTypeSkipped,
					InvariantName: invariant.OpenCurrentExecution,
				},
			},
		},
	}).Times(4)
	mockSkippedWriter.EXPECT().Add(store.FixOutputEntity{
		Execution: &entity.ConcreteExecution{
			Execution: entity.Execution{
				DomainID: "disallow_domain",
			},
		},
		Input: store.ScanOutputEntity{
			Execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					DomainID: "disallow_domain",
				},
			},
		},
		Result: invariant.ManagerFixResult{
			FixResultType: invariant.FixResultTypeSkipped,
		},
	}).Times(2)
	mockSkippedWriter.EXPECT().Flush().Return(nil)
	mockFailedWriter.EXPECT().Flush().Return(nil)
	mockFixedWriter.EXPECT().Flush().Return(nil)
	mockSkippedWriter.EXPECT().FlushedKeys().Return(&store.Keys{UUID: "skipped_keys_uuid"})
	mockFailedWriter.EXPECT().FlushedKeys().Return(&store.Keys{UUID: "failed_keys_uuid"})
	mockFixedWriter.EXPECT().FlushedKeys().Return(&store.Keys{UUID: "fixed_keys_uuid"})
	domainCache := cache.NewMockDomainCache(s.controller)
	domainCache.EXPECT().GetDomainName("skipped").Return("skipped", nil).Times(4)
	domainCache.EXPECT().GetDomainName("history_missing").Return("history_missing", nil).Times(2)
	domainCache.EXPECT().GetDomainName("first_history_event").Return("first_history_event", nil).Times(1)
	domainCache.EXPECT().GetDomainName("orphan_execution").Return("orphan_execution", nil).Times(1)
	domainCache.EXPECT().GetDomainName("failed").Return("failed", nil).Times(2)
	domainCache.EXPECT().GetDomainName("disallow_domain").Return("disallow_domain", nil).Times(2)

	allowDomain := func(domain string) bool {
		return domain != "disallow_domain"
	}
	fixer := &ShardFixer{
		shardID:          0,
		invariantManager: mockInvariantManager,
		skippedWriter:    mockSkippedWriter,
		failedWriter:     mockFailedWriter,
		fixedWriter:      mockFixedWriter,
		itr:              mockItr,
		progressReportFn: func() {},
		domainCache:      domainCache,
		allowDomain:      allowDomain,
		scope:            metrics.NoopScope,
	}
	result := fixer.Fix()
	s.Equal(FixReport{
		ShardID: 0,
		Stats: FixStats{
			EntitiesCount: 12,
			FixedCount:    4,
			SkippedCount:  6,
			FailedCount:   2,
		},
		Result: FixResult{
			ShardFixKeys: &FixKeys{
				Fixed:   &store.Keys{UUID: "fixed_keys_uuid"},
				Failed:  &store.Keys{UUID: "failed_keys_uuid"},
				Skipped: &store.Keys{UUID: "skipped_keys_uuid"},
			},
		},
	}, result)
}

func TestFix_DomainLookupError(t *testing.T) {
	tests := map[string]struct {
		domainErr    error
		allowDomain  dynamicproperties.BoolPropertyFnWithDomainFilter
		wantRunFixes int
		wantFailure  *ControlFlowFailure
		wantStats    FixStats
	}{
		"domain not found runs fixes when all domains are allowed": {
			domainErr:    &types.EntityNotExistsError{Message: "domain not found"},
			allowDomain:  dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
			wantRunFixes: 2,
			wantStats:    FixStats{EntitiesCount: 2, FailedCount: 2},
		},
		"domain not found is skipped by a per-domain allowlist": {
			domainErr:   &types.EntityNotExistsError{Message: "domain not found"},
			allowDomain: func(domain string) bool { return domain == "allowed-domain" },
			wantStats:   FixStats{EntitiesCount: 2, SkippedCount: 2},
		},
		"other errors abort the fix": {
			domainErr:   errors.New("timeout"),
			allowDomain: dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
			wantFailure: &ControlFlowFailure{
				Info:        "failed to get domain name",
				InfoDetails: "timeout",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			execution := &entity.ConcreteExecution{Execution: entity.Execution{DomainID: "missing"}}
			calls := 0
			mockItr := store.NewMockScanOutputIterator(ctrl)
			mockItr.EXPECT().HasNext().DoAndReturn(func() bool { return calls < 2 }).AnyTimes()
			mockItr.EXPECT().Next().DoAndReturn(func() (*store.ScanOutputEntity, error) {
				calls++
				return &store.ScanOutputEntity{Execution: execution}, nil
			}).AnyTimes()
			mockInvariantManager := invariant.NewMockManager(ctrl)
			mockInvariantManager.EXPECT().RunFixes(gomock.Any(), execution).Return(invariant.ManagerFixResult{
				FixResultType: invariant.FixResultTypeFailed,
			}).Times(tc.wantRunFixes)
			domainCache := cache.NewMockDomainCache(ctrl)
			domainCache.EXPECT().GetDomainName("missing").Return("", tc.domainErr).AnyTimes()
			failedWriter := store.NewMockExecutionWriter(ctrl)
			failedWriter.EXPECT().Add(gomock.Any()).Return(nil).AnyTimes()
			skippedWriter := store.NewMockExecutionWriter(ctrl)
			skippedWriter.EXPECT().Add(gomock.Any()).Return(nil).AnyTimes()
			fixedWriter := store.NewMockExecutionWriter(ctrl)
			for _, w := range []*store.MockExecutionWriter{failedWriter, skippedWriter, fixedWriter} {
				w.EXPECT().Flush().Return(nil).AnyTimes()
				w.EXPECT().FlushedKeys().Return(nil).AnyTimes()
			}

			fixer := &ShardFixer{
				ctx:              context.Background(),
				itr:              mockItr,
				skippedWriter:    skippedWriter,
				failedWriter:     failedWriter,
				fixedWriter:      fixedWriter,
				invariantManager: mockInvariantManager,
				progressReportFn: func() {},
				domainCache:      domainCache,
				allowDomain:      tc.allowDomain,
				scope:            metrics.NoopScope,
			}
			result := fixer.Fix()

			require.Equal(t, tc.wantFailure, result.Result.ControlFlowFailure)
			require.Equal(t, tc.wantStats, result.Stats)
		})
	}
}
