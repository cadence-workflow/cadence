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

package metered

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func TestWrappers(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := frontend.NewMockClient(ctrl)

		testScope := tally.NewTestScope("", nil)
		metricsClient := metrics.NewClient(testScope, metrics.ServiceIdx(0))

		clientMock.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).Times(1)

		retryableClient := NewFrontendClient(
			clientMock,
			metricsClient)

		_, err := retryableClient.CountWorkflowExecutions(context.Background(), &types.CountWorkflowExecutionsRequest{})
		assert.NoError(t, err)
		assert.Len(t, testScope.Snapshot().Counters(), 1, "there should be a single counter registered")
		assert.Len(t, testScope.Snapshot().Timers(), 1, "there should be a single timer registered")
	})
	t.Run("error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := frontend.NewMockClient(ctrl)

		testScope := tally.NewTestScope("", nil)
		metricsClient := metrics.NewClient(testScope, metrics.ServiceIdx(0))

		clientMock.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("error"))

		retryableClient := NewFrontendClient(
			clientMock,
			metricsClient)

		_, err := retryableClient.CountWorkflowExecutions(context.Background(), &types.CountWorkflowExecutionsRequest{})
		assert.Error(t, err)
		assert.Len(t, testScope.Snapshot().Counters(), 2, "there should be two counters registered, one for call and one for failure")
	})
}

// Matching has a special logic that should emit a metric if a request is forwarded.
func TestMatching(t *testing.T) {
	t.Run("forwarded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := matching.NewMockClient(ctrl)

		testScope := tally.NewTestScope("", nil)
		metricsClient := metrics.NewClient(testScope, metrics.ServiceIdx(0))

		clientMock.EXPECT().AddActivityTask(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&types.AddActivityTaskResponse{}, nil).Times(1)

		retryableClient := NewMatchingClient(
			clientMock,
			metricsClient)

		_, err := retryableClient.AddActivityTask(context.Background(), &types.AddActivityTaskRequest{
			ForwardedFrom: "test",
			TaskList:      &types.TaskList{Name: "test"},
		})
		assert.NoError(t, err)
		assert.Len(t, testScope.Snapshot().Counters(), 2, "there should be two counters registered, one for call and one for forwarded")
	})
	t.Run("forwarded poller", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := matching.NewMockClient(ctrl)

		testScope := tally.NewTestScope("", nil)
		metricsClient := metrics.NewClient(testScope, metrics.ServiceIdx(0))

		clientMock.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&types.MatchingPollForDecisionTaskResponse{}, nil).Times(1)

		retryableClient := NewMatchingClient(
			clientMock,
			metricsClient)

		_, err := retryableClient.PollForDecisionTask(context.Background(), &types.MatchingPollForDecisionTaskRequest{
			ForwardedFrom: "test",
			PollRequest: &types.PollForDecisionTaskRequest{
				TaskList: &types.TaskList{Name: "test"},
			},
		})
		assert.NoError(t, err)
		assert.Len(t, testScope.Snapshot().Counters(), 2, "there should be two counters registered, one for call and one for forwarded")
	})
	t.Run("not forwarded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := matching.NewMockClient(ctrl)

		testScope := tally.NewTestScope("", nil)
		metricsClient := metrics.NewClient(testScope, metrics.ServiceIdx(0))

		clientMock.EXPECT().AddActivityTask(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&types.AddActivityTaskResponse{}, nil).Times(1)

		retryableClient := NewMatchingClient(
			clientMock,
			metricsClient)

		_, err := retryableClient.AddActivityTask(context.Background(), &types.AddActivityTaskRequest{
			ForwardedFrom: "",
			TaskList:      &types.TaskList{Name: "test"},
		})
		assert.NoError(t, err)
		assert.Len(t, testScope.Snapshot().Counters(), 1, "there should be one counters registered, one for call")
	})
	t.Run("invalid task list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := matching.NewMockClient(ctrl)

		testScope := tally.NewTestScope("", nil)
		metricsClient := metrics.NewClient(testScope, metrics.ServiceIdx(0))

		clientMock.EXPECT().AddActivityTask(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&types.AddActivityTaskResponse{}, nil).Times(1)

		retryableClient := NewMatchingClient(
			clientMock,
			metricsClient)

		_, err := retryableClient.AddActivityTask(context.Background(), &types.AddActivityTaskRequest{
			ForwardedFrom: "",
			TaskList:      &types.TaskList{Name: common.ReservedTaskListPrefix + "test"},
		})
		assert.NoError(t, err)
		assert.Len(t, testScope.Snapshot().Counters(), 2, "there should be two counters registered, one for call and one for invalid task list")
	})
}
