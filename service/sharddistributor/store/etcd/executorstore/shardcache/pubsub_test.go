package shardcache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func TestExecutorStatePubSub_SubscribeUnsubscribe(t *testing.T) {
	defer goleak.VerifyNone(t)
	pubsub := newExecutorStatePubSub(testlogger.New(t), "test-ns")

	ch, unsub := pubsub.subscribe(context.Background())
	assert.NotNil(t, ch)
	assert.Len(t, pubsub.subscribers, 1)

	unsub()
	assert.Len(t, pubsub.subscribers, 0)

	// Unsubscribe is idempotent
	unsub()
	assert.Len(t, pubsub.subscribers, 0)
}

func TestExecutorStatePubSub_Publish(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("no subscribers doesn't panic", func(t *testing.T) {
		pubsub := newExecutorStatePubSub(testlogger.New(t), "test-ns")
		require.NotPanics(t, func() {
			pubsub.publish(map[*store.ShardOwner][]string{})
		})
	})

	t.Run("multiple subscribers receive updates", func(t *testing.T) {
		pubsub := newExecutorStatePubSub(testlogger.New(t), "test-ns")
		ch1, unsub1 := pubsub.subscribe(context.Background())
		ch2, unsub2 := pubsub.subscribe(context.Background())
		defer unsub1()
		defer unsub2()

		testState := map[*store.ShardOwner][]string{
			{ExecutorID: "exec-1", Metadata: map[string]string{}}: {"shard-1"},
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			state := <-ch1
			assert.Equal(t, testState, state)
			wg.Done()
		}()
		go func() {
			state := <-ch2
			assert.Equal(t, testState, state)
			wg.Done()
		}()
		time.Sleep(10 * time.Millisecond)

		pubsub.publish(testState)

		wg.Wait()
	})

	t.Run("non-blocking publish to slow consumer", func(t *testing.T) {
		pubsub := newExecutorStatePubSub(testlogger.New(t), "test-ns")

		// We create a subscriber that doesn't read from the channel, this should still not block
		_, slowUnsub := pubsub.subscribe(context.Background())
		defer slowUnsub()

		testState := map[*store.ShardOwner][]string{
			{ExecutorID: "exec-1", Metadata: map[string]string{}}: {"shard-1"},
		}

		// We do not read from the slow channel, this should still not block
		for range 10 {
			pubsub.publish(testState)
		}
	})
}
