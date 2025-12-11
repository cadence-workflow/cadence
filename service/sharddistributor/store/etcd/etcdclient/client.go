package etcdclient

import clientv3 "go.etcd.io/etcd/client/v3"

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=client_mock.go Client

// Client is an interface that groups the etcd clientv3 interfaces
type Client interface {
	clientv3.Cluster
	clientv3.KV
	clientv3.Lease
	clientv3.Watcher
	clientv3.Auth
	clientv3.Maintenance
}
