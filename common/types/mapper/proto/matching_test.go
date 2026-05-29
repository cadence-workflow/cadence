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

package proto

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"

	matchingv1 "github.com/uber/cadence/.gen/proto/matching/v1"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
	"github.com/uber/cadence/common/types/testdata"
)

func TestMatchingAddActivityTaskRequest(t *testing.T) {
	for _, item := range []*types.AddActivityTaskRequest{nil, {}, &testdata.MatchingAddActivityTaskRequest} {
		assert.Equal(t, item, ToMatchingAddActivityTaskRequest(FromMatchingAddActivityTaskRequest(item)))
	}
}

func TestMatchingAddDecisionTaskRequest(t *testing.T) {
	for _, item := range []*types.AddDecisionTaskRequest{nil, {}, &testdata.MatchingAddDecisionTaskRequest} {
		assert.Equal(t, item, ToMatchingAddDecisionTaskRequest(FromMatchingAddDecisionTaskRequest(item)))
	}
}

func TestMatchingAddActivityTaskResponse(t *testing.T) {
	for _, item := range []*types.AddActivityTaskResponse{nil, {}, &testdata.MatchingAddActivityTaskResponse} {
		assert.Equal(t, item, ToMatchingAddActivityTaskResponse(FromMatchingAddActivityTaskResponse(item)))
	}
}

func TestMatchingAddDecisionTaskResponse(t *testing.T) {
	for _, item := range []*types.AddDecisionTaskResponse{nil, {}, &testdata.MatchingAddDecisionTaskResponse} {
		assert.Equal(t, item, ToMatchingAddDecisionTaskResponse(FromMatchingAddDecisionTaskResponse(item)))
	}
}

func TestMatchingCancelOutstandingPollRequest(t *testing.T) {
	for _, item := range []*types.CancelOutstandingPollRequest{nil, {}, &testdata.MatchingCancelOutstandingPollRequest} {
		assert.Equal(t, item, ToMatchingCancelOutstandingPollRequest(FromMatchingCancelOutstandingPollRequest(item)))
	}
}

func TestMatchingDescribeTaskListRequest(t *testing.T) {
	for _, item := range []*types.MatchingDescribeTaskListRequest{nil, {}, &testdata.MatchingDescribeTaskListRequest} {
		assert.Equal(t, item, ToMatchingDescribeTaskListRequest(FromMatchingDescribeTaskListRequest(item)))
	}
}

func TestMatchingDescribeTaskListResponse(t *testing.T) {
	for _, item := range []*types.DescribeTaskListResponse{nil, {}, &testdata.MatchingDescribeTaskListResponse} {
		assert.Equal(t, item, ToMatchingDescribeTaskListResponse(FromMatchingDescribeTaskListResponse(item)))
	}
}

func TestMatchingDescribeTaskListResponseMap(t *testing.T) {
	for _, item := range []map[string]*types.DescribeTaskListResponse{nil, {}, testdata.DescribeTaskListResponseMap} {
		assert.Equal(t, item, ToMatchingDescribeTaskListResponseMap(FromMatchingDescribeTaskListResponseMap(item)))
	}
}

func TestMatchingListTaskListPartitionsRequest(t *testing.T) {
	for _, item := range []*types.MatchingListTaskListPartitionsRequest{nil, {}, &testdata.MatchingListTaskListPartitionsRequest} {
		assert.Equal(t, item, ToMatchingListTaskListPartitionsRequest(FromMatchingListTaskListPartitionsRequest(item)))
	}
}

func TestMatchingListTaskListPartitionsResponse(t *testing.T) {
	for _, item := range []*types.ListTaskListPartitionsResponse{nil, {}, &testdata.MatchingListTaskListPartitionsResponse} {
		assert.Equal(t, item, ToMatchingListTaskListPartitionsResponse(FromMatchingListTaskListPartitionsResponse(item)))
	}
}

func TestMatchingPollForActivityTaskRequest(t *testing.T) {
	for _, item := range []*types.MatchingPollForActivityTaskRequest{nil, {}, &testdata.MatchingPollForActivityTaskRequest} {
		assert.Equal(t, item, ToMatchingPollForActivityTaskRequest(FromMatchingPollForActivityTaskRequest(item)))
	}
}

func TestMatchingPollForActivityTaskResponse(t *testing.T) {
	for _, item := range []*types.MatchingPollForActivityTaskResponse{nil, {}, &testdata.MatchingPollForActivityTaskResponse} {
		assert.Equal(t, item, ToMatchingPollForActivityTaskResponse(FromMatchingPollForActivityTaskResponse(item)))
	}
}

func TestTaskListPartitionConfig(t *testing.T) {
	for _, item := range []*types.TaskListPartitionConfig{nil, {}, &testdata.TaskListPartitionConfig} {
		assert.Equal(t, item, ToTaskListPartitionConfig(FromTaskListPartitionConfig(item)))
	}
}

func TestMatchingPollForDecisionTaskRequest(t *testing.T) {
	for _, item := range []*types.MatchingPollForDecisionTaskRequest{nil, {}, &testdata.MatchingPollForDecisionTaskRequest} {
		assert.Equal(t, item, ToMatchingPollForDecisionTaskRequest(FromMatchingPollForDecisionTaskRequest(item)))
	}
}

func TestMatchingPollForDecisionTaskResponse(t *testing.T) {
	for _, item := range []*types.MatchingPollForDecisionTaskResponse{nil, {}, &testdata.MatchingPollForDecisionTaskResponse} {
		assert.Equal(t, item, ToMatchingPollForDecisionTaskResponse(FromMatchingPollForDecisionTaskResponse(item)))
	}
}

func TestMatchingQueryWorkflowRequest(t *testing.T) {
	for _, item := range []*types.MatchingQueryWorkflowRequest{nil, {}, &testdata.MatchingQueryWorkflowRequest} {
		assert.Equal(t, item, ToMatchingQueryWorkflowRequest(FromMatchingQueryWorkflowRequest(item)))
	}
}

func TestMatchingQueryWorkflowResponse(t *testing.T) {
	for _, item := range []*types.MatchingQueryWorkflowResponse{nil, {}, &testdata.MatchingQueryWorkflowResponse} {
		assert.Equal(t, item, ToMatchingQueryWorkflowResponse(FromMatchingQueryWorkflowResponse(item)))
	}
}

func TestMatchingRespondQueryTaskCompletedRequest(t *testing.T) {
	for _, item := range []*types.MatchingRespondQueryTaskCompletedRequest{nil, {}, &testdata.MatchingRespondQueryTaskCompletedRequest} {
		assert.Equal(t, item, ToMatchingRespondQueryTaskCompletedRequest(FromMatchingRespondQueryTaskCompletedRequest(item)))
	}
}

func TestMatchingGetTaskListsByDomainRequest(t *testing.T) {
	for _, item := range []*types.GetTaskListsByDomainRequest{nil, {}, &testdata.MatchingGetTaskListsByDomainRequest} {
		assert.Equal(t, item, ToMatchingGetTaskListsByDomainRequest(FromMatchingGetTaskListsByDomainRequest(item)))
	}
}

func TestMatchingGetTaskListsByDomainResponse(t *testing.T) {
	for _, item := range []*types.GetTaskListsByDomainResponse{nil, {}, &testdata.GetTaskListsByDomainResponse} {
		assert.Equal(t, item, ToMatchingGetTaskListsByDomainResponse(FromMatchingGetTaskListsByDomainResponse(item)))
	}
}

func TestMatchingUpdateTaskListPartitionConfigRequest(t *testing.T) {
	for _, item := range []*types.MatchingUpdateTaskListPartitionConfigRequest{nil, {}, &testdata.MatchingUpdateTaskListPartitionConfigRequest} {
		assert.Equal(t, item, ToMatchingUpdateTaskListPartitionConfigRequest(FromMatchingUpdateTaskListPartitionConfigRequest(item)))
	}
}

func TestMatchingUpdateTaskListPartitionConfigResponse(t *testing.T) {
	for _, item := range []*types.MatchingUpdateTaskListPartitionConfigResponse{nil, {}} {
		assert.Equal(t, item, ToMatchingUpdateTaskListPartitionConfigResponse(FromMatchingUpdateTaskListPartitionConfigResponse(item)))
	}
}

func TestMatchingRefreshTaskListPartitionConfigRequest(t *testing.T) {
	for _, item := range []*types.MatchingRefreshTaskListPartitionConfigRequest{nil, {}, &testdata.MatchingRefreshTaskListPartitionConfigRequest} {
		assert.Equal(t, item, ToMatchingRefreshTaskListPartitionConfigRequest(FromMatchingRefreshTaskListPartitionConfigRequest(item)))
	}
}

func TestMatchingRefreshTaskListPartitionConfigResponse(t *testing.T) {
	for _, item := range []*types.MatchingRefreshTaskListPartitionConfigResponse{nil, {}} {
		assert.Equal(t, item, ToMatchingRefreshTaskListPartitionConfigResponse(FromMatchingRefreshTaskListPartitionConfigResponse(item)))
	}
}

func TestToMatchingTaskListPartitionConfig(t *testing.T) {
	cases := []struct {
		name     string
		config   *matchingv1.TaskListPartitionConfig
		expected *types.TaskListPartitionConfig
	}{
		{
			name: "happy path",
			config: &matchingv1.TaskListPartitionConfig{
				Version:            1,
				NumReadPartitions:  2,
				NumWritePartitions: 2,
				ReadPartitions: map[int32]*matchingv1.TaskListPartition{
					0: {IsolationGroups: []string{"foo"}},
					1: {IsolationGroups: []string{"bar"}},
				},
				WritePartitions: map[int32]*matchingv1.TaskListPartition{
					0: {IsolationGroups: []string{"baz"}},
					1: {IsolationGroups: []string{"bar"}},
				},
			},
			expected: &types.TaskListPartitionConfig{
				Version: 1,
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {IsolationGroups: []string{"foo"}},
					1: {IsolationGroups: []string{"bar"}},
				},
				WritePartitions: map[int]*types.TaskListPartition{
					0: {IsolationGroups: []string{"baz"}},
					1: {IsolationGroups: []string{"bar"}},
				},
			},
		},
		{
			name: "numbers only",
			config: &matchingv1.TaskListPartitionConfig{
				Version:            1,
				NumReadPartitions:  2,
				NumWritePartitions: 2,
			},
			expected: &types.TaskListPartitionConfig{
				Version: 1,
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {},
					1: {},
				},
				WritePartitions: map[int]*types.TaskListPartition{
					0: {},
					1: {},
				},
			},
		},
		{
			name: "number mismatch",
			config: &matchingv1.TaskListPartitionConfig{
				Version:            1,
				NumReadPartitions:  2,
				NumWritePartitions: 1,
				ReadPartitions: map[int32]*matchingv1.TaskListPartition{
					0: {IsolationGroups: []string{"foo"}},
					1: {IsolationGroups: []string{"bar"}},
				},
				WritePartitions: map[int32]*matchingv1.TaskListPartition{
					0: {IsolationGroups: []string{"baz"}},
					1: {IsolationGroups: []string{"bar"}},
				},
			},
			expected: &types.TaskListPartitionConfig{
				Version: 1,
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {IsolationGroups: []string{"foo"}},
					1: {IsolationGroups: []string{"bar"}},
				},
				WritePartitions: map[int]*types.TaskListPartition{
					0: {},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := ToTaskListPartitionConfig(tc.config)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

// --- Fuzz tests for matching mapper functions

// TaskSourceFuzzer generates valid TaskSource enum values (0: History, 1: DbBacklog).
func TaskSourceFuzzer(e *types.TaskSource, c fuzz.Continue) {
	*e = types.TaskSource(c.Intn(2)) // 0-1: History, DbBacklog
}

// CancelOutstandingPollRequestFuzzer ensures TaskListType *int32 only contains
// valid values (0=Decision, 1=Activity) since out-of-range values map to nil
// on the return path.
func CancelOutstandingPollRequestFuzzer(r *types.CancelOutstandingPollRequest, c fuzz.Continue) {
	c.FuzzNoCustom(r)
	if r.TaskListType != nil {
		*r.TaskListType = int32(c.Intn(2)) // 0-1: Decision, Activity
	}
}

func TestLoadBalancerHintsFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromLoadBalancerHints, ToLoadBalancerHints)
}

func TestMatchingTaskListPartitionFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMatchingTaskListPartition, ToMatchingTaskListPartition)
}

func TestActivityTaskDispatchInfoFuzz(t *testing.T) {
	// [BUG] Attempt is *int64 in types but int32 in proto:
	//   - nil input maps to &0 (non-nil), breaking the nil round-trip
	//   - int64 values outside int32 range are truncated
	// ScheduledEvent is a HistoryEvent requiring complex enum handling;
	// it is tested comprehensively in api_test.go (TestHistoryEventFuzz).
	testutils.RunMapperFuzzTest(t, FromActivityTaskDispatchInfo, ToActivityTaskDispatchInfo,
		testutils.WithExcludedFields("Attempt", "ScheduledEvent"),
	)
}

func TestMatchingAddActivityTaskRequestFuzz(t *testing.T) {
	// ActivityTaskDispatchInfo is tested separately in TestActivityTaskDispatchInfoFuzz.
	testutils.RunMapperFuzzTest(t, FromMatchingAddActivityTaskRequest, ToMatchingAddActivityTaskRequest,
		testutils.WithCustomFuncs(TaskSourceFuzzer),
		testutils.WithExcludedFields("ActivityTaskDispatchInfo"),
	)
}

func TestMatchingAddDecisionTaskRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMatchingAddDecisionTaskRequest, ToMatchingAddDecisionTaskRequest,
		testutils.WithCustomFuncs(TaskSourceFuzzer),
	)
}

func TestMatchingCancelOutstandingPollRequestFuzz(t *testing.T) {
	// TaskListType is *int32 in types (0=Decision, 1=Activity); out-of-range values
	// map to nil on the return path. CancelOutstandingPollRequestFuzzer constrains it.
	testutils.RunMapperFuzzTest(t, FromMatchingCancelOutstandingPollRequest, ToMatchingCancelOutstandingPollRequest,
		testutils.WithCustomFuncs(CancelOutstandingPollRequestFuzzer),
	)
}

func TestMatchingDescribeTaskListRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMatchingDescribeTaskListRequest, ToMatchingDescribeTaskListRequest)
}

func TestMatchingDescribeTaskListResponseFuzz(t *testing.T) {
	// PartitionConfig contains map[int] keys that truncate to int32 in proto;
	// specific partition scenarios are tested in TestToMatchingTaskListPartitionConfig.
	testutils.RunMapperFuzzTest(t, FromMatchingDescribeTaskListResponse, ToMatchingDescribeTaskListResponse,
		testutils.WithExcludedFields("PartitionConfig"),
	)
}

func TestMatchingListTaskListPartitionsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMatchingListTaskListPartitionsRequest, ToMatchingListTaskListPartitionsRequest)
}

func TestMatchingListTaskListPartitionsResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMatchingListTaskListPartitionsResponse, ToMatchingListTaskListPartitionsResponse)
}

func TestMatchingPollForActivityTaskRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMatchingPollForActivityTaskRequest, ToMatchingPollForActivityTaskRequest)
}

func TestMatchingPollForActivityTaskResponseFuzz(t *testing.T) {
	// [BUG] BacklogCountHint has no corresponding field in the activity task proto message; it is silently dropped.
	// PartitionConfig contains map[int] keys that truncate to int32 in proto;
	// specific partition scenarios are tested in TestToMatchingTaskListPartitionConfig.
	testutils.RunMapperFuzzTest(t, FromMatchingPollForActivityTaskResponse, ToMatchingPollForActivityTaskResponse,
		testutils.WithExcludedFields("BacklogCountHint", "PartitionConfig"),
	)
}

func TestMatchingPollForDecisionTaskRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMatchingPollForDecisionTaskRequest, ToMatchingPollForDecisionTaskRequest)
}

func TestMatchingPollForDecisionTaskResponseFuzz(t *testing.T) {
	// [BUG] Attempt is int64 in types but int32 in proto; values outside int32 range are truncated.
	// PartitionConfig contains map[int] keys that truncate to int32 in proto;
	// specific partition scenarios are tested in TestToMatchingTaskListPartitionConfig.
	// DecisionInfo contains HistoryEvent fields requiring non-nil EventType;
	// HistoryEvent fuzzing is tested in api_test.go (TestHistoryEventFuzz).
	testutils.RunMapperFuzzTest(t, FromMatchingPollForDecisionTaskResponse, ToMatchingPollForDecisionTaskResponse,
		testutils.WithExcludedFields("Attempt", "PartitionConfig", "DecisionInfo"),
	)
}

func TestMatchingQueryWorkflowRequestFuzz(t *testing.T) {
	// QueryWorkflowRequest contains QueryRejectCondition and QueryConsistencyLevel enums
	// that require valid values (0-1 each); out-of-range values map to nil on return.
	testutils.RunMapperFuzzTest(t, FromMatchingQueryWorkflowRequest, ToMatchingQueryWorkflowRequest,
		testutils.WithCustomFuncs(QueryRejectConditionFuzzer, QueryConsistencyLevelFuzzer),
	)
}

func TestMatchingQueryWorkflowResponseFuzz(t *testing.T) {
	// PartitionConfig contains map[int] keys that truncate to int32 in proto;
	// specific partition scenarios are tested in TestToMatchingTaskListPartitionConfig.
	testutils.RunMapperFuzzTest(t, FromMatchingQueryWorkflowResponse, ToMatchingQueryWorkflowResponse,
		testutils.WithExcludedFields("PartitionConfig"),
	)
}

func TestMatchingRespondQueryTaskCompletedRequestFuzz(t *testing.T) {
	// QueryTaskCompletedType only has 2 valid values (0=Completed, 1=Failed);
	// the default fuzzer generates arbitrary int values so CompletedTypeFuzzer constrains the range.
	testutils.RunMapperFuzzTest(t, FromMatchingRespondQueryTaskCompletedRequest, ToMatchingRespondQueryTaskCompletedRequest,
		testutils.WithCustomFuncs(CompletedTypeFuzzer),
	)
}

func TestMatchingGetTaskListsByDomainRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMatchingGetTaskListsByDomainRequest, ToMatchingGetTaskListsByDomainRequest)
}

func TestMatchingGetTaskListsByDomainResponseFuzz(t *testing.T) {
	// PartitionConfig contains map[int] keys that truncate to int32 in proto;
	// specific partition scenarios are tested in TestToMatchingTaskListPartitionConfig.
	testutils.RunMapperFuzzTest(t, FromMatchingGetTaskListsByDomainResponse, ToMatchingGetTaskListsByDomainResponse,
		testutils.WithExcludedFields("PartitionConfig"),
	)
}

func TestMatchingUpdateTaskListPartitionConfigRequestFuzz(t *testing.T) {
	// PartitionConfig contains map[int] keys that truncate to int32 in proto;
	// specific partition scenarios are tested in TestToMatchingTaskListPartitionConfig.
	testutils.RunMapperFuzzTest(t, FromMatchingUpdateTaskListPartitionConfigRequest, ToMatchingUpdateTaskListPartitionConfigRequest,
		testutils.WithExcludedFields("PartitionConfig"),
	)
}

func TestMatchingUpdateTaskListPartitionConfigResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMatchingUpdateTaskListPartitionConfigResponse, ToMatchingUpdateTaskListPartitionConfigResponse)
}

func TestMatchingRefreshTaskListPartitionConfigRequestFuzz(t *testing.T) {
	// PartitionConfig contains map[int] keys that truncate to int32 in proto;
	// specific partition scenarios are tested in TestToMatchingTaskListPartitionConfig.
	testutils.RunMapperFuzzTest(t, FromMatchingRefreshTaskListPartitionConfigRequest, ToMatchingRefreshTaskListPartitionConfigRequest,
		testutils.WithExcludedFields("PartitionConfig"),
	)
}

func TestMatchingRefreshTaskListPartitionConfigResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMatchingRefreshTaskListPartitionConfigResponse, ToMatchingRefreshTaskListPartitionConfigResponse)
}
