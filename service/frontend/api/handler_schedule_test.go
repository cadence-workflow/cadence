// Copyright (c) 2026 Uber Technologies, Inc.
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

package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/client"
	dc "github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	frontendcfg "github.com/uber/cadence/service/frontend/config"
	"github.com/uber/cadence/service/frontend/validate"
	"github.com/uber/cadence/service/worker/scheduler"
)

type scheduleTestFixture struct {
	t              *testing.T
	ctrl           *gomock.Controller
	mockResource   *resource.Test
	domainCache    *cache.MockDomainCache
	historyClient  *history.MockClient
	versionChecker *client.MockVersionChecker
	handler        *WorkflowHandler
}

func newScheduleTestFixture(t *testing.T) *scheduleTestFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockResource := resource.NewTest(t, ctrl, metrics.Frontend)

	versionChecker := client.NewMockVersionChecker(ctrl)
	versionChecker.EXPECT().ClientSupported(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	versionChecker.EXPECT().SupportsWorkflowAlreadyCompletedError(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockResource.MembershipResolver.EXPECT().MemberCount(service.Frontend).Return(5, nil).AnyTimes()

	config := frontendcfg.NewConfig(
		dc.NewCollection(dc.NewInMemoryClient(), mockResource.Logger),
		10, false, "hostname", mockResource.Logger,
	)
	config.EmitSignalNameMetricsTag = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)

	handler := NewWorkflowHandler(mockResource, config, versionChecker, nil)

	return &scheduleTestFixture{
		t:              t,
		ctrl:           ctrl,
		mockResource:   mockResource,
		domainCache:    mockResource.DomainCache,
		historyClient:  mockResource.HistoryClient,
		versionChecker: versionChecker,
		handler:        handler,
	}
}

func (f *scheduleTestFixture) finish() {
	f.ctrl.Finish()
	f.mockResource.Finish(f.t)
}

func TestCreateSchedule(t *testing.T) {
	validRequest := &types.CreateScheduleRequest{
		Domain:     testDomain,
		ScheduleID: "my-schedule",
		Spec:       &types.ScheduleSpec{CronExpression: "*/5 * * * *"},
		Action: &types.ScheduleAction{
			StartWorkflow: &types.StartWorkflowAction{
				WorkflowType: &types.WorkflowType{Name: "my-workflow"},
				TaskList:     &types.TaskList{Name: "my-tasklist"},
				Input:        []byte(`{"k":1}`),
			},
		},
	}

	tests := map[string]struct {
		request     *types.CreateScheduleRequest
		mockFn      func(*scheduleTestFixture)
		wantErr     bool
		wantErrType interface{}
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty domain": {
			request: &types.CreateScheduleRequest{},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty schedule ID": {
			request: &types.CreateScheduleRequest{Domain: testDomain},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"nil spec": {
			request: &types.CreateScheduleRequest{Domain: testDomain, ScheduleID: "s1"},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty cron expression": {
			request: &types.CreateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				Spec:       &types.ScheduleSpec{},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"invalid cron expression": {
			request: &types.CreateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				Spec:       &types.ScheduleSpec{CronExpression: "not-a-cron"},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"impossible cron date (Feb 30) parses but never fires": {
			request: &types.CreateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				Spec:       &types.ScheduleSpec{CronExpression: "0 0 30 2 *"},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"nil action": {
			request: &types.CreateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				Spec:       &types.ScheduleSpec{CronExpression: "* * * * *"},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"invalid SKIP_NEW + CATCH_UP_ALL": {
			request: &types.CreateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				Spec:       &types.ScheduleSpec{CronExpression: "* * * * *"},
				Action: &types.ScheduleAction{
					StartWorkflow: &types.StartWorkflowAction{
						WorkflowType: &types.WorkflowType{Name: "wf"},
						TaskList:     &types.TaskList{Name: "tl"},
					},
				},
				Policies: &types.SchedulePolicies{
					OverlapPolicy: types.ScheduleOverlapPolicySkipNew,
					CatchUpPolicy: types.ScheduleCatchUpPolicyAll,
				},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"domain not found": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return("", errors.New("not found")).AnyTimes()
			},
			wantErr: true,
		},
		"already exists": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, &types.WorkflowExecutionAlreadyStartedError{Message: "already started"})
			},
			wantErr: true,
		},
		"user search attributes use reserved key": {
			request: &types.CreateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "my-schedule",
				Spec:       &types.ScheduleSpec{CronExpression: "*/5 * * * *"},
				Action: &types.ScheduleAction{
					StartWorkflow: &types.StartWorkflowAction{
						WorkflowType: &types.WorkflowType{Name: "my-workflow"},
						TaskList:     &types.TaskList{Name: "my-tasklist"},
					},
				},
				SearchAttributes: &types.SearchAttributes{IndexedFields: map[string][]byte{
					"CadenceScheduleState": []byte(`"paused"`),
				}},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"history error": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("internal error"))
			},
			wantErr: true,
		},
		"BUFFER with buffer_limit above system limit succeeds (warns)": {
			request: &types.CreateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "my-schedule",
				Spec:       &types.ScheduleSpec{CronExpression: "*/5 * * * *"},
				Action: &types.ScheduleAction{
					StartWorkflow: &types.StartWorkflowAction{
						WorkflowType: &types.WorkflowType{Name: "my-workflow"},
						TaskList:     &types.TaskList{Name: "my-tasklist"},
					},
				},
				Policies: &types.SchedulePolicies{
					OverlapPolicy: types.ScheduleOverlapPolicyBuffer,
					BufferLimit:   int32(scheduler.MaxBufferedFiresSystemLimit * 2),
				},
			},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.StartWorkflowExecutionResponse{RunID: "test-run-id"}, nil)
			},
			wantErr: false,
		},
		"success": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistoryStartWorkflowExecutionRequest, _ ...yarpc.CallOption) (*types.StartWorkflowExecutionResponse, error) {
						assert.Equal(t, testDomainID, req.DomainUUID)
						assert.Equal(t, "cadence-scheduler:my-schedule", req.StartRequest.WorkflowID)
						assert.Equal(t, scheduler.WorkflowTypeName, req.StartRequest.WorkflowType.Name)
						assert.Equal(t, scheduler.TaskListName, req.StartRequest.TaskList.Name)

						var input scheduler.SchedulerWorkflowInput
						require.NoError(t, json.Unmarshal(req.StartRequest.Input, &input))
						assert.Equal(t, testDomain, input.Domain)
						assert.Equal(t, "my-schedule", input.ScheduleID)
						assert.Equal(t, "*/5 * * * *", input.Spec.CronExpression)
						require.NotNil(t, input.Action.StartWorkflow)
						assert.Equal(t, []byte(`{"k":1}`), input.Action.StartWorkflow.Input)

						return &types.StartWorkflowExecutionResponse{RunID: "test-run-id"}, nil
					})
			},
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.CreateSchedule(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestDescribeSchedule(t *testing.T) {
	descResult := scheduler.ScheduleDescription{
		ScheduleID:  "my-schedule",
		Domain:      testDomain,
		Spec:        types.ScheduleSpec{CronExpression: "*/10 * * * *"},
		Action:      types.ScheduleAction{StartWorkflow: &types.StartWorkflowAction{WorkflowType: &types.WorkflowType{Name: "wf"}}},
		Policies:    types.SchedulePolicies{OverlapPolicy: types.ScheduleOverlapPolicySkipNew},
		Paused:      true,
		PauseReason: "maintenance",
		PausedBy:    "admin",
		TotalRuns:   42,
	}
	descBytes, _ := json.Marshal(descResult)

	validRequest := &types.DescribeScheduleRequest{
		Domain:     testDomain,
		ScheduleID: "my-schedule",
	}

	tests := map[string]struct {
		request  *types.DescribeScheduleRequest
		mockFn   func(*scheduleTestFixture)
		wantErr  bool
		checkErr func(*testing.T, error)
		check    func(*testing.T, *types.DescribeScheduleResponse)
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty domain": {
			request: &types.DescribeScheduleRequest{},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty schedule ID": {
			request: &types.DescribeScheduleRequest{Domain: testDomain},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"domain not found": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return("", errors.New("not found")).AnyTimes().AnyTimes()
			},
			wantErr: true,
		},
		"query error": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("query failed"))
			},
			wantErr: true,
		},
		"schedule not found": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(nil, &types.EntityNotExistsError{Message: "workflow not found"})
			},
			wantErr: true,
		},
		// Regression: a scheduler workflow that ended FAILED (e.g. invalid cron) must not
		// surface as ACTIVE. The handler sets QueryRejectConditionNotCompletedCleanly so
		// Cadence rejects the query for any non-clean close status.
		"scheduler workflow failed - query rejected": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				closeStatus := types.WorkflowExecutionCloseStatusFailed
				f.historyClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(&types.HistoryQueryWorkflowResponse{
						Response: &types.QueryWorkflowResponse{
							QueryRejected: &types.QueryRejected{CloseStatus: &closeStatus},
						},
					}, nil)
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				var internalErr *types.InternalServiceError
				assert.ErrorAs(t, err, &internalErr)
				assert.Contains(t, internalErr.Message, "FAILED")
			},
		},
		"scheduler workflow terminated - query rejected": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				closeStatus := types.WorkflowExecutionCloseStatusTerminated
				f.historyClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					Return(&types.HistoryQueryWorkflowResponse{
						Response: &types.QueryWorkflowResponse{
							QueryRejected: &types.QueryRejected{CloseStatus: &closeStatus},
						},
					}, nil)
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				var internalErr *types.InternalServiceError
				assert.ErrorAs(t, err, &internalErr)
				assert.Contains(t, internalErr.Message, "TERMINATED")
			},
		},
		"success": {
			request: validRequest,
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistoryQueryWorkflowRequest, _ ...yarpc.CallOption) (*types.HistoryQueryWorkflowResponse, error) {
						assert.Equal(t, testDomainID, req.DomainUUID)
						assert.Equal(t, "cadence-scheduler:my-schedule", req.Request.Execution.WorkflowID)
						assert.Equal(t, scheduler.QueryTypeDescribe, req.Request.Query.QueryType)
						// Verify the reject condition is set so closed/failed workflows are detected.
						require.NotNil(t, req.Request.QueryRejectCondition)
						assert.Equal(t, types.QueryRejectConditionNotCompletedCleanly, *req.Request.QueryRejectCondition)

						return &types.HistoryQueryWorkflowResponse{
							Response: &types.QueryWorkflowResponse{
								QueryResult: descBytes,
							},
						}, nil
					})
			},
			wantErr: false,
			check: func(t *testing.T, resp *types.DescribeScheduleResponse) {
				assert.Equal(t, "*/10 * * * *", resp.Spec.CronExpression)
				assert.True(t, resp.State.Paused)
				assert.Equal(t, "maintenance", resp.State.PauseInfo.Reason)
				assert.Equal(t, "admin", resp.State.PauseInfo.PausedBy)
				assert.Equal(t, int64(42), resp.Info.TotalRuns)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.DescribeSchedule(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
				if tt.check != nil {
					tt.check(t, resp)
				}
			}
		})
	}
}

func TestPauseSchedule(t *testing.T) {
	tests := map[string]struct {
		request *types.PauseScheduleRequest
		mockFn  func(*scheduleTestFixture)
		wantErr bool
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty domain": {
			request: &types.PauseScheduleRequest{},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty schedule ID": {
			request: &types.PauseScheduleRequest{Domain: testDomain},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"signal error": {
			request: &types.PauseScheduleRequest{Domain: testDomain, ScheduleID: "s1", Reason: "maintenance"},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(errors.New("signal failed"))
			},
			wantErr: true,
		},
		"success": {
			request: &types.PauseScheduleRequest{Domain: testDomain, ScheduleID: "s1", Reason: "maintenance"},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistorySignalWorkflowExecutionRequest, _ ...yarpc.CallOption) error {
						assert.Equal(t, testDomainID, req.DomainUUID)
						assert.Equal(t, "cadence-scheduler:s1", req.SignalRequest.WorkflowExecution.WorkflowID)
						assert.Equal(t, scheduler.SignalNamePause, req.SignalRequest.SignalName)

						var signal scheduler.PauseSignal
						require.NoError(t, json.Unmarshal(req.SignalRequest.Input, &signal))
						assert.Equal(t, "maintenance", signal.Reason)
						return nil
					})
			},
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.PauseSchedule(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestUnpauseSchedule(t *testing.T) {
	tests := map[string]struct {
		request *types.UnpauseScheduleRequest
		mockFn  func(*scheduleTestFixture)
		wantErr bool
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"success": {
			request: &types.UnpauseScheduleRequest{
				Domain:        testDomain,
				ScheduleID:    "s1",
				CatchUpPolicy: types.ScheduleCatchUpPolicyOne,
			},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistorySignalWorkflowExecutionRequest, _ ...yarpc.CallOption) error {
						assert.Equal(t, scheduler.SignalNameUnpause, req.SignalRequest.SignalName)
						var signal scheduler.UnpauseSignal
						require.NoError(t, json.Unmarshal(req.SignalRequest.Input, &signal))
						assert.Equal(t, types.ScheduleCatchUpPolicyOne, signal.CatchUpPolicy)
						return nil
					})
			},
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.UnpauseSchedule(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestUpdateSchedule(t *testing.T) {
	tests := map[string]struct {
		request *types.UpdateScheduleRequest
		mockFn  func(*scheduleTestFixture)
		wantErr bool
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"nothing to update": {
			request: &types.UpdateScheduleRequest{Domain: testDomain, ScheduleID: "s1"},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"invalid SKIP_NEW + CATCH_UP_ALL": {
			request: &types.UpdateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				Policies: &types.SchedulePolicies{
					OverlapPolicy: types.ScheduleOverlapPolicySkipNew,
					CatchUpPolicy: types.ScheduleCatchUpPolicyAll,
				},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"success with spec update": {
			request: &types.UpdateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				Spec:       &types.ScheduleSpec{CronExpression: "0 * * * *"},
			},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistorySignalWorkflowExecutionRequest, _ ...yarpc.CallOption) error {
						assert.Equal(t, scheduler.SignalNameUpdate, req.SignalRequest.SignalName)
						var signal scheduler.UpdateSignal
						require.NoError(t, json.Unmarshal(req.SignalRequest.Input, &signal))
						assert.Equal(t, "0 * * * *", signal.Spec.CronExpression)
						assert.Nil(t, signal.Action)
						assert.Nil(t, signal.Policies)
						return nil
					})
			},
			wantErr: false,
		},
		"invalid cron expression in spec update": {
			request: &types.UpdateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				Spec:       &types.ScheduleSpec{CronExpression: "not-a-cron"},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"impossible cron date in spec update (Feb 30) parses but never fires": {
			request: &types.UpdateScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				Spec:       &types.ScheduleSpec{CronExpression: "0 0 30 2 *"},
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.UpdateSchedule(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestDeleteSchedule(t *testing.T) {
	tests := map[string]struct {
		request *types.DeleteScheduleRequest
		mockFn  func(*scheduleTestFixture)
		wantErr bool
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty domain": {
			request: &types.DeleteScheduleRequest{ScheduleID: "s1"},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"empty schedule ID": {
			request: &types.DeleteScheduleRequest{Domain: testDomain},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"success: signal delivered": {
			request: &types.DeleteScheduleRequest{Domain: testDomain, ScheduleID: "s1"},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistorySignalWorkflowExecutionRequest, _ ...yarpc.CallOption) error {
						assert.Equal(t, scheduler.SignalNameDelete, req.SignalRequest.SignalName)
						assert.Nil(t, req.SignalRequest.Input)
						return nil
					})
			},
			wantErr: false,
		},
		"idempotent: scheduler workflow not found is treated as already deleted": {
			request: &types.DeleteScheduleRequest{Domain: testDomain, ScheduleID: "s1"},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.EntityNotExistsError{Message: "workflow cadence-scheduler:s1 not found"})
			},
			wantErr: false,
		},
		"idempotent: scheduler workflow already closed is treated as already deleted": {
			request: &types.DeleteScheduleRequest{Domain: testDomain, ScheduleID: "s1"},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.WorkflowExecutionAlreadyCompletedError{Message: "workflow execution already completed"})
			},
			wantErr: false,
		},
		"non-idempotent error from delete signal passes through unchanged": {
			request: &types.DeleteScheduleRequest{Domain: testDomain, ScheduleID: "s1"},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(errors.New("some internal error"))
			},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.DeleteSchedule(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestBackfillSchedule(t *testing.T) {
	now := time.Now()
	tests := map[string]struct {
		request *types.BackfillScheduleRequest
		mockFn  func(*scheduleTestFixture)
		wantErr bool
	}{
		"nil request": {
			request: nil,
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"missing start time": {
			request: &types.BackfillScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				EndTime:    now,
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"missing end time": {
			request: &types.BackfillScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				StartTime:  now.Add(-time.Hour),
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"end before start": {
			request: &types.BackfillScheduleRequest{
				Domain:     testDomain,
				ScheduleID: "s1",
				StartTime:  now,
				EndTime:    now.Add(-time.Hour),
			},
			mockFn:  func(f *scheduleTestFixture) {},
			wantErr: true,
		},
		"success": {
			request: &types.BackfillScheduleRequest{
				Domain:        testDomain,
				ScheduleID:    "s1",
				StartTime:     now.Add(-time.Hour),
				EndTime:       now,
				OverlapPolicy: types.ScheduleOverlapPolicyConcurrent,
				BackfillID:    "bf-1",
			},
			mockFn: func(f *scheduleTestFixture) {
				f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
				f.historyClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.HistorySignalWorkflowExecutionRequest, _ ...yarpc.CallOption) error {
						assert.Equal(t, scheduler.SignalNameBackfill, req.SignalRequest.SignalName)
						var signal scheduler.BackfillSignal
						require.NoError(t, json.Unmarshal(req.SignalRequest.Input, &signal))
						assert.Equal(t, types.ScheduleOverlapPolicyConcurrent, signal.OverlapPolicy)
						assert.Equal(t, "bf-1", signal.BackfillID)
						return nil
					})
			},
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newScheduleTestFixture(t)
			defer f.finish()
			tt.mockFn(f)

			resp, err := f.handler.BackfillSchedule(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestListSchedules(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		f := newScheduleTestFixture(t)
		defer f.finish()

		resp, err := f.handler.ListSchedules(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("empty domain", func(t *testing.T) {
		f := newScheduleTestFixture(t)
		defer f.finish()

		resp, err := f.handler.ListSchedules(context.Background(), &types.ListSchedulesRequest{})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("success with results", func(t *testing.T) {
		f := newScheduleTestFixture(t)
		defer f.finish()

		f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()

		pausedStateBytes, _ := json.Marshal(scheduler.ScheduleStatePaused)
		cronBytes, _ := json.Marshal("0 6 * * *")
		typeBytes, _ := json.Marshal("my-target-workflow")

		f.mockResource.VisibilityMgr.On("ListOpenWorkflowExecutionsByType", mock.Anything, mock.MatchedBy(func(req *persistence.ListWorkflowExecutionsByTypeRequest) bool {
			return req.Domain == testDomain && req.WorkflowTypeName == scheduler.WorkflowTypeName
		})).Return(&persistence.ListWorkflowExecutionsResponse{
			Executions: []*types.WorkflowExecutionInfo{
				{
					Execution: &types.WorkflowExecution{
						WorkflowID: "cadence-scheduler:sched-1",
						RunID:      "run-1",
					},
					Type: &types.WorkflowType{Name: scheduler.WorkflowTypeName},
					SearchAttributes: &types.SearchAttributes{IndexedFields: map[string][]byte{
						scheduler.SearchAttrScheduleState:        pausedStateBytes,
						scheduler.SearchAttrScheduleCron:         cronBytes,
						scheduler.SearchAttrScheduleWorkflowType: typeBytes,
					}},
				},
				{
					Execution: &types.WorkflowExecution{
						WorkflowID: "cadence-scheduler:sched-2",
						RunID:      "run-2",
					},
					Type: &types.WorkflowType{Name: scheduler.WorkflowTypeName},
				},
			},
			NextPageToken: []byte("next"),
		}, nil).Once()

		resp, err := f.handler.ListSchedules(context.Background(), &types.ListSchedulesRequest{
			Domain:   testDomain,
			PageSize: 10,
		})
		assert.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Schedules, 2)

		assert.Equal(t, "sched-1", resp.Schedules[0].ScheduleID)
		require.NotNil(t, resp.Schedules[0].State)
		assert.True(t, resp.Schedules[0].State.Paused)
		assert.Equal(t, "0 6 * * *", resp.Schedules[0].CronExpression)
		require.NotNil(t, resp.Schedules[0].WorkflowType)
		assert.Equal(t, "my-target-workflow", resp.Schedules[0].WorkflowType.Name)

		assert.Equal(t, "sched-2", resp.Schedules[1].ScheduleID)
		require.NotNil(t, resp.Schedules[1].State)
		assert.False(t, resp.Schedules[1].State.Paused, "missing search attribute should default to not paused")
		assert.Empty(t, resp.Schedules[1].CronExpression, "missing cron search attribute should yield empty string")
		assert.Nil(t, resp.Schedules[1].WorkflowType, "missing workflow type search attribute should yield nil")

		assert.Equal(t, []byte("next"), resp.NextPageToken)
	})

	t.Run("query path when list visibility filter disabled", func(t *testing.T) {
		f := newScheduleTestFixture(t)
		defer f.finish()

		f.handler.config.DisableListVisibilityByFilter = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)

		f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()

		f.mockResource.VisibilityMgr.On("ListWorkflowExecutions", mock.Anything, mock.MatchedBy(func(req *persistence.ListWorkflowExecutionsByQueryRequest) bool {
			return req.Domain == testDomain && req.Query == "WorkflowType = 'cadence-scheduler' and CloseTime = missing"
		})).Return(&persistence.ListWorkflowExecutionsResponse{
			Executions: []*types.WorkflowExecutionInfo{
				{
					Execution: &types.WorkflowExecution{
						WorkflowID: "cadence-scheduler:sched-q",
						RunID:      "run-q",
					},
					Type: &types.WorkflowType{Name: scheduler.WorkflowTypeName},
				},
			},
			NextPageToken: []byte("tok-q"),
		}, nil).Once()

		resp, err := f.handler.ListSchedules(context.Background(), &types.ListSchedulesRequest{
			Domain:   testDomain,
			PageSize: 5,
		})
		assert.NoError(t, err)
		require.NotNil(t, resp)
		require.Len(t, resp.Schedules, 1)
		assert.Equal(t, "sched-q", resp.Schedules[0].ScheduleID)
		assert.Equal(t, []byte("tok-q"), resp.NextPageToken)
	})

	t.Run("skips visibility rows without schedule workflow id prefix", func(t *testing.T) {
		f := newScheduleTestFixture(t)
		defer f.finish()

		f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()

		f.mockResource.VisibilityMgr.On("ListOpenWorkflowExecutionsByType", mock.Anything, mock.Anything).
			Return(&persistence.ListWorkflowExecutionsResponse{
				Executions: []*types.WorkflowExecutionInfo{
					{
						Execution: &types.WorkflowExecution{WorkflowID: "not-scheduler-wf", RunID: "r1"},
						Type:      &types.WorkflowType{Name: scheduler.WorkflowTypeName},
					},
					{
						Execution: &types.WorkflowExecution{WorkflowID: "cadence-scheduler:good", RunID: "r2"},
						Type:      &types.WorkflowType{Name: scheduler.WorkflowTypeName},
					},
				},
			}, nil).Once()

		resp, err := f.handler.ListSchedules(context.Background(), &types.ListSchedulesRequest{Domain: testDomain})
		require.NoError(t, err)
		require.Len(t, resp.Schedules, 1)
		assert.Equal(t, "good", resp.Schedules[0].ScheduleID)
	})
}

func TestNormalizeScheduleError(t *testing.T) {
	t.Run("describe not found returns friendly message", func(t *testing.T) {
		f := newScheduleTestFixture(t)
		defer f.finish()

		f.domainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil).AnyTimes()
		f.historyClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).
			Return(nil, &types.EntityNotExistsError{Message: "workflow cadence-scheduler:s1 not found"})

		_, err := f.handler.DescribeSchedule(context.Background(), &types.DescribeScheduleRequest{
			Domain:     testDomain,
			ScheduleID: "s1",
		})

		var notFound *types.EntityNotExistsError
		require.True(t, errors.As(err, &notFound))
		assert.Contains(t, notFound.Message, `schedule "s1" not found in domain`)
	})

}

func TestScheduleWorkflowID(t *testing.T) {
	assert.Equal(t, "cadence-scheduler:my-schedule", scheduleWorkflowID("my-schedule"))
	assert.Equal(t, "cadence-scheduler:", scheduleWorkflowID(""))
}

func TestValidateSchedulePolicies(t *testing.T) {
	tests := map[string]struct {
		policies *types.SchedulePolicies
		wantErr  bool
	}{
		"nil policies": {
			policies: nil,
			wantErr:  false,
		},
		"valid SKIP_NEW + SKIP": {
			policies: &types.SchedulePolicies{
				OverlapPolicy: types.ScheduleOverlapPolicySkipNew,
				CatchUpPolicy: types.ScheduleCatchUpPolicySkip,
			},
			wantErr: false,
		},
		"valid SKIP_NEW + ONE": {
			policies: &types.SchedulePolicies{
				OverlapPolicy: types.ScheduleOverlapPolicySkipNew,
				CatchUpPolicy: types.ScheduleCatchUpPolicyOne,
			},
			wantErr: false,
		},
		"invalid SKIP_NEW + ALL": {
			policies: &types.SchedulePolicies{
				OverlapPolicy: types.ScheduleOverlapPolicySkipNew,
				CatchUpPolicy: types.ScheduleCatchUpPolicyAll,
			},
			wantErr: true,
		},
		"valid TERMINATE_PREVIOUS + ALL": {
			policies: &types.SchedulePolicies{
				OverlapPolicy: types.ScheduleOverlapPolicyTerminatePrevious,
				CatchUpPolicy: types.ScheduleCatchUpPolicyAll,
			},
			wantErr: false,
		},
		"valid CONCURRENT + ALL": {
			policies: &types.SchedulePolicies{
				OverlapPolicy: types.ScheduleOverlapPolicyConcurrent,
				CatchUpPolicy: types.ScheduleCatchUpPolicyAll,
			},
			wantErr: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateSchedulePolicies(tt.policies)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateSchedule_ShuttingDown(t *testing.T) {
	f := newScheduleTestFixture(t)
	defer f.finish()
	f.handler.shuttingDown = 1

	_, err := f.handler.CreateSchedule(context.Background(), &types.CreateScheduleRequest{})
	assert.Equal(t, validate.ErrShuttingDown, err)
}
