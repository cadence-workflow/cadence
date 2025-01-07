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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func setUpMocksForNoSQLConfigStore(t *testing.T) (*nosqlConfigStore, *nosqlplugin.MockDB) {
	ctrl := gomock.NewController(t)
	mockDB := nosqlplugin.NewMockDB(ctrl)
	logger := log.NewNoop()

	// Creating the nosqlConfigStore and injecting the mocked DB
	configStore := &nosqlConfigStore{
		nosqlStore: nosqlStore{
			logger: logger,
			db:     mockDB,
		},
	}

	return configStore, mockDB
}

func TestFetchConfig(t *testing.T) {
	testCases := []struct {
		name           string
		setupMock      func(mockDB *nosqlplugin.MockDB)
		configType     persistence.ConfigType
		expectError    bool
		expectedError  string
		expectedResult *persistence.InternalConfigStoreEntry
	}{
		{
			name: "success",
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().
					SelectLatestConfig(gomock.Any(), int(persistence.DynamicConfig)).
					Return(&persistence.InternalConfigStoreEntry{
						Version: 1,
						Values:  &persistence.DataBlob{Encoding: common.EncodingTypeThriftRW, Data: []byte("config-values")},
					}, nil).Times(1)
			},
			configType:     persistence.DynamicConfig,
			expectError:    false,
			expectedResult: &persistence.InternalConfigStoreEntry{Version: 1, Values: &persistence.DataBlob{Encoding: common.EncodingTypeThriftRW, Data: []byte("config-values")}},
		},
		{
			name: "config not found",
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().
					SelectLatestConfig(gomock.Any(), int(persistence.DynamicConfig)).
					Return(nil, errors.New("not found")).
					Times(1)
				mockDB.EXPECT().IsNotFoundError(errors.New("not found")).Return(true).Times(1)
			},
			configType:     persistence.DynamicConfig,
			expectError:    false,
			expectedResult: nil,
		},
		{
			name: "fetch error",
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().
					SelectLatestConfig(gomock.Any(), int(persistence.DynamicConfig)).
					Return(nil, errors.New("fetch error")).
					Times(1)
				mockDB.EXPECT().IsNotFoundError(errors.New("fetch error")).Return(false).Times(1)
				mockDB.EXPECT().IsNotFoundError(errors.New("fetch error")).Return(true).Times(1)
			},
			configType:    persistence.DynamicConfig,
			expectError:   true,
			expectedError: "fetch error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configStore, mockDB := setUpMocksForNoSQLConfigStore(t)

			tc.setupMock(mockDB)

			result, err := configStore.FetchConfig(context.Background(), tc.configType)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func TestUpdateConfig(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(mockDB *nosqlplugin.MockDB)
		value         *persistence.InternalConfigStoreEntry
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().
					InsertConfig(gomock.Any(), &persistence.InternalConfigStoreEntry{
						Version: 1,
						Values:  &persistence.DataBlob{Encoding: common.EncodingTypeThriftRW, Data: []byte("config-values")},
					}).
					Return(nil).
					Times(1)
			},
			value: &persistence.InternalConfigStoreEntry{
				Version: 1,
				Values:  &persistence.DataBlob{Encoding: common.EncodingTypeThriftRW, Data: []byte("config-values")},
			},
			expectError: false,
		},
		{
			name: "condition failure",
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().
					InsertConfig(gomock.Any(), gomock.Any()).
					Return(&nosqlplugin.ConditionFailure{}).
					Times(1)
			},
			value: &persistence.InternalConfigStoreEntry{
				Version: 1,
				Values:  &persistence.DataBlob{Encoding: common.EncodingTypeThriftRW, Data: []byte("config-values")},
			},
			expectError:   true,
			expectedError: "Version 1 already exists. Condition Failed",
		},
		{
			name: "insert error",
			setupMock: func(mockDB *nosqlplugin.MockDB) {
				mockDB.EXPECT().
					InsertConfig(gomock.Any(), gomock.Any()).
					Return(errors.New("insert error")).
					Times(1)
				mockDB.EXPECT().IsNotFoundError(errors.New("insert error")).Return(true).Times(1)
			},
			value: &persistence.InternalConfigStoreEntry{
				Version: 1,
				Values:  &persistence.DataBlob{Encoding: common.EncodingTypeThriftRW, Data: []byte("config-values")},
			},
			expectError:   true,
			expectedError: "insert error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configStore, mockDB := setUpMocksForNoSQLConfigStore(t)

			tc.setupMock(mockDB)

			err := configStore.UpdateConfig(context.Background(), tc.value)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
