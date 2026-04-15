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

package host

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log"
)

func chaosTestCfg() ChaosConfig {
	return ChaosConfig{
		Enable:      true,
		IntervalMs:  1, // clamped to minSleepMs (100 ms) internally
		JitterMs:    0,
		MinHosts:    1,
		StopChance:  1.0,
		StartChance: 1.0,
	}
}

func mockIdentity(ctrl *gomock.Controller) *MockHostController {
	mc := NewMockHostController(ctrl)
	mc.EXPECT().HostIdentity(gomock.Any()).AnyTimes().DoAndReturn(
		func(i int) string { return fmt.Sprintf("host-%d", i) },
	)
	return mc
}

// waitOrFail blocks on ch until it is closed or the timeout elapses.
func waitOrFail(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(msg)
	}
}

// TestChaosMonkey_StopsHost verifies that the chaos monkey stops a running host
// when StopChance=1.0 and the cluster is above MinHosts.
func TestChaosMonkey_StopsHost(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mc := mockIdentity(ctrl)

	stopped := make(chan struct{})
	var once sync.Once
	var stopCount int64
	mc.EXPECT().StopHost(gomock.Any()).AnyTimes().Do(func(int) {
		atomic.AddInt64(&stopCount, 1)
		once.Do(func() { close(stopped) })
	})
	mc.EXPECT().StartHost(gomock.Any()).AnyTimes()

	cfg := chaosTestCfg()
	cm := NewChaosMonkey(mc, cfg, 3, log.NewNoop())

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); cm.Run(ctx) }()

	waitOrFail(t, stopped, 5*time.Second, "timeout: StopHost was never called")
	cancel()
	wg.Wait()

	if atomic.LoadInt64(&stopCount) < 1 {
		t.Errorf("expected StopHost >= 1 call, got %d", stopCount)
	}
}

// TestChaosMonkey_StartsHost verifies that the chaos monkey starts a stopped
// host when StartChance=1.0.
func TestChaosMonkey_StartsHost(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mc := mockIdentity(ctrl)

	started := make(chan struct{})
	var once sync.Once
	var startCount int64
	mc.EXPECT().StartHost(gomock.Any()).AnyTimes().Do(func(int) {
		atomic.AddInt64(&startCount, 1)
		once.Do(func() { close(started) })
	})
	mc.EXPECT().StopHost(gomock.Any()).Times(0) // canStop=false, MinHosts=3

	cfg := chaosTestCfg()
	cfg.MinHosts = 3 // all 3 hosts must stay running → canStop=false
	cfg.StartChance = 1.0

	cm := NewChaosMonkey(mc, cfg, 3, log.NewNoop())
	// Pre-stop host 0 by directly setting running state.
	cm.running[0] = false

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); cm.Run(ctx) }()

	waitOrFail(t, started, 5*time.Second, "timeout: StartHost was never called")
	cancel()
	wg.Wait()

	if atomic.LoadInt64(&startCount) < 1 {
		t.Errorf("expected StartHost >= 1 call, got %d", startCount)
	}
}

// TestChaosMonkey_StopsOrStartsNotBoth verifies that a single tick never both
// stops and starts a host — at most one action fires per tick.
func TestChaosMonkey_StopsOrStartsNotBoth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mc := mockIdentity(ctrl)

	var stopCount, startCount int64
	mc.EXPECT().StopHost(gomock.Any()).AnyTimes().Do(func(int) {
		atomic.AddInt64(&stopCount, 1)
	})
	mc.EXPECT().StartHost(gomock.Any()).AnyTimes().Do(func(int) {
		atomic.AddInt64(&startCount, 1)
	})

	cfg := chaosTestCfg() // StopChance=1.0, StartChance=1.0
	cm := NewChaosMonkey(mc, cfg, 3, log.NewNoop())
	cm.running[0] = false // one host already stopped so canStart=true from tick 1

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); cm.Run(ctx) }()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}

	stops := atomic.LoadInt64(&stopCount)
	starts := atomic.LoadInt64(&startCount)
	// With at-most-one-action-per-tick, total actions == number of ticks.
	// stops+starts can never exceed total ticks, and each tick does at most one.
	// We can't know the exact tick count but we can verify no tick did both:
	// if both fired simultaneously we'd see stops+starts > ticks. Since we
	// can't count ticks directly, we verify the sum is plausible (≤5 ticks
	// in 500ms at 100ms minimum interval).
	if stops+starts > 5 {
		t.Errorf("too many actions in 500ms: stops=%d starts=%d (expected ≤5 total ticks)", stops, starts)
	}
}

// TestChaosMonkey_ForcedRecoveryAtMinHosts verifies that when the cluster is at
// MinHosts (canStop=false), startAction fires unconditionally every tick
// regardless of StartChance. This ensures hosts are always recovered when the
// cluster cannot afford to stop any more.
func TestChaosMonkey_ForcedRecoveryAtMinHosts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mc := mockIdentity(ctrl)

	started := make(chan struct{})
	var once sync.Once
	mc.EXPECT().StartHost(gomock.Any()).AnyTimes().Do(func(int) {
		once.Do(func() { close(started) })
	})
	mc.EXPECT().StopHost(gomock.Any()).Times(0) // canStop=false

	cfg := chaosTestCfg()
	cfg.MinHosts = 3    // equals numHosts → canStop always false
	cfg.StopChance = 1.0
	cfg.StartChance = 0.01 // near-zero: would almost never fire without the forced-recovery fix

	cm := NewChaosMonkey(mc, cfg, 3, log.NewNoop())
	cm.running[0] = false // one host stopped → canStart=true

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); cm.Run(ctx) }()

	// With forced recovery, StartHost must fire within the first tick (≤200 ms).
	// With only StartChance=0.01, it would take ~100 ticks on average without the fix.
	waitOrFail(t, started, 500*time.Millisecond, "forced recovery: StartHost not called within first tick")
	cancel()
	wg.Wait()
}

// called when running hosts == MinHosts.
func TestChaosMonkey_NeverDropsBelowMinHosts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mc := NewMockHostController(ctrl)
	mc.EXPECT().StopHost(gomock.Any()).Times(0)
	mc.EXPECT().StartHost(gomock.Any()).Times(0)

	cfg := chaosTestCfg()
	cfg.MinHosts = 2    // equals numHosts → canStop always false
	cfg.StopChance = 1.0 // would stop if it could

	cm := NewChaosMonkey(mc, cfg, 2, log.NewNoop())

	// Two ticks at 100 ms each.
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); cm.Run(ctx) }()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

// TestChaosMonkey_RestartsAllOnCancel verifies that all stopped hosts are
// restarted before Run returns when the context is cancelled.
func TestChaosMonkey_RestartsAllOnCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mc := mockIdentity(ctrl)

	stopped := make(chan struct{})
	var stopOnce sync.Once
	var stopCount int64

	mc.EXPECT().StopHost(gomock.Any()).AnyTimes().Do(func(int) {
		atomic.AddInt64(&stopCount, 1)
		stopOnce.Do(func() { close(stopped) })
	})
	mc.EXPECT().StartHost(gomock.Any()).AnyTimes().Do(func(int) {
	})

	cfg := chaosTestCfg()
	cm := NewChaosMonkey(mc, cfg, 3, log.NewNoop())

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); cm.Run(ctx) }()

	waitOrFail(t, stopped, 5*time.Second, "timeout: StopHost was never called")
	cancel()
	wg.Wait()

	// After Run returns, all hosts must be running — restartAll cleaned up.
	for i, running := range cm.running {
		if !running {
			t.Errorf("host %d is still stopped after Run returned", i)
		}
	}
}

// TestChaosMonkey_EnableFalse verifies that Run is a no-op when Enable=false.
func TestChaosMonkey_EnableFalse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mc := NewMockHostController(ctrl)
	mc.EXPECT().StopHost(gomock.Any()).Times(0)
	mc.EXPECT().StartHost(gomock.Any()).Times(0)
	mc.EXPECT().HostIdentity(gomock.Any()).Times(0)

	cfg := ChaosConfig{Enable: false}
	cm := NewChaosMonkey(mc, cfg, 3, log.NewNoop())

	done := make(chan struct{})
	go func() { cm.Run(context.Background()); close(done) }()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return immediately when Enable=false")
	}
}
