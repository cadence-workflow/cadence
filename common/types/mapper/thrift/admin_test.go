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
)

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
	kafkaQueueType := shared.AsyncWorkflowQueueTypeKafka
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
					PredefinedQueueName: strPtr("test-queue"),
				},
			},
			want: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					PredefinedQueueName: "test-queue",
				},
			},
		},
		"kafka inline queue": {
			input: &admin.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: strPtr("test-domain"),
				Configuration: &shared.AsyncWorkflowConfiguration{
					QueueType: &kafkaQueueType,
					KafkaConfig: &shared.AsyncWorkflowKafkaQueueConfiguration{
						Topic:         strPtr("test-topic"),
						DlqTopic:      strPtr("test-dlq-topic"),
						ConsumerGroup: strPtr("test-consumer-group"),
						Brokers:       []string{"test-broker1", "test-broker2"},
						Properties: map[string]string{
							"test-key1": "test-value1",
						},
					},
				},
			},
			want: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
				Configuration: &types.AsyncWorkflowConfiguration{
					QueueType: types.AsyncWorkflowQueueTypeKafka,
					KafkaConfig: &types.AsyncWorkflowKafkaQueueConfiguration{
						Topic:         "test-topic",
						DLQTopic:      "test-dlq-topic",
						ConsumerGroup: "test-consumer-group",
						Brokers:       []string{"test-broker1", "test-broker2"},
						Properties: map[string]string{
							"test-key1": "test-value1",
						},
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
	kafkaQueueType := shared.AsyncWorkflowQueueTypeKafka
	invalidQueueType := shared.AsyncWorkflowQueueTypeInvalid
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
					PredefinedQueueName: "test-queue",
				},
			},
			want: &admin.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &shared.AsyncWorkflowConfiguration{
					PredefinedQueueName: strPtr("test-queue"),
					QueueType:           &invalidQueueType,
				},
			},
		},
		"kafka inline queue": {
			input: &types.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &types.AsyncWorkflowConfiguration{
					QueueType: types.AsyncWorkflowQueueTypeKafka,
					KafkaConfig: &types.AsyncWorkflowKafkaQueueConfiguration{
						Topic:         "test-topic",
						DLQTopic:      "test-dlq-topic",
						ConsumerGroup: "test-consumer-group",
						Brokers:       []string{"test-broker1", "test-broker2"},
						Properties: map[string]string{
							"test-key1": "test-value1",
						},
					},
				},
			},
			want: &admin.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &shared.AsyncWorkflowConfiguration{
					QueueType:           &kafkaQueueType,
					PredefinedQueueName: strPtr(""),
					KafkaConfig: &shared.AsyncWorkflowKafkaQueueConfiguration{
						Topic:         strPtr("test-topic"),
						DlqTopic:      strPtr("test-dlq-topic"),
						ConsumerGroup: strPtr("test-consumer-group"),
						Brokers:       []string{"test-broker1", "test-broker2"},
						Properties: map[string]string{
							"test-key1": "test-value1",
						},
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
