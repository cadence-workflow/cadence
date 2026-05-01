package ringpopprovider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/uber/ringpop-go/discovery"
	"github.com/uber/ringpop-go/discovery/jsonfile"
	"github.com/uber/ringpop-go/discovery/statichosts"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/peerprovider/ringpopprovider/config"
)

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
	cfg *config.Config,
	logger log.Logger,
) (discovery.DiscoverProvider, error) {

	if cfg.DiscoveryProvider != nil {
		// custom discovery provider takes first precedence
		return cfg.DiscoveryProvider, nil
	}

	switch cfg.BootstrapMode {
	case config.BootstrapModeHosts:
		return statichosts.New(cfg.BootstrapHosts...), nil
	case config.BootstrapModeFile:
		return jsonfile.New(cfg.BootstrapFile), nil
	case config.BootstrapModeDNS:
		return newDNSProvider(cfg.BootstrapHosts, net.DefaultResolver, logger), nil
	case config.BootstrapModeDNSSRV:
		return newDNSSRVProvider(cfg.BootstrapHosts, net.DefaultResolver, logger), nil
	}
	return nil, fmt.Errorf("unknown bootstrap mode %q", cfg.BootstrapMode)
}
