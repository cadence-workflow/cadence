// Copyright (c) 2018 Uber Technologies, Inc.
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

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/uber-go/tally"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/.gen/go/shared"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/compatibility"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/uber/cadence/common"
	simTypes "github.com/uber/cadence/simulation/replication/types"
)

var (
	clusterName = flag.String("cluster", "", "cluster name")

	ready int32
)

func main() {
	config := zap.NewDevelopmentConfig()
	config.Level.SetLevel(zapcore.InfoLevel)

	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}

	flag.Parse()
	if *clusterName == "" {
		logger.Fatal("cluster name is not set")
	}

	simCfg, err := simTypes.LoadConfig()
	if err != nil {
		logger.Fatal("failed to load simulation config", zap.Error(err))
	}

	cluster, ok := simCfg.Clusters[*clusterName]
	if !ok {
		logger.Fatal("cluster not found in config", zap.String("cluster", *clusterName))
	}

	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: simTypes.WorkerIdentityFor(*clusterName),
		Outbounds: yarpc.Outbounds{
			"cadence-frontend": {Unary: grpc.NewTransport().NewSingleOutbound(cluster.GRPCEndpoint)},
		},
	})
	err = dispatcher.Start()
	if err != nil {
		logger.Fatal("Failed to start dispatcher", zap.Error(err))
	}
	defer dispatcher.Stop()

	clientConfig := dispatcher.ClientConfig("cadence-frontend")

	cadenceClient := compatibility.NewThrift2ProtoAdapter(
		apiv1.NewDomainAPIYARPCClient(clientConfig),
		apiv1.NewWorkflowAPIYARPCClient(clientConfig),
		apiv1.NewWorkerAPIYARPCClient(clientConfig),
		apiv1.NewVisibilityAPIYARPCClient(clientConfig),
	)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&ready) == 1 {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	go http.ListenAndServe(":6060", nil)
	waitUntilDomainReady(logger, cadenceClient, simCfg)

	workerOptions := worker.Options{
		Identity:     simTypes.WorkerIdentityFor(*clusterName),
		Logger:       logger,
		MetricsScope: tally.NewTestScope(simTypes.TasklistName, map[string]string{"cluster": *clusterName}),
	}

	w := worker.New(
		cadenceClient,
		simCfg.Domain.Name,
		simTypes.TasklistName,
		workerOptions,
	)

	w.RegisterWorkflowWithOptions(TestWorkflow, workflow.RegisterOptions{Name: simTypes.WorkflowName})
	w.RegisterActivityWithOptions(TestActivity, activity.RegisterOptions{Name: simTypes.ActivityName})

	err = w.Start()
	if err != nil {
		logger.Fatal("Failed to start worker", zap.Error(err))
	}
	defer w.Stop()
	logger.Info("Started worker", zap.String("cluster", *clusterName), zap.String("endpoint", cluster.GRPCEndpoint))

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	logger.Info("Waiting for SIGINT or SIGTERM")
	sig := <-sigs
	logger.Sugar().Infof("Received signal: %v so terminating", sig)
}

func waitUntilDomainReady(logger *zap.Logger, client workflowserviceclient.Interface, simCfg *simTypes.ReplicationSimulationConfig) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := client.DescribeDomain(ctx, &shared.DescribeDomainRequest{
			Name: common.StringPtr(simCfg.Domain.Name),
		})

		cancel()
		if err == nil {
			logger.Info("Domain is ready", zap.String("domain", simCfg.Domain.Name))
			atomic.StoreInt32(&ready, 1)
			return
		}

		logger.Info("Domain not ready", zap.String("domain", simCfg.Domain.Name), zap.Error(err))
		time.Sleep(2 * time.Second)
	}
}
