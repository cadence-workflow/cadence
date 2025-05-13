package fxutil

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// MockFxApp implements fxutil.App for testing
type MockFxApp struct {
	StartFunc func(context.Context) error
	StopFunc  func(context.Context) error
	DoneFunc  func() <-chan os.Signal
}

func (m *MockFxApp) Start(ctx context.Context) error {
	return m.StartFunc(ctx)
}

func (m *MockFxApp) Stop(ctx context.Context) error {
	return m.StopFunc(ctx)
}

func (m *MockFxApp) Done() <-chan os.Signal {
	return m.DoneFunc()
}

// TestRunServicesSuccess tests successful service execution
func TestRunServicesSuccess(t *testing.T) {
	// Create a test application using fxtest
	app := fxtest.New(t,
		fx.Provide(func() string { return "test-service" }),
		fx.Invoke(func(s string) {
			// Just a simple component that does nothing
		}),
	)

	// Create a done channel
	done := make(chan os.Signal, 1)

	// Wrap fxtest.App in our interface
	appInterface := &MockFxApp{
		StartFunc: app.Start,
		StopFunc:  app.Stop,
		DoneFunc:  func() <-chan os.Signal { return done },
	}

	// Run in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunServices([]string{"service1"}, func(name string) App {
			return appInterface
		})
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Send signal to stop
	close(done)

	// Check result
	err := <-errCh
	assert.NoError(t, err)
}

// TestRunServicesStartError tests failure during service start
func TestRunServicesStartError(t *testing.T) {
	// Create a mock app that fails on start
	startError := errors.New("failed to start")
	app := &MockFxApp{
		StartFunc: func(ctx context.Context) error {
			return startError
		},
		StopFunc: func(ctx context.Context) error {
			return nil
		},
		DoneFunc: func() <-chan os.Signal {
			ch := make(chan os.Signal)
			return ch
		},
	}

	// Run the services
	err := RunServices([]string{"service1"}, func(name string) App {
		return app
	})

	// Verify error was returned
	assert.Error(t, err)
	assert.True(t, errors.Is(err, startError), "Error chain should contain the original error")
}

// TestRunServicesStopError tests failure during service stop
func TestRunServicesStopError(t *testing.T) {
	// Create a mock app that fails on stop
	stopError := errors.New("failed to stop")
	done := make(chan os.Signal, 1)
	app := &MockFxApp{
		StartFunc: func(ctx context.Context) error {
			return nil
		},
		StopFunc: func(ctx context.Context) error {
			return stopError
		},
		DoneFunc: func() <-chan os.Signal {
			return done
		},
	}

	// Run in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunServices([]string{"service1"}, func(name string) App {
			return app
		})
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Signal that the service is done
	close(done)

	// Check the error
	err := <-errCh
	assert.Error(t, err)
	assert.True(t, errors.Is(err, stopError), "Error chain should contain the stop error")
}

// TestRunServicesCascadeFailure tests that when one service fails, others get stopped
func TestRunServicesCascadeFailure(t *testing.T) {
	// We'll use this to track context cancellation
	var contextCancelled bool
	var contextMu sync.Mutex

	// Create two apps - one will fail, one will succeed but should be stopped
	startErr := errors.New("service 2 failed to start")

	// Track app lifecycle events
	app1Started := false
	app1Stopped := false
	app2Started := false

	// First app - will start successfully
	done1 := make(chan os.Signal, 1)
	app1 := &MockFxApp{
		StartFunc: func(ctx context.Context) error {
			app1Started = true

			// Monitor for context cancellation in a goroutine
			go func() {
				<-ctx.Done()
				contextMu.Lock()
				contextCancelled = true
				contextMu.Unlock()
				// Close done channel to simulate app stopping due to context
				close(done1)
			}()

			return nil
		},
		StopFunc: func(ctx context.Context) error {
			app1Stopped = true
			return nil
		},
		DoneFunc: func() <-chan os.Signal {
			return done1
		},
	}

	// Second app - will fail to start
	app2 := &MockFxApp{
		StartFunc: func(ctx context.Context) error {
			app2Started = true
			return startErr
		},
		StopFunc: func(ctx context.Context) error {
			t.Fatal("App2 Stop should never be called since Start failed")
			return nil
		},
		DoneFunc: func() <-chan os.Signal {
			ch := make(chan os.Signal)
			return ch // Never signals
		},
	}

	// Build app provider that returns different apps for different services
	appProvider := func(name string) App {
		switch name {
		case "service1":
			return app1
		case "service2":
			return app2
		default:
			t.Fatalf("Unexpected service name: %s", name)
			return nil
		}
	}

	// Run services
	err := RunServices([]string{"service1", "service2"}, appProvider)

	// Verify results
	require.Error(t, err, "Should return an error")
	assert.True(t, app1Started, "App1 should have started")
	assert.True(t, app2Started, "App2 should have started")
	assert.True(t, app1Stopped, "App1 should have been stopped due to context cancellation")

	// Check if context was cancelled
	contextMu.Lock()
	assert.True(t, contextCancelled, "Context should have been cancelled")
	contextMu.Unlock()

	// Check error content
	assert.Contains(t, err.Error(), "service 2")
}
