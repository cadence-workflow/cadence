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

package host

import (
	"errors"
	"fmt"
	"sync"

	"github.com/uber/cadence/common/membership"
)

type simpleResolver struct {
	hostInfo  membership.HostInfo
	resolvers map[string]*simpleHashring

	mu          sync.RWMutex
	subscribers map[string]map[string]chan<- *membership.ChangedEvent // service -> name -> channel
}

// NewSimpleResolver returns a membership resolver interface
func NewSimpleResolver(serviceName string, hosts map[string][]membership.HostInfo, currentHost membership.HostInfo) membership.Resolver {
	resolvers := make(map[string]*simpleHashring, len(hosts))
	for service, hostList := range hosts {
		resolvers[service] = newSimpleHashring(hostList)
	}
	return &simpleResolver{
		hostInfo:    currentHost,
		resolvers:   resolvers,
		subscribers: make(map[string]map[string]chan<- *membership.ChangedEvent),
	}
}

// NewSimpleResolverWithHashrings creates a resolver that shares pre-created hashrings.
// This allows multiple resolvers to see the same membership mutations.
func NewSimpleResolverWithHashrings(serviceName string, rings map[string]*simpleHashring, currentHost membership.HostInfo) membership.Resolver {
	return &simpleResolver{
		hostInfo:    currentHost,
		resolvers:   rings,
		subscribers: make(map[string]map[string]chan<- *membership.ChangedEvent),
	}
}

func (s *simpleResolver) Start() {
}

func (s *simpleResolver) Stop() {
}

func (s *simpleResolver) EvictSelf() error {
	return nil
}

func (s *simpleResolver) WhoAmI() (membership.HostInfo, error) {
	return s.hostInfo, nil
}

func (s *simpleResolver) Subscribe(service string, name string, notifyChannel chan<- *membership.ChangedEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subscribers[service]; !ok {
		s.subscribers[service] = make(map[string]chan<- *membership.ChangedEvent)
	}
	s.subscribers[service][name] = notifyChannel
	return nil
}

func (s *simpleResolver) Unsubscribe(service string, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if subs, ok := s.subscribers[service]; ok {
		delete(subs, name)
	}
	return nil
}

// NotifySubscribers sends the event to all subscribers for the given service.
// Uses non-blocking send to avoid deadlock if a subscriber isn't reading.
func (s *simpleResolver) NotifySubscribers(service string, event *membership.ChangedEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	subs, ok := s.subscribers[service]
	if !ok {
		return
	}
	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *simpleResolver) Lookup(service string, key string) (membership.HostInfo, error) {
	resolver, ok := s.resolvers[service]
	if !ok {
		return membership.HostInfo{}, fmt.Errorf("cannot lookup host for service %q", service)
	}
	return resolver.Lookup(key)
}

func (s *simpleResolver) LookupN(service string, key string, n int) ([]membership.HostInfo, error) {
	resolver, ok := s.resolvers[service]
	if !ok {
		return nil, fmt.Errorf("cannot lookup host for service %q", service)
	}
	return resolver.LookupN(key, n)
}

func (s *simpleResolver) MemberCount(service string) (int, error) {
	members, err := s.Members(service)
	return len(members), err
}

func (s *simpleResolver) Members(service string) ([]membership.HostInfo, error) {
	resolver, ok := s.resolvers[service]
	if !ok {
		return nil, fmt.Errorf("cannot lookup host for service %q", service)
	}
	return resolver.Members(), nil
}

func (s *simpleResolver) LookupByAddress(service string, address string) (membership.HostInfo, error) {
	resolver, ok := s.resolvers[service]
	if !ok {
		return membership.HostInfo{}, fmt.Errorf("cannot lookup host for service %q", service)
	}
	for _, m := range resolver.Members() {
		if belongs, err := m.Belongs(address); err == nil && belongs {
			return m, nil
		}
	}

	return membership.HostInfo{}, errors.New("host not found")
}
