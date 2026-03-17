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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
)

var testTimeNow = time.Date(2024, 12, 30, 23, 59, 59, 0, time.UTC)

func setUpMocksForDomainAuditManager(t *testing.T) (*domainAuditManagerImpl, *MockDomainAuditStore) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockStore := NewMockDomainAuditStore(ctrl)

	m := &domainAuditManagerImpl{
		persistence: mockStore,
		timeSrc:     clock.NewMockedTimeSourceAt(testTimeNow),
		serializer:  NewPayloadSerializer(),
		logger:      log.NewNoop(),
		dc: &DynamicConfiguration{
			DomainAuditLogTTL: func(domainID string) time.Duration { return time.Hour * 24 * 365 },
		},
	}

	return m, mockStore
}

func TestGetDomainAuditLogs(t *testing.T) {
	ctx := context.Background()
	epoch := time.Unix(0, 0)
	minTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	fixedTime := time.Date(2024, 6, 30, 23, 59, 59, 0, time.UTC)

	testCases := []struct {
		name            string
		request         *GetDomainAuditLogsRequest
		expectedRequest *GetDomainAuditLogsRequest
		storeResp       *InternalGetDomainAuditLogsResponse
		storeErr        error
		wantErr         bool
	}{
		{
			name: "nil MinCreatedTime defaults to epoch",
			request: &GetDomainAuditLogsRequest{
				DomainID:       "domain-1",
				MaxCreatedTime: &fixedTime,
			},
			expectedRequest: &GetDomainAuditLogsRequest{
				DomainID:       "domain-1",
				MinCreatedTime: &epoch,
				MaxCreatedTime: &fixedTime,
			},
			storeResp: &InternalGetDomainAuditLogsResponse{},
		},
		{
			name: "nil MaxCreatedTime defaults to timeSrc.Now()",
			request: &GetDomainAuditLogsRequest{
				DomainID:       "domain-1",
				MinCreatedTime: &minTime,
			},
			expectedRequest: &GetDomainAuditLogsRequest{
				DomainID:       "domain-1",
				MinCreatedTime: &minTime,
				MaxCreatedTime: &testTimeNow,
			},
			storeResp: &InternalGetDomainAuditLogsResponse{},
		},
		{
			name: "both nil times defaulted",
			request: &GetDomainAuditLogsRequest{
				DomainID: "domain-1",
			},
			expectedRequest: &GetDomainAuditLogsRequest{
				DomainID:       "domain-1",
				MinCreatedTime: &epoch,
				MaxCreatedTime: &testTimeNow,
			},
			storeResp: &InternalGetDomainAuditLogsResponse{},
		},
		{
			name: "non-nil times passed through unchanged",
			request: &GetDomainAuditLogsRequest{
				DomainID:       "domain-1",
				MinCreatedTime: &minTime,
				MaxCreatedTime: &testTimeNow,
			},
			expectedRequest: &GetDomainAuditLogsRequest{
				DomainID:       "domain-1",
				MinCreatedTime: &minTime,
				MaxCreatedTime: &testTimeNow,
			},
			storeResp: &InternalGetDomainAuditLogsResponse{},
		},
		{
			name: "store error propagated",
			request: &GetDomainAuditLogsRequest{
				DomainID:       "domain-1",
				MinCreatedTime: &minTime,
				MaxCreatedTime: &testTimeNow,
			},
			expectedRequest: &GetDomainAuditLogsRequest{
				DomainID:       "domain-1",
				MinCreatedTime: &minTime,
				MaxCreatedTime: &testTimeNow,
			},
			storeErr: errors.New("store error"),
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m, mockStore := setUpMocksForDomainAuditManager(t)
			mockStore.EXPECT().GetDomainAuditLogs(ctx, tc.expectedRequest).Return(tc.storeResp, tc.storeErr).Times(1)

			resp, err := m.GetDomainAuditLogs(ctx, tc.request)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestGetDomainAuditLogs_MutateDefaultTimeOnRetry(t *testing.T) {
	m, mockStore := setUpMocksForDomainAuditManager(t)
	mockTime := m.timeSrc.(clock.MockedTimeSource)

	t1 := mockTime.Now()

	// a request with nil MaxCreatedTime (meaning "up to now")
	request := &GetDomainAuditLogsRequest{
		DomainID: "test-domain",
	}

	// first call at T1
	mockStore.EXPECT().GetDomainAuditLogs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *GetDomainAuditLogsRequest) (*InternalGetDomainAuditLogsResponse, error) {
			assert.Equal(t, t1, *req.MaxCreatedTime)
			return &InternalGetDomainAuditLogsResponse{}, nil
		}).Times(1)

	_, _ = m.GetDomainAuditLogs(context.Background(), request)

	mockTime.Advance(2 * time.Minute)
	t2 := t1.Add(2 * time.Minute)

	// second call at T2 (like in retry loop)
	mockStore.EXPECT().GetDomainAuditLogs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *GetDomainAuditLogsRequest) (*InternalGetDomainAuditLogsResponse, error) {
			if *req.MaxCreatedTime == t1 {
				t.Errorf("MaxCreatedTime is still T1 (%v) but should be new Time.Now T2 (%v)", t1, t2)
			}
			return &InternalGetDomainAuditLogsResponse{}, nil
		}).Times(1)

	_, _ = m.GetDomainAuditLogs(context.Background(), request)
}

func TestCreateDomainAuditLog_NilStateBeforeSerializesToEmptyBlob(t *testing.T) {
	// RED: This test verifies that when StateBefore is nil (e.g., CREATE operation),
	// the manager serializes an empty GetDomainResponse instead of leaving it as nil.
	// This allows us to keep NOT NULL database columns.

	m, mockStore := setUpMocksForDomainAuditManager(t)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createdTime := time.Now()

	stateAfter := &GetDomainResponse{
		Info: &DomainInfo{
			ID:   "domain-123",
			Name: "test-domain",
		},
	}

	// Mock expectation: StateBefore should NOT be nil, it should be a serialized empty response
	mockStore.EXPECT().CreateDomainAuditLog(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *InternalCreateDomainAuditLogRequest) (*CreateDomainAuditLogResponse, error) {
			// StateBefore should be a non-nil DataBlob with serialized empty GetDomainResponse
			assert.NotNil(t, req.StateBefore, "StateBefore should be serialized, not nil")
			assert.NotNil(t, req.StateBefore.Data, "StateBefore.Data should contain serialized bytes")
			assert.NotEmpty(t, req.StateBefore.Data, "StateBefore.Data should not be empty")

			// StateAfter should be properly serialized
			assert.NotNil(t, req.StateAfter, "StateAfter should be serialized")
			assert.NotNil(t, req.StateAfter.Data, "StateAfter.Data should contain serialized bytes")

			return &CreateDomainAuditLogResponse{EventID: eventID}, nil
		}).Times(1)

	// Call with nil StateBefore (typical for CREATE operations)
	resp, err := m.CreateDomainAuditLog(ctx, &CreateDomainAuditLogRequest{
		DomainID:      "domain-123",
		EventID:       eventID,
		CreatedTime:   createdTime,
		StateBefore:   nil, // nil for CREATE
		StateAfter:    stateAfter,
		OperationType: DomainAuditOperationTypeCreate,
		Identity:      "test-user",
		IdentityType:  "user",
		Comment:       "domain created",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCreateDomainAuditLog_NilStateAfterSerializesToEmptyBlob(t *testing.T) {
	// RED: This test verifies that when StateAfter is nil (e.g., DELETE operation),
	// the manager serializes an empty GetDomainResponse instead of leaving it as nil.

	m, mockStore := setUpMocksForDomainAuditManager(t)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createdTime := time.Now()

	stateBefore := &GetDomainResponse{
		Info: &DomainInfo{
			ID:   "domain-123",
			Name: "test-domain",
		},
	}

	// Mock expectation: StateAfter should NOT be nil, it should be a serialized empty response
	mockStore.EXPECT().CreateDomainAuditLog(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *InternalCreateDomainAuditLogRequest) (*CreateDomainAuditLogResponse, error) {
			// StateBefore should be properly serialized
			assert.NotNil(t, req.StateBefore, "StateBefore should be serialized")
			assert.NotNil(t, req.StateBefore.Data, "StateBefore.Data should contain serialized bytes")

			// StateAfter should be a non-nil DataBlob with serialized empty GetDomainResponse
			assert.NotNil(t, req.StateAfter, "StateAfter should be serialized, not nil")
			assert.NotNil(t, req.StateAfter.Data, "StateAfter.Data should contain serialized bytes")
			assert.NotEmpty(t, req.StateAfter.Data, "StateAfter.Data should not be empty")

			return &CreateDomainAuditLogResponse{EventID: eventID}, nil
		}).Times(1)

	// Call with nil StateAfter (typical for DELETE operations)
	resp, err := m.CreateDomainAuditLog(ctx, &CreateDomainAuditLogRequest{
		DomainID:      "domain-123",
		EventID:       eventID,
		CreatedTime:   createdTime,
		StateBefore:   stateBefore,
		StateAfter:    nil, // nil for DELETE
		OperationType: DomainAuditOperationTypeDelete,
		Identity:      "test-user",
		IdentityType:  "user",
		Comment:       "domain deleted",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetDomainAuditLogs_EmptySerializedResponseReturnsNil(t *testing.T) {
	// RED: This test verifies that when retrieving audit logs, an empty serialized
	// GetDomainResponse is converted back to nil for the API layer.
	// This maintains the API contract while allowing NOT NULL database columns.

	m, mockStore := setUpMocksForDomainAuditManager(t)
	ctx := context.Background()

	// Serialize an empty GetDomainResponse (what we store for nil states)
	emptyResponse := &GetDomainResponse{}
	emptyBlob, err := serializeGetDomainResponse(emptyResponse, constants.EncodingTypeThriftRWSnappy)
	assert.NoError(t, err)
	assert.NotNil(t, emptyBlob)

	// Serialize a real domain state for StateAfter
	realResponse := &GetDomainResponse{
		Info: &DomainInfo{
			ID:   "domain-123",
			Name: "test-domain",
		},
	}
	realBlob, err := serializeGetDomainResponse(realResponse, constants.EncodingTypeThriftRWSnappy)
	assert.NoError(t, err)
	assert.NotNil(t, realBlob)

	// Mock store returns logs with serialized empty responses
	mockStore.EXPECT().GetDomainAuditLogs(ctx, gomock.Any()).
		Return(&InternalGetDomainAuditLogsResponse{
			AuditLogs: []*InternalDomainAuditLog{
				{
					EventID:       "event-1",
					DomainID:      "domain-123",
					StateBefore:   emptyBlob,  // Empty serialized response
					StateAfter:    realBlob,   // Real serialized response
					OperationType: DomainAuditOperationTypeCreate,
					CreatedTime:   time.Now(),
				},
			},
		}, nil).Times(1)

	resp, err := m.GetDomainAuditLogs(ctx, &GetDomainAuditLogsRequest{
		DomainID: "domain-123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.AuditLogs, 1)

	// The empty serialized response should be converted back to nil for the API
	assert.Nil(t, resp.AuditLogs[0].StateBefore, "Empty serialized response should be returned as nil")
	assert.NotNil(t, resp.AuditLogs[0].StateAfter, "Real state should be deserialized")
}

func TestSerializeDeserializeEmptyGetDomainResponse(t *testing.T) {
	// RED: This test verifies the round-trip serialization of an empty GetDomainResponse

	// Serialize an empty response
	emptyResponse := &GetDomainResponse{}
	blob, err := serializeGetDomainResponse(emptyResponse, constants.EncodingTypeThriftRWSnappy)

	assert.NoError(t, err)
	assert.NotNil(t, blob, "Serialized blob should not be nil")
	assert.NotNil(t, blob.Data, "Serialized data should not be nil")
	assert.NotEmpty(t, blob.Data, "Serialized data should contain bytes")

	// Deserialize it back
	deserialized, err := deserializeGetDomainResponse(blob)
	assert.NoError(t, err)
	assert.NotNil(t, deserialized, "Deserialized response should not be nil")

	// Verify it's empty (all fields nil)
	assert.Nil(t, deserialized.Info)
	assert.Nil(t, deserialized.Config)
	assert.Nil(t, deserialized.ReplicationConfig)
}

func TestCreateDomainAuditLog_EndToEnd_WithNOTNULLColumns(t *testing.T) {
	// This test demonstrates the end-to-end flow that would fail with NOT NULL columns
	// BEFORE our fix, but works correctly AFTER the fix.
	// It simulates a CREATE operation (nil StateBefore) -> manager serializes empty response
	// -> store receives non-nil blob -> INSERT succeeds with NOT NULL columns

	m, mockStore := setUpMocksForDomainAuditManager(t)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createdTime := time.Now()

	// Verify the manager sends non-nil, non-empty DataBlobs to the store
	mockStore.EXPECT().CreateDomainAuditLog(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *InternalCreateDomainAuditLogRequest) (*CreateDomainAuditLogResponse, error) {
			// With our fix, these are NEVER nil, even for CREATE operations
			assert.NotNil(t, req.StateBefore)
			assert.NotNil(t, req.StateBefore.Data)
			assert.NotEmpty(t, req.StateBefore.Data)
			assert.Equal(t, constants.EncodingTypeThriftRWSnappy, req.StateBefore.Encoding)

			assert.NotNil(t, req.StateAfter)
			assert.NotNil(t, req.StateAfter.Data)
			assert.NotEmpty(t, req.StateAfter.Data)
			assert.Equal(t, constants.EncodingTypeThriftRWSnappy, req.StateAfter.Encoding)

			// Verify they're serialized empty responses, not nil
			deserializedBefore, err := deserializeGetDomainResponse(req.StateBefore)
			assert.NoError(t, err)
			assert.True(t, isEmptyGetDomainResponse(deserializedBefore))

			deserializedAfter, err := deserializeGetDomainResponse(req.StateAfter)
			assert.NoError(t, err)
			assert.NotNil(t, deserializedAfter.Info)
			assert.Equal(t, "domain-123", deserializedAfter.Info.ID)

			return &CreateDomainAuditLogResponse{EventID: eventID}, nil
		}).Times(1)

	// Simulate CREATE operation with nil StateBefore
	resp, err := m.CreateDomainAuditLog(ctx, &CreateDomainAuditLogRequest{
		DomainID:    "domain-123",
		EventID:     eventID,
		CreatedTime: createdTime,
		StateBefore: nil, // This is what handler.go passes for CREATE
		StateAfter: &GetDomainResponse{
			Info: &DomainInfo{
				ID:   "domain-123",
				Name: "test-domain",
			},
		},
		OperationType: DomainAuditOperationTypeCreate,
		Identity:      "test-user",
		IdentityType:  "user",
		Comment:       "domain created",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, eventID, resp.EventID)
}
