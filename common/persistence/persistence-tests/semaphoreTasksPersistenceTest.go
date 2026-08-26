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

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/persistence"
)

type (
	SemaphoreTaskPersistenceSuite struct {
		*TestBase
		*require.Assertions
	}
)

func (s *SemaphoreTaskPersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

func (s *SemaphoreTaskPersistenceSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *SemaphoreTaskPersistenceSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

// TestClaimSemaphoreTaskBucket covers the first claim of a new bucket and the range_id bump on a re-claim.
func (s *SemaphoreTaskPersistenceSuite) TestClaimSemaphoreTaskBucket() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreTaskManager()
	s.NoError(err)
	s.NotNil(manager)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()

	// first claim creates the control row
	first, err := manager.ClaimSemaphoreTaskBucket(ctx, &persistence.ClaimSemaphoreTaskBucketRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        0,
	})
	s.NoError(err)
	s.Equal(int64(1), first.RangeID)
	s.Equal(int64(0), first.AckLevel)

	// second claim bumps range_id and preserves ack_level
	second, err := manager.ClaimSemaphoreTaskBucket(ctx, &persistence.ClaimSemaphoreTaskBucketRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        0,
	})
	s.NoError(err)
	s.Equal(int64(2), second.RangeID)
	s.Equal(int64(0), second.AckLevel)

	got, err := manager.GetSemaphoreTaskBucketState(ctx, &persistence.GetSemaphoreTaskBucketStateRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        0,
	})
	s.NoError(err)
	s.Equal(int64(2), got.RangeID)
	s.Equal(int64(0), got.AckLevel)
}

func (s *SemaphoreTaskPersistenceSuite) TestGetSemaphoreTaskBucketStateNotFound() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreTaskManager()
	s.NoError(err)
	defer manager.Close()

	_, err = manager.GetSemaphoreTaskBucketState(ctx, &persistence.GetSemaphoreTaskBucketStateRequest{
		DomainID:      uuid.NewString(),
		SemaphoreName: "does-not-exist",
		Bucket:        0,
	})
	s.Error(err)
}

// TestTaskQueueLifecycle walks the full bucket-owner flow:
// claim -> enqueue tasks -> read above ack -> advance ack -> range-delete -> count.
func (s *SemaphoreTaskPersistenceSuite) TestTaskQueueLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreTaskManager()
	s.NoError(err)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()
	bucket := 1

	claim, err := manager.ClaimSemaphoreTaskBucket(ctx, &persistence.ClaimSemaphoreTaskBucketRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
	})
	s.NoError(err)

	deadline := time.Now().UTC().Add(time.Hour)
	tasks := []*persistence.SemaphoreTask{
		{TaskID: 1, WorkflowID: "wf-1", RunID: uuid.NewString(), HoldID: 11, AcquireDeadline: &deadline},
		{TaskID: 2, WorkflowID: "wf-2", RunID: uuid.NewString(), HoldID: 12},
		{TaskID: 3, WorkflowID: "wf-3", RunID: uuid.NewString(), HoldID: 13},
	}
	_, err = manager.CreateSemaphoreTasks(ctx, &persistence.CreateSemaphoreTasksRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
		RangeID:       claim.RangeID,
		Tasks:         tasks,
	})
	s.NoError(err)

	// read everything above the ack level
	readResp, err := manager.GetSemaphoreTasks(ctx, &persistence.GetSemaphoreTasksRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
		ReadLevel:     claim.AckLevel,
		MaxReadLevel:  100,
		BatchSize:     10,
	})
	s.NoError(err)
	s.Len(readResp.Tasks, 3)
	// FIFO order by task_id
	s.Equal(int64(1), readResp.Tasks[0].TaskID)
	s.Equal("wf-1", readResp.Tasks[0].WorkflowID)
	s.Equal(int64(11), readResp.Tasks[0].HoldID)
	s.NotNil(readResp.Tasks[0].AcquireDeadline)
	s.Equal(int64(3), readResp.Tasks[2].TaskID)
	s.Nil(readResp.Tasks[1].AcquireDeadline)

	countResp, err := manager.GetSemaphoreTasksCount(ctx, &persistence.GetSemaphoreTasksCountRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
		ReadLevel:     claim.AckLevel,
	})
	s.NoError(err)
	s.Equal(int64(3), countResp.Count)

	// grant the first two: advance ack_level then drain the granted rows
	_, err = manager.UpdateSemaphoreTaskBucketState(ctx, &persistence.UpdateSemaphoreTaskBucketStateRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
		RangeID:       claim.RangeID,
		AckLevel:      2,
	})
	s.NoError(err)

	_, err = manager.RangeCompleteSemaphoreTasks(ctx, &persistence.RangeCompleteSemaphoreTasksRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
		ReadLevel:     0,
		AckLevel:      2,
	})
	s.NoError(err)

	afterResp, err := manager.GetSemaphoreTasks(ctx, &persistence.GetSemaphoreTasksRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
		ReadLevel:     2,
		MaxReadLevel:  100,
		BatchSize:     10,
	})
	s.NoError(err)
	s.Len(afterResp.Tasks, 1)
	s.Equal(int64(3), afterResp.Tasks[0].TaskID)

	bucketResp, err := manager.GetSemaphoreTaskBucketState(ctx, &persistence.GetSemaphoreTaskBucketStateRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
	})
	s.NoError(err)
	s.Equal(int64(2), bucketResp.AckLevel)
}

// TestStaleRangeIDIsFencedOut verifies a writer that lost the bucket cannot enqueue or advance the
// ack_level: both operations must fail with ConditionFailedError once range_id has moved on.
func (s *SemaphoreTaskPersistenceSuite) TestStaleRangeIDIsFencedOut() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreTaskManager()
	s.NoError(err)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()
	bucket := 2

	stale, err := manager.ClaimSemaphoreTaskBucket(ctx, &persistence.ClaimSemaphoreTaskBucketRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
	})
	s.NoError(err)

	// a new owner takes over the bucket
	_, err = manager.ClaimSemaphoreTaskBucket(ctx, &persistence.ClaimSemaphoreTaskBucketRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
	})
	s.NoError(err)

	_, err = manager.CreateSemaphoreTasks(ctx, &persistence.CreateSemaphoreTasksRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
		RangeID:       stale.RangeID,
		Tasks: []*persistence.SemaphoreTask{
			{TaskID: 1, WorkflowID: "wf-1", RunID: uuid.NewString(), HoldID: 11},
		},
	})
	s.Error(err)
	_, ok := err.(*persistence.ConditionFailedError)
	s.True(ok, "expected ConditionFailedError, got %T: %v", err, err)

	_, err = manager.UpdateSemaphoreTaskBucketState(ctx, &persistence.UpdateSemaphoreTaskBucketStateRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
		RangeID:       stale.RangeID,
		AckLevel:      5,
	})
	s.Error(err)
	_, ok = err.(*persistence.ConditionFailedError)
	s.True(ok, "expected ConditionFailedError, got %T: %v", err, err)
}

// TestBucketsAreIndependent verifies each (domain, semaphore, bucket) partition has its own
// control row and task queue.
func (s *SemaphoreTaskPersistenceSuite) TestBucketsAreIndependent() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreTaskManager()
	s.NoError(err)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()

	claimA, err := manager.ClaimSemaphoreTaskBucket(ctx, &persistence.ClaimSemaphoreTaskBucketRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: 0,
	})
	s.NoError(err)
	claimB, err := manager.ClaimSemaphoreTaskBucket(ctx, &persistence.ClaimSemaphoreTaskBucketRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: 1,
	})
	s.NoError(err)

	_, err = manager.CreateSemaphoreTasks(ctx, &persistence.CreateSemaphoreTasksRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: 0, RangeID: claimA.RangeID,
		Tasks: []*persistence.SemaphoreTask{
			{TaskID: 1, WorkflowID: "wf-1", RunID: uuid.NewString(), HoldID: 11},
		},
	})
	s.NoError(err)

	countA, err := manager.GetSemaphoreTasksCount(ctx, &persistence.GetSemaphoreTasksCountRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: 0, ReadLevel: 0,
	})
	s.NoError(err)
	s.Equal(int64(1), countA.Count)

	countB, err := manager.GetSemaphoreTasksCount(ctx, &persistence.GetSemaphoreTasksCountRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: 1, ReadLevel: 0,
	})
	s.NoError(err)
	s.Equal(int64(0), countB.Count)
	s.Equal(claimB.RangeID, int64(1))
}
