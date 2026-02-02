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
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/sync/errgroup"

	"github.com/uber/cadence/common/persistence"
)

type beanmocks struct {
	mockCtrl           *gomock.Controller
	domainManager      *persistence.MockDomainManager
	domainAuditManager *persistence.MockDomainAuditManager
	taskManager        *persistence.MockTaskManager
	visibilityManager  *persistence.MockVisibilityManager
	replicationManager *persistence.MockQueueManager
	shardManager       *persistence.MockShardManager
	historyManager     *persistence.MockHistoryManager
	configManager      *persistence.MockConfigStoreManager
}

func beanSetup(t *testing.T) (f *MockFactory, m beanmocks, defaultMocks func()) {
	ctrl := gomock.NewController(t)
	m = beanmocks{
		mockCtrl:           ctrl,
		domainManager:      persistence.NewMockDomainManager(ctrl),
		domainAuditManager: persistence.NewMockDomainAuditManager(ctrl),
		taskManager:        persistence.NewMockTaskManager(ctrl),
		visibilityManager:  persistence.NewMockVisibilityManager(ctrl),
		replicationManager: persistence.NewMockQueueManager(ctrl),
		shardManager:       persistence.NewMockShardManager(ctrl),
		historyManager:     persistence.NewMockHistoryManager(ctrl),
		configManager:      persistence.NewMockConfigStoreManager(ctrl),
	}
	f = NewMockFactory(ctrl)
	defaultMocks = func() {
		// allow any of them to be called once or never, individual tests will set earlier mocks as needed
		f.EXPECT().NewDomainManager().Return(m.domainManager, nil).MaxTimes(1)
		f.EXPECT().NewDomainAuditManager().Return(m.domainAuditManager, nil).MaxTimes(1)
		f.EXPECT().NewTaskManager().Return(m.taskManager, nil).MaxTimes(1)
		f.EXPECT().NewVisibilityManager(gomock.Any(), gomock.Any()).Return(m.visibilityManager, nil).MaxTimes(1)
		f.EXPECT().NewDomainReplicationQueueManager().Return(m.replicationManager, nil).MaxTimes(1)
		f.EXPECT().NewShardManager().Return(m.shardManager, nil).MaxTimes(1)
		f.EXPECT().NewHistoryManager().Return(m.historyManager, nil).MaxTimes(1)
		f.EXPECT().NewConfigStoreManager().Return(m.configManager, nil).MaxTimes(1)
	}
	return f, m, defaultMocks
}

func TestBeanCoverage(t *testing.T) {
	// not particularly valuable but needed to hit min-% goals
	t.Run("NewBeanFromFactory", func(t *testing.T) {
		t.Parallel()
		// coverage for the New func, very straightforward err-branches.
		tests := map[string]struct {
			mockSetup func(t *testing.T, f *MockFactory)
			err       string
		}{
			"success": {
				mockSetup: func(t *testing.T, f *MockFactory) {
					// intentionally empty - setup already assume success
				},
				err: "",
			},
			"domain manager error": {
				mockSetup: func(t *testing.T, f *MockFactory) {
					f.EXPECT().NewDomainManager().Return(nil, fmt.Errorf("no domain manager"))
				},
				err: "no domain manager",
			},
			"task manager error": {
				mockSetup: func(t *testing.T, f *MockFactory) {
					f.EXPECT().NewTaskManager().Return(nil, fmt.Errorf("no task manager"))
				},
				err: "no task manager",
			},
			"visibility manager error": {
				mockSetup: func(t *testing.T, f *MockFactory) {
					f.EXPECT().NewVisibilityManager(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("no visibility manager"))
				},
				err: "no visibility manager",
			},
			"domain replication queue manager error": {
				mockSetup: func(t *testing.T, f *MockFactory) {
					f.EXPECT().NewDomainReplicationQueueManager().Return(nil, fmt.Errorf("no domain replication queue manager"))
				},
				err: "no domain replication queue manager",
			},
			"shard manager error": {
				mockSetup: func(t *testing.T, f *MockFactory) {
					f.EXPECT().NewShardManager().Return(nil, fmt.Errorf("no shard manager"))
				},
				err: "no shard manager",
			},
			"history manager error": {
				mockSetup: func(t *testing.T, f *MockFactory) {
					f.EXPECT().NewHistoryManager().Return(nil, fmt.Errorf("no history manager"))
				},
				err: "no history manager",
			},
			"config manager error": {
				mockSetup: func(t *testing.T, f *MockFactory) {
					f.EXPECT().NewConfigStoreManager().Return(nil, fmt.Errorf("no config manager"))
				},
				err: "no config manager",
			},
		}
		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				f, _, defaultMocks := beanSetup(t)
				test.mockSetup(t, f)
				defaultMocks()
				impl, err := NewBeanFromFactory(f, nil, nil)
				if test.err != "" {
					assert.ErrorContains(t, err, test.err)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, impl)
				}
			})
		}
	})
	t.Run("Simple getters", func(t *testing.T) {
		t.Parallel()
		f, m, defaultMocks := beanSetup(t)
		defaultMocks() // build all mock objects
		impl, err := NewBeanFromFactory(f, nil, nil)
		require.NoError(t, err)

		// these need to be concurrency-safe, so run them concurrently
		var g errgroup.Group
		g.Go(errgroupAssertEqual(t, m.domainManager, impl.GetDomainManager))
		g.Go(errgroupAssertEqual(t, m.domainAuditManager, impl.GetDomainAuditManager))
		g.Go(errgroupAssertEqual(t, m.taskManager, impl.GetTaskManager))
		g.Go(errgroupAssertEqual(t, m.visibilityManager, impl.GetVisibilityManager))
		g.Go(errgroupAssertEqual(t, m.replicationManager, impl.GetDomainReplicationQueueManager))
		g.Go(errgroupAssertEqual(t, m.shardManager, impl.GetShardManager))
		g.Go(errgroupAssertEqual(t, m.historyManager, impl.GetHistoryManager))
		g.Go(errgroupAssertEqual(t, m.configManager, impl.GetConfigStoreManager))
		require.NoError(t, g.Wait())
		// execution managers are per shard, checked separately
	})
	t.Run("Simple setters", func(t *testing.T) {
		t.Parallel()
		f, _, defaultMocks := beanSetup(t)
		_, m2, _ := beanSetup(t) // make a second set of mock objects, so they can be compared
		defaultMocks()           // allow constructors to be called
		impl, err := NewBeanFromFactory(f, nil, nil)
		require.NoError(t, err)

		// these need to be concurrency-safe, so run them concurrently
		var g errgroup.Group

		g.Go(errgroupAssertSets(t, m2.domainManager, impl.SetDomainManager, impl.GetDomainManager))
		g.Go(errgroupAssertSets(t, m2.domainAuditManager, impl.SetDomainAuditManager, impl.GetDomainAuditManager))
		g.Go(errgroupAssertSets(t, m2.taskManager, impl.SetTaskManager, impl.GetTaskManager))
		g.Go(errgroupAssertSets(t, m2.visibilityManager, impl.SetVisibilityManager, impl.GetVisibilityManager))
		g.Go(errgroupAssertSets(t, m2.replicationManager, impl.SetDomainReplicationQueueManager, impl.GetDomainReplicationQueueManager))
		g.Go(errgroupAssertSets(t, m2.shardManager, impl.SetShardManager, impl.GetShardManager))
		g.Go(errgroupAssertSets(t, m2.historyManager, impl.SetHistoryManager, impl.GetHistoryManager))
		g.Go(errgroupAssertSets(t, m2.configManager, impl.SetConfigStoreManager, impl.GetConfigStoreManager))
		require.NoError(t, g.Wait())
		// execution managers are per shard, checked separately
	})
	t.Run("Execution manager getter", func(t *testing.T) {
		t.Parallel()
		f, m, defaultMocks := beanSetup(t)

		// execution managers are per shard, make sure they're set up correctly
		ex1, ex2 := persistence.NewMockExecutionManager(m.mockCtrl), persistence.NewMockExecutionManager(m.mockCtrl)
		f.EXPECT().NewExecutionManager(1).Return(ex1, nil).Times(1)
		f.EXPECT().NewExecutionManager(2).Return(ex2, nil).Times(1)

		// build all other mock objects so New works.
		// these will not duplicate ^ above instances, so if they are used they'll fail assert.Equal checks.
		defaultMocks()
		impl, err := NewBeanFromFactory(f, nil, nil)
		require.NoError(t, err)

		// must be concurrency safe, so run them concurrently
		var g errgroup.Group
		g.Go(errgroupAssertExecutionManagerEqual(t, 1, ex1, impl))
		g.Go(errgroupAssertExecutionManagerEqual(t, 2, ex2, impl))

		// make sure re-getting doesn't make a duplicate / a different instance.
		// the `.Times(1)` also ensures this does not make an extra construction.
		g.Go(errgroupAssertExecutionManagerEqual(t, 1, ex1, impl))
		g.Go(errgroupAssertExecutionManagerEqual(t, 2, ex2, impl))
		require.NoError(t, g.Wait())
	})
	t.Run("Execution manager setter", func(t *testing.T) {
		t.Parallel()
		f, m, defaultMocks := beanSetup(t)

		// seed with some pre-existing data so we can say "not same as before".
		f.EXPECT().NewExecutionManager(1).Return(persistence.NewMockExecutionManager(m.mockCtrl), nil).Times(1)
		f.EXPECT().NewExecutionManager(2).Return(persistence.NewMockExecutionManager(m.mockCtrl), nil).Times(1)
		defaultMocks()
		impl, err := NewBeanFromFactory(f, nil, nil)
		require.NoError(t, err)
		_, err = impl.GetExecutionManager(1)
		require.NoError(t, err, "setup sanity check failed")
		_, err = impl.GetExecutionManager(2)
		require.NoError(t, err, "setup sanity check failed")

		// make the execution managers that will be set
		ex1, ex2 := persistence.NewMockExecutionManager(m.mockCtrl), persistence.NewMockExecutionManager(m.mockCtrl)

		// must be concurrency safe, so run them concurrently
		var g errgroup.Group
		g.Go(errgroupAssertSetsExecutionManager(t, 1, ex1, impl))
		g.Go(errgroupAssertSetsExecutionManager(t, 2, ex2, impl))
		require.NoError(t, g.Wait())
	})
	t.Run("Lifecycle", func(t *testing.T) {
		t.Parallel()
		f, m, defaultMocks := beanSetup(t)

		// make some execution managers so they can be closed
		ex1, ex2 := persistence.NewMockExecutionManager(m.mockCtrl), persistence.NewMockExecutionManager(m.mockCtrl)
		f.EXPECT().NewExecutionManager(1).Return(ex1, nil).Times(1)
		f.EXPECT().NewExecutionManager(2).Return(ex2, nil).Times(1)

		// expect everything to close
		m.domainManager.EXPECT().Close().Return().Times(1)
		m.domainAuditManager.EXPECT().Close().Return().Times(1)
		m.taskManager.EXPECT().Close().Return().Times(1)
		m.visibilityManager.EXPECT().Close().Return().Times(1)
		m.replicationManager.EXPECT().Close().Return().Times(1)
		m.shardManager.EXPECT().Close().Return().Times(1)
		m.historyManager.EXPECT().Close().Return().Times(1)
		m.configManager.EXPECT().Close().Return().Times(1)
		ex1.EXPECT().Close().Return().Times(1)
		ex2.EXPECT().Close().Return().Times(1)
		// which includes the execution-manager-factory itself
		f.EXPECT().Close().Return().Times(1)

		defaultMocks() // for New
		impl, err := NewBeanFromFactory(f, nil, nil)
		require.NoError(t, err)

		// seed the execution managers (could call Set instead)
		v, err := impl.GetExecutionManager(1)
		require.NoError(t, err)
		require.NotNil(t, v)
		v, err = impl.GetExecutionManager(2)
		require.NoError(t, err)
		require.NotNil(t, v)

		// ensure everything is closed
		impl.Close()
	})
}

// generics complains that the interface and mock-impl types don't match,
// so this is done via reflection.  testify ensures the values and types are the same.
//
// `actualFn` needs to be a `func() T` which returns the expected value/type.
func errgroupAssertEqual(t *testing.T, expected, getFn any) func() error {
	t.Helper()
	return func() error {
		t.Helper()
		assertMocksEqual(t, expected, reflect.ValueOf(getFn).Call(nil)[0].Interface())
		return nil
	}
}

// similar to above, not generics-friendly.
// setFn must be `func(T)` and getFn must be `func() T`, or the calls will panic.
func errgroupAssertSets(t *testing.T, expected, setFn, getFn any) func() error {
	t.Helper()
	return func() error {
		t.Helper()
		// sanity check first
		if !assertMocksNotEqual(t, expected, reflect.ValueOf(getFn).Call(nil)[0].Interface()) {
			return nil
		}

		// perform the SetX call.
		// panics if incorrect.
		reflect.ValueOf(setFn).Call([]reflect.Value{reflect.ValueOf(expected)})

		// make sure the getter gets the set value
		assertMocksEqual(t, expected, reflect.ValueOf(getFn).Call(nil)[0].Interface())
		return nil
	}
}

func errgroupAssertExecutionManagerEqual(t *testing.T, arg int, expected persistence.ExecutionManager, impl Bean) func() error {
	t.Helper()
	return func() error {
		t.Helper()
		val, err := impl.GetExecutionManager(arg)
		assert.NoError(t, err) // cannot use require in other goroutines
		assertMocksEqual(t, expected, val)
		return err
	}
}

func errgroupAssertSetsExecutionManager(t *testing.T, arg int, expected persistence.ExecutionManager, impl Bean) func() error {
	t.Helper()
	return func() error {
		t.Helper()

		// sanity check first
		val, err := impl.GetExecutionManager(arg)
		if !assert.NoError(t, err, "could not do initial Get to check values") {
			return nil
		}
		if !assertMocksNotEqual(t, expected, val) {
			return nil
		}

		impl.SetExecutionManager(arg, expected)

		val, err = impl.GetExecutionManager(arg)
		assert.NoError(t, err) // cannot use require in other goroutines
		assertMocksEqual(t, expected, val)
		return err
	}
}

// mock-equality helper because `assert.Equal` considers all mocks of the same type to be equal,
// as they (generally) use the same mock controller and recorder, and it ignores different pointers
// due to using reflect.DeepEqual.
func assertMocksEqual(t *testing.T, expected, actual any) bool {
	t.Helper()
	if actual != expected {
		t.Errorf("does not contain the expected reference, got: %T(%p) expected: %T(%p)", actual, actual, expected, expected)
		return false
	}
	return true
}

// mock-not-equality helper because `assert.NotEqual` considers all mocks of the same type to be equal,
// as they (generally) use the same mock controller and recorder, and it ignores different pointers
// due to using reflect.DeepEqual.
func assertMocksNotEqual(t *testing.T, expected, actual any) bool {
	t.Helper()
	if actual == expected {
		t.Errorf("actual and expected values (pointers) are identical")
		return false
	}
	return true
}
