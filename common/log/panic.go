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

package log

import (
	"fmt"
	"runtime/debug"

	"github.com/uber/cadence/common/log/tag"
)

// CapturePanic is used to capture panic, it will log the panic and also return the error through pointer.
// If the panic value is not error then a default error is returned
// We have to use pointer is because in golang: "recover return nil if was not called directly by a deferred function."
// And we have to set the returned error otherwise our handler will return nil as error which is incorrect
// errPanic MUST be the result from calling recover, which MUST be done in a single level deep
// deferred function. The usual way of calling this is:
// - defer func() { log.CapturePanic(recover(), logger, &err) }()
func CapturePanic(errPanic interface{}, logger Logger, retError *error) {
	if errPanic != nil {
		err, ok := errPanic.(error)
		if !ok {
			err = fmt.Errorf("panic object is not error: %#v", errPanic)
		}

		st := string(debug.Stack())

		// This function is called in deferred block and is all over the place.
		// We want the log to point to the line of panic, not this line, or stack of the defer function.
		logger.Helper().Helper().Error("Panic is captured", tag.SysStackTrace(st), tag.Error(err))

		if retError != nil {
			*retError = err
		}
	}
}
