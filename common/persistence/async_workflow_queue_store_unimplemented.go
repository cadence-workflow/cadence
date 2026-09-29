// Copyright (c) 2026 Uber Technologies, Inc.
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

package persistence

import (
	"context"
	"errors"
)

// ErrAsyncWorkflowQueueNotImplemented is returned by every data method of the unimplemented store.
// It is deliberately not a BadRequestError so that metrics count it as a persistence failure.
var ErrAsyncWorkflowQueueNotImplemented = errors.New("async workflow queue persistence is not implemented")

type unimplementedAsyncWorkflowQueueStore struct {
	name string
}

var _ AsyncWorkflowQueueStore = (*unimplementedAsyncWorkflowQueueStore)(nil)

// NewUnimplementedAsyncWorkflowQueueStore returns a store for backends without async workflow queue support.
func NewUnimplementedAsyncWorkflowQueueStore(name string) AsyncWorkflowQueueStore {
	return &unimplementedAsyncWorkflowQueueStore{name: name}
}

func (s *unimplementedAsyncWorkflowQueueStore) GetName() string { return s.name }
func (s *unimplementedAsyncWorkflowQueueStore) Close()          {}

func (s *unimplementedAsyncWorkflowQueueStore) EnqueueAsyncWorkflowMessage(_ context.Context, _ *InternalEnqueueAsyncWorkflowMessageRequest) error {
	return ErrAsyncWorkflowQueueNotImplemented
}

func (s *unimplementedAsyncWorkflowQueueStore) ReadAsyncWorkflowMessages(_ context.Context, _ *ReadAsyncWorkflowMessagesRequest) (*ReadAsyncWorkflowMessagesResponse, error) {
	return nil, ErrAsyncWorkflowQueueNotImplemented
}

func (s *unimplementedAsyncWorkflowQueueStore) GetAsyncWorkflowAckLevels(_ context.Context, _ *GetAsyncWorkflowAckLevelsRequest) (*GetAsyncWorkflowAckLevelsResponse, error) {
	return nil, ErrAsyncWorkflowQueueNotImplemented
}

func (s *unimplementedAsyncWorkflowQueueStore) UpdateAsyncWorkflowAckLevel(_ context.Context, _ *InternalUpdateAsyncWorkflowAckLevelRequest) error {
	return ErrAsyncWorkflowQueueNotImplemented
}

func (s *unimplementedAsyncWorkflowQueueStore) RangeDeleteAsyncWorkflowMessages(_ context.Context, _ *RangeDeleteAsyncWorkflowMessagesRequest) error {
	return ErrAsyncWorkflowQueueNotImplemented
}

func (s *unimplementedAsyncWorkflowQueueStore) EnqueueAsyncWorkflowMessageToDLQ(_ context.Context, _ *InternalEnqueueAsyncWorkflowMessageToDLQRequest) error {
	return ErrAsyncWorkflowQueueNotImplemented
}

func (s *unimplementedAsyncWorkflowQueueStore) ReadAsyncWorkflowMessagesFromDLQ(_ context.Context, _ *ReadAsyncWorkflowMessagesFromDLQRequest) (*ReadAsyncWorkflowMessagesFromDLQResponse, error) {
	return nil, ErrAsyncWorkflowQueueNotImplemented
}

func (s *unimplementedAsyncWorkflowQueueStore) RangeDeleteAsyncWorkflowMessagesFromDLQ(_ context.Context, _ *RangeDeleteAsyncWorkflowMessagesFromDLQRequest) error {
	return ErrAsyncWorkflowQueueNotImplemented
}
