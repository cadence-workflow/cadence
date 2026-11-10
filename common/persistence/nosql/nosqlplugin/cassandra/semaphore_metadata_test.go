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

func TestInsertSemaphoreMetadata(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	row := &nosqlplugin.SemaphoreMetadataRow{
		DomainID:      "10000000-1000-f000-f000-000000000000",
		SemaphoreName: "sem-1",
		Size:          100,
		BucketSize:    10,
		CreatedTime:   now,
	}

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "successfully applied",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)
			},
			wantQueries: []string{
				`INSERT INTO semaphore_metadata (` +
					`domain_id, semaphore_name, size, bucket_size, created_time) ` +
					`VALUES(10000000-1000-f000-f000-000000000000, sem-1, 100, 10, 2025-06-01T12:00:00Z) IF NOT EXISTS`,
			},
		},
		{
			name: "not applied - already exists",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return false, nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name: "mapscancas failed",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return false, errors.New("mapscancas failed")
				}).Times(1)
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
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := persistence.NewDefaultDynamicConfiguration()

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			err := db.InsertSemaphoreMetadata(context.Background(), row)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tc.wantQueries != nil {
				assert.Equal(t, tc.wantQueries, session.queries)
			}
		})
	}
}

func TestSelectSemaphoreMetadata(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	domainID := "10000000-1000-f000-f000-000000000000"
	semaphoreName := "sem-1"

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantRow     *nosqlplugin.SemaphoreMetadataRow
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "found",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) error {
						*args[0].(*string) = domainID
						*args[1].(*string) = semaphoreName
						*args[2].(*int) = 100
						*args[3].(*int) = 10
						*args[4].(*time.Time) = now
						return nil
					}).Times(1)
			},
			wantRow: &nosqlplugin.SemaphoreMetadataRow{
				DomainID:      domainID,
				SemaphoreName: semaphoreName,
				Size:          100,
				BucketSize:    10,
				CreatedTime:   now,
			},
			wantQueries: []string{
				`SELECT domain_id, semaphore_name, size, bucket_size, created_time ` +
					`FROM semaphore_metadata ` +
					`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1`,
			},
		},
		{
			name: "not found",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) error {
						return errors.New("not found")
					}).Times(1)
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
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := persistence.NewDefaultDynamicConfiguration()

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			row, err := db.SelectSemaphoreMetadata(context.Background(), domainID, semaphoreName)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRow, row)
			if tc.wantQueries != nil {
				assert.Equal(t, tc.wantQueries, session.queries)
			}
		})
	}
}

func TestSelectSemaphoreMetadataByDomain(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	domainID := "10000000-1000-f000-f000-000000000000"

	tests := []struct {
		name        string
		filter      *nosqlplugin.SemaphoreMetadataFilter
		queryMockFn func(query *gocql.MockQuery)
		iterMockFn  func(iter *gocql.MockIter)
		wantRows    []*nosqlplugin.SemaphoreMetadataRow
		wantToken   []byte
		nilIter     bool
		wantErr     bool
	}{
		{
			name:   "no results",
			filter: &nosqlplugin.SemaphoreMetadataFilter{DomainID: domainID},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte(nil)).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows:  nil,
			wantToken: nil,
		},
		{
			name:   "multiple results",
			filter: &nosqlplugin.SemaphoreMetadataFilter{DomainID: domainID},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) bool {
						*args[0].(*string) = domainID
						*args[1].(*string) = "sem-1"
						*args[2].(*int) = 100
						*args[3].(*int) = 10
						*args[4].(*time.Time) = now
						return true
					}).Times(1)
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) bool {
						*args[0].(*string) = domainID
						*args[1].(*string) = "sem-2"
						*args[2].(*int) = 200
						*args[3].(*int) = 20
						*args[4].(*time.Time) = now
						return true
					}).Times(1)
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte(nil)).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.SemaphoreMetadataRow{
				{DomainID: domainID, SemaphoreName: "sem-1", Size: 100, BucketSize: 10, CreatedTime: now},
				{DomainID: domainID, SemaphoreName: "sem-2", Size: 200, BucketSize: 20, CreatedTime: now},
			},
			wantToken: nil,
		},
		{
			name:   "page size limits and returns token",
			filter: &nosqlplugin.SemaphoreMetadataFilter{DomainID: domainID, PageSize: 2},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().PageSize(2).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) bool {
						*args[0].(*string) = domainID
						*args[1].(*string) = "sem-1"
						*args[2].(*int) = 100
						*args[3].(*int) = 10
						*args[4].(*time.Time) = now
						return true
					}).Times(1)
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) bool {
						*args[0].(*string) = domainID
						*args[1].(*string) = "sem-2"
						*args[2].(*int) = 200
						*args[3].(*int) = 20
						*args[4].(*time.Time) = now
						return true
					}).Times(1)
				// breaks after PageSize rows; no third Scan
				iter.EXPECT().PageState().Return([]byte("next-page")).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.SemaphoreMetadataRow{
				{DomainID: domainID, SemaphoreName: "sem-1", Size: 100, BucketSize: 10, CreatedTime: now},
				{DomainID: domainID, SemaphoreName: "sem-2", Size: 200, BucketSize: 20, CreatedTime: now},
			},
			wantToken: []byte("next-page"),
		},
		{
			name:   "with page token",
			filter: &nosqlplugin.SemaphoreMetadataFilter{DomainID: domainID, PageSize: 10, NextPageToken: []byte("prev-token")},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().PageSize(10).Return(query).Times(1)
				query.EXPECT().PageState([]byte("prev-token")).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte(nil)).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows:  nil,
			wantToken: nil,
		},
		{
			name:    "iterator is nil",
			filter:  &nosqlplugin.SemaphoreMetadataFilter{DomainID: domainID},
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
			filter: &nosqlplugin.SemaphoreMetadataFilter{DomainID: domainID},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := persistence.NewDefaultDynamicConfiguration()

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			rows, token, err := db.SelectSemaphoreMetadataByDomain(context.Background(), tc.filter)

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
