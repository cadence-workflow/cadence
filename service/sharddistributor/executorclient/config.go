package executorclient

import "time"

type Config struct {
	Namespace         string        `yaml:"namespace"`
	HeartBeatInterval time.Duration `yaml:"heartbeat_interval"`
}
