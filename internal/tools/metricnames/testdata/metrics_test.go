package structured

import (
	"time"
)

func ignoredInTestFiles() {
	var emitter Emitter
	tags := TestTags{}

	// test code is allowed to break the rules because it does not run in production

	emitter.Count("base_count", 1, tags)
	tags.Gauge("gauge", 12)
	tags.Histogram("hist_ns", nil, time.Second)

	const sneakyConst = "bad_metric"
	tags.IntHistogram(sneakyConst, nil, 3)
	emitter.Count(sneakyConst, 1, tags)
}
