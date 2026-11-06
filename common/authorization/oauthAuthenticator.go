// Copyright (c) 2026 Uber Technologies, Inc.
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

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
)

type oauthAuthenticator struct {
	authority *oauthAuthority
}

// NewOAuthAuthenticator creates an Authorizer that validates caller credentials and
// nothing else. It ignores the attributes: it reports whether callers are who they
// claim to be, not what they are allowed to do. Use it for endpoints that name no
// resource to authorize against, and that check permissions on each item they return.
//
// Prefer NewOAuthAuthorizerAndAuthenticator when the deployment needs both, so that the
// two share one oauthAuthority.
func NewOAuthAuthenticator(oauthConfig config.OAuthAuthorizer, log log.Logger) (Authorizer, error) {
	authority, err := newOAuthAuthority(oauthConfig, log)
	if err != nil {
		return nil, err
	}

	return newOAuthAuthenticator(authority), nil
}

func newOAuthAuthenticator(authority *oauthAuthority) *oauthAuthenticator {
	return &oauthAuthenticator{authority: authority}
}

func (a *oauthAuthenticator) Authorize(ctx context.Context, _ *Attributes) (Result, error) {
	if _, err := a.authority.getAuthClaims(ctx); err != nil {
		return Result{Decision: DecisionDeny}, nil
	}

	return Result{Decision: DecisionAllow}, nil
}
