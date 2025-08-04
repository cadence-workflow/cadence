//go:generate mockgen -package $GOPACKAGE -destination mitigator_mock.go github.com/uber/cadence/service/history/queuev2 Mitigator
package queuev2

import (
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
)

type (
	Mitigator interface {
		Mitigate(Alert)
	}

	mitigatorImpl struct {
		monitor      Monitor
		logger       log.Logger
		metricsScope metrics.Scope
	}
)

func NewMitigator(
	monitor Monitor,
	logger log.Logger,
	metricsScope metrics.Scope,
) Mitigator {
	return &mitigatorImpl{
		monitor:      monitor,
		logger:       logger,
		metricsScope: metricsScope,
	}
}

func (m *mitigatorImpl) Mitigate(alert Alert) {
	// TODO: implement the mitigation logic
}
