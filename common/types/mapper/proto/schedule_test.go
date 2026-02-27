// Copyright (c) 2026 Uber Technologies, Inc.
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

package proto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/testdata"
)

func TestScheduleSpec(t *testing.T) {
	for _, item := range []*types.ScheduleSpec{nil, {}, &testdata.ScheduleSpec} {
		assert.Equal(t, item, ToScheduleSpec(FromScheduleSpec(item)))
	}
}

func TestStartWorkflowAction(t *testing.T) {
	for _, item := range []*types.StartWorkflowAction{nil, {}, &testdata.ScheduleStartWorkflowAction} {
		assert.Equal(t, item, ToStartWorkflowAction(FromStartWorkflowAction(item)))
	}
}

func TestScheduleAction(t *testing.T) {
	for _, item := range []*types.ScheduleAction{nil, {}, &testdata.ScheduleAction} {
		assert.Equal(t, item, ToScheduleAction(FromScheduleAction(item)))
	}
}

func TestSchedulePolicies(t *testing.T) {
	for _, item := range []*types.SchedulePolicies{nil, {}, &testdata.SchedulePolicies} {
		assert.Equal(t, item, ToSchedulePolicies(FromSchedulePolicies(item)))
	}
}
