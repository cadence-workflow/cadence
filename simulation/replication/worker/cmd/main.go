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
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
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
	"github.com/uber/cadence/simulation/replication/workflows"
)

var (
	clusterName = flag.String("cluster", "", "cluster name")

	ready int32

	domainErrMu   sync.Mutex
	domainLastErr = map[string]string{}
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
		Name: simTypes.WorkerIdentityFor(*clusterName, ""),
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

	type healthStatus struct {
		Cluster      string            `json:"cluster"`
		GRPCEndpoint string            `json:"grpcEndpoint"`
		ReadyDomains int32             `json:"readyDomains"`
		TotalDomains int               `json:"totalDomains"`
		LastErrors   map[string]string `json:"lastErrors,omitempty"`
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		readyDomains := atomic.LoadInt32(&ready)
		isReady := readyDomains == int32(len(simCfg.Domains))

		domainErrMu.Lock()
		lastErrors := make(map[string]string, len(domainLastErr))
		for k, v := range domainLastErr {
			lastErrors[k] = v
		}
		domainErrMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		status := healthStatus{
			Cluster:      *clusterName,
			GRPCEndpoint: cluster.GRPCEndpoint,
			ReadyDomains: readyDomains,
			TotalDomains: len(simCfg.Domains),
			LastErrors:   lastErrors,
		}
		b, _ := json.Marshal(status) // best-effort diagnostics

		if isReady {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_, _ = w.Write(b)
	})
	go http.ListenAndServe(":6060", nil)

	wg := sync.WaitGroup{}
	for domainName := range simCfg.Domains {
		wg.Add(1)

		go func(name string) {
			waitUntilDomainReady(logger, cadenceClient, name)
			wg.Done()
		}(domainName)
	}
	wg.Wait()

	// Create a channel to receive termination signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Create a slice to hold the started workers
	var workers []worker.Worker

	for domainName := range simCfg.Domains {
		workerOptions := worker.Options{
			Identity:     simTypes.WorkerIdentityFor(*clusterName, domainName),
			Logger:       logger,
			MetricsScope: tally.NewTestScope(simTypes.TasklistName, map[string]string{"cluster": *clusterName}),
		}

		w := worker.New(
			cadenceClient,
			domainName,
			simTypes.TasklistName,
			workerOptions,
		)

		for name, wf := range workflows.Workflows(*clusterName) {
			w.RegisterWorkflowWithOptions(wf, workflow.RegisterOptions{Name: name})
		}

		for name, act := range workflows.Activities {
			w.RegisterActivityWithOptions(act, activity.RegisterOptions{Name: name})
		}

		err := w.Start()
		if err != nil {
			logger.Fatal("Failed to start worker", zap.Error(err))
		}
		workers = append(workers, w) // Add the worker to the slice

		fmt.Printf("Started worker for domain: %s\n", domainName)
		logger.Info("Started worker", zap.String("cluster", *clusterName), zap.String("endpoint", cluster.GRPCEndpoint))
	}

	logger.Info("All workers started. Waiting for SIGINT or SIGTERM")
	sig := <-sigs
	logger.Sugar().Infof("Received signal: %v so terminating", sig)

	// Stop each worker gracefully
	logger.Info("Stopping workers...")
	for _, w := range workers {
		w.Stop()
		logger.Info("Stopped worker")
	}
}

func waitUntilDomainReady(logger *zap.Logger, client workflowserviceclient.Interface, domainName string) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := client.DescribeDomain(ctx, &shared.DescribeDomainRequest{
			Name: common.StringPtr(domainName),
		})

		cancel()
		if err == nil {
			logger.Info("Domains is ready", zap.String("domain", domainName))
			atomic.AddInt32(&ready, 1)
			return
		}

		domainErrMu.Lock()
		domainLastErr[domainName] = err.Error()
		domainErrMu.Unlock()

		logger.Info("Domains not ready", zap.String("domain", domainName), zap.Error(err))
		time.Sleep(2 * time.Second)
	}
}
