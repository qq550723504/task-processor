# ListingKit ZITADEL Auth Convergence Design

## Status

- Status: Draft
- Date: 2026-06-21
- Scope: ListingKit UI proxy and Go API authentication boundary

## Problem

ListingKit currently validates ZITADEL identity in two places:

1. The Next.js proxy checks the Auth.js session or incoming bearer token, calls ZITADEL introspection, applies allowlist authorization, then forwards the request.
2. The Go API middleware reads the forwarded `Authorization: Bearer ...` token, calls ZITADEL introspection again, injects tenant/user headers, and applies route authorization.

This double validation is conservative, but it creates avoidable complexity:

- Local and automated validation have two failure modes: missing frontend session and missing backend bearer token.
- ZITADEL introspection is duplicated for proxied API requests.
- Allowlist policy can drift between TypeScript and Go implementations.
- Error messages do not clearly identify the authoritative auth layer.

## Decision

Make the Go API the only authoritative ListingKit API authentication and authorization layer.

The Next.js app remains responsible for browser login and session storage. The Next.js ListingKit proxy becomes a token-forwarding boundary rather than a second authorization authority.

## Target Flow

```text
Browser
  -> Next.js Auth.js / ZITADEL login
  -> Next.js /api/listing-kits/*
       - read bearer token from request header, or access token from Auth.js session
       - if no token is available, return a frontend session error
       - forward Authorization: Bearer <token>
       - forward trace headers
       - do not call ZITADEL introspection
       - do not apply ListingKit allowlist authorization
  -> Go API /api/v1/*
       - call ZITADEL discovery/introspection
       - reject missing, inactive, or invalid tokens
       - inject tenant/user/role headers
       - apply allowlist and route role authorization
       - execute handlers
```

Direct callers may also call the Go API with a valid bearer token. The Go API must not trust tenant or user headers unless they are derived from the verified token by the middleware.

## Component Responsibilities

### Next.js UI

Owns:

- Auth.js login/logout/session lifecycle.
- User-facing session status.
- Extracting an access token from Auth.js session.
- Forwarding bearer tokens to the Go API.
- Returning a clear `Missing ZITADEL session` error when a browser request has no session/token.

Does not own:

- ZITADEL token introspection for ListingKit API proxy requests.
- ListingKit allowlist authorization.
- Route role authorization.
- Authoritative tenant/user identity for Go handlers.

### Go API

Owns:

- ZITADEL discovery and token introspection.
- Missing, inactive, invalid, and unauthorized token responses.
- Tenant/user/role header injection from verified identity.
- ListingKit allowlist authorization.
- Route role authorization.
- Direct API access with bearer tokens for Go-based integration tests and operational tooling.

## Error Semantics

The optimized flow keeps two error classes, with clearer ownership:

| Layer | Situation | Response |
| --- | --- | --- |
| Next proxy | Browser request has no Auth.js session and no bearer header | `401 zitadel_session_missing` or current compatible `401 zitadel_token_invalid` with `Missing ZITADEL session` |
| Go API | Request reaches backend without bearer token | `401 zitadel_token_missing` |
| Go API | Token introspection fails or token inactive | `401 zitadel_token_invalid` |
| Go API | Identity is authenticated but not allowed | `403 zitadel_access_denied` or route-specific permission error |

For compatibility, frontend UI may continue to display `Missing ZITADEL session` for browser-session failures.

## Migration Plan

1. Add/adjust frontend proxy tests proving proxied requests do not call ZITADEL introspection when a session token or bearer token exists.
2. Change `verifyListingKitRequestIdentity` so it:
   - accepts an incoming bearer token without introspection;
   - otherwise reads Auth.js session access token;
   - optionally reads cached session identity for forwarding headers only;
   - does not call `verifyZitadelAccessToken` for proxy requests.
3. Keep `authorizeZitadelIdentity` available only for `/api/zitadel-auth/session` if still needed for UI status, or retire it from the proxy path.
4. Keep `buildListingKitUpstreamHeaders` from forwarding caller-supplied tenant/user headers as authoritative identity. Prefer verified or session-derived identity, and let Go overwrite identity headers after introspection.
5. Add/adjust Go tests confirming backend middleware remains authoritative for:
   - missing bearer token;
   - inactive token;
   - allowed user;
   - forbidden user;
   - route role permission.
6. Run Go backend verification as the authoritative API validation:
   - `go test ./internal/listingkit/httpapi -run "TestListingKitZitadelAuth" -count=1`
   - `go test ./internal/app/httpapi ./internal/listingkit/httpapi -count=1`
   - `go test ./... -count=1`
7. Run targeted frontend proxy tests only because this change touches Next.js proxy behavior.

## Security Notes

- The Go API remains protected for direct callers because it still requires and introspects bearer tokens.
- The proxy must not mint or trust tenant/user identity without a token.
- Forwarded tenant/user headers are convenience hints only; Go middleware overwrites them from ZITADEL identity.
- Keeping the backend as the only authority prevents TypeScript/Go allowlist drift.

## Out Of Scope

- Replacing Auth.js.
- Changing ZITADEL client configuration.
- Introducing a private proxy-to-backend service token.
- Reworking non-ListingKit authentication.
- Changing SDS or SHEIN platform login flows.

## Acceptance Criteria

- A browser request with a valid Auth.js session reaches Go with `Authorization: Bearer <access_token>`.
- A direct Go API request with a valid bearer token works without the Next.js session.
- The Next.js proxy no longer performs ZITADEL introspection for ListingKit API requests.
- Go backend middleware remains the only place that decides ListingKit API authentication and authorization.
- Existing route role protections continue to pass.
- Missing-session and missing-bearer errors are distinguishable by layer.
