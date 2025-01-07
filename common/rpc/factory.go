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

package rpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	nethttp "net/http"
	"sync"

	"go.uber.org/yarpc"
	"go.uber.org/yarpc/transport/grpc"
	yarpchttp "go.uber.org/yarpc/transport/http"
	"go.uber.org/yarpc/transport/tchannel"
	"google.golang.org/grpc/credentials"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/service"
)

const (
	defaultGRPCSizeLimit = 4 * 1024 * 1024
	factoryComponentName = "rpc-factory"
)

var (
	// P2P outbounds are only needed for history and matching services
	servicesToTalkP2P = []string{service.History, service.Matching}
)

// Factory is an implementation of rpc.Factory interface
type FactoryImpl struct {
	maxMessageSize int
	channel        tchannel.Channel
	dispatcher     *yarpc.Dispatcher
	outbounds      *Outbounds
	logger         log.Logger
	serviceName    string
	wg             sync.WaitGroup
	ctx            context.Context
	cancelFn       context.CancelFunc
	peerLister     PeerLister
}

// NewFactory builds a new rpc.Factory
func NewFactory(logger log.Logger, p Params) *FactoryImpl {
	logger = logger.WithTags(tag.ComponentRPCFactory)

	inbounds := yarpc.Inbounds{}
	// Create TChannel transport
	// This is here only because ringpop expects tchannel.ChannelTransport,
	// everywhere else we use regular tchannel.Transport.
	ch, err := tchannel.NewChannelTransport(
		tchannel.ServiceName(p.ServiceName),
		tchannel.ListenAddr(p.TChannelAddress))
	if err != nil {
		logger.Fatal("Failed to create transport channel", tag.Error(err))
	}
	tchannel, err := tchannel.NewTransport(tchannel.ServiceName(p.ServiceName))
	if err != nil {
		logger.Fatal("Failed to create tchannel transport", tag.Error(err))
	}

	inbounds = append(inbounds, ch.NewInbound())
	logger.Info("Listening for TChannel requests", tag.Address(p.TChannelAddress))

	// Create gRPC transport
	var options []grpc.TransportOption
	if p.GRPCMaxMsgSize > 0 {
		options = append(options, grpc.ServerMaxRecvMsgSize(p.GRPCMaxMsgSize))
		options = append(options, grpc.ClientMaxRecvMsgSize(p.GRPCMaxMsgSize))
	}
	grpcTransport := grpc.NewTransport(options...)
	if len(p.GRPCAddress) > 0 {
		listener, err := net.Listen("tcp", p.GRPCAddress)
		if err != nil {
			logger.Fatal("Failed to listen on GRPC port", tag.Error(err))
		}

		var inboundOptions []grpc.InboundOption
		if p.InboundTLS != nil {
			inboundOptions = append(inboundOptions, grpc.InboundCredentials(credentials.NewTLS(p.InboundTLS)))
		}

		inbounds = append(inbounds, grpcTransport.NewInbound(listener, inboundOptions...))
		logger.Info("Listening for GRPC requests", tag.Address(p.GRPCAddress))
	}
	// Create http inbound if configured
	if p.HTTP != nil {
		interceptor := func(handler nethttp.Handler) nethttp.Handler {
			return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
				procedure := r.Header.Get(yarpchttp.ProcedureHeader)
				if _, found := p.HTTP.Procedures[procedure]; found {
					handler.ServeHTTP(w, r)
					return
				}
				nethttp.NotFound(w, r)
			})
		}

		inboundOptions := []yarpchttp.InboundOption{yarpchttp.Interceptor(interceptor)}

		if p.HTTP.TLS != nil {
			inboundOptions = append(inboundOptions,
				yarpchttp.InboundTLSConfiguration(p.HTTP.TLS),
				yarpchttp.InboundTLSMode(p.HTTP.Mode))
			logger.Info(fmt.Sprintf("Enabling HTTP TLS with Mode %q", p.HTTP.Mode))
		}

		httpinbound := yarpchttp.NewTransport().NewInbound(p.HTTP.Address, inboundOptions...)

		inbounds = append(inbounds, httpinbound)
		logger.Info("Listening for HTTP requests", tag.Address(p.HTTP.Address))
	}
	// Create outbounds
	outbounds := &Outbounds{}
	if p.OutboundsBuilder != nil {
		outbounds, err = p.OutboundsBuilder.Build(grpcTransport, tchannel)
		if err != nil {
			logger.Fatal("Failed to create outbounds", tag.Error(err))
		}
	}

	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name:               p.ServiceName,
		Inbounds:           inbounds,
		Outbounds:          outbounds.Outbounds,
		InboundMiddleware:  p.InboundMiddleware,
		OutboundMiddleware: p.OutboundMiddleware,
	})

	ctx, cancel := context.WithCancel(context.Background())
	return &FactoryImpl{
		maxMessageSize: p.GRPCMaxMsgSize,
		dispatcher:     dispatcher,
		channel:        ch.Channel(),
		outbounds:      outbounds,
		serviceName:    p.ServiceName,
		logger:         logger,
		ctx:            ctx,
		cancelFn:       cancel,
	}
}

// GetDispatcher return a cached dispatcher
func (d *FactoryImpl) GetDispatcher() *yarpc.Dispatcher {
	return d.dispatcher
}

// GetChannel returns Tchannel Channel used by Ringpop
func (d *FactoryImpl) GetTChannel() tchannel.Channel {
	return d.channel
}

func (d *FactoryImpl) GetMaxMessageSize() int {
	if d.maxMessageSize == 0 {
		return defaultGRPCSizeLimit
	}
	return d.maxMessageSize
}

func (d *FactoryImpl) Start(peerLister PeerLister) error {
	d.peerLister = peerLister
	// subscribe to membership changes for history and matching. This is needed to update the peers for rpc
	for _, svc := range servicesToTalkP2P {
		ch := make(chan *membership.ChangedEvent, 1)
		if err := d.peerLister.Subscribe(svc, factoryComponentName, ch); err != nil {
			return fmt.Errorf("rpc factory failed to subscribe to membership updates for svc: %v, err: %v", svc, err)
		}
		d.wg.Add(1)
		go d.listenMembershipChanges(svc, ch)
	}

	return nil
}

func (d *FactoryImpl) Stop() error {
	d.logger.Info("stopping rpc factory")

	for _, svc := range servicesToTalkP2P {
		if err := d.peerLister.Unsubscribe(svc, factoryComponentName); err != nil {
			d.logger.Error("rpc factory failed to unsubscribe from membership updates", tag.Error(err), tag.Service(svc))
		}
	}

	d.cancelFn()
	d.wg.Wait()

	d.logger.Info("stopped rpc factory")
	return nil
}

func (d *FactoryImpl) listenMembershipChanges(svc string, ch chan *membership.ChangedEvent) {
	defer d.wg.Done()

	for {
		select {
		case <-ch:
			d.logger.Debug("rpc factory received membership changed event", tag.Service(svc))
			members, err := d.peerLister.Members(svc)
			if err != nil {
				d.logger.Error("rpc factory failed to get members from membership resolver", tag.Error(err), tag.Service(svc))
				continue
			}

			d.outbounds.UpdatePeers(svc, members)
		case <-d.ctx.Done():
			d.logger.Info("rpc factory stopped so listenMembershipChanges returning", tag.Service(svc))
			return
		}
	}
}

func createDialer(transport *grpc.Transport, tlsConfig *tls.Config) *grpc.Dialer {
	var dialOptions []grpc.DialOption
	if tlsConfig != nil {
		dialOptions = append(dialOptions, grpc.DialerCredentials(credentials.NewTLS(tlsConfig)))
	}
	return transport.NewDialer(dialOptions...)
}
