package shardscanner

import (
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
)

const (
	negativeDomainCacheTTL      = 10 * time.Second
	negativeDomainCacheMaxCount = 1000
)

type domainCacheWithNegativeCaching struct {
	cache.DomainCache
	notFound cache.Cache
}

func newDomainCacheWithNegativeCaching(domainCache cache.DomainCache, timeSource clock.TimeSource) cache.DomainCache {
	return &domainCacheWithNegativeCaching{
		DomainCache: domainCache,
		notFound: cache.New(&cache.Options{
			TTL:        negativeDomainCacheTTL,
			MaxCount:   negativeDomainCacheMaxCount,
			TimeSource: timeSource,
		}),
	}
}

func (c *domainCacheWithNegativeCaching) GetDomainByID(id string) (*cache.DomainCacheEntry, error) {
	if err := c.cachedNotFound(id); err != nil {
		return nil, err
	}
	entry, err := c.DomainCache.GetDomainByID(id)
	c.rememberNotFound(id, err)
	return entry, err
}

func (c *domainCacheWithNegativeCaching) GetDomainName(id string) (string, error) {
	if err := c.cachedNotFound(id); err != nil {
		return "", err
	}
	name, err := c.DomainCache.GetDomainName(id)
	c.rememberNotFound(id, err)
	return name, err
}

func (c *domainCacheWithNegativeCaching) cachedNotFound(id string) error {
	err, _ := c.notFound.Get(id).(error)
	return err
}

func (c *domainCacheWithNegativeCaching) rememberNotFound(id string, err error) {
	if common.IsEntityNotExistsError(err) {
		c.notFound.Put(id, err)
	}
}
