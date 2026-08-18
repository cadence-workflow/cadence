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

package cassandra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

const (
	testSemaphoreDomainID = "10000000-1000-f000-f000-000000000000"
	testSemaphoreName     = "sem-1"
)

func newTestSemaphoreTokenDB(t *testing.T, session gocql.Session) *CDB {
	ctrl := gomock.NewController(t)
	client := gocql.NewMockClient(ctrl)
	cfg := &config.NoSQL{}
	logger := testlogger.New(t)
	dc := &persistence.DynamicConfiguration{}
	return NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))
}

func TestInsertSemaphoreTokens(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	rows := []*nosqlplugin.SemaphoreTokenRow{
		{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0, TokenID: 1, UpdatedTime: now},
		{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0, TokenID: 2, UpdatedTime: now},
	}

	t.Run("empty rows is a no-op", func(t *testing.T) {
		session := &fakeSession{iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		assert.NoError(t, db.InsertSemaphoreTokens(context.Background(), nil))
		assert.Empty(t, session.batches)
	})

	t.Run("seeds free rows", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASApplied: true, iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		err := db.InsertSemaphoreTokens(context.Background(), rows)
		assert.NoError(t, err)
		assert.Len(t, session.batches, 1)
		assert.Equal(t, []string{
			`INSERT INTO semaphore_tokens (domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
				`VALUES(10000000-1000-f000-f000-000000000000, sem-1, 0, 0, 1, __NONE__, __FREE__, -1, ` + now.UTC().Format(time.RFC3339) + `) IF NOT EXISTS`,
			`INSERT INTO semaphore_tokens (domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
				`VALUES(10000000-1000-f000-f000-000000000000, sem-1, 0, 0, 2, __NONE__, __FREE__, -1, ` + now.UTC().Format(time.RFC3339) + `) IF NOT EXISTS`,
		}, session.batches[0].queries)
		assert.True(t, session.iter.closed)
	})

	t.Run("batch error", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASErr: errors.New("boom"), iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		assert.Error(t, db.InsertSemaphoreTokens(context.Background(), rows))
	})
}

func TestGrantSemaphoreToken(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	row := &nosqlplugin.SemaphoreTokenRow{
		DomainID:      testSemaphoreDomainID,
		SemaphoreName: testSemaphoreName,
		Bucket:        0,
		TokenID:       5,
		OwnerID:       "owner-abc",
		UpdatedTime:   now,
	}

	t.Run("applied", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASApplied: true, iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		applied, err := db.GrantSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.True(t, applied)
		assert.Len(t, session.batches, 1)
		assert.Equal(t, []string{
			`UPDATE semaphore_tokens SET holder = owner-abc, updated_time = ` + now.UTC().Format(time.RFC3339) + ` ` +
				`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 AND bucket = 0 ` +
				`AND type = 0 AND token_id = 5 AND owner_id = __NONE__ IF holder = __FREE__`,
			`INSERT INTO semaphore_tokens (domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
				`VALUES(10000000-1000-f000-f000-000000000000, sem-1, 0, 1, -1, owner-abc, __NONE__, 5, ` + now.UTC().Format(time.RFC3339) + `)`,
		}, session.batches[0].queries)
		assert.True(t, session.iter.closed)
	})

	t.Run("not applied", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASApplied: false, iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		applied, err := db.GrantSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.False(t, applied)
	})

	t.Run("error", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASErr: errors.New("boom"), iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		applied, err := db.GrantSemaphoreToken(context.Background(), row)
		assert.Error(t, err)
		assert.False(t, applied)
	})
}

func TestReleaseSemaphoreToken(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	row := &nosqlplugin.SemaphoreTokenRow{
		DomainID:      testSemaphoreDomainID,
		SemaphoreName: testSemaphoreName,
		Bucket:        0,
		TokenID:       5,
		OwnerID:       "owner-abc",
		UpdatedTime:   now,
	}

	t.Run("applied", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASApplied: true, iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		applied, err := db.ReleaseSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.True(t, applied)
		assert.Len(t, session.batches, 1)
		assert.Equal(t, []string{
			`UPDATE semaphore_tokens SET holder = __FREE__, updated_time = ` + now.UTC().Format(time.RFC3339) + ` ` +
				`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 AND bucket = 0 ` +
				`AND type = 0 AND token_id = 5 AND owner_id = __NONE__ IF holder = owner-abc`,
			`DELETE FROM semaphore_tokens ` +
				`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 AND bucket = 0 ` +
				`AND type = 1 AND token_id = -1 AND owner_id = owner-abc`,
		}, session.batches[0].queries)
		assert.True(t, session.iter.closed)
	})

	t.Run("not applied", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASApplied: false, iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		applied, err := db.ReleaseSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.False(t, applied)
	})

	t.Run("error", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASErr: errors.New("boom"), iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		applied, err := db.ReleaseSemaphoreToken(context.Background(), row)
		assert.Error(t, err)
		assert.False(t, applied)
	})
}

func TestSelectSemaphoreTokenByID(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantRow     *nosqlplugin.SemaphoreTokenRow
		wantErr     bool
	}{
		{
			name: "held slot normalizes sentinels",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) error {
						*args[0].(*string) = testSemaphoreDomainID
						*args[1].(*string) = testSemaphoreName
						*args[2].(*int) = 0
						*args[3].(*int) = 5
						*args[4].(*string) = ownerNoneSentinel // token row owner_id key
						*args[5].(*string) = "owner-abc"       // holder
						*args[6].(*int) = emptyHeldToken       // held_token N/A on token row
						*args[7].(*time.Time) = now
						return nil
					}).Times(1)
			},
			wantRow: &nosqlplugin.SemaphoreTokenRow{
				DomainID:      testSemaphoreDomainID,
				SemaphoreName: testSemaphoreName,
				Bucket:        0,
				TokenID:       5,
				OwnerID:       "",
				Holder:        "owner-abc",
				HeldToken:     0,
				UpdatedTime:   now,
			},
		},
		{
			name: "free slot normalizes holder",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) error {
						*args[0].(*string) = testSemaphoreDomainID
						*args[1].(*string) = testSemaphoreName
						*args[2].(*int) = 0
						*args[3].(*int) = 5
						*args[4].(*string) = ownerNoneSentinel
						*args[5].(*string) = freeSentinel
						*args[6].(*int) = emptyHeldToken
						*args[7].(*time.Time) = now
						return nil
					}).Times(1)
			},
			wantRow: &nosqlplugin.SemaphoreTokenRow{
				DomainID:      testSemaphoreDomainID,
				SemaphoreName: testSemaphoreName,
				Bucket:        0,
				TokenID:       5,
				OwnerID:       "",
				Holder:        "",
				HeldToken:     0,
				UpdatedTime:   now,
			},
		},
		{
			name: "not found",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("not found")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{query: query}
			db := newTestSemaphoreTokenDB(t, session)

			row, err := db.SelectSemaphoreTokenByID(context.Background(), testSemaphoreDomainID, testSemaphoreName, 0, 5)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRow, row)
		})
	}
}

func TestSelectSemaphoreTokenByOwner(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantRow     *nosqlplugin.SemaphoreTokenRow
		wantErr     bool
	}{
		{
			name: "found normalizes sentinels",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) error {
						*args[0].(*string) = testSemaphoreDomainID
						*args[1].(*string) = testSemaphoreName
						*args[2].(*int) = 0
						*args[3].(*int) = emptyTokenID         // token_id N/A on owner row
						*args[4].(*string) = "owner-abc"       // owner_id
						*args[5].(*string) = ownerNoneSentinel // holder N/A on owner row
						*args[6].(*int) = 5                    // held_token
						*args[7].(*time.Time) = now
						return nil
					}).Times(1)
			},
			wantRow: &nosqlplugin.SemaphoreTokenRow{
				DomainID:      testSemaphoreDomainID,
				SemaphoreName: testSemaphoreName,
				Bucket:        0,
				TokenID:       0,
				OwnerID:       "owner-abc",
				Holder:        "",
				HeldToken:     5,
				UpdatedTime:   now,
			},
		},
		{
			name: "not found",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("not found")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{query: query}
			db := newTestSemaphoreTokenDB(t, session)

			row, err := db.SelectSemaphoreTokenByOwner(context.Background(), testSemaphoreDomainID, testSemaphoreName, 0, "owner-abc")
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRow, row)
		})
	}
}

func TestSelectSemaphoreTokensByBucket(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		filter      *nosqlplugin.SemaphoreTokenFilter
		queryMockFn func(query *gocql.MockQuery)
		iterMockFn  func(iter *gocql.MockIter)
		nilIter     bool
		wantRows    []*nosqlplugin.SemaphoreTokenRow
		wantToken   []byte
		wantErr     bool
	}{
		{
			name:   "mixed rows normalized",
			filter: &nosqlplugin.SemaphoreTokenFilter{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				// a held token row
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) bool {
						*args[0].(*string) = testSemaphoreDomainID
						*args[1].(*string) = testSemaphoreName
						*args[2].(*int) = 0
						*args[3].(*int) = rowTypeSemaphoreToken
						*args[4].(*int) = 5
						*args[5].(*string) = ownerNoneSentinel
						*args[6].(*string) = "owner-abc"
						*args[7].(*int) = emptyHeldToken
						*args[8].(*time.Time) = now
						return true
					}).Times(1)
				// the matching owner row
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) bool {
						*args[0].(*string) = testSemaphoreDomainID
						*args[1].(*string) = testSemaphoreName
						*args[2].(*int) = 0
						*args[3].(*int) = rowTypeSemaphoreOwner
						*args[4].(*int) = emptyTokenID
						*args[5].(*string) = "owner-abc"
						*args[6].(*string) = ownerNoneSentinel
						*args[7].(*int) = 5
						*args[8].(*time.Time) = now
						return true
					}).Times(1)
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte(nil)).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.SemaphoreTokenRow{
				{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0, TokenID: 5, OwnerID: "", Holder: "owner-abc", HeldToken: 0, UpdatedTime: now},
				{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0, TokenID: 0, OwnerID: "owner-abc", Holder: "", HeldToken: 5, UpdatedTime: now},
			},
			wantToken: nil,
		},
		{
			name:   "page size limits and returns token",
			filter: &nosqlplugin.SemaphoreTokenFilter{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0, PageSize: 1},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().PageSize(1).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) bool {
						*args[0].(*string) = testSemaphoreDomainID
						*args[1].(*string) = testSemaphoreName
						*args[2].(*int) = 0
						*args[3].(*int) = rowTypeSemaphoreToken
						*args[4].(*int) = 5
						*args[5].(*string) = ownerNoneSentinel
						*args[6].(*string) = freeSentinel
						*args[7].(*int) = emptyHeldToken
						*args[8].(*time.Time) = now
						return true
					}).Times(1)
				iter.EXPECT().PageState().Return([]byte("next")).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.SemaphoreTokenRow{
				{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0, TokenID: 5, OwnerID: "", Holder: "", HeldToken: 0, UpdatedTime: now},
			},
			wantToken: []byte("next"),
		},
		{
			name:    "iterator is nil",
			filter:  &nosqlplugin.SemaphoreTokenFilter{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0},
			nilIter: true,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Iter().Return(nil).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {},
			wantErr:    true,
		},
		{
			name:   "iterator close fails",
			filter: &nosqlplugin.SemaphoreTokenFilter{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte(nil)).Times(1)
				iter.EXPECT().Close().Return(errors.New("close failed")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			iter := gocql.NewMockIter(ctrl)

			tc.queryMockFn(query)
			if !tc.nilIter {
				query.EXPECT().Iter().Return(iter).Times(1)
			}
			tc.iterMockFn(iter)

			session := &fakeSession{query: query}
			db := newTestSemaphoreTokenDB(t, session)

			rows, token, err := db.SelectSemaphoreTokensByBucket(context.Background(), tc.filter)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRows, rows)
			assert.Equal(t, tc.wantToken, token)
		})
	}
}
