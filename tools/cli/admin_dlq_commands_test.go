// Copyright (c) 2020 Uber Technologies, Inc.
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

package cli

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

// withStdinShards replaces os.Stdin with a pipe feeding the given shard IDs so
// getShards reads them deterministically regardless of the test environment.
func withStdinShards(t *testing.T, shards string) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, err = io.WriteString(w, shards)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
}

func TestAdminGetDLQMessages(t *testing.T) {
	const taskID int64 = 1

	hydratedTask := &types.ReplicationTask{
		TaskType:                types.ReplicationTaskTypeHistoryV2.Ptr(),
		SourceTaskID:            taskID,
		HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{},
	}

	tests := []struct {
		name             string
		replicationTasks []*types.ReplicationTask
	}{
		{
			// Regression test: the history service returns nil entries in
			// ReplicationTasks for tasks it could not hydrate (e.g. the source
			// workflow was deleted). The CLI used to dereference task.SourceTaskID
			// unconditionally and panic.
			name:             "nil replication task is skipped without panic",
			replicationTasks: []*types.ReplicationTask{nil},
		},
		{
			name:             "hydrated replication task",
			replicationTasks: []*types.ReplicationTask{hydratedTask},
		},
		{
			name:             "mix of nil and hydrated tasks",
			replicationTasks: []*types.ReplicationTask{nil, hydratedTask},
		},
		{
			name:             "no tasks",
			replicationTasks: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			withStdinShards(t, "1\n")

			cliCtx := clitest.NewCLIContext(
				t,
				td.app,
				clitest.StringArgument(FlagDLQType, "history"),
				clitest.StringArgument(FlagSourceCluster, testCluster),
			)

			td.mockAdminClient.EXPECT().
				ReadDLQMessages(gomock.Any(), gomock.Any()).
				Return(&types.ReadDLQMessagesResponse{
					ReplicationTasks: tt.replicationTasks,
					ReplicationTasksInfo: []*types.ReplicationTaskInfo{
						{DomainID: testDomainID, WorkflowID: testWorkflowID, RunID: testRunID, TaskID: taskID},
					},
				}, nil).
				Times(1)

			td.mockFrontendClient.EXPECT().
				DescribeDomain(gomock.Any(), gomock.Any()).
				Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{Name: testDomain},
				}, nil).
				AnyTimes()

			assert.NotPanics(t, func() {
				err := AdminGetDLQMessages(cliCtx)
				assert.NoError(t, err)
			})
		})
	}
}
