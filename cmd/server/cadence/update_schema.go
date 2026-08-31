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

package cadence

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"go.uber.org/zap"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	persistenceClient "github.com/uber/cadence/common/persistence/client"
)

// Amount of time allowed to connect to all DBs
const setupTimeout = 2 * time.Minute

// Backoff whenever we fail to connect to a DB
const setupRetryInterval = 3 * time.Second

type setupTask struct {
	pluginName string
	dbType     persistence.DBType
	identifier string
	setupDB    persistence.SetupDB
}

// schemaUpdateTask tracks a pending schema update and the SchemaDB it should be applied to.
type schemaUpdateTask struct {
	// pluginName/dbType groups updates so all DBs in the same group can be
	// advanced version-by-version together.
	pluginName string
	dbType     persistence.DBType
	identifier string
	schemaDB   persistence.SchemaDB
	update     *persistence.SchemaUpdate
}

// newPersistenceFactory constructs a persistence factory from the provided config. It doesn't support dynamic config
// or the more complex features of a full server.
func newPersistenceFactory(cfg config.Config, logger log.Logger) persistenceClient.Factory {
	dc := dynamicconfig.NewNopCollection()
	clusterName := ""
	if cfg.ClusterGroupMetadata != nil {
		clusterName = cfg.ClusterGroupMetadata.CurrentClusterName
	}
	return persistenceClient.NewFactory(
		&cfg.Persistence,
		func() float64 { return 0 },
		clusterName,
		nil, // no metrics client needed for schema updates
		logger,
		persistence.NewDynamicConfiguration(dc),
	)
}

// newUpdateSchemaLogger builds a simple zap-based logger suitable for the CLI update-schema command.
func newUpdateSchemaLogger() log.Logger {
	return log.NewLogger(zap.Must(zap.NewProduction()))
}

// runUpdateSchema applies all pending schema updates for every AdminDB instance returned
// by factory. It follows the steps below:
//
//  1. Within a 2-minute retry window, construct a SetupDB for every AdminDB. Any errors
//     are retried until the deadline.
//  2. For each SetupDB, call IsSetup; if the result is false, call Setup.
//  3. For each AdminDB that reports SupportsSchema() == true, create a SchemaDB, check
//     the current schema version against the latest, and collect any updates that need
//     to be applied.
//  4. Sort the collected updates by (PluginName/DBType, Version) and apply them in that order.
func runUpdateSchema(ctx context.Context, factory persistenceClient.Factory, logger log.Logger, timeSource clock.TimeSource) error {
	adminDBs, err := factory.NewAdminDBs()
	if err != nil {
		return fmt.Errorf("get admin DBs: %w", err)
	}

	setupTasks, setupDBs, err := connectToDBs(ctx, logger, timeSource, adminDBs)
	defer closeSetupDBs(setupDBs)
	if err != nil {
		return err
	}

	if err = ensureSetup(ctx, logger, setupTasks); err != nil {
		return err
	}

	schemaDBs, updates, err := collectSchemaUpdates(ctx, adminDBs)
	defer closeSchemaDBs(schemaDBs)
	if err != nil {
		return err
	}

	return applyUpdates(ctx, logger, updates)
}

// connectToDBs tries to create a SetupDB for every AdminDB within a 2-minute
// window, retrying on any error until the deadline is exceeded.
func connectToDBs(ctx context.Context, logger log.Logger, timeSource clock.TimeSource, adminDBs []persistence.AdminDB) ([]setupTask, []persistence.SetupDB, error) {
	tasks := make([]setupTask, 0, len(adminDBs))
	setupDBs := make([]persistence.SetupDB, 0, len(adminDBs))
	ctx, cancel := timeSource.ContextWithTimeout(ctx, setupTimeout)
	defer cancel()
	for _, adminDB := range adminDBs {
		setupDB, err := retryUntilConnected(ctx, logger, adminDB, timeSource)
		if err != nil {
			return tasks, setupDBs, err
		}
		tasks = append(tasks, setupTask{
			pluginName: adminDB.PluginName(),
			dbType:     adminDB.DBType(),
			identifier: adminDB.Identifier(),
			setupDB:    setupDB,
		})
		setupDBs = append(setupDBs, setupDB)
	}
	return tasks, setupDBs, nil
}

func retryUntilConnected(ctx context.Context, logger log.Logger, adminDB persistence.AdminDB, timeSource clock.TimeSource) (persistence.SetupDB, error) {
	var setupDB persistence.SetupDB
	for setupDB == nil {
		var err error
		setupDB, err = adminDB.CreateSetupDB()
		if err == nil && setupDB != nil {
			return setupDB, nil
		}
		logger.Warn("Failed to connect to database",
			tag.PersistencePluginName(adminDB.PluginName()),
			tag.PersistenceDBType(string(adminDB.DBType())),
			tag.PersistenceDBIdentifier(adminDB.Identifier()),
			tag.Error(err))
		if err = timeSource.SleepWithContext(ctx, setupRetryInterval); err != nil {
			return nil, err
		}
	}
	return setupDB, nil
}

// ensureSetup calls Setup on any SetupDB that reports IsSetup() == false.
func ensureSetup(ctx context.Context, logger log.Logger, tasks []setupTask) error {
	for _, t := range tasks {
		isSetup, err := t.setupDB.IsSetup(ctx)
		if err != nil {
			return fmt.Errorf("checking setup status for %s: %w", setupTarget(t), err)
		}
		if isSetup {
			logger.Info("Database already set up",
				tag.PersistencePluginName(t.pluginName),
				tag.PersistenceDBType(string(t.dbType)),
				tag.PersistenceDBIdentifier(t.identifier),
			)
			continue
		}
		logger.Info("Setting up database...",
			tag.PersistencePluginName(t.pluginName),
			tag.PersistenceDBType(string(t.dbType)),
			tag.PersistenceDBIdentifier(t.identifier),
		)
		if err = t.setupDB.Setup(ctx, nil); err != nil {
			return fmt.Errorf("setting up %s: %w", setupTarget(t), err)
		}
		logger.Info("Database set up successfully",
			tag.PersistencePluginName(t.pluginName),
			tag.PersistenceDBType(string(t.dbType)),
			tag.PersistenceDBIdentifier(t.identifier),
		)
	}
	return nil
}

// collectSchemaUpdates iterates over every AdminDB that supports schema management,
// creates a SchemaDB, ensures versioning tables exist, and returns all pending updates.
// The returned schemaDBs slice must be closed by the caller.
func collectSchemaUpdates(ctx context.Context, adminDBs []persistence.AdminDB) ([]persistence.SchemaDB, []schemaUpdateTask, error) {
	var schemaDBs []persistence.SchemaDB
	var updates []schemaUpdateTask

	for _, adminDB := range adminDBs {
		if !adminDB.SupportsSchema() {
			continue
		}
		adminDBID := adminDBIdentity(adminDB)

		schemaDB, err := adminDB.CreateSchemaDB()
		if err != nil {
			return schemaDBs, nil, fmt.Errorf("creating schema DB for %s: %w", adminDBID, err)
		}
		schemaDBs = append(schemaDBs, schemaDB)

		hasVersioning, err := schemaDB.HasSchemaVersioning(ctx)
		if err != nil {
			return schemaDBs, nil, fmt.Errorf("checking schema versioning for %s: %w", adminDBID, err)
		}
		if !hasVersioning {
			if err := schemaDB.SetupVersioning(ctx); err != nil {
				return schemaDBs, nil, fmt.Errorf("setting up schema versioning for %s: %w", adminDBID, err)
			}
			// Skip applying all the incremental updates, just apply one version that gets them to the latest
			skipToLatest, err := schemaDB.LatestSchema().SkipToLatest()
			if err != nil {
				return schemaDBs, nil, fmt.Errorf("failed reading latest schema for %s: %w", adminDBID, err)
			}
			updates = append(updates, schemaUpdateTask{
				pluginName: adminDB.PluginName(),
				dbType:     adminDB.DBType(),
				identifier: adminDB.Identifier(),
				schemaDB:   schemaDB,
				update:     skipToLatest,
			})
			continue
		}

		currentVersion, err := schemaDB.GetSchemaVersion(ctx)
		if err != nil {
			return schemaDBs, nil, fmt.Errorf("getting schema version for %s: %w", adminDBID, err)
		}

		latestSchema := schemaDB.LatestSchema()
		if !currentVersion.IsBefore(latestSchema.LatestVersion()) {
			// Already up to date, no action needed.
			continue
		}

		allUpdates, err := latestSchema.AllUpdates()
		if err != nil {
			return schemaDBs, nil, fmt.Errorf("listing schema updates for %s: %w", adminDBID, err)
		}

		for _, u := range allUpdates {
			if currentVersion.IsBefore(u.Version) {
				updates = append(updates, schemaUpdateTask{
					pluginName: adminDB.PluginName(),
					dbType:     adminDB.DBType(),
					identifier: adminDB.Identifier(),
					schemaDB:   schemaDB,
					update:     u,
				})
			}
		}
	}

	return schemaDBs, updates, nil
}

func applyUpdates(ctx context.Context, logger log.Logger, updates []schemaUpdateTask) error {
	// Sort by (PluginName, DBType, Version) so DBs in the same plugin/type group are
	// advanced together version-by-version.
	slices.SortStableFunc(updates, func(a, b schemaUpdateTask) int {
		if pluginCmp := cmp.Compare(a.pluginName, b.pluginName); pluginCmp != 0 {
			return pluginCmp
		}
		if dbTypeCmp := cmp.Compare(string(a.dbType), string(b.dbType)); dbTypeCmp != 0 {
			return dbTypeCmp
		}
		if versionCmp := a.update.Version.Compare(b.update.Version); versionCmp != 0 {
			return versionCmp
		}
		return cmp.Compare(a.identifier, b.identifier)
	})

	for _, entry := range updates {
		logger.Info(
			"Applying schema update...",
			tag.PersistencePluginName(entry.pluginName),
			tag.PersistenceDBType(string(entry.dbType)),
			tag.PersistenceDBIdentifier(entry.identifier),
			tag.SchemaUpdateVersion(entry.update.Version.String()),
		)
		if err := entry.schemaDB.UpdateSchema(ctx, entry.update); err != nil {
			return fmt.Errorf("failed applying schema update v%s to %s: %w", entry.update.Version, schemaUpdateTarget(entry), err)
		}
		logger.Info(
			"Applied schema update",
			tag.PersistencePluginName(entry.pluginName),
			tag.PersistenceDBType(string(entry.dbType)),
			tag.PersistenceDBIdentifier(entry.identifier),
			tag.SchemaUpdateVersion(entry.update.Version.String()),
		)
	}
	return nil
}

// These three helper functions are for error messages to make it clear which DB is having issues.

func adminDBIdentity(adminDB persistence.AdminDB) string {
	return fmt.Sprintf("%s/%s/%s", adminDB.PluginName(), adminDB.DBType(), adminDB.Identifier())
}

func setupTarget(t setupTask) string {
	return fmt.Sprintf("%s/%s/%s", t.pluginName, t.dbType, t.identifier)
}

func schemaUpdateTarget(entry schemaUpdateTask) string {
	return fmt.Sprintf("%s/%s/%s", entry.pluginName, entry.dbType, entry.identifier)
}

func closeSetupDBs(setupDBs []persistence.SetupDB) {
	for _, setupDB := range setupDBs {
		if setupDB != nil {
			setupDB.Close()
		}
	}
}

func closeSchemaDBs(sdbs []persistence.SchemaDB) {
	for _, sdb := range sdbs {
		if sdb != nil {
			sdb.Close()
		}
	}
}
