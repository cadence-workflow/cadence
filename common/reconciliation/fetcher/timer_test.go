// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

package fetcher

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/pagination"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
)

func TestTimerIterator(t *testing.T) {
	ctrl := gomock.NewController(t)
	retryer := persistence.NewMockRetryer(ctrl)
	retryer.EXPECT().GetTimerIndexTasks(gomock.Any(), gomock.Any()).
		Return(&persistence.GetTimerIndexTasksResponse{}, nil).
		Times(1)

	iterator := TimerIterator(
		context.Background(),
		retryer,
		time.Now(),
		time.Now(),
		10,
	)
	require.NotNil(t, iterator)
}

func TestGetUserTimers(t *testing.T) {
	fixedTimestamp, err := time.Parse(time.RFC3339, "2023-12-12T22:08:41Z")
	if err != nil {
		t.Fatalf("Failed to parse timestamp: %v", err)
	}

	pageSize := 10
	minTimestamp := fixedTimestamp.Add(-time.Hour)
	maxTimestamp := fixedTimestamp
	nonNilToken := []byte("non-nil-token")

	testCases := []struct {
		name          string
		setupMock     func(ctrl *gomock.Controller) *persistence.MockRetryer
		token         pagination.PageToken
		expectedPage  pagination.Page
		expectedError error
	}{
		{
			name: "Success",
			setupMock: func(ctrl *gomock.Controller) *persistence.MockRetryer {
				mockRetryer := persistence.NewMockRetryer(ctrl)
				timerTasks := []*persistence.TimerTaskInfo{
					{
						DomainID:            "testDomainID",
						WorkflowID:          "testWorkflowID",
						RunID:               "testRunID",
						VisibilityTimestamp: fixedTimestamp,
						TaskType:            persistence.TaskTypeUserTimer,
					},
				}

				mockRetryer.EXPECT().
					GetTimerIndexTasks(gomock.Any(), &persistence.GetTimerIndexTasksRequest{
						MinTimestamp:  minTimestamp,
						MaxTimestamp:  maxTimestamp,
						BatchSize:     pageSize,
						NextPageToken: nil,
					}).
					Return(&persistence.GetTimerIndexTasksResponse{
						Timers:        timerTasks,
						NextPageToken: nil,
					}, nil)

				mockRetryer.EXPECT().GetShardID().Return(123)

				return mockRetryer
			},
			token: nil,
			expectedPage: pagination.Page{
				Entities: []pagination.Entity{
					&entity.Timer{
						ShardID:             123,
						DomainID:            "testDomainID",
						WorkflowID:          "testWorkflowID",
						RunID:               "testRunID",
						TaskType:            persistence.TaskTypeUserTimer,
						VisibilityTimestamp: fixedTimestamp,
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "Non-nil Pagination Token Provided",
			setupMock: func(ctrl *gomock.Controller) *persistence.MockRetryer {
				mockRetryer := persistence.NewMockRetryer(ctrl)

				mockRetryer.EXPECT().
					GetTimerIndexTasks(gomock.Any(), &persistence.GetTimerIndexTasksRequest{
						MinTimestamp:  minTimestamp,
						MaxTimestamp:  maxTimestamp,
						BatchSize:     pageSize,
						NextPageToken: nonNilToken,
					}).
					Return(&persistence.GetTimerIndexTasksResponse{
						Timers:        nil,
						NextPageToken: nonNilToken,
					}, nil)

				return mockRetryer
			},
			token: nonNilToken,
			expectedPage: pagination.Page{
				Entities:     nil,
				CurrentToken: nonNilToken,
				NextToken:    nonNilToken,
			},
			expectedError: nil,
		},
		{
			name: "Invalid Timer Causes Error",
			setupMock: func(ctrl *gomock.Controller) *persistence.MockRetryer {
				mockRetryer := persistence.NewMockRetryer(ctrl)

				invalidTimer := &persistence.TimerTaskInfo{
					DomainID:            "", // Invalid as it's empty
					WorkflowID:          "testWorkflowID",
					RunID:               "testRunID",
					VisibilityTimestamp: fixedTimestamp,
					TaskType:            persistence.TaskTypeUserTimer,
				}

				mockRetryer.EXPECT().
					GetTimerIndexTasks(gomock.Any(), &persistence.GetTimerIndexTasksRequest{
						MinTimestamp:  minTimestamp,
						MaxTimestamp:  maxTimestamp,
						BatchSize:     pageSize,
						NextPageToken: nil,
					}).
					Return(&persistence.GetTimerIndexTasksResponse{
						Timers:        []*persistence.TimerTaskInfo{invalidTimer},
						NextPageToken: nil,
					}, nil)

				mockRetryer.EXPECT().GetShardID().Return(123)

				return mockRetryer
			},
			token:         nil,
			expectedPage:  pagination.Page{},
			expectedError: fmt.Errorf("empty DomainID"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRetryer := tc.setupMock(ctrl)

			fetchFn := getUserTimers(mockRetryer, minTimestamp, maxTimestamp, pageSize)
			page, err := fetchFn(context.Background(), tc.token)

			if tc.expectedError != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.expectedError.Error(), "Error should match")
			} else {
				require.NoError(t, err, "No error is expected")
			}

			require.Equal(t, tc.expectedPage, page, "Page should match")
		})
	}
}
