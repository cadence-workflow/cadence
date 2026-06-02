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
	"time"

	"go.uber.org/cadence/workflow"
)

// batchExecutor is a function that takes in a batch of DomainFailoverPreferences, runs an activity
// (either to failover or to rebalance the given domains) and returns the success and failed subsets.
// It is used to abstract away the activity implementation from the caller, to allow re-use of processDomainsInBatches for different activities.
type batchExecutor func(ctx workflow.Context, batch []DomainFailoverPreferences) (success, failed []string)

// processDomainsInBatches splits prefs into batches of size batchSize and calls executeBatch for each,
// sleeps waitBetween between successive batches, and aggregates results. onBatchStart is invoked once
// before each batch starts (used by FailoverWorkflow to honour pause/resume signals); it may be nil.
// The helper is activity-agnostic — each workflow supplies its own executeBatch function.
func processDomainsInBatches(
	ctx workflow.Context,
	prefs []DomainFailoverPreferences,
	batchSize int,
	waitBetween time.Duration,
	onBatchStart func(),
	executeBatch batchExecutor,
) (success, failed []string) {
	if len(prefs) == 0 {
		return nil, nil
	}
	if batchSize <= 0 {
		batchSize = len(prefs)
	}

	total := len(prefs)
	times := total / batchSize
	if total%batchSize != 0 {
		times++
	}

	for i := 0; i < times; i++ {
		if onBatchStart != nil {
			onBatchStart()
		}
		end := (i + 1) * batchSize
		if end > total {
			end = total
		}
		batch := prefs[i*batchSize : end]
		s, f := executeBatch(ctx, batch)
		success = append(success, s...)
		failed = append(failed, f...)
		if i != times-1 {
			workflow.Sleep(ctx, waitBetween)
		}
	}
	return success, failed
}

// domainNames extracts the DomainName field from each entry in prefs. Used by batch executors
// to report an all-failed list when the underlying activity call errors out.
func domainNames(prefs []DomainFailoverPreferences) []string {
	if len(prefs) == 0 {
		return nil
	}
	names := make([]string, len(prefs))
	for i, p := range prefs {
		names[i] = p.DomainName
	}
	return names
}
