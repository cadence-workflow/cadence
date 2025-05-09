package etcd

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/fx"

	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/leader/leaderstore"
)

func init() {
	leaderstore.RegisterStore("etcd", fx.Provide(NewStore))
}

type LeaderStore struct {
	client         *clientv3.Client
	electionConfig etcdCfg
}

type StoreParams struct {
	fx.In

	// Client could be provided externally.
	Client    *clientv3.Client `optional:"true"`
	Cfg       config.LeaderElection
	Lifecycle fx.Lifecycle
}

type etcdCfg struct {
	Endpoints   []string      `yaml:"endpoints"`
	DialTimeout time.Duration `yaml:"dialTimeout"`
	Prefix      string        `yaml:"prefix"`
	ElectionTTL time.Duration `yaml:"electionTTL"`
}

// NewStore creates a new leaderstore backed by ETCD.
func NewStore(p StoreParams) (leaderstore.Store, error) {
	if !p.Cfg.Enabled {
		return nil, nil
	}

	var err error

	var out etcdCfg
	if err := p.Cfg.Store.StorageParams.Decode(&out); err != nil {
		return nil, fmt.Errorf("bad config: %w", err)
	}

	etcdClient := p.Client
	if etcdClient == nil {
		etcdClient, err = clientv3.New(clientv3.Config{
			Endpoints:   out.Endpoints,
			DialTimeout: out.DialTimeout,
		})
		if err != nil {
			return nil, err
		}
	}

	p.Lifecycle.Append(fx.StopHook(etcdClient.Close))

	return &LeaderStore{
		client:         etcdClient,
		electionConfig: out,
	}, nil
}

func (ls *LeaderStore) CreateElection(ctx context.Context, namespace string) (el leaderstore.Election, err error) {
	// Create a new session for election
	session, err := concurrency.NewSession(ls.client,
		concurrency.WithTTL(int(ls.electionConfig.ElectionTTL.Seconds())),
		concurrency.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Create election
	electionKey := fmt.Sprintf("/%s/%s", ls.electionConfig.Prefix, namespace)
	etcdElection := concurrency.NewElection(session, electionKey)

	return &election{election: etcdElection, session: session}, nil
}

// election is a wrapper around etcd.concurrency.Election to abstract implementation from etcd types.
type election struct {
	session  *concurrency.Session
	election *concurrency.Election
}

func (e *election) Resign(ctx context.Context) error {
	return e.election.Resign(ctx)
}

func (e *election) Cleanup(ctx context.Context) error {
	err := e.session.Close()
	if err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	return nil
}

func (e *election) Campaign(ctx context.Context, host string) error {
	return e.election.Campaign(ctx, host)
}

func (e *election) Done() <-chan struct{} {
	return e.session.Done()
}
