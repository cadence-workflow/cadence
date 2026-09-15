## Cadence has two authorizer options:

1. OAuthAuthorizer: validates JWTs issued by your Identity Provider and enforces permissions.
2. NoopAuthorizer: turns authorization off.

In order to configure, add an authorization section to Cadence server config [example](https://github.com/cadence-workflow/cadence/blob/master/config/development_oauth.yaml). These fields map 1:1 to the Go structs in [common/config](https://github.com/cadence-workflow/cadence/blob/master/common/config/authorization.go).

### Option A for OAuth : Validate tokens via JWKS


    authorization:
        oauthAuthorizer:
            enable: true 
            # Reject tokens with excessively long TTL (seconds). Optional but recommended.
            maxJwtTTL: 3600 
    
            # JWT verification config (algorithm + how to fetch public keys)
            jwtCredentials:
                algorithm: RS256         # supported: RS256
            # publicKey is optional if you supply a JWKS URL (below)
            # publicKey: /etc/cadence/keys/idp-public.pem

            provider:
                jwksURL: "https://YOUR_IDP/.well-known/jwks.json"
                # Optional JSONPath-like claims locations used by Cadence:
                groupsAttributePath: "groups"      
                adminAttributePath: "admin"

### Option B for OAuth : Validate tokens via a static public key


    authorization:
        oauthAuthorizer:
            enable: true
            maxJwtTTL: 3600
            jwtCredentials:
                algorithm: RS256
                publicKey: /etc/cadence/keys/idp-public.pem

### NoopAuthorizer: Turning authz off


    authorization:
        noopAuthorizer:
            enable: true

## Authentication-only authorizer

The authenticator uses the same `authorization.oauthAuthorizer.enable` flag: when enabled,
it validates JWTs without checking permissions; otherwise, it is a no-op. It lets APIs
such as `ListDomains` check credentials before checking access to individual domains.

If you use a custom authorizer, then it's recommended you can provide an optional `resource.Params.Authenticator`
implementing the same `Authorizer` interface. The authenticator checks credentials only; the
authorizer also checks permissions. The authorizer must still validate credentials because
other APIs call it directly. For an example of sharing credential validation,
see `oauthAuthority` and `NewOAuthAuthorizerAndAuthenticator` in
[oauthAuthority.go](oauthAuthority.go). If no custom authenticator is supplied, the configured OAuth
or no-op behavior applies.

This is an interim step toward separating authentication from authorization. The authorizer
still validates credentials on each call; a future change could authenticate once per
request and pass the caller's identity to the authorizer.

## Background

The server constructs an authorization.Attributes object for each API call (actor, API name, domain, optional workflow/tasklist), evaluates the token, and returns an allow/deny Decision. JWTs are expected to contain Cadence-specific claims including groups and (optionally) an admin flag.

### Key structs & functions:

```
authorization.Authorizer interface 

authorization.Attributes 
 
authorization.Decision

authorization.JWTClaims
```

When OAuth authZ is enabled, clients must present a valid JWT to the frontend service on every call (Cadence uses the provided token to authorize the API/Domain access). The exact header/wire placement is handled by Cadence’s server middleware and the client transport; the important bit is that the token must validate against your jwksURL/publicKey, include expected claims (groups/admin), and not exceed maxJwtTTL. 
