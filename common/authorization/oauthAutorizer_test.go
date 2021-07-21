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
	"regexp"
	"testing"

	"github.com/cristalhq/jwt/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/yarpc/api/encoding"
	"go.uber.org/yarpc/api/transport"
	"golang.org/x/net/context"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

var pubKeyTest = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAscukltHilaq+o5gIVE4P
GwWl+esvJ2EaEpWw6ogr98Un11YJ4oKkwIkLw4iIo0tveCINA3cZmxaW1RejRWKE
qYFtQ1rYd6BsnFAHXWh2R3A1FtpG6ANUEGkE7OAJe2/L42E/ImJ+GQxRvartInDM
yfiRfB7+L2n3wG+Ni+hBNMtAaX4Wwbj2hup21Jjuo96TuhcGImBFBATGWaYR2wqe
/6by9wJexPHlY/1uDp3SnzF1dCLjp76SGCfyYqOGC/PxhQi7mDxeH9/tIC+lt/Sz
wc1n8gZLtlRlZHinvYa8lhWXqVYw6WD8h4LTgALq9iY+beD1PFQSY1GkQtt0RhRw
eQIDAQAB
-----END PUBLIC KEY-----`

var privKeyTest = `-----BEGIN PRIVATE KEY-----
MIIEvwIBADANBgkqhkiG9w0BAQEFAASCBKkwggSlAgEAAoIBAQCxy6SW0eKVqr6j
mAhUTg8bBaX56y8nYRoSlbDqiCv3xSfXVgnigqTAiQvDiIijS294Ig0DdxmbFpbV
F6NFYoSpgW1DWth3oGycUAddaHZHcDUW2kboA1QQaQTs4Al7b8vjYT8iYn4ZDFG9
qu0icMzJ+JF8Hv4vaffAb42L6EE0y0BpfhbBuPaG6nbUmO6j3pO6FwYiYEUEBMZZ
phHbCp7/pvL3Al7E8eVj/W4OndKfMXV0IuOnvpIYJ/Jio4YL8/GFCLuYPF4f3+0g
L6W39LPBzWfyBku2VGVkeKe9hryWFZepVjDpYPyHgtOAAur2Jj5t4PU8VBJjUaRC
23RGFHB5AgMBAAECggEABj1T9Orf0W9nskDQ2QQ7cuVdZEJjpMrbTK1Aw1L8/Qc9
TSkINDEayaV9mn1RXe61APcBSdP4ER7nXfTZiQ21LhLcWWg9T3cbh1b70oRqyI9z
Pi6HSBeWz4kfUBX9izMQFBZKzjYn6qaJp1b8bGXKRWkcvPRZqLhmsRPmeH3xrOHe
qsIDhYXMjRoOgEUxLbk8iPLP6nx0icPJl/tHK2l76R+1Ko6TBE69Md2krUIuh0u4
nm9n+Az+0GuvkFsLw5KMGhSBeqB+ez5qtFa8T8CUCn98IjiUDOwgZdFrNldFLcZf
putw7O2qCA9LT+mFBQ6CVsVu/9tKeXQ9sJ7p3lxhwQKBgQDjt7HNIabLncdXPMu0
ByRyNVme0+Y1vbj9Q7iodk77hvlzWpD1p5Oyvq7cN+Cb4c1iO/ZQXMyUw+9hLgmf
LNquH2d4hK1Jerzc/ciwu6dUBsCW8+0VJd4M2UNN15rJMPvbZGmqMq9Np1iCTCjE
dvHo7xjPcJhsbhMbHq+PaUU7OQKBgQDH4KuaHBFTGUPkRaQGAZNRB8dDvSExV6ID
Pblzr80g9kKHUnQCQfIDLjHVgDbTaSCdRw7+EXRyRmLy5mfPWEbUFfIemEpEcEcb
3geWeVDx4Z/FwprWFuVifRopRSQ/FAbMXLIui7OHXWLEtzBvLkR/uS2VIVPm10PV
pbh2EXifQQKBgQDbcOLbjelBYLt/euvGgfeCQ50orIS1Fy5UidVCKjh0tR5gJk95
G1L+tjilqQc+0LtuReBYkwTm+2YMXSQSi1P05fh9MEYZgDjOMZYbkcpu887V6Rx3
+7Te5uOv+OyFozmhs0MMK6m5iGGHtsK2iPUYBoj/Jj8MhorM4KZH6ic4KQKBgQCl
3zIpg09xSc9Iue5juZz6qtzXvzWzkAj4bZnggq1VxGfzix6Q3Q8tSoG6r1tQWLbj
Lpwnhm6/guAMud6+eIDW8ptqfnFrmE26t6hOXMEq6lXANT5vmrKj6DP0uddZrZHy
uJ55+B91n68elvPP4HKiGBfW4cCSGmTGAXAyM0+JwQKBgQCz2cNiFrr+oEnlHDLg
EqsiEufppT4FSZPy9/MtuWuMgEOBu34cckYaai+nahQLQvH62KskTK0EUjE1ywub
NPORuXcugxIBMHWyseOS7lrtrlSBxU9gntS7jHdM3IMrrUy9YZBvPvFGP0wLdpKM
nvt3vT46hs3n28XZpb18uRkSDw==
-----END PRIVATE KEY-----`

type (
	oauthSuite struct {
		suite.Suite
		logger          *log.MockLogger
		cfg             config.OAuthAuthorizer
		att             Attributes
		token           string
		tokenExpiredIat string
		ctx             context.Context
	}
)

func TestOAuthSuite(t *testing.T) {
	suite.Run(t, new(oauthSuite))
}

func (s *oauthSuite) SetupTest() {
	s.logger = &log.MockLogger{}
	s.cfg = config.OAuthAuthorizer{
		Enable: true,
		JwtCredentials: config.JwtCredentials{
			Algorithm:  jwt.RS256.String(),
			PublicKey:  pubKeyTest,
			PrivateKey: privKeyTest,
		},
		MaxJwtTTL: 300000001,
	}
	// https://jwt.io/#debugger-io?token=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwicGVybWlzc2lvbiI6InJlYWQiLCJkb21haW4iOiJ0ZXN0LWRvbWFpbiIsImlhdCI6MTYyNjMzNjQ2MywiVFRMIjozMDAwMDAwMDB9.r1e83j6J392u4oAM7S7RYEDpeEilGThev2rK6RxqRXJIYiQlqKo1siDQjgHmj5PNUyEAQJF54CcXiaWJpTPWiPOxuRGtfJbUjSTnU2TiLvUiYU9bYt5U1w_UdlGzOD0ULhXPv2bzujAgtuQiRutwpljuQZwqqSDzILAMZlD5NMhEajYbE1P_0kv7esHO4oofTh__G3VZ_2fEi52GA8lwqoqBH3tQ1RK5QblnK5zMG5zBy8yK6JUmdoAGnKugjkJdDu8ERI4lNeIaWhD6kV8lksmPY0CxLfbmqLP3BIhvRF7zOeI1ocwa_4lpk4U6QRZ2w4hyGSEtD3sMmz1wl_uQCw&publicKey=-----BEGIN%20PUBLIC%20KEY-----%0AMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAscukltHilaq%2Bo5gIVE4P%0AGwWl%2BesvJ2EaEpWw6ogr98Un11YJ4oKkwIkLw4iIo0tveCINA3cZmxaW1RejRWKE%0AqYFtQ1rYd6BsnFAHXWh2R3A1FtpG6ANUEGkE7OAJe2%2FL42E%2FImJ%2BGQxRvartInDM%0AyfiRfB7%2BL2n3wG%2BNi%2BhBNMtAaX4Wwbj2hup21Jjuo96TuhcGImBFBATGWaYR2wqe%0A%2F6by9wJexPHlY%2F1uDp3SnzF1dCLjp76SGCfyYqOGC%2FPxhQi7mDxeH9%2FtIC%2Blt%2FSz%0Awc1n8gZLtlRlZHinvYa8lhWXqVYw6WD8h4LTgALq9iY%2BbeD1PFQSY1GkQtt0RhRw%0AeQIDAQAB%0A-----END%20PUBLIC%20KEY-----
	s.token = `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6Ik
		pvaG4gRG9lIiwicGVybWlzc2lvbiI6InJlYWQiLCJkb21haW4iOiJ0ZXN0LWRvbWFpbiIsImlhdCI6MTYyNjMzNjQ
		2MywiVFRMIjozMDAwMDAwMDB9.r1e83j6J392u4oAM7S7RYEDpeEilGThev2rK6RxqRXJIYiQlqKo1siDQjgHmj5P
		NUyEAQJF54CcXiaWJpTPWiPOxuRGtfJbUjSTnU2TiLvUiYU9bYt5U1w_UdlGzOD0ULhXPv2bzujAgtuQiRutwplju
		QZwqqSDzILAMZlD5NMhEajYbE1P_0kv7esHO4oofTh__G3VZ_2fEi52GA8lwqoqBH3tQ1RK5QblnK5zMG5zBy8yK6
		JUmdoAGnKugjkJdDu8ERI4lNeIaWhD6kV8lksmPY0CxLfbmqLP3BIhvRF7zOeI1ocwa_4lpk4U6QRZ2w4hyGSEtD3
		sMmz1wl_uQCw`
	// https://jwt.io/#debugger-io?token=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwicGVybWlzc2lvbiI6InJlYWQiLCJkb21haW4iOiJ0ZXN0LWRvbWFpbiIsImlhdCI6MTYyNjMzNjQ2MywiVFRMIjoxfQ.P_T3O54F_aiHcaMwyeh2GXtzgWhyKSLkuu8rtGAylK0HOsHYRIkbjdx251kaDEf2B-QP6KKCiXhDgZ_Q42Tb477zjl9IYGRqEj9JZ7PwGuRWCEZWUaFHgB4XmkviHDMamBB5jqg2I2XYklyNO3r2m45_AcQ3dAU4uLiwBwSVKy_YsMldEvGKMC86JvGcYPhu-LLvrJSViQVyuBGjUor6YREuadAZHyKuoMunLq5b_BW2hTf_67kGiyRL5_DxBBGbiNeHDPNoBUNUAx4Nbe1rAckREL8VULVFC_HZ0bDiM7KMJJ0t6zLcgP8Z3Q3341nfhv9r3qG_6U343ZgTPZfQNQ&publicKey=-----BEGIN%20PUBLIC%20KEY-----%0AMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAscukltHilaq%2Bo5gIVE4P%0AGwWl%2BesvJ2EaEpWw6ogr98Un11YJ4oKkwIkLw4iIo0tveCINA3cZmxaW1RejRWKE%0AqYFtQ1rYd6BsnFAHXWh2R3A1FtpG6ANUEGkE7OAJe2%2FL42E%2FImJ%2BGQxRvartInDM%0AyfiRfB7%2BL2n3wG%2BNi%2BhBNMtAaX4Wwbj2hup21Jjuo96TuhcGImBFBATGWaYR2wqe%0A%2F6by9wJexPHlY%2F1uDp3SnzF1dCLjp76SGCfyYqOGC%2FPxhQi7mDxeH9%2FtIC%2Blt%2FSz%0Awc1n8gZLtlRlZHinvYa8lhWXqVYw6WD8h4LTgALq9iY%2BbeD1PFQSY1GkQtt0RhRw%0AeQIDAQAB%0A-----END%20PUBLIC%20KEY-----
	s.tokenExpiredIat = `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwib
		mFtZSI6IkpvaG4gRG9lIiwicGVybWlzc2lvbiI6InJlYWQiLCJkb21haW4iOiJ0ZXN0LWRvbWFpbiIsImlhdCI6MTY
		yNjMzNjQ2MywiVFRMIjoxfQ.P_T3O54F_aiHcaMwyeh2GXtzgWhyKSLkuu8rtGAylK0HOsHYRIkbjdx251kaDEf2B-
		QP6KKCiXhDgZ_Q42Tb477zjl9IYGRqEj9JZ7PwGuRWCEZWUaFHgB4XmkviHDMamBB5jqg2I2XYklyNO3r2m45_AcQ3
		dAU4uLiwBwSVKy_YsMldEvGKMC86JvGcYPhu-LLvrJSViQVyuBGjUor6YREuadAZHyKuoMunLq5b_BW2hTf_67kGiy
		RL5_DxBBGbiNeHDPNoBUNUAx4Nbe1rAckREL8VULVFC_HZ0bDiM7KMJJ0t6zLcgP8Z3Q3341nfhv9r3qG_6U343ZgT
		PZfQNQ`

	re := regexp.MustCompile(`\r?\n?\t`)
	s.token = re.ReplaceAllString(s.token, "")
	s.tokenExpiredIat = re.ReplaceAllString(s.tokenExpiredIat, "")

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
	s.ctx = ctx
}

func (s *oauthSuite) TearDownTest() {
	s.logger.AssertExpectations(s.T())
}

func (s *oauthSuite) TestCorrectPayload() {
	authorizer := NewOAuthAuthorizer(s.cfg, s.logger)
	result, err := authorizer.Authorize(s.ctx, &s.att)
	s.NoError(err)
	s.Equal(result.Decision, DecisionAllow)
}

func (s *oauthSuite) TestIncorrectPublicKey() {
	s.cfg.JwtCredentials.PublicKey = "incorrectPublicKey"
	authorizer := NewOAuthAuthorizer(s.cfg, s.logger)
	result, err := authorizer.Authorize(s.ctx, &s.att)
	s.EqualError(err, "failed to parse PEM block containing the public key")
	s.Equal(result.Decision, DecisionDeny)
}

func (s *oauthSuite) TestIncorrectAlgorithm() {
	s.cfg.JwtCredentials.Algorithm = "SHA256"
	authorizer := NewOAuthAuthorizer(s.cfg, s.logger)
	result, err := authorizer.Authorize(s.ctx, &s.att)
	s.EqualError(err, "jwt: algorithm is not supported")
	s.Equal(result.Decision, DecisionDeny)
}

func (s *oauthSuite) TestMaxTTLLargerInToken() {
	s.cfg.MaxJwtTTL = 1
	authorizer := NewOAuthAuthorizer(s.cfg, s.logger)
	s.logger.On("Debug", "request is not authorized", mock.MatchedBy(func(t []tag.Tag) bool {
		return fmt.Sprintf("%v", t[0].Field().Interface) == "TTL in token is larger than MaxTTL allowed"
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
	authorizer := NewOAuthAuthorizer(s.cfg, s.logger)
	s.logger.On("Debug", "request is not authorized", mock.MatchedBy(func(t []tag.Tag) bool {
		return fmt.Sprintf("%v", t[0].Field().Interface) == "jwt: token format is not valid"
	}))
	result, _ := authorizer.Authorize(ctx, &s.att)
	s.Equal(result.Decision, DecisionDeny)
}

func (s *oauthSuite) TestIatExpiredToken() {
	ctx := context.Background()
	ctx, call := encoding.NewInboundCall(ctx)
	err := call.ReadFromRequest(&transport.Request{
		Headers: transport.NewHeaders().With(common.AuthorizationTokenHeaderName, s.tokenExpiredIat),
	})
	s.NoError(err)
	authorizer := NewOAuthAuthorizer(s.cfg, s.logger)
	s.logger.On("Debug", "request is not authorized", mock.MatchedBy(func(t []tag.Tag) bool {
		return fmt.Sprintf("%v", t[0].Field().Interface) == "JWT has expired"
	}))
	result, _ := authorizer.Authorize(ctx, &s.att)
	s.Equal(result.Decision, DecisionDeny)
}

func (s *oauthSuite) TestIncorrectPermissionInAttributes() {
	s.att.Permission = PermissionWrite
	authorizer := NewOAuthAuthorizer(s.cfg, s.logger)
	s.logger.On("Debug", "request is not authorized", mock.MatchedBy(func(t []tag.Tag) bool {
		return fmt.Sprintf("%v", t[0].Field().Interface) == "token doesn't have the right permission"
	}))
	result, _ := authorizer.Authorize(s.ctx, &s.att)
	s.Equal(result.Decision, DecisionDeny)
}

func (s *oauthSuite) TestIncorrectDomainInAttributes() {
	s.att.DomainName = "myotherdomain"
	authorizer := NewOAuthAuthorizer(s.cfg, s.logger)
	s.logger.On("Debug", "request is not authorized", mock.MatchedBy(func(t []tag.Tag) bool {
		return fmt.Sprintf("%v", t[0].Field().Interface) == "domain in token doesn't match with current domain"
	}))
	result, _ := authorizer.Authorize(s.ctx, &s.att)
	s.Equal(result.Decision, DecisionDeny)
}
