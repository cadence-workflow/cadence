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
	"math/rand"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestInternalWorkflowExecutionInfo(t *testing.T) {
	expected := &persistence.InternalWorkflowExecutionInfo{
		ParentDomainID:                     uuid.New(),
		ParentWorkflowID:                   "ParentWorkflowID",
		ParentRunID:                        uuid.New(),
		FirstExecutionRunID:                uuid.New(),
		InitiatedID:                        int64(rand.Intn(1000)),
		CompletionEventBatchID:             int64(rand.Intn(1000)),
		CompletionEvent:                    persistence.NewDataBlob([]byte(`CompletionEvent`), constants.EncodingTypeJSON),
		TaskList:                           "TaskList",
		TaskListKind:                       types.TaskListKindNormal,
		WorkflowTypeName:                   "WorkflowTypeName",
		WorkflowTimeout:                    time.Minute * time.Duration(rand.Intn(10)),
		DecisionStartToCloseTimeout:        time.Minute * time.Duration(rand.Intn(10)),
		ExecutionContext:                   []byte("ExecutionContext"),
		State:                              rand.Intn(1000),
		CloseStatus:                        rand.Intn(1000),
		LastFirstEventID:                   int64(rand.Intn(1000)),
		LastEventTaskID:                    int64(rand.Intn(1000)),
		LastProcessedEvent:                 int64(rand.Intn(1000)),
		StartTimestamp:                     time.UnixMilli(1752018142820),
		LastUpdatedTimestamp:               time.UnixMilli(1752018142821),
		CreateRequestID:                    "CreateRequestID",
		SignalCount:                        int32(rand.Intn(1000)),
		DecisionVersion:                    int64(rand.Intn(1000)),
		DecisionScheduleID:                 int64(rand.Intn(1000)),
		DecisionStartedID:                  int64(rand.Intn(1000)),
		DecisionRequestID:                  "DecisionRequestID",
		DecisionTimeout:                    time.Minute * time.Duration(rand.Intn(10)),
		DecisionAttempt:                    int64(rand.Intn(1000)),
		DecisionStartedTimestamp:           time.UnixMilli(1752018142822),
		DecisionScheduledTimestamp:         time.UnixMilli(1752018142823),
		DecisionOriginalScheduledTimestamp: time.UnixMilli(1752018142824),
		CancelRequested:                    true,
		CancelRequestID:                    "CancelRequestID",
		StickyTaskList:                     "StickyTaskList",
		StickyScheduleToStartTimeout:       time.Minute * time.Duration(rand.Intn(10)),
		ClientLibraryVersion:               "ClientLibraryVersion",
		ClientFeatureVersion:               "ClientFeatureVersion",
		ClientImpl:                         "ClientImpl",
		AutoResetPoints:                    persistence.NewDataBlob([]byte("AutoResetPoints"), constants.EncodingTypeJSON),
		Attempt:                            int32(rand.Intn(1000)),
		HasRetryPolicy:                     true,
		InitialInterval:                    time.Minute * time.Duration(rand.Intn(10)),
		BackoffCoefficient:                 rand.Float64() * 1000,
		MaximumInterval:                    time.Minute * time.Duration(rand.Intn(10)),
		ExpirationTime:                     time.UnixMilli(1752018142825),
		MaximumAttempts:                    int32(rand.Intn(1000)),
		NonRetriableErrors:                 []string{"RetryNonRetryableErrors"},
		BranchToken:                        []byte("EventBranchToken"),
		CronSchedule:                       "CronSchedule",
		ExpirationInterval:                 time.Minute * time.Duration(rand.Intn(10)),
		Memo:                               map[string][]byte{"key_1": []byte("Memo")},
		SearchAttributes:                   map[string][]byte{"key_1": []byte("SearchAttributes")},
		HistorySize:                        int64(rand.Intn(1000)),
		PartitionConfig:                    map[string]string{"zone": "dca1"},
		IsCron:                             true,
		ActiveClusterSelectionPolicy:       persistence.NewDataBlob([]byte("ActiveClusterSelectionPolicy"), constants.EncodingTypeJSON),
		CronOverlapPolicy:                  types.CronOverlapPolicySkipped,
	}
	actual := ToInternalWorkflowExecutionInfo(FromInternalWorkflowExecutionInfo(expected))
	assert.Equal(t, expected, actual)
}
