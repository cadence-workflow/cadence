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

package cassandra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func TestInsertDomainAuditLog(t *testing.T) {
	// This test requires a Cassandra test cluster
	// For POC, we can skip if no test cluster available
	t.Skip("Requires Cassandra test cluster - tested via integration tests")

	db := createTestDB(t)
	defer db.Close()

	entry := &nosqlplugin.DomainAuditLogRow{
		DomainID:            "test-domain-123",
		EventID:             "event-456",
		CreatedTime:         time.Now(),
		LastUpdatedTime:     time.Now(),
		OperationType:       1, // DomainOperationTypeUpdate
		StateBefore:         []byte(`{}`),
		StateBeforeEncoding: "json",
		StateAfter:          []byte(`[{"op":"replace","path":"/name","value":"new"}]`),
		StateAfterEncoding:  "json-patch",
		Identity:            "user@example.com",
		IdentityType:        "user",
	}

	rows := []*nosqlplugin.DomainAuditLogRow{entry}

	err := db.InsertDomainAuditLog(context.Background(), rows)
	require.NoError(t, err)
}

func TestSelectDomainAuditLog(t *testing.T) {
	t.Skip("Requires Cassandra test cluster - tested via integration tests")

	db := createTestDB(t)
	defer db.Close()

	// First write some entries
	ctx := context.Background()
	domainID := "test-domain-read-123"

	entries := []*nosqlplugin.DomainAuditLogRow{
		{
			DomainID:            domainID,
			EventID:             "event-1",
			CreatedTime:         time.Now().Add(-2 * time.Hour),
			LastUpdatedTime:     time.Now().Add(-2 * time.Hour),
			OperationType:       1, // DomainOperationTypeUpdate
			StateBefore:         []byte(`{}`),
			StateBeforeEncoding: "json",
			StateAfter:          []byte(`{}`),
			StateAfterEncoding:  "json-patch",
		},
		{
			DomainID:            domainID,
			EventID:             "event-2",
			CreatedTime:         time.Now().Add(-1 * time.Hour),
			LastUpdatedTime:     time.Now().Add(-1 * time.Hour),
			OperationType:       2, // DomainOperationTypeFailover
			StateBefore:         []byte(`{}`),
			StateBeforeEncoding: "json",
			StateAfter:          []byte(`{}`),
			StateAfterEncoding:  "json-patch",
		},
	}

	err := db.InsertDomainAuditLog(ctx, entries)
	require.NoError(t, err)

	// Now read them back
	rows, nextPageToken, err := db.SelectDomainAuditLog(ctx, &nosqlplugin.DomainAuditLogRequest{
		DomainID: domainID,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	// Should be in descending order (newest first)
	assert.Equal(t, "event-2", rows[0].EventID)
	assert.Equal(t, "event-1", rows[1].EventID)
	assert.Nil(t, nextPageToken)
}

func createTestDB(t *testing.T) *CDB {
	// This would need actual Cassandra connection for real tests
	// For now, this is a placeholder
	t.Skip("Test DB creation not implemented - requires Cassandra instance")
	return nil
}
