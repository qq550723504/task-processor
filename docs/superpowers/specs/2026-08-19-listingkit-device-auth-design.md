# ListingKit device-authorized acceptance design

## Purpose

Enable an operator to acquire a short-lived ZITADEL user access token through
OAuth 2.0 Device Authorization and use it for an authenticated, local
ListingKit acceptance run. The first use is a controlled PAY-042 generation
canary for canonical tenant `373211199677923496`, whose approved billing
tenant is `1038`.

This is an operator workflow, not an application login replacement and not an
unattended deployment credential.

## Decisions

- Use ZITADEL's RFC 8628 Device Authorization Grant, not a copied browser
  session, password grant, personal token, or production client secret.
- Require an interactive sign-in for every acceptance run. Do not request
  `offline_access`, cache refresh tokens, or persist an access token.
- Reuse the existing `scripts/1688-runtime-acceptance.ps1` workflow and its
  existing `CREATE-1688-TASK` confirmation for any POST operation.
- Add an authenticated identity read endpoint before task creation. This
  prevents an operator from spending a canary generation under an unexpected
  ZITADEL organization.
- Keep the local usage ledger, quota reservation, and subscription fallback
  unchanged. This work only obtains the caller credential safely.

## Architecture

```text
operator browser -- ZITADEL login/MFA --> ZITADEL
       ^                                      |
       | verification URI and one-time code   | short-lived access token
       |                                      v
PowerShell acceptance runner --> ListingKit API --> existing introspection
                                  |                    |
                                  v                    v
                         auth-context check     current tenant/role rules
                                  |
                                  v
                      existing preflight / explicitly confirmed task POST
```

### Runtime components

1. `scripts/lib/listingkit-device-auth.ps1` owns the OAuth exchange. It is
   dot-sourced only by the acceptance runner and returns the access token in
   process memory.
2. `scripts/1688-runtime-acceptance.ps1` gains opt-in device-auth parameters:
   `-UseDeviceAuthorization`, `-IssuerURL`, `-ClientID`, and
   `-ExpectedTenantID`. Existing environment/file-token behavior remains
   unchanged for backwards compatibility.
3. The ListingKit API exposes one authenticated, read-only
   `GET /api/v1/listing-kits/auth-context` route. It returns only the verified
   canonical tenant ID, subject ID, and roles supplied by the existing
   middleware. It never returns a bearer token, client secret, raw
   introspection payload, or billing credentials.

The runner first calls `auth-context` and requires the returned canonical
tenant to equal `-ExpectedTenantID`. It then runs the current public and
authenticated health checks. A task-creating mode still requires the existing
literal confirmation and retains its existing source/store requirements.

## OAuth and input contract

The operator supplies only public configuration:

- ZITADEL issuer URL;
- the registered public device-client ID;
- ListingKit API base URL; and
- expected canonical tenant ID.

The helper obtains OIDC discovery from the configured issuer URL plus
`/.well-known/openid-configuration`, requires both the device
authorization and token endpoints, and requests the minimally required
interactive scopes `openid profile`. It sends the public client ID to the
device endpoint, displays the verification URI and user code, and polls at
the provider-supplied interval until success, denial, expiry, or caller
timeout.

The ZITADEL application must be registered as a public/native device client
with Device Authorization enabled. The operator account must belong to the
expected organization and have an already accepted ListingKit role (normally
`listingkit_operator` or stronger). Client registration and role assignment
are external prerequisites; this repository must not automate them with a
directory or invitation secret.

## Security rules

- Allow only `https` issuer and API URLs in production. Permit `http` only for
  literal loopback hosts in tests.
- Reject discovery endpoints that are not HTTPS (or permitted loopback) and
  not same-origin with the configured issuer.
- Keep the access token only in a local variable; never set a process-wide
  environment variable, write a token file, return it as script output, or
  include it in errors, evidence, or PowerShell transcript output.
- Redact authorization headers, access tokens, device codes, and URI query
  parameters before reporting an HTTP failure.
- Do not use `ZITADEL_CLIENT_SECRET`,
  `TASK_PROCESSOR_LISTINGKIT_ZITADEL_TENANT_DIRECTORY_TOKEN`, member-invite
  credentials, or Kubernetes Secret reads in the operator tool.
- Do not open the browser by default. An explicit `-OpenBrowser` option may
  open only the verified provider URI.

## Error handling

- Discovery/configuration errors fail before any API request.
- Provider `authorization_pending` continues polling; `slow_down` increases
  the interval; `access_denied`, `expired_token`, invalid responses, and the
  local timeout fail closed.
- A failed `auth-context` request, tenant mismatch, or missing required role
  prevents health checks and all task POSTs.
- A successful token does not bypass existing API authorization,
  subscription, store access, quota, or task-confirmation gates.

## Tests and acceptance evidence

Pester tests will mock the device and token endpoints and prove:

1. happy-path token stays in memory and permits the authenticated preflight;
2. no token is printed or written to the workspace;
3. endpoint scheme/origin injection is rejected;
4. pending, slow-down, denial, expiry, and malformed provider responses fail
   with redacted messages;
5. an `auth-context` tenant mismatch prevents any POST;
6. existing static-token and no-token behavior remains compatible; and
7. the new route returns only verified identity fields and is protected by the
   existing ZITADEL middleware.

A production acceptance run is separate from implementation validation. It
must record the generated task ID and canonical tenant without recording the
token, then use the existing PAY-042 evidence path to verify that the task is
admitted only through billing tenant `1038`.

## Operator preflight

After a ZITADEL administrator has registered a public device client and given
the operator an accepted ListingKit role, run the read-only preflight from the
repository root. Substitute only public issuer and client values; do not place
a token in the command, environment, or a file.

```powershell
pwsh ./scripts/1688-runtime-acceptance.ps1 `
  -Mode Preflight `
  -UseDeviceAuthorization `
  -IssuerURL 'https://issuer.example' `
  -ClientID 'public-device-client-id' `
  -ExpectedTenantID '373211199677923496'
```

The command displays a verification URI and one-time user code, then checks
the authenticated canonical tenant before it performs the existing health
checks. It creates no task. `Crawl` and `EndToEnd` remain separately guarded
by the existing source/store requirements and
`-ConfirmCreateTask CREATE-1688-TASK`.

## Non-goals

- No service account, client-credentials grant, refresh-token storage, CI
  authentication, or unattended task creation.
- No change to production payment, OpenMeter projection, entitlement policy,
  quota semantics, or tenant mapping.
- No scraping of browser state, Kubernetes Secrets, or production client
  secrets.

## Rollout and rollback

The helper and route are inert until an operator opts into
`-UseDeviceAuthorization`. Existing token-based callers are unchanged. If the
new flow proves unsuitable, remove the opt-in script and route; no persisted
credential or data migration needs rollback.

## References

- [ZITADEL device authorization guide](https://zitadel.com/docs/guides/integrate/login/oidc/device-authorization)
- [ZITADEL supported OAuth grant types](https://zitadel.com/docs/apis/openidoauth/grant-types)
