// Copyright (c) 2021 Uber Technologies, Inc.
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
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"

	"go.uber.org/yarpc"
	yarpctls "go.uber.org/yarpc/api/transport/tls"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

// Params allows to configure rpc.Factory
type Params struct {
	ServiceName     string
	TChannelAddress string
	GRPCAddress     string
	GRPCMaxMsgSize  int
	HTTP            *httpParams

	InboundTLS  *tls.Config
	OutboundTLS map[string]*tls.Config

	InboundMiddleware  yarpc.InboundMiddleware
	OutboundMiddleware yarpc.OutboundMiddleware

	OutboundsBuilder OutboundsBuilder
}

type httpParams struct {
	Address    string
	Procedures map[string]struct{}
	TLS        *tls.Config
	Mode       yarpctls.Mode
}

// NewParams creates parameters for rpc.Factory from the given config
func NewParams(serviceName string, config *config.Config, dc *dynamicconfig.Collection, logger log.Logger, metricsCl metrics.Client) (Params, error) {
	serviceConfig, err := config.GetServiceConfig(serviceName)
	if err != nil {
		return Params{}, err
	}

	listenIP, err := GetListenIP(serviceConfig.RPC)
	if err != nil {
		return Params{}, fmt.Errorf("get listen IP: %v", err)
	}

	inboundTLS, err := serviceConfig.RPC.TLS.ToTLSConfig()
	if err != nil {
		return Params{}, fmt.Errorf("inbound TLS config: %v", err)
	}
	outboundTLS := map[string]*tls.Config{}
	for _, outboundServiceName := range service.List {
		outboundServiceConfig, err := config.GetServiceConfig(outboundServiceName)
		if err != nil {
			continue
		}
		outboundTLS[outboundServiceName], err = outboundServiceConfig.RPC.TLS.ToTLSConfig()
		if err != nil {
			return Params{}, fmt.Errorf("outbound %s TLS config: %v", outboundServiceName, err)
		}
	}

	enableGRPCOutbound := dc.GetBoolProperty(dynamicproperties.EnableGRPCOutbound)()

	publicClientOutbound, err := newPublicClientOutbound(config)
	if err != nil {
		return Params{}, fmt.Errorf("public client outbound: %v", err)
	}

	forwardingRules, err := getForwardingRules(dc)
	if err != nil {
		return Params{}, err
	}
	if len(forwardingRules) == 0 {
		// not set, load from static config
		forwardingRules = config.HeaderForwardingRules
	}
	var http *httpParams

	if serviceConfig.RPC.HTTP != nil {
		if serviceConfig.RPC.HTTP.Port <= 0 {
			return Params{}, errors.New("HTTP port is not set")
		}
		procedureMap := map[string]struct{}{}

		for _, v := range serviceConfig.RPC.HTTP.Procedures {
			procedureMap[v] = struct{}{}
		}

		http = &httpParams{
			Address:    net.JoinHostPort(listenIP.String(), strconv.Itoa(int(serviceConfig.RPC.HTTP.Port))),
			Procedures: procedureMap,
		}

		if serviceConfig.RPC.HTTP.TLS.Enabled {
			httptls, err := serviceConfig.RPC.HTTP.TLS.ToTLSConfig()
			if err != nil {
				return Params{}, fmt.Errorf("creating TLS config for HTTP: %w", err)
			}

			http.TLS = httptls
			http.Mode = serviceConfig.RPC.HTTP.TLSMode
		}
	}

	outboundsBuilders := []OutboundsBuilder{
		NewDirectOutboundBuilder(
			service.History,
			enableGRPCOutbound,
			outboundTLS[service.History],
			NewDirectPeerChooserFactory(service.History, logger, metricsCl),
			dc.GetBoolProperty(dynamicproperties.EnableConnectionRetainingDirectChooser),
		),
		NewDirectOutboundBuilder(
			service.Matching,
			enableGRPCOutbound,
			outboundTLS[service.Matching],
			NewDirectPeerChooserFactory(service.Matching, logger, metricsCl),
			dc.GetBoolProperty(dynamicproperties.EnableConnectionRetainingDirectChooser),
		),
		publicClientOutbound,
	}
	if config.ShardDistributorClient.HostPort != "" {
		outboundsBuilders = append(outboundsBuilders, NewSingleGRPCOutboundBuilder(
			service.ShardDistributor,
			service.ShardDistributor,
			config.ShardDistributorClient.HostPort,
		))
	}

	return Params{
		ServiceName:      serviceName,
		HTTP:             http,
		TChannelAddress:  net.JoinHostPort(listenIP.String(), strconv.Itoa(int(serviceConfig.RPC.Port))),
		GRPCAddress:      net.JoinHostPort(listenIP.String(), strconv.Itoa(int(serviceConfig.RPC.GRPCPort))),
		GRPCMaxMsgSize:   serviceConfig.RPC.GRPCMaxMsgSize,
		OutboundsBuilder: CombineOutbounds(outboundsBuilders...),
		InboundTLS:       inboundTLS,
		OutboundTLS:      outboundTLS,
		InboundMiddleware: yarpc.InboundMiddleware{
			// order matters: ForwardPartitionConfigMiddleware must be applied after ClientPartitionConfigMiddleware
			Unary: yarpc.UnaryInboundMiddleware(&InboundMetricsMiddleware{}, &ClientPartitionConfigMiddleware{}, &ForwardPartitionConfigMiddleware{}),
		},
		OutboundMiddleware: yarpc.OutboundMiddleware{
			Unary: yarpc.UnaryOutboundMiddleware(&HeaderForwardingMiddleware{
				Rules: forwardingRules,
			}, &ForwardPartitionConfigMiddleware{}),
		},
	}, nil
}

func getForwardingRules(dc *dynamicconfig.Collection) ([]config.HeaderRule, error) {
	var forwardingRules []config.HeaderRule
	dynForwarding := dc.GetListProperty(dynamicproperties.HeaderForwardingRules)()
	if len(dynForwarding) > 0 {
		for _, f := range dynForwarding {
			switch v := f.(type) {
			case config.HeaderRule: // default or correctly typed value
				forwardingRules = append(forwardingRules, v)
			case map[string]interface{}: // loaded from generic deserialization, compatible with encoding/json
				add, aok := v["Add"].(bool)
				m, mok := v["Match"].(string)
				if !aok || !mok {
					return nil, fmt.Errorf("invalid generic types for header forwarding rule: %#v", v)
				}
				r, err := regexp.Compile(m)
				if err != nil {
					return nil, fmt.Errorf("invalid match regex in header forwarding rule: %q, err: %w", m, err)
				}
				forwardingRules = append(forwardingRules, config.HeaderRule{
					Add:   add,
					Match: r,
				})
			default: // unknown
				return nil, fmt.Errorf("unrecognized dynamic header forwarding type: %T", v)
			}
		}
	}
	return forwardingRules, nil
}

// GetListenIP returns the IP address to bind/listen on based on RPC config
// It respects bindOnLocalHost, bindOnIP, or falls back to auto-detection
func GetListenIP(config config.RPC) (net.IP, error) {
	if config.BindOnLocalHost && len(config.BindOnIP) > 0 {
		return nil, fmt.Errorf("bindOnLocalHost and bindOnIP are mutually exclusive")
	}

	if config.BindOnLocalHost {
		return net.IPv4(127, 0, 0, 1), nil
	}

	if len(config.BindOnIP) > 0 {
		ip := net.ParseIP(config.BindOnIP)
		if ip != nil && ip.To4() != nil {
			return ip.To4(), nil
		}
		if ip != nil && ip.To16() != nil {
			return ip.To16(), nil
		}
		return nil, fmt.Errorf("unable to parse bindOnIP value or it is not an IPv4 or IPv6 address: %s", config.BindOnIP)
	}
	return ListenIP()
}
