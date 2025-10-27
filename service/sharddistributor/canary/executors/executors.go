package executors

import (
	"go.uber.org/fx"

	"github.com/uber/cadence/client/sharddistributor"
	exetrnalshardassignment "github.com/uber/cadence/service/sharddistributor/canary/externalshardassignment"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/executorclient"
)

type ExecutorResult struct {
	fx.Out
	Executor executorclient.Executor[*processor.ShardProcessor] `group:"executor-fixed-proc"`
}

type ExecutorEphemeralResult struct {
	fx.Out
	Executor executorclient.Executor[*processorephemeral.ShardProcessor] `group:"executor-ephemeral-proc"`
}

func NewExecutorWithFixedNamespace(params executorclient.Params[*processor.ShardProcessor]) (ExecutorResult, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, "shard-distributor-canary")
	return ExecutorResult{Executor: executor}, err
}

func NewExecutorWithEphemeralNamespace(params executorclient.Params[*processorephemeral.ShardProcessor]) (ExecutorEphemeralResult, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, "shard-distributor-canary-ephemeral")
	return ExecutorEphemeralResult{Executor: executor}, err
}

func NewExecutorLocalPassthroughNamespace(params executorclient.Params[*processor.ShardProcessor]) (ExecutorResult, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, "test-local-passthrough")
	return ExecutorResult{Executor: executor}, err
}
func NewExecutorLocalPassthroughShadowNamespace(params executorclient.Params[*processorephemeral.ShardProcessor]) (ExecutorEphemeralResult, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, "test-local-passthrough-shadow")
	return ExecutorEphemeralResult{Executor: executor}, err
}
func NewExecutorDistrebutedPassthroughNamespace(params executorclient.Params[*processor.ShardProcessor]) (ExecutorResult, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, "test-distributed-passthrough")
	return ExecutorResult{Executor: executor}, err
}

func NewExecutorExternalAssignmentNamespace(params executorclient.Params[*processorephemeral.ShardProcessor], shardDistributorClient sharddistributor.Client) (ExecutorEphemeralResult, *exetrnalshardassignment.ShardAssigner, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, "test-external-assignment")
	assigner := exetrnalshardassignment.NewShardAssigner(exetrnalshardassignment.ShardAssignerParams{
		Logger:           params.Logger,
		TimeSource:       params.TimeSource,
		ShardDistributor: shardDistributorClient,
		Executorclient:   executor,
	}, "test-external-assignment")

	return ExecutorEphemeralResult{Executor: executor}, assigner, err
}

type ExecutorsParams struct {
	fx.In
	Lc                 fx.Lifecycle
	ExecutorsFixed     []executorclient.Executor[*processor.ShardProcessor]          `group:"executor-fixed-proc"`
	Executorsephemeral []executorclient.Executor[*processorephemeral.ShardProcessor] `group:"executor-ephemeral-proc"`
}

func NewExecutorsModule(params ExecutorsParams) {
	for _, e := range params.ExecutorsFixed {
		params.Lc.Append(fx.StartStopHook(e.Start, e.Stop))
	}
	for _, e := range params.Executorsephemeral {
		params.Lc.Append(fx.StartStopHook(e.Start, e.Stop))
	}
}

var Module = fx.Module(
	"Executors",
	fx.Provide(NewExecutorWithFixedNamespace,
		NewExecutorWithEphemeralNamespace,
		NewExecutorLocalPassthroughNamespace,
		NewExecutorLocalPassthroughShadowNamespace,
		NewExecutorDistrebutedPassthroughNamespace,
	),
	fx.Module("Executor-with-external-assignment",
		fx.Provide(NewExecutorExternalAssignmentNamespace),
		fx.Invoke(func(lifecycle fx.Lifecycle, shardAssigner *exetrnalshardassignment.ShardAssigner) {
			lifecycle.Append(fx.StartStopHook(shardAssigner.Start, shardAssigner.Stop))
		}),
	),
	fx.Invoke(NewExecutorsModule),
)
