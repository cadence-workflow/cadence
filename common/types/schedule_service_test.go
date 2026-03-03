// Copyright (c) 2024 Uber Technologies, Inc.
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

package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScheduleListEntry_NilGetters(t *testing.T) {
	var v *ScheduleListEntry
	assert.Equal(t, "", v.GetScheduleID())
	assert.Nil(t, v.GetWorkflowType())
	assert.Nil(t, v.GetState())
	assert.Equal(t, "", v.GetCronExpression())
}

func TestScheduleListEntry_Getters(t *testing.T) {
	wt := &WorkflowType{Name: "test-wf"}
	st := &ScheduleState{Paused: true}
	v := &ScheduleListEntry{
		ScheduleID:     "sched-1",
		WorkflowType:   wt,
		State:          st,
		CronExpression: "*/5 * * * *",
	}
	assert.Equal(t, "sched-1", v.GetScheduleID())
	assert.Equal(t, wt, v.GetWorkflowType())
	assert.Equal(t, st, v.GetState())
	assert.Equal(t, "*/5 * * * *", v.GetCronExpression())
}

func TestCreateScheduleRequest_NilGetters(t *testing.T) {
	var v *CreateScheduleRequest
	assert.Equal(t, "", v.GetDomain())
	assert.Equal(t, "", v.GetScheduleID())
	assert.Nil(t, v.GetSpec())
	assert.Nil(t, v.GetAction())
	assert.Nil(t, v.GetPolicies())
	assert.Nil(t, v.GetMemo())
	assert.Nil(t, v.GetSearchAttributes())
}

func TestDescribeScheduleRequest_NilGetters(t *testing.T) {
	var v *DescribeScheduleRequest
	assert.Equal(t, "", v.GetDomain())
	assert.Equal(t, "", v.GetScheduleID())
}

func TestDescribeScheduleResponse_NilGetters(t *testing.T) {
	var v *DescribeScheduleResponse
	assert.Nil(t, v.GetSpec())
	assert.Nil(t, v.GetAction())
	assert.Nil(t, v.GetPolicies())
	assert.Nil(t, v.GetState())
	assert.Nil(t, v.GetInfo())
	assert.Nil(t, v.GetMemo())
	assert.Nil(t, v.GetSearchAttributes())
}

func TestUpdateScheduleRequest_NilGetters(t *testing.T) {
	var v *UpdateScheduleRequest
	assert.Equal(t, "", v.GetDomain())
	assert.Equal(t, "", v.GetScheduleID())
	assert.Nil(t, v.GetSpec())
	assert.Nil(t, v.GetAction())
	assert.Nil(t, v.GetPolicies())
	assert.Nil(t, v.GetSearchAttributes())
}

func TestDeleteScheduleRequest_NilGetters(t *testing.T) {
	var v *DeleteScheduleRequest
	assert.Equal(t, "", v.GetDomain())
	assert.Equal(t, "", v.GetScheduleID())
}

func TestPauseScheduleRequest_NilGetters(t *testing.T) {
	var v *PauseScheduleRequest
	assert.Equal(t, "", v.GetDomain())
	assert.Equal(t, "", v.GetScheduleID())
	assert.Equal(t, "", v.GetReason())
}

func TestUnpauseScheduleRequest_NilGetters(t *testing.T) {
	var v *UnpauseScheduleRequest
	assert.Equal(t, "", v.GetDomain())
	assert.Equal(t, "", v.GetScheduleID())
	assert.Equal(t, "", v.GetReason())
	assert.Equal(t, ScheduleCatchUpPolicyInvalid, v.GetCatchUpPolicy())
}

func TestUnpauseScheduleRequest_Getters(t *testing.T) {
	v := &UnpauseScheduleRequest{
		Domain:        "test-domain",
		ScheduleID:    "sched-1",
		Reason:        "resuming",
		CatchUpPolicy: ScheduleCatchUpPolicyAll,
	}
	assert.Equal(t, "test-domain", v.GetDomain())
	assert.Equal(t, "sched-1", v.GetScheduleID())
	assert.Equal(t, "resuming", v.GetReason())
	assert.Equal(t, ScheduleCatchUpPolicyAll, v.GetCatchUpPolicy())
}

func TestListSchedulesRequest_NilGetters(t *testing.T) {
	var v *ListSchedulesRequest
	assert.Equal(t, "", v.GetDomain())
	assert.Equal(t, int32(0), v.GetPageSize())
	assert.Nil(t, v.GetNextPageToken())
}

func TestListSchedulesResponse_NilGetters(t *testing.T) {
	var v *ListSchedulesResponse
	assert.Nil(t, v.GetSchedules())
	assert.Nil(t, v.GetNextPageToken())
}

func TestBackfillScheduleRequest_NilGetters(t *testing.T) {
	var v *BackfillScheduleRequest
	assert.Equal(t, "", v.GetDomain())
	assert.Equal(t, "", v.GetScheduleID())
	assert.Equal(t, time.Time{}, v.GetStartTime())
	assert.Equal(t, time.Time{}, v.GetEndTime())
	assert.Equal(t, ScheduleOverlapPolicyInvalid, v.GetOverlapPolicy())
	assert.Equal(t, "", v.GetBackfillID())
}

func TestBackfillScheduleRequest_Getters(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	v := &BackfillScheduleRequest{
		Domain:        "test-domain",
		ScheduleID:    "sched-1",
		StartTime:     now,
		EndTime:       now.Add(time.Hour),
		OverlapPolicy: ScheduleOverlapPolicyBuffer,
		BackfillID:    "bf-1",
	}
	assert.Equal(t, "test-domain", v.GetDomain())
	assert.Equal(t, "sched-1", v.GetScheduleID())
	assert.Equal(t, now, v.GetStartTime())
	assert.Equal(t, now.Add(time.Hour), v.GetEndTime())
	assert.Equal(t, ScheduleOverlapPolicyBuffer, v.GetOverlapPolicy())
	assert.Equal(t, "bf-1", v.GetBackfillID())
}
