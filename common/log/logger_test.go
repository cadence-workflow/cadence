// Copyright (c) 2017 Uber Technologies, Inc.
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

package log

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/uber/cadence/common/log/tag"
)

const anyNum = "[0-9]+"

func TestLoggers(t *testing.T) {
	for _, tc := range []struct {
		name          string
		loggerFactory func(zapLogger *zap.Logger) Logger
	}{
		{
			name: "normal",
			loggerFactory: func(zapLogger *zap.Logger) Logger {
				return New(zapLogger)
			},
		},
		{
			name: "throttled",
			loggerFactory: func(zapLogger *zap.Logger) Logger {
				return NewThrottledLogger(New(zapLogger), func() int { return 1 })
			},
		},
		// Unfortunately, replay logger is impossible to test because it requires a workflow context, which is not exposed by go client.
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, scenario := range []struct {
				name     string
				fn       func(logger Logger)
				expected string
			}{
				{
					name: "debug",
					fn: func(logger Logger) {
						logger.Debug("test debug")
					},
					expected: `{"level":"debug","msg":"test debug","logging-call-at":"logger_test.go:%s"}`,
				},
				{
					name: "info",
					fn: func(logger Logger) {
						logger.Info("test info")
					},
					expected: `{"level":"info","msg":"test info","logging-call-at":"logger_test.go:%s"}`,
				},
				{
					name: "warn",
					fn: func(logger Logger) {
						logger.Warn("test warn")
					},
					expected: `{"level":"warn","msg":"test warn","logging-call-at":"logger_test.go:%s"}`,
				},
				{
					name: "error",
					fn: func(logger Logger) {
						logger.Error("test error")
					},
					expected: `{"level":"error","msg":"test error","logging-call-at":"logger_test.go:%s"}`,
				},
				{
					name: "debugf",
					fn: func(logger Logger) {
						logger.Debugf("test %v", "debug")
					},
					expected: `{"level":"debug","msg":"test debug","logging-call-at":"logger_test.go:%s"}`,
				},
				{
					name: "with tags",
					fn: func(logger Logger) {
						logger.WithTags(tag.Error(fmt.Errorf("test error"))).Info("test info", tag.WorkflowActionWorkflowStarted)
					},
					expected: `{"level":"info","msg":"test info","error":"test error","wf-action":"add-workflow-started-event","logging-call-at":"logger_test.go:%s"}`,
				},
				{
					name: "empty message",
					fn: func(logger Logger) {
						logger.Info("", tag.WorkflowActionWorkflowStarted)
					},
					expected: `{"level":"info","msg":"` + DefaultMsgForEmpty + `","wf-action":"add-workflow-started-event","logging-call-at":"logger_test.go:%s"}`,
				},
				{
					name: "error with details",
					fn: func(logger Logger) {
						logger.Error("oh no", tag.Error(&testError{"workflow123"}))
					},
					expected: `{"level":"error","msg":"oh no","error":"test error","error-details":{"workflow-id":"workflow123"},"logging-call-at":"logger_test.go:%s"}`,
				},
			} {
				t.Run(scenario.name, func(t *testing.T) {
					buf := &strings.Builder{}
					zapLogger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zapcore.EncoderConfig{
						MessageKey:     "msg",
						LevelKey:       "level",
						NameKey:        "logger",
						EncodeLevel:    zapcore.LowercaseLevelEncoder,
						EncodeTime:     zapcore.ISO8601TimeEncoder,
						EncodeDuration: zapcore.StringDurationEncoder,
					}), zapcore.AddSync(buf), zap.DebugLevel))
					logger := tc.loggerFactory(zapLogger)
					scenario.fn(logger)
					out := strings.TrimRight(buf.String(), "\n")
					expected := fmt.Sprintf(scenario.expected, anyNum)
					assert.Regexp(t, expected, out)
				})
			}
		})
	}
}

func TestLoggerCallAt(t *testing.T) {
	buf := &strings.Builder{}
	zapLogger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}), zapcore.AddSync(buf), zap.DebugLevel))

	logger := New(zapLogger)
	preCaller := caller(1)
	logger.WithTags(tag.Error(fmt.Errorf("test error"))).Info("test info", tag.WorkflowActionWorkflowStarted)

	out := strings.TrimRight(buf.String(), "\n")
	sps := strings.Split(preCaller, ":")
	par, err := strconv.Atoi(sps[1])
	assert.Nil(t, err)
	lineNum := fmt.Sprintf("%v", par+1)
	assert.Equal(t, `{"level":"info","msg":"test info","error":"test error","wf-action":"add-workflow-started-event","logging-call-at":"logger_test.go:`+lineNum+`"}`, out)

}

var sampleFnStub = func(i int) bool { return true }

func TestLogger_Sampled(t *testing.T) {
	buf := &strings.Builder{}
	zapLogger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}), zapcore.AddSync(buf), zap.DebugLevel))
	logger := newLogger(zapLogger, 0, sampleFnStub)

	// No Assertion here, since default logger uses random function.
	// But we check that it is
	logger.SampleInfo("test info", 1, tag.WorkflowActionWorkflowStarted)
	out := strings.TrimRight(buf.String(), "\n")
	assert.Regexp(t, `{"level":"info","msg":"test info","wf-action":"add-workflow-started-event","logging-call-at":"logger_test.go:`+anyNum+`"}`, out)
}

type testError struct {
	WorkflowID string
}

func (e testError) Error() string { return "test error" }
func (e testError) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("workflow-id", e.WorkflowID)
	return nil
}
