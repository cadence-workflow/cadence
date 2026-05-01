// Copyright (c) 2019 Uber Technologies, Inc.
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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc/api/encoding"
	"go.uber.org/yarpc/api/transport"
	"golang.org/x/net/context"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
)

type (
	oauthSuite struct {
		suite.Suite
		logger      *log.MockLogger
		cfg         config.OAuthAuthorizer
		providerCfg config.OAuthAuthorizer
		att         Attributes
		token       string
		controller  *gomock.Controller
		domainCache *cache.MockDomainCache
		ctx         context.Context
		domainEntry *cache.DomainCacheEntry
	}
)

func TestOAuthSuite(t *testing.T) {
	suite.Run(t, new(oauthSuite))
}

func (s *oauthSuite) SetupTest() {
	s.logger = log.NewMockLogger(gomock.NewController(s.T()))
	s.cfg = config.OAuthAuthorizer{
		Enable: true,
		JwtCredentials: &config.JwtCredentials{
			Algorithm: jwt.SigningMethodRS256.Name,
			PublicKey: "../../config/credentials/keytest.pub",
		},
		MaxJwtTTL: 300000001,
	}
	s.providerCfg = config.OAuthAuthorizer{
		Enable: true,
		Provider: &config.OAuthProvider{
			GroupsAttributePath: "tst_group",
			AdminAttributePath:  "tst_admin",
		},
	}
	// https://jwt.io/#debugger-io?token=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJTdWIiOiIxMjM0NTY3ODkwIiwiTmFtZSI6IkpvaG4gRG9lIiwiR3JvdXBzIjoiYSBiIGMiLCJBZG1pbiI6ZmFsc2UsIklhdCI6MTYyNzUzODcxMiwiVFRMIjozMDAwMDAwMDB9.bh4s8-l1bjG7-QFzuouPy9WPvkq3_9U2e815WFrN-M247NQROBii8ju_N21i6ixK0t-VZTgcJs2B4aN4w1uiCTCg6NyhdeeG8Xd8NcYw0Oq7fjSoFmOXzDzljY6oi9M1XXniNrDIMBLfKXx8tgseSBwOnWoT3vja3ioU6ReqD3Xsp-Wg_clDhb6vtA6pDtnaCVXJNStLSbgWyi-1Mxo9ar92zRDV5YsMaBdUjFUT2bW9QcFzMFAqpHin0QEIa6GPZezY-yn88k5S5cT6Yh7WA4C0Q6C3H1n3EOS05Phwpxt840w7zjh5XR0-rd8-kRX84pHMh0GwHfjV1K7jBQ2QnQ&publicKey=-----BEGIN%20PUBLIC%20KEY-----%0AMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAscukltHilaq%2Bo5gIVE4P%0AGwWl%2BesvJ2EaEpWw6ogr98Un11YJ4oKkwIkLw4iIo0tveCINA3cZmxaW1RejRWKE%0AqYFtQ1rYd6BsnFAHXWh2R3A1FtpG6ANUEGkE7OAJe2%2FL42E%2FImJ%2BGQxRvartInDM%0AyfiRfB7%2BL2n3wG%2BNi%2BhBNMtAaX4Wwbj2hup21Jjuo96TuhcGImBFBATGWaYR2wqe%0A%2F6by9wJexPHlY%2F1uDp3SnzF1dCLjp76SGCfyYqOGC%2FPxhQi7mDxeH9%2FtIC%2Blt%2FSz%0Awc1n8gZLtlRlZHinvYa8lhWXqVYw6WD8h4LTgALq9iY%2BbeD1PFQSY1GkQtt0RhRw%0AeQIDAQAB%0A-----END%20PUBLIC%20KEY-----
	s.token = `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJTdWIiOiIxMjM0NTY3ODkwIiwiTmFtZSI6IkpvaG4gRG9lIiwiR3JvdXBzIjoiYSBiIGMiLCJBZG1pbiI6ZmFsc2UsIklhdCI6MTYyNzUzODcxMiwiVFRMIjozMDAwMDAwMDB9.bh4s8-l1bjG7-QFzuouPy9WPvkq3_9U2e815WFrN-M247NQROBii8ju_N21i6ixK0t-VZTgcJs2B4aN4w1uiCTCg6NyhdeeG8Xd8NcYw0Oq7fjSoFmOXzDzljY6oi9M1XXniNrDIMBLfKXx8tgseSBwOnWoT3vja3ioU6ReqD3Xsp-Wg_clDhb6vtA6pDtnaCVXJNStLSbgWyi-1Mxo9ar92zRDV5YsMaBdUjFUT2bW9QcFzMFAqpHin0QEIa6GPZezY-yn88k5S5cT6Yh7WA4C0Q6C3H1n3EOS05Phwpxt840w7zjh5XR0-rd8-kRX84pHMh0GwHfjV1K7jBQ2QnQ`
	ctx := context.Background()
	ctx, call := encoding.NewInboundCall(ctx)
	err := call.ReadFromRequest(&transport.Request{
		Headers: transport.NewHeaders().With(common.AuthorizationTokenHeaderName, s.token),
	})
	s.NoError(err)
	s.att = Attributes{
		Actor:      "John Doe",
		APIName:    "",
		DomainName: "test-domain",
		TaskList:   nil,
		Permission: PermissionRead,
	}

	s.domainEntry = cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
			Data: map[string]string{
				constants.DomainDataKeyForReadGroups: "c",
			},
		},
		&persistence.DomainConfig{Retention: 1},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234, // not used
	)

	s.controller = gomock.NewController(s.T())
	s.domainCache = cache.NewMockDomainCache(s.controller)
	s.ctx = ctx
}

func (s *oauthSuite) TestCorrectPayload() {
	s.domainCache.EXPECT().GetDomain(s.att.DomainName).Return(s.domainEntry, nil).Times(1)
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.NoError(err)
	result, err := authorizer.Authorize(s.ctx, &s.att)
	s.NoError(err)
	s.Equal(result.Decision, DecisionAllow)
}

func (s *oauthSuite) TestItIsAdmin() {
	ctx := context.Background()
	ctx, call := encoding.NewInboundCall(ctx)
	// https://jwt.io/#debugger-io?token=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJTdWIiOiIxMjM0NTY3ODkwIiwiTmFtZSI6IkpvaG4gRG9lIiwiR3JvdXBzIjoiYSBiIGMiLCJBZG1pbiI6dHJ1ZSwiSWF0IjoxNjI3NTM4NzEyLCJUVEwiOjMwMDAwMDAwMH0.W_989GT8UWm-W7Hv0L2A3fND0Ly_CCuAdVMMoCs-l_GYxgxHP4_P5S9ejqh28AhUYllWNTRR_zM_hNakqnlufz09HP7mwlEKsxQrfoaycX20n8b7V-CktlysyVE2ZbCMt0Ef_MJF6bOOJ4JsayP6TQFXTP7QSUqNTpRYLZcBLlKHDZYm8uol_1EEs3kV5j3lP-WNcR18xBG0UIptakatm7aQEfPWOWnbRUpg9XVv3c4Bt8no4TW1z0XmFF9dD8vb2U-idPkPFstZwOZ0Ikn9nCt4W44kbeCC-i8uCe5SRiqNFWtvjnTBTVqXm27owT7ZbJwqvmMmhZ86Lz7eGtxgPQ&publicKey=-----BEGIN%20PUBLIC%20KEY-----%0AMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAscukltHilaq%2Bo5gIVE4P%0AGwWl%2BesvJ2EaEpWw6ogr98Un11YJ4oKkwIkLw4iIo0tveCINA3cZmxaW1RejRWKE%0AqYFtQ1rYd6BsnFAHXWh2R3A1FtpG6ANUEGkE7OAJe2%2FL42E%2FImJ%2BGQxRvartInDM%0AyfiRfB7%2BL2n3wG%2BNi%2BhBNMtAaX4Wwbj2hup21Jjuo96TuhcGImBFBATGWaYR2wqe%0A%2F6by9wJexPHlY%2F1uDp3SnzF1dCLjp76SGCfyYqOGC%2FPxhQi7mDxeH9%2FtIC%2Blt%2FSz%0Awc1n8gZLtlRlZHinvYa8lhWXqVYw6WD8h4LTgALq9iY%2BbeD1PFQSY1GkQtt0RhRw%0AeQIDAQAB%0A-----END%20PUBLIC%20KEY-----
	token := `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJTdWIiOiIxMjM0NTY3ODkwIiwiTmFtZSI6IkpvaG4gRG9lIiwiR3JvdXBzIjoiYSBiIGMiLCJBZG1pbiI6dHJ1ZSwiSWF0IjoxNjI3NTM4NzEyLCJUVEwiOjMwMDAwMDAwMH0.W_989GT8UWm-W7Hv0L2A3fND0Ly_CCuAdVMMoCs-l_GYxgxHP4_P5S9ejqh28AhUYllWNTRR_zM_hNakqnlufz09HP7mwlEKsxQrfoaycX20n8b7V-CktlysyVE2ZbCMt0Ef_MJF6bOOJ4JsayP6TQFXTP7QSUqNTpRYLZcBLlKHDZYm8uol_1EEs3kV5j3lP-WNcR18xBG0UIptakatm7aQEfPWOWnbRUpg9XVv3c4Bt8no4TW1z0XmFF9dD8vb2U-idPkPFstZwOZ0Ikn9nCt4W44kbeCC-i8uCe5SRiqNFWtvjnTBTVqXm27owT7ZbJwqvmMmhZ86Lz7eGtxgPQ`
	err := call.ReadFromRequest(&transport.Request{
		Headers: transport.NewHeaders().With(common.AuthorizationTokenHeaderName, token),
	})
	s.NoError(err)
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.NoError(err)
	result, err := authorizer.Authorize(ctx, &s.att)
	s.NoError(err)
	s.Equal(result.Decision, DecisionAllow)
}

func (s *oauthSuite) TestEmptyToken() {
	ctx := context.Background()
	ctx, call := encoding.NewInboundCall(ctx)
	err := call.ReadFromRequest(&transport.Request{
		Headers: transport.NewHeaders().With(common.AuthorizationTokenHeaderName, ""),
	})
	s.NoError(err)
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.NoError(err)
	s.logger.EXPECT().Debug("request is not authorized", gomock.Cond(func(t []tag.Tag) bool {
		return fmt.Sprintf("%v", t[0].Field().Interface) == "token is not set in header"
	}))
	result, _ := authorizer.Authorize(ctx, &s.att)
	s.Equal(result.Decision, DecisionDeny)
}

func (s *oauthSuite) TestGetDomainError() {
	s.domainCache.EXPECT().GetDomain(s.att.DomainName).Return(nil, fmt.Errorf("error")).Times(1)
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.NoError(err)
	result, err := authorizer.Authorize(s.ctx, &s.att)
	s.Equal(result.Decision, DecisionDeny)
	s.EqualError(err, "error")
}

func (s *oauthSuite) TestIncorrectPublicKey() {
	s.cfg.JwtCredentials.PublicKey = "incorrectPublicKey"
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.Equal(nil, authorizer)
	s.EqualError(err, "loading RSA public key: invalid public key path incorrectPublicKey")
}

func (s *oauthSuite) TestIncorrectAlgorithm() {
	s.cfg.JwtCredentials.Algorithm = "SHA256"
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.Equal(nil, authorizer)
	s.ErrorContains(err, "algorithm \"SHA256\" is not supported")
}

func (s *oauthSuite) TestMaxTTLLargerInToken() {
	s.cfg.MaxJwtTTL = 1
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.NoError(err)
	s.logger.EXPECT().Debug("request is not authorized", gomock.Cond(func(t []tag.Tag) bool {
		return strings.HasPrefix(fmt.Sprintf("%v", t[0].Field().Interface), "token TTL:")
	}))
	result, _ := authorizer.Authorize(s.ctx, &s.att)
	s.Equal(result.Decision, DecisionDeny)
}

func (s *oauthSuite) TestIncorrectToken() {
	ctx := context.Background()
	ctx, call := encoding.NewInboundCall(ctx)
	err := call.ReadFromRequest(&transport.Request{
		Headers: transport.NewHeaders().With(common.AuthorizationTokenHeaderName, "test"),
	})
	s.NoError(err)
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.NoError(err)
	s.logger.EXPECT().Debug("request is not authorized", gomock.Cond(func(t []tag.Tag) bool {
		return fmt.Sprintf("%v", t[0].Field().Interface) == "token is malformed: token contains an invalid number of segments"
	}))
	result, _ := authorizer.Authorize(ctx, &s.att)
	s.Equal(result.Decision, DecisionDeny)
}

func (s *oauthSuite) TestIatExpiredToken() {
	ctx := context.Background()
	ctx, call := encoding.NewInboundCall(ctx)
	// https://jwt.io/#debugger-io?token=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJTdWIiOiIxMjM0NTY3ODkwIiwiTmFtZSI6IkpvaG4gRG9lIiwiR3JvdXBzIjoiYSBiIGMiLCJBZG1pbiI6ZmFsc2UsIklhdCI6MTYyNzUzODcxMiwiVFRMIjoxfQ.KLOkzV6sIBFCctbcbK98qT5v7ifL_H_6DAzkKsIE4124m5-LtVClA71o5ZtHuoZoiN2xwvGGnkOYg-LbrMajSjsixhGhgz0sAzAomufKACNX1eW9vB5onfTw2q26rpBz0vkIzBYFqUFor3BS30p0V_lnVQGYWRoIcDYspgTyDqMcJ_T77NVBlsyl6ISGiRdv_COcpMEqE_jse7ZKwuoNnQRQp97J3fapPXd6w6qB_PAPlZSXHikvIXG-_9o60RFcB8GDn1lvjZC1NUzGvM2CpVzS4r1_ViKjnjXMuWEPKOyNjQ6LBV9JkRx86N-6jy5V74OyXi-YkiSMplxAKY2G5g&publicKey=-----BEGIN%20PUBLIC%20KEY-----%0AMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAscukltHilaq%2Bo5gIVE4P%0AGwWl%2BesvJ2EaEpWw6ogr98Un11YJ4oKkwIkLw4iIo0tveCINA3cZmxaW1RejRWKE%0AqYFtQ1rYd6BsnFAHXWh2R3A1FtpG6ANUEGkE7OAJe2%2FL42E%2FImJ%2BGQxRvartInDM%0AyfiRfB7%2BL2n3wG%2BNi%2BhBNMtAaX4Wwbj2hup21Jjuo96TuhcGImBFBATGWaYR2wqe%0A%2F6by9wJexPHlY%2F1uDp3SnzF1dCLjp76SGCfyYqOGC%2FPxhQi7mDxeH9%2FtIC%2Blt%2FSz%0Awc1n8gZLtlRlZHinvYa8lhWXqVYw6WD8h4LTgALq9iY%2BbeD1PFQSY1GkQtt0RhRw%0AeQIDAQAB%0A-----END%20PUBLIC%20KEY-----
	token := `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJTdWIiOiIxMjM0NTY3ODkwIiwiTmFtZSI6IkpvaG4gRG9lIiwiR3JvdXBzIjoiYSBiIGMiLCJBZG1pbiI6ZmFsc2UsIklhdCI6MTYyNzUzODcxMiwiVFRMIjoxfQ.KLOkzV6sIBFCctbcbK98qT5v7ifL_H_6DAzkKsIE4124m5-LtVClA71o5ZtHuoZoiN2xwvGGnkOYg-LbrMajSjsixhGhgz0sAzAomufKACNX1eW9vB5onfTw2q26rpBz0vkIzBYFqUFor3BS30p0V_lnVQGYWRoIcDYspgTyDqMcJ_T77NVBlsyl6ISGiRdv_COcpMEqE_jse7ZKwuoNnQRQp97J3fapPXd6w6qB_PAPlZSXHikvIXG-_9o60RFcB8GDn1lvjZC1NUzGvM2CpVzS4r1_ViKjnjXMuWEPKOyNjQ6LBV9JkRx86N-6jy5V74OyXi-YkiSMplxAKY2G5g`
	err := call.ReadFromRequest(&transport.Request{
		Headers: transport.NewHeaders().With(common.AuthorizationTokenHeaderName, token),
	})
	s.NoError(err)
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.NoError(err)
	s.logger.EXPECT().Debug("request is not authorized", gomock.Cond(func(t []tag.Tag) bool {
		return fmt.Sprintf("%v", t[0].Field().Interface) == "token is expired"
	}))
	result, _ := authorizer.Authorize(ctx, &s.att)
	s.Equal(result.Decision, DecisionDeny)
}

func (s *oauthSuite) TestDifferentGroup() {
	s.domainEntry.GetInfo().Data[constants.DomainDataKeyForReadGroups] = "AdifferentGroup"
	s.domainCache.EXPECT().GetDomain(s.att.DomainName).Return(s.domainEntry, nil).Times(1)
	s.att.Permission = PermissionWrite
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.NoError(err)
	s.logger.EXPECT().Debug("request is not authorized", gomock.Cond(func(t []tag.Tag) bool {
		return fmt.Sprintf("%v", t[0].Field().Interface) == "token doesn't have the right permission, jwt groups: [a b c], allowed groups: map[]"
	}))
	result, _ := authorizer.Authorize(s.ctx, &s.att)
	s.Equal(result.Decision, DecisionDeny)
}

func (s *oauthSuite) TestExternalProviderWithoutJWKSWillFail() {
	authorizer, err := NewOAuthAuthorizer(s.providerCfg, s.logger, s.domainCache)
	s.Error(err)
	s.Equal(nil, authorizer)

}

func (s *oauthSuite) TestIncorrectPermission() {
	s.domainCache.EXPECT().GetDomain(s.att.DomainName).Return(s.domainEntry, nil).Times(1)
	s.att.Permission = Permission(15)
	authorizer, err := NewOAuthAuthorizer(s.cfg, s.logger, s.domainCache)
	s.NoError(err)
	s.logger.EXPECT().Debug("request is not authorized", gomock.Cond(func(t []tag.Tag) bool {
		return fmt.Sprintf("%v", t[0].Field().Interface) == "permission 15 is not supported"
	}))
	result, err := authorizer.Authorize(s.ctx, &s.att)
	s.NoError(err)
	s.Equal(result.Decision, DecisionDeny)
}

func Test_oauthAuthority_validateTTL(t *testing.T) {

	tests := []struct {
		name      string
		claims    *JWTClaims
		ttlConfig int64
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:    "Empty claims will fail TTL validation",
			claims:  &JWTClaims{},
			wantErr: assert.Error,
		},
		{
			name: "Claims with IAT and Claim TTL will pass",
			claims: &JWTClaims{
				TTL: 300,
				RegisteredClaims: jwt.RegisteredClaims{
					IssuedAt: jwt.NewNumericDate(time.Now()),
				},
			},
			wantErr:   assert.NoError,
			ttlConfig: 500,
		},

		{
			name: "Claims with IAT but without TTL or ExpiresAT will fail TTL validation",
			claims: &JWTClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					IssuedAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
				},
			},
			ttlConfig: 1,
			wantErr:   assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &oauthAuthority{
				config: config.OAuthAuthorizer{MaxJwtTTL: tt.ttlConfig},
			}
			tt.wantErr(t, validator.validateTTL(tt.claims), fmt.Sprintf("validateTTL(%v)", tt.claims))
		})
	}
}

func TestIsTokenInternal(t *testing.T) {
	internalToken := &jwt.Token{
		Header: map[string]interface{}{},
	}
	internalTokenWithKid := &jwt.Token{
		Header: map[string]interface{}{
			"kid": jwtInternalIssuer,
		},
		Claims: JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer: jwtInternalIssuer,
			},
		},
	}
	externalToken := &jwt.Token{
		Header: map[string]interface{}{
			"kid": "3lkj323jkj3",
		},
		Claims: JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer: "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_hNqHxsxaM",
			},
		},
	}

	tests := []struct {
		name  string
		token *jwt.Token
		want  bool
	}{
		{
			name:  "internal token w/o kid",
			token: internalToken,
			want:  true,
		},
		{
			name:  "internal token with kid",
			token: internalTokenWithKid,
			want:  true,
		},
		{
			name:  "external token with kid",
			token: externalToken,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, isTokenInternal(tt.token), "isTokenInternal(%v)", tt.token)
		})
	}
}

func Test_oauthAuthority_parseExternal(t *testing.T) {
	claim := map[string]interface{}{"cognito:groups": []interface{}{"domain2", "domain1", "group1"}}

	tests := []struct {
		name       string
		config     config.OAuthAuthorizer
		mapToken   map[string]interface{}
		claims     *JWTClaims
		wantGroups string
		wantAdmin  bool
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name: "empty config will not alter token",
			config: config.OAuthAuthorizer{
				Provider: &config.OAuthProvider{
					GroupsAttributePath: "",
					AdminAttributePath:  "",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "groups incorrect path will result into an error",
			config: config.OAuthAuthorizer{
				Provider: &config.OAuthProvider{
					GroupsAttributePath: "/bad/path",
					AdminAttributePath:  "",
				},
			},
			wantErr: assert.Error,
		},
		{
			name: "admin incorrect path will result into an error",
			config: config.OAuthAuthorizer{
				Provider: &config.OAuthProvider{
					GroupsAttributePath: "",
					AdminAttributePath:  "/bad/path",
				},
			},
			wantErr: assert.Error,
		},
		{
			name: "correct groups path will fill claims",
			config: config.OAuthAuthorizer{
				Provider: &config.OAuthProvider{
					GroupsAttributePath: "\"cognito:groups\" | join(' ', @)",
				},
			},
			mapToken:   claim,
			wantErr:    assert.NoError,
			wantGroups: "domain2 domain1 group1",
			wantAdmin:  false,
		},
		{
			name: "correct admin path will fill claims",
			config: config.OAuthAuthorizer{
				Provider: &config.OAuthProvider{
					AdminAttributePath: "\"cognito:groups\" | contains(@, 'group1')",
				},
			},
			mapToken:   claim,
			wantErr:    assert.NoError,
			wantGroups: "",
			wantAdmin:  true,
		},
		{
			name: "non bool result for admin will result in error",
			config: config.OAuthAuthorizer{
				Provider: &config.OAuthProvider{
					AdminAttributePath: "\"cognito:groups\"",
				},
			},
			mapToken:   claim,
			wantErr:    assert.Error,
			wantGroups: "",
			wantAdmin:  false,
		},
		{
			name: "non string result for groups will result in error",
			config: config.OAuthAuthorizer{
				Provider: &config.OAuthProvider{
					GroupsAttributePath: "\"cognito:groups\" | contains(@, 'group1')",
				},
			},
			mapToken:   claim,
			wantErr:    assert.Error,
			wantGroups: "",
			wantAdmin:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &oauthAuthority{
				config: tt.config,
			}
			actualClaim := &JWTClaims{}
			err := a.parseExternal(tt.mapToken, actualClaim)
			tt.wantErr(t, err)
			assert.Equal(t, tt.wantGroups, actualClaim.Groups)
			assert.Equal(t, tt.wantAdmin, actualClaim.Admin)
		})
	}
}
