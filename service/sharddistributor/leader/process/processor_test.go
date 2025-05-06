package process

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/config"
)

type testDeps struct {
	logger     log.Logger
	timeSource clock.MockedTimeSource
	factory    Factory
}

func TestProcessorFactory_CreateProcessor(t *testing.T) {
	// Arrange
	deps := setupTest(t)
	namespace := "test-namespace"

	// Act
	processor := deps.factory.CreateProcessor(namespace)

	// Assert
	assert.NotNil(t, processor, "Processor should not be nil")
	assert.IsType(t, &namespaceProcessor{}, processor, "Processor should be of type namespaceProcessor")

	// Check internal state through behavior
	// Start and stop to verify the namespace is set correctly
	err := processor.Start(context.Background())
	require.NoError(t, err)

	err = processor.Stop(context.Background())
	require.NoError(t, err)
}

func TestNamespaceProcessor_Start(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Arrange
	deps := setupTest(t)
	namespace := "test-namespace"
	processor := deps.factory.CreateProcessor(namespace)

	// Act
	err := processor.Start(context.Background())

	// Assert
	require.NoError(t, err, "Start should not return an error")

	// Test idempotency - starting again should not error
	err = processor.Start(context.Background())
	assert.NoError(t, err, "Starting an already running processor should not error")

	// Cleanup
	_ = processor.Stop(context.Background())
}

func TestNamespaceProcessor_Stop(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Arrange
	deps := setupTest(t)
	namespace := "test-namespace"
	processor := deps.factory.CreateProcessor(namespace)

	// Start the processor first
	err := processor.Start(context.Background())
	require.NoError(t, err)

	// Act
	err = processor.Stop(context.Background())

	// Assert
	require.NoError(t, err, "Stop should not return an error")

	// Test idempotency - stopping again should not error
	err = processor.Stop(context.Background())
	assert.NoError(t, err, "Stopping an already stopped processor should not error")
}

func TestNamespaceProcessor_StopWithoutStart(t *testing.T) {
	// Arrange
	deps := setupTest(t)
	namespace := "test-namespace"
	processor := deps.factory.CreateProcessor(namespace)

	// Act
	err := processor.Stop(context.Background())

	// Assert
	assert.NoError(t, err, "Stopping a processor that hasn't been started should not error")
}

func TestNamespaceProcessor_CancelledContext(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Arrange
	deps := setupTest(t)
	namespace := "test-namespace"
	processor := deps.factory.CreateProcessor(namespace)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start the processor
	err := processor.Start(ctx)
	require.NoError(t, err)

	// Act
	cancel() // Cancel the context

	// Give the goroutine time to react to the cancellation
	time.Sleep(10 * time.Millisecond)

	// Assert - we can't directly check if the process is running
	// but we can start it again to verify it's not running
	newCtx := context.Background()
	err = processor.Start(newCtx)
	require.NoError(t, err, "Should be able to start processor after context cancellation")

	// Cleanup
	_ = processor.Stop(newCtx)
}

func TestNamespaceProcessor_RunProcess(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Arrange
	deps := setupTest(t)
	namespace := "test-namespace"
	processor := deps.factory.CreateProcessor(namespace)

	// Act
	err := processor.Start(context.Background())
	require.NoError(t, err)

	// Advance the fake clock to trigger ticker
	deps.timeSource.Advance(6 * time.Second)

	// Give a little time for the ticker goroutine to run
	time.Sleep(10 * time.Millisecond)

	// There's no visible state change to assert from the ticker,
	// but we can verify the ticker advances without error

	// Cleanup
	err = processor.Stop(context.Background())
	require.NoError(t, err)
}

// setupTest provides common setup for all tests
func setupTest(t *testing.T) *testDeps {
	t.Helper()

	logger := testlogger.New(t)
	timeSource := clock.NewMockedTimeSource()
	factory := NewProcessorFactory(logger, timeSource, config.LeaderElection{Process: config.LeaderProcess{Period: 5 * time.Second}})

	return &testDeps{
		logger:     logger,
		timeSource: timeSource,
		factory:    factory,
	}
}
