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

	"github.com/uber/cadence/environment"
	"github.com/uber/cadence/testflags"
	"github.com/uber/cadence/tools/cassandra"
	"github.com/uber/cadence/tools/common/schema/test"
)

type (
	SetupSchemaTestSuite struct {
		test.SetupSchemaTestBase
		client cassandra.CqlClient
	}
)

func TestSetupSchemaTestSuite(t *testing.T) {
	testflags.RequireCassandra(t)
	suite.Run(t, new(SetupSchemaTestSuite))
}

func (s *SetupSchemaTestSuite) SetupSuite() {
	os.Setenv("CASSANDRA_HOST", environment.GetCassandraAddress())
	client, err := NewTestCQLClient(cassandra.SystemKeyspace)
	if err != nil {
		log.Fatal("Error creating CQLClient")
	}
	s.client = client
	s.SetupSuiteBase(client)
}

func (s *SetupSchemaTestSuite) TearDownSuite() {
	s.TearDownSuiteBase()
}

func (s *SetupSchemaTestSuite) TestCreateKeyspace() {
	s.Nil(cassandra.RunTool([]string{"./tool", "create", "-k", "foobar123", "--rf", "1"}))
	err := s.client.DropKeyspace("foobar123")
	s.Nil(err)
}

func (s *SetupSchemaTestSuite) TestSetupSchema() {
	client, err := NewTestCQLClient(s.DBName)
	s.Nil(err)
	s.RunSetupTest(cassandra.BuildCLIOptions(), client, "-k", CreateTestCQLFileContent(), []string{"tasks", "events"})
}
