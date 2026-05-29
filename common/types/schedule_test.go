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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScheduleOverlapPolicy_UnmarshalText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want ScheduleOverlapPolicy
		err  bool
	}{
		{name: "invalid", text: "INVALID", want: ScheduleOverlapPolicyInvalid},
		{name: "skip_new", text: "SKIP_NEW", want: ScheduleOverlapPolicySkipNew},
		{name: "buffer", text: "BUFFER", want: ScheduleOverlapPolicyBuffer},
		{name: "concurrent", text: "CONCURRENT", want: ScheduleOverlapPolicyConcurrent},
		{name: "cancel_previous", text: "CANCEL_PREVIOUS", want: ScheduleOverlapPolicyCancelPrevious},
		{name: "terminate_previous", text: "TERMINATE_PREVIOUS", want: ScheduleOverlapPolicyTerminatePrevious},
		{name: "lowercase", text: "skip_new", want: ScheduleOverlapPolicySkipNew},
		{name: "numeric", text: "2", want: ScheduleOverlapPolicy(2)},
		{name: "unknown", text: "UNKNOWN", err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ScheduleOverlapPolicy
			err := got.UnmarshalText([]byte(tt.text))
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestScheduleOverlapPolicy_MarshalText(t *testing.T) {
	tests := []struct {
		name string
		val  ScheduleOverlapPolicy
		want string
	}{
		{name: "invalid", val: ScheduleOverlapPolicyInvalid, want: "INVALID"},
		{name: "skip_new", val: ScheduleOverlapPolicySkipNew, want: "SKIP_NEW"},
		{name: "buffer", val: ScheduleOverlapPolicyBuffer, want: "BUFFER"},
		{name: "concurrent", val: ScheduleOverlapPolicyConcurrent, want: "CONCURRENT"},
		{name: "cancel_previous", val: ScheduleOverlapPolicyCancelPrevious, want: "CANCEL_PREVIOUS"},
		{name: "terminate_previous", val: ScheduleOverlapPolicyTerminatePrevious, want: "TERMINATE_PREVIOUS"},
		{name: "unknown_numeric", val: ScheduleOverlapPolicy(99), want: "ScheduleOverlapPolicy(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.val.MarshalText()
			assert.NoError(t, err)
			assert.Equal(t, tt.want, string(b))
		})
	}
}

func TestScheduleOverlapPolicy_RoundTrip(t *testing.T) {
	for _, val := range []ScheduleOverlapPolicy{
		ScheduleOverlapPolicyInvalid,
		ScheduleOverlapPolicySkipNew,
		ScheduleOverlapPolicyBuffer,
		ScheduleOverlapPolicyConcurrent,
		ScheduleOverlapPolicyCancelPrevious,
		ScheduleOverlapPolicyTerminatePrevious,
	} {
		b, err := val.MarshalText()
		assert.NoError(t, err)
		var got ScheduleOverlapPolicy
		err = got.UnmarshalText(b)
		assert.NoError(t, err)
		assert.Equal(t, val, got)
	}
}

func TestScheduleCatchUpPolicy_UnmarshalText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want ScheduleCatchUpPolicy
		err  bool
	}{
		{name: "invalid", text: "INVALID", want: ScheduleCatchUpPolicyInvalid},
		{name: "skip", text: "SKIP", want: ScheduleCatchUpPolicySkip},
		{name: "one", text: "ONE", want: ScheduleCatchUpPolicyOne},
		{name: "all", text: "ALL", want: ScheduleCatchUpPolicyAll},
		{name: "lowercase", text: "all", want: ScheduleCatchUpPolicyAll},
		{name: "numeric", text: "1", want: ScheduleCatchUpPolicy(1)},
		{name: "unknown", text: "UNKNOWN", err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ScheduleCatchUpPolicy
			err := got.UnmarshalText([]byte(tt.text))
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestScheduleCatchUpPolicy_MarshalText(t *testing.T) {
	tests := []struct {
		name string
		val  ScheduleCatchUpPolicy
		want string
	}{
		{name: "invalid", val: ScheduleCatchUpPolicyInvalid, want: "INVALID"},
		{name: "skip", val: ScheduleCatchUpPolicySkip, want: "SKIP"},
		{name: "one", val: ScheduleCatchUpPolicyOne, want: "ONE"},
		{name: "all", val: ScheduleCatchUpPolicyAll, want: "ALL"},
		{name: "unknown_numeric", val: ScheduleCatchUpPolicy(99), want: "ScheduleCatchUpPolicy(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.val.MarshalText()
			assert.NoError(t, err)
			assert.Equal(t, tt.want, string(b))
		})
	}
}

func TestScheduleCatchUpPolicy_RoundTrip(t *testing.T) {
	for _, val := range []ScheduleCatchUpPolicy{
		ScheduleCatchUpPolicyInvalid,
		ScheduleCatchUpPolicySkip,
		ScheduleCatchUpPolicyOne,
		ScheduleCatchUpPolicyAll,
	} {
		b, err := val.MarshalText()
		assert.NoError(t, err)
		var got ScheduleCatchUpPolicy
		err = got.UnmarshalText(b)
		assert.NoError(t, err)
		assert.Equal(t, val, got)
	}
}

func TestScheduleOverlapPolicy_Ptr(t *testing.T) {
	val := ScheduleOverlapPolicyBuffer
	ptr := val.Ptr()
	assert.Equal(t, &val, ptr)
}

func TestScheduleCatchUpPolicy_Ptr(t *testing.T) {
	val := ScheduleCatchUpPolicyAll
	ptr := val.Ptr()
	assert.Equal(t, &val, ptr)
}
