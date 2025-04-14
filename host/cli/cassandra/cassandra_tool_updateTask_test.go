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
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/schema/cassandra"
	"github.com/uber/cadence/testflags"
	cassandra2 "github.com/uber/cadence/tools/cassandra"
	"github.com/uber/cadence/tools/common/schema/test"
)

type UpdateSchemaTestSuite struct {
	test.UpdateSchemaTestBase
}

func TestUpdateSchemaTestSuite(t *testing.T) {
	testflags.RequireCassandra(t)
	suite.Run(t, new(UpdateSchemaTestSuite))
}

func (s *UpdateSchemaTestSuite) SetupSuite() {
	client, err := NewTestCQLClient(cassandra2.SystemKeyspace)
	if err != nil {
		log.Fatal("Error creating CQLClient")
	}
	s.SetupSuiteBase(client)
}

func (s *UpdateSchemaTestSuite) TearDownSuite() {
	s.TearDownSuiteBase()
}

func (s *UpdateSchemaTestSuite) TestUpdateSchema() {
	client, err := NewTestCQLClient(s.DBName)
	s.Nil(err)
	defer client.Close()
	s.RunUpdateSchemaTest(cassandra2.BuildCLIOptions(), client, "-k", CreateTestCQLFileContent(), []string{"events", "tasks"})
}

func (s *UpdateSchemaTestSuite) TestDryrun() {
	client, err := NewTestCQLClient(s.DBName)
	s.Nil(err)
	defer client.Close()
	dir := rootRelativePath + "schema/cassandra/cadence/versioned"
	s.RunDryrunTest(cassandra2.BuildCLIOptions(), client, "-k", dir, cassandra.Version)
}

func (s *UpdateSchemaTestSuite) TestVisibilityDryrun() {
	client, err := NewTestCQLClient(s.DBName)
	s.Nil(err)
	defer client.Close()
	dir := rootRelativePath + "schema/cassandra/visibility/versioned"
	s.RunDryrunTest(cassandra2.BuildCLIOptions(), client, "-k", dir, cassandra.VisibilityVersion)
}

func (s *UpdateSchemaTestSuite) TestShortcut() {
	client, err := NewTestCQLClient(s.DBName)
	s.Nil(err)
	defer client.Close()
	dir := rootRelativePath + "schema/cassandra/cadence/versioned"

	cqlshArgs := []string{"--cqlversion=3.4.6", "-e", "DESC KEYSPACE %s;"}
	if cassandraHost := os.Getenv("CASSANDRA_HOST"); cassandraHost != "" {
		cqlshArgs = append(cqlshArgs, cassandraHost)
	}
	s.RunShortcutTest(cassandra2.BuildCLIOptions(), client, "-k", dir, "cqlsh", cqlshArgs...)
}
