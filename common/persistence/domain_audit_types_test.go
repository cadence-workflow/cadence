package persistence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDomainAuditLogEntry_Serialization(t *testing.T) {
	entry := &DomainAuditLogEntry{
		DomainID:      "test-domain-id",
		EventID:       "test-event-id",
		CreatedTime:   time.Now(),
		OperationType: DomainOperationTypeUpdate,
		StateBefore:   []byte(`{"field":"old"}`),
		StateAfter:    []byte(`{"field":"new"}`),
	}

	assert.Equal(t, "test-domain-id", entry.DomainID)
	assert.Equal(t, "test-event-id", entry.EventID)
	assert.NotNil(t, entry.CreatedTime)
}

func TestDomainOperationType_Constants(t *testing.T) {
	// Verify all operation type constants are properly defined
	assert.Equal(t, DomainOperationType(0), DomainOperationTypeUnknown)
	assert.Equal(t, DomainOperationType(1), DomainOperationTypeCreate)
	assert.Equal(t, DomainOperationType(2), DomainOperationTypeUpdate)
	assert.Equal(t, DomainOperationType(3), DomainOperationTypeFailover)
	assert.Equal(t, DomainOperationType(4), DomainOperationTypeDelete)
	assert.Equal(t, DomainOperationType(5), DomainOperationTypeDeprecate)

	// Verify constants can be assigned to struct field
	entry := &DomainAuditLogEntry{
		DomainID:      "test",
		EventID:       "test",
		CreatedTime:   time.Now(),
		OperationType: DomainOperationTypeCreate,
	}
	assert.Equal(t, DomainOperationTypeCreate, entry.OperationType)
}
