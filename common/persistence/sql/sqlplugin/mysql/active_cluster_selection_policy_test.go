// Copyright (c) 2026 Uber Technologies, Inc.
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

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqldriver"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

func TestInsertIntoActiveClusterSelectionPolicy(t *testing.T) {
	domainID := serialization.UUID(uuid.NewRandom())
	runID := serialization.UUID(uuid.NewRandom())

	tests := []struct {
		name      string
		row       *sqlplugin.ActiveClusterSelectionPolicyRow
		mockSetup func(*sqldriver.MockDriver)
		wantErr   bool
	}{
		{
			name: "successful insert",
			row: &sqlplugin.ActiveClusterSelectionPolicyRow{
				ShardID:      7,
				DomainID:     domainID,
				WorkflowID:   "wf-1",
				RunID:        runID,
				Data:         []byte("policy-bytes"),
				DataEncoding: "thriftrw",
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().ExecContext(
					gomock.Any(),
					0,
					insertActiveClusterSelectionPolicyQuery,
					7,
					domainID,
					"wf-1",
					runID,
					[]byte("policy-bytes"),
					"thriftrw",
				).Return(nil, nil)
			},
		},
		{
			name: "exec error is returned",
			row: &sqlplugin.ActiveClusterSelectionPolicyRow{
				ShardID:      1,
				DomainID:     domainID,
				WorkflowID:   "wf-1",
				RunID:        runID,
				Data:         []byte("x"),
				DataEncoding: "thriftrw",
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().ExecContext(
					gomock.Any(),
					gomock.Any(),
					insertActiveClusterSelectionPolicyQuery,
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(nil, errors.New("boom"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDriver := sqldriver.NewMockDriver(ctrl)
			tc.mockSetup(mockDriver)

			mdb := &DB{
				driver:      mockDriver,
				converter:   &converter{},
				numDBShards: 1,
			}
			_, err := mdb.InsertIntoActiveClusterSelectionPolicy(context.Background(), tc.row)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestSelectFromActiveClusterSelectionPolicy(t *testing.T) {
	domainID := serialization.UUID(uuid.NewRandom())
	runID := serialization.UUID(uuid.NewRandom())

	tests := []struct {
		name      string
		filter    *sqlplugin.ActiveClusterSelectionPolicyFilter
		mockSetup func(*sqldriver.MockDriver)
		wantRow   *sqlplugin.ActiveClusterSelectionPolicyRow
		wantErr   error
	}{
		{
			name: "row found",
			filter: &sqlplugin.ActiveClusterSelectionPolicyFilter{
				ShardID: 7, DomainID: domainID, WorkflowID: "wf-1", RunID: runID,
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().GetContext(
					gomock.Any(),
					0,
					gomock.Any(),
					getActiveClusterSelectionPolicyQuery,
					7, domainID, "wf-1", runID,
				).DoAndReturn(func(_ context.Context, _ int, dest interface{}, _ string, _ ...interface{}) error {
					row := dest.(*sqlplugin.ActiveClusterSelectionPolicyRow)
					row.ShardID = 7
					row.DomainID = domainID
					row.WorkflowID = "wf-1"
					row.RunID = runID
					row.Data = []byte("policy-bytes")
					row.DataEncoding = "thriftrw"
					return nil
				})
			},
			wantRow: &sqlplugin.ActiveClusterSelectionPolicyRow{
				ShardID:      7,
				DomainID:     domainID,
				WorkflowID:   "wf-1",
				RunID:        runID,
				Data:         []byte("policy-bytes"),
				DataEncoding: "thriftrw",
			},
		},
		{
			name: "no rows found propagates sql.ErrNoRows",
			filter: &sqlplugin.ActiveClusterSelectionPolicyFilter{
				ShardID: 1, DomainID: domainID, WorkflowID: "wf-1", RunID: runID,
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().GetContext(
					gomock.Any(), gomock.Any(), gomock.Any(),
					getActiveClusterSelectionPolicyQuery,
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(sql.ErrNoRows)
			},
			wantErr: sql.ErrNoRows,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDriver := sqldriver.NewMockDriver(ctrl)
			tc.mockSetup(mockDriver)

			mdb := &DB{
				driver:      mockDriver,
				converter:   &converter{},
				numDBShards: 1,
			}
			got, err := mdb.SelectFromActiveClusterSelectionPolicy(context.Background(), tc.filter)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRow, got)
		})
	}
}

func TestDeleteFromActiveClusterSelectionPolicy(t *testing.T) {
	domainID := serialization.UUID(uuid.NewRandom())
	runID := serialization.UUID(uuid.NewRandom())

	tests := []struct {
		name      string
		filter    *sqlplugin.ActiveClusterSelectionPolicyFilter
		mockSetup func(*sqldriver.MockDriver)
		wantErr   bool
	}{
		{
			name: "successful delete",
			filter: &sqlplugin.ActiveClusterSelectionPolicyFilter{
				ShardID: 7, DomainID: domainID, WorkflowID: "wf-1", RunID: runID,
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().ExecContext(
					gomock.Any(),
					0,
					deleteActiveClusterSelectionPolicyQuery,
					7, domainID, "wf-1", runID,
				).Return(nil, nil)
			},
		},
		{
			name: "exec error is returned",
			filter: &sqlplugin.ActiveClusterSelectionPolicyFilter{
				ShardID: 1, DomainID: domainID, WorkflowID: "wf-1", RunID: runID,
			},
			mockSetup: func(mockDriver *sqldriver.MockDriver) {
				mockDriver.EXPECT().ExecContext(
					gomock.Any(), gomock.Any(),
					deleteActiveClusterSelectionPolicyQuery,
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(nil, errors.New("boom"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDriver := sqldriver.NewMockDriver(ctrl)
			tc.mockSetup(mockDriver)

			mdb := &DB{
				driver:      mockDriver,
				converter:   &converter{},
				numDBShards: 1,
			}
			_, err := mdb.DeleteFromActiveClusterSelectionPolicy(context.Background(), tc.filter)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
