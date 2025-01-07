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

package thrift

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/.gen/go/admin"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/testdata"
)

func TestAdminAddSearchAttributeRequest(t *testing.T) {
	for _, item := range []*types.AddSearchAttributeRequest{nil, {}, &testdata.AdminAddSearchAttributeRequest} {
		assert.Equal(t, item, ToAdminAddSearchAttributeRequest(FromAdminAddSearchAttributeRequest(item)))
	}
}
func TestAdminCloseShardRequest(t *testing.T) {
	for _, item := range []*types.CloseShardRequest{nil, {}, &testdata.AdminCloseShardRequest} {
		assert.Equal(t, item, ToAdminCloseShardRequest(FromAdminCloseShardRequest(item)))
	}
}
func TestAdminDeleteWorkflowRequest(t *testing.T) {
	for _, item := range []*types.AdminDeleteWorkflowRequest{nil, {}, &testdata.AdminDeleteWorkflowRequest} {
		assert.Equal(t, item, ToAdminDeleteWorkflowRequest(FromAdminDeleteWorkflowRequest(item)))
	}
}
func TestAdminDeleteWorkflowResponse(t *testing.T) {
	for _, item := range []*types.AdminDeleteWorkflowResponse{nil, {}, &testdata.AdminDeleteWorkflowResponse} {
		assert.Equal(t, item, ToAdminDeleteWorkflowResponse(FromAdminDeleteWorkflowResponse(item)))
	}
}
func TestAdminDescribeClusterResponse(t *testing.T) {
	for _, item := range []*types.DescribeClusterResponse{nil, {}, &testdata.AdminDescribeClusterResponse} {
		assert.Equal(t, item, ToAdminDescribeClusterResponse(FromAdminDescribeClusterResponse(item)))
	}
}
func TestAdminDescribeHistoryHostRequest(t *testing.T) {
	for _, item := range []*types.DescribeHistoryHostRequest{
		nil,
		&testdata.AdminDescribeHistoryHostRequest_ByHost,
		&testdata.AdminDescribeHistoryHostRequest_ByShard,
		&testdata.AdminDescribeHistoryHostRequest_ByExecution,
	} {
		assert.Equal(t, item, ToAdminDescribeHistoryHostRequest(FromAdminDescribeHistoryHostRequest(item)))
	}
}
func TestAdminDescribeHistoryHostResponse(t *testing.T) {
	for _, item := range []*types.DescribeHistoryHostResponse{nil, {}, &testdata.AdminDescribeHistoryHostResponse} {
		assert.Equal(t, item, ToAdminDescribeHistoryHostResponse(FromAdminDescribeHistoryHostResponse(item)))
	}
}
func TestAdminDescribeQueueRequest(t *testing.T) {
	for _, item := range []*types.DescribeQueueRequest{nil, {}, &testdata.AdminDescribeQueueRequest} {
		assert.Equal(t, item, ToAdminDescribeQueueRequest(FromAdminDescribeQueueRequest(item)))
	}
}
func TestAdminDescribeQueueResponse(t *testing.T) {
	for _, item := range []*types.DescribeQueueResponse{nil, {}, &testdata.AdminDescribeQueueResponse} {
		assert.Equal(t, item, ToAdminDescribeQueueResponse(FromAdminDescribeQueueResponse(item)))
	}
}
func TestAdminDescribeShardDistributionRequest(t *testing.T) {
	for _, item := range []*types.DescribeShardDistributionRequest{nil, {}, &testdata.AdminDescribeShardDistributionRequest} {
		assert.Equal(t, item, ToAdminDescribeShardDistributionRequest(FromAdminDescribeShardDistributionRequest(item)))
	}
}
func TestAdminDescribeShardDistributionResponse(t *testing.T) {
	for _, item := range []*types.DescribeShardDistributionResponse{nil, {}, &testdata.AdminDescribeShardDistributionResponse} {
		assert.Equal(t, item, ToAdminDescribeShardDistributionResponse(FromAdminDescribeShardDistributionResponse(item)))
	}
}
func TestAdminDescribeWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.AdminDescribeWorkflowExecutionRequest{nil, {}, &testdata.AdminDescribeWorkflowExecutionRequest} {
		assert.Equal(t, item, ToAdminDescribeWorkflowExecutionRequest(FromAdminDescribeWorkflowExecutionRequest(item)))
	}
}
func TestAdminDescribeWorkflowExecutionResponse(t *testing.T) {
	for _, item := range []*types.AdminDescribeWorkflowExecutionResponse{nil, {ShardID: "0"}, &testdata.AdminDescribeWorkflowExecutionResponse} {
		assert.Equal(t, item, ToAdminDescribeWorkflowExecutionResponse(FromAdminDescribeWorkflowExecutionResponse(item)))
	}
}
func TestAdminGetDomainIsolationGroupsRequest(t *testing.T) {
	for _, item := range []*types.GetDomainIsolationGroupsRequest{nil, {}, &testdata.AdminGetDomainIsolationGroupsRequest} {
		assert.Equal(t, item, ToAdminGetDomainIsolationGroupsRequest(FromAdminGetDomainIsolationGroupsRequest(item)))
	}
}
func TestAdminGetDomainIsolationGroupsResponse(t *testing.T) {
	for _, item := range []*types.GetDomainIsolationGroupsResponse{nil, {}, &testdata.AdminGetDomainIsolationGroupsResponse} {
		assert.Equal(t, item, ToAdminGetDomainIsolationGroupsResponse(FromAdminGetDomainIsolationGroupsResponse(item)))
	}
}
func TestAdminGetDomainReplicationMessagesRequest(t *testing.T) {
	for _, item := range []*types.GetDomainReplicationMessagesRequest{nil, {}, &testdata.AdminGetDomainReplicationMessagesRequest} {
		assert.Equal(t, item, ToAdminGetDomainReplicationMessagesRequest(FromAdminGetDomainReplicationMessagesRequest(item)))
	}
}
func TestAdminGetDomainReplicationMessagesResponse(t *testing.T) {
	for _, item := range []*types.GetDomainReplicationMessagesResponse{nil, {}, &testdata.AdminGetDomainReplicationMessagesResponse} {
		assert.Equal(t, item, ToAdminGetDomainReplicationMessagesResponse(FromAdminGetDomainReplicationMessagesResponse(item)))
	}
}
func TestAdminGetDynamicConfigRequest(t *testing.T) {
	for _, item := range []*types.GetDynamicConfigRequest{nil, {}, &testdata.AdminGetDynamicConfigRequest} {
		assert.Equal(t, item, ToAdminGetDynamicConfigRequest(FromAdminGetDynamicConfigRequest(item)))
	}
}
func TestAdminGetDynamicConfigResponse(t *testing.T) {
	for _, item := range []*types.GetDynamicConfigResponse{nil, {}, &testdata.AdminGetDynamicConfigResponse} {
		assert.Equal(t, item, ToAdminGetDynamicConfigResponse(FromAdminGetDynamicConfigResponse(item)))
	}
}
func TestAdminGetGlobalIsolationGroupsRequest(t *testing.T) {
	for _, item := range []*types.GetGlobalIsolationGroupsRequest{nil, {}, &testdata.AdminGetGlobalIsolationGroupsRequest} {
		assert.Equal(t, item, ToAdminGetGlobalIsolationGroupsRequest(FromAdminGetGlobalIsolationGroupsRequest(item)))
	}
}
func TestAdminGetReplicationMessagesRequest(t *testing.T) {
	for _, item := range []*types.GetReplicationMessagesRequest{nil, {}, &testdata.AdminGetReplicationMessagesRequest} {
		assert.Equal(t, item, ToAdminGetReplicationMessagesRequest(FromAdminGetReplicationMessagesRequest(item)))
	}
}
func TestAdminGetReplicationMessagesResponse(t *testing.T) {
	for _, item := range []*types.GetReplicationMessagesResponse{nil, {}, &testdata.AdminGetReplicationMessagesResponse} {
		assert.Equal(t, item, ToAdminGetReplicationMessagesResponse(FromAdminGetReplicationMessagesResponse(item)))
	}
}
func TestAdminGetWorkflowExecutionRawHistoryV2Request(t *testing.T) {
	for _, item := range []*types.GetWorkflowExecutionRawHistoryV2Request{nil, {}, &testdata.AdminGetWorkflowExecutionRawHistoryV2Request} {
		assert.Equal(t, item, ToAdminGetWorkflowExecutionRawHistoryV2Request(FromAdminGetWorkflowExecutionRawHistoryV2Request(item)))
	}
}
func TestAdminGetWorkflowExecutionRawHistoryV2Response(t *testing.T) {
	for _, item := range []*types.GetWorkflowExecutionRawHistoryV2Response{nil, {}, &testdata.AdminGetWorkflowExecutionRawHistoryV2Response} {
		assert.Equal(t, item, ToAdminGetWorkflowExecutionRawHistoryV2Response(FromAdminGetWorkflowExecutionRawHistoryV2Response(item)))
	}
}
func TestAdminListDynamicConfigRequest(t *testing.T) {
	for _, item := range []*types.ListDynamicConfigRequest{nil, {}, &testdata.AdminListDynamicConfigRequest} {
		assert.Equal(t, item, ToAdminListDynamicConfigRequest(FromAdminListDynamicConfigRequest(item)))
	}
}
func TestAdminListDynamicConfigResponse(t *testing.T) {
	for _, item := range []*types.ListDynamicConfigResponse{nil, {}, &testdata.AdminListDynamicConfigResponse} {
		assert.Equal(t, item, ToAdminListDynamicConfigResponse(FromAdminListDynamicConfigResponse(item)))
	}
}
func TestAdminMaintainCorruptWorkflowRequest(t *testing.T) {
	for _, item := range []*types.AdminMaintainWorkflowRequest{nil, {}, &testdata.AdminMaintainCorruptWorkflowRequest} {
		assert.Equal(t, item, ToAdminMaintainCorruptWorkflowRequest(FromAdminMaintainCorruptWorkflowRequest(item)))
	}
}
func TestAdminMaintainCorruptWorkflowResponse(t *testing.T) {
	for _, item := range []*types.AdminMaintainWorkflowResponse{nil, {}, &testdata.AdminMaintainCorruptWorkflowResponse} {
		assert.Equal(t, item, ToAdminMaintainCorruptWorkflowResponse(FromAdminMaintainCorruptWorkflowResponse(item)))
	}
}
func TestAdminReapplyEventsRequest(t *testing.T) {
	for _, item := range []*types.ReapplyEventsRequest{nil, {}, &testdata.AdminReapplyEventsRequest} {
		assert.Equal(t, item, ToAdminReapplyEventsRequest(FromAdminReapplyEventsRequest(item)))
	}
}
func TestAdminRefreshWorkflowTasksRequest(t *testing.T) {
	for _, item := range []*types.RefreshWorkflowTasksRequest{nil, {}, &testdata.AdminRefreshWorkflowTasksRequest} {
		assert.Equal(t, item, ToAdminRefreshWorkflowTasksRequest(FromAdminRefreshWorkflowTasksRequest(item)))
	}
}
func TestAdminRemoveTaskRequest(t *testing.T) {
	for _, item := range []*types.RemoveTaskRequest{nil, {}, &testdata.AdminRemoveTaskRequest} {
		assert.Equal(t, item, ToAdminRemoveTaskRequest(FromAdminRemoveTaskRequest(item)))
	}
}
func TestAdminResendReplicationTasksRequest(t *testing.T) {
	for _, item := range []*types.ResendReplicationTasksRequest{nil, {}, &testdata.AdminResendReplicationTasksRequest} {
		assert.Equal(t, item, ToAdminResendReplicationTasksRequest(FromAdminResendReplicationTasksRequest(item)))
	}
}
func TestAdminResetQueueRequest(t *testing.T) {
	for _, item := range []*types.ResetQueueRequest{nil, {}, &testdata.AdminResetQueueRequest} {
		assert.Equal(t, item, ToAdminResetQueueRequest(FromAdminResetQueueRequest(item)))
	}
}

func TestAdminGetCrossClusterTasksRequest(t *testing.T) {
	for _, item := range []*types.GetCrossClusterTasksRequest{nil, {}, &testdata.AdminGetCrossClusterTasksRequest} {
		assert.Equal(t, item, ToAdminGetCrossClusterTasksRequest(FromAdminGetCrossClusterTasksRequest(item)))
	}
}

func TestAdminGetCrossClusterTasksResponse(t *testing.T) {
	for _, item := range []*types.GetCrossClusterTasksResponse{nil, {}, &testdata.AdminGetCrossClusterTasksResponse} {
		assert.Equal(t, item, ToAdminGetCrossClusterTasksResponse(FromAdminGetCrossClusterTasksResponse(item)))
	}
}

func TestAdminRespondCrossClusterTasksCompletedRequest(t *testing.T) {
	for _, item := range []*types.RespondCrossClusterTasksCompletedRequest{nil, {}, &testdata.AdminRespondCrossClusterTasksCompletedRequest} {
		assert.Equal(t, item, ToAdminRespondCrossClusterTasksCompletedRequest(FromAdminRespondCrossClusterTasksCompletedRequest(item)))
	}
}

func TestAdminRespondCrossClusterTasksCompletedResponse(t *testing.T) {
	for _, item := range []*types.RespondCrossClusterTasksCompletedResponse{nil, {}, &testdata.AdminRespondCrossClusterTasksCompletedResponse} {
		assert.Equal(t, item, ToAdminRespondCrossClusterTasksCompletedResponse(FromAdminRespondCrossClusterTasksCompletedResponse(item)))
	}
}
func TestAdminUpdateDomainIsolationGroupsRequest(t *testing.T) {
	for _, item := range []*types.UpdateDomainIsolationGroupsRequest{nil, {}, &testdata.AdminUpdateDomainIsolationGroupsRequest} {
		assert.Equal(t, item, ToAdminUpdateDomainIsolationGroupsRequest(FromAdminUpdateDomainIsolationGroupsRequest(item)))
	}
}
func TestAdminUpdateDomainIsolationGroupsResponse(t *testing.T) {
	for _, item := range []*types.UpdateDomainIsolationGroupsResponse{nil, {}, &testdata.AdminUpdateDomainIsolationGroupsResponse} {
		assert.Equal(t, item, ToAdminUpdateDomainIsolationGroupsResponse(FromAdminUpdateDomainIsolationGroupsResponse(item)))
	}
}
func TestAdminRestoreDynamicConfigRequest(t *testing.T) {
	for _, item := range []*types.RestoreDynamicConfigRequest{nil, {}, &testdata.AdminRestoreDynamicConfigRequest} {
		assert.Equal(t, item, ToAdminRestoreDynamicConfigRequest(FromAdminRestoreDynamicConfigRequest(item)))
	}
}
func TestAdminUpdateDynamicConfigRequest(t *testing.T) {
	for _, item := range []*types.UpdateDynamicConfigRequest{nil, {}, &testdata.AdminUpdateDynamicConfigRequest} {
		assert.Equal(t, item, ToAdminUpdateDynamicConfigRequest(FromAdminUpdateDynamicConfigRequest(item)))
	}
}
func TestAdminUpdateGlobalIsolationGroupsResponse(t *testing.T) {
	for _, item := range []*types.UpdateGlobalIsolationGroupsResponse{nil, {}, &testdata.AdminUpdateGlobalIsolationGroupsResponse} {
		assert.Equal(t, item, ToAdminUpdateGlobalIsolationGroupsResponse(FromAdminUpdateGlobalIsolationGroupsResponse(item)))
	}
}

func TestFromGetGlobalIsolationGroupsResponse(t *testing.T) {
	tests := map[string]struct {
		in       *types.GetGlobalIsolationGroupsResponse
		expected *admin.GetGlobalIsolationGroupsResponse
	}{
		"Valid mapping": {
			in: &types.GetGlobalIsolationGroupsResponse{
				IsolationGroups: types.IsolationGroupConfiguration{
					"zone 0": {
						Name:  "zone 0",
						State: types.IsolationGroupStateHealthy,
					},
					"zone 1": {
						Name:  "zone 1",
						State: types.IsolationGroupStateDrained,
					},
				},
			},
			expected: &admin.GetGlobalIsolationGroupsResponse{
				IsolationGroups: &shared.IsolationGroupConfiguration{
					IsolationGroups: []*shared.IsolationGroupPartition{
						{
							Name:  strPtr("zone 0"),
							State: igStatePtr(shared.IsolationGroupStateHealthy),
						},
						{
							Name:  strPtr("zone 1"),
							State: igStatePtr(shared.IsolationGroupStateDrained),
						},
					},
				},
			},
		},
		"nil - 1": {
			in:       &types.GetGlobalIsolationGroupsResponse{},
			expected: &admin.GetGlobalIsolationGroupsResponse{},
		},
		"nil - 2": {
			expected: nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			res := FromAdminGetGlobalIsolationGroupsResponse(td.in)
			if res != nil && res.IsolationGroups != nil {
				sort.Slice(res.IsolationGroups.IsolationGroups, func(i int, j int) bool {
					return *res.IsolationGroups.IsolationGroups[i].Name < *res.IsolationGroups.IsolationGroups[j].Name
				})
			}
			assert.Equal(t, td.expected, res, "expected value")
			roundTrip := ToAdminGetGlobalIsolationGroupsResponse(res)
			assert.Equal(t, td.in, roundTrip, "roundtrip value")
		})
	}
}

func TestToGetGlobalIsolationGroupsRequest(t *testing.T) {

	tests := map[string]struct {
		in       *admin.GetGlobalIsolationGroupsRequest
		expected *types.GetGlobalIsolationGroupsRequest
	}{
		"Valid mapping": {
			in:       &admin.GetGlobalIsolationGroupsRequest{},
			expected: &types.GetGlobalIsolationGroupsRequest{},
		},
		"nil - 2": {
			in:       &admin.GetGlobalIsolationGroupsRequest{},
			expected: &types.GetGlobalIsolationGroupsRequest{},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, ToAdminGetGlobalIsolationGroupsRequest(td.in))
		})
	}
}

func TestFromGetDomainIsolationGroupsResponse(t *testing.T) {
	tests := map[string]struct {
		in       *types.GetDomainIsolationGroupsResponse
		expected *admin.GetDomainIsolationGroupsResponse
	}{
		"Valid mapping": {
			in: &types.GetDomainIsolationGroupsResponse{
				IsolationGroups: types.IsolationGroupConfiguration{
					"zone 0": {
						Name:  "zone 0",
						State: types.IsolationGroupStateHealthy,
					},
					"zone 1": {
						Name:  "zone 1",
						State: types.IsolationGroupStateDrained,
					},
				},
			},
			expected: &admin.GetDomainIsolationGroupsResponse{
				IsolationGroups: &shared.IsolationGroupConfiguration{
					IsolationGroups: []*shared.IsolationGroupPartition{
						{
							Name:  strPtr("zone 0"),
							State: igStatePtr(shared.IsolationGroupStateHealthy),
						},
						{
							Name:  strPtr("zone 1"),
							State: igStatePtr(shared.IsolationGroupStateDrained),
						},
					},
				},
			},
		},
		"empty": {
			in: &types.GetDomainIsolationGroupsResponse{
				IsolationGroups: types.IsolationGroupConfiguration{},
			},
			expected: &admin.GetDomainIsolationGroupsResponse{
				IsolationGroups: &shared.IsolationGroupConfiguration{
					IsolationGroups: []*shared.IsolationGroupPartition{},
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			res := FromAdminGetDomainIsolationGroupsResponse(td.in)
			// map iteration is nondeterministic
			sort.Slice(res.IsolationGroups.IsolationGroups, func(i int, j int) bool {
				return *res.IsolationGroups.IsolationGroups[i].Name > *res.IsolationGroups.IsolationGroups[j].Name
			})
		})
	}
}

func TestToGetDomainIsolationGroupsRequest(t *testing.T) {

	tests := map[string]struct {
		in       *admin.GetDomainIsolationGroupsRequest
		expected *types.GetDomainIsolationGroupsRequest
	}{
		"Valid mapping": {
			in: &admin.GetDomainIsolationGroupsRequest{
				Domain: strPtr("domain123"),
			},
			expected: &types.GetDomainIsolationGroupsRequest{
				Domain: "domain123",
			},
		},
		"empty": {
			in:       &admin.GetDomainIsolationGroupsRequest{},
			expected: &types.GetDomainIsolationGroupsRequest{},
		},
		"nil": {
			in:       nil,
			expected: nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, ToAdminGetDomainIsolationGroupsRequest(td.in))
		})
	}
}

func TestFromUpdateGlobalIsolationGroupsResponse(t *testing.T) {

	tests := map[string]struct {
		in       *types.UpdateGlobalIsolationGroupsResponse
		expected *admin.UpdateGlobalIsolationGroupsResponse
	}{
		"Valid mapping": {},
		"empty": {
			in:       &types.UpdateGlobalIsolationGroupsResponse{},
			expected: &admin.UpdateGlobalIsolationGroupsResponse{},
		},
		"nil": {
			in:       nil,
			expected: nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, FromAdminUpdateGlobalIsolationGroupsResponse(td.in))
		})
	}
}

func TestToUpdateGlobalIsolationGroupsRequest(t *testing.T) {

	tests := map[string]struct {
		in       *admin.UpdateGlobalIsolationGroupsRequest
		expected *types.UpdateGlobalIsolationGroupsRequest
	}{
		"Valid mapping": {
			in: &admin.UpdateGlobalIsolationGroupsRequest{
				IsolationGroups: &shared.IsolationGroupConfiguration{
					IsolationGroups: []*shared.IsolationGroupPartition{
						{
							Name:  strPtr("zone 1"),
							State: igStatePtr(shared.IsolationGroupStateHealthy),
						},
						{
							Name:  strPtr("zone 2"),
							State: igStatePtr(shared.IsolationGroupStateDrained),
						},
					},
				},
			},
			expected: &types.UpdateGlobalIsolationGroupsRequest{
				IsolationGroups: types.IsolationGroupConfiguration{
					"zone 1": types.IsolationGroupPartition{
						Name:  "zone 1",
						State: types.IsolationGroupStateHealthy,
					},
					"zone 2": types.IsolationGroupPartition{
						Name:  "zone 2",
						State: types.IsolationGroupStateDrained,
					},
				},
			},
		},
		"empty": {
			in:       &admin.UpdateGlobalIsolationGroupsRequest{},
			expected: &types.UpdateGlobalIsolationGroupsRequest{},
		},
		"nil": {
			in:       nil,
			expected: nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			res := ToAdminUpdateGlobalIsolationGroupsRequest(td.in)
			assert.Equal(t, td.expected, res)
			roundTrip := FromAdminUpdateGlobalIsolationGroupsRequest(res)
			if td.in != nil {
				assert.Equal(t, td.in, roundTrip)
			}
		})
	}
}

func TestFromUpdateDomainIsolationGroupsResponse(t *testing.T) {

	tests := map[string]struct {
		in       *types.UpdateDomainIsolationGroupsResponse
		expected *admin.UpdateDomainIsolationGroupsResponse
	}{
		"empty": {
			in:       &types.UpdateDomainIsolationGroupsResponse{},
			expected: &admin.UpdateDomainIsolationGroupsResponse{},
		},
		"nil": {
			in:       nil,
			expected: nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, FromAdminUpdateDomainIsolationGroupsResponse(td.in))
		})
	}
}

func TestToUpdateDomainIsolationGroupsRequest(t *testing.T) {

	tests := map[string]struct {
		in       *admin.UpdateDomainIsolationGroupsRequest
		expected *types.UpdateDomainIsolationGroupsRequest
	}{
		"valid": {
			in: &admin.UpdateDomainIsolationGroupsRequest{
				Domain: strPtr("test-domain"),
				IsolationGroups: &shared.IsolationGroupConfiguration{
					IsolationGroups: []*shared.IsolationGroupPartition{
						{
							Name:  strPtr("zone-1"),
							State: igStatePtr(shared.IsolationGroupStateHealthy),
						},
						{
							Name:  strPtr("zone-2"),
							State: igStatePtr(shared.IsolationGroupStateDrained),
						},
					},
				},
			},
			expected: &types.UpdateDomainIsolationGroupsRequest{
				Domain: "test-domain",
				IsolationGroups: types.IsolationGroupConfiguration{
					"zone-1": types.IsolationGroupPartition{
						Name:  "zone-1",
						State: types.IsolationGroupStateHealthy,
					},
					"zone-2": types.IsolationGroupPartition{
						Name:  "zone-2",
						State: types.IsolationGroupStateDrained,
					},
				},
			},
		},
		"nil": {
			in:       nil,
			expected: nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, ToAdminUpdateDomainIsolationGroupsRequest(td.in))
		})
	}
}

func TestToGetGlobalIsolationGroupsResponse(t *testing.T) {

	tests := map[string]struct {
		in       *admin.GetGlobalIsolationGroupsResponse
		expected *types.GetGlobalIsolationGroupsResponse
	}{
		"valid": {
			in: &admin.GetGlobalIsolationGroupsResponse{
				IsolationGroups: &shared.IsolationGroupConfiguration{
					IsolationGroups: []*shared.IsolationGroupPartition{
						{
							Name:  strPtr("zone-1"),
							State: igStatePtr(shared.IsolationGroupStateDrained),
						},
						{
							Name:  strPtr("zone-2"),
							State: igStatePtr(shared.IsolationGroupStateHealthy),
						},
					},
				},
			},
			expected: &types.GetGlobalIsolationGroupsResponse{
				IsolationGroups: map[string]types.IsolationGroupPartition{
					"zone-1": {
						Name:  "zone-1",
						State: types.IsolationGroupStateDrained,
					},
					"zone-2": {
						Name:  "zone-2",
						State: types.IsolationGroupStateHealthy,
					},
				},
			},
		},
		"no groups": {
			in:       &admin.GetGlobalIsolationGroupsResponse{},
			expected: &types.GetGlobalIsolationGroupsResponse{},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, ToAdminGetGlobalIsolationGroupsResponse(td.in))
		})
	}
}

func TestToAdminUpdateDomainAsyncWorkflowConfiguratonRequest(t *testing.T) {
	enabled := true
	tests := map[string]struct {
		input *admin.UpdateDomainAsyncWorkflowConfiguratonRequest
		want  *types.UpdateDomainAsyncWorkflowConfiguratonRequest
	}{
		"empty": {
			input: &admin.UpdateDomainAsyncWorkflowConfiguratonRequest{},
			want:  &types.UpdateDomainAsyncWorkflowConfiguratonRequest{},
		},
		"nil": {
			input: nil,
			want:  nil,
		},
		"predefined queue": {
			input: &admin.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: strPtr("test-domain"),
				Configuration: &shared.AsyncWorkflowConfiguration{
					Enabled:             &enabled,
					PredefinedQueueName: strPtr("test-queue"),
				},
			},
			want: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:             enabled,
					PredefinedQueueName: "test-queue",
				},
			},
		},
		"inline queue": {
			input: &admin.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: strPtr("test-domain"),
				Configuration: &shared.AsyncWorkflowConfiguration{
					Enabled:   &enabled,
					QueueType: strPtr("kafka"),
					QueueConfig: &shared.DataBlob{
						EncodingType: shared.EncodingTypeJSON.Ptr(),
						Data:         []byte("test-data"),
					},
				},
			},
			want: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:   enabled,
					QueueType: "kafka",
					QueueConfig: &types.DataBlob{
						EncodingType: types.EncodingTypeJSON.Ptr(),
						Data:         []byte("test-data"),
					},
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.want, ToAdminUpdateDomainAsyncWorkflowConfiguratonRequest(td.input))
		})
	}
}

func TestFromAdminUpdateDomainAsyncWorkflowConfiguratonResponse(t *testing.T) {
	tests := map[string]struct {
		input *types.UpdateDomainAsyncWorkflowConfiguratonResponse
		want  *admin.UpdateDomainAsyncWorkflowConfiguratonResponse
	}{
		"empty": {
			input: &types.UpdateDomainAsyncWorkflowConfiguratonResponse{},
			want:  &admin.UpdateDomainAsyncWorkflowConfiguratonResponse{},
		},
		"nil": {
			input: nil,
			want:  nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.want, FromAdminUpdateDomainAsyncWorkflowConfiguratonResponse(td.input))
		})
	}
}

func TestToAdminGetDomainAsyncWorkflowConfiguratonRequest(t *testing.T) {
	tests := map[string]struct {
		input *admin.GetDomainAsyncWorkflowConfiguratonRequest
		want  *types.GetDomainAsyncWorkflowConfiguratonRequest
	}{
		"empty": {
			input: &admin.GetDomainAsyncWorkflowConfiguratonRequest{},
			want:  &types.GetDomainAsyncWorkflowConfiguratonRequest{},
		},
		"nil": {
			input: nil,
			want:  nil,
		},
		"valid": {
			input: &admin.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: strPtr("test-domain"),
			},
			want: &types.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.want, ToAdminGetDomainAsyncWorkflowConfiguratonRequest(td.input))
		})
	}
}

func TestFromAdminGetDomainAsyncWorkflowConfiguratonResponse(t *testing.T) {
	enabled := true
	tests := map[string]struct {
		input *types.GetDomainAsyncWorkflowConfiguratonResponse
		want  *admin.GetDomainAsyncWorkflowConfiguratonResponse
	}{
		"empty": {
			input: &types.GetDomainAsyncWorkflowConfiguratonResponse{},
			want:  &admin.GetDomainAsyncWorkflowConfiguratonResponse{},
		},
		"nil": {
			input: nil,
			want:  nil,
		},
		"predefined queue": {
			input: &types.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:             enabled,
					PredefinedQueueName: "test-queue",
				},
			},
			want: &admin.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &shared.AsyncWorkflowConfiguration{
					Enabled:             &enabled,
					PredefinedQueueName: strPtr("test-queue"),
					QueueType:           strPtr(""),
				},
			},
		},
		"inline queue": {
			input: &types.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:   enabled,
					QueueType: "kafka",
					QueueConfig: &types.DataBlob{
						EncodingType: types.EncodingTypeJSON.Ptr(),
						Data:         []byte("test-data"),
					},
				},
			},
			want: &admin.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &shared.AsyncWorkflowConfiguration{
					Enabled:             &enabled,
					PredefinedQueueName: strPtr(""),
					QueueType:           strPtr("kafka"),
					QueueConfig: &shared.DataBlob{
						EncodingType: shared.EncodingTypeJSON.Ptr(),
						Data:         []byte("test-data"),
					},
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.want, FromAdminGetDomainAsyncWorkflowConfiguratonResponse(td.input))
		})
	}
}

func TestFromAdminGetDomainAsyncWorkflowConfiguratonRequest(t *testing.T) {
	tests := map[string]struct {
		input *types.GetDomainAsyncWorkflowConfiguratonRequest
		want  *admin.GetDomainAsyncWorkflowConfiguratonRequest
	}{
		"empty": {
			input: &types.GetDomainAsyncWorkflowConfiguratonRequest{},
			want: &admin.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: strPtr(""),
			},
		},
		"nil": {
			input: nil,
			want:  nil,
		},
		"valid": {
			input: &types.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
			want: &admin.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: strPtr("test-domain"),
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.want, FromAdminGetDomainAsyncWorkflowConfiguratonRequest(td.input))
		})
	}
}

func TestToAdminGetDomainAsyncWorkflowConfiguratonResponse(t *testing.T) {
	enabled := true
	tests := map[string]struct {
		input *admin.GetDomainAsyncWorkflowConfiguratonResponse
		want  *types.GetDomainAsyncWorkflowConfiguratonResponse
	}{
		"empty": {
			input: &admin.GetDomainAsyncWorkflowConfiguratonResponse{},
			want:  &types.GetDomainAsyncWorkflowConfiguratonResponse{},
		},
		"nil": {
			input: nil,
			want:  nil,
		},
		"predefined queue": {
			input: &admin.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &shared.AsyncWorkflowConfiguration{
					Enabled:             &enabled,
					PredefinedQueueName: strPtr("test-queue"),
				},
			},
			want: &types.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:             enabled,
					PredefinedQueueName: "test-queue",
				},
			},
		},
		"inline queue": {
			input: &admin.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &shared.AsyncWorkflowConfiguration{
					Enabled:   &enabled,
					QueueType: strPtr("kafka"),
					QueueConfig: &shared.DataBlob{
						EncodingType: shared.EncodingTypeJSON.Ptr(),
						Data:         []byte("test-data"),
					},
				},
			},
			want: &types.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:   enabled,
					QueueType: "kafka",
					QueueConfig: &types.DataBlob{
						EncodingType: types.EncodingTypeJSON.Ptr(),
						Data:         []byte("test-data"),
					},
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.want, ToAdminGetDomainAsyncWorkflowConfiguratonResponse(td.input))
		})
	}
}

func TestFromAdminUpdateDomainAsyncWorkflowConfiguratonRequest(t *testing.T) {
	enabled := true
	tests := map[string]struct {
		input *types.UpdateDomainAsyncWorkflowConfiguratonRequest
		want  *admin.UpdateDomainAsyncWorkflowConfiguratonRequest
	}{
		"empty": {
			input: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{},
			want: &admin.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: strPtr(""),
			},
		},
		"nil": {
			input: nil,
			want:  nil,
		},
		"predefined queue": {
			input: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:             enabled,
					PredefinedQueueName: "test-queue",
				},
			},
			want: &admin.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: strPtr("test-domain"),
				Configuration: &shared.AsyncWorkflowConfiguration{
					Enabled:             &enabled,
					PredefinedQueueName: strPtr("test-queue"),
					QueueType:           strPtr(""),
				},
			},
		},
		"inline queue": {
			input: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:   enabled,
					QueueType: "kafka",
					QueueConfig: &types.DataBlob{
						EncodingType: types.EncodingTypeJSON.Ptr(),
						Data:         []byte("test-data"),
					},
				},
			},
			want: &admin.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: strPtr("test-domain"),
				Configuration: &shared.AsyncWorkflowConfiguration{
					Enabled:             &enabled,
					PredefinedQueueName: strPtr(""),
					QueueType:           strPtr("kafka"),
					QueueConfig: &shared.DataBlob{
						EncodingType: shared.EncodingTypeJSON.Ptr(),
						Data:         []byte("test-data"),
					},
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.want, FromAdminUpdateDomainAsyncWorkflowConfiguratonRequest(td.input))
		})
	}
}

func TestToAdminUpdateDomainAsyncWorkflowConfiguratonResponse(t *testing.T) {
	tests := map[string]struct {
		input *admin.UpdateDomainAsyncWorkflowConfiguratonResponse
		want  *types.UpdateDomainAsyncWorkflowConfiguratonResponse
	}{
		"empty": {
			input: &admin.UpdateDomainAsyncWorkflowConfiguratonResponse{},
			want:  &types.UpdateDomainAsyncWorkflowConfiguratonResponse{},
		},
		"nil": {
			input: nil,
			want:  nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.want, ToAdminUpdateDomainAsyncWorkflowConfiguratonResponse(td.input))
		})
	}
}
