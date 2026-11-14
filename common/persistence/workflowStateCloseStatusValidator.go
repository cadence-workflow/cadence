// Copyright (c) 2017 Uber Technologies, Inc.
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

package persistence

import (
	"fmt"

	"github.com/uber/cadence/common/types"
)

var (
	validWorkflowStates = map[int]struct{}{
		WorkflowStateCreated:   {},
		WorkflowStateRunning:   {},
		WorkflowStateCompleted: {},
		WorkflowStateZombie:    {},
		WorkflowStateCorrupted: {},
		WorkflowStatePreparing: {},
	}

	validWorkflowCloseStatuses = map[int]struct{}{
		WorkflowCloseStatusNone:           {},
		WorkflowCloseStatusCompleted:      {},
		WorkflowCloseStatusFailed:         {},
		WorkflowCloseStatusCanceled:       {},
		WorkflowCloseStatusTerminated:     {},
		WorkflowCloseStatusContinuedAsNew: {},
		WorkflowCloseStatusTimedOut:       {},
	}
)

// ValidateCreateWorkflowStateCloseStatus validate workflow state and close status
func ValidateCreateWorkflowStateCloseStatus(
	state int,
	closeStatus int,
) error {

	if err := validateWorkflowState(state); err != nil {
		return err
	}
	if err := validateWorkflowCloseStatus(closeStatus); err != nil {
		return err
	}

	// An import writes its imported events, including the close event, while the run is
	// Preparing, so any close status is valid.
	if state == WorkflowStatePreparing {
		return nil
	}

	// validate workflow state & close status
	if state == WorkflowStateCompleted || closeStatus != WorkflowCloseStatusNone {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("Create workflow with invalid state: %v or close status: %v",
				state, closeStatus),
		}
	}
	return nil
}

// ValidateUpdateWorkflowStateCloseStatus validate workflow state and close status
func ValidateUpdateWorkflowStateCloseStatus(
	state int,
	closeStatus int,
) error {

	if err := validateWorkflowState(state); err != nil {
		return err
	}
	if err := validateWorkflowCloseStatus(closeStatus); err != nil {
		return err
	}

	// An import writes its imported events, including the close event, while the run is
	// Preparing, so any close status is valid.
	if state == WorkflowStatePreparing {
		return nil
	}

	// validate workflow state & close status
	if closeStatus == WorkflowCloseStatusNone {
		if state == WorkflowStateCompleted {
			return &types.InternalServiceError{
				Message: fmt.Sprintf("Update workflow with invalid state: %v or close status: %v",
					state, closeStatus),
			}
		}
	} else {
		// WorkflowCloseStatusCompleted
		// WorkflowCloseStatusFailed
		// WorkflowCloseStatusCanceled
		// WorkflowCloseStatusTerminated
		// WorkflowCloseStatusContinuedAsNew
		// WorkflowCloseStatusTimedOut
		if state != WorkflowStateCompleted {
			return &types.InternalServiceError{
				Message: fmt.Sprintf("Update workflow with invalid state: %v or close status: %v",
					state, closeStatus),
			}
		}
	}
	return nil
}

// validateWorkflowState validate workflow state
func validateWorkflowState(
	state int,
) error {

	if _, ok := validWorkflowStates[state]; !ok {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("Invalid workflow state: %v", state),
		}
	}

	return nil
}

// validateWorkflowCloseStatus validate workflow close status
func validateWorkflowCloseStatus(
	closeStatus int,
) error {

	if _, ok := validWorkflowCloseStatuses[closeStatus]; !ok {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("Invalid workflow close status: %v", closeStatus),
		}
	}

	return nil
}

// ToInternalWorkflowExecutionCloseStatus convert persistence representation of close status to internal representation
func ToInternalWorkflowExecutionCloseStatus(
	closeStatus int,
) *types.WorkflowExecutionCloseStatus {

	switch closeStatus {
	case WorkflowCloseStatusNone:
		return nil
	case WorkflowCloseStatusCompleted:
		return types.WorkflowExecutionCloseStatusCompleted.Ptr()
	case WorkflowCloseStatusFailed:
		return types.WorkflowExecutionCloseStatusFailed.Ptr()
	case WorkflowCloseStatusCanceled:
		return types.WorkflowExecutionCloseStatusCanceled.Ptr()
	case WorkflowCloseStatusTerminated:
		return types.WorkflowExecutionCloseStatusTerminated.Ptr()
	case WorkflowCloseStatusContinuedAsNew:
		return types.WorkflowExecutionCloseStatusContinuedAsNew.Ptr()
	case WorkflowCloseStatusTimedOut:
		return types.WorkflowExecutionCloseStatusTimedOut.Ptr()
	default:
		panic("Invalid value for enum WorkflowExecutionCloseStatus")
	}
}

// FromInternalWorkflowExecutionCloseStatus convert internal representation of close status to persistence representation
func FromInternalWorkflowExecutionCloseStatus(
	closeStatus *types.WorkflowExecutionCloseStatus,
) int {
	if closeStatus == nil {
		return WorkflowCloseStatusNone
	}
	switch *closeStatus {
	case types.WorkflowExecutionCloseStatusCompleted:
		return WorkflowCloseStatusCompleted
	case types.WorkflowExecutionCloseStatusFailed:
		return WorkflowCloseStatusFailed
	case types.WorkflowExecutionCloseStatusCanceled:
		return WorkflowCloseStatusCanceled
	case types.WorkflowExecutionCloseStatusTerminated:
		return WorkflowCloseStatusTerminated
	case types.WorkflowExecutionCloseStatusContinuedAsNew:
		return WorkflowCloseStatusContinuedAsNew
	case types.WorkflowExecutionCloseStatusTimedOut:
		return WorkflowCloseStatusTimedOut
	default:
		panic("Invalid value for enum WorkflowExecutionCloseStatus")
	}
}
