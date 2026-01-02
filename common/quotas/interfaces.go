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

//go:generate mockgen -package=$GOPACKAGE -destination=limiter_mock.go github.com/uber/cadence/common/quotas Limiter

package quotas

import (
	"context"

	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
)

// RPSFunc returns a float64 as the RPS
type RPSFunc func() float64

// RPSKeyFunc returns a float64 as the RPS for the given key
type RPSKeyFunc func(key string) float64

// Info corresponds to information required to determine rate limits
type Info struct {
	Domain string
}

// Limiter corresponds to basic rate limiting functionality.
//
// TODO: This can likely be replaced with clock.Ratelimiter, now that it exists,
// but it is being left as a read-only mirror for now as only these methods are
// currently needed in areas that currently use this Limiter.
type Limiter interface {
	// Allow attempts to allow a request to go through. The method returns
	// immediately with a true or false indicating if the request can make
	// progress
	Allow() bool

	// Wait waits till the deadline for a rate limit token to allow the request
	// to go through.
	Wait(ctx context.Context) error

	// Reserve reserves a rate limit token
	Reserve() clock.Reservation

	// Limit returns the current configured ratelimit.
	//
	// If this Limiter wraps multiple values, this is generally the "most relevant" one,
	// i.e. the one that is most likely to apply to the next request
	Limit() rate.Limit
}

// Policy corresponds to a quota policy. A policy allows implementing layered
// and more complex rate limiting functionality.
type Policy interface {
	// Allow attempts to allow a request to go through. The method returns
	// immediately with a true or false indicating if the request can make
	// progress
	Allow(info Info) bool

	// Wait waits up till the context deadline for a rate limit token to allow
	// the request to go through. Returns nil if request is allowed.
	Wait(ctx context.Context, info Info) error
}
