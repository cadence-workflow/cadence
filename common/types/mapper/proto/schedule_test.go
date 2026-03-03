// Copyright (c) 2026 Uber Technologies, Inc.
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

package proto

import (
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
	"github.com/uber/cadence/common/types/testdata"
)

func TestScheduleSpec(t *testing.T) {
	for _, item := range []*types.ScheduleSpec{nil, {}, &testdata.ScheduleSpec} {
		assert.Equal(t, item, ToScheduleSpec(FromScheduleSpec(item)))
	}
}

func TestStartWorkflowAction(t *testing.T) {
	for _, item := range []*types.StartWorkflowAction{nil, {}, &testdata.ScheduleStartWorkflowAction} {
		assert.Equal(t, item, ToStartWorkflowAction(FromStartWorkflowAction(item)))
	}
}

func TestScheduleAction(t *testing.T) {
	for _, item := range []*types.ScheduleAction{nil, {}, &testdata.ScheduleAction} {
		assert.Equal(t, item, ToScheduleAction(FromScheduleAction(item)))
	}
}

func TestSchedulePolicies(t *testing.T) {
	for _, item := range []*types.SchedulePolicies{nil, {}, &testdata.SchedulePolicies} {
		assert.Equal(t, item, ToSchedulePolicies(FromSchedulePolicies(item)))
	}
}

func TestSchedulePauseInfo(t *testing.T) {
	for _, item := range []*types.SchedulePauseInfo{nil, {}, &testdata.SchedulePauseInfo} {
		assert.Equal(t, item, ToSchedulePauseInfo(FromSchedulePauseInfo(item)))
	}
}

func TestScheduleState(t *testing.T) {
	for _, item := range []*types.ScheduleState{nil, {}, &testdata.ScheduleState} {
		assert.Equal(t, item, ToScheduleState(FromScheduleState(item)))
	}
}

func TestBackfillInfo(t *testing.T) {
	for _, item := range []*types.BackfillInfo{nil, {}, &testdata.ScheduleBackfillInfo} {
		assert.Equal(t, item, ToBackfillInfo(FromBackfillInfo(item)))
	}
}

func TestScheduleInfo(t *testing.T) {
	for _, item := range []*types.ScheduleInfo{nil, {}, &testdata.ScheduleInfo} {
		assert.Equal(t, item, ToScheduleInfo(FromScheduleInfo(item)))
	}
}

func TestScheduleListEntry(t *testing.T) {
	for _, item := range []*types.ScheduleListEntry{nil, {}, &testdata.ScheduleListEntry} {
		assert.Equal(t, item, ToScheduleListEntry(FromScheduleListEntry(item)))
	}
}

func scheduleFuzzer(f *fuzz.Fuzzer) *fuzz.Fuzzer {
	return f.Funcs(
		func(t *time.Time, c fuzz.Continue) {
			if c.Intn(10) < 3 {
				*t = time.Time{}
				return
			}
			*t = time.Unix(c.Int63n(4102444800), c.Int63n(1e9)).UTC()
		},
		func(d *time.Duration, c fuzz.Continue) {
			if c.Intn(10) < 3 {
				*d = 0
				return
			}
			*d = time.Duration(c.Int63n(int64(24 * time.Hour)))
		},
		func(p *types.ScheduleOverlapPolicy, c fuzz.Continue) {
			*p = types.ScheduleOverlapPolicy(c.Intn(6)) // 0-5: Invalid through TerminatePrevious
		},
		func(p *types.ScheduleCatchUpPolicy, c fuzz.Continue) {
			*p = types.ScheduleCatchUpPolicy(c.Intn(4)) // 0-3: Invalid through All
		},
		func(p *types.TaskListKind, c fuzz.Continue) {
			*p = types.TaskListKind(c.Intn(3)) // 0-2: Normal, Sticky, Ephemeral
		},
	).NilChance(0.3)
}

func TestScheduleSpecFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.ScheduleSpec
		fuzzer.Fuzz(&orig)
		out := ToScheduleSpec(FromScheduleSpec(orig))
		assert.Equal(t, orig, out, "ScheduleSpec did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.CronExpression == "" && orig.StartTime.IsZero() && orig.EndTime.IsZero() && orig.Jitter == 0 {
			return "empty"
		}
		return "filled"
	})
}

func TestStartWorkflowActionFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.StartWorkflowAction
		fuzzer.Fuzz(&orig)
		out := ToStartWorkflowAction(FromStartWorkflowAction(orig))
		assert.Equal(t, orig, out, "StartWorkflowAction did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.WorkflowType == nil && orig.TaskList == nil && orig.Input == nil && orig.WorkflowIDPrefix == "" {
			return "empty"
		}
		return "filled"
	})
}

func TestScheduleActionFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.ScheduleAction
		fuzzer.Fuzz(&orig)
		out := ToScheduleAction(FromScheduleAction(orig))
		assert.Equal(t, orig, out, "ScheduleAction did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.StartWorkflow == nil {
			return "empty"
		}
		return "filled"
	})
}

func TestSchedulePoliciesFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.SchedulePolicies
		fuzzer.Fuzz(&orig)
		out := ToSchedulePolicies(FromSchedulePolicies(orig))
		assert.Equal(t, orig, out, "SchedulePolicies did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.OverlapPolicy == 0 && orig.CatchUpPolicy == 0 && orig.CatchUpWindow == 0 && !orig.PauseOnFailure {
			return "empty"
		}
		return "filled"
	})
}

func TestSchedulePauseInfoFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.SchedulePauseInfo
		fuzzer.Fuzz(&orig)
		out := ToSchedulePauseInfo(FromSchedulePauseInfo(orig))
		assert.Equal(t, orig, out, "SchedulePauseInfo did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Reason == "" && orig.PausedAt.IsZero() && orig.PausedBy == "" {
			return "empty"
		}
		return "filled"
	})
}

func TestScheduleStateFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.ScheduleState
		fuzzer.Fuzz(&orig)
		out := ToScheduleState(FromScheduleState(orig))
		assert.Equal(t, orig, out, "ScheduleState did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if !orig.Paused && orig.PauseInfo == nil {
			return "empty"
		}
		return "filled"
	})
}

func TestBackfillInfoFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.BackfillInfo
		fuzzer.Fuzz(&orig)
		out := ToBackfillInfo(FromBackfillInfo(orig))
		assert.Equal(t, orig, out, "BackfillInfo did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.BackfillID == "" && orig.StartTime.IsZero() && orig.EndTime.IsZero() {
			return "empty"
		}
		return "filled"
	})
}

func TestScheduleInfoFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.ScheduleInfo
		fuzzer.Fuzz(&orig)
		out := ToScheduleInfo(FromScheduleInfo(orig))
		assert.Equal(t, orig, out, "ScheduleInfo did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.OngoingBackfills == nil {
			return "empty"
		}
		return "filled"
	})
}

func TestScheduleListEntryFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.ScheduleListEntry
		fuzzer.Fuzz(&orig)
		out := ToScheduleListEntry(FromScheduleListEntry(orig))
		assert.Equal(t, orig, out, "ScheduleListEntry did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.ScheduleID == "" && orig.WorkflowType == nil && orig.State == nil && orig.CronExpression == "" {
			return "empty"
		}
		return "filled"
	})
}

// --- CRUD request/response deterministic tests ---

func TestCreateScheduleRequest(t *testing.T) {
	for _, item := range []*types.CreateScheduleRequest{nil, {}, &testdata.CreateScheduleRequest} {
		assert.Equal(t, item, ToCreateScheduleRequest(FromCreateScheduleRequest(item)))
	}
}

func TestCreateScheduleResponse(t *testing.T) {
	for _, item := range []*types.CreateScheduleResponse{nil, {}, &testdata.CreateScheduleResponse} {
		assert.Equal(t, item, ToCreateScheduleResponse(FromCreateScheduleResponse(item)))
	}
}

func TestDescribeScheduleRequest(t *testing.T) {
	for _, item := range []*types.DescribeScheduleRequest{nil, {}, &testdata.DescribeScheduleRequest} {
		assert.Equal(t, item, ToDescribeScheduleRequest(FromDescribeScheduleRequest(item)))
	}
}

func TestDescribeScheduleResponse(t *testing.T) {
	for _, item := range []*types.DescribeScheduleResponse{nil, {}, &testdata.DescribeScheduleResponse} {
		assert.Equal(t, item, ToDescribeScheduleResponse(FromDescribeScheduleResponse(item)))
	}
}

func TestUpdateScheduleRequest(t *testing.T) {
	for _, item := range []*types.UpdateScheduleRequest{nil, {}, &testdata.UpdateScheduleRequest} {
		assert.Equal(t, item, ToUpdateScheduleRequest(FromUpdateScheduleRequest(item)))
	}
}

func TestUpdateScheduleResponse(t *testing.T) {
	for _, item := range []*types.UpdateScheduleResponse{nil, {}, &testdata.UpdateScheduleResponse} {
		assert.Equal(t, item, ToUpdateScheduleResponse(FromUpdateScheduleResponse(item)))
	}
}

func TestDeleteScheduleRequest(t *testing.T) {
	for _, item := range []*types.DeleteScheduleRequest{nil, {}, &testdata.DeleteScheduleRequest} {
		assert.Equal(t, item, ToDeleteScheduleRequest(FromDeleteScheduleRequest(item)))
	}
}

func TestDeleteScheduleResponse(t *testing.T) {
	for _, item := range []*types.DeleteScheduleResponse{nil, {}, &testdata.DeleteScheduleResponse} {
		assert.Equal(t, item, ToDeleteScheduleResponse(FromDeleteScheduleResponse(item)))
	}
}

// --- CRUD request/response fuzz tests ---

func TestCreateScheduleRequestFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.CreateScheduleRequest
		fuzzer.Fuzz(&orig)
		out := ToCreateScheduleRequest(FromCreateScheduleRequest(orig))
		assert.Equal(t, orig, out, "CreateScheduleRequest did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Domain == "" && orig.ScheduleID == "" && orig.Spec == nil {
			return "empty"
		}
		return "filled"
	})
}

func TestDescribeScheduleRequestFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.DescribeScheduleRequest
		fuzzer.Fuzz(&orig)
		out := ToDescribeScheduleRequest(FromDescribeScheduleRequest(orig))
		assert.Equal(t, orig, out, "DescribeScheduleRequest did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Domain == "" && orig.ScheduleID == "" {
			return "empty"
		}
		return "filled"
	})
}

func TestDescribeScheduleResponseFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.DescribeScheduleResponse
		fuzzer.Fuzz(&orig)
		out := ToDescribeScheduleResponse(FromDescribeScheduleResponse(orig))
		assert.Equal(t, orig, out, "DescribeScheduleResponse did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Spec == nil && orig.Action == nil && orig.State == nil {
			return "empty"
		}
		return "filled"
	})
}

func TestUpdateScheduleRequestFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.UpdateScheduleRequest
		fuzzer.Fuzz(&orig)
		out := ToUpdateScheduleRequest(FromUpdateScheduleRequest(orig))
		assert.Equal(t, orig, out, "UpdateScheduleRequest did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Domain == "" && orig.ScheduleID == "" && orig.Spec == nil {
			return "empty"
		}
		return "filled"
	})
}

func TestDeleteScheduleRequestFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.DeleteScheduleRequest
		fuzzer.Fuzz(&orig)
		out := ToDeleteScheduleRequest(FromDeleteScheduleRequest(orig))
		assert.Equal(t, orig, out, "DeleteScheduleRequest did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Domain == "" && orig.ScheduleID == "" {
			return "empty"
		}
		return "filled"
	})
}

// --- Action request/response deterministic tests ---

func TestPauseScheduleRequest(t *testing.T) {
	for _, item := range []*types.PauseScheduleRequest{nil, {}, &testdata.PauseScheduleRequest} {
		assert.Equal(t, item, ToPauseScheduleRequest(FromPauseScheduleRequest(item)))
	}
}

func TestPauseScheduleResponse(t *testing.T) {
	for _, item := range []*types.PauseScheduleResponse{nil, {}, &testdata.PauseScheduleResponse} {
		assert.Equal(t, item, ToPauseScheduleResponse(FromPauseScheduleResponse(item)))
	}
}

func TestUnpauseScheduleRequest(t *testing.T) {
	for _, item := range []*types.UnpauseScheduleRequest{nil, {}, &testdata.UnpauseScheduleRequest} {
		assert.Equal(t, item, ToUnpauseScheduleRequest(FromUnpauseScheduleRequest(item)))
	}
}

func TestUnpauseScheduleResponse(t *testing.T) {
	for _, item := range []*types.UnpauseScheduleResponse{nil, {}, &testdata.UnpauseScheduleResponse} {
		assert.Equal(t, item, ToUnpauseScheduleResponse(FromUnpauseScheduleResponse(item)))
	}
}

func TestListSchedulesRequest(t *testing.T) {
	for _, item := range []*types.ListSchedulesRequest{nil, {}, &testdata.ListSchedulesRequest} {
		assert.Equal(t, item, ToListSchedulesRequest(FromListSchedulesRequest(item)))
	}
}

func TestListSchedulesResponse(t *testing.T) {
	for _, item := range []*types.ListSchedulesResponse{nil, {}, &testdata.ListSchedulesResponse} {
		assert.Equal(t, item, ToListSchedulesResponse(FromListSchedulesResponse(item)))
	}
}

func TestBackfillScheduleRequest(t *testing.T) {
	for _, item := range []*types.BackfillScheduleRequest{nil, {}, &testdata.BackfillScheduleRequest} {
		assert.Equal(t, item, ToBackfillScheduleRequest(FromBackfillScheduleRequest(item)))
	}
}

func TestBackfillScheduleResponse(t *testing.T) {
	for _, item := range []*types.BackfillScheduleResponse{nil, {}, &testdata.BackfillScheduleResponse} {
		assert.Equal(t, item, ToBackfillScheduleResponse(FromBackfillScheduleResponse(item)))
	}
}

// --- Action request/response fuzz tests ---

func TestPauseScheduleRequestFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.PauseScheduleRequest
		fuzzer.Fuzz(&orig)
		out := ToPauseScheduleRequest(FromPauseScheduleRequest(orig))
		assert.Equal(t, orig, out, "PauseScheduleRequest did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Domain == "" && orig.ScheduleID == "" {
			return "empty"
		}
		return "filled"
	})
}

func TestUnpauseScheduleRequestFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.UnpauseScheduleRequest
		fuzzer.Fuzz(&orig)
		out := ToUnpauseScheduleRequest(FromUnpauseScheduleRequest(orig))
		assert.Equal(t, orig, out, "UnpauseScheduleRequest did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Domain == "" && orig.ScheduleID == "" {
			return "empty"
		}
		return "filled"
	})
}

func TestListSchedulesRequestFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.ListSchedulesRequest
		fuzzer.Fuzz(&orig)
		out := ToListSchedulesRequest(FromListSchedulesRequest(orig))
		assert.Equal(t, orig, out, "ListSchedulesRequest did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Domain == "" && orig.NextPageToken == nil {
			return "empty"
		}
		return "filled"
	})
}

func TestListSchedulesResponseFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.ListSchedulesResponse
		fuzzer.Fuzz(&orig)
		out := ToListSchedulesResponse(FromListSchedulesResponse(orig))
		assert.Equal(t, orig, out, "ListSchedulesResponse did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Schedules == nil && orig.NextPageToken == nil {
			return "empty"
		}
		return "filled"
	})
}

func TestBackfillScheduleRequestFuzz(t *testing.T) {
	testutils.EnsureFuzzCoverage(t, []string{"nil", "empty", "filled"}, func(t *testing.T, f *fuzz.Fuzzer) string {
		fuzzer := scheduleFuzzer(f)
		var orig *types.BackfillScheduleRequest
		fuzzer.Fuzz(&orig)
		out := ToBackfillScheduleRequest(FromBackfillScheduleRequest(orig))
		assert.Equal(t, orig, out, "BackfillScheduleRequest did not survive round-tripping")

		if orig == nil {
			return "nil"
		}
		if orig.Domain == "" && orig.ScheduleID == "" {
			return "empty"
		}
		return "filled"
	})
}
