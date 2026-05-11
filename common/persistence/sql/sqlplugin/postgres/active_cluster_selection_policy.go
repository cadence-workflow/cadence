// Copyright (c) 2026 Uber Technologies, Inc.
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

package postgres

import (
	"context"
	"database/sql"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

func (pdb *db) InsertIntoActiveClusterSelectionPolicy(ctx context.Context, row *sqlplugin.ActiveClusterSelectionPolicyRow) (sql.Result, error) {
	return nil, sqlplugin.ErrNotImplemented
}

func (pdb *db) SelectFromActiveClusterSelectionPolicy(ctx context.Context, filter *sqlplugin.ActiveClusterSelectionPolicyFilter) (*sqlplugin.ActiveClusterSelectionPolicyRow, error) {
	return nil, sqlplugin.ErrNotImplemented
}

func (pdb *db) DeleteFromActiveClusterSelectionPolicy(ctx context.Context, filter *sqlplugin.ActiveClusterSelectionPolicyFilter) (sql.Result, error) {
	return nil, sqlplugin.ErrNotImplemented
}
