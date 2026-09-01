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

package mongodb

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func (db *mdb) InsertSemaphoreTokens(ctx context.Context, rows []*nosqlplugin.SemaphoreOwnershipRow) error {
	return fmt.Errorf("InsertSemaphoreTokens is not implemented")
}

func (db *mdb) GrantSemaphoreToken(ctx context.Context, row *nosqlplugin.SemaphoreOwnershipRow) (nosqlplugin.SemaphoreGrantResult, error) {
	return nosqlplugin.SemaphoreGrantResult{}, fmt.Errorf("GrantSemaphoreToken is not implemented")
}

func (db *mdb) ReleaseSemaphoreToken(ctx context.Context, row *nosqlplugin.SemaphoreOwnershipRow) (bool, error) {
	return false, fmt.Errorf("ReleaseSemaphoreToken is not implemented")
}

func (db *mdb) SelectSemaphoreOwnershipByToken(ctx context.Context, domainID, semaphoreName string, bucket, tokenID int) (*nosqlplugin.SemaphoreOwnershipRow, error) {
	return nil, fmt.Errorf("SelectSemaphoreOwnershipByToken is not implemented")
}

func (db *mdb) SelectSemaphoreOwnershipByOwner(ctx context.Context, domainID, semaphoreName string, bucket int, ownerID string) (*nosqlplugin.SemaphoreOwnershipRow, error) {
	return nil, fmt.Errorf("SelectSemaphoreOwnershipByOwner is not implemented")
}

func (db *mdb) SelectSemaphoreOwnershipsByBucket(ctx context.Context, filter *nosqlplugin.SemaphoreOwnershipFilter) ([]*nosqlplugin.SemaphoreOwnershipRow, []byte, error) {
	return nil, nil, fmt.Errorf("SelectSemaphoreOwnershipsByBucket is not implemented")
}
