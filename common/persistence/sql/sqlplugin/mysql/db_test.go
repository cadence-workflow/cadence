package mysql

import (
	"errors"
	"fmt"
	"testing"

	"github.com/VividCortex/mysqlerr"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
)

func TestIsSchemaElementExistsError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil", err: nil, expected: false},
		{name: "generic error", err: errors.New("table already exists"), expected: false},
		{name: "table exists", err: &mysql.MySQLError{Number: mysqlerr.ER_TABLE_EXISTS_ERROR}, expected: true},
		{name: "duplicate column", err: &mysql.MySQLError{Number: mysqlerr.ER_DUP_FIELDNAME}, expected: true},
		{name: "duplicate index", err: &mysql.MySQLError{Number: mysqlerr.ER_DUP_KEYNAME}, expected: true},
		{name: "wrapped", err: fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: mysqlerr.ER_TABLE_EXISTS_ERROR}), expected: true},
		{name: "duplicate row", err: &mysql.MySQLError{Number: mysqlerr.ER_DUP_ENTRY}, expected: false},
		{name: "missing table", err: &mysql.MySQLError{Number: mysqlerr.ER_NO_SUCH_TABLE}, expected: false},
	}
	db := &DB{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, db.IsSchemaElementExistsError(tt.err))
		})
	}
}
