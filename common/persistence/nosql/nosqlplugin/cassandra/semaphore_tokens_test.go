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
	"github.com/uber/cadence/common/semaphore"
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

// TestSemaphoreSentinelsMatchTheOwnerIDEncoding checks each literal is still what the encoder
// writes. It catches a typo the first time it runs, and a changed owner_id encoding after that.
//
// Do not fix a failure by updating the literal. These bytes are in the primary key of every
// token row already seeded, so the stored rows have to be migrated first.
func TestSemaphoreSentinelsMatchTheOwnerIDEncoding(t *testing.T) {
	tests := []struct {
		name     string
		sentinel string
		holdID   int64
	}{
		{name: "owner-none sentinel is the encoded owner id with hold id -1", sentinel: ownerNoneSentinel, holdID: -1},
		{name: "free sentinel is the encoded owner id with hold id -2", sentinel: freeSentinel, holdID: -2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := semaphore.Owner{WorkflowID: "", RunID: emptyRunID, HoldID: tc.holdID}
			assert.Equal(t, want.String(), tc.sentinel)
		})
	}
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

	t.Run("applied: sends the token row update and the owner row insert in one batch", func(t *testing.T) {
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
				`AND type = 1 AND token_id = 5 AND owner_id = ` + ownerNoneSentinel + ` IF holder IN (` + freeSentinel + `, null)`,

			`INSERT INTO semaphore_tokens (domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
				`VALUES(10000000-1000-f000-f000-000000000000, sem-1, 0, 2, -1, owner-abc, {}, 5, ` + now.UTC().Format(time.RFC3339) + `) IF NOT EXISTS`,
		}, session.batches[0].queries)
		assert.True(t, session.iter.closed)
	})

	t.Run("refused, another owner holds the token: slot taken", func(t *testing.T) {
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

	t.Run("refused, owner row is the first returned row: already held, with that token", func(t *testing.T) {
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

	t.Run("refused, owner row comes after the token row: already held, with that token", func(t *testing.T) {
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

	t.Run("refused, owner rows without a valid held token come first: skipped, the valid owner row gives the token", func(t *testing.T) {
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

	t.Run("refused, only owner rows without a valid held token: slot taken", func(t *testing.T) {
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

	t.Run("batch fails: returns the error and no outcome", func(t *testing.T) {
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

	t.Run("applied: sends the token row update and the owner row delete in one batch", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASApplied: true, iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		applied, err := db.ReleaseSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.True(t, applied)
		assert.Len(t, session.batches, 1)
		assert.Equal(t, []string{
			`UPDATE semaphore_tokens SET holder = ` + freeSentinel + `, updated_time = ` + now.UTC().Format(time.RFC3339) + ` ` +
				`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 AND bucket = 0 ` +
				`AND type = 1 AND token_id = 5 AND owner_id = ` + ownerNoneSentinel + ` IF holder = owner-abc`,
			`DELETE FROM semaphore_tokens ` +
				`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 AND bucket = 0 ` +
				`AND type = 2 AND token_id = -1 AND owner_id = owner-abc`,
		}, session.batches[0].queries)
		assert.True(t, session.iter.closed)
	})

	t.Run("refused, owner no longer holds the token: not applied, no error", func(t *testing.T) {
		session := &fakeSession{mapExecuteBatchCASApplied: false, iter: &fakeIter{}}
		db := newTestSemaphoreTokenDB(t, session)
		applied, err := db.ReleaseSemaphoreToken(context.Background(), row)
		assert.NoError(t, err)
		assert.False(t, applied)
	})

	t.Run("batch fails: returns the error", func(t *testing.T) {
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
			name: "held token row: owner_id sentinel reads back as empty",
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
			name: "free token row: FREE holder reads back as empty",
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
			name: "read fails: returns the error",
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
			name: "owner row: token_id sentinel reads back as 0",
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
			name: "read fails: returns the error",
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
			name:   "token and owner rows: both returned, sentinels read back as zero values",
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
			name:   "page size set: stops at the page size and returns the next page token",
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
			name:    "query gives no iterator: returns an error",
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
			name:   "iterator close fails: returns the error",
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
