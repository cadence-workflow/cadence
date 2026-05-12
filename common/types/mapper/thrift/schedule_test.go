// Copyright (c) 2024 Uber Technologies, Inc.
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

package thrift

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/testdata"
)

// --- Enum round-trips ---

func TestScheduleOverlapPolicyConversion(t *testing.T) {
	for _, policy := range []types.ScheduleOverlapPolicy{
		types.ScheduleOverlapPolicyInvalid,
		types.ScheduleOverlapPolicySkipNew,
		types.ScheduleOverlapPolicyBuffer,
		types.ScheduleOverlapPolicyConcurrent,
		types.ScheduleOverlapPolicyCancelPrevious,
		types.ScheduleOverlapPolicyTerminatePrevious,
	} {
		assert.Equal(t, policy, ToScheduleOverlapPolicy(FromScheduleOverlapPolicy(policy)))
	}
}

func TestToScheduleOverlapPolicyNil(t *testing.T) {
	assert.Equal(t, types.ScheduleOverlapPolicyInvalid, ToScheduleOverlapPolicy(nil))
}

func TestScheduleCatchUpPolicyConversion(t *testing.T) {
	for _, policy := range []types.ScheduleCatchUpPolicy{
		types.ScheduleCatchUpPolicyInvalid,
		types.ScheduleCatchUpPolicySkip,
		types.ScheduleCatchUpPolicyOne,
		types.ScheduleCatchUpPolicyAll,
	} {
		assert.Equal(t, policy, ToScheduleCatchUpPolicy(FromScheduleCatchUpPolicy(policy)))
	}
}

func TestToScheduleCatchUpPolicyNil(t *testing.T) {
	assert.Equal(t, types.ScheduleCatchUpPolicyInvalid, ToScheduleCatchUpPolicy(nil))
}

// --- Nil-safety tests ---

func TestFromScheduleSpecNil(t *testing.T) {
	assert.Nil(t, FromScheduleSpec(nil))
}

func TestToScheduleSpecNil(t *testing.T) {
	assert.Nil(t, ToScheduleSpec(nil))
}

func TestFromStartWorkflowActionNil(t *testing.T) {
	assert.Nil(t, FromStartWorkflowAction(nil))
}

func TestToStartWorkflowActionNil(t *testing.T) {
	assert.Nil(t, ToStartWorkflowAction(nil))
}

func TestFromScheduleActionNil(t *testing.T) {
	assert.Nil(t, FromScheduleAction(nil))
}

func TestToScheduleActionNil(t *testing.T) {
	assert.Nil(t, ToScheduleAction(nil))
}

func TestFromSchedulePoliciesNil(t *testing.T) {
	assert.Nil(t, FromSchedulePolicies(nil))
}

func TestToSchedulePoliciesNil(t *testing.T) {
	assert.Nil(t, ToSchedulePolicies(nil))
}

func TestFromSchedulePauseInfoNil(t *testing.T) {
	assert.Nil(t, FromSchedulePauseInfo(nil))
}

func TestToSchedulePauseInfoNil(t *testing.T) {
	assert.Nil(t, ToSchedulePauseInfo(nil))
}

func TestFromScheduleStateNil(t *testing.T) {
	assert.Nil(t, FromScheduleState(nil))
}

func TestToScheduleStateNil(t *testing.T) {
	assert.Nil(t, ToScheduleState(nil))
}

func TestFromBackfillInfoNil(t *testing.T) {
	assert.Nil(t, FromBackfillInfo(nil))
}

func TestToBackfillInfoNil(t *testing.T) {
	assert.Nil(t, ToBackfillInfo(nil))
}

func TestFromScheduleInfoNil(t *testing.T) {
	assert.Nil(t, FromScheduleInfo(nil))
}

func TestToScheduleInfoNil(t *testing.T) {
	assert.Nil(t, ToScheduleInfo(nil))
}

func TestFromScheduleListEntryNil(t *testing.T) {
	assert.Nil(t, FromScheduleListEntry(nil))
}

func TestToScheduleListEntryNil(t *testing.T) {
	assert.Nil(t, ToScheduleListEntry(nil))
}

// --- Round-trip tests ---

func TestScheduleSpecConversion(t *testing.T) {
	for _, item := range []*types.ScheduleSpec{nil, {}, &testdata.ScheduleSpecThrift} {
		assert.Equal(t, item, ToScheduleSpec(FromScheduleSpec(item)))
	}
}

func TestSchedulePauseInfoConversion(t *testing.T) {
	for _, item := range []*types.SchedulePauseInfo{nil, {}, &testdata.SchedulePauseInfoThrift} {
		assert.Equal(t, item, ToSchedulePauseInfo(FromSchedulePauseInfo(item)))
	}
}

func TestBackfillInfoConversion(t *testing.T) {
	for _, item := range []*types.BackfillInfo{nil, {}, &testdata.ScheduleBackfillInfoThrift} {
		assert.Equal(t, item, ToBackfillInfo(FromBackfillInfo(item)))
	}
}

func TestScheduleInfoConversion(t *testing.T) {
	for _, item := range []*types.ScheduleInfo{nil, {}, &testdata.ScheduleInfoThrift} {
		assert.Equal(t, item, ToScheduleInfo(FromScheduleInfo(item)))
	}
}

func TestScheduleStateConversion(t *testing.T) {
	for _, item := range []*types.ScheduleState{nil, {}, &testdata.ScheduleStateThrift} {
		assert.Equal(t, item, ToScheduleState(FromScheduleState(item)))
	}
}

func TestScheduleListEntryConversion(t *testing.T) {
	for _, item := range []*types.ScheduleListEntry{nil, {}, &testdata.ScheduleListEntryThrift} {
		assert.Equal(t, item, ToScheduleListEntry(FromScheduleListEntry(item)))
	}
}

func TestStartWorkflowActionConversion(t *testing.T) {
	for _, item := range []*types.StartWorkflowAction{nil, {}, &testdata.ScheduleStartWorkflowAction} {
		assert.Equal(t, item, ToStartWorkflowAction(FromStartWorkflowAction(item)))
	}
}

func TestScheduleActionConversion(t *testing.T) {
	for _, item := range []*types.ScheduleAction{nil, {}, &testdata.ScheduleAction} {
		assert.Equal(t, item, ToScheduleAction(FromScheduleAction(item)))
	}
}

func TestSchedulePoliciesConversion(t *testing.T) {
	for _, item := range []*types.SchedulePolicies{nil, {}, &testdata.SchedulePolicies} {
		assert.Equal(t, item, ToSchedulePolicies(FromSchedulePolicies(item)))
	}
}

func TestCreateScheduleRequestConversion(t *testing.T) {
	for _, item := range []*types.CreateScheduleRequest{nil, {}, &testdata.CreateScheduleRequestThrift} {
		assert.Equal(t, item, ToCreateScheduleRequest(FromCreateScheduleRequest(item)))
	}
}

func TestCreateScheduleResponseConversion(t *testing.T) {
	for _, item := range []*types.CreateScheduleResponse{nil, {}, &testdata.CreateScheduleResponse} {
		assert.Equal(t, item, ToCreateScheduleResponse(FromCreateScheduleResponse(item)))
	}
}

func TestDescribeScheduleRequestConversion(t *testing.T) {
	for _, item := range []*types.DescribeScheduleRequest{nil, {}, &testdata.DescribeScheduleRequest} {
		assert.Equal(t, item, ToDescribeScheduleRequest(FromDescribeScheduleRequest(item)))
	}
}

func TestDescribeScheduleResponseConversion(t *testing.T) {
	for _, item := range []*types.DescribeScheduleResponse{nil, {}, &testdata.DescribeScheduleResponseThrift} {
		assert.Equal(t, item, ToDescribeScheduleResponse(FromDescribeScheduleResponse(item)))
	}
}

func TestUpdateScheduleRequestConversion(t *testing.T) {
	for _, item := range []*types.UpdateScheduleRequest{nil, {}, &testdata.UpdateScheduleRequestThrift} {
		assert.Equal(t, item, ToUpdateScheduleRequest(FromUpdateScheduleRequest(item)))
	}
}

func TestDeleteScheduleRequestConversion(t *testing.T) {
	for _, item := range []*types.DeleteScheduleRequest{nil, {}, &testdata.DeleteScheduleRequest} {
		assert.Equal(t, item, ToDeleteScheduleRequest(FromDeleteScheduleRequest(item)))
	}
}

func TestPauseScheduleRequestConversion(t *testing.T) {
	for _, item := range []*types.PauseScheduleRequest{nil, {}, &testdata.PauseScheduleRequest} {
		assert.Equal(t, item, ToPauseScheduleRequest(FromPauseScheduleRequest(item)))
	}
}

func TestUnpauseScheduleRequestConversion(t *testing.T) {
	for _, item := range []*types.UnpauseScheduleRequest{nil, {}, &testdata.UnpauseScheduleRequest} {
		assert.Equal(t, item, ToUnpauseScheduleRequest(FromUnpauseScheduleRequest(item)))
	}
}

func TestBackfillScheduleRequestConversion(t *testing.T) {
	for _, item := range []*types.BackfillScheduleRequest{nil, {}, &testdata.BackfillScheduleRequestThrift} {
		assert.Equal(t, item, ToBackfillScheduleRequest(FromBackfillScheduleRequest(item)))
	}
}

func TestListSchedulesRequestConversion(t *testing.T) {
	for _, item := range []*types.ListSchedulesRequest{nil, {}, &testdata.ListSchedulesRequest} {
		assert.Equal(t, item, ToListSchedulesRequest(FromListSchedulesRequest(item)))
	}
}

func TestListSchedulesResponseConversion(t *testing.T) {
	for _, item := range []*types.ListSchedulesResponse{nil, {}, &testdata.ListSchedulesResponseThrift} {
		assert.Equal(t, item, ToListSchedulesResponse(FromListSchedulesResponse(item)))
	}
}

// --- Empty response functions return non-nil empty structs ---

func TestFromUpdateScheduleResponse(t *testing.T) {
	assert.NotNil(t, FromUpdateScheduleResponse(nil))
	assert.NotNil(t, FromUpdateScheduleResponse(&types.UpdateScheduleResponse{}))
}

func TestFromDeleteScheduleResponse(t *testing.T) {
	assert.NotNil(t, FromDeleteScheduleResponse(nil))
	assert.NotNil(t, FromDeleteScheduleResponse(&types.DeleteScheduleResponse{}))
}

func TestFromPauseScheduleResponse(t *testing.T) {
	assert.NotNil(t, FromPauseScheduleResponse(nil))
	assert.NotNil(t, FromPauseScheduleResponse(&types.PauseScheduleResponse{}))
}

func TestFromUnpauseScheduleResponse(t *testing.T) {
	assert.NotNil(t, FromUnpauseScheduleResponse(nil))
	assert.NotNil(t, FromUnpauseScheduleResponse(&types.UnpauseScheduleResponse{}))
}

func TestFromBackfillScheduleResponse(t *testing.T) {
	assert.NotNil(t, FromBackfillScheduleResponse(nil))
	assert.NotNil(t, FromBackfillScheduleResponse(&types.BackfillScheduleResponse{}))
}
