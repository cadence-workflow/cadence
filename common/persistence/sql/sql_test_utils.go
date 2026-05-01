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

package sql

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence/persistence-tests/testcluster"
	"github.com/uber/cadence/environment"
)

// testCluster allows executing cassandra operations in testing.
type testCluster struct {
	dbName    string
	schemaDir string
	cfg       config.SQL
}

var _ testcluster.PersistenceTestCluster = (*testCluster)(nil)

// NewTestCluster returns a new SQL test cluster
func NewTestCluster(pluginName, dbName, username, password, host string, port int, schemaDir string) (testcluster.PersistenceTestCluster, error) {
	var result testCluster
	var err error
	if port == 0 {
		port, err = environment.GetMySQLPort()
		if err != nil {
			return nil, err
		}
	}

	if schemaDir == "" {
		return nil, errors.New("schemaDir is empty")
	}
	result.dbName = dbName
	result.schemaDir = schemaDir
	result.cfg = config.SQL{
		User:            username,
		Password:        password,
		ConnectAddr:     fmt.Sprintf("%v:%v", host, port),
		ConnectProtocol: "tcp",
		PluginName:      pluginName,
		DatabaseName:    dbName,
		NumShards:       4,
		EncodingType:    "thriftrw",
		DecodingTypes:   []string{"thriftrw"},
	}
	return &result, nil
}

// DatabaseName from PersistenceTestCluster interface
func (s *testCluster) DatabaseName() string {
	return s.dbName
}

// SetupTestDatabase from PersistenceTestCluster interface
func (s *testCluster) SetupTestDatabase() {
	s.createDatabase()

	schemaDir := s.schemaDir + "/"
	if !strings.HasPrefix(schemaDir, "/") && !strings.HasPrefix(schemaDir, "../") {
		cadencePackageDir, err := getCadencePackageDir()
		if err != nil {
			log.Fatal(err)
		}
		schemaDir = cadencePackageDir + schemaDir
	}
	s.loadSchema([]string{"schema.sql"}, schemaDir)
	s.loadVisibilitySchema([]string{"schema.sql"}, schemaDir)
}

// Config returns the persistence config for connecting to this test cluster
func (s *testCluster) Config() config.Persistence {
	cfg := s.cfg
	return config.Persistence{
		DefaultStore:    "test",
		VisibilityStore: "test",
		DataStores: map[string]config.DataStore{
			"test": {SQL: &cfg},
		},
		TransactionSizeLimit: dynamicproperties.GetIntPropertyFn(constants.DefaultTransactionSizeLimit),
		ErrorInjectionRate:   dynamicproperties.GetFloatPropertyFn(0),
		NumHistoryShards:     s.cfg.NumShards,
	}
}

// TearDownTestDatabase from PersistenceTestCluster interface
func (s *testCluster) TearDownTestDatabase() {
	s.dropDatabase()
}

// createDatabase from PersistenceTestCluster interface
func (s *testCluster) createDatabase() {
	if s.cfg.PluginName == "sqlite" {
		// sqlite doesn't support creating database
		return
	}

	cfg2 := s.cfg
	// NOTE need to connect with empty name to create new database
	cfg2.DatabaseName = ""
	db, err := NewSQLAdminDB(&cfg2)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	err = db.CreateDatabase(s.cfg.DatabaseName)
	if err != nil {
		panic(err)
	}
}

// dropDatabase from PersistenceTestCluster interface
func (s *testCluster) dropDatabase() {
	if s.cfg.PluginName == "sqlite" {
		// sqlite doesn't support dropping database
		return
	}

	cfg2 := s.cfg
	// NOTE need to connect with empty name to drop the database
	cfg2.DatabaseName = ""
	db, err := NewSQLAdminDB(&cfg2)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()
	err = db.DropDatabase(s.cfg.DatabaseName)
	if err != nil {
		panic(err)
	}
}

// loadSchema from PersistenceTestCluster interface
func (s *testCluster) loadSchema(fileNames []string, schemaDir string) {
	workflowSchemaDir := schemaDir + "/cadence"
	err := s.loadDatabaseSchema(workflowSchemaDir, fileNames, true)
	if err != nil {
		log.Fatal(err)
	}
}

// loadVisibilitySchema from PersistenceTestCluster interface
func (s *testCluster) loadVisibilitySchema(fileNames []string, schemaDir string) {
	workflowSchemaDir := schemaDir + "/visibility"
	err := s.loadDatabaseSchema(workflowSchemaDir, fileNames, true)
	if err != nil {
		log.Fatal(err)
	}
}

func getCadencePackageDir() (string, error) {
	cadencePackageDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	cadenceIndex := strings.LastIndex(cadencePackageDir, "cadence/")
	cadencePackageDir = cadencePackageDir[:cadenceIndex+len("cadence/")]
	return cadencePackageDir, err
}

// loadDatabaseSchema loads the schema from the given .sql files on this database
func (s *testCluster) loadDatabaseSchema(dir string, fileNames []string, override bool) (err error) {
	db, err := NewSQLAdminDB(&s.cfg)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	for _, file := range fileNames {
		// This is only used in tests. Excluding it from security scanners
		// #nosec
		content, err := ioutil.ReadFile(dir + "/" + file)
		if err != nil {
			return fmt.Errorf("error reading contents of file %v:%v", file, err.Error())
		}
		err = db.ExecSchemaOperationQuery(context.Background(), string(content))
		if err != nil {
			return fmt.Errorf("error loading schema from %v: %v", file, err.Error())
		}
	}
	return nil
}
