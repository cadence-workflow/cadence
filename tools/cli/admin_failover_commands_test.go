// Copyright (c) 2017-2020 Uber Technologies Inc.
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
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/failovermanager"
)

func TestAdminFailoverStart(t *testing.T) {
	oldUUIDFn := uuidFn
	uuidFn = func() string { return "test-uuid" }
	oldGetOperatorFn := getOperatorFn
	getOperatorFn = func() (string, error) { return "test-user", nil }
	defer func() {
		uuidFn = oldUUIDFn
		getOperatorFn = oldGetOperatorFn
	}()

	tests := []struct {
		desc                    string
		sourceCluster           string
		targetCluster           string
		failoverBatchSize       int
		failoverWaitTime        int
		gracefulFailoverTimeout int
		failoverWFTimeout       int
		failoverDomains         []string
		failoverDrillWaitTime   int
		failoverCron            string
		runID                   string
		mockFn                  func(*testing.T, *frontend.MockClient)
		wantErr                 bool
	}{
		{
			desc:                    "success",
			sourceCluster:           "cluster1",
			targetCluster:           "cluster2",
			failoverBatchSize:       10,
			failoverWaitTime:        120,
			gracefulFailoverTimeout: 300,
			failoverWFTimeout:       600,
			failoverDomains:         []string{"domain1", "domain2"},
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// first drill workflow will be signalled to pause in case it is running.
				m.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)

				// then failover workflow will be started
				wantReq := &types.StartWorkflowExecutionRequest{
					Domain:                              common.SystemLocalDomainName,
					RequestID:                           "test-uuid",
					WorkflowID:                          failovermanager.FailoverWorkflowID,
					WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
					TaskList:                            &types.TaskList{Name: failovermanager.TaskListName},
					Input:                               []byte(`{"TargetCluster":"cluster2","SourceCluster":"cluster1","BatchFailoverSize":10,"BatchFailoverWaitTimeInSeconds":120,"Domains":["domain1","domain2"],"DrillWaitTime":0,"GracefulFailoverTimeoutInSeconds":300}`),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(600), // == failoverWFTimeout
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(defaultDecisionTimeoutInSeconds),
					Memo: mustGetWorkflowMemo(t, map[string]interface{}{
						common.MemoKeyForOperator: "test-user",
					}),
					WorkflowType: &types.WorkflowType{Name: failovermanager.FailoverWorkflowTypeName},
				}
				resp := &types.StartWorkflowExecutionResponse{}
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.StartWorkflowExecutionRequest, opts ...yarpc.CallOption) (*types.StartWorkflowExecutionResponse, error) {
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return resp, nil
					}).Times(1)
			},
		},
		{
			desc:          "startworkflow fails",
			wantErr:       true,
			sourceCluster: "cluster1",
			targetCluster: "cluster2",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// first drill workflow will be signalled to pause in case it is running.
				m.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)

				// then failover workflow will be started
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.StartWorkflowExecutionRequest, opts ...yarpc.CallOption) (*types.StartWorkflowExecutionResponse, error) {
						return nil, fmt.Errorf("failed to start workflow")
					}).Times(1)
			},
		},
		{
			desc:          "source and target cluster same",
			wantErr:       true,
			sourceCluster: "cluster1",
			targetCluster: "cluster1",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// no frontend calls due to validation failure
			},
		},
		{
			desc:          "no source cluster specified",
			wantErr:       true,
			sourceCluster: "",
			targetCluster: "cluster2",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// no frontend calls due to validation failure
			},
		},
		{
			desc:          "no target cluster specified",
			wantErr:       true,
			sourceCluster: "cluster1",
			targetCluster: "",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// no frontend calls due to validation failure
			},
		},
		{
			desc:                    "success with cron",
			sourceCluster:           "cluster1",
			targetCluster:           "cluster2",
			failoverBatchSize:       10,
			failoverWaitTime:        120,
			gracefulFailoverTimeout: 300,
			failoverWFTimeout:       600,
			failoverDrillWaitTime:   30,
			failoverCron:            "0 0 * * *",
			failoverDomains:         []string{"domain1", "domain2"},
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// failover drill workflow will be started
				wantReq := &types.StartWorkflowExecutionRequest{
					Domain:                              common.SystemLocalDomainName,
					RequestID:                           "test-uuid",
					CronSchedule:                        "0 0 * * *",
					WorkflowID:                          failovermanager.DrillWorkflowID,
					WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
					TaskList:                            &types.TaskList{Name: failovermanager.TaskListName},
					Input:                               []byte(`{"TargetCluster":"cluster2","SourceCluster":"cluster1","BatchFailoverSize":10,"BatchFailoverWaitTimeInSeconds":120,"Domains":["domain1","domain2"],"DrillWaitTime":30000000000,"GracefulFailoverTimeoutInSeconds":300}`),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(600), // == failoverWFTimeout
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(defaultDecisionTimeoutInSeconds),
					Memo: mustGetWorkflowMemo(t, map[string]interface{}{
						common.MemoKeyForOperator: "test-user",
					}),
					WorkflowType: &types.WorkflowType{Name: failovermanager.FailoverWorkflowTypeName},
				}
				resp := &types.StartWorkflowExecutionResponse{}
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.StartWorkflowExecutionRequest, opts ...yarpc.CallOption) (*types.StartWorkflowExecutionResponse, error) {
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return resp, nil
					}).Times(1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			frontendCl := frontend.NewMockClient(ctrl)

			// Set up mocks for the current test case
			tc.mockFn(t, frontendCl)

			// Create mock app with clientFactoryMock, including any deps errors
			app := NewCliApp(&clientFactoryMock{
				serverFrontendClient: frontendCl,
			})

			args := []string{"", "admin", "cluster", "failover", "start",
				"--sc", tc.sourceCluster,
				"--tc", tc.targetCluster,
				"--failover_batch_size", strconv.Itoa(tc.failoverBatchSize),
				"--failover_wait_time_second", strconv.Itoa(tc.failoverWaitTime),
				"--failover_timeout_seconds", strconv.Itoa(tc.gracefulFailoverTimeout),
				"--execution_timeout", strconv.Itoa(tc.failoverWFTimeout),
				"--domains", strings.Join(tc.failoverDomains, ","),
				"--failover_drill_wait_second", strconv.Itoa(tc.failoverDrillWaitTime),
				"--cron", tc.failoverCron,
			}
			err := app.Run(args)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error: %v, wantErr?: %v", err, tc.wantErr)
			}
		})
	}
}

func TestAdminFailoverPauseResume(t *testing.T) {
	tests := []struct {
		desc          string
		runID         string
		pauseOrResume string
		mockFn        func(*testing.T, *frontend.MockClient)
		wantErr       bool
	}{
		{
			desc:          "pause success",
			pauseOrResume: "pause",
			runID:         "runid1",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.SignalWorkflowExecutionRequest, opts ...yarpc.CallOption) error {
						wantReq := &types.SignalWorkflowExecutionRequest{
							Domain: common.SystemLocalDomainName,
							WorkflowExecution: &types.WorkflowExecution{
								WorkflowID: failovermanager.FailoverWorkflowID,
								RunID:      "runid1",
							},
							SignalName: failovermanager.PauseSignal,
							Identity:   getCliIdentity(),
						}
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return nil
					}).Times(1)
			},
		},
		{
			desc:          "pause signal workflow fails",
			pauseOrResume: "pause",
			wantErr:       true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, r *types.SignalWorkflowExecutionRequest, opts ...yarpc.CallOption) error {
						return fmt.Errorf("failed to signal workflow")
					}).Times(1)
			},
		},
		{
			desc:          "resume success",
			pauseOrResume: "resume",
			runID:         "runid1",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				wantReq := &types.SignalWorkflowExecutionRequest{
					Domain: common.SystemLocalDomainName,
					WorkflowExecution: &types.WorkflowExecution{
						WorkflowID: failovermanager.FailoverWorkflowID,
						RunID:      "runid1",
					},
					SignalName: failovermanager.ResumeSignal,
					Identity:   getCliIdentity(),
				}
				m.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.SignalWorkflowExecutionRequest, opts ...yarpc.CallOption) error {
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return nil
					}).Times(1)
			},
		},
		{
			desc:          "resume signal workflow fails",
			pauseOrResume: "resume",
			wantErr:       true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, r *types.SignalWorkflowExecutionRequest, opts ...yarpc.CallOption) error {
						return fmt.Errorf("failed to signal workflow")
					}).Times(1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			frontendCl := frontend.NewMockClient(ctrl)

			// Set up mocks for the current test case
			tc.mockFn(t, frontendCl)

			// Create mock app with clientFactoryMock, including any deps errors
			app := NewCliApp(&clientFactoryMock{
				serverFrontendClient: frontendCl,
			})

			args := []string{"", "admin", "cluster", "failover", tc.pauseOrResume,
				"--rid", tc.runID,
			}
			err := app.Run(args)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error: %v, wantErr?: %v", err, tc.wantErr)
			}
		})
	}
}

func TestAdminFailoverQuery(t *testing.T) {
	queryResult := failovermanager.QueryResult{
		TotalDomains: 10,
		Success:      2,
		Failed:       3,
	}
	tests := []struct {
		desc    string
		mockFn  func(*testing.T, *frontend.MockClient)
		wantErr bool
	}{
		{
			desc: "success",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.QueryWorkflowRequest, opts ...yarpc.CallOption) (*types.QueryWorkflowResponse, error) {
						wantReq := &types.QueryWorkflowRequest{
							Domain: common.SystemLocalDomainName,
							Execution: &types.WorkflowExecution{
								WorkflowID: failovermanager.FailoverWorkflowID,
								RunID:      "",
							},
							Query: &types.WorkflowQuery{
								QueryType: failovermanager.QueryType,
							},
						}
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return &types.QueryWorkflowResponse{
							QueryResult: mustMarshalQueryResult(t, queryResult),
						}, nil
					}).Times(1)

				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.DescribeWorkflowExecutionRequest, opts ...yarpc.CallOption) (*types.DescribeWorkflowExecutionResponse, error) {
						wantReq := &types.DescribeWorkflowExecutionRequest{
							Domain: common.SystemLocalDomainName,
							Execution: &types.WorkflowExecution{
								WorkflowID: failovermanager.FailoverWorkflowID,
								RunID:      "",
							},
						}
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return &types.DescribeWorkflowExecutionResponse{
							WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
								CloseStatus: types.WorkflowExecutionCloseStatusTerminated.Ptr(),
							},
						}, nil
					}).Times(1)
			},
		},
		{
			desc:    "query failed",
			wantErr: true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.QueryWorkflowRequest, opts ...yarpc.CallOption) (*types.QueryWorkflowResponse, error) {
						return nil, fmt.Errorf("failed to query workflow")
					}).Times(1)
			},
		},
		{
			desc:    "describe failed",
			wantErr: true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.QueryWorkflowRequest, opts ...yarpc.CallOption) (*types.QueryWorkflowResponse, error) {
						wantReq := &types.QueryWorkflowRequest{
							Domain: common.SystemLocalDomainName,
							Execution: &types.WorkflowExecution{
								WorkflowID: failovermanager.FailoverWorkflowID,
								RunID:      "",
							},
							Query: &types.WorkflowQuery{
								QueryType: failovermanager.QueryType,
							},
						}
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return &types.QueryWorkflowResponse{
							QueryResult: mustMarshalQueryResult(t, queryResult),
						}, nil
					}).Times(1)

				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.DescribeWorkflowExecutionRequest, opts ...yarpc.CallOption) (*types.DescribeWorkflowExecutionResponse, error) {
						return nil, fmt.Errorf("failed to describe workflow")
					}).Times(1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			frontendCl := frontend.NewMockClient(ctrl)

			// Set up mocks for the current test case
			tc.mockFn(t, frontendCl)

			// Create mock app with clientFactoryMock, including any deps errors
			app := NewCliApp(&clientFactoryMock{
				serverFrontendClient: frontendCl,
			})

			args := []string{"", "admin", "cluster", "failover", "query"}
			err := app.Run(args)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error: %v, wantErr?: %v", err, tc.wantErr)
			}
		})
	}
}

func TestAdminFailoverAbort(t *testing.T) {
	tests := []struct {
		desc    string
		mockFn  func(*testing.T, *frontend.MockClient)
		wantErr bool
	}{
		{
			desc: "success",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.TerminateWorkflowExecutionRequest, opts ...yarpc.CallOption) error {
						wantReq := &types.TerminateWorkflowExecutionRequest{
							Domain: common.SystemLocalDomainName,
							WorkflowExecution: &types.WorkflowExecution{
								WorkflowID: failovermanager.FailoverWorkflowID,
								RunID:      "",
							},
							Reason: "Failover aborted through admin CLI",
						}
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return nil
					}).Times(1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			frontendCl := frontend.NewMockClient(ctrl)

			// Set up mocks for the current test case
			tc.mockFn(t, frontendCl)

			// Create mock app with clientFactoryMock, including any deps errors
			app := NewCliApp(&clientFactoryMock{
				serverFrontendClient: frontendCl,
			})

			args := []string{"", "admin", "cluster", "failover", "abort"}
			err := app.Run(args)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error: %v, wantErr?: %v", err, tc.wantErr)
			}
		})
	}
}

func TestAdminFailoverRollback(t *testing.T) {
	oldUUIDFn := uuidFn
	uuidFn = func() string { return "test-uuid" }
	oldGetOperatorFn := getOperatorFn
	getOperatorFn = func() (string, error) { return "test-user", nil }
	defer func() {
		uuidFn = oldUUIDFn
		getOperatorFn = oldGetOperatorFn
	}()

	tests := []struct {
		desc    string
		mockFn  func(*testing.T, *frontend.MockClient)
		wantErr bool
	}{
		{
			desc: "success",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// query to check if it's running.
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.QueryWorkflowRequest, opts ...yarpc.CallOption) (*types.QueryWorkflowResponse, error) {
						wantReq := &types.QueryWorkflowRequest{
							Domain: common.SystemLocalDomainName,
							Execution: &types.WorkflowExecution{
								WorkflowID: failovermanager.FailoverWorkflowID,
								RunID:      "",
							},
							Query: &types.WorkflowQuery{
								QueryType: failovermanager.QueryType,
							},
						}
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return &types.QueryWorkflowResponse{
							QueryResult: mustMarshalQueryResult(t, failovermanager.QueryResult{
								State: failovermanager.WorkflowRunning,
							}),
						}, nil
					}).Times(1)

				// terminate since it's running
				m.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.TerminateWorkflowExecutionRequest, opts ...yarpc.CallOption) error {
						wantReq := &types.TerminateWorkflowExecutionRequest{
							Domain: common.SystemLocalDomainName,
							WorkflowExecution: &types.WorkflowExecution{
								WorkflowID: failovermanager.FailoverWorkflowID,
								RunID:      "",
							},
							Reason:   "Rollback",
							Identity: getCliIdentity(),
						}
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return nil
					}).Times(1)

				// query again to get domains
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.QueryWorkflowRequest, opts ...yarpc.CallOption) (*types.QueryWorkflowResponse, error) {
						wantReq := &types.QueryWorkflowRequest{
							Domain: common.SystemLocalDomainName,
							Execution: &types.WorkflowExecution{
								WorkflowID: failovermanager.FailoverWorkflowID,
								RunID:      "",
							},
							Query: &types.WorkflowQuery{
								QueryType: failovermanager.QueryType,
							},
						}
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return &types.QueryWorkflowResponse{
							QueryResult: mustMarshalQueryResult(t, failovermanager.QueryResult{
								State:          failovermanager.WorkflowAborted,
								SourceCluster:  "cluster1",
								TargetCluster:  "cluster2",
								SuccessDomains: []string{"domain1", "domain2"},
								FailedDomains:  []string{"domain3"},
							}),
						}, nil
					}).Times(1)

				// failback for success+failed domains
				//
				// first drill workflow will be signalled to pause in case it is running.
				m.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				// then failover workflow will be started to perform failback
				resp := &types.StartWorkflowExecutionResponse{}
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.StartWorkflowExecutionRequest, opts ...yarpc.CallOption) (*types.StartWorkflowExecutionResponse, error) {
						wantReq := &types.StartWorkflowExecutionRequest{
							Domain:                common.SystemLocalDomainName,
							RequestID:             "test-uuid",
							WorkflowID:            failovermanager.FailoverWorkflowID,
							WorkflowIDReusePolicy: types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
							TaskList:              &types.TaskList{Name: failovermanager.TaskListName},
							Input: mustMarshalFailoverParams(t, failovermanager.FailoverParams{
								SourceCluster:                  "cluster2",
								TargetCluster:                  "cluster1",
								Domains:                        []string{"domain1", "domain2", "domain3"},
								BatchFailoverSize:              20, // default value will be used
								BatchFailoverWaitTimeInSeconds: 30, // default value will be used
							}),
							ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1200), // default value will be used
							TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(defaultDecisionTimeoutInSeconds),
							Memo: mustGetWorkflowMemo(t, map[string]interface{}{
								common.MemoKeyForOperator: "test-user",
							}),
							WorkflowType: &types.WorkflowType{Name: failovermanager.FailoverWorkflowTypeName},
						}
						if diff := cmp.Diff(wantReq, gotReq); diff != "" {
							t.Fatalf("Request mismatch (-want +got):\n%s", diff)
						}
						return resp, nil
					}).Times(1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			frontendCl := frontend.NewMockClient(ctrl)

			// Set up mocks for the current test case
			tc.mockFn(t, frontendCl)

			// Create mock app with clientFactoryMock, including any deps errors
			app := NewCliApp(&clientFactoryMock{
				serverFrontendClient: frontendCl,
			})

			args := []string{"", "admin", "cluster", "failover", "rollback"}
			err := app.Run(args)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error: %v, wantErr?: %v", err, tc.wantErr)
			}
		})
	}
}

func mustGetWorkflowMemo(t *testing.T, input map[string]interface{}) *types.Memo {
	memo, err := getWorkflowMemo(input)
	if err != nil {
		t.Fatalf("failed to get workflow memo: %v", err)
	}
	return memo
}

func mustMarshalQueryResult(t *testing.T, queryResult failovermanager.QueryResult) []byte {
	res, err := json.Marshal(queryResult)
	if err != nil {
		t.Fatalf("failed to marshal query result: %v", err)
	}
	return res
}

func mustMarshalFailoverParams(t *testing.T, p failovermanager.FailoverParams) []byte {
	res, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal failover params: %v", err)
	}
	return res
}
