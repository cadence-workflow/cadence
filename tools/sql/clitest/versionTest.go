// Copyright (c) 2019 Uber Technologies, Inc.
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

package clitest

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"runtime"
	"strconv"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/environment"
	"github.com/uber/cadence/tools/sql"
)

type (
	// VersionTestSuite defines a test suite
	VersionTestSuite struct {
		*require.Assertions // override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test, not merely log an error
		suite.Suite
		pluginName string
	}
)

// NewVersionTestSuite returns a test suite
func NewVersionTestSuite(pluginName string) *VersionTestSuite {
	return &VersionTestSuite{
		pluginName: pluginName,
	}
}

// SetupTest setups test suite
func (s *VersionTestSuite) SetupTest() {
	s.Assertions = require.New(s.T()) // Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
}

// TestVerifyCompatibleVersion test
func (s *VersionTestSuite) TestVerifyCompatibleVersion() {
	database := "cadence_test"
	visDatabase := "cadence_visibility_test"
	_, filename, _, ok := runtime.Caller(0)
	s.True(ok)
	root := path.Dir(path.Dir(path.Dir(path.Dir(filename))))
	sqlFile := path.Join(root, "schema/mysql/v8/cadence/schema.sql")
	visSQLFile := path.Join(root, "schema/mysql/v8/visibility/schema.sql")

	defer s.createDatabase(database)()
	defer s.createDatabase(visDatabase)()
	mysqlPort, err := environment.GetMySQLPort()
	s.NoError(err)
	port := strconv.Itoa(mysqlPort)
	err = sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", database,
		"-pl", s.pluginName,
		"-q",
		"setup-schema",
		"-f", sqlFile,
		"-version", "10.0",
		"-o",
	})
	s.NoError(err)
	err = sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", visDatabase,
		"-pl", s.pluginName,
		"-q",
		"setup-schema",
		"-f", visSQLFile,
		"-version", "10.0",
		"-o",
	})
	s.NoError(err)

	defaultCfg := config.SQL{
		ConnectAddr:   fmt.Sprintf("%v:%v", environment.GetMySQLAddress(), port),
		User:          environment.GetMySQLUser(),
		Password:      environment.GetMySQLPassword(),
		PluginName:    s.pluginName,
		DatabaseName:  database,
		EncodingType:  "thriftrw",
		DecodingTypes: []string{"thriftrw"},
	}
	visibilityCfg := defaultCfg
	visibilityCfg.DatabaseName = visDatabase
	cfg := config.Persistence{
		DefaultStore:    "default",
		VisibilityStore: "visibility",
		DataStores: map[string]config.DataStore{
			"default":    {SQL: &defaultCfg},
			"visibility": {SQL: &visibilityCfg},
		},
		TransactionSizeLimit: dynamicconfig.GetIntPropertyFn(common.DefaultTransactionSizeLimit),
		ErrorInjectionRate:   dynamicconfig.GetFloatPropertyFn(0),
	}
	s.NoError(sql.VerifyCompatibleVersion(cfg))
}

// TestCheckCompatibleVersion test
func (s *VersionTestSuite) TestCheckCompatibleVersion() {
	flags := []struct {
		expectedVersion string
		actualVersion   string
		errStr          string
		expectedFail    bool
	}{
		{"2.0", "1.0", "version mismatch", false},
		{"1.0", "1.0", "", false},
		{"1.0", "2.0", "", false},
		{"1.0", "abc", "schema_version' doesn't exist", true},
	}
	for _, flag := range flags {
		s.runCheckCompatibleVersion(flag.expectedVersion, flag.actualVersion, flag.errStr, flag.expectedFail)
	}
}

func (s *VersionTestSuite) createDatabase(database string) func() {
	connection, err := newTestConn("", s.pluginName)
	s.NoError(err)
	err = connection.CreateDatabase(database)
	s.NoError(err)
	return func() {
		s.NoError(connection.DropDatabase(database))
		connection.Close()
	}
}

func (s *VersionTestSuite) runCheckCompatibleVersion(
	expected string, actual string, errStr string, expectedFail bool,
) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	database := fmt.Sprintf("version_test_%v", r.Int63())
	defer s.createDatabase(database)()

	tmpDir := s.T().TempDir()

	subdir := tmpDir + "/" + database
	s.NoError(os.Mkdir(subdir, os.FileMode(0744)))

	s.createSchemaForVersion(subdir, actual)
	if expected != actual {
		s.createSchemaForVersion(subdir, expected)
	}

	sqlFile := subdir + "/v" + actual + "/tmp.sql"
	mysqlPort, err := environment.GetMySQLPort()
	s.NoError(err)
	port := strconv.Itoa(mysqlPort)

	err = sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", database,
		"-pl", s.pluginName,
		"setup-schema",
		"-f", sqlFile,
		"-version", actual,
		"-o",
	})
	if expectedFail {
		s.Error(err)
	} else {
		s.NoError(err)
	}

	cfg := config.SQL{
		ConnectAddr:   fmt.Sprintf("%v:%v", environment.GetMySQLAddress(), port),
		User:          environment.GetMySQLUser(),
		Password:      environment.GetMySQLPassword(),
		PluginName:    s.pluginName,
		DatabaseName:  database,
		EncodingType:  "thriftrw",
		DecodingTypes: []string{"thriftrw"},
	}
	err = sql.CheckCompatibleVersion(cfg, expected)
	if len(errStr) > 0 {
		s.Error(err)
		s.Contains(err.Error(), errStr)
	} else {
		s.NoError(err)
	}
}

func (s *VersionTestSuite) createSchemaForVersion(subdir string, v string) {
	vDir := subdir + "/v" + v
	s.NoError(os.Mkdir(vDir, os.FileMode(0744)))
	cqlFile := vDir + "/tmp.sql"
	s.NoError(ioutil.WriteFile(cqlFile, []byte{}, os.FileMode(0644)))
}

func (s *VersionTestSuite) TestMultipleDatabaseVersionInCompatible() {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	r2 := rand.New(rand.NewSource(time.Now().UnixNano()))
	database1 := fmt.Sprintf("version_test_%v", r1.Int63())
	database2 := fmt.Sprintf("version_test_%v", r2.Int63())
	defer s.createDatabase(database1)()
	defer s.createDatabase(database2)()

	tmpDir := s.T().TempDir()

	subdir := tmpDir + "/" + "db"
	s.NoError(os.Mkdir(subdir, os.FileMode(0744)))

	s.createSchemaForVersion(subdir, "0.1")
	s.createSchemaForVersion(subdir, "0.2")
	s.createSchemaForVersion(subdir, "0.3")
	mysqlPort, err := environment.GetMySQLPort()
	s.NoError(err)
	port := strconv.Itoa(mysqlPort)
	s.NoError(sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", database1,
		"-pl", s.pluginName,
		"-q",
		"setup-schema",
		"-version", "0.2",
		"-o",
	}))

	s.NoError(sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", database2,
		"-pl", s.pluginName,
		"-q",
		"setup-schema",
		"-version", "0.3",
		"-o",
	}))

	cfg := config.SQL{
		PluginName:           s.pluginName,
		EncodingType:         "thriftrw",
		DecodingTypes:        []string{"thriftrw"},
		UseMultipleDatabases: true,
		NumShards:            2,
		MultipleDatabasesConfig: []config.MultipleDatabasesConfigEntry{
			{
				ConnectAddr:  fmt.Sprintf("%v:%v", environment.GetMySQLAddress(), port),
				User:         environment.GetMySQLUser(),
				Password:     environment.GetMySQLPassword(),
				DatabaseName: database1,
			},
			{
				ConnectAddr:  fmt.Sprintf("%v:%v", environment.GetMySQLAddress(), port),
				User:         environment.GetMySQLUser(),
				Password:     environment.GetMySQLPassword(),
				DatabaseName: database2,
			},
		},
	}
	err = sql.CheckCompatibleVersion(cfg, "0.3")
	s.Error(err)
	s.Contains(err.Error(), "version mismatch")
}

func (s *VersionTestSuite) TestMultipleDatabaseVersionAllLowerCompatible() {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	r2 := rand.New(rand.NewSource(time.Now().UnixNano()))
	database1 := fmt.Sprintf("version_test_%v", r1.Int63())
	database2 := fmt.Sprintf("version_test_%v", r2.Int63())
	defer s.createDatabase(database1)()
	defer s.createDatabase(database2)()

	tmpDir := s.T().TempDir()

	subdir := tmpDir + "/" + "db"
	s.NoError(os.Mkdir(subdir, os.FileMode(0744)))

	s.createSchemaForVersion(subdir, "0.1")
	s.createSchemaForVersion(subdir, "0.2")
	s.createSchemaForVersion(subdir, "0.3")
	mysqlPort, err := environment.GetMySQLPort()
	s.NoError(err)
	port := strconv.Itoa(mysqlPort)
	s.NoError(sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", database1,
		"-pl", s.pluginName,
		"-q",
		"setup-schema",
		"-version", "0.2",
		"-o",
	}))

	s.NoError(sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", database2,
		"-pl", s.pluginName,
		"-q",
		"setup-schema",
		"-version", "0.2",
		"-o",
	}))

	cfg := config.SQL{
		PluginName:           s.pluginName,
		EncodingType:         "thriftrw",
		DecodingTypes:        []string{"thriftrw"},
		UseMultipleDatabases: true,
		NumShards:            2,
		MultipleDatabasesConfig: []config.MultipleDatabasesConfigEntry{
			{
				ConnectAddr:  fmt.Sprintf("%v:%v", environment.GetMySQLAddress(), port),
				User:         environment.GetMySQLUser(),
				Password:     environment.GetMySQLPassword(),
				DatabaseName: database1,
			},
			{
				ConnectAddr:  fmt.Sprintf("%v:%v", environment.GetMySQLAddress(), port),
				User:         environment.GetMySQLUser(),
				Password:     environment.GetMySQLPassword(),
				DatabaseName: database2,
			},
		},
	}
	err = sql.CheckCompatibleVersion(cfg, "0.2")
	s.NoError(err)
}

func (s *VersionTestSuite) TestMultipleDatabaseVersionPartialLowerCompatible() {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	r2 := rand.New(rand.NewSource(time.Now().UnixNano()))
	database1 := fmt.Sprintf("version_test_%v", r1.Int63())
	database2 := fmt.Sprintf("version_test_%v", r2.Int63())
	defer s.createDatabase(database1)()
	defer s.createDatabase(database2)()

	tmpDir := s.T().TempDir()

	subdir := tmpDir + "/" + "db"
	s.NoError(os.Mkdir(subdir, os.FileMode(0744)))

	s.createSchemaForVersion(subdir, "0.1")
	s.createSchemaForVersion(subdir, "0.2")
	s.createSchemaForVersion(subdir, "0.3")
	mysqlPort, err := environment.GetMySQLPort()
	s.NoError(err)
	port := strconv.Itoa(mysqlPort)
	s.NoError(sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", database1,
		"-pl", s.pluginName,
		"-q",
		"setup-schema",
		"-version", "0.3",
		"-o",
	}))

	s.NoError(sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", database2,
		"-pl", s.pluginName,
		"-q",
		"setup-schema",
		"-version", "0.2",
		"-o",
	}))

	cfg := config.SQL{
		PluginName:           s.pluginName,
		EncodingType:         "thriftrw",
		DecodingTypes:        []string{"thriftrw"},
		UseMultipleDatabases: true,
		NumShards:            2,
		MultipleDatabasesConfig: []config.MultipleDatabasesConfigEntry{
			{
				ConnectAddr:  fmt.Sprintf("%v:%v", environment.GetMySQLAddress(), port),
				User:         environment.GetMySQLUser(),
				Password:     environment.GetMySQLPassword(),
				DatabaseName: database1,
			},
			{
				ConnectAddr:  fmt.Sprintf("%v:%v", environment.GetMySQLAddress(), port),
				User:         environment.GetMySQLUser(),
				Password:     environment.GetMySQLPassword(),
				DatabaseName: database2,
			},
		},
	}
	err = sql.CheckCompatibleVersion(cfg, "0.2")
	s.NoError(err)
}

func (s *VersionTestSuite) TestMultipleDatabaseVersionExactlyMatchCompatible() {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	r2 := rand.New(rand.NewSource(time.Now().UnixNano()))
	database1 := fmt.Sprintf("version_test_%v", r1.Int63())
	database2 := fmt.Sprintf("version_test_%v", r2.Int63())
	defer s.createDatabase(database1)()
	defer s.createDatabase(database2)()

	tmpDir := s.T().TempDir()

	subdir := tmpDir + "/" + "db"
	s.NoError(os.Mkdir(subdir, os.FileMode(0744)))

	s.createSchemaForVersion(subdir, "0.1")
	s.createSchemaForVersion(subdir, "0.2")
	s.createSchemaForVersion(subdir, "0.3")
	mysqlPort, err := environment.GetMySQLPort()
	s.NoError(err)
	port := strconv.Itoa(mysqlPort)
	s.NoError(sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", database1,
		"-pl", s.pluginName,
		"-q",
		"setup-schema",
		"-version", "0.3",
		"-o",
	}))

	s.NoError(sql.RunTool([]string{
		"./tool",
		"-ep", environment.GetMySQLAddress(),
		"-p", port,
		"-u", environment.GetMySQLUser(),
		"-pw", environment.GetMySQLPassword(),
		"-db", database2,
		"-pl", s.pluginName,
		"-q",
		"setup-schema",
		"-version", "0.3",
		"-o",
	}))

	cfg := config.SQL{
		PluginName:           s.pluginName,
		EncodingType:         "thriftrw",
		DecodingTypes:        []string{"thriftrw"},
		UseMultipleDatabases: true,
		NumShards:            2,
		MultipleDatabasesConfig: []config.MultipleDatabasesConfigEntry{
			{
				ConnectAddr:  fmt.Sprintf("%v:%v", environment.GetMySQLAddress(), port),
				User:         environment.GetMySQLUser(),
				Password:     environment.GetMySQLPassword(),
				DatabaseName: database1,
			},
			{
				ConnectAddr:  fmt.Sprintf("%v:%v", environment.GetMySQLAddress(), port),
				User:         environment.GetMySQLUser(),
				Password:     environment.GetMySQLPassword(),
				DatabaseName: database2,
			},
		},
	}
	err = sql.CheckCompatibleVersion(cfg, "0.3")
	s.NoError(err)
}
