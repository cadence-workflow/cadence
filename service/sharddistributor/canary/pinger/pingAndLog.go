package pinger

import (
	"context"
	"time"

	"github.com/uber-go/tally"
	"go.uber.org/yarpc"
	"go.uber.org/zap"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	canarymetrics "github.com/uber/cadence/service/sharddistributor/canary/metrics"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
)

const (
	pingTimeout = 5 * time.Second
)

func PingShard(ctx context.Context, canaryClient sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCClient, metricsScope tally.Scope, logger *zap.Logger, namespace, shardKey string) {
	request := &sharddistributorv1.PingRequest{
		ShardKey:  shardKey,
		Namespace: namespace,
	}

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	start := time.Now()
	response, err := canaryClient.Ping(ctx, request, yarpc.WithShardKey(shardKey), yarpc.WithHeader(spectatorclient.NamespaceHeader, namespace))
	metricsScope.Histogram(canarymetrics.CanaryPingLatency, canarymetrics.CanaryPingLatencyBuckets).RecordDuration(time.Since(start))

	if err != nil {
		metricsScope.Counter(canarymetrics.CanaryPingFailure).Inc(1)
		logger.Error("Failed to ping shard", zap.String("namespace", namespace), zap.String("shard_key", shardKey), zap.Error(err))
		return
	}

	// Verify response
	if !response.GetOwnsShard() {
		metricsScope.Counter(canarymetrics.CanaryPingOwnershipMismatch).Inc(1)
		logger.Warn("Executor does not own shard", zap.String("namespace", namespace), zap.String("shard_key", shardKey), zap.String("executor_id", response.GetExecutorId()))
		return
	}
	metricsScope.Counter(canarymetrics.CanaryPingSuccess).Inc(1)
	logger.Debug("Successfully pinged shard owner", zap.String("namespace", namespace), zap.String("shard_key", shardKey), zap.String("executor_id", response.GetExecutorId()))
}
