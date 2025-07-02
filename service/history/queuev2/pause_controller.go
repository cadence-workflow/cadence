//go:generate mockgen -package $GOPACKAGE -destination pause_controller_mock.go github.com/uber/cadence/service/history/queuev2 PauseController
package queuev2

import (
	"sync"
	"time"

	"github.com/uber/cadence/common/clock"
)

type (
	// PauseController is a controller that allows to pause and resume a background job.
	// For example, if you have a background job like this:
	// func run(ctx context.Context, wg *sync.WaitGroup, ...) {
	//   defer wg.Done()
	//   for {
	//     select {
	//     case <-ctx.Done():
	//       return
	//     case <-notifyCh:
	//       doSomething(...)
	//     }
	//   }
	// }
	// you can integrate the pause controller into the run function like this:
	// func run(ctx context.Context, wg *sync.WaitGroup, pauseController PauseController, ...) {
	//   defer wg.Done()
	//   pauseController.Subscribe("run", notifyCh)
	//   for {
	//     select {
	//     case <-ctx.Done():
	//       return
	//     case <-notifyCh:
	//       doSomething(pauseController, ...)
	//     }
	//   }
	// }
	//
	// func doSomething(pauseController PauseController, ...) {
	//   if someCondition {
	//     pauseController.Pause(10 * time.Second)
	//   }
	//   if pauseController.IsPaused() {
	//     return
	//   }
	//   // do the actual work
	// }
	PauseController interface {
		// Stop the pause controller but don't send notification to the subscribers. no-op if it's not paused.
		Stop()
		// Pause the job for the given duration. Zero and negative durations are ignored.
		// If it's already paused, the pause duration can only be updated to a longer duration.
		Pause(time.Duration)
		// Resume the job immediately. If it's not paused, this is a no-op.
		Resume()
		// Check if the job is paused.
		IsPaused() bool
		// Subscribe to the pause controller.
		Subscribe(string, chan<- struct{})
		// Unsubscribe from the pause controller.
		Unsubscribe(string)
	}

	pauseControllerImpl struct {
		sync.Mutex
		subscribers map[string]chan<- struct{}
		timeSource  clock.TimeSource
		pauseUntil  time.Time
		timer       clock.Timer
	}
)

func NewPauseController(timeSource clock.TimeSource) PauseController {
	return &pauseControllerImpl{
		timeSource:  timeSource,
		subscribers: make(map[string]chan<- struct{}),
	}
}

func (p *pauseControllerImpl) IsPaused() bool {
	p.Lock()
	defer p.Unlock()
	return p.timer != nil
}

func (p *pauseControllerImpl) Subscribe(id string, ch chan<- struct{}) {
	p.Lock()
	defer p.Unlock()
	p.subscribers[id] = ch
}

func (p *pauseControllerImpl) Unsubscribe(id string) {
	p.Lock()
	defer p.Unlock()
	delete(p.subscribers, id)
}

func (p *pauseControllerImpl) Stop() {
	p.Lock()
	defer p.Unlock()
	p.stopTimerLocked()
	p.timer = nil
	p.pauseUntil = time.Time{}
}

func (p *pauseControllerImpl) Pause(duration time.Duration) {
	if duration <= 0 {
		return
	}

	p.Lock()
	defer p.Unlock()

	newPauseUntil := p.timeSource.Now().Add(duration)
	if newPauseUntil.Before(p.pauseUntil) {
		return
	}

	p.stopTimerLocked()
	p.timer = p.timeSource.AfterFunc(duration, func() {
		p.Lock()
		defer p.Unlock()
		p.timer = nil
		p.pauseUntil = time.Time{}
		p.notifySubscribers()
	})
	p.pauseUntil = newPauseUntil
}

func (p *pauseControllerImpl) Resume() {
	p.Lock()
	defer p.Unlock()

	if p.timer == nil {
		return
	}
	p.stopTimerLocked()
	p.timer = nil
	p.pauseUntil = time.Time{}
	p.notifySubscribers()
}

func (p *pauseControllerImpl) stopTimerLocked() {
	if p.timer != nil {
		p.timer.Stop()
	}
}

func (p *pauseControllerImpl) notifySubscribers() {
	for _, ch := range p.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
