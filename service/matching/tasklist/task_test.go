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

	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestNewInternalTask(t *testing.T) {
	cases := []struct {
		name                    string
		partitionConfig         map[string]string
		source                  types.TaskSource
		forwardedFrom           string
		forSyncMatch            bool
		isolationGroup          string
		expectedPartitionConfig map[string]string
		additionalAssertions    func(t *testing.T, task *InternalTask)
	}{
		{
			name:         "sync match",
			source:       types.TaskSourceHistory,
			forSyncMatch: true,
			additionalAssertions: func(t *testing.T, task *InternalTask) {
				// Only initialized for sync match
				assert.NotNil(t, task.ResponseC)
				assert.True(t, task.IsSyncMatch())
			},
		},
		{
			name:         "async match",
			source:       types.TaskSourceDbBacklog,
			forSyncMatch: false,
			additionalAssertions: func(t *testing.T, task *InternalTask) {
				// Only initialized for sync match
				assert.Nil(t, task.ResponseC)
				assert.False(t, task.IsSyncMatch())
			},
		},
		{
			name:          "forwarded from history",
			source:        types.TaskSourceDbBacklog,
			forSyncMatch:  true,
			forwardedFrom: "elsewhere",
			additionalAssertions: func(t *testing.T, task *InternalTask) {
				assert.True(t, task.IsForwarded())
				assert.True(t, task.IsSyncMatch())
			},
		},
		{
			name:          "forwarded from backlog",
			source:        types.TaskSourceDbBacklog,
			forSyncMatch:  true,
			forwardedFrom: "elsewhere",
			additionalAssertions: func(t *testing.T, task *InternalTask) {
				assert.True(t, task.IsForwarded())
				// Still technically sync match, just on a different host
				assert.True(t, task.IsSyncMatch())
			},
		},
		{
			name:           "tasklist isolation",
			source:         types.TaskSourceDbBacklog,
			isolationGroup: "a",
			partitionConfig: map[string]string{
				isolationgroup.GroupKey:      "a",
				isolationgroup.WorkflowIDKey: "workflowID",
			},
			expectedPartitionConfig: map[string]string{
				isolationgroup.OriginalGroupKey: "a",
				isolationgroup.GroupKey:         "a",
				isolationgroup.WorkflowIDKey:    "workflowID",
			},
		},
		{
			name:           "tasklist isolation - leaked",
			source:         types.TaskSourceDbBacklog,
			isolationGroup: "",
			partitionConfig: map[string]string{
				isolationgroup.GroupKey:      "a",
				isolationgroup.WorkflowIDKey: "workflowID",
			},
			expectedPartitionConfig: map[string]string{
				isolationgroup.OriginalGroupKey: "a",
				isolationgroup.GroupKey:         "",
				isolationgroup.WorkflowIDKey:    "workflowID",
			},
		},
		{
			name:           "tasklist isolation - forwarded",
			source:         types.TaskSourceDbBacklog,
			isolationGroup: "",
			partitionConfig: map[string]string{
				isolationgroup.OriginalGroupKey: "a",
				isolationgroup.GroupKey:         "",
				isolationgroup.WorkflowIDKey:    "workflowID",
			},
			expectedPartitionConfig: map[string]string{
				isolationgroup.OriginalGroupKey: "a",
				isolationgroup.GroupKey:         "",
				isolationgroup.WorkflowIDKey:    "workflowID",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			completionFunc := func(_ *persistence.TaskInfo, _ error) {}
			taskInfo := defaultTaskInfo(tc.partitionConfig)
			activityDispatchInfo := &types.ActivityTaskDispatchInfo{WorkflowDomain: "domain"}
			task := newInternalTask(taskInfo, completionFunc, tc.source, tc.forwardedFrom, tc.forSyncMatch, activityDispatchInfo, tc.isolationGroup)
			assert.Equal(t, defaultTaskInfo(tc.expectedPartitionConfig), task.Event.TaskInfo)
			assert.NotNil(t, task.Event.completionFunc)
			assert.Equal(t, tc.source, task.source)
			assert.Equal(t, tc.forwardedFrom, task.forwardedFrom)
			assert.Equal(t, tc.isolationGroup, task.isolationGroup)
			assert.Equal(t, activityDispatchInfo, task.ActivityTaskDispatchInfo)
			if tc.additionalAssertions != nil {
				tc.additionalAssertions(t, task)
			}
		})
	}
}

func defaultTaskInfo(partitionConfig map[string]string) *persistence.TaskInfo {
	return &persistence.TaskInfo{
		DomainID:                      "DomainID",
		WorkflowID:                    "WorkflowID",
		RunID:                         "RunID",
		TaskID:                        1,
		ScheduleID:                    2,
		ScheduleToStartTimeoutSeconds: 3,
		Expiry:                        time.UnixMicro(4),
		CreatedTime:                   time.UnixMicro(5),
		PartitionConfig:               partitionConfig,
	}
}
