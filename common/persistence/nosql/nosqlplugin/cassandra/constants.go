// Copyright (c) 2017-2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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
	"time"

	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

var (
	defaultDateTime            = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	defaultVisibilityTimestamp = p.UnixNanoToDBTimestamp(defaultDateTime.UnixNano())
)

const (
	// Cassandra max support time is 2038-01-19T03:14:06+00:00. Updated this to 5 years to support until year 2033
	// See https://github.com/uber/cadence/issues/4200
	maxCassandraTTL = int64(157680000)

	// We use local serial consistency level as the default consistency level for conditional updates
	cassandraDefaultSerialConsLevel = gocql.LocalSerial

	// We use local quorum consistency level as the default consistency level
	cassandraDefaultConsLevel = gocql.LocalQuorum

	// Although Cadence core data models always require strong consistency, reading visibility is a special case that
	// eventual consistency is sufficient.
	// That's because the engine layer writes into visibility with eventual consistency anyway(using transfer tasks)
	// Do NOT use it in other places, unless you are sure it's the same special cases like reading visibility
	cassandraLowConslevel = gocql.LocalOne

	// We use all consistency level for delete operations to prevent the data resurrection issue
	cassandraAllConslevel = gocql.All
)

const (
	// Row types for table executions
	rowTypeShard = iota
	rowTypeExecution
	rowTypeTransferTask
	rowTypeTimerTask
	rowTypeReplicationTask
	rowTypeDLQ
	rowTypeCrossClusterTask
	rowTypeWorkflowRequestStart
	rowTypeWorkflowRequestSignal
	rowTypeWorkflowRequestCancel
	rowTypeWorkflowRequestReset
)

// Guidelines for creating new special UUID constants
// Each UUID should be of the form: E0000000-R000-f000-f000-00000000000x
// Where x is any hexadecimal value, E represents the entity type valid values are:
// E = {DomainID = 1, WorkflowID = 2, RunID = 3}
// R represents row type in executions table, valid values are:
// R = {Shard = 0, Execution = 1, Transfer = 2, Timer = 3, Replication = 4, DLQ = 5, CrossCluster = 6}
const (
	// Special Domains related constants
	emptyDomainID = "10000000-0000-f000-f000-000000000000"
	// Special Run IDs
	emptyRunID     = "30000000-0000-f000-f000-000000000000"
	permanentRunID = "30000000-0000-f000-f000-000000000001"
	// Row Constants for Shard Row
	rowTypeShardDomainID   = "10000000-1000-f000-f000-000000000000"
	rowTypeShardWorkflowID = "20000000-1000-f000-f000-000000000000"
	rowTypeShardRunID      = "30000000-1000-f000-f000-000000000000"
	// Row Constants for Transfer Task Row
	rowTypeTransferDomainID   = "10000000-3000-f000-f000-000000000000"
	rowTypeTransferWorkflowID = "20000000-3000-f000-f000-000000000000"
	rowTypeTransferRunID      = "30000000-3000-f000-f000-000000000000"
	// Row Constants for Cross Cluster Task Row
	rowTypeCrossClusterDomainID = "10000000-7000-f000-f000-000000000000"
	rowTypeCrossClusterRunID    = "30000000-7000-f000-f000-000000000000"
	// Row Constants for Timer Task Row
	rowTypeTimerDomainID   = "10000000-4000-f000-f000-000000000000"
	rowTypeTimerWorkflowID = "20000000-4000-f000-f000-000000000000"
	rowTypeTimerRunID      = "30000000-4000-f000-f000-000000000000"
	// Row Constants for Replication Task Row
	rowTypeReplicationDomainID   = "10000000-5000-f000-f000-000000000000"
	rowTypeReplicationWorkflowID = "20000000-5000-f000-f000-000000000000"
	rowTypeReplicationRunID      = "30000000-5000-f000-f000-000000000000"
	// Row Constants for Replication Task DLQ Row. Source cluster name will be used as WorkflowID.
	rowTypeDLQDomainID = "10000000-6000-f000-f000-000000000000"
	rowTypeDLQRunID    = "30000000-6000-f000-f000-000000000000"
	// Special TaskId constants
	rowTypeExecutionTaskID      = int64(-10)
	rowTypeShardTaskID          = int64(-11)
	emptyInitiatedID            = int64(-7)
	emptyWorkflowRequestVersion = int64(-1000)
	workflowRequestTTLInSeconds = 10800
)
