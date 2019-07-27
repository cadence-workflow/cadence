// Copyright (c) 2019 Uber Technologies, Inc.
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

// Code generated by mockery v1.0.0. DO NOT EDIT.

package history

import context "context"
import mock "github.com/stretchr/testify/mock"

// mockNDCStateRebuilder is an autogenerated mock type for the nDCStateRebuilder type
type mockNDCStateRebuilder struct {
	mock.Mock
}

// prepareMutableState provides a mock function with given fields: ctx, branchIndex, incomingVersion
func (_m *mockNDCStateRebuilder) prepareMutableState(ctx context.Context, branchIndex int, incomingVersion int64) (mutableState, bool, error) {
	ret := _m.Called(ctx, branchIndex, incomingVersion)

	var r0 mutableState
	if rf, ok := ret.Get(0).(func(context.Context, int, int64) mutableState); ok {
		r0 = rf(ctx, branchIndex, incomingVersion)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(mutableState)
		}
	}

	var r1 bool
	if rf, ok := ret.Get(1).(func(context.Context, int, int64) bool); ok {
		r1 = rf(ctx, branchIndex, incomingVersion)
	} else {
		r1 = ret.Get(1).(bool)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(context.Context, int, int64) error); ok {
		r2 = rf(ctx, branchIndex, incomingVersion)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}
