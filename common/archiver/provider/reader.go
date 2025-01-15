package provider

import (
	"context"
	"fmt"
	"github.com/golang/groupcache/lru"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/syncmap"
	"sync"
)

func NewExecutionContainer(serviceName string, archiverFactories syncmap.SyncMap[string, executionReaderConstructor]) archiver.ExecutionReadContainer {
	return &readContainer{
		serviceName:       serviceName,
		mu:                sync.RWMutex{},
		archiverFactories: archiverFactories,
		dc:                nil, // instantiated later
		readerCache:       lru.New(100),
	}
}

type readContainer struct {
	serviceName       string
	mu                sync.RWMutex
	archiverFactories syncmap.SyncMap[string, executionReaderConstructor]
	cfgs              config.ExecutionArchiverProvider
	dc                cache.DomainCache
	readerCache       *lru.Cache // archiver cache, keyed by domain ID

	Logger        log.Logger
	MetricsClient metrics.Client
	DomainCache   cache.DomainCache
}

func (e *readContainer) GetWorkflowExecutionForPersistence(ctx context.Context, req *archiver.GetExecutionRequest) (*archiver.GetExecutionResponse, error) {
	e.mu.RLock()
	cachedReader, ok := e.readerCache.Get(req.DomainID)
	if ok {
		reader, ok := cachedReader.(archiver.ExecutionReader)
		if !ok {
			return nil, fmt.Errorf("programmatic error, cached execution reader was an unexpected type")
		}
		e.mu.RUnlock()
		return reader.GetWorkflowExecutionForPersistence(ctx, req)
	}
	e.mu.RUnlock()

	reader, err := e.setupProvider(req.DomainID)
	if err != nil {
		return nil, err
	}

	return reader.GetWorkflowExecutionForPersistence(ctx, req)
}

func (e *readContainer) setupProvider(domainID string) (archiver.ExecutionReader, error) {

	e.mu.Lock()
	defer e.mu.Unlock()

	domain, err := e.dc.GetDomainByID(domainID)
	if err != nil {
		return nil, err
	}
	uri, err := archiver.NewURI(domain.GetConfig().HistoryArchivalURI)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse archival URI for domain: %w", err)
	}

	archiverKey := getArchiverKey(uri.Scheme(), e.serviceName)

	executionReaderConstructor, ok := e.archiverFactories.Get(archiverKey)
	if !ok {
		return nil, fmt.Errorf("couldn't find archiver for key: %s", archiverKey)
	}

	cfg, ok := e.cfgs[executionReaderConstructor.configKey]
	if !ok {
		return nil, fmt.Errorf("no archiver config for scheme %q, config key %q", uri.Scheme(), executionReaderConstructor.configKey)
	}
	reader, err := executionReaderConstructor.fn(cfg, e)
	if err != nil {
		return nil, fmt.Errorf("failed to setup constructor for %q, config key %q. Err: %w", uri, executionReaderConstructor.configKey, err)
	}

	e.readerCache.Add(domainID, reader)

	return reader, nil
}

func (e *readContainer) SetDomainCache(dc cache.DomainCache) {
	e.DomainCache = dc
}

func (e *readContainer) GetDomainCache() cache.DomainCache {
	return e.DomainCache
}

func (e *readContainer) GetLog() log.Logger {
	return e.Logger
}
func (e *readContainer) GetMetrics() metrics.Client {
	return e.MetricsClient
}

//func (r *readManager) ReadExecution() {
//
//	archiverKey := p.getArchiverKey(scheme, serviceName)
//	p.RLock()
//	if executionArchiver, ok := p.executionArchivers[archiverKey]; ok {
//		p.RUnlock()
//		return executionArchiver, nil
//	}
//	p.RUnlock()
//
//	container, ok := p.executionArchiveContainers[serviceName]
//	if !ok {
//		return nil, ErrBootstrapContainerNotFound
//	}
//
//	constructor, ok := executionArchiverConstructors.Get(scheme)
//	if !ok {
//		return nil, fmt.Errorf("no visibility archiver constructor for scheme %q", scheme)
//	}
//
//	cfg, ok := p.executionArchiverConfigs[constructor.configKey]
//	if !ok {
//		return nil, fmt.Errorf("no visibility archiver config for scheme %q, config key %q", scheme, constructor.configKey)
//	}
//
//	p.Lock()
//	defer p.Unlock()
//	if existingExecutionArchiver, ok := p.executionArchivers[archiverKey]; ok {
//		return existingExecutionArchiver, nil
//	}
//
//	executionArchiver, err := constructor.fn(cfg, container)
//	if err != nil {
//		return nil, fmt.Errorf("visibility archiver constructor failed for scheme %q, config key %q: err: %w", scheme, constructor.configKey, err)
//	}
//
//	p.executionArchivers[archiverKey] = executionArchiver
//	return executionArchiver, nil
//}
