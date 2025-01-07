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

package ringpopprovider

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/discovery/statichosts"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common/log/testlogger"
)

type RingpopSuite struct {
	*require.Assertions
	suite.Suite
}

func TestRingpopSuite(t *testing.T) {
	suite.Run(t, new(RingpopSuite))
}

func (s *RingpopSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *RingpopSuite) TestHostsMode() {
	var cfg Config
	err := yaml.Unmarshal([]byte(getHostsConfig()), &cfg)
	s.Nil(err)
	s.Equal("test", cfg.Name)
	s.Equal(BootstrapModeHosts, cfg.BootstrapMode)
	s.Equal([]string{"127.0.0.1:1111"}, cfg.BootstrapHosts)
	s.Equal(time.Second*30, cfg.MaxJoinDuration)
	err = cfg.validate()
	s.Nil(err)
}

func (s *RingpopSuite) TestFileMode() {
	var cfg Config
	err := yaml.Unmarshal([]byte(getJSONConfig()), &cfg)
	s.Nil(err)
	s.Equal("test", cfg.Name)
	s.Equal(BootstrapModeFile, cfg.BootstrapMode)
	s.Equal("/tmp/file.json", cfg.BootstrapFile)
	s.Equal(time.Second*30, cfg.MaxJoinDuration)
	err = cfg.validate()
	s.Nil(err)
}

func (s *RingpopSuite) TestCustomMode() {
	var cfg Config
	err := yaml.Unmarshal([]byte(getCustomConfig()), &cfg)
	s.Nil(err)
	s.Equal("test", cfg.Name)
	s.Equal(BootstrapModeCustom, cfg.BootstrapMode)
	s.NotNil(cfg.validate())
	cfg.DiscoveryProvider = statichosts.New("127.0.0.1")
	s.Nil(cfg.validate())
}

type mockResolver struct {
	Hosts map[string][]string
	SRV   map[string][]net.SRV
	suite *RingpopSuite
}

func (resolver *mockResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	addrs, ok := resolver.Hosts[host]
	if !ok {
		return nil, fmt.Errorf("Host was not resolved: %s", host)
	}
	return addrs, nil
}

func (resolver *mockResolver) LookupSRV(ctx context.Context, service string, proto string, name string) (string, []*net.SRV, error) {
	var records []*net.SRV
	srvs, ok := resolver.SRV[service]
	if !ok {
		return "", nil, fmt.Errorf("Host was not resolved: %s", service)
	}

	for _, record := range srvs {
		srvRecord := record
		records = append(records, &srvRecord)
	}

	return "test", records, nil
}

func (s *RingpopSuite) TestDNSMode() {
	var cfg Config
	err := yaml.Unmarshal([]byte(getDNSConfig()), &cfg)
	s.Nil(err)
	s.Equal("test", cfg.Name)
	s.Equal(BootstrapModeDNS, cfg.BootstrapMode)
	s.Equal("10.66.1.71", cfg.BroadcastAddress)
	s.Nil(cfg.validate())
	logger := testlogger.New(s.T())

	s.ElementsMatch(
		[]string{
			"example.net:1111",
			"example.net:1112",
			"unknown-duplicate.example.net:1111",
			"unknown-duplicate.example.net:1111",
			"badhostport",
		},
		cfg.BootstrapHosts,
	)

	provider := newDNSProvider(
		cfg.BootstrapHosts,
		&mockResolver{
			Hosts: map[string][]string{"example.net": []string{"10.0.0.0", "10.0.0.1"}},
		},
		logger,
	)
	cfg.DiscoveryProvider = provider
	s.ElementsMatch(
		[]string{
			"example.net:1111",
			"example.net:1112",
			"unknown-duplicate.example.net:1111",
			"badhostport",
		},
		provider.UnresolvedHosts,
		"duplicate entries should be removed",
	)

	hostports, err := cfg.DiscoveryProvider.Hosts()
	s.Nil(err)
	s.ElementsMatch(
		[]string{
			"10.0.0.0:1111", "10.0.0.1:1111",
			"10.0.0.0:1112", "10.0.0.1:1112",
		},
		hostports,
	)

	cfg.DiscoveryProvider = newDNSProvider(
		cfg.BootstrapHosts,
		&mockResolver{Hosts: map[string][]string{}},
		logger,
	)
	hostports, err = cfg.DiscoveryProvider.Hosts()
	s.Nil(hostports)
	s.NotNil(err, "error should be returned when no hosts")
}

func (s *RingpopSuite) TestDNSSRVMode() {
	var cfg Config
	err := yaml.Unmarshal([]byte(getDNSSRVConfig()), &cfg)
	s.Nil(err)
	s.Equal("test", cfg.Name)
	s.Equal(BootstrapModeDNSSRV, cfg.BootstrapMode)
	s.Nil(cfg.validate())
	logger := testlogger.New(s.T())

	s.ElementsMatch(
		[]string{
			"service-a.example.net",
			"service-b.example.net",
			"unknown-duplicate.example.net",
			"unknown-duplicate.example.net",
			"badhostport",
		},
		cfg.BootstrapHosts,
	)

	provider := newDNSSRVProvider(
		cfg.BootstrapHosts,
		&mockResolver{
			SRV: map[string][]net.SRV{
				"service-a": []net.SRV{{Target: "az1-service-a.addr.example.net", Port: 7755}, {Target: "az2-service-a.addr.example.net", Port: 7566}},
				"service-b": []net.SRV{{Target: "az1-service-b.addr.example.net", Port: 7788}, {Target: "az2-service-b.addr.example.net", Port: 7896}},
			},
			Hosts: map[string][]string{
				"az1-service-a.addr.example.net": []string{"10.0.0.1"},
				"az2-service-a.addr.example.net": []string{"10.0.2.0", "10.0.2.3"},
				"az1-service-b.addr.example.net": []string{"10.0.3.0", "10.0.3.3"},
				"az2-service-b.addr.example.net": []string{"10.0.3.1"},
			},
			suite: s,
		},
		logger,
	)
	cfg.DiscoveryProvider = provider
	s.ElementsMatch(
		[]string{
			"service-a.example.net",
			"service-b.example.net",
			"unknown-duplicate.example.net",
			"badhostport",
		},
		provider.UnresolvedHosts,
		"duplicate entries should be removed",
	)

	// Expect unknown-duplicate.example.net to not resolve
	_, err = cfg.DiscoveryProvider.Hosts()
	s.NotNil(err)

	// Remove known bad hosts from Unresolved list
	provider.UnresolvedHosts = []string{
		"service-a.example.net",
		"service-b.example.net",
		"badhostport",
	}

	// Expect badhostport to not seperate service name
	_, err = cfg.DiscoveryProvider.Hosts()
	s.NotNil(err)

	// Remove known bad hosts from Unresolved list
	provider.UnresolvedHosts = []string{
		"service-a.example.net",
		"service-b.example.net",
	}

	hostports, err := cfg.DiscoveryProvider.Hosts()
	s.Nil(err)
	s.ElementsMatch(
		[]string{
			"10.0.0.1:7755",
			"10.0.2.0:7566", "10.0.2.3:7566",
			"10.0.3.0:7788", "10.0.3.3:7788",
			"10.0.3.1:7896",
		},
		hostports,
	)

	cfg.DiscoveryProvider = newDNSProvider(
		cfg.BootstrapHosts,
		&mockResolver{Hosts: map[string][]string{}},
		logger,
	)
	hostports, err = cfg.DiscoveryProvider.Hosts()
	s.Nil(hostports)
	s.NotNil(err, "error should be returned when no hosts")
}

func (s *RingpopSuite) TestInvalidConfig() {
	var cfg Config
	s.NotNil(cfg.validate())
	cfg.Name = "test"
	s.NotNil(cfg.validate())
	cfg.BootstrapMode = BootstrapModeNone
	s.NotNil(cfg.validate())
	_, err := parseBootstrapMode("unknown")
	s.NotNil(err)
}

func getJSONConfig() string {
	return `name: "test"
bootstrapMode: "file"
bootstrapFile: "/tmp/file.json"
maxJoinDuration: 30s`
}

func getHostsConfig() string {
	return `name: "test"
bootstrapMode: "hosts"
bootstrapHosts: ["127.0.0.1:1111"]
maxJoinDuration: 30s`
}

func getCustomConfig() string {
	return `name: "test"
bootstrapMode: "custom"
maxJoinDuration: 30s`
}

func getDNSConfig() string {
	return `name: "test"
bootstrapMode: "dns"
broadcastAddress: "10.66.1.71"
bootstrapHosts:
- example.net:1111
- example.net:1112
- unknown-duplicate.example.net:1111
- unknown-duplicate.example.net:1111
- badhostport
maxJoinDuration: 30s`
}

func getDNSSRVConfig() string {
	return `name: "test"
bootstrapMode: "dns-srv"
bootstrapHosts:
- service-a.example.net
- service-b.example.net
- unknown-duplicate.example.net
- unknown-duplicate.example.net
- badhostport
maxJoinDuration: 30s`
}
