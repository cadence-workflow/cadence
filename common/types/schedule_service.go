// Copyright (c) 2024 Uber Technologies, Inc.
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

package types

import "time"

// --- Request/Response Types for Schedule APIs ---

// ScheduleListEntry represents a single schedule in a list response.
type ScheduleListEntry struct {
	ScheduleID     string         `json:"scheduleId,omitempty"`
	WorkflowType   *WorkflowType  `json:"workflowType,omitempty"`
	State          *ScheduleState `json:"state,omitempty"`
	CronExpression string         `json:"cronExpression,omitempty"`
}

func (v *ScheduleListEntry) GetScheduleID() (o string) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

func (v *ScheduleListEntry) GetWorkflowType() *WorkflowType {
	if v != nil {
		return v.WorkflowType
	}
	return nil
}

func (v *ScheduleListEntry) GetState() *ScheduleState {
	if v != nil {
		return v.State
	}
	return nil
}

func (v *ScheduleListEntry) GetCronExpression() (o string) {
	if v != nil {
		return v.CronExpression
	}
	return
}

// CreateScheduleRequest is the request to create a new schedule.
type CreateScheduleRequest struct {
	Domain           string            `json:"domain,omitempty"`
	ScheduleID       string            `json:"scheduleId,omitempty"`
	Spec             *ScheduleSpec     `json:"spec,omitempty"`
	Action           *ScheduleAction   `json:"action,omitempty"`
	Policies         *SchedulePolicies `json:"policies,omitempty"`
	Memo             *Memo             `json:"-"` // Filtering PII
	SearchAttributes *SearchAttributes `json:"-"` // Filtering PII
}

func (v *CreateScheduleRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *CreateScheduleRequest) GetScheduleID() (o string) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

func (v *CreateScheduleRequest) GetSpec() *ScheduleSpec {
	if v != nil {
		return v.Spec
	}
	return nil
}

func (v *CreateScheduleRequest) GetAction() *ScheduleAction {
	if v != nil {
		return v.Action
	}
	return nil
}

func (v *CreateScheduleRequest) GetPolicies() *SchedulePolicies {
	if v != nil {
		return v.Policies
	}
	return nil
}

func (v *CreateScheduleRequest) GetMemo() *Memo {
	if v != nil {
		return v.Memo
	}
	return nil
}

func (v *CreateScheduleRequest) GetSearchAttributes() *SearchAttributes {
	if v != nil {
		return v.SearchAttributes
	}
	return nil
}

// CreateScheduleResponse is the response for creating a schedule.
type CreateScheduleResponse struct{}

// DescribeScheduleRequest is the request to describe a schedule.
type DescribeScheduleRequest struct {
	Domain     string `json:"domain,omitempty"`
	ScheduleID string `json:"scheduleId,omitempty"`
}

func (v *DescribeScheduleRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *DescribeScheduleRequest) GetScheduleID() (o string) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

// DescribeScheduleResponse is the response for describing a schedule.
type DescribeScheduleResponse struct {
	Spec             *ScheduleSpec     `json:"spec,omitempty"`
	Action           *ScheduleAction   `json:"action,omitempty"`
	Policies         *SchedulePolicies `json:"policies,omitempty"`
	State            *ScheduleState    `json:"state,omitempty"`
	Info             *ScheduleInfo     `json:"info,omitempty"`
	Memo             *Memo             `json:"-"` // Filtering PII
	SearchAttributes *SearchAttributes `json:"-"` // Filtering PII
}

func (v *DescribeScheduleResponse) GetSpec() *ScheduleSpec {
	if v != nil {
		return v.Spec
	}
	return nil
}

func (v *DescribeScheduleResponse) GetAction() *ScheduleAction {
	if v != nil {
		return v.Action
	}
	return nil
}

func (v *DescribeScheduleResponse) GetPolicies() *SchedulePolicies {
	if v != nil {
		return v.Policies
	}
	return nil
}

func (v *DescribeScheduleResponse) GetState() *ScheduleState {
	if v != nil {
		return v.State
	}
	return nil
}

func (v *DescribeScheduleResponse) GetInfo() *ScheduleInfo {
	if v != nil {
		return v.Info
	}
	return nil
}

func (v *DescribeScheduleResponse) GetMemo() *Memo {
	if v != nil {
		return v.Memo
	}
	return nil
}

func (v *DescribeScheduleResponse) GetSearchAttributes() *SearchAttributes {
	if v != nil {
		return v.SearchAttributes
	}
	return nil
}

// UpdateScheduleRequest is the request to update a schedule.
type UpdateScheduleRequest struct {
	Domain           string            `json:"domain,omitempty"`
	ScheduleID       string            `json:"scheduleId,omitempty"`
	Spec             *ScheduleSpec     `json:"spec,omitempty"`
	Action           *ScheduleAction   `json:"action,omitempty"`
	Policies         *SchedulePolicies `json:"policies,omitempty"`
	Memo             *Memo             `json:"-"` // Filtering PII
	SearchAttributes *SearchAttributes `json:"-"` // Filtering PII
}

func (v *UpdateScheduleRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *UpdateScheduleRequest) GetScheduleID() (o string) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

func (v *UpdateScheduleRequest) GetSpec() *ScheduleSpec {
	if v != nil {
		return v.Spec
	}
	return nil
}

func (v *UpdateScheduleRequest) GetAction() *ScheduleAction {
	if v != nil {
		return v.Action
	}
	return nil
}

func (v *UpdateScheduleRequest) GetPolicies() *SchedulePolicies {
	if v != nil {
		return v.Policies
	}
	return nil
}

func (v *UpdateScheduleRequest) GetMemo() *Memo {
	if v != nil {
		return v.Memo
	}
	return nil
}

func (v *UpdateScheduleRequest) GetSearchAttributes() *SearchAttributes {
	if v != nil {
		return v.SearchAttributes
	}
	return nil
}

// UpdateScheduleResponse is the response for updating a schedule.
type UpdateScheduleResponse struct{}

// DeleteScheduleRequest is the request to delete a schedule.
type DeleteScheduleRequest struct {
	Domain     string `json:"domain,omitempty"`
	ScheduleID string `json:"scheduleId,omitempty"`
}

func (v *DeleteScheduleRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *DeleteScheduleRequest) GetScheduleID() (o string) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

// DeleteScheduleResponse is the response for deleting a schedule.
type DeleteScheduleResponse struct{}

// PauseScheduleRequest is the request to pause a schedule.
type PauseScheduleRequest struct {
	Domain     string `json:"domain,omitempty"`
	ScheduleID string `json:"scheduleId,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

func (v *PauseScheduleRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *PauseScheduleRequest) GetScheduleID() (o string) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

func (v *PauseScheduleRequest) GetReason() (o string) {
	if v != nil {
		return v.Reason
	}
	return
}

// PauseScheduleResponse is the response for pausing a schedule.
type PauseScheduleResponse struct{}

// UnpauseScheduleRequest is the request to resume a paused schedule.
type UnpauseScheduleRequest struct {
	Domain        string                `json:"domain,omitempty"`
	ScheduleID    string                `json:"scheduleId,omitempty"`
	Reason        string                `json:"reason,omitempty"`
	CatchUpPolicy ScheduleCatchUpPolicy `json:"catchUpPolicy,omitempty"`
}

func (v *UnpauseScheduleRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *UnpauseScheduleRequest) GetScheduleID() (o string) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

func (v *UnpauseScheduleRequest) GetReason() (o string) {
	if v != nil {
		return v.Reason
	}
	return
}

func (v *UnpauseScheduleRequest) GetCatchUpPolicy() (o ScheduleCatchUpPolicy) {
	if v != nil {
		return v.CatchUpPolicy
	}
	return
}

// UnpauseScheduleResponse is the response for resuming a schedule.
type UnpauseScheduleResponse struct{}

// ListSchedulesRequest is the request to list schedules in a domain.
type ListSchedulesRequest struct {
	Domain        string `json:"domain,omitempty"`
	PageSize      int32  `json:"pageSize,omitempty"`
	NextPageToken []byte `json:"nextPageToken,omitempty"`
}

func (v *ListSchedulesRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *ListSchedulesRequest) GetPageSize() (o int32) {
	if v != nil {
		return v.PageSize
	}
	return
}

func (v *ListSchedulesRequest) GetNextPageToken() (o []byte) {
	if v != nil {
		return v.NextPageToken
	}
	return
}

// ListSchedulesResponse is the response for listing schedules.
type ListSchedulesResponse struct {
	Schedules     []*ScheduleListEntry `json:"schedules,omitempty"`
	NextPageToken []byte               `json:"nextPageToken,omitempty"`
}

func (v *ListSchedulesResponse) GetSchedules() (o []*ScheduleListEntry) {
	if v != nil {
		return v.Schedules
	}
	return
}

func (v *ListSchedulesResponse) GetNextPageToken() (o []byte) {
	if v != nil {
		return v.NextPageToken
	}
	return
}

// BackfillScheduleRequest is the request to trigger a backfill for a time range.
type BackfillScheduleRequest struct {
	Domain        string                `json:"domain,omitempty"`
	ScheduleID    string                `json:"scheduleId,omitempty"`
	StartTime     time.Time             `json:"startTime,omitempty"`
	EndTime       time.Time             `json:"endTime,omitempty"`
	OverlapPolicy ScheduleOverlapPolicy `json:"overlapPolicy,omitempty"`
	BackfillID    string                `json:"backfillId,omitempty"`
}

func (v *BackfillScheduleRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *BackfillScheduleRequest) GetScheduleID() (o string) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

func (v *BackfillScheduleRequest) GetStartTime() (o time.Time) {
	if v != nil {
		return v.StartTime
	}
	return
}

func (v *BackfillScheduleRequest) GetEndTime() (o time.Time) {
	if v != nil {
		return v.EndTime
	}
	return
}

func (v *BackfillScheduleRequest) GetOverlapPolicy() (o ScheduleOverlapPolicy) {
	if v != nil {
		return v.OverlapPolicy
	}
	return
}

func (v *BackfillScheduleRequest) GetBackfillID() (o string) {
	if v != nil {
		return v.BackfillID
	}
	return
}

// BackfillScheduleResponse is the response for triggering a backfill.
type BackfillScheduleResponse struct{}
