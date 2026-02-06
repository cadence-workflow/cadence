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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination factory_mock.go -self_package github.com/uber/cadence/tools/cli

package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/olivere/elastic"
	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"github.com/urfave/cli/v2"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/peer"
	"go.uber.org/yarpc/peer/hostport"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/yarpc/transport/tchannel"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"

	serverAdmin "github.com/uber/cadence/.gen/go/admin/adminserviceclient"
	serverFrontend "github.com/uber/cadence/.gen/go/cadence/workflowserviceclient"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	grpcClient "github.com/uber/cadence/client/wrappers/grpc"
	"github.com/uber/cadence/client/wrappers/thrift"
	"github.com/uber/cadence/common"
	cc "github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/common/commoncli"
)

const (
	cadenceClientName      = "cadence-client"
	cadenceFrontendService = "cadence-frontend"
)

// ContextKey is an alias for string, used as context key
type ContextKey string

const (
	// CtxKeyJWT is the name of the context key for the JWT
	CtxKeyJWT = ContextKey("ctxKeyJWT")
)

// ClientFactory is used to construct rpc clients
type ClientFactory interface {
	ServerFrontendClient(c *cli.Context) (frontend.Client, error)
	ServerAdminClient(c *cli.Context) (admin.Client, error)

	// ServerFrontendClientForMigration frontend client of the migration destination
	ServerFrontendClientForMigration(c *cli.Context) (frontend.Client, error)
	// ServerAdminClientForMigration admin client of the migration destination
	ServerAdminClientForMigration(c *cli.Context) (admin.Client, error)

	ElasticSearchClient(c *cli.Context) (*elastic.Client, error)

	ServerConfig(c *cli.Context) (*config.Config, error)
}

type clientFactory struct {
	dispatcher          *yarpc.Dispatcher // lazy, via ensureDispatcher
	dispatcherMigration *yarpc.Dispatcher // lazy, via ensureDispatcherForMigration
	logger              *zap.Logger
}

// NewClientFactory creates a new ClientFactory
func NewClientFactory(logger *zap.Logger) ClientFactory {
	return &clientFactory{
		logger: logger,
	}
}

// ServerConfig returns Cadence server configs.
// Use in some CLI admin operations (e.g. accessing DB directly)
func (b *clientFactory) ServerConfig(c *cli.Context) (*config.Config, error) {
	env := c.String(FlagServiceEnv)
	zone := c.String(FlagServiceZone)
	configDir := c.String(FlagServiceConfigDir)

	var cfg config.Config
	err := config.Load(env, configDir, zone, &cfg)
	if err != nil {
		return nil, commoncli.Problem(
			fmt.Sprintf(
				"failed to load config (for --%v %q --%v %q --%v %q)",
				FlagServiceEnv, env, FlagServiceZone, zone, FlagServiceConfigDir, configDir,
			),
			err,
		)
	}
	return &cfg, nil
}

// ServerFrontendClient builds a frontend client (based on server side thrift interface)
func (b *clientFactory) ServerFrontendClient(c *cli.Context) (frontend.Client, error) {
	err := b.ensureDispatcher(c)
	if err != nil {
		return nil, commoncli.Problem("failed to create frontend client dependency", err)
	}
	clientConfig := b.dispatcher.ClientConfig(cadenceFrontendService)
	if c.String(FlagTransport) == grpcTransport {
		return grpcClient.NewFrontendClient(
			apiv1.NewDomainAPIYARPCClient(clientConfig),
			apiv1.NewWorkflowAPIYARPCClient(clientConfig),
			apiv1.NewWorkerAPIYARPCClient(clientConfig),
			apiv1.NewVisibilityAPIYARPCClient(clientConfig),
		), nil
	}
	return thrift.NewFrontendClient(serverFrontend.New(clientConfig)), nil
}

// ServerAdminClient builds an admin client (based on server side thrift interface)
func (b *clientFactory) ServerAdminClient(c *cli.Context) (admin.Client, error) {
	err := b.ensureDispatcher(c)
	if err != nil {
		return nil, commoncli.Problem("failed to create admin client dependency", err)
	}
	clientConfig := b.dispatcher.ClientConfig(cadenceFrontendService)
	if c.String(FlagTransport) == grpcTransport {
		return grpcClient.NewAdminClient(adminv1.NewAdminAPIYARPCClient(clientConfig)), nil
	}
	return thrift.NewAdminClient(serverAdmin.New(clientConfig)), nil
}

// ServerFrontendClientForMigration builds a frontend client (based on server side thrift interface)
func (b *clientFactory) ServerFrontendClientForMigration(c *cli.Context) (frontend.Client, error) {
	err := b.ensureDispatcherForMigration(c)
	if err != nil {
		return nil, commoncli.Problem("failed to create frontend client dependency", err)
	}
	clientConfig := b.dispatcherMigration.ClientConfig(cadenceFrontendService)
	if c.String(FlagTransport) == grpcTransport {
		return grpcClient.NewFrontendClient(
			apiv1.NewDomainAPIYARPCClient(clientConfig),
			apiv1.NewWorkflowAPIYARPCClient(clientConfig),
			apiv1.NewWorkerAPIYARPCClient(clientConfig),
			apiv1.NewVisibilityAPIYARPCClient(clientConfig),
		), nil
	}
	return thrift.NewFrontendClient(serverFrontend.New(clientConfig)), nil
}

// ServerAdminClientForMigration builds an admin client (based on server side thrift interface)
func (b *clientFactory) ServerAdminClientForMigration(c *cli.Context) (admin.Client, error) {
	err := b.ensureDispatcherForMigration(c)
	if err != nil {
		return nil, commoncli.Problem("failed to create admin client dependency", err)
	}
	clientConfig := b.dispatcherMigration.ClientConfig(cadenceFrontendService)
	if c.String(FlagTransport) == grpcTransport {
		return grpcClient.NewAdminClient(adminv1.NewAdminAPIYARPCClient(clientConfig)), nil
	}
	return thrift.NewAdminClient(serverAdmin.New(clientConfig)), nil
}

// ElasticSearchClient builds an ElasticSearch client
func (b *clientFactory) ElasticSearchClient(c *cli.Context) (*elastic.Client, error) {

	url, err := getRequiredOption(c, FlagURL)
	if err != nil {
		return nil, fmt.Errorf("Required flag not present %w", err)
	}

	retrier := elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(128*time.Millisecond, 513*time.Millisecond))

	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetRetrier(retrier),
	)
	if err != nil {
		return nil, commoncli.Problem(
			fmt.Sprintf("failed to create ElasticSearch client (for --%v %q)", FlagURL, url),
			err,
		)
	}

	return client, nil
}

func (b *clientFactory) ensureDispatcher(c *cli.Context) error {
	if b.dispatcher != nil {
		return nil
	}
	d, err := b.newClientDispatcher(c, c.String(FlagAddress))
	if err != nil {
		return commoncli.Problem(
			fmt.Sprintf("failed to create dispatcher (for --%v %q)", FlagAddress, c.String(FlagAddress)),
			err,
		)
	}
	b.dispatcher = d
	return nil
}

func (b *clientFactory) ensureDispatcherForMigration(c *cli.Context) error {
	if b.dispatcherMigration != nil {
		return nil
	}
	dm, err := b.newClientDispatcher(c, c.String(FlagDestinationAddress))
	if err != nil {
		return fmt.Errorf(
			"failed to create dispatcher for migration (for --%v %q): %w",
			FlagDestinationAddress, c.String(FlagDestinationAddress),
			err,
		)
	}
	b.dispatcherMigration = dm
	return nil
}

func (b *clientFactory) newClientDispatcher(c *cli.Context, hostPortOverride string) (*yarpc.Dispatcher, error) {
	shouldUseGrpc := c.String(FlagTransport) == grpcTransport

	hostPort := tchannelPort
	if shouldUseGrpc {
		hostPort = grpcPort
	}
	if hostPortOverride != "" {
		hostPort = hostPortOverride
	}
	var outbounds transport.Outbounds
	if shouldUseGrpc {
		grpcTransport := grpc.NewTransport()
		outbounds = transport.Outbounds{Unary: grpc.NewTransport().NewSingleOutbound(hostPort)}

		tlsCertificatePath := c.String(FlagTLSCertPath)
		if tlsCertificatePath != "" {
			caCert, err := os.ReadFile(tlsCertificatePath)
			if err != nil {
				return nil, commoncli.Problem(
					fmt.Sprintf("unable to find server CA certificate, from --%s %q", FlagTLSCertPath, tlsCertificatePath),
					err,
				)
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				b.logger.Fatal("Failed to add server CA certificate", zap.Error(err))
			}
			tlsConfig := tls.Config{
				RootCAs: caCertPool,
			}
			tlsCreds := credentials.NewTLS(&tlsConfig)
			tlsChooser := peer.NewSingle(hostport.Identify(hostPort), grpcTransport.NewDialer(grpc.DialerCredentials(tlsCreds)))
			outbounds = transport.Outbounds{Unary: grpc.NewTransport().NewOutbound(tlsChooser)}
		}
	} else {
		ch, err := tchannel.NewChannelTransport(tchannel.ServiceName(cadenceClientName), tchannel.ListenAddr("127.0.0.1:0"))
		if err != nil {
			return nil, commoncli.Problem("failed create tchannel client transport", err)
		}
		outbounds = transport.Outbounds{Unary: ch.NewSingleOutbound(hostPort)}
	}

	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name:      cadenceClientName,
		Outbounds: yarpc.Outbounds{cadenceFrontendService: outbounds},
		OutboundMiddleware: yarpc.OutboundMiddleware{
			Unary: &versionMiddleware{},
		},
	})

	if err := dispatcher.Start(); err != nil {
		if stoperr := dispatcher.Stop(); stoperr != nil {
			return nil, commoncli.Problem(
				"failed to start (and stop) tchannel dispatcher",
				// currently does not print very well due to joined errors, but that can be improved
				fmt.Errorf("start err: %w, stop err: %w", err, stoperr),
			)
		}
		return nil, commoncli.Problem("failed to start tchannel dispatcher", err)
	}
	return dispatcher, nil
}

type versionMiddleware struct {
}

func (vm *versionMiddleware) Call(ctx context.Context, request *transport.Request, out transport.UnaryOutbound) (*transport.Response, error) {
	request.Headers = request.Headers.
		With(common.ClientImplHeaderName, cc.CLI).
		With(common.FeatureVersionHeaderName, cc.SupportedCLIVersion).
		With(common.ClientFeatureFlagsHeaderName, cc.FeatureFlagsHeader(cc.DefaultCLIFeatureFlags)).
		With(common.CallerTypeHeaderName, types.CallerTypeCLI.String())
	if jwtKey, ok := ctx.Value(CtxKeyJWT).(string); ok {
		request.Headers = request.Headers.With(common.AuthorizationTokenHeaderName, jwtKey)
	}
	return out.Call(ctx, request)
}

func getJWT(c *cli.Context) string {
	return c.String(FlagJWT)
}

func getJWTPrivateKey(c *cli.Context) string {
	return c.String(FlagJWTPrivateKey)
}
