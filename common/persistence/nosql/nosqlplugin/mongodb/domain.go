// Copyright (c) 2020 Uber Technologies, Inc.
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
	"time"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

// Insert a new record to domain, return error if failed or already exists
// Return ConditionFailure if the condition doesn't meet
func (db *mdb) InsertDomain(
	ctx context.Context,
	row *nosqlplugin.DomainRow,
) error {
	panic("TODO")
}

// Update domain
func (db *mdb) UpdateDomain(
	ctx context.Context,
	row *nosqlplugin.DomainRow,
) error {
	panic("TODO")
}

// Get one domain data, either by domainID or domainName
func (db *mdb) SelectDomain(
	ctx context.Context,
	domainID *string,
	domainName *string,
) (*nosqlplugin.DomainRow, error) {
	panic("TODO")
}

// Get all domain data
func (db *mdb) SelectAllDomains(
	ctx context.Context,
	pageSize int,
	pageToken []byte,
) ([]*nosqlplugin.DomainRow, []byte, error) {
	panic("TODO")
}

// Delete a domain, either by domainID or domainName
func (db *mdb) DeleteDomain(
	ctx context.Context,
	domainID *string,
	domainName *string,
) error {
	panic("TODO")
}

func (db *mdb) SelectDomainMetadata(
	ctx context.Context,
) (int64, error) {
	panic("TODO")
}

// InsertDomainAuditLog inserts domain audit log entries
func (db *mdb) InsertDomainAuditLog(
	ctx context.Context,
	rows []*nosqlplugin.DomainAuditLogRow,
) error {
	panic("TODO")
}

// SelectDomainAuditLog selects domain audit log entries with pagination
func (db *mdb) SelectDomainAuditLog(
	ctx context.Context,
	request *nosqlplugin.DomainAuditLogRequest,
) ([]*nosqlplugin.DomainAuditLogRow, []byte, error) {
	panic("TODO")
}

// GetDomainAuditLogEntry retrieves a specific domain audit log entry by its composite key
func (db *mdb) GetDomainAuditLogEntry(
	ctx context.Context,
	domainID string,
	eventID string,
	createdTime time.Time,
) (*nosqlplugin.DomainAuditLogRow, error) {
	panic("TODO")
}
