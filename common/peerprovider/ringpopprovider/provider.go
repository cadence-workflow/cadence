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
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/events"
	rpmembership "github.com/uber/ringpop-go/membership"
	"github.com/uber/ringpop-go/swim"
	tcg "github.com/uber/tchannel-go"
	"go.uber.org/yarpc/transport/tchannel"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	ringpopproviderconfig "github.com/uber/cadence/common/peerprovider/ringpopprovider/config"
)

type (
	// Provider use ringpop to announce membership changes
	Provider struct {
		status      int32
		service     string
		ringpop     *ringpop.Ringpop
		bootParams  *swim.BootstrapOptions
		logger      log.Logger
		portmap     membership.PortMap
		mu          sync.RWMutex
		subscribers map[string]func(membership.ChangedEvent)
	}
)

const (
	// roleKey label is set by every single service as soon as it bootstraps its
	// ringpop instance. The data for this key is the service name
	roleKey = "serviceName"
)

var _ membership.PeerProvider = (*Provider)(nil)

func New(
	service string,
	config *ringpopproviderconfig.Config,
	channel tchannel.Channel,
	portMap membership.PortMap,
	logger log.Logger,
) (*Provider, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	discoveryProvider, err := newDiscoveryProvider(config, logger)
	if err != nil {
		return nil, fmt.Errorf("ringpop discovery provider: %w", err)
	}

	bootstrapOpts := &swim.BootstrapOptions{
		MaxJoinDuration:  config.MaxJoinDuration,
		DiscoverProvider: discoveryProvider,
	}

	ch := channel.(*tcg.Channel)
	opts := []ringpop.Option{ringpop.Channel(ch)}
	if config.BroadcastAddress != "" {
		broadcastIP := net.ParseIP(config.BroadcastAddress)
		if broadcastIP == nil {
			return nil, fmt.Errorf("failed parsing broadcast address %q, err: %w", config.BroadcastAddress, err)
		}
		logger.Info("Initializing ringpop with custom broadcast address", tag.Address(broadcastIP.String()))
		opts = append(opts, ringpop.AddressResolverFunc(broadcastAddrResolver(ch, broadcastIP)))
	}
	rp, err := ringpop.New(config.Name, opts...)
	if err != nil {
		return nil, fmt.Errorf("ringpop instance creation: %w", err)
	}

	return NewRingpopProvider(service, rp, portMap, bootstrapOpts, logger), nil
}

// NewRingpopProvider sets up ringpop based peer provider
func NewRingpopProvider(
	service string,
	rp *ringpop.Ringpop,
	portMap membership.PortMap,
	bootstrapOpts *swim.BootstrapOptions,
	logger log.Logger,
) *Provider {
	return &Provider{
		service:     service,
		status:      common.DaemonStatusInitialized,
		bootParams:  bootstrapOpts,
		logger:      logger,
		portmap:     portMap,
		ringpop:     rp,
		subscribers: map[string]func(membership.ChangedEvent){},
	}
}

// Start starts ringpop
func (r *Provider) Start() {
	if !atomic.CompareAndSwapInt32(
		&r.status,
		common.DaemonStatusInitialized,
		common.DaemonStatusStarted,
	) {
		return
	}

	_, err := r.ringpop.Bootstrap(r.bootParams)
	if err != nil {
		r.logger.Fatal("unable to bootstrap ringpop", tag.Error(err))
	}

	// Get updates from ringpop ring
	r.ringpop.AddListener(r)

	labels, err := r.ringpop.Labels()
	if err != nil {
		r.logger.Fatal("unable to get ring pop labels", tag.Error(err))
	}

	// set port labels
	for name, port := range r.portmap {
		if err = labels.Set(name, strconv.Itoa(int(port))); err != nil {
			r.logger.Fatal("unable to set port label", tag.Error(err))
		}
	}

	if err = labels.Set(roleKey, r.service); err != nil {
		r.logger.Fatal("unable to set ringpop role label", tag.Error(err))
	}
}

// HandleEvent handles updates from ringpop
func (r *Provider) HandleEvent(event events.Event) {
	// We only care about RingChangedEvent
	e, ok := event.(events.RingChangedEvent)
	if !ok {
		return
	}

	change := membership.ChangedEvent{
		HostsAdded:   e.ServersAdded,
		HostsUpdated: e.ServersUpdated,
		HostsRemoved: e.ServersRemoved,
	}
	r.logger.Info("Received a ringpop ring changed event", tag.MembershipChangeEvent(change))
	r.notifySubscribers(change)
}

func (r *Provider) notifySubscribers(event membership.ChangedEvent) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, handler := range r.subscribers {
		handler(event)
	}
}

func (r *Provider) SelfEvict() error {
	return r.ringpop.SelfEvict()
}

// GetMembers returns all hosts with a specified role value
func (r *Provider) GetMembers(service string) ([]membership.HostInfo, error) {
	var res []membership.HostInfo

	// filter member by service name, add port info to Hostinfo if they are present
	memberData := func(member swim.Member) bool {
		if v, ok := member.Label(roleKey); !ok || v != service {
			return false
		}

		// replicating swim member isReachable() method here as we are skipping other predicates
		if member.Status != swim.Alive && member.Status != swim.Suspect {
			return false
		}

		portMap := make(membership.PortMap)
		if v, ok := member.Label(membership.PortTchannel); ok {
			port, err := labelToPort(v)
			if err != nil {
				r.logger.Warn("tchannel port cannot be converted", tag.Error(err), tag.Value(v))
			} else {
				portMap[membership.PortTchannel] = port
			}
		} else {
			// for backwards compatibility: get tchannel port from member address
			_, port, err := net.SplitHostPort(member.Address)
			if err != nil {
				r.logger.Warn("getting ringpop member port from address", tag.Value(member.Address), tag.Error(err))
			} else {
				tchannelPort, err := labelToPort(port)
				if err != nil {
					r.logger.Warn("tchannel port cannot be converted", tag.Error(err), tag.Value(port))
				} else {
					portMap[membership.PortTchannel] = tchannelPort
				}
			}

		}

		if v, ok := member.Label(membership.PortGRPC); ok {
			port, err := labelToPort(v)
			if err != nil {
				r.logger.Warn("grpc port cannot be converted", tag.Error(err), tag.Value(v))
			} else {
				portMap[membership.PortGRPC] = port
			}
		}

		res = append(res, membership.NewDetailedHostInfo(member.GetAddress(), member.Identity(), portMap))

		return true
	}
	_, err := r.ringpop.GetReachableMembers(memberData)
	if err != nil {
		return nil, fmt.Errorf("ringpop get members: %w", err)
	}

	return res, nil
}

// WhoAmI returns address of this instance
func (r *Provider) WhoAmI() (membership.HostInfo, error) {
	address, err := r.ringpop.WhoAmI()
	if err != nil {
		return membership.HostInfo{}, fmt.Errorf("ringpop doesn't know Who Am I: %w", err)
	}

	labels, err := r.ringpop.Labels()
	if err != nil {
		return membership.HostInfo{}, fmt.Errorf("getting ringpop labels: %w", err)
	}

	hostIdentity := address
	// this is needed to in a situation when Cadence is trying to identify the owner for a key
	// make sure we are comparing identities, but not host:port pairs
	rpIdentity, set := labels.Get(rpmembership.IdentityLabelKey)
	if set {
		hostIdentity = rpIdentity
	}

	return membership.NewDetailedHostInfo(address, hostIdentity, r.portmap), nil
}

// Stop stops ringpop
func (r *Provider) Stop() {
	if !atomic.CompareAndSwapInt32(
		&r.status,
		common.DaemonStatusStarted,
		common.DaemonStatusStopped,
	) {
		return
	}

	r.ringpop.Destroy()
}

// Subscribe allows to be subscribed for ring changes
func (r *Provider) Subscribe(name string, handler func(membership.ChangedEvent)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.subscribers[name]
	if ok {
		return fmt.Errorf("%q already subscribed to ringpop provider", name)
	}

	r.subscribers[name] = handler
	return nil
}

func labelToPort(label string) (uint16, error) {
	port, err := strconv.ParseUint(label, 10, 16)
	if err != nil {
		return 0, err
	}
	return uint16(port), nil
}

func broadcastAddrResolver(ch *tcg.Channel, broadcastIP net.IP) func() (string, error) {
	return func() (string, error) {
		peerInfo := ch.PeerInfo()
		if peerInfo.IsEphemeralHostPort() {
			// not listening yet
			return "", ringpop.ErrEphemeralAddress
		}

		_, port, err := net.SplitHostPort(peerInfo.HostPort)
		if err != nil {
			return "", fmt.Errorf("failed splitting tchannel's hostport %q, err: %w", peerInfo.HostPort, err)
		}

		// return broadcast_ip:listen_port
		return net.JoinHostPort(broadcastIP.String(), port), nil
	}
}
