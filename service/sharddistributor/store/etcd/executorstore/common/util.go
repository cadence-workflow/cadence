package common

import (
	"math"
	"time"
)

func CalculateSmoothedLoad(prev, current float64, lastUpdate, now time.Time) float64 {
	if math.IsNaN(current) || math.IsInf(current, 0) {
		current = 0
	}
	const tau = 30 * time.Second // smaller = more responsive, larger = smoother
	if lastUpdate.IsZero() || tau <= 0 {
		return current
	}
	if now.Before(lastUpdate) {
		return current
	}
	dt := now.Sub(lastUpdate)
	alpha := 1 - math.Exp(-dt.Seconds()/tau.Seconds())
	return (1-alpha)*prev + alpha*current
}
