package structured

import (
	"fmt"
	"math"
	"time"

	"github.com/uber-go/tally"
)

// Nearly all histograms should use pre-defined buckets here, to encourage consistency.
// Name them more by their intent than their exact ranges - if you need something outside the intent,
// make a new bucket.
//
// Extreme, rare cases should still create a new bucket here, not declare one in-line elsewhere.
//
// Partly this helps us document our ranges, and partly it ensures that any customized emitter
// can easily detect buckets with an `==` comparison, and can choose to use a different one
// if needed (e.g. to reduce scale if it is too costly to support).
var (
	// Default1ms10m is our "default" set of buckets, targeting 1ms through 100s,
	// and is "rounded up" slightly to reach 80 buckets == 16 minutes.  (100s needs 68 buckets)
	//
	// If you need sub-millisecond precision, or substantially longer durations than about 1 minute,
	// consider using a different histogram.
	Default1ms10m = MakeExponentialHistogram(time.Millisecond, 2, func(last time.Duration, length int) bool {
		return last >= 100*time.Second && length == 80
	})

	// Default1ms24h covers things like activity latency, where "longer than 1 day" is not particularly worth being
	// precise about.  If they care about more precise / longer-range values, they can emit them in their code.
	Default1ms24h = MakeExponentialHistogram(time.Millisecond, 2, func(last time.Duration, length int) bool {
		// 24h leads to 108 buckets, 32h gives 112 which divides better
		return last >= 24*time.Hour && length == 112
	})

	// UpTo10kInts is a histogram for small counters, like "how many replication tasks did we receive".
	// This targets 1 to 10,000.
	UpTo10kInts = MakeExponentialHistogram(1, 2, func(last time.Duration, length int) bool {
		return last >= 10000
	})
)

// MakeExponentialHistogram is a replacement for tally.MustMakeExponentialDurationBuckets,
// tailored to make "construct a range" or "ensure N buckets" simpler for OTEL-compatible exponential histograms,
// and ensures values are as precise as possible (preventing display-space-wasteful numbers like 3.99996ms),
// and that all histograms start with a 0 value (else erroneous negative values are impossible to notice).
//
// Start time must be positive, scale must be 0..3 (inclusive), and the loop MUST stop within 320 or fewer steps,
// to prevent extremely-costly histograms.
// Even 320 is far too many tbh, target 160 for very-high-value data, and generally around 80 or less (ideally a value
// that is divisible by 2 many times).
//
// The stop callback will be given the current largest value and the number of values generated so far.
// This excludes the zero value (you should ignore it anyway), so you can stop at your target value,
// or == a highly-divisible-by-2 number (do not go over).
// The buckets produced may be padded further to reach a "full" power-of-2 row, as this simplifies math elsewhere
// and costs very little compared to the rest of the histogram.
//
// For all values produced, please add a test to print the concrete values, and record the length and maximum time
// so they can be quickly checked when reading.
func MakeExponentialHistogram(start time.Duration, scale int, stop func(last time.Duration, length int) bool) tally.DurationBuckets {
	if start <= 0 {
		panic(fmt.Sprintf("start must be greater than 0 or it will not grow exponentially, got %v", start))
	}
	if scale < 0 || scale > 3 {
		// anything outside this range is currently not expected and probably a mistake
		panic(fmt.Sprintf("scale must be between 0 (grows by *2) and 3 (grows by *2^1/8), got scale: %v", scale))
	}
	buckets := tally.DurationBuckets{
		time.Duration(0), // else "too low" and "negative" are impossible to tell apart.
		// ^ note this must be excluded from calculations below, hence -1 everywhere.
	}
	for {
		if len(buckets) > 320 {
			panic(fmt.Sprintf("over 320 buckets is too many, choose a smaller range or smaller scale.  "+
				"started at: %v, scale: %v, last value: %v",
				start, scale, buckets[len(buckets)-1]))
		}
		buckets = append(buckets, nextBucket(start, len(buckets)-1, scale))

		// stop when requested.
		if stop(buckets[len(buckets)-1], len(buckets)-1) {
			break
		}
	}

	// fill in as many buckets as are necessary to make a full "row", i.e. just
	// before the next power of 2 from the original value.
	// this ensures simple and accurate sub-setting math exists for at least
	// all 0..3-scale metrics.
	powerOfTwoWidth := int(math.Pow(2, float64(scale))) // num of buckets needed to double a value
	for (len(buckets)-1)%powerOfTwoWidth != 0 {
		buckets = append(buckets, nextBucket(start, len(buckets)-1, scale))
	}
	return buckets
}

func nextBucket(start time.Duration, num int, scale int) time.Duration {
	// calculating it from `start` each time reduces floating point error, ensuring "clean" multiples
	// at every power of 2 (and possibly others), instead of e.g. "1ms ... 1.9999994ms" which occurs
	// if you try to build from the previous value each time.
	return time.Duration(
		float64(start) *
			math.Pow(2, float64(num)/math.Pow(2, float64(scale))))
}
