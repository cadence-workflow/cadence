// Copyright (c) 2019 Uber Technologies, Inc.
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
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

const (
	testDomainID    = "test-domain-id"
	testWorkflowID  = "test-workflow-id"
	testRunID       = "test-run-id"
	testDomainName  = "test-domain"
	testTimeout     = time.Second * 10
	testBranchToken = "branch-token"
)

var (
	matchingChecksum = checksum.Checksum{
		Version: 1,
		Flavor:  checksum.FlavorIEEECRC32OverThriftBinary,
		Value:   []byte{223, 81, 9, 232},
	}
	mismatchedChecksum = checksum.Checksum{
		Version: 1,
		Flavor:  checksum.FlavorIEEECRC32OverThriftBinary,
		Value:   []byte{0xff, 0xff, 0xff, 0xff},
	}
)

// setupDetectionMocks sets up mocks for the detection path in VerifyAndRepairWorkflowIfNeeded.
// Sets GetChecksum to return a mismatched checksum (so corruption is always detected) and
// configures the pending info mocks needed for logging on mismatch.
func setupDetectionMocks(ms *MockMutableState, domainID, workflowID, runID string) {
	ms.EXPECT().GetChecksum().Return(mismatchedChecksum).AnyTimes()
	ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	}).AnyTimes()
	ms.EXPECT().GetDomainEntry().Return(cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: domainID, Name: testDomainName},
		&persistence.DomainConfig{},
		&persistence.DomainReplicationConfig{ActiveClusterName: "active"},
		0,
	)).AnyTimes()
	ms.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{}).AnyTimes()
	ms.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{}).AnyTimes()
	ms.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{}).AnyTimes()
	ms.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{}).AnyTimes()
	ms.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{}).AnyTimes()
}

func setupVersionHistories(ms *MockMutableState) {
	ms.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte(testBranchToken),
				Items: []*persistence.VersionHistoryItem{
					{EventID: 9, Version: 1},
				},
			},
		},
	}).AnyTimes()
	ms.EXPECT().GetCurrentBranchToken().Return([]byte(testBranchToken), nil).Times(1)
}

func setupDomainCacheMocks(testShard *shard.TestContext) {
	testShard.Resource.DomainCache.EXPECT().GetDomainName(testDomainID).Return(testDomainName, nil).Times(1)
	testShard.Resource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: testDomainID, Name: testDomainName},
		&persistence.DomainConfig{},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: "active",
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: "active"},
			},
		},
		1234,
	), nil).Times(1)
}

func setupSuccessfulRebuild(
	ctrl *gomock.Controller,
	mockStateRebuilder *MockStateRebuilder,
	checksumMatch bool,
) *MockMutableState {
	mockRebuiltMS := NewMockMutableState(ctrl)
	if checksumMatch {
		mockRebuiltMS.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
			DomainID:   testDomainID,
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		}).AnyTimes()
	} else {
		mockRebuiltMS.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
			DomainID:    testDomainID,
			WorkflowID:  testWorkflowID,
			RunID:       testRunID,
			NextEventID: 999,
		}).AnyTimes()
	}
	mockRebuiltMS.EXPECT().GetVersionHistories().Return(nil).AnyTimes()
	mockRebuiltMS.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{}).AnyTimes()
	mockRebuiltMS.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{}).AnyTimes()
	mockRebuiltMS.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{}).AnyTimes()
	mockRebuiltMS.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{}).AnyTimes()
	mockRebuiltMS.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{}).AnyTimes()

	mockStateRebuilder.EXPECT().Rebuild(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		[]byte(testBranchToken),
		int64(9),
		int64(1),
		gomock.Any(),
		gomock.Any(),
		"",
	).Return(mockRebuiltMS, int64(0), nil).Times(1)

	mockRebuiltMS.EXPECT().SetHistorySize(int64(0)).Times(1)
	mockRebuiltMS.EXPECT().SetUpdateCondition(int64(0)).Times(1)
	mockRebuiltMS.EXPECT().GetHistorySize().Return(int64(1024)).Times(1)
	mockRebuiltMS.EXPECT().CloseTransactionAsMutation(gomock.Any(), TransactionPolicyPassive).Return(
		&persistence.WorkflowMutation{
			ExecutionInfo: &persistence.WorkflowExecutionInfo{
				DomainID:   testDomainID,
				WorkflowID: testWorkflowID,
				RunID:      testRunID,
			},
		},
		nil,
		nil,
	).Times(1)

	return mockRebuiltMS
}

func TestNewWorkflowRepairer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testShard := shard.NewTestContext(
		t,
		ctrl,
		&persistence.ShardInfo{
			ShardID: 0,
			RangeID: 1,
		},
		config.NewForTest(),
	)

	mockLogger := log.NewNoop()
	mockMetricsClient := metrics.NewClient(tally.NoopScope, metrics.History, metrics.MigrationConfig{})

	repairer := NewWorkflowRepairer(testShard, mockLogger, mockMetricsClient)

	require.NotNil(t, repairer)
	impl, ok := repairer.(*workflowRepairerImpl)
	require.True(t, ok)
	require.Equal(t, testShard, impl.shard)
	require.Equal(t, mockLogger, impl.logger)
	require.Equal(t, mockMetricsClient, impl.metricsClient)
	require.Nil(t, impl.stateRebuilder) // lazily initialized on first repair
	require.NotNil(t, impl.scope)
}

func TestCorruptionType_String(t *testing.T) {
	require.Equal(t, "None", CorruptionTypeNone.String())
	require.Equal(t, "ChecksumMismatch", CorruptionTypeChecksumMismatch.String())
	require.Equal(t, "Unknown", CorruptionType(999).String())
}

func TestWorkflowRepairer_VerifyAndRepairWorkflowIfNeeded(t *testing.T) {
	testError := errors.New("test-error")
	testPersistenceError := &persistence.ConditionFailedError{Msg: "conflict"}

	tests := []struct {
		name         string
		setupFunc    func(*gomock.Controller, *shard.TestContext, *config.Config, *MockMutableState, *MockStateRebuilder)
		wantRepaired bool
		wantErr      bool
		wantErrIs    error
	}{
		{
			name: "no checksum stored - skip verification",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				ms.EXPECT().GetChecksum().Return(checksum.Checksum{}).Times(1)
			},
			wantRepaired: false,
		},
		{
			name: "nil domain entry - skip verification",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				ms.EXPECT().GetChecksum().Return(mismatchedChecksum).Times(1)
				ms.EXPECT().GetDomainEntry().Return(nil).Times(1)
			},
			wantRepaired: false,
		},
		{
			name: "checksum matches - no corruption",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				ms.EXPECT().GetChecksum().Return(matchingChecksum).Times(1)
				ms.EXPECT().GetDomainEntry().Return(cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: testDomainID, Name: testDomainName},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{ActiveClusterName: "active"},
					0,
				)).AnyTimes()
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				}).AnyTimes()
				ms.EXPECT().GetVersionHistories().Return(nil).AnyTimes()
				ms.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{}).AnyTimes()
				ms.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{}).AnyTimes()
				ms.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{}).AnyTimes()
				ms.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{}).AnyTimes()
				ms.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{}).AnyTimes()
			},
			wantRepaired: false,
		},
		{
			name: "checksum mismatch - repair disabled",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				ms.EXPECT().GetVersionHistories().Return(nil).AnyTimes()
			},
			wantRepaired: false,
			wantErr:      false,
		},
		{
			name: "checksum mismatch - repair enabled - rebuild fails",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				ms.EXPECT().GetCurrentBranchToken().Return([]byte{}, testError).Times(1)
				ms.EXPECT().GetVersionHistories().Return(nil).AnyTimes()
			},
			wantRepaired: false,
			wantErr:      true,
			wantErrIs:    testError,
		},
		{
			name: "checksum mismatch - repair enabled - version histories nil",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				ms.EXPECT().GetCurrentBranchToken().Return([]byte(testBranchToken), nil).Times(1)
				// GetVersionHistories returns nil for both calls (checksum.go:73 and repairViaRebuild)
				ms.EXPECT().GetVersionHistories().Return(nil).AnyTimes()
			},
			wantRepaired: false,
			wantErr:      true,
			wantErrIs:    ErrMissingVersionHistories,
		},
		{
			name: "checksum mismatch - repair enabled - GetCurrentVersionHistory fails",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				ms.EXPECT().GetCurrentBranchToken().Return([]byte(testBranchToken), nil).Times(1)
				ms.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
					CurrentVersionHistoryIndex: 99,
					Histories:                  []*persistence.VersionHistory{},
				}).AnyTimes()
			},
			wantRepaired: false,
			wantErr:      true,
		},
		{
			name: "checksum mismatch - repair enabled - GetLastItem fails",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				ms.EXPECT().GetCurrentBranchToken().Return([]byte(testBranchToken), nil).Times(1)
				ms.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories: []*persistence.VersionHistory{
						{BranchToken: []byte(testBranchToken), Items: []*persistence.VersionHistoryItem{}},
					},
				}).AnyTimes()
			},
			wantRepaired: false,
			wantErr:      true,
		},
		{
			name: "checksum mismatch - repair enabled - rebuild timeout",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				setupVersionHistories(ms)
				sr.EXPECT().Rebuild(
					gomock.Any(), gomock.Any(), gomock.Any(),
					[]byte(testBranchToken), int64(9), int64(1),
					gomock.Any(), gomock.Any(), "",
				).Return(nil, int64(0), context.DeadlineExceeded).Times(1)
			},
			wantRepaired: false,
			wantErr:      true,
			wantErrIs:    context.DeadlineExceeded,
		},
		{
			name: "checksum mismatch - repair enabled - RequireChecksumMatchAfterRebuildRepair blocks repair",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				mockConfig.RequireChecksumMatchAfterRebuildRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				setupVersionHistories(ms)

				mockRebuiltMS := NewMockMutableState(ctrl)
				mockRebuiltMS.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:    testDomainID,
					WorkflowID:  testWorkflowID,
					RunID:       testRunID,
					NextEventID: 999,
				}).AnyTimes()
				mockRebuiltMS.EXPECT().GetVersionHistories().Return(nil).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{}).AnyTimes()

				sr.EXPECT().Rebuild(
					gomock.Any(), gomock.Any(), gomock.Any(),
					[]byte(testBranchToken), int64(9), int64(1),
					gomock.Any(), gomock.Any(), "",
				).Return(mockRebuiltMS, int64(0), nil).Times(1)
				mockRebuiltMS.EXPECT().SetHistorySize(int64(0)).Times(1)
			},
			wantRepaired: false,
			wantErr:      true,
			wantErrIs:    ErrChecksumMismatchAfterRebuild,
		},
		{
			name: "checksum mismatch - repair enabled - GetDomainName fails",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				mockConfig.RequireChecksumMatchAfterRebuildRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				setupVersionHistories(ms)

				// Minimal rebuild mock — GetDomainName fails before GetHistorySize/CloseTransactionAsMutation are reached
				mockRebuiltMS := NewMockMutableState(ctrl)
				mockRebuiltMS.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				}).AnyTimes()
				mockRebuiltMS.EXPECT().GetVersionHistories().Return(nil).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{}).AnyTimes()
				sr.EXPECT().Rebuild(
					gomock.Any(), gomock.Any(), gomock.Any(),
					[]byte(testBranchToken), int64(9), int64(1),
					gomock.Any(), gomock.Any(), "",
				).Return(mockRebuiltMS, int64(0), nil).Times(1)
				mockRebuiltMS.EXPECT().SetHistorySize(int64(0)).Times(1)
				mockRebuiltMS.EXPECT().SetUpdateCondition(int64(0)).Times(1)
				testShard.Resource.DomainCache.EXPECT().GetDomainName(testDomainID).Return("", testError).Times(1)
			},
			wantRepaired: false,
			wantErr:      true,
			wantErrIs:    testError,
		},
		{
			name: "checksum mismatch - repair enabled - CloseTransactionAsMutation fails",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				mockConfig.RequireChecksumMatchAfterRebuildRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				setupVersionHistories(ms)

				mockRebuiltMS := NewMockMutableState(ctrl)
				mockRebuiltMS.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				}).AnyTimes()
				mockRebuiltMS.EXPECT().GetVersionHistories().Return(nil).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{}).AnyTimes()

				sr.EXPECT().Rebuild(
					gomock.Any(), gomock.Any(), gomock.Any(),
					[]byte(testBranchToken), int64(9), int64(1),
					gomock.Any(), gomock.Any(), "",
				).Return(mockRebuiltMS, int64(0), nil).Times(1)

				mockRebuiltMS.EXPECT().SetHistorySize(int64(0)).Times(1)
				mockRebuiltMS.EXPECT().SetUpdateCondition(int64(0)).Times(1)
				mockRebuiltMS.EXPECT().CloseTransactionAsMutation(gomock.Any(), TransactionPolicyPassive).Return(nil, nil, testError).Times(1)
				testShard.Resource.DomainCache.EXPECT().GetDomainName(testDomainID).Return(testDomainName, nil).Times(1)
			},
			wantRepaired: false,
			wantErr:      true,
			wantErrIs:    testError,
		},
		{
			name: "checksum mismatch - repair enabled - CloseTransactionAsMutation returns unexpected events",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				mockConfig.RequireChecksumMatchAfterRebuildRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				setupVersionHistories(ms)

				mockRebuiltMS := NewMockMutableState(ctrl)
				mockRebuiltMS.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				}).AnyTimes()
				mockRebuiltMS.EXPECT().GetVersionHistories().Return(nil).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{}).AnyTimes()

				sr.EXPECT().Rebuild(
					gomock.Any(), gomock.Any(), gomock.Any(),
					[]byte(testBranchToken), int64(9), int64(1),
					gomock.Any(), gomock.Any(), "",
				).Return(mockRebuiltMS, int64(0), nil).Times(1)

				mockRebuiltMS.EXPECT().SetHistorySize(int64(0)).Times(1)
				mockRebuiltMS.EXPECT().SetUpdateCondition(int64(0)).Times(1)
				mockRebuiltMS.EXPECT().GetHistorySize().Return(int64(1024)).Times(1)
				mockRebuiltMS.EXPECT().CloseTransactionAsMutation(gomock.Any(), TransactionPolicyPassive).Return(
					&persistence.WorkflowMutation{ExecutionInfo: &persistence.WorkflowExecutionInfo{DomainID: testDomainID}},
					[]*persistence.WorkflowEvents{{DomainID: testDomainID}},
					nil,
				).Times(1)
				testShard.Resource.DomainCache.EXPECT().GetDomainName(testDomainID).Return(testDomainName, nil).Times(1)
			},
			wantRepaired: false,
			wantErr:      true,
		},
		{
			name: "checksum mismatch - repair enabled - UpdateWorkflowExecution fails",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				mockConfig.RequireChecksumMatchAfterRebuildRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				setupVersionHistories(ms)

				mockRebuiltMS := NewMockMutableState(ctrl)
				mockRebuiltMS.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				}).AnyTimes()
				mockRebuiltMS.EXPECT().GetVersionHistories().Return(nil).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{}).AnyTimes()
				mockRebuiltMS.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{}).AnyTimes()

				sr.EXPECT().Rebuild(
					gomock.Any(), gomock.Any(), gomock.Any(),
					[]byte(testBranchToken), int64(9), int64(1),
					gomock.Any(), gomock.Any(), "",
				).Return(mockRebuiltMS, int64(0), nil).Times(1)

				mockRebuiltMS.EXPECT().SetHistorySize(int64(0)).Times(1)
				mockRebuiltMS.EXPECT().SetUpdateCondition(int64(0)).Times(1)
				mockRebuiltMS.EXPECT().GetHistorySize().Return(int64(1024)).Times(1)
				mockRebuiltMS.EXPECT().CloseTransactionAsMutation(gomock.Any(), TransactionPolicyPassive).Return(
					&persistence.WorkflowMutation{ExecutionInfo: &persistence.WorkflowExecutionInfo{DomainID: testDomainID}},
					nil, nil,
				).Times(1)
				testShard.Resource.DomainCache.EXPECT().GetDomainName(testDomainID).Return(testDomainName, nil).Times(1)
				testShard.Resource.DomainCache.EXPECT().GetDomainByID(testDomainID).Return(cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: testDomainID, Name: testDomainName},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{
						ActiveClusterName: "active",
						Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: "active"}},
					},
					1234,
				), nil).Times(1)
				testShard.Resource.ExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, testPersistenceError).Once()
			},
			wantRepaired: false,
			wantErr:      true,
			wantErrIs:    testPersistenceError,
		},
		{
			name: "checksum mismatch - repair enabled and succeeds - checksum matches after rebuild",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				mockConfig.RequireChecksumMatchAfterRebuildRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				setupVersionHistories(ms)
				setupSuccessfulRebuild(ctrl, sr, true)
				setupDomainCacheMocks(testShard)
				testShard.Resource.ExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{}, nil).Once()
			},
			wantRepaired: true,
		},
		{
			name: "checksum mismatch - repair enabled and succeeds - checksum mismatch after rebuild (RequireChecksumMatchAfterRebuildRepair disabled)",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				mockConfig.RequireChecksumMatchAfterRebuildRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
				setupDetectionMocks(ms, testDomainID, testWorkflowID, testRunID)
				setupVersionHistories(ms)
				setupSuccessfulRebuild(ctrl, sr, false)
				setupDomainCacheMocks(testShard)
				testShard.Resource.ExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{}, nil).Once()
			},
			wantRepaired: true,
		},
		{
			// Covers the MutableStateRebuildChecksumMatch counter (line 298 in workflow_repairer.go).
			// The stored checksum equals what the rebuilt state produces (matchingChecksum), but the
			// original mutable state has non-nil version histories so its fresh checksum differs from
			// the stored one — corruption is detected, repair runs, and the rebuilt checksum matches.
			name: "checksum mismatch - repair enabled and succeeds - rebuilt checksum matches persisted checksum",
			setupFunc: func(ctrl *gomock.Controller, testShard *shard.TestContext, mockConfig *config.Config, ms *MockMutableState, sr *MockStateRebuilder) {
				mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
				mockConfig.EnableCorruptionAutoRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
				mockConfig.CorruptionRepairTimeout = dynamicproperties.GetDurationPropertyFnFilteredByDomain(testTimeout)
				mockConfig.RequireChecksumMatchAfterRebuildRepair = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)

				// Stored checksum = matchingChecksum (same as what the rebuild will produce).
				ms.EXPECT().GetChecksum().Return(matchingChecksum).Times(1)
				ms.EXPECT().GetDomainEntry().Return(cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: testDomainID, Name: testDomainName},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{ActiveClusterName: "active"},
					0,
				)).AnyTimes()
				ms.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				}).AnyTimes()
				// Non-nil version histories are included in the checksum payload, so
				// generateMutableStateChecksum(ms) != matchingChecksum → corruption detected.
				ms.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories: []*persistence.VersionHistory{
						{
							BranchToken: []byte(testBranchToken),
							Items:       []*persistence.VersionHistoryItem{{EventID: 9, Version: 1}},
						},
					},
				}).AnyTimes()
				ms.EXPECT().GetCurrentBranchToken().Return([]byte(testBranchToken), nil).Times(1)
				ms.EXPECT().GetPendingTimerInfos().Return(map[string]*persistence.TimerInfo{}).AnyTimes()
				ms.EXPECT().GetPendingActivityInfos().Return(map[int64]*persistence.ActivityInfo{}).AnyTimes()
				ms.EXPECT().GetPendingChildExecutionInfos().Return(map[int64]*persistence.ChildExecutionInfo{}).AnyTimes()
				ms.EXPECT().GetPendingRequestCancelExternalInfos().Return(map[int64]*persistence.RequestCancelInfo{}).AnyTimes()
				ms.EXPECT().GetPendingSignalExternalInfos().Return(map[int64]*persistence.SignalInfo{}).AnyTimes()

				// Rebuilt state has nil version histories + standard exec info → checksum = matchingChecksum = stored.
				setupSuccessfulRebuild(ctrl, sr, true)
				setupDomainCacheMocks(testShard)
				testShard.Resource.ExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{}, nil).Once()
			},
			wantRepaired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ms := NewMockMutableState(ctrl)
			mockLogger := log.NewNoop()
			mockMetricsClient := metrics.NewClient(tally.NoopScope, metrics.History, metrics.MigrationConfig{})
			mockConfig := config.NewForTest()
			mockConfig.MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
			sr := NewMockStateRebuilder(ctrl)

			testShard := shard.NewTestContext(
				t,
				ctrl,
				&persistence.ShardInfo{ShardID: 0, RangeID: 1},
				mockConfig,
			)

			tt.setupFunc(ctrl, testShard, mockConfig, ms, sr)

			repairer := &workflowRepairerImpl{
				shard:          testShard,
				stateRebuilder: sr,
				logger:         mockLogger,
				metricsClient:  mockMetricsClient,
				scope:          mockMetricsClient.Scope(metrics.WorkflowCorruptionRepairScope),
			}

			result, err := repairer.VerifyAndRepairWorkflowIfNeeded(context.Background(), ms)

			require.Equal(t, tt.wantRepaired, result)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					require.ErrorIs(t, err, tt.wantErrIs)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
