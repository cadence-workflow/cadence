package clock

import (
	"context"
	"time"

	smclock "github.com/cadence-workflow/shard-manager/common/clock"
)

// smTimeSourceAdapter adapts a cadence TimeSource to the shard-manager
// smclock.TimeSource interface.
//
// The two interfaces are method-for-method identical, but Go interface
// satisfaction is invariant on return types: NewTicker/NewTimer/AfterFunc
// return the repo-local Ticker/Timer interface types, so a cadence TimeSource
// does not satisfy smclock.TimeSource directly even though cadence's Ticker and
// Timer structurally implement smclock's. This shim only exists to launder
// those three return types across the package boundary.
//
// TODO: remove this once shard-manager's clock-consuming components depend on
// minimal interfaces that don't inject Ticker/Timer-returning methods, at which
// point a cadence TimeSource satisfies them natively.
type smTimeSourceAdapter struct {
	ts TimeSource
}

func NewSMTimeSourceAdapter(ts TimeSource) smclock.TimeSource {
	return &smTimeSourceAdapter{ts: ts}
}

func (a *smTimeSourceAdapter) After(d time.Duration) <-chan time.Time {
	return a.ts.After(d)
}

func (a *smTimeSourceAdapter) Sleep(d time.Duration) {
	a.ts.Sleep(d)
}

func (a *smTimeSourceAdapter) SleepWithContext(ctx context.Context, d time.Duration) error {
	return a.ts.SleepWithContext(ctx, d)
}

func (a *smTimeSourceAdapter) Now() time.Time {
	return a.ts.Now()
}

func (a *smTimeSourceAdapter) Since(t time.Time) time.Duration {
	return a.ts.Since(t)
}

func (a *smTimeSourceAdapter) NewTicker(d time.Duration) smclock.Ticker {
	return a.ts.NewTicker(d)
}

func (a *smTimeSourceAdapter) NewTimer(d time.Duration) smclock.Timer {
	return a.ts.NewTimer(d)
}

func (a *smTimeSourceAdapter) AfterFunc(d time.Duration, f func()) smclock.Timer {
	return a.ts.AfterFunc(d, f)
}

func (a *smTimeSourceAdapter) ContextWithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return a.ts.ContextWithTimeout(ctx, d)
}

func (a *smTimeSourceAdapter) ContextWithDeadline(ctx context.Context, t time.Time) (context.Context, context.CancelFunc) {
	return a.ts.ContextWithDeadline(ctx, t)
}
