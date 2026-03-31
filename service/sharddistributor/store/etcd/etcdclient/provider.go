package etcdclient

import (
	"fmt"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx"
)

// NewClientFromConfig creates a new ETCD client from configuration.
func NewClientFromConfig(cfg BaseConfig, lc fx.Lifecycle) (Client, error) {
	rawClient, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create etcd client: %w", err)
	}
	lc.Append(fx.StopHook(rawClient.Close))

	return NewClient(rawClient), nil
}
