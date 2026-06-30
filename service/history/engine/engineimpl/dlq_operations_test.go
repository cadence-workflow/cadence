// Copyright (c) 2017-2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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

package engineimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/proto"
)

// fakeDLQHandler is a minimal replication.DLQHandler used to drive
// historyEngineImpl.ReadDLQMessages in isolation.
type fakeDLQHandler struct {
	tasks    []*types.ReplicationTask
	taskInfo []*types.ReplicationTaskInfo
	token    []byte
}

func (f *fakeDLQHandler) Start() {}
func (f *fakeDLQHandler) Stop()  {}

func (f *fakeDLQHandler) GetMessageCount(ctx context.Context, forceFetch bool) (map[string]int64, error) {
	return nil, nil
}

func (f *fakeDLQHandler) ReadMessages(
	ctx context.Context,
	sourceCluster string,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]*types.ReplicationTask, []*types.ReplicationTaskInfo, []byte, error) {
	return f.tasks, f.taskInfo, f.token, nil
}

func (f *fakeDLQHandler) PurgeMessages(ctx context.Context, sourceCluster string, lastMessageID int64) error {
	return nil
}

func (f *fakeDLQHandler) MergeMessages(
	ctx context.Context,
	sourceCluster string,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]byte, error) {
	return nil, nil
}

func TestReadDLQMessages_DropsNilTasks(t *testing.T) {
	task1 := &types.ReplicationTask{TaskType: types.ReplicationTaskTypeHistoryV2.Ptr(), SourceTaskID: 1}
	task3 := &types.ReplicationTask{TaskType: types.ReplicationTaskTypeHistoryV2.Ptr(), SourceTaskID: 3}

	tests := []struct {
		name      string
		tasks     []*types.ReplicationTask
		taskInfo  []*types.ReplicationTaskInfo
		wantTasks []*types.ReplicationTask
	}{
		{
			name:      "no tasks",
			wantTasks: []*types.ReplicationTask{},
		},
		{
			name:      "all hydrated",
			tasks:     []*types.ReplicationTask{task1, task3},
			taskInfo:  []*types.ReplicationTaskInfo{{TaskID: 1}, {TaskID: 3}},
			wantTasks: []*types.ReplicationTask{task1, task3},
		},
		{
			// Regression: ReadMessages returns nil entries for tasks that could not be
			// hydrated. A nil *ReplicationTask cannot be marshaled into the RPC response
			// and used to panic the history service during serialization.
			name:      "nil task is dropped",
			tasks:     []*types.ReplicationTask{task1, nil, task3},
			taskInfo:  []*types.ReplicationTaskInfo{{TaskID: 1}, {TaskID: 2}, {TaskID: 3}},
			wantTasks: []*types.ReplicationTask{task1, task3},
		},
		{
			name:      "only nil tasks",
			tasks:     []*types.ReplicationTask{nil, nil},
			taskInfo:  []*types.ReplicationTaskInfo{{TaskID: 1}, {TaskID: 2}},
			wantTasks: []*types.ReplicationTask{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &historyEngineImpl{
				replicationDLQHandler: &fakeDLQHandler{tasks: tt.tasks, taskInfo: tt.taskInfo},
			}

			resp, err := e.ReadDLQMessages(context.Background(), &types.ReadDLQMessagesRequest{
				Type:          types.DLQTypeReplication.Ptr(),
				SourceCluster: "source",
			})
			require.NoError(t, err)

			assert.Equal(t, tt.wantTasks, resp.ReplicationTasks)
			assert.NotContains(t, resp.ReplicationTasks, (*types.ReplicationTask)(nil))
			// The full set of tasks is still represented by their info entries.
			assert.Equal(t, tt.taskInfo, resp.ReplicationTasksInfo)

			// The response must be serializable: a nil entry would panic the generated
			// protobuf MarshalToSizedBuffer, which is the bug this guards against.
			assert.NotPanics(t, func() {
				_, marshalErr := proto.FromAdminReadDLQMessagesResponse(resp).Marshal()
				assert.NoError(t, marshalErr)
			})
		})
	}
}
