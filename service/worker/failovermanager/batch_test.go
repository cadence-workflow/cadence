// Copyright (c) 2017-2020 Uber Technologies Inc.
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

package failovermanager

import (
	"reflect"
	"testing"
	"time"

	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/workflow"
)

// prefsFromNames builds a slice of DomainFailoverPreferences from domain names with a shared
// PreferredCluster — handy for tests that only care about routing not attribute updates.
func prefsFromNames(names ...string) []DomainFailoverPreferences {
	out := make([]DomainFailoverPreferences, len(names))
	for i, n := range names {
		out[i] = DomainFailoverPreferences{DomainName: n, PreferredCluster: "t"}
	}
	return out
}

// runBatchHelper executes processDomainsInBatches inside a workflow context so workflow.Sleep
// is available. The wrapper workflow returns the aggregated success and failed slices.
func runBatchHelper(
	t *testing.T,
	prefs []DomainFailoverPreferences,
	batchSize int,
	waitBetween time.Duration,
	onBatchStart func(),
	executeBatch batchExecutor,
) (success, failed []string) {
	t.Helper()
	type result struct {
		Success []string
		Failed  []string
	}
	wf := func(ctx workflow.Context) (*result, error) {
		s, f := processDomainsInBatches(ctx, prefs, batchSize, waitBetween, onBatchStart, executeBatch)
		return &result{Success: s, Failed: f}, nil
	}
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(wf)
	env.ExecuteWorkflow(wf)
	if !env.IsWorkflowCompleted() {
		t.Fatalf("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	var r result
	if err := env.GetWorkflowResult(&r); err != nil {
		t.Fatalf("get result error: %v", err)
	}
	return r.Success, r.Failed
}

func TestProcessDomainsInBatches_Empty(t *testing.T) {
	var calls int
	executor := func(ctx workflow.Context, batch []DomainFailoverPreferences) ([]string, []string) {
		calls++
		return domainNames(batch), nil
	}
	success, failed := runBatchHelper(t, nil, 10, time.Second, nil, executor)
	if len(success) != 0 || len(failed) != 0 {
		t.Errorf("want empty results, got success=%v failed=%v", success, failed)
	}
	if calls != 0 {
		t.Errorf("executor should not be called for empty prefs, got %d calls", calls)
	}
}

func TestProcessDomainsInBatches_SingleBatch(t *testing.T) {
	var seen [][]string
	executor := func(ctx workflow.Context, batch []DomainFailoverPreferences) ([]string, []string) {
		seen = append(seen, domainNames(batch))
		return domainNames(batch), nil
	}
	success, _ := runBatchHelper(t, prefsFromNames("d1", "d2"), 10, time.Second, nil, executor)
	if len(seen) != 1 {
		t.Fatalf("want 1 batch, got %d", len(seen))
	}
	if !reflect.DeepEqual(seen[0], []string{"d1", "d2"}) {
		t.Errorf("unexpected batch: %v", seen[0])
	}
	if !reflect.DeepEqual(success, []string{"d1", "d2"}) {
		t.Errorf("unexpected success list: %v", success)
	}
}

func TestProcessDomainsInBatches_MultipleBatches(t *testing.T) {
	var seen [][]string
	executor := func(ctx workflow.Context, batch []DomainFailoverPreferences) ([]string, []string) {
		seen = append(seen, domainNames(batch))
		return domainNames(batch), nil
	}
	success, _ := runBatchHelper(t, prefsFromNames("d1", "d2", "d3"), 2, time.Second, nil, executor)
	if len(seen) != 2 {
		t.Fatalf("want 2 batches, got %d", len(seen))
	}
	if !reflect.DeepEqual(seen[0], []string{"d1", "d2"}) {
		t.Errorf("batch[0] = %v, want [d1 d2]", seen[0])
	}
	if !reflect.DeepEqual(seen[1], []string{"d3"}) {
		t.Errorf("batch[1] = %v, want [d3]", seen[1])
	}
	if !reflect.DeepEqual(success, []string{"d1", "d2", "d3"}) {
		t.Errorf("unexpected aggregate success: %v", success)
	}
}

func TestProcessDomainsInBatches_ExactMultiple(t *testing.T) {
	var seen [][]string
	executor := func(ctx workflow.Context, batch []DomainFailoverPreferences) ([]string, []string) {
		seen = append(seen, domainNames(batch))
		return domainNames(batch), nil
	}
	runBatchHelper(t, prefsFromNames("d1", "d2", "d3", "d4"), 2, time.Second, nil, executor)
	if len(seen) != 2 {
		t.Fatalf("want 2 batches for exact multiple, got %d", len(seen))
	}
	if !reflect.DeepEqual(seen[1], []string{"d3", "d4"}) {
		t.Errorf("last batch should be [d3 d4], got %v", seen[1])
	}
}

func TestProcessDomainsInBatches_PartialFailures(t *testing.T) {
	executor := func(ctx workflow.Context, batch []DomainFailoverPreferences) ([]string, []string) {
		// First entry in each batch succeeds, second fails.
		var ok, fail []string
		for i, p := range batch {
			if i%2 == 0 {
				ok = append(ok, p.DomainName)
			} else {
				fail = append(fail, p.DomainName)
			}
		}
		return ok, fail
	}
	success, failed := runBatchHelper(t, prefsFromNames("d1", "d2", "d3", "d4"), 2, time.Second, nil, executor)
	if !reflect.DeepEqual(success, []string{"d1", "d3"}) {
		t.Errorf("success = %v, want [d1 d3]", success)
	}
	if !reflect.DeepEqual(failed, []string{"d2", "d4"}) {
		t.Errorf("failed = %v, want [d2 d4]", failed)
	}
}

func TestProcessDomainsInBatches_OnBatchStartCalledPerBatch(t *testing.T) {
	var hookCalls int
	hook := func() { hookCalls++ }
	executor := func(ctx workflow.Context, batch []DomainFailoverPreferences) ([]string, []string) {
		return domainNames(batch), nil
	}
	runBatchHelper(t, prefsFromNames("d1", "d2", "d3", "d4", "d5"), 2, time.Second, hook, executor)
	if hookCalls != 3 {
		t.Errorf("hook should fire once per batch (3), got %d", hookCalls)
	}
}

func TestProcessDomainsInBatches_NilOnBatchStart(t *testing.T) {
	executor := func(ctx workflow.Context, batch []DomainFailoverPreferences) ([]string, []string) {
		return domainNames(batch), nil
	}
	success, _ := runBatchHelper(t, prefsFromNames("d1", "d2"), 1, time.Second, nil, executor)
	if len(success) != 2 {
		t.Errorf("nil hook should be tolerated; want 2 successes, got %v", success)
	}
}

func TestProcessDomainsInBatches_ZeroBatchSizeFallsBackToOneBatch(t *testing.T) {
	var seen [][]string
	executor := func(ctx workflow.Context, batch []DomainFailoverPreferences) ([]string, []string) {
		seen = append(seen, domainNames(batch))
		return domainNames(batch), nil
	}
	runBatchHelper(t, prefsFromNames("d1", "d2", "d3"), 0, time.Second, nil, executor)
	if len(seen) != 1 {
		t.Errorf("batchSize<=0 should fold all prefs into one batch; got %d batches", len(seen))
	}
}

func TestDomainNames(t *testing.T) {
	if domainNames(nil) != nil {
		t.Error("domainNames(nil) should return nil")
	}
	got := domainNames([]DomainFailoverPreferences{
		{DomainName: "d1"},
		{DomainName: "d2"},
	})
	if !reflect.DeepEqual(got, []string{"d1", "d2"}) {
		t.Errorf("domainNames = %v, want [d1 d2]", got)
	}
}
