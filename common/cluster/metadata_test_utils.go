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
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	commonMetrics "github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

const (
	// TestCurrentClusterInitialFailoverVersion is initial failover version for current cluster
	TestCurrentClusterInitialFailoverVersion = int64(0)
	// TestAlternativeClusterInitialFailoverVersion is initial failover version for alternative cluster
	TestAlternativeClusterInitialFailoverVersion = int64(1)
	// TestDisabledClusterInitialFailoverVersion is initial failover version for disabled cluster
	TestDisabledClusterInitialFailoverVersion = int64(2)
	// TestFailoverVersionIncrement is failover version increment used for test
	TestFailoverVersionIncrement = int64(10)
	// TestCurrentClusterName is current cluster used for test
	TestCurrentClusterName = "active"
	// TestAlternativeClusterName is alternative cluster used for test
	TestAlternativeClusterName = "standby"
	// TestDisabledClusterName is disabled cluster used for test
	TestDisabledClusterName = "disabled"
	// TestRegion1 is region1 used for test
	TestRegion1 = "region1"
	// TestRegion2 is region2 used for test
	TestRegion2 = "region2"
	// TestCurrentClusterFrontendAddress is the ip port address of current cluster
	TestCurrentClusterFrontendAddress = "127.0.0.1:7104"
	// TestAlternativeClusterFrontendAddress is the ip port address of alternative cluster
	TestAlternativeClusterFrontendAddress = "127.0.0.1:8104"
	// TestClusterXDCTransport is the RPC transport used for XDC traffic <tchannel|grpc>
	TestClusterXDCTransport = "grpc"
)

var (
	// TestAllClusterNames is the all cluster names used for test
	TestAllClusterNames = []string{TestCurrentClusterName, TestAlternativeClusterName}
	// TestAllClusterInfo is the same as above, just convenient for test mocking
	TestAllClusterInfo = map[string]config.ClusterInformation{
		TestCurrentClusterName: {
			Enabled:                true,
			InitialFailoverVersion: TestCurrentClusterInitialFailoverVersion,
			RPCName:                service.Frontend,
			RPCAddress:             TestCurrentClusterFrontendAddress,
			RPCTransport:           TestClusterXDCTransport,
		},
		TestAlternativeClusterName: {
			Enabled:                true,
			InitialFailoverVersion: TestAlternativeClusterInitialFailoverVersion,
			RPCName:                service.Frontend,
			RPCAddress:             TestAlternativeClusterFrontendAddress,
			RPCTransport:           TestClusterXDCTransport,
		},
		TestDisabledClusterName: {
			Enabled:                false,
			InitialFailoverVersion: TestDisabledClusterInitialFailoverVersion,
		},
	}

	// TestSingleDCAllClusterNames is the all cluster names used for test
	TestSingleDCAllClusterNames = []string{TestCurrentClusterName}
	// TestSingleDCClusterInfo is the same as above, just convenient for test mocking
	TestSingleDCClusterInfo = map[string]config.ClusterInformation{
		TestCurrentClusterName: {
			Enabled:                true,
			InitialFailoverVersion: TestCurrentClusterInitialFailoverVersion,
			RPCName:                service.Frontend,
			RPCAddress:             TestCurrentClusterFrontendAddress,
			RPCTransport:           TestClusterXDCTransport,
		},
	}

	// TestActiveClusterMetadata is metadata for an active cluster
	TestActiveClusterMetadata = NewMetadata(
		config.ClusterGroupMetadata{
			FailoverVersionIncrement: TestFailoverVersionIncrement,
			PrimaryClusterName:       TestCurrentClusterName,
			CurrentClusterName:       TestCurrentClusterName,
			ClusterGroup:             TestAllClusterInfo,
		},
		func(d string) bool { return false },
		commonMetrics.NewNoopMetricsClient(),
		log.NewNoop(),
	)

	// TestPassiveClusterMetadata is metadata for a passive cluster
	TestPassiveClusterMetadata = NewMetadata(
		config.ClusterGroupMetadata{
			FailoverVersionIncrement: TestFailoverVersionIncrement,
			PrimaryClusterName:       TestCurrentClusterName,
			CurrentClusterName:       TestAlternativeClusterName,
			ClusterGroup:             TestAllClusterInfo,
		},
		func(d string) bool { return false },
		commonMetrics.NewNoopMetricsClient(),
		log.NewNoop(),
	)
)

// GetTestClusterMetadata return an cluster metadata instance, which is initialized
func GetTestClusterMetadata(isPrimaryCluster bool) Metadata {
	primaryClusterName := TestCurrentClusterName
	if !isPrimaryCluster {
		primaryClusterName = TestAlternativeClusterName
	}

	return NewMetadata(
		config.ClusterGroupMetadata{
			FailoverVersionIncrement: TestFailoverVersionIncrement,
			PrimaryClusterName:       primaryClusterName,
			CurrentClusterName:       TestCurrentClusterName,
			ClusterGroup:             TestAllClusterInfo,
		},
		func(d string) bool { return false },
		commonMetrics.NewNoopMetricsClient(),
		log.NewNoop(),
	)
}
