package cassandra

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

func TestApplyUpdate(t *testing.T) {
	existsErr := errors.New("already exists")
	otherErr := errors.New("syntax error")
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
			session := gocql.NewMockSession(ctrl)
			client := gocql.NewMockClient(ctrl)

			stmt1 := gocql.NewMockQuery(ctrl)
			session.EXPECT().Query("stmt1").Return(stmt1)
			stmt1.EXPECT().Exec().Return(tt.stmt1Err)
			client.EXPECT().IsSchemaElementExistsError(gomock.Any()).DoAndReturn(func(err error) bool {
				return errors.Is(err, existsErr)
			}).AnyTimes()
			if tt.expectStmt2 {
				stmt2 := gocql.NewMockQuery(ctrl)
				session.EXPECT().Query("stmt2").Return(stmt2)
				stmt2.EXPECT().Exec().Return(nil)
			}
			db := NewCassandraDBFromSession(&config.NoSQL{}, session, testlogger.New(t), persistence.NewDefaultDynamicConfiguration(), DbWithClient(client))

			err := db.applyUpdate(t.Context(), &persistence.SchemaUpdate{DDLStatements: []string{"stmt1", "stmt2"}}, tt.mode)
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
