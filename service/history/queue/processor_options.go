// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package queue

import "github.com/uber/cadence/common/dynamicconfig"

type queueProcessorOptions struct {
	BatchSize                            dynamicconfig.IntPropertyFn
	DeleteBatchSize                      dynamicconfig.IntPropertyFn
	MaxPollRPS                           dynamicconfig.IntPropertyFn
	MaxPollInterval                      dynamicconfig.DurationPropertyFn
	MaxPollIntervalJitterCoefficient     dynamicconfig.FloatPropertyFn
	UpdateAckInterval                    dynamicconfig.DurationPropertyFn
	UpdateAckIntervalJitterCoefficient   dynamicconfig.FloatPropertyFn
	RedispatchInterval                   dynamicconfig.DurationPropertyFn
	MaxRedispatchQueueSize               dynamicconfig.IntPropertyFn
	MaxStartJitterInterval               dynamicconfig.DurationPropertyFn
	SplitQueueInterval                   dynamicconfig.DurationPropertyFn
	SplitQueueIntervalJitterCoefficient  dynamicconfig.FloatPropertyFn
	EnableSplit                          dynamicconfig.BoolPropertyFn
	SplitMaxLevel                        dynamicconfig.IntPropertyFn
	EnableRandomSplitByDomainID          dynamicconfig.BoolPropertyFnWithDomainIDFilter
	RandomSplitProbability               dynamicconfig.FloatPropertyFn
	EnablePendingTaskSplitByDomainID     dynamicconfig.BoolPropertyFnWithDomainIDFilter
	PendingTaskSplitThreshold            dynamicconfig.MapPropertyFn
	EnableStuckTaskSplitByDomainID       dynamicconfig.BoolPropertyFnWithDomainIDFilter
	StuckTaskSplitThreshold              dynamicconfig.MapPropertyFn
	SplitLookAheadDurationByDomainID     dynamicconfig.DurationPropertyFnWithDomainIDFilter
	PollBackoffInterval                  dynamicconfig.DurationPropertyFn
	PollBackoffIntervalJitterCoefficient dynamicconfig.FloatPropertyFn
	EnablePersistQueueStates             dynamicconfig.BoolPropertyFn
	EnableLoadQueueStates                dynamicconfig.BoolPropertyFn
	EnableGracefulSyncShutdown           dynamicconfig.BoolPropertyFn
	EnableValidator                      dynamicconfig.BoolPropertyFn
	ValidationInterval                   dynamicconfig.DurationPropertyFn
	// MaxPendingTaskSize is used in cross cluster queue to limit the pending task count
	MaxPendingTaskSize dynamicconfig.IntPropertyFn
	MetricScope        int
}
