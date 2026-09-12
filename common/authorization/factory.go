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
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
)

func NewAuthorizer(authorization config.Authorization, logger log.Logger, domainCache cache.DomainCache) (Authorizer, error) {
	switch true {
	case authorization.OAuthAuthorizer.Enable:
		return NewOAuthAuthorizer(authorization.OAuthAuthorizer, logger, domainCache)
	default:
		return NewNopAuthorizer()
	}
}

// NewAuthenticator creates an Authorizer that only validates caller credentials,
// matching however the configured authorizer identifies callers.
//
// Prefer NewAuthorizerAndAuthenticator when the deployment needs both.
func NewAuthenticator(authorization config.Authorization, logger log.Logger) (Authorizer, error) {
	switch true {
	case authorization.OAuthAuthorizer.Enable:
		return NewOAuthAuthenticator(authorization.OAuthAuthorizer, logger)
	default:
		return NewNopAuthorizer()
	}
}

// NewAuthorizerAndAuthenticator creates an authorizer and a matching authenticator.
// For OAuth, both share one token validator and verification key set. This avoids
// duplicate JWKS fetches and potentially different keys from separate initialization.
// Use this constructor when both implementations are needed.
func NewAuthorizerAndAuthenticator(
	authorization config.Authorization,
	logger log.Logger,
	domainCache cache.DomainCache,
) (Authorizer, Authorizer, error) {
	switch true {
	case authorization.OAuthAuthorizer.Enable:
		return NewOAuthAuthorizerAndAuthenticator(authorization.OAuthAuthorizer, logger, domainCache)
	default:
		authorizer, err := NewNopAuthorizer()
		if err != nil {
			return nil, nil, err
		}
		authenticator, err := NewNopAuthorizer()
		if err != nil {
			return nil, nil, err
		}
		return authorizer, authenticator, nil
	}
}
