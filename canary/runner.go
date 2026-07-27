// Copyright (c) 2019 Uber Technologies, Inc.
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

package canary

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/compatibility"
	"go.uber.org/multierr"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/peer"
	"go.uber.org/yarpc/peer/hostport"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/yarpc/transport/tchannel"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"

	"github.com/uber/cadence/common/log"
)

type canaryRunner struct {
	*RuntimeContext
	config *Canary
}

// NewCanaryRunner creates and returns a runnable which spins
// up a set of canaries based on supplied config
func NewCanaryRunner(cfg *Config) (Runnable, error) {
	logger, err := cfg.Log.NewZapLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %v", err)
	}

	metricsScope := cfg.Metrics.NewScope(log.NewLogger(logger), "cadence-canary")

	if cfg.Cadence.ServiceName == "" {
		cfg.Cadence.ServiceName = CadenceServiceName
	}

	var dispatcher *yarpc.Dispatcher
	var runtimeContext *RuntimeContext
	if cfg.Cadence.GRPCHostNameAndPort != "" {
		var outbounds transport.Outbounds
		if cfg.Cadence.TLSCAFile != "" {
			caCert, err := ioutil.ReadFile(cfg.Cadence.TLSCAFile)
			if err != nil {
				logger.Fatal("Failed to load server CA certificate", zap.Error(err))
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				logger.Fatal("Failed to add server CA certificate", zap.Error(err))
			}
			tlsConfig := tls.Config{
				RootCAs: caCertPool,
			}
			tlsCreds := credentials.NewTLS(&tlsConfig)
			grpcTransport := grpc.NewTransport()
			tlsChooser := peer.NewSingle(hostport.Identify(cfg.Cadence.GRPCHostNameAndPort), grpcTransport.NewDialer(grpc.DialerCredentials(tlsCreds)))
			outbounds = transport.Outbounds{Unary: grpcTransport.NewOutbound(tlsChooser)}
		} else {
			outbounds = transport.Outbounds{Unary: grpc.NewTransport().NewSingleOutbound(cfg.Cadence.GRPCHostNameAndPort)}
		}
		dispatcher = yarpc.NewDispatcher(yarpc.Config{
			Name: CanaryServiceName,
			Outbounds: yarpc.Outbounds{
				cfg.Cadence.ServiceName: outbounds,
			},
		})
		clientConfig := dispatcher.ClientConfig(cfg.Cadence.ServiceName)
		runtimeContext = NewRuntimeContext(
			logger,
			metricsScope,
			compatibility.NewThrift2ProtoAdapter(
				compatibility.AdapterClients{
					Domain:     apiv1.NewDomainAPIYARPCClient(clientConfig),
					Workflow:   apiv1.NewWorkflowAPIYARPCClient(clientConfig),
					Worker:     apiv1.NewWorkerAPIYARPCClient(clientConfig),
					Visibility: apiv1.NewVisibilityAPIYARPCClient(clientConfig),
					Schedule:   apiv1.NewScheduleAPIYARPCClient(clientConfig),
				},
			),
		)
	} else if cfg.Cadence.ThriftHostNameAndPort != "" {
		tch, err := tchannel.NewChannelTransport(
			tchannel.ServiceName(CanaryServiceName),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create transport channel: %v", err)
		}
		dispatcher = yarpc.NewDispatcher(yarpc.Config{
			Name: CanaryServiceName,
			Outbounds: yarpc.Outbounds{
				cfg.Cadence.ServiceName: {Unary: tch.NewSingleOutbound(cfg.Cadence.ThriftHostNameAndPort)},
			},
		})
		runtimeContext = NewRuntimeContext(
			logger,
			metricsScope,
			workflowserviceclient.New(dispatcher.ClientConfig(cfg.Cadence.ServiceName)),
		)
	} else {
		return nil, fmt.Errorf("must specify either gRPC address(address) or Thrift address (host) in the config")
	}

	if err := dispatcher.Start(); err != nil {
		dispatcher.Stop()
		return nil, fmt.Errorf("failed to create outbound transport channel: %v", err)
	}

	return &canaryRunner{
		RuntimeContext: runtimeContext,
		config:         &cfg.Canary,
	}, nil
}

// Run runs the canaries
func (r *canaryRunner) Run(mode string) error {
	r.metrics.Counter("restarts").Inc(1)
	if len(r.config.Excludes) != 0 {
		updateSanityChildWFList(r.config.Excludes)
	}

	if r.config.Cron.CronSchedule == "" {
		r.config.Cron.CronSchedule = "@every 30s"
	}
	if r.config.Cron.CronExecutionTimeout == 0 {
		r.config.Cron.CronExecutionTimeout = 18 * time.Minute
	}
	if r.config.Cron.StartJobTimeout == 0 {
		r.config.Cron.StartJobTimeout = 9 * time.Minute
	}

	tasks := make([]Runnable, len(r.config.Domains))
	for i, d := range r.config.Domains {
		i, d := i, d
		canaryTask := newCanary(d, r.RuntimeContext, r.config)
		r.logger.Info("starting canary", zap.String("domain", d))
		tasks[i] = runnableFunc(func(mode string) error {
			if err := canaryTask.Run(mode); err != nil {
				return fmt.Errorf("domain %s: %w", d, err)
			}
			return nil
		})
	}
	return runCanaries(tasks, mode, r.logger)
}

// runnableFunc adapts a plain function to the Runnable interface.
type runnableFunc func(mode string) error

func (f runnableFunc) Run(mode string) error {
	return f(mode)
}

// runCanaries runs every task concurrently and only returns an error if
// every task failed - a single task's start error should not take down
// the others, since that would lose observability for the rest during
// an incident.
func runCanaries(tasks []Runnable, mode string, logger *zap.Logger) error {
	var wg sync.WaitGroup
	errs := make([]error, len(tasks))
	for i, task := range tasks {
		i, task := i, task
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := task.Run(mode); err != nil {
				logger.Error("canary failed to run", zap.Error(err))
				errs[i] = err
			}
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err == nil {
			return nil
		}
	}
	return multierr.Combine(errs...)
}

func updateSanityChildWFList(excludes []string) {
	var temp []string
	for _, childName := range sanityChildWFList {
		if !isStringInList(childName, excludes) {
			temp = append(temp, childName)
		}
	}
	sanityChildWFList = temp
}

func isStringInList(str string, list []string) bool {
	for _, l := range list {
		if l == str {
			return true
		}
	}
	return false
}
