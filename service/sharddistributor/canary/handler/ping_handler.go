package handler

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

// PingHandler handles ping requests to verify executor ownership of shards
type PingHandler struct {
	logger             *zap.Logger
	executorsFixed     map[string]executorclient.Executor[*processor.ShardProcessor]          // namespace -> executor
	executorsEphemeral map[string]executorclient.Executor[*processorephemeral.ShardProcessor] // namespace -> executor
}

// Params are the parameters for creating a PingHandler
type Params struct {
	fx.In

	Logger *zap.Logger

	ExecutorsFixed     []executorclient.Executor[*processor.ShardProcessor]          `group:"executor-fixed-proc"`
	ExecutorsEphemeral []executorclient.Executor[*processorephemeral.ShardProcessor] `group:"executor-ephemeral-proc"`
}

// NewPingHandler creates a new PingHandler
func NewPingHandler(params Params) *PingHandler {
	// Create maps of executors for quick lookup
	executorsFixed := make(map[string]executorclient.Executor[*processor.ShardProcessor])
	for _, executor := range params.ExecutorsFixed {
		executorsFixed[executor.GetNamespace()] = executor
	}
	executorsEphemeral := make(map[string]executorclient.Executor[*processorephemeral.ShardProcessor])
	for _, executor := range params.ExecutorsEphemeral {
		executorsEphemeral[executor.GetNamespace()] = executor
	}

	// Return the handler
	return &PingHandler{
		logger:             params.Logger,
		executorsFixed:     executorsFixed,
		executorsEphemeral: executorsEphemeral,
	}
}

// Ping handles ping requests to check shard ownership
func (h *PingHandler) Ping(ctx context.Context, request *sharddistributorv1.PingRequest) (*sharddistributorv1.PingResponse, error) {
	h.logger.Info("Received ping request",
		zap.String("shard_key", request.GetShardKey()),
		zap.String("namespace", request.GetNamespace()))

	namespace := request.GetNamespace()

	// Check fixed executors
	if executor, found := h.executorsFixed[namespace]; found {
		return checkOwnerShipAndLog(ctx, executor, request, h), nil
	}

	// Check ephemeral executors
	if executor, found := h.executorsEphemeral[namespace]; found {
		return checkOwnerShipAndLog(ctx, executor, request, h), nil
	}

	// Namespace not found
	h.logger.Warn("Namespace executor not found",
		zap.String("namespace", namespace))

	return &sharddistributorv1.PingResponse{
		ExecutorId: "",
		OwnsShard:  false,
		ShardKey:   request.GetShardKey(),
	}, nil
}

func checkOwnerShipAndLog[T executorclient.ShardProcessor](ctx context.Context, executor executorclient.Executor[T], request *sharddistributorv1.PingRequest, h *PingHandler) *sharddistributorv1.PingResponse {
	// We just check that we have a processor for the shard
	_, err := executor.GetShardProcess(ctx, request.GetShardKey())
	ownshard := err == nil

	metadata := executor.GetMetadata()
	executorID := getExecutorID(metadata)

	response := &sharddistributorv1.PingResponse{
		ExecutorId: executorID,
		OwnsShard:  ownshard,
		ShardKey:   request.GetShardKey(),
	}

	h.logger.Info("Responding to ping",
		zap.String("shard_key", request.GetShardKey()),
		zap.Bool("owns_shard", ownshard),
		zap.String("executor_id", executorID))

	return response
}

func getExecutorID(metadata map[string]string) string {
	if addr, ok := metadata["grpc_address"]; ok && addr != "" {
		return addr
	}
	return ""
}
