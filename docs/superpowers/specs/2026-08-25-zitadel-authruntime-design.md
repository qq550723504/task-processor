# Zitadel Authentication Runtime Extraction Design

## Status

Proposed for review.

## Goal

Move ZITADEL discovery and token-introspection behavior out of the ListingKit HTTPAPI implementation into a neutral authentication-runtime package, while preserving the existing public compatibility entry points and request behavior.

## Context and current ownership

The current implementation is split across these files under `internal/listingkit/httpapi`:

- `zitadel_auth.go` owns runtime configuration and middleware state types.
- `zitadel_auth_runtime.go` owns configuration, global runtime state, and middleware construction.
- `zitadel_auth_middleware.go` owns discovery, token introspection, verified identity projection, trusted-header replacement, and authentication errors.
- `zitadel_auth_route_authorization.go` owns ListingKit route policy and permission checks.

The first auth-context slice already established `internal/authidentity` as the neutral owner of verified identity context. The next boundary should make the component that verifies ZITADEL credentials neutral as well. ListingKit-specific route policy remains a separate concern.

## Scope

This slice will:

1. Add `internal/authruntime/zitadel` as the owner of ZITADEL discovery and token-introspection runtime behavior.
2. Move the reusable authentication runtime configuration, discovery cache, introspection response mapping, and verified identity projection there.
3. Keep `internal/listingkit/httpapi.NewZitadelAuthMiddlewareFromEnv` and `ConfigureListingKitZitadelAuth` as compatibility adapters during migration.
4. Preserve the current `authidentity.AuthenticatedIdentity` context values, trusted-header cleanup/replacement, HTTP status codes, error keys, and fail-closed behavior.
5. Keep `RouteRequiresZitadelAuth`, `NewRouteRoleMiddleware`, ListingKit permission mapping, and `ConfigureListingKitAuthorization` in `internal/listingkit/httpapi` for this slice.
6. Add boundary tests that prevent new consumers from importing the retired ListingKit implementation for generic ZITADEL verification, while allowing the explicit compatibility adapter.

## Non-goals

This slice will not:

- migrate ListingKit route authorization or `internal/authz` permission policy;
- change ZITADEL issuer, client, allowlist, or timeout semantics;
- change SHEIN Login, Local Agent, 1688, tenant bridge, database Builder, or deployment behavior;
- remove the compatibility functions from `internal/listingkit/httpapi`;
- introduce a new authentication provider abstraction or replace the existing HTTP introspection protocol.

## Alternatives considered

### A. Extract only a low-level introspection client

This would move discovery and introspection calls but leave identity validation, header cleanup, and context projection in ListingKit HTTPAPI. It reduces the first diff but leaves generic authentication behavior owned by ListingKit, so it does not fully achieve the boundary goal.

### B. Extract the full authentication runtime, keep route authorization local (recommended)

The neutral package owns the provider-facing verification lifecycle and verified identity projection. ListingKit HTTPAPI remains an adapter and retains ListingKit-specific route and permission policy. This matches the ownership rule, keeps the migration independently testable, and avoids mixing provider verification with product authorization.

### C. Extract authentication and route authorization together

This would move more code but couples a neutral provider runtime to ListingKit permission semantics. It creates a larger compatibility surface and makes it harder to review whether a change affects authentication or business authorization. It is deferred to a separate slice if still needed.

## Proposed architecture

```text
ZITADEL HTTP provider
        |
        v
internal/authruntime/zitadel
  - runtime config
  - discovery cache
  - token introspection
  - verified identity validation
  - authidentity context projection
        |
        v
internal/listingkit/httpapi compatibility adapter
  - ConfigureListingKitZitadelAuth
  - NewZitadelAuthMiddlewareFromEnv
        |
        +--> ListingKit route authorization remains local
              - RouteRequiresZitadelAuth
              - NewRouteRoleMiddleware
              - ListingKit permission mapping
```

The neutral package may depend on `internal/authidentity`, shared configuration types, and standard HTTP/Gin primitives. It must not depend on `internal/listingkit` or ListingKit route descriptors. The compatibility adapter may depend on both the neutral runtime and ListingKit-specific authorization code.

## Interface and compatibility contract

The implementation plan should expose a small neutral contract equivalent to:

```go
type Config struct {
    IssuerURL    string
    ClientID     string
    ClientSecret string
    HTTPClient   *http.Client
    Required     bool
    AllowedTenantIDs map[string]struct{}
    AllowedUserIDs   map[string]struct{}
    AllowedRoles     map[string]struct{}
}

func NewMiddleware(cfg Config) gin.HandlerFunc
```

The exact exported names may follow repository conventions, but the contract must make provider verification independent from ListingKit route authorization. The adapter remains responsible for translating `config.ListingKitZitadelConfig`, configuring the existing ListingKit authorizer, and returning the same middleware behavior to current callers.

The runtime must continue to:

- reject missing or incomplete configuration with `503 zitadel_auth_not_configured`;
- reject missing bearer tokens with `401 zitadel_token_missing`;
- reject invalid, inactive, malformed, unavailable, or failed introspection with `401 zitadel_token_invalid`;
- reject missing resource owner or subject with `403 zitadel_tenant_missing` or `403 zitadel_user_missing`;
- clear caller-supplied identity headers before setting verified values;
- write `authidentity.AuthenticatedIdentity` to the request context before the handler continues;
- apply the existing global allowlist fail-closed behavior when configured.

## Data flow

1. The app/server obtains the compatibility middleware from `listingkit/httpapi` exactly as today.
2. The compatibility adapter translates ListingKit config and calls the neutral runtime constructor.
3. The neutral middleware validates configuration and extracts the Bearer token.
4. The neutral runtime loads and caches the provider discovery document.
5. It introspects the token using the configured client credentials and maps the verified subject, resource owner, and roles.
6. It rejects missing identity fields or failed global allowlist authorization using the existing response contract.
7. It removes untrusted identity headers and writes the verified `authidentity` context.
8. It calls the next handler; ListingKit route middleware may then apply product-specific permissions.

## Error handling and concurrency

- Preserve the current fail-closed behavior for missing configuration, missing token, discovery failure, introspection failure, inactive token, and missing canonical identity fields.
- Preserve request context cancellation through discovery and introspection HTTP calls.
- Preserve the existing five-second default HTTP timeout when callers do not provide a client.
- Keep discovery caching scoped to a middleware instance; protect it with the existing synchronization strategy or an equivalent race-safe implementation.
- Do not log bearer tokens, client secrets, or raw introspection payloads.

## Testing strategy

The implementation must use TDD and retain the existing behavior tests while adding neutral-package tests for:

- missing configuration and missing bearer token;
- discovery success and cached reuse;
- discovery non-2xx, malformed, and transport failures;
- introspection success with subject, resource owner, and deduplicated roles;
- inactive, malformed, non-2xx, and transport-failed introspection;
- missing resource owner and missing subject;
- global allowlist success and fail-closed rejection;
- forged identity-header removal and verified `authidentity` context projection;
- compatibility adapter behavior from existing `listingkit/httpapi` callers.

Boundary tests must verify that generic authentication-runtime consumers use `internal/authruntime/zitadel`, while the ListingKit compatibility adapter is the only permitted bridge to the retired implementation path. Existing route authorization tests remain unchanged and continue to validate ListingKit permission semantics.

## Rollout and retirement

This is a behavior-preserving internal migration. The first implementation PR should contain only the neutral runtime, its tests, the compatibility adapter, and the boundary documentation/test guard. No production configuration or deployment manifest changes are required. The compatibility adapter remains until repository-wide callers have migrated and a separate retirement slice confirms zero external references.

## Acceptance criteria

- ZITADEL verification and verified identity projection are owned by `internal/authruntime/zitadel`.
- `internal/listingkit/httpapi` public compatibility entry points still compile and preserve behavior.
- ListingKit route authorization remains in `internal/listingkit/httpapi` and is not duplicated in the neutral runtime.
- Focused neutral-runtime, compatibility, route-authorization, and boundary tests pass.
- No new generic authentication consumer imports the old ListingKit implementation path.
- No SHEIN Login, Local Agent, 1688, tenant bridge, database Builder, or deployment code changes are included.
