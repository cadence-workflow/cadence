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
	"context"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

type oauthAuthorizer struct {
	authority   *oauthAuthority
	domainCache cache.DomainCache
	log         log.Logger
}

// NewOAuthAuthorizer creates an oauth Authorizer
func NewOAuthAuthorizer(
	oauthConfig config.OAuthAuthorizer,
	log log.Logger,
	domainCache cache.DomainCache,
) (Authorizer, error) {
	authority, err := newOAuthAuthority(oauthConfig, log)
	if err != nil {
		return nil, err
	}

	return newOAuthAuthorizer(authority, log, domainCache), nil
}

func newOAuthAuthorizer(
	authority *oauthAuthority,
	log log.Logger,
	domainCache cache.DomainCache,
) *oauthAuthorizer {
	return &oauthAuthorizer{
		authority:   authority,
		domainCache: domainCache,
		log:         log,
	}
}

// Authorize validates caller credentials and checks domain permissions.
func (a *oauthAuthorizer) Authorize(ctx context.Context, attributes *Attributes) (Result, error) {
	claims, err := a.authority.getAuthClaims(ctx)
	if err != nil {
		return Result{Decision: DecisionDeny}, nil
	}

	if claims.Admin {
		return Result{Decision: DecisionAllow}, nil
	}

	domain, err := a.domainCache.GetDomain(attributes.DomainName)
	if err != nil {
		return Result{Decision: DecisionDeny}, err
	}

	if err := validatePermission(claims, attributes, domain.GetInfo().Data); err != nil {
		a.log.Debug("request is not authorized", tag.Error(err))
		return Result{Decision: DecisionDeny}, nil
	}

	return Result{Decision: DecisionAllow}, nil
}
