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

package nosql

import (
	"testing"

	"github.com/uber/cadence/common/config"
)

// TestClusterParams are params for test cluster initialization.
type TestClusterParams struct {
	PluginName    string
	KeySpace      string
	Username      string
	Password      string
	Host          string
	Port          int
	ProtoVersion  int
	SchemaBaseDir string
	// Replicas defaults to 1 if not set
	Replicas int
	// MaxConns defaults to 2 if not set
	MaxConns int
}

// NewTestCluster returns a new cassandra test cluster
func NewTestCluster(_ *testing.T, params TestClusterParams) config.Persistence {
	cfg := config.NoSQL{
		PluginName:   params.PluginName,
		User:         params.Username,
		Password:     params.Password,
		Hosts:        params.Host,
		Port:         params.Port,
		MaxConns:     maxConns(params.MaxConns),
		Keyspace:     params.KeySpace,
		ProtoVersion: params.ProtoVersion,
	}

	return config.Persistence{
		DefaultStore:    "test",
		VisibilityStore: "test",
		DataStores: map[string]config.DataStore{
			"test": {NoSQL: &cfg},
		},
	}
}

func maxConns(maxConns int) int {
	if maxConns == 0 {
		return 2
	}

	return maxConns
}
