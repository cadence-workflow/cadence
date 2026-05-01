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

package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/asyncworkflow/queue"
	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestGetProducerByDomain(t *testing.T) {
	testCases := []struct {
		name      string
		domain    string
		mockSetup func(*cache.MockDomainCache, *queue.MockProvider, *provider.MockQueue, *cache.MockCache)
		wantErr   bool
	}{
		{
			name:   "Success case - cache miss, predefined queue",
			domain: "test-domain",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockProvider *queue.MockProvider, mockQueue *provider.MockQueue, mockProducerCache *cache.MockCache) {
				mockDomainCache.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					nil,
					&persistence.DomainConfig{
						AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
							Enabled:             true,
							PredefinedQueueName: "testQueue",
						},
					},
					nil,
					0,
				), nil)
				mockProvider.EXPECT().GetPredefinedQueue("testQueue").Return(mockQueue, nil)
				mockQueue.EXPECT().ID().Return("q1")
				mockProducerCache.EXPECT().Get(gomock.Any()).Return(nil)
				producer := messaging.NewNoopProducer()
				mockQueue.EXPECT().CreateProducer(gomock.Any()).Return(producer, nil)
				mockProducerCache.EXPECT().PutIfNotExist("q1", producer).Return(producer, nil)
			},
			wantErr: false,
		},
		{
			name:   "Success case - cache miss, customized queue",
			domain: "test-domain",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockProvider *queue.MockProvider, mockQueue *provider.MockQueue, mockProducerCache *cache.MockCache) {
				mockDomainCache.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					nil,
					&persistence.DomainConfig{
						AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
							Enabled:   true,
							QueueType: "kafka",
							QueueConfig: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         []byte(`{"brokers":["localhost:9092"],"topics":["test-topic"]}`),
							},
						},
					},
					nil,
					0,
				), nil)
				mockProvider.EXPECT().GetQueue("kafka", gomock.Any()).Return(mockQueue, nil)
				mockQueue.EXPECT().ID().Return("q1")
				mockProducerCache.EXPECT().Get(gomock.Any()).Return(nil)
				producer := messaging.NewNoopProducer()
				mockQueue.EXPECT().CreateProducer(gomock.Any()).Return(producer, nil)
				mockProducerCache.EXPECT().PutIfNotExist("q1", producer).Return(producer, nil)
			},
			wantErr: false,
		},
		{
			name:   "Success case - cache hit, predefined queue",
			domain: "test-domain",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockProvider *queue.MockProvider, mockQueue *provider.MockQueue, mockProducerCache *cache.MockCache) {
				mockDomainCache.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					nil,
					&persistence.DomainConfig{
						AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
							Enabled:             true,
							PredefinedQueueName: "testQueue",
						},
					},
					nil,
					0,
				), nil)
				mockProvider.EXPECT().GetPredefinedQueue("testQueue").Return(mockQueue, nil)
				mockQueue.EXPECT().ID().Return("q1")
				producer := messaging.NewNoopProducer()
				mockProducerCache.EXPECT().Get(gomock.Any()).Return(producer)
			},
			wantErr: false,
		},
		{
			name:   "Success case - cache hit, customized queue",
			domain: "test-domain",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockProvider *queue.MockProvider, mockQueue *provider.MockQueue, mockProducerCache *cache.MockCache) {
				mockDomainCache.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					nil,
					&persistence.DomainConfig{
						AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
							Enabled:   true,
							QueueType: "kafka",
							QueueConfig: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         []byte(`{"brokers":["localhost:9092"],"topics":["test-topic"]}`),
							},
						},
					},
					nil,
					0,
				), nil)
				mockProvider.EXPECT().GetQueue("kafka", gomock.Any()).Return(mockQueue, nil)
				mockQueue.EXPECT().ID().Return("q1")
				producer := messaging.NewNoopProducer()
				mockProducerCache.EXPECT().Get(gomock.Any()).Return(producer)
			},
			wantErr: false,
		},
		{
			name:   "Error case - domain cache error",
			domain: "test-domain",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockProvider *queue.MockProvider, mockQueue *provider.MockQueue, mockProducerCache *cache.MockCache) {
				mockDomainCache.EXPECT().GetDomain("test-domain").Return(nil, fmt.Errorf("error"))
			},
			wantErr: true,
		},
		{
			name:   "Error case - provider error",
			domain: "test-domain",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockProvider *queue.MockProvider, mockQueue *provider.MockQueue, mockProducerCache *cache.MockCache) {
				mockDomainCache.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					nil,
					&persistence.DomainConfig{
						AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
							Enabled:   true,
							QueueType: "kafka",
							QueueConfig: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         []byte(`{"brokers":["localhost:9092"],"topics":["test-topic"]}`),
							},
						},
					},
					nil,
					0,
				), nil)
				mockProvider.EXPECT().GetQueue("kafka", gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
			wantErr: true,
		},
		{
			name:   "Error case - cache miss, create producer error",
			domain: "test-domain",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockProvider *queue.MockProvider, mockQueue *provider.MockQueue, mockProducerCache *cache.MockCache) {
				mockDomainCache.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					nil,
					&persistence.DomainConfig{
						AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
							Enabled:   true,
							QueueType: "kafka",
							QueueConfig: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         []byte(`{"brokers":["localhost:9092"],"topics":["test-topic"]}`),
							},
						},
					},
					nil,
					0,
				), nil)
				mockProvider.EXPECT().GetQueue("kafka", gomock.Any()).Return(mockQueue, nil)
				mockQueue.EXPECT().ID().Return("q1")
				mockProducerCache.EXPECT().Get(gomock.Any()).Return(nil)
				mockQueue.EXPECT().CreateProducer(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
			wantErr: true,
		},
		{
			name:   "Error case - async wf not enabled",
			domain: "test-domain",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockProvider *queue.MockProvider, mockQueue *provider.MockQueue, mockProducerCache *cache.MockCache) {
				mockDomainCache.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					nil,
					&persistence.DomainConfig{
						AsyncWorkflowConfig: types.AsyncWorkflowConfiguration{
							Enabled: false,
						},
					},
					nil,
					0,
				), nil)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			mockProvider := queue.NewMockProvider(mockCtrl)
			mockQueue := provider.NewMockQueue(mockCtrl)
			mockProducerCache := cache.NewMockCache(mockCtrl)

			producerManager := NewProducerManager(
				mockDomainCache,
				mockProvider,
				log.NewNoop(),
				metrics.NewNoopMetricsClient(),
			)
			producerManager.(*producerManagerImpl).producerCache = mockProducerCache

			tc.mockSetup(mockDomainCache, mockProvider, mockQueue, mockProducerCache)

			producer, err := producerManager.GetProducerByDomain(tc.domain)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, producer)
			}
		})
	}
}
