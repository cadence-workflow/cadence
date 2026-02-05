package clusterredirection

import (
	"context"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func getRequestedConsistencyLevelFromContext(ctx context.Context) types.QueryConsistencyLevel {
	requestedConsistencyLevelFromCtx, ok := ctx.Value(common.QueryConsistencyLevelHeaderName).(types.QueryConsistencyLevel)
	if ok {
		return requestedConsistencyLevelFromCtx
	}
	return types.QueryConsistencyLevelEventual
}
