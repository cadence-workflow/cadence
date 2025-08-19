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
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

func TestCDBBasics(t *testing.T) {
	ctrl := gomock.NewController(t)
	session := gocql.NewMockSession(ctrl)
	client := gocql.NewMockClient(ctrl)
	cfg := &config.NoSQL{}
	logger := testlogger.New(t)
	dc := &persistence.DynamicConfiguration{}

	db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

	if db.PluginName() != PluginName {
		t.Errorf("got plugin name: %v but want %v", db.PluginName(), PluginName)
	}

	client.EXPECT().IsNotFoundError(nil).Return(true).Times(1)
	if db.IsNotFoundError(nil) != true {
		t.Errorf("IsNotFoundError returned false but want true")
	}

	client.EXPECT().IsTimeoutError(nil).Return(true).Times(1)
	if db.IsTimeoutError(nil) != true {
		t.Errorf("IsTimeoutError returned false but want true")
	}

	client.EXPECT().IsThrottlingError(nil).Return(true).Times(1)
	if db.IsThrottlingError(nil) != true {
		t.Errorf("IsNotFoundError returned false but want true")
	}

	client.EXPECT().IsDBUnavailableError(nil).Return(true).Times(1)
	if db.IsDBUnavailableError(nil) != true {
		t.Errorf("IsDBUnavailableError returned false but want true")
	}

	client.EXPECT().IsCassandraConsistencyError(nil).Return(true).Times(1)
	if db.isCassandraConsistencyError(nil) != true {
		t.Errorf("IsNotFoundError returned false but want true")
	}

	session.EXPECT().Close().Times(1)
	db.Close()
}

func TestExecuteWithConsistencyAll(t *testing.T) {
	tests := []struct {
		name                            string
		enableAllConsistencyLevelDelete bool
		queryMockPrep                   func(*gocql.MockQuery)
		clientMockPrep                  func(*gocql.MockClient)
		wantErr                         bool
	}{
		{
			name:                            "all consistency level delete disabled - success",
			enableAllConsistencyLevelDelete: false,
			queryMockPrep: func(query *gocql.MockQuery) {
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:                            "all consistency level delete disabled - failed",
			enableAllConsistencyLevelDelete: false,
			queryMockPrep: func(query *gocql.MockQuery) {
				query.EXPECT().Exec().Return(errors.New("failed")).Times(1)
			},
			wantErr: true,
		},
		{
			name:                            "all consistency level delete enabled - query consistency level all - success",
			enableAllConsistencyLevelDelete: true,
			queryMockPrep: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(cassandraAllConslevel).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:                            "all consistency level delete enabled - query consistency level all - returns consistency error - success",
			enableAllConsistencyLevelDelete: true,
			queryMockPrep: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(cassandraAllConslevel).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("fail for the first call")).Times(1)

				query.EXPECT().Consistency(cassandraDefaultConsLevel).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			clientMockPrep: func(client *gocql.MockClient) {
				client.EXPECT().IsCassandraConsistencyError(gomock.Any()).Return(true).Times(1)
			},
			wantErr: false,
		},
		{
			name:                            "all consistency level delete enabled - query consistency level all - returns consistency error - failed",
			enableAllConsistencyLevelDelete: true,
			queryMockPrep: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(cassandraAllConslevel).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("fail for the first call")).Times(1)

				query.EXPECT().Consistency(cassandraDefaultConsLevel).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("fail for the 2nd call too")).Times(1)
			},
			clientMockPrep: func(client *gocql.MockClient) {
				client.EXPECT().IsCassandraConsistencyError(gomock.Any()).Return(true).Times(1)
			},
			wantErr: true,
		},
		{
			name:                            "all consistency level delete enabled - query consistency level all - returns non-consistency error - failed",
			enableAllConsistencyLevelDelete: true,
			queryMockPrep: func(query *gocql.MockQuery) {
				query.EXPECT().Consistency(cassandraAllConslevel).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed")).Times(1)
			},
			clientMockPrep: func(client *gocql.MockClient) {
				client.EXPECT().IsCassandraConsistencyError(gomock.Any()).Return(false).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			session := gocql.NewMockSession(ctrl)
			client := gocql.NewMockClient(ctrl)
			query := gocql.NewMockQuery(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{
				EnableCassandraAllConsistencyLevelDelete: func(opts ...dynamicproperties.FilterOption) bool {
					return tc.enableAllConsistencyLevelDelete
				},
			}

			if tc.queryMockPrep != nil {
				tc.queryMockPrep(query)
			}
			if tc.clientMockPrep != nil {
				tc.clientMockPrep(client)
			}

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			err := db.executeWithConsistencyAll(query)
			if (err != nil) != tc.wantErr {
				t.Errorf("got err: %v but wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestExecuteBatchWithConsistencyAll(t *testing.T) {
	tests := []struct {
		name                            string
		enableAllConsistencyLevelDelete bool
		sessionMockPrep                 func(*gocql.MockSession)
		batchMockPrep                   func(*gocql.MockBatch)
		clientMockPrep                  func(*gocql.MockClient)
		wantErr                         bool
	}{
		{
			name:                            "all consistency level delete disabled - success",
			enableAllConsistencyLevelDelete: false,
			sessionMockPrep: func(session *gocql.MockSession) {
				session.EXPECT().ExecuteBatch(gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:                            "all consistency level delete disabled - failed",
			enableAllConsistencyLevelDelete: false,
			sessionMockPrep: func(session *gocql.MockSession) {
				session.EXPECT().ExecuteBatch(gomock.Any()).Return(errors.New("failed")).Times(1)
			},
			wantErr: true,
		},
		{
			name:                            "all consistency level delete enabled - success",
			enableAllConsistencyLevelDelete: true,
			sessionMockPrep: func(session *gocql.MockSession) {
				session.EXPECT().ExecuteBatch(gomock.Any()).Return(nil).Times(1)
			},
			batchMockPrep: func(batch *gocql.MockBatch) {
				batch.EXPECT().Consistency(cassandraAllConslevel).Return(batch).Times(1)
			},
			wantErr: false,
		},
		{
			name:                            "all consistency level delete enabled - executebatch(all) failed with consistency err - success",
			enableAllConsistencyLevelDelete: true,
			sessionMockPrep: func(session *gocql.MockSession) {
				session.EXPECT().ExecuteBatch(gomock.Any()).Return(errors.New("fail the first call")).Times(1)
				// second call succeeds
				session.EXPECT().ExecuteBatch(gomock.Any()).Return(nil).Times(1)
			},
			clientMockPrep: func(client *gocql.MockClient) {
				client.EXPECT().IsCassandraConsistencyError(gomock.Any()).Return(true).Times(1)
			},
			batchMockPrep: func(batch *gocql.MockBatch) {
				batch.EXPECT().Consistency(cassandraAllConslevel).Return(batch).Times(1)
				batch.EXPECT().Consistency(cassandraDefaultConsLevel).Return(batch).Times(1)
			},
			wantErr: false,
		},
		{
			name:                            "all consistency level delete enabled - executebatch(all) failed with non-consistency err - failed",
			enableAllConsistencyLevelDelete: true,
			sessionMockPrep: func(session *gocql.MockSession) {
				session.EXPECT().ExecuteBatch(gomock.Any()).Return(errors.New("fail the first call")).Times(1)
			},
			clientMockPrep: func(client *gocql.MockClient) {
				client.EXPECT().IsCassandraConsistencyError(gomock.Any()).Return(false).Times(1)
			},
			batchMockPrep: func(batch *gocql.MockBatch) {
				batch.EXPECT().Consistency(cassandraAllConslevel).Return(batch).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			session := gocql.NewMockSession(ctrl)
			client := gocql.NewMockClient(ctrl)
			batch := gocql.NewMockBatch(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{
				EnableCassandraAllConsistencyLevelDelete: func(opts ...dynamicproperties.FilterOption) bool {
					return tc.enableAllConsistencyLevelDelete
				},
			}

			if tc.sessionMockPrep != nil {
				tc.sessionMockPrep(session)
			}
			if tc.batchMockPrep != nil {
				tc.batchMockPrep(batch)
			}
			if tc.clientMockPrep != nil {
				tc.clientMockPrep(client)
			}

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			err := db.executeBatchWithConsistencyAll(batch)
			if (err != nil) != tc.wantErr {
				t.Errorf("got err: %v but wantErr: %v", err, tc.wantErr)
			}
		})
	}
}
