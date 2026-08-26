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

package serialization

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
)

func TestTaskSerializerThriftRW(t *testing.T) {
	dc := &persistence.DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	}
	parser, err := NewParser(dc)
	require.NoError(t, err)
	taskSerializer := NewTaskSerializer(parser)

	workflowIdentifier := persistence.WorkflowIdentifier{
		DomainID:   "8be8a310-7d20-483e-a5d2-48659dc47602",
		WorkflowID: "test-workflow",
		RunID:      "4be8a310-7d20-483e-a5d2-48659dc47609",
	}

	testCases := []struct {
		category persistence.HistoryTaskCategory
		task     persistence.Task
	}{
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.DecisionTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: time.Unix(1, 1),
				},
				TargetDomainID: "0be8a310-7d20-483e-a5d2-48659dc47602",
				TaskList:       "test-tl",
				ScheduleID:     1,
			},
		},
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.ActivityTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             2,
					TaskID:              2,
					VisibilityTimestamp: time.Unix(2, 2),
				},
				TargetDomainID: "2be8a310-7d20-483e-a5d2-48659dc47602",
				TaskList:       "test-tl2",
				ScheduleID:     2,
			},
		},
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.CloseExecutionTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             3,
					TaskID:              3,
					VisibilityTimestamp: time.Unix(3, 3),
				},
			},
		},
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.CancelExecutionTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             4,
					TaskID:              4,
					VisibilityTimestamp: time.Unix(4, 4),
				},
				TargetDomainID:          "3be8a310-7d20-483e-a5d2-48659dc47602",
				TargetWorkflowID:        "target-wf",
				TargetRunID:             "5be8a310-7d20-483e-a5d2-48659dc47609",
				TargetChildWorkflowOnly: true,
				InitiatedID:             4,
			},
		},
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.StartChildExecutionTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             5,
					TaskID:              5,
					VisibilityTimestamp: time.Unix(5, 5),
				},
				TargetDomainID:   "3be8a310-7d20-483e-a5d2-48659dc47602",
				TargetWorkflowID: "target-wf",
				InitiatedID:      5,
			},
		},
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.SignalExecutionTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             5,
					TaskID:              5,
					VisibilityTimestamp: time.Unix(5, 5),
				},
				TargetDomainID:          "4be8a310-7d20-483e-a5d2-48659dc47608",
				TargetWorkflowID:        "target-wf2",
				TargetRunID:             "7be8a310-7d20-483e-a5d2-48659dc47606",
				TargetChildWorkflowOnly: true,
				InitiatedID:             5,
			},
		},
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.RecordWorkflowStartedTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             6,
					TaskID:              6,
					VisibilityTimestamp: time.Unix(6, 6),
				},
			},
		},
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.ResetWorkflowTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             7,
					TaskID:              7,
					VisibilityTimestamp: time.Unix(7, 7),
				},
			},
		},
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.UpsertWorkflowSearchAttributesTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             8,
					TaskID:              8,
					VisibilityTimestamp: time.Unix(8, 8),
				},
			},
		},
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.RecordWorkflowClosedTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             9,
					TaskID:              9,
					VisibilityTimestamp: time.Unix(9, 9),
				},
			},
		},
		{
			category: persistence.HistoryTaskCategoryTransfer,
			task: &persistence.RecordChildExecutionCompletedTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             10,
					TaskID:              10,
					VisibilityTimestamp: time.Unix(10, 10),
				},
				TargetDomainID:   "7be8a310-7d20-483e-a5d2-48659dc47608",
				TargetWorkflowID: "target-wf2",
				TargetRunID:      "2be8a310-7d20-483e-a5d2-48659dc47606",
			},
		},
		{
			category: persistence.HistoryTaskCategoryTimer,
			task: &persistence.DecisionTimeoutTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             11,
					TaskID:              11,
					VisibilityTimestamp: time.Unix(11, 11),
				},
				EventID:         11,
				ScheduleAttempt: 11,
				TimeoutType:     11,
			},
		},
		{
			category: persistence.HistoryTaskCategoryTimer,
			task: &persistence.ActivityTimeoutTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             12,
					TaskID:              12,
					VisibilityTimestamp: time.Unix(12, 12),
				},
				EventID:     12,
				Attempt:     12,
				TimeoutType: 12,
			},
		},
		{
			category: persistence.HistoryTaskCategoryTimer,
			task: &persistence.UserTimerTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             13,
					TaskID:              13,
					VisibilityTimestamp: time.Unix(13, 13),
				},
				EventID: 13,
			},
		},
		{
			category: persistence.HistoryTaskCategoryTimer,
			task: &persistence.WorkflowTimeoutTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             14,
					TaskID:              14,
					VisibilityTimestamp: time.Unix(14, 14),
				},
			},
		},
		{
			category: persistence.HistoryTaskCategoryTimer,
			task: &persistence.DeleteHistoryEventTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             15,
					TaskID:              15,
					VisibilityTimestamp: time.Unix(15, 15),
				},
			},
		},
		{
			category: persistence.HistoryTaskCategoryTimer,
			task: &persistence.ActivityRetryTimerTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             16,
					TaskID:              16,
					VisibilityTimestamp: time.Unix(16, 16),
				},
				EventID: 16,
				Attempt: 16,
			},
		},
		{
			category: persistence.HistoryTaskCategoryTimer,
			task: &persistence.WorkflowBackoffTimerTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             17,
					TaskID:              17,
					VisibilityTimestamp: time.Unix(17, 17),
				},
				TimeoutType: 17,
			},
		},
		{
			category: persistence.HistoryTaskCategoryReplication,
			task: &persistence.HistoryReplicationTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             18,
					TaskID:              18,
					VisibilityTimestamp: time.Unix(18, 18),
				},
				FirstEventID:      18,
				NextEventID:       18,
				BranchToken:       []byte("18"),
				NewRunBranchToken: []byte("180"),
			},
		},
		{
			category: persistence.HistoryTaskCategoryReplication,
			task: &persistence.SyncActivityTask{
				WorkflowIdentifier: workflowIdentifier,
				TaskData: persistence.TaskData{
					Version:             19,
					TaskID:              19,
					VisibilityTimestamp: time.Unix(19, 19),
				},
				ScheduledID: 19,
			},
		},
		{
			category: persistence.HistoryTaskCategoryReplication,
			task: &persistence.FailoverMarkerTask{
				DomainID: "4be8a310-7d20-483e-a5d2-48659dc47608",
				TaskData: persistence.TaskData{
					Version:             20,
					TaskID:              20,
					VisibilityTimestamp: time.Unix(20, 20),
				},
			},
		},
	}

	for _, tc := range testCases {
		blob, err := taskSerializer.SerializeTask(tc.category, tc.task)
		assert.NoError(t, err)
		task, err := taskSerializer.DeserializeTask(tc.category, &blob)
		assert.NoError(t, err)
		task.SetTaskID(tc.task.GetTaskID())
		if tc.category.Type() == persistence.HistoryTaskCategoryTypeScheduled {
			task.SetVisibilityTimestamp(tc.task.GetVisibilityTimestamp())
		}
		assert.Equal(t, tc.task, task)
	}
}
