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
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
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
				return NewLogger(zapLogger)
			},
		},
		{
			name: "throttled",
			loggerFactory: func(zapLogger *zap.Logger) Logger {
				return NewThrottledLogger(NewLogger(zapLogger), dynamicproperties.GetIntPropertyFn(1))
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
					expected: `{"level":"info","msg":"` + defaultMsgForEmpty + `","wf-action":"add-workflow-started-event","logging-call-at":"logger_test.go:%s"}`,
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

	logger := NewLogger(zapLogger)
	preCaller := caller(1)
	logger.WithTags(tag.Error(fmt.Errorf("test error"))).Info("test info", tag.WorkflowActionWorkflowStarted)

	out := strings.TrimRight(buf.String(), "\n")
	sps := strings.Split(preCaller, ":")
	par, err := strconv.Atoi(sps[1])
	assert.Nil(t, err)
	lineNum := fmt.Sprintf("%v", par+1)
	assert.Equal(t, `{"level":"info","msg":"test info","error":"test error","wf-action":"add-workflow-started-event","logging-call-at":"logger_test.go:`+lineNum+`"}`, out)

}

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
	logger := NewLogger(zapLogger, WithSampleFunc(func(i int) bool {
		return true
	}))

	// No Assertion here, since default logger uses random function.
	// But we check that it is
	logger.SampleInfo("test info", 1, tag.WorkflowActionWorkflowStarted)
	out := strings.TrimRight(buf.String(), "\n")
	assert.Regexp(t, `{"level":"info","msg":"test info","wf-action":"add-workflow-started-event","logging-call-at":"logger_test.go:`+anyNum+`"}`, out)
}

func TestDebugOn(t *testing.T) {
	buf := &strings.Builder{}
	l := zap.NewAtomicLevel()
	zapLogger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}), zapcore.AddSync(buf), l))

	logger := NewLogger(zapLogger)

	// Set level to debug and check if debugOn is true
	l.SetLevel(zap.DebugLevel)
	assert.True(t, logger.DebugOn())

	// Set level to info and check if debugOn is false
	l.SetLevel(zap.InfoLevel)
	assert.False(t, logger.DebugOn())

	// Set level to debug again and check if debugOn is true
	l.SetLevel(zap.DebugLevel)
	assert.True(t, logger.DebugOn())
}

func TestHelper(t *testing.T) {
	// Test without Helper() first
	t.Run("WithoutHelper", func(t *testing.T) {
		buf := &strings.Builder{}
		zapLogger := createTestZapLogger(buf)
		logger := NewLogger(zapLogger)

		expectedLine := helperFuncWithoutHelper(logger)

		out := strings.TrimRight(buf.String(), "\n")

		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(out), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output as JSON: %v", err)
		}

		logCaller, ok := logEntry["caller"].(string)
		if !ok {
			t.Fatal("Log entry does not contain caller information")
		}

		require.Contains(t, logCaller, "logger_test.go")

		// Get the line from the caller string
		parts := strings.Split(logCaller, ":")
		if len(parts) != 2 {
			t.Fatalf("Unexpected caller format: %s", logCaller)
		}

		// Extract line number
		lineStr := parts[1]
		lineNum, err := strconv.Atoi(lineStr)
		if err != nil {
			t.Fatalf("Failed to parse line number from %s: %v", logCaller, err)
		}

		// With Helper(), the caller should be near the line where helper was called
		// Allow a small range of lines to avoid brittle tests
		if lineNum < expectedLine {
			t.Errorf("Caller line number (%d) is not near the calling line (%d)", lineNum, expectedLine)
		}
	})

	// Test with Helper()
	t.Run("WithHelper", func(t *testing.T) {
		buf := &strings.Builder{}
		zapLogger := createTestZapLogger(buf)
		logger := NewLogger(zapLogger)

		// Line number where the helper is called from the test
		callerLine := line() // Get current line number
		helperFuncWithHelper(logger)

		out := strings.TrimRight(buf.String(), "\n")

		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(out), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output as JSON: %v", err)
		}

		logCaller, ok := logEntry["caller"].(string)
		if !ok {
			t.Fatal("Log entry does not contain caller information")
		}

		require.Contains(t, logCaller, "logger_test.go")

		// Get the line from the caller string
		parts := strings.Split(logCaller, ":")
		if len(parts) != 2 {
			t.Fatalf("Unexpected caller format: %s", logCaller)
		}

		// Extract line number
		lineStr := parts[1]
		lineNum, err := strconv.Atoi(lineStr)
		if err != nil {
			t.Fatalf("Failed to parse line number from %s: %v", logCaller, err)
		}

		// With Helper(), the caller should be near the line where helper was called
		// Allow a small range of lines to avoid brittle tests
		if lineNum < callerLine+1 {
			t.Errorf("Caller line number (%d) is not near the calling line (%d)", lineNum, callerLine)
		}
	})
}

type testError struct {
	WorkflowID string
}

func (e testError) Error() string { return "test error" }
func (e testError) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("workflow-id", e.WorkflowID)
	return nil
}

// Helper function to create the zap logger
func createTestZapLogger(buf *strings.Builder) *zap.Logger {
	l := zap.NewAtomicLevel()
	return zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}), zapcore.AddSync(buf), l), zap.AddStacktrace(zapcore.ErrorLevel), zap.AddCaller())
}

func helperFuncWithoutHelper(l Logger) (lineNum int) {
	lineNum = line() + 1
	l.Error("test info", tag.WorkflowActionWorkflowStarted)
	return lineNum
}

func helperFuncWithHelper(l Logger) {
	l.Helper().Error("test info", tag.WorkflowActionWorkflowStarted)
}

// Helper function to get current line number
func line() int {
	_, _, line, _ := runtime.Caller(1)
	return line
}
