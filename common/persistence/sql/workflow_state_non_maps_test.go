// Copyright (c) 2017-2020 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

package sql

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

func TestUpdateSignalsRequested(t *testing.T) {
	shardID := 1
	domainID := serialization.MustParseUUID("bf2360c6-62c7-48a2-bd95-fe6254bca628")
	workflowID := "test"
	runID := serialization.MustParseUUID("bf2360c6-a2c7-48a2-bd95-fe6254bca628")
	testCases := []struct {
		name                   string
		signalRequestedIDs     []string
		deleteSignalRequestIDs []string
		mockSetup              func(*sqlplugin.MockTx)
		wantErr                bool
	}{
		{
			name:                   "Success case",
			signalRequestedIDs:     []string{"s1", "s2"},
			deleteSignalRequestIDs: []string{"s3", "s4"},
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().InsertIntoSignalsRequestedSets(gomock.Any(), []sqlplugin.SignalsRequestedSetsRow{
					{
						ShardID:    int64(shardID),
						DomainID:   domainID,
						WorkflowID: workflowID,
						RunID:      runID,
						SignalID:   "s1",
					},
					{
						ShardID:    int64(shardID),
						DomainID:   domainID,
						WorkflowID: workflowID,
						RunID:      runID,
						SignalID:   "s2",
					},
				}).Return(nil, nil)
				mockTx.EXPECT().DeleteFromSignalsRequestedSets(gomock.Any(), &sqlplugin.SignalsRequestedSetsFilter{
					ShardID:    int64(shardID),
					DomainID:   domainID,
					WorkflowID: workflowID,
					RunID:      runID,
					SignalIDs:  []string{"s3", "s4"},
				}).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name:                   "Error case - failed to insert",
			signalRequestedIDs:     []string{"s1", "s2"},
			deleteSignalRequestIDs: []string{"s3", "s4"},
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
		{
			name:                   "Error case - failed to delete",
			signalRequestedIDs:     []string{"s1", "s2"},
			deleteSignalRequestIDs: []string{"s3", "s4"},
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, nil)
				mockTx.EXPECT().DeleteFromSignalsRequestedSets(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockTx := sqlplugin.NewMockTx(ctrl)

			tc.mockSetup(mockTx)
			err := updateSignalsRequested(context.Background(), mockTx, tc.signalRequestedIDs, tc.deleteSignalRequestIDs, shardID, domainID, workflowID, runID)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}

func TestUpdateBufferedEvents(t *testing.T) {
	shardID := 1
	domainID := serialization.MustParseUUID("bf2360c6-62c7-48a2-bd95-fe6254bca628")
	workflowID := "test"
	runID := serialization.MustParseUUID("bf2360c6-a2c7-48a2-bd95-fe6254bca628")
	testCases := []struct {
		name      string
		batch     *persistence.DataBlob
		mockSetup func(*sqlplugin.MockTx)
		wantErr   bool
	}{
		{
			name:  "Success case",
			batch: &persistence.DataBlob{Data: []byte(`buffer`), Encoding: constants.EncodingType("buffer")},
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				mockTx.EXPECT().InsertIntoBufferedEvents(gomock.Any(), []sqlplugin.BufferedEventsRow{
					{
						ShardID:      shardID,
						DomainID:     domainID,
						WorkflowID:   workflowID,
						RunID:        runID,
						Data:         []byte(`buffer`),
						DataEncoding: "buffer",
					},
				}).Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name:  "Error case",
			batch: &persistence.DataBlob{Data: []byte(`buffer`), Encoding: constants.EncodingType("buffer")},
			mockSetup: func(mockTx *sqlplugin.MockTx) {
				err := errors.New("some error")
				mockTx.EXPECT().InsertIntoBufferedEvents(gomock.Any(), gomock.Any()).Return(nil, err)
				mockTx.EXPECT().IsNotFoundError(err).Return(true)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockTx := sqlplugin.NewMockTx(ctrl)

			tc.mockSetup(mockTx)
			err := updateBufferedEvents(context.Background(), mockTx, tc.batch, shardID, domainID, workflowID, runID)
			if tc.wantErr {
				assert.Error(t, err, "Expected an error for test case")
			} else {
				assert.NoError(t, err, "Did not expect an error for test case")
			}
		})
	}
}
