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
	"github.com/dgryski/go-farm"

	"github.com/uber/cadence/common/membership"
)

type simpleHashring struct {
	hosts    []membership.HostInfo
	hashfunc func([]byte) uint32
}

// newSimpleHashring returns a service resolver that maintains static mapping
// between services and host info
func newSimpleHashring(hosts []membership.HostInfo) *simpleHashring {
	return &simpleHashring{
		hosts:    hosts,
		hashfunc: farm.Fingerprint32,
	}
}

func (s *simpleHashring) Lookup(key string) (membership.HostInfo, error) {
	hash := int(s.hashfunc([]byte(key)))
	idx := hash % len(s.hosts)
	return s.hosts[idx], nil
}

// LookupN returns up to n hosts for the given key using consecutive slots in
// the ring, wrapping around if needed.
func (s *simpleHashring) LookupN(key string, n int) ([]membership.HostInfo, error) {
	if len(s.hosts) == 0 {
		return nil, membership.ErrInsufficientHosts
	}
	hash := int(s.hashfunc([]byte(key)))
	start := hash % len(s.hosts)
	count := n
	if count > len(s.hosts) {
		count = len(s.hosts)
	}
	result := make([]membership.HostInfo, count)
	for i := range count {
		result[i] = s.hosts[(start+i)%len(s.hosts)]
	}
	return result, nil
}

func (s *simpleHashring) AddListener(name string, notifyChannel chan<- *membership.ChangedEvent) error {
	return nil
}

func (s *simpleHashring) RemoveListener(name string) error {
	return nil
}

func (s *simpleHashring) MemberCount() int {
	return len(s.hosts)
}

func (s *simpleHashring) Members() []membership.HostInfo {
	return s.hosts
}
