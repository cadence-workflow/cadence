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

type oauthAuthority struct {
	config      config.OAuthAuthorizer
	domainCache cache.DomainCache
	log         log.Logger
	parser      *jwt.Parser
	publicKey   interface{}
	jwks        *keyfunc.JWKS
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

// NewOAuthAuthorizer creates an oauth Authorizer
func NewOAuthAuthorizer(
	oauthConfig config.OAuthAuthorizer,
	log log.Logger,
	domainCache cache.DomainCache,
) (Authorizer, error) {
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
		config:      oauthConfig,
		domainCache: domainCache,
		log:         log,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}),
			jwt.WithIssuedAt(),
		),
		publicKey: key,
		jwks:      jwks,
	}, nil
}

// Authorize defines the logic to verify get claims from token
func (a *oauthAuthority) Authorize(ctx context.Context, attributes *Attributes) (Result, error) {
	call := yarpc.CallFromContext(ctx)

	token := call.Header(common.AuthorizationTokenHeaderName)
	if token == "" {
		a.log.Debug("request is not authorized", tag.Error(errors.New("token is not set in header")))
		return Result{Decision: DecisionDeny}, nil
	}

	var claims JWTClaims
	parsedToken, err := a.parser.ParseWithClaims(token, &claims, a.keyFunc)
	if err != nil {
		a.log.Debug("request is not authorized", tag.Error(err))
		return Result{Decision: DecisionDeny}, nil
	}

	if !isTokenInternal(parsedToken) {
		parsed, _, err := a.parser.ParseUnverified(token, jwt.MapClaims{})
		if err != nil {
			a.log.Debug("request is not authorized", tag.Error(err))
			return Result{Decision: DecisionDeny}, nil
		}

		if err := a.parseExternal(parsed.Claims.(jwt.MapClaims), &claims); err != nil {
			a.log.Debug("request is not authorized", tag.Error(err))
			return Result{Decision: DecisionDeny}, nil
		}
	}

	if err := a.validateTTL(&claims); err != nil {
		a.log.Debug("request is not authorized", tag.Error(err))
		return Result{Decision: DecisionDeny}, nil
	}

	if claims.Admin {
		return Result{Decision: DecisionAllow}, nil
	}

	// For cluster-level APIs (no domain), allow any authenticated non-admin user.
	// Domain-level permission checks rely on groups configured in domain data, which
	// does not exist for cluster-scoped operations. A valid token is sufficient.
	if attributes.DomainName == "" {
		return Result{Decision: DecisionAllow}, nil
	}

	domain, err := a.domainCache.GetDomain(attributes.DomainName)
	if err != nil {
		return Result{Decision: DecisionDeny}, err
	}

	if err := validatePermission(&claims, attributes, domain.GetInfo().Data); err != nil {
		a.log.Debug("request is not authorized", tag.Error(err))
		return Result{Decision: DecisionDeny}, nil
	}

	return Result{Decision: DecisionAllow}, nil
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
