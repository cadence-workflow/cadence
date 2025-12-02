package metricsconstants

const (
	// Operation tag names for ShardDistributorSpectator metrics
	ShardDistributorSpectatorOperationTagName                    = "ShardDistributorSpectator"
	ShardDistributorSpectatorGetShardOwnerOperationTagName       = "ShardDistributorSpectatorGetShardOwner"
	ShardDistributorSpectatorWatchNamespaceStateOperationTagName = "ShardDistributorSpectatorWatchNamespaceState"

	// Counter metrics
	ShardDistributorSpectatorClientRequests = "shard_distributor_spectator_client_requests"
	ShardDistributorSpectatorClientFailures = "shard_distributor_spectator_client_failures"

	// Timer metrics
	ShardDistributorSpectatorClientLatency = "shard_distributor_spectator_client_latency"
)
