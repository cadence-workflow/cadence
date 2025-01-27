package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/config"
)

func TestPlugin_CreateDB(t *testing.T) {
	p := &plugin{}
	db, err := p.CreateDB(&config.SQL{})

	assert.NoError(t, err)
	assert.NotNil(t, db)
}

func TestPlugin_CreateAdminDB(t *testing.T) {
	p := &plugin{}
	db, err := p.CreateDB(&config.SQL{})

	assert.NoError(t, err)
	assert.NotNil(t, db)
}
