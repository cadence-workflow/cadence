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
