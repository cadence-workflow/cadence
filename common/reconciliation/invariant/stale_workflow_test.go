// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package invariant

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/types"
)

func setup(t *testing.T, retentionDays int) (*persistence.MockRetryer, *staleWorkflowCheck) {
	ctrl := gomock.NewController(t)
	pr := persistence.NewMockRetryer(ctrl)
	dc := cache.NewMockDomainCache(ctrl)
	domainEntry := cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: domainID, Name: domainName},
		&persistence.DomainConfig{Retention: int32(retentionDays)},
		true,
		nil,
		0,
		nil,
		0,
		0,
		0)
	dc.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil).AnyTimes()
	dc.EXPECT().GetDomainName(domainID).Return(domainName, nil).AnyTimes()

	invariant := NewStaleWorkflow(pr, dc, testlogger.NewZap(t))
	impl, ok := invariant.(*staleWorkflowCheck)
	if !ok {
		t.Fatalf("NewStaleWorkflowVersion returned invariant of type %T but want *staleWorkflowCheck", invariant)
	}
	return pr, impl
}

// some acceptable defaults for tests that don't care about precise windows
const (
	testRetentionDays              = 5
	testRetentionAfterSafetyMargin = (time.Duration(testRetentionDays) * 24 * time.Hour) + retentionSafetyMargin
	oneDay                         = 24 * time.Hour
)

// This test case validates the Check method for stale workflow invariant is doing the correct thing at high level.
// Rest of the tests in this file covers the internals of CheckAge which is called by Check.
func TestCheck(t *testing.T) {
	tests := []struct {
		desc       string
		execution  any
		mockFn     func(*persistence.MockRetryer)
		wantResult CheckResult
	}{
		{
			desc: "corrupted workflow past expiration",
			execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					WorkflowID: workflowID,
					RunID:      runID,
					DomainID:   domainID,
				},
			},
			mockFn: func(pr *persistence.MockRetryer) {
				pr.EXPECT().GetWorkflowExecution(gomock.Any(), &persistence.GetWorkflowExecutionRequest{
					DomainID: domainID,
					Execution: types.WorkflowExecution{
						WorkflowID: workflowID,
						RunID:      runID,
					},
				}).Return(execution(domainID, running, time.Now().Add(-testRetentionAfterSafetyMargin-2*time.Hour), time.Hour, nil), nil).Times(1)

				pr.EXPECT().GetShardID().Return(123).Times(1)
				pr.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).
					Return(&persistence.ReadHistoryBranchResponse{
						HistoryEvents: []*types.HistoryEvent{
							{
								WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
							},
						},
					}, nil).Times(1)
			},
			wantResult: CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   "stale_workflow",
				Info:            "stale running workflow",
			},
		},
		{
			desc: "normal healthy workflow",
			execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					WorkflowID: workflowID,
					RunID:      runID,
					DomainID:   domainID,
				},
			},
			mockFn: func(pr *persistence.MockRetryer) {
				pr.EXPECT().GetWorkflowExecution(gomock.Any(), &persistence.GetWorkflowExecutionRequest{
					DomainID: domainID,
					Execution: types.WorkflowExecution{
						WorkflowID: workflowID,
						RunID:      runID,
					},
				}).Return(execution(domainID, running, time.Now().Add(-2*time.Hour), time.Hour, nil), nil).Times(1)

				pr.EXPECT().GetShardID().Return(123).Times(1)
				pr.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).
					Return(&persistence.ReadHistoryBranchResponse{
						HistoryEvents: []*types.HistoryEvent{
							{
								WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
							},
						},
					}, nil).Times(1)
			},
			wantResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   "stale_workflow",
				Info:            "running workflow is within expiration window",
			},
		},
		{
			desc: "zombie healthy workflow",
			execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					WorkflowID: workflowID,
					RunID:      runID,
					DomainID:   domainID,
				},
			},
			mockFn: func(pr *persistence.MockRetryer) {
				pr.EXPECT().GetWorkflowExecution(gomock.Any(), &persistence.GetWorkflowExecutionRequest{
					DomainID: domainID,
					Execution: types.WorkflowExecution{
						WorkflowID: workflowID,
						RunID:      runID,
					},
				}).Return(execution(domainID, zombie, time.Now().Add(-2*time.Hour), time.Hour, nil), nil).Times(1)

				pr.EXPECT().GetShardID().Return(123).Times(1)
				pr.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).
					Return(&persistence.ReadHistoryBranchResponse{
						HistoryEvents: []*types.HistoryEvent{
							{
								WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
							},
						},
					}, nil).Times(1)
			},
			wantResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   "stale_workflow",
				Info:            "zombie workflow is within expiration window",
			},
		},
		{
			desc:      "invalid execution object type",
			execution: &entity.Timer{}, // invalid object type
			mockFn:    func(pr *persistence.MockRetryer) {},
			wantResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   "stale_workflow",
				Info:            "expected concrete execution",
			},
		},
		{
			desc: "missing workflow id",
			execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					WorkflowID: "", // missing workflow id
					RunID:      runID,
					DomainID:   domainID,
				},
			},
			mockFn: func(pr *persistence.MockRetryer) {},
			wantResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   "stale_workflow",
				Info:            "missing critical data",
			},
		},
		{
			desc: "get workflow execution failed",
			execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					WorkflowID: workflowID,
					RunID:      runID,
					DomainID:   domainID,
				},
			},
			mockFn: func(pr *persistence.MockRetryer) {
				pr.EXPECT().GetWorkflowExecution(gomock.Any(), &persistence.GetWorkflowExecutionRequest{
					DomainID: domainID,
					Execution: types.WorkflowExecution{
						WorkflowID: workflowID,
						RunID:      runID,
					},
				}).Return(nil, fmt.Errorf("failed")).Times(1)
			},
			wantResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   "stale_workflow",
				Info:            "failed to get concrete execution record",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			pr, impl := setup(t, testRetentionDays)

			tc.mockFn(pr)

			gotResult := impl.Check(context.Background(), tc.execution)
			gotResult.InfoDetails = "" // ignore details for now
			assert.Equal(t, tc.wantResult, gotResult)
		})
	}
}

func TestFix(t *testing.T) {
	tests := []struct {
		desc       string
		execution  any
		mockFn     func(*persistence.MockRetryer)
		wantResult FixResult
	}{
		{
			desc: "healthy workflow skipped",
			execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					WorkflowID: workflowID,
					RunID:      runID,
					DomainID:   domainID,
				},
			},
			mockFn: func(pr *persistence.MockRetryer) {
				pr.EXPECT().GetWorkflowExecution(gomock.Any(), &persistence.GetWorkflowExecutionRequest{
					DomainID: domainID,
					Execution: types.WorkflowExecution{
						WorkflowID: workflowID,
						RunID:      runID,
					},
				}).Return(execution(domainID, running, time.Now().Add(-2*time.Hour), time.Hour, nil), nil).Times(1)

				pr.EXPECT().GetShardID().Return(123).Times(1)
				pr.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).
					Return(&persistence.ReadHistoryBranchResponse{
						HistoryEvents: []*types.HistoryEvent{
							{
								WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
							},
						},
					}, nil).Times(1)
			},
			wantResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: "stale_workflow",
				Info:          "no need to fix: running workflow is within expiration window",
			},
		},
		{
			desc: "corrupted workflow deleted",
			execution: &entity.ConcreteExecution{
				Execution: entity.Execution{
					WorkflowID: workflowID,
					RunID:      runID,
					DomainID:   domainID,
				},
			},
			mockFn: func(pr *persistence.MockRetryer) {
				pr.EXPECT().GetWorkflowExecution(gomock.Any(), &persistence.GetWorkflowExecutionRequest{
					DomainID: domainID,
					Execution: types.WorkflowExecution{
						WorkflowID: workflowID,
						RunID:      runID,
					},
				}).Return(execution(domainID, running, time.Now().Add(-testRetentionAfterSafetyMargin-2*time.Hour), time.Hour, nil), nil).Times(1)

				pr.EXPECT().GetShardID().Return(123).Times(1)
				pr.EXPECT().ReadHistoryBranch(gomock.Any(), gomock.Any()).
					Return(&persistence.ReadHistoryBranchResponse{
						HistoryEvents: []*types.HistoryEvent{
							{
								WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
							},
						},
					}, nil).Times(1)

				pr.EXPECT().DeleteWorkflowExecution(gomock.Any(), &persistence.DeleteWorkflowExecutionRequest{
					DomainID:   domainID,
					WorkflowID: workflowID,
					RunID:      runID,
					DomainName: domainName,
				}).Return(nil).Times(1)

				pr.EXPECT().DeleteCurrentWorkflowExecution(gomock.Any(), &persistence.DeleteCurrentWorkflowExecutionRequest{
					DomainID:   domainID,
					WorkflowID: workflowID,
					RunID:      runID,
					DomainName: domainName,
				}).Return(nil).Times(1)
			},
			wantResult: FixResult{
				FixResultType: FixResultTypeFixed,
				InvariantName: "stale_workflow",
				CheckResult: CheckResult{
					CheckResultType: CheckResultTypeCorrupted,
					InvariantName:   "stale_workflow",
					Info:            "stale running workflow",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			pr, impl := setup(t, testRetentionDays)

			tc.mockFn(pr)

			gotResult := impl.Fix(context.Background(), tc.execution)
			gotResult.CheckResult.InfoDetails = "" // ignore details for now
			gotResult.InfoDetails = ""             // ignore details for now
			assert.Equal(t, tc.wantResult, gotResult)
		})
	}
}

func TestStillRunning(t *testing.T) {
	// lots of similarity in these tests, abstract it a bit.
	run := func(created time.Time, timeout, backoff time.Duration) (pastExpiration bool, result CheckResult) {
		// sanity checks
		if timeout <= 0 {
			panic("cannot have non-positive timeouts")
		}
		if backoff < 0 {
			panic("cannot have negative backoff")
		}
		if time.Now().Before(created) {
			panic("created cannot be in the future")
		}

		pr, impl := setup(t, testRetentionDays)
		withOpenHistory(
			created,
			backoff,
			pr,
		)
		wf := execution(
			domainID,
			running,
			created,
			timeout,
			nil,
		)
		return impl.CheckAge(wf)
	}
	assertHealthy := func(t *testing.T, result CheckResult, isPastExpiration bool) {
		t.Logf("result: %#v", result)
		assert.Equal(t, CheckResultTypeHealthy, result.CheckResultType, "result should be healthy")
		assert.False(t, isPastExpiration, "workflow should not be past expiration")
	}

	t.Run("healthy", func(t *testing.T) {
		pastExpiration, result := run(
			time.Now().Add(-time.Hour),
			2*time.Hour,
			0,
		)

		// still running, still within retention window, "healthy"
		assertHealthy(t, result, pastExpiration)
	})
	t.Run("healthy-long-workflow", func(t *testing.T) {
		// specifically: this workflow runs longer than the retention window, to make sure it's not prematurely removed
		pastExpiration, result := run(
			time.Now().Add(-testRetentionAfterSafetyMargin).Add(-oneDay), // create 1 day before safety margin
			testRetentionAfterSafetyMargin+(2*oneDay),                    // expires tomorrow
			0,
		)

		// still running, still within retention window, "healthy"
		assertHealthy(t, result, pastExpiration)
	})
	t.Run("healthy-short-workflow-long-backoff", func(t *testing.T) {
		// create+timeout alone should have cleaned this up by ~1 day, but backoff means it has not yet done anything
		pastExpiration, result := run(
			time.Now().Add(-testRetentionAfterSafetyMargin).Add(-oneDay),
			time.Minute, // with only create date, should have already been cleaned up
			testRetentionAfterSafetyMargin+(2*oneDay), // but backoff means it has not actually "started" yet
		)

		assertHealthy(t, result, pastExpiration)
	})
	t.Run("bad-running-but-retained", func(t *testing.T) {
		pastExpiration, result := run(
			time.Now().Add(-time.Hour),
			time.Minute,
			0,
		)

		// still within the retention window, so it's treated as "healthy" for the purposes of this check,
		// even though it's "corrupt" because it should not still be running.
		assertHealthy(t, result, pastExpiration)
	})
	t.Run("bad-running-and-corrupt", func(t *testing.T) {
		// created a day before retention margin
		created := time.Now().Add(-testRetentionAfterSafetyMargin).Add(-oneDay)
		pastExpiration, result := run(
			created,     // created a day before retention margin
			time.Minute, // should have been deleted 23h59m ago at the latest
			0,
		)

		// should be cleaned up because it's simply too old to reasonably exist
		t.Logf("result: %#v", result)
		assert.Equal(t, CheckResultTypeCorrupted, result.CheckResultType, "result should be corrupted, as it has expired")
		assert.True(t, pastExpiration, "workflow should be past expiration, created: %v, retention days: %v + %v safety, now: %v", created.Format(dateOnly), testRetentionDays, retentionSafetyMargin, time.Now().Format(dateOnly))
	})
	t.Run("bad-running-and-backoff-and-corrupt", func(t *testing.T) {
		// created 2 days before retention margin
		created := time.Now().Add(-testRetentionAfterSafetyMargin).Add(-(2 * oneDay))
		pastExpiration, result := run(
			created,
			time.Minute,
			oneDay, // with one day backoff + timeout, should have been cleaned 23h59m ago
		)

		// should be cleaned up because it's simply too old to reasonably exist
		t.Logf("result: %#v", result)
		assert.Equal(t, CheckResultTypeCorrupted, result.CheckResultType, "result should be corrupted, as it has expired")
		assert.True(t, pastExpiration, "workflow should be past expiration, created: %v, backoff: %v, retention days: %v + %v safety, now: %v", created.Format(dateOnly), oneDay, testRetentionDays, retentionSafetyMargin, time.Now().Format(dateOnly))

	})
}

func TestPresent(t *testing.T) {
	// paranoia: nil pointers and interfaces combine in surprising ways in Go.
	var s *string
	var i *int
	assert.False(t, anyPresent(s, i), "null pointers should not be present")

	present := "non null"
	assert.True(t, anyPresent(s, i, &present), "fully populated values should be present")

	var empty string
	assert.True(t, anyPresent(s, i, &empty), "non-null empty values should still be present")
}

func TestComplete(t *testing.T) {
	// lots of similarity in these tests, abstract it a bit.
	run := func(t *testing.T, created time.Time, timeout time.Duration, completed time.Time) (pastExpiration bool, result CheckResult) {
		// sanity checks
		if timeout <= 0 {
			panic("cannot have non-positive timeouts")
		}
		if completed.Before(created) {
			panic("cannot complete before being created")
		}
		if time.Now().Before(created) {
			panic("created cannot be in the future")
		}
		if created.Add(timeout).Before(completed) {
			panic("timeout would have already completed this workflow")
		}

		pr, impl := setup(t, testRetentionDays)
		withClosedHistory(&completed, pr)

		// randomize completion time vs nil, as both seem important.  log which for reproduction purposes.
		var maybeCompleted *time.Time
		if rand.Intn(2) == 1 {
			t.Log("including completed time")
			maybeCompleted = &completed
		} else {
			t.Log("leaving completed time as nil")
		}

		wf := execution(domainID, closed, created, timeout, maybeCompleted)

		return impl.CheckAge(wf)
	}
	assertHealthy := func(t *testing.T, result CheckResult, isPastExpiration bool) {
		t.Logf("result: %#v", result)
		assert.Equal(t, CheckResultTypeHealthy, result.CheckResultType, "result should be healthy")
		assert.False(t, isPastExpiration, "workflow should not be past expiration")
	}
	assertCorrupt := func(t *testing.T, result CheckResult, isPastExpiration bool) {
		t.Logf("result: %#v", result)
		assert.Equal(t, CheckResultTypeCorrupted, result.CheckResultType, "result should be corrupted, as it has expired")
		assert.True(t, isPastExpiration, "workflow should be past expiration")
	}

	t.Run("recently-complete", func(t *testing.T) {
		startTime := time.Now().Add(-time.Hour)
		timeout := time.Hour
		timeoutTime := startTime.Add(timeout)
		pastExpiration, result := run(t, startTime, timeout, timeoutTime)

		// started and completed well within retention
		assertHealthy(t, result, pastExpiration)
	})
	t.Run("long-ago-complete", func(t *testing.T) {
		startTime := time.Now().Add(-testRetentionAfterSafetyMargin).Add(-2 * oneDay)
		timeout := time.Hour
		timeoutTime := startTime.Add(timeout)
		pastExpiration, result := run(t, startTime, timeout, timeoutTime)

		// completed long ago, still around
		assertCorrupt(t, result, pastExpiration)
	})
	t.Run("long-ago-complete-but-not-yet-timeout", func(t *testing.T) {
		startTime := time.Now().Add(-testRetentionAfterSafetyMargin).Add(-2 * oneDay) // same as above
		timeout := time.Since(startTime) + oneDay                                     // workflow timeout would fire tomorrow
		timeoutTime := startTime.Add(time.Hour)                                       // same as above

		pastExpiration, result := run(t, startTime, timeout, timeoutTime)

		// completed long ago, still around.  timeout would keep it present if it were considered.
		assertCorrupt(t, result, pastExpiration)
	})
	t.Run("long-ago-started-but-recently-complete", func(t *testing.T) {
		startTime := time.Now().Add(-testRetentionAfterSafetyMargin).Add(-2 * oneDay) // same as above
		timeout := time.Since(startTime) - oneDay                                     // workflow timeout occurred yesterday
		timeoutTime := startTime.Add(timeout)                                         // and it timed out

		pastExpiration, result := run(t, startTime, timeout, timeoutTime)

		// complete, ran longer than retention, still present
		assertHealthy(t, result, pastExpiration)
	})
	t.Run("long-long-ago-started-and-long-complete", func(t *testing.T) {
		startTime := time.Now().Add(-(2 * testRetentionAfterSafetyMargin)).Add(-2 * oneDay) // ~2x older
		timeout := 10 * testRetentionAfterSafetyMargin                                      // workflow timeout far in the future
		timeoutTime := startTime.Add(testRetentionAfterSafetyMargin).Add(oneDay)            // completed retention+oneDay ago

		pastExpiration, result := run(t, startTime, timeout, timeoutTime)

		// ran longer than retention, completed long than retention ago, should be gone
		assertCorrupt(t, result, pastExpiration)
	})
}

func TestCompleteWithRunningFallback(t *testing.T) {
	pr, impl := setup(t, testRetentionDays)
	startTime := time.Now().Add(-testRetentionAfterSafetyMargin).Add(-2 * oneDay) // started long ago
	withOpenHistoryFallback(startTime, 0, pr)
	wf := execution(
		domainID,
		closed,
		startTime,
		time.Hour, // would have timed out 1 hour after starting, outside retention
		nil,       // no completed time value
	)
	pastExpiration, result := impl.CheckAge(wf)

	t.Logf("result: %#v", result)
	assert.Equal(t, CheckResultTypeCorrupted, result.CheckResultType, "result should be corrupted, as it has expired")
	assert.True(t, pastExpiration, "workflow should be past expiration")
	assert.Contains(t, result.InfoDetails, "completed workflow", "should have recognized it's completed")
	assert.Contains(t, result.InfoDetails, "treating as running", "should have mentioned that it has fallen back to running")
}

func withClosedHistory(finish *time.Time, pr *persistence.MockRetryer) {
	pr.EXPECT().GetShardID().Return(0).MinTimes(1)

	firstPage := &persistence.ReadHistoryBranchResponse{
		HistoryEvents: []*types.HistoryEvent{
			{
				Timestamp:                               nil, // unsure, probably should be start time, but currently unused
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
			},
			{
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
			},
		},
		NextPageToken: []byte("not nil"),
	}
	var ts *int64 // always present from samples I've seen, and asserted
	if finish != nil {
		nano := finish.UnixNano()
		ts = &nano
	}
	final := &persistence.ReadHistoryBranchResponse{
		HistoryEvents: []*types.HistoryEvent{
			{
				Timestamp: ts, // always present from samples I've seen, and asserted
				WorkflowExecutionCompletedEventAttributes: &types.WorkflowExecutionCompletedEventAttributes{
					DecisionTaskCompletedEventID: 3,
				},
			},
		},
	}
	pr.EXPECT().ReadHistoryBranch(
		gomock.Any(),
		requestMaxEvent{common.EndEventID, ""}, // not a first-event-only request
	).Return(firstPage, nil).Times(1)
	pr.EXPECT().ReadHistoryBranch(
		gomock.Any(),
		requestMaxEvent{common.EndEventID, "not nil"}, // token from first page
	).Return(final, nil).Times(1)
}

func withOpenHistory(start time.Time, backoff time.Duration, pr *persistence.MockRetryer) {
	pr.EXPECT().GetShardID().Return(0).MinTimes(1)

	seconds := int32(backoff.Seconds())
	res := &persistence.ReadHistoryBranchResponse{
		HistoryEvents: []*types.HistoryEvent{
			{
				Timestamp: nil, // unsure, probably should be start time, but currently unused
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
					FirstDecisionTaskBackoffSeconds: &seconds,
				},
			},
		},
	}
	pr.EXPECT().ReadHistoryBranch(
		gomock.Any(),
		requestMaxEvent{2, ""}, // does not match a "read all history" / "get last event" request
	).Return(res, nil).Times(1)
}

// essentially incomplete-closed-then-open, but it's annoying to break them apart reusably so it's duplicated here
func withOpenHistoryFallback(start time.Time, backoff time.Duration, pr *persistence.MockRetryer) {
	closedHistory := &persistence.ReadHistoryBranchResponse{
		HistoryEvents: []*types.HistoryEvent{
			{
				Timestamp:                               nil, // unsure, probably should be start time, but currently unused
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
			},
			{
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
			},
		},
		NextPageToken: nil, // premature end of history
	}
	pr.EXPECT().ReadHistoryBranch(
		gomock.Any(),
		requestMaxEvent{common.EndEventID, ""}, // not a first-event-only request
	).Return(closedHistory, nil).Times(1)

	// and then it's just another open request, no need to customize
	withOpenHistory(start, backoff, pr)
}

type requestMaxEvent struct {
	max   int64
	token string
}

var _ gomock.Matcher = requestMaxEvent{}

func (m requestMaxEvent) Matches(x interface{}) bool {
	req, ok := x.(*persistence.ReadHistoryBranchRequest)
	if !ok {
		return false
	}
	if req.MaxEventID > m.max {
		return false // wrong max id
	}
	// nil and empty-array are both empty string, which is close enough
	return m.token == string(req.NextPageToken)
}

func (m requestMaxEvent) String() string {
	return fmt.Sprintf("max event ID must be <= %v and next page token must be %q", m.max, m.token)
}

type workflowstate int

// minor type-safety hack, we should change these iotas
const (
	running workflowstate = persistence.WorkflowStateRunning
	closed  workflowstate = persistence.WorkflowStateCompleted
	zombie  workflowstate = persistence.WorkflowStateZombie
)

func execution(domainid string, state workflowstate, created time.Time, timeout time.Duration, completed *time.Time) *persistence.GetWorkflowExecutionResponse {
	res := &persistence.GetWorkflowExecutionResponse{
		State: &persistence.WorkflowMutableState{
			// needed for many things
			ExecutionInfo: &persistence.WorkflowExecutionInfo{
				DomainID:        domainid,
				State:           int(state), /* untyped iota, persistence.WorkflowState* */
				StartTimestamp:  created,
				WorkflowTimeout: int32(timeout.Seconds()),
			},
			// needed for branch token
			VersionHistories: &persistence.VersionHistories{
				CurrentVersionHistoryIndex: 0,
				Histories: []*persistence.VersionHistory{
					{
						BranchToken: []byte("fake-branch-token"),
					},
				},
			},
		},
	}
	if completed != nil {
		// not yet seen IRL, but asserted to match the last event timestamp in full history
		nano := completed.UnixNano()
		res.State.ExecutionInfo.CompletionEvent = &types.HistoryEvent{
			Timestamp: &nano,
		}
	}
	return res
}
