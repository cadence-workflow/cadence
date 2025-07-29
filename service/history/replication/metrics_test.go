package replication

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
)

func TestHistogramRange(t *testing.T) {
	assert.Len(t, default1ms100sBuckets, 161)
	// defaultScale=4 -> upper bound of just under ~1s
	// defaultScale=3, 0.1ms start -> upper bound of 96s
	// defaultScale=3, 1ms start -> upper bound of ~16 minutes
	// defaultScale=2 -> upper bound of ~30 years
	// actually 16m upper bound at scale=3
	assert.Equal(t, 100*time.Second, default1ms100sBuckets[len(default1ms100sBuckets)-1])
	// goes 0, 1ms, 1.09ms, 1.19ms, 1.3ms, 1.41ms, 1.54ms, 1.68ms, 1.83ms, 2ms...
	// this feels pretty high-precision :\
	// math look sound though, it essentially perfectly matches the samples.
	//
	// it's *oddly* high precision near 1ms for such bad precision below 1ms,
	// which is a fairly common latency regime...
	//
	// this kinda keeps convincing me that this shouldn't be a zero-centered thing,
	// it should be a double-sided sinusoidal that is most precise at the target value.
	assert.Equal(t, time.Millisecond, fmt.Sprintf("%v", default1ms100sBuckets))
	for i := 1; i < len(default1ms100sBuckets); i += 8 {
		t.Logf("%v", default1ms100sBuckets[i:i+8])
	}
	/*
		full range, groups of 10:
		   	[1ms 1.090507ms 1.189206ms 1.296838ms 1.414211ms 1.542208ms 1.681789ms 1.834003ms 1.999994ms 2.181008ms]
		   	[2.378406ms 2.59367ms 2.828417ms 3.08441ms 3.363572ms 3.668001ms 3.999983ms 4.362012ms 4.756807ms 5.187334ms]
		   	[5.656827ms 6.168813ms 6.727138ms 7.335996ms 7.99996ms 8.724018ms 9.513609ms 10.374664ms 11.313651ms 12.337623ms]
		   	[13.454273ms 14.671988ms 15.999916ms 17.448032ms 19.027213ms 20.749322ms 22.627296ms 24.675241ms 26.908541ms 29.343972ms]
		   	[31.999828ms 34.896059ms 38.054422ms 41.498641ms 45.254588ms 49.350478ms 53.817077ms 58.687938ms 63.99965ms 69.792113ms]
		   	[76.108838ms 82.997276ms 90.509171ms 98.70095ms 107.634149ms 117.375871ms 127.999294ms 139.584219ms 152.21767ms 165.994546ms]
		   	[181.018335ms 197.401894ms 215.268291ms 234.751735ms 255.998582ms 279.168433ms 304.435334ms 331.989085ms 362.036664ms 394.803781ms]
		   	[430.536576ms 469.503465ms 511.997159ms 558.33686ms 608.870663ms 663.978166ms 724.073324ms 789.607558ms 861.073147ms 939.006925ms]
		   	[1.023994312s 1.116673715s 1.217741321s 1.327956326s 1.448146642s 1.579215111s 1.72214629s 1.878013846s 2.047988621s 2.233347427s]
		   	[2.435482638s 2.655912649s 2.896293281s 3.158430218s 3.444292575s 3.756027686s 4.095977235s 4.466694847s 4.87096527s 5.311825292s]
		   	[5.792586555s 6.31686043s 6.888585145s 7.512055367s 8.191954465s 8.933389689s 9.741930534s 10.623650578s 11.585173104s 12.633720854s]
		   	[13.777170283s 15.024110727s 16.383908924s 17.866779372s 19.483861062s 21.24730115s 23.170346202s 25.267441701s 27.554340559s 30.048221448s]
		   	[32.767817841s 35.733558738s 38.967722119s 42.494602295s 46.340692399s 50.534883398s 55.108681114s 1m0.096442891s 1m5.535635678s 1m11.467117471s]
		   	[1m17.935444233s 1m24.989204584s 1m32.681384791s 1m41.069766788s 1m50.21736222s 2m0.192885774s 2m11.071271347s 2m22.934234934s 2m35.870888458s 2m49.97840916s]
		   	[3m5.362769575s 3m22.139533569s 3m40.434724434s 4m0.385771543s 4m22.14254269s 4m45.868469863s 5m11.74177691s 5m39.956818315s 6m10.725539144s 6m44.279067133s]
		   	[7m20.869448863s 8m0.77154308s 8m44.285085374s 9m31.736939721s 10m23.483553816s 11m19.913636625s 12m21.451078284s 13m28.558134261s 14m41.738897721s 16m1.543086156s]

		or perhaps more usefully, groups of 8, where each row doubles:
			[1ms 1.090507ms 1.189206ms 1.296838ms 1.414211ms 1.542208ms 1.681789ms 1.834003ms]
			[1.999994ms 2.181008ms 2.378406ms 2.59367ms 2.828417ms 3.08441ms 3.363572ms 3.668001ms]
			[3.999983ms 4.362012ms 4.756807ms 5.187334ms 5.656827ms 6.168813ms 6.727138ms 7.335996ms]
			[7.99996ms 8.724018ms 9.513609ms 10.374664ms 11.313651ms 12.337623ms 13.454273ms 14.671988ms]
			[15.999916ms 17.448032ms 19.027213ms 20.749322ms 22.627296ms 24.675241ms 26.908541ms 29.343972ms]
			[31.999828ms 34.896059ms 38.054422ms 41.498641ms 45.254588ms 49.350478ms 53.817077ms 58.687938ms]
			[63.99965ms 69.792113ms 76.108838ms 82.997276ms 90.509171ms 98.70095ms 107.634149ms 117.375871ms]
			[127.999294ms 139.584219ms 152.21767ms 165.994546ms 181.018335ms 197.401894ms 215.268291ms 234.751735ms]
			[255.998582ms 279.168433ms 304.435334ms 331.989085ms 362.036664ms 394.803781ms 430.536576ms 469.503465ms]
			[511.997159ms 558.33686ms 608.870663ms 663.978166ms 724.073324ms 789.607558ms 861.073147ms 939.006925ms]
			[1.023994312s 1.116673715s 1.217741321s 1.327956326s 1.448146642s 1.579215111s 1.72214629s 1.878013846s]
			[2.047988621s 2.233347427s 2.435482638s 2.655912649s 2.896293281s 3.158430218s 3.444292575s 3.756027686s]
			[4.095977235s 4.466694847s 4.87096527s 5.311825292s 5.792586555s 6.31686043s 6.888585145s 7.512055367s]
			[8.191954465s 8.933389689s 9.741930534s 10.623650578s 11.585173104s 12.633720854s 13.777170283s 15.024110727s]
			[16.383908924s 17.866779372s 19.483861062s 21.24730115s 23.170346202s 25.267441701s 27.554340559s 30.048221448s]
			[32.767817841s 35.733558738s 38.967722119s 42.494602295s 46.340692399s 50.534883398s 55.108681114s 1m0.096442891s]
			[1m5.535635678s 1m11.467117471s 1m17.935444233s 1m24.989204584s 1m32.681384791s 1m41.069766788s 1m50.21736222s 2m0.192885774s]
			[2m11.071271347s 2m22.934234934s 2m35.870888458s 2m49.97840916s 3m5.362769575s 3m22.139533569s 3m40.434724434s 4m0.385771543s]
			[4m22.14254269s 4m45.868469863s 5m11.74177691s 5m39.956818315s 6m10.725539144s 6m44.279067133s 7m20.869448863s 8m0.77154308s]
			[8m44.285085374s 9m31.736939721s 10m23.483553816s 11m19.913636625s 12m21.451078284s 13m28.558134261s 14m41.738897721s 16m1.543086156s]

		I feel like this should be made to be precise though... it's not going to display nicely, and it allows the error to compound.
		Is it worth faking it, and getting more round numbers, by doubling and then sub-dividing from scratch?

		That yields the following, which seems worth the effort:
			[1ms 1.090507ms 1.189206ms 1.296838ms 1.414211ms 1.542208ms 1.681789ms 1.834003ms]
			[2ms 2.181015ms 2.378413ms 2.593677ms 2.828424ms 3.084418ms 3.363581ms 3.668011ms]
			[4ms 4.36203ms 4.756827ms 5.187356ms 5.656851ms 6.168839ms 6.727166ms 7.336026ms]
			[8ms 8.724061ms 9.513655ms 10.374714ms 11.313705ms 12.337682ms 13.454337ms 14.672058ms]
			[16ms 17.448123ms 19.027313ms 20.749431ms 22.627414ms 24.675369ms 26.90868ms 29.344123ms]
			[32ms 34.896247ms 38.054627ms 41.498865ms 45.254833ms 49.350745ms 53.817369ms 58.688257ms]
			[64ms 69.792494ms 76.109254ms 82.99773ms 90.509666ms 98.70149ms 107.634738ms 117.376514ms]
			[128ms 139.584989ms 152.218509ms 165.995461ms 181.019333ms 197.402982ms 215.269478ms 234.75303ms]
			[256ms 279.169979ms 304.43702ms 331.990924ms 362.038669ms 394.805968ms 430.538961ms 469.506066ms]
			[512ms 558.339959ms 608.874042ms 663.981851ms 724.077342ms 789.61194ms 861.077926ms 939.012136ms]
			[1.024s 1.116679918s 1.217748085s 1.327963703s 1.448154686s 1.579223883s 1.722155856s 1.878024277s]
			[2.048s 2.233359836s 2.43549617s 2.655927406s 2.896309373s 3.158447767s 3.444311713s 3.756048556s]
			[4.096s 4.466719672s 4.870992341s 5.311854813s 5.792618748s 6.316895537s 6.888623429s 7.512097116s]
			[8.192s 8.933439345s 9.741984685s 10.62370963s 11.585237501s 12.633791079s 13.777246864s 15.02419424s]
			[16.384s 17.866878691s 19.483969371s 21.247419262s 23.170475004s 25.267582161s 27.554493732s 30.048388484s]
			[32.768s 35.733757383s 38.967938743s 42.494838525s 46.340950009s 50.535164323s 55.108987465s 1m0.096776969s]
			[1m5.536s 1m11.467514767s 1m17.935877487s 1m24.989677051s 1m32.68190002s 1m41.070328649s 1m50.217974934s 2m0.193553944s]
			[2m11.072s 2m22.935029535s 2m35.871754976s 2m49.979354105s 3m5.363800044s 3m22.140657304s 3m40.435949876s 4m0.387107897s]
			[4m22.144s 4m45.870059071s 5m11.743509954s 5m39.958708213s 6m10.727600093s 6m44.281314613s 7m20.871899757s 8m0.774215799s]
			[8m44.288s 9m31.740118143s 10m23.487019909s 11m19.917416427s 12m21.455200187s 13m28.562629228s 14m41.743799517s 16m1.548431602s]
	*/

	// works, but the precision benefits are basically nonexistent.
	// so use it for simplifying construction, but there isn't any *need*.
	moreprecise := makeBuckets(160, func(i int) time.Duration {
		// exponential func starting at 1ms and growing by 2^1/8 each time, computed uniquely per offset to minimize error.
		//
		// this helps significantly with 2ms+, as it achieves exact power-of-2 millisecond values
		// until you can't notice it any more (too many seconds worth), rather than the simple
		// compounding "multiply prev by 2^(1/8)":
		//   curr = time.Duration(float64(curr) * factor)
		// which yields `1.999994ms` rather than `2ms` and remains bad for all integer-powers after.
		//
		// simply resetting on each integer power (i.e. accumulate error 7 times, then reset)
		// works quite well, but there's no need to make this efficient, and this is simpler.
		// it only runs a couple of times at process start, not per-metric or anything like that.
		return time.Duration(float64(time.Millisecond) * math.Pow(2, float64(i)/8))
	})
	moreprecise = append(tally.DurationBuckets{0}, moreprecise...)
	for i := 1; i < len(moreprecise); i += 8 {
		t.Logf("%v", moreprecise[i:i+8])
	}
}

func TestLogHistogramRange(t *testing.T) {
	t.Log("naive compounding exponential ---")
	// kinda terrible, e.g.
	//   [1ms 1.090507ms 1.189206ms 1.296838ms 1.414211ms 1.542208ms 1.681789ms 1.834003ms]
	//   [1.999994ms 2.181008ms 2.378406ms 2.59367ms 2.828417ms 3.08441ms 3.363572ms 3.668001ms]
	//   [3.999983ms 4.362012ms 4.756807ms 5.187334ms 5.656827ms 6.168813ms 6.727138ms 7.335996ms]
	//    ^^^^^^^^ compounding error makes nasty buckets
	t.Logf("\t%v", naive1ms100sBuckets[0:1])
	for i := 1; i < len(naive1ms100sBuckets); i += 8 {
		t.Logf("\t%v", naive1ms100sBuckets[i:i+8])
	}
	t.Log("reset-every-integer-power exponential ---")
	// better:
	//   [1ms 1.090507ms 1.189206ms 1.296838ms 1.414211ms 1.542208ms 1.681789ms 1.834003ms]
	//   [2ms 2.181015ms 2.378413ms 2.593677ms 2.828424ms 3.084418ms 3.363581ms 3.668011ms]
	//   [4ms 4.36203ms 4.756827ms 5.187356ms 5.656851ms 6.168839ms 6.727166ms 7.336026ms]
	//    ^^^ much better buckets
	t.Logf("\t%v", default1ms100sBuckets[0:1])
	for i := 1; i < len(default1ms100sBuckets); i += 8 {
		t.Logf("\t%v", default1ms100sBuckets[i:i+8])
	}
	t.Log("reset-every-time exponential ---")
	// best?
	//   [1ms 1.090507ms 1.189207ms 1.296839ms 1.414213ms 1.54221ms 1.681792ms 1.834008ms]  // <- rarely, very slightly smaller output (seems never worse?)
	//   [2ms 2.181015ms 2.378414ms 2.593679ms 2.828427ms 3.084421ms 3.363585ms 3.668016ms]
	//   [4ms 4.36203ms 4.756828ms 5.187358ms 5.656854ms 6.168843ms 6.727171ms 7.336032ms]
	//    ^^^ same buckets at start                                                  ^^ sometimes changes these last 2 digits, but we don't care about precision at this level
	moreprecise := makeBuckets(160, func(i int) time.Duration {
		return time.Duration(float64(time.Millisecond) * math.Pow(2, float64(i)/8))
	})
	moreprecise = append(tally.DurationBuckets{0}, moreprecise...)
	t.Logf("\t%v", moreprecise[0:1])
	for i := 1; i < len(moreprecise); i += 8 {
		t.Logf("\t%v", moreprecise[i:min(i+8, len(moreprecise))])
	}
	t.Log("80-bucket half precision ---")
	halfprecision := makeBuckets(80, func(i int) time.Duration {
		return time.Duration(float64(time.Millisecond) * math.Pow(2, float64(i)/4))
	})
	halfprecision = append(tally.DurationBuckets{0}, halfprecision...)
	t.Logf("\t%v", halfprecision[0:1])
	for i := 1; i < len(halfprecision); i += 4 {
		t.Logf("\t%v", halfprecision[i:min(i+4, len(halfprecision))])
	}
	t.Log("half precision with new helper covering 1ms-100s (68 buckets at scale=2) ---")
	// TODO: yea I like this one best.
	// I don't think I need the full 160-evenly-multiplying thing, 136 is fine (scale=3).
	//
	// this still retains perfect subsetting down to scale=1, and lower allowing
	// imprecision in the top bucket only, which is probably fine.
	// TODO: figure out exactly what that looks like and how to handle it, but seems fine.
	newhalf := makeExponentialBuckets(time.Millisecond, 100*time.Second, 2)
	t.Logf("\t%v", newhalf[0:1])
	for i := 1; i < len(newhalf); i += 4 {
		t.Logf("\t%v", newhalf[i:min(i+4, len(newhalf))])
	}
	t.Log("full precision starting at 1s, 160 things ----")
	// is this good enough for workflow end-to-end time?
	// result: 267 hours at peak, not enough.  not even two weeks.
	longrange := makeBuckets(160, func(i int) time.Duration {
		return time.Duration(float64(time.Second) * math.Pow(2, float64(i)/8))
	})
	longrange = append(tally.DurationBuckets{0}, longrange...)
	t.Logf("\t%v", longrange[0:1])
	for i := 1; i < len(longrange); i += 8 {
		t.Logf("\t%v", longrange[i:min(i+8, len(longrange))])
	}
	t.Log("half precision starting at 1s, 160 things ----")
	// maybe this works?
	// result: yes definitely, max-int reached.  needs to be reduced.
	longerrange := makeBuckets(160, func(i int) time.Duration {
		return time.Duration(float64(time.Second) * math.Pow(2, float64(i)/4))
	})
	longerrange = append(tally.DurationBuckets{0}, longerrange...)
	t.Logf("\t%v", longerrange[0:1])
	for i := 1; i < len(longerrange); i += 4 {
		t.Logf("\t%v", longerrange[i:min(i+4, len(longerrange))])
	}
	t.Log("half precision starting at 1s, ending at 3y ----")
	// maybe this works?
	// at half precision this gives us 109 buckets for 3.5y of data, which is pretty reasonable.
	// most won't touch that full range.
	betterlongrange := makeExponentialBuckets(time.Second, 3*365*24*time.Hour, 2)
	t.Logf("\t%v", betterlongrange[0:1])
	for i := 1; i < len(betterlongrange); i += 4 {
		t.Logf("\t%v", betterlongrange[i:min(i+4, len(betterlongrange))])
	}
	t.Logf("number of buckets needed: %d", len(betterlongrange))
	// if we really want to get fancy, we can allow a double-ended histogram here
	// per domain, max 2 scale.  that way they can choose their precision-focus-point.
}

func makeBuckets(num int, factor func(i int) time.Duration) tally.DurationBuckets {
	all := make([]time.Duration, num)
	for i := range all {
		all[i] = factor(i)
	}
	return all
}

// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponential-scale
// divisorExponent is OTEL's "scale" value, and will be used in a 2^(1/(2^divisorExponent)) calculation.
// generally this means 0, 1, 2, or 3, which will yield:
//   - 0: buckets that double in value each time (value = value*2)
//   - 1: buckets that double in value every 2 buckets (2^n/2)
//   - 2: buckets that double in value every 4 buckets (2^n/4)
//   - 3: buckets that double in value every 8 buckets (2^n/8)
//
// going over 3 is not recommended, as it will almost always make FAR too many buckets.
// if the range and scale values given will reach or exceed 1000 buckets, this func will panic to prevent insanity.
//
// this will adjust the maximum value to always produce a 2^scale multiple of buckets.
func makeExponentialBuckets(min, max time.Duration, scale int) tally.DurationBuckets {
	// start with 0 and min, the rest will be computed off it.
	// this is equivalent to `min*2^0/(2^scale)` == `min*2^0` == `min*1` == min,
	// it just simplifies math later.
	all := []time.Duration{
		min,
	}
	// build up the range
	for {
		all = append(all, nextBucket(all, float64(scale)))
		if all[len(all)-1] >= max {
			break
		}
		if len(all) >= 1000 {
			panic(fmt.Sprintf(
				"far too many buckets between %v and %v and a scaling factor of %v == 2^1/%v, "+
					"choose a smaller range or smaller scale",
				min, max, scale, int(math.Pow(2, float64(scale)))))
		}
	}
	// number of buckets needed to "fill" a power of 2
	powerOfTwoWidth := int(math.Pow(2, float64(scale)))
	for len(all)%powerOfTwoWidth != 0 {
		all = append(all, nextBucket(all, float64(scale)))
	}
	// add a leading 0
	result := make([]time.Duration, 0, len(all)+1)
	result = append(result, 0)
	result = append(result, all...)
	return result
}

func nextBucket(all []time.Duration, scale float64) time.Duration {
	// initial * num * 2^(i/(2^scale))
	// re-calculating it every time is of course expensive, but it should be done
	// at process start and never again so it's fine.
	// this way reduces floating point error, ensuring "clean" multiples at every
	// power of 2 (and possible others), instead of e.g. "1ms ... 1.9999994ms".
	return time.Duration(
		float64(all[0]) *
			math.Pow(2, float64(len(all))/math.Pow(2, float64(scale))))
}

/*
maybe a different tactic?
- ask for a counter (or gauge or histogram)
- builder appends tags if desired
- `Inc(1)` constructs those tags on the fly and increments

makes for a much smaller api each time, and no need to repeat the name/type/can't confuse types.

does not, however, prevent mixing up tags or changing tags...
we could add a verifier at each name, but we can't do anything when it fails :\
it would document expectations though.
*/
