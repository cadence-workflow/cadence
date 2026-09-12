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
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

const (
	// amount of time between each conceptual "tick" of the test's clocks, both real and fake.
	// sadly even 10ms has some excessive latency a few % of the time, particularly during the Wait check.
	fuzzGranularity = 100 * time.Millisecond

	// keep running tests until this amount of time has passed, or a failure has occurred.
	// this could be a static count of tests to run, but a target duration
	// is a bit less fiddly.  "many attempts" is the goal, not any specific number.
	//
	// alternatively, tests have a deadline, this could run until near that deadline.
	// but right now that's not fine-tuned per package, so it's quite long.
	fuzzDuration = 8 * time.Second // mostly stays under 10s, feels reasonable

	// debug-level output is very noisy on successful runs, so hide it by default.
	// using a custom seed will also set this to true.
	fuzzDebugLog   = false
	fuzzCustomSeed = 0 // override with non-zero seed to run just one test with that seed
)

func TestAgainstRealRatelimit(t *testing.T) {
	// make sure to skip this test when checking coverage, as coverage it generates is not stable.
	// t.Skip("skipped to check coverage")

	const (
		// number of [fuzzGranularity] events to trigger per round,
		// and also the number of [fuzzGranularity] periods before a token is added.
		//
		// each test takes at least fuzzGranularity*events*rounds, so keep it kinda small.
		events = 10
		// number of rounds of "N events" to try
		rounds = 2

		// less than events to give Wait a better chance of waiting,
		// but anything up to (events*rounds)/2 will have some effect.
		maxBurst = events / 2
	)

	// The mock ratelimiter should behave the same as a real ratelimiter in all cases.
	//
	// So fuzz test it: throw random calls at non-overlapping times that are a bit
	// spaced out (to prevent noise due to busy CPUs) and make sure all impls agree.
	//
	// If a test fails, please verify by hand with the failing seed.
	// Enabling debug logs can help show what's happening and what *should* be happening,
	// though there is a chance it's just due to CPU contention.
	t.Run("fuzz", func(t *testing.T) {
		deadline := time.Now().Add(fuzzDuration)
		for testnum := 0; !t.Failed() && time.Now().Before(deadline); testnum++ {
			if testnum > 0 && fuzzCustomSeed != 0 {
				// already ran the seeded test, stop early.
				break
			}
			t.Run(fmt.Sprintf("attempt %v", testnum), func(t *testing.T) {
				t.Cleanup(func() {
					if t.Failed() {
						/*
							Is this test becoming too slow or too noisy?

							The easiest "likely to actually work" fix is probably going to
							require detecting excessive lag, and retrying instead of failing.

							That could miss some real flaws if they are racy by nature, but
							seems like it might be good enough.
						*/
						t.Logf("---- CAUTION ----")
						t.Logf("these tests are randomized by design, a random failure may be a real flaw!")
						t.Logf("please replay with the failing seed (set `fuzzCustomSeed`) and check detailed output to see what the behavior should be.")
						t.Logf("also try setting `fuzzDebugLog` to false to true to show Tokens() progression, this can help explain many issues.")
						t.Logf("")
						t.Logf("if you are intentionally making changes, be sure to run a few hundred rounds to make sure your changes are stable,")
						t.Logf("and in particular make sure an 'Advancing mock time ...' event occurs and times match, as that is a bit rare")
						t.Logf("---- CAUTION ----")
					}
				})

				// parallel saves a fair bit of time but introduces a lot of CPU contention
				// and that leads to a moderate amount of flakiness.
				//
				// unfortunately not recommended here.
				// t.Parallel()

				seed := time.Now().UnixNano()
				if fuzzCustomSeed != 0 {
					seed = fuzzCustomSeed // override seed to test a specific scenario
				}
				rng := rand.New(rand.NewSource(seed))
				t.Logf("rand seed: %v", seed)
				round := makeRandomSchedule(t, rng, rounds, events)
				burst := rng.Intn(maxBurst) // zero is not very interesting, but allowed
				// waits do not usually need to pause and it's tough to assert randomly.
				// any waited periods are still asserted to be similar though.
				// the expected-wait case is tested separately below.
				noMinWait := time.Duration(0)
				assertRatelimitersBehaveSimilarly(t, burst, noMinWait, round)
			})
		}
	})

	// also check edge cases that random tests only rarely or never hit:
	t.Run("edge cases", func(t *testing.T) {
		// Wait at the end of a round until a token recovers at the start of the next.
		t.Run("wait must wait similarly", func(t *testing.T) {
			schedule := [][]string{
				{"a", "_", "_", "w"}, // consume the only token, then wait with 0.75 tokens.
				{"_", "_", "_", "_"}, // first slot must be _ to recover a token and unblock the wait.
			}
			logSchedule(t, schedule)
			minWait := fuzzGranularity / 2 // not waiting will fail this test
			burst := 1
			assertRatelimitersBehaveSimilarly(t, burst, minWait, schedule)
		})
		t.Run("canceled contexts do not consume tokens", func(t *testing.T) {
			actual := rate.NewLimiter(rate.Every(time.Second), 1)
			wrapped := NewRatelimiter(rate.Every(time.Second), 1)

			canceledCtx, cancel := context.WithCancel(context.Background())
			cancel()

			actualErr := actual.Wait(canceledCtx)
			actualTokens := actual.Tokens()

			wrappedErr := wrapped.Wait(canceledCtx)
			wrappedTokens := wrapped.Tokens()

			assert.ErrorIs(t, actualErr, context.Canceled)
			assert.ErrorIs(t, wrappedErr, context.Canceled)

			assert.Equal(t, 1.0, actualTokens, "rate.Limiter should still have a token available")
			assert.Equal(t, 1.0, wrappedTokens, "wrapped should still have a token available")
		})
		t.Run("cancel while waiting returns tokens", func(t *testing.T) {
			// expect to recover 0.1 token after sleep
			actual := rate.NewLimiter(rate.Every(10*fuzzGranularity), 1)
			wrapped := NewRatelimiter(rate.Every(10*fuzzGranularity), 1)

			actual.Allow()
			wrapped.Allow()

			t.Logf("tokens in real: %0.5f, actual: %0.5f", actual.Tokens(), wrapped.Tokens())
			assert.InDeltaf(t, 0, actual.Tokens(), 0.1, "rate.Limiter should have near zero tokens")
			assert.InDeltaf(t, 0, wrapped.Tokens(), 0.1, "wrapped should have near zero tokens")

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				time.Sleep(fuzzGranularity)
				cancel()
			}()

			var actualDur, wrappedDur time.Duration
			var actualErr, wrappedErr error

			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				start := time.Now()
				actualErr = actual.Wait(ctx)
				actualDur = time.Since(start).Round(time.Millisecond)
			}()
			go func() {
				defer wg.Done()
				start := time.Now()
				wrappedErr = wrapped.Wait(ctx)
				wrappedDur = time.Since(start).Round(time.Millisecond)
			}()
			wg.Wait()

			assert.ErrorIs(t, actualErr, context.Canceled)
			assert.ErrorIs(t, wrappedErr, context.Canceled)

			// in particular, this would go to -1 if the token was not successfully returned.
			// that can happen if canceling near the wait time elapsing, but that would take a full second in this test.
			t.Logf("tokens in real: %0.5f, actual: %0.5f", actual.Tokens(), wrapped.Tokens())
			assert.InDeltaf(t, 0.1, actual.Tokens(), 0.1, "rate.Limiter should have returned token when canceled, and the 1/10th sleep should remain")
			assert.InDeltaf(t, 0.1, wrapped.Tokens(), 0.1, "wrapped should have returned token when canceled, and the 1/10th sleep should remain")

			t.Logf("waiting times for real: %v, actual: %v", actualDur, wrappedDur)
			assert.InDeltaf(t, actualDur, fuzzGranularity, float64(fuzzGranularity/2), "rate.Limiter should have waited until canceled")
			assert.InDeltaf(t, wrappedDur, fuzzGranularity, float64(fuzzGranularity/2), "wrapped should have waited until canceled")
		})
	})

}

/*
Generate a random schedule for testing behavior.

Schedules look like:

	[r _ a w _ _ _ _ _ w]
	[_ c _ _ _ _ w _ _ _]

Which means that the first round will:
  - reserve()
  - do nothing
  - allow()
  - wait() (may extend to next event, so the next one is always "_")
  - ...

By the end of that first array it'll reach the end of the `rate.Every` time, and
may recover tokens during the next round's runtime.  So the second round will:
  - do nothing, just refresh .1 while waiting ==> will have 1.0 tokens regenerated
    since the test began, as it was reserved earlier (if non-zero burst).
    (this could be immediately consumed by last round's final wait)
  - reserve but cancel, restoring the token (if any), and maybe refresh .1 token
  - do nothing, maybe refresh .1 more token...
  - ... and try to wait on the 7th event.

Non-blank events do not overlap between rounds to avoid triggering a race
between the old consumed tokens being refreshed and a new call consuming them,
as these make for very flaky tests unless time is mocked.
*/
func makeRandomSchedule(t *testing.T, rng *rand.Rand, rounds, events int) [][]string {
	// generate some non-colliding "A == perform an Allow call" rounds.
	schedule := make([][]string, rounds) // using string so it prints nicely
	for i := 0; i < rounds; i++ {
		round := make([]string, events)
		for j := 0; j < events; j++ {
			round[j] = "_" // "do nothing" marker
		}
		schedule[i] = round
	}
	skipWait := false
	for i := 0; i < len(schedule[0]); i++ {
		if skipWait {
			skipWait = false
			continue
		}
		// pick one to set to true, or none (1 chance for none)
		set := rng.Intn(len(schedule) + 1)
		if set < len(schedule) {
			// 1/3rd of them are waits
			switch rng.Intn(4) {
			case 0:
				schedule[set][i] = "a" // allow
			case 1:
				schedule[set][i] = "r" // reserve
			case 2:
				schedule[set][i] = "c" // reserve but cancel
			case 3:
				schedule[set][i] = "w" // wait
				// waits can pause for an event and a half, so skip the next event to avoid overlapping
				skipWait = true
			default:
				panic("missing a case")
			}
		}
	}
	logSchedule(t, schedule)

	return schedule
}

func logSchedule(t *testing.T, schedule [][]string) {
	t.Log("round setup:")
	for _, round := range schedule {
		t.Logf("\t%v", round)
	}
}

func debugLogf(t *testing.T, format string, args ...interface{}) {
	// by default be silent, but log if checking a specific seed or force-enabled.
	if fuzzDebugLog || fuzzCustomSeed != 0 {
		t.Helper()
		t.Logf(format, args...)
	}
}

func assertRatelimitersBehaveSimilarly(t *testing.T, burst int, minWaitDuration time.Duration, schedule [][]string) {
	rounds, eventsPerRound := len(schedule), len(schedule[0])
	limit := rate.Every(fuzzGranularity * time.Duration(eventsPerRound)) // refresh after each full round
	t.Logf("limit: %v, burst: %v", fuzzGranularity, burst)

	initialNow := time.Now().Round(time.Millisecond) // easier to read when logging/debugging when rounded
	ts := NewMockedTimeSourceAt(initialNow)
	compressedTS := NewMockedTimeSourceAt(initialNow)

	actual := rate.NewLimiter(limit, burst)
	wrapped := NewRatelimiter(limit, burst)
	mocked := NewRateLimiterWithTimeSource(ts, limit, burst)
	compressed := NewRateLimiterWithTimeSource(compressedTS, limit, burst)

	// record "to be executed" closures on the time-compressed ratelimiter too,
	// so it can be checked at a sped up rate.
	// nil values mean "nothing to do".
	compressedReplay := make([][]func(t *testing.T), rounds)
	for i := range compressedReplay {
		compressedReplay[i] = make([]func(t *testing.T), eventsPerRound)
	}

	ticker := time.NewTicker(fuzzGranularity)
	defer ticker.Stop()
	for round := range schedule {
		for event := range schedule[round] {
			<-ticker.C
			ts.Advance(fuzzGranularity) // may also be advanced inside wait
			debugLogf(t, "Tokens before round, real: %0.2f, wrapped: %0.2f, mocked: %0.2f", actual.Tokens(), wrapped.Tokens(), mocked.Tokens())

			switch schedule[round][event] {
			case "a":
				// call Allow on everything
				a, w, m := actual.Allow(), wrapped.Allow(), mocked.Allow()
				assertAllowsAgree(t, "Allow", round, event, a, w, m, nil)
				compressedReplay[round][event] = func(t *testing.T) {
					c := compressed.Allow()
					assertAllowsAgree(t, "Allow", round, event, a, w, m, &c)
				}
			case "r":
				// call Reserve on everything, don't cancel any we acquired successfully
				now := time.Now()
				_a, _w, _m := actual.ReserveN(now, 1), wrapped.Reserve(), mocked.Reserve()
				a, w, m := _a.OK() && _a.DelayFrom(now) == 0, _w.Allow(), _m.Allow()

				// un-reserve any that were not successful, we don't leave them dangling.
				if !a {
					_a.CancelAt(now)
				}
				_w.Used(w)
				_m.Used(m)
				assertAllowsAgree(t, "Reserve", round, event, a, w, m, nil)
				compressedReplay[round][event] = func(t *testing.T) {
					_c := compressed.Reserve()
					c := _c.Allow()
					_c.Used(c)
					assertAllowsAgree(t, "Reserve", round, event, a, w, m, &c)
				}
			case "c":
				// call Reserve on everything, and cancel them all
				now := time.Now() // needed for the real ratelimiter to cancel successfully like the wrapper does
				_a, _w, _m := actual.ReserveN(now, 1), wrapped.Reserve(), mocked.Reserve()
				_a.CancelAt(now)
				_w.Used(false)
				_m.Used(false)
				t.Logf("round[%v][%v] Cancel:     canceled", round, event)
				compressedReplay[round][event] = func(t *testing.T) {
					compressed.Reserve().Used(false)
					t.Logf("round[%v][%v] Cancel:     canceled", round, event)
				}
			case "w":
				/*
					Try a brief Wait on everything.

					ctx should expire in the middle of the *next* event, so all three possibilities are exercised:
					  - sometimes unblocks immediately because a token is available
					  - sometimes waits because a token will become available soon
					  - sometimes unblocks immediately because a token will not become available in time

					you can see waits logged like this, they're somewhat rare though.  otherwise wait times are ~0s:
						regular:
							ratelimiter_test.go:402: Wait elapsed: 20ms, wrapped err: <nil>
							ratelimiter_test.go:639: Advancing mock time by 20ms
							ratelimiter_test.go:402: Wait elapsed: 20ms, mocked err: <nil>
							ratelimiter_test.go:402: Wait elapsed: 20ms, actual err: <nil>
							ratelimiter_test.go:655: round[0][9] Wait:       actual: true,  wrapped: true,  mocked: true,
							ratelimiter_test.go:455:             Wait time:  actual: 20ms,  wrapped: 20ms,  mocked: 20ms,
						compressed:
							ratelimiter_test.go:672: Advancing mock time by 20ms
							ratelimiter_test.go:402: Wait elapsed: 20ms, compressed err: <nil>
							ratelimiter_test.go:684: round[0][9] Wait:       actual: true,  wrapped: true,  mocked: true,  compressed: true
							ratelimiter_test.go:455:             Wait time:  actual: 20ms,  wrapped: 20ms,  mocked: 20ms,  compressed: 20ms
				*/
				timeout := fuzzGranularity + (fuzzGranularity / 2)
				started := time.Now() // intentionally gathered outside the goroutines, to reveal goroutine-starting lag
				ctx, cancel := context.WithTimeout(context.Background(), timeout)

				a, w, m := false, false, false
				var aLatency, wLatency, mLatency time.Duration
				var g errgroup.Group
				g.Go(func() error {
					_a := actual.Wait(ctx)
					aLatency = time.Since(started).Round(time.Millisecond)
					debugLogf(t, "Wait elapsed: %v, actual err: %v", aLatency, _a)
					a = _a == nil
					return nil
				})
				g.Go(func() error {
					_w := wrapped.Wait(ctx)
					wLatency = time.Since(started).Round(time.Millisecond)
					debugLogf(t, "Wait elapsed: %v, wrapped err: %v", wLatency, _w)
					w = _w == nil
					return nil
				})
				g.Go(func() error {
					// mocked time needs something to advance it, or it'll block until timeout...
					// but don't advance time if it returned immediately, the others won't wait either.
					done := make(chan struct{})
					go func() {
						select {
						case <-time.After(fuzzGranularity):
							t.Logf("Advancing mock time by %v", fuzzGranularity)
							ts.Advance(fuzzGranularity)
						case <-done:
							// do nothing, it is not waiting
						}
					}()

					_m := mocked.Wait(ctx)
					close(done)
					mLatency = time.Since(started).Round(time.Millisecond)
					debugLogf(t, "Wait elapsed: %v, mocked err: %v", mLatency, _m)
					m = _m == nil
					return nil
				})
				_ = g.Wait()

				assertAllowsAgree(t, "Wait", round, event, a, w, m, nil)
				assertLatenciesSimilar(t, minWaitDuration, timeout, aLatency, wLatency, mLatency, nil)
				compressedReplay[round][event] = func(t *testing.T) {
					// need a mocked-time context, or the real deadline will not match the mocked deadline
					ctx := &timesourceContext{
						ts:       compressedTS,
						deadline: compressedTS.Now().Add(fuzzGranularity + (fuzzGranularity / 2)),
					}

					done := make(chan struct{})
					go func() {
						// wait until blocked inside Wait
						compressedTS.BlockUntil(1)
						// sleep briefly to encourage close(done) if racing, or blocked on something else
						time.Sleep(fuzzGranularity / 10)
						select {
						case <-done:
							// do nothing, Wait did not block correctly.
							// this goroutine may leak if no blockers occur. it's "fine".
						default:
							// blocked inside Wait, advance time so it unblocks
							t.Logf("Advancing mock time by %v", fuzzGranularity)
							compressedTS.Advance(fuzzGranularity)
						}
					}()

					compressedStarted := compressedTS.Now()
					_c := compressed.Wait(ctx)
					close(done)
					cLatency := time.Since(started).Round(time.Millisecond)
					cLatency = compressedTS.Since(compressedStarted)
					debugLogf(t, "Wait elapsed: %v, compressed err: %v", cLatency, _c)
					c := _c == nil
					assertAllowsAgree(t, "Wait", round, event, a, w, m, &c)
					assertLatenciesSimilar(t, minWaitDuration, timeout, aLatency, wLatency, mLatency, &cLatency)
				}
				cancel()
			case "_":
				// do nothing
			default:
				panic("unhandled case: " + schedule[round][event])
			}

			debugLogf(t, "Tokens after round, real: %0.2f, wrapped: %0.2f, mocked: %0.2f", actual.Tokens(), wrapped.Tokens(), mocked.Tokens())
		}
	}
	t.Run("compressed time", func(t *testing.T) {
		// and now replay the compressed ratelimiter and make sure it matches too,
		// as time-compressed must behave the same as real-time.
		//
		// this is primarily intended to detect cases where real-time is accidentally used,
		// as ~zero time actually passes, which is quite different from the above tests.
		//
		// it's not perfect, but it does eventually notice such bugs, as you can see by
		// changing literally any timesource.Now() calls into time.Now() and running
		// tests a few times.
		// depending on the change it might not notice in most tests, but eventually a
		// problematic combination is triggered and can be replayed with the logged seed.
		for round := range schedule {
			for event := range schedule[round] {
				compressedTS.Advance(fuzzGranularity)
				debugLogf(t, "Tokens before compressed round: %0.2f", compressed.Tokens())
				replay := compressedReplay[round][event]
				if replay != nil {
					replay(t)
				}
				debugLogf(t, "Tokens after compressed round: %0.2f", compressed.Tokens())
			}
		}
	})
}

// broken out to consts to help keep padding the same, for easier visual scanning
const (
	similarBehaviorLogComponents  = "%-11s %-14s %-15s %-14s" // "{what:} {actual} {wrapped} {mocked}" with padding
	similarBehaviorLogNormalRound = "round[%v][%v] " + similarBehaviorLogComponents
)

func assertAllowsAgree(t *testing.T, what string, round, event int, actual, wrapped, mocked bool, compressed *bool) {
	t.Helper()
	sWhat := what + ":"
	sActual := fmt.Sprintf("actual: %v,", actual)
	sWrapped := fmt.Sprintf("wrapped: %v,", wrapped)
	sMocked := fmt.Sprintf("mocked: %v,", mocked)
	// format strings should share padding so output aligns and it's easy to read.
	// unfortunately only strings do this nicely.s
	if compressed != nil {
		sCompressed := "compressed: " + fmt.Sprint(*compressed)
		t.Logf(similarBehaviorLogNormalRound+" %s", round, event, sWhat, sActual, sWrapped, sMocked, sCompressed)
		assert.True(t, actual == wrapped && wrapped == mocked && mocked == *compressed, "ratelimiters disagree")
	} else {
		t.Logf(similarBehaviorLogNormalRound, round, event, sWhat, sActual, sWrapped, sMocked)
		assert.True(t, actual == wrapped && wrapped == mocked, "ratelimiters disagree")
	}
}

func assertLatenciesSimilar(t *testing.T, mustBeGreaterThan, mustBeLessThan, actual, wrapped, mocked time.Duration, compressed *time.Duration) {
	t.Helper()
	// none are currently expected to wait, but if they do, they must not wait the full timeout.
	// this also helps handle small cpu stutter, as some may wait 1ms or so.
	maxLatency := maxDur(actual, wrapped, mocked)
	minLatency := minDur(actual, wrapped, mocked)
	compressedString := ""
	if compressed != nil {
		maxLatency = maxDur(maxLatency, *compressed)
		minLatency = minDur(minLatency, *compressed)
		compressedString = fmt.Sprintf("compressed: %v", *compressed)
	}
	assert.GreaterOrEqualf(t, minLatency, mustBeGreaterThan, "minimum latency must be within bounds, else waits did not wait long enough")
	assert.LessOrEqualf(t, maxLatency, mustBeLessThan, "maximum latency must not exceed timeout, it implies wait did not unblock when it should")

	// log format tries to align with assertAllowsAgree
	indent := strings.Repeat(" ", len("round[1][1]"))
	sActual := fmt.Sprintf("actual: %v,", actual)
	sWrapped := fmt.Sprintf("wrapped: %v,", wrapped)
	sMocked := fmt.Sprintf("mocked: %v,", mocked)
	t.Logf("%s "+similarBehaviorLogComponents+" %s",
		indent, "Wait time:", sActual, sWrapped, sMocked, compressedString)

	// asserting is... tough.  long-ish noise latencies are somewhat common,
	// sometimes even exceeding 10ms to schedule a goroutine.
	//
	// if this becomes too noisy, just switch to logging, and check by hand
	// when making changes.
	if minLatency < fuzzGranularity/2 {
		// "no wait" test, assert that they are all under this fuzzGranularity
		assertNoWait := func(what string, wait time.Duration) {
			t.Helper()
			assert.LessOrEqual(t, wait, fuzzGranularity/2, "%v ratelimiter waited too long", what)
		}
		assertNoWait("actual", actual)
		assertNoWait("wrapped", wrapped)
		assertNoWait("mocked", mocked)
		if compressed != nil {
			assertNoWait("compressed", *compressed)
		}
	} else {
		// "wait" test, assert that they all waited, and none waited significantly longer
		assertWaited := func(what string, wait time.Duration) {
			t.Helper()
			assert.True(t,
				wait > fuzzGranularity/2 && wait < fuzzGranularity+(fuzzGranularity/2),
				"%v waited an incorrect amount of time, should be %v < %v <%v",
				what, fuzzGranularity/2, wait, fuzzGranularity+(fuzzGranularity/2))
		}
		assertWaited("actual", actual)
		assertWaited("wrapped", wrapped)
		assertWaited("mocked", mocked)
		if compressed != nil {
			assertWaited("compressed", *compressed)
		}
	}
}

func maxDur(d time.Duration, ds ...time.Duration) time.Duration {
	for _, tmp := range ds {
		if tmp > d {
			d = tmp
		}
	}
	return d
}
func minDur(d time.Duration, ds ...time.Duration) time.Duration {
	for _, tmp := range ds {
		if tmp < d {
			d = tmp
		}
	}
	return d
}
