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
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/yarpc/api/encoding"
	"go.uber.org/yarpc/api/transport"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/testlogger"
)

func TestOAuthAuthenticator(t *testing.T) {
	authenticator, err := NewOAuthAuthenticator(cfgOAuth().OAuthAuthorizer, testlogger.New(t))
	require.NoError(t, err)
	privateKey, err := common.LoadRSAPrivateKey("../../config/credentials/keytest")
	require.NoError(t, err)

	now := time.Now()
	signToken := func(expiresAt time.Time) string {
		token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    jwtInternalIssuer,
				IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Minute)),
				ExpiresAt: jwt.NewNumericDate(expiresAt),
			},
		}).SignedString(privateKey)
		require.NoError(t, err)
		return token
	}

	testCases := []struct {
		name  string
		token string
		want  Decision
	}{
		{"valid token without groups", signToken(now.Add(time.Minute)), DecisionAllow},
		{"missing token", "", DecisionDeny},
		{"malformed token", "not-a-jwt", DecisionDeny},
		{"expired token", signToken(now.Add(-time.Minute)), DecisionDeny},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, call := encoding.NewInboundCall(context.Background())
			request := &transport.Request{}
			if tc.token != "" {
				request.Headers = transport.NewHeaders().With(common.AuthorizationTokenHeaderName, tc.token)
			}
			require.NoError(t, call.ReadFromRequest(request))

			// Authentication must succeed without domain permissions.
			result, err := authenticator.Authorize(ctx, &Attributes{
				APIName:    "ListDomains",
				Permission: PermissionRead,
				DomainName: "some-domain",
			})
			require.NoError(t, err)
			assert.Equal(t, tc.want, result.Decision)
		})
	}
}
