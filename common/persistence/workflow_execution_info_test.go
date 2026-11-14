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

package persistence

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	workflowStates = map[string]int{
		"Void":      WorkflowStateVoid,
		"Created":   WorkflowStateCreated,
		"Running":   WorkflowStateRunning,
		"Completed": WorkflowStateCompleted,
		"Zombie":    WorkflowStateZombie,
		"Preparing": WorkflowStatePreparing,
	}
	workflowCloseStatuses = map[string]int{
		"None":           WorkflowCloseStatusNone,
		"Completed":      WorkflowCloseStatusCompleted,
		"Failed":         WorkflowCloseStatusFailed,
		"Canceled":       WorkflowCloseStatusCanceled,
		"Terminated":     WorkflowCloseStatusTerminated,
		"ContinuedAsNew": WorkflowCloseStatusContinuedAsNew,
		"TimedOut":       WorkflowCloseStatusTimedOut,
	}
)

func TestWorkflowExecutionInfoSetNextEventID(t *testing.T) {
	info := &WorkflowExecutionInfo{NextEventID: 1}
	info.SetNextEventID(2)

	assert.Equal(t, int64(2), info.NextEventID)
}

func TestWorkflowExecutionInfoIncreaseNextEventID(t *testing.T) {
	info := &WorkflowExecutionInfo{NextEventID: 1}

	info.IncreaseNextEventID()

	assert.Equal(t, int64(2), info.NextEventID)
}

func TestWorkflowExecutionInfoSetLastFirstEventID(t *testing.T) {
	info := &WorkflowExecutionInfo{LastFirstEventID: 1}

	info.SetLastFirstEventID(2)

	assert.Equal(t, int64(2), info.LastFirstEventID)
}

func TestWorkflowExecutionInfoIsRunning(t *testing.T) {
	tests := map[string]struct {
		state    int
		expected bool
	}{
		"Created": {
			WorkflowStateCreated, true,
		},
		"Running": {
			WorkflowStateRunning, true,
		},
		"Completed": {
			WorkflowStateCompleted, false,
		},
		"Zombie": {
			WorkflowStateZombie, false,
		},
		"Corrupted": {
			WorkflowStateCorrupted, false,
		},
		"Preparing": {
			WorkflowStatePreparing, false,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			info := &WorkflowExecutionInfo{
				State: test.state,
			}
			assert.Equal(t, test.expected, info.IsRunning())
		})
	}
}

type transitionChecker func(executionInfo *WorkflowExecutionInfo, toState, closeStatus int) bool
type stateChecker func(executionInfo *WorkflowExecutionInfo, closeStatus int) bool
type testCase struct {
	name       string
	fromState  int
	fromStatus int
	toState    int
	toStatus   int
}

func TestWorkflowExecutionInfoUpdateWorkflowStateCloseStatus(t *testing.T) {
	expectedTransitions := map[int]transitionChecker{
		WorkflowStateVoid: toAnyState(),
		WorkflowStateCreated: toStates(map[int]stateChecker{
			WorkflowStateCreated:   withCloseStatusNone(),
			WorkflowStateRunning:   withCloseStatusNone(),
			WorkflowStateCompleted: withCloseStatus(WorkflowCloseStatusTerminated, WorkflowCloseStatusTimedOut, WorkflowCloseStatusContinuedAsNew),
			WorkflowStateZombie:    withCloseStatusNone(),
			WorkflowStatePreparing: withCloseStatusNone(),
		}),
		WorkflowStateRunning: toStates(map[int]stateChecker{
			WorkflowStateRunning:   withCloseStatusNone(),
			WorkflowStateCompleted: withAnyCloseStatusExcept(WorkflowCloseStatusNone),
			WorkflowStateZombie:    withCloseStatusNone(),
			WorkflowStatePreparing: withCloseStatusNone(),
		}),
		WorkflowStateCompleted: toStates(map[int]stateChecker{
			WorkflowStateCompleted: func(executionInfo *WorkflowExecutionInfo, closeStatus int) bool {
				return executionInfo.CloseStatus == closeStatus
			},
		}),
		WorkflowStateZombie: toStates(map[int]stateChecker{
			WorkflowStateCreated:   withCloseStatusNone(),
			WorkflowStateRunning:   withCloseStatusNone(),
			WorkflowStateCompleted: withAnyCloseStatusExcept(WorkflowCloseStatusNone),
			WorkflowStateZombie:    withAnyCloseStatusExcept(WorkflowCloseStatusNone),
			WorkflowStatePreparing: withAnyCloseStatusExcept(),
		}),
		WorkflowStatePreparing: toStates(map[int]stateChecker{
			WorkflowStatePreparing: withAnyCloseStatusExcept(),
			WorkflowStateZombie:    withAnyCloseStatusExcept(),
			WorkflowStateCreated:   withCloseStatusNone(),
			WorkflowStateRunning:   withCloseStatusNone(),
			WorkflowStateCompleted: withAnyCloseStatusExcept(WorkflowCloseStatusNone),
		}),
	}
	var testCases []*testCase

	// From every state with a CloseStatus of None to every state and close status
	for fromStateName, fromState := range workflowStates {
		for toStateName, toState := range workflowStates {
			for closeStatusName, closeStatus := range workflowCloseStatuses {
				testCases = append(testCases, &testCase{
					name:       fmt.Sprintf("(%s,None)->(%s,%s)", fromStateName, toStateName, closeStatusName),
					fromState:  fromState,
					fromStatus: WorkflowCloseStatusNone,
					toState:    toState,
					toStatus:   closeStatus,
				})
			}
		}
	}
	// Ensure Completed doesn't allow for changing the Status
	testCases = append(testCases, &testCase{
		name:       "(Completed,Failed)->(Completed, Completed)",
		fromState:  WorkflowStateCompleted,
		fromStatus: WorkflowCloseStatusFailed,
		toState:    WorkflowStateCompleted,
		toStatus:   WorkflowCloseStatusCompleted,
	})
	testCases = append(testCases, &testCase{
		name:       "(Completed,Completed)->(Completed,Completed)",
		fromState:  WorkflowStateCompleted,
		fromStatus: WorkflowCloseStatusCompleted,
		toState:    WorkflowStateCompleted,
		toStatus:   WorkflowCloseStatusCompleted,
	})
	// Invalid states
	testCases = append(testCases, &testCase{
		name:       "(Completed,Completed)->(?,Completed)",
		fromState:  WorkflowStateCompleted,
		fromStatus: WorkflowCloseStatusCompleted,
		toState:    100000,
		toStatus:   WorkflowCloseStatusCompleted,
	})
	testCases = append(testCases, &testCase{
		name:       "(?,Completed)->(Completed,Completed)",
		fromState:  100000,
		fromStatus: WorkflowCloseStatusCompleted,
		toState:    WorkflowStateCompleted,
		toStatus:   WorkflowCloseStatusCompleted,
	})

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			executionInfo := &WorkflowExecutionInfo{
				State:       test.fromState,
				CloseStatus: test.fromStatus,
			}

			var noErr bool
			if isValid, ok := expectedTransitions[test.fromState]; ok {
				noErr = isValid(executionInfo, test.toState, test.toStatus)
			} else {
				noErr = false
			}

			err := executionInfo.UpdateWorkflowStateCloseStatus(test.toState, test.toStatus)

			if noErr {
				assert.NoError(t, err)
				assert.Equal(t, test.toState, executionInfo.State)
				assert.Equal(t, test.toStatus, executionInfo.CloseStatus)
			} else {
				assert.Error(t, err)
			}

		})
	}
}

func toAnyState() transitionChecker {
	return func(workflowExecutionInfo *WorkflowExecutionInfo, toState, closeStatus int) bool {
		return true
	}
}

func toStates(validStates map[int]stateChecker) transitionChecker {
	return func(workflowExecutionInfo *WorkflowExecutionInfo, toState, closeStatus int) bool {
		if checker, ok := validStates[toState]; ok {
			return checker(workflowExecutionInfo, closeStatus)
		}
		return false
	}
}

func withCloseStatusNone() stateChecker {
	return withCloseStatus(WorkflowCloseStatusNone)
}

func withCloseStatus(closeStatuses ...int) stateChecker {
	return func(workflowExecutionInfo *WorkflowExecutionInfo, closeStatus int) bool {
		return contains(closeStatuses, closeStatus)
	}
}

func withAnyCloseStatusExcept(closeStatuses ...int) stateChecker {
	return func(workflowExecutionInfo *WorkflowExecutionInfo, closeStatus int) bool {
		for _, v := range closeStatuses {
			if v == closeStatus {
				return false
			}
		}
		return true
	}
}

func contains(slice []int, value int) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
