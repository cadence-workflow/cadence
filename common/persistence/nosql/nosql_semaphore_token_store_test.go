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

package nosql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func setUpMocksForSemaphoreTokenStore(t *testing.T) (*nosqlSemaphoreTokenStore, *nosqlplugin.MockDB) {
	ctrl := gomock.NewController(t)
	dbMock := nosqlplugin.NewMockDB(ctrl)

	store := &nosqlSemaphoreTokenStore{
		nosqlStore: nosqlStore{
			db: dbMock,
		},
	}

	return store, dbMock
}

func TestNoSQLSeedSemaphoreTokens(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1234567890, 0).UTC()

	tests := map[string]struct {
		setupMock func(*nosqlplugin.MockDB)
		expectErr bool
	}{
		"success maps request to rows": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				expectedRows := []*nosqlplugin.SemaphoreOwnershipRow{
					{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 1, UpdatedTime: now},
					{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 2, UpdatedTime: now},
				}
				dbMock.EXPECT().InsertSemaphoreTokens(ctx, expectedRows).Return(nil).Times(1)
			},
		},
		"error propagates": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertSemaphoreTokens(ctx, gomock.Any()).Return(errors.New("db error")).Times(1)
				expectNotACommonError(dbMock)
			},
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForSemaphoreTokenStore(t)
			tc.setupMock(dbMock)

			err := store.SeedSemaphoreTokens(ctx, &persistence.SeedSemaphoreTokensRequest{
				DomainID:      "domain-1",
				SemaphoreName: "sem-1",
				Bucket:        0,
				TokenIDs:      []int{1, 2},
			}, now)

			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestNoSQLGrantSemaphoreToken(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1234567890, 0).UTC()

	// The outcome cases below check that the store surfaces each grant outcome to its
	// caller unchanged, and that an unset or unrecognized one is rejected rather than
	// read as a decision (validateGrantOutcome).
	tests := map[string]struct {
		setupMock     func(*nosqlplugin.MockDB)
		expectErr     bool
		wantOutcome   persistence.SemaphoreGrantOutcome
		wantHeldToken int
	}{
		"Applied": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				expectedRow := &nosqlplugin.SemaphoreOwnershipRow{
					DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc", UpdatedTime: now,
				}
				dbMock.EXPECT().GrantSemaphoreToken(ctx, expectedRow).
					Return(nosqlplugin.SemaphoreGrantResult{Outcome: persistence.SemaphoreGrantApplied}, nil).Times(1)
			},
			wantOutcome: persistence.SemaphoreGrantApplied,
		},
		"SlotTaken": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().GrantSemaphoreToken(ctx, gomock.Any()).
					Return(nosqlplugin.SemaphoreGrantResult{Outcome: persistence.SemaphoreGrantSlotTaken}, nil).Times(1)
			},
			wantOutcome: persistence.SemaphoreGrantSlotTaken,
		},
		"AlreadyHeld carries the held token": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().GrantSemaphoreToken(ctx, gomock.Any()).
					Return(nosqlplugin.SemaphoreGrantResult{Outcome: persistence.SemaphoreGrantAlreadyHeld, HeldToken: 7}, nil).Times(1)
			},
			wantOutcome:   persistence.SemaphoreGrantAlreadyHeld,
			wantHeldToken: 7,
		},
		"unrecognized outcome is an error": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().GrantSemaphoreToken(ctx, gomock.Any()).
					Return(nosqlplugin.SemaphoreGrantResult{Outcome: persistence.SemaphoreGrantUnknown}, nil).Times(1)
			},
			expectErr: true,
		},
		"error propagates": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().GrantSemaphoreToken(ctx, gomock.Any()).Return(nosqlplugin.SemaphoreGrantResult{}, errors.New("db error")).Times(1)
				expectNotACommonError(dbMock)
			},
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForSemaphoreTokenStore(t)
			tc.setupMock(dbMock)

			resp, err := store.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc",
			}, now)

			if tc.expectErr {
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

func TestNoSQLReleaseSemaphoreToken(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1234567890, 0).UTC()

	tests := map[string]struct {
		setupMock   func(*nosqlplugin.MockDB)
		expectErr   bool
		wantApplied bool
	}{
		"applied passes through": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				expectedRow := &nosqlplugin.SemaphoreOwnershipRow{
					DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc", UpdatedTime: now,
				}
				dbMock.EXPECT().ReleaseSemaphoreToken(ctx, expectedRow).Return(true, nil).Times(1)
			},
			wantApplied: true,
		},
		"not applied passes through": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().ReleaseSemaphoreToken(ctx, gomock.Any()).Return(false, nil).Times(1)
			},
			wantApplied: false,
		},
		"error propagates": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().ReleaseSemaphoreToken(ctx, gomock.Any()).Return(false, errors.New("db error")).Times(1)
				expectNotACommonError(dbMock)
			},
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForSemaphoreTokenStore(t)
			tc.setupMock(dbMock)

			applied, err := store.ReleaseSemaphoreToken(ctx, &persistence.ReleaseSemaphoreTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, OwnerID: "owner-abc",
			}, now)

			if tc.expectErr {
				assert.Error(t, err)
				assert.False(t, applied)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantApplied, applied)
		})
	}
}

func TestNoSQLGetSemaphoreOwnershipByToken(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		setupMock func(*nosqlplugin.MockDB)
		expectErr bool
		expected  *persistence.SemaphoreOwnership
	}{
		"success maps row to token": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				row := &nosqlplugin.SemaphoreOwnershipRow{
					RowType: persistence.SemaphoreRowTypeToken, DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, Holder: "owner-abc",
				}
				dbMock.EXPECT().SelectSemaphoreOwnershipByToken(ctx, "domain-1", "sem-1", 0, 5).Return(row, nil).Times(1)
			},
			expected: &persistence.SemaphoreOwnership{
				RowType: persistence.SemaphoreRowTypeToken, DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, Holder: "owner-abc",
			},
		},
		"error propagates": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreOwnershipByToken(ctx, "domain-1", "sem-1", 0, 5).Return(nil, errors.New("not found")).Times(1)
				expectNotACommonError(dbMock)
			},
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForSemaphoreTokenStore(t)
			tc.setupMock(dbMock)

			got, err := store.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5,
			})

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestNoSQLGetSemaphoreOwnershipByOwner(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		setupMock func(*nosqlplugin.MockDB)
		expectErr bool
		expected  *persistence.SemaphoreOwnership
	}{
		"success maps row to token": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				row := &nosqlplugin.SemaphoreOwnershipRow{
					RowType: persistence.SemaphoreRowTypeOwner, DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, OwnerID: "owner-abc", HeldToken: 5,
				}
				dbMock.EXPECT().SelectSemaphoreOwnershipByOwner(ctx, "domain-1", "sem-1", 0, "owner-abc").Return(row, nil).Times(1)
			},
			expected: &persistence.SemaphoreOwnership{
				RowType: persistence.SemaphoreRowTypeOwner, DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, OwnerID: "owner-abc", HeldToken: 5,
			},
		},
		"error propagates": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreOwnershipByOwner(ctx, "domain-1", "sem-1", 0, "owner-abc").Return(nil, errors.New("not found")).Times(1)
				expectNotACommonError(dbMock)
			},
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForSemaphoreTokenStore(t)
			tc.setupMock(dbMock)

			got, err := store.GetSemaphoreOwnershipByOwner(ctx, &persistence.GetSemaphoreOwnershipByOwnerRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, OwnerID: "owner-abc",
			})

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestNoSQLScanSemaphoreBucket(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		setupMock     func(*nosqlplugin.MockDB)
		expectErr     bool
		expectedCount int
		validate      func(*testing.T, *persistence.ScanSemaphoreBucketResponse)
	}{
		"success maps rows and token": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				rows := []*nosqlplugin.SemaphoreOwnershipRow{
					{RowType: persistence.SemaphoreRowTypeToken, DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, TokenID: 5, Holder: "owner-abc"},
					{RowType: persistence.SemaphoreRowTypeOwner, DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 0, OwnerID: "owner-abc", HeldToken: 5},
				}
				expectedFilter := &nosqlplugin.SemaphoreOwnershipFilter{
					DomainID:      "domain-1",
					SemaphoreName: "sem-1",
					Bucket:        0,
					PageSize:      10,
					NextPageToken: []byte("cur"),
				}
				dbMock.EXPECT().SelectSemaphoreOwnershipsByBucket(ctx, expectedFilter).
					Return(rows, []byte("next"), nil).Times(1)
			},
			expectedCount: 2,
			validate: func(t *testing.T, resp *persistence.ScanSemaphoreBucketResponse) {
				assert.Equal(t, persistence.SemaphoreRowTypeToken, resp.Ownerships[0].RowType)
				assert.Equal(t, 5, resp.Ownerships[0].TokenID)
				assert.Equal(t, persistence.SemaphoreRowTypeOwner, resp.Ownerships[1].RowType)
				assert.Equal(t, "owner-abc", resp.Ownerships[1].OwnerID)
				assert.Equal(t, []byte("next"), resp.NextPageToken)
			},
		},
		"empty result returns empty slice": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreOwnershipsByBucket(ctx, gomock.Any()).
					Return(nil, nil, nil).Times(1)
			},
			expectedCount: 0,
			validate: func(t *testing.T, resp *persistence.ScanSemaphoreBucketResponse) {
				assert.NotNil(t, resp.Ownerships)
				assert.Empty(t, resp.Ownerships)
				assert.Nil(t, resp.NextPageToken)
			},
		},
		"error propagates": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreOwnershipsByBucket(ctx, gomock.Any()).
					Return(nil, nil, errors.New("db error")).Times(1)
				expectNotACommonError(dbMock)
			},
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForSemaphoreTokenStore(t)
			tc.setupMock(dbMock)

			resp, err := store.ScanSemaphoreBucket(ctx, &persistence.ScanSemaphoreBucketRequest{
				DomainID:      "domain-1",
				SemaphoreName: "sem-1",
				Bucket:        0,
				PageSize:      10,
				NextPageToken: []byte("cur"),
			})

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			require.NotNil(t, resp)
			assert.Len(t, resp.Ownerships, tc.expectedCount)
			if tc.validate != nil {
				tc.validate(t, resp)
			}
		})
	}
}
