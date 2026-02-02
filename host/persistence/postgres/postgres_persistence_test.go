// Copyright (c) 2019 Uber Technologies, Inc.
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

package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/postgres"
	"github.com/uber/cadence/testflags"
)

func TestPostgresSQLHistoryV2PersistenceSuite(t *testing.T) {
	testflags.RequirePostgres(t)
	s := new(pt.HistoryV2PersistenceSuite)
	options, err := postgres.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, options)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestPostgresSQLMatchingPersistenceSuite(t *testing.T) {
	testflags.RequirePostgres(t)
	s := new(pt.MatchingPersistenceSuite)
	options, err := postgres.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, options)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestPostgresSQLMetadataPersistenceSuiteV2(t *testing.T) {
	testflags.RequirePostgres(t)
	s := new(pt.MetadataPersistenceSuiteV2)
	options, err := postgres.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, options)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestPostgresSQLShardPersistenceSuite(t *testing.T) {
	testflags.RequirePostgres(t)
	s := new(pt.ShardPersistenceSuite)
	options, err := postgres.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, options)
	s.TestBase.Setup()
	suite.Run(t, s)
}

type ExecutionManagerSuite struct {
	pt.ExecutionManagerSuite
}

func (s *ExecutionManagerSuite) TestCreateWorkflowExecutionWithWorkflowRequestsDedup() {
	s.T().Skip("skip the test until we store workflow_request in postgres sql")
}

func (s *ExecutionManagerSuite) TestUpdateWorkflowExecutionWithWorkflowRequestsDedup() {
	s.T().Skip("skip the test until we store workflow_request in postgres sql")
}

func TestPostgresSQLExecutionManagerSuite(t *testing.T) {
	testflags.RequirePostgres(t)
	s := new(ExecutionManagerSuite)
	options, err := postgres.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, options)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestPostgresSQLExecutionManagerWithEventsV2(t *testing.T) {
	testflags.RequirePostgres(t)
	s := new(pt.ExecutionManagerSuiteForEventsV2)
	option, err := postgres.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestPostgresSQLVisibilityPersistenceSuite(t *testing.T) {
	testflags.RequirePostgres(t)
	s := new(pt.DBVisibilityPersistenceSuite)
	options, err := postgres.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, options)
	s.TestBase.Setup()
	suite.Run(t, s)
}

// TODO flaky test
// https://github.com/uber/cadence/issues/2877
/*
FAIL: TestPostgresSQLQueuePersistence/TestDomainReplicationQueue (0.26s)
        queuePersistenceTest.go:102:
            	Error Trace:	queuePersistenceTest.go:102
            	Error:      	Not equal:
            	            	expected: 99
            	            	actual  : 98
            	Test:       	TestPostgresSQLQueuePersistence/TestDomainReplicationQueue
*/
// func TestPostgresSQLQueuePersistence(t *testing.T) {
//	s := new(pt.QueuePersistenceSuite)
//	s.TestBase = pt.NewTestBaseWithSQL(GetTestClusterOption())
//	s.TestBase.Setup()
//	suite.Run(t, s)
// }

func TestPostgresSQLConfigPersistence(t *testing.T) {
	testflags.RequirePostgres(t)
	s := new(pt.ConfigStorePersistenceSuite)
	options, err := postgres.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, options)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestPostgresSQLDomainAuditPersistence(t *testing.T) {
	testflags.RequirePostgres(t)
	s := new(pt.DomainAuditPersistenceSuite)
	options, err := postgres.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, options)
	s.TestBase.Setup()
	suite.Run(t, s)
}
