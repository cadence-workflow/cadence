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

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
	"github.com/uber/cadence/common/types/testdata"
)

func TestDLQType(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.DLQType
	}{
		{
			desc:  "non-nil input test",
			input: types.DLQTypeReplication.Ptr(),
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromDLQType(tc.input)
			roundTripObj := ToDLQType(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestDomainOperation(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.DomainOperation
	}{
		{
			desc:  "non-nil input test",
			input: types.DomainOperationCreate.Ptr(),
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromDomainOperation(tc.input)
			roundTripObj := ToDomainOperation(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestDomainTaskAttributes(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.DomainTaskAttributes
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.DomainTaskAttributes,
		},
		{
			desc:  "empty input test",
			input: &types.DomainTaskAttributes{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromDomainTaskAttributes(tc.input)
			roundTripObj := ToDomainTaskAttributes(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestFailoverMarkerAttributes(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.FailoverMarkerAttributes
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.FailoverMarkerAttributes,
		},
		{
			desc:  "empty input test",
			input: &types.FailoverMarkerAttributes{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromFailoverMarkerAttributes(tc.input)
			roundTripObj := ToFailoverMarkerAttributes(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestFailoverMarkers(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.FailoverMarkers
	}{
		{
			desc:  "non-nil input test",
			input: &types.FailoverMarkers{FailoverMarkers: testdata.FailoverMarkerAttributesArray},
		},
		{
			desc:  "empty input test",
			input: &types.FailoverMarkers{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromFailoverMarkers(tc.input)
			roundTripObj := ToFailoverMarkers(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminGetDLQReplicationMessagesRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.GetDLQReplicationMessagesRequest
	}{
		{
			desc: "non-nil input test",
			input: &types.GetDLQReplicationMessagesRequest{
				TaskInfos: testdata.ReplicationTaskInfoArray,
			},
		},
		{
			desc:  "empty input test",
			input: &types.GetDLQReplicationMessagesRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminGetDLQReplicationMessagesRequest(tc.input)
			roundTripObj := ToAdminGetDLQReplicationMessagesRequest(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminGetDLQReplicationMessagesResponse(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.GetDLQReplicationMessagesResponse
	}{
		{
			desc: "non-nil input test",
			input: &types.GetDLQReplicationMessagesResponse{
				ReplicationTasks: testdata.ReplicationTaskArray,
			},
		},
		{
			desc:  "empty input test",
			input: &types.GetDLQReplicationMessagesResponse{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminGetDLQReplicationMessagesResponse(tc.input)
			roundTripObj := ToAdminGetDLQReplicationMessagesResponse(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminGetDomainReplicationMessageRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.GetDomainReplicationMessagesRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.GetDomainReplicationMessagesRequest,
		},
		{
			desc:  "empty input test",
			input: &types.GetDomainReplicationMessagesRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminGetDomainReplicationMessagesRequest(tc.input)
			roundTripObj := ToAdminGetDomainReplicationMessagesRequest(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminGetDomainReplicationMessageResponse(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.GetDomainReplicationMessagesResponse
	}{
		{
			desc: "non-nil input test",
			input: &types.GetDomainReplicationMessagesResponse{
				Messages: &testdata.ReplicationMessages,
			},
		},
		{
			desc:  "empty input test",
			input: &types.GetDomainReplicationMessagesResponse{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminGetDomainReplicationMessagesResponse(tc.input)
			roundTripObj := ToAdminGetDomainReplicationMessagesResponse(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminGetReplicationMessageRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.GetReplicationMessagesRequest
	}{
		{
			desc: "non-nil input test",
			input: &types.GetReplicationMessagesRequest{
				Tokens:      testdata.ReplicationTokenArray,
				ClusterName: testdata.ClusterName1,
			},
		},
		{
			desc:  "empty input test",
			input: &types.GetReplicationMessagesRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminGetReplicationMessagesRequest(tc.input)
			roundTripObj := ToAdminGetReplicationMessagesRequest(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminGetReplicationMessageResponse(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.GetReplicationMessagesResponse
	}{
		{
			desc: "non-nil input test",
			input: &types.GetReplicationMessagesResponse{
				MessagesByShard: testdata.ReplicationMessagesMap,
			},
		},
		{
			desc:  "empty input test",
			input: &types.GetReplicationMessagesResponse{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminGetReplicationMessagesResponse(tc.input)
			roundTripObj := ToAdminGetReplicationMessagesResponse(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestHistoryTaskV2Attributes(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.HistoryTaskV2Attributes
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.HistoryTaskV2Attributes,
		},
		{
			desc:  "empty input test",
			input: &types.HistoryTaskV2Attributes{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromHistoryTaskV2Attributes(tc.input)
			roundTripObj := ToHistoryTaskV2Attributes(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminMergeDLQMessagesRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.MergeDLQMessagesRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.MergeDLQMessagesRequest,
		},
		{
			desc:  "empty input test",
			input: &types.MergeDLQMessagesRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminMergeDLQMessagesRequest(tc.input)
			roundTripObj := ToAdminMergeDLQMessagesRequest(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminMergeDLQMessagesResponse(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.MergeDLQMessagesResponse
	}{
		{
			desc: "non-nil input test",
			input: &types.MergeDLQMessagesResponse{
				NextPageToken: testdata.Token1,
			},
		},
		{
			desc:  "empty input test",
			input: &types.MergeDLQMessagesResponse{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminMergeDLQMessagesResponse(tc.input)
			roundTripObj := ToAdminMergeDLQMessagesResponse(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminPurgeDLQMessagesRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.PurgeDLQMessagesRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.PurgeDLQMessagesRequest,
		},
		{
			desc:  "empty input test",
			input: &types.PurgeDLQMessagesRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminPurgeDLQMessagesRequest(tc.input)
			roundTripObj := ToAdminPurgeDLQMessagesRequest(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminReadDLQMessagesRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.ReadDLQMessagesRequest
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.ReadDLQMessagesRequest,
		},
		{
			desc:  "empty input test",
			input: &types.ReadDLQMessagesRequest{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminReadDLQMessagesRequest(tc.input)
			roundTripObj := ToAdminReadDLQMessagesRequest(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestAdminReadDLQMessagesResponse(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.ReadDLQMessagesResponse
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.ReadDLQMessagesResponse,
		},
		{
			desc:  "empty input test",
			input: &types.ReadDLQMessagesResponse{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromAdminReadDLQMessagesResponse(tc.input)
			roundTripObj := ToAdminReadDLQMessagesResponse(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestReplicationMessages(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.ReplicationMessages
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.ReplicationMessages,
		},
		{
			desc:  "empty input test",
			input: &types.ReplicationMessages{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromReplicationMessages(tc.input)
			roundTripObj := ToReplicationMessages(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestReplicationTask(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.ReplicationTask
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.ReplicationTask_Domain,
		},
		{
			desc:  "empty input test",
			input: &types.ReplicationTask{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromReplicationTask(tc.input)
			roundTripObj := ToReplicationTask(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestReplicationTaskInfo(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.ReplicationTaskInfo
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.ReplicationTaskInfo,
		},
		{
			desc:  "empty input test",
			input: &types.ReplicationTaskInfo{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromReplicationTaskInfo(tc.input)
			roundTripObj := ToReplicationTaskInfo(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestReplicationTaskType(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.ReplicationTaskType
	}{
		{
			desc:  "non-nil input test",
			input: types.ReplicationTaskTypeDomain.Ptr(),
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromReplicationTaskType(tc.input)
			roundTripObj := ToReplicationTaskType(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestReplicationToken(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.ReplicationToken
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.ReplicationToken,
		},
		{
			desc:  "empty input test",
			input: &types.ReplicationToken{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromReplicationToken(tc.input)
			roundTripObj := ToReplicationToken(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestSyncActivityTaskAttributes(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.SyncActivityTaskAttributes
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.SyncActivityTaskAttributes,
		},
		{
			desc:  "empty input test",
			input: &types.SyncActivityTaskAttributes{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromSyncActivityTaskAttributes(tc.input)
			roundTripObj := ToSyncActivityTaskAttributes(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestSyncShardStatus(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.SyncShardStatus
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.SyncShardStatus,
		},
		{
			desc:  "empty input test",
			input: &types.SyncShardStatus{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromSyncShardStatus(tc.input)
			roundTripObj := ToSyncShardStatus(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestSyncShardStatusTaskAttributes(t *testing.T) {
	testCases := []struct {
		desc  string
		input *types.SyncShardStatusTaskAttributes
	}{
		{
			desc:  "non-nil input test",
			input: &testdata.SyncShardStatusTaskAttributes,
		},
		{
			desc:  "empty input test",
			input: &types.SyncShardStatusTaskAttributes{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromSyncShardStatusTaskAttributes(tc.input)
			roundTripObj := ToSyncShardStatusTaskAttributes(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestFailoverMarkerAttributesArray(t *testing.T) {
	testCases := []struct {
		desc  string
		input []*types.FailoverMarkerAttributes
	}{
		{
			desc:  "non-nil input test",
			input: testdata.FailoverMarkerAttributesArray,
		},
		{
			desc:  "empty input test",
			input: []*types.FailoverMarkerAttributes{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromFailoverMarkerAttributesArray(tc.input)
			roundTripObj := ToFailoverMarkerAttributesArray(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestReplicationTaskInfoArray(t *testing.T) {
	testCases := []struct {
		desc  string
		input []*types.ReplicationTaskInfo
	}{
		{
			desc:  "non-nil input test",
			input: testdata.ReplicationTaskInfoArray,
		},
		{
			desc:  "empty input test",
			input: []*types.ReplicationTaskInfo{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromReplicationTaskInfoArray(tc.input)
			roundTripObj := ToReplicationTaskInfoArray(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestReplicationTaskArray(t *testing.T) {
	testCases := []struct {
		desc  string
		input []*types.ReplicationTask
	}{
		{
			desc:  "non-nil input test",
			input: testdata.ReplicationTaskArray,
		},
		{
			desc:  "empty input test",
			input: []*types.ReplicationTask{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromReplicationTaskArray(tc.input)
			roundTripObj := ToReplicationTaskArray(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestReplicationTokenArray(t *testing.T) {
	testCases := []struct {
		desc  string
		input []*types.ReplicationToken
	}{
		{
			desc:  "non-nil input test",
			input: testdata.ReplicationTokenArray,
		},
		{
			desc:  "empty input test",
			input: []*types.ReplicationToken{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromReplicationTokenArray(tc.input)
			roundTripObj := ToReplicationTokenArray(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

func TestReplicationMessagesMap(t *testing.T) {
	testCases := []struct {
		desc  string
		input map[int32]*types.ReplicationMessages
	}{
		{
			desc:  "non-nil input test",
			input: testdata.ReplicationMessagesMap,
		},
		{
			desc:  "empty input test",
			input: map[int32]*types.ReplicationMessages{},
		},
		{
			desc:  "nil input test",
			input: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			thriftObj := FromReplicationMessagesMap(tc.input)
			roundTripObj := ToReplicationMessagesMap(thriftObj)
			assert.Equal(t, tc.input, roundTripObj)
		})
	}
}

// replicatorFuzzers constrains the values reachable from the replicator mappers
// that their From* functions panic on or silently drop.
func replicatorFuzzers() testutils.FuzzOption {
	return testutils.WithCustomFuncs(
		testutils.ArchivalStatusFuzzer,
		testutils.DomainStatusFuzzer,
		testutils.DomainOperationFuzzer,
		testutils.DLQTypeFuzzer,
		testutils.ReplicationTaskTypeFuzzer,
		testutils.EncodingTypeFuzzer,
		testutils.FailureCategoryFuzzer,
		testutils.IsolationGroupConfigurationFuzzer,
	)
}

func TestDLQTypeFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDLQType, ToDLQType,
		testutils.WithCustomFuncs(testutils.DLQTypeFuzzer))
}

func TestDomainOperationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDomainOperation, ToDomainOperation,
		testutils.WithCustomFuncs(testutils.DomainOperationFuzzer))
}

func TestDomainTaskAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDomainTaskAttributes, ToDomainTaskAttributes,
		replicatorFuzzers())
}

func TestFailoverMarkerAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromFailoverMarkerAttributes, ToFailoverMarkerAttributes,
		replicatorFuzzers())
}

func TestFailoverMarkersFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromFailoverMarkers, ToFailoverMarkers,
		replicatorFuzzers())
}

func TestAdminGetDLQReplicationMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDLQReplicationMessagesRequest, ToAdminGetDLQReplicationMessagesRequest,
		replicatorFuzzers())
}

func TestAdminGetDLQReplicationMessagesResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDLQReplicationMessagesResponse, ToAdminGetDLQReplicationMessagesResponse,
		replicatorFuzzers())
}

func TestAdminGetDomainReplicationMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDomainReplicationMessagesRequest, ToAdminGetDomainReplicationMessagesRequest,
		replicatorFuzzers())
}

func TestAdminGetDomainReplicationMessagesResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetDomainReplicationMessagesResponse, ToAdminGetDomainReplicationMessagesResponse,
		replicatorFuzzers())
}

func TestAdminGetReplicationMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetReplicationMessagesRequest, ToAdminGetReplicationMessagesRequest,
		replicatorFuzzers())
}

func TestAdminGetReplicationMessagesResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminGetReplicationMessagesResponse, ToAdminGetReplicationMessagesResponse,
		replicatorFuzzers())
}

func TestHistoryTaskV2AttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryTaskV2Attributes, ToHistoryTaskV2Attributes,
		replicatorFuzzers())
}

func TestAdminMergeDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminMergeDLQMessagesRequest, ToAdminMergeDLQMessagesRequest,
		replicatorFuzzers())
}

func TestAdminMergeDLQMessagesResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminMergeDLQMessagesResponse, ToAdminMergeDLQMessagesResponse,
		replicatorFuzzers())
}

func TestAdminPurgeDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminPurgeDLQMessagesRequest, ToAdminPurgeDLQMessagesRequest,
		replicatorFuzzers())
}

func TestAdminReadDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminReadDLQMessagesRequest, ToAdminReadDLQMessagesRequest,
		replicatorFuzzers())
}

func TestAdminReadDLQMessagesResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAdminReadDLQMessagesResponse, ToAdminReadDLQMessagesResponse,
		replicatorFuzzers())
}

func TestReplicationMessagesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationMessages, ToReplicationMessages,
		replicatorFuzzers())
}

func TestReplicationTaskFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationTask, ToReplicationTask,
		replicatorFuzzers())
}

func TestReplicationTaskInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationTaskInfo, ToReplicationTaskInfo,
		replicatorFuzzers())
}

func TestReplicationTaskTypeFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationTaskType, ToReplicationTaskType,
		testutils.WithCustomFuncs(testutils.ReplicationTaskTypeFuzzer))
}

func TestReplicationTokenFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationToken, ToReplicationToken,
		replicatorFuzzers())
}

func TestSyncActivityTaskAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSyncActivityTaskAttributes, ToSyncActivityTaskAttributes,
		replicatorFuzzers())
}

func TestSyncShardStatusFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSyncShardStatus, ToSyncShardStatus,
		replicatorFuzzers())
}

func TestSyncShardStatusTaskAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSyncShardStatusTaskAttributes, ToSyncShardStatusTaskAttributes,
		replicatorFuzzers())
}

func TestFailoverMarkerAttributesArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromFailoverMarkerAttributesArray, ToFailoverMarkerAttributesArray,
		replicatorFuzzers())
}

func TestReplicationTaskInfoArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationTaskInfoArray, ToReplicationTaskInfoArray,
		replicatorFuzzers())
}

func TestReplicationTaskArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationTaskArray, ToReplicationTaskArray,
		replicatorFuzzers())
}

func TestReplicationTokenArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationTokenArray, ToReplicationTokenArray,
		replicatorFuzzers())
}

func TestReplicationMessagesMapFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationMessagesMap, ToReplicationMessagesMap,
		replicatorFuzzers())
}
