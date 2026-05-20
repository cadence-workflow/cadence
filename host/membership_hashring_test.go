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
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/membership"
)

func newTestHost(port uint16) membership.HostInfo {
	return newHost(port)
}

func TestSimpleHashring_AddHost(t *testing.T) {
	ring := newSimpleHashring([]membership.HostInfo{newTestHost(7201)})
	require.Equal(t, 1, ring.MemberCount())

	ring.AddHost(newTestHost(7202))
	require.Equal(t, 2, ring.MemberCount())
	require.Len(t, ring.Members(), 2)
}

func TestSimpleHashring_RemoveHost(t *testing.T) {
	host1 := newTestHost(7201)
	host2 := newTestHost(7202)
	ring := newSimpleHashring([]membership.HostInfo{host1, host2})
	require.Equal(t, 2, ring.MemberCount())

	ring.RemoveHost(host1)
	require.Equal(t, 1, ring.MemberCount())
	require.Equal(t, host2.Identity(), ring.Members()[0].Identity())
}

func TestSimpleHashring_RemoveHost_NotFound(t *testing.T) {
	ring := newSimpleHashring([]membership.HostInfo{newTestHost(7201)})
	ring.RemoveHost(newTestHost(9999)) // no-op
	require.Equal(t, 1, ring.MemberCount())
}

func TestSimpleHashring_ConcurrentAccess(t *testing.T) {
	ring := newSimpleHashring([]membership.HostInfo{newTestHost(7201)})
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		port := uint16(8000 + i)
		go func() {
			defer wg.Done()
			ring.AddHost(newTestHost(port))
		}()
		go func() {
			defer wg.Done()
			ring.Lookup("some-key")
		}()
	}
	wg.Wait()
	// no panic = pass
}
