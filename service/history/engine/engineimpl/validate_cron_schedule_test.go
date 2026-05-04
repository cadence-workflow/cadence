// Copyright (c) 2017-2021 Uber Technologies, Inc.
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
)

func TestValidateCronSchedule(t *testing.T) {
	const testDomain = "test-domain"

	type configOverride struct {
		minInterval time.Duration
		enforce     bool
	}

	tests := []struct {
		name         string
		cronSchedule string
		cfg          configOverride
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name:         "no cron schedule is allowed",
			cronSchedule: "",
			cfg:          configOverride{minInterval: time.Hour, enforce: true},
		},
		{
			name:         "min interval disabled accepts any schedule",
			cronSchedule: "* * * * *",
			cfg:          configOverride{minInterval: 0, enforce: true},
		},
		{
			name:         "schedule meeting the minimum is accepted",
			cronSchedule: "0 * * * *", // hourly
			cfg:          configOverride{minInterval: time.Hour, enforce: true},
		},
		{
			name:         "schedule above the minimum is accepted",
			cronSchedule: "0 */6 * * *", // every 6 hours
			cfg:          configOverride{minInterval: time.Hour, enforce: true},
		},
		{
			name:         "violation with enforcement rejects the request",
			cronSchedule: "* * * * *", // every minute
			cfg:          configOverride{minInterval: time.Hour, enforce: true},
			wantErr:      true,
			wantErrMsg:   "below the minimum allowed interval",
		},
		{
			name:         "violation without enforcement logs but accepts",
			cronSchedule: "* * * * *",
			cfg:          configOverride{minInterval: time.Hour, enforce: false},
		},
		{
			name:         "non-uniform interval caught by min gap",
			cronSchedule: "0 9,17 * * *", // gaps of 8h and 16h
			cfg:          configOverride{minInterval: 12 * time.Hour, enforce: true},
			wantErr:      true,
			wantErrMsg:   "below the minimum allowed interval",
		},
		{
			name:         "every 30 seconds violates 1 minute minimum",
			cronSchedule: "@every 30s",
			cfg:          configOverride{minInterval: time.Minute, enforce: true},
			wantErr:      true,
		},
		{
			name:         "invalid cron schedule is not rejected by this validator",
			cronSchedule: "not-a-cron",
			cfg:          configOverride{minInterval: time.Hour, enforce: true},
			// The existing pipeline validates the schedule via backoff.ValidateSchedule
			// elsewhere; this validator only applies when the schedule can be parsed,
			// so it must not double-surface parse errors.
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			override := tt.cfg
			cfg := &config.Config{
				MinCronInterval: func(domain string) time.Duration {
					require.Equal(t, testDomain, domain)
					return override.minInterval
				},
				EnforceMinCronInterval: func(domain string) bool {
					require.Equal(t, testDomain, domain)
					return override.enforce
				},
			}
			e := &historyEngineImpl{
				config: cfg,
				logger: testlogger.New(t),
			}
			request := &types.StartWorkflowExecutionRequest{
				Domain:       testDomain,
				WorkflowID:   "test-workflow",
				WorkflowType: &types.WorkflowType{Name: "test-workflow-type"},
				CronSchedule: tt.cronSchedule,
			}

			err := e.validateCronSchedule(request)
			if tt.wantErr {
				require.Error(t, err)
				var badRequest *types.BadRequestError
				require.ErrorAs(t, err, &badRequest)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
