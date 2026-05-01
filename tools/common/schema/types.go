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

package schema

import (
	"fmt"
	"io/fs"
	"regexp"
)

//go:generate mockgen -source $GOFILE -destination schema_client_mock.go -package schema github.com/uber/cadence/tools/common/schema SchemaClient

type (
	// ConfigError is an error type that
	// represents a problem with the config
	ConfigError struct {
		msg string
	}
	// UpdateConfig holds the config
	// params for executing a UpdateTask
	UpdateConfig struct {
		DBName        string
		TargetVersion string
		SchemaFS      fs.FS
		IsDryRun      bool
	}
	// SetupConfig holds the config
	// params need by the SetupTask
	SetupConfig struct {
		SchemaFilePath    string
		InitialVersion    string
		Overwrite         bool // overwrite previous data
		DisableVersioning bool // do not use schema versioning
	}
	// SchemaClient is the database interface that's required to be implemented
	// for the schema-tool to work
	SchemaClient interface {
		// ExecDDLQuery executes a schema statement
		ExecDDLQuery(stmt string, args ...interface{}) error
		// DropAllTables drops all tables
		DropAllTables() error
		// CreateSchemaVersionTables sets up the schema version tables
		CreateSchemaVersionTables() error
		// ReadSchemaVersion returns the current schema version for the keyspace
		ReadSchemaVersion() (string, error)
		// UpdateSchemaVersion updates the schema version for the keyspace
		UpdateSchemaVersion(newVersion string, minCompatibleVersion string) error
		// WriteSchemaUpdateLog adds an entry to the schema update history table
		WriteSchemaUpdateLog(oldVersion string, newVersion string, manifestMD5 string, desc string) error
		// Close gracefully closes the client object
		Close()
	}
)

const (
	// CLIOptEndpoint is the cli option for endpoint
	CLIOptEndpoint = "endpoint"
	// CLIOptPort is the cli option for port
	CLIOptPort = "port"
	// CLIOptUser is the cli option for user
	CLIOptUser = "user"
	// CLIOptPassword is the cli option for password
	CLIOptPassword = "password"
	// CLIOptAllowedAuthenticatorsis the cli option for cassandra allowed authenticators
	CLIOptAllowedAuthenticators = "allowed-authenticators"
	// CLIOptTimeout is the cli option for timeout
	CLIOptTimeout = "timeout"
	// CLIOptConnectTimeout is the cli option for connection timeout
	CLIOptConnectTimeout = "connect-timeout"
	// CLIOptKeyspace is the cli option for keyspace
	CLIOptKeyspace = "keyspace"
	// CLIOptDatabase is the cli option for datacenter
	CLIOptDatacenter = "datacenter"
	// CLIOptDatabase is the cli option for database
	CLIOptDatabase = "database"
	// CLIOptPluginName is the cli option for plugin name
	CLIOptPluginName = "plugin"
	// CLIOptConnectAttributes is the cli option for connect attributes (key/values via a url query string)
	CLIOptConnectAttributes = "connect-attributes"
	// CLIOptVersion is the cli option for version
	CLIOptVersion = "version"
	// CLIOptSchemaFile is the cli option for schema file
	CLIOptSchemaFile = "schema-file"
	// CLIOptOverwrite is the cli option for overwrite
	CLIOptOverwrite = "overwrite"
	// CLIOptDisableVersioning is the cli option to disabling versioning
	CLIOptDisableVersioning = "disable-versioning"
	// CLIOptTargetVersion is the cli option for target version
	CLIOptTargetVersion = "version"
	// CLIOptDryrun is the cli option for enabling dryrun
	CLIOptDryrun = "dryrun"
	// CLIOptSchemaDir is the cli option for schema directory
	CLIOptSchemaDir = "schema-dir"
	// CLIOptReplicationFactor is the cli option for replication factor
	CLIOptReplicationFactor = "replication-factor"
	// CLIOptQuiet is the cli option for quiet mode
	CLIOptQuiet = "quiet"
	// CLIOptProtoVersion is the cli option for protocol version
	CLIOptProtoVersion = "protocol-version"

	// CLIFlagEndpoint is the cli flag for endpoint
	CLIFlagEndpoint = CLIOptEndpoint
	// CLIFlagPort is the cli flag for port
	CLIFlagPort = CLIOptPort
	// CLIFlagUser is the cli flag for user
	CLIFlagUser = CLIOptUser
	// CLIFlagPassword is the cli flag for password
	CLIFlagPassword = CLIOptPassword
	// CLIFlagAllowedAuthenticators is the cli flag for whitelisting custom authenticators
	CLIFlagAllowedAuthenticators = CLIOptAllowedAuthenticators
	// CLIFlagConnectTimeout is the cli flag for connection timeout
	CLIFlagConnectTimeout = CLIOptConnectTimeout
	// CLIFlagTimeout is the cli flag for timeout
	CLIFlagTimeout = CLIOptTimeout
	// CLIFlagKeyspace is the cli flag for keyspace
	CLIFlagKeyspace = CLIOptKeyspace
	// CLIFlagDatacenter is the cli flag for datacenter
	CLIFlagDatacenter = CLIOptDatacenter
	// CLIFlagDatabase is the cli flag for database
	CLIFlagDatabase = CLIOptDatabase
	// CLIFlagPluginName is the cli flag for plugin name
	CLIFlagPluginName = CLIOptPluginName
	// CLIFlagConnectAttributes allows arbitrary connect attributes
	CLIFlagConnectAttributes = CLIOptConnectAttributes
	// CLIFlagVersion is the cli flag for version
	CLIFlagVersion = CLIOptVersion
	// CLIFlagSchemaFile is the cli flag for schema file
	CLIFlagSchemaFile = CLIOptSchemaFile
	// CLIFlagOverwrite is the cli flag for overwrite
	CLIFlagOverwrite = CLIOptOverwrite
	// CLIFlagDisableVersioning is the cli flag for disabling versioning
	CLIFlagDisableVersioning = CLIOptDisableVersioning
	// CLIFlagTargetVersion is the cli flag for target version
	CLIFlagTargetVersion = CLIOptTargetVersion
	// CLIFlagDryrun is the cli flag for dryrun
	CLIFlagDryrun = CLIOptDryrun
	// CLIFlagSchemaDir is the cli flag for schema directory
	CLIFlagSchemaDir = CLIOptSchemaDir
	// CLIFlagReplicationFactor is the cli flag for replication factor
	CLIFlagReplicationFactor = CLIOptReplicationFactor
	// CLIFlagQuiet is the cli flag for quiet mode
	CLIFlagQuiet = CLIOptQuiet
	// CLIFlagProtoVersion is the cli flag for protocol version
	CLIFlagProtoVersion = CLIOptProtoVersion

	// CLIFlagEnableTLS enables cassandra client TLS
	CLIFlagEnableTLS = "tls"
	// CLIFlagTLSCertFile is the optional tls cert file (tls must be enabled)
	CLIFlagTLSCertFile = "tls-cert-file"
	// CLIFlagTLSKeyFile is the optional tls key file (tls must be enabled)
	CLIFlagTLSKeyFile = "tls-key-file"
	// CLIFlagTLSCaFile is the optional tls CA file (tls must be enabled)
	CLIFlagTLSCaFile = "tls-ca-file"
	// CLIFlagTLSEnableHostVerification enables tls host verification (tls must be enabled)
	CLIFlagTLSEnableHostVerification = "tls-enable-host-verification"
	// CLIFlagTLSServerName is the Server Name Indication to verify the hostname on the returned certificates.
	// It is also included in the client's handshake to support virtual hosting unless it is an IP address.
	CLIFlagTLSServerName = "tls-server-name"
)

var rmspaceRegex = regexp.MustCompile(`\s+`)

// NewConfigError creates and returns an instance of ConfigError
func NewConfigError(msg string) error {
	return &ConfigError{msg: msg}
}

// Error returns a string representation of this error
func (e *ConfigError) Error() string {
	return fmt.Sprintf("Config Error:%v", e.msg)
}
