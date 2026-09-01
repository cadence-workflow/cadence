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

package cassandra

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql/public"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/testflags"

	_ "github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra" // register cassandra plugin
)

func TestCassandraHistoryPersistence(t *testing.T) {
	s := new(pt.HistoryV2PersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraMatchingPersistence(t *testing.T) {
	s := new(pt.MatchingPersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraDomainPersistence(t *testing.T) {
	s := new(pt.MetadataPersistenceSuiteV2)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraShardPersistence(t *testing.T) {
	s := new(pt.ShardPersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraShardMigrationPersistence(t *testing.T) {
	s := new(pt.ShardPersistenceSuite)
	s.TestBase = CassandraTestBase(t, func(dc *persistence.DynamicConfiguration) {
		dc.ReadNoSQLShardFromDataBlob = dynamicproperties.GetBoolPropertyFn(true)
	})
	suite.Run(t, s)
}

func TestCassandraVisibilityPersistence(t *testing.T) {
	s := new(pt.DBVisibilityPersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraExecutionManager(t *testing.T) {
	s := new(pt.ExecutionManagerSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraExecutionManagerWithEventsV2(t *testing.T) {
	s := new(pt.ExecutionManagerSuiteForEventsV2)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraQueuePersistence(t *testing.T) {
	s := new(pt.QueuePersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraConfigStorePersistence(t *testing.T) {
	s := new(pt.ConfigStorePersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraDomainAuditPersistence(t *testing.T) {
	s := new(pt.DomainAuditPersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraHistoryTaskDLQPersistence(t *testing.T) {
	s := new(pt.HistoryTaskDLQPersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraSemaphoreMetadataPersistence(t *testing.T) {
	s := new(pt.SemaphoreMetadataPersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraSemaphoreTokenPersistence(t *testing.T) {
	s := new(pt.SemaphoreTokenPersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

func TestCassandraSemaphoreTaskPersistence(t *testing.T) {
	s := new(pt.SemaphoreTaskPersistenceSuite)
	s.TestBase = CassandraTestBase(t)
	suite.Run(t, s)
}

// CassandraTestBase creates and sets up a TestBase backed by an external/public Cassandra.
// Optional dcOpts can be used to customize the DynamicConfiguration before the test base is set up.
func CassandraTestBase(t *testing.T, dcOpts ...func(*persistence.DynamicConfiguration)) *pt.TestBase {
	testflags.RequireCassandra(t)
	dc := *persistence.NewDefaultDynamicConfiguration()
	dc.EnableCassandraAllConsistencyLevelDelete = dynamicproperties.GetBoolPropertyFn(true)
	dc.EnableHistoryTaskDualWriteMode = dynamicproperties.GetBoolPropertyFn(true)
	dc.EnableWorkflowTimerTaskCleanup = dynamicproperties.GetBoolPropertyFn(true)
	for _, opt := range dcOpts {
		opt(&dc)
	}
	base := pt.NewTestBase(t, pt.TestBaseParams{
		PersistenceConfig:    pt.SimplePersistenceConfig(t, public.NewTestConfigWithPublicCassandra),
		DynamicConfiguration: &dc,
	})
	base.Setup()
	return base
}
