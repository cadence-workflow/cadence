// Copyright (c) 2025 Uber Technologies, Inc.
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

package persistencetests

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"

	p "github.com/uber/cadence/common/persistence"
)

type (
	// HistoryTaskDLQPersistenceSuite contains history task DLQ persistence tests.
	// These exercise the real CQL against the history_task_dlq and
	// history_task_dlq_ack_level tables introduced for the history task DLQ feature.
	HistoryTaskDLQPersistenceSuite struct {
		*TestBase
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions

		dlqManager p.HistoryTaskDLQManager
	}
)

// SetupSuite implementation
func (s *HistoryTaskDLQPersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

// SetupTest implementation
func (s *HistoryTaskDLQPersistenceSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())

	var err error
	s.dlqManager, err = s.ExecutionMgrFactory.NewHistoryTaskDLQManager()
	s.NoError(err)
}

// TearDownSuite implementation
func (s *HistoryTaskDLQPersistenceSuite) TearDownSuite() {
	if s.dlqManager != nil {
		s.dlqManager.Close()
	}
	s.TearDownWorkflowStore()
}

// newTransferTask builds a transfer-category (immediate) DLQ task with the given identifiers.
// The task's workflow/run/domain identifiers must be valid UUIDs because the transfer task
// serializer parses them as such.
func (s *HistoryTaskDLQPersistenceSuite) newTransferTask(taskID, version int64) *p.ActivityTask {
	return &p.ActivityTask{
		WorkflowIdentifier: p.WorkflowIdentifier{
			DomainID:   uuid.New(),
			WorkflowID: "task-workflow",
			RunID:      uuid.New(),
		},
		TaskData: p.TaskData{
			Version: version,
			TaskID:  taskID,
		},
		TargetDomainID: uuid.New(),
		TaskList:       "task-tasklist",
		ScheduleID:     11,
	}
}

// newTimerTask builds a timer-category DLQ task with the given identifiers and fire time.
// The task's workflow/run/domain identifiers must be valid UUIDs because the timer task
// serializer parses them as such.
func (s *HistoryTaskDLQPersistenceSuite) newTimerTask(taskID, version int64, visibilityTS time.Time) *p.UserTimerTask {
	return &p.UserTimerTask{
		WorkflowIdentifier: p.WorkflowIdentifier{
			DomainID:   uuid.New(),
			WorkflowID: "timer-workflow",
			RunID:      uuid.New(),
		},
		TaskData: p.TaskData{
			Version:             version,
			TaskID:              taskID,
			VisibilityTimestamp: visibilityTS,
		},
		EventID:  7,
		TaskList: "timer-tasklist",
	}
}

// TestCreateAndGetHistoryDLQTask writes a single transfer task and reads it back, asserting the
// deserialized task round-trips and that the page byte size is reported.
func (s *HistoryTaskDLQPersistenceSuite) TestCreateAndGetHistoryDLQTask() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	const (
		shardID = 1001
		domain  = "domain-create-get"
		scope   = "scope"
		name    = "cluster-a"
	)
	task := s.newTransferTask(1, 5)

	err := s.dlqManager.CreateHistoryDLQTask(ctx, p.CreateHistoryDLQTaskRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		Task:                  task,
	})
	s.NoError(err)

	resp, err := s.dlqManager.GetHistoryDLQTasks(ctx, p.HistoryDLQGetTasksRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		TaskCategory:          p.HistoryTaskCategoryTransfer,
		InclusiveMinTaskKey:   p.MinimumHistoryTaskKey,
		ExclusiveMaxTaskKey:   p.MaximumHistoryTaskKey,
		PageSize:              10,
	})
	s.NoError(err)
	s.Len(resp.Tasks, 1)
	s.Greater(resp.PageSizeBytes, 0)

	got := resp.Tasks[0]
	s.Equal(task.GetTaskID(), got.GetTaskID())
	s.Equal(task.GetVersion(), got.GetVersion())
	s.Equal(task.GetWorkflowID(), got.GetWorkflowID())
	s.Equal(task.GetRunID(), got.GetRunID())
	s.Equal(p.HistoryTaskCategoryTransfer, got.GetTaskCategory())
}

// TestCreateInitializesAckLevelSentinel verifies that CreateHistoryDLQAckLevelIfNotExists seeds an
// ack-level row at the minimum task key, and that a subsequent call is a no-op that never overwrites
// recorded progress.
func (s *HistoryTaskDLQPersistenceSuite) TestCreateInitializesAckLevelSentinel() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	const (
		shardID = 1002
		domain  = "domain-sentinel"
		scope   = "scope"
		name    = "cluster-a"
	)

	err := s.dlqManager.CreateHistoryDLQAckLevelIfNotExists(ctx, p.CreateHistoryDLQAckLevelRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		TaskCategory:          p.HistoryTaskCategoryTransfer,
	})
	s.NoError(err)

	ackLevels, err := s.dlqManager.GetHistoryDLQAckLevels(ctx, p.HistoryDLQGetAckLevelsRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		TaskCategory:          p.HistoryTaskCategoryTransfer,
	})
	s.NoError(err)
	s.Len(ackLevels, 1)
	s.Equal(p.MinimumHistoryTaskKey.GetTaskID(), ackLevels[0].AckLevelTaskID)
	s.Equal(p.HistoryTaskCategoryTransfer, ackLevels[0].TaskCategory)

	// Advance the ack level
	s.NoError(s.dlqManager.UpdateHistoryDLQAckLevel(ctx, p.HistoryDLQUpdateAckLevelRequest{
		ShardID:                   shardID,
		DomainID:                  domain,
		ClusterAttributeScope:     scope,
		ClusterAttributeName:      name,
		TaskCategory:              p.HistoryTaskCategoryTransfer,
		UpdatedInclusiveReadLevel: p.NewImmediateTaskKey(42),
	}))
	// Re-seed the ack level
	s.NoError(s.dlqManager.CreateHistoryDLQAckLevelIfNotExists(ctx, p.CreateHistoryDLQAckLevelRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		TaskCategory:          p.HistoryTaskCategoryTransfer,
	}))
	// Verify the ack level is still at the advanced level
	ackLevels, err = s.dlqManager.GetHistoryDLQAckLevels(ctx, p.HistoryDLQGetAckLevelsRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		TaskCategory:          p.HistoryTaskCategoryTransfer,
	})
	s.NoError(err)
	s.Len(ackLevels, 1)
	s.Equal(int64(42), ackLevels[0].AckLevelTaskID)
}

// TestUpdateAndGetAckLevel updates an ack level and verifies it is read back, and that a subsequent
// task create does not reset the recorded progress.
func (s *HistoryTaskDLQPersistenceSuite) TestUpdateAndGetAckLevel() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	const (
		shardID = 1003
		domain  = "domain-update-ack"
		scope   = "scope"
		name    = "cluster-a"
	)

	s.NoError(s.dlqManager.CreateHistoryDLQTask(ctx, p.CreateHistoryDLQTaskRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		Task:                  s.newTransferTask(1, 1),
	}))

	newAckLevel := p.NewImmediateTaskKey(42)
	s.NoError(s.dlqManager.UpdateHistoryDLQAckLevel(ctx, p.HistoryDLQUpdateAckLevelRequest{
		ShardID:                   shardID,
		DomainID:                  domain,
		ClusterAttributeScope:     scope,
		ClusterAttributeName:      name,
		TaskCategory:              p.HistoryTaskCategoryTransfer,
		UpdatedInclusiveReadLevel: newAckLevel,
	}))

	ackLevels, err := s.dlqManager.GetHistoryDLQAckLevels(ctx, p.HistoryDLQGetAckLevelsRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		TaskCategory:          p.HistoryTaskCategoryTransfer,
	})
	s.NoError(err)
	s.Len(ackLevels, 1)
	s.Equal(int64(42), ackLevels[0].AckLevelTaskID)

	// A later write must not clobber the recorded ack level back to the sentinel.
	s.NoError(s.dlqManager.CreateHistoryDLQTask(ctx, p.CreateHistoryDLQTaskRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		Task:                  s.newTransferTask(2, 1),
	}))
	ackLevels, err = s.dlqManager.GetHistoryDLQAckLevels(ctx, p.HistoryDLQGetAckLevelsRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		TaskCategory:          p.HistoryTaskCategoryTransfer,
	})
	s.NoError(err)
	s.Len(ackLevels, 1)
	s.Equal(int64(42), ackLevels[0].AckLevelTaskID)
}

// TestGetHistoryDLQTasksPagination inserts several tasks and reads them back across multiple pages,
// asserting every task is returned exactly once in clustering order.
func (s *HistoryTaskDLQPersistenceSuite) TestGetHistoryDLQTasksPagination() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	const (
		shardID  = 1004
		domain   = "domain-pagination"
		scope    = "scope"
		name     = "cluster-a"
		numTasks = 5
		pageSize = 2
	)

	for i := 1; i <= numTasks; i++ {
		s.NoError(s.dlqManager.CreateHistoryDLQTask(ctx, p.CreateHistoryDLQTaskRequest{
			ShardID:               shardID,
			DomainID:              domain,
			ClusterAttributeScope: scope,
			ClusterAttributeName:  name,
			Task:                  s.newTransferTask(int64(i), 1),
		}))
	}

	var gotIDs []int64
	var token []byte
	for {
		resp, err := s.dlqManager.GetHistoryDLQTasks(ctx, p.HistoryDLQGetTasksRequest{
			ShardID:               shardID,
			DomainID:              domain,
			ClusterAttributeScope: scope,
			ClusterAttributeName:  name,
			TaskCategory:          p.HistoryTaskCategoryTransfer,
			InclusiveMinTaskKey:   p.MinimumHistoryTaskKey,
			ExclusiveMaxTaskKey:   p.MaximumHistoryTaskKey,
			PageSize:              pageSize,
			NextPageToken:         token,
		})
		s.NoError(err)
		for _, t := range resp.Tasks {
			gotIDs = append(gotIDs, t.GetTaskID())
		}
		token = resp.NextPageToken
		if len(token) == 0 {
			break
		}
	}

	s.Equal([]int64{1, 2, 3, 4, 5}, gotIDs)
}

// TestDeleteHistoryDLQTasks deletes tasks below an exclusive-max key and asserts only the tasks at
// or above the key remain.
func (s *HistoryTaskDLQPersistenceSuite) TestDeleteHistoryDLQTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	const (
		shardID = 1005
		domain  = "domain-delete"
		scope   = "scope"
		name    = "cluster-a"
	)

	for i := 1; i <= 4; i++ {
		s.NoError(s.dlqManager.CreateHistoryDLQTask(ctx, p.CreateHistoryDLQTaskRequest{
			ShardID:               shardID,
			DomainID:              domain,
			ClusterAttributeScope: scope,
			ClusterAttributeName:  name,
			Task:                  s.newTransferTask(int64(i), 1),
		}))
	}

	// Delete tasks with key < (epoch, 3): removes task IDs 1 and 2.
	s.NoError(s.dlqManager.DeleteHistoryDLQTasks(ctx, p.HistoryDLQDeleteTasksRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		TaskCategory:          p.HistoryTaskCategoryTransfer,
		ExclusiveMaxTaskKey:   p.NewImmediateTaskKey(3),
	}))

	resp, err := s.dlqManager.GetHistoryDLQTasks(ctx, p.HistoryDLQGetTasksRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		TaskCategory:          p.HistoryTaskCategoryTransfer,
		InclusiveMinTaskKey:   p.MinimumHistoryTaskKey,
		ExclusiveMaxTaskKey:   p.MaximumHistoryTaskKey,
		PageSize:              10,
	})
	s.NoError(err)

	var remaining []int64
	for _, t := range resp.Tasks {
		remaining = append(remaining, t.GetTaskID())
	}
	s.Equal([]int64{3, 4}, remaining)
}

// TestAckLevelFilteringScopes writes tasks across multiple domains, cluster attributes, and task
// categories within a single shard, then asserts the shard-wide, by-domain, by-cluster-attribute,
// and by-category ack-level queries each return the expected partitions with no cross-partition bleed.
func (s *HistoryTaskDLQPersistenceSuite) TestAckLevelFilteringScopes() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	const (
		shardID = 1006
		scope   = "scope"
		domainA = "domain-a"
		domainB = "domain-b"
		nameX   = "cluster-x"
		nameY   = "cluster-y"
	)
	timerTS := time.Now().Truncate(p.DBTimestampMinPrecision).UTC()

	// (domainA, nameX) transfer, (domainA, nameY) transfer, (domainB, nameX) timer.
	s.NoError(s.dlqManager.CreateHistoryDLQTask(ctx, p.CreateHistoryDLQTaskRequest{
		ShardID: shardID, DomainID: domainA, ClusterAttributeScope: scope, ClusterAttributeName: nameX,
		Task: s.newTransferTask(1, 1),
	}))
	s.NoError(s.dlqManager.CreateHistoryDLQAckLevelIfNotExists(ctx, p.CreateHistoryDLQAckLevelRequest{
		ShardID: shardID, DomainID: domainA, ClusterAttributeScope: scope, ClusterAttributeName: nameX,
		TaskCategory: p.HistoryTaskCategoryTransfer,
	}))
	s.NoError(s.dlqManager.CreateHistoryDLQTask(ctx, p.CreateHistoryDLQTaskRequest{
		ShardID: shardID, DomainID: domainA, ClusterAttributeScope: scope, ClusterAttributeName: nameY,
		Task: s.newTransferTask(1, 1),
	}))
	s.NoError(s.dlqManager.CreateHistoryDLQAckLevelIfNotExists(ctx, p.CreateHistoryDLQAckLevelRequest{
		ShardID: shardID, DomainID: domainA, ClusterAttributeScope: scope, ClusterAttributeName: nameY,
		TaskCategory: p.HistoryTaskCategoryTransfer,
	}))
	s.NoError(s.dlqManager.CreateHistoryDLQTask(ctx, p.CreateHistoryDLQTaskRequest{
		ShardID: shardID, DomainID: domainB, ClusterAttributeScope: scope, ClusterAttributeName: nameX,
		Task: s.newTimerTask(1, 1, timerTS),
	}))
	s.NoError(s.dlqManager.CreateHistoryDLQAckLevelIfNotExists(ctx, p.CreateHistoryDLQAckLevelRequest{
		ShardID: shardID, DomainID: domainB, ClusterAttributeScope: scope, ClusterAttributeName: nameX,
		TaskCategory: p.HistoryTaskCategoryTimer,
	}))

	// Shard-wide: all three partitions.
	all, err := s.dlqManager.GetHistoryDLQAckLevels(ctx, p.HistoryDLQGetAckLevelsRequest{ShardID: shardID})
	s.NoError(err)
	s.Len(all, 3)

	// By-domain: the two domainA partitions.
	byDomain, err := s.dlqManager.GetHistoryDLQAckLevels(ctx, p.HistoryDLQGetAckLevelsRequest{
		ShardID: shardID, DomainID: domainA,
	})
	s.NoError(err)
	s.Len(byDomain, 2)
	for _, ack := range byDomain {
		s.Equal(domainA, ack.DomainID)
	}

	// By-cluster-attribute: exactly one partition.
	byAttr, err := s.dlqManager.GetHistoryDLQAckLevels(ctx, p.HistoryDLQGetAckLevelsRequest{
		ShardID: shardID, DomainID: domainA, ClusterAttributeScope: scope, ClusterAttributeName: nameX,
	})
	s.NoError(err)
	s.Len(byAttr, 1)
	s.Equal(nameX, byAttr[0].ClusterAttributeName)

	// By-category (shard-wide, transfer only): the two transfer partitions.
	transferOnly, err := s.dlqManager.GetHistoryDLQAckLevels(ctx, p.HistoryDLQGetAckLevelsRequest{
		ShardID: shardID, TaskCategory: p.HistoryTaskCategoryTransfer,
	})
	s.NoError(err)
	s.Len(transferOnly, 2)
	for _, ack := range transferOnly {
		s.Equal(p.HistoryTaskCategoryTransfer, ack.TaskCategory)
	}
}

// TestTimerCategoryRoundTrip writes and reads back a timer-category task, covering the non-transfer
// task type and the timestamp-ordered clustering key.
func (s *HistoryTaskDLQPersistenceSuite) TestTimerCategoryRoundTrip() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	const (
		shardID = 1007
		domain  = "domain-timer"
		scope   = "scope"
		name    = "cluster-a"
	)
	timerTS := time.Now().Truncate(p.DBTimestampMinPrecision).UTC()
	task := s.newTimerTask(99, 3, timerTS)

	s.NoError(s.dlqManager.CreateHistoryDLQTask(ctx, p.CreateHistoryDLQTaskRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		Task:                  task,
	}))

	resp, err := s.dlqManager.GetHistoryDLQTasks(ctx, p.HistoryDLQGetTasksRequest{
		ShardID:               shardID,
		DomainID:              domain,
		ClusterAttributeScope: scope,
		ClusterAttributeName:  name,
		TaskCategory:          p.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey:   p.MinimumHistoryTaskKey,
		ExclusiveMaxTaskKey:   p.MaximumHistoryTaskKey,
		PageSize:              10,
	})
	s.NoError(err)
	s.Len(resp.Tasks, 1)

	got := resp.Tasks[0]
	s.Equal(task.GetTaskID(), got.GetTaskID())
	s.Equal(task.GetVersion(), got.GetVersion())
	s.Equal(task.GetWorkflowID(), got.GetWorkflowID())
	s.Equal(task.GetRunID(), got.GetRunID())
	s.Equal(p.HistoryTaskCategoryTimer, got.GetTaskCategory())
	s.Equal(timerTS, got.GetTaskKey().GetScheduledTime().UTC())
}
