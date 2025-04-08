package logfx

import (
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
)

// Module provides fx options to initialize logger from configuration.
var Module = fx.Options(
	fx.Provide(zapBuilder),
	ModuleWithoutZap,
)

// ModuleWithoutZap provides fx options to initialize logger from existing zap logger, which might be provided outside.
// This is useful for monorepos or any application that uses centralize log configuration like Uber monorepo.
var ModuleWithoutZap = fx.Options(
	fx.Provide(log.NewLogger))

type zapBuilderParams struct {
	fx.In

	Cfg config.Config
}

func zapBuilder(p zapBuilderParams) (*zap.Logger, error) {
	logger, err := p.Cfg.Log.NewZapLogger()
	if err != nil {
		return nil, err
	}
	return logger, nil
}
