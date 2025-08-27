package structured

import (
	"fmt"
	"math"
	"slices"
	"strconv"
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
	// and is "rounded up" slightly to reach 80 buckets == 16 minutes (100s needs 68 buckets),
	// plus multi-minute exceptions are common enough to support for the small additional cost.
	//
	// If you need sub-millisecond precision, or substantially longer durations than about 1 minute,
	// consider using a different histogram.
	Default1ms10m = makeSubsettableHistogram(time.Millisecond, 2, func(last time.Duration, length int) bool {
		return last >= 10*time.Minute && length == 80
	})

	// High1ms24h covers things like activity latency, where "longer than 1 day" is not
	// particularly worth being precise about.
	//
	// This uses a lot of buckets, so it must be used with relatively-low cardinality elsewhere,
	// and/or consider dual emitting (Mid1ms24h or lower) so overviews can be queried efficiently.
	High1ms24h = makeSubsettableHistogram(time.Millisecond, 2, func(last time.Duration, length int) bool {
		// 24h leads to 108 buckets, raise to 112 for cleaner subsetting
		return last >= 24*time.Hour && length == 112
	})

	// Mid1ms24h is a one-scale-lower version of High1ms24h,
	// for use when we know it's too detailed to be worth emitting.
	//
	// This uses 57 buckets, half of High1ms24h's 113
	Mid1ms24h = High1ms24h.subsetTo(1)

	// Low1ms10s is for things that are usually very fast, like most individual database calls,
	// and is MUCH lower cardinality than Default1ms10m so it's more suitable for things like per-shard metrics.
	Low1ms10s = makeSubsettableHistogram(time.Millisecond, 1, func(last time.Duration, length int) bool {
		// 10s needs 26 buckets, raise to 32 for cleaner subsetting
		return last >= 10*time.Second && length == 32
	})

	// Mid1To32k is a histogram for small counters, like "how many replication tasks did we receive".
	// This targets 1 to 32k
	Mid1To32k = IntSubsettableHistogram(makeSubsettableHistogram(1, 2, func(last time.Duration, length int) bool {
		// 10k needs 56 buckets, raise to 64 for cleaner subsetting
		return last >= 10000 && length == 64
	}))
)

// SubsettableHistogram is a duration-based histogram that can be subset to a lower scale
// in a standardized, predictable way.  It is intentionally compatible with Prometheus and OTEL's
// "exponential histograms": https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponentialhistogram
//
// These histograms MUST always have a "_ns" suffix in their name to avoid confusion with timers.
type SubsettableHistogram struct {
	tallyBuckets tally.DurationBuckets

	scale int
}

// IntSubsettableHistogram is a non-duration-based integer-distribution histogram, otherwise identical
// to SubsettableHistogram but built as a separate type so you cannot pass the wrong one.
//
// These histograms MUST always have a "_counts" suffix in their name to avoid confusion with timers,
// or modify the Emitter to allow different suffixes if something else reads better.
type IntSubsettableHistogram SubsettableHistogram

// currently we have no apparent need for float-histograms,
// as all our value ranges go from >=1 to many thousands, where
// decimal precision is pointless and mostly just looks bad.
//
// if we ever need 0..1 precision in histograms, we can add them then.
// 	type FloatSubsettableHistogram struct {
// 		tally.ValueBuckets
// 		scale int
// 	}

func (s SubsettableHistogram) subsetTo(newScale int) SubsettableHistogram {
	if newScale >= s.scale {
		panic(fmt.Sprintf("scale %v is not less than the current scale %v", newScale, s.scale))
	}
	if newScale < 0 {
		panic(fmt.Sprintf("negative scales (%v == greater than *2 per step) are possible, but do not have tests yet", newScale))
	}
	dup := SubsettableHistogram{
		tallyBuckets: slices.Clone(s.tallyBuckets),
		scale:        s.scale,
	}
	// remove every other bucket per -1 scale
	for dup.scale > newScale {
		if (len(dup.tallyBuckets)-1)%2 != 0 {
			panic(fmt.Sprintf("cannot subset from scale %v to %v, %v-buckets is not divisible by 2", dup.scale, dup.scale-1, len(dup.tallyBuckets)-1))
		}
		half := make(tally.DurationBuckets, 0, ((len(dup.tallyBuckets)-1)/2)+1)
		half = append(half, dup.tallyBuckets[0]) // keep the zero value
		for i := 1; i < len(dup.tallyBuckets); i += 2 {
			half = append(half, dup.tallyBuckets[i]) // add first, third, etc
		}
		dup.tallyBuckets = half
		dup.scale--
	}
	return dup
}

func (i IntSubsettableHistogram) subsetTo(newScale int) IntSubsettableHistogram {
	return IntSubsettableHistogram(SubsettableHistogram(i).subsetTo(newScale))
}

// makeSubsettableHistogram is a replacement for tally.MustMakeExponentialDurationBuckets,
// tailored to make "construct a range" or "ensure N buckets" simpler for OTEL-compatible exponential histograms
// (i.e. https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponentialhistogram),
// ensures values are as precise as possible (preventing display-space-wasteful numbers like 3.99996ms),
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
// For all values produced, please add a test to print the concrete values, and record the length and maximum value
// so they can be quickly checked when reading.
func makeSubsettableHistogram(start time.Duration, scale int, stop func(last time.Duration, length int) bool) SubsettableHistogram {
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
	// this ensures subsetting keeps "round" numbers as long as possible.
	powerOfTwoWidth := int(math.Pow(2, float64(scale))) // num of buckets needed to double a value
	for (len(buckets)-1)%powerOfTwoWidth != 0 {
		buckets = append(buckets, nextBucket(start, len(buckets)-1, scale))
	}
	return SubsettableHistogram{
		tallyBuckets: buckets,
		scale:        scale,
	}
}

func nextBucket(start time.Duration, num int, scale int) time.Duration {
	// calculating it from `start` each time reduces floating point error, ensuring "clean" multiples
	// at every power of 2 (and possibly others), instead of e.g. "1ms ... 1.9999994ms" which occurs
	// if you try to build from the previous value each time.
	return time.Duration(
		float64(start) *
			math.Pow(2, float64(num)/math.Pow(2, float64(scale))))
}

func (s SubsettableHistogram) histScale() int                 { return s.scale }
func (s SubsettableHistogram) width() int                     { return int(math.Pow(2, float64(s.scale))) }
func (s SubsettableHistogram) len() int                       { return len(s.tallyBuckets) }
func (s SubsettableHistogram) start() time.Duration           { return s.tallyBuckets[1] }
func (s SubsettableHistogram) end() time.Duration             { return s.tallyBuckets[len(s.tallyBuckets)-1] }
func (s SubsettableHistogram) buckets() tally.DurationBuckets { return s.tallyBuckets }
func (s SubsettableHistogram) writeTags(metricName string, into map[string]string, tagCollisionComplaints Emitter) {
	writeHistogramTags(s, into, func(key string) {
		tagCollisionComplaints.Count(
			"error_histogram_tag_collision",
			1,
			TagsFromMap(into).With( // carry along existing tags in case it narrows down the cause
				"bad_key", key,
				"metric_name", metricName,
			))
	})
}
func (s SubsettableHistogram) print(to func(string, ...any)) {
	to("%v\n", s.tallyBuckets[0:1]) // zero value on its own row
	for rowStart := 1; rowStart < s.len(); rowStart += s.width() {
		to("%v\n", s.tallyBuckets[rowStart:rowStart+s.width()])
	}
}

func (i IntSubsettableHistogram) histScale() int       { return i.scale }
func (i IntSubsettableHistogram) width() int           { return int(math.Pow(2, float64(i.scale))) }
func (i IntSubsettableHistogram) len() int             { return len(i.tallyBuckets) }
func (i IntSubsettableHistogram) start() time.Duration { return i.tallyBuckets[1] }
func (i IntSubsettableHistogram) end() time.Duration {
	return i.tallyBuckets[len(i.tallyBuckets)-1]
}
func (i IntSubsettableHistogram) buckets() tally.DurationBuckets { return i.tallyBuckets }
func (i IntSubsettableHistogram) writeTags(metricName string, into map[string]string, tagCollisionComplaints Emitter) {
	writeHistogramTags(i, into, func(key string) {
		tagCollisionComplaints.Count(
			"error_int_histogram_tag_collision",
			1,
			TagsFromMap(into).With( // carry along existing tags in case it narrows down the cause
				"bad_key", key,
				"metric_name", metricName,
			))
	})
}
func (i IntSubsettableHistogram) print(to func(string, ...any)) {
	// fairly unreadable as duration-strings, so convert to int by hand
	to("[%d]\n", int(i.tallyBuckets[0])) // zero value on its own row
	for rowStart := 1; rowStart < i.len(); rowStart += i.width() {
		ints := make([]int, 0, i.width())
		for _, d := range i.tallyBuckets[rowStart : rowStart+i.width()] {
			ints = append(ints, int(d))
		}
		to("%v\n", ints)
	}
}

func writeHistogramTags[T any](h histogrammy[T], into map[string]string, tagCollision func(key string)) {
	if _, ok := into["histogram_start"]; ok {
		tagCollision("histogram_start")
	}
	if _, ok := into["histogram_end"]; ok {
		tagCollision("histogram_end")
	}
	if _, ok := into["histogram_scale"]; ok {
		tagCollision("histogram_scale")
	}
	// record the full range and scale of the histogram so it can be recreated from any individual metric.
	into["histogram_start"] = strconv.Itoa(int(h.start()))
	into["histogram_end"] = strconv.Itoa(int(h.end()))
	// include the scale, so we know how far away from the requested scale it is, when re-subsetting.
	into["histogram_scale"] = strconv.Itoa(h.histScale())
}

// internal utility/test methods, but could be exposed if there's a use for it
type histogrammy[T any] interface {
	histScale() int                 // exponential scale value.  0..3 inclusive.
	width() int                     // number of values per power of 2 == how wide to print each row.  1, 2, 4, or 8.
	len() int                       // number of buckets
	start() time.Duration           // first non-zero bucket
	end() time.Duration             // last bucket
	buckets() tally.DurationBuckets // access to all buckets
	subsetTo(newScale int) T        // generic so specific types can be returned
	writeTags(metricName string, into map[string]string, tagCollisionComplaints Emitter)

	print(to func(string, ...any)) // test-oriented printer
}
