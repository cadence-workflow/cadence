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

	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := new(WorkflowIDRateLimitIntegrationSuite)
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *WorkflowIDRateLimitIntegrationSuite) SetupSuite() {
	s.SetupLogger()

	s.Logger.Info("Running integration test against test cluster")
	clusterMetadata := NewClusterMetadata(s.T(), s.TestClusterConfig)
	dc := persistence.DynamicConfiguration{
		EnableCassandraAllConsistencyLevelDelete: dynamicproperties.GetBoolPropertyFn(true),
		PersistenceSampleLoggingRate:             dynamicproperties.GetIntPropertyFn(100),
		EnableShardIDMetrics:                     dynamicproperties.GetBoolPropertyFn(true),
		EnableHistoryTaskDualWriteMode:           dynamicproperties.GetBoolPropertyFn(true),
		ReadNoSQLHistoryTaskFromDataBlob:         dynamicproperties.GetBoolPropertyFn(false),
		SerializationEncoding:                    dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		ReadNoSQLShardFromDataBlob:               dynamicproperties.GetBoolPropertyFn(true),
		HistoryNodeDeleteBatchSize:               dynamicproperties.GetIntPropertyFn(1000),
	}
	params := pt.TestBaseParams{
		DefaultTestCluster:    s.DefaultTestCluster,
		VisibilityTestCluster: s.VisibilityTestCluster,
		ClusterMetadata:       clusterMetadata,
		DynamicConfiguration:  dc,
	}
	cluster, err := NewCluster(s.T(), s.TestClusterConfig, s.Logger, params)
	s.Require().NoError(err)
	s.TestCluster = cluster
	s.Engine = s.TestCluster.GetFrontendClient()
	s.AdminClient = s.TestCluster.GetAdminClient()

	s.DomainName = s.RandomizeStr("integration-test-domain")
	s.Require().NoError(s.RegisterDomain(s.DomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))

	s.domainCacheRefresh()
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

	// The ratelimit is 5 per second with a burst of 5, so we should be able to start 5 workflows without any error
	for i := 0; i < 5; i++ {
		_, err := s.Engine.StartWorkflowExecution(ctx, request)
		assert.NoError(s.T(), err)
	}

	// Now we should get a rate limit error (with some fuzziness for time passing)
	limited := 0
	for i := 0; i < 5; i++ {
		_, err := s.Engine.StartWorkflowExecution(ctx, request)
		var busyErr *types.ServiceBusyError
		if err != nil {
			if assert.ErrorAs(s.T(), err, &busyErr) {
				limited++
				assert.Equal(s.T(), constants.WorkflowIDRateLimitReason, busyErr.Reason)
			}
		}
	}
	// 5 fails occasionally, trying 4.  If needed, reduce to 3 or find a way to
	// make this test less sensitive to latency, as test-runner hosts vary a lot.
	assert.GreaterOrEqual(s.T(), limited, 4, "should have encountered some rate-limit errors after the burst was exhausted")

	// After 1 second (200ms at a minimum) we should be able to start more workflows without being limited
	time.Sleep(1 * time.Second)
	_, err := s.Engine.StartWorkflowExecution(ctx, request)
	assert.NoError(s.T(), err)
}
