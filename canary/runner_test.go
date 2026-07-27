// Copyright (c) 2019 Uber Technologies, Inc.
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

package canary

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRunCanaries(t *testing.T) {
	errA := errors.New("domain a failed")
	errB := errors.New("domain b failed")

	tests := []struct {
		name    string
		results []error
		wantErr bool
	}{
		{
			name:    "all succeed",
			results: []error{nil, nil},
			wantErr: false,
		},
		{
			name:    "one fails one succeeds",
			results: []error{errA, nil},
			wantErr: false,
		},
		{
			name:    "all fail",
			results: []error{errA, errB},
			wantErr: true,
		},
		{
			name:    "no tasks",
			results: []error{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := make([]Runnable, len(tt.results))
			for i, res := range tt.results {
				res := res
				tasks[i] = runnableFunc(func(mode string) error {
					return res
				})
			}

			err := runCanaries(tasks, ModeAll, zap.NewNop())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
