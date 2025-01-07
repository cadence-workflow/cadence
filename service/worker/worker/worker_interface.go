// The MIT License (MIT)

// Copyright (c) 2024 Uber Technologies Inc.

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

package worker

import (
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
)

//go:generate mockgen -source $GOFILE -destination worker_mock.go -package worker github.com/uber/cadence/service/worker/worker Worker

type Worker interface {
	RegisterActivity(activity interface{})
	RegisterActivityWithOptions(activity interface{}, options activity.RegisterOptions)
	RegisterWorkflow(workflow interface{})
	RegisterWorkflowWithOptions(workflow interface{}, options workflow.RegisterOptions)
	Start() error
	Stop()
	Run() error
}
