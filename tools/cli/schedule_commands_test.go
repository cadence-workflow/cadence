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

package cli

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
)

func newScheduleTestApp(t *testing.T, mockClient *frontend.MockClient) *cli.App {
	t.Helper()
	return NewCliApp(&clientFactoryMock{
		serverFrontendClient: mockClient,
	})
}

func newScheduleCLIContext(app *cli.App, flags map[string]string) *cli.Context {
	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "domain")
	set.String(FlagTransport, grpcTransport, "transport")
	set.Int(FlagPageSize, 10, "page size")
	set.Int(FlagExecutionTimeout, 3600, "execution timeout")
	set.Int(FlagDecisionTimeout, 10, "decision timeout")
	for k, v := range flags {
		set.String(k, v, k)
	}
	return cli.NewContext(app, set, nil)
}

func TestScheduleCLI_CreateSchedule(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)
	app := newScheduleTestApp(t, mockClient)

	mockClient.EXPECT().CreateSchedule(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.CreateScheduleRequest, _ ...interface{}) (*types.CreateScheduleResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			assert.Equal(t, "my-sched", req.ScheduleID)
			assert.Equal(t, "*/5 * * * *", req.Spec.CronExpression)
			assert.Equal(t, "my-wf", req.Action.StartWorkflow.WorkflowType.Name)
			return &types.CreateScheduleResponse{ScheduleID: "my-sched"}, nil
		})

	c := newScheduleCLIContext(app, map[string]string{
		FlagScheduleID:     "my-sched",
		FlagCronExpression: "*/5 * * * *",
		FlagWorkflowType:   "my-wf",
		FlagTaskList:       "my-tl",
	})
	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.CreateSchedule(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_DescribeSchedule(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)
	app := newScheduleTestApp(t, mockClient)

	mockClient.EXPECT().DescribeSchedule(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.DescribeScheduleRequest, _ ...interface{}) (*types.DescribeScheduleResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			assert.Equal(t, "my-sched", req.ScheduleID)
			return &types.DescribeScheduleResponse{
				Spec:     &types.ScheduleSpec{CronExpression: "*/5 * * * *"},
				Action:   &types.ScheduleAction{StartWorkflow: &types.StartWorkflowAction{WorkflowType: &types.WorkflowType{Name: "my-wf"}}},
				Policies: &types.SchedulePolicies{},
				State:    &types.ScheduleState{Paused: false},
				Info:     &types.ScheduleInfo{TotalRuns: 10},
			}, nil
		})

	c := newScheduleCLIContext(app, map[string]string{
		FlagScheduleID: "my-sched",
	})
	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.DescribeSchedule(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_PauseSchedule(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)

	mockClient.EXPECT().PauseSchedule(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.PauseScheduleRequest, _ ...interface{}) (*types.PauseScheduleResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			assert.Equal(t, "my-sched", req.ScheduleID)
			assert.Equal(t, "maint", req.Reason)
			return &types.PauseScheduleResponse{}, nil
		})

	app := newScheduleTestApp(t, mockClient)
	c := newScheduleCLIContext(app, map[string]string{
		FlagScheduleID: "my-sched",
		FlagReason:     "maint",
	})
	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.PauseSchedule(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_UnpauseSchedule(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)

	mockClient.EXPECT().UnpauseSchedule(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.UnpauseScheduleRequest, _ ...interface{}) (*types.UnpauseScheduleResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			assert.Equal(t, "my-sched", req.ScheduleID)
			return &types.UnpauseScheduleResponse{}, nil
		})

	app := newScheduleTestApp(t, mockClient)
	c := newScheduleCLIContext(app, map[string]string{
		FlagScheduleID: "my-sched",
	})
	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.UnpauseSchedule(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_DeleteSchedule(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)

	mockClient.EXPECT().DeleteSchedule(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.DeleteScheduleRequest, _ ...interface{}) (*types.DeleteScheduleResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			assert.Equal(t, "my-sched", req.ScheduleID)
			return &types.DeleteScheduleResponse{}, nil
		})

	app := newScheduleTestApp(t, mockClient)
	c := newScheduleCLIContext(app, map[string]string{
		FlagScheduleID: "my-sched",
	})
	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.DeleteSchedule(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_UpdateSchedule(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)

	mockClient.EXPECT().UpdateSchedule(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.UpdateScheduleRequest, _ ...interface{}) (*types.UpdateScheduleResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			assert.Equal(t, "my-sched", req.ScheduleID)
			assert.Equal(t, "0 * * * *", req.Spec.CronExpression)
			return &types.UpdateScheduleResponse{}, nil
		})

	app := newScheduleTestApp(t, mockClient)
	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "", "domain")
	set.String(FlagTransport, "", "transport")
	set.String(FlagScheduleID, "", "schedule_id")
	set.String(FlagCronExpression, "", "cron")
	set.Parse([]string{
		"--" + FlagDomain, "test-domain",
		"--" + FlagTransport, grpcTransport,
		"--" + FlagScheduleID, "my-sched",
		"--" + FlagCronExpression, "0 * * * *",
	})
	c := cli.NewContext(app, set, nil)

	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.UpdateSchedule(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_ParseOverlapPolicy(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected types.ScheduleOverlapPolicy
		wantErr  bool
	}{
		"skip_new":           {input: "SkipNew", expected: types.ScheduleOverlapPolicySkipNew},
		"buffer":             {input: "buffer", expected: types.ScheduleOverlapPolicyBuffer},
		"concurrent":         {input: "Concurrent", expected: types.ScheduleOverlapPolicyConcurrent},
		"cancel_previous":    {input: "cancel_previous", expected: types.ScheduleOverlapPolicyCancelPrevious},
		"terminate_previous": {input: "TerminatePrevious", expected: types.ScheduleOverlapPolicyTerminatePrevious},
		"invalid":            {input: "invalid", wantErr: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseOverlapPolicy(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestScheduleCLI_ParseCatchUpPolicy(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected types.ScheduleCatchUpPolicy
		wantErr  bool
	}{
		"skip":    {input: "skip", expected: types.ScheduleCatchUpPolicySkip},
		"one":     {input: "One", expected: types.ScheduleCatchUpPolicyOne},
		"all":     {input: "ALL", expected: types.ScheduleCatchUpPolicyAll},
		"invalid": {input: "invalid", wantErr: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseCatchUpPolicy(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestScheduleCLI_BackfillSchedule(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)

	mockClient.EXPECT().BackfillSchedule(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.BackfillScheduleRequest, _ ...interface{}) (*types.BackfillScheduleResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			assert.Equal(t, "my-sched", req.ScheduleID)
			assert.Equal(t, "bf-1", req.BackfillID)
			assert.False(t, req.StartTime.IsZero())
			assert.False(t, req.EndTime.IsZero())
			assert.True(t, req.StartTime.Before(req.EndTime))
			return &types.BackfillScheduleResponse{}, nil
		})

	app := newScheduleTestApp(t, mockClient)
	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "", "")
	set.String(FlagTransport, "", "")
	set.String(FlagScheduleID, "", "")
	set.String(FlagStartTime, "", "")
	set.String(FlagEndTime, "", "")
	set.String(FlagBackfillID, "", "")
	set.Parse([]string{
		"--" + FlagDomain, "test-domain",
		"--" + FlagTransport, grpcTransport,
		"--" + FlagScheduleID, "my-sched",
		"--" + FlagStartTime, "2024-01-01T00:00:00Z",
		"--" + FlagEndTime, "2024-01-02T00:00:00Z",
		"--" + FlagBackfillID, "bf-1",
	})
	c := cli.NewContext(app, set, nil)

	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.BackfillSchedule(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_BackfillSchedule_EndBeforeStart(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)
	app := newScheduleTestApp(t, mockClient)

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "", "")
	set.String(FlagTransport, "", "")
	set.String(FlagScheduleID, "", "")
	set.String(FlagStartTime, "", "")
	set.String(FlagEndTime, "", "")
	set.Parse([]string{
		"--" + FlagDomain, "test-domain",
		"--" + FlagTransport, grpcTransport,
		"--" + FlagScheduleID, "my-sched",
		"--" + FlagStartTime, "2024-01-02T00:00:00Z",
		"--" + FlagEndTime, "2024-01-01T00:00:00Z",
	})
	c := cli.NewContext(app, set, nil)

	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.BackfillSchedule(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start_time must be before end_time")
}

func TestScheduleCLI_BackfillSchedule_InvalidTimeFormat(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)
	app := newScheduleTestApp(t, mockClient)

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "", "")
	set.String(FlagTransport, "", "")
	set.String(FlagScheduleID, "", "")
	set.String(FlagStartTime, "", "")
	set.String(FlagEndTime, "", "")
	set.Parse([]string{
		"--" + FlagDomain, "test-domain",
		"--" + FlagTransport, grpcTransport,
		"--" + FlagScheduleID, "my-sched",
		"--" + FlagStartTime, "not-a-date",
		"--" + FlagEndTime, "2024-01-02T00:00:00Z",
	})
	c := cli.NewContext(app, set, nil)

	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.BackfillSchedule(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid start_time format")
}

func TestScheduleCLI_ListSchedules(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)

	mockClient.EXPECT().ListSchedules(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.ListSchedulesRequest, _ ...interface{}) (*types.ListSchedulesResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			return &types.ListSchedulesResponse{
				Schedules: []*types.ScheduleListEntry{
					{
						ScheduleID:     "sched-1",
						WorkflowType:   &types.WorkflowType{Name: "wf-1"},
						CronExpression: "*/5 * * * *",
						State:          &types.ScheduleState{Paused: false},
					},
				},
			}, nil
		})

	app := newScheduleTestApp(t, mockClient)
	c := newScheduleCLIContext(app, map[string]string{})

	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.ListSchedules(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_CreateMissingDomain(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)
	app := newScheduleTestApp(t, mockClient)

	set := flag.NewFlagSet("test", 0)
	set.String(FlagScheduleID, "my-sched", "schedule_id")
	set.String(FlagCronExpression, "*/5 * * * *", "cron")
	set.String(FlagWorkflowType, "my-wf", "wf type")
	c := cli.NewContext(app, set, nil)

	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.CreateSchedule(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "domain")
}
