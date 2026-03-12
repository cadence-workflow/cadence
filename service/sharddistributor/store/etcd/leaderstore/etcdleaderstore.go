package leaderstore

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/fx"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdclient"
)

const (
	// defaultElectionTTL is the default time-to-live for an election session.
	// If the leader does not renew its lease within this time, it will lose leadership.
	defaultElectionTTL = 10 * time.Second
)

type LeaderStore struct {
	client etcdclient.Client
	config etcdclient.LeaderStoreConfig
}

// StoreParams defines the dependencies for the etcd store, for use with fx.
type StoreParams struct {
	fx.In

	Client etcdclient.Client `name:"leaderstore"`
	Cfg    etcdclient.LeaderStoreConfig
	Logger log.Logger
}

// NewLeaderStore creates a new leaderstore backed by ETCD.
func NewLeaderStore(p StoreParams) (store.Elector, error) {
	cfg := p.Cfg
	if cfg.ElectionTTL == 0 {
		cfg.ElectionTTL = defaultElectionTTL
	}

	return &LeaderStore{
		client: p.Client,
		config: cfg,
	}, nil
}

func (ls *LeaderStore) CreateElection(ctx context.Context, namespace string) (el store.Election, err error) {
	// Create a new session for election
	session, err := ls.client.NewSession(
		concurrency.WithTTL(int(ls.config.ElectionTTL.Seconds())),
		concurrency.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	namespacePrefix := fmt.Sprintf("%s/%s", ls.config.Prefix, namespace)
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
