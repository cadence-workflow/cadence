package leaderstore

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/fx"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	// defaultElectionTTL is the default time-to-live for an election session.
	// If the leader does not renew its lease within this time, it will lose leadership.
	defaultElectionTTL = 10 * time.Second
)

type LeaderStore struct {
	client         *clientv3.Client
	electionConfig etcdCfg
}

type etcdCfg struct {
	Endpoints   []string      `yaml:"endpoints"`
	DialTimeout time.Duration `yaml:"dialTimeout"`
	Prefix      string        `yaml:"prefix"`
	ElectionTTL time.Duration `yaml:"electionTTL"`
}

// StoreParams defines the dependencies for the etcd store, for use with fx.
type StoreParams struct {
	fx.In

	Client    *clientv3.Client `optional:"true"`
	Cfg       config.ShardDistribution
	Lifecycle fx.Lifecycle
	Logger    log.Logger
}

// NewLeaderStore creates a new leaderstore backed by ETCD.
func NewLeaderStore(p StoreParams) (store.Elector, error) {
	var err error

	var out etcdCfg
	if err := p.Cfg.LeaderStore.StorageParams.Decode(&out); err != nil {
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

	if out.ElectionTTL == 0 {
		out.ElectionTTL = defaultElectionTTL
	}

	p.Lifecycle.Append(fx.StopHook(etcdClient.Close))

	return &LeaderStore{
		client:         etcdClient,
		electionConfig: out,
	}, nil
}

func (ls *LeaderStore) CreateElection(ctx context.Context, namespace string) (el store.Election, err error) {
	// Create a new session for election
	session, err := concurrency.NewSession(ls.client,
		concurrency.WithTTL(int(ls.electionConfig.ElectionTTL.Seconds())),
		concurrency.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	namespacePrefix := fmt.Sprintf("%s/%s", ls.electionConfig.Prefix, namespace)
	electionKey := fmt.Sprintf("%s/leader", namespacePrefix)
	etcdElection := concurrency.NewElection(session, electionKey)

	return &election{election: etcdElection, session: session, prefix: namespacePrefix}, nil
}

// election is a wrapper around etcd.concurrency.Election to abstract implementation from etcd types.
type election struct {
	session  *concurrency.Session
	election *concurrency.Election
	prefix   string
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

func (e *election) Guard() store.GuardFunc {
	return func(txn store.Txn) (store.Txn, error) {
		// The guard receives the generic Txn and asserts it to the concrete type it expects.
		etcdTxn, ok := txn.(clientv3.Txn)
		if !ok {
			return nil, fmt.Errorf("invalid transaction type for etcd guard: expected clientv3.Txn, got %T", txn)
		}
		// It applies the etcd-specific condition and returns the modified generic Txn.
		return etcdTxn.If(clientv3.Compare(clientv3.ModRevision(e.election.Key()), "=", e.election.Rev())), nil
	}
}
