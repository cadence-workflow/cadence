// Copyright (c) 2017-2021 Uber Technologies Inc.

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

package lib

import (
	"github.com/uber-go/tally"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/log"
)

const (
	defaultCadenceLocalHostPort = "127.0.0.1:7933"
	defaultCadenceServiceName   = "cadence-frontend"
)

// ContextKey is an alias for string, used as context key
type ContextKey string

const (
	// CtxKeyRuntimeContext is the name of the context key whose value is the RuntimeContext
	CtxKeyRuntimeContext = ContextKey("ctxKeyRuntimeCtx")

	// CtxKeyCadenceClient is the name of the context key for the cadence client this cadence worker listens to
	CtxKeyCadenceClient = ContextKey("ctxKeyCadenceClient")
)

// RuntimeContext contains all of the context information
// needed at cadence bench runtime
type RuntimeContext struct {
	Bench   Bench
	Cadence Cadence
	Logger  *zap.Logger
	Metrics tally.Scope
}

// NewRuntimeContext builds a runtime context from the config
func NewRuntimeContext(cfg *Config) (*RuntimeContext, error) {
	logger, err := cfg.Log.NewZapLogger()
	if err != nil {
		return nil, err
	}

	metricsScope := cfg.Metrics.NewScope(log.NewLogger(logger), cfg.Bench.Name)

	if cfg.Cadence.ServiceName == "" {
		cfg.Cadence.ServiceName = defaultCadenceServiceName
	}

	if cfg.Cadence.HostNameAndPort == "" {
		cfg.Cadence.HostNameAndPort = defaultCadenceLocalHostPort
	}

	return &RuntimeContext{
		Bench:   cfg.Bench,
		Cadence: cfg.Cadence,
		Logger:  logger,
		Metrics: metricsScope,
	}, nil
}
