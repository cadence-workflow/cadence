package spectators

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/canary/config"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
)

func TestNewSpectatorsWithNamespace(t *testing.T) {
	tests := []struct {
		name          string
		numSpectators int
		expectedCount int
	}{
		{
			name:          "positive number of spectators",
			numSpectators: 3,
			expectedCount: 3,
		},
		{
			name:          "zero spectators defaults to 1",
			numSpectators: 0,
			expectedCount: 1,
		},
		{
			name:          "negative spectators defaults to 1",
			numSpectators: -5,
			expectedCount: 1,
		},
		{
			name:          "one spectator",
			numSpectators: 1,
			expectedCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			namespace := "test-namespace"
			params := createTestSpectatorParams(t, namespace)

			spectators, err := NewSpectatorsWithNamespace(params, namespace, tc.numSpectators)

			require.NoError(t, err)
			assert.Len(t, spectators, tc.expectedCount)
		})
	}
}

func TestNewSpectators(t *testing.T) {
	t.Run("creates fixed and ephemeral spectators", func(t *testing.T) {
		fixedNamespace := "fixed-ns"
		ephemeralNamespace := "ephemeral-ns"

		cfg := config.Config{
			Canary: config.CanaryConfig{
				NumFixedSpectators:     2,
				NumEphemeralSpectators: 3,
			},
		}

		spectatorParams := createTestSpectatorParams(t, fixedNamespace, ephemeralNamespace)

		p := Params{
			Config:          cfg,
			SpectatorParams: spectatorParams,
		}

		spectators, err := NewSpectators(p, fixedNamespace, ephemeralNamespace)

		require.NoError(t, err)
		assert.Len(t, spectators, 5) // 2 fixed + 3 ephemeral
	})

	t.Run("uses default of 1 when spectator counts are zero", func(t *testing.T) {
		fixedNamespace := "fixed-ns"
		ephemeralNamespace := "ephemeral-ns"

		cfg := config.Config{
			Canary: config.CanaryConfig{
				NumFixedSpectators:     0,
				NumEphemeralSpectators: 0,
			},
		}

		spectatorParams := createTestSpectatorParams(t, fixedNamespace, ephemeralNamespace)

		p := Params{
			Config:          cfg,
			SpectatorParams: spectatorParams,
		}

		spectators, err := NewSpectators(p, fixedNamespace, ephemeralNamespace)

		require.NoError(t, err)
		assert.Len(t, spectators, 2) // 1 fixed + 1 ephemeral (defaults)
	})
}

func TestNewSpectators_ErrorCases(t *testing.T) {
	t.Run("returns error when fixed namespace config is missing", func(t *testing.T) {
		fixedNamespace := "missing-fixed-ns"
		ephemeralNamespace := "ephemeral-ns"

		cfg := config.Config{
			Canary: config.CanaryConfig{
				NumFixedSpectators:     1,
				NumEphemeralSpectators: 1,
			},
		}

		// Only configure ephemeral namespace, not fixed
		spectatorParams := createTestSpectatorParams(t, ephemeralNamespace)

		p := Params{
			Config:          cfg,
			SpectatorParams: spectatorParams,
		}

		spectators, err := NewSpectators(p, fixedNamespace, ephemeralNamespace)

		assert.Error(t, err)
		assert.Nil(t, spectators)
	})

	t.Run("returns error when ephemeral namespace config is missing", func(t *testing.T) {
		fixedNamespace := "fixed-ns"
		ephemeralNamespace := "missing-ephemeral-ns"

		cfg := config.Config{
			Canary: config.CanaryConfig{
				NumFixedSpectators:     1,
				NumEphemeralSpectators: 1,
			},
		}

		// Only configure fixed namespace, not ephemeral
		spectatorParams := createTestSpectatorParams(t, fixedNamespace)

		p := Params{
			Config:          cfg,
			SpectatorParams: spectatorParams,
		}

		spectators, err := NewSpectators(p, fixedNamespace, ephemeralNamespace)

		assert.Error(t, err)
		assert.Nil(t, spectators)
	})
}

func TestModule(t *testing.T) {
	t.Run("module returns valid fx.Option", func(t *testing.T) {
		fixedNamespace := "fixed-ns"
		ephemeralNamespace := "ephemeral-ns"

		option := Module(fixedNamespace, ephemeralNamespace)

		// Verify the module returns a valid fx.Option
		assert.NotNil(t, option)
	})
}

func TestNewSpectatorsWithNamespace_MultipleSpectators(t *testing.T) {
	t.Run("creates requested number of spectators", func(t *testing.T) {
		namespace := "test-ns"
		numSpectators := 5
		params := createTestSpectatorParams(t, namespace)

		spectators, err := NewSpectatorsWithNamespace(params, namespace, numSpectators)

		require.NoError(t, err)
		assert.Len(t, spectators, numSpectators)

		// Verify each spectator is non-nil
		for i, sp := range spectators {
			assert.NotNil(t, sp, "spectator at index %d should not be nil", i)
		}
	})
}

// createTestSpectatorParams creates a spectatorclient.Params for testing with the given namespaces
func createTestSpectatorParams(t *testing.T, namespaces ...string) spectatorclient.Params {
	t.Helper()

	var namespaceConfigs []clientcommon.NamespaceConfig
	for _, ns := range namespaces {
		namespaceConfigs = append(namespaceConfigs, clientcommon.NamespaceConfig{
			Namespace:         ns,
			HeartBeatInterval: time.Second,
			TTLShard:          time.Minute,
			TTLReport:         time.Second * 30,
		})
	}

	return spectatorclient.Params{
		Client:       sharddistributor.NewMockClient(nil),
		MetricsScope: tally.NoopScope,
		Logger:       testlogger.New(t),
		Config: clientcommon.Config{
			Namespaces: namespaceConfigs,
		},
		TimeSource: clock.NewRealTimeSource(),
	}
}

// TestNewSpectatorsWithNamespace_ErrorPropagation tests that errors from NewSpectatorWithNamespace are propagated
func TestNewSpectatorsWithNamespace_ErrorPropagation(t *testing.T) {
	t.Run("propagates error when spectator creation fails", func(t *testing.T) {
		namespace := "unconfigured-namespace"
		// Create params without the namespace configured to trigger an error
		params := spectatorclient.Params{
			Client:       sharddistributor.NewMockClient(nil),
			MetricsScope: tally.NoopScope,
			Logger:       testlogger.New(t),
			Config: clientcommon.Config{
				Namespaces: []clientcommon.NamespaceConfig{}, // Empty - no namespaces configured
			},
			TimeSource: clock.NewRealTimeSource(),
		}

		spectators, err := NewSpectatorsWithNamespace(params, namespace, 1)

		assert.Error(t, err)
		assert.Nil(t, spectators)
	})
}
