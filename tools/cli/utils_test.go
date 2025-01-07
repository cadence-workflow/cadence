// Copyright (c) 2022 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cli

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/authorization"
	"github.com/uber/cadence/common/testing/testdatagen/idlfuzzedtestdata"
	"github.com/uber/cadence/common/types"
)

func Test_JSONHistorySerializer(t *testing.T) {
	serializer := JSONHistorySerializer{}
	h := &types.History{
		Events: []*types.HistoryEvent{
			{
				ID:        1,
				Version:   1,
				EventType: types.EventTypeWorkflowExecutionStarted.Ptr(),
			},
			{
				ID:        2,
				Version:   1,
				EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
			},
		},
	}
	data, err := serializer.Serialize(h)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	h2, err := serializer.Deserialize(data)
	assert.NoError(t, err)
	assert.Equal(t, h, h2)
}

func Test_ParseIntMultiRange(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectOutput []int
		expectError  string
	}{
		{
			name:         "empty",
			input:        "",
			expectOutput: []int{},
		},
		{
			name:         "single number",
			input:        " 1 ",
			expectOutput: []int{1},
		},
		{
			name:         "single range",
			input:        "1 - 3 ",
			expectOutput: []int{1, 2, 3},
		},
		{
			name:         "multi range",
			input:        "1 - 3 ,,  6",
			expectOutput: []int{1, 2, 3, 6},
		},
		{
			name:         "overlapping ranges",
			input:        "1-3,2-4",
			expectOutput: []int{1, 2, 3, 4},
		},
		{
			name:        "invalid single number",
			input:       "1a",
			expectError: "single number \"1a\": strconv.Atoi: parsing \"1a\": invalid syntax",
		},
		{
			name:        "invalid lower bound",
			input:       "1a-2",
			expectError: "lower range of \"1a-2\": strconv.Atoi: parsing \"1a\": invalid syntax",
		},
		{
			name:        "invalid upper bound",
			input:       "1-2a",
			expectError: "upper range of \"1-2a\": strconv.Atoi: parsing \"2a\": invalid syntax",
		},
		{
			name:        "invalid range",
			input:       "1-2-3",
			expectError: "invalid range \"1-2-3\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := parseIntMultiRange(tt.input)
			if tt.expectError != "" {
				assert.EqualError(t, err, tt.expectError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectOutput, output)
			}
		})
	}
}

func Test_anyToStringWorksWithTime(t *testing.T) {
	tm := time.Date(2020, 1, 15, 14, 30, 45, 0, time.UTC)

	assert.Equal(
		t,
		"2020-01-15 14:30:45 +0000 UTC",
		anyToString(tm, false, 100),
	)
	assert.Equal(
		t,
		"2020-01-15 ...  +0000 UTC",
		anyToString(tm, false, 20),
		"trimming should work for time.Time as well",
	)
}

func Test_anyToString(t *testing.T) {
	info := struct {
		Name   string
		Number int
		Time   time.Time
	}{
		"Joel",
		1234,
		time.Date(2019, 1, 15, 14, 30, 45, 0, time.UTC),
	}

	res := anyToString(info, false, 100)
	assert.Equal(t, "{Name:Joel, Number:1234, Time:2019-01-15 14:30:45 +0000 UTC}", res)
}

func TestJSONHistorySerializer_Serialize(t *testing.T) {
	gen := idlfuzzedtestdata.NewFuzzerWithIDLTypes(t)
	h := types.History{}
	gen.Fuzz(&h)
	serializer := JSONHistorySerializer{}
	data, err := serializer.Serialize(&h)
	assert.NoError(t, err)
	roundTrip, err := serializer.Deserialize(data)
	assert.NoError(t, err)
	assert.Equal(t, h, *roundTrip)
}

func TestEventColorFunction(t *testing.T) {
	tests := []struct {
		eventType     types.EventType
		expectedColor func(format string, a ...interface{}) string
	}{
		// Blue color for start events
		{eventType: types.EventTypeWorkflowExecutionStarted, expectedColor: color.BlueString},
		{eventType: types.EventTypeChildWorkflowExecutionStarted, expectedColor: color.BlueString},

		// Green color for completion events
		{eventType: types.EventTypeWorkflowExecutionCompleted, expectedColor: color.GreenString},
		{eventType: types.EventTypeChildWorkflowExecutionCompleted, expectedColor: color.GreenString},

		// Red color for failure events
		{eventType: types.EventTypeWorkflowExecutionFailed, expectedColor: color.RedString},
		{eventType: types.EventTypeRequestCancelActivityTaskFailed, expectedColor: color.RedString},
		{eventType: types.EventTypeCancelTimerFailed, expectedColor: color.RedString},
		{eventType: types.EventTypeStartChildWorkflowExecutionFailed, expectedColor: color.RedString},
		{eventType: types.EventTypeChildWorkflowExecutionFailed, expectedColor: color.RedString},
		{eventType: types.EventTypeRequestCancelExternalWorkflowExecutionFailed, expectedColor: color.RedString},
		{eventType: types.EventTypeSignalExternalWorkflowExecutionFailed, expectedColor: color.RedString},
		{eventType: types.EventTypeActivityTaskFailed, expectedColor: color.RedString},

		// Yellow color for timeout and cancel events
		{eventType: types.EventTypeWorkflowExecutionTimedOut, expectedColor: color.YellowString},
		{eventType: types.EventTypeActivityTaskTimedOut, expectedColor: color.YellowString},
		{eventType: types.EventTypeWorkflowExecutionCanceled, expectedColor: color.YellowString},
		{eventType: types.EventTypeChildWorkflowExecutionTimedOut, expectedColor: color.YellowString},
		{eventType: types.EventTypeDecisionTaskTimedOut, expectedColor: color.YellowString},

		// Magenta color for child workflow canceled
		{eventType: types.EventTypeChildWorkflowExecutionCanceled, expectedColor: color.MagentaString},

		// Default (no color)
		{eventType: types.EventTypeDecisionTaskScheduled, expectedColor: func(format string, a ...interface{}) string { return format }},
	}

	for _, tt := range tests {
		t.Run(tt.eventType.String(), func(t *testing.T) {
			colorFunc := EventColorFunction(tt.eventType)
			result := colorFunc("test message %s", "color")

			// Apply the expected color function to get expected formatted output
			expected := tt.expectedColor("test message %s", "color")

			assert.Equal(t, expected, result, "EventType %v did not return expected color function", tt.eventType)
		})
	}
}

func TestGetEventAttributes(t *testing.T) {
	tests := []struct {
		name     string
		event    *types.HistoryEvent
		expected interface{}
	}{
		// Workflow execution events
		{
			name: "WorkflowExecutionStarted",
			event: &types.HistoryEvent{
				EventType:                               types.EventTypeWorkflowExecutionStarted.Ptr(),
				WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
			},
			expected: &types.WorkflowExecutionStartedEventAttributes{},
		},
		{
			name: "WorkflowExecutionCompleted",
			event: &types.HistoryEvent{
				EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
				WorkflowExecutionCompletedEventAttributes: &types.WorkflowExecutionCompletedEventAttributes{},
			},
			expected: &types.WorkflowExecutionCompletedEventAttributes{},
		},
		{
			name: "WorkflowExecutionFailed",
			event: &types.HistoryEvent{
				EventType:                              types.EventTypeWorkflowExecutionFailed.Ptr(),
				WorkflowExecutionFailedEventAttributes: &types.WorkflowExecutionFailedEventAttributes{},
			},
			expected: &types.WorkflowExecutionFailedEventAttributes{},
		},
		// More workflow execution events
		{
			name: "WorkflowExecutionTimedOut",
			event: &types.HistoryEvent{
				EventType:                                types.EventTypeWorkflowExecutionTimedOut.Ptr(),
				WorkflowExecutionTimedOutEventAttributes: &types.WorkflowExecutionTimedOutEventAttributes{},
			},
			expected: &types.WorkflowExecutionTimedOutEventAttributes{},
		},
		// Decision task events
		{
			name: "DecisionTaskScheduled",
			event: &types.HistoryEvent{
				EventType:                            types.EventTypeDecisionTaskScheduled.Ptr(),
				DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{},
			},
			expected: &types.DecisionTaskScheduledEventAttributes{},
		},
		{
			name: "DecisionTaskStarted",
			event: &types.HistoryEvent{
				EventType:                          types.EventTypeDecisionTaskStarted.Ptr(),
				DecisionTaskStartedEventAttributes: &types.DecisionTaskStartedEventAttributes{},
			},
			expected: &types.DecisionTaskStartedEventAttributes{},
		},
		{
			name: "DecisionTaskCompleted",
			event: &types.HistoryEvent{
				EventType:                            types.EventTypeDecisionTaskCompleted.Ptr(),
				DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{},
			},
			expected: &types.DecisionTaskCompletedEventAttributes{},
		},
		{
			name: "DecisionTaskTimedOut",
			event: &types.HistoryEvent{
				EventType:                           types.EventTypeDecisionTaskTimedOut.Ptr(),
				DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{},
			},
			expected: &types.DecisionTaskTimedOutEventAttributes{},
		},
		// Activity task events
		{
			name: "ActivityTaskScheduled",
			event: &types.HistoryEvent{
				EventType:                            types.EventTypeActivityTaskScheduled.Ptr(),
				ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{},
			},
			expected: &types.ActivityTaskScheduledEventAttributes{},
		},
		{
			name: "ActivityTaskStarted",
			event: &types.HistoryEvent{
				EventType:                          types.EventTypeActivityTaskStarted.Ptr(),
				ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{},
			},
			expected: &types.ActivityTaskStartedEventAttributes{},
		},
		{
			name: "ActivityTaskCompleted",
			event: &types.HistoryEvent{
				EventType:                            types.EventTypeActivityTaskCompleted.Ptr(),
				ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{},
			},
			expected: &types.ActivityTaskCompletedEventAttributes{},
		},
		{
			name: "ActivityTaskFailed",
			event: &types.HistoryEvent{
				EventType:                         types.EventTypeActivityTaskFailed.Ptr(),
				ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{},
			},
			expected: &types.ActivityTaskFailedEventAttributes{},
		},
		{
			name: "ActivityTaskTimedOut",
			event: &types.HistoryEvent{
				EventType:                           types.EventTypeActivityTaskTimedOut.Ptr(),
				ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{},
			},
			expected: &types.ActivityTaskTimedOutEventAttributes{},
		},
		{
			name: "ActivityTaskCancelRequested",
			event: &types.HistoryEvent{
				EventType: types.EventTypeActivityTaskCancelRequested.Ptr(),
				ActivityTaskCancelRequestedEventAttributes: &types.ActivityTaskCancelRequestedEventAttributes{},
			},
			expected: &types.ActivityTaskCancelRequestedEventAttributes{},
		},
		{
			name: "RequestCancelActivityTaskFailed",
			event: &types.HistoryEvent{
				EventType: types.EventTypeRequestCancelActivityTaskFailed.Ptr(),
				RequestCancelActivityTaskFailedEventAttributes: &types.RequestCancelActivityTaskFailedEventAttributes{},
			},
			expected: &types.RequestCancelActivityTaskFailedEventAttributes{},
		},
		{
			name: "ActivityTaskCanceled",
			event: &types.HistoryEvent{
				EventType:                           types.EventTypeActivityTaskCanceled.Ptr(),
				ActivityTaskCanceledEventAttributes: &types.ActivityTaskCanceledEventAttributes{},
			},
			expected: &types.ActivityTaskCanceledEventAttributes{},
		},
		// Timer events
		{
			name: "TimerStarted",
			event: &types.HistoryEvent{
				EventType:                   types.EventTypeTimerStarted.Ptr(),
				TimerStartedEventAttributes: &types.TimerStartedEventAttributes{},
			},
			expected: &types.TimerStartedEventAttributes{},
		},
		{
			name: "TimerFired",
			event: &types.HistoryEvent{
				EventType:                 types.EventTypeTimerFired.Ptr(),
				TimerFiredEventAttributes: &types.TimerFiredEventAttributes{},
			},
			expected: &types.TimerFiredEventAttributes{},
		},
		{
			name: "CancelTimerFailed",
			event: &types.HistoryEvent{
				EventType:                        types.EventTypeCancelTimerFailed.Ptr(),
				CancelTimerFailedEventAttributes: &types.CancelTimerFailedEventAttributes{},
			},
			expected: &types.CancelTimerFailedEventAttributes{},
		},
		{
			name: "TimerCanceled",
			event: &types.HistoryEvent{
				EventType:                    types.EventTypeTimerCanceled.Ptr(),
				TimerCanceledEventAttributes: &types.TimerCanceledEventAttributes{},
			},
			expected: &types.TimerCanceledEventAttributes{},
		},
		// Workflow execution cancellation events
		{
			name: "WorkflowExecutionCancelRequested",
			event: &types.HistoryEvent{
				EventType: types.EventTypeWorkflowExecutionCancelRequested.Ptr(),
				WorkflowExecutionCancelRequestedEventAttributes: &types.WorkflowExecutionCancelRequestedEventAttributes{},
			},
			expected: &types.WorkflowExecutionCancelRequestedEventAttributes{},
		},
		{
			name: "WorkflowExecutionCanceled",
			event: &types.HistoryEvent{
				EventType:                                types.EventTypeWorkflowExecutionCanceled.Ptr(),
				WorkflowExecutionCanceledEventAttributes: &types.WorkflowExecutionCanceledEventAttributes{},
			},
			expected: &types.WorkflowExecutionCanceledEventAttributes{},
		},
		// External workflow execution cancellation events
		{
			name: "RequestCancelExternalWorkflowExecutionInitiated",
			event: &types.HistoryEvent{
				EventType: types.EventTypeRequestCancelExternalWorkflowExecutionInitiated.Ptr(),
				RequestCancelExternalWorkflowExecutionInitiatedEventAttributes: &types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{},
			},
			expected: &types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{},
		},
		{
			name: "RequestCancelExternalWorkflowExecutionFailed",
			event: &types.HistoryEvent{
				EventType: types.EventTypeRequestCancelExternalWorkflowExecutionFailed.Ptr(),
				RequestCancelExternalWorkflowExecutionFailedEventAttributes: &types.RequestCancelExternalWorkflowExecutionFailedEventAttributes{},
			},
			expected: &types.RequestCancelExternalWorkflowExecutionFailedEventAttributes{},
		},
		{
			name: "ExternalWorkflowExecutionCancelRequested",
			event: &types.HistoryEvent{
				EventType: types.EventTypeExternalWorkflowExecutionCancelRequested.Ptr(),
				ExternalWorkflowExecutionCancelRequestedEventAttributes: &types.ExternalWorkflowExecutionCancelRequestedEventAttributes{},
			},
			expected: &types.ExternalWorkflowExecutionCancelRequestedEventAttributes{},
		},
		// Marker and signaling events
		{
			name: "MarkerRecorded",
			event: &types.HistoryEvent{
				EventType:                     types.EventTypeMarkerRecorded.Ptr(),
				MarkerRecordedEventAttributes: &types.MarkerRecordedEventAttributes{},
			},
			expected: &types.MarkerRecordedEventAttributes{},
		},
		{
			name: "WorkflowExecutionSignaled",
			event: &types.HistoryEvent{
				EventType:                                types.EventTypeWorkflowExecutionSignaled.Ptr(),
				WorkflowExecutionSignaledEventAttributes: &types.WorkflowExecutionSignaledEventAttributes{},
			},
			expected: &types.WorkflowExecutionSignaledEventAttributes{},
		},
		{
			name: "WorkflowExecutionTerminated",
			event: &types.HistoryEvent{
				EventType: types.EventTypeWorkflowExecutionTerminated.Ptr(),
				WorkflowExecutionTerminatedEventAttributes: &types.WorkflowExecutionTerminatedEventAttributes{},
			},
			expected: &types.WorkflowExecutionTerminatedEventAttributes{},
		},
		{
			name: "WorkflowExecutionContinuedAsNew",
			event: &types.HistoryEvent{
				EventType: types.EventTypeWorkflowExecutionContinuedAsNew.Ptr(),
				WorkflowExecutionContinuedAsNewEventAttributes: &types.WorkflowExecutionContinuedAsNewEventAttributes{},
			},
			expected: &types.WorkflowExecutionContinuedAsNewEventAttributes{},
		},
		// Child workflow execution events
		{
			name: "StartChildWorkflowExecutionInitiated",
			event: &types.HistoryEvent{
				EventType: types.EventTypeStartChildWorkflowExecutionInitiated.Ptr(),
				StartChildWorkflowExecutionInitiatedEventAttributes: &types.StartChildWorkflowExecutionInitiatedEventAttributes{},
			},
			expected: &types.StartChildWorkflowExecutionInitiatedEventAttributes{},
		},
		{
			name: "StartChildWorkflowExecutionFailed",
			event: &types.HistoryEvent{
				EventType: types.EventTypeStartChildWorkflowExecutionFailed.Ptr(),
				StartChildWorkflowExecutionFailedEventAttributes: &types.StartChildWorkflowExecutionFailedEventAttributes{},
			},
			expected: &types.StartChildWorkflowExecutionFailedEventAttributes{},
		},
		{
			name: "ChildWorkflowExecutionStarted",
			event: &types.HistoryEvent{
				EventType: types.EventTypeChildWorkflowExecutionStarted.Ptr(),
				ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{},
			},
			expected: &types.ChildWorkflowExecutionStartedEventAttributes{},
		},
		{
			name: "ChildWorkflowExecutionCompleted",
			event: &types.HistoryEvent{
				EventType: types.EventTypeChildWorkflowExecutionCompleted.Ptr(),
				ChildWorkflowExecutionCompletedEventAttributes: &types.ChildWorkflowExecutionCompletedEventAttributes{},
			},
			expected: &types.ChildWorkflowExecutionCompletedEventAttributes{},
		},
		{
			name: "ChildWorkflowExecutionFailed",
			event: &types.HistoryEvent{
				EventType: types.EventTypeChildWorkflowExecutionFailed.Ptr(),
				ChildWorkflowExecutionFailedEventAttributes: &types.ChildWorkflowExecutionFailedEventAttributes{},
			},
			expected: &types.ChildWorkflowExecutionFailedEventAttributes{},
		},
		{
			name: "ChildWorkflowExecutionCanceled",
			event: &types.HistoryEvent{
				EventType: types.EventTypeChildWorkflowExecutionCanceled.Ptr(),
				ChildWorkflowExecutionCanceledEventAttributes: &types.ChildWorkflowExecutionCanceledEventAttributes{},
			},
			expected: &types.ChildWorkflowExecutionCanceledEventAttributes{},
		},
		{
			name: "ChildWorkflowExecutionTimedOut",
			event: &types.HistoryEvent{
				EventType: types.EventTypeChildWorkflowExecutionTimedOut.Ptr(),
				ChildWorkflowExecutionTimedOutEventAttributes: &types.ChildWorkflowExecutionTimedOutEventAttributes{},
			},
			expected: &types.ChildWorkflowExecutionTimedOutEventAttributes{},
		},
		{
			name: "ChildWorkflowExecutionTerminated",
			event: &types.HistoryEvent{
				EventType: types.EventTypeChildWorkflowExecutionTerminated.Ptr(),
				ChildWorkflowExecutionTerminatedEventAttributes: &types.ChildWorkflowExecutionTerminatedEventAttributes{},
			},
			expected: &types.ChildWorkflowExecutionTerminatedEventAttributes{},
		},
		// External workflow execution signal events
		{
			name: "SignalExternalWorkflowExecutionInitiated",
			event: &types.HistoryEvent{
				EventType: types.EventTypeSignalExternalWorkflowExecutionInitiated.Ptr(),
				SignalExternalWorkflowExecutionInitiatedEventAttributes: &types.SignalExternalWorkflowExecutionInitiatedEventAttributes{},
			},
			expected: &types.SignalExternalWorkflowExecutionInitiatedEventAttributes{},
		},
		{
			name: "SignalExternalWorkflowExecutionFailed",
			event: &types.HistoryEvent{
				EventType: types.EventTypeSignalExternalWorkflowExecutionFailed.Ptr(),
				SignalExternalWorkflowExecutionFailedEventAttributes: &types.SignalExternalWorkflowExecutionFailedEventAttributes{},
			},
			expected: &types.SignalExternalWorkflowExecutionFailedEventAttributes{},
		},
		{
			name: "ExternalWorkflowExecutionSignaled",
			event: &types.HistoryEvent{
				EventType: types.EventTypeExternalWorkflowExecutionSignaled.Ptr(),
				ExternalWorkflowExecutionSignaledEventAttributes: &types.ExternalWorkflowExecutionSignaledEventAttributes{},
			},
			expected: &types.ExternalWorkflowExecutionSignaledEventAttributes{},
		},
		// Default case: when an unknown EventType is provided
		{
			name: "UnknownEventType",
			event: &types.HistoryEvent{
				EventType: types.EventType(-1).Ptr(), // Undefined EventType
			},
			expected: &types.HistoryEvent{
				EventType: types.EventType(-1).Ptr(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getEventAttributes(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIntSliceToSet(t *testing.T) {
	// Test with a slice containing unique and duplicate elements
	input := []int{1, 2, 2, 3}
	expected := map[int]struct{}{
		1: {},
		2: {},
		3: {},
	}

	// Call the function and check the result
	result := intSliceToSet(input)
	assert.Equal(t, expected, result)
}

func TestPrintMessage(t *testing.T) {
	// Create a buffer to capture the output
	var buf bytes.Buffer

	// Define the message to print
	msg := "Test message"
	expectedOutput := fmt.Sprintf("cadence: %s\n", msg)

	// Call the function with the buffer as output
	printMessage(&buf, msg)

	// Assert that the output matches the expected output
	assert.Equal(t, expectedOutput, buf.String())
}

func TestGetRequiredInt64Option(t *testing.T) {
	tests := []struct {
		name          string
		optionName    string
		optionValue   int64
		isSet         bool
		expectedValue int64
		expectedError string
	}{
		{
			name:          "OptionSet",
			optionName:    "test-option",
			optionValue:   42,
			isSet:         true,
			expectedValue: 42,
			expectedError: "",
		},
		{
			name:          "OptionNotSet",
			optionName:    "test-option",
			optionValue:   0,
			isSet:         false,
			expectedValue: 0,
			expectedError: "option test-option is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a CLI app and flag set
			app := cli.NewApp()
			set := flag.NewFlagSet("test", 0)

			// Define the option flag
			set.Int64(tt.optionName, tt.optionValue, "test option flag")

			// Conditionally set the option based on the test case
			if tt.isSet {
				require.NoError(t, set.Set(tt.optionName, fmt.Sprintf("%d", tt.optionValue)))
			}

			// Create the CLI context
			c := cli.NewContext(app, set, nil)

			// Call getRequiredInt64Option
			result, err := getRequiredInt64Option(c, tt.optionName)

			// Validate results
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}
		})
	}
}

func TestParseSingleTs(t *testing.T) {
	// Test with a valid timestamp format
	validInput := "2023-10-31T14:45:30"
	expectedTime := time.Date(2023, 10, 31, 14, 45, 30, 0, time.UTC)
	result, err := parseSingleTs(validInput)
	assert.NoError(t, err)
	assert.Equal(t, expectedTime, result)

	// Test with an invalid timestamp format
	invalidInput := "31-10-2023"
	result, err = parseSingleTs(invalidInput)
	assert.Error(t, err)
}

func TestPrompt(t *testing.T) {
	// Simulate user input for "y" to prevent os.Exit
	r, w, _ := os.Pipe()
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()
	os.Stdin = r

	fmt.Fprint(w, "y\n") // write simulated input to the pipe
	w.Close()            // close the write end to signal EOF

	prompt("Test message") // if input is "y", it will not exit
}

func TestSerializeSearchAttributes(t *testing.T) {
	// Test with nil input
	result, err := serializeSearchAttributes(nil)
	assert.NoError(t, err)
	assert.Nil(t, result)

	// Test with valid input
	input := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}
	result, err = serializeSearchAttributes(input)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify serialized values
	expectedAttr := make(map[string][]byte)
	for k, v := range input {
		attrBytes, _ := json.Marshal(v)
		expectedAttr[k] = attrBytes
	}
	assert.Equal(t, expectedAttr, result.IndexedFields)

	// Test with input that causes JSON marshalling error
	inputWithError := map[string]interface{}{
		"key": make(chan int), // unsupported type for JSON serialization
	}
	result, err = serializeSearchAttributes(inputWithError)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestPrintError(t *testing.T) {
	showErrorStackEnv := "CADENCE_CLI_SHOW_STACKS"

	tests := []struct {
		name        string
		msg         string
		err         error
		setEnv      bool
		expectedOut string
	}{
		{
			name:        "Error with details and stack trace prompt",
			msg:         "Test message",
			err:         fmt.Errorf("sample error"),
			setEnv:      false,
			expectedOut: fmt.Sprintf("%s Test message\n%s sample error\n('export %s=1' to see stack traces)\n", colorRed("Error:"), colorMagenta("Error Details:"), showErrorStackEnv),
		},
		{
			name:        "Error without details",
			msg:         "Test message",
			err:         nil,
			expectedOut: fmt.Sprintf("%s Test message\n", colorRed("Error:")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			// Set or unset environment variable as needed
			if tt.setEnv {
				os.Setenv(showErrorStackEnv, "1")
			} else {
				os.Unsetenv(showErrorStackEnv)
			}

			// Call the function
			printError(&buf, tt.msg, tt.err)

			// Verify output matches expected output
			output := buf.String()
			assert.Contains(t, output, tt.expectedOut)

			// If stack trace is expected, verify it is included in the output
			if tt.setEnv && tt.err != nil {
				assert.Contains(t, output, "goroutine")
			}
		})
	}
}

func TestRemovePrevious2LinesFromTerminal(t *testing.T) {
	// Create a buffer to capture the output
	var buf bytes.Buffer

	// Call the function
	removePrevious2LinesFromTerminal(&buf)

	// Verify the output matches the expected ANSI escape sequences
	expectedOutput := "\033[1A\033[2K\033[1A\033[2K"
	assert.Equal(t, expectedOutput, buf.String())
}

func TestPrettyPrintJSONObject_Error(t *testing.T) {
	// Define a struct containing an unmarshalable field (e.g., chan type)
	type InvalidStruct struct {
		Name string
		Ch   chan int
	}

	// Create an instance of InvalidStruct
	invalidObj := InvalidStruct{
		Name: "TestName",
		Ch:   make(chan int),
	}

	// Use bytes.Buffer as writer to capture the output
	var buf bytes.Buffer

	// Call the function
	prettyPrintJSONObject(&buf, invalidObj)

	// Verify that output contains error message and raw object content
	output := buf.String()
	assert.Contains(t, output, "Error when try to print pretty:")
	assert.Contains(t, output, "TestName")
	assert.Contains(t, output, "chan int") // Ensure unmarshalable field type information is included
}

func TestCreateJWT(t *testing.T) {
	// Generate a temporary RSA private key for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	// Save the private key in PKCS#8 format to a temporary file
	tmpFile, err := os.CreateTemp("", "test_key.pem")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Encode the private key in PEM PKCS#8 format and write it to the temp file
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	assert.NoError(t, err)
	err = pem.Encode(tmpFile, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})
	assert.NoError(t, err)
	tmpFile.Close()

	// Call createJWT with the path to the temporary private key file
	tokenString, err := createJWT(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenString)

	// Parse the token to validate its claims and signature
	token, err := jwt.ParseWithClaims(tokenString, &authorization.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return &privateKey.PublicKey, nil
	})
	assert.NoError(t, err)
	assert.True(t, token.Valid)

	// Validate the claims
	claims, ok := token.Claims.(*authorization.JWTClaims)
	assert.True(t, ok)
	assert.True(t, claims.Admin)
	assert.WithinDuration(t, time.Now(), claims.IssuedAt.Time, time.Second)
	assert.WithinDuration(t, time.Now().Add(10*time.Minute), claims.ExpiresAt.Time, time.Second)
}
