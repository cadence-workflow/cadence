package cassandra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

const (
	wantInsertAckLevelQuery = `INSERT INTO async_workflow_queue ` +
		`(shard_id, type, source_cluster, message_id, ack_level, created_time, last_updated_time) ` +
		`VALUES (3, 1, c1, -12345, 40, 2025-01-06T15:00:00Z, 2025-01-06T15:00:00Z) IF NOT EXISTS`
	wantUpdateAckLevelQuery = `UPDATE async_workflow_queue SET ack_level = 40, last_updated_time = 2025-01-06T15:00:00Z ` +
		`WHERE shard_id = 3 AND type = 1 AND source_cluster = c1 AND message_id = -12345 ` +
		`IF ack_level < 40`
)

func TestUpsertAsyncWorkflowAckLevel(t *testing.T) {
	now := time.Date(2025, 1, 6, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		casResults  []func(map[string]interface{}) (bool, error)
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "no control row yet: IF NOT EXISTS applies and no update is issued",
			casResults: []func(map[string]interface{}) (bool, error){
				func(map[string]interface{}) (bool, error) { return true, nil },
			},
			wantQueries: []string{wantInsertAckLevelQuery},
		},
		{
			name: "control row exists with a lower ack level: conditional update applies",
			casResults: []func(map[string]interface{}) (bool, error){
				func(prev map[string]interface{}) (bool, error) { prev["ack_level"] = int64(10); return false, nil },
				func(map[string]interface{}) (bool, error) { return true, nil },
			},
			wantQueries: []string{wantInsertAckLevelQuery, wantUpdateAckLevelQuery},
		},
		{
			name: "control row exists with an equal or higher ack level: update not applied is a silent no-op",
			casResults: []func(map[string]interface{}) (bool, error){
				func(prev map[string]interface{}) (bool, error) { prev["ack_level"] = int64(90); return false, nil },
				func(prev map[string]interface{}) (bool, error) { prev["ack_level"] = int64(90); return false, nil },
			},
			wantQueries: []string{wantInsertAckLevelQuery, wantUpdateAckLevelQuery},
		},
		{
			name: "insert error is returned and no update is issued",
			casResults: []func(map[string]interface{}) (bool, error){
				func(map[string]interface{}) (bool, error) { return false, errors.New("insert failed") },
			},
			wantQueries: []string{wantInsertAckLevelQuery},
			wantErr:     true,
		},
		{
			name: "update error is returned",
			casResults: []func(map[string]interface{}) (bool, error){
				func(prev map[string]interface{}) (bool, error) { prev["ack_level"] = int64(10); return false, nil },
				func(map[string]interface{}) (bool, error) { return false, errors.New("update failed") },
			},
			wantQueries: []string{wantInsertAckLevelQuery, wantUpdateAckLevelQuery},
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(len(tc.casResults))
			var calls []any
			for _, fn := range tc.casResults {
				fn := fn
				calls = append(calls, query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(fn).Times(1))
			}
			gomock.InOrder(calls...)
			session := &fakeSession{query: query}
			db := newTestCDB(t, session)

			err := db.UpsertAsyncWorkflowAckLevel(context.Background(), 3, "c1", 40, now)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantQueries, session.queries)
		})
	}
}

func TestSelectAsyncWorkflowMessages(t *testing.T) {
	created := time.Date(2025, 1, 6, 15, 0, 0, 0, time.UTC)
	const wantQuery = `SELECT message_id, message_payload, message_payload_encoding, domain_name, workflow_id, request_id, request_type, created_time ` +
		`FROM async_workflow_queue ` +
		`WHERE shard_id = 3 AND type = 0 AND source_cluster = c1 AND message_id > 10`

	scan := func(id int64) []interface{} {
		return []interface{}{id, []byte{byte(id)}, "thriftrw", "domain", "wf", "req", int(1), created}
	}

	tests := []struct {
		name      string
		pageSize  int
		iter      *fakeIter
		wantIDs   []int64
		wantErr   bool
		wantFirst bool
	}{
		{
			name:      "returns rows in scan order",
			pageSize:  10,
			iter:      &fakeIter{scanInputs: [][]interface{}{scan(11), scan(12)}},
			wantIDs:   []int64{11, 12},
			wantFirst: true,
		},
		{
			name:     "stops at pageSize even if the iterator has more",
			pageSize: 2,
			iter:     &fakeIter{scanInputs: [][]interface{}{scan(11), scan(12), scan(13)}},
			wantIDs:  []int64{11, 12},
		},
		{
			name:     "empty queue",
			pageSize: 5,
			iter:     &fakeIter{},
			wantIDs:  nil,
		},
		{
			name:     "iterator close error is returned",
			pageSize: 5,
			iter:     &fakeIter{scanInputs: [][]interface{}{scan(11)}, closeErr: errors.New("boom")},
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().PageSize(tc.pageSize).Return(query).Times(1)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			query.EXPECT().Iter().Return(tc.iter).Times(1)
			session := &fakeSession{query: query}
			db := newTestCDB(t, session)

			rows, err := db.SelectAsyncWorkflowMessages(context.Background(), 3, "c1", 10, tc.pageSize)

			assert.Equal(t, []string{wantQuery}, session.queries)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			var gotIDs []int64
			for _, r := range rows {
				gotIDs = append(gotIDs, r.MessageID)
				assert.Equal(t, 3, r.ShardID)
				assert.Equal(t, "c1", r.SourceCluster)
			}
			assert.Equal(t, tc.wantIDs, gotIDs)
			if tc.wantFirst {
				assert.Equal(t, "thriftrw", rows[0].PayloadEncoding)
				assert.Equal(t, []byte{11}, rows[0].Payload)
				assert.Equal(t, "domain", rows[0].DomainName)
				assert.Equal(t, "wf", rows[0].WorkflowID)
				assert.Equal(t, "req", rows[0].RequestID)
				assert.Equal(t, 1, rows[0].RequestType)
				assert.Equal(t, created, rows[0].CreatedTime)
			}
			assert.True(t, tc.iter.closed)
		})
	}
}

func TestInsertAsyncWorkflowMessage(t *testing.T) {
	created := time.Date(2025, 1, 6, 15, 0, 0, 0, time.UTC)
	const wantQuery = `INSERT INTO async_workflow_queue ` +
		`(shard_id, type, source_cluster, message_id, message_payload, message_payload_encoding, domain_name, workflow_id, request_id, request_type, created_time) ` +
		`VALUES (3, 0, c1, 11, [112 97 121 108 111 97 100], thriftrw, domain, wf, req, 1, 2025-01-06T15:00:00Z) IF NOT EXISTS`
	insertErr := errors.New("insert failed")

	tests := []struct {
		name    string
		applied bool
		casErr  error
		assert  func(t *testing.T, err error)
	}{
		{
			name:    "new message id: insert applies",
			applied: true,
			assert:  func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:    "message id already exists: condition failure",
			applied: false,
			assert: func(t *testing.T, err error) {
				var conditionFailure *nosqlplugin.ConditionFailure
				require.ErrorAs(t, err, &conditionFailure)
			},
		},
		{
			name:   "query error is returned",
			casErr: insertErr,
			assert: func(t *testing.T, err error) { require.ErrorIs(t, err, insertErr) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			query.EXPECT().MapScanCAS(gomock.Any()).Return(tc.applied, tc.casErr).Times(1)
			session := &fakeSession{query: query}
			db := newTestCDB(t, session)

			err := db.InsertAsyncWorkflowMessage(context.Background(), &nosqlplugin.AsyncWorkflowMessageRow{
				ShardID:         3,
				SourceCluster:   "c1",
				MessageID:       11,
				DomainName:      "domain",
				WorkflowID:      "wf",
				RequestID:       "req",
				RequestType:     1,
				Payload:         []byte("payload"),
				PayloadEncoding: "thriftrw",
				CreatedTime:     created,
			})

			assert.Equal(t, []string{wantQuery}, session.queries)
			tc.assert(t, err)
		})
	}
}

func TestSelectAsyncWorkflowAckLevels(t *testing.T) {
	const wantQuery = `SELECT source_cluster, ack_level FROM async_workflow_queue WHERE shard_id = 3 AND type = 1`

	tests := []struct {
		name    string
		iter    *fakeIter
		want    map[string]int64
		wantErr bool
	}{
		{
			name: "returns ack level per source cluster",
			iter: &fakeIter{scanInputs: [][]interface{}{{"c1", int64(5)}, {"c2", int64(9)}}},
			want: map[string]int64{"c1": 5, "c2": 9},
		},
		{
			name: "no control rows returns an empty non-nil map",
			iter: &fakeIter{},
			want: map[string]int64{},
		},
		{
			name:    "iterator close error is returned",
			iter:    &fakeIter{scanInputs: [][]interface{}{{"c1", int64(5)}}, closeErr: errors.New("boom")},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			query.EXPECT().Iter().Return(tc.iter).Times(1)
			session := &fakeSession{query: query}
			db := newTestCDB(t, session)

			got, err := db.SelectAsyncWorkflowAckLevels(context.Background(), 3)

			assert.Equal(t, []string{wantQuery}, session.queries)
			assert.True(t, tc.iter.closed)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRangeDeleteAsyncWorkflowMessages(t *testing.T) {
	const wantQuery = `DELETE FROM async_workflow_queue WHERE shard_id = 3 AND type = 0 AND source_cluster = c1 AND message_id <= 20`

	tests := []struct {
		name           string
		allConsistency bool
		queryMockFn    func(query *gocql.MockQuery)
		wantErr        bool
	}{
		{
			name: "flag off: plain Exec succeeds",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
		},
		{
			name: "flag off: Exec error is returned",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("delete failed")).Times(1)
			},
			wantErr: true,
		},
		{
			name:           "flag on: delete runs at consistency ALL",
			allConsistency: true,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Consistency(cassandraAllConslevel).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{query: query}
			dc := persistence.NewDefaultDynamicConfiguration()
			dc.EnableCassandraAllConsistencyLevelDelete = func(opts ...dynamicproperties.FilterOption) bool {
				return tc.allConsistency
			}
			db := NewCassandraDBFromSession(&config.NoSQL{}, session, testlogger.New(t), dc, DbWithClient(gocql.NewMockClient(ctrl)))

			err := db.RangeDeleteAsyncWorkflowMessages(context.Background(), 3, "c1", 20)

			assert.Equal(t, []string{wantQuery}, session.queries)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
