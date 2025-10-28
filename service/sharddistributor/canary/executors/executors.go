package executors

import (
	"go.uber.org/fx"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/service/sharddistributor/canary/externalshardassignment"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/executorclient"
)

const (
	// LocalPassthroughNamespace namespace used to test the local passthrough
	LocalPassthroughNamespace = "test-local-passthrough"
	// LocalPassthroughShadowNamespace namespace used to test the local passthrough shadow
	LocalPassthroughShadowNamespace = "test-local-passthrough-shadow"
	// DistributedPassthroughNamespace namespace used to test the distributed passthrough
	DistributedPassthroughNamespace = "test-distributed-passthrough"
	// ExternalAssignmentNamespace namespace used to test the external assignments
	ExternalAssignmentNamespace = "test-external-assignment"
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
	executor, err := executorclient.NewExecutorWithNamespace(params, LocalPassthroughNamespace)
	return ExecutorResult{Executor: executor}, err
}
func NewExecutorLocalPassthroughShadowNamespace(params executorclient.Params[*processorephemeral.ShardProcessor]) (ExecutorEphemeralResult, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, LocalPassthroughShadowNamespace)
	return ExecutorEphemeralResult{Executor: executor}, err
}
func NewExecutorDistributedPassthroughNamespace(params executorclient.Params[*processor.ShardProcessor]) (ExecutorResult, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, DistributedPassthroughNamespace)
	return ExecutorResult{Executor: executor}, err
}

func NewExecutorExternalAssignmentNamespace(params executorclient.Params[*processorephemeral.ShardProcessor], shardDistributorClient sharddistributor.Client) (ExecutorEphemeralResult, *externalshardassignment.ShardAssigner, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, ExternalAssignmentNamespace)
	assigner := externalshardassignment.NewShardAssigner(externalshardassignment.ShardAssignerParams{
		Logger:           params.Logger,
		TimeSource:       params.TimeSource,
		ShardDistributor: shardDistributorClient,
		Executorclient:   executor,
	}, ExternalAssignmentNamespace)

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
		NewExecutorDistributedPassthroughNamespace,
	),
	fx.Module("Executor-with-external-assignment",
		fx.Provide(NewExecutorExternalAssignmentNamespace),
		fx.Invoke(func(lifecycle fx.Lifecycle, shardAssigner *externalshardassignment.ShardAssigner) {
			lifecycle.Append(fx.StartStopHook(shardAssigner.Start, shardAssigner.Stop))
		}),
	),
	fx.Invoke(NewExecutorsModule),
)
