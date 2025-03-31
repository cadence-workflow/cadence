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
	"errors"
	"time"

	"github.com/uber-go/tally"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
)

const (
	// EnvKeyRoot the environment variable key for runtime root dir
	EnvKeyRoot = "CADENCE_CANARY_ROOT"
	// EnvKeyConfigDir the environment variable key for config dir
	EnvKeyConfigDir = "CADENCE_CANARY_CONFIG_DIR"
	// EnvKeyEnvironment is the environment variable key for environment
	EnvKeyEnvironment = "CADENCE_CANARY_ENVIRONMENT"
	// EnvKeyAvailabilityZone is the environment variable key for AZ
	EnvKeyAvailabilityZone = "CADENCE_CANARY_AVAILABILITY_ZONE"
	// EnvKeyMode is the environment variable key for Mode
	EnvKeyMode = "CADENCE_CANARY_MODE"
)

const (
	// CadenceServiceName is the default service name for cadence frontend
	CadenceServiceName = "cadence-frontend"
	// CanaryServiceName is the default service name for cadence canary
	CanaryServiceName = "cadence-canary"
	// CrossClusterCanaryModeFull is a canary testing mode which tests all permutations of
	// the cross-cluster/domain feature
	CrossClusterCanaryModeFull = "test-all"
)

type (
	// Config contains the configurable yaml
	// properties for the canary runtime
	Config struct {
		Canary  Canary         `yaml:"canary"`
		Cadence Cadence        `yaml:"cadence"`
		Log     log.Config     `yaml:"log"`
		Metrics config.Metrics `yaml:"metrics"`
	}

	// Canary contains the configuration for canary tests
	Canary struct {
		CrossClusterTestMode string   `yaml:"crossClusterTestMode"`
		CanaryDomainClusters []string `yaml:"canaryDomainClusters"` // the clusters to set for each domain
		Domains              []string `yaml:"domains"`
		Excludes             []string `yaml:"excludes"`
		Cron                 Cron     `yaml:"cron"`
	}

	// Cron contains configuration for the cron workflow for canary
	Cron struct {
		CronSchedule         string        `yaml:"cronSchedule"`         // default to "@every 30s"
		CronExecutionTimeout time.Duration `yaml:"cronExecutionTimeout"` // default to 18 minutes
		StartJobTimeout      time.Duration `yaml:"startJobTimeout"`      // default to 9 minutes
	}

	// Cadence contains the configuration for cadence service
	Cadence struct {
		ServiceName string `yaml:"service"`
		// support Thrift for backward compatibility. It will be ignored if host (gRPC) is used.
		ThriftHostNameAndPort string `yaml:"host"`
		// gRPC host name and port
		GRPCHostNameAndPort string `yaml:"address"`
		// TLS cert file if TLS is enabled on the Cadence server
		TLSCAFile string `yaml:"tlsCaFile"`
	}
)

// Validate validates canary configration
func (c *Config) Validate() error {
	if len(c.Canary.Domains) == 0 {
		return errors.New("missing value for domains property")
	}
	return nil
}

// RuntimeContext contains all the context
// information needed to run the canary
type RuntimeContext struct {
	logger  *zap.Logger
	metrics tally.Scope
	service workflowserviceclient.Interface
}

// NewRuntimeContext builds a runtime context from the config
func NewRuntimeContext(
	logger *zap.Logger,
	scope tally.Scope,
	service workflowserviceclient.Interface,
) *RuntimeContext {
	return &RuntimeContext{
		logger:  logger,
		metrics: scope,
		service: service,
	}
}
