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

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
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
		dc:          &DynamicConfiguration{},
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
