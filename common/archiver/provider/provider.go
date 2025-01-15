// Copyright (c) 2017 Uber Technologies, Inc.
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

package provider

import (
	"errors"
	"fmt"
	"sync"

	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/syncmap"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination provider_mock.go -self_package github.com/uber/cadence/common/archival

var (
	// ErrUnknownScheme is the error for unknown archiver scheme
	ErrUnknownScheme = errors.New("unknown archiver scheme")
	// ErrNotSupported is the error for not supported archiver implementation
	ErrNotSupported = errors.New("archiver provider not supported")
	// ErrBootstrapContainerNotFound is the error for unable to find the bootstrap container given serviceName
	ErrBootstrapContainerNotFound = errors.New("unable to find bootstrap container for the given service name")
	// ErrArchiverConfigNotFound is the error for unable to find the config for an archiver given scheme
	ErrArchiverConfigNotFound = errors.New("unable to find archiver config for the given scheme")
	// ErrBootstrapContainerAlreadyRegistered is the error for registering multiple containers for the same serviceName
	ErrBootstrapContainerAlreadyRegistered = errors.New("bootstrap container has already been registered")
)

type (
	// ArchiverProvider returns history or visibility archiver based on the scheme and serviceName.
	// The archiver for each combination of scheme and serviceName will be created only once and cached.
	ArchiverProvider interface {
		RegisterBootstrapContainer(
			serviceName string,
			historyContainer *archiver.HistoryBootstrapContainer,
			visibilityContainter *archiver.VisibilityBootstrapContainer,
			executionArchiverContainer *archiver.ExecutionArchiverBootstrapContainer,
			executionReaderContainer archiver.ExecutionReadContainer,
		) error
		GetHistoryArchiver(scheme, serviceName string) (archiver.HistoryArchiver, error)
		GetVisibilityArchiver(scheme, serviceName string) (archiver.VisibilityArchiver, error)
		GetExecutionArchiver(scheme, serviceName string) (archiver.ExecutionArchiver, error)
		GetExecutionReader()
	}

	archiverProvider struct {
		sync.RWMutex

		historyArchiverConfigs    config.HistoryArchiverProvider
		visibilityArchiverConfigs config.VisibilityArchiverProvider
		executionArchiverConfigs  config.ExecutionArchiverProvider

		// Key for the container is just serviceName
		historyContainers          map[string]*archiver.HistoryBootstrapContainer
		visibilityContainers       map[string]*archiver.VisibilityBootstrapContainer
		executionArchiveContainers map[string]*archiver.ExecutionArchiverBootstrapContainer

		// Key for the archiver is scheme + serviceName
		historyArchivers    map[string]archiver.HistoryArchiver
		visibilityArchivers map[string]archiver.VisibilityArchiver
		executionArchivers  map[string]archiver.ExecutionArchiver
		executionReaders    map[string]archiver.ExecutionReader
	}

	historyConstructor struct {
		fn func(cfg *config.YamlNode, container *archiver.HistoryBootstrapContainer) (archiver.HistoryArchiver, error)
		// yaml key where this config exists, under archival.history.provider.
		// This almost certainly should be the same as the scheme, but that'll need more work.
		configKey string
	}
	visibilityConstructor struct {
		fn func(cfg *config.YamlNode, container *archiver.VisibilityBootstrapContainer) (archiver.VisibilityArchiver, error)
		// yaml key where this config exists, under archival.visibility.provider.
		// This almost certainly should be the same as the scheme, but that'll need more work.
		configKey string
	}
	executionArchiverConstructor struct {
		fn        func(cfg *config.YamlNode, container *archiver.ExecutionArchiverBootstrapContainer) (archiver.ExecutionArchiver, error)
		configKey string
	}
	executionReaderConstructor struct {
		fn        func(cfg *config.YamlNode, container archiver.ExecutionReadContainer) (archiver.ExecutionReader, error)
		configKey string
	}
)

var (
	historyConstructors           = syncmap.New[string, historyConstructor]()
	visibilityConstructors        = syncmap.New[string, visibilityConstructor]()
	executionArchiverConstructors = syncmap.New[string, executionArchiverConstructor]()
	executionReaderConstructors   = syncmap.New[string, executionReaderConstructor]()
)

func RegisterHistoryArchiver(scheme, configKey string, constructor func(cfg *config.YamlNode, container *archiver.HistoryBootstrapContainer) (archiver.HistoryArchiver, error)) error {
	inserted := historyConstructors.Put(scheme, historyConstructor{
		fn:        constructor,
		configKey: configKey,
	})
	if !inserted {
		return fmt.Errorf("history archiver already registered for scheme %q", scheme)
	}
	return nil
}

func RegisterExecutionArchiver(scheme, configKey string, constructor func(cfg *config.YamlNode, container *archiver.ExecutionArchiverBootstrapContainer) (archiver.ExecutionArchiver, error)) error {
	inserted := executionArchiverConstructors.Put(scheme, executionArchiverConstructor{
		fn:        constructor,
		configKey: configKey,
	})
	if !inserted {
		return fmt.Errorf("history archiver already registered for scheme %q", scheme)
	}
	return nil
}

func RegisterExecutionReader(scheme, configKey string, constructor func(cfg *config.YamlNode, container archiver.ExecutionReadContainer) (archiver.ExecutionReader, error)) error {
	inserted := executionReaderConstructors.Put(scheme, executionReaderConstructor{
		fn:        constructor,
		configKey: configKey,
	})
	if !inserted {
		return fmt.Errorf("history archiver already registered for scheme %q", scheme)
	}
	return nil
}

func RegisterVisibilityArchiver(scheme, configKey string, constructor func(cfg *config.YamlNode, container *archiver.VisibilityBootstrapContainer) (archiver.VisibilityArchiver, error)) error {
	inserted := visibilityConstructors.Put(scheme, visibilityConstructor{
		fn:        constructor,
		configKey: configKey,
	})
	if !inserted {
		return fmt.Errorf("visibility archiver already registered for scheme %q", scheme)
	}
	return nil
}

// NewArchiverProvider returns a new Archiver provider
func NewArchiverProvider(
	historyArchiverConfigs config.HistoryArchiverProvider,
	visibilityArchiverConfigs config.VisibilityArchiverProvider,
	executionArchiverConfigs config.ExecutionArchiverProvider,
) ArchiverProvider {
	return &archiverProvider{
		historyArchiverConfigs:     historyArchiverConfigs,
		visibilityArchiverConfigs:  visibilityArchiverConfigs,
		executionArchiverConfigs:   executionArchiverConfigs,
		historyContainers:          make(map[string]*archiver.HistoryBootstrapContainer),
		visibilityContainers:       make(map[string]*archiver.VisibilityBootstrapContainer),
		executionArchiveContainers: make(map[string]*archiver.ExecutionArchiverBootstrapContainer),
		historyArchivers:           make(map[string]archiver.HistoryArchiver),
		visibilityArchivers:        make(map[string]archiver.VisibilityArchiver),
		executionArchivers:         make(map[string]archiver.ExecutionArchiver),
	}
}

// RegisterBootstrapContainer stores the given bootstrap container given the serviceName
// The container should be registered when a service starts up and before GetArchiver() is ever called.
// Later calls to GetArchiver() will used the registered container to initialize new archivers.
// If the container for a service has already registered, and this method is invoked for that service again
// with an non-nil container, an error will be returned.
func (p *archiverProvider) RegisterBootstrapContainer(
	serviceName string,
	historyContainer *archiver.HistoryBootstrapContainer,
	visibilityContainter *archiver.VisibilityBootstrapContainer,
	executionArchiverContainer *archiver.ExecutionArchiverBootstrapContainer,
	executionReaderContainer archiver.ExecutionReadContainer,
) error {
	p.Lock()
	defer p.Unlock()

	if _, ok := p.historyContainers[serviceName]; ok && historyContainer != nil {
		return ErrBootstrapContainerAlreadyRegistered
	}
	if _, ok := p.visibilityContainers[serviceName]; ok && visibilityContainter != nil {
		return ErrBootstrapContainerAlreadyRegistered
	}
	if _, ok := p.executionArchiveContainers[serviceName]; ok && executionArchiverContainer != nil {
		return ErrBootstrapContainerAlreadyRegistered
	}
	//if _, ok := p.executionReaderContainers[serviceName]; ok && executionReaderContainer != nil {
	//	return ErrBootstrapContainerAlreadyRegistered
	//}

	if historyContainer != nil {
		p.historyContainers[serviceName] = historyContainer
	}
	if visibilityContainter != nil {
		p.visibilityContainers[serviceName] = visibilityContainter
	}
	if executionArchiverContainer != nil {
		p.executionArchiveContainers[serviceName] = executionArchiverContainer
	}
	//if executionReaderContainer != nil {
	//	p.executionReaderContainers[serviceName] = executionReaderContainer
	//}
	return nil
}

func (p *archiverProvider) GetHistoryArchiver(scheme, serviceName string) (historyArchiver archiver.HistoryArchiver, err error) {
	archiverKey := getArchiverKey(scheme, serviceName)
	p.RLock()
	if historyArchiver, ok := p.historyArchivers[archiverKey]; ok {
		p.RUnlock()
		return historyArchiver, nil
	}
	p.RUnlock()

	container, ok := p.historyContainers[serviceName]
	if !ok {
		return nil, ErrBootstrapContainerNotFound
	}

	constructor, ok := historyConstructors.Get(scheme)
	if !ok {
		return nil, fmt.Errorf("no history archiver constructor for scheme %q", scheme)
	}

	cfg, ok := p.historyArchiverConfigs[constructor.configKey]
	if !ok {
		return nil, fmt.Errorf("no history archiver config for scheme %q, config key %q", scheme, constructor.configKey)
	}

	p.Lock()
	defer p.Unlock()
	if existingHistoryArchiver, ok := p.historyArchivers[archiverKey]; ok {
		return existingHistoryArchiver, nil
	}

	historyArchiver, err = constructor.fn(cfg, container)
	if err != nil {
		return nil, fmt.Errorf("history archiver constructor failed for scheme %q, config key %q: err: %w", scheme, constructor.configKey, err)
	}

	p.historyArchivers[archiverKey] = historyArchiver
	return historyArchiver, nil
}

func (p *archiverProvider) GetVisibilityArchiver(scheme, serviceName string) (archiver.VisibilityArchiver, error) {
	archiverKey := getArchiverKey(scheme, serviceName)
	p.RLock()
	if visibilityArchiver, ok := p.visibilityArchivers[archiverKey]; ok {
		p.RUnlock()
		return visibilityArchiver, nil
	}
	p.RUnlock()

	container, ok := p.visibilityContainers[serviceName]
	if !ok {
		return nil, ErrBootstrapContainerNotFound
	}

	constructor, ok := visibilityConstructors.Get(scheme)
	if !ok {
		return nil, fmt.Errorf("no visibility archiver constructor for scheme %q", scheme)
	}

	cfg, ok := p.visibilityArchiverConfigs[constructor.configKey]
	if !ok {
		return nil, fmt.Errorf("no visibility archiver config for scheme %q, config key %q", scheme, constructor.configKey)
	}

	visibilityArchiver, err := constructor.fn(cfg, container)
	if err != nil {
		return nil, fmt.Errorf("visibility archiver constructor failed for scheme %q, config key %q: err: %w", scheme, constructor.configKey, err)
	}

	p.Lock()
	defer p.Unlock()
	if existingVisibilityArchiver, ok := p.visibilityArchivers[archiverKey]; ok {
		return existingVisibilityArchiver, nil
	}
	p.visibilityArchivers[archiverKey] = visibilityArchiver
	return visibilityArchiver, nil
}

func getArchiverKey(scheme, serviceName string) string {
	return scheme + ":" + serviceName
}

func (p *archiverProvider) GetExecutionArchiver(scheme, serviceName string) (archiver.ExecutionArchiver, error) {
	archiverKey := getArchiverKey(scheme, serviceName)
	p.RLock()
	if executionArchiver, ok := p.executionArchivers[archiverKey]; ok {
		p.RUnlock()
		return executionArchiver, nil
	}
	p.RUnlock()

	container, ok := p.executionArchiveContainers[serviceName]
	if !ok {
		return nil, ErrBootstrapContainerNotFound
	}

	constructor, ok := executionArchiverConstructors.Get(scheme)
	if !ok {
		return nil, fmt.Errorf("no visibility archiver constructor for scheme %q", scheme)
	}

	cfg, ok := p.executionArchiverConfigs[constructor.configKey]
	if !ok {
		return nil, fmt.Errorf("no visibility archiver config for scheme %q, config key %q", scheme, constructor.configKey)
	}

	p.Lock()
	defer p.Unlock()
	if existingExecutionArchiver, ok := p.executionArchivers[archiverKey]; ok {
		return existingExecutionArchiver, nil
	}

	executionArchiver, err := constructor.fn(cfg, container)
	if err != nil {
		return nil, fmt.Errorf("visibility archiver constructor failed for scheme %q, config key %q: err: %w", scheme, constructor.configKey, err)
	}

	p.executionArchivers[archiverKey] = executionArchiver
	return executionArchiver, nil
}

//func (p *archiverProvider) GetExecutionReader(service string) (archiver.ExecutionReader, error) {
//	container, ok := p.executionReaderContainer[service]
//	if !ok {
//		return nil, ErrBootstrapContainerNotFound
//	}
//
//	constructor, ok := executionReaderConstructors.Get(scheme)
//	if !ok {
//		return nil, fmt.Errorf("no visibility archiver constructor for scheme %q", scheme)
//	}
//
//	// this shares the same config as archiver, since it's just the read section of the same thing
//	cfg, ok := p.executionArchiverConfigs[constructor.configKey]
//	if !ok {
//		return nil, fmt.Errorf("no visibility archiver config for scheme %q, config key %q", scheme, constructor.configKey)
//	}
//
//	p.Lock()
//	defer p.Unlock()
//	if existingExecutionReader, ok := p.executionReaders[archiverKey]; ok {
//		return existingExecutionReader, nil
//	}
//
//	executionReader, err := constructor.fn(cfg, container)
//	if err != nil {
//		return nil, fmt.Errorf("visibility archiver constructor failed for scheme %q, config key %q: err: %w", scheme, constructor.configKey, err)
//	}
//
//	p.executionReaders[archiverKey] = executionReader
//	return executionReader, nil
//}
