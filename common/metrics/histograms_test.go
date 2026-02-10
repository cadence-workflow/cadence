package metrics

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHistogramValues(t *testing.T) {
	t.Run("default_1ms_to_100s", func(t *testing.T) {
		checkHistogram(t, Default1ms100s, `
[0s]
[1ms 1.189207ms 1.414213ms 1.681792ms]
[2ms 2.378414ms 2.828427ms 3.363585ms]
[4ms 4.756828ms 5.656854ms 6.727171ms]
[8ms 9.513656ms 11.313708ms 13.454342ms]
[16ms 19.027313ms 22.627416ms 26.908685ms]
[32ms 38.054627ms 45.254833ms 53.81737ms]
[64ms 76.109255ms 90.509667ms 107.634741ms]
[128ms 152.21851ms 181.019335ms 215.269482ms]
[256ms 304.437021ms 362.038671ms 430.538964ms]
[512ms 608.874042ms 724.077343ms 861.077929ms]
[1.024s 1.217748085s 1.448154687s 1.722155858s]
[2.048s 2.435496171s 2.896309375s 3.444311716s]
[4.096s 4.870992343s 5.792618751s 6.888623433s]
[8.192s 9.741984686s 11.585237502s 13.777246867s]
[16.384s 19.483969372s 23.170475005s 27.554493735s]
[32.768s 38.967938744s 46.340950011s 55.10898747s]
[1m5.536s 1m17.935877488s 1m32.681900023s 1m50.21797494s]
[2m11.072s 2m35.871754977s 3m5.363800047s 3m40.43594988s]
[4m22.144s 5m11.743509955s 6m10.727600094s 7m20.87189976s]
[8m44.288s 10m23.48701991s 12m21.455200189s 14m41.743799521s]
[17m28.576s]
`)
	})
	t.Run("low_1ms_to_100s", func(t *testing.T) {
		checkHistogram(t, Low1ms100s, `
[0s]
[1ms 1.414213ms]
[2ms 2.828427ms]
[4ms 5.656854ms]
[8ms 11.313708ms]
[16ms 22.627416ms]
[32ms 45.254833ms]
[64ms 90.509667ms]
[128ms 181.019335ms]
[256ms 362.038671ms]
[512ms 724.077343ms]
[1.024s 1.448154687s]
[2.048s 2.896309375s]
[4.096s 5.792618751s]
[8.192s 11.585237502s]
[16.384s 23.170475005s]
[32.768s 46.340950011s]
[1m5.536s 1m32.681900023s]
[2m11.072s 3m5.363800047s]
[4m22.144s 6m10.727600094s]
[8m44.288s 12m21.455200189s]
[17m28.576s]
`)
	})
	t.Run("high_1ms_to_24h", func(t *testing.T) {
		checkHistogram(t, High1ms24h, `
[0s]
[1ms 1.189207ms 1.414213ms 1.681792ms]
[2ms 2.378414ms 2.828427ms 3.363585ms]
[4ms 4.756828ms 5.656854ms 6.727171ms]
[8ms 9.513656ms 11.313708ms 13.454342ms]
[16ms 19.027313ms 22.627416ms 26.908685ms]
[32ms 38.054627ms 45.254833ms 53.81737ms]
[64ms 76.109255ms 90.509667ms 107.634741ms]
[128ms 152.21851ms 181.019335ms 215.269482ms]
[256ms 304.437021ms 362.038671ms 430.538964ms]
[512ms 608.874042ms 724.077343ms 861.077929ms]
[1.024s 1.217748085s 1.448154687s 1.722155858s]
[2.048s 2.435496171s 2.896309375s 3.444311716s]
[4.096s 4.870992343s 5.792618751s 6.888623433s]
[8.192s 9.741984686s 11.585237502s 13.777246867s]
[16.384s 19.483969372s 23.170475005s 27.554493735s]
[32.768s 38.967938744s 46.340950011s 55.10898747s]
[1m5.536s 1m17.935877488s 1m32.681900023s 1m50.21797494s]
[2m11.072s 2m35.871754977s 3m5.363800047s 3m40.43594988s]
[4m22.144s 5m11.743509955s 6m10.727600094s 7m20.87189976s]
[8m44.288s 10m23.48701991s 12m21.455200189s 14m41.743799521s]
[17m28.576s 20m46.974039821s 24m42.910400378s 29m23.487599042s]
[34m57.152s 41m33.948079642s 49m25.820800757s 58m46.975198084s]
[1h9m54.304s 1h23m7.896159284s 1h38m51.641601515s 1h57m33.950396168s]
[2h19m48.608s 2h46m15.792318568s 3h17m43.283203031s 3h55m7.900792337s]
[4h39m37.216s 5h32m31.584637137s 6h35m26.566406062s 7h50m15.801584674s]
[9h19m14.432s 11h5m3.169274274s 13h10m53.132812125s 15h40m31.603169349s]
[18h38m28.864s 22h10m6.338548549s 26h21m46.265624251s 31h21m3.206338698s]
[37h16m57.728s 44h20m12.677097099s 52h43m32.531248503s 62h42m6.412677396s]
[74h33m55.456s]
`)
	})
	t.Run("mid_1ms_24h", func(t *testing.T) {
		checkHistogram(t, Mid1ms24h, `
[0s]
[1ms 1.414213ms]
[2ms 2.828427ms]
[4ms 5.656854ms]
[8ms 11.313708ms]
[16ms 22.627416ms]
[32ms 45.254833ms]
[64ms 90.509667ms]
[128ms 181.019335ms]
[256ms 362.038671ms]
[512ms 724.077343ms]
[1.024s 1.448154687s]
[2.048s 2.896309375s]
[4.096s 5.792618751s]
[8.192s 11.585237502s]
[16.384s 23.170475005s]
[32.768s 46.340950011s]
[1m5.536s 1m32.681900023s]
[2m11.072s 3m5.363800047s]
[4m22.144s 6m10.727600094s]
[8m44.288s 12m21.455200189s]
[17m28.576s 24m42.910400378s]
[34m57.152s 49m25.820800757s]
[1h9m54.304s 1h38m51.641601515s]
[2h19m48.608s 3h17m43.283203031s]
[4h39m37.216s 6h35m26.566406062s]
[9h19m14.432s 13h10m53.132812125s]
[18h38m28.864s 26h21m46.265624251s]
[37h16m57.728s 52h43m32.531248503s]
[74h33m55.456s]
`)
	})
	t.Run("mid_to_16k_ints", func(t *testing.T) {
		checkHistogram(t, Mid1To16k, `
[0]
[8 9 11 13]
[16 19 22 26]
[32 38 45 53]
[64 76 90 107]
[128 152 181 215]
[256 304 362 430]
[512 608 724 861]
[1024 1217 1448 1722]
[2048 2435 2896 3444]
[4096 4870 5792 6888]
[8192 9741 11585 13777]
[16384 19483 23170 27554]
[32768 38967 46340 55108]
[65536 77935 92681 110217]
[131072 155871 185363 220435]
[262144 311743 370727 440871]
[524288]
`)
	})
}

// most histograms should pass this check, but fuzzy comparison is fine if needed for extreme cases.
func checkHistogram[T any](t *testing.T, h histogrammy[T], expected string) {
	var buf strings.Builder
	h.print(func(s string, a ...any) {
		str := fmt.Sprintf(s, a...)
		t.Logf(str)
		buf.WriteString(str)
	})
	if strings.TrimSpace(expected) != strings.TrimSpace(buf.String()) {
		t.Error("histogram definition changed, update the test if this is intended")
	}

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
			// tally also produces incorrect results if you start at e.g. 1,
			// as it just produces endless `1` values.
			assert.Equalf(t, buckets[i-h.width()]*2, buckets[i],
				"current row's value (%v) is not a power of 2 greater than previous (%v), skewed / bad math?",
				buckets[i-h.width()], buckets[i])
		}
	}
}
