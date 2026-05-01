// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package ringpopprovider

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/tchannel-go"
	"go.uber.org/goleak"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	ringpopproviderconfig "github.com/uber/cadence/common/peerprovider/ringpopprovider/config"
)

const testServiceName = "test-service"

type srvAndCh struct {
	service  string
	ch       *tchannel.Channel
	provider *Provider
}

func TestRingpopProvider(t *testing.T) {
	t.Cleanup(func() {
		goleak.VerifyNone(t,
			// ignore the goroutines leaked by ringpop library
			goleak.IgnoreTopFunction("github.com/rcrowley/go-metrics.(*meterArbiter).tick"),
			goleak.IgnoreTopFunction("github.com/uber/ringpop-go.(*Ringpop).startTimers.func1"),
			goleak.IgnoreTopFunction("github.com/uber/ringpop-go.(*Ringpop).startTimers.func2"))
	})
	logger := testlogger.New(t)
	serviceName := "matching"

	matchingChs, cleanupMatchingChs, err := createAndListenChannels(serviceName, 3)
	t.Cleanup(cleanupMatchingChs)
	if err != nil {
		t.Fatalf("Failed to create and listen on channels: %v", err)
	}

	irrelevantChs, cleanupIrrelevantChs, err := createAndListenChannels("random-svc", 1)
	t.Cleanup(cleanupIrrelevantChs)
	if err != nil {
		t.Fatalf("Failed to create and listen on channels: %v", err)
	}

	allServicesAndChs := append(matchingChs, irrelevantChs...)
	cfg := ringpopproviderconfig.Config{
		BootstrapMode:   ringpopproviderconfig.BootstrapModeHosts,
		Name:            "ring",
		BootstrapHosts:  toHosts(t, allServicesAndChs),
		MaxJoinDuration: 10 * time.Second,
	}

	t.Logf("Config: %+v", cfg)

	// start ringpop provider for each channel
	var wg sync.WaitGroup
	for i, svc := range allServicesAndChs {
		svc := svc
		cfg := cfg
		if i == 0 {
			// set broadcast address for the first provider to test that path
			cfg.BroadcastAddress = "127.0.0.1"
		}
		p, err := New(svc.service, &cfg, svc.ch, membership.PortMap{}, logger)
		if err != nil {
			t.Fatalf("Failed to create ringpop provider: %v", err)
		}

		svc.provider = p

		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Start()
		}()
		t.Cleanup(p.Stop)
	}

	t.Logf("Waiting for %d ringpop providers to start", len(matchingChs)+len(irrelevantChs))
	wg.Wait()

	sleep := 5 * time.Second
	t.Logf("Sleeping for %d seconds for ring to update", int(sleep.Seconds()))
	time.Sleep(sleep)

	provider := matchingChs[0].provider
	hostInfo, err := provider.WhoAmI()
	if err != nil {
		t.Fatalf("Failed to get who am I: %v", err)
	}

	t.Logf("Who am I: %+v", hostInfo.GetAddress())

	members, err := provider.GetMembers(serviceName)
	if err != nil {
		t.Fatalf("Failed to get members: %v", err)
	}

	if len(members) != 3 {
		t.Fatalf("Expected 3 members, got %v", len(members))
	}

	// Evict one of the providers and make sure it's removed from the ring
	matchingChs[1].provider.SelfEvict()
	t.Logf("A peer is evicted. Sleeping for %d seconds for ring to update", int(sleep.Seconds()))
	time.Sleep(sleep)

	members, err = provider.GetMembers(serviceName)
	if err != nil {
		t.Fatalf("Failed to get members: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("Expected 2 members, got %v", len(members))
	}
}

func TestSubscribeAndNotify(t *testing.T) {
	provider := NewRingpopProvider(testServiceName, nil, nil, nil, testlogger.New(t))

	ringpopEvent := events.RingChangedEvent{
		ServersAdded:   []string{"aa", "bb", "cc"},
		ServersUpdated: []string{"dd"},
		ServersRemoved: []string{"ee", "ff"},
	}
	expectedEvent := membership.ChangedEvent{
		HostsAdded:   ringpopEvent.ServersAdded,
		HostsUpdated: ringpopEvent.ServersUpdated,
		HostsRemoved: ringpopEvent.ServersRemoved,
	}

	var calls1, calls2 int
	require.NoError(t,
		provider.Subscribe("subscriber1",
			func(ev membership.ChangedEvent) {
				calls1++
				assert.Equal(t, ev, expectedEvent)
			},
		))

	require.NoError(t,
		provider.Subscribe("subscriber2",
			func(ev membership.ChangedEvent) {
				calls2++
				assert.Equal(t, ev, expectedEvent)
			},
		))

	require.Error(t,
		provider.Subscribe(
			"subscriber2",
			func(membership.ChangedEvent) { t.Error("Should never be called") },
		),
		"Subscribe doesn't allow duplicate names",
	)

	provider.HandleEvent(ringpopEvent)
	assert.Equal(t, 1, calls1, "every subscriber must have been called once")
	assert.Equal(t, 1, calls2, "every subscriber must have been called once")
}

func createAndListenChannels(serviceName string, n int) ([]*srvAndCh, func(), error) {
	var res []*srvAndCh
	cleanupFn := func(srvs []*srvAndCh) func() {
		return func() {
			for _, s := range srvs {
				s.ch.Close()
			}
		}
	}
	for i := 0; i < n; i++ {
		ch, err := tchannel.NewChannel(serviceName, nil)
		if err != nil {
			return nil, cleanupFn(res), err
		}

		if err := ch.ListenAndServe("localhost:0"); err != nil {
			return nil, cleanupFn(res), err
		}

		res = append(res, &srvAndCh{service: serviceName, ch: ch})
	}
	return res, cleanupFn(res), nil
}

func toHosts(t *testing.T, allServicesAndChs []*srvAndCh) []string {
	var hosts []string
	for _, svc := range allServicesAndChs {
		if svc.ch.PeerInfo().IsEphemeralHostPort() {
			t.Fatalf("Channel %v is not listening on a port", svc.ch)
		}
		hosts = append(hosts, svc.ch.PeerInfo().HostPort)
	}
	return hosts
}

func TestLabelToPort(t *testing.T) {
	tests := []struct {
		label   string
		want    uint16
		wantErr bool
	}{
		{
			label: "0",
			want:  0,
		},
		{
			label: "1234",
			want:  1234,
		},
		{
			label:   "-1",
			wantErr: true,
		},
		{
			label: "32768",
			want:  32768,
		},
		{
			label: "65535",
			want:  65535,
		},
		{
			label:   "65536", // greater than max uint16 (2^16-1)
			wantErr: true,
		},
	}

	for _, tc := range tests {
		got, err := labelToPort(tc.label)
		if got != tc.want || (err != nil) != tc.wantErr {
			t.Errorf("labelToPort(%v) = %v, %v; want %v, %v", tc.label, got, err, tc.want, tc.wantErr)
		}
	}
}
