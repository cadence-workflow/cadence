// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

const (
	testShardID      = 1234
	testCluster      = "test-cluster"
	testDomain       = "test-domain"
	testDomainID     = "0000001-0000002-0000003"
	testTaskList     = "test-tasklist"
	testTaskListType = "decision"
	testQueueType    = 2 // transfer queue
	testWorkflowID   = "test-workflow-id"
	testRunID        = "test-run-id"
)

type cliTestData struct {
	ctrl               *gomock.Controller
	mockFrontendClient *frontend.MockClient
	mockAdminClient    *admin.MockClient
	ioHandler          *testIOHandler
	app                *cli.App
	mockManagerFactory *MockManagerFactory
}

func newCLITestData(t *testing.T) *cliTestData {
	var td cliTestData

	td.ctrl = gomock.NewController(t)

	td.mockFrontendClient = frontend.NewMockClient(td.ctrl)
	td.mockAdminClient = admin.NewMockClient(td.ctrl)
	td.mockManagerFactory = NewMockManagerFactory(td.ctrl)
	td.ioHandler = &testIOHandler{}

	// Create a new CLI app with client factory and persistence manager factory
	td.app = NewCliApp(
		&clientFactoryMock{
			serverFrontendClient: td.mockFrontendClient,
			serverAdminClient:    td.mockAdminClient,
		},
		WithIOHandler(td.ioHandler),
		WithManagerFactory(td.mockManagerFactory), // Inject the mocked persistence manager factory
	)
	return &td
}

func (td *cliTestData) consoleOutput() string {
	return td.ioHandler.outputBytes.String()
}

func TestAdminResetQueue(t *testing.T) {
	tests := []struct {
		name           string
		testSetup      func(td *cliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "no shardID argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "missing cluster argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					/* no cluster argument */
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "missing queue type argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.StringArgument(FlagCluster, testCluster),
					/* no queue type argument */
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "all arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.StringArgument(FlagCluster, testCluster),
					clitest.IntArgument(FlagQueueType, testQueueType),
				)

				td.mockAdminClient.EXPECT().ResetQueue(gomock.Any(), &types.ResetQueueRequest{
					ShardID:     testShardID,
					ClusterName: testCluster,
					Type:        common.Int32Ptr(testQueueType),
				})

				return cliCtx
			},
			errContains:    "",
			expectedOutput: "Reset queue state succeeded\n",
		},
		{
			name: "ResetQueue returns an error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.StringArgument(FlagCluster, testCluster),
					clitest.IntArgument(FlagQueueType, testQueueType),
				)

				td.mockAdminClient.EXPECT().ResetQueue(gomock.Any(), gomock.Any()).
					Return(errors.New("critical error"))

				return cliCtx
			},
			errContains: "Failed to reset queue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminResetQueue(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestAdminDescribeQueue(t *testing.T) {
	tests := []struct {
		name           string
		testSetup      func(td *cliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "no shardID argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "missing cluster argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					/* no cluster argument */
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "missing queue type argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.StringArgument(FlagCluster, testCluster),
					/* no queue type argument */
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "all arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.StringArgument(FlagCluster, testCluster),
					clitest.IntArgument(FlagQueueType, testQueueType),
				)

				td.mockAdminClient.EXPECT().DescribeQueue(gomock.Any(), &types.DescribeQueueRequest{
					ShardID:     testShardID,
					ClusterName: testCluster,
					Type:        common.Int32Ptr(testQueueType),
				}).Return(&types.DescribeQueueResponse{
					ProcessingQueueStates: []string{"state1", "state2"},
				}, nil)

				return cliCtx
			},
			errContains:    "",
			expectedOutput: "state1\nstate2\n",
		},
		{
			name: "DescribeQueue returns an error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.StringArgument(FlagCluster, testCluster),
					clitest.IntArgument(FlagQueueType, testQueueType),
				)

				td.mockAdminClient.EXPECT().DescribeQueue(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("critical error"))

				return cliCtx
			},
			errContains: "Failed to describe queue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminDescribeQueue(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestAdminRefreshWorkflowTasks(t *testing.T) {
	tests := []struct {
		name           string
		testSetup      func(td *cliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "no domain argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "missing workflowID argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					/* no workflowID argument */
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "all arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagWorkflowID, testWorkflowID),
					clitest.StringArgument(FlagRunID, testRunID),
				)

				td.mockAdminClient.EXPECT().RefreshWorkflowTasks(gomock.Any(), &types.RefreshWorkflowTasksRequest{
					Domain: testDomain,
					Execution: &types.WorkflowExecution{
						WorkflowID: testWorkflowID,
						RunID:      testRunID,
					},
				}).Return(nil)

				return cliCtx
			},
			errContains:    "",
			expectedOutput: "Refresh workflow task succeeded.\n",
		},
		{
			name: "RefreshWorkflowTasks returns an error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagWorkflowID, testWorkflowID),
					clitest.StringArgument(FlagRunID, testRunID),
				)

				td.mockAdminClient.EXPECT().RefreshWorkflowTasks(gomock.Any(), gomock.Any()).
					Return(errors.New("critical error"))

				return cliCtx
			},
			errContains: "Refresh workflow task failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminRefreshWorkflowTasks(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestAdminDescribeHistoryHost(t *testing.T) {
	tests := []struct {
		name           string
		testSetup      func(td *cliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "no arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "at least one of them is required to provide to lookup host: workflowID, shardID and host address",
		},
		{
			name: "workflow-id provided, but empty",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument(FlagWorkflowID, ""))
			},
			errContains: "at least one of them is required to provide to lookup host: workflowID, shardID and host address",
		},
		{
			name: "addr provided, but empty",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument(FlagHistoryAddress, ""))
			},
			errContains: "at least one of them is required to provide to lookup host: workflowID, shardID and host address",
		},
		{
			name: "calling with all arguments",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.StringArgument(FlagWorkflowID, testWorkflowID),
					clitest.StringArgument(FlagHistoryAddress, "host:port"),
					clitest.BoolArgument(FlagPrintFullyDetail, false),
				)

				td.mockAdminClient.EXPECT().DescribeHistoryHost(gomock.Any(),
					&types.DescribeHistoryHostRequest{
						ExecutionForHost: &types.WorkflowExecution{WorkflowID: testWorkflowID},
						ShardIDForHost:   common.Int32Ptr(int32(testShardID)),
						HostAddress:      common.StringPtr("host:port"),
					}).Return(
					&types.DescribeHistoryHostResponse{NumberOfShards: 12, Address: "host:port"},
					nil,
				)

				return cliCtx
			},
			errContains: "",
			expectedOutput: `{
  "numberOfShards": 12,
  "address": "host:port"
}
`,
		},
		{
			name: "DescribeHistoryHost returns an error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagWorkflowID, testWorkflowID),
				)

				td.mockAdminClient.EXPECT().DescribeHistoryHost(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("critical error"))

				return cliCtx
			},
			errContains: "Describe history host failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminDescribeHistoryHost(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestAdminCloseShard(t *testing.T) {
	tests := []struct {
		name        string
		testSetup   func(td *cliTestData) *cli.Context
		errContains string // empty if no error is expected
	}{
		{
			name: "no arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required option not found",
		},
		{
			name: "calling with all arguments",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
				)

				td.mockAdminClient.EXPECT().CloseShard(gomock.Any(),
					&types.CloseShardRequest{
						ShardID: testShardID,
					}).Return(nil)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "CloseShard returns an error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
				)

				td.mockAdminClient.EXPECT().CloseShard(gomock.Any(), gomock.Any()).
					Return(errors.New("critical error"))

				return cliCtx
			},
			errContains: "Close shard task has failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminCloseShard(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func TestAdminRemoveTask(t *testing.T) {
	tests := []struct {
		name        string
		testSetup   func(td *cliTestData) *cli.Context
		errContains string // empty if no error is expected
	}{
		{
			name: "no arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "missing ShardID",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.Int64Argument(FlagTaskID, 123),
					clitest.IntArgument(FlagTaskType, 1), // type is provided
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "missing TaskID",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.IntArgument(FlagShardID, 1),
					clitest.IntArgument(FlagTaskType, 1), // type is provided
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "missing TaskType",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.IntArgument(FlagShardID, 1),
					clitest.Int64Argument(FlagTaskID, 123),
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "calling with all arguments",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.Int64Argument(FlagTaskID, 123),
					clitest.IntArgument(FlagTaskType, 1), // some valid type
				)

				td.mockAdminClient.EXPECT().RemoveTask(gomock.Any(),
					&types.RemoveTaskRequest{
						ShardID:             int32(testShardID),
						Type:                common.Int32Ptr(1),
						TaskID:              123,
						VisibilityTimestamp: common.Int64Ptr(0),
						ClusterName:         "",
					}).Return(nil)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "RemoveTask returns an error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.Int64Argument(FlagTaskID, 123),
					clitest.IntArgument(FlagTaskType, 1), // some valid type
				)

				td.mockAdminClient.EXPECT().RemoveTask(gomock.Any(), gomock.Any()).
					Return(errors.New("critical error"))

				return cliCtx
			},
			errContains: "Remove task has failed",
		},
		{
			name: "calling with Timer task requiring visibility timestamp",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.Int64Argument(FlagTaskID, 123),
					clitest.IntArgument(FlagTaskType, int(constants.TaskTypeTimer)),
					clitest.Int64Argument(FlagTaskVisibilityTimestamp, 1616161616), // visibility timestamp
				)

				td.mockAdminClient.EXPECT().RemoveTask(gomock.Any(),
					&types.RemoveTaskRequest{
						ShardID:             int32(testShardID),
						Type:                common.Int32Ptr(int32(constants.TaskTypeTimer)),
						TaskID:              123,
						VisibilityTimestamp: common.Int64Ptr(1616161616),
						ClusterName:         "",
					}).Return(nil)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "calling with Timer task requiring visibility timestamp, but not provided",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.Int64Argument(FlagTaskID, 123),
					clitest.IntArgument(FlagTaskType, int(constants.TaskTypeTimer)),
					// visibility timestamp is missing though FlagTaskType is common.TaskTypeTimer
				)

				return cliCtx
			},
			errContains: "Required flag not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminRemoveTask(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func TestAdminGetShardID(t *testing.T) {
	tests := []struct {
		name           string
		testSetup      func(td *cliTestData) *cli.Context
		expectedOutput string // expected output to check against
		errContains    string // empty if no error is expected
	}{
		{
			name: "no WorkflowID provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "numberOfShards not provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument(FlagWorkflowID, "some-workflow-id"),
					/* numberOfShards is missing */
				)
			},
			errContains: "numberOfShards is required",
		},
		{
			name: "numberOfShards is zero",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument(FlagWorkflowID, "some-workflow-id"),
					clitest.IntArgument(FlagNumberOfShards, 0), // zero is invalid
				)
			},
			errContains: "numberOfShards is required",
		},
		{
			name: "valid inputs",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(t, td.app,
					clitest.StringArgument(FlagWorkflowID, testWorkflowID),
					clitest.IntArgument(FlagNumberOfShards, 10), // valid number of shards
				)

				return cliCtx
			},
			expectedOutput: "ShardID for workflowID: test-workflow-id is 6\n",
			errContains:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminGetShardID(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}

			// If there is no error, check the output
			if tt.expectedOutput != "" {
				assert.Contains(t, tt.expectedOutput, td.consoleOutput())
			}
		})
	}
}

func TestAdminDescribeShardDistribution(t *testing.T) {
	tests := []struct {
		name           string
		testSetup      func(td *cliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "all arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagPageSize, 10),
					clitest.IntArgument(FlagPageID, 1),
					clitest.StringArgument(FlagFormat, formatJSON),
				)

				td.mockAdminClient.EXPECT().DescribeShardDistribution(
					gomock.Any(),
					&types.DescribeShardDistributionRequest{
						PageSize: 10,
						PageID:   1,
					}).Return(
					&types.DescribeShardDistributionResponse{
						NumberOfShards: 1,
						Shards: map[int32]string{
							1: "identity1",
						},
					}, nil,
				)

				return cliCtx
			},
			errContains: "",
			expectedOutput: `Total Number of Shards: 1
Number of Shards Returned: 1
[
  {
    "ShardID": 1,
    "Identity": "identity1"
  }
]
`,
		},
		{
			name: "no shards are returned",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagPageSize, 10),
					clitest.IntArgument(FlagPageID, 1),
					clitest.StringArgument(FlagFormat, formatJSON),
				)

				td.mockAdminClient.EXPECT().DescribeShardDistribution(gomock.Any(), gomock.Any()).
					Return(
						&types.DescribeShardDistributionResponse{
							NumberOfShards: 10,
							Shards:         nil, // no shards
						}, nil,
					)

				return cliCtx
			},
			errContains:    "",
			expectedOutput: "Total Number of Shards: 10\nNumber of Shards Returned: 0\n",
		},
		{
			name: "DescribeShardDistribution returns an error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagPageSize, 10),
					clitest.IntArgument(FlagPageID, 1),
				)

				td.mockAdminClient.EXPECT().DescribeShardDistribution(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("critical error"))

				return cliCtx
			},
			errContains: "Shard list failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminDescribeShardDistribution(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestAdminMaintainCorruptWorkflow(t *testing.T) {
	tests := []struct {
		name        string
		testSetup   func(td *cliTestData) *cli.Context
		errContains string // empty if no error is expected
	}{
		{
			name: "no domain argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "all arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagWorkflowID, testWorkflowID),
					clitest.StringArgument(FlagRunID, testRunID),
					clitest.BoolArgument(FlagSkipErrorMode, true),
				)

				td.mockAdminClient.EXPECT().MaintainCorruptWorkflow(gomock.Any(), &types.AdminMaintainWorkflowRequest{
					Domain: testDomain,
					Execution: &types.WorkflowExecution{
						WorkflowID: testWorkflowID,
						RunID:      testRunID,
					},
					SkipErrors: true,
				}).Return(nil, nil)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "MaintainCorruptWorkflow returns an error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagWorkflowID, testWorkflowID),
					clitest.StringArgument(FlagRunID, testRunID),
					clitest.BoolArgument(FlagSkipErrorMode, false),
				)

				td.mockAdminClient.EXPECT().MaintainCorruptWorkflow(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("critical error"))

				return cliCtx
			},
			errContains: "Operation AdminMaintainCorruptWorkflow failed.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminMaintainCorruptWorkflow(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func TestAdminDescribeShard(t *testing.T) {
	tests := []struct {
		name        string
		testSetup   func(td *cliTestData) *cli.Context
		errContains string // empty if no error is expected
		checkOutput func(td *cliTestData)
	}{
		{
			name: "no ShardID argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "all arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
				)

				mockShardManager := persistence.NewMockShardManager(td.ctrl)
				mockShardManager.EXPECT().GetShard(
					gomock.Any(),
					&persistence.GetShardRequest{ShardID: testShardID},
				).Return(&persistence.GetShardResponse{
					ShardInfo: &persistence.ShardInfo{
						ShardID: testShardID,
						Owner:   "host-abc",
					},
				}, nil)

				td.mockManagerFactory.EXPECT().initializeShardManager(gomock.Any()).
					Return(mockShardManager, nil)

				return cliCtx
			},
			errContains: "",
			checkOutput: func(td *cliTestData) {
				// We must have visualised GetShardResponse in console.
				// Check it is a valid JSON and important fields are there
				var resp persistence.GetShardResponse

				require.NoError(t, json.Unmarshal([]byte(td.consoleOutput()), &resp))
				assert.Equal(t, testShardID, resp.ShardInfo.ShardID)
				assert.Equal(t, "host-abc", resp.ShardInfo.Owner)
			},
		},
		{
			name: "all arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
				)

				mockShardManager := persistence.NewMockShardManager(td.ctrl)
				mockShardManager.EXPECT().GetShard(
					gomock.Any(),
					&persistence.GetShardRequest{ShardID: testShardID},
				).Return(&persistence.GetShardResponse{
					ShardInfo: &persistence.ShardInfo{
						ShardID: testShardID,
						Owner:   "host-abc",
					},
				}, nil)

				td.mockManagerFactory.EXPECT().initializeShardManager(gomock.Any()).
					Return(mockShardManager, nil)

				return cliCtx
			},
			errContains: "",
			checkOutput: func(td *cliTestData) {
				// We must have visualised GetShardResponse in console.
				// Check it is a valid JSON and important fields are there
				var resp persistence.GetShardResponse

				require.NoError(t, json.Unmarshal([]byte(td.consoleOutput()), &resp))
				assert.Equal(t, testShardID, resp.ShardInfo.ShardID)
				assert.Equal(t, "host-abc", resp.ShardInfo.Owner)
			},
		},
		{
			name: "GetShard returns an error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
				)

				mockShardManager := persistence.NewMockShardManager(td.ctrl)
				mockShardManager.EXPECT().GetShard(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("critical error"))

				td.mockManagerFactory.EXPECT().initializeShardManager(gomock.Any()).
					Return(mockShardManager, nil)

				return cliCtx
			},
			errContains: "Failed to describe shard",
		},
		{
			name: "failed to initializeShardManager",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
				)

				td.mockManagerFactory.EXPECT().initializeShardManager(gomock.Any()).
					Return(nil, errors.New("failed to initializeShardManager"))

				return cliCtx
			},
			errContains: "failed to initializeShardManager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminDescribeShard(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
				require.NotNil(t, tt.checkOutput)
				tt.checkOutput(td)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func TestAdminSetShardRangeID(t *testing.T) {
	tests := []struct {
		name           string
		testSetup      func(td *cliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "no ShardID argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "no RangeID argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					// FlagRangeID is missing
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "all arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.Int64Argument(FlagRangeID, 133),
				)

				mockShardManager := persistence.NewMockShardManager(td.ctrl)

				mockShardManager.EXPECT().GetShard(
					gomock.Any(),
					&persistence.GetShardRequest{ShardID: testShardID},
				).Return(&persistence.GetShardResponse{
					ShardInfo: &persistence.ShardInfo{
						ShardID:          testShardID,
						Owner:            "host-abc",
						RangeID:          123,
						StolenSinceRenew: 100, // this supposed to be incremented
					},
				}, nil)

				mockShardManager.EXPECT().UpdateShard(
					gomock.Any(),
					gomock.Any(),
				).Do(func(ctx context.Context, req *persistence.UpdateShardRequest) {
					// we can't use input arguments matching as it contains current time
					assert.Equal(t, int64(123), req.PreviousRangeID)
					assert.Equal(t, 101, req.ShardInfo.StolenSinceRenew)
					assert.Equal(t, int64(133), req.ShardInfo.RangeID)

					now := time.Now()

					// check time is really updated
					assert.WithinRange(
						t,
						req.ShardInfo.UpdatedAt,
						now.Add(-time.Minute),
						time.Now(),
						"didn't update UpdatedAt?",
					)
				}).Return(nil)

				td.mockManagerFactory.EXPECT().initializeShardManager(gomock.Any()).
					Return(mockShardManager, nil)

				return cliCtx
			},
			errContains:    "",
			expectedOutput: "Successfully updated rangeID from 123 to 133 for shard 1234.\n",
		},
		{
			name: "all arguments provided, but UpdateShard fails",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.Int64Argument(FlagRangeID, 133),
				)

				mockShardManager := persistence.NewMockShardManager(td.ctrl)

				mockShardManager.EXPECT().GetShard(
					gomock.Any(),
					gomock.Any(),
				).Return(&persistence.GetShardResponse{
					ShardInfo: &persistence.ShardInfo{
						ShardID: testShardID,
					},
				}, nil)

				mockShardManager.EXPECT().UpdateShard(
					gomock.Any(),
					gomock.Any(),
				).Return(errors.New("critical failure"))

				td.mockManagerFactory.EXPECT().initializeShardManager(gomock.Any()).
					Return(mockShardManager, nil)

				return cliCtx
			},
			errContains: "Failed to reset shard rangeID.",
		},
		{
			name: "GetShard returns an error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.Int64Argument(FlagRangeID, 123),
				)

				mockShardManager := persistence.NewMockShardManager(td.ctrl)
				mockShardManager.EXPECT().GetShard(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("critical error"))

				td.mockManagerFactory.EXPECT().initializeShardManager(gomock.Any()).
					Return(mockShardManager, nil)

				return cliCtx
			},
			errContains: "Failed to get shardInfo.",
		},
		{
			name: "failed to initializeShardManager",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.IntArgument(FlagShardID, testShardID),
					clitest.Int64Argument(FlagRangeID, 123),
				)

				td.mockManagerFactory.EXPECT().initializeShardManager(gomock.Any()).
					Return(nil, errors.New("failed to initializeShardManager"))

				return cliCtx
			},
			errContains: "failed to initializeShardManager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminSetShardRangeID(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestAdminGetDomainIDOrName(t *testing.T) {
	tests := []struct {
		name           string
		testSetup      func(td *cliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "no DomainID or DomainName argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Need either domainName or domainID",
		},
		{
			name: "DomainID provided, valid response",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomainID, testDomainID),
				)

				mockDomainManager := persistence.NewMockDomainManager(td.ctrl)
				mockDomainManager.EXPECT().GetDomain(
					gomock.Any(),
					&persistence.GetDomainRequest{ID: testDomainID},
				).Return(&persistence.GetDomainResponse{
					Info: &persistence.DomainInfo{
						ID:   testDomainID,
						Name: testDomain,
					},
				}, nil)

				td.mockManagerFactory.EXPECT().initializeDomainManager(gomock.Any()).
					Return(mockDomainManager, nil)

				return cliCtx
			},
			errContains:    "",
			expectedOutput: fmt.Sprintf("domainName for domainID %v is %v\n", testDomainID, testDomain),
		},
		{
			name: "DomainName provided, valid response",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
				)

				mockDomainManager := persistence.NewMockDomainManager(td.ctrl)
				mockDomainManager.EXPECT().GetDomain(
					gomock.Any(),
					&persistence.GetDomainRequest{Name: testDomain},
				).Return(&persistence.GetDomainResponse{
					Info: &persistence.DomainInfo{
						ID:   testDomainID,
						Name: testDomain,
					},
				}, nil)

				td.mockManagerFactory.EXPECT().initializeDomainManager(gomock.Any()).
					Return(mockDomainManager, nil)

				return cliCtx
			},
			errContains:    "",
			expectedOutput: fmt.Sprintf("domainID for domainName %v is %v\n", testDomain, testDomainID),
		},
		{
			name: "DomainManager returns an error for domainID",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomainID, testDomainID),
				)

				mockDomainManager := persistence.NewMockDomainManager(td.ctrl)
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("critical error"))

				td.mockManagerFactory.EXPECT().initializeDomainManager(gomock.Any()).
					Return(mockDomainManager, nil)

				return cliCtx
			},
			errContains: "GetDomain error",
		},
		{
			name: "DomainManager returns an error for domainName",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
				)

				mockDomainManager := persistence.NewMockDomainManager(td.ctrl)
				mockDomainManager.EXPECT().GetDomain(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("critical error"))

				td.mockManagerFactory.EXPECT().initializeDomainManager(gomock.Any()).
					Return(mockDomainManager, nil)

				return cliCtx
			},
			errContains: "GetDomain error",
		},
		{
			name: "failed to initializeDomainManager",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomainID, testDomainID),
				)

				td.mockManagerFactory.EXPECT().initializeDomainManager(gomock.Any()).
					Return(nil, errors.New("failed to initializeDomainManager"))

				return cliCtx
			},
			errContains: "failed to initializeDomainManager",
		},
		{
			name: "both DomainID and DomainName provided",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomainID, testDomainID),
					clitest.StringArgument(FlagDomain, testDomain),
				)

				return cliCtx
			},
			errContains: "Need either domainName or domainID", // Expecting the error when both are provided
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminGetDomainIDOrName(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestAdminDeleteWorkflow(t *testing.T) {
	tests := []struct {
		name        string
		testSetup   func(td *cliTestData) *cli.Context
		errContains string // empty if no error is expected
	}{
		{
			name: "no domain argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "no workflow ID argument",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "remote delete success",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagWorkflowID, testWorkflowID),
					clitest.StringArgument(FlagRunID, testRunID),
					clitest.BoolArgument(FlagRemote, true),
					clitest.BoolArgument(FlagSkipErrorMode, false),
				)

				td.mockAdminClient.EXPECT().DeleteWorkflow(gomock.Any(), &types.AdminDeleteWorkflowRequest{
					Domain: testDomain,
					Execution: &types.WorkflowExecution{
						WorkflowID: testWorkflowID,
						RunID:      testRunID,
					},
					SkipErrors: false,
				}).Return(&types.AdminDeleteWorkflowResponse{}, nil)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "remote delete returns error",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagWorkflowID, testWorkflowID),
					clitest.StringArgument(FlagRunID, testRunID),
					clitest.BoolArgument(FlagRemote, true),
				)

				td.mockAdminClient.EXPECT().DeleteWorkflow(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("delete failed"))

				return cliCtx
			},
			errContains: "Operation AdminMaintainCorruptWorkflow failed.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminDeleteWorkflow(cliCtx)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}
