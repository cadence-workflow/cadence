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

package persistence

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
)

type semaphoreTaskManagerImpl struct {
	persistence SemaphoreTaskStore
	logger      log.Logger
	timeSrc     clock.TimeSource
}

// NewSemaphoreTaskManagerImpl returns a new SemaphoreTaskManager
func NewSemaphoreTaskManagerImpl(persistence SemaphoreTaskStore, logger log.Logger) SemaphoreTaskManager {
	return &semaphoreTaskManagerImpl{
		persistence: persistence,
		logger:      logger,
		timeSrc:     clock.NewRealTimeSource(),
	}
}

func (m *semaphoreTaskManagerImpl) GetName() string {
	return m.persistence.GetName()
}

func (m *semaphoreTaskManagerImpl) Close() {
	m.persistence.Close()
}

func (m *semaphoreTaskManagerImpl) ClaimSemaphoreTaskBucket(
	ctx context.Context,
	request *ClaimSemaphoreTaskBucketRequest,
) (*ClaimSemaphoreTaskBucketResponse, error) {
	if err := validateBucketIdentity(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	// Zero is the "I hold nothing, take the bucket" case; negative is never a real range_id.
	if request.RangeID < 0 {
		return nil, fmt.Errorf("RangeID must not be negative, got %d", request.RangeID)
	}
	return m.persistence.ClaimSemaphoreTaskBucket(ctx, request)
}

func (m *semaphoreTaskManagerImpl) GetSemaphoreTaskBucketState(
	ctx context.Context,
	request *GetSemaphoreTaskBucketStateRequest,
) (*GetSemaphoreTaskBucketStateResponse, error) {
	if err := validateBucketIdentity(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	return m.persistence.GetSemaphoreTaskBucketState(ctx, request)
}

// UpdateSemaphoreTaskBucketState writes AckLevel as given. The RangeID fence proves the caller still owns
// the bucket but does not constrain AckLevel: a value below the persisted one rewinds the cursor
// and re-reads acked tasks. Monotonicity is enforced by the bucket owner's in-memory cursor, not
// at this layer; see messaging.AckManager.SetAckLevel.
func (m *semaphoreTaskManagerImpl) UpdateSemaphoreTaskBucketState(
	ctx context.Context,
	request *UpdateSemaphoreTaskBucketStateRequest,
) (*UpdateSemaphoreTaskBucketStateResponse, error) {
	if err := validateBucketIdentity(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	if request.RangeID <= 0 {
		return nil, fmt.Errorf("RangeID must be positive, got %d", request.RangeID)
	}
	if request.AckLevel < 0 {
		return nil, fmt.Errorf("AckLevel must not be negative, got %d", request.AckLevel)
	}
	return m.persistence.UpdateSemaphoreTaskBucketState(ctx, request)
}

func (m *semaphoreTaskManagerImpl) CreateSemaphoreTasks(
	ctx context.Context,
	request *CreateSemaphoreTasksRequest,
) (*CreateSemaphoreTasksResponse, error) {
	if err := validateBucketIdentity(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	if request.RangeID <= 0 {
		return nil, fmt.Errorf("RangeID must be positive, got %d", request.RangeID)
	}
	if len(request.Tasks) == 0 {
		return nil, fmt.Errorf("Tasks must not be empty")
	}
	// Stamp CreatedTime for any task that lacks one.
	now := m.timeSrc.Now().UTC()
	for _, w := range request.Tasks {
		if w.CreatedTime.IsZero() {
			w.CreatedTime = now
		}
	}
	return m.persistence.CreateSemaphoreTasks(ctx, request)
}

func (m *semaphoreTaskManagerImpl) GetSemaphoreTasks(
	ctx context.Context,
	request *GetSemaphoreTasksRequest,
) (*GetSemaphoreTasksResponse, error) {
	if err := validateBucketIdentity(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	// A non-positive BatchSize would not read zero rows: it disables the page limit and the
	// store's early exit, turning the read into an unbounded scan of the bucket partition.
	if request.BatchSize <= 0 {
		return nil, fmt.Errorf("BatchSize must be positive, got %d", request.BatchSize)
	}
	return m.persistence.GetSemaphoreTasks(ctx, request)
}

func (m *semaphoreTaskManagerImpl) CompleteSemaphoreTasksLessThan(
	ctx context.Context,
	request *CompleteSemaphoreTasksLessThanRequest,
) (*CompleteSemaphoreTasksLessThanResponse, error) {
	if err := validateBucketIdentity(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	return m.persistence.CompleteSemaphoreTasksLessThan(ctx, request)
}

func (m *semaphoreTaskManagerImpl) GetSemaphoreTasksCount(
	ctx context.Context,
	request *GetSemaphoreTasksCountRequest,
) (*GetSemaphoreTasksCountResponse, error) {
	if err := validateBucketIdentity(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	return m.persistence.GetSemaphoreTasksCount(ctx, request)
}

func validateBucketIdentity(domainID, semaphoreName string, bucket int) error {
	if domainID == "" {
		return fmt.Errorf("DomainID is required")
	}
	if semaphoreName == "" {
		return fmt.Errorf("SemaphoreName is required")
	}
	if bucket < 0 {
		return fmt.Errorf("Bucket must not be negative, got %d", bucket)
	}
	return nil
}
