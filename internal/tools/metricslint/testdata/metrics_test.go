package structured

import (
	"time"
)

func ignoredInTestFiles() {
	var emitter Emitter
	tags := Tags{}

	// test code is allowed to break the rules because it does not run in production

	emitter.Count("base_count", 1, tags)
	emitter.Gauge("gauge", 12, tags)
	emitter.Histogram("hist_ns", nil, time.Second, tags)

	const sneakyConst = "bad_metric"
	emitter.IntHistogram(sneakyConst, nil, 3, tags)
	emitter.Count(sneakyConst, 1, tags)
}
