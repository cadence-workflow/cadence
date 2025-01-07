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

package clock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type (
	// Ratelimiter is a test-friendly version of [golang.org/x/time/rate.Limiter],
	// which can be backed by any TimeSource.  The changes summarize as:
	//   - Time is never allowed to rewind, and does not advance when canceling,
	//     fixing core issues with [golang.org/x/time/rate.Limiter] and greatly
	//     improving the chances of successfully canceling a reserved token in our
	//     usage (from "almost literally never" to "likely 99%+").
	//   - Reservations are simplified and do not allow waiting, but are MUCH more
	//     likely to return tokens when canceled or not allowed.
	//   - All MethodAt APIs have been excluded as we do not use them, and they would
	//     have to bypass the contained TimeSource, which seems error-prone.
	//   - All `MethodN` APIs (specifically the ">1 event" parts) have been excluded
	//     because we do not use them, but they seem fairly easy to add if needed later.
	Ratelimiter interface {
		// Allow returns true if an event can be performed now, i.e. there
		// are burst-able tokens available to use.
		//
		// To perform events at a steady rate, use Wait instead.
		Allow() bool
		// Burst returns the maximum burst size allowed by this Ratelimiter.
		// A Burst of zero returns false from Allow, unless its Limit is infinity.
		//
		// To see the number of burstable tokens available now, use Tokens.
		Burst() int
		// Limit returns the maximum overall rate of events that are allowed.
		Limit() rate.Limit
		// Reserve claims a single Allow() call that can be canceled if not used.
		// Check Reservation.Allow() to see if the event is allowed.
		//
		// This Reservation does not support waiting, and needs to have
		// Reservation.Used(true/false) called as soon as possible to have the
		// best chance of returning its unused token.
		Reserve() Reservation
		// SetBurst sets the Burst value
		SetBurst(newBurst int)
		// SetLimit sets the Limit value
		SetLimit(newLimit rate.Limit)
		// Tokens returns the number of immediately-allowable events when called.
		// Values >= 1 will lead to Allow returning true.
		//
		// These values are NOT able to guarantee that a later call will succeed
		// or fail, as other calls to Allow or Reserve may occur before you are
		// able to act on the returned value.
		// It is essentially only suitable for monitoring or test use, and calling
		// it may lead to failing to cancel reserved tokens (as it advances time).
		Tokens() float64
		// Wait blocks until one token is available (and consumes it), or it returns
		// an error and does not consume any tokens if:
		//   - the context is canceled
		//   - a token will never become available (burst == 0)
		//   - a token is not expected to become available in time
		//     (short deadline and/or many pending reservations or waiters)
		//
		// If an error is returned, regardless of the reason, you may NOT proceed
		// with the limited event you were waiting for.
		Wait(ctx context.Context) error
	}

	// Reservation is a simplified and test-friendly version of [golang.org/x/time/rate.Reservation]
	// that only allows checking if it is successful or not, and (possibly) returning unused tokens.
	//
	// This Reservation MUST have Allow() checked and marked as Used(true/false)
	// as soon as possible, or an unused token is unlikely to be returned.
	//
	// Recommended use is either by using defer:
	//
	// 	func allow() (allowed bool) {
	//     	r := ratelimiter.Reserve()
	// 		// defer in a closure to read the most-recent value, not the value when deferred
	// 		defer func() { r.Used(allowed) }()
	// 		allowed = r.Allow()
	// 		if !allowed {
	//			return false // also marks the reservation as not-used
	// 		}
	// 		if somethingElseAllowsIt() {
	// 			// allow the event, and mark the reservation as used so the token is consumed
	//			return true
	// 		}
	// 		// cancel the reservation and possibly return the token
	// 		return false
	// 	}
	//
	// Or directly calling Used when appropriate:
	//
	// 	func allow() bool {
	//     	r := ratelimiter.Reserve()
	// 		allowed := r.Allow()
	// 		if !allowed {
	// 			r.Used(false) // mark the reservation as not-used
	//			return false
	// 		}
	// 		if somethingElseAllowsIt() {
	// 			// allow the event, and mark the reservation as used
	// 			r.Used(true)
	//			return true
	// 		}
	// 		r.Used(false) // mark the reservation as not-used, and restore the unused token
	// 		return false
	// 	}
	Reservation interface {
		// Allow returns true if a request should be allowed.
		//
		// This may be called at any time and it will return the same value each time,
		// but it becomes less correct as time passes, so you are expected to call it ~immediately.
		//
		// As soon as possible, also call Used(true/false) to consume / return the reserved token.
		//
		// This is equivalent to `OK() == true && DelayFrom(reservationTime) == 0`
		// with [rate.Reservation].
		Allow() bool

		// Used marks this Reservation as either used or not-used, and it must be called
		// once on every Reservation:
		//  - If true, the event is assumed to be allowed, and the Ratelimiter token
		//    will remain consumed.
		//  - If false, the Reservation will be rolled back, possibly restoring a Ratelimiter token.
		//    This is equivalent to calling Cancel() on a [rate.Reservation], but it is more likely
		//    to return the token.
		Used(wasUsed bool)
	}

	ratelimiter struct {
		// important design note: the lock MUST be held both while acquiring AND while handling
		// `timesource.Now()`.  otherwise time will not be monotonically handled due to waiting
		// on mutexes, and the *rate.Limiter internal time may go backwards and misbehave.
		timesource TimeSource
		latestNow  time.Time // updated on each call, never allowed to rewind
		limiter    *rate.Limiter
		mut        sync.Mutex
	}

	// a reservation that allows an immediate call, and can be canceled
	allowedReservation struct {
		res *rate.Reservation
		// reservedAt is used to cancel at the reservation time, restoring tokens
		// if (and only if) no other time-advancing calls have interleaved
		reservedAt time.Time
		// needs access to the parent-wrapped-ratelimiter to ensure time is not rewound
		limiter *ratelimiter
	}
	// a reservation that does NOT allow an immediate call.
	// the *rate.Reservation has already been canceled, so this type is trivial.
	deniedReservation struct{}
)

var (
	_ Ratelimiter = (*ratelimiter)(nil)
	_ Reservation = (*allowedReservation)(nil)
	_ Reservation = deniedReservation{}
)

// err constants to keep allocations low
var (
	// ErrCannotWait is used as the common base for two internal reasons why Ratelimiter.Wait call cannot
	// succeed, without directly exposing the reason except as an explanatory string.
	//
	// Callers should only ever care about ErrCannotWait to be resistant to racing behavior.
	// Or perhaps ideally: not care about the reason at all beyond "not a ctx.Err()".
	ErrCannotWait             = fmt.Errorf("ratelimiter.Wait() cannot be satisfied")
	errWaitLongerThanDeadline = fmt.Errorf("would wait longer than ctx deadline: %w", ErrCannotWait)
	errWaitZeroBurst          = fmt.Errorf("zero burst will never allow: %w", ErrCannotWait)
)

func NewRatelimiter(lim rate.Limit, burst int) Ratelimiter {
	return &ratelimiter{
		timesource: NewRealTimeSource(),
		limiter:    rate.NewLimiter(lim, burst),
		// intentionally zero, matches rate.Limiter and helps fill the bucket if it is changed before any use.
		latestNow: time.Time{},
	}
}

func NewMockRatelimiter(ts TimeSource, lim rate.Limit, burst int) Ratelimiter {
	return &ratelimiter{
		timesource: ts,
		limiter:    rate.NewLimiter(lim, burst),
		// intentionally zero, matches rate.Limiter and helps fill the bucket if it is changed before any use.
		latestNow: time.Time{},
	}
}

// lockNow updates the current "now" time, and ensures it never rolls back.
// This should be equivalent to setting `r.timesource.Now()`, outside of tests
// that abuse mocked time and Advance with negative values.
//
// The lock MUST be held until all time-accepting methods are called on the
// underlying rate.Limiter, as otherwise the internal "now" value may rewind
// and cause undefined behavior.
func (r *ratelimiter) lockNow() (now time.Time, unlock func()) {
	r.mut.Lock()
	newNow := r.timesource.Now() // caution: must be after acquiring the lock
	if newNow.After(r.latestNow) {
		// this should always be true because `time.Now()` is monotonic, and it is
		// always acquired inside the lock (so values are strictly ordered and
		// not arbitrarily delayed).
		//
		// that means e.g. lockReservedNow should not ever receive a value newer than
		// the Now we just acquired, so r.latestNow should never be newer either, so
		// this branch is always selected (unless time is equal).
		//
		// that said: it still shouldn't be allowed to rewind, so it's checked.
		r.latestNow = newNow
	}
	return r.latestNow, r.mut.Unlock
}

// lockReservedNow gets the correct "now" time to use, ensuring it never rolls back.
// This is intended to be used by reservations, to allow them to return claimed tokens
// after arbitrary waits, as long as no other calls have interleaved.
//
// The returned value MUST be used instead of the passed time: it may be the same or newer.
// The passed value may rewind the rate.Limiter's internal time record, and cause undefined behavior.
//
// The lock MUST be held until all time-accepting methods are called on the
// underlying rate.Limiter, as otherwise the internal "now" value may rewind
// and cause undefined behavior.
func (r *ratelimiter) lockReservedNow(tryNow time.Time) (now time.Time, unlock func()) {
	r.mut.Lock()
	if tryNow.After(r.latestNow) {
		// this should never occur!
		//
		// reservations and their reserved-time are gathered behind a lock, and time is monotonic.
		// that means that a reserved-time should ALWAYS be the same or older than any other time
		// the ratelimiter sees -> its time would have to go backwards.
		//
		// that said, if this DOES happen, it must still maintain the newest time.
		// this should only occur in tests that move time backwards, unless there's
		// an implementation bug in the ratelimiter or Go's time.
		r.latestNow = tryNow
	}
	return r.latestNow, r.mut.Unlock
}

func (r *ratelimiter) Allow() bool {
	now, unlock := r.lockNow()
	defer unlock()
	return r.limiter.AllowN(now, 1)
}

func (r *ratelimiter) Reserve() Reservation {
	now, unlock := r.lockNow()
	defer unlock()
	res := r.limiter.ReserveN(now, 1)
	if res.OK() && res.DelayFrom(now) == 0 {
		// usable token, return a cancel-able handler in case it's not used
		return &allowedReservation{
			res:        res,
			reservedAt: now,
			limiter:    r,
		}
	}
	// unusable, immediately cancel it to get rid of the future-reservation
	// since we don't allow waiting for it.
	//
	// this always succeeds and returns the token because nothing is allowed to
	// interleave, unlike if it is done asynchronously, so it returns MANY more
	// tokens when contended.
	res.CancelAt(now)
	return deniedReservation{}
}

func (r *ratelimiter) Burst() int {
	// rate.Limiter.Burst does not currently advance time,
	// so this does not need to hold the ratelimiter lock.
	// if this changes, use lockNow.
	return r.limiter.Burst()
}

func (r *ratelimiter) Limit() rate.Limit {
	// rate.Limiter.Limit does not currently advance time,
	// so this does not need to hold the ratelimiter lock.
	// if this changes, use lockNow.
	return r.limiter.Limit()
}

func (r *ratelimiter) SetBurst(newBurst int) {
	r.mut.Lock()
	defer r.mut.Unlock()
	// setting burst/limit does not advance time, unlike the underlying limiter.
	//
	// this allows calling them in any order, and the next request
	// will fill the token bucket to match elapsed time.
	//
	// this prefers new burst/limit values over past values,
	// as they are assumed to be "better", and in particular ensures the first
	// time-advancing call fills with the full values (starting from 0 time, like
	// the underlying limiter does).
	r.limiter.SetBurstAt(r.latestNow, newBurst)
}

func (r *ratelimiter) SetLimit(newLimit rate.Limit) {
	r.mut.Lock()
	defer r.mut.Unlock()
	// setting burst/limit does not advance time, unlike the underlying limiter.
	//
	// this allows calling them in any order, and the next request
	// will fill the token bucket to match elapsed time.
	//
	// this prefers new burst/limit values over past values,
	// as they are assumed to be "better", and in particular ensures the first
	// time-advancing call fills with the full values (starting from 0 time, like
	// the underlying limiter does).
	r.limiter.SetLimitAt(r.latestNow, newLimit)
}

func (r *ratelimiter) Tokens() float64 {
	now, unlock := r.lockNow()
	defer unlock()
	return r.limiter.TokensAt(now)
}

func (r *ratelimiter) Wait(ctx context.Context) (err error) {
	if err := ctx.Err(); err != nil {
		// canceled contexts imply that the limited-work will not be done,
		// so do not allow any tokens to be consumed.
		// rate.Limiter also behaves this way in Wait.
		return err
	}

	now, unlock := r.lockNow()
	unlockOnce := doOnce(unlock)
	defer unlockOnce() // unlock if panic or returned early with no err

	res := r.limiter.ReserveN(now, 1)
	defer func() {
		if err != nil {
			// err return means "not allowed", so cancel the reservation.
			//
			// note that this makes a separate call to get the current time:
			// this is intentional, as time may have passed while waiting.
			//
			// if the cancellation happened before the time-to-act (the delay),
			// the reservation will still be successfully rolled back.

			// ensure we are unlocked, which will not have happened if an err
			// was returned immediately.
			unlockOnce()

			// (re)-acquire the latest now value.
			//
			// it should not be advanced to the "real" now, to improve the chance
			// that the token will be restored, but if there was any waiting
			// it's pretty likely that other calls occurred while unlocked and
			// the most-recently-observed time must be maintained.
			now, unlock := r.lockReservedNow(now)
			defer unlock()
			res.CancelAt(now)
		}
	}()

	if !res.OK() {
		// !OK can only mean "impossible to wait" with `ReserveN`.
		// Since there is no deadline passed to the limiter (the reservation
		// contains the delay), and we only ever request 1 token, this can only
		// mean a burst of 0 (and non-infinite rate.Limit).
		return errWaitZeroBurst
	}
	delay := res.DelayFrom(now)
	if delay == 0 {
		return nil // available token, allow it
	}
	if deadline, ok := ctx.Deadline(); ok && now.Add(delay).After(deadline) {
		return errWaitLongerThanDeadline // don't wait for a known failure
	}

	unlockOnce() // unlock before waiting

	// wait for cancellation or the waiter's turn.
	// re-acquiring Now() here is valid and may shorten the delay, but may add
	// more branches and isn't strictly necessary as sleeps aren't precise anyway.
	timer := r.timesource.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.Chan():
		return nil
	}
}

func (r *allowedReservation) Used(used bool) {
	if !used {
		now, unlock := r.limiter.lockReservedNow(r.reservedAt)
		// lock must be held while canceling, because it can affect the limiter's time
		defer unlock()

		// if no calls interleaved, this will be the same as the reservation time,
		// and the cancellation will be able to roll back an immediately-available call.
		//
		// otherwise, the ratelimiter's time has been advanced, and this will fail to
		// restore any tokens.
		r.res.CancelAt(now)
	}
	// if used, do nothing.
	//
	// this method largely exists so it can be mocked and asserted,
	// or customized by an implementation that tracks allowed/rejected metrics.
}

func (r *allowedReservation) Allow() bool {
	// this is equivalent to `r.res.OK() && r.res.DelayFrom(r.reservedAt) == 0`,
	// but this is known to be `true` when it is first reserved and `r.reservedAt`
	// does not ever change, so it can be skipped here.
	return true
}

func (r deniedReservation) Used(used bool) {
	// reservation already canceled, nothing to roll back
}

func (r deniedReservation) Allow() bool {
	return false
}

// a small sync.Once-alike for single-threaded use.
//
// Ratelimiter.Wait benchmarks perform measurably better with this vs sync.Once,
// though not by a large margin.
func doOnce(f func()) func() {
	called := false
	return func() {
		if called {
			return
		}
		called = true
		f()
	}
}
