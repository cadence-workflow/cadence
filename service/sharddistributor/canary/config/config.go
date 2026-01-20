package config

import "time"

// Config is the configuration for the shard distributor canary
type Config struct {
	Canary CanaryConfig `yaml:"canary"`
}

type CanaryConfig struct {
	// NumFixedExecutors is the number of executors of fixed namespace
	// Values more than 1 will create multiple executors processing the same fixed namespace
	// Default: 1
	NumFixedExecutors int `yaml:"numFixedExecutors"`

	// NumEphemeralExecutors is the number of executors of ephemeral namespace
	// Values more than 1 will create multiple executors processing the same ephemeral namespace
	// Default: 1
	NumEphemeralExecutors int `yaml:"numEphemeralExecutors"`

	// NumShardCreators is the number of shard creators creating new ephemeral shards.
	// ShardCreator uses Ping API of Canary and Spectator library to retrieve an owner of a new ephemeral shard
	// This call causes a creation of the new ephemeral shard in the shard distributor service.
	// Values more than 1 enable testing of concurrent shard creation.
	// Default: 1
	NumShardCreators int `yaml:"numShardCreators"`

	// ShardCreationInterval is the interval between creating new ephemeral shards by each shard creator.
	// Default: 1s
	ShardCreationInterval time.Duration `yaml:"shardCreationInterval"`
}
