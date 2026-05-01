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

package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

var testDomain = "domainID"

func TestIsolationPickWritePartition(t *testing.T) {
	tl := "tl"
	cases := []struct {
		name             string
		group            string
		config           *types.TaskListPartitionConfig
		disableIsolation bool
		shouldFallback   bool
		allowed          []string
	}{
		{
			name:  "single partition",
			group: "a",
			config: &types.TaskListPartitionConfig{
				WritePartitions: map[int]*types.TaskListPartition{
					0: {},
				},
			},
			allowed: []string{tl},
		},
		{
			name:  "single partition - isolation disabled",
			group: "a",
			config: &types.TaskListPartitionConfig{
				WritePartitions: map[int]*types.TaskListPartition{
					0: {},
				},
			},
			disableIsolation: true,
			shouldFallback:   true,
			allowed:          []string{"fallback"},
		},
		{
			name:  "multiple partitions - single option",
			group: "b",
			config: &types.TaskListPartitionConfig{
				WritePartitions: map[int]*types.TaskListPartition{
					0: {[]string{"a"}},
					1: {[]string{"b"}},
				},
			},
			allowed: []string{getPartitionTaskListName(tl, 1)},
		},
		{
			name:  "multiple partitions - multiple options",
			group: "a",
			config: &types.TaskListPartitionConfig{
				WritePartitions: map[int]*types.TaskListPartition{
					0: {[]string{"a"}},
					1: {[]string{"a"}},
				},
			},
			allowed: []string{tl, getPartitionTaskListName(tl, 1)},
		},
		{
			name:  "multiple partitions - no match",
			group: "c",
			config: &types.TaskListPartitionConfig{
				WritePartitions: map[int]*types.TaskListPartition{
					0: {[]string{"a"}},
					1: {[]string{"b"}},
				},
			},
			shouldFallback: true,
			allowed:        []string{"fallback"},
		},
		{
			name: "fallback - no group",
			config: &types.TaskListPartitionConfig{
				WritePartitions: map[int]*types.TaskListPartition{
					0: {[]string{"a"}},
					1: {[]string{"b"}},
				},
			},
			shouldFallback: true,
			allowed:        []string{"fallback"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lb, fallback := createWithMocks(t, !tc.disableIsolation, tc.config)
			req := &types.AddDecisionTaskRequest{
				DomainUUID: testDomain,
				TaskList: &types.TaskList{
					Name: tl,
					Kind: types.TaskListKindSticky.Ptr(),
				},
			}
			if tc.group != "" {
				req.PartitionConfig = map[string]string{
					isolationgroup.GroupKey: tc.group,
				}
			}
			if tc.shouldFallback {
				fallback.EXPECT().PickWritePartition(int(types.TaskListTypeDecision), req).Return("fallback").Times(1)
			}
			p := lb.PickWritePartition(0, req)
			assert.Contains(t, tc.allowed, p)
		})
	}
}

func TestIsolationPickReadPartition(t *testing.T) {
	tl := "tl"
	cases := []struct {
		name             string
		group            string
		config           *types.TaskListPartitionConfig
		disableIsolation bool
		allowance        func(balancer *MockWeightedLoadBalancer)
		expected         string
	}{
		{
			name:  "single partition",
			group: "a",
			config: &types.TaskListPartitionConfig{
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {},
				},
			},
			expected: tl,
		},
		{
			name:  "single partition - isolation disabled",
			group: "a",
			config: &types.TaskListPartitionConfig{
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {},
				},
			},
			disableIsolation: true,
			allowance: func(balancer *MockWeightedLoadBalancer) {
				balancer.EXPECT().PickReadPartition(gomock.Any(), gomock.Any(), gomock.Any()).Return("fallback")
			},
			expected: "fallback",
		},
		{
			name:  "multiple partitions - single option",
			group: "b",
			config: &types.TaskListPartitionConfig{
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {[]string{"a"}},
					1: {[]string{"b"}},
				},
			},
			expected: getPartitionTaskListName(tl, 1),
		},
		{
			name:  "multiple partitions - multiple options",
			group: "a",
			config: &types.TaskListPartitionConfig{
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {[]string{"a"}},
					1: {[]string{"a"}},
					2: {[]string{"b"}},
				},
			},
			allowance: func(balancer *MockWeightedLoadBalancer) {
				balancer.EXPECT().PickBetween(testDomain, tl, 0, []int{0, 1}).Return(1)
			},
			expected: getPartitionTaskListName(tl, 1),
		},
		{
			name:  "multiple partitions - no matching",
			group: "c",
			config: &types.TaskListPartitionConfig{
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {[]string{"a"}},
					1: {[]string{"b"}},
				},
			},
			allowance: func(balancer *MockWeightedLoadBalancer) {
				balancer.EXPECT().PickReadPartition(gomock.Any(), gomock.Any(), gomock.Any()).Return("fallback")
			},
			expected: "fallback",
		},
		{
			name: "fallback - no group",
			config: &types.TaskListPartitionConfig{
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {[]string{"a"}},
					1: {[]string{"b"}},
				},
			},
			allowance: func(balancer *MockWeightedLoadBalancer) {
				balancer.EXPECT().PickReadPartition(gomock.Any(), gomock.Any(), gomock.Any()).Return("fallback")
			},
			expected: "fallback",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lb, fallback := createWithMocks(t, !tc.disableIsolation, tc.config)
			req := &types.MatchingQueryWorkflowRequest{
				DomainUUID: testDomain,
				TaskList: &types.TaskList{
					Name: tl,
					Kind: types.TaskListKindSticky.Ptr(),
				},
			}
			if tc.allowance != nil {
				tc.allowance(fallback)
			}
			actual := lb.PickReadPartition(0, req, tc.group)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestIsolationGetPartitionsForGroup(t *testing.T) {
	cases := []struct {
		name       string
		group      string
		partitions map[int]*types.TaskListPartition
		expected   []int
	}{
		{
			name:  "single partition",
			group: "a",
			partitions: map[int]*types.TaskListPartition{
				0: {[]string{"a", "b", "c"}},
			},
			expected: []int{0},
		},
		{
			name:  "single partition - wildcard",
			group: "a",
			partitions: map[int]*types.TaskListPartition{
				0: {},
			},
			expected: []int{0},
		},
		{
			name:  "single partition - no options",
			group: "a",
			partitions: map[int]*types.TaskListPartition{
				0: {[]string{"b"}},
			},
			expected: nil,
		},
		{
			name:  "multiple partitions - single option",
			group: "b",
			partitions: map[int]*types.TaskListPartition{
				0: {[]string{"a", "c"}},
				1: {[]string{"b"}},
			},
			expected: []int{1},
		},
		{
			name:  "multiple partitions - multiple options",
			group: "b",
			partitions: map[int]*types.TaskListPartition{
				0: {[]string{"a", "b", "c"}},
				1: {[]string{"b"}},
				2: {[]string{"d"}},
			},
			expected: []int{0, 1},
		},
		{
			name:  "multiple partitions - multiple options with wildcard",
			group: "b",
			partitions: map[int]*types.TaskListPartition{
				0: {[]string{"a", "c"}},
				1: {[]string{"b"}},
				2: {},
			},
			expected: []int{1, 2},
		},
		{
			name:  "multiple partitions - no options",
			group: "d",
			partitions: map[int]*types.TaskListPartition{
				0: {[]string{"a", "c"}},
				1: {[]string{"b"}},
				2: {[]string{"c"}},
			},
			expected: nil,
		},
		{
			name: "no group",
			partitions: map[int]*types.TaskListPartition{
				0: {[]string{"a", "b", "c"}},
			},
			expected: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getPartitionsForGroup(tc.group, tc.partitions)
			if tc.expected == nil {
				assert.Nil(t, actual)
			} else {
				assert.ElementsMatch(t, tc.expected, actual)
			}
		})
	}
}

func createWithMocks(t *testing.T, isolationEnabled bool, config *types.TaskListPartitionConfig) (*isolationLoadBalancer, *MockWeightedLoadBalancer) {
	ctrl := gomock.NewController(t)
	fallback := NewMockWeightedLoadBalancer(ctrl)
	cfg := NewMockPartitionConfigProvider(ctrl)
	cfg.EXPECT().GetPartitionConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(config).AnyTimes()
	dynamicClient := dynamicconfig.NewInMemoryClient()
	require.NoError(t, dynamicClient.UpdateValue(dynamicproperties.EnablePartitionIsolationGroupAssignment, isolationEnabled))
	dc := dynamicconfig.NewCollection(dynamicClient, testlogger.New(t))
	lb := NewIsolationLoadBalancer(fallback, cfg, func(s string) (string, error) {
		return s, nil
	}, dc).(*isolationLoadBalancer)

	return lb, fallback
}
