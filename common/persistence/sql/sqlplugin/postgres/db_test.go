package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestIsSchemaElementExistsError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil", err: nil, expected: false},
		{name: "generic error", err: errors.New("relation already exists"), expected: false},
		{name: "duplicate table", err: &pq.Error{Code: ErrDuplicateTable}, expected: true},
		{name: "duplicate column", err: &pq.Error{Code: ErrDuplicateColumn}, expected: true},
		{name: "duplicate object", err: &pq.Error{Code: ErrDuplicateObject}, expected: true},
		{name: "wrapped", err: fmt.Errorf("wrapped: %w", &pq.Error{Code: ErrDuplicateTable}), expected: true},
		{name: "duplicate row", err: &pq.Error{Code: ErrDupEntry}, expected: false},
		{name: "undefined table", err: &pq.Error{Code: "42P01"}, expected: false},
	}
	db := &db{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, db.IsSchemaElementExistsError(tt.err))
		})
	}
}
