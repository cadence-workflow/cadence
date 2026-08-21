// Copyright (c) 2025 Uber Technologies, Inc.
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

package dynamodb

import (
	"context"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func (db *ddb) InsertSemaphoreTokens(ctx context.Context, rows []*nosqlplugin.SemaphoreTokenRow) error {
	panic("TODO: InsertSemaphoreTokens is not implemented")
}

func (db *ddb) GrantSemaphoreToken(ctx context.Context, row *nosqlplugin.SemaphoreTokenRow) (nosqlplugin.SemaphoreGrantResult, error) {
	panic("TODO: GrantSemaphoreToken is not implemented")
}

func (db *ddb) ReleaseSemaphoreToken(ctx context.Context, row *nosqlplugin.SemaphoreTokenRow) (bool, error) {
	panic("TODO: ReleaseSemaphoreToken is not implemented")
}

func (db *ddb) SelectSemaphoreTokenByID(ctx context.Context, domainID, semaphoreName string, bucket, tokenID int) (*nosqlplugin.SemaphoreTokenRow, error) {
	panic("TODO: SelectSemaphoreTokenByID is not implemented")
}

func (db *ddb) SelectSemaphoreTokenByOwner(ctx context.Context, domainID, semaphoreName string, bucket int, ownerID string) (*nosqlplugin.SemaphoreTokenRow, error) {
	panic("TODO: SelectSemaphoreTokenByOwner is not implemented")
}

func (db *ddb) SelectSemaphoreBucketRows(ctx context.Context, filter *nosqlplugin.SemaphoreTokenFilter) ([]*nosqlplugin.SemaphoreTokenRow, []byte, error) {
	panic("TODO: SelectSemaphoreBucketRows is not implemented")
}
