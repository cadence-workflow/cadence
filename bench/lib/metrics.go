// Copyright (c) 2017-2021 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package lib

import (
	"fmt"
	"time"

	"github.com/uber-go/tally"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

// counters go here
const (
	startedCount           = "started"
	FailedCount            = "failed"
	successCount           = "succeeded"
	errTimeoutCount        = "errors.timeout"
	errIncompatibleVersion = "errors.incompatibleversion"
)

// latency metrics go here
const (
	latency      = "latency"
	startLatency = "latency.schedule-to-start"
)

const workflowVersion = 1
const workflowChangeID = "initial"

// WorkflowMetricsProfile is the state that's needed to
// record success/failed and latency metrics at the end
// of a workflow
type WorkflowMetricsProfile struct {
	ctx            workflow.Context
	startTimestamp int64
	Scope          tally.Scope
}

// BeginWorkflow executes the common steps involved in all the workflow functions
// It checks for workflow task version compatibility and also records the execution
// in m3. This function must be the first call in every workflow function
// Returns metrics scope on success, error on failure
func BeginWorkflow(ctx workflow.Context, wfType string, scheduledTimeNanos int64) (*WorkflowMetricsProfile, error) {
	profile := recordWorkflowStart(ctx, wfType, scheduledTimeNanos)
	if err := checkWFVersionCompatibility(ctx); err != nil {
		profile.Scope.Counter(errIncompatibleVersion).Inc(1)
		return nil, err
	}
	return profile, nil
}

// End records the elapsed time and reports the latency,
// success, failed counts to m3
func (profile *WorkflowMetricsProfile) End(err error) error {
	now := workflow.Now(profile.ctx).UnixNano()
	elapsed := time.Duration(now - profile.startTimestamp)
	return recordWorkflowEnd(profile.Scope, elapsed, err)
}

// RecordActivityStart emits metrics at the beginning of an activity function
func RecordActivityStart(
	scope tally.Scope,
	name string,
	scheduledTimeNanos int64,
) (tally.Scope, tally.Stopwatch) {
	scope = scope.Tagged(map[string]string{"operation": name})
	elapsed := max(0, time.Now().UnixNano()-scheduledTimeNanos)
	scope.Timer(startLatency).Record(time.Duration(elapsed))
	scope.Counter(startedCount).Inc(1)
	sw := scope.Timer(latency).Start()
	return scope, sw
}

// RecordActivityEnd emits metrics at the end of an activity function
func RecordActivityEnd(
	scope tally.Scope,
	sw tally.Stopwatch,
	err error,
) {
	sw.Stop()
	if err != nil {
		scope.Counter(FailedCount).Inc(1)
		return
	}
	scope.Counter(successCount).Inc(1)
}

// workflowMetricScope creates and returns a child metric scope with tags
// that identify the current workflow type
func workflowMetricScope(
	ctx workflow.Context,
	wfType string,
) tally.Scope {
	parent := workflow.GetMetricsScope(ctx)
	return parent.Tagged(map[string]string{"operation": wfType})
}

// recordWorkflowStart emits metrics at the beginning of a workflow function
func recordWorkflowStart(
	ctx workflow.Context,
	wfType string,
	scheduledTimeNanos int64,
) *WorkflowMetricsProfile {
	now := workflow.Now(ctx).UnixNano()
	scope := workflowMetricScope(ctx, wfType)
	elapsed := max(0, now-scheduledTimeNanos)
	scope.Timer(startLatency).Record(time.Duration(elapsed))
	scope.Counter(startedCount).Inc(1)
	return &WorkflowMetricsProfile{
		ctx:            ctx,
		startTimestamp: now,
		Scope:          scope,
	}
}

// recordWorkflowEnd emits metrics at the end of a workflow function
func recordWorkflowEnd(
	scope tally.Scope,
	elapsed time.Duration,
	err error,
) error {
	scope.Timer(latency).Record(elapsed)
	if err == nil {
		scope.Counter(successCount).Inc(1)
		return err
	}
	scope.Counter(FailedCount).Inc(1)
	if _, ok := err.(*workflow.TimeoutError); ok {
		scope.Counter(errTimeoutCount).Inc(1)
	}
	return err
}

// checkWFVersionCompatibility takes a workflow.Context param and
// validates that the workflow task currently being handled
// is compatible with this version of the bench - this method
// MUST only be called within a workflow function and it MUST
// be the first line in the workflow function
// Returns an error if the version is incompatible
func checkWFVersionCompatibility(ctx workflow.Context) error {
	version := workflow.GetVersion(ctx, workflowChangeID, workflowVersion, workflowVersion)
	if version != workflowVersion {
		workflow.GetLogger(ctx).Error("workflow version mismatch",
			zap.Int("want", int(workflowVersion)), zap.Int("got", int(version)))
		return fmt.Errorf("workflow version mismatch, want=%v, got=%v", workflowVersion, version)
	}
	return nil
}
