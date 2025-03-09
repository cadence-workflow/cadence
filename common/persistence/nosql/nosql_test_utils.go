// Copyright (c) 2017 Uber Technologies, Inc.
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

package nosql

import (
	"testing"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/persistence-tests/testcluster"
)

// testCluster allows executing cassandra operations in testing.
type testCluster struct {
	logger log.Logger

	keyspace      string
	schemaBaseDir string
	replicas      int
	cfg           config.NoSQL
}

// TestClusterParams are params for test cluster initialization.
type TestClusterParams struct {
	PluginName    string
	KeySpace      string
	Username      string
	Password      string
	Host          string
	Port          int
	ProtoVersion  int
	SchemaBaseDir string
	// Replicas defaults to 1 if not set
	Replicas int
	// MaxConns defaults to 2 if not set
	MaxConns int
}

// NewTestCluster returns a new cassandra test cluster
// if schemaBaseDir is empty, it will be auto-resolved based on os.Getwd()
// otherwise the specified value will be used (used by internal tests)
func NewTestCluster(t *testing.T, params TestClusterParams) testcluster.PersistenceTestCluster {
	return &testCluster{
		logger:        testlogger.New(t),
		keyspace:      params.KeySpace,
		schemaBaseDir: params.SchemaBaseDir,
		replicas:      replicas(params.Replicas),
		cfg: config.NoSQL{
			PluginName:   params.PluginName,
			User:         params.Username,
			Password:     params.Password,
			Hosts:        params.Host,
			Port:         params.Port,
			MaxConns:     maxConns(params.MaxConns),
			Keyspace:     params.KeySpace,
			ProtoVersion: params.ProtoVersion,
		},
	}
}

// Config returns the persistence config for connecting to this test cluster
func (s *testCluster) Config() config.Persistence {
	return config.Persistence{
		DefaultStore:    "test",
		VisibilityStore: "test",
		DataStores: map[string]config.DataStore{
			"test": {NoSQL: &s.cfg},
		},
		TransactionSizeLimit: dynamicconfig.GetIntPropertyFn(constants.DefaultTransactionSizeLimit),
		ErrorInjectionRate:   dynamicconfig.GetFloatPropertyFn(0),
	}
}

// SetupTestDatabase from PersistenceTestCluster interface
func (s *testCluster) SetupTestDatabase() {
	adminDB, err := NewNoSQLAdminDB(&s.cfg, s.logger, &persistence.DynamicConfiguration{})

	if err != nil {
		s.logger.Fatal(err.Error())
	}
	err = adminDB.SetupTestDatabase(s.schemaBaseDir, s.replicas)
	if err != nil {
		s.logger.Fatal(err.Error())
	}
}

// TearDownTestDatabase from PersistenceTestCluster interface
func (s *testCluster) TearDownTestDatabase() {
	adminDB, err := NewNoSQLAdminDB(&s.cfg, s.logger, &persistence.DynamicConfiguration{})
	if err != nil {
		s.logger.Fatal(err.Error())
	}
	err = adminDB.TeardownTestDatabase()
	if err != nil {
		s.logger.Fatal(err.Error())
	}
}

func replicas(replicas int) int {
	if replicas == 0 {
		return 1
	}

	return replicas
}

func maxConns(maxConns int) int {
	if maxConns == 0 {
		return 2
	}

	return maxConns
}
