// Copyright (c) 2017-2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package gocql

import (
	"crypto/tls"
	"strings"

	"github.com/gocql/gocql"

	"github.com/uber/cadence/environment"
)

var (
	registered Client
)

// GetRegisteredClient gets a gocql client based registered object
func GetRegisteredClient() Client {
	if registered == nil {
		panic("binary build error: gocql client is not registered yet!")
	}
	return registered
}

// RegisterClient registers a client into this package, can only be called once
func RegisterClient(c Client) {
	if registered == nil {
		registered = c
	} else {
		panic("binary build error: gocql client is already register!")
	}
}

func newCassandraCluster(cfg ClusterConfig) *gocql.ClusterConfig {
	hosts := parseHosts(cfg.Hosts)
	cluster := gocql.NewCluster(hosts...)
	if cfg.ProtoVersion == 0 {
		cfg.ProtoVersion = environment.CassandraDefaultProtoVersionInteger
	}
	cluster.ProtoVersion = cfg.ProtoVersion
	if cfg.Port > 0 {
		cluster.Port = cfg.Port
	}
	if cfg.User != "" && cfg.Password != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username:              cfg.User,
			Password:              cfg.Password,
			AllowedAuthenticators: cfg.AllowedAuthenticators,
		}
	}
	if cfg.Keyspace != "" {
		cluster.Keyspace = cfg.Keyspace
	}
	if cfg.Datacenter != "" {
		cluster.HostFilter = gocql.DataCentreHostFilter(cfg.Datacenter)
	}
	if cfg.Region != "" {
		cluster.HostFilter = regionHostFilter(cfg.Region)
	}

	if cfg.TLS != nil && cfg.TLS.Enabled {
		cluster.SslOpts = &gocql.SslOptions{
			CertPath:               cfg.TLS.CertFile,
			KeyPath:                cfg.TLS.KeyFile,
			CaPath:                 cfg.TLS.CaFile,
			EnableHostVerification: cfg.TLS.EnableHostVerification,

			Config: &tls.Config{
				ServerName: cfg.TLS.ServerName,
			},
		}
	}
	if cfg.MaxConns > 0 {
		cluster.NumConns = cfg.MaxConns
	}

	if cfg.HostSelectionPolicy != nil {
		cluster.PoolConfig.HostSelectionPolicy = cfg.HostSelectionPolicy
	} else {
		// set default option if configuration was not provided
		cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	}

	return cluster
}

// regionHostFilter returns a gocql host filter for the given region name
func regionHostFilter(region string) gocql.HostFilter {
	return gocql.HostFilterFunc(func(host *gocql.HostInfo) bool {
		applicationRegion := region
		if len(host.DataCenter()) < 3 {
			return false
		}
		return host.DataCenter()[:3] == applicationRegion
	})
}

// parseHosts returns parses a list of hosts separated by comma
func parseHosts(input string) []string {
	var hosts = make([]string, 0)
	for _, h := range strings.Split(input, ",") {
		if host := strings.TrimSpace(h); len(host) > 0 {
			hosts = append(hosts, host)
		}
	}
	return hosts
}
