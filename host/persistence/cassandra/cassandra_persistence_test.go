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
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql/public"
	persistencetests "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/testflags"
)

func TestCassandraHistoryPersistence(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.HistoryV2PersistenceSuite)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.Setup()
	suite.Run(t, s)
}

func TestCassandraMatchingPersistence(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.MatchingPersistenceSuite)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.Setup()
	suite.Run(t, s)
}

func TestCassandraDomainPersistence(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.MetadataPersistenceSuiteV2)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.Setup()
	suite.Run(t, s)
}

func TestCassandraShardPersistence(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.ShardPersistenceSuite)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.Setup()
	suite.Run(t, s)
}

func TestCassandraShardMigrationPersistence(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.ShardPersistenceSuite)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.TestBase.DynamicConfiguration.ReadNoSQLShardFromDataBlob = dynamicproperties.GetBoolPropertyFn(true)
	s.Setup()
	suite.Run(t, s)
}

func TestCassandraVisibilityPersistence(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.DBVisibilityPersistenceSuite)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.Setup()
	suite.Run(t, s)
}

func TestCassandraExecutionManager(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.ExecutionManagerSuite)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.Setup()
	suite.Run(t, s)
}

func TestCassandraExecutionManagerWithEventsV2(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.ExecutionManagerSuiteForEventsV2)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.Setup()
	suite.Run(t, s)
}

func TestCassandraQueuePersistence(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.QueuePersistenceSuite)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.Setup()
	suite.Run(t, s)
}

func TestCassandraConfigStorePersistence(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.ConfigStorePersistenceSuite)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.Setup()
	suite.Run(t, s)
}

func TestCassandraDomainAuditPersistence(t *testing.T) {
	testflags.RequireCassandra(t)
	s := new(persistencetests.DomainAuditPersistenceSuite)
	s.TestBase = public.NewTestBaseWithPublicCassandra(t, &persistencetests.TestBaseOptions{})
	s.Setup()
	suite.Run(t, s)
}
