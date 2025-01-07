// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package nosql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

func testFixtureDomainInfo() *persistence.DomainInfo {
	return &persistence.DomainInfo{
		ID:          "domain-id",
		Name:        "domain-name",
		Status:      1,
		Description: "domain-desc",
		OwnerEmail:  "owner@test.com",
		Data:        map[string]string{"test": "a"},
	}
}

func testFixtureInternalDomainConfig() *persistence.InternalDomainConfig {
	return &persistence.InternalDomainConfig{
		Retention:                time.Hour,
		HistoryArchivalStatus:    types.ArchivalStatus(1),
		HistoryArchivalURI:       "s3://test-history-archival",
		VisibilityArchivalStatus: types.ArchivalStatus(2),
		VisibilityArchivalURI:    "s3://test-visibility-archival",
		BadBinaries: &persistence.DataBlob{
			Encoding: "base64",
			Data:     []byte("bad-binaries"),
		},
		IsolationGroups: &persistence.DataBlob{
			Encoding: "base64",
			Data:     []byte("isolation-groups"),
		},
		AsyncWorkflowsConfig: &persistence.DataBlob{
			Encoding: "base64",
			Data:     []byte("async-workflows"),
		},
	}
}

func testFixtureDomainReplicationConfig() *persistence.DomainReplicationConfig {
	return &persistence.DomainReplicationConfig{
		ActiveClusterName: "cluster-1",
		Clusters: []*persistence.ClusterReplicationConfig{
			{
				ClusterName: "cluster-1",
			},
			{
				ClusterName: "cluster-2",
			},
		},
	}
}

func setUpMocksForDomainStore(t *testing.T) (*nosqlDomainStore, *nosqlplugin.MockDB, *MockshardedNosqlStore) {
	ctrl := gomock.NewController(t)
	dbMock := nosqlplugin.NewMockDB(ctrl)
	shardedNoSQLStoreMock := NewMockshardedNosqlStore(ctrl)

	shardedStore := nosqlStore{
		db: dbMock,
	}

	shardedNoSQLStoreMock.EXPECT().GetDefaultShard().Return(shardedStore).AnyTimes()

	domainStore := &nosqlDomainStore{
		nosqlStore:         shardedNoSQLStoreMock.GetDefaultShard(),
		currentClusterName: "test-cluster",
	}

	return domainStore, dbMock, shardedNoSQLStoreMock
}

func TestCreateDomain(t *testing.T) {
	testCases := []struct {
		name        string
		setupMock   func(*nosqlplugin.MockDB)
		request     *persistence.InternalCreateDomainRequest
		expectError bool
		expectedID  string
	}{
		{
			name: "success",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertDomain(gomock.Any(), &nosqlplugin.DomainRow{
					Info:                        testFixtureDomainInfo(),
					Config:                      testFixtureInternalDomainConfig(),
					ReplicationConfig:           testFixtureDomainReplicationConfig(),
					ConfigVersion:               1,
					FailoverVersion:             2,
					FailoverNotificationVersion: persistence.InitialFailoverNotificationVersion,
					PreviousFailoverVersion:     common.InitialPreviousFailoverVersion,
					IsGlobalDomain:              true,
					LastUpdatedTime:             time.Unix(1, 2),
				}).Return(nil).Times(1)
			},
			request: &persistence.InternalCreateDomainRequest{
				Info:              testFixtureDomainInfo(),
				Config:            testFixtureInternalDomainConfig(),
				ReplicationConfig: testFixtureDomainReplicationConfig(),
				ConfigVersion:     1,
				FailoverVersion:   2,
				IsGlobalDomain:    true,
				LastUpdatedTime:   time.Unix(1, 2),
			},
			expectError: false,
			expectedID:  "domain-id",
		},
		{
			name: "domain already exists error",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertDomain(gomock.Any(), gomock.Any()).Return(&types.DomainAlreadyExistsError{}).Times(1)
			},
			request: &persistence.InternalCreateDomainRequest{
				Info:              testFixtureDomainInfo(),
				Config:            testFixtureInternalDomainConfig(),
				ReplicationConfig: testFixtureDomainReplicationConfig(),
				IsGlobalDomain:    true,
				LastUpdatedTime:   time.Now(),
			},
			expectError: true,
		},
		{
			name: "common error",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertDomain(gomock.Any(), gomock.Any()).Return(errors.New("test error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			request: &persistence.InternalCreateDomainRequest{
				Info:              testFixtureDomainInfo(),
				Config:            testFixtureInternalDomainConfig(),
				ReplicationConfig: testFixtureDomainReplicationConfig(),
				IsGlobalDomain:    true,
				LastUpdatedTime:   time.Now(),
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks for each test case individually
			store, dbMock, _ := setUpMocksForDomainStore(t)

			// Setup test-specific mock behavior
			tc.setupMock(dbMock)

			// Execute the method under test
			resp, err := store.CreateDomain(context.Background(), tc.request)

			// Validate results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedID, resp.ID)
			}
		})
	}
}

func TestUpdateDomain(t *testing.T) {
	testCases := []struct {
		name        string
		setupMock   func(*nosqlplugin.MockDB)
		request     *persistence.InternalUpdateDomainRequest
		expectError bool
	}{
		{
			name: "success",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().UpdateDomain(gomock.Any(), &nosqlplugin.DomainRow{
					Info:                        testFixtureDomainInfo(),
					Config:                      testFixtureInternalDomainConfig(),
					ReplicationConfig:           testFixtureDomainReplicationConfig(),
					ConfigVersion:               1,
					FailoverVersion:             2,
					FailoverNotificationVersion: 3,
					PreviousFailoverVersion:     4,
					NotificationVersion:         5,
					FailoverEndTime:             common.Ptr(time.Unix(2, 3)),
					LastUpdatedTime:             time.Unix(1, 2),
				}).Return(nil).Times(1)
			},
			request: &persistence.InternalUpdateDomainRequest{
				Info:                        testFixtureDomainInfo(),
				Config:                      testFixtureInternalDomainConfig(),
				ReplicationConfig:           testFixtureDomainReplicationConfig(),
				ConfigVersion:               1,
				FailoverVersion:             2,
				FailoverNotificationVersion: 3,
				PreviousFailoverVersion:     4,
				NotificationVersion:         5,
				FailoverEndTime:             common.Ptr(time.Unix(2, 3)),
				LastUpdatedTime:             time.Unix(1, 2),
			},
			expectError: false,
		},
		{
			name: "common error",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(errors.New("test error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			request: &persistence.InternalUpdateDomainRequest{
				Info:                        testFixtureDomainInfo(),
				Config:                      testFixtureInternalDomainConfig(),
				ReplicationConfig:           testFixtureDomainReplicationConfig(),
				ConfigVersion:               1,
				FailoverVersion:             2,
				FailoverNotificationVersion: 3,
				PreviousFailoverVersion:     4,
				NotificationVersion:         5,
				FailoverEndTime:             common.Ptr(time.Unix(2, 3)),
				LastUpdatedTime:             time.Unix(1, 2),
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks for each test case individually
			store, dbMock, _ := setUpMocksForDomainStore(t)

			// Setup test-specific mock behavior
			tc.setupMock(dbMock)

			// Execute the method under test
			err := store.UpdateDomain(context.Background(), tc.request)

			// Validate results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetDomain(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*nosqlplugin.MockDB)
		request       *persistence.GetDomainRequest
		expectError   bool
		expected      *persistence.InternalGetDomainResponse
		expectedError string
	}{
		{
			name: "success by ID",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectDomain(gomock.Any(), common.Ptr("test-domain-id"), nil).Return(&nosqlplugin.DomainRow{
					Info:                        testFixtureDomainInfo(),
					Config:                      testFixtureInternalDomainConfig(),
					ReplicationConfig:           testFixtureDomainReplicationConfig(),
					ConfigVersion:               1,
					FailoverVersion:             2,
					FailoverNotificationVersion: 3,
					PreviousFailoverVersion:     4,
					NotificationVersion:         5,
					FailoverEndTime:             common.Ptr(time.Unix(2, 3)),
					LastUpdatedTime:             time.Unix(1, 2),
				}, nil).Times(1)
			},
			request:     &persistence.GetDomainRequest{ID: "test-domain-id"},
			expectError: false,
			expected: &persistence.InternalGetDomainResponse{
				Info:                        testFixtureDomainInfo(),
				Config:                      testFixtureInternalDomainConfig(),
				ReplicationConfig:           testFixtureDomainReplicationConfig(),
				ConfigVersion:               1,
				FailoverVersion:             2,
				FailoverNotificationVersion: 3,
				PreviousFailoverVersion:     4,
				NotificationVersion:         5,
				FailoverEndTime:             common.Ptr(time.Unix(2, 3)),
				LastUpdatedTime:             time.Unix(1, 2),
			},
		},
		{
			name: "success by name",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectDomain(gomock.Any(), nil, common.Ptr("test-domain")).Return(&nosqlplugin.DomainRow{
					Info:                        testFixtureDomainInfo(),
					Config:                      testFixtureInternalDomainConfig(),
					ReplicationConfig:           testFixtureDomainReplicationConfig(),
					ConfigVersion:               1,
					FailoverVersion:             2,
					FailoverNotificationVersion: 3,
					PreviousFailoverVersion:     4,
					NotificationVersion:         5,
					FailoverEndTime:             common.Ptr(time.Unix(2, 3)),
					LastUpdatedTime:             time.Unix(1, 2),
				}, nil).Times(1)
			},
			request:     &persistence.GetDomainRequest{Name: "test-domain"},
			expectError: false,
			expected: &persistence.InternalGetDomainResponse{
				Info:                        testFixtureDomainInfo(),
				Config:                      testFixtureInternalDomainConfig(),
				ReplicationConfig:           testFixtureDomainReplicationConfig(),
				ConfigVersion:               1,
				FailoverVersion:             2,
				FailoverNotificationVersion: 3,
				PreviousFailoverVersion:     4,
				NotificationVersion:         5,
				FailoverEndTime:             common.Ptr(time.Unix(2, 3)),
				LastUpdatedTime:             time.Unix(1, 2),
			},
		},
		{
			name: "bad request error - both id and domain are set",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				// No database call should be made
			},
			request:       &persistence.GetDomainRequest{ID: "test-id", Name: "test-name"},
			expectError:   true,
			expectedError: "GetDomain operation failed.  Both ID and Name specified in request.",
		},
		{
			name: "bad request error - both id and domain are empty",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				// No database call should be made
			},
			request:       &persistence.GetDomainRequest{},
			expectError:   true,
			expectedError: "GetDomain operation failed.  Both ID and Name are empty.",
		},
		{
			name: "not found error - by name",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectDomain(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			request:       &persistence.GetDomainRequest{Name: "test-domain"},
			expectError:   true,
			expectedError: "Domain test-domain does not exist.",
		},
		{
			name: "not found error - by id",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectDomain(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			request:       &persistence.GetDomainRequest{ID: "test-domain-id"},
			expectError:   true,
			expectedError: "Domain test-domain-id does not exist.",
		},
		{
			name: "generic db error",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectDomain(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("test error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(false).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			request:       &persistence.GetDomainRequest{ID: "test-domain-id"},
			expectError:   true,
			expectedError: "GetDomain failed. Error: test error ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks for each test case individually
			store, dbMock, _ := setUpMocksForDomainStore(t)

			// Setup test-specific mock behavior
			tc.setupMock(dbMock)

			// Execute the method under test
			resp, err := store.GetDomain(context.Background(), tc.request)

			// Validate results
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, resp)
			}
		})
	}
}

func TestListDomains(t *testing.T) {
	testCases := []struct {
		name        string
		setupMock   func(*nosqlplugin.MockDB)
		request     *persistence.ListDomainsRequest
		expectError bool
		expected    *persistence.InternalListDomainsResponse
	}{
		{
			name: "success",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectAllDomains(gomock.Any(), 1, []byte("token")).Return([]*nosqlplugin.DomainRow{
					{
						Info:              &persistence.DomainInfo{ID: "domain-id-1"},
						ReplicationConfig: testFixtureDomainReplicationConfig(),
					},
				}, []byte("next-token"), nil).Times(1)
			},
			request: &persistence.ListDomainsRequest{
				PageSize:      1,
				NextPageToken: []byte("token"),
			},
			expectError: false,
			expected: &persistence.InternalListDomainsResponse{
				Domains: []*persistence.InternalGetDomainResponse{
					{
						Info:              &persistence.DomainInfo{ID: "domain-id-1", Data: map[string]string{}},
						ReplicationConfig: testFixtureDomainReplicationConfig(),
					},
				},
				NextPageToken: []byte("next-token"),
			},
		},
		{
			name: "common error",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectAllDomains(gomock.Any(), 10, []byte("token")).Return(nil, nil, errors.New("test error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			request: &persistence.ListDomainsRequest{
				PageSize:      10,
				NextPageToken: []byte("token"),
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks for each test case individually
			store, dbMock, _ := setUpMocksForDomainStore(t)

			// Setup test-specific mock behavior
			tc.setupMock(dbMock)

			// Execute the method under test
			resp, err := store.ListDomains(context.Background(), tc.request)

			// Validate results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, resp)
			}
		})
	}
}

func TestGetMetadata(t *testing.T) {
	testCases := []struct {
		name            string
		setupMock       func(*nosqlplugin.MockDB)
		expectError     bool
		expectedVersion int64
	}{
		{
			name: "success",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectDomainMetadata(gomock.Any()).Return(int64(10), nil).Times(1)
			},
			expectError:     false,
			expectedVersion: 10,
		},
		{
			name: "common error",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectDomainMetadata(gomock.Any()).Return(int64(0), errors.New("test error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks for each test case individually
			store, dbMock, _ := setUpMocksForDomainStore(t)

			// Setup test-specific mock behavior
			tc.setupMock(dbMock)

			// Execute the method under test
			resp, err := store.GetMetadata(context.Background())

			// Validate results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedVersion, resp.NotificationVersion)
			}
		})
	}
}
