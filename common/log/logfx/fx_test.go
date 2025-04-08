package logfx

import (
	"testing"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
)

func TestLogFx(t *testing.T) {
	app := fxtest.New(t,
		fx.Provide(func() config.Config {
			return config.Config{}
		}),
		Module,
		fx.Invoke(func(logger log.Logger) {
			logger.Info("hello world")
		}),
	)
	app.RequireStart().RequireStop()
}

func TestLogFxWithExternalZapLogger(t *testing.T) {
	app := fxtest.New(t,
		fx.Provide(func() *zap.Logger {
			return zaptest.NewLogger(t)
		}),
		ModuleWithoutZap,
		fx.Invoke(func(logger log.Logger) {
			logger.Info("hello world")
		}))
	app.RequireStart().RequireStop()
}
