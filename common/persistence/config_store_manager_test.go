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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

func setUpMocksForConfigStoreManager(t *testing.T) (*configStoreManagerImpl, *MockConfigStore, *MockPayloadSerializer) {
	ctrl := gomock.NewController(t)
	mockStore := NewMockConfigStore(ctrl)
	mockSerializer := NewMockPayloadSerializer(ctrl)
	logger := log.NewNoop()

	return &configStoreManagerImpl{
		serializer:  mockSerializer,
		persistence: mockStore,
		logger:      logger,
	}, mockStore, mockSerializer
}

func TestNewConfigStoreManagerImpl(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := NewMockConfigStore(ctrl)
	logger := log.NewNoop()
	m := NewConfigStoreManagerImpl(mockStore, logger)
	assert.NotNil(t, m)
	cm, ok := m.(*configStoreManagerImpl)
	assert.True(t, ok)
	assert.Equal(t, mockStore, cm.persistence)
	assert.Equal(t, logger, cm.logger)
	assert.NotNil(t, cm.serializer)
}

func TestFetchDynamicConfig(t *testing.T) {
	encodingType := common.EncodingTypeThriftRW
	testCases := []struct {
		name             string
		setupMock        func(mockStore *MockConfigStore, mockSerializer *MockPayloadSerializer)
		cfgType          ConfigType
		expectError      bool
		expectedError    string
		expectedResponse *FetchDynamicConfigResponse
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockConfigStore, mockSerializer *MockPayloadSerializer) {
				// Mocking persistence DataBlob
				mockStore.EXPECT().FetchConfig(gomock.Any(), DynamicConfig).Return(&InternalConfigStoreEntry{
					Version:   1,
					Timestamp: time.Now(),
					Values:    &DataBlob{Encoding: encodingType, Data: []byte("serialized-values")},
				}, nil).Times(1)

				// Mocking deserialization of persistence.DataBlob into types.DynamicConfigBlob
				mockSerializer.EXPECT().
					DeserializeDynamicConfigBlob(&DataBlob{Encoding: encodingType, Data: []byte("serialized-values")}).
					Return(&types.DynamicConfigBlob{
						SchemaVersion: 1,
						Entries: []*types.DynamicConfigEntry{
							{
								Name: "TestEntry",
								Values: []*types.DynamicConfigValue{
									{
										Value: &types.DataBlob{
											EncodingType: types.EncodingTypeThriftRW.Ptr(),
											Data:         []byte("config-value"),
										},
										Filters: []*types.DynamicConfigFilter{
											{
												Name: "Filter1",
												Value: &types.DataBlob{
													EncodingType: types.EncodingTypeThriftRW.Ptr(),
													Data:         []byte("filter-value"),
												},
											},
										},
									},
								},
							},
						},
					}, nil).Times(1)
			},
			cfgType:     DynamicConfig, // Updated to use DynamicConfig
			expectError: false,
			expectedResponse: &FetchDynamicConfigResponse{
				Snapshot: &DynamicConfigSnapshot{
					Version: 1,
					Values: &types.DynamicConfigBlob{
						SchemaVersion: 1,
						Entries: []*types.DynamicConfigEntry{
							{
								Name: "TestEntry",
								Values: []*types.DynamicConfigValue{
									{
										Value: &types.DataBlob{
											EncodingType: types.EncodingTypeThriftRW.Ptr(),
											Data:         []byte("config-value"),
										},
										Filters: []*types.DynamicConfigFilter{
											{
												Name: "Filter1",
												Value: &types.DataBlob{
													EncodingType: types.EncodingTypeThriftRW.Ptr(),
													Data:         []byte("filter-value"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "fetch config error",
			setupMock: func(mockStore *MockConfigStore, mockSerializer *MockPayloadSerializer) {
				mockStore.EXPECT().FetchConfig(gomock.Any(), gomock.Any()).Return(nil, errors.New("fetch error")).Times(1)
			},
			cfgType:       DynamicConfig, // Updated to use DynamicConfig
			expectError:   true,
			expectedError: "fetch error",
		},
		{
			name: "deserialization error",
			setupMock: func(mockStore *MockConfigStore, mockSerializer *MockPayloadSerializer) {
				mockStore.EXPECT().FetchConfig(gomock.Any(), gomock.Any()).Return(&InternalConfigStoreEntry{
					Version:   1,
					Timestamp: time.Now(),
					Values:    &DataBlob{Encoding: encodingType, Data: []byte("serialized-values")},
				}, nil).Times(1)

				mockSerializer.EXPECT().
					DeserializeDynamicConfigBlob(&DataBlob{Encoding: encodingType, Data: []byte("serialized-values")}).
					Return(nil, errors.New("deserialization error")).Times(1)
			},
			cfgType:       DynamicConfig, // Updated to use DynamicConfig
			expectError:   true,
			expectedError: "deserialization error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configStoreManager, mockStore, mockSerializer := setUpMocksForConfigStoreManager(t)

			tc.setupMock(mockStore, mockSerializer)

			resp, err := configStoreManager.FetchDynamicConfig(context.Background(), tc.cfgType)

			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResponse, resp)
			}
		})
	}
}

func TestUpdateDynamicConfig(t *testing.T) {
	encodingType := common.EncodingTypeThriftRW
	testCases := []struct {
		name          string
		setupMock     func(mockStore *MockConfigStore, mockSerializer *MockPayloadSerializer)
		cfgType       ConfigType
		request       *UpdateDynamicConfigRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockConfigStore, mockSerializer *MockPayloadSerializer) {
				// Mock serialization of types.DynamicConfigBlob to persistence.DataBlob
				mockSerializer.EXPECT().
					SerializeDynamicConfigBlob(&types.DynamicConfigBlob{
						SchemaVersion: 1,
						Entries: []*types.DynamicConfigEntry{
							{
								Name: "TestEntry",
								Values: []*types.DynamicConfigValue{
									{
										Value: &types.DataBlob{
											EncodingType: types.EncodingTypeThriftRW.Ptr(),
											Data:         []byte("config-value"),
										},
										Filters: []*types.DynamicConfigFilter{
											{
												Name: "Filter1",
												Value: &types.DataBlob{
													EncodingType: types.EncodingTypeThriftRW.Ptr(),
													Data:         []byte("filter-value"),
												},
											},
										},
									},
								},
							},
						},
					}, common.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: encodingType, Data: []byte("serialized-values")}, nil).Times(1)

				mockStore.EXPECT().UpdateConfig(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			cfgType: DynamicConfig, // Updated to use DynamicConfig
			request: &UpdateDynamicConfigRequest{
				Snapshot: &DynamicConfigSnapshot{
					Version: 1,
					Values: &types.DynamicConfigBlob{
						SchemaVersion: 1,
						Entries: []*types.DynamicConfigEntry{
							{
								Name: "TestEntry",
								Values: []*types.DynamicConfigValue{
									{
										Value: &types.DataBlob{
											EncodingType: types.EncodingTypeThriftRW.Ptr(),
											Data:         []byte("config-value"),
										},
										Filters: []*types.DynamicConfigFilter{
											{
												Name: "Filter1",
												Value: &types.DataBlob{
													EncodingType: types.EncodingTypeThriftRW.Ptr(),
													Data:         []byte("filter-value"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "serialization error",
			setupMock: func(mockStore *MockConfigStore, mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					SerializeDynamicConfigBlob(&types.DynamicConfigBlob{
						SchemaVersion: 1,
						Entries: []*types.DynamicConfigEntry{
							{
								Name: "TestEntry",
								Values: []*types.DynamicConfigValue{
									{
										Value: &types.DataBlob{
											EncodingType: types.EncodingTypeThriftRW.Ptr(),
											Data:         []byte("config-value"),
										},
										Filters: []*types.DynamicConfigFilter{
											{
												Name: "Filter1",
												Value: &types.DataBlob{
													EncodingType: types.EncodingTypeThriftRW.Ptr(),
													Data:         []byte("filter-value"),
												},
											},
										},
									},
								},
							},
						},
					}, common.EncodingTypeThriftRW).
					Return(nil, errors.New("serialization error")).Times(1)
			},
			cfgType: DynamicConfig, // Updated to use DynamicConfig
			request: &UpdateDynamicConfigRequest{
				Snapshot: &DynamicConfigSnapshot{
					Version: 1,
					Values: &types.DynamicConfigBlob{
						SchemaVersion: 1,
						Entries: []*types.DynamicConfigEntry{
							{
								Name: "TestEntry",
								Values: []*types.DynamicConfigValue{
									{
										Value: &types.DataBlob{
											EncodingType: types.EncodingTypeThriftRW.Ptr(),
											Data:         []byte("config-value"),
										},
										Filters: []*types.DynamicConfigFilter{
											{
												Name: "Filter1",
												Value: &types.DataBlob{
													EncodingType: types.EncodingTypeThriftRW.Ptr(),
													Data:         []byte("filter-value"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError:   true,
			expectedError: "serialization error",
		},
		{
			name: "update config error",
			setupMock: func(mockStore *MockConfigStore, mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					SerializeDynamicConfigBlob(&types.DynamicConfigBlob{
						SchemaVersion: 1,
						Entries: []*types.DynamicConfigEntry{
							{
								Name: "TestEntry",
								Values: []*types.DynamicConfigValue{
									{
										Value: &types.DataBlob{
											EncodingType: types.EncodingTypeThriftRW.Ptr(),
											Data:         []byte("config-value"),
										},
										Filters: []*types.DynamicConfigFilter{
											{
												Name: "Filter1",
												Value: &types.DataBlob{
													EncodingType: types.EncodingTypeThriftRW.Ptr(),
													Data:         []byte("filter-value"),
												},
											},
										},
									},
								},
							},
						},
					}, common.EncodingTypeThriftRW).
					Return(&DataBlob{Encoding: encodingType, Data: []byte("serialized-values")}, nil).Times(1)

				mockStore.EXPECT().UpdateConfig(gomock.Any(), gomock.Any()).
					Return(errors.New("update error")).Times(1)
			},
			cfgType: DynamicConfig, // Updated to use DynamicConfig
			request: &UpdateDynamicConfigRequest{
				Snapshot: &DynamicConfigSnapshot{
					Version: 1,
					Values: &types.DynamicConfigBlob{
						SchemaVersion: 1,
						Entries: []*types.DynamicConfigEntry{
							{
								Name: "TestEntry",
								Values: []*types.DynamicConfigValue{
									{
										Value: &types.DataBlob{
											EncodingType: types.EncodingTypeThriftRW.Ptr(),
											Data:         []byte("config-value"),
										},
										Filters: []*types.DynamicConfigFilter{
											{
												Name: "Filter1",
												Value: &types.DataBlob{
													EncodingType: types.EncodingTypeThriftRW.Ptr(),
													Data:         []byte("filter-value"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError:   true,
			expectedError: "update error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configStoreManager, mockStore, mockSerializer := setUpMocksForConfigStoreManager(t)

			tc.setupMock(mockStore, mockSerializer)

			err := configStoreManager.UpdateDynamicConfig(context.Background(), tc.request, tc.cfgType)

			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCloseConfigStoreManager(t *testing.T) {
	t.Run("close persistence", func(t *testing.T) {
		configStoreManager, mockStore, _ := setUpMocksForConfigStoreManager(t)

		// Expect Close method to be called once
		mockStore.EXPECT().Close().Times(1)

		// Call the Close method
		configStoreManager.Close()
	})
}
