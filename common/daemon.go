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

package common

import "context"

const (
	// used for background threads

	// DaemonStatusInitialized coroutine pool initialized
	DaemonStatusInitialized int32 = 0
	// DaemonStatusStarted coroutine pool started
	DaemonStatusStarted int32 = 1
	// DaemonStatusStopped coroutine pool stopped
	DaemonStatusStopped int32 = 2
)

type (
	// Daemon is the base interfaces implemented by
	// background tasks within Cadence
	//
	// Deprecated: Use DaemonV2 instead for context-aware lifecycle management
	Daemon interface {
		Start()
		Stop()
	}

	// DaemonV2 is the context-aware version of Daemon
	// for background tasks that need graceful shutdown coordination
	DaemonV2 interface {
		Start(context.Context) error
		Stop(context.Context) error
	}
)
