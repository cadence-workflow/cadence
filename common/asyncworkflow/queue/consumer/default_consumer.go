// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package consumer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
)

const (
	defaultShutdownTimeout = 5 * time.Second
	defaultStartWFTimeout  = 3 * time.Second
	defaultConcurrency     = 100
)

type DefaultConsumer struct {
	queueID          string
	innerConsumer    messaging.Consumer
	logger           log.Logger
	scope            metrics.Scope
	ctx              context.Context
	cancelFn         context.CancelFunc
	wg               sync.WaitGroup
	shutdownTimeout  time.Duration
	msgDecoder       codec.BinaryEncoder
	requestProcessor *RequestProcessor
	concurrency      int
}

type Option func(*DefaultConsumer)

func WithConcurrency(concurrency int) Option {
	return func(c *DefaultConsumer) {
		c.concurrency = concurrency
	}
}

func New(
	queueID string,
	innerConsumer messaging.Consumer,
	logger log.Logger,
	metricsClient metrics.Client,
	frontendClient frontend.Client,
	options ...Option,
) *DefaultConsumer {
	ctx, cancelFn := context.WithCancel(context.Background())
	taggedLogger := logger.WithTags(tag.AsyncWFQueueID(queueID))
	c := &DefaultConsumer{
		queueID:          queueID,
		innerConsumer:    innerConsumer,
		logger:           taggedLogger,
		scope:            metricsClient.Scope(metrics.AsyncWorkflowConsumerScope),
		ctx:              ctx,
		cancelFn:         cancelFn,
		shutdownTimeout:  defaultShutdownTimeout,
		msgDecoder:       codec.NewThriftRWEncoder(),
		requestProcessor: NewRequestProcessor(frontendClient, defaultStartWFTimeout, taggedLogger),
		concurrency:      defaultConcurrency,
	}

	for _, opt := range options {
		opt(c)
	}

	return c
}

func (c *DefaultConsumer) Start() error {
	if err := c.innerConsumer.Start(); err != nil {
		return err
	}

	for i := 0; i < c.concurrency; i++ {
		c.wg.Add(1)
		go c.runProcessLoop()
		c.logger.Info("Started process loop", tag.Counter(i))
	}
	c.logger.Info("Started consumer", tag.Dynamic("concurrency", c.concurrency))
	return nil
}

func (c *DefaultConsumer) Stop() {
	c.logger.Info("Stopping consumer")
	c.cancelFn()
	c.wg.Wait()
	if !common.AwaitWaitGroup(&c.wg, c.shutdownTimeout) {
		c.logger.Warn("Consumer timed out on shutdown", tag.Dynamic("timeout", c.shutdownTimeout))
		return
	}

	c.innerConsumer.Stop()
	c.logger.Info("Stopped consumer")
}

func (c *DefaultConsumer) runProcessLoop() {
	defer c.wg.Done()

	for {
		select {
		case msg, ok := <-c.innerConsumer.Messages():
			if !ok {
				c.logger.Info("Consumer channel closed")
				return
			}

			c.processMessage(msg)
		case <-c.ctx.Done():
			c.logger.Info("Consumer context done so terminating loop")
			return
		}
	}
}

func (c *DefaultConsumer) processMessage(msg messaging.Message) {
	logger := c.logger.WithTags(tag.Dynamic("partition", msg.Partition()), tag.Dynamic("offset", msg.Offset()))
	logger.Debug("Received message")

	asyncProcessStart := time.Now()
	sw := c.scope.StartTimer(metrics.AsyncWorkflowProcessMsgLatency)
	defer func() {
		sw.Stop()
		c.scope.ExponentialHistogram(metrics.AsyncWorkflowProcessMsgLatencyHistogram, time.Since(asyncProcessStart))
	}()

	var request sqlblobs.AsyncRequestMessage
	if err := c.msgDecoder.Decode(msg.Value(), &request); err != nil {
		logger.Error("Failed to decode message", tag.Error(err))
		c.scope.IncCounter(metrics.AsyncWorkflowFailureCorruptMsgCount)
		if err := msg.Nack(); err != nil {
			logger.Error("Failed to nack message", tag.Error(err))
		}
		return
	}

	logTags, err := c.processRequest(&request)
	if err != nil {
		logger.Error("Failed to process message", append(logTags, tag.Error(err))...)
		if nackErr := msg.Nack(); nackErr != nil {
			logger.Error("Failed to nack message", append(logTags, tag.Dynamic("original-error", err.Error()), tag.Error(nackErr))...)
		}
		return
	}

	logger = logger.WithTags(logTags...)
	if err := msg.Ack(); err != nil {
		logger.Error("Failed to ack message", tag.Error(err))
	}
	logger.Info("Processed message successfully")
}

func (c *DefaultConsumer) processRequest(request *sqlblobs.AsyncRequestMessage) ([]tag.Tag, error) {
	requestType := request.GetType().String()
	scope := c.scope.Tagged(metrics.AsyncWFRequestTypeTag(requestType))
	logTags := []tag.Tag{tag.AsyncWFRequestType(requestType)}

	prepared, err := c.requestProcessor.Prepare(request)
	if err != nil {
		scope.IncCounter(metrics.AsyncWorkflowFailureCorruptMsgCount)
		return logTags, err
	}

	scope = scope.Tagged(metrics.DomainTag(prepared.Domain))
	logTags = append(logTags, tag.WorkflowDomainName(prepared.Domain), tag.WorkflowID(prepared.WorkflowID))

	var runID string
	op := func(ctx context.Context) error {
		var err error
		runID, err = prepared.Invoke(ctx)
		return err
	}

	if err := callFrontendWithRetries(c.ctx, op); err != nil {
		scope.IncCounter(metrics.AsyncWorkflowFailureByFrontendCount)
		return logTags, fmt.Errorf("%s failed after all attempts: %w", requestType, err)
	}

	scope.IncCounter(metrics.AsyncWorkflowSuccessCount)
	logTags = append(logTags, tag.WorkflowRunID(runID))
	return logTags, nil
}

func callFrontendWithRetries(ctx context.Context, op func(ctx context.Context) error) error {
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(common.CreateFrontendServiceRetryPolicy()),
		backoff.WithRetryableError(isRetryableProcessingError),
	)

	return throttleRetry.Do(ctx, op)
}

// isRetryableProcessingError gates retries of the frontend invocation: corrupt
// messages are never retryable, everything else follows the standard transient
// error classification.
func isRetryableProcessingError(err error) bool {
	if errors.Is(err, ErrCorruptMessage) {
		return false
	}
	return common.IsServiceTransientError(err)
}
