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

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsolationGroupConfiguration_ToPartitionList(t *testing.T) {
	tests := map[string]struct {
		in          *IsolationGroupConfiguration
		expectedOut []IsolationGroupPartition
	}{
		"valid value": {
			in: &IsolationGroupConfiguration{
				"zone-1": {
					Name:  "zone-1",
					State: IsolationGroupStateDrained,
				},
				"zone-2": {
					Name:  "zone-2",
					State: IsolationGroupStateHealthy,
				},
				"zone-3": {
					Name:  "zone-3",
					State: IsolationGroupStateDrained,
				},
			},
			expectedOut: []IsolationGroupPartition{
				{
					Name:  "zone-1",
					State: IsolationGroupStateDrained,
				},
				{
					Name:  "zone-2",
					State: IsolationGroupStateHealthy,
				},
				{
					Name:  "zone-3",
					State: IsolationGroupStateDrained,
				},
			},
		},
	}
	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expectedOut, td.in.ToPartitionList())
		})
	}
}

func TestIsolationGroupConfigurationDeepCopy(t *testing.T) {
	tests := []struct {
		name  string
		input IsolationGroupConfiguration
	}{
		{
			name:  "nil",
			input: nil,
		},
		{
			name:  "empty",
			input: IsolationGroupConfiguration{},
		},
		{
			name: "single group",
			input: IsolationGroupConfiguration{
				"zone-1": {
					Name:  "zone-1",
					State: IsolationGroupStateDrained,
				},
			},
		},
		{
			name: "multiple groups",
			input: IsolationGroupConfiguration{
				"zone-1": {
					Name:  "zone-1",
					State: IsolationGroupStateDrained,
				},
				"zone-2": {
					Name:  "zone-2",
					State: IsolationGroupStateHealthy,
				},
				"zone-3": {
					Name:  "zone-3",
					State: IsolationGroupStateDrained,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.input.DeepCopy()
			assert.Equal(t, tc.input, got)

			if tc.input == nil {
				return
			}

			tc.input["new"] = IsolationGroupPartition{Name: "new", State: IsolationGroupStateHealthy}
			assert.NotEqual(t, tc.input, got)
		})
	}
}

func TestAsyncWorkflowConfigurationDeepCopy(t *testing.T) {
	tests := []struct {
		name  string
		input AsyncWorkflowConfiguration
	}{
		{
			name:  "empty",
			input: AsyncWorkflowConfiguration{},
		},
		{
			name: "predefined queue",
			input: AsyncWorkflowConfiguration{
				Enabled:             true,
				PredefinedQueueName: "test-async-wf-queue",
			},
		},
		{
			name: "custom queue",
			input: AsyncWorkflowConfiguration{
				Enabled:   true,
				QueueType: "custom",
				QueueConfig: &DataBlob{
					EncodingType: EncodingTypeThriftRW.Ptr(),
					Data:         []byte("test-async-wf-queue"),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.input.DeepCopy()
			assert.Equal(t, tc.input, got)
			if tc.input.QueueConfig != nil {
				// assert that queue configs look the same but underlying slice is different
				assert.Equal(t, tc.input.QueueConfig, got.QueueConfig)
				if tc.input.QueueConfig.Data != nil && identicalByteArray(tc.input.QueueConfig.Data, got.QueueConfig.Data) {
					t.Error("expected DeepCopy to return a new QueueConfig.Data")
				}
			}
		})
	}
}

func TestAddSearchAttributeRequest_GetSearchAttribute(t *testing.T) {
	tests := []struct {
		name    string
		request *AddSearchAttributeRequest
		want    map[string]IndexedValueType
	}{
		{
			name:    "Nil request",
			request: nil,
			want:    nil,
		},
		{
			name: "Nil SearchAttribute",
			request: &AddSearchAttributeRequest{
				SearchAttribute: nil,
			},
			want: nil,
		},
		{
			name: "With SearchAttribute",
			request: &AddSearchAttributeRequest{
				SearchAttribute: map[string]IndexedValueType{
					"attr1": 1,
					"attr2": 2,
				},
			},
			want: map[string]IndexedValueType{
				"attr1": 1,
				"attr2": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetSearchAttribute()
			assert.Equal(t, tt.want, got, "GetSearchAttribute() result mismatch")
		})
	}
}

func TestAdminDescribeWorkflowExecutionRequest_GetDomain(t *testing.T) {
	tests := []struct {
		name    string
		request *AdminDescribeWorkflowExecutionRequest
		want    string
	}{
		{
			name:    "Nil request",
			request: nil,
			want:    "",
		},
		{
			name:    "Empty Domain",
			request: &AdminDescribeWorkflowExecutionRequest{},
			want:    "",
		},
		{
			name: "With Domain",
			request: &AdminDescribeWorkflowExecutionRequest{
				Domain: "test-domain",
			},
			want: "test-domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetDomain()
			assert.Equal(t, tt.want, got, "GetDomain() result mismatch")
		})
	}
}

func TestAdminDescribeWorkflowExecutionResponse_GetShardID(t *testing.T) {
	tests := []struct {
		name     string
		response *AdminDescribeWorkflowExecutionResponse
		want     string
	}{
		{
			name:     "Nil response",
			response: nil,
			want:     "",
		},
		{
			name:     "Empty ShardID",
			response: &AdminDescribeWorkflowExecutionResponse{},
			want:     "",
		},
		{
			name: "With ShardID",
			response: &AdminDescribeWorkflowExecutionResponse{
				ShardID: "shard-123",
			},
			want: "shard-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.GetShardID()
			assert.Equal(t, tt.want, got, "GetShardID() result mismatch")
		})
	}
}

func TestAdminDescribeWorkflowExecutionResponse_GetMutableStateInDatabase(t *testing.T) {
	tests := []struct {
		name     string
		response *AdminDescribeWorkflowExecutionResponse
		want     string
	}{
		{
			name:     "Nil response",
			response: nil,
			want:     "",
		},
		{
			name:     "Empty MutableStateInDatabase",
			response: &AdminDescribeWorkflowExecutionResponse{},
			want:     "",
		},
		{
			name: "With MutableStateInDatabase",
			response: &AdminDescribeWorkflowExecutionResponse{
				MutableStateInDatabase: "state-data",
			},
			want: "state-data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.GetMutableStateInDatabase()
			assert.Equal(t, tt.want, got, "GetMutableStateInDatabase() result mismatch")
		})
	}
}

func TestGetWorkflowExecutionRawHistoryV2Request_GetDomain(t *testing.T) {
	tests := []struct {
		name    string
		request *GetWorkflowExecutionRawHistoryV2Request
		want    string
	}{
		{
			name:    "Nil request",
			request: nil,
			want:    "",
		},
		{
			name:    "Empty Domain",
			request: &GetWorkflowExecutionRawHistoryV2Request{},
			want:    "",
		},
		{
			name: "With Domain",
			request: &GetWorkflowExecutionRawHistoryV2Request{
				Domain: "test-domain",
			},
			want: "test-domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetDomain()
			assert.Equal(t, tt.want, got, "GetDomain() result mismatch")
		})
	}
}

func TestGetWorkflowExecutionRawHistoryV2Request_GetStartEventID(t *testing.T) {
	tests := []struct {
		name    string
		request *GetWorkflowExecutionRawHistoryV2Request
		want    int64
	}{
		{
			name:    "Nil request",
			request: nil,
			want:    0,
		},
		{
			name:    "Nil StartEventID",
			request: &GetWorkflowExecutionRawHistoryV2Request{},
			want:    0,
		},
		{
			name: "With StartEventID",
			request: &GetWorkflowExecutionRawHistoryV2Request{
				StartEventID: ptrInt64(100),
			},
			want: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetStartEventID()
			assert.Equal(t, tt.want, got, "GetStartEventID() result mismatch")
		})
	}
}

func TestGetWorkflowExecutionRawHistoryV2Request_GetMaximumPageSize(t *testing.T) {
	tests := []struct {
		name    string
		request *GetWorkflowExecutionRawHistoryV2Request
		want    int32
	}{
		{
			name:    "Nil request",
			request: nil,
			want:    0,
		},
		{
			name:    "Default MaximumPageSize",
			request: &GetWorkflowExecutionRawHistoryV2Request{},
			want:    0,
		},
		{
			name: "With MaximumPageSize",
			request: &GetWorkflowExecutionRawHistoryV2Request{
				MaximumPageSize: 100,
			},
			want: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetMaximumPageSize()
			assert.Equal(t, tt.want, got, "GetMaximumPageSize() result mismatch")
		})
	}
}

func TestGetWorkflowExecutionRawHistoryV2Request_GetStartEventVersion(t *testing.T) {
	tests := []struct {
		name    string
		request *GetWorkflowExecutionRawHistoryV2Request
		want    int64
	}{
		{
			name:    "Nil request",
			request: nil,
			want:    0,
		},
		{
			name:    "Nil StartEventVersion",
			request: &GetWorkflowExecutionRawHistoryV2Request{},
			want:    0,
		},
		{
			name: "With StartEventVersion",
			request: &GetWorkflowExecutionRawHistoryV2Request{
				StartEventVersion: ptrInt64(123),
			},
			want: 123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetStartEventVersion()
			assert.Equal(t, tt.want, got, "GetStartEventVersion() result mismatch")
		})
	}
}

func TestGetWorkflowExecutionRawHistoryV2Request_GetEndEventID(t *testing.T) {
	tests := []struct {
		name    string
		request *GetWorkflowExecutionRawHistoryV2Request
		want    int64
	}{
		{
			name:    "Nil request",
			request: nil,
			want:    0,
		},
		{
			name:    "Nil EndEventID",
			request: &GetWorkflowExecutionRawHistoryV2Request{},
			want:    0,
		},
		{
			name: "With EndEventID",
			request: &GetWorkflowExecutionRawHistoryV2Request{
				EndEventID: ptrInt64(200),
			},
			want: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetEndEventID()
			assert.Equal(t, tt.want, got, "GetEndEventID() result mismatch")
		})
	}
}

func TestGetWorkflowExecutionRawHistoryV2Request_GetEndEventVersion(t *testing.T) {
	tests := []struct {
		name    string
		request *GetWorkflowExecutionRawHistoryV2Request
		want    int64
	}{
		{
			name:    "Nil request",
			request: nil,
			want:    0,
		},
		{
			name:    "Nil EndEventVersion",
			request: &GetWorkflowExecutionRawHistoryV2Request{},
			want:    0,
		},
		{
			name: "With EndEventVersion",
			request: &GetWorkflowExecutionRawHistoryV2Request{
				EndEventVersion: ptrInt64(456),
			},
			want: 456,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetEndEventVersion()
			assert.Equal(t, tt.want, got, "GetEndEventVersion() result mismatch")
		})
	}
}
func TestGetWorkflowExecutionRawHistoryV2Response_GetHistoryBatches(t *testing.T) {
	tests := []struct {
		name     string
		response *GetWorkflowExecutionRawHistoryV2Response
		want     []*DataBlob
	}{
		{
			name:     "Nil response",
			response: nil,
			want:     nil,
		},
		{
			name:     "Empty HistoryBatches",
			response: &GetWorkflowExecutionRawHistoryV2Response{},
			want:     nil,
		},
		{
			name: "With HistoryBatches",
			response: &GetWorkflowExecutionRawHistoryV2Response{
				HistoryBatches: []*DataBlob{
					{
						EncodingType: EncodingTypeJSON.Ptr(),
						Data:         []byte("data1"),
					},
					{
						EncodingType: EncodingTypeThriftRW.Ptr(),
						Data:         []byte("data2"),
					},
				},
			},
			want: []*DataBlob{
				{
					EncodingType: EncodingTypeJSON.Ptr(),
					Data:         []byte("data1"),
				},
				{
					EncodingType: EncodingTypeThriftRW.Ptr(),
					Data:         []byte("data2"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.GetHistoryBatches()
			assert.Equal(t, tt.want, got, "GetHistoryBatches() result mismatch")
		})
	}
}

func TestGetWorkflowExecutionRawHistoryV2Response_GetVersionHistory(t *testing.T) {
	tests := []struct {
		name     string
		response *GetWorkflowExecutionRawHistoryV2Response
		want     *VersionHistory
	}{
		{
			name:     "Nil response",
			response: nil,
			want:     nil,
		},
		{
			name:     "Empty VersionHistory",
			response: &GetWorkflowExecutionRawHistoryV2Response{},
			want:     nil,
		},
		{
			name: "With VersionHistory",
			response: &GetWorkflowExecutionRawHistoryV2Response{
				VersionHistory: &VersionHistory{
					Items: []*VersionHistoryItem{
						{
							EventID: 1,
							Version: 1,
						},
						{
							EventID: 2,
							Version: 2,
						},
					},
				},
			},
			want: &VersionHistory{
				Items: []*VersionHistoryItem{
					{
						EventID: 1,
						Version: 1,
					},
					{
						EventID: 2,
						Version: 2,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.GetVersionHistory()
			assert.Equal(t, tt.want, got, "GetVersionHistory() result mismatch")
		})
	}
}

func TestResendReplicationTasksRequest_GetWorkflowID(t *testing.T) {
	tests := []struct {
		name     string
		request  *ResendReplicationTasksRequest
		expected string
	}{
		{
			name:     "Nil request",
			request:  nil,
			expected: "",
		},
		{
			name:     "Empty WorkflowID",
			request:  &ResendReplicationTasksRequest{},
			expected: "",
		},
		{
			name: "With WorkflowID",
			request: &ResendReplicationTasksRequest{
				WorkflowID: "test-workflow-id",
			},
			expected: "test-workflow-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetWorkflowID()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResendReplicationTasksRequest_GetRunID(t *testing.T) {
	tests := []struct {
		name     string
		request  *ResendReplicationTasksRequest
		expected string
	}{
		{
			name:     "Nil request",
			request:  nil,
			expected: "",
		},
		{
			name:     "Empty RunID",
			request:  &ResendReplicationTasksRequest{},
			expected: "",
		},
		{
			name: "With RunID",
			request: &ResendReplicationTasksRequest{
				RunID: "test-run-id",
			},
			expected: "test-run-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetRunID()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResendReplicationTasksRequest_GetRemoteCluster(t *testing.T) {
	tests := []struct {
		name     string
		request  *ResendReplicationTasksRequest
		expected string
	}{
		{
			name:     "Nil request",
			request:  nil,
			expected: "",
		},
		{
			name:     "Empty RemoteCluster",
			request:  &ResendReplicationTasksRequest{},
			expected: "",
		},
		{
			name: "With RemoteCluster",
			request: &ResendReplicationTasksRequest{
				RemoteCluster: "test-remote-cluster",
			},
			expected: "test-remote-cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetRemoteCluster()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestAdminDeleteWorkflowRequest_GetDomain(t *testing.T) {
	tests := []struct {
		name     string
		request  *AdminDeleteWorkflowRequest
		expected string
	}{
		{
			name:     "Nil request",
			request:  nil,
			expected: "",
		},
		{
			name:     "Empty Domain",
			request:  &AdminDeleteWorkflowRequest{},
			expected: "",
		},
		{
			name: "With Domain",
			request: &AdminDeleteWorkflowRequest{
				Domain: "test-domain",
			},
			expected: "test-domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetDomain()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestAdminDeleteWorkflowRequest_GetExecution(t *testing.T) {
	tests := []struct {
		name     string
		request  *AdminDeleteWorkflowRequest
		expected *WorkflowExecution
	}{
		{
			name:     "Nil request",
			request:  nil,
			expected: nil,
		},
		{
			name:     "Nil Execution",
			request:  &AdminDeleteWorkflowRequest{},
			expected: nil,
		},
		{
			name: "With Execution",
			request: &AdminDeleteWorkflowRequest{
				Execution: &WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
			},
			expected: &WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetExecution()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestAdminDeleteWorkflowRequest_GetSkipErrors(t *testing.T) {
	tests := []struct {
		name     string
		request  *AdminDeleteWorkflowRequest
		expected bool
	}{
		{
			name:     "Nil request",
			request:  nil,
			expected: false,
		},
		{
			name:     "SkipErrors is false",
			request:  &AdminDeleteWorkflowRequest{SkipErrors: false},
			expected: false,
		},
		{
			name:     "SkipErrors is true",
			request:  &AdminDeleteWorkflowRequest{SkipErrors: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.GetSkipErrors()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFromIsolationGroupPartitionList(t *testing.T) {
	tests := []struct {
		name string
		in   []IsolationGroupPartition
		want IsolationGroupConfiguration
	}{
		{
			name: "empty list",
			in:   []IsolationGroupPartition{},
			want: IsolationGroupConfiguration{},
		},
		{
			name: "single group",
			in: []IsolationGroupPartition{
				{Name: "zone-1", State: IsolationGroupStateHealthy},
			},
			want: IsolationGroupConfiguration{
				"zone-1": {Name: "zone-1", State: IsolationGroupStateHealthy},
			},
		},
		{
			name: "multiple groups",
			in: []IsolationGroupPartition{
				{Name: "zone-1", State: IsolationGroupStateHealthy},
				{Name: "zone-2", State: IsolationGroupStateDrained},
			},
			want: IsolationGroupConfiguration{
				"zone-1": {Name: "zone-1", State: IsolationGroupStateHealthy},
				"zone-2": {Name: "zone-2", State: IsolationGroupStateDrained},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromIsolationGroupPartitionList(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func ptrInt64(i int64) *int64 {
	return &i
}
