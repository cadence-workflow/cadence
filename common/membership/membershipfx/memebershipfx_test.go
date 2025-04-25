package membershipfx

import (
	"testing"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
)

func TestFxStartStop(t *testing.T) {
	app := fxtest.New(t, fx.Provide(func() appParams {
		ctrl := gomock.NewController(t)
		provider := membership.NewMockPeerProvider(ctrl)
		return appParams{
			Clock:         clock.NewMockedTimeSource(),
			PeerProvider:  provider,
			Logger:        testlogger.New(t),
			MetricsClient: metrics.NewNoopMetricsClient(),
		}
	}), Module)
	app.RequireStart().RequireStop()
}

type appParams struct {
	fx.Out

	Clock         clock.TimeSource
	PeerProvider  membership.PeerProvider
	Logger        log.Logger
	MetricsClient metrics.Client
}
