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
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

func TestAdminDescribeTaskList(t *testing.T) {
	response := &types.DescribeTaskListResponse{
		Pollers: []*types.PollerInfo{
			{
				Identity: "test-poller",
			},
		},
		TaskListStatus: &types.TaskListStatus{
			BacklogCountHint: 10,
		},
	}
	taskList := &types.TaskList{
		Name: testTaskList,
	}

	tests := []struct {
		name        string
		allowance   func(td *cliTestData)
		tlType      string
		format      string
		baseArgs    []clitest.CliArgument
		assertions  func(t *testing.T, td *cliTestData)
		expectedErr string
	}{
		{
			name: "success",
			allowance: func(td *cliTestData) {
				td.mockFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:                testDomain,
					TaskList:              taskList,
					TaskListType:          types.TaskListTypeActivity.Ptr(),
					IncludeTaskListStatus: true,
				}).Return(response, nil).Times(1)
				td.mockFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:                testDomain,
					TaskList:              taskList,
					TaskListType:          types.TaskListTypeDecision.Ptr(),
					IncludeTaskListStatus: true,
				}).Return(response, nil).Times(1)
			},
		},
		{
			name:   "success - decision only",
			tlType: "decision",
			allowance: func(td *cliTestData) {
				td.mockFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:                testDomain,
					TaskList:              taskList,
					TaskListType:          types.TaskListTypeDecision.Ptr(),
					IncludeTaskListStatus: true,
				}).Return(response, nil).Times(1)
			},
		},
		{
			name:   "success - activity only",
			tlType: "activity",
			allowance: func(td *cliTestData) {
				td.mockFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:                testDomain,
					TaskList:              taskList,
					TaskListType:          types.TaskListTypeActivity.Ptr(),
					IncludeTaskListStatus: true,
				}).Return(response, nil).Times(1)
			},
		},
		{
			name:   "json",
			tlType: "decision",
			format: "json",
			allowance: func(td *cliTestData) {
				td.mockFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:                testDomain,
					TaskList:              taskList,
					TaskListType:          types.TaskListTypeDecision.Ptr(),
					IncludeTaskListStatus: true,
				}).Return(response, nil).Times(1)
			},
			assertions: func(t *testing.T, td *cliTestData) {
				expected := map[types.TaskListType]*types.DescribeTaskListResponse{
					types.TaskListTypeDecision: response,
				}
				output := td.consoleOutput()
				var result map[types.TaskListType]*types.DescribeTaskListResponse
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)
				assert.Equal(t, expected, result)
			},
		},
		{
			name:        "error - missing domain",
			expectedErr: "Required flag not found",
			baseArgs: []clitest.CliArgument{
				clitest.StringArgument(FlagTaskList, testTaskList),
			},
		},
		{
			name:        "error - missing tasklist",
			expectedErr: "Required flag not found",
			baseArgs: []clitest.CliArgument{
				clitest.StringArgument(FlagDomain, testDomain),
			},
		},

		{
			name:   "error - from server",
			tlType: "decision",
			allowance: func(td *cliTestData) {
				td.mockFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:                testDomain,
					TaskList:              taskList,
					TaskListType:          types.TaskListTypeDecision.Ptr(),
					IncludeTaskListStatus: true,
				}).Return(nil, errors.New("oh no")).Times(1)
			},
			expectedErr: "oh no",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			td := newCLITestData(t)

			var args []clitest.CliArgument
			if tc.baseArgs == nil {
				args = []clitest.CliArgument{
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagTaskList, testTaskList),
				}
			} else {
				args = tc.baseArgs
			}
			if tc.tlType != "" {
				args = append(args, clitest.StringArgument(FlagTaskListType, tc.tlType))
			}
			if tc.format != "" {
				args = append(args, clitest.StringArgument(FlagFormat, tc.format))
			}

			cliCtx := clitest.NewCLIContext(
				t,
				td.app,
				args...,
			)

			if tc.allowance != nil {
				tc.allowance(td)
			}

			err := AdminDescribeTaskList(cliCtx)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectedErr)
			}

			if tc.assertions != nil {
				tc.assertions(t, td)
			}
		})
	}
}

func TestAdminListTaskList(t *testing.T) {
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
	// Define table of test cases
	tests := []struct {
		name          string
		setupMocks    func(*frontend.MockClient)
		assertions    func(t *testing.T, td *cliTestData)
		expectedError string
		domainFlag    string
		format        string
	}{
		{
			name: "Success",
			setupMocks: func(client *frontend.MockClient) {
				client.EXPECT().
					GetTaskListsByDomain(gomock.Any(), gomock.Any()).
					Return(expectedResponse, nil).
					Times(1)
			},
			domainFlag: "test-domain",
		},
		{
			name:   "Success - json",
			format: "json",
			setupMocks: func(client *frontend.MockClient) {
				client.EXPECT().
					GetTaskListsByDomain(gomock.Any(), gomock.Any()).
					Return(expectedResponse, nil).
					Times(1)
			},
			assertions: func(t *testing.T, td *cliTestData) {
				output := td.consoleOutput()
				var result *types.GetTaskListsByDomainResponse
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)
				assert.Equal(t, expectedResponse, result)
			},
			domainFlag: "test-domain",
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
		},
		{
			name:          "NoDomainFlag",
			setupMocks:    func(client *frontend.MockClient) {},
			expectedError: "Required flag not found",
			domainFlag:    "", // Omit Domain flag
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
					/* omit the domain flag */
					clitest.StringArgument(FlagFormat, tt.format),
				)
			} else {
				cliCtx = clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagFormat, tt.format),
				)
			}

			err := AdminListTaskList(cliCtx)
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedError)
			}

			if tt.assertions != nil {
				tt.assertions(t, td)
			}
		})
	}
}

func TestAdminUpdateTaskListPartitionConfig(t *testing.T) {
	// Define table of test cases
	tests := []struct {
		name               string
		setupMocks         func(*admin.MockClient, *frontend.MockClient)
		expectedError      string
		domainFlag         string
		taskListFlag       string
		taskListType       string
		numReadPartitions  int
		numWritePartitions int
		force              bool
	}{
		{
			name: "Success",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers: []*types.PollerInfo{
						{
							Identity: "poller",
						},
					},
					PartitionConfig: nil,
				}, nil).Times(1)
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
			name: "Success - both types",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers: []*types.PollerInfo{
						{
							Identity: "poller",
						},
					},
					PartitionConfig: nil,
				}, nil).Times(1)
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeActivity.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers: []*types.PollerInfo{
						{
							Identity: "poller",
						},
					},
					PartitionConfig: nil,
				}, nil).Times(1)
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
				client.EXPECT().
					UpdateTaskListPartitionConfig(gomock.Any(), &types.UpdateTaskListPartitionConfigRequest{
						Domain:       "test-domain",
						TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
						TaskListType: types.TaskListTypeActivity.Ptr(),
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
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name: "Success - force",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
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
			force:              true,
		},
		{
			name: "UpdateTaskListPartitionConfigFails",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers: []*types.PollerInfo{
						{
							Identity: "poller",
						},
					},
					PartitionConfig: nil,
				}, nil).Times(1)
				client.EXPECT().
					UpdateTaskListPartitionConfig(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("API failed")).
					Times(1)
			},
			expectedError:      "API failed",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name:               "NoDomainFlag",
			expectedError:      "Required flag not found",
			domainFlag:         "", // Omit Domain flag
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name:               "NoTaskListFlag",
			expectedError:      "Required flag not found",
			domainFlag:         "test-domain",
			taskListType:       "decision",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name:               "Invalid task list type",
			expectedError:      "Invalid task list type: valid types are 'activity', 'decision', or empty (both)",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			taskListType:       "ihsdajhi",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name:               "NoReadPartitionFlag",
			expectedError:      "Required flag not found",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numWritePartitions: 2,
		},
		{
			name:              "NoWritePartitionFlag",
			expectedError:     "Required flag not found",
			domainFlag:        "test-domain",
			taskListFlag:      "test-tasklist",
			taskListType:      "decision",
			numReadPartitions: 2,
		},
		{
			name: "safe - removing drained partition",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
				// Check the root for pollers and config safety
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers: []*types.PollerInfo{
						{
							Identity: "poller",
						},
					},
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         1,
						ReadPartitions:  createPartitions(3),
						WritePartitions: createPartitions(1),
					},
				}, nil).Times(1)
				// Check the partitions being drained
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:                "test-domain",
					TaskList:              &types.TaskList{Name: getPartitionTaskListName("test-tasklist", 2), Kind: types.TaskListKindNormal.Ptr()},
					TaskListType:          types.TaskListTypeDecision.Ptr(),
					IncludeTaskListStatus: true,
				}).Return(&types.DescribeTaskListResponse{
					Pollers: nil,
					TaskListStatus: &types.TaskListStatus{
						BacklogCountHint: 0,
					},
					PartitionConfig: nil,
				}, nil).Times(1)
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:                "test-domain",
					TaskList:              &types.TaskList{Name: getPartitionTaskListName("test-tasklist", 1), Kind: types.TaskListKindNormal.Ptr()},
					TaskListType:          types.TaskListTypeDecision.Ptr(),
					IncludeTaskListStatus: true,
				}).Return(&types.DescribeTaskListResponse{
					Pollers: nil,
					TaskListStatus: &types.TaskListStatus{
						BacklogCountHint: 0,
					},
					PartitionConfig: nil,
				}, nil).Times(1)
				// do the update
				client.EXPECT().
					UpdateTaskListPartitionConfig(gomock.Any(), &types.UpdateTaskListPartitionConfigRequest{
						Domain:       "test-domain",
						TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
						TaskListType: types.TaskListTypeDecision.Ptr(),
						PartitionConfig: &types.TaskListPartitionConfig{
							ReadPartitions:  createPartitions(1),
							WritePartitions: createPartitions(1),
						},
					}).
					Return(&types.UpdateTaskListPartitionConfigResponse{}, nil).
					Times(1)
			},
			expectedError:      "",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numReadPartitions:  1,
			numWritePartitions: 1,
		},
		{
			name: "Safe - only one type missing pollers",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers: []*types.PollerInfo{
						{
							Identity: "poller",
						},
					},
					PartitionConfig: nil,
				}, nil).Times(1)
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeActivity.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers:         nil,
					PartitionConfig: nil,
				}, nil).Times(1)
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
				client.EXPECT().
					UpdateTaskListPartitionConfig(gomock.Any(), &types.UpdateTaskListPartitionConfigRequest{
						Domain:       "test-domain",
						TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
						TaskListType: types.TaskListTypeActivity.Ptr(),
						PartitionConfig: &types.TaskListPartitionConfig{
							ReadPartitions:  createPartitions(2),
							WritePartitions: createPartitions(2),
						},
					}).
					Return(&types.UpdateTaskListPartitionConfigResponse{}, nil).
					Times(1)
			},
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name: "Unsafe - no pollers",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers:         nil,
					PartitionConfig: nil,
				}, nil).Times(1)
			},
			expectedError:      "no pollers",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name: "Unsafe - both types no pollers",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers:         nil,
					PartitionConfig: nil,
				}, nil).Times(1)
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeActivity.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers:         nil,
					PartitionConfig: nil,
				}, nil).Times(1)
			},
			expectedError:      "no pollers",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name: "Unsafe - removing active write partition",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers: []*types.PollerInfo{
						{
							Identity: "poller",
						},
					},
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         1,
						ReadPartitions:  createPartitions(3),
						WritePartitions: createPartitions(3),
					},
				}, nil).Times(1)
			},
			expectedError:      "remove write partitions, then read partitions",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name: "Unsafe - removing non-drained partition",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers: []*types.PollerInfo{
						{
							Identity: "poller",
						},
					},
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         1,
						ReadPartitions:  createPartitions(3),
						WritePartitions: createPartitions(2),
					},
				}, nil).Times(1)
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:                "test-domain",
					TaskList:              &types.TaskList{Name: getPartitionTaskListName("test-tasklist", 2), Kind: types.TaskListKindNormal.Ptr()},
					TaskListType:          types.TaskListTypeDecision.Ptr(),
					IncludeTaskListStatus: true,
				}).Return(&types.DescribeTaskListResponse{
					Pollers: nil,
					TaskListStatus: &types.TaskListStatus{
						BacklogCountHint: 1,
					},
					PartitionConfig: nil,
				}, nil)
			},
			expectedError:      "partition 2 still has 1 tasks remaining",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			taskListType:       "decision",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
		{
			name: "Unsafe - one type fails validation",
			setupMocks: func(client *admin.MockClient, f *frontend.MockClient) {
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeDecision.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers: []*types.PollerInfo{
						{
							Identity: "poller",
						},
					},
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         1,
						ReadPartitions:  createPartitions(3),
						WritePartitions: createPartitions(3),
					},
				}, nil).Times(1)
				f.EXPECT().DescribeTaskList(gomock.Any(), &types.DescribeTaskListRequest{
					Domain:       "test-domain",
					TaskList:     &types.TaskList{Name: "test-tasklist", Kind: types.TaskListKindNormal.Ptr()},
					TaskListType: types.TaskListTypeActivity.Ptr(),
				}).Return(&types.DescribeTaskListResponse{
					Pollers: []*types.PollerInfo{
						{
							Identity: "poller",
						},
					},
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         1,
						ReadPartitions:  createPartitions(2),
						WritePartitions: createPartitions(2),
					},
				}, nil).Times(1)
			},
			expectedError:      "test-tasklist:Decision failed validation:",
			domainFlag:         "test-domain",
			taskListFlag:       "test-tasklist",
			numReadPartitions:  2,
			numWritePartitions: 2,
		},
	}

	// Loop through test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)

			if tt.setupMocks != nil {
				tt.setupMocks(td.mockAdminClient, td.mockFrontendClient)
			}

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
			if tt.force {
				cliArgs = append(cliArgs, clitest.BoolArgument(FlagForce, true))
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
