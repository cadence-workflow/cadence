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

package host

import (
	"flag"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/types"
)

func TestWorkflowIDRateLimitIntegrationSuite(t *testing.T) {
	// Loads the flags for persistence etc., if none are given they are set in ./flag.go
	flag.Parse()

	clusterConfig, err := GetTestClusterConfig("testdata/integration_wfidratelimit_cluster.yaml")
	require.NoError(t, err)

	clusterConfig.TimeSource = clock.NewMockedTimeSource()
	clusterConfig.HistoryDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.WorkflowIDExternalRPS: 5,
	}

	testCluster := NewTestPersistenceConfig(t)

	s := new(WorkflowIDRateLimitIntegrationSuite)
	params := IntegrationBaseParams{
		PersistenceConfig: testCluster,
		TestClusterConfig: clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *WorkflowIDRateLimitIntegrationSuite) SetupSuite() {
	s.SetupLogger()

	s.Logger.Info("Running integration test against test cluster")
	clusterMetadata := NewClusterMetadata(s.T(), s.TestClusterConfig)
	dc := *persistence.NewDefaultDynamicConfiguration()
	dc.EnableCassandraAllConsistencyLevelDelete = dynamicproperties.GetBoolPropertyFn(true)
	dc.EnableHistoryTaskDualWriteMode = dynamicproperties.GetBoolPropertyFn(true)
	params := pt.TestBaseParams{
		PersistenceConfig:    s.PersistenceConfig,
		ClusterMetadata:      &clusterMetadata,
		DynamicConfiguration: &dc,
	}
	cluster, err := NewCluster(s.T(), s.TestClusterConfig, s.Logger, params)
	s.Require().NoError(err)
	s.TestCluster = cluster
	s.Engine = s.TestCluster.GetFrontendClient()
	s.AdminClient = s.TestCluster.GetAdminClient()

	s.DomainName = s.RandomizeStr("integration-test-domain")
	s.Require().NoError(s.RegisterDomain(s.DomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))

	s.domainCacheRefresh(s.DomainName)
}

func (s *WorkflowIDRateLimitIntegrationSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *WorkflowIDRateLimitIntegrationSuite) TearDownSuite() {
	s.TearDownBaseSuite()
}

func (s *WorkflowIDRateLimitIntegrationSuite) TestWorkflowIDSpecificRateLimits() {
	const (
		testWorkflowID   = "integration-workflow-specific-rate-limit-test"
		testWorkflowType = "integration-workflow-specific-rate-limit-test-type"
		testTaskListName = "integration-workflow-specific-rate-limit-test-taskList"
		testIdentity     = "worker1"
	)

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          testWorkflowID,
		WorkflowType:                        &types.WorkflowType{Name: testWorkflowType},
		TaskList:                            &types.TaskList{Name: testTaskListName},
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            testIdentity,

		WorkflowIDReusePolicy: types.WorkflowIDReusePolicyTerminateIfRunning.Ptr(),
	}

	ctx, cancel := createContext()
	defer cancel()

	// History's rate limiter runs on the cluster's mocked time source, so time only moves when the test advances it.
	// The ratelimit is 5 per second with a burst of 5, so the first 5 starts are allowed and the next 5 are limited.
	for i := 0; i < 5; i++ {
		_, err := s.Engine.StartWorkflowExecution(ctx, request)
		assert.NoError(s.T(), err)
	}
	for i := 0; i < 5; i++ {
		_, err := s.Engine.StartWorkflowExecution(ctx, request)
		var busyErr *types.ServiceBusyError
		if assert.ErrorAs(s.T(), err, &busyErr) {
			assert.Equal(s.T(), constants.WorkflowIDRateLimitReason, busyErr.Reason)
		}
	}

	// After 1 second the bucket has refilled, so we can start more workflows.
	s.TestClusterConfig.TimeSource.Advance(time.Second)
	_, err := s.Engine.StartWorkflowExecution(ctx, request)
	assert.NoError(s.T(), err)
}
