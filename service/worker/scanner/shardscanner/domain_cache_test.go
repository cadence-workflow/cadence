package shardscanner

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestDomainCacheWithNegativeCaching(t *testing.T) {
	const domainID = "domain-id"
	notExistsErr := &types.EntityNotExistsError{Message: "domain not found"}
	otherErr := errors.New("timeout")
	entry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: domainID, Name: "domain-name"},
		&persistence.DomainConfig{},
		"active",
	)

	// lookup runs the same call against either of the overridden methods
	type lookup func(dc cache.DomainCache) error
	byName := func(dc cache.DomainCache) error {
		_, err := dc.GetDomainName(domainID)
		return err
	}
	byID := func(dc cache.DomainCache) error {
		_, err := dc.GetDomainByID(domainID)
		return err
	}

	tests := map[string]struct {
		lookup  lookup
		advance time.Duration
		// expect sets up the underlying cache expectations for two lookups separated by advance
		expect  func(m *cache.MockDomainCache)
		wantErr error
	}{
		"name: not found is cached": {
			lookup: byName,
			expect: func(m *cache.MockDomainCache) {
				m.EXPECT().GetDomainName(domainID).Return("", notExistsErr).Times(1)
			},
			wantErr: notExistsErr,
		},
		"name: not found expires after TTL": {
			lookup:  byName,
			advance: negativeDomainCacheTTL + time.Second,
			expect: func(m *cache.MockDomainCache) {
				m.EXPECT().GetDomainName(domainID).Return("", notExistsErr).Times(2)
			},
			wantErr: notExistsErr,
		},
		"name: found is not cached": {
			lookup: byName,
			expect: func(m *cache.MockDomainCache) {
				m.EXPECT().GetDomainName(domainID).Return("domain-name", nil).Times(2)
			},
		},
		"name: other errors are not cached": {
			lookup: byName,
			expect: func(m *cache.MockDomainCache) {
				m.EXPECT().GetDomainName(domainID).Return("", otherErr).Times(2)
			},
			wantErr: otherErr,
		},
		"id: not found is cached": {
			lookup: byID,
			expect: func(m *cache.MockDomainCache) {
				m.EXPECT().GetDomainByID(domainID).Return(nil, notExistsErr).Times(1)
			},
			wantErr: notExistsErr,
		},
		"id: not found expires after TTL": {
			lookup:  byID,
			advance: negativeDomainCacheTTL + time.Second,
			expect: func(m *cache.MockDomainCache) {
				m.EXPECT().GetDomainByID(domainID).Return(nil, notExistsErr).Times(2)
			},
			wantErr: notExistsErr,
		},
		"id: found is not cached": {
			lookup: byID,
			expect: func(m *cache.MockDomainCache) {
				m.EXPECT().GetDomainByID(domainID).Return(entry, nil).Times(2)
			},
		},
		"id: other errors are not cached": {
			lookup: byID,
			expect: func(m *cache.MockDomainCache) {
				m.EXPECT().GetDomainByID(domainID).Return(nil, otherErr).Times(2)
			},
			wantErr: otherErr,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(ctrl)
			tc.expect(mockDomainCache)
			timeSource := clock.NewMockedTimeSource()
			dc := newDomainCacheWithNegativeCaching(mockDomainCache, timeSource)

			assert.Equal(t, tc.wantErr, tc.lookup(dc))
			timeSource.Advance(tc.advance)
			assert.Equal(t, tc.wantErr, tc.lookup(dc))
		})
	}
}

func TestDomainCacheWithNegativeCaching_SharedBetweenMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	notExistsErr := &types.EntityNotExistsError{Message: "domain not found"}
	mockDomainCache.EXPECT().GetDomainName("domain-id").Return("", notExistsErr).Times(1)
	dc := newDomainCacheWithNegativeCaching(mockDomainCache, clock.NewMockedTimeSource())

	_, err := dc.GetDomainName("domain-id")
	assert.Equal(t, notExistsErr, err)
	entry, err := dc.GetDomainByID("domain-id")
	assert.Nil(t, entry)
	assert.Equal(t, notExistsErr, err)
}

func TestDomainCacheWithNegativeCaching_CachesPerID(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	notExistsErr := &types.EntityNotExistsError{Message: "domain not found"}
	mockDomainCache.EXPECT().GetDomainName("missing-id").Return("", notExistsErr).Times(1)
	mockDomainCache.EXPECT().GetDomainName("existing-id").Return("domain-name", nil).Times(2)
	dc := newDomainCacheWithNegativeCaching(mockDomainCache, clock.NewMockedTimeSource())

	for i := 0; i < 2; i++ {
		_, err := dc.GetDomainName("missing-id")
		assert.Equal(t, notExistsErr, err)
		name, err := dc.GetDomainName("existing-id")
		assert.NoError(t, err)
		assert.Equal(t, "domain-name", name)
	}
}
