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

package retryable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func TestFrontendClientRetryableError(t *testing.T) {
	ctrl := gomock.NewController(t)
	clientMock := frontend.NewMockClient(ctrl)
	// One failure, one success
	clientMock.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &types.ServiceBusyError{
			Message: "error",
		}).Times(1)
	clientMock.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).Times(1)

	retryableClient := NewFrontendClient(
		clientMock,
		common.CreateFrontendServiceRetryPolicy(),
		common.IsServiceBusyError)

	_, err := retryableClient.CountWorkflowExecutions(context.Background(), &types.CountWorkflowExecutionsRequest{})
	assert.NoError(t, err)
}

func TestFrontendClientNonRetryableError(t *testing.T) {
	ctrl := gomock.NewController(t)
	clientMock := frontend.NewMockClient(ctrl)
	// One failure, one success
	clientMock.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &types.BadRequestError{
			Message: "error",
		}).Times(1)

	retryableClient := NewFrontendClient(
		clientMock,
		common.CreateFrontendServiceRetryPolicy(),
		common.IsServiceBusyError)

	_, err := retryableClient.CountWorkflowExecutions(context.Background(), &types.CountWorkflowExecutionsRequest{})
	assert.Error(t, err)
}

func TestMatchingClientRetries(t *testing.T) {
	transientErr := &types.ServiceBusyError{Message: "busy"}

	tests := []struct {
		name    string
		wantErr error
		call    func(*matching.MockClient, matching.Client) error
	}{
		{
			name:    "PollForActivityTask is not retried",
			wantErr: transientErr,
			call: func(m *matching.MockClient, c matching.Client) error {
				m.EXPECT().PollForActivityTask(gomock.Any(), gomock.Any()).Return(nil, transientErr).Times(1)
				_, err := c.PollForActivityTask(context.Background(), &types.MatchingPollForActivityTaskRequest{})
				return err
			},
		},
		{
			name:    "PollForDecisionTask is not retried",
			wantErr: transientErr,
			call: func(m *matching.MockClient, c matching.Client) error {
				m.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any()).Return(nil, transientErr).Times(1)
				_, err := c.PollForDecisionTask(context.Background(), &types.MatchingPollForDecisionTaskRequest{})
				return err
			},
		},
		{
			name: "QueryWorkflow is retried",
			call: func(m *matching.MockClient, c matching.Client) error {
				gomock.InOrder(
					m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(nil, transientErr),
					m.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(&types.MatchingQueryWorkflowResponse{}, nil),
				)
				_, err := c.QueryWorkflow(context.Background(), &types.MatchingQueryWorkflowRequest{})
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientMock := matching.NewMockClient(gomock.NewController(t))
			retryableClient := NewMatchingClient(clientMock, common.CreateMatchingServiceRetryPolicy(), common.IsServiceBusyError)

			err := tt.call(clientMock, retryableClient)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
