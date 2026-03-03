package metrics

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/uber-go/tally"
)

// Nearly all histograms should use pre-defined buckets here, to encourage consistency.
//
// When creating a new one, make sure to read makeSubsettableHistogram for context, and choose
// values that communicate your *intent*, not the actual final value.  Add a test to show the
// resulting buckets, so reviewers can see just how much / how little you chose.
//
// Similarly, name them more by their intent than their exact ranges, because "it has just barely
// enough buckets" is almost always not *actually* enough when things misbehave.
// Choosing by intent also helps make adjusting these values safer if needed (e.g. subsetting or
// reducing the range), because we can be sure all intents match.
var (
	// Default1ms100s is our "default" set of buckets, targeting at least 1ms through 100s,
	// and is "rounded up" slightly to reach 80 buckets (100s needs 68 buckets),
	// plus multi-minute exceptions are common enough to support for the small additional cost
	// (this goes up to ~15m).
	//
	// That makes this *relatively* costly, but hopefully good enough for most metrics without
	// needing any careful thought.  If this proves incorrect, it will be down-scaled without
	// checking existing uses, so make sure to use something else if you require a certain level
	// of precision.
	//
	// If you need sub-millisecond precision, or substantially longer durations than about 1 minute,
	// consider using a different histogram.
	Default1ms100s = makeSubsettableHistogram(2, time.Millisecond, 100*time.Second, 80)

	// Low1ms100s is a half-resolution version of Default1ms100s, intended for use any time the default
	// is expected to be more costly than we feel comfortable with.  Or for automatic down-scaling
	// at runtime via dynamic config, if that's ever built.
	//
	// This uses only 40 buckets, so it's likely good enough for most purposes, e.g. database operations
	// that might have high cardinality but should not exceed about a minute even in exceptional cases.
	// Reducing this to "1ms to 10s" seems reasonable for some cases, but that still needs ~32 buckets,
	// which isn't a big savings and reduces our ability to notice abnormally slow events.
	Low1ms100s = Default1ms100s.subsetTo(1)

	// High1ms24h covers things like activity latency, where ranges are very large but
	// "longer than 1 day" is not particularly worth being precise about.
	//
	// This uses a lot of buckets, so it must be used with relatively-low cardinality elsewhere,
	// and/or consider dual emitting (Mid1ms24h or lower) so overviews can be queried efficiently.
	High1ms24h = makeSubsettableHistogram(2, time.Millisecond, 24*time.Hour, 112)

	// Mid1ms24h is a one-scale-lower version of High1ms24h,
	// for use when we know it's too detailed to be worth emitting.
	//
	// This uses 56 buckets, half of High1ms24h's 112
	Mid1ms24h = High1ms24h.subsetTo(1)

	// Mid1To16k is a histogram for small counters, like "how many replication tasks did we receive".
	//
	// This targets single-digits through ~10k with some buffer, and ends well below 1M.
	//
	// Note: we intentionally start at 8 (not 1). Starting at 1 causes duplicate bucket boundaries
	// due to float->duration truncation, which Prometheus rejects.
	Mid1To16k = IntSubsettableHistogram(makeSubsettableHistogram(2, 8, 16384, 64))
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
// These histograms MUST always have a "_counts" suffix in their name to avoid confusion with timers.
// (more suffixes will likely be allowed as needed)
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

// subsetTo takes an existing histogram and reduces its detail level.
//
// this can be replaced by simply creating a new histogram with the same args
// as the original but half the length, but it's added because: 1) it works too,
// and 2) to document how the process works, because we'll likely be doing this
// at query time to reduce the level of detail in small / over-crowded graphs.
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

	// compress every other bucket per -1 scale
	for dup.scale > newScale {
		if (len(dup.tallyBuckets)-2)%2 != 0 { // -2 for 0 and last-2^N
			panic(fmt.Sprintf(
				"cannot subset from scale %v to %v, %v-buckets is not divisible by 2",
				dup.scale, dup.scale-1, len(dup.tallyBuckets)-2))
		}
		if len(dup.tallyBuckets) <= 3 {
			// at 3 buckets, there's just 0, start, end.
			//
			// this is well past the point of being useful,
			// and it means we might try to slice `[1:0]` or similar.
			panic(fmt.Sprintf(
				"not enough buckets to subset from scale %d to %d, only have %d: %v",
				dup.scale, dup.scale-1, len(dup.tallyBuckets), dup.tallyBuckets))
		}

		// the first and last buckets will be kept, the rest will lose half per scale
		bucketsToCompress := dup.tallyBuckets[1 : len(dup.tallyBuckets)-2] // trim

		half := make(tally.DurationBuckets, 0, (len(bucketsToCompress)/2)+2)
		half = append(half, dup.tallyBuckets[0]) // keep the zero value intact
		for i := 0; i < len(bucketsToCompress); i += 2 {
			half = append(half, bucketsToCompress[i]) // keep the first, third, etc
		}
		half = append(half, dup.tallyBuckets[len(dup.tallyBuckets)-1]) // keep the last 2^N intact

		dup.tallyBuckets = half
		dup.scale--
	}
	return dup
}

func (i IntSubsettableHistogram) subsetTo(newScale int) IntSubsettableHistogram {
	return IntSubsettableHistogram(SubsettableHistogram(i).subsetTo(newScale))
}

func (s SubsettableHistogram) tags() map[string]string {
	return map[string]string{
		// use the duration string func, as "1ms" duration suffix is clearer than "1000000" for nanoseconds.
		// this also helps differentiate "tracks time" from "may be a general value distribution",
		// both visually when querying by hand and for any future automation (if needed).
		"histogram_start": s.start().String(),
		"histogram_end":   s.end().String(),
		"histogram_scale": strconv.Itoa(s.scale),
	}
}

func (i IntSubsettableHistogram) tags() map[string]string {
	return map[string]string{
		"histogram_start": strconv.Itoa(int(i.start())),
		"histogram_end":   strconv.Itoa(int(i.end())),
		"histogram_scale": strconv.Itoa(i.scale),
	}
}

// makeSubsettableHistogram is a replacement for tally.MustMakeExponentialDurationBuckets,
// tailored to make "construct a range" or "ensure N buckets" simpler for OTEL-compatible exponential histograms
// (i.e. https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponentialhistogram).
//
// It ensures values are as precise as possible (preventing display-space-wasteful numbers like 3.99996ms),
// that all histograms start with a 0 value (else erroneous negative values are impossible to notice),
// and avoids some bugs with low values (tally.MustMakeExponentialDurationBuckets misbehaves for small numbers,
// causing all buckets to == the minimum value).
//
// # To use
//
// Choose scale/start/end values that match your *intent*, i.e. the range you want to capture, and then choose
// a length that EXCEEDS that by at least 2x, ideally 4x-10x, and ends at a highly-divisible-by-2 value.
//
// Length should ideally be a highly-divisible-by-2 value, to keep subsetting "clean" as long as possible,
// i.e. the ranges of values in each bucket stays the same rather than having the second-to-last one cover
// a smaller range.
//
// This length will be padded internally to add both a zero value and a next-power-of-2 value, to help keep
// negative values identifiable, and keep the upper limit unchanged across all scales (as it cannot cleanly
// subset, because it holds an infinite range of values).
//
// The exact length / exceeding / etc does not matter (just write a test so it's documented and can be reviewed),
// it just serves to document your intent and to make sure that we have some head-room so we can understand how
// wrongly we guessed at our needs if it's exceeded during an incident of some kind.  We've failed at this
// multiple times in the past, it's worth paying a bit extra to be able to diagnose problems.
//
// For all histograms produced, add a test to print the concrete values, so they can be quickly checked when reading.
func makeSubsettableHistogram(scale int, start, end time.Duration, length int) SubsettableHistogram {
	if scale < 0 || scale > 3 {
		// anything outside this range is currently not expected and probably a mistake,
		// but any value is technically sound.
		panic(fmt.Sprintf("scale must be between 0 (grows by *2) and 3 (grows by *2^1/8), got scale: %v", scale))
	}
	if start <= 0 {
		panic(fmt.Sprintf("start must be greater than 0 or it will not grow exponentially, got %v", start))
	}
	if start >= end {
		panic(fmt.Sprintf("start must be less than end (%v < %v)", start, end))
	}
	if length < 12 || length > 160 {
		// 160 is probably higher than we should consider, but going further is currently risking much too high costs.
		// if this changes, e.g. due to metrics optimizations, just adjust this validation.
		//
		// 12 is pretty arbitrary, I just don't expect us to ever make a histogram that small, so it's probably
		// from accidentally passing scale or something to the wrong argument.
		panic(fmt.Sprintf("length must be between 12 < %d <=160", length))
	}
	// make sure the number of buckets completes a "full" row,
	// to which we will add one more to reach the next start*2^N value.
	//
	// for a more visual example of what this means, see the logged strings in tests:
	// each "row" of values must be the same width, and it must have a single value in the last row.
	//
	// adding a couple buckets to ensure this is met costs very little, and ensures
	// subsetting combines values more consistently (not crossing rows) for longer.
	powerOfTwoWidth := int(math.Pow(2, float64(scale))) // num of buckets needed to double a value
	missing := length % powerOfTwoWidth
	if missing != 0 {
		panic(fmt.Sprintf(`number of buckets must end at a power of 2 of the starting value.  `+
			`got %d, probably raise to %d`,
			length, length+missing,
		))
	}

	var buckets tally.DurationBuckets
	for i := 0; i < length; i++ {
		buckets = append(buckets, nextBucket(start, len(buckets), scale))
	}
	// the loop above has "filled" a full row, we need one more to reach the next power of 2.
	buckets = append(buckets, nextBucket(start, len(buckets), scale))

	if last(buckets) < end*2 {
		panic(fmt.Sprintf("not enough buckets (%d) to exceed the end target (%v) by at least 2x. "+
			"you are STRONGLY encouraged to include ~2x-10x more range than your intended end value, "+
			"preferably 4x+, because we almost always underestimate the ranges needed during incidents",
			length, last(buckets)),
		)
	}

	return SubsettableHistogram{
		tallyBuckets: append(
			// always include a zero value at the beginning, so negative values are noticeable ("-inf to 0" bucket)
			tally.DurationBuckets{0},
			buckets...,
		),
		scale: scale,
	}
}

// last-item-in-slice helper to eliminate some magic `-1`s
func last[T any, X ~[]T](s X) T {
	return s[len(s)-1]
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
func (s SubsettableHistogram) print(to func(string, ...any)) {
	to("%v\n", s.tallyBuckets[0:1]) // zero value on its own row
	for rowStart := 1; rowStart < s.len(); rowStart += s.width() {
		to("%v\n", s.tallyBuckets[rowStart:min(rowStart+s.width(), len(s.tallyBuckets))])
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
func (i IntSubsettableHistogram) print(to func(string, ...any)) {
	// fairly unreadable as duration-strings, so convert to int by hand
	to("[%d]\n", int(i.tallyBuckets[0])) // zero value on its own row
	for rowStart := 1; rowStart < i.len(); rowStart += i.width() {
		ints := make([]int, 0, i.width())
		for _, d := range i.tallyBuckets[rowStart:min(rowStart+i.width(), len(i.tallyBuckets))] {
			ints = append(ints, int(d))
		}
		to("%v\n", ints)
	}
}

var _ histogrammy[SubsettableHistogram] = SubsettableHistogram{}
var _ histogrammy[IntSubsettableHistogram] = IntSubsettableHistogram{}

// internal utility/test methods, but could be exposed if there's a use for it
type histogrammy[T any] interface {
	histScale() int                 // exponential scale value.  0..3 inclusive.
	width() int                     // number of values per power of 2 == how wide to print each row.  1, 2, 4, or 8.
	len() int                       // number of buckets
	start() time.Duration           // first non-zero bucket
	end() time.Duration             // last bucket
	buckets() tally.DurationBuckets // access to all buckets
	subsetTo(newScale int) T        // generic so specific types can be returned
	tags() map[string]string        // start, end, and scale tags that need to be implicitly added

	print(to func(string, ...any)) // test-oriented printer
}
