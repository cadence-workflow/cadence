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

package timeout

import (
	"context"
	"time"
)

const (
	// AdminDefaultTimeout is the admin service default timeout used to make calls
	AdminDefaultTimeout = 10 * time.Second
	// AdminDefaultLargeTimeout is the admin service large default timeout used to make calls
	AdminDefaultLargeTimeout = time.Minute
	// FrontendDefaultTimeout is the frontend service default timeout used to make calls
	FrontendDefaultTimeout = 10 * time.Second
	// FrontendDefaultLongPollTimeout is the frontend service long poll default timeout used to make calls
	FrontendDefaultLongPollTimeout = time.Minute * 3
	// MatchingDefaultTimeout is the default timeout used to make calls
	MatchingDefaultTimeout = time.Minute
	// MatchingDefaultLongPollTimeout is the long poll default timeout used to make calls
	MatchingDefaultLongPollTimeout = time.Minute * 2
	// HistoryDefaultTimeout is the default timeout used to make calls
	HistoryDefaultTimeout = time.Second * 30
	// ShardDistributorDefaultTimeout is the default timeout used to make calls
	ShardDistributorDefaultTimeout = time.Second * 10
	// ShardDistributorExecutorDefaultTimeout is the default timeout used to make calls
	ShardDistributorExecutorDefaultTimeout = time.Second * 10
)

func createContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}
