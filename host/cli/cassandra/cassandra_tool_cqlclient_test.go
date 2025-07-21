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

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/testflags"
	"github.com/uber/cadence/tools/cassandra"
	"github.com/uber/cadence/tools/common/schema/test"

	_ "github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql/public" // needed to load the default gocql client
)

type (
	CQLClientTestSuite struct {
		test.DBTestBase
	}
)

var _ test.DB = (cassandra.CqlClient)(nil)

func TestCQLClientTestSuite(t *testing.T) {
	testflags.RequireCassandra(t)
	suite.Run(t, new(CQLClientTestSuite))
}

func (s *CQLClientTestSuite) SetupTest() {
	s.Assertions = require.New(s.T()) // Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
}

func (s *CQLClientTestSuite) SetupSuite() {
	client, err := NewTestCQLClient(cassandra.SystemKeyspace)
	if err != nil {
		s.Log.Fatal("error creating CQLClient, ", tag.Error(err))
	}
	s.SetupSuiteBase(client)
}

func (s *CQLClientTestSuite) TearDownSuite() {
	s.TearDownSuiteBase()
}

func (s *CQLClientTestSuite) TestParseCQLFile() {
	s.RunParseFileTest(CreateTestCQLFileContent())
}

func (s *CQLClientTestSuite) TestCQLClient() {
	client, err := NewTestCQLClient(s.DBName)
	s.Nil(err)
	s.RunCreateTest(client)
	s.RunUpdateTest(client)
	s.RunDropTest(client)
	client.Close()
}
