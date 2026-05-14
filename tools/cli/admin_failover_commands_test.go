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

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
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
			desc:                    "when valid params it should start the failover workflow",
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
					Domain:                              constants.SystemLocalDomainName,
					RequestID:                           "test-uuid",
					WorkflowID:                          failovermanager.FailoverWorkflowID,
					WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
					TaskList:                            &types.TaskList{Name: failovermanager.TaskListName},
					Input:                               []byte(`{"TargetCluster":"cluster2","SourceCluster":"cluster1","BatchFailoverSize":10,"BatchFailoverWaitTimeInSeconds":120,"Domains":["domain1","domain2"],"DrillWaitTime":0,"GracefulFailoverTimeoutInSeconds":300}`),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(600), // == failoverWFTimeout
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(defaultDecisionTimeoutInSeconds),
					Memo: mustGetWorkflowMemo(t, map[string]interface{}{
						constants.MemoKeyForOperator: "test-user",
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
			desc:          "when StartWorkflowExecution fails it should return error",
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
			desc:          "when source and target cluster are the same it should return error",
			wantErr:       true,
			sourceCluster: "cluster1",
			targetCluster: "cluster1",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// no frontend calls due to validation failure
			},
		},
		{
			desc:          "when no source cluster specified it should return error",
			wantErr:       true,
			sourceCluster: "",
			targetCluster: "cluster2",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// no frontend calls due to validation failure
			},
		},
		{
			desc:          "when no target cluster specified it should return error",
			wantErr:       true,
			sourceCluster: "cluster1",
			targetCluster: "",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// no frontend calls due to validation failure
			},
		},
		{
			desc:                    "when cron schedule specified it should start the drill workflow",
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
					Domain:                              constants.SystemLocalDomainName,
					RequestID:                           "test-uuid",
					CronSchedule:                        "0 0 * * *",
					WorkflowID:                          failovermanager.DrillWorkflowID,
					WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
					TaskList:                            &types.TaskList{Name: failovermanager.TaskListName},
					Input:                               []byte(`{"TargetCluster":"cluster2","SourceCluster":"cluster1","BatchFailoverSize":10,"BatchFailoverWaitTimeInSeconds":120,"Domains":["domain1","domain2"],"DrillWaitTime":30000000000,"GracefulFailoverTimeoutInSeconds":300}`),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(600), // == failoverWFTimeout
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(defaultDecisionTimeoutInSeconds),
					Memo: mustGetWorkflowMemo(t, map[string]interface{}{
						constants.MemoKeyForOperator: "test-user",
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
		useDrill      bool
		mockFn        func(*testing.T, *frontend.MockClient)
		wantErr       bool
	}{
		{
			desc:          "when pause requested it should signal PauseSignal to FailoverWorkflowID",
			pauseOrResume: "pause",
			runID:         "runid1",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.SignalWorkflowExecutionRequest, opts ...yarpc.CallOption) error {
						wantReq := &types.SignalWorkflowExecutionRequest{
							Domain: constants.SystemLocalDomainName,
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
			desc:          "when pause and SignalWorkflowExecution fails it should return error",
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
			desc:          "when resume requested it should signal ResumeSignal to FailoverWorkflowID",
			pauseOrResume: "resume",
			runID:         "runid1",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				wantReq := &types.SignalWorkflowExecutionRequest{
					Domain: constants.SystemLocalDomainName,
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
			desc:          "when resume and SignalWorkflowExecution fails it should return error",
			pauseOrResume: "resume",
			wantErr:       true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, r *types.SignalWorkflowExecutionRequest, opts ...yarpc.CallOption) error {
						return fmt.Errorf("failed to signal workflow")
					}).Times(1)
			},
		},
		{
			desc:          "when pause and drill flag set it should signal DrillWorkflowID",
			pauseOrResume: "pause",
			useDrill:      true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.SignalWorkflowExecutionRequest, opts ...yarpc.CallOption) error {
						if gotReq.WorkflowExecution.WorkflowID != failovermanager.DrillWorkflowID {
							t.Fatalf("expected DrillWorkflowID, got %s", gotReq.WorkflowExecution.WorkflowID)
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

			args := []string{"", "admin", "cluster", "failover", tc.pauseOrResume,
				"--rid", tc.runID,
			}
			if tc.useDrill {
				args = append(args, "--failover_drill")
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
			desc: "when workflow is terminated it should return aborted state",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.QueryWorkflowRequest, opts ...yarpc.CallOption) (*types.QueryWorkflowResponse, error) {
						wantReq := &types.QueryWorkflowRequest{
							Domain: constants.SystemLocalDomainName,
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
							Domain: constants.SystemLocalDomainName,
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
			desc:    "when QueryWorkflow fails it should return error",
			wantErr: true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.QueryWorkflowRequest, opts ...yarpc.CallOption) (*types.QueryWorkflowResponse, error) {
						return nil, fmt.Errorf("failed to query workflow")
					}).Times(1)
			},
		},
		{
			desc:    "when DescribeWorkflowExecution fails it should return error",
			wantErr: true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.QueryWorkflowRequest, opts ...yarpc.CallOption) (*types.QueryWorkflowResponse, error) {
						wantReq := &types.QueryWorkflowRequest{
							Domain: constants.SystemLocalDomainName,
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
		{
			desc:    "when QueryResult is nil it should return error",
			wantErr: true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(&types.QueryWorkflowResponse{QueryResult: nil}, nil).Times(1)
			},
		},
		{
			desc:    "when QueryResult is invalid JSON it should return error",
			wantErr: true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(&types.QueryWorkflowResponse{QueryResult: []byte("not-valid-json")}, nil).Times(1)
			},
		},
		{
			desc: "when workflow is not terminated it should return state unchanged",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(&types.QueryWorkflowResponse{
						QueryResult: mustMarshalQueryResult(t, queryResult),
					}, nil).Times(1)
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
					}, nil).Times(1)
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
			desc: "when called it should terminate FailoverWorkflowID",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.TerminateWorkflowExecutionRequest, opts ...yarpc.CallOption) error {
						wantReq := &types.TerminateWorkflowExecutionRequest{
							Domain: constants.SystemLocalDomainName,
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
			desc: "when workflow is running it should terminate and start failback",
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				// query to check if it's running.
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, gotReq *types.QueryWorkflowRequest, opts ...yarpc.CallOption) (*types.QueryWorkflowResponse, error) {
						wantReq := &types.QueryWorkflowRequest{
							Domain: constants.SystemLocalDomainName,
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
							Domain: constants.SystemLocalDomainName,
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
							Domain: constants.SystemLocalDomainName,
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
							Domain:                constants.SystemLocalDomainName,
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
								constants.MemoKeyForOperator: "test-user",
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
		{
			desc:    "when first QueryWorkflow fails it should return error",
			wantErr: true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("query failed")).Times(1)
			},
		},
		{
			desc:    "when workflow running and TerminateWorkflowExecution fails it should return error",
			wantErr: true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(&types.QueryWorkflowResponse{
						QueryResult: mustMarshalQueryResult(t, failovermanager.QueryResult{
							State: failovermanager.WorkflowRunning,
						}),
					}, nil).Times(1)
				m.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("terminate failed")).Times(1)
			},
		},
		{
			desc:    "when workflow not running and second QueryWorkflow fails it should return error",
			wantErr: true,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(&types.QueryWorkflowResponse{
						QueryResult: mustMarshalQueryResult(t, failovermanager.QueryResult{
							State: failovermanager.WorkflowCompleted,
						}),
					}, nil).Times(1)
				m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("second query failed")).Times(1)
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

func TestAdminFailoverList(t *testing.T) {
	tests := []struct {
		desc     string
		useDrill bool
		wantWFID string
		mockFn   func(*testing.T, *frontend.MockClient)
		wantErr  bool
	}{
		{
			desc:     "when drill flag not set it should list FailoverWorkflowID executions",
			wantWFID: failovermanager.FailoverWorkflowID,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 0}, nil).Times(1)
				m.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, req *types.ListClosedWorkflowExecutionsRequest, opts ...yarpc.CallOption) (*types.ListClosedWorkflowExecutionsResponse, error) {
						if req.ExecutionFilter == nil || req.ExecutionFilter.WorkflowID != failovermanager.FailoverWorkflowID {
							t.Fatalf("expected WorkflowID %s, got %v", failovermanager.FailoverWorkflowID, req.ExecutionFilter)
						}
						return &types.ListClosedWorkflowExecutionsResponse{}, nil
					}).Times(1)
			},
		},
		{
			desc:     "when drill flag set it should list DrillWorkflowID executions",
			useDrill: true,
			wantWFID: failovermanager.DrillWorkflowID,
			mockFn: func(t *testing.T, m *frontend.MockClient) {
				m.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 0}, nil).Times(1)
				m.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, req *types.ListClosedWorkflowExecutionsRequest, opts ...yarpc.CallOption) (*types.ListClosedWorkflowExecutionsResponse, error) {
						if req.ExecutionFilter == nil || req.ExecutionFilter.WorkflowID != failovermanager.DrillWorkflowID {
							t.Fatalf("expected WorkflowID %s, got %v", failovermanager.DrillWorkflowID, req.ExecutionFilter)
						}
						return &types.ListClosedWorkflowExecutionsResponse{}, nil
					}).Times(1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			frontendCl := frontend.NewMockClient(ctrl)
			tc.mockFn(t, frontendCl)

			app := NewCliApp(&clientFactoryMock{
				serverFrontendClient: frontendCl,
			})

			args := []string{"", "admin", "cluster", "failover", "list"}
			if tc.useDrill {
				args = append(args, "--failover_drill")
			}
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
