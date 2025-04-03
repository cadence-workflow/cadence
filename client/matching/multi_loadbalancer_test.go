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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/collection"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

func TestNewMultiLoadBalancer(t *testing.T) {
	ctrl := gomock.NewController(t)
	randomMock := NewMockLoadBalancer(ctrl)
	roundRobinMock := NewMockLoadBalancer(ctrl)
	lbs := map[string]LoadBalancer{
		"random":      randomMock,
		"round-robin": roundRobinMock,
	}
	domainIDToName := func(domainID string) (string, error) {
		return "testDomainName", nil
	}
	dc := collection.NewCollection(dynamicconfig.NewNopClient(), testlogger.New(t))
	lb := NewMultiLoadBalancer(randomMock, lbs, domainIDToName, dc, testlogger.New(t))
	assert.NotNil(t, lb)
	multiLB, ok := lb.(*multiLoadBalancer)
	assert.NotNil(t, multiLB)
	assert.True(t, ok)
	assert.NotNil(t, multiLB.defaultLoadBalancer)
	assert.NotNil(t, multiLB.loadBalancers)
	assert.NotNil(t, multiLB.domainIDToName)
	assert.NotNil(t, multiLB.loadbalancerStrategy)
	assert.NotNil(t, multiLB.logger)
}

func TestMultiLoadBalancer_PickWritePartition(t *testing.T) {

	// Mock the domainIDToName function
	domainIDToName := func(domainID string) (string, error) {
		if domainID == "valid-domain" {
			return "valid-domain-name", nil
		}
		return "", errors.New("domain not found")
	}

	// Test cases
	tests := []struct {
		name                 string
		domainID             string
		taskList             types.TaskList
		taskListType         int
		forwardedFrom        string
		loadbalancerStrategy string
		expectedPartition    string
	}{
		{
			name:                 "random partition when domainIDToName fails",
			domainID:             "invalid-domain",
			taskList:             types.TaskList{Name: "test-tasklist"},
			taskListType:         1,
			forwardedFrom:        "",
			loadbalancerStrategy: "random",
			expectedPartition:    "random-partition",
		},
		{
			name:                 "round-robin partition enabled",
			domainID:             "valid-domain",
			taskList:             types.TaskList{Name: "test-tasklist"},
			taskListType:         1,
			forwardedFrom:        "",
			loadbalancerStrategy: "round-robin",
			expectedPartition:    "roundrobin-partition",
		},
		{
			name:                 "random partition when round-robin disabled",
			domainID:             "valid-domain",
			taskList:             types.TaskList{Name: "test-tasklist"},
			taskListType:         1,
			forwardedFrom:        "",
			loadbalancerStrategy: "invalid-enum",
			expectedPartition:    "random-partition",
		},
		{
			name:                 "cannot repartition - forwarded",
			domainID:             "valid-domain",
			taskList:             types.TaskList{Name: "test-tasklist"},
			taskListType:         1,
			forwardedFrom:        "somewhere",
			loadbalancerStrategy: "random",
			expectedPartition:    "test-tasklist",
		},
		{
			name:                 "cannot repartition - sticky",
			domainID:             "valid-domain",
			taskList:             types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindSticky.Ptr()},
			taskListType:         1,
			forwardedFrom:        "",
			loadbalancerStrategy: "random",
			expectedPartition:    "test-tasklist",
		},
		{
			name:                 "cannot repartition - partition",
			domainID:             "valid-domain",
			taskList:             types.TaskList{Name: "/__cadence_sys/test-tasklist"},
			taskListType:         1,
			forwardedFrom:        "",
			loadbalancerStrategy: "random",
			expectedPartition:    "/__cadence_sys/test-tasklist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &types.AddDecisionTaskRequest{
				DomainUUID:    tt.domainID,
				TaskList:      &tt.taskList,
				ForwardedFrom: tt.forwardedFrom,
			}
			// Mock behavior for random and round robin load balancers
			ctrl := gomock.NewController(t)

			// Mock the LoadBalancer interface
			randomMock := NewMockLoadBalancer(ctrl)
			roundRobinMock := NewMockLoadBalancer(ctrl)
			randomMock.EXPECT().PickWritePartition(tt.taskListType, req).Return("random-partition").AnyTimes()
			roundRobinMock.EXPECT().PickWritePartition(tt.taskListType, req).Return("roundrobin-partition").AnyTimes()

			loadbalancerStrategyFn := func(domainName, taskListName string, taskListType int) string {
				return tt.loadbalancerStrategy
			}

			// Create multiLoadBalancer
			lb := &multiLoadBalancer{
				defaultLoadBalancer: randomMock,
				loadBalancers: map[string]LoadBalancer{
					"round-robin": roundRobinMock,
				},
				domainIDToName:       domainIDToName,
				loadbalancerStrategy: loadbalancerStrategyFn,
				logger:               testlogger.New(t),
			}

			// Call PickWritePartition and assert result
			partition := lb.PickWritePartition(tt.taskListType, req)
			assert.Equal(t, tt.expectedPartition, partition)
		})
	}
}

func TestMultiLoadBalancer_PickReadPartition(t *testing.T) {

	// Mock the domainIDToName function
	domainIDToName := func(domainID string) (string, error) {
		if domainID == "valid-domain" {
			return "valid-domain-name", nil
		}
		return "", errors.New("domain not found")
	}

	// Test cases
	tests := []struct {
		name                 string
		domainID             string
		taskList             types.TaskList
		taskListType         int
		forwardedFrom        string
		loadbalancerStrategy string
		expectedPartition    string
	}{
		{
			name:                 "random partition when domainIDToName fails",
			domainID:             "invalid-domain",
			taskList:             types.TaskList{Name: "test-tasklist"},
			taskListType:         1,
			forwardedFrom:        "",
			loadbalancerStrategy: "random",
			expectedPartition:    "random-partition",
		},
		{
			name:                 "round-robin partition enabled",
			domainID:             "valid-domain",
			taskList:             types.TaskList{Name: "test-tasklist"},
			taskListType:         1,
			forwardedFrom:        "",
			loadbalancerStrategy: "round-robin",
			expectedPartition:    "roundrobin-partition",
		},
		{
			name:                 "random partition when round-robin disabled",
			domainID:             "valid-domain",
			taskList:             types.TaskList{Name: "test-tasklist"},
			taskListType:         1,
			forwardedFrom:        "",
			loadbalancerStrategy: "invalid-enum",
			expectedPartition:    "random-partition",
		},
		{
			name:                 "cannot repartition - forwarded",
			domainID:             "valid-domain",
			taskList:             types.TaskList{Name: "test-tasklist"},
			taskListType:         1,
			forwardedFrom:        "somewhere",
			loadbalancerStrategy: "random",
			expectedPartition:    "test-tasklist",
		},
		{
			name:                 "cannot repartition - sticky",
			domainID:             "valid-domain",
			taskList:             types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindSticky.Ptr()},
			taskListType:         1,
			forwardedFrom:        "",
			loadbalancerStrategy: "random",
			expectedPartition:    "test-tasklist",
		},
		{
			name:                 "cannot repartition - partition",
			domainID:             "valid-domain",
			taskList:             types.TaskList{Name: "/__cadence_sys/test-tasklist"},
			taskListType:         1,
			forwardedFrom:        "",
			loadbalancerStrategy: "random",
			expectedPartition:    "/__cadence_sys/test-tasklist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &types.AddDecisionTaskRequest{
				DomainUUID:    tt.domainID,
				TaskList:      &tt.taskList,
				ForwardedFrom: tt.forwardedFrom,
			}
			// Mock behavior for random and round robin load balancers
			ctrl := gomock.NewController(t)

			// Mock the LoadBalancer interface
			randomMock := NewMockLoadBalancer(ctrl)
			roundRobinMock := NewMockLoadBalancer(ctrl)
			randomMock.EXPECT().PickReadPartition(tt.taskListType, req, "").Return("random-partition").AnyTimes()
			roundRobinMock.EXPECT().PickReadPartition(tt.taskListType, req, "").Return("roundrobin-partition").AnyTimes()

			// Mock dynamic config for loadbalancer strategy
			loadbalancerStrategyFn := func(domainName, taskListName string, taskListType int) string {
				return tt.loadbalancerStrategy
			}

			// Create multiLoadBalancer
			lb := &multiLoadBalancer{
				defaultLoadBalancer: randomMock,
				loadBalancers: map[string]LoadBalancer{
					"round-robin": roundRobinMock,
				},
				domainIDToName:       domainIDToName,
				loadbalancerStrategy: loadbalancerStrategyFn,
				logger:               testlogger.New(t),
			}

			// Call PickReadPartition and assert result
			partition := lb.PickReadPartition(tt.taskListType, req, "")
			assert.Equal(t, tt.expectedPartition, partition)
		})
	}
}

func TestMultiLoadBalancer_UpdateWeight(t *testing.T) {
	// Mock the domainIDToName function
	domainIDToName := func(domainID string) (string, error) {
		if domainID == "valid-domain" {
			return "valid-domain-name", nil
		}
		return "", errors.New("domain not found")
	}

	// Test cases
	tests := []struct {
		name                 string
		domainID             string
		taskList             types.TaskList
		taskListType         int
		forwardedFrom        string
		partition            string
		loadBalancerHints    *types.LoadBalancerHints
		loadbalancerStrategy string
		shouldUpdate         bool
	}{
		{
			name:          "do nothing when domainIDToName fails",
			domainID:      "invalid-domain",
			taskList:      types.TaskList{Name: "test-tasklist"},
			taskListType:  1,
			forwardedFrom: "",
			partition:     "partition-1",
			loadBalancerHints: &types.LoadBalancerHints{
				BacklogCount:  10,
				RatePerSecond: 1,
			},
			loadbalancerStrategy: "random",
			shouldUpdate:         false,
		},
		{
			name:          "update weight with round-robin load balancer",
			domainID:      "valid-domain",
			taskList:      types.TaskList{Name: "test-tasklist"},
			taskListType:  1,
			forwardedFrom: "",
			partition:     "partition-2",
			loadBalancerHints: &types.LoadBalancerHints{
				BacklogCount:  20,
				RatePerSecond: 2,
			},
			loadbalancerStrategy: "round-robin",
			shouldUpdate:         true,
		},
		{
			name:          "do nothing when strategy is unsupported",
			domainID:      "valid-domain",
			taskList:      types.TaskList{Name: "test-tasklist"},
			taskListType:  1,
			forwardedFrom: "",
			partition:     "partition-3",
			loadBalancerHints: &types.LoadBalancerHints{
				BacklogCount:  30,
				RatePerSecond: 3,
			},
			loadbalancerStrategy: "invalid-strategy",
			shouldUpdate:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &types.AddDecisionTaskRequest{
				DomainUUID:    tt.domainID,
				TaskList:      &tt.taskList,
				ForwardedFrom: tt.forwardedFrom,
			}
			// Mock behavior for random and round-robin load balancers
			ctrl := gomock.NewController(t)

			// Mock the LoadBalancer interface
			randomMock := NewMockLoadBalancer(ctrl)
			roundRobinMock := NewMockLoadBalancer(ctrl)

			if tt.shouldUpdate {
				roundRobinMock.EXPECT().UpdateWeight(tt.taskListType, req, tt.partition, tt.loadBalancerHints).Times(1)
			} else {
				roundRobinMock.EXPECT().UpdateWeight(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			}

			loadbalancerStrategyFn := func(domainName, taskListName string, taskListType int) string {
				return tt.loadbalancerStrategy
			}

			// Create multiLoadBalancer
			lb := &multiLoadBalancer{
				defaultLoadBalancer: randomMock,
				loadBalancers: map[string]LoadBalancer{
					"round-robin": roundRobinMock,
				},
				domainIDToName:       domainIDToName,
				loadbalancerStrategy: loadbalancerStrategyFn,
				logger:               testlogger.New(t),
			}

			// Call UpdateWeight
			lb.UpdateWeight(tt.taskListType, req, tt.partition, tt.loadBalancerHints)
		})
	}
}
