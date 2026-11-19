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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmespath/go-jmespath"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

var _ jwt.Claims = (*JWTClaims)(nil)

const (
	groupSeparator    = " "
	jwtInternalIssuer = "internal-jwt"
)

// oauthAuthority verifies caller credentials and is shared by oauthAuthorizer and oauthAuthenticator.
type oauthAuthority struct {
	config    config.OAuthAuthorizer
	log       log.Logger
	parser    *jwt.Parser
	publicKey interface{}
	jwks      *keyfunc.JWKS
}

// JWTClaims is a Cadence specific claim with embeded Claims defined https://datatracker.ietf.org/doc/html/rfc7519#section-4.1
type JWTClaims struct {
	jwt.RegisteredClaims

	Name   string
	Groups string // separated by space
	Admin  bool
	TTL    int64 // TODO should be removed. ExpiresAt should be used
}

func (j JWTClaims) GetGroups() []string {
	return strings.Split(j.Groups, groupSeparator)
}

func newOAuthAuthority(oauthConfig config.OAuthAuthorizer, log log.Logger) (*oauthAuthority, error) {
	var jwks *keyfunc.JWKS
	var key interface{}
	var err error

	if oauthConfig.JwtCredentials != nil {
		if oauthConfig.JwtCredentials.Algorithm != jwt.SigningMethodRS256.Name {
			return nil, fmt.Errorf("algorithm %q is not supported", oauthConfig.JwtCredentials.Algorithm)
		}

		if key, err = common.LoadRSAPublicKey(oauthConfig.JwtCredentials.PublicKey); err != nil {
			return nil, fmt.Errorf("loading RSA public key: %w", err)
		}
	}

	if oauthConfig.Provider != nil {
		if oauthConfig.Provider.JWKSURL == "" {
			return nil, fmt.Errorf("JWKSURL is not set")
		}
		// Create the JWKS from the resource at the given URL.
		if jwks, err = keyfunc.Get(oauthConfig.Provider.JWKSURL, keyfunc.Options{}); err != nil {
			return nil, fmt.Errorf("creating JWKS from resource: %s error: %w", oauthConfig.Provider.JWKSURL, err)
		}
	}

	return &oauthAuthority{
		config: oauthConfig,
		log:    log,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}),
			jwt.WithIssuedAt(),
		),
		publicKey: key,
		jwks:      jwks,
	}, nil
}

// NewOAuthAuthorizerAndAuthenticator creates an oauth Authorizer together with a matching
// authentication-only Authorizer, both backed by a single oauthAuthority.
//
// An authority configured against a JWKS endpoint fetches the key set when constructed.
// Building the pair together makes one initial fetch and shares the same key set between
// authentication and authorization. The current JWKS options do not enable automatic refresh.
func NewOAuthAuthorizerAndAuthenticator(
	oauthConfig config.OAuthAuthorizer,
	log log.Logger,
	domainCache cache.DomainCache,
) (Authorizer, Authorizer, error) {
	authority, err := newOAuthAuthority(oauthConfig, log)
	if err != nil {
		return nil, nil, err
	}

	return newOAuthAuthorizer(authority, log, domainCache), newOAuthAuthenticator(authority), nil
}

// getAuthClaims verifies the caller's credentials and returns the claims they carry.
func (a *oauthAuthority) getAuthClaims(ctx context.Context) (*JWTClaims, error) {
	claims, err := a.parseClaims(ctx)
	if err != nil {
		a.log.Debug("request is not authenticated", tag.Error(err))
		return nil, err
	}
	return claims, nil
}

func (a *oauthAuthority) parseClaims(ctx context.Context) (*JWTClaims, error) {
	call := yarpc.CallFromContext(ctx)

	token := call.Header(common.AuthorizationTokenHeaderName)
	if token == "" {
		return nil, errors.New("token is not set in header")
	}

	var claims JWTClaims
	parsedToken, err := a.parser.ParseWithClaims(token, &claims, a.keyFunc)
	if err != nil {
		return nil, err
	}

	if !isTokenInternal(parsedToken) {
		parsed, _, err := a.parser.ParseUnverified(token, jwt.MapClaims{})
		if err != nil {
			return nil, err
		}

		if err := a.parseExternal(parsed.Claims.(jwt.MapClaims), &claims); err != nil {
			return nil, err
		}
	}

	if err := a.validateTTL(&claims); err != nil {
		return nil, err
	}

	return &claims, nil
}

// keyFunc returns correct key to check signature
func (a *oauthAuthority) keyFunc(token *jwt.Token) (interface{}, error) {
	if isTokenInternal(token) && a.publicKey != nil {
		return a.publicKey, nil
	}
	// External provider with JWKS provided
	// https://datatracker.ietf.org/doc/html/rfc7517
	if a.jwks != nil {
		return a.jwks.Keyfunc(token)
	}

	return nil, errors.New("no public key for verification")
}

func (a *oauthAuthority) validateTTL(claims *JWTClaims) error {
	// Fill ExpiresAt when TTL is passed
	if claims.TTL > 0 {
		claims.ExpiresAt = jwt.NewNumericDate(claims.IssuedAt.Time.Add(time.Second * time.Duration(claims.TTL)))
	}

	exp, err := claims.GetExpirationTime()

	if err != nil || exp == nil {
		return errors.New("ExpiresAt is not set")
	}

	timeLeft := exp.Unix() - time.Now().Unix()
	if timeLeft < 0 {
		return errors.New("token is expired")
	}

	if timeLeft > a.config.MaxJwtTTL {
		return fmt.Errorf("token TTL: %d is larger than MaxTTL allowed: %d", timeLeft, a.config.MaxJwtTTL)
	}

	return nil
}

func isTokenInternal(token *jwt.Token) bool {
	// external providers should set kid part always
	if _, ok := token.Header["kid"]; !ok {
		return true
	}

	issuer, err := token.Claims.GetIssuer()
	if err != nil {
		return false
	}

	return issuer == jwtInternalIssuer
}

func (a *oauthAuthority) parseExternal(rawClaims map[string]interface{}, claims *JWTClaims) error {
	if a.config.Provider.GroupsAttributePath != "" {
		userGroups, err := jmespath.Search(a.config.Provider.GroupsAttributePath, rawClaims)
		if err != nil {
			return fmt.Errorf("extracting JWT Groups claim: %w", err)
		}

		if _, ok := userGroups.(string); !ok {
			return errors.New("cannot convert groups to string")
		}

		claims.Groups = userGroups.(string)
	}

	if a.config.Provider.AdminAttributePath != "" {
		isAdmin, err := jmespath.Search(a.config.Provider.AdminAttributePath, rawClaims)
		if err != nil {
			return fmt.Errorf("extracting JWT Admin claim: %w", err)
		}
		if _, ok := isAdmin.(bool); !ok {
			return errors.New("cannot convert isAdmin to bool")
		}
		claims.Admin = isAdmin.(bool)
	}

	return nil
}
