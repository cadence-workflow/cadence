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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

func TestAdminDescribeTaskList(t *testing.T) {
	td := newCLITestData(t)

	expectedResponse := &types.DescribeTaskListResponse{
		Pollers: []*types.PollerInfo{
			{
				Identity: "test-poller",
			},
		},
		TaskListStatus: &types.TaskListStatus{
			BacklogCountHint: 10,
		},
	}
	td.mockFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(expectedResponse, nil).Times(1)

	cliCtx := newTaskListCLIContext(t, td.app)
	err := AdminDescribeTaskList(cliCtx)
	assert.NoError(t, err)
}

func TestAdminDescribeTaskList_DescribeTaskListFails(t *testing.T) {
	td := newCLITestData(t)

	td.mockFrontendClient.EXPECT().
		DescribeTaskList(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("DescribeTaskList failed")).
		Times(1)

	cliCtx := newTaskListCLIContext(t, td.app)
	err := AdminDescribeTaskList(cliCtx)
	assert.ErrorContains(t, err, "Operation DescribeTaskList failed.")
}

func TestAdminDescribeTaskList_NoTaskListStatus(t *testing.T) {
	td := newCLITestData(t)

	expectedResponse := &types.DescribeTaskListResponse{
		Pollers:        []*types.PollerInfo{},
		TaskListStatus: nil,
	}

	td.mockFrontendClient.EXPECT().
		DescribeTaskList(gomock.Any(), gomock.Any()).
		Return(expectedResponse, nil).
		Times(1)

	cliCtx := newTaskListCLIContext(t, td.app)
	err := AdminDescribeTaskList(cliCtx)
	assert.ErrorContains(t, err, "No tasklist status information.")
}

func TestAdminDescribeTaskList_NoPollers(t *testing.T) {
	td := newCLITestData(t)
	expectedResponse := &types.DescribeTaskListResponse{
		Pollers: []*types.PollerInfo{},
		TaskListStatus: &types.TaskListStatus{
			BacklogCountHint: 0,
		},
	}

	td.mockFrontendClient.EXPECT().
		DescribeTaskList(gomock.Any(), gomock.Any()).
		Return(expectedResponse, nil).
		Times(1)

	cliCtx := newTaskListCLIContext(t, td.app)
	err := AdminDescribeTaskList(cliCtx)
	assert.ErrorContains(t, err, "No poller for tasklist: test-tasklist")
}

func TestAdminDescribeTaskList_GetRequiredOptionDomainError(t *testing.T) {
	td := newCLITestData(t)

	cliCtx := clitest.NewCLIContext(
		t,
		td.app,
		/* omit the domain flag */
		clitest.StringArgument(FlagTaskList, testTaskList),
		clitest.StringArgument(FlagTaskListType, testTaskListType),
	)

	err := AdminDescribeTaskList(cliCtx)
	assert.ErrorContains(t, err, "Required flag not found: ")
}

func TestAdminDescribeTaskList_GetRequiredOptionTaskListError(t *testing.T) {
	td := newCLITestData(t)

	cliCtx := clitest.NewCLIContext(
		t,
		td.app,
		clitest.StringArgument(FlagDomain, testDomain),
		/* omit the task-list flag */
		clitest.StringArgument(FlagTaskListType, testTaskListType),
	)

	err := AdminDescribeTaskList(cliCtx)
	assert.ErrorContains(t, err, "Required flag not found: ")
}

func TestAdminDescribeTaskList_InvalidTaskListType(t *testing.T) {
	td := newCLITestData(t)

	expectedResponse := &types.DescribeTaskListResponse{
		Pollers: []*types.PollerInfo{
			{
				Identity: "test-poller",
			},
		},
		TaskListStatus: &types.TaskListStatus{
			BacklogCountHint: 10,
		},
	}
	td.mockFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(expectedResponse, nil).Times(1)

	cliCtx := clitest.NewCLIContext(
		t,
		td.app,
		clitest.StringArgument(FlagDomain, testDomain),
		clitest.StringArgument(FlagTaskList, testTaskList),
		clitest.StringArgument(FlagTaskListType, "activity"),
	)
	err := AdminDescribeTaskList(cliCtx)
	assert.NoError(t, err)
}

func TestAdminListTaskList(t *testing.T) {
	// Define table of test cases
	tests := []struct {
		name          string
		setupMocks    func(*frontend.MockClient)
		expectedError string
		domainFlag    string
		taskListFlag  string
		taskListType  string
	}{
		{
			name: "Success",
			setupMocks: func(client *frontend.MockClient) {
				expectedResponse := &types.GetTaskListsByDomainResponse{
					DecisionTaskListMap: map[string]*types.DescribeTaskListResponse{
						"decision-tasklist-1": {
							Pollers: []*types.PollerInfo{
								{Identity: "poller1"},
								{Identity: "poller2"},
							},
						},
						"decision-tasklist-2": {
							Pollers: []*types.PollerInfo{
								{Identity: "poller3"},
							},
						},
					},
					ActivityTaskListMap: map[string]*types.DescribeTaskListResponse{
						"activity-tasklist-1": {
							Pollers: []*types.PollerInfo{
								{Identity: "poller4"},
							},
						},
					},
				}
				client.EXPECT().
					GetTaskListsByDomain(gomock.Any(), gomock.Any()).
					Return(expectedResponse, nil).
					Times(1)
			},
			expectedError: "",
			domainFlag:    "test-domain",
			taskListFlag:  "test-tasklist",
			taskListType:  "decision",
		},
		{
			name: "GetTaskListsByDomainFails",
			setupMocks: func(client *frontend.MockClient) {
				client.EXPECT().
					GetTaskListsByDomain(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("GetTaskListsByDomain failed")).
					Times(1)
			},
			expectedError: "Operation GetTaskListByDomain failed",
			domainFlag:    "test-domain",
			taskListFlag:  "test-tasklist",
			taskListType:  "decision",
		},
		{
			name:          "NoDomainFlag",
			setupMocks:    func(client *frontend.MockClient) {},
			expectedError: "Required flag not found",
			domainFlag:    "", // Omit Domain flag
			taskListFlag:  "test-tasklist",
			taskListType:  "decision",
		},
	}

	// Loop through test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)

			// Set up mocks for the current test case
			tt.setupMocks(td.mockFrontendClient)

			var cliCtx *cli.Context
			if tt.domainFlag == "" {
				cliCtx = clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagTaskList, testTaskList),
					/* omit the domain flag */
					clitest.StringArgument(FlagTaskListType, testTaskListType),
				)
			} else {
				// construct cli context with all the required arguments
				cliCtx = newTaskListCLIContext(t, td.app)
			}

			err := AdminListTaskList(cliCtx)
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedError)
			}
		})
	}
}

func TestAdminUpdateTaskListPartitionConfig(t *testing.T) {
	// Define table of test cases
	tests := []struct {
		name               string
		setupMocks         func(*admin.MockClient)
		expectedError      string
		domainFlag         string
		taskListFlag       string
		taskListType       string
		numReadPartitions  int
		numWritePartitions int
	}{
		{
			name: "Success",
			setupMocks: func(client *admin.MockClient) {
				client.EXPECT().
					UpdateTaskListPartitionConfig(gomock.Any(), &types.UpdateTaskListPartitionConfigRequest{
						Domain:       "test-domain",
						TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
						TaskListType: types.TaskListTypeDecision.Ptr(),
						PartitionConfig: &types.TaskListPartitionConfig{
							ReadPartitions:  createPartitions(2),
							WritePartitions: createPartitions(2),
						},
					}).
					Return(&types.UpdateTaskListPartitionConfigResponse{}, nil).
					Times(1)
			},
			expectedError:      "",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name: "UpdateTaskListPartitionConfigFails",
			setupMocks: func(client *admin.MockClient) {
				client.EXPECT().
					UpdateTaskListPartitionConfig(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("API failed")).
					Times(1)
			},
			expectedError:      "Operation UpdateTaskListPartitionConfig failed.: API failed",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name:               "NoDomainFlag",
			setupMocks:         func(client *admin.MockClient) {},
			expectedError:      "Required flag not found",
			domainFlag:         "", // Omit Domain flag
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name:               "NoTaskListFlag",
			setupMocks:         func(client *admin.MockClient) {},
			expectedError:      "Required flag not found",
			domainFlag:         "test-domain",
			taskListType:       "decision",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name:               "Invalid task list type",
			setupMocks:         func(client *admin.MockClient) {},
			expectedError:      "Invalid task list type: valid types are [activity, decision]",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name:               "NoReadPartitionFlag",
			setupMocks:         func(client *admin.MockClient) {},
			expectedError:      "Required flag not found",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numWritePartitions: 2,
		},
		{
			name:              "NoWritePartitionFlag",
			setupMocks:        func(client *admin.MockClient) {},
			expectedError:     "Required flag not found",
			domainFlag:        "test-domain",
			taskListFlag:      "test-tasklist",
			taskListType:      "decision",
			numReadPartitions: 2,
		},
	}

	// Loop through test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)

			// Set up mocks for the current test case
			tt.setupMocks(td.mockAdminClient)

			var cliArgs []clitest.CliArgument
			if tt.domainFlag != "" {
				cliArgs = append(cliArgs, clitest.StringArgument(FlagDomain, tt.domainFlag))
			}
			if tt.taskListFlag != "" {
				cliArgs = append(cliArgs, clitest.StringArgument(FlagTaskList, tt.taskListFlag))
			}
			if tt.taskListType != "" {
				cliArgs = append(cliArgs, clitest.StringArgument(FlagTaskListType, tt.taskListType))
			}
			if tt.numReadPartitions != 0 {
				cliArgs = append(cliArgs, clitest.IntArgument(FlagNumReadPartitions, tt.numReadPartitions))
			}
			if tt.numWritePartitions != 0 {
				cliArgs = append(cliArgs, clitest.IntArgument(FlagNumWritePartitions, tt.numWritePartitions))
			}
			cliCtx := clitest.NewCLIContext(
				t,
				td.app,
				cliArgs...,
			)

			err := AdminUpdateTaskListPartitionConfig(cliCtx)
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedError)
			}
		})
	}
}

// Helper function to set up the CLI context
func newTaskListCLIContext(t *testing.T, app *cli.App) *cli.Context {
	return clitest.NewCLIContext(
		t,
		app,
		clitest.StringArgument(FlagDomain, testDomain),
		clitest.StringArgument(FlagTaskList, testTaskList),
		clitest.StringArgument(FlagTaskListType, testTaskListType),
	)
}
