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
	"math/rand"

	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/log/tag"
)

type replayLogger struct {
	logger            Logger
	ctx               workflow.Context
	enableLogInReplay bool
}

const skipForReplayLogger = skipForDefaultLogger + 1

// NewReplayLogger creates a logger which is aware of cadence's replay mode
func NewReplayLogger(logger Logger, ctx workflow.Context, enableLogInReplay bool) Logger {
	lg, ok := logger.(*loggerImpl)
	if ok {
		logger = &loggerImpl{
			zapLogger:     lg.zapLogger,
			skip:          skipForReplayLogger,
			sampleLocalFn: lg.sampleLocalFn,
		}
	} else {
		logger.Warn("ReplayLogger may not emit callat tag correctly because the logger passed in is not loggerImpl")
	}
	return &replayLogger{
		logger:            logger,
		ctx:               ctx,
		enableLogInReplay: enableLogInReplay,
	}
}

func (r *replayLogger) Debugf(msg string, args ...any) {
	if workflow.IsReplaying(r.ctx) && !r.enableLogInReplay {
		return
	}

	r.logger.Debugf(fmt.Sprintf(msg, args...))
}

func (r *replayLogger) Debug(msg string, tags ...tag.Tag) {
	if workflow.IsReplaying(r.ctx) && !r.enableLogInReplay {
		return
	}
	r.logger.Debug(msg, tags...)
}

func (r *replayLogger) Info(msg string, tags ...tag.Tag) {
	if workflow.IsReplaying(r.ctx) && !r.enableLogInReplay {
		return
	}
	r.logger.Info(msg, tags...)
}

func (r *replayLogger) Warn(msg string, tags ...tag.Tag) {
	if workflow.IsReplaying(r.ctx) && !r.enableLogInReplay {
		return
	}
	r.logger.Warn(msg, tags...)
}

func (r *replayLogger) Error(msg string, tags ...tag.Tag) {
	if workflow.IsReplaying(r.ctx) && !r.enableLogInReplay {
		return
	}
	r.logger.Error(msg, tags...)
}

func (r *replayLogger) Fatal(msg string, tags ...tag.Tag) {
	if workflow.IsReplaying(r.ctx) && !r.enableLogInReplay {
		return
	}
	r.logger.Fatal(msg, tags...)
}

func (r *replayLogger) SampleInfo(msg string, sampleRate int, tags ...tag.Tag) {
	if rand.Intn(sampleRate) == 0 {
		if workflow.IsReplaying(r.ctx) && !r.enableLogInReplay {
			return
		}
		r.logger.Info(msg, tags...)
	}
}

func (r *replayLogger) DebugOn() bool {
	return r.logger.DebugOn()
}

func (r *replayLogger) WithTags(tags ...tag.Tag) Logger {
	return &replayLogger{
		logger:            r.logger.WithTags(tags...),
		ctx:               r.ctx,
		enableLogInReplay: r.enableLogInReplay,
	}
}

func (r *replayLogger) Helper() Logger {
	return &replayLogger{
		logger:            r.logger.Helper(),
		ctx:               r.ctx,
		enableLogInReplay: r.enableLogInReplay,
	}
}
