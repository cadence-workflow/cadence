// Copyright (c) 2020 Uber Technologies, Inc.
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

package task

import (
	"testing"

	"github.com/uber/cadence/common/dynamicconfig"
)

func TestSchedulerOptionsString(t *testing.T) {
	tests := []struct {
		desc            string
		schedulerType   int
		queueSize       int
		workerCount     dynamicconfig.IntPropertyFn
		dispatcherCount int
		wantErr         bool
		want            string
	}{
		{
			desc:            "FIFO",
			schedulerType:   int(SchedulerTypeFIFO),
			queueSize:       1,
			workerCount:     dynamicconfig.GetIntPropertyFn(3),
			dispatcherCount: 1,
			want:            "{schedulerType:1, fifoSchedulerOptions:{QueueSize: 1, WorkerCount: 3, DispatcherCount: 1}, wrrSchedulerOptions:<nil>}",
		},
		{
			desc:            "WRR",
			schedulerType:   int(SchedulerTypeWRR),
			queueSize:       3,
			workerCount:     dynamicconfig.GetIntPropertyFn(4),
			dispatcherCount: 5,
			want:            "{schedulerType:2, fifoSchedulerOptions:<nil>, wrrSchedulerOptions:{QueueSize: 3, DispatcherCount: 5}}",
		},
		{
			desc:          "InvalidSchedulerType",
			schedulerType: 3,
			wantErr:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			o, err := NewSchedulerOptions[int](tc.schedulerType, tc.queueSize, tc.workerCount, tc.dispatcherCount, nil, nil)
			if (err != nil) != tc.wantErr {
				t.Errorf("Got error: %v, wantErr: %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if got := o.String(); got != tc.want {
				t.Errorf("Got: %v, want: %v", got, tc.want)
			}
		})
	}
}
