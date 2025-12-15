package executors

import (
	"go.uber.org/fx"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/service/sharddistributor/canary/externalshardassignment"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
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

func NewExecutorWithFixedNamespace(params executorclient.Params[*processor.ShardProcessor], namespace string) (ExecutorResult, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, namespace)
	return ExecutorResult{Executor: executor}, err
}

func NewExecutorWithEphemeralNamespace(params executorclient.Params[*processorephemeral.ShardProcessor], namespace string) (ExecutorEphemeralResult, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, namespace)
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

func NewExecutorExternalAssignmentNamespace(params executorclient.Params[*processorephemeral.ShardProcessor], shardDistributorClient sharddistributor.Client, namespace string) (ExecutorEphemeralResult, *externalshardassignment.ShardAssigner, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, namespace)
	assigner := externalshardassignment.NewShardAssigner(externalshardassignment.ShardAssignerParams{
		Logger:           params.Logger,
		TimeSource:       params.TimeSource,
		ShardDistributor: shardDistributorClient,
		Executorclient:   executor,
	}, namespace)

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

func Module(fixedNamespace, ephemeralNamespace, externalAssignmentNamespace string) fx.Option {
	return fx.Module(
		"Executors",
		// Executor that is used for testing a namespace with fixed shards
		fx.Provide(
			func(params executorclient.Params[*processor.ShardProcessor]) (ExecutorResult, error) {
				return NewExecutorWithFixedNamespace(params, fixedNamespace)
			}),
		// Executor that is used for testing a namespaces with ephemeral shards
		fx.Provide(func(params executorclient.Params[*processorephemeral.ShardProcessor]) (ExecutorEphemeralResult, error) {
			return NewExecutorWithEphemeralNamespace(params, ephemeralNamespace)
		}),
		// Executor used for testing a namespace where the shards are assigned externally and reflected in the state of the SD
		// this is reproducing the behaviour that matching service is going to have during the DistributedPassthrough mode
		fx.Module("Executor-with-external-assignment",
			fx.Provide(func(params executorclient.Params[*processorephemeral.ShardProcessor], shardDistributorClient sharddistributor.Client) (ExecutorEphemeralResult, *externalshardassignment.ShardAssigner, error) {
				return NewExecutorExternalAssignmentNamespace(params, shardDistributorClient, externalAssignmentNamespace)
			}),
			fx.Invoke(func(lifecycle fx.Lifecycle, shardAssigner *externalshardassignment.ShardAssigner) {
				lifecycle.Append(fx.StartStopHook(shardAssigner.Start, shardAssigner.Stop))
			}),
		),
		fx.Invoke(NewExecutorsModule),
	)
}

// ReplayFixedModule starts N fixed-namespace executors for a single namespace.
// It is used by the CSV replay canary to simulate multiple executors in a single process.
func ReplayFixedModule(namespace string, fixedExecutorCount int) fx.Option {
	if fixedExecutorCount <= 0 {
		fixedExecutorCount = 1
	}

	var options []fx.Option
	for i := 0; i < fixedExecutorCount; i++ {
		options = append(options, fx.Provide(func(params executorclient.Params[*processor.ShardProcessor]) (ExecutorResult, error) {
			return NewExecutorWithFixedNamespace(params, namespace)
		}))
	}
	options = append(options, fx.Invoke(NewExecutorsModule))

	return fx.Module("ReplayFixedExecutors", fx.Options(options...))
}
