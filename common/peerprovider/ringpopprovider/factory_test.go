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

	provider := newDNSProvider(
		cfg.BootstrapHosts,
		&mockResolver{
			Hosts: map[string][]string{"example.net": []string{"10.0.0.0", "10.0.0.1"}},
		},
		logger,
	)
	cfg.DiscoveryProvider = provider
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

	hostports, err := cfg.DiscoveryProvider.Hosts()
	assert.Nil(t, err)
	assert.ElementsMatch(
		t,
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
	assert.Nil(t, hostports)
	assert.NotNil(t, err, "error should be returned when no hosts")
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
		},
		logger,
	)
	cfg.DiscoveryProvider = provider
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

	// Expect unknown-duplicate.example.net to not resolve
	_, err = cfg.DiscoveryProvider.Hosts()
	assert.NotNil(t, err)

	// Remove known bad hosts from Unresolved list
	provider.UnresolvedHosts = []string{
		"service-a.example.net",
		"service-b.example.net",
		"badhostport",
	}

	// Expect badhostport to not seperate service name
	_, err = cfg.DiscoveryProvider.Hosts()
	assert.NotNil(t, err)

	// Remove known bad hosts from Unresolved list
	provider.UnresolvedHosts = []string{
		"service-a.example.net",
		"service-b.example.net",
	}

	hostports, err := cfg.DiscoveryProvider.Hosts()
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
		"duplicate entries should be removed",
	)

	cfg.DiscoveryProvider = newDNSProvider(
		cfg.BootstrapHosts,
		&mockResolver{Hosts: map[string][]string{}},
		logger,
	)
	hostports, err = cfg.DiscoveryProvider.Hosts()
	assert.Nil(t, hostports)
	assert.NotNil(t, err, "error should be returned when no hosts")
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
