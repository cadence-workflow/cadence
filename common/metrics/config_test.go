package metrics

import (
	"maps"
	"testing"
)

func TestHistogramMigrationMetricsExist(t *testing.T) {
	dup := maps.Clone(HistogramMigrationMetrics)
	for _, serviceMetrics := range MetricDefs {
		for _, def := range serviceMetrics {
			delete(dup, def.metricName.String())
		}
	}
	if len(dup) != 0 {
		t.Error("HistogramMigrationMetrics contains metric names which do not exist:", dup)
	}
}
