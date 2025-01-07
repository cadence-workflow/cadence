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

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/testdata"
	"github.com/uber/cadence/common/types"
)

func TestInsertDomain(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-03T18:00:00Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name                      string
		row                       *nosqlplugin.DomainRow
		queryMockFn               func(query *gocql.MockQuery)
		clientMockFn              func(client *gocql.MockClient)
		mapExecuteBatchCASApplied bool
		mapExecuteBatchCASPrev    map[string]any
		mapExecuteBatchCASErr     error
		wantSessionQueries        []string
		wantBatchQueries          []string
		wantErr                   bool
	}{
		{
			name: "insertion MapScanCAS failed",
			row:  testdata.NewDomainRow(ts),
			queryMockFn: func(query *gocql.MockQuery) {
				// mock calls for insert
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return false, errors.New("some random error")
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name: "insertion MapScanCAS could not apply",
			row:  testdata.NewDomainRow(ts),
			queryMockFn: func(query *gocql.MockQuery) {
				// mock calls for insert
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return false, nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name: "insertion success - select metadata failed",
			row:  testdata.NewDomainRow(ts),
			queryMockFn: func(query *gocql.MockQuery) {
				// mock calls for insert
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)

				// mock calls for SelectDomainMetadata
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return errors.New("some random error")
				}).Times(1)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsNotFoundError(gomock.Any()).Return(false).Times(1)
			},
			wantErr: true,
		},
		{
			name:                  "insertion success - select metadata success - insertion to domains_by_name_v2 failed",
			row:                   testdata.NewDomainRow(ts),
			mapExecuteBatchCASErr: errors.New("insert to domains_by_name_v2 failed"),
			queryMockFn: func(query *gocql.MockQuery) {
				// mock calls for insert
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)

				// mock calls for SelectDomainMetadata
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name:                      "insertion success - select metadata success - insertion to domains_by_name_v2 not applied - orphan domain deletion failed",
			row:                       testdata.NewDomainRow(ts),
			mapExecuteBatchCASApplied: false,
			queryMockFn: func(query *gocql.MockQuery) {
				// mock calls for insert
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)

				// mock calls for SelectDomainMetadata
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return nil
				}).Times(1)

				// mock calls for deleting orphan domain
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("orphan domain deletion failure")).Times(1)
			},
			wantErr: true,
		},
		{
			name:                      "insertion success - select metadata success - insertion to domains_by_name_v2 not applied - domain already exists",
			row:                       testdata.NewDomainRow(ts),
			mapExecuteBatchCASApplied: false,
			mapExecuteBatchCASPrev: map[string]any{
				"name": testdata.NewDomainRow(ts).Info.Name, // this will causedomain already exist error
			},
			queryMockFn: func(query *gocql.MockQuery) {
				// mock calls for insert
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)

				// mock calls for SelectDomainMetadata
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return nil
				}).Times(1)

				// mock calls for deleting orphan domain
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: true,
		},
		{
			name:                      "all success",
			row:                       testdata.NewDomainRow(ts),
			mapExecuteBatchCASApplied: true,
			queryMockFn: func(query *gocql.MockQuery) {
				// mock calls for insert
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)

				// mock calls for SelectDomainMetadata
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					notificationVersion := args[0].(*int64)
					*notificationVersion = 7
					return nil
				}).Times(1)
			},
			wantSessionQueries: []string{
				`INSERT INTO domains (id, domain) VALUES(test-domain-id, {name: test-domain-name}) IF NOT EXISTS`,
				`SELECT notification_version FROM domains_by_name_v2 WHERE domains_partition = 0 and name = cadence-domain-metadata `,
			},
			wantBatchQueries: []string{
				`INSERT INTO domains_by_name_v2 (` +
					`domains_partition, ` +
					`name, ` +
					`domain, ` +
					`config, ` +
					`replication_config, ` +
					`is_global_domain, ` +
					`config_version, ` +
					`failover_version, ` +
					`failover_notification_version, ` +
					`previous_failover_version, ` +
					`failover_end_time, ` +
					`last_updated_time, ` +
					`notification_version) ` +
					`VALUES(` +
					`0, ` +
					`test-domain-name, ` +
					`{id: test-domain-id, name: test-domain-name, status: 0, description: test-domain-description, owner_email: test-domain-owner-email, data: map[k1:v1] }, ` +
					`{retention: 7, emit_metric: true, archival_bucket: test-archival-bucket, archival_status: ENABLED,history_archival_status: ENABLED, history_archival_uri: test-history-archival-uri, visibility_archival_status: ENABLED, visibility_archival_uri: test-visibility-archival-uri, bad_binaries: [98 97 100 45 98 105 110 97 114 105 101 115],bad_binaries_encoding: thriftrw,isolation_groups: [105 115 111 108 97 116 105 111 110 45 103 114 111 117 112],isolation_groups_encoding: thriftrw,async_workflow_config: [97 115 121 110 99 45 119 111 114 107 102 108 111 119 115 45 99 111 110 102 105 103],async_workflow_config_encoding: thriftrw}, ` +
					`{active_cluster_name: test-active-cluster-name, clusters: [map[cluster_name:test-cluster-name]] }, ` +
					`true, ` +
					`3, ` +
					`4, ` +
					`0, ` +
					`-1, ` +
					`1712167200000000000, ` +
					`1712167200000000000, ` +
					`7) ` +
					`IF NOT EXISTS`,
				`UPDATE domains_by_name_v2 SET notification_version = 8 WHERE domains_partition = 0 and name = cadence-domain-metadata IF notification_version = 7 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{
				query:                     query,
				mapExecuteBatchCASApplied: tc.mapExecuteBatchCASApplied,
				mapExecuteBatchCASPrev:    tc.mapExecuteBatchCASPrev,
				mapExecuteBatchCASErr:     tc.mapExecuteBatchCASErr,
				iter:                      &fakeIter{},
			}
			client := gocql.NewMockClient(ctrl)
			if tc.clientMockFn != nil {
				tc.clientMockFn(client)
			}
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			err := db.InsertDomain(context.Background(), tc.row)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantSessionQueries, session.queries); diff != "" {
				t.Fatalf("Session queries mismatch (-want +got):\n%s", diff)
			}

			if len(session.batches) != 1 {
				t.Fatalf("Expected 1 batch, got %v", len(session.batches))
			}

			if diff := cmp.Diff(tc.wantBatchQueries, session.batches[0].queries); diff != "" {
				t.Fatalf("Batch queries mismatch (-want +got):\n%s", diff)
			}

			if !session.iter.closed {
				t.Error("Expected iter to be closed")
			}
		})
	}
}

func TestUpdateDomain(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-04T18:00:00Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name                      string
		row                       *nosqlplugin.DomainRow
		mapExecuteBatchCASApplied bool
		mapExecuteBatchCASPrev    map[string]any
		mapExecuteBatchCASErr     error
		wantBatchQueries          []string
		wantErr                   bool
	}{
		{
			name: "mapExecuteBatchCAS could not apply",
			row: func() *nosqlplugin.DomainRow {
				r := testdata.NewDomainRow(ts)
				r.FailoverEndTime = nil
				return r
			}(),
			mapExecuteBatchCASApplied: false,
			wantErr:                   true,
		},
		{
			name: "mapExecuteBatchCAS failed",
			row: func() *nosqlplugin.DomainRow {
				r := testdata.NewDomainRow(ts)
				r.FailoverEndTime = nil
				return r
			}(),
			mapExecuteBatchCASErr: errors.New("some random error"),
			wantErr:               true,
		},
		{
			name: "empty failover end time",
			row: func() *nosqlplugin.DomainRow {
				r := testdata.NewDomainRow(ts)
				r.FailoverEndTime = nil
				return r
			}(),
			mapExecuteBatchCASApplied: true,
			wantBatchQueries: []string{
				`UPDATE domains_by_name_v2 SET ` +
					`domain = {id: test-domain-id, name: test-domain-name, status: 0, description: test-domain-description, owner_email: test-domain-owner-email, data: map[k1:v1] }, ` +
					`config = {retention: 7, emit_metric: true, archival_bucket: test-archival-bucket, archival_status: ENABLED,history_archival_status: ENABLED, history_archival_uri: test-history-archival-uri, visibility_archival_status: ENABLED, visibility_archival_uri: test-visibility-archival-uri, bad_binaries: [98 97 100 45 98 105 110 97 114 105 101 115],bad_binaries_encoding: thriftrw,isolation_groups: [105 115 111 108 97 116 105 111 110 45 103 114 111 117 112],isolation_groups_encoding: thriftrw,async_workflow_config: [97 115 121 110 99 45 119 111 114 107 102 108 111 119 115 45 99 111 110 102 105 103],async_workflow_config_encoding: thriftrw}, ` +
					`replication_config = {active_cluster_name: test-active-cluster-name, clusters: [map[cluster_name:test-cluster-name]] }, ` +
					`config_version = 3 ,` +
					`failover_version = 4 ,` +
					`failover_notification_version = 0 , ` +
					`previous_failover_version = 0 , ` +
					`failover_end_time = 0,` +
					`last_updated_time = 1712253600000000000,` +
					`notification_version = 5 ` +
					`WHERE domains_partition = 0 and name = test-domain-name`,
				`UPDATE domains_by_name_v2 SET notification_version = 6 WHERE ` +
					`domains_partition = 0 and ` +
					`name = cadence-domain-metadata ` +
					`IF notification_version = 5 `,
			},
		},
		{
			name:                      "success",
			row:                       testdata.NewDomainRow(ts),
			mapExecuteBatchCASApplied: true,
			wantBatchQueries: []string{
				`UPDATE domains_by_name_v2 SET ` +
					`domain = {id: test-domain-id, name: test-domain-name, status: 0, description: test-domain-description, owner_email: test-domain-owner-email, data: map[k1:v1] }, ` +
					`config = {retention: 7, emit_metric: true, archival_bucket: test-archival-bucket, archival_status: ENABLED,history_archival_status: ENABLED, history_archival_uri: test-history-archival-uri, visibility_archival_status: ENABLED, visibility_archival_uri: test-visibility-archival-uri, bad_binaries: [98 97 100 45 98 105 110 97 114 105 101 115],bad_binaries_encoding: thriftrw,isolation_groups: [105 115 111 108 97 116 105 111 110 45 103 114 111 117 112],isolation_groups_encoding: thriftrw,async_workflow_config: [97 115 121 110 99 45 119 111 114 107 102 108 111 119 115 45 99 111 110 102 105 103],async_workflow_config_encoding: thriftrw}, ` +
					`replication_config = {active_cluster_name: test-active-cluster-name, clusters: [map[cluster_name:test-cluster-name]] }, ` +
					`config_version = 3 ,` +
					`failover_version = 4 ,` +
					`failover_notification_version = 0 , ` +
					`previous_failover_version = 0 , ` +
					`failover_end_time = 1712253600000000000,` +
					`last_updated_time = 1712253600000000000,` +
					`notification_version = 5 ` +
					`WHERE domains_partition = 0 and name = test-domain-name`,
				`UPDATE domains_by_name_v2 SET notification_version = 6 WHERE ` +
					`domains_partition = 0 and ` +
					`name = cadence-domain-metadata ` +
					`IF notification_version = 5 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			session := &fakeSession{
				query:                     query,
				mapExecuteBatchCASApplied: tc.mapExecuteBatchCASApplied,
				mapExecuteBatchCASPrev:    tc.mapExecuteBatchCASPrev,
				mapExecuteBatchCASErr:     tc.mapExecuteBatchCASErr,
				iter:                      &fakeIter{},
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			err := db.UpdateDomain(context.Background(), tc.row)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if len(session.batches) != 1 {
				t.Fatalf("Expected 1 batch, got %v", len(session.batches))
			}

			if diff := cmp.Diff(tc.wantBatchQueries, session.batches[0].queries); diff != "" {
				t.Fatalf("Batch queries mismatch (-want +got):\n%s", diff)
			}

			if !session.iter.closed {
				t.Error("Expected iter to be closed")
			}
		})
	}
}

func TestSelectDomain(t *testing.T) {
	tests := []struct {
		name        string
		domainID    *string
		domainName  *string
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name:       "both domainName and domainID not provided",
			domainName: nil,
			domainID:   nil,
			wantErr:    true,
		},
		{
			name:       "both domainName and domainID provided",
			domainID:   common.StringPtr("domain_id_1"),
			domainName: common.StringPtr("domain_name_1"),
			wantErr:    true,
		},
		{
			name:       "domainName not provided - success",
			domainID:   common.StringPtr("domain_id_1"),
			domainName: nil,
			queryMockFn: func(query *gocql.MockQuery) {
				// mock calls to select domainName
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					name := args[0].(**string)
					domainName := "domain_name_1"
					*name = &domainName
					return nil
				}).Times(1)

				// mock calls to select domain
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).Return(nil).Times(1)
			},
			wantQueries: []string{
				`SELECT domain.name FROM domains WHERE id = domain_id_1`,
				`SELECT domain.id, domain.name, domain.status, domain.description, domain.owner_email, domain.data, config.retention, config.emit_metric, config.archival_bucket, config.archival_status, config.history_archival_status, config.history_archival_uri, config.visibility_archival_status, config.visibility_archival_uri, config.bad_binaries, config.bad_binaries_encoding, replication_config.active_cluster_name, replication_config.clusters, config.isolation_groups,config.isolation_groups_encoding,config.async_workflow_config,config.async_workflow_config_encoding,is_global_domain, config_version, failover_version, failover_notification_version, previous_failover_version, failover_end_time, last_updated_time, notification_version FROM domains_by_name_v2 WHERE domains_partition = 0 and name = domain_name_1`,
			},
		},
		{
			name:       "domainName not provided - scan failure",
			domainID:   common.StringPtr("domain_id_1"),
			domainName: nil,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return errors.New("some random error")
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name:       "domainID not provided - scan failure",
			domainID:   nil,
			domainName: common.StringPtr("domain_name_1"),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return errors.New("some random error")
				}).Times(1)

			},
			wantErr: true,
		},
		{
			name:       "domainID not provided - success",
			domainID:   nil,
			domainName: common.StringPtr("domain_name_1"),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).Return(nil).Times(1)
			},
			wantQueries: []string{
				`SELECT domain.id, domain.name, domain.status, domain.description, domain.owner_email, domain.data, config.retention, config.emit_metric, config.archival_bucket, config.archival_status, config.history_archival_status, config.history_archival_uri, config.visibility_archival_status, config.visibility_archival_uri, config.bad_binaries, config.bad_binaries_encoding, replication_config.active_cluster_name, replication_config.clusters, config.isolation_groups,config.isolation_groups_encoding,config.async_workflow_config,config.async_workflow_config_encoding,is_global_domain, config_version, failover_version, failover_notification_version, previous_failover_version, failover_end_time, last_updated_time, notification_version FROM domains_by_name_v2 WHERE domains_partition = 0 and name = domain_name_1`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			if tc.queryMockFn != nil {
				tc.queryMockFn(query)
			}
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			gotRow, err := db.SelectDomain(context.Background(), tc.domainID, tc.domainName)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if gotRow == nil {
				t.Error("Expected domain row to be returned")
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectAllDomains(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-03T18:00:00Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name        string
		pageSize    int
		pagetoken   []byte
		iter        *fakeIter
		wantQueries []string
		wantRows    []*nosqlplugin.DomainRow
		wantErr     bool
	}{
		{
			name:    "nil iter",
			wantErr: true,
		},
		{
			name:    "iter close failed",
			iter:    &fakeIter{closeErr: errors.New("some random error")},
			wantErr: true,
		},
		{
			name: "success",
			iter: &fakeIter{
				scanInputs: [][]interface{}{
					{
						"domain_name_1",
						"domain_id_1",
						"domain_name_1",
						persistence.DomainStatusRegistered,
						"domain_description_1",
						"domain_owner_email_1",
						map[string]string{"k1": "v1"},
						int32(7),
						true,
						"test-archival-bucket",
						types.ArchivalStatusEnabled,
						types.ArchivalStatusEnabled,
						"test-history-archival-uri",
						types.ArchivalStatusEnabled,
						"test-visibility-archival-uri",
						[]byte("bad-binaries"),
						"thriftrw",
						[]byte("isolation-groups"),
						"thriftrw",
						[]byte("async-workflow-config"),
						"thriftrw",
						"test-active-cluster-name",
						[]map[string]interface{}{},
						true,
						int64(3),
						int64(4),
						int64(0),
						int64(-1),
						int64(1712167200000000000),
						int64(1712167200000000000),
						int64(7),
					},
				},
			},
			wantRows: []*nosqlplugin.DomainRow{
				{
					Info: &persistence.DomainInfo{
						ID:          "domain_id_1",
						Name:        "domain_name_1",
						Description: "domain_description_1",
						OwnerEmail:  "domain_owner_email_1",
						Data:        map[string]string{"k1": "v1"},
					},
					Config: &persistence.InternalDomainConfig{
						Retention:                7 * 24 * time.Hour,
						EmitMetric:               true,
						ArchivalBucket:           "test-archival-bucket",
						ArchivalStatus:           types.ArchivalStatusEnabled,
						HistoryArchivalStatus:    types.ArchivalStatusEnabled,
						HistoryArchivalURI:       "test-history-archival-uri",
						VisibilityArchivalStatus: types.ArchivalStatusEnabled,
						VisibilityArchivalURI:    "test-visibility-archival-uri",
						BadBinaries:              &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("bad-binaries")},
						IsolationGroups:          &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("isolation-groups")},
						AsyncWorkflowsConfig:     &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("async-workflow-config")},
					},
					ReplicationConfig: &persistence.DomainReplicationConfig{
						ActiveClusterName: "test-active-cluster-name",
						Clusters:          []*persistence.ClusterReplicationConfig{},
					},
					ConfigVersion:           3,
					FailoverVersion:         4,
					PreviousFailoverVersion: -1,
					FailoverEndTime:         &ts,
					NotificationVersion:     7,
					LastUpdatedTime:         ts,
					IsGlobalDomain:          true,
				},
			},
			wantQueries: []string{
				`SELECT name, domain.id, domain.name, domain.status, domain.description, domain.owner_email, domain.data, config.retention, config.emit_metric, config.archival_bucket, config.archival_status, config.history_archival_status, config.history_archival_uri, config.visibility_archival_status, config.visibility_archival_uri, config.bad_binaries, config.bad_binaries_encoding, config.isolation_groups, config.isolation_groups_encoding, config.async_workflow_config, config.async_workflow_config_encoding, replication_config.active_cluster_name, replication_config.clusters, is_global_domain, config_version, failover_version, failover_notification_version, previous_failover_version, failover_end_time, last_updated_time, notification_version FROM domains_by_name_v2 WHERE domains_partition = 0 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().PageSize(gomock.Any()).Return(query).Times(1)
			query.EXPECT().PageState(gomock.Any()).Return(query).Times(1)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			if tc.iter != nil {
				query.EXPECT().Iter().Return(tc.iter).Times(1)
			} else {
				query.EXPECT().Iter().Return(nil).Times(1)
			}

			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(cfg, session, logger, nil, dbWithClient(client))

			gotRows, _, err := db.SelectAllDomains(context.Background(), tc.pageSize, tc.pagetoken)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantRows, gotRows); diff != "" {
				t.Fatalf("Task rows mismatch (-want +got):\n%s", diff)
			}

			if !tc.iter.closed {
				t.Fatal("iterator not closed")
			}
		})
	}
}

func TestSelectDomainMetadata(t *testing.T) {
	tests := []struct {
		name         string
		queryMockFn  func(query *gocql.MockQuery)
		clientMockFn func(client *gocql.MockClient)
		wantNtfVer   int64
		wantQueries  []string
		wantErr      bool
	}{
		{
			name: "success",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					notificationVersion := args[0].(*int64)
					*notificationVersion = 3
					return nil
				}).Times(1)
			},
			wantNtfVer: 3,
			wantQueries: []string{
				`SELECT notification_version FROM domains_by_name_v2 WHERE domains_partition = 0 and name = cadence-domain-metadata `,
			},
		},
		{
			name: "scan failure - isnotfound",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return errors.New("some error that will be considered as not found by client mock")
				}).Times(1)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			wantNtfVer: 0,
			wantQueries: []string{
				`SELECT notification_version FROM domains_by_name_v2 WHERE domains_partition = 0 and name = cadence-domain-metadata `,
			},
		},
		{
			name: "scan failure",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return errors.New("some random error")
				}).Times(1)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsNotFoundError(gomock.Any()).Return(false).Times(1)
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
			if tc.clientMockFn != nil {
				tc.clientMockFn(client)
			}
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			gotNtfVer, err := db.SelectDomainMetadata(context.Background())

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if gotNtfVer != tc.wantNtfVer {
				t.Errorf("Got notification version = %v, want %v", gotNtfVer, tc.wantNtfVer)
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeleteDomain(t *testing.T) {
	tests := []struct {
		name         string
		domainID     *string
		domainName   *string
		queryMockFn  func(query *gocql.MockQuery)
		clientMockFn func(client *gocql.MockClient)
		wantQueries  []string
		wantErr      bool
	}{
		{
			name:       "both domainName and domainID not provided",
			domainName: nil,
			domainID:   nil,
			wantErr:    true,
		},
		{
			name:       "domainName not provided",
			domainID:   common.StringPtr("domain_id_1"),
			domainName: nil,
			queryMockFn: func(query *gocql.MockQuery) {
				// mock calls for initial select
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					name := args[0].(*string)
					*name = "domain_name_1"
					return nil
				}).Times(1)

				// mock calls for delete from domains_by_name_v2
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)

				// mock calls for delete from domains
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantQueries: []string{
				`SELECT domain.name FROM domains WHERE id = domain_id_1`,
				`DELETE FROM domains_by_name_v2 WHERE domains_partition = 0 and name = domain_name_1`,
				`DELETE FROM domains WHERE id = domain_id_1`,
			},
		},
		{
			name:       "domainName not provided - scan failure - isnotfound",
			domainID:   common.StringPtr("domain_id_1"),
			domainName: nil,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return errors.New("some error that will be considered as not found by client mock")
				}).Times(1)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			wantQueries: []string{
				`SELECT domain.name FROM domains WHERE id = domain_id_1`,
			},
		},
		{
			name:       "domainName not provided - scan failure",
			domainID:   common.StringPtr("domain_id_1"),
			domainName: nil,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return errors.New("some random error")
				}).Times(1)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsNotFoundError(gomock.Any()).Return(false).Times(1)
			},
			wantErr: true,
		},
		{
			name:       "domainID not provided",
			domainID:   nil,
			domainName: common.StringPtr("domain_name_1"),
			queryMockFn: func(query *gocql.MockQuery) {
				// mock calls for initial select
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				// Ideally we should be using DoAndReturn here to set the domainID,
				// but it panics because gomock doesn't handle n-arity func calls with nil params such as query.Scan(&id, nil, nil)
				query.EXPECT().Scan(gomock.Any()).Return(nil).Times(1)

				// mock calls for delete from domains_by_name_v2
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)

				// mock calls for delete from domains
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantQueries: []string{
				`SELECT domain.id, domain.name, domain.status, domain.description, domain.owner_email, domain.data, config.retention, config.emit_metric, config.archival_bucket, config.archival_status, config.history_archival_status, config.history_archival_uri, config.visibility_archival_status, config.visibility_archival_uri, config.bad_binaries, config.bad_binaries_encoding, replication_config.active_cluster_name, replication_config.clusters, config.isolation_groups,config.isolation_groups_encoding,config.async_workflow_config,config.async_workflow_config_encoding,is_global_domain, config_version, failover_version, failover_notification_version, previous_failover_version, failover_end_time, last_updated_time, notification_version FROM domains_by_name_v2 WHERE domains_partition = 0 and name = domain_name_1`,
				`DELETE FROM domains_by_name_v2 WHERE domains_partition = 0 and name = domain_name_1`,
				`DELETE FROM domains WHERE id = `, // domainID is nil, so we expect an empty string here. See the comment above inside mockQueryFn.
			},
		},
		{
			name:       "domainID not provided - scan failure - isnotfound",
			domainID:   nil,
			domainName: common.StringPtr("domain_name_1"),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).Return(errors.New("some error that will be considered as not found by client mock")).Times(1)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			wantQueries: []string{
				`SELECT domain.id, domain.name, domain.status, domain.description, domain.owner_email, domain.data, config.retention, config.emit_metric, config.archival_bucket, config.archival_status, config.history_archival_status, config.history_archival_uri, config.visibility_archival_status, config.visibility_archival_uri, config.bad_binaries, config.bad_binaries_encoding, replication_config.active_cluster_name, replication_config.clusters, config.isolation_groups,config.isolation_groups_encoding,config.async_workflow_config,config.async_workflow_config_encoding,is_global_domain, config_version, failover_version, failover_notification_version, previous_failover_version, failover_end_time, last_updated_time, notification_version FROM domains_by_name_v2 WHERE domains_partition = 0 and name = domain_name_1`,
			},
		},
		{
			name:       "domainID not provided - scan failure",
			domainID:   nil,
			domainName: common.StringPtr("domain_name_1"),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).Return(errors.New("some random error")).Times(1)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsNotFoundError(gomock.Any()).Return(false).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			if tc.queryMockFn != nil {
				tc.queryMockFn(query)
			}
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			if tc.clientMockFn != nil {
				tc.clientMockFn(client)
			}
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{
				EnableCassandraAllConsistencyLevelDelete: func(opts ...dynamicconfig.FilterOption) bool {
					return false
				},
			}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			err := db.DeleteDomain(context.Background(), tc.domainID, tc.domainName)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
