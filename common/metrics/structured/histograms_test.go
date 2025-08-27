package structured

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
)

func TestHistogramValues(t *testing.T) {
	t.Run("default_1ms_to_10m", func(t *testing.T) {
		checkHistogram(t, Default1ms10m)
		assert.Equal(t, 81, Default1ms10m.len(), "wrong number of buckets")
		assertBetween(t, 10*time.Minute, Default1ms10m.max(), 15*time.Minute) // roughly 14m 42s
	})
	t.Run("high_1ms_to_24h", func(t *testing.T) {
		checkHistogram(t, High1ms24h)
		assert.Equal(t, 113, High1ms24h.len(), "wrong number of buckets")
		assertBetween(t, 24*time.Hour, High1ms24h.max(), 64*time.Hour) // roughly 63h
	})
	t.Run("mid_1ms_24h", func(t *testing.T) {
		checkHistogram(t, Mid1ms24h)
		assert.Equal(t, 57, Mid1ms24h.len(), "wrong number of buckets")
		assertBetween(t, 12*time.Hour, Mid1ms24h.max(), 64*time.Hour) // roughly 53h
	})
	t.Run("mid_to_32k_ints", func(t *testing.T) {
		// note: this histogram has some duplicates:
		// [0]
		// [1 1 1 1]
		// [2 2 2 3]
		// [4 4 5 6]
		// [8 9 11 13]
		// ...
		//
		// this wastes a bit of memory, but Tally will choose the same index each
		// time for a specific value, so it does not cause extra non-zero series
		// to be stored or emitted.
		//
		// if this turns out to be expensive in Prometheus / etc, it's easy
		// enough to start the histogram at 8, and dual-emit a non-exponential
		// histogram for lower values.
		checkHistogram(t, Mid1To32k)
		assert.Equal(t, 65, Mid1To32k.len(), "wrong number of buckets")
		assertBetween(t, 32_000, int(Mid1To32k.max()), 64_000) // 55108
	})
	t.Run("low_cardinality_1ms_10s", func(t *testing.T) {
		checkHistogram(t, Low1ms10s)
		assert.Equal(t, 33, Low1ms10s.len(), "wrong number of buckets")
		assertBetween(t, 10*time.Second, Low1ms10s.max(), time.Minute) // roughly 46s
	})
}

// test helpers, but could be moved elsewhere if they prove useful
func (s SubsettableHistogram) width() int                     { return int(math.Pow(2, float64(s.scale))) }
func (s SubsettableHistogram) len() int                       { return len(s.tallyBuckets) }
func (s SubsettableHistogram) max() time.Duration             { return s.tallyBuckets[len(s.tallyBuckets)-1] }
func (s SubsettableHistogram) buckets() tally.DurationBuckets { return s.tallyBuckets }

func (i IntSubsettableHistogram) width() int { return int(math.Pow(2, float64(i.scale))) }
func (i IntSubsettableHistogram) len() int   { return len(i.tallyBuckets) }
func (i IntSubsettableHistogram) max() time.Duration {
	return i.tallyBuckets[len(i.tallyBuckets)-1]
}
func (i IntSubsettableHistogram) buckets() tally.DurationBuckets { return i.tallyBuckets }

type histogrammy interface {
	SubsettableHistogram | IntSubsettableHistogram

	width() int
	len() int
	max() time.Duration
	buckets() tally.DurationBuckets
}

type numeric interface {
	~int | ~int64
}

func assertBetween[T numeric](t *testing.T, min, actual, max T, msgAndArgs ...interface{}) {
	if actual < min || actual > max {
		assert.Fail(t, fmt.Sprintf("value %v not between %v and %v", actual, min, max), msgAndArgs...)
	}
}

// most histograms should pass this check, but fuzzy comparison is fine if needed for extreme cases.
func checkHistogram[T histogrammy](t *testing.T, h T) {
	printHistogram(t, h)
	buckets := h.buckets()
	assert.EqualValues(t, 0, buckets[0], "first bucket should always be zero")
	for i := 1; i < len(buckets); i += h.width() {
		if i > 1 {
			// ensure good float math.
			//
			// this is *intentionally* doing exact comparisons, as floating point math with
			// human-friendly integers have precise power-of-2 multiples for a very long time.
			//
			// note that the equivalent tally buckets, e.g.:
			//     tally.MustMakeExponentialDurationBuckets(time.Millisecond, math.Pow(2, 1.0/4.0))
			// fails this test, and the logs produced show ugly e.g. 31.999942ms values.
			// it also produces incorrect results if you start at e.g. 1, as it never exceeds 1.
			assert.Equalf(t, buckets[i-h.width()]*2, buckets[i],
				"current row's value (%v) is not a power of 2 greater than previous (%v), skewed / bad math?",
				buckets[i-h.width()], buckets[i])
		}
	}
}

func printHistogram[T histogrammy](t *testing.T, histogram T) {
	switch h := (any)(histogram).(type) {
	case SubsettableHistogram:
		t.Log(h.buckets()[:1])
		for i := 1; i < len(h.buckets()); i += h.width() {
			t.Log(h.buckets()[i : i+h.width()]) // display row by row for easier reading.
			// ^ this will panic if the histograms are not an even multiple of `width`,
			// that's also a sign that it's constructed incorrectly.
		}
	case IntSubsettableHistogram:
		hi := make([]int, len(h.buckets())) // convert to int
		for i, v := range h.buckets() {
			hi[i] = int(v)
		}
		t.Log(hi[:1])
		for i := 1; i < len(hi); i += h.width() {
			t.Log(hi[i : i+h.width()])
		}
	default:
		panic("unreachable")
	}
}
