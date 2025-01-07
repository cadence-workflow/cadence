// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cli

import (
	"flag"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/client"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra"
	"github.com/uber/cadence/common/persistence/sql"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/reconciliation/invariant"
	commonFlag "github.com/uber/cadence/tools/common/flag"
)

func TestDefaultManagerFactory(t *testing.T) {
	tests := []struct {
		name         string
		setupMocks   func(*client.MockFactory, *gomock.Controller) interface{}
		methodToTest func(*defaultManagerFactory, *cli.Context) (interface{}, error)
		expectError  bool
	}{
		{
			name: "initializeExecutionManager - success",
			setupMocks: func(mockFactory *client.MockFactory, ctrl *gomock.Controller) interface{} {
				mockExecutionManager := persistence.NewMockExecutionManager(ctrl)
				mockFactory.EXPECT().NewExecutionManager(gomock.Any()).Return(mockExecutionManager, nil)
				return mockExecutionManager
			},
			methodToTest: func(f *defaultManagerFactory, ctx *cli.Context) (interface{}, error) {
				return f.initializeExecutionManager(ctx, 1)
			},
			expectError: false,
		},
		{
			name: "initializeExecutionManager - error",
			setupMocks: func(mockFactory *client.MockFactory, ctrl *gomock.Controller) interface{} {
				mockFactory.EXPECT().NewExecutionManager(gomock.Any()).Return(nil, fmt.Errorf("some error"))
				return nil
			},
			methodToTest: func(f *defaultManagerFactory, ctx *cli.Context) (interface{}, error) {
				return f.initializeExecutionManager(ctx, 1)
			},
			expectError: true,
		},
		{
			name: "initializeHistoryManager - success",
			setupMocks: func(mockFactory *client.MockFactory, ctrl *gomock.Controller) interface{} {
				mockHistoryManager := persistence.NewMockHistoryManager(ctrl)
				mockFactory.EXPECT().NewHistoryManager().Return(mockHistoryManager, nil)
				return mockHistoryManager
			},
			methodToTest: func(f *defaultManagerFactory, ctx *cli.Context) (interface{}, error) {
				return f.initializeHistoryManager(ctx)
			},
			expectError: false,
		},
		{
			name: "initializeHistoryManager - error",
			setupMocks: func(mockFactory *client.MockFactory, ctrl *gomock.Controller) interface{} {
				mockFactory.EXPECT().NewHistoryManager().Return(nil, fmt.Errorf("some error"))
				return nil
			},
			methodToTest: func(f *defaultManagerFactory, ctx *cli.Context) (interface{}, error) {
				return f.initializeHistoryManager(ctx)
			},
			expectError: true,
		},
		{
			name: "initializeShardManager - success",
			setupMocks: func(mockFactory *client.MockFactory, ctrl *gomock.Controller) interface{} {
				mockShardManager := persistence.NewMockShardManager(ctrl)
				mockFactory.EXPECT().NewShardManager().Return(mockShardManager, nil)
				return mockShardManager
			},
			methodToTest: func(f *defaultManagerFactory, ctx *cli.Context) (interface{}, error) {
				return f.initializeShardManager(ctx)
			},
			expectError: false,
		},
		{
			name: "initializeShardManager - error",
			setupMocks: func(mockFactory *client.MockFactory, ctrl *gomock.Controller) interface{} {
				mockFactory.EXPECT().NewShardManager().Return(nil, fmt.Errorf("some error"))
				return nil
			},
			methodToTest: func(f *defaultManagerFactory, ctx *cli.Context) (interface{}, error) {
				return f.initializeShardManager(ctx)
			},
			expectError: true,
		},
		{
			name: "initializeDomainManager - success",
			setupMocks: func(mockFactory *client.MockFactory, ctrl *gomock.Controller) interface{} {
				mockDomainManager := persistence.NewMockDomainManager(ctrl)
				mockFactory.EXPECT().NewDomainManager().Return(mockDomainManager, nil)
				return mockDomainManager
			},
			methodToTest: func(f *defaultManagerFactory, ctx *cli.Context) (interface{}, error) {
				return f.initializeDomainManager(ctx)
			},
			expectError: false,
		},
		{
			name: "initializeDomainManager - error",
			setupMocks: func(mockFactory *client.MockFactory, ctrl *gomock.Controller) interface{} {
				mockFactory.EXPECT().NewDomainManager().Return(nil, fmt.Errorf("some error"))
				return nil
			},
			methodToTest: func(f *defaultManagerFactory, ctx *cli.Context) (interface{}, error) {
				return f.initializeDomainManager(ctx)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockFactory := client.NewMockFactory(ctrl)
			expectedManager := tt.setupMocks(mockFactory, ctrl)

			f := &defaultManagerFactory{persistenceFactory: mockFactory}
			app := &cli.App{}
			ctx := cli.NewContext(app, nil, nil)

			result, err := tt.methodToTest(f, ctx)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expectedManager, result)
			}
		})
	}
}

func TestInitPersistenceFactory(t *testing.T) {
	ctrl := gomock.NewController(t)

	// Mock the ManagerFactory and ClientFactory
	mockClientFactory := NewMockClientFactory(ctrl)
	mockPersistenceFactory := client.NewMockFactory(ctrl)

	// Set up the context and app
	set := flag.NewFlagSet("test", 0)
	app := NewCliApp(mockClientFactory)
	c := cli.NewContext(app, set, nil)

	// Mock ServerConfig to return an error
	mockClientFactory.EXPECT().ServerConfig(gomock.Any()).Return(nil, fmt.Errorf("config error")).Times(1)

	// Initialize the ManagerFactory with the mock ClientFactory
	managerFactory := defaultManagerFactory{
		persistenceFactory: mockPersistenceFactory,
	}

	// Call initPersistenceFactory and validate results
	factory, err := managerFactory.initPersistenceFactory(c)

	// Assert that no error occurred and a default config was used
	assert.NoError(t, err)
	assert.NotNil(t, factory)
}

func TestInitializeInvariantManager(t *testing.T) {
	// Create an instance of defaultManagerFactory
	factory := &defaultManagerFactory{}

	// Define some fake invariants for testing
	invariants := []invariant.Invariant{}

	// Call initializeInvariantManager
	manager, err := factory.initializeInvariantManager(invariants)

	// Check that no error is returned
	require.NoError(t, err, "Expected no error from initializeInvariantManager")

	// Check that the returned Manager is not nil
	require.NotNil(t, manager, "Expected non-nil invariant.Manager")
}

func TestOverrideDataStore(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func(app *cli.App) *cli.Context
		inputDataStore config.DataStore
		expectedError  string
		expectedSQL    *config.SQL
	}{
		{
			name: "OverrideDBType_Cassandra",
			setupContext: func(app *cli.App) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagDBType, cassandra.PluginName, "DB type flag")
				require.NoError(t, set.Set(FlagDBType, cassandra.PluginName)) // Set DBType to Cassandra
				return cli.NewContext(app, set, nil)
			},
			inputDataStore: config.DataStore{}, // Empty DataStore to trigger createDataStore
			expectedError:  "",
			expectedSQL:    nil, // No SQL expected for Cassandra
		},
		{
			name: "OverrideSQLDataStore",
			setupContext: func(app *cli.App) *cli.Context {
				// Create a new mock SQL plugin using gomock
				ctrl := gomock.NewController(t)
				mockSQLPlugin := sqlplugin.NewMockPlugin(ctrl)

				// Register the mock SQL plugin for "mysql"
				sql.RegisterPlugin("mysql", mockSQLPlugin)

				set := flag.NewFlagSet("test", 0)
				set.String(FlagDBType, "mysql", "DB type flag") // Set SQL database type
				set.String(FlagDBAddress, "127.0.0.1", "DB address flag")
				set.String(FlagDBPort, "3306", "DB port flag")
				set.String(FlagUsername, "testuser", "DB username flag")
				set.String(FlagPassword, "testpass", "DB password flag")
				connAttr := &commonFlag.StringMap{}
				require.NoError(t, connAttr.Set("attr1=value1"))
				require.NoError(t, connAttr.Set("attr2=value2"))
				set.Var(connAttr, FlagConnectionAttributes, "Connection attributes flag")
				require.NoError(t, set.Set(FlagDBType, "mysql"))
				require.NoError(t, set.Set(FlagDBAddress, "127.0.0.1"))
				require.NoError(t, set.Set(FlagDBPort, "3306"))
				require.NoError(t, set.Set(FlagUsername, "testuser"))
				require.NoError(t, set.Set(FlagPassword, "testpass"))

				return cli.NewContext(app, set, nil)
			},
			expectedError: "",
			expectedSQL: &config.SQL{
				PluginName:  "mysql",
				ConnectAddr: "127.0.0.1:3306",
				User:        "testuser",
				Password:    "testpass",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up app and context
			app := cli.NewApp()
			c := tt.setupContext(app)

			// Call overrideDataStore with initial DataStore and capture result
			result, err := overrideDataStore(c, tt.inputDataStore)

			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				// Validate SQL DataStore settings if expected
				if tt.expectedSQL != nil && result.SQL != nil {
					assert.Equal(t, tt.expectedSQL.PluginName, result.SQL.PluginName)
					assert.Equal(t, tt.expectedSQL.ConnectAddr, result.SQL.ConnectAddr)
					assert.Equal(t, tt.expectedSQL.User, result.SQL.User)
					assert.Equal(t, tt.expectedSQL.Password, result.SQL.Password)
				}
			}
		})
	}
}

func TestOverrideTLS(t *testing.T) {
	tests := []struct {
		name         string
		setupContext func(app *cli.App) *cli.Context
		expectedTLS  config.TLS
	}{
		{
			name: "AllTLSFlagsSet",
			setupContext: func(app *cli.App) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.Bool(FlagEnableTLS, true, "Enable TLS flag")
				set.String(FlagTLSCertPath, "/path/to/cert", "TLS Cert Path")
				set.String(FlagTLSKeyPath, "/path/to/key", "TLS Key Path")
				set.String(FlagTLSCaPath, "/path/to/ca", "TLS CA Path")
				set.Bool(FlagTLSEnableHostVerification, true, "Enable Host Verification")

				require.NoError(t, set.Set(FlagEnableTLS, "true"))
				require.NoError(t, set.Set(FlagTLSCertPath, "/path/to/cert"))
				require.NoError(t, set.Set(FlagTLSKeyPath, "/path/to/key"))
				require.NoError(t, set.Set(FlagTLSCaPath, "/path/to/ca"))
				require.NoError(t, set.Set(FlagTLSEnableHostVerification, "true"))

				return cli.NewContext(app, set, nil)
			},
			expectedTLS: config.TLS{
				Enabled:                true,
				CertFile:               "/path/to/cert",
				KeyFile:                "/path/to/key",
				CaFile:                 "/path/to/ca",
				EnableHostVerification: true,
			},
		},
		{
			name: "PartialTLSFlagsSet",
			setupContext: func(app *cli.App) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.Bool(FlagEnableTLS, true, "Enable TLS flag")
				set.String(FlagTLSCertPath, "/path/to/cert", "TLS Cert Path")

				require.NoError(t, set.Set(FlagEnableTLS, "true"))
				require.NoError(t, set.Set(FlagTLSCertPath, "/path/to/cert"))

				return cli.NewContext(app, set, nil)
			},
			expectedTLS: config.TLS{
				Enabled:  true,
				CertFile: "/path/to/cert",
			},
		},
		{
			name: "NoTLSFlagsSet",
			setupContext: func(app *cli.App) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				return cli.NewContext(app, set, nil)
			},
			expectedTLS: config.TLS{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up app and context
			app := cli.NewApp()
			c := tt.setupContext(app)

			// Initialize an empty TLS config and apply overrideTLS
			tlsConfig := &config.TLS{}
			overrideTLS(c, tlsConfig)

			// Validate TLS config settings
			assert.Equal(t, tt.expectedTLS.Enabled, tlsConfig.Enabled)
			assert.Equal(t, tt.expectedTLS.CertFile, tlsConfig.CertFile)
			assert.Equal(t, tt.expectedTLS.KeyFile, tlsConfig.KeyFile)
			assert.Equal(t, tt.expectedTLS.CaFile, tlsConfig.CaFile)
			assert.Equal(t, tt.expectedTLS.EnableHostVerification, tlsConfig.EnableHostVerification)
		})
	}
}

func newClientFactoryMock() *clientFactoryMock {
	return &clientFactoryMock{
		serverFrontendClient: frontend.NewMockClient(gomock.NewController(nil)),
		serverAdminClient:    admin.NewMockClient(gomock.NewController(nil)),
		config: &config.Config{
			Persistence: config.Persistence{
				DefaultStore: "default",
				DataStores: map[string]config.DataStore{
					"default": {NoSQL: &config.NoSQL{PluginName: cassandra.PluginName}},
				},
			},
			ClusterGroupMetadata: &config.ClusterGroupMetadata{
				CurrentClusterName: "current-cluster",
			},
		},
	}
}
