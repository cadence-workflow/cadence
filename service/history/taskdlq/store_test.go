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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
)

// mockBackend is a test double for HistoryDLQBackend.
type mockBackend struct {
	calledWith *createArgs
	err        error
}

type createArgs struct {
	shardID               int
	domainID              string
	clusterAttributeScope string
	clusterAttributeName  string
	task                  persistence.Task
}

func (m *mockBackend) CreateHistoryDLQTask(ctx context.Context, shardID int, domainID, clusterAttributeScope, clusterAttributeName string, task persistence.Task) error {
	m.calledWith = &createArgs{
		shardID:               shardID,
		domainID:              domainID,
		clusterAttributeScope: clusterAttributeScope,
		clusterAttributeName:  clusterAttributeName,
		task:                  task,
	}
	return m.err
}

func TestStoreImpl_AddTask_DelegatesAllFieldsToBackend(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backend := &mockBackend{}
	store := NewStore(backend)

	mockTask := persistence.NewMockTask(ctrl)
	mockTask.EXPECT().GetDomainID().Return("test-domain").AnyTimes()

	req := AddTaskRequest{
		ShardID:               42,
		DomainID:              "test-domain",
		ClusterAttributeScope: "scope",
		ClusterAttributeName:  "name",
		Task:                  mockTask,
	}

	err := store.AddTask(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, backend.calledWith)
	assert.Equal(t, 42, backend.calledWith.shardID)
	assert.Equal(t, "test-domain", backend.calledWith.domainID)
	assert.Equal(t, "scope", backend.calledWith.clusterAttributeScope)
	assert.Equal(t, "name", backend.calledWith.clusterAttributeName)
	assert.Equal(t, mockTask, backend.calledWith.task)
}

func TestStoreImpl_AddTask_PropagatesBackendError(t *testing.T) {
	sentinel := errors.New("backend error")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backend := &mockBackend{err: sentinel}
	store := NewStore(backend)

	mockTask := persistence.NewMockTask(ctrl)
	err := store.AddTask(context.Background(), AddTaskRequest{Task: mockTask})

	assert.ErrorIs(t, err, sentinel)
}

func TestStoreImpl_GetAckLevels_Panics(t *testing.T) {
	store := NewStore(&mockBackend{})
	assert.Panics(t, func() {
		_, _ = store.GetAckLevels(context.Background(), 1)
	})
}

func TestStoreImpl_GetAckLevelsForPartition_Panics(t *testing.T) {
	store := NewStore(&mockBackend{})
	assert.Panics(t, func() {
		_, _ = store.GetAckLevelsForPartition(context.Background(), 1, "", "", "")
	})
}

func TestStoreImpl_GetTasks_Panics(t *testing.T) {
	store := NewStore(&mockBackend{})
	assert.Panics(t, func() {
		_, _ = store.GetTasks(context.Background(), GetTasksRequest{})
	})
}

func TestStoreImpl_UpdateAckLevel_Panics(t *testing.T) {
	store := NewStore(&mockBackend{})
	assert.Panics(t, func() {
		_ = store.UpdateAckLevel(context.Background(), UpdateAckLevelRequest{})
	})
}

func TestStoreImpl_DeleteTasks_Panics(t *testing.T) {
	store := NewStore(&mockBackend{})
	assert.Panics(t, func() {
		_ = store.DeleteTasks(context.Background(), DeleteTasksRequest{})
	})
}
