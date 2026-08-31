// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package metrics

import (
	"runtime"
	"runtime/metrics"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/log"
)

var (
	// Revision is the VCS revision associated with this build. Overridden using ldflags
	// at compile time. Example:
	// $ go build -ldflags "-X github.com/uber/cadence/common/metrics.Revision=abcdef" ...
	// see get-ldflags.sh for GIT_REVISION
	Revision = "unknown"

	// Branch is the VCS branch associated with this build.
	Branch = "unknown"

	// ReleaseVersion is the version associated with this build.
	ReleaseVersion = "unknown"

	// BuildDate is the date this build was created.
	BuildDate = "unknown"

	// BuildTimeUnix is the seconds since epoch representing the date this build was created.
	BuildTimeUnix = "0"

	// goVersion is the current runtime version.
	goVersion = runtime.Version()
	// cadenceVersion is the current version of cadence
	cadenceVersion = VersionString
)

const (
	// buildInfoMetricName is the emitted build information metric's name.
	buildInfoMetricName = "build_information"

	// buildAgeMetricName is the emitted build age metric's name.
	buildAgeMetricName = "build_age"
)

// RuntimeMetricsReporter A struct containing the state of the RuntimeMetricsReporter.
type RuntimeMetricsReporter struct {
	scope          tally.Scope
	buildInfoScope tally.Scope
	reportInterval time.Duration
	started        int32
	quit           chan struct{}
	wg             sync.WaitGroup
	logger         log.Logger
	lastNumGC      uint32
	buildTime      time.Time
	cpuSamples     []metrics.Sample
}

// NewRuntimeMetricsReporter Creates a new RuntimeMetricsReporter.
// hostName is optional; when non-empty it is added as a host tag on the reported metrics.
func NewRuntimeMetricsReporter(
	scope tally.Scope,
	reportInterval time.Duration,
	logger log.Logger,
	instanceID string,
	hostName string,
) *RuntimeMetricsReporter {
	const (
		base    = 10
		bitSize = 64
	)
	if len(instanceID) > 0 {
		scope = scope.Tagged(map[string]string{instance: instanceID})
	}
	if len(hostName) > 0 {
		scope = scope.Tagged(map[string]string{host: hostName})
	}
	var memstats runtime.MemStats
	runtime.ReadMemStats(&memstats)
	rReporter := &RuntimeMetricsReporter{
		scope:          scope,
		reportInterval: reportInterval,
		logger:         logger,
		lastNumGC:      memstats.NumGC,
		quit:           make(chan struct{}),
		// See description of each of these here https://go.dev/src/runtime/metrics.go
		cpuSamples: []metrics.Sample{
			{Name: "/cpu/classes/gc/total:cpu-seconds"},
			{Name: "/cpu/classes/total:cpu-seconds"},
			{Name: "/cpu/classes/idle:cpu-seconds"},
		},
	}
	rReporter.buildInfoScope = scope.Tagged(
		map[string]string{
			revisionTag:       Revision,
			branchTag:         Branch,
			buildDateTag:      BuildDate,
			buildVersionTag:   ReleaseVersion,
			goVersionTag:      goVersion,
			cadenceVersionTag: cadenceVersion,
		},
	)
	sec, err := strconv.ParseInt(BuildTimeUnix, base, bitSize)
	if err != nil || sec < 0 {
		sec = 0
	}
	rReporter.buildTime = time.Unix(sec, 0)
	return rReporter
}

// report Sends runtime metrics to the local metrics collector.
func (r *RuntimeMetricsReporter) report() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.Read(r.cpuSamples)

	r.scope.Gauge(NumGoRoutinesGauge).Update(float64(runtime.NumGoroutine()))
	r.scope.Gauge(GoMaxProcsGauge).Update(float64(runtime.GOMAXPROCS(0)))
	r.scope.Gauge(MemoryAllocatedGauge).Update(float64(memStats.Alloc))
	r.scope.Gauge(MemoryTotalAllocGauge).Update(float64(memStats.TotalAlloc))
	r.scope.Gauge(MemoryHeapGauge).Update(float64(memStats.HeapAlloc))
	r.scope.Gauge(MemoryHeapIdleGauge).Update(float64(memStats.HeapIdle))
	r.scope.Gauge(MemoryHeapInuseGauge).Update(float64(memStats.HeapInuse))
	r.scope.Gauge(MemoryHeapSysGauge).Update(float64(memStats.HeapSys))
	r.scope.Gauge(MemoryHeapReleasedGauge).Update(float64(memStats.HeapReleased))
	r.scope.Gauge(MemoryHeapObjectsGauge).Update(float64(memStats.HeapObjects))
	r.scope.Gauge(MemoryStackGauge).Update(float64(memStats.StackInuse))
	r.scope.Gauge(MemoryStackSysGauge).Update(float64(memStats.StackSys))
	r.scope.Gauge(MemoryMallocsGauge).Update(float64(memStats.Mallocs))
	r.scope.Gauge(MemoryFreesGauge).Update(float64(memStats.Frees))
	r.scope.Gauge(MemoryNextGCGauge).Update(float64(memStats.NextGC))
	r.scope.Gauge(MemoryGCCPUFracGauge).Update(memStats.GCCPUFraction)

	// memStats.NumGC is a perpetually incrementing counter (unless it wraps at 2^32)
	num := memStats.NumGC
	lastNum := atomic.SwapUint32(&r.lastNumGC, num) // reset for the next iteration
	if delta := num - lastNum; delta > 0 {
		r.scope.Counter(NumGCCounter).Inc(int64(delta))
		if delta > 255 {
			// too many GCs happened, the timestamps buffer got wrapped around. Report only the last 256
			lastNum = num - 256
		}
		for i := lastNum; i != num; i++ {
			pause := memStats.PauseNs[i%256]
			r.scope.Timer(GcPauseMsTimer).Record(time.Duration(pause))
		}
	}

	gcCPUTotal := r.cpuSamples[0].Value
	totalCPUTime := r.cpuSamples[1].Value
	idleCPUTime := r.cpuSamples[2].Value

	if gcCPUTotal.Kind() != metrics.KindBad {
		r.scope.Gauge(GcCPUTotalGauge).Update(gcCPUTotal.Float64())
	}
	if totalCPUTime.Kind() != metrics.KindBad && idleCPUTime.Kind() != metrics.KindBad {
		r.scope.Gauge(CPUTotalGauge).Update(totalCPUTime.Float64() - idleCPUTime.Float64())
	}

	// report build info
	buildInfoGauge := r.buildInfoScope.Gauge(buildInfoMetricName)
	buildAgeGauge := r.buildInfoScope.Gauge(buildAgeMetricName)
	buildInfoGauge.Update(1.0)
	buildAgeGauge.Update(float64(time.Since(r.buildTime)))
}

// Start Starts the reporter thread that periodically emits metrics.
func (r *RuntimeMetricsReporter) Start() {
	if !atomic.CompareAndSwapInt32(&r.started, 0, 1) {
		return
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.reportInterval)
		for {
			select {
			case <-ticker.C:
				r.report()
			case <-r.quit:
				ticker.Stop()
				return
			}
		}
	}()
	r.logger.Info("RuntimeMetricsReporter started")
}

// Stop Stops reporting of runtime metrics and waits for the reporting goroutine to exit.
// The reporter cannot be started again after it's been stopped.
func (r *RuntimeMetricsReporter) Stop() {
	close(r.quit)
	r.wg.Wait()
	r.logger.Info("RuntimeMetricsReporter stopped")
}
