package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
)

func TestGaugeMode(t *testing.T) {
	ts := tally.NewTestScope("", nil)
	findName := func(m MetricIdx) string {
		def, ok := MetricDefs[Common][m]
		if ok {
			return def.metricName.String()
		}
		def, ok = MetricDefs[History][m]
		if ok {
			return def.metricName.String()
		}
		t.Fatalf("MetricDef not found in common or history: %v", m)
		return "unknown"
	}

	orig := GaugeMigrationMetrics
	t.Cleanup(func() {
		GaugeMigrationMetrics = orig
	})

	GaugeMigrationMetrics = map[string]struct{}{
		findName(CadenceLatency):                    {},
		findName(BaseCacheByteSize):                 {},
		findName(PersistenceLatency):                {},
		findName(GlobalRatelimiterQuota):            {},
		findName(CadenceDcRedirectionClientLatency): {},
		findName(CadenceShardSuccessGauge):          {},
	}

	c := NewClient(ts, History, MigrationConfig{
		Gauge: GaugeMigration{
			// Default: ..., left at default value (timer)
			Names: map[string]bool{
				findName(CadenceLatency):         true,  // timer type
				findName(BaseCacheByteSize):      false, // gauge type
				findName(PersistenceLatency):     false, // timer type
				findName(GlobalRatelimiterQuota): true,  // gauge type
			},
		},
	})
	scope := c.Scope(HistoryDescribeQueueScope) // scope doesn't matter for this test

	scope.RecordTimer(CadenceLatency, time.Second)
	scope.UpdateGauge(BaseCacheByteSize, 1.0)

	scope.RecordTimer(PersistenceLatency, 2*time.Second)
	scope.UpdateGauge(GlobalRatelimiterQuota, 2.0)

	// unspecified -> default config (timer)
	scope.RecordTimer(CadenceDcRedirectionClientLatency, 3*time.Second)
	scope.UpdateGauge(CadenceShardSuccessGauge, 3.0)

	// not migrating -> always emit
	scope.RecordTimer(CadenceErrBadRequestCounter, 4*time.Second)
	scope.UpdateGauge(BaseCacheByteSizeLimitGauge, 4.0)

	s := ts.Snapshot()
	findMetric := func(idx MetricIdx) (timer, gauge bool) {
		name := findName(idx)
		for _, v := range s.Timers() {
			if v.Name() == name {
				t.Logf("found timer: %v = %v", v.Name(), v.Values())
				timer = true
				break
			}
		}
		for _, v := range s.Gauges() {
			if v.Name() == name {
				t.Logf("found gauge: %v = %v", v.Name(), v.Value())
				gauge = true
				break
			}
		}
		return
	}
	assertFound := func(idx MetricIdx, timer, gauge bool) {
		name := findName(idx)
		foundTimer, foundGauge := findMetric(idx)
		assert.Equalf(t, timer, foundTimer, "wrong timer behavior for %v", name)
		assert.Equalf(t, gauge, foundGauge, "wrong gauge behavior for %v", name)
	}

	// only the timer
	assertFound(CadenceLatency, true, false)
	assertFound(BaseCacheByteSize, false, false)
	// only the gauge
	assertFound(PersistenceLatency, false, false)
	assertFound(GlobalRatelimiterQuota, false, true)
	// timers only (via default)
	assertFound(CadenceDcRedirectionClientLatency, true, false)
	assertFound(CadenceShardSuccessGauge, false, false)
	// not migrating, the correct type should be emitted
	assertFound(CadenceErrBadRequestCounter, true, false)
	assertFound(BaseCacheByteSizeLimitGauge, false, true)
}

func TestCounterMode(t *testing.T) {
	ts := tally.NewTestScope("", nil)
	findName := func(m MetricIdx) string {
		def, ok := MetricDefs[Common][m]
		if ok {
			return def.metricName.String()
		}
		def, ok = MetricDefs[History][m]
		if ok {
			return def.metricName.String()
		}
		t.Fatalf("MetricDef not found in common or history: %v", m)
		return "unknown"
	}

	orig := CounterMigrationMetrics
	t.Cleanup(func() {
		CounterMigrationMetrics = orig
	})

	CounterMigrationMetrics = map[string]struct{}{
		findName(CadenceLatency):                    {},
		findName(CadenceRequests):                   {},
		findName(PersistenceLatency):                {},
		findName(CadenceFailures):                   {},
		findName(CadenceDcRedirectionClientLatency): {},
		findName(CadenceErrBadRequestCounter):       {},
	}

	c := NewClient(ts, History, MigrationConfig{
		Counter: CounterMigration{
			// Default: ..., left at default value (timer)
			Names: map[string]bool{
				findName(CadenceLatency):     true,  // timer type
				findName(CadenceRequests):    false, // counter type
				findName(PersistenceLatency): false, // timer type
				findName(CadenceFailures):    true,  // counter type
			},
		},
	})
	scope := c.Scope(HistoryDescribeQueueScope) // scope doesn't matter for this test

	scope.RecordTimer(CadenceLatency, time.Second)
	scope.IncCounter(CadenceRequests)

	scope.RecordTimer(PersistenceLatency, 2*time.Second)
	scope.IncCounter(CadenceFailures)

	// unspecified -> default config (timer)
	scope.RecordTimer(CadenceDcRedirectionClientLatency, 3*time.Second)
	scope.IncCounter(CadenceErrBadRequestCounter)

	// not migrating -> always emit
	scope.RecordTimer(CadenceErrServiceBusyCounter, 4*time.Second)
	scope.IncCounter(CadenceErrDomainNotActiveCounter)

	s := ts.Snapshot()
	findMetric := func(idx MetricIdx) (timer, counter bool) {
		name := findName(idx)
		for _, v := range s.Timers() {
			if v.Name() == name {
				t.Logf("found timer: %v = %v", v.Name(), v.Values())
				timer = true
				break
			}
		}
		for _, v := range s.Counters() {
			if v.Name() == name {
				t.Logf("found counter: %v = %v", v.Name(), v.Value())
				counter = true
				break
			}
		}
		return
	}
	assertFound := func(idx MetricIdx, timer, counter bool) {
		name := findName(idx)
		foundTimer, foundCounter := findMetric(idx)
		assert.Equalf(t, timer, foundTimer, "wrong timer behavior for %v", name)
		assert.Equalf(t, counter, foundCounter, "wrong counter behavior for %v", name)
	}

	// only the timer
	assertFound(CadenceLatency, true, false)
	assertFound(CadenceRequests, false, false)
	// only the counter
	assertFound(PersistenceLatency, false, false)
	assertFound(CadenceFailures, false, true)
	// timers only (via default)
	assertFound(CadenceDcRedirectionClientLatency, true, false)
	assertFound(CadenceErrBadRequestCounter, false, false)
	// not migrating, the correct type should be emitted
	assertFound(CadenceErrServiceBusyCounter, true, false)
	assertFound(CadenceErrDomainNotActiveCounter, false, true)
}

func TestHistogramMode(t *testing.T) {
	ts := tally.NewTestScope("", nil)
	findName := func(m MetricIdx) string {
		def, ok := MetricDefs[Common][m]
		if ok {
			return def.metricName.String()
		}
		def, ok = MetricDefs[History][m]
		if ok {
			return def.metricName.String()
		}
		t.Fatalf("MetricDef not found in common or history: %v", m)
		return "unknown"
	}

	orig := HistogramMigrationMetrics
	t.Cleanup(func() {
		HistogramMigrationMetrics = orig
	})

	HistogramMigrationMetrics = map[string]struct{}{
		findName(CadenceLatency):                          {},
		findName(ExponentialReplicationTaskLatency):       {},
		findName(PersistenceLatencyPerShard):              {},
		findName(TaskProcessingLatencyHistogram):          {},
		findName(PersistenceLatency):                      {},
		findName(PersistenceLatencyHistogram):             {},
		findName(PersistenceLatencyHistogramPerHost):      {},
		findName(TaskAttemptTimer):                        {},
		findName(TaskAttemptCountsHistogram):              {},
		findName(TaskQueueLatency):                        {},
		findName(TaskQueueLatencyHistogram):               {},
		findName(TaskLatencyPerDomain):                    {},
		findName(TaskLatencyPerDomainHistogram):           {},
		findName(TaskAttemptTimerPerDomain):               {},
		findName(TaskAttemptPerDomainCountsHistogram):     {},
		findName(TaskProcessingLatencyPerDomain):          {},
		findName(TaskProcessingLatencyPerDomainHistogram): {},
		findName(TaskQueueLatencyPerDomain):               {},
		findName(TaskQueueLatencyPerDomainHistogram):      {},
	}

	c := NewClient(ts, History, MigrationConfig{
		Histogram: HistogramMigration{
			// Default: ..., left at default value
			Names: map[string]bool{
				findName(CadenceLatency):                    true,  // timer type
				findName(ExponentialReplicationTaskLatency): false, // histogram type

				findName(PersistenceLatencyPerShard):     false, // timer type
				findName(TaskProcessingLatencyHistogram): true,  // histogram type
			},
		},
	})
	scope := c.Scope(HistoryDescribeQueueScope) // scope doesn't matter for this test

	scope.RecordTimer(CadenceLatency, time.Second)
	scope.ExponentialHistogram(ExponentialReplicationTaskLatency, 2*time.Second)

	scope.RecordTimer(PersistenceLatencyPerShard, 3*time.Second)
	scope.ExponentialHistogram(TaskProcessingLatencyHistogram, 4*time.Second)

	// unspecified -> default config
	scope.RecordTimer(PersistenceLatency, 5*time.Second)
	scope.RecordHistogramDuration(PersistenceLatencyHistogram, 6*time.Second)

	// not migrating -> always emit
	scope.RecordTimer(CadenceDcRedirectionClientLatency, 7*time.Second)
	scope.RecordHistogramDuration(GlobalRatelimiterStartupUsageHistogram, 8*time.Second)

	s := ts.Snapshot()
	findMetric := func(idx MetricIdx) (timer, histogram bool) {
		name := findName(idx)
		for _, v := range s.Timers() {
			if v.Name() == name {
				t.Logf("found timer: %v = %v", v.Name(), v.Values())
				timer = true
				break
			}
		}
		for _, v := range s.Histograms() {
			if v.Name() == findName(idx) {
				nzDur := make(map[time.Duration]int64, 1)
				for k, val := range v.Durations() {
					if val != 0 {
						nzDur[k] = val
					}
				}
				nzVal := make(map[float64]int64, 1)
				for k, val := range v.Values() {
					if val != 0 {
						nzVal[k] = val
					}
				}
				t.Logf("found histogram: %v = %v (values: %v)", v.Name(), nzDur, nzVal)
				histogram = true
				break
			}
		}
		return
	}
	assertFound := func(idx MetricIdx, timer, histogram bool) {
		name := findName(idx)
		foundTimer, foundHistogram := findMetric(idx)
		assert.Equalf(t, foundTimer, timer, "wrong timer behavior for %v", name)
		assert.Equalf(t, foundHistogram, histogram, "wrong histogram behavior for %v", name)
	}

	// only the timer
	assertFound(CadenceLatency, true, false)
	assertFound(ExponentialReplicationTaskLatency, false, false)
	// only the histogram
	assertFound(PersistenceLatencyPerShard, false, false)
	assertFound(TaskProcessingLatencyHistogram, false, true)
	// timers only (via default)
	assertFound(PersistenceLatency, true, false)
	assertFound(PersistenceLatencyHistogram, false, false)
	// not migrating, the correct type should be emitted
	assertFound(CadenceDcRedirectionClientLatency, true, false)
	assertFound(GlobalRatelimiterStartupUsageHistogram, false, true)

	// when fixing: check logs!  you should see metrics with values for: 1, 4, 7, 8
}

func TestExponentialHistogramRollup(t *testing.T) {
	ts := tally.NewTestScope("", nil)
	findName := func(m MetricIdx) string {
		def, ok := MetricDefs[History][m]
		if ok {
			return def.metricName.String()
		}
		t.Fatalf("MetricDef not found in history: %v", m)
		return "unknown"
	}
	findRollupName := func(m MetricIdx) string {
		def, ok := MetricDefs[History][m]
		if ok {
			return def.metricRollupName.String()
		}
		t.Fatalf("MetricDef not found in history: %v", m)
		return "unknown"
	}

	orig := HistogramMigrationMetrics
	t.Cleanup(func() {
		HistogramMigrationMetrics = orig
	})

	HistogramMigrationMetrics = map[string]struct{}{
		findName(TaskProcessingLatencyPerDomainHistogram):       {},
		findRollupName(TaskProcessingLatencyPerDomainHistogram): {},
		findName(TaskAttemptPerDomainCountsHistogram):           {},
		findRollupName(TaskAttemptPerDomainCountsHistogram):     {},
	}

	c := NewClient(ts, History, MigrationConfig{
		Histogram: HistogramMigration{
			Names: map[string]bool{
				findName(TaskProcessingLatencyPerDomainHistogram):       true,
				findRollupName(TaskProcessingLatencyPerDomainHistogram): true,
				findName(TaskAttemptPerDomainCountsHistogram):           true,
				findRollupName(TaskAttemptPerDomainCountsHistogram):     true,
			},
		},
	})

	scope := c.Scope(HistoryDescribeQueueScope, DomainTag("test-domain"))

	scope.ExponentialHistogram(TaskProcessingLatencyPerDomainHistogram, time.Second)
	scope.IntExponentialHistogram(TaskAttemptPerDomainCountsHistogram, 42)

	s := ts.Snapshot()
	histNames := make(map[string]bool)
	for _, h := range s.Histograms() {
		histNames[h.Name()] = true
	}

	assert.True(t, histNames[findName(TaskProcessingLatencyPerDomainHistogram)],
		"per-domain ExponentialHistogram metric should be emitted")
	assert.True(t, histNames[findRollupName(TaskProcessingLatencyPerDomainHistogram)],
		"rollup ExponentialHistogram metric should be emitted on rootScope")

	assert.True(t, histNames[findName(TaskAttemptPerDomainCountsHistogram)],
		"per-domain IntExponentialHistogram metric should be emitted")
	assert.True(t, histNames[findRollupName(TaskAttemptPerDomainCountsHistogram)],
		"rollup IntExponentialHistogram metric should be emitted on rootScope")
}

func TestGaugeRollupUsesRootScope(t *testing.T) {
	rootScope := tally.NewTestScope("", nil)
	childScope := tally.NewTestScope("", nil)

	const testIdx MetricIdx = 9999
	defs := map[MetricIdx]metricDefinition{
		testIdx: {
			metricName:       "gauge_per_domain",
			metricRollupName: "gauge_rollup",
			metricType:       Gauge,
		},
	}

	scope := newMetricsScope(rootScope, childScope, defs, true, MigrationConfig{})

	scope.UpdateGauge(testIdx, 42.0)

	childSnap := childScope.Snapshot()
	rootSnap := rootScope.Snapshot()

	foundInChild := false
	for _, g := range childSnap.Gauges() {
		if g.Name() == "gauge_per_domain" {
			foundInChild = true
		}
		assert.NotEqualf(t, "gauge_rollup", g.Name(),
			"rollup gauge should NOT be emitted on child scope")
	}
	assert.True(t, foundInChild, "per-domain gauge should be emitted on child scope")

	foundInRoot := false
	for _, g := range rootSnap.Gauges() {
		if g.Name() == "gauge_rollup" {
			foundInRoot = true
		}
	}
	assert.True(t, foundInRoot, "rollup gauge should be emitted on root scope")
}

func TestRecordHistogramDurationDomainTaggedDualEmit(t *testing.T) {
	rootScope := tally.NewTestScope("", nil)
	childScope := tally.NewTestScope("", map[string]string{domain: "test-domain"})

	defs := map[MetricIdx]metricDefinition{
		CadenceLatencyHistogram: {
			metricName: "cadence_latency_ns",
			metricType: Histogram,
		},
	}

	scope := newMetricsScope(rootScope, childScope, defs, true, MigrationConfig{
		Histogram: HistogramMigration{Default: "histogram"},
	})

	scope.RecordHistogramDuration(CadenceLatencyHistogram, 5*time.Millisecond)

	// per-domain series emitted on child scope
	domainFound := false
	allFound := false
	for _, h := range childScope.Snapshot().Histograms() {
		if h.Name() == "cadence_latency_ns" {
			tags := h.Tags()
			if tags[domain] == "test-domain" {
				domainFound = true
			}
			if tags[domain] == allValue {
				allFound = true
			}
		}
	}
	assert.True(t, domainFound, "per-domain histogram series should be emitted")
	assert.True(t, allFound, "aggregate domain=all histogram series should be emitted")
}
