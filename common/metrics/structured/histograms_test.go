package structured

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
)

func TestHistogramValues(t *testing.T) {
	t.Run("default_1ms_to_15m", func(t *testing.T) {
		// 1ms to 100s is a generally reasonable range for lots of things, but we need a bit more.
		//
		// going up to 80 buckets (plus a zero value) gives us more room to accurately subset at query time,
		// and doesn't cost much more than the 68 buckets needed to exactly reach 100s.
		//
		// this is therefore our "default" histogram bucket.  if you need sub-millisecond or larger values
		// than about 10 minutes, choose a different one.
		checkHistogram(t, Default1ms10m, 4)
		assert.Equal(t, 81, len(Default1ms10m), "wrong number of buckets")
		assert.Less(t, Default1ms10m[len(Default1ms10m)-1], 15*time.Minute) // roughly 14m 42s
	})
	t.Run("default_1ms_to_24h", func(t *testing.T) {
		checkHistogram(t, Default1ms24h, 4)
		assert.Equal(t, 113, len(Default1ms24h), "wrong number of buckets")
		assert.Less(t, Default1ms24h[len(Default1ms24h)-1], 63*time.Hour)
	})
}

// most histograms should pass this check, but fuzzy comparison is fine if needed for extreme cases.
func checkHistogram(t *testing.T, h tally.DurationBuckets, width int) {
	printHistogram(t, h, 4)
	assert.EqualValues(t, 0, h[0], "first bucket should always be zero")
	for i := 1; i < len(h); i += width {
		if i > 1 {
			// ensure good float math.
			//
			// this is *intentionally* doing exact comparisons, as floating point math with
			// human-friendly integers have precise power-of-2 multiples for a very long time.
			//
			// note that the equivalent tally buckets, e.g.:
			//     tally.MustMakeExponentialDurationBuckets(time.Millisecond, math.Pow(2, 1.0/4.0))
			// fails this test, and the logs produced show ugly e.g. 31.999942ms values.
			assert.Equalf(t, h[i-width]*2, h[i],
				"current row's value (%v) is not a power of 2 greater than previous (%v), skewed / bad math?",
				h[i-width], h[i])
		}
	}
}

func printHistogram(t *testing.T, h tally.DurationBuckets, width int) {
	t.Log(h[:1])
	for i := 1; i < len(h); i += width {
		t.Log(h[i : i+width]) // display row by row for easier reading.
		// ^ this will panic if the histograms are not an even multiple of `width`,
		// that's also a sign that it's constructed incorrectly.
	}
}
