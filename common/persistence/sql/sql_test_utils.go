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

package sql

import (
	"fmt"

	"github.com/uber/cadence/common/config"
)

// NewTestCluster returns a new SQL test cluster
func NewTestCluster(pluginName, dbName, username, password, host string, port int) (config.Persistence, error) {
	var connectAddr string
	// CloudSQL doesn't need a port, don't add it
	if port > 0 {
		connectAddr = fmt.Sprintf("%s:%d", host, port)
	} else {
		connectAddr = host
	}

	cfg := config.SQL{
		User:            username,
		Password:        password,
		ConnectAddr:     connectAddr,
		ConnectProtocol: "tcp",
		PluginName:      pluginName,
		DatabaseName:    dbName,
		NumShards:       4,
		EncodingType:    "thriftrw",
		DecodingTypes:   []string{"thriftrw"},
	}

	return config.Persistence{
		DefaultStore:    "test",
		VisibilityStore: "test",
		DataStores: map[string]config.DataStore{
			"test": {SQL: &cfg},
		},
		NumHistoryShards: cfg.NumShards,
	}, nil
}
