package queuev2

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"github.com/uber/cadence/common/clock"
)

func TestPauseController_Basic(t *testing.T) {
	defer goleak.VerifyNone(t)
	timeSource := clock.NewMockedTimeSource()
	controller := NewPauseController(timeSource)
	ch := make(chan struct{}, 100)
	controller.Subscribe("test", ch)
	// Initially not paused
	assert.False(t, controller.IsPaused())

	// pause for 100ms
	controller.Pause(100 * time.Millisecond)
	assert.True(t, controller.IsPaused())

	// advance 50ms, should still be paused
	timeSource.BlockUntil(1)
	timeSource.Advance(50 * time.Millisecond)
	assert.True(t, controller.IsPaused())

	// pause for 200ms and this should reset the pause duration
	controller.Pause(200 * time.Millisecond)
	assert.True(t, controller.IsPaused())

	// Advance time to trigger timer
	timeSource.BlockUntil(1)
	timeSource.Advance(200 * time.Millisecond)
	// wait for the timer to expire
	<-ch
	assert.False(t, controller.IsPaused())

	// Resume when not paused should be no-op
	controller.Resume()
	assert.False(t, controller.IsPaused())
	select {
	case <-ch:
		t.Fatal("resume when not paused should be no-op")
	default:
		// Expected
	}

	// Pause then resume
	controller.Pause(100 * time.Millisecond)
	assert.True(t, controller.IsPaused())

	controller.Resume()
	assert.False(t, controller.IsPaused())
	select {
	case <-ch:
	default:
		t.Fatal("channel should receive notification after resume")
	}

	// Resume again should be no-op
	controller.Resume()
	assert.False(t, controller.IsPaused())
	select {
	case <-ch:
		t.Fatal("resume when not paused should be no-op")
	default:
		// Expected
	}

	// Stop when not paused
	controller.Stop()
	assert.False(t, controller.IsPaused())
	select {
	case <-ch:
		t.Fatal("channel should not receive notification after stop")
	default:
		// Expected
	}

	// Pause then stop
	controller.Pause(100 * time.Millisecond)
	assert.True(t, controller.IsPaused())
	controller.Stop()
	assert.False(t, controller.IsPaused())

	select {
	case <-ch:
		t.Fatal("channel should not receive notification after stop")
	default:
		// Expected
	}

	controller.Pause(100 * time.Millisecond)
	assert.True(t, controller.IsPaused())

	// Pause duration is less than the previous pause duration, should not reset the pause duration
	controller.Pause(10 * time.Millisecond)
	timeSource.BlockUntil(1)
	timeSource.Advance(20 * time.Millisecond)
	assert.True(t, controller.IsPaused())
	select {
	case <-ch:
		t.Fatal("channel should not receive notification")
	default:
		// Expected
	}
	timeSource.BlockUntil(1)
	timeSource.Advance(100 * time.Millisecond)
	// wait for the timer to expire
	<-ch
	assert.False(t, controller.IsPaused())
}

func TestPauseController_Subscribe_Unsubscribe(t *testing.T) {
	timeSource := clock.NewMockedTimeSource()
	controller := NewPauseController(timeSource)

	// Subscribe a channel
	ch1 := make(chan struct{}, 1)
	controller.Subscribe("sub1", ch1)

	// Subscribe another channel
	ch2 := make(chan struct{}, 1)
	controller.Subscribe("sub2", ch2)

	// Unsubscribe one
	controller.Unsubscribe("sub1")

	// Pause and resume to trigger notifications
	controller.Pause(100 * time.Millisecond)
	controller.Resume()

	// Only ch2 should receive notification
	select {
	case <-ch1:
		t.Fatal("ch1 should not receive notification after unsubscribe")
	default:
		// Expected
	}

	select {
	case <-ch2:
		// Expected
	default:
		t.Fatal("ch2 should receive notification")
	}
}

func TestPauseController_NotifySubscribers(t *testing.T) {
	timeSource := clock.NewMockedTimeSource()
	controller := NewPauseController(timeSource)

	// Subscribe multiple channels
	ch1 := make(chan struct{}, 1)
	ch2 := make(chan struct{}, 1)
	ch3 := make(chan struct{}, 1)

	controller.Subscribe("sub1", ch1)
	controller.Subscribe("sub2", ch2)
	controller.Subscribe("sub3", ch3)

	// Pause and resume to trigger notifications
	controller.Pause(100 * time.Millisecond)
	controller.Resume()

	// All channels should receive notifications
	select {
	case <-ch1:
		// Expected
	default:
		t.Fatal("ch1 should receive notification")
	}

	select {
	case <-ch2:
		// Expected
	default:
		t.Fatal("ch2 should receive notification")
	}

	select {
	case <-ch3:
		// Expected
	default:
		t.Fatal("ch3 should receive notification")
	}
}

func TestPauseController_SubscriberChannelBlocking(t *testing.T) {
	timeSource := clock.NewMockedTimeSource()
	controller := NewPauseController(timeSource)

	// Subscribe a channel with no buffer (will block)
	ch := make(chan struct{})
	controller.Subscribe("sub1", ch)

	// Pause and resume to trigger notification
	controller.Pause(100 * time.Millisecond)
	controller.Resume()

	// The notification should be sent non-blocking
	// If the channel is full or blocked, it should not cause deadlock
	// This test verifies that the select with default case works correctly
	assert.False(t, controller.IsPaused())
}

func TestPauseController_ZeroDurationPause(t *testing.T) {
	timeSource := clock.NewMockedTimeSource()
	controller := NewPauseController(timeSource)

	// Subscribe a channel
	ch := make(chan struct{}, 1)
	controller.Subscribe("sub1", ch)

	// Pause with zero duration
	controller.Pause(0)
	assert.False(t, controller.IsPaused())

	// Channel should not receive notification
	select {
	case <-ch:
		t.Fatal("channel should not receive notification for zero duration pause")
	default:
		// Expected
	}
}

func TestPauseController_NegativeDurationPause(t *testing.T) {
	timeSource := clock.NewMockedTimeSource()
	controller := NewPauseController(timeSource)

	// Subscribe a channel
	ch := make(chan struct{}, 1)
	controller.Subscribe("sub1", ch)

	// Pause with negative duration
	controller.Pause(-100 * time.Millisecond)
	assert.False(t, controller.IsPaused())

	// Channel should not receive notification
	select {
	case <-ch:
		t.Fatal("channel should not receive notification for negative duration pause")
	default:
		// Expected
	}
}

func TestPauseController_ConcurrentAccess(t *testing.T) {
	defer goleak.VerifyNone(t)
	timeSource := clock.NewMockedTimeSource()
	controller := NewPauseController(timeSource)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Test concurrent pause/resume operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ch := make(chan struct{}, 1)
			controller.Subscribe("sub"+string(rune(id)), ch)
			controller.Pause(time.Duration(id+1) * time.Millisecond)
			controller.Resume()
			controller.IsPaused()
			controller.Unsubscribe("sub" + string(rune(id)))
		}(i)
	}

	wg.Wait()
}
