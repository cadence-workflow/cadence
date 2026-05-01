// Copyright (c) 2020 Uber Technologies, Inc.
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

package cassandra

import (
	"fmt"
	"time"

	gogocql "github.com/gocql/gocql"
	"github.com/hailocab/go-hostpool"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/environment"
)

const (
	// PluginName is the name of the plugin
	PluginName            = "cassandra"
	defaultSessionTimeout = 10 * time.Second
	defaultConnectTimeout = 2 * time.Second
)

type plugin struct{}

var _ nosqlplugin.Plugin = (*plugin)(nil)

func init() {
	nosql.RegisterPlugin(PluginName, &plugin{})
}

// CreateDB initialize the db object
func (p *plugin) CreateDB(cfg *config.NoSQL, logger log.Logger, dc *persistence.DynamicConfiguration) (nosqlplugin.DB, error) {
	return p.doCreateDB(cfg, logger, dc)
}

// CreateAdminDB initialize the AdminDB object
func (p *plugin) CreateAdminDB(cfg *config.NoSQL, logger log.Logger, dc *persistence.DynamicConfiguration) (nosqlplugin.AdminDB, error) {
	// the keyspace is not created yet, so use empty and let the Cassandra connect
	keyspace := cfg.Keyspace
	cfg.Keyspace = ""
	// change it back
	defer func() {
		cfg.Keyspace = keyspace
	}()

	return p.doCreateDB(cfg, logger, dc)
}

func (p *plugin) doCreateDB(cfg *config.NoSQL, logger log.Logger, dc *persistence.DynamicConfiguration) (*CDB, error) {
	gocqlConfig, err := toGoCqlConfig(cfg)
	if err != nil {
		return nil, err
	}
	session, err := gocql.GetRegisteredClient().CreateSession(gocqlConfig)
	if err != nil {
		return nil, err
	}
	db := NewCassandraDBFromSession(cfg, session, logger, dc)
	return db, nil
}

func toGoCqlConfig(cfg *config.NoSQL) (gocql.ClusterConfig, error) {
	var err error
	if cfg.Port == 0 {
		cfg.Port, err = environment.GetCassandraPort()
		if err != nil {
			return gocql.ClusterConfig{}, err
		}
	}
	if cfg.Hosts == "" {
		cfg.Hosts = environment.GetCassandraAddress()
	}
	if cfg.ProtoVersion == 0 {
		cfg.ProtoVersion, err = environment.GetCassandraProtoVersion()
		if err != nil {
			return gocql.ClusterConfig{}, err
		}
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = defaultSessionTimeout
	}

	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = defaultConnectTimeout
	}

	if cfg.Consistency == "" {
		cfg.Consistency = cassandraDefaultConsLevel.String()
	}

	if cfg.SerialConsistency == "" {
		cfg.SerialConsistency = cassandraDefaultSerialConsLevel.String()
	}

	consistency, err := gocql.ParseConsistency(cfg.Consistency)
	if err != nil {
		return gocql.ClusterConfig{}, err
	}

	serialConsistency, err := gocql.ParseSerialConsistency(cfg.SerialConsistency)
	if err != nil {
		return gocql.ClusterConfig{}, err
	}

	hostSelection, err := toHostSelectionPolicy(cfg.HostSelectionPolicy)
	if err != nil {
		return gocql.ClusterConfig{}, err
	}

	return gocql.ClusterConfig{
		Hosts:                 cfg.Hosts,
		Port:                  cfg.Port,
		User:                  cfg.User,
		Password:              cfg.Password,
		AllowedAuthenticators: cfg.AllowedAuthenticators,
		Keyspace:              cfg.Keyspace,
		Region:                cfg.Region,
		Datacenter:            cfg.Datacenter,
		MaxConns:              cfg.MaxConns,
		TLS:                   cfg.TLS,
		ProtoVersion:          cfg.ProtoVersion,
		Consistency:           consistency,
		SerialConsistency:     serialConsistency,
		Timeout:               cfg.Timeout,
		ConnectTimeout:        cfg.ConnectTimeout,
		HostSelectionPolicy:   hostSelection,
	}, nil
}

func toHostSelectionPolicy(policy string) (gogocql.HostSelectionPolicy, error) {
	switch policy {
	case "", "tokenaware,roundrobin":
		return gogocql.TokenAwareHostPolicy(gogocql.RoundRobinHostPolicy()), nil
	case "hostpool-epsilon-greedy":
		return gogocql.HostPoolHostPolicy(
			hostpool.NewEpsilonGreedy(nil, 0, &hostpool.LinearEpsilonValueCalculator{}),
		), nil
	case "roundrobin":
		return gogocql.RoundRobinHostPolicy(), nil
	default:
		return nil, fmt.Errorf("unknown gocql host selection policy: %q", policy)
	}
}
