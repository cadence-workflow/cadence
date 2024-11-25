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
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log/testlogger"
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
				WFID:                        "BDF3D8D9-5235-4CE8-BBDF-6A37589C9DC7",
			},
			expected: igA,
		},
		"default behaviour - wf starting in a zone/isolationGroup must run in an available zone only. If not in available list, return no zone": {
			availablePartitionGroups: isolationGroupsAllHealthy,
			wfPartitionCfg: defaultWorkflowPartitionConfig{
				WorkflowStartIsolationGroup: string("something-else"),
				WFID:                        "BDF3D8D9-5235-4CE8-BBDF-6A37589C9DC7",
			},
			expected: "",
		},
		"... and it should be deterministic": {
			availablePartitionGroups: isolationGroupsAllHealthy,
			wfPartitionCfg: defaultWorkflowPartitionConfig{
				WorkflowStartIsolationGroup: string("something-else"),
				WFID:                        "BDF3D8D9-5235-4CE8-BBDF-6A37589C9DC7",
			},
			expected: "",
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			partitioner := defaultPartitioner{
				log:                 testlogger.New(t),
				isolationGroupState: nil,
			}
			res := partitioner.pickIsolationGroup(td.wfPartitionCfg, td.availablePartitionGroups, PollerInfo{})
			assert.Equal(t, td.expected, res)
		})
	}
}

func TestDefaultPartitioner_GetIsolationGroupByDomainID(t *testing.T) {

	domainID := "some-domain-id"
	sampleTasklist := "a-tasklist"
	validIsolationGroup := types.IsolationGroupConfiguration{
		"zone-2": {
			Name:  "zone-2",
			State: types.IsolationGroupStateHealthy,
		},
		"zone-3": {
			Name:  "zone-3",
			State: types.IsolationGroupStateHealthy,
		},
	}
	isolationGroups := []string{"zone-1", "zone-2", "zone-3"}

	tests := map[string]struct {
		stateAffordance      func(state *isolationgroup.MockState)
		incomingContext      context.Context
		partitionKeyPassedIn PartitionConfig
		expectedValue        string
		expectedError        error
	}{
		"happy path - zone is available - zone pinning": {
			partitionKeyPassedIn: PartitionConfig{
				IsolationGroupKey: "zone-2",
				WorkflowIDKey:     "wf-id",
			},
			incomingContext: context.Background(),
			stateAffordance: func(state *isolationgroup.MockState) {
				state.EXPECT().AvailableIsolationGroupsByDomainID(gomock.Any(), domainID, sampleTasklist, isolationGroups).Return(validIsolationGroup, nil)
			},
			expectedValue: "zone-2",
		},
		"happy path - zone is not - zone fallback": {
			partitionKeyPassedIn: PartitionConfig{
				IsolationGroupKey: "zone-1",
				WorkflowIDKey:     "wf-id",
			},
			incomingContext: context.Background(),
			stateAffordance: func(state *isolationgroup.MockState) {
				state.EXPECT().AvailableIsolationGroupsByDomainID(gomock.Any(), domainID, sampleTasklist, isolationGroups).Return(validIsolationGroup, nil)
			},
			expectedValue: "",
		},
		"Error condition - No zones listed though the feature is enabled": {
			partitionKeyPassedIn: PartitionConfig{
				IsolationGroupKey: "zone-1",
				WorkflowIDKey:     "wf-id",
			},
			incomingContext: context.Background(),
			stateAffordance: func(state *isolationgroup.MockState) {
				state.EXPECT().AvailableIsolationGroupsByDomainID(gomock.Any(), domainID, sampleTasklist, isolationGroups).Return(
					types.IsolationGroupConfiguration{}, nil)
			},
			expectedValue: "",
			expectedError: errors.New("no isolation-groups are available"),
		},
		"Error condition - No isolation-group information passed in": {
			partitionKeyPassedIn: PartitionConfig{},
			stateAffordance:      func(state *isolationgroup.MockState) {},
			incomingContext:      context.Background(),
			expectedValue:        "",
			expectedError:        errors.New("invalid partition config"),
		},
		"Error condition - No isolation-group information passed in 2": {
			partitionKeyPassedIn: nil,
			stateAffordance:      func(state *isolationgroup.MockState) {},
			incomingContext:      context.Background(),
			expectedValue:        "",
			expectedError:        errors.New("invalid partition config"),
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ig := isolationgroup.NewMockState(ctrl)
			td.stateAffordance(ig)
			partitioner := NewDefaultPartitioner(testlogger.New(t), ig)
			res, err := partitioner.GetIsolationGroupByDomainID(td.incomingContext, PollerInfo{
				DomainID:                 domainID,
				TasklistName:             sampleTasklist,
				AvailableIsolationGroups: isolationGroups,
			}, td.partitionKeyPassedIn)

			assert.Equal(t, td.expectedValue, res)
			assert.Equal(t, td.expectedError, err)
		})
	}
}
