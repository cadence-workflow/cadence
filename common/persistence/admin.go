package persistence

import (
	"cmp"
	"context"
	"fmt"
	"strconv"
	"strings"
)

//go:generate mockgen -package $GOPACKAGE -destination admin_mock.go -self_package github.com/uber/cadence/common/persistence github.com/uber/cadence/common/persistence AdminDB,SetupDB,SchemaDB,Schema

type DBType string

const (
	DBTypeDefault    DBType = "default"
	DBTypeVisibility DBType = "visibility"
)

type (
	// AdminDB represents the configuration required to connect to a backing persistence technology for admin operations
	// outside of general Cadence usage.
	AdminDB interface {
		// PluginName returns the persistence plugin name (for example, mysql or cassandra).
		PluginName() string
		// DBType returns whether this admin DB is for default or visibility persistence.
		DBType() DBType
		// Identifier returns an identifier that distinguishes this admin DB instance
		// from other admin DB instances of the same plugin/type.
		Identifier() string
		// CreateSetupDB establishes a connection to the database that can be used to create the necessary database/keyspaces
		// and any minimum required tables.
		CreateSetupDB() (SetupDB, error)
		// SupportsSchema returns whether this persistence technology supports schema management and versioning. If it
		// doesn't, then the setup operations performed by SetupDB should include all necessary steps to make the persistence
		// technology usable.
		SupportsSchema() bool
		// CreateSchemaDB establishes a connection to the specific database/keyspace for this persistence technology to
		// support schema management and versioning.
		CreateSchemaDB() (SchemaDB, error)
	}
	// SetupDB represents a generic connection to a specific persistence technology. It can create/drop databases/keyspaces/tables,
	// but doesn't represent a specific connection to a database/keyspace
	SetupDB interface {
		// IsSetup returns true if the backing database has the correct DB/keyspace
		IsSetup(ctx context.Context) (bool, error)
		// Setup creates the necessary DB/keyspace and minimum required tables.
		Setup(ctx context.Context, options map[string]string) error
		// Teardown deletes all tables and the used DB/keyspace.
		Teardown(ctx context.Context) error
		Close()
	}
	// SchemaDB represents a specific connection to a persistence technology and DB/keyspace that is tracked by schema
	// version management. It provides the latest schema metadata for this technology, along with the ability to inspect/update
	// the schema version
	SchemaDB interface {
		// LatestSchema returns metadata for the latest schema versions for this plugin
		LatestSchema() Schema

		// HasSchemaVersioning returns true if schema versioning has been applied to this DB at least once.
		HasSchemaVersioning(ctx context.Context) (bool, error)
		SetupVersioning(ctx context.Context) error

		// GetSchemaVersion returns the current schema version
		GetSchemaVersion(ctx context.Context) (Version, error)
		UpdateSchema(ctx context.Context, update *SchemaUpdate) error
		Close()
	}
	Schema interface {
		LatestVersion() Version
		// AllUpdates returns a sorted list of SchemaUpdates
		AllUpdates() ([]*SchemaUpdate, error)
		// SkipToLatest returns a SchemaUpdate that allows for installing just the latest schema version. It's ideal
		// for new installations
		SkipToLatest() (*SchemaUpdate, error)
	}
	SchemaUpdate struct {
		Version              Version
		MinCompatibleVersion Version
		DDLStatements        []string
		ManifestMD5          string
		Description          string
	}

	Version struct {
		Major, Minor int
	}
)

func ParseVersion(ver string) (Version, error) {
	vals := strings.Split(ver, ".")

	switch len(vals) {
	case 1:
		major, err := strconv.Atoi(vals[0])
		if err != nil {
			return Version{}, fmt.Errorf("invalid version: %w", err)
		}
		return Version{Major: major, Minor: 0}, nil
	case 2:
		major, err := strconv.Atoi(vals[0])
		if err != nil {
			return Version{}, fmt.Errorf("invalid major version: %w", err)
		}
		minor, err := strconv.Atoi(vals[1])
		if err != nil {
			return Version{}, fmt.Errorf("invalid minor version: %w", err)
		}
		return Version{Major: major, Minor: minor}, nil
	default:
		return Version{}, fmt.Errorf("invalid version: %s", ver)
	}
}

func (v Version) IsBefore(other Version) bool {
	return v.Compare(other) < 0
}

func (v Version) Compare(other Version) int {
	majorCmp := cmp.Compare(v.Major, other.Major)
	if majorCmp != 0 {
		return majorCmp
	}
	return cmp.Compare(v.Minor, other.Minor)
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}
