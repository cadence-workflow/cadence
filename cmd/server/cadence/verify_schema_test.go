package cadence

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	persistenceClient "github.com/uber/cadence/common/persistence/client"
)

func TestCheckDB(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*gomock.Controller) persistence.AdminDB
		expectedError string
	}{
		{
			name: "create setup db fails",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockAdminDB.EXPECT().CreateSetupDB().Return(nil, errors.New("connection failed"))
				return mockAdminDB
			},
			expectedError: "failed to connect",
		},
		{
			name: "is setup check fails",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockSetupDB := persistence.NewMockSetupDB(ctrl)
				gomock.InOrder(
					mockAdminDB.EXPECT().CreateSetupDB().Return(mockSetupDB, nil),
					mockSetupDB.EXPECT().IsSetup(gomock.Any()).Return(false, errors.New("connection error")),
					mockSetupDB.EXPECT().Close(),
				)
				return mockAdminDB
			},
			expectedError: "failed to check if setup",
		},
		{
			name: "database not setup",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockSetupDB := persistence.NewMockSetupDB(ctrl)
				gomock.InOrder(
					mockAdminDB.EXPECT().CreateSetupDB().Return(mockSetupDB, nil),
					mockSetupDB.EXPECT().IsSetup(gomock.Any()).Return(false, nil),
					mockSetupDB.EXPECT().Close(),
				)
				return mockAdminDB
			},
			expectedError: "not setup",
		},
		{
			name: "database does not support schema",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockSetupDB := persistence.NewMockSetupDB(ctrl)
				gomock.InOrder(
					mockAdminDB.EXPECT().CreateSetupDB().Return(mockSetupDB, nil),
					mockSetupDB.EXPECT().IsSetup(gomock.Any()).Return(true, nil),
					mockAdminDB.EXPECT().SupportsSchema().Return(false),
					mockSetupDB.EXPECT().Close(),
				)
				return mockAdminDB
			},
			expectedError: "",
		},
		{
			name: "create schema db fails",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockSetupDB := persistence.NewMockSetupDB(ctrl)
				gomock.InOrder(
					mockAdminDB.EXPECT().CreateSetupDB().Return(mockSetupDB, nil),
					mockSetupDB.EXPECT().IsSetup(gomock.Any()).Return(true, nil),
					mockAdminDB.EXPECT().SupportsSchema().Return(true),
					mockAdminDB.EXPECT().CreateSchemaDB().Return(nil, errors.New("db creation failed")),
					mockSetupDB.EXPECT().Close(),
				)
				return mockAdminDB
			},
			expectedError: "failed to create schema db",
		},
		{
			name: "check schema versioning fails",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockSetupDB := persistence.NewMockSetupDB(ctrl)
				mockSchemaDB := persistence.NewMockSchemaDB(ctrl)
				gomock.InOrder(
					mockAdminDB.EXPECT().CreateSetupDB().Return(mockSetupDB, nil),
					mockSetupDB.EXPECT().IsSetup(gomock.Any()).Return(true, nil),
					mockAdminDB.EXPECT().SupportsSchema().Return(true),
					mockAdminDB.EXPECT().CreateSchemaDB().Return(mockSchemaDB, nil),
					mockSchemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(false, errors.New("query failed")),
					mockSchemaDB.EXPECT().Close(),
					mockSetupDB.EXPECT().Close(),
				)
				return mockAdminDB
			},
			expectedError: "failed to check for schema versioning",
		},
		{
			name: "schema versioning not setup",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockSetupDB := persistence.NewMockSetupDB(ctrl)
				mockSchemaDB := persistence.NewMockSchemaDB(ctrl)
				gomock.InOrder(
					mockAdminDB.EXPECT().CreateSetupDB().Return(mockSetupDB, nil),
					mockSetupDB.EXPECT().IsSetup(gomock.Any()).Return(true, nil),
					mockAdminDB.EXPECT().SupportsSchema().Return(true),
					mockAdminDB.EXPECT().CreateSchemaDB().Return(mockSchemaDB, nil),
					mockSchemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(false, nil),
					mockSchemaDB.EXPECT().Close(),
					mockSetupDB.EXPECT().Close(),
				)
				return mockAdminDB
			},
			expectedError: "schema versioning is not setup",
		},
		{
			name: "get current schema version fails",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockSetupDB := persistence.NewMockSetupDB(ctrl)
				mockSchemaDB := persistence.NewMockSchemaDB(ctrl)
				mockSchema := persistence.NewMockSchema(ctrl)
				gomock.InOrder(
					mockAdminDB.EXPECT().CreateSetupDB().Return(mockSetupDB, nil),
					mockSetupDB.EXPECT().IsSetup(gomock.Any()).Return(true, nil),
					mockAdminDB.EXPECT().SupportsSchema().Return(true),
					mockAdminDB.EXPECT().CreateSchemaDB().Return(mockSchemaDB, nil),
					mockSchemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil),
					mockSchemaDB.EXPECT().LatestSchema().Return(mockSchema),
					mockSchema.EXPECT().LatestVersion().Return(persistence.Version{Major: 1, Minor: 0}),
					mockSchemaDB.EXPECT().GetSchemaVersion(gomock.Any()).Return(persistence.Version{}, errors.New("version query failed")),
					mockSchemaDB.EXPECT().Close(),
					mockSetupDB.EXPECT().Close(),
				)
				return mockAdminDB
			},
			expectedError: "failed to get current schema version",
		},
		{
			name: "current version before latest version",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockSetupDB := persistence.NewMockSetupDB(ctrl)
				mockSchemaDB := persistence.NewMockSchemaDB(ctrl)
				mockSchema := persistence.NewMockSchema(ctrl)
				gomock.InOrder(
					mockAdminDB.EXPECT().CreateSetupDB().Return(mockSetupDB, nil),
					mockSetupDB.EXPECT().IsSetup(gomock.Any()).Return(true, nil),
					mockAdminDB.EXPECT().SupportsSchema().Return(true),
					mockAdminDB.EXPECT().CreateSchemaDB().Return(mockSchemaDB, nil),
					mockSchemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil),
					mockSchemaDB.EXPECT().LatestSchema().Return(mockSchema),
					mockSchema.EXPECT().LatestVersion().Return(persistence.Version{Major: 2, Minor: 0}),
					mockSchemaDB.EXPECT().GetSchemaVersion(gomock.Any()).Return(persistence.Version{Major: 1, Minor: 5}, nil),
					mockSchemaDB.EXPECT().Close(),
					mockSetupDB.EXPECT().Close(),
				)
				return mockAdminDB
			},
			expectedError: "current schema version 1.5 is before latest version 2.0",
		},
		{
			name: "schema version matches",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockSetupDB := persistence.NewMockSetupDB(ctrl)
				mockSchemaDB := persistence.NewMockSchemaDB(ctrl)
				mockSchema := persistence.NewMockSchema(ctrl)
				gomock.InOrder(
					mockAdminDB.EXPECT().CreateSetupDB().Return(mockSetupDB, nil),
					mockSetupDB.EXPECT().IsSetup(gomock.Any()).Return(true, nil),
					mockAdminDB.EXPECT().SupportsSchema().Return(true),
					mockAdminDB.EXPECT().CreateSchemaDB().Return(mockSchemaDB, nil),
					mockSchemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil),
					mockSchemaDB.EXPECT().LatestSchema().Return(mockSchema),
					mockSchema.EXPECT().LatestVersion().Return(persistence.Version{Major: 2, Minor: 0}),
					mockSchemaDB.EXPECT().GetSchemaVersion(gomock.Any()).Return(persistence.Version{Major: 2, Minor: 0}, nil),
					mockSchemaDB.EXPECT().Close(),
					mockSetupDB.EXPECT().Close(),
				)
				return mockAdminDB
			},
			expectedError: "",
		},
		{
			name: "schema version newer than latest",
			setup: func(ctrl *gomock.Controller) persistence.AdminDB {
				mockAdminDB := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockSetupDB := persistence.NewMockSetupDB(ctrl)
				mockSchemaDB := persistence.NewMockSchemaDB(ctrl)
				mockSchema := persistence.NewMockSchema(ctrl)
				gomock.InOrder(
					mockAdminDB.EXPECT().CreateSetupDB().Return(mockSetupDB, nil),
					mockSetupDB.EXPECT().IsSetup(gomock.Any()).Return(true, nil),
					mockAdminDB.EXPECT().SupportsSchema().Return(true),
					mockAdminDB.EXPECT().CreateSchemaDB().Return(mockSchemaDB, nil),
					mockSchemaDB.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil),
					mockSchemaDB.EXPECT().LatestSchema().Return(mockSchema),
					mockSchema.EXPECT().LatestVersion().Return(persistence.Version{Major: 2, Minor: 0}),
					mockSchemaDB.EXPECT().GetSchemaVersion(gomock.Any()).Return(persistence.Version{Major: 2, Minor: 5}, nil),
					mockSchemaDB.EXPECT().Close(),
					mockSetupDB.EXPECT().Close(),
				)
				return mockAdminDB
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAdminDB := tt.setup(ctrl)
			err := checkDB(context.Background(), mockAdminDB)

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckSchemas(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*gomock.Controller) persistenceClient.Factory
		expectedErrors []string // substrings expected in the error message
	}{
		{
			name: "factory.NewAdminDBs fails",
			setup: func(ctrl *gomock.Controller) persistenceClient.Factory {
				mockFactory := persistenceClient.NewMockFactory(ctrl)
				mockFactory.EXPECT().NewAdminDBs().Return(nil, errors.New("db connection failed"))
				return mockFactory
			},
			expectedErrors: []string{"get admin DBs"},
		},
		{
			name: "all databases succeed",
			setup: func(ctrl *gomock.Controller) persistenceClient.Factory {
				mockFactory := persistenceClient.NewMockFactory(ctrl)
				mockAdminDB1 := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockAdminDB2 := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-2")
				mockSetupDB1 := persistence.NewMockSetupDB(ctrl)
				mockSetupDB2 := persistence.NewMockSetupDB(ctrl)
				mockSchemaDB1 := persistence.NewMockSchemaDB(ctrl)
				mockSchemaDB2 := persistence.NewMockSchemaDB(ctrl)
				mockSchema1 := persistence.NewMockSchema(ctrl)
				mockSchema2 := persistence.NewMockSchema(ctrl)

				gomock.InOrder(
					mockFactory.EXPECT().NewAdminDBs().Return([]persistence.AdminDB{mockAdminDB1, mockAdminDB2}, nil),
					// DB 1
					mockAdminDB1.EXPECT().CreateSetupDB().Return(mockSetupDB1, nil),
					mockSetupDB1.EXPECT().IsSetup(gomock.Any()).Return(true, nil),
					mockAdminDB1.EXPECT().SupportsSchema().Return(true),
					mockAdminDB1.EXPECT().CreateSchemaDB().Return(mockSchemaDB1, nil),
					mockSchemaDB1.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil),
					mockSchemaDB1.EXPECT().LatestSchema().Return(mockSchema1),
					mockSchema1.EXPECT().LatestVersion().Return(persistence.Version{Major: 1, Minor: 0}),
					mockSchemaDB1.EXPECT().GetSchemaVersion(gomock.Any()).Return(persistence.Version{Major: 1, Minor: 0}, nil),
					mockSchemaDB1.EXPECT().Close(),
					mockSetupDB1.EXPECT().Close(),
					// DB 2
					mockAdminDB2.EXPECT().CreateSetupDB().Return(mockSetupDB2, nil),
					mockSetupDB2.EXPECT().IsSetup(gomock.Any()).Return(true, nil),
					mockAdminDB2.EXPECT().SupportsSchema().Return(true),
					mockAdminDB2.EXPECT().CreateSchemaDB().Return(mockSchemaDB2, nil),
					mockSchemaDB2.EXPECT().HasSchemaVersioning(gomock.Any()).Return(true, nil),
					mockSchemaDB2.EXPECT().LatestSchema().Return(mockSchema2),
					mockSchema2.EXPECT().LatestVersion().Return(persistence.Version{Major: 1, Minor: 0}),
					mockSchemaDB2.EXPECT().GetSchemaVersion(gomock.Any()).Return(persistence.Version{Major: 1, Minor: 0}, nil),
					mockSchemaDB2.EXPECT().Close(),
					mockSetupDB2.EXPECT().Close(),
				)
				return mockFactory
			},
		},
		{
			name: "all databases fail",
			setup: func(ctrl *gomock.Controller) persistenceClient.Factory {
				mockFactory := persistenceClient.NewMockFactory(ctrl)
				mockAdminDB1 := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockAdminDB2 := newMockAdminDB(ctrl, "cassandra", persistence.DBTypeVisibility, "db-2")

				gomock.InOrder(
					mockFactory.EXPECT().NewAdminDBs().Return([]persistence.AdminDB{mockAdminDB1, mockAdminDB2}, nil),
					// Sorting by plugin name puts cassandra first
					mockAdminDB2.EXPECT().CreateSetupDB().Return(nil, errors.New("connection failed")),
					mockAdminDB1.EXPECT().CreateSetupDB().Return(nil, errors.New("connection failed")),
				)
				return mockFactory
			},
			expectedErrors: []string{
				"cassandra/visibility/db-2: failed to connect",
				"mysql/default/db-1: failed to connect",
			},
		},
		{
			name: "only some databases fail",
			setup: func(ctrl *gomock.Controller) persistenceClient.Factory {
				mockFactory := persistenceClient.NewMockFactory(ctrl)
				mockAdminDB1 := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-1")
				mockAdminDB2 := newMockAdminDB(ctrl, "mysql", persistence.DBTypeDefault, "db-2")
				mockSetupDB2 := persistence.NewMockSetupDB(ctrl)

				gomock.InOrder(
					mockFactory.EXPECT().NewAdminDBs().Return([]persistence.AdminDB{mockAdminDB1, mockAdminDB2}, nil),
					// DB 1 fails to connect
					mockAdminDB1.EXPECT().CreateSetupDB().Return(nil, errors.New("connection failed")),
					// DB 2 connects but isn't set up
					mockAdminDB2.EXPECT().CreateSetupDB().Return(mockSetupDB2, nil),
					mockSetupDB2.EXPECT().IsSetup(gomock.Any()).Return(false, nil),
					mockSetupDB2.EXPECT().Close(),
				)
				return mockFactory
			},
			expectedErrors: []string{
				"mysql/default/db-1: failed to connect",
				"mysql/default/db-2: not setup",
			},
		},
		{
			name: "empty admin databases list",
			setup: func(ctrl *gomock.Controller) persistenceClient.Factory {
				mockFactory := persistenceClient.NewMockFactory(ctrl)
				mockFactory.EXPECT().NewAdminDBs().Return([]persistence.AdminDB{}, nil)
				return mockFactory
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockFactory := tt.setup(ctrl)
			err := checkSchemas(context.Background(), mockFactory)

			if len(tt.expectedErrors) == 0 {
				require.NoError(t, err)
				return
			}
			for _, expectedError := range tt.expectedErrors {
				require.ErrorContains(t, err, expectedError)
			}
		})
	}
}
