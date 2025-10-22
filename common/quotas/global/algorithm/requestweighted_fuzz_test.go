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

package algorithm

import (
	"encoding/binary"
	"maps"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas/global/shared"
)

// Calls Update and HostWeights as many times as there is sufficient data,
// to make sure multiple updates do not lead to Infs or NaNs, because floating point is hard.
//
// This is best to let run for a very long time after making changes, as odd sequences may be necessary.
// E.g. it took nearly 30 minutes to hit a case where `elapsed` had an `INT_MIN` value from the data bytes,
// which failed earlier attempts to math.Abs it with `-val` because `-INT_MIN == INT_MIN`.
//
// Your *local* fuzz corpus will help re-running to find more interesting things more quickly,
// but a cleared cache will have to start over and may take a long time.
func FuzzMultiUpdate(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		accept, reject, elapsed, data, msg := ints(data)
		if msg != "" {
			t.Skip(msg)
		}
		var host, key string
		if len(data) > 1 {
			// single-char strings are probably best, some collision is a good thing.
			host, key = string(data[0]), string(data[1])
			// data = data[2:] // leaving the data in there is fine, it'll just become part of the accept/reject ints.
		} else {
			t.Skip("not enough data for keys")
		}

		l, obs := testlogger.NewObserved(t) // failed sanity checks will fail the fuzz test, as they use .Error
		i, err := New(metrics.NewNoopMetricsClient(), l, Config{
			// TODO: fuzz test with different weights too? though only some human-friendly values are likely, like 0.25..0.75
			// extremely high or low values are kinda-intentionally allowed to misbehave since they're really not rational to set,
			// but it might be a good exercise to make sure the math is reasonable even in those edge cases...
			NewDataWeight:  func(opts ...dynamicproperties.FilterOption) float64 { return 0.5 },
			UpdateInterval: func(opts ...dynamicproperties.FilterOption) time.Duration { return time.Second },
			DecayAfter:     func(opts ...dynamicproperties.FilterOption) time.Duration { return time.Hour },
			GcAfter:        func(opts ...dynamicproperties.FilterOption) time.Duration { return time.Hour },
		})
		if err != nil {
			t.Fatal(err)
		}

		// if it takes more than a couple seconds, fuzz considers it stuck.
		// 1000 seems too long, 100 is pretty quick, probably just lower if necessary.
		for iter := 0; iter < 100; iter++ {
			// send as many updates as we have data for
			t.Logf("accept=%d, reject=%d, elapsed=%d, host=%q, key=%q", accept, reject, elapsed, host, key)

			err = i.Update(UpdateParams{
				ID: Identity(host),
				Load: map[Limit]Requests{
					Limit(key): {
						Accepted: int(accept),
						Rejected: int(reject),
					},
				},
				Elapsed: time.Duration(int(elapsed)),
			})
			if err != nil {
				t.Fatal(err)
			}

			u := i.(*impl).usage

			// collect and sort the layers of map keys,
			// as determinism helps the fuzzing system choose branches better.
			keySet := make(map[Limit]struct{})
			identSet := make(map[Identity]struct{})
			for limitKey, hostUsage := range u {
				keySet[limitKey] = struct{}{}
				for ident := range hostUsage {
					identSet[ident] = struct{}{}
				}
			}
			if accept+reject > 0 && len(keySet) == 0 {
				t.Error("no identities")
			}
			if accept+reject > 0 && len(identSet) == 0 {
				t.Error("no keys")
			}

			keys := slices.Collect(maps.Keys(keySet))
			idents := slices.Collect(maps.Keys(identSet))
			slices.Sort(keys)
			slices.Sort(idents)

			// scan for NaNs and Infs in internal data
			for _, k := range keys {
				hu := u[k]
				for _, ident := range idents {
					us := hu[ident]
					if bad := fsane(us.accepted); bad != "" {
						t.Errorf("%v for accepted rps:%q, key:%q", bad, k, ident)
					}
					if bad := fsane(us.rejected); bad != "" {
						t.Errorf("%v for rejected rps:%q, key:%q", bad, k, ident)
					}
				}
			}

			// and get all hosts and check the weight calculations for all keys too
			for _, ident := range idents {
				res, err := i.HostUsage(ident, keys)
				if err != nil {
					t.Fatal(err)
				}
				if len(res) == 0 {
					t.Error("no results")
				}
				for _, k := range keys {
					r, ok := res[k]
					if !ok {
						// currently all requested keys are expected to be returned
						t.Errorf("key not found: %q", k)
					}
					if bad := fsane(r.Used); bad != "" {
						t.Error(bad, "usage")
					}
					if r.Used < 0 {
						t.Error("negative usage")
					}
					if bad := fsane(r.Weight); bad != "" {
						t.Error(bad, "weight")
					}
					if r.Weight < 0 {
						t.Error("negative weight")
					} else if r.Weight > 1 {
						t.Error("too much weight")
					}
				}
			}

			// check for error logs, as this would imply sanity check violations
			shared.AssertNoSanityCheckFailures(t, obs.TakeAll())

			// refresh for the next round
			accept, reject, elapsed, data, msg = ints(data)
			if msg != "" {
				break // not enough data for another round
			}
			if len(data) > 1 {
				host, key = string(data[0]), string(data[1])
				// data = data[2:] // leaving the data in there is fine
			} else {
				break // not enough data for another round
			}
		}
	})
}

// float sanity check because it's a lot to write out every time
func fsane[T ~float64](t T) string {
	if math.IsNaN(float64(t)) {
		return "NaN"
	}
	if math.IsInf(float64(t), 0) {
		return "Inf"
	}
	return ""
}

// helper to pull varints out of a pile of bytes.
// returns the unused portion of data,
// and returns a non-empty string if there was a problem
func ints(inData []byte) (accept, reject, elapsed int64, data []byte, err string) {
	data = inData
	accept, data, msg := getPositiveInt64(data)
	if msg != "" {
		return 0, 0, 0, nil, "not enough data"
	}
	reject, data, msg = getPositiveInt64(data)
	if msg != "" {
		return 0, 0, 0, nil, "not enough data"
	}
	elapsed, data, msg = getPositiveInt64(data)
	if msg != "" {
		return 0, 0, 0, nil, "not enough data"
	}
	if elapsed == 0 {
		return 0, 0, 0, nil, "zero elapsed time, cannot use"
	}
	return accept, reject, elapsed, data, ""
}

// helpers need helpers sometimes
func getPositiveInt64(data []byte) (int64, []byte, string) {
	val, read := binary.Varint(data)
	if read == 0 {
		return 0, nil, "not enough data"
	}
	if read > 0 {
		data = data[read:] // successfully read an int
	}
	if read < 0 {
		data = data[-read:] // overflowed and stopped reading part way
	}
	if val < 0 {
		val = -val
	}
	if val < 0 {
		// -INT_MIN == INT_MIN, so the above check may have done nothing.
		// so special case INT_MIN by rolling it over to INT_MAX.
		val--
	}
	// and last but not least: make sure it's below our "impossible" value when accept+reject are combined.
	// partly this ensures random fuzzed rps doesn't exceed it (which is common),
	// and partly it just asserts "we do not test irrational rates".
	val = val % ((guessImpossibleRps - 1) / 2)
	return val, data, ""
}

// not covered by the larger fuzz test because it pins some "reasonable" values.
func FuzzMissedUpdate(f *testing.F) {
	f.Fuzz(func(t *testing.T, decay, gc, age, rate int, weight float64) {
		if decay <= 0 || gc <= 0 || age <= 0 || rate <= 0 {
			t.Skip()
		}
		scalar := configSnapshot{
			weight:     weight - math.Floor(weight), // 0..1, idk about negatives but it doesn't seem to break either
			rate:       time.Duration(rate),
			decayAfter: time.Duration(decay),
			gcAfter:    time.Duration(gc),
		}.missedUpdateScalar(time.Duration(age))
		if scalar < 0 || scalar > 1 {
			t.Fail()
		}
		if bad := fsane(scalar); bad != "" {
			t.Error(bad, "scalar")
		}
	})
}
