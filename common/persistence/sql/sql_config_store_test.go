// Modifications Copyright (c) 2020 Uber Technologies Inc.

// Copyright (c) 2020 Temporal Technologies, Inc.

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

package sql

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

func TestFetchConfig(t *testing.T) {

	testCases := []struct {
		name       string
		configType persistence.ConfigType
		mockSetup  func(*sqlplugin.MockDB)
		want       *persistence.InternalConfigStoreEntry
		wantErr    bool
	}{
		{
			name:       "Success case",
			configType: persistence.DynamicConfig,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().SelectLatestConfig(gomock.Any(), gomock.Any()).Return(&persistence.InternalConfigStoreEntry{}, nil)
				mockDB.EXPECT().IsNotFoundError(nil).Return(false)
			},
			want:    &persistence.InternalConfigStoreEntry{},
			wantErr: false,
		},
		{
			name:       "Not found error",
			configType: persistence.DynamicConfig,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				notFoundErr := errors.New("not found")
				mockDB.EXPECT().SelectLatestConfig(gomock.Any(), gomock.Any()).Return(nil, notFoundErr)
				mockDB.EXPECT().IsNotFoundError(notFoundErr).Return(true)
			},
			want:    nil,
			wantErr: false,
		},
		{
			name:       "Database error",
			configType: persistence.DynamicConfig,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("db error")
				mockDB.EXPECT().SelectLatestConfig(gomock.Any(), gomock.Any()).Return(nil, err)
				mockDB.EXPECT().IsNotFoundError(err).Return(false)
				mockDB.EXPECT().IsNotFoundError(err).Return(false)
				mockDB.EXPECT().IsTimeoutError(err).Return(true)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := NewSQLConfigStore(mockDB, nil, nil)
			require.NoError(t, err, "Failed to create sql config store")

			tc.mockSetup(mockDB)
			got, err := store.FetchConfig(context.Background(), tc.configType)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case: %s", tc.name)
			} else {
				assert.NoError(t, err, "Did not expect an error for test case: %s", tc.name)
				assert.Equal(t, tc.want, got, "Unexpected result for test case: %s", tc.name)
			}
		})
	}
}

func TestUpdateConfig(t *testing.T) {
	testEntry := &persistence.InternalConfigStoreEntry{
		RowType: int(persistence.DynamicConfig),
		Version: 0,
		Values: &persistence.DataBlob{
			Data: []byte(`0x0000`),
		},
	}

	testCases := []struct {
		name      string
		value     *persistence.InternalConfigStoreEntry
		mockSetup func(*sqlplugin.MockDB)
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name:  "Success case",
			value: testEntry,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				mockDB.EXPECT().InsertConfig(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "Duplicate Entry error",
			value: testEntry,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				dupError := errors.New("duplicate entry")
				mockDB.EXPECT().InsertConfig(gomock.Any(), gomock.Any()).Return(dupError)
				mockDB.EXPECT().IsDupEntryError(dupError).Return(true)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var expectedErr *persistence.ConditionFailedError
				assert.True(t, errors.As(err, &expectedErr), "Expected the error to be ConditionFailedError")
			},
		},
		{
			name:  "Database error",
			value: testEntry,
			mockSetup: func(mockDB *sqlplugin.MockDB) {
				err := errors.New("db error")
				mockDB.EXPECT().InsertConfig(gomock.Any(), gomock.Any()).Return(err)
				mockDB.EXPECT().IsDupEntryError(err).Return(false)
				mockDB.EXPECT().IsNotFoundError(err).Return(false)
				mockDB.EXPECT().IsTimeoutError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDB := sqlplugin.NewMockDB(ctrl)
			store, err := NewSQLConfigStore(mockDB, nil, nil)
			require.NoError(t, err, "Failed to create sql config store")

			tc.mockSetup(mockDB)
			err = store.UpdateConfig(context.Background(), tc.value)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case: %s", tc.name)
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err, "Did not expect an error for test case: %s", tc.name)
			}
		})
	}
}
