package sql

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

func TestSQLSchemaDB_UpdateSchema(t *testing.T) {
	existsErr := errors.New("already exists")
	otherErr := errors.New("syntax error")
	update := &persistence.SchemaUpdate{
		Version:              persistence.Version{Major: 0, Minor: 2},
		MinCompatibleVersion: persistence.Version{Major: 0, Minor: 2},
		DDLStatements:        []string{"stmt1", "stmt2"},
		ManifestMD5:          "md5",
		Description:          "desc",
	}
	tests := []struct {
		name            string
		mode            persistence.SchemaUpdateMode
		stmt1Err        error
		expectStmt2     bool
		expectErr       error
		expectDuplicate bool
	}{
		{
			name:        "strict success",
			mode:        persistence.SchemaUpdateModeStrict,
			expectStmt2: true,
		},
		{
			name:        "resume success",
			mode:        persistence.SchemaUpdateModeResume,
			expectStmt2: true,
		},
		{
			name:            "strict fails on already applied statement",
			mode:            persistence.SchemaUpdateModeStrict,
			stmt1Err:        existsErr,
			expectErr:       existsErr,
			expectDuplicate: true,
		},
		{
			name:        "resume skips already applied statement",
			mode:        persistence.SchemaUpdateModeResume,
			stmt1Err:    existsErr,
			expectStmt2: true,
		},
		{
			name:      "strict fails on other errors",
			mode:      persistence.SchemaUpdateModeStrict,
			stmt1Err:  otherErr,
			expectErr: otherErr,
		},
		{
			name:      "resume fails on other errors",
			mode:      persistence.SchemaUpdateModeResume,
			stmt1Err:  otherErr,
			expectErr: otherErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			crud := sqlplugin.NewMockAdminDB(ctrl)
			crud.EXPECT().ReadSchemaVersion("db").Return("0.1", nil)
			crud.EXPECT().ExecSchemaOperationQuery(gomock.Any(), "stmt1").Return(tt.stmt1Err)
			crud.EXPECT().IsSchemaElementExistsError(gomock.Any()).DoAndReturn(func(err error) bool {
				return errors.Is(err, existsErr)
			}).AnyTimes()
			if tt.expectStmt2 {
				crud.EXPECT().ExecSchemaOperationQuery(gomock.Any(), "stmt2").Return(nil)
				crud.EXPECT().UpdateSchemaVersion("db", "0.2", "0.2").Return(nil)
				crud.EXPECT().WriteSchemaUpdateLog("0.1", "0.2", "md5", "desc").Return(nil)
			}
			schemaDB := NewSQLSchemaDB(testlogger.New(t), "db", crud, nil)

			err := schemaDB.UpdateSchema(t.Context(), update, tt.mode)
			if tt.expectErr != nil {
				assert.ErrorIs(t, err, tt.expectErr)
			} else {
				assert.NoError(t, err)
			}
			var duplicateErr *persistence.DuplicateSchemaElementError
			if assert.Equal(t, tt.expectDuplicate, errors.As(err, &duplicateErr)) && tt.expectDuplicate {
				assert.Equal(t, "stmt1", duplicateErr.Statement)
			}
		})
	}
}
