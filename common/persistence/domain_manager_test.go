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

package persistence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

func testFixtureDomainInfo() *DomainInfo {
	return &DomainInfo{
		ID:          "domain-id",
		Name:        "domain-name",
		Status:      1,
		Description: "domain-desc",
		OwnerEmail:  "owner@test.com",
		Data:        map[string]string{"test": "a"},
	}
}

func testFixtureDomainConfig() *DomainConfig {
	return &DomainConfig{
		Retention:                1,
		EmitMetric:               true,
		HistoryArchivalStatus:    types.ArchivalStatusEnabled,
		HistoryArchivalURI:       "s3://abc",
		VisibilityArchivalStatus: types.ArchivalStatusEnabled,
		VisibilityArchivalURI:    "s3://xyz",
		BadBinaries:              types.BadBinaries{},
		IsolationGroups:          map[string]types.IsolationGroupPartition{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}},
		AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
			Enabled:             true,
			PredefinedQueueName: "q",
			QueueType:           "kafka",
		},
	}
}

func testFixtureDomainReplicationConfig() *DomainReplicationConfig {
	return &DomainReplicationConfig{
		ActiveClusterName: "cluster-1",
		Clusters: []*ClusterReplicationConfig{
			{
				ClusterName: "cluster-1",
			},
			{
				ClusterName: "cluster-2",
			},
		},
	}
}

func testFixtureDomainReplicationConfigWithActiveClusters() *DomainReplicationConfig {
	return &DomainReplicationConfig{
		Clusters: []*ClusterReplicationConfig{
			{
				ClusterName: "cluster-1",
			},
			{
				ClusterName: "cluster-2",
			},
		},
		ActiveClusters: &types.ActiveClusters{
			AttributeScopes: map[string]types.ClusterAttributeScope{
				"region": {
					ClusterAttributes: map[string]types.ActiveClusterInfo{
						"region-1": {
							ActiveClusterName: "cluster-1",
							FailoverVersion:   1,
						},
						"region-2": {
							ActiveClusterName: "cluster-2",
							FailoverVersion:   2,
						},
					},
				},
			},
		},
	}
}

func testFixtureDomainReplicationConfigInternal() *InternalDomainReplicationConfig {
	return &InternalDomainReplicationConfig{
		ActiveClusterName: "cluster-1",
		Clusters: []*ClusterReplicationConfig{
			{
				ClusterName: "cluster-1",
			},
			{
				ClusterName: "cluster-2",
			},
		},
	}
}

func testFixtureDomainReplicationConfigInternalWithActiveClusters() *InternalDomainReplicationConfig {
	return &InternalDomainReplicationConfig{
		Clusters: []*ClusterReplicationConfig{
			{
				ClusterName: "cluster-1",
			},
			{
				ClusterName: "cluster-2",
			},
		},
		ActiveClustersConfig: &DataBlob{Encoding: constants.EncodingTypeThriftRWSnappy, Data: []byte("active-clusters-config")},
	}
}

func setUpMocksForDomainManager(t *testing.T) (*domainManagerImpl, *MockDomainStore, *MockPayloadSerializer) {
	ctrl := gomock.NewController(t)
	mockStore := NewMockDomainStore(ctrl)
	mockSerializer := NewMockPayloadSerializer(ctrl)
	logger := log.NewNoop()

	domainManager := NewDomainManagerImpl(mockStore, logger, mockSerializer, &DynamicConfiguration{SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW))}).(*domainManagerImpl)

	return domainManager, mockStore, mockSerializer
}

func TestCreateDomain(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockDomainStore, *MockPayloadSerializer)
		request       *CreateDomainRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					SerializeBadBinaries(&types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeIsolationGroups(&types.IsolationGroupConfiguration{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeAsyncWorkflowsConfig(&types.AsyncWorkflowConfiguration{Enabled: true, PredefinedQueueName: "q", QueueType: "kafka"}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeActiveClusters(nil, constants.EncodingTypeThriftRWSnappy).
					Return(nil, nil).Times(1)

				expectedReq := &InternalCreateDomainRequest{
					Info: testFixtureDomainInfo(),
					Config: &InternalDomainConfig{
						Retention:                common.DaysToDuration(1),
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusEnabled,
						HistoryArchivalURI:       "s3://abc",
						VisibilityArchivalStatus: types.ArchivalStatusEnabled,
						VisibilityArchivalURI:    "s3://xyz",
						BadBinaries:              &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")},
						IsolationGroups:          &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")},
						AsyncWorkflowsConfig:     &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")},
					},
					ReplicationConfig: testFixtureDomainReplicationConfigInternal(),
					IsGlobalDomain:    true,
					ConfigVersion:     1,
					FailoverVersion:   10,
					LastUpdatedTime:   time.Unix(0, 100),
					CurrentTimeStamp:  time.Now(),
				}

				mockStore.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, actualRequest *InternalCreateDomainRequest) (*CreateDomainResponse, error) {
						assert.WithinDuration(t, expectedReq.CurrentTimeStamp, actualRequest.CurrentTimeStamp, time.Second)
						return nil, nil
					})
			},
			request: &CreateDomainRequest{
				Info:              testFixtureDomainInfo(),
				Config:            testFixtureDomainConfig(),
				ReplicationConfig: testFixtureDomainReplicationConfig(),
				IsGlobalDomain:    true,
				ConfigVersion:     1,
				FailoverVersion:   10,
				LastUpdatedTime:   100,
			},
			expectError: false,
		},
		{
			name: "success with activeclusters config populated",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					SerializeBadBinaries(&types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeIsolationGroups(&types.IsolationGroupConfiguration{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeAsyncWorkflowsConfig(&types.AsyncWorkflowConfiguration{Enabled: true, PredefinedQueueName: "q", QueueType: "kafka"}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeActiveClusters(testFixtureDomainReplicationConfigWithActiveClusters().ActiveClusters, constants.EncodingTypeThriftRWSnappy).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRWSnappy, Data: []byte("active-clusters-config")}, nil).Times(1)

				expectedReq := &InternalCreateDomainRequest{
					Info: testFixtureDomainInfo(),
					Config: &InternalDomainConfig{
						Retention:                common.DaysToDuration(1),
						EmitMetric:               true,
						HistoryArchivalStatus:    types.ArchivalStatusEnabled,
						HistoryArchivalURI:       "s3://abc",
						VisibilityArchivalStatus: types.ArchivalStatusEnabled,
						VisibilityArchivalURI:    "s3://xyz",
						BadBinaries:              &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")},
						IsolationGroups:          &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")},
						AsyncWorkflowsConfig:     &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")},
					},
					ReplicationConfig: testFixtureDomainReplicationConfigInternalWithActiveClusters(),
					IsGlobalDomain:    true,
					ConfigVersion:     1,
					FailoverVersion:   10,
					LastUpdatedTime:   time.Unix(0, 100),
					CurrentTimeStamp:  time.Now(),
				}

				mockStore.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, actualRequest *InternalCreateDomainRequest) (*CreateDomainResponse, error) {
						assert.WithinDuration(t, expectedReq.CurrentTimeStamp, actualRequest.CurrentTimeStamp, time.Second)
						return nil, nil
					})
			},
			request: &CreateDomainRequest{
				Info:              testFixtureDomainInfo(),
				Config:            testFixtureDomainConfig(),
				ReplicationConfig: testFixtureDomainReplicationConfigWithActiveClusters(),
				IsGlobalDomain:    true,
				ConfigVersion:     1,
				FailoverVersion:   10,
				LastUpdatedTime:   100,
			},
			expectError: false,
		},
		{
			name: "serialization error",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					SerializeBadBinaries(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("serialization error")).Times(1)
			},
			request: &CreateDomainRequest{
				Info:              &DomainInfo{ID: "domain1"},
				Config:            &DomainConfig{},
				ReplicationConfig: &DomainReplicationConfig{},
				IsGlobalDomain:    true,
				ConfigVersion:     1,
				FailoverVersion:   1,
				LastUpdatedTime:   time.Now().UnixNano(),
			},
			expectError:   true,
			expectedError: "serialization error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			domainManager, mockStore, mockSerializer := setUpMocksForDomainManager(t)

			tc.setupMock(mockStore, mockSerializer)

			_, err := domainManager.CreateDomain(context.Background(), tc.request)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetDomain(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockDomainStore, *MockPayloadSerializer)
		request       *GetDomainRequest
		expectError   bool
		expectedError string
		expected      *GetDomainResponse
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockStore.EXPECT().
					GetDomain(gomock.Any(), &GetDomainRequest{ID: "domain1"}).
					Return(&InternalGetDomainResponse{
						Info: testFixtureDomainInfo(),
						Config: &InternalDomainConfig{
							Retention:                common.DaysToDuration(1),
							EmitMetric:               true,
							HistoryArchivalStatus:    types.ArchivalStatusEnabled,
							HistoryArchivalURI:       "s3://abc",
							VisibilityArchivalStatus: types.ArchivalStatusEnabled,
							VisibilityArchivalURI:    "s3://xyz",
							BadBinaries:              &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")},
							IsolationGroups:          &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")},
							AsyncWorkflowsConfig:     &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")},
						},
						ReplicationConfig: testFixtureDomainReplicationConfigInternal(),
						IsGlobalDomain:    true,
						ConfigVersion:     1,
						FailoverVersion:   10,
						LastUpdatedTime:   time.Unix(0, 100),
						FailoverEndTime:   common.Ptr(time.Unix(0, 200)),
					}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeBadBinaries(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")}).
					Return(&types.BadBinaries{}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeIsolationGroups(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")}).
					Return(&types.IsolationGroupConfiguration{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeAsyncWorkflowsConfig(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")}).
					Return(&types.AsyncWorkflowConfiguration{}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeActiveClusters(nil).
					Return(nil, nil).Times(1)
			},
			request:     &GetDomainRequest{ID: "domain1"},
			expectError: false,
			expected: &GetDomainResponse{
				Info: testFixtureDomainInfo(),
				Config: &DomainConfig{
					Retention:                1,
					EmitMetric:               true,
					HistoryArchivalStatus:    types.ArchivalStatusEnabled,
					HistoryArchivalURI:       "s3://abc",
					VisibilityArchivalStatus: types.ArchivalStatusEnabled,
					VisibilityArchivalURI:    "s3://xyz",
					BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
					IsolationGroups:          map[string]types.IsolationGroupPartition{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}},
					AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{},
				},
				ReplicationConfig: testFixtureDomainReplicationConfig(),
				IsGlobalDomain:    true,
				ConfigVersion:     1,
				FailoverVersion:   10,
				LastUpdatedTime:   100,
				FailoverEndTime:   common.Ptr(time.Unix(0, 200).UnixNano()),
			},
		},
		{
			name: "success with activeclusters config populated",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockStore.EXPECT().
					GetDomain(gomock.Any(), &GetDomainRequest{ID: "domain1"}).
					Return(&InternalGetDomainResponse{
						Info: testFixtureDomainInfo(),
						Config: &InternalDomainConfig{
							Retention:                common.DaysToDuration(1),
							EmitMetric:               true,
							HistoryArchivalStatus:    types.ArchivalStatusEnabled,
							HistoryArchivalURI:       "s3://abc",
							VisibilityArchivalStatus: types.ArchivalStatusEnabled,
							VisibilityArchivalURI:    "s3://xyz",
							BadBinaries:              &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")},
							IsolationGroups:          &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")},
							AsyncWorkflowsConfig:     &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")},
						},
						ReplicationConfig: testFixtureDomainReplicationConfigInternalWithActiveClusters(),
						IsGlobalDomain:    true,
						ConfigVersion:     1,
						FailoverVersion:   10,
						LastUpdatedTime:   time.Unix(0, 100),
						FailoverEndTime:   common.Ptr(time.Unix(0, 200)),
					}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeBadBinaries(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")}).
					Return(&types.BadBinaries{}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeIsolationGroups(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")}).
					Return(&types.IsolationGroupConfiguration{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeAsyncWorkflowsConfig(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")}).
					Return(&types.AsyncWorkflowConfiguration{}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeActiveClusters(testFixtureDomainReplicationConfigInternalWithActiveClusters().ActiveClustersConfig).
					Return(testFixtureDomainReplicationConfigWithActiveClusters().ActiveClusters, nil).Times(1)
			},
			request:     &GetDomainRequest{ID: "domain1"},
			expectError: false,
			expected: &GetDomainResponse{
				Info: testFixtureDomainInfo(),
				Config: &DomainConfig{
					Retention:                1,
					EmitMetric:               true,
					HistoryArchivalStatus:    types.ArchivalStatusEnabled,
					HistoryArchivalURI:       "s3://abc",
					VisibilityArchivalStatus: types.ArchivalStatusEnabled,
					VisibilityArchivalURI:    "s3://xyz",
					BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
					IsolationGroups:          map[string]types.IsolationGroupPartition{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}},
					AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{},
				},
				ReplicationConfig: testFixtureDomainReplicationConfigWithActiveClusters(),
				IsGlobalDomain:    true,
				ConfigVersion:     1,
				FailoverVersion:   10,
				LastUpdatedTime:   100,
				FailoverEndTime:   common.Ptr(time.Unix(0, 200).UnixNano()),
			},
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockStore.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &GetDomainRequest{ID: "domain1"},
			expectError:   true,
			expectedError: "persistence error",
		},
		{
			name: "deserialization error",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockStore.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(&InternalGetDomainResponse{
						Info:              &DomainInfo{ID: "domain1"},
						Config:            &InternalDomainConfig{},
						ReplicationConfig: &InternalDomainReplicationConfig{},
					}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeBadBinaries(gomock.Any()).
					Return(nil, errors.New("deserialization error")).Times(1)
			},
			request:       &GetDomainRequest{ID: "domain1"},
			expectError:   true,
			expectedError: "deserialization error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			domainManager, mockStore, mockSerializer := setUpMocksForDomainManager(t)

			tc.setupMock(mockStore, mockSerializer)

			resp, err := domainManager.GetDomain(context.Background(), tc.request)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, resp)
			}
		})
	}
}

func TestUpdateDomain(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockDomainStore, *MockPayloadSerializer)
		request       *UpdateDomainRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					SerializeBadBinaries(&types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeIsolationGroups(&types.IsolationGroupConfiguration{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeAsyncWorkflowsConfig(&types.AsyncWorkflowConfiguration{Enabled: true, PredefinedQueueName: "q", QueueType: "kafka"}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeActiveClusters(nil, constants.EncodingTypeThriftRWSnappy).
					Return(nil, nil).Times(1)
				mockStore.EXPECT().
					UpdateDomain(gomock.Any(), &InternalUpdateDomainRequest{
						Info: testFixtureDomainInfo(),
						Config: &InternalDomainConfig{
							Retention:                common.DaysToDuration(1),
							EmitMetric:               true,
							HistoryArchivalStatus:    types.ArchivalStatusEnabled,
							HistoryArchivalURI:       "s3://abc",
							VisibilityArchivalStatus: types.ArchivalStatusEnabled,
							VisibilityArchivalURI:    "s3://xyz",
							BadBinaries:              &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")},
							IsolationGroups:          &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")},
							AsyncWorkflowsConfig:     &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")},
						},
						ReplicationConfig:           testFixtureDomainReplicationConfigInternal(),
						ConfigVersion:               1,
						FailoverVersion:             10,
						FailoverNotificationVersion: 11,
						PreviousFailoverVersion:     9,
						NotificationVersion:         1000,
						LastUpdatedTime:             time.Unix(0, 100),
						FailoverEndTime:             common.Ptr(time.Unix(0, 200)),
					}).
					Return(nil).Times(1)
			},
			request: &UpdateDomainRequest{
				Info:                        testFixtureDomainInfo(),
				Config:                      testFixtureDomainConfig(),
				ReplicationConfig:           testFixtureDomainReplicationConfig(),
				ConfigVersion:               1,
				FailoverVersion:             10,
				FailoverNotificationVersion: 11,
				PreviousFailoverVersion:     9,
				LastUpdatedTime:             100,
				FailoverEndTime:             common.Ptr(time.Unix(0, 200).UnixNano()),
				NotificationVersion:         1000,
			},
			expectError: false,
		},
		{
			name: "success with activeclusters config populated",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					SerializeBadBinaries(&types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeIsolationGroups(&types.IsolationGroupConfiguration{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeAsyncWorkflowsConfig(&types.AsyncWorkflowConfiguration{Enabled: true, PredefinedQueueName: "q", QueueType: "kafka"}, constants.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")}, nil).Times(1)
				mockSerializer.EXPECT().
					SerializeActiveClusters(testFixtureDomainReplicationConfigWithActiveClusters().ActiveClusters, constants.EncodingTypeThriftRWSnappy).
					Return(&DataBlob{Encoding: constants.EncodingTypeThriftRWSnappy, Data: []byte("active-clusters-config")}, nil).Times(1)
				mockStore.EXPECT().
					UpdateDomain(gomock.Any(), &InternalUpdateDomainRequest{
						Info: testFixtureDomainInfo(),
						Config: &InternalDomainConfig{
							Retention:                common.DaysToDuration(1),
							EmitMetric:               true,
							HistoryArchivalStatus:    types.ArchivalStatusEnabled,
							HistoryArchivalURI:       "s3://abc",
							VisibilityArchivalStatus: types.ArchivalStatusEnabled,
							VisibilityArchivalURI:    "s3://xyz",
							BadBinaries:              &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")},
							IsolationGroups:          &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")},
							AsyncWorkflowsConfig:     &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")},
						},
						ReplicationConfig:           testFixtureDomainReplicationConfigInternalWithActiveClusters(),
						ConfigVersion:               1,
						FailoverVersion:             10,
						FailoverNotificationVersion: 11,
						PreviousFailoverVersion:     9,
						NotificationVersion:         1000,
						LastUpdatedTime:             time.Unix(0, 100),
						FailoverEndTime:             common.Ptr(time.Unix(0, 200)),
					}).
					Return(nil).Times(1)
			},
			request: &UpdateDomainRequest{
				Info:                        testFixtureDomainInfo(),
				Config:                      testFixtureDomainConfig(),
				ReplicationConfig:           testFixtureDomainReplicationConfigWithActiveClusters(),
				ConfigVersion:               1,
				FailoverVersion:             10,
				FailoverNotificationVersion: 11,
				PreviousFailoverVersion:     9,
				LastUpdatedTime:             100,
				FailoverEndTime:             common.Ptr(time.Unix(0, 200).UnixNano()),
				NotificationVersion:         1000,
			},
			expectError: false,
		},
		{
			name: "serialization error",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					SerializeBadBinaries(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("serialization error")).Times(1)
			},
			request: &UpdateDomainRequest{
				Info:                        testFixtureDomainInfo(),
				Config:                      testFixtureDomainConfig(),
				ReplicationConfig:           testFixtureDomainReplicationConfig(),
				ConfigVersion:               1,
				FailoverVersion:             10,
				FailoverNotificationVersion: 11,
				PreviousFailoverVersion:     9,
				LastUpdatedTime:             100,
				NotificationVersion:         1000,
			},
			expectError:   true,
			expectedError: "serialization error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			domainManager, mockStore, mockSerializer := setUpMocksForDomainManager(t)

			tc.setupMock(mockStore, mockSerializer)

			err := domainManager.UpdateDomain(context.Background(), tc.request)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeleteDomain(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockDomainStore, *MockPayloadSerializer)
		request       *DeleteDomainRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockDomainStore, _ *MockPayloadSerializer) {
				mockStore.EXPECT().
					DeleteDomain(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
			request:     &DeleteDomainRequest{ID: "domain1"},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockDomainStore, _ *MockPayloadSerializer) {
				mockStore.EXPECT().
					DeleteDomain(gomock.Any(), gomock.Any()).
					Return(errors.New("persistence error")).Times(1)
			},
			request:       &DeleteDomainRequest{ID: "domain1"},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			domainManager, mockStore, mockSerializer := setUpMocksForDomainManager(t)

			tc.setupMock(mockStore, mockSerializer)

			err := domainManager.DeleteDomain(context.Background(), tc.request)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeleteDomainByName(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockDomainStore, *MockPayloadSerializer)
		request       *DeleteDomainByNameRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockDomainStore, _ *MockPayloadSerializer) {
				mockStore.EXPECT().
					DeleteDomainByName(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
			request:     &DeleteDomainByNameRequest{Name: "domain1"},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockDomainStore, _ *MockPayloadSerializer) {
				mockStore.EXPECT().
					DeleteDomainByName(gomock.Any(), gomock.Any()).
					Return(errors.New("persistence error")).Times(1)
			},
			request:       &DeleteDomainByNameRequest{Name: "domain1"},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			domainManager, mockStore, mockSerializer := setUpMocksForDomainManager(t)

			tc.setupMock(mockStore, mockSerializer)

			err := domainManager.DeleteDomainByName(context.Background(), tc.request)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListDomains(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockDomainStore, *MockPayloadSerializer)
		request       *ListDomainsRequest
		expectError   bool
		expectedError string
		expected      *ListDomainsResponse
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockStore.EXPECT().
					ListDomains(gomock.Any(), gomock.Any()).
					Return(&InternalListDomainsResponse{
						Domains: []*InternalGetDomainResponse{
							{
								Info: testFixtureDomainInfo(),
								Config: &InternalDomainConfig{
									Retention:                common.DaysToDuration(1),
									EmitMetric:               true,
									HistoryArchivalStatus:    types.ArchivalStatusEnabled,
									HistoryArchivalURI:       "s3://abc",
									VisibilityArchivalStatus: types.ArchivalStatusEnabled,
									VisibilityArchivalURI:    "s3://xyz",
									BadBinaries:              &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")},
									IsolationGroups:          &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")},
									AsyncWorkflowsConfig:     &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")},
								},
								ReplicationConfig: testFixtureDomainReplicationConfigInternal(),
								IsGlobalDomain:    true,
								ConfigVersion:     1,
								FailoverVersion:   10,
								LastUpdatedTime:   time.Unix(0, 100),
								FailoverEndTime:   common.Ptr(time.Unix(0, 200)),
							},
						},
						NextPageToken: []byte("token"),
					}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeBadBinaries(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")}).
					Return(&types.BadBinaries{}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeIsolationGroups(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")}).
					Return(&types.IsolationGroupConfiguration{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeAsyncWorkflowsConfig(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")}).
					Return(&types.AsyncWorkflowConfiguration{}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeActiveClusters(nil).
					Return(nil, nil).Times(1)
			},
			request:     &ListDomainsRequest{PageSize: 10},
			expectError: false,
			expected: &ListDomainsResponse{
				Domains: []*GetDomainResponse{
					{

						Info: testFixtureDomainInfo(),
						Config: &DomainConfig{
							Retention:                1,
							EmitMetric:               true,
							HistoryArchivalStatus:    types.ArchivalStatusEnabled,
							HistoryArchivalURI:       "s3://abc",
							VisibilityArchivalStatus: types.ArchivalStatusEnabled,
							VisibilityArchivalURI:    "s3://xyz",
							BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
							IsolationGroups:          map[string]types.IsolationGroupPartition{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}},
							AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{},
						},
						ReplicationConfig: testFixtureDomainReplicationConfig(),
						IsGlobalDomain:    true,
						ConfigVersion:     1,
						FailoverVersion:   10,
						LastUpdatedTime:   100,
						FailoverEndTime:   common.Ptr(time.Unix(0, 200).UnixNano()),
					},
				},
				NextPageToken: []byte("token"),
			},
		},
		{
			name: "success with activeclusters config populated",
			setupMock: func(mockStore *MockDomainStore, mockSerializer *MockPayloadSerializer) {
				mockStore.EXPECT().
					ListDomains(gomock.Any(), gomock.Any()).
					Return(&InternalListDomainsResponse{
						Domains: []*InternalGetDomainResponse{
							{
								Info: testFixtureDomainInfo(),
								Config: &InternalDomainConfig{
									Retention:                common.DaysToDuration(1),
									EmitMetric:               true,
									HistoryArchivalStatus:    types.ArchivalStatusEnabled,
									HistoryArchivalURI:       "s3://abc",
									VisibilityArchivalStatus: types.ArchivalStatusEnabled,
									VisibilityArchivalURI:    "s3://xyz",
									BadBinaries:              &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")},
									IsolationGroups:          &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")},
									AsyncWorkflowsConfig:     &DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")},
								},
								ReplicationConfig: testFixtureDomainReplicationConfigInternalWithActiveClusters(),
								IsGlobalDomain:    true,
								ConfigVersion:     1,
								FailoverVersion:   10,
								LastUpdatedTime:   time.Unix(0, 100),
								FailoverEndTime:   common.Ptr(time.Unix(0, 200)),
							},
						},
						NextPageToken: []byte("token"),
					}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeBadBinaries(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("bad-binaries")}).
					Return(&types.BadBinaries{}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeIsolationGroups(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("isolation-groups")}).
					Return(&types.IsolationGroupConfiguration{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeAsyncWorkflowsConfig(&DataBlob{Encoding: constants.EncodingTypeThriftRW, Data: []byte("async-workflow-config")}).
					Return(&types.AsyncWorkflowConfiguration{}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeActiveClusters(testFixtureDomainReplicationConfigInternalWithActiveClusters().ActiveClustersConfig).
					Return(testFixtureDomainReplicationConfigWithActiveClusters().ActiveClusters, nil).Times(1)
			},
			request:     &ListDomainsRequest{PageSize: 10},
			expectError: false,
			expected: &ListDomainsResponse{
				Domains: []*GetDomainResponse{
					{

						Info: testFixtureDomainInfo(),
						Config: &DomainConfig{
							Retention:                1,
							EmitMetric:               true,
							HistoryArchivalStatus:    types.ArchivalStatusEnabled,
							HistoryArchivalURI:       "s3://abc",
							VisibilityArchivalStatus: types.ArchivalStatusEnabled,
							VisibilityArchivalURI:    "s3://xyz",
							BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
							IsolationGroups:          map[string]types.IsolationGroupPartition{"abc": {Name: "abc", State: types.IsolationGroupStateDrained}},
							AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{},
						},
						ReplicationConfig: testFixtureDomainReplicationConfigWithActiveClusters(),
						IsGlobalDomain:    true,
						ConfigVersion:     1,
						FailoverVersion:   10,
						LastUpdatedTime:   100,
						FailoverEndTime:   common.Ptr(time.Unix(0, 200).UnixNano()),
					},
				},
				NextPageToken: []byte("token"),
			},
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockDomainStore, _ *MockPayloadSerializer) {
				mockStore.EXPECT().
					ListDomains(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request:       &ListDomainsRequest{PageSize: 10},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			domainManager, mockStore, mockSerializer := setUpMocksForDomainManager(t)

			tc.setupMock(mockStore, mockSerializer)

			resp, err := domainManager.ListDomains(context.Background(), tc.request)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, resp)
			}
		})
	}
}

func TestGetMetadata(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockDomainStore)
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockDomainStore) {
				mockStore.EXPECT().
					GetMetadata(gomock.Any()).
					Return(&GetMetadataResponse{NotificationVersion: 10}, nil).Times(1)
			},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockDomainStore) {
				mockStore.EXPECT().
					GetMetadata(gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			domainManager, mockStore, _ := setUpMocksForDomainManager(t)

			tc.setupMock(mockStore)

			// Execute the GetMetadata method
			resp, err := domainManager.GetMetadata(context.Background())

			// Validate results
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int64(10), resp.NotificationVersion)
			}
		})
	}
}

func TestDomainManagerClose(t *testing.T) {
	t.Run("close persistence store", func(t *testing.T) {
		domainManager, mockStore, _ := setUpMocksForDomainManager(t)

		// Expect the Close method to be called once
		mockStore.EXPECT().Close().Times(1)

		// Execute the Close method
		domainManager.Close()

		// No need to assert as we are just expecting the method call
	})
}
