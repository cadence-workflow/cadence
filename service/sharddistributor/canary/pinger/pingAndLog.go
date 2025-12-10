package pinger

import (
	"context"
	"time"

	"go.uber.org/yarpc"
	"go.uber.org/zap"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
)

const (
	pingTimeout = 5 * time.Second
)

func PingShard(ctx context.Context, canaryClient sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCClient, logger *zap.Logger, namespace, shardKey string) {
	request := &sharddistributorv1.PingRequest{
		ShardKey:  shardKey,
		Namespace: namespace,
	}

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	response, err := canaryClient.Ping(ctx, request, yarpc.WithShardKey(shardKey), yarpc.WithHeader(spectatorclient.NamespaceHeader, namespace))
	if err != nil {
		logger.Error("Failed to ping shard", zap.String("namespace", namespace), zap.String("shard_key", shardKey), zap.Error(err))
		return
	}

	// Verify response
	if !response.GetOwnsShard() {
		logger.Warn("Executor does not own shard", zap.String("namespace", namespace), zap.String("shard_key", shardKey), zap.String("executor_id", response.GetExecutorId()))
		return
	}

	logger.Info("Successfully pinged shard owner", zap.String("namespace", namespace), zap.String("shard_key", shardKey), zap.String("executor_id", response.GetExecutorId()))
}
