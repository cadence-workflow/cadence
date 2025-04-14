// Copyright (c) 2021 Uber Technologies, Inc.
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
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/environment"
	"github.com/uber/cadence/tools/cassandra"
)

// NOTE: change this when moving the test files around during refactoring
// Path to root folder of cadence repo
const rootRelativePath = "../../../"

func NewTestCQLClient(keyspace string) (cassandra.CqlClient, error) {
	protoVersion, err := environment.GetCassandraProtoVersion()
	if err != nil {
		return nil, err
	}
	return cassandra.NewCQLClient(&cassandra.CQLClientConfig{
		Hosts:                 environment.GetCassandraAddress(),
		Port:                  cassandra.DefaultCassandraPort,
		Keyspace:              keyspace,
		Timeout:               cassandra.DefaultTimeout,
		User:                  environment.GetCassandraUsername(),
		Password:              environment.GetCassandraPassword(),
		AllowedAuthenticators: environment.GetCassandraAllowedAuthenticators(),
		NumReplicas:           1,
		ProtoVersion:          protoVersion,
	}, gocql.All)
}

func CreateTestCQLFileContent() string {
	return `
-- test cql file content

CREATE TABLE events (
  domain_id      uuid,
  workflow_id    text,
  run_id         uuid,
  -- We insert a batch of events with each append transaction.
  -- This field stores the event id of first event in the batch.
  first_event_id bigint,
  range_id       bigint,
  tx_id          bigint,
  data           blob, -- Batch of workflow execution history events as a blob
  data_encoding  text, -- Protocol used for history serialization
  data_version   int,  -- history blob version
  PRIMARY KEY ((domain_id, workflow_id, run_id), first_event_id)
);

-- Stores activity or workflow tasks
CREATE TABLE tasks (
  domain_id        uuid,
  task_list_name   text,
  task_list_type   int, -- enum TaskListType {ActivityTask, DecisionTask}
  type             int, -- enum rowType {Task, TaskList}
  task_id          bigint,  -- unique identifier for tasks, monotonically increasing
  range_id         bigint static, -- Used to ensure that only one process can write to the table
  task             text,
  task_list        text,
  PRIMARY KEY ((domain_id, task_list_name, task_list_type), type, task_id)
);

`
}
