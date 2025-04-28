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

package tasklist

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
)

const cycleInterval = time.Second * 15

type testCycle struct {
	// If not specified, defaults to the previous cycle's value
	metrics *aggregatePartitionMetrics
	// If not specified, defaults to the previous cycle's value
	partitions map[int]*types.TaskListPartition
	// If not specified then there should be no change this cycle. The returned value should be equal to partitions
	// and changed should be false
	expected map[int]*types.TaskListPartition

	callback func(dynamicClient dynamicconfig.Client)
}

func TestAdjustWritePartitions(t *testing.T) {
	cases := []struct {
		name               string
		groupsPerPartition int
		cycles             []*testCycle
	}{
		{
			name: "initial balance",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
			},
		},
		{
			name: "initial balance - from one partition",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
				{},
				{},
				{},
				{
					partitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
					},
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
			},
		},
		{
			name: "initial balance - from one partition skewed",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 300,
							"b": 75,
							"c": 2,
							"d": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
				{},
				{},
				{},
				{
					partitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
					},
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "c", "d"}},
					},
				},
			},
		},
		{
			name:               "one partition minimum",
			groupsPerPartition: 1,
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a"}},
						1: {IsolationGroups: []string{"b"}},
					},
				},
			},
		},
		{
			name: "require pollers",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a"}},
						1: {IsolationGroups: []string{"a"}},
					},
				},
			},
		},
		{
			name: "fewer groups than partitions",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
							"c": 98,
							"d": 97,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
						2: {},
						3: {},
						4: {},
						5: {},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "d"}},
						1: {IsolationGroups: []string{"a", "d"}},
						2: {IsolationGroups: []string{"a", "d"}},
						3: {IsolationGroups: []string{"b", "c"}},
						4: {IsolationGroups: []string{"b", "c"}},
						5: {IsolationGroups: []string{"b", "c"}},
					},
				},
			},
		},
		{
			name: "fewer partitions than groups",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
							"c": 98,
							"d": 97,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "d"}},
						1: {IsolationGroups: []string{"b", "c"}},
					},
				},
			},
		},
		{
			name:               "fewer active groups than IsolationGroupsPerPartition",
			groupsPerPartition: 4,
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
			},
		},
		{
			name: "single partition",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
				{},
				{},
				{},
				{},
			},
		},
		{
			name: "single group",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a"}},
						1: {IsolationGroups: []string{"a"}},
					},
				},
			},
		},
		{
			name: "pollers added",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
						},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "c"}},
						1: {IsolationGroups: []string{"a", "b", "c"}},
					},
				},
			},
		},
		{
			name: "pollers removed",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "c"}},
						1: {IsolationGroups: []string{"a", "b", "c"}},
					},
				},
				{
					metrics: &aggregatePartitionMetrics{
						totalQPS: 1000,
						qpsByIsolationGroup: map[string]float64{
							"a": 101,
							"b": 100,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
			},
		},
		{
			name: "scale up - single partition",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
							"c": 98,
							"d": 97,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "d"}},
						1: {IsolationGroups: []string{"b", "c"}},
					},
				},
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 101,
							"b": 99,
							"c": 98,
							"d": 97,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "d"}},
						1: {IsolationGroups: []string{"a", "b", "c"}},
					},
				},
			},
		},
		{
			name: "scale up - multiple partition",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 201,
							"b": 99,
							"c": 98,
							"d": 97,
							"e": 15,
							"f": 10,
							"g": 5,
							"h": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
							"e": true,
							"f": true,
							"g": true,
							"h": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "h"}}, // 202
						1: {IsolationGroups: []string{"b", "g"}}, // 104
						2: {IsolationGroups: []string{"c", "f"}}, // 108
						3: {IsolationGroups: []string{"d", "e"}}, // 112
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "h"}},      // 68
						1: {IsolationGroups: []string{"a", "b", "g"}}, // 171
						2: {IsolationGroups: []string{"a", "c", "f"}}, // 175
						3: {IsolationGroups: []string{"d", "e"}},      // 112
					},
				},
			},
		},
		{
			name: "scale up - multiple groups",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 102,
							"b": 101,
							"c": 98,
							"d": 97,
							"e": 15,
							"f": 10,
							"g": 5,
							"h": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
							"e": true,
							"f": true,
							"g": true,
							"h": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "h"}}, // 103
						1: {IsolationGroups: []string{"b", "g"}}, // 106
						2: {IsolationGroups: []string{"c", "f"}}, // 108
						3: {IsolationGroups: []string{"d", "e"}}, // 112
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						// Kind of a weird case as 0 and 1 (which have the groups that need to be split)
						// also happen to be the smallest groups.
						0: {IsolationGroups: []string{"a", "b", "h"}}, // 103
						1: {IsolationGroups: []string{"a", "b", "g"}}, // 106
						2: {IsolationGroups: []string{"c", "f"}},      // 108
						3: {IsolationGroups: []string{"d", "e"}},      // 112
					},
				},
			},
		},
		{
			name: "immediate scale up on new partition",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 303,
							"b": 99,
							"c": 98,
							"d": 97,
							"e": 15,
							"f": 10,
							"g": 5,
							"h": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
							"e": true,
							"f": true,
							"g": true,
							"h": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "g", "h"}}, // 206
						1: {IsolationGroups: []string{"a", "c", "f"}},      // 209
						2: {IsolationGroups: []string{"a", "d", "e"}},      // 213
					},
				},
				{},
				{},
				{},
				{}, // If we had the space "a" would have scaled up on this cycle
				{
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "g", "h"}},
						1: {IsolationGroups: []string{"a", "c", "f"}},
						2: {IsolationGroups: []string{"a", "d", "e"}},
						// adaptive scaler gave us another partition
						3: {},
					},
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "g", "h"}}, // 81.75
						1: {IsolationGroups: []string{"a", "c", "f"}}, // 183.75
						2: {IsolationGroups: []string{"a", "d", "e"}}, // 187.75
						// a is scaled up and b is taken from group 0 because it has the most isolation groups
						// and minimizes the difference in size between the two groups (93 for b vs 96 for g vs 103 for h)
						3: {IsolationGroups: []string{"a", "b"}}, // 174.75
					},
				},
			},
		},
		{
			name: "immediate scale up on new partition - multiple",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 303,
							"b": 303,
							"c": 98,
							"d": 97,
							"e": 15,
							"f": 10,
							"g": 5,
							"h": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
							"e": true,
							"f": true,
							"g": true,
							"h": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "c"}},                // 300
						1: {IsolationGroups: []string{"a", "b", "d"}},                // 299
						2: {IsolationGroups: []string{"a", "b", "e", "f", "g", "h"}}, // 233
					},
				},
				{},
				{},
				{},
				{}, // If we had the space "a" and "b" would have scaled up on this cycle
				{
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "c"}},
						1: {IsolationGroups: []string{"a", "b", "d"}},
						2: {IsolationGroups: []string{"a", "b", "e", "f", "g", "h"}},
						// adaptive scaler gave us another partition
						3: {},
					},
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "c"}},
						1: {IsolationGroups: []string{"a", "b", "d"}},
						2: {IsolationGroups: []string{"a", "b", "e", "f", "g", "h"}},
						3: {IsolationGroups: []string{"a", "b"}},
					},
				},
			},
		},
		{
			name: "scale down - single partition",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 75,
							"b": 99,
							"c": 98,
							"d": 97,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "c"}},
						1: {IsolationGroups: []string{"a", "b", "d"}},
					},
				},
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 74,
							"b": 99,
							"c": 98,
							"d": 97,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "c"}},
						1: {IsolationGroups: []string{"b", "d"}},
					},
				},
			},
		},
		{
			name: "scale down - multiple partition",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 74,
							"b": 99,
							"c": 98,
							"d": 97,
							"e": 15,
							"f": 10,
							"g": 5,
							"h": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
							"e": true,
							"f": true,
							"g": true,
							"h": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "h"}},      // 25
						1: {IsolationGroups: []string{"a", "b", "g"}}, // 129
						2: {IsolationGroups: []string{"a", "c", "f"}}, // 132
						3: {IsolationGroups: []string{"d", "e"}},      // 112
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "h"}}, // 25
						1: {IsolationGroups: []string{"b", "g"}}, // 105
						2: {IsolationGroups: []string{"c", "f"}}, // 108
						3: {IsolationGroups: []string{"d", "e"}}, // 112
					},
				},
			},
		},
		{
			name: "scale down - multiple groups",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 74,
							"b": 73,
							"c": 98,
							"d": 97,
							"e": 15,
							"f": 10,
							"g": 5,
							"h": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
							"e": true,
							"f": true,
							"g": true,
							"h": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "h"}}, // ~49
						1: {IsolationGroups: []string{"a", "b", "g"}}, // ~54
						2: {IsolationGroups: []string{"c", "f"}},      // 108
						3: {IsolationGroups: []string{"d", "e"}},      // 112
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "h"}},
						1: {IsolationGroups: []string{"b", "g"}},
						2: {IsolationGroups: []string{"c", "f"}},
						3: {IsolationGroups: []string{"d", "e"}},
					},
				},
			},
		},
		{
			name: "scale down - single partition left",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 74,
							"b": 73,
							"c": 98,
							"d": 97,
							"e": 15,
							"f": 10,
							"g": 5,
							"h": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
							"e": true,
							"f": true,
							"g": true,
							"h": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "h"}},
					},
					expected: map[int]*types.TaskListPartition{
						0: {},
					},
				},
			},
		},
		{
			name: "scale up then down",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
							"c": 98,
							"d": 97,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "d"}},
						1: {IsolationGroups: []string{"b", "c"}},
					},
				},
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 101,
							"b": 99,
							"c": 98,
							"d": 97,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "d"}},
						1: {IsolationGroups: []string{"a", "b", "c"}},
					},
				},
				{
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "d"}},
						1: {IsolationGroups: []string{"a", "b", "c"}},
					},
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 199,
							"b": 99,
							"c": 98,
							"d": 97,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
				},
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 74,
							"b": 99,
							"c": 98,
							"d": 97,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							"c": true,
							"d": true,
						},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "d"}},
						1: {IsolationGroups: []string{"b", "c"}},
					},
				},
			},
		},
		{
			name: "poller added then removed",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a"}},
						1: {IsolationGroups: []string{"a"}},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
				{
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 100,
							"b": 99,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
						},
					},
				},
				{},
				{},
				{},
				{
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a"}},
						1: {IsolationGroups: []string{"a"}},
					},
				},
				{
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a"}},
						1: {IsolationGroups: []string{"a"}},
					},
				},
			},
		},
		{
			name: "ensure minimum partitions",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 2,
							"b": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a"}},
						1: {IsolationGroups: []string{"b"}},
					},
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
			},
		},
		{
			name:               "increase minimum partitions",
			groupsPerPartition: 1,
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 2,
							"b": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a"}},
						1: {IsolationGroups: []string{"b"}},
					},
				},
				{
					callback: func(dynamicClient dynamicconfig.Client) {
						require.NoError(t, dynamicClient.UpdateValue(dynamicproperties.MatchingIsolationGroupsPerPartition, 2))
					},
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
			},
		},
		{
			name:               "decrease minimum partitions",
			groupsPerPartition: 2,
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 2,
							"b": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a"}},
						1: {IsolationGroups: []string{"b"}},
					},
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
				{
					callback: func(dynamicClient dynamicconfig.Client) {
						require.NoError(t, dynamicClient.UpdateValue(dynamicproperties.MatchingIsolationGroupsPerPartition, 1))
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
				{},
				{},
				{},
				{
					// Has to go through a normal scale down operation
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"b"}},
						1: {IsolationGroups: []string{"a"}},
					},
				},
			},
		},
		{
			name: "new partition - increase partitionsPerGroup",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 2,
							"b": 1,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
					},
				},
				{
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
						2: {},
					},
					// adaptive scaler gave us a new partition and even though we don't think these groups need more
					// partitions, we need to ensure every partition is assigned groups
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},
						1: {IsolationGroups: []string{"a", "b"}},
						2: {IsolationGroups: []string{"a", "b"}},
					},
				},
			},
		},
		{
			name: "new partition - move groups",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 99,
							"b": 1,
							"c": 2,
							"d": 3,
							"e": 4,
							"f": 5,
							"g": 6,
							"h": 7,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
							// Even though the others aren't polled, they're still active because they're assigned
							// to a partition and we haven't been missing pollers long enough to remove them
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},                     // 100
						1: {IsolationGroups: []string{"c", "d", "e", "f", "g", "h"}}, // 27
					},
				},
				{
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},                     // 100
						1: {IsolationGroups: []string{"c", "d", "e", "f", "g", "h"}}, // 27
						2: {},
					},
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b"}},           // 100
						1: {IsolationGroups: []string{"c", "d", "e", "f"}}, // 13
						2: {IsolationGroups: []string{"g", "h"}},           // 14
					},
				},
			},
		},
		{
			name: "new partition - move groups from multiple partitions",
			cycles: []*testCycle{
				{
					metrics: &aggregatePartitionMetrics{
						qpsByIsolationGroup: map[string]float64{
							"a": 99,
							"b": 1,
							"c": 2,
							"d": 3,
							"e": 4,
							"f": 5,
						},
						hasPollersByIsolationGroup: map[string]bool{
							"a": true,
							"b": true,
						},
					},
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "c"}}, // 103
						1: {IsolationGroups: []string{"d", "e", "f"}}, // 12
					},
				},
				{
					partitions: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"a", "b", "c"}}, // 103
						1: {IsolationGroups: []string{"d", "e", "f"}}, // 12
						2: {},
					},
					expected: map[int]*types.TaskListPartition{
						0: {IsolationGroups: []string{"b", "c"}}, // 3
						1: {IsolationGroups: []string{"e", "f"}}, // 9
						2: {IsolationGroups: []string{"a", "d"}}, // 102
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cycles := tc.cycles
			dynamicClient := dynamicconfig.NewInMemoryClient()
			tcConfig := isolationConfig(t, dynamicClient, tc.groupsPerPartition)
			mockTime := clock.NewMockedTimeSource()
			balancer := newIsolationBalancer(mockTime, metrics.NoopScope(metrics.Matching), tcConfig)
			var prevMetrics *aggregatePartitionMetrics
			var prevPartitions map[int]*types.TaskListPartition
			for i, cycle := range cycles {
				mockTime.Advance(cycleInterval)
				cycleMetrics := cycle.metrics
				if cycleMetrics == nil {
					cycleMetrics = prevMetrics
				}
				cyclePartitions := cycle.partitions
				if cyclePartitions == nil {
					cyclePartitions = prevPartitions
				}
				if len(cyclePartitions) == 0 || cycleMetrics == nil {
					assert.Fail(t, "invalid cycle configuration at cycle %d", i)
					return
				}
				if cycle.callback != nil {
					cycle.callback(dynamicClient)
				}
				actual, actualChanged := balancer.adjustWritePartitions(cycleMetrics, cyclePartitions)
				if cycle.expected != nil {
					assert.True(t, actualChanged, "cycle %d - expected change", i)
					assert.Equal(t, cycle.expected, actual, "cycle %d - did not match expected", i)
				} else {
					assert.False(t, actualChanged, "cycle %d - expected no change", i)
					assert.Equal(t, cyclePartitions, actual, "cycle %d - was unexpectedly changed", i)
				}
				prevMetrics = cycleMetrics
				prevPartitions = cyclePartitions
			}
		})
	}
}

func isolationConfig(t *testing.T, dynamicClient dynamicconfig.Client, groupsPerPartition int) *config.TaskListConfig {
	if groupsPerPartition == 0 {
		groupsPerPartition = 2
	}
	taskListID, err := NewIdentifier("test-domain-id", "test-task-list", 0)
	require.NoError(t, err)
	logger := testlogger.New(t)
	cfg := newTaskListConfig(taskListID, config.NewConfig(dynamicconfig.NewCollection(dynamicClient, logger), "test-host", func() []string { return nil }), "test-domain")
	require.NoError(t, dynamicClient.UpdateValue(dynamicproperties.MatchingPartitionUpscaleRPS, 200))
	require.NoError(t, dynamicClient.UpdateValue(dynamicproperties.MatchingPartitionDownscaleFactor, 0.75))
	require.NoError(t, dynamicClient.UpdateValue(dynamicproperties.MatchingIsolationGroupsPerPartition, groupsPerPartition))
	require.NoError(t, dynamicClient.UpdateValue(dynamicproperties.MatchingIsolationGroupUpscaleSustainedDuration, cycleInterval*4))
	require.NoError(t, dynamicClient.UpdateValue(dynamicproperties.MatchingIsolationGroupDownscaleSustainedDuration, cycleInterval*4))
	require.NoError(t, dynamicClient.UpdateValue(dynamicproperties.MatchingIsolationGroupHasPollersSustainedDuration, cycleInterval*4))
	require.NoError(t, dynamicClient.UpdateValue(dynamicproperties.MatchingIsolationGroupNoPollersSustainedDuration, cycleInterval*4))

	return cfg
}
