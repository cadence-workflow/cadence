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

func setUpMocksForSemaphoreMetadataStore(t *testing.T) (*nosqlSemaphoreMetadataStore, *nosqlplugin.MockDB) {
	ctrl := gomock.NewController(t)
	dbMock := nosqlplugin.NewMockDB(ctrl)

	store := &nosqlSemaphoreMetadataStore{
		nosqlStore: nosqlStore{
			db: dbMock,
		},
	}

	return store, dbMock
}

// expectNotACommonError makes convertCommonErrors treat the error as a generic
// (non-not-found, non-timeout, etc.) error so it maps to InternalServiceError.
func expectNotACommonError(dbMock *nosqlplugin.MockDB) {
	dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(false).AnyTimes()
	dbMock.EXPECT().IsTimeoutError(gomock.Any()).Return(false).AnyTimes()
	dbMock.EXPECT().IsThrottlingError(gomock.Any()).Return(false).AnyTimes()
	dbMock.EXPECT().IsDBUnavailableError(gomock.Any()).Return(false).AnyTimes()
}

func TestNoSQLCreateSemaphore(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1234567890, 0).UTC()

	semaphore := &persistence.SemaphoreMetadata{
		DomainID:      "domain-1",
		SemaphoreName: "sem-1",
		Size:          100,
		BucketSize:    10,
		CreatedTime:   now,
	}

	tests := map[string]struct {
		setupMock func(*nosqlplugin.MockDB)
		assertErr func(*testing.T, error)
	}{
		"success maps semaphore to row": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				expectedRow := &nosqlplugin.SemaphoreMetadataRow{
					DomainID:      "domain-1",
					SemaphoreName: "sem-1",
					Size:          100,
					BucketSize:    10,
					CreatedTime:   now,
				}
				dbMock.EXPECT().InsertSemaphoreMetadata(ctx, expectedRow).Return(nil).Times(1)
			},
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		"condition failure maps to ConditionFailedError": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertSemaphoreMetadata(ctx, gomock.Any()).
					Return(nosqlplugin.NewConditionFailure("semaphore")).Times(1)
			},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				_, ok := err.(*persistence.ConditionFailedError)
				assert.True(t, ok, "expected *persistence.ConditionFailedError, got %T", err)
			},
		},
		"generic db error maps via convertCommonErrors": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertSemaphoreMetadata(ctx, gomock.Any()).
					Return(errors.New("db error")).Times(1)
				expectNotACommonError(dbMock)
			},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				// must NOT be a ConditionFailedError
				_, ok := err.(*persistence.ConditionFailedError)
				assert.False(t, ok, "generic error should not become ConditionFailedError")
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForSemaphoreMetadataStore(t)
			tc.setupMock(dbMock)

			err := store.CreateSemaphore(ctx, semaphore)
			tc.assertErr(t, err)
		})
	}
}

func TestNoSQLGetSemaphore(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1234567890, 0).UTC()

	tests := map[string]struct {
		setupMock func(*nosqlplugin.MockDB)
		expectErr bool
		expected  *persistence.SemaphoreMetadata
	}{
		"success maps row to semaphore": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				row := &nosqlplugin.SemaphoreMetadataRow{
					DomainID:      "domain-1",
					SemaphoreName: "sem-1",
					Size:          100,
					BucketSize:    10,
					CreatedTime:   now,
				}
				dbMock.EXPECT().SelectSemaphoreMetadata(ctx, "domain-1", "sem-1").Return(row, nil).Times(1)
			},
			expected: &persistence.SemaphoreMetadata{
				DomainID:      "domain-1",
				SemaphoreName: "sem-1",
				Size:          100,
				BucketSize:    10,
				CreatedTime:   now,
			},
		},
		"error propagates": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreMetadata(ctx, "domain-1", "sem-1").Return(nil, errors.New("not found")).Times(1)
				expectNotACommonError(dbMock)
			},
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForSemaphoreMetadataStore(t)
			tc.setupMock(dbMock)

			got, err := store.GetSemaphore(ctx, &persistence.GetSemaphoreRequest{
				DomainID:      "domain-1",
				SemaphoreName: "sem-1",
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

func TestNoSQLListSemaphores(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1234567890, 0).UTC()

	tests := map[string]struct {
		setupMock     func(*nosqlplugin.MockDB)
		expectErr     bool
		expectedCount int
		validate      func(*testing.T, *persistence.ListSemaphoresResponse)
	}{
		"success maps rows and token": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				rows := []*nosqlplugin.SemaphoreMetadataRow{
					{DomainID: "domain-1", SemaphoreName: "sem-1", Size: 100, BucketSize: 10, CreatedTime: now},
					{DomainID: "domain-1", SemaphoreName: "sem-2", Size: 200, BucketSize: 20, CreatedTime: now},
				}
				expectedFilter := &nosqlplugin.SemaphoreMetadataFilter{
					DomainID:      "domain-1",
					PageSize:      10,
					NextPageToken: []byte("cur"),
				}
				dbMock.EXPECT().SelectSemaphoreMetadataByDomain(ctx, expectedFilter).
					Return(rows, []byte("next"), nil).Times(1)
			},
			expectedCount: 2,
			validate: func(t *testing.T, resp *persistence.ListSemaphoresResponse) {
				assert.Equal(t, "sem-1", resp.Semaphores[0].SemaphoreName)
				assert.Equal(t, 200, resp.Semaphores[1].Size)
				assert.Equal(t, []byte("next"), resp.NextPageToken)
			},
		},
		"empty result returns empty slice": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreMetadataByDomain(ctx, gomock.Any()).
					Return(nil, nil, nil).Times(1)
			},
			expectedCount: 0,
			validate: func(t *testing.T, resp *persistence.ListSemaphoresResponse) {
				assert.NotNil(t, resp.Semaphores)
				assert.Empty(t, resp.Semaphores)
				assert.Nil(t, resp.NextPageToken)
			},
		},
		"error propagates": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreMetadataByDomain(ctx, gomock.Any()).
					Return(nil, nil, errors.New("db error")).Times(1)
				expectNotACommonError(dbMock)
			},
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForSemaphoreMetadataStore(t)
			tc.setupMock(dbMock)

			resp, err := store.ListSemaphores(ctx, &persistence.ListSemaphoresRequest{
				DomainID:      "domain-1",
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
			assert.Len(t, resp.Semaphores, tc.expectedCount)
			if tc.validate != nil {
				tc.validate(t, resp)
			}
		})
	}
}
