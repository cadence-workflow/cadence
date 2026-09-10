package schema

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/config"
)

func TestWithDefaults(t *testing.T) {
	persistenceConfig := &config.Persistence{DefaultStore: "default"}

	tests := []struct {
		name    string
		opts    Options
		wantErr string
		verify  func(t *testing.T, opts Options)
	}{
		{
			name:    "missing cluster name",
			opts:    Options{Config: persistenceConfig},
			wantErr: "ClusterName is required",
		},
		{
			name:    "missing config",
			opts:    Options{ClusterName: "cluster"},
			wantErr: "Config is required",
		},
		{
			name: "fills in all defaults",
			opts: Options{ClusterName: "cluster", Config: persistenceConfig},
			verify: func(t *testing.T, opts Options) {
				assert.Equal(t, "cluster", opts.ClusterName)
				assert.Same(t, persistenceConfig, opts.Config)
				assert.Empty(t, opts.SetupOptions)
				assert.NotNil(t, opts.SetupOptions)
				assert.NotNil(t, opts.Logger)
				assert.Equal(t, defaultConnectTimeout, opts.ConnectTimeout)
				assert.NotNil(t, opts.Time)
			},
		},
		{
			name: "keeps provided values",
			opts: Options{
				ClusterName:    "cluster",
				Config:         persistenceConfig,
				SetupOptions:   map[string]string{"key": "value"},
				ConnectTimeout: 30 * time.Second,
			},
			verify: func(t *testing.T, opts Options) {
				assert.Equal(t, map[string]string{"key": "value"}, opts.SetupOptions)
				assert.Equal(t, 30*time.Second, opts.ConnectTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := withDefaults(tt.opts)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			tt.verify(t, got)
		})
	}
}

func TestOptionsFromConfig(t *testing.T) {
	cfg := config.Config{
		Persistence: config.Persistence{DefaultStore: "default"},
	}
	cfg.ClusterGroupMetadata = &config.ClusterGroupMetadata{CurrentClusterName: "primary"}

	opts := OptionsFromConfig(cfg)
	assert.Equal(t, "primary", opts.ClusterName)
	assert.Equal(t, "default", opts.Config.DefaultStore)
}
