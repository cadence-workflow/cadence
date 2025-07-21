// Copyright (c) 2018 Uber Technologies, Inc.
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

package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/persistence"
)

func TestGetOrUseDefaultActiveCluster(t *testing.T) {
	tests := []struct {
		name            string
		currentCluster  string
		activeCluster   string
		expectedCluster string
	}{
		{
			name:            "empty active cluster",
			currentCluster:  "cluster1",
			activeCluster:   "",
			expectedCluster: "cluster1",
		},
		{
			name:            "non-empty active cluster",
			currentCluster:  "cluster1",
			activeCluster:   "cluster2",
			expectedCluster: "cluster2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetOrUseDefaultActiveCluster(tt.currentCluster, tt.activeCluster)
			assert.Equal(t, tt.expectedCluster, got)
		})
	}
}

func TestGetOrUseDefaultClusters(t *testing.T) {
	tests := []struct {
		name             string
		currentCluster   string
		clusters         []*persistence.ClusterReplicationConfig
		expectedClusters []*persistence.ClusterReplicationConfig
	}{
		{
			name:           "empty clusters",
			currentCluster: "cluster1",
			clusters:       []*persistence.ClusterReplicationConfig{},
			expectedClusters: []*persistence.ClusterReplicationConfig{
				&persistence.ClusterReplicationConfig{
					ClusterName: "cluster1",
				},
			},
		},
		{
			name:           "non-empty clusters",
			currentCluster: "cluster1",
			clusters: []*persistence.ClusterReplicationConfig{
				&persistence.ClusterReplicationConfig{
					ClusterName: "cluster2",
				},
			},
			expectedClusters: []*persistence.ClusterReplicationConfig{
				&persistence.ClusterReplicationConfig{
					ClusterName: "cluster2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetOrUseDefaultClusters(tt.currentCluster, tt.clusters)
			assert.Equal(t, tt.expectedClusters, got)
		})
	}
}
