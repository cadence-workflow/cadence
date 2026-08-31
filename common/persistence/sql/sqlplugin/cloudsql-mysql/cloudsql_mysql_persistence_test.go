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

package cloudsqlmysql

import (
	"testing"

	"github.com/stretchr/testify/suite"

	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/testflags"
)

// This is intentionally in a weird spot because it's part of a separate go module
// It also requires considerable manual configuration to actually run, such as provisioning the necessary cloud
// resources

func TestCloudSQLMySQLHistoryV2PersistenceSuite(t *testing.T) {
	s := new(pt.HistoryV2PersistenceSuite)
	s.TestBase = CloudMySQLTestBase(t)
	suite.Run(t, s)
}

func TestCloudSQLMySQLMatchingPersistenceSuite(t *testing.T) {
	s := new(pt.MatchingPersistenceSuite)
	s.TestBase = CloudMySQLTestBase(t)
	suite.Run(t, s)
}

func TestCloudSQLMySQLMetadataPersistenceSuiteV2(t *testing.T) {
	s := new(pt.MetadataPersistenceSuiteV2)
	s.TestBase = CloudMySQLTestBase(t)
	suite.Run(t, s)
}

func TestCloudSQLMySQLShardPersistenceSuite(t *testing.T) {
	s := new(pt.ShardPersistenceSuite)
	s.TestBase = CloudMySQLTestBase(t)
	suite.Run(t, s)
}

type ExecutionManagerSuite struct {
	pt.ExecutionManagerSuite
}

func (s *ExecutionManagerSuite) TestCreateWorkflowExecutionWithWorkflowRequestsDedup() {
	s.T().Skip("skip the test until we store workflow_request in mysql")
}

func (s *ExecutionManagerSuite) TestUpdateWorkflowExecutionWithWorkflowRequestsDedup() {
	s.T().Skip("skip the test until we store workflow_request in mysql")
}

func TestCloudSQLMySQLExecutionManagerSuite(t *testing.T) {
	s := new(ExecutionManagerSuite)
	s.TestBase = CloudMySQLTestBase(t)
	suite.Run(t, s)
}

func TestCloudSQLMySQLExecutionManagerWithEventsV2(t *testing.T) {
	s := new(pt.ExecutionManagerSuiteForEventsV2)
	s.TestBase = CloudMySQLTestBase(t)
	suite.Run(t, s)
}

func TestCloudSQLMySQLVisibilityPersistenceSuite(t *testing.T) {
	s := new(pt.DBVisibilityPersistenceSuite)
	s.TestBase = CloudMySQLTestBase(t)
	suite.Run(t, s)
}

func TestCloudSQLMySQLQueuePersistence(t *testing.T) {
	s := new(pt.QueuePersistenceSuite)
	s.TestBase = CloudMySQLTestBase(t)
	suite.Run(t, s)
}

func TestCloudSQLMySQLConfigPersistence(t *testing.T) {
	s := new(pt.ConfigStorePersistenceSuite)
	s.TestBase = CloudMySQLTestBase(t)
	suite.Run(t, s)
}

func TestCloudSQLMySQLDomainAuditPersistence(t *testing.T) {
	s := new(pt.DomainAuditPersistenceSuite)
	s.TestBase = CloudMySQLTestBase(t)
	suite.Run(t, s)
}

func CloudMySQLTestBase(t *testing.T) *pt.TestBase {
	testflags.RequireMySQL(t)
	base := pt.NewTestBase(t, pt.TestBaseParams{
		PersistenceConfig: pt.SimplePersistenceConfig(t, GetTestConfig),
	})
	base.Setup()
	return base
}
