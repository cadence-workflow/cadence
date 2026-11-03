package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/service/frontend/config"
)

func TestStopDisconnectsPollersOnlyWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		res := resource.NewTest(t, gomock.NewController(t), metrics.Frontend)
		if enabled {
			res.MatchingPollerDisconnector.EXPECT().DisconnectPollers(gomock.Any()).Return(nil)
		}
		wh := &WorkflowHandler{
			Resource: res,
			config:   &config.Config{CancelOutstandingPollsOnShutdown: dynamicproperties.GetBoolPropertyFn(enabled)},
		}

		wh.Stop()

		assert.True(t, wh.isShuttingDown())
	}
}
