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

import (
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

type queueProcessorOptions struct {
	BatchSize                            dynamicproperties.IntPropertyFn
	DeleteBatchSize                      dynamicproperties.IntPropertyFn
	MaxPollRPS                           dynamicproperties.IntPropertyFn
	MaxPollInterval                      dynamicproperties.DurationPropertyFn
	MaxPollIntervalJitterCoefficient     dynamicproperties.FloatPropertyFn
	UpdateAckInterval                    dynamicproperties.DurationPropertyFn
	UpdateAckIntervalJitterCoefficient   dynamicproperties.FloatPropertyFn
	RedispatchInterval                   dynamicproperties.DurationPropertyFn
	MaxRedispatchQueueSize               dynamicproperties.IntPropertyFn
	MaxStartJitterInterval               dynamicproperties.DurationPropertyFn
	SplitQueueInterval                   dynamicproperties.DurationPropertyFn
	SplitQueueIntervalJitterCoefficient  dynamicproperties.FloatPropertyFn
	EnableSplit                          dynamicproperties.BoolPropertyFn
	SplitMaxLevel                        dynamicproperties.IntPropertyFn
	EnableRandomSplitByDomainID          dynamicproperties.BoolPropertyFnWithDomainIDFilter
	RandomSplitProbability               dynamicproperties.FloatPropertyFn
	EnablePendingTaskSplitByDomainID     dynamicproperties.BoolPropertyFnWithDomainIDFilter
	PendingTaskSplitThreshold            dynamicproperties.MapPropertyFn
	EnableStuckTaskSplitByDomainID       dynamicproperties.BoolPropertyFnWithDomainIDFilter
	StuckTaskSplitThreshold              dynamicproperties.MapPropertyFn
	SplitLookAheadDurationByDomainID     dynamicproperties.DurationPropertyFnWithDomainIDFilter
	PollBackoffInterval                  dynamicproperties.DurationPropertyFn
	PollBackoffIntervalJitterCoefficient dynamicproperties.FloatPropertyFn
	EnablePersistQueueStates             dynamicproperties.BoolPropertyFn
	EnableLoadQueueStates                dynamicproperties.BoolPropertyFn
	EnableGracefulSyncShutdown           dynamicproperties.BoolPropertyFn
	EnableValidator                      dynamicproperties.BoolPropertyFn
	ValidationInterval                   dynamicproperties.DurationPropertyFn
	// MaxPendingTaskSize is used in cross cluster queue to limit the pending task count
	MaxPendingTaskSize dynamicproperties.IntPropertyFn
	MetricScope        metrics.ScopeIdx
}
