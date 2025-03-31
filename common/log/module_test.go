package log

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/config"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

type testCfg struct {
	LogCfg Config `yaml:"log"`
}

func TestModule(t *testing.T) {
	app := fxtest.New(t,
		fx.Provide(func() config.Provider {
			cfgProvider, err := config.NewYAML(config.Static(testCfg{}))
			require.NoError(t, err)
			return cfgProvider
		}),
		Module)
	app.RequireStart()
	app.RequireStart()
}
