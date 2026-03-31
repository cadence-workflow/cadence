package etcdclient

import (
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=client_mock.go Client

// Client is an interface that groups the etcd clientv3 interfaces
// and provides session creation for distributed coordination.
type Client interface {
	clientv3.Cluster
	clientv3.KV
	clientv3.Lease
	clientv3.Watcher
	clientv3.Auth
	clientv3.Maintenance

	// Close closes the client connection.
	Close() error

	// NewSession creates a new etcd session for distributed coordination.
	// This enables leader election and distributed locking via the concurrency package.
	NewSession(opts ...concurrency.SessionOption) (*concurrency.Session, error)
}

// Ensure *clientv3.Client can be wrapped to implement Client.
var _ Client = (*clientWrapper)(nil)

// clientWrapper wraps clientv3.Client to implement the Client interface.
type clientWrapper struct {
	*clientv3.Client
}

// NewClient wraps a clientv3.Client to implement the Client interface.
func NewClient(c *clientv3.Client) Client {
	return &clientWrapper{Client: c}
}

// NewSession creates a new etcd session for distributed coordination.
func (c *clientWrapper) NewSession(opts ...concurrency.SessionOption) (*concurrency.Session, error) {
	return concurrency.NewSession(c.Client, opts...)
}
