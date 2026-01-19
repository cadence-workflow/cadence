package config

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
}
