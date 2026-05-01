// Copyright (c) 2021 Uber Technologies, Inc.
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

package ndc

import (
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/persistence-tests/testcluster"
	"github.com/uber/cadence/common/testing"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/host"
)

// NOTE: the following definitions can't be defined in *_test.go
// since they need to be exported and used by our internal tests

type (
	NDCIntegrationTestSuite struct {
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
		suite.Suite
		active     *host.TestCluster
		generator  testing.Generator
		serializer persistence.PayloadSerializer
		logger     log.Logger

		domainName                  string
		domainID                    string
		version                     int64
		versionIncrement            int64
		mockAdminClient             map[string]admin.Client
		standByReplicationTasksChan chan *types.ReplicationTask
		standByTaskID               int64

		clusterConfigs        []*host.TestClusterConfig
		defaultTestCluster    testcluster.PersistenceTestCluster
		VisibilityTestCluster testcluster.PersistenceTestCluster
	}

	NDCIntegrationTestSuiteParams struct {
		ClusterConfigs        []*host.TestClusterConfig
		DefaultTestCluster    testcluster.PersistenceTestCluster
		VisibilityTestCluster testcluster.PersistenceTestCluster
	}
)

func NewNDCIntegrationTestSuite(params NDCIntegrationTestSuiteParams) *NDCIntegrationTestSuite {
	return &NDCIntegrationTestSuite{
		clusterConfigs:        params.ClusterConfigs,
		defaultTestCluster:    params.DefaultTestCluster,
		VisibilityTestCluster: params.VisibilityTestCluster,
	}
}
