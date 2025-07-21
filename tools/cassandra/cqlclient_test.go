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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

func TestNewCQLClientWithRetry(t *testing.T) {
	defer goleak.VerifyNone(t)

	mockClient, mockSession, mockClock := setUpMocks(t)
	cfg := configuration()
	expectedCfg := expectedConfig()

	// Fail the first two times, then succeed.
	mockClient.EXPECT().CreateSession(expectedCfg).Return(mockSession, assert.AnError)
	mockClient.EXPECT().CreateSession(expectedCfg).Return(mockSession, assert.AnError)
	mockClient.EXPECT().CreateSession(expectedCfg).Return(mockSession, nil)

	go func() {
		// First retry happens after 1 second
		mockClock.BlockUntil(1)
		mockClock.Advance(1 * time.Second)
		// Second retry happens after 2 seconds
		mockClock.BlockUntil(1)
		mockClock.Advance(2 * time.Second)
	}()

	testRetrier := createClientCreationRetrier()
	backoff.WithClock(mockClock)(testRetrier)

	cqlClient, err := NewCQLClientWithRetry(cfg, gocql.One, mockClient, testRetrier)
	assert.NoError(t, err)

	cqlClientImpl, ok := cqlClient.(*CqlClientImpl)
	require.True(t, ok)

	assert.Equal(t, 333, cqlClientImpl.nReplicas)
	assert.Equal(t, cfg, cqlClientImpl.cfg)
	assert.Equal(t, mockSession, cqlClientImpl.session)
}

func TestNewCQLClientWithRetry_Fail(t *testing.T) {
	defer goleak.VerifyNone(t)

	mockClient, mockSession, mockClock := setUpMocks(t)
	cfg := configuration()
	expectedCfg := expectedConfig()

	// There is a off by one error in the retrier implementation, so we need to add one more call
	// if this is fixed, it's fine to remove the +1
	mockClient.EXPECT().CreateSession(expectedCfg).Return(mockSession, assert.AnError).Times(ClientCreationRetryMaximumAttempts + 1)

	go func() {
		// We double the wait time each iteration
		nextAdvance := 1 * time.Second

		// Wait until someone sleeps, then advance the clock more than the backoff would be.
		for i := 0; i < ClientCreationRetryMaximumAttempts; i++ {
			mockClock.BlockUntil(1)
			mockClock.Advance(nextAdvance)
			nextAdvance *= 2
		}
	}()

	testRetrier := createClientCreationRetrier()
	backoff.WithClock(mockClock)(testRetrier)

	_, err := NewCQLClientWithRetry(cfg, gocql.One, mockClient, testRetrier)
	assert.ErrorIs(t, err, assert.AnError)
}

func setUpMocks(t *testing.T) (*gocql.MockClient, *gocql.MockSession, clock.MockedTimeSource) {
	ctrl := gomock.NewController(t)
	mockClient := gocql.NewMockClient(ctrl)
	mockSession := gocql.NewMockSession(ctrl)
	mockClock := clock.NewMockedTimeSource()
	return mockClient, mockSession, mockClock
}

func configuration() *CQLClientConfig {
	return &CQLClientConfig{
		Hosts:                 "testHost",
		Port:                  1234,
		User:                  "testUser",
		Password:              "testPassword",
		AllowedAuthenticators: []string{"testAuthenticator"},
		Keyspace:              "testKeyspace",
		Timeout:               111,
		ConnectTimeout:        222,
		NumReplicas:           333,
		ProtoVersion:          444,
		TLS:                   &config.TLS{Enabled: true},
	}
}

func expectedConfig() gocql.ClusterConfig {
	return gocql.ClusterConfig{
		Hosts:                 "testHost",
		Port:                  1234,
		User:                  "testUser",
		Password:              "testPassword",
		AllowedAuthenticators: []string{"testAuthenticator"},
		Keyspace:              "testKeyspace",
		TLS:                   &config.TLS{Enabled: true},
		Timeout:               111 * time.Second,
		ConnectTimeout:        222 * time.Second,
		ProtoVersion:          444,
		Consistency:           gocql.One,
	}
}
