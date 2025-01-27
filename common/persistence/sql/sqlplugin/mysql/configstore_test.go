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

package mysql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqldriver"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

func TestInsertConfig(t *testing.T) {
	testCases := []struct {
		name        string
		row         *persistence.InternalConfigStoreEntry
		mockSetup   func(*sqldriver.MockDriver)
		expectError bool
	}{
		{
			name: "Success case",
			row: &persistence.InternalConfigStoreEntry{
				Values: &persistence.DataBlob{},
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().ExecContext(gomock.Any(), sqlplugin.DbDefaultShard, _insertConfigQuery, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			expectError: false,
		},
		{
			name: "Error case",
			row: &persistence.InternalConfigStoreEntry{
				Values: &persistence.DataBlob{},
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().ExecContext(gomock.Any(), sqlplugin.DbDefaultShard, _insertConfigQuery, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDriver := sqldriver.NewMockDriver(ctrl)
			mdb := &DB{driver: mockDriver, converter: &converter{}}

			// Setup mock expectations
			tc.mockSetup(mockDriver)

			err := mdb.InsertConfig(context.Background(), tc.row)
			if tc.expectError {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestSelectLatestConfig(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name        string
		rowType     int
		setupMock   func(*sqldriver.MockDriver)
		expectError bool
		expectedRow *persistence.InternalConfigStoreEntry
	}{
		{
			name:    "Success case",
			rowType: 1,
			setupMock: func(md *sqldriver.MockDriver) {
				row := sqlplugin.ClusterConfigRow{
					RowType:      1,
					Version:      -2,
					Timestamp:    now,
					Data:         []byte("test data"),
					DataEncoding: "json",
				}
				md.EXPECT().GetContext(gomock.Any(), sqlplugin.DbDefaultShard, gomock.Any(), _selectLatestConfigQuery, 1).DoAndReturn(
					func(ctx context.Context, shardID int, r *sqlplugin.ClusterConfigRow, query string, args ...interface{}) error {
						*r = row
						return nil
					},
				)
			},
			expectError: false,
			expectedRow: &persistence.InternalConfigStoreEntry{
				RowType:   1,
				Version:   2,
				Timestamp: now,
				Values: &persistence.DataBlob{
					Data:     []byte("test data"),
					Encoding: common.EncodingType("json"),
				},
			},
		},
		{
			name:    "Error case",
			rowType: 2,
			setupMock: func(md *sqldriver.MockDriver) {
				md.EXPECT().GetContext(gomock.Any(), sqlplugin.DbDefaultShard, gomock.Any(), _selectLatestConfigQuery, 2).Return(errors.New("some error"))
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDriver := sqldriver.NewMockDriver(ctrl)
			mdb := &DB{driver: mockDriver, converter: &converter{}}

			tc.setupMock(mockDriver)

			row, err := mdb.SelectLatestConfig(context.Background(), tc.rowType)
			if tc.expectError {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
				assert.Equal(t, tc.expectedRow, row, "Expected result to be the same for test case")
			}
		})
	}
}
