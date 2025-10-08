package metricsfx

import (
	"testing"

	"github.com/uber-go/tally"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

func TestModule(t *testing.T) {
	fxApp := fxtest.New(t,
		testlogger.Module(t),
		fx.Provide(fx.Annotated{
			Target: func() string { return service.Frontend },
			Name:   "service-full-name"},
			func() config.Service {
				return config.Service{}
			}),
		fx.Provide(func() metrics.HistogramMigration { return metrics.HistogramMigration{} }),
		Module,
		fx.Invoke(func(mc metrics.Client) {}))
	fxApp.RequireStart().RequireStop()
}

func TestModuleWithExternalScope(t *testing.T) {
	fxApp := fxtest.New(t,
		testlogger.Module(t),
		fx.Provide(func() tally.Scope { return tally.NoopScope },
			fx.Annotated{
				Target: func() string { return service.Frontend },
				Name:   "service-full-name"}),
		fx.Provide(func() metrics.HistogramMigration { return metrics.HistogramMigration{} }),
		ModuleForExternalScope,
		fx.Invoke(func(mc metrics.Client) {}))
	fxApp.RequireStart().RequireStop()
}
