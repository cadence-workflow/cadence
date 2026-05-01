// Copyright (c) 2017-2020 Uber Technologies, Inc.
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

package errors

import (
	"context"
	"errors"
	"math/rand"

	"github.com/uber/cadence/common/types"
)

var (
	// ErrFakeServiceBusy is a fake service busy error.
	ErrFakeServiceBusy = &types.ServiceBusyError{Message: "Fake Service Busy Error."}
	// ErrFakeInternalService is a fake internal service error.
	ErrFakeInternalService = &types.InternalServiceError{Message: "Fake Internal Service Error."}
	// ErrFakeTimeout is a fake timeout error.
	ErrFakeTimeout = context.DeadlineExceeded
	// ErrFakeUnhandled is a fake unhandled error.
	ErrFakeUnhandled = errors.New("fake unhandled error")
)

var (
	fakeErrors = []error{
		ErrFakeServiceBusy,
		ErrFakeInternalService,
		ErrFakeTimeout,
		ErrFakeUnhandled,
	}
)

// ShouldForwardCall determines if the call should be forward to the underlying
// client given the fake error generated
func ShouldForwardCall(
	err error,
) bool {
	if err == nil {
		return true
	}

	if err == ErrFakeTimeout || err == ErrFakeUnhandled {
		// forward the call with 50% chance
		return rand.Intn(2) == 0
	}

	return false
}

// GenerateFakeError generates a random fake error
func GenerateFakeError(
	errorRate float64,
) error {
	if rand.Float64() < errorRate {
		return fakeErrors[rand.Intn(len(fakeErrors))]
	}

	return nil
}
