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

package client

import (
	"math/rand"
	"testing"

	"github.com/IBM/sarama/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/messaging/kafka"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
)

/*
New...Manager methods are intentionally NOT tested here.

They require importing a datastore plugin, which imports the persistence tests,
which imports this package, leading to a cycle.
They also generally try to connect to the target datastore *immediately* upon construction,
which means these would need to be integration tests...
...which we already have, in the persistence/persistence-tests package.
*/

func TestNew(t *testing.T) {
	// largely a sanity test for makeFactory, but it does ensure it constructs and closes,
	// though this does not actually achieve very much.
	fact := makeFactory(t)
	fact.Close()
}

// check ensures the func returns a value and no error.
// most manager-constructors can use this, but not all.
func check[T interface{}](t *testing.T, fn func() (T, error)) {
	val, err := fn()
	assert.NoError(t, err, "manager-constructor method should not error")
	assert.NotNil(t, val, "manager-constructor method should return a non-nil value")
}

func TestFactoryMethods(t *testing.T) {
	t.Run("NewTaskManager", func(t *testing.T) {
		fact := makeFactory(t)
		ds := mockDatastore(t, fact, storeTypeTask)

		ds.EXPECT().NewTaskStore().Return(nil, nil).MinTimes(1)
		check(t, fact.NewTaskManager)
	})
	t.Run("NewShardManager", func(t *testing.T) {
		fact := makeFactory(t)
		ds := mockDatastore(t, fact, storeTypeShard)

		ds.EXPECT().NewShardStore().Return(nil, nil).MinTimes(1)
		check(t, fact.NewShardManager)
	})
	t.Run("NewHistoryManager", func(t *testing.T) {
		fact := makeFactory(t)
		ds := mockDatastore(t, fact, storeTypeHistory)

		ds.EXPECT().NewHistoryStore().Return(nil, nil).MinTimes(1)
		check(t, fact.NewHistoryManager)
	})
	t.Run("NewDomainManager", func(t *testing.T) {
		fact := makeFactory(t)
		ds := mockDatastore(t, fact, storeTypeMetadata)

		ds.EXPECT().NewDomainStore().Return(nil, nil).MinTimes(1)
		check(t, fact.NewDomainManager)
	})
	t.Run("NewExecutionManager", func(t *testing.T) {
		// cannot control the persistence.NewExecutionManagerImpl call, so we must prevent calls to it.
		// currently the only call is the metrics wrapper calling GetShardID(), so just don't use the metrics wrapper.
		fact := makeFactoryWithMetrics(t, false)
		ds := mockDatastore(t, fact, storeTypeExecution)

		shard := rand.Int()
		ds.EXPECT().NewExecutionStore(shard).Return(nil, nil).MinTimes(1)
		em, err := fact.NewExecutionManager(shard)
		assert.NoError(t, err)
		assert.NotNil(t, em)
	})
	t.Run("NewVisibilityManager can be nil", func(t *testing.T) {
		fact := makeFactory(t)
		// no datastores are mocked as there are no calls at all expected
		vm, err := fact.NewVisibilityManager(
			nil, // params are unused
			&service.Config{
				// if both of these are nil, a nil response is correct,
				// because no "manager" is needed:
				// ES cannot be dynamically enabled, so no dual-writing / etc is
				// needed, so the baseline database store is sufficient.
				ReadVisibilityStoreName:  nil,
				WriteVisibilityStoreName: nil,
			})
		assert.NoError(t, err)
		assert.Nil(t, vm, "nil response is expected if advanced visibility cannot be enabled dynamically")
	})
	t.Run("NewDomainReplicationQueueManager", func(t *testing.T) {
		fact := makeFactory(t)
		ds := mockDatastore(t, fact, storeTypeQueue)

		ds.EXPECT().NewQueue(persistence.DomainReplicationQueueType).Return(nil, nil).MinTimes(1)
		check(t, fact.NewDomainReplicationQueueManager)
	})
	t.Run("NewConfigStoreManager", func(t *testing.T) {
		fact := makeFactory(t)
		ds := mockDatastore(t, fact, storeTypeConfigStore)

		ds.EXPECT().NewConfigStore().Return(nil, nil).MinTimes(1)
		check(t, fact.NewConfigStoreManager)
	})
	t.Run("NewVisibilityManager_TripleVisibilityManager_Pinot", func(t *testing.T) {
		fact := makeFactory(t)
		ds := mockDatastore(t, fact, storeTypeVisibility)

		logger := testlogger.New(t)
		mc := messaging.NewMockClient(gomock.NewController(t))
		mc.EXPECT().NewProducer(gomock.Any()).Return(kafka.NewKafkaProducer("test-topic", mocks.NewSyncProducer(t, nil), logger), nil).MinTimes(1)
		readFromClosed := true
		testAttributes := map[string]interface{}{
			"CustomAttribute": "test", // Define your custom attributes and types
		}

		ds.EXPECT().NewVisibilityStore(readFromClosed).Return(nil, nil).MinTimes(1)
		_, err := fact.NewVisibilityManager(&Params{
			PersistenceConfig: config.Persistence{
				// a configured VisibilityStore uses the db store, which is mockable,
				// unlike basically every other store.
				AdvancedVisibilityStore: "pinot-visibility",
				VisibilityStore:         "fake",
				DataStores: map[string]config.DataStore{
					"pinot-visibility": {
						Pinot: &config.PinotVisibilityConfig{
							// fields are unused but must be non-nil
							Cluster: "cluster",
							Broker:  "broker",
						}, // fields are unused but must be non-nil
					},
				},
			},
			MessagingClient: mc,
			PinotConfig: &config.PinotVisibilityConfig{
				Migration: config.VisibilityMigration{
					Enabled: true,
				},
			},
			ESConfig: &config.ElasticSearchConfig{
				Indices: map[string]string{
					"visibility": "test-index",
				},
			},
		}, &service.Config{
			// must be non-nil to create a "manager", else nil return from NewVisibilityManager is expected
			ReadVisibilityStoreName: func(domain string) string {
				return "es" // any value is fine as there are no read calls
			},
			// non-nil avoids a warning log
			EnableReadDBVisibilityFromClosedExecutionV2: func(opts ...dynamicproperties.FilterOption) bool {
				return readFromClosed // any value is fine as there are no read calls
			},
			ValidSearchAttributes: func(opts ...dynamicproperties.FilterOption) map[string]interface{} {
				return testAttributes
			},
		})
		assert.NoError(t, err)
	})
	t.Run("NewVisibilityManager_DualVisibilityManager_ES", func(t *testing.T) {
		fact := makeFactory(t)
		ds := mockDatastore(t, fact, storeTypeVisibility)

		logger := testlogger.New(t)
		mc := messaging.NewMockClient(gomock.NewController(t))
		mc.EXPECT().NewProducer(gomock.Any()).Return(kafka.NewKafkaProducer("test-topic", mocks.NewSyncProducer(t, nil), logger), nil).MinTimes(1)
		readFromClosed := true
		testAttributes := map[string]interface{}{
			"CustomAttribute": "test", // Define your custom attributes and types
		}

		ds.EXPECT().NewVisibilityStore(readFromClosed).Return(nil, nil).MinTimes(1)
		_, err := fact.NewVisibilityManager(&Params{
			PersistenceConfig: config.Persistence{
				// a configured VisibilityStore uses the db store, which is mockable,
				// unlike basically every other store.
				AdvancedVisibilityStore: "es-visibility",
				VisibilityStore:         "fake",
				DataStores: map[string]config.DataStore{
					"es-visibility": {
						ElasticSearch: &config.ElasticSearchConfig{
							// fields are unused but must be non-nil
							Indices: map[string]string{
								"visibility": "test-index",
							},
						}, // fields are unused but must be non-nil
					},
				},
			},
			MessagingClient: mc,
			ESConfig: &config.ElasticSearchConfig{
				Indices: map[string]string{
					"visibility": "test-index",
				},
			},
		}, &service.Config{
			// must be non-nil to create a "manager", else nil return from NewVisibilityManager is expected
			ReadVisibilityStoreName: func(domain string) string {
				return "es" // any value is fine as there are no read calls
			},
			// non-nil avoids a warning log
			EnableReadDBVisibilityFromClosedExecutionV2: func(opts ...dynamicproperties.FilterOption) bool {
				return readFromClosed // any value is fine as there are no read calls
			},
			ValidSearchAttributes: func(opts ...dynamicproperties.FilterOption) map[string]interface{} {
				return testAttributes
			},
		})
		assert.NoError(t, err)
	})
}

func makeFactory(t *testing.T) Factory {
	return makeFactoryWithMetrics(t, true)
}

func makeFactoryWithMetrics(t *testing.T, withMetrics bool) Factory {
	qpsFn := func() float64 { return 1000 } // anything non-zero exercises the ratelimit config paths
	logger := testlogger.New(t)
	var met metrics.Client
	if withMetrics {
		met = metrics.NewClient(tally.NewTestScope("", nil), service.GetMetricsServiceIdx(service.Frontend, logger), metrics.HistogramMigration{})
	}
	ctrl := gomock.NewController(t)
	dc := dynamicconfig.NewCollection(dynamicconfig.NewMockClient(ctrl), logger)
	pdc := persistence.NewDynamicConfiguration(dc)

	cfg := &config.Persistence{
		DefaultStore:            "fake",
		VisibilityStore:         "fake",
		AdvancedVisibilityStore: "fake",
		NumHistoryShards:        1024,
		DataStores: map[string]config.DataStore{
			"fake": {
				NoSQL:         &config.NoSQL{},                 // fields are unused but must be non-nil
				ElasticSearch: &config.ElasticSearchConfig{},   // fields are unused but must be non-nil
				Pinot:         &config.PinotVisibilityConfig{}, // fields are unused but must be non-nil
			},
		},
		TransactionSizeLimit: nil,
		ErrorInjectionRate: func(opts ...dynamicproperties.FilterOption) float64 {
			return 0.5 // half errors, unused in these tests beyond "nonzero" so it wraps with the error injector
		},
	}

	return NewFactory(cfg, qpsFn, "test cluster", met, logger, pdc)
}

func mockDatastore(t *testing.T, fact Factory, store storeType) *MockDataStoreFactory {
	ctrl := gomock.NewController(t)
	impl := fact.(*factoryImpl)

	// I would prefer to mock everything so calls to the wrong store could have better errors,
	// but there doesn't seem to be a way to "name" these or say "any call fails with X message",
	// nor can you check what calls occurred after the fact.
	//
	// so just mock the only thing expected, so you get a nil panic stack trace when it fails
	// somewhere in a persistence database connection or something.  it's sometimes easier to debug.
	mock := NewMockDataStoreFactory(ctrl)
	ds := impl.datastores[store]
	ds.factory = mock
	impl.datastores[store] = ds // write back the value type

	return mock
}

func TestVisibilityManagers(t *testing.T) {
	tests := []struct {
		name        string
		advanced    string
		datastores  map[string]config.DataStore
		esConfig    *config.ElasticSearchConfig
		osConfig    *config.ElasticSearchConfig
		pinotConfig *config.PinotVisibilityConfig
	}{
		{
			name:     "TripleVisibilityManager_OpenSearch",
			advanced: "os-visibility",
			datastores: map[string]config.DataStore{
				"os-visibility": {
					ElasticSearch: &config.ElasticSearchConfig{
						// fields are unused but must be non-nil
						Indices: map[string]string{
							"visibility": "test-index",
						},
					}, // fields are unused but must be non-nil
				},
			},
			osConfig: &config.ElasticSearchConfig{
				Indices: map[string]string{
					"visibility": "test-index",
				},
				Migration: config.VisibilityMigration{
					Enabled: true,
				},
			},
			esConfig: &config.ElasticSearchConfig{
				Indices: map[string]string{
					"visibility": "test-index",
				},
			},
		},
		{
			name:     "NewVisibilityManager_TripleVisibilityManager_Pinot",
			advanced: "pinot-visibility",
			datastores: map[string]config.DataStore{
				"pinot-visibility": {
					Pinot: &config.PinotVisibilityConfig{
						Cluster: "cluster",
						Broker:  "broker",
					},
				},
			},
			pinotConfig: &config.PinotVisibilityConfig{
				Migration: config.VisibilityMigration{
					Enabled: true,
				},
			},
			esConfig: &config.ElasticSearchConfig{
				Indices: map[string]string{
					"visibility": "test-index",
				},
			},
		},
		{
			name:     "NewVisibilityManager_DualVisibilityManager_Pinot",
			advanced: "pinot-visibility",
			datastores: map[string]config.DataStore{
				"pinot-visibility": {
					Pinot: &config.PinotVisibilityConfig{
						Cluster: "cluster",
						Broker:  "broker",
					},
				},
			},
			pinotConfig: &config.PinotVisibilityConfig{
				Migration: config.VisibilityMigration{
					Enabled: false,
				},
			},
		},
		{
			name:     "NewVisibilityManager_DualVisibilityManager_ES",
			advanced: "es-visibility",
			datastores: map[string]config.DataStore{
				"es-visibility": {
					ElasticSearch: &config.ElasticSearchConfig{
						Indices: map[string]string{
							"visibility": "test-index",
						},
					},
				},
			},
			esConfig: &config.ElasticSearchConfig{
				Indices: map[string]string{
					"visibility": "test-index",
				},
			},
		},
		{
			name:     "NewVisibilityManager_DualVisibilityManager_OS",
			advanced: "os-visibility",
			datastores: map[string]config.DataStore{
				"os-visibility": {
					ElasticSearch: &config.ElasticSearchConfig{
						Indices: map[string]string{
							"visibility": "test-index",
						},
					},
				},
			},
			osConfig: &config.ElasticSearchConfig{
				Indices: map[string]string{
					"visibility": "test-index",
				},
				Migration: config.VisibilityMigration{
					Enabled: false,
				},
			},
		},
	}

	for _, test := range tests {
		fact := makeFactory(t)
		ds := mockDatastore(t, fact, storeTypeVisibility)
		ds.EXPECT().NewVisibilityStore(true).Return(nil, nil).MinTimes(1)
		mc := messaging.NewMockClient(gomock.NewController(t))
		mc.EXPECT().NewProducer(gomock.Any()).Return(kafka.NewKafkaProducer("test-topic", mocks.NewSyncProducer(t, nil), testlogger.New(t)), nil).MinTimes(1)

		_, err := fact.NewVisibilityManager(&Params{
			PersistenceConfig: config.Persistence{
				AdvancedVisibilityStore: test.advanced,
				VisibilityStore:         "fake",
				DataStores:              test.datastores,
			},
			MessagingClient: mc,
			PinotConfig:     test.pinotConfig,
			ESConfig:        test.esConfig,
			OSConfig:        test.osConfig,
		}, &service.Config{
			// must be non-nil to create a "manager", else nil return from NewVisibilityManager is expected
			ReadVisibilityStoreName: func(domain string) string {
				return "es" // any value is fine as there are no read calls
			},
			// non-nil avoids a warning log
			EnableReadDBVisibilityFromClosedExecutionV2: func(opts ...dynamicproperties.FilterOption) bool {
				return true // any value is fine as there are no read calls
			},
			ValidSearchAttributes: func(opts ...dynamicproperties.FilterOption) map[string]interface{} {
				return map[string]interface{}{
					"CustomAttribute": "test",
				}
			},
		})
		assert.NoError(t, err)
	}
}
