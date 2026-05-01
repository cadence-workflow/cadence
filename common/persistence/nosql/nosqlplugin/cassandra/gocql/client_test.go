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

package gocql

import (
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
)

func Test_GetRegisteredClient(t *testing.T) {
	assert.Panics(t, func() { GetRegisteredClient() })
}

func Test_GetRegisteredClientNotNil(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	registered = NewMockClient(mockCtrl)
	assert.Equal(t, registered, GetRegisteredClient())
}

func Test_RegisterClient(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	RegisterClient(nil)
}

func Test_RegisterClientNotNil(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	newClient := NewMockClient(mockCtrl)
	registered = nil
	RegisterClient(newClient)
	assert.Equal(t, newClient, registered)
}

func Test_newCassandraCluster(t *testing.T) {
	testFullConfig := ClusterConfig{
		Hosts:      "testHost1,testHost2,testHost3,testHost4",
		Port:       123,
		User:       "testUser",
		Password:   "testPassword",
		Keyspace:   "testKeyspace",
		Datacenter: "testDatacenter",
		Region:     "testRegion",
		TLS: &config.TLS{
			Enabled:  true,
			CertFile: "testCertFile",
			KeyFile:  "testKeyFile",
		},
		MaxConns: 10,
	}
	clusterConfig := newCassandraCluster(testFullConfig)
	assert.Equal(t, []string{"testHost1", "testHost2", "testHost3", "testHost4"}, clusterConfig.Hosts)
	assert.Equal(t, testFullConfig.Port, clusterConfig.Port)
	assert.Equal(t, testFullConfig.User, clusterConfig.Authenticator.(gocql.PasswordAuthenticator).Username)
	assert.Equal(t, testFullConfig.Password, clusterConfig.Authenticator.(gocql.PasswordAuthenticator).Password)
	assert.Equal(t, testFullConfig.Keyspace, clusterConfig.Keyspace)
	assert.Equal(t, testFullConfig.TLS.CertFile, clusterConfig.SslOpts.CertPath)
	assert.Equal(t, testFullConfig.TLS.KeyFile, clusterConfig.SslOpts.KeyPath)
	assert.Equal(t, testFullConfig.MaxConns, clusterConfig.NumConns)

	assert.False(t, clusterConfig.HostFilter.Accept(&gocql.HostInfo{}))
}
