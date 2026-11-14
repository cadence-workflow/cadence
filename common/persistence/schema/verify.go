package schema

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/uber/cadence/common/persistence"
	persistenceClient "github.com/uber/cadence/common/persistence/client"
)

// upgradeHint is added to the verification error when a schema is behind, so operators
// know how to upgrade it.
const upgradeHint = "to upgrade, run update-schema with cadence-cassandra-tool or cadence-sql-tool for each outdated database, " +
	"see https://github.com/cadence-workflow/cadence/blob/master/docs/persistence.md#upgrading-the-schema"

// outdatedSchemaError reports a schema whose version is behind the latest one.
type outdatedSchemaError struct {
	current persistence.Version
	latest  persistence.Version
}

func (e *outdatedSchemaError) Error() string {
	return fmt.Sprintf("current schema version %v is before latest version %v", e.current, e.latest)
}

func Verify(ctx context.Context, opts Options) error {
	opts, err := withDefaults(opts)
	if err != nil {
		return err
	}
	factory := newPersistenceFactory(opts.ClusterName, opts.Config, opts.Logger)
	defer factory.Close()

	return checkSchemas(ctx, factory)
}

func checkSchemas(ctx context.Context, factory persistenceClient.Factory) error {
	adminDBs, err := factory.NewAdminDBs()
	if err != nil {
		return fmt.Errorf("get admin DBs: %w", err)
	}
	// Sort them just for deterministic output and to group by plugin name and DB type.
	slices.SortStableFunc(adminDBs, func(a, b persistence.AdminDB) int {
		return cmp.Compare(describeDB(a), describeDB(b))
	})
	var result []error
	for _, db := range adminDBs {
		dbError := checkDB(ctx, db)
		if dbError != nil {
			result = append(result, fmt.Errorf("%s: %w", describeDB(db), dbError))
		}
	}
	if len(result) > 0 {
		var outdated *outdatedSchemaError
		if errors.As(errors.Join(result...), &outdated) {
			result = append(result, errors.New(upgradeHint))
		}
		return errors.Join(result...)
	}
	return nil
}

func checkDB(ctx context.Context, db persistence.AdminDB) error {
	setupDB, err := db.CreateSetupDB()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer setupDB.Close()
	ok, err := setupDB.IsSetup(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if setup: %w", err)
	}
	if !ok {
		return fmt.Errorf("not setup")
	}
	// DB is setup and it doesn't support schema, so we're done
	if !db.SupportsSchema() {
		return nil
	}
	schemaDB, err := db.CreateSchemaDB()
	if err != nil {
		return fmt.Errorf("failed to create schema db: %w", err)
	}
	defer schemaDB.Close()
	ok, err = schemaDB.HasSchemaVersioning(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for schema versioning: %w", err)
	}
	if !ok {
		return fmt.Errorf("schema versioning is not setup")
	}
	latestVersion := schemaDB.LatestSchema().LatestVersion()
	currentVersion, err := schemaDB.GetSchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %w", err)
	}
	if currentVersion.IsBefore(latestVersion) {
		return &outdatedSchemaError{current: currentVersion, latest: latestVersion}
	}
	return nil
}
