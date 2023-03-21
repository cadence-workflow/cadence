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

package partition

import (
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
)

func TestPickingAZone(t *testing.T) {

	igA := string("isolationGroupA")
	igB := string("isolationGroupB")
	igC := string("isolationGroupC")

	isolationGroupsAllHealthy := types.IsolationGroupConfiguration{
		igA: {
			Name:  igA,
			State: types.IsolationGroupStateHealthy,
		},
		igB: {
			Name:  igB,
			State: types.IsolationGroupStateHealthy,
		},
		igC: {
			Name:  igC,
			State: types.IsolationGroupStateHealthy,
		},
	}

	tests := map[string]struct {
		availablePartitionGroups types.IsolationGroupConfiguration
		wfPartitionCfg           defaultWorkflowPartitionConfig
		expected                 string
		expectedErr              error
	}{
		"default behaviour - wf starting in a zone/isolationGroup should stay there if everything's healthy": {
			availablePartitionGroups: isolationGroupsAllHealthy,
			wfPartitionCfg: defaultWorkflowPartitionConfig{
				WorkflowStartIsolationGroup: igA,
				RunID:                       "BDF3D8D9-5235-4CE8-BBDF-6A37589C9DC7",
			},
			expected: igA,
		},
		"default behaviour - wf starting in a zone/isolationGroup must run in an available zone only. If not in available list, pick a random one": {
			availablePartitionGroups: isolationGroupsAllHealthy,
			wfPartitionCfg: defaultWorkflowPartitionConfig{
				WorkflowStartIsolationGroup: string("something-else"),
				RunID:                       "BDF3D8D9-5235-4CE8-BBDF-6A37589C9DC7",
			},
			expected: igC,
		},
		"... and it should be deterministic": {
			availablePartitionGroups: isolationGroupsAllHealthy,
			wfPartitionCfg: defaultWorkflowPartitionConfig{
				WorkflowStartIsolationGroup: string("something-else"),
				RunID:                       "BDF3D8D9-5235-4CE8-BBDF-6A37589C9DC7",
			},
			expected: igC,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			res := pickIsolationGroup(td.wfPartitionCfg, td.availablePartitionGroups)
			assert.Equal(t, td.expected, res)
		})
	}
}

func TestDefaultPartitionerFallbackPickerDistribution(t *testing.T) {

	count := make(map[string]int)
	var isolationGroups []string

	for i := 0; i < 100; i++ {
		ig := string(fmt.Sprintf("isolationGroup-%d", i))
		isolationGroups = append(isolationGroups, ig)
		count[ig] = 0
	}

	for i := 0; i < 100000; i++ {
		result := pickIsolationGroupFallback(isolationGroups, defaultWorkflowPartitionConfig{
			WorkflowStartIsolationGroup: "not-a-present-isolationGroup", // always force a fallback to the simple hash
			RunID:                       uuid.New().String(),
		})

		c, ok := count[result]
		if !ok {
			t.Fatal("the result wasn't found in the healthy list, something is wrong with the logic for selecting healthy isolationGroups")
		}
		count[result] = c + 1
	}

	for k, v := range count {
		assert.True(t, v > 0, "failed to pick a isolationGroup %s", k)
	}
}
