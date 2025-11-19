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

package sqlite

import (
	"testing"

	"github.com/stretchr/testify/suite"

	pt "github.com/uber/cadence/common/persistence/persistence-tests"
)

func TestSQLiteHistoryV2PersistenceSuite(t *testing.T) {
	s := new(pt.HistoryV2PersistenceSuite)
	option := GetTestClusterOption()
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestSQLiteMatchingPersistenceSuite(t *testing.T) {
	s := new(pt.MatchingPersistenceSuite)
	option := GetTestClusterOption()
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestSQLiteMetadataPersistenceSuiteV2(t *testing.T) {
	s := new(pt.MetadataPersistenceSuiteV2)
	option := GetTestClusterOption()
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestSQLiteShardPersistenceSuite(t *testing.T) {
	s := new(pt.ShardPersistenceSuite)
	option := GetTestClusterOption()
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

type ExecutionManagerSuite struct {
	pt.ExecutionManagerSuite
}

func (s *ExecutionManagerSuite) TestCreateWorkflowExecutionWithWorkflowRequestsDedup() {
	s.T().Skip("skip the test until we store workflow_request in sqlite")
}

func (s *ExecutionManagerSuite) TestUpdateWorkflowExecutionWithWorkflowRequestsDedup() {
	s.T().Skip("skip the test until we store workflow_request in sqlite")
}

func TestSQLiteExecutionManagerSuite(t *testing.T) {
	s := new(ExecutionManagerSuite)
	option := GetTestClusterOption()
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestSQLiteExecutionManagerWithEventsV2(t *testing.T) {
	s := new(pt.ExecutionManagerSuiteForEventsV2)
	option := GetTestClusterOption()
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestSQLiteVisibilityPersistenceSuite(t *testing.T) {
	s := new(pt.DBVisibilityPersistenceSuite)
	option := GetTestClusterOption()
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestSQLiteQueuePersistence(t *testing.T) {
	s := new(pt.QueuePersistenceSuite)
	option := GetTestClusterOption()
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestSQLiteConfigPersistence(t *testing.T) {
	s := new(pt.ConfigStorePersistenceSuite)
	option := GetTestClusterOption()
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestSQLiteDomainAuditPersistence(t *testing.T) {
	t.Skip("DomainAuditPersistence is only implemented for NoSQL/Cassandra in this PR")
}
