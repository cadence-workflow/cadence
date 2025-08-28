package metrics

import (
	"time"
)

type Scope interface {
	IncCounter(counter int)
	AddCounter(counter int, delta int64)
	StartTimer(timer int) Stopwatch
	RecordTimer(timer int, d time.Duration)
	RecordHistogramDuration(timer int, d time.Duration)
	RecordHistogramValue(timer int, value float64)
	ExponentialHistogram(hist int, d time.Duration)
	IntExponentialHistogram(hist int, value int)
	UpdateGauge(gauge int, value float64)
	Tagged(tags ...Tag) Scope
}
type Tag struct{}
type Stopwatch interface{}

func do() {
	var scope Scope

	scope.IncCounter(123)
	scope.IncCounter(123)
	scope.ExponentialHistogram(123, 0)
}
