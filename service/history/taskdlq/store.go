// Copyright (c) 2017-2020 Uber Technologies, Inc.
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

package taskdlq

import (
	"context"

	"github.com/uber/cadence/common/persistence"
)

// HistoryDLQBackend is the persistence-layer contract for writing to the history task DLQ.
// This interface will be satisfied by persistence.HistoryTaskDLQManager when the persistence layer
// is written. TODO(c-warren): Remove this file once the persistence layer is written.
type HistoryDLQBackend interface {
	CreateHistoryDLQTask(ctx context.Context, shardID int, domainID, clusterAttributeScope, clusterAttributeName string, task persistence.Task) error
}

type storeImpl struct {
	backend HistoryDLQBackend
}

// NewStore creates a HistoryTaskDLQStore backed by the given persistence backend.
func NewStore(backend HistoryDLQBackend) HistoryTaskDLQStore {
	return &storeImpl{backend: backend}
}

func (s *storeImpl) AddTask(ctx context.Context, req AddTaskRequest) error {
	return s.backend.CreateHistoryDLQTask(
		ctx,
		req.ShardID,
		req.DomainID,
		req.ClusterAttributeScope,
		req.ClusterAttributeName,
		req.Task,
	)
}

func (s *storeImpl) GetAckLevels(_ context.Context, _ int) ([]AckLevel, error) {
	panic("storeImpl.GetAckLevels: not yet implemented — requires DLQ processor wiring")
}

func (s *storeImpl) GetAckLevelsForPartition(_ context.Context, _ int, _, _, _ string) ([]AckLevel, error) {
	panic("storeImpl.GetAckLevelsForPartition: not yet implemented — requires DLQ processor wiring")
}

func (s *storeImpl) GetTasks(_ context.Context, _ GetTasksRequest) (GetTasksResponse, error) {
	panic("storeImpl.GetTasks: not yet implemented — requires DLQ processor wiring")
}

func (s *storeImpl) UpdateAckLevel(_ context.Context, _ UpdateAckLevelRequest) error {
	panic("storeImpl.UpdateAckLevel: not yet implemented — requires DLQ processor wiring")
}

func (s *storeImpl) DeleteTasks(_ context.Context, _ DeleteTasksRequest) error {
	panic("storeImpl.DeleteTasks: not yet implemented — requires DLQ processor wiring")
}
