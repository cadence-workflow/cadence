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

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/testdata"
)

func TestInsertVisibility(t *testing.T) {
	tests := []struct {
		desc          string
		row           *nosqlplugin.VisibilityRowForInsert
		ttlSeconds    int64
		queryMockFunc func(*gocql.MockQuery)
		wantQueries   []string
		wantErr       bool
	}{
		{
			desc:       "Query with ttl less than maxCassandraTTL",
			row:        testdata.NewVisibilityRowForInsert(),
			ttlSeconds: int64(1000),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().WithTimestamp(gomock.Any()).Return(query)
				query.EXPECT().Exec().Return(nil)
			},
			wantQueries: []string{
				`INSERT INTO open_executions (domain_id, domain_partition,  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time )VALUES (test-domain-id, 0, test-workflow-id, test-run-id, 1712009321000, 1712009321000, test-type-name, [], json, test-task-list, false, 1, 2024-04-01T22:08:41Z, 1, STARTED, , -6795364578871) using TTL 1000`,
			},
			wantErr: false,
		},
		{
			desc:       "Query With ttl greater than maxCassandraTTL",
			row:        testdata.NewVisibilityRowForInsert(),
			ttlSeconds: maxCassandraTTL + 1,
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().WithTimestamp(gomock.Any()).Return(query)
				query.EXPECT().Exec().Return(nil)
			},
			wantQueries: []string{
				`INSERT INTO open_executions(domain_id, domain_partition,  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time )VALUES (test-domain-id, 0, test-workflow-id, test-run-id, 1712009321000, 1712009321000, test-type-name, [], json, test-task-list, false, 1, 2024-04-01T22:08:41Z, 1, STARTED, , -6795364578871)`,
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			if test.queryMockFunc != nil {
				test.queryMockFunc(query)
			}
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			err := db.InsertVisibility(context.Background(), test.ttlSeconds, test.row)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.wantQueries, session.queries)
		})
	}
}

func TestUpdateVisibility(t *testing.T) {
	tests := []struct {
		desc        string
		row         *nosqlplugin.VisibilityRowForUpdate
		ttlSeconds  int64
		wantQueries []string
		wantErr     bool
		wantPanic   bool
	}{
		{
			desc:       "Query with ttl less than maxCassandraTTL",
			row:        testdata.NewVisibilityRowForUpdate(false, true),
			ttlSeconds: int64(100),
			wantQueries: []string{
				`DELETE FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND start_time = 1712009321000 AND run_id = test-run-id`,
				`INSERT INTO closed_executions (domain_id, domain_partition,  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time )VALUES (test-domain-id, 0, test-workflow-id, test-run-id, 1712009321000, 1712009321000, 1712009261000, test-type-name, COMPLETED, 1, [], json, test-task-list, false, 1, 2024-04-01T22:08:41Z, 1, STARTED, , -6795364578871) using TTL 100`,
				`INSERT INTO closed_executions_v2 (domain_id, domain_partition,  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time )VALUES (test-domain-id, 0, test-workflow-id, test-run-id, 1712009321000, 1712009321000, 1712009261000, test-type-name, COMPLETED, 1, [], json, test-task-list, false, 1, 2024-04-01T22:08:41Z, 1, STARTED, , -6795364578871) using TTL 100`,
			},
			wantErr:   false,
			wantPanic: false,
		},
		{
			desc:       "Query with ttl greater than maxCassandraTTL",
			row:        testdata.NewVisibilityRowForUpdate(false, true),
			ttlSeconds: maxCassandraTTL + 1,
			wantQueries: []string{
				`DELETE FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND start_time = 1712009321000 AND run_id = test-run-id`,
				`INSERT INTO closed_executions (domain_id, domain_partition,  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time )VALUES (test-domain-id, 0, test-workflow-id, test-run-id, 1712009321000, 1712009321000, 1712009261000, test-type-name, COMPLETED, 1, [], json, test-task-list, false, 1, 2024-04-01T22:08:41Z, 1, STARTED, , -6795364578871)`,
				`INSERT INTO closed_executions_v2 (domain_id, domain_partition,  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time )VALUES (test-domain-id, 0, test-workflow-id, test-run-id, 1712009321000, 1712009321000, 1712009261000, test-type-name, COMPLETED, 1, [], json, test-task-list, false, 1, 2024-04-01T22:08:41Z, 1, STARTED, , -6795364578871)`,
			},
			wantErr:   false,
			wantPanic: false,
		},
		{
			desc:       "panic if updateCloseToOpen is set",
			row:        testdata.NewVisibilityRowForUpdate(true, true),
			ttlSeconds: int64(100),
			wantErr:    false,
			wantPanic:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != test.wantPanic {
					t.Errorf("test: %s, panicWanted: %v, panicOccured: %v", test.desc, test.wantPanic, r != nil)
				}
			}()
			ctrl := gomock.NewController(t)
			session := &fakeSession{
				query: gocql.NewMockQuery(ctrl),
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))
			err := db.UpdateVisibility(context.Background(), test.ttlSeconds, test.row)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.wantQueries, session.batches[0].queries)
		})
	}
}

func TestSelectOneClosedWorkflow(t *testing.T) {
	tests := []struct {
		desc        string
		domainID    string
		workflowID  string
		runID       string
		itrMockFunc func(*gocql.MockIter)
		wantQueries []string
		wantError   bool
		wantResult  bool
	}{
		{
			desc:        "return error when query's iter function returns nil",
			domainID:    testdata.DomainID,
			workflowID:  testdata.WorkflowID,
			runID:       testdata.RunID,
			itrMockFunc: nil,
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND workflow_id = test-workflow-id AND run_id = test-run-id ALLOW FILTERING `,
			},
			wantError:  true,
			wantResult: false,
		},
		{
			desc:       "return nil if reading closed_workflow_execution_record returns false",
			domainID:   testdata.DomainID,
			workflowID: testdata.WorkflowID,
			runID:      testdata.RunID,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND workflow_id = test-workflow-id AND run_id = test-run-id ALLOW FILTERING `,
			},
			wantError:  false,
			wantResult: false,
		},
		{
			desc:       "return error if closing iterator fails",
			domainID:   testdata.DomainID,
			workflowID: testdata.WorkflowID,
			runID:      testdata.RunID,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(true)
				itr.EXPECT().Close().Return(errors.New("close error"))
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND workflow_id = test-workflow-id AND run_id = test-run-id ALLOW FILTERING `,
			},
			wantError:  true,
			wantResult: false,
		},
		{
			desc:       "success",
			domainID:   testdata.DomainID,
			workflowID: testdata.WorkflowID,
			runID:      testdata.RunID,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(true)
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND workflow_id = test-workflow-id AND run_id = test-run-id ALLOW FILTERING `,
			},
			wantError:  false,
			wantResult: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().WithContext(gomock.Any()).Return(query)
			if test.itrMockFunc != nil {
				itr := gocql.NewMockIter(ctrl)
				test.itrMockFunc(itr)
				query.EXPECT().Iter().Return(itr)
			} else {
				query.EXPECT().Iter().Return(nil)
			}
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))
			result, err := db.SelectOneClosedWorkflow(context.Background(), test.domainID, test.workflowID, test.runID)
			if test.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if test.wantResult {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
			assert.Equal(t, test.wantQueries, session.queries)
		})
	}
}

func TestDeleteVisibility(t *testing.T) {
	tests := []struct {
		desc           string
		domainID       string
		workflowID     string
		runID          string
		mockItr        bool
		itrMockFunc    func(*gocql.MockIter)
		queryMockFunc  func(*gocql.MockQuery)
		context        context.Context
		dc             *persistence.DynamicConfiguration
		clientMockFunc func(*gocql.MockClient)
		wantQueries    []string
		wantError      bool
	}{
		{
			desc:           "return nil if visibility_admin_key not present in context",
			domainID:       "",
			workflowID:     "",
			runID:          "",
			mockItr:        false,
			itrMockFunc:    nil,
			queryMockFunc:  nil,
			context:        context.Background(),
			dc:             &persistence.DynamicConfiguration{},
			clientMockFunc: nil,
			wantQueries:    nil,
			wantError:      false,
		},
		{
			desc:        "return error when query's iter function returns nil",
			domainID:    testdata.DomainID,
			workflowID:  testdata.WorkflowID,
			runID:       testdata.RunID,
			mockItr:     true,
			itrMockFunc: nil,
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query)
			},
			context:        context.WithValue(context.Background(), persistence.VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			dc:             &persistence.DynamicConfiguration{},
			clientMockFunc: nil,
			wantQueries:    []string{`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND run_id = test-run-id ALLOW FILTERING`},
			wantError:      true,
		},
		{
			desc:       "return nil if reading open_workflow_execution_records returns false",
			domainID:   testdata.DomainID,
			workflowID: testdata.WorkflowID,
			runID:      testdata.RunID,
			mockItr:    true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
			},
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query)
			},
			context:        context.WithValue(context.Background(), persistence.VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			dc:             &persistence.DynamicConfiguration{},
			clientMockFunc: nil,
			wantQueries:    []string{`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND run_id = test-run-id ALLOW FILTERING`},
			wantError:      false,
		},
		{
			desc:       "return error if closing iterator fails",
			domainID:   testdata.DomainID,
			workflowID: testdata.WorkflowID,
			runID:      testdata.RunID,
			mockItr:    true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(true)
				itr.EXPECT().Close().Return(errors.New("close error"))
			},
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query)
			},
			context:        context.WithValue(context.Background(), persistence.VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			dc:             &persistence.DynamicConfiguration{},
			clientMockFunc: nil,
			wantQueries:    []string{`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND run_id = test-run-id ALLOW FILTERING`},
			wantError:      true,
		},
		{
			desc:       "success with db dynamic_configuration as nil",
			domainID:   testdata.DomainID,
			workflowID: testdata.WorkflowID,
			runID:      testdata.RunID,
			mockItr:    true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(true)
				itr.EXPECT().Close().Return(nil)
			},
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(2)
				query.EXPECT().Exec().Return(nil)
			},
			context:        context.WithValue(context.Background(), persistence.VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			dc:             nil,
			clientMockFunc: nil,
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND run_id = test-run-id ALLOW FILTERING`,
				`DELETE FROM open_executions WHERE domain_id = test-domain-id and domain_partition = 0 and start_time = 0001-01-01T00:00:00Z and run_id = test-run-id `,
			},
			wantError: false,
		},
		{
			desc:       "success with enable_cassandra_all_consistency_level_delete db dynamic_configuration",
			domainID:   testdata.DomainID,
			workflowID: testdata.WorkflowID,
			runID:      testdata.RunID,
			mockItr:    true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(true)
				itr.EXPECT().Close().Return(nil)
			},
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(2)
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().Exec().Return(nil)
			},
			context: context.WithValue(context.Background(), persistence.VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			dc: &persistence.DynamicConfiguration{EnableCassandraAllConsistencyLevelDelete: func(opts ...dynamicproperties.FilterOption) bool {
				return true
			}},
			clientMockFunc: nil,
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND run_id = test-run-id ALLOW FILTERING`,
				`DELETE FROM open_executions WHERE domain_id = test-domain-id and domain_partition = 0 and start_time = 0001-01-01T00:00:00Z and run_id = test-run-id `,
			},
			wantError: false,
		},
		{
			desc:       "succes with cassandra_default_consistency_level when cassandra_all_consistency_level fails",
			domainID:   testdata.DomainID,
			workflowID: testdata.WorkflowID,
			runID:      testdata.RunID,
			context:    context.WithValue(context.Background(), persistence.VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(2)
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().Exec().Return(errors.New("all consistency level fail"))
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().Exec().Return(nil)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(true)
				itr.EXPECT().Close().Return(nil)
			},
			dc: &persistence.DynamicConfiguration{
				EnableCassandraAllConsistencyLevelDelete: func(opts ...dynamicproperties.FilterOption) bool { return true },
			},
			clientMockFunc: func(client *gocql.MockClient) {
				client.EXPECT().IsCassandraConsistencyError(gomock.Any()).Return(true)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND run_id = test-run-id ALLOW FILTERING`,
				`DELETE FROM open_executions WHERE domain_id = test-domain-id and domain_partition = 0 and start_time = 0001-01-01T00:00:00Z and run_id = test-run-id `,
			},
			wantError: false,
		},
		{
			desc:       "return error cassandra_all_consistency_level fails and error is not cassandra consistency error",
			domainID:   testdata.DomainID,
			workflowID: testdata.WorkflowID,
			runID:      testdata.RunID,
			context:    context.WithValue(context.Background(), persistence.VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(2)
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().Exec().Return(errors.New("all consistency level fail"))
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(true)
				itr.EXPECT().Close().Return(nil)
			},
			dc: &persistence.DynamicConfiguration{
				EnableCassandraAllConsistencyLevelDelete: func(opts ...dynamicproperties.FilterOption) bool { return true },
			},
			clientMockFunc: func(client *gocql.MockClient) {
				client.EXPECT().IsCassandraConsistencyError(gomock.Any()).Return(false)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND run_id = test-run-id ALLOW FILTERING`,
				`DELETE FROM open_executions WHERE domain_id = test-domain-id and domain_partition = 0 and start_time = 0001-01-01T00:00:00Z and run_id = test-run-id `,
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			if test.queryMockFunc != nil {
				test.queryMockFunc(query)
			}
			if test.mockItr && test.itrMockFunc != nil {
				itr := gocql.NewMockIter(ctrl)
				test.itrMockFunc(itr)
				query.EXPECT().Iter().Return(itr)
			} else if test.mockItr {
				query.EXPECT().Iter().Return(nil)
			}
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			if test.clientMockFunc != nil {
				test.clientMockFunc(client)
			}
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			db := NewCassandraDBFromSession(cfg, session, logger, test.dc, DbWithClient(client))
			err := db.DeleteVisibility(test.context, test.domainID, test.workflowID, test.runID)
			if test.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.wantQueries, session.queries)
		})
	}
}

func TestSelectVisibility(t *testing.T) {
	tests := []struct {
		desc          string
		filter        *nosqlplugin.VisibilityFilter
		queryMockFunc func(query *gocql.MockQuery)
		mockItr       bool
		itrMockFunc   func(itr *gocql.MockIter)
		wantQueries   []string
		wantError     bool
		wantResult    bool
		wantPanic     bool
	}{
		{
			desc:          "panic for invalid filter type",
			filter:        testdata.NewSelectVisibilityRequestFilter(nosqlplugin.VisibilityFilterType(10), nosqlplugin.SortByStartTime),
			queryMockFunc: nil,
			mockItr:       false,
			itrMockFunc:   nil,
			wantQueries:   nil,
			wantError:     false,
			wantResult:    false,
			wantPanic:     true,
		},
		{
			desc: "all-open type filter return error if iterator is nil",
			// passing sort type as StartByStartTime(as it is the default value) for cases where sort type is not needed
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.AllOpen, nosqlplugin.SortByStartTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr:     true,
			itrMockFunc: nil,
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition IN (0) AND start_time >= 1712009321000 AND start_time <= 1712009321000 `,
			},
			wantError:  true,
			wantResult: false,
			wantPanic:  false,
		},
		{
			desc:   "all-open type filter return error if closing iterator fails",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.AllOpen, nosqlplugin.SortByStartTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(15)...).Return(true)
				itr.EXPECT().Scan(generateMockParams(15)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(errors.New("close error"))
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition IN (0) AND start_time >= 1712009321000 AND start_time <= 1712009321000 `,
			},
			wantError:  true,
			wantResult: false,
			wantPanic:  false,
		},
		{
			desc:   "all-open type filter success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.AllOpen, nosqlplugin.SortByStartTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(true)
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition IN (0) AND start_time >= 1712009321000 AND start_time <= 1712009321000 `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:   "all-closed type filter sort by start-time success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.AllClosed, nosqlplugin.SortByStartTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions WHERE domain_id = test-domain-id AND domain_partition IN (0) AND start_time >= 1712009321000 AND start_time <= 1712009321000 `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:   "all-closed type filter sort by closed-time success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.AllClosed, nosqlplugin.SortByClosedTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions_v2 WHERE domain_id = test-domain-id AND domain_partition IN (0) AND close_time >= 1712009321000 AND close_time <= 1712009321000 `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:   "panic on all-closed type filter invalid sort attribute",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.AllClosed, nosqlplugin.VisibilitySortType(100)),
			queryMockFunc: func(query *gocql.MockQuery) {
			},
			mockItr:     false,
			itrMockFunc: nil,
			wantQueries: nil,
			wantError:   false,
			wantResult:  false,
			wantPanic:   true,
		},
		{
			desc:   "open-by-workflow-type filter success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.OpenByWorkflowType, nosqlplugin.SortByStartTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(15)...).Return(true)
				itr.EXPECT().Scan(generateMockParams(15)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND start_time >= 1712009321000 AND start_time <= 1712009321000 AND workflow_type_name = test-workflow-type `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:   "closed-by-workflow-type filter sort by start-time success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.ClosedByWorkflowType, nosqlplugin.SortByStartTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND start_time >= 1712009321000 AND start_time <= 1712009321000 AND workflow_type_name = test-workflow-type `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:   "closed-by-workflow-type filter sort by closed-time success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.ClosedByWorkflowType, nosqlplugin.SortByClosedTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions_v2 WHERE domain_id = test-domain-id AND domain_partition = 0 AND close_time >= 1712009321000 AND close_time <= 1712009321000 AND workflow_type_name = test-workflow-type `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:          "panic on closed-by-workflow-type filter invalid sort attribute",
			filter:        testdata.NewSelectVisibilityRequestFilter(nosqlplugin.ClosedByWorkflowType, nosqlplugin.VisibilitySortType(100)),
			queryMockFunc: nil,
			mockItr:       false,
			itrMockFunc:   nil,
			wantQueries:   nil,
			wantError:     false,
			wantResult:    false,
			wantPanic:     true,
		},
		{
			desc:   "open-by-workflow-id filter success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.OpenByWorkflowID, nosqlplugin.SortByStartTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(15)...).Return(true)
				itr.EXPECT().Scan(generateMockParams(15)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time FROM open_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND start_time >= 1712009321000 AND start_time <= 1712009321000 AND workflow_id = test-workflow-id `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:   "closed-by-workflow-id filter sort by start-time success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.ClosedByWorkflowID, nosqlplugin.SortByStartTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND start_time >= 1712009321000 AND start_time <= 1712009321000 AND workflow_id = test-workflow-id `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:   "closed-by-workflow-id filter sort by closed-time success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.ClosedByWorkflowID, nosqlplugin.SortByClosedTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions_v2 WHERE domain_id = test-domain-id AND domain_partition = 0 AND close_time >= 1712009321000 AND close_time <= 1712009321000 AND workflow_id = test-workflow-id `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:          "panic on closed-by-workflow-id filter invalid sort attribute",
			filter:        testdata.NewSelectVisibilityRequestFilter(nosqlplugin.ClosedByWorkflowID, nosqlplugin.VisibilitySortType(100)),
			queryMockFunc: nil,
			mockItr:       false,
			itrMockFunc:   nil,
			wantQueries:   nil,
			wantError:     false,
			wantResult:    false,
			wantPanic:     true,
		},
		{
			desc:   "closed-by-closed-status filter sort by start-time success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.ClosedByClosedStatus, nosqlplugin.SortByStartTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions WHERE domain_id = test-domain-id AND domain_partition = 0 AND start_time >= 1712009321000 AND start_time <= 1712009321000 AND status = 0 `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:   "closed-by-closed-status filter sort by closed-time success",
			filter: testdata.NewSelectVisibilityRequestFilter(nosqlplugin.ClosedByClosedStatus, nosqlplugin.SortByClosedTime),
			queryMockFunc: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(gomock.Any()).Return(query)
				query.EXPECT().WithContext(gomock.Any()).Return(query)
				query.EXPECT().PageSize(gomock.Any()).Return(query)
				query.EXPECT().PageState(gomock.Any()).Return(query)
			},
			mockItr: true,
			itrMockFunc: func(itr *gocql.MockIter) {
				itr.EXPECT().Scan(generateMockParams(18)...).Return(false)
				itr.EXPECT().PageState().Return([]byte("test"))
				itr.EXPECT().Close().Return(nil)
			},
			wantQueries: []string{
				`SELECT  workflow_id, run_id, start_time, execution_time, close_time, workflow_type_name, status, history_length, memo, encoding, task_list, is_cron, num_clusters, update_time, shard_id FROM closed_executions_v2 WHERE domain_id = test-domain-id AND domain_partition = 0 AND close_time >= 1712009321000 AND close_time <= 1712009321000 AND status = 0 `,
			},
			wantError:  false,
			wantResult: true,
			wantPanic:  false,
		},
		{
			desc:          "panic on closed-by-closed-status filter invalid sort attribute",
			filter:        testdata.NewSelectVisibilityRequestFilter(nosqlplugin.ClosedByClosedStatus, nosqlplugin.VisibilitySortType(100)),
			queryMockFunc: nil,
			mockItr:       false,
			itrMockFunc:   nil,
			wantQueries:   nil,
			wantError:     false,
			wantResult:    false,
			wantPanic:     true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != test.wantPanic {
					t.Errorf("test: %s, panicWanted: %v, panicOccured: %v", test.desc, test.wantPanic, r != nil)
				}
			}()

			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			if test.queryMockFunc != nil {
				test.queryMockFunc(query)
			}
			if test.mockItr && test.itrMockFunc != nil {
				itr := gocql.NewMockIter(ctrl)
				test.itrMockFunc(itr)
				query.EXPECT().Iter().Return(itr)
			} else if test.mockItr {
				query.EXPECT().Iter().Return(nil)
			}
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))
			result, err := db.SelectVisibility(context.Background(), test.filter)
			if test.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if test.wantResult {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
			assert.Equal(t, test.wantQueries, session.queries)
		})
	}
}

func generateMockParams(count int) []interface{} {
	params := []interface{}{}
	for i := 0; i < count; i++ {
		params = append(params, gomock.Any())
	}
	return params
}
