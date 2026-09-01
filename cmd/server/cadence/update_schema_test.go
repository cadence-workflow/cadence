package cadence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	persistenceclient "github.com/uber/cadence/common/persistence/client"
)

type connectResult struct {
	tasks    []setupTask
	setupDBs []persistence.SetupDB
	err      error
}

func TestConnectToDBs(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, ctrl *gomock.Controller)
	}{
		{
			name: "success without retries",
			run: func(t *testing.T, ctrl *gomock.Controller) {
				logger := testlogger.New(t)
				timeSource := clock.NewMockedTimeSourceAt(time.Unix(0, 0))
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				setupDB := persistence.NewMockSetupDB(ctrl)

				adminDB.EXPECT().CreateSetupDB().Return(setupDB, nil)
				setupDB.EXPECT().Close()

				tasks, setupDBs, err := connectToDBs(context.Background(), logger, timeSource, []persistence.AdminDB{adminDB})
				require.NoError(t, err)
				require.Len(t, tasks, 1)
				require.Len(t, setupDBs, 1)
				assert.Same(t, adminDB, tasks[0].adminDB)
				assert.Same(t, setupDB, tasks[0].setupDB)
				assert.Same(t, setupDB, setupDBs[0])

				closeSetupDBs(setupDBs)
			},
		},
		{
			name: "retries until success",
			run: func(t *testing.T, ctrl *gomock.Controller) {
				logger := testlogger.New(t)
				timeSource := clock.NewMockedTimeSourceAt(time.Unix(0, 0))
				adminDB := newMockAdminDB(ctrl, "cassandra", persistence.DBTypeVisibility, "shard-a")
				setupDB := persistence.NewMockSetupDB(ctrl)
				connectErr := errors.New("connect failed")

				gomock.InOrder(
					adminDB.EXPECT().CreateSetupDB().Return(nil, connectErr),
					adminDB.EXPECT().CreateSetupDB().Return(setupDB, nil),
				)
				setupDB.EXPECT().Close()

				resultCh := make(chan connectResult, 1)
				go func() {
					tasks, setupDBs, err := connectToDBs(context.Background(), logger, timeSource, []persistence.AdminDB{adminDB})
					resultCh <- connectResult{tasks: tasks, setupDBs: setupDBs, err: err}
				}()

				timeSource.BlockUntil(2)
				timeSource.Advance(setupRetryInterval)

				res := waitForConnectResult(t, resultCh)
				require.NoError(t, res.err)
				require.Len(t, res.tasks, 1)
				require.Len(t, res.setupDBs, 1)

				closeSetupDBs(res.setupDBs)
			},
		},
		{
			name: "returns context deadline exceeded when retries exhaust timeout",
			run: func(t *testing.T, ctrl *gomock.Controller) {
				logger := testlogger.New(t)
				timeSource := clock.NewMockedTimeSourceAt(time.Unix(0, 0))
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-timeout")
				adminDB.EXPECT().CreateSetupDB().Return(nil, errors.New("still down")).AnyTimes()

				resultCh := make(chan connectResult, 1)
				go func() {
					tasks, setupDBs, err := connectToDBs(context.Background(), logger, timeSource, []persistence.AdminDB{adminDB})
					resultCh <- connectResult{tasks: tasks, setupDBs: setupDBs, err: err}
				}()

				timeSource.BlockUntil(2)
				timeSource.Advance(setupTimeout + setupRetryInterval)

				res := waitForConnectResult(t, resultCh)
				require.ErrorIs(t, res.err, context.DeadlineExceeded)
				assert.Empty(t, res.tasks)
				assert.Empty(t, res.setupDBs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tt.run(t, ctrl)
		})
	}
}

func TestEnsureSetup(t *testing.T) {
	tests := []struct {
		name         string
		isSetup      bool
		isSetupErr   error
		setupErr     error
		wantErr      string
		wantMessages []string
	}{
		{
			name:         "already setup",
			isSetup:      true,
			wantMessages: []string{"Database already set up"},
		},
		{
			name:         "setup required succeeds",
			isSetup:      false,
			wantMessages: []string{"Setting up database...", "Database set up successfully"},
		},
		{
			name:       "is setup check fails",
			isSetupErr: errors.New("is setup failed"),
			wantErr:    "checking setup status",
		},
		{
			name:         "setup fails",
			isSetup:      false,
			setupErr:     errors.New("setup failed"),
			wantErr:      "setting up mysql/default/db-1",
			wantMessages: []string{"Setting up database..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			logger := testlogger.New(t)
			setupDB := persistence.NewMockSetupDB(ctrl)

			setupDB.EXPECT().IsSetup(gomock.Any()).Return(tt.isSetup, tt.isSetupErr)
			if tt.isSetupErr == nil && !tt.isSetup {
				setupDB.EXPECT().Setup(gomock.Any(), nil).Return(tt.setupErr)
			}

			tasks := []setupTask{{
				adminDB: newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1"),
				setupDB: setupDB,
			}}

			err := ensureSetup(context.Background(), logger, tasks)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCollectSchemaUpdates(t *testing.T) {
	updateV1 := &persistence.SchemaUpdate{Version: persistence.Version{Major: 1, Minor: 0}}
	updateV2 := &persistence.SchemaUpdate{Version: persistence.Version{Major: 2, Minor: 0}}
	updateV3 := &persistence.SchemaUpdate{Version: persistence.Version{Major: 3, Minor: 0}}

	tests := []struct {
		name            string
		setup           func(*gomock.Controller) []persistence.AdminDB
		wantErrContains string
		wantSchemaDBs   int
		expectedUpdates []schemaUpdateTask
	}{
		{
			name: "skips admin dbs without schema support",
			setup: func(ctrl *gomock.Controller) []persistence.AdminDB {
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				adminDB.EXPECT().SupportsSchema().Return(false)
				return []persistence.AdminDB{adminDB}
			},
		},
		{
			name: "create schema db error",
			setup: func(ctrl *gomock.Controller) []persistence.AdminDB {
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				adminDB.EXPECT().SupportsSchema().Return(true)
				adminDB.EXPECT().CreateSchemaDB().Return(nil, errors.New("create schema db failed"))
				return []persistence.AdminDB{adminDB}
			},
			wantErrContains: "creating schema DB",
		},
		{
			name: "has schema versioning error returns opened schema dbs",
			setup: func(ctrl *gomock.Controller) []persistence.AdminDB {
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				schemaDB := persistence.NewMockSchemaDB(ctrl)
				adminDB.EXPECT().SupportsSchema().Return(true)
				adminDB.EXPECT().CreateSchemaDB().Return(schemaDB, nil)
				schemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(false, errors.New("has versioning failed"))
				schemaDB.EXPECT().Close()
				return []persistence.AdminDB{adminDB}
			},
			wantErrContains: "checking schema versioning",
			wantSchemaDBs:   1,
		},
		{
			name: "setup versioning error",
			setup: func(ctrl *gomock.Controller) []persistence.AdminDB {
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				schemaDB := persistence.NewMockSchemaDB(ctrl)
				adminDB.EXPECT().SupportsSchema().Return(true)
				adminDB.EXPECT().CreateSchemaDB().Return(schemaDB, nil)
				schemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(false, nil)
				schemaDB.EXPECT().SetupVersioning(gomock.Any()).Return(errors.New("setup versioning failed"))
				schemaDB.EXPECT().Close()
				return []persistence.AdminDB{adminDB}
			},
			wantErrContains: "setting up schema versioning",
			wantSchemaDBs:   1,
		},
		{
			name: "skip to latest error",
			setup: func(ctrl *gomock.Controller) []persistence.AdminDB {
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				schemaDB := persistence.NewMockSchemaDB(ctrl)
				schema := persistence.NewMockSchema(ctrl)
				adminDB.EXPECT().SupportsSchema().Return(true)
				adminDB.EXPECT().CreateSchemaDB().Return(schemaDB, nil)
				schemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(false, nil)
				schemaDB.EXPECT().SetupVersioning(gomock.Any()).Return(nil)
				schemaDB.EXPECT().LatestSchema().Return(schema)
				schema.EXPECT().SkipToLatest().Return(nil, errors.New("skip to latest failed"))
				schemaDB.EXPECT().Close()
				return []persistence.AdminDB{adminDB}
			},
			wantErrContains: "failed reading latest schema",
			wantSchemaDBs:   1,
		},
		{
			name: "no versioning returns skip to latest update",
			setup: func(ctrl *gomock.Controller) []persistence.AdminDB {
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				schemaDB := persistence.NewMockSchemaDB(ctrl)
				schema := persistence.NewMockSchema(ctrl)
				adminDB.EXPECT().SupportsSchema().Return(true)
				adminDB.EXPECT().CreateSchemaDB().Return(schemaDB, nil)
				schemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(false, nil)
				schemaDB.EXPECT().SetupVersioning(gomock.Any()).Return(nil)
				schemaDB.EXPECT().LatestSchema().Return(schema)
				schema.EXPECT().SkipToLatest().Return(updateV3, nil)
				schemaDB.EXPECT().Close()
				return []persistence.AdminDB{adminDB}
			},
			wantSchemaDBs: 1,
			expectedUpdates: []schemaUpdateTask{{
				update: updateV3,
			}},
		},
		{
			name: "already up to date",
			setup: func(ctrl *gomock.Controller) []persistence.AdminDB {
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				schemaDB := persistence.NewMockSchemaDB(ctrl)
				schema := persistence.NewMockSchema(ctrl)
				adminDB.EXPECT().SupportsSchema().Return(true)
				adminDB.EXPECT().CreateSchemaDB().Return(schemaDB, nil)
				schemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil)
				schemaDB.EXPECT().GetSchemaVersion(gomock.Any()).Return(updateV3.Version, nil)
				schemaDB.EXPECT().LatestSchema().Return(schema)
				schema.EXPECT().LatestVersion().Return(updateV3.Version)
				schemaDB.EXPECT().Close()
				return []persistence.AdminDB{adminDB}
			},
			wantSchemaDBs:   1,
			expectedUpdates: []schemaUpdateTask{},
		},
		{
			name: "get schema version error",
			setup: func(ctrl *gomock.Controller) []persistence.AdminDB {
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				schemaDB := persistence.NewMockSchemaDB(ctrl)
				adminDB.EXPECT().SupportsSchema().Return(true)
				adminDB.EXPECT().CreateSchemaDB().Return(schemaDB, nil)
				schemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil)
				schemaDB.EXPECT().GetSchemaVersion(gomock.Any()).Return(persistence.Version{}, errors.New("get version failed"))
				schemaDB.EXPECT().Close()
				return []persistence.AdminDB{adminDB}
			},
			wantErrContains: "getting schema version",
			wantSchemaDBs:   1,
		},
		{
			name: "all updates error",
			setup: func(ctrl *gomock.Controller) []persistence.AdminDB {
				adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				schemaDB := persistence.NewMockSchemaDB(ctrl)
				schema := persistence.NewMockSchema(ctrl)
				adminDB.EXPECT().SupportsSchema().Return(true)
				adminDB.EXPECT().CreateSchemaDB().Return(schemaDB, nil)
				schemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil)
				schemaDB.EXPECT().GetSchemaVersion(gomock.Any()).Return(persistence.Version{Major: 1, Minor: 0}, nil)
				schemaDB.EXPECT().LatestSchema().Return(schema)
				schema.EXPECT().LatestVersion().Return(persistence.Version{Major: 2, Minor: 0})
				schema.EXPECT().AllUpdates().Return(nil, errors.New("all updates failed"))
				schemaDB.EXPECT().Close()
				return []persistence.AdminDB{adminDB}
			},
			wantErrContains: "listing schema updates",
			wantSchemaDBs:   1,
		},
		{
			name: "filters updates newer than current version",
			setup: func(ctrl *gomock.Controller) []persistence.AdminDB {
				adminDB := newMockAdminDB(ctrl, "cassandra", persistence.DBTypeVisibility, "shard-a")
				schemaDB := persistence.NewMockSchemaDB(ctrl)
				schema := persistence.NewMockSchema(ctrl)
				adminDB.EXPECT().SupportsSchema().Return(true)
				adminDB.EXPECT().CreateSchemaDB().Return(schemaDB, nil)
				schemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil)
				schemaDB.EXPECT().GetSchemaVersion(gomock.Any()).Return(persistence.Version{Major: 1, Minor: 0}, nil)
				schemaDB.EXPECT().LatestSchema().Return(schema)
				schema.EXPECT().LatestVersion().Return(persistence.Version{Major: 3, Minor: 0})
				schema.EXPECT().AllUpdates().Return([]*persistence.SchemaUpdate{
					updateV1,
					updateV2,
					updateV3,
				}, nil)
				schemaDB.EXPECT().Close()
				return []persistence.AdminDB{adminDB}
			},
			wantSchemaDBs: 1,
			expectedUpdates: []schemaUpdateTask{
				{update: updateV2},
				{update: updateV3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			adminDBs := tt.setup(ctrl)

			schemaDBs, updates, err := collectSchemaUpdates(context.Background(), adminDBs)
			if tt.wantErrContains != "" {
				defer closeSchemaDBs(schemaDBs)
				require.ErrorContains(t, err, tt.wantErrContains)
				require.Len(t, schemaDBs, tt.wantSchemaDBs)
				assert.Nil(t, updates)
				return
			}

			defer closeSchemaDBs(schemaDBs)
			require.NoError(t, err)
			require.Len(t, schemaDBs, tt.wantSchemaDBs)
			require.Len(t, updates, len(tt.expectedUpdates))
			for i, want := range tt.expectedUpdates {
				assert.Same(t, want.update, updates[i].update)
			}
		})
	}
}

func TestApplyUpdates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		updates         func(*gomock.Controller, *[]string) []schemaUpdateTask
		wantErrContains string
		expected        []string
	}{
		{
			name: "sorts and applies updates",
			updates: func(ctrl *gomock.Controller, actual *[]string) []schemaUpdateTask {
				db1 := persistence.NewMockSchemaDB(ctrl)
				db2 := persistence.NewMockSchemaDB(ctrl)
				db3 := persistence.NewMockSchemaDB(ctrl)
				v1 := &persistence.SchemaUpdate{Version: persistence.Version{Major: 1, Minor: 0}}
				v2 := &persistence.SchemaUpdate{Version: persistence.Version{Major: 2, Minor: 0}}
				db1.EXPECT().UpdateSchema(gomock.Any(), v2).DoAndReturn(func(context.Context, *persistence.SchemaUpdate) error {
					*actual = append(*actual, "b/default/id-2@2.0")
					return nil
				})
				db2.EXPECT().UpdateSchema(gomock.Any(), v2).DoAndReturn(func(context.Context, *persistence.SchemaUpdate) error {
					*actual = append(*actual, "a/visibility/id-1@2.0")
					return nil
				})
				gomock.InOrder(
					db3.EXPECT().UpdateSchema(gomock.Any(), v1).DoAndReturn(func(context.Context, *persistence.SchemaUpdate) error {
						*actual = append(*actual, "a/default/id-0@1.0")
						return nil
					}),
					db3.EXPECT().UpdateSchema(gomock.Any(), v2).DoAndReturn(func(context.Context, *persistence.SchemaUpdate) error {
						*actual = append(*actual, "a/default/id-0@2.0")
						return nil
					}),
				)

				return []schemaUpdateTask{
					// This is the reverse of the expected order
					// Plugin B
					{adminDB: newMockAdminDB(ctrl, "b", persistence.DBTypeDefault, "id-2"), schemaDB: db1, update: v2},
					// Plugin A, Visibility DB, update to v2
					{adminDB: newMockAdminDB(ctrl, "a", persistence.DBTypeVisibility, "id-1"), schemaDB: db2, update: v2},
					// Plugin A, Default DB, update to v1, then v2
					{adminDB: newMockAdminDB(ctrl, "a", persistence.DBTypeDefault, "id-0"), schemaDB: db3, update: v2},
					{adminDB: newMockAdminDB(ctrl, "a", persistence.DBTypeDefault, "id-0"), schemaDB: db3, update: v1},
				}
			},
			expected: []string{
				"a/default/id-0@1.0",
				"a/default/id-0@2.0",
				"a/visibility/id-1@2.0",
				"b/default/id-2@2.0",
			},
		},
		{
			name: "stops on first update error",
			updates: func(ctrl *gomock.Controller, order *[]string) []schemaUpdateTask {
				db1 := persistence.NewMockSchemaDB(ctrl)
				db2 := persistence.NewMockSchemaDB(ctrl)
				v1 := &persistence.SchemaUpdate{Version: persistence.Version{Major: 1, Minor: 0}}
				v2 := &persistence.SchemaUpdate{Version: persistence.Version{Major: 2, Minor: 0}}
				db1.EXPECT().UpdateSchema(gomock.Any(), v1).DoAndReturn(func(context.Context, *persistence.SchemaUpdate) error {
					*order = append(*order, "a/default/id-0@1.0")
					return errors.New("update failed")
				})
				return []schemaUpdateTask{
					{adminDB: newMockAdminDB(ctrl, "b", persistence.DBTypeDefault, "id-2"), schemaDB: db2, update: v2},
					{adminDB: newMockAdminDB(ctrl, "a", persistence.DBTypeDefault, "id-0"), schemaDB: db1, update: v1},
				}
			},
			wantErrContains: "failed applying schema update v1.0",
			expected:        []string{"a/default/id-0@1.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			logger := testlogger.New(t)
			var actual []string

			err := applyUpdates(context.Background(), logger, tt.updates(ctrl, &actual))
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestRunUpdateSchema(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := testlogger.New(t)
	timeSource := clock.NewMockedTimeSourceAt(time.Unix(0, 0))
	factory := persistenceclient.NewMockFactory(ctrl)
	adminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
	setupDB := persistence.NewMockSetupDB(ctrl)
	schemaDB := persistence.NewMockSchemaDB(ctrl)
	schema := persistence.NewMockSchema(ctrl)
	update := &persistence.SchemaUpdate{Version: persistence.Version{Major: 2, Minor: 0}}

	// Happy path e2e

	factory.EXPECT().NewAdminDBs().Return([]persistence.AdminDB{adminDB}, nil)
	// Setup
	adminDB.EXPECT().CreateSetupDB().Return(setupDB, nil)
	setupDB.EXPECT().IsSetup(gomock.Any()).Return(false, nil)
	setupDB.EXPECT().Setup(gomock.Any(), nil).Return(nil)
	// Schema planning
	adminDB.EXPECT().SupportsSchema().Return(true)
	adminDB.EXPECT().CreateSchemaDB().Return(schemaDB, nil)
	schemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil)
	schemaDB.EXPECT().GetSchemaVersion(gomock.Any()).Return(persistence.Version{Major: 1, Minor: 0}, nil)
	schemaDB.EXPECT().LatestSchema().Return(schema)
	schema.EXPECT().LatestVersion().Return(persistence.Version{Major: 2, Minor: 0})
	schema.EXPECT().AllUpdates().Return([]*persistence.SchemaUpdate{update}, nil)
	// Schema execution
	schemaDB.EXPECT().UpdateSchema(gomock.Any(), update).Return(nil)
	setupDB.EXPECT().Close()
	schemaDB.EXPECT().Close()

	err := runUpdateSchema(context.Background(), factory, logger, timeSource)
	require.NoError(t, err)
}

func newMockAdminDB(ctrl *gomock.Controller, pluginName string, dbType persistence.DBType, identifier string) *persistence.MockAdminDB {
	adminDB := persistence.NewMockAdminDB(ctrl)
	adminDB.EXPECT().PluginName().Return(pluginName).AnyTimes()
	adminDB.EXPECT().DBType().Return(dbType).AnyTimes()
	adminDB.EXPECT().Identifier().Return(identifier).AnyTimes()
	return adminDB
}

func waitForConnectResult(t *testing.T, resultCh <-chan connectResult) connectResult {
	t.Helper()
	select {
	case res := <-resultCh:
		return res
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for connectToDBs to return")
		return connectResult{}
	}
}
