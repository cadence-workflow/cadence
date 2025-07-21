// Copyright (c) 2020 Uber Technologies, Inc.
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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/testing/testdatagen/idlfuzzedtestdata"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/testdata"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/shard"
)

func TestCopyActivityInfo(t *testing.T) {
	t.Run("test CopyActivityInfo Mapping", func(t *testing.T) {
		f := idlfuzzedtestdata.NewFuzzerWithIDLTypes(t)

		d1 := persistence.ActivityInfo{}
		f.Fuzz(&d1)
		d2 := CopyActivityInfo(t, &d1)

		assert.Equal(t, &d1, d2)
	})
}

func TestCopyWorkflowExecutionInfo(t *testing.T) {
	t.Run("test ExecutionInfo Mapping", func(t *testing.T) {
		f := idlfuzzedtestdata.NewFuzzerWithIDLTypes(t)

		d1 := persistence.WorkflowExecutionInfo{}
		f.Fuzz(&d1)
		d2 := CopyWorkflowExecutionInfo(t, &d1)

		assert.Equal(t, &d1, d2)
	})
}

func TestCopyTimerInfoMapping(t *testing.T) {
	t.Run("test Timer info Mapping", func(t *testing.T) {
		f := idlfuzzedtestdata.NewFuzzerWithIDLTypes(t)

		d1 := persistence.TimerInfo{}
		f.Fuzz(&d1)
		d2 := CopyTimerInfo(t, &d1)

		assert.Equal(t, &d1, d2)
	})
}

func TestChildWorkflowMapping(t *testing.T) {
	t.Run("test child workflwo info Mapping", func(t *testing.T) {
		f := idlfuzzedtestdata.NewFuzzerWithIDLTypes(t)

		d1 := persistence.ChildExecutionInfo{}
		f.Fuzz(&d1)
		d2 := CopyChildInfo(t, &d1)

		assert.Equal(t, &d1, d2)
	})
}

func TestCopySignalInfo(t *testing.T) {
	t.Run("test signal info Mapping", func(t *testing.T) {
		f := idlfuzzedtestdata.NewFuzzerWithIDLTypes(t)

		d1 := persistence.SignalInfo{}
		f.Fuzz(&d1)
		d2 := CopySignalInfo(t, &d1)

		assert.Equal(t, &d1, d2)
	})
}

func TestCopyCancellationInfo(t *testing.T) {
	t.Run("test signal info Mapping", func(t *testing.T) {
		f := idlfuzzedtestdata.NewFuzzerWithIDLTypes(t)

		d1 := persistence.RequestCancelInfo{}
		f.Fuzz(&d1)
		d2 := CopyCancellationInfo(t, &d1)

		assert.Equal(t, &d1, d2)
	})
}

func TestFindAutoResetPoint(t *testing.T) {
	timeSource := clock.NewRealTimeSource()

	// case 1: nil
	_, pt := FindAutoResetPoint(timeSource, nil, nil)
	assert.Nil(t, pt)

	// case 2: empty
	_, pt = FindAutoResetPoint(timeSource, &types.BadBinaries{}, &types.ResetPoints{})
	assert.Nil(t, pt)

	pt0 := &types.ResetPointInfo{
		BinaryChecksum: "abc",
		Resettable:     true,
	}
	pt1 := &types.ResetPointInfo{
		BinaryChecksum: "def",
		Resettable:     true,
	}
	pt3 := &types.ResetPointInfo{
		BinaryChecksum: "ghi",
		Resettable:     false,
	}

	expiredNowNano := time.Now().UnixNano() - int64(time.Hour)
	notExpiredNowNano := time.Now().UnixNano() + int64(time.Hour)
	pt4 := &types.ResetPointInfo{
		BinaryChecksum:   "expired",
		Resettable:       true,
		ExpiringTimeNano: common.Int64Ptr(expiredNowNano),
	}

	pt5 := &types.ResetPointInfo{
		BinaryChecksum:   "notExpired",
		Resettable:       true,
		ExpiringTimeNano: common.Int64Ptr(notExpiredNowNano),
	}

	// case 3: two intersection
	_, pt = FindAutoResetPoint(timeSource, &types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"abc": {},
			"def": {},
		},
	}, &types.ResetPoints{
		Points: []*types.ResetPointInfo{
			pt0, pt1, pt3,
		},
	})
	assert.Equal(t, pt, pt0)

	// case 4: one intersection
	_, pt = FindAutoResetPoint(timeSource, &types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"none":    {},
			"def":     {},
			"expired": {},
		},
	}, &types.ResetPoints{
		Points: []*types.ResetPointInfo{
			pt4, pt0, pt1, pt3,
		},
	})
	assert.Equal(t, pt, pt1)

	// case 4: no intersection
	_, pt = FindAutoResetPoint(timeSource, &types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"none1": {},
			"none2": {},
		},
	}, &types.ResetPoints{
		Points: []*types.ResetPointInfo{
			pt0, pt1, pt3,
		},
	})
	assert.Nil(t, pt)

	// case 5: not resettable
	_, pt = FindAutoResetPoint(timeSource, &types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"none1": {},
			"ghi":   {},
		},
	}, &types.ResetPoints{
		Points: []*types.ResetPointInfo{
			pt0, pt1, pt3,
		},
	})
	assert.Nil(t, pt)

	// case 6: one intersection of expired
	_, pt = FindAutoResetPoint(timeSource, &types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"none":    {},
			"expired": {},
		},
	}, &types.ResetPoints{
		Points: []*types.ResetPointInfo{
			pt0, pt1, pt3, pt4, pt5,
		},
	})
	assert.Nil(t, pt)

	// case 7: one intersection of not expired
	_, pt = FindAutoResetPoint(timeSource, &types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"none":       {},
			"notExpired": {},
		},
	}, &types.ResetPoints{
		Points: []*types.ResetPointInfo{
			pt0, pt1, pt3, pt4, pt5,
		},
	})
	assert.Equal(t, pt, pt5)
}

func TestTrimBinaryChecksums(t *testing.T) {
	testCases := []struct {
		name           string
		maxResetPoints int
		expected       []string
	}{
		{
			name:           "not reach limit",
			maxResetPoints: 6,
			expected:       []string{"checksum1", "checksum2", "checksum3", "checksum4", "checksum5"},
		},
		{
			name:           "reach at limit",
			maxResetPoints: 5,
			expected:       []string{"checksum2", "checksum3", "checksum4", "checksum5"},
		},
		{
			name:           "exceeds limit",
			maxResetPoints: 2,
			expected:       []string{"checksum5"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			currResetPoints := []*types.ResetPointInfo{
				{
					BinaryChecksum: "checksum1",
					RunID:          "run1",
				},
				{
					BinaryChecksum: "checksum2",
					RunID:          "run2",
				},
				{
					BinaryChecksum: "checksum3",
					RunID:          "run3",
				},
				{
					BinaryChecksum: "checksum4",
					RunID:          "run4",
				},
				{
					BinaryChecksum: "checksum5",
					RunID:          "run5",
				},
			}
			var recentBinaryChecksums []string
			for _, rp := range currResetPoints {
				recentBinaryChecksums = append(recentBinaryChecksums, rp.GetBinaryChecksum())
			}
			recentBinaryChecksums, currResetPoints = trimBinaryChecksums(recentBinaryChecksums, currResetPoints, tc.maxResetPoints)
			assert.Equal(t, tc.expected, recentBinaryChecksums)
			assert.Equal(t, len(tc.expected), len(currResetPoints))
		})
	}

	// test empty case
	currResetPoints := make([]*types.ResetPointInfo, 0, 1)
	var recentBinaryChecksums []string
	for _, rp := range currResetPoints {
		recentBinaryChecksums = append(recentBinaryChecksums, rp.GetBinaryChecksum())
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("The function panicked: %v", r)
		}
	}()
	trimedBinaryChecksums, trimedResetPoints := trimBinaryChecksums(recentBinaryChecksums, currResetPoints, 2)
	assert.Equal(t, recentBinaryChecksums, trimedBinaryChecksums)
	assert.Equal(t, currResetPoints, trimedResetPoints)
}

func TestConvertWorkflowRequests(t *testing.T) {
	inputs := map[persistence.WorkflowRequest]struct{}{}
	inputs[persistence.WorkflowRequest{RequestID: "aaa", Version: 1, RequestType: persistence.WorkflowRequestTypeStart}] = struct{}{}
	inputs[persistence.WorkflowRequest{RequestID: "aaa", Version: 1, RequestType: persistence.WorkflowRequestTypeSignal}] = struct{}{}

	expectedOutputs := []*persistence.WorkflowRequest{
		{
			RequestID:   "aaa",
			Version:     1,
			RequestType: persistence.WorkflowRequestTypeStart,
		},
		{
			RequestID:   "aaa",
			Version:     1,
			RequestType: persistence.WorkflowRequestTypeSignal,
		},
	}

	assert.ElementsMatch(t, expectedOutputs, convertWorkflowRequests(inputs))
}

func TestCreatePersistenceMutableState(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockShardContext := shard.NewMockContext(ctrl)
	logger := testlogger.New(t)
	mockEventsCache := events.NewMockCache(ctrl)
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockShardContext.EXPECT().GetClusterMetadata().Return(constants.TestClusterMetadata).Times(2)
	mockShardContext.EXPECT().GetEventsCache().Return(mockEventsCache)
	mockShardContext.EXPECT().GetConfig().Return(config.NewForTest())
	mockShardContext.EXPECT().GetTimeSource().Return(clock.NewMockedTimeSource())
	mockShardContext.EXPECT().GetMetricsClient().Return(metrics.NewClient(tally.NoopScope, metrics.History))
	mockShardContext.EXPECT().GetDomainCache().Return(mockDomainCache)
	mockShardContext.EXPECT().GetLogger().Return(logger).AnyTimes()

	builder := newMutableStateBuilder(mockShardContext, logger, constants.TestLocalDomainEntry)
	builder.pendingActivityInfoIDs[0] = &persistence.ActivityInfo{}
	builder.pendingTimerInfoIDs["some-key"] = &persistence.TimerInfo{}
	builder.pendingSignalInfoIDs[0] = &persistence.SignalInfo{}
	builder.pendingChildExecutionInfoIDs[0] = &persistence.ChildExecutionInfo{}
	builder.bufferedEvents = []*types.HistoryEvent{{}}
	builder.updateBufferedEvents = []*types.HistoryEvent{{}}
	builder.versionHistories = &persistence.VersionHistories{CurrentVersionHistoryIndex: 0, Histories: []*persistence.VersionHistory{}}
	builder.pendingRequestCancelInfoIDs[0] = &persistence.RequestCancelInfo{}
	builder.decisionTaskManager.(*mutableStateDecisionTaskManagerImpl).HasInFlightDecision()
	builder.executionInfo.DecisionStartedID = 1

	mutableState := CreatePersistenceMutableState(t, builder)
	assert.NotNil(t, mutableState)
	assert.Equal(t, builder.executionInfo, mutableState.ExecutionInfo)
	assert.Equal(t, builder.pendingActivityInfoIDs, mutableState.ActivityInfos)
	assert.Equal(t, builder.pendingSignalInfoIDs, mutableState.SignalInfos)
	assert.Equal(t, builder.pendingTimerInfoIDs, mutableState.TimerInfos)
	assert.Equal(t, builder.pendingRequestCancelInfoIDs, mutableState.RequestCancelInfos)
	assert.Equal(t, len(builder.bufferedEvents)+len(builder.updateBufferedEvents), len(mutableState.BufferedEvents))
}

func TestGetChildExecutionDomainName(t *testing.T) {
	t.Run("nonempty domain ID", func(t *testing.T) {
		childInfo := &persistence.ChildExecutionInfo{DomainID: testdata.DomainID}
		mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
		expected := testdata.DomainName
		mockDomainCache.EXPECT().GetDomainName(childInfo.DomainID).Return(expected, nil)
		name, err := GetChildExecutionDomainName(childInfo, mockDomainCache, constants.TestLocalDomainEntry)
		require.NoError(t, err)
		assert.Equal(t, expected, name)
	})

	t.Run("nonempty domain name", func(t *testing.T) {
		childInfo := &persistence.ChildExecutionInfo{DomainNameDEPRECATED: testdata.DomainName}
		parentDomainEntry := constants.TestLocalDomainEntry
		mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
		name, err := GetChildExecutionDomainName(childInfo, mockDomainCache, parentDomainEntry)
		require.NoError(t, err)
		assert.Equal(t, testdata.DomainName, name)
	})

	t.Run("empty childInfo", func(t *testing.T) {
		childInfo := &persistence.ChildExecutionInfo{}
		parentDomainEntry := constants.TestLocalDomainEntry
		mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
		name, err := GetChildExecutionDomainName(childInfo, mockDomainCache, parentDomainEntry)
		require.NoError(t, err)
		assert.Equal(t, parentDomainEntry.GetInfo().Name, name)
	})
}

func TestGetChildExecutionDomainID(t *testing.T) {
	t.Run("nonempty domain ID", func(t *testing.T) {
		childInfo := &persistence.ChildExecutionInfo{DomainID: testdata.DomainID}
		mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
		name, err := GetChildExecutionDomainID(childInfo, mockDomainCache, constants.TestLocalDomainEntry)
		require.NoError(t, err)
		assert.Equal(t, testdata.DomainID, name)
	})

	t.Run("nonempty domain name", func(t *testing.T) {
		childInfo := &persistence.ChildExecutionInfo{DomainNameDEPRECATED: testdata.DomainName}
		parentDomainEntry := constants.TestLocalDomainEntry
		mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
		mockDomainCache.EXPECT().GetDomainID(testdata.DomainName).Return(testdata.DomainID, nil)
		name, err := GetChildExecutionDomainID(childInfo, mockDomainCache, parentDomainEntry)
		require.NoError(t, err)
		assert.Equal(t, testdata.DomainID, name)
	})

	t.Run("empty childInfo", func(t *testing.T) {
		childInfo := &persistence.ChildExecutionInfo{}
		parentDomainEntry := constants.TestLocalDomainEntry
		mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
		name, err := GetChildExecutionDomainID(childInfo, mockDomainCache, parentDomainEntry)
		require.NoError(t, err)
		assert.Equal(t, parentDomainEntry.GetInfo().ID, name)
	})
}

func TestGetChildExecutionDomainEntry(t *testing.T) {
	t.Run("nonempty domain ID", func(t *testing.T) {
		childInfo := &persistence.ChildExecutionInfo{DomainID: testdata.DomainID}
		mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
		expected := &cache.DomainCacheEntry{}
		mockDomainCache.EXPECT().GetDomainByID(childInfo.DomainID).Return(expected, nil)
		entry, err := GetChildExecutionDomainEntry(t, childInfo, mockDomainCache, constants.TestLocalDomainEntry)
		require.NoError(t, err)
		assert.Equal(t, expected, entry)
	})

	t.Run("nonempty domain name", func(t *testing.T) {
		childInfo := &persistence.ChildExecutionInfo{DomainNameDEPRECATED: testdata.DomainName}
		parentDomainEntry := constants.TestLocalDomainEntry
		mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
		expected := &cache.DomainCacheEntry{}
		mockDomainCache.EXPECT().GetDomain(childInfo.DomainNameDEPRECATED).Return(expected, nil)
		entry, err := GetChildExecutionDomainEntry(t, childInfo, mockDomainCache, parentDomainEntry)
		require.NoError(t, err)
		assert.Equal(t, expected, entry)
	})

	t.Run("empty childInfo", func(t *testing.T) {
		childInfo := &persistence.ChildExecutionInfo{}
		parentDomainEntry := constants.TestLocalDomainEntry
		mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
		entry, err := GetChildExecutionDomainEntry(t, childInfo, mockDomainCache, parentDomainEntry)
		require.NoError(t, err)
		assert.Equal(t, parentDomainEntry, entry)
	})
}

func TestConvert(t *testing.T) {
	t.Run("convertSyncActivityInfos", func(t *testing.T) {
		executionInfo := &persistence.WorkflowExecutionInfo{
			DomainID:   "some-domain-id",
			WorkflowID: "some-workflow-id",
			RunID:      "some-run-id",
		}
		activityInfos := map[int64]*persistence.ActivityInfo{1: {Version: 1, ScheduleID: 1}}
		inputs := map[int64]struct{}{1: {}}
		outputs := convertSyncActivityInfos(executionInfo, activityInfos, inputs)
		assert.NotNil(t, outputs)
		assert.Equal(t, 1, len(outputs))
		assert.Equal(t, int64(1), outputs[0].(*persistence.SyncActivityTask).ScheduledID)
		assert.Equal(t, int64(1), outputs[0].GetVersion())
		assert.Equal(t, executionInfo.DomainID, outputs[0].(*persistence.SyncActivityTask).DomainID)
		assert.Equal(t, executionInfo.WorkflowID, outputs[0].(*persistence.SyncActivityTask).WorkflowID)
		assert.Equal(t, executionInfo.RunID, outputs[0].(*persistence.SyncActivityTask).RunID)
	})
}

func TestScheduleDecision(t *testing.T) {
	t.Run("mutable state has pending decision", func(t *testing.T) {
		mockMutableState := NewMockMutableState(gomock.NewController(t))
		mockMutableState.EXPECT().HasPendingDecision().Return(true)
		err := ScheduleDecision(mockMutableState)
		require.NoError(t, err)
	})

	t.Run("internal service error", func(t *testing.T) {
		mockMutableState := NewMockMutableState(gomock.NewController(t))
		mockMutableState.EXPECT().HasPendingDecision().Return(false)
		mockMutableState.EXPECT().AddDecisionTaskScheduledEvent(false).Return(nil, errors.New("some error"))
		err := ScheduleDecision(mockMutableState)
		assert.NotNil(t, err)
		assert.Equal(t, "Failed to add decision scheduled event.", err.Error())
	})

	t.Run("success", func(t *testing.T) {
		mockMutableState := NewMockMutableState(gomock.NewController(t))
		mockMutableState.EXPECT().HasPendingDecision().Return(false)
		mockMutableState.EXPECT().AddDecisionTaskScheduledEvent(false).Return(nil, nil)
		err := ScheduleDecision(mockMutableState)
		require.NoError(t, err)
	})
}

func TestFailDecision(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockMutableState := NewMockMutableState(gomock.NewController(t))
		decision := &DecisionInfo{}
		failureCause := new(types.DecisionTaskFailedCause)
		mockMutableState.EXPECT().AddDecisionTaskFailedEvent(
			decision.ScheduleID,
			decision.StartedID,
			*failureCause,
			nil,
			IdentityHistoryService,
			"",
			"",
			"",
			"",
			int64(0),
			"",
		).Return(nil, nil)
		mockMutableState.EXPECT().FlushBufferedEvents() // only on success
		err := FailDecision(mockMutableState, decision, *failureCause)
		require.NoError(t, err)
	})

	t.Run("failure", func(t *testing.T) {
		mockMutableState := NewMockMutableState(gomock.NewController(t))
		decision := &DecisionInfo{}
		failureCause := new(types.DecisionTaskFailedCause)
		mockMutableState.EXPECT().AddDecisionTaskFailedEvent(
			decision.ScheduleID,
			decision.StartedID,
			*failureCause,
			nil,
			IdentityHistoryService,
			"",
			"",
			"",
			"",
			int64(0),
			"",
		).Return(nil, errors.New("some error adding failed event"))
		err := FailDecision(mockMutableState, decision, *failureCause)
		assert.NotNil(t, err)
		assert.Equal(t, "some error adding failed event", err.Error())
	})
}
