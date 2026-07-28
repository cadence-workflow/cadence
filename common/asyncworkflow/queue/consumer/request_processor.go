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
	"sort"
	"time"

	"go.uber.org/yarpc"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

// ErrCorruptMessage marks async request messages that can never be processed
// successfully: undecodable envelope or payload, unsupported encoding, or
// unsupported request type. Callers should route such messages to a DLQ (or
// nack them) instead of retrying.
var ErrCorruptMessage = errors.New("corrupt async request message")

type (
	// PreparedRequest is a fully decoded async workflow request bound to a
	// single-attempt frontend invocation. Retry policy is the caller's concern.
	PreparedRequest struct {
		// RequestType is the string form of sqlblobs.AsyncRequestType.
		RequestType string
		// Domain is the domain name from the inner request.
		Domain string
		// WorkflowID is the workflow ID from the inner request.
		WorkflowID string
		// Invoke performs one frontend call attempt, bounded by the processor's
		// call timeout. WorkflowExecutionAlreadyStartedError is treated as
		// success. On success it returns the run ID of the (already) started
		// execution.
		Invoke func(ctx context.Context) (runID string, err error)
	}

	// RequestProcessor decodes async request messages into frontend
	// invocations. It is shared by the Kafka DefaultConsumer and the history
	// service's per-shard queue consumer.
	RequestProcessor struct {
		frontendClient frontend.Client
		msgDecoder     codec.BinaryEncoder
		callTimeout    time.Duration
		logger         log.Logger
	}
)

// NewRequestProcessor creates a RequestProcessor calling the given frontend
// client with the given per-attempt timeout.
func NewRequestProcessor(
	frontendClient frontend.Client,
	callTimeout time.Duration,
	logger log.Logger,
) *RequestProcessor {
	return &RequestProcessor{
		frontendClient: frontendClient,
		msgDecoder:     codec.NewThriftRWEncoder(),
		callTimeout:    callTimeout,
		logger:         logger,
	}
}

// DecodeEnvelope decodes the raw ThriftRW-encoded AsyncRequestMessage envelope.
// Failures wrap ErrCorruptMessage.
func (p *RequestProcessor) DecodeEnvelope(payload []byte) (*sqlblobs.AsyncRequestMessage, error) {
	var request sqlblobs.AsyncRequestMessage
	if err := p.msgDecoder.Decode(payload, &request); err != nil {
		return nil, fmt.Errorf("%w: decoding envelope: %v", ErrCorruptMessage, err)
	}
	return &request, nil
}

// Prepare decodes the inner request of the envelope and returns it bound to a
// single-attempt frontend invocation. Decode failures and unsupported
// encodings/types wrap ErrCorruptMessage.
func (p *RequestProcessor) Prepare(request *sqlblobs.AsyncRequestMessage) (*PreparedRequest, error) {
	yarpcCallOpts := getYARPCOptions(request.GetHeader())

	switch request.GetType() {
	case sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest:
		startWFReq, err := p.decodeStartWorkflowRequest(request.GetPayload(), request.GetEncoding())
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCorruptMessage, err)
		}
		return &PreparedRequest{
			RequestType: request.GetType().String(),
			Domain:      startWFReq.GetDomain(),
			WorkflowID:  startWFReq.GetWorkflowID(),
			Invoke: func(ctx context.Context) (string, error) {
				ctx, cancel := context.WithTimeout(ctx, p.callTimeout)
				defer cancel()
				resp, err := p.frontendClient.StartWorkflowExecution(ctx, startWFReq, yarpcCallOpts...)
				if runID, ok := p.alreadyStarted(err, startWFReq.GetWorkflowID()); ok {
					return runID, nil
				}
				if err != nil {
					return "", err
				}
				return resp.GetRunID(), nil
			},
		}, nil
	case sqlblobs.AsyncRequestTypeSignalWithStartWorkflowExecutionAsyncRequest:
		signalWithStartReq, err := p.decodeSignalWithStartWorkflowRequest(request.GetPayload(), request.GetEncoding())
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCorruptMessage, err)
		}
		return &PreparedRequest{
			RequestType: request.GetType().String(),
			Domain:      signalWithStartReq.GetDomain(),
			WorkflowID:  signalWithStartReq.GetWorkflowID(),
			Invoke: func(ctx context.Context) (string, error) {
				ctx, cancel := context.WithTimeout(ctx, p.callTimeout)
				defer cancel()
				resp, err := p.frontendClient.SignalWithStartWorkflowExecution(ctx, signalWithStartReq, yarpcCallOpts...)
				if runID, ok := p.alreadyStarted(err, signalWithStartReq.GetWorkflowID()); ok {
					return runID, nil
				}
				if err != nil {
					return "", err
				}
				return resp.GetRunID(), nil
			},
		}, nil
	default:
		return nil, fmt.Errorf("%w: %v", ErrCorruptMessage, &UnsupportedRequestType{Type: request.GetType()})
	}
}

// alreadyStarted reports whether err is WorkflowExecutionAlreadyStartedError,
// which async request processing treats as success (idempotent redelivery).
func (p *RequestProcessor) alreadyStarted(err error, workflowID string) (string, bool) {
	var startedError *types.WorkflowExecutionAlreadyStartedError
	if errors.As(err, &startedError) {
		p.logger.Info("Received WorkflowExecutionAlreadyStartedError, treating it as a success",
			tag.WorkflowID(workflowID), tag.WorkflowRunID(startedError.RunID))
		return startedError.RunID, true
	}
	return "", false
}

func (p *RequestProcessor) decodeStartWorkflowRequest(payload []byte, encoding string) (*types.StartWorkflowExecutionRequest, error) {
	if encoding != string(constants.EncodingTypeThriftRW) {
		return nil, &UnsupportedEncoding{EncodingType: encoding}
	}

	var thriftObj shared.StartWorkflowExecutionAsyncRequest
	if err := p.msgDecoder.Decode(payload, &thriftObj); err != nil {
		return nil, err
	}

	startRequest := thrift.ToStartWorkflowExecutionAsyncRequest(&thriftObj)
	return startRequest.StartWorkflowExecutionRequest, nil
}

func (p *RequestProcessor) decodeSignalWithStartWorkflowRequest(payload []byte, encoding string) (*types.SignalWithStartWorkflowExecutionRequest, error) {
	if encoding != string(constants.EncodingTypeThriftRW) {
		return nil, &UnsupportedEncoding{EncodingType: encoding}
	}

	var thriftObj shared.SignalWithStartWorkflowExecutionAsyncRequest
	if err := p.msgDecoder.Decode(payload, &thriftObj); err != nil {
		return nil, err
	}

	signalWithStartRequest := thrift.ToSignalWithStartWorkflowExecutionAsyncRequest(&thriftObj)
	return signalWithStartRequest.SignalWithStartWorkflowExecutionRequest, nil
}

func getYARPCOptions(header *shared.Header) []yarpc.CallOption {
	if header == nil || header.GetFields() == nil {
		return nil
	}

	// sort the header fields to make the tests deterministic
	fields := header.GetFields()
	sortedKeys := make([]string, 0, len(fields))
	for k := range fields {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var opts []yarpc.CallOption
	for _, k := range sortedKeys {
		opts = append(opts, yarpc.WithHeader(k, string(fields[k])))
	}
	return opts
}
