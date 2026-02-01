package executors

import (
	"go.uber.org/fx"

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/service/sharddistributor/canary/config"
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

type ExecutorsResult struct {
	fx.Out
	Executors []executorclient.Executor[*processor.ShardProcessor] `group:"executor-fixed-proc,flatten"`
}

type ExecutorsEphemeralResult struct {
	fx.Out
	Executors []executorclient.Executor[*processorephemeral.ShardProcessor] `group:"executor-ephemeral-proc,flatten"`
}

func NewExecutorsWithFixedNamespace(params executorclient.Params[*processor.ShardProcessor], namespace string, numExecutors int) (ExecutorsResult, error) {
	var result ExecutorsResult

	if numExecutors <= 0 {
		numExecutors = 1
	}

	for i := 0; i < numExecutors; i++ {
		executor, err := executorclient.NewExecutorWithNamespace(params, namespace)
		if err != nil {
			return ExecutorsResult{}, err
		}
		result.Executors = append(result.Executors, executor)
	}

	return result, nil
}

func NewExecutorWithFixedNamespace(params executorclient.Params[*processor.ShardProcessor], namespace string) (ExecutorResult, error) {
	executor, err := executorclient.NewExecutorWithNamespace(params, namespace)
	return ExecutorResult{Executor: executor}, err
}

func NewExecutorsWithEphemeralNamespace(params executorclient.Params[*processorephemeral.ShardProcessor], namespace string, numExecutors int) (ExecutorsEphemeralResult, error) {
	var result ExecutorsEphemeralResult

	if numExecutors <= 0 {
		numExecutors = 1
	}

	for i := 0; i < numExecutors; i++ {
		executor, err := executorclient.NewExecutorWithNamespace(params, namespace)
		if err != nil {
			return ExecutorsEphemeralResult{}, err
		}
		result.Executors = append(result.Executors, executor)
	}

	return result, nil
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
	return fx.Module("Executors",
		// Executor that is used for testing a namespace with fixed shards
		fx.Provide(func(cfg config.Config, params executorclient.Params[*processor.ShardProcessor]) (ExecutorsResult, error) {
			return NewExecutorsWithFixedNamespace(params, fixedNamespace, cfg.Canary.NumFixedExecutors)
		}),

		// Executor that is used for testing a namespaces with ephemeral shards
		fx.Provide(func(cfg config.Config, params executorclient.Params[*processorephemeral.ShardProcessor]) (ExecutorsEphemeralResult, error) {
			return NewExecutorsWithEphemeralNamespace(params, ephemeralNamespace, cfg.Canary.NumEphemeralExecutors)
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
