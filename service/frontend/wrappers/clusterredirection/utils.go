package clusterredirection

import (
	"context"

	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/types"
	"go.uber.org/yarpc"
)

func getRequestedConsistencyLevelFromContext(ctx context.Context) types.QueryConsistencyLevel {
	call := yarpc.CallFromContext(ctx)
	if call == nil {
		return types.QueryConsistencyLevelEventual
	}
	featureFlags := client.GetFeatureFlagsFromHeader(call)
	if featureFlags.AutoforwardingEnabled {
		return types.QueryConsistencyLevelStrong
	}
	return types.QueryConsistencyLevelEventual
}
