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

func TestInsertHistoryDLQAckLevelIfNotExistsRow(t *testing.T) {
	ackTS := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2025, 6, 1, 13, 0, 0, 0, time.UTC)

	row := &nosqlplugin.HistoryDLQAckLevelRow{
		ShardID:               7,
		DomainID:              "domain-x",
		ClusterAttributeScope: "scope-1",
		ClusterAttributeName:  "cluster-a",
		TaskCategory:          2,
		AckLevelVisibilityTS:  ackTS,
		AckLevelTaskID:        0,
		LastUpdatedAt:         updatedAt,
	}

	// The rendered CQL must carry the IF NOT EXISTS suffix: this is what makes the write a
	// no-op sentinel that never overwrites recorded progress.
	const wantQuery = `INSERT INTO history_task_dlq_ack_level (` +
		`shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
		`task_category, ack_level_visibility_ts, ack_level_task_id, last_updated_at) ` +
		`VALUES(7, domain-x, scope-1, cluster-a, 2, 2025-06-01T12:00:00Z, 0, 2025-06-01T13:00:00Z) IF NOT EXISTS`

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantErr     bool
	}{
		{
			name: "success discards the [applied] result and returns no error",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
		},
		{
			name: "exec failure is propagated",
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
			session := &fakeSession{query: query}
			client := gocql.NewMockClient(ctrl)
			db := NewCassandraDBFromSession(&config.NoSQL{}, session, testlogger.New(t), &persistence.DynamicConfiguration{}, DbWithClient(client))

			err := db.InsertHistoryDLQAckLevelIfNotExistsRow(context.Background(), row)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, []string{wantQuery}, session.queries)
		})
	}
}

func TestInsertOrUpdateHistoryDLQAckLevelRow(t *testing.T) {
	ackTS := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2025, 6, 1, 13, 0, 0, 0, time.UTC)

	row := &nosqlplugin.HistoryDLQAckLevelRow{
		ShardID:               7,
		DomainID:              "domain-x",
		ClusterAttributeScope: "scope-1",
		ClusterAttributeName:  "cluster-a",
		TaskCategory:          2,
		AckLevelVisibilityTS:  ackTS,
		AckLevelTaskID:        42,
		LastUpdatedAt:         updatedAt,
	}

	// The upsert path is an unconditional write — no IF NOT EXISTS — so it overwrites progress.
	const wantQuery = `INSERT INTO history_task_dlq_ack_level (` +
		`shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
		`task_category, ack_level_visibility_ts, ack_level_task_id, last_updated_at) ` +
		`VALUES(7, domain-x, scope-1, cluster-a, 2, 2025-06-01T12:00:00Z, 42, 2025-06-01T13:00:00Z)`

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantErr     bool
	}{
		{
			name: "success renders an unconditional upsert",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
		},
		{
			name: "exec failure is propagated",
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
			session := &fakeSession{query: query}
			client := gocql.NewMockClient(ctrl)
			db := NewCassandraDBFromSession(&config.NoSQL{}, session, testlogger.New(t), &persistence.DynamicConfiguration{}, DbWithClient(client))

			err := db.InsertOrUpdateHistoryDLQAckLevelRow(context.Background(), row)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, []string{wantQuery}, session.queries)
		})
	}
}
