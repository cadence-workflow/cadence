// Copyright (c) 2020 Uber Technologies, Inc.
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

func TestInsertDomainAuditLog(t *testing.T) {
	now := time.Now().UTC()
	domainID := "test-domain-id"
	eventID := "test-event-id"

	tests := []struct {
		name        string
		row         *nosqlplugin.DomainAuditLogRow
		queryMockFn func(query *gocql.MockQuery)
		wantErr     bool
	}{
		{
			name: "successfully inserted",
			row: &nosqlplugin.DomainAuditLogRow{
				DomainID:            domainID,
				EventID:             eventID,
				StateBefore:         []byte("state-before"),
				StateBeforeEncoding: "json",
				StateAfter:          []byte("state-after"),
				StateAfterEncoding:  "json",
				OperationType:       persistence.DomainAuditOperationTypeFailover,
				CreatedTime:         now,
				LastUpdatedTime:     now,
				Identity:            "test-identity",
				IdentityType:        "user",
				Comment:             "test comment",
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
		},
		{
			name: "exec failed",
			row: &nosqlplugin.DomainAuditLogRow{
				DomainID:      domainID,
				EventID:       eventID,
				OperationType: persistence.DomainAuditOperationTypeFailover,
				CreatedTime:   now,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("exec failed")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			err := db.InsertDomainAuditLog(context.Background(), tc.row)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSelectDomainAuditLogs(t *testing.T) {
	domainID := "test-domain-id"
	operationType := persistence.DomainAuditOperationTypeFailover
	minTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	maxTime := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
	createdTime1 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	createdTime2 := time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		filter      *nosqlplugin.DomainAuditLogFilter
		queryMockFn func(query *gocql.MockQuery)
		iterMockFn  func(iter *gocql.MockIter)
		wantRows    []*nosqlplugin.DomainAuditLogRow
		wantToken   []byte
		wantErr     bool
	}{
		{
			name: "success with default time range and no results",
			filter: &nosqlplugin.DomainAuditLogFilter{
				DomainID:      domainID,
				OperationType: operationType,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte(nil)).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows:  []*nosqlplugin.DomainAuditLogRow{},
			wantToken: nil,
		},
		{
			name: "success with custom time range and single result",
			filter: &nosqlplugin.DomainAuditLogFilter{
				DomainID:       domainID,
				OperationType:  operationType,
				MinCreatedTime: &minTime,
				MaxCreatedTime: &maxTime,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				// First call - return true and populate fields
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).DoAndReturn(func(args ...interface{}) bool {
					*args[0].(*string) = "event-1"
					*args[1].(*string) = domainID
					*args[2].(*[]byte) = []byte("state-before-1")
					*args[3].(*string) = "json"
					*args[4].(*[]byte) = []byte("state-after-1")
					*args[5].(*string) = "json"
					*args[6].(*persistence.DomainAuditOperationType) = operationType
					*args[7].(*time.Time) = createdTime1
					*args[8].(*time.Time) = createdTime1
					*args[9].(*string) = "identity-1"
					*args[10].(*string) = "user"
					*args[11].(*string) = "comment-1"
					return true
				}).Times(1)
				// Second call - return false to end iteration
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte("next-page")).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.DomainAuditLogRow{
				{
					EventID:             "event-1",
					DomainID:            domainID,
					StateBefore:         []byte("state-before-1"),
					StateBeforeEncoding: "json",
					StateAfter:          []byte("state-after-1"),
					StateAfterEncoding:  "json",
					OperationType:       operationType,
					CreatedTime:         createdTime1,
					LastUpdatedTime:     createdTime1,
					Identity:            "identity-1",
					IdentityType:        "user",
					Comment:             "comment-1",
				},
			},
			wantToken: []byte("next-page"),
		},
		{
			name: "success with page size set",
			filter: &nosqlplugin.DomainAuditLogFilter{
				DomainID:      domainID,
				OperationType: operationType,
				PageSize:      10,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().PageSize(10).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte(nil)).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows:  []*nosqlplugin.DomainAuditLogRow{},
			wantToken: nil,
		},
		{
			name: "success with pagination token",
			filter: &nosqlplugin.DomainAuditLogFilter{
				DomainID:      domainID,
				OperationType: operationType,
				PageSize:      10,
				NextPageToken: []byte("prev-page-token"),
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().PageSize(10).Return(query).Times(1)
				query.EXPECT().PageState([]byte("prev-page-token")).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte(nil)).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows:  []*nosqlplugin.DomainAuditLogRow{},
			wantToken: nil,
		},
		{
			name: "success with page size limiting - breaks after PageSize rows",
			filter: &nosqlplugin.DomainAuditLogFilter{
				DomainID:      domainID,
				OperationType: operationType,
				PageSize:      2,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().PageSize(2).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				// First row
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).DoAndReturn(func(args ...interface{}) bool {
					*args[0].(*string) = "event-1"
					*args[1].(*string) = domainID
					*args[7].(*time.Time) = createdTime1
					*args[8].(*time.Time) = createdTime1
					return true
				}).Times(1)
				// Second row
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).DoAndReturn(func(args ...interface{}) bool {
					*args[0].(*string) = "event-2"
					*args[1].(*string) = domainID
					*args[7].(*time.Time) = createdTime2
					*args[8].(*time.Time) = createdTime2
					return true
				}).Times(1)
				// Should break after 2 rows, no third Scan call
				iter.EXPECT().PageState().Return([]byte("next-page")).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.DomainAuditLogRow{
				{
					EventID:         "event-1",
					DomainID:        domainID,
					CreatedTime:     createdTime1,
					LastUpdatedTime: createdTime1,
				},
				{
					EventID:         "event-2",
					DomainID:        domainID,
					CreatedTime:     createdTime2,
					LastUpdatedTime: createdTime2,
				},
			},
			wantToken: []byte("next-page"),
		},
		{
			name: "success with multiple results",
			filter: &nosqlplugin.DomainAuditLogFilter{
				DomainID:      domainID,
				OperationType: operationType,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				// First row
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).DoAndReturn(func(args ...interface{}) bool {
					*args[0].(*string) = "event-1"
					*args[1].(*string) = domainID
					*args[7].(*time.Time) = createdTime1
					*args[8].(*time.Time) = createdTime1
					return true
				}).Times(1)
				// Second row
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).DoAndReturn(func(args ...interface{}) bool {
					*args[0].(*string) = "event-2"
					*args[1].(*string) = domainID
					*args[7].(*time.Time) = createdTime2
					*args[8].(*time.Time) = createdTime2
					return true
				}).Times(1)
				// End iteration
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(false).Times(1)
				iter.EXPECT().PageState().Return([]byte(nil)).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.DomainAuditLogRow{
				{
					EventID:         "event-1",
					DomainID:        domainID,
					CreatedTime:     createdTime1,
					LastUpdatedTime: createdTime1,
				},
				{
					EventID:         "event-2",
					DomainID:        domainID,
					CreatedTime:     createdTime2,
					LastUpdatedTime: createdTime2,
				},
			},
			wantToken: nil,
		},
		{
			name: "error when iterator is nil",
			filter: &nosqlplugin.DomainAuditLogFilter{
				DomainID:      domainID,
				OperationType: operationType,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Iter().Return(nil).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				// No calls expected
			},
			wantErr: true,
		},
		{
			name: "error when iterator close fails",
			filter: &nosqlplugin.DomainAuditLogFilter{
				DomainID:      domainID,
				OperationType: operationType,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().Scan(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(false).Times(1)
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

			// Set up Iter() to return our mock iterator (unless the test expects nil)
			if tc.name != "error when iterator is nil" {
				query.EXPECT().Iter().Return(iter).Times(1)
			}

			tc.iterMockFn(iter)

			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			rows, token, err := db.SelectDomainAuditLogs(context.Background(), tc.filter)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, len(tc.wantRows), len(rows), "number of rows should match")

			for i, wantRow := range tc.wantRows {
				if i < len(rows) {
					assert.Equal(t, wantRow.EventID, rows[i].EventID)
					assert.Equal(t, wantRow.DomainID, rows[i].DomainID)
					assert.Equal(t, wantRow.StateBefore, rows[i].StateBefore)
					assert.Equal(t, wantRow.StateBeforeEncoding, rows[i].StateBeforeEncoding)
					assert.Equal(t, wantRow.StateAfter, rows[i].StateAfter)
					assert.Equal(t, wantRow.StateAfterEncoding, rows[i].StateAfterEncoding)
					assert.Equal(t, wantRow.OperationType, rows[i].OperationType)
					assert.Equal(t, wantRow.CreatedTime.Unix(), rows[i].CreatedTime.Unix())
					assert.Equal(t, wantRow.LastUpdatedTime.Unix(), rows[i].LastUpdatedTime.Unix())
					assert.Equal(t, wantRow.Identity, rows[i].Identity)
					assert.Equal(t, wantRow.IdentityType, rows[i].IdentityType)
					assert.Equal(t, wantRow.Comment, rows[i].Comment)
				}
			}

			assert.Equal(t, tc.wantToken, token)
		})
	}
}
