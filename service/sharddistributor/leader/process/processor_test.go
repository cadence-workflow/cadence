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

func TestNamespaceProcessor_Terminate(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Arrange
	deps := setupTest(t)
	namespace := "test-namespace"
	processor := deps.factory.CreateProcessor(namespace)

	// Start the processor first
	err := processor.Run(context.Background())
	require.NoError(t, err)

	// Act
	err = processor.Terminate(context.Background())

	// Assert
	require.NoError(t, err, "Stop should not return an error")

	// Test double terminate should fail.
	err = processor.Terminate(context.Background())
	assert.Error(t, err, "processor has not been started")
}

func TestNamespaceProcessor_TerminateWithoutRun(t *testing.T) {
	// Arrange
	deps := setupTest(t)
	namespace := "test-namespace"
	processor := deps.factory.CreateProcessor(namespace)

	// Act
	err := processor.Terminate(context.Background())

	// Assert
	assert.Error(t, err, "processor has not been started")
}

func TestNamespaceProcessor_RunProcess(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Arrange
	deps := setupTest(t)
	namespace := "test-namespace"
	processor := deps.factory.CreateProcessor(namespace)

	// Act
	err := processor.Run(context.Background())
	require.NoError(t, err)

	// Advance the fake clock to trigger ticker
	deps.timeSource.Advance(6 * time.Second)

	// Give a little time for the ticker goroutine to run
	time.Sleep(10 * time.Millisecond)

	// There's no visible state change to assert from the ticker,
	// but we can verify the ticker advances without error

	// Cleanup
	err = processor.Terminate(context.Background())
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
