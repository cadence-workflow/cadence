package persistencetests

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	p "github.com/uber/cadence/common/persistence"
)

type (
	// AdminPersistenceSuite verifies the schema management functionality of the AdminDB, SetupDB, and SchemaDB
	// implementations for a persistence plugin
	AdminPersistenceSuite struct {
		*TestBase
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
	}
)

func (s *AdminPersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

func (s *AdminPersistenceSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
}

func (s *AdminPersistenceSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

func (s *AdminPersistenceSuite) TestSetupDBIsSetup() {
	s.forEachAdminDB(false, func(r *require.Assertions, adminDB p.AdminDB) {
		setupDB, err := adminDB.CreateSetupDB()
		r.NoError(err)
		defer setupDB.Close()

		isSetup, err := setupDB.IsSetup(s.T().Context())
		r.NoError(err)
		r.True(isSetup)
	})
}

// TestSchemaUpdates verifies that the schema is at the latest version and then simulates retrying schema updates that
// were partially applied, in both strict and resume mode. Both happen in a single test because reapplying updates permanently advances the schema
// version of the DBs shared by this suite.
func (s *AdminPersistenceSuite) TestSchemaUpdates() {
	s.forEachAdminDB(true, func(r *require.Assertions, adminDB p.AdminDB) {
		ctx := s.T().Context()
		schemaDB, err := adminDB.CreateSchemaDB()
		r.NoError(err)
		defer schemaDB.Close()

		hasVersioning, err := schemaDB.HasSchemaVersioning(ctx)
		r.NoError(err)
		r.True(hasVersioning)

		latest := schemaDB.LatestSchema().LatestVersion()
		version, err := schemaDB.GetSchemaVersion(ctx)
		r.NoError(err)
		r.Equal(latest, version)

		// Every DDL statement in SkipToLatest and in each incremental update has already been applied. Reapplying
		// all of them, in order, exercises every kind of DDL statement in the schema. Strict mode must reject
		// statements whose schema element already exists, while resume mode must skip them and finish the update.
		skipToLatest, err := schemaDB.LatestSchema().SkipToLatest()
		r.NoError(err)
		allUpdates, err := schemaDB.LatestSchema().AllUpdates()
		r.NoError(err)
		r.NotEmpty(allUpdates)
		updates := append([]*p.SchemaUpdate{skipToLatest}, allUpdates...)

		for i, update := range updates {
			r.NotEmpty(update.DDLStatements)
			// Each update must have a version greater than the current one, so renumber them after the latest
			duplicate := *update
			duplicate.Version = p.Version{Major: latest.Major, Minor: latest.Minor + i + 1}
			duplicate.MinCompatibleVersion = duplicate.Version
			duplicate.Description = fmt.Sprintf("Duplicate of v%s: %s", update.Version, update.Description)

			err = schemaDB.UpdateSchema(ctx, &duplicate, p.SchemaUpdateModeStrict)
			// Some updates only contain statements that can be safely reapplied, such as modifying a column type
			if err != nil {
				var duplicateErr *p.DuplicateSchemaElementError
				r.ErrorAs(err, &duplicateErr, "reapplying %s in strict mode", duplicate.Description)
				r.Contains(duplicate.DDLStatements, duplicateErr.Statement)

				version, err = schemaDB.GetSchemaVersion(ctx)
				r.NoError(err)
				r.True(version.IsBefore(duplicate.Version), "strict mode must not record the version after a failure")

				err = schemaDB.UpdateSchema(ctx, &duplicate, p.SchemaUpdateModeResume)
				r.NoError(err, "reapplying %s in resume mode", duplicate.Description)
			}

			version, err = schemaDB.GetSchemaVersion(ctx)
			r.NoError(err)
			r.Equal(duplicate.Version, version)
		}
	})
}

func (s *AdminPersistenceSuite) forEachAdminDB(requireSchema bool, fn func(r *require.Assertions, adminDB p.AdminDB)) {
	s.NotEmpty(s.AdminDBs)
	for _, adminDB := range s.AdminDBs {
		if requireSchema && !adminDB.SupportsSchema() {
			continue
		}
		name := fmt.Sprintf("%s/%s", adminDB.PluginName(), adminDB.DBType())
		s.Run(name, func() {
			// Assertions must be bound to the subtest, otherwise a failure calls FailNow on the parent test
			fn(require.New(s.T()), adminDB)
		})
	}
}
