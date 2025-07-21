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
		WorkflowTypeName:                   "WorkflowTypeName",
		WorkflowTimeout:                    time.Minute * time.Duration(rand.Intn(10)),
		DecisionStartToCloseTimeout:        time.Minute * time.Duration(rand.Intn(10)),
		ExecutionContext:                   []byte("ExecutionContext"),
		State:                              rand.Intn(1000),
		CloseStatus:                        rand.Intn(1000),
		LastFirstEventID:                   int64(rand.Intn(1000)),
		LastEventTaskID:                    int64(rand.Intn(1000)),
		LastProcessedEvent:                 int64(rand.Intn(1000)),
		StartTimestamp:                     time.Now(),
		LastUpdatedTimestamp:               time.Now(),
		CreateRequestID:                    "CreateRequestID",
		SignalCount:                        int32(rand.Intn(1000)),
		DecisionVersion:                    int64(rand.Intn(1000)),
		DecisionScheduleID:                 int64(rand.Intn(1000)),
		DecisionStartedID:                  int64(rand.Intn(1000)),
		DecisionRequestID:                  "DecisionRequestID",
		DecisionTimeout:                    time.Minute * time.Duration(rand.Intn(10)),
		DecisionAttempt:                    int64(rand.Intn(1000)),
		DecisionStartedTimestamp:           time.Now(),
		DecisionScheduledTimestamp:         time.Now(),
		DecisionOriginalScheduledTimestamp: time.Now(),
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
		ExpirationTime:                     time.Now(),
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
	assert.Equal(t, expected.ParentDomainID, actual.ParentDomainID)
	assert.Equal(t, expected.ParentWorkflowID, actual.ParentWorkflowID)
	assert.Equal(t, expected.ParentRunID, actual.ParentRunID)
	assert.Equal(t, expected.FirstExecutionRunID, actual.FirstExecutionRunID)
	assert.Equal(t, expected.InitiatedID, actual.InitiatedID)
	assert.Equal(t, expected.CompletionEventBatchID, actual.CompletionEventBatchID)
	assert.Equal(t, expected.CompletionEvent, actual.CompletionEvent)
	assert.Equal(t, expected.TaskList, actual.TaskList)
	assert.Equal(t, expected.WorkflowTypeName, actual.WorkflowTypeName)
	assert.True(t, (expected.WorkflowTimeout-actual.WorkflowTimeout) < time.Second)
	assert.True(t, (expected.DecisionStartToCloseTimeout-actual.DecisionStartToCloseTimeout) < time.Second)
	assert.Equal(t, expected.ExecutionContext, actual.ExecutionContext)
	assert.Equal(t, expected.State, actual.State)
	assert.Equal(t, expected.CloseStatus, actual.CloseStatus)
	assert.Equal(t, expected.LastFirstEventID, actual.LastFirstEventID)
	assert.Equal(t, expected.LastEventTaskID, actual.LastEventTaskID)
	assert.Equal(t, expected.LastProcessedEvent, actual.LastProcessedEvent)
	assert.Equal(t, expected.StartTimestamp.Sub(actual.StartTimestamp), time.Duration(0))
	assert.Equal(t, expected.LastUpdatedTimestamp.Sub(actual.LastUpdatedTimestamp), time.Duration(0))
	assert.Equal(t, expected.CreateRequestID, actual.CreateRequestID)
	assert.Equal(t, expected.SignalCount, actual.SignalCount)
	assert.Equal(t, expected.DecisionVersion, actual.DecisionVersion)
	assert.Equal(t, expected.DecisionScheduleID, actual.DecisionScheduleID)
	assert.Equal(t, expected.DecisionStartedID, actual.DecisionStartedID)
	assert.Equal(t, expected.DecisionRequestID, actual.DecisionRequestID)
	assert.True(t, (expected.DecisionTimeout-actual.DecisionTimeout) < time.Second)
	assert.Equal(t, expected.DecisionAttempt, actual.DecisionAttempt)
	assert.Equal(t, expected.DecisionStartedTimestamp.Sub(actual.DecisionStartedTimestamp), time.Duration(0))
	assert.Equal(t, expected.DecisionScheduledTimestamp.Sub(actual.DecisionScheduledTimestamp), time.Duration(0))
	assert.Equal(t, expected.DecisionOriginalScheduledTimestamp.Sub(actual.DecisionOriginalScheduledTimestamp), time.Duration(0))
	assert.Equal(t, expected.CancelRequested, actual.CancelRequested)
	assert.Equal(t, expected.CancelRequestID, actual.CancelRequestID)
	assert.Equal(t, expected.StickyTaskList, actual.StickyTaskList)
	assert.True(t, (expected.StickyScheduleToStartTimeout-actual.StickyScheduleToStartTimeout) < time.Second)
	assert.Equal(t, expected.ClientLibraryVersion, actual.ClientLibraryVersion)
	assert.Equal(t, expected.ClientFeatureVersion, actual.ClientFeatureVersion)
	assert.Equal(t, expected.ClientImpl, actual.ClientImpl)
	assert.Equal(t, expected.AutoResetPoints, actual.AutoResetPoints)
	assert.Equal(t, expected.Attempt, actual.Attempt)
	assert.Equal(t, expected.HasRetryPolicy, actual.HasRetryPolicy)
	assert.True(t, (expected.InitialInterval-actual.InitialInterval) < time.Second)
	assert.Equal(t, expected.BackoffCoefficient, actual.BackoffCoefficient)
	assert.True(t, (expected.MaximumInterval-actual.MaximumInterval) < time.Second)
	assert.Equal(t, expected.ExpirationTime.Sub(actual.ExpirationTime), time.Duration(0))
	assert.Equal(t, expected.MaximumAttempts, actual.MaximumAttempts)
	assert.Equal(t, expected.NonRetriableErrors, actual.NonRetriableErrors)
	assert.Equal(t, expected.BranchToken, actual.BranchToken)
	assert.Equal(t, expected.CronSchedule, actual.CronSchedule)
	assert.Equal(t, expected.CronOverlapPolicy, actual.CronOverlapPolicy)
	assert.True(t, (expected.ExpirationInterval-actual.ExpirationInterval) < time.Second)
	assert.Equal(t, expected.Memo, actual.Memo)
	assert.Equal(t, expected.SearchAttributes, actual.SearchAttributes)
	assert.Equal(t, expected.HistorySize, actual.HistorySize)
	assert.Equal(t, expected.PartitionConfig, actual.PartitionConfig)
	assert.Equal(t, expected.IsCron, actual.IsCron)
	assert.Equal(t, expected.ActiveClusterSelectionPolicy, actual.ActiveClusterSelectionPolicy)
}
