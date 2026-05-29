// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package cluster

import (
	"fmt"
	"sort"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

func TestMetadataBehaviour(t *testing.T) {
	const clusterName1 = "c1"
	const initialFailoverVersionC1 = 0
	const clusterName2 = "c2"
	const initialFailoverVersionC2 = 2
	const clusterName3 = "c3"
	const initialFailoverVersionC3 = 4

	const failoverVersionIncrement = 100

	tests := map[string]struct {
		failoverCluster string
		currentVersion  int64
		expectedOut     int64
	}{
		"a new domain, created in c1 already - a failover to c1 where it already is should have no effect": {
			failoverCluster: clusterName1,
			currentVersion:  0,
			expectedOut:     0,
		},
		"a new domain, created in c1 already - a failover to c2 should set the failover version to be based on c2": {
			failoverCluster: clusterName2,
			currentVersion:  0,
			expectedOut:     2,
		},
		"a failover to c3 should set the failover version to be based on c3": {
			failoverCluster: clusterName3,
			currentVersion:  2,
			expectedOut:     4,
		},
		"a subsequent failover back to c1 should increment the failover version by failoverVersionIncrement": {
			failoverCluster: clusterName1,
			currentVersion:  2,
			expectedOut:     100,
		},
		"when the current failover version matches the target cluster it should not increment the failover version": {
			failoverCluster: clusterName1,
			currentVersion:  100,
			expectedOut:     100,
		},
		"and a subsequent fail back over to c2": {
			failoverCluster: clusterName2,
			currentVersion:  100,
			expectedOut:     102,
		},
		"and a subsequent fail back over to c1 should skip over c3": {
			failoverCluster: clusterName1,
			currentVersion:  102,
			expectedOut:     200,
		},
		"Ensuring the behaviour of getNextFailoverVersion can handle negative numbers 1": {
			failoverCluster: clusterName1,
			currentVersion:  -1,
			expectedOut:     0,
		},
		"Ensuring the behaviour of getNextFailoverVersion can handle negative numbers 2": {
			failoverCluster: clusterName2,
			currentVersion:  -1,
			expectedOut:     2,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			m := Metadata{
				failoverVersionIncrement: failoverVersionIncrement,
				allClusters: map[string]config.ClusterInformation{
					clusterName1: {
						InitialFailoverVersion: initialFailoverVersionC1,
					},
					clusterName2: {
						InitialFailoverVersion: initialFailoverVersionC2,
					},
					clusterName3: {
						InitialFailoverVersion: initialFailoverVersionC3,
					},
				},
				versionToClusterName: map[int64]string{
					initialFailoverVersionC1: clusterName1,
					initialFailoverVersionC2: clusterName2,
					initialFailoverVersionC3: clusterName3,
				},
				useNewFailoverVersionOverride: func(domain string) bool { return false },
				metrics:                       metrics.NewNoopMetricsClient().Scope(0),
				log:                           testlogger.New(t),
			}
			assert.Equal(t, td.expectedOut, m.GetNextFailoverVersion(td.failoverCluster, td.currentVersion, "a domain"), name)
		})
	}
}

func TestFailoverVersionLogicIsMonotonic(t *testing.T) {

	const clusterName1 = "c1"
	const initialFailoverVersionC1 = 0
	var minFailoverVersionC1 = int64(1)
	const clusterName2 = "c2"
	const initialFailoverVersionC2 = 2
	var minFailoverVersionC2 = int64(3)

	const failoverVersionIncrement = 100
	const someDomainMigrating = "a domain migrating"
	const someDomainNotMigrating = "a domain not migrating"

	m := Metadata{
		failoverVersionIncrement: failoverVersionIncrement,
		allClusters: map[string]config.ClusterInformation{
			clusterName1: {
				InitialFailoverVersion:    initialFailoverVersionC1,
				NewInitialFailoverVersion: &minFailoverVersionC1,
			},
			clusterName2: {
				InitialFailoverVersion:    initialFailoverVersionC2,
				NewInitialFailoverVersion: &minFailoverVersionC2,
			},
		},
		versionToClusterName: map[int64]string{
			initialFailoverVersionC1: clusterName1,
			initialFailoverVersionC2: clusterName2,
		},
		useNewFailoverVersionOverride: func(domain string) bool { return someDomainMigrating == domain },
		metrics:                       metrics.NewNoopMetricsClient().Scope(0),
		log:                           testlogger.New(t),
	}

	current := int64(0)
	iterations := 0

	for iterations < 4000 {
		next := m.GetNextFailoverVersion(clusterName2, current, someDomainNotMigrating)
		assert.Truef(t, next > current, "current: %d, next %d", current, next)
		current = next

		next = m.GetNextFailoverVersion(clusterName1, current, someDomainNotMigrating)
		assert.Truef(t, next > current, "current: %d, next %d", current, next)
		current = next

		next = m.GetNextFailoverVersion(clusterName1, current, someDomainMigrating)
		assert.Truef(t, next > current, "current: %d, next %d", current, next)
		current = next
		iterations++
	}
}

func TestResolvingClusterVersion(t *testing.T) {
	const clusterName1 = "c1"
	const initialFailoverVersionC1 = 0
	var newInitialFailoverVersionC1 = int64(1)
	const clusterName2 = "c2"
	const initialFailoverVersionC2 = 2
	var newInitialFailoverVersionC2 = int64(3)
	const failoverVersionIncrement = 100

	tests := map[string]struct {
		input          int64
		expectedOutput string
		expectedErr    error
	}{
		"first cluster, normal version": {
			input:          initialFailoverVersionC1,
			expectedOutput: clusterName1,
		},
		"first cluster, minFailover version": {
			input:          newInitialFailoverVersionC1,
			expectedOutput: clusterName1,
		},
		"second cluster, normal version": {
			input:          initialFailoverVersionC2,
			expectedOutput: clusterName2,
		},
		"second cluster, minFailover version": {
			input:          newInitialFailoverVersionC2,
			expectedOutput: clusterName2,
		},

		"first cluster, normal version - higher versions": {
			input:          200,
			expectedOutput: clusterName1,
		},
		"first cluster, minFailover version - higher versions": {
			input:          101,
			expectedOutput: clusterName1,
		},
		"second cluster, normal initialFailover version - higher versions": {
			input:          302,
			expectedOutput: clusterName2,
		},
		"second cluster, minFailover version - higher versions": {
			input:          403,
			expectedOutput: clusterName2,
		},

		"invalid input": {
			input:       599,
			expectedErr: fmt.Errorf("could not resolve failover version: 599"),
		},
	}

	m := Metadata{
		failoverVersionIncrement: failoverVersionIncrement,
		allClusters: map[string]config.ClusterInformation{
			clusterName1: {
				InitialFailoverVersion:    initialFailoverVersionC1,
				NewInitialFailoverVersion: &newInitialFailoverVersionC1,
			},
			clusterName2: {
				InitialFailoverVersion:    initialFailoverVersionC2,
				NewInitialFailoverVersion: &newInitialFailoverVersionC2,
			},
		},
		versionToClusterName: map[int64]string{
			initialFailoverVersionC1: clusterName1,
			initialFailoverVersionC2: clusterName2,
		},
		metrics: metrics.NewNoopMetricsClient().Scope(0),
		log:     log.NewNoop(),
	}

	for name, td := range tests {
		t.Run(name, func(*testing.T) {
			out, err := m.resolveServerName(td.input)
			assert.Equal(t, td.expectedOutput, out)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func TestIsPartOfTheSameCluster(t *testing.T) {

	const clusterName1 = "c1"
	const initialFailoverVersionC1 = 0
	const clusterName2 = "c2"
	const initialFailoverVersionC2 = 2
	const failoverVersionIncrement = 100

	tests := map[string]struct {
		v1                             int64
		v2                             int64
		useMinFailoverVersionAllowance func(string) bool
		expectedResult                 bool
	}{
		"normal case v1 - is from the same cluster": {
			v1:             0,
			v2:             0,
			expectedResult: true,
		},
		"normal case v2 - is from a different cluster": {
			v1:             0,
			v2:             2,
			expectedResult: false,
		},
		"normal case v3 - is from the same cluster": {
			v1:             0,
			v2:             100,
			expectedResult: true,
		},
		"normal case v4 - is from a different cluster": {
			v1:             0,
			v2:             102,
			expectedResult: false,
		},
	}

	for name, td := range tests {
		t.Run(name, func(*testing.T) {
			m := Metadata{
				failoverVersionIncrement: failoverVersionIncrement,
				allClusters: map[string]config.ClusterInformation{
					clusterName1: {
						InitialFailoverVersion: initialFailoverVersionC1,
					},
					clusterName2: {
						InitialFailoverVersion: initialFailoverVersionC2,
					},
				},
				versionToClusterName: map[int64]string{
					initialFailoverVersionC1: clusterName1,
					initialFailoverVersionC2: clusterName2,
				},
				log:     log.NewNoop(),
				metrics: metrics.NewNoopMetricsClient().Scope(0),
			}

			assert.Equal(t, td.expectedResult, m.IsVersionFromSameCluster(td.v1, td.v2), name)
		})

	}
}

func TestIsPartOfTheSameClusterAPIFixing(t *testing.T) {

	const clusterName1 = "c1"
	const initialFailoverVersionC1 = 0
	const clusterName2 = "c2"
	const initialFailoverVersionC2 = 2
	const failoverVersionIncrement = 100

	tests := []struct {
		v0       int64
		v1       int64
		expected bool
	}{
		{v0: 0, v1: 0, expected: true},
		{v0: 0, v1: 1, expected: false},
		{v0: 0, v1: 2, expected: false},
		{v0: 0, v1: 3, expected: false},
		{v0: 0, v1: 4, expected: false},
		{v0: 0, v1: 5, expected: false},
		{v0: 0, v1: 6, expected: false},
		{v0: 0, v1: 7, expected: false},
		{v0: 0, v1: 8, expected: false},
		{v0: 0, v1: 9, expected: false},
		{v0: 0, v1: 10, expected: false},
		{v0: 0, v1: 11, expected: false},
		{v0: 0, v1: 12, expected: false},
		{v0: 0, v1: 13, expected: false},
		{v0: 0, v1: 14, expected: false},
		{v0: 0, v1: 15, expected: false},
		{v0: 0, v1: 16, expected: false},
		{v0: 0, v1: 17, expected: false},
		{v0: 0, v1: 18, expected: false},
		{v0: 0, v1: 19, expected: false},
		{v0: 0, v1: 20, expected: false},
		{v0: 0, v1: 21, expected: false},
		{v0: 0, v1: 22, expected: false},
		{v0: 0, v1: 23, expected: false},
		{v0: 0, v1: 24, expected: false},
		{v0: 0, v1: 25, expected: false},
		{v0: 0, v1: 26, expected: false},
		{v0: 0, v1: 27, expected: false},
		{v0: 0, v1: 28, expected: false},
		{v0: 0, v1: 29, expected: false},
		{v0: 0, v1: 30, expected: false},
		{v0: 0, v1: 31, expected: false},
		{v0: 0, v1: 32, expected: false},
		{v0: 0, v1: 33, expected: false},
		{v0: 0, v1: 34, expected: false},
		{v0: 0, v1: 35, expected: false},
		{v0: 0, v1: 36, expected: false},
		{v0: 0, v1: 37, expected: false},
		{v0: 0, v1: 38, expected: false},
		{v0: 0, v1: 39, expected: false},
		{v0: 0, v1: 40, expected: false},
		{v0: 0, v1: 41, expected: false},
		{v0: 0, v1: 42, expected: false},
		{v0: 0, v1: 43, expected: false},
		{v0: 0, v1: 44, expected: false},
		{v0: 0, v1: 45, expected: false},
		{v0: 0, v1: 46, expected: false},
		{v0: 0, v1: 47, expected: false},
		{v0: 0, v1: 48, expected: false},
		{v0: 0, v1: 49, expected: false},
		{v0: 0, v1: 50, expected: false},
		{v0: 0, v1: 51, expected: false},
		{v0: 0, v1: 52, expected: false},
		{v0: 0, v1: 53, expected: false},
		{v0: 0, v1: 54, expected: false},
		{v0: 0, v1: 55, expected: false},
		{v0: 0, v1: 56, expected: false},
		{v0: 0, v1: 57, expected: false},
		{v0: 0, v1: 58, expected: false},
		{v0: 0, v1: 59, expected: false},
		{v0: 0, v1: 60, expected: false},
		{v0: 0, v1: 61, expected: false},
		{v0: 0, v1: 62, expected: false},
		{v0: 0, v1: 63, expected: false},
		{v0: 0, v1: 64, expected: false},
		{v0: 0, v1: 65, expected: false},
		{v0: 0, v1: 66, expected: false},
		{v0: 0, v1: 67, expected: false},
		{v0: 0, v1: 68, expected: false},
		{v0: 0, v1: 69, expected: false},
		{v0: 0, v1: 70, expected: false},
		{v0: 0, v1: 71, expected: false},
		{v0: 0, v1: 72, expected: false},
		{v0: 0, v1: 73, expected: false},
		{v0: 0, v1: 74, expected: false},
		{v0: 0, v1: 75, expected: false},
		{v0: 0, v1: 76, expected: false},
		{v0: 0, v1: 77, expected: false},
		{v0: 0, v1: 78, expected: false},
		{v0: 0, v1: 79, expected: false},
		{v0: 0, v1: 80, expected: false},
		{v0: 0, v1: 81, expected: false},
		{v0: 0, v1: 82, expected: false},
		{v0: 0, v1: 83, expected: false},
		{v0: 0, v1: 84, expected: false},
		{v0: 0, v1: 85, expected: false},
		{v0: 0, v1: 86, expected: false},
		{v0: 0, v1: 87, expected: false},
		{v0: 0, v1: 88, expected: false},
		{v0: 0, v1: 89, expected: false},
		{v0: 0, v1: 90, expected: false},
		{v0: 0, v1: 91, expected: false},
		{v0: 0, v1: 92, expected: false},
		{v0: 0, v1: 93, expected: false},
		{v0: 0, v1: 94, expected: false},
		{v0: 0, v1: 95, expected: false},
		{v0: 0, v1: 96, expected: false},
		{v0: 0, v1: 97, expected: false},
		{v0: 0, v1: 98, expected: false},
		{v0: 0, v1: 99, expected: false},
		{v0: 0, v1: 100, expected: true},
		{v0: 0, v1: 101, expected: false},
		{v0: 0, v1: 102, expected: false},
		{v0: 0, v1: 103, expected: false},
		{v0: 0, v1: 104, expected: false},
		{v0: 0, v1: 105, expected: false},
		{v0: 0, v1: 106, expected: false},
		{v0: 0, v1: 107, expected: false},
		{v0: 0, v1: 108, expected: false},
		{v0: 0, v1: 109, expected: false},
		{v0: 0, v1: 110, expected: false},
		{v0: 0, v1: 111, expected: false},
		{v0: 0, v1: 112, expected: false},
		{v0: 0, v1: 113, expected: false},
		{v0: 0, v1: 114, expected: false},
		{v0: 0, v1: 115, expected: false},
		{v0: 0, v1: 116, expected: false},
		{v0: 0, v1: 117, expected: false},
		{v0: 0, v1: 118, expected: false},
		{v0: 0, v1: 119, expected: false},
		{v0: 0, v1: 120, expected: false},
		{v0: 0, v1: 121, expected: false},
		{v0: 0, v1: 122, expected: false},
		{v0: 0, v1: 123, expected: false},
		{v0: 0, v1: 124, expected: false},
		{v0: 0, v1: 125, expected: false},
		{v0: 0, v1: 126, expected: false},
		{v0: 0, v1: 127, expected: false},
		{v0: 0, v1: 128, expected: false},
		{v0: 0, v1: 129, expected: false},
		{v0: 0, v1: 130, expected: false},
		{v0: 0, v1: 131, expected: false},
		{v0: 0, v1: 132, expected: false},
		{v0: 0, v1: 133, expected: false},
		{v0: 0, v1: 134, expected: false},
		{v0: 0, v1: 135, expected: false},
		{v0: 0, v1: 136, expected: false},
		{v0: 0, v1: 137, expected: false},
		{v0: 0, v1: 138, expected: false},
		{v0: 0, v1: 139, expected: false},
		{v0: 0, v1: 140, expected: false},
		{v0: 0, v1: 141, expected: false},
		{v0: 0, v1: 142, expected: false},
		{v0: 0, v1: 143, expected: false},
		{v0: 0, v1: 144, expected: false},
		{v0: 0, v1: 145, expected: false},
		{v0: 0, v1: 146, expected: false},
		{v0: 0, v1: 147, expected: false},
		{v0: 0, v1: 148, expected: false},
		{v0: 0, v1: 149, expected: false},
		{v0: 0, v1: 150, expected: false},
		{v0: 0, v1: 151, expected: false},
		{v0: 0, v1: 152, expected: false},
		{v0: 0, v1: 153, expected: false},
		{v0: 0, v1: 154, expected: false},
		{v0: 0, v1: 155, expected: false},
		{v0: 0, v1: 156, expected: false},
		{v0: 0, v1: 157, expected: false},
		{v0: 0, v1: 158, expected: false},
		{v0: 0, v1: 159, expected: false},
		{v0: 0, v1: 160, expected: false},
		{v0: 0, v1: 161, expected: false},
		{v0: 0, v1: 162, expected: false},
		{v0: 0, v1: 163, expected: false},
		{v0: 0, v1: 164, expected: false},
		{v0: 0, v1: 165, expected: false},
		{v0: 0, v1: 166, expected: false},
		{v0: 0, v1: 167, expected: false},
		{v0: 0, v1: 168, expected: false},
		{v0: 0, v1: 169, expected: false},
		{v0: 0, v1: 170, expected: false},
		{v0: 0, v1: 171, expected: false},
		{v0: 0, v1: 172, expected: false},
		{v0: 0, v1: 173, expected: false},
		{v0: 0, v1: 174, expected: false},
		{v0: 0, v1: 175, expected: false},
		{v0: 0, v1: 176, expected: false},
		{v0: 0, v1: 177, expected: false},
		{v0: 0, v1: 178, expected: false},
		{v0: 0, v1: 179, expected: false},
		{v0: 0, v1: 180, expected: false},
		{v0: 0, v1: 181, expected: false},
		{v0: 0, v1: 182, expected: false},
		{v0: 0, v1: 183, expected: false},
		{v0: 0, v1: 184, expected: false},
		{v0: 0, v1: 185, expected: false},
		{v0: 0, v1: 186, expected: false},
		{v0: 0, v1: 187, expected: false},
		{v0: 0, v1: 188, expected: false},
		{v0: 0, v1: 189, expected: false},
		{v0: 0, v1: 190, expected: false},
		{v0: 0, v1: 191, expected: false},
		{v0: 0, v1: 192, expected: false},
		{v0: 0, v1: 193, expected: false},
		{v0: 0, v1: 194, expected: false},
		{v0: 0, v1: 195, expected: false},
		{v0: 0, v1: 196, expected: false},
		{v0: 0, v1: 197, expected: false},
		{v0: 0, v1: 198, expected: false},
		{v0: 0, v1: 199, expected: false},
		{v0: 0, v1: 200, expected: true},
	}

	m := Metadata{
		failoverVersionIncrement: failoverVersionIncrement,
		allClusters: map[string]config.ClusterInformation{
			clusterName1: {
				InitialFailoverVersion: initialFailoverVersionC1,
			},
			clusterName2: {
				InitialFailoverVersion: initialFailoverVersionC2,
			},
		},
		versionToClusterName: map[int64]string{
			initialFailoverVersionC1: clusterName1,
			initialFailoverVersionC2: clusterName2,
		},
		useNewFailoverVersionOverride: func(domain string) bool { return false },
		metrics:                       metrics.NewNoopMetricsClient().Scope(0),
		log:                           log.NewNoop(),
	}

	for i := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			assert.Equal(t, tests[i].expected, m.IsVersionFromSameCluster(tests[i].v0, tests[i].v1))
		})
	}
}

func TestClusterNameForFailoverVersion(t *testing.T) {

	const clusterName1 = "c1"
	const initialFailoverVersionC1 = 0
	const clusterName2 = "c2"
	const initialFailoverVersionC2 = 2
	const failoverVersionIncrement = 100

	tests := []struct {
		v0       int64
		v1       int64
		expected bool
	}{
		{v0: 0, v1: 0, expected: true},
		{v0: 0, v1: 1, expected: false},
		{v0: 0, v1: 2, expected: false},
		{v0: 0, v1: 3, expected: false},
		{v0: 0, v1: 4, expected: false},
		{v0: 0, v1: 5, expected: false},
		{v0: 0, v1: 6, expected: false},
		{v0: 0, v1: 7, expected: false},
		{v0: 0, v1: 8, expected: false},
		{v0: 0, v1: 9, expected: false},
		{v0: 0, v1: 10, expected: false},
		{v0: 0, v1: 11, expected: false},
		{v0: 0, v1: 12, expected: false},
		{v0: 0, v1: 13, expected: false},
		{v0: 0, v1: 14, expected: false},
		{v0: 0, v1: 15, expected: false},
		{v0: 0, v1: 16, expected: false},
		{v0: 0, v1: 17, expected: false},
		{v0: 0, v1: 18, expected: false},
		{v0: 0, v1: 19, expected: false},
		{v0: 0, v1: 20, expected: false},
		{v0: 0, v1: 21, expected: false},
		{v0: 0, v1: 22, expected: false},
		{v0: 0, v1: 23, expected: false},
		{v0: 0, v1: 24, expected: false},
		{v0: 0, v1: 25, expected: false},
		{v0: 0, v1: 26, expected: false},
		{v0: 0, v1: 27, expected: false},
		{v0: 0, v1: 28, expected: false},
		{v0: 0, v1: 29, expected: false},
		{v0: 0, v1: 30, expected: false},
		{v0: 0, v1: 31, expected: false},
		{v0: 0, v1: 32, expected: false},
		{v0: 0, v1: 33, expected: false},
		{v0: 0, v1: 34, expected: false},
		{v0: 0, v1: 35, expected: false},
		{v0: 0, v1: 36, expected: false},
		{v0: 0, v1: 37, expected: false},
		{v0: 0, v1: 38, expected: false},
		{v0: 0, v1: 39, expected: false},
		{v0: 0, v1: 40, expected: false},
		{v0: 0, v1: 41, expected: false},
		{v0: 0, v1: 42, expected: false},
		{v0: 0, v1: 43, expected: false},
		{v0: 0, v1: 44, expected: false},
		{v0: 0, v1: 45, expected: false},
		{v0: 0, v1: 46, expected: false},
		{v0: 0, v1: 47, expected: false},
		{v0: 0, v1: 48, expected: false},
		{v0: 0, v1: 49, expected: false},
		{v0: 0, v1: 50, expected: false},
		{v0: 0, v1: 51, expected: false},
		{v0: 0, v1: 52, expected: false},
		{v0: 0, v1: 53, expected: false},
		{v0: 0, v1: 54, expected: false},
		{v0: 0, v1: 55, expected: false},
		{v0: 0, v1: 56, expected: false},
		{v0: 0, v1: 57, expected: false},
		{v0: 0, v1: 58, expected: false},
		{v0: 0, v1: 59, expected: false},
		{v0: 0, v1: 60, expected: false},
		{v0: 0, v1: 61, expected: false},
		{v0: 0, v1: 62, expected: false},
		{v0: 0, v1: 63, expected: false},
		{v0: 0, v1: 64, expected: false},
		{v0: 0, v1: 65, expected: false},
		{v0: 0, v1: 66, expected: false},
		{v0: 0, v1: 67, expected: false},
		{v0: 0, v1: 68, expected: false},
		{v0: 0, v1: 69, expected: false},
		{v0: 0, v1: 70, expected: false},
		{v0: 0, v1: 71, expected: false},
		{v0: 0, v1: 72, expected: false},
		{v0: 0, v1: 73, expected: false},
		{v0: 0, v1: 74, expected: false},
		{v0: 0, v1: 75, expected: false},
		{v0: 0, v1: 76, expected: false},
		{v0: 0, v1: 77, expected: false},
		{v0: 0, v1: 78, expected: false},
		{v0: 0, v1: 79, expected: false},
		{v0: 0, v1: 80, expected: false},
		{v0: 0, v1: 81, expected: false},
		{v0: 0, v1: 82, expected: false},
		{v0: 0, v1: 83, expected: false},
		{v0: 0, v1: 84, expected: false},
		{v0: 0, v1: 85, expected: false},
		{v0: 0, v1: 86, expected: false},
		{v0: 0, v1: 87, expected: false},
		{v0: 0, v1: 88, expected: false},
		{v0: 0, v1: 89, expected: false},
		{v0: 0, v1: 90, expected: false},
		{v0: 0, v1: 91, expected: false},
		{v0: 0, v1: 92, expected: false},
		{v0: 0, v1: 93, expected: false},
		{v0: 0, v1: 94, expected: false},
		{v0: 0, v1: 95, expected: false},
		{v0: 0, v1: 96, expected: false},
		{v0: 0, v1: 97, expected: false},
		{v0: 0, v1: 98, expected: false},
		{v0: 0, v1: 99, expected: false},
		{v0: 0, v1: 100, expected: true},
		{v0: 0, v1: 101, expected: false},
		{v0: 0, v1: 102, expected: false},
		{v0: 0, v1: 103, expected: false},
		{v0: 0, v1: 104, expected: false},
		{v0: 0, v1: 105, expected: false},
		{v0: 0, v1: 106, expected: false},
		{v0: 0, v1: 107, expected: false},
		{v0: 0, v1: 108, expected: false},
		{v0: 0, v1: 109, expected: false},
		{v0: 0, v1: 110, expected: false},
		{v0: 0, v1: 111, expected: false},
		{v0: 0, v1: 112, expected: false},
		{v0: 0, v1: 113, expected: false},
		{v0: 0, v1: 114, expected: false},
		{v0: 0, v1: 115, expected: false},
		{v0: 0, v1: 116, expected: false},
		{v0: 0, v1: 117, expected: false},
		{v0: 0, v1: 118, expected: false},
		{v0: 0, v1: 119, expected: false},
		{v0: 0, v1: 120, expected: false},
		{v0: 0, v1: 121, expected: false},
		{v0: 0, v1: 122, expected: false},
		{v0: 0, v1: 123, expected: false},
		{v0: 0, v1: 124, expected: false},
		{v0: 0, v1: 125, expected: false},
		{v0: 0, v1: 126, expected: false},
		{v0: 0, v1: 127, expected: false},
		{v0: 0, v1: 128, expected: false},
		{v0: 0, v1: 129, expected: false},
		{v0: 0, v1: 130, expected: false},
		{v0: 0, v1: 131, expected: false},
		{v0: 0, v1: 132, expected: false},
		{v0: 0, v1: 133, expected: false},
		{v0: 0, v1: 134, expected: false},
		{v0: 0, v1: 135, expected: false},
		{v0: 0, v1: 136, expected: false},
		{v0: 0, v1: 137, expected: false},
		{v0: 0, v1: 138, expected: false},
		{v0: 0, v1: 139, expected: false},
		{v0: 0, v1: 140, expected: false},
		{v0: 0, v1: 141, expected: false},
		{v0: 0, v1: 142, expected: false},
		{v0: 0, v1: 143, expected: false},
		{v0: 0, v1: 144, expected: false},
		{v0: 0, v1: 145, expected: false},
		{v0: 0, v1: 146, expected: false},
		{v0: 0, v1: 147, expected: false},
		{v0: 0, v1: 148, expected: false},
		{v0: 0, v1: 149, expected: false},
		{v0: 0, v1: 150, expected: false},
		{v0: 0, v1: 151, expected: false},
		{v0: 0, v1: 152, expected: false},
		{v0: 0, v1: 153, expected: false},
		{v0: 0, v1: 154, expected: false},
		{v0: 0, v1: 155, expected: false},
		{v0: 0, v1: 156, expected: false},
		{v0: 0, v1: 157, expected: false},
		{v0: 0, v1: 158, expected: false},
		{v0: 0, v1: 159, expected: false},
		{v0: 0, v1: 160, expected: false},
		{v0: 0, v1: 161, expected: false},
		{v0: 0, v1: 162, expected: false},
		{v0: 0, v1: 163, expected: false},
		{v0: 0, v1: 164, expected: false},
		{v0: 0, v1: 165, expected: false},
		{v0: 0, v1: 166, expected: false},
		{v0: 0, v1: 167, expected: false},
		{v0: 0, v1: 168, expected: false},
		{v0: 0, v1: 169, expected: false},
		{v0: 0, v1: 170, expected: false},
		{v0: 0, v1: 171, expected: false},
		{v0: 0, v1: 172, expected: false},
		{v0: 0, v1: 173, expected: false},
		{v0: 0, v1: 174, expected: false},
		{v0: 0, v1: 175, expected: false},
		{v0: 0, v1: 176, expected: false},
		{v0: 0, v1: 177, expected: false},
		{v0: 0, v1: 178, expected: false},
		{v0: 0, v1: 179, expected: false},
		{v0: 0, v1: 180, expected: false},
		{v0: 0, v1: 181, expected: false},
		{v0: 0, v1: 182, expected: false},
		{v0: 0, v1: 183, expected: false},
		{v0: 0, v1: 184, expected: false},
		{v0: 0, v1: 185, expected: false},
		{v0: 0, v1: 186, expected: false},
		{v0: 0, v1: 187, expected: false},
		{v0: 0, v1: 188, expected: false},
		{v0: 0, v1: 189, expected: false},
		{v0: 0, v1: 190, expected: false},
		{v0: 0, v1: 191, expected: false},
		{v0: 0, v1: 192, expected: false},
		{v0: 0, v1: 193, expected: false},
		{v0: 0, v1: 194, expected: false},
		{v0: 0, v1: 195, expected: false},
		{v0: 0, v1: 196, expected: false},
		{v0: 0, v1: 197, expected: false},
		{v0: 0, v1: 198, expected: false},
		{v0: 0, v1: 199, expected: false},
		{v0: 0, v1: 200, expected: true},
	}

	m := Metadata{
		failoverVersionIncrement: failoverVersionIncrement,
		allClusters: map[string]config.ClusterInformation{
			clusterName1: {
				InitialFailoverVersion: initialFailoverVersionC1,
			},
			clusterName2: {
				InitialFailoverVersion: initialFailoverVersionC2,
			},
		},
		versionToClusterName: map[int64]string{
			initialFailoverVersionC1: clusterName1,
			initialFailoverVersionC2: clusterName2,
		},
		metrics:                       metrics.NewNoopMetricsClient().Scope(0),
		useNewFailoverVersionOverride: func(domain string) bool { return false },
		log:                           log.NewNoop(),
	}

	for i := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			assert.Equal(t, tests[i].expected, m.IsVersionFromSameCluster(tests[i].v0, tests[i].v1))
		})
	}
}

func TestServerResolution(t *testing.T) {
	const clusterName1 = "c1"
	const initialFailoverVersionC1 = 0
	const clusterName2 = "c2"
	const initialFailoverVersionC2 = 2
	const failoverVersionIncrement = 100
	const domainToMigrate = "some domain"

	allClusters := map[string]config.ClusterInformation{
		clusterName1: {
			InitialFailoverVersion: initialFailoverVersionC1,
		},
		clusterName2: {
			InitialFailoverVersion: initialFailoverVersionC2,
		},
	}

	impl := Metadata{
		failoverVersionIncrement: failoverVersionIncrement,
		allClusters:              allClusters,
		versionToClusterName: map[int64]string{
			initialFailoverVersionC1: clusterName1,
			initialFailoverVersionC2: clusterName2,
		},
		metrics:                       metrics.NewNoopMetricsClient().Scope(0),
		useNewFailoverVersionOverride: func(domain string) bool { return domain == domainToMigrate },
		log:                           log.NewNoop(),
	}

	err := quick.Check(func(currentFOVersion int64, migrateDomain bool) bool {
		var nextFailoverVersion int64
		if migrateDomain {
			fo := impl.GetNextFailoverVersion(clusterName1, currentFOVersion, domainToMigrate)
			nextFailoverVersion = fo
		} else {
			fo := impl.GetNextFailoverVersion(clusterName1, currentFOVersion, "a different domain")
			nextFailoverVersion = fo
		}
		// do a round-trip
		clusterNameResolved, err := impl.ClusterNameForFailoverVersion(nextFailoverVersion)
		require.NoError(t, err)
		return clusterName1 == clusterNameResolved
	}, &quick.Config{})
	assert.NoError(t, err)
}

// the point of this test is to assert that there's no clumsy errors, and
// in an unmigrated state, the old implementation for getting versions and the new
// one are equal
func TestNoChangesInUnmigratedState(t *testing.T) {
	const clusterName1 = "c1"
	const initialFailoverVersionC1 = 0
	const clusterName2 = "c2"
	const initialFailoverVersionC2 = 2
	const failoverVersionIncrement = 100

	allClusters := map[string]config.ClusterInformation{
		clusterName1: {
			InitialFailoverVersion: initialFailoverVersionC1,
		},
		clusterName2: {
			InitialFailoverVersion: initialFailoverVersionC2,
		},
	}

	oldImpl := func(cluster string, currentFailoverVersion int64) int64 {
		info, ok := allClusters[cluster]
		if !ok {
			panic(fmt.Sprintf(
				"Unknown cluster name: %v with given cluster initial failover version map: %v.",
				cluster,
				allClusters,
			))
		}
		failoverVersion := currentFailoverVersion/failoverVersionIncrement*failoverVersionIncrement + info.InitialFailoverVersion
		if failoverVersion < currentFailoverVersion {
			return failoverVersion + failoverVersionIncrement
		}
		return failoverVersion
	}

	newImpl := Metadata{
		failoverVersionIncrement: failoverVersionIncrement,
		allClusters:              allClusters,
		versionToClusterName: map[int64]string{
			initialFailoverVersionC1: clusterName1,
			initialFailoverVersionC2: clusterName2,
		},
		metrics:                       metrics.NewNoopMetricsClient().Scope(0),
		useNewFailoverVersionOverride: func(domain string) bool { return false },
		log:                           log.NewNoop(),
	}

	err := quick.CheckEqual(func(currVersion int64) int64 {
		// partially apply the cluster-name, since the fuzzer trying to guess that is pointless
		return oldImpl(clusterName2, currVersion)
	}, func(currentVersion int64) int64 {
		return newImpl.GetNextFailoverVersion(clusterName2, currentVersion, "some domain") // all domains are turned off
	},
		&quick.Config{})
	assert.NoError(t, err)
}

// the point of this test is to assert that there's no clumsy errors, and
// in an unmigrated state, the old implementation for getting versions and the new
// one are equal
func TestFailoverVersionResolution(t *testing.T) {
	const clusterName1 = "c1"
	const initialFailoverVersionC1 = 0
	var newFailoverVersionC1 = int64(1)
	const clusterName2 = "c2"
	const initialFailoverVersionC2 = 2
	const failoverVersionIncrement = 100

	allClusters := map[string]config.ClusterInformation{
		clusterName1: {
			InitialFailoverVersion:    initialFailoverVersionC1,
			NewInitialFailoverVersion: &newFailoverVersionC1,
		},
		clusterName2: {
			InitialFailoverVersion: initialFailoverVersionC2,
		},
	}

	sut := Metadata{
		failoverVersionIncrement: failoverVersionIncrement,
		allClusters:              allClusters,
		versionToClusterName: map[int64]string{
			initialFailoverVersionC1: clusterName1,
			initialFailoverVersionC2: clusterName2,
		},
		metrics:                       metrics.NewNoopMetricsClient().Scope(0),
		useNewFailoverVersionOverride: func(domain string) bool { return false },
		log:                           log.NewNoop(),
	}

	tests := map[string]struct {
		in          int64
		expectedOut string
		expectedErr error
	}{
		"normal failover version": {
			in:          initialFailoverVersionC1,
			expectedOut: clusterName1,
		},
		"New failover version": {
			in:          initialFailoverVersionC1,
			expectedOut: clusterName1,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			out, err := sut.ClusterNameForFailoverVersion(td.in)
			assert.NoError(t, err)
			assert.Equal(t, td.expectedOut, out)
		})
	}
}

func TestGetters(t *testing.T) {
	m := NewMetadata(
		config.ClusterGroupMetadata{
			ClusterGroup: map[string]config.ClusterInformation{
				"cluster0": {
					InitialFailoverVersion: 1,
					Enabled:                true,
				},
				"cluster1": {
					InitialFailoverVersion: 3,
					Enabled:                true,
				},
				"cluster2": {
					InitialFailoverVersion: 5,
					Enabled:                false,
				},
			},
			FailoverVersionIncrement: 100,
			CurrentClusterName:       "cluster0",
			PrimaryClusterName:       "cluster0",
		},
		func(d string) bool { return false },
		metrics.NewNoopMetricsClient(),
		log.NewNoop(),
	)

	assert.True(t, m.IsPrimaryCluster())
	assert.Equal(t, "cluster0", m.GetCurrentClusterName())

	// do existence checks
	assert.Equal(t, []string{"cluster0", "cluster1", "cluster2"}, keysOfClusterInfoMap(m.GetAllClusterInfo()))
	assert.Equal(t, []string{"cluster0", "cluster1"}, keysOfClusterInfoMap(m.GetEnabledClusterInfo()))
	assert.Equal(t, []string{"cluster1"}, keysOfClusterInfoMap(m.GetRemoteClusterInfo()))
}

func keysOfClusterInfoMap(m map[string]config.ClusterInformation) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
