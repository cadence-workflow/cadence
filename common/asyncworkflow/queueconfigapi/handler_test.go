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

package queueconfigapi

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

func TestGetConfiguraton(t *testing.T) {
	tests := map[string]struct {
		req                 *types.GetDomainAsyncWorkflowConfiguratonRequest
		domainHandlerMockFn func(*domain.MockHandler)
		wantResp            *types.GetDomainAsyncWorkflowConfiguratonResponse
		wantErr             bool
	}{
		"Domain handler fails to DescribeDomain": {
			req: &types.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
			domainHandlerMockFn: func(m *domain.MockHandler) {
				m.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed")).Times(1)
			},
			wantErr: true,
		},
		"Domain config is nil": {
			req: &types.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
			domainHandlerMockFn: func(m *domain.MockHandler) {
				resp := &types.DescribeDomainResponse{
					Configuration: nil,
				}
				m.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
			},
			wantResp: &types.GetDomainAsyncWorkflowConfiguratonResponse{},
		},
		"Domain async wf config is nil": {
			req: &types.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
			domainHandlerMockFn: func(m *domain.MockHandler) {
				resp := &types.DescribeDomainResponse{
					Configuration: &types.DomainConfiguration{
						AsyncWorkflowConfig: nil,
					},
				}
				m.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
			},
			wantResp: &types.GetDomainAsyncWorkflowConfiguratonResponse{},
		},
		"Success": {
			req: &types.GetDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
			domainHandlerMockFn: func(m *domain.MockHandler) {
				resp := &types.DescribeDomainResponse{
					Configuration: &types.DomainConfiguration{
						AsyncWorkflowConfig: &types.AsyncWorkflowConfiguration{
							Enabled:             true,
							PredefinedQueueName: "test-queue",
							QueueType:           "kafka",
							QueueConfig: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         []byte(`{"topic":"test-topic","dlq_topic":"test-dlq-topic","consumer_group":"test-consumer-group","brokers":["test-broker"]}`),
							},
						},
					},
				}
				m.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
			},
			wantResp: &types.GetDomainAsyncWorkflowConfiguratonResponse{
				Configuration: &types.AsyncWorkflowConfiguration{
					Enabled:             true,
					PredefinedQueueName: "test-queue",
					QueueType:           "kafka",
					QueueConfig: &types.DataBlob{
						EncodingType: types.EncodingTypeJSON.Ptr(),
						Data:         []byte(`{"topic":"test-topic","dlq_topic":"test-dlq-topic","consumer_group":"test-consumer-group","brokers":["test-broker"]}`),
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			domainHandlerMock := domain.NewMockHandler(ctrl)
			if tc.domainHandlerMockFn != nil {
				tc.domainHandlerMockFn(domainHandlerMock)
			}

			handler := New(testlogger.New(t), domainHandlerMock)
			resp, err := handler.GetConfiguraton(context.Background(), tc.req)

			if tc.wantErr != (err != nil) {
				t.Fatalf("Error mismatch. Got: %v, want?: %v", err, tc.wantErr)
			}

			assert.Equal(t, tc.wantResp, resp)
		})
	}
}

func TestUpdateConfiguration(t *testing.T) {
	tests := map[string]struct {
		req                 *types.UpdateDomainAsyncWorkflowConfiguratonRequest
		domainHandlerMockFn func(*domain.MockHandler)
		wantResp            *types.UpdateDomainAsyncWorkflowConfiguratonResponse
		wantErr             bool
	}{
		"Domain handler fails to UpdateAsyncWorkflowConfiguraton": {
			req: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
			domainHandlerMockFn: func(m *domain.MockHandler) {
				m.EXPECT().UpdateAsyncWorkflowConfiguraton(gomock.Any(), gomock.Any()).Return(errors.New("failed")).Times(1)
			},
			wantErr: true,
		},
		"nil request": {
			req:     nil,
			wantErr: true,
		},
		"Success": {
			req: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
				Domain: "test-domain",
			},
			domainHandlerMockFn: func(m *domain.MockHandler) {
				m.EXPECT().UpdateAsyncWorkflowConfiguraton(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantResp: &types.UpdateDomainAsyncWorkflowConfiguratonResponse{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			domainHandlerMock := domain.NewMockHandler(ctrl)
			if tc.domainHandlerMockFn != nil {
				tc.domainHandlerMockFn(domainHandlerMock)
			}

			handler := New(testlogger.New(t), domainHandlerMock)
			resp, err := handler.UpdateConfiguration(context.Background(), tc.req)

			if tc.wantErr != (err != nil) {
				t.Fatalf("Error mismatch. Got: %v, want?: %v", err, tc.wantErr)
			}

			assert.Equal(t, tc.wantResp, resp)
		})
	}
}
