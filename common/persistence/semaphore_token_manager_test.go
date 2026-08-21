// Copyright (c) 2025 Uber Technologies, Inc.
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
	"github.com/uber/cadence/common/log/testlogger"
)

func newTestSemaphoreTokenManager(store SemaphoreTokenStore, timeSrc clock.TimeSource, t *testing.T) *semaphoreTokenManagerImpl {
	return &semaphoreTokenManagerImpl{
		persistence: store,
		logger:      testlogger.New(t),
		timeSrc:     timeSrc,
	}
}

func TestSemaphoreTokenManagerSeedSemaphoreTokens(t *testing.T) {
	fixedTime := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		request   *SeedSemaphoreTokensRequest
		setupMock func(store *MockSemaphoreTokenStore)
		wantErr   bool
	}{
		{
			name: "success stamps updated time",
			request: &SeedSemaphoreTokensRequest{
				DomainID:      "domain-1",
				SemaphoreName: "sem-1",
				Bucket:        0,
				TokenIDs:      []int{1, 2, 3},
			},
			setupMock: func(store *MockSemaphoreTokenStore) {
				store.EXPECT().SeedSemaphoreTokens(gomock.Any(), gomock.Any(), fixedTime).Return(nil).Times(1)
			},
		},
		{
			name: "missing domain id",
			request: &SeedSemaphoreTokensRequest{
				SemaphoreName: "sem-1",
				TokenIDs:      []int{1},
			},
			setupMock: func(store *MockSemaphoreTokenStore) {},
			wantErr:   true,
		},
		{
			name: "missing semaphore name",
			request: &SeedSemaphoreTokensRequest{
				DomainID: "domain-1",
				TokenIDs: []int{1},
			},
			setupMock: func(store *MockSemaphoreTokenStore) {},
			wantErr:   true,
		},
		{
			name: "negative bucket",
			request: &SeedSemaphoreTokensRequest{
				DomainID:      "domain-1",
				SemaphoreName: "sem-1",
				Bucket:        -1,
				TokenIDs:      []int{1},
			},
			setupMock: func(store *MockSemaphoreTokenStore) {},
			wantErr:   true,
		},
		{
			name: "empty token ids",
			request: &SeedSemaphoreTokensRequest{
				DomainID:      "domain-1",
				SemaphoreName: "sem-1",
				TokenIDs:      nil,
			},
			setupMock: func(store *MockSemaphoreTokenStore) {},
			wantErr:   true,
		},
		{
			name: "non-positive token id",
			request: &SeedSemaphoreTokensRequest{
				DomainID:      "domain-1",
				SemaphoreName: "sem-1",
				TokenIDs:      []int{1, 0},
			},
			setupMock: func(store *MockSemaphoreTokenStore) {},
			wantErr:   true,
		},
		{
			name: "store error is propagated",
			request: &SeedSemaphoreTokensRequest{
				DomainID:      "domain-1",
				SemaphoreName: "sem-1",
				TokenIDs:      []int{1},
			},
			setupMock: func(store *MockSemaphoreTokenStore) {
				store.EXPECT().SeedSemaphoreTokens(gomock.Any(), gomock.Any(), fixedTime).Return(errors.New("boom")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockSemaphoreTokenStore(ctrl)
			tc.setupMock(store)

			m := newTestSemaphoreTokenManager(store, clock.NewMockedTimeSourceAt(fixedTime), t)
			err := m.SeedSemaphoreTokens(context.Background(), tc.request)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestSemaphoreTokenManagerGrantSemaphoreToken(t *testing.T) {
	fixedTime := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		request       *GrantSemaphoreTokenRequest
		setupMock     func(store *MockSemaphoreTokenStore)
		wantErr       bool
		wantOutcome   SemaphoreGrantOutcome
		wantHeldToken int
	}{
		{
			name: "applied",
			request: &GrantSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc",
			},
			setupMock: func(store *MockSemaphoreTokenStore) {
				store.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any(), fixedTime).Return(&GrantSemaphoreTokenResponse{Outcome: SemaphoreGrantApplied}, nil).Times(1)
			},
			wantOutcome: SemaphoreGrantApplied,
		},
		{
			name: "not applied - slot taken is not an error",
			request: &GrantSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc",
			},
			setupMock: func(store *MockSemaphoreTokenStore) {
				store.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any(), fixedTime).Return(&GrantSemaphoreTokenResponse{Outcome: SemaphoreGrantSlotTaken}, nil).Times(1)
			},
			wantOutcome: SemaphoreGrantSlotTaken,
		},
		{
			name: "not applied - owner already holds surfaces the held token",
			request: &GrantSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc",
			},
			setupMock: func(store *MockSemaphoreTokenStore) {
				store.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any(), fixedTime).Return(&GrantSemaphoreTokenResponse{Outcome: SemaphoreGrantAlreadyHeld, HeldToken: 7}, nil).Times(1)
			},
			wantOutcome:   SemaphoreGrantAlreadyHeld,
			wantHeldToken: 7,
		},
		{
			name: "missing token id",
			request: &GrantSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 0, OwnerID: "owner-abc",
			},
			setupMock: func(store *MockSemaphoreTokenStore) {},
			wantErr:   true,
		},
		{
			name: "missing owner id",
			request: &GrantSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "",
			},
			setupMock: func(store *MockSemaphoreTokenStore) {},
			wantErr:   true,
		},
		{
			name: "store error",
			request: &GrantSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc",
			},
			setupMock: func(store *MockSemaphoreTokenStore) {
				store.EXPECT().GrantSemaphoreToken(gomock.Any(), gomock.Any(), fixedTime).Return(nil, errors.New("boom")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockSemaphoreTokenStore(ctrl)
			tc.setupMock(store)

			m := newTestSemaphoreTokenManager(store, clock.NewMockedTimeSourceAt(fixedTime), t)
			resp, err := m.GrantSemaphoreToken(context.Background(), tc.request)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantOutcome, resp.Outcome)
			assert.Equal(t, tc.wantHeldToken, resp.HeldToken)
		})
	}
}

func TestSemaphoreTokenManagerReleaseSemaphoreToken(t *testing.T) {
	fixedTime := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		request     *ReleaseSemaphoreTokenRequest
		setupMock   func(store *MockSemaphoreTokenStore)
		wantErr     bool
		wantApplied bool
	}{
		{
			name: "applied",
			request: &ReleaseSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc",
			},
			setupMock: func(store *MockSemaphoreTokenStore) {
				store.EXPECT().ReleaseSemaphoreToken(gomock.Any(), gomock.Any(), fixedTime).Return(true, nil).Times(1)
			},
			wantApplied: true,
		},
		{
			name: "not applied is not an error",
			request: &ReleaseSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc",
			},
			setupMock: func(store *MockSemaphoreTokenStore) {
				store.EXPECT().ReleaseSemaphoreToken(gomock.Any(), gomock.Any(), fixedTime).Return(false, nil).Times(1)
			},
			wantApplied: false,
		},
		{
			name: "missing owner id",
			request: &ReleaseSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "",
			},
			setupMock: func(store *MockSemaphoreTokenStore) {},
			wantErr:   true,
		},
		{
			name: "store error",
			request: &ReleaseSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc",
			},
			setupMock: func(store *MockSemaphoreTokenStore) {
				store.EXPECT().ReleaseSemaphoreToken(gomock.Any(), gomock.Any(), fixedTime).Return(false, errors.New("boom")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockSemaphoreTokenStore(ctrl)
			tc.setupMock(store)

			m := newTestSemaphoreTokenManager(store, clock.NewMockedTimeSourceAt(fixedTime), t)
			resp, err := m.ReleaseSemaphoreToken(context.Background(), tc.request)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantApplied, resp.Applied)
		})
	}
}

func TestSemaphoreTokenManagerGetSemaphoreTokenByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTokenStore(ctrl)

	req := &GetSemaphoreTokenByIDRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5}
	want := &SemaphoreToken{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, Holder: "owner-abc"}
	store.EXPECT().GetSemaphoreTokenByID(gomock.Any(), req).Return(want, nil).Times(1)

	m := newTestSemaphoreTokenManager(store, clock.NewMockedTimeSource(), t)
	resp, err := m.GetSemaphoreTokenByID(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, want, resp.Token)
}

func TestSemaphoreTokenManagerGetSemaphoreTokenByIDValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTokenStore(ctrl)

	m := newTestSemaphoreTokenManager(store, clock.NewMockedTimeSource(), t)
	_, err := m.GetSemaphoreTokenByID(context.Background(), &GetSemaphoreTokenByIDRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 0})
	assert.Error(t, err)
}

func TestSemaphoreTokenManagerGetSemaphoreTokenByOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTokenStore(ctrl)

	req := &GetSemaphoreTokenByOwnerRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, OwnerID: "owner-abc"}
	want := &SemaphoreToken{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, OwnerID: "owner-abc", HeldToken: 5}
	store.EXPECT().GetSemaphoreTokenByOwner(gomock.Any(), req).Return(want, nil).Times(1)

	m := newTestSemaphoreTokenManager(store, clock.NewMockedTimeSource(), t)
	resp, err := m.GetSemaphoreTokenByOwner(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, want, resp.Token)
}

func TestSemaphoreTokenManagerGetSemaphoreTokenByOwnerValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTokenStore(ctrl)

	m := newTestSemaphoreTokenManager(store, clock.NewMockedTimeSource(), t)
	_, err := m.GetSemaphoreTokenByOwner(context.Background(), &GetSemaphoreTokenByOwnerRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, OwnerID: ""})
	assert.Error(t, err)
}

func TestSemaphoreTokenManagerScanSemaphoreBucket(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTokenStore(ctrl)

	req := &ScanSemaphoreBucketRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, PageSize: 10}
	want := &ScanSemaphoreBucketResponse{
		Rows:          []*SemaphoreToken{{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 1}},
		NextPageToken: []byte("token"),
	}
	store.EXPECT().ScanSemaphoreBucket(gomock.Any(), req).Return(want, nil).Times(1)

	m := newTestSemaphoreTokenManager(store, clock.NewMockedTimeSource(), t)
	resp, err := m.ScanSemaphoreBucket(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, want, resp)
}

func TestSemaphoreTokenManagerGetNameAndClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTokenStore(ctrl)

	store.EXPECT().GetName().Return("cassandra").Times(1)
	store.EXPECT().Close().Times(1)

	m := newTestSemaphoreTokenManager(store, clock.NewMockedTimeSource(), t)
	assert.Equal(t, "cassandra", m.GetName())
	m.Close()
}
