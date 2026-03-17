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

func TestCreateDomainAuditLog_SerializeStates(t *testing.T) {
	// This test verifies that nil StateBefore/StateAfter are serialized as empty GetDomainResponse{}
	// instead of being left as nil, allowing us to maintain NOT NULL database columns.

	domainResponse := &GetDomainResponse{
		Info: &DomainInfo{
			ID:   "domain-123",
			Name: "test-domain",
		},
	}

	tests := []struct {
		name              string
		stateBefore       *GetDomainResponse
		stateAfter        *GetDomainResponse
		operationType     DomainAuditOperationType
		expectEmptyBefore bool
		expectEmptyAfter  bool
	}{
		{
			name:              "correctly serializes nil StateBefore as an empty GetDomainResponse{}",
			stateBefore:       nil,
			stateAfter:        domainResponse,
			operationType:     DomainAuditOperationTypeCreate,
			expectEmptyBefore: true,
			expectEmptyAfter:  false,
		},
		{
			name:              "correctly serializes nil StateAfter as an empty GetDomainResponse{}",
			stateBefore:       domainResponse,
			stateAfter:        nil,
			operationType:     DomainAuditOperationTypeDelete,
			expectEmptyBefore: false,
			expectEmptyAfter:  true,
		},
		{
			name:              "correctly serializes both StateBefore and StateAfter as is",
			stateBefore:       domainResponse,
			stateAfter:        domainResponse,
			operationType:     DomainAuditOperationTypeUpdate,
			expectEmptyBefore: false,
			expectEmptyAfter:  false,
		},
		{
			name:              "correctly serializes both StateBefore and StateAfter as empty GetDomainResponse{}",
			stateBefore:       nil,
			stateAfter:        nil,
			operationType:     DomainAuditOperationTypeUpdate,
			expectEmptyBefore: true,
			expectEmptyAfter:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, mockStore := setUpMocksForDomainAuditManager(t)
			ctx := context.Background()

			eventID := uuid.Must(uuid.NewV7()).String()
			createdTime := time.Now()

			mockStore.EXPECT().CreateDomainAuditLog(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, req *InternalCreateDomainAuditLogRequest) (*CreateDomainAuditLogResponse, error) {
					// Both blobs should ALWAYS be non-nil with non-empty data
					assert.NotNil(t, req.StateBefore, "StateBefore blob should never be nil")
					assert.NotNil(t, req.StateBefore.Data, "StateBefore.Data should not be nil")
					assert.NotEmpty(t, req.StateBefore.Data, "StateBefore.Data should contain serialized bytes")

					assert.NotNil(t, req.StateAfter, "StateAfter blob should never be nil")
					assert.NotNil(t, req.StateAfter.Data, "StateAfter.Data should not be nil")
					assert.NotEmpty(t, req.StateAfter.Data, "StateAfter.Data should contain serialized bytes")

					// Verify whether they're empty or populated
					deserializedBefore, err := deserializeGetDomainResponse(req.StateBefore)
					assert.NoError(t, err)
					assert.Equal(t, tt.expectEmptyBefore, isEmptyGetDomainResponse(deserializedBefore),
						"StateBefore empty state mismatch")

					deserializedAfter, err := deserializeGetDomainResponse(req.StateAfter)
					assert.NoError(t, err)
					assert.Equal(t, tt.expectEmptyAfter, isEmptyGetDomainResponse(deserializedAfter),
						"StateAfter empty state mismatch")

					return &CreateDomainAuditLogResponse{EventID: eventID}, nil
				}).Times(1)

			resp, err := m.CreateDomainAuditLog(ctx, &CreateDomainAuditLogRequest{
				DomainID:      "domain-123",
				EventID:       eventID,
				CreatedTime:   createdTime,
				StateBefore:   tt.stateBefore,
				StateAfter:    tt.stateAfter,
				OperationType: tt.operationType,
				Identity:      "test-user",
				IdentityType:  "user",
				Comment:       "test operation",
			})

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, eventID, resp.EventID)
		})
	}
}

func TestGetDomainAuditLogs_DeserializesEmptyResponsesToNil(t *testing.T) {
	domainResponse := &GetDomainResponse{
		Info: &DomainInfo{
			ID:   "domain-123",
			Name: "test-domain",
		},
	}
	emptyResponse := &GetDomainResponse{}

	tests := []struct {
		name                string
		stateBefore         *GetDomainResponse
		stateAfter          *GetDomainResponse
		expectNilBefore     bool
		expectNilAfter      bool
		expectDomainIDAfter bool
	}{
		{
			name:                "when StateBefore is nil, it should be deserialized as an empty GetDomainResponse{}",
			stateBefore:         emptyResponse,
			stateAfter:          domainResponse,
			expectNilBefore:     true,
			expectNilAfter:      false,
			expectDomainIDAfter: true,
		},
		{
			name:                "when StateAfter is nil, it should be deserialized as an empty GetDomainResponse{}",
			stateBefore:         domainResponse,
			stateAfter:          emptyResponse,
			expectNilBefore:     false,
			expectNilAfter:      true,
			expectDomainIDAfter: false,
		},
		{
			name:                "when both StateBefore and StateAfter are populated, they should be deserialized as is",
			stateBefore:         domainResponse,
			stateAfter:          domainResponse,
			expectNilBefore:     false,
			expectNilAfter:      false,
			expectDomainIDAfter: true,
		},
		{
			name:                "when both StateBefore and StateAfter are empty, they should be deserialized as empty GetDomainResponse{}",
			stateBefore:         emptyResponse,
			stateAfter:          emptyResponse,
			expectNilBefore:     true,
			expectNilAfter:      true,
			expectDomainIDAfter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, mockStore := setUpMocksForDomainAuditManager(t)
			ctx := context.Background()

			// Serialize the test data
			var stateBeforeBlob, stateAfterBlob *DataBlob
			var err error

			stateBeforeBlob, err = serializeGetDomainResponse(tt.stateBefore, constants.EncodingTypeThriftRWSnappy)
			assert.NoError(t, err)

			stateAfterBlob, err = serializeGetDomainResponse(tt.stateAfter, constants.EncodingTypeThriftRWSnappy)
			assert.NoError(t, err)

			mockStore.EXPECT().GetDomainAuditLogs(ctx, gomock.Any()).
				Return(&InternalGetDomainAuditLogsResponse{
					AuditLogs: []*InternalDomainAuditLog{
						{
							EventID:       "event-1",
							DomainID:      "domain-123",
							StateBefore:   stateBeforeBlob,
							StateAfter:    stateAfterBlob,
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

			// Verify empty serialized responses are converted to nil
			if tt.expectNilBefore {
				assert.Nil(t, resp.AuditLogs[0].StateBefore, "Empty serialized StateBefore should return nil")
			} else {
				assert.NotNil(t, resp.AuditLogs[0].StateBefore, "Populated StateBefore should be deserialized")
			}

			if tt.expectNilAfter {
				assert.Nil(t, resp.AuditLogs[0].StateAfter, "Empty serialized StateAfter should return nil")
			} else {
				assert.NotNil(t, resp.AuditLogs[0].StateAfter, "Populated StateAfter should be deserialized")
				if tt.expectDomainIDAfter {
					assert.Equal(t, "domain-123", resp.AuditLogs[0].StateAfter.Info.ID)
				}
			}
		})
	}
}

func TestGetDomainAuditLogs_BackwardCompatibilityWithOldCassandraData(t *testing.T) {
	// Verifies that old Cassandra data with nil blobs returns nil without a deserialization error.

	m, mockStore := setUpMocksForDomainAuditManager(t)
	ctx := context.Background()

	// Serialize a real StateAfter to simulate existing Cassandra data
	stateAfterResponse := &GetDomainResponse{
		Info: &DomainInfo{
			ID:   "domain-123",
			Name: "test-domain",
		},
	}
	stateAfterBlob, err := serializeGetDomainResponse(stateAfterResponse, constants.EncodingTypeThriftRWSnappy)
	assert.NoError(t, err)

	// OLD Cassandra data: StateBefore is nil (NoSQL store returns nil when len(row.StateBefore) == 0)
	mockStore.EXPECT().GetDomainAuditLogs(ctx, gomock.Any()).
		Return(&InternalGetDomainAuditLogsResponse{
			AuditLogs: []*InternalDomainAuditLog{
				{
					EventID:       "old-event-1",
					DomainID:      "domain-123",
					StateBefore:   nil, // Old data - NULL in Cassandra, nil from NoSQL store
					StateAfter:    stateAfterBlob,
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

	// Old data with nil should still return nil to API (backward compatible)
	assert.Nil(t, resp.AuditLogs[0].StateBefore, "Old Cassandra data with nil should return nil")
	assert.NotNil(t, resp.AuditLogs[0].StateAfter, "StateAfter should be deserialized")
	assert.Equal(t, "domain-123", resp.AuditLogs[0].StateAfter.Info.ID)
}
