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
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestScheduleCLI_CreateSchedule_ConcurrencyLimit(t *testing.T) {
	makeCtx := func(app *cli.App, extraArgs []string) *cli.Context {
		set := flag.NewFlagSet("test", 0)
		set.String(FlagDomain, "", "")
		set.String(FlagTransport, "", "")
		set.String(FlagScheduleID, "", "")
		set.String(FlagCronExpression, "", "")
		set.String(FlagWorkflowType, "", "")
		set.String(FlagOverlapPolicy, "", "")
		set.Int(FlagConcurrencyLimit, 0, "")
		set.Int(FlagExecutionTimeout, 0, "")
		set.Int(FlagDecisionTimeout, 0, "")
		baseArgs := []string{
			"--" + FlagDomain, "test-domain",
			"--" + FlagTransport, grpcTransport,
			"--" + FlagScheduleID, "my-sched",
			"--" + FlagCronExpression, "*/5 * * * *",
			"--" + FlagWorkflowType, "my-wf",
			"--" + FlagExecutionTimeout, "3600",
			"--" + FlagDecisionTimeout, "10",
		}
		_ = set.Parse(append(baseArgs, extraArgs...))
		return cli.NewContext(app, set, nil)
	}

	tests := []struct {
		name        string
		extraArgs   []string
		setupMock   func(*frontend.MockClient)
		wantErr     bool
		errContains string
	}{
		{
			name:      "concurrent policy with limit succeeds",
			extraArgs: []string{"--" + FlagOverlapPolicy, "concurrent", "--" + FlagConcurrencyLimit, "3"},
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().CreateSchedule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ interface{}, req *types.CreateScheduleRequest, _ ...interface{}) (*types.CreateScheduleResponse, error) {
						assert.Equal(t, types.ScheduleOverlapPolicyConcurrent, req.Policies.OverlapPolicy)
						assert.Equal(t, int32(3), req.Policies.ConcurrencyLimit)
						return &types.CreateScheduleResponse{ScheduleID: "my-sched"}, nil
					})
			},
		},
		{
			name:        "concurrency_limit without overlap_policy returns error",
			extraArgs:   []string{"--" + FlagConcurrencyLimit, "3"},
			wantErr:     true,
			errContains: "--concurrency_limit requires --overlap_policy concurrent",
		},
		{
			name:        "concurrency_limit with non-concurrent policy returns error",
			extraArgs:   []string{"--" + FlagOverlapPolicy, "skipnew", "--" + FlagConcurrencyLimit, "3"},
			wantErr:     true,
			errContains: "--concurrency_limit requires --overlap_policy concurrent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockClient := frontend.NewMockClient(mockCtrl)
			app := newScheduleTestApp(t, mockClient)
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}
			c := makeCtx(app, tt.extraArgs)
			sc := &scheduleCLIImpl{frontendClient: mockClient}
			err := sc.CreateSchedule(c)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
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

func TestPrintDescribeSchedule_OngoingBackfills(t *testing.T) {
	resp := &types.DescribeScheduleResponse{
		Spec:     &types.ScheduleSpec{CronExpression: "0 * * * *"},
		Action:   &types.ScheduleAction{StartWorkflow: &types.StartWorkflowAction{WorkflowType: &types.WorkflowType{Name: "wf"}}},
		Policies: &types.SchedulePolicies{},
		State:    &types.ScheduleState{},
		Info: &types.ScheduleInfo{
			TotalRuns: 4,
			OngoingBackfills: []*types.BackfillInfo{
				{
					BackfillID:    "bf-a",
					StartTime:     time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
					EndTime:       time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
					RunsTotal:     24,
					RunsCompleted: 7,
				},
				nil, // nil entries must be tolerated
				{
					BackfillID:    "bf-b",
					StartTime:     time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
					EndTime:       time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
					RunsTotal:     12,
					RunsCompleted: 12,
				},
			},
		},
	}

	out := captureStdout(t, func() { printDescribeSchedule(resp) })
	assert.Contains(t, out, "Ongoing Backfills:")
	assert.Contains(t, out, "id: bf-a")
	assert.Contains(t, out, "progress: 7/24")
	assert.Contains(t, out, "id: bf-b")
	assert.Contains(t, out, "progress: 12/12")
}

func TestPrintDescribeSchedule_NoOngoingBackfills(t *testing.T) {
	resp := &types.DescribeScheduleResponse{
		Spec:     &types.ScheduleSpec{CronExpression: "0 * * * *"},
		Action:   &types.ScheduleAction{StartWorkflow: &types.StartWorkflowAction{WorkflowType: &types.WorkflowType{Name: "wf"}}},
		Policies: &types.SchedulePolicies{},
		State:    &types.ScheduleState{},
		Info:     &types.ScheduleInfo{TotalRuns: 4},
	}

	out := captureStdout(t, func() { printDescribeSchedule(resp) })
	assert.NotContains(t, out, "Ongoing Backfills")
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// what fn wrote. Used to assert against printDescribeSchedule output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	require.NoError(t, w.Close())

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return strings.TrimRight(buf.String(), "\n")
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

func TestScheduleCLI_UpdateSchedule_ConcurrencyLimit(t *testing.T) {
	makeCtx := func(app *cli.App, extraArgs []string) *cli.Context {
		set := flag.NewFlagSet("test", 0)
		set.String(FlagDomain, "", "")
		set.String(FlagTransport, "", "")
		set.String(FlagScheduleID, "", "")
		set.String(FlagOverlapPolicy, "", "")
		set.Int(FlagConcurrencyLimit, 0, "")
		baseArgs := []string{
			"--" + FlagDomain, "test-domain",
			"--" + FlagTransport, grpcTransport,
			"--" + FlagScheduleID, "my-sched",
		}
		_ = set.Parse(append(baseArgs, extraArgs...))
		return cli.NewContext(app, set, nil)
	}

	tests := []struct {
		name        string
		extraArgs   []string
		setupMock   func(*frontend.MockClient)
		wantErr     bool
		errContains string
	}{
		{
			name:      "only concurrency_limit succeeds",
			extraArgs: []string{"--" + FlagConcurrencyLimit, "3"},
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeSchedule(gomock.Any(), gomock.Any()).
					Return(&types.DescribeScheduleResponse{Policies: &types.SchedulePolicies{}}, nil)
				m.EXPECT().UpdateSchedule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ interface{}, req *types.UpdateScheduleRequest, _ ...interface{}) (*types.UpdateScheduleResponse, error) {
						assert.Nil(t, req.Spec)
						assert.NotNil(t, req.Policies)
						assert.Equal(t, int32(3), req.Policies.ConcurrencyLimit)
						return &types.UpdateScheduleResponse{}, nil
					})
			},
		},
		{
			name:      "concurrency_limit 0 removes the cap",
			extraArgs: []string{"--" + FlagConcurrencyLimit, "0"},
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeSchedule(gomock.Any(), gomock.Any()).
					Return(&types.DescribeScheduleResponse{Policies: &types.SchedulePolicies{ConcurrencyLimit: 7}}, nil)
				m.EXPECT().UpdateSchedule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ interface{}, req *types.UpdateScheduleRequest, _ ...interface{}) (*types.UpdateScheduleResponse, error) {
						assert.NotNil(t, req.Policies)
						assert.Equal(t, int32(0), req.Policies.ConcurrencyLimit)
						return &types.UpdateScheduleResponse{}, nil
					})
			},
		},
		{
			name:      "concurrent policy with limit succeeds",
			extraArgs: []string{"--" + FlagOverlapPolicy, "concurrent", "--" + FlagConcurrencyLimit, "5"},
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeSchedule(gomock.Any(), gomock.Any()).
					Return(&types.DescribeScheduleResponse{Policies: &types.SchedulePolicies{}}, nil)
				m.EXPECT().UpdateSchedule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ interface{}, req *types.UpdateScheduleRequest, _ ...interface{}) (*types.UpdateScheduleResponse, error) {
						assert.Equal(t, types.ScheduleOverlapPolicyConcurrent, req.Policies.OverlapPolicy)
						assert.Equal(t, int32(5), req.Policies.ConcurrencyLimit)
						return &types.UpdateScheduleResponse{}, nil
					})
			},
		},
		{
			name:      "concurrency_limit with non-concurrent overlap policy returns error",
			extraArgs: []string{"--" + FlagOverlapPolicy, "skipnew", "--" + FlagConcurrencyLimit, "3"},
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeSchedule(gomock.Any(), gomock.Any()).
					Return(&types.DescribeScheduleResponse{Policies: &types.SchedulePolicies{}}, nil)
			},
			wantErr:     true,
			errContains: "--concurrency_limit requires --overlap_policy concurrent",
		},
		{
			// Switching overlap policy on a schedule with an inherited ConcurrencyLimit
			// must not falsely reject the update: the inherited limit is harmless when
			// the user did not explicitly pass --concurrency_limit.
			name:      "overlap policy switch with inherited concurrency_limit succeeds",
			extraArgs: []string{"--" + FlagOverlapPolicy, "skipnew"},
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeSchedule(gomock.Any(), gomock.Any()).
					Return(&types.DescribeScheduleResponse{Policies: &types.SchedulePolicies{
						OverlapPolicy:    types.ScheduleOverlapPolicyConcurrent,
						ConcurrencyLimit: 5,
					}}, nil)
				m.EXPECT().UpdateSchedule(gomock.Any(), gomock.Any()).Return(&types.UpdateScheduleResponse{}, nil)
			},
		},
		{
			name:    "no flags returns error",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockClient := frontend.NewMockClient(mockCtrl)
			app := newScheduleTestApp(t, mockClient)
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}
			c := makeCtx(app, tt.extraArgs)
			sc := &scheduleCLIImpl{frontendClient: mockClient}
			err := sc.UpdateSchedule(c)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
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

	mockClient.EXPECT().DescribeSchedule(gomock.Any(), gomock.Any()).
		Return(&types.DescribeScheduleResponse{}, nil)
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

func TestScheduleCLI_BuildPoliciesFromFlags(t *testing.T) {
	app := cli.NewApp()

	makeCtx := func(args []string) *cli.Context {
		set := flag.NewFlagSet("test", 0)
		set.String(FlagOverlapPolicy, "", "")
		set.String(FlagCatchUpPolicy, "", "")
		set.Int(FlagConcurrencyLimit, 0, "")
		_ = set.Parse(args)
		return cli.NewContext(app, set, nil)
	}

	tests := []struct {
		name       string
		args       []string
		wantResult *types.SchedulePolicies
		wantErr    bool
	}{
		{
			name:       "no flags returns nil",
			args:       nil,
			wantResult: nil,
		},
		{
			name:       "only --overlap_policy sets OverlapPolicy in result",
			args:       []string{"--" + FlagOverlapPolicy, "concurrent"},
			wantResult: &types.SchedulePolicies{OverlapPolicy: types.ScheduleOverlapPolicyConcurrent},
		},
		{
			name:       "only --catch_up_policy sets CatchUpPolicy in result",
			args:       []string{"--" + FlagCatchUpPolicy, "skip"},
			wantResult: &types.SchedulePolicies{CatchUpPolicy: types.ScheduleCatchUpPolicySkip},
		},
		{
			name:       "only --concurrency_limit sets ConcurrencyLimit in result",
			args:       []string{"--" + FlagConcurrencyLimit, "3"},
			wantResult: &types.SchedulePolicies{ConcurrencyLimit: 3},
		},
		{
			name: "--overlap_policy concurrent and --concurrency_limit both set in result",
			args: []string{"--" + FlagOverlapPolicy, "concurrent", "--" + FlagConcurrencyLimit, "3"},
			wantResult: &types.SchedulePolicies{
				OverlapPolicy:    types.ScheduleOverlapPolicyConcurrent,
				ConcurrencyLimit: 3,
			},
		},
		{
			name: "--concurrency_limit with non-concurrent overlap policy passes through without error",
			args: []string{"--" + FlagOverlapPolicy, "skipnew", "--" + FlagConcurrencyLimit, "3"},
			wantResult: &types.SchedulePolicies{
				OverlapPolicy:    types.ScheduleOverlapPolicySkipNew,
				ConcurrencyLimit: 3,
			},
		},
		{
			name:    "negative --concurrency_limit returns error",
			args:    []string{"--" + FlagConcurrencyLimit, "-1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := makeCtx(tt.args)
			got, err := buildPoliciesFromFlags(c, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResult, got)
			}
		})
	}
}

func TestScheduleCLI_CreateSchedule_AllFields(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)
	app := newScheduleTestApp(t, mockClient)

	mockClient.EXPECT().CreateSchedule(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.CreateScheduleRequest, _ ...interface{}) (*types.CreateScheduleResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			assert.Equal(t, "sched-full", req.ScheduleID)

			// spec
			assert.Equal(t, "*/5 * * * *", req.Spec.CronExpression)
			assert.Equal(t, "2024-01-01T00:00:00Z", req.Spec.StartTime.UTC().Format(time.RFC3339))
			assert.Equal(t, "2025-01-01T00:00:00Z", req.Spec.EndTime.UTC().Format(time.RFC3339))
			assert.Equal(t, 30*time.Second, req.Spec.Jitter)

			// action
			sw := req.Action.StartWorkflow
			assert.Equal(t, "my-wf", sw.WorkflowType.Name)
			assert.Equal(t, "sched-wf-", sw.WorkflowIDPrefix)
			assert.NotNil(t, sw.RetryPolicy)
			assert.Equal(t, int32(3), sw.RetryPolicy.MaximumAttempts)
			assert.Equal(t, int32(1), sw.RetryPolicy.InitialIntervalInSeconds)
			assert.Equal(t, 2.0, sw.RetryPolicy.BackoffCoefficient)

			// policies
			assert.NotNil(t, req.Policies)
			assert.Equal(t, types.ScheduleOverlapPolicyBuffer, req.Policies.OverlapPolicy)
			assert.Equal(t, types.ScheduleCatchUpPolicyAll, req.Policies.CatchUpPolicy)
			assert.Equal(t, time.Hour, req.Policies.CatchUpWindow)
			assert.True(t, req.Policies.PauseOnFailure)
			assert.Equal(t, int32(10), req.Policies.BufferLimit)

			return &types.CreateScheduleResponse{ScheduleID: "sched-full"}, nil
		})

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "", "")
	set.String(FlagTransport, "", "")
	set.String(FlagScheduleID, "", "")
	set.String(FlagCronExpression, "", "")
	set.String(FlagWorkflowType, "", "")
	set.String(FlagTaskList, "", "")
	set.Int(FlagExecutionTimeout, 0, "")
	set.Int(FlagDecisionTimeout, 0, "")
	set.String(FlagInput, "", "")
	set.String(FlagStartTime, "", "")
	set.String(FlagEndTime, "", "")
	set.String(FlagJitter, "", "")
	set.String(FlagWorkflowIDPrefix, "", "")
	set.Int(FlagRetryAttempts, 0, "")
	set.Int(FlagRetryInterval, 0, "")
	set.Float64(FlagRetryBackoff, 0, "")
	set.String(FlagOverlapPolicy, "", "")
	set.String(FlagCatchUpPolicy, "", "")
	set.String(FlagCatchUpWindow, "", "")
	set.Bool(FlagPauseOnFailure, false, "")
	set.Int(FlagBufferLimit, 0, "")
	set.String(FlagMemoKey, "", "")
	set.String(FlagMemo, "", "")
	set.String(FlagSearchAttributesKey, "", "")
	set.String(FlagSearchAttributesVal, "", "")
	_ = set.Parse([]string{
		"--" + FlagDomain, "test-domain",
		"--" + FlagTransport, grpcTransport,
		"--" + FlagScheduleID, "sched-full",
		"--" + FlagCronExpression, "*/5 * * * *",
		"--" + FlagWorkflowType, "my-wf",
		"--" + FlagExecutionTimeout, "3600",
		"--" + FlagDecisionTimeout, "10",
		"--" + FlagStartTime, "2024-01-01T00:00:00Z",
		"--" + FlagEndTime, "2025-01-01T00:00:00Z",
		"--" + FlagJitter, "30s",
		"--" + FlagWorkflowIDPrefix, "sched-wf-",
		"--" + FlagRetryAttempts, "3",
		"--" + FlagRetryInterval, "1",
		"--" + FlagRetryBackoff, "2.0",
		"--" + FlagOverlapPolicy, "buffer",
		"--" + FlagCatchUpPolicy, "all",
		"--" + FlagCatchUpWindow, "1h",
		"--" + FlagPauseOnFailure,
		"--" + FlagBufferLimit, "10",
	})
	c := cli.NewContext(app, set, nil)
	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.CreateSchedule(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_CreateSchedule_BufferLimitRequiresBufferPolicy(t *testing.T) {
	tests := []struct {
		name        string
		overlap     string
		wantErr     bool
		errContains string
	}{
		{
			name:        "buffer_limit with non-buffer policy errors",
			overlap:     "concurrent",
			wantErr:     true,
			errContains: "--buffer_limit requires --overlap_policy buffer",
		},
		{
			name:    "buffer_limit with buffer policy succeeds",
			overlap: "buffer",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockClient := frontend.NewMockClient(mockCtrl)
			app := newScheduleTestApp(t, mockClient)

			if !tt.wantErr {
				mockClient.EXPECT().CreateSchedule(gomock.Any(), gomock.Any()).
					Return(&types.CreateScheduleResponse{ScheduleID: "s"}, nil)
			}

			set := flag.NewFlagSet("test", 0)
			set.String(FlagDomain, "", "")
			set.String(FlagTransport, "", "")
			set.String(FlagScheduleID, "", "")
			set.String(FlagCronExpression, "", "")
			set.String(FlagWorkflowType, "", "")
			set.Int(FlagExecutionTimeout, 0, "")
			set.Int(FlagDecisionTimeout, 0, "")
			set.String(FlagOverlapPolicy, "", "")
			set.Int(FlagBufferLimit, 0, "")
			_ = set.Parse([]string{
				"--" + FlagDomain, "test-domain",
				"--" + FlagTransport, grpcTransport,
				"--" + FlagScheduleID, "s",
				"--" + FlagCronExpression, "*/5 * * * *",
				"--" + FlagWorkflowType, "wf",
				"--" + FlagExecutionTimeout, "3600",
				"--" + FlagDecisionTimeout, "10",
				"--" + FlagOverlapPolicy, tt.overlap,
				"--" + FlagBufferLimit, "5",
			})
			c := cli.NewContext(app, set, nil)
			sc := &scheduleCLIImpl{frontendClient: mockClient}
			err := sc.CreateSchedule(c)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScheduleCLI_UpdateSchedule_AllFields(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)
	app := newScheduleTestApp(t, mockClient)

	mockClient.EXPECT().DescribeSchedule(gomock.Any(), gomock.Any()).
		Return(&types.DescribeScheduleResponse{}, nil)
	mockClient.EXPECT().UpdateSchedule(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.UpdateScheduleRequest, _ ...interface{}) (*types.UpdateScheduleResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			assert.Equal(t, "sched-full", req.ScheduleID)

			// spec
			assert.NotNil(t, req.Spec)
			assert.Equal(t, "0 * * * *", req.Spec.CronExpression)
			assert.Equal(t, 30*time.Second, req.Spec.Jitter)

			// no action update — action changes require delete+recreate
			assert.Nil(t, req.Action)

			// policies
			assert.NotNil(t, req.Policies)
			assert.Equal(t, time.Hour, req.Policies.CatchUpWindow)
			assert.True(t, req.Policies.PauseOnFailure)

			return &types.UpdateScheduleResponse{}, nil
		})

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "", "")
	set.String(FlagTransport, "", "")
	set.String(FlagScheduleID, "", "")
	set.String(FlagCronExpression, "", "")
	set.String(FlagJitter, "", "")
	set.String(FlagCatchUpWindow, "", "")
	set.Bool(FlagPauseOnFailure, false, "")
	_ = set.Parse([]string{
		"--" + FlagDomain, "test-domain",
		"--" + FlagTransport, grpcTransport,
		"--" + FlagScheduleID, "sched-full",
		"--" + FlagCronExpression, "0 * * * *",
		"--" + FlagJitter, "30s",
		"--" + FlagCatchUpWindow, "1h",
		"--" + FlagPauseOnFailure,
	})
	c := cli.NewContext(app, set, nil)
	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.UpdateSchedule(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_UpdateSchedule_SpecWithoutCron(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)
	app := newScheduleTestApp(t, mockClient)

	mockClient.EXPECT().DescribeSchedule(gomock.Any(), gomock.Any()).
		Return(&types.DescribeScheduleResponse{}, nil)

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "", "")
	set.String(FlagTransport, "", "")
	set.String(FlagScheduleID, "", "")
	set.String(FlagJitter, "", "")
	_ = set.Parse([]string{
		"--" + FlagDomain, "test-domain",
		"--" + FlagTransport, grpcTransport,
		"--" + FlagScheduleID, "s",
		"--" + FlagJitter, "30s",
	})
	c := cli.NewContext(app, set, nil)
	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.UpdateSchedule(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--cron_expression is required")
}

// TestScheduleCLI_UpdateSchedule_PartialUpdatePreservesUnsetFields verifies that
// updating a subset of Spec/Policies fields preserves the rest, rather than
// resetting them to zero values.
func TestScheduleCLI_UpdateSchedule_PartialUpdatePreservesUnsetFields(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)
	app := newScheduleTestApp(t, mockClient)

	existingStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	existingEnd := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	mockClient.EXPECT().DescribeSchedule(gomock.Any(), gomock.Any()).
		Return(&types.DescribeScheduleResponse{
			Spec: &types.ScheduleSpec{
				CronExpression: "*/5 * * * *",
				StartTime:      existingStart,
				EndTime:        existingEnd,
				Jitter:         20 * time.Second,
			},
			Policies: &types.SchedulePolicies{
				OverlapPolicy:  types.ScheduleOverlapPolicyBuffer,
				CatchUpPolicy:  types.ScheduleCatchUpPolicyAll,
				CatchUpWindow:  3 * time.Hour,
				PauseOnFailure: true,
				BufferLimit:    9,
			},
		}, nil)

	mockClient.EXPECT().UpdateSchedule(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.UpdateScheduleRequest, _ ...interface{}) (*types.UpdateScheduleResponse, error) {
			// Only --jitter and --catch_up_window were passed; everything else
			// must carry over from the existing schedule.
			require.NotNil(t, req.Spec)
			assert.Equal(t, "*/5 * * * *", req.Spec.CronExpression)
			assert.True(t, existingStart.Equal(req.Spec.StartTime))
			assert.True(t, existingEnd.Equal(req.Spec.EndTime))
			assert.Equal(t, 45*time.Second, req.Spec.Jitter)

			require.NotNil(t, req.Policies)
			assert.Equal(t, types.ScheduleOverlapPolicyBuffer, req.Policies.OverlapPolicy)
			assert.Equal(t, types.ScheduleCatchUpPolicyAll, req.Policies.CatchUpPolicy)
			assert.Equal(t, time.Hour, req.Policies.CatchUpWindow)
			assert.True(t, req.Policies.PauseOnFailure)
			assert.Equal(t, int32(9), req.Policies.BufferLimit)

			return &types.UpdateScheduleResponse{}, nil
		})

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "", "")
	set.String(FlagTransport, "", "")
	set.String(FlagScheduleID, "", "")
	set.String(FlagJitter, "", "")
	set.String(FlagCatchUpWindow, "", "")
	_ = set.Parse([]string{
		"--" + FlagDomain, "test-domain",
		"--" + FlagTransport, grpcTransport,
		"--" + FlagScheduleID, "sched-partial",
		"--" + FlagJitter, "45s",
		"--" + FlagCatchUpWindow, "1h",
	})
	c := cli.NewContext(app, set, nil)
	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.UpdateSchedule(c)
	assert.NoError(t, err)
}

func TestScheduleCLI_BuildPoliciesFromFlags_WithNewFields(t *testing.T) {
	app := cli.NewApp()

	makeCtx := func(args []string) *cli.Context {
		set := flag.NewFlagSet("test", 0)
		set.String(FlagOverlapPolicy, "", "")
		set.String(FlagCatchUpPolicy, "", "")
		set.Int(FlagConcurrencyLimit, 0, "")
		set.String(FlagCatchUpWindow, "", "")
		set.Bool(FlagPauseOnFailure, false, "")
		set.Int(FlagBufferLimit, 0, "")
		_ = set.Parse(args)
		return cli.NewContext(app, set, nil)
	}

	tests := []struct {
		name       string
		args       []string
		wantResult *types.SchedulePolicies
		wantErr    bool
	}{
		{
			name:       "catch_up_window set",
			args:       []string{"--" + FlagCatchUpWindow, "2h"},
			wantResult: &types.SchedulePolicies{CatchUpWindow: 2 * time.Hour},
		},
		{
			name:       "pause_on_failure set",
			args:       []string{"--" + FlagPauseOnFailure},
			wantResult: &types.SchedulePolicies{PauseOnFailure: true},
		},
		{
			name:       "buffer_limit set",
			args:       []string{"--" + FlagBufferLimit, "5"},
			wantResult: &types.SchedulePolicies{BufferLimit: 5},
		},
		{
			name: "all new policy fields combined",
			args: []string{
				"--" + FlagCatchUpWindow, "30m",
				"--" + FlagPauseOnFailure,
				"--" + FlagBufferLimit, "10",
			},
			wantResult: &types.SchedulePolicies{
				CatchUpWindow:  30 * time.Minute,
				PauseOnFailure: true,
				BufferLimit:    10,
			},
		},
		{
			name:    "invalid catch_up_window returns error",
			args:    []string{"--" + FlagCatchUpWindow, "not-a-duration"},
			wantErr: true,
		},
		{
			name:    "negative buffer_limit returns error",
			args:    []string{"--" + FlagBufferLimit, "-1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := makeCtx(tt.args)
			got, err := buildPoliciesFromFlags(c, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResult, got)
			}
		})
	}
}

func TestScheduleCLI_PrintDescribeSchedule_AllFields(t *testing.T) {
	pausedAt := time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC)
	lastRun := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	nextRun := time.Date(2024, 6, 1, 10, 5, 0, 0, time.UTC)
	createTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	execTimeout := int32(3600)
	decTimeout := int32(10)

	resp := &types.DescribeScheduleResponse{
		Spec: &types.ScheduleSpec{
			CronExpression: "*/5 * * * *",
			StartTime:      startTime,
			EndTime:        endTime,
			Jitter:         30 * time.Second,
		},
		Action: &types.ScheduleAction{
			StartWorkflow: &types.StartWorkflowAction{
				WorkflowType:                        &types.WorkflowType{Name: "my-wf"},
				TaskList:                            &types.TaskList{Name: "my-tl"},
				WorkflowIDPrefix:                    "sched-wf-",
				ExecutionStartToCloseTimeoutSeconds: &execTimeout,
				TaskStartToCloseTimeoutSeconds:      &decTimeout,
				RetryPolicy: &types.RetryPolicy{
					MaximumAttempts:          3,
					InitialIntervalInSeconds: 1,
					BackoffCoefficient:       2.0,
					MaximumIntervalInSeconds: 60,
				},
			},
		},
		Policies: &types.SchedulePolicies{
			OverlapPolicy:  types.ScheduleOverlapPolicyBuffer,
			CatchUpPolicy:  types.ScheduleCatchUpPolicyAll,
			CatchUpWindow:  time.Hour,
			PauseOnFailure: true,
			BufferLimit:    5,
		},
		State: &types.ScheduleState{
			Paused: true,
			PauseInfo: &types.SchedulePauseInfo{
				Reason:   "maintenance",
				PausedBy: "admin@uber.com",
				PausedAt: pausedAt,
			},
		},
		Info: &types.ScheduleInfo{
			TotalRuns:            42,
			MissedRuns:           3,
			SkippedRuns:          1,
			BufferedFireCount:    5,
			RunningWorkflowCount: 2,
			LastRunTime:          lastRun,
			NextRunTime:          nextRun,
			CreateTime:           createTime,
			LastUpdateTime:       lastRun,
			OngoingBackfills: []*types.BackfillInfo{
				{
					BackfillID:    "bf-001",
					StartTime:     startTime,
					EndTime:       endTime,
					RunsCompleted: 15,
					RunsTotal:     30,
				},
			},
		},
	}

	// Capture stdout
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	printDescribeSchedule(resp)
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	output := string(out)

	checks := []string{
		"*/5 * * * *",
		"2024-01-01T00:00:00Z", // start time
		"2025-01-01T00:00:00Z", // end time
		"30s",                  // jitter
		"my-wf",
		"my-tl",
		"sched-wf-",
		"3600s", // exec timeout
		"10s",   // decision timeout
		"max_attempts=3",
		"BUFFER",
		"ALL",
		"1h",              // catch-up window
		"true",            // pause on failure
		"5 (0=unlimited)", // buffer limit
		"PAUSED",
		"maintenance",
		"admin@uber.com",
		"2024-06-01T09:00:00Z", // paused at
		"42",                   // total runs
		"3",                    // missed runs
		"1",                    // skipped runs
		"5",                    // buffered fires
		"2",                    // running workflows
		"2024-06-01T10:00:00Z", // last run
		"2024-06-01T10:05:00Z", // next run
		"2024-01-01T00:00:00Z", // created
		"bf-001",
		"15/30",
	}

	for _, want := range checks {
		assert.True(t, strings.Contains(output, want),
			fmt.Sprintf("expected output to contain %q\nactual output:\n%s", want, output))
	}
}

func TestScheduleCLI_CreateSchedule_InvalidJitter(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClient := frontend.NewMockClient(mockCtrl)
	app := newScheduleTestApp(t, mockClient)

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "", "")
	set.String(FlagTransport, "", "")
	set.String(FlagScheduleID, "", "")
	set.String(FlagCronExpression, "", "")
	set.String(FlagWorkflowType, "", "")
	set.Int(FlagExecutionTimeout, 0, "")
	set.Int(FlagDecisionTimeout, 0, "")
	set.String(FlagJitter, "", "")
	_ = set.Parse([]string{
		"--" + FlagDomain, "test-domain",
		"--" + FlagTransport, grpcTransport,
		"--" + FlagScheduleID, "s",
		"--" + FlagCronExpression, "*/5 * * * *",
		"--" + FlagWorkflowType, "wf",
		"--" + FlagExecutionTimeout, "3600",
		"--" + FlagDecisionTimeout, "10",
		"--" + FlagJitter, "not-a-duration",
	})
	c := cli.NewContext(app, set, nil)
	sc := &scheduleCLIImpl{frontendClient: mockClient}
	err := sc.CreateSchedule(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "jitter")
}
