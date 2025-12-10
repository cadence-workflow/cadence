package executorstore

import (
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx"

	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdclient"
)

type ClientParams struct {
	fx.In

	// If provided, this client will be used instead of creating a new one
	Client    *clientv3.Client `optional:"true"`
	Cfg       ETCDConfig
	Lifecycle fx.Lifecycle
}

type ClientOutput struct {
	fx.Out

	Client etcdclient.Client `name:"executorstore"`
}

// NewClient creates a new etcd client or uses the provided one
func NewClient(p ClientParams) (ClientOutput, error) {
	if p.Client != nil {
		return ClientOutput{Client: p.Client}, nil
	}

	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   p.Cfg.Endpoints,
		DialTimeout: p.Cfg.DialTimeout,
	})
	if err != nil {
		return ClientOutput{}, err
	}

	p.Lifecycle.Append(fx.StopHook(etcdClient.Close))
	return ClientOutput{Client: etcdClient}, nil
}

type ETCDConfig struct {
	Endpoints   []string      `yaml:"endpoints"`
	DialTimeout time.Duration `yaml:"dialTimeout"`
	Prefix      string        `yaml:"prefix"`
	Compression string        `yaml:"compression"`
}

// NewETCDConfig parses ETCDConfig from ShardDistribution config
func NewETCDConfig(cfg config.ShardDistribution) (ETCDConfig, error) {
	var etcdCfg ETCDConfig
	if err := cfg.Store.StorageParams.Decode(&etcdCfg); err != nil {
		return etcdCfg, fmt.Errorf("bad config for etcd store: %w", err)
	}

	return etcdCfg, nil
}
