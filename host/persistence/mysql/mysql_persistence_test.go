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

package mysql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/mysql"
	"github.com/uber/cadence/testflags"
)

func TestMySQLHistoryV2PersistenceSuite(t *testing.T) {
	testflags.RequireMySQL(t)
	s := new(pt.HistoryV2PersistenceSuite)
	option, err := mysql.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestMySQLMatchingPersistenceSuite(t *testing.T) {
	testflags.RequireMySQL(t)
	s := new(pt.MatchingPersistenceSuite)
	option, err := mysql.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestMySQLMetadataPersistenceSuiteV2(t *testing.T) {
	testflags.RequireMySQL(t)
	s := new(pt.MetadataPersistenceSuiteV2)
	option, err := mysql.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestMySQLShardPersistenceSuite(t *testing.T) {
	testflags.RequireMySQL(t)
	s := new(pt.ShardPersistenceSuite)
	option, err := mysql.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
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

func TestMySQLExecutionManagerSuite(t *testing.T) {
	testflags.RequireMySQL(t)
	s := new(ExecutionManagerSuite)
	option, err := mysql.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestMySQLExecutionManagerWithEventsV2(t *testing.T) {
	testflags.RequireMySQL(t)
	s := new(pt.ExecutionManagerSuiteForEventsV2)
	option, err := mysql.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestMySQLVisibilityPersistenceSuite(t *testing.T) {
	testflags.RequireMySQL(t)
	s := new(pt.DBVisibilityPersistenceSuite)
	option, err := mysql.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestMySQLQueuePersistence(t *testing.T) {
	testflags.RequireMySQL(t)
	s := new(pt.QueuePersistenceSuite)
	option, err := mysql.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}

func TestMySQLConfigPersistence(t *testing.T) {
	testflags.RequireMySQL(t)
	s := new(pt.ConfigStorePersistenceSuite)
	option, err := mysql.GetTestClusterOption()
	assert.NoError(t, err)
	s.TestBase = pt.NewTestBaseWithSQL(t, option)
	s.TestBase.Setup()
	suite.Run(t, s)
}
