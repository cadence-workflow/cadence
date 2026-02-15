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
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var IsMetric = regexp.MustCompile(`^[a-z][a-z_]*$`).MatchString

func TestScopeDefsMapped(t *testing.T) {
	for i := PersistenceCreateShardScope; i < NumCommonScopes; i++ {
		key, ok := ScopeDefs[Common][i]
		require.True(t, ok)
		require.NotEmpty(t, key)
		for tag := range key.tags {
			assert.True(t, IsMetric(tag), "metric tags should conform to regex")
		}
	}
	for i := AdminDescribeHistoryHostScope; i < NumAdminScopes; i++ {
		key, ok := ScopeDefs[Frontend][i]
		require.True(t, ok)
		require.NotEmpty(t, key)
		for tag := range key.tags {
			assert.True(t, IsMetric(tag), "metric tags should conform to regex")
		}
	}
	for i := FrontendStartWorkflowExecutionScope; i < NumFrontendScopes; i++ {
		key, ok := ScopeDefs[Frontend][i]
		require.True(t, ok)
		require.NotEmpty(t, key)
		for tag := range key.tags {
			assert.True(t, IsMetric(tag), "metric tags should conform to regex")
		}
	}
	for i := HistoryStartWorkflowExecutionScope; i < NumHistoryScopes; i++ {
		key, ok := ScopeDefs[History][i]
		require.True(t, ok)
		require.NotEmpty(t, key)
		for tag := range key.tags {
			assert.True(t, IsMetric(tag), "metric tags should conform to regex")
		}
	}
	for i := MatchingPollForDecisionTaskScope; i < NumMatchingScopes; i++ {
		key, ok := ScopeDefs[Matching][i]
		require.True(t, ok)
		require.NotEmpty(t, key)
		for tag := range key.tags {
			assert.True(t, IsMetric(tag), "metric tags should conform to regex")
		}
	}
	for i := ReplicatorScope; i < NumWorkerScopes; i++ {
		key, ok := ScopeDefs[Worker][i]
		require.True(t, ok)
		require.NotEmpty(t, key)
		for tag := range key.tags {
			assert.True(t, IsMetric(tag), "metric tags should conform to regex")
		}
	}
	for i := ShardDistributorGetShardOwnerScope; i < NumShardDistributorScopes; i++ {
		key, ok := ScopeDefs[ShardDistributor][i]
		require.True(t, ok)
		require.NotEmpty(t, key)
		for tag := range key.tags {
			assert.True(t, IsMetric(tag), "metric tags should conform to regex")
		}
	}
}

func TestMetricDefsMapped(t *testing.T) {
	for i := CadenceRequests; i < NumCommonMetrics; i++ {
		key, ok := MetricDefs[Common][i]
		require.True(t, ok, "common enum %v is missing a metric definition", i)
		require.NotEmpty(t, key)
	}
	for i := TaskRequests; i < NumHistoryMetrics; i++ {
		key, ok := MetricDefs[History][i]
		require.True(t, ok, "history enum %v is missing a metric definition", i)
		require.NotEmpty(t, key)
	}
	for i := PollSuccessPerTaskListCounter; i < NumMatchingMetrics; i++ {
		key, ok := MetricDefs[Matching][i]
		require.True(t, ok, "matching enum %v is missing a metric definition", i)
		require.NotEmpty(t, key)
	}
	for i := ReplicatorMessages; i < NumWorkerMetrics; i++ {
		key, ok := MetricDefs[Worker][i]
		require.True(t, ok, "worker enum %v is missing a metric definition", i)
		require.NotEmpty(t, key)
	}
	for i := ShardDistributorRequests; i < NumShardDistributorMetrics; i++ {
		key, ok := MetricDefs[ShardDistributor][i]
		require.True(t, ok, "shard distributor enum %v is missing a metric definition", i)
		require.NotEmpty(t, key)
	}
}

func TestMetricDefs(t *testing.T) {
	for service, metrics := range MetricDefs {
		for _, metricDef := range metrics {
			matched := IsMetric(string(metricDef.metricName))
			assert.True(t, matched, fmt.Sprintf("Service: %v, metric_name: %v", service, metricDef.metricName))
		}
	}
}

// "index -> operation" must be unique so they can be looked up without needing to know the service ID.
//
// Duplicate indexes with the same operation name are technically fine, but there doesn't seem to be any benefit in allowing it,
// and it trivially ensures that all values have only one operation name.
func TestOperationIndexesAreUnique(t *testing.T) {
	seen := make(map[ScopeIdx]bool)
	for serviceIdx, serviceOps := range ScopeDefs {
		for idx := range serviceOps {
			if seen[idx] {
				t.Error("duplicate operation index:", idx, "with name:", serviceOps[idx].operation, "in service:", serviceIdx)
			}
			seen[idx] = true

			// to serve as documentation: operations are NOT unique due to dups across services.
			// this is probably fine, they're just used as metric tag values.
			// but not being able to prevent dups means it's possible we have unintentional name collisions.
			//
			// name := serviceOps[idx].operation
			// if seenNames[name] {
			// 	t.Error("duplicate operation string:", name, "at different indexes")
			// }
			// seenNames[name] = true
		}
	}
}

func TestMetricsAreUnique(t *testing.T) {
	// Duplicate indexes is arguably fine, but there doesn't seem to be any benefit in allowing it.
	t.Run("indexes", func(t *testing.T) {
		seen := make(map[MetricIdx]string)
		for _, serviceMetrics := range MetricDefs {
			for idx, met := range serviceMetrics {
				if prev, ok := seen[idx]; ok {
					t.Errorf("duplicate metric index %d, with names %q and %q", idx, met.metricName.String(), prev)
				}
				seen[idx] = met.metricName.String()
			}
		}
	})
	// Duplicate names carry a high risk of causing different-tag collisions in Prometheus, and should not be allowed.
	t.Run("names", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, serviceMetrics := range MetricDefs {
			for _, met := range serviceMetrics {
				name := met.metricName.String()
				if seen[name] {
					switch name {
					case "cache_full", "cache_miss", "cache_hit", "cadence_requests_per_tl", "cross_cluster_fetch_errors":
						continue // known dup.  worth changing as some cause problems.
					}
					t.Errorf("duplicate metric name %q", name)
				}
				seen[name] = true
			}
		}
	})
}

func TestHistogramSuffixes(t *testing.T) {
	for _, serviceMetrics := range MetricDefs {
		for _, def := range serviceMetrics {
			if def.intExponentialBuckets != nil {
				if !strings.HasSuffix(def.metricName.String(), "_counts") {
					t.Errorf("int-exponential-histogram metric %q should have suffix \"_counts\"", def.metricName.String())
				}
			}
			if def.exponentialBuckets != nil {
				if !strings.HasSuffix(def.metricName.String(), "_ns") {
					t.Errorf("exponential-histogram metric %q should have suffix \"_ns\"", def.metricName.String())
				}
			}
		}
	}
}

func TestExponentialDurationBuckets(t *testing.T) {
	factor := math.Pow(2, 0.25)
	assert.Equal(t, 80, len(ExponentialDurationBuckets))
	assert.Equal(t, 0*time.Millisecond, ExponentialDurationBuckets[0], "bucket[0] mismatch")
	assert.Equal(t, 1*time.Millisecond, ExponentialDurationBuckets[1], "bucket[1] mismatch")
	assert.InDelta(t, bucketVal(1*time.Millisecond, factor, 2), ExponentialDurationBuckets[2], float64(time.Millisecond), "bucket[2] mismatch")
	assert.InDelta(t, bucketVal(1*time.Millisecond, factor, 79), ExponentialDurationBuckets[79], 0.1*float64(time.Second), "bucket[79] mismatch")
}

func bucketVal(start time.Duration, factor float64, bucket int) time.Duration {
	if bucket == 0 {
		return 0
	}
	return time.Duration(math.Pow(factor, float64(bucket-1)) * float64(start))
}
