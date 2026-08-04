package ringpopprovider

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common/log/testlogger"
	ringpopproviderconfig "github.com/uber/cadence/common/peerprovider/ringpopprovider/config"
)

type mockResolver struct {
	Hosts map[string][]string
	SRV   map[string][]net.SRV
}

func (resolver *mockResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	addrs, ok := resolver.Hosts[host]
	if !ok {
		// Return a DNSError with IsNotFound to test that code path
		if host == "notfound.example.net" {
			return nil, &net.DNSError{
				Err:        "no such host",
				Name:       host,
				IsNotFound: true,
			}
		}
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

func TestDNSMode(t *testing.T) {
	var cfg ringpopproviderconfig.Config
	err := yaml.Unmarshal([]byte(getDNSConfig()), &cfg)
	assert.Nil(t, err)
	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, ringpopproviderconfig.BootstrapModeDNS, cfg.BootstrapMode)
	assert.Equal(t, "10.66.1.71", cfg.BroadcastAddress)
	assert.Nil(t, cfg.Validate())
	logger := testlogger.New(t)

	assert.ElementsMatch(
		t,
		[]string{
			"example.net:1111",
			"example.net:1112",
			"unknown-duplicate.example.net:1111",
			"unknown-duplicate.example.net:1111",
			"badhostport",
		},
		cfg.BootstrapHosts,
	)

	// Test deduplication of bootstrap hosts
	provider := newDNSProvider(
		cfg.BootstrapHosts,
		&mockResolver{
			Hosts: map[string][]string{"example.net": []string{"10.0.0.0", "10.0.0.1"}},
		},
		logger,
	)
	assert.ElementsMatch(
		t,
		[]string{
			"example.net:1111",
			"example.net:1112",
			"unknown-duplicate.example.net:1111",
			"badhostport",
		},
		provider.UnresolvedHosts,
		"duplicate entries should be removed",
	)

	// Test successful resolution with valid hosts
	provider = newDNSProvider(
		[]string{"example.net:1111", "example.net:1112"},
		&mockResolver{
			Hosts: map[string][]string{"example.net": []string{"10.0.0.0", "10.0.0.1"}},
		},
		logger,
	)
	hostports, err := provider.Hosts()
	assert.Nil(t, err)
	assert.ElementsMatch(
		t,
		[]string{
			"10.0.0.0:1111", "10.0.0.1:1111",
			"10.0.0.0:1112", "10.0.0.1:1112",
		},
		hostports,
	)

	// Test error when host has no port (SplitHostPort fails)
	provider = newDNSProvider(
		[]string{"badhostport"},
		&mockResolver{Hosts: map[string][]string{}},
		logger,
	)
	_, err = provider.Hosts()
	assert.NotNil(t, err, "should return error for malformed host:port")

	// Test error when DNS resolution fails (not IsNotFound)
	provider = newDNSProvider(
		[]string{"unknown.example.net:1111"},
		&mockResolver{Hosts: map[string][]string{}},
		logger,
	)
	_, err = provider.Hosts()
	assert.NotNil(t, err, "should return error when DNS resolution fails")

	// Test DNS IsNotFound case - should continue and return empty
	provider = newDNSProvider(
		[]string{"notfound.example.net:1111"},
		&mockResolver{Hosts: map[string][]string{}},
		logger,
	)
	hostports, err = provider.Hosts()
	assert.Nil(t, err, "should not return error for DNS not found")
	assert.Empty(t, hostports, "should return empty list when DNS not found")

	// Test DNS IsNotFound mixed with successful resolution
	provider = newDNSProvider(
		[]string{"notfound.example.net:1111", "example.net:1112"},
		&mockResolver{Hosts: map[string][]string{"example.net": []string{"10.0.0.1"}}},
		logger,
	)
	hostports, err = provider.Hosts()
	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{"10.0.0.1:1112"}, hostports, "should skip not found and return resolved")

	// Test empty result when DNS returns IsNotFound for all hosts
	provider = newDNSProvider(
		[]string{"example.net:1111"},
		&mockResolver{Hosts: map[string][]string{"example.net": []string{}}},
		logger,
	)
	hostports, err = provider.Hosts()
	assert.Nil(t, err)
	assert.Empty(t, hostports, "should return empty list when DNS returns empty")
}

func TestDNSSRVMode(t *testing.T) {
	var cfg ringpopproviderconfig.Config
	err := yaml.Unmarshal([]byte(getDNSSRVConfig()), &cfg)
	assert.Nil(t, err)
	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, ringpopproviderconfig.BootstrapModeDNSSRV, cfg.BootstrapMode)
	assert.Nil(t, cfg.Validate())
	logger := testlogger.New(t)

	assert.ElementsMatch(
		t,
		[]string{
			"service-a.example.net",
			"service-b.example.net",
			"unknown-duplicate.example.net",
			"unknown-duplicate.example.net",
			"badhostport",
		},
		cfg.BootstrapHosts,
	)

	// Test deduplication
	provider := newDNSSRVProvider(
		cfg.BootstrapHosts,
		&mockResolver{
			SRV: map[string][]net.SRV{
				"service-a": []net.SRV{{Target: "az1-service-a.addr.example.net", Port: 7755}},
			},
			Hosts: map[string][]string{
				"az1-service-a.addr.example.net": []string{"10.0.0.1"},
			},
		},
		logger,
	)
	assert.ElementsMatch(
		t,
		[]string{
			"service-a.example.net",
			"service-b.example.net",
			"unknown-duplicate.example.net",
			"badhostport",
		},
		provider.UnresolvedHosts,
		"duplicate entries should be removed",
	)

	// Test successful SRV resolution
	provider = newDNSSRVProvider(
		[]string{"service-a.example.net", "service-b.example.net"},
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
		},
		logger,
	)
	hostports, err := provider.Hosts()
	assert.Nil(t, err)
	assert.ElementsMatch(
		t,
		[]string{
			"10.0.0.1:7755",
			"10.0.2.0:7566", "10.0.2.3:7566",
			"10.0.3.0:7788", "10.0.3.3:7788",
			"10.0.3.1:7896",
		},
		hostports,
	)

	// Test error when SRV lookup fails
	provider = newDNSSRVProvider(
		[]string{"unknown-duplicate.example.net"},
		&mockResolver{
			SRV:   map[string][]net.SRV{},
			Hosts: map[string][]string{},
		},
		logger,
	)
	_, err = provider.Hosts()
	assert.NotNil(t, err, "should return error when SRV lookup fails")

	// Test error when hostname cannot be separated (less than 3 parts)
	provider = newDNSSRVProvider(
		[]string{"badhostport"},
		&mockResolver{
			SRV:   map[string][]net.SRV{},
			Hosts: map[string][]string{},
		},
		logger,
	)
	_, err = provider.Hosts()
	assert.NotNil(t, err, "should return error for malformed hostname")

	// Test error when individual SRV target fails to resolve
	provider = newDNSSRVProvider(
		[]string{"service-a.example.net"},
		&mockResolver{
			SRV: map[string][]net.SRV{
				"service-a": []net.SRV{
					{Target: "bad.addr.example.net", Port: 7755},
				},
			},
			Hosts: map[string][]string{},
		},
		logger,
	)
	_, err = provider.Hosts()
	assert.NotNil(t, err, "should return error when SRV target fails to resolve")

	// Test DNS IsNotFound for SRV target - should skip and continue
	provider = newDNSSRVProvider(
		[]string{"service-a.example.net"},
		&mockResolver{
			SRV: map[string][]net.SRV{
				"service-a": []net.SRV{
					{Target: "notfound.example.net", Port: 7755},
					{Target: "good.addr.example.net", Port: 7756},
				},
			},
			Hosts: map[string][]string{
				"good.addr.example.net": []string{"10.0.0.1"},
			},
		},
		logger,
	)
	hostports, err = provider.Hosts()
	assert.Nil(t, err, "should skip not found SRV targets")
	assert.ElementsMatch(t, []string{"10.0.0.1:7756"}, hostports, "should return resolved targets")

	// Test all SRV targets are DNS not found - should return empty
	provider = newDNSSRVProvider(
		[]string{"service-a.example.net"},
		&mockResolver{
			SRV: map[string][]net.SRV{
				"service-a": []net.SRV{
					{Target: "notfound.example.net", Port: 7755},
				},
			},
			Hosts: map[string][]string{},
		},
		logger,
	)
	hostports, err = provider.Hosts()
	assert.Nil(t, err)
	assert.Empty(t, hostports, "should return empty when all SRV targets not found")
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
