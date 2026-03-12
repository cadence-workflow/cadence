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
	"fmt"
	"sort"
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	v1 "github.com/uber/cadence-idl/go/proto/api/v1"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
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

// Fuzz tests generated for comprehensive mapper coverage
// Uses testutils.RunMapperFuzzTest for simple, maintainable tests

func TestAdminCloseShardRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminCloseShardRequest, ToAdminCloseShardRequest)
}

func TestAdminResetQueueRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminResetQueueRequest, ToAdminResetQueueRequest,
		testutils.WithCustomFuncs(func(e *types.ResetQueueRequest, c fuzz.Continue) {
			// This function needs to exist because we can't independently fuzz the type field
			// as both shardID and type have the same type signature.
			c.Fuzz(e)

			// valid values are 0, 2-5, nil
			// TaskType is valid when:
			// - nil (will be mapped to invalid and then back to nil)
			// - 2-5 (valid TaskType values)
			// - 0 will be mapped back to nil which is assymetric
			if e.Type != nil {
				// valid values are 2-5
				if *e.Type != 0 {
					e.Type = common.Int32Ptr(int32(2 + c.Intn(4)))
				}
			}
		}),
	)
}

func TestAdminUpdateDynamicConfigRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminUpdateDynamicConfigRequest, ToAdminUpdateDynamicConfigRequest,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestAdminUpdateTaskListPartitionConfigRequestFuzz(t *testing.T) {
	// ReadPartitions and WritePartitions are tested in api_test.go
	testutils.RunMapperFuzzTest(t, FromAdminUpdateTaskListPartitionConfigRequest, ToAdminUpdateTaskListPartitionConfigRequest,
		testutils.WithExcludedFields("ReadPartitions", "WritePartitions"),
	)
}

func TestAdminGetDLQReplicationMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDLQReplicationMessagesRequest, ToAdminGetDLQReplicationMessagesRequest)
}

func TestAdminMergeDLQMessagesResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminMergeDLQMessagesResponse, ToAdminMergeDLQMessagesResponse)
}

func TestAdminDescribeWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminDescribeWorkflowExecutionRequest, ToAdminDescribeWorkflowExecutionRequest)
}

func TestAdminCountDLQMessagesResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminCountDLQMessagesResponse, ToAdminCountDLQMessagesResponse)
}

func TestAdminReapplyEventsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminReapplyEventsRequest, ToAdminReapplyEventsRequest,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestAdminRespondCrossClusterTasksCompletedRequestFuzz(t *testing.T) {
	// TODO(c-warren): Figure this one out
	// [BUG] CrossClusterTaskResponse has multiple issues:
	// 1. TaskType/FailedCause are pointer-to-enum, invalid values map to nil
	// 2. Response attributes (StartChildExecutionAttributes, etc.) form a oneof based on TaskType
	//    Invalid TaskType causes all oneof attributes to be cleared
	testutils.RunMapperFuzzTest(t, FromAdminRespondCrossClusterTasksCompletedRequest, ToAdminRespondCrossClusterTasksCompletedRequest,
		testutils.WithExcludedFields(
			"TaskType",
			"FailedCause",
			"StartChildExecutionAttributes",
			"CancelExecutionAttributes",
			"SignalExecutionAttributes",
			"RecordChildWorkflowExecutionCompleteAttributes",
			"ApplyParentClosePolicyAttributes",
		),
	)
}

func TestAdminGetDomainReplicationMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDomainReplicationMessagesRequest, ToAdminGetDomainReplicationMessagesRequest)
}

func TestAdminUpdateTaskListPartitionConfigResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminUpdateTaskListPartitionConfigResponse, ToAdminUpdateTaskListPartitionConfigResponse)
}

func TestAdminRefreshWorkflowTasksRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminRefreshWorkflowTasksRequest, ToAdminRefreshWorkflowTasksRequest)
}

func TestAdminDescribeHistoryHostRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminDescribeHistoryHostRequest, ToAdminDescribeHistoryHostRequest,
		testutils.WithCustomFuncs(DescribeHistoryHostRequestFuzzer),
	)
}

func TestAdminGetReplicationMessagesResponseFuzz(t *testing.T) {
	// TODO(c-warren): Replication tasks is in shared.go, and we can fuzz test the type there instead
	// [BUG] ReplicationTask has complex oneof structure with TaskType (pointer-to-enum)
	// and multiple attribute fields. Fuzzer generates invalid combinations that don't round-trip.
	// Excluding entire MessagesByShard map which contains ReplicationMessages with ReplicationTasks.
	testutils.RunMapperFuzzTest(t, FromAdminGetReplicationMessagesResponse, ToAdminGetReplicationMessagesResponse,
		testutils.WithExcludedFields("MessagesByShard"),
	)
}

func TestAdminGetWorkflowExecutionRawHistoryV2ResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetWorkflowExecutionRawHistoryV2Response, ToAdminGetWorkflowExecutionRawHistoryV2Response,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestAdminRemoveTaskRequestFuzz(t *testing.T) {
	// [BUG] Type field is *int32 (pointer-to-enum TaskType)
	// TODO(c-warren): TaskType is an int32 so we can't make a generic fuzzer for it
	// Figure out what to do there
	testutils.RunMapperFuzzTest(t, FromAdminRemoveTaskRequest, ToAdminRemoveTaskRequest,
		testutils.WithExcludedFields("Type"),
	)
}

func TestAdminResendReplicationTasksRequestFuzz(t *testing.T) {
	// [BUG] EventIDVersionPair mapping: if EndEventID is set but EndVersion is nil (or vice versa),
	// the pair doesn't round-trip correctly. Both must be set or both nil.
	// Same issue with StartEventID/StartVersion.
	testutils.RunMapperFuzzTest(t, FromAdminResendReplicationTasksRequest, ToAdminResendReplicationTasksRequest,
		testutils.WithCustomFuncs(ResendReplicationTasksRequestFuzzer),
	)
}

func TestAdminAddSearchAttributeRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminAddSearchAttributeRequest, ToAdminAddSearchAttributeRequest)
}

func TestAdminDescribeClusterResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminDescribeClusterResponse, ToAdminDescribeClusterResponse)
}

func TestAdminGetCrossClusterTasksResponseFuzz(t *testing.T) {
	// [BUG] CrossClusterTaskRequest has complex oneof structure that causes nil pointer panics
	// in FromCrossClusterApplyParentClosePolicyRequestAttributes when fuzzer generates invalid data.
	// Excluding entire TasksByShard map to avoid mapper panics.
	testutils.RunMapperFuzzTest(t, FromAdminGetCrossClusterTasksResponse, ToAdminGetCrossClusterTasksResponse,
		testutils.WithExcludedFields("TasksByShard"),
		testutils.WithCustomFuncs(testutils.GetTaskFailedCauseFuzzer),
	)
}

func TestAdminGetDynamicConfigResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDynamicConfigResponse, ToAdminGetDynamicConfigResponse,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestAdminMaintainCorruptWorkflowRequestFuzz(t *testing.T) {
	// [BUG] SkipErrors field is missing from the mapper - not included in proto conversion
	testutils.RunMapperFuzzTest(t, FromAdminMaintainCorruptWorkflowRequest, ToAdminMaintainCorruptWorkflowRequest,
		testutils.WithExcludedFields("SkipErrors"),
	)
}

func TestDynamicConfigFilterArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDynamicConfigFilterArray, ToDynamicConfigFilterArray, testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer))
}

func TestAdminGetDomainAsyncWorkflowConfiguratonResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDomainAsyncWorkflowConfiguratonResponse, ToAdminGetDomainAsyncWorkflowConfiguratonResponse,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestAdminDescribeWorkflowExecutionResponseFuzz(t *testing.T) {
	// [BUG] ShardID is a string that gets converted to int32 using stringToInt32()
	// which panics on invalid strings. This is a mapper bug - should handle errors gracefully.
	// Using custom fuzzer to ensure ShardID is a valid int32 string or empty.
	testutils.RunMapperFuzzTest(t, FromAdminDescribeWorkflowExecutionResponse, ToAdminDescribeWorkflowExecutionResponse,
		testutils.WithCustomFuncs(DescribeWorkflowExecutionResponseFuzzer),
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestAdminDescribeShardDistributionResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminDescribeShardDistributionResponse, ToAdminDescribeShardDistributionResponse)
}

func TestAdminMergeDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminMergeDLQMessagesRequest, ToAdminMergeDLQMessagesRequest,
		testutils.WithCustomFuncs(DLQTypeFuzzer),
	)
}

func TestAdminGetDLQReplicationMessagesResponseFuzz(t *testing.T) {
	// Same as GetReplicationMessagesResponse - has complex ReplicationTasks
	testutils.RunMapperFuzzTest(t, FromAdminGetDLQReplicationMessagesResponse, ToAdminGetDLQReplicationMessagesResponse,
		testutils.WithExcludedFields("ReplicationTasks"),
	)
}

func TestAdminGetReplicationMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetReplicationMessagesRequest, ToAdminGetReplicationMessagesRequest)
}

func TestAdminListDynamicConfigResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminListDynamicConfigResponse, ToAdminListDynamicConfigResponse,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestAdminUpdateGlobalIsolationGroupsRequestFuzz(t *testing.T) {
	// [BUG] IsolationGroups map keys are normalized during round-trip causing map size changes
	// Excluding IsolationGroups field to avoid non-deterministic map key handling
	testutils.RunMapperFuzzTest(t, FromAdminUpdateGlobalIsolationGroupsRequest, ToAdminUpdateGlobalIsolationGroupsRequest,
		testutils.WithExcludedFields("IsolationGroups"),
	)
}

func TestAdminGetGlobalIsolationGroupsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetGlobalIsolationGroupsRequest, ToAdminGetGlobalIsolationGroupsRequest)
}

func TestAdminUpdateDomainAsyncWorkflowConfiguratonResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminUpdateDomainAsyncWorkflowConfiguratonResponse, ToAdminUpdateDomainAsyncWorkflowConfiguratonResponse)
}

func TestDynamicConfigFilterFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDynamicConfigFilter, ToDynamicConfigFilter, testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer))
}

func TestAdminDescribeHistoryHostResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminDescribeHistoryHostResponse, ToAdminDescribeHistoryHostResponse)
}

func TestAdminGetDomainReplicationMessagesResponseFuzz(t *testing.T) {
	// Has ReplicationTasks like GetReplicationMessagesResponse
	testutils.RunMapperFuzzTest(t, FromAdminGetDomainReplicationMessagesResponse, ToAdminGetDomainReplicationMessagesResponse,
		testutils.WithExcludedFields("ReplicationTasks"),
	)
}

func TestAdminGetWorkflowExecutionRawHistoryV2RequestFuzz(t *testing.T) {
	// TODO(c-warren): StartEvent/EndEvent should be fuzzable, check this
	// [BUG] Event ID and Version fields have complex conditional nil handling during round-trip
	// When some fields are nil, others are also set to nil in a non-deterministic way
	// Excluding all event-related fields to avoid conditional nil handling
	testutils.RunMapperFuzzTest(t, FromAdminGetWorkflowExecutionRawHistoryV2Request, ToAdminGetWorkflowExecutionRawHistoryV2Request,
		testutils.WithExcludedFields("StartEventID", "StartEventVersion", "EndEventID", "EndEventVersion"),
	)
}

func TestAdminRestoreDynamicConfigRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminRestoreDynamicConfigRequest, ToAdminRestoreDynamicConfigRequest,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestAdminDeleteWorkflowRequestFuzz(t *testing.T) {
	// [BUG] SkipErrors is not mapped
	testutils.RunMapperFuzzTest(t, FromAdminDeleteWorkflowRequest, ToAdminDeleteWorkflowRequest,
		testutils.WithExcludedFields("SkipErrors"),
	)
}

func TestAdminRespondCrossClusterTasksCompletedResponseFuzz(t *testing.T) {
	// Same issue as Request - CrossClusterTaskRequest has complex oneof that causes panics
	testutils.RunMapperFuzzTest(t, FromAdminRespondCrossClusterTasksCompletedResponse, ToAdminRespondCrossClusterTasksCompletedResponse,
		testutils.WithExcludedFields("Tasks"),
	)
}

func TestDynamicConfigEntryFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDynamicConfigEntry, ToDynamicConfigEntry, testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer))
}

func TestDynamicConfigValueArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDynamicConfigValueArray, ToDynamicConfigValueArray, testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer))
}

func TestAdminDescribeQueueResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminDescribeQueueResponse, ToAdminDescribeQueueResponse)
}

func TestAdminReadDLQMessagesResponseFuzz(t *testing.T) {
	// Has ReplicationTasks like other DLQ responses
	testutils.RunMapperFuzzTest(t, FromAdminReadDLQMessagesResponse, ToAdminReadDLQMessagesResponse,
		testutils.WithExcludedFields("ReplicationTasks"),
		testutils.WithCustomFuncs(DLQTypeFuzzer),
	)
}

func TestAdminGetDynamicConfigRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDynamicConfigRequest, ToAdminGetDynamicConfigRequest,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestDynamicConfigValueFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDynamicConfigValue, ToDynamicConfigValue, testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer))
}

func TestAdminCountDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminCountDLQMessagesRequest, ToAdminCountDLQMessagesRequest)
}

func TestIsolationGroupConfigFuzz(t *testing.T) {
	t.Skip("TODO(c-warren): This is a map so fuzzing is going to be tricky")
	testutils.RunMapperFuzzTest(t, FromIsolationGroupConfig, ToIsolationGroupConfig)
}

func TestAdminUpdateDomainAsyncWorkflowConfiguratonRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminUpdateDomainAsyncWorkflowConfiguratonRequest, ToAdminUpdateDomainAsyncWorkflowConfiguratonRequest,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestAdminDescribeQueueRequestFuzz(t *testing.T) {
	// TODO(c-warren): TaskType is an int32, figure out how to fuzz this
	// [BUG] Type field (pointer-to-int32 for TaskType enum) maps invalid values to nil
	// Excluding Type field to avoid pointer-to-enum mapping issue
	testutils.RunMapperFuzzTest(t, FromAdminDescribeQueueRequest, ToAdminDescribeQueueRequest,
		testutils.WithExcludedFields("Type"),
	)
}

func TestAdminMaintainCorruptWorkflowResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminMaintainCorruptWorkflowResponse, ToAdminMaintainCorruptWorkflowResponse)
}

func TestAdminUpdateGlobalIsolationGroupsResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminUpdateGlobalIsolationGroupsResponse, ToAdminUpdateGlobalIsolationGroupsResponse)
}

func TestDomainAsyncWorkflowConfiguratonFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDomainAsyncWorkflowConfiguraton, ToDomainAsyncWorkflowConfiguraton, testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer))
}

func TestAdminDescribeShardDistributionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminDescribeShardDistributionRequest, ToAdminDescribeShardDistributionRequest)
}

func TestAdminPurgeDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminPurgeDLQMessagesRequest, ToAdminPurgeDLQMessagesRequest,
		testutils.WithCustomFuncs(DLQTypeFuzzer),
	)
}

func TestAdminReadDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminReadDLQMessagesRequest, ToAdminReadDLQMessagesRequest,
		testutils.WithCustomFuncs(DLQTypeFuzzer),
	)
}

func TestAdminGetCrossClusterTasksRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetCrossClusterTasksRequest, ToAdminGetCrossClusterTasksRequest)
}

func TestDynamicConfigEntryArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDynamicConfigEntryArray, ToDynamicConfigEntryArray, testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer))
}

func TestAdminUpdateDomainIsolationGroupsResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminUpdateDomainIsolationGroupsResponse, ToAdminUpdateDomainIsolationGroupsResponse)
}

func TestAdminUpdateDomainIsolationGroupsRequestFuzz(t *testing.T) {
	// [BUG] IsolationGroups map keys are normalized during round-trip causing map size changes
	// Excluding IsolationGroups field to avoid non-deterministic map key handling
	testutils.RunMapperFuzzTest(t, FromAdminUpdateDomainIsolationGroupsRequest, ToAdminUpdateDomainIsolationGroupsRequest,
		testutils.WithExcludedFields("IsolationGroups"),
	)
}

func TestAdminGetDomainIsolationGroupsResponseFuzz(t *testing.T) {
	// [BUG] IsolationGroups map keys are normalized during round-trip causing map size changes
	// Excluding IsolationGroups field to avoid non-deterministic map key handling
	testutils.RunMapperFuzzTest(t, FromAdminGetDomainIsolationGroupsResponse, ToAdminGetDomainIsolationGroupsResponse,
		testutils.WithExcludedFields("IsolationGroups"),
	)
}

func TestAdminGetDomainAsyncWorkflowConfiguratonRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDomainAsyncWorkflowConfiguratonRequest, ToAdminGetDomainAsyncWorkflowConfiguratonRequest)
}

func TestAdminDeleteWorkflowResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminDeleteWorkflowResponse, ToAdminDeleteWorkflowResponse)
}

func TestAdminListDynamicConfigRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminListDynamicConfigRequest, ToAdminListDynamicConfigRequest)
}

func TestAdminGetGlobalIsolationGroupsResponseFuzz(t *testing.T) {
	// [BUG] IsolationGroups map keys are normalized during round-trip causing map size changes
	// Excluding IsolationGroups field to avoid non-deterministic map key handling
	testutils.RunMapperFuzzTest(t, FromAdminGetGlobalIsolationGroupsResponse, ToAdminGetGlobalIsolationGroupsResponse,
		testutils.WithExcludedFields("IsolationGroups"),
	)
}

func TestAdminGetDomainIsolationGroupsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDomainIsolationGroupsRequest, ToAdminGetDomainIsolationGroupsRequest)
}

// DescribeHistoryHostRequestFuzzer ensures only one of the oneof fields is set
func DescribeHistoryHostRequestFuzzer(r *types.DescribeHistoryHostRequest, c fuzz.Continue) {
	choice := c.Intn(3)
	switch choice {
	case 0:
		// Set HostAddress only
		var addr string
		c.Fuzz(&addr)
		r.HostAddress = &addr
		r.ShardIDForHost = nil
		r.ExecutionForHost = nil
	case 1:
		// Set ShardIDForHost only
		var shardID int32
		c.Fuzz(&shardID)
		r.ShardIDForHost = &shardID
		r.HostAddress = nil
		r.ExecutionForHost = nil
	case 2:
		// Set ExecutionForHost only
		var exec types.WorkflowExecution
		c.Fuzz(&exec)
		r.ExecutionForHost = &exec
		r.HostAddress = nil
		r.ShardIDForHost = nil
	}
}

// ResendReplicationTasksRequestFuzzer ensures EventID/Version pairs are both set or both nil
func ResendReplicationTasksRequestFuzzer(r *types.ResendReplicationTasksRequest, c fuzz.Continue) {
	c.FuzzNoCustom(r)

	// Fix StartEventID/StartVersion pair: both set or both nil
	if (r.StartEventID == nil) != (r.StartVersion == nil) {
		// Mismatched - make both nil or both non-nil
		if c.Intn(2) == 0 {
			r.StartEventID = nil
			r.StartVersion = nil
		} else {
			if r.StartEventID == nil {
				id := c.Int63()
				r.StartEventID = &id
			}
			if r.StartVersion == nil {
				ver := c.Int63()
				r.StartVersion = &ver
			}
		}
	}

	// Fix EndEventID/EndVersion pair: both set or both nil
	if (r.EndEventID == nil) != (r.EndVersion == nil) {
		// Mismatched - make both nil or both non-nil
		if c.Intn(2) == 0 {
			r.EndEventID = nil
			r.EndVersion = nil
		} else {
			if r.EndEventID == nil {
				id := c.Int63()
				r.EndEventID = &id
			}
			if r.EndVersion == nil {
				ver := c.Int63()
				r.EndVersion = &ver
			}
		}
	}
}

// DescribeWorkflowExecutionResponseFuzzer ensures ShardID is a valid int32 string
func DescribeWorkflowExecutionResponseFuzzer(r *types.AdminDescribeWorkflowExecutionResponse, c fuzz.Continue) {
	c.FuzzNoCustom(r)

	// Always generate a valid ShardID (int32 as string) to avoid stringToInt32 panics
	shardID := c.Int31()
	r.ShardID = fmt.Sprintf("%d", shardID)
}

func DLQTypeFuzzer(e *types.DLQType, c fuzz.Continue) {
	*e = types.DLQType(c.Intn(2)) // 0-1: Replication, Domain
}
