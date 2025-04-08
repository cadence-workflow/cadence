package cadence

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/logfx"
)

func TestFxDependencies(t *testing.T) {
	err := fx.ValidateApp(config.Module,
		logfx.Module,
		fx.Provide(func() appContext {
			return appContext{
				CfgContext: config.Context{
					Environment: "",
					Zone:        "",
				},
				ConfigDir: "",
				RootDir:   "",
				Services:  []string{"frontend"},
			}
		}),
		Module)
	require.NoError(t, err)
}

func TestFxStart(t *testing.T) {
	fxApp := fxtest.New(
		t,
		config.Module,
		logfx.Module,
		fx.Provide(func() appContext {
			return appContext{
				CfgContext: config.Context{
					Environment: "",
					Zone:        "",
				},
				ConfigDir: "../../../config",
				RootDir:   ".",
				Services:  []string{"frontend"},
			}
		}),
		Module)

	fxApp.RequireStart().RequireStop()
}
