// Copyright (c) 2021 Uber Technologies Inc.
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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/testdata"
)

func TestMatchingAddActivityTaskRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.AddActivityTaskRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingAddActivityTaskRequest,
		},
		{
			desc:  "empty input test",
			input: &types.AddActivityTaskRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingAddActivityTaskRequest(tc.input)
		roundTripObj := ToMatchingAddActivityTaskRequest(thriftObj)
		assert.Equal(t, tc.input, roundTripObj)
	}
}

func TestActivityTaskDispatchInfo(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.ActivityTaskDispatchInfo
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingActivityTaskDispatchInfo,
		},
		{
			desc:  "empty input test",
			input: &types.ActivityTaskDispatchInfo{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromActivityTaskDispatchInfo(tc.input)
		roundTripObj := ToActivityTaskDispatchInfo(thriftObj)
		opts := cmpopts.IgnoreFields(types.WorkflowExecutionStartedEventAttributes{}, "ParentWorkflowDomainID")
		if diff := cmp.Diff(tc.input, roundTripObj, opts); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestMatchingAddDecisionTaskRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.AddDecisionTaskRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingAddDecisionTaskRequest,
		},
		{
			desc:  "empty input test",
			input: &types.AddDecisionTaskRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingAddDecisionTaskRequest(tc.input)
		roundTripObj := ToMatchingAddDecisionTaskRequest(thriftObj)
		assert.Equal(t, tc.input, roundTripObj)
	}
}

func TestMatchingCancelOutStandingPollRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.CancelOutstandingPollRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingCancelOutstandingPollRequest,
		},
		{
			desc:  "empty input test",
			input: &types.CancelOutstandingPollRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingCancelOutstandingPollRequest(tc.input)
		roundTripObj := ToMatchingCancelOutstandingPollRequest(thriftObj)
		assert.Equal(t, tc.input, roundTripObj)
	}
}

func TestMatchingDescribeTaskListRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.MatchingDescribeTaskListRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingDescribeTaskListRequest,
		},
		{
			desc:  "empty input test",
			input: &types.MatchingDescribeTaskListRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingDescribeTaskListRequest(tc.input)
		roundTripObj := ToMatchingDescribeTaskListRequest(thriftObj)
		assert.Equal(t, tc.input, roundTripObj)
	}
}

func TestMatchingListTaskListPartitionsRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.MatchingListTaskListPartitionsRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingListTaskListPartitionsRequest,
		},
		{
			desc:  "empty input test",
			input: &types.MatchingListTaskListPartitionsRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingListTaskListPartitionsRequest(tc.input)
		roundTripObj := ToMatchingListTaskListPartitionsRequest(thriftObj)
		assert.Equal(t, tc.input, roundTripObj)
	}
}

func TestMatchingPollForActivityRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.MatchingPollForActivityTaskRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingPollForActivityTaskRequest,
		},
		{
			desc:  "empty input test",
			input: &types.MatchingPollForActivityTaskRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingPollForActivityTaskRequest(tc.input)
		roundTripObj := ToMatchingPollForActivityTaskRequest(thriftObj)
		assert.Equal(t, tc.input, roundTripObj)
	}
}

func TestMatchingPollForDecisionRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.MatchingPollForDecisionTaskRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingPollForDecisionTaskRequest,
		},
		{
			desc:  "empty input test",
			input: &types.MatchingPollForDecisionTaskRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingPollForDecisionTaskRequest(tc.input)
		roundTripObj := ToMatchingPollForDecisionTaskRequest(thriftObj)
		assert.Equal(t, tc.input, roundTripObj)
	}
}

func TestMatchingPollForDecisionResponse(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.MatchingPollForDecisionTaskResponse
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingPollForDecisionTaskResponse,
		},
		{
			desc:  "empty input test",
			input: &types.MatchingPollForDecisionTaskResponse{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingPollForDecisionTaskResponse(tc.input)
		roundTripObj := ToMatchingPollForDecisionTaskResponse(thriftObj)
		opt := cmpopts.IgnoreFields(types.WorkflowExecutionStartedEventAttributes{}, "ParentWorkflowDomainID")
		opt2 := cmpopts.IgnoreFields(types.MatchingPollForDecisionTaskResponse{}, "PartitionConfig")
		opt3 := cmpopts.IgnoreFields(types.MatchingPollForDecisionTaskResponse{}, "LoadBalancerHints")
		if diff := cmp.Diff(tc.input, roundTripObj, opt, opt2, opt3); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestMatchingPollForActivityTaskResponse(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.MatchingPollForActivityTaskResponse
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingPollForActivityTaskResponse,
		},
		{
			desc:  "empty input test",
			input: &types.MatchingPollForActivityTaskResponse{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingPollForActivityTaskResponse(tc.input)
		roundTripObj := ToMatchingPollForActivityTaskResponse(thriftObj)
		opt := cmpopts.IgnoreFields(types.MatchingPollForActivityTaskResponse{}, "PartitionConfig")
		opt2 := cmpopts.IgnoreFields(types.MatchingPollForActivityTaskResponse{}, "LoadBalancerHints")
		if diff := cmp.Diff(tc.input, roundTripObj, opt, opt2); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestMatchingQueryWorkflowRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.MatchingQueryWorkflowRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingQueryWorkflowRequest,
		},
		{
			desc:  "empty input test",
			input: &types.MatchingQueryWorkflowRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingQueryWorkflowRequest(tc.input)
		roundTripObj := ToMatchingQueryWorkflowRequest(thriftObj)
		assert.Equal(t, tc.input, roundTripObj)
	}
}

func TestMatchingRespondQueryTaskCompletedRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.MatchingRespondQueryTaskCompletedRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MatchingRespondQueryTaskCompletedRequest,
		},
		{
			desc:  "empty input test",
			input: &types.MatchingRespondQueryTaskCompletedRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromMatchingRespondQueryTaskCompletedRequest(tc.input)
		roundTripObj := ToMatchingRespondQueryTaskCompletedRequest(thriftObj)
		assert.Equal(t, tc.input, roundTripObj)
	}
}

func TestTaskSource(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.TaskSource
	}{
		{
			desc:  "non-nil TaskSourceHistory input test",
			input: types.TaskSourceHistory.Ptr(),
		},
		{
			desc:  "non-nil TaskSourceDBBacklog input test",
			input: types.TaskSourceDbBacklog.Ptr(),
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		thriftObj := FromTaskSource(tc.input)
		roundTripObj := ToTaskSource(thriftObj)
		assert.Equal(t, tc.input, roundTripObj)
	}
}
