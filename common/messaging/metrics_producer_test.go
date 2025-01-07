// Copyright (c) 2017 Uber Technologies, Inc.
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

package messaging

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
)

func TestPublish(t *testing.T) {
	tests := []struct {
		desc                string
		tags                []metrics.Tag
		producerFails       bool
		metricsClientMockFn func() *mocks.Client
	}{
		{
			desc:          "success",
			producerFails: false,
			tags: []metrics.Tag{
				metrics.TopicTag("test-topic-1"),
			},
			metricsClientMockFn: func() *mocks.Client {
				metricsClient := &mocks.Client{}
				metricsScope := &mocks.Scope{}
				metricsClient.
					On("Scope", metrics.MessagingClientPublishScope, metrics.TopicTag("test-topic-1")).
					Return(metricsScope).
					Once()
				metricsScope.On("IncCounter", metrics.CadenceClientRequests).Once()

				sw := metrics.NoopScope(metrics.MessagingClientPublishScope).StartTimer(-1)
				metricsScope.On("StartTimer", metrics.CadenceClientLatency).Return(sw).Once()
				return metricsClient
			},
		},
		{
			desc:          "failure",
			producerFails: true,
			tags: []metrics.Tag{
				metrics.TopicTag("test-topic-2"),
			},
			metricsClientMockFn: func() *mocks.Client {
				metricsClient := &mocks.Client{}
				metricsScope := &mocks.Scope{}
				metricsClient.
					On("Scope", metrics.MessagingClientPublishScope, metrics.TopicTag("test-topic-2")).
					Return(metricsScope).
					Once()
				metricsScope.On("IncCounter", metrics.CadenceClientRequests).Once()
				metricsScope.On("IncCounter", metrics.CadenceClientFailures).Once()

				sw := metrics.NoopScope(metrics.MessagingClientPublishScope).StartTimer(-1)
				metricsScope.On("StartTimer", metrics.CadenceClientLatency).Return(sw).Once()
				return metricsClient
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// setup
			ctrl := gomock.NewController(t)
			mockProducer := NewMockProducer(ctrl)
			msg := "custom-message"
			if tc.producerFails {
				mockProducer.EXPECT().Publish(gomock.Any(), msg).Return(errors.New("publish failed")).Times(1)
			} else {
				mockProducer.EXPECT().Publish(gomock.Any(), msg).Return(nil).Times(1)
			}
			metricsClient := tc.metricsClientMockFn()

			// create producer and call publish
			p := NewMetricProducer(mockProducer, metricsClient, WithMetricTags(tc.tags...))
			err := p.Publish(context.Background(), msg)

			// validations
			if tc.producerFails != (err != nil) {
				t.Errorf("expected producer to fail: %v, got: %v", tc.producerFails, err)
			}
			if err != nil {
				return
			}

			metricsClient.AssertExpectations(t)
		})
	}
}
