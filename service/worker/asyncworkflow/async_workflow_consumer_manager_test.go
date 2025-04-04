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

package asyncworkflow

import (
	"errors"
	"fmt"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/asyncworkflow/queue"
	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type domainWithConfig struct {
	name                 string
	asyncWFCfg           types.AsyncWorkflowConfiguration
	failQueueCreation    bool
	failConsumerCreation bool
	failConsumerStart    bool
}

func TestConsumerManager(t *testing.T) {
	tests := []struct {
		name string
		// firstRoundDomains will be returned by the domain cache when the consumer manager starts
		firstRoundDomains []domainWithConfig
		// wantFirstRoundConsumers is the list of consumers that should be created after the first round
		wantFirstRoundConsumers []string
		// secondRoundDomains will be returned by the domain cache after the first refresh interval
		secondRoundDomains []domainWithConfig
		// wantSecondRoundConsumers is the list of consumers that should be created after the second round
		wantSecondRoundConsumers []string
	}{
		{
			name: "no domains",
		},
		{
			name: "empty queue config",
			firstRoundDomains: []domainWithConfig{
				{
					name:       "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{},
				},
			},
			wantFirstRoundConsumers: []string{},
		},
		{
			name: "queue creation fails",
			firstRoundDomains: []domainWithConfig{
				{
					name: "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             true,
						PredefinedQueueName: "queue1",
					},
					failQueueCreation: true,
				},
			},
			wantFirstRoundConsumers: []string{},
		},
		{
			name: "consumer creation fails",
			firstRoundDomains: []domainWithConfig{
				{
					name: "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             true,
						PredefinedQueueName: "queue1",
					},
					failConsumerCreation: true,
				},
			},
			wantFirstRoundConsumers: []string{},
		},
		{
			name: "consumer start fails",
			firstRoundDomains: []domainWithConfig{
				{
					name: "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             true,
						PredefinedQueueName: "queue1",
					},
					failConsumerStart: true,
				},
			},
			wantFirstRoundConsumers: []string{},
		},
		{
			name: "queue disabled",
			firstRoundDomains: []domainWithConfig{
				{
					name: "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             false,
						PredefinedQueueName: "queue1",
					},
				},
			},
			wantFirstRoundConsumers: []string{},
		},
		{
			name: "queue disabled in second round",
			firstRoundDomains: []domainWithConfig{
				{
					name: "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             true,
						PredefinedQueueName: "queue1",
					},
				},
			},
			wantFirstRoundConsumers: []string{"queue1"},
			secondRoundDomains: []domainWithConfig{
				{
					name: "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             false,
						PredefinedQueueName: "queue1",
					},
				},
			},
			wantSecondRoundConsumers: []string{},
		},
		{
			name: "same consumers for both rounds",
			firstRoundDomains: []domainWithConfig{
				{
					name: "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             true,
						PredefinedQueueName: "queue1",
					},
				},
			},
			wantFirstRoundConsumers: []string{"queue1"},
			secondRoundDomains: []domainWithConfig{
				{
					name: "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             true,
						PredefinedQueueName: "queue1",
					},
				},
			},
			wantSecondRoundConsumers: []string{"queue1"},
		},
		{
			name: "shared queue by multiple domains and different enable-disable states",
			firstRoundDomains: []domainWithConfig{
				{
					name: "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             true,
						PredefinedQueueName: "shared_queue",
					},
				},
				{
					name: "domain2",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             false,
						PredefinedQueueName: "shared_queue",
					},
				},
				{
					name: "domain3",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:   true,
						QueueType: "kafka",
						QueueConfig: &types.DataBlob{
							EncodingType: types.EncodingTypeJSON.Ptr(),
							Data:         []byte(`{"brokers":["localhost:9092"],"topics":["test-topic"]}`),
						},
					},
				},
			},
			wantFirstRoundConsumers: []string{
				"shared_queue",
				`queuetype:kafka,queueconfig:{"brokers":["localhost:9092"],"topics":["test-topic"]}`,
			},
			secondRoundDomains: []domainWithConfig{
				{
					name: "domain1",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             true,
						PredefinedQueueName: "shared_queue",
					},
				},
				{
					name: "domain2",
					asyncWFCfg: types.AsyncWorkflowConfiguration{
						Enabled:             true,
						PredefinedQueueName: "shared_queue",
					},
				},
			},
			wantSecondRoundConsumers: []string{"shared_queue"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockTimeSrc := clock.NewMockedTimeSource()
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			// setup mocks for 2 rounds of domain cache refresh
			mockQueueProvider := queue.NewMockProvider(ctrl)
			for _, domainsForRound := range [][]domainWithConfig{tc.firstRoundDomains, tc.secondRoundDomains} {
				mockDomainCache.EXPECT().
					GetAllDomain().
					Return(toDomainCacheEntries(domainsForRound)).
					Times(1)
				for _, dwc := range domainsForRound {
					if dwc.asyncWFCfg == (types.AsyncWorkflowConfiguration{}) {
						continue
					}

					if dwc.failQueueCreation {
						mockQueueProvider.EXPECT().
							GetPredefinedQueue(dwc.asyncWFCfg.PredefinedQueueName).
							Return(nil, errors.New("queue creation failed"))
					} else {
						queueMock := provider.NewMockQueue(ctrl)
						queueMock.EXPECT().ID().Return(queueID(dwc.asyncWFCfg)).AnyTimes()

						if dwc.asyncWFCfg.PredefinedQueueName != "" {
							mockQueueProvider.EXPECT().
								GetPredefinedQueue(dwc.asyncWFCfg.PredefinedQueueName).
								Return(queueMock, nil).AnyTimes()
						} else {
							mockQueueProvider.EXPECT().
								GetQueue(gomock.Any(), gomock.Any()).
								Return(queueMock, nil).AnyTimes()
						}
						if !dwc.asyncWFCfg.Enabled {
							continue
						}

						if dwc.failConsumerCreation {
							queueMock.EXPECT().CreateConsumer(gomock.Any()).
								Return(nil, errors.New("consumer creation failed")).AnyTimes()
						} else {
							mockConsumer := provider.NewMockConsumer(ctrl)
							var startErr error
							if dwc.failConsumerStart {
								startErr = errors.New("consumer start failed")
							}

							mockConsumer.EXPECT().Start().Return(startErr).AnyTimes()
							mockConsumer.EXPECT().Stop().AnyTimes()
							queueMock.EXPECT().CreateConsumer(gomock.Any()).Return(mockConsumer, nil).AnyTimes()
						}
					}
				}
			}

			// create consumer manager
			cm := NewConsumerManager(
				testlogger.New(t),
				metrics.NewNoopMetricsClient(),
				mockDomainCache,
				mockQueueProvider,
				nil,
				WithTimeSource(mockTimeSrc),
			)

			cm.Start()
			defer cm.Stop()

			// wait for the first round of consumers to be created
			time.Sleep(50 * time.Millisecond)
			// verify consumers
			t.Log("first round comparison")
			if diff := cmpQueueIDs(cm.activeConsumers, tc.wantFirstRoundConsumers); diff != "" {
				t.Fatalf("Consumer mismatch after first round (-want +got):\n%s", diff)
			}

			// wait for the second round of consumers to be created
			mockTimeSrc.Advance(defaultRefreshInterval)
			time.Sleep(50 * time.Millisecond)
			// verify consumers
			t.Log("second round comparison")
			if diff := cmpQueueIDs(cm.activeConsumers, tc.wantSecondRoundConsumers); diff != "" {
				t.Fatalf("Consumer mismatch after second round (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConsumerManagerEnabledDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockTimeSrc := clock.NewMockedTimeSource()
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockQueueProvider := queue.NewMockProvider(ctrl)
	dwc := domainWithConfig{
		name: "domain1",
		asyncWFCfg: types.AsyncWorkflowConfiguration{
			Enabled:   true,
			QueueType: "kafka",
			QueueConfig: &types.DataBlob{
				EncodingType: types.EncodingTypeJSON.Ptr(),
				Data:         []byte(`{"brokers":["localhost:9092"],"topics":["test-topic"]}`),
			},
		},
	}

	mockDomainCache.EXPECT().GetAllDomain().Return(toDomainCacheEntries([]domainWithConfig{dwc})).AnyTimes()
	queueMock := provider.NewMockQueue(ctrl)
	queueMock.EXPECT().ID().Return(queueID(dwc.asyncWFCfg)).AnyTimes()
	mockQueueProvider.EXPECT().GetQueue(gomock.Any(), gomock.Any()).Return(queueMock, nil).AnyTimes()
	mockConsumer := provider.NewMockConsumer(ctrl)
	mockConsumer.EXPECT().Start().Return(nil).AnyTimes()
	mockConsumer.EXPECT().Stop().AnyTimes()
	queueMock.EXPECT().CreateConsumer(gomock.Any()).Return(mockConsumer, nil).AnyTimes()

	var consumerMgrEnabled, consumerCount int32

	// create consumer manager
	cm := NewConsumerManager(
		testlogger.New(t),
		metrics.NewNoopMetricsClient(),
		mockDomainCache,
		mockQueueProvider,
		nil,
		WithTimeSource(mockTimeSrc),
		WithEnabledPropertyFn(func(opts ...dynamicproperties.FilterOption) bool {
			return atomic.LoadInt32(&consumerMgrEnabled) == 1
		}),
		WithEmitConsumerCountMetrifFn(func(count int) {
			atomic.StoreInt32(&consumerCount, int32(count))
		}),
	)

	cm.Start()
	defer cm.Stop()

	// wait for the first round of consumers to be created and verify consumer count
	atomic.StoreInt32(&consumerMgrEnabled, 1)
	time.Sleep(50 * time.Millisecond)
	t.Log("first round comparison")
	got := atomic.LoadInt32(&consumerCount)
	want := 1 // consumer manager is enabled
	if got != int32(want) {
		t.Fatalf("Consumer count mismatch after first round, want: %v, got: %v", want, got)
	}

	// disable consumer manager and wait for the second round of refresh
	atomic.StoreInt32(&consumerMgrEnabled, 0)
	mockTimeSrc.Advance(defaultRefreshInterval)
	time.Sleep(50 * time.Millisecond)
	got = atomic.LoadInt32(&consumerCount)
	want = 0 // all consumers should be stopped when consumer manager is disabled
	if got != int32(want) {
		t.Fatalf("Consumer count mismatch after second round, want: %v, got: %v", want, got)
	}

	// enable consumer manager and wait for the third round of refresh
	atomic.StoreInt32(&consumerMgrEnabled, 1)
	mockTimeSrc.Advance(defaultRefreshInterval)
	time.Sleep(50 * time.Millisecond)
	got = atomic.LoadInt32(&consumerCount)
	want = 1 // consumer manager is enabled
	if got != int32(want) {
		t.Fatalf("Consumer count mismatch after third round, want: %v, got: %v", want, got)
	}
}

func toDomainCacheEntries(domains []domainWithConfig) map[string]*cache.DomainCacheEntry {
	result := make(map[string]*cache.DomainCacheEntry, len(domains))
	for _, d := range domains {
		result[d.name] = cache.NewGlobalDomainCacheEntryForTest(
			&persistence.DomainInfo{
				Name: d.name,
			},
			&persistence.DomainConfig{
				AsyncWorkflowConfig: d.asyncWFCfg,
			},
			nil,
			0,
		)
	}
	return result
}

func queueID(asyncWFCfg types.AsyncWorkflowConfiguration) string {
	if asyncWFCfg.PredefinedQueueName != "" {
		return asyncWFCfg.PredefinedQueueName
	}

	if asyncWFCfg.QueueConfig == nil {
		return ""
	}

	return fmt.Sprintf("queuetype:%s,queueconfig:%s", asyncWFCfg.QueueType, string(asyncWFCfg.QueueConfig.Data))
}

func cmpQueueIDs(activeConsumers map[string]provider.Consumer, want []string) string {
	got := make([]string, 0, len(activeConsumers))
	for qID := range activeConsumers {
		got = append(got, qID)
	}

	if want == nil {
		want = []string{}
	}

	sort.Strings(got)
	sort.Strings(want)

	return cmp.Diff(want, got)
}
