// Copyright (c) 2021 Uber Technologies, Inc.
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

package authorization

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
)

type (
	factorySuite struct {
		suite.Suite
		logger log.Logger
	}
)

func TestFactorySuite(t *testing.T) {
	suite.Run(t, new(factorySuite))
}

func (s *factorySuite) SetupTest() {
	s.logger = testlogger.New(s.Suite.T())
}

func cfgNoop() config.Authorization {
	return config.Authorization{
		OAuthAuthorizer: config.OAuthAuthorizer{
			Enable: false,
		},
		NoopAuthorizer: config.NoopAuthorizer{
			Enable: true,
		},
	}
}

func cfgOAuth() config.Authorization {
	return config.Authorization{
		OAuthAuthorizer: config.OAuthAuthorizer{
			Enable: true,
			JwtCredentials: &config.JwtCredentials{
				Algorithm: jwt.SigningMethodRS256.Name,
				PublicKey: "../../config/credentials/keytest.pub",
			},
			MaxJwtTTL: 12345,
		},
	}
}

func (s *factorySuite) TestFactoryNoopAuthorizer() {
	cfgOAuthVar := cfgOAuth()

	publicKey, _ := common.LoadRSAPublicKey(cfgOAuthVar.OAuthAuthorizer.JwtCredentials.PublicKey)

	var tests = []struct {
		cfg      config.Authorization
		expected Authorizer
		err      error
	}{
		{cfgNoop(), &nopAuthority{}, nil},
		{cfgOAuthVar, &oauthAuthorizer{
			authority: &oauthAuthority{
				config:    cfgOAuthVar.OAuthAuthorizer,
				log:       s.logger,
				publicKey: publicKey,
				parser:    jwt.NewParser(jwt.WithValidMethods([]string{cfgOAuthVar.OAuthAuthorizer.JwtCredentials.Algorithm}), jwt.WithIssuedAt()),
			},
			log: s.logger,
		}, nil},
	}

	for _, test := range tests {
		authorizer, err := NewAuthorizer(test.cfg, s.logger, nil)
		s.Equal(authorizer, test.expected)
		s.Equal(err, test.err)
	}
}

func TestNewAuthenticator(t *testing.T) {
	logger := testlogger.New(t)

	testCases := []struct {
		name string
		cfg  config.Authorization
		want Authorizer
	}{
		{
			name: "noop when oauth is disabled",
			cfg:  cfgNoop(),
			want: &nopAuthority{},
		},
		{
			name: "oauth authenticator when oauth is enabled",
			cfg:  cfgOAuth(),
			want: &oauthAuthenticator{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authenticator, err := NewAuthenticator(tc.cfg, logger)
			require.NoError(t, err)
			assert.IsType(t, tc.want, authenticator)
		})
	}
}

func TestNewAuthorizerAndAuthenticator(t *testing.T) {
	logger := testlogger.New(t)

	t.Run("noop when oauth is disabled", func(t *testing.T) {
		authorizer, authenticator, err := NewAuthorizerAndAuthenticator(cfgNoop(), logger, nil)
		require.NoError(t, err)
		assert.Equal(t, &nopAuthority{}, authorizer)
		assert.Equal(t, &nopAuthority{}, authenticator)
	})

	t.Run("oauth pair shares one authority", func(t *testing.T) {
		authorizer, authenticator, err := NewAuthorizerAndAuthenticator(cfgOAuth(), logger, nil)
		require.NoError(t, err)

		require.IsType(t, &oauthAuthorizer{}, authorizer)
		require.IsType(t, &oauthAuthenticator{}, authenticator)
		assert.Same(t,
			authorizer.(*oauthAuthorizer).authority,
			authenticator.(*oauthAuthenticator).authority)
	})

	t.Run("propagates construction failures", func(t *testing.T) {
		cfg := cfgOAuth()
		cfg.OAuthAuthorizer.JwtCredentials.Algorithm = "SHA256"

		authorizer, authenticator, err := NewAuthorizerAndAuthenticator(cfg, logger, nil)
		assert.ErrorContains(t, err, `algorithm "SHA256" is not supported`)
		assert.Nil(t, authorizer)
		assert.Nil(t, authenticator)
	})
}
