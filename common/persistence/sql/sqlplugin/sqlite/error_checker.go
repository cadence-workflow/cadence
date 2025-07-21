// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ncruces/go-sqlite3"
)

// IsDupEntryError verify if the error is a duplicate entry error
func (mdb *DB) IsDupEntryError(err error) bool {
	var sqlErr *sqlite3.Error
	if ok := errors.As(err, &sqlErr); !ok {
		return false
	}

	switch sqlErr.ExtendedCode() {
	case
		// https://sqlite.org/rescode.html#constraint_unique
		sqlite3.CONSTRAINT_UNIQUE,

		// https://sqlite.org/rescode.html#constraint_primarykey
		sqlite3.CONSTRAINT_PRIMARYKEY:
		return true
	}

	return false
}

// IsNotFoundError verify if the error is a not found error
func (mdb *DB) IsNotFoundError(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// IsTimeoutError verify if the error is a timeout error
func (mdb *DB) IsTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var sqlErr *sqlite3.Error
	if ok := errors.As(err, &sqlErr); !ok {
		return false
	}

	// https://sqlite.org/rescode.html#busy_timeout
	if sqlErr.Timeout() {
		return true
	}

	// https://sqlite.org/rescode.html#interrupt
	if sqlErr.Code() == sqlite3.INTERRUPT {
		return true
	}

	return false
}

// IsThrottlingError verify if the error is a throttling error
func (mdb *DB) IsThrottlingError(_ error) bool {
	return false
}
