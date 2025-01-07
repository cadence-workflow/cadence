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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	v1 "github.com/uber/cadence-idl/go/proto/api/v1"

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
	assert.Panics(t, func() { ToAdminDescribeHistoryHostRequest(&adminv1.DescribeHistoryHostRequest{}) })
	assert.Panics(t, func() { FromAdminDescribeHistoryHostRequest(&types.DescribeHistoryHostRequest{}) })
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
func TestAdminGetDLQReplicationMessagesRequest(t *testing.T) {
	for _, item := range []*types.GetDLQReplicationMessagesRequest{nil, {}, &testdata.AdminGetDLQReplicationMessagesRequest} {
		assert.Equal(t, item, ToAdminGetDLQReplicationMessagesRequest(FromAdminGetDLQReplicationMessagesRequest(item)))
	}
}
func TestAdminGetDLQReplicationMessagesResponse(t *testing.T) {
	for _, item := range []*types.GetDLQReplicationMessagesResponse{nil, {}, &testdata.AdminGetDLQReplicationMessagesResponse} {
		assert.Equal(t, item, ToAdminGetDLQReplicationMessagesResponse(FromAdminGetDLQReplicationMessagesResponse(item)))
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
func TestAdminCountDLQMessagesRequest(t *testing.T) {
	for _, item := range []*types.CountDLQMessagesRequest{nil, {}, &testdata.AdminCountDLQMessagesRequest} {
		assert.Equal(t, item, ToAdminCountDLQMessagesRequest(FromAdminCountDLQMessagesRequest(item)))
	}
}
func TestAdminCountDLQMessagesResponse(t *testing.T) {
	for _, item := range []*types.CountDLQMessagesResponse{nil, {}, &testdata.AdminCountDLQMessagesResponse} {
		assert.Equal(t, item, ToAdminCountDLQMessagesResponse(FromAdminCountDLQMessagesResponse(item)))
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
func TestAdminMergeDLQMessagesRequest(t *testing.T) {
	for _, item := range []*types.MergeDLQMessagesRequest{nil, {}, &testdata.AdminMergeDLQMessagesRequest} {
		assert.Equal(t, item, ToAdminMergeDLQMessagesRequest(FromAdminMergeDLQMessagesRequest(item)))
	}
}
func TestAdminMergeDLQMessagesResponse(t *testing.T) {
	for _, item := range []*types.MergeDLQMessagesResponse{nil, {}, &testdata.AdminMergeDLQMessagesResponse} {
		assert.Equal(t, item, ToAdminMergeDLQMessagesResponse(FromAdminMergeDLQMessagesResponse(item)))
	}
}
func TestAdminPurgeDLQMessagesRequest(t *testing.T) {
	for _, item := range []*types.PurgeDLQMessagesRequest{nil, {}, &testdata.AdminPurgeDLQMessagesRequest} {
		assert.Equal(t, item, ToAdminPurgeDLQMessagesRequest(FromAdminPurgeDLQMessagesRequest(item)))
	}
}
func TestAdminReadDLQMessagesRequest(t *testing.T) {
	for _, item := range []*types.ReadDLQMessagesRequest{nil, {}, &testdata.AdminReadDLQMessagesRequest} {
		assert.Equal(t, item, ToAdminReadDLQMessagesRequest(FromAdminReadDLQMessagesRequest(item)))
	}
}
func TestAdminReadDLQMessagesResponse(t *testing.T) {
	for _, item := range []*types.ReadDLQMessagesResponse{nil, {}, &testdata.AdminReadDLQMessagesResponse} {
		assert.Equal(t, item, ToAdminReadDLQMessagesResponse(FromAdminReadDLQMessagesResponse(item)))
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

func TestFromAdminGetGlobalIsolationGroupsResponse(t *testing.T) {
	tests := map[string]struct {
		in       *types.GetGlobalIsolationGroupsResponse
		expected *adminv1.GetGlobalIsolationGroupsResponse
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
			expected: &adminv1.GetGlobalIsolationGroupsResponse{
				IsolationGroups: &v1.IsolationGroupConfiguration{
					IsolationGroups: []*v1.IsolationGroupPartition{
						{
							Name:  "zone 0",
							State: v1.IsolationGroupState_ISOLATION_GROUP_STATE_HEALTHY,
						},
						{
							Name:  "zone 1",
							State: v1.IsolationGroupState_ISOLATION_GROUP_STATE_DRAINED,
						},
					},
				},
			},
		},
		"nil - 1": {
			in: &types.GetGlobalIsolationGroupsResponse{
				IsolationGroups: types.IsolationGroupConfiguration{},
			},
			expected: &adminv1.GetGlobalIsolationGroupsResponse{
				IsolationGroups: &v1.IsolationGroupConfiguration{},
			},
		},
		"nil - 2": {
			expected: nil,
		},
		"nil - 3": {
			in: &types.GetGlobalIsolationGroupsResponse{
				IsolationGroups: nil,
			},
			expected: &adminv1.GetGlobalIsolationGroupsResponse{
				IsolationGroups: nil,
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			res := FromAdminGetGlobalIsolationGroupsResponse(td.in)
			assert.Equal(t, td.expected, res, "mapping")
			roundTrip := ToAdminGetGlobalIsolationGroupsResponse(res)
			if td.in != nil {
				assert.Equal(t, td.in, roundTrip, "roundtrip")
			}
		})
	}
}

func TestToAdminGetGlobalIsolationGroupsRequest(t *testing.T) {

	tests := map[string]struct {
		in       *adminv1.GetGlobalIsolationGroupsRequest
		expected *types.GetGlobalIsolationGroupsRequest
	}{
		"Valid mapping": {
			in:       &adminv1.GetGlobalIsolationGroupsRequest{},
			expected: &types.GetGlobalIsolationGroupsRequest{},
		},
		"nil - 2": {
			in:       &adminv1.GetGlobalIsolationGroupsRequest{},
			expected: &types.GetGlobalIsolationGroupsRequest{},
		},
		"nil": {
			in:       nil,
			expected: nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, ToAdminGetGlobalIsolationGroupsRequest(td.in))
		})
	}
}

func TestFromAdminGetDomainIsolationGroupsResponse(t *testing.T) {
	tests := map[string]struct {
		in       *types.GetDomainIsolationGroupsResponse
		expected *adminv1.GetDomainIsolationGroupsResponse
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
			expected: &adminv1.GetDomainIsolationGroupsResponse{
				IsolationGroups: &v1.IsolationGroupConfiguration{
					IsolationGroups: []*v1.IsolationGroupPartition{
						{
							Name:  "zone 0",
							State: v1.IsolationGroupState_ISOLATION_GROUP_STATE_HEALTHY,
						},
						{
							Name:  "zone 1",
							State: v1.IsolationGroupState_ISOLATION_GROUP_STATE_DRAINED,
						},
					},
				},
			},
		},
		"empty": {
			in: &types.GetDomainIsolationGroupsResponse{
				IsolationGroups: types.IsolationGroupConfiguration{},
			},
			expected: &adminv1.GetDomainIsolationGroupsResponse{
				IsolationGroups: &v1.IsolationGroupConfiguration{
					IsolationGroups: []*v1.IsolationGroupPartition{},
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			res := FromAdminGetDomainIsolationGroupsResponse(td.in)
			// map iteration is nondeterministic
			sort.Slice(res.IsolationGroups.IsolationGroups, func(i int, j int) bool {
				return res.IsolationGroups.IsolationGroups[i].Name > res.IsolationGroups.IsolationGroups[j].Name
			})
		})
	}
}

func TestToAdminGetDomainIsolationGroupsRequest(t *testing.T) {

	tests := map[string]struct {
		in       *adminv1.GetDomainIsolationGroupsRequest
		expected *types.GetDomainIsolationGroupsRequest
	}{
		"Valid mapping": {
			in: &adminv1.GetDomainIsolationGroupsRequest{
				Domain: "domain123",
			},
			expected: &types.GetDomainIsolationGroupsRequest{
				Domain: "domain123",
			},
		},
		"empty": {
			in:       &adminv1.GetDomainIsolationGroupsRequest{},
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

func TestFromAdminUpdateGlobalIsolationGroupsResponse(t *testing.T) {

	tests := map[string]struct {
		in       *types.UpdateGlobalIsolationGroupsResponse
		expected *adminv1.UpdateGlobalIsolationGroupsResponse
	}{
		"Valid mapping": {},
		"empty": {
			in:       &types.UpdateGlobalIsolationGroupsResponse{},
			expected: &adminv1.UpdateGlobalIsolationGroupsResponse{},
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

func TestToAdminUpdateGlobalIsolationGroupsRequest(t *testing.T) {

	tests := map[string]struct {
		in       *adminv1.UpdateGlobalIsolationGroupsRequest
		expected *types.UpdateGlobalIsolationGroupsRequest
	}{
		"Valid mapping": {
			in: &adminv1.UpdateGlobalIsolationGroupsRequest{
				IsolationGroups: &v1.IsolationGroupConfiguration{
					IsolationGroups: []*v1.IsolationGroupPartition{
						{
							Name:  "zone 1",
							State: v1.IsolationGroupState_ISOLATION_GROUP_STATE_HEALTHY,
						},
						{
							Name:  "zone 2",
							State: v1.IsolationGroupState_ISOLATION_GROUP_STATE_DRAINED,
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
			in:       &adminv1.UpdateGlobalIsolationGroupsRequest{},
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
			assert.Equal(t, td.expected, res, "conversion")
			roundTrip := FromAdminUpdateGlobalIsolationGroupsRequest(res)
			if td.in != nil {
				assert.Equal(t, td.in, roundTrip, "roundtrip")
			}
		})
	}
}

func TestFromAdminUpdateDomainIsolationGroupsResponse(t *testing.T) {

	tests := map[string]struct {
		in       *types.UpdateDomainIsolationGroupsResponse
		expected *adminv1.UpdateDomainIsolationGroupsResponse
	}{
		"empty": {
			in:       &types.UpdateDomainIsolationGroupsResponse{},
			expected: &adminv1.UpdateDomainIsolationGroupsResponse{},
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
		in       *adminv1.UpdateDomainIsolationGroupsRequest
		expected *types.UpdateDomainIsolationGroupsRequest
	}{
		"valid": {
			in: &adminv1.UpdateDomainIsolationGroupsRequest{
				Domain: "test-domain",
				IsolationGroups: &v1.IsolationGroupConfiguration{
					IsolationGroups: []*v1.IsolationGroupPartition{
						{
							Name:  "zone-1",
							State: v1.IsolationGroupState_ISOLATION_GROUP_STATE_HEALTHY,
						},
						{
							Name:  "zone-2",
							State: v1.IsolationGroupState_ISOLATION_GROUP_STATE_DRAINED,
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

func TestToAdminGetDomainAsyncWorkflowConfiguratonRequest(t *testing.T) {
	tests := map[string]struct {
		in       *adminv1.GetDomainAsyncWorkflowConfiguratonRequest
		expected *types.GetDomainAsyncWorkflowConfiguratonRequest
	}{
		"nil": {
			in:       nil,
			expected: nil,
		},
		"empty": {
			in:       &adminv1.GetDomainAsyncWorkflowConfiguratonRequest{},
			expected: &types.GetDomainAsyncWorkflowConfiguratonRequest{},
		},
		"valid": {
			in: &adminv1.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
			expected: &types.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, ToAdminGetDomainAsyncWorkflowConfiguratonRequest(td.in))
		})
	}
}

func TestFromAdminGetDomainAsyncWorkflowConfiguratonResponse(t *testing.T) {
	tests := map[string]struct {
		in       *types.GetDomainAsyncWorkflowConfiguratonResponse
		expected *adminv1.GetDomainAsyncWorkflowConfiguratonResponse
	}{
		"nil": {
			in:       nil,
			expected: nil,
		},
		"empty": {
			in:       &types.GetDomainAsyncWorkflowConfiguratonResponse{},
			expected: &adminv1.GetDomainAsyncWorkflowConfiguratonResponse{},
		},
		"predefined queue": {
			in: &types.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:             true,
					PredefinedQueueName: "test-queue",
				},
			},
			expected: &adminv1.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &v1.AsyncWorkflowConfiguration{
					Enabled:             true,
					PredefinedQueueName: "test-queue",
				},
			},
		},
		"inline queue": {
			in: &types.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:   true,
					QueueType: "kafka",
					QueueConfig: &types.DataBlob{
						EncodingType: types.EncodingTypeJSON.Ptr(),
						Data:         []byte(`{"topic":"test-topic","dlq_topic":"test-dlq-topic","consumer_group":"test-consumer-group","brokers":["test-broker-1","test-broker-2"],"properties":{"test-key-1":"test-value-1"}}`),
					},
				},
			},
			expected: &adminv1.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &v1.AsyncWorkflowConfiguration{
					Enabled:   true,
					QueueType: "kafka",
					QueueConfig: &v1.DataBlob{
						EncodingType: v1.EncodingType_ENCODING_TYPE_JSON,
						Data:         []byte(`{"topic":"test-topic","dlq_topic":"test-dlq-topic","consumer_group":"test-consumer-group","brokers":["test-broker-1","test-broker-2"],"properties":{"test-key-1":"test-value-1"}}`),
					},
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, FromAdminGetDomainAsyncWorkflowConfiguratonResponse(td.in))
		})
	}
}

func TestToAdminUpdateDomainAsyncWorkflowConfiguratonRequest(t *testing.T) {
	tests := map[string]struct {
		in       *adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest
		expected *types.UpdateDomainAsyncWorkflowConfiguratonRequest
	}{
		"nil": {
			in:       nil,
			expected: nil,
		},
		"empty": {
			in:       &adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest{},
			expected: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{},
		},
		"predefined queue": {
			in: &adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &v1.AsyncWorkflowConfiguration{
					Enabled:             true,
					PredefinedQueueName: "test-queue",
				},
			},
			expected: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:             true,
					PredefinedQueueName: "test-queue",
				},
			},
		},
		"inline queue": {
			in: &adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &v1.AsyncWorkflowConfiguration{
					Enabled:   true,
					QueueType: "kafka",
					QueueConfig: &v1.DataBlob{
						EncodingType: v1.EncodingType_ENCODING_TYPE_JSON,
						Data:         []byte(`{"topic":"test-topic","dlq_topic":"test-dlq-topic","consumer_group":"test-consumer-group","brokers":["test-broker-1","test-broker-2"],"properties":{"test-key-1":"test-value-1"}}`),
					},
				},
			},
			expected: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:   true,
					QueueType: "kafka",
					QueueConfig: &types.DataBlob{
						EncodingType: types.EncodingTypeJSON.Ptr(),
						Data:         []byte(`{"topic":"test-topic","dlq_topic":"test-dlq-topic","consumer_group":"test-consumer-group","brokers":["test-broker-1","test-broker-2"],"properties":{"test-key-1":"test-value-1"}}`),
					},
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, ToAdminUpdateDomainAsyncWorkflowConfiguratonRequest(td.in))
		})
	}
}

func TestFromAdminUpdateDomainAsyncWorkflowConfiguratonResponse(t *testing.T) {
	tests := map[string]struct {
		in       *types.UpdateDomainAsyncWorkflowConfiguratonResponse
		expected *adminv1.UpdateDomainAsyncWorkflowConfiguratonResponse
	}{
		"nil": {
			in:       nil,
			expected: nil,
		},
		"empty": {
			in:       &types.UpdateDomainAsyncWorkflowConfiguratonResponse{},
			expected: &adminv1.UpdateDomainAsyncWorkflowConfiguratonResponse{},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, FromAdminUpdateDomainAsyncWorkflowConfiguratonResponse(td.in))
		})
	}
}

func TestFromAdminGetDomainAsyncWorkflowConfiguratonRequest(t *testing.T) {
	tests := map[string]struct {
		in       *types.GetDomainAsyncWorkflowConfiguratonRequest
		expected *adminv1.GetDomainAsyncWorkflowConfiguratonRequest
	}{
		"nil": {
			in:       nil,
			expected: nil,
		},
		"empty": {
			in:       &types.GetDomainAsyncWorkflowConfiguratonRequest{},
			expected: &adminv1.GetDomainAsyncWorkflowConfiguratonRequest{},
		},
		"valid": {
			in: &types.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
			expected: &adminv1.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, FromAdminGetDomainAsyncWorkflowConfiguratonRequest(td.in))
		})
	}
}

func TestToAdminGetDomainAsyncWorkflowConfiguratonResponse(t *testing.T) {
	tests := map[string]struct {
		in       *adminv1.GetDomainAsyncWorkflowConfiguratonResponse
		expected *types.GetDomainAsyncWorkflowConfiguratonResponse
	}{
		"nil": {
			in:       nil,
			expected: nil,
		},
		"empty": {
			in:       &adminv1.GetDomainAsyncWorkflowConfiguratonResponse{},
			expected: &types.GetDomainAsyncWorkflowConfiguratonResponse{},
		},
		"predefined queue": {
			in: &adminv1.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &v1.AsyncWorkflowConfiguration{
					PredefinedQueueName: "test-queue",
				},
			},
			expected: &types.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &types.AsyncWorkflowConfiguration{
					PredefinedQueueName: "test-queue",
				},
			},
		},
		"inline queue": {
			in: &adminv1.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &v1.AsyncWorkflowConfiguration{
					Enabled:   true,
					QueueType: "kafka",
					QueueConfig: &v1.DataBlob{
						EncodingType: v1.EncodingType_ENCODING_TYPE_JSON,
						Data:         []byte(`{"topic":"test-topic","dlq_topic":"test-dlq-topic","consumer_group":"test-consumer-group","brokers":["test-broker-1","test-broker-2"],"properties":{"test-key-1":"test-value-1"}}`),
					},
				},
			},
			expected: &types.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:   true,
					QueueType: "kafka",
					QueueConfig: &types.DataBlob{
						EncodingType: types.EncodingTypeJSON.Ptr(),
						Data:         []byte(`{"topic":"test-topic","dlq_topic":"test-dlq-topic","consumer_group":"test-consumer-group","brokers":["test-broker-1","test-broker-2"],"properties":{"test-key-1":"test-value-1"}}`),
					},
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, ToAdminGetDomainAsyncWorkflowConfiguratonResponse(td.in))
		})
	}
}

func TestFromAdminUpdateDomainAsyncWorkflowConfiguratonRequest(t *testing.T) {
	tests := map[string]struct {
		in       *types.UpdateDomainAsyncWorkflowConfiguratonRequest
		expected *adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest
	}{
		"nil": {
			in:       nil,
			expected: nil,
		},
		"empty": {
			in:       &types.UpdateDomainAsyncWorkflowConfiguratonRequest{},
			expected: &adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest{},
		},
		"predefined queue": {
			in: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					PredefinedQueueName: "test-queue",
				},
			},
			expected: &adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &v1.AsyncWorkflowConfiguration{
					PredefinedQueueName: "test-queue",
				},
			},
		},
		"kafka inline queue": {
			in: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:   true,
					QueueType: "kafka",
					QueueConfig: &types.DataBlob{
						EncodingType: types.EncodingTypeJSON.Ptr(),
						Data:         []byte(`{"topic":"test-topic","dlq_topic":"test-dlq-topic","consumer_group":"test-consumer-group","brokers":["test-broker-1","test-broker-2"],"properties":{"test-key-1":"test-value-1"}}`),
					},
				},
			},
			expected: &adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &v1.AsyncWorkflowConfiguration{
					Enabled:   true,
					QueueType: "kafka",
					QueueConfig: &v1.DataBlob{
						EncodingType: v1.EncodingType_ENCODING_TYPE_JSON,
						Data:         []byte(`{"topic":"test-topic","dlq_topic":"test-dlq-topic","consumer_group":"test-consumer-group","brokers":["test-broker-1","test-broker-2"],"properties":{"test-key-1":"test-value-1"}}`),
					},
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, FromAdminUpdateDomainAsyncWorkflowConfiguratonRequest(td.in))
		})
	}
}

func TestToAdminUpdateDomainAsyncWorkflowConfiguratonResponse(t *testing.T) {
	tests := map[string]struct {
		in       *adminv1.UpdateDomainAsyncWorkflowConfiguratonResponse
		expected *types.UpdateDomainAsyncWorkflowConfiguratonResponse
	}{
		"nil": {
			in:       nil,
			expected: nil,
		},
		"empty": {
			in:       &adminv1.UpdateDomainAsyncWorkflowConfiguratonResponse{},
			expected: &types.UpdateDomainAsyncWorkflowConfiguratonResponse{},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expected, ToAdminUpdateDomainAsyncWorkflowConfiguratonResponse(td.in))
		})
	}
}

func TestAdminUpdateTaskListPartitionConfigRequest(t *testing.T) {
	for _, item := range []*types.UpdateTaskListPartitionConfigRequest{nil, {}, &testdata.AdminUpdateTaskListPartitionConfigRequest} {
		assert.Equal(t, item, ToAdminUpdateTaskListPartitionConfigRequest(FromAdminUpdateTaskListPartitionConfigRequest(item)))
	}
}

func TestAdminUpdateTaskListPartitionConfigResponse(t *testing.T) {
	for _, item := range []*types.UpdateTaskListPartitionConfigResponse{nil, {}} {
		assert.Equal(t, item, ToAdminUpdateTaskListPartitionConfigResponse(FromAdminUpdateTaskListPartitionConfigResponse(item)))
	}
}
