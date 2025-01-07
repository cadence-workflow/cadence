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
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/uber/ringpop-go/discovery"
	"github.com/uber/ringpop-go/discovery/jsonfile"
	"github.com/uber/ringpop-go/discovery/statichosts"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

// BootstrapMode is an enum type for ringpop bootstrap mode
type BootstrapMode int

const (
	// BootstrapModeNone represents a bootstrap mode set to nothing or invalid
	BootstrapModeNone BootstrapMode = iota
	// BootstrapModeFile represents a file-based bootstrap mode
	BootstrapModeFile
	// BootstrapModeHosts represents a list of hosts passed in the configuration
	BootstrapModeHosts
	// BootstrapModeCustom represents a custom bootstrap mode
	BootstrapModeCustom
	// BootstrapModeDNS represents a list of hosts passed in the configuration
	// to be resolved, and the resulting addresses are used for bootstrap
	BootstrapModeDNS
	// BootstrapModeDNSSRV represents a list of DNS hosts passed in the configuration
	// to resolve secondary addresses that DNS SRV record would return resulting in
	// a host list that will contain multiple dynamic addresses and their unique ports
	BootstrapModeDNSSRV
)

const (
	defaultMaxJoinDuration = 10 * time.Second
)

// Config contains the ringpop config items
type Config struct {
	// Name to be used in ringpop advertisement
	Name string `yaml:"name" validate:"nonzero"`
	// BroadcastAddress is communicated with peers to connect to this container/host.
	// This is useful when running cadence in K8s and the containers need to listen on 0.0.0.0 but advertise their pod IP to peers.
	// If not set, the listen address will be used as broadcast address.
	BroadcastAddress string `yaml:"broadcastAddress"`
	// BootstrapMode is a enum that defines the ringpop bootstrap method, currently supports: hosts, files, custom, dns, and dns-srv
	BootstrapMode BootstrapMode `yaml:"bootstrapMode"`
	// BootstrapHosts is a list of seed hosts to be used for ringpop bootstrap
	BootstrapHosts []string `yaml:"bootstrapHosts"`
	// BootstrapFile is the file path to be used for ringpop bootstrap
	BootstrapFile string `yaml:"bootstrapFile"`
	// MaxJoinDuration is the max wait time to join the ring
	MaxJoinDuration time.Duration `yaml:"maxJoinDuration"`
	// Custom discovery provider, cannot be specified through yaml
	DiscoveryProvider discovery.DiscoverProvider `yaml:"-"`
}

func (rpConfig *Config) validate() error {
	if len(rpConfig.Name) == 0 {
		return fmt.Errorf("ringpop config missing `name` param")
	}

	if rpConfig.MaxJoinDuration == 0 {
		rpConfig.MaxJoinDuration = defaultMaxJoinDuration
	}

	return validateBootstrapMode(rpConfig)
}

// UnmarshalYAML is called by the yaml package to convert
// the config YAML into a BootstrapMode.
func (m *BootstrapMode) UnmarshalYAML(
	unmarshal func(interface{}) error,
) error {

	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	var err error
	*m, err = parseBootstrapMode(s)
	return err
}

// parseBootstrapMode reads a string value and returns a bootstrap mode.
func parseBootstrapMode(
	mode string,
) (BootstrapMode, error) {

	switch strings.ToLower(mode) {
	case "hosts":
		return BootstrapModeHosts, nil
	case "file":
		return BootstrapModeFile, nil
	case "custom":
		return BootstrapModeCustom, nil
	case "dns":
		return BootstrapModeDNS, nil
	case "dns-srv":
		return BootstrapModeDNSSRV, nil
	}
	return BootstrapModeNone, fmt.Errorf("invalid ringpop bootstrap mode %q", mode)
}

func validateBootstrapMode(
	rpConfig *Config,
) error {

	switch rpConfig.BootstrapMode {
	case BootstrapModeFile:
		if len(rpConfig.BootstrapFile) == 0 {
			return fmt.Errorf("ringpop config missing bootstrap file param")
		}
	case BootstrapModeHosts, BootstrapModeDNS, BootstrapModeDNSSRV:
		if len(rpConfig.BootstrapHosts) == 0 {
			return fmt.Errorf("ringpop config missing boostrap hosts param")
		}
	case BootstrapModeCustom:
		if rpConfig.DiscoveryProvider == nil {
			return fmt.Errorf("ringpop bootstrapMode is set to custom but discoveryProvider is nil")
		}
	default:
		return fmt.Errorf("ringpop config with unknown boostrap mode %q", rpConfig.BootstrapMode)
	}
	return nil
}

type dnsHostResolver interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
	LookupSRV(ctx context.Context, service, proto, name string) (cname string, addrs []*net.SRV, err error)
}

type dnsProvider struct {
	UnresolvedHosts []string
	Resolver        dnsHostResolver
	Logger          log.Logger
}

func newDNSProvider(
	hosts []string,
	resolver dnsHostResolver,
	logger log.Logger,
) *dnsProvider {

	set := map[string]struct{}{}
	for _, hostport := range hosts {
		set[hostport] = struct{}{}
	}

	var keys []string
	for key := range set {
		keys = append(keys, key)
	}
	return &dnsProvider{
		UnresolvedHosts: keys,
		Resolver:        resolver,
		Logger:          logger,
	}
}

func (provider *dnsProvider) Hosts() ([]string, error) {
	var results []string
	resolvedHosts := map[string][]string{}
	for _, hostPort := range provider.UnresolvedHosts {
		host, port, err := net.SplitHostPort(hostPort)
		if err != nil {
			provider.Logger.Warn("could not split host and port", tag.Address(hostPort), tag.Error(err))
			continue
		}

		resolved, exists := resolvedHosts[host]
		if !exists {
			resolved, err = provider.Resolver.LookupHost(context.Background(), host)
			if err != nil {
				provider.Logger.Warn("could not resolve host", tag.Address(host), tag.Error(err))
				continue
			}
			resolvedHosts[host] = resolved
		}
		for _, r := range resolved {
			results = append(results, net.JoinHostPort(r, port))
		}
	}
	if len(results) == 0 {
		return nil, errors.New("no hosts found, and bootstrap requires at least one")
	}
	return results, nil
}

type dnsSRVProvider struct {
	UnresolvedHosts []string
	Resolver        dnsHostResolver
	Logger          log.Logger
}

func newDNSSRVProvider(
	hosts []string,
	resolver dnsHostResolver,
	logger log.Logger,
) *dnsSRVProvider {

	set := map[string]struct{}{}
	for _, hostport := range hosts {
		set[hostport] = struct{}{}
	}

	var keys []string
	for key := range set {
		keys = append(keys, key)
	}

	return &dnsSRVProvider{
		UnresolvedHosts: keys,
		Resolver:        resolver,
		Logger:          logger,
	}
}

func (provider *dnsSRVProvider) Hosts() ([]string, error) {
	var results []string
	resolvedHosts := map[string][]string{}

	for _, host := range provider.UnresolvedHosts {
		hostParts := strings.Split(host, ".")
		if len(hostParts) <= 2 {
			return nil, fmt.Errorf("could not seperate host from domain %q", host)
		}
		serviceName := hostParts[0]
		domain := strings.Join(hostParts[1:], ".")
		resolved, exists := resolvedHosts[serviceName]
		if !exists {
			_, srvs, err := provider.Resolver.LookupSRV(context.Background(), serviceName, "tcp", domain)

			if err != nil {
				return nil, fmt.Errorf("could not resolve host: %s.%s", serviceName, domain)
			}

			var targets []string
			for _, s := range srvs {
				addrs, err := provider.Resolver.LookupHost(context.Background(), s.Target)

				if err != nil {
					provider.Logger.Warn("could not resolve srv dns host", tag.Address(s.Target), tag.Error(err))
					continue
				}
				for _, a := range addrs {
					targets = append(targets, net.JoinHostPort(a, fmt.Sprintf("%d", s.Port)))
				}
			}
			resolvedHosts[serviceName] = targets
			resolved = targets
		}

		results = append(results, resolved...)
	}

	if len(results) == 0 {
		return nil, errors.New("no hosts found, and bootstrap requires at least one")
	}
	return results, nil
}

func newDiscoveryProvider(
	cfg *Config,
	logger log.Logger,
) (discovery.DiscoverProvider, error) {

	if cfg.DiscoveryProvider != nil {
		// custom discovery provider takes first precedence
		return cfg.DiscoveryProvider, nil
	}

	switch cfg.BootstrapMode {
	case BootstrapModeHosts:
		return statichosts.New(cfg.BootstrapHosts...), nil
	case BootstrapModeFile:
		return jsonfile.New(cfg.BootstrapFile), nil
	case BootstrapModeDNS:
		return newDNSProvider(cfg.BootstrapHosts, net.DefaultResolver, logger), nil
	case BootstrapModeDNSSRV:
		return newDNSSRVProvider(cfg.BootstrapHosts, net.DefaultResolver, logger), nil
	}
	return nil, fmt.Errorf("unknown bootstrap mode %q", cfg.BootstrapMode)
}
