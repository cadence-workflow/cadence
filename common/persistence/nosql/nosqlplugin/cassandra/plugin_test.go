// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package cassandra

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"testing"
	"time"
)

func Test_toGoCqlConfig(t *testing.T) {

	tests := []struct {
		name    string
		cfg     *config.NoSQL
		want    gocql.ClusterConfig
		wantErr assert.ErrorAssertionFunc
	}{
		{
			"empty config will be filled with defaults",
			&config.NoSQL{},
			gocql.ClusterConfig{
				Hosts:             "127.0.0.1",
				Port:              9042,
				ProtoVersion:      4,
				Timeout:           time.Second * 10,
				Consistency:       gocql.LocalQuorum,
				SerialConsistency: gocql.LocalSerial,
				ConnectTimeout:    time.Second * 2,
			},
			assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toGoCqlConfig(tt.cfg)
			if !tt.wantErr(t, err, fmt.Sprintf("toGoCqlConfig(%v)", tt.cfg)) {
				return
			}
			assert.Equalf(t, tt.want, got, "toGoCqlConfig(%v)", tt.cfg)
		})
	}
}
