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

package sql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

// MockErrorChecker is a mock implementation of the sqlplugin.ErrorChecker interface
type MockErrorChecker struct{}

var _ sqlplugin.ErrorChecker = (*MockErrorChecker)(nil)

func (m *MockErrorChecker) IsNotFoundError(err error) bool {
	if strings.Contains(err.Error(), "not found") {
		return true
	}
	return false
}

func (m *MockErrorChecker) IsTimeoutError(err error) bool {
	if strings.Contains(err.Error(), "timeout") {
		return true
	}
	return false
}

func (m *MockErrorChecker) IsThrottlingError(err error) bool {
	if strings.Contains(err.Error(), "throttle") {
		return true
	}
	return false
}

func (m *MockErrorChecker) IsDupEntryError(err error) bool {
	return false
}

func TestConvertCommonErrors(t *testing.T) {
	errChecker := &MockErrorChecker{}
	tests := []struct {
		name      string
		operation string
		message   string
		err       error
		wantError error
	}{
		{
			name:      "ConditionFailedError",
			operation: "Create",
			message:   "creation",
			err:       &persistence.ConditionFailedError{},
			wantError: &persistence.ConditionFailedError{},
		},
		{
			name:      "NotFoundError",
			operation: "Get",
			message:   "retrieval",
			err:       errors.New("not found"),
			wantError: &types.EntityNotExistsError{Message: "Get failed. retrieval Error: not found"},
		},
		{
			name:      "TimeoutError",
			operation: "Update",
			message:   "update",
			err:       errors.New("timeout"),
			wantError: &persistence.TimeoutError{Msg: "Update timed out. update Error: timeout"},
		},
		{
			name:      "ThrottlingError",
			operation: "Delete",
			message:   "deletion",
			err:       errors.New("throttle"),
			wantError: &types.ServiceBusyError{Message: "Delete operation failed. deletion Error: throttle"},
		},
		{
			name:      "InternalServiceError",
			operation: "List",
			message:   "listing",
			err:       errors.New("generic error"),
			wantError: &types.InternalServiceError{Message: "List operation failed. listing Error: generic error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotError := convertCommonErrors(errChecker, tt.operation, tt.message, tt.err)
			if gotError.Error() != tt.wantError.Error() {
				t.Errorf("convertCommonErrors() error = %v, wantErr %v", gotError, tt.wantError)
			}
		})
	}
}

func TestTxExecute(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*sqlplugin.MockDB, *sqlplugin.MockTx)
		operation string
		fn        func(sqlplugin.Tx) error
		wantError error
	}{
		{
			name: "Success",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().Commit().Return(nil)
			},
			operation: "Insert",
			fn:        func(sqlplugin.Tx) error { return nil },
			wantError: nil,
		},
		{
			name: "Error",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().Rollback().Return(nil)
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false)
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false)
			},
			operation: "Insert",
			fn:        func(sqlplugin.Tx) error { return errors.New("error") },
			wantError: &types.InternalServiceError{Message: "Insert operation failed.  Error: error"},
		},
		{
			name: "BeginTxError",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false)
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false)
			},
			operation: "Insert",
			fn:        func(sqlplugin.Tx) error { return nil },
			wantError: &types.InternalServiceError{Message: "Insert operation failed. Failed to start transaction. Error: error"},
		},
		{
			name: "CommitError",
			mockSetup: func(mockDB *sqlplugin.MockDB, mockTx *sqlplugin.MockTx) {
				mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
				mockTx.EXPECT().Commit().Return(errors.New("error"))
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
				mockDB.EXPECT().IsTimeoutError(gomock.Any()).Return(false)
				mockDB.EXPECT().IsThrottlingError(gomock.Any()).Return(false)
			},
			operation: "Insert",
			fn:        func(sqlplugin.Tx) error { return nil },
			wantError: &types.InternalServiceError{Message: "Insert operation failed. Failed to commit transaction. Error: error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := sqlplugin.NewMockDB(ctrl)
			mockTx := sqlplugin.NewMockTx(ctrl)
			tt.mockSetup(mockDB, mockTx)

			s := &sqlStore{db: mockDB, logger: testlogger.New(t)}

			gotError := s.txExecute(context.Background(), 0, tt.operation, tt.fn)
			assert.Equal(t, tt.wantError, gotError)
		})
	}
}

func TestGobSerialize(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name:    "Serialize string",
			input:   "test string",
			wantErr: false,
		},
		{
			name:    "Serialize struct",
			input:   struct{ A int }{1},
			wantErr: false,
		},
		{
			name:    "Serialize unsupported type",
			input:   make(chan int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gobSerialize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("gobSerialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) == 0 {
				t.Errorf("gobSerialize() returned empty byte slice")
			}
		})
	}
}

func TestGobDeserialize(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		target  interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name:    "Deserialize to string",
			input:   mustGobSerialize("test string"),
			target:  new(string),
			want:    "test string",
			wantErr: false,
		},
		{
			name:    "Deserialize to struct",
			input:   mustGobSerialize(struct{ A int }{1}),
			target:  new(struct{ A int }),
			want:    struct{ A int }{1},
			wantErr: false,
		},
		{
			name:    "Deserialize with invalid data",
			input:   []byte("invalid"),
			target:  new(string),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gobDeserialize(tt.input, tt.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, reflect.ValueOf(tt.target).Elem().Interface())
			}
		})
	}
}

// mustGobSerialize is a helper function that panics if serialization fails. Used for setting up tests.
func mustGobSerialize(v interface{}) []byte {
	b, err := gobSerialize(v)
	if err != nil {
		panic(fmt.Sprintf("mustGobSerialize: %v", err))
	}
	return b
}
