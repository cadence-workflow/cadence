package statistics

import (
	"fmt"
	"math"
	"time"
)

func CalculateSmoothedLoad(prev, current float64, lastUpdate, now time.Time) (float64, error) {
	if math.IsNaN(current) || math.IsInf(current, 0) {
		return 0, fmt.Errorf("current load is NaN or Inf: %f", current)
	}
	if math.IsNaN(prev) || math.IsInf(prev, 0) {
		return 0, fmt.Errorf("previous load is NaN or Inf: %f", prev)
	}
	const tau = 30 * time.Second // smaller = more responsive, larger = smoother
	if lastUpdate.IsZero() || tau <= 0 {
		return current, nil
	}
	if now.Before(lastUpdate) {
		return current, nil
	}
	dt := now.Sub(lastUpdate)
	alpha := 1 - math.Exp(-dt.Seconds()/tau.Seconds())
	return (1-alpha)*prev + alpha*current, nil
}
