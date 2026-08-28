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
	rows := []*nosqlplugin.SemaphoreOwnershipRow{
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
				`VALUES(10000000-1000-f000-f000-000000000000, sem-1, 0, 1, 1, __NONE__, __FREE__, {}, ` + now.UTC().Format(time.RFC3339) + `) IF NOT EXISTS`,
			`INSERT INTO semaphore_tokens (domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
				`VALUES(10000000-1000-f000-f000-000000000000, sem-1, 0, 1, 2, __NONE__, __FREE__, {}, ` + now.UTC().Format(time.RFC3339) + `) IF NOT EXISTS`,
		}, session.batches[0].queries)
		assert.True(t, session.iter.closed)
	})

	t.Run("re-seeding an already seeded bucket is a no-op, not an error", func(t *testing.T) {
		// Every IF NOT EXISTS fails, so nothing is written; ignoring the applied flag is
		// deliberate. The held slot in `previous` shows a re-seed cannot clobber a holder.
		session := &fakeSession{
			mapExecuteBatchCASApplied: false,
			mapExecuteBatchCASPrev: map[string]any{
				"type":     int(persistence.SemaphoreRowTypeToken),
				"token_id": 1,
				"holder":   "owner-abc",
			},
			iter: &fakeIter{},
		}
		db := newTestSemaphoreTokenDB(t, session)

		assert.NoError(t, db.InsertSemaphoreTokens(context.Background(), rows))
		assert.Len(t, session.batches, 1)
		assert.True(t, session.iter.closed)
	})

	t.Run("batch error", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASErr: errors.New("boom"), iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		assert.Error(t, db.InsertSemaphoreTokens(context.Background(), rows))
		// The iterator must still be released when the batch fails.
		assert.True(t, session.iter.closed)
	})
}

func TestGrantSemaphoreToken(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	row := &nosqlplugin.SemaphoreOwnershipRow{
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
		result, err := db.GrantSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.Equal(t, persistence.SemaphoreGrantApplied, result.Outcome)
		assert.Zero(t, result.HeldToken)
		assert.Len(t, session.batches, 1)
		assert.Equal(t, []string{
			`UPDATE semaphore_tokens SET holder = owner-abc, updated_time = ` + now.UTC().Format(time.RFC3339) + ` ` +
				`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 AND bucket = 0 ` +
				`AND type = 1 AND token_id = 5 AND owner_id = __NONE__ IF holder = __FREE__`,
			`INSERT INTO semaphore_tokens (domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
				`VALUES(10000000-1000-f000-f000-000000000000, sem-1, 0, 2, -1, owner-abc, {}, 5, ` + now.UTC().Format(time.RFC3339) + `) IF NOT EXISTS`,
		}, session.batches[0].queries)
		assert.True(t, session.iter.closed)
	})

	t.Run("not applied - slot taken by someone else", func(t *testing.T) {
		// The conflicting row is the token row (someone else holds it); no owner
		// row is returned, so the outcome is SlotTaken (retry another slot).
		session := &fakeSession{
			mapExecuteBatchCASApplied: false,
			mapExecuteBatchCASPrev: map[string]any{
				"type":   int(persistence.SemaphoreRowTypeToken),
				"holder": "owner-xyz",
			},
			iter: &fakeIter{},
		}
		db := newTestSemaphoreTokenDB(t, session)
		result, err := db.GrantSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.Equal(t, persistence.SemaphoreGrantSlotTaken, result.Outcome)
		assert.Zero(t, result.HeldToken)
		assert.True(t, session.iter.closed)
	})

	t.Run("not applied - owner already holds a token (previous row)", func(t *testing.T) {
		// The owner row is the first conflicting row, returned in `previous`.
		session := &fakeSession{
			mapExecuteBatchCASApplied: false,
			mapExecuteBatchCASPrev: map[string]any{
				"type":       int(persistence.SemaphoreRowTypeOwner),
				"held_token": 7,
			},
			iter: &fakeIter{},
		}
		db := newTestSemaphoreTokenDB(t, session)
		result, err := db.GrantSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.Equal(t, persistence.SemaphoreGrantAlreadyHeld, result.Outcome)
		assert.Equal(t, 7, result.HeldToken)
		assert.True(t, session.iter.closed)
	})

	t.Run("not applied - owner already holds a token (iterator row)", func(t *testing.T) {
		// The token conflict comes back first in `previous`; the owner row is
		// returned through the iterator and must still be found.
		session := &fakeSession{
			mapExecuteBatchCASApplied: false,
			mapExecuteBatchCASPrev: map[string]any{
				"type":   int(persistence.SemaphoreRowTypeToken),
				"holder": "owner-abc",
			},
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{"type": int(persistence.SemaphoreRowTypeOwner), "held_token": 9},
				},
			},
		}
		db := newTestSemaphoreTokenDB(t, session)
		result, err := db.GrantSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.Equal(t, persistence.SemaphoreGrantAlreadyHeld, result.Outcome)
		assert.Equal(t, 9, result.HeldToken)
		assert.True(t, session.iter.closed)
	})

	t.Run("not applied - owner row with an unusable held_token is skipped", func(t *testing.T) {
		// A malformed owner row must neither be reported as a hold nor stop the search:
		// the well-formed owner row behind it is the one that carries the answer.
		session := &fakeSession{
			mapExecuteBatchCASApplied: false,
			mapExecuteBatchCASPrev: map[string]any{
				"type": int(persistence.SemaphoreRowTypeOwner), // held_token missing entirely
			},
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{"type": int(persistence.SemaphoreRowTypeOwner), "held_token": 0}, // present but not a slot id
					{"type": int(persistence.SemaphoreRowTypeOwner), "held_token": 9},
				},
			},
		}
		db := newTestSemaphoreTokenDB(t, session)
		result, err := db.GrantSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.Equal(t, persistence.SemaphoreGrantAlreadyHeld, result.Outcome)
		assert.Equal(t, 9, result.HeldToken)
		assert.True(t, session.iter.closed)
	})

	t.Run("not applied - only malformed owner rows falls back to slot taken", func(t *testing.T) {
		session := &fakeSession{
			mapExecuteBatchCASApplied: false,
			mapExecuteBatchCASPrev: map[string]any{
				"type":       int(persistence.SemaphoreRowTypeOwner),
				"held_token": -1,
			},
			iter: &fakeIter{},
		}
		db := newTestSemaphoreTokenDB(t, session)
		result, err := db.GrantSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.Equal(t, persistence.SemaphoreGrantSlotTaken, result.Outcome)
		assert.Zero(t, result.HeldToken)
		assert.True(t, session.iter.closed)
	})

	t.Run("error", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASErr: errors.New("boom"), iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		result, err := db.GrantSemaphoreToken(context.Background(), row)
		assert.Error(t, err)
		assert.Equal(t, persistence.SemaphoreGrantUnknown, result.Outcome)
	})
}

func TestReleaseSemaphoreToken(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	row := &nosqlplugin.SemaphoreOwnershipRow{
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
				`AND type = 1 AND token_id = 5 AND owner_id = __NONE__ IF holder = owner-abc`,
			`DELETE FROM semaphore_tokens ` +
				`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 AND bucket = 0 ` +
				`AND type = 2 AND token_id = -1 AND owner_id = owner-abc`,
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

func TestSelectSemaphoreOwnershipByToken(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantRow     *nosqlplugin.SemaphoreOwnershipRow
		wantErr     bool
	}{
		{
			name: "held slot normalizes sentinels",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) error {
						*args[0].(*string) = testSemaphoreDomainID
						*args[1].(*string) = testSemaphoreName
						*args[2].(*int) = 0
						*args[3].(*persistence.SemaphoreRowType) = persistence.SemaphoreRowTypeToken
						*args[4].(*int) = 5
						*args[5].(*string) = ownerNoneSentinel // token row owner_id key
						*args[6].(*string) = "owner-abc"       // holder
						*args[7].(*int) = 0                    // held_token unset on token rows -> reads as 0
						*args[8].(*time.Time) = now
						return nil
					}).Times(1)
			},
			wantRow: &nosqlplugin.SemaphoreOwnershipRow{
				RowType:       persistence.SemaphoreRowTypeToken,
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
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) error {
						*args[0].(*string) = testSemaphoreDomainID
						*args[1].(*string) = testSemaphoreName
						*args[2].(*int) = 0
						*args[3].(*persistence.SemaphoreRowType) = persistence.SemaphoreRowTypeToken
						*args[4].(*int) = 5
						*args[5].(*string) = ownerNoneSentinel
						*args[6].(*string) = freeSentinel
						*args[7].(*int) = 0
						*args[8].(*time.Time) = now
						return nil
					}).Times(1)
			},
			wantRow: &nosqlplugin.SemaphoreOwnershipRow{
				RowType:       persistence.SemaphoreRowTypeToken,
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
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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

			row, err := db.SelectSemaphoreOwnershipByToken(context.Background(), testSemaphoreDomainID, testSemaphoreName, 0, 5)
			// The selected columns must match scanSemaphoreOwnershipRow one for one,
			// `type` included: gocql rejects a count mismatch and matches the rest by
			// position.
			assert.Equal(t, []string{
				`SELECT domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time ` +
					`FROM semaphore_tokens WHERE domain_id = 10000000-1000-f000-f000-000000000000 ` +
					`AND semaphore_name = sem-1 AND bucket = 0 AND type = 1 AND token_id = 5`,
			}, session.queries)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRow, row)
		})
	}
}

func TestSelectSemaphoreOwnershipByOwner(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantRow     *nosqlplugin.SemaphoreOwnershipRow
		wantErr     bool
	}{
		{
			name: "found normalizes sentinels",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) error {
						*args[0].(*string) = testSemaphoreDomainID
						*args[1].(*string) = testSemaphoreName
						*args[2].(*int) = 0
						*args[3].(*persistence.SemaphoreRowType) = persistence.SemaphoreRowTypeOwner
						*args[4].(*int) = emptyTokenID   // token_id N/A on owner row
						*args[5].(*string) = "owner-abc" // owner_id
						*args[6].(*string) = ""          // holder unset on owner rows -> reads as ""
						*args[7].(*int) = 5              // held_token
						*args[8].(*time.Time) = now
						return nil
					}).Times(1)
			},
			wantRow: &nosqlplugin.SemaphoreOwnershipRow{
				RowType:       persistence.SemaphoreRowTypeOwner,
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
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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

			row, err := db.SelectSemaphoreOwnershipByOwner(context.Background(), testSemaphoreDomainID, testSemaphoreName, 0, "owner-abc")
			assert.Equal(t, []string{
				`SELECT domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time ` +
					`FROM semaphore_tokens WHERE domain_id = 10000000-1000-f000-f000-000000000000 ` +
					`AND semaphore_name = sem-1 AND bucket = 0 AND type = 2 AND token_id = -1 AND owner_id = owner-abc`,
			}, session.queries)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRow, row)
		})
	}
}

func TestSelectSemaphoreOwnershipsByBucket(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		filter      *nosqlplugin.SemaphoreOwnershipFilter
		queryMockFn func(query *gocql.MockQuery)
		iterMockFn  func(iter *gocql.MockIter)
		nilIter     bool
		wantRows    []*nosqlplugin.SemaphoreOwnershipRow
		wantToken   []byte
		wantErr     bool
	}{
		{
			name:   "mixed rows normalized",
			filter: &nosqlplugin.SemaphoreOwnershipFilter{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0},
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
						*args[3].(*persistence.SemaphoreRowType) = persistence.SemaphoreRowTypeToken
						*args[4].(*int) = 5
						*args[5].(*string) = ownerNoneSentinel
						*args[6].(*string) = "owner-abc"
						*args[7].(*int) = 0
						*args[8].(*time.Time) = now
						return true
					}).Times(1)
				// the matching owner row
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) bool {
						*args[0].(*string) = testSemaphoreDomainID
						*args[1].(*string) = testSemaphoreName
						*args[2].(*int) = 0
						*args[3].(*persistence.SemaphoreRowType) = persistence.SemaphoreRowTypeOwner
						*args[4].(*int) = emptyTokenID
						*args[5].(*string) = "owner-abc"
						*args[6].(*string) = ""
						*args[7].(*int) = 5
						*args[8].(*time.Time) = now
						return true
					}).Times(1)
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte(nil)).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.SemaphoreOwnershipRow{
				{RowType: persistence.SemaphoreRowTypeToken, DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0, TokenID: 5, OwnerID: "", Holder: "owner-abc", HeldToken: 0, UpdatedTime: now},
				{RowType: persistence.SemaphoreRowTypeOwner, DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0, TokenID: 0, OwnerID: "owner-abc", Holder: "", HeldToken: 5, UpdatedTime: now},
			},
			wantToken: nil,
		},
		{
			name:   "page size limits and returns token",
			filter: &nosqlplugin.SemaphoreOwnershipFilter{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0, PageSize: 1},
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
						*args[3].(*persistence.SemaphoreRowType) = persistence.SemaphoreRowTypeToken
						*args[4].(*int) = 5
						*args[5].(*string) = ownerNoneSentinel
						*args[6].(*string) = freeSentinel
						*args[7].(*int) = 0
						*args[8].(*time.Time) = now
						return true
					}).Times(1)
				iter.EXPECT().PageState().Return([]byte("next")).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.SemaphoreOwnershipRow{
				{RowType: persistence.SemaphoreRowTypeToken, DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0, TokenID: 5, OwnerID: "", Holder: "", HeldToken: 0, UpdatedTime: now},
			},
			wantToken: []byte("next"),
		},
		{
			name:    "iterator is nil",
			filter:  &nosqlplugin.SemaphoreOwnershipFilter{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0},
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
			filter: &nosqlplugin.SemaphoreOwnershipFilter{DomainID: testSemaphoreDomainID, SemaphoreName: testSemaphoreName, Bucket: 0},
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

			rows, token, err := db.SelectSemaphoreOwnershipsByBucket(context.Background(), tc.filter)
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
